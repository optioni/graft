package catalog_test

import (
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

func TestExpand_Globs(t *testing.T) {
	t.Parallel()

	three := []string{"agent:apply-orchestrator", "agent:tdd-reviewer", "schema:tdd"}

	tests := []struct {
		name      string
		items     []string
		selectors []string
		want      []string
	}{
		{
			name:      "a trailing star selects every item of a kind",
			items:     three,
			selectors: []string{"agent:*"},
			want:      []string{"agent:apply-orchestrator", "agent:tdd-reviewer"},
		},
		{
			name:      "a prefix glob selects a subset",
			items:     three,
			selectors: []string{"agent:tdd-*"},
			want:      []string{"agent:tdd-reviewer"},
		},
		{
			// ? matches one character and never zero, so schema:td is not selected.
			name:      "a question mark matches exactly one character",
			items:     []string{"schema:td", "schema:tdd"},
			selectors: []string{"schema:td?"},
			want:      []string{"schema:tdd"},
		},
		{
			name:      "overlapping selectors yield each item once",
			items:     three,
			selectors: []string{"agent:*", "agent:tdd-reviewer"},
			want:      []string{"agent:apply-orchestrator", "agent:tdd-reviewer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := catalog.Expand(fixture(tt.items...), source, tt.selectors)
			if err != nil {
				t.Fatalf("Expand() error = %v, want nil", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Expand() = %q, want %q", ids(got), tt.want)
			}
			if !sameStrings(ids(got), tt.want) {
				t.Errorf("Expand() = %q, want %q", ids(got), tt.want)
			}
		})
	}
}

func TestExpand_GlobErrors(t *testing.T) {
	t.Parallel()

	three := []string{"agent:apply-orchestrator", "agent:tdd-reviewer", "schema:tdd"}
	const provides = "catalog provides agent:apply-orchestrator, agent:tdd-reviewer, schema:tdd"

	tests := []struct {
		name      string
		items     []string
		selectors []string
		want      string
	}{
		{
			// A glob that adopts items a source adds later still has to match one
			// today.
			name:      "a glob matching nothing",
			items:     three,
			selectors: []string{"hook:*"},
			want:      `source "openspec-schemas": selector "hook:*" matches no item; ` + provides,
		},
		{
			// The kind is compared literally, so *:tdd is not a way to select both
			// kinds — it is a selector naming a kind no catalog has. Both items here
			// have the name tdd, and neither is selected.
			name:      "the kind position is matched literally",
			items:     []string{"agent:tdd-reviewer", "schema:tdd"},
			selectors: []string{"*:tdd"},
			want: `source "openspec-schemas": selector "*:tdd" matches no item; ` +
				`catalog provides agent:tdd-reviewer, schema:tdd`,
		},
		{
			name:      "one selector matching does not excuse another that does not",
			items:     three,
			selectors: []string{"agent:*", "schema:missing"},
			want:      `source "openspec-schemas": selector "schema:missing" matches no item; ` + provides,
		},
		{
			// A bad pattern is a typo, and typo protection is the point of the
			// no-match rule, so it is reported rather than swallowed into "no match".
			name:      "a malformed glob pattern",
			items:     three,
			selectors: []string{"agent:[tdd"},
			want:      `source "openspec-schemas": invalid selector pattern "agent:[tdd"`,
		},
		{
			// Reported even though no item could have been compared against it: the
			// message must not depend on what the catalog happens to hold.
			name:      "a malformed pattern under a kind with no items",
			items:     three,
			selectors: []string{"hook:[tdd"},
			want:      `source "openspec-schemas": invalid selector pattern "hook:[tdd"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := catalog.Expand(fixture(tt.items...), source, tt.selectors)
			if got != nil {
				t.Errorf("Expand() items = %q, want nil", ids(got))
			}
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Expand() error = %v, want %q", err, tt.want)
			}
		})
	}
}
