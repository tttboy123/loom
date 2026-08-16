# P2D-W2D Failure Isolation Source Matrix V34

Status: `SOURCE VERIFIED / INSTALLED-LIVE MATRIX OPEN`

Date: 2026-08-16

## Acceptance boundary

V34 adds the consolidated source-level single-Agent failure isolation matrix
for the Phase 2D G4 gate. It proves, at the Team coordinator boundary, that
every runtime failure class stays Agent-local: the affected sub-agent's Attempt
fails with the exact closed terminal reason and the node is recovery-blocked,
while healthy peer sub-agents and the main Agent succeed and no peer inherits
the failure. It does not replace the installed-live matrix, which still
requires real Providers and an installed App.

## Matrix cells (source)

`TestPhase2DPerAgentFailureIsolationMatrix` in `internal/app/team_execution_test.go`
drives a four-Provider Team (`openai` main / `anthropic` sub-a / `kimi` sub-b /
`minimax` sub-c) and, per cell, injects the failure on exactly one sub-agent:

| Cell | Affected Agent | Attempt result | Peers |
|---|---|---|---|
| provider_rate_limited | sub-a | failed / `provider_rate_limited` | all succeeded |
| provider_timeout | sub-b | failed / `provider_timeout` | all succeeded |
| provider_insufficient_balance | sub-c | failed / `provider_insufficient_balance` | all succeeded |
| credential_unavailable | sub-a | failed / `credential_unavailable` | all succeeded |

For every cell:

- the Team result is `blocked` (the failed sub-agent's recovery blocks
  aggregation) with all four nodes executed;
- the affected node is `blocked` with exactly one failed Attempt carrying the
  exact closed terminal reason;
- every healthy node (other sub-agents and main) is `succeeded` with an empty
  terminal reason — the failure never leaks to a peer.

Preflight-block classes (credential revision conflict, Enrollment revoked /
policy drift) are proven by the existing V32 per-Agent preflight tests and the
W2C binding tests; the installed-live matrix re-runs the same cells through the
App.

## Verification

Passed:

- `go test ./internal/app/ -run TestPhase2DPerAgentFailureIsolationMatrix -count=1`
- `go test ./internal/app/ -count=1` (complete package)
- `go test -race ./internal/app/ -run 'TestPhase2DPerAgentFailureIsolationMatrix|TestFourProviderTeam' -count=1`
- `go vet ./internal/app/`, `gofmt -l` clean, `git diff --check` clean,
  `go build ./...`
- `go test ./internal/work/ ./internal/runtime/ ./internal/toolbroker/enrollment/ -count=1`

## Privacy and live status

No App was installed, no network request, Provider, real credential or user
workspace was used, and no remote tool capability was published. The
installed-live G4 matrix remains operator-executable via the V33 acceptance
runbook. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
