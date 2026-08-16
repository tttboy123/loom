# Phase 4 · 4.1 RoundTable — 06 Security Evidence

Status: `PASS`
Date: 2026-08-17

## Permissions

- Daemon evidence root and artifact shards:
  `stat -f '%Sp'` on `~/Library/Application Support/Loom/state/evidence` and
  `artifacts/sha256/...` → `drwx------` (0700).
- Artifact file → `-rw-------` (0600).
- Live journey logs in this tree → `-rw-------` (0600), matching the phase2d
  `acceptance/live/` convention; committed summary `.md` evidence follows the
  repo convention for evidence summaries (0644), identical to
  `.loom-evidence/phase2d/*.md`.

## No secrets in Journal / Evidence

- Journal roundtable payloads scanned (all rt-live-e2e sessions) for
  `sk-`, `BEGIN *PRIVATE`, `Bearer`, `api_key`, `password` → 0 matches.
  Journal facts carry only: session id/title/moderator seat, seat names,
  bounded message body (the governed handoff content), digests, correlation
  UUIDs, timestamps.
- Evidence AlignmentSummary artifact scanned with the same pattern → 0
  matches. Artifact carries only canonical summary fields + bounded message
  bodies + digest-only artifact refs.

## No moderator-confirmed message is delivered

- A message is only `inserted` (delivered into the round) after the target
  seat `acknowledged` a `relayed` message; the moderator gates relay/insert/
  drop/conclude (non-moderator relay rejected in tests + route test
  `not_moderator`). Dropped messages are never inserted.

## External capabilities fail closed

- Default production composition does not publish any remote tool / A2A
  endpoint; RoundTable exposes only the strict local `roundtable_*` IPC
  surface. External A2A adapters are explicitly out of scope for 4.1.

## Verdict

Gate 6 `PASS`: no credential/prompt/Provider body in Journal/evidence/logs;
per-hop moderator confirmation is enforced; external capability remains
fail-closed.
