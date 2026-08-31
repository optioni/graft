package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The outer loop for `graft add`: the real binary, a real fixture repository, and a
// working directory holding neither graft.toml nor graft.lock. `add` is the one command
// that creates the manifest, so the state it starts from is the state no other
// acceptance test here can produce.

// newBareConsumer is a repository graft runs in that holds nothing at all — no
// graft.toml, no graft.lock. Every other command fails here; `add` is the answer to that.
func newBareConsumer(t *testing.T) *consumer {
	t.Helper()
	return &consumer{t: t, dir: t.TempDir(), env: []string{"XDG_CACHE_HOME=" + t.TempDir()}}
}

// exists reports whether a path is there at all, which is how "nothing was written" is
// asserted: read would fail the test, and absence is the expected outcome.
func (c *consumer) exists(path string) bool {
	c.t.Helper()
	_, err := os.Lstat(filepath.Join(c.dir, filepath.FromSlash(path)))
	return err == nil
}

func TestGraftAddDeclaresASourceAndSyncsIt(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	sha := repo.commit("v1")
	repo.tag("v1.0.0")

	c := newBareConsumer(t)

	got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir+"@v1.0.0", "agent:reviewer")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty: add's report is a summary, and summaries go to stderr", got.stdout)
	}

	if got := c.read(".claude/agents/reviewer.md"); got != "# reviewer\n" {
		t.Errorf(".claude/agents/reviewer.md = %q, want %q", got, "# reviewer\n")
	}

	manifest := c.read("graft.toml")
	for _, want := range []string{
		"[sources.shared]",
		`git     = "` + repo.dir + `"`,
		`rev     = "v1.0.0"`,
		`install = ["agent:reviewer"]`,
	} {
		if !strings.Contains(manifest, want) {
			t.Errorf("graft.toml does not contain %s:\n%s", want, manifest)
		}
	}

	lock := c.read("graft.lock")
	for _, want := range []string{
		`name     = "shared"`,
		`rev      = "v1.0.0"`,
		`resolved = "` + sha + `"`,
		`id    = "agent:reviewer"`,
		`".claude/agents/reviewer.md"`,
	} {
		if !strings.Contains(lock, want) {
			t.Errorf("graft.lock does not contain %s:\n%s", want, lock)
		}
	}

	if !strings.Contains(got.stderr, `graft.toml: added source "shared" at v1.0.0`) {
		t.Errorf("the report does not name the manifest edit:\n%s", got.stderr)
	}
}
