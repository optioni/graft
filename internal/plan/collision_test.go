package plan

import (
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// flatAgents is the kind SPEC.md's own catalog declares: everything into one directory,
// nested structure discarded.
var flatAgents = catalog.Kind{To: []string{".claude/agents/"}, Flatten: true}

// TestCollision_TwoItemsOfOneSource: a collision is an error rather than
// last-writer-wins, because the loser would be a file the lock claims and a later sync
// would delete.
func TestCollision_TwoItemsOfOneSource(t *testing.T) {
	review := item("agent:review", "agent", "review", "extras/agents/review.md")
	packed := item("agent:pack", "agent", "pack", "extras/agents/pack")

	in := sourceInput(
		"shared",
		map[string]catalog.Kind{"agent": flatAgents},
		[]catalog.Item{review, packed},
		map[string]Listing{
			review.ID: {Files: []string{"review.md"}},
			packed.ID: {Dir: true, Files: []string{"nested/review.md"}},
		},
		nil,
	)

	p, err := Build([]Input{in}, emptyLock())
	assertNoPlan(t, p, err,
		`source "shared" item "agent:pack" and source "shared" item "agent:review" both resolve to ".claude/agents/review.md"`)
}

func TestCollision_TwoSources(t *testing.T) {
	alpha := agentSource("alpha", nil, agentX)
	beta := agentSource("beta", nil, agentX)

	// Supplied in reverse, so the message proves the walk is by source name rather
	// than by the order a caller assembled the slice.
	p, err := Build([]Input{beta, alpha}, emptyLock())
	assertNoPlan(t, p, err,
		`source "alpha" item "agent:x" and source "beta" item "agent:x" both resolve to ".claude/agents/x.md"`)
}

// TestCollision_APathTheLockAlreadyClaims: the lock having claimed the path grants no
// precedence, and nothing in the lock is pruned, because a failed build produces no
// plan at all.
func TestCollision_APathTheLockAlreadyClaims(t *testing.T) {
	packed := item("agent:y", "agent", "y", "extras/agents/y")
	in := sourceInput(
		"shared",
		map[string]catalog.Kind{"agent": flatAgents},
		[]catalog.Item{agentX, packed},
		map[string]Listing{
			agentX.ID: {Files: []string{"x.md"}},
			packed.ID: {Dir: true, Files: []string{"nested/x.md"}},
		},
		nil,
	)
	lk := lockOf(lockSource("shared", lockItem("agent:x", ".claude/agents/x.md")))

	p, err := Build([]Input{in}, lk)
	assertNoPlan(t, p, err,
		`source "shared" item "agent:x" and source "shared" item "agent:y" both resolve to ".claude/agents/x.md"`)
}
