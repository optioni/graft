package catalog

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/optioni/graft/internal/itemid"
)

// parseItems reads the provides list. An entry that has not earned an identity yet is
// reported by its 0-based position; once kind and name are known the item is named by
// its id, which is what a consumer's selector and the lock both use.
func parseItems(raw map[string]any, kinds map[string]Kind, filename string) ([]Item, error) {
	v, ok := raw["provides"]
	if !ok || v == nil {
		return nil, nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, errf(filename, "provides must be a list")
	}

	items := make([]Item, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for i, e := range list {
		at := func(format string, args ...any) error {
			return errf(filename, "provides[%d]: %s", i, fmt.Sprintf(format, args...))
		}
		body, ok := e.(map[string]any)
		if !ok {
			return nil, at("must be a mapping")
		}

		kind, err := field(body, "kind", at)
		if err != nil {
			return nil, err
		}
		name, err := field(body, "name", at)
		if err != nil {
			return nil, err
		}
		from, err := field(body, "from", at)
		if err != nil {
			return nil, err
		}

		// The kind:name grammar lives in internal/itemid because graft.toml
		// selectors, graft.lock ids, and catalog items all have to agree on it — a
		// second grammar here would let a selector silently fail to match an id the
		// lock happily records. The two checks after it are not a second grammar but a
		// producer-side restriction: itemid says what an id is, and a catalog is the
		// only thing that ever mints one, so it is the right place to refuse the names
		// that would misbehave downstream. See plainName.
		id := kind + ":" + name
		if !itemid.Valid(id) {
			return nil, at("invalid item id %q: want kind:name", id)
		}
		if name == "." || name == ".." {
			return nil, at(`invalid name %q: a name may not be "." or ".."`, name)
		}
		if !plainName(name) {
			return nil, at("invalid name %q: want letters, digits, dot, dash, or underscore", name)
		}
		item := func(format string, args ...any) error {
			return errf(filename, "item %q: %s", id, fmt.Sprintf(format, args...))
		}

		if _, declared := kinds[kind]; !declared {
			// The reverse — a kind declared and never provided — is fine: a source
			// may declare hook and ship no hooks yet. An item without a kind has no
			// destination and could never be installed.
			return nil, item("kind %q is not declared", kind)
		}
		if _, dup := seen[id]; dup {
			return nil, errf(filename, "duplicate item %q", id)
		}
		if !inSource(from) {
			return nil, item("from %q is not a relative path inside the source", from)
		}
		seen[id] = struct{}{}
		items = append(items, Item{ID: id, Kind: kind, Name: name, From: from})
	}

	// Byte-wise, so the order cannot depend on map iteration, locale, or platform.
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

// plainName reports whether an item name is a single path segment of ordinary
// characters. internal/itemid says what an id *is*; this says what a source may
// actually publish, the same way inSource below constrains from rather than moving a
// rule into a shared package. Three things break without it, none of them noisily:
//
//   - path.Match's "*" does not cross "/", so an item named nested/thing is invisible
//     to a kind:* selector while that selector still matches its siblings. The install
//     silently drops an item and the no-match guard never fires, because the selector
//     did match something.
//   - A name holding a glob metacharacter makes a consumer's exact selector match a
//     neighbour — agent:a*b also selects agent:ab — or makes the item unselectable at
//     all, since a name of [x] comes back as an invalid pattern.
//   - {name} is interpolated into a destination by a later package, so a name of ".."
//     aims a write at a directory the catalog's own `to` never named. SPEC.md's one
//     mitigation for an untrusted source is that the destination is shown before
//     install; a name that rewrites it defeats exactly that.
//
// The set is deliberately narrow. It covers every name in SPEC.md's examples and can
// be widened later without invalidating a catalog that already parses.
func plainName(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

// inSource reports whether p names something inside the source tree. SPEC.md makes
// "from stays inside the source tree" an invariant, and it is checked here rather than
// where the path is joined to a real directory, because a rule enforced after the join
// is a rule enforced too late.
//
// ".." and a leading "/" are the obvious escapes. Two quieter cases need the same
// answer: "." names the whole source tree rather than a thing it provides, and an
// uncleaned alias like "./extras/tdd" or "extras/tdd/" names the same path as the
// cleaned form while looking different to every later comparison.
func inSource(p string) bool {
	if p == "" || p == "." || strings.HasPrefix(p, "/") {
		return false
	}
	if slices.Contains(strings.Split(p, "/"), "..") {
		return false
	}
	return path.Clean(p) == p
}
