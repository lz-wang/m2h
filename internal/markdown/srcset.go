package markdown

import "strings"

// srcsetURLRange locates only a candidate URL. Descriptors, separators and
// whitespace remain byte-for-byte intact; the browser owns their validation.
type srcsetURLRange struct{ start, end int }

// srcsetURLRanges follows the HTML srcset splitting and descriptor-tokenizer
// states: URL tokens end at ASCII whitespace, trailing commas separate bare
// candidates, and commas within descriptor parentheses do not split candidates.
// In particular, a data URL's embedded commas are part of its URL token.
// https://html.spec.whatwg.org/multipage/images.html#parse-a-srcset-attribute
func srcsetURLRanges(value string) []srcsetURLRange {
	var ranges []srcsetURLRange
	for position := 0; position < len(value); {
		for position < len(value) && (isASCIISpace(value[position]) || value[position] == ',') {
			position++
		}
		start := position
		for position < len(value) && !isASCIISpace(value[position]) {
			position++
		}
		end := position
		for end > start && value[end-1] == ',' {
			end--
		}
		if end > start {
			ranges = append(ranges, srcsetURLRange{start, end})
		}
		if end < position {
			continue
		}
		inParens := false
		for position < len(value) {
			char := value[position]
			position++
			if inParens {
				if char == ')' {
					inParens = false
				}
			} else {
				if char == ',' {
					break
				}
				if char == '(' {
					inParens = true
				}
			}
		}
	}
	return ranges
}

func rewriteSrcset(value string, options RenderOptions) string {
	var rewritten strings.Builder
	copied := 0
	for _, span := range srcsetURLRanges(value) {
		rewritten.WriteString(value[copied:span.start])
		rewritten.Write(rewriteDestination([]byte(value[span.start:span.end]), options, true))
		copied = span.end
	}
	rewritten.WriteString(value[copied:])
	return rewritten.String()
}
