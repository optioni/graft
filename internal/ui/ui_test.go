package ui_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/optioni/graft/internal/ui"
)

func TestPrint(t *testing.T) {
	t.Parallel()

	var out, errs bytes.Buffer
	u := ui.New(&out, &errs, false)
	u.Print("openspec-schemas v1.2.0")

	if got, want := out.String(), "openspec-schemas v1.2.0\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if got := errs.String(); got != "" {
		t.Errorf("stderr = %q, want empty", got)
	}
	if err := u.WriteError(); err != nil {
		t.Errorf("WriteError() = %v, want nil", err)
	}
}

func TestNote(t *testing.T) {
	t.Parallel()

	var out, errs bytes.Buffer
	u := ui.New(&out, &errs, false)
	u.Note(`run "graft --help" for usage`)

	if got, want := errs.String(), "run \"graft --help\" for usage\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	if got := out.String(); got != "" {
		t.Errorf("stdout = %q, want empty", got)
	}
}

func TestFail(t *testing.T) {
	t.Parallel()

	// The message is one internal/catalog actually produces. A message invented for the
	// test would not catch a report that mangles quoting.
	const expansion = `source "shared": selector "agent:*" matches no item; catalog provides schema:tdd`

	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"a failure-mode-table message": {errors.New(expansion), "graft: " + expansion + "\n"},
		"a resolution failure":         {errors.New(`source "shared": rev "v9.9.9" not found`), "graft: source \"shared\": rev \"v9.9.9\" not found\n"},
		"nil":                          {nil, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var out, errs bytes.Buffer
			u := ui.New(&out, &errs, false)
			u.Fail(tc.err)

			if got := errs.String(); got != tc.want {
				t.Errorf("stderr = %q, want %q", got, tc.want)
			}
			if got := out.String(); got != "" {
				t.Errorf("stdout = %q, want empty", got)
			}
		})
	}
}

// failingWriter fails every write, which is how a full disk looks to a caller.
type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

// countingWriter fails every write with a fresh wrapper, so a recorder that overwrote its
// stored error on each write would still be reported as failing — but with the wrong one.
type countingWriter struct {
	err error
	n   int
}

func (w *countingWriter) Write([]byte) (int, error) {
	w.n++
	return 0, fmt.Errorf("write %d: %w", w.n, w.err)
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	full := errors.New("disk full")

	t.Run("a failed write to stdout is recorded", func(t *testing.T) {
		t.Parallel()

		var errs bytes.Buffer
		u := ui.New(failingWriter{full}, &errs, false)
		u.Print("anything")

		if err := u.WriteError(); !errors.Is(err, full) {
			t.Errorf("WriteError() = %v, want %v", err, full)
		}
	})

	t.Run("a failed write to stderr is recorded", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		u := ui.New(&out, failingWriter{full}, false)
		u.Fail(errors.New("something"))

		if err := u.WriteError(); !errors.Is(err, full) {
			t.Errorf("WriteError() = %v, want %v", err, full)
		}
	})

	t.Run("a stream keeps its first failure", func(t *testing.T) {
		t.Parallel()

		first := errors.New("first")
		var errs bytes.Buffer
		u := ui.New(&countingWriter{err: first}, &errs, false)
		u.Print("one")
		u.Print("two")

		if err := u.WriteError(); err == nil || err.Error() != "write 1: first" {
			t.Errorf("WriteError() = %v, want the failure from the first write", err)
		}
	})
}

func TestOutAndErrRecord(t *testing.T) {
	t.Parallel()

	// Out and Err exist so a dependency that renders its own output — cobra's help — writes
	// through the UI rather than around it, and so its write failures are still seen.
	full := errors.New("disk full")

	var errs bytes.Buffer
	u := ui.New(failingWriter{full}, &errs, false)
	if _, err := u.Out().Write([]byte("help text")); err == nil {
		t.Error("writing to Out() returned no error, want the underlying failure")
	}
	if err := u.WriteError(); !errors.Is(err, full) {
		t.Errorf("WriteError() = %v, want %v", err, full)
	}

	var out bytes.Buffer
	v := ui.New(&out, &errs, false)
	if _, err := v.Err().Write([]byte("usage")); err != nil {
		t.Errorf("writing to Err(): %v", err)
	}
	if got := errs.String(); got != "usage" {
		t.Errorf("stderr = %q, want %q", got, "usage")
	}
}
