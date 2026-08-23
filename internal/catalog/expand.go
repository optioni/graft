package catalog

import (
	"fmt"
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
	seen := make(map[string]struct{}, len(c.Items))
	for _, sel := range selectors {
		matched := false
		for _, it := range c.Items {
			if it.ID != sel {
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

// provided lists every id the catalog offers, so a typo is visible against the real
// vocabulary rather than only being named back at the reader. Parse already returns
// items sorted; sorting here too means the listing holds for a catalog Expand did not
// build.
func provided(c *Catalog) string {
	if len(c.Items) == 0 {
		return "no items"
	}
	ids := make([]string, 0, len(c.Items))
	for _, it := range c.Items {
		ids = append(ids, it.ID)
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}
