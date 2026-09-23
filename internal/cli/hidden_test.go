package cli

import (
	"context"
	"testing"

	"github.com/lz-wang/m2h/internal/check"
	"github.com/lz-wang/m2h/internal/server"
)

// TestServeHiddenFlagDefaultsAndPins pins the CLI contract for the serve
// command: hidden filtering is on by default, --hidden admits hidden paths
// and --no-hidden restores the default explicitly.
func TestServeHiddenFlagDefaultsAndPins(t *testing.T) {
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
	if captured.Hidden {
		t.Fatalf("serve hidden = true by default, want false")
	}

	if _, _, err := runCommand(t, "--hidden", "./docs"); err != nil {
		t.Fatalf("serve error = %v", err)
	}
	if !captured.Hidden {
		t.Fatalf("--hidden left serve hidden = false")
	}

	if _, _, err := runCommand(t, "--no-hidden", "./docs"); err != nil {
		t.Fatalf("serve error = %v", err)
	}
	if captured.Hidden {
		t.Fatalf("--no-hidden left serve hidden = true")
	}
}

// TestCheckHiddenFlagDefaultsAndPins pins the same contract for the check
// subcommand, so both commands always see the same document scope.
func TestCheckHiddenFlagDefaultsAndPins(t *testing.T) {
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
	if captured.Hidden {
		t.Fatalf("check hidden = true by default, want false")
	}

	if _, _, err := runCommand(t, "check", "--hidden", "./docs"); err != nil {
		t.Fatalf("check error = %v", err)
	}
	if !captured.Hidden {
		t.Fatalf("--hidden left check hidden = false")
	}

	if _, _, err := runCommand(t, "check", "--no-hidden", "./docs"); err != nil {
		t.Fatalf("check error = %v", err)
	}
	if captured.Hidden {
		t.Fatalf("--no-hidden left check hidden = true")
	}
}
