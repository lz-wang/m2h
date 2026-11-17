// Package auth implements m2h's optional authentication layer: the
// deployment configuration (auth mode, local accounts, LDAP), the
// authenticators behind each mode, and the session and token machinery the
// authenticated server builds on. Everything here is dormant unless the
// server is started with an explicit auth mode.
package auth

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Mode selects the authentication model of one server run.
type Mode string

const (
	// ModeNone is the default read-only mode: no login, no annotations,
	// no tokens, no database.
	ModeNone Mode = "none"
	// ModeLocal authenticates against accounts declared by M2H_LOCAL_USERS_JSON.
	ModeLocal Mode = "local"
	// ModeLDAP authenticates against an LDAP directory with a user allowlist.
	ModeLDAP Mode = "ldap"
)

// ParseMode validates an auth mode string. The empty string is ModeNone so
// callers can pass unset values through unchanged.
func ParseMode(value string) (Mode, error) {
	if value == "" {
		return ModeNone, nil
	}
	switch mode := Mode(value); mode {
	case ModeNone, ModeLocal, ModeLDAP:
		return mode, nil
	default:
		return ModeNone, fmt.Errorf("invalid auth mode %q: expected none, local, or ldap", value)
	}
}

// User is one local account. Exactly one of Password (plain secret kept only
// in memory) or Hash (a PHC-formatted Argon2id string) is set; verification
// never writes either to persistent storage or logs.
type User struct {
	Username string
	Password string
	Hash     string
}

// LDAPConfig is the directory-connection half of ModeLDAP.
type LDAPConfig struct {
	URL          string
	BindDN       string
	BindPassword string
	UserBaseDN   string
	UserFilter   string
	AllowedUsers []string
}

// Config is the resolved, validated configuration of one authenticated run.
// A Config with ModeNone carries no requirements.
type Config struct {
	Mode       Mode
	LocalUsers []User
	LDAP       LDAPConfig
	// PublicURL mirrors M2H_PUBLIC_URL: the externally reachable origin
	// (reverse-proxy deployments) used for secure-cookie decisions and
	// server-generated absolute links. Empty means "not configured".
	PublicURL string
}

// Environment variable names understood by the auth layer. The data
// directory variable (M2H_DATA_DIR) belongs to the CLI/server layer.
const (
	EnvLocalUsersJSON = "M2H_LOCAL_USERS_JSON"
	EnvLDAPURL        = "M2H_LDAP_URL"
	EnvLDAPBindDN     = "M2H_LDAP_BIND_DN"
	EnvLDAPBindPW     = "M2H_LDAP_BIND_PASSWORD"
	EnvLDAPUserBaseDN = "M2H_LDAP_USER_BASE_DN"
	EnvLDAPUserFilter = "M2H_LDAP_USER_FILTER"
	EnvLDAPAllowed    = "M2H_LDAP_ALLOWED_USERS"
	EnvPublicURL      = "M2H_PUBLIC_URL"
)

// LoadConfig reads the authentication environment for mode and validates the
// result. Providing authentication variables never enables authentication by
// itself — mode decides, and each mode demands its complete configuration.
func LoadConfig(mode Mode, lookup func(string) string) (Config, error) {
	if lookup == nil {
		lookup = func(string) string { return "" }
	}
	config := Config{Mode: mode, PublicURL: strings.TrimSpace(lookup(EnvPublicURL))}
	if config.PublicURL != "" {
		parsed, err := url.Parse(config.PublicURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return Config{}, fmt.Errorf("invalid %s: expected an absolute http(s) URL", EnvPublicURL)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return Config{}, fmt.Errorf("invalid %s: scheme must be http or https", EnvPublicURL)
		}
	}

	switch mode {
	case ModeNone:
		return config, nil
	case ModeLocal:
		users, err := parseLocalUsers(lookup(EnvLocalUsersJSON))
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", EnvLocalUsersJSON, err)
		}
		config.LocalUsers = users
		return config, nil
	case ModeLDAP:
		ldap, err := parseLDAPConfig(lookup)
		if err != nil {
			return Config{}, err
		}
		config.LDAP = ldap
		return config, nil
	default:
		return Config{}, fmt.Errorf("invalid auth mode %q: expected none, local, or ldap", mode)
	}
}

// parseLocalUsers decodes M2H_LOCAL_USERS_JSON. The documented shape is a
// JSON object mapping usernames to passwords; values carrying a PHC
// "$argon2id$" prefix are accepted as password hashes instead of plain
// secrets. Duplicate keys, non-string values, blank usernames and blank
// secrets are configuration errors — a half-configured account must fail
// startup, not login halfway.
func parseLocalUsers(raw string) ([]User, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("required for auth mode local")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := opening.(json.Delim); !ok || delimiter != '{' {
		return nil, fmt.Errorf("expected a JSON object mapping usernames to passwords")
	}
	users := make(map[string]User)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		username, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("expected string usernames")
		}
		if _, duplicate := users[username]; duplicate {
			return nil, fmt.Errorf("duplicate username %q", username)
		}
		var value string
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("password of %q must be a string: %w", username, err)
		}
		user, err := newUser(username, value)
		if err != nil {
			return nil, err
		}
		users[username] = user
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("at least one user is required")
	}
	ordered := make([]User, 0, len(users))
	for _, username := range sortedKeys(users) {
		ordered = append(ordered, users[username])
	}
	return ordered, nil
}

func newUser(username, secret string) (User, error) {
	if username == "" {
		return User{}, fmt.Errorf("username must not be empty")
	}
	if len(username) > 64 {
		return User{}, fmt.Errorf("username %q exceeds 64 characters", username)
	}
	if strings.TrimSpace(username) != username || strings.ContainsFunc(username, isControlRune) {
		return User{}, fmt.Errorf("username %q must not be padded or contain control characters", username)
	}
	if secret == "" {
		return User{}, fmt.Errorf("password of %q must not be empty", username)
	}
	if strings.HasPrefix(secret, "$argon2id$") {
		if _, err := decodePHC(secret); err != nil {
			return User{}, fmt.Errorf("argon2id hash of %q: %w", username, err)
		}
		return User{Username: username, Hash: secret}, nil
	}
	return User{Username: username, Password: secret}, nil
}

func isControlRune(value rune) bool {
	return value < 0x20 || value == 0x7f
}

func sortedKeys(users map[string]User) []string {
	keys := make([]string, 0, len(users))
	for username := range users {
		keys = append(keys, username)
	}
	// Deterministic order keeps validation and logging stable.
	for index := 1; index < len(keys); index++ {
		for offset := index; offset > 0 && keys[offset] < keys[offset-1]; offset-- {
			keys[offset], keys[offset-1] = keys[offset-1], keys[offset]
		}
	}
	return keys
}

func parseLDAPConfig(lookup func(string) string) (LDAPConfig, error) {
	config := LDAPConfig{
		URL:          strings.TrimSpace(lookup(EnvLDAPURL)),
		BindDN:       strings.TrimSpace(lookup(EnvLDAPBindDN)),
		BindPassword: lookup(EnvLDAPBindPW),
		UserBaseDN:   strings.TrimSpace(lookup(EnvLDAPUserBaseDN)),
		UserFilter:   strings.TrimSpace(lookup(EnvLDAPUserFilter)),
		AllowedUsers: splitAllowlist(lookup(EnvLDAPAllowed)),
	}
	switch {
	case config.URL == "":
		return LDAPConfig{}, fmt.Errorf("%s is required for auth mode ldap", EnvLDAPURL)
	case config.BindDN == "":
		return LDAPConfig{}, fmt.Errorf("%s is required for auth mode ldap", EnvLDAPBindDN)
	case config.BindPassword == "":
		return LDAPConfig{}, fmt.Errorf("%s is required for auth mode ldap", EnvLDAPBindPW)
	case config.UserBaseDN == "":
		return LDAPConfig{}, fmt.Errorf("%s is required for auth mode ldap", EnvLDAPUserBaseDN)
	case config.UserFilter == "":
		return LDAPConfig{}, fmt.Errorf("%s is required for auth mode ldap", EnvLDAPUserFilter)
	case len(config.AllowedUsers) == 0:
		return LDAPConfig{}, fmt.Errorf("%s must list at least one user for auth mode ldap", EnvLDAPAllowed)
	}
	parsed, err := url.Parse(config.URL)
	if err != nil || parsed.Host == "" {
		return LDAPConfig{}, fmt.Errorf("invalid %s: expected ldap://host or ldaps://host", EnvLDAPURL)
	}
	if parsed.Scheme != "ldap" && parsed.Scheme != "ldaps" {
		return LDAPConfig{}, fmt.Errorf("invalid %s: scheme must be ldap or ldaps", EnvLDAPURL)
	}
	if !strings.Contains(config.UserFilter, "%s") {
		return LDAPConfig{}, fmt.Errorf("%s must contain %%s where the username is inserted", EnvLDAPUserFilter)
	}
	return config, nil
}

// splitAllowlist parses the comma-separated M2H_LDAP_ALLOWED_USERS value:
// entries are trimmed, blanks dropped, duplicates collapsed, order kept.
func splitAllowlist(raw string) []string {
	seen := make(map[string]bool)
	allowed := make([]string, 0)
	for entry := range strings.SplitSeq(raw, ",") {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		allowed = append(allowed, trimmed)
	}
	return allowed
}
