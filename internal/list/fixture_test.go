package list_test

import (
	"github.com/optioni/graft/internal/lock"
)

// specLock is SPEC.md's own graft.lock example as a value: one source, two items, the
// second holding six files. It is the fixture rather than a paraphrase of it, because both
// renderings are contracts and the only way to keep a contract is to hold the bytes.
func specLock() *lock.Lock {
	return &lock.Lock{
		Version: lock.Version,
		Sources: []lock.Source{{
			Name:     "openspec-schemas",
			Git:      "github.com/optioni/openspec-schemas",
			Rev:      "v1.2.0",
			Resolved: "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5",
			Items: []lock.Item{
				{
					ID:    "agent:apply-orchestrator",
					Files: []string{".claude/agents/apply-orchestrator.md"},
				},
				{
					ID: "schema:tdd",
					Files: []string{
						"openspec/schemas/tdd/schema.yaml",
						"openspec/schemas/tdd/templates/design.md",
						"openspec/schemas/tdd/templates/planning-review.md",
						"openspec/schemas/tdd/templates/proposal.md",
						"openspec/schemas/tdd/templates/spec.md",
						"openspec/schemas/tdd/templates/tasks.md",
					},
				},
			},
		}},
	}
}

// specDocument is what specLock lists as under --json, character for character. It is
// written out rather than assembled, so a change to the encoder cannot quietly change the
// contract and the expectation together.
const specDocument = `{
  "version": 2,
  "sources": [
    {
      "name": "openspec-schemas",
      "git": "github.com/optioni/openspec-schemas",
      "rev": "v1.2.0",
      "matched": "",
      "resolved": "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5",
      "items": [
        {
          "id": "agent:apply-orchestrator",
          "kind": "agent",
          "name": "apply-orchestrator",
          "files": [
            ".claude/agents/apply-orchestrator.md"
          ]
        },
        {
          "id": "schema:tdd",
          "kind": "schema",
          "name": "tdd",
          "files": [
            "openspec/schemas/tdd/schema.yaml",
            "openspec/schemas/tdd/templates/design.md",
            "openspec/schemas/tdd/templates/planning-review.md",
            "openspec/schemas/tdd/templates/proposal.md",
            "openspec/schemas/tdd/templates/spec.md",
            "openspec/schemas/tdd/templates/tasks.md"
          ]
        }
      ]
    }
  ]
}
`

// emptyDocument is what a repository with nothing installed lists as. A machine-readable
// form that printed nothing would make "nothing is installed" indistinguishable from "the
// command did not run".
const emptyDocument = `{
  "version": 2,
  "sources": []
}
`
