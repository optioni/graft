package manifest

import (
	"fmt"
	"strings"
	"unicode"
)

// revKey is the one key SetRev is allowed to move.
const revKey = "rev"

// SetRev returns data with the value of source name's rev replaced by rev, and every other
// byte of the file left exactly as it was.
//
// It edits text rather than re-serializing a parsed manifest, and that is the whole design.
// graft.toml is written by a human and reviewed in a diff: decoding and re-encoding would
// delete every comment, normalize the alignment SPEC.md's own example uses, and reorder keys,
// turning a one-line pin move into a whole-file change nobody can read. So the edit is
// surgical — the span between the value's quotation marks and nothing else — and any shape it
// cannot rewrite exactly is refused rather than guessed at.
//
// The refusal matters more than the rewrite. A wrong guess corrupts the consumer's own
// request, and a wrong *target* — an edited comment, an edited sub-table key — is worse than a
// refusal, because it looks like success while the run resolves the old rev.
//
// SetRev creates, modifies, and deletes nothing. It returns bytes; internal/apply is the only
// package that puts them anywhere. On any error it returns no bytes at all, so no caller can
// write a half-edited file.
func SetRev(data []byte, name, rev string) ([]byte, error) {
	if err := checkLiteral(revKey, rev); err != nil {
		return nil, err
	}

	text := string(data)
	lo, hi, ok := sourceTableSpan(text, name)
	if !ok {
		return nil, cannotMove(name)
	}

	open := "" // the multi-line delimiter still to be closed, or "" outside every string
	for pos := lo; pos < hi; {
		line, next := lineAt(text, pos)
		raw := strings.TrimSuffix(line, "\r")

		// Inside a multi-line string these bytes are a value, not syntax. A `rev = "…"`
		// written inside one belongs to whichever key opened the string — so a scanner that
		// read it as a key would edit a line in a completely different key's value, which is
		// the one failure this function exists to make impossible.
		if open != "" {
			open = continueString(raw, open)
			pos = next
			continue
		}

		// The key search is on the trimmed line and the value edit on the raw one: the first
		// may not be fooled by indentation, the second may not disturb a byte.
		if trimmed := strings.TrimSpace(raw); trimmed == "" || strings.HasPrefix(trimmed, "#") {
			// A commented-out `rev` is not the key. Skipping comments here is what keeps
			// `# rev = "v0.9.0"` above the real key from being the line that moves.
			pos = next
			continue
		}
		if eq := keyAssignment(line, revKey); eq >= 0 {
			lo, hi, ok := quotedSpan(line, eq)
			if !ok {
				// The key is here and its value is not something this can rewrite exactly —
				// a multi-line string, a bare value, an unterminated one. Refusing beats
				// continuing to a later `rev` in another table.
				return nil, cannotMove(name)
			}
			return []byte(text[:pos+lo] + rev + text[pos+hi:]), nil
		}
		open = openString(raw)
		pos = next
	}
	return nil, cannotMove(name)
}

// openString reports the multi-line delimiter the line leaves open, or "" when the line ends
// outside every string.
//
// It is a scan rather than a parse, and its errors are all in the safe direction: a state it
// gets wrong makes SetRev skip a line, and a skipped line produces the refusal. Reading a
// line it should have skipped is the failure that cannot be allowed, and that is the one
// this closes.
func openString(line string) string {
	for i := 0; i < len(line); {
		switch c := line[i]; c {
		case '#':
			// Outside every string, so the rest of the line is a comment.
			return ""
		case '"', '\'':
			delim := strings.Repeat(string(c), 3)
			if strings.HasPrefix(line[i:], delim) {
				j := strings.Index(line[i+len(delim):], delim)
				if j < 0 {
					return delim
				}
				i += len(delim) + j + len(delim)
				continue
			}
			j := closingQuote(line, i)
			if j < 0 {
				// A single-line string with no closing quote. TOML has no such value and
				// a line-oriented scan cannot recover, so the rest of the line is left
				// alone rather than guessed at.
				return ""
			}
			i = j + 1
		default:
			i++
		}
	}
	return ""
}

// continueString consumes a line inside an open multi-line string and returns the delimiter
// still open after it.
func continueString(line, open string) string {
	i := strings.Index(line, open)
	if i < 0 {
		return open
	}
	return openString(line[i+len(open):])
}

// closingQuote returns the index of the quotation mark closing the single-line string opened
// at i, or -1 when the line ends first. A backslash escapes inside a basic string only; a
// literal string has no escapes at all.
func closingQuote(line string, i int) int {
	q := line[i]
	for j := i + 1; j < len(line); j++ {
		if q == '"' && line[j] == '\\' {
			j++
			continue
		}
		if line[j] == q {
			return j
		}
	}
	return -1
}

// checkLiteral refuses a value that cannot be written literally into a TOML string.
//
// A quotation mark of either kind would close the string it is written into, and everything
// after it becomes manifest syntax: a newline and `install = []` appended to a rev is a key
// the consumer never wrote. A backslash starts an escape, and a control character is invalid
// inside a TOML basic string. None of the three can appear in a git ref, so a value that would
// have to be escaped to be written is a value that was not a rev — which is why it is refused
// rather than escaped.
//
// unicode.IsControl rather than a `< 0x20` test: DEL and the C1 range are invalid inside a
// TOML basic string too.
//
// key names the value in the message — `rev`, `git`, `selector` — because the append writes
// three kinds of value and a message that named none of them would leave the reader hunting.
// The wording for `rev` is byte-identical to the one this refusal has always produced.
func checkLiteral(key, value string) error {
	for _, r := range value {
		if r == '"' || r == '\'' || r == '\\' || unicode.IsControl(r) {
			return fmt.Errorf("%s: %s %q contains a quote, a backslash, or a control character", Filename, key, value)
		}
	}
	return nil
}

// cannotMove is the one refusal for every shape SetRev declines to rewrite. A table that is
// not found and a `rev` that is not a plain key produce the same message: "rev is not a plain
// key under [sources.x]" is true of both, and one message is the shape of SPEC.md's
// failure-mode table.
func cannotMove(name string) error {
	return fmt.Errorf("%s: source %q: cannot move the pin: %s is not a plain key under [sources.%s]",
		Filename, name, revKey, name)
}

// lineAt returns the line beginning at pos, without its terminator, and the offset of the
// next line. The terminator is left in the text rather than normalized, so a CRLF file stays
// a CRLF file and a file with no final newline does not gain one.
func lineAt(text string, pos int) (string, int) {
	if i := strings.IndexByte(text[pos:], '\n'); i >= 0 {
		return text[pos : pos+i], pos + i + 1
	}
	return text[pos:], len(text)
}

// isSourceTable reports whether header is the standard table of source name.
//
// The key path is compared exactly — `sources`, then the name — so `[sources.x.kinds]` is a
// different table and `[[sources.x]]` is not a standard table at all.
func isSourceTable(header, name string) bool {
	header = strings.TrimSpace(stripComment(header))
	if strings.HasPrefix(header, "[[") || !strings.HasPrefix(header, "[") || !strings.HasSuffix(header, "]") {
		return false
	}
	parts, ok := splitKeyPath(header[1 : len(header)-1])
	return ok && len(parts) == 2 && parts[0] == "sources" && parts[1] == name
}

// stripComment cuts a line at the first `#` that is not inside a quoted string, so a header
// carrying a trailing comment is still recognised and a `#` inside a quoted key is not
// mistaken for one. It is used on header lines only: a value's own `#` is inside its quotes,
// and quotedSpan finds the value's end at its closing quote rather than at a comment.
func stripComment(s string) string {
	var quote byte
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#':
			return s[:i]
		}
	}
	return s
}

// splitKeyPath splits a dotted TOML key path, unquoting each part. A part that is quoted has
// its quotes removed; whitespace around a part is not significant and is dropped.
//
// A name whose quoted form carries significant leading or trailing whitespace does not match
// here and produces the refusal. That is the fail-closed direction: a missed match refuses,
// while a loose match edits the wrong line.
func splitKeyPath(s string) ([]string, bool) {
	var (
		parts []string
		cur   strings.Builder
		quote byte
	)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
				continue
			}
			cur.WriteByte(c)
		case c == '"' || c == '\'':
			quote = c
		case c == '.':
			parts = append(parts, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if quote != 0 {
		return nil, false
	}
	return append(parts, strings.TrimSpace(cur.String())), true
}

// keyAssignment returns the offset just past the `=` of a line assigning key, or -1 when the
// line is not one. Leading whitespace is tolerated; anything else between the key and the `=`
// — a dot, another character — is not this key.
func keyAssignment(line, key string) int {
	i := skipSpace(line, 0)
	if !strings.HasPrefix(line[i:], key) {
		return -1
	}
	j := skipSpace(line, i+len(key))
	if j >= len(line) || line[j] != '=' {
		return -1
	}
	return j + 1
}

// quotedSpan returns the offsets of the value's content — after the opening quote and before
// the closing one — for a single-line quoted string starting at or after from.
//
// The value's end is its closing quotation mark, never the first `#` on the line and never the
// end of the line. Cutting at `#` would corrupt a rev containing one, which git permits, and
// rebuilding the line from everything before `=` would delete a comment trailing the value.
//
// A TOML multi-line string is refused rather than rewritten: manifest.Parse accepts one, and a
// line-oriented edit would strand its continuation and leave a graft.toml that no longer
// parses.
func quotedSpan(line string, from int) (int, int, bool) {
	i := skipSpace(line, from)
	if i >= len(line) {
		return 0, 0, false
	}
	q := line[i]
	if q != '"' && q != '\'' {
		return 0, 0, false
	}
	if strings.HasPrefix(line[i:], strings.Repeat(string(q), 3)) {
		return 0, 0, false
	}

	j := closingQuote(line, i)
	if j < 0 {
		return 0, 0, false
	}
	return i + 1, j, true
}

func skipSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}
