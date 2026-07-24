# ADR-0005: Local evolution Sidecar with candidate activation

**Date**: 2026-07-24
**Status**: accepted
**Deciders**: lune, Codex

## Context

Loom should improve the user's personal experience by learning preferences, reusable procedures and effective team patterns. Direct online mutation of Agent definitions or rules would make active behavior unpredictable and couple learning failures to task execution. Personal traces can also contain sensitive project context.

## Decision

Loom runs evolution as a separate local Sidecar after terminal tasks or during idle time. The Sidecar receives redacted data through a restricted daemon API, generates versioned candidates, evaluates them offline and requires user activation before they affect Memory, Skill, AgentDefinition or strategy.

## Alternatives Considered

### Alternative 1: Mutate active Agents online

- **Pros**: Immediate adaptation without user maintenance.
- **Cons**: Uncontrolled regressions, hard-to-reproduce behavior and privilege drift.
- **Why not**: Learning output must not silently change an executing team.

### Alternative 2: Keep all Agent and Skill definitions static

- **Pros**: Maximum predictability and simpler storage.
- **Cons**: No personal compounding and repeated setup for similar work.
- **Why not**: It removes a core product advantage while ignoring useful execution evidence.

## Consequences

### Positive

- Learning failures cannot block or mutate active execution.
- Candidates are measurable, reviewable, versioned and reversible.

### Negative

- Improvements arrive after evaluation and user activation rather than immediately.
- The daemon needs a redaction and candidate-ingestion API.

### Risks

- Sensitive traces could enter evolution storage; mitigate with local encryption, project partitions, explicit retention and minimum-context retrieval.
