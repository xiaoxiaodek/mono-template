package apperrors

import "fmt"

type Code string

const (
	CodeValidationError Code = "VALIDATION_ERROR"
	CodeUnauthorized    Code = "UNAUTHORIZED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeRateLimited     Code = "RATE_LIMITED"
	CodeTooLarge        Code = "TOO_LARGE"
	CodeDependencyError Code = "DEPENDENCY_ERROR"
	CodeInternalError   Code = "INTERNAL_ERROR"
)

type Error struct {
	Code    Code
	Message string
	Cause   error
}

func New(code Code, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}

	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func (e *Error) PublicMessage() string {
	if e.Message == "" {
		return "request failed"
	}
	return e.Message
}
