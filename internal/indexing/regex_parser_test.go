package indexing

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
)

// ─────────────────────────────────────────────
// LanguageDetector extended tests
// ─────────────────────────────────────────────

func TestLanguageDetector_Extended(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		// Go
		{"main.go", "go"},
		// Python
		{"script.py", "python"},
		{"types.pyi", "python"},
		// JS / TS
		{"app.js", "javascript"},
		{"app.jsx", "javascript"},
		{"app.mjs", "javascript"},
		{"app.ts", "typescript"},
		{"app.tsx", "typescript"},
		// JVM
		{"Main.java", "java"},
		{"Main.kt", "kotlin"},
		{"Main.kts", "kotlin"},
		{"Main.scala", "scala"},
		// Systems
		{"main.rs", "rust"},
		{"main.c", "c"},
		{"main.h", "c"},
		{"main.cpp", "cpp"},
		{"main.cc", "cpp"},
		{"main.hpp", "cpp"},
		{"main.cs", "csharp"},
		// Scripting
		{"app.rb", "ruby"},
		{"app.php", "php"},
		{"run.sh", "shell"},
		{"run.bash", "shell"},
		// Mobile
		{"App.swift", "swift"},
		// Docs
		{"README.md", "markdown"},
		{"README.rst", "markdown"},
		{"notes.txt", "markdown"},
		// Config
		{"config.json", "config"},
		{"config.yaml", "config"},
		{"config.yml", "config"},
		{"config.toml", "config"},
		// SQL
		{"schema.sql", "sql"},
		// Web
		{"index.html", "web"},
		{"style.css", "web"},
		{"style.scss", "web"},
		{"App.vue", "web"},
		{"App.svelte", "web"},
		// Unknown
		{"binary.exe", "unknown"},
		{"archive.zip", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := LanguageDetector(tt.path)
			if got != tt.expected {
				t.Errorf("LanguageDetector(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

// ─────────────────────────────────────────────
// RegexParser tests
// ─────────────────────────────────────────────

func TestRegexParser_Python(t *testing.T) {
	code := `
import os

def hello(name):
    print(f"Hello {name}")

async def fetch(url):
    pass

class MyClass:
    def method(self):
        pass
`
	tmpFile := writeTempFile(t, "test.py", code)
	p := NewRegexParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) < 3 {
		t.Errorf("Expected at least 3 chunks (hello, fetch, MyClass), got %d", len(chunks))
	}
	assertContainsName(t, chunks, "hello")
	assertContainsName(t, chunks, "fetch")
	assertContainsName(t, chunks, "MyClass")
}

func TestRegexParser_JavaScript(t *testing.T) {
	code := `
function greet(name) {
    return "Hello " + name;
}

class Animal {
    constructor(name) {
        this.name = name;
    }
}

const add = (a, b) => a + b;

export async function fetchData(url) {
    return fetch(url);
}
`
	tmpFile := writeTempFile(t, "test.js", code)
	p := NewRegexParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks, got %d", len(chunks))
	}
}

func TestRegexParser_TypeScript(t *testing.T) {
	code := `
export interface User {
    id: number;
    name: string;
}

export class UserService {
    getUser(id: number): User {
        return { id, name: "test" };
    }
}

export async function fetchUser(id: number): Promise<User> {
    return { id, name: "fetched" };
}
`
	tmpFile := writeTempFile(t, "test.ts", code)
	p := NewRegexParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks, got %d", len(chunks))
	}
}

func TestRegexParser_Rust(t *testing.T) {
	code := `
pub struct Point {
    x: f64,
    y: f64,
}

impl Point {
    pub fn new(x: f64, y: f64) -> Self {
        Point { x, y }
    }
}

pub fn distance(a: &Point, b: &Point) -> f64 {
    ((a.x - b.x).powi(2) + (a.y - b.y).powi(2)).sqrt()
}
`
	tmpFile := writeTempFile(t, "test.rs", code)
	p := NewRegexParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks (struct, impl, fn), got %d", len(chunks))
	}
}

func TestRegexParser_Ruby(t *testing.T) {
	code := `
module Greeter
  class Hello
    def greet(name)
      puts "Hello #{name}"
    end
  end
end
`
	tmpFile := writeTempFile(t, "test.rb", code)
	p := NewRegexParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks (module, class, def), got %d", len(chunks))
	}
}

func TestRegexParser_Shell(t *testing.T) {
	code := `#!/bin/bash

function setup() {
    echo "Setting up..."
}

cleanup() {
    echo "Cleaning up..."
}

setup
cleanup
`
	tmpFile := writeTempFile(t, "test.sh", code)
	p := NewRegexParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) < 1 {
		t.Errorf("Expected at least 1 chunk, got %d", len(chunks))
	}
}

func TestRegexParser_FallbackToGeneric(t *testing.T) {
	// A Python file with no matching patterns should fall back to generic chunking
	code := strings.Repeat("x = 1\n", 60) // 60 lines, no functions/classes
	tmpFile := writeTempFile(t, "test.py", code)
	p := NewRegexParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Should produce at least 1 generic chunk
	if len(chunks) < 1 {
		t.Errorf("Expected at least 1 generic chunk, got %d", len(chunks))
	}
}

// ─────────────────────────────────────────────
// GenericParser tests
// ─────────────────────────────────────────────

func TestGenericParser_Markdown(t *testing.T) {
	content := strings.Repeat("# Section\nSome text here.\n\n", 30) // ~90 lines
	tmpFile := writeTempFile(t, "README.md", content)
	p := NewGenericParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks for 90-line file, got %d", len(chunks))
	}
}

func TestGenericParser_JSON(t *testing.T) {
	content := `{"name": "test", "version": "1.0.0", "dependencies": {}}`
	tmpFile := writeTempFile(t, "package.json", content)
	p := NewGenericParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) < 1 {
		t.Errorf("Expected at least 1 chunk, got %d", len(chunks))
	}
}

func TestGenericParser_EmptyFile(t *testing.T) {
	tmpFile := writeTempFile(t, "empty.md", "")
	p := NewGenericParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Empty file should produce no chunks
	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty file, got %d", len(chunks))
	}
}

// ─────────────────────────────────────────────
// MultiParser tests
// ─────────────────────────────────────────────

func TestMultiParser_RoutesGoToGoParser(t *testing.T) {
	code := `package main

func Hello() string {
	return "hello"
}
`
	tmpFile := writeTempFile(t, "main.go", code)
	p := NewMultiParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("Expected chunks from GoParser for .go file")
	}
	// GoParser sets language = "go"
	for _, c := range chunks {
		if c.Language != "go" {
			t.Errorf("Expected language 'go', got %q", c.Language)
		}
	}
}

func TestMultiParser_RoutesPythonToRegexParser(t *testing.T) {
	code := `def greet():
    print("hi")
`
	tmpFile := writeTempFile(t, "greet.py", code)
	p := NewMultiParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("Expected chunks from RegexParser for .py file")
	}
}

func TestMultiParser_RoutesMarkdownToGenericParser(t *testing.T) {
	content := strings.Repeat("Some markdown content.\n", 10)
	tmpFile := writeTempFile(t, "notes.md", content)
	p := NewMultiParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("Expected chunks from GenericParser for .md file")
	}
}

func TestMultiParser_SkipsUnknownExtensions(t *testing.T) {
	tmpFile := writeTempFile(t, "binary.xyz", "some binary content")
	p := NewMultiParser()
	chunks, err := p.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for unknown extension, got %d", len(chunks))
	}
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	return path
}

func assertContainsName(t *testing.T, chunks []*domain.CodeChunk, name string) {
	t.Helper()
	found := false
	for _, chunk := range chunks {
		if chunk.Metadata["name"] == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find chunk with name %q, but didn't", name)
	}
}
