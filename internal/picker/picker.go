// Package picker is the multi-select `graft add` shows when an invocation names no
// selectors and there is a terminal to show it on.
//
// It chooses selectors and has no other powers. It returns a list of strings; it never
// reads a manifest, writes a file, resolves a rev, or computes a destination of its own —
// the destinations it shows are handed to it, computed by internal/plan, which is what
// keeps the interactive path from becoming a second implementation of the tool.
//
// The model is a value and every behavior is a function of it and one key, so the whole
// widget is tested by pressing keys at a struct. What is left — raw mode, decoding bytes
// into keys, writing frames — is the driver in run.go, which a bytes.Reader reaches every
// line of but term.MakeRaw itself.
package picker

import (
	"slices"
)

// Item is one thing a source offers: its id, its kind, and where its files would land.
type Item struct {
	ID           string
	Kind         string
	Destinations []string
}

// Key is a key press the model understands. Everything else is KeyNone, which the model
// ignores: a user pressing an unbound key has not made a mistake worth stopping for.
type Key int

// The keys the picker binds. KeyNone is every other key, and the model ignores it.
const (
	KeyNone Key = iota
	KeyUp
	KeyDown
	KeySpace
	KeyAll
	KeyEnter
	KeyCancel
	KeyYes
	KeyNo
)

// phase is which screen the model is showing.
type phase int

const (
	phaseList phase = iota
	phaseOffer
	phaseDone
	phaseCancelled
)

// Model is the picker's whole state. Update returns a new one rather than mutating, so a
// test can hold the model from before a key and compare.
type Model struct {
	items    []Item
	selected []bool
	cursor   int

	// offers are the kinds still to be asked about, in catalog order, and accepted holds
	// the ones the user collapsed to kind:*.
	offers   []string
	accepted []string

	phase phase

	// title is the heading the list opens with — the source header `--list` prints.
	title string

	// height is how many item lines the list may occupy, and top the first item shown.
	height int
	top    int
}

// defaultHeight is the window used when the terminal's own height is unknown. It is a
// number of *item* lines, so a short terminal scrolls rather than printing a list that
// runs off the top of it.
const defaultHeight = 10

// New builds a model over the items a source offers, in the order they are to be shown.
func New(items []Item) Model {
	return Model{
		items:    slices.Clone(items),
		selected: make([]bool, len(items)),
		height:   defaultHeight,
	}
}

// WithHeight returns the model windowed to n item lines. A non-positive n keeps the
// default, so a terminal whose size cannot be read is not a picker that renders nothing.
func (m Model) WithHeight(n int) Model {
	if n > 0 {
		m.height = n
		m = m.scroll()
	}
	return m
}

// Update applies one key press and returns the model it produces.
func (m Model) Update(k Key) Model {
	if m.phase == phaseDone || m.phase == phaseCancelled {
		return m
	}
	if k == KeyCancel {
		m.phase = phaseCancelled
		return m
	}
	if m.phase == phaseOffer {
		return m.offer(k)
	}
	return m.list(k)
}

// list applies a key on the item list.
func (m Model) list(k Key) Model {
	switch k {
	case KeyUp:
		if m.cursor > 0 {
			m.cursor--
			m = m.scroll()
		}
	case KeyDown:
		if m.cursor < len(m.items)-1 {
			m.cursor++
			m = m.scroll()
		}
	case KeySpace:
		if len(m.items) > 0 {
			m.selected = slices.Clone(m.selected)
			m.selected[m.cursor] = !m.selected[m.cursor]
		}
	case KeyAll:
		// One key that both selects everything and clears everything: with every item
		// already selected, the only thing `a` can usefully mean is the reverse.
		all := len(m.items) > 0 && !slices.Contains(m.selected, false)
		m.selected = make([]bool, len(m.items))
		for i := range m.selected {
			m.selected[i] = !all
		}
	case KeyEnter:
		// An empty confirmation is a cancellation: an add with no selectors would append a
		// source whose install list the next parse refuses, so the refusal belongs here
		// rather than after the picker has already succeeded.
		if len(m.chosen()) == 0 {
			m.phase = phaseCancelled
			return m
		}
		if m.offers = m.wholeKinds(); len(m.offers) > 0 {
			m.phase = phaseOffer
			return m
		}
		m.phase = phaseDone
	}
	return m
}

// offer applies a key on the collapse offer.
func (m Model) offer(k Key) Model {
	switch k {
	case KeyYes:
		m.accepted = append(slices.Clone(m.accepted), m.offers[0])
		m.offers = slices.Clone(m.offers[1:])
	case KeyNo:
		m.offers = slices.Clone(m.offers[1:])
	default:
		return m
	}
	if len(m.offers) == 0 {
		m.phase = phaseDone
	}
	return m
}

// scroll keeps the cursor inside the window.
func (m Model) scroll() Model {
	switch {
	case m.cursor < m.top:
		m.top = m.cursor
	case m.cursor >= m.top+m.height:
		m.top = m.cursor - m.height + 1
	}
	if m.top < 0 {
		m.top = 0
	}
	return m
}

// Done reports whether the picker has finished, by confirmation or cancellation.
func (m Model) Done() bool { return m.phase == phaseDone || m.phase == phaseCancelled }

// Cancelled reports whether it finished without a selection to install.
func (m Model) Cancelled() bool { return m.phase == phaseCancelled }

// Selectors is what was chosen: the selectors to write into graft.toml, ordered by kind as
// the catalog orders them and by id within a kind, so one selection always produces one
// manifest. A cancelled picker chose nothing.
func (m Model) Selectors() []string {
	if m.phase != phaseDone {
		return nil
	}
	var out []string
	for _, kind := range m.kinds() {
		if slices.Contains(m.accepted, kind) {
			out = append(out, kind+":*")
			continue
		}
		var ids []string
		for i, it := range m.items {
			if m.selected[i] && it.Kind == kind {
				ids = append(ids, it.ID)
			}
		}
		slices.Sort(ids)
		out = append(out, ids...)
	}
	return out
}

// chosen is the ids selected right now, in catalog order, whatever the phase.
func (m Model) chosen() []string {
	var out []string
	for i, it := range m.items {
		if m.selected[i] {
			out = append(out, it.ID)
		}
	}
	return out
}

// kinds is every kind in the order the catalog first named it.
func (m Model) kinds() []string {
	var out []string
	for _, it := range m.items {
		if !slices.Contains(out, it.Kind) {
			out = append(out, it.Kind)
		}
	}
	return out
}

// wholeKinds is the kinds whose every item is selected and which have more than one item.
//
// A kind with one item is never offered: `agent:*` and `agent:reviewer` name the same thing
// today and different things tomorrow, and offering that as a formality would teach the
// user to accept an offer without reading it.
func (m Model) wholeKinds() []string {
	var out []string
	for _, kind := range m.kinds() {
		count, chosen := 0, 0
		for i, it := range m.items {
			if it.Kind != kind {
				continue
			}
			count++
			if m.selected[i] {
				chosen++
			}
		}
		if count > 1 && count == chosen {
			out = append(out, kind)
		}
	}
	return out
}

// Glob is the selector a collapsed kind becomes. It is spelled here so the model, the
// offer's own text, and any future caller cannot disagree about it.
func Glob(kind string) string { return kind + ":*" }
