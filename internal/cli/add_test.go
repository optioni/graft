package cli_test

import (
	"io"
	"testing"

	"github.com/optioni/graft/internal/cli"
)

// `graft add` takes one source, any number of selectors, and two flags. Everything below
// is refused before internal/add is reached, which is what lets these run in-process with
// no working directory and no fixture: an invocation the surface rejects never becomes a
// run against anything, and never starts a git process.

func addUsage(t *testing.T, args []string, want string, terminal bool) {
	t.Helper()

	o := cli.Options{Args: args}
	if terminal {
		o.IsTerminal = func(io.Writer) bool { return true }
	}
	stdout, stderr, code := run(t, o)

	if wantErr := "graft: " + want + "\n" + hint + "\n"; stderr != wantErr {
		t.Errorf("stderr = %q, want %q", stderr, wantErr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestAddRequiresASource(t *testing.T) {
	t.Parallel()
	addUsage(t, []string{"add"}, "add requires a source", false)
}

// An empty positional is not "no source": an unset shell variable would otherwise make
// `graft add "$SOURCE" agent:x` derive a name from nothing at all.
func TestAddRefusesAnEmptySource(t *testing.T) {
	t.Parallel()
	addUsage(t, []string{"add", "", "agent:reviewer"}, "source may not be empty", false)
}

func TestAddRefusesAnEmptyRev(t *testing.T) {
	t.Parallel()
	addUsage(t, []string{"add", "optioni/shared@", "agent:reviewer"}, "rev may not be empty", false)
}

func TestAddRefusesAMalformedSelector(t *testing.T) {
	t.Parallel()
	addUsage(t, []string{"add", "optioni/shared", "reviewer"}, `invalid selector "reviewer": want kind:name`, false)
}

// The picker is a later change. Until it exists this refusal is the whole behavior, and
// it is the same on a terminal and off one — never a hang, never a guess.
func TestAddWithoutSelectorsIsRefused(t *testing.T) {
	t.Parallel()

	const want = "add requires at least one selector, or --list to see what the source offers"
	for _, terminal := range []bool{false, true} {
		addUsage(t, []string{"add", "optioni/shared"}, want, terminal)
	}
}

func TestAddRefusesListWithSelectors(t *testing.T) {
	t.Parallel()
	addUsage(t, []string{"add", "optioni/shared", "--list", "agent:reviewer"}, "--list takes no selectors", false)
}

// --list writes nothing, so --no-sync has nothing to suppress. Two flags that contradict
// each other are refused rather than silently ordered.
func TestAddRefusesListWithNoSync(t *testing.T) {
	t.Parallel()
	addUsage(t, []string{"add", "optioni/shared", "--list", "--no-sync"}, "--list and --no-sync cannot be combined", false)
}

// add has two flags and no others. --dry-run in particular is the one a user reaches for,
// and --list is already the form that writes nothing.
func TestAddRejectsAnUnknownFlag(t *testing.T) {
	t.Parallel()
	addUsage(t, []string{"add", "optioni/shared", "--dry-run", "agent:reviewer"}, "unknown flag: --dry-run", false)
}
