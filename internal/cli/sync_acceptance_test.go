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

func newSourceRepo(t *testing.T) *sourceRepo { return newNamedSourceRepo(t, "shared") }

// newNamedSourceRepo builds a fixture whose directory carries a chosen name. The name is
// not decoration: `graft add` derives a source name from the git value's last path
// segment, so the fixture's own directory name is what the manifest it writes is keyed on,
// and two fixtures in one test need two names.
func newNamedSourceRepo(t *testing.T, name string) *sourceRepo {
	t.Helper()
	r := &sourceRepo{t: t, dir: filepath.Join(t.TempDir(), name)}
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

// removeDir deletes the whole source repository, so a later command that still tries to
// reach it fails rather than silently succeeding — the only proof that it never tried.
func (r *sourceRepo) removeDir() {
	r.t.Helper()
	if err := os.RemoveAll(r.dir); err != nil {
		r.t.Fatalf("removing the source repository: %v", err)
	}
}

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

// The report is a summary, and SPEC.md sends summaries to the error stream so a pipe
// carries only what a program can consume. A sync writes nothing to standard output on any
// path, which is asserted here across a real process boundary rather than across two
// buffers a test wired up itself.
func TestGraftSyncReportGoesToStderr(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.0.0")

	c := newConsumer(t, manifestFor(repo, "v1.0.0", "schema:tdd", "agent:*"))

	first := runGraftIn(t, bin, c.dir, c.env, "sync")
	if first.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", first.code, first.stderr)
	}
	if first.stdout != "" {
		t.Errorf("stdout = %q, want empty", first.stdout)
	}
	for _, want := range []string{"added  agent:reviewer", "added  schema:tdd", "3 files written, 0 removed"} {
		if !strings.Contains(first.stderr, want) {
			t.Errorf("the report does not contain %q:\n%s", want, first.stderr)
		}
	}
	// A pipe is not a terminal, so colour is dropped and the report is plain text.
	if strings.ContainsRune(first.stderr, '\x1b') {
		t.Errorf("the report carries colour on a pipe:\n%q", first.stderr)
	}

	// A sync with nothing to do says so, and says nothing else.
	second := runGraftIn(t, bin, c.dir, c.env, "sync")
	if second.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", second.code, second.stderr)
	}
	if second.stderr != "up to date\n" {
		t.Errorf("stderr = %q, want %q", second.stderr, "up to date\n")
	}
	if second.stdout != "" {
		t.Errorf("stdout = %q, want empty", second.stdout)
	}
}

// A dropped item is reported with the note that says why it went, and --dry-run reports the
// same thing while changing nothing.
func TestGraftSyncReportsRemovals(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.0.0")

	c := newConsumer(t, manifestFor(repo, "v1.0.0", "schema:tdd", "agent:*"))
	if got := runGraftIn(t, bin, c.dir, c.env, "sync"); got.code != 0 {
		t.Fatalf("first sync: %s", got.stderr)
	}

	c.writeFile("graft.toml", manifestFor(repo, "v1.0.0", "schema:tdd"))

	dry := runGraftIn(t, bin, c.dir, c.env, "sync", "--dry-run")
	if dry.code != 0 {
		t.Fatalf("dry run: exit %d\n%s", dry.code, dry.stderr)
	}
	for _, want := range []string{"removed  agent:reviewer", "no longer installed", "to remove - nothing written"} {
		if !strings.Contains(dry.stderr, want) {
			t.Errorf("the dry-run report does not contain %q:\n%s", want, dry.stderr)
		}
	}
	if c.read(".claude/agents/reviewer.md") != "# reviewer\n" {
		t.Error("the dry run deleted the file it reported")
	}

	real := runGraftIn(t, bin, c.dir, c.env, "sync")
	if real.code != 0 {
		t.Fatalf("sync: exit %d\n%s", real.code, real.stderr)
	}
	if !strings.Contains(real.stderr, "review with `git diff`") {
		t.Errorf("the report does not point at git diff:\n%s", real.stderr)
	}
	if _, err := os.Stat(filepath.Join(c.dir, ".claude")); err == nil {
		t.Error(".claude/ was left behind after its only file was pruned")
	}
}

// A sync that fails writes nothing to standard output, so a caller piping graft never has
// to tell a report from an error.
func TestGraftSyncStdoutEmptyOnFailure(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	dir := t.TempDir()

	got := runGraftIn(t, bin, dir, []string{"XDG_CACHE_HOME=" + t.TempDir()}, "sync")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
	if want := "graft: graft.toml not found\n"; got.stderr != want {
		t.Errorf("stderr = %q, want %q", got.stderr, want)
	}
}
