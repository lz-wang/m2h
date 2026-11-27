package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
	"github.com/lz-wang/m2h/internal/store"
)

const (
	sharePagePrefix       = "/share/"
	shareDocumentPrefix   = "/api/public/shares/"
	shareManagementPrefix = "/api/shares/"
)

var shareDurations = map[string]time.Duration{
	"10m": 10 * time.Minute, "30m": 30 * time.Minute, "1h": time.Hour,
	"6h": 6 * time.Hour, "12h": 12 * time.Hour, "24h": 24 * time.Hour,
	"3d": 3 * 24 * time.Hour, "7d": 7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour, "90d": 90 * 24 * time.Hour, "permanent": 0,
}

type shareResponse struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	DocumentRef documentRefRequest `json:"documentRef"`
	CreatedAt   int64              `json:"createdAt"`
	ExpiresAt   int64              `json:"expiresAt"`
	URL         string             `json:"url"`
}

func requestLogURI(address *url.URL, basePath string) string {
	path := strings.TrimPrefix(address.Path, basePath)
	for _, prefix := range []string{shareDocumentPrefix, shareManagementPrefix, sharePagePrefix} {
		if strings.HasPrefix(path, prefix) {
			return basePath + prefix + "[redacted]"
		}
	}
	return address.RequestURI()
}

func validShareID(id string) bool {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(id)
	return err == nil && len(id) == 43 && len(decoded) == 32
}

func (stack *authStack) shareOwnerAllowed(share store.Share) bool {
	provider, _, _ := strings.Cut(share.OwnerID, ":")
	return share.OwnerName != "" && provider == string(stack.mode) &&
		stack.authorizer.IsPrincipalAllowed(auth.Principal{ID: share.OwnerID, Username: share.OwnerName, Provider: provider})
}

func (stack *authStack) pruneShareOwners(ctx context.Context, workspace workspace) error {
	records, err := stack.dataStore.ListShares(ctx, "", time.Now())
	if err != nil {
		return err
	}
	for _, record := range records {
		if _, served := workspace.rootByIdentity(record.RootID); served && !stack.shareOwnerAllowed(record) {
			if err := stack.dataStore.RevokeShare(ctx, record.OwnerID, record.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
	}
	return nil
}

func (handler *documentHandler) serveShares(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	identity := principalFromRequest(request)
	if identity == nil || identity.Via != principalViaSession {
		writeJSONError(response, http.StatusForbidden, "share management requires a browser session")
		return
	}
	switch request.Method {
	case http.MethodGet:
		records, err := handler.auth.dataStore.ListShares(request.Context(), identity.ID, time.Now())
		if err != nil {
			writeJSONError(response, http.StatusInternalServerError, "list document shares")
			return
		}
		items := make([]shareResponse, 0, len(records))
		for _, record := range records {
			items = append(items, handler.shareResponse(record))
		}
		writeJSON(response, http.StatusOK, items)
	case http.MethodPost:
		handler.createShare(response, request, identity)
	default:
		response.Header().Set("Allow", "GET, POST")
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (handler *documentHandler) createShare(response http.ResponseWriter, request *http.Request, identity *principal) {
	var body struct {
		DocumentRef documentRefRequest `json:"documentRef"`
		Duration    string             `json:"duration"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || decoder.Decode(new(any)) != io.EOF {
		writeJSONError(response, http.StatusBadRequest, "invalid share request")
		return
	}
	if body.Duration == "" {
		body.Duration = "30m"
	}
	duration, ok := shareDurations[body.Duration]
	if !ok {
		writeJSONError(response, http.StatusBadRequest, "invalid share duration")
		return
	}
	root, relative, _, err := handler.resolveShareDocument(body.DocumentRef)
	if err != nil {
		writeJSONError(response, resolveVisibleStatus(err), "document not found")
		return
	}

	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		writeJSONError(response, http.StatusInternalServerError, "generate share link")
		return
	}
	now := time.Now()
	candidate := store.Share{
		ID: base64.RawURLEncoding.EncodeToString(entropy[:]), OwnerID: identity.ID, OwnerName: identity.Username,
		RootID: root.identity, DocumentPath: relative, CreatedAt: now.UnixMilli(),
	}
	if duration != 0 {
		candidate.ExpiresAt = now.Add(duration).UnixMilli()
	}
	record, err := handler.auth.dataStore.CreateShare(request.Context(), candidate)
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "create document share")
		return
	}
	status := http.StatusOK
	if record.ID == candidate.ID {
		status = http.StatusCreated
	}
	writeJSON(response, status, handler.shareResponse(record))
}

func (handler *documentHandler) shareResponse(record store.Share) shareResponse {
	ref := documentRefRequest{RootID: record.RootID, Path: record.DocumentPath}
	title := filepath.Base(ref.Path)
	if root, _, target, err := handler.resolveShareDocument(ref); err == nil {
		if contents, err := readShareFile(root, target); err == nil {
			if body, metadata, err := markdown.ParseFrontMatter(contents); err == nil {
				if fallback, err := markdown.Title(body, ref.Path); err == nil {
					title = markdown.PreferredTitle(metadata, fallback)
				}
			}
		}
	}
	return shareResponse{
		ID: record.ID, Title: title, DocumentRef: ref, CreatedAt: record.CreatedAt, ExpiresAt: record.ExpiresAt,
		URL: handler.auth.publicOrigin + handler.basePath + sharePagePrefix + record.ID,
	}
}

func (handler *documentHandler) serveShareItem(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodDelete {
		response.Header().Set("Allow", http.MethodDelete)
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identity := principalFromRequest(request)
	if identity == nil || identity.Via != principalViaSession {
		writeJSONError(response, http.StatusForbidden, "share management requires a browser session")
		return
	}
	id := strings.TrimPrefix(request.URL.Path, shareManagementPrefix)
	if !validShareID(id) {
		writeJSONError(response, http.StatusNotFound, "share not found")
		return
	}
	err := handler.auth.dataStore.RevokeShare(request.Context(), identity.ID, id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeJSONError(response, http.StatusNotFound, "share not found")
	case err != nil:
		writeJSONError(response, http.StatusInternalServerError, "revoke document share")
	default:
		response.WriteHeader(http.StatusNoContent)
	}
}

func (handler *documentHandler) resolveShareDocument(ref documentRefRequest) (workspaceRoot, string, string, error) {
	root, ok := handler.workspace.rootByIdentity(ref.RootID)
	if !ok || ref.Path == "" || files.IsHiddenPath(ref.Path) {
		return workspaceRoot{}, "", "", fs.ErrNotExist
	}
	virtual := handler.workspace.publicPath(root.id, ref.Path)
	root, relative, target, err := handler.resolveVisibleDocument(virtual)
	if err != nil {
		return workspaceRoot{}, "", "", err
	}
	canonical, err := filepath.Rel(root.scope.root, target)
	if err != nil || files.IsHiddenPath(filepath.ToSlash(canonical)) {
		return workspaceRoot{}, "", "", fs.ErrNotExist
	}
	return root, relative, canonical, nil
}

func setShareHeaders(response http.ResponseWriter) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("X-Robots-Tag", "noindex, nofollow")
}

func (handler *documentHandler) loadPublicShare(request *http.Request, id string) (store.Share, workspaceRoot, string, error) {
	if !validShareID(id) {
		return store.Share{}, workspaceRoot{}, "", fs.ErrNotExist
	}
	record, err := handler.auth.dataStore.FindShare(request.Context(), id, time.Now())
	if errors.Is(err, sql.ErrNoRows) || err == nil && !handler.auth.shareOwnerAllowed(record) {
		return store.Share{}, workspaceRoot{}, "", fs.ErrNotExist
	}
	if err != nil {
		return store.Share{}, workspaceRoot{}, "", err
	}
	root, _, target, err := handler.resolveShareDocument(documentRefRequest{RootID: record.RootID, Path: record.DocumentPath})
	if err != nil && !errors.Is(err, errIgnoreUnavailable) {
		err = fs.ErrNotExist
	}
	return record, root, target, err
}

func shareStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if errors.Is(err, fs.ErrNotExist) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func (handler *documentHandler) serveSharedPage(response http.ResponseWriter, request *http.Request) {
	setShareHeaders(response)
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, route, nested := strings.Cut(strings.TrimPrefix(request.URL.EscapedPath(), sharePagePrefix), "/")
	if nested {
		encoded, ok := strings.CutPrefix(route, "assets/")
		if !ok {
			http.NotFound(response, request)
			return
		}
		handler.serveSharedAsset(response, request, id, encoded)
		return
	}
	_, _, _, err := handler.loadPublicShare(request, id)
	status := shareStatus(err)
	if status == http.StatusInternalServerError {
		http.Error(response, "read document share", status)
		return
	}
	index, err := fs.ReadFile(handler.ui, "index.html")
	if err != nil {
		http.Error(response, "read embedded WebUI", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(status)
	if request.Method == http.MethodGet {
		_, _ = response.Write(renderIndexHTML(index, handler.basePath, handler.cdn))
	}
}

type publicDocumentResponse struct {
	Title string             `json:"title"`
	HTML  string             `json:"html"`
	TOC   []tocEntryResponse `json:"toc"`
}

func (handler *documentHandler) serveSharedDocument(response http.ResponseWriter, request *http.Request) {
	setShareHeaders(response)
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", "GET")
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(request.URL.EscapedPath(), shareDocumentPrefix)
	document, _, err := handler.renderPublicShare(request, id)
	if err != nil {
		status := shareStatus(err)
		writeJSONError(response, status, http.StatusText(status))
		return
	}
	writeJSON(response, http.StatusOK, document)
}

func readShareFile(root workspaceRoot, target string) ([]byte, error) {
	base, err := os.OpenRoot(root.scope.root)
	if err != nil {
		return nil, err
	}
	defer base.Close()
	return base.ReadFile(target)
}

func (handler *documentHandler) renderPublicShare(request *http.Request, id string) (publicDocumentResponse, map[string]sharedAsset, error) {
	record, root, target, err := handler.loadPublicShare(request, id)
	if err != nil {
		return publicDocumentResponse{}, nil, err
	}
	contents, err := readShareFile(root, target)
	if err != nil {
		return publicDocumentResponse{}, nil, err
	}
	body, metadata, err := markdown.ParseFrontMatter(contents)
	if err != nil {
		return publicDocumentResponse{}, nil, fmt.Errorf("parse shared document: %w", err)
	}
	resources := make(map[string]sharedAsset)
	rendered, err := markdown.RenderShare(body, record.DocumentPath, func(relative string) string {
		asset, found := resources[relative]
		if !found {
			asset = resolveSharedAsset(root, relative)
			resources[relative] = asset
		}
		if asset.target == "" {
			return ""
		}
		escaped := (&url.URL{Path: "/" + relative}).EscapedPath()
		return handler.basePath + sharePagePrefix + id + "/assets" + strings.ReplaceAll(escaped, ",", "%2C")
	})
	if err != nil {
		return publicDocumentResponse{}, nil, err
	}
	return publicDocumentResponse{Title: markdown.PreferredTitle(metadata, rendered.Title), HTML: rendered.Body,
		TOC: tocEntriesFrom(rendered.Headings)}, resources, nil
}

type sharedAsset struct {
	root   string
	target string
}

func resolveSharedAsset(root workspaceRoot, relative string) sharedAsset {
	admitted, err := root.scope.assetAdmittedByPolicy(relative)
	if err != nil || !admitted || files.IsHiddenPath(relative) {
		return sharedAsset{}
	}
	resolved, err := resolveRequestFile(root.scope.root, relative)
	if err != nil || files.IsHiddenPath(resolved.relative) {
		return sharedAsset{}
	}
	admitted, err = root.scope.assetAdmittedByPolicy(resolved.relative)
	if err != nil || !admitted {
		return sharedAsset{}
	}
	return sharedAsset{root: root.scope.root, target: filepath.FromSlash(resolved.relative)}
}

func (handler *documentHandler) serveSharedAsset(response http.ResponseWriter, request *http.Request, id, path string) {
	relative, err := url.PathUnescape(path)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	_, resources, err := handler.renderPublicShare(request, id)
	if err != nil {
		status := shareStatus(err)
		http.Error(response, http.StatusText(status), status)
		return
	}
	asset := resources[relative]
	if asset.target == "" {
		http.NotFound(response, request)
		return
	}
	root, err := os.OpenRoot(asset.root)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	defer root.Close()
	file, err := root.Open(asset.target)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(response, request)
		return
	}
	response.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	http.ServeContent(response, request, info.Name(), info.ModTime(), file)
}
