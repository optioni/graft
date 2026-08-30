package sync_test

import (
	"strings"
	"testing"
)

// The rule this whole change exists to hold, on the sync path: a range in graft.toml
// beside a matched tag in graft.lock is a resolved pin, and sync installs it exactly as
// it installs any other — the first resolution and the never-re-resolving guarantee are
// both exercised here.

func TestSyncRangeNewerTagSatisfyingItDoesNotMoveThePin(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.2.0")

	c := newConsumer(t)
	c.manifest(r, "^1.2.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	before := c.read("graft.lock")
	if !strings.Contains(before, `matched  = "v1.2.0"`) {
		t.Fatalf("the first sync did not record matched = v1.2.0:\n%s", before)
	}

	r.write("extras/schemas/tdd/templates/spec.md", "# spec\n")
	r.commit("v2")
	r.tag("v1.3.0")

	if _, err := c.run(); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if got := c.read("graft.lock"); got != before {
		t.Errorf("graft.lock changed after a sync over a range with a newer satisfying tag:\n before:\n%s\n after:\n%s", before, got)
	}
	if c.exists("openspec/schemas/tdd/templates/spec.md") {
		t.Error("sync installed a file from a tag the lock does not pin — it re-evaluated the range")
	}
}

func TestSyncRangeSourceWithNoLockEntryIsResolvedOnceAndRecorded(t *testing.T) {
	t.Parallel()

	shared, extra := newRepo(t), newRepo(t)
	shared.seed()
	shared.commit("v1")
	shared.tag("v1.0.0")

	extra.catalog("  - { kind: agent,  name: helper, from: extras/agents/helper.md }\n")
	extra.write("extras/agents/helper.md", "# helper v2.0.0\n")
	extra.commit("v1")
	extra.tag("v2.0.0")
	extra.write("extras/agents/helper.md", "# helper v2.1.0\n")
	extraTip := extra.commit("v2")
	extra.tag("v2.1.0")

	c := newConsumer(t)
	c.manifest(shared, "v1.0.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	sharedBefore := c.read("graft.lock")

	c.file("graft.toml",
		sourceBlock("shared", shared, "v1.0.0")+sourceBlock("extra", extra, "^2.0.0", "agent:helper"))

	if _, err := c.run(); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	lockText := c.read("graft.lock")
	for _, want := range []string{
		`rev      = "^2.0.0"`,
		`matched  = "v2.1.0"`,
		`resolved = "` + extraTip + `"`,
	} {
		if !strings.Contains(lockText, want) {
			t.Errorf("graft.lock does not contain %s:\n%s", want, lockText)
		}
	}
	if !strings.Contains(sharedBefore, `name     = "shared"`) || !strings.Contains(lockText, `name     = "shared"`) {
		t.Errorf("shared's entry is missing")
	}
	if got := c.read(".claude/agents/helper.md"); got != "# helper v2.1.0\n" {
		t.Errorf("helper.md = %q, want the v2.1.0 content", got)
	}
}

func TestSyncRangeWithNoLockEntryThatNoTagSatisfiesFailsTheRun(t *testing.T) {
	t.Parallel()

	shared, extra := newRepo(t), newRepo(t)
	shared.seed()
	shared.commit("v1")
	shared.tag("v1.0.0")

	extra.catalog("  - { kind: agent,  name: helper, from: extras/agents/helper.md }\n")
	extra.write("extras/agents/helper.md", "# helper\n")
	extra.commit("v1")
	extra.tag("v1.0.0")

	c := newConsumer(t)
	c.file("graft.toml",
		sourceBlock("shared", shared, "v1.0.0")+sourceBlock("extra", extra, "^9.0.0", "agent:helper"))

	_, err := c.run()
	assertError(t, err, `source "extra": rev "^9.0.0" matches none of the source's semver tags`)

	c.assertEntries("graft.toml")
	c.assertCacheEmpty()
}

func TestUpdateRangeNewTagSatisfyingItMovesThePin(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	first := r.commit("v1")
	r.tag("v1.2.0")

	c := newConsumer(t)
	c.manifest(r, "^1.2.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	r.write("extras/schemas/tdd/templates/spec.md", "# spec\n")
	second := r.commit("v2")
	r.tag("v1.3.0")

	if _, err := c.update(); err != nil {
		t.Fatalf("update: %v", err)
	}

	lockText := c.read("graft.lock")
	for _, want := range []string{`matched  = "v1.3.0"`, `resolved = "` + second + `"`, `rev      = "^1.2.0"`} {
		if !strings.Contains(lockText, want) {
			t.Errorf("graft.lock does not contain %s:\n%s", want, lockText)
		}
	}
	if strings.Contains(lockText, first) {
		t.Errorf("the old sha is still recorded:\n%s", lockText)
	}
	if got := c.read("openspec/schemas/tdd/templates/spec.md"); got != "# spec\n" {
		t.Errorf("the new tag's file = %q, want %q", got, "# spec\n")
	}
}

func TestUpdateRangeNewTagOutsideItDoesNotMoveThePin(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.2.0")

	c := newConsumer(t)
	c.manifest(r, "^1.2.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	before := c.read("graft.lock")

	r.write("extras/agents/late.md", "# late\n")
	r.commit("v2")
	r.tag("v2.0.0")

	report, err := c.update()
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !report.UpToDate() {
		t.Error("the report is not up to date, but v2.0.0 crosses a major the range excludes")
	}
	if got := c.read("graft.lock"); got != before {
		t.Errorf("graft.lock changed:\n%s", got)
	}
}

// A range that stops matching fails `update`, which re-evaluates it, but never `sync`,
// which does not.
func TestUpdateRangeThatStopsMatchingFailsUpdateNotSync(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.2.0")

	c := newConsumer(t)
	c.manifest(r, "^1.2.0")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	before, lockBefore := c.entries(), c.read("graft.lock")

	// The satisfying tag is deleted, but a non-satisfying one survives — otherwise the
	// source would publish no semver tags at all, and the message would name that
	// instead of naming an unsatisfiable range.
	r.git("-C", r.dir, "tag", "-d", "v1.2.0")
	r.tag("v2.0.0")

	_, err := c.update()
	assertError(t, err, `source "shared": rev "^1.2.0" matches none of the source's semver tags`)
	c.assertEntries(before...)
	if got := c.read("graft.lock"); got != lockBefore {
		t.Errorf("graft.lock changed on a failed update:\n%s", got)
	}

	if _, err := c.run(); err != nil {
		t.Errorf("sync on the same repository: %v, want success — sync never re-resolves", err)
	}
}
