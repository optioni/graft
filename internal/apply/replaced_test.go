package apply_test

import (
	"slices"
	"testing"

	"github.com/optioni/graft/internal/apply"
	"github.com/optioni/graft/internal/plan"
)

// Whether a write replaced something is a question only this package may ask, because it is
// the only one that may look at the working tree. All three conditions have to hold, and
// each of the three near-misses below is a case that would otherwise put a number in the
// summary that a reader learns to skip.

// applyOne runs a one-write plan and returns what it reported replacing.
func applyOne(t *testing.T, repo, src *tree, claimed bool) []string {
	t.Helper()

	w := write("schema.yaml", "openspec/schemas/tdd/schema.yaml")
	w.Claimed = claimed
	p := &plan.Plan{Writes: []plan.Write{w}, Lock: lockOf("openspec/schemas/tdd/schema.yaml")}

	replaced, err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return replaced
}

func TestRunReportsAHandWrittenFileItReplaced(t *testing.T) {
	t.Parallel()

	src := newTree(t)
	src.file("schema.yaml", "name: tdd\n")
	repo := newTree(t)
	repo.file("openspec/schemas/tdd/schema.yaml", "name: mine, written by hand\n")

	got := applyOne(t, repo, src, false)

	if want := []string{"openspec/schemas/tdd/schema.yaml"}; !slices.Equal(got, want) {
		t.Errorf("replaced = %q, want %q", got, want)
	}
	// Reported, not refused: a synced file is a derived artifact and adoption is how a
	// repository starts using graft.
	if got := repo.read("openspec/schemas/tdd/schema.yaml"); got != "name: tdd\n" {
		t.Errorf("the file was not written: %q", got)
	}
}

// A destination the lock claims is graft's own file being rewritten, which is what a sync
// does rather than something to report.
func TestRunDoesNotReportAClaimedDestination(t *testing.T) {
	t.Parallel()

	src := newTree(t)
	src.file("schema.yaml", "name: tdd\n")
	repo := newTree(t)
	repo.file("openspec/schemas/tdd/schema.yaml", "name: tdd, an older version\n")

	if got := applyOne(t, repo, src, true); len(got) != 0 {
		t.Errorf("replaced = %q, want none", got)
	}
}

func TestRunDoesNotReportIdenticalBytes(t *testing.T) {
	t.Parallel()

	src := newTree(t)
	src.file("schema.yaml", "name: tdd\n")
	repo := newTree(t)
	repo.file("openspec/schemas/tdd/schema.yaml", "name: tdd\n")

	if got := applyOne(t, repo, src, false); len(got) != 0 {
		t.Errorf("replaced = %q, want none: identical bytes replaced nothing", got)
	}
}

func TestRunDoesNotReportAnAbsentDestination(t *testing.T) {
	t.Parallel()

	src := newTree(t)
	src.file("schema.yaml", "name: tdd\n")
	repo := newTree(t)

	if got := applyOne(t, repo, src, false); len(got) != 0 {
		t.Errorf("replaced = %q, want none: an absent destination is an ordinary write", got)
	}
}

// A partial run's account of what it replaced would describe a state that never existed —
// the same reason a failed run writes no lock.
func TestAFailedRunReportsNothing(t *testing.T) {
	t.Parallel()

	src := newTree(t)
	src.file("a.md", "one\n")
	src.file("b.md", "two\n")
	repo := newTree(t)
	repo.file("docs/a.md", "mine\n")
	repo.mkdir("docs/b.md") // a directory where a file must go: refused in the pre-flight

	p := &plan.Plan{
		Writes: []plan.Write{write("a.md", "docs/a.md"), write("b.md", "docs/b.md")},
		Lock:   lockOf("docs/a.md", "docs/b.md"),
	}
	replaced, err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)

	if err == nil {
		t.Fatal("Run succeeded over a directory where a file must go")
	}
	if len(replaced) != 0 {
		t.Errorf("replaced = %q, want none from a failed run", replaced)
	}
	// The pre-flight refuses before the first write, so the file that would have been
	// replaced is still the consumer's own.
	if got := repo.read("docs/a.md"); got != "mine\n" {
		t.Errorf("docs/a.md = %q, want the run to have written nothing", got)
	}
}
