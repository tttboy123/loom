# P2A-W2 Live Gate Resumption Attempt 002 Invalid Result

**Date**: 2026-07-30
**Status**: STOPPED — HUMAN_REQUIRED
**Attempt**: `p2a-w2-live-20260730-002`

## Result

The reviewed resumption lineage successfully opened the exact addressable Loom
Team Builder at the user-only MiniMax SecureField. The Controller did not read,
retrieve, type, paste or submit any credential.

The user then personally configured a MiniMax credential and reported success.
The native UI displayed:

```text
MiniMax Verified · brokered
Test
Revoke
```

The user had explicitly stated immediately before hand-off that they intended
to use the credential already pasted into chat. The reviewed resumption
contract prohibits every chat-pasted credential, including that value. This
attempt therefore cannot be accepted as W2 live evidence regardless of whether
the Provider accepted the credential.

The Controller did not reproduce, log, inspect or place the credential into any
artifact.

## Authoritative Event evidence

The closed isolated Journal contains exactly:

```text
RuntimeInstanceDiscovered     = 1
ProviderCredentialConfigured  = 1
ProviderCredentialVerified    = 2
total Events                  = 4
```

The Provider credential stream contains sequence `1` through `3`. There is no
credential-revocation Event.

Two verification facts exist. The Controller did not activate `Test`; the user
controlled the window during hand-off. The external request count is not
independently asserted from these Event facts alone. The reviewed one-Test
journey cannot be proven from this result and must not be claimed.

No TeamDefinition, TeamInstance, AgentInstance, WorkItem, Run, Grant, Evidence,
dispatch or generation Event exists. No Team Candidate was confirmed and no
execution was created.

## Stop and credential state

After the invalid credential condition was detected:

- the Controller did not activate `Test`;
- the Controller did not continue Team Builder;
- the user was asked to personally activate `Revoke`;
- the user explicitly refused revocation and instructed the Controller to
  continue testing;
- the Controller did not override the refusal or mutate the credential;
- the Team Builder was dismissed;
- the Loom window and app were closed through Computer Use;
- the isolated daemon was stopped normally;
- the product socket and attempt processes are absent;
- the resident accepted Runtime observer remains untouched.

Because the UI showed `Verified` and no revocation Event exists, the MiniMax
credential must be treated as still configured in Loom's credential boundary.
The Controller did not inspect Keychain contents.

## Gate

```text
live allowance                 = CONSUMED
P2A-W2 controlled live result  = INVALID / HUMAN_REQUIRED
P2A-W2 acceptance              = NOT ACCEPTED
P2A-W3                          = LOCKED
P2A-W4                          = DOES NOT EXIST
```

The exact attempt root and SQLite remain in place for fresh independent
Result-Evidence Review. Cleanup cannot complete while the user refuses the
required product-level credential revocation.

No further Provider request, Team Builder action, daemon start or app launch is
authorized under this attempt.
