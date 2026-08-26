# Phase 2D trust, capacity and route-transition acceptance

Status: `SOURCE + INSTALLED GOVERNANCE VERIFIED`

Date: 2026-08-24

Bundle: Loom `0.5.3` build `127`

## Accepted boundary

- Route review derives the source trust policy from the last immutable
  Segment execution binding. Trust domain, retention mode and data region are
  compared independently. A changed dimension requires a separate user
  acknowledgement before confirmation; unchanged policy does not.
- Context Capsule admission freezes an explicit capacity authority. Exact,
  estimated and unavailable states are distinct. The deterministic input
  budget subtracts reserved output and adapter/tool overhead, contribution
  totals are privacy-safe, and every omission remains explicit. Unknown model
  capacity fails closed to `unavailable`; Loom does not invent precision.
- Route Transition acceptance covers all three disclosure modes through the
  authenticated private UDS boundary. Missing or stale expected bindings cause
  zero mutation and zero responder calls. An accepted transition freezes the
  target binding, Capsule/receipt digests and Incident correlation.
- Explicit Codex models advertised by the catalog now remain exact through
  execution-binding and Context-target resolution. Cross-Provider and unknown
  model substitutions still fail closed.

## Verification

- `go test -p 4 ./... -count=1`: PASS.
- `go test -race ./internal/contextcapsule -count=1`: PASS.
- Focused `internal/api` and `cmd/loomd` Route Transition, capacity persistence,
  tamper and Codex model race tests: PASS.
- `go vet ./...`: PASS.
- `swift test --package-path apps/macos`: 342 tests, 2 conditional skips,
  0 failures.
- `git diff --check`: PASS.

The installed App was built with the repository builder, passed strict deep
code-sign verification and transactional installer dry-run/replacement.

- App executable SHA-256:
  `fb4ebbce2487dbc19a92fb18c6ca63fae25d0ea8cdc25d1bb514d7ed9072ec3b`
- Bundled daemon SHA-256:
  `3011561ec9175e9e162f4e611868f7ccc6dcaf92a38275dd8807b642fdd77a2f`

Installed accessibility inspection verified:

- the grouped Harness -> Provider Account Route menu remains intact;
- DeepSeek to MiniMax review names all three trust-boundary changes;
- Confirm is disabled before acknowledgement and enabled afterward;
- `continue_with_context`, `summary_only` and `start_clean` are independently
  selectable;
- legacy Segments honestly render unavailable capacity instead of fabricated
  model limits;
- App shutdown also terminates its managed daemon, and restart reconstructs
  the canonical daemon argv before the UI automatically returns to
  `Local service ready`.

No credential was read or changed and no real Provider request was made for
this acceptance. Previously accepted real-conversation evidence remains the
networked dispatch proof. The four-distinct-Provider Team, real-account
revoke/rate-limit matrix and custom-endpoint live Conversation remain explicitly
deferred to a later Phase.
