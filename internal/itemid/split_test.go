package itemid_test

import (
	"testing"

	"github.com/optioni/graft/internal/itemid"
)

// splitCases is the grammar as a table: what an id splits into, and whether it is an id at
// all. Valid and Split are asserted against the same rows, which is what makes "the grammar
// is stated once" a property a test can fail on rather than a claim in a comment.
var splitCases = map[string]struct {
	kind, name string
	ok         bool
}{
	"schema:tdd":           {"schema", "tdd", true},
	"agent:*":              {"agent", "*", true},
	"agent:outside-in-*":   {"agent", "outside-in-*", true},
	"schema:openspec/tdd":  {"schema", "openspec/tdd", true},
	"schema name:with dot": {"schema name", "with dot", true},
	"tdd":                  {"", "", false},
	"schema:":              {"", "", false},
	":tdd":                 {"", "", false},
	"schema:tdd:extra":     {"", "", false},
	":":                    {"", "", false},
	"":                     {"", "", false},
}

func TestSplit(t *testing.T) {
	t.Parallel()

	for in, want := range splitCases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			kind, name, ok := itemid.Split(in)
			if kind != want.kind || name != want.name || ok != want.ok {
				t.Errorf("Split(%q) = %q, %q, %v; want %q, %q, %v",
					in, kind, name, ok, want.kind, want.name, want.ok)
			}
		})
	}
}

// The two halves of one grammar. Valid asks whether an id parses and Split parses it, so a
// rule tightened in one and not the other is a defect no test of either alone would catch.
func TestValidAgreesWithSplit(t *testing.T) {
	t.Parallel()

	for in, want := range splitCases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			// Against the table rather than against Split's own answer: Valid is written
			// through Split today, and a test comparing the two would pass by construction
			// and go on passing if either were later re-implemented on its own.
			if got := itemid.Valid(in); got != want.ok {
				t.Errorf("Valid(%q) = %v, want %v", in, got, want.ok)
			}
		})
	}
}
