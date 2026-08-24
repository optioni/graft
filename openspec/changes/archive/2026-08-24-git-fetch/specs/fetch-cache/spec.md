## ADDED Requirements

### Requirement: A fetched tree lives at a path derived from the remote and the sha

`internal/source` SHALL place a fetched tree at `<root>/<host>/<owner>/<repo>/<sha>/`,
where `<root>` is the cache root, the segments before `<sha>` come from the source's clone
URL, and `<sha>` is the resolved commit. Deriving that path SHALL be a pure function of the
URL and the sha: it creates nothing and contacts nothing.

The URL's *identity* is its host and its path. Scheme, user, port, a trailing `/`, and a
`.git` suffix SHALL NOT change the derived path, so the same repository addressed over HTTPS
and over ssh is fetched once rather than twice. A trailing slash SHALL be trimmed before the
`.git` suffix is, or `…/b.git/` and `…/b.git` become two entries for one repository. A URL with no host — a filesystem path — SHALL derive
its segments under the literal host segment `local`, so a fixture repository or a local
mirror has a deterministic entry like every other source.

Every derived segment SHALL be reduced to characters that are safe as a single path
component: anything outside letters, digits, `.`, `-`, and `_` becomes `-`, and a segment
of `.` or `..` is prefixed with `_`. A remote SHALL NOT be able to aim a cache entry at a
directory outside the cache root.

A `sha` that is not 40 lowercase hex characters SHALL be refused with
`source "<name>": resolved "<sha>" is not a 40-character hex sha`, before any directory is
created — the same wording `internal/plan` already uses, because the two packages must not
disagree about what a valid `resolved` is.

#### Scenario: The cache path mirrors the remote and the sha

- **WHEN** the cache root is `/c` and source `shared`, whose clone URL is
  `https://github.com/optioni/openspec-schemas`, is fetched at sha
  `fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5`
- **THEN** the entry's path is
  `/c/github.com/optioni/openspec-schemas/fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5`
- **AND** deriving it creates no directory

#### Scenario: The same repository over ssh and over HTTPS is one entry

- **WHEN** the cache root is `/c` and the same sha is derived for
  `git@github.com:optioni/openspec-schemas.git` and for
  `https://github.com/optioni/openspec-schemas`
- **THEN** both derive the identical path under `/c/github.com/optioni/openspec-schemas/`
- **AND** `https://github.com/optioni/openspec-schemas.git/`, with both a `.git` suffix and
  a trailing slash, derives that same path

#### Scenario: A filesystem remote gets an entry under `local`

- **WHEN** the cache root is `/c` and the clone URL is `/srv/mirrors/assets`
- **THEN** the entry's path is `/c/local/srv/mirrors/assets/<sha>`

#### Scenario: A hostile remote cannot escape the cache root

- **WHEN** the clone URL contains `..` segments, percent-encoded separators, or characters
  outside `[A-Za-z0-9._-]` — for example `https://example.com/../../etc/passwd`,
  `https://example.com/..%2f..%2fetc`, or `git@..:../../etc/x`
- **THEN** every derived segment is sanitized, the `..` segments become `_..`, and the
  resulting path is still inside the cache root
- **AND** containment is established by `filepath.Rel(root, entry)` succeeding with no `..`
  segment in its result, not by inspecting the string for substrings

#### Scenario: A sha that is not a sha is refused

- **WHEN** source `shared` is fetched at sha `not-a-sha`
- **THEN** it fails with
  `source "shared": resolved "not-a-sha" is not a 40-character hex sha`
- **AND** no directory is created under the cache root, which stays exactly as it was

### Requirement: The cache root defaults to `~/.cache/graft`

`internal/source` SHALL offer the default cache root as a value the caller passes in, never
as a global it reads for itself, so every test names its own root and no test can write to
the developer's real cache. The default SHALL be `$XDG_CACHE_HOME/graft` when
`XDG_CACHE_HOME` is set to an absolute path, and `<home>/.cache/graft` otherwise — the two
spellings of the one location SPEC.md names.

Computing the default SHALL create nothing; the directory is made when the first entry is
written. When neither `XDG_CACHE_HOME` nor a home directory can be determined, it SHALL fail
with `cannot determine the cache root: <reason>` rather than falling back to a relative path
or to the working directory.

#### Scenario: The default root under a home directory

- **WHEN** `XDG_CACHE_HOME` is unset and the home directory is `/home/dev`
- **THEN** the default cache root is `/home/dev/.cache/graft`
- **AND** neither `/home/dev/.cache` nor `/home/dev/.cache/graft` is created

#### Scenario: `XDG_CACHE_HOME` moves the default root

- **WHEN** `XDG_CACHE_HOME` is `/var/tmp/cache`
- **THEN** the default cache root is `/var/tmp/cache/graft`

#### Scenario: A relative `XDG_CACHE_HOME` is ignored

- **WHEN** `XDG_CACHE_HOME` is `relative/cache`
- **THEN** the default cache root falls back to `<home>/.cache/graft`, because a cache root
  that moves with the working directory would give the same source two entries

#### Scenario: No home directory and no `XDG_CACHE_HOME` is an error

- **WHEN** `XDG_CACHE_HOME` is unset and the home directory cannot be determined
- **THEN** it fails with a message beginning `cannot determine the cache root: `
- **AND** it does not fall back to `.cache/graft` relative to the working directory, which
  would put a cache inside whatever repository happened to be current

### Requirement: A fetch populates the cache with the tree at that sha

On a cache miss, `internal/source` SHALL fetch the named sha from the source's clone URL
and leave the tree's files at the entry's path. The entry SHALL hold the source's files and
**no `.git` directory**: the cache stores a tree, not a repository, which keeps the entry
small and keeps anything that walks it from finding a repository it did not expect.

A fetch SHALL write only under the cache root. It SHALL create, modify, and delete nothing
in the repository graft is running in — `internal/apply` remains the only package that
writes there.

**Holding that takes more than not naming the consumer's paths.** graft may be running
*inside* a git operation — a `post-merge` hook, `git rebase --exec`, `git bisect run` — and
git sets `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_OBJECT_DIRECTORY`,
`GIT_COMMON_DIR` and their relatives when it does. Inherited by the internal checkout,
`GIT_INDEX_FILE` makes it rewrite the consumer's index and `GIT_OBJECT_DIRECTORY` deposits
the source's objects in the consumer's `.git/objects`. Every such variable SHALL be removed
from the environment of every git command graft runs. Nothing else SHALL be removed:
`GIT_ASKPASS`, `SSH_AUTH_SOCK`, and the user's `credential.helper` are how a private source
works at all.

A fetch SHALL likewise not run the consumer's git **hooks**. A globally configured
`core.hooksPath` applies to every repository, including the one graft creates for its own
use, and a `post-checkout` hook firing there can write anywhere. Hooks are not
source-controlled, so this is not an execution route a source opens — it is the same
containment claim, which a hook would make untrue in an ordinary setup.

**An entry SHALL hold the bytes the commit recorded.** `git checkout` normally honours the
source's own committed `.gitattributes`, which is a file the source controls: `* text
eol=crlf` rewrites every line ending, `ident` expands `$Id$`, and `filter=lfs` selects a
filter driver whose command comes from the *consumer's* git configuration. The last is the
serious one — it is a source-controlled file causing a program to run, which is exactly what
ENGINEERING.md says a source cannot do. The checkout SHALL therefore run with in-tree
attributes disabled, so no `.gitattributes` in the fetched commit can alter a byte or select
a filter.

Because git **ignores an unknown configuration key in silence**, a graft running against a
git too old to know that setting would drop the defence with nothing said. graft SHALL
therefore refuse to fetch with a git older than the version that honours it, failing with
`git <version> is too old: graft needs git <min> or newer` — an environmental failure like
`git not found on PATH`, and carrying no source prefix for the same reason: no other source
would fare better.

#### Scenario: A first fetch writes the tree

- **WHEN** the cache is empty and source `shared`, a repository whose commit
  holds `catalog.yaml` and `extras/agents/a.md`, is fetched at that commit's sha
- **THEN** the entry directory holds `catalog.yaml` and `extras/agents/a.md` with the
  contents that commit recorded
- **AND** the entry holds no `.git` directory

#### Scenario: A fetch of an older commit gets that commit's tree

- **WHEN** the source repository has two commits and the **first** one's sha is fetched
- **THEN** the entry holds the first commit's files, not the tip's, and a file added by the
  second commit is absent

#### Scenario: A source's `.gitattributes` does not alter the cached bytes

- **WHEN** the source's commit holds `.gitattributes` declaring `* text eol=crlf` and
  `*.md ident`, alongside `a.txt` whose blob is `hello\nworld\n` and `b.md` whose blob is
  `$Id$\n`
- **THEN** the entry's `a.txt` is byte-identical to the blob — LF endings, not CRLF — and
  its `b.md` still reads `$Id$`
- **AND** the assertion is made against `git cat-file blob`'s output for those paths, so it
  compares the entry to what the commit actually recorded rather than to a literal a test
  author guessed

#### Scenario: An inherited git environment does not reach the consumer's repository

- **WHEN** a fetch runs with `GIT_INDEX_FILE`, `GIT_DIR`, `GIT_WORK_TREE`,
  `GIT_OBJECT_DIRECTORY`, or `GIT_COMMON_DIR` pointing at a consumer repository, as git
  itself sets them when it runs a hook
- **THEN** the fetch succeeds and the consumer repository is byte-identical afterwards,
  `.git` included — its index unchanged and no new object written
- **AND** each variable is asserted on its own, because they fail differently and the quiet
  ones are the dangerous ones: `GIT_WORK_TREE` makes the fetch fail loudly, while
  `GIT_INDEX_FILE` rewrites the consumer's index and reports success

#### Scenario: A consumer's git hooks do not run during a fetch

- **WHEN** the consumer's git configuration sets `core.hooksPath` to a directory holding an
  executable `post-checkout` hook, and a fetch runs
- **THEN** the hook does not run

#### Scenario: A git too old to disable in-tree attributes is refused

- **WHEN** the available git is older than the version that honours the setting
- **THEN** the fetch fails with `git <version> is too old: graft needs git <min> or newer`
- **AND** the failure is loud rather than silent, which is the point: git ignores an unknown
  configuration key without complaint, so an unchecked old git would quietly restore a
  source's power to rewrite the bytes it is cached as

#### Scenario: A fetch writes nothing outside the cache root

- **WHEN** a fetch runs with the cache root set to a directory of its own
- **THEN** every path created is under that root, and the consumer's repository — including
  `graft.toml`, `graft.lock`, and every destination directory — is untouched

### Requirement: An existing cache entry is reused without contacting the remote

When the entry's path already exists as a directory, `internal/source` SHALL return it and
run **no git command at all**. This is the whole of SPEC.md's "network unavailable, cache
hit: proceeds" — a `sync` against a lock whose shas are already cached does no network I/O,
so it works on a plane.

On a cache **miss** with the remote unreachable, the fetch SHALL fail with
`source "<name>": cannot fetch "<sha>" from "<url>": <first line of git's output>`, naming
what it needed to fetch and where from.

#### Scenario: A second fetch of the same sha works with the remote gone

- **WHEN** a sha has been fetched into the cache, and the source repository is then deleted
  so its clone URL names nothing
- **THEN** fetching the same sha again succeeds and returns the same entry
- **AND** its files are still readable, which is only possible if no git command ran

#### Scenario: A cache miss with no reachable remote is an error naming both

- **WHEN** the cache is empty and source `shared` is fetched at sha
  `fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5` from a clone URL naming a directory that does
  not exist
- **THEN** it fails with a message beginning
  `source "shared": cannot fetch "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5" from "<url>": `
- **AND** the message names the sha and the URL, so a user offline knows exactly what to be
  online for

#### Scenario: A sha that the remote does not have is the same error

- **WHEN** the remote is reachable but holds no commit
  `0000000000000000000000000000000000000000`, and that sha is fetched
- **THEN** it fails with the same `cannot fetch ... from ...` message
- **AND** nothing is left at the entry's path

### Requirement: A partial fetch never becomes a cache entry

`internal/source` SHALL fetch into a temporary directory beside the entry's final path and
publish it by renaming, so the entry's existence means the entry is complete. A fetch that
fails at any step SHALL leave no directory at the entry's path and SHALL leave no temporary
directory behind.

If the rename fails because another process published the same entry first, the fetch SHALL
treat that as a cache hit rather than an error: two graft runs racing on one sha both get
the same immutable tree.

#### Scenario: A failed fetch leaves the cache as it found it

- **WHEN** a fetch fails because the remote does not have the sha
- **THEN** no directory exists at the entry's path
- **AND** the entry's parent directory holds no leftover temporary directory

#### Scenario: A fetch into an unusable cache root fails without a partial entry

- **WHEN** the cache root names an existing **file** rather than a directory
- **THEN** the fetch fails with a message beginning
  `source "shared": cannot create cache entry for "<sha>": `
- **AND** the file is left exactly as it was, byte for byte
