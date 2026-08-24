package apply_test

import (
	"fmt"
	"testing"

	"github.com/optioni/graft/internal/apply"
	"github.com/optioni/graft/internal/plan"
)

// The prune set comes from internal/plan, which derives it from graft.lock alone. A file
// absent from graft.lock is therefore invisible here: it is never in the set, and this
// package never looks in a directory to find one. That is what lets .claude/agents/ hold
// repo-owned agents beside synced ones, so it is tested in every shape the directory can
// take rather than asserted once.
func TestRunForeignFileSurvives(t *testing.T) {
	t.Parallel()

	const foreign = ".claude/agents/local-reviewer.md"
	const synced = ".claude/agents/apply-orchestrator.md"

	t.Run("the synced file beside it is pruned", func(t *testing.T) {
		t.Parallel()

		repo := newTree(t)
		repo.file(foreign, "repo-owned\n")
		repo.file(synced, "synced\n")

		p := &plan.Plan{Prune: []string{synced}, Lock: emptyLock()}
		if err := apply.Run(repo.dir, nil, p); err != nil {
			t.Fatalf("Run: %v", err)
		}

		repo.assertEntries(".claude/", ".claude/agents/", foreign, "graft.lock")
		if got := repo.read(foreign); got != "repo-owned\n" {
			t.Errorf("%s = %q, want %q", foreign, got, "repo-owned\n")
		}
	})

	t.Run("the synced file beside it is rewritten", func(t *testing.T) {
		t.Parallel()

		repo := newTree(t)
		repo.file(foreign, "repo-owned\n")
		repo.file(synced, "old\n")
		src := newTree(t)
		src.file("extras/apply-orchestrator.md", "new\n")

		p := &plan.Plan{
			Writes: []plan.Write{write("extras/apply-orchestrator.md", synced)},
			Lock:   lockOf(synced),
		}
		if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err != nil {
			t.Fatalf("Run: %v", err)
		}

		repo.assertEntries(".claude/", ".claude/agents/", foreign, synced, "graft.lock")
		if got := repo.read(foreign); got != "repo-owned\n" {
			t.Errorf("%s = %q, want %q", foreign, got, "repo-owned\n")
		}
	})

	t.Run("a second synced file lands beside it", func(t *testing.T) {
		t.Parallel()

		repo := newTree(t)
		repo.file(foreign, "repo-owned\n")
		src := newTree(t)
		src.file("extras/reviewer.md", "# reviewer\n")

		p := &plan.Plan{
			Writes: []plan.Write{write("extras/reviewer.md", ".claude/agents/reviewer.md")},
			Lock:   lockOf(".claude/agents/reviewer.md"),
		}
		if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err != nil {
			t.Fatalf("Run: %v", err)
		}

		repo.assertEntries(
			".claude/", ".claude/agents/", foreign, ".claude/agents/reviewer.md", "graft.lock",
		)
		if got := repo.read(foreign); got != "repo-owned\n" {
			t.Errorf("%s = %q, want %q", foreign, got, "repo-owned\n")
		}
	})
}

// The prune set handed in is the only source of deletions this package has. Ten unrecorded
// files beside one recorded one is the shape that would catch a directory listing sneaking
// in — a "clean up everything graft installed here" that reached one file too far.
func TestRunNeverEnumeratesADirectory(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	want := []string{".claude/", ".claude/agents/", "graft.lock"}
	for i := range 10 {
		name := fmt.Sprintf(".claude/agents/mine-%d.md", i)
		repo.file(name, "mine\n")
		want = append(want, name)
	}
	repo.file(".claude/agents/synced.md", "synced\n")

	p := &plan.Plan{Prune: []string{".claude/agents/synced.md"}, Lock: emptyLock()}
	if err := apply.Run(repo.dir, nil, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	repo.assertEntries(want...)
}

// A sync stays idempotent after a user deletes a synced file by hand: the lock still
// claims it, so it is still in the prune set, and there is nothing to do about that.
func TestRunPruneMissingPath(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("openspec/schemas/tdd/other.yaml", "kept\n")

	p := &plan.Plan{Prune: []string{"openspec/schemas/tdd/schema.yaml"}, Lock: emptyLock()}
	if err := apply.Run(repo.dir, nil, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	repo.assertEntries(
		"graft.lock",
		"openspec/",
		"openspec/schemas/",
		"openspec/schemas/tdd/",
		"openspec/schemas/tdd/other.yaml",
	)
}

// graft only ever writes regular files, so anything else at a path the lock claims means
// the tree is not what the lock says it is. Recursively deleting it would be the one
// mistake this whole design exists to prevent.
func TestRunPruneDirectoryRefused(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("docs/api/index.md", "mine\n")

	p := &plan.Plan{Prune: []string{"docs/api"}, Lock: emptyLock()}
	err := apply.Run(repo.dir, nil, p)
	assertError(t, err, `cannot remove "docs/api": it is not a regular file`)

	repo.assertEntries("docs/", "docs/api/", "docs/api/index.md")
}

func TestRunPruneSymlinkRefused(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file(".claude/agents/local-reviewer.md", "repo-owned\n")
	repo.symlink("local-reviewer.md", ".claude/agents/x.md")

	p := &plan.Plan{Prune: []string{".claude/agents/x.md"}, Lock: emptyLock()}
	err := apply.Run(repo.dir, nil, p)
	assertError(t, err, `cannot remove ".claude/agents/x.md": it is not a regular file`)

	// Removing the link would succeed and would be a deletion of something graft never
	// wrote. Its target surviving is the weaker half; the link surviving is the point.
	if !repo.exists(".claude/agents/x.md") {
		t.Error("the symlink was removed")
	}
	if got := repo.read(".claude/agents/local-reviewer.md"); got != "repo-owned\n" {
		t.Errorf("local-reviewer.md = %q, want %q", got, "repo-owned\n")
	}
}

// The dangerous half of the ancestry rule. The lock claims vendor/x.md; vendor has since
// become a link to the repository's own docs/; and Lstat declines to follow only the last
// component, so an unguarded removal deletes docs/x.md — a path no lock claims, removed by
// graft, with nothing said.
func TestRunPruneSymlinkedParentRefused(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("docs/x.md", "repo-owned\n")
	repo.symlink("docs", "vendor")

	p := &plan.Plan{Prune: []string{"vendor/x.md"}, Lock: emptyLock()}
	err := apply.Run(repo.dir, nil, p)
	assertError(t, err, `cannot remove "vendor/x.md": "vendor" is not a directory`)

	if got := repo.read("docs/x.md"); got != "repo-owned\n" {
		t.Errorf("docs/x.md = %q, want %q", got, "repo-owned\n")
	}
	repo.assertEntries("docs/", "docs/x.md", "vendor")
}

func TestRunPruneFileParentRefused(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("docs", "not a directory\n")

	p := &plan.Plan{Prune: []string{"docs/x.md"}, Lock: emptyLock()}
	err := apply.Run(repo.dir, nil, p)
	assertError(t, err, `cannot remove "docs/x.md": "docs" is not a directory`)

	repo.assertEntries("docs")
}

// internal/plan makes the write set and the prune set a difference over path strings, and
// on a case-sensitive filesystem that is the end of it. On APFS and NTFS it is not: a
// source renaming Foo.md to foo.md puts one path in each set naming one file, and pruning
// it would delete the file the write just created — leaving graft.lock claiming a path that
// is not there and a run that reported success.
//
// The comparison is by identity rather than by folded string, so a filesystem where the two
// really are different files still performs both operations. That is what this test asserts
// on each platform: the written file survives everywhere, and the old name survives only
// where it is a file of its own.
func TestRunNeverPrunesAFileItJustWrote(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file(".claude/agents/Foo.md", "old\n")
	src := newTree(t)
	src.file("extras/foo.md", "new\n")

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/foo.md", ".claude/agents/foo.md")},
		Prune:  []string{".claude/agents/Foo.md"},
		Lock:   lockOf(".claude/agents/foo.md"),
	}
	if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The file the lock claims is on disk with the source's bytes, whichever filesystem
	// this is running on.
	if got := repo.read(".claude/agents/foo.md"); got != "new\n" {
		t.Errorf(".claude/agents/foo.md = %q, want %q", got, "new\n")
	}
	if !repo.exists(".claude/agents") {
		t.Fatal(".claude/agents/ was removed")
	}
}
