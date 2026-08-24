package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/optioni/graft/internal/ui"
)

// In-package, because registering a command against a real root is the only way to
// exercise routing, a command's error, and a command's stdout before `sync` exists.

func TestMainCommandError(t *testing.T) {
	t.Parallel()

	const message = `source "shared": rev "v9.9.9" not found`

	var stdout, stderr bytes.Buffer
	u := ui.New(&stdout, &stderr, false)
	root := newRoot(u, Options{})
	root.AddCommand(&cobra.Command{
		Use:  "boom",
		RunE: func(*cobra.Command, []string) error { return errors.New(message) },
	})
	root.SetArgs([]string{"boom"})

	code := execute(u, root)

	if want := "graft: " + message + "\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
	// Not a usage error, so no hint: a marker applied to everything would fail here.
	if strings.Contains(stderr.String(), "--help") {
		t.Errorf("stderr carries a usage hint for a plain failure:\n%s", stderr.String())
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestMainCommandOutput(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	u := ui.New(&stdout, &stderr, false)
	root := newRoot(u, Options{})
	root.AddCommand(&cobra.Command{
		Use: "speak",
		RunE: func(*cobra.Command, []string) error {
			u.Print("hello")
			return nil
		},
	})
	root.SetArgs([]string{"speak"})

	code := execute(u, root)

	if want := "hello\n"; stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

// A registered subcommand must not disturb the refusal of an unrecognised one: the root's
// Args validator is consulted only when nothing matched, which is why it keeps working
// once `sync` arrives.
func TestUnknownCommandBesideARegisteredOne(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	u := ui.New(&stdout, &stderr, false)
	root := newRoot(u, Options{})
	root.AddCommand(&cobra.Command{Use: "speak", RunE: func(*cobra.Command, []string) error { return nil }})
	root.SetArgs([]string{"frobnicate"})

	if code := execute(u, root); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if want := "graft: unknown command \"frobnicate\"\n" + hintLine + "\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}
