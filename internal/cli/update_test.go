package cli_test

import (
	"strings"
	"testing"

	"github.com/optioni/graft/internal/cli"
)

// `graft update` takes at most one positional argument — the source to move. Everything
// below is refused before internal/sync is reached, which is what lets these run in-process
// without a working directory of their own: an argument the surface rejects never becomes a
// run against anything.

func TestUpdateTakesAtMostOneArgument(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := run(t, cli.Options{Args: []string{"update", "shared", "extra"}})

	if want := "graft: unknown argument \"extra\"\n" + hint + "\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

// update has two flags and no others. --force in particular is the one a user reaches for
// when a run refuses, and there is nothing for it to force.
func TestUpdateRejectsAnUnknownFlag(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := run(t, cli.Options{Args: []string{"update", "--force"}})

	if want := "graft: unknown flag: --force\n" + hint + "\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

// --to writes one source's pin into graft.toml, so it has nothing to mean without a source
// to write it for. The refusal is the command surface's rather than internal/sync's: the
// invocation is malformed, which is what earns the hint line.
func TestUpdateToWithoutASourceIsAUsageError(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := run(t, cli.Options{Args: []string{"update", "--to", "v1.1.0"}})

	if want := "graft: --to requires a source\n" + hint + "\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

// An explicitly empty --to is told apart from an absent one through Flags().Changed, not
// through the value: "" is also what an unset flag holds, and `graft update --to= shared`
// asks for a pin that is not a rev rather than for no pin move at all.
func TestUpdateToWithAnEmptyRevIsAUsageError(t *testing.T) {
	t.Parallel()

	for name, args := range map[string][]string{
		"separate": {"update", "--to", "", "shared"},
		"joined":   {"update", "--to=", "shared"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, code := run(t, cli.Options{Args: args})

			if want := "graft: --to requires a rev\n" + hint + "\n"; stderr != want {
				t.Errorf("stderr = %q, want %q", stderr, want)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
		})
	}
}

// A subcommand's own help is a thing the user asked for, so it goes to stdout and exits 0 —
// the same rule the root's help follows.
func TestUpdateHelpGoesToStandardOutput(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := run(t, cli.Options{Args: []string{"update", "--help"}})

	for _, want := range []string{"update", "--to", "--dry-run"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("`graft update --help` does not mention %q:\n%s", want, stdout)
		}
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

// An empty positional is not the same as no positional. internal/sync reads an empty source
// name as every source, so `graft update "$SOURCE"` with an unset variable would otherwise
// update everything — and `graft update --to v1.1.0 ""` would discard the flag and exit 0
// having moved no pin at all. The sentinel is safe against a manifest, which cannot name a
// source "", and not against a shell.
func TestUpdateRefusesAnEmptySourceName(t *testing.T) {
	t.Parallel()

	for name, args := range map[string][]string{
		"alone":     {"update", ""},
		"with --to": {"update", "--to", "v1.1.0", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, code := run(t, cli.Options{Args: args})

			if want := "graft: source name may not be empty\n" + hint + "\n"; stderr != want {
				t.Errorf("stderr = %q, want %q", stderr, want)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
		})
	}
}
