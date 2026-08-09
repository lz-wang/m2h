package server

import (
	"fmt"
	"os/exec"
	"runtime"
)

func openBrowser(address string) error {
	name, arguments, err := browserCommand(runtime.GOOS, address)
	if err != nil {
		return err
	}
	command := exec.Command(name, arguments...)
	if err := command.Start(); err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}
	if err := command.Process.Release(); err != nil {
		return fmt.Errorf("release %s process: %w", name, err)
	}
	return nil
}

func browserCommand(goos, address string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{address}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", address}, nil
	case "linux", "freebsd", "openbsd", "netbsd":
		return "xdg-open", []string{address}, nil
	default:
		return "", nil, fmt.Errorf("open browser: unsupported platform %q", goos)
	}
}
