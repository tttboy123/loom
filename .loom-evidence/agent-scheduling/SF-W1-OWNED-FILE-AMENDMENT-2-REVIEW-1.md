# SF-W1 Owned-File Amendment 2 — Independent Review 1 (PASS)

Date: `2026-08-04`

Reviewer: fresh independent read-only Reviewer (separate from the Amendment
authorship). Scope: `SF-W1-OWNED-FILE-AMENDMENT-2.md`, against the accepted
`SF-WORKITEMS.md` (SF-W1 allowlist), the accepted P3A commit `7d5f0b01`
(`cmd/loomd/run.go`, `cmd/loomd/product_daemon.go`), the live SF-W1 worktree
(`cmd/loomd/sf1_queue_wire_test.go`, `scripts/verify-sf1-cross-client-journey.sh`,
`docs/runbooks/sf1-cross-client-journey.md`), `SF-W1-SOURCE-LOCK.json`, and
the accepted P3A evidence substrate
(`scripts/verify-phase3a-cross-client-journey.sh`,
`docs/runbooks/phase3a-cross-client-journey.md`).

## Verdict

```text
P0 = 0
P1 = 0
P2 = 1
Product/Authority: PASS
Operational and Trace Governance: PASS
VERDICT: PASS
```

## §5 acceptance checks (each independently verified)

1. **Bounded supersession.** The Amendment amends only the SF-W1 owned-file
   allowlist (per its §2) with four paths and additive-only carve-outs. The
   live worktree diff of `SF-WORKITEMS.md`/`SF-EXIT-CONTRACT.md` contains
   only the already-reviewed Schema Amendment 1/2 references and the
   Owned-File Amendment 1 reference; no Amendment-2 text is integrated into
   the contracts yet (correct: integration follows this Review PASS). No
   SF-W2/SF-W3 allowlist, schema, authority boundary, RED item, journey
   definition, verification matrix, or acceptance item is changed by the
   Amendment's operative content. ADR-0014 and `SF-EXIT-CONTRACT.md` remain
   untouched by its operative content.
2. **Four additions are strictly additive delivery/evidence substrate
   required by the frozen SF-W1 Candidate and its atomic commit.**
   - `cmd/loomd/run.go` — verified against the accepted P3A commit
     `7d5f0b01`: `product_daemon.go` calls
     `newDaemonBuildFailure("build_assets", …)` at five sites (lines
     1693/1697/1714/1732/1736), while `validDaemonBuildFailureReason` in
     `run.go` does not contain `build_assets`; the P3A-committed reason
     therefore silently degraded to `build_unknown` (fallback in
     `newDaemonBuildFailure`). The live worktree diff adds exactly the two
     strings `build_assets`, `build_queue` to the validator, with no other
     behavior, reason, or validation-semantic change. The Amendment's
     factual claim is confirmed; the completion is a bounded defect
     completion, not a new product behavior.
   - `cmd/loomd/sf1_queue_wire_test.go` — new test in package `main`
     (`TestSF1QueueWireOverSocket`). It drives the production
     `localProductHandlerWithComposition` dispatch over a real Unix socket
     (`localipc.NewServer`/`localipc.NewClient`) with the production
     `CallJourney` client, covering `queue_command` create_job (receipt with
     2 Event IDs), `queue_snapshot` (1 admitted Job), duplicate-DAG-node
     rejection with wire `conflict` code, and duplicate gap observation
     converging on one `gap_id` with `merge_duplicate` and zero new facts.
     The test passes independently (`go test -count=1
     -run TestSF1QueueWireOverSocket ./cmd/loomd/` → ok, 4.9s). The
     Amendment-1 allowlist covered `product_daemon_test.go` only; this new
     path was genuinely missing.
   - `scripts/verify-sf1-cross-client-journey.sh` — new read-only journey
     verifier mirroring the accepted P3A verifier structure: 0700/0600
     evidence modes, manifest self-digest, journey-ID drift checks across
     daemon/IPC logs, dual-client traffic (`gui`/`tui` kinds), production
     app launch reads (`loom-swift-*` rows ≥ 2), real-PTY transcript
     (`Loom ·`), screenshot presence, SQLite integrity, zero duplicate
     Event/idempotency keys, zero stream gaps, `matches_journal=true`,
     event-summary/sqlite-summary identity, binary digest identity,
     canonical `view_version` from stream heads, evidence-file
     digest/mode identity, postflight cleanliness (processes/sockets/locks/
     leases/temps), socket/lock residue, and secret-like material. The
     script performs no writes, no deletes, and starts no processes; it
     fails closed with distinct exit codes.
   - `docs/runbooks/sf1-cross-client-journey.md` — new runbook documenting
     the frozen SF-W1 journey (two shared-owned-path Jobs admitted with
     Conflict Arbiter serialization, DAG-cycle typed `denied` rejection,
     one digest-bound Gap Proposal with duplicate convergence, daemon
     restart/reconnect checkpoint) and the verification command, following
     the accepted P3A runbook format and the frozen alternative-verification
     method, including the GUI evidence surface as defined by
     `P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md`.
3. **No authority/schema/RED/journey/acceptance change.** The Amendment's
   operative content is limited to allowlist additions with additive-only
   carve-outs; §4 explicitly denies any other change, and none appears in
   the text or the live diffs covered by the four paths.
4. **No SF-W4 / thin WorkItem.** The Amendment explicitly attaches to the
   existing and only SF-W1; no new WorkItem, split boundary, or vertical
   capability is created.
5. **Digest binding.** `SF-W1-SOURCE-LOCK.json` records
   `owned_file_amendment_2 = c6999e3ce2550cad6b1ca659ae6568a619f78c3feb7e52f5395e8b1657086669`,
   which matches the live SHA-256 of `SF-W1-OWNED-FILE-AMENDMENT-2.md`
   exactly; the other three amendment digests in the lock also match their
   live files exactly.

## P2 finding (non-blocking, closure required before the SF-W1 atomic commit)

`SF-W1-PROGRESS.md` states "`SF-W1-OWNED-FILE-AMENDMENT-2.md` + Review 1
PASS" while this review is the one producing that PASS and no review
artifact existed at the time the progress document was written (the
Amendment's own status header at freeze time said `PENDING REVIEW`). The
source lock was likewise generated before this review landed. Closure:
update `SF-W1-PROGRESS.md` to reference this review artifact and its PASS
verdict (and any later Review 2, if one is required), and confirm the
final evidence bundle is generated/regenerated only after the review
verdict, before exact staging and the atomic commit. This is a
documentation-precision / process-ordering item; it does not affect the
Amendment's substance, the product code, or any authority/schema/RED/
journey/acceptance boundary.

## Conclusion

The Amendment is bounded, additive, factually accurate (the `build_assets`
validator gap in the accepted P3A commit is confirmed), and required for
the frozen SF-W1 journey and its one atomic local commit. It closes the
implementability gap between the accepted allowlist and the delivered
SF-W1 Candidate without expanding authority, schema, RED, journey, or
acceptance scope.

VERDICT: `PASS`
