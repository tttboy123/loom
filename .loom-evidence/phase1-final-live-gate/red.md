# Behavioral RED

Date: 2026-07-27

Command:

```text
go test ./internal/runtime -run 'Pi0821|PiRuntimeProbeNoModels' -count=1
```

Result: `FAIL` as required.

The current product parser rejected the exact valid Pi 0.82.1 three-line
no-model diagnostic with `ErrInvalidPiMetadataOutput`. The pre-existing
no-model observation path also failed when changed to the exact Pi 0.82.1
fixture. This is a behavioral compatibility failure, not a compile failure.

No external model asset was downloaded or started and no live canary ran.

## Product Repair 2 RED

Date: 2026-07-27

Command:

```text
go test ./internal/runtime ./internal/runtime/piadapter \
  -run 'Pi0821|PiRPCBridge|PiLocalModel' -count=1
```

Result: `FAIL` behaviorally as required.

- `TestPi0821NoModelsDiagnosticCompatibility` proved the current parser
  incorrectly accepted a leading blank physical line.
- `TestPiRPCBridgeTranslatesCorrelatedTranscript` proved the current decoder
  rejected the real Pi 0.82.1 top-level assistant
  `message_start -> message_update(text...) -> message_end` lifecycle because
  it still required an invented nested `start` update.

Both packages compiled and executed their tests. No external model asset was
downloaded or started and no live canary ran.

## Product Repair 3 RED

Date: 2026-07-27

Command:

```text
go test ./internal/runtime/piadapter \
  -run 'TestPiLocalModelServerFailsClosed/health_then_early_exit' -count=1
```

Result: `FAIL` behaviorally as required.

The fake local server returned one exact health response and then exited. The
current product incorrectly returned a non-nil ready server with no error,
directly reproducing Implementation Review 3's concurrency finding.

No external model asset was downloaded or started and no live canary ran.
