// Package annotation defines the data model and validation of document
// annotations: reader commentary anchored to spans of rendered document
// text, stored apart from the Markdown source (which stays read-only).
package annotation

// SelectorVersion is the current text-selector algorithm version. Version 1
// positions and quotes address the browser-normalized reading text (the
// concatenated text nodes of the rendered body with UI decorations — line
// numbers, copy buttons, anchors — excluded), measured in Unicode code
// points. A future algorithm bump changes this constant and the stored
// version, letting old anchors stay interpretable.
const SelectorVersion = 1

// Color identifies an annotation's display color.
type Color string

const (
	ColorBlue  Color = "blue"
	ColorGreen Color = "green"
	ColorAmber Color = "amber"
)

// WithDefault returns blue for clients that omit the color.
func (color Color) WithDefault() Color {
	if color == "" {
		return ColorBlue
	}
	return color
}

func (color Color) Valid() bool {
	return color == ColorBlue || color == ColorGreen || color == ColorAmber
}

// Field limits, enforced server-side so a malicious or buggy client cannot
// bloat the database. Positions live in the normalized reading text, so
// they are bounded by a generous document-size ceiling rather than any
// particular document.
const (
	MaxQuoteRunes   = 2000
	MaxCommentRunes = 10000
	MaxContextRunes = 100
	MaxPosition     = 5_000_000
)

// QuoteSelector is the W3C TextQuoteSelector shape: the exact quoted text
// plus up to MaxContextRunes of surrounding context each side.
type QuoteSelector struct {
	Exact  string `json:"exact"`
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
}

// PositionSelector is the W3C TextPositionSelector shape over the
// normalized reading text: a half-open [start, end) range in Unicode code
// point offsets.
type PositionSelector struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Selector pins one annotation to a span of text. Quote and position both
// address the same normalized text, so the quote's rune length equals the
// position span — the pair is redundant by design, quote survives edits,
// position makes lookups O(1).
type Selector struct {
	Quote     QuoteSelector    `json:"quote"`
	Position  PositionSelector `json:"position"`
	HeadingID string           `json:"headingId,omitempty"`
	TabID     string           `json:"tabId,omitempty"`
	BlockHint string           `json:"blockHint,omitempty"`
	Version   int              `json:"version"`
}

// Annotation is one stored annotation. ID is a server-generated UUID;
// RootID and DocumentPath form the stable document identity; AuthorID is
// the authenticated principal that created it; DocumentHash is the
// sha256:… of the Markdown source bytes the annotation was made against.
type Annotation struct {
	ID           string
	RootID       string
	DocumentPath string
	AuthorID     string
	AuthorName   string
	Selector     Selector
	Comment      string
	Color        Color
	DocumentHash string
	CreatedAt    int64
	UpdatedAt    int64
}
