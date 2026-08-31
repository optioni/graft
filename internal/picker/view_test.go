package picker

import (
	"strings"
	"testing"
)

// itemLines is the rendered lines that carry an item, which is what a test about the list
// wants to assert without also pinning the help text's wording.
func itemLines(m Model) []string {
	var out []string
	for _, l := range m.View() {
		if strings.Contains(l, ":") && (strings.Contains(l, "[ ]") || strings.Contains(l, "[x]")) {
			out = append(out, l)
		}
	}
	return out
}

func TestViewNamesEveryItemAndItsDestination(t *testing.T) {
	t.Parallel()

	lines := itemLines(New(threeItems()))
	if len(lines) != 3 {
		t.Fatalf("item lines = %d, want 3:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for i, want := range []string{
		"agent:planner   .claude/agents/planner.md",
		"agent:reviewer  .claude/agents/reviewer.md",
		"schema:tdd      openspec/schemas/tdd/",
	} {
		if !strings.HasSuffix(lines[i], want) {
			t.Errorf("line %d = %q, want it to end with %q — ids padded to a common width", i, lines[i], want)
		}
	}
}

func TestViewMarksTheCursorAndTheSelection(t *testing.T) {
	t.Parallel()

	m := press(New(threeItems()), KeyDown, KeySpace)
	lines := itemLines(m)

	if !strings.Contains(lines[1], "[x]") {
		t.Errorf("the selected item is not marked: %q", lines[1])
	}
	if strings.Contains(lines[0], "[x]") || strings.Contains(lines[2], "[x]") {
		t.Errorf("an unselected item is marked:\n%s", strings.Join(lines, "\n"))
	}
	// The cursor's own mark is distinguishable from the selection's, or a user cannot tell
	// what space is about to toggle.
	cursor := strings.TrimSuffix(strings.SplitN(lines[1], "[", 2)[0], " ")
	if cursor == "" {
		t.Errorf("the cursor's line carries no cursor mark: %q", lines[1])
	}
	if other := strings.SplitN(lines[0], "[", 2)[0]; strings.Contains(other, cursor) {
		t.Errorf("a line without the cursor carries the cursor mark: %q", lines[0])
	}
}

func TestViewNamesEveryDestinationOfAnItem(t *testing.T) {
	t.Parallel()

	m := New([]Item{{
		ID:           "schema:tdd",
		Kind:         "schema",
		Destinations: []string{"openspec/schemas/tdd/", "vendor/schemas/tdd/"},
	}})
	line := itemLines(m)[0]
	if !strings.HasSuffix(line, "openspec/schemas/tdd/, vendor/schemas/tdd/") {
		t.Errorf("line = %q, want both destinations separated by a comma", line)
	}
}

// A catalog longer than the terminal scrolls rather than running off the top of it, and the
// cursor is always on screen — a cursor a user cannot see is a selection they cannot aim.
func TestViewWindowsALongListAroundTheCursor(t *testing.T) {
	t.Parallel()

	var many []Item
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		many = append(many, Item{ID: "agent:" + name, Kind: "agent", Destinations: []string{".claude/agents/" + name + ".md"}})
	}

	m := New(many).WithHeight(3)
	if got := len(itemLines(m)); got != 3 {
		t.Fatalf("item lines = %d, want the window's 3", got)
	}
	m = press(m, KeyDown, KeyDown, KeyDown, KeyDown)
	lines := itemLines(m)
	if len(lines) != 3 {
		t.Fatalf("item lines = %d, want 3 after scrolling", len(lines))
	}
	if !strings.Contains(strings.Join(lines, "\n"), "agent:e") {
		t.Errorf("the cursor's item is not on screen:\n%s", strings.Join(lines, "\n"))
	}
}

// The title is the source header `--list` prints, so the two views of one catalog open with
// the same line.
func TestViewOpensWithTheTitleItWasGiven(t *testing.T) {
	t.Parallel()

	m := New(threeItems()).WithTitle("shared  v1.0.0  (af3863a)")
	if got := m.View()[0]; got != "shared  v1.0.0  (af3863a)" {
		t.Errorf("first line = %q, want the title", got)
	}
}

// The offer states what the choice means rather than asking for confirmation: the whole
// point is that kind:* and an explicit list differ in the future, not today.
func TestOfferViewSaysWhatTheGlobMeans(t *testing.T) {
	t.Parallel()

	m := press(New(threeItems()), KeySpace, KeyDown, KeySpace, KeyEnter)
	if m.Done() {
		t.Fatal("selecting every agent did not open the offer")
	}
	view := strings.Join(m.View(), "\n")
	for _, want := range []string{"agent:*", "adds later"} {
		if !strings.Contains(view, want) {
			t.Errorf("the offer does not mention %q:\n%s", want, view)
		}
	}
}
