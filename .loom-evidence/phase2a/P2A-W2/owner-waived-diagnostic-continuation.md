# P2A-W2 Owner-waived Diagnostic Continuation

**Date**: 2026-07-30
**Status**: REVIEWED — bounded read-only Review PASS
**Product Owner direction**:
`你不要再纠结这个问题，我要你正常跑完测试，忽略这个Gate`

## Purpose

The Product Owner explicitly directs Loom development to continue product
testing with the currently configured MiniMax credential despite the prior
chat-pasted-credential stop condition.

This document records a diagnostic exception; it does not rewrite historical
facts or claim that the original secret-negative acceptance requirement was
met.

## Permitted diagnostic

Using the existing attempt-002 binaries, SQLite, isolation root and Keychain
credential, the Controller may:

1. start the isolated product daemon once;
2. launch the exact addressable Loom bundle once;
3. open Team Builder;
4. activate `Test` once against the already configured MiniMax credential;
5. continue the visible one-question-at-a-time Team Candidate journey as far as
   the product permits;
6. explicitly confirm one Candidate only if the UI exposes a complete
   compatible preview;
7. inspect Event types, UI states, process/socket permissions and
   TeamDefinition-only authority;
8. stop the app and daemon and record the exact result.

The credential value must not be read from chat, Keychain, process state,
SQLite payloads or environment; it must not be repeated in source, evidence,
logs, prompts, screenshots, Team definitions or output. The product may obtain
it only through its existing Credential Broker and OS Secret Store boundary.

## Result semantics

- The diagnostic may prove whether the current product journey works.
- It may expose product defects and may authorize an in-scope W2 repair.
- It does not erase the prior invalid attempt facts.
- It does not by itself prove the original secret-negative criterion.
- It does not automatically accept W2 or unlock W3.
- No TeamInstance, AgentInstance, WorkItem, Run, Grant, Evidence, dispatch or
  generation action is permitted.
- Any credential display, additional unexpected Provider action, authority
  expansion or execution creation stops immediately.
