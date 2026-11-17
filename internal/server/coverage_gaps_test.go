package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lz-wang/m2h/internal/annotation"
	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/markdown"
	"github.com/lz-wang/m2h/internal/store"
)

// buildTestHandler assembles a minimal authenticated handler for direct
// branch coverage of handlers whose error paths the middleware normally
// screens off.
func buildTestHandler(t *testing.T) (*documentHandler, *store.Store) {
	t.Helper()
	base := t.TempDir()
	docs := filepath.Join(base, "docs")
	writeTestFile(t, filepath.Join(docs, "guide.md"), "# Guide")
	resolved, err := files.Resolve(docs)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := newWorkspace([]files.Input{resolved}, files.DiscoverOptions{}, false)
	if err != nil {
		t.Fatal(err)
	}
	dataStore, err := store.Open(filepath.Join(base, "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dataStore.Close() })
	if err := dataStore.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := workspace.assignIdentities(context.Background(), dataStore.EnsureDocumentRoot); err != nil {
		t.Fatal(err)
	}
	stack, err := buildAuthStack(localAuthConfig(), dataStore, "")
	if err != nil {
		t.Fatal(err)
	}
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte(`<div id="root"></div>`)}}
	handler := &documentHandler{
		workspace:  workspace,
		ui:         ui,
		version:    "test",
		basePath:   "",
		auth:       stack,
		runContext: context.Background(),
	}
	handler.discover = func(ctx context.Context, scope rootScope) (files.Discovery, error) {
		return scope.discover(ctx)
	}
	return handler, dataStore
}

func TestHandlerBranchesWithoutMiddleware(t *testing.T) {
	handler, _ := buildTestHandler(t)

	tests := []struct {
		name   string
		method string
		target string
		body   string
		want   int
	}{
		{name: "me without principal", method: http.MethodGet, target: "/api/auth/me", want: http.StatusUnauthorized},
		{name: "logout without principal is guarded by origin", method: http.MethodPost, target: "/api/auth/logout", want: http.StatusForbidden},
		{name: "logout method guard", method: http.MethodGet, target: "/api/auth/logout", want: http.StatusMethodNotAllowed},
		{name: "me method guard", method: http.MethodPost, target: "/api/auth/me", want: http.StatusMethodNotAllowed},
		{name: "me method guard with origin", method: http.MethodPost, target: "/api/auth/me", want: http.StatusMethodNotAllowed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			handler.serveAuthMe(recorder, request)
			handler.serveLogout(recorder, request)
			// The specific handler under test is chosen by target.
			recorder = httptest.NewRecorder()
			request = httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			switch {
			case strings.HasPrefix(test.target, "/api/auth/me"):
				handler.serveAuthMe(recorder, request)
			case strings.HasPrefix(test.target, "/api/auth/logout"):
				handler.serveLogout(recorder, request)
			case strings.HasPrefix(test.target, "/api/annotations"):
				handler.serveAnnotations(recorder, request)
			}
			if recorder.Code != test.want {
				t.Fatalf("%s %s = %d, want %d", test.method, test.target, recorder.Code, test.want)
			}
		})
	}

	// Without the store the annotation endpoints refuse early.
	noStore := &documentHandler{workspace: handler.workspace, auth: &authStack{mode: auth.ModeLocal, dataStore: nil}}
	var recorder *httptest.ResponseRecorder
	var request *http.Request
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/annotations?path=guide.md", nil)
	request = requestWithPrincipal(request, &principal{ID: "local:alice", Via: principalViaSession})
	noStore.serveAnnotations(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("list without store = %d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/annotations", strings.NewReader("{}"))
	request = requestWithPrincipal(request, &principal{ID: "local:alice", Via: principalViaSession})
	noStore.serveCreateAnnotation(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("create without store = %d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/annotations/x", nil)
	request = requestWithPrincipal(request, &principal{ID: "local:alice", Via: principalViaSession})
	noStore.serveDeleteAnnotation(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("delete without store = %d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/annotations/x", nil)
	noStore.serveDeleteAnnotation(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("delete without principal = %d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPatch, "/api/annotations/x/anchor", strings.NewReader("{}"))
	request = requestWithPrincipal(request, &principal{ID: "local:alice", Via: principalViaSession})
	noStore.serveReanchorAnnotation(recorder, request, "x")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("re-anchor without store = %d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPatch, "/api/annotations/x/anchor", strings.NewReader("{}"))
	noStore.serveReanchorAnnotation(recorder, request, "x")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("re-anchor without principal = %d", recorder.Code)
	}
	// The method guards of the item router.
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/annotations/x", nil)
	handler.serveAnnotationItem(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("item router wrong method = %d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/api/annotations/x/anchor", nil)
	handler.serveAnnotationItem(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("anchor wrong method = %d", recorder.Code)
	}
}

func TestReanchorAnnotationBranches(t *testing.T) {
	handler, dataStore := buildTestHandler(t)
	root := handler.workspace.primary()

	validSelector := annotation.Selector{
		Quote:    annotation.QuoteSelector{Exact: "Guide", Prefix: "", Suffix: ""},
		Position: annotation.PositionSelector{Start: 2, End: 7},
		Version:  1,
	}
	owned := annotation.Annotation{
		RootID: root.identity, DocumentPath: "guide.md",
		AuthorID: "local:alice", AuthorName: "alice",
		Selector: validSelector, DocumentHash: "sha256:x",
	}
	if err := dataStore.CreateAnnotation(context.Background(), &owned); err != nil {
		t.Fatal(err)
	}

	post := func(id, body string, principal *principal) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPatch, "/api/annotations/"+id+"/anchor", strings.NewReader(body))
		if principal != nil {
			request = requestWithPrincipal(request, principal)
		}
		handler.serveReanchorAnnotation(recorder, request, id)
		return recorder
	}

	alice := &principal{ID: "local:alice", Via: principalViaSession, Provider: "local"}
	bob := &principal{ID: "local:bob", Via: principalViaSession, Provider: "local"}

	if recorder := post("no-such", `{"selector":{"quote":{"exact":"G"},"position":{"start":2,"end":3},"version":1}}`, alice); recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown re-anchor = %d", recorder.Code)
	}
	if recorder := post(owned.ID, `not json`, alice); recorder.Code != http.StatusBadRequest {
		t.Fatalf("malformed re-anchor = %d", recorder.Code)
	}
	if recorder := post(owned.ID, `{"selector":{"quote":{"exact":"G"},"position":{"start":2,"end":9},"version":1}}`, alice); recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid selector re-anchor = %d", recorder.Code)
	}
	if recorder := post(owned.ID, `{"selector":{"quote":{"exact":"G"},"position":{"start":2,"end":3},"version":1}}`, bob); recorder.Code != http.StatusForbidden {
		t.Fatalf("foreign re-anchor = %d", recorder.Code)
	}

	// A bearer principal may not re-anchor even as the author.
	tokenPrincipal := &principal{ID: "local:alice", Via: principalViaToken, Provider: "local"}
	if recorder := post(owned.ID, `{"selector":{"quote":{"exact":"G"},"position":{"start":2,"end":3},"version":1}}`, tokenPrincipal); recorder.Code != http.StatusForbidden {
		t.Fatalf("bearer re-anchor = %d", recorder.Code)
	}

	// An annotation on a document that left the workspace is a plain miss.
	gone := annotation.Annotation{
		RootID: "not-served", DocumentPath: "gone.md",
		AuthorID: "local:alice", AuthorName: "alice",
		Selector: validSelector, DocumentHash: "sha256:x",
	}
	// The unserved root must still exist for the foreign key.
	goneRootID, err := dataStore.EnsureDocumentRoot(context.Background(), filepath.Join(t.TempDir(), "gone"), store.RootKindDirectory)
	if err != nil {
		t.Fatal(err)
	}
	gone.RootID = goneRootID
	if err := dataStore.CreateAnnotation(context.Background(), &gone); err != nil {
		t.Fatal(err)
	}
	if recorder := post(gone.ID, `{"selector":{"quote":{"exact":"G"},"position":{"start":2,"end":3},"version":1}}`, alice); recorder.Code != http.StatusNotFound {
		t.Fatalf("re-anchor on unserved document = %d", recorder.Code)
	}
}

func TestDeleteAnnotationErrorBranches(t *testing.T) {
	handler, dataStore := buildTestHandler(t)
	root := handler.workspace.primary()

	entry := annotation.Annotation{
		RootID: root.identity, DocumentPath: "guide.md",
		AuthorID: "local:alice", AuthorName: "alice",
		Selector: annotation.Selector{
			Quote:    annotation.QuoteSelector{Exact: "Guide"},
			Position: annotation.PositionSelector{Start: 2, End: 7},
			Version:  1,
		},
		DocumentHash: "sha256:x",
	}
	if err := dataStore.CreateAnnotation(context.Background(), &entry); err != nil {
		t.Fatal(err)
	}

	del := func(target string, principal *principal) int {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, target, nil)
		if principal != nil {
			request = requestWithPrincipal(request, principal)
		}
		handler.serveDeleteAnnotation(recorder, request)
		return recorder.Code
	}
	alice := &principal{ID: "local:alice", Via: principalViaSession, Provider: "local"}

	if code := del("/api/annotations/not-an-id", alice); code != http.StatusNotFound {
		t.Fatalf("unknown annotation = %d", code)
	}
	// Deleting a hidden document's annotation answers as a miss.
	writeTestFile(t, filepath.Join(handler.workspace.primary().scope.root, "hidden.md"), "# Hidden")
	if code := del("/api/annotations/slash/id", alice); code != http.StatusBadRequest {
		t.Fatalf("slash id = %d", code)
	}
	if code := del("/api/annotations/"+entry.ID, nil); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated delete = %d", code)
	}
}

func TestDocumentHashRefusesMissingDocument(t *testing.T) {
	handler, _ := buildTestHandler(t)
	root := handler.workspace.primary()
	if _, err := handler.documentHash(root, "missing.md"); err == nil {
		t.Fatalf("hash of a missing document succeeded")
	}
}

func TestResolveVisibleStatusIgnoreFailure(t *testing.T) {
	if resolveVisibleStatus(errors.Join(errIgnoreUnavailable)) != http.StatusInternalServerError {
		t.Fatalf("ignore failure must be a server fault")
	}
	if resolveVisibleStatus(errors.New("nope")) != http.StatusNotFound {
		t.Fatalf("plain miss must 404")
	}
	void(markdown.ModeAuto)
}

func void(_ ...any) {}
