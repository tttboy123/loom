# Phase 2C Replacement Journey Attempt 020 - Failed

Attempt 020 is preserved and must not be promoted.

The Repair 18 source-locked binaries used main Journey UUID
`559cf809-344f-49d8-b712-c3f292a47dc4`. J1-J4 passed the TUI authority
boundary: ordinary chat and the complete Team Draft created no Team, Mission,
Run, or Evidence facts; explicit Team confirmation created one confirmed Team
and no work. J5 proved preflight remained non-authoritative and explicit Start
created the Mission and Runs.

The Mission then exhausted two attempts with `runtime_process_failed` and
became `blocked`, so the attempt cannot verify the required human-review restart
path and is not a clean J1-J10 run. Read-only process inspection found an older
orphaned `llama-server` listening on the product's fixed loopback port `18427`.
It began about 2 hours 40 minutes before Attempt 020, had the repository as its
working directory, and was not a child of the resident demo daemon. Both Run
captures contained zero frames, consistent with failure before RPC exchange.

After preserving this attempt, the Attempt 020 TUI and daemon were stopped
cleanly. The identified stale Journey model process received `SIGTERM`, exited,
and released port `18427`. The watchdog-managed resident demo daemon remained
running and was not modified. The separate decision fixture was never started.
J6-J10 were not run and no Phase or ADR acceptance claim is made.
