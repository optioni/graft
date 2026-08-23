package lock

import (
	"fmt"

	"github.com/optioni/graft/internal/manifest"
)

// CheckPins reports whether graft.toml and graft.lock agree on every rev they both
// name. A source present in only one of the two is fine: one with no lock entry is
// resolved on the next sync, and one no longer in the manifest has its files pruned.
//
// A disagreement is an error because sync never re-resolves a pin — it installs what
// the lock says — so a manifest that moved cannot be honoured until `graft update`
// moves the lock with it. Sources are checked in manifest order, which manifest.Parse
// sorts by name, so a lock disagreeing on several sources always names the same one.
func CheckPins(sources []manifest.Source, lk *Lock) error {
	pinned := make(map[string]string, len(lk.Sources))
	for _, s := range lk.Sources {
		pinned[s.Name] = s.Rev
	}
	for _, s := range sources {
		rev, ok := pinned[s.Name]
		if !ok || rev == s.Rev {
			continue
		}
		return fmt.Errorf(
			"graft.toml has rev %q for source %q but graft.lock has %q; run `graft update` to move the pin",
			s.Rev, s.Name, rev,
		)
	}
	return nil
}
