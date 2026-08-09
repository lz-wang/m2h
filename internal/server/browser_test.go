package server

import (
	"strings"
	"testing"
)

func TestBrowserCommandUsesPlatformLauncher(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos     string
		wantName string
		wantArg  string
	}{
		{goos: "darwin", wantName: "open", wantArg: "http://localhost/"},
		{goos: "windows", wantName: "rundll32", wantArg: "url.dll,FileProtocolHandler"},
		{goos: "linux", wantName: "xdg-open", wantArg: "http://localhost/"},
		{goos: "freebsd", wantName: "xdg-open", wantArg: "http://localhost/"},
		{goos: "openbsd", wantName: "xdg-open", wantArg: "http://localhost/"},
		{goos: "netbsd", wantName: "xdg-open", wantArg: "http://localhost/"},
	}
	for _, test := range tests {
		name, arguments, err := browserCommand(test.goos, "http://localhost/")
		if err != nil {
			t.Fatalf("browserCommand(%q) error = %v", test.goos, err)
		}
		if name != test.wantName || !strings.Contains(strings.Join(arguments, " "), test.wantArg) {
			t.Errorf("browserCommand(%q) = %q %v", test.goos, name, arguments)
		}
	}
	if _, _, err := browserCommand("plan9", "http://localhost/"); err == nil {
		t.Fatal("browserCommand() accepted an unsupported platform")
	}
}
