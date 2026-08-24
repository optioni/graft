package source

import (
	"fmt"
	"strings"
)

// CloneURL expands a source's git value — stored by graft.toml exactly as written — into
// something git accepts. The shorthand form host/owner/repo becomes an HTTPS URL; every
// other form is passed through unchanged, because SPEC.md says git is "anything git clone
// accepts" and graft has no business rewriting a URL a user chose deliberately.
//
// A value beginning with "-" is refused. It is not a remote git clone accepts, it is an
// option: `git ls-remote --upload-pack=./pwn.sh refs/tags/v1` parses the first word as a
// flag, promotes the refspec to the repository operand, and runs the script. An explicit
// argv does not prevent that, because argv position is not what git uses to tell an
// option from an operand. The invocations pass "--" as well; this is the guard that does
// not depend on git's behavior.
//
// CloneURL is pure: it contacts nothing, reads nothing, and creates nothing.
func CloneURL(name, git string) (string, error) {
	if strings.HasPrefix(git, "-") {
		return "", sourceErrf(name)("git %q may not begin with %q", git, "-")
	}
	if !isShorthand(git) {
		return git, nil
	}
	return "https://" + git, nil
}

// isShorthand reports whether s is the host/owner/repo form. Everything that could be a
// URL or a path is excluded first, so the remaining question is only whether the first
// segment looks like a hostname — which is what separates github.com/o/r from a
// two-segment relative path like mirrors/assets.
func isShorthand(s string) bool {
	switch {
	case s == "":
		return false
	case strings.Contains(s, "://"):
		return false
	case strings.HasPrefix(s, "/"), strings.HasPrefix(s, "."):
		return false
	}
	// An scp-style address carries a colon before the first slash: git@host:owner/repo.
	// A colon after the first slash is an odd path, not an address, and is left alone.
	if i := strings.IndexByte(s, ':'); i >= 0 {
		if j := strings.IndexByte(s, '/'); j < 0 || i < j {
			return false
		}
	}
	host, rest, ok := strings.Cut(s, "/")
	if !ok || rest == "" {
		return false
	}
	// A hostname has a dot. Requiring one is what keeps "mirrors/assets" a path rather
	// than a remote graft invented on the user's behalf.
	return strings.Contains(host, ".") && !strings.HasPrefix(host, ".") && !strings.HasSuffix(host, ".")
}

// sourceErrf builds the per-source error prefix every message in this package carries,
// in one place, following catalog.errf and manifest.validate's fail closure. A manifest
// with several sources always says which one failed.
func sourceErrf(name string) func(format string, args ...any) error {
	return func(format string, args ...any) error {
		return fmt.Errorf("source %q: %s", name, fmt.Sprintf(format, args...))
	}
}

// itemErrf is sourceErrf with an item id, for the failures that belong to one item's
// from rather than to the source as a whole.
func itemErrf(name, id string) func(format string, args ...any) error {
	return func(format string, args ...any) error {
		return fmt.Errorf("source %q: item %q: %s", name, id, fmt.Sprintf(format, args...))
	}
}

// errf prefixes an error with the file it came from, the shape catalog.Load uses for a
// read that fails for a reason other than absence.
func errf(filename string, err error) error {
	return fmt.Errorf("%s: %w", filename, err)
}
