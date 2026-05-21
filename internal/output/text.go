package output

import (
	"fmt"
	"io"

	"github.com/chrisd313/web-crawler/internal/crawler"
)

// TextReporter prints results in human-readable format.
type TextReporter struct {
	w io.Writer
}

// NewText returns a TextReporter writing to w.
func NewText(w io.Writer) *TextReporter {
	return &TextReporter{w: w}
}

func (r *TextReporter) Report(result crawler.PageResult) {
	fmt.Fprintf(r.w, "URL: %s\nLinks:\n", result.URL)
	for _, link := range result.Links {
		fmt.Fprintf(r.w, "  - %s\n", link)
	}
	fmt.Fprintln(r.w)
}
