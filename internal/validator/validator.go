package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Guru2308/rag-code/internal/errors"
)

// ValidateFilePath validates that a file path exists and is accessible
func ValidateFilePath(path string) error {
	if path == "" {
		return errors.ValidationError("path cannot be empty")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeValidation, "invalid path")
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.NotFoundError(fmt.Sprintf("path does not exist: %s", absPath))
		}
		return errors.Wrap(err, errors.ErrorTypeValidation, "cannot access path")
	}

	if !info.IsDir() {
		return errors.ValidationError(fmt.Sprintf("path is not a directory: %s", absPath))
	}

	return nil
}

// ValidateNonEmpty validates that a string is not empty
func ValidateNonEmpty(value, fieldName string) error {
	if strings.TrimSpace(value) == "" {
		return errors.ValidationError(fmt.Sprintf("%s cannot be empty", fieldName))
	}
	return nil
}

// ValidateRange validates that a value is within a range
func ValidateRange(value, min, max int, fieldName string) error {
	if value < min || value > max {
		return errors.ValidationError(
			fmt.Sprintf("%s must be between %d and %d, got %d", fieldName, min, max, value),
		)
	}
	return nil
}

// ValidateOneOf validates that a value is one of the allowed values
func ValidateOneOf(value string, allowed []string, fieldName string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return errors.ValidationError(
		fmt.Sprintf("%s must be one of %v, got %s", fieldName, allowed, value),
	)
}
