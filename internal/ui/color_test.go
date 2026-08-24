package ui_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/ui"
)

func TestColorEnabled(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		noColor  string
		terminal bool
		want     bool
	}{
		// Unset and present-but-empty are one case by construction: a Getenv-shaped
		// function reports both as "", and graft has no reason to tell them apart.
		"a terminal with NO_COLOR unset":        {"", true, true},
		"a terminal with NO_COLOR empty":        {"", true, true},
		"NO_COLOR=1":                            {"1", true, false},
		"NO_COLOR=0":                            {"0", true, false},
		"NO_COLOR=false":                        {"false", true, false},
		"a redirected stdout":                   {"", false, false},
		"a redirected stdout with NO_COLOR set": {"1", false, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := ui.ColorEnabled(tc.noColor, tc.terminal); got != tc.want {
				t.Errorf("ColorEnabled(%q, %t) = %t, want %t", tc.noColor, tc.terminal, got, tc.want)
			}
		})
	}
}

func TestStyle(t *testing.T) {
	t.Parallel()

	const s = "removed  agent:phase-orchestrator"

	t.Run("colour off returns the input byte for byte", func(t *testing.T) {
		t.Parallel()

		u := ui.New(&bytes.Buffer{}, &bytes.Buffer{}, false)
		if got := u.Bold(s); got != s {
			t.Errorf("Bold() = %q, want %q", got, s)
		}
		if got := u.Dim(s); got != s {
			t.Errorf("Dim() = %q, want %q", got, s)
		}
	})

	t.Run("colour on wraps the input in escapes", func(t *testing.T) {
		t.Parallel()

		u := ui.New(&bytes.Buffer{}, &bytes.Buffer{}, true)
		for name, got := range map[string]string{"Bold": u.Bold(s), "Dim": u.Dim(s)} {
			if !strings.HasPrefix(got, "\x1b[") {
				t.Errorf("%s() = %q, want a leading escape sequence", name, got)
			}
			if !strings.HasSuffix(got, "\x1b[0m") {
				t.Errorf("%s() = %q, want a trailing reset", name, got)
			}
			if !strings.Contains(got, s) {
				t.Errorf("%s() = %q, want it to contain %q", name, got, s)
			}
		}
		if u.Bold(s) == u.Dim(s) {
			t.Errorf("Bold and Dim produced the same string %q", u.Bold(s))
		}
	})
}

// TestIsTerminal asserts the negative direction only, against four real objects.
//
// /dev/null is the case that earns the golang.org/x/term dependency: it has the
// character-device mode bit set, so the dependency-free fi.Mode()&os.ModeCharDevice
// idiom answers "terminal" for `graft sync > /dev/null` — the exact redirection the
// colour rule exists to catch.
//
// The positive direction is asserted nowhere, deliberately. `go test` runs with stdout on
// a pipe, and a pty is not a way out: on darwin /dev/ptmx opens successfully and
// term.IsTerminal on the master returns false, so a "skip when unavailable" test would
// never skip — it would fail permanently, here and on the macos-latest CI leg. Getting it
// to pass needs the slave side through darwin ioctl numbers or a third dependency for one
// assertion. What is left unasserted is term.IsTerminal returning true, which is
// golang.org/x/term's own contract; the decision that consumes it is covered in both
// directions by TestColorEnabled, and the wiring by TestMainColorFollowsStdout.
func TestIsTerminal(t *testing.T) {
	t.Parallel()

	t.Run("an in-memory buffer is not a terminal", func(t *testing.T) {
		t.Parallel()
		if ui.IsTerminal(&bytes.Buffer{}) {
			t.Error("IsTerminal(*bytes.Buffer) = true, want false")
		}
	})

	t.Run("neither end of a pipe is a terminal", func(t *testing.T) {
		t.Parallel()

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe: %v", err)
		}
		t.Cleanup(func() { _ = r.Close(); _ = w.Close() })

		if ui.IsTerminal(w) {
			t.Error("IsTerminal(pipe write end) = true, want false")
		}
		if ui.IsTerminal(r) {
			t.Error("IsTerminal(pipe read end) = true, want false")
		}
	})

	t.Run("a character device is not a terminal", func(t *testing.T) {
		t.Parallel()

		f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			t.Fatalf("opening %s: %v", os.DevNull, err)
		}
		t.Cleanup(func() { _ = f.Close() })

		// The premise: a mode check would get this wrong.
		fi, err := f.Stat()
		if err != nil {
			t.Fatalf("stat %s: %v", os.DevNull, err)
		}
		if fi.Mode()&os.ModeCharDevice == 0 {
			t.Fatalf("%s is not a character device on this platform; the case no longer bites", os.DevNull)
		}

		if ui.IsTerminal(f) {
			t.Errorf("IsTerminal(%s) = true, want false", os.DevNull)
		}
	})
}
