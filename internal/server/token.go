package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/store"
)

// Token API wire types.

type tokenResourceRequest struct {
	Type   string `json:"type"`
	RootID string `json:"rootId"`
	Path   string `json:"path"`
}

type tokenScopeRequest struct {
	Permissions []string             `json:"permissions"`
	Resource    tokenResourceRequest `json:"resource"`
}

type createTokenRequest struct {
	Kind          string            `json:"kind"`
	Name          string            `json:"name"`
	ExpiresInDays *float64          `json:"expiresInDays"`
	Scope         tokenScopeRequest `json:"scope"`
	DocumentHash  string            `json:"documentHash"`
}

type tokenResponse struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	Scope        tokenScopeRequest `json:"scope"`
	CreatedAt    int64             `json:"createdAt"`
	ExpiresAt    int64             `json:"expiresAt"`
	LastUsedAt   int64             `json:"lastUsedAt"`
	DocumentHash string            `json:"documentHash,omitempty"`
	// Secret carries the plaintext exactly once, on creation.
	Secret string `json:"secret,omitempty"`
}

// serveTokens dispatches the token collection endpoint (session-only).
func (handler *documentHandler) serveTokens(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		handler.serveListTokens(response, request)
	case http.MethodPost:
		handler.serveCreateToken(response, request)
	default:
		response.Header().Set("Allow", "GET, POST")
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (handler *documentHandler) serveTokenItem(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodDelete {
		response.Header().Set("Allow", http.MethodDelete)
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	handler.serveRevokeToken(response, request)
}

func tokenResponseFrom(record store.TokenRecord, scope auth.TokenScope) tokenResponse {
	converted := tokenScopeRequest{
		Permissions: scope.Permissions,
		Resource: tokenResourceRequest{
			Type:   scope.Resource.Type,
			RootID: scope.Resource.RootID,
			Path:   scope.Resource.Path,
		},
	}
	return tokenResponse{
		ID:           record.ID,
		Name:         record.Name,
		Kind:         record.Kind,
		Scope:        converted,
		CreatedAt:    record.CreatedAt,
		ExpiresAt:    record.ExpiresAt,
		LastUsedAt:   record.LastUsedAt,
		DocumentHash: record.DocumentHash,
	}
}

func (handler *documentHandler) serveListTokens(response http.ResponseWriter, request *http.Request) {
	identity := principalFromRequest(request)
	if identity == nil || identity.Via != principalViaSession {
		writeJSONError(response, http.StatusForbidden, "token management requires a browser session")
		return
	}
	records, err := handler.auth.dataStore.ListTokens(request.Context(), identity.ID)
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "list tokens")
		return
	}
	items := make([]tokenResponse, 0, len(records))
	for _, record := range records {
		scope, scopeErr := auth.UnmarshalScope(record.ScopeJSON)
		if scopeErr != nil {
			continue // a scope that no longer parses is dead weight, not a 500
		}
		items = append(items, tokenResponseFrom(record, scope))
	}
	writeJSON(response, http.StatusOK, struct {
		Tokens []tokenResponse `json:"tokens"`
	}{Tokens: items})
}

// serveCreateToken mints a token. Manual creation from the settings UI
// uses all_published; the "copy to agent" flow uses a document scope with
// an optional content-version binding.
func (handler *documentHandler) serveCreateToken(response http.ResponseWriter, request *http.Request) {
	identity := principalFromRequest(request)
	if identity == nil || identity.Via != principalViaSession {
		writeJSONError(response, http.StatusForbidden, "token management requires a browser session")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 16<<10)
	var body createTokenRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeJSONError(response, http.StatusBadRequest, "invalid token request")
		return
	}
	kind := auth.TokenKind(body.Kind)
	def, max, err := auth.TokenKindBounds(kind)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "kind must be temporary or persistent")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len(name) > 64 {
		writeJSONError(response, http.StatusBadRequest, "name is required (at most 64 characters)")
		return
	}
	lifetime := def
	if body.ExpiresInDays != nil {
		lifetime = time.Duration(*body.ExpiresInDays * float64(24*time.Hour))
		if lifetime <= 0 || lifetime > max {
			writeJSONError(response, http.StatusBadRequest, "expiresInDays is out of range for this kind")
			return
		}
	}

	scopePayload, err := json.Marshal(body.Scope)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "invalid token scope")
		return
	}
	scope, err := auth.ParseTokenScope(scopePayload)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, err.Error())
		return
	}
	// Root- and document-scoped tokens must name documents this workspace
	// actually serves — otherwise the scope is a stale reference at birth.
	if scope.Resource.Type != auth.ResourceAllPublished {
		root, _, resolveErr := handler.resolveAnnotationDocument(documentRefRequest{
			RootID: scope.Resource.RootID, Path: scope.Resource.Path,
		})
		if resolveErr != nil {
			writeJSONError(response, http.StatusBadRequest, "scope names a document this server does not serve")
			return
		}
		scope.Resource.RootID = root.identity
	}
	documentHash := ""
	if body.DocumentHash != "" {
		if !strings.HasPrefix(body.DocumentHash, "sha256:") {
			writeJSONError(response, http.StatusBadRequest, "documentHash must be a sha256: value")
			return
		}
		documentHash = body.DocumentHash
	}

	secret, hash, err := auth.GenerateTokenSecret()
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "generate token")
		return
	}
	scopeJSON, err := auth.MarshalScope(scope)
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "encode token scope")
		return
	}
	record := store.TokenRecord{
		SecretHash:   hash,
		Name:         name,
		Kind:         string(kind),
		OwnerID:      identity.ID,
		ScopeJSON:    scopeJSON,
		DocumentHash: documentHash,
		ExpiresAt:    time.Now().Add(lifetime).UnixMilli(),
	}
	if err := handler.auth.dataStore.CreateToken(request.Context(), &record); err != nil {
		writeJSONError(response, http.StatusInternalServerError, "create token")
		return
	}
	created := tokenResponseFrom(record, scope)
	created.Secret = secret
	writeJSON(response, http.StatusCreated, created)
}

func (handler *documentHandler) serveRevokeToken(response http.ResponseWriter, request *http.Request) {
	identity := principalFromRequest(request)
	if identity == nil || identity.Via != principalViaSession {
		writeJSONError(response, http.StatusForbidden, "token management requires a browser session")
		return
	}
	id := strings.TrimPrefix(request.URL.Path, "/api/tokens/")
	if id == "" || strings.Contains(id, "/") {
		writeJSONError(response, http.StatusBadRequest, "invalid token id")
		return
	}
	revoked, err := handler.auth.dataStore.RevokeToken(request.Context(), id, identity.ID)
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "revoke token")
		return
	}
	if !revoked {
		writeJSONError(response, http.StatusNotFound, "token not found")
		return
	}
	writeJSON(response, http.StatusOK, struct {
		OK bool `json:"ok"`
	}{OK: true})
}

// tokenLastUsedThrottle spreads last_used_at writes: a token hammered by
// an agent still writes at most once per interval.
const tokenLastUsedThrottle = time.Minute

// authorizeBearer validates an Authorization: Bearer credential and
// enforces its scope for the requested route. A presented bearer is the
// only authority — no fallback to cookies — and an invalid one is a 401.
func (handler *documentHandler) authorizeBearer(response http.ResponseWriter, request *http.Request, next http.Handler) {
	credential := strings.TrimSpace(strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer"))
	if credential == "" || !auth.LooksLikeTokenSecret(credential) {
		writeUnauthorized(response, request.URL.Path)
		return
	}
	record, err := handler.auth.dataStore.FindTokenByHash(request.Context(), auth.HashTokenSecret(credential))
	if err != nil {
		if errors.Is(err, store.ErrTokenNotFound) {
			writeUnauthorized(response, request.URL.Path)
			return
		}
		writeJSONError(response, http.StatusInternalServerError, "validate token")
		return
	}
	scope, err := auth.UnmarshalScope(record.ScopeJSON)
	if err != nil {
		writeUnauthorized(response, request.URL.Path)
		return
	}
	// The owning account must still belong to the mode this server runs —
	// a local account's token dies the moment the deployment switches to
	// LDAP, without waiting for anyone to revoke it.
	ownerProvider, _, _ := strings.Cut(record.OwnerID, ":")
	if ownerProvider != string(handler.auth.mode) {
		writeUnauthorized(response, request.URL.Path)
		return
	}

	path := request.URL.Path
	if bearerForbiddenPath(path) {
		writeJSONError(response, http.StatusForbidden, "tokens cannot call this endpoint")
		return
	}
	if err := handler.enforceTokenScope(response, request, path, scope, record); err != nil {
		return
	}

	identity := &principal{
		ID:          record.OwnerID,
		Username:    record.Name,
		DisplayName: record.OwnerID,
		Provider:    ownerProvider,
		Via:         principalViaToken,
	}
	next.ServeHTTP(response, requestWithPrincipal(request, identity))

	// Bookkeeping after the response: throttled last-used stamp.
	if time.Since(time.UnixMilli(record.LastUsedAt)) > tokenLastUsedThrottle {
		touchContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		_ = handler.auth.dataStore.TouchToken(touchContext, record.ID, time.Now())
		cancel()
	}
}

// bearerForbiddenPath lists the endpoints a token must never reach even
// with a valid credential: authentication itself and token management.
func bearerForbiddenPath(path string) bool {
	switch {
	case path == "/api/auth/login", path == "/api/auth/logout", path == "/api/auth/me":
		return true
	case path == "/api/tokens" || strings.HasPrefix(path, "/api/tokens/"):
		return true
	default:
		return false
	}
}

// enforceTokenScope applies the route-specific authorization. It answers
// the response itself when the scope falls short.
func (handler *documentHandler) enforceTokenScope(
	response http.ResponseWriter,
	request *http.Request,
	path string,
	scope auth.TokenScope,
	record store.TokenRecord,
) error {
	switch {
	case path == "/api/search":
		if !scope.Allows(auth.PermissionSearchRead) {
			writeJSONError(response, http.StatusForbidden, "token scope does not permit search")
			return errHandled
		}
		if scope.Resource.Type != auth.ResourceAllPublished {
			writeJSONError(response, http.StatusForbidden, "token scope does not cover workspace-wide search")
			return errHandled
		}
		return nil
	case path == "/api/files":
		if !scope.Allows(auth.PermissionDocumentRead) {
			writeJSONError(response, http.StatusForbidden, "token scope does not permit document listing")
			return errHandled
		}
		if scope.Resource.Type != auth.ResourceAllPublished {
			writeJSONError(response, http.StatusForbidden, "token scope does not cover the workspace listing")
			return errHandled
		}
		return nil
	case path == "/api/document", path == "/api/file-metadata", path == "/raw/", strings.HasPrefix(path, "/raw/"):
		return handler.enforceTokenDocumentScope(response, request, path, scope, record)
	case path == "/api/annotations":
		if request.Method != http.MethodGet {
			// Writes are session-only; the handler double-checks.
			return nil
		}
		return handler.enforceTokenDocumentScope(response, request, path, scope, record)
	case path == "/assets/" || strings.HasPrefix(path, "/assets/"):
		writeJSONError(response, http.StatusForbidden, "tokens cannot read attachments")
		return errHandled
	default:
		return nil
	}
}

// errHandled signals that the caller already answered the request.
var errHandled = errors.New("token scope handled the response")

// enforceTokenDocumentScope checks document-level reach for the
// document-reading routes: the scope must cover the addressed document,
// and a content-bound token must match the document's current hash (409
// otherwise).
func (handler *documentHandler) enforceTokenDocumentScope(
	response http.ResponseWriter,
	request *http.Request,
	path string,
	scope auth.TokenScope,
	record store.TokenRecord,
) error {
	if !scope.Allows(auth.PermissionDocumentRead) {
		writeJSONError(response, http.StatusForbidden, "token scope does not permit document reads")
		return errHandled
	}
	var virtual string
	switch {
	case path == "/api/document" || path == "/api/file-metadata" || path == "/api/annotations":
		value, ok := singlePathQuery(request)
		if !ok {
			writeJSONError(response, http.StatusBadRequest, "exactly one path query parameter is required")
			return errHandled
		}
		decoded, err := files.DecodeRelativePath(value)
		if err != nil {
			writeJSONError(response, http.StatusBadRequest, "invalid document path")
			return errHandled
		}
		virtual = decoded
	default:
		decoded, err := files.DecodeRelativePath(strings.TrimPrefix(path, "/raw/"))
		if err != nil {
			writeJSONError(response, http.StatusBadRequest, "invalid document path")
			return errHandled
		}
		virtual = decoded
	}
	root, relative, _, err := handler.resolveVisibleDocument(virtual)
	if err != nil {
		writeJSONError(response, resolveVisibleStatus(err), "document not found")
		return errHandled
	}
	if !scope.CoversDocument(root.identity, relative) {
		writeJSONError(response, http.StatusForbidden, "token scope does not cover this document")
		return errHandled
	}
	if record.DocumentHash != "" && (path == "/api/document" || strings.HasPrefix(path, "/raw/")) {
		current, err := handler.documentHash(root, relative)
		if err != nil {
			writeJSONError(response, resolveVisibleStatus(err), "document not found")
			return errHandled
		}
		if current != record.DocumentHash {
			writeJSONError(response, http.StatusConflict, "document_changed")
			return errHandled
		}
	}
	return nil
}
