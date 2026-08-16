# P2D-W2D CV1 Contract: Loom Credential Vault

Status: `ACTIVE / CV1 FROZEN / CV2-CV4 PRODUCTION SOURCE CANDIDATES / CV5 PARTIAL`

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

This contract extends P2D-W2D and supplies W2A/W2B/W2C dispatch. It does not
create a new Goal, replace the DeepSeek conversation live gate, or reset
Conversation Segment, Context Capsule, or per-Agent binding Candidates.

## Boundary

`loomd` alone opens the Vault and holds the VMK. Swift may send a credential
only during explicit configure/replace over the private UDS. Conversation,
Agent, Prompt, Journal, Evidence, diagnostics, argv, ordinary environment, and
Provider Account projections contain no secret bytes.

Normal Provider calls use:

`Frozen binding -> exact account/reference/revision validation -> lease acquire -> local AEAD decrypt -> Provider call -> lease close/zeroize`

They never launch a Keychain helper. Existing Keychain access is optional,
explicit, one-time migration only.

## CV slices

- `CV1`: this contract, ADR-0020, threat model, schema, and RED;
- `CV2`: AES-GCM envelope, HKDF domains, LocalKeyFile, VaultStore, filesystem
  hardening, and crypto tests;
- `CV3`: lease manager plus conversation/Agent hot-path replacement;
- `CV4`: configure/verify/replace/revoke rollback and explicit migration;
- `CV5`: Vault status/recovery/rotation/export UI and diagnostics;
- `CV6`: installed DeepSeek multi-turn/restart and mixed-Team live matrix.

## Storage schema

Default paths are:

- `~/Library/Application Support/Loom/private/vault.key`;
- `~/Library/Application Support/Loom/state/credential-vault.db`.

Directories are `0700`; files are `0600`. Symlinks, non-regular files, wrong
owner, group/world permissions, link count other than one, key identity drift,
and path replacement fail closed.

The versioned key envelope contains only magic, schema/cipher/key-provider
versions, key identity, and a random 256-bit VMK. The VMK is stored separately
from the Vault database and loaded once per daemon session.

Each encrypted row contains exactly:

- credential reference, Provider ID, Provider Account ID, credential revision;
- schema, cipher, and key version;
- wrapped DEK and wrap nonce;
- secret ciphertext and data nonce;
- canonical AAD digest and status;
- created and updated UTC timestamps.

No row contains plaintext, a key preview, Authorization header, Provider body,
Prompt, nonce log, or reusable decrypted cache.

## Cryptographic contract

- random 32-byte DEK for each credential revision;
- AES-256-GCM with unique random 12-byte nonce for both secret and DEK wrap;
- HKDF-SHA256 VMK derivation with versioned domain strings;
- canonical length-delimited AAD over schema, reference, Provider, account,
  revision, and cipher version; Vault metadata separately binds key version and
  key identity so rotation can rewrap only the DEK;
- unknown version, malformed lengths, substitution, tag failure, nonce reuse,
  or revision drift fails closed;
- key rotation rewraps DEKs where possible and never exposes secret bytes.

Encrypted backup uses a separate versioned export envelope. A user passphrase
is processed with Argon2id (`time=3`, `memory=64 MiB`, `threads=4`, random
128-bit salt), then split through HKDF under `loom/export-wrap/v1` into distinct
DEK and bundle subkeys. Export unwraps each credential DEK under the active VMK
and rewraps only that DEK; it never builds an aggregate plaintext API-key
archive and never exports `vault.key`. The complete manifest, including
Provider Account identity and ciphertext, is itself AES-256-GCM encrypted and
bound to the versioned KDF header. Export is bounded to 256 credentials and
4 MiB, rejects pending credential mutations, clears the passphrase buffer, and
publishes a new `0600` `.loomvault` file with `O_EXCL` plus parent-directory
fsync. Wrong passphrase, tamper, unknown shape/version, existing destination,
or unsafe parent fails closed.

## LocalKeyFile threat model

The default mode protects a database copied without its key file. It does not
claim protection from the same logged-in user, root, live daemon compromise, or
memory inspection. No UI or documentation may call this stronger than local
envelope encryption. Passphrase mode uses Argon2id and External mode uses a
managed key/lease provider in later slices; neither blocks CV2 LocalKeyFile.

## Transactions and migration

Configure writes encrypted pending state before non-secret metadata commit and
deletes pending ciphertext on failure. Verify acquires one exact lease and
commits only safe status. Replace/revoke require exact revision, are atomic and
rollback-safe, and invalidate old leases immediately.

Migration is user-initiated. Re-entry is the default. Optional helper migration
reads once, writes and verifies the Vault, records a non-secret receipt, then
deletes the Keychain item only after explicit confirmation. Failure preserves
the old item and metadata and creates no half-migration.

## Diagnostics and UI

Closed stages add `vault_key_load`, `vault_open`, `vault_encrypt`,
`vault_commit`, `vault_decrypt`, `vault_aad_validation`, `vault_rotation`,
`vault_recovery`, `vault_export`,
`credential_lease_issue`, `credential_lease_expire`,
`credential_lease_revoke`, `migration_read`, `migration_commit`, and
`migration_cleanup`.

Diagnostics record only Incident ID, safe operation/account/reference identity,
revision, stage, elapsed time, result, and retryability. Ciphertext, nonces,
wrapped DEKs, keys, headers, prompts, and Provider bodies are excluded.

UI states are Unlocked, Locked, Migration required, and Recovery required. It
shows storage mode and safe recovery actions; Provider rows show revision and
status only. Known Vault failures never collapse to `Unavailable`.

## Current implementation boundary

CV1 is frozen. CV2 provides the versioned envelope, domain-separated HKDF,
canonical AAD, LocalKeyFile, encrypted SQLite VaultStore, strict file identity,
fault-injected transaction tests, restart behavior, and exact account/revision
isolation. CV3 provides bounded leases and production Conversation/Agent
injection; close, expiry, revoke, shutdown, and in-flight races cancel and clear
plaintext. CV4 provides encrypted pending mutations, metadata/finalize saga,
rollback, restart reconciliation, exact lease invalidation, and explicit
re-entry Import when legacy metadata has no Vault row.

Production daemon construction now enables and owns the Vault. Normal setup,
Conversation, and Agent dispatch do not construct or read ProductKeychainStore.
The legacy SecretStore lease adapter and helper remain only for tests and future
explicit one-time migration. No plaintext fallback exists. Exact Provider
Account Journal streams drive idempotency; Provider-only credential lookup is
not an authorization path.

CV5 is partial: setup projects Migration required or Recovery required from
Vault availability, suppresses unusable Profiles, presents Loom Credential
Vault in Swift, and performs explicit re-entry plus verify in one action. A
strict top-level projection now carries the active storage mode, aggregate
status, and migration/recovery account counts into Runtime & Providers. It does
not expose actions that are not implemented. Classified LocalKeyFile load/open
failure keeps the daemon and Setup UI in a restricted recovery mode while all
credential mutations and leases fail closed with the original stage and no
Keychain fallback. Other construction failures remain fatal. The closed Vault
stage set is accepted by diagnostics and Swift. KeyMaterial retains the exact
key-file device/inode identity; VMK lease, health, and mutation boundaries
revalidate it without rereading key bytes, so live path replacement fails
closed and projects recovery. Rotation is now a source Candidate: an explicit
next-version LocalKeyFile is staged without replacement; every active DEK wrap
and Vault metadata advance in one SQLite transaction while credential
ciphertext remains unchanged; pending credential mutations
reject rotation; and pre-commit failure retains the old key and rows. A fixed
pending-key startup protocol promotes only a database-committed key or removes
an uncommitted key, while mismatch fails closed at `vault_rotation`. The lease
generation barrier revokes active plaintext and rejects in-flight old-generation
acquisitions. The private UDS and Swift UI expose a real Rotate key action with
an extended timeout, safe operational diagnostic, stage, Incident ID, and
authoritative Setup refresh. Interactive Lock now / Unlock is also a source
Candidate: Lock closes the lease manager and VaultStore, revokes and zeroizes
active plaintext leases, and releases the session VMK before projecting
`locked`. Unlock revalidates rotation and key-file identity, rebuilds store,
lease access, verifier, and coordinator, reconciles pending mutations, then
projects `unlocked`. Strict UDS methods, extended deadlines, safe diagnostics,
and the Swift action refresh the authoritative Snapshot and retain stage plus
Incident ID on failure. Explicit Recovery reset is now a source Candidate and
is available only from `recovery_required`: the destructive UI confirmation
maps to one exact private UDS confirmation, validates every canonical Vault
file before deletion, removes the unusable VMK/ciphertext and SQLite sidecars,
fsyncs both owner-only parent directories, then rebuilds the runtime in place.
No metadata is silently promoted or rewritten; accounts with old authoritative
references project `migration_required` and require API-key re-entry. Unsafe
paths, ordinary unlocked/locked runtimes, repeated reset, and incorrect
confirmation fail closed at `vault_recovery`. Passphrase-protected encrypted
export is also a source Candidate: the Swift sheet requires matching bounded
passphrases and a user-selected destination; private UDS transports base64
bytes, daemon admission validates the request, and `vault_export` diagnostics
exclude passphrase and destination. Source restore-format tests rewrap exported
DEKs under a new VMK and recover exact Provider Account/revision bindings, but
no restore UI is claimed in this Phase slice. Optional helper migration remains
open and is not part of the normal runtime path.

Installed v0.5.2 build 20 is now an installation/startup Candidate for this
boundary. The installed App and bundled daemon match the packaged hashes and
pass strict deep signature verification. Launch starts the canonical child
daemon, creates/opens the LocalKeyFile Vault under owner-only directories, and
retains `0600` permissions on `vault.key`, `credential-vault.db`, and the UDS.
Its non-secret Setup Snapshot first projected the legacy DeepSeek revision as
`migration_required / vault_entry_missing`, suppressed the stale Conversation
Profile, and exposed the explicit `Move key to Vault` action. The user then
completed real re-entry. Installed diagnostics record replace plus verify
success without secret content; the snapshot reports DeepSeek verified revision
6 and publishes `conversation-deepseek-deepseek-chat-r6`. A real App/daemon
restart preserved Vault file identity and `0600` permissions, automatically
unlocked LocalKeyFile, restored revision 6/Profile r6, and left no active
credential helper. Build 19 remains the transactional rollback bundle. This
does not close CV6: a new r6 reply, multi-turn no-helper observation, and the
mixed-Team matrix still must pass before an installed Keychain-removal claim is
permitted.

The first r6 Conversation attempt after restart did not produce a Provider
reply. Incident `loom-chat-d6873ebe-e5f7-41b5-9b24-a204688ca2bb` created the
expected new immutable Segment and frozen Profile binding, but authoritative
Attempt state is `failed / conversation_unavailable`; the stored Loom response
is the bounded local fallback. Build 20's operational layer incorrectly recorded
the enclosing successful IPC response as `conversation_dispatch / succeeded`.
This is an observability failure and leaves Vault lease versus Provider
HTTP/auth/rate-limit unresolved. No helper process was present, so the evidence
does not regress the no-Keychain hot-path boundary, but it also does not satisfy
the live reply gate.

The source Candidate now freezes Incident ID, failure stage, safe code, and
retryability on every Conversation Attempt; classifies Provider DNS/TLS/connect,
HTTP auth/rate-limit/rejection/server/invalid-response outcomes without body
retention; and makes daemon/App diagnostics honor the Attempt terminal state.
The App preserves the failed thread and draft and renders the specific recovery
action. A new installed retry is required to identify and fix the actual r6
failure before CV6 can advance.

Installed v0.5.2 build 21 now carries that staged-failure Candidate. Its App and
daemon hashes match the packaged bundle, deep signature and dry-run install
validation pass, and build 20 remains the rollback bundle. Startup preserves
the owner-only Vault identities, auto-unlocks LocalKeyFile, restores verified
DeepSeek revision 6/Profile r6, and starts no helper. DeepSeek is selected in the
UI without dispatch. Build 21 still requires one user retry before the live
failure class or reply gate can be claimed.

The final source regression for this increment passes full relevant Go
credentials/app/api/localipc/runtime/daemon packages, focused Vault and daemon
race checks including live key-path replacement, rotation/restart, and explicit
recovery reset/re-entry plus encrypted export/rewrap, Go vet, diff checks, 167
XCTest with one intentional visual-export skip, and six Swift
Testing contracts. These source results plus build 20's real import/verify and
restart smoke do not satisfy CV6's complete installed live gate.

## CV1 RED matrix

RED must fail only because the Vault symbols do not exist and cover:

1. AES-GCM roundtrip, tamper, AAD/account/revision substitution, unknown
   version, empty/oversized secret, and nonce uniqueness;
2. HKDF credential/conversation/export domain separation;
3. owner-only key/database creation and rejection of symlink, hardlink,
   world-readable, wrong identity, missing/changed key, and partial state;
4. restart, concurrent multi-account isolation, zeroization, lease expiry and
   revoke;
5. absence of plaintext from database, diagnostics, Journal, argv, global
   environment, and exports.

CV1 is frozen because the ADR, this contract, CURRENT/amendment, and focused RED
agree. The GREEN source Candidate and build 20 startup smoke do not claim CV6
completion before the live matrix passes.

## Installed build 39 multi-turn Vault advancement

Read-only evidence from the currently running v0.5.2 build 39 installation
advances CV6 beyond the earlier build-20/21 startup-only state. The installed
LocalKeyFile Vault is unlocked after restart; DeepSeek account revision 6 is
verified and backed by one active encrypted Vault row; the key file, database,
and UDS remain owner-only `0600`. Five DeepSeek r6 Attempts completed
successfully after the current App and bundled daemon started, and earlier
Provider timeouts were followed by successful calls. This accepts installed
import, verify, Profile publication, automatic restart unlock, repeated real
conversation dispatch, and recovery for the observed DeepSeek account.

CV6 remains `ACTIVE / PARTIAL`. No helper or Keychain process was active at the
time of inspection, but the historical successful calls did not include a
dedicated process-spawn observation, so no-helper hot-path removal is not yet
fully proven. A retained build-39 delivery manifest was also unavailable, so
the evidence is bound to exact installed hashes rather than a retained
Candidate comparison. Post-rotation continuity, encrypted export/restore,
single-entry corruption and revoke isolation, and the installed mixed-Team
DeepSeek/OpenAI/Anthropic/MiniMax matrix remain open.

## Domain-separated Team Capsule storage advancement

The Vault now applies its reserved `loom/conversation-wrap/v1` domain to real
Team Mission Context Capsule storage. Each Conversation owns one random DEK;
only that DEK's wrapped form and encrypted Capsule payloads enter the owner-only
Vault database. Capsule Authority metadata remains non-secret and is bound into
canonical AAD. Prompt/context body, credential, Provider body, and hidden
reasoning are not plaintext columns.

Store tests prove restart lookup, exact digest/Conversation binding, tamper and
substitution rejection, nonce uniqueness, Conversation-local crypto-erasure,
and VMK rotation rewrap. Mission compilation is wired fail closed: an
unavailable or locked Vault cannot silently dispatch with an unpersisted or
plaintext Capsule. This advances the encrypted Capsule portion of CV3/CV5 but
does not complete CV6. Transcript and ExternalSessionHandle encryption,
context-aware encrypted backup/restore, installed migration/restart, live
rotation continuity, and the mixed-Team matrix remain open.

## Encrypted ordinary Conversation document advancement

The Vault now stores ordinary thread transcripts as canonical per-Conversation
encrypted documents under `loom/conversation-wrap/v1`. Transcript and Capsule
ciphertexts share the Conversation DEK lifecycle but use one global nonce
reservation table, closing cross-table nonce reuse. Monotonic document revisions,
AAD identity, idempotent exact retry, tamper rejection, restart lookup, rotation
rewrap, and Conversation-local crypto-erasure are covered by source tests.

The product daemon uses this Store whenever the default Vault is unlocked. A
locked/recovery Vault exposes recovery governance while chat fails closed; it
does not use plaintext persistence. Legacy JSON migration is resumable and
validates owner-only file and directory identity before best-effort overwrite,
fsync, unlink, and parent fsync. No physical-erasure claim is made for APFS
snapshots, SSD remapping, or external backups.

CV6 remains `ACTIVE / PARTIAL`: installed build 39 has not migrated, and
installed encrypted restart/reply, migration diagnostics, encrypted native
session handles, context-aware backup/restore, rotation continuity, and the
mixed-Team live matrix remain required.

## Build 40 encrypted Conversation Candidate

The CV3/CV5 ordinary Conversation storage increment is retained as uninstalled
v0.5.2 build 40. Its arm64 App and daemon, deep strict signature, owner-only
bundle, no-symlink boundary, ZIP byte equivalence, and exact hashes are recorded
in the Candidate manifest. The build-only path passed; launch smoke was omitted
because it could perform the one-time migration against the current user's real
build-39 transcript.

This packaging evidence does not advance CV6 live acceptance. Build 39 remains
installed and unchanged. Explicitly approved migration, encrypted restart and
reply, migration diagnostics/recovery UI, encrypted native handles, no-helper
hot-path instrumentation, and mixed-Team Vault isolation remain open.

## Encrypted ExternalSessionHandle advancement

The Conversation DEK boundary now also stores opaque Provider-native session
handles. Handle AAD binds the complete route identity and monotonically
versioned handle kind; its data nonce is reserved in the same Conversation-wide
registry used by transcript and Context Capsule ciphertext. Account, Model,
Segment, auth, credential-reference/revision, metadata, ciphertext, and tag
substitution fail closed. Vault VMK rotation rewraps the owning Conversation
DEK, and Conversation crypto-erasure cascades handle ciphertext without
affecting peer Conversations.

The production Vault runtime exposes only a bounded callback lease and clears
both caller-owned write buffers and read leases. Native auth is represented by
the explicit absence of account/credential metadata; account-bound auth
requires the exact Provider Account and positive revision. Stateless Provider
clients remain stateless, so this Store does not create unaudited retention or
silently reuse arbitrary response IDs.

This is source progress after the uninstalled build 40 Candidate. CV6 still
requires packaging and installed restart/isolation, a real stateful-capability
adapter acceptance path, migration diagnostics, context-aware backup/restore,
no-helper instrumentation, rotation continuity, and mixed-Team Vault isolation.

## Migration diagnostics source amendment

Conversation migration now uses the accepted Vault diagnostic stages and a
single startup Incident ID. Read/validation and encrypted-commit failures leave
the legacy source untouched and keep the product daemon's governance surfaces
available while making chat non-replyable. The App receives only a bounded
stage, Incident ID, and retryability projection; no transcript or thread
identity enters diagnostics. Build 41 predates this code. Installed migration,
restart, recovery, and reply remain CV6 acceptance work.

Build 42 packages this source with passing source and static transport gates.
It remains unlaunched and uninstalled; CV6 installed migration, restart,
recovery, reply, no-helper, rotation, and mixed-Team gates stay open.

## CV3 ordinary runtime no-helper source gate

The default product daemon and the production product daemon both use the Loom
Credential Vault. Omitting an injected credential backend is no longer an
implicit request for `ProductKeychainStore`. Setup composition without a Vault
mutator or an explicit legacy test store fails closed.

Normal non-test daemon source cannot construct `ProductKeychainStore`. Only a
future component whose filename explicitly identifies `credential_migration`
may do so, and that component remains opt-in and outside configure, verify,
Conversation, and Agent Attempt hot paths. A source AST test and release build
gate enforce the constructor boundary.

The daemon binds `credential_runtime=vault` into privacy-safe operational
records, including chat and Agent Attempt records. `explicit_legacy` is
reserved for deliberately injected legacy/test composition. Unknown or
changing values are rejected, and Swift uses the same closed decoder. This
marker contains no secret material and is not execution authority.

Complete source verification passes, including a private-UDS test that starts
the generic daemon, performs a chat operation, and observes the Vault marker.
CV6 remains open: only an approved installed process observation can prove that
no credential helper is spawned during import, repeated real Provider calls,
Agent Attempts, restart, or optional migration. No installed data was migrated
or modified by this source gate.

Build 43 packages the source gate with passing release, architecture,
signature, permission, symlink, and ZIP byte-equivalence checks. It remains
uninstalled and its App was not launched. The exact bundled daemon was launched
directly with isolated temporary state: two sessions selected
`credential_runtime=vault`, preserved Vault file identity across restart, and
completed lock/unlock without an observed helper process. The bounded sampler
cannot prove absence between samples, and no real credential, Provider call, or
Agent Attempt was used. CV6 still requires the approved installed process-level
no-helper matrix and all previously open rotation, migration, restart,
isolation, and mixed-Team observations.

## Build 44 direct no-helper packaged-runtime gate

The credentials process boundary now increments a process-local monotonic
counter immediately before any Darwin `--credential-helper` start attempt.
Operational diagnostics explicitly snapshot that count. The exact bundled
build-44 daemon ran two sessions against the same isolated owner-only Vault;
all five lock, unlock, and content-free invalid-chat records reported
`credential_runtime=vault` and `credential_helper_spawn_attempts=0`. Vault key
and database identity remained stable across restart.

This closes the build-43 sampling blind spot for the isolated ordinary runtime,
but not CV6. Installed real-key import, verification, multi-turn Provider calls,
Agent Attempts, mixed-Team dispatch, restart, and the explicit optional
migration path still require approved counter-backed observation.

## CV3 Provider Account concurrency amendment

The Vault runtime lifecycle mutex no longer spans a Provider callback. It now
validates lifecycle state and snapshots the active lease access under the lock,
then relies on `CredentialLeaseManager` for callback cancellation, expiry,
revoke, rotation exclusion, and zeroization. Independent Provider Accounts can
therefore overlap without sharing a client or credential, and one slow network
call no longer holds the Vault lifecycle lock.

A RED test proved the prior behavior serialized a held DeepSeek callback and a
second Anthropic callback. The GREEN implementation passes focused normal and
race tests together with Vault lock and rotation regressions. Evidence is
`../P2D-W2D-vault-account-local-lease-concurrency.md`.

This advances CV3 source behavior only. Installed account-level concurrency,
limits, cost accounting, and the complete CV6 no-helper matrix remain open.

Build 47 packages this repair together with the W2C observed workspace
baseline. Complete source, race, release, architecture, signature, permission,
no-symlink, contract-string, and ZIP byte-equivalence gates pass. The App and
bundled daemon remain unlaunched and uninstalled, so CV6 remains open.

## Current Phase alignment

Credential Vault remains a P2D-W2D component used by W2A Conversation routing
and W2B/W2C Agent dispatch; it is not a separate product Goal. The production
daemon's ordinary path selects the Vault, and adapters receive only exact
short-lived account/reference/revision leases. The remaining Keychain helper
entry point is restricted to the explicit optional one-time migration boundary
and tests. There is no plaintext, argv, ambient-environment, or Keychain
runtime fallback.

Account-local Provider timeout classification is specified in
`P2D-W2D-observability-governance.md` and does not change the Vault crypto or
lease contract. CV6 still requires approved installed multi-turn/restart,
no-helper counter, rotation, revoke isolation, and mixed-Team observation.

## Four-Provider encrypted-record corruption source gate

The mixed-Team source matrix now composes the real encrypted VaultStore with
four exact Agent bindings. After all four credentials are encrypted and the
Store is closed, a test alters only one MiniMax ciphertext and reopens the
Vault with the same LocalKeyFile. The MiniMax lease fails closed at
`vault_decrypt`; OpenAI, Anthropic, and Kimi continue through their independent
records and succeed. Database bytes and Team Journal facts contain none of the
four plaintext test secrets.

This strengthens CV6's source prerequisite for single-entry corruption
isolation. It does not replace the approved installed matrix, which still must
observe the same isolation together with the direct no-helper counter, restart
and rotation continuity, real Provider replies, accounting, and fallback.

The encrypted corruption case now also traverses the product Agent hot path and
persistent operational diagnostic store. It proves the Provider client is not
called and the exact matching Attempt receives
`vault_decrypt / credential_unavailable / not retryable`. Recovery-required
Vault stages are governed by one shared credential-stage retry policy used by
Native and Harness adapters; retry metadata is no longer invented separately by
the Board. Build 60 contains the final closed allowlist: only lease issue,
expiry, and revoke may be retryable; every other known or unknown stage is not
directly retryable. Installed CV6 remains open.

Vault-backed fallback source composition is also closed. A corrupted primary
record cannot authorize or supply a fallback. Only a pre-existing versioned
approval between exact frozen binding digests permits a new Attempt, and that
Attempt resolves its own Provider Account, reference, and revision from an
independent encrypted record. A four-Provider Team proves the failed primary,
approved backup, and three healthy peer identities remain isolated. This does
not replace installed CV6 observation.

## Four-Provider rotation and restart source gate

The mixed-Team source matrix now also executes the production rotation
transaction across all four exact credential records. The lease rotation
barrier revokes a pre-rotation lease, all credential DEKs are rewrapped under a
new LocalKeyFile version, the pending key is atomically promoted, and canonical
key material is adopted. After closing and reopening the Vault and lease
manager, all four Agents acquire fresh leases and succeed with their original
frozen binding digest and credential revision.

This closes source composition for post-rotation restart continuity. Installed
CV6 still requires counter-backed observation through real Provider calls
before and after restart/rotation; source success cannot prove the installed
process, network, or account state.
