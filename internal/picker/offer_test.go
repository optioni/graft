package picker

import (
	"slices"
	"testing"
)

// The collapse offer is a semantic choice, not a confirmation: `agent:*` adopts agents the
// source adds later and an explicit list does not, and the only moment a user has the
// context to decide is right after selecting every agent.

func TestSelectingEveryItemOfAKindOffersTheGlob(t *testing.T) {
	t.Parallel()

	// Both agents, not the schema.
	base := press(New(threeItems()), KeySpace, KeyDown, KeySpace, KeyEnter)
	if base.Done() {
		t.Fatal("the offer was not shown")
	}

	accepted := base.Update(KeyYes)
	if !accepted.Done() || accepted.Cancelled() {
		t.Fatalf("accepting did not finish: done=%t cancelled=%t", accepted.Done(), accepted.Cancelled())
	}
	if got := accepted.Selectors(); !slices.Equal(got, []string{"agent:*"}) {
		t.Errorf("accepted selectors = %q, want the glob alone", got)
	}

	declined := base.Update(KeyNo)
	if got := declined.Selectors(); !slices.Equal(got, []string{"agent:planner", "agent:reviewer"}) {
		t.Errorf("declined selectors = %q, want both ids", got)
	}
}

func TestAKindWithOneItemIsNeverOffered(t *testing.T) {
	t.Parallel()

	// The schema alone: its kind is wholly selected, and has one item.
	m := press(New(threeItems()), KeyDown, KeyDown, KeySpace, KeyEnter)

	if !m.Done() || m.Cancelled() {
		t.Fatalf("done=%t cancelled=%t, want a confirmed selection with no offer", m.Done(), m.Cancelled())
	}
	if got := m.Selectors(); !slices.Equal(got, []string{"schema:tdd"}) {
		t.Errorf("selectors = %q, want the id rather than a glob", got)
	}
}

func TestTwoWhollySelectedKindsAreOfferedSeparately(t *testing.T) {
	t.Parallel()

	items := append(threeItems(), Item{
		ID: "schema:review", Kind: "schema", Destinations: []string{"openspec/schemas/review/"},
	})

	// Everything selected: both kinds are whole and both have two items.
	m := New(items).Update(KeyAll).Update(KeyEnter)
	if m.Done() {
		t.Fatal("no offer was shown")
	}

	// Accept the first kind offered, decline the second.
	m = m.Update(KeyYes)
	if m.Done() {
		t.Fatal("the second kind was not offered separately")
	}
	m = m.Update(KeyNo)

	if !m.Done() || m.Cancelled() {
		t.Fatalf("done=%t cancelled=%t after both offers", m.Done(), m.Cancelled())
	}
	want := []string{"agent:*", "schema:review", "schema:tdd"}
	if got := m.Selectors(); !slices.Equal(got, want) {
		t.Errorf("selectors = %q, want %q — one glob, the other kind's ids in order", got, want)
	}
}

func TestCancellingAtTheOfferCancelsEverything(t *testing.T) {
	t.Parallel()

	m := press(New(threeItems()), KeySpace, KeyDown, KeySpace, KeyEnter, KeyCancel)

	if !m.Done() || !m.Cancelled() {
		t.Fatalf("done=%t cancelled=%t, want a cancellation", m.Done(), m.Cancelled())
	}
	if got := m.Selectors(); len(got) != 0 {
		t.Errorf("selectors = %q, want none", got)
	}
}

// An unbound key at the offer is not an answer. Treating anything but y or n as one would
// make a stray keystroke decide whether the manifest adopts future items.
func TestAnUnboundKeyAtTheOfferIsNotAnAnswer(t *testing.T) {
	t.Parallel()

	m := press(New(threeItems()), KeySpace, KeyDown, KeySpace, KeyEnter)
	if got := m.Update(KeySpace); got.Done() {
		t.Error("space answered the offer")
	}
	if got := m.Update(KeyEnter); got.Done() {
		t.Error("enter answered the offer")
	}
}
