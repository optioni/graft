## Context

<!-- Background, current state, constraints -->

## Goals / Non-Goals

**Goals:**
<!-- What this design aims to achieve -->

**Non-Goals:**
<!-- What is explicitly out of scope -->

## Boundaries

<!-- Modules, layers, and collaborators this change touches, and which existing pattern
     each new piece follows. -->

## Contracts

<!-- For any interface a separate consumer depends on: shape, error surface, pagination
     or streaming, and compatibility. Additive or breaking? Name every consumer affected. -->

## Persistence and Rollout

<!-- State each of these, using "none" rather than omitting:
     migration · backfill · seeding · cache invalidation · index rebuild ·
     authorization · observability · deployment -->

## Test Boundaries

<!-- Every external dependency and collaborator this change touches. "real" and "replaced"
     are both answers; silence is not. No task may invent a boundary this table omits. -->

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| <!-- datastore --> | <!-- real (containerized) --> | <!-- replaced --> |
| <!-- external API --> | <!-- mocked at the HTTP layer --> | <!-- mocked --> |

## Test Strategy

<!-- Map each behavior to the fastest tier its dependencies allow — see the project
     context for this repository's tiers and their commands. State whether this change
     takes the outer-loop acceptance test, and if not, why not. -->

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| <!-- Exact scenario name --> | <!-- Test or deterministic evidence --> | <!-- tier --> | <!-- real and replaced --> | <!-- focused command --> |

## Decisions

<!-- Key design decisions, rationale, and alternatives considered -->

## Risks / Trade-offs

<!-- [Risk] → Mitigation -->

## Migration Plan

<!-- Deploy order, migration and backfill steps, rollback strategy — or why none are needed -->

## Open Questions

<!-- Unresolved decisions, or state that none remain -->
