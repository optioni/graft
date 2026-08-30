## Why

`rev` names one immutable thing, so moving a source forward means editing `graft.toml` by
hand and knowing which tag to write. A consumer tracking a source that tags regularly does
that edit every release, and `graft update` — the command whose whole job is moving a pin —
cannot help, because it re-resolves the rev it was given rather than choosing a newer one.
`rev = "main"` is the escape hatch today and it is the wrong one: it pins nothing, adopts
every commit, and gives up the review story pinning exists for.

A range is the missing middle. `rev = "^1.2.0"` says *the newest 1.x, and tell me which one
you picked* — a stable request that still moves on demand, resolved once by `update` and
recorded in the lock, never re-evaluated behind the consumer's back.

**This overturns a stated non-goal.** PRD.md says "no version solving, no semver ranges"
and SPEC.md says "No ranges, no semver". That was a deliberate decision and this change
reverses it deliberately, on the owner's call. What stays true is the reason behind it:
graft still resolves one source at a time, with no dependency graph, no transitive
resolution, no conflict solving, and no registry. Choosing the newest tag matching a range
is a `max` over one list, not a solver.

## What Changes

- **`rev` may be a semver range** — `^1.2.0`, `~1.2`, `>=1.2.0 <2.0.0`. `graft.toml` already
  accepts the string; what changes is resolution, which today reports
  `rev "^1.2.0" not found` because it looks the value up as a ref.
- **A range is distinguished from a ref name by syntax, not by lookup.** A rev is a range
  **iff** its first character is one of `^ ~ > < =`, or it contains a space, or it contains
  `||`, or it is exactly `*`. Nothing that resolves as a ref today can become a range
  tomorrow, and the test needs no network. Bare `1.x` and `1.2.x` are deliberately *not*
  ranges: they are legal ref names, and a rule with an ambiguous case is a rule that silently
  picks wrong. A tag literally named `>=1.2.0` becomes unpinnable — accepted knowingly, since
  no real tag is spelled that way.
- **A range resolves by listing the source's tags** — `git ls-remote --tags`, keep the
  parseable semver ones, discard prereleases unless the range names one, take the highest
  that satisfies. No match is an error naming the range and the tags considered.
- **`update` re-evaluates a range; `sync` never does.** A range in `graft.toml` with a sha in
  `graft.lock` is not drift, so `sync` installs the lock's sha exactly as it does today. This
  is `npm ci` versus `npm update`, and it is what keeps sync reproducible offline.
- **`graft.lock` records which tag the range matched.** **BREAKING** to `lock-format` in that
  the published format gains a key: `matched`, written only when `rev` is a range and omitted
  otherwise, so every lock without a range is byte-identical to what graft writes today and
  the lock's own `version` stays at `1`. Without it a version bump is a bare sha change, and
  "reviewing a version bump means reading the lock diff" stops being true.
- **The sync report's half-rendering rule is corrected while it is being extended.** Its live
  wording says that when the rev *or* the sha moved, *both halves* render as `<old> -> <new>` —
  which already contradicted its own scenario showing `shared  main  (aaaaaaa -> bbbbbbb)`,
  one rev and two shas. The rule becomes per-half: each of rev, matched, and sha renders twice
  only if it moved. No rendered byte changes for any case the live scenarios pin; the sentence
  starts describing what the scenarios already required.
- **The sync report and `graft list` name the matched tag** when there is one.
  **BREAKING** to `list-execution`: every source object in the `--json` document gains a
  `matched` member, and the document's own `version` moves from `1` to `2` — which is what
  that field exists for.
- **A new dependency**, `github.com/Masterminds/semver/v3` v3.5.0. Range parsing and
  precedence — prerelease ordering, build metadata, `^0.x` — is a specification with corners,
  and a hand-rolled one that is subtly wrong picks the wrong tag while looking like it
  worked. It is parse-and-compare only: no network, no filesystem, no process.

## Non-Goals

- **No dependency resolution.** One source, one range, one list of that source's own tags.
  No transitive sources, no shared constraint, nothing to conflict.
- **No range in `graft add`'s arguments, and no default rev.** `add-command` is the next
  change and decides what it writes; this one only makes a range a legal thing to write.
- **No re-evaluation on `sync`, ever, under any flag.** Not `--refresh`, not `--latest`.
  `update` is the command that moves a pin. The one thing `sync` still does is resolve a
  source the lock has no entry for — a range included, exactly as it already does for a tag —
  because a first resolution is not a re-evaluation and a hand-added source must be
  installable without reaching for `update`.
- **No lockfile-only range check.** No `graft outdated`, no report of a newer tag that
  `update` would pick. That is a command, and it needs proposing as one.
- **No non-semver ordering.** A source whose tags are `release-2024-01` publishes no
  versions this change can order; its consumer pins a tag, exactly as today.
- **No change to what a range means once resolved.** The lock holds a sha, `sync` installs
  it, and every downstream package sees what it sees today.

## Capabilities

**New Capabilities**

- `rev-ranges` — what a range is, how it is told apart from a ref name, the supported
  syntax, how tags are listed and filtered, which tag wins, and what an unsatisfiable range
  reports.

**Modified Capabilities**

- `rev-resolution` — resolution gains a second path taken only for a range, and the existing
  ref path is unchanged for every value that is not one.
- `lock-format` — the `matched` key: when it is written, when it is absent, and where it
  sorts.
- `update-execution` — a range is re-evaluated against the tag list rather than resolved as
  a ref, and the report says which tag it moved to.
- `sync-report` — a source whose rev is a range renders the matched tag beside it.
- `list-execution` — the `--json` document gains `matched`, and the plain form shows it.
- `sync-execution` — a source the lock has no entry for is resolved exactly once, and a range
  is a fourth case that sentence must name. This is the one path on which `sync` lists tags,
  and it is a first resolution rather than a re-evaluation: `sync` resolves what the lock does
  not know and never re-resolves what it does.
- `sync-plan` — the next lock the plan builds carries the matched tag, because `graft.lock`
  requires it for a range and refuses it for a ref, and a plan may never build a lock a later
  `sync` would refuse to read.

Unmodified and deliberately so: `manifest-format` — it never constrained `rev` beyond
non-empty and literally-writable, so a range already parses; the value that rejects one today
is resolution, and widening a rule the manifest never held would be inventing one to relax.
`selector-expansion`, `destination-computation`, `file-application`, `fetch-cache` — a
resolved sha is a resolved sha.

## Impact

- `internal/source` — new: tag listing and range matching. `Resolve` gains the branch that
  routes a range away from the ref lookup.
- `internal/manifest` — nothing. It never judged a rev's syntax, and `SetRev` moves a value
  without interpreting it.
- `internal/lock` — the `matched` field, its serialization slot, and its omission.
- `internal/sync` — passes the matched tag into the report; re-resolution under `update`
  routes through the range path.
- `internal/plan` — carries the matched tag into the next lock. It stays pure: one more string
  travelling through, and no opinion about whether a rev is a range.
- `internal/list` — one field in the document and one in the plain form.
- `go.mod` — one new direct dependency.
- `PRD.md` — the non-goal this change overturns, rewritten to say what is still true rather
  than deleted.
- `SPEC.md` — the `rev` line, the lock example, the `graft update` section, and the failure
  modes a range adds.
- `CLAUDE.md` — the pin rule gains the range case: `sync` still never re-resolves, and what
  `update` re-resolves may now be a range rather than a ref. The separate rewording that
  makes `add` a second pin-mover belongs to `add-command`, not here.
- No sibling repository, no external service, no background job.
