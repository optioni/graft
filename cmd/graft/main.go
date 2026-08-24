// Command graft vendors files from a source git repository into the repository it
// runs in. See SPEC.md for the behavior this is working toward.
//
// This file is wiring and an exit, and deliberately nothing else. Taskfile.yml's ci task
// runs lint, cover, and build, and only cover runs go test — over ./internal/... So a
// decision made here is not merely invisible to the coverage gate, it is never executed by
// CI at all. internal/cli.Main returns the exit code for that reason.
package main

import (
	"os"

	"github.com/optioni/graft/internal/cli"
)

// Injected at build time with -ldflags.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	os.Exit(cli.Main(cli.Options{
		Args:    os.Args[1:],
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: version,
		Commit:  commit,
		Date:    date,
	}))
}
