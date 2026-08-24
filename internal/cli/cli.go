// Package cli turns graft's arguments into an outcome and an exit code.
//
// It lives under internal/ rather than in cmd/graft because coverage is measured over
// ./internal/... only, and because Taskfile.yml's ci task runs lint, cover, and build —
// cover being the only one that runs go test, and only over ./internal/... So a decision
// made beside main() is not merely unmeasured, it is unexecuted by CI.
//
// Main returns the exit code rather than calling os.Exit, which is what lets every
// exit-status behavior be asserted from a test in the same process.
package cli
