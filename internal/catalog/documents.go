package catalog

import (
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/token"
)

// documents counts the YAML documents in src.
//
// The count is taken from the token stream rather than from the decoder, which looks like
// the long way round and is not. A decoder cannot report what it declined to read, and
// this one stops early on inputs that do hold a second document: after the first document
// it returns io.EOF whenever two markers are adjacent, so "version: 1 --- --- kinds: b"
// decodes to the first document with a nil error and kind b silently gone — the exact
// failure the single-document rule exists to prevent. The mirror case, a file opening with
// two markers, decodes to nothing and would be reported as a missing version.
//
// Counting tokens also answers for a file the decoder cannot parse at all. The lexer is
// error tolerant, so content after a marker is reported as the extra document it is rather
// than as a syntax error inside a document graft is about to say should not be there.
//
// A marker splits the stream into regions. A region holding anything but comments is a
// document, and so is an empty region between two markers — an empty document is still a
// document, and what follows it is content the decoder drops. An empty region before the
// first marker or after the last is not, which is what makes a leading marker, a trailing
// marker, and both together one document rather than two or three.
func documents(src []byte) int {
	// content[i] reports whether region i holds anything but comments and directives;
	// directive[i] reports whether it holds a directive at all.
	content := []bool{false}
	directive := []bool{false}
	// directiveLine holds the line a directive is being read from, or 0 when none is.
	// Lines are numbered from 1, so 0 matches nothing.
	directiveLine := 0
	for _, t := range lexer.Tokenize(string(src)) {
		switch t.Type {
		case token.DocumentHeaderType, token.DocumentEndType:
			content = append(content, false)
			directive = append(directive, false)
			directiveLine = 0
		case token.CommentType:
			// Not content: a comment after a trailing marker discards nothing.
		case token.DirectiveType:
			// Only the directive's own line is set aside, and a directive is one line
			// by definition: its arguments arrive as ordinary tokens and are what would
			// otherwise be read as content. Anything on a later line is content in a
			// region the marker really did open.
			directive[len(directive)-1] = true
			directiveLine = t.Position.Line
		default:
			if t.Position.Line != directiveLine {
				content[len(content)-1] = true
			}
		}
	}

	n := 0
	last := len(content) - 1
	for i, held := range content {
		// A directives prologue such as "%YAML 1.2" is closed by a "---" that opens
		// the one document rather than ending a document before it, which is why a
		// region holding nothing else is not a document. That reading depends on the
		// marker: in the last region no marker follows, so nothing there is a prologue
		// and the directive is content after a separator like any other.
		if i == last && directive[i] {
			held = true
		}
		if held || (i > 0 && i < last) {
			n++
		}
	}
	return n
}
