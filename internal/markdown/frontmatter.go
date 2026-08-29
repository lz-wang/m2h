package markdown

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// FrontMatter is the structured metadata extracted from a document's YAML
// frontmatter. Entries preserves the original source order; Title, the three
// date derivations and Tags are normalized fields the preview UI surfaces as
// the document's display title and the toolbar summary. CreatedDate and
// UpdatedDate are alias-normalized (see the priority tables below) so the WebUI
// never has to reinterpret raw frontmatter keys.
type FrontMatter struct {
	Entries     []FrontMatterEntry
	Title       string
	CreatedDate string
	UpdatedDate string
	Date        string
	Tags        []string
}

// FrontMatterEntry is a single frontmatter key/value pair rendered as a table
// row. Value is a display string: scalars keep their decoded text, while
// sequences and mappings are re-serialized as readable YAML. Line and Column
// locate the key inside the frontmatter YAML block, 1-based, so diagnostics
// can point at the source line; the caller adds the file-level offset (the
// opening `---` delimiter sits above the block).
type FrontMatterEntry struct {
	Key    string
	Value  string
	Line   int
	Column int
}

// ParseFrontMatter splits optional YAML frontmatter from source. When a valid
// `--- ... ---` block opens the document it returns the parsed metadata and the
// remaining Markdown body; otherwise it returns source unchanged with a nil
// FrontMatter. A declared frontmatter block whose YAML is invalid or not a
// mapping yields an error so callers can surface it as a user-facing failure.
func ParseFrontMatter(source []byte) (body []byte, frontMatter *FrontMatter, err error) {
	raw, body, ok := splitFrontMatter(source)
	if !ok {
		return source, nil, nil
	}
	frontMatter, err = parseFrontMatterYAML(raw)
	if err != nil {
		return nil, nil, err
	}
	return body, frontMatter, nil
}

// FrontMatterLineOffset reports the 1-based source line where the Markdown
// body starts after a leading frontmatter block, or 1 when the document has
// none. Callers that inspect the body use it to translate body-relative
// positions back to source positions.
func FrontMatterLineOffset(source []byte) int {
	raw, _, ok := splitFrontMatter(source)
	if !ok {
		return 1
	}
	// One line for the opening delimiter, the raw YAML lines, and one for
	// the closing delimiter.
	return bytes.Count(raw, []byte("\n")) + 3
}

// splitFrontMatter locates a leading `---` opening delimiter and its matching
// closing `---` line. It returns the YAML between them and the remaining body.
// A missing closing delimiter is treated as ordinary Markdown (not frontmatter)
// because a lone `---` is also a Markdown horizontal rule.
func splitFrontMatter(source []byte) (raw []byte, body []byte, ok bool) {
	if !bytes.HasPrefix(source, []byte("---")) {
		return nil, nil, false
	}
	if len(source) > 3 && source[3] != '\n' && source[3] != '\r' {
		return nil, nil, false
	}

	var afterOpening []byte
	switch {
	case len(source) <= 3:
		afterOpening = nil
	default:
		afterOpening = source[4:]
		if source[3] == '\r' && len(afterOpening) > 0 && afterOpening[0] == '\n' {
			afterOpening = afterOpening[1:]
		}
	}

	offset := 0
	search := afterOpening
	for {
		end := bytes.IndexByte(search, '\n')
		var line string
		if end < 0 {
			line = string(search)
		} else {
			line = string(search[:end])
		}
		if strings.TrimSpace(line) == "---" {
			raw = afterOpening[:offset]
			if end < 0 {
				body = nil
			} else {
				body = afterOpening[offset+end+1:]
			}
			return raw, body, true
		}
		if end < 0 {
			return nil, nil, false
		}
		search = search[end+1:]
		offset += end + 1
	}
}

func parseFrontMatterYAML(raw []byte) (*FrontMatter, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return &FrontMatter{}, nil
	}

	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("frontmatter is not valid YAML: %w", err)
	}
	if len(document.Content) == 0 {
		return &FrontMatter{}, nil
	}

	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("frontmatter must be a mapping")
	}

	meta := &FrontMatter{}
	createdPriority := math.MaxInt
	updatedPriority := math.MaxInt
	for i := 0; i+1 < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valueNode := root.Content[i+1]
		key := keyNode.Value

		displayValue, err := formatFrontMatterValue(valueNode)
		if err != nil {
			return nil, err
		}
		meta.Entries = append(meta.Entries, FrontMatterEntry{
			Key:    key,
			Value:  displayValue,
			Line:   keyNode.Line,
			Column: keyNode.Column,
		})

		switch key {
		case "title":
			meta.Title = normalizeFrontMatterTitle(valueNode)
		case "date":
			meta.Date = normalizeFrontMatterDate(valueNode)
		case "tags":
			meta.Tags = normalizeFrontMatterTags(valueNode)
		}
		// The alias families resolve by fixed priority, never by YAML key
		// order: whichever alias appears first or last, create_date always
		// outranks create_at and create_time (and likewise for update_*).
		if priority, aliased := createDatePriorities[key]; aliased {
			meta.CreatedDate, createdPriority = preferDateAlias(
				meta.CreatedDate,
				createdPriority,
				priority,
				normalizeFrontMatterDate(valueNode),
			)
		}
		if priority, aliased := updateDatePriorities[key]; aliased {
			meta.UpdatedDate, updatedPriority = preferDateAlias(
				meta.UpdatedDate,
				updatedPriority,
				priority,
				normalizeFrontMatterDate(valueNode),
			)
		}
	}
	return meta, nil
}

// Alias priority tables for the created/updated date derivations. Lower wins.
// Deliberately only these exact keys: no guessing at created_at/modified_at
// style names, which would make the summary rules grow without bound.
var createDatePriorities = map[string]int{
	"create_date": 0,
	"create_at":   1,
	"create_time": 2,
}

var updateDatePriorities = map[string]int{
	"update_date": 0,
	"update_at":   1,
	"update_time": 2,
}

// preferDateAlias keeps the better of the current and the candidate date
// within one alias family. A candidate that does not normalize to an ISO date
// contributes nothing (the raw value stays visible in the entries table), and
// an empty higher-priority alias never blocks a valid lower-priority one.
func preferDateAlias(current string, currentPriority, candidatePriority int, candidate string) (string, int) {
	if candidate == "" || candidatePriority >= currentPriority {
		return current, currentPriority
	}
	return candidate, candidatePriority
}

// formatFrontMatterValue renders a frontmatter value for the full table view.
// Scalars use their decoded text; sequences and mappings are re-serialized as
// readable YAML so multi-line values stay legible inside the table cell.
func formatFrontMatterValue(node *yaml.Node) (string, error) {
	if node.Kind == yaml.ScalarNode {
		return node.Value, nil
	}
	data, err := yaml.Marshal(node)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// normalizeFrontMatterTitle accepts only a scalar title. Sequence or mapping
// titles stay visible in the full frontmatter table but never become the
// document's display title, which would otherwise surface re-serialized YAML
// as a heading.
func normalizeFrontMatterTitle(node *yaml.Node) string {
	if node.Kind != yaml.ScalarNode {
		return ""
	}
	return strings.TrimSpace(node.Value)
}

// PreferredTitle resolves a document's display title: a non-empty frontmatter
// title wins, anything else falls back to the caller's derived title (the
// first H1 with a filename fallback). Centralizing the preference keeps the
// file list and the document view from ever disagreeing about the rule.
func PreferredTitle(frontMatter *FrontMatter, fallback string) string {
	if frontMatter != nil && frontMatter.Title != "" {
		return frontMatter.Title
	}
	return fallback
}

// normalizeFrontMatterDate accepts only ISO-style dates and returns the
// calendar day (YYYY-MM-DD) for the toolbar summary. Arbitrary strings remain
// visible in the full frontmatter table but never reach the summary, which is
// more reliable than guessing a date format.
func normalizeFrontMatterDate(node *yaml.Node) string {
	if node.Kind != yaml.ScalarNode {
		return ""
	}
	value := strings.TrimSpace(node.Value)
	if !IsISODate(value) {
		return ""
	}
	return value[:10]
}

// IsISODate reports whether value begins with a valid ISO calendar date
// (YYYY-MM-DD) followed by the end of the value, a 'T' or a space — the exact
// contract frontmatter date summaries apply. The check command reuses it to
// report date fields that can never reach the summary.
func IsISODate(value string) bool {
	if len(value) < 10 {
		return false
	}
	candidate := value[:10]
	if _, err := time.Parse("2006-01-02", candidate); err != nil {
		return false
	}
	if len(value) > 10 && value[10] != 'T' && value[10] != ' ' {
		return false
	}
	return true
}

// normalizeFrontMatterTags supports YAML sequences, inline flow sequences, and
// a single scalar tag. Comma-separated scalars are intentionally not split
// since the value itself may legitimately contain a comma.
func normalizeFrontMatterTags(node *yaml.Node) []string {
	switch node.Kind {
	case yaml.ScalarNode:
		value := strings.TrimSpace(node.Value)
		if value == "" {
			return nil
		}
		return []string{value}
	case yaml.SequenceNode:
		result := make([]string, 0, len(node.Content))
		seen := make(map[string]struct{}, len(node.Content))
		for _, child := range node.Content {
			if child.Kind != yaml.ScalarNode {
				continue
			}
			value := strings.TrimSpace(child.Value)
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
		return result
	}
	return nil
}
