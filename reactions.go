package main

import (
	"unicode"
	"unicode/utf8"
)

const maxReactionRunes = 12

// validReaction accepts what an emoji picker produces (a few characters, some
// of them joiners or variation selectors) and refuses anything that looks like
// text: letters and digits, spaces and control characters.
func validReaction(s string) bool {
	n := utf8.RuneCountInString(s)
	if n == 0 || n > maxReactionRunes || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}
