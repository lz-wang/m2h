package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	urfavecli "github.com/urfave/cli/v3"

	"github.com/lz-wang/m2h/internal/auth"
)

// envLookup reads the process environment for the serve command. It is a
// package variable so tests can inject a deterministic environment.
var envLookup = os.Getenv

// authFlag selects the authentication model of the document server.
func authFlag() *urfavecli.StringFlag {
	return &urfavecli.StringFlag{
		Name:             "auth",
		Value:            string(auth.ModeNone),
		Usage:            "authentication mode: none, local, or ldap",
		ValidateDefaults: true,
		Local:            true,
		Validator: func(value string) error {
			if _, err := auth.ParseMode(value); err != nil {
				return fmt.Errorf("Error: --auth must be one of none, local, or ldap")
			}
			return nil
		},
	}
}

// dataDirFlag declares the SQLite data directory of authenticated runs.
// Precedence lives in resolveDataDir: --data-dir, then M2H_DATA_DIR, then
// the per-OS default.
func dataDirFlag() *urfavecli.StringFlag {
	return &urfavecli.StringFlag{
		Name:  "data-dir",
		Usage: "data directory for the authentication database (default: $M2H_DATA_DIR or the user data directory)",
		Local: true,
	}
}

// resolveDataDir applies the data-directory precedence: an explicit
// --data-dir wins, then M2H_DATA_DIR, then the per-OS default.
func resolveDataDir(command *urfavecli.Command) string {
	if command.IsSet("data-dir") {
		return strings.TrimSpace(command.String("data-dir"))
	}
	if fromEnv := strings.TrimSpace(envLookup("M2H_DATA_DIR")); fromEnv != "" {
		return fromEnv
	}
	return defaultDataDir()
}

// defaultDataDir returns the per-OS default data directory: the user data
// area of the platform with an m2h subdirectory. It is a variable so tests
// can freeze the computed default.
var defaultDataDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "m2h-data")
	}
	switch runtime.GOOS {
	case "windows":
		if appData := envLookup("AppData"); appData != "" {
			return filepath.Join(appData, "m2h")
		}
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "m2h")
	default:
		if dataHome := envLookup("XDG_DATA_HOME"); dataHome != "" {
			return filepath.Join(dataHome, "m2h")
		}
		return filepath.Join(home, ".local", "share", "m2h")
	}
	return filepath.Join(home, ".m2h")
}

// authOptions assembles the auth-related server options from the parsed
// command. Configuration problems surface here — before any port is
// opened — because a half-configured auth mode must not start serving.
// The config is resolved for every mode (so M2H_PUBLIC_URL is validated
// even without authentication); its mode-specific halves stay empty for
// none.
func authOptions(command *urfavecli.Command) (auth.Mode, *auth.Config, string, error) {
	mode, err := auth.ParseMode(command.String("auth"))
	if err != nil {
		return auth.ModeNone, nil, "", fmt.Errorf("Error: --auth must be one of none, local, or ldap")
	}
	dataDir := resolveDataDir(command)
	if dataDir == "" {
		return auth.ModeNone, nil, "", fmt.Errorf("Error: --data-dir must not be empty")
	}
	config, err := auth.LoadConfig(mode, envLookup)
	if err != nil {
		return mode, nil, "", fmt.Errorf("Error: %w", err)
	}
	return mode, &config, dataDir, nil
}
