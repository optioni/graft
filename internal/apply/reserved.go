package apply

import (
	"fmt"
	"path"
	"strings"

	"github.com/optioni/graft/internal/lock"
	"github.com/optioni/graft/internal/manifest"
)

// gitDir is the one directory name graft treats as off limits. The rule is on a path's
// first segment, never a string prefix: ".github/workflows/" and ".gitignore" are ordinary
// destinations, and placing a workflow file is something ENGINEERING.md's security note
// accepts and names.
const gitDir = ".git"

// checkReserved refuses a path graft may not touch, whichever verb is being applied to it.
//
// The floor is here rather than in internal/plan for two reasons. It holds whichever
// planner produced the plan — the same argument the os.Root already makes — and the rule it
// enforces is not the one internal/plan enforces: internal/plan refuses a destination that
// *escapes* the repository root, and .git/config does not escape it. Kinds are arbitrary
// and no rule anywhere constrains what a `to` may name, so without this nothing at all
// stands between a catalog and the consumer's git configuration.
//
// That matters more than it looks. SPEC.md offers exactly one mitigation for an untrusted
// source — every sync's effect is a git diff — and a file inside .git/ is invisible to it,
// being untracked. .git/config alone turns placing a file into running a program on the
// user's next git command, through core.fsmonitor, core.sshCommand, or an alias, which is
// the opposite of the claim that graft executes nothing a source provides.
//
// graft.toml is the consumer's own request and graft.lock is graft's own record. An item
// placed at either would be destroyed by the run that installed it, and the lock would then
// claim a file a later prune would delete.
func checkReserved(verb, over, p string) error {
	if first, _, _ := strings.Cut(path.Clean(p), "/"); first == gitDir {
		return fmt.Errorf("cannot %s %q: graft never %ss inside %q", verb, p, verb, gitDir)
	}
	for _, own := range [...]string{manifest.Filename, lock.Filename} {
		if path.Clean(p) == own {
			return fmt.Errorf("cannot %s %q: graft never %ss %s%q", verb, p, verb, over, own)
		}
	}
	return nil
}

// checkReservedWrite and checkReservedRemove pin the two wordings. The verb reads in the
// message as well as naming the operation, and only the write side says "over" — graft
// writes *over* a file it may not replace, and removes one it may not delete.
func checkReservedWrite(p string) error  { return checkReserved("write", "over ", p) }
func checkReservedRemove(p string) error { return checkReserved("remove", "", p) }
