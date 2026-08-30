package sync

import (
	"fmt"
	"strings"

	"github.com/optioni/graft/internal/ui"
)

// upToDateLine is what a sync with nothing to do prints, and the whole of what it prints.
// Output that appears when nothing happened trains the reader to stop reading it.
const upToDateLine = "up to date"

// Lines renders the report, one string per line, for the caller to write to the error
// stream. A summary is not machine-readable output, and SPEC.md sends both it and errors to
// stderr so a pipe is never corrupted by text a human was meant to read.
//
// u supplies the colour decision the whole run already took from stdout and NO_COLOR; this
// is not a second decision. Styling is applied after every width is computed, so enabling
// colour never moves a column.
func (r *Report) Lines(u *ui.UI) []string {
	if r.upToDate {
		return []string{upToDateLine}
	}

	var out []string
	for i, s := range r.Sources {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, s.header())
		// A source with a moved pin and no item lines is a header on its own. Emitting the
		// blank line unconditionally would put two in a row before the next block.
		if items := s.itemLines(u); len(items) > 0 {
			out = append(out, "")
			out = append(out, items...)
		}
	}
	if len(out) > 0 {
		out = append(out, "")
	}
	return append(out, r.summary())
}

// header names the source and what moved: `name  rev  (sha)`, with either half rendered as
// `old -> new` when it moved. A source the lock had never seen, and one being reported only
// because it is going away, have no previous half and render each once.
//
// The matched column appears only for a source whose rev is a range — which is exactly
// when Matched or PrevMatched is non-empty, since a ref's report never carries one — so a
// report containing no range is byte-identical to what graft printed before ranges
// existed. A range renders its own request unchanged, because a range does not move
// unless the consumer edits it, and shows the movement in the matched column instead.
func (s SourceReport) header() string {
	parts := []string{s.Name, transition(s.PrevRev, s.Rev)}
	if s.Matched != "" || s.PrevMatched != "" {
		parts = append(parts, transition(s.PrevMatched, s.Matched))
	}
	parts = append(parts, "("+transition(ui.ShortSHA(s.PrevResolved), ui.ShortSHA(s.Resolved))+")")
	return strings.Join(parts, "  ")
}

// transition renders `old -> new`, or just the new value when there is no old one or the
// two are the same.
func transition(before, after string) string {
	if before == "" || before == after {
		return after
	}
	return before + " -> " + after
}

// itemLines renders one source's block, aligned within that block.
//
// The verb and the id are padded and followed by two spaces. The count is padded and
// followed by two spaces only when a note follows it; otherwise the line ends at the count.
// SPEC.md's example has no trailing whitespace on any line, and trailing spaces are
// invisible in a diff and rot without anyone noticing.
func (s SourceReport) itemLines(u *ui.UI) []string {
	var verbWidth, idWidth, countWidth int
	counts := make([]string, len(s.Items))
	for i, it := range s.Items {
		counts[i] = ui.FileCount(it.Files)
		verbWidth = max(verbWidth, len(it.Verb))
		idWidth = max(idWidth, len(it.ID))
		countWidth = max(countWidth, len(counts[i]))
	}

	out := make([]string, 0, len(s.Items))
	for i, it := range s.Items {
		// Padding is computed on the unstyled text and the escape sequences go on after,
		// so a coloured report has the same columns as a plain one.
		line := "  " + u.Bold(it.Verb) + ui.Pad(len(it.Verb), verbWidth) + "  " +
			it.ID + ui.Pad(len(it.ID), idWidth) + "  " + counts[i]
		if it.Note != "" {
			line += ui.Pad(len(counts[i]), countWidth) + "  " + u.Dim(it.Note)
		}
		out = append(out, line)
	}
	return out
}

// summary is the last line: what was written, what was removed, and where to look. A dry
// run says what would happen instead, so a reader can never mistake a plan for a result.
func (r *Report) summary() string {
	if r.DryRun {
		return fmt.Sprintf("%s to write, %d to remove - nothing written", ui.FileCount(r.Written), r.Removed)
	}
	return fmt.Sprintf("%s written, %d removed - review with `git diff`", ui.FileCount(r.Written), r.Removed)
}
