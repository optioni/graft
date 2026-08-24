package sync

import (
	"fmt"
	"strings"

	"github.com/optioni/graft/internal/ui"
)

// shortSHA is how many characters of a resolved sha the report shows.
const shortSHA = 7

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
		out = append(out, s.header(), "")
		out = append(out, s.itemLines(u)...)
	}
	if len(out) > 0 {
		out = append(out, "")
	}
	return append(out, r.summary())
}

// header names the source and what moved: `name  rev  (sha)`, with either half rendered as
// `old -> new` when it moved. A source the lock had never seen, and one being reported only
// because it is going away, have no previous half and render each once.
func (s SourceReport) header() string {
	return fmt.Sprintf("%s  %s  (%s)",
		s.Name,
		transition(s.PrevRev, s.Rev),
		transition(short(s.PrevResolved), short(s.Resolved)),
	)
}

// transition renders `old -> new`, or just the new value when there is no old one or the
// two are the same.
func transition(before, after string) string {
	if before == "" || before == after {
		return after
	}
	return before + " -> " + after
}

func short(sha string) string {
	if len(sha) <= shortSHA {
		return sha
	}
	return sha[:shortSHA]
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
		counts[i] = fileCount(it.Files)
		verbWidth = max(verbWidth, len(it.Verb))
		idWidth = max(idWidth, len(it.ID))
		countWidth = max(countWidth, len(counts[i]))
	}

	out := make([]string, 0, len(s.Items))
	for i, it := range s.Items {
		// Padding is computed on the unstyled text and the escape sequences go on after,
		// so a coloured report has the same columns as a plain one.
		line := "  " + u.Bold(it.Verb) + pad(len(it.Verb), verbWidth) + "  " +
			it.ID + pad(len(it.ID), idWidth) + "  " + counts[i]
		if it.Note != "" {
			line += pad(len(counts[i]), countWidth) + "  " + u.Dim(it.Note)
		}
		out = append(out, line)
	}
	return out
}

// pad returns the spaces that take a field of the given length up to width.
func pad(length, width int) string {
	if length >= width {
		return ""
	}
	return strings.Repeat(" ", width-length)
}

// summary is the last line: what was written, what was removed, and where to look. A dry
// run says what would happen instead, so a reader can never mistake a plan for a result.
func (r *Report) summary() string {
	if r.DryRun {
		return fmt.Sprintf("%s to write, %d to remove - nothing written", fileCount(r.Written), r.Removed)
	}
	return fmt.Sprintf("%s written, %d removed - review with `git diff`", fileCount(r.Written), r.Removed)
}

// fileCount renders a count with its noun: "1 file" or "<n> files".
func fileCount(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}
