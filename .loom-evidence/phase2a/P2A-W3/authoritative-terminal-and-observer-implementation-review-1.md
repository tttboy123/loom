# P2A-W3 Authoritative Terminal and Observer Implementation Review 1

**Date**: 2026-08-02  
**Reviewed source-lock SHA-256**:
`c63641f0d5204113cca074e40308a136b65477f0760dc1f140178893bea5a195`  
**Reviewer**: fresh independent read-only Implementation Reviewer  
**Verdict**: `FAIL`

## Findings

```text
P0: none
P1: 2
P2: none
```

1. The classifier allowed an unknown leaf to be encapsulated for factory,
   projection, identity-metadata and write reasons. This contradicted the
   contract's unconditional known-plus-unknown `observer_unknown` rule, and the
   tests encoded the same incorrect exception.
2. The copied-state test synthesized one equivalent Runtime event in a fresh
   database. It did not copy the retained six-fact live SQLite or independently
   prove saved-Team authority immutability across the success and failure
   matrices.

The Reviewer found no blocking issue in the credential Broker or real Swift UDS
closure. It verified the reviewed lock and all locked-authority hashes before
returning `FAIL`. It performed no live, Provider, Secret Store, Pi, native,
daemon, staging, commit, or file-write action.

Repair is required inside the same unique W3 contract. The reviewed source lock
is superseded and cannot authorize live activity.
