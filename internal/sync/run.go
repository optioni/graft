package sync

import (
	"fmt"
	"path/filepath"
	"slices"

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

	// Manifest is the graft.toml this run honours and writes, in place of the one on
	// disk, or nil to read the file. `graft add` edits the manifest before anything is
	// resolved, and a run that resolved the file while writing the edit would install one
	// thing and record another.
	//
	// It is never set together with Update.To: two sources of manifest bytes is exactly
	// that failure, and Run refuses the combination rather than choosing between them.
	Manifest []byte

	// Update makes this run a `graft update`: the sources it names have their revs
	// re-resolved rather than taken from the lock. A nil Update is `graft sync`, which
	// re-resolves nothing — the zero Options is exactly the sync behavior, so this field
	// is the one difference between the two commands.
	Update *Update
}

// Update names what a `graft update` run moves.
type Update struct {
	// Source is the one source to re-resolve, or "" for every source the manifest
	// declares. An empty name cannot collide with a real one: manifest.Parse refuses a
	// source whose name is empty.
	Source string

	// To is the rev to write into graft.toml for Source before anything is resolved, or ""
	// to leave the manifest alone.
	//
	// It is honoured only together with Source, and that is a precondition rather than a
	// checked error: `graft update --to <rev>` with no source is refused by the command
	// surface, where it earns the hint line a usage error carries. A guard here would be a
	// branch no user can reach.
	To string
}

// refreshes reports whether this run re-resolves the named source rather than taking its
// sha from the lock. A nil Update is `graft sync` and refreshes nothing, which is what keeps
// rev = "main" from drifting between syncs.
func (o Options) refreshes(name string) bool {
	return o.Update != nil && (o.Update.Source == "" || o.Update.Source == name)
}

// Run makes the tree match the lock and returns what changed.
//
// Every error is returned exactly as the package that raised it worded it. SPEC.md's
// failure-mode table is written as those messages, each of which already locates its own
// problem — source "shared": …, graft.toml: …, catalog.yaml: … — so a second layer of
// context here would say the same thing twice.
func Run(o Options) (*Report, error) {
	m, data, err := readManifest(o)
	if err != nil {
		return nil, err
	}
	current, err := lock.Load(filepath.Join(o.Root, lock.Filename))
	if err != nil {
		return nil, err
	}

	// Before the first resolution and the first fetch, and before the manifest is edited.
	// A mistyped source name must produce this message rather than the manifest editor's
	// refusal, which would be technically true about a table that does not exist and of no
	// use to anyone.
	if o.Update != nil && o.Update.Source != "" && !declares(m, o.Update.Source) {
		return nil, fmt.Errorf("%s has no source %q", manifest.Filename, o.Update.Source)
	}

	// The run resolves what will be on disk, not what was: movePin re-parses the bytes it
	// edited and hands them back together, so nothing downstream can read one and write the
	// other.
	moved, edited, err := movePin(o, data)
	if err != nil {
		return nil, err
	}
	if moved != nil {
		m = edited
	}

	// Narrowed to the sources this run does not re-resolve. A disagreement is real for a
	// source whose sha still comes from the lock, and it is precisely what re-resolving the
	// source repairs — so checking it against a source being updated would refuse the run
	// that fixes it. A manifest whose rev moved cannot otherwise be honoured until
	// `graft update` moves the lock with it, and finding that out after a network round
	// trip would be finding it out too late.
	if err := lock.CheckPins(pinned(o, m), current); err != nil {
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
	report := newReport(current, p, res.catalogs, o.DryRun)
	if o.DryRun {
		return report, nil
	}

	// The manifest edit reaches disk here or not at all: it is an argument to the apply, so
	// --dry-run returning above leaves graft.toml exactly as it was, and a refusal inside
	// internal/apply leaves it there too.
	var opts []apply.Option
	switch {
	case moved != nil:
		opts = append(opts, apply.WithManifest(moved))
	case o.Manifest != nil:
		// The bytes the run resolved, and the same object: readManifest parsed these and
		// nothing else, so what reaches disk cannot describe a different request.
		opts = append(opts, apply.WithManifest(o.Manifest))
	}
	if err := apply.Run(o.Root, res.trees, p, opts...); err != nil {
		return nil, err
	}
	return report, nil
}

// readManifest returns the manifest this run honours and the bytes it parsed.
//
// Given bytes, it parses those and never touches the file: `graft add` has already read
// graft.toml, amended it, and is holding the result, and re-reading the path would be
// reading a file that could have changed between the two reads.
//
// Manifest bytes and a pin move together would give the run two sources of manifest bytes
// — one to resolve, one to write — which is the corruption the whole edit discipline
// exists to prevent. It is unreachable from the command surface, and refused here anyway.
func readManifest(o Options) (*manifest.Manifest, []byte, error) {
	if o.Manifest == nil {
		return manifest.Read(filepath.Join(o.Root, manifest.Filename))
	}
	if o.Update != nil && o.Update.To != "" {
		return nil, nil, fmt.Errorf("%s: a run cannot both be given manifest bytes and move a pin", manifest.Filename)
	}
	m, err := manifest.Parse(o.Manifest, manifest.Filename)
	if err != nil {
		return nil, nil, err
	}
	return m, o.Manifest, nil
}

// declares reports whether the manifest holds a source of this name.
func declares(m *manifest.Manifest, name string) bool {
	return slices.ContainsFunc(m.Sources, func(s manifest.Source) bool { return s.Name == name })
}

// pinned is the sources whose pins must agree with the lock: every one this run does not
// re-resolve.
//
// The filtering is here rather than inside lock.CheckPins because *which* sources to ask
// about is a decision about this run, and internal/lock's job is the lock's format. Order is
// preserved, and manifest.Parse sorts by name, so a lock disagreeing on several sources
// always names the same one.
func pinned(o Options, m *manifest.Manifest) []manifest.Source {
	if o.Update == nil {
		return m.Sources
	}
	out := make([]manifest.Source, 0, len(m.Sources))
	for _, s := range m.Sources {
		if !o.refreshes(s.Name) {
			out = append(out, s)
		}
	}
	return out
}

// movePin returns the graft.toml bytes this run will write and the manifest they parse to,
// or nil for both when the run writes none.
//
// The bytes are re-parsed here rather than mutated in the already-parsed manifest, and the
// source's rev in the re-parse is checked against what was asked for. That check is not belt
// and braces: manifest.SetRev edits text, and a text edit's real failure is landing on the
// wrong line — a commented-out key, a key in a sub-table — which produces a file that parses
// perfectly while the run resolves the old rev. Comparing the value turns every failure in
// that class into a failed run.
//
// Both halves are returned together so that what goes to disk and what the run resolves are
// the same object rather than two readings of it, and so the caller has no parse of its own
// to get wrong.
func movePin(o Options, data []byte) ([]byte, *manifest.Manifest, error) {
	if o.Update == nil || o.Update.To == "" || o.Update.Source == "" {
		return nil, nil, nil
	}

	moved, err := manifest.SetRev(data, o.Update.Source, o.Update.To)
	if err != nil {
		return nil, nil, err
	}
	edited, err := manifest.Parse(moved, manifest.Filename)
	if err != nil {
		return nil, nil, err
	}
	for _, s := range edited.Sources {
		if s.Name == o.Update.Source && s.Rev == o.Update.To {
			return moved, edited, nil
		}
	}
	return nil, nil, fmt.Errorf("%s: source %q: the pin did not move to %q",
		manifest.Filename, o.Update.Source, o.Update.To)
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
	// matched travels the same road as the sha: read from the previous lock on the
	// non-resolving path, and only from source.Resolve on the branch that actually
	// resolves. Carrying it on only the resolving path would make every sync over a
	// range write a lock its own validation refuses.
	type pin struct{ sha, matched string }
	pinned := make(map[string]pin, len(current.Sources))
	for _, s := range current.Sources {
		pinned[s.Name] = pin{sha: s.Resolved, matched: s.Matched}
	}

	cache := source.Cache{Root: o.CacheRoot}
	out := resolved{
		inputs:   make([]plan.Input, 0, len(m.Sources)),
		trees:    make(map[string]string, len(m.Sources)),
		catalogs: make(map[string]*catalog.Catalog, len(m.Sources)),
	}

	for _, s := range m.Sources {
		p, known := pinned[s.Name]
		// The one branch that tells a sync from an update. Everything below it — the fetch,
		// the catalog, the expansion, the listing — is the same code either way.
		if !known || o.refreshes(s.Name) {
			var err error
			if p.sha, p.matched, err = source.Resolve(s.Name, s.Git, s.Rev); err != nil {
				return resolved{}, err
			}
		}

		entry, err := cache.Fetch(s.Name, s.Git, p.sha)
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
			Resolved: p.sha,
			Matched:  p.matched,
			Catalog:  cat,
			Items:    listings,
		})
		out.trees[s.Name] = entry
		out.catalogs[s.Name] = cat
	}
	return out, nil
}
