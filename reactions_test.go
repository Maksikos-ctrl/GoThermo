package main

import "testing"

func TestValidReaction(t *testing.T) {
	for _, ok := range []string{"👍", "❤️", "🎉", "👨‍👩‍👧", "🇸🇰", "✅", "🙏🏽"} {
		if !validReaction(ok) {
			t.Errorf("%q should be accepted", ok)
		}
	}
	for _, bad := range []string{"", "a", "hello", "1", "👍 ", "\n", "👍\x00", "😀😀😀😀😀😀😀😀😀😀😀😀😀", "\xff"} {
		if validReaction(bad) {
			t.Errorf("%q should be refused", bad)
		}
	}
}
