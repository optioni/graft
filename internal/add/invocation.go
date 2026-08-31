// Package add performs `graft add`: amend graft.toml so it declares what the invocation
// asked for, then either sync the result or stop, and — under --list — print what the
// source offers without writing anything at all.
//
// It is a sequence package in the shape of internal/sync, and it decides nothing another
// package already decides. The manifest edits belong to internal/manifest, the default pin
// and the fetch to internal/source, destinations to internal/plan, every write to
// internal/apply, and the sync itself to internal/sync. What lives here is the order those
// happen in, and the summary of what the manifest edit did.
//
// Nothing here writes to the working tree. The bytes it produces reach disk through
// internal/apply, which stays the only writer.
package add

import (
	"fmt"
	"strings"
)

// SplitSource splits a source argument into its git value and the rev written after `@`,
// reporting whether an `@` was there at all.
//
// The `@` is read as introducing a rev only when the text before it holds a `/` or a `:`.
// That condition is the whole rule, and it is what keeps `git@github.com:optioni/shared` a
// git value rather than a repository named `git` pinned to a rev — an scp-style address
// carries its `@` before any path separator, and every git value carries one or the other.
//
// An `@` with nothing after it is reported as present with an empty rev, so the caller can
// refuse it: `graft add optioni/shared@` asks for a pin it did not name, which is a
// different mistake from asking for no pin.
func SplitSource(spec string) (git, rev string, hasRev bool) {
	i := strings.LastIndexByte(spec, '@')
	if i < 0 || !strings.ContainsAny(spec[:i], "/:") {
		return spec, "", false
	}
	return spec[:i], spec[i+1:], true
}

// DeriveName is the name of the [sources.<name>] table for a git value: its final path
// segment, after the last `/` or `:`, with a trailing `.git` removed and a trailing `/`
// ignored. `optioni/shared`, `https://github.com/optioni/shared.git`, and
// `git@github.com:optioni/shared` all derive `shared`.
//
// The name must be a TOML bare key. A dot is excluded along with everything else outside
// the set, because [sources.my.repo] is a sub-table of `my` rather than the source named
// `my.repo` — and a quoted key, which would parse, leaves a shape the in-place edits have
// to guess at. A git value that derives anything else is refused rather than quoted: the
// consumer picks a name by hand, and graft never invents one for them.
func DeriveName(git string) (string, error) {
	name := strings.TrimRight(git, "/")
	if i := strings.LastIndexAny(name, "/:"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(name, ".git")
	if !bareKey(name) {
		return "", fmt.Errorf("cannot derive a source name from %q", git)
	}
	return name, nil
}

// bareKey reports whether name may be written as a TOML bare key.
//
// Duplicated from internal/manifest rather than shared, exactly as isSHA is duplicated
// across three packages there: the predicate is six lines, and exporting it would put the
// definition of a writable source name in a package that does not choose one. The two
// copies are held together by the refusal in AddSource, which is what a wrong answer here
// would run into.
func bareKey(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}
