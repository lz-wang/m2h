package auth

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    Mode
		wantErr bool
	}{
		{name: "empty is none", value: "", want: ModeNone},
		{name: "none", value: "none", want: ModeNone},
		{name: "local", value: "local", want: ModeLocal},
		{name: "ldap", value: "ldap", want: ModeLDAP},
		{name: "invalid", value: "oauth", wantErr: true},
		{name: "case sensitive", value: "Local", wantErr: true},
		{name: "whitespace rejected", value: " local", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseMode(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("ParseMode(%q) = %q, want error", test.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMode(%q) failed: %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("ParseMode(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestLoadConfigNoneIgnoresAuthVariables(t *testing.T) {
	values := map[string]string{
		EnvLocalUsersJSON: `{"alice": "password-1"}`,
		EnvLDAPURL:        "ldaps://ldap.example.internal:636",
	}
	config, err := LoadConfig(ModeNone, env(values))
	if err != nil {
		t.Fatalf("LoadConfig(none) failed: %v", err)
	}
	if config.Mode != ModeNone {
		t.Fatalf("mode = %q, want none", config.Mode)
	}
	if config.LocalUsers != nil {
		t.Fatalf("none mode must not parse local users")
	}
}

func TestLoadConfigLocal(t *testing.T) {
	tests := []struct {
		name    string
		users   string
		wantErr string
	}{
		{name: "multiple users", users: `{"alice": "password-1", "bob": "password-2"}`},
		{name: "argon2id hash accepted", users: fmt.Sprintf(`{"alice": "$argon2id$v=19$m=65536,t=3,p=4$%s$%s"}`,
			"c2FsdHNhbHQ", "c2hhZG93c2hhZG93")}, // salt(8B)/digest valid base64
		{name: "missing variable", users: "", wantErr: "required for auth mode local"},
		{name: "empty object", users: `{}`, wantErr: "at least one user"},
		{name: "not an object", users: `["alice"]`, wantErr: "expected a JSON object"},
		{name: "invalid JSON", users: `{alice:password}`, wantErr: "invalid character"},
		{name: "duplicate usernames", users: `{"alice": "one", "alice": "two"}`, wantErr: "duplicate username"},
		{name: "empty username", users: `{"": "password"}`, wantErr: "username must not be empty"},
		{name: "empty password", users: `{"alice": ""}`, wantErr: "must not be empty"},
		{name: "non-string password", users: `{"alice": 42}`, wantErr: "must be a string"},
		{name: "padded username", users: `{" alice": "password"}`, wantErr: "must not be padded"},
		{name: "malformed hash", users: `{"alice": "$argon2id$v=18$m=65536,t=3,p=4$c2FsdA$"}`, wantErr: "argon2id hash"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config, err := LoadConfig(ModeLocal, env(map[string]string{EnvLocalUsersJSON: test.users}))
			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("LoadConfig(local, %q) succeeded, want error", test.users)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %q, want substring %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig(local, %q) failed: %v", test.users, err)
			}
			if len(config.LocalUsers) == 0 {
				t.Fatalf("local users missing")
			}
			for _, user := range config.LocalUsers {
				if user.Username == "" || (user.Password == "" && user.Hash == "") {
					t.Fatalf("incomplete user record: %+v", user)
				}
				if user.Password != "" && user.Hash != "" {
					t.Fatalf("user %q carries both password and hash", user.Username)
				}
			}
		})
	}
}

func TestLoadConfigLocalUsersSorted(t *testing.T) {
	config, err := LoadConfig(ModeLocal, env(map[string]string{
		EnvLocalUsersJSON: `{"zoe": "p", "alice": "p", "mid": "p"}`,
	}))
	if err != nil {
		t.Fatalf("LoadConfig(local) failed: %v", err)
	}
	got := []string{}
	for _, user := range config.LocalUsers {
		got = append(got, user.Username)
	}
	want := []string{"alice", "mid", "zoe"}
	if !slices.Equal(got, want) {
		t.Fatalf("users = %v, want %v", got, want)
	}
}

func TestLoadConfigLDAP(t *testing.T) {
	complete := map[string]string{
		EnvLDAPURL:        "ldaps://ldap.example.internal:636",
		EnvLDAPBindDN:     "uid=service,ou=people,dc=example,dc=internal",
		EnvLDAPBindPW:     "service-secret",
		EnvLDAPUserBaseDN: "ou=people,dc=example,dc=internal",
		EnvLDAPUserFilter: "(uid=%s)",
		EnvLDAPAllowed:    "alice, bob ,alice",
	}
	t.Run("complete configuration", func(t *testing.T) {
		config, err := LoadConfig(ModeLDAP, env(complete))
		if err != nil {
			t.Fatalf("LoadConfig(ldap) failed: %v", err)
		}
		if !slices.Equal(config.LDAP.AllowedUsers, []string{"alice", "bob"}) {
			t.Fatalf("allowlist = %v, want [alice bob]", config.LDAP.AllowedUsers)
		}
	})
	t.Run("ldap URL with StartTLS scheme", func(t *testing.T) {
		values := maps.Clone(complete)
		values[EnvLDAPURL] = "ldap://ldap.example.internal:389"
		if _, err := LoadConfig(ModeLDAP, env(values)); err != nil {
			t.Fatalf("LoadConfig(ldap) failed: %v", err)
		}
	})
	required := []struct {
		name string
		key  string
	}{
		{name: "url", key: EnvLDAPURL},
		{name: "bind dn", key: EnvLDAPBindDN},
		{name: "bind password", key: EnvLDAPBindPW},
		{name: "user base dn", key: EnvLDAPUserBaseDN},
		{name: "user filter", key: EnvLDAPUserFilter},
		{name: "allowlist", key: EnvLDAPAllowed},
	}
	for _, field := range required {
		t.Run("missing "+field.name, func(t *testing.T) {
			values := maps.Clone(complete)
			delete(values, field.key)
			_, err := LoadConfig(ModeLDAP, env(values))
			if err == nil {
				t.Fatalf("LoadConfig(ldap) without %s succeeded", field.key)
			}
			if !strings.Contains(err.Error(), field.key) {
				t.Fatalf("error = %q, want it to name %s", err, field.key)
			}
		})
	}
	t.Run("blank allowlist", func(t *testing.T) {
		values := maps.Clone(complete)
		values[EnvLDAPAllowed] = " , ,"
		if _, err := LoadConfig(ModeLDAP, env(values)); err == nil {
			t.Fatalf("blank allowlist accepted")
		}
	})
	t.Run("wrong URL scheme", func(t *testing.T) {
		values := maps.Clone(complete)
		values[EnvLDAPURL] = "https://example.com"
		if _, err := LoadConfig(ModeLDAP, env(values)); err == nil {
			t.Fatalf("https URL accepted for LDAP")
		}
	})
	t.Run("filter without placeholder", func(t *testing.T) {
		values := maps.Clone(complete)
		values[EnvLDAPUserFilter] = "(objectClass=person)"
		if _, err := LoadConfig(ModeLDAP, env(values)); err == nil {
			t.Fatalf("filter without %%s accepted")
		}
	})
}

func TestLoadConfigPublicURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "absent", value: ""},
		{name: "https", value: "https://docs.example.com/docs"},
		{name: "http", value: "http://docs.example.com"},
		{name: "relative", value: "docs.example.com", wantErr: true},
		{name: "ftp", value: "ftp://docs.example.com", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadConfig(ModeNone, env(map[string]string{EnvPublicURL: test.value}))
			if test.wantErr && err == nil {
				t.Fatalf("public URL %q accepted", test.value)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("public URL %q rejected: %v", test.value, err)
			}
		})
	}
}

func TestLoadConfigNilLookup(t *testing.T) {
	if _, err := LoadConfig(ModeNone, nil); err != nil {
		t.Fatalf("LoadConfig(none, nil) failed: %v", err)
	}
	_, err := LoadConfig(ModeLocal, nil)
	if err == nil {
		t.Fatalf("LoadConfig(local, nil) accepted")
	}
}
