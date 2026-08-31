package manifest

import "strings"

// sourceTableSpan returns the byte range of the body of [sources.<name>] — from the start
// of the line after its header to the start of the next table header, or the end of the
// text — and whether that table was found at all.
//
// It is the line walk SetRev performs, lifted out so that every edit in this package
// agrees about where a source's table begins and ends. The two properties that matter are
// the ones SetRev's own comments give reasons for: a `[sources.b]` written inside a
// multi-line string is a value rather than a header, and *any* header ends the previous
// table — stopping only at the next `[sources.` header would walk straight into
// [sources.<name>.kinds], where a kind may legally be named `rev` or `install`.
func sourceTableSpan(text, name string) (lo, hi int, ok bool) {
	lo = -1
	open := "" // the multi-line delimiter still to be closed, or "" outside every string
	for pos := 0; pos < len(text); {
		line, next := lineAt(text, pos)
		raw := strings.TrimSuffix(line, "\r")

		if open != "" {
			open = continueString(raw, open)
			pos = next
			continue
		}

		if trimmed := strings.TrimSpace(raw); strings.HasPrefix(trimmed, "[") {
			if lo >= 0 {
				return lo, pos, true
			}
			if isSourceTable(trimmed, name) {
				lo = next
			}
		}
		open = openString(raw)
		pos = next
	}
	if lo >= 0 {
		return lo, len(text), true
	}
	return 0, 0, false
}
