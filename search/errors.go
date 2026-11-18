package search

import (
	"errors"
	"fmt"
)

var ErrNotConfigured = errors.New("search engine is not configured")

type ConfigError struct {
	Field string
	Cause error
}

func (e *ConfigError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("search config %s: %v", e.Field, e.Cause)
	}
	return fmt.Sprintf("search config %s is invalid", e.Field)
}

func (e *ConfigError) Unwrap() error { return e.Cause }

type HTTPError struct{ StatusCode int }

func (e *HTTPError) Error() string {
	return fmt.Sprintf("search request returned HTTP %d", e.StatusCode)
}

type TaskError struct {
	ID     int64
	Status string
	Cause  string
}

func (e *TaskError) Error() string {
	if e.Cause != "" {
		return fmt.Sprintf("search task %d %s: %s", e.ID, e.Status, e.Cause)
	}
	return fmt.Sprintf("search task %d %s", e.ID, e.Status)
}
