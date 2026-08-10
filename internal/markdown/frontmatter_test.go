package markdown

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseFrontMatterNoFrontMatter(t *testing.T) {
	t.Parallel()

	source := []byte("# Hello\n\n正文内容\n")
	body, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if frontMatter != nil {
		t.Fatalf("frontmatter = %+v, want nil", frontMatter)
	}
	if !reflect.DeepEqual(body, source) {
		t.Fatalf("body = %q, want source unchanged", body)
	}
}

func TestParseFrontMatterMissingClosingDelimiter(t *testing.T) {
	t.Parallel()

	source := []byte("---\n\n普通 Markdown\n")
	body, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if frontMatter != nil {
		t.Fatalf("frontmatter = %+v, want nil (lone --- is a horizontal rule)", frontMatter)
	}
	if !reflect.DeepEqual(body, source) {
		t.Fatalf("body = %q, want source unchanged", body)
	}
}

func TestParseFrontMatterEmptyBlock(t *testing.T) {
	t.Parallel()

	source := []byte("---\n---\n# Hello\n")
	body, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if frontMatter == nil {
		t.Fatal("frontmatter = nil, want empty FrontMatter")
	}
	if len(frontMatter.Entries) != 0 || frontMatter.Date != "" || frontMatter.Tags != nil {
		t.Fatalf("frontmatter = %+v, want empty", frontMatter)
	}
	if string(body) != "# Hello\n" {
		t.Fatalf("body = %q, want %q", body, "# Hello\n")
	}
}

func TestParseFrontMatterDateOnly(t *testing.T) {
	t.Parallel()

	source := []byte("---\ndate: 2026-07-11\n---\n# Title\n")
	body, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if frontMatter == nil {
		t.Fatal("frontmatter = nil")
	}
	if frontMatter.Date != "2026-07-11" {
		t.Fatalf("Date = %q", frontMatter.Date)
	}
	if len(frontMatter.Entries) != 1 || frontMatter.Entries[0].Key != "date" {
		t.Fatalf("Entries = %+v", frontMatter.Entries)
	}
	if string(body) != "# Title\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestParseFrontMatterDateTime(t *testing.T) {
	t.Parallel()

	source := []byte("---\ndate: 2026-07-11T14:32:00+08:00\n---\nbody\n")
	_, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if frontMatter.Date != "2026-07-11" {
		t.Fatalf("Date = %q, want date-only summary", frontMatter.Date)
	}
	if frontMatter.Entries[0].Value != "2026-07-11T14:32:00+08:00" {
		t.Fatalf("full value = %q", frontMatter.Entries[0].Value)
	}
}

func TestParseFrontMatterInvalidDate(t *testing.T) {
	t.Parallel()

	source := []byte("---\ndate: abc\n---\nbody\n")
	_, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if frontMatter.Date != "" {
		t.Fatalf("Date = %q, want empty for invalid date", frontMatter.Date)
	}
	if len(frontMatter.Entries) != 1 || frontMatter.Entries[0].Key != "date" {
		t.Fatalf("invalid date should still appear in the table: %+v", frontMatter.Entries)
	}
}

func TestParseFrontMatterTags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "sequence",
			source: "---\ntags:\n  - Go\n  - Markdown\n---\nbody\n",
			want:   []string{"Go", "Markdown"},
		},
		{
			name:   "inline",
			source: "---\ntags: [Go, Markdown]\n---\nbody\n",
			want:   []string{"Go", "Markdown"},
		},
		{
			name:   "scalar",
			source: "---\ntags: Go\n---\nbody\n",
			want:   []string{"Go"},
		},
		{
			name:   "comma not split",
			source: "---\ntags: Go, Markdown\n---\nbody\n",
			want:   []string{"Go, Markdown"},
		},
		{
			name:   "duplicates deduped",
			source: "---\ntags:\n  - Go\n  - Go\n---\nbody\n",
			want:   []string{"Go"},
		},
	}

	for _, tc := range cases {
		_, frontMatter, err := ParseFrontMatter([]byte(tc.source))
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if !reflect.DeepEqual(frontMatter.Tags, tc.want) {
			t.Errorf("%s: Tags = %v, want %v", tc.name, frontMatter.Tags, tc.want)
		}
	}
}

func TestParseFrontMatterFieldOrderPreserved(t *testing.T) {
	t.Parallel()

	source := []byte("---\ntitle: m2h\ndate: 2026-07-11\ntags:\n  - Go\nauthor: lzwang\ndraft: false\n---\nbody\n")
	_, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantKeys := []string{"title", "date", "tags", "author", "draft"}
	if len(frontMatter.Entries) != len(wantKeys) {
		t.Fatalf("Entries = %+v", frontMatter.Entries)
	}
	for i, want := range wantKeys {
		if frontMatter.Entries[i].Key != want {
			t.Fatalf("entry %d key = %q, want %q (order not preserved)", i, frontMatter.Entries[i].Key, want)
		}
	}
}

func TestParseFrontMatterNestedAndArbitraryValues(t *testing.T) {
	t.Parallel()

	source := []byte("---\nauthor: lzwang\ncount: 12\ndraft: false\ntags:\n  - Go\n  - Markdown\nproject:\n  name: m2h\n  language: Go\n---\nbody\n")
	_, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	values := make(map[string]string, len(frontMatter.Entries))
	for _, entry := range frontMatter.Entries {
		values[entry.Key] = entry.Value
	}

	if values["author"] != "lzwang" {
		t.Errorf("author value = %q", values["author"])
	}
	if values["count"] != "12" {
		t.Errorf("count value = %q", values["count"])
	}
	if values["draft"] != "false" {
		t.Errorf("draft value = %q", values["draft"])
	}
	if values["tags"] != "- Go\n- Markdown" {
		t.Errorf("tags value = %q", values["tags"])
	}
	if values["project"] != "name: m2h\nlanguage: Go" {
		t.Errorf("project value = %q", values["project"])
	}
}

func TestParseFrontMatterInvalidYAML(t *testing.T) {
	t.Parallel()

	source := []byte("---\ntags: [\n---\nbody\n")
	_, _, err := ParseFrontMatter(source)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestParseFrontMatterNonMapping(t *testing.T) {
	t.Parallel()

	source := []byte("---\njust text\n---\nbody\n")
	_, _, err := ParseFrontMatter(source)
	if err == nil {
		t.Fatal("expected error for non-mapping frontmatter, got nil")
	}
	if !strings.Contains(err.Error(), "mapping") {
		t.Fatalf("error = %v, want it to mention mapping", err)
	}
}

func TestParseFrontMatterHorizontalRuleNotFrontMatter(t *testing.T) {
	t.Parallel()

	// A leading paragraph followed by a horizontal rule must not be parsed as
	// frontmatter because the file does not open with "---".
	source := []byte("intro\n\n---\n\nmore\n")
	body, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if frontMatter != nil {
		t.Fatalf("frontmatter = %+v, want nil", frontMatter)
	}
	if !reflect.DeepEqual(body, source) {
		t.Fatalf("body = %q, want source unchanged", body)
	}
}
