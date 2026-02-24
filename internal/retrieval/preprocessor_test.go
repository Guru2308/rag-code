package retrieval

import (
	"reflect"
	"testing"
)

func TestPreprocess(t *testing.T) {
	p := NewQueryPreprocessor()
	q := "Get User By ID"
	res := p.Preprocess(q)

	if res.Original != q {
		t.Errorf("Original = %v, want %v", res.Original, q)
	}
	// "get", "user", "by", "id". "by" is stopword? defaultStopWords has "by"
	// "get", "user", "id"
	expected := []string{"get", "user", "id"}
	if !reflect.DeepEqual(res.Filtered, expected) {
		t.Errorf("Filtered = %v, want %v", res.Filtered, expected)
	}
}

func TestTokenize(t *testing.T) {
	p := NewQueryPreprocessor()
	tests := []struct {
		input string
		want  []string
	}{
		{"snake_case", []string{"snake", "case"}},
		{"camelCase", []string{"camel", "case"}},
		{"normal text", []string{"normal", "text"}},
		{"api.endpoint", []string{"api", "endpoint"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := p.tokenize(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tokenize(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
