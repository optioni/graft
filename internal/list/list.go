// Package list turns graft.lock into what `graft list` prints: a listing value and its
// two renderings.
//
// It is a separate package from internal/sync rather than a function in it because
// internal/sync is the resolution sequence — fetch, plan, apply — and list performs none of
// it. Keeping them apart makes "this command cannot write and cannot reach the network" an
// observable property of the import graph rather than a claim in a comment.
package list

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"github.com/optioni/graft/internal/itemid"
	"github.com/optioni/graft/internal/lock"
)

// Version is the version of the --json document's format, and not graft.lock's. The two
// are both 1 today and are free to move apart: the lock is graft's working file and the
// document is a projection published for consumers, which is the whole reason the
// projection exists. One number meaning two things would tie them back together.
const Version = 1

// Listing is what a lock records, ordered and ready to render. Its JSON tags and its field
// order are the published contract: encoding/json emits members in struct field order, so
// moving a field here changes the document.
type Listing struct {
	Version int      `json:"version"`
	Sources []Source `json:"sources"`
}

// Source is one source's block. Resolved is the full forty-character sha — the shortened
// form exists to be read by a person, and a program comparing shas needs all of it.
type Source struct {
	Name     string `json:"name"`
	Git      string `json:"git"`
	Rev      string `json:"rev"`
	Resolved string `json:"resolved"`
	Items    []Item `json:"items"`
}

// Item is one installed item. Kind and Name are the two halves of ID, carried rather than
// left to be derived: kind:name is graft's grammar, and a consumer filtering by kind should
// not have to re-implement it in a language where the obvious split on a colon is wrong.
type Item struct {
	ID    string   `json:"id"`
	Kind  string   `json:"kind"`
	Name  string   `json:"name"`
	Files []string `json:"files"`
}

// FromLock builds the listing a lock describes, ordered the way graft orders everything it
// writes: sources by name, items by id, files by path. internal/lock parses in on-disk
// order and only its Marshal sorts, so ordering here is what makes two locks with the same
// content list identically.
//
// Nothing the argument holds is reordered or shared: every slice is a fresh one, so a
// caller still holding the lock is unaffected and a later edit to the listing cannot reach
// back into it.
func FromLock(l *lock.Lock) *Listing {
	out := &Listing{Version: Version, Sources: make([]Source, 0, len(l.Sources))}
	for _, s := range l.Sources {
		src := Source{
			Name:     s.Name,
			Git:      s.Git,
			Rev:      s.Rev,
			Resolved: s.Resolved,
			// Allocated with a length of zero rather than left nil, so a source with no
			// items marshals as [] and not as null — the single most common way a JSON
			// contract breaks a consumer, and invisible in any test that only decodes.
			Items: make([]Item, 0, len(s.Items)),
		}
		for _, it := range s.Items {
			// An id that does not parse yields two empty halves. It cannot arrive from a
			// lock internal/lock validated, which refuses one, and the document says what
			// it knows rather than inventing a half.
			kind, name, _ := itemid.Split(it.ID)
			files := make([]string, len(it.Files))
			copy(files, it.Files)
			slices.Sort(files)
			src.Items = append(src.Items, Item{ID: it.ID, Kind: kind, Name: name, Files: files})
		}
		slices.SortFunc(src.Items, func(a, b Item) int { return strings.Compare(a.ID, b.ID) })
		out.Sources = append(out.Sources, src)
	}
	slices.SortFunc(out.Sources, func(a, b Source) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// Empty reports whether nothing is installed. A lock with no sources and no lock at all are
// the same answer, which is why the question is asked of the listing rather than of the file.
func (l *Listing) Empty() bool { return len(l.Sources) == 0 }

// JSON renders the complete document, the trailing newline included.
//
// The encoder is configured rather than json.Marshal called: SetEscapeHTML(false) is the
// load-bearing half, because Go escapes <, > and & by default and a git URL carrying a
// query string would come back with three characters replaced. Encode appends the trailing
// newline, so the document's bytes have one owner rather than being finished off by
// whatever writes them.
func (l *Listing) JSON() []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	// Cannot fail: the value is a tree of strings, ints, and slices of those, and the
	// writer is a bytes.Buffer. Returning an error would put an unreachable branch in every
	// caller and a hole in the coverage gate.
	_ = enc.Encode(l)
	return b.Bytes()
}
