package server

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/lz-wang/m2h/internal/files"
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
	Path    string                 `json:"path"`
	Title   string                 `json:"title"`
	Snippet string                 `json:"snippet,omitempty"`
	Heading *searchHeadingResponse `json:"heading,omitempty"`
}

type searchResponse struct {
	Query   string                 `json:"query"`
	Results []searchResultResponse `json:"results"`
}

// serveSearch answers GET /api/search?q=<query> with a workspace-wide
// full-text scan. Like /api/files there is no index and no watcher: the
// request discovers the currently publishable files, reads and projects
// them, and ranks the matches — the answer always reflects the disk state
// at request time. Scope rules are inherited unchanged from serveFiles, so
// --glob, --depth, hidden paths, single-file scopes and multi-root
// identities behave exactly as the document routes do.
func (handler *documentHandler) serveSearch(response http.ResponseWriter, request *http.Request) {
	query, ok := searchQuery(request, response)
	if !ok {
		return
	}

	matches := make([]search.Result, 0)
	for _, root := range handler.workspace.roots {
		if request.Context().Err() != nil {
			return
		}
		discovered, err := handler.discover(request.Context(), root.scope)
		if err != nil {
			// A cancelled context surfaces as a discover error first; the
			// client is gone, so stop without answering.
			if request.Context().Err() != nil {
				return
			}
			if errors.Is(err, fs.ErrNotExist) {
				discovered = files.Discovery{}
			} else {
				writeJSONError(response, http.StatusInternalServerError, "discover Markdown files")
				return
			}
		}
		for _, entry := range discovered.Markdown {
			// The client aborts superseded queries; walking the rest of the
			// workspace to build a response nobody reads is wasted work.
			if request.Context().Err() != nil {
				return
			}
			// Discovery only records candidates; the read re-enters the same
			// resolveVisibleDocument boundary /api/document and /raw use, so
			// an entry that vanished, was re-hidden or now escapes its root
			// after discovery is simply absent from this scan — exactly the
			// answer the document routes give for the same file.
			virtual := handler.workspace.publicPath(root.id, entry.RelativePath)
			_, relative, target, err := handler.resolveVisibleDocument(virtual)
			if err != nil {
				continue
			}
			contents, err := os.ReadFile(target)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				writeJSONError(response, http.StatusInternalServerError, "search Markdown documents")
				return
			}
			document, err := searchDocument(contents, relative)
			if err != nil {
				writeJSONError(response, http.StatusInternalServerError, "search Markdown documents")
				return
			}
			if result, matched := search.Match(document, query); matched {
				result.Path = handler.workspace.publicPath(root.id, document.Path)
				matches = append(matches, result)
			}
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
