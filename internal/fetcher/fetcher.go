
package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/temoto/robotstxt"
	"golang.org/x/net/html"
)

// Fetcher fetches a page and returns absolute URLs of all links on it.
type Fetcher interface {
	Fetch(pageURL string) ([]string, error)
}

// HTTP implements Fetcher using net/http.
type HTTP struct {
	client *http.Client
	robots *robotstxt.RobotsData
}

// New returns an HTTP fetcher with a 10-second timeout.
func New(ctx context.Context, baseURL string) (*HTTP, error) {
	return NewWithClient(ctx, baseURL, &http.Client{Timeout: 10 * time.Second})
}

// NewWithClient returns an HTTP fetcher with a custom HTTP client.
func NewWithClient(ctx context.Context, baseURL string, client *http.Client) (*HTTP, error) {
	h := &HTTP{client: client}
	if err := h.loadRobots(ctx, baseURL); err != nil {
		// Log but don't fail — missing robots.txt is fine
		fmt.Fprintf(io.Discard, "warning: could not load robots.txt: %v\n", err)
	}
	return h, nil
}

func (f *HTTP) loadRobots(ctx context.Context, baseURL string) error {
	u, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", u.Scheme, u.Host)

	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("robots.txt returned %d", resp.StatusCode)
	}

	var data *robotstxt.RobotsData
	data, err = robotstxt.FromResponse(resp)
	if err != nil {
		return err
	}
	f.robots = data
	return nil
}

// IsAllowed checks if the URL is allowed by robots.txt.
func (f *HTTP) IsAllowed(pageURL string) bool {
	if f.robots == nil {
		return true
	}
	u, err := url.Parse(pageURL)
	if err != nil {
		return false
	}
	return f.robots.TestAgent(u.Path, "*")
}

func (f *HTTP) Fetch(pageURL string) ([]string, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL %s: %w", pageURL, err)
	}

	resp, err := f.client.Get(pageURL)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", pageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("GET %s: status %d", pageURL, resp.StatusCode)
	}

	return extractLinks(resp.Body, base)
}

func extractLinks(body io.Reader, base *url.URL) ([]string, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	seen := make(map[string]bool)
	var links []string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					if link := resolveLink(attr.Val, base); link != "" && !seen[link] {
						seen[link] = true
						links = append(links, link)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return links, nil
}

func resolveLink(href string, base *url.URL) string {
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}

	resolved := base.ResolveReference(u)

	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}

	resolved.Fragment = ""
	return resolved.String()
}
