package source

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// snapshot records every path below dir with its bytes, so a test can assert a tree is
// untouched rather than assert only that one file it thought of is still there.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			out[rel+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return out
}

func assertSame(t *testing.T, what string, before, after map[string]string) {
	t.Helper()
	for p, want := range before {
		got, ok := after[p]
		switch {
		case !ok:
			t.Errorf("%s: %q disappeared", what, p)
		case got != want:
			t.Errorf("%s: %q changed:\n got %q\nwant %q", what, p, got, want)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			t.Errorf("%s: %q appeared", what, p)
		}
	}
}

func readEntryFile(t *testing.T, entry, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(entry, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestFetchRefusesOptionShapedRemote(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	want := `source "shared": git "--upload-pack=./pwn.sh" may not begin with "-"`

	entry, err := Cache{Root: t.TempDir()}.Fetch("shared", "--upload-pack=./pwn.sh", testSHA)
	if err == nil || err.Error() != want {
		t.Fatalf("Fetch:\n got %v\nwant %q", err, want)
	}
	if entry != "" {
		t.Errorf("Fetch: want an empty entry on refusal, got %q", entry)
	}
}

func TestFetchWritesTheTree(t *testing.T) {
	r := newRepo(t)
	r.write("catalog.yaml", "version: 1\n")
	r.write("extras/agents/a.md", "agent a\n")
	sha := r.commit("one")

	c := Cache{Root: filepath.Join(t.TempDir(), "cache")}
	entry, err := c.Fetch("shared", r.URL(), sha)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if got := readEntryFile(t, entry, "catalog.yaml"); got != "version: 1\n" {
		t.Errorf("catalog.yaml:\n got %q\nwant %q", got, "version: 1\n")
	}
	if got := readEntryFile(t, entry, "extras/agents/a.md"); got != "agent a\n" {
		t.Errorf("extras/agents/a.md:\n got %q\nwant %q", got, "agent a\n")
	}
	// The cache holds a tree, not a repository. An entry with a .git is a repository
	// anything walking the entry did not expect to find.
	if _, err := os.Lstat(filepath.Join(entry, ".git")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("entry holds a .git; the cache stores a tree, not a repository")
	}
}

// TestFetchIgnoresGitattributes: git checkout normally honours the source's own
// committed .gitattributes, which is a file the source controls. The bytes are the
// visible half; the reason it matters is `filter=`, which selects a driver whose command
// comes from the consumer's git config.
func TestFetchIgnoresGitattributes(t *testing.T) {
	r := newRepo(t)
	r.write(".gitattributes", "* text eol=crlf\n*.md ident\n")
	r.write("a.txt", "hello\nworld\n")
	r.write("b.md", "$Id$\n")
	sha := r.commit("one")

	c := Cache{Root: filepath.Join(t.TempDir(), "cache")}
	entry, err := c.Fetch("shared", r.URL(), sha)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	// Compared against what the commit recorded, not against a literal a test author
	// guessed at.
	for _, path := range []string{"a.txt", "b.md"} {
		want := r.blob(sha, path)
		if got := readEntryFile(t, entry, path); got != want {
			t.Errorf("%s:\n got %q\nwant the committed blob %q", path, got, want)
		}
	}
}

func TestFetchOlderCommitGetsThatCommitsTree(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	first := r.commit("one")
	r.write("b.txt", "two\n")
	r.commit("two")

	c := Cache{Root: filepath.Join(t.TempDir(), "cache")}
	entry, err := c.Fetch("shared", r.URL(), first)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if got := readEntryFile(t, entry, "a.txt"); got != "one\n" {
		t.Errorf("a.txt:\n got %q\nwant %q", got, "one\n")
	}
	if _, err := os.Lstat(filepath.Join(entry, "b.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("entry holds b.txt, which the second commit added; the first commit's tree was wanted")
	}
}

// TestFetchWritesNothingOutsideTheCacheRoot asserts the consumer's repository positively
// rather than checking only that the cache looks right. A test that inspected the cache
// alone would stay green if a write landed anywhere else.
func TestFetchWritesNothingOutsideTheCacheRoot(t *testing.T) {
	r := newRepo(t)
	r.write("catalog.yaml", "version: 1\n")
	sha := r.commit("one")

	consumer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(consumer, ".claude", "agents"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for path, content := range map[string]string{
		"graft.toml":                       "[sources.shared]\n",
		"graft.lock":                       "version = 1\n",
		".claude/agents/local-reviewer.md": "repo-owned\n",
	} {
		if err := os.WriteFile(filepath.Join(consumer, filepath.FromSlash(path)), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", path, err)
		}
	}
	before := snapshot(t, consumer)

	root := filepath.Join(t.TempDir(), "cache")
	if _, err := (Cache{Root: root}).Fetch("shared", r.URL(), sha); err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}

	assertSame(t, "consumer tree", before, snapshot(t, consumer))
}

// TestFetchCacheHitNeedsNoGit is SPEC.md's "network unavailable, cache hit: proceeds",
// asserted as what it actually claims: not that a failure is tolerated, but that nothing
// is tried. Deleting the source repository alone would not prove it — emptying PATH is
// what makes a git invocation impossible.
func TestFetchCacheHitNeedsNoGit(t *testing.T) {
	r := newRepo(t)
	r.write("catalog.yaml", "version: 1\n")
	sha := r.commit("one")

	c := Cache{Root: filepath.Join(t.TempDir(), "cache")}
	first, err := c.Fetch("shared", r.URL(), sha)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if err := os.RemoveAll(r.URL()); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	t.Setenv("PATH", t.TempDir())

	second, err := c.Fetch("shared", r.URL(), sha)
	if err != nil {
		t.Fatalf("Fetch on a cache hit with no remote and no git: %v", err)
	}
	if second != first {
		t.Errorf("Fetch:\n got %q\nwant the same entry %q", second, first)
	}
	if got := readEntryFile(t, second, "catalog.yaml"); got != "version: 1\n" {
		t.Errorf("catalog.yaml:\n got %q\nwant %q", got, "version: 1\n")
	}
}

func TestFetchCacheMissWithNoReachableRemote(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	root := filepath.Join(t.TempDir(), "cache")
	c := Cache{Root: root}
	prefix := `source "shared": cannot fetch "` + testSHA + `" from "` + missing + `": `

	entry, err := c.Fetch("shared", missing, testSHA)
	if err == nil {
		t.Fatalf("Fetch: want an error, got %q", entry)
	}
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("Fetch:\n got %q\nwant a message beginning %q", err.Error(), prefix)
	}
	if rest := strings.TrimPrefix(err.Error(), prefix); rest == "" {
		t.Errorf("Fetch: want git's own first line after the prefix, got nothing: %q", err)
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("Fetch: message carries git's terminal advice on later lines: %q", err)
	}
}

func TestFetchSHATheRemoteDoesNotHave(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")

	const absent = "0000000000000000000000000000000000000000"
	root := filepath.Join(t.TempDir(), "cache")
	c := Cache{Root: root}
	prefix := `source "shared": cannot fetch "` + absent + `" from "` + r.URL() + `": `

	entry, err := c.Fetch("shared", r.URL(), absent)
	if err == nil {
		t.Fatalf("Fetch: want an error, got %q", entry)
	}
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("Fetch:\n got %q\nwant a message beginning %q", err.Error(), prefix)
	}
	want, wantErr := c.Entry("shared", r.URL(), absent)
	if wantErr != nil {
		t.Fatalf("Entry: %v", wantErr)
	}
	if _, err := os.Lstat(want); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed fetch left something at the entry path %q", want)
	}
}

// TestFetchLeavesNoPartialEntry: an entry keyed by an immutable sha is never re-fetched,
// so an incomplete one would be wrong forever. Asserting only the entry path would stay
// green against an implementation that abandons its scaffold.
func TestFetchLeavesNoPartialEntry(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commit("one")

	const absent = "0000000000000000000000000000000000000000"
	c := Cache{Root: filepath.Join(t.TempDir(), "cache")}
	entry, err := c.Entry("shared", r.URL(), absent)
	if err != nil {
		t.Fatalf("Entry: %v", err)
	}
	if _, err := c.Fetch("shared", r.URL(), absent); err == nil {
		t.Fatalf("Fetch: want an error for a sha the remote does not have")
	}

	if _, err := os.Lstat(entry); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("entry %q exists after a failed fetch", entry)
	}
	parent := filepath.Dir(entry)
	if entries, err := os.ReadDir(parent); err == nil {
		var left []string
		for _, e := range entries {
			left = append(left, e.Name())
		}
		if len(left) != 0 {
			slices.Sort(left)
			t.Errorf("a failed fetch left %q in the entry's parent directory %q", left, parent)
		}
	}
}

func TestFetchUnusableCacheRoot(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	sha := r.commit("one")

	// The root names an existing file rather than a directory.
	root := filepath.Join(t.TempDir(), "cache")
	if err := os.WriteFile(root, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	prefix := `source "shared": cannot create cache entry for "` + sha + `": `

	entry, err := (Cache{Root: root}).Fetch("shared", r.URL(), sha)
	if err == nil {
		t.Fatalf("Fetch: want an error, got %q", entry)
	}
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Errorf("Fetch:\n got %q\nwant a message beginning %q", err.Error(), prefix)
	}
	data, readErr := os.ReadFile(root)
	if readErr != nil {
		t.Fatalf("the cache root file is gone: %v", readErr)
	}
	if string(data) != "not a directory\n" {
		t.Errorf("cache root file changed:\n got %q\nwant %q", data, "not a directory\n")
	}
}
