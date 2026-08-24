## ADDED Requirements

### Requirement: A catalog is a single YAML document

`catalog.yaml` SHALL hold exactly one YAML document. A file holding more than one SHALL be
an error, and no catalog SHALL be returned. YAML's document markers are `---` and `...`, and
a decoder reading only the first document silently discards everything a source wrote after
one — an install short by whatever those documents provided, with no guard able to notice: a
`kind:*` selector still matches the items that survived, so the no-match error never fires.

The document count SHALL be taken from the file's tokens, before decoding, and SHALL NOT be
inferred from what the decoder returns. A decoder that stops at the first document cannot
report how many it declined to read, and this decoder stops early on some inputs that do
hold a second document. Counting tokens also decides the count for a file the decoder cannot
parse at all, so a fault after a separator is reported as the extra document it is rather
than as a syntax error in a document graft was never going to read.

A document SHALL be a region of the file delimited by those markers, counted as follows: a
region holding anything other than comments and whitespace is a document, and so is an empty
region that lies between two markers — an empty document is still a document, and content
after one is content the decoder drops. An empty region before the first marker or after the
last SHALL NOT be a document, so a separator opening the file, a separator closing it, and
both together introduce nothing and SHALL be accepted. A `---` inside a block scalar or a
quoted string is not a marker and SHALL NOT be counted.

A YAML directives prologue — a line opening with `%`, such as `%YAML 1.2` or `%TAG` — is an
exception to that reading, because YAML requires a `---` to close one. That marker opens the
one document rather than ending a document before it, so a region holding nothing but
directives SHALL NOT be a document and such a file SHALL be accepted. The exception SHALL
depend on the closing marker: in the last region no marker follows, so a directive there is
not a prologue and SHALL be counted as content after a separator like any other. A `%` that
is not a directive — inside a quoted string, a plain scalar, or a block scalar — SHALL NOT
be treated as one.

#### Scenario: A directives prologue is not a second document

- **WHEN** `catalog.yaml` holds `%YAML 1.2`, then a line `---`, then a valid catalog
- **THEN** the catalog is returned and no error is reported, because the `---` closes the
  prologue and opens the file's single document rather than separating two
- **AND** the same holds for a `%TAG` directive

#### Scenario: A directive that no marker closes is content after a separator

- **WHEN** `catalog.yaml` holds a valid catalog, then a line `---`, then `%YAML 1.2`, with
  or without a mapping after it
- **THEN** the error message is exactly
  `catalog.yaml: multiple YAML documents; a catalog is a single document`
- **AND** it is not the decoder's own complaint about a directive outside a document, which
  would quote the file back naming the directive as the fault when the fault is that the
  region is there at all

#### Scenario: A second document is an error

- **WHEN** `catalog.yaml` holds a valid catalog, then a line `---`, then a second mapping
  declaring another kind and another `provides` entry
- **THEN** the error message is exactly
  `catalog.yaml: multiple YAML documents; a catalog is a single document`
- **AND** no catalog is returned, rather than the first document's catalog being returned
  while the items after the separator are silently dropped

#### Scenario: Content after a separator is reported even when it is malformed

- **WHEN** `catalog.yaml` holds a valid catalog, then a line `---`, then the malformed line
  `kinds: [unclosed`
- **THEN** the error message is exactly
  `catalog.yaml: multiple YAML documents; a catalog is a single document`
- **AND** the decoder's own complaint about the malformed second document is not reported
  instead — it quotes the offending lines back, and the fault is that the content is there
  at all

#### Scenario: An empty document between two separators is still a document

- **WHEN** `catalog.yaml` holds a valid catalog, then `---`, then `---`, then a mapping
  declaring another kind
- **THEN** the error message is exactly
  `catalog.yaml: multiple YAML documents; a catalog is a single document`
- **AND** no catalog is returned, because the trailing mapping is content the decoder drops
  in silence — this is the case a count taken from the decoder cannot see

#### Scenario: A file opening with two separators is more than one document

- **WHEN** `catalog.yaml` holds `---`, then `---`, then `version: 1`
- **THEN** the error message is exactly
  `catalog.yaml: multiple YAML documents; a catalog is a single document`
- **AND** the message is not `catalog.yaml: version is required`, which is what the decoder
  alone reports for a file that plainly declares a version

#### Scenario: A separator inside a scalar is not a separator

- **WHEN** `catalog.yaml` holds one document in which a `to` value is the quoted string
  `"---"`, or a block scalar whose body contains a line `---`
- **THEN** loading succeeds and returns that catalog
- **AND** no multiple-documents error is reported, because the count is over document
  markers rather than over lines that look like one

#### Scenario: A trailing separator with nothing after it is accepted

- **WHEN** `catalog.yaml` holds a valid catalog followed by a line `---` and nothing else,
  or followed by `---` and then only a comment
- **THEN** loading succeeds and returns the catalog the first document declared
- **AND** no error is returned, because nothing was discarded

#### Scenario: A leading separator is accepted, with or without a trailing one

- **WHEN** `catalog.yaml` opens with a line `---` and holds one document after it, whether
  or not it also ends with a line `---`
- **THEN** loading succeeds and returns that document's catalog
- **AND** the pair is not counted as two documents, because neither marker has a document
  on the far side of it

## MODIFIED Requirements

### Requirement: Catalog loading and absence

`internal/catalog` SHALL load `catalog.yaml` from a path and return a parsed catalog, or an
error. A `catalog.yaml` that does not exist SHALL be an error saying the source is not
graftable — graft never falls back to guessing a source's layout. Loading SHALL NOT create,
modify, or delete any file, and SHALL read no path other than the one it was given.

A path that exists but is not a regular file — a symlink, a directory, a device — SHALL be
an error naming that, checked without following the path. `os.ReadFile` on a symlink would
otherwise read whatever it points at, which is a path other than the one loading was given,
and a decoder error quotes the offending lines back verbatim: a source repository could
commit `catalog.yaml` as a link to a file in the consumer's own tree and have its contents
read out in an error message. The error SHALL NOT be the not-graftable one, which claims
something about the source repository that a failed read does not establish.

#### Scenario: A valid catalog loads

- **WHEN** the tree contains only `catalog.yaml` holding:
  ```yaml
  version: 1
  kinds:
    schema:
      to: "openspec/schemas/{name}"
    agent:
      to: ".claude/agents/"
      flatten: true
  provides:
    - { kind: schema, name: tdd, from: extras/openspec-schemas/tdd }
    - { kind: agent, name: apply-orchestrator, from: extras/agents/apply-orchestrator.md }
  ```
- **THEN** loading returns a catalog at version `1` with two kinds and two items
- **AND** the tree is unchanged — no file is created, modified, or deleted

#### Scenario: A catalog with zero provides loads

- **WHEN** `catalog.yaml` holds `version: 1` and a `kinds` map but no `provides` key
- **THEN** loading succeeds and returns a catalog with zero items
- **AND** no error is returned, because a source that offers nothing yet is a legitimate
  state; the failure belongs to any selector aimed at it

#### Scenario: A catalog with neither kinds nor provides loads

- **WHEN** `catalog.yaml` holds only `version: 1`
- **THEN** loading succeeds and returns a catalog with zero kinds and zero items

#### Scenario: A missing catalog is the not-graftable error

- **WHEN** loading is asked for `catalog.yaml` in a directory that has no such file
- **THEN** the error message is exactly `catalog.yaml not found: the source is not graftable`
- **AND** the tree is unchanged — no `catalog.yaml` is created

#### Scenario: A symlinked catalog is refused without being read

- **WHEN** `catalog.yaml` is a symlink to another readable file, whatever that file holds
- **THEN** the error message is exactly `catalog.yaml: not a regular file`
- **AND** no content from the link's target appears in the error, so a file outside the
  path loading was given cannot be quoted back through a decoder message
- **AND** no catalog is returned

#### Scenario: A dangling symlink is refused as a link, not as an absence

- **WHEN** `catalog.yaml` is a symlink whose target does not exist
- **THEN** the error message is exactly `catalog.yaml: not a regular file`
- **AND** it is not the not-graftable error, because the source did publish a
  `catalog.yaml` — it published one graft will not read

#### Scenario: A directory named catalog.yaml is refused

- **WHEN** `catalog.yaml` exists and is a directory
- **THEN** the error message is exactly `catalog.yaml: not a regular file`
- **AND** it is not the not-graftable error: the source may well be graftable, and what
  went wrong is the read

#### Scenario: Malformed YAML is an error

- **WHEN** `catalog.yaml` holds `version: 1` followed by the line `kinds: [unclosed`
- **THEN** an error is returned whose message begins with `catalog.yaml: `
- **AND** no catalog is returned, so no caller can act on a half-decoded file

#### Scenario: A catalog that is not a mapping is an error

- **WHEN** `catalog.yaml` holds a valid YAML sequence rather than a mapping, such as
  `- schema:tdd`
- **THEN** the error message is exactly `catalog.yaml: top level must be a mapping`
- **AND** no catalog is returned

#### Scenario: An empty catalog file is a missing-version error

- **WHEN** `catalog.yaml` exists and is zero bytes
- **THEN** the error message is exactly `catalog.yaml: version is required`
- **AND** the file's emptiness is treated as an empty mapping rather than as a syntax
  error, so the message names what is actually missing

### Requirement: Catalog format version gating

The catalog SHALL carry `version`, and this graft SHALL accept only version `1`. A version
this binary does not know SHALL fail and say to upgrade graft, and SHALL be checked before
any other validation so a future format's new keys are reported as "upgrade" rather than as
unknown keys.

An integer literal too wide for the decoder's integer types SHALL be reported as a version
this graft does not support, not as a non-integer. Such a literal is handed back by the
decoder as a `string`, exactly as a quoted `version: "1"` is, so the two SHALL be told
apart by the literal's *shape* rather than by its Go type: a decimal literal — an optional
`+` or `-`, then decimal digits with optional `_` separators — whose value **overflows**
every 64-bit integer type is a version; anything else arriving as a string is not an integer
at all. Separators SHALL be removed before the range test, and only a range failure SHALL
count: a value that merely fails to parse is not a wide literal. A quoted `version: "1"`
SHALL therefore keep reporting the non-integer message, because `1` is a value the decoder
could hold and so was quoted deliberately. The offending literal SHALL be printed as
written, never clamped or reformatted.

The rule is over shape alone, so a *quoted* literal too wide to hold is reported as a
version too. Both spellings are refused and both name the literal; separating them would
need the source token's quoting rather than the decoded value.

#### Scenario: A missing version is an error

- **WHEN** `catalog.yaml` declares `kinds` and `provides` but no `version` key
- **THEN** the error message is exactly `catalog.yaml: version is required`
- **AND** no catalog is returned

#### Scenario: A newer version fails and says to upgrade

- **WHEN** `catalog.yaml` holds `version: 2` together with a top-level key this graft does
  not know, such as `requires: []`
- **THEN** the error message is exactly
  `catalog.yaml: version 2 is not supported by this graft; upgrade graft`
- **AND** the unknown key is not reported instead, because version is checked first
- **AND** no catalog is returned

#### Scenario: A version below 1 is an error

- **WHEN** `catalog.yaml` holds `version: 0`
- **THEN** the error message is exactly `catalog.yaml: version 0 is not a known catalog version`
- **AND** no catalog is returned

#### Scenario: A version literal wider than any integer type says to upgrade

- **WHEN** `catalog.yaml` holds `version: 99999999999999999999999999`, a literal past what
  `uint64` can hold
- **THEN** the error message is exactly
  `catalog.yaml: version 99999999999999999999999999 is not supported by this graft; upgrade graft`
- **AND** it is not the non-integer message, which would send the reader looking for a
  malformed file rather than for a newer graft
- **AND** no catalog is returned

#### Scenario: A sign or separators do not change the answer

- **WHEN** `catalog.yaml` holds `version: +99999999999999999999999999`,
  `version: 99_999_999_999_999_999_999_999_999` with digit separators, or
  `version: "99999999999999999999999999"` quoted
- **THEN** the error message is exactly
  `catalog.yaml: version <literal> is not supported by this graft; upgrade graft` with
  `<literal>` the value as written, sign included
- **AND** the quoted spelling gets the same answer as the bare one, because the rule reads
  the literal's shape and a value this wide is a version whichever way it was written

#### Scenario: A hugely negative version literal is not a known version

- **WHEN** `catalog.yaml` holds `version: -99999999999999999999999999`
- **THEN** the error message is exactly
  `catalog.yaml: version -99999999999999999999999999 is not a known catalog version`
- **AND** no catalog is returned, because a version below `1` is not a future format however
  it is written

#### Scenario: A quoted version is not an integer

- **WHEN** `catalog.yaml` holds `version: "1"`
- **THEN** the error message is exactly `catalog.yaml: version must be an integer`
- **AND** the shape rule does not rescue it: `1` is a value the decoder can hold, so a
  string carrying it was quoted deliberately and is not an integer

#### Scenario: A version that is neither an integer nor an integer literal is an error

- **WHEN** `catalog.yaml` holds `version: 1.5`, or `version: true`, or
  `version: "99999999999999999999999999x"`, or the quoted `version: "-1"`
- **THEN** the error message is exactly `catalog.yaml: version must be an integer`
- **AND** no catalog is returned, and in the last case the shape rule declines it because
  `-1` is a value the decoder could hold — a quoted value that fits is a string, not a
  version

### Requirement: Kind declarations

`kinds.<kind>` SHALL declare where a class of thing belongs. `to` SHALL be either a
non-empty string or a non-empty list of non-empty strings, and SHALL be carried verbatim —
graft SHALL NOT interpolate `{name}`, resolve a trailing `/`, or clean the path at parse
time, because computing a destination belongs to a later change. `flatten` SHALL be a
boolean defaulting to `false`.

Two entries of one `to` list SHALL be a duplicate when they name the same destination, not
only when they are spelled alike. Two entries name the same destination when their cleaned
forms agree **and** they agree on whether they end in `/`: `.claude/agents/` and
`.claude/agents//` are one destination, and so are `a/b` and `./a/b`, while `a/{name}` and
`a/{name}/` are not — a trailing slash is a no-op for a directory item and significant for
a file item, and `destination-computation` already requires that pair to be one destination
for the first and two for the second. Comparing the raw strings lets a kind declare one
destination twice and be refused later, by a message about escaping the repo root, for a
catalog whose actual fault is a duplicate. The comparison SHALL be the only thing that
cleans: `Kind.To` still carries every entry exactly as written.

The rule is deliberately not a general spelling-equivalence: `a/b/.` and `a/b/` are not
collapsed, because cleaning removes the `.` and leaves no trailing slash to compare. Nothing
rests on it — `destination-computation` refuses an uncleaned destination outright — and a
rule that tried to cover every spelling would be reimplementing that one.

#### Scenario: A string-valued to is carried verbatim

- **WHEN** a catalog declares kind `schema` with `to: "openspec/schemas/{name}"`
- **THEN** the parsed kind carries exactly one destination, the string
  `openspec/schemas/{name}`, with `{name}` uninterpolated
- **AND** its `flatten` is `false`, because the key was absent

#### Scenario: A trailing slash is preserved

- **WHEN** a catalog declares kind `agent` with `to: ".claude/agents/"` and `flatten: true`
- **THEN** the parsed kind carries the destination `.claude/agents/` with its trailing slash
  intact, because the slash is what later means "into this directory"
- **AND** its `flatten` is `true`

#### Scenario: An uncleaned destination is carried verbatim

- **WHEN** a catalog declares kind `agent` with `to: "./.claude//agents/"`
- **THEN** the parsed kind carries the destination `./.claude//agents/` exactly as written
- **AND** nothing is cleaned at parse time, because the duplicate comparison is the only
  place a cleaned form is used
- **AND** the catalog parses even though `destination-computation` will later refuse this
  destination for requiring cleaned form — the two rules are separate, and this one decides
  only whether two entries are the same

#### Scenario: A list-valued to is carried in declared order

- **WHEN** a catalog declares kind `agent` with
  `to: [".claude/agents/", ".codex/agents/"]`
- **THEN** the parsed kind carries both destinations in that order

#### Scenario: An empty kind name is an error

- **WHEN** a catalog declares a kind whose key is the empty string (`"": { to: "x/" }`)
- **THEN** the error message is exactly `catalog.yaml: kind name is empty`
- **AND** no catalog is returned

#### Scenario: A missing or empty to is an error

- **WHEN** a catalog declares kind `agent` with no `to` key, or with `to: ""`, or with
  `to: []`
- **THEN** the error message is exactly `catalog.yaml: kind "agent": to is required`
- **AND** no catalog is returned

#### Scenario: An empty destination inside a list is an error

- **WHEN** a catalog declares kind `agent` with `to: [".claude/agents/", ""]`
- **THEN** the error message is exactly
  `catalog.yaml: kind "agent": to contains an empty destination`
- **AND** no catalog is returned

#### Scenario: A to of the wrong type is an error

- **WHEN** a catalog declares kind `agent` with `to: { dir: ".claude/agents/" }`, or with
  `to: 7`
- **THEN** the error message is exactly
  `catalog.yaml: kind "agent": to must be a string or a list of strings`
- **AND** no catalog is returned

#### Scenario: A repeated destination within one kind is an error

- **WHEN** a catalog declares kind `agent` with
  `to: [".claude/agents/", ".claude/agents/"]`
- **THEN** the error message is exactly
  `catalog.yaml: kind "agent": duplicate destination ".claude/agents/"`
- **AND** no catalog is returned, because the kind would otherwise write every one of its
  items to the same path twice

#### Scenario: Two spellings of one destination are an error naming both

- **WHEN** a catalog declares kind `agent` with
  `to: [".claude/agents/", ".claude/agents//"]`
- **THEN** the error message is exactly
  `catalog.yaml: kind "agent": duplicate destination ".claude/agents//": same path as ".claude/agents/"`
- **AND** both spellings are named, because a message repeating one string would send the
  reader looking for two identical entries that are not there
- **AND** no catalog is returned

#### Scenario: A dot-slash prefix is the same destination

- **WHEN** a catalog declares kind `agent` with `to: ["a/b", "./a/b"]`
- **THEN** the error message is exactly
  `catalog.yaml: kind "agent": duplicate destination "./a/b": same path as "a/b"`
- **AND** no catalog is returned

#### Scenario: A trailing slash makes two destinations, not one

- **WHEN** a catalog declares kind `doc` with `to: ["docs/{name}", "docs/{name}/"]`
- **THEN** the kind parses and carries both destinations in declared order
- **AND** no duplicate is reported, because for an item whose `from` names a file these are
  two different destinations — one names the file, the other a directory to put it in — and
  `destination-computation` owns the per-item rule that decides between them
