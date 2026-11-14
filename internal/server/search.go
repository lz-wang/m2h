package server

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/lz-wang/m2h/internal/markdown"
	"github.com/lz-wang/m2h/internal/search"
)

const (
	// maxSearchResults caps one response. The WebUI has no pagination; a
	// workspace scan that surfaces more than this many documents needs a
	// more specific query, not more scrolling.
	maxSearchResults = 50
	// maxSearchQueryRunes bounds the query a single scan will accept. One
	// rune is legal on purpose: single-character CJK queries ("图", "表")
	// are legitimate, whether to send them automatically is the client's
	// decision.
	maxSearchQueryRunes = 128
)

type searchHeadingResponse struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type searchResultResponse struct {
	TabID   string                 `json:"tabId,omitempty"`
	Path    string                 `json:"path"`
	Title   string                 `json:"title"`
	Snippet string                 `json:"snippet,omitempty"`
	Heading *searchHeadingResponse `json:"heading,omitempty"`
}

type searchResponse struct {
	Query   string                 `json:"query"`
	Results []searchResultResponse `json:"results"`
}

// serveSearch answers GET /api/search?q=<query> from the shared search
// snapshot. The index refreshes the file set before answering (every file
// re-discovered, only changed files re-read and re-parsed), so queries cost
// a metadata scan plus in-memory matching instead of reading the whole
// library. Scope rules are inherited unchanged from serveFiles — discovery
// and every read go through the same resolveVisibleDocument boundary the
// document routes use, so --glob, --depth, hidden paths, single-file scopes
// and multi-root identities behave exactly as before.
func (handler *documentHandler) serveSearch(response http.ResponseWriter, request *http.Request) {
	query, ok := searchQuery(request, response)
	if !ok {
		return
	}

	snapshot, err := handler.search.acquire(request.Context())
	if err != nil {
		// A cancelled context surfaces as a refresh error first; the client
		// is gone, so stop without answering.
		if request.Context().Err() != nil {
			return
		}
		switch {
		case errors.Is(err, errIgnoreUnavailable):
			writeJSONError(response, http.StatusInternalServerError, "evaluate ignore rules")
		case errors.Is(err, errSearchDiscover):
			writeJSONError(response, http.StatusInternalServerError, "discover Markdown files")
		default:
			writeJSONError(response, http.StatusInternalServerError, "search Markdown documents")
		}
		return
	}

	// The client aborts superseded queries; matching the rest of the cached
	// library to build a response nobody reads is wasted work.
	matches := make([]search.Result, 0)
	for publicPath, indexed := range snapshot.Files {
		if request.Context().Err() != nil {
			return
		}
		if result, matched := search.Match(indexed.Document, query); matched {
			result.Path = publicPath
			matches = append(matches, result)
		}
	}
	if request.Context().Err() != nil {
		return
	}
	search.SortResults(matches)
	if len(matches) > maxSearchResults {
		matches = matches[:maxSearchResults]
	}

	results := make([]searchResultResponse, 0, len(matches))
	for _, match := range matches {
		result := searchResultResponse{
			Path:    match.Path,
			Title:   match.Title,
			Snippet: match.Snippet,
			TabID:   match.TabID,
		}
		if match.HeadingID != "" {
			result.Heading = &searchHeadingResponse{ID: match.HeadingID, Text: match.HeadingText}
		}
		results = append(results, result)
	}
	writeJSON(response, http.StatusOK, searchResponse{Query: query, Results: results})
}

// searchQuery applies the search endpoint's strict query contract, mirroring
// the API's strict query-parameter style: exactly one parameter named q,
// non-empty after trimming, at most maxSearchQueryRunes runes.
func searchQuery(request *http.Request, response http.ResponseWriter) (string, bool) {
	values, exists := request.URL.Query()["q"]
	if !exists || len(values) != 1 || len(request.URL.Query()) != 1 {
		writeJSONError(response, http.StatusBadRequest, "exactly one q query parameter is required")
		return "", false
	}
	query := strings.TrimSpace(values[0])
	if query == "" {
		writeJSONError(response, http.StatusBadRequest, "search query must not be empty")
		return "", false
	}
	if utf8.RuneCountInString(query) > maxSearchQueryRunes {
		writeJSONError(response, http.StatusBadRequest, "search query is too long")
		return "", false
	}
	return query, true
}

// searchDocument turns one document's bytes — already read through the
// shared resolveVisibleDocument boundary — into a searchable Document.
// Invalid frontmatter must not fail the whole scan: the sidebar
// still lists such documents (and /api/document answers 422 for them), so
// the projection degrades to the full source with empty description and
// tags and the frontmatter-derived title preference off — the document
// stays searchable, and opening it still lands on the existing 422 page.
func searchDocument(contents []byte, relativePath string) (search.Document, error) {
	source := contents
	var frontMatter *markdown.FrontMatter
	if body, metadata, err := markdown.ParseFrontMatter(contents); err == nil {
		source = body
		frontMatter = metadata
	}
	projection, err := markdown.ProjectForSearch(source, relativePath)
	if err != nil {
		return search.Document{}, err
	}
	document := search.Document{
		Path:   relativePath,
		Title:  markdown.PreferredTitle(frontMatter, projection.Title),
		Chunks: projection.Chunks,
	}
	if frontMatter != nil {
		document.Description = frontMatter.Description
		document.Tags = frontMatter.Tags
	}
	return document, nil
}
