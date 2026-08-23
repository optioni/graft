package plan

import (
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// TestCollision_OneDestinationInsideAnother: two items whose paths do not match and
// still cannot both exist — one item's file is the directory another item's file needs.
// Apply would fail partway through, and because the lock is written last the file
// already written would be absent from graft.lock, outside anything a later prune could
// reach. That state is the one this whole design exists to make unreachable, which is
// why the check belongs where the plan is built rather than where it is applied.
func TestCollision_OneDestinationInsideAnother(t *testing.T) {
	api := item("doc:api", "doc", "api", "extras/docs/api.md")
	packed := item("schema:api", "schema", "api", "extras/schemas/api")

	in := sourceInput(
		"shared",
		map[string]catalog.Kind{
			"doc":    {To: []string{"docs/{name}"}},
			"schema": {To: []string{"docs/{name}"}},
		},
		[]catalog.Item{api, packed},
		map[string]Listing{
			api.ID:    {Files: []string{"api.md"}},
			packed.ID: {Dir: true, Files: []string{"index.md"}},
		},
		nil,
	)

	p, err := Build([]Input{in}, emptyLock())
	assertNoPlan(t, p, err,
		`source "shared" item "doc:api" writes "docs/api" and source "shared" item "schema:api" writes "docs/api/index.md": one cannot contain the other`)
}

// TestCollision_ADirectoryClaimedAfterItsFile is the same clash reached from the other
// side of the walk, so neither order lets it through.
func TestCollision_ADirectoryClaimedAfterItsFile(t *testing.T) {
	packed := item("agent:api", "agent", "api", "extras/agents/api")
	api := item("doc:api", "doc", "api", "extras/docs/api.md")

	in := sourceInput(
		"shared",
		map[string]catalog.Kind{
			"agent": {To: []string{"docs/{name}"}},
			"doc":   {To: []string{"docs/{name}"}},
		},
		[]catalog.Item{packed, api},
		map[string]Listing{
			packed.ID: {Dir: true, Files: []string{"index.md"}},
			api.ID:    {Files: []string{"api.md"}},
		},
		nil,
	)

	p, err := Build([]Input{in}, emptyLock())
	assertNoPlan(t, p, err,
		`source "shared" item "agent:api" writes "docs/api/index.md" and source "shared" item "doc:api" writes "docs/api": one cannot contain the other`)
}

// TestCollision_SiblingsInOneDirectoryAreNotANesting guards the check against being too
// eager: two files in the same directory share every ancestor and collide with nothing.
func TestCollision_SiblingsInOneDirectoryAreNotANesting(t *testing.T) {
	p, err := Build([]Input{schemaSource(Listing{
		Dir:   true,
		Files: []string{"schema.yaml", "templates/design.md", "templates/proposal.md"},
	})}, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	assertWrites(t, p,
		"openspec/schemas/tdd/schema.yaml",
		"openspec/schemas/tdd/templates/design.md",
		"openspec/schemas/tdd/templates/proposal.md",
	)
}

// TestBuild_RefusesAResolvedThatIsNotASHA: a plan may never build a lock a later sync
// would refuse to read, and lock.Parse requires a 40-character lowercase hex sha. This
// is the one load-time constraint a caller can violate silently — every other one is a
// consequence of what planning already guarantees — so it is checked rather than
// assumed, and checked here rather than one run later in a different package against a
// file the user is told not to edit.
func TestBuild_RefusesAResolvedThatIsNotASHA(t *testing.T) {
	for _, resolved := range []string{"", "v1.2.0", "FAE2A30C1D4B8E9F0A2B3C4D5E6F708192A3B4C5", resolvedSHA + "0"} {
		in := schemaSource(Listing{Dir: true, Files: []string{"schema.yaml"}})
		in.Resolved = resolved

		p, err := Build([]Input{in}, emptyLock())
		assertNoPlan(t, p, err, `source "shared": resolved "`+resolved+`" is not a 40-character hex sha`)
	}
}

// TestBuild_RefusesTheShaBeforePlanningAnything: the refusal precedes expansion, so a
// source that would also fail for a bad selector reports the sha it was handed.
func TestBuild_RefusesTheShaBeforePlanningAnything(t *testing.T) {
	in := withInstall(schemaSource(Listing{Dir: true, Files: []string{"schema.yaml"}}), "schema:typo")
	in.Resolved = ""

	p, err := Build([]Input{in}, lockOf(lockSource("shared", lockItem("schema:tdd", "openspec/schemas/tdd/schema.yaml"))))
	assertNoPlan(t, p, err, `source "shared": resolved "" is not a 40-character hex sha`)
}

// TestCollision_OneItemNestingAgainstItself pins the one route to a nesting clash that
// needs no second item: for a *file* item, `docs/` and `docs` are genuinely two
// destinations — one a directory to place into, one the file itself — so the
// interpolate-alike check rightly does not fire, and they then meet as a nesting. It is
// refused, which is what matters; the message naming one item as both parties is
// accurate but reads oddly, and is recorded in planning-review.md as deferred wording.
func TestCollision_OneItemNestingAgainstItself(t *testing.T) {
	notes := item("doc:x", "doc", "x", "extras/docs/x.md")
	in := sourceInput(
		"shared",
		map[string]catalog.Kind{"doc": {To: []string{"docs/", "docs"}}},
		[]catalog.Item{notes},
		map[string]Listing{notes.ID: {Files: []string{"x.md"}}},
		nil,
	)

	p, err := Build([]Input{in}, emptyLock())
	assertNoPlan(t, p, err,
		`source "shared" item "doc:x" writes "docs/x.md" and source "shared" item "doc:x" writes "docs": one cannot contain the other`)
}
