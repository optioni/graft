package apply_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/optioni/graft/internal/apply"
	"github.com/optioni/graft/internal/plan"
)

// The floor under internal/plan's repo-root rule and internal/source's own containment. It
// is the check that holds when a check upstream is wrong, and it costs one call — so it is
// exercised with plans plan.Build would refuse to produce.
func TestRunDestinationEscapeRefused(t *testing.T) {
	t.Parallel()

	outside := newTree(t)
	repo := newTree(t)
	src := newTree(t)
	src.file("extras/x.md", "from the source\n")

	rel, err := filepath.Rel(repo.dir, filepath.Join(outside.dir, "outside.md"))
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	escaping := filepath.ToSlash(rel)

	t.Run("as a destination", func(t *testing.T) {
		p := &plan.Plan{
			Writes: []plan.Write{write("extras/x.md", escaping)},
			Lock:   lockOf(escaping),
		}
		if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err == nil {
			t.Fatal("no error, want one: the destination leaves the repository root")
		}
		if outside.exists("outside.md") {
			t.Error("a file was written outside the repository root")
		}
	})

	t.Run("as a prune path", func(t *testing.T) {
		outside.file("outside.md", "not graft's\n")

		p := &plan.Plan{Prune: []string{escaping}, Lock: emptyLock()}
		if err := apply.Run(repo.dir, nil, p); err == nil {
			t.Fatal("no error, want one: the prune path leaves the repository root")
		}
		if got := outside.read("outside.md"); got != "not graft's\n" {
			t.Errorf("outside.md = %q, want %q", got, "not graft's\n")
		}
	})
}

func TestRunSourceEscapeRefused(t *testing.T) {
	t.Parallel()

	beside := newTree(t)
	beside.file("secret.md", "another cache entry\n")
	repo := newTree(t)
	src := newTree(t)

	rel, err := filepath.Rel(src.dir, filepath.Join(beside.dir, "secret.md"))
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	escaping := filepath.ToSlash(rel)

	p := &plan.Plan{
		Writes: []plan.Write{write(escaping, "docs/secret.md")},
		Lock:   lockOf("docs/secret.md"),
	}
	err = apply.Run(repo.dir, map[string]string{"shared": src.dir}, p)
	assertErrorPrefix(t, err, `source "shared": cannot read "`+escaping+`": `)

	repo.assertEntries()
}

func TestRunMissingRoot(t *testing.T) {
	t.Parallel()

	parent := newTree(t)
	missing := filepath.Join(parent.dir, "not-a-repository")

	err := apply.Run(missing, nil, &plan.Plan{Lock: emptyLock()})
	assertErrorPrefix(t, err, `cannot open the repository root "`+missing+`": `)

	// graft never creates the repository it runs in.
	if _, statErr := os.Lstat(missing); statErr == nil {
		t.Error("the root was created")
	}
	parent.assertEntries()
}
