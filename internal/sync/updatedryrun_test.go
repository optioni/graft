package sync_test

import (
	"io"
	"testing"

	"github.com/optioni/graft/internal/sync"
	"github.com/optioni/graft/internal/ui"
)

// --dry-run under an update is the same early return `graft sync` already had: the run stops
// after the plan is built, before internal/apply is called at all. Since the manifest edit is
// an argument to that call rather than a step of its own, a previewed `--to` cannot reach
// disk — and that is the half of the promise a file-existence check would miss, so these
// assert the whole tree and both files byte for byte.

// A previewed update moves neither of graft's files, even though its whole point is that the
// sha it resolves has moved.
func TestDryRunOfAnUpdateWritesNeitherOfGraftsFiles(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	first := r.commit("v1")

	c := newConsumer(t)
	c.manifest(r, "main")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	manifestBefore, lockBefore := c.read("graft.toml"), c.read("graft.lock")

	r.write("extras/schemas/tdd/templates/spec.md", "# spec\n")
	second := r.commit("v2")

	rep, err := c.dryRunUpdate(sync.Update{})
	if err != nil {
		t.Fatalf("dry run of an update: %v", err)
	}

	// The report says what would move, which is what makes previewing an update worth the
	// flag; the assertions below say nothing did.
	lines := rep.Lines(ui.New(io.Discard, io.Discard, false))
	if want := "shared  main  (" + first[:7] + " -> " + second[:7] + ")"; lines[0] != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}
	assertItems(t, rep, "updated agent:reviewer 1", "updated schema:tdd 3")
	assertSummary(t, rep, "4 files to write, 0 to remove - nothing written")

	if got := c.read("graft.toml"); got != manifestBefore {
		t.Errorf("graft.toml changed:\n%s", got)
	}
	if got := c.read("graft.lock"); got != lockBefore {
		t.Errorf("graft.lock changed:\n%s", got)
	}
	c.assertEntries(installed()...)
	if c.exists("openspec/schemas/tdd/templates/spec.md") {
		t.Error("the new rev's file was written by a dry run")
	}
}

// --to is the one path that writes graft.toml, so a dry run of it is the one that has to be
// asserted against the manifest's own bytes rather than against the tree.
func TestDryRunOfUpdateToLeavesTheManifestWhereItWas(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	first := r.commit("v1")
	r.tag("v1.0.0")
	r.write("extras/schemas/tdd/templates/spec.md", "# spec\n")
	second := r.commit("v2")
	r.tag("v1.1.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	manifestBefore, lockBefore := c.read("graft.toml"), c.read("graft.lock")

	rep, err := c.dryRunUpdate(sync.Update{Source: "shared", To: "v1.1.0"})
	if err != nil {
		t.Fatalf("dry run of --to: %v", err)
	}

	lines := rep.Lines(ui.New(io.Discard, io.Discard, false))
	want := "shared  v1.0.0 -> v1.1.0  (" + first[:7] + " -> " + second[:7] + ")"
	if lines[0] != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}

	if got := c.read("graft.toml"); got != manifestBefore {
		t.Errorf("graft.toml moved on a dry run:\n%s", got)
	}
	if got := c.read("graft.lock"); got != lockBefore {
		t.Errorf("graft.lock changed:\n%s", got)
	}
	c.assertEntries(installed()...)
}

// A first update in a repository with no lock is the run with the most to write, so it is
// the one where "creates no directory" is worth asserting: the tree holds the manifest and
// nothing else, not even the empty destinations the apply would have made.
func TestDryRunOfAFirstUpdateCreatesNoDirectory(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")

	rep, err := c.dryRunUpdate(sync.Update{})
	if err != nil {
		t.Fatalf("dry run of a first update: %v", err)
	}

	assertItems(t, rep, "added agent:reviewer 1", "added schema:tdd 2")
	assertSummary(t, rep, "3 files to write, 0 to remove - nothing written")

	c.assertEntries("graft.toml")
}
