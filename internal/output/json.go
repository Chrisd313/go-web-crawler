package output

import (
	"encoding/json"
	"io"
	"log"

	"github.com/chrisd313/web-crawler/internal/crawler"
)

// JSONReporter prints results as newline-delimited JSON.
type JSONReporter struct {
	enc *json.Encoder
}

// NewJSON returns a JSONReporter writing to w.
func NewJSON(w io.Writer) *JSONReporter {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &JSONReporter{enc: enc}
}

func (r *JSONReporter) Report(result crawler.PageResult) {
	if err := r.enc.Encode(result); err != nil {
		log.Printf("output: encode: %v", err)
	}
}
