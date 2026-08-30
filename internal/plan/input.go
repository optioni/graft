package plan

import (
	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/manifest"
)

// Input is one source's fully resolved inputs: what the consumer asked for, what the
// source's catalog offers, the sha its rev resolved to, and what each installed item's
// From contributes. Producing the last two is internal/source's job; this package
// takes them as values so that it never has to look.
type Input struct {
	Source   manifest.Source
	Resolved string
	Catalog  *catalog.Catalog

	// Matched is the tag a range resolved to, exactly as internal/source reported it —
	// empty for a ref. It is carried verbatim into the next lock's Source.Matched: this
	// package forms no opinion about whether a rev is a range, because that predicate
	// belongs to the packages that interpret a rev, and a third opinion here is a third
	// place for them to disagree.
	Matched string

	// Items is keyed by item id. An installed item with no entry listed nothing,
	// which is the same state as an item whose From names an empty directory.
	Items map[string]Listing
}
