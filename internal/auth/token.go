package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// TokenKind separates the two token lifecycles: temporary shares (minutes)
// and persistent agent credentials (months).
type TokenKind string

const (
	TokenTemporary  TokenKind = "temporary"
	TokenPersistent TokenKind = "persistent"
)

// Token permissions. The first version grants read-only knowledge-base
// access; annotation writes are session-only by construction.
const (
	PermissionDocumentRead = "document:read"
	PermissionSearchRead   = "search:read"
)

// Resource types scope a token's reach: every published document, one
// root's tree, or exactly one document.
const (
	ResourceAllPublished = "all_published"
	ResourceRoot         = "root"
	ResourceDocument     = "document"
)

// TokenScope is the authorization envelope of one token.
type TokenScope struct {
	Permissions []string `json:"permissions"`
	Resource    struct {
		Type   string `json:"type"`
		RootID string `json:"rootId,omitempty"`
		Path   string `json:"path,omitempty"`
	} `json:"resource"`
}

// TokenExpiry bounds finite lifetimes. Persistent tokens can also be permanent.
const (
	TemporaryTokenDefault  = 30 * time.Minute
	TemporaryTokenMax      = 24 * time.Hour
	PersistentTokenDefault = 180 * 24 * time.Hour
	PersistentTokenMax     = 365 * 24 * time.Hour
)

// tokenPrefix marks m2h-issued bearer secrets so unrelated Authorization
// schemes fail fast.
const tokenPrefix = "m2h_"

// ErrInvalidScope marks rejected scope payloads.
var ErrInvalidScope = errors.New("invalid token scope")

// ParseTokenScope validates the wire form of a scope.
func ParseTokenScope(payload []byte) (TokenScope, error) {
	var scope TokenScope
	if err := json.Unmarshal(payload, &scope); err != nil {
		return TokenScope{}, fmt.Errorf("%w: malformed JSON", ErrInvalidScope)
	}
	if len(scope.Permissions) == 0 {
		return TokenScope{}, fmt.Errorf("%w: at least one permission is required", ErrInvalidScope)
	}
	seen := make(map[string]bool, len(scope.Permissions))
	for _, permission := range scope.Permissions {
		if permission != PermissionDocumentRead && permission != PermissionSearchRead {
			return TokenScope{}, fmt.Errorf("%w: unknown permission %q", ErrInvalidScope, permission)
		}
		if seen[permission] {
			return TokenScope{}, fmt.Errorf("%w: duplicate permission %q", ErrInvalidScope, permission)
		}
		seen[permission] = true
	}
	switch scope.Resource.Type {
	case ResourceAllPublished:
		if scope.Resource.RootID != "" || scope.Resource.Path != "" {
			return TokenScope{}, fmt.Errorf("%w: all_published takes no resource ids", ErrInvalidScope)
		}
	case ResourceRoot:
		if scope.Resource.RootID == "" || scope.Resource.Path != "" {
			return TokenScope{}, fmt.Errorf("%w: root scope requires rootId", ErrInvalidScope)
		}
	case ResourceDocument:
		if scope.Resource.RootID == "" || scope.Resource.Path == "" {
			return TokenScope{}, fmt.Errorf("%w: document scope requires rootId and path", ErrInvalidScope)
		}
	default:
		return TokenScope{}, fmt.Errorf("%w: unknown resource type %q", ErrInvalidScope, scope.Resource.Type)
	}
	return scope, nil
}

// Allows reports whether the scope grants one permission.
func (scope TokenScope) Allows(permission string) bool {
	return slices.Contains(scope.Permissions, permission)
}

// CoversDocument reports whether the scope reaches one root-relative
// document. all_published reaches everything; root reaches its own tree;
// document reaches exactly the named file.
func (scope TokenScope) CoversDocument(rootID, path string) bool {
	switch scope.Resource.Type {
	case ResourceAllPublished:
		return true
	case ResourceRoot:
		return scope.Resource.RootID == rootID
	case ResourceDocument:
		return scope.Resource.RootID == rootID && scope.Resource.Path == path
	default:
		return false
	}
}

// MarshalScope encodes the scope for storage.
func MarshalScope(scope TokenScope) (string, error) {
	encoded, err := json.Marshal(scope)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// UnmarshalScope decodes a stored scope; a scope that was valid when
// written stays valid now (unknown parts were rejected at creation).
func UnmarshalScope(raw string) (TokenScope, error) {
	var scope TokenScope
	if err := json.Unmarshal([]byte(raw), &scope); err != nil {
		return TokenScope{}, fmt.Errorf("stored token scope: %w", err)
	}
	return scope, nil
}

// TokenKindBounds returns the default and maximum lifetime of one kind.
func TokenKindBounds(kind TokenKind) (def, max time.Duration, err error) {
	switch kind {
	case TokenTemporary:
		return TemporaryTokenDefault, TemporaryTokenMax, nil
	case TokenPersistent:
		return PersistentTokenDefault, PersistentTokenMax, nil
	default:
		return 0, 0, fmt.Errorf("invalid token kind %q", string(kind))
	}
}

// GenerateTokenSecret mints one 256-bit secret in its presentation form
// ("m2h_" + base64url). The database never sees the plaintext.
func GenerateTokenSecret() (secret, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate token secret: %w", err)
	}
	secret = tokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return secret, HashTokenSecret(secret), nil
}

// HashTokenSecret derives the storage key of a presented secret.
func HashTokenSecret(secret string) string {
	digest := sha256.Sum256([]byte(secret))
	return fmt.Sprintf("%x", digest)
}

// LooksLikeTokenSecret reports whether an Authorization bearer value has
// m2h's token shape.
func LooksLikeTokenSecret(secret string) bool {
	return strings.HasPrefix(secret, tokenPrefix) && len(secret) > len(tokenPrefix)
}
