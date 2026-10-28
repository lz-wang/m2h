package server

import (
	"fmt"
	"strings"
)

// NormalizeBasePath validates an HTTP deployment prefix. The empty result means
// origin root; filesystem and workspace paths never pass through this function.
func NormalizeBasePath(value string) (string, error) {
	if value == "" || value == "/" {
		return "", nil
	}
	invalid := func() (string, error) {
		return "", fmt.Errorf("invalid --base-path %q: expected / or an absolute path of ASCII URL segments (letters, digits, -, _, ., ~), without empty, . or .. segments", value)
	}
	if !strings.HasPrefix(value, "/") {
		return invalid()
	}
	normalized := strings.TrimSuffix(value, "/")
	for _, segment := range strings.Split(normalized[1:], "/") {
		if segment == "" || segment == "." || segment == ".." {
			return invalid()
		}
		for _, char := range segment {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("-_.~", char)) {
				return invalid()
			}
		}
	}
	return normalized, nil
}
