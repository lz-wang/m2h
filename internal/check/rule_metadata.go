package check

import (
	"fmt"
	"strings"

	"github.com/lz-wang/m2h/internal/markdown"
)

// checkMetadataRules compares normalized metadata with the same heading and
// tag values the document UI uses. Optional metadata stays optional.
func checkMetadataRules(current *indexedDocument, rules RuleSet) []Diagnostic {
	meta := current.frontMatter
	if meta == nil {
		return nil
	}
	diagnostics := make([]Diagnostic, 0)
	if rules.Enabled(RuleFrontMatterTitleMismatch) && meta.Title != "" {
		for _, heading := range current.inspection.Headings {
			if heading.Level != 1 {
				continue
			}
			if strings.Join(strings.Fields(meta.Title), " ") != heading.Text {
				diagnostics = append(diagnostics, current.diagnosticForRule(RuleFrontMatterTitleMismatch,
					fmt.Sprintf("frontmatter title %q does not match first H1 %q", meta.Title, heading.Text), metadataPosition(meta, "title")))
			}
			break
		}
	}
	if rules.Enabled(RuleFrontMatterTagsCount) {
		for _, entry := range meta.Entries {
			if entry.Key != "tags" {
				continue
			}
			if count := len(meta.Tags); count < 1 || count > 5 {
				diagnostics = append(diagnostics, current.diagnosticForRule(RuleFrontMatterTagsCount,
					fmt.Sprintf("frontmatter has %d tags; expected 1 to 5", count), metadataPosition(meta, "tags")))
			}
			break
		}
	}
	return diagnostics
}

func metadataPosition(meta *markdown.FrontMatter, key string) markdown.Position {
	for _, entry := range meta.Entries {
		if entry.Key == key {
			return markdown.Position{Line: max(entry.Line+1, 1), Column: max(entry.Column, 1)}
		}
	}
	return markdown.Position{Line: 1, Column: 1}
}
