package response

import (
	"context"
	"net/http"

	"github.com/ibednov/go-lepsios/i18n"
	"github.com/ibednov/go-lepsios/log"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// ErrorBody is the standard error JSON envelope.
type ErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Error writes a structured error response.
func Error(c *gin.Context, httpCode int, errorCode, message string) {
	c.JSON(httpCode, ErrorBody{
		Error:   errorCode,
		Message: message,
		Code:    httpCode,
	})
}

// Unauthorized writes 401.
func Unauthorized(c *gin.Context, errorCode, message string) {
	Error(c, http.StatusUnauthorized, errorCode, message)
}

// BadRequest writes 400.
func BadRequest(c *gin.Context, errorCode, message string) {
	Error(c, http.StatusBadRequest, errorCode, message)
}

// NotFound writes 404.
func NotFound(c *gin.Context, errorCode, message string) {
	Error(c, http.StatusNotFound, errorCode, message)
}

// Conflict writes 409.
func Conflict(c *gin.Context, errorCode, message string) {
	Error(c, http.StatusConflict, errorCode, message)
}

// Internal writes 500.
func Internal(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}

// ErrorMapper maps service errors to HTTP response fields.
type ErrorMapper func(err error) (httpCode int, errorCode, messageKey string, ok bool)

// Handle maps err via mapper and localizer.
func Handle(c *gin.Context, err error, mapper ErrorMapper, localizer *i18n.Localizer) {
	if err == nil {
		return
	}

	logger := logFromGin(c)

	httpCode, errorCode, messageKey, ok := mapper(err)
	if !ok {
		logger.Error().Err(err).Str("path", c.Request.URL.Path).Msg("http.unmapped_error")
		Internal(c, err.Error())
		return
	}

	message := messageKey
	if localizer != nil {
		message = localizer.T(messageKey)
	}

	if httpCode >= http.StatusInternalServerError {
		logger.Error().
			Str("error_code", errorCode).
			Str("message", message).
			Int("status", httpCode).
			Msg("http.error")
	} else if httpCode >= http.StatusBadRequest {
		logger.Warn().
			Str("error_code", errorCode).
			Str("message", message).
			Int("status", httpCode).
			Msg("http.error")
	}

	Error(c, httpCode, errorCode, message)
}

func logFromGin(c *gin.Context) zerolog.Logger {
	if c == nil || c.Request == nil {
		return log.FromContext(context.TODO())
	}
	return log.FromContext(c.Request.Context())
}
