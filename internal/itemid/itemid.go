// Package itemid holds the kind:name grammar. An item's identity is kind:name and it
// appears in two consumer-owned files — as a selector in graft.toml and as an item id
// in graft.lock — so the grammar lives in one place while each file keeps its own
// error wording.
package itemid

import "strings"

// Split returns the two halves of s, and whether s is an id at all — exactly one colon
// with a non-empty half on each side. Glob metacharacters are accepted as written;
// matching an id or a selector against a catalog is not this package's job.
//
// An id that does not parse yields two empty strings rather than a partial answer, so a
// caller that forgets to check ok gets nothing rather than half of something.
func Split(s string) (kind, name string, ok bool) {
	kind, name, ok = strings.Cut(s, ":")
	if !ok || kind == "" || name == "" || strings.Contains(name, ":") {
		return "", "", false
	}
	return kind, name, true
}

// Valid reports whether s is syntactically kind:name. It is Split's third result: the
// grammar is stated once, so a rule tightened in one cannot leave the other behind.
func Valid(s string) bool {
	_, _, ok := Split(s)
	return ok
}
