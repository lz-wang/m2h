package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// annotationFixture logs two users in and captures the document identity
// the authenticated /api/document serves.
type annotationFixture struct {
	*authedTestServer
	document struct {
		Ref  documentRefResponse `json:"documentRef"`
		Hash string              `json:"documentHash"`
	}
	alice *http.Cookie
	bob   *http.Cookie
}

func newAnnotationFixture(t *testing.T) *annotationFixture {
	t.Helper()
	fixture := &annotationFixture{authedTestServer: newAuthedTestServer(t, localAuthConfig())}
	fixture.alice = fixture.loginAs(t, "alice", "password-1")
	fixture.bob = fixture.loginAs(t, "bob", "password-2")

	request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/document?path=guide.md", nil)
	request.AddCookie(fixture.alice)
	response := fixture.do(request)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("document = %d", response.StatusCode)
	}
	body, _ := io.ReadAll(response.Body)
	if err := json.Unmarshal(body, &fixture.document); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if fixture.document.Ref.RootID == "" || fixture.document.Ref.Path != "guide.md" ||
		!strings.HasPrefix(fixture.document.Hash, "sha256:") {
		t.Fatalf("document identity missing from authenticated response: %s", body)
	}
	return fixture
}

func (fixture *annotationFixture) createAnnotation(t *testing.T, session *http.Cookie, comment string) *http.Response {
	t.Helper()
	payload := map[string]any{
		"documentRef":  fixture.document.Ref,
		"documentHash": fixture.document.Hash,
		"selector": map[string]any{
			"quote":    map[string]any{"exact": "Body text", "prefix": "# Guide\n\n", "suffix": ""},
			"position": map[string]any{"start": 10, "end": 19},
			"version":  1,
		},
		"comment": comment,
	}
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/annotations", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", fixture.server.URL)
	if session != nil {
		request.AddCookie(session)
	}
	return fixture.do(request)
}

func decodeAnnotations(t *testing.T, response *http.Response) []annotationResponse {
	t.Helper()
	defer response.Body.Close()
	var payload struct {
		Annotations []annotationResponse `json:"annotations"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode annotations: %v", err)
	}
	return payload.Annotations
}

func TestAnnotationCreateListDeleteLifecycle(t *testing.T) {
	fixture := newAnnotationFixture(t)

	created := fixture.createAnnotation(t, fixture.alice, "需要补充备份方案")
	if created.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(created.Body)
		created.Body.Close()
		t.Fatalf("create = %d %s", created.StatusCode, body)
	}
	var entry annotationResponse
	if err := json.NewDecoder(created.Body).Decode(&entry); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	created.Body.Close()
	if entry.ID == "" || entry.AuthorName != "alice" || !entry.Mine || entry.Quote != "Body text" {
		t.Fatalf("created annotation = %+v", entry)
	}

	// Both users see the annotation; only alice owns it.
	list := func(session *http.Cookie) []annotationResponse {
		request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/annotations?path=guide.md", nil)
		request.AddCookie(session)
		response := fixture.do(request)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("list = %d", response.StatusCode)
		}
		return decodeAnnotations(t, response)
	}
	aliceList := list(fixture.alice)
	bobList := list(fixture.bob)
	if len(aliceList) != 1 || len(bobList) != 1 {
		t.Fatalf("list counts = %d / %d", len(aliceList), len(bobList))
	}
	if !aliceList[0].Mine || bobList[0].Mine {
		t.Fatalf("mine flags wrong: alice=%v bob=%v", aliceList[0].Mine, bobList[0].Mine)
	}

	// Bob cannot delete alice's annotation.
	deleteRequest, _ := http.NewRequest(http.MethodDelete, fixture.server.URL+"/api/annotations/"+entry.ID, nil)
	deleteRequest.AddCookie(fixture.bob)
	deleteRequest.Header.Set("Origin", fixture.server.URL)
	forbidden := fixture.do(deleteRequest)
	forbidden.Body.Close()
	if forbidden.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign delete = %d, want 403", forbidden.StatusCode)
	}

	// Alice deletes her own.
	own, _ := http.NewRequest(http.MethodDelete, fixture.server.URL+"/api/annotations/"+entry.ID, nil)
	own.AddCookie(fixture.alice)
	own.Header.Set("Origin", fixture.server.URL)
	deleted := fixture.do(own)
	deleted.Body.Close()
	if deleted.StatusCode != http.StatusOK {
		t.Fatalf("delete = %d", deleted.StatusCode)
	}
	if remaining := list(fixture.alice); len(remaining) != 0 {
		t.Fatalf("annotation survived delete: %+v", remaining)
	}
}

func TestAnnotationCommentEditingAuthorization(t *testing.T) {
	fixture := newAnnotationFixture(t)
	created := fixture.createAnnotation(t, fixture.alice, "原评论")
	var entry annotationResponse
	if err := json.NewDecoder(created.Body).Decode(&entry); err != nil {
		t.Fatal(err)
	}
	created.Body.Close()
	for _, test := range []struct {
		name    string
		session *http.Cookie
		want    int
	}{{"other author", fixture.bob, http.StatusForbidden}, {"author", fixture.alice, http.StatusOK}} {
		t.Run(test.name, func(t *testing.T) {
			request, _ := http.NewRequest(http.MethodPatch, fixture.server.URL+"/api/annotations/"+entry.ID+"/comment", strings.NewReader(`{"comment":"新评论"}`))
			request.Header.Set("Origin", fixture.server.URL)
			request.AddCookie(test.session)
			response := fixture.do(request)
			defer response.Body.Close()
			if response.StatusCode != test.want {
				t.Fatalf("status = %d", response.StatusCode)
			}
		})
	}
	updated, err := fixture.stack.dataStore.GetAnnotation(context.Background(), entry.ID)
	if err != nil || updated.Comment != "新评论" {
		t.Fatalf("updated = %+v, %v", updated, err)
	}
}

func TestAnnotationCommentEditingValidation(t *testing.T) {
	fixture := newAnnotationFixture(t)
	created := fixture.createAnnotation(t, fixture.alice, "原评论")
	var entry annotationResponse
	if err := json.NewDecoder(created.Body).Decode(&entry); err != nil {
		t.Fatal(err)
	}
	created.Body.Close()
	for _, test := range []struct {
		name, id, method, body string
		want                   int
	}{
		{"invalid JSON", entry.ID, http.MethodPatch, "{", http.StatusBadRequest},
		{"long comment", entry.ID, http.MethodPatch, `{"comment":"` + strings.Repeat("x", 10001) + `"}`, http.StatusBadRequest},
		{"missing", "missing", http.MethodPatch, `{"comment":"x"}`, http.StatusNotFound},
		{"wrong method", entry.ID, http.MethodPost, `{"comment":"x"}`, http.StatusMethodNotAllowed},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, _ := http.NewRequest(test.method, fixture.server.URL+"/api/annotations/"+test.id+"/comment", strings.NewReader(test.body))
			request.Header.Set("Origin", fixture.server.URL)
			request.AddCookie(fixture.alice)
			response := fixture.do(request)
			defer response.Body.Close()
			if response.StatusCode != test.want {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.want)
			}
		})
	}
}

func TestAnnotationCommentEditingUnavailableDocument(t *testing.T) {
	fixture := newAnnotationFixture(t)
	created := fixture.createAnnotation(t, fixture.alice, "原评论")
	var entry annotationResponse
	if err := json.NewDecoder(created.Body).Decode(&entry); err != nil {
		t.Fatal(err)
	}
	created.Body.Close()
	if err := os.Remove(filepath.Join(fixture.docsDir, "guide.md")); err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodPatch, fixture.server.URL+"/api/annotations/"+entry.ID+"/comment", strings.NewReader(`{"comment":"new"}`))
	request.Header.Set("Origin", fixture.server.URL)
	request.AddCookie(fixture.alice)
	response := fixture.do(request)
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestAnnotationCommentRequiresStoreAndSession(t *testing.T) {
	handler := &documentHandler{}
	request := httptest.NewRequest(http.MethodPatch, "/api/annotations/a/comment", strings.NewReader(`{"comment":"x"}`))
	response := httptest.NewRecorder()
	handler.serveUpdateAnnotationComment(response, request, "a")
	if response.Code != http.StatusForbidden {
		t.Fatalf("anonymous = %d", response.Code)
	}
	request = requestWithPrincipal(request, &principal{ID: "local:alice", Via: principalViaSession})
	response = httptest.NewRecorder()
	handler.serveUpdateAnnotationComment(response, request, "a")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("no store = %d", response.Code)
	}
}

func TestAnnotationCreateValidatesInput(t *testing.T) {
	fixture := newAnnotationFixture(t)

	tests := []struct {
		name   string
		mutate func(payload map[string]any)
	}{
		{name: "bad selector version", mutate: func(p map[string]any) {
			p["selector"].(map[string]any)["version"] = 99
		}},
		{name: "empty quote", mutate: func(p map[string]any) {
			p["selector"].(map[string]any)["quote"].(map[string]any)["exact"] = ""
		}},
		{name: "span mismatch", mutate: func(p map[string]any) {
			p["selector"].(map[string]any)["position"].(map[string]any)["end"] = 999
		}},
		{name: "missing hash", mutate: func(p map[string]any) {
			p["documentHash"] = ""
		}},
		{name: "unknown root", mutate: func(p map[string]any) {
			p["documentRef"] = documentRefResponse{RootID: "not-a-known-root", Path: "guide.md"}
		}},
		{name: "unknown document", mutate: func(p map[string]any) {
			p["documentRef"] = documentRefResponse{RootID: fixture.document.Ref.RootID, Path: "missing.md"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := map[string]any{
				"documentRef":  fixture.document.Ref,
				"documentHash": fixture.document.Hash,
				"selector": map[string]any{
					"quote":    map[string]any{"exact": "Body text", "prefix": "", "suffix": ""},
					"position": map[string]any{"start": 10, "end": 19},
					"version":  1,
				},
				"comment": "x",
			}
			test.mutate(payload)
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/annotations", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", fixture.server.URL)
			request.AddCookie(fixture.alice)
			response := fixture.do(request)
			response.Body.Close()
			if response.StatusCode == http.StatusCreated {
				t.Fatalf("invalid annotation accepted")
			}
		})
	}
}

func TestAnnotationCreateRejectsStaleDocument(t *testing.T) {
	fixture := newAnnotationFixture(t)

	// Edit the document behind the server's back.
	guide := fixture.serverDocument(t)
	writeTestFile(t, guide, "# Guide\n\nBody text edited.")

	response := fixture.createAnnotation(t, fixture.alice, "stale")
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("stale create = %d, want 409", response.StatusCode)
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil || payload.Error != "document_changed" {
		t.Fatalf("conflict body = %+v (err %v)", payload, err)
	}
}

func TestAnnotationsRequireAuthentication(t *testing.T) {
	fixture := newAnnotationFixture(t)

	list, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/annotations?path=guide.md", nil)
	anonymousList := fixture.do(list)
	anonymousList.Body.Close()
	if anonymousList.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous list = %d", anonymousList.StatusCode)
	}

	create := fixture.createAnnotation(t, nil, "x")
	create.Body.Close()
	if create.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous create = %d", create.StatusCode)
	}
}

func TestAnnotationDeleteRequiresSameOrigin(t *testing.T) {
	fixture := newAnnotationFixture(t)
	created := fixture.createAnnotation(t, fixture.alice, "csrf probe")
	var entry annotationResponse
	_ = json.NewDecoder(created.Body).Decode(&entry)
	created.Body.Close()

	request, _ := http.NewRequest(http.MethodDelete, fixture.server.URL+"/api/annotations/"+entry.ID, nil)
	request.AddCookie(fixture.alice)
	response := fixture.do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin delete = %d, want 403", response.StatusCode)
	}
}

func TestAnnotationReanchorByAuthor(t *testing.T) {
	fixture := newAnnotationFixture(t)
	created := fixture.createAnnotation(t, fixture.alice, "reanchor me")
	var entry annotationResponse
	if err := json.NewDecoder(created.Body).Decode(&entry); err != nil {
		t.Fatal(err)
	}
	created.Body.Close()

	// Bob may not re-anchor alice's annotation.
	foreign, _ := json.Marshal(map[string]any{"selector": map[string]any{
		"quote":    map[string]any{"exact": "Guide", "prefix": "", "suffix": ""},
		"position": map[string]any{"start": 2, "end": 7},
		"version":  1,
	}})
	foreignRequest, _ := http.NewRequest(http.MethodPatch,
		fixture.server.URL+"/api/annotations/"+entry.ID+"/anchor", bytes.NewReader(foreign))
	foreignRequest.Header.Set("Content-Type", "application/json")
	foreignRequest.Header.Set("Origin", fixture.server.URL)
	foreignRequest.AddCookie(fixture.bob)
	forbidden := fixture.do(foreignRequest)
	forbidden.Body.Close()
	if forbidden.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign re-anchor = %d", forbidden.StatusCode)
	}

	own, _ := json.Marshal(map[string]any{
		"documentHash": fixture.document.Hash,
		"selector": map[string]any{
			"quote":    map[string]any{"exact": "Guide", "prefix": "", "suffix": ""},
			"position": map[string]any{"start": 2, "end": 7},
			"version":  1,
		},
	})
	request, _ := http.NewRequest(http.MethodPatch,
		fixture.server.URL+"/api/annotations/"+entry.ID+"/anchor", bytes.NewReader(own))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", fixture.server.URL)
	request.AddCookie(fixture.alice)
	response := fixture.do(request)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("re-anchor = %d", response.StatusCode)
	}
	var updated annotationResponse
	if err := json.NewDecoder(response.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Selector.Quote.Exact != "Guide" || updated.Quote != "Guide" {
		t.Fatalf("re-anchored selector = %+v", updated.Selector)
	}
}

// TestAnnotationReanchorRequiresCurrentDocumentHash pins the optimistic
// concurrency contract of the human re-anchor flow: the client states
// which version it re-selected text on, and a document that moved under
// that selection is a 409, never a silently inconsistent stored anchor.
func TestAnnotationReanchorRequiresCurrentDocumentHash(t *testing.T) {
	fixture := newAnnotationFixture(t)
	created := fixture.createAnnotation(t, fixture.alice, "reanchor hash")
	var entry annotationResponse
	if err := json.NewDecoder(created.Body).Decode(&entry); err != nil {
		t.Fatal(err)
	}
	created.Body.Close()

	selectorPayload := func() map[string]any {
		return map[string]any{
			"quote":    map[string]any{"exact": "Guide", "prefix": "", "suffix": ""},
			"position": map[string]any{"start": 2, "end": 7},
			"version":  1,
		}
	}
	patch := func(body map[string]any) *http.Response {
		encoded, _ := json.Marshal(body)
		request := fixture.requestWithSession(
			http.MethodPatch, "/api/annotations/"+entry.ID+"/anchor", fixture.alice, encoded)
		return fixture.do(request)
	}

	// A re-anchor without a document version is malformed.
	response := patch(map[string]any{"selector": selectorPayload()})
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("hash-less re-anchor = %d, want 400", response.StatusCode)
	}

	// A re-anchor from a stale version conflicts instead of storing a
	// selector bound to the wrong hash.
	stale := map[string]any{
		"selector":     selectorPayload(),
		"documentHash": "sha256:" + strings.Repeat("0", 64),
	}
	conflict := patch(stale)
	defer conflict.Body.Close()
	if conflict.StatusCode != http.StatusConflict {
		t.Fatalf("stale re-anchor = %d, want 409", conflict.StatusCode)
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(conflict.Body).Decode(&payload); err != nil || payload.Error != "document_changed" {
		t.Fatalf("conflict body = %+v (err %v)", payload, err)
	}

	// The current version anchors cleanly.
	current := map[string]any{
		"selector":     selectorPayload(),
		"documentHash": fixture.document.Hash,
	}
	accepted := patch(current)
	defer accepted.Body.Close()
	if accepted.StatusCode != http.StatusOK {
		t.Fatalf("current re-anchor = %d, want 200", accepted.StatusCode)
	}
}

// serverDocument returns the fixture's guide.md path so tests can edit the
// file behind the running server.
func (fixture *annotationFixture) serverDocument(t *testing.T) string {
	t.Helper()
	return fixture.docsDir + "/guide.md"
}
