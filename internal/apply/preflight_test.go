package apply_test

import (
	"testing"

	"github.com/optioni/graft/internal/apply"
	"github.com/optioni/graft/internal/plan"
)

// Every refusal this package specifies is decidable from an Lstat before the first byte is
// written. Discovering one halfway through would guarantee a partial apply — and, because
// the lock is never written, the identical failure would repeat on every subsequent sync,
// leaving the user a stuck command and a tree graft cannot describe.
func TestPreflightRefusesBeforeAnyWrite(t *testing.T) {
	t.Parallel()

	t.Run("a destination that is not a regular file", func(t *testing.T) {
		t.Parallel()

		repo := newTree(t)
		repo.file("docs/c/index.md", "mine\n")
		src := newTree(t)
		src.file("extras/a", "a\n")
		src.file("extras/b", "b\n")
		src.file("extras/c", "c\n")

		p := &plan.Plan{
			Writes: []plan.Write{
				write("extras/a", "docs/a"),
				write("extras/b", "docs/b"),
				write("extras/c", "docs/c"),
			},
			Lock: lockOf("docs/a", "docs/b", "docs/c"),
		}
		err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
		assertError(t, err, `cannot write "docs/c": it exists and is not a regular file`)

		repo.assertEntries("docs/", "docs/c/", "docs/c/index.md")
	})

	t.Run("a prune path that is not a regular file", func(t *testing.T) {
		t.Parallel()

		repo := newTree(t)
		repo.file("docs/api/index.md", "mine\n")
		repo.file("graft.lock", "the previous lock\n")
		src := newTree(t)
		src.file("extras/a", "a\n")

		p := &plan.Plan{
			Writes: []plan.Write{write("extras/a", "openspec/a.md")},
			Prune:  []string{"docs/api"},
			Lock:   lockOf("openspec/a.md"),
		}
		err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
		assertError(t, err, `cannot remove "docs/api": it is not a regular file`)

		repo.assertEntries("docs/", "docs/api/", "docs/api/index.md", "graft.lock")
		if got := repo.read("graft.lock"); got != "the previous lock\n" {
			t.Errorf("graft.lock = %q, want the previous one untouched", got)
		}
	})

	t.Run("a source file that is not there", func(t *testing.T) {
		t.Parallel()

		repo := newTree(t)
		src := newTree(t)
		src.file("extras/a", "a\n")

		p := &plan.Plan{
			Writes: []plan.Write{
				write("extras/a", "docs/a"),
				write("extras/gone.md", "docs/b"),
			},
			Lock: lockOf("docs/a", "docs/b"),
		}
		err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
		assertErrorPrefix(t, err, `source "shared": cannot read "extras/gone.md": `)

		repo.assertEntries()
	})

	t.Run("an unregistered source named by the second write", func(t *testing.T) {
		t.Parallel()

		repo := newTree(t)
		src := newTree(t)
		src.file("extras/a", "a\n")

		p := &plan.Plan{
			Writes: []plan.Write{
				write("extras/a", "docs/a"),
				{Source: "other", Item: "agent:x", From: "extras/x", Dest: "docs/b"},
			},
			Lock: lockOf("docs/a", "docs/b"),
		}
		err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
		assertError(t, err, `source "other": no fetched tree`)

		repo.assertEntries()
	})

	t.Run("a reserved path named by the second write", func(t *testing.T) {
		t.Parallel()

		repo := newTree(t)
		src := newTree(t)
		src.file("extras/a", "a\n")
		src.file("extras/config", "[core]\n")

		p := &plan.Plan{
			Writes: []plan.Write{
				write("extras/a", "docs/a"),
				write("extras/config", ".git/config"),
			},
			Lock: lockOf(".git/config", "docs/a"),
		}
		err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
		assertError(t, err, `cannot write ".git/config": graft never writes inside ".git"`)

		repo.assertEntries()
	})
}
