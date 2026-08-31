<!-- Every group needs a kind marker: behavior | refactor | operational.
     Behavior: RED → GREEN → REFACTOR. Refactor: CHARACTERIZE → REFACTOR → VERIFY.
     Operational: CHECK → CHANGE → VERIFY — this is where plumbing goes (manifests,
     lockfiles, CI and build config, formatter settings). Do NOT write a test that
     asserts a config key; the command that consumes the config is the check. -->

## 0. Acceptance Test — Outer Loop RED
<!-- kind: behavior -->
<!-- OPT-IN. Keep this group only when the change alters a client-visible interface whose
     end-to-end wiring is the real risk; DELETE it otherwise and say so in design.md →
     Test Strategy. See the project context for what an acceptance test costs here. -->

- [ ] 0.1 Set up the harness and the replaced collaborators named in design.md → Test Boundaries
- [ ] 0.2 RED: Write the failing end-to-end test for <!-- headline spec scenario -->
- [ ] 0.3 Confirm it fails because the behavior is missing, not because the harness is misconfigured

## 1. <!-- Outermost component -->
<!-- kind: behavior -->

- [ ] 1.1 RED: Write failing tests for: <!-- scenario-derived names -->
- [ ] 1.2 GREEN: <!-- Implementation task -->
- [ ] 1.3 REFACTOR: <!-- Cleanup while tests remain green, or state why none is needed -->
- [ ] 1.4 Run the group tests — no regressions

## 2. <!-- Next component inward; add the contract or persistence gate if it applies -->
<!-- kind: behavior -->

- [ ] 2.1 RED: Write failing tests for: <!-- scenario-derived names -->
- [ ] 2.2 GREEN: <!-- Implementation task -->
- [ ] 2.3 CHECK: <!-- Contract gate: regenerate/re-inspect the published interface and review the diff -->
- [ ] 2.4 Run the group tests — no regressions

## 3. <!-- Structure-only refactor -->
<!-- kind: refactor -->

- [ ] 3.1 CHARACTERIZE: <!-- Confirm existing behavior is covered and green -->
- [ ] 3.2 REFACTOR: <!-- Change structure without changing the characterization tests -->
- [ ] 3.3 VERIFY: <!-- Run unchanged tests — no regressions -->

## N-1. Acceptance Test — Outer Loop GREEN
<!-- kind: behavior -->
<!-- Keep only when group 0 is present; DELETE alongside it otherwise. -->

- [ ] N-1.1 VERIFY: Confirm the acceptance test now passes end to end
- [ ] N-1.2 REFACTOR: Clean up harness setup and replaced collaborators if warranted

## N. Change Review
<!-- kind: operational -->

- [ ] N.1 CHECK: Dispatch an independent reviewer against proposal, specs, design, and tasks
- [ ] N.2 CHANGE: Fix every CRITICAL, resolve or accept each WARNING, re-run affected tests
- [ ] N.3 VERIFY: Confirm no blocking or unowned finding remains

## N+1. Documentation
<!-- kind: operational -->
<!-- OPT-IN. Keep this group ONLY when the change has durable information to add or
     correct in a maintained document. Omit it entirely otherwise — never add a generic
     documentation-review or changelog task.
     Each task names the document, section, and audience, states whether it adds,
     rewrites, or removes, and gives the durable reason it stays useful. A group adding
     more than ~10 lines to one document must also say what it corrects or replaces
     there. Where a repository keeps nested agent guidance, name the NARROWEST file that
     covers the rule. -->

- [ ] N+1.1 <Add|Rewrite|Remove> in <doc>: <section> (audience: <who>) — <what changes, what it replaces, and why it stays useful>

## N+2. Lint & Verify
<!-- kind: operational -->
<!-- Replace with this repository's actual commands — see the project context and rules. -->

- [ ] N+2.1 CHECK: Inspect the intended verification commands and affected tiers
- [ ] N+2.2 VERIFY: Run the project's linter — 0 errors
- [ ] N+2.3 VERIFY: Run the project's type checker — 0 errors
- [ ] N+2.4 VERIFY: Run the project's test suite — green
- [ ] N+2.5 VERIFY: Run the coverage gate when gated code is affected
- [ ] N+2.6 VERIFY: Run `openspec validate <change> --strict`
