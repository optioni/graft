package add_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/add"
	"github.com/optioni/graft/internal/ui"
)

// Everything here is real: a real git repository, a real fetch cache, a real consumer
// directory. Nothing reaches the network — a fixture's clone URL is a filesystem path,
// which exercises the same git code a remote would.
//
// The cache root is a value the test names, never source.DefaultCacheRoot, so no test in
// this package can write into the developer's real ~/.cache/graft.

// repo is a fixture source repository. Its directory is named rather than left as
// t.TempDir()'s numeric leaf, because the name graft derives is its last path segment.
// user.name and user.email are set on the repository, not the machine, or committing
// fails on a clean CI runner.
type repo struct {
	t   *testing.T
	dir string
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	r := &repo{t: t, dir: filepath.Join(t.TempDir(), "shared")}
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

func (r *repo) commit(message string) string {
	r.t.Helper()
	r.git("-C", r.dir, "add", "-A")
	r.git("-C", r.dir, "commit", "-q", "-m", message)
	return r.git("-C", r.dir, "rev-parse", "HEAD")
}

func (r *repo) tag(name string) { r.git("-C", r.dir, "tag", name) }

// head is the sha the fixture's HEAD names, so a test asserts against what git recorded
// rather than against a literal its author guessed at.
func (r *repo) head() string {
	r.t.Helper()
	return r.git("-C", r.dir, "rev-parse", "HEAD")
}

// shortSHA is the form graft prints, spelled here so the expectation and the renderer
// cannot drift apart silently.
func shortSHA(sha string) string { return ui.ShortSHA(sha) }

// seed is the offer every test here starts from: one directory item of two files, one
// file item, under the two kinds SPEC.md's own examples use.
func (r *repo) seed() {
	r.t.Helper()
	r.write("catalog.yaml", `version: 1

kinds:
  schema:
    to: "openspec/schemas/{name}"
  agent:
    to: ".claude/agents/"
    flatten: true

provides:
  - { kind: schema, name: tdd,      from: extras/schemas/tdd }
  - { kind: agent,  name: reviewer, from: extras/agents/reviewer.md }
`)
	r.write("extras/schemas/tdd/schema.yaml", "name: tdd\n")
	r.write("extras/schemas/tdd/templates/design.md", "# design\n")
	r.write("extras/agents/reviewer.md", "# reviewer\n")
}

// consumer is a repository graft runs in, with a fetch cache of its own. It holds no
// graft.toml until a test writes one or an add creates it.
type consumer struct {
	t     *testing.T
	dir   string
	cache string
}

func newConsumer(t *testing.T) *consumer {
	t.Helper()
	return &consumer{t: t, dir: t.TempDir(), cache: t.TempDir()}
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

// request is the add a test performs, with the consumer's own roots filled in.
func (c *consumer) request(r *repo, rev string, install ...string) add.Request {
	git := r.dir
	if rev != "" {
		git += "@" + rev
	}
	value, pinned, _ := add.SplitSource(git)
	return add.Request{
		Root:      c.dir,
		CacheRoot: c.cache,
		Git:       value,
		Rev:       pinned,
		Install:   install,
	}
}

// manifestFor is a hand-written graft.toml declaring the fixture, in the shape a consumer
// would have written it.
func manifestFor(r *repo, rev string, install ...string) string {
	quoted := make([]string, 0, len(install))
	for _, s := range install {
		quoted = append(quoted, fmt.Sprintf("%q", s))
	}
	return fmt.Sprintf("[sources.shared]\ngit     = %q\nrev     = %q\ninstall = [%s]\n",
		r.dir, rev, strings.Join(quoted, ", "))
}
