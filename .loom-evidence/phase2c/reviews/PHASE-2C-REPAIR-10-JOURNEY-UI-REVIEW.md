# Phase 2C Repair 10 Journey, UI, and Engineering Review

**Review type**: independent read-only final review  
**Source lock**: `69278d6bb5b08fe47cd370e21de5ebee74e73d3e17625b1eccad993cd967508f`  
**Fixture**: Attempt 009  
**Verdict**: FAIL  
**Counts**: P0=0, P1=3, P2=2

## Findings

### P1 - TUI Attention recovery is visually stale after `r`

The recovered authoritative snapshot is online and has removed
`restore_runtime`, but the TUI `r` handler on `ScreenAttention` only calls
`loadPermissionAttention()`, while the Attention view renders
`model.snapshot.Attention`. This explains why `restore_runtime` stayed visible
until navigating to Runtimes and refreshing the primary snapshot.

**Mandatory fix**: make Attention refresh the primary product snapshot, or
otherwise refresh the exact data it renders, then rerun J7 and J9 in a real PTY.

### P1 - J10 lacked the frozen live screenshot matrix at review time

The reviewed fixture contained only one live light screenshot and explicitly
recorded `live_dark_compact_screenshots: false`. Static XCTest rendering was
useful but did not satisfy the required live matrix.

**Mandatory fix at review time**: capture the required dark, compact, and width
screenshots or amend the contract. This finding was subsequently closed without
source changes by six signed-Release `screencapture` checkpoints at light/dark
900/1080/1440. Attempt 009 remains failed because the other P1 findings remain.

### P1 - J9 native accessibility/keyboard coverage is incomplete

The fixture proves real TUI traversal and native static accessibility tests but
does not contain a live/manual native traversal record. Computer Use may remain
skipped, but the accepted alternative still must prove unique controls, visible
focus, and Escape behavior.

**Mandatory fix**: provide live native traversal evidence or amend the gate.

### P2 - Journey ID traceability is split

Main TUI snapshot/chat/setup/mission and GUI requests use the replacement
Journey ID, while TUI `permissions_attention` uses a generated second ID. The
decision fixture also has its own intentional Journey ID. This is not an
authority failure, but it weakens auditability.

**Follow-up**: pass and record one explicit main Journey ID consistently, and
list intentional subfixture IDs in the evidence manifest.

### P2 - native Recent labels are too generic

The rail collapses attention items to repeated `Needs your attention` labels.
Governance closed by default is correct and chat remains unmistakably primary.

**Follow-up**: include action or Mission context in the recent title or subtitle.

## Positive Evidence

- Repair 10 source bytes match ordered lock digest
  `7396c1c42e0663f2ccd60a6ec51b53577884ab2cd9a1583349ff4afcea7f11ee`.
- The complete deterministic matrix is green.
- The Journal supports the controlled Runtime offline-to-online authority path.
- No P0 authority or security issue was found.

## Final Adjudication

Repair 11 is required before installable Phase 2C acceptance. Mandatory work is
the Attention refresh correction plus live native J9 evidence. The Journey ID
and Recent-label P2 findings must be solved or explicitly accepted by the final
whole-Phase review; they may not disappear from the evidence chain.

