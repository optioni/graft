package source

import (
	"errors"
	"strings"

	"github.com/optioni/graft/internal/rev"
)

// Resolve turns a source's rev — a tag, a branch, a full sha, or a semver range — into
// the 40-character lowercase hex commit sha graft.lock records as resolved, and the tag
// name a range matched. name locates errors, so a manifest with several sources always
// says which one failed.
//
// The second return value is the tag a range resolved to, exactly as the remote spells
// it — v1.3.0, not 1.3.0 — because that is what graft.lock records as matched. A ref
// names itself, so resolving one returns an empty matched: there is nothing further to
// record.
//
// Resolution creates, modifies, and deletes nothing, anywhere.
func Resolve(name, git, revision string) (sha, matched string, err error) {
	fail := sourceErrf(name)
	// A sha is already the answer. Returning before exec.LookPath, not merely before the
	// query, is what makes an already-resolved pin work with no git at all — a round trip
	// here could only turn a working resolution into a broken one.
	if isSHA(revision) {
		return revision, "", nil
	}
	if revision == "" {
		return "", "", fail("rev is empty")
	}

	// Classified by syntax alone, before either path touches the network: a lookup here
	// would make a pin's meaning depend on what the remote happens to contain.
	if rev.IsRange(revision) {
		return resolveRange(name, git, revision)
	}

	url, err := CloneURL(name, git)
	if err != nil {
		return "", "", err
	}

	// Queried explicitly rather than by pattern. git ls-remote matches a pattern against
	// the tail of a ref name, so refs/tags/v1 alone would also match a ref named
	// refs/heads/x/refs/tags/v1 — the patterns keep the response small, and the parse
	// below is what makes the match exact.
	peeled := "refs/tags/" + revision + "^{}"
	tag := "refs/tags/" + revision
	head := "refs/heads/" + revision

	out, detail, gitErr := gitOutput("ls-remote", "--", url, tag, peeled, head)
	if gitErr != nil {
		if errors.Is(gitErr, errNoGit) {
			return "", "", gitErr
		}
		// A non-zero exit means the remote could not be read. A rev that simply does not
		// exist exits 0 with no output, so the two failures are told apart by the signal
		// rather than by parsing a message: one is a typo in graft.toml, the other a
		// network or permission problem.
		return "", "", fail("cannot reach %q: %s", url, detail)
	}

	found := map[string]string{}
	for line := range strings.SplitSeq(out, "\n") {
		lineSHA, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || !isSHA(lineSHA) {
			continue
		}
		// Exact comparison, not a suffix test. This is the half that makes the query
		// explicit; without it a pattern's tail match could answer for the wrong ref.
		if ref == peeled || ref == tag || ref == head {
			found[ref] = lineSHA
		}
	}
	// git rev-parse's own precedence, and the reading under which a pin names the
	// immutable thing: the commit an annotated tag points at beats the tag object, and a
	// tag beats a branch of the same name.
	for _, ref := range []string{peeled, tag, head} {
		if s, ok := found[ref]; ok {
			return s, "", nil
		}
	}
	return "", "", fail("rev %q not found", revision)
}

// resolveRange lists the source's tags and selects the highest one satisfying revision,
// returning its commit sha and the tag name exactly as the remote spelled it.
//
// The range is parsed before the network is ever touched: a malformed range must not
// cost a round trip, and must never fall back to a ref lookup — a typo in a range and a
// typo in a ref name are different mistakes, and conflating them would report the wrong
// one. Tag listing shares CloneURL's leading-dash refusal and gitOutput's
// prompt-disabling environment with the ref path; there is exactly one way to reach the
// network in this package, and this is the second call site for it.
func resolveRange(name, git, revision string) (sha, matched string, err error) {
	fail := sourceErrf(name)

	if _, err := parseRange(name, revision); err != nil {
		return "", "", err
	}

	url, err := CloneURL(name, git)
	if err != nil {
		return "", "", err
	}

	out, detail, gitErr := gitOutput("ls-remote", "--tags", "--", url)
	if gitErr != nil {
		if errors.Is(gitErr, errNoGit) {
			return "", "", gitErr
		}
		// A network failure is not a range failure: the tag list was never obtained, so
		// this must not be reported as an unsatisfiable range.
		return "", "", fail("cannot reach %q: %s", url, detail)
	}

	// direct holds each tag's own sha — the commit itself for a lightweight tag, the tag
	// object for an annotated one. peeled holds the commit an annotated tag's ^{} line
	// names, which wins whenever it exists: graft.lock's resolved must never be a tag
	// object.
	direct := map[string]string{}
	peeled := map[string]string{}
	seen := map[string]struct{}{}
	var names []string
	for line := range strings.SplitSeq(out, "\n") {
		lineSHA, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || !isSHA(lineSHA) {
			continue
		}
		tagName, isTag := strings.CutPrefix(ref, "refs/tags/")
		if !isTag {
			continue
		}
		if bareName, isPeeled := strings.CutSuffix(tagName, "^{}"); isPeeled {
			peeled[bareName] = lineSHA
			continue
		}
		direct[tagName] = lineSHA
		if _, dup := seen[tagName]; !dup {
			seen[tagName] = struct{}{}
			names = append(names, tagName)
		}
	}

	matched, err = MatchRange(name, revision, names)
	if err != nil {
		return "", "", err
	}
	if s, ok := peeled[matched]; ok {
		return s, matched, nil
	}
	return direct[matched], matched, nil
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
