package files

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"
	"unicode"
)

// DecodeRelativePath converts a percent-encoded slash-style relative path —
// exactly as a browser address or a Markdown destination writes it — into the
// cleaned, root-relative slash path the filesystem boundary expects: bounded
// repeated percent-decoding, backslash normalization, rejection of empty,
// NUL, absolute and Windows-volume paths and of ".." segments, then
// path.Clean. It is the single decoder shared by the document server's
// request handling and the check command's reference resolution, so both can
// never disagree about which path a URL names.
//
// On failure it returns the best-effort decoded value alongside the error so
// diagnostics can still show the path it rejected.
func DecodeRelativePath(value string) (string, error) {
	for iteration := range 8 {
		decoded, err := url.PathUnescape(value)
		if err != nil {
			return "", fmt.Errorf("decode path: %w", err)
		}
		if decoded == value {
			break
		}
		value = decoded
		if iteration == 7 {
			return "", errors.New("path exceeds decoding limit")
		}
	}
	value = strings.ReplaceAll(value, "\\", "/")
	if value == "" || strings.ContainsRune(value, '\x00') || path.IsAbs(value) || hasWindowsVolume(value) {
		return value, errors.New("path must be relative")
	}
	if slices.Contains(strings.Split(value, "/"), "..") {
		return value, errors.New("path escapes root")
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return cleaned, errors.New("invalid relative path")
	}
	return cleaned, nil
}

// hasWindowsVolume reports whether value starts with a Windows drive prefix
// (C:), which can never name a path inside a served workspace root.
func hasWindowsVolume(value string) bool {
	return len(value) >= 2 && unicode.IsLetter(rune(value[0])) && value[1] == ':'
}
