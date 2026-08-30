// Package rev holds the one predicate that decides whether a graft.toml rev is a semver
// range or a ref: a tag, a branch, or a full sha. It is a leaf package — it imports
// nothing of graft's own — because both internal/lock and internal/source must ask it
// and may never disagree. The obvious home for this predicate is internal/source, and it
// does not compile there: internal/source/listing.go imports internal/plan, and
// internal/plan/build.go imports internal/lock, so internal/lock importing
// internal/source would close a cycle. A leaf package is the only shape that lets one
// definition serve both, following internal/itemid, which internal/lock already imports
// for exactly this reason.
package rev

import "strings"

// IsRange reports whether rev is a semver range rather than a ref, decided from the
// string alone — no network call and no filesystem access. It is the only definition of
// this predicate in the whole program: internal/source asks it before resolving, and
// internal/lock asks it when validating a lock's matched key.
//
// rev is a range when, and only when, one of the following holds:
//
//   - its first character is one of ^ ~ > < =
//   - it contains an ASCII space
//   - it contains ||
//   - it is exactly *
//
// Every other rev is a ref. The rule is syntactic rather than a lookup because a lookup
// would make a pin's meaning depend on what the remote happens to contain: a rev that is
// a ref today and a range tomorrow is a pin that silently changed what it asks for.
//
// Most of what the rule claims costs nothing: ^ and ~ as leading characters, and * and
// the space wherever they appear, are already illegal in a git ref name, so claiming
// them gives up nothing that could have been a tag. >, <, and = are legal in a ref name
// and are claimed deliberately — a tag named ">=1.2.0" becomes unreachable as a pin, a
// cost paid knowingly against a case that does not occur. A bare x-range like "1.x" is
// deliberately left a ref: it is a plausible branch name, and a rule with an ambiguous
// case is a rule that silently picks wrong.
func IsRange(rev string) bool {
	if rev == "" {
		return false
	}
	switch rev[0] {
	case '^', '~', '>', '<', '=':
		return true
	}
	return rev == "*" || strings.Contains(rev, " ") || strings.Contains(rev, "||")
}
