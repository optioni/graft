package source

import (
	"strings"
	"testing"
)

// testSHA is a well-formed sha that belongs to no fixture. It stands in wherever the
// value's shape matters and its existence does not.
const testSHA = "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5"

// TestGitNotOnPATH pins the message for the one runtime dependency SPEC.md declares.
// Surfacing exec.LookPath's own error would name a Go package the user cannot act on.
func TestGitNotOnPATH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	sha, matched, err := Resolve("shared", "/srv/mirrors/assets", "main")
	if err == nil {
		t.Fatalf("Resolve: want an error, got %q", sha)
	}
	if err.Error() != "git not found on PATH" {
		t.Errorf("Resolve:\n got %q\nwant %q", err.Error(), "git not found on PATH")
	}
	if strings.Contains(err.Error(), "exec") {
		t.Errorf("Resolve: message surfaces an exec error a user cannot act on: %q", err)
	}
	if matched != "" {
		t.Errorf("Resolve: want an empty matched tag on failure, got %q", matched)
	}
}

func TestResolveRefusesOptionShapedRemote(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	want := `source "shared": git "--upload-pack=./pwn.sh" may not begin with "-"`

	sha, matched, err := Resolve("shared", "--upload-pack=./pwn.sh", "main")
	if err == nil || err.Error() != want {
		t.Fatalf("Resolve:\n got %v\nwant %q", err, want)
	}
	if sha != "" {
		t.Errorf("Resolve: want an empty sha on refusal, got %q", sha)
	}
	if matched != "" {
		t.Errorf("Resolve: want an empty matched tag on refusal, got %q", matched)
	}
}

func TestResolveBranchResolvesToItsTip(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	first := r.commit("one")
	r.write("b.txt", "two\n")
	tip := r.commit("two")

	got, matched, err := Resolve("shared", r.URL(), "main")
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	if got != tip {
		t.Errorf("Resolve(main):\n got %q\nwant %q (the tip, not %q)", got, tip, first)
	}
	if matched != "" {
		t.Errorf("Resolve(main): matched = %q, want empty: a branch is a ref and names itself", matched)
	}
}

func TestResolveLightweightTagResolvesToItsCommit(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	commit := r.commit("one")
	r.tag("v1.0.1")

	got, matched, err := Resolve("shared", r.URL(), "v1.0.1")
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	if got != commit {
		t.Errorf("Resolve(v1.0.1):\n got %q\nwant %q", got, commit)
	}
	if matched != "" {
		t.Errorf("Resolve(v1.0.1): matched = %q, want empty: v1.0.1 is a ref and names itself", matched)
	}
}

// TestResolveAnnotatedTagResolvesToTheCommit asserts both halves: that the answer is the
// commit, and that it is not the tag object's own sha. Asserting only the first would
// pass against an implementation that never peels, because a lightweight tag would still
// work and nothing would say which case was covered.
func TestResolveAnnotatedTagResolvesToTheCommit(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	commit := r.commit("one")
	tagObject := r.annotatedTag("v1.0.0")

	if tagObject == commit {
		t.Fatalf("fixture: annotated tag object %q equals the commit; the test proves nothing", tagObject)
	}
	got, _, err := Resolve("shared", r.URL(), "v1.0.0")
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	if got != commit {
		t.Errorf("Resolve(v1.0.0):\n got %q\nwant the commit %q", got, commit)
	}
	if got == tagObject {
		t.Errorf("Resolve(v1.0.0): returned the tag object %q, which is not a commit and would put a non-commit into graft.lock", tagObject)
	}
}

func TestResolveTagWinsOverABranchOfTheSameName(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	first := r.commit("one")
	r.branch("release")
	r.write("b.txt", "two\n")
	tip := r.commit("two")
	r.tag("release")

	got, _, err := Resolve("shared", r.URL(), "release")
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	if got != tip {
		t.Errorf("Resolve(release):\n got %q\nwant the tag's commit %q, not the branch's %q", got, tip, first)
	}
}

// TestResolveFullSHAPassesThroughOffline gives an unreachable remote and an empty PATH,
// so it can pass only if no git command runs. Asserting merely that the value comes back
// would stay green against an implementation that resolved it remotely.
func TestResolveFullSHAPassesThroughOffline(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	got, _, err := Resolve("shared", "/nonexistent-remote", testSHA)
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	if got != testSHA {
		t.Errorf("Resolve(%q):\n got %q\nwant it unchanged", testSHA, got)
	}
}

// TestResolveUppercaseSHAIsNotASHA: graft.lock records lowercase hex, so silently
// lowercasing a rev would make graft.toml and graft.lock disagree about what was asked
// for. An uppercase rev is a ref name, and there is no such ref.
func TestResolveUppercaseSHAIsNotASHA(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")
	upper := strings.ToUpper(testSHA)

	sha, _, err := Resolve("shared", r.URL(), upper)
	if err == nil {
		t.Fatalf("Resolve(%q): want an error, got %q", upper, sha)
	}
	want := `source "shared": rev "` + upper + `" not found`
	if err.Error() != want {
		t.Errorf("Resolve(%q):\n got %q\nwant %q", upper, err.Error(), want)
	}
}

func TestResolveRevNoRefMatches(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")

	sha, _, err := Resolve("shared", r.URL(), "v9.9.9")
	if err == nil {
		t.Fatalf("Resolve: want an error, got %q", sha)
	}
	want := `source "shared": rev "v9.9.9" not found`
	if err.Error() != want {
		t.Errorf("Resolve:\n got %q\nwant %q", err.Error(), want)
	}
	if sha != "" {
		t.Errorf("Resolve: want an empty sha on failure, got %q", sha)
	}
}

// TestResolveAbbreviatedSHAIsNotARev uses a real abbreviation of a real commit. SPEC.md
// admits a tag, a branch, or a full sha and nothing else; an abbreviation is not stable
// enough to pin.
func TestResolveAbbreviatedSHAIsNotARev(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	commit := r.commit("one")
	short := commit[:7]

	sha, _, err := Resolve("shared", r.URL(), short)
	if err == nil {
		t.Fatalf("Resolve(%q): want an error, got %q", short, sha)
	}
	want := `source "shared": rev "` + short + `" not found`
	if err.Error() != want {
		t.Errorf("Resolve(%q):\n got %q\nwant %q", short, err.Error(), want)
	}
}

// TestResolveUnreachableRemote asserts the graft-owned prefix exactly and asserts the
// message is one line. Git's own wording is not pinned: over the local transport two
// processes write the same pipe, so which line arrives first is not deterministic.
func TestResolveUnreachableRemote(t *testing.T) {
	missing := t.TempDir() + "/nope"
	prefix := `source "shared": cannot reach "` + missing + `": `

	sha, _, err := Resolve("shared", missing, "main")
	if err == nil {
		t.Fatalf("Resolve: want an error, got %q", sha)
	}
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("Resolve:\n got %q\nwant a message beginning %q", err.Error(), prefix)
	}
	if rest := strings.TrimPrefix(err.Error(), prefix); rest == "" {
		t.Errorf("Resolve: want git's own first line after the prefix, got nothing: %q", err)
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("Resolve: message carries git's terminal advice on later lines: %q", err)
	}
	// One is a typo in graft.toml, the other a network or permission problem. They must
	// not read alike.
	if strings.Contains(err.Error(), "not found") {
		t.Errorf("Resolve: unreachable remote is indistinguishable from a missing rev: %q", err)
	}
	if sha != "" {
		t.Errorf("Resolve: want an empty sha on failure, got %q", sha)
	}
}

func TestResolveEmptyRev(t *testing.T) {
	// PATH is emptied so a green result cannot mean git ran and declined.
	t.Setenv("PATH", t.TempDir())

	sha, _, err := Resolve("shared", "/srv/mirrors/assets", "")
	if err == nil {
		t.Fatalf("Resolve: want an error, got %q", sha)
	}
	want := `source "shared": rev is empty`
	if err.Error() != want {
		t.Errorf("Resolve:\n got %q\nwant %q", err.Error(), want)
	}
	if sha != "" {
		t.Errorf("Resolve: want an empty sha on failure, got %q", sha)
	}
}
