package sync_test

import (
	"os"
	"strings"
	"testing"
)

func TestRunFirstSync(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	sha := r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")

	if _, err := c.run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	c.assertEntries(installed()...)
	if got := c.read("openspec/schemas/tdd/schema.yaml"); got != "name: tdd\n" {
		t.Errorf("schema.yaml = %q", got)
	}
	if got := c.read(".claude/agents/reviewer.md"); got != "# reviewer\n" {
		t.Errorf("reviewer.md = %q", got)
	}

	lockText := c.read("graft.lock")
	for _, want := range []string{
		`name     = "shared"`,
		`rev      = "v1.0.0"`,
		`resolved = "` + sha + `"`,
		`id    = "agent:reviewer"`,
		`id    = "schema:tdd"`,
	} {
		if !strings.Contains(lockText, want) {
			t.Errorf("graft.lock does not contain %s:\n%s", want, lockText)
		}
	}
}

func TestRunIsIdempotent(t *testing.T) {
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
	first := c.read("graft.lock")

	if _, err := c.run(); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	c.assertEntries(installed()...)
	if second := c.read("graft.lock"); second != first {
		t.Errorf("the lock changed between two identical syncs:\n%s\n%s", first, second)
	}
	if got := c.read("openspec/schemas/tdd/schema.yaml"); got != "name: tdd\n" {
		t.Errorf("schema.yaml = %q", got)
	}
}

func TestRunMissingManifest(t *testing.T) {
	t.Parallel()

	c := newConsumer(t)

	_, err := c.run()
	assertError(t, err, "graft.toml not found")

	c.assertEntries()
	if entries, _ := os.ReadDir(c.cache); len(entries) != 0 {
		t.Errorf("the cache is not empty: %v", entries)
	}
}

// A source the lock has never seen is resolved once and recorded; a source the lock already
// pins is not re-resolved, and its recorded sha does not move.
func TestRunResolvesNewSource(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	sha := r.commit("v1")
	r.tag("v1.0.0")

	extra := newRepo(t)
	extra.catalog("  - { kind: agent, name: extra, from: extras/extra.md }\n")
	extra.write("extras/extra.md", "# extra\n")
	extraSHA := extra.commit("v2")
	extra.tag("v2.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	c.file("graft.toml", c.read("graft.toml")+"\n"+sourceBlock("extra", extra, "v2.0.0", "agent:extra"))
	if _, err := c.run(); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	if got := c.read(".claude/agents/extra.md"); got != "# extra\n" {
		t.Errorf("extra.md = %q", got)
	}
	lockText := c.read("graft.lock")
	for _, want := range []string{`resolved = "` + sha + `"`, `resolved = "` + extraSHA + `"`} {
		if !strings.Contains(lockText, want) {
			t.Errorf("graft.lock does not contain %s:\n%s", want, lockText)
		}
	}
	// Sources by name: "extra" before "shared", whatever order the manifest was written in.
	if strings.Index(lockText, `name     = "extra"`) > strings.Index(lockText, `name     = "shared"`) {
		t.Errorf("sources are not ordered by name:\n%s", lockText)
	}
}

// sync installs what the lock says, whatever the rev names today. A branch that has moved
// is the case that makes the rule visible: re-resolving would let rev = "main" drift under
// the user between syncs, which defeats the point of pinning.
func TestRunNeverReResolves(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	pinned := r.commit("v1")

	c := newConsumer(t)
	c.manifest(r, "main")
	if _, err := c.run(); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// The branch advances with a new file the catalog also offers.
	r.catalog(`  - { kind: schema, name: tdd,      from: extras/schemas/tdd }
  - { kind: agent,  name: reviewer, from: extras/agents/reviewer.md }
  - { kind: agent,  name: latecomer, from: extras/agents/latecomer.md }
`)
	r.write("extras/agents/latecomer.md", "# latecomer\n")
	moved := r.commit("v2")
	if moved == pinned {
		t.Fatal("the fixture did not advance the branch")
	}

	if _, err := c.run(); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	c.assertEntries(installed()...)
	if c.exists(".claude/agents/latecomer.md") {
		t.Error("the sync installed a file from a commit the lock does not pin")
	}
	if got := c.read("graft.lock"); !strings.Contains(got, `resolved = "`+pinned+`"`) {
		t.Errorf("the pin moved:\n%s", got)
	}
}

func TestRunPinDisagreement(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")
	r.tag("v1.3.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	before := c.entries()
	lockBefore := c.read("graft.lock")

	c.manifest(r, "v1.3.0")

	_, err := c.run()
	assertError(t, err,
		"graft.toml has rev \"v1.3.0\" for source \"shared\" but graft.lock has \"v1.0.0\";"+
			" run `graft update` to move the pin")

	c.assertEntries(before...)
	if got := c.read("graft.lock"); got != lockBefore {
		t.Errorf("graft.lock changed:\n%s", got)
	}
}

// The pin check precedes every fetch, so a manifest that moved cannot cause network access,
// let alone a write. An unreachable remote with an empty cache is what proves it: a fetch
// attempted at all would fail with a different message.
func TestRunPinCheckPrecedesFetch(t *testing.T) {
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

	// A fresh cache and a remote that is gone: anything that fetches will say so.
	c.cache = t.TempDir()
	r.remove(".")
	c.manifest(r, "v9.9.9")

	_, err := c.run()
	assertErrorContains(t, err, "run `graft update` to move the pin")

	if entries, _ := os.ReadDir(c.cache); len(entries) != 0 {
		t.Errorf("the cache is not empty: %v", entries)
	}
}

// A source in the lock that graft.toml no longer declares is pruned from the lock alone:
// its rev is not declared anywhere, so there is nothing to resolve and nothing to fetch.
func TestRunPrunesDroppedSource(t *testing.T) {
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
	c.file(".claude/agents/local-reviewer.md", "repo-owned\n")

	// The manifest now declares a different source, and the old one's remote is gone —
	// so a run that tried to fetch it would fail rather than prune it.
	other := newRepo(t)
	other.catalog("  - { kind: agent, name: other, from: extras/other.md }\n")
	other.write("extras/other.md", "# other\n")
	other.commit("v1")
	other.tag("v1.0.0")
	r.remove(".")
	c.file("graft.toml", sourceBlock("other", other, "v1.0.0", "agent:other"))

	if _, err := c.run(); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	c.assertEntries(
		".claude/",
		".claude/agents/",
		".claude/agents/local-reviewer.md",
		".claude/agents/other.md",
		"graft.lock",
		"graft.toml",
	)
	if got := c.read(".claude/agents/local-reviewer.md"); got != "repo-owned\n" {
		t.Errorf("the repo-owned agent was touched: %q", got)
	}
	if got := c.read("graft.lock"); strings.Contains(got, `name     = "shared"`) {
		t.Errorf("the dropped source is still in the lock:\n%s", got)
	}
	// openspec/ held only synced files and is gone with them.
	if c.exists("openspec") {
		t.Error("openspec/ was left behind empty")
	}
}
