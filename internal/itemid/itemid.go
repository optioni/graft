// Package itemid holds the kind:name grammar. An item's identity is kind:name and it
// appears in two consumer-owned files — as a selector in graft.toml and as an item id
// in graft.lock — so the grammar lives in one place while each file keeps its own
// error wording.
package itemid

import "strings"

// Valid reports whether s is syntactically kind:name — exactly one colon with a
// non-empty half on each side. Glob metacharacters are accepted as written; matching
// an id or a selector against a catalog is not this package's job.
func Valid(s string) bool {
	kind, name, ok := strings.Cut(s, ":")
	return ok && kind != "" && name != "" && !strings.Contains(name, ":")
}
