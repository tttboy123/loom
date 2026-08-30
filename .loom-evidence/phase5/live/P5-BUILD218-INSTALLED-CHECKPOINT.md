# P5 Build 218 installed checkpoint

Status: `INSTALLED PARTIAL / CONVERSATION STARTUP ACCEPTED / RESTART CONTINUITY ACCEPTED / PHASE 5 OPEN`

## Installed identity

- product: Loom `0.5.3` Build 218;
- App SHA-256: `3752d255afb28df5e7d2d99899a9b70612e7f19190f000301371d081940e0c15`;
- bundled daemon SHA-256: `585c2e72f4cb4ecb6815d68d4016373bd6a315bada6fe5fa19fd23333ed4830f`;
- strict deep bundle signature verification: passed;
- managed App/daemon parent relationship: passed;
- credential runtime: Loom Vault, with zero credential-helper spawn attempts.

## Source verification

- complete macOS suite: 407 XCTest cases, two conditional skips, zero
  failures;
- Swift Testing: 20 contract cases, zero failures;
- complete Go repository and `go vet ./...`: passed after the final
  initialization-reason correction;
- focused deferred-runtime and RoundTable initialization race tests: passed;
- the Pi cancellation lifecycle test exposed and closed a test-only marker
  scheduling race, then passed 25 repeated runs and five Race runs.

## Cold-start behavior

Build 218 moves Agent Runtime construction behind the local product service
boundary. Conversation setup, setup projection and local IPC no longer wait for
all Agent runtime probes before becoming available. The deferred runtime slot
fails closed while initialization is in progress and returns a retryable,
stage-specific `agent_runtime_initialization` result to governed surfaces.

Three installed cold starts observed private socket readiness at approximately
5.2, 5.7 and 6.4 seconds. The final accessibility-timed cold run measured:

- first App window: 0.120 seconds;
- private daemon socket: 6.360 seconds;
- conversation Route and Model controls: 9.480 seconds;
- Agent Runtime completion diagnostic: 40.604 seconds observed by the probe,
  with 34,527 milliseconds recorded by the daemon;
- exact Mission-linked concluded RoundTable restoration: 41.349 seconds;
- user-visible `invalid_request` or `RoundTable invalid` state: not observed.

The Route and Model controls therefore became available more than thirty
seconds before the governed Agent Runtime and restored RoundTable. Socket
readiness is not used as a substitute for user-visible conversation readiness.

The completion event used Incident ID
`loom-agent-runtime-3a24b80e-c5b4-490f-95c5-ae6ce73317af` and composition
snapshot digest
`56c62fe079986178b59a046c254d11394267e275fa2723d31316aca6748a60ad`.
It contains no Prompt, conversation body, Provider response or credential.

One content-negative `agent_attempt_reconcile / provider_outcome_uncertain`
diagnostic was emitted for a previously nonterminal Attempt during restart.
It remained seat-local, did not replace the concluded restored Session and did
not produce a RoundTable error in the UI. This is distinct from the rejected
Build 217 placeholder-reason regression.

## Restart continuity and visual evidence

Build 217 was an intermediate local installation only. Visual review caught a
red `RoundTable invalid_request` state caused by the deferred placeholder
overwriting the initialization reason. Build 218 explicitly preserves the
initialization reason and maps it to a neutral, automatically retried
preparation state.

Build 218 then restored the exact authoritative Mission-linked RoundTable with
two Agent results and frozen routes after a full App and daemon restart. No
untrusted cached RoundTable projection supplied the restored content.

Screenshots:

- `P5-BUILD218-AGENT-RUNTIME-PREPARING.jpeg`
  - SHA-256: `040b2ec2293911232545d7808a3091de0c985c24bd1fe68266458847a8f98dca`
- `P5-BUILD218-RESTORED-ROUNDTABLE.jpeg`
  - SHA-256: `4e5a150b4de38eea4a53aa216277289abb2f52f7d496df989c03c07b309fb147`

## Open acceptance gates

- compact installed rendering and text-size stress remain open;
- complete keyboard and VoiceOver click-path acceptance remains open;
- installed Mission and RoundTable error-intervention paths remain open;
- a new real Mission/RoundTable workflow remains open;
- repeated cold-start distribution and further Agent Runtime probe performance
  work remain useful, but no longer block ordinary conversation startup;
- no credential was changed and no paid Provider call was started for this
  checkpoint.

Build 218 accepts the first installed conversation-startup optimization and
exact governed-context restart continuity. It is not Phase 5 completion.
