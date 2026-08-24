package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cache is a content-addressed store of fetched source trees. Root is a value the caller
// passes in, never a global this package reads for itself, so every test names its own
// root and none can write to the developer's real cache.
type Cache struct {
	Root string
}

// DefaultCacheRoot is the cache root graft uses when a caller names none:
// $XDG_CACHE_HOME/graft when that is set to an absolute path, and <home>/.cache/graft
// otherwise — the two spellings of the one location SPEC.md names. It creates nothing;
// the directory is made when the first entry is written.
func DefaultCacheRoot() (string, error) {
	return defaultCacheRoot(os.Getenv, os.UserHomeDir)
}

// defaultCacheRoot is DefaultCacheRoot with its two environment lookups injected, so the
// rules can be tested without setting HOME on the process running the tests.
func defaultCacheRoot(getenv func(string) string, home func() (string, error)) (string, error) {
	// Honoured only when absolute. A relative one would give the same source a different
	// entry per working directory, which is the one thing a content-addressed cache may
	// not do.
	if xdg := getenv("XDG_CACHE_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "graft"), nil
	}
	dir, err := home()
	if err != nil {
		return "", fmt.Errorf("cannot determine the cache root: %w", err)
	}
	return filepath.Join(dir, ".cache", "graft"), nil
}

// Entry is the path a source's tree at sha occupies: <Root>/<host>/<path...>/<sha>.
//
// It is pure — it creates nothing and contacts nothing — so a caller may ask where an
// entry would be without making one. That is also what lets Fetch decide it has a cache
// hit before starting any subprocess.
func (c Cache) Entry(name, git, sha string) (string, error) {
	if !isSHA(sha) {
		return "", sourceErrf(name)("resolved %q is not a 40-character hex sha", sha)
	}
	url, err := CloneURL(name, git)
	if err != nil {
		return "", err
	}
	segments := append(identity(url), sha)
	return filepath.Join(append([]string{c.Root}, segments...)...), nil
}

// identity reduces a clone URL to the path segments that name the repository it points
// at. Scheme, user, port, a trailing slash, and a .git suffix are all dropped, so the
// same repository over HTTPS and over ssh is fetched once rather than twice.
//
// Every segment is sanitized, and the sanitizer runs per segment after splitting, so a
// segment can neither contain a separator nor climb. That is the containment guarantee:
// no remote can aim a cache entry outside the cache root.
func identity(url string) []string {
	host, path := splitRemote(url)

	// The slash first, or ".../b.git/" and ".../b.git" become two entries for one
	// repository.
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, ".git")

	// A URL with no host — a filesystem path — gets a deterministic entry like every
	// other source rather than being a special case with none.
	out := []string{"local"}
	if host != "" {
		out[0] = safeSegment(host)
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "" {
			continue
		}
		out = append(out, safeSegment(seg))
	}
	return out
}

// splitRemote separates a clone URL into its host and its path, covering the three forms
// CloneURL can produce: a URL with a scheme, an scp-style user@host:path, and a bare
// filesystem path.
func splitRemote(url string) (host, path string) {
	if _, rest, ok := strings.Cut(url, "://"); ok {
		authority, p, found := strings.Cut(rest, "/")
		if !found {
			return stripUserAndPort(authority), ""
		}
		return stripUserAndPort(authority), p
	}
	// A colon before the first slash is an scp-style address; one after it is an odd
	// path, and a path is what the fallthrough treats it as.
	if i := strings.IndexByte(url, ':'); i >= 0 {
		if j := strings.IndexByte(url, '/'); j < 0 || i < j {
			return stripUserAndPort(url[:i]), url[i+1:]
		}
	}
	return "", url
}

func stripUserAndPort(authority string) string {
	if i := strings.LastIndexByte(authority, '@'); i >= 0 {
		authority = authority[i+1:]
	}
	if i := strings.IndexByte(authority, ':'); i >= 0 {
		authority = authority[:i]
	}
	return authority
}

// safeSegment reduces one path component to characters that are safe as a single
// component. It lives here with the rule it enforces rather than being shared with
// plan.insideRepo or lock.isRepoRelative, which constrain different things: those two
// refuse a path, and this one rewrites a remote's contribution so there is nothing left
// to refuse.
func safeSegment(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := b.String()
	// "." and ".." are the two components that mean something to a path resolver rather
	// than naming a directory. Prefixed rather than replaced, so they stay legible in a
	// cache listing.
	if out == "." || out == ".." {
		return "_" + out
	}
	return out
}
