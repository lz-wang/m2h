package auth

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestLocalAuthenticatorVerifiesPlainPasswords(t *testing.T) {
	authenticator, err := NewLocalAuthenticator([]User{
		{Username: "alice", Password: "password-1"},
		{Username: "bob", Password: "password-2"},
	})
	if err != nil {
		t.Fatalf("NewLocalAuthenticator failed: %v", err)
	}

	principal, err := authenticator.Authenticate(context.Background(), "alice", "password-1")
	if err != nil {
		t.Fatalf("alice login failed: %v", err)
	}
	if principal.ID != "local:alice" || principal.Username != "alice" ||
		principal.DisplayName != "alice" || principal.Provider != "local" {
		t.Fatalf("principal = %+v", principal)
	}

	bob, err := authenticator.Authenticate(context.Background(), "bob", "password-2")
	if err != nil || bob.ID != "local:bob" {
		t.Fatalf("bob login = %+v err %v — identities must be per-user", bob, err)
	}

	// Usernames are exact: no padding, no case games.
	if _, err := authenticator.Authenticate(context.Background(), "Alice", "password-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("case-variant username accepted: %v", err)
	}
}

func TestLocalAuthenticatorVerifiesArgon2Hashes(t *testing.T) {
	hash, err := encodeArgon2idForTest("correct horse")
	if err != nil {
		t.Fatalf("encode hash failed: %v", err)
	}
	authenticator, err := NewLocalAuthenticator([]User{{Username: "alice", Hash: hash}})
	if err != nil {
		t.Fatalf("NewLocalAuthenticator failed: %v", err)
	}

	if _, err := authenticator.Authenticate(context.Background(), "alice", "correct horse"); err != nil {
		t.Fatalf("argon2 login failed: %v", err)
	}
	if _, err := authenticator.Authenticate(context.Background(), "alice", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong argon2 password accepted: %v", err)
	}
}

func TestLocalAuthenticatorUniformFailures(t *testing.T) {
	authenticator, err := NewLocalAuthenticator([]User{
		{Username: "alice", Password: "password-1"},
	})
	if err != nil {
		t.Fatalf("NewLocalAuthenticator failed: %v", err)
	}
	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "wrong password", username: "alice", password: "nope"},
		{name: "unknown user", username: "mallory", password: "password-1"},
		{name: "empty username", username: "", password: "password-1"},
		{name: "empty password", username: "alice", password: ""},
		{name: "control characters", username: "ali\x00ce", password: "password-1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			principal, err := authenticator.Authenticate(context.Background(), test.username, test.password)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("Authenticate(%q, %q) = %+v err %v, want ErrInvalidCredentials",
					test.username, test.password, principal, err)
			}
		})
	}
}

func TestNewLocalAuthenticatorRejectsBrokenInput(t *testing.T) {
	tests := []struct {
		name  string
		users []User
	}{
		{name: "empty list", users: nil},
		{name: "duplicate usernames", users: []User{
			{Username: "alice", Password: "one"},
			{Username: "alice", Password: "two"},
		}},
		{name: "incomplete user", users: []User{{Username: "alice"}}},
		{name: "empty username", users: []User{{Password: "p"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewLocalAuthenticator(test.users); err == nil {
				t.Fatalf("NewLocalAuthenticator(%v) accepted", test.users)
			}
		})
	}
}

// TestLocalPrincipalAuthorization pins the re-authorization check persisted
// sessions and tokens rely on: a principal minted by an earlier process must
// stop being allowed the moment its account leaves the configuration.
func TestLocalPrincipalAuthorization(t *testing.T) {
	authenticator, err := NewLocalAuthenticator([]User{
		{Username: "alice", Password: "password-1"},
	})
	if err != nil {
		t.Fatalf("NewLocalAuthenticator failed: %v", err)
	}
	if !authenticator.IsPrincipalAllowed(Principal{ID: "local:alice", Username: "alice", Provider: "local"}) {
		t.Fatalf("configured user must stay allowed")
	}
	if authenticator.IsPrincipalAllowed(Principal{ID: "local:bob", Username: "bob", Provider: "local"}) {
		t.Fatalf("removed user must lose authorization immediately")
	}
	// The username decides — a live username stays allowed whatever stale
	// id or provider the persisted credential carries.
	if !authenticator.IsPrincipalAllowed(Principal{ID: "local:bob", Username: "alice", Provider: "ldap"}) {
		t.Fatalf("authorization must key on the configured username, not the id")
	}
}

// encodeArgon2idForTest builds a PHC string with fixed fast parameters.
func encodeArgon2idForTest(password string) (string, error) {
	params, err := decodePHC("$argon2id$v=19$m=32768,t=2,p=2$" +
		"c2FsdHNhbHQ" + "$dGhpc2lzYXRlc3RkaWdlc3RoaXNpc2F0ZXN0ZGlnZXN0")
	if err != nil {
		return "", err
	}
	digest := argon2.IDKey([]byte(password), params.salt,
		params.time, params.memoryKB, params.threads, uint32(len(params.digest)))
	return "$argon2id$v=19$m=32768,t=2,p=2$c2FsdHNhbHQ$" +
		phcBase64(digest), nil
}
