package apply_test

import (
	"testing"

	"github.com/optioni/graft/internal/apply"
	"github.com/optioni/graft/internal/plan"
)

// SPEC.md offers one mitigation for an untrusted source: every sync's effect is a git
// diff. A file placed inside .git/ is invisible to it — untracked, so git status says
// nothing — and .git/config alone turns placing a file into running a program, through
// core.fsmonitor, core.sshCommand, or an alias. Nothing upstream catches it: internal/plan
// refuses a destination that escapes the repository root, and .git/config does not escape
// it, while kinds are arbitrary and no rule constrains what a `to` may name.
func TestRunReservedPaths(t *testing.T) {
	t.Parallel()

	t.Run("a destination inside .git", func(t *testing.T) {
		t.Parallel()

		repo := newTree(t)
		repo.file(".git/config", "[core]\n")
		src := newTree(t)
		src.file("extras/config", "[core]\n\tsshCommand = pwn\n")

		p := &plan.Plan{
			Writes: []plan.Write{write("extras/config", ".git/config")},
			Lock:   lockOf(".git/config"),
		}
		err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
		assertError(t, err, `cannot write ".git/config": graft never writes inside ".git"`)

		if got := repo.read(".git/config"); got != "[core]\n" {
			t.Errorf(".git/config = %q, want %q", got, "[core]\n")
		}
		if repo.exists("graft.lock") {
			t.Error("graft.lock was written; the check runs before anything else")
		}
	})

	t.Run("a prune path inside .git", func(t *testing.T) {
		t.Parallel()

		repo := newTree(t)
		repo.file(".git/hooks/pre-commit", "#!/bin/sh\n")

		p := &plan.Plan{
			Prune: []string{".git/hooks/pre-commit"},
			Lock:  emptyLock(),
		}
		err := apply.Run(repo.dir, nil, p)
		assertError(t, err, `cannot remove ".git/hooks/pre-commit": graft never removes inside ".git"`)

		if !repo.exists(".git/hooks/pre-commit") {
			t.Error("the hook was removed; a hand-edited lock may not aim a deletion into .git")
		}
	})

	t.Run("graft's own two files", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"graft.toml", "graft.lock"} {
			repo := newTree(t)
			repo.file(name, "mine\n")
			src := newTree(t)
			src.file("extras/x", "theirs\n")

			p := &plan.Plan{
				Writes: []plan.Write{write("extras/x", name)},
				Lock:   lockOf(name),
			}
			err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
			assertError(t, err, `cannot write "`+name+`": graft never writes over "`+name+`"`)

			if got := repo.read(name); got != "mine\n" {
				t.Errorf("%s = %q, want %q", name, got, "mine\n")
			}
		}
	})

	t.Run("graft's own two files as prune paths", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"graft.toml", "graft.lock"} {
			repo := newTree(t)
			repo.file(name, "mine\n")

			p := &plan.Plan{Prune: []string{name}, Lock: emptyLock()}
			err := apply.Run(repo.dir, nil, p)
			assertError(t, err, `cannot remove "`+name+`": graft never removes "`+name+`"`)

			if got := repo.read(name); got != "mine\n" {
				t.Errorf("%s = %q, want %q", name, got, "mine\n")
			}
		}
	})

	// The rule is on the first path segment, never a string prefix. Placing a workflow
	// file is something ENGINEERING.md's security note accepts and names — CI runs it, and
	// trusting a source means trusting what it places.
	t.Run("a path merely beginning with .git", func(t *testing.T) {
		t.Parallel()

		repo := newTree(t)
		src := newTree(t)
		src.file("extras/ci.yml", "on: push\n")
		src.file("extras/ignore", "*.tmp\n")

		p := &plan.Plan{
			Writes: []plan.Write{
				write("extras/ci.yml", ".github/workflows/ci.yml"),
				write("extras/ignore", ".gitignore"),
			},
			Lock: lockOf(".github/workflows/ci.yml", ".gitignore"),
		}
		if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err != nil {
			t.Fatalf("Run: %v", err)
		}

		if got := repo.read(".github/workflows/ci.yml"); got != "on: push\n" {
			t.Errorf(".github/workflows/ci.yml = %q", got)
		}
		if got := repo.read(".gitignore"); got != "*.tmp\n" {
			t.Errorf(".gitignore = %q", got)
		}
	})
}

// A case-insensitive filesystem — APFS by default, and NTFS — makes ".GIT/config" the same
// file as ".git/config". A byte-exact comparison is therefore no refusal at all on the
// platform most of this is developed on: the write lands in the real .git/config, and a
// prune aimed there takes the repository's hooks with it.
//
// The fold is unconditional rather than per-platform, so the behavior and these tests are
// the same everywhere. A directory genuinely named ".GIT" on a case-sensitive filesystem is
// not a destination worth supporting.
func TestRunReservedPathsFoldCase(t *testing.T) {
	t.Parallel()

	t.Run("a destination", func(t *testing.T) {
		t.Parallel()

		for _, dest := range []string{".GIT/config", ".Git/config", "GRAFT.TOML", "Graft.Lock"} {
			repo := newTree(t)
			repo.file(".git/config", "[core]\n")
			src := newTree(t)
			src.file("extras/x", "[core]\n\tsshCommand = pwn\n")

			p := &plan.Plan{
				Writes: []plan.Write{write("extras/x", dest)},
				Lock:   lockOf(dest),
			}
			if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err == nil {
				t.Errorf("writing %q was allowed", dest)
			}
			if got := repo.read(".git/config"); got != "[core]\n" {
				t.Errorf("after writing %q, .git/config = %q", dest, got)
			}
		}
	})

	t.Run("a prune path", func(t *testing.T) {
		t.Parallel()

		for _, dest := range []string{".GIT/hooks/pre-commit", "GRAFT.LOCK"} {
			repo := newTree(t)
			repo.file(".git/hooks/pre-commit", "#!/bin/sh\n")

			p := &plan.Plan{Prune: []string{dest}, Lock: emptyLock()}
			if err := apply.Run(repo.dir, nil, p); err == nil {
				t.Errorf("pruning %q was allowed", dest)
			}
			if !repo.exists(".git/hooks/pre-commit") {
				t.Errorf("pruning %q removed the hook", dest)
			}
		}
	})
}

// The floor refuses a path that names nothing rather than passing it downstream. Neither
// internal/plan nor internal/lock can produce one, but this function exists to hold when a
// check upstream is wrong.
func TestRunReservedPathsRefuseEmpty(t *testing.T) {
	t.Parallel()

	for _, dest := range []string{"", "."} {
		repo := newTree(t)
		src := newTree(t)
		src.file("extras/x", "x\n")

		p := &plan.Plan{
			Writes: []plan.Write{write("extras/x", dest)},
			Lock:   emptyLock(),
		}
		if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err == nil {
			t.Errorf("writing %q was allowed", dest)
		}
		repo.assertEntries()
	}
}
