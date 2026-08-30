package ui_test

import (
	"io"
	"testing"

	"github.com/optioni/graft/internal/ui"
)

func TestFileCount(t *testing.T) {
	t.Parallel()

	for n, want := range map[int]string{1: "1 file", 6: "6 files", 0: "0 files"} {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			if got := ui.FileCount(n); got != want {
				t.Errorf("FileCount(%d) = %q, want %q", n, got, want)
			}
		})
	}
}

func TestShortSHA(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5": "fae2a30",
		// Neither padded nor truncated: a value shorter than the width comes back whole,
		// and one exactly at it is already its own short form.
		"abc":     "abc",
		"abcdefg": "abcdefg",
	} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			if got := ui.ShortSHA(in); got != want {
				t.Errorf("ShortSHA(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestPad(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		length, width int
		want          string
	}{
		"a field short of the width": {3, 7, "    "},
		"a field exactly at it":      {7, 7, ""},
		"a field already beyond it":  {9, 7, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := ui.Pad(tc.length, tc.width); got != tc.want {
				t.Errorf("Pad(%d, %d) = %q, want %q", tc.length, tc.width, got, tc.want)
			}
		})
	}
}

// Pad takes a length rather than a string, which is what makes "padding is computed on
// unstyled text" enforceable: styling changes a field's length by seven bytes it does not
// occupy on screen, and a Pad that measured the styled text would move a column the moment
// colour was enabled.
func TestPadIsComputedOnUnstyledText(t *testing.T) {
	t.Parallel()

	const field, width = "added", 9
	styled := ui.New(io.Discard, io.Discard, true).Bold(field)

	if len(styled) == len(field) {
		t.Fatal("styling did not change the string's length; the case this guards cannot arise")
	}
	if got, want := ui.Pad(len(field), width), "    "; got != want {
		t.Errorf("Pad on the unstyled length = %q, want %q", got, want)
	}
	if got := ui.Pad(len(styled), width); got != "" {
		t.Errorf("Pad on the styled length = %q; a renderer that passed it would lose the column", got)
	}
}
