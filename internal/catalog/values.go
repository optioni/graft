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

// sequence reads a list out of the decoded document, treating an absent or null value
// as empty.
func sequence(v any) []any {
	s, _ := v.([]any)
	return s
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
