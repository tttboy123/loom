# Final Live Gate Pi RPC Rejection Reason-Code Instrumentation Verification

- Date: `2026-07-27`
- Amendment: `PHASE1-FINAL-LIVE-PI-RPC-REJECTION-REASON-CODE-1`
- Baseline: `a4e6f73cdbcf2dcdc986a5c04eadf9b5c32360ca`
- Contract Review: `PASS`
- Live Runtime/model/canary during implementation: `NO`

## Mandatory RED

After Contract Reviewer `PASS`, tests were added before production or harness
changes.

Command:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(RejectionReasonCodes|RejectionReasonCodeNonDisclosure|RejectionReasonCodePrecedence)$' \
  -count=1
```

Result: genuine `FAIL`, exit `1`.

All fifteen frozen rejection cases still returned only:

```text
Pi RPC protocol failed
response=true agent=true turns=1 message=true done=<boolean> settled=false
```

They lacked the required bounded `phase/event/reason` triple. The precedence
test also failed for that exact missing behavior. The test compiled and reached
the intended assertions; there was no fixture, environment, or build failure.

Command:

```text
go test ./internal/app \
  -run '^TestFinalLiveRejectionDiagnosticCanaryIsolation$' \
  -count=1
```

Result: genuine `FAIL`, exit `1`.

The test proved the baseline still selected:

```text
resolved-live-manifest-progressive-identity-canary.json
controlled-canary-progressive-identity-*
```

instead of the new diagnostic manifest and attempt prefix.

RED output contained no private absolute path, prompt, credential, raw Grant,
raw RPC record, hidden reasoning, or model output.

## Focused GREEN

After the minimal implementation:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(RejectionReasonCodes|RejectionReasonCodeNonDisclosure|RejectionReasonCodePrecedence)$' \
  -count=1
```

Result:

```text
ok  	loom-pi-rebuild/internal/runtime/piadapter	9.032s
```

```text
go test ./internal/app \
  -run '^TestFinalLiveRejectionDiagnosticCanaryIsolation$' \
  -count=1
```

Result:

```text
ok  	loom-pi-rebuild/internal/app	1.307s
```

After adding explicit assistant-schema, event-shape, and FrameSink
non-disclosure coverage, the changed adapter focused test was rerun:

```text
ok  	loom-pi-rebuild/internal/runtime/piadapter	5.608s
```

## Impact checks

```text
go test ./internal/runtime/piadapter ./internal/app -count=1
```

Result:

```text
ok  	loom-pi-rebuild/internal/runtime/piadapter	25.965s
ok  	loom-pi-rebuild/internal/app	7.154s
```

## Focused race

```text
go test -race ./internal/runtime/piadapter \
  -run '^TestPiRPC(RejectionReasonCodes|RejectionReasonCodeNonDisclosure|RejectionReasonCodePrecedence)$' \
  -count=30
```

Result:

```text
ok  	loom-pi-rebuild/internal/runtime/piadapter	138.442s
```

```text
go test -race ./internal/app \
  -run '^TestFinalLiveRejectionDiagnosticCanaryIsolation$' \
  -count=30
```

Result:

```text
ok  	loom-pi-rebuild/internal/app	1.659s
```

## Repository and race checks

```text
go test ./... -count=1
```

Result: exit `0`; all `21` listed packages passed or reported no test files.
Relevant affected-package lines:

```text
ok  	loom-pi-rebuild/internal/app	17.250s
ok  	loom-pi-rebuild/internal/runtime/piadapter	44.901s
```

```text
go test -race ./... -count=1
```

Result: exit `0`; all `21` listed packages passed or reported no test files.
Relevant affected-package lines:

```text
ok  	loom-pi-rebuild/internal/app	27.747s
ok  	loom-pi-rebuild/internal/runtime/piadapter	42.166s
```

## Static, module, format, and cross-platform checks

```text
go vet ./...
```

Result: exit `0`, no output.

```text
go mod verify
```

Result:

```text
all modules verified
```

```text
GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime/piadapter ./internal/app
```

Result:

```text
ok  	loom-pi-rebuild/internal/runtime/piadapter	0.003s
ok  	loom-pi-rebuild/internal/app	0.003s
```

Both `gofmt -d` over the three owned Go files and `git diff --check` exited
`0` with no output.

## Scope and acceptance-equivalence evidence

The implementation lineage changes exactly these product/test files:

```text
internal/runtime/piadapter/rpc_bridge_adapter.go
internal/runtime/piadapter/rpc_bridge_adapter_test.go
internal/app/final_live_gate_live_test.go
```

It adds no module, dependency, protocol, Journal, Projection, StateWriter,
Supervisor, authorization, Evidence-authority, daemon, scheduler, CLI, Runtime,
model, source-lock, credential, or environment-file change. Direct diff checks
over those excluded paths were empty.

The existing Pi RPC success, progressive identity, strict rejection, output
size, cancellation, cleanup, and non-disclosure tests all passed in the
affected-package and repository matrices. Every prior rejection remains an
`ErrPiRPCProtocol` rejection; the new error adds only a closed enum triple.
FrameSink diagnostic coverage also proves that arbitrary downstream error text
is not propagated.

The opt-in final-live test remained skipped during ordinary matrices. No live
Pi, llama.cpp, model, or Runtime process was started. Before Review:

- no listener existed on port `18427`;
- no Pi, llama-server, or Loom daemon process was resident;
- no diagnostic manifest existed; and
- no `controlled-canary-rejection-diagnostic-*` attempt existed.

## Historical and installed binding preservation

The following hashes remained exact:

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

additional-live-canary.md
2559ef9177900910e867cf196266fa2a836dd78f75c1ed37403c4ef183f2a014

progressive-identity-live-canary.md
5a154572c00de938cf81a31134c6a5d1c225ce8cb917be735cff787955cc59a7

resolved-live-manifest.json
b1fb029113655aad94e713ca9d5184b0083a847d28f72487e36ef974aa74fe83

resolved-live-manifest-additional-canary.json
222d6004fc06a8afc3e1c9a8369216d9b6f3f97fa116a386c039df27362c3c32

resolved-live-manifest-progressive-identity-canary.json
da04a07f47e84e278b29a841f9495849ffd923f27fe00acd4f8fc2d84e83f16c

source-provenance-amendment.md
a8f9a26573b2ce9db9ccaeb2f315063bbf07d5bd0a0b67754d38ca23d78c7d26
```

Installed/runtime bindings remained:

```text
Pi CLI
af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca

llama-server
a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b

model
cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046
```

All seven locked installed Pi source hashes matched `source-lock.json`.
Original, additional, and progressive canary SQLite hashes remained:

```text
914f4d872246cfb124be17136d6c3077c08dd3bb2697f9b517d8ee14e65e76ae
ba1ef877ccd4ebacafe492eeefca88e10602c3133770b201a624ad996a5fb1a3
9f6ff64d8ceaf09dc7bf4b4747f141cffc942a9a4bd5113d5302789b58422a7f
```

No user-owned dirty file was edited by this lineage. At completion of the
verification matrix, nothing was staged, committed, pushed, merged, or
activated.

VERDICT: PASS
