package auth

import (
	"errors"
	"testing"
	"time"
)

func TestParseTokenScope(t *testing.T) {
	valid := []string{
		`{"permissions":["document:read","search:read"],"resource":{"type":"all_published"}}`,
		`{"permissions":["document:read"],"resource":{"type":"root","rootId":"r-1"}}`,
		`{"permissions":["document:read"],"resource":{"type":"document","rootId":"r-1","path":"a.md"}}`,
	}
	for _, payload := range valid {
		if _, err := ParseTokenScope([]byte(payload)); err != nil {
			t.Fatalf("ParseTokenScope(%s) failed: %v", payload, err)
		}
	}
	invalid := map[string]string{
		"no permissions":        `{"permissions":[],"resource":{"type":"all_published"}}`,
		"unknown permission":    `{"permissions":["document:write"],"resource":{"type":"all_published"}}`,
		"duplicate permission":  `{"permissions":["document:read","document:read"],"resource":{"type":"all_published"}}`,
		"unknown resource type": `{"permissions":["document:read"],"resource":{"type":"wildcard"}}`,
		"all_published with id": `{"permissions":["document:read"],"resource":{"type":"all_published","rootId":"r"}}`,
		"root without id":       `{"permissions":["document:read"],"resource":{"type":"root"}}`,
		"document without path": `{"permissions":["document:read"],"resource":{"type":"document","rootId":"r"}}`,
		"malformed JSON":        `{`,
	}
	for name, payload := range invalid {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseTokenScope([]byte(payload)); !errors.Is(err, ErrInvalidScope) {
				t.Fatalf("ParseTokenScope(%s) = %v, want ErrInvalidScope", payload, err)
			}
		})
	}
}

func TestTokenScopeCoverage(t *testing.T) {
	var all TokenScope
	all.Permissions = []string{PermissionDocumentRead, PermissionSearchRead}
	all.Resource.Type = ResourceAllPublished

	var root TokenScope
	root.Permissions = []string{PermissionDocumentRead}
	root.Resource.Type = ResourceRoot
	root.Resource.RootID = "r-1"

	var document TokenScope
	document.Permissions = []string{PermissionDocumentRead}
	document.Resource.Type = ResourceDocument
	document.Resource.RootID = "r-1"
	document.Resource.Path = "design.md"

	if !all.CoversDocument("r-1", "x.md") || !all.CoversDocument("r-2", "y.md") {
		t.Fatalf("all_published must cover everything")
	}
	if !root.CoversDocument("r-1", "anything.md") || root.CoversDocument("r-2", "anything.md") {
		t.Fatalf("root scope must cover exactly its tree")
	}
	if !document.CoversDocument("r-1", "design.md") ||
		document.CoversDocument("r-1", "other.md") ||
		document.CoversDocument("r-2", "design.md") {
		t.Fatalf("document scope must cover exactly its document")
	}
	if all.Allows(PermissionSearchRead) && !document.Allows(PermissionSearchRead) {
		// expected split
	} else {
		t.Fatalf("permission check broken")
	}
}

func TestTokenKindBounds(t *testing.T) {
	def, max, err := TokenKindBounds(TokenTemporary)
	if err != nil || def != 30*time.Minute || max != 24*time.Hour {
		t.Fatalf("temporary bounds = %v %v %v", def, max, err)
	}
	def, max, err = TokenKindBounds(TokenPersistent)
	if err != nil || def != 180*24*time.Hour || max != 365*24*time.Hour {
		t.Fatalf("persistent bounds = %v %v %v", def, max, err)
	}
	if _, _, err := TokenKindBounds("eternal"); err == nil {
		t.Fatalf("unknown kind accepted")
	}
}

func TestTokenSecretRoundTrip(t *testing.T) {
	secret, hash, err := GenerateTokenSecret()
	if err != nil {
		t.Fatalf("GenerateTokenSecret failed: %v", err)
	}
	if !LooksLikeTokenSecret(secret) {
		t.Fatalf("secret %q lacks the m2h_ shape", secret)
	}
	if HashTokenSecret(secret) != hash {
		t.Fatalf("hash is not derivable from the secret")
	}
	if secret == hash || len(hash) != 64 {
		t.Fatalf("hash shape wrong: %q", hash)
	}
	another, _, _ := GenerateTokenSecret()
	if another == secret {
		t.Fatalf("secrets repeat")
	}
	if LooksLikeTokenSecret("bearer abc") || LooksLikeTokenSecret("m2h_") {
		t.Fatalf("non-token credentials accepted as tokens")
	}
}

func TestScopeMarshalRoundTrip(t *testing.T) {
	var scope TokenScope
	scope.Permissions = []string{PermissionDocumentRead}
	scope.Resource.Type = ResourceDocument
	scope.Resource.RootID = "r-1"
	scope.Resource.Path = "a.md"
	encoded, err := MarshalScope(scope)
	if err != nil {
		t.Fatalf("MarshalScope failed: %v", err)
	}
	back, err := UnmarshalScope(encoded)
	if err != nil || !back.CoversDocument("r-1", "a.md") || !back.Allows(PermissionDocumentRead) {
		t.Fatalf("round trip lost scope: %+v %v", back, err)
	}
}
