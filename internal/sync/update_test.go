package sync_test

import (
	"strings"
	"testing"
)

// `sync` takes a source's sha from the lock and never asks the remote again. `update` is
// that one decision with a different answer, and everything downstream of it — the fetch,
// the catalog, the plan, the applier, the prune rule, the lock written last, the report —
// is the same code. So the tests here are about which sha a run resolves to, and about the
// two things only an update can do: move a pin, and move one in graft.toml.

// The rule this whole change exists to hold, tested from both sides on one fixture: the
// branch has advanced, and `update` follows it where `sync` does not.
func TestUpdateMovesABranchPin(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	pinned := r.commit("v1")

	c := newConsumer(t)
	c.manifest(r, "main")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	r.write("extras/schemas/tdd/templates/spec.md", "# spec\n")
	moved := r.commit("v2")
	if moved == pinned {
		t.Fatal("the fixture did not advance the branch")
	}

	// sync first: the pin does not move, which is the behavior update is contrasted with.
	if _, err := c.run(); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if got := c.read("graft.lock"); !strings.Contains(got, `resolved = "`+pinned+`"`) {
		t.Fatalf("sync moved the pin:\n%s", got)
	}
	if c.exists("openspec/schemas/tdd/templates/spec.md") {
		t.Fatal("sync installed a file from a commit the lock does not pin")
	}

	if _, err := c.update(); err != nil {
		t.Fatalf("update: %v", err)
	}

	lockText := c.read("graft.lock")
	if !strings.Contains(lockText, `resolved = "`+moved+`"`) {
		t.Errorf("the pin did not move to %s:\n%s", moved, lockText)
	}
	if got := c.read("openspec/schemas/tdd/templates/spec.md"); got != "# spec\n" {
		t.Errorf("the new commit's file = %q, want %q", got, "# spec\n")
	}
	// The request did not change, only what it resolved to.
	if !strings.Contains(lockText, `rev      = "main"`) {
		t.Errorf("the rev changed:\n%s", lockText)
	}
}

func TestUpdateWithNothingNewReportsUpToDate(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	before := c.read("graft.lock")

	report, err := c.update()
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !report.UpToDate() {
		t.Error("the report is not up to date, but nothing moved")
	}
	if got := c.read("graft.lock"); got != before {
		t.Errorf("graft.lock changed:\n%s", got)
	}
	c.assertEntries(installed()...)
}

// An update in a repository that has never synced is not a special case: there is no pin to
// move, so every source is resolved and the run is a first sync with the same result.
func TestUpdateWithNoLockInstallsEverything(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	sha := r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")

	report, err := c.update()
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if report.UpToDate() {
		t.Error("a first update is never nothing to do")
	}

	c.assertEntries(installed()...)
	if got := c.read("graft.lock"); !strings.Contains(got, `resolved = "`+sha+`"`) {
		t.Errorf("graft.lock does not record the resolved sha:\n%s", got)
	}
}

// A source graft.toml no longer declares has its rev declared nowhere, so there is nothing
// to re-resolve. Its git directory is removed first: any fetch or resolution would fail.
func TestUpdatePrunesADroppedSourceWithoutResolvingIt(t *testing.T) {
	t.Parallel()

	shared := newRepo(t)
	shared.seed()
	shared.commit("v1")
	shared.tag("v1.0.0")

	retired := newRepo(t)
	retired.catalog("  - { kind: agent,  name: retiree, from: extras/agents/retiree.md }\n")
	retired.write("extras/agents/retiree.md", "# retiree\n")
	retired.commit("v1")
	retired.tag("v1.0.0")

	c := newConsumer(t)
	c.file("graft.toml",
		sourceBlock("shared", shared, "v1.0.0")+
			sourceBlock("retired", retired, "v1.0.0", "agent:retiree"))
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if !c.exists(".claude/agents/retiree.md") {
		t.Fatal("the fixture did not install the retiring source")
	}

	c.manifest(shared, "v1.0.0")
	retired.remove(".git")

	if _, err := c.update(); err != nil {
		t.Fatalf("update: %v", err)
	}

	if c.exists(".claude/agents/retiree.md") {
		t.Error("the dropped source's file was not pruned")
	}
	if got := c.read("graft.lock"); strings.Contains(got, `name     = "retired"`) {
		t.Errorf("graft.lock still records the dropped source:\n%s", got)
	}
}

func TestUpdateRevNotFoundLeavesTheTreeAlone(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	before, lockBefore := c.entries(), c.read("graft.lock")

	c.manifest(r, "v9.9.9")
	_, err := c.update()
	assertError(t, err, `source "shared": rev "v9.9.9" not found`)

	c.assertEntries(before...)
	if got := c.read("graft.lock"); got != lockBefore {
		t.Errorf("graft.lock changed:\n%s", got)
	}
}

// The prune-set concentration point. A moved pin is a new way to reach the prune set: the
// newer rev stops providing an item, so its files go. A repo-owned file sitting in the same
// destination is not in graft.lock and must survive untouched.
func TestUpdateRemovesADroppedItemAndKeepsAForeignFile(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.catalog(`  - { kind: agent,  name: reviewer, from: extras/agents/reviewer.md }
  - { kind: agent,  name: planner,  from: extras/agents/planner.md }
`)
	r.write("extras/agents/reviewer.md", "# reviewer\n")
	r.write("extras/agents/planner.md", "# planner\n")
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0", "agent:*")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	c.file(".claude/agents/local-reviewer.md", "repo-owned\n")

	// The newer rev stops offering agent:planner.
	r.catalog("  - { kind: agent,  name: reviewer, from: extras/agents/reviewer.md }\n")
	r.remove("extras/agents/planner.md")
	r.commit("v2")
	r.tag("v1.1.0")
	c.manifest(r, "v1.1.0", "agent:*")

	report, err := c.updateTo("shared", "v1.1.0")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if c.exists(".claude/agents/planner.md") {
		t.Error("the item the new rev no longer provides was not removed")
	}
	if got := c.read(".claude/agents/local-reviewer.md"); got != "repo-owned\n" {
		t.Errorf("the repo-owned file = %q, want it untouched: graft.lock never claimed it", got)
	}
	if !c.exists(".claude/agents") {
		t.Error(".claude/agents/ was removed, but it still holds a file")
	}

	var note string
	for _, s := range report.Sources {
		for _, it := range s.Items {
			if it.ID == "agent:planner" {
				note = it.Verb + " " + it.Note
			}
		}
	}
	if note != "removed no longer provided" {
		t.Errorf("agent:planner reported as %q, want %q", note, "removed no longer provided")
	}
}

// A graft.toml that parses and declares nothing is a legitimate state, and an update over it
// resolves nothing and fetches nothing.
func TestUpdateWithNoSourcesDoesNothing(t *testing.T) {
	t.Parallel()

	c := newConsumer(t)
	c.file("graft.toml", "# nothing yet\n")

	report, err := c.update()
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !report.UpToDate() {
		t.Error("an update over a manifest with no sources has nothing to do")
	}
	c.assertEntries("graft.lock", "graft.toml")
	c.assertCacheEmpty()
}

func TestUpdateSourceMovesOnlyThatPin(t *testing.T) {
	t.Parallel()

	shared, extra := newRepo(t), newRepo(t)
	shared.seed()
	sharedFirst := shared.commit("v1")
	extra.catalog("  - { kind: agent,  name: helper, from: extras/agents/helper.md }\n")
	extra.write("extras/agents/helper.md", "# helper\n")
	extraFirst := extra.commit("v1")

	c := newConsumer(t)
	c.file("graft.toml",
		sourceBlock("shared", shared, "main")+sourceBlock("extra", extra, "main", "agent:helper"))
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	shared.write("extras/agents/late.md", "# late\n")
	sharedMoved := shared.commit("v2")
	extra.write("extras/agents/helper.md", "# helper v2\n")
	extraMoved := extra.commit("v2")
	if sharedMoved == sharedFirst || extraMoved == extraFirst {
		t.Fatal("the fixture did not advance both branches")
	}

	if _, err := c.updateSource("shared"); err != nil {
		t.Fatalf("update shared: %v", err)
	}

	lockText := c.read("graft.lock")
	if !strings.Contains(lockText, `resolved = "`+sharedMoved+`"`) {
		t.Errorf("shared's pin did not move:\n%s", lockText)
	}
	if !strings.Contains(lockText, `resolved = "`+extraFirst+`"`) {
		t.Errorf("extra's pin moved, and it was not named:\n%s", lockText)
	}
	if got := c.read(".claude/agents/helper.md"); got != "# helper\n" {
		t.Errorf("extra's file = %q, want the recorded sha's bytes", got)
	}
}

func TestUpdateRefusesASourceTheManifestDoesNotDeclare(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")

	_, err := c.updateSource("sharde")
	assertError(t, err, `graft.toml has no source "sharde"`)

	c.assertEntries("graft.toml")
	c.assertCacheEmpty()
}

// A source the lock records but the manifest no longer declares has its rev declared
// nowhere, so it is not a source an update can move. The refusal prunes nothing.
func TestUpdateRefusesASourceOnlyTheLockKnows(t *testing.T) {
	t.Parallel()

	shared, retired := newRepo(t), newRepo(t)
	shared.seed()
	shared.commit("v1")
	shared.tag("v1.0.0")
	retired.catalog("  - { kind: agent,  name: retiree, from: extras/agents/retiree.md }\n")
	retired.write("extras/agents/retiree.md", "# retiree\n")
	retired.commit("v1")
	retired.tag("v1.0.0")

	c := newConsumer(t)
	c.file("graft.toml",
		sourceBlock("shared", shared, "v1.0.0")+
			sourceBlock("retired", retired, "v1.0.0", "agent:retiree"))
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	c.manifest(shared, "v1.0.0")

	_, err := c.updateSource("retired")
	assertError(t, err, `graft.toml has no source "retired"`)

	if got := c.read(".claude/agents/retiree.md"); got != "# retiree\n" {
		t.Errorf("the retired source's file = %q, want it still there: a refused run prunes nothing", got)
	}
}

// The pin check is what `sync` fails on when a manifest moved by hand. `update` is the
// remedy it points at, so it skips the check for every source it re-resolves.
func TestUpdateRepairsAHandMovedRev(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")
	r.write("extras/agents/late.md", "# late\n")
	moved := r.commit("v2")
	r.tag("v1.1.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	c.manifest(r, "v1.1.0")

	// This is the run sync refuses.
	_, err := c.run()
	assertError(t, err,
		"graft.toml has rev \"v1.1.0\" for source \"shared\" but graft.lock has \"v1.0.0\";"+
			" run `graft update` to move the pin")

	if _, err := c.update(); err != nil {
		t.Fatalf("update: %v", err)
	}
	lockText := c.read("graft.lock")
	if !strings.Contains(lockText, `rev      = "v1.1.0"`) || !strings.Contains(lockText, `resolved = "`+moved+`"`) {
		t.Errorf("the lock did not follow the manifest:\n%s", lockText)
	}
}

// Narrowing the pin check to the sources being re-resolved does not switch it off: a source
// whose sha still comes from the lock is still checked, and still stops the run before the
// network.
func TestUpdateOneSourceStillRefusesAnotherDisagreement(t *testing.T) {
	t.Parallel()

	shared, extra := newRepo(t), newRepo(t)
	shared.seed()
	shared.commit("v1")
	shared.tag("v1.0.0")
	extra.catalog("  - { kind: agent,  name: helper, from: extras/agents/helper.md }\n")
	extra.write("extras/agents/helper.md", "# helper\n")
	extra.commit("v1")
	extra.tag("v1.0.0")
	extra.tag("v2.0.0")

	c := newConsumer(t)
	c.file("graft.toml",
		sourceBlock("shared", shared, "v1.0.0")+sourceBlock("extra", extra, "v1.0.0", "agent:helper"))
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	lockBefore := c.read("graft.lock")

	c.file("graft.toml",
		sourceBlock("shared", shared, "v1.0.0")+sourceBlock("extra", extra, "v2.0.0", "agent:helper"))

	_, err := c.updateSource("shared")
	assertError(t, err,
		"graft.toml has rev \"v2.0.0\" for source \"extra\" but graft.lock has \"v1.0.0\";"+
			" run `graft update` to move the pin")

	if got := c.read("graft.lock"); got != lockBefore {
		t.Errorf("graft.lock changed:\n%s", got)
	}
}
