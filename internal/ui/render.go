package ui

import (
	"fmt"
	"strings"
)

// shortSHA is how many characters of a resolved sha graft shows a person.
const shortSHA = 7

// ShortSHA returns the first [shortSHA] characters of a resolved sha, or sha unchanged
// when it is no longer than that.
//
// It lives here rather than in a renderer because two commands print a shortened sha and
// six characters in one and seven in the other is a defect no test of either alone would
// catch.
func ShortSHA(sha string) string {
	if len(sha) <= shortSHA {
		return sha
	}
	return sha[:shortSHA]
}

// FileCount renders a count with its noun: "1 file" or "<n> files".
func FileCount(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

// Pad returns the spaces that take a field of the given length up to width.
//
// The length is the caller's to supply and is the unstyled one: padding computed on styled
// text would move a column the moment colour was enabled.
func Pad(length, width int) string {
	if length >= width {
		return ""
	}
	return strings.Repeat(" ", width-length)
}
