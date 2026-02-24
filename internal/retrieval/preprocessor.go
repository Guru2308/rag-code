package retrieval

import (
	"regexp"
	"strings"
	"unicode"
)

// QueryPreprocessor handles query normalization and tokenization
type QueryPreprocessor struct {
	stopWords map[string]bool
}

// ProcessedQuery represents a preprocessed query
type ProcessedQuery struct {
	Original string
	Tokens   []string
	Filtered []string // tokens after stop word removal
}

// NewQueryPreprocessor creates a new query preprocessor
func NewQueryPreprocessor() *QueryPreprocessor {
	return &QueryPreprocessor{
		stopWords: defaultStopWords(),
	}
}

// Preprocess normalizes and tokenizes a query
func (p *QueryPreprocessor) Preprocess(query string) ProcessedQuery {
	// Normalize: lowercase and trim
	normalized := strings.ToLower(strings.TrimSpace(query))

	// Tokenize
	tokens := p.tokenize(normalized)

	// Filter stop words
	filtered := p.filterStopWords(tokens)

	return ProcessedQuery{
		Original: query,
		Tokens:   tokens,
		Filtered: filtered,
	}
}

// tokenize splits text into tokens, preserving code-specific patterns
func (p *QueryPreprocessor) tokenize(text string) []string {
	var tokens []string

	// Split on whitespace and punctuation, but preserve:
	// - camelCase: getUserById -> get, User, By, Id
	// - snake_case: get_user_by_id -> get, user, by, id
	// - dots: api.handler -> api, handler

	words := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) && r != '_'
	})

	for _, word := range words {
		// Handle snake_case
		if strings.Contains(word, "_") {
			parts := strings.Split(word, "_")
			for _, part := range parts {
				if part != "" {
					tokens = append(tokens, p.splitCamelCase(part)...)
				}
			}
		} else {
			tokens = append(tokens, p.splitCamelCase(word)...)
		}
	}

	return tokens
}

// splitCamelCase splits camelCase words
func (p *QueryPreprocessor) splitCamelCase(word string) []string {
	if word == "" {
		return nil
	}

	// Simple camelCase split: find uppercase letters
	re := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	split := re.ReplaceAllString(word, `$1 $2`)

	parts := strings.Fields(split)
	var result []string
	for _, part := range parts {
		if part != "" {
			result = append(result, strings.ToLower(part))
		}
	}

	// If no splits happened, return the original word
	if len(result) == 0 {
		return []string{strings.ToLower(word)}
	}

	return result
}

// filterStopWords removes common stop words from tokens
func (p *QueryPreprocessor) filterStopWords(tokens []string) []string {
	var filtered []string
	for _, token := range tokens {
		if !p.stopWords[token] && len(token) > 1 {
			filtered = append(filtered, token)
		}
	}
	return filtered
}

// defaultStopWords returns a set of common English stop words
func defaultStopWords() map[string]bool {
	words := []string{
		"a", "an", "and", "are", "as", "at", "be", "by", "for",
		"from", "has", "he", "in", "is", "it", "its", "of", "on",
		"that", "the", "to", "was", "will", "with", "this", "have",
		"i", "you", "we", "they", "what", "where", "when", "why",
		"how", "which", "who", "can", "could", "would", "should",
		"do", "does", "did", "don't", "doesn't", "didn't",
	}

	stopWords := make(map[string]bool)
	for _, word := range words {
		stopWords[word] = true
	}
	return stopWords
}
