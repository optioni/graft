package list

import (
	"github.com/optioni/graft/internal/ui"
)

// Lines renders the listing a person reads, one string per line, for the caller to write to
// the standard output stream. A listing is the content the caller asked for rather than a
// summary of something that happened, which is what puts it on the stream `--version` and
// help already use.
//
// It takes no *ui.UI because it has nothing to style: the two things the sync report styles
// — a verb and a trailing note — do not exist here, and inventing a style for a listing to
// demonstrate that colour works is decoration. The phrases it shares with that report come
// from internal/ui so the two cannot disagree about them.
//
// An empty listing renders as no lines at all. "Nothing installed" is a note about the
// absence of content, and the command surface writes it where notes go.
func (l *Listing) Lines() []string {
	var out []string
	for i, s := range l.Sources {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, s.header())
		// The blank line introduces item lines. A source with none is its header alone, and
		// emitting the line unconditionally would leave two blanks before the next block.
		if items := s.itemLines(); len(items) > 0 {
			out = append(out, "")
			out = append(out, items...)
		}
	}
	return out
}

// header names the source, the rev the lock records, and the short form of its sha. When
// the source's rev is a range, the matched tag appears between the rev and the sha — the
// column is absent, not empty and not padded, for every ref, so a listing containing no
// range is byte-identical to what graft printed before ranges existed. Two headers in one
// listing are never padded into a shared layout: each block is independent.
func (s Source) header() string {
	h := s.Name + "  " + s.Rev
	if s.Matched != "" {
		h += "  " + s.Matched
	}
	return h + "  (" + ui.ShortSHA(s.Resolved) + ")"
}

// itemLines renders one source's block, aligned within that block: the id padded to the
// widest id there, then the file count. The line ends at the count, so no line carries
// trailing whitespace.
func (s Source) itemLines() []string {
	var idWidth int
	for _, it := range s.Items {
		idWidth = max(idWidth, len(it.ID))
	}

	out := make([]string, 0, len(s.Items))
	for _, it := range s.Items {
		out = append(out, "  "+it.ID+ui.Pad(len(it.ID), idWidth)+"  "+ui.FileCount(len(it.Files)))
	}
	return out
}
