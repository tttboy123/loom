# P2A-W3 Native Launcher Live Invocation Transaction Amendment Review

**Date**: 2026-08-02  
**Reviewer**: fresh independent read-only Contract Amendment Reviewer  
**Amendment SHA-256**:
`f1ab059b5cafd89a56267c9211794a84633827e1f2857069a9720504ddeb6a42`  
**Verdict**: `PASS`

## Findings

```text
P0: none
P1: none
P2: none
```

The amendment truthfully binds Pi-004's Controller-only cause: one Pi binary
invocation exited `2` with stderr `invalid input`, while no
daemon/socket/Pi/local-model/preflight/Start/Provider activity occurred and
SQLite remained byte-identical. It does not misclassify that evidence as a Loom
product failure.

The argv guard is sufficient for the known failure. It freezes exact executable
identity plus the ordered 32-token array, canonical `{executable,args}` digest,
first element `--state`, 16 flags plus 16 values, exactly two
`--runtime-dir` occurrences and one of every other flag. It forbids positional
arguments, `daemon`, wrapper-added tokens, PATH lookup and free-form command
reconstruction. Any mismatch or failure consumes the replacement and stops.

The one replacement allowance is bounded and governance-sound: it does not
reuse Pi-004, reopen MiniMax, change product source/authority, or create
P2A-W4. It remains conditional on source/external identity revalidation and a
fresh manifest/root. This Review itself started no live action.

**FINAL VERDICT**: `PASS / ONE FRESH PI REPLACEMENT MAY BE PREFLIGHTED`
