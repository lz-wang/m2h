package files

import "testing"

func TestIsActiveWebAsset(t *testing.T) {
	t.Parallel()

	for _, active := range []string{
		"page.html", "page.htm", "page.xhtml", "app.js", "app.mjs", "app.cjs",
		"style.css", "nested/PAGE.HTML", "nested/App.JS",
	} {
		if !IsActiveWebAsset(active) {
			t.Errorf("IsActiveWebAsset(%q) = false, want true", active)
		}
	}
	for _, passive := range []string{
		"image.png", "diagram.svg", "manual.pdf", "archive.zip", "movie.mp4",
		"data.json", "notes.txt", "noext", "htmlish.md.txt",
	} {
		if IsActiveWebAsset(passive) {
			t.Errorf("IsActiveWebAsset(%q) = true, want false", passive)
		}
	}
}
