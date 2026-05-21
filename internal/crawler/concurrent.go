package crawler

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/chrisd313/web-crawler/internal/fetcher"
	"golang.org/x/time/rate"
)

// Concurrent crawls a subdomain using a coordinator goroutine and a pool of worker goroutines.
type Concurrent struct {
	fetcher *fetcher.HTTP
	workers int
	limiter *rate.Limiter
}

// NewConcurrent returns a Concurrent crawler with the given number of workers and optional rate limit (requests per second).
// If rps <= 0, no rate limiting is applied.
func NewConcurrent(f *fetcher.HTTP, workers int, rps float64) *Concurrent {
	if workers < 1 {
		workers = 1
	}
	var limiter *rate.Limiter
	if rps > 0 {
		limiter = rate.NewLimiter(rate.Limit(rps), 1)
	}
	return &Concurrent{fetcher: f, workers: workers, limiter: limiter}
}

// Crawl visits all reachable pages on the same host as startURL using up to c.workers
// goroutines concurrently. The context can be cancelled to stop the crawl gracefully.
// Results are sent on the returned channel, closed on completion.
func (c *Concurrent) Crawl(ctx context.Context, startURL string) (<-chan PageResult, error) {
	u, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("parse start URL: %w", err)
	}
	allowedHost := u.Host

	type workerResult struct {
		pageURL string
		links   []string
		err     error
	}

	results := make(chan PageResult)

	go func() {
		defer close(results)

		workerResults := make(chan workerResult, c.workers)
		normStart := normalise(startURL)
		visited := map[string]bool{normStart: true}
		queue := []string{startURL}
		inFlight := 0

		for len(queue) > 0 || inFlight > 0 {
			// Dispatch as many workers as we have capacity for.
			for len(queue) > 0 && inFlight < c.workers {
				select {
				case <-ctx.Done():
					// Context cancelled, stop dispatching
					break
				default:
				}

				pageURL := queue[0]
				queue = queue[1:]
				inFlight++
				go func(pu string) {
					if c.limiter != nil {
						c.limiter.Wait(ctx)
					}
					links, err := c.fetcher.Fetch(pu)
					workerResults <- workerResult{pageURL: pu, links: links, err: err}
				}(pageURL)
			}

			// Receive one result from any worker.
			select {
			case wr := <-workerResults:
				inFlight--

				if wr.err != nil {
					fmt.Fprintf(os.Stderr, "error fetching %s: %v\n", wr.pageURL, wr.err)
					continue
				}

				for _, link := range wr.links {
					lu, parseErr := url.Parse(link)
					if parseErr != nil || lu.Host != allowedHost {
						continue
					}
					// Check robots.txt before adding to queue
					if !c.fetcher.IsAllowed(link) {
						continue
					}
					if key := normalise(link); !visited[key] {
						visited[key] = true
						queue = append(queue, link)
					}
				}

				results <- PageResult{URL: wr.pageURL, Links: wr.links}
			case <-ctx.Done():
				// Context cancelled - drain remaining results and exit
				return
			}
		}
	}()

	return results, nil
}
