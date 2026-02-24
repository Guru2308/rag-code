package indexing

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
)

func TestParser_ExtractImports(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

import (
	"fmt"
	"strings"
	"github.com/example/pkg"
)

func main() {
	fmt.Println("hello")
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser()
	chunks, err := parser.Parse(context.Background(), testFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Find the import chunk
	var importChunk *domain.CodeChunk
	for _, chunk := range chunks {
		if chunk.ChunkType == domain.ChunkTypeImport {
			importChunk = chunk
			break
		}
	}

	if importChunk == nil {
		t.Fatal("Expected import chunk to be found")
	}

	imports := importChunk.Metadata["imports"]
	if imports == "" {
		t.Fatal("Expected imports metadata to be populated")
	}

	importList := strings.Split(imports, ",")
	if len(importList) != 3 {
		t.Errorf("Expected 3 imports, got %d: %v", len(importList), importList)
	}

	// Check that imports contain expected values
	expectedImports := map[string]bool{
		"fmt":                    true,
		"strings":                true,
		"github.com/example/pkg": true,
	}

	for _, imp := range importList {
		imp = strings.TrimSpace(imp)
		if !expectedImports[imp] {
			t.Errorf("Unexpected import: %s", imp)
		}
	}
}

func TestParser_ExtractFunctionCalls(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

import "fmt"

func helper() {
	fmt.Println("helper")
}

func main() {
	helper()
	fmt.Println("main")
	result := process()
}

func process() string {
	return "done"
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser()
	chunks, err := parser.Parse(context.Background(), testFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Find the main function chunk
	var mainChunk *domain.CodeChunk
	for _, chunk := range chunks {
		if chunk.ChunkType == domain.ChunkTypeFunction && chunk.Metadata["name"] == "main" {
			mainChunk = chunk
			break
		}
	}

	if mainChunk == nil {
		t.Fatal("Expected main function chunk to be found")
	}

	calls := mainChunk.Metadata["calls"]
	if calls == "" {
		t.Fatal("Expected calls metadata to be populated")
	}

	callList := strings.Split(calls, ",")
	if len(callList) < 2 {
		t.Errorf("Expected at least 2 function calls, got %d: %v", len(callList), callList)
	}

	// Verify expected calls are present
	callMap := make(map[string]bool)
	for _, call := range callList {
		callMap[strings.TrimSpace(call)] = true
	}

	expectedCalls := []string{"helper", "fmt.Println", "process"}
	for _, expected := range expectedCalls {
		found := false
		for call := range callMap {
			if call == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find call to %s", expected)
		}
	}
}

func TestParser_ExtractMethodReceiver(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

type MyStruct struct {
	value int
}

func (m *MyStruct) Method1() {
	m.value = 10
}

func (m MyStruct) Method2() int {
	return m.value
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser()
	chunks, err := parser.Parse(context.Background(), testFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Find method chunks
	methodCount := 0
	for _, chunk := range chunks {
		if chunk.ChunkType == domain.ChunkTypeMethod {
			methodCount++
			receiver := chunk.Metadata["receiver"]
			if receiver != "MyStruct" {
				t.Errorf("Expected receiver to be MyStruct, got %s", receiver)
			}
		}
	}

	if methodCount != 2 {
		t.Errorf("Expected 2 methods, got %d", methodCount)
	}
}

func TestParser_ExtractTypeNames(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	// Note: In Go AST, separate type declarations create separate GenDecl nodes
	// Grouping them in the same block would create one GenDecl with multiple specs
	code := `package main

type (
	User struct {
		Name string
		Age  int
	}
	Status int
)
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser()
	chunks, err := parser.Parse(context.Background(), testFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should have one type chunk with both types
	var typeChunk *domain.CodeChunk
	for _, chunk := range chunks {
		if chunk.ChunkType == domain.ChunkTypeClass {
			typeChunk = chunk
			break
		}
	}

	if typeChunk == nil {
		t.Fatal("Expected type chunk to be found")
	}

	types := typeChunk.Metadata["types"]
	if types == "" {
		t.Fatal("Expected types metadata to be populated")
	}

	// Should contain both User and Status
	if !strings.Contains(types, "User") {
		t.Errorf("Expected types to contain User, got: %s", types)
	}
	if !strings.Contains(types, "Status") {
		t.Errorf("Expected types to contain Status, got: %s", types)
	}
}

func TestLanguageDetector(t *testing.T) {
	tests := []struct {
		filePath string
		expected string
	}{
		{"/path/to/file.go", "go"},
		{"/path/to/file.py", "python"},
		{"/path/to/file.js", "javascript"},
		{"/path/to/file.ts", "typescript"},
		{"/path/to/file.java", "java"},
		{"/path/to/file.cpp", "cpp"},
		{"/path/to/file.rs", "rust"},
		{"/path/to/file.lua", "lua"},
		{"/path/to/file.dart", "dart"},
		{"/path/to/file.hs", "haskell"},
		{"/path/to/file.ex", "elixir"},
		{"/path/to/file.clj", "clojure"},
	}

	for _, tt := range tests {
		result := LanguageDetector(tt.filePath)
		if result != tt.expected {
			t.Errorf("LanguageDetector(%s) = %s, expected %s", tt.filePath, result, tt.expected)
		}
	}
}
