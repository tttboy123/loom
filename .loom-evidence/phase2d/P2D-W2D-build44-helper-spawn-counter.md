# P2D-W2D build 44 helper spawn-attempt counter gate

Status: `CURRENT / PARTIAL`

Date: 2026-08-12

This gate replaces process polling with direct process-local instrumentation.
It exercises the exact bundled build-44 `loomd` against isolated owner-only
state without launching `Loom.app`, installing the Candidate, importing a
credential, or calling a Provider.

## Artifact identity

- Candidate:
  `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build44-candidate-2026-08-12/Loom.app`
- bundled daemon SHA-256:
  `e042c8a87f23e84ee32c527261a022118aa91670242c9f3457ed001b750e110f`
- source baseline HEAD:
  `651f156afda37a8e703cbc0396f9f38b7912600b`
- isolated state root:
  `/private/tmp/loom-build44-packaged-runtime.DzYDUk`
- installed Loom remained v0.5.2 build 39.

## Instrumentation contract

The credentials package owns a process-local monotonic `uint64` counter. The
Darwin Keychain process boundary increments it immediately before the sole
`command.Start()` call for `--credential-helper`, including failed start
attempts. Operational diagnostics snapshot the counter as
`credential_helper_spawn_attempts` when each record is appended. The value is
non-secret, observational only, and resets on daemon restart.

Go tests prove monotonic increments and source ordering. Go and Swift tests
prove the field is explicitly serialized, safely decoded/exported, and remains
backward compatible with older records that omit it.

## Packaged runtime observations

The first bundled-daemon session completed:

- production-client ping;
- Credential Vault lock and unlock;
- a content-free invalid `chat_message`, rejected before Provider dispatch;
- `setup_snapshot`, reporting `unlocked / local_key_file` with zero migration
  and recovery accounts.

The three operational records were:

| Operation | Credential runtime | Helper attempts | Stage | Result |
| --- | --- | ---: | --- | --- |
| `credential_vault_lock` | `vault` | 0 | `credential_lease_revoke` | succeeded |
| `credential_vault_unlock` | `vault` | 0 | `vault_open` | succeeded |
| `chat_message` | `vault` | 0 | `conversation_dispatch` | failed / `invalid_request` |

After a clean exit, the exact daemon restarted against the same state. The
Vault was already `unlocked / local_key_file`; another lock/unlock cycle
produced two more records with `credential_runtime=vault` and
`credential_helper_spawn_attempts=0`. Thus both daemon processes independently
reported zero helper start attempts, without relying on sampling cadence.

The Vault key retained device/inode `16777229:107193254`, mode `0600`, owner
UID 501, and size 62. The Vault database retained device/inode
`16777229:107193257`, mode `0600`, owner UID 501, and size 86016. The UDS was
owner-only `0600` and received a new inode after restart, as expected.

## Verification

- focused RED failed only on missing Go/Swift counter contracts;
- focused Go credentials/daemon and Swift diagnostic tests passed after GREEN;
- serialized `go test -p 1 ./...` passed;
- `go test -race ./internal/credentials ./cmd/loomd` passed;
- `go vet ./...` passed;
- complete Swift passed: 180 XCTest cases, one intentional skip, plus eight
  Swift Testing contracts, with zero failures;
- release construction, arm64, deep strict signature, owner-only modes,
  no-symlink, source gates, and ZIP byte-equivalence passed.

## Acceptance boundary

This closes the polling ambiguity for the isolated ordinary Vault path. It does
not prove the approved installed CV6 matrix: a real credential import,
verification, repeated Provider replies, Agent Attempts, restart, mixed-Team
dispatch, or optional one-time migration helper. No API key, Authorization
header, Prompt, conversation body, Provider response, ciphertext, nonce,
wrapped DEK, or hidden reasoning was used or retained in this evidence.

