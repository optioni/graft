package source

import "strings"

// Resolve turns a source's rev — a tag, a branch, or a full sha — into the 40-character
// lowercase hex commit sha graft.lock records as resolved. name locates errors, so a
// manifest with several sources always says which one failed.
//
// Resolution creates, modifies, and deletes nothing, anywhere.
func Resolve(name, git, rev string) (string, error) {
	fail := sourceErrf(name)
	// A sha is already the answer. Returning before exec.LookPath, not merely before the
	// query, is what makes an already-resolved pin work with no git at all — a round trip
	// here could only turn a working resolution into a broken one.
	if isSHA(rev) {
		return rev, nil
	}
	if rev == "" {
		return "", fail("rev is empty")
	}
	url, err := CloneURL(name, git)
	if err != nil {
		return "", err
	}

	// Queried explicitly rather than by pattern. git ls-remote matches a pattern against
	// the tail of a ref name, so refs/tags/v1 alone would also match a ref named
	// refs/heads/x/refs/tags/v1 — the patterns keep the response small, and the parse
	// below is what makes the match exact.
	peeled := "refs/tags/" + rev + "^{}"
	tag := "refs/tags/" + rev
	head := "refs/heads/" + rev

	out, detail, err := gitOutput("", "ls-remote", "--", url, tag, peeled, head)
	if err != nil {
		if err == errNoGit { //nolint:errorlint // a sentinel returned directly, never wrapped
			return "", err
		}
		// A non-zero exit means the remote could not be read. A rev that simply does not
		// exist exits 0 with no output, so the two failures are told apart by the signal
		// rather than by parsing a message: one is a typo in graft.toml, the other a
		// network or permission problem.
		return "", fail("cannot reach %q: %s", url, detail)
	}

	found := map[string]string{}
	for line := range strings.SplitSeq(out, "\n") {
		sha, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || !isSHA(sha) {
			continue
		}
		// Exact comparison, not a suffix test. This is the half that makes the query
		// explicit; without it a pattern's tail match could answer for the wrong ref.
		if ref == peeled || ref == tag || ref == head {
			found[ref] = sha
		}
	}
	// git rev-parse's own precedence, and the reading under which a pin names the
	// immutable thing: the commit an annotated tag points at beats the tag object, and a
	// tag beats a branch of the same name.
	for _, ref := range []string{peeled, tag, head} {
		if sha, ok := found[ref]; ok {
			return sha, nil
		}
	}
	return "", fail("rev %q not found", rev)
}

// isSHA reports whether s is the 40-character lowercase hex sha graft.lock requires.
//
// Duplicated from internal/lock and internal/plan rather than shared, exactly as those
// two already duplicate it from each other, and producing the identical message. Three
// copies of six lines is the price of three packages that do not depend on one another; a
// shared one would put the definition of a valid resolved somewhere none of them owns.
func isSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}
