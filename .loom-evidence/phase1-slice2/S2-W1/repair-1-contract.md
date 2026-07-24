# S2-W1 Repair 1 Contract

- Lineage: `S2-W1`
- Repair attempt: `1/3`
- Risk: Strict
- Trigger: fresh implementation Review 1 `FAIL`
- Frozen ownership: unchanged
- Product files allowed to change:
  `internal/agents/definition.go`,
  `internal/agents/definition_test.go`
- Runtime production and tests: unchanged

## Mandatory Repair RED

Before changing `definition.go`, tests must prove:

1. an empty requested `DefinitionID` fails closed with a typed error;
2. a target stable ID resolves only within that ID even when another ID has a
   higher-precedence or higher-version eligible definition;
3. RuntimeProfile selection is represented outside AgentDefinition, and
   switching that external association preserves the same validated definition
   without cloning or mutation; and
4. AgentDefinition still has no RuntimeProfile/model/Provider/Runtime authority
   field.

The focused repair RED must fail against the current production digest because
empty Definition ID is still accepted, not because of syntax, environment,
dependency, or unrelated failures.

## Minimal repair

- Require a non-empty `ResolutionContext.DefinitionID`.
- Remove wildcard stable-ID matching.
- Keep project > reusable > transient precedence and highest-version behavior
  within the requested stable ID.
- Do not add a profile-switching product API, binding authority, catalog,
  persistence, discovery, or new dependency. The independence proof remains
  structural and external to AgentDefinition.

## Checks

- repair RED and focused GREEN;
- full `internal/agents` and `internal/runtime` packages;
- focused race `-count=50`;
- repository impact and race;
- `go vet ./...`;
- gofmt, diff, branch/head, owned-scope, import-boundary, and digest checks;
- fresh independent implementation Reviewer.

## Scope classification

- Developer-owned Candidate: the four product/test files in the original
  contract.
- Controller-owned status/evidence:
  `docs/CURRENT.md`, `PROGRESS.md`, and
  `.loom-evidence/phase1-slice2/S2-W1/`.
- Pre-existing unrelated and excluded from the S2-W1 commit:
  `AGENTS.md`, `.codex/`, and `.loom-drafts/`.
