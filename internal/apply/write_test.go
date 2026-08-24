package apply_test

import (
	"io/fs"
	"testing"

	"github.com/optioni/graft/internal/apply"
	"github.com/optioni/graft/internal/plan"
)

func TestRunCreatesParentDirectories(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	src := newTree(t)
	src.file("extras/schemas/tdd/templates/design.md", "# design\n")

	p := &plan.Plan{
		Writes: []plan.Write{write(
			"extras/schemas/tdd/templates/design.md",
			"openspec/schemas/tdd/templates/design.md",
		)},
		Lock: lockOf("openspec/schemas/tdd/templates/design.md"),
	}
	if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := repo.read("openspec/schemas/tdd/templates/design.md"); got != "# design\n" {
		t.Errorf("content = %q, want %q", got, "# design\n")
	}
	repo.assertEntries(
		"graft.lock",
		"openspec/",
		"openspec/schemas/",
		"openspec/schemas/tdd/",
		"openspec/schemas/tdd/templates/",
		"openspec/schemas/tdd/templates/design.md",
	)
	if got := repo.mode("openspec/schemas/tdd/templates"); got != 0o755 {
		t.Errorf("directory mode = %v, want %v", got, 0o755)
	}
}

// A synced file is a derived artifact: always overwritten, never merged, never backed up.
// git diff is the report, so there is nothing for this package to preserve.
func TestRunOverwritesHandEdits(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("openspec/schemas/tdd/schema.yaml", "edited by hand\n")
	src := newTree(t)
	src.file("extras/tdd/schema.yaml", "version: 1\n")

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/tdd/schema.yaml", "openspec/schemas/tdd/schema.yaml")},
		Lock:   lockOf("openspec/schemas/tdd/schema.yaml"),
	}
	if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := repo.read("openspec/schemas/tdd/schema.yaml"); got != "version: 1\n" {
		t.Errorf("content = %q, want %q", got, "version: 1\n")
	}
	repo.assertEntries(
		"graft.lock",
		"openspec/",
		"openspec/schemas/",
		"openspec/schemas/tdd/",
		"openspec/schemas/tdd/schema.yaml",
	)
}

// The mode is graft's, not the source's and not the destination's. The second case is
// the one that fails against a truncate-in-place implementation: the permission argument
// to a create-and-truncate open applies only when the file is created, so a destination
// someone once ran chmod +x on would keep the bit while graft replaced its contents with
// source-controlled bytes.
func TestRunNormalizesMode(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		sourceMode, destMode fs.FileMode
		destExists           bool
	}{
		"fresh":              {sourceMode: 0o755},
		"fresh setuid":       {sourceMode: 0o4755},
		"existing":           {sourceMode: 0o644, destMode: 0o755, destExists: true},
		"existing from 0600": {sourceMode: 0o644, destMode: 0o600, destExists: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := newTree(t)
			if tc.destExists {
				repo.fileMode(".claude/hooks/x", "old\n", tc.destMode)
			}
			src := newTree(t)
			src.fileMode("extras/x", "new\n", tc.sourceMode)

			p := &plan.Plan{
				Writes: []plan.Write{write("extras/x", ".claude/hooks/x")},
				Lock:   lockOf(".claude/hooks/x"),
			}
			if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err != nil {
				t.Fatalf("Run: %v", err)
			}

			if got := repo.read(".claude/hooks/x"); got != "new\n" {
				t.Errorf("content = %q, want %q", got, "new\n")
			}
			if got := repo.mode(".claude/hooks/x"); got != 0o644 {
				t.Errorf("mode = %v, want %v: a source may not leave an executable file in a consumer's repository", got, 0o644)
			}
		})
	}
}

func TestRunUnreadableSourceFile(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	src := newTree(t)

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/gone.md", "docs/gone.md")},
		Lock:   lockOf("docs/gone.md"),
	}
	err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
	assertErrorPrefix(t, err, `source "shared": cannot read "extras/gone.md": `)

	repo.assertEntries()
}

func TestRunUnregisteredSource(t *testing.T) {
	t.Parallel()

	repo := newTree(t)

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/x.md", "docs/x.md")},
		Lock:   lockOf("docs/x.md"),
	}
	err := apply.Run(repo.dir, map[string]string{}, p)
	assertError(t, err, `source "shared": no fetched tree`)

	repo.assertEntries()
}

// A plan with nothing in it is a legitimate state — a manifest with no sources, or one
// whose every item contributes no files — and the lock still gets written, because the
// lock is a record of the plan rather than of what happened to be written.
func TestRunEmptyPlanWritesOnlyTheLock(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("README.md", "mine\n")

	if err := apply.Run(repo.dir, nil, &plan.Plan{Lock: emptyLock()}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	repo.assertEntries("README.md", "graft.lock")
	if got := repo.read("README.md"); got != "mine\n" {
		t.Errorf("README.md = %q, want %q", got, "mine\n")
	}
	if got := repo.read("graft.lock"); got != "# graft.lock — generated by `graft sync`. Do not edit.\nversion = 1\n" {
		t.Errorf("graft.lock = %q", got)
	}
}

func TestRunTouchesNothingOutsideThePlan(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file(".claude/agents/local-reviewer.md", "repo-owned\n")
	repo.file("docs/notes.md", "notes\n")
	src := newTree(t)
	src.file("extras/apply.md", "# apply\n")

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/apply.md", ".claude/agents/apply.md")},
		Lock:   lockOf(".claude/agents/apply.md"),
	}
	if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	repo.assertEntries(
		".claude/",
		".claude/agents/",
		".claude/agents/apply.md",
		".claude/agents/local-reviewer.md",
		"docs/",
		"docs/notes.md",
		"graft.lock",
	)
	if got := repo.read(".claude/agents/local-reviewer.md"); got != "repo-owned\n" {
		t.Errorf("local-reviewer.md = %q, want %q", got, "repo-owned\n")
	}
	if got := repo.read("docs/notes.md"); got != "notes\n" {
		t.Errorf("docs/notes.md = %q, want %q", got, "notes\n")
	}
}

// The destination's own type has to be decided before a write can be safe: the write
// removes an existing destination so the new file gets graft's mode rather than the old
// one's, and removing a path it has not confirmed regular is the mistake this package
// exists to avoid. An empty directory would go; a symlink would go while its target
// stayed, which is a deletion of something graft never wrote.
func TestRunDestinationDirectoryRefused(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("docs/api/index.md", "mine\n")
	src := newTree(t)
	src.file("extras/api", "# api\n")

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/api", "docs/api")},
		Lock:   lockOf("docs/api"),
	}
	err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
	assertError(t, err, `cannot write "docs/api": it exists and is not a regular file`)

	repo.assertEntries("docs/", "docs/api/", "docs/api/index.md")
	if got := repo.read("docs/api/index.md"); got != "mine\n" {
		t.Errorf("docs/api/index.md = %q, want %q", got, "mine\n")
	}
}

func TestRunDestinationSymlinkRefused(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file(".claude/agents/local-reviewer.md", "repo-owned\n")
	repo.symlink("local-reviewer.md", ".claude/agents/x.md")
	src := newTree(t)
	src.file("extras/x.md", "from the source\n")

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/x.md", ".claude/agents/x.md")},
		Lock:   lockOf(".claude/agents/x.md"),
	}
	err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
	assertError(t, err, `cannot write ".claude/agents/x.md": it exists and is not a regular file`)

	// The check ran before anything was opened for writing, so the link's target is
	// untouched. A check after the open would have already truncated it.
	if got := repo.read(".claude/agents/local-reviewer.md"); got != "repo-owned\n" {
		t.Errorf("local-reviewer.md = %q, want %q", got, "repo-owned\n")
	}
	if !repo.exists(".claude/agents/x.md") {
		t.Error("the symlink was removed; graft may not delete what it did not write")
	}
}

// internal/source never lists anything but a regular file, so a plan reaching here with a
// directory or a link as a source path means something upstream changed. The refusal is
// worded as a read failure because that is what it is: there are no bytes to copy.
func TestRunSourcePathNotRegular(t *testing.T) {
	t.Parallel()

	for name, seed := range map[string]func(*tree){
		"a directory": func(x *tree) { x.file("extras/dir/inner.md", "x\n") },
		"a symlink":   func(x *tree) { x.file("extras/real.md", "x\n"); x.symlink("real.md", "extras/dir") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := newTree(t)
			src := newTree(t)
			seed(src)

			p := &plan.Plan{
				Writes: []plan.Write{write("extras/dir", "docs/x.md")},
				Lock:   lockOf("docs/x.md"),
			}
			err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
			assertError(t, err, `source "shared": cannot read "extras/dir": not a regular file`)

			repo.assertEntries()
		})
	}
}

// A registered tree that is not there at all. The message names the read the caller was
// about to attempt, because "the cache entry is missing" is not something a user can act
// on and the file that was wanted is.
func TestRunUnopenableSourceTree(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	missing := newTree(t).path("no-such-entry")

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/x.md", "docs/x.md")},
		Lock:   lockOf("docs/x.md"),
	}
	err := apply.Run(repo.dir, map[string]string{"shared": missing}, p)
	assertErrorPrefix(t, err, `source "shared": cannot read "extras/x.md": `)

	repo.assertEntries()
}
