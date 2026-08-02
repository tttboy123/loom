# P2A-W3 Final Walkthrough root004 Result

Date: 2026-08-03

Verdict: `DIAGNOSTIC FAIL`

The fresh root004 native-window walkthrough passed the safe-name boundary on
Mission Board, New Mission, Teams, Needs You, Library, Runtime & Providers,
Mission result, and the Evidence Inspector selector. The Go TUI then passed
Board, New Mission, Team Builder, Runs / History, Compare, and Attention, but
opening the selected completed Mission returned:

```text
State unavailable · state_unavailable
```

No navigation continued after that failure. The TUI, native app, and isolated
daemon were stopped. No objective, preflight, Start, Provider Test, approval,
recovery, cancel, terminal, or other product mutation was requested.

## Preserved state

- root: `/Users/lune/Library/Application Support/Loom/phase2a-w3-walkthrough-20260803-004`
- final SQLite SHA-256:
  `3af352562fbc85cb47c9509096655f2cda4a05ddbb999702bc764ab90f77ed56`
- `PRAGMA integrity_check`: `ok`
- final Event count: `103`
- daemon stderr: empty
- daemon stdout SHA-256:
  `16e83254742da5fe0bc665dfb61f6eb462f793621de2bb6128293535c0e5fb4d`
- product socket and lock: absent after stop
- isolated root processes and open SQLite handles: absent after stop
- all screenshots and the TUI transcript: owner-only `0600`

The copied SQLite is byte-identical to the accepted Pi-006 final state. root004
is diagnostic evidence only and is never reusable as a replacement walkthrough.

## Native screenshot hashes

```text
e94f5535e634e513ced1dadf9b7998a9a551eddc6ba8b8b28062fdf1998030d2  native-board.jpeg
f9e60146a63c9c1528f220f2d67df410cb19134a9075fa7a809160f5b02d4c93  native-new-mission.jpeg
2e52a31ae2323ba1c84cce607d33220a439b6545f66a1c30f9ef49a151f2561f  native-teams.jpeg
9b62975cf80acf80657a9cdad17fc061e7afc46674ff06e1b0cad4507fe0844b  native-needs-you.jpeg
a8acea5359c0aaf7bb3aa1b924cb099cfe8881fd9777a50119c6331d045dd864  native-library.jpeg
723c4fc35bcf4906cd4a2d03df29cf844c223aebab40f64c766a5a50238e116f  native-runtime-providers.jpeg
f22dfd2894e90dc7407073de52f137614c873c1c7386e705e7ec412dbac7ea78  native-mission-result.jpeg
f95fd34f1e9034735368b9b8ff1ed0cacbbedfeb7c2b2e44f8741c4686496d8f  native-evidence-inspector.jpeg
77d0326338f8a3a723c76305b01aa8441763862806715a3c4bca8040f32bc8d7  tui-transcript.txt
```

