// Package envelope is the Laravel/md-tests JSON response shape:
//
//	{ "status", "data", "errors", "meta" }
//
// Prefer github.com/ibednov/go-lepsios/httpx/response for the simpler
// { data, meta?, message? } envelope used by newer services.
package envelope

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/httpx/response"
)

const traceHeader = "X-Trace-ID"

// Status is the API response status field.
type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

// ErrorDetail matches Laravel/Rust error item.
type ErrorDetail struct {
	Message string   `json:"message"`
	Code    string   `json:"code"`
	Path    []string `json:"path,omitempty"`
}

// Meta is response metadata.
type Meta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
	Locale    string `json:"locale"`
}

// Envelope is the standard API response wrapper.
type Envelope[T any] struct {
	Status Status        `json:"status"`
	Data   *T            `json:"data"`
	Errors []ErrorDetail `json:"errors"`
	Meta   Meta          `json:"meta"`
}

// Pagination re-exports shared page meta for callers that only import envelope.
type Pagination = response.Pagination

// NewErrorDetail creates an error detail.
func NewErrorDetail(message, code string) ErrorDetail {
	return ErrorDetail{Message: message, Code: code}
}

// MetaFromContext builds Meta from gin request context.
func MetaFromContext(c *gin.Context) Meta {
	requestID, _ := c.Get(traceHeader)
	id, _ := requestID.(string)
	if id == "" {
		id = c.GetHeader(traceHeader)
	}

	locale := c.GetString("locale")
	if locale == "" {
		locale = "ru"
	}

	return Meta{
		RequestID: id,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Locale:    locale,
	}
}

// Success returns a success envelope.
func Success[T any](c *gin.Context, data T) Envelope[T] {
	return Envelope[T]{
		Status: StatusSuccess,
		Data:   &data,
		Errors: []ErrorDetail{},
		Meta:   MetaFromContext(c),
	}
}

// SuccessEmpty returns a success envelope without data.
func SuccessEmpty(c *gin.Context) Envelope[struct{}] {
	return Envelope[struct{}]{
		Status: StatusSuccess,
		Data:   nil,
		Errors: []ErrorDetail{},
		Meta:   MetaFromContext(c),
	}
}

// Error returns an error envelope.
func Error(c *gin.Context) Envelope[struct{}] {
	return Envelope[struct{}]{
		Status: StatusError,
		Data:   nil,
		Errors: []ErrorDetail{},
		Meta:   MetaFromContext(c),
	}
}

// WithErrors attaches errors to an error envelope.
func WithErrors(c *gin.Context, errors ...ErrorDetail) Envelope[struct{}] {
	resp := Error(c)
	resp.Errors = errors
	return resp
}

// NormalizePage re-exports response.NormalizePage.
func NormalizePage(page, perPage int) (int, int) {
	return response.NormalizePage(page, perPage)
}

// NewPagination re-exports response.NewPagination.
func NewPagination(page, perPage, totalCount int) Pagination {
	return response.NewPagination(page, perPage, totalCount)
}

// SuccessWithPagination returns a success payload with flat + nested pagination meta.
func SuccessWithPagination[T any](c *gin.Context, data T, p Pagination) gin.H {
	resp := Success(c, data)
	return gin.H{
		"status": resp.Status,
		"data":   data,
		"errors": resp.Errors,
		"meta": gin.H{
			"request_id": resp.Meta.RequestID,
			"timestamp":  resp.Meta.Timestamp,
			"locale":     resp.Meta.Locale,
			"page":       p.Page,
			"perPage":    p.PerPage,
			"totalCount": p.TotalCount,
			"totalPages": p.TotalPages,
			"pagination": gin.H{
				"page":       p.Page,
				"perPage":    p.PerPage,
				"totalCount": p.TotalCount,
				"totalPages": p.TotalPages,
			},
		},
	}
}
