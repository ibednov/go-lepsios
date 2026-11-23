package errors

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedSource    = errors.New("unsupported source")
	ErrTransient            = errors.New("transient crawl error")
	ErrBlocked              = errors.New("source blocked challenge or forbidden")
	ErrBadCurrency          = errors.New("unexpected currency")
	ErrEmptyOffers          = errors.New("no offers extracted")
	ErrSkippedProduct       = errors.New("product intentionally skipped")
	ErrSourceAlreadyRunning = errors.New("crawl already running for source")
)

// CrawlError is a typed sidecar-style error for crawl operations.
type CrawlError struct {
	Code   string // transient, blocked, parse, currency, unsupported
	Source string
	Err    error
}

func (e *CrawlError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("crawl error: code=%s source=%s", e.Code, e.Source)
	}
	return fmt.Sprintf("crawl error: code=%s source=%s err=%v", e.Code, e.Source, e.Err)
}

func (e *CrawlError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Transient(source string, err error) *CrawlError {
	return &CrawlError{Code: "transient", Source: source, Err: err}
}

func Blocked(source string, err error) *CrawlError {
	return &CrawlError{Code: "blocked", Source: source, Err: err}
}

func ParseFailed(source string, err error) *CrawlError {
	return &CrawlError{Code: "parse", Source: source, Err: err}
}

func SkippedProduct(source string, reason error) *CrawlError {
	return &CrawlError{Code: "skip", Source: source, Err: fmt.Errorf("%w: %v", ErrSkippedProduct, reason)}
}
