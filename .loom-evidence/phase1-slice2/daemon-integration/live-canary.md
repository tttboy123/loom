# S2-EXIT-1 Controlled Live Canary

- Date: `2026-07-25`
- Platform: local macOS foreground process
- Repository branch: `codex/loom-platform-slice2`
- Baseline before Candidate: `39a9e0a`
- Temporary root: `/tmp/loom-s2-exit-canary.NJsL91`
- Real user Pi discovered on ambient `PATH`: no

This canary used the real compiled `cmd/loomd`, real clock/timer, real isolated
Pi metadata process runner, real SQLite Journal, accepted StateWriters, and real
projection replay. The executable named `pi` was the deterministic fixture at
`.loom-evidence/phase1-slice2/daemon-integration/fixture/pi`.

It proves the daemon integration and lifecycle. It does **not** prove that a
real user Pi installation is present or ready.

## Build

```text
go build -o /tmp/loom-s2-exit-canary.NJsL91/bin/loomd ./cmd/loomd
go build -o /tmp/loom-s2-exit-canary.NJsL91/bin/loom ./cmd/loom
```

```text
1cfbd4d091de8816ab92d13288e730f450a47b6514658fbf1563e64c24e97011  loomd
c920f22c90bbc2516633bdaab4a6b544ff5011a646e716727601ea328f1507e4  loom
```

## First discovery

Fixture mode: `stable`.

```json
{"completed_cycles":1,"discovery_events":1,"status_events":0,"no_write_cycles":0,"runtime_facts":[{"runtime_instance_id":"pi-local-1","executable_version":"1.0.0","status":"online","model_ids":["provider/model-a"],"discovery_sequence":1,"status_sequence":0}]}
```

SQLite:

```text
1|RuntimeInstanceDiscovered|1.0.0|provider/model-a
```

## Restart with unchanged metadata

The first daemon closed. A new compiled daemon opened the same state.

```json
{"completed_cycles":1,"discovery_events":0,"status_events":0,"no_write_cycles":1,"runtime_facts":[{"runtime_instance_id":"pi-local-1","executable_version":"1.0.0","status":"online","model_ids":["provider/model-a"],"discovery_sequence":1,"status_sequence":0}]}
```

SQLite remained exactly one Event with maximum sequence 1:

```text
1|1
```

## Restart with inventory change

Fixture mode changed to `inventory` without changing the bound executable.

```json
{"completed_cycles":1,"discovery_events":1,"status_events":0,"no_write_cycles":0,"runtime_facts":[{"runtime_instance_id":"pi-local-1","executable_version":"2.0.0","status":"online","model_ids":["provider/model-b"],"discovery_sequence":2,"status_sequence":0}]}
```

SQLite:

```text
1|RuntimeInstanceDiscovered|1.0.0|provider/model-a
2|RuntimeInstanceDiscovered|2.0.0|provider/model-b
```

## Forced metadata-process failure and recovery

Fixture mode `fail` made the metadata child exit unsuccessfully. `loomd`
returned only the stable non-disclosing message and exit class:

```text
daemon failed
exit=4
```

The Journal remained exactly two Events with maximum sequence 2. The isolation
root contained no invocation residue.

After restoring fixture mode `inventory`, a new daemon rebuilt the same
projection and correctly selected no write:

```json
{"completed_cycles":1,"discovery_events":0,"status_events":0,"no_write_cycles":1,"runtime_facts":[{"runtime_instance_id":"pi-local-1","executable_version":"2.0.0","status":"online","model_ids":["provider/model-b"],"discovery_sequence":2,"status_sequence":0}]}
```

SQLite remained:

```text
2|2
```

## Signal-canceled recurrence

An unbounded foreground daemon used a real 50ms timer. After 300ms it received
`SIGTERM`.

```json
{"completed_cycles":5,"discovery_events":0,"status_events":0,"no_write_cycles":5,"runtime_facts":[{"runtime_instance_id":"pi-local-1","executable_version":"2.0.0","status":"online","model_ids":["provider/model-b"],"discovery_sequence":2,"status_sequence":0}]}
```

```text
signal_exit=0
2|2
orphan=no
```

No `loomd` or `loom-pi-metadata-*` child remained, and the isolation root was
empty.

## Permissions and final authority

```text
700 state/
700 isolation/
600 state/loom.db
600 state/loom.db.lock
```

Final authoritative Journal facts:

```text
1|RuntimeInstanceDiscovered|1.0.0|provider/model-a
2|RuntimeInstanceDiscovered|2.0.0|provider/model-b
```

The final JSON projection facts select sequence 2, version `2.0.0`, and model
`provider/model-b`, matching the Journal.

No Agent session, model call, Provider/network request, credential access,
user Pi state, service installation, launch-at-login item, or resident daemon
was created.

VERDICT: PASS
