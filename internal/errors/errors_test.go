package errors

import (
	"errors"
	"testing"
)

func TestAppError(t *testing.T) {
	t.Run("New", func(t *testing.T) {
		err := New(ErrorTypeValidation, "validation error")
		if err.Type != ErrorTypeValidation {
			t.Errorf("Type = %v, want %v", err.Type, ErrorTypeValidation)
		}
		if err.Message != "validation error" {
			t.Errorf("Message = %v, want %v", err.Message, "validation error")
		}
		if err.Error() != "validation: validation error" {
			t.Errorf("Error() = %v", err.Error())
		}
	})

	t.Run("Wrap", func(t *testing.T) {
		baseErr := errors.New("base error")
		err := Wrap(baseErr, ErrorTypeInternal, "internal error")

		if err.Type != ErrorTypeInternal {
			t.Errorf("Type = %v, want %v", err.Type, ErrorTypeInternal)
		}
		if err.Err != baseErr {
			t.Errorf("Err = %v, want %v", err.Err, baseErr)
		}
		if err.Error() != "internal: internal error: base error" {
			t.Errorf("Error() = %v", err.Error())
		}
		if errors.Unwrap(err) != baseErr {
			t.Errorf("Unwrap() = %v, want %v", errors.Unwrap(err), baseErr)
		}
	})

	t.Run("WithContext", func(t *testing.T) {
		err := New(ErrorTypeNotFound, "not found").WithContext("id", "123")
		if val, ok := err.Context["id"]; !ok || val != "123" {
			t.Errorf("Context id = %v, want 123", val)
		}
	})

	t.Run("Is", func(t *testing.T) {
		err := New(ErrorTypeValidation, "invalid")
		if !Is(err, ErrorTypeValidation) {
			t.Error("Is(ErrorTypeValidation) should be true")
		}
		if Is(err, ErrorTypeNotFound) {
			t.Error("Is(ErrorTypeNotFound) should be false")
		}

		stdErr := errors.New("standard error")
		if Is(stdErr, ErrorTypeValidation) {
			t.Error("Is(stdErr, ErrorTypeValidation) should be false")
		}
	})

	t.Run("Constructors", func(t *testing.T) {
		var tests = []struct {
			name string
			err  *AppError
			typ  ErrorType
		}{
			{"ValidationError", ValidationError("msg"), ErrorTypeValidation},
			{"NotFoundError", NotFoundError("msg"), ErrorTypeNotFound},
			{"InternalError", InternalError("msg"), ErrorTypeInternal},
			{"ExternalError", ExternalError("msg", nil), ErrorTypeExternal},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.err.Type != tt.typ {
					t.Errorf("Type = %v, want %v", tt.err.Type, tt.typ)
				}
			})
		}
	})
}
