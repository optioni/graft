package apply_test

import (
	"testing"

	"github.com/optioni/graft/internal/apply"
	"github.com/optioni/graft/internal/plan"
)

// An os.Root refuses a path that leaves its root and follows a symlink that stays inside
// it. That second half is why the root alone is not the floor the invariants need: a write
// through a symlinked parent lands at a path graft.lock does not name, so the file an item
// places does not stay inside that item's own destination and a later prune aims wherever
// the link resolves that day.
func TestRunDestinationSymlinkedParentRefused(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("shared/agents/local-reviewer.md", "repo-owned\n")
	repo.symlink("shared", ".claude")
	src := newTree(t)
	src.file("extras/x.md", "from the source\n")

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/x.md", ".claude/agents/x.md")},
		Lock:   lockOf(".claude/agents/x.md"),
	}
	err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
	assertError(t, err, `cannot write ".claude/agents/x.md": ".claude" is not a directory`)

	repo.assertEntries(
		".claude",
		"shared/",
		"shared/agents/",
		"shared/agents/local-reviewer.md",
	)
}

// The message names the ancestor rather than surfacing the operating system's own "not a
// directory", which names neither the destination nor the path the user has to fix.
func TestRunDestinationFileParentRefused(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("openspec", "not a directory\n")
	src := newTree(t)
	src.file("extras/x.yaml", "name: tdd\n")

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/x.yaml", "openspec/schemas/x.yaml")},
		Lock:   lockOf("openspec/schemas/x.yaml"),
	}
	err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
	assertError(t, err, `cannot write "openspec/schemas/x.yaml": "openspec" is not a directory`)

	repo.assertEntries("openspec")
}

// The shallowest offender is named, so a destination with two bad ancestors always reports
// the same one rather than whichever the walk happened to reach.
func TestRunDestinationNamesTheShallowestBadAncestor(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("a/real", "x\n")
	repo.symlink("real", "a/link")
	repo.symlink("a", "top")
	src := newTree(t)
	src.file("extras/x.md", "y\n")

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/x.md", "top/link/x.md")},
		Lock:   lockOf("top/link/x.md"),
	}
	err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
	assertError(t, err, `cannot write "top/link/x.md": "top" is not a directory`)
}
