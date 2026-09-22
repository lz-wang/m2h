package cli

import (
	"context"
	"testing"

	"github.com/lz-wang/m2h/internal/check"
	"github.com/lz-wang/m2h/internal/server"
)

// TestServeGitignoreFlagDefaultsAndInverse pins the CLI contract: gitignore
// is on by default and --no-gitignore turns it off for the server.
func TestServeGitignoreFlagDefaultsAndInverse(t *testing.T) {
	previous := runServer
	t.Cleanup(func() { runServer = previous })

	var captured server.Options
	runServer = func(_ context.Context, options server.Options) error {
		captured = options
		return nil
	}

	if _, _, err := runCommand(t, "./docs"); err != nil {
		t.Fatalf("serve error = %v", err)
	}
	if !captured.Gitignore {
		t.Fatalf("serve gitignore = false by default, want true")
	}

	if _, _, err := runCommand(t, "--no-gitignore", "./docs"); err != nil {
		t.Fatalf("serve error = %v", err)
	}
	if captured.Gitignore {
		t.Fatalf("--no-gitignore left serve gitignore = true")
	}
}

// TestCheckGitignoreFlagDefaultsAndInverse pins the same contract for the
// check subcommand, so both commands always see the same document scope.
func TestCheckGitignoreFlagDefaultsAndInverse(t *testing.T) {
	previous := runCheck
	t.Cleanup(func() { runCheck = previous })

	var captured check.Options
	runCheck = func(_ context.Context, options check.Options) (check.Result, error) {
		captured = options
		return check.Result{}, nil
	}

	if _, _, err := runCommand(t, "check", "./docs"); err != nil {
		t.Fatalf("check error = %v", err)
	}
	if !captured.Gitignore {
		t.Fatalf("check gitignore = false by default, want true")
	}

	if _, _, err := runCommand(t, "check", "--no-gitignore", "./docs"); err != nil {
		t.Fatalf("check error = %v", err)
	}
	if captured.Gitignore {
		t.Fatalf("--no-gitignore left check gitignore = true")
	}
}
