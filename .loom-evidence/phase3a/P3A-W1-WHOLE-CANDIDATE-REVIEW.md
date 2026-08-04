# P3A-W1 Whole-Candidate Review (final roots)

Date: `2026-08-04`

Reviewer: fresh independent read-only Reviewer (`p3a_optrace_reviewer_v7`,
read-only). Scope: the complete P3A-W1 candidate — source inventory, the
full governance/evidence chain under `.loom-evidence/phase3a/`, the eight
final cross-client journey roots under `/private/tmp/loom-p3a-v5-roots/`
(`-r2` generation), and repository identity. All audits were performed
read-only; nothing was staged, committed, pushed, merged, launched or
mutated. The only file written by this Reviewer is this review artifact.

## Verdict

```text
P0 = 0
P1 = 0
P2 = 2 (documentation precision, required closures below)
Product/Authority: PASS
Operational/Trace Governance: PASS
Overall Whole-Candidate Review: PASS
```

## 1. Repository identity

- `pwd` = `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`
  = Git top-level.
- `git branch --show-current` = `codex/loom-platform-slice2`.
- `git rev-parse HEAD` = `6d380233b5b89309a1a7ce3919aa611654e0f4ee` (the
  Phase 2B baseline; no new commit since).
- `git diff --cached --stat` = empty (nothing staged).
- `docs/CURRENT.md` records the Phase 3A status as
  `P3A-W1 = CANDIDATE COMPLETE / AWAITING WHOLE-CANDIDATE REVIEW + ATOMIC COMMIT`,
  consistent with the observed worktree.

## 2. Source candidate boundary

- `P3A-W1/source-lock.json` (schema `loom.phase3a.p3a-w1.source-lock.v1`,
  generated 2026-08-04T11:00:40Z) declares 80 paths with
  `ordered_sha256_lines_digest = 0d5d4296ee1ff99e9af1c23792c1eae9ab81030d60dd0ff8fee1b75b83397666`,
  method = SHA-256 of lexicographically path-sorted
  `'<file_sha256>  <relative_path>'` lines.
- Independently recomputed the digest over the CURRENT disk state:
  **MATCH** (`0d5d4296…`). All 80 paths exist on disk and are either
  modified-from-baseline or new candidate files; none is missing.
- The `intentionally_excluded` list is exactly:
  `internal/projection/team_execution_test.go`, `AGENTS.md`, `PROGRESS.md`,
  `README.md`, `.codex/**`, `.loom-drafts/**`,
  `.loom-evidence/phase1-final-live-gate/**`,
  `.loom-evidence/phase1-slice3/**`, `.loom-evidence/phase2c/**`,
  `.loom-evidence/plan-amendments/**`, `apps/macos/.build/**`.
  `internal/projection/team_execution_test.go` is modified in the worktree
  but is NOT in the lock (verified).

## 3. Governance chain (closed, no open FAIL/P1)

Verified in `.loom-evidence/phase3a/`:

- Entry: `ENTRY-AUDIT.md` + `BOUNDED-ENTRY-AMENDMENT-PROPOSAL.md` +
  `ENTRY-AMENDMENT-DISCOVERY.md` + `ENTRY-AMENDMENT.md` with
  `ENTRY-AMENDMENT-REVIEW-1.md` PASS (P0=0 P1=0 P2=0).
- Gate 1: `GATE1-CONTRACT-REVIEW-1.md` FAIL (P1=4) -> `P3A-W1-CONTRACT-REPAIR-1.md`
  -> `GATE1-CONTRACT-REVIEW-2.md` FAIL (P1=2) -> `P3A-W1-CONTRACT-REPAIR-2.md`
  -> `GATE1-CONTRACT-REVIEW-3.md` PASS (P0=0 P1=0 P2=0; ADR-0013 accepted).
- Mandatory RED: `P3A-W1/red.md` recorded.
- Implementation: `P3A-W1-IMPLEMENTATION-REVIEW-FINAL.md` PASS and
  `P3A-W1-IMPLEMENTATION-REVIEW-FINAL-TREE.md` PASS (both P0=P1=P2=0);
  `P3A-W1-OWNED-PATH-REPAIR.md` + `-REVIEW-1` (FAIL, P2=1) +
  `-REVIEW-2` (PASS); `P3A-W1-IMPLEMENTATION-REPAIR.md` +
  `-REVIEW-1` (FAIL) + `-REVIEW-2` (FAIL) + `-REVIEW-3` (FINAL PASS).
- Amendments: `P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` +
  `-REVIEWS.md` (Reviews 1-3 FAIL -> Review 4 FINAL PASS,
  SHA-256 `e7710c140345f3da5a18a362547fa2f83da9388db3d8dfc4adc70c697c3b4965`);
  `P3A-W1-EXACT-LINEAGE-MATERIALIZATION-JOURNEY-AMENDMENT.md` +
  `-REVIEW-1` (PASS); `P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md` +
  `-REVIEW-3` (PASS) + `-REVIEW-4` (FINAL PASS, P0=P1=P2=0, SHA-256
  `4461f1d72f9b3c77f8aeac0b4b2a720a0a772299c583682c6a3690d4dc240e09`).
- Result reviews: `P3A-W1-OPERATIONAL-TRACE-REVIEW-FINAL-V5.md` (FAIL,
  F1/F2/F3) and `P3A-W1-PRODUCT-RESULT-REVIEW-FINAL-V5.md` (FAIL) are
  historical and their findings are closed by the `-r2` replacement roots,
  the corrected result.md wording, and the frozen GUI Evidence Surface
  Amendment. `P3A-W1-PRODUCT-RESULT-REVIEW-FINAL-V6.md` (PASS, P0=P1=P2=0)
  binds the final roots. See §6 for the dual-result record precision
  finding and the final-generation certification.
- `P3A-W1/JOURNEY-ROOTS-INVENTORY.md` (final) documents the eight final
  roots, the final binary digests and the superseded v1-v4 generations.

## 4. Journey roots — independent final-generation audit

The candidate's `scripts/verify-phase3a-cross-client-journey.sh verify --root`
returns PASS for all eight final roots (re-run by this Reviewer:
`happy-create-evaluate-activate-bind-execute-clean-r2`,
`cancel-reject-retain-r2`, `stale-view-digest-generation`,
`concurrent-single-winner`, `crash-before-cas-r2`,
`crash-after-cas-before-response-r2`, `projection-failure-rebuild-reconnect`,
`slow-client-redelivery-clean-restart`).

Independent checks (all PASS):

1. **Permissions** — every root 0700; every evidence file/dir 0700/0600 as
   required (spot-verified `stat` on all eight roots).
2. **Schema conformance** — required evidence files present in all roots;
   `gui/actions.jsonl`, `tui/keystrokes.jsonl`, `timeline.jsonl`,
   `ipc/request-response-summary.jsonl`, `daemon/structured-log.jsonl`,
   processes pre/postflight, journal/projection/artifact summaries and
   `manifest.json` present with the frozen §8 shapes (the verify script
   checks the full field set; this Reviewer additionally parsed every
   JSONL/JSON file per root).
3. **Journey correlation** — zero daemon records missing or diverging from
   the root's journey ID (`daemon_noj=0` in all eight roots); IPC logs carry
   both `client_kind=gui` and `client_kind=tui` (dual-client coverage).
4. **GUI evidence surface** (frozen
   `P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md` §3):
   - App-originated launch reads `loom-swift-<uuid>` present in every root,
     INCLUDING both crash roots (F1 closure): crash-before-cas-r2 has
     `loom-swift-e6d4eb8a-2cd1-49cb-8d2d-112e6ef96f5f` and
     `loom-swift-03ee7b71-828b-4d34-8594-37c6ebd47b04`; crash-after-cas-r2
     has `loom-swift-71441337-a137-4157-a4e1-3ff35d1ce42e` and
     `loom-swift-29fe68bf-e4d7-45d0-bc87-a4b283ee808a` (two per crash root,
     non-probe).
   - Screenshots: ≥1 real PNG per root (1-2 per root), stored 0600 under
     `gui/screenshots/`, PNG-signature verified; per the frozen Amendment,
     captures are not required to differ visually per journey because the
     main window renders the default workbench view.
   - Actions are driven through the production Swift client over the real
     socket; mutation/read rows (`create_skill`, `record_evaluation`,
     `activate`, `set_binding`, variants, read-back) are present in
     `gui/actions.jsonl`/IPC logs with the frozen §8 fields.
5. **Real PTY TUI** — every `tui/transcript.txt` contains the `Loom ·`
   marker (6-9 occurrences per root).
6. **Postflight/cleanup** — processes/sockets/locks/leases/temps all empty
   (`procs=0 sockets=0 locks=0 leases=0 temps=0`) and `cleanup-proof.txt`
   present in all eight roots; no socket/lock residue.
7. **Secret hygiene** — scan for `sk-*`, API-key, bearer-token and
   private-key material across all root evidence: no matches.
8. **Manifest integrity** — spot-verified the happy root's 23
   `evidence_files` digests/sizes/modes against disk: all match;
   `manifest_digest` self-consistent; `network_allowed=false` and
   `provider_credentials_present=false` in every root manifest.
9. **Binary identity** — on-disk `bin/loomd`, `bin/loom` and
   `app/Loom.app/Contents/MacOS/LoomLocalApp` match each root's
   `production_components` in all eight roots:
   daemon `5bd5d86ab918abd0f966c44105e3793b3fc5f32664a415209ea1987b7ed706b3`,
   TUI `29e3775c541d5264e4db5507719d555739524f31d4934ab8d9ef313bcb1813b3`,
   GUI `87c378122909eaf06aea78415e8fe9f941cf371b78d577cee38bf8c52fa14478`.
10. **Source binding** — fresh builds from the current tree are
    BYTE-IDENTICAL to the root binaries: `go build ./cmd/loomd` =
    `5bd5d86a…`, `go build ./cmd/loom` = `29e3775c…`, and the reviewed
    `scripts/build-loom-local-app.sh --output …/Loom.app` GUI bundle =
    `87c37812…`. The final roots therefore bind the final source for all
    three components.
11. **SQLite audit (independent, read-only)** — happy root: integrity `ok`,
    498 events, 0 duplicate `id`, 0 duplicate `idempotency_key`, 0 stream
    gaps, 492 journey-correlated (matches `result.md` 492/498).
    crash-after root: integrity `ok`, 8 events, 0 duplicates, 0 gaps, 5
    journey-correlated (platform-foundation exception).
12. **Scenario assertions (per-root journal/IPC truthfulness)** —
    - S1 happy: 1 definition / 1 revision / 1 candidate / 1 evaluation /
      1 activation / 1 binding / 2 materialization-published + 2 cleaned;
      projection 1/1/1/1/1/2 matches_journal.
    - S2 cancel-reject-retain: exactly 2 `CandidateCreated`, 1
      `CandidateRejected`, 1 `CandidateRetained`.
    - S3 stale-view-digest-generation: only create/definition/revision
      facts; rejected stale_view/wrong_digest activations produced ZERO new
      Journal facts (no evaluation/activation rows).
    - S4 concurrent-single-winner: exactly one
      `EvolutionAssetCandidateRejected` (single CAS winner).
    - S5 crash-before-cas: no `EvolutionAssetCandidateActivated` (zero
      activation).
    - S6 crash-after-cas-before-response: exactly one
      `EvolutionAssetCandidateActivated` (single committed effect).
    - S7 projection-failure: one activation; `projection/summary.json`
      `matches_journal=true` post-restart.
    - S8 slow-client: one activation; result.md records `error:timeout`,
      no hidden retry, no duplicate Event/Evidence on clean restart.

## 5. Source/verification matrices

- `go test -count=1 ./...` — ALL packages pass (fresh, uncached).
- `go vet ./...` — clean.
- `gofmt -l` over the locked `.go` paths — empty.
- `go mod tidy` — no diff vs. `go.mod`/`go.sum` at baseline.
- Swift (`apps/macos`): `swift build -c release` PASS; `swift test` —
  89 XCTest cases executed, 0 failures (1 skipped), plus 4 Swift Testing
  cases, 0 failures.
- The implementation review chain records the full race/vet/tidy/gofmt and
  Swift TSAN/Release matrix results with P0=P1=P2=0 (final + final-tree
  reviews); this Reviewer's independent rerun of the deterministic Go/Swift
  checks above confirms the current tree.

## 6. Findings (P2, documentation precision — required closures)

**P2-1 — `P3A-W1-DUAL-RESULT-REVIEWS.md` binds the superseded root
generation.** The file (written 16:59, before the final generation) cites
binary digests daemon `de6c3087…`/GUI `74b1f32f…` and the happy root's
147-event count, which match the v4 generation now explicitly superseded by
`JOURNEY-ROOTS-INVENTORY.md`. The final `-r2` roots bind daemon
`5bd5d86a…`, TUI `29e3775c…`, GUI `87c37812…` and the happy root records
498 events (492 journey-correlated). The inventory and `docs/CURRENT.md`
cite the dual-result file as the final certification, so the record must be
made coherent.

Required closure (no product/authority/journey change): re-issue or add a
dated addendum to `P3A-W1-DUAL-RESULT-REVIEWS.md` that (a) corrects the
binary digests to the final generation (`5bd5d86a…`/`29e3775c…`/
`87c37812…`), (b) names the eight final `-r2` roots and the happy-root
counts (498 events, 492 journey-correlated), (c) records that the earlier
Review A/Review B text applied to the superseded v4 generation, and
(d) records the final-generation certification chain: Product Result
Review V6 PASS (final roots) + GUI Evidence Surface Amendment Review 4
PASS (final roots) + this Whole-Candidate Review's independent
Operational/Trace audit of the final roots (§4 above, PASS). After the
addendum, re-run `verify` on the eight roots (unchanged roots, no re-freeze
required) and note the verification in the addendum.

**P2-2 — `P3A-W1/OPERATIONAL-TRACE-REVIEW.md` is titled "Final PASS" but is
scoped to the superseded v3/v4 generation.** Its binary digests
(`de6c3087…`/`74b1f32f…`) are consistent with the generation it names, but
the "Final" title can be misread as the final-generation certification.
Required closure: mark it as a superseded/historical review for the v3/v4
generation in the inventory (and/or amend its title), so the record
unambiguously attributes the final-generation Operational/Trace
certification to the chain in P2-1(d).

Neither finding affects product code, authority, schema, scenario
assertions, journey evidence or the verification matrices; both are
review-artifact precision issues.

## 7. Final-generation certification note

Because the dual-result review artifact predates the final generation, this
Whole-Candidate Review independently re-certifies the final-generation
Operational and Trace Behavior surface (schema, drift, dual-client, PTY,
screenshots, postflight, permissions, hygiene, manifest integrity, binary
identity and source binding, SQLite integrity, variant-path truthfulness)
on all eight `-r2` roots: **PASS**. Combined with Product Result Review V6
(final roots, PASS) and GUI Evidence Surface Amendment Review 4 (final
roots, PASS), the final generation satisfies the dual Result acceptance
once the P2-1/P2-2 documentation closures are applied.

## Conclusion

The P3A-W1 candidate is ready for exact staging and one atomic local commit
once the P2-1 and P2-2 documentation closures are applied to the review
records. No product code, schema, authority, journey or evidence change is
required. Nothing was staged, pushed or merged.

VERDICT: `PASS` (P0=0, P1=0, P2=2 — required documentation closures)
