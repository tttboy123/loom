# Final Live Gate Pi RPC Progressive Assistant Identity Verification

- Contract: `PHASE1-FINAL-LIVE-PI-RPC-PROGRESSIVE-IDENTITY-1`
- Baseline: `e32f65035ec7c477bbee9e60cffa7c0989b028ce`
- Verification date: `2026-07-27`
- Live Runtime/model/canary used: `NO`
- Progressive-identity canary authorization consumed: `NO`

## Contract gate

Fresh independent Contract Review returned `PASS` with no Critical or Important
findings. The Reviewer confirmed the locked Pi `0.82.1` absent-to-present
`responseId` diagnosis, the bounded progressive identity state, strict
adversarial matrix, owned files, independent manifest/attempt boundary, and
one-invocation/no-retry accounting.

Two non-blocking notes were retained in the contract review:

- baseline compares a derived semantic identity key, not raw message bytes;
- timestamp representation drift and terminal-first `reasoning` should remain
  explicit fail-closed cases.

No contract behavior or scope changed after review. Only the status line was
synchronized as gates advanced.

## Mandatory RED

After Contract Review `PASS` and before production/harness behavior changed,
the exact focused commands failed:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821ProgressiveAssistantIdentity|ProgressiveAssistantIdentityRejections)$' \
  -count=1

FAIL: TestPiRPCPi0821ProgressiveAssistantIdentity
Execute() error = Pi RPC protocol failed
response=true agent=true turns=1 message=true done=false settled=false

go test ./internal/app \
  -run '^TestFinalLiveProgressiveIdentityCanaryIsolation$' \
  -count=1

FAIL: independent_manifest
current = resolved-live-manifest-additional-canary.json

FAIL: independent_attempt_prefix
current = controlled-canary-additional-*
```

The Runtime RED reproduced the locked live failure boundary: baseline rejected
the first valid progressive assistant update after `message_start`. The app RED
proved the baseline still targeted the consumed additional manifest and
attempt-prefix lineage.

RED output contained no prompt, credential, raw RPC record, raw Grant, hidden
reasoning, private absolute path, or model output. No Runtime, model, Provider,
network, or live canary ran.

## Minimal repair

The bounded Candidate:

- replaces the serialized assistant identity key with one transcript-local
  state;
- binds exact timestamp representation at assistant `message_start`;
- requires `responseId` absent initially, present and bounded at first
  `text_start`, and byte-exactly stable through every later partial and
  terminal;
- rejects all `responseModel` and `cacheWrite1h` presence;
- permits `reasoning` presence to change only once, absent to present, on an
  assistant update, then requires it on every later assistant message;
- retains independent usage-value validation while allowing numeric usage to
  progress;
- preserves exact top-level/nested partial equality, text accumulation,
  content-index, tool/thinking/error/retry/unknown/duplicate/size/Grant
  rejection, terminal semantic equality, and Bridge/Grant sequencing;
- updates the deterministic fixtures to the genuine locked Pi progressive
  shape and adds response-ID, response-model, immutable timestamp, partial
  mismatch, cache-write, and reasoning adversarial cases; and
- routes the opt-in harness to
  `resolved-live-manifest-progressive-identity-canary.json` and
  `controlled-canary-progressive-identity-*`.

The first package union after focused GREEN correctly found one historical test
still asserting that the current manifest must be the consumed additional
manifest. The bounded test repair preserved its original no-alias requirement,
added the additional manifest as a read-only excluded alias, and froze the new
progressive-identity filename. No production behavior changed for that repair.

## Verification matrix

Final focused checks passed:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821ProgressiveAssistantIdentity|ProgressiveAssistantIdentityRejections)$' \
  -count=1
PASS: ok loom-pi-rebuild/internal/runtime/piadapter 5.809s

go test ./internal/app \
  -run '^TestFinalLiveProgressiveIdentityCanaryIsolation$' \
  -count=1
PASS: ok loom-pi-rebuild/internal/app 0.607s
```

The owned-package union passed:

```text
go test ./internal/runtime/piadapter ./internal/app -count=1
PASS:
  internal/runtime/piadapter 22.066s
  internal/app 7.171s
```

Repeated focused race checks passed:

```text
go test -race ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821ProgressiveAssistantIdentity|ProgressiveAssistantIdentityRejections)$' \
  -count=30
PASS: ok loom-pi-rebuild/internal/runtime/piadapter 183.017s

go test -race ./internal/app \
  -run '^TestFinalLiveProgressiveIdentityCanaryIsolation$' \
  -count=30
PASS: ok loom-pi-rebuild/internal/app 1.450s
```

The complete repository passed:

```text
go test ./... -count=1
PASS:
  internal/app 18.189s
  internal/runtime/piadapter 35.630s
  all other packages passed

go test -race ./... -count=1
PASS:
  internal/app 26.670s
  internal/runtime/piadapter 36.694s
  all other packages passed
```

All remaining frozen checks passed:

```text
go vet ./...
PASS: exit 0

go mod verify
PASS: all modules verified

gofmt -d \
  internal/runtime/piadapter/rpc_bridge_adapter.go \
  internal/runtime/piadapter/rpc_bridge_adapter_test.go \
  internal/app/final_live_gate_live_test.go
PASS: empty output

git diff --check
PASS: empty output

GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime/piadapter ./internal/app
PASS: both packages
```

## Scope and preservation audit

The Candidate Go diff is limited to:

- `internal/runtime/piadapter/rpc_bridge_adapter.go`
- `internal/runtime/piadapter/rpc_bridge_adapter_test.go`
- `internal/app/final_live_gate_live_test.go`

This lineage adds only the amendment, contract review, verification, and later
implementation-review evidence. Pre-existing user-owned dirt remains excluded.
No module/dependency, Bridge v1, Journal, Projection, StateWriter, policy,
Grant/Evidence authority, Runtime/model installation, source lock, credential,
`.env`, daemon, scheduler, CLI, or queued product capability changed.

Historical evidence remains exact:

```text
contract.md
9c5e71767f9983bc53089a8c22a03fcf430bd784d5e2fc602b618a9fca9e1dff

contract-review.md
7cbbeb13b4699fee65e0454329306f967bcc16a32e87aca5fe790c43454b22a1

source-lock.json
e542aa319219f486cb45538df53ff49fb03fb180b748b9661bfb2497d58658f5

live-canary.md
2dbd149403f290800503514293a204288bb85c62d7f4c7acb4c409db1a5a1665

replacement-live-canary.md
05434337f83418c9e68e310e1e0281802935711e9cc4eaa5f408641396248273

resolved-live-manifest.json
b1fb029113655aad94e713ca9d5184b0083a847d28f72487e36ef974aa74fe83

source-provenance-amendment.md
a8f9a26573b2ce9db9ccaeb2f315063bbf07d5bd0a0b67754d38ca23d78c7d26

additional-live-canary.md
2559ef9177900910e867cf196266fa2a836dd78f75c1ed37403c4ef183f2a014

resolved-live-manifest-additional-canary.json
222d6004fc06a8afc3e1c9a8369216d9b6f3f97fa116a386c039df27362c3c32

original canary SQLite
914f4d872246cfb124be17136d6c3077c08dd3bb2697f9b517d8ee14e65e76ae

additional canary SQLite
ba1ef877ccd4ebacafe492eeefca88e10602c3133770b201a624ad996a5fb1a3
```

No progressive-identity manifest or private attempt exists. No historical
artifact, manifest, SQLite database, or private attempt was mutated.

The only `LOOM_AGENT_GRANT` match in changed source is the pre-existing fixture
guard that fails if the environment variable leaks into the child process.
No raw Grant value, private-key marker, mirror URL, private materialization
path, credential, hidden reasoning, or model output was added.

## Gate result

The Candidate is ready for fresh independent Implementation Review. Staging,
commit, installed-binding revalidation, manifest creation, and the one
authorized live canary remain blocked until that Reviewer returns `PASS`.

VERDICT: PASS
