# Attempt 023 Startup Diagnostic

The first daemon invocation used a new probe identity against a byte-for-byte
copy of the Attempt 022 state. It failed closed with `observer_inventory`
before the product socket existed and before structured request logging began.
No app was launched, no IPC request was served, and no Journal fact changed.

SQLite then provided the already-authoritative observer identity:

- probe: `probe.pi.phase2c-attempt022`
- runtime: `runtime.pi.phase2c-attempt022`
- device: `device.phase2c-attempt022`
- display name: `Pi 0.82.1 Attempt 022`

The replacement daemon reused those exact values. It opened the private test
socket and rebuilt view version
`b099ed9db9fbab64bac82c28f57423dfe7a60c447ea07edd6ff20e066aac9a5f`.
This was a controlled harness-identity correction, not a product repair or an
authority transition.

Postflight proves the replacement database and chat-thread file remain
byte-identical to their Attempt 022 sources. SQLite reports `ok` and
`168/168/168` Event, Event-ID, and idempotency-key counts.
