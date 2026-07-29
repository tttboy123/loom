# P2A-W1 Replacement Result-Evidence Repair 1

**Date**: 2026-07-28
**Scope**: evidence-only; no product or live action

## Correction

The previously recorded
`5c7e59572c7a79ba1a30bdf9f5f187cea276cbd1ca69a0a7383e150b23417193`
is the SHA-256 of a pipe-delimited SQLite query row. It remained the same
before and after both gates, but it is not the canonical `GlobalReadView`
stream-head digest and must not be labeled as such.

The immutable database remains:

- SHA-256
  `91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4`;
- `integrity_check=ok`;
- one Event;
- one head:
  `runtime_instance:runtime.pi.earendil-works.0.82.1`, sequence `1`, Event ID
  `280fd3c09106c45a7c5b06920fe80645`.

Reproducing `globalReadViewVersion` exactly as sorted
`stream_id + NUL + decimal_sequence + NUL + event_id + newline` gives:

```text
6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05
```

Both live evidence files now use that value for the canonical digest and
retain this disclosure of the earlier method error.

## Preserved result

This repair did not:

- bootout, bootstrap, signal, or inspect a credential value;
- install or run the Candidate;
- open the TUI or use Computer Use;
- change SQLite, Event Journal, installed files, sockets, credentials, or
  Provider state;
- stage or commit;
- create a third live allowance.

The replacement remains `FAIL — ROLLED_BACK — HUMAN_REQUIRED`.
