package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SuccessBody is the standard success JSON envelope.
type SuccessBody struct {
	Data    any    `json:"data"`
	Message string `json:"message,omitempty"`
}

// OK writes 200 with data.
func OK(c *gin.Context, data any) {
	OKWithMessage(c, data, "")
}

// OKWithMessage writes 200 with data and message.
func OKWithMessage(c *gin.Context, data any, message string) {
	c.JSON(http.StatusOK, SuccessBody{Data: data, Message: message})
}

// Created writes 201 with data.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, SuccessBody{Data: data})
}

// CreatedWithMessage writes 201 with data and message.
func CreatedWithMessage(c *gin.Context, data any, message string) {
	c.JSON(http.StatusCreated, SuccessBody{Data: data, Message: message})
}

// NoContent writes 204.
func NoContent(c *gin.Context) {
	c.AbortWithStatus(http.StatusNoContent)
}
