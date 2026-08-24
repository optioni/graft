package cli_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/cli"
)

const hint = `run "graft --help" for usage`

func TestMainUnknown(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		args []string
		want string
	}{
		"an unknown command":       {[]string{"frobnicate"}, `graft: unknown command "frobnicate"`},
		"version is not a command": {[]string{"version"}, `graft: unknown command "version"`},
		"no completion command":    {[]string{"completion"}, `graft: unknown command "completion"`},
		// CompletionOptions.DisableDefaultCmd does not remove these: cobra registers them
		// on every Execute, and left alone `graft __complete ''` exits 0 and writes ":0"
		// to stdout, bypassing both the UI and the argument validator.
		"the hidden completion protocol":                  {[]string{"__complete", ""}, `graft: unknown command "__complete"`},
		"the hidden completion protocol, no descriptions": {[]string{"__completeNoDesc", ""}, `graft: unknown command "__completeNoDesc"`},
		"an unknown flag":                                 {[]string{"--nope"}, "graft: unknown flag: --nope"},
		"an unknown shorthand flag":                       {[]string{"-v"}, "graft: unknown shorthand flag: 'v' in -v"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, code := run(t, cli.Options{Args: tc.args})

			if want := tc.want + "\n" + hint + "\n"; stderr != want {
				t.Errorf("stderr = %q, want %q", stderr, want)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			// SilenceUsage: a usage error points at help, it does not print it.
			if strings.Contains(stderr, "Usage:") {
				t.Errorf("stderr carries the usage block:\n%s", stderr)
			}
		})
	}
}

func TestMainUnknownNamesOnlyTheFirst(t *testing.T) {
	t.Parallel()

	_, stderr, _ := run(t, cli.Options{Args: []string{"frobnicate", "wibble"}})

	if !strings.Contains(stderr, "frobnicate") {
		t.Errorf("stderr does not name the first argument: %q", stderr)
	}
	if strings.Contains(stderr, "wibble") {
		t.Errorf("stderr names an argument after the first: %q", stderr)
	}
}

func TestMainWriteFailure(t *testing.T) {
	t.Parallel()

	// Without the recording writers cobra is given, --help and a bare invocation both
	// exit 0 here with nothing on either stream: cobra.Command.Help() returns nil
	// unconditionally and discards its renderer's error.
	for name, args := range map[string][]string{
		"version":      {"--version"},
		"help":         {"--help"},
		"no arguments": nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			code := cli.Main(cli.Options{
				Args:       args,
				Stdout:     failingWriter{errors.New("disk full")},
				Stderr:     &stderr,
				Getenv:     func(string) string { return "" },
				IsTerminal: func(io.Writer) bool { return false },
			})

			if want := "graft: cannot write output: disk full\n"; stderr.String() != want {
				t.Errorf("stderr = %q, want %q", stderr.String(), want)
			}
			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
		})
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestMainColorFollowsStdout(t *testing.T) {
	t.Parallel()

	// An identity assertion rather than an output one: nothing in this change emits styled
	// output through Main, so the only observable fact is which stream the decision was
	// taken from. This goes red if the wiring is inverted.
	var stdout, stderr bytes.Buffer
	var asked []io.Writer

	cli.Main(cli.Options{
		Args:       []string{"--version"},
		Stdout:     &stdout,
		Stderr:     &stderr,
		Getenv:     func(string) string { return "" },
		IsTerminal: func(w io.Writer) bool { asked = append(asked, w); return false },
	})

	if len(asked) != 1 {
		t.Fatalf("the terminal test was consulted %d times, want exactly 1", len(asked))
	}
	if asked[0] != io.Writer(&stdout) {
		t.Error("the terminal test was not asked about the stdout stream")
	}
	for _, w := range asked {
		if w == io.Writer(&stderr) {
			t.Error("the terminal test was asked about the stderr stream")
		}
	}
}
