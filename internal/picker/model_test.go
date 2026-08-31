package picker

import (
	"reflect"
	"slices"
	"testing"
)

// The picker is a value. Every behavior worth asserting is a function of a model and a key,
// which is what lets the whole widget be tested by pressing keys at it rather than by
// driving a terminal.

func threeItems() []Item {
	return []Item{
		{ID: "agent:planner", Kind: "agent", Destinations: []string{".claude/agents/planner.md"}},
		{ID: "agent:reviewer", Kind: "agent", Destinations: []string{".claude/agents/reviewer.md"}},
		{ID: "schema:tdd", Kind: "schema", Destinations: []string{"openspec/schemas/tdd/"}},
	}
}

// press applies keys in order, so a test reads as the sequence a user typed.
func press(m Model, keys ...Key) Model {
	for _, k := range keys {
		m = m.Update(k)
	}
	return m
}

func TestCursorMovesAndSpaceSelects(t *testing.T) {
	t.Parallel()

	m := press(New(threeItems()), KeyDown, KeyDown, KeySpace, KeyUp, KeySpace)

	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	if got := m.chosen(); !slices.Equal(got, []string{"agent:reviewer", "schema:tdd"}) {
		t.Errorf("chosen = %q, want the second and third items", got)
	}
}

// A cursor that wrapped would move a user to the far end of a list they were reading. It
// stops, and stopping changes nothing else about the model.
func TestCursorStopsAtBothEnds(t *testing.T) {
	t.Parallel()

	top := New(threeItems())
	if got := top.Update(KeyUp); !reflect.DeepEqual(got, top) {
		t.Errorf("up at the first item changed the model")
	}

	bottom := press(New(threeItems()), KeyDown, KeyDown)
	if got := bottom.Update(KeyDown); !reflect.DeepEqual(got, bottom) {
		t.Errorf("down at the last item changed the model")
	}
}

func TestAllSelectsEverythingThenClearsIt(t *testing.T) {
	t.Parallel()

	m := press(New(threeItems()), KeySpace, KeyAll)
	if got := len(m.chosen()); got != 3 {
		t.Errorf("chosen = %d items, want all 3", got)
	}

	m = m.Update(KeyAll)
	if got := m.chosen(); len(got) != 0 {
		t.Errorf("chosen = %q, want none after a second `a`", got)
	}
}

// A user pressing a key that does nothing has not made a mistake worth stopping for.
func TestAnUnboundKeyChangesNothing(t *testing.T) {
	t.Parallel()

	m := press(New(threeItems()), KeyDown, KeySpace)
	if got := m.Update(KeyNone); !reflect.DeepEqual(got, m) {
		t.Error("an unbound key changed the model")
	}
	if m.Done() {
		t.Error("an unbound key ended the picker")
	}
}

func TestEnterConfirmsInCatalogOrder(t *testing.T) {
	t.Parallel()

	// Selected bottom-up, so an implementation recording click order would return them
	// backwards.
	m := press(New(threeItems()), KeyDown, KeyDown, KeySpace, KeyUp, KeyUp, KeySpace, KeyEnter)

	if !m.Done() {
		t.Fatal("enter did not end the picker")
	}
	if m.Cancelled() {
		t.Fatal("enter cancelled")
	}
	if got := m.Selectors(); !slices.Equal(got, []string{"agent:planner", "schema:tdd"}) {
		t.Errorf("selectors = %q, want them in catalog order", got)
	}
}

func TestCancellingDiscardsASelection(t *testing.T) {
	t.Parallel()

	for _, k := range []Key{KeyCancel} {
		m := press(New(threeItems()), KeySpace, KeyDown, KeySpace, k)
		if !m.Done() || !m.Cancelled() {
			t.Errorf("cancel key did not cancel: done=%t cancelled=%t", m.Done(), m.Cancelled())
		}
		if got := m.Selectors(); len(got) != 0 {
			t.Errorf("selectors = %q, want none after a cancellation", got)
		}
	}
}

// An add with no selectors is not a thing to write: it would append a source whose install
// list the next parse refuses. Confirming an empty selection is therefore the same outcome
// as walking away from it.
func TestConfirmingNothingIsACancellation(t *testing.T) {
	t.Parallel()

	m := New(threeItems()).Update(KeyEnter)
	if !m.Done() || !m.Cancelled() {
		t.Errorf("empty confirmation: done=%t cancelled=%t, want both true", m.Done(), m.Cancelled())
	}
	if got := m.Selectors(); len(got) != 0 {
		t.Errorf("selectors = %q, want none", got)
	}
}

// A picker over a source that offers nothing has nothing to ask. It is not reached in
// practice — --list prints "(no items)" and an add would have refused the empty selection
// anyway — and it may not panic.
func TestAModelOverNoItemsCancelsRatherThanPanicking(t *testing.T) {
	t.Parallel()

	m := New(nil)
	m = press(m, KeyDown, KeyUp, KeySpace, KeyAll, KeyEnter)
	if !m.Done() || !m.Cancelled() {
		t.Errorf("empty picker: done=%t cancelled=%t, want both true", m.Done(), m.Cancelled())
	}
}
