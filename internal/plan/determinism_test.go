package plan

import (
	"bytes"
	"slices"
	"testing"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/lock"
)

// docSource publishes one file item at the top of the repo, so a test can name a
// destination directly and watch where it lands in the write order.
func docSource(name, doc string) Input {
	it := item("doc:"+doc, "doc", doc, "extras/docs/"+doc+".md")
	return sourceInput(
		name,
		map[string]catalog.Kind{"doc": {To: []string{"{name}.md"}}},
		[]catalog.Item{it},
		map[string]Listing{it.ID: {Files: []string{doc + ".md"}}},
		nil,
	)
}

// TestDeterminism_WritesAreOrderedByDestination: by destination, not by source, item,
// or the order a caller assembled anything. internal/apply walks this slice, so its
// order is part of the plan rather than a presentation detail.
func TestDeterminism_WritesAreOrderedByDestination(t *testing.T) {
	zeta := docSource("zeta", "b")
	alpha := docSource("alpha", "a")

	for _, order := range [][]Input{{zeta, alpha}, {alpha, zeta}} {
		p, err := Build(order, emptyLock())
		if err != nil {
			t.Fatalf("Build: unexpected error: %v", err)
		}
		assertWrites(t, p, "a.md", "b.md")
	}
}

// shuffled is one non-trivial plan's inputs, assembled in the order given. Reversing
// every slice must not change a byte of the resulting lock.
func shuffled(reverse bool) []Input {
	files := []string{"schema.yaml", "templates/design.md", "templates/proposal.md"}
	items := []catalog.Item{tdd, agentX, agentY}
	selectors := []string{"schema:tdd", "agent:*"}
	if reverse {
		slices.Reverse(files)
		slices.Reverse(items)
		slices.Reverse(selectors)
	}

	shared := withInstall(sourceInput(
		"shared",
		map[string]catalog.Kind{"schema": schemaKind, "agent": agentKind},
		items,
		map[string]Listing{
			tdd.ID:    {Dir: true, Files: files},
			agentX.ID: {Files: []string{"x.md"}},
			agentY.ID: {Files: []string{"y.md"}},
		},
		nil,
	), selectors...)
	other := docSource("other", "README")

	order := []Input{shared, other}
	if reverse {
		slices.Reverse(order)
	}
	return order
}

// TestDeterminism_OrderingIsIndependentOfInputOrder asserts byte equality, not
// reflect.DeepEqual of the structs. A lock that reorders on every run churns the diff
// of every consumer, and only the bytes say whether it does.
func TestDeterminism_OrderingIsIndependentOfInputOrder(t *testing.T) {
	first, err := Build(shuffled(false), emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	second, err := Build(shuffled(true), emptyLock())
	if err != nil {
		t.Fatalf("Build from reversed inputs: unexpected error: %v", err)
	}

	if !bytes.Equal(lock.Marshal(first.Lock), lock.Marshal(second.Lock)) {
		t.Errorf("the next lock depends on input order:\n first %s\nsecond %s",
			lock.Marshal(first.Lock), lock.Marshal(second.Lock))
	}
	if !slices.Equal(first.Writes, second.Writes) {
		t.Errorf("the write order depends on input order:\n first %+v\nsecond %+v",
			first.Writes, second.Writes)
	}
}

// TestDeterminism_AnIdempotentReplanPrunesNothing is the idempotent re-sync property at
// the plan tier: feed a plan's own lock back in as the current one and nothing moves.
// The plan still holds a write for every file, because a plan never decides that a file
// needs no writing.
func TestDeterminism_AnIdempotentReplanPrunesNothing(t *testing.T) {
	inputs := shuffled(false)

	first, err := Build(inputs, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	second, err := Build(inputs, first.Lock)
	if err != nil {
		t.Fatalf("re-Build: unexpected error: %v", err)
	}

	if len(second.Prune) != 0 {
		t.Errorf("Prune: want nothing pruned on a re-plan, got %q", second.Prune)
	}
	if !bytes.Equal(lock.Marshal(first.Lock), lock.Marshal(second.Lock)) {
		t.Errorf("the lock changed on a re-plan:\n first %s\nsecond %s",
			lock.Marshal(first.Lock), lock.Marshal(second.Lock))
	}
	if !slices.Equal(first.Writes, second.Writes) {
		t.Errorf("Writes: a re-plan dropped or reordered writes:\n first %+v\nsecond %+v",
			first.Writes, second.Writes)
	}
	if len(second.Writes) == 0 {
		t.Error("Writes: a re-plan wrote nothing; a plan has no notion of unchanged")
	}
}

// TestDeterminism_BuildDoesNotReorderItsCallersSlice: Build sorts, and a caller that
// handed it a slice still holds the slice it handed over.
func TestDeterminism_BuildDoesNotReorderItsCallersSlice(t *testing.T) {
	inputs := []Input{docSource("zeta", "b"), docSource("alpha", "a")}

	if _, err := Build(inputs, emptyLock()); err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	if inputs[0].Source.Name != "zeta" || inputs[1].Source.Name != "alpha" {
		t.Errorf("Build reordered its caller's slice: got %q, %q",
			inputs[0].Source.Name, inputs[1].Source.Name)
	}
}
