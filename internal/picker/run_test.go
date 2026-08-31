package picker

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

// The driver is tested with a byte stream and a raw-mode function that does nothing. There
// is deliberately no pseudo-terminal anywhere in this package: a pty would test the
// operating system's line discipline, need a dependency to create, and be the flakiest
// thing in the suite — while proving nothing about whether `j`, `space`, `enter` chooses
// the second item.

// scripted runs the driver over a fixed key stream, reporting how many times raw mode was
// restored.
func scripted(t *testing.T, keys string, m Model) (sel []string, out string, restores int, err error) {
	t.Helper()

	var buf bytes.Buffer
	term := Terminal{
		In:  strings.NewReader(keys),
		Out: &buf,
		MakeRaw: func() (func(), error) {
			return func() { restores++ }, nil
		},
	}
	sel, err = Run(term, m)
	return sel, buf.String(), restores, err
}

func TestRunChoosesFromAScriptedKeyStream(t *testing.T) {
	t.Parallel()

	sel, out, restores, err := scripted(t, "j \r", New(threeItems()))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !slices.Equal(sel, []string{"agent:reviewer"}) {
		t.Errorf("selectors = %q, want the second item", sel)
	}
	if !strings.Contains(out, "agent:reviewer") {
		t.Errorf("no frame was drawn:\n%q", out)
	}
	if restores != 1 {
		t.Errorf("raw mode restored %d times, want exactly 1", restores)
	}
}

// An arrow key arrives as three bytes. Reading the escape as a bare key would cancel the
// picker every time a user pressed down.
func TestRunReadsAnArrowKeyAsOneEvent(t *testing.T) {
	t.Parallel()

	sel, _, _, err := scripted(t, "\x1b[B \r", New(threeItems()))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !slices.Equal(sel, []string{"agent:reviewer"}) {
		t.Errorf("selectors = %q, want the arrow to have moved the cursor once", sel)
	}
}

func TestRunTreatsAClosedInputAsACancellation(t *testing.T) {
	t.Parallel()

	sel, _, restores, err := scripted(t, "", New(threeItems()))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sel) != 0 {
		t.Errorf("selectors = %q, want none", sel)
	}
	if restores != 1 {
		t.Errorf("raw mode restored %d times, want exactly 1", restores)
	}
}

func TestRunRestoresRawModeAfterAWriteFailure(t *testing.T) {
	t.Parallel()

	restores := 0
	term := Terminal{
		In:  strings.NewReader("j \r"),
		Out: failingWriter{},
		MakeRaw: func() (func(), error) {
			return func() { restores++ }, nil
		},
	}
	if _, err := Run(term, New(threeItems())); err == nil {
		t.Fatal("Run succeeded with a writer that fails")
	}
	if restores != 1 {
		t.Errorf("raw mode restored %d times, want exactly 1", restores)
	}
}

// Raw mode that cannot be entered is the one failure the driver cannot work around: a
// terminal still in cooked mode would echo the keys and line-buffer them.
func TestRunReportsARawModeFailure(t *testing.T) {
	t.Parallel()

	term := Terminal{
		In:      strings.NewReader("\r"),
		Out:     &bytes.Buffer{},
		MakeRaw: func() (func(), error) { return nil, errors.New("not a terminal") },
	}
	if _, err := Run(term, New(threeItems())); err == nil {
		t.Fatal("Run succeeded when raw mode could not be entered")
	}
}

// The collapse offer is answered through the same driver, so the whole flow — select both,
// confirm, accept the glob — is one key stream.
func TestRunAnswersTheCollapseOfferFromTheSameStream(t *testing.T) {
	t.Parallel()

	sel, _, _, err := scripted(t, " j \ry", New(threeItems()))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !slices.Equal(sel, []string{"agent:*"}) {
		t.Errorf("selectors = %q, want the collapsed glob", sel)
	}
}

// The picker leaves the terminal as it found it: the last frame is erased, so the report
// that follows is not printed under a list of checkboxes.
func TestRunErasesItsLastFrame(t *testing.T) {
	t.Parallel()

	_, out, _, err := scripted(t, "q", New(threeItems()))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.HasSuffix(out, "\x1b[J") {
		t.Errorf("the last frame was not erased:\n%q", out)
	}
}

// failingWriter is a stream that cannot be written to at all.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("stream closed") }
