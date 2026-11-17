package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrInvalidCredentials is the unified authentication failure. Callers must
// not distinguish unknown users from wrong passwords in responses, logs or
// timing — every failure path returns exactly this error.
var ErrInvalidCredentials = errors.New("invalid credentials")

// Principal is the authenticated identity one session or token carries.
type Principal struct {
	// ID is the internal stable identity, namespaced by provider
	// ("local:alice", "ldap:<uuid>").
	ID          string
	Username    string
	DisplayName string
	Provider    string
}

// Authenticator verifies a username/password pair against one identity
// provider. Implementations must keep failure paths uniform.
type Authenticator interface {
	Authenticate(ctx context.Context, username, password string) (Principal, error)
}

// PrincipalAuthorizer reports whether a previously authenticated principal
// is still authorized under the current configuration. Persisted sessions
// and API tokens re-check this on every request, so removing an account
// (or dropping it from the LDAP allowlist) retires its outstanding
// credentials without waiting for their expiry.
type PrincipalAuthorizer interface {
	IsPrincipalAllowed(principal Principal) bool
}

// LocalAuthenticator verifies accounts declared by M2H_LOCAL_USERS_JSON.
// Passwords and hashes live only in memory for the lifetime of the process;
// nothing is persisted or logged.
type LocalAuthenticator struct {
	users map[string]User
	// dummyParams drive the timing-equalizing verification for unknown
	// usernames: a fixed argon2id run with plausible parameters.
	dummyParams argon2Params
}

// NewLocalAuthenticator indexes the configured users by username.
func NewLocalAuthenticator(users []User) (*LocalAuthenticator, error) {
	indexed := make(map[string]User, len(users))
	for _, user := range users {
		if _, exists := indexed[user.Username]; exists {
			return nil, fmt.Errorf("duplicate local user %q", user.Username)
		}
		if user.Username == "" || (user.Password == "" && user.Hash == "") {
			return nil, fmt.Errorf("incomplete local user %q", user.Username)
		}
		indexed[user.Username] = user
	}
	if len(indexed) == 0 {
		return nil, fmt.Errorf("at least one local user is required")
	}
	return &LocalAuthenticator{
		users: indexed,
		// Fixed plausible parameters (64 MiB, t=3, p=4, 16-byte salt,
		// 32-byte digest length): any value works, it just has to cost
		// the same as a real verify.
		dummyParams: argon2Params{version: 19, memoryKB: 65536, time: 3, threads: 4,
			salt:   []byte("m2h-dummy-salt!"),
			digest: make([]byte, 32)},
	}, nil
}

// Authenticate verifies one local account. Unknown usernames, wrong
// passwords and malformed inputs all return ErrInvalidCredentials after
// comparable work.
func (authenticator *LocalAuthenticator) Authenticate(_ context.Context, username, password string) (Principal, error) {
	if username == "" || password == "" || strings.ContainsFunc(username, isControlRune) {
		authenticator.burnDummy()
		return Principal{}, ErrInvalidCredentials
	}
	user, known := authenticator.users[username]
	if !known {
		authenticator.burnDummy()
		return Principal{}, ErrInvalidCredentials
	}
	if !verifyLocalSecret(user, password) {
		return Principal{}, ErrInvalidCredentials
	}
	return Principal{
		ID:          "local:" + user.Username,
		Username:    user.Username,
		DisplayName: user.Username,
		Provider:    string(ModeLocal),
	}, nil
}

// IsPrincipalAllowed answers whether the account still exists in the
// configured user table. A local account removed from M2H_LOCAL_USERS_JSON
// loses its sessions and tokens at the next request.
func (authenticator *LocalAuthenticator) IsPrincipalAllowed(principal Principal) bool {
	_, known := authenticator.users[principal.Username]
	return known
}

// burnDummy performs one hash verification with the dummy parameters so an
// unknown-username failure costs the same as a wrong password against a
// hashed account.
func (authenticator *LocalAuthenticator) burnDummy() {
	_ = verifyArgon2(authenticator.dummyParams, "m2h-uniform-timing")
}

// verifyLocalSecret checks one candidate against the configured secret
// form. Plain comparisons hash both sides first so the constant-time
// comparison sees fixed-length digests.
func verifyLocalSecret(user User, password string) bool {
	if user.Hash != "" {
		params, err := decodePHC(user.Hash)
		if err != nil {
			// Configuration was validated at startup; a broken hash here is
			// unreachable, but failing closed keeps the account locked.
			return false
		}
		return verifyArgon2(params, password)
	}
	candidate := sha256.Sum256([]byte(password))
	configured := sha256.Sum256([]byte(user.Password))
	return subtle.ConstantTimeCompare(candidate[:], configured[:]) == 1
}

// verifyArgon2 recomputes the Argon2id digest from the hash's own
// parameters and compares it in constant time.
func verifyArgon2(params argon2Params, password string) bool {
	digest := argon2.IDKey([]byte(password), params.salt,
		params.time, params.memoryKB, params.threads, uint32(len(params.digest)))
	return subtle.ConstantTimeCompare(digest, params.digest) == 1
}
