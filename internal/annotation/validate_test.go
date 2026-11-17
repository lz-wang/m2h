package annotation

import (
	"strings"
	"testing"
)

func validSelector() Selector {
	return Selector{
		Quote:    QuoteSelector{Exact: "使用 SQLite", Prefix: "建议", Suffix: "作为存储"},
		Position: PositionSelector{Start: 120, End: 129},
		Version:  SelectorVersion,
	}
}

func TestValidateSelectorAcceptsValidInput(t *testing.T) {
	if err := ValidateSelector(validSelector()); err != nil {
		t.Fatalf("valid selector rejected: %v", err)
	}
	// Multibyte positions count code points, not bytes.
	selector := validSelector()
	selector.Quote.Exact = "中文标注🎯"
	selector.Position = PositionSelector{Start: 0, End: 5}
	if err := ValidateSelector(selector); err != nil {
		t.Fatalf("multibyte selector rejected: %v", err)
	}
}

func TestValidateSelectorRejections(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Selector)
		wantErr string
	}{
		{
			name:    "wrong version",
			mutate:  func(s *Selector) { s.Version = 2 },
			wantErr: "version",
		},
		{
			name:    "empty quote",
			mutate:  func(s *Selector) { s.Quote.Exact = "" },
			wantErr: "must not be empty",
		},
		{
			name: "quote too long",
			mutate: func(s *Selector) {
				s.Quote.Exact = strings.Repeat("字", MaxQuoteRunes+1)
				s.Position = PositionSelector{Start: 0, End: MaxQuoteRunes + 1}
			},
			wantErr: "exceeds",
		},
		{
			name: "prefix too long",
			mutate: func(s *Selector) {
				s.Quote.Prefix = strings.Repeat("a", MaxContextRunes+1)
			},
			wantErr: "prefix exceeds",
		},
		{
			name: "suffix too long",
			mutate: func(s *Selector) {
				s.Quote.Suffix = strings.Repeat("a", MaxContextRunes+1)
			},
			wantErr: "suffix exceeds",
		},
		{
			name: "backwards range",
			mutate: func(s *Selector) {
				s.Position = PositionSelector{Start: 10, End: 10}
			},
			wantErr: "forward range",
		},
		{
			name: "negative start",
			mutate: func(s *Selector) {
				s.Position = PositionSelector{Start: -1, End: 8}
			},
			wantErr: "forward range",
		},
		{
			name: "position beyond ceiling",
			mutate: func(s *Selector) {
				s.Position = PositionSelector{Start: MaxPosition, End: MaxPosition + 9}
			},
			wantErr: "maximum document size",
		},
		{
			name: "span length mismatch",
			mutate: func(s *Selector) {
				s.Position = PositionSelector{Start: 0, End: 99}
			},
			wantErr: "does not match",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selector := validSelector()
			test.mutate(&selector)
			err := ValidateSelector(selector)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateSelector() = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateCommentLimit(t *testing.T) {
	if err := ValidateComment(""); err != nil {
		t.Fatalf("empty comment rejected: %v", err)
	}
	if err := ValidateComment("需要补充备份方案"); err != nil {
		t.Fatalf("normal comment rejected: %v", err)
	}
	if err := ValidateComment(strings.Repeat("字", MaxCommentRunes)); err != nil {
		t.Fatalf("limit-length comment rejected: %v", err)
	}
	if err := ValidateComment(strings.Repeat("字", MaxCommentRunes+1)); err == nil {
		t.Fatalf("overlong comment accepted")
	}
}
