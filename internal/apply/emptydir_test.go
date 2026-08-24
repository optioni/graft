package apply_test

import (
	"testing"

	"github.com/optioni/graft/internal/apply"
	"github.com/optioni/graft/internal/plan"
)

func TestRunRemovesEmptiedDirectories(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("openspec/schemas/tdd/schema.yaml", "a\n")
	repo.file("openspec/schemas/tdd/templates/design.md", "b\n")
	repo.file("README.md", "mine\n")

	p := &plan.Plan{
		Prune: []string{
			"openspec/schemas/tdd/schema.yaml",
			"openspec/schemas/tdd/templates/design.md",
		},
		Lock: emptyLock(),
	}
	if err := apply.Run(repo.dir, nil, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	repo.assertEntries("README.md", "graft.lock")
}

func TestRunKeepsNonEmptyDirectory(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file(".claude/agents/local-reviewer.md", "repo-owned\n")
	repo.file(".claude/agents/apply-orchestrator.md", "synced\n")

	p := &plan.Plan{Prune: []string{".claude/agents/apply-orchestrator.md"}, Lock: emptyLock()}
	if err := apply.Run(repo.dir, nil, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	repo.assertEntries(
		".claude/", ".claude/agents/", ".claude/agents/local-reviewer.md", "graft.lock",
	)
}

// Only the ancestry of a pruned path is ever a candidate. A directory that was already
// empty before the sync was not emptied by graft, and removing it would be graft deleting
// something it has no record of.
func TestRunLeavesUnrelatedEmptyDirectory(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.mkdir("scratch")
	repo.file("docs/old.md", "synced\n")

	p := &plan.Plan{Prune: []string{"docs/old.md"}, Lock: emptyLock()}
	if err := apply.Run(repo.dir, nil, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	repo.assertEntries("graft.lock", "scratch/")
}

// Unlinking a symlink succeeds however full its target is, so "a non-empty directory fails
// harmlessly" is true of directories and false of links. Without a check, a user's
// vendor -> shared convenience link in the ancestry of a pruned path is deleted — a path
// absent from graft.lock, removed by graft, with nothing said.
func TestRunKeepsSymlinkedAncestor(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("shared/keep.md", "repo-owned\n")
	repo.symlink("shared", "agents")

	// The ancestor rule refuses the prune path before its existence is ever consulted, so
	// the removal walk is never reached. Its own non-directory check is therefore a floor
	// under this one rather than the thing that saves the link here.
	p := &plan.Plan{Prune: []string{"agents/x.md"}, Lock: emptyLock()}
	err := apply.Run(repo.dir, nil, p)
	assertError(t, err, `cannot remove "agents/x.md": "agents" is not a directory`)

	repo.assertEntries("agents", "shared/", "shared/keep.md")
}

// A link that is not in a pruned path's ancestry is never a candidate at all, which is the
// other half of "only the ancestry of a pruned path is considered".
func TestRunKeepsSymlinkedAncestorOfARealPrune(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("shared/keep.md", "repo-owned\n")
	repo.file("docs/x.md", "synced\n")
	repo.symlink("shared", "docs/link")

	p := &plan.Plan{Prune: []string{"docs/x.md"}, Lock: emptyLock()}
	if err := apply.Run(repo.dir, nil, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// docs/ still holds the link, so it is not empty and is kept; the link itself was
	// never a candidate, being neither an ancestor of the pruned path nor a directory.
	repo.assertEntries(
		"docs/", "docs/link", "graft.lock", "shared/", "shared/keep.md",
	)
	if got := repo.read("shared/keep.md"); got != "repo-owned\n" {
		t.Errorf("shared/keep.md = %q, want %q", got, "repo-owned\n")
	}
}

func TestRunPruneAtRoot(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("README.md", "synced\n")

	p := &plan.Plan{Prune: []string{"README.md"}, Lock: emptyLock()}
	if err := apply.Run(repo.dir, nil, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The repository root is never a removal candidate, and it is still here to hold the
	// lock that was written after the prune.
	repo.assertEntries("graft.lock")
}

// The order, pinned by the two adjacencies an effect can actually distinguish: docs/
// disappearing puts the directory removal after the prune, and templates/ surviving with
// new.md in it puts it after the writes.
func TestRunOrdersOperations(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("openspec/schemas/tdd/templates/old.md", "old\n")
	repo.file("docs/gone.md", "gone\n")
	src := newTree(t)
	src.file("extras/new.md", "new\n")

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/new.md", "openspec/schemas/tdd/templates/new.md")},
		Prune:  []string{"docs/gone.md", "openspec/schemas/tdd/templates/old.md"},
		Lock:   lockOf("openspec/schemas/tdd/templates/new.md"),
	}
	if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	repo.assertEntries(
		"graft.lock",
		"openspec/",
		"openspec/schemas/",
		"openspec/schemas/tdd/",
		"openspec/schemas/tdd/templates/",
		"openspec/schemas/tdd/templates/new.md",
	)
}

// A prune path the user had already deleted by hand unlinks nothing, so the directory it
// used to live in was not emptied by graft. Removing it anyway would be graft deleting
// something it has no record of — the same mistake as scanning a directory for files to
// prune, arrived at from the other side.
func TestRunKeepsADirectoryItDidNotEmpty(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.mkdir("vendor/tools")

	p := &plan.Plan{Prune: []string{"vendor/tools/x.md"}, Lock: emptyLock()}
	if err := apply.Run(repo.dir, nil, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	repo.assertEntries("graft.lock", "vendor/", "vendor/tools/")
}
