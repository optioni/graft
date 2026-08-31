package manifest

import (
	"fmt"
	"strings"
)

// AddInstall returns data with selectors added to source name's install array, every
// other byte of the file left exactly as it was, and the array's own formatting kept.
//
// The insertion point is immediately after the array's last element, and after that
// element's trailing comma when it has one — not before the closing bracket. Anchoring on
// the last element makes the insertion point a token rather than a shape, so a comment or
// a blank line a consumer left at the end of the array survives untouched, and no rule is
// needed for either.
//
// The one existing byte this may add is a comma on an element that had none, because an
// array whose last element carries no comma cannot gain a sibling without one. Everything
// else it cannot rewrite exactly is refused, for the reason SetRev is: graft.toml is a
// file graft did not write, and a wrong guess corrupts the consumer's own request while
// looking like success.
//
// Selectors the array already holds are not added again, and an amendment that adds
// nothing returns the original bytes rather than an equivalent re-rendering.
//
// AddInstall creates, modifies, and deletes nothing, and returns no bytes at all on any
// error.
func AddInstall(data []byte, name string, selectors []string) ([]byte, error) {
	for _, sel := range selectors {
		if err := checkLiteral("selector", sel); err != nil {
			return nil, err
		}
	}

	text := string(data)
	lo, hi, ok := sourceTableSpan(text, name)
	if !ok {
		return nil, cannotAmend(name)
	}
	arr, err := findInstall(text, lo, hi, name)
	if err != nil {
		return nil, err
	}

	// Deduplicated against what the array holds and against itself, in the order given,
	// because a manifest declaring one selector twice is one manifest.Parse refuses.
	held := make(map[string]struct{}, len(arr.elements))
	for _, e := range arr.elements {
		held[e] = struct{}{}
	}
	add := make([]string, 0, len(selectors))
	for _, sel := range selectors {
		if _, dup := held[sel]; dup {
			continue
		}
		held[sel] = struct{}{}
		add = append(add, sel)
	}
	if len(add) == 0 {
		return append([]byte(nil), data...), nil
	}

	at, insert := arr.insertion(add)
	return []byte(text[:at] + insert + text[at:]), nil
}

// installArray is where one source's install array is and what shape it was written in.
type installArray struct {
	open       int      // the offset of "["
	close      int      // the offset of "]"
	lastEnd    int      // the offset just past the last element's closing quote, or -1
	afterComma int      // the offset just past the last element's trailing comma, or -1
	indent     string   // the leading whitespace of the line the last element sat on
	multiline  bool     // whether "[" and "]" are on different lines
	elements   []string // each element's content, in order
}

// insertion returns where to write and what to write there.
func (a installArray) insertion(add []string) (int, string) {
	quotedAll := make([]string, 0, len(add))
	for _, sel := range add {
		quotedAll = append(quotedAll, quoted(sel))
	}

	// An array with no element yet: everything goes straight after the bracket, which
	// keeps a one-line array on one line and leaves a multi-line one valid.
	if a.lastEnd < 0 {
		return a.open + 1, strings.Join(quotedAll, ", ")
	}

	if !a.multiline {
		if a.afterComma >= 0 {
			return a.afterComma, " " + strings.Join(quotedAll, ", ")
		}
		return a.lastEnd, ", " + strings.Join(quotedAll, ", ")
	}

	var b strings.Builder
	at := a.afterComma
	if at < 0 {
		// The comma the previous element never had. It is written at that element's own
		// line end, which is the one existing line this may touch.
		at = a.lastEnd
		b.WriteString(",")
	}
	for i, q := range quotedAll {
		b.WriteString("\n" + a.indent + q)
		// The array's own style: every element carries a comma, or the last one does not.
		if a.afterComma >= 0 || i < len(quotedAll)-1 {
			b.WriteString(",")
		}
	}
	return at, b.String()
}

// findInstall locates the install array in the body of a source's table, between lo and
// hi, and describes its shape. Every shape it cannot rewrite exactly is one refusal.
func findInstall(text string, lo, hi int, name string) (installArray, error) {
	open := "" // the multi-line delimiter still to be closed, or "" outside every string
	for pos := lo; pos < hi; {
		line, next := lineAt(text, pos)
		raw := strings.TrimSuffix(line, "\r")

		// Inside a multi-line string these bytes are a value, and an `install = [` written
		// in one belongs to whichever key opened it.
		if open != "" {
			open = continueString(raw, open)
			pos = next
			continue
		}
		if trimmed := strings.TrimSpace(raw); trimmed == "" || strings.HasPrefix(trimmed, "#") {
			// A commented-out install is not the key, exactly as a commented-out rev is
			// not the key the pin move finds.
			pos = next
			continue
		}
		if eq := keyAssignment(line, installKey); eq >= 0 {
			return scanArray(text, pos+eq, hi, name)
		}
		open = openString(raw)
		pos = next
	}
	return installArray{}, cannotAmend(name)
}

// scanArray reads the array beginning at or after from, collecting its elements and the
// offsets the insertion needs. Only single-line quoted strings with no escape in them are
// admitted: an escaped element cannot be compared to a selector without decoding it, and
// an amendment that mis-compares adds a duplicate the next parse refuses.
func scanArray(text string, from, hi int, name string) (installArray, error) {
	fail := func() (installArray, error) { return installArray{}, cannotAmend(name) }

	i := skipBlank(text, from, hi)
	if i >= hi || text[i] != '[' {
		return fail()
	}
	a := installArray{open: i, lastEnd: -1, afterComma: -1}

	i++
	wantElement := true
	for {
		i = skipBlank(text, i, hi)
		if i >= hi {
			return fail()
		}
		switch c := text[i]; c {
		case '#':
			// A comment runs to the end of its line and is not an element.
			for i < hi && text[i] != '\n' {
				i++
			}
		case ']':
			a.close = i
			a.multiline = strings.Contains(text[a.open:a.close], "\n")
			return a, nil
		case ',':
			if wantElement {
				// A comma with no element before it, or two in a row.
				return fail()
			}
			a.afterComma = i + 1
			wantElement = true
			i++
		case '"', '\'':
			if !wantElement {
				// Two elements with no comma between them.
				return fail()
			}
			if strings.HasPrefix(text[i:], strings.Repeat(string(c), 3)) {
				return fail()
			}
			line, _ := lineAt(text, i)
			rel := closingQuote(line, 0)
			if rel < 0 {
				return fail()
			}
			content := line[1:rel]
			if strings.ContainsAny(content, "\\") {
				return fail()
			}
			a.elements = append(a.elements, content)
			a.lastEnd = i + rel + 1
			a.afterComma = -1
			a.indent = lineIndent(text, i)
			wantElement = false
			i = a.lastEnd
		default:
			// A number, a boolean, a nested array, an inline table: not a selector, and
			// not something this may rewrite around.
			return fail()
		}
	}
}

// skipBlank advances past spaces, tabs, carriage returns, and newlines.
func skipBlank(text string, i, hi int) int {
	for i < hi {
		switch text[i] {
		case ' ', '\t', '\r', '\n':
			i++
		default:
			return i
		}
	}
	return hi
}

// lineIndent is the leading whitespace of the line the offset sits on.
func lineIndent(text string, at int) string {
	start := strings.LastIndexByte(text[:at], '\n') + 1
	line := text[start:at]
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// cannotAmend is the one refusal for every shape AddInstall declines to rewrite. A table
// that is not found, a key that is not there, and a value that is not a plain array all
// produce it: "install is not a plain array of strings under [sources.x]" is true of each,
// and one message is the shape SPEC.md's failure-mode table takes.
func cannotAmend(name string) error {
	return fmt.Errorf("%s: source %q: cannot amend %s: %s is not a plain array of strings under [sources.%s]",
		Filename, name, installKey, installKey, name)
}
