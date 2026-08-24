## Why

Every pure package now agrees about what a consumer asked for, what a source offers, and
which files would land where — but nothing has ever contacted a source repository. A `rev`
is still a string, `plan.Input.Resolved` and `plan.Input.Items` have no producer, and the
claim SPEC.md makes about working offline ("Network unavailable, cache hit: proceeds") is
untested because there is no cache.

## What Changes

- New `internal/source`: rev resolution, the fetch cache, and enumeration of a fetched
  tree. It is the only package that runs `git`, and it writes **only** under the cache root
  — never to the working tree.
- **Rev resolution** — `git ls-remote` turns a tag or branch into the commit sha it names;
  an annotated tag resolves to the commit it points at, not the tag object; a 40-character
  sha passes through without contacting the remote. A rev nothing matches is an error
  naming the rev and the source.
- **Shorthand expansion** — `host/owner/repo` becomes an HTTPS URL. `graft.toml` stores
  `git` exactly as written, so expanding it belongs to the package that talks to git.
- **The content-addressed fetch cache** under `~/.cache/graft/<host>/<owner>/<repo>/<sha>/`.
  An entry is written by fetching into a sibling temporary directory and renaming it into
  place, so a half-fetched tree is never mistaken for a cache hit. An existing entry is
  reused without running any git command at all — which is what makes a resolved sync work
  offline.
- **Enumeration** — a fetched tree yields its `catalog.yaml` and, per item, the
  `plan.Listing` its `from` contributes. This is the producer `destination-and-plan` left
  unwritten, and it discharges that change's deferred note that a listing's fidelity to a
  real tree is `git-fetch`'s contract.
- Not **BREAKING**: no file format changes and no command exists yet to change output.
  `graft.lock` keeps `version = 1`.

## Non-Goals

- **No working-tree writes.** `internal/apply` is still the sole writer of the repository
  graft runs in; this package's only writes are cache entries under its own root.
- **No planning.** Destinations, the prune set, and the next lock are `internal/plan`'s and
  already exist.
- **No command surface.** No cobra, no `sync`, no progress output, no `--dry-run`.
- **No latest-semver-tag default.** `add` defaults an omitted `rev` to a source's newest
  semver tag; that rule, and the tag enumeration it needs, land with `add-command`.
- **No auth layer.** Private repos work exactly as far as the user's existing git
  credentials already reach. graft adds no credential store, no token flag, and no prompt.
- **No execution of source content.** Nothing fetched is run. The cache holds a tree, not a
  repository: the git directory is built *beside* the work tree, so no `.git` ever exists
  inside a published entry. Three narrower routes are closed with it, because this is the
  first change that runs `git` against a remote the consumer does not control — a `git`
  value beginning with `-` (git parses it as an option, and `--upload-pack=` is arbitrary
  execution), a committed `.gitattributes` (a `filter=` driver runs a command from the
  *consumer's* config), and a symlink anywhere below an entry (an ordinary read follows it
  straight out of the tree).
- Nothing near the PRD's non-goals: no dependency resolution, no registry, no merge
  behavior, and no runtime dependency on graft.

## Capabilities

### New Capabilities
- `rev-resolution`: expanding a source's `git` value to a clone URL, and turning a `rev`
  into the 40-character commit sha graft records — including every way that fails.
- `fetch-cache`: where a fetched tree lives, how an entry is published atomically, when the
  network is contacted and when it is not, and the offline behavior on a hit and on a miss.
- `source-listing`: reading `catalog.yaml` out of a fetched tree, and turning one item's
  `from` into the `plan.Listing` that `plan.Build` consumes.

### Modified Capabilities
None. `manifest-format`, `lock-format`, `catalog-format`, `selector-expansion`,
`destination-computation`, and `sync-plan` keep every requirement they have.

## Impact

- New package `internal/source`; new specs `rev-resolution`, `fetch-cache`, and
  `source-listing`.
- New runtime expectation made concrete: `git` on `PATH`, already stated by SPEC.md and
  ENGINEERING.md. No new Go module dependency.
- No change to `internal/{manifest,lock,catalog,itemid,plan}`, to `cmd/graft`, to `go.mod`,
  or to any file format. Nothing outside this repository is affected.
