package server

import (
	"errors"
	"net/http"
	"os"
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

	// Candidates are validated in rank order until 50 of them survive —
	// never truncated to 50 first. A cached entry whose file was deleted,
	// re-hidden, re-pointed or rewritten must drop out of the response, and
	// the documents ranked behind it still get their chance to fill the
	// page.
	results := make([]searchResultResponse, 0, min(len(matches), maxSearchResults))
	for _, match := range matches {
		if len(results) == maxSearchResults {
			break
		}
		if request.Context().Err() != nil {
			return
		}
		valid, err := handler.searchResultValid(snapshot, match.Path)
		if err != nil {
			writeJSONError(response, http.StatusInternalServerError, "evaluate ignore rules")
			return
		}
		if !valid {
			continue
		}
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

// searchResultValid re-checks one candidate against the publishing boundary
// before its cached projection reaches the response. The snapshot may be
// older than the filesystem, so the entry must still resolve through the
// same resolveVisibleDocument boundary /api/document answers from, to the
// same canonical target with the same size and mtime the projection was
// built from. A miss on any of those means the snippet in hand describes a
// file state that no longer exists, and the result is dropped rather than
// served stale. Only an ignore-rule failure is a server fault; everything
// else is the document routes' 404 answer: simply absent.
func (handler *documentHandler) searchResultValid(snapshot *searchSnapshot, publicPath string) (bool, error) {
	indexed, exists := snapshot.Files[publicPath]
	if !exists {
		return false, nil
	}
	_, _, target, err := handler.resolveVisibleDocument(publicPath)
	if err != nil {
		if errors.Is(err, errIgnoreUnavailable) {
			return false, err
		}
		return false, nil
	}
	if target != indexed.Target {
		return false, nil
	}
	info, err := os.Stat(target)
	if err != nil || info.Size() != indexed.Size || !info.ModTime().Equal(indexed.ModTime) {
		return false, nil
	}
	return true, nil
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
