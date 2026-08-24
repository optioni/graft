package sync_test

import (
	"testing"
)

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

	if _, err := c.dryRun(); err != nil {
		t.Fatalf("dry run: %v", err)
	}

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

	if _, err := c.dryRun(); err != nil {
		t.Fatalf("dry run: %v", err)
	}

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
