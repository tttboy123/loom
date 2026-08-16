# P2D-W2D Live Acceptance Readiness V33

Status: `ACCEPTANCE READY / INSTALLED-LIVE GATES OPEN, OPERATOR EXECUTION REQUIRED`

Date: 2026-08-16

## Acceptance boundary

V33 makes every remaining Phase 2D completion gate executable. The source
chain behind each gate is source-verified (V26-V32 and predecessors); the
installed-live gates (CV6, real conversation, mixed-Team ATL9, single-Agent
failure isolation matrix, installed Web/MCP diagnostics, accounting UI) require
a real installed App bundle, real Provider credentials and an approved
operator. V33 itself performs no live execution, installs no App and touches no
credential.

## Deliverables

- `.loom-evidence/phase2d/acceptance/PHASE-2D-LIVE-ACCEPTANCE.md`: the matrix
  and runbook for gates G1-G6 with prerequisites (build/install commands),
  per-gate steps, pass criteria, evidence files and privacy rules.
- `scripts/phase2d-live-acceptance.sh`: a safe source library that checks
  installed App/daemon prerequisites, prints a structured PASS/OPEN summary,
  and exports bounded owner-only (`0600`) privacy-safe gate evidence. It never
  reads, writes or transmits credentials, secrets, Prompts or Provider bodies,
  and runs in source-only mode (`LOOM_PHASE2D_ACCEPTANCE_SOURCE_ONLY=1`).
- `scripts/test-phase2d-live-acceptance.sh`: validates the harness in a
  private temp dir — source-only load, forbidden secret-shaped markers,
  required functions, missing/invalid app classification, all-gates-OPEN state,
  `0600` evidence permissions, invalid gate/incident rejection.
- Release-readiness re-confirmed: `scripts/test-build-loom-local-app.sh` PASS
  (unsigned App bundle build + smoke test; no credentials or network).

## Gate matrix (installed-live, operator-executed)

| Gate | Content | Source proof |
|---|---|---|
| G1 | Installed credential import / vault restart / revoke persistence (CV6) | vault + credential broker source; V22-V31 chain |
| G2 | Real conversation through frozen Profile + Vault lease + bounded reply | conversation router source (V20+); `provider/*_conversation.go` |
| G3 | Mixed Provider Team ATL9 (4 Agents, independent accounts/models) | `TestFourProviderTeam*` source tests |
| G4 | Single-Agent failure isolation matrix (revoke / corrupt record / rate limit / timeout / enrollment drift) | `TestFourProviderTeamRevokedCredentialIsolatesOneAgent`, `TestFourProviderTeamCorruptVaultRecordIsolatesOneAgent`, timeout classification, V32 preflight |
| G5 | Installed Web/MCP diagnostics + revocation isolation | V29-V32 source chain |
| G6 | Per-Account accounting + governance UI (provider_reported vs rate_card_estimate, accounting-incomplete) | governance visibility + cost provenance source |

## Verification

Passed:

- `sh -n scripts/phase2d-live-acceptance.sh`
- `sh scripts/test-phase2d-live-acceptance.sh` (fixture PASS)
- `scripts/test-build-loom-local-app.sh` (native app build fixture PASS)
- `git diff --check`

## Privacy and live status

No App was installed or launched, no network request was made, no Provider,
real credential or user workspace was used, and no remote tool capability was
published. The six gates remain OPEN and operator-executable.

Phase 2D remains the sole `ACTIVE / PARTIAL` Goal. Definition of done: all six
gates PASS with evidence files, full source verification green, and
`docs/CURRENT.md` records Phase 2D `COMPLETE / INSTALLED LIVE`.
