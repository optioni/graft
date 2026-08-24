// Package sync performs SPEC.md's resolution sequence: read the consumer's two files,
// take each source's pin from the lock, fetch it, read its catalog, list what each
// installed item contributes, build a plan, and apply it.
//
// It decides nothing that another package already decides. Selector expansion belongs to
// internal/catalog, destinations and the prune set to internal/plan, resolution and
// fetching to internal/source, and every write to internal/apply. What lives here is the
// order those happen in, and the report that describes what changed.
//
// The order is the contract rather than an implementation detail. Steps 1 through 7 —
// everything up to and including building the plan — create, modify, and delete nothing in
// the repository, so a failure at any of them leaves the working tree exactly as it was.
// The pin check comes before the first fetch for the same kind of reason: a manifest that
// moved must not be able to cause network access, let alone a write.
//
// The repository root and the cache root arrive as values. Nothing here reads a global or
// an environment variable, which is what lets every test name its own roots and keeps them
// out of the developer's real ~/.cache/graft.
package sync
