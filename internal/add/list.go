package add

import (
	"slices"
	"strings"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/manifest"
	"github.com/optioni/graft/internal/picker"
	"github.com/optioni/graft/internal/plan"
	"github.com/optioni/graft/internal/source"
	"github.com/optioni/graft/internal/ui"
)

// noItems is what a source offering nothing prints. `graft list` renders a source with no
// items as its header alone, and this command does not: there, the absence of items is a
// fact about the consumer's own lock, and here it is the answer to the question that was
// asked.
const noItems = "  (no items)"

// List returns the lines `graft add <source> --list` prints: a header naming the source,
// the rev, and the sha it resolved to, then every item the source offers with the
// destination its files would be written to.
//
// The destinations are this consumer's, not the catalog's proposal: when graft.toml
// already declares the source, its [sources.<name>.kinds] overrides win, because the
// destination is what a consumer actually agrees to. A graft.toml that is not there at all
// is not an error — the whole point of --list is to be usable before there is one.
//
// Nothing is written to the repository. The fetch populates the content-addressed cache,
// exactly as --dry-run's does, and the cache is not the working tree.
func List(r Request) ([]string, error) {
	name, err := DeriveName(r.Git)
	if err != nil {
		return nil, err
	}
	rev, err := effectiveRev(r, name, nil)
	if err != nil {
		return nil, err
	}
	title, items, err := offer(r, name, rev)
	if err != nil {
		return nil, err
	}

	out := []string{title}
	if len(items) == 0 {
		return append(out, noItems), nil
	}

	var idWidth int
	for _, it := range items {
		idWidth = max(idWidth, len(it.ID))
	}
	out = append(out, "")
	for _, it := range items {
		out = append(out, "  "+it.ID+ui.Pad(len(it.ID), idWidth)+"  "+strings.Join(it.Destinations, ", "))
	}
	return out, nil
}

// offer is what a source has to show: a header naming it, the rev, and the sha it resolved
// to, and every item it provides with the destinations this consumer would get.
//
// It is one function because `--list` and the picker must show the same thing. Two
// walks of a catalog would agree today and drift the first time either learned something
// the other did not, and the destination is what a consumer actually agrees to.
//
// Nothing is written. The fetch populates the content-addressed cache, exactly as
// --dry-run's does, and the cache is not the working tree.
func offer(r Request, name, rev string) (string, []picker.Item, error) {
	sha, _, err := source.Resolve(name, r.Git, rev)
	if err != nil {
		return "", nil, err
	}

	cache := source.Cache{Root: r.CacheRoot}
	entry, err := cache.Fetch(name, r.Git, sha)
	if err != nil {
		return "", nil, err
	}
	cat, err := source.ReadCatalog(entry)
	if err != nil {
		return "", nil, err
	}

	overrides, err := declaredOverrides(r.Root, name)
	if err != nil {
		return "", nil, err
	}

	in := plan.Input{
		Source:   manifest.Source{Name: name, Git: r.Git, Rev: rev, Kinds: overrides},
		Resolved: sha,
		Catalog:  cat,
		Items:    make(map[string]plan.Listing, len(cat.Items)),
	}
	// Every item is offered, because the selectors are exactly what has not been chosen yet.
	items := slices.Clone(cat.Items)
	slices.SortFunc(items, func(a, b catalog.Item) int { return strings.Compare(a.ID, b.ID) })
	for _, it := range items {
		listing, err := source.List(entry, name, it)
		if err != nil {
			return "", nil, err
		}
		in.Items[it.ID] = listing
	}

	out := make([]picker.Item, 0, len(items))
	for _, it := range items {
		dests, err := plan.ItemDestinations(in, it)
		if err != nil {
			return "", nil, err
		}
		out = append(out, picker.Item{ID: it.ID, Kind: it.Kind, Destinations: dests})
	}
	return name + "  " + rev + "  (" + ui.ShortSHA(sha) + ")", out, nil
}

// declaredOverrides is the consumer's destination overrides for this source, or nil when
// graft.toml does not declare it — or does not exist. A manifest that exists and does not
// parse is refused in internal/manifest's own words, exactly as it is for an add.
func declaredOverrides(root, name string) (map[string]string, error) {
	_, m, err := readManifest(root)
	if err != nil {
		return nil, err
	}
	if s := declared(m, name); s != nil {
		return s.Kinds, nil
	}
	return nil, nil
}
