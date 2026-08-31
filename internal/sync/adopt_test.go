package sync

import (
	"strings"
	"testing"

	"github.com/optioni/graft/internal/plan"
)

// A report that says `added` about a file it overwrote is worse than silence: a reader who
// checks the report and moves on has been told the wrong thing. These pin the words.

// reportOver is one source's report with the items given, plus the plan whose writes place
// them, so adopt has something to map destinations back through.
func reportOver(items []ItemReport, writes []plan.Write) (*Report, *plan.Plan) {
	r := &Report{
		Sources: []SourceReport{{Name: "shared", Rev: "v1.0.0", Resolved: "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5", Items: items}},
		Written: len(writes),
	}
	return r, &plan.Plan{Writes: writes}
}

func TestAdoptCorrectsTheVerbThatWouldBeFalse(t *testing.T) {
	t.Parallel()

	r, p := reportOver(
		[]ItemReport{{Verb: verbAdded, ID: "agent:reviewer", Files: 1}},
		[]plan.Write{{Source: "shared", Item: "agent:reviewer", Dest: ".claude/agents/reviewer.md"}},
	)
	r.adopt(p, []string{".claude/agents/reviewer.md"})

	got := r.Sources[0].Items[0]
	if got.Verb != verbAdopted {
		t.Errorf("verb = %q, want %q", got.Verb, verbAdopted)
	}
	if got.Note != noteReplaced {
		t.Errorf("note = %q, want %q", got.Note, noteReplaced)
	}
	if line := r.Lines(plainUI())[2]; line != "  adopted  agent:reviewer  1 file  replaced existing content" {
		t.Errorf("line = %q", line)
	}
}

// An item already in the lock that gains a file at an occupied path has genuinely been
// updated. The verb is corrected only where it would say the opposite of what happened.
func TestAdoptLeavesAnUpdatedVerbAlone(t *testing.T) {
	t.Parallel()

	r, p := reportOver(
		[]ItemReport{{Verb: verbUpdated, ID: "schema:tdd", Files: 6}},
		[]plan.Write{{Source: "shared", Item: "schema:tdd", Dest: "openspec/schemas/tdd/schema.yaml"}},
	)
	r.adopt(p, []string{"openspec/schemas/tdd/schema.yaml"})

	got := r.Sources[0].Items[0]
	if got.Verb != verbUpdated {
		t.Errorf("verb = %q, want %q", got.Verb, verbUpdated)
	}
	if got.Note != noteReplaced {
		t.Errorf("note = %q, want %q", got.Note, noteReplaced)
	}
}

func TestAdoptTouchesNothingWhenNothingWasReplaced(t *testing.T) {
	t.Parallel()

	r, p := reportOver(
		[]ItemReport{{Verb: verbAdded, ID: "agent:reviewer", Files: 1}},
		[]plan.Write{{Source: "shared", Item: "agent:reviewer", Dest: ".claude/agents/reviewer.md"}},
	)
	before := r.Lines(plainUI())
	r.adopt(p, nil)

	if r.Replaced != 0 {
		t.Errorf("Replaced = %d, want 0", r.Replaced)
	}
	after := r.Lines(plainUI())
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("a report with nothing replaced changed:\n%q\n%q", before, after)
	}
	if got := r.Sources[0].Items[0]; got.Verb != verbAdded || got.Note != "" {
		t.Errorf("item = %+v, want an added item with no note", got)
	}
}

// One item replacing several files is several files in the count: the summary counts files,
// which is what a reader is about to go and look at in `git diff`.
func TestTheSummaryCountsReplacedFilesNotItems(t *testing.T) {
	t.Parallel()

	r, p := reportOver(
		[]ItemReport{{Verb: verbAdded, ID: "schema:tdd", Files: 4}},
		[]plan.Write{
			{Source: "shared", Item: "schema:tdd", Dest: "openspec/schemas/tdd/a.md"},
			{Source: "shared", Item: "schema:tdd", Dest: "openspec/schemas/tdd/b.md"},
		},
	)
	r.Written = 4
	r.adopt(p, []string{"openspec/schemas/tdd/a.md", "openspec/schemas/tdd/b.md"})

	want := "4 files written (2 replaced existing content), 0 removed - review with `git diff`"
	if got := r.summary(); got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

func TestTheSummaryCarriesNoParentheticalWhenNothingWasReplaced(t *testing.T) {
	t.Parallel()

	r := &Report{Written: 6, Removed: 1}
	want := "6 files written, 1 removed - review with `git diff`"
	if got := r.summary(); got != want {
		t.Errorf("summary = %q, want %q — byte-identical to what graft printed before adoption was reported", got, want)
	}
}

// A dry run reaches no write, so it observes no replacement — and says so by saying nothing,
// which is the same line it printed before.
func TestADryRunSummaryIsUnchanged(t *testing.T) {
	t.Parallel()

	r := &Report{Written: 6, Removed: 1, DryRun: true}
	if want := "6 files to write, 1 to remove - nothing written"; r.summary() != want {
		t.Errorf("summary = %q, want %q", r.summary(), want)
	}
}
