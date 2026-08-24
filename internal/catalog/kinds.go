package catalog

import (
	"fmt"
	"path"
	"strings"
)

// parseKinds reads the kinds mapping. Keys are walked in sorted order so a catalog
// with two faulty kinds always reports the same one — Go's map iteration is
// randomised, and every error message here is asserted by a test.
//
// Nothing in this function interprets a destination. {name} is left uninterpolated, a
// trailing slash is left in place, and no path is cleaned or joined: computing a
// destination is internal/plan's job, and splitting it across two packages is how the
// "no destination escapes the repo root" invariant ends up checked in only one. The one
// cleaned form in the package lives inside destKey, is never stored, and decides only
// whether two entries of one `to` are the same destination.
func parseKinds(raw map[string]any, filename string) (map[string]Kind, error) {
	v, ok := raw["kinds"]
	if !ok || v == nil {
		return map[string]Kind{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errf(filename, "kinds must be a mapping")
	}

	kinds := make(map[string]Kind, len(m))
	for _, name := range sortedKeys(m) {
		if name == "" {
			return nil, errf(filename, "kind name is empty")
		}
		fail := func(format string, args ...any) error {
			return errf(filename, "kind %q: %s", name, fmt.Sprintf(format, args...))
		}

		body, ok := m[name].(map[string]any)
		if !ok {
			return nil, fail("must be a mapping")
		}
		to, err := destinations(body["to"], fail)
		if err != nil {
			return nil, err
		}
		flatten, err := flag(body["flatten"], fail)
		if err != nil {
			return nil, err
		}
		kinds[name] = Kind{To: to, Flatten: flatten}
	}
	return kinds, nil
}

// destinations normalises `to` to a list. A string-valued `to` becomes a one-element
// list, which is the smallest representation that lets a later package treat both
// spellings with one code path while discarding nothing.
func destinations(v any, fail func(string, ...any) error) ([]string, error) {
	var to []string
	switch t := v.(type) {
	case nil:
		return nil, fail("to is required")
	case string:
		if t == "" {
			return nil, fail("to is required")
		}
		to = []string{t}
	case []any:
		if len(t) == 0 {
			return nil, fail("to is required")
		}
		for _, e := range t {
			d, ok := e.(string)
			if !ok {
				return nil, fail("to must be a string or a list of strings")
			}
			if d == "" {
				return nil, fail("to contains an empty destination")
			}
			to = append(to, d)
		}
	default:
		return nil, fail("to must be a string or a list of strings")
	}

	seen := make(map[string]string, len(to))
	for _, d := range to {
		key := destKey(d)
		first, dup := seen[key]
		switch {
		case !dup:
			seen[key] = d
		case first == d:
			// Otherwise every item of this kind would be written to the same path
			// twice, and the second write would look like a collision with itself.
			return nil, fail("duplicate destination %q", d)
		default:
			// Two spellings of one path. Both are named: a message repeating one
			// string would send the reader looking for two identical entries.
			return nil, fail("duplicate destination %q: same path as %q", d, first)
		}
	}
	return to, nil
}

// destKey is the identity of a destination as written: what two entries have to share
// before they are one destination. Cleaning alone is not it. A trailing slash means
// "into this directory", which is a no-op for an item whose `from` names a directory and
// significant for one naming a file — so destination-computation requires "docs/{name}"
// and "docs/{name}/" to be one destination for the first and two for the second, and a
// key that cleaned the slash away would make a catalog that spec requires to work
// unparseable here. The slash therefore survives cleaning.
//
// What this does collapse is the spellings that mean one path under either reading:
// ".claude/agents/" against ".claude/agents//", and "a/b" against "./a/b".
//
// It cleans for comparison only. Kind.To carries every entry exactly as written, because
// the package that computes a destination reads the raw string. A dot-suffixed spelling
// such as "a/b/." is not collapsed against "a/b/" — cleaning removes the "." and leaves
// no slash to compare — and nothing rests on it, since destination-computation refuses an
// uncleaned destination outright.
func destKey(d string) string {
	key := path.Clean(d)
	if strings.HasSuffix(d, "/") {
		key += "/"
	}
	return key
}

// flag reads an optional boolean, defaulting to false when the key is absent or null.
func flag(v any, fail func(string, ...any) error) (bool, error) {
	if v == nil {
		return false, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fail("flatten must be a boolean")
	}
	return b, nil
}
