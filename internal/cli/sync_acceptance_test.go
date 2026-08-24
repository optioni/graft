package cli_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The outer loop for `graft sync`: the real binary, a real fixture repository, a real
// working directory, and a real fetch cache, with nothing replaced. It is the one test
// that can fail for a reason none of the unit or integration tests can — a plan applied
// against the wrong root, a report on the wrong stream, a cache root read from a global.
//
// The subprocess carries its own working directory and its own environment, so no test
// here calls t.Chdir or t.Setenv: both are process-global, and the process running the
// tests is not the process under test.

// runGraftIn runs the compiled binary in dir with extra environment entries appended to
// the parent's. XDG_CACHE_HOME is what keeps a test off the developer's real cache, and
// source.DefaultCacheRoot honours it only when it is absolute — t.TempDir() is, and that
// is load-bearing rather than incidental.
func runGraftIn(t *testing.T, bin, dir string, env []string, args ...string) result {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("running %s %v: %v", bin, args, err)
		}
		code = exit.ExitCode()
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// sourceRepo is a fixture source repository: a real git repository holding a real
// catalog.yaml. user.name and user.email are set on the repository rather than the
// machine, or committing fails on a clean CI runner where no global identity exists.
type sourceRepo struct {
	t   *testing.T
	dir string
}

func newSourceRepo(t *testing.T) *sourceRepo {
	t.Helper()
	r := &sourceRepo{t: t, dir: t.TempDir()}
	r.git("-c", "init.defaultBranch=main", "init", "-q", r.dir)
	r.git("-C", r.dir, "config", "user.name", "graft fixture")
	r.git("-C", r.dir, "config", "user.email", "fixture@graft.test")
	return r
}

func (r *sourceRepo) git(args ...string) string {
	r.t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *sourceRepo) write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatalf("MkdirAll %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatalf("WriteFile %s: %v", full, err)
	}
}

func (r *sourceRepo) commit(message string) string {
	r.t.Helper()
	r.git("-C", r.dir, "add", "-A")
	r.git("-C", r.dir, "commit", "-q", "-m", message)
	return r.git("-C", r.dir, "rev-parse", "HEAD")
}

func (r *sourceRepo) tag(name string) { r.git("-C", r.dir, "tag", name) }

// seedCatalog writes the fixture's offer: one directory item and one file item, under
// the two kinds SPEC.md's own examples use.
func (r *sourceRepo) seedCatalog() {
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

// consumer is a repository graft runs in: a directory holding graft.toml and nothing
// else until a sync puts something there.
type consumer struct {
	t   *testing.T
	dir string
	env []string
}

func newConsumer(t *testing.T, manifest string) *consumer {
	t.Helper()
	c := &consumer{t: t, dir: t.TempDir()}
	c.env = []string{"XDG_CACHE_HOME=" + t.TempDir()}
	c.writeFile("graft.toml", manifest)
	return c
}

func (c *consumer) writeFile(path, content string) {
	c.t.Helper()
	full := filepath.Join(c.dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		c.t.Fatalf("MkdirAll %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		c.t.Fatalf("WriteFile %s: %v", full, err)
	}
}

// read returns a file's contents, failing the test if it is not there.
func (c *consumer) read(path string) string {
	c.t.Helper()
	data, err := os.ReadFile(filepath.Join(c.dir, filepath.FromSlash(path)))
	if err != nil {
		c.t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

// manifestFor is the consumer's request against a fixture repository. The git value is
// a filesystem path, which reaches the same code as a remote URL without a network.
func manifestFor(repo *sourceRepo, rev string, install ...string) string {
	quoted := make([]string, 0, len(install))
	for _, s := range install {
		quoted = append(quoted, fmt.Sprintf("%q", s))
	}
	return fmt.Sprintf(`[sources.shared]
git     = %q
rev     = %q
install = [%s]
`, repo.dir, rev, strings.Join(quoted, ", "))
}

func TestGraftSyncInstallsWhatTheManifestAsksFor(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	sha := repo.commit("v1")
	repo.tag("v1.0.0")

	c := newConsumer(t, manifestFor(repo, "v1.0.0", "schema:tdd", "agent:*"))

	got := runGraftIn(t, bin, c.dir, c.env, "sync")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty: a sync's report is a summary, and summaries go to stderr", got.stdout)
	}
	if got.stderr == "" {
		t.Error("stderr is empty: a first sync has something to report")
	}

	for path, want := range map[string]string{
		"openspec/schemas/tdd/schema.yaml":         "name: tdd\n",
		"openspec/schemas/tdd/templates/design.md": "# design\n",
		".claude/agents/reviewer.md":               "# reviewer\n",
	} {
		if got := c.read(path); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}

	lock := c.read("graft.lock")
	for _, want := range []string{
		`name     = "shared"`,
		`rev      = "v1.0.0"`,
		fmt.Sprintf(`resolved = %q`, sha),
		`id    = "agent:reviewer"`,
		`id    = "schema:tdd"`,
		`".claude/agents/reviewer.md"`,
		`"openspec/schemas/tdd/schema.yaml"`,
		`"openspec/schemas/tdd/templates/design.md"`,
	} {
		if !strings.Contains(lock, want) {
			t.Errorf("graft.lock does not contain %s:\n%s", want, lock)
		}
	}
}
