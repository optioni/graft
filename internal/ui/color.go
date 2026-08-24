package ui

import (
	"io"
	"os"

	"golang.org/x/term"
)

// ANSI select-graphic-rendition sequences. Two styles, not a palette: they are the minimum
// that makes "colour is dropped" an observable behavior rather than a flag nobody reads,
// and they are the two SPEC.md's report format needs — a bold item verb and a dimmed
// trailing note.
const (
	bold  = "\x1b[1m"
	dim   = "\x1b[2m"
	reset = "\x1b[0m"
)

// ColorEnabled reports whether styled output is allowed.
//
// It is SPEC.md's rule as written: colour is dropped when stdout is not a terminal or
// NO_COLOR is set. One decision is taken for the whole run and applied to both streams,
// including the error stream where graft's coloured output actually goes. Read literally
// that means a run whose stdout is redirected prints no colour anywhere — kept on purpose,
// because one decision cannot make two streams disagree, and because the alternative writes
// escape sequences into a captured log.
//
// noColor is the value of the NO_COLOR environment variable. Present and non-empty disables
// colour whatever the value, which is the published convention: a truthiness test would
// enable colour for NO_COLOR=0 and NO_COLOR=false. Absent and present-but-empty are one
// case, because a Getenv-shaped reader cannot tell them apart and graft has no reason to.
//
// The function is pure, so the rule is testable without a terminal; [IsTerminal] is the one
// impure half.
func ColorEnabled(noColor string, terminal bool) bool {
	return noColor == "" && terminal
}

// IsTerminal reports whether w is a terminal. Anything that is not an [os.File] is not.
//
// The question is asked of the operating system rather than of the file's mode bits. A
// character device is not a terminal: os.DevNull has the character-device bit set, so a
// mode check would report `graft sync > /dev/null` as a terminal and emit colour into it.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// Bold styles s when colour is enabled, and returns s unchanged when it is not.
func (u *UI) Bold(s string) string { return u.style(bold, s) }

// Dim styles s when colour is enabled, and returns s unchanged when it is not.
func (u *UI) Dim(s string) string { return u.style(dim, s) }

// style returns s unchanged rather than wrapping it in empty sequences, so a report line is
// byte-identical with colour off to what a caller would have written by hand.
func (u *UI) style(code, s string) string {
	if !u.color {
		return s
	}
	return code + s + reset
}
