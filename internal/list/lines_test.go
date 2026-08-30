package list_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/list"
	"github.com/optioni/graft/internal/lock"
)

// assertLines compares two renderings and fails on trailing whitespace, which is invisible
// in a diff and rots without anyone noticing.
func assertLines(t *testing.T, got, want []string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Errorf("lines:\n got %q\nwant %q", got, want)
	}
	for i, line := range got {
		if strings.TrimRight(line, " ") != line {
			t.Errorf("line %d carries trailing whitespace: %q", i, line)
		}
	}
}

func TestLinesRendersSpecExample(t *testing.T) {
	t.Parallel()

	assertLines(t, list.FromLock(specLock()).Lines(), []string{
		"openspec-schemas  v1.2.0  (fae2a30)",
		"",
		"  agent:apply-orchestrator  1 file",
		"  schema:tdd                6 files",
	})
}

func TestLinesSeparatesBlocksWithOneBlankLine(t *testing.T) {
	t.Parallel()

	l := &lock.Lock{Version: lock.Version, Sources: []lock.Source{
		{
			Name: "a", Git: "example.test/a", Rev: "v1.0.0",
			Resolved: "aaaaaaa1111111111111111111111111111111111",
			Items:    []lock.Item{{ID: "agent:a", Files: []string{"a.md"}}},
		},
		{
			Name: "b", Git: "example.test/b", Rev: "v2.0.0",
			Resolved: "bbbbbbb2222222222222222222222222222222222",
			Items:    []lock.Item{{ID: "agent:b", Files: []string{"b.md"}}},
		},
	}}

	// The last line is b's item line: a listing that ended with a blank one would put an
	// empty line between the content and whatever the caller printed next.
	assertLines(t, list.FromLock(l).Lines(), []string{
		"a  v1.0.0  (aaaaaaa)",
		"",
		"  agent:a  1 file",
		"",
		"b  v2.0.0  (bbbbbbb)",
		"",
		"  agent:b  1 file",
	})
}

func TestLinesSourceWithNoItemsIsItsHeaderAlone(t *testing.T) {
	t.Parallel()

	l := &lock.Lock{Version: lock.Version, Sources: []lock.Source{{
		Name: "empty", Git: "example.test/r", Rev: "main",
		Resolved: "aaaaaaa1111111111111111111111111111111111",
	}}}

	// No blank line inside the block: the blank line introduces item lines, and emitting it
	// unconditionally would leave two in a row before the next block.
	assertLines(t, list.FromLock(l).Lines(), []string{"empty  main  (aaaaaaa)"})
}

func TestLinesShaShorterThanSevenIsPrintedWhole(t *testing.T) {
	t.Parallel()

	// Not a sha internal/lock would accept, which is the point: the renderer shortens what
	// it is given and neither pads nor truncates a value it cannot shorten.
	l := &list.Listing{Version: list.Version, Sources: []list.Source{{
		Name: "shared", Rev: "main", Resolved: "abc", Items: []list.Item{},
	}}}

	assertLines(t, l.Lines(), []string{"shared  main  (abc)"})
}

func TestLinesOfAnEmptyListingAreNone(t *testing.T) {
	t.Parallel()

	// Nothing installed is a note on the error stream, which the command surface writes.
	// The renderer's answer is no lines at all rather than a sentence that parses as an item.
	if got := list.FromLock(&lock.Lock{Version: lock.Version}).Lines(); len(got) != 0 {
		t.Errorf("lines = %q, want none", got)
	}
}
