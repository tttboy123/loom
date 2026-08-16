# Phase 2C Repair 11 Replacement Journey - Attempt 010

**Verdict**: FAIL - harness configuration corrected in a new attempt  
**Journey ID**: `7ad4c563-b396-40dd-92ba-39a34b1a9bf7`  
**Decision subfixture**: `767b32c8-8a2b-4a90-8de9-fcda523ff9f5`  
**Source lock**: `f9bc2e977ec72488b8fa382ed0900a80f90000508f722714b5cb120380450948`

Attempt 010 used a fresh private root and explicitly supplied the main Journey
UUID to the TUI. J1 ordinary chat, J2 folder-display isolation, and J3
offline/reconnect recovery passed without Team or Mission authority.

At J4, the harness omitted the already-verified local-model private root,
executable, and model flags from daemon startup. Runtime discovery was online
but exposed no compatible model, so `builder_start` correctly failed closed with
`incompatible`. This is a harness configuration error, not a product defect.

The TUI and daemon were cleanly stopped. No source changed, no partial result is
promoted, and this state database will not be reused. A fresh Attempt 011 must
start from J1 with the complete frozen runtime configuration.
