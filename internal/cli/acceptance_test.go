package cli_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The acceptance tier: the real binary, real arguments, real pipes, and a real exit
// status. It lives here rather than beside main.go because a test in cmd/graft would
// never run in CI — task ci is lint, cover, and build, and cover runs go test over
// ./internal/... only. cmd/graft is the one file no other test can reach.

type result struct {
	stdout string
	stderr string
	code   int
}

// buildGraft compiles cmd/graft into a directory the calling test owns. It is called
// once by the parent test rather than once per case: a t.TempDir() is removed when its
// own test ends, so a build shared through package state would hand later tests a path
// that no longer exists.
func buildGraft(t *testing.T) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "graft")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/optioni/graft/cmd/graft")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build cmd/graft: %v\n%s", err, stderr.String())
	}
	return bin
}

// runGraft runs the compiled binary in dir, capturing the two streams separately so the
// split is asserted across a real process boundary rather than across two buffers a test
// wired up itself.
func runGraft(t *testing.T, bin, dir string, args ...string) result {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
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

func TestGraftBinary(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)

	t.Run("no arguments prints help and succeeds", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		got := runGraft(t, bin, dir)

		if got.code != 0 {
			t.Errorf("exit code = %d, want 0", got.code)
		}
		if got.stderr != "" {
			t.Errorf("stderr = %q, want empty", got.stderr)
		}
		for _, want := range []string{"graft", "Usage:", "--version", "--help"} {
			if !strings.Contains(got.stdout, want) {
				t.Errorf("stdout does not mention %q:\n%s", want, got.stdout)
			}
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("reading the working directory: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("the working directory is not empty: %v", entries)
		}
	})
}
