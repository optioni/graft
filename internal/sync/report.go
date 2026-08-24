package sync

// Report is what a sync changed, as a value: which sources moved, which items were added,
// updated, or removed, and how many files were written and deleted.
//
// It is built from the lock that was on disk, the lock the plan produced, and the plan's
// own counts — never from the tree. Every planned file is written on every sync, so
// "updated" cannot mean "the bytes changed", and no comparison this package could make
// would let it mean that.
//
// The fields and the rendering land in the groups that own them; today a sync returns an
// empty one so that the sequence in run.go can be tested on its own.
type Report struct{}
