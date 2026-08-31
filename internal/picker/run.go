package picker

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Terminal is everything the driver needs from the process: where keys come from, where
// frames go, and how to put the terminal into raw mode.
//
// All three are values rather than things this package looks up, which is what lets every
// line below be reached from a bytes.Reader and a no-op raw-mode function. The one line
// that cannot be tested that way is term.MakeRaw itself, and it lives in the caller.
type Terminal struct {
	In  io.Reader
	Out io.Writer

	// MakeRaw puts the terminal into raw mode and returns the function that restores it.
	// A nil MakeRaw means the caller has already arranged whatever mode it wants.
	MakeRaw func() (restore func(), err error)
}

// Run drives the model to a decision and returns the selectors it chose.
//
// No selectors and no error is a cancellation — the user pressed q, esc, or ctrl-c, or
// confirmed an empty selection, or the input ended. The three are one outcome on purpose:
// each means there is nothing to write, and a caller that distinguished them would have to
// invent a difference the user did not make.
//
// Raw mode is restored on every return path, including a write failure and a cancellation.
// The last frame is erased before returning, so whatever the caller prints next is not
// printed underneath a list of checkboxes.
func Run(t Terminal, m Model) ([]string, error) {
	if t.MakeRaw != nil {
		restore, err := t.MakeRaw()
		if err != nil {
			return nil, err
		}
		defer restore()
	}

	keys := bufio.NewReader(t.In)
	drawn := 0
	for !m.Done() {
		n, err := t.draw(m.View(), drawn)
		drawn = n
		if err != nil {
			return nil, err
		}
		k, err := readKey(keys)
		if err != nil {
			// A stream that has ended has no more choices in it. Cancelling is the only
			// honest reading: hanging would wait for a key that cannot come, and treating
			// it as a confirmation would install whatever happened to be highlighted.
			m = m.Update(KeyCancel)
			break
		}
		m = m.Update(k)
	}
	if err := t.erase(drawn); err != nil {
		return nil, err
	}
	return m.Selectors(), nil
}

// draw writes one frame over the previous one and returns the number of lines it wrote.
//
// Raw mode turns off the terminal's own line discipline, so each line ends with a carriage
// return as well as a newline — without it every line would start where the last one ended.
func (t Terminal) draw(lines []string, prev int) (int, error) {
	var b strings.Builder
	b.WriteString(up(prev))
	b.WriteString(clearDown)
	for _, l := range lines {
		b.WriteString(l + "\r\n")
	}
	_, err := io.WriteString(t.Out, b.String())
	return len(lines), err
}

// erase removes the last frame, leaving the cursor where the picker started.
func (t Terminal) erase(drawn int) error {
	if drawn == 0 {
		return nil
	}
	_, err := io.WriteString(t.Out, up(drawn)+clearDown)
	return err
}

// clearDown erases from the cursor to the end of the screen.
const clearDown = "\x1b[J"

// up moves the cursor up n lines, or nowhere at all when there is nothing to move over.
func up(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("\x1b[%dA", n)
}

// readKey decodes one key press, returning KeyNone for anything the model does not bind.
//
// An escape sequence is read whole. Reading `esc [ B` as a bare escape and two letters
// would cancel the picker every time a user pressed the down arrow, which is the one
// decoding mistake that makes the widget unusable — hence the buffered-bytes test rather
// than a blocking read: a lone escape key arrives alone, while an arrow's three bytes
// arrive together.
func readKey(r *bufio.Reader) (Key, error) {
	c, err := r.ReadByte()
	if err != nil {
		return KeyNone, err
	}
	switch c {
	case ctrlC, 'q':
		return KeyCancel, nil
	case '\r', '\n':
		return KeyEnter, nil
	case ' ':
		return KeySpace, nil
	case 'a':
		return KeyAll, nil
	case 'k':
		return KeyUp, nil
	case 'j':
		return KeyDown, nil
	case 'y':
		return KeyYes, nil
	case 'n':
		return KeyNo, nil
	case esc:
		return escapeKey(r), nil
	}
	return KeyNone, nil
}

// escapeKey decodes what follows an escape byte. Nothing buffered means the escape key
// itself, which cancels; a sequence this does not recognise is an unbound key rather than
// a cancellation, because a terminal emitting one has not asked to stop.
func escapeKey(r *bufio.Reader) Key {
	if r.Buffered() == 0 {
		return KeyCancel
	}
	introducer, err := r.ReadByte()
	if err != nil || (introducer != '[' && introducer != 'O') || r.Buffered() == 0 {
		return KeyNone
	}
	final, err := r.ReadByte()
	if err != nil {
		return KeyNone
	}
	switch final {
	case 'A':
		return KeyUp
	case 'B':
		return KeyDown
	}
	return KeyNone
}

const (
	ctrlC = 3
	esc   = 0x1b
)
