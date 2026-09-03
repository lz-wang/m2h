// Package search ranks projected Markdown documents against a query. It is a
// pure matching engine: it knows nothing about the filesystem, HTTP or
// frontmatter parsing — callers feed it Documents (metadata plus the chunks
// produced by markdown.ProjectForSearch) and get back at most one Result per
// document, carrying the best matching section.
package search

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/lz-wang/m2h/internal/markdown"
)

// Field weights. The numbers are internal constants, not an API contract —
// only their order is: an exact title beats a partial title, which beats a
// path hit, then tags, the description, a heading, body prose and finally
// code. The description outranks ordinary prose because frontmatter declares
// it the document's summary, not a random paragraph.
const (
	scoreTitleExact    = 1000
	scoreTitleContains = 800
	scorePath          = 650
	scoreTag           = 600
	scoreDescription   = 550
	scoreHeadingChunk  = 500
	scoreTextChunk     = 300
	scoreCodeChunk     = 250

	// Snippet shaping: at most maxSnippetRunes runes around the first
	// matched token, keeping snippetContextRunes of lead-in so the match is
	// readable in context. All slicing is rune-based — a byte cut would
	// break UTF-8 in exactly the CJK documents this search exists for.
	maxSnippetRunes     = 200
	snippetContextRunes = 40
)

// Document is one searchable document: its addressable identity (path,
// title), the frontmatter-derived metadata and the semantic chunks projected
// from its body.
type Document struct {
	Path        string
	Title       string
	Description string
	Tags        []string
	Chunks      []markdown.SearchChunk
}

// Result is the single best match of one document. Snippet is a plain-text
// excerpt around the best matching content (empty for metadata-only matches
// that carry no description), and HeadingID/HeadingText locate the matching
// section — both empty when the document matched only on metadata or on
// content before its first anchored heading. Score is internal: it orders
// results but is never exposed to clients.
type Result struct {
	Path        string
	Title       string
	Snippet     string
	HeadingID   string
	HeadingText string

	score int
}

// Match reports whether the document satisfies every query token, and the
// best single result when it does. Query rules for this first version are
// deliberately simple: whitespace-separated tokens joined by AND,
// case-insensitive substring matching (which makes CJK substring searches
// work with no segmentation). No OR, wildcard, fuzzy or stemming.
func Match(document Document, query string) (Result, bool) {
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return Result{}, false
	}

	titleLower := strings.ToLower(document.Title)
	pathLower := strings.ToLower(document.Path)
	descriptionLower := strings.ToLower(document.Description)
	descriptionHit := false

	total := 0
	chunkHits := make([]bool, len(document.Chunks))
	for _, token := range tokens {
		best := 0
		if strings.Contains(titleLower, token) {
			best = scoreTitleContains
			if len(tokens) == 1 && titleLower == token {
				best = scoreTitleExact
			}
		}
		if strings.Contains(pathLower, token) && best < scorePath {
			best = scorePath
		}
		for _, tag := range document.Tags {
			if strings.Contains(strings.ToLower(tag), token) {
				best = max(best, scoreTag)
				break
			}
		}
		if strings.Contains(descriptionLower, token) {
			descriptionHit = true
			best = max(best, scoreDescription)
		}
		for index, chunk := range document.Chunks {
			if !strings.Contains(strings.ToLower(chunk.Text), token) {
				continue
			}
			chunkHits[index] = true
			best = max(best, chunkScore(chunk))
		}
		// AND semantics: one unsatisfied token excludes the document, no
		// matter how strongly the others matched.
		if best == 0 {
			return Result{}, false
		}
		total += best
	}

	result := Result{
		Path:  document.Path,
		Title: document.Title,
		score: total,
	}
	section, bestChunk, bestTotal := bestSection(document.Chunks, chunkHits, tokens)
	if bestTotal > 0 {
		result.HeadingID = section.HeadingID
		result.HeadingText = section.HeadingText
		result.Snippet = snippetAround(bestChunk.Text, tokens)
		return result, true
	}
	// Metadata-only match. A matching description is the summary the author
	// wrote for exactly this purpose; a title/path/tag match alone should
	// not fabricate a body excerpt.
	if descriptionHit {
		result.Snippet = truncateRunes(document.Description, maxSnippetRunes)
	}
	return result, true
}

// SortResults orders matched results strongest-first, with the path as a
// deterministic tie-break so equal scores never flip between requests.
func SortResults(results []Result) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].Path < results[j].Path
	})
}

// tokenize normalizes a raw query into lowercased AND tokens.
func tokenize(query string) []string {
	fields := strings.Fields(strings.TrimSpace(query))
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		tokens = append(tokens, strings.ToLower(field))
	}
	return tokens
}

func chunkScore(chunk markdown.SearchChunk) int {
	switch chunk.Kind {
	case markdown.SearchChunkHeading:
		return scoreHeadingChunk
	case markdown.SearchChunkCode:
		return scoreCodeChunk
	default:
		return scoreTextChunk
	}
}

// sectionMatch accumulates one section's matched chunks while sections are
// visited in document order.
type sectionMatch struct {
	headingID   string
	headingText string
	total       int
	bestChunk   markdown.SearchChunk
	bestWeight  int
}

// bestSection groups the matched chunks by their section and returns the
// strongest one — the section where the matched tokens score highest — along
// with its own best chunk for the snippet. Sections keep document order;
// ties go to the earlier section. A zero total means no chunk matched and
// the caller must fall back to metadata-only shaping.
func bestSection(chunks []markdown.SearchChunk, chunkHits []bool, tokens []string) (markdown.SearchChunk, markdown.SearchChunk, int) {
	type sectionKey struct {
		id, text string
	}
	var order []sectionKey
	sections := make(map[sectionKey]*sectionMatch)

	for index, chunk := range chunks {
		if !chunkHits[index] {
			continue
		}
		key := sectionKey{id: chunk.HeadingID, text: chunk.HeadingText}
		section := sections[key]
		if section == nil {
			section = &sectionMatch{headingID: chunk.HeadingID, headingText: chunk.HeadingText}
			sections[key] = section
			order = append(order, key)
		}
		weight := chunkWeight(chunk, tokens)
		section.total += weight
		if section.bestChunk.Text == "" || weight > section.bestWeight {
			section.bestChunk = chunk
			section.bestWeight = weight
		}
	}

	var bestSection, bestChunk markdown.SearchChunk
	bestTotal := 0
	for _, key := range order {
		section := sections[key]
		if section.total > bestTotal {
			bestTotal = section.total
			bestSection = markdown.SearchChunk{
				Kind:        markdown.SearchChunkHeading,
				Text:        section.headingText,
				HeadingID:   section.headingID,
				HeadingText: section.headingText,
			}
			bestChunk = section.bestChunk
		}
	}
	return bestSection, bestChunk, bestTotal
}

// chunkWeight is the score one chunk earns across all matched tokens; it is
// the measure competing chunks of a section are compared on.
func chunkWeight(chunk markdown.SearchChunk, tokens []string) int {
	weight := 0
	lower := strings.ToLower(chunk.Text)
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			weight += chunkScore(chunk)
		}
	}
	return weight
}

// snippetAround shapes a plain-text excerpt of at most maxSnippetRunes runes
// around the first token occurrence. The anchor is located on a lowercased
// copy so matching stays case-insensitive, but the excerpt is cut from the
// original text; when lowercasing shifts rune counts (rare Unicode edge
// cases) the excerpt degrades to a head truncation instead of risking a
// misaligned cut.
func snippetAround(text string, tokens []string) string {
	runes := []rune(text)
	if len(runes) <= maxSnippetRunes {
		return text
	}
	lowerText := strings.ToLower(text)
	if utf8.RuneCountInString(lowerText) != len(runes) {
		return truncateRunes(text, maxSnippetRunes)
	}

	anchorByte := -1
	for _, token := range tokens {
		if index := strings.Index(lowerText, token); index >= 0 && (anchorByte < 0 || index < anchorByte) {
			anchorByte = index
		}
	}
	if anchorByte < 0 {
		return truncateRunes(text, maxSnippetRunes)
	}
	anchor := len([]rune(lowerText[:anchorByte]))

	start := max(0, anchor-snippetContextRunes)
	end := start + maxSnippetRunes
	if end > len(runes) {
		end = len(runes)
		start = max(0, end-maxSnippetRunes)
	}
	var builder strings.Builder
	if start > 0 {
		builder.WriteRune('…')
	}
	builder.WriteString(string(runes[start:end]))
	if end < len(runes) {
		builder.WriteRune('…')
	}
	return builder.String()
}

// truncateRunes cuts text to limit runes, rune-safely, marking the cut.
func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}
