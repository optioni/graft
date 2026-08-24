## ADDED Requirements

### Requirement: A fetched tree yields its catalog

`internal/source` SHALL read `catalog.yaml` from the root of a fetched tree and return the
parsed catalog. This is SPEC.md's resolution step 4, and it lives here because this package
is the only one that knows where a fetched tree is. It SHALL add no error of its own: a
missing or invalid `catalog.yaml` surfaces `internal/catalog`'s existing message, so the
"not graftable" wording has exactly one owner.

Reading the catalog SHALL create, modify, and delete nothing, and SHALL read no path other
than `catalog.yaml` **resolved inside the entry**. A source commits its own `catalog.yaml`,
so it may commit a symlink under that name; an ordinary read follows one, and a link to an
absolute path outside the entry makes graft parse a file the source never contained. Every
read below an entry SHALL therefore be contained to that entry, and a `catalog.yaml`
resolving outside it SHALL NOT be read.

#### Scenario: A catalog in the fetched tree parses

- **WHEN** a fetched entry holds a `catalog.yaml` declaring `version: 1`, a `schema` kind
  with `to: "openspec/schemas/{name}"`, and one item
  `{ kind: schema, name: tdd, from: extras/tdd }`
- **THEN** reading it returns a catalog at version `1` with one kind and one item whose id
  is `schema:tdd`
- **AND** the entry is unchanged — no file is created, modified, or deleted

#### Scenario: A source with no catalog is not graftable

- **WHEN** a fetched entry holds files but no `catalog.yaml`
- **THEN** reading it fails with `catalog.yaml not found: the source is not graftable`
- **AND** the message is `internal/catalog`'s own, not a second wording of the same failure

#### Scenario: A `catalog.yaml` leaving the entry is not read

- **WHEN** a fetched entry's `catalog.yaml` is a symlink whose target is `/etc/hosts`
- **THEN** reading it fails and returns no catalog
- **AND** the contents of `/etc/hosts` appear nowhere in the result or in the error, so a
  source cannot use graft to read a file the consumer never offered it

### Requirement: An item's `from` is listed as the files it contributes

`internal/source` SHALL turn one item's `from`, resolved inside a fetched tree, into the
`plan.Listing` that `plan.Build` consumes. A `from` naming a **regular file** SHALL produce
a listing that is not a directory holding exactly that file's base name. A `from` naming a
**directory** SHALL produce a directory listing of every regular file at or below it, each
path relative to `from`, slash-separated, and sorted ascending — so a listing is byte-stable
across platforms and across two runs, which is what keeps the lock's `files` from churning.

A path that is neither a regular file nor a directory — a symlink, a socket, a device —
SHALL NOT appear in a listing. graft copies file content; a symlink is not content, and
following one is how a source aims a read at whatever sits beside its own tree.

**Containment is to the fetched entry, and it covers every component of `from`, not only its
last.** Refusing a `from` that is *itself* a symlink is not enough: a source that commits
`extras` as a symlink to `../../..` and declares `from: extras/secrets` reads a directory
outside the entry entirely, while `catalog.yaml`'s own rule — relative, cleaned, no `..`
segment — sees nothing wrong, because nothing is wrong with the *string*. Every path
operation `internal/source` performs below an entry SHALL be resolved within that entry, so
a component that leaves it fails rather than resolves. A symlink that stays inside the entry
is the source's own tree and is not this rule's business; leaving the entry is.

Listing SHALL create, modify, and delete nothing.

#### Scenario: A `from` naming a file lists exactly that file

- **WHEN** item `agent:apply-orchestrator` has `from: extras/agents/apply-orchestrator.md`
  and the fetched tree holds that file
- **THEN** the listing is not a directory and its files are exactly
  `["apply-orchestrator.md"]` — the base name, relative to `from`, as
  `destination-computation` requires for a file item

#### Scenario: A `from` naming a directory lists its tree

- **WHEN** item `schema:tdd` has `from: extras/tdd` and the fetched tree holds
  `extras/tdd/schema.yaml`, `extras/tdd/templates/proposal.md`, and
  `extras/tdd/templates/design.md`
- **THEN** the listing is a directory and its files are
  `["schema.yaml", "templates/design.md", "templates/proposal.md"]` — relative to `from`,
  slash-separated, sorted ascending

#### Scenario: An empty directory lists nothing and is still a directory

- **WHEN** item `schema:empty` has `from: extras/empty`, a directory holding no files
- **THEN** the listing is a directory with zero files
- **AND** no error is returned, because `sync-plan` already declares an item contributing
  no files a legitimate state that still appears in the lock

#### Scenario: A directory holding only empty subdirectories lists nothing

- **WHEN** `from` names a directory whose only content is further empty directories
- **THEN** the listing is a directory with zero files, because a directory is not a file
  graft can copy and an empty one carries nothing to place

#### Scenario: A symlink is not listed

- **WHEN** `from` names a directory holding the regular file `real.md` and a symlink
  `link.md` pointing at `../../../../etc/passwd`
- **THEN** the listing holds `["real.md"]` and does not hold `link.md`
- **AND** no error is returned, so one stray symlink cannot make an otherwise valid source
  unusable

#### Scenario: A `from` that does not exist is an error naming the item

- **WHEN** item `schema:tdd` has `from: extras/gone` and the fetched tree has no such path
- **THEN** listing fails with
  `source "shared": item "schema:tdd": from "extras/gone" not found in the source tree`
- **AND** the returned listing is empty, so no caller can plan a write from it

#### Scenario: A `from` naming a symlink is refused

- **WHEN** item `schema:tdd` has `from: extras/tdd` and that path is a symlink rather than a
  real file or directory
- **THEN** listing fails with
  `source "shared": item "schema:tdd": from "extras/tdd" is not a regular file or directory`
- **AND** the symlink is not followed, so a `from` cannot reach outside the fetched tree
  even though `catalog.yaml` already constrains it to a relative path inside the source

#### Scenario: A `from` reached through a symlinked parent is refused

- **WHEN** item `schema:tdd` has `from: extras/tdd`, the fetched entry holds `extras` as a
  symlink pointing at `../outside`, and `../outside/tdd` is a real directory holding
  `id_rsa`
- **THEN** listing fails and the returned listing is empty
- **AND** `id_rsa` appears in no listing, because the parent component is resolved inside the
  entry and leaving it is a failure rather than a redirection

#### Scenario: A `from` naming a submodule lists nothing

- **WHEN** item `schema:tdd` has `from: extras/tdd` and the source committed that path as a
  submodule, so the fetched entry holds it as an empty directory
- **THEN** the listing is a directory with zero files and no error is returned
- **AND** nothing is cloned and no second remote is contacted, which is why a submodule is
  not a way for a source to reach a repository the consumer never named

#### Scenario: Listing changes nothing

- **WHEN** any listing above is taken
- **THEN** the fetched entry is byte-identical afterwards, and nothing is created in the
  consumer's working tree

### Requirement: A fetched source's listings drive a plan unchanged

The listings this package produces SHALL be usable as `plan.Input.Items` with no adaptation
— same type, same relative-path convention, same file-versus-directory meaning. This closes
`destination-and-plan`'s deferred note that whether a `Listing` faithfully describes a real
fetched tree is `git-fetch`'s contract, tested here against real fixture repositories.

#### Scenario: A fetched fixture plans the writes its tree implies

- **WHEN** a fixture repository holding `catalog.yaml`, `extras/tdd/schema.yaml`,
  `extras/tdd/templates/proposal.md`, and `extras/agents/apply-orchestrator.md` is resolved
  from `rev = "v1.0.0"`, fetched, its catalog read, its selectors `["schema:tdd",
  "agent:*"]` expanded, and each item listed
- **THEN** feeding those values to `plan.Build` against an empty lock yields writes to
  `openspec/schemas/tdd/schema.yaml`, `openspec/schemas/tdd/templates/proposal.md`, and
  `.claude/agents/apply-orchestrator.md`
- **AND** each write's source path names a file that actually exists in the fetched entry,
  which is the property a hand-written `Listing` in a unit test cannot check
