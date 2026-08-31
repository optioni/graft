package cli_test

import (
	"testing"

	"github.com/optioni/graft/internal/cli"
)

// `graft add` takes one source, any number of selectors, and two flags. Everything below
// is refused before internal/add is reached, which is what lets these run in-process with
// no working directory and no fixture: an invocation the surface rejects never becomes a
// run against anything, and never starts a git process.

func addUsage(t *testing.T, args []string, want string, interactive bool) {
	t.Helper()

	o := cli.Options{Args: args, Interactive: func() bool { return interactive }}
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

// Off a terminal there is nobody to ask, and graft never guesses a set of items to
// install. The refusal names what it needed rather than hanging on a pipe.
func TestAddWithoutSelectorsIsRefusedWithNoTerminal(t *testing.T) {
	t.Parallel()

	addUsage(t, []string{"add", "optioni/shared"},
		"add requires at least one selector, or --list to see what the source offers", false)
}

// On a terminal the same invocation is allowed through to the picker. It is asserted by
// what fails instead: the run reaches name derivation, which is past the validator, and
// fails there without a network call or a picker being drawn.
func TestAddWithoutSelectorsIsAllowedThroughOnATerminal(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := run(t, cli.Options{
		Args:        []string{"add", "optioni/sh ared"},
		Interactive: func() bool { return true },
	})

	if want := "graft: cannot derive a source name from \"optioni/sh ared\"\n"; stderr != want {
		t.Errorf("stderr = %q, want %q — the no-selector refusal must not have fired", stderr, want)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
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
