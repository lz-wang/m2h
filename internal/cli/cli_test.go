package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func runCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command, err := New("dev-20260809-fe65804", &stdout, &stderr)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	err = command.Run(context.Background(), append([]string{"m2h"}, args...))
	return stdout.String(), stderr.String(), err
}

func TestVersionCommandsMatch(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"version"}, {"--version"}} {
		stdout, stderr, err := runCommand(t, args...)
		if err != nil {
			t.Fatalf("m2h %v returned error: %v", args, err)
		}
		if stderr != "" {
			t.Fatalf("m2h %v wrote stderr %q", args, stderr)
		}
		if got, want := stdout, "dev-20260809-fe65804\n"; got != want {
			t.Fatalf("m2h %v output = %q, want %q", args, got, want)
		}
	}
}

func TestRootHelpDocumentsCommands(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := runCommand(t)
	if err != nil {
		t.Fatalf("root help returned error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("root help wrote stderr %q", stderr)
	}
	for _, want := range []string{"convert", "preview", "view", "version", "--version", "-v"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("root help does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestCommandHelpDocumentsContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		command string
		want    []string
	}{
		{
			command: "convert",
			want: []string{
				"--output", "-o", "--glob", "--depth", "-d", "(default: 2)",
				"--mode", "(default: \"auto\")", "--copy-assets", "(default: true)",
				"--unsafe-html",
			},
		},
		{
			command: "preview",
			want: []string{
				"--host", "(default: \"127.0.0.1\")", "--port", "-p", "(default: 8793)",
				"--browser", "--mode", "(default: \"auto\")", "--unsafe-html", "--glob",
				"--depth", "-d", "(default: 2)",
			},
		},
		{command: "view", want: []string{"--mode", "(default: \"auto\")"}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.command, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, err := runCommand(t, test.command, "--help")
			if err != nil {
				t.Fatalf("help returned error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("help wrote stderr %q", stderr)
			}
			for _, want := range test.want {
				if !strings.Contains(stdout, want) {
					t.Errorf("help output does not contain %q:\n%s", want, stdout)
				}
			}
		})
	}
}

func TestUnknownFlagsReturnStableError(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"--unknown"}, {"convert", "README.md", "--unknown"}} {
		_, _, err := runCommand(t, args...)
		if err == nil {
			t.Fatalf("m2h %v succeeded, want error", args)
		}
		if got, want := err.Error(), "Error: unknown option"; got != want {
			t.Fatalf("m2h %v error = %q, want %q", args, got, want)
		}
	}
}

func TestFeatureHandlersReportDevelopmentError(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"convert", "preview", "view"} {
		_, _, err := runCommand(t, command, "README.md")
		if err == nil {
			t.Fatalf("m2h %s succeeded, want error", command)
		}
		want := "Error: " + command + " is not implemented in this release"
		if got := err.Error(); got != want {
			t.Fatalf("m2h %s error = %q, want %q", command, got, want)
		}
	}
}

func TestModeValidationRunsBeforeFeatureHandlers(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"convert", "preview", "view"} {
		_, _, err := runCommand(t, command, "README.md", "--mode", "sepia")
		if err == nil {
			t.Fatalf("m2h %s accepted an invalid mode", command)
		}
		if got, want := err.Error(), "Error: --mode must be one of light, dark, or auto"; got != want {
			t.Fatalf("m2h %s error = %q, want %q", command, got, want)
		}
	}
}

func TestInvalidFlagValueGetsStablePrefix(t *testing.T) {
	t.Parallel()

	_, _, err := runCommand(t, "preview", "README.md", "--port", "many")
	if err == nil {
		t.Fatal("preview accepted a non-integer port")
	}
	if !strings.HasPrefix(err.Error(), "Error: ") {
		t.Fatalf("error = %q, want stable Error prefix", err)
	}
}

func TestInvalidVersionFailsConstruction(t *testing.T) {
	t.Parallel()

	if _, err := New("invalid", &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("New() succeeded with an invalid version")
	}
}
