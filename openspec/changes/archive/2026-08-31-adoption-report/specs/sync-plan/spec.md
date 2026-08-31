## ADDED Requirements

### Requirement: Each planned write records whether the lock already claimed its destination

`internal/plan` SHALL record, for every planned write, whether the lock it was given already
claims that destination path for any source and item.

It is a set operation over the lock this package is already handed, so it costs no filesystem
access and this package stays pure. The question belongs here rather than in the applier
because the previous lock is a planning input: an applier asking it would need the previous
lock passed to it for no other reason, and would then hold two records of what graft owns.

A path the lock claims is a file graft wrote and is rewriting. A path it does not claim is
either absent or somebody else's, and only the filesystem can say which — that is
`internal/apply`'s question, and this flag is what tells it which paths are worth asking about.

#### Scenario: A path the lock claims is marked claimed

- **WHEN** the lock records `agent:reviewer` at `.claude/agents/reviewer.md` and the new plan
  writes that same path
- **THEN** that write is marked as claimed by the lock

#### Scenario: A path claimed under a different source or item still counts as claimed

- **WHEN** the lock claims `openspec/schemas/tdd/schema.yaml` for source `a`, and the new plan
  writes that path for source `b`
- **THEN** the write is marked claimed: the question is whether graft owned the path, not
  which item owned it

#### Scenario: A path no lock claims is not marked

- **WHEN** the plan writes `.claude/agents/reviewer.md` and the lock records no such path
- **THEN** the write is not marked claimed, whether or not anything exists there — this
  package does not look

#### Scenario: An empty lock claims nothing

- **WHEN** the plan is built against a lock with no sources
- **THEN** no write is marked claimed
