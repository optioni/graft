package source

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestFetchIgnoresInheritedGitState is the case the planning review missed. graft may be
// running inside a git operation — a post-merge hook, `git rebase --exec`, `git bisect
// run` — and those set GIT_INDEX_FILE, GIT_DIR, and GIT_OBJECT_DIRECTORY. Inherited, the
// internal checkout writes the consumer's index: reproduced by hand, the index grew from
// 137 to 198 bytes and `git status` reported a phantom `AD catalog.yaml`.
//
// Each variable is set on its own, because they fail differently and the quiet ones are
// the dangerous ones — GIT_WORK_TREE makes the fetch fail loudly, while GIT_INDEX_FILE
// corrupts the consumer's index and reports success. The consumer's whole repository,
// .git included, is snapshotted and compared.
func TestFetchIgnoresInheritedGitState(t *testing.T) {
	for _, name := range gitState {
		t.Run(name, func(t *testing.T) {
			src := newRepo(t)
			src.write("catalog.yaml", "version: 1\n")
			sha := src.commit("one")

			consumer := newRepo(t)
			consumer.write("kept.txt", "kept\n")
			consumer.commit("one")
			before := snapshot(t, consumer.URL())

			t.Setenv(name, valueFor(name, consumer.URL()))

			c := Cache{Root: filepath.Join(t.TempDir(), "cache")}
			entry, err := c.Fetch("shared", src.URL(), sha)
			if err != nil {
				t.Fatalf("Fetch: unexpected error: %v", err)
			}
			if got := readEntryFile(t, entry, "catalog.yaml"); got != "version: 1\n" {
				t.Errorf("catalog.yaml:\n got %q\nwant %q", got, "version: 1\n")
			}
			assertSame(t, "consumer repository", before, snapshot(t, consumer.URL()))
		})
	}
}

// valueFor is a plausible value for each repo-state variable, of the kind git itself sets
// when it runs a hook.
func valueFor(name, repo string) string {
	switch name {
	case "GIT_DIR", "GIT_COMMON_DIR":
		return filepath.Join(repo, ".git")
	case "GIT_WORK_TREE":
		return repo
	case "GIT_INDEX_FILE":
		return filepath.Join(repo, ".git", "index")
	case "GIT_OBJECT_DIRECTORY":
		return filepath.Join(repo, ".git", "objects")
	case "GIT_ALTERNATE_OBJECT_DIRECTORIES":
		return filepath.Join(repo, ".git", "objects")
	case "GIT_INDEX_VERSION":
		return "4"
	case "GIT_NAMESPACE":
		return "graft-test"
	case "GIT_PREFIX":
		return "sub/"
	}
	return repo
}

// TestGitEnvStripsRepoState pins which variables are removed and which survive. The
// second half is the contract SPEC.md makes about credentials: graft does not interfere
// with the user's existing git setup.
func TestGitEnvStripsRepoState(t *testing.T) {
	in := []string{
		"GIT_DIR=/x/.git",
		"GIT_WORK_TREE=/x",
		"GIT_INDEX_FILE=/x/.git/index",
		"GIT_INDEX_VERSION=4",
		"GIT_OBJECT_DIRECTORY=/x/.git/objects",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=/y",
		"GIT_COMMON_DIR=/x/.git",
		"GIT_NAMESPACE=ns",
		"GIT_PREFIX=sub/",
		"GIT_ASKPASS=/usr/bin/askpass",
		"SSH_AUTH_SOCK=/tmp/agent",
		"HOME=/home/dev",
		"PATH=/usr/bin",
	}
	got := gitEnv(in)
	joined := strings.Join(got, "\n")

	for _, name := range gitState {
		if strings.Contains(joined, name+"=") {
			t.Errorf("gitEnv: %s survived; it points git at the consumer's repository", name)
		}
	}
	for _, kept := range []string{"GIT_ASKPASS=", "SSH_AUTH_SOCK=", "HOME=", "PATH="} {
		if !strings.Contains(joined, kept) {
			t.Errorf("gitEnv: %s was stripped; graft does not interfere with the user's git setup", kept)
		}
	}
	if !strings.Contains(joined, "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("gitEnv: prompting is not disabled; a private source with no credentials would hang")
	}
}

// TestFetchSuppressesConsumerHooks: a globally configured post-checkout hook is not
// source-controlled, so it is not an execution route a source opens — but it fires during
// graft's internal checkout and can write anywhere, which would make "writes only under
// the cache root" untrue in an ordinary setup.
func TestFetchSuppressesConsumerHooks(t *testing.T) {
	src := newRepo(t)
	src.write("catalog.yaml", "version: 1\n")
	sha := src.commit("one")

	hooks := t.TempDir()
	marker := filepath.Join(t.TempDir(), "fired")
	script := "#!/bin/sh\necho fired > " + marker + "\n"
	if err := os.WriteFile(filepath.Join(hooks, "post-checkout"), []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// A config file of the consumer's own, reached the way a user's global config is.
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(cfg, []byte("[core]\n\thooksPath = "+hooks+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	c := Cache{Root: filepath.Join(t.TempDir(), "cache")}
	if _, err := c.Fetch("shared", src.URL(), sha); err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if _, err := os.Lstat(marker); err == nil {
		t.Errorf("the consumer's post-checkout hook ran during graft's internal checkout")
	}
}

// TestCheckVersion pins the refusal that keeps attr.tree from failing open. git ignores
// an unknown -c key in silence, so on a git older than 2.40 the source's .gitattributes
// would take effect again with nothing said.
func TestCheckVersion(t *testing.T) {
	cases := []struct {
		out  string
		want string
	}{
		{"git version 2.48.1\n", ""},
		{"git version 2.40.0\n", ""},
		{"git version 3.0.0\n", ""},
		{"git version 2.39.5\n", "git 2.39.5 is too old: graft needs git 2.40 or newer"},
		{"git version 1.9.1\n", "git 1.9.1 is too old: graft needs git 2.40 or newer"},
		{"git version 2.39.3 (Apple Git-146)\n", "git 2.39.3 is too old: graft needs git 2.40 or newer"},
		// Unparseable is accepted: a git that does not report its version the usual way
		// is more likely a wrapper than an ancient binary.
		{"something else entirely\n", ""},
		{"git version banana\n", ""},
		{"", ""},
	}
	for _, c := range cases {
		err := checkVersion(c.out)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("checkVersion(%q): unexpected error %v", c.out, err)
		case c.want != "" && err == nil:
			t.Errorf("checkVersion(%q): want %q, got no error", c.out, c.want)
		case c.want != "" && err != nil && err.Error() != c.want:
			t.Errorf("checkVersion(%q):\n got %q\nwant %q", c.out, err.Error(), c.want)
		}
	}
}

// TestFetchConcurrentOnOneSHA covers the lost-rename branch, which the specs require and
// no test reached. Two runs racing on one sha both want the same immutable tree, so the
// loser treats the failure as a hit rather than an error.
func TestFetchConcurrentOnOneSHA(t *testing.T) {
	src := newRepo(t)
	src.write("catalog.yaml", "version: 1\n")
	sha := src.commit("one")

	c := Cache{Root: filepath.Join(t.TempDir(), "cache")}
	const n = 12
	entries := make([]string, n)
	errs := make([]error, n)

	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func() {
			defer wg.Done()
			entries[i], errs[i] = c.Fetch("shared", src.URL(), sha)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Fetch %d: %v", i, err)
		}
		if entries[i] != entries[0] {
			t.Errorf("Fetch %d:\n got %q\nwant the same entry %q", i, entries[i], entries[0])
		}
	}
	if got := readEntryFile(t, entries[0], "catalog.yaml"); got != "version: 1\n" {
		t.Errorf("catalog.yaml:\n got %q\nwant %q", got, "version: 1\n")
	}
	// No scaffold survives a race any more than it survives a failure.
	parent := filepath.Dir(entries[0])
	got, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range got {
		if strings.HasPrefix(e.Name(), ".graft-fetch-") {
			t.Errorf("a raced fetch left the scaffold %q behind", e.Name())
		}
	}
}

// TestFetchEntryPathSquattedByAFile covers the other half of the rename branch: the
// destination exists but is not a directory, so it is a failure rather than a hit.
func TestFetchEntryPathSquattedByAFile(t *testing.T) {
	src := newRepo(t)
	src.write("catalog.yaml", "version: 1\n")
	sha := src.commit("one")

	c := Cache{Root: filepath.Join(t.TempDir(), "cache")}
	entry, err := c.Entry("shared", src.URL(), sha)
	if err != nil {
		t.Fatalf("Entry: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(entry), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(entry, []byte("squatter\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := c.Fetch("shared", src.URL(), sha)
	if err == nil {
		t.Fatalf("Fetch: want an error, got %q", got)
	}
	prefix := `source "shared": cannot create cache entry for "` + sha + `": `
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Errorf("Fetch:\n got %q\nwant a message beginning %q", err.Error(), prefix)
	}
	data, readErr := os.ReadFile(entry)
	if readErr != nil || string(data) != "squatter\n" {
		t.Errorf("the squatting file was disturbed: %q, %v", data, readErr)
	}
}
