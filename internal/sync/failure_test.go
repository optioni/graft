package sync_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Every failure below happens before the plan is applied, so each asserts the same three
// things: the exact message, that every pre-existing file is byte-identical, and that
// graft.lock was neither created nor modified. SPEC.md's failure-mode table is the product
// for a CLI this size, and a message that drifts is a contract change.

// untouched captures a consumer's tree and lock, and fails the test if either moved.
func untouched(t *testing.T, c *consumer) func() {
	t.Helper()
	before := c.entries()
	var lockBefore string
	if c.exists("graft.lock") {
		lockBefore = c.read("graft.lock")
	}
	return func() {
		t.Helper()
		if got := c.entries(); !slices.Equal(got, before) {
			t.Errorf("the working tree changed:\n got %v\nwant %v", got, before)
		}
		if lockBefore == "" {
			if c.exists("graft.lock") {
				t.Error("graft.lock was created by a run that failed")
			}
			return
		}
		if got := c.read("graft.lock"); got != lockBefore {
			t.Errorf("graft.lock changed:\n%s", got)
		}
	}
}

func TestRunRevNotFound(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")

	c := newConsumer(t)
	c.manifest(r, "v9.9.9")
	check := untouched(t, c)

	_, err := c.run()
	assertError(t, err, `source "shared": rev "v9.9.9" not found`)
	check()
}

func TestRunNoCatalog(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.write("extras/agents/reviewer.md", "# reviewer\n")
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")
	check := untouched(t, c)

	_, err := c.run()
	assertError(t, err, "catalog.yaml not found: the source is not graftable")
	check()
}

func TestRunInvalidCatalog(t *testing.T) {
	t.Parallel()

	// Asserted in full, not merely as "mentions catalog.yaml": that weaker test passes on
	// `catalog.yaml not found`, which is the *missing*-catalog row of the failure-mode
	// table and a different condition entirely.
	for name, tc := range map[string]struct{ body, want string }{
		"a kinds list": {
			"version: 1\nkinds:\n  - this is not a mapping\n",
			"catalog.yaml: kinds must be a mapping",
		},
		"an undeclared kind": {
			"version: 1\nkinds:\n  agent:\n    to: \".claude/agents/\"\nprovides:\n" +
				"  - { kind: schema, name: tdd, from: extras/tdd }\n",
			`catalog.yaml: item "schema:tdd": kind "schema" is not declared`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := newRepo(t)
			r.write("catalog.yaml", tc.body)
			r.write("extras/tdd/schema.yaml", "x\n")
			r.commit("v1")
			r.tag("v1.0.0")

			c := newConsumer(t)
			c.manifest(r, "v1.0.0", "schema:tdd")
			check := untouched(t, c)

			_, err := c.run()
			assertError(t, err, tc.want)
			check()
		})
	}
}

func TestRunSelectorMatchesNothing(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0", "schema:tdd-workflwo")
	check := untouched(t, c)

	_, err := c.run()
	assertError(t, err,
		`source "shared": selector "schema:tdd-workflwo" matches no item;`+
			` catalog provides agent:reviewer, schema:tdd`)
	check()
}

// Two sources placing an item at one path. The plan fails before any write, so the path
// does not exist afterwards — which is the difference between a collision and
// last-writer-wins: the loser would be a file the lock claims and a later sync deletes.
func TestRunCollisionLeavesTreeUntouched(t *testing.T) {
	t.Parallel()

	alpha := newRepo(t)
	alpha.catalog("  - { kind: agent, name: x, from: extras/x.md }\n")
	alpha.write("extras/x.md", "alpha\n")
	alpha.commit("v1")
	alpha.tag("v1.0.0")

	beta := newRepo(t)
	beta.catalog("  - { kind: agent, name: x, from: extras/x.md }\n")
	beta.write("extras/x.md", "beta\n")
	beta.commit("v1")
	beta.tag("v1.0.0")

	c := newConsumer(t)
	c.file("graft.toml",
		sourceBlock("alpha", alpha, "v1.0.0", "agent:x")+"\n"+
			sourceBlock("beta", beta, "v1.0.0", "agent:x"))
	check := untouched(t, c)

	_, err := c.run()
	assertError(t, err,
		`source "alpha" item "agent:x" and source "beta" item "agent:x"`+
			` both resolve to ".claude/agents/x.md"`)
	check()
	if c.exists(".claude/agents/x.md") {
		t.Error("a colliding file was written")
	}
}

// The repo-root rule reaching a user through a real plan.Build, rather than through a
// hand-built plan handed straight to internal/apply.
func TestRunDestinationEscapesRepoRoot(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.write("catalog.yaml", "version: 1\nkinds:\n  schema:\n    to: \"../outside/{name}\"\nprovides:\n"+
		"  - { kind: schema, name: tdd, from: extras/tdd }\n")
	r.write("extras/tdd/schema.yaml", "x\n")
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0", "schema:tdd")
	check := untouched(t, c)

	_, err := c.run()
	assertError(t, err,
		`source "shared": item "schema:tdd": destination "../outside/tdd" escapes the repo root`)
	check()

	if _, statErr := os.Lstat(filepath.Join(filepath.Dir(c.dir), "outside")); statErr == nil {
		t.Error("a directory was created beside the repository root")
	}
}

// A `from` that is a committed symlink out of the tree. It could otherwise read a file
// outside the source entirely — catalog's own rule, that a from is relative and cleaned
// and has no ".." segment, finds nothing wrong with the string.
func TestRunListingClimbsOut(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.catalog("  - { kind: agent, name: link, from: extras/link }\n")
	r.write("extras/keep.md", "x\n")
	if err := os.Symlink("/etc", filepath.Join(r.dir, "extras", "link")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0", "agent:link")
	check := untouched(t, c)

	_, err := c.run()
	assertError(t, err,
		`source "shared": item "agent:link": from "extras/link" is not a regular file or directory`)
	check()
}

// SPEC.md's two network rows. A resolved sync works on a plane, and one that needs a fetch
// it cannot make says what it needed.
func TestRunCacheHitOffline(t *testing.T) {
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

	r.remove(".")

	if _, err := c.run(); err != nil {
		t.Fatalf("Run against a warm cache and a deleted remote: %v", err)
	}
	c.assertEntries(installed()...)
}

func TestRunCacheMissOffline(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	sha := r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// A cold cache and a remote that is gone: the fetch is the only way to the tree.
	c.cache = t.TempDir()
	r.remove(".")
	check := untouched(t, c)

	_, err := c.run()
	assertErrorContains(t, err, `source "shared": cannot fetch "`+sha+`" from "`+r.dir+`"`)
	check()
}
