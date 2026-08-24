// Package source resolves a source repository's rev to a commit sha, fetches that
// commit's tree into a content-addressed cache, and reads what the fetched tree holds.
// It is the only package that runs git.
//
// It writes only under the cache root it is given. internal/apply is the sole writer of
// the repository graft runs in, and nothing here touches it — a cache entry is not the
// working tree.
//
// A fetched tree is untrusted input. graft executes nothing a source provides, and
// keeping that true takes three specific defences rather than a policy: a git value that
// begins with "-" is refused before it can become a git option, the checkout runs with
// the source's own .gitattributes disabled so no filter driver can be selected, and every
// read below an entry goes through an os.Root so no symlink can resolve out of it.
package source
