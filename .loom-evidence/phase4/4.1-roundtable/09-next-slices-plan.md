# Phase 4 · 4.1 RoundTable — 09 Next Slices (4.2–4.5) Frozen Prompts

Status: `PLANNED / READY` — execute one slice at a time, freeze each before the
next. Every slice is a separate atomic commit; no two slices share writers.
Date: 2026-08-17

Shared invariants (unchanged): Event Journal is the only authority; writes only
via `AppendBatchIfStreamHeads` CAS; projections rebuildable; strict IPC +
`validMethod`/Swift allowlist/route registry three-way consistency; Evidence
dirs 0700 / sensitive files 0600; digest-only references; no credential/Prompt/
Provider body in Journal/evidence/logs; external capabilities fail-closed;
single writer per slice.

---

## 4.2 — 导入/导出合同（ContextPacket / AlignmentSummary）

Goal: make a concluded RoundTable handoff exportable and re-importable across
both surfaces (macOS + TUI) with a bounded, versioned, provable contract.

In scope:
- Define a versioned `ContextPacket`/`AlignmentSummary` export document that
  carries: schema version, session id, provenance (moderator + seats + digest
  chain), expiry (not before X / not after Y), bounded message bodies, and
  digest-only artifact references.
- Export IPC (`roundtable_export`) that materializes the document as an
  Evidence artifact (content-addressed, 0600) and records the export fact in
  the Journal (CAS).
- Import IPC (`roundtable_import`) that verifies provenance + digest + expiry +
  dual-surface permission, is idempotent (same packet → same result, no double
  session), and rejects tampered/expired/foreign packets.
- macOS + TUI entry points for export/import; README/guide entries.

Out of scope (frozen): remote transfer, cloud sync, multi-tenant identity,
credential exchange, auto-relay.

Acceptance gates:
1. Export → artifact exists, sha256 == Journal digest, provenance fields
   present; import of that artifact reproduces the identical session view
   (same digest) on both surfaces.
2. Expired packet rejected; tampered digest rejected; foreign provider account
   rejected; import twice → idempotent (no duplicate session, no second
   continue).
3. Strict IPC three-way + strict decode (unknown fields rejected).
4. Installed-live E2E on the real socket (export → import → restart replay).
5. Security scan: no secret in exported/imported content or logs.
6. Independent review + operator sign-off before 4.3.

Verification: focused → package → race → vet → gofmt → `git diff --check` →
full Go suite → `swift test` → installed live E2E.

---

## 4.3 — 可替换 Provider 路由后端

Goal: decouple RoundTable message transport from the current local-only
enforcement so a future external seat can plug in without changing the ledger.

In scope:
- Define a `RoundtableTransport` seam (interface) behind the existing
  moderator/relay semantics; the local IPC transport remains the only active
  implementation in 4.3.
- Route selection by session/seat capability metadata with explicit
  fail-closed when a transport is unavailable.
- Keep Journal CAS + Evidence + strict IPC unchanged; add typed route
  availability/error codes.

Out of scope: actually wiring an external transport, cross-account auth,
credential adapters.

Acceptance gates:
1. Transport seam unit-tested; default local transport behavior bit-identical
   to 4.1 (live journey replay).
2. Unavailable transport → typed `state_unavailable`/`transport_unavailable`,
   never a silent fallback that bypasses moderator confirmation.
3. No new authority; all writes still CAS; evidence/digest rules intact.
4. Installed-live regression (4.1 journey) + independent review + sign-off.

---

## 4.4 — 外部 A2A 席位（依赖 4.2 + 4.3）

Goal: a bounded external AgentCard/A2A seat that can participate in a
RoundTable hop without becoming a second authority.

In scope (only after 4.2 + 4.3 land and freeze):
- One explicit external seat adapter behind the 4.3 transport seam, limited to
  propose/ack of digest-bound messages; moderator still gates relay/insert/
  conclude locally.
- A2A handshake metadata + capability disclosure; outbound stays proposal-only.

Out of scope: multi-user tenancy, auto-approval, auto-relay, cross-account
credential reuse, cloud broker.

Acceptance gates:
1. External seat proposes/acks only; any attempt to relay/insert/conclude from
   outside is rejected (moderator gate intact).
2. Full 4.1 journey + one external hop live; restart replay identical.
3. Secret/prompt scan clean; A2A adapter fail-closed without handshake.
4. Independent review + operator sign-off.

---

## 4.5 — 协作平台适配 + 可选 Web UI

Goal: surface RoundTable on a collaboration platform (e.g., Multica-class) and
optionally a Web UI, without weakening the local-first authority.

In scope:
- A read-only/confirm-only bridge mapping Journal facts to the platform's
  thread model; all writes stay behind the daemon + CAS.
- Optional static Web UI (read + confirm) served locally; no credential in the
  browser.

Out of scope: turning the platform into a write authority, cloud sync of the
Journal, removing local-first guarantees.

Acceptance gates:
1. Platform thread renders the AlignmentSummary + message status from the
  Journal projection; confirm actions go through the same IPC/CAS path.
2. Web UI read-only by default; write actions require moderator identity +
  policy; no secret leaves the daemon.
3. Full 4.1 journey regression + platform/Web live checks.
4. Independent review + operator sign-off; then Phase 4 complete.
