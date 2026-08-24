<!-- No outer-loop acceptance group. design.md → Test Strategy records why: no command
     exists yet (`command-surface` and `sync-command` are later changes) and `internal/apply`
     — the only thing that could make a fetch observable in a working tree — does not exist,
     so an end-to-end group here could only drive an invented harness, a boundary the Test
     Boundaries table does not name. Group 12 is placed last instead: it runs the real chain
     from a fixture repository into the real `plan.Build`, and it is the one test that can
     fail for a reason none of the others can. -->

## 1. Package scaffold and the fixture-repository harness
<!-- kind: operational -->

**Concentration point.** A fixture git repository needs `user.name` and `user.email` set on
**the repository**, not the machine, or commits fail on a clean CI runner. Every integration
group below builds on this helper, so a harness that only works on a developer laptop makes
eleven groups green locally and red in CI.

- [x] 1.1 CHECK: Create `internal/source/source.go` holding only the package doc comment —
      `source` is the only package that runs `git`; it writes only under the cache root it is
      given, never into the repository graft runs in — and confirm `go build ./internal/source/`
      succeeds
- [x] 1.2 CHANGE: Add `internal/source/fixture_test.go` with a `newRepo(t)` helper that
      creates a repository under `t.TempDir()` with `git -c init.defaultBranch=main init -q`,
      then `git -C <dir> config user.name` and `git -C <dir> config user.email` — repository
      scope, never `--global` — and methods to write a file, commit, tag lightweight, tag
      annotated, and branch, each returning the resulting sha
- [x] 1.3 VERIFY: Prove the repository-scope config is load-bearing rather than assumed: run
      one commit-making helper call under `env -u HOME GIT_CONFIG_GLOBAL=/dev/null
      GIT_CONFIG_SYSTEM=/dev/null go test ./internal/source/ -run Fixture` and confirm it
      passes, which is the clean-CI-runner condition reproduced locally
- [x] 1.4 VERIFY: Confirm the helper sets the default branch explicitly, so a fixture's branch
      name does not depend on the runner's git configuration, and confirm `go test
      ./internal/source/` is green

## 2. Clone URL expansion
<!-- kind: behavior -->

Pure, and the only part of this change that touches no filesystem and no subprocess.

- [x] 2.1 RED: Write failing table tests for *Shorthand expands to HTTPS*; *A URL carrying a
      scheme is passed through*; *An scp-style address is passed through*; *A filesystem path
      is passed through* — the last covering both `/srv/mirrors/openspec-schemas` and
      `../sibling-repo`. Assert that no directory is created and no command is run by giving
      the test no temp dir at all
- [x] 2.2 RED: Write *A remote that looks like an option is refused*, asserting `source
      "shared": git "--upload-pack=./pwn.sh" may not begin with "-"`, and assert the same
      refusal from `Resolve` and from `Fetch` with `PATH` emptied, so a green test cannot mean
      "git ran and declined". This is a verified arbitrary-execution path, not a hypothetical:
      `git ls-remote --upload-pack=./pwn.sh refs/tags/v1` runs the script and promotes the
      refspec to the repository operand (design.md → D2)
- [x] 2.3 GREEN: Add `CloneURL(name, git string) (string, error)`, refusing a value beginning
      with `-` before anything else, then expanding only the shorthand form — no scheme, no
      `user@host:` prefix, not a filesystem path, and a first segment that looks like a
      hostname — to `https://` + the value, and passing every other form through unchanged
- [x] 2.4 CHECK: Contract gate — re-read SPEC.md's `graft.toml` section and confirm `git` is
      still documented as "anything `git clone` accepts" with shorthand `host/owner/repo`
      expanding to HTTPS, and that expansion still rewrites nothing else. Record that refusing
      a leading `-` narrows "anything `git clone` accepts" deliberately: a leading-dash value
      is not a remote `git clone` accepts, it is an option
- [x] 2.5 REFACTOR: Confirm the hostname test is one predicate used once, or state that no
      refactor was needed
- [x] 2.6 Run `go test ./internal/source/` — no regressions

## 3. Running git, and its absence
<!-- kind: behavior -->

The seam every later group uses: explicit argv, no shell, prompting disabled, stderr reduced
to its first line.

- [x] 3.1 RED: Write a failing test for *git is not on PATH* — `t.Setenv("PATH", t.TempDir())`,
      then `Resolve` — asserting the exact message `git not found on PATH`. Assert it is not
      an `exec.Error` surfaced verbatim, which a user cannot act on
- [x] 3.2 GREEN: Add the unexported git runner: `exec.LookPath("git")` first, mapping its
      failure to `git not found on PATH`; `exec.Command` with an explicit argv and never a
      shell; the child's environment being the parent's plus `GIT_TERMINAL_PROMPT=0` and
      nothing else scrubbed (design.md → D3); stderr captured into a `bytes.Buffer`
- [x] 3.3 GREEN: Separate options from operands with `--` in every invocation that takes a
      URL — `ls-remote -- <url> <refs…>` and `remote add origin -- <url>` — as the second of
      the two independent guards in design.md → D2. An explicit argv alone does not stop a
      value from becoming a flag; argv position is not what git uses to tell one from the
      other
- [x] 3.4 GREEN: Add the unexported `firstLine` helper reducing a captured stderr buffer to
      its first non-empty trimmed line, so a per-source error never carries git's terminal
      advice on later lines
- [x] 3.5 REFACTOR: Confirm every git invocation in the package goes through this one runner —
      no second `exec.Command` anywhere — or state that no refactor was needed
- [x] 3.6 Run `go test ./internal/source/` — no regressions

## 4. Rev resolution
<!-- kind: behavior -->

`git ls-remote` against three explicit refs, with peeled tag beating tag beating branch, and
a full sha short-circuiting before anything can fail environmentally.

- [x] 4.1 RED: Write failing tests for *A branch resolves to its tip*; *A lightweight tag
      resolves to its commit*; *An annotated tag resolves to the commit, not the tag object*;
      *A tag wins over a branch of the same name*; *A full sha passes through without
      contacting the remote*; *An uppercase sha is not treated as a sha*. Build every fixture
      with the group 1 helper. The annotated-tag test asserts both that the returned sha is
      the commit **and** that it differs from the tag object's own sha — asserting only the
      first would pass against an implementation that never peels
- [x] 4.2 RED: The full-sha test gives a clone URL naming a path that does not exist **and**
      empties `PATH`, so it can only pass if no git command runs. Asserting merely that the
      value comes back would stay green against an implementation that resolves it remotely
- [x] 4.3 GREEN: Add `Resolve(name, git, rev string) (string, error)`. Return `rev` unchanged
      when it is 40 lowercase hex characters, **before** `exec.LookPath` (design.md → D5)
- [x] 4.4 GREEN: Invoke `git ls-remote -- <url> refs/tags/<rev> refs/tags/<rev>^{}
      refs/heads/<rev>`, and compare each output line's ref column to those three names with
      `==` rather than trusting the pattern match — `ls-remote` matches the tail of a ref
      name, so a pattern alone would let `refs/heads/x/refs/tags/v1` answer for `v1`
      (design.md → D4)
- [x] 4.5 GREEN: Apply the precedence peeled tag → tag → branch, so a pin names the immutable
      thing
- [x] 4.6 CHECK: Contract gate — re-read SPEC.md's `graft.lock` section and confirm `resolved`
      is still documented as the sha a `rev` became, that `rev` records the request
      unchanged, and that the format carries no content hash this resolution would have to
      produce
- [x] 4.7 REFACTOR: Confirm the three ref names are constructed in one place and used by both
      the invocation and the comparison, so they cannot drift apart, or state that no refactor
      was needed
- [x] 4.8 Run `go test ./internal/source/` — no regressions

## 5. Rev resolution failures
<!-- kind: behavior -->

**Concentration point.** Error strings are asserted by tests. Every message below is a
deliberate contract; changing one later is a contract change, never an incidental edit.

- [x] 5.1 RED: Write failing tests for *A rev no ref matches*, asserting `source "shared": rev
      "v9.9.9" not found`; *An abbreviated sha is not a rev*, asserting `source "shared": rev
      "47f73fc" not found` against a repository where that abbreviation is real; *An empty
      rev*, asserting `source "shared": rev is empty` with `PATH` emptied to prove no git ran;
      *An unreachable remote*, asserting the prefix `source "shared": cannot reach "<url>": `
- [x] 5.2 RED: The unreachable-remote test additionally asserts the message contains no
      newline, so git's later lines of terminal advice cannot leak into a per-source error,
      and asserts that the failure is distinguishable from the not-found one — one is a typo
      in `graft.toml`, the other a network or permission problem
- [x] 5.3 GREEN: Return the empty-rev error before any lookup; map a zero-exit run with no
      matching ref line to the not-found error; map a non-zero exit to the unreachable error
      carrying `firstLine` of stderr. On every failure the returned sha is `""`, asserted in
      each case
- [x] 5.4 REFACTOR: Extract the per-source error prefix into one closure, as `catalog.errf`
      and `plan.itemErrf` already do, or state that no refactor was needed
- [x] 5.5 Run `go test ./internal/source/` — no regressions

## 6. The cache path
<!-- kind: behavior -->

**Concentration point in the same family as `plan`'s repo-root rule: a remote must not be
able to aim a write outside the root graft chose.** Here the root is the cache rather than
the working tree, and the derivation is pure so it can be checked before anything exists.

- [x] 6.1 RED: Write failing tests for *The cache path mirrors the remote and the sha*; *The
      same repository over ssh and over HTTPS is one entry*, whose cases include a URL
      carrying **both** a `.git` suffix and a trailing slash — trimming them in the wrong
      order gives one repository two entries; *A filesystem remote gets an entry under
      `local`*; *A sha that is not a sha is refused*, asserting the exact message
      `source "shared": resolved "not-a-sha" is not a 40-character hex sha` — byte for byte
      the message `internal/plan` already produces, and the tail of `internal/lock`'s, which
      carries a `graft.lock: ` prefix of its own. Three packages must not disagree about what
      a valid `resolved` is
- [x] 6.2 RED: Write *A hostile remote cannot escape the cache root* as a table of hostile
      URLs — `..` segments, separators inside a segment, an empty host, a segment that is
      exactly `.` — asserting for each that `filepath.Rel(root, entry)` succeeds and its
      result contains no `..` segment. Assert containment with `filepath.Rel`, not by
      inspecting the string, because `filepath.Rel` is what actually answers the question
- [x] 6.3 RED: Every case asserts the cache root was **not created**, so deriving a path
      stays a pure function a caller can ask speculatively
- [x] 6.4 GREEN: Add `type Cache struct{ Root string }` and `func (c Cache) Entry(name, git,
      sha string) (string, error)` returning `<Root>/<host>/<path…>/<sha>`. Parse the three
      URL forms `CloneURL` can produce — scheme, scp-style, filesystem path — reducing each to
      host plus path, dropping scheme, user, port, and a trailing `.git` (design.md → D6)
- [x] 6.5 GREEN: Trim a trailing `/` **before** a trailing `.git`, derive `local` as the host
      segment when the URL has none, and pass every segment through the unexported
      `safeSegment`: characters outside `[A-Za-z0-9._-]` become
      `-`, a segment of `.` or `..` is prefixed with `_`, empty segments are dropped. Refuse a
      bad `sha` before any of this
- [x] 6.6 REFACTOR: Confirm `safeSegment` is applied per segment after splitting — so a
      segment can neither contain a separator nor climb — and that it lives in this package
      with its own wording rather than being shared with `plan.insideRepo` or
      `lock.isRepoRelative`, which enforce different rules (design.md → Boundaries). Or state
      that no refactor was needed
- [x] 6.7 Run `go test ./internal/source/` — no regressions

## 7. The default cache root
<!-- kind: behavior -->

- [x] 7.1 RED: Write failing tests for *The default root under a home directory*; *`XDG_CACHE_HOME`
      moves the default root*; *A relative `XDG_CACHE_HOME` is ignored*; *No home directory and
      no `XDG_CACHE_HOME` is an error*, asserting the prefix `cannot determine the cache root: `
      and that no relative fallback is returned — a cache inside whichever repository happened
      to be current is worse than a failure. Drive them through the
      unexported seam with stub `getenv` and `home` functions, never `t.Setenv` — a test that
      has to set `HOME` is one edit away from writing to the developer's real cache
      (design.md → D7)
- [x] 7.2 RED: Each case asserts that neither `<home>/.cache` nor `<home>/.cache/graft` is
      created, using paths under `t.TempDir()` so the assertion is real rather than notional
- [x] 7.3 GREEN: Add `defaultCacheRoot(getenv func(string) string, home func() (string,
      error)) (string, error)` honouring `XDG_CACHE_HOME` only when it is absolute, and the
      exported `DefaultCacheRoot()` calling it with `os.Getenv` and `os.UserHomeDir`
- [x] 7.4 REFACTOR: Confirm the cache root reaches `Cache` as a field the caller passes and is
      never read from a global inside the package, which is what keeps every test on its own
      root. Or state that no refactor was needed
- [x] 7.5 Run `go test ./internal/source/` — no regressions

## 8. Fetching into the cache
<!-- kind: behavior -->

**Concentration point.** `internal/apply` is the only package permitted to write to the
working tree, and it does not exist yet. This group writes files for the first time in the
project, and every one of them must land under the cache root the caller named.

- [x] 8.1 RED: Write failing tests for *A first fetch writes the tree*, asserting file
      contents and that the entry holds **no `.git`**; *A fetch of an older commit gets that
      commit's tree*, asserting a file added by the second commit is absent
- [x] 8.2 RED: Write *A source's `.gitattributes` does not alter the cached bytes* against a
      fixture committing `* text eol=crlf` and `*.md ident`, asserting each entry file is
      byte-identical to `git cat-file blob <sha>:<path>` rather than to a literal a test
      author guessed. Verified failure without the fix: the blob `hello\nworld\n` lands as
      `hello\r\nworld\r\n` and `$Id$` becomes a real hash. The reason this is not cosmetic
      is `filter=lfs`, which selects a driver whose command comes from the **consumer's** git
      config — a source-controlled file causing a program to run
- [x] 8.3 RED: Write *A fetch writes nothing outside the cache root* as a positive assertion:
      build a fixture consumer tree under a second `t.TempDir()` holding `graft.toml`,
      `graft.lock`, and a destination directory, snapshot every path and its bytes, run the
      fetch, and assert the snapshot is identical. A test that only checks the cache root
      would stay green if a write landed anywhere else
- [x] 8.4 GREEN: Add `func (c Cache) Fetch(name, git, sha string) (string, error)`: create the
      entry's parent, `os.MkdirTemp` a sibling scaffold there so the publishing rename stays
      within one filesystem, `os.Mkdir` the work tree inside it — `checkout --work-tree`
      fails with `fatal: this operation must be run in a work tree` without it — and `defer
      os.RemoveAll` the scaffold on every path
- [x] 8.5 GREEN: Fetch with the bare git directory **beside** the work tree, never inside it,
      so no `.git` ever exists within the tree that is published: `init -q --bare tmp/git`,
      `remote add origin -- <url>`, `fetch --depth 1 --no-tags -q origin <sha>`, then
      `-c attr.tree=4b825dc642cb6eb9a060e54bf8d69288fbee4904 -c core.bare=false
      --work-tree=tmp/tree checkout -q --detach FETCH_HEAD`. `attr.tree` pointed at the empty
      tree is what disables the source's in-tree `.gitattributes`; `-c core.autocrlf=false -c
      core.eol=lf` was tried and does not close it (design.md → D8, D9)
- [x] 8.6 CHECK: Persistence gate — confirm what this change requires of stored data:
      migration, backfill, seeding, and index rebuild all **none**; cache invalidation
      **none by construction**, because an entry is keyed by an immutable commit sha, which is
      exactly why an incomplete entry would be wrong forever and why group 9 exists
      (design.md → Persistence and Rollout)
- [x] 8.7 REFACTOR: Confirm the fetch writes nothing outside `c.Root` — no temp file in the
      system temp directory, no config written to the user's home — or state that no refactor
      was needed
- [x] 8.8 Run `go test ./internal/source/` — no regressions

## 9. Cache hits, fetch failures, and atomic publication
<!-- kind: behavior -->

The offline guarantee and its precondition. A hit runs no git command at all, which is only
safe because an entry cannot exist unless it is complete.

- [x] 9.1 RED: Write a failing test for *A second fetch of the same sha works with the remote
      gone*: fetch, then `os.RemoveAll` the source repository **and** `t.Setenv("PATH",
      t.TempDir())`, then fetch again and assert the same entry with readable files. Deleting
      the repository alone is not enough — emptying `PATH` is what proves no git command ran,
      which is what SPEC.md's "network unavailable, cache hit: proceeds" actually claims
- [x] 9.2 RED: Write failing tests for *A cache miss with no reachable remote is an error
      naming both*, asserting the prefix `source "shared": cannot fetch "<sha>" from "<url>": `
      and no newline in the message. The prefix is graft's and is asserted exactly; git's own
      first line is not, because over the local transport two processes write the same pipe and
      which line arrives first is not deterministic (design.md → D13). Then *A sha that the
      remote does not have is the same error*, additionally asserting nothing exists at the
      entry's path; and *A fetch into an unusable cache root fails without a partial entry*,
      with the root a regular file, asserting the prefix `source "shared": cannot create cache
      entry for "<sha>": ` and that the file's bytes are unchanged
- [x] 9.3 RED: Write *A failed fetch leaves the cache as it found it*, asserting both that the
      entry path does not exist **and** that the entry's parent directory holds no leftover
      directory of any name. Asserting only the entry path would stay green against an
      implementation that abandons its scaffold
- [x] 9.4 GREEN: Return the entry immediately when it already exists as a directory, before
      `exec.LookPath` and before any subprocess
- [x] 9.5 GREEN: Publish by `os.Rename(tmp/tree, entry)`. When the rename fails because the
      destination exists, re-`Stat` the entry and treat a directory there as a hit rather than
      an error — two runs racing on one sha both want the same immutable tree (design.md → D8)
- [x] 9.6 REFACTOR: Confirm every error path leaves the cache root exactly as it found it, and
      that the cleanup is a single `defer` rather than a removal repeated per failure branch.
      Or state that no refactor was needed
- [x] 9.7 Run `go test ./internal/source/` — no regressions

## 10. Reading the catalog from a fetched tree
<!-- kind: behavior -->

- [x] 10.1 RED: Write failing tests for *A catalog in the fetched tree parses*, asserting
      version, one kind, and one item whose id is `schema:tdd`; and *A source with no catalog
      is not graftable*, asserting the exact message `catalog.yaml not found: the source is
      not graftable` and that it is `internal/catalog`'s own wording rather than a second
      spelling of the same failure
- [x] 10.2 RED: Write *A `catalog.yaml` leaving the entry is not read*: plant a real file
      outside the entry, symlink `catalog.yaml` at it with an absolute target, and assert the
      read fails and that the outside file's contents appear in neither the result nor the
      error. A source commits its own `catalog.yaml`, so it may commit a symlink under that
      name, and `os.ReadFile` follows one
- [x] 10.3 RED: Assert the entry is unchanged by the read — no file created, modified, or
      deleted — and that no path other than `catalog.yaml` **resolved inside the entry** is
      opened
- [x] 10.4 GREEN: Add `ReadCatalog(entry string) (*catalog.Catalog, error)` reading the bytes
      through `os.OpenRoot(entry)` and parsing them with `catalog.Parse`. Delegate to
      `catalog.Load` only on `fs.ErrNotExist` — where there is by definition no link to follow
      — so the not-graftable wording keeps exactly one owner (design.md → D12)
- [x] 10.5 CHECK: Contract gate — re-read SPEC.md's `catalog.yaml` section and confirm the file
      is still documented as living at the source's root under exactly that name, and that its
      absence is still an error with no fallback to guessing a layout
- [x] 10.6 Run `go test ./internal/source/` — no regressions

## 11. Listing an item's `from`
<!-- kind: behavior -->

**Concentration point.** A listing's order is what keeps `graft.lock`'s `files` from
churning — the same determinism requirement the lock itself carries, one layer earlier.
Sorted, slash-separated, and stable across platforms, or every sync produces a diff.

- [x] 11.1 RED: Write failing tests for *A `from` naming a file lists exactly that file*,
      asserting `Dir` is false and `Files` is exactly `["apply-orchestrator.md"]` — the base
      name, which is what `destination-computation` requires for a file item; *A `from` naming
      a directory lists its tree*, asserting exact slice equality **including order**, not set
      equality; *An empty directory lists nothing and is still a directory*; *A directory
      holding only empty subdirectories lists nothing*
- [x] 11.2 RED: Write *A symlink is not listed*, with the link pointing at
      `../../../../etc/passwd`, asserting the link's name is absent, the real file is present,
      and **no error** is returned — one stray link may not make a valid source unusable
- [x] 11.3 RED: Write failing tests for *A `from` that does not exist is an error naming the
      item*, asserting `source "shared": item "schema:tdd": from "extras/gone" not found in
      the source tree`; and *A `from` naming a symlink is refused*, asserting `source
      "shared": item "schema:tdd": from "extras/tdd" is not a regular file or directory` and
      that the link target is never read. Both assert the returned listing is the zero value,
      so no caller can plan a write from a failed listing
- [x] 11.4 RED: Write *A `from` reached through a symlinked parent is refused*, the case
      refusing the last component alone does not cover: plant a real `id_rsa` outside the
      entry, make `extras` a symlink to that outside directory, and declare `from:
      extras/tdd`. Assert the listing fails and that neither `id_rsa` nor its contents appear
      anywhere. Verified to succeed without the fix — `os.Lstat` does not follow the final
      element but does resolve every intermediate one, and `catalog.inSource` sees nothing
      wrong with the string
- [x] 11.5 RED: Write *A `from` naming a submodule lists nothing*, against a fixture with a
      committed gitlink, asserting a directory listing with zero files, no error, and that no
      second repository was contacted
- [x] 11.6 RED: Write *Listing changes nothing*: snapshot the entry's paths, modes, sizes, and
      bytes, take every listing above, and assert the snapshot is identical afterwards and
      that nothing was created in a consumer tree built beside it
- [x] 11.7 GREEN: Add `List(entry, name string, it catalog.Item) (plan.Listing, error)`
      returning `plan.Listing` **itself**, not a convertible twin — `source-listing`'s "usable
      with no adaptation, same type" is enforced by the type system rather than by a test
      (design.md → D1)
- [x] 11.8 GREEN: Do every path operation through `os.OpenRoot(entry)`, never through a joined
      path: `root.Lstat(from)` for the file-or-directory question — `Lstat`, never `Stat`, so a
      symlink is answered about rather than followed — then `root.OpenRoot(from)` and
      `fs.WalkDir(fromRoot.FS(), ".")` for a directory, which contains the walk to the item's
      own subtree and never follows a link. Admit only `d.Type().IsRegular()`; relativise with
      `filepath.Rel`, convert with `filepath.ToSlash`, and sort ascending (design.md → D10, D11)
- [x] 11.9 CHECK: Contract gate — re-read SPEC.md's `graft.lock` section and confirm the
      per-item `files` list this listing ultimately feeds is still documented as sorted by
      path and as the sole authority for deletion, so an unsorted listing here would churn the
      lock's diff on every sync
- [x] 11.10 REFACTOR: Confirm the item error prefix reuses group 5's closure shape rather than
      formatting `source %q: item %q: ` a second time, and that no read below an entry bypasses
      its `*os.Root`. Or state that no refactor was needed
- [x] 11.11 Run `go test ./internal/source/` — no regressions

## 12. A fetched source drives a plan
<!-- kind: behavior -->

This group discharges `destination-and-plan`'s deferred note that whether a `Listing`
faithfully describes a real fetched tree is `git-fetch`'s contract. It is scheduled last
because it is the only test that can fail for a reason none of the others can.

- [ ] 12.1 RED: Write a failing test for *A fetched fixture plans the writes its tree
      implies*: build a fixture repository holding `catalog.yaml`, `extras/tdd/schema.yaml`,
      `extras/tdd/templates/proposal.md`, and `extras/agents/apply-orchestrator.md`; tag it
      `v1.0.0`; then run the real chain — `Resolve`, `Fetch`, `ReadCatalog`, `catalog.Expand`
      with `["schema:tdd", "agent:*"]`, `List` per item — into the real `plan.Build` against
      an empty lock. Assert the writes land at `openspec/schemas/tdd/schema.yaml`,
      `openspec/schemas/tdd/templates/proposal.md`, and
      `.claude/agents/apply-orchestrator.md`
- [ ] 12.2 RED: Assert additionally that **every** `Write.From`, joined under the fetched
      entry, names a file that actually exists — the property a hand-written `Listing` in a
      `plan` unit test cannot check, and the one that would catch the file-item asymmetry
      where `Write.From` is `item.From` itself rather than a path below it
- [ ] 12.3 RED: Run the whole chain twice against the same fixture and assert
      `bytes.Equal(lock.Marshal(a.Lock), lock.Marshal(b.Lock))`. Byte equality, not
      `reflect.DeepEqual` — the determinism concentration point, asserted here over values a
      real directory walk produced rather than over literals
- [ ] 12.4 GREEN: Make it pass with no new production code if the earlier groups are correct;
      if it needs any, that is a defect in a group above and is fixed there rather than
      patched here
- [ ] 12.5 CHECK: Confirm `internal/plan` is still pure — `go test ./internal/plan/ -run
      Impure` green — and that nothing in this group added a filesystem call to `plan`. A test
      that needs a real directory to exercise plan logic is a signal the boundary moved; this
      test needs one to exercise **source** logic, and it lives in `./internal/source/`
- [ ] 12.6 Run `go test ./internal/source/ ./internal/plan/` — no regressions

## 13. Documentation
<!-- kind: operational -->

- [ ] 13.1 CHECK: Re-read SPEC.md's Resolution section and its failure-mode table and confirm
      what this change implemented still matches: step 2's `git ls-remote`, step 3's
      content-addressed cache path, step 4's catalog read, and the two network rows ("cache
      hit: proceeds", "cache miss: error naming what it needed to fetch")
- [ ] 13.2 CHANGE: Add to AGENTS.md's "Rules that are easy to get wrong" **only** durable
      pitfalls this change actually discovered and that the existing rules do not already
      cover. Two candidates: a cache entry must never be written in place, because an entry
      keyed by an immutable sha is never re-fetched, so a partial one is wrong forever; and
      "graft executes nothing from a source" now has three specific ways it can be broken by
      accident — a `git` value that becomes an option, a committed `.gitattributes` selecting
      a filter driver, and a symlink followed out of an entry. Net addition: ~6 lines. Weigh
      each against the existing rules and add only what survives; if neither does, add nothing
      and say so
- [ ] 13.3 VERIFY: Confirm no section of SPEC.md, PRD.md, ENGINEERING.md, or AGENTS.md now
      contradicts what was built, and that `openspec/IMPLEMENTATION-ORDER.md`'s Phase 3 row
      still describes this change accurately

## 14. Change Review
<!-- kind: operational -->

- [ ] 14.1 CHECK: Dispatch an independent reviewer — a fresh subagent given only proposal.md,
      the three spec files, design.md, tasks.md, and the diff, never a fork of the
      implementing session — with these concentration points named: (a) a cache hit runs **no**
      git command, and the test would go red if it did; (b) no path this package writes can
      land outside the cache root, for any URL a remote controls; (c) an interrupted or failed
      fetch can never leave a directory at an entry's path; (d) nothing is written into the
      repository graft runs in, `internal/apply` still being the only future writer; (e) every
      asserted error string matches the specs and design.md → Contracts exactly; (f) no code
      path lets a source repository cause anything to execute — no hook, no config, no
      filter, no submodule, and no `.git` inside a published entry; (g) no value from `graft.toml`
      or from a source can become a git **option** rather than an operand, and no committed
      `.gitattributes` can select a filter driver; (h) no read below an entry can resolve
      outside it, through any component of `from` and not only its last; (i) nothing was added
      to `cmd/graft`, where the coverage gate cannot see it
- [ ] 14.1a CHECK: Name to that reviewer the three execution and containment holes an earlier
      review already found and this plan closes, and ask specifically whether the fixes hold:
      a `git` value beginning with `-`, a committed `.gitattributes`, and a symlinked parent
      component of `from`. A reviewer told only the general shape of the threat tends to
      re-derive the same three; the useful answer is a fourth
- [ ] 14.2 CHANGE: Fix every CRITICAL, resolve or consciously accept each WARNING with a
      one-line reason, note each SUGGESTION, and re-run the affected tests
- [ ] 14.3 VERIFY: Confirm no blocking or unowned finding remains, and that any contract
      changed while fixing findings was written back into the owning artifact and
      planning-review.md

## 15. Lint & Verify
<!-- kind: operational -->

- [ ] 15.1 CHECK: Inspect the intended verification commands and affected tiers — two tiers
      here (unit and integration, both `./internal/source/`), both plain `go test` with no
      build tag and no network, and confirm the integration tier is visible to the coverage
      gate rather than hidden behind a tag
- [ ] 15.2 VERIFY: Run `task lint` — golangci-lint clean and `gofumpt -l` silent, 0 errors
- [ ] 15.3 VERIFY: Run `task test` — `go test -race ./...` green
- [ ] 15.4 VERIFY: Run `task cover` — at or above the 80% floor measured over `./internal/...`,
      and report `internal/source`'s own figure alongside the total
- [ ] 15.5 VERIFY: Run `task build` — the binary builds, which is Go's type check
- [ ] 15.6 VERIFY: Run `openspec validate git-fetch --strict` — clean
