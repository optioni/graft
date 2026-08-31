package plan

import (
	"testing"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/lock"
)

// A planned write records whether the lock already claimed its destination. It is the half
// of "did this replace something" that can be answered without a filesystem, which is why
// it is answered here and the other half is not.

// claimedFor builds a plan over one file item and reports whether its write was marked.
func claimedFor(t *testing.T, lk *lock.Lock) bool {
	t.Helper()

	it := item("agent:reviewer", "agent", "reviewer", "extras/agents/reviewer.md")
	in := sourceInput("shared",
		map[string]catalog.Kind{"agent": {To: []string{".claude/agents/"}, Flatten: true}},
		[]catalog.Item{it},
		map[string]Listing{it.ID: {Files: []string{"reviewer.md"}}},
		nil,
	)
	p, err := Build([]Input{in}, lk)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(p.Writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(p.Writes))
	}
	return p.Writes[0].Claimed
}

// lockClaiming is a lock recording one item holding one path, under a chosen source and id.
func lockClaiming(source, id, path string) *lock.Lock {
	return &lock.Lock{
		Version: lock.Version,
		Sources: []lock.Source{{
			Name:     source,
			Git:      "github.com/optioni/" + source,
			Rev:      "v1.0.0",
			Resolved: "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5",
			Items:    []lock.Item{{ID: id, Files: []string{path}}},
		}},
	}
}

func TestAPathTheLockClaimsIsMarked(t *testing.T) {
	t.Parallel()

	if !claimedFor(t, lockClaiming("shared", "agent:reviewer", ".claude/agents/reviewer.md")) {
		t.Error("a path the lock claims was not marked claimed")
	}
}

// The question is whether graft owned the path, not which item owned it: a path moving
// between sources or items is still graft rewriting its own file.
func TestAPathClaimedElsewhereStillCounts(t *testing.T) {
	t.Parallel()

	if !claimedFor(t, lockClaiming("other", "schema:tdd", ".claude/agents/reviewer.md")) {
		t.Error("a path claimed under another source and item was not marked claimed")
	}
}

func TestAPathNoLockClaimsIsNotMarked(t *testing.T) {
	t.Parallel()

	if claimedFor(t, lockClaiming("shared", "agent:reviewer", ".claude/agents/other.md")) {
		t.Error("a path the lock does not claim was marked claimed")
	}
}

func TestAnEmptyLockClaimsNothing(t *testing.T) {
	t.Parallel()

	for name, lk := range map[string]*lock.Lock{
		"no sources": {Version: lock.Version},
		"no lock":    nil,
	} {
		if claimedFor(t, lk) {
			t.Errorf("%s: a write was marked claimed", name)
		}
	}
}
