package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/cli"
)

// The argument surface in process, over buffers, in the pattern sync_test.go and
// update_test.go established. It needs no working directory, because a refused argument
// never reaches the command's RunE.
//
// These cases are also asserted across a real process boundary in list_acceptance_test.go,
// and the redundancy is deliberate: the acceptance tier proves the streams and the exit
// code, and this tier puts the validator itself where the coverage gate can see it — a
// subprocess is executed by CI but not instrumented by it.

// `graft list` takes no positional argument. cobra.NoArgs would say
// `unknown command "shared" for "graft list"`, which is a second error format inside a tool
// whose error format is the contract.
func TestListTakesNoArguments(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := run(t, cli.Options{Args: []string{"list", "shared"}})

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

// The first unrecognised argument is named and the rest are not: a list of everything graft
// did not want says less than the one thing it stopped at.
func TestListNamesOnlyTheFirstArgument(t *testing.T) {
	t.Parallel()

	_, stderr, code := run(t, cli.Options{Args: []string{"list", "shared", "other"}})

	if !strings.Contains(stderr, `"shared"`) {
		t.Errorf("stderr does not name the first argument: %q", stderr)
	}
	if strings.Contains(stderr, "other") {
		t.Errorf("stderr names the second argument too: %q", stderr)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

// --json is the only flag. There is no --format and no second output form: one rendering
// for a person, one for a program, and the second is versioned so it can be extended
// without a third.
func TestListRejectsAnyOtherFlag(t *testing.T) {
	t.Parallel()

	for _, flag := range []string{"--format=yaml", "--source=shared", "--dry-run"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()

			name, _, _ := strings.Cut(flag, "=")
			stdout, stderr, code := run(t, cli.Options{Args: []string{"list", flag}})

			if want := "graft: unknown flag: " + name + "\n" + hint + "\n"; stderr != want {
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

// A command that writes nothing has nothing to offer a dry run.
func TestListHelpNamesOnlyTheFlagItHas(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := run(t, cli.Options{Args: []string{"list", "--help"}})

	for _, want := range []string{"list", "--json"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help does not name %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "--dry-run") {
		t.Errorf("help offers --dry-run on a command that writes nothing:\n%s", stdout)
	}
	if stderr != "" || code != 0 {
		t.Errorf("stderr = %q, code = %d", stderr, code)
	}
}

// The two renderings in process, which is what puts the command's own branches where the
// coverage gate can see them: a subprocess is executed by CI but not instrumented by it.
//
// The working directory is the package's own, which holds no graft.lock — so this is the
// "nothing installed" case, driven through the real command surface. The precondition is
// asserted rather than assumed, because a fixture lock added here later would turn a
// meaningful test into one that silently stopped testing the branch it names.
func TestListInAnUnsyncedDirectory(t *testing.T) {
	t.Parallel()

	if _, err := os.Stat("graft.lock"); !os.IsNotExist(err) {
		t.Skipf("the package directory holds a graft.lock: %v", err)
	}

	t.Run("the plain form notes the absence on stderr", func(t *testing.T) {
		t.Parallel()

		stdout, stderr, code := run(t, cli.Options{Args: []string{"list"}})

		if stderr != "nothing installed\n" {
			t.Errorf("stderr = %q, want %q", stderr, "nothing installed\n")
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want byte-empty", stdout)
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})

	// The document is written whole. A second trailing newline is what a line-oriented
	// printer would add, and it is the failure this asserts against.
	t.Run("the json form still prints a document", func(t *testing.T) {
		t.Parallel()

		stdout, stderr, code := run(t, cli.Options{Args: []string{"list", "--json"}})

		if want := "{\n  \"version\": 1,\n  \"sources\": []\n}\n"; stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
		if stderr != "" || code != 0 {
			t.Errorf("stderr = %q, code = %d", stderr, code)
		}
	})
}
