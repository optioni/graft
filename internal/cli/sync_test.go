package cli_test

import (
	"strings"
	"testing"

	"github.com/optioni/graft/internal/cli"
)

// `graft sync` takes no positional argument. cobra.NoArgs would say
// `unknown command "shared" for "graft sync"`, which is a second error format inside a tool
// whose error format is the contract.
func TestSyncTakesNoArguments(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := run(t, cli.Options{Args: []string{"sync", "shared"}})

	if want := "graft: unknown argument \"shared\"\n" + hint + "\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

// There is no --force and no --frozen. Sync always overwrites, and sync is always frozen;
// a flag that made it do its job would be the bug it exists to prevent.
func TestSyncRejectsForceAndFrozen(t *testing.T) {
	t.Parallel()

	for _, flag := range []string{"--force", "--frozen"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, code := run(t, cli.Options{Args: []string{"sync", flag}})

			if want := "graft: unknown flag: " + flag + "\n" + hint + "\n"; stderr != want {
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

func TestHelpListsSync(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := run(t, cli.Options{Args: []string{"--help"}})

	if !strings.Contains(stdout, "sync") {
		t.Errorf("help does not name sync:\n%s", stdout)
	}
	// Neither a help command nor a completion command is offered: SPEC.md's command table
	// names neither, and the flags section's "help for graft" is the --help flag, not one.
	for _, line := range strings.Split(stdout, "\n") {
		for _, absent := range []string{"help ", "completion "} {
			if strings.HasPrefix(strings.TrimSpace(line), absent) {
				t.Errorf("help offers a %qcommand, which SPEC.md's table does not name:\n%s", absent, stdout)
			}
		}
	}
	if stderr != "" || code != 0 {
		t.Errorf("stderr = %q, code = %d", stderr, code)
	}
}

// Registering a subcommand makes cobra install its built-in help command. SPEC.md's command
// table names none, and `--version` already sets the precedent that one spelling is enough.
func TestHelpIsNotACommand(t *testing.T) {
	t.Parallel()

	for name, args := range map[string][]string{
		"alone":         {"help"},
		"with-argument": {"help", "sync"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, code := run(t, cli.Options{Args: args})

			if want := "graft: unknown command \"help\"\n" + hint + "\n"; stderr != want {
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
