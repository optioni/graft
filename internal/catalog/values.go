package catalog

import (
	"math"
	"sort"
)

// This file holds every read of a value out of the generically decoded document. They
// live together so a future field cannot be read one way by parseKinds and another way
// by parseItems.

// asInt accepts the integer kinds the YAML decoder actually produces — which one it
// picks depends on the literal's sign and magnitude, not on the schema.
func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case uint64:
		if n > math.MaxInt {
			return 0, false
		}
		return int(n), true
	}
	return 0, false
}

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
