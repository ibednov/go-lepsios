package files

import "fmt"

type Code string

const (
	CodeInvalidInput Code = "invalid_input"
	CodeNotFound     Code = "not_found"
	CodeAccessDenied Code = "access_denied"
	CodeUnavailable  Code = "unavailable"
	CodeInternal     Code = "internal"
)

type SidecarError struct {
	Code   Code
	Source string
	Err    error
}

const ErrorPrefix = "sidecar.files"

func (e *SidecarError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf(
			"%s code=%s source=%s",
			ErrorPrefix,
			e.Code,
			e.Source,
		)
	}
	return fmt.Sprintf(
		"%s code=%s source=%s error=%v",
		ErrorPrefix,
		e.Code,
		e.Source,
		e.Err,
	)
}

func (e *SidecarError) Unwrap() error {
	return e.Err
}

func WrapError(code Code, source string, err error) error {
	if err == nil {
		return nil
	}

	if code == "" {
		code = CodeInternal
	}

	return &SidecarError{
		Code:   code,
		Source: source,
		Err:    err,
	}
}
