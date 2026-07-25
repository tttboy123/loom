# S2-EXIT-1 Mandatory RED

Command:

```text
go test ./internal/app ./cmd/loomd \
  -run 'Test(LocalRuntimeObservationDaemon|Run)' -count=1
```

Result: `FAIL`, before any S2-EXIT-1 product file existed.

The compile failures were limited to the frozen missing API and command
symbols:

```text
undefined: NewLocalRuntimeObservationDaemon
undefined: LocalRuntimeObservationDaemonConfig
undefined: LocalRuntimeObservationFact
undefined: app.LocalRuntimeObservationDaemonResult
undefined: daemonRunner
undefined: daemonBuilder
undefined: run
undefined: exitSuccess
```

No pre-existing product error appeared. Product implementation began only
after this RED was recorded.
