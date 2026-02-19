package indexing

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/errors"
	"github.com/Guru2308/rag-code/internal/logger"
)

// languagePatterns holds regex patterns for extracting semantic blocks per language.
// Each pattern matches the START of a top-level declaration (function, class, etc.).
var languagePatterns = map[string][]*regexp.Regexp{
	"python": {
		regexp.MustCompile(`(?m)^(async\s+def\s+\w+|def\s+\w+|class\s+\w+)`),
	},
	"javascript": {
		regexp.MustCompile(`(?m)^(export\s+)?(async\s+)?function\s+\w+`),
		regexp.MustCompile(`(?m)^(export\s+)?(default\s+)?class\s+\w+`),
		regexp.MustCompile(`(?m)^(export\s+)?(const|let|var)\s+\w+\s*=\s*(async\s+)?\(`),
		regexp.MustCompile(`(?m)^(export\s+)?(const|let|var)\s+\w+\s*=\s*(async\s+)?function`),
	},
	"typescript": {
		regexp.MustCompile(`(?m)^(export\s+)?(async\s+)?function\s+\w+`),
		regexp.MustCompile(`(?m)^(export\s+)?(abstract\s+)?class\s+\w+`),
		regexp.MustCompile(`(?m)^(export\s+)?interface\s+\w+`),
		regexp.MustCompile(`(?m)^(export\s+)?type\s+\w+\s*=`),
		regexp.MustCompile(`(?m)^(export\s+)?(const|let|var)\s+\w+\s*=\s*(async\s+)?\(`),
	},
	"java": {
		regexp.MustCompile(`(?m)^\s*(public|private|protected|static|final|abstract|synchronized)[\w\s<>\[\]]*\s+\w+\s*\(`),
		regexp.MustCompile(`(?m)^\s*(public|private|protected)?\s*(abstract\s+)?class\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(public\s+)?interface\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(public\s+)?enum\s+\w+`),
	},
	"kotlin": {
		regexp.MustCompile(`(?m)^\s*(suspend\s+)?fun\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(data\s+|sealed\s+|abstract\s+|open\s+)?class\s+\w+`),
		regexp.MustCompile(`(?m)^\s*object\s+\w+`),
		regexp.MustCompile(`(?m)^\s*interface\s+\w+`),
	},
	"swift": {
		regexp.MustCompile(`(?m)^\s*(public|private|internal|open|fileprivate)?\s*(static\s+|class\s+)?(func)\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(public|private|internal|open)?\s*(final\s+)?class\s+\w+`),
		regexp.MustCompile(`(?m)^\s*struct\s+\w+`),
		regexp.MustCompile(`(?m)^\s*protocol\s+\w+`),
		regexp.MustCompile(`(?m)^\s*enum\s+\w+`),
	},
	"rust": {
		regexp.MustCompile(`(?m)^\s*(pub(\([\w:]+\))?\s+)?(async\s+)?fn\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(pub(\([\w:]+\))?\s+)?struct\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(pub(\([\w:]+\))?\s+)?enum\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(pub(\([\w:]+\))?\s+)?trait\s+\w+`),
		regexp.MustCompile(`(?m)^\s*impl(\s*<[^>]*>)?\s+\w+`),
	},
	"cpp": {
		regexp.MustCompile(`(?m)^[\w:*&<>\s]+\s+\w+\s*\([^;]*\)\s*(\{|$)`),
		regexp.MustCompile(`(?m)^\s*(class|struct)\s+\w+`),
		regexp.MustCompile(`(?m)^\s*namespace\s+\w+`),
	},
	"c": {
		regexp.MustCompile(`(?m)^[\w*\s]+\s+\w+\s*\([^;]*\)\s*\{`),
		regexp.MustCompile(`(?m)^\s*(struct|enum|union)\s+\w+`),
	},
	"csharp": {
		regexp.MustCompile(`(?m)^\s*(public|private|protected|internal|static|virtual|override|abstract|async)[\w\s<>\[\]]*\s+\w+\s*\(`),
		regexp.MustCompile(`(?m)^\s*(public|private|protected|internal)?\s*(abstract\s+|sealed\s+)?class\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(public\s+)?interface\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(public\s+)?enum\s+\w+`),
		regexp.MustCompile(`(?m)^\s*namespace\s+[\w.]+`),
	},
	"scala": {
		regexp.MustCompile(`(?m)^\s*(def)\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(case\s+|abstract\s+|sealed\s+)?class\s+\w+`),
		regexp.MustCompile(`(?m)^\s*object\s+\w+`),
		regexp.MustCompile(`(?m)^\s*trait\s+\w+`),
	},
	"ruby": {
		regexp.MustCompile(`(?m)^\s*def\s+\w+`),
		regexp.MustCompile(`(?m)^\s*class\s+\w+`),
		regexp.MustCompile(`(?m)^\s*module\s+\w+`),
	},
	"php": {
		regexp.MustCompile(`(?m)^\s*(public|private|protected|static)?\s*function\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(abstract\s+|final\s+)?class\s+\w+`),
		regexp.MustCompile(`(?m)^\s*interface\s+\w+`),
		regexp.MustCompile(`(?m)^\s*trait\s+\w+`),
	},
	"shell": {
		regexp.MustCompile(`(?m)^\s*\w[\w-]*\s*\(\s*\)\s*\{`),
		regexp.MustCompile(`(?m)^\s*function\s+\w+`),
	},
}

// chunkTypeForLanguage returns the best ChunkType for a matched pattern keyword.
func chunkTypeForLine(line string) domain.ChunkType {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case strings.Contains(lower, "class "):
		return domain.ChunkTypeClass
	case strings.Contains(lower, "interface "), strings.Contains(lower, "trait "), strings.Contains(lower, "protocol "):
		return domain.ChunkTypeClass
	case strings.Contains(lower, "struct "), strings.Contains(lower, "enum "), strings.Contains(lower, "impl "):
		return domain.ChunkTypeClass
	default:
		return domain.ChunkTypeFunction
	}
}

// chunkID generates a stable ID for a chunk.
func chunkID(filePath string, startLine int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", filePath, startLine)))
	return fmt.Sprintf("%x", h[:8])
}

// ─────────────────────────────────────────────
// RegexParser
// ─────────────────────────────────────────────

// RegexParser parses source files using language-specific regex patterns to
// identify semantic boundaries (functions, classes, etc.) and extract them as chunks.
type RegexParser struct{}

// NewRegexParser creates a new RegexParser.
func NewRegexParser() *RegexParser {
	return &RegexParser{}
}

// Parse extracts semantic chunks from a source file using regex patterns.
func (p *RegexParser) Parse(ctx context.Context, filePath string) ([]*domain.CodeChunk, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeExternal, "failed to read file")
	}

	lang := LanguageDetector(filePath)
	patterns, hasPatterns := languagePatterns[lang]

	if !hasPatterns {
		// Fall back to generic line-based chunking
		return genericChunk(filePath, lang, string(content)), nil
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	// Find all match positions (line numbers, 0-indexed)
	matchLines := findMatchLines(lines, patterns)

	if len(matchLines) == 0 {
		// No semantic blocks found — fall back to generic chunking
		logger.Debug("No semantic patterns matched, using generic chunking", "path", filePath, "lang", lang)
		return genericChunk(filePath, lang, string(content)), nil
	}

	chunks := make([]*domain.CodeChunk, 0, len(matchLines))
	for i, startLine := range matchLines {
		// End of this chunk = start of next match (or EOF)
		endLine := totalLines
		if i+1 < len(matchLines) {
			endLine = matchLines[i+1]
		}

		// Trim trailing blank lines
		for endLine > startLine+1 && strings.TrimSpace(lines[endLine-1]) == "" {
			endLine--
		}

		chunkContent := strings.Join(lines[startLine:endLine], "\n")
		if strings.TrimSpace(chunkContent) == "" {
			continue
		}

		name := extractName(lines[startLine])
		chunks = append(chunks, &domain.CodeChunk{
			ID:        chunkID(filePath, startLine+1),
			FilePath:  filePath,
			Language:  lang,
			Content:   chunkContent,
			ChunkType: chunkTypeForLine(lines[startLine]),
			StartLine: startLine + 1,
			EndLine:   endLine,
			Metadata: map[string]string{
				"name": name,
			},
		})
	}

	logger.Debug("Parsed file with regex parser",
		"path", filePath,
		"lang", lang,
		"chunks", len(chunks),
	)

	return chunks, nil
}

// findMatchLines returns the 0-indexed line numbers where any pattern matches.
func findMatchLines(lines []string, patterns []*regexp.Regexp) []int {
	seen := make(map[int]bool)
	for _, pat := range patterns {
		for i, line := range lines {
			if pat.MatchString(line) && !seen[i] {
				seen[i] = true
			}
		}
	}
	// Collect and sort
	result := make([]int, 0, len(seen))
	for i := range seen {
		result = append(result, i)
	}
	// Simple insertion sort (usually small)
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j] < result[j-1]; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}

// extractName tries to pull a meaningful name from the first line of a chunk.
func extractName(line string) string {
	line = strings.TrimSpace(line)
	// Strip common keywords to get to the identifier
	for _, kw := range []string{
		"export default ", "export ", "public ", "private ", "protected ",
		"static ", "async ", "abstract ", "final ", "sealed ", "open ",
		"suspend ", "override ", "virtual ", "pub ", "async fn ", "fn ",
		"def ", "class ", "function ", "func ", "fun ", "struct ",
		"interface ", "trait ", "enum ", "impl ", "object ", "module ",
		"namespace ", "type ",
	} {
		if strings.HasPrefix(strings.ToLower(line), kw) {
			line = line[len(kw):]
		}
	}
	// Take up to the first non-identifier character
	end := strings.IndexAny(line, " \t(<{:[=")
	if end > 0 {
		return line[:end]
	}
	if len(line) > 64 {
		return line[:64]
	}
	return line
}

// ─────────────────────────────────────────────
// GenericParser
// ─────────────────────────────────────────────

const (
	genericChunkSize    = 50 // lines per chunk
	genericChunkOverlap = 5  // overlap between chunks
)

// GenericParser splits any text file into fixed-size overlapping line windows.
// Used for config files, markdown, SQL, HTML, CSS, etc.
type GenericParser struct {
	ChunkSize int
	Overlap   int
}

// NewGenericParser creates a new GenericParser with default settings.
func NewGenericParser() *GenericParser {
	return &GenericParser{
		ChunkSize: genericChunkSize,
		Overlap:   genericChunkOverlap,
	}
}

// Parse splits the file into overlapping line-window chunks.
func (p *GenericParser) Parse(ctx context.Context, filePath string) ([]*domain.CodeChunk, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeExternal, "failed to read file")
	}

	lang := LanguageDetector(filePath)
	chunks := genericChunk(filePath, lang, string(content))

	logger.Debug("Parsed file with generic parser",
		"path", filePath,
		"lang", lang,
		"chunks", len(chunks),
	)

	return chunks, nil
}

// genericChunk is a shared helper used by both RegexParser (fallback) and GenericParser.
func genericChunk(filePath, lang, content string) []*domain.CodeChunk {
	lines := strings.Split(content, "\n")
	total := len(lines)
	if total == 0 {
		return nil
	}

	chunkSize := genericChunkSize
	overlap := genericChunkOverlap

	var chunks []*domain.CodeChunk
	for start := 0; start < total; start += chunkSize - overlap {
		end := start + chunkSize
		if end > total {
			end = total
		}

		chunkContent := strings.Join(lines[start:end], "\n")
		if strings.TrimSpace(chunkContent) == "" {
			if end >= total {
				break
			}
			continue
		}

		chunks = append(chunks, &domain.CodeChunk{
			ID:        chunkID(filePath, start+1),
			FilePath:  filePath,
			Language:  lang,
			Content:   chunkContent,
			ChunkType: domain.ChunkTypeOther,
			StartLine: start + 1,
			EndLine:   end,
			Metadata:  map[string]string{},
		})

		if end >= total {
			break
		}
	}

	return chunks
}
