package catalog

import "sort"

// This file holds every read of a value out of the generically decoded document. They
// live together so a future field cannot be read one way by parseKinds and another way
// by parseItems.

// field reads a required string. An absent, null, or empty value is "required" rather
// than "wrong type": a key written but left blank is the same mistake as one omitted,
// and saying so twice two different ways helps nobody.
func field(m map[string]any, key string, at func(string, ...any) error) (string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", at("%s is required", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", at("%s must be a string", key)
	}
	if s == "" {
		return "", at("%s is required", key)
	}
	return s, nil
}

// sortedKeys returns m's keys in ascending byte order, so validation reports the same
// fault on every run rather than whichever one Go's randomised map iteration reached
// first.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
