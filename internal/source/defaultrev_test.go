package source

import (
	"path/filepath"
	"strings"
	"testing"
)

// The default pin `graft add` writes when the invocation names none. It is the source's
// own answer to "what should I install", which is why it is resolved from the remote
// rather than guessed at, and why it is a ref rather than a range: a range is a policy
// only the consumer can choose.

func TestDefaultRevTakesTheHighestStableTag(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")
	r.tag("v1.2.0")
	r.tag("nightly")
	r.write("a.txt", "two\n")
	r.commit("two")
	r.tag("v1.3.0")
	r.tag("v2.0.0-rc.1")

	got, err := DefaultRev("shared", r.URL())
	if err != nil {
		t.Fatalf("DefaultRev: %v", err)
	}
	if got != "v1.3.0" {
		t.Errorf("DefaultRev = %q, want %q: the prerelease is excluded and the non-semver tag ignored", got, "v1.3.0")
	}
}

// An annotated tag's own object sha is not a commit. DefaultRev returns a name, so the
// distinction is Resolve's to make later — but the name must still be the tag's, spelled
// as the remote spells it.
func TestDefaultRevNamesAnAnnotatedTagOnce(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")
	r.annotatedTag("v1.4.0")

	got, err := DefaultRev("shared", r.URL())
	if err != nil {
		t.Fatalf("DefaultRev: %v", err)
	}
	if got != "v1.4.0" {
		t.Errorf("DefaultRev = %q, want %q", got, "v1.4.0")
	}
}

func TestDefaultRevFallsBackToTheDefaultBranch(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")
	r.tag("nightly")
	r.git("-C", r.dir, "branch", "-m", "trunk")

	got, err := DefaultRev("shared", r.URL())
	if err != nil {
		t.Fatalf("DefaultRev: %v", err)
	}
	if got != "trunk" {
		t.Errorf("DefaultRev = %q, want %q", got, "trunk")
	}
}

// A source publishing only prereleases has nothing stable to offer, and the branch is a
// better answer than a release candidate nobody asked for.
func TestDefaultRevFallsBackWhenEveryTagIsAPrerelease(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")
	r.tag("v2.0.0-rc.1")

	got, err := DefaultRev("shared", r.URL())
	if err != nil {
		t.Fatalf("DefaultRev: %v", err)
	}
	if got != "main" {
		t.Errorf("DefaultRev = %q, want %q", got, "main")
	}
}

func TestDefaultRevRefusesARepositoryWithNothingToPin(t *testing.T) {
	t.Parallel()

	r := newRepo(t) // initialised and never committed to: no tags, no HEAD to report

	got, err := DefaultRev("shared", r.URL())
	if err == nil {
		t.Fatalf("DefaultRev = %q, want an error", got)
	}
	if want := `source "shared": has no semver tag and no default branch`; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
	if got != "" {
		t.Errorf("rev returned beside an error: %q", got)
	}
}

// A network failure is not an empty repository: the two are told apart so the reader
// knows whether to fix the URL or stop asking this source for a default.
func TestDefaultRevReportsAnUnreachableSourceAsUnreachable(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "no-such-repo")

	_, err := DefaultRev("shared", missing)
	if err == nil {
		t.Fatal("DefaultRev succeeded against a path with no repository")
	}
	if !strings.HasPrefix(err.Error(), `source "shared": cannot reach `) {
		t.Errorf("error = %q, want it to report an unreachable source", err)
	}
}

func TestDefaultRevRefusesAGitValueThatWouldBecomeAnOption(t *testing.T) {
	t.Parallel()

	_, err := DefaultRev("shared", "--upload-pack=touch /tmp/x")
	if err == nil {
		t.Fatal("DefaultRev succeeded on a git value beginning with a dash")
	}
	if want := `source "shared": git "--upload-pack=touch /tmp/x" may not begin with "-"`; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}
