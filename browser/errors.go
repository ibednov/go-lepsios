package browser

import "errors"

var (
	ErrClosed            = errors.New("browser client is closed")
	ErrEmptyPageURL      = errors.New("page url is empty")
	ErrEmptyRequestURL   = errors.New("request url is empty")
	ErrEmptyGlob         = errors.New("response url glob is empty")
	ErrHostNotAllowed    = errors.New("url host is not allowed")
	ErrUnauthorized      = errors.New("browser sidecar unauthorized")
	ErrRemoteUnavailable = errors.New("browser sidecar unavailable")
	ErrNotInstalled      = errors.New("playwright driver/chromium missing; run: go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium")
)
