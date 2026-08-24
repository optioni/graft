package cli_test

import (
	"bytes"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestCmdGraftImports is a test rather than a command someone runs and records, because the
// regression it guards is a later change adding an import to main.go — in the one directory
// CI never runs a test over.
func TestCmdGraftImports(t *testing.T) {
	t.Parallel()

	t.Run("main imports only os and the command surface", func(t *testing.T) {
		t.Parallel()

		// buildinfo must not appear: the build strings travel to internal/cli as strings and
		// buildinfo.Format is called where the coverage gate can see it, so an import of it
		// here is exactly the failure.
		want := []string{"github.com/optioni/graft/internal/cli", "os"}
		got := goList(t, "-f", `{{join .Imports "\n"}}`, "../../cmd/graft")

		if !slices.Equal(got, want) {
			t.Errorf("imports of cmd/graft = %v, want %v", got, want)
		}
	})

	t.Run("cmd holds exactly one package", func(t *testing.T) {
		t.Parallel()

		got := goList(t, "../../cmd/...")
		if want := []string{"github.com/optioni/graft/cmd/graft"}; !slices.Equal(got, want) {
			t.Errorf("packages under cmd/ = %v, want %v", got, want)
		}
	})
}

func goList(t *testing.T, args ...string) []string {
	t.Helper()

	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list %v: %v\n%s", args, err, stderr.String())
	}

	var out []string
	for line := range strings.SplitSeq(strings.TrimSpace(stdout.String()), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
