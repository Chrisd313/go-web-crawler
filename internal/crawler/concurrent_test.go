package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chrisd313/web-crawler/internal/fetcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body>
			<a href="/about">About</a>
			<a href="https://external.com/page">External</a>
		</body></html>`)
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body><a href="/">Home</a></body></html>`)
	})
	return httptest.NewServer(mux)
}

func TestConcurrent_VisitsAllSubdomainPages(t *testing.T) {
	// Arrange
	ts := buildTestServer(t)
	defer ts.Close()

	ctx := context.Background()
	f, err := fetcher.New(ctx, ts.URL+"/")
	require.NoError(t, err)

	c := NewConcurrent(f, 3, 0)

	// Act
	resultCh, err := c.Crawl(ctx, ts.URL+"/")
	require.NoError(t, err)

	var visited []string
	for r := range resultCh {
		visited = append(visited, r.URL)
	}
	sort.Strings(visited)

	// Assert
	want := []string{ts.URL + "/", ts.URL + "/about"}
	sort.Strings(want)
	assert.Equal(t, want, visited)
}

func TestConcurrent_DoesNotVisitPageTwice(t *testing.T) {
	// Arrange
	ts := buildTestServer(t)
	defer ts.Close()

	ctx := context.Background()
	f, err := fetcher.New(ctx, ts.URL+"/")
	require.NoError(t, err)

	c := NewConcurrent(f, 3, 0)

	// Act
	resultCh, err := c.Crawl(ctx, ts.URL+"/")
	require.NoError(t, err)

	counts := make(map[string]int)
	for r := range resultCh {
		counts[r.URL]++
	}

	// Assert
	for u, n := range counts {
		assert.Equal(t, 1, n, "URL %q visited %d times, want 1", u, n)
	}
}

func TestConcurrent_UsesMultipleWorkers(t *testing.T) {
	// Arrange
	var current, peak int64

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<a href="/a">A</a><a href="/b">B</a><a href="/c">C</a>
		</body></html>`)
	})
	for _, p := range []string{"/a", "/b", "/c"} {
		path := p
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt64(&current, 1)
			for {
				old := atomic.LoadInt64(&peak)
				if n <= old || atomic.CompareAndSwapInt64(&peak, old, n) {
					break
				}
			}
			defer atomic.AddInt64(&current, -1)
			time.Sleep(5 * time.Millisecond)
			fmt.Fprint(w, `<html><body></body></html>`)
		})
	}
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()
	f, err := fetcher.New(ctx, ts.URL+"/")
	require.NoError(t, err)

	c := NewConcurrent(f, 3, 0)

	// Act
	resultCh, err := c.Crawl(ctx, ts.URL+"/")
	require.NoError(t, err)

	var visited []string
	for r := range resultCh {
		visited = append(visited, r.URL)
	}

	// Assert
	assert.Len(t, visited, 4)
	peakWorkers := atomic.LoadInt64(&peak)
	assert.GreaterOrEqual(t, peakWorkers, int64(2))
}

func TestConcurrent_IncludesExternalLinksInResult(t *testing.T) {
	// Arrange
	ts := buildTestServer(t)
	defer ts.Close()

	ctx := context.Background()
	f, err := fetcher.New(ctx, ts.URL+"/")
	require.NoError(t, err)

	c := NewConcurrent(f, 3, 0)

	// Act
	resultCh, err := c.Crawl(ctx, ts.URL+"/")
	require.NoError(t, err)

	var rootResult *PageResult
	for r := range resultCh {
		if r.URL == ts.URL+"/" || r.URL == ts.URL {
			rc := r
			rootResult = &rc
		}
	}

	// Assert
	require.NotNil(t, rootResult)
	assert.Contains(t, rootResult.Links, "https://external.com/page")
}

func TestConcurrent_CrawlWithInvalidStartURL(t *testing.T) {
	// Arrange
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body></body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()
	f, err := fetcher.New(ctx, ts.URL+"/")
	require.NoError(t, err)

	c := NewConcurrent(f, 3, 0)

	// Act
	resultCh, err := c.Crawl(ctx, "://invalid-url")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resultCh)
}

func TestConcurrent_CrawlWithContextCancellation(t *testing.T) {
	// Arrange
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<a href="/a">A</a><a href="/b">B</a><a href="/c">C</a>
		</body></html>`)
	})
	for _, p := range []string{"/a", "/b", "/c"} {
		path := p
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			fmt.Fprint(w, `<html><body></body></html>`)
		})
	}
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	f, err := fetcher.New(ctx, ts.URL+"/")
	require.NoError(t, err)

	c := NewConcurrent(f, 3, 0)

	// Act
	resultCh, err := c.Crawl(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Cancel context early
	time.Sleep(2 * time.Millisecond)
	cancel()

	var visited int
	for r := range resultCh {
		visited++
		_ = r
	}

	// Assert - should get at least the root page
	assert.GreaterOrEqual(t, visited, 1)
}
