package catalog

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Expand matches a source's install selectors against what its catalog provides and
// returns the deduplicated union, ordered by item id. It is a pure function of a
// catalog, a source name, and a selector list: it reads no file, runs no command, and
// creates, modifies, or deletes nothing.
//
// source names the [sources.<name>] block the selectors came from, and is carried only
// so the error can locate the problem. Taking a name rather than a manifest.Source is
// deliberate: a catalog describes a source repository's offer and has no business
// knowing the consumer's file format.
//
// Every selector is checked. A union check would let agent:* cover for a misspelled
// schema:tdd-workflwo, and typo protection is the point of the rule.
func Expand(c *Catalog, source string, selectors []string) ([]Item, error) {
	var out []Item
	seen := make(map[string]struct{}, len(c.items()))
	for _, sel := range selectors {
		match, ok := matcher(sel)
		if !ok {
			return nil, fmt.Errorf("source %q: invalid selector pattern %q", source, sel)
		}
		matched := false
		for _, it := range c.items() {
			if !match(it) {
				continue
			}
			matched = true
			if _, dup := seen[it.ID]; dup {
				continue
			}
			seen[it.ID] = struct{}{}
			out = append(out, it)
		}
		if !matched {
			return nil, fmt.Errorf(
				"source %q: selector %q matches no item; catalog provides %s",
				source, sel, provided(c),
			)
		}
	}

	// By id rather than in the order the selectors were written, which is what keeps
	// the lock's item order independent of how a consumer wrote its manifest.
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// matcher builds the test one selector applies to an item. A selector is kind:name;
// the kind is compared literally because SPEC.md places the glob in the name position
// only, and the name is matched with path.Match. plainName in items.go is what makes
// path.Match's one surprising rule — that * does not cross a separator — unable to
// bite: a name holding "/" never survives parsing, so a kind:* selector cannot skip
// silently past an item its catalog declared.
//
// The pattern is validated up front, against the empty string, rather than on the
// first item it is compared with: a malformed pattern is a typo whichever kind it
// names, and reporting it only when the catalog happens to hold an item of that kind
// would make the message depend on the catalog. ErrBadPattern is path.Match's only
// error, so a bool carries everything the caller can act on.
func matcher(sel string) (func(Item) bool, bool) {
	kind, name, _ := strings.Cut(sel, ":")
	if _, err := path.Match(name, ""); err != nil {
		return nil, false
	}
	return func(it Item) bool {
		if it.Kind != kind {
			return false
		}
		ok, _ := path.Match(name, it.Name)
		return ok
	}, true
}

// provided lists every id the catalog offers, so a typo is visible against the real
// vocabulary rather than only being named back at the reader. Parse already returns
// items sorted; sorting here too means the listing holds for a catalog Expand did not
// build.
func provided(c *Catalog) string {
	if len(c.items()) == 0 {
		return "no items"
	}
	ids := make([]string, 0, len(c.items()))
	for _, it := range c.items() {
		ids = append(ids, it.ID)
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}

// items reads a catalog's items through a nil check, so Expand is total: a caller that
// reaches it with a nil catalog — a parse error whose error return went unchecked — gets
// the ordinary no-match error naming an empty vocabulary instead of a panic. The failure
// stays loud, because every selector that caller wrote still matches nothing.
func (c *Catalog) items() []Item {
	if c == nil {
		return nil
	}
	return c.Items
}
