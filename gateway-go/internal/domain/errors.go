package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors for common repository / service outcomes.
var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrInvalidInput  = errors.New("invalid input")
	ErrUnavailable   = errors.New("unavailable")
	ErrTimeout       = errors.New("timeout")
	ErrCircuitOpen   = errors.New("circuit open")
)

// AppError is the unified application error returned across layers.
// Handlers return it; middleware maps Code to HTTP status.
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError builds an AppError with an optional wrapped cause.
func NewAppError(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

// HTTP-oriented constructors (status codes as Code).
func BadRequest(message string, err error) *AppError {
	return NewAppError(400, message, err)
}

func NotFound(message string, err error) *AppError {
	return NewAppError(404, message, err)
}

func Conflict(message string, err error) *AppError {
	return NewAppError(409, message, err)
}

func Internal(message string, err error) *AppError {
	return NewAppError(500, message, err)
}

func Unavailable(message string, err error) *AppError {
	return NewAppError(503, message, err)
}
