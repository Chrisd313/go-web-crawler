package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/chrisd313/web-crawler/internal/crawler"
	"github.com/chrisd313/web-crawler/internal/fetcher"
	"github.com/chrisd313/web-crawler/internal/output"
)

func main() {
	startURL := flag.String("url", "", "Starting URL to crawl (required)")
	format := flag.String("format", "text", "Output format: text or json")
	workers := flag.Int("workers", 5, "Number of concurrent workers")
	rps := flag.Float64("rate", 0, "Request rate limit (requests per second, 0 = unlimited)")
	flag.Parse()

	if *startURL == "" {
		fmt.Fprintln(os.Stderr, "error: --url is required")
		flag.Usage()
		os.Exit(1)
	}

	// Set up graceful shutdown on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	f, err := fetcher.New(ctx, *startURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize fetcher: %v\n", err)
		os.Exit(1)
	}

	c := crawler.NewConcurrent(f, *workers, *rps)

	var r output.Reporter
	switch *format {
	case "text":
		r = output.NewText(os.Stdout)
	case "json":
		r = output.NewJSON(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown --format %q (want: text or json)\n", *format)
		os.Exit(1)
	}

	results, err := c.Crawl(ctx, *startURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: crawl failed: %v\n", err)
		os.Exit(1)
	}

	for result := range results {
		r.Report(result)
	}
}
