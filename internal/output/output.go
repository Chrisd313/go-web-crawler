package output

import "github.com/chrisd313/web-crawler/internal/crawler"

// Reporter prints a PageResult.
type Reporter interface {
	Report(crawler.PageResult)
}
