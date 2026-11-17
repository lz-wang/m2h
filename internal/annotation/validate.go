package annotation

import "fmt"

// ValidateSelector checks one selector against the version contract and
// the field limits. rune-count limits keep multibyte text honest: quote,
// context and comment ceilings are measured in characters, never bytes.
func ValidateSelector(selector Selector) error {
	if selector.Version != SelectorVersion {
		return fmt.Errorf("selector version %d is not supported (expected %d)",
			selector.Version, SelectorVersion)
	}
	exactRunes := runeLen(selector.Quote.Exact)
	if selector.Quote.Exact == "" {
		return fmt.Errorf("quote text must not be empty")
	}
	if exactRunes > MaxQuoteRunes {
		return fmt.Errorf("quote text exceeds %d characters", MaxQuoteRunes)
	}
	if runeLen(selector.Quote.Prefix) > MaxContextRunes {
		return fmt.Errorf("quote prefix exceeds %d characters", MaxContextRunes)
	}
	if runeLen(selector.Quote.Suffix) > MaxContextRunes {
		return fmt.Errorf("quote suffix exceeds %d characters", MaxContextRunes)
	}
	if selector.Position.Start < 0 || selector.Position.End <= selector.Position.Start {
		return fmt.Errorf("position must be a non-empty, forward range")
	}
	if selector.Position.End > MaxPosition {
		return fmt.Errorf("position exceeds the maximum document size")
	}
	if selector.Position.End-selector.Position.Start != exactRunes {
		return fmt.Errorf("position span does not match the quoted text length")
	}
	return nil
}

// ValidateComment checks the free-text comment. The comment is plain text
// everywhere it is displayed — never interpreted as HTML — so validation is
// about size, not content.
func ValidateComment(comment string) error {
	if runeLen(comment) > MaxCommentRunes {
		return fmt.Errorf("comment exceeds %d characters", MaxCommentRunes)
	}
	return nil
}

func runeLen(value string) int {
	count := 0
	for range value {
		count++
	}
	return count
}
