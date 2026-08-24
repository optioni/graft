package sync_test

import (
	"fmt"
	"io"
	"slices"
	"testing"

	"github.com/optioni/graft/internal/sync"
	"github.com/optioni/graft/internal/ui"
)

// assertItems compares a report's item lines as "verb id files [note]", which is what the
// scenarios describe. The exact rendering is pinned by internal/sync's own tests; here the
// question is which items a dry run names.
func assertItems(t *testing.T, r *sync.Report, want ...string) {
	t.Helper()
	var got []string
	for _, s := range r.Sources {
		for _, it := range s.Items {
			line := fmt.Sprintf("%s %s %d", it.Verb, it.ID, it.Files)
			if it.Note != "" {
				line += " " + it.Note
			}
			got = append(got, line)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("items:\n got %q\nwant %q", got, want)
	}
}

func assertSummary(t *testing.T, r *sync.Report, want string) {
	t.Helper()
	lines := r.Lines(ui.New(io.Discard, io.Discard, false))
	if got := lines[len(lines)-1]; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

// --dry-run stops after the plan is built. SPEC.md's promise is that it touches nothing,
// "including creating no directories" — which is why these assert the whole tree rather
// than the absence of a few files.
//
// The fetch still happens: there is no plan without a catalog and no catalog without a
// fetch. The cache is not the working tree.
func TestRunDryRunCreatesNothing(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")

	rep, err := c.dryRun()
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}

	// The report names what would be added, which is the half of --dry-run that makes the
	// flag worth having: the tree assertion below only proves it did nothing.
	assertItems(t, rep, "added agent:reviewer 1", "added schema:tdd 2")
	assertSummary(t, rep, "3 files to write, 0 to remove - nothing written")

	c.assertEntries("graft.toml")
}

func TestRunDryRunDeletesNothing(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	lockBefore := c.read("graft.lock")

	c.manifest(r, "v1.0.0", "schema:tdd")

	rep, err := c.dryRun()
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	assertItems(t, rep, "removed agent:reviewer 1 no longer installed")
	assertSummary(t, rep, "2 files to write, 1 to remove - nothing written")

	c.assertEntries(installed()...)
	if got := c.read(".claude/agents/reviewer.md"); got != "# reviewer\n" {
		t.Errorf("the dropped item's file was deleted: %q", got)
	}
	if got := c.read("graft.lock"); got != lockBefore {
		t.Errorf("graft.lock changed:\n%s", got)
	}
}

func TestRunDryRunFailsAlike(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0", "schema:tdd-workflwo")
	want := `source "shared": selector "schema:tdd-workflwo" matches no item;` +
		` catalog provides agent:reviewer, schema:tdd`

	_, dryErr := c.dryRun()
	assertError(t, dryErr, want)

	_, err := c.run()
	assertError(t, err, want)

	c.assertEntries("graft.toml")
}
