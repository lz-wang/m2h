package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lz-wang/m2h/internal/auth"
)

func tokenFixture(owner string, expiresIn time.Duration) storeTokenFixture {
	secret, hash, _ := auth.GenerateTokenSecret()
	scope, _ := auth.MarshalScope(func() auth.TokenScope {
		var scope auth.TokenScope
		scope.Permissions = []string{auth.PermissionDocumentRead}
		scope.Resource.Type = auth.ResourceAllPublished
		return scope
	}())
	return storeTokenFixture{
		record: TokenRecord{
			SecretHash: hash,
			Name:       "llm-agent",
			Kind:       string(auth.TokenPersistent),
			OwnerID:    owner,
			ScopeJSON:  scope,
			ExpiresAt:  time.Now().Add(expiresIn).UnixMilli(),
		},
		secret: secret,
		hash:   hash,
	}
}

type storeTokenFixture struct {
	record TokenRecord
	secret string
	hash   string
}

func TestTokenLifecycle(t *testing.T) {
	dataStore := openMigrated(t)
	ctx := context.Background()

	fixture := tokenFixture("local:alice", time.Hour)
	if err := dataStore.CreateToken(ctx, &fixture.record); err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}
	if fixture.record.ID == "" || fixture.record.CreatedAt == 0 {
		t.Fatalf("create did not fill id/timestamps")
	}

	found, err := dataStore.FindTokenByHash(ctx, fixture.hash)
	if err != nil {
		t.Fatalf("FindTokenByHash failed: %v", err)
	}
	if found.ID != fixture.record.ID || found.OwnerID != "local:alice" {
		t.Fatalf("found = %+v", found)
	}
	if !found.Active() {
		t.Fatalf("fresh token must be active")
	}

	listed, err := dataStore.ListTokens(ctx, "local:alice")
	if err != nil || len(listed) != 1 || listed[0].ID != fixture.record.ID {
		t.Fatalf("ListTokens = %v (err %v)", listed, err)
	}
	foreign, err := dataStore.ListTokens(ctx, "local:bob")
	if err != nil || len(foreign) != 0 {
		t.Fatalf("foreign listing = %v (err %v)", foreign, err)
	}

	if err := dataStore.TouchToken(ctx, fixture.record.ID, time.Now()); err != nil {
		t.Fatalf("TouchToken failed: %v", err)
	}
	touched, _ := dataStore.FindTokenByHash(ctx, fixture.hash)
	if touched.LastUsedAt == 0 {
		t.Fatalf("last_used_at not stamped")
	}

	// A foreign owner cannot revoke.
	if revoked, err := dataStore.RevokeToken(ctx, fixture.record.ID, "local:bob"); err != nil || revoked {
		t.Fatalf("foreign revoke = %v %v", revoked, err)
	}
	if _, err := dataStore.RevokeToken(ctx, fixture.record.ID, "local:alice"); err != nil {
		t.Fatalf("owner revoke failed: %v", err)
	}
	if _, err := dataStore.FindTokenByHash(ctx, fixture.hash); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("revoked token still found: %v", err)
	}
	if listed, _ := dataStore.ListTokens(ctx, "local:alice"); len(listed) != 0 {
		t.Fatalf("revoked token still listed")
	}
}

func TestExpiredTokensAreMissing(t *testing.T) {
	dataStore := openMigrated(t)
	ctx := context.Background()

	fixture := tokenFixture("local:alice", -time.Minute)
	if err := dataStore.CreateToken(ctx, &fixture.record); err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}
	if _, err := dataStore.FindTokenByHash(ctx, fixture.hash); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expired token found: %v", err)
	}
}

func TestUnknownTokenHashIsMissing(t *testing.T) {
	dataStore := openMigrated(t)
	if _, err := dataStore.FindTokenByHash(context.Background(), "deadbeef"); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("unknown hash = %v", err)
	}
}
