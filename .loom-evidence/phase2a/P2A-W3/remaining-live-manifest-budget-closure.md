# P2A-W3 Remaining Live Manifest Budget Closure

**Date**: 2026-08-02  
**Scope**: unconsumed MiniMax and Pi manifests only  
**Product source/authority change**: none

The Codex result review permits the two separate unconsumed lineages to proceed
after completing their previously underspecified invocation budgets. This is a
pre-consumption manifest clarification, not a Codex retry, replacement canary,
product Amendment or P2A-W4.

Prior frozen attempt-copy hashes are retained:

```text
MiniMax f42d2d8ce4af4e1bee4c96548723cf34666b783f43bc431ce4b71fe362f109ef
Pi      34506c7e2e18ec9f7eb58ccfd2cbdc8fb6015f1ddc110aafa57ffdd50b5a68bd
```

Before either launch, its prior bytes are retained as
`manifest/live-manifest.initial.json`, and the active manifest adds exactly:

```text
process_timeout_seconds = 30
```

Thirty seconds is bounded by the accepted Pi metadata process maximum. Every
other source, artifact, external identity, claim, deadline, one-shot and
no-retry field remains unchanged. The active manifest SHA is frozen again before
the corresponding daemon's first and only start.
