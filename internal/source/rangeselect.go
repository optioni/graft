package source

import (
	"github.com/Masterminds/semver/v3"
)

// MatchRange selects the highest tag among tags that satisfies rev, a semver range, and
// returns the tag name exactly as the remote spelled it. name locates the error, exactly
// as every other message in this package.
//
// Selection is deterministic: given the same range and the same set of tag names, it
// selects the same tag regardless of the order tags arrive in, breaking a tie between
// two tags naming the same version by taking the lower tag name under byte-wise
// comparison. It creates, modifies, and deletes nothing — it is pure over its arguments.
func MatchRange(name, rev string, tags []string) (string, error) {
	fail := sourceErrf(name)

	constraint, err := semver.NewConstraint(rev)
	if err != nil {
		return "", fail("rev %q is not a valid semver range", rev)
	}

	type candidate struct {
		tag string
		v   *semver.Version
	}
	var parsed []candidate
	for _, tag := range tags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			// A source is free to publish "latest" and "release-2024-01" beside its
			// versions; a tag that does not parse as semver is discarded silently.
			continue
		}
		parsed = append(parsed, candidate{tag: tag, v: v})
	}
	if len(parsed) == 0 {
		return "", fail("rev %q is a range, and the source publishes no semver tags", rev)
	}

	var best *candidate
	for i := range parsed {
		c := &parsed[i]
		// Constraint.Check already implements the npm reading: a prerelease is excluded
		// unless the range's own text names one of the same major.minor.patch, which is
		// exactly why this dependency was taken over a hand-rolled parser.
		if !constraint.Check(c.v) {
			continue
		}
		if best == nil {
			best = c
			continue
		}
		switch cmp := c.v.Compare(best.v); {
		case cmp > 0:
			best = c
		case cmp == 0 && c.tag < best.tag:
			// A tie under semver precedence — build metadata, or a tag with and
			// without a leading v — is broken by the lower tag name, so the result
			// never depends on the order the remote listed them in.
			best = c
		}
	}
	if best == nil {
		return "", fail("rev %q matches none of the source's semver tags", rev)
	}
	return best.tag, nil
}
