# P2A-W3 Final Walkthrough root005 Result

Date: 2026-08-03

Verdict: `PASS`

The wholly fresh root005 replacement used current locked source, a private
signed native bundle and an exact copy of the accepted Pi-006 103-Event state.
The native app and Go TUI were operated through their real private product
socket. No fixture IPC server or alternate state source was used.

## Native walkthrough

Computer Use verified these ordinary product surfaces:

- Mission Board: one human-named completed Mission;
- New Mission: Coding/Knowledge, human Team selection and an empty objective;
- Teams: confirmed human Team name and availability;
- Needs You: verification failure attention;
- Library: human Runtime names, History and Compare;
- Runtime & Providers: Codex available, MiniMax unconfigured, no secret shown;
- completed Mission Room: human Mission, Plan, activity and terminal state;
- Inspector Team: `Main · Terminal · Attempt 2`;
- Inspector Plan: `Ship the reviewed release · Succeeded · Attempt 2`;
- Inspector Changes and Evidence: distinct sections that explicitly report
  complete authoritative activity unavailable because the bounded Timeline
  still has another page.

The last two surfaces prove Repair 6's fail-closed behavior on the real state:
they do not present partial history as empty or complete, reuse Team content,
or expose an internal ID/digest.

## TUI walkthrough

The current Go TUI opened the selected completed Mission successfully. The
root004 `State unavailable · state_unavailable` failure did not recur. It
rendered human-safe Mission detail and Timeline milestones, then traversed:

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

Runs selected two local records and Compare displayed Previous/Current with
the human Runtime name and Evidence counts. Team Timeline rendered the first
bounded page and a read-only next-page request completed without an error.

## No-terminal boundary

No objective text was entered. No preflight, Start, Provider Test, approval,
recovery, cancel, credential, terminal or other authoritative mutation was
requested. The daemon completed one unchanged observation cycle:

```json
{"completed_cycles":1,"discovery_events":0,"status_events":0,"no_write_cycles":1,"runtime_facts":[{"runtime_instance_id":"pi-0.82.1-p2a-w3-pi","executable_version":"0.82.1","status":"online","model_ids":["loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m"],"discovery_sequence":1,"status_sequence":0}]}
```

## Preserved state and cleanup

- root:
  `/Users/lune/Library/Application Support/Loom/phase2a-w3-walkthrough-20260803-005`
- initial and final SQLite SHA-256:
  `3af352562fbc85cb47c9509096655f2cda4a05ddbb999702bc764ab90f77ed56`
- SQLite integrity: `ok`
- initial and final Event count: `103`
- product socket and socket lock: absent after stop
- root005 app, TUI and daemon processes: absent after stop
- open SQLite handles: absent after stop
- isolation root: empty
- unrelated `demo-resident` daemon: still running on its own boundary
- all screenshot evidence: regular owner-only `0600`

## Native screenshot hashes

```text
e94f5535e634e513ced1dadf9b7998a9a551eddc6ba8b8b28062fdf1998030d2  native-board.png
f9e60146a63c9c1528f220f2d67df410cb19134a9075fa7a809160f5b02d4c93  native-new-mission.png
baa84fe70a750e95216cc272eea7ce21d1d7f22c916b0ad0fcdcdb1115ed69ed  native-teams.png
3623f06d555db67646f8f529b627d91588812b127dbe6885836db6e4faefd80e  native-needs-you.png
eefa4d61e9e49be25db99ea77c7f37077037a4148c5bed132186148b5dca3f34  native-library.png
723c4fc35bcf4906cd4a2d03df29cf844c223aebab40f64c766a5a50238e116f  native-runtime-providers.png
34c3530c067f5fc4eed67363d1ff10f7ba95af93002ebb70b8d7dcd341990d7b  native-mission-team.png
82589121e8e32b388644a7d962acafc9436346dd4daaa54c59e7813117b16ab7  native-mission-plan.png
83ca069dbd68961a4c2a23995c828069f79cc72d4083158c49f4de6e00385a6f  native-mission-changes.png
3620ac9853a3b4d9f1143eb81ee64e098e0e0a6c5c14cc8fa441807927028b94  native-mission-evidence.png
```

root005 is consumed acceptance evidence and must never be reused for another
walkthrough or live action.
