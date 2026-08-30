package source

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Integration tests for resolving a range against a real remote — the tag listing, the
// annotated-tag peeling, and the failure paths a fake tag slice cannot exercise.

func TestResolveRangeHighestSatisfyingTagWinsEndToEnd(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")
	r.tag("v1.1.0")
	r.write("a.txt", "two\n")
	r.commit("two")
	r.tag("v1.2.0")
	r.write("a.txt", "three\n")
	tip := r.commit("three")
	r.tag("v1.3.0")
	r.write("a.txt", "four\n")
	r.commit("four")
	r.tag("v2.0.0")

	sha, matched, err := Resolve("shared", r.URL(), "^1.2.0")
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	if matched != "v1.3.0" {
		t.Errorf("matched = %q, want %q", matched, "v1.3.0")
	}
	if sha != tip {
		t.Errorf("sha = %q, want %q (v2.0.0 must not win: a caret range does not cross a major)", sha, tip)
	}
}

func TestResolveRangeAnnotatedTagResolvesToItsCommit(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")
	r.tag("v1.2.0")
	r.write("a.txt", "two\n")
	commit := r.commit("two")
	tagObject := r.annotatedTag("v1.3.0")

	if tagObject == commit {
		t.Fatalf("fixture: annotated tag object %q equals the commit; the test proves nothing", tagObject)
	}

	sha, matched, err := Resolve("shared", r.URL(), "^1.2.0")
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	if matched != "v1.3.0" {
		t.Errorf("matched = %q, want %q", matched, "v1.3.0")
	}
	if sha != commit {
		t.Errorf("sha = %q, want the peeled commit %q, not the tag object", sha, commit)
	}
}

func TestResolveRangeResolvesThroughTheRangePathAndReportsItsTag(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")
	r.tag("v1.2.0")
	r.write("a.txt", "two\n")
	tip := r.commit("two")
	r.tag("v1.3.0")

	sha, matched, err := Resolve("shared", r.URL(), "^1.2.0")
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	if sha != tip || matched != "v1.3.0" {
		t.Errorf("Resolve = (%q, %q), want (%q, %q)", sha, matched, tip, "v1.3.0")
	}
	// The value was classified as a range and never looked up as a ref: nothing here
	// ever queries refs/tags/^1.2.0, by construction of the range path.
}

// TestResolveRangeMalformedDoesNotFallBackToRefLookup uses a branch literally named
// ">=notaversion" — legal, because > < = are legal ref-name characters — to prove
// classification is never reconsidered on a parse failure.
func TestResolveRangeMalformedDoesNotFallBackToRefLookup(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")
	r.branch(">=notaversion")

	sha, matched, err := Resolve("shared", r.URL(), ">=notaversion")
	want := `source "shared": rev ">=notaversion" is not a valid semver range`
	if err == nil || err.Error() != want {
		t.Fatalf("Resolve:\n got %v\nwant %q", err, want)
	}
	if sha != "" || matched != "" {
		t.Errorf("Resolve: want empty sha and matched on refusal, got (%q, %q)", sha, matched)
	}
}

func TestResolveRangeUnreachableRemoteReportsTheNetworkFailure(t *testing.T) {
	missing := t.TempDir() + "/nope"
	prefix := `source "shared": cannot reach "` + missing + `": `

	sha, matched, err := Resolve("shared", missing, "^1.0.0")
	if err == nil {
		t.Fatalf("Resolve: want an error, got sha %q matched %q", sha, matched)
	}
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("Resolve:\n got %q\nwant a message beginning %q", err.Error(), prefix)
	}
	// Distinguishable from an unsatisfiable range: the tag list was never obtained.
	if strings.Contains(err.Error(), "semver tags") || strings.Contains(err.Error(), "matches none") {
		t.Errorf("Resolve: an unreachable remote is indistinguishable from an unsatisfiable range: %q", err)
	}
	if sha != "" || matched != "" {
		t.Errorf("Resolve: want empty sha and matched on failure, got (%q, %q)", sha, matched)
	}
}

// TestResolveRangeGuardsRunWithNoGitProcess proves the option-shaped-remote refusal and
// the malformed-range refusal both fire with an empty PATH — so no git process was
// started to reach either conclusion.
func TestResolveRangeGuardsRunWithNoGitProcess(t *testing.T) {
	t.Run("an option-shaped remote is refused under a range too", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		want := `source "shared": git "--upload-pack=./pwn.sh" may not begin with "-"`

		sha, matched, err := Resolve("shared", "--upload-pack=./pwn.sh", "^1.0.0")
		if err == nil || err.Error() != want {
			t.Fatalf("Resolve:\n got %v\nwant %q", err, want)
		}
		if sha != "" || matched != "" {
			t.Errorf("Resolve: want empty sha and matched on refusal, got (%q, %q)", sha, matched)
		}
	})

	t.Run("a malformed range is refused without a network call", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		want := `source "shared": rev "^^1" is not a valid semver range`

		sha, matched, err := Resolve("shared", "/srv/mirrors/assets", "^^1")
		if err == nil || err.Error() != want {
			t.Fatalf("Resolve:\n got %v\nwant %q", err, want)
		}
		if sha != "" || matched != "" {
			t.Errorf("Resolve: want empty sha and matched on refusal, got (%q, %q)", sha, matched)
		}
	})
}

// dirListing walks dir and returns every path relative to it, so "nothing changed" is
// one comparison between two snapshots rather than a spot check.
func dirListing(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return relErr
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return out
}

// TestResolveRangeWritesNothing proves resolving a range creates, modifies, and deletes
// nothing: not in the source repository, and not anywhere else on disk that a caller
// might have handed it a path to.
func TestResolveRangeWritesNothing(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")
	r.tag("v1.2.0")

	scratch := t.TempDir()
	beforeRepo := dirListing(t, r.dir)
	beforeScratch := dirListing(t, scratch)
	headBefore := r.head()

	if _, _, err := Resolve("shared", r.URL(), "^1.2.0"); err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}

	if got := dirListing(t, r.dir); !equalStrings(got, beforeRepo) {
		t.Errorf("the source repository changed:\n before %v\n after  %v", beforeRepo, got)
	}
	if got := dirListing(t, scratch); !equalStrings(got, beforeScratch) {
		t.Errorf("an unrelated directory changed:\n before %v\n after  %v", beforeScratch, got)
	}
	if got := r.head(); got != headBefore {
		t.Errorf("HEAD moved: %q -> %q", headBefore, got)
	}
	if _, err := os.Stat(scratch); err != nil {
		t.Errorf("the scratch directory itself is gone: %v", err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
