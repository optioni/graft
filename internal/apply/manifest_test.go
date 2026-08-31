package apply_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/apply"
)

// `graft add --no-sync` records what the consumer asked for and syncs nothing. It cannot
// reach that through a plan: an empty plan applied against a populated lock has a prune
// set of everything the lock claims, so the convenient path is a data-loss bug. This is
// the entry point that cannot express one.

func TestManifestWritesGraftTomlAndNothingElse(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	if err := apply.Manifest(repo.dir, []byte("[sources.shared]\n")); err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	if got := repo.read("graft.toml"); got != "[sources.shared]\n" {
		t.Errorf("graft.toml = %q", got)
	}
	if want := []string{"graft.toml"}; !slices.Equal(repo.entries(), want) {
		t.Errorf("the repository holds %v, want %v", repo.entries(), want)
	}
}

// A manifest-only apply has no prune set, and the proof is a repository whose lock claims
// files: every one of them is still there afterwards.
func TestManifestPrunesNothing(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("graft.lock", "version = 1\n")
	repo.file(".claude/agents/reviewer.md", "# reviewer\n")
	repo.file(".claude/agents/repo-owned.md", "# not graft's\n")
	before := repo.entries()

	if err := apply.Manifest(repo.dir, []byte("[sources.shared]\n")); err != nil {
		t.Fatalf("Manifest: %v", err)
	}

	if got := repo.read("graft.lock"); got != "version = 1\n" {
		t.Errorf("graft.lock changed: %q", got)
	}
	want := append(slices.Clone(before), "graft.toml")
	slices.Sort(want)
	got := repo.entries()
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("the repository holds %v, want %v", got, want)
	}
}

func TestManifestRefusesAGraftTomlThatIsNotARegularFile(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.mkdir("graft.toml")

	err := apply.Manifest(repo.dir, []byte("[sources.shared]\n"))
	if err == nil {
		t.Fatal("Manifest succeeded over a directory named graft.toml")
	}
	if want := `cannot write "graft.toml": it exists and is not a regular file`; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}

// The staging path is the one path this package writes that no plan and no lock names, so
// a leftover that is not a regular file is refused rather than removed: deleting a path no
// lock claims is the one thing graft may never do.
func TestManifestRefusesAStagingLeftoverThatIsNotARegularFile(t *testing.T) {
	t.Parallel()

	outside := filepath.Join(t.TempDir(), "elsewhere.toml")
	if err := os.WriteFile(outside, []byte("do not touch\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	repo := newTree(t)
	repo.symlink(outside, ".graft.toml.tmp")

	err := apply.Manifest(repo.dir, []byte("[sources.shared]\n"))
	if err == nil {
		t.Fatal("Manifest succeeded with a symlink at the staging path")
	}
	if !strings.HasPrefix(err.Error(), `cannot write ".graft.toml.tmp"`) {
		t.Errorf("error = %q, want it to name the staging path", err)
	}
	if repo.exists("graft.toml") {
		t.Error("graft.toml was written after the refusal")
	}
	if !repo.exists(".graft.toml.tmp") {
		t.Error("the staging symlink was removed: graft may not delete a path no lock claims")
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "do not touch\n" {
		t.Errorf("the symlink's target was written through: %q, %v", data, err)
	}
}

// A repository root that cannot be opened is named, exactly as the plan-carrying path
// names it: graft never creates the repository it runs in.
func TestManifestRefusesARootThatIsNotThere(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "no-such-repo")

	err := apply.Manifest(missing, []byte("[sources.shared]\n"))
	if err == nil {
		t.Fatal("Manifest succeeded against a root that does not exist")
	}
	if !strings.HasPrefix(err.Error(), "cannot open the repository root ") {
		t.Errorf("error = %q, want it to name the root", err)
	}
}
