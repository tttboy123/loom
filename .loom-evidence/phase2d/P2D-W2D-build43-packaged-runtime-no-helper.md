# P2D-W2D build 43 packaged-runtime Vault gate

Status: `CURRENT / PARTIAL`

Date: 2026-08-12

This gate exercises the exact bundled build-43 `loomd` directly without
launching `Loom.app`, installing the Candidate, reading a real credential, or
using the installed Loom state. It advances packaged-runtime evidence only; it
does not replace the installed Provider and mixed-Team live matrix.

## Artifact identity

- Candidate:
  `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build43-candidate-2026-08-12/Loom.app`
- bundled daemon SHA-256:
  `8064273e4c4e74f4fb6f2b5e094c09ad65d5411ac9105a3abee67a611a906a5e`
- source baseline HEAD:
  `651f156afda37a8e703cbc0396f9f38b7912600b`
- isolated state root:
  `/private/tmp/loom-build43-packaged-runtime.ON4Tjb`
- installed Loom remained v0.5.2 build 39.

The daemon arguments referenced only the isolated state, isolation, run, and
socket paths. The existing Pi and Node runtime directories were read-only
runtime inputs. No Codex or Claude executable and no credential or Provider
request was supplied.

## Admission observations

1. A first `/tmp/...` root failed closed at `build_assets`; macOS canonicalizes
   `/tmp` through `/private/tmp`, while the product path contract requires an
   exact canonical path.
2. A first canonical `/private/tmp/...` attempt with only the Pi `.bin` failed
   at `observer_version_process` because the isolated runtime search path did
   not include the Node interpreter required by the Pi shebang.
3. Supplying the same Pi and Node runtime directories used by local-App
   expansion started the bundled daemon successfully with an owner-only `0600`
   UDS.

These failed admissions created no service process, credential, or Provider
call and demonstrate fail-closed path/runtime validation.

## UDS and Vault observations

The first daemon session completed:

- `ping`: success;
- `credential_vault_lock`: success;
- `credential_vault_unlock`: success;
- intentionally incomplete `chat_message`: fail closed as
  `conversation_dispatch / invalid_request` before Provider dispatch.

The owner-only operational file recorded:

| Operation | Credential runtime | Stage | Result |
| --- | --- | --- | --- |
| `credential_vault_lock` | `vault` | `credential_lease_revoke` | `succeeded` |
| `credential_vault_unlock` | `vault` | `vault_open` | `succeeded` |
| `chat_message` | `vault` | `conversation_dispatch` | `failed / invalid_request` |

The daemon then exited cleanly and was restarted against the same isolated
state. `setup_snapshot` reported `unlocked / local_key_file`, zero migration
accounts, and zero recovery accounts. A second lock/unlock cycle succeeded and
again recorded `credential_runtime=vault`.

Before and after restart, the Vault key retained device/inode
`16777229:107092642`, mode `0600`, owner UID 501, and size 62. The Vault database
retained device/inode `16777229:107092645`, mode `0600`, owner UID 501, and size
86016. The UDS retained mode `0600` and owner UID 501 while receiving a new
inode, as expected for a restarted socket.

## Helper observation boundary

During each UDS request window, a 2 ms process sampler searched for the exact
bundled-daemon helper argv suffix `--credential-helper`. Both owner-only sample
files contain zero observations. The ordinary runtime diagnostic marker was
`vault` in every recorded operation.

This is stronger packaged-runtime evidence than source inspection alone, but a
polling sampler cannot prove that no extremely short-lived process existed
between samples. The release constructor gate and `credential_runtime=vault`
marker provide independent evidence that the normal backend was the Loom
Vault. Installed process observation across real import, multi-turn Provider
calls, Agent Attempts, and restart remains the CV6 acceptance boundary.

## Non-disclosure

No API key, Authorization header, Prompt, conversation body, Provider response,
ciphertext, nonce, wrapped DEK, or hidden reasoning was used or retained in
this evidence. The invalid chat request contained no message content.
