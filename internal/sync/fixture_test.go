package sync_test

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/sync"
)

// Everything here is real: real git repositories, a real fetch cache, a real consumer
// directory. Nothing is mocked, and nothing reaches the network — a fixture's clone URL is
// a filesystem path, which exercises the same git code as a remote would.
//
// The cache root is a value the test names, never source.DefaultCacheRoot, so no test in
// this package can write into the developer's real ~/.cache/graft.

// repo is a fixture source repository. user.name and user.email are set on **the
// repository**, not the machine, or committing fails on a clean CI runner where no global
// identity exists.
type repo struct {
	t   *testing.T
	dir string
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	r := &repo{t: t, dir: t.TempDir()}
	r.git("-c", "init.defaultBranch=main", "init", "-q", r.dir)
	r.git("-C", r.dir, "config", "user.name", "graft fixture")
	r.git("-C", r.dir, "config", "user.email", "fixture@graft.test")
	return r
}

func (r *repo) git(args ...string) string {
	r.t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *repo) write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatalf("WriteFile: %v", err)
	}
}

func (r *repo) remove(path string) {
	r.t.Helper()
	if err := os.RemoveAll(filepath.Join(r.dir, filepath.FromSlash(path))); err != nil {
		r.t.Fatalf("RemoveAll: %v", err)
	}
}

func (r *repo) commit(message string) string {
	r.t.Helper()
	r.git("-C", r.dir, "add", "-A")
	r.git("-C", r.dir, "commit", "-q", "-m", message)
	return r.git("-C", r.dir, "rev-parse", "HEAD")
}

func (r *repo) tag(name string) { r.git("-C", r.dir, "tag", name) }

// catalog writes catalog.yaml with the two kinds SPEC.md's own examples use.
func (r *repo) catalog(provides string) {
	r.t.Helper()
	r.write("catalog.yaml", `version: 1

kinds:
  schema:
    to: "openspec/schemas/{name}"
  agent:
    to: ".claude/agents/"
    flatten: true

provides:
`+provides)
}

// seed is the fixture every test starts from unless it needs something else: one directory
// item of two files, one file item.
func (r *repo) seed() {
	r.t.Helper()
	r.catalog(`  - { kind: schema, name: tdd,      from: extras/schemas/tdd }
  - { kind: agent,  name: reviewer, from: extras/agents/reviewer.md }
`)
	r.write("extras/schemas/tdd/schema.yaml", "name: tdd\n")
	r.write("extras/schemas/tdd/templates/design.md", "# design\n")
	r.write("extras/agents/reviewer.md", "# reviewer\n")
}

// consumer is a repository graft runs in, with a fetch cache of its own.
type consumer struct {
	t     *testing.T
	dir   string
	cache string
}

func newConsumer(t *testing.T) *consumer {
	t.Helper()
	return &consumer{t: t, dir: t.TempDir(), cache: t.TempDir()}
}

// manifest writes graft.toml declaring the source "shared". install defaults to the seed
// fixture's two selectors.
func (c *consumer) manifest(r *repo, rev string, install ...string) {
	c.t.Helper()
	c.file("graft.toml", sourceBlock("shared", r, rev, install...))
}

// sourceBlock renders one [sources.<name>] table, so a test can write a manifest naming a
// source other than "shared" or naming two of them.
func sourceBlock(name string, r *repo, rev string, install ...string) string {
	if len(install) == 0 {
		install = []string{"schema:tdd", "agent:*"}
	}
	quoted := make([]string, 0, len(install))
	for _, s := range install {
		quoted = append(quoted, fmt.Sprintf("%q", s))
	}
	return fmt.Sprintf("[sources.%s]\ngit     = %q\nrev     = %q\ninstall = [%s]\n",
		name, r.dir, rev, strings.Join(quoted, ", "))
}

func (c *consumer) file(path, content string) {
	c.t.Helper()
	full := filepath.Join(c.dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		c.t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		c.t.Fatalf("WriteFile: %v", err)
	}
}

func (c *consumer) options() sync.Options {
	return sync.Options{Root: c.dir, CacheRoot: c.cache}
}

// run performs a sync with the consumer's own root and cache.
func (c *consumer) run() (*sync.Report, error) {
	c.t.Helper()
	return sync.Run(c.options())
}

func (c *consumer) read(path string) string {
	c.t.Helper()
	data, err := os.ReadFile(filepath.Join(c.dir, filepath.FromSlash(path)))
	if err != nil {
		c.t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func (c *consumer) exists(path string) bool {
	c.t.Helper()
	_, err := os.Lstat(filepath.Join(c.dir, filepath.FromSlash(path)))
	return err == nil
}

// entries lists everything below the consumer's repository, directories marked with a
// trailing slash, so "the working tree is unchanged" is one comparison.
func (c *consumer) entries() []string {
	c.t.Helper()
	var out []string
	err := filepath.WalkDir(c.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(c.dir, p)
		if relErr != nil || rel == "." {
			return relErr
		}
		name := filepath.ToSlash(rel)
		if d.IsDir() {
			name += "/"
		}
		out = append(out, name)
		return nil
	})
	if err != nil {
		c.t.Fatalf("walking: %v", err)
	}
	slices.Sort(out)
	return out
}

func (c *consumer) assertEntries(want ...string) {
	c.t.Helper()
	slices.Sort(want)
	if got := c.entries(); !slices.Equal(got, want) {
		c.t.Errorf("tree contents:\n got %v\nwant %v", got, want)
	}
}

// installed is what the seed fixture puts in a consumer, plus the manifest and the lock.
func installed() []string {
	return []string{
		".claude/",
		".claude/agents/",
		".claude/agents/reviewer.md",
		"graft.lock",
		"graft.toml",
		"openspec/",
		"openspec/schemas/",
		"openspec/schemas/tdd/",
		"openspec/schemas/tdd/schema.yaml",
		"openspec/schemas/tdd/templates/",
		"openspec/schemas/tdd/templates/design.md",
	}
}

func assertError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error, want %q", want)
	}
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error, want one mentioning %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want one mentioning %q", err.Error(), want)
	}
}
