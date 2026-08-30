# Rev Resolution Specification

## Purpose

A source's `graft.toml` names a repository and a `rev` — a tag, a branch, or a full sha.
`graft.lock` records the 40-character lowercase hex commit sha that `rev` became. This
capability covers the journey between them: expanding a `git` value into a clone URL, and
turning a `rev` into the commit it names, including every way that fails.

Resolution reads a remote and nothing else. It creates, modifies, and deletes nothing,
anywhere, and it is the first place graft touches a repository the consumer does not
control — so it is also where the rule that a `git` value may never become a git *option*
is enforced.

## Requirements

### Requirement: A source's git value expands to a clone URL

`internal/source` SHALL expand a source's `git` value, stored by `graft.toml` exactly as
written, into something `git` accepts. A value of the shorthand form `host/owner/repo` —
no scheme, no `user@host:` prefix, not a filesystem path, and whose first segment looks
like a hostname — SHALL become `https://host/owner/repo`. Every other form SHALL be passed
through unchanged, because SPEC.md says `git` is "anything `git clone` accepts" and graft
has no business rewriting a URL a user chose deliberately.

Expansion SHALL be a pure function of the string: it contacts nothing, reads nothing, and
creates nothing.

A `git` value that begins with `-` SHALL NOT be expanded and SHALL NOT be handed to `git`.
`git ls-remote <url> <refs...>` parses a leading-dash operand as an **option**, so a value
of `--upload-pack=./script` makes git run that script and promotes the first refspec to the
repository operand — arbitrary code execution chosen by whoever wrote `graft.toml`. Every
invocation SHALL additionally separate options from operands with `--`, so the refusal and
git's own `strange pathname` guard both stand and neither is the only thing holding.

#### Scenario: Shorthand expands to HTTPS

- **WHEN** a source's `git` is `github.com/optioni/openspec-schemas`
- **THEN** the clone URL is `https://github.com/optioni/openspec-schemas`
- **AND** nothing is fetched and no file is created

#### Scenario: A URL carrying a scheme is passed through

- **WHEN** a source's `git` is `https://example.com/team/assets.git`
- **THEN** the clone URL is `https://example.com/team/assets.git`, unchanged

#### Scenario: An scp-style address is passed through

- **WHEN** a source's `git` is `git@github.com:optioni/openspec-schemas.git`
- **THEN** the clone URL is `git@github.com:optioni/openspec-schemas.git`, unchanged

#### Scenario: A filesystem path is passed through

- **WHEN** a source's `git` is `/srv/mirrors/openspec-schemas`
- **THEN** the clone URL is `/srv/mirrors/openspec-schemas`, unchanged
- **AND** a `git` of `../sibling-repo` is likewise unchanged, so a local fixture or a
  neighbouring checkout can be a source

### Requirement: A rev resolves to the commit sha it names

`internal/source` SHALL resolve a source's `rev` to the 40-character lowercase hex commit
sha that `graft.lock` records as `resolved`.

A `rev` SHALL first be classified as a **ref** or a **range** by syntax alone, as
`rev-ranges` defines. A range SHALL resolve by listing the source's tags and selecting the
highest that satisfies it; everything below governs a ref, and is unchanged.

A ref — a tag, a branch, or a full sha — SHALL resolve using `git ls-remote` against the
source's clone URL, querying `refs/tags/<rev>`, `refs/tags/<rev>^{}`, and `refs/heads/<rev>`
explicitly rather than by pattern, so a rev can never match a ref whose name merely ends
with it.

When several of those refs exist, the peeled tag SHALL win over the tag object, and a tag
SHALL win over a branch of the same name — git's own `rev-parse` precedence, and the
reading under which a pin means the immutable thing.

A `rev` that is already 40 lowercase hex characters SHALL be returned unchanged **without
contacting the remote**, because there is nothing to look up and a network round trip for a
value already final would break offline resolution for no gain.

Resolution SHALL report, alongside the sha, the tag a range matched, and SHALL report an
empty tag name for every ref. A ref names itself; only a range needs recording separately.

Resolving SHALL create, modify, and delete nothing, anywhere.

#### Scenario: A branch resolves to its tip

- **WHEN** a source repository has `refs/heads/main` at
  `47f73fc0813a4ee9a264f6a9f67dae38466e7dd2` and a source pins `rev = "main"`
- **THEN** resolution returns `47f73fc0813a4ee9a264f6a9f67dae38466e7dd2`
- **AND** the reported matched tag is empty
- **AND** neither the source repository nor the consumer's working tree is modified

#### Scenario: A lightweight tag resolves to its commit

- **WHEN** the source repository has a lightweight tag `v1.0.1` pointing directly at the
  commit `47f73fc0813a4ee9a264f6a9f67dae38466e7dd2` and a source pins `rev = "v1.0.1"`
- **THEN** resolution returns `47f73fc0813a4ee9a264f6a9f67dae38466e7dd2`
- **AND** the reported matched tag is empty, because `v1.0.1` is a ref and names itself

#### Scenario: An annotated tag resolves to the commit, not the tag object

- **WHEN** the source repository has an annotated tag `v1.0.0` whose own object is
  `c834e4f4d61e379612fe4d67ef2f7ea9fdad8f79` and which points at the commit
  `47f73fc0813a4ee9a264f6a9f67dae38466e7dd2`, and a source pins `rev = "v1.0.0"`
- **THEN** resolution returns the commit `47f73fc0813a4ee9a264f6a9f67dae38466e7dd2`
- **AND** it does not return the tag object's sha, which is not a commit and would put a
  non-commit into `graft.lock`'s `resolved`

#### Scenario: A tag wins over a branch of the same name

- **WHEN** the source repository has both `refs/tags/release` and `refs/heads/release`, at
  different commits, and a source pins `rev = "release"`
- **THEN** resolution returns the commit the tag names

#### Scenario: A full sha passes through without contacting the remote

- **WHEN** a source pins `rev = "47f73fc0813a4ee9a264f6a9f67dae38466e7dd2"` and the clone
  URL names a path that does not exist
- **THEN** resolution returns `47f73fc0813a4ee9a264f6a9f67dae38466e7dd2` and succeeds
- **AND** no git command is run, which is why an unreachable remote cannot fail it

#### Scenario: An uppercase sha is not treated as a sha

- **WHEN** a source pins `rev` to the same 40 characters in uppercase
- **THEN** resolution treats it as a ref name and looks it up, because `graft.lock` records
  lowercase hex and silently lowercasing a rev would make the manifest and the lock disagree
  about what was asked for

#### Scenario: A range resolves through the range path and reports its tag

- **WHEN** a source publishes `v1.2.0` and `v1.3.0` and pins `rev = "^1.2.0"`
- **THEN** resolution returns `v1.3.0`'s commit sha and reports the matched tag `v1.3.0`
- **AND** it queries no `refs/tags/^1.2.0`, because the value was never classified as a ref

### Requirement: An unresolvable rev is an error naming the rev and the source

Resolution SHALL fail, returning an empty sha, when a **ref** matches no queried ref, when
the remote cannot be reached, when `rev` is empty, or when `git` is not on `PATH`. A
**range** that cannot be satisfied SHALL fail with the messages `rev-ranges` specifies; the
unreachable-remote, empty-rev, absent-git, and option-shaped-remote failures below apply to
both paths alike.

Every message SHALL name the source, so a manifest with several sources always says which
one failed, and the messages SHALL be exactly:

- no matching ref: `source "<name>": rev "<rev>" not found`
- unreachable remote: `source "<name>": cannot reach "<url>": <first line of git's output>`
- empty rev: `source "<name>": rev is empty`
- git absent: `git not found on PATH`
- option-shaped remote: `source "<name>": git "<value>" may not begin with "-"`

Resolution SHALL never prompt for credentials: it SHALL run git with terminal prompting
disabled, so a private source without usable credentials fails with the unreachable-remote
error instead of hanging forever on a password prompt.

#### Scenario: A rev no ref matches

- **WHEN** a source named `shared` pins `rev = "v9.9.9"` and the source repository has no
  such tag or branch
- **THEN** resolution fails with `source "shared": rev "v9.9.9" not found`
- **AND** the returned sha is empty

#### Scenario: An abbreviated sha is not a rev

- **WHEN** a source named `shared` pins `rev = "47f73fc"`, the first seven characters of a
  commit that exists in the source repository
- **THEN** resolution fails with `source "shared": rev "47f73fc" not found`, because
  SPEC.md admits a tag, a branch, a full sha, or a range and nothing else, and an
  abbreviation is not stable enough to pin

#### Scenario: An unreachable remote

- **WHEN** a source named `shared` has a clone URL naming a directory that does not exist
  and pins `rev = "main"`
- **THEN** resolution fails with a message beginning
  `source "shared": cannot reach "<url>": ` and carrying git's own first output line
- **AND** the failure is distinguishable from a missing rev, because one is a typo in
  `graft.toml` and the other is a network or permission problem

#### Scenario: An empty rev

- **WHEN** resolution is asked for an empty `rev` on a source named `shared`
- **THEN** it fails with `source "shared": rev is empty` and runs no git command
- **AND** the empty string is classified as neither a range nor a ref, because the
  empty-rev check precedes classification

#### Scenario: A remote that looks like an option is refused

- **WHEN** a source named `shared` has `git = "--upload-pack=./pwn.sh"` and any `rev`
- **THEN** resolution fails with
  `source "shared": git "--upload-pack=./pwn.sh" may not begin with "-"`
- **AND** no git command is run at all, so nothing can be executed even once
- **AND** the same refusal applies to fetching, because both invocations take the same value

#### Scenario: git is not on PATH

- **WHEN** `PATH` contains no `git` executable and resolution is attempted
- **THEN** it fails with `git not found on PATH`
- **AND** the message names the one runtime dependency SPEC.md declares, rather than
  surfacing an exec error a user cannot act on

#### Scenario: An option-shaped remote is refused under a range too

- **WHEN** a source named `shared` has `git = "--upload-pack=./pwn.sh"` and pins
  `rev = "^1.0.0"`
- **THEN** resolution fails with the same option-shaped-remote message
- **AND** no `git ls-remote --tags` is run, so the range path cannot become a second way to
  execute a program
