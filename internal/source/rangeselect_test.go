package source

import (
	"slices"
	"testing"
)

// MatchRange takes a []string of tag names — the remote's job is producing that slice,
// and that is group 4 — so every case here is a unit test over a slice, no git process
// started.

func TestMatchRangeCaretRangeParses(t *testing.T) {
	t.Parallel()

	if _, err := MatchRange("shared", "^1.2.0", []string{"v1.2.0"}); err != nil {
		t.Errorf("MatchRange: unexpected error: %v", err)
	}
}

func TestMatchRangeMalformedRangeIsRefusedWithoutANetworkCall(t *testing.T) {
	t.Parallel()

	tag, err := MatchRange("shared", "^^1", []string{"v1.2.0"})
	want := `source "shared": rev "^^1" is not a valid semver range`
	if err == nil || err.Error() != want {
		t.Fatalf("MatchRange:\n got %v\nwant %q", err, want)
	}
	if tag != "" {
		t.Errorf("MatchRange: want an empty tag on refusal, got %q", tag)
	}
}

func TestMatchRangeSelection(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		rng  string
		tags []string
		want string
	}{
		"the highest satisfying tag wins": {
			"^1.2.0", []string{"v1.1.0", "v1.2.0", "v1.3.0", "v2.0.0"}, "v1.3.0",
		},
		"a tag without the v prefix is accepted": {
			"^1.2.0", []string{"1.2.0", "1.3.0"}, "1.3.0",
		},
		"unparseable tags are ignored rather than refused": {
			"^1.0.0", []string{"v1.2.0", "latest", "release-2024-01", "nightly"}, "v1.2.0",
		},
		"a range matching exactly one tag": {
			"^1.2.0", []string{"v1.2.0"}, "v1.2.0",
		},
		"an exact-version range selects that version": {
			"=1.2.0", []string{"v1.2.0", "v1.3.0"}, "v1.2.0",
		},
		"build metadata does not affect precedence": {
			"^1.2.0", []string{"v1.3.0", "v1.3.0+build.1"}, "v1.3.0",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := MatchRange("shared", tc.rng, tc.tags)
			if err != nil {
				t.Fatalf("MatchRange: unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("MatchRange(%q, %q) = %q, want %q", tc.rng, tc.tags, got, tc.want)
			}
		})
	}
}

func TestMatchRangePrereleases(t *testing.T) {
	t.Parallel()

	t.Run("a prerelease is not selected by a plain range", func(t *testing.T) {
		t.Parallel()
		got, err := MatchRange("shared", "^1.2.0", []string{"v1.2.0", "v1.3.0-rc.1"})
		if err != nil {
			t.Fatalf("MatchRange: unexpected error: %v", err)
		}
		if got != "v1.2.0" {
			t.Errorf("MatchRange = %q, want %q: a prerelease must not win even though it sorts higher", got, "v1.2.0")
		}
	})

	t.Run("a range naming a prerelease admits it", func(t *testing.T) {
		t.Parallel()
		got, err := MatchRange("shared", ">=1.3.0-rc.0", []string{"v1.2.0", "v1.3.0-rc.1"})
		if err != nil {
			t.Fatalf("MatchRange: unexpected error: %v", err)
		}
		if got != "v1.3.0-rc.1" {
			t.Errorf("MatchRange = %q, want %q", got, "v1.3.0-rc.1")
		}
	})

	t.Run("only prereleases exist and the range names none", func(t *testing.T) {
		t.Parallel()
		_, err := MatchRange("shared", "^1.0.0", []string{"v1.0.0-alpha.1"})
		want := `source "shared": rev "^1.0.0" matches none of the source's semver tags`
		if err == nil || err.Error() != want {
			t.Fatalf("MatchRange:\n got %v\nwant %q", err, want)
		}
	})
}

func TestMatchRangeUnsatisfiable(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		rng  string
		tags []string
		want string
	}{
		"no tag satisfies the range": {
			"^2.0.0",
			[]string{"v1.0.0", "v1.1.0"},
			`source "shared": rev "^2.0.0" matches none of the source's semver tags`,
		},
		"the source publishes no semver tags": {
			"^1.0.0",
			[]string{"latest", "release-2024-01"},
			`source "shared": rev "^1.0.0" is a range, and the source publishes no semver tags`,
		},
		"the source publishes no tags at all": {
			"^1.0.0", nil,
			`source "shared": rev "^1.0.0" is a range, and the source publishes no semver tags`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tag, err := MatchRange("shared", tc.rng, tc.tags)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("MatchRange:\n got %v\nwant %q", err, tc.want)
			}
			if tag != "" {
				t.Errorf("MatchRange: want an empty tag on failure, got %q", tag)
			}
		})
	}
}

func TestMatchRangeDeterminism(t *testing.T) {
	t.Parallel()

	t.Run("tag order from the remote does not affect the result", func(t *testing.T) {
		t.Parallel()
		tags := []string{"v1.1.0", "v1.2.0", "v1.3.0", "v2.0.0"}
		first, err := MatchRange("shared", "^1.2.0", tags)
		if err != nil {
			t.Fatalf("MatchRange: unexpected error: %v", err)
		}

		// A fixed reversed ordering rather than a random shuffle: an unseeded shuffle of
		// a four-element slice lands on the original ordering about once in 24 runs,
		// which would silently assert nothing that time.
		reversed := slices.Clone(tags)
		slices.Reverse(reversed)
		second, err := MatchRange("shared", "^1.2.0", reversed)
		if err != nil {
			t.Fatalf("MatchRange: unexpected error: %v", err)
		}
		if first != second {
			t.Errorf("MatchRange is order-dependent: %q vs %q", first, second)
		}
	})

	t.Run("two tags naming the same version", func(t *testing.T) {
		t.Parallel()
		got, err := MatchRange("shared", "^1.2.0", []string{"v1.3.0", "1.3.0"})
		if err != nil {
			t.Fatalf("MatchRange: unexpected error: %v", err)
		}
		if got != "1.3.0" {
			t.Errorf(`MatchRange = %q, want "1.3.0": the tie breaks toward the lower tag name`, got)
		}

		got2, err := MatchRange("shared", "^1.2.0", []string{"1.3.0", "v1.3.0"})
		if err != nil {
			t.Fatalf("MatchRange: unexpected error: %v", err)
		}
		if got2 != got {
			t.Errorf("MatchRange depends on input order: %q vs %q", got2, got)
		}
	})
}
