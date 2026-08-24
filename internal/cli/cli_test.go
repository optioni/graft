package cli_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/buildinfo"
	"github.com/optioni/graft/internal/cli"
)

// run drives Main with buffers for both streams and stubs for everything it would
// otherwise read from the process, so no test depends on the developer's environment.
func run(t *testing.T, o cli.Options) (stdout, stderr string, code int) {
	t.Helper()

	var out, errs bytes.Buffer
	o.Stdout, o.Stderr = &out, &errs
	if o.Getenv == nil {
		o.Getenv = func(string) string { return "" }
	}
	if o.IsTerminal == nil {
		o.IsTerminal = func(io.Writer) bool { return false }
	}
	// Main first: return operands are evaluated left to right, so reading the buffers in
	// the return statement would read them before anything wrote to them.
	code = cli.Main(o)
	return out.String(), errs.String(), code
}

func TestMainVersion(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		version, commit, date string
	}{
		"a released build":  {"v1.2.0", "abc1234", "2026-08-23"},
		"an unbuilt binary": {"", "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, code := run(t, cli.Options{
				Args:    []string{"--version"},
				Version: tc.version,
				Commit:  tc.commit,
				Date:    tc.date,
			})

			// Built by calling buildinfo.Format rather than by restating it, so the two
			// cannot drift apart.
			want := buildinfo.Format(tc.version, tc.commit, tc.date) + "\n"
			if stdout != want {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
		})
	}
}

func TestMainHelp(t *testing.T) {
	t.Parallel()

	bare, bareErr, bareCode := run(t, cli.Options{Args: nil})
	flag, flagErr, flagCode := run(t, cli.Options{Args: []string{"--help"}})

	for _, c := range []struct {
		name   string
		stderr string
		code   int
	}{{"no arguments", bareErr, bareCode}, {"--help", flagErr, flagCode}} {
		if c.stderr != "" {
			t.Errorf("%s: stderr = %q, want empty", c.name, c.stderr)
		}
		if c.code != 0 {
			t.Errorf("%s: exit code = %d, want 0", c.name, c.code)
		}
	}

	// Substrings, never a golden file: every later change adds a command, and a golden
	// that churns on every change is a golden nobody reads.
	for _, want := range []string{"graft", "Usage:", "--version", "--help"} {
		if !strings.Contains(bare, want) {
			t.Errorf("help does not mention %q:\n%s", want, bare)
		}
	}

	// A relation between two outputs, which does not churn.
	if bare != flag {
		t.Errorf("`graft` and `graft --help` differ:\n%q\n%q", bare, flag)
	}
}
