package rev_test

import (
	"testing"

	"github.com/optioni/graft/internal/rev"
)

// A 40-character lowercase hex sha belonging to no fixture: it stands in wherever the
// value's shape matters and its existence does not.
const testSHA = "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5"

func TestIsRange(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		in   string
		want bool
	}{
		"a caret rev is a range":                  {"^1.2.0", true},
		"a plain tag is a ref":                     {"v1.2.0", false},
		"a branch name containing a dash is a ref": {"release-2024-01", false},
		"a compound range with a space is a range": {">=1.2.0 <2.0.0", true},
		"an alternation is a range":                {"1.2.x||1.3.x", true},
		"a bare x-range is a ref, not a range":     {"1.x", false},
		"a full sha is a ref and never a range":    {testSHA, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := rev.IsRange(tc.in); got != tc.want {
				t.Errorf("IsRange(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
