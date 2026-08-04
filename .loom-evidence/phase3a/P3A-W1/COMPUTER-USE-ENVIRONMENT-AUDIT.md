# P3A-W1 Computer Use Environment Audit

Date: `2026-08-04`

Status: `EVIDENCE — GUI JOURNEY REQUIRES APP-AUTHENTICATED COMPUTER USE`

## Finding

The mandatory Cross-client Exit Gate requires a real native-window GUI journey
driven through the reviewed Computer Use runtime. In this Codex session the
`node_repl` tool is not exposed. The following were verified read-only:

1. The official Computer Use client
   (`SkyComputerUseClient ... mcp`) exposes the full toolset
   (`list_apps`, `get_app_state`, `click`, `set_value`, `type_text`,
   `press_key`, `scroll`, `select_text`, `perform_secondary_action`).
2. The CUAService is running and its Unix socket
   (`~/Library/Group Containers/2DC432GLL2.com.openai.sky.CUAService/IPC/
   computeruse.sock`) accepts connections.
3. The CUAService enforces sender authentication: an unauthenticated client
   is rejected with `Computer Use server error -10000: Sender process is not
   authenticated`. Authentication is provided by the Codex app through its
   XPC/CodexAuth bridge; it cannot be forged from a shell process.
4. A directly spawned `node_repl` kernel lacks `nodeRepl.nativePipe` /
   `launchServices`, so the `@oai/sky` transport cannot establish its pipe;
   the trusted path requires the app to spawn the kernel.

## Consequence

The real native-window GUI half of every journey scenario must be executed in
a Codex session where the Computer Use runtime is exposed to the agent (or by
the Product Owner through the app's own computer-use surface). The TUI/PTY
half, daemon harness, evidence producers and freeze/verify tooling are ready
and validated. No AppleScript, CGEvent synthesis or accessibility-tree-only
substitute may replace it (Exit Contract §10, runbook, Computer Use skill).

## Unblocking options

1. Re-run the journey execution in a Codex thread/session that exposes the
   `node_repl` Computer Use tool (the app's trusted channel).
2. Or have the Product Owner drive the native-window actions in the app while
   this thread records `gui/actions.jsonl` + screenshots and completes the
   freeze; the runbook documents the exact launch command
   (`--journey-id <UUID>` argument on the root-local signed executable).
