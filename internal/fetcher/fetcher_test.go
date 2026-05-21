package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetch_ExtractsAbsoluteLinks(t *testing.T) {
	// Arrange
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body>
			<a href="/about">About</a>
			<a href="https://external.com/page">External</a>
		</body></html>`)
	}))
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act
	links, err := f.Fetch(ts.URL + "/")

	// Assert
	require.NoError(t, err)
	want := []string{ts.URL + "/about", "https://external.com/page"}
	sort.Strings(links)
	sort.Strings(want)
	assert.Equal(t, want, links)
}

func TestFetch_StripsFragments(t *testing.T) {
	// Arrange
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body><a href="/page#section">Link</a></body></html>`)
	}))
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act
	links, err := f.Fetch(ts.URL + "/")

	// Assert
	require.NoError(t, err)
	assert.Len(t, links, 1)
	assert.Equal(t, ts.URL+"/page", links[0])
}

func TestFetch_DiscardsBadSchemes(t *testing.T) {
	// Arrange
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body>
			<a href="mailto:foo@bar.com">Email</a>
			<a href="tel:+123456">Phone</a>
			<a href="javascript:void(0)">JS</a>
			<a href="/good">Good</a>
		</body></html>`)
	}))
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act
	links, err := f.Fetch(ts.URL + "/")

	// Assert
	require.NoError(t, err)
	assert.Len(t, links, 1)
	assert.Equal(t, ts.URL+"/good", links[0])
}

func TestFetch_DeduplicatesLinks(t *testing.T) {
	// Arrange
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body>
			<a href="/page">First</a>
			<a href="/page">Duplicate</a>
		</body></html>`)
	}))
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act
	links, err := f.Fetch(ts.URL + "/")

	// Assert
	require.NoError(t, err)
	assert.Len(t, links, 1)
}

func TestFetch_ErrorOnNon200(t *testing.T) {
	// Arrange
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act
	links, err := f.Fetch(ts.URL + "/missing")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, links)
}

func TestFetch_ErrorOn500(t *testing.T) {
	// Arrange
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act
	links, err := f.Fetch(ts.URL + "/error")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, links)
}

func TestFetch_HandlesEmptyPage(t *testing.T) {
	// Arrange
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body></body></html>`)
	}))
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act
	links, err := f.Fetch(ts.URL + "/")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, links)
}

func TestFetch_HandlesRelativeURLs(t *testing.T) {
	// Arrange
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body>
			<a href="relative">Relative</a>
			<a href="/absolute">Absolute</a>
			<a href="./current">Current</a>
		</body></html>`)
	}))
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act
	links, err := f.Fetch(ts.URL + "/")

	// Assert
	require.NoError(t, err)
	assert.Len(t, links, 3)
	assert.Contains(t, links, ts.URL+"/relative")
	assert.Contains(t, links, ts.URL+"/absolute")
	assert.Contains(t, links, ts.URL+"/current")
}

func TestIsAllowed_AllowsPathsNotDisallowed(t *testing.T) {
	// Arrange
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `User-agent: *
Disallow: /admin
Disallow: /private`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body></body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act & Assert
	assert.True(t, f.IsAllowed(ts.URL+"/"))
	assert.True(t, f.IsAllowed(ts.URL+"/about"))
	assert.True(t, f.IsAllowed(ts.URL+"/contact"))
}

func TestIsAllowed_BlocksDisallowedPaths(t *testing.T) {
	// Arrange
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `User-agent: *
Disallow: /admin
Disallow: /private`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body></body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act & Assert
	assert.False(t, f.IsAllowed(ts.URL+"/admin"))
	assert.False(t, f.IsAllowed(ts.URL+"/admin/panel"))
	assert.False(t, f.IsAllowed(ts.URL+"/private"))
	assert.False(t, f.IsAllowed(ts.URL+"/private/data"))
}

func TestIsAllowed_WhenNoRobotsTxt(t *testing.T) {
	// Arrange
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act & Assert
	assert.True(t, f.IsAllowed(ts.URL+"/anything"))
	assert.True(t, f.IsAllowed(ts.URL+"/admin"))
}

func TestIsAllowed_WithInvalidURL(t *testing.T) {
	// Arrange
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `User-agent: *
Disallow: /admin`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body></body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act & Assert - invalid URL should return false
	assert.False(t, f.IsAllowed("://invalid"))
}

func TestNew_WithMissingRobotsTxt(t *testing.T) {
	// Arrange
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ctx := context.Background()

	// Act
	f, err := New(ctx, ts.URL+"/")

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestNew_WithInvalidRobotsTxt(t *testing.T) {
	// Arrange
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "invalid robots.txt content\ngarbage\n")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body></body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()

	// Act
	f, err := New(ctx, ts.URL+"/")

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestNew_WithValidRobotsTxt(t *testing.T) {
	// Arrange
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `User-agent: *
Disallow: /private`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body></body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()

	// Act
	f, err := New(ctx, ts.URL+"/")

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, f)
	assert.False(t, f.IsAllowed(ts.URL+"/private"))
	assert.True(t, f.IsAllowed(ts.URL+"/public"))
}

func TestFetch_WithInvalidHTML(t *testing.T) {
	// Arrange
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `not valid html at all ;;;; <<<>>>`)
	}))
	defer ts.Close()

	ctx := context.Background()
	f, err := New(ctx, ts.URL+"/")
	require.NoError(t, err)

	// Act
	links, err := f.Fetch(ts.URL + "/")

	// Assert - should parse gracefully and return no links
	require.NoError(t, err)
	assert.Empty(t, links)
}
