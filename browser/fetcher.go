package browser

import "context"

// PageSnapshot is the body of fetch(requestURL) after warmup plus that page HTML.
type PageSnapshot struct {
	Body   []byte
	HTML   []byte
	Status int
}

// PageFetcher opens warmup URL (antibot JS) and fetch(requestURL) in page context.
type PageFetcher interface {
	FetchInPage(ctx context.Context, warmupURL, requestURL string) (PageSnapshot, error)
}
