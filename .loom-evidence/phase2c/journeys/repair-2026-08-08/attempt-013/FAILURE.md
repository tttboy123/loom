# Phase 2C Replacement Journey Attempt 013 - Failed

Attempt 013 is preserved and must not be promoted.

Repair 13 passed its replacement source lock, complete deterministic matrix,
fresh signed Release, and a direct AX smoke with zero unlabeled enabled product
actions. The clean Journey then passed J1 ordinary no-folder chat and selected
the J2 folder without reading its sentinel.

The first folder-scoped chat immediately after daemon restart exposed a timeout
ownership race. The daemon received `chat_message` at monotonic offset
`127270110` microseconds and emitted its bounded safe response at `137336890`,
an elapsed `10.066780s`. The TUI client deadline was exactly `10s`, equal to the
server's bounded extended request deadline, so the client rendered
`State unavailable · timeout` before the safe tentative runtime-unavailable
message arrived and was persisted. This violates the Repair 5 no-raw-timeout
intent and J2/J3 continuity. The folder sentinel was not read and no Team,
Mission, Run, approval, or other execution authority was created.

Repair 14 must retain the server's 10-second bounded work deadline while giving
Go and Swift clients a 15-second long-operation receive window. Equal client and
server long deadlines must fail configuration. A fresh source review, lock,
matrix, signed Release, and clean J1-J10 journey are required.

Temporary root remains read-only evidence:

- `/private/tmp/loom-phase2c-chat-20260808-013`

