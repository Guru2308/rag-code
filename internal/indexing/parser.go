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
		// TODO: Extract receiver type name
	}

	return chunk
}

// extractGenDecl extracts general declarations (types, constants, variables)
func (p *GoParser) extractGenDecl(filePath, content string, gen *ast.GenDecl) *domain.CodeChunk {
	start := p.fset.Position(gen.Pos())
	end := p.fset.Position(gen.End())

	lines := strings.Split(content, "\n")
	declContent := strings.Join(lines[start.Line-1:end.Line], "\n")

	var chunkType domain.ChunkType
	switch gen.Tok {
	case token.TYPE:
		chunkType = domain.ChunkTypeClass // Types are similar to classes
	case token.IMPORT:
		chunkType = domain.ChunkTypeImport
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
		Metadata:  make(map[string]string),
	}
}

// LanguageDetector determines the programming language of a file
func LanguageDetector(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	languageMap := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".java": "java",
		".cpp":  "cpp",
		".c":    "c",
		".rs":   "rust",
	}

	if lang, ok := languageMap[ext]; ok {
		return lang
	}
	return "unknown"
}

// TODO: Implement parsers for other languages (Python, JavaScript, TypeScript)
// TODO: Extract import/dependency information
// TODO: Extract function signatures and documentation
// TODO: Handle syntax errors gracefully
