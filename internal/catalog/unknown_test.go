package catalog_test

import (
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// A misspelled key in a source's catalog would otherwise install nothing and say
// nothing, so decoding is strict and the message names where the key appeared.
func TestParse_UnknownKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "at the top level",
			in:   "version: 1\nkind:\n  agent:\n    to: \".claude/agents/\"\n",
			want: `catalog.yaml: unknown key "kind"`,
		},
		{
			name: "inside a kind",
			in:   "version: 1\nkinds:\n  agent:\n    to: \".claude/agents/\"\n    flat: true\n",
			want: `catalog.yaml: kind "agent": unknown key "flat"`,
		},
		{
			name: "inside a provides entry",
			in: declared + "provides:\n  - { kind: agent, name: apply, from: a.md }\n" +
				"  - { kind: agent, name: tdd, path: agents/tdd.md }\n",
			want: `catalog.yaml: provides[1]: unknown key "path"`,
		},
		{
			// SPEC.md's own open question. Rejecting it rather than accepting and
			// ignoring it is what keeps the question askable at version 2.
			name: "requires is not a catalog key at version 1",
			in:   "version: 1\nrequires: []\n",
			want: `catalog.yaml: unknown key "requires"`,
		},
		{
			name: "several unknown keys report the lowest-sorting one",
			in:   "version: 1\nzebra: 1\nalpha: 2\n",
			want: `catalog.yaml: unknown key "alpha"`,
		},
		{
			// The walk runs before item validation, or this entry would be reported
			// for the from it also lacks.
			name: "an unknown key beats the missing field beside it",
			in:   declared + "provides:\n  - { kind: agent, name: tdd, path: a.md }\n",
			want: `catalog.yaml: provides[0]: unknown key "path"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := catalog.Parse([]byte(tt.in), "catalog.yaml")
			if c != nil {
				t.Errorf("Parse() catalog = %+v, want nil", c)
			}
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Parse() error = %v, want %q", err, tt.want)
			}
		})
	}
}
