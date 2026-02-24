package validator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFilePath(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "validator_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid dir", tmpDir, false},
		{"empty path", "", true},
		{"non-existent path", filepath.Join(tmpDir, "non_existent"), true},
		{"file path (expect dir)", tmpFile, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNonEmpty(t *testing.T) {
	if err := ValidateNonEmpty("value", "field"); err != nil {
		t.Errorf("ValidateNonEmpty(value) error = %v", err)
	}
	if err := ValidateNonEmpty("", "field"); err == nil {
		t.Error("ValidateNonEmpty(empty) expected error")
	}
	if err := ValidateNonEmpty("  ", "field"); err == nil {
		t.Error("ValidateNonEmpty(spaces) expected error")
	}
}

func TestValidateRange(t *testing.T) {
	if err := ValidateRange(5, 1, 10, "field"); err != nil {
		t.Errorf("ValidateRange(5) error = %v", err)
	}
	if err := ValidateRange(0, 1, 10, "field"); err == nil {
		t.Error("ValidateRange(0) expected error")
	}
	if err := ValidateRange(11, 1, 10, "field"); err == nil {
		t.Error("ValidateRange(11) expected error")
	}
}

func TestValidateOneOf(t *testing.T) {
	allowed := []string{"a", "b", "c"}
	if err := ValidateOneOf("a", allowed, "field"); err != nil {
		t.Errorf("ValidateOneOf(a) error = %v", err)
	}
	if err := ValidateOneOf("d", allowed, "field"); err == nil {
		t.Error("ValidateOneOf(d) expected error")
	}
}
