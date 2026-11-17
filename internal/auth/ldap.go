package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"

	goldap "github.com/go-ldap/ldap/v3"
)

// ldapConn is the slice of the LDAP client the authenticator needs. The
// interface keeps the directory conversation testable without a live
// server; production uses goldap.Conn.
type ldapConn interface {
	Bind(username, password string) error
	SearchDirectory(request *goldap.SearchRequest) (*goldap.SearchResult, error)
	Start(config *tls.Config) error
	Close() error
}

// ldapDialer opens a connection for one authentication attempt.
type ldapDialer func(config LDAPConfig) (ldapConn, error)

// LDAPAuthenticator implements search-then-bind authentication against one
// LDAP directory with a username allowlist.
type LDAPAuthenticator struct {
	config LDAPConfig
	dial   ldapDialer
}

// ldapTimeout bounds every dial and directory operation of one attempt.
const ldapTimeout = 10 * time.Second

// NewLDAPAuthenticator builds the authenticator from validated
// configuration.
func NewLDAPAuthenticator(config LDAPConfig) (*LDAPAuthenticator, error) {
	if config.URL == "" || config.UserBaseDN == "" || config.UserFilter == "" ||
		len(config.AllowedUsers) == 0 {
		return nil, fmt.Errorf("incomplete LDAP configuration")
	}
	return &LDAPAuthenticator{config: config, dial: dialLDAP}, nil
}

// dialLDAP connects to the configured directory. ldaps:// talks TLS from
// the first byte; ldap:// upgrades with StartTLS before any credentials
// travel. Certificates are always verified — there is no insecure mode.
func dialLDAP(config LDAPConfig) (ldapConn, error) {
	parsed, err := url.Parse(config.URL)
	if err != nil || parsed.Host == "" ||
		(parsed.Scheme != "ldap" && parsed.Scheme != "ldaps") {
		return nil, fmt.Errorf("invalid LDAP URL %q: expected ldap://host or ldaps://host", config.URL)
	}
	connection, err := goldap.DialURL(parsed.String(),
		goldap.DialWithDialer(&net.Dialer{Timeout: ldapTimeout}),
	)
	if err != nil {
		return nil, fmt.Errorf("connect LDAP server: %w", err)
	}
	connection.SetTimeout(ldapTimeout)
	if parsed.Scheme == "ldap" {
		if err := connection.StartTLS(&tls.Config{ServerName: parsed.Hostname()}); err != nil {
			connection.Close()
			return nil, fmt.Errorf("upgrade LDAP connection with StartTLS: %w", err)
		}
	}
	return connAdapter{connection}, nil
}

// connAdapter teaches *goldap.Conn the ldapConn interface shape (Search is
// exposed as SearchDirectory to keep the interface name honest).
type connAdapter struct {
	conn *goldap.Conn
}

func (adapter connAdapter) Bind(username, password string) error {
	return adapter.conn.Bind(username, password)
}

func (adapter connAdapter) SearchDirectory(request *goldap.SearchRequest) (*goldap.SearchResult, error) {
	return adapter.conn.Search(request)
}

func (adapter connAdapter) Start(config *tls.Config) error {
	return adapter.conn.StartTLS(config)
}

func (adapter connAdapter) Close() error {
	return adapter.conn.Close()
}

// Authenticate runs the allowlist → service bind → user search → user bind
// conversation. Every failure — allowlist refusal, directory trouble,
// ambiguous matches, wrong credentials — returns ErrInvalidCredentials so
// responses cannot distinguish them; empty passwords never reach a bind.
func (authenticator *LDAPAuthenticator) Authenticate(ctx context.Context, username, password string) (Principal, error) {
	if username == "" || password == "" || strings.ContainsFunc(username, isControlRune) {
		return Principal{}, ErrInvalidCredentials
	}
	if !authenticator.allowed(username) {
		return Principal{}, ErrInvalidCredentials
	}
	if err := ctx.Err(); err != nil {
		return Principal{}, ErrInvalidCredentials
	}

	connection, err := authenticator.dial(authenticator.config)
	if err != nil {
		return Principal{}, ErrInvalidCredentials
	}
	defer connection.Close()

	if err := connection.Bind(authenticator.config.BindDN, authenticator.config.BindPassword); err != nil {
		return Principal{}, ErrInvalidCredentials
	}

	// EscapeFilter is the standard go-ldap filter escaper: parentheses,
	// wildcards, NUL and friends cannot alter the filter's structure.
	filter := fmt.Sprintf(authenticator.config.UserFilter, goldap.EscapeFilter(username))
	request := goldap.NewSearchRequest(
		authenticator.config.UserBaseDN,
		goldap.ScopeWholeSubtree, goldap.NeverDerefAliases, 2, int(ldapTimeout.Seconds()), false,
		filter,
		// The distinguished name rides on the entry itself; entryUUID is
		// the stable identity (survives renames) and cn the display name.
		[]string{"dn", "entryUUID", "cn"},
		nil,
	)
	result, err := connection.SearchDirectory(request)
	if err != nil {
		return Principal{}, ErrInvalidCredentials
	}
	if len(result.Entries) != 1 {
		// Zero matches and ambiguous matches are equally fatal.
		return Principal{}, ErrInvalidCredentials
	}
	entry := result.Entries[0]

	if err := connection.Bind(entry.DN, password); err != nil {
		return Principal{}, ErrInvalidCredentials
	}
	return ldapPrincipal(entry, username), nil
}

func (authenticator *LDAPAuthenticator) allowed(username string) bool {
	return slices.Contains(authenticator.config.AllowedUsers, username)
}

// IsPrincipalAllowed applies the local allowlist to a previously
// authenticated principal: a username dropped from M2H_LDAP_ALLOWED_USERS
// loses its sessions and tokens at the next request, without a directory
// round trip.
func (authenticator *LDAPAuthenticator) IsPrincipalAllowed(principal Principal) bool {
	return authenticator.allowed(principal.Username)
}

// ldapPrincipal derives the stable internal identity: the directory's
// entryUUID when present (survives renames), else the distinguished name,
// else the allowlisted username.
func ldapPrincipal(entry *goldap.Entry, username string) Principal {
	identity := entry.GetAttributeValue("entryUUID")
	if strings.TrimSpace(identity) == "" {
		identity = strings.TrimSpace(entry.DN)
	}
	if identity == "" {
		identity = username
	}
	display := username
	if value := entry.GetAttributeValue("cn"); strings.TrimSpace(value) != "" {
		display = strings.TrimSpace(value)
	}
	return Principal{
		ID:          "ldap:" + identity,
		Username:    username,
		DisplayName: display,
		Provider:    string(ModeLDAP),
	}
}
