package indexing

import (
	"context"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/logger"
)

// genericLanguages are languages that have no semantic patterns — use GenericParser.
var genericLanguages = map[string]bool{
	"markdown": true,
	"config":   true,
	"sql":      true,
	"web":      true,
}

// MultiParser dispatches to the appropriate parser based on file language.
//
//   - .go            → GoParser  (full AST, extracts functions/types/methods)
//   - code languages → RegexParser (regex-based semantic extraction)
//   - config/md/sql  → GenericParser (fixed-size line windows)
type MultiParser struct {
	goParser      *GoParser
	regexParser   *RegexParser
	genericParser *GenericParser
}

// NewMultiParser creates a MultiParser with all sub-parsers initialized.
func NewMultiParser() *MultiParser {
	return &MultiParser{
		goParser:      NewGoParser(),
		regexParser:   NewRegexParser(),
		genericParser: NewGenericParser(),
	}
}

// Parse routes the file to the appropriate parser based on its detected language.
func (m *MultiParser) Parse(ctx context.Context, filePath string) ([]*domain.CodeChunk, error) {
	lang := LanguageDetector(filePath)

	switch {
	case lang == "go":
		logger.Debug("Routing to GoParser", "path", filePath)
		return m.goParser.Parse(ctx, filePath)

	case genericLanguages[lang]:
		logger.Debug("Routing to GenericParser", "path", filePath, "lang", lang)
		return m.genericParser.Parse(ctx, filePath)

	case lang != "unknown":
		logger.Debug("Routing to RegexParser", "path", filePath, "lang", lang)
		return m.regexParser.Parse(ctx, filePath)

	default:
		// unknown — skip
		return nil, nil
	}
}
