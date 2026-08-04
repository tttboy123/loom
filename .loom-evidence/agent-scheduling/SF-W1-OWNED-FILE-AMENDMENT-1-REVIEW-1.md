# SF-W1 Owned-File Amendment 1 — Independent Review 1 (PASS)

Date: `2026-08-04`

Reviewer: fresh independent read-only Reviewer (separate from the Amendment
authorship). Scope: `SF-W1-OWNED-FILE-AMENDMENT-1.md`, against the accepted
`SF-WORKITEMS.md` (SF-W1 allowlist), `SF-EXIT-CONTRACT.md` §13–§14,
`GATE1-CONTRACT-REVIEW-2.md`, and the accepted P3A alternative-verification
substrate (`P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` +
`P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md`).

## Verdict

```text
P0 = 0
P1 = 0
P2 = 0
Product/Authority: PASS
Operational and Trace Governance: PASS
VERDICT: PASS
```

## §5 acceptance checks (each independently verified)

1. **Bounded supersession.** The Amendment amends only the SF-W1 allowlist
   with four additive-only files. It does not touch SF-W2/SF-W3 allowlists,
   contract schemas, authority boundaries, RED items, journey definitions,
   verification matrix, or acceptance items. `SF-EXIT-CONTRACT.md` and
   ADR-0014 are untouched by the Amendment's operative content.
2. **Strictly additive wiring required by the frozen journey.**
   - Daemon routing: `cmd/loomd/product_daemon.go`'s
     `localProductHandlerWithComposition` is the sole IPC dispatch switch;
     the queue methods cannot be served without additive routing there.
   - TUI wiring: `internal/tui/model.go` owns `screens`, Tab/refresh keys,
     and `View` dispatch; the frozen SF-W1 journey requires the queue state
     "visible in both clients" in the real PTY TUI, which is impossible
     without additive model wiring. The SF-W1 allowlist already owns
     `internal/tui/queue.go`; the model wiring is the missing counterpart.
   - Probe drive: the accepted alternative-verification method drives
     GUI-side actions through `LoomLocalAppContractProbe` over the
     production Swift client; the frozen journey requires queue action
     modes, which are additive to the probe only.
3. **No authority/schema/RED/journey/acceptance change.** The Amendment's
   operative content is limited to owned-file allowlist additions with
   additive-only carve-outs; §4 explicitly denies any other change, and no
   such change appears in the text.
4. **No SF-W4 / thin WorkItem.** The Amendment explicitly attaches to the
   existing and only SF-W1; no new WorkItem or split boundary is created.
   Addendum (same review): `internal/tui/model_test.go` joins item 3 as the
   additive navigation-test counterpart of the `ScreenQueue` wiring; the
   `LocalIPCClient.swift` visibility widening joins item 4 (additive only,
   no behavior or wire change); the review conclusion is unchanged
   (P0=P1=P2=0).

## Conclusion

The Amendment is bounded, additive, production-only, and required for the
frozen SF-W1 journey. It closes the implementability gap between the
accepted allowlist and the accepted journey without expanding authority,
schema, RED, or acceptance scope.

VERDICT: `PASS`
