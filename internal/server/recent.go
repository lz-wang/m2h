package server

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/markdown"
	"github.com/lz-wang/m2h/internal/store"
)

const (
	recentLimit      = 10
	recentTTL        = 30 * time.Second
	recentHeaderSize = 64 * 1024
)

type recentDocument struct {
	Path       string `json:"path"`
	Title      string `json:"title"`
	Date       string `json:"date"`
	DateSource string `json:"dateSource"`
}

type recentResponse struct {
	Created []recentDocument `json:"created"`
	Updated []recentDocument `json:"updated"`
}

type recentMetadata struct {
	created, updated recentDocument
	target           string
	info             fs.FileInfo
}

type recentCache struct {
	mu        sync.Mutex
	expiresAt time.Time
	files     map[string]recentMetadata
}

func (handler *documentHandler) serveRecent(w http.ResponseWriter, r *http.Request) {
	forceRefresh := r.URL.RawQuery == "refresh=true"
	if r.URL.RawQuery != "" && !forceRefresh {
		writeJSONError(w, http.StatusBadRequest, "only refresh=true is supported")
		return
	}
	result := recentResponse{Created: []recentDocument{}, Updated: []recentDocument{}}
	record, _ := r.Context().Value(tokenRecordContextKey{}).(store.TokenRecord)
	scope, scoped := handler.scopeOfRequestToken(r)
	var err error
	switch {
	case scoped && scope.Resource.Type == auth.ResourceDocument:
		var version *http.Request
		if record.DocumentHash != "" {
			// Derive metadata from the exact bytes that match the token.
			version = r
		}
		err = handler.recentScopedDocument(scope.Resource.RootID, scope.Resource.Path, version, &result)
	case record.DocumentHash == "":
		err = handler.recentLists(r, forceRefresh, &result)
	}
	if r.Context().Err() != nil {
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "read recent Markdown documents")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (handler *documentHandler) recentScopedDocument(rootID, relative string, version *http.Request, result *recentResponse) error {
	root, found := handler.workspace.rootByIdentity(rootID)
	if !found {
		return nil
	}
	virtual := handler.workspace.publicPath(root.id, relative)
	metadata, ok, err := handler.loadRecent(virtual, recentMetadata{}, version)
	if ok {
		result.Created = append(result.Created, metadata.created)
		if metadata.updated.Date != metadata.created.Date {
			result.Updated = append(result.Updated, metadata.updated)
		}
	}
	return err
}

func (handler *documentHandler) recentLists(r *http.Request, forceRefresh bool, result *recentResponse) error {
	snapshot, err := handler.recentSnapshot(r.Context(), forceRefresh)
	if err != nil {
		return err
	}
	filter := handler.tokenScopeFilter(r)
	candidates := make([]recentMetadata, 0, len(snapshot))
	for virtual, metadata := range snapshot {
		if filter(virtual) {
			candidates = append(candidates, metadata)
		}
	}
	// A candidate can make both lists. Reuse its live visibility check.
	visible := make(map[string]bool)
	for _, list := range []*[]recentDocument{&result.Created, &result.Updated} {
		document := func(metadata recentMetadata) recentDocument {
			if list == &result.Created {
				return metadata.created
			}
			return metadata.updated
		}
		slices.SortFunc(candidates, func(a, b recentMetadata) int {
			left, right := document(a), document(b)
			return cmp.Or(cmp.Compare(right.Date, left.Date),
				b.info.ModTime().Compare(a.info.ModTime()), cmp.Compare(left.Path, right.Path))
		})
		for _, candidate := range candidates {
			if len(*list) == recentLimit {
				break
			}
			if err := r.Context().Err(); err != nil {
				return err
			}
			if list == &result.Updated && candidate.updated.Date == candidate.created.Date {
				continue
			}
			entry := document(candidate)
			valid, checked := visible[entry.Path]
			if !checked {
				valid, err = handler.recentStillVisible(entry.Path, candidate)
				if err != nil {
					return err
				}
				visible[entry.Path] = valid
			}
			if valid {
				*list = append(*list, entry)
			}
		}
	}
	return nil
}

func (handler *documentHandler) recentSnapshot(ctx context.Context, forceRefresh bool) (map[string]recentMetadata, error) {
	cache := &handler.recent
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if !forceRefresh && time.Now().Before(cache.expiresAt) {
		return cache.files, nil
	}

	next := make(map[string]recentMetadata, len(cache.files))
	for _, root := range handler.workspace.roots {
		discovered, err := handler.discover(ctx, root.scope)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("discover recent Markdown files: %w", err)
		}
		for _, entry := range discovered.Markdown {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			virtual := handler.workspace.publicPath(root.id, entry.RelativePath)
			metadata, ok, err := handler.loadRecent(virtual, cache.files[virtual], nil)
			if err != nil {
				return nil, err
			}
			if ok {
				next[virtual] = metadata
			}
		}
	}
	cache.files = next
	cache.expiresAt = time.Now().Add(recentTTL)
	return next, nil
}

func (handler *documentHandler) recentStillVisible(virtual string, cached recentMetadata) (bool, error) {
	_, _, target, err := handler.resolveVisibleDocument(virtual)
	if err != nil || target != cached.target {
		return false, recentPolicyError(err)
	}
	file, err := os.Open(target)
	if err != nil {
		return false, nil
	}
	defer file.Close()
	info, err := file.Stat()
	return err == nil && sameRecentFile(cached.info, info), nil
}

func (handler *documentHandler) loadRecent(virtual string, cached recentMetadata, version *http.Request) (recentMetadata, bool, error) {
	_, relative, target, err := handler.resolveVisibleDocument(virtual)
	if err != nil {
		return recentMetadata{}, false, recentPolicyError(err)
	}
	file, err := os.Open(target)
	if err != nil {
		return recentMetadata{}, false, nil
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return recentMetadata{}, false, nil
	}
	if target == cached.target && sameRecentFile(cached.info, info) {
		return cached, true, nil
	}

	var body []byte
	var frontMatter *markdown.FrontMatter
	if version != nil {
		contents, err := io.ReadAll(file)
		if err != nil || !tokenContentMatches(version, contents) {
			return recentMetadata{}, false, nil
		}
		body, frontMatter = recentFrontMatter(contents)
	} else {
		body, frontMatter, err = readRecentFrontMatter(file)
		if err != nil {
			return recentMetadata{}, false, nil
		}
	}
	title := markdown.PreferredTitle(frontMatter, "")
	if title == "" {
		title, err = markdown.Title(body, relative)
		if err != nil {
			return recentMetadata{}, false, err
		}
	}
	after, err := file.Stat()
	if err != nil || !sameRecentFile(info, after) {
		return recentMetadata{}, false, nil
	}

	var created, updated string
	if frontMatter != nil {
		created, updated = frontMatter.CreatedDate, frontMatter.UpdatedDate
	}
	entry := func(date string) recentDocument {
		source := "frontmatter"
		if date == "" {
			date = info.ModTime().Format(time.DateOnly)
			source = "mtime"
		}
		return recentDocument{Path: virtual, Title: title, Date: date, DateSource: source}
	}
	return recentMetadata{created: entry(created), updated: entry(updated), info: info, target: target}, true, nil
}

func sameRecentFile(before, after fs.FileInfo) bool {
	return os.SameFile(before, after) && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

func recentPolicyError(err error) error {
	if errors.Is(err, errIgnoreUnavailable) {
		return err
	}
	return nil
}

func readRecentFrontMatter(file io.Reader) ([]byte, *markdown.FrontMatter, error) {
	header, err := io.ReadAll(io.LimitReader(file, recentHeaderSize+1))
	if err != nil {
		return nil, nil, err
	}
	if len(header) <= recentHeaderSize {
		body, metadata := recentFrontMatter(header)
		return body, metadata, nil
	}
	// A complete frontmatter title lets us skip the rest of the file. With
	// no title, parse the full source so a late H1 or link definition still
	// produces the same title as the reader.
	if end := bytes.LastIndexByte(header, '\n'); end >= 0 {
		_, metadata := recentFrontMatter(header[:end+1])
		if markdown.PreferredTitle(metadata, "") != "" {
			return nil, metadata, nil
		}
	}
	contents, err := io.ReadAll(io.MultiReader(bytes.NewReader(header), file))
	if err != nil {
		return nil, nil, err
	}
	body, metadata := recentFrontMatter(contents)
	return body, metadata, nil
}

func recentFrontMatter(contents []byte) ([]byte, *markdown.FrontMatter) {
	body, metadata, err := markdown.ParseFrontMatter(contents)
	if err != nil {
		return contents, nil
	}
	return body, metadata
}
