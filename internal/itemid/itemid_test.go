package itemid_test

import (
	"testing"

	"github.com/optioni/graft/internal/itemid"
)

func TestValid(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]bool{
		"schema:tdd":           true,
		"agent:*":              true,
		"agent:outside-in-*":   true,
		"agent:outside-in-?":   true,
		"tdd":                  false,
		"schema:":              false,
		":tdd":                 false,
		"schema:tdd:extra":     false,
		":":                    false,
		"":                     false,
		"schema:openspec/tdd":  true,
		"schema name:with dot": true,
	} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if got := itemid.Valid(in); got != want {
				t.Errorf("Valid(%q) = %v, want %v", in, got, want)
			}
		})
	}
}
