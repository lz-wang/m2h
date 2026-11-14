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
// only their order is: a partial title beats a path hit, then tags, the
// description, a heading, body prose and finally code. An exact title is
// not a weight at all: it is a document-level flag that sorts ahead of
// every scored result. The description outranks ordinary prose because
// frontmatter declares it the document's summary, not a random paragraph.
const (
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
// content before its first anchored heading. Score and exactTitle are
// internal: they order results but are never exposed to clients.
type Result struct {
	Path        string
	Title       string
	Snippet     string
	HeadingID   string
	HeadingText string
	TabID       string

	score      int
	exactTitle bool
}

// PreparedQuery is one query tokenized, lowercased and normalized exactly
// once. A search request matches the same query against every document in
// the index; splitting and normalizing per document would repeat work the
// answer never depends on. The zero value matches nothing.
type PreparedQuery struct {
	tokens     []string
	normalized string // tokens joined by single spaces, for exact-title hits
}

// PrepareQuery parses a raw query into its reusable form.
func PrepareQuery(query string) PreparedQuery {
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return PreparedQuery{}
	}
	return PreparedQuery{tokens: tokens, normalized: strings.Join(tokens, " ")}
}

// PreparedDocument caches the lowercase search text of every matchable field
// next to the original document. Lowercasing is matching's dominant cost —
// every chunk, once per token, on every query — so the index pays it once
// per document revision and the match loop reads only precomputed forms.
// The originals are kept verbatim: snippets, titles and descriptions must
// keep their case and Unicode shape.
type PreparedDocument struct {
	Document Document

	titleLower           string
	normalizedTitleLower string // whitespace-collapsed titleLower, for exact-title hits
	pathLower            string
	descriptionLower     string
	tagsLower            []string
	chunksLower          []string // parallel to Document.Chunks
}

// PrepareDocument builds the reusable search form of one document. Call it
// when a document enters (or changes inside) an index, not per query.
func PrepareDocument(document Document) PreparedDocument {
	prepared := PreparedDocument{
		Document:         document,
		titleLower:       strings.ToLower(document.Title),
		pathLower:        strings.ToLower(document.Path),
		descriptionLower: strings.ToLower(document.Description),
		tagsLower:        make([]string, len(document.Tags)),
		chunksLower:      make([]string, len(document.Chunks)),
	}
	prepared.normalizedTitleLower = strings.Join(strings.Fields(prepared.titleLower), " ")
	for index, tag := range document.Tags {
		prepared.tagsLower[index] = strings.ToLower(tag)
	}
	for index, chunk := range document.Chunks {
		prepared.chunksLower[index] = strings.ToLower(chunk.Text)
	}
	return prepared
}

// Match reports whether the document satisfies every query token, and the
// best single result when it does. Query rules for this first version are
// deliberately simple: whitespace-separated tokens joined by AND,
// case-insensitive substring matching (which makes CJK substring searches
// work with no segmentation). No OR, wildcard, fuzzy or stemming.
//
// It is the compatibility entry: it prepares both sides per call. Callers
// matching one query against many documents should PrepareQuery once and
// match PreparedDocuments instead.
func Match(document Document, query string) (Result, bool) {
	preparedQuery := PrepareQuery(query)
	if len(preparedQuery.tokens) == 0 {
		return Result{}, false
	}
	prepared := PrepareDocument(document)
	return preparedQuery.Match(&prepared)
}

// Match answers from the prepared forms: identical matching, scoring,
// section choice and snippet shaping to the package-level Match, with zero
// lowercasing in the loop.
func (query PreparedQuery) Match(document *PreparedDocument) (Result, bool) {
	if len(query.tokens) == 0 {
		return Result{}, false
	}

	descriptionHit := false
	// An exact title is a document-level signal, compared on
	// whitespace-normalized forms: a query like "markdown rendering" must
	// rank the title "Markdown Rendering" above "Markdown Rendering Guide"
	// no matter how the per-token field scores add up. Field weights stay
	// per token; only the exact flag rides above them.
	exactTitle := document.normalizedTitleLower == query.normalized

	total := 0
	chunkHits := make([]bool, len(document.Document.Chunks))
	for _, token := range query.tokens {
		best := 0
		if strings.Contains(document.titleLower, token) {
			best = scoreTitleContains
		}
		if strings.Contains(document.pathLower, token) && best < scorePath {
			best = scorePath
		}
		for _, tag := range document.tagsLower {
			if strings.Contains(tag, token) {
				best = max(best, scoreTag)
				break
			}
		}
		if strings.Contains(document.descriptionLower, token) {
			descriptionHit = true
			best = max(best, scoreDescription)
		}
		for index, lower := range document.chunksLower {
			if !strings.Contains(lower, token) {
				continue
			}
			chunkHits[index] = true
			best = max(best, chunkScore(document.Document.Chunks[index]))
		}
		// AND semantics: one unsatisfied token excludes the document, no
		// matter how strongly the others matched.
		if best == 0 {
			return Result{}, false
		}
		total += best
	}

	result := Result{
		Path:       document.Document.Path,
		Title:      document.Document.Title,
		score:      total,
		exactTitle: exactTitle,
	}
	section, bestChunk, bestLower, bestTotal := bestSection(document, chunkHits, query.tokens)
	if bestTotal > 0 {
		result.HeadingID = section.HeadingID
		result.HeadingText = section.HeadingText
		result.TabID = section.TabID
		result.Snippet = snippetAround(bestChunk.Text, bestLower, query.tokens)
		return result, true
	}
	// Metadata-only match. A matching description is the summary the author
	// wrote for exactly this purpose; a title/path/tag match alone should
	// not fabricate a body excerpt.
	if descriptionHit {
		result.Snippet = truncateRunes(document.Document.Description, maxSnippetRunes)
	}
	return result, true
}

// SortResults orders matched results strongest-first: an exact title always
// leads, whatever its relevance score, then the scored results, with the
// path as a deterministic tie-break so equal scores never flip between
// requests.
func SortResults(results []Result) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].exactTitle != results[j].exactTitle {
			return results[i].exactTitle
		}
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
	tabID       string
	total       int
	bestChunk   markdown.SearchChunk
	bestLower   string // precomputed lowercase of bestChunk.Text
	bestWeight  int
}

// bestSection groups the matched chunks by their section and returns the
// strongest one — the section where the matched tokens score highest — along
// with its own best chunk (and that chunk's precomputed lowercase form, for
// snippet anchoring). Sections keep document order; ties go to the earlier
// section. A zero total means no chunk matched and the caller must fall
// back to metadata-only shaping.
func bestSection(document *PreparedDocument, chunkHits []bool, tokens []string) (markdown.SearchChunk, markdown.SearchChunk, string, int) {
	type sectionKey struct {
		id, text, tabID string
	}
	var order []sectionKey
	sections := make(map[sectionKey]*sectionMatch)

	for index, chunk := range document.Document.Chunks {
		if !chunkHits[index] {
			continue
		}
		key := sectionKey{id: chunk.HeadingID, text: chunk.HeadingText, tabID: chunk.TabID}
		section := sections[key]
		if section == nil {
			section = &sectionMatch{headingID: chunk.HeadingID, headingText: chunk.HeadingText, tabID: chunk.TabID}
			sections[key] = section
			order = append(order, key)
		}
		weight := chunkWeight(chunk, document.chunksLower[index], tokens)
		section.total += weight
		if section.bestChunk.Text == "" || weight > section.bestWeight {
			section.bestChunk = chunk
			section.bestLower = document.chunksLower[index]
			section.bestWeight = weight
		}
	}

	var bestSection, bestChunk markdown.SearchChunk
	var bestLower string
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
				TabID:       section.tabID,
			}
			bestChunk = section.bestChunk
			bestLower = section.bestLower
		}
	}
	return bestSection, bestChunk, bestLower, bestTotal
}

// chunkWeight is the score one chunk earns across all matched tokens; it is
// the measure competing chunks of a section are compared on.
func chunkWeight(chunk markdown.SearchChunk, lower string, tokens []string) int {
	weight := 0
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			weight += chunkScore(chunk)
		}
	}
	return weight
}

// snippetAround shapes a plain-text excerpt of at most maxSnippetRunes runes
// around the first token occurrence. The anchor is located on the document's
// precomputed lowercase copy so matching stays case-insensitive, but the
// excerpt is cut from the original text; when lowercasing shifts rune counts
// (rare Unicode edge cases) the excerpt degrades to a head truncation
// instead of risking a misaligned cut.
func snippetAround(text, lowerText string, tokens []string) string {
	runes := []rune(text)
	if len(runes) <= maxSnippetRunes {
		return text
	}
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
