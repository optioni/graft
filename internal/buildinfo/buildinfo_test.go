package buildinfo_test

import (
	"testing"

	"github.com/optioni/graft/internal/buildinfo"
)

func TestFormat(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		version, commit, date string
		want                  string
	}{
		"all fields":   {"v1.2.0", "abc1234", "2026-08-23", "graft v1.2.0 (abc1234, built 2026-08-23)"},
		"unset build":  {"dev", "unknown", "unknown", "graft dev (unknown, built unknown)"},
		"empty fields": {"", "", "", "graft unknown (unknown, built unknown)"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := buildinfo.Format(tc.version, tc.commit, tc.date); got != tc.want {
				t.Errorf("Format() = %q, want %q", got, tc.want)
			}
		})
	}
}
