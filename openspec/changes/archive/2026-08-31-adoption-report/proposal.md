## Why

`graft add` on a directory that already holds files overwrites them and reports `added`. The
verb asserts the opposite of what happened: nothing was added, something was replaced. Found
in the field — five repositories vendoring the same source were behind by the same commit,
and a sixth carried a local edit made directly in a synced file that a first sync would
silently replace.

`git diff` remains the recovery path for anything committed, and that is not in question.
What is missing is any word from graft saying there was something to recover.

## What Changes

- **BREAKING** (a command's observable output): a report line whose item replaced content the
  lock did not claim carries the note `replaced existing content`, and its verb is `adopted`
  rather than `added`.
- The summary counts them: `7 files written (1 replaced existing content), 0 removed`. The
  parenthetical appears only when the count is non-zero.
- An `updated` item that replaced unclaimed content keeps the verb `updated` and gains the
  same note — the verb is only corrected where it would otherwise be actively false.
- Adoption is a filesystem fact, so `--dry-run`, which reaches no filesystem write, reports
  none. Stated rather than left to be discovered.
- `internal/plan` gains, per planned write, whether the previous lock claimed that
  destination. It stays pure: that is a set operation on the lock it is already given.
- `internal/apply` reports which destinations it replaced. It is the only package that may
  look, and looking is one comparison against bytes it is already about to write.

## Non-Goals

- **A prompt, a confirmation, or a `--force`.** `add` works without a terminal, and SPEC.md
  has already refused a force flag: sync always overwrites, and a flag to make it do its job
  is the bug it prevents.
- **Showing destinations before writing under the non-interactive `add`.** SPEC.md's sentence
  claiming that is corrected to describe `--list` and the picker, which do.
- **Backing anything up.** git is the backup, and a second one graft managed would be a
  second thing to trust.
- **Refusing to overwrite.** Vendored files are derived artifacts with `node_modules`
  semantics. Adoption is the normal way a repository starts using graft.
- **Content comparison anywhere else.** No `unchanged` verb, no hashes in the lock.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `sync-report`: the verb, the note, and the summary count.
- `file-application`: applying a plan reports which destinations it replaced.
- `sync-plan`: each planned write records whether the previous lock claimed its destination.

## Impact

- Changed: `internal/plan` (one field per write), `internal/apply` (`Run` returns what it
  replaced), `internal/sync` (report and renderer), SPEC.md's Output section and its
  destination sentence.
- No format change: `graft.lock` and the `graft list --json` document are untouched, and both
  version numbers stay where they are.
