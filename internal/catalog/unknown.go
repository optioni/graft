package catalog

import "slices"

// rejectUnknown walks the decoded document and refuses any key the format does not
// define. It runs before every field is read, so an entry carrying both a misspelled
// key and the key it was meant to be reports the misspelling rather than the absence
// it causes — the latter would send a source author looking for the wrong bug.
//
// A value of the wrong shape is skipped rather than reported here: parseKinds and
// parseItems own those messages, and duplicating them would let the two disagree.
func rejectUnknown(raw map[string]any, filename string) error {
	if k, ok := unknownKey(raw, "version", "kinds", "provides"); ok {
		return errf(filename, "unknown key %q", k)
	}

	kinds, _ := raw["kinds"].(map[string]any)
	for _, name := range sortedKeys(kinds) {
		body, ok := kinds[name].(map[string]any)
		if !ok {
			continue
		}
		if k, ok := unknownKey(body, "to", "flatten"); ok {
			return errf(filename, "kind %q: unknown key %q", name, k)
		}
	}

	list, _ := raw["provides"].([]any)
	for i, e := range list {
		body, ok := e.(map[string]any)
		if !ok {
			continue
		}
		// By index, because an entry carrying an unknown key may have no usable
		// identity yet.
		if k, ok := unknownKey(body, "kind", "name", "from"); ok {
			return errf(filename, "provides[%d]: unknown key %q", i, k)
		}
	}
	return nil
}

// unknownKey returns the lowest-sorting key of m that is not in allowed, so a mapping
// carrying several unknown keys always reports the same one.
func unknownKey(m map[string]any, allowed ...string) (string, bool) {
	for _, k := range sortedKeys(m) {
		if !slices.Contains(allowed, k) {
			return k, true
		}
	}
	return "", false
}
