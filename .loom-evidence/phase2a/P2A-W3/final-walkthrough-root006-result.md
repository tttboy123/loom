# P2A-W3 Final Walkthrough root006 Result

**Date**: 2026-08-03  
**Attempt**: `phase2a-w3-walkthrough-20260803-006`  
**Product verdict**: `PASS`  
**Evidence verdict**: independent Result Review pending

The single fresh replacement walkthrough used current locked artifacts, a
private signed native bundle and an exact private copy of accepted Pi-006
103-Event state. Native and TUI read through the real root006 product socket.
No fixture server or alternate state source was used.

## Native product result

Computer Use opened the exact root006 bundle and verified:

- Mission Board: `P2AW3ControlledTeam · Complete · Succeeded`;
- Mission Room: `Mission completed`, one node, one complete, zero in review;
- Team Inspector: `Main · Terminal · Attempt 2`;
- Plan Inspector: `Ship the reviewed release · Succeeded · Attempt 2`;
- Changes Inspector: complete Attempt 1 and Attempt 2 review, acceptance,
  verification and rejection/recovery history, with no incomplete-Timeline or
  unavailable placeholder;
- Evidence Inspector: four distinct visible Evidence rows, two for Attempt 1
  and two for Attempt 2;
- the disabled composer and read-only decision availability remained visible.

Exact screenshots are private mode `0600`:

```text
e94f5535e634e513ced1dadf9b7998a9a551eddc6ba8b8b28062fdf1998030d2  native-board.png
5a977c633f7a89cfd606ae1584d06088db639e8b027fdb462000c5fcfa758e8f  native-mission-team.png
5cdf3f6adbd860e52812700fe2d84671245c7de9d66f4c1244b7f9ed76f4a623  native-mission-plan.png
830c0ba69f5eddd50f726e39f19e9cce9640a0d030a521ac025df02e5f041269  native-mission-changes.png
bb38ee2b7b19d8a6da9fa183d415f2ee16db5d1bd3b0bf09819f21eb6928dab9  native-mission-evidence.png
```

## TUI product result

The exact root006 TUI traversed the real socket without error:

```text
Board
Mission
Team Builder
Runs / History
Compare
Attention
Team Timeline
Team Timeline next bounded page
```

It rendered the completed human-named Mission, four terminal Runs, two selected
Runs in Previous/Current Compare with the human Pi Runtime name and Evidence
counts, one Attention item, and Timeline milestones including Source/Verifier
Evidence, rejection, bounded recovery and succeeded terminal. The next-page
read completed without an error or mutation.

Durable transcript hashes:

```text
b244711465a417eba793e4fedec68c9b39562980f971a240dabf7012c3bb0c08  tui-transcript.raw
cb1a0415d0da22c4cd4b2826f289f75e3dd4abe67ae35091d1ddf9fa45359945  tui-transcript.txt
```

## No-write boundary

No objective text was entered. No preflight, Start, Provider Test, approval,
recovery, cancel, terminal, credential or Provider request was made. The single
daemon start reported exactly:

```json
{"completed_cycles":1,"discovery_events":0,"status_events":0,"no_write_cycles":1,"runtime_facts":[{"runtime_instance_id":"pi-0.82.1-p2a-w3-pi","executable_version":"0.82.1","status":"online","model_ids":["loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m"],"discovery_sequence":1,"status_sequence":0}]}
```

Daemon result SHA-256:
`16e83254742da5fe0bc665dfb61f6eb462f793621de2bb6128293535c0e5fb4d`.

## State and cleanup

- initial and final SQLite SHA-256:
  `3af352562fbc85cb47c9509096655f2cda4a05ddbb999702bc764ab90f77ed56`;
- immutable integrity: `ok`;
- initial and final Event count: `103`;
- product socket and socket lock: absent after stop;
- root006 native, TUI and daemon processes: absent;
- open root006 SQLite handles: absent;
- isolation root: empty;
- evidence files: regular owner-only `0600`;
- evidence and SQLite non-disclosure scans: PASS.

The unrelated `demo-resident` boundary was never targeted by a command,
argument, signal or state path. Its SQLite remains mode `0600`, integrity `ok`,
exactly one Event, and retains its 2026-07-28 modification time. Its preflight
process PID 18814 was no longer present at final cleanup. This walkthrough did
not restart or otherwise mutate that unrelated service; Result Review must
assess this honest liveness note separately from root006 product/state proof.

root006 is consumed and must never be reused.
