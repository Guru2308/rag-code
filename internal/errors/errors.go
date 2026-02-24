package errors

import (
	"errors"
	"fmt"
)

// ErrorType represents the category of error
type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "validation"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeInternal     ErrorType = "internal"
	ErrorTypeExternal     ErrorType = "external"
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	ErrorTypeConflict     ErrorType = "conflict"
)

// AppError represents an application error with additional context
type AppError struct {
	Type    ErrorType
	Message string
	Err     error
	Context map[string]any
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap implements the errors.Unwrap interface
func (e *AppError) Unwrap() error {
	return e.Err
}

// New creates a new AppError
func New(errType ErrorType, message string) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Context: make(map[string]any),
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, errType ErrorType, message string) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Err:     err,
		Context: make(map[string]any),
	}
}

// WithContext adds context to the error
func (e *AppError) WithContext(key string, value any) *AppError {
	e.Context[key] = value
	return e
}

// Is checks if the error is of a specific type
func Is(err error, errType ErrorType) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == errType
	}
	return false
}

// Common error constructors
func ValidationError(message string) *AppError {
	return New(ErrorTypeValidation, message)
}

func NotFoundError(message string) *AppError {
	return New(ErrorTypeNotFound, message)
}

func InternalError(message string) *AppError {
	return New(ErrorTypeInternal, message)
}

func ExternalError(message string, err error) *AppError {
	return Wrap(err, ErrorTypeExternal, message)
}
