package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/server"
)

// withEnv swaps the CLI's environment lookup for a fixed table (and
// restores both it and any process environment touched by tests).
func withEnv(t *testing.T, values map[string]string) {
	t.Helper()
	previousLookup := envLookup
	previousDataDir := defaultDataDir
	t.Cleanup(func() {
		envLookup = previousLookup
		defaultDataDir = previousDataDir
	})
	envLookup = func(name string) string { return values[name] }
	// Freeze the computed default so tests do not depend on the machine.
	defaultDataDir = func() string { return "/default/m2h-data" }
}

func captureServerOptions(t *testing.T) *server.Options {
	t.Helper()
	previous := runServer
	t.Cleanup(func() { runServer = previous })
	captured := &server.Options{}
	runServer = func(_ context.Context, options server.Options) error {
		*captured = options
		return nil
	}
	return captured
}

func TestServeAuthFlagDefaultsToNone(t *testing.T) {
	withEnv(t, map[string]string{})
	captured := captureServerOptions(t)

	if _, _, err := runCommand(t, "docs.md"); err != nil {
		t.Fatalf("m2h docs.md returned error: %v", err)
	}
	if captured.AuthMode != auth.ModeNone {
		t.Fatalf("default auth mode = %q, want none", captured.AuthMode)
	}
	if captured.DataDir == "" {
		t.Fatalf("default data dir missing")
	}
	if captured.AuthConfig == nil || captured.AuthConfig.Mode != auth.ModeNone {
		t.Fatalf("default auth config = %+v, want a none-mode config", captured.AuthConfig)
	}
}

func TestServeAuthVariablesDoNotEnableAuth(t *testing.T) {
	withEnv(t, map[string]string{
		auth.EnvLocalUsersJSON: `{"alice": "password-1"}`,
	})
	captured := captureServerOptions(t)

	if _, _, err := runCommand(t, "docs.md"); err != nil {
		t.Fatalf("m2h docs.md returned error: %v", err)
	}
	if captured.AuthMode != auth.ModeNone {
		t.Fatalf("auth variables alone enabled auth mode %q", captured.AuthMode)
	}
}

func TestServeAuthLocalOptions(t *testing.T) {
	withEnv(t, map[string]string{
		auth.EnvLocalUsersJSON: `{"alice": "password-1", "bob": "password-2"}`,
	})
	captured := captureServerOptions(t)

	if _, _, err := runCommand(t, "--auth", "local", "--data-dir", "/tmp/m2h-data", "docs.md"); err != nil {
		t.Fatalf("m2h --auth local returned error: %v", err)
	}
	if captured.AuthMode != auth.ModeLocal {
		t.Fatalf("auth mode = %q, want local", captured.AuthMode)
	}
	if captured.DataDir != "/tmp/m2h-data" {
		t.Fatalf("data dir = %q, want /tmp/m2h-data", captured.DataDir)
	}
	if captured.AuthConfig == nil || len(captured.AuthConfig.LocalUsers) != 2 {
		t.Fatalf("local users not forwarded: %+v", captured.AuthConfig)
	}
}

func TestServeAuthLocalWithoutUsersFailsBeforeServe(t *testing.T) {
	withEnv(t, map[string]string{})
	called := false
	previous := runServer
	t.Cleanup(func() { runServer = previous })
	runServer = func(_ context.Context, _ server.Options) error {
		called = true
		return nil
	}

	_, _, err := runCommand(t, "--auth", "local", "docs.md")
	if err == nil {
		t.Fatalf("m2h --auth local without users succeeded")
	}
	if !strings.Contains(err.Error(), auth.EnvLocalUsersJSON) {
		t.Fatalf("error = %q, want it to name %s", err, auth.EnvLocalUsersJSON)
	}
	if called {
		t.Fatalf("server started despite invalid auth configuration")
	}
}

func TestServeAuthInvalidModeRejected(t *testing.T) {
	withEnv(t, map[string]string{})
	if _, _, err := runCommand(t, "--auth", "oauth", "docs.md"); err == nil {
		t.Fatalf("m2h --auth oauth succeeded")
	}
}

func TestServeLDAPRequiresCompleteConfiguration(t *testing.T) {
	withEnv(t, map[string]string{
		auth.EnvLDAPURL:        "ldaps://ldap.example.internal:636",
		auth.EnvLDAPBindDN:     "uid=service,ou=people,dc=example,dc=internal",
		auth.EnvLDAPBindPW:     "service-secret",
		auth.EnvLDAPUserBaseDN: "ou=people,dc=example,dc=internal",
		auth.EnvLDAPUserFilter: "(uid=%s)",
		// allowlist missing on purpose
	})
	if _, _, err := runCommand(t, "--auth", "ldap", "docs.md"); err == nil {
		t.Fatalf("incomplete LDAP configuration accepted")
	}

	withEnv(t, map[string]string{
		auth.EnvLDAPURL:        "ldaps://ldap.example.internal:636",
		auth.EnvLDAPBindDN:     "uid=service,ou=people,dc=example,dc=internal",
		auth.EnvLDAPBindPW:     "service-secret",
		auth.EnvLDAPUserBaseDN: "ou=people,dc=example,dc=internal",
		auth.EnvLDAPUserFilter: "(uid=%s)",
		auth.EnvLDAPAllowed:    "alice,bob",
	})
	captured := captureServerOptions(t)
	if _, _, err := runCommand(t, "--auth", "ldap", "docs.md"); err != nil {
		t.Fatalf("complete LDAP configuration rejected: %v", err)
	}
	if captured.AuthMode != auth.ModeLDAP || !slices.Equal(captured.AuthConfig.LDAP.AllowedUsers, []string{"alice", "bob"}) {
		t.Fatalf("LDAP options not forwarded: mode %q config %+v", captured.AuthMode, captured.AuthConfig)
	}
}

func TestServeDataDirPrecedence(t *testing.T) {
	t.Run("flag beats environment", func(t *testing.T) {
		withEnv(t, map[string]string{"M2H_DATA_DIR": "/from-env"})
		captured := captureServerOptions(t)
		if _, _, err := runCommand(t, "--data-dir", "/from-flag", "docs.md"); err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if captured.DataDir != "/from-flag" {
			t.Fatalf("data dir = %q, want /from-flag", captured.DataDir)
		}
	})
	t.Run("environment fills unset flag", func(t *testing.T) {
		withEnv(t, map[string]string{"M2H_DATA_DIR": "/from-env"})
		captured := captureServerOptions(t)
		if _, _, err := runCommand(t, "docs.md"); err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if captured.DataDir != "/from-env" {
			t.Fatalf("data dir = %q, want /from-env", captured.DataDir)
		}
	})
	t.Run("default without either", func(t *testing.T) {
		withEnv(t, map[string]string{})
		captured := captureServerOptions(t)
		if _, _, err := runCommand(t, "docs.md"); err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if captured.DataDir != "/default/m2h-data" {
			t.Fatalf("data dir = %q, want default", captured.DataDir)
		}
	})
}

func TestPublicURLForwardedFromEnvironment(t *testing.T) {
	withEnv(t, map[string]string{auth.EnvPublicURL: "https://docs.example.com/docs"})
	captured := captureServerOptions(t)
	if _, _, err := runCommand(t, "docs.md"); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if captured.PublicURL != "https://docs.example.com/docs" {
		t.Fatalf("public URL = %q", captured.PublicURL)
	}

	withEnv(t, map[string]string{auth.EnvPublicURL: "not-a-url"})
	if _, _, err := runCommand(t, "docs.md"); err == nil {
		t.Fatalf("invalid public URL accepted")
	}
}

func TestDefaultDataDirPerOS(t *testing.T) {
	values := map[string]string{}
	lookup := func(name string) string { return values[name] }
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	t.Setenv("HOME", home) // keep UserHomeDir stable for the duration
	original := envLookup
	t.Cleanup(func() { envLookup = original })
	envLookup = lookup

	got := defaultDataDir()
	switch {
	case strings.Contains(got, "Library/Application Support/m2h"),
		strings.Contains(got, ".local/share/m2h"),
		strings.Contains(got, "AppData"):
	default:
		t.Fatalf("default data dir %q is not a per-OS data location", got)
	}
	if !strings.HasSuffix(filepath.ToSlash(got), "m2h") {
		t.Fatalf("default data dir %q lacks m2h suffix", got)
	}
}

func TestXDGDataHomeOverridesDefault(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("XDG data home only applies outside %s", runtime.GOOS)
	}
	values := map[string]string{"XDG_DATA_HOME": "/xdg/data"}
	original := envLookup
	t.Cleanup(func() { envLookup = original })
	envLookup = func(name string) string { return values[name] }

	if got := defaultDataDir(); got != filepath.Join("/xdg/data", "m2h") {
		t.Fatalf("default data dir = %q, want /xdg/data/m2h", got)
	}
}
