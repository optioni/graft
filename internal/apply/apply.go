// Package apply performs a plan's file operations. It is the only package in graft
// permitted to create, modify, or delete anything in the repository graft is running in.
//
// It derives nothing. Every path it touches comes from the plan it was given: it never
// scans a destination directory, never compares file contents, never decides that a file
// needs no writing, and never extends the prune set. That is what makes internal/plan's
// purity worth having — the checks that ran before anything was written are the checks
// that govern what gets written.
//
// The order is SPEC.md's resolution step 8, with one pass in front of it: check everything
// checkable, write the planned files, delete the prune set, remove the directories the
// prune set left empty, write graft.lock. The pre-flight pass is what makes this package's
// refusals leave the tree byte-identical rather than partly applied, and the lock going
// last is what keeps a failed run's record describing the previous state rather than a
// state that never existed.
//
// Containment takes two mechanisms rather than one. Every path resolves through an os.Root
// — at the repository for what is written, at a fetched tree for what is read — which
// refuses a path that leaves its root. An os.Root does, however, follow a symlink that
// stays inside it, so the root alone would let a repo-owned link redirect a write, or aim a
// deletion at a file graft.lock does not name. Hence the second mechanism: every ancestor
// of every path must be a directory, examined without following it.
package apply
