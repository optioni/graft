package sync

import (
	"path/filepath"

	"github.com/optioni/graft/internal/apply"
	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/lock"
	"github.com/optioni/graft/internal/manifest"
	"github.com/optioni/graft/internal/plan"
	"github.com/optioni/graft/internal/source"
)

// Options is everything a sync reads from its surroundings.
//
// Both roots are values rather than things this package looks up for itself. A cache root
// read from the environment here would mean every test either writes into the developer's
// real ~/.cache/graft or monkeys with process-global state to avoid it.
type Options struct {
	// Root is the repository graft runs in. graft.toml and graft.lock live in it, and it
	// is the only directory anything writes to.
	Root string

	// CacheRoot is the content-addressed fetch cache. It is not the working tree, and
	// writing to it is not touching the repository.
	CacheRoot string

	// DryRun stops after the plan is built. Nothing is written, nothing is deleted, and
	// no directory is created.
	DryRun bool
}

// Run makes the tree match the lock and returns what changed.
//
// Every error is returned exactly as the package that raised it worded it. SPEC.md's
// failure-mode table is written as those messages, each of which already locates its own
// problem — source "shared": …, graft.toml: …, catalog.yaml: … — so a second layer of
// context here would say the same thing twice.
func Run(o Options) (*Report, error) {
	m, err := manifest.Load(filepath.Join(o.Root, manifest.Filename))
	if err != nil {
		return nil, err
	}
	current, err := lock.Load(filepath.Join(o.Root, lock.Filename))
	if err != nil {
		return nil, err
	}

	// Before the first resolution and the first fetch. A manifest whose rev moved cannot
	// be honoured until `graft update` moves the lock with it, and finding that out after
	// a network round trip would be finding it out too late.
	if err := lock.CheckPins(m.Sources, current); err != nil {
		return nil, err
	}

	res, err := resolve(o, m, current)
	if err != nil {
		return nil, err
	}

	p, err := plan.Build(res.inputs, current)
	if err != nil {
		return nil, err
	}

	// --dry-run stops here. Everything before this point created, modified, and deleted
	// nothing in the repository, so returning is the whole implementation of "touches
	// nothing, including creating no directories".
	//
	// The fetch above still happened, and that is not a leak: there is no plan without a
	// catalog and no catalog without a fetch, and internal/source writes under the cache
	// root only. The cost is that a dry run reaches none of internal/apply's refusals, so
	// a clean one says the plan is valid rather than that the sync will succeed.
	if o.DryRun {
		return &Report{}, nil
	}
	if err := apply.Run(o.Root, res.trees, p); err != nil {
		return nil, err
	}
	return &Report{}, nil
}

// resolved is what walking the manifest's sources produces: the planner's inputs, the
// fetched tree per source for the applier, and the catalog per source for the report.
type resolved struct {
	inputs   []plan.Input
	trees    map[string]string
	catalogs map[string]*catalog.Catalog
}

// resolve performs SPEC.md's steps 2 through 6 for every source the manifest declares.
//
// A source the lock already records is never re-resolved — its sha comes straight from the
// lock, whatever its rev names today. That is what keeps rev = "main" from drifting under
// a user between syncs; moving a pin is `graft update`, always explicit.
//
// A source the lock records but the manifest no longer declares is not reached at all. Its
// rev is not declared anywhere, so there is nothing to resolve and nothing to fetch, and
// its files are pruned from the lock alone — the only record of them that exists.
//
// Nothing here writes to the repository. internal/source writes under the cache root and
// nowhere else, and the cache is not the working tree.
func resolve(o Options, m *manifest.Manifest, current *lock.Lock) (resolved, error) {
	pinned := make(map[string]string, len(current.Sources))
	for _, s := range current.Sources {
		pinned[s.Name] = s.Resolved
	}

	cache := source.Cache{Root: o.CacheRoot}
	out := resolved{
		inputs:   make([]plan.Input, 0, len(m.Sources)),
		trees:    make(map[string]string, len(m.Sources)),
		catalogs: make(map[string]*catalog.Catalog, len(m.Sources)),
	}

	for _, s := range m.Sources {
		sha, known := pinned[s.Name]
		if !known {
			var err error
			if sha, err = source.Resolve(s.Name, s.Git, s.Rev); err != nil {
				return resolved{}, err
			}
		}

		entry, err := cache.Fetch(s.Name, s.Git, sha)
		if err != nil {
			return resolved{}, err
		}
		cat, err := source.ReadCatalog(entry)
		if err != nil {
			return resolved{}, err
		}

		// Expanded here because the listings are per item, and again inside plan.Build
		// because that package may not depend on a caller having done it. The second
		// expansion is cheap and the alternative is a plan that trusts its input.
		items, err := catalog.Expand(cat, s.Name, s.Install)
		if err != nil {
			return resolved{}, err
		}
		listings := make(map[string]plan.Listing, len(items))
		for _, it := range items {
			listing, err := source.List(entry, s.Name, it)
			if err != nil {
				return resolved{}, err
			}
			listings[it.ID] = listing
		}

		out.inputs = append(out.inputs, plan.Input{
			Source:   s,
			Resolved: sha,
			Catalog:  cat,
			Items:    listings,
		})
		out.trees[s.Name] = entry
		out.catalogs[s.Name] = cat
	}
	return out, nil
}
