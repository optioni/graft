package add_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/add"
	"github.com/optioni/graft/internal/ui"
)

// plainUI renders without colour, into buffers nothing reads: the lines are the assertion,
// and where they end up is internal/cli's decision.
func plainUI() *ui.UI { return ui.New(&bytes.Buffer{}, &bytes.Buffer{}, false) }

// The manifest edit is printed before the sync report, because it is the thing the reader
// has to notice: a file they own changed.
func TestReportPrintsTheManifestEditBeforeTheSyncReport(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	got, err := add.Run(c.request(r, "v1.0.0", "agent:reviewer"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	lines := got.Lines(plainUI())
	if len(lines) == 0 {
		t.Fatal("the report has no lines")
	}
	if want := `graft.toml: added source "shared" at v1.0.0`; lines[0] != want {
		t.Errorf("first line = %q, want %q", lines[0], want)
	}
	if !slices.ContainsFunc(lines[1:], func(l string) bool { return strings.Contains(l, "agent:reviewer") }) {
		t.Errorf("the sync report does not follow the edit:\n%q", lines)
	}
}

// Moving a pin and adding a selector are two edits, and they are reported in the order
// they happened: the pin first, because it decides what the selector resolves against.
func TestReportNamesAMovedPinBeforeAnAddedSelector(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")
	r.write("extras/schemas/tdd/templates/tasks.md", "# tasks\n")
	r.commit("v2")
	r.tag("v2.0.0")

	c := newConsumer(t)
	c.file("graft.toml", manifestFor(r, "v1.0.0", "agent:reviewer"))
	if _, err := add.Run(c.request(r, "v1.0.0", "agent:reviewer")); err != nil {
		t.Fatalf("seeding the lock: %v", err)
	}

	got, err := add.Run(c.request(r, "v2.0.0", "schema:tdd"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{
		`graft.toml: moved source "shared" to v2.0.0`,
		`graft.toml: added schema:tdd to source "shared"`,
	}
	if len(got.Edits) != 2 || !slices.Equal(got.Edits, want) {
		t.Errorf("edits = %q, want %q", got.Edits, want)
	}
}

// An add that changed nothing says so, rather than printing a sync report with no
// explanation of why the command was run at all.
func TestReportSaysUnchangedWhenTheManifestAlreadySaidIt(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.file("graft.toml", manifestFor(r, "v1.0.0", "agent:reviewer"))

	got, err := add.Run(c.request(r, "", "agent:reviewer"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if lines := got.Lines(plainUI()); lines[0] != "graft.toml: unchanged" {
		t.Errorf("first line = %q, want %q", lines[0], "graft.toml: unchanged")
	}
}

// Under --no-sync there is no sync report to follow, and the edit lines stand alone.
func TestReportUnderNoSyncIsTheEditAlone(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	req := c.request(r, "v1.0.0", "agent:reviewer")
	req.NoSync = true

	got, err := add.Run(req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{`graft.toml: added source "shared" at v1.0.0`}
	if lines := got.Lines(plainUI()); !slices.Equal(lines, want) {
		t.Errorf("lines = %q, want %q", lines, want)
	}
}
