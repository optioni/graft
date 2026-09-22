## ADDED Requirements

### Requirement: <!-- requirement name -->
<!-- requirement text, using SHALL/MUST -->

#### Scenario: <!-- scenario name -->
- **WHEN** <!-- concrete input or caller condition -->
- **THEN** <!-- observable outcome -->
- **AND** <!-- relevant side effect: what is persisted, published, or invalidated -->

#### Scenario: <!-- unauthorized or failure case -->
- **WHEN** <!-- caller lacks permission, or the domain rule is violated -->
- **THEN** <!-- the failure the caller observes -->
- **AND** <!-- what is NOT persisted or published -->

## MODIFIED Requirements
<!-- DELETE this section when no existing requirement changes.
     Copy the ENTIRE requirement block from the live openspec/specs/<capability>/spec.md as
     it stands at current HEAD — not from an earlier delta, not from memory. Header text
     must match exactly. Then DIFF your edited block against the live one and account for
     every difference: each is either this change's edit or a silent revert of work that
     landed since. This block replaces the live requirement whole at archive time, so a
     stale copy does not conflict — it reverts. A block carried back byte-identical is not
     a modification; drop it. -->

### Requirement: <!-- exact name as it appears in the live spec -->
<!-- full updated requirement text, with every scenario the live requirement carries -->

#### Scenario: <!-- carried or changed scenario name -->
- **WHEN** <!-- concrete input or caller condition -->
- **THEN** <!-- observable outcome -->
