// Package plan computes, from values alone, which files a sync will write, which it
// will delete, and what the next graft.lock will record. It is pure: it reads no file,
// stats no path, runs no command, opens no network connection, and creates, modifies,
// or deletes nothing in the working tree.
//
// The two facts a plan cannot compute for itself arrive as values. Enumerating a
// fetched source tree is internal/source's work, so an item's file listing is an input;
// performing a write is internal/apply's, so a plan describes writes rather than doing
// them. That split is what makes SPEC.md's "nothing touches the tree until every check
// passes" a property of this type rather than a promise about a call order.
package plan
