package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chrisd313/web-crawler/internal/crawler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testResult = crawler.PageResult{
	URL:   "https://crawlme.monzo.com/",
	Links: []string{"https://crawlme.monzo.com/about", "https://crawlme.monzo.com/contact"},
}

func TestTextReporter_ReportsURL(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	r := NewText(&buf)

	// Act
	r.Report(testResult)

	// Assert
	out := buf.String()
	assert.Contains(t, out, "URL: https://crawlme.monzo.com/")
	assert.Contains(t, out, "- https://crawlme.monzo.com/about")
	assert.Contains(t, out, "- https://crawlme.monzo.com/contact")
}

func TestTextReporter_ReportsEmptyLinks(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	r := NewText(&buf)
	result := crawler.PageResult{URL: "https://example.com/", Links: nil}

	// Act
	r.Report(result)

	// Assert
	out := buf.String()
	assert.Contains(t, out, "URL: https://example.com/")
	assert.NotContains(t, out, "  - ")
}

func TestTextReporter_FormatsWithNewlines(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	r := NewText(&buf)

	// Act
	r.Report(testResult)

	// Assert
	out := buf.String()
	assert.Contains(t, out, "Links:\n")
}

func TestJSONReporter_ReportsURLAndLinks(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	r := NewJSON(&buf)

	// Act
	r.Report(testResult)

	// Assert
	out := strings.TrimSpace(buf.String())
	want := `{"url":"https://crawlme.monzo.com/","links":["https://crawlme.monzo.com/about","https://crawlme.monzo.com/contact"]}`
	assert.Equal(t, want, out)
}

func TestJSONReporter_ReportsNullLinksWhenEmpty(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	r := NewJSON(&buf)
	result := crawler.PageResult{URL: "https://example.com/", Links: nil}

	// Act
	r.Report(result)

	// Assert
	out := strings.TrimSpace(buf.String())
	want := `{"url":"https://example.com/","links":null}`
	assert.Equal(t, want, out)
}

func TestJSONReporter_OutputIsNewlineDelimited(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	r := NewJSON(&buf)

	// Act
	r.Report(testResult)

	// Assert
	out := buf.String()
	assert.True(t, strings.HasSuffix(out, "\n"), "JSON output should end with newline")
}

func TestTextReporter_HandlesSingleLink(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	r := NewText(&buf)
	result := crawler.PageResult{
		URL:   "https://example.com/",
		Links: []string{"https://example.com/other"},
	}

	// Act
	r.Report(result)

	// Assert
	out := buf.String()
	assert.Contains(t, out, "- https://example.com/other")
}

func TestJSONReporter_HandlesSingleLink(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	r := NewJSON(&buf)
	result := crawler.PageResult{
		URL:   "https://example.com/",
		Links: []string{"https://example.com/other"},
	}

	// Act
	r.Report(result)

	// Assert
	out := strings.TrimSpace(buf.String())
	require.Contains(t, out, `"links":["https://example.com/other"]`)
}
