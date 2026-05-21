package crawler

import (
	"context"
	"net/url"
)

// PageResult holds the URL of a visited page and all links found on it.
type PageResult struct {
	URL   string   `json:"url"`
	Links []string `json:"links"`
}

// Crawler crawls pages starting from a given URL and streams results as they are visited.
type Crawler interface {
	Crawl(ctx context.Context, startURL string) (<-chan PageResult, error)
}

// normalise strips fragments and treats http as https for deduplication.
func normalise(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// If parsing fails, return rawURL as-is; deduplication may miss this entry.
		return rawURL
	}
	u.Fragment = ""
	if u.Scheme == "http" {
		u.Scheme = "https"
	}
	return u.String()
}
