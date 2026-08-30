package cli_test

import (
	"strings"
	"testing"
)

// The outer loop for semver-ranges: a range crosses six packages — internal/rev
// classifies it, internal/source resolves it, internal/plan carries it, internal/lock
// records it, internal/sync reports it, and internal/list publishes it. The two promises
// no single-package test can hold — that a lock with no range is byte-identical, and that
// `sync` never re-evaluates a range — are both end to end, which is why this test exists
// at this tier. design.md's Test Strategy records the reasoning.

// TestGraftUpdateResolvesARangeAndRecordsTheMatchedTag is the headline scenario: a
// consumer pinning rev = "^1.2.0" against a source publishing v1.2.0 and, later, v1.3.0.
func TestGraftUpdateResolvesARangeAndRecordsTheMatchedTag(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	shaOld := repo.commit("v1")
	repo.tag("v1.2.0")

	c := newConsumer(t, manifestFor(repo, "^1.2.0", "schema:tdd", "agent:*"))

	first := runGraftIn(t, bin, c.dir, c.env, "update")
	if first.code != 0 {
		t.Fatalf("first update: exit %d\nstderr:\n%s", first.code, first.stderr)
	}
	if got := c.read("graft.lock"); !strings.Contains(got, `matched  = "v1.2.0"`) {
		t.Fatalf("graft.lock after the first update does not record matched = v1.2.0:\n%s", got)
	}

	repo.write("extras/schemas/tdd/schema.yaml", "name: tdd v2\n")
	shaNew := repo.commit("v2")
	repo.tag("v1.3.0")

	second := runGraftIn(t, bin, c.dir, c.env, "update")
	if second.code != 0 {
		t.Fatalf("second update: exit %d\nstderr:\n%s", second.code, second.stderr)
	}

	lockText := c.read("graft.lock")
	for _, want := range []string{
		`rev      = "^1.2.0"`,
		`matched  = "v1.3.0"`,
		`resolved = "` + shaNew + `"`,
	} {
		if !strings.Contains(lockText, want) {
			t.Errorf("graft.lock does not contain %s:\n%s", want, lockText)
		}
	}

	wantHeader := "shared  ^1.2.0  v1.2.0 -> v1.3.0  (" + shaOld[:7] + " -> " + shaNew[:7] + ")"
	if !strings.Contains(second.stderr, wantHeader) {
		t.Errorf("the report does not contain %q:\n%s", wantHeader, second.stderr)
	}

	doc := runGraftIn(t, bin, c.dir, c.env, "list", "--json")
	if doc.code != 0 {
		t.Fatalf("list --json: exit %d\nstderr:\n%s", doc.code, doc.stderr)
	}
	for _, want := range []string{`"version": 2`, `"matched": "v1.3.0"`} {
		if !strings.Contains(doc.stdout, want) {
			t.Errorf("list --json does not contain %s:\n%s", want, doc.stdout)
		}
	}
}

// TestGraftSyncDoesNotReEvaluateARange is the other half: once a range's matched tag is
// recorded, `graft sync` installs it exactly, never listing tags again — proven by
// deleting the source repository before the sync runs.
func TestGraftSyncDoesNotReEvaluateARange(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.2.0")

	c := newConsumer(t, manifestFor(repo, "^1.2.0", "schema:tdd", "agent:*"))
	if first := runGraftIn(t, bin, c.dir, c.env, "update"); first.code != 0 {
		t.Fatalf("seeding update: exit %d\nstderr:\n%s", first.code, first.stderr)
	}
	before := c.read("graft.lock")
	if !strings.Contains(before, `matched  = "v1.2.0"`) {
		t.Fatalf("seeding update did not record matched = v1.2.0:\n%s", before)
	}

	repo.write("extras/schemas/tdd/schema.yaml", "name: tdd v2\n")
	repo.commit("v2")
	repo.tag("v1.3.0")

	// Deleted so any tag listing on the sync path would fail rather than silently
	// succeed — the only proof that no `git ls-remote --tags` was run.
	repo.removeDir()

	got := runGraftIn(t, bin, c.dir, c.env, "sync")
	if got.code != 0 {
		t.Fatalf("sync: exit %d\nstderr:\n%s", got.code, got.stderr)
	}
	after := c.read("graft.lock")
	if after != before {
		t.Errorf("graft.lock changed after a sync over a range:\n before:\n%s\n after:\n%s", before, after)
	}
}
