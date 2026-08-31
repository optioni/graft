package source

import (
	"errors"
	"strings"
)

// DefaultRev is the pin `graft add` writes when the invocation names no rev: the source's
// highest stable semver tag, falling back to the name of the branch its HEAD points at.
//
// The result is a ref, never a range and never a sha. A range would be a policy — "adopt
// this source's future minors" — and inferring one from an invocation that stated none
// would make every add a standing subscription; a sha belongs in graft.lock, which is the
// file that records what a rev became.
//
// Tag selection is MatchRange's, asked for the range "*", so the default pin and a range
// pin can never disagree about which tag is latest — including about prereleases, which
// that constraint excludes. A source whose only tags are prereleases therefore falls back
// to its branch, and so does one whose tags are not semver at all: both of MatchRange's
// failures mean the same thing here, that there is no stable version to name.
//
// One round trip, asking for the symbolic ref alongside every other ref, because the two
// answers are needed together and a second call could see a different repository.
//
// DefaultRev creates, modifies, and deletes nothing, anywhere.
func DefaultRev(name, git string) (string, error) {
	fail := sourceErrf(name)

	url, err := CloneURL(name, git)
	if err != nil {
		return "", err
	}

	out, detail, gitErr := gitOutput("ls-remote", "--symref", "--", url)
	if gitErr != nil {
		if errors.Is(gitErr, errNoGit) {
			return "", gitErr
		}
		// A source that cannot be read has published nothing observable, which is not the
		// same as a source that has published nothing. Reporting the second for the first
		// would send the reader to the repository instead of to the network.
		return "", fail("cannot reach %q: %s", url, detail)
	}

	tags, head := parseSymrefListing(out)
	if matched, err := MatchRange(name, "*", tags); err == nil {
		return matched, nil
	}
	if head != "" {
		return head, nil
	}
	return "", fail("has no semver tag and no default branch")
}

// parseSymrefListing splits `git ls-remote --symref` output into the tag names it lists
// and the short name of the branch HEAD points at.
//
// A peeled ^{} line names the commit inside an annotated tag and carries no tag name of
// its own, so it is skipped: the tag is already listed by its own line, and counting both
// would offer the same version twice.
func parseSymrefListing(out string) (tags []string, head string) {
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if rest, isSymref := strings.CutPrefix(line, "ref: "); isSymref {
			ref, target, ok := strings.Cut(rest, "\t")
			if !ok || strings.TrimSpace(target) != "HEAD" {
				continue
			}
			if branch, isBranch := strings.CutPrefix(ref, "refs/heads/"); isBranch {
				head = branch
			}
			continue
		}
		lineSHA, ref, ok := strings.Cut(line, "\t")
		if !ok || !isSHA(lineSHA) {
			continue
		}
		tag, isTag := strings.CutPrefix(ref, "refs/tags/")
		if !isTag || strings.HasSuffix(tag, "^{}") {
			continue
		}
		tags = append(tags, tag)
	}
	return tags, head
}
