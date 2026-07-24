# S2-W1 Fresh Implementation Review 1

- Review type: fresh independent read-only strict implementation review
- Candidate contract SHA256:
  `3b3a54f71419d59ac67c81edface6bcc1dd2ff9c6603bc4c3cce83be4bc599d0`
- Branch/head: `codex/loom-platform-slice2` at `f0820be`
- Verdict: `FAIL`
- Product repair attempt consumed: `1/3`

## Required findings

1. `ResolveDefinition` treated an empty `ResolutionContext.DefinitionID` as a
   wildcard. Mixed stable IDs could therefore resolve an unintended role
   instead of failing closed. The frozen contract resolves definitions within
   one stable ID.
2. The RuntimeProfile switching test compared the same AgentDefinition value
   twice without exercising a meaningful external profile association. It did
   not prove the frozen independence acceptance boundary.
3. The worktree contains Controller state surfaces and pre-existing unrelated
   changes outside the Developer-owned product Candidate. Their classification
   must be explicit before an atomic S2-W1 commit.

## Independent verification

The Reviewer independently reran and observed exit `0` for:

```text
focused GREEN
package full
impact
focused race -count=50
repository race
go vet
gofmt check
git diff --check
```

Those green checks do not override the required correctness and evidence
findings.

## Repair gate

Repair 1 must:

- add a failing regression test before changing production code;
- require an explicit stable Definition ID and prove mixed-ID inputs cannot
  resolve the wrong role;
- replace the hollow independence assertion with a structural/external
  association proof that RuntimeProfile selection remains outside
  AgentDefinition;
- preserve all runtime package behavior and all frozen ownership/trust
  boundaries;
- classify `docs/CURRENT.md`, `PROGRESS.md`, and `.loom-evidence` as
  Controller-owned status/evidence surfaces, and exclude the pre-existing
  `AGENTS.md` change plus `.codex`/`.loom-drafts` from the S2-W1 commit.

VERDICT: FAIL
