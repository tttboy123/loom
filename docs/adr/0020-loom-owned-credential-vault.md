# ADR-0020: Loom-owned Credential Vault

**Date**: 2026-08-10
**Status**: accepted
**Deciders**: Product Owner, Loom Architecture Controller

## Context

The installed credential path currently launches a process-attested Keychain
helper for every secret read. That repaired the immediate installation failure,
but it makes normal conversation and Agent dispatch depend on an OS credential
round trip and a new helper process. Phase 2D needs automatic App startup,
multi-turn conversation, per-Agent Provider Accounts, and exact revision
isolation without placing secrets in Swift, prompts, Journal, Evidence,
diagnostics, argv, or ambient environment.

## Decision

1. Loom owns a versioned Credential Vault. `loomd` is the only process that
   opens it, retains the Vault Master Key (VMK), and decrypts credential bytes.
2. Existing `credentials.SecretStore`, opaque `CredentialReference`,
   `ProviderAccountID`, `CredentialRevision`, and `FrozenExecutionBinding`
   contracts remain. The Vault replaces `ProductKeychainStore` beneath them.
3. The Phase 2D default is `LocalKeyFile`: an owner-only, separately stored
   random 256-bit VMK is loaded once when `loomd` starts. Passphrase and
   External Secret Provider modes implement the same `VaultKeyProvider`
   interface in later slices.
4. Each credential revision receives a random 256-bit DEK. AES-256-GCM encrypts
   the secret; a KEK derived from the VMK by HKDF-SHA256 with domain
   `loom/credential-wrap/v1` wraps the DEK with an independent nonce.
5. Canonical structured AAD binds schema version, credential reference,
   Provider, Provider Account, credential revision, and cipher version. The
   Vault record and singleton metadata separately bind key version and key
   identity. This keeps credential ciphertext stable during VMK rotation while
   substitution, unknown versions, nonce/tag failure, revision drift, or key
   metadata drift still fails closed.
6. Credential, conversation, and export wrapping use separate HKDF domains:
   `loom/credential-wrap/v1`, `loom/conversation-wrap/v1`, and
   `loom/export-wrap/v1`.
7. Provider verification, conversation routing, and Agent adapters acquire an
   exact short-lived plaintext lease from `CredentialLeaseManager`. Closing,
   expiry, replace, revoke, or rotation invalidates and zeroizes the lease.
   Normal dispatch never starts a Keychain helper.
8. Configure and replace are transactional: encrypted pending state precedes
   non-secret metadata commit; failure removes pending ciphertext. Revoke
   crypto-erases the record and invalidates leases. Journal contains only
   reference, revision, status, and safe facts.
9. Existing Keychain credentials require an explicit one-time migration or
   user re-entry. Migration is never automatic and never deletes the Keychain
   item until Vault write, verification, metadata transition, and user
   confirmation succeed. The helper is absent from the normal runtime path.
10. There is no plaintext fallback.
11. Conversation content uses the same VMK lifecycle but a separate per-
    Conversation DEK hierarchy under `loom/conversation-wrap/v1`. Transcript
    documents and Context Capsule payloads are encrypted independently from
    credentials. Non-secret Authority metadata may remain queryable, but
    Prompt, transcript body, Capsule body, Provider response, and hidden
    reasoning do not enter plaintext columns.
12. Every data nonce is reserved in one Conversation-wide registry before its
    ciphertext commit. Capsule and transcript tables cannot independently reuse
    the same nonce under a shared Conversation DEK. A conflict fails the entire
    transaction.
13. Legacy `chat-threads.json` migration is resumable and fail-closed. Loom
    writes and verifies all encrypted per-thread documents before atomically
    renaming the legacy file to a migration-pending identity. Normal chat starts
    only after that identity is revalidated, overwritten, fsynced, removed, and
    its parent directory fsynced. A legacy/Vault disagreement preserves the
    legacy file and requires recovery; Loom never chooses one silently.
14. When the Vault is locked or in recovery, normal product startup may expose
    recovery governance, but chat dispatch remains unavailable. It does not
    fall back to the legacy file or an ambient plaintext store.
15. Provider-native response, conversation, and prompt-cache handles are opaque
    encrypted Conversation records. Canonical AAD binds Conversation, Segment,
    Provider, Provider Account, Model, auth mode, credential reference/revision,
    handle kind, and monotonic handle revision. Native-auth routes bind the
    explicit absence of Provider Account and credential metadata instead of
    inventing an account. Handle ciphertext shares the per-Conversation DEK and
    global nonce registry with transcript and Capsule payloads. A daemon-only
    callback lease zeroizes plaintext after use. A Provider that does not
    advertise reusable native state remains stateless; Loom never persists a
    response ID merely because one appears in a response.
16. Ordinary daemon and setup composition defaults to the Loom Vault and may
    not construct `ProductKeychainStore`. A Keychain constructor is permitted
    only inside an explicitly named, user-triggered credential migration
    component. Missing ordinary-runtime credential composition fails closed;
    it never falls back to Keychain or plaintext.
17. Operational diagnostics may record only the closed backend marker `vault`
    or the explicitly injected legacy/test marker `explicit_legacy`. The marker
    is observational, is not execution authority, and does not replace the
    installed process-level no-helper acceptance gate.
18. The Vault runtime mutex protects component lifecycle and state transitions;
    it must not remain held across a Provider verifier, Conversation, or Agent
    callback. Exact short-lived leases own callback cancellation, expiry,
    revoke, rotation exclusion, and zeroization so independent Provider
    Accounts can execute concurrently without sharing credential material.

## Threat Model

LocalKeyFile protects against disclosure of `credential-vault.db` alone,
accidental database copying, and backups that exclude the private key file. It
does not claim resistance to an attacker with the same macOS user authority,
root authority, live daemon memory access, or control of the unlocked process.
FileVault and operating-system account security remain complementary controls.

The Vault rejects symlinks, non-regular files, wrong owner, group/world access,
unexpected hard links, key identity drift, and path replacement. It uses
owner-only directories/files, descriptor-relative identity revalidation,
exclusive creation, atomic replacement, and file plus parent-directory fsync.
The key file is excluded from ordinary sync/backup where the platform allows;
encrypted export is an explicit separate operation.

Overwriting and unlinking a legacy plaintext file is a best-effort local
cleanup, not a claim that APFS snapshots, SSD remapping, or external backups
are physically erased. New encrypted Conversation state supports local crypto-
erasure by deleting its wrapped Conversation DEK. Provider-side retention and
deletion remain separately governed capabilities.

## Alternatives Considered

### Read Keychain on every Provider call

Rejected as the normal path because it adds process and OS-auth coupling to
every turn and Agent Attempt. It remains only as an optional bounded migration
source.

### Encrypt all credentials with one database key

Rejected because one key compromise exposes every row and rotation requires
re-encrypting all secret payloads. Per-entry DEKs support bounded compromise and
cheap KEK rewrap.

### SHA-256 of a master secret as the wrapping key

Rejected because it provides no domain separation. HKDF gives explicit,
versioned separation between credential, conversation, and export material.

### Plaintext or environment fallback

Rejected because it would place long-lived secrets outside the Vault boundary
and make process inspection or inherited environment an authorization path.

## Consequences

- Installed Loom opens without Keychain prompts and supports repeated Provider
  calls with only local AEAD work per lease.
- The key file and encrypted database become security-critical local assets
  with strict lifecycle, backup, recovery, and rotation requirements.
- Loss of the LocalKeyFile VMK is fail-closed and requires credential recovery;
  ciphertext alone is intentionally insufficient.
- Passphrase unlock, enterprise secret providers, encrypted export, rotation
  UI, and explicit migration remain versioned Phase 2D slices.
- Normal production chat no longer requires a plaintext transcript file once
  the encrypted migration has completed. Existing installed bundles require a
  new source-derived installation and migration acceptance before this is an
  installed capability.
- The Vault can preserve Provider-native state without allowing it to cross a
  route boundary. Actual state reuse still depends on an explicit Provider
  capability and adapter contract; current stateless clients are unchanged.
- A slow Provider Account does not serialize another account at the Vault
  lifecycle lock. Account concurrency remains subject to its explicit capacity,
  budget, and policy controls.

## Migration Failure Isolation Amendment

A Conversation migration failure is isolated from daemon governance.
Migration read, encrypted commit, and plaintext cleanup emit privacy-safe
operational diagnostics under one Incident ID. The legacy source is preserved,
chat fails closed, and Provider setup, Vault recovery, diagnostics, and Team
governance remain available. The UI may project only the safe stage,
retryability, and Incident ID; it must not turn migration retryability into
resending a user message.

## Ordinary Runtime Constructor Amendment

The accepted one-time migration exception is made structurally explicit.
Non-test daemon code outside a `credential_migration` component cannot call the
Keychain store constructor. The generic daemon defaults to the Vault just as
the production builder does, while deliberately injected legacy stores remain
available to bounded compatibility tests. Release construction and AST tests
enforce the boundary independently.

The daemon also binds a closed `credential_runtime` marker into privacy-safe
operational diagnostics. This gives support evidence for which backend the
session selected without exposing a credential, secret, path, Prompt, or
Provider content. It does not prove historical process non-creation, so CV6
still requires approved installed observation.
