package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/lz-wang/m2h/internal/annotation"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/store"
)

// annotation API wire types. The selector shape mirrors the persisted
// model one-to-one; comments travel as plain text and are rendered as
// plain text by the WebUI.
type annotationSelectorRequest struct {
	Quote struct {
		Exact  string `json:"exact"`
		Prefix string `json:"prefix"`
		Suffix string `json:"suffix"`
	} `json:"quote"`
	Position struct {
		Start int `json:"start"`
		End   int `json:"end"`
	} `json:"position"`
	HeadingID string `json:"headingId"`
	TabID     string `json:"tabId"`
	BlockHint string `json:"blockHint"`
	Version   int    `json:"version"`
}

type documentRefRequest struct {
	RootID string `json:"rootId"`
	Path   string `json:"path"`
}

type createAnnotationRequest struct {
	DocumentRef  documentRefRequest        `json:"documentRef"`
	DocumentHash string                    `json:"documentHash"`
	Selector     annotationSelectorRequest `json:"selector"`
	Comment      string                    `json:"comment"`
	Color        annotation.Color          `json:"color"`
}

type annotationResponse struct {
	ID           string                    `json:"id"`
	AuthorID     string                    `json:"authorId"`
	AuthorName   string                    `json:"authorName"`
	Comment      string                    `json:"comment"`
	Color        annotation.Color          `json:"color"`
	Quote        string                    `json:"quote"`
	Selector     annotationSelectorRequest `json:"selector"`
	DocumentHash string                    `json:"documentHash"`
	// Mine marks the caller's own annotation so the UI can offer deletion
	// without learning principal ids.
	Mine      bool  `json:"mine"`
	CreatedAt int64 `json:"createdAt"`
	UpdatedAt int64 `json:"updatedAt"`
}

func selectorFromRequest(request annotationSelectorRequest) annotation.Selector {
	return annotation.Selector{
		Quote: annotation.QuoteSelector{
			Exact:  request.Quote.Exact,
			Prefix: request.Quote.Prefix,
			Suffix: request.Quote.Suffix,
		},
		Position: annotation.PositionSelector{
			Start: request.Position.Start,
			End:   request.Position.End,
		},
		HeadingID: request.HeadingID,
		TabID:     request.TabID,
		BlockHint: request.BlockHint,
		Version:   request.Version,
	}
}

func selectorResponse(selector annotation.Selector) annotationSelectorRequest {
	var response annotationSelectorRequest
	response.Quote.Exact = selector.Quote.Exact
	response.Quote.Prefix = selector.Quote.Prefix
	response.Quote.Suffix = selector.Quote.Suffix
	response.Position.Start = selector.Position.Start
	response.Position.End = selector.Position.End
	response.HeadingID = selector.HeadingID
	response.TabID = selector.TabID
	response.BlockHint = selector.BlockHint
	response.Version = selector.Version
	return response
}

func annotationResponseFrom(entry annotation.Annotation, callerID string) annotationResponse {
	return annotationResponse{
		ID:           entry.ID,
		AuthorID:     entry.AuthorID,
		AuthorName:   entry.AuthorName,
		Comment:      entry.Comment,
		Color:        entry.Color,
		Quote:        entry.Selector.Quote.Exact,
		Selector:     selectorResponse(entry.Selector),
		DocumentHash: entry.DocumentHash,
		Mine:         entry.AuthorID == callerID,
		CreatedAt:    entry.CreatedAt,
		UpdatedAt:    entry.UpdatedAt,
	}
}

// annotationStore returns the persistence layer, or nil when the run has
// none (the default mode registers no annotation routes, so this only
// guards against construction mistakes).
func (handler *documentHandler) annotationStore() *store.Store {
	if handler.auth == nil || handler.auth.dataStore == nil {
		return nil
	}
	return handler.auth.dataStore
}

// resolveAnnotationDocument maps a stored document reference onto the
// currently served workspace. Unknown roots, unpublished paths and
// traversal attempts all collapse to a plain miss: the caller learns
// nothing about why.
func (handler *documentHandler) resolveAnnotationDocument(ref documentRefRequest) (workspaceRoot, string, error) {
	if ref.RootID == "" || ref.Path == "" {
		return workspaceRoot{}, "", errors.New("document reference is required")
	}
	root, found := handler.workspace.rootByIdentity(ref.RootID)
	if !found {
		return workspaceRoot{}, "", errors.New("document root is not served")
	}
	virtual := handler.workspace.publicPath(root.id, ref.Path)
	resolved, relative, _, err := handler.resolveVisibleDocument(virtual)
	if err != nil {
		return workspaceRoot{}, "", err
	}
	return resolved, relative, nil
}

// serveAnnotations dispatches the collection endpoint: GET lists one
// document's annotations, POST creates one.
func (handler *documentHandler) serveAnnotations(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
	case http.MethodPost:
		handler.serveCreateAnnotation(response, request)
		return
	default:
		response.Header().Set("Allow", "GET, POST")
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	dataStore := handler.annotationStore()
	if dataStore == nil {
		writeJSONError(response, http.StatusServiceUnavailable, "annotations are not available")
		return
	}
	value, ok := singlePathQuery(request)
	if !ok {
		writeJSONError(response, http.StatusBadRequest, "exactly one path query parameter is required")
		return
	}
	virtual, err := files.DecodeRelativePath(value)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "invalid document path")
		return
	}
	root, relative, _, err := handler.resolveVisibleDocument(virtual)
	if err != nil {
		writeJSONError(response, resolveVisibleStatus(err), "document not found")
		return
	}
	entries, err := dataStore.ListAnnotations(request.Context(), root.identity, relative)
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "list annotations")
		return
	}
	caller := principalFromRequest(request)
	callerID := ""
	if caller != nil {
		callerID = caller.ID
	}
	items := make([]annotationResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, annotationResponseFrom(entry, callerID))
	}
	writeJSON(response, http.StatusOK, struct {
		Annotations []annotationResponse `json:"annotations"`
	}{Annotations: items})
}

// serveCreateAnnotation answers POST /api/annotations. The author comes
// from the session — never from the request body — and the document must
// be served, visible and at exactly the version the client annotated, or
// the creation fails with 409 document_changed.
func (handler *documentHandler) serveCreateAnnotation(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identity := principalFromRequest(request)
	if identity == nil {
		writeJSONError(response, http.StatusUnauthorized, "authentication required")
		return
	}
	// Tokens may read annotations (search/reader scope) but never write
	// them: annotation authorship is a session-human act.
	if identity.Via != principalViaSession {
		writeJSONError(response, http.StatusForbidden, "annotations can only be created from a browser session")
		return
	}
	dataStore := handler.annotationStore()
	if dataStore == nil {
		writeJSONError(response, http.StatusServiceUnavailable, "annotations are not available")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 64<<10)
	var body createAnnotationRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeJSONError(response, http.StatusBadRequest, "invalid annotation request")
		return
	}
	selector := selectorFromRequest(body.Selector)
	if err := annotation.ValidateSelector(selector); err != nil {
		writeJSONError(response, http.StatusBadRequest, err.Error())
		return
	}
	if err := annotation.ValidateComment(body.Comment); err != nil {
		writeJSONError(response, http.StatusBadRequest, err.Error())
		return
	}
	color := body.Color.WithDefault()
	if !color.Valid() {
		writeJSONError(response, http.StatusBadRequest, "annotation color must be blue, green, or amber")
		return
	}
	if !strings.HasPrefix(body.DocumentHash, "sha256:") {
		writeJSONError(response, http.StatusBadRequest, "document hash is required")
		return
	}
	root, relative, err := handler.resolveAnnotationDocument(body.DocumentRef)
	if err != nil {
		writeJSONError(response, http.StatusNotFound, "document not found")
		return
	}
	hash, err := handler.documentHash(root, relative)
	if err != nil {
		writeJSONError(response, resolveVisibleStatus(err), "document not found")
		return
	}
	if hash != body.DocumentHash {
		writeJSONError(response, http.StatusConflict, "document_changed")
		return
	}
	entry := annotation.Annotation{
		RootID:       root.identity,
		DocumentPath: relative,
		AuthorID:     identity.ID,
		AuthorName:   identity.DisplayName,
		Selector:     selector,
		Comment:      body.Comment,
		Color:        color,
		DocumentHash: hash,
	}
	if err := dataStore.CreateAnnotation(request.Context(), &entry); err != nil {
		writeJSONError(response, http.StatusInternalServerError, "create annotation")
		return
	}
	writeJSON(response, http.StatusCreated, annotationResponseFrom(entry, identity.ID))
}

// serveDeleteAnnotation answers DELETE /api/annotations/{id}. Authors may
// delete their own annotations only; the annotation's document must still
// be published, so a hidden document's annotations cannot be probed.
func (handler *documentHandler) serveDeleteAnnotation(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodDelete {
		response.Header().Set("Allow", http.MethodDelete)
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identity := principalFromRequest(request)
	if identity == nil {
		writeJSONError(response, http.StatusUnauthorized, "authentication required")
		return
	}
	if identity.Via != principalViaSession {
		writeJSONError(response, http.StatusForbidden, "annotations can only be deleted from a browser session")
		return
	}
	dataStore := handler.annotationStore()
	if dataStore == nil {
		writeJSONError(response, http.StatusServiceUnavailable, "annotations are not available")
		return
	}
	id := strings.TrimPrefix(request.URL.Path, "/api/annotations/")
	if id == "" || strings.Contains(id, "/") {
		writeJSONError(response, http.StatusBadRequest, "invalid annotation id")
		return
	}
	entry, err := dataStore.GetAnnotation(request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrAnnotationNotFound) {
			writeJSONError(response, http.StatusNotFound, "annotation not found")
			return
		}
		writeJSONError(response, http.StatusInternalServerError, "load annotation")
		return
	}
	// The document must still be served and visible — a hidden or removed
	// document's annotations are unreachable, whatever their id.
	if _, _, err := handler.resolveAnnotationDocument(documentRefRequest{
		RootID: entry.RootID, Path: entry.DocumentPath,
	}); err != nil {
		writeJSONError(response, http.StatusNotFound, "annotation not found")
		return
	}
	if entry.AuthorID != identity.ID {
		writeJSONError(response, http.StatusForbidden, "only the author can delete this annotation")
		return
	}
	if _, err := dataStore.DeleteAnnotation(request.Context(), id, identity.ID); err != nil {
		writeJSONError(response, http.StatusInternalServerError, "delete annotation")
		return
	}
	writeJSON(response, http.StatusOK, struct {
		OK bool `json:"ok"`
	}{OK: true})
}

// serveAnnotationItem routes the per-annotation endpoints:
// DELETE /api/annotations/{id} (phase 3) and PATCH
// /api/annotations/{id}/anchor (phase 4's human re-anchoring).
func (handler *documentHandler) serveAnnotationItem(response http.ResponseWriter, request *http.Request) {
	remainder := strings.TrimPrefix(request.URL.Path, "/api/annotations/")
	if id, ok := strings.CutSuffix(remainder, "/comment"); ok && id != "" && !strings.Contains(id, "/") {
		if request.Method != http.MethodPatch {
			response.Header().Set("Allow", http.MethodPatch)
			writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handler.serveUpdateAnnotationComment(response, request, id)
		return
	}
	if id, ok := strings.CutSuffix(remainder, "/anchor"); ok && id != "" {
		if request.Method != http.MethodPatch {
			response.Header().Set("Allow", http.MethodPatch)
			writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handler.serveReanchorAnnotation(response, request, id)
		return
	}
	handler.serveDeleteAnnotation(response, request)
}

func (handler *documentHandler) serveUpdateAnnotationComment(response http.ResponseWriter, request *http.Request, id string) {
	identity := principalFromRequest(request)
	if identity == nil || identity.Via != principalViaSession {
		writeJSONError(response, http.StatusForbidden, "annotation editing requires a browser session")
		return
	}
	dataStore := handler.annotationStore()
	if dataStore == nil {
		writeJSONError(response, http.StatusServiceUnavailable, "annotations are not available")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 64<<10)
	var body struct {
		Comment string            `json:"comment"`
		Color   *annotation.Color `json:"color"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeJSONError(response, http.StatusBadRequest, "invalid annotation request")
		return
	}
	if err := annotation.ValidateComment(body.Comment); err != nil {
		writeJSONError(response, http.StatusBadRequest, err.Error())
		return
	}
	if body.Color != nil && !body.Color.Valid() {
		writeJSONError(response, http.StatusBadRequest, "annotation color must be blue, green, or amber")
		return
	}
	entry, err := dataStore.GetAnnotation(request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrAnnotationNotFound) {
			writeJSONError(response, http.StatusNotFound, "annotation not found")
		} else {
			writeJSONError(response, http.StatusInternalServerError, "load annotation")
		}
		return
	}
	if _, _, err := handler.resolveAnnotationDocument(documentRefRequest{RootID: entry.RootID, Path: entry.DocumentPath}); err != nil {
		writeJSONError(response, http.StatusNotFound, "annotation not found")
		return
	}
	if entry.AuthorID != identity.ID {
		writeJSONError(response, http.StatusForbidden, "only the author can edit this annotation")
		return
	}
	if err := dataStore.UpdateAnnotationComment(request.Context(), id, identity.ID, body.Comment, body.Color); err != nil {
		if errors.Is(err, store.ErrAnnotationNotFound) {
			writeJSONError(response, http.StatusNotFound, "annotation not found")
		} else {
			writeJSONError(response, http.StatusInternalServerError, "update annotation")
		}
		return
	}
	entry, err = dataStore.GetAnnotation(request.Context(), id)
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "load annotation")
		return
	}
	writeJSON(response, http.StatusOK, annotationResponseFrom(entry, identity.ID))
}

// serveReanchorAnnotation answers PATCH /api/annotations/{id}/anchor: the
// author confirms a new selector for an annotation whose old anchor no
// longer resolves. The stored selector and document hash move to the
// current document version in one update.
func (handler *documentHandler) serveReanchorAnnotation(response http.ResponseWriter, request *http.Request, id string) {
	identity := principalFromRequest(request)
	if identity == nil {
		writeJSONError(response, http.StatusUnauthorized, "authentication required")
		return
	}
	if identity.Via != principalViaSession {
		writeJSONError(response, http.StatusForbidden, "annotations can only be changed from a browser session")
		return
	}
	dataStore := handler.annotationStore()
	if dataStore == nil {
		writeJSONError(response, http.StatusServiceUnavailable, "annotations are not available")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 64<<10)
	var body struct {
		Selector annotationSelectorRequest `json:"selector"`
		// DocumentHash states which version the client re-selected text
		// on. It must match the served document, or the new selector
		// would be stored against a version it never addressed.
		DocumentHash string `json:"documentHash"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeJSONError(response, http.StatusBadRequest, "invalid annotation request")
		return
	}
	selector := selectorFromRequest(body.Selector)
	if err := annotation.ValidateSelector(selector); err != nil {
		writeJSONError(response, http.StatusBadRequest, err.Error())
		return
	}
	entry, err := dataStore.GetAnnotation(request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrAnnotationNotFound) {
			writeJSONError(response, http.StatusNotFound, "annotation not found")
			return
		}
		writeJSONError(response, http.StatusInternalServerError, "load annotation")
		return
	}
	if entry.AuthorID != identity.ID {
		writeJSONError(response, http.StatusForbidden, "only the author can re-anchor this annotation")
		return
	}
	root, relative, err := handler.resolveAnnotationDocument(documentRefRequest{
		RootID: entry.RootID, Path: entry.DocumentPath,
	})
	if err != nil {
		writeJSONError(response, http.StatusNotFound, "annotation not found")
		return
	}
	if !strings.HasPrefix(body.DocumentHash, "sha256:") {
		writeJSONError(response, http.StatusBadRequest, "document hash is required")
		return
	}
	hash, err := handler.documentHash(root, relative)
	if err != nil {
		writeJSONError(response, resolveVisibleStatus(err), "document not found")
		return
	}
	if hash != body.DocumentHash {
		writeJSONError(response, http.StatusConflict, "document_changed")
		return
	}
	if err := dataStore.UpdateAnnotationSelector(request.Context(), id, identity.ID, selector, hash); err != nil {
		// The conditional UPDATE can lose its own race: the annotation
		// deleted between the load above and this statement is a plain
		// miss, not a server fault.
		if errors.Is(err, store.ErrAnnotationNotFound) {
			writeJSONError(response, http.StatusNotFound, "annotation not found")
			return
		}
		writeJSONError(response, http.StatusInternalServerError, "re-anchor annotation")
		return
	}
	updated, err := dataStore.GetAnnotation(request.Context(), id)
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, "load annotation")
		return
	}
	writeJSON(response, http.StatusOK, annotationResponseFrom(updated, identity.ID))
}

// documentHash reads one root-relative document and returns the
// sha256:… digest of its raw Markdown bytes.
func (handler *documentHandler) documentHash(root workspaceRoot, relative string) (string, error) {
	virtual := handler.workspace.publicPath(root.id, relative)
	_, _, target, err := handler.resolveVisibleDocument(virtual)
	if err != nil {
		return "", err
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(contents)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
