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
	if len(frontMatter.Entries) != 0 || frontMatter.Date != "" || frontMatter.Tags != nil ||
		frontMatter.CreatedDate != "" || frontMatter.UpdatedDate != "" {
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

func TestParseFrontMatterEntryPositions(t *testing.T) {
	t.Parallel()

	source := []byte("---\ntitle: Guide\ndate: yesterday\ntags:\n  - go\n---\nbody\n")
	_, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []struct {
		key    string
		line   int
		column int
	}{
		{key: "title", line: 1, column: 1},
		{key: "date", line: 2, column: 1},
		{key: "tags", line: 3, column: 1},
	}
	if len(frontMatter.Entries) != len(want) {
		t.Fatalf("Entries = %+v, want %d entries", frontMatter.Entries, len(want))
	}
	for index, expected := range want {
		entry := frontMatter.Entries[index]
		if entry.Key != expected.key || entry.Line != expected.line || entry.Column != expected.column {
			t.Fatalf("entry %d = %+v, want key %q at %d:%d (positions are relative to the YAML block)",
				index, entry, expected.key, expected.line, expected.column)
		}
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

func TestParseFrontMatterTitle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "scalar title",
			source: "---\ntitle: m2h 使用指南\n---\n# Markdown to HTML\n",
			want:   "m2h 使用指南",
		},
		{
			name:   "quoted empty title",
			source: "---\ntitle: \"\"\n---\n# Heading\n",
			want:   "",
		},
		{
			name:   "blank title",
			source: "---\ntitle:\n---\n# Heading\n",
			want:   "",
		},
		{
			name:   "surrounding whitespace trimmed",
			source: "---\ntitle: '  padded  '\n---\n",
			want:   "padded",
		},
		{
			name:   "sequence title ignored",
			source: "---\ntitle:\n  - foo\n  - bar\n---\n",
			want:   "",
		},
		{
			name:   "mapping title ignored",
			source: "---\ntitle:\n  main: foo\n---\n",
			want:   "",
		},
	}

	for _, tc := range cases {
		_, frontMatter, err := ParseFrontMatter([]byte(tc.source))
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if frontMatter.Title != tc.want {
			t.Errorf("%s: Title = %q, want %q", tc.name, frontMatter.Title, tc.want)
		}
	}
}

func TestParseFrontMatterTitleStillListedAsEntry(t *testing.T) {
	t.Parallel()

	// A sequence title is rejected as the display title but must stay in the
	// full frontmatter table, exactly like an invalid date.
	source := []byte("---\ntitle:\n  - foo\n  - bar\n---\nbody\n")
	_, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(frontMatter.Entries) != 1 || frontMatter.Entries[0].Key != "title" {
		t.Fatalf("Entries = %+v, want the title entry kept", frontMatter.Entries)
	}
}

func TestPreferredTitle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		frontMatter *FrontMatter
		fallback    string
		want        string
	}{
		{
			name:        "nil frontmatter falls back",
			frontMatter: nil,
			fallback:    "Heading",
			want:        "Heading",
		},
		{
			name:        "empty title falls back",
			frontMatter: &FrontMatter{},
			fallback:    "Heading",
			want:        "Heading",
		},
		{
			name:        "title wins over fallback",
			frontMatter: &FrontMatter{Title: "使用指南"},
			fallback:    "Heading",
			want:        "使用指南",
		},
	}

	for _, tc := range cases {
		if got := PreferredTitle(tc.frontMatter, tc.fallback); got != tc.want {
			t.Errorf("%s: PreferredTitle = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestParseFrontMatterCreatedUpdatedAliases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		source  string
		created string
		updated string
		date    string
	}{
		{
			name:    "create_date outranks create_at and create_time",
			source:  "---\ncreate_date: 2026-08-18\ncreate_at: 2026-08-20\ncreate_time: 2026-08-22\n---\nbody\n",
			created: "2026-08-18",
		},
		{
			name:    "create alias priority is independent of YAML order",
			source:  "---\ncreate_time: 2026-08-22\ncreate_date: 2026-08-18\ncreate_at: 2026-08-20\n---\nbody\n",
			created: "2026-08-18",
		},
		{
			name:    "create_at outranks create_time when create_date is absent",
			source:  "---\ncreate_time: 2026-08-22\ncreate_at: 2026-08-20\n---\nbody\n",
			created: "2026-08-20",
		},
		{
			name:    "update_date outranks update_at and update_time",
			source:  "---\nupdate_date: 2026-08-18\nupdate_at: 2026-08-20\nupdate_time: 2026-08-22\n---\nbody\n",
			updated: "2026-08-18",
		},
		{
			name:    "update alias priority is independent of YAML order",
			source:  "---\nupdate_time: 2026-08-22\nupdate_date: 2026-08-18\nupdate_at: 2026-08-20\n---\nbody\n",
			updated: "2026-08-18",
		},
		{
			name:    "update_at outranks update_time when update_date is absent",
			source:  "---\nupdate_time: 2026-08-22\nupdate_at: 2026-08-20\n---\nbody\n",
			updated: "2026-08-20",
		},
		{
			name:    "create and update and date all normalize independently",
			source:  "---\ncreate_at: 2026-08-20T11:20:00+08:00\nupdate_at: 2026-08-28T19:20:00+08:00\ndate: 2026-08-15\n---\nbody\n",
			created: "2026-08-20",
			updated: "2026-08-28",
			date:    "2026-08-15",
		},
		{
			name:    "create with date",
			source:  "---\ncreate_date: 2026-08-20\ndate: 2026-08-15\n---\nbody\n",
			created: "2026-08-20",
			date:    "2026-08-15",
		},
		{
			name:    "update with date",
			source:  "---\nupdate_date: 2026-08-28\ndate: 2026-08-15\n---\nbody\n",
			updated: "2026-08-28",
			date:    "2026-08-15",
		},
		{
			name:   "date only",
			source: "---\ndate: 2026-08-15\n---\nbody\n",
			date:   "2026-08-15",
		},
		{
			name:   "no date fields at all",
			source: "---\ntitle: m2h\n---\nbody\n",
		},
		{
			name:    "ISO datetime normalizes to the calendar day",
			source:  "---\ncreate_at: 2026-08-28T18:30:22+08:00\n---\nbody\n",
			created: "2026-08-28",
		},
		{
			name:   "time-only value has no date information",
			source: "---\ncreate_time: \"18:30:22\"\n---\nbody\n",
		},
		{
			name:   "sequence and mapping values never reach the summary",
			source: "---\ncreate_date:\n  - 2026-08-01\nupdate_at:\n  when: 2026-08-02\n---\nbody\n",
		},
		{
			name:    "invalid higher-priority alias does not block a valid lower one",
			source:  "---\ncreate_date: not-a-date\ncreate_at: 2026-08-20\n---\nbody\n",
			created: "2026-08-20",
		},
	}

	for _, tc := range cases {
		_, frontMatter, err := ParseFrontMatter([]byte(tc.source))
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if frontMatter.CreatedDate != tc.created {
			t.Errorf("%s: CreatedDate = %q, want %q", tc.name, frontMatter.CreatedDate, tc.created)
		}
		if frontMatter.UpdatedDate != tc.updated {
			t.Errorf("%s: UpdatedDate = %q, want %q", tc.name, frontMatter.UpdatedDate, tc.updated)
		}
		if frontMatter.Date != tc.date {
			t.Errorf("%s: Date = %q, want %q", tc.name, frontMatter.Date, tc.date)
		}
	}
}

func TestParseFrontMatterAliasKeysStillListedAsEntries(t *testing.T) {
	t.Parallel()

	// The alias keys are derivations, not replacements: the full frontmatter
	// table keeps every raw key/value pair exactly as authored.
	source := []byte("---\ncreate_time: \"18:30:22\"\nupdate_at: 2026-08-28\n---\nbody\n")
	_, frontMatter, err := ParseFrontMatter(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantKeys := []string{"create_time", "update_at"}
	if len(frontMatter.Entries) != len(wantKeys) {
		t.Fatalf("Entries = %+v, want %d entries", frontMatter.Entries, len(wantKeys))
	}
	for i, want := range wantKeys {
		if frontMatter.Entries[i].Key != want {
			t.Errorf("entry %d key = %q, want %q", i, frontMatter.Entries[i].Key, want)
		}
	}
	if frontMatter.Entries[0].Value != "18:30:22" {
		t.Errorf("create_time value = %q, want the raw source value", frontMatter.Entries[0].Value)
	}
	if frontMatter.CreatedDate != "" {
		t.Errorf("CreatedDate = %q, want empty for a time-only value", frontMatter.CreatedDate)
	}
	if frontMatter.UpdatedDate != "2026-08-28" {
		t.Errorf("UpdatedDate = %q, want %q", frontMatter.UpdatedDate, "2026-08-28")
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
