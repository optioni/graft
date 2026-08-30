package sync_test

import (
	"strings"
	"testing"

	"github.com/optioni/graft/internal/ui"
)

// --to is the only path in graft update that writes graft.toml. The file is a human's, so
// the assertion is always the same one: exactly one line differs, and it is the rev.

// manifestWithComment is what a person actually writes: a comment, the aligned keys SPEC.md's
// own example uses, and a second source that must not move.
func manifestWithComment(shared, extra *repo, rev string) string {
	return "# pinned deliberately\n" +
		sourceBlock("shared", shared, rev) +
		"\n# a second source, which does not move\n" +
		sourceBlock("extra", extra, "v1.0.0", "agent:helper")
}

// changedLines returns the lines of after that differ from before, positionally.
func changedLines(before, after string) []string {
	old, current := strings.Split(before, "\n"), strings.Split(after, "\n")
	var out []string
	for i, line := range current {
		if i >= len(old) || old[i] != line {
			out = append(out, line)
		}
	}
	for i := len(current); i < len(old); i++ {
		out = append(out, "(removed) "+old[i])
	}
	return out
}

func TestUpdateToMovesOneLineOfTheManifest(t *testing.T) {
	t.Parallel()

	shared, extra := newRepo(t), newRepo(t)
	shared.seed()
	shared.commit("v1")
	shared.tag("v1.0.0")
	shared.write("extras/schemas/tdd/templates/spec.md", "# spec\n")
	moved := shared.commit("v2")
	shared.tag("v1.1.0")

	extra.catalog("  - { kind: agent,  name: helper, from: extras/agents/helper.md }\n")
	extra.write("extras/agents/helper.md", "# helper\n")
	extra.commit("v1")
	extra.tag("v1.0.0")

	before := manifestWithComment(shared, extra, "v1.0.0")
	c := newConsumer(t)
	c.file("graft.toml", before)
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	if _, err := c.updateTo("shared", "v1.1.0"); err != nil {
		t.Fatalf("update --to: %v", err)
	}

	after := c.read("graft.toml")
	if diff := changedLines(before, after); len(diff) != 1 || diff[0] != `rev     = "v1.1.0"` {
		t.Errorf("graft.toml changed in %d lines %q, want exactly one reading %q",
			len(diff), diff, `rev     = "v1.1.0"`)
	}

	lockText := c.read("graft.lock")
	if !strings.Contains(lockText, `rev      = "v1.1.0"`) || !strings.Contains(lockText, `resolved = "`+moved+`"`) {
		t.Errorf("the lock did not follow the manifest:\n%s", lockText)
	}
	if got := c.read("openspec/schemas/tdd/templates/spec.md"); got != "# spec\n" {
		t.Errorf("the new rev's file = %q, want %q", got, "# spec\n")
	}
}

// An update without --to moves the lock and nothing else: the request did not change, only
// what it resolved to.
func TestUpdateWithoutToNeverWritesTheManifest(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")

	c := newConsumer(t)
	c.manifest(r, "main")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	before := c.read("graft.toml")

	r.write("extras/agents/late.md", "# late\n")
	moved := r.commit("v2")

	if _, err := c.update(); err != nil {
		t.Fatalf("update: %v", err)
	}

	if got := c.read("graft.toml"); got != before {
		t.Errorf("graft.toml = %q, want %q", got, before)
	}
	lockText := c.read("graft.lock")
	if !strings.Contains(lockText, `rev      = "main"`) {
		t.Errorf("the rev changed:\n%s", lockText)
	}
	if !strings.Contains(lockText, `resolved = "`+moved+`"`) {
		t.Errorf("the resolved sha did not move:\n%s", lockText)
	}
}

func TestUpdateToARevThatDoesNotExistLeavesTheManifest(t *testing.T) {
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
	manifestBefore, lockBefore, entriesBefore := c.read("graft.toml"), c.read("graft.lock"), c.entries()

	_, err := c.updateTo("shared", "v9.9.9")
	assertError(t, err, `source "shared": rev "v9.9.9" not found`)

	if got := c.read("graft.toml"); got != manifestBefore {
		t.Errorf("graft.toml moved on a failed run:\n%s", got)
	}
	if got := c.read("graft.lock"); got != lockBefore {
		t.Errorf("graft.lock changed:\n%s", got)
	}
	c.assertEntries(entriesBefore...)
}

func TestUpdateToRefusesASourceTheManifestDoesNotDeclare(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")
	before := c.read("graft.toml")

	// The membership message, not the manifest editor's: both are true, and only this one
	// helps.
	_, err := c.updateTo("sharde", "v1.1.0")
	assertError(t, err, `graft.toml has no source "sharde"`)

	if got := c.read("graft.toml"); got != before {
		t.Errorf("graft.toml changed:\n%s", got)
	}
	c.assertCacheEmpty()
}

// A manifest shape the editor cannot rewrite exactly stops the run before anything is
// fetched — and an update without --to still works on the same file, because only --to needs
// to edit it.
func TestUpdateToRefusesAManifestShapeItCannotRewrite(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	inline := "[sources]\nshared = { git = " + quoted(r.dir) + ", rev = \"v1.0.0\", install = [\"schema:tdd\", \"agent:*\"] }\n"
	c := newConsumer(t)
	c.file("graft.toml", inline)

	_, err := c.updateTo("shared", "v1.1.0")
	assertError(t, err,
		`graft.toml: source "shared": cannot move the pin: rev is not a plain key under [sources.shared]`)

	if got := c.read("graft.toml"); got != inline {
		t.Errorf("graft.toml changed:\n%s", got)
	}
	c.assertEntries("graft.toml")
	c.assertCacheEmpty()

	// The same manifest updates fine without --to: the refusal is about editing the file,
	// not about reading it.
	if _, err := c.update(); err != nil {
		t.Errorf("update without --to: %v", err)
	}
}

// The report is the one sync-report specifies, rendered by the same code. What an update
// adds is that its two-sided header forms are finally reachable.
func TestUpdateReportShowsWhatMoved(t *testing.T) {
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

	report, err := c.updateTo("shared", "v1.1.0")
	if err != nil {
		t.Fatalf("update --to: %v", err)
	}

	lines := report.Lines(ui.New(nil, nil, false))
	want := "shared  v1.0.0 -> v1.1.0  (" + first[:7] + " -> " + second[:7] + ")"
	if lines[0] != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}
	if last := lines[len(lines)-1]; last != "4 files written, 0 removed - review with `git diff`" {
		t.Errorf("summary = %q", last)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "updated  schema:tdd") {
		t.Errorf("the report does not report the moved item:\n%s", strings.Join(lines, "\n"))
	}
}

// A branch pin renders its rev once and both shas: the request did not move, the sha did.
func TestUpdateReportShowsOneRevAndTwoShasForABranch(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	first := r.commit("v1")

	c := newConsumer(t)
	c.manifest(r, "main")
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	r.write("extras/agents/late.md", "# late\n")
	second := r.commit("v2")

	report, err := c.update()
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	lines := report.Lines(ui.New(nil, nil, false))
	want := "shared  main  (" + first[:7] + " -> " + second[:7] + ")"
	if lines[0] != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}
}

// quoted renders a string as a TOML basic string, matching what the fixture helpers write.
func quoted(s string) string { return `"` + s + `"` }

// TestUpdateToCanWriteARangeIntoTheManifest: `--to` is not limited to a ref — a range is
// just another value the in-place editor writes literally, and the run that follows
// resolves it exactly as `update` resolves any range.
func TestUpdateToCanWriteARangeIntoTheManifest(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")
	r.write("extras/schemas/tdd/templates/spec.md", "# spec\n")
	tip := r.commit("v2")
	r.tag("v1.3.0")

	before := sourceBlock("shared", r, "v1.0.0")
	c := newConsumer(t)
	c.file("graft.toml", before)
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	if _, err := c.updateTo("shared", "^1.2.0"); err != nil {
		t.Fatalf("update --to: %v", err)
	}

	after := c.read("graft.toml")
	if diff := changedLines(before, after); len(diff) != 1 || diff[0] != `rev     = "^1.2.0"` {
		t.Errorf("graft.toml changed in %d lines %q, want exactly one reading %q",
			len(diff), diff, `rev     = "^1.2.0"`)
	}

	lockText := c.read("graft.lock")
	for _, want := range []string{`rev      = "^1.2.0"`, `matched  = "v1.3.0"`, `resolved = "` + tip + `"`} {
		if !strings.Contains(lockText, want) {
			t.Errorf("graft.lock does not contain %s:\n%s", want, lockText)
		}
	}
}

// TestUpdateToCanWriteARangeContainingASpace: the space is inside a TOML string and
// needs no escaping, so the in-place editor writes it literally — exactly as it would
// any other rev containing no quote, backslash, or control character.
func TestUpdateToCanWriteARangeContainingASpace(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	before := sourceBlock("shared", r, "v1.0.0")
	c := newConsumer(t)
	c.file("graft.toml", before)
	if _, err := c.run(); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	if _, err := c.updateTo("shared", ">=1.0.0 <2.0.0"); err != nil {
		t.Fatalf("update --to: %v", err)
	}

	after := c.read("graft.toml")
	if !strings.Contains(after, `rev     = ">=1.0.0 <2.0.0"`) {
		t.Errorf("graft.toml does not hold the range with its space intact:\n%s", after)
	}
}
