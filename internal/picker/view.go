package picker

import (
	"fmt"
	"strings"

	"github.com/optioni/graft/internal/ui"
)

// helpLine is the one line of instructions the list carries. It names every key the model
// binds, because a key binding a user cannot discover is a key binding that does not exist.
const helpLine = "space toggles · a all · enter confirms · q cancels"

// View renders the current screen as lines, without a trailing newline on any of them.
//
// It is a pure function of the model, which is what lets a test assert the picker's
// appearance without a terminal. The driver is what turns these lines into a frame.
func (m Model) View() []string {
	if m.phase == phaseOffer {
		return m.offerView()
	}
	return m.listView()
}

// WithTitle returns the model with a heading line — the same header `graft add --list`
// prints for a source, so the two views of one catalog open the same way.
func (m Model) WithTitle(title string) Model {
	m.title = title
	return m
}

func (m Model) listView() []string {
	var out []string
	if m.title != "" {
		out = append(out, m.title, "")
	}

	var idWidth int
	for _, it := range m.items {
		idWidth = max(idWidth, len(it.ID))
	}

	end := min(m.top+m.height, len(m.items))
	for i := m.top; i < end; i++ {
		it := m.items[i]
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		box := "[ ]"
		if m.selected[i] {
			box = "[x]"
		}
		out = append(out, cursor+box+" "+it.ID+ui.Pad(len(it.ID), idWidth)+"  "+strings.Join(it.Destinations, ", "))
	}

	// Said only when there is something off screen, so the ordinary case is not carrying a
	// line about scrolling that never changes.
	if len(m.items) > m.height {
		out = append(out, "", fmt.Sprintf("showing %d-%d of %d", m.top+1, end, len(m.items)))
	}
	return append(out, "", helpLine)
}

// offerView asks about one kind: what the glob means, what the explicit list means, and
// nothing about whether the user is sure.
func (m Model) offerView() []string {
	kind := m.offers[0]
	ids := 0
	for _, it := range m.items {
		if it.Kind == kind {
			ids++
		}
	}
	return []string{
		fmt.Sprintf("Every %s is selected.", kind),
		"",
		fmt.Sprintf("  %s%s adopts every %s this source adds later", Glob(kind), ui.Pad(len(Glob(kind)), 14), kind),
		fmt.Sprintf("  the %d ids%s install exactly what you chose", ids, ui.Pad(len(fmt.Sprintf("the %d ids", ids)), 14)),
		"",
		fmt.Sprintf("Write %s instead of the ids?  y/n", Glob(kind)),
	}
}
