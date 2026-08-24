package sync_test

import (
	"os"
	"path/filepath"
	"testing"
)

// The nothing-to-do predicate, exercised through whole syncs rather than hand-built locks:
// the unit tests pin the rule, and these pin that a real run reaches it.

func TestReportUpToDate(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")

	first, err := c.run()
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if first.UpToDate() {
		t.Error("the first sync reported nothing to do")
	}

	second, err := c.run()
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if !second.UpToDate() {
		t.Errorf("the second sync reported %+v, want nothing to do", second.Sources)
	}
}

// The case where the predicate and the tree disagree, kept deliberately: the lock did not
// move and nothing was pruned, so the run is nothing to do even though six files came back.
// git status is where a sync's effect shows up, and it shows them.
func TestReportUpToDateAfterHandDeletion(t *testing.T) {
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

	for _, path := range []string{"openspec", ".claude"} {
		if err := os.RemoveAll(filepath.Join(c.dir, path)); err != nil {
			t.Fatalf("RemoveAll %s: %v", path, err)
		}
	}

	rep, err := c.run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.UpToDate() {
		t.Errorf("report = %+v, want nothing to do", rep.Sources)
	}
	c.assertEntries(installed()...)
	if got := c.read("openspec/schemas/tdd/schema.yaml"); got != "name: tdd\n" {
		t.Errorf("the files did not come back: %q", got)
	}
}

func TestReportDryRunUpToDate(t *testing.T) {
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

	rep, err := c.dryRun()
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	// --dry-run changes what the summary says, not what nothing to do means.
	if !rep.UpToDate() {
		t.Errorf("report = %+v, want nothing to do", rep.Sources)
	}
}
