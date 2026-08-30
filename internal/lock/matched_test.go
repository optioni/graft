package lock_test

import (
	"strings"
	"testing"

	"github.com/optioni/graft/internal/lock"
)

// rangeSource is a lock body pinning a range, so a test can name only what differs from
// canonicalLock.
func rangeSourceBody() string {
	return `version = 1

[[source]]
name     = "openspec-schemas"
git      = "github.com/optioni/openspec-schemas"
rev      = "^1.2.0"
matched  = "v1.3.0"
resolved = "` + sha + `"
`
}

func TestParse_RangeSourceCarriesMatched(t *testing.T) {
	t.Parallel()

	l, err := lock.Parse([]byte(rangeSourceBody()), "graft.lock")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if len(l.Sources) != 1 {
		t.Fatalf("Sources = %+v, want exactly one", l.Sources)
	}
	s := l.Sources[0]
	if s.Rev != "^1.2.0" || s.Matched != "v1.3.0" {
		t.Errorf("Rev, Matched = %q, %q, want %q, %q", s.Rev, s.Matched, "^1.2.0", "v1.3.0")
	}
}

// TestMarshal_RangeCarriesTheMatchedTag: a range's lock carries the matched tag, and a
// ref's carries no matched key at all — not as an empty string, and not as an
// absent-but-present key.
func TestMarshal_RangeCarriesTheMatchedTag(t *testing.T) {
	t.Parallel()

	l := &lock.Lock{Version: 1, Sources: []lock.Source{{
		Name: "openspec-schemas", Git: "github.com/optioni/openspec-schemas",
		Rev: "^1.2.0", Matched: "v1.3.0", Resolved: sha,
	}}}

	got := string(lock.Marshal(l))
	for _, want := range []string{
		`rev      = "^1.2.0"`,
		`matched  = "v1.3.0"`,
		`resolved = "` + sha + `"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Marshal() = %s, want it to contain %q", got, want)
		}
	}
}

func TestMarshal_RefCarriesNoMatchedKey(t *testing.T) {
	t.Parallel()

	got := string(lock.Marshal(canonicalLock()))
	if strings.Contains(got, "matched") {
		t.Errorf("Marshal() of a ref pin carries a matched key:\n%s", got)
	}
}

// TestMarshal_RangeSourceSerializesMatchedBetweenRevAndResolved asserts the exact four
// header lines, aligned in the same column resolved already uses.
func TestMarshal_RangeSourceSerializesMatchedBetweenRevAndResolved(t *testing.T) {
	t.Parallel()

	l := &lock.Lock{Version: 1, Sources: []lock.Source{{
		Name: "openspec-schemas", Git: "github.com/optioni/openspec-schemas",
		Rev: "^1.2.0", Matched: "v1.3.0", Resolved: sha,
	}}}

	want := "name     = \"openspec-schemas\"\n" +
		"git      = \"github.com/optioni/openspec-schemas\"\n" +
		"rev      = \"^1.2.0\"\n" +
		"matched  = \"v1.3.0\"\n" +
		"resolved = \"" + sha + "\"\n"
	if got := string(lock.Marshal(l)); !strings.Contains(got, want) {
		t.Errorf("Marshal() = %s, want it to contain:\n%s", got, want)
	}
}

func TestParse_MatchedValidation(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		in   string
		want string
	}{
		"a matched key on a ref pin is refused": {
			in: `version = 1

[[source]]
name     = "openspec-schemas"
git      = "github.com/optioni/openspec-schemas"
rev      = "v1.2.0"
matched  = "v1.2.0"
resolved = "` + sha + `"
`,
			want: `graft.lock: source "openspec-schemas": matched is only valid when rev is a range`,
		},
		"a range pin without a matched key is refused": {
			in: `version = 1

[[source]]
name     = "openspec-schemas"
git      = "github.com/optioni/openspec-schemas"
rev      = "^1.2.0"
resolved = "` + sha + `"
`,
			want: `graft.lock: source "openspec-schemas": matched is required when rev is a range`,
		},
		"an empty matched value is refused": {
			in: `version = 1

[[source]]
name     = "openspec-schemas"
git      = "github.com/optioni/openspec-schemas"
rev      = "^1.2.0"
matched  = ""
resolved = "` + sha + `"
`,
			want: `graft.lock: source "openspec-schemas": matched is empty`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			l, err := lock.Parse([]byte(tc.in), "graft.lock")
			if err == nil {
				t.Fatalf("Parse() error = nil, want %q", tc.want)
			}
			if got := err.Error(); got != tc.want {
				t.Errorf("Parse() error = %q, want %q", got, tc.want)
			}
			if l != nil {
				t.Errorf("Parse() lock = %+v, want nil", l)
			}
		})
	}
}

// TestParse_TheRangeTestIsTheSameOneResolutionUses: "1.x" is classified as a ref by
// internal/rev's rule, so a source pinning it needs no matched key. Lock validation and
// resolution ask the same function, and can never disagree about whether a rev is a
// range.
func TestParse_TheRangeTestIsTheSameOneResolutionUses(t *testing.T) {
	t.Parallel()

	in := `version = 1

[[source]]
name     = "openspec-schemas"
git      = "github.com/optioni/openspec-schemas"
rev      = "1.x"
resolved = "` + sha + `"
`
	l, err := lock.Parse([]byte(in), "graft.lock")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil: 1.x is a ref, not a range", err)
	}
	if len(l.Sources) != 1 || l.Sources[0].Rev != "1.x" {
		t.Errorf("Sources = %+v, want one source pinning 1.x", l.Sources)
	}
}
