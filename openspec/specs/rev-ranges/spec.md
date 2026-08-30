# rev-ranges Specification

## Purpose
TBD - created by archiving change semver-ranges. Update Purpose after archive.

## Requirements

### Requirement: A rev is a range or a ref, decided by syntax alone

`internal/rev` SHALL classify a `rev` as a **range** or a **ref** from the string alone,
with no network call and no filesystem access, before any resolution is attempted. It SHALL
be the **only** definition of that predicate: `internal/source` asks it before resolving and
`internal/lock` asks it when validating `matched`, and a second copy would let a lock demand a
`matched` for a pin resolution says has none.

A rev SHALL be a range when, and only when, any of the following holds:

- its first character is one of `^`, `~`, `>`, `<`, or `=`
- it contains an ASCII space
- it contains `||`
- it is exactly `*`

Every other rev SHALL be a ref, and SHALL resolve exactly as it does today.

The rule is syntactic rather than a lookup because a lookup would make the meaning of a pin
depend on what the remote happens to contain: a rev that is a ref today and a range tomorrow
is a pin that silently changes what it asks for. Most of what the rule claims is already
unusable as a ref name: `^` and `~` as leading characters, and `*` and the space wherever they
appear, are illegal in a git ref name, so claiming them gives up nothing that could have been
a tag. `>`, `<`, and `=` are legal in a ref name and are given up deliberately: a tag named
`>=1.2.0` becomes unreachable as a pin, which is a cost paid knowingly against a case that
does not occur.

A range SHALL be recorded in `graft.toml` and `graft.lock` verbatim, exactly as any other
rev is. Nothing normalizes, reorders, or rewrites it.

#### Scenario: A caret rev is a range

- **WHEN** a source pins `rev = "^1.2.0"`
- **THEN** it is classified as a range
- **AND** no git command is run to reach that conclusion

#### Scenario: A plain tag is a ref

- **WHEN** a source pins `rev = "v1.2.0"`
- **THEN** it is classified as a ref
- **AND** resolution queries `refs/tags/v1.2.0`, `refs/tags/v1.2.0^{}`, and
  `refs/heads/v1.2.0` exactly as it does today

#### Scenario: A branch name containing a dash is a ref

- **WHEN** a source pins `rev = "release-2024-01"`
- **THEN** it is classified as a ref, because no leading operator, no space, and no `||`
  appears in it

#### Scenario: A compound range with a space is a range

- **WHEN** a source pins `rev = ">=1.2.0 <2.0.0"`
- **THEN** it is classified as a range on both the leading `>` and the space

#### Scenario: An alternation is a range

- **WHEN** a source pins `rev = "1.2.x||1.3.x"`
- **THEN** it is classified as a range on the `||`, even though its first character is a
  digit

#### Scenario: A bare x-range is a ref, not a range

- **WHEN** a source pins `rev = "1.x"`
- **THEN** it is classified as a **ref**, because `1.x` is a legal git ref name and a rule
  with an ambiguous case is a rule that silently picks wrong
- **AND** resolution reports `source "<name>": rev "1.x" not found` when no such ref exists,
  rather than treating it as a range that matched nothing

#### Scenario: A full sha is a ref and never a range

- **WHEN** a source pins a 40-character lowercase hex sha
- **THEN** it is classified as a ref and returned unchanged without contacting the remote,
  exactly as today

### Requirement: A range is parsed with semver constraint syntax and refused when malformed

A rev classified as a range SHALL be parsed as a semver constraint. Parsing SHALL be
deterministic, SHALL contact no remote, and SHALL create, modify, and delete nothing.

A range that does not parse SHALL be an error naming the source and the range, exactly:

```
source "<name>": rev "<range>" is not a valid semver range
```

The classification SHALL NOT be reconsidered on a parse failure. A malformed range SHALL
NOT fall back to a ref lookup, because falling back would send a typo to the network and
report `not found`, naming the wrong problem.

#### Scenario: A caret range parses

- **WHEN** a source pins `rev = "^1.2.0"`
- **THEN** the range parses and no error is returned

#### Scenario: A malformed range is refused without a network call

- **WHEN** a source named `shared` pins `rev = "^^1"`
- **THEN** resolution fails with `source "shared": rev "^^1" is not a valid semver range`
- **AND** no git command is run
- **AND** the returned sha is empty

#### Scenario: A malformed range does not fall back to a ref lookup

- **WHEN** a source named `shared` pins `rev = ">=notaversion"` and the source repository has
  a branch of that exact name
- **THEN** resolution fails with the invalid-range message and does not return the branch's
  commit

### Requirement: A range selects the highest satisfying tag from the source's own tags

Resolving a range SHALL list the source's tags with `git ls-remote --tags` against the
source's clone URL, and SHALL consider only that source's tags. There is no registry, no
second source, and no dependency graph.

Each listed tag name SHALL be parsed as a semver version, accepting an optional leading `v`.
A tag that does not parse SHALL be discarded without error — a source is free to publish
`latest` and `release-2024-01` beside its versions.

Among the tags that parse and satisfy the range, resolution SHALL select the **highest** by
semver precedence. When two tags compare equal under that precedence — `1.3.0` beside
`v1.3.0`, or a build-metadata suffix that precedence ignores — the tie SHALL be broken by
taking the **lower** tag name under byte-wise comparison, so the selection never depends on
the order the remote listed them in. The selected tag SHALL be resolved to its commit sha by the same
precedence the ref path uses: an annotated tag's peeled commit beats the tag object, so
`graft.lock` records a commit and never a tag object.

Resolution SHALL return both the 40-character sha and the **tag name as the remote spells
it** — `v1.3.0`, not `1.3.0` — because that name is what `graft.lock` records as `matched`
and what a human would check out.

Listing tags SHALL create, modify, and delete nothing, anywhere, and SHALL run git with
terminal prompting disabled, exactly as ref resolution does.

#### Scenario: The highest satisfying tag wins

- **WHEN** a source publishes tags `v1.1.0`, `v1.2.0`, `v1.3.0`, and `v2.0.0`, and pins
  `rev = "^1.2.0"`
- **THEN** resolution selects `v1.3.0`
- **AND** returns that tag's commit sha and the tag name `v1.3.0`
- **AND** `v2.0.0` is not selected, because a caret range does not cross a major

#### Scenario: A tag without the v prefix is accepted

- **WHEN** a source publishes tags `1.2.0` and `1.3.0` and pins `rev = "^1.2.0"`
- **THEN** resolution selects `1.3.0` and returns the tag name exactly as the remote spells
  it, with no `v` added

#### Scenario: Unparseable tags are ignored rather than refused

- **WHEN** a source publishes `v1.2.0`, `latest`, `release-2024-01`, and `nightly`, and pins
  `rev = "^1.0.0"`
- **THEN** resolution selects `v1.2.0` and returns no error
- **AND** the three unparseable tags are discarded silently

#### Scenario: An annotated tag resolves to its commit

- **WHEN** the highest satisfying tag `v1.3.0` is an annotated tag whose own object is
  `c834e4f4d61e379612fe4d67ef2f7ea9fdad8f79` and which points at the commit
  `47f73fc0813a4ee9a264f6a9f67dae38466e7dd2`
- **THEN** resolution returns `47f73fc0813a4ee9a264f6a9f67dae38466e7dd2`
- **AND** it does not return the tag object's sha

#### Scenario: A range matching exactly one tag

- **WHEN** a source publishes only `v1.2.0` and pins `rev = "^1.2.0"`
- **THEN** resolution selects `v1.2.0`

#### Scenario: Build metadata does not affect precedence

- **WHEN** a source publishes `v1.3.0` and `v1.3.0+build.1` and pins `rev = "^1.2.0"`
- **THEN** the two compare equal by semver precedence, because build metadata is ignored in
  comparison
- **AND** selection resolves the tie by taking the lower tag name, so `v1.3.0` is selected and
  the result is deterministic rather than dependent on the order the remote listed them

#### Scenario: An exact-version range selects that version

- **WHEN** a source publishes `v1.2.0` and `v1.3.0` and pins `rev = "=1.2.0"`
- **THEN** resolution selects `v1.2.0`, not the higher tag

### Requirement: Prereleases are excluded unless the range asks for one

A tag whose semver version carries a prerelease identifier — `v2.0.0-rc.1` — SHALL NOT be
selected for a range that names no prerelease. A range that names a prerelease SHALL make
prereleases of that same major, minor, and patch eligible.

This is the npm reading and the one that matches what a pin is for: a consumer asking for
`^1.0.0` is asking for a release, and adopting a release candidate because it sorts higher
would be the pin choosing risk on the consumer's behalf.

#### Scenario: A prerelease is not selected by a plain range

- **WHEN** a source publishes `v1.2.0` and `v1.3.0-rc.1`, and pins `rev = "^1.2.0"`
- **THEN** resolution selects `v1.2.0`
- **AND** `v1.3.0-rc.1` is not selected even though it sorts higher

#### Scenario: A range naming a prerelease admits it

- **WHEN** a source publishes `v1.2.0` and `v1.3.0-rc.1`, and pins `rev = ">=1.3.0-rc.0"`
- **THEN** resolution selects `v1.3.0-rc.1`

#### Scenario: Only prereleases exist and the range names none

- **WHEN** a source publishes only `v1.0.0-alpha.1` and pins `rev = "^1.0.0"`
- **THEN** resolution fails with the unsatisfiable-range error, because no release tag
  satisfies the range

### Requirement: A range that no tag satisfies is an error naming the range

Resolution SHALL fail, returning an empty sha and an empty tag name, when no tag satisfies
the range. The message SHALL name the source and the range, and SHALL distinguish a source
that publishes no usable versions at all from one whose versions simply do not match, so the
reader knows whether to fix the range or stop using a range on that source:

- no tag parses as semver:
  `source "<name>": rev "<range>" is a range, and the source publishes no semver tags`
- tags parse but none satisfies:
  `source "<name>": rev "<range>" matches none of the source's semver tags`

A remote that cannot be reached SHALL report the existing unreachable-remote message
unchanged, because a network failure is not a range failure.

#### Scenario: No tag satisfies the range

- **WHEN** a source named `shared` publishes `v1.0.0` and `v1.1.0` and pins `rev = "^2.0.0"`
- **THEN** resolution fails with
  `source "shared": rev "^2.0.0" matches none of the source's semver tags`
- **AND** the returned sha is empty

#### Scenario: The source publishes no semver tags

- **WHEN** a source named `shared` publishes only `latest` and `release-2024-01` and pins
  `rev = "^1.0.0"`
- **THEN** resolution fails with
  `source "shared": rev "^1.0.0" is a range, and the source publishes no semver tags`

#### Scenario: The source publishes no tags at all

- **WHEN** a source named `shared` has no tags whatsoever and pins `rev = "^1.0.0"`
- **THEN** resolution fails with
  `source "shared": rev "^1.0.0" is a range, and the source publishes no semver tags`
- **AND** the empty tag list is not itself an error

#### Scenario: An unreachable remote under a range reports the network failure

- **WHEN** a source named `shared` has a clone URL naming a directory that does not exist and
  pins `rev = "^1.0.0"`
- **THEN** resolution fails with a message beginning
  `source "shared": cannot reach "<url>": ` carrying git's own first output line
- **AND** it does not report an unsatisfiable range, because the tag list was never obtained

### Requirement: Range selection is deterministic and touches nothing

Selecting a tag for a range SHALL depend only on the range and the set of tag names
returned by the remote. Given the same range and the same tag set, it SHALL select the same
tag, in any order the tags arrive, on every run.

Range resolution SHALL create, modify, and delete no file, anywhere — not in the working
tree, not in the fetch cache, and not in the source repository.

#### Scenario: Tag order from the remote does not affect the result

- **WHEN** the same four tags are presented in two different orders for the range `^1.2.0`
- **THEN** the same tag is selected both times

#### Scenario: Two tags naming the same version

- **WHEN** a source publishes both `1.3.0` and `v1.3.0` at different commits and pins
  `rev = "^1.2.0"`
- **THEN** `1.3.0` is selected — the tie is broken by taking the **lower** tag name under
  byte-wise comparison, and `"1.3.0" < "v1.3.0"`
- **AND** the same tag is selected whichever order the remote listed them in, so the result
  never depends on input order

#### Scenario: Resolving a range writes nothing

- **WHEN** a range is resolved against a source repository
- **THEN** the consumer's working tree is byte-identical before and after
- **AND** no cache directory is created
- **AND** the source repository is unmodified
