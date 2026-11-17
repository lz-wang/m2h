package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"

	goldap "github.com/go-ldap/ldap/v3"
)

// fakeLDAPConn scripts one authentication conversation.
type fakeLDAPConn struct {
	startTLSCalled bool
	binds          []struct{ username, password string }
	bindResults    map[string]error // username -> error ("" password key for service bind)
	searches       []*goldap.SearchRequest
	searchResult   *goldap.SearchResult
	searchErr      error
	closed         bool
}

func (conn *fakeLDAPConn) Bind(username, password string) error {
	conn.binds = append(conn.binds, struct{ username, password string }{username, password})
	if conn.bindResults == nil {
		return nil
	}
	if err, ok := conn.bindResults[username]; ok {
		return err
	}
	return conn.bindResults["*"]
}

func (conn *fakeLDAPConn) SearchDirectory(request *goldap.SearchRequest) (*goldap.SearchResult, error) {
	conn.searches = append(conn.searches, request)
	if conn.searchErr != nil {
		return nil, conn.searchErr
	}
	return conn.searchResult, nil
}

func (conn *fakeLDAPConn) Start(*tls.Config) error {
	conn.startTLSCalled = true
	return nil
}

func (conn *fakeLDAPConn) Close() error {
	conn.closed = true
	return nil
}

func testLDAPConfig() LDAPConfig {
	return LDAPConfig{
		URL:          "ldaps://ldap.example.internal:636",
		BindDN:       "uid=service,ou=people,dc=example,dc=internal",
		BindPassword: "service-secret",
		UserBaseDN:   "ou=people,dc=example,dc=internal",
		UserFilter:   "(uid=%s)",
		AllowedUsers: []string{"alice", "bob"},
	}
}

func withFakeLDAP(t *testing.T, conn *fakeLDAPConn, config LDAPConfig) *LDAPAuthenticator {
	t.Helper()
	authenticator, err := NewLDAPAuthenticator(config)
	if err != nil {
		t.Fatalf("NewLDAPAuthenticator failed: %v", err)
	}
	authenticator.dial = func(LDAPConfig) (ldapConn, error) { return conn, nil }
	return authenticator
}

func singleEntry(dn, uuid string) *goldap.SearchResult {
	entry := goldap.NewEntry(dn, map[string][]string{})
	if uuid != "" {
		attribute := goldap.NewEntryAttribute("entryUUID", []string{uuid})
		entry.Attributes = append(entry.Attributes, attribute)
	}
	return &goldap.SearchResult{Entries: []*goldap.Entry{entry}}
}

func TestLDAPAuthenticateHappyPath(t *testing.T) {
	conn := &fakeLDAPConn{searchResult: singleEntry("uid=alice,ou=people,dc=example,dc=internal", "uuid-1")}
	authenticator := withFakeLDAP(t, conn, testLDAPConfig())

	principal, err := authenticator.Authenticate(context.Background(), "alice", "alice-secret")
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}
	if principal.ID != "ldap:uuid-1" || principal.Username != "alice" || principal.Provider != "ldap" {
		t.Fatalf("principal = %+v", principal)
	}
	if principal.DisplayName == "" {
		t.Fatalf("display name missing")
	}
	if len(conn.binds) != 2 {
		t.Fatalf("binds = %v, want service bind then user bind", conn.binds)
	}
	if conn.binds[0].username != "uid=service,ou=people,dc=example,dc=internal" {
		t.Fatalf("first bind = %q, want the service account", conn.binds[0].username)
	}
	if conn.binds[1].username != "uid=alice,ou=people,dc=example,dc=internal" ||
		conn.binds[1].password != "alice-secret" {
		t.Fatalf("user bind = %+v", conn.binds[1])
	}
	if !conn.closed {
		t.Fatalf("connection not closed")
	}
}

func TestLDAPIdentityFallsBackFromUUIDToDN(t *testing.T) {
	conn := &fakeLDAPConn{searchResult: singleEntry("uid=alice,ou=people,dc=example,dc=internal", "")}
	authenticator := withFakeLDAP(t, conn, testLDAPConfig())

	principal, err := authenticator.Authenticate(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}
	if principal.ID != "ldap:uid=alice,ou=people,dc=example,dc=internal" {
		t.Fatalf("fallback identity = %q", principal.ID)
	}
}

func TestLDAPSearchFilterEscapesUsername(t *testing.T) {
	// Put an injection-shaped username on the allowlist so the search
	// runs; the fake directory binds anything, so what matters is the
	// filter that reached it.
	config := testLDAPConfig()
	config.AllowedUsers = []string{`alice*)(uid=admin`}
	conn := &fakeLDAPConn{searchResult: singleEntry("uid=x", "u1")}
	authenticator := withFakeLDAP(t, conn, config)

	if _, err := authenticator.Authenticate(context.Background(), `alice*)(uid=admin`, "pw"); err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}
	if len(conn.searches) != 1 {
		t.Fatalf("searches = %d, want 1", len(conn.searches))
	}
	want := `(uid=alice\2a\29\28uid=admin)`
	if conn.searches[0].Filter != want {
		t.Fatalf("filter = %q, want %q", conn.searches[0].Filter, want)
	}
}

func TestLDAPAuthenticateFailures(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		prepare  func(conn *fakeLDAPConn, config *LDAPConfig)
	}{
		{
			name:     "empty password",
			username: "alice", password: "",
			prepare: func(*fakeLDAPConn, *LDAPConfig) {},
		},
		{
			name:     "outside allowlist",
			username: "mallory", password: "pw",
			prepare: func(*fakeLDAPConn, *LDAPConfig) {},
		},
		{
			name:     "service bind fails",
			username: "alice", password: "pw",
			prepare: func(conn *fakeLDAPConn, _ *LDAPConfig) {
				conn.bindResults = map[string]error{"uid=service,ou=people,dc=example,dc=internal": errors.New("bad service credentials")}
			},
		},
		{
			name:     "search fails",
			username: "alice", password: "pw",
			prepare: func(conn *fakeLDAPConn, _ *LDAPConfig) {
				conn.searchErr = errors.New("directory unavailable")
			},
		},
		{
			name:     "no match",
			username: "alice", password: "pw",
			prepare: func(conn *fakeLDAPConn, _ *LDAPConfig) {
				conn.searchResult = &goldap.SearchResult{}
			},
		},
		{
			name:     "ambiguous match",
			username: "alice", password: "pw",
			prepare: func(conn *fakeLDAPConn, _ *LDAPConfig) {
				conn.searchResult = &goldap.SearchResult{Entries: []*goldap.Entry{
					goldap.NewEntry("uid=alice,ou=a", map[string][]string{}),
					goldap.NewEntry("uid=alice,ou=b", map[string][]string{}),
				}}
			},
		},
		{
			name:     "user bind fails",
			username: "alice", password: "wrong",
			prepare: func(conn *fakeLDAPConn, _ *LDAPConfig) {
				conn.searchResult = singleEntry("uid=alice,ou=people,dc=example,dc=internal", "u1")
				conn.bindResults = map[string]error{
					"uid=alice,ou=people,dc=example,dc=internal": errors.New("invalid credentials"),
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := &fakeLDAPConn{}
			config := testLDAPConfig()
			test.prepare(conn, &config)
			authenticator := withFakeLDAP(t, conn, config)

			principal, err := authenticator.Authenticate(context.Background(), test.username, test.password)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("Authenticate = %+v err %v, want ErrInvalidCredentials", principal, err)
			}
		})
	}
}

func TestLDAPDialFailureIsAuthenticationFailure(t *testing.T) {
	authenticator, err := NewLDAPAuthenticator(testLDAPConfig())
	if err != nil {
		t.Fatalf("NewLDAPAuthenticator failed: %v", err)
	}
	authenticator.dial = func(LDAPConfig) (ldapConn, error) {
		return nil, errors.New("dial tcp: connection refused")
	}
	if _, err := authenticator.Authenticate(context.Background(), "alice", "pw"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("dial failure = %v, want ErrInvalidCredentials (no local fallback)", err)
	}
}

func TestLDAPCancelledContextRefuses(t *testing.T) {
	conn := &fakeLDAPConn{}
	authenticator := withFakeLDAP(t, conn, testLDAPConfig())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := authenticator.Authenticate(ctx, "alice", "pw"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("cancelled context = %v, want refusal", err)
	}
	if len(conn.binds) != 0 {
		t.Fatalf("cancelled context still dialed the directory")
	}
}

func TestNewLDAPAuthenticatorRequiresCompleteConfig(t *testing.T) {
	if _, err := NewLDAPAuthenticator(LDAPConfig{}); err == nil {
		t.Fatalf("empty config accepted")
	}
}

// TestLDAPRealDialerReachesNetwork documents that dialLDAP against a dead
// port fails closed (and exercises the URL validation).
func TestLDAPRealDialerReachesNetwork(t *testing.T) {
	if _, err := dialLDAP(LDAPConfig{URL: "ldaps://127.0.0.1:1"}); err == nil {
		t.Fatalf("dial to closed port succeeded")
	}
	if _, err := dialLDAP(LDAPConfig{URL: "ftp://example.com"}); err == nil {
		t.Fatalf("invalid scheme accepted")
	}
}

func TestConnAdapterForwardsCalls(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	// Drain the server end so protocol writes never block forever.
	go func() {
		buffer := make([]byte, 256)
		for {
			if _, err := server.Read(buffer); err != nil {
				return
			}
		}
	}()
	connection := goldap.NewConn(client, false)
	// Nothing answers on the pipe, so every operation must time out fast
	// instead of blocking.
	connection.SetTimeout(100 * time.Millisecond)
	go connection.Start()
	defer connection.Close()
	adapter := connAdapter{conn: connection}

	if err := adapter.Bind("cn=x", "pw"); err == nil {
		t.Fatalf("bind over a raw pipe unexpectedly succeeded")
	}
	if _, err := adapter.SearchDirectory(&goldap.SearchRequest{}); err == nil {
		t.Fatalf("search over a raw pipe unexpectedly succeeded")
	}
	if err := adapter.Start(&tls.Config{}); err == nil {
		t.Fatalf("starttls over a raw pipe unexpectedly succeeded")
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestLDAPSearchRequestsIdentityAttributes(t *testing.T) {
	conn := &fakeLDAPConn{searchResult: singleEntry("uid=alice,ou=people,dc=example,dc=internal", "uuid-1")}
	authenticator := withFakeLDAP(t, conn, testLDAPConfig())

	principal, err := authenticator.Authenticate(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}
	// The stable identity must come from entryUUID — only possible when the
	// search actually asked the directory for it (and for the cn display
	// name).
	if principal.ID != "ldap:uuid-1" {
		t.Fatalf("identity fell back to DN: %q — the search attributes are missing entryUUID/cn", principal.ID)
	}
	if len(conn.searches) != 1 {
		t.Fatalf("searches = %d", len(conn.searches))
	}
	attributes := conn.searches[0].Attributes
	for _, want := range []string{"dn", "entryUUID", "cn"} {
		found := false
		for _, attribute := range attributes {
			if attribute == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("search attributes = %v, want %q included", attributes, want)
		}
	}
}
