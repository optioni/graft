// Command graft vendors files from a source git repository into the repository it
// runs in. See SPEC.md for the behavior this is working toward.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/optioni/graft/internal/buildinfo"
)

// Injected at build time with -ldflags.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "graft:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		_, err := fmt.Fprintln(out, buildinfo.Format(version, commit, date))
		return err
	}
	return errors.New("not implemented yet — see SPEC.md")
}
