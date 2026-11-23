package productparse

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ibednov/go-lepsios/currency"
)

type RawParseResult struct {
	Title      string
	Price      *float64
	Currency   currency.Code
	ImageURL   *string
	PriceError string
	WB         []WBCall
}

type WBCall struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	URL    string `json:"url"`
	Status int    `json:"status,omitempty"`
	Body   string `json:"body,omitempty"`
	Error  string `json:"error,omitempty"`
}

type SidecarError struct {
	Code   string // invalid_input, not_found, ...
	Source string
	Err    error
}

func (e *SidecarError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("sidecar error: code=%s source=%s", e.Code, e.Source)
	}
	return fmt.Sprintf("sidecar error: code=%s source=%s err=%v", e.Code, e.Source, e.Err)
}

func (e *SidecarError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// APIErrorCode — код для API/логов (PARSER_*).
func (e *SidecarError) APIErrorCode() string {
	if e == nil {
		return ""
	}
	switch e.Code {
	case "unsupported_domain":
		return "PARSER_UNSUPPORTED_DOMAIN"
	case "invalid_input":
		return "PARSER_INVALID_URL"
	case "no_price":
		return "PARSER_PRICE_UNAVAILABLE"
	default:
		return "PARSER_PARSE_FAILED"
	}
}

// StrategyName из Source: parser.wildberries.parse_card → wildberries.
func (e *SidecarError) StrategyName() string {
	if e == nil {
		return ""
	}
	return strategyNameFromSource(e.Source)
}

func strategyNameFromSource(source string) string {
	parts := strings.Split(source, ".")
	if len(parts) >= 2 && parts[0] == "parser" {
		return parts[1]
	}
	return ""
}

// ParseErrorCode для любой ошибки парсера (sidecar, timeout, …).
func ParseErrorCode(err error) string {
	if err == nil {
		return ""
	}

	var sidecarErr *SidecarError
	if errors.As(err, &sidecarErr) {
		return sidecarErr.APIErrorCode()
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "PARSER_TIMEOUT"
	}

	return "PARSER_INTERNAL_ERROR"
}
