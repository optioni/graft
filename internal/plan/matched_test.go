package plan

import (
	"strings"
	"testing"

	"github.com/optioni/graft/internal/lock"
)

// TestBuild_ARangeSourceCarriesTheMatchedTagAndRoundTrips: plan.Input gains Matched,
// carried verbatim into the lock.Source Build constructs. Without this, every range
// source produces matched = "" in the next lock, which lock's own validation refuses —
// so the round trip through lock.Parse is the assertion that matters, not merely that
// the bytes contain the tag.
func TestBuild_ARangeSourceCarriesTheMatchedTagAndRoundTrips(t *testing.T) {
	in := schemaSource(Listing{Dir: true, Files: []string{"schema.yaml"}})
	in.Source.Rev = "^1.2.0"
	in.Matched = "v1.3.0"

	p, err := Build([]Input{in}, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}

	data := lock.Marshal(p.Lock)
	if !strings.Contains(string(data), `matched  = "v1.3.0"`) {
		t.Errorf("next lock:\n%s\ndoes not carry matched", data)
	}

	parsed, err := lock.Parse(data, "graft.lock")
	if err != nil {
		t.Fatalf("lock.Parse of the plan's own lock failed: %v\n%s", err, data)
	}
	if len(parsed.Sources) != 1 || parsed.Sources[0].Matched != "v1.3.0" {
		t.Errorf("round trip: Sources = %+v, want one source with matched v1.3.0", parsed.Sources)
	}
}

// TestBuild_ARefSourceCarriesNoMatchedTag: the serialized bytes contain no matched line
// at all — not an empty one, which lock.Parse would refuse just as it refuses an empty
// matched value declared explicitly.
func TestBuild_ARefSourceCarriesNoMatchedTag(t *testing.T) {
	in := schemaSource(Listing{Dir: true, Files: []string{"schema.yaml"}})

	p, err := Build([]Input{in}, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}

	data := lock.Marshal(p.Lock)
	if strings.Contains(string(data), "matched") {
		t.Errorf("next lock:\n%s\ncarries a matched line for a ref pin", data)
	}

	if _, err := lock.Parse(data, "graft.lock"); err != nil {
		t.Fatalf("lock.Parse of the plan's own lock failed: %v\n%s", err, data)
	}
}
