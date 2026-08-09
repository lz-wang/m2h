package webui

import (
	"io/fs"
	"strings"
	"testing"
)

func TestContentIncludesIndex(t *testing.T) {
	t.Parallel()

	index, err := fs.ReadFile(Content(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `id="root"`) {
		t.Fatalf("embedded index is missing root mount: %s", index)
	}
}
