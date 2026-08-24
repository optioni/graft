package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repo is a fixture git repository built in a temp dir. Every integration test in this
// package needs one, and every one of them needs the same two pieces of setup: an
// explicit default branch, so a fixture's branch name does not depend on the runner's
// git configuration, and user.name and user.email set on the repository rather than the
// machine, or committing fails on a clean CI runner where no global identity exists.
type repo struct {
	t   *testing.T
	dir string
}

// newRepo creates an initialised repository with an identity of its own.
func newRepo(t *testing.T) *repo {
	t.Helper()
	r := &repo{t: t, dir: t.TempDir()}
	r.git("-c", "init.defaultBranch=main", "init", "-q", r.dir)
	// Repository scope, never --global: a fixture that borrows the developer's identity
	// passes locally and fails everywhere else.
	r.git("-C", r.dir, "config", "user.name", "graft fixture")
	r.git("-C", r.dir, "config", "user.email", "fixture@graft.test")
	return r
}

// URL is the fixture's clone URL: a filesystem path, which exercises the same git code
// paths as a remote without needing a network.
func (r *repo) URL() string { return r.dir }

// git runs one git command and fails the test with its output if it does not succeed.
func (r *repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// write creates a file in the work tree, making its parent directories as needed.
func (r *repo) write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatalf("MkdirAll %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatalf("WriteFile %s: %v", full, err)
	}
}

// commit stages everything and commits it, returning the new commit's sha.
func (r *repo) commit(message string) string {
	r.t.Helper()
	r.git("-C", r.dir, "add", "-A")
	r.git("-C", r.dir, "commit", "-q", "-m", message)
	return r.head()
}

func (r *repo) head() string {
	r.t.Helper()
	return r.git("-C", r.dir, "rev-parse", "HEAD")
}

// tag creates a lightweight tag at HEAD, which points directly at the commit.
func (r *repo) tag(name string) { r.git("-C", r.dir, "tag", name) }

// annotatedTag creates an annotated tag at HEAD and returns the tag object's own sha,
// which is not a commit and must never reach graft.lock's resolved.
func (r *repo) annotatedTag(name string) string {
	r.t.Helper()
	r.git("-C", r.dir, "tag", "-a", name, "-m", "annotated "+name)
	return r.git("-C", r.dir, "rev-parse", "refs/tags/"+name)
}

// branch creates a branch at HEAD without switching to it.
func (r *repo) branch(name string) { r.git("-C", r.dir, "branch", name) }

// blob is what the commit actually recorded at path — the bytes an entry must hold,
// rather than a literal a test author guessed at.
func (r *repo) blob(sha, path string) string {
	r.t.Helper()
	cmd := exec.Command("git", "-C", r.dir, "cat-file", "blob", sha+":"+path)
	out, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("cat-file blob %s:%s: %v", sha, path, err)
	}
	return string(out)
}

// TestFixtureCommitsWithoutAMachineIdentity is the evidence for this harness. Run with
// the global and system config removed — which is what a clean CI runner looks like — it
// passes only because newRepo sets an identity on the repository itself.
func TestFixtureCommitsWithoutAMachineIdentity(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	sha := r.commit("one")
	if len(sha) != 40 {
		t.Fatalf("commit: want a 40-character sha, got %q", sha)
	}
	if got := r.git("-C", r.dir, "rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Errorf("default branch: got %q, want %q", got, "main")
	}
}
