package browser

import "context"

// JSONCapturer opens a page and returns the first response body whose URL
// matches glob (Playwright syntax, for example `**/cards/v4/detail**`).
type JSONCapturer interface {
	CaptureJSON(ctx context.Context, pageURL, urlGlob string) ([]byte, error)
}
