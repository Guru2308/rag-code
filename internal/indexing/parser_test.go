package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGoParser_Parse(t *testing.T) {
	p := NewGoParser()
	ctx := context.Background()

	// Create a temp go file
	tmpDir, _ := os.MkdirTemp("", "parser_test")
	defer os.RemoveAll(tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	content := `package test
import "fmt"
// MyFunc is a function
func MyFunc() {
	fmt.Println("hello")
}
type MyStruct struct {
	Field string
}
`
	os.WriteFile(goFile, []byte(content), 0644)

	chunks, err := p.Parse(ctx, goFile)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should extract: import, func, type
	if len(chunks) < 3 {
		t.Errorf("Expected at least 3 chunks, got %d", len(chunks))
	}

	foundFunc := false
	foundStruct := false
	for _, c := range chunks {
		if c.Metadata["name"] == "MyFunc" {
			foundFunc = true
		}
		if c.ChunkType == "class" { // GoParser maps token.TYPE to ChunkTypeClass
			foundStruct = true
		}
	}

	if !foundFunc {
		t.Error("Did not find MyFunc")
	}
	if !foundStruct {
		t.Error("Did not find MyStruct")
	}
}

func TestLanguageDetector(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"README.md", "unknown"},
	}

	for _, tt := range tests {
		if got := LanguageDetector(tt.filename); got != tt.want {
			t.Errorf("LanguageDetector(%q) = %v, want %v", tt.filename, got, tt.want)
		}
	}
}
