package main

import (
	"bytes"
	"testing"

	webui "github.com/lz-wang/m2h/web"
)

func TestRunReturnsExitCodeAndRoutesOutput(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "version",
			args:       []string{"m2h", "version"},
			wantCode:   0,
			wantStdout: M2HVersion + "\n",
		},
		{
			name:       "unknown option",
			args:       []string{"m2h", "--unknown"},
			wantCode:   1,
			wantStderr: "Error: unknown option\n",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if got := run(test.args, webui.Content(), &stdout, &stderr); got != test.wantCode {
				t.Fatalf("run() exit code = %d, want %d", got, test.wantCode)
			}
			if got := stdout.String(); got != test.wantStdout {
				t.Fatalf("stdout = %q, want %q", got, test.wantStdout)
			}
			if got := stderr.String(); got != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, test.wantStderr)
			}
		})
	}
}

func TestRunRejectsInvalidInjectedVersion(t *testing.T) {
	previous := M2HVersion
	M2HVersion = "invalid"
	t.Cleanup(func() {
		M2HVersion = previous
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := run([]string{"m2h", "version"}, nil, &stdout, &stderr); got != 1 {
		t.Fatalf("run() exit code = %d, want 1", got)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("invalid m2h version")) {
		t.Fatalf("stderr = %q, want invalid version error", stderr.String())
	}
}
