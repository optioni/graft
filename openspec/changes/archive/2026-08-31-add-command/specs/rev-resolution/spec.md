## ADDED Requirements

### Requirement: A source's default rev is its highest semver tag, falling back to its default branch

`internal/source` SHALL resolve a source's **default rev** — the pin `graft add` writes when
the invocation names none — from the source's own published refs, in one `git ls-remote`
call that asks for the symbolic ref of `HEAD` alongside the tags.

The default rev SHALL be the highest tag that parses as semver and is not a prerelease,
spelled exactly as the remote spells it (`v1.3.0`, not `1.3.0`), selected by the same rule
`rev-ranges` uses for the range `*` — so the two can never disagree about which tag is
latest. A tag that does not parse as semver SHALL be ignored rather than compete.

When the source publishes no tag that parses as semver, the default rev SHALL be the short
name of the branch `HEAD` points at — `main` for `ref: refs/heads/main`. The result is a ref
either way, never a range and never a sha: the sha belongs in `graft.lock`, and a range is a
policy only the consumer can choose.

When the source publishes neither a semver tag nor a symbolic `HEAD`, resolution SHALL fail
with `source "<name>": has no semver tag and no default branch`. A source that cannot be
reached SHALL fail with this package's existing wording,
`source "<name>": cannot reach "<url>": <detail>`, distinguished from the row above so a
network problem is never reported as an empty repository.

The lookup SHALL go through the same `CloneURL` leading-dash refusal, the same `--` before
the URL, and the same prompt-disabling environment every other network call in this package
goes through. It SHALL create, modify, and delete nothing, anywhere.

#### Scenario: The highest stable tag wins

- **WHEN** a source publishes `v1.2.0`, `v1.3.0`, `v2.0.0-rc.1`, and `nightly`, and its
  default rev is resolved
- **THEN** the result is `v1.3.0` — the prerelease excluded, the non-semver tag ignored

#### Scenario: A source with no semver tags falls back to its branch

- **WHEN** a source publishes only the tag `nightly`, and `HEAD` is `refs/heads/trunk`
- **THEN** the result is `trunk`

#### Scenario: An empty repository is an error, not an empty rev

- **WHEN** a source publishes no tags and no `HEAD`
- **THEN** the failure is `source "shared": has no semver tag and no default branch`
- **AND** no rev is returned, so no caller can write an empty pin into `graft.toml`

#### Scenario: An unreachable source is not reported as an empty one

- **WHEN** the remote cannot be read at all
- **THEN** the failure is `source "shared": cannot reach "<url>": <detail>`

#### Scenario: A git value beginning with a dash is refused before the call

- **WHEN** the default rev of a source whose `git` is `--upload-pack=touch /tmp/x` is resolved
- **THEN** the failure is `CloneURL`'s existing refusal and no `git` process is run
