package indexing

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/errors"
	"github.com/Guru2308/rag-code/internal/logger"
)

// Parser extracts semantic information from code files
type Parser interface {
	Parse(ctx context.Context, filePath string) ([]*domain.CodeChunk, error)
}

// GoParser parses Go source files using AST
type GoParser struct {
	fset *token.FileSet
}

// NewGoParser creates a new Go parser
func NewGoParser() *GoParser {
	return &GoParser{
		fset: token.NewFileSet(),
	}
}

// Parse extracts functions, types, and other declarations from a Go file
func (p *GoParser) Parse(ctx context.Context, filePath string) ([]*domain.CodeChunk, error) {
	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeExternal, "failed to read file")
	}

	// Parse AST
	file, err := parser.ParseFile(p.fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeExternal, "failed to parse Go file")
	}

	chunks := make([]*domain.CodeChunk, 0)

	// Extract package-level declarations
	for _, decl := range file.Decls {
		chunk := p.extractDeclaration(filePath, string(content), decl)
		if chunk != nil {
			chunks = append(chunks, chunk)
		}
	}

	logger.Debug("Parsed file",
		"path", filePath,
		"chunks", len(chunks),
	)

	return chunks, nil
}

// extractDeclaration extracts a code chunk from an AST declaration
func (p *GoParser) extractDeclaration(filePath, content string, decl ast.Decl) *domain.CodeChunk {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return p.extractFunction(filePath, content, d)
	case *ast.GenDecl:
		return p.extractGenDecl(filePath, content, d)
	default:
		return nil
	}
}

// extractFunction extracts a function declaration
func (p *GoParser) extractFunction(filePath, content string, fn *ast.FuncDecl) *domain.CodeChunk {
	start := p.fset.Position(fn.Pos())
	end := p.fset.Position(fn.End())

	// Extract the function source code
	lines := strings.Split(content, "\n")
	funcContent := strings.Join(lines[start.Line-1:end.Line], "\n")

	chunk := &domain.CodeChunk{
		FilePath:  filePath,
		Language:  "go",
		Content:   funcContent,
		ChunkType: domain.ChunkTypeFunction,
		StartLine: start.Line,
		EndLine:   end.Line,
		Metadata: map[string]string{
			"name": fn.Name.Name,
		},
	}

	// If it's a method, record the receiver type
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		chunk.ChunkType = domain.ChunkTypeMethod
		if recv := fn.Recv.List[0].Type; recv != nil {
			if starExpr, ok := recv.(*ast.StarExpr); ok {
				if ident, ok := starExpr.X.(*ast.Ident); ok {
					chunk.Metadata["receiver"] = ident.Name
				}
			} else if ident, ok := recv.(*ast.Ident); ok {
				chunk.Metadata["receiver"] = ident.Name
			}
		}
	}

	// Extract function calls within this function
	calls := p.extractFunctionCalls(fn)
	if len(calls) > 0 {
		chunk.Metadata["calls"] = strings.Join(calls, ",")
	}

	return chunk
}

// extractGenDecl extracts general declarations (types, constants, variables)
func (p *GoParser) extractGenDecl(filePath, content string, gen *ast.GenDecl) *domain.CodeChunk {
	// For type declarations with multiple specs, we'll create one chunk with all types
	// This keeps related type declarations together
	start := p.fset.Position(gen.Pos())
	end := p.fset.Position(gen.End())

	lines := strings.Split(content, "\n")
	declContent := strings.Join(lines[start.Line-1:end.Line], "\n")

	var chunkType domain.ChunkType
	metadata := make(map[string]string)

	switch gen.Tok {
	case token.TYPE:
		chunkType = domain.ChunkTypeClass // Types are similar to classes
		// Extract all type names in this declaration block
		var typeNames []string
		for _, spec := range gen.Specs {
			if typeSpec, ok := spec.(*ast.TypeSpec); ok {
				typeNames = append(typeNames, typeSpec.Name.Name)
			}
		}
		if len(typeNames) > 0 {
			metadata["types"] = strings.Join(typeNames, ",")
			// Use first type name as the primary name
			metadata["name"] = typeNames[0]
		}
	case token.IMPORT:
		chunkType = domain.ChunkTypeImport
		// Extract import paths
		imports := p.extractImports(gen)
		if len(imports) > 0 {
			metadata["imports"] = strings.Join(imports, ",")
		}
	default:
		chunkType = domain.ChunkTypeOther
	}

	return &domain.CodeChunk{
		FilePath:  filePath,
		Language:  "go",
		Content:   declContent,
		ChunkType: chunkType,
		StartLine: start.Line,
		EndLine:   end.Line,
		Metadata:  metadata,
	}
}

// extractImports extracts import paths from an import declaration
func (p *GoParser) extractImports(gen *ast.GenDecl) []string {
	var imports []string
	for _, spec := range gen.Specs {
		if importSpec, ok := spec.(*ast.ImportSpec); ok {
			// Remove quotes from import path
			importPath := strings.Trim(importSpec.Path.Value, "\"")
			imports = append(imports, importPath)
		}
	}
	return imports
}

// extractFunctionCalls extracts function calls from a function body
func (p *GoParser) extractFunctionCalls(fn *ast.FuncDecl) []string {
	if fn.Body == nil {
		return nil
	}

	calls := make(map[string]bool)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			var funcName string
			switch fun := callExpr.Fun.(type) {
			case *ast.Ident:
				// Simple function call: foo()
				funcName = fun.Name
			case *ast.SelectorExpr:
				// Method call or package function: obj.Method() or pkg.Func()
				if ident, ok := fun.X.(*ast.Ident); ok {
					funcName = ident.Name + "." + fun.Sel.Name
				}
			}
			if funcName != "" {
				calls[funcName] = true
			}
		}
		return true
	})

	result := make([]string, 0, len(calls))
	for call := range calls {
		result = append(result, call)
	}
	return result
}

// LanguageDetector determines the programming language of a file based on its extension.
func LanguageDetector(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	languageMap := map[string]string{
		// Go
		".go": "go",

		// Python
		".py":  "python",
		".pyw": "python",
		".pyi": "python",

		// JavaScript
		".js":  "javascript",
		".mjs": "javascript",
		".cjs": "javascript",
		".jsx": "javascript",

		// TypeScript
		".ts":  "typescript",
		".tsx": "typescript",

		// Java
		".java": "java",

		// Kotlin
		".kt":  "kotlin",
		".kts": "kotlin",

		// Swift
		".swift": "swift",

		// Rust
		".rs": "rust",

		// C / C++
		".c":   "c",
		".h":   "c",
		".cpp": "cpp",
		".cc":  "cpp",
		".cxx": "cpp",
		".hpp": "cpp",
		".hxx": "cpp",

		// C#
		".cs": "csharp",

		// Scala
		".scala": "scala",
		".sc":    "scala",

		// Ruby
		".rb":   "ruby",
		".rake": "ruby",

		// PHP
		".php": "php",

		// Shell
		".sh":   "shell",
		".bash": "shell",
		".zsh":  "shell",
		".fish": "shell",

		// Markdown / docs
		".md":  "markdown",
		".mdx": "markdown",
		".rst": "markdown",
		".txt": "markdown",

		// Config / data
		".json": "config",
		".yaml": "config",
		".yml":  "config",
		".toml": "config",
		".ini":  "config",
		".env":  "config",

		// SQL
		".sql": "sql",

		// Web
		".html":   "web",
		".htm":    "web",
		".css":    "web",
		".scss":   "web",
		".sass":   "web",
		".less":   "web",
		".svelte": "web",
		".vue":    "web",
	}

	if lang, ok := languageMap[ext]; ok {
		return lang
	}
	return "unknown"
}
