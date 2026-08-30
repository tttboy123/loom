# Phase 5 Build 236 New Task keyboard acceptance

Status: `INSTALLED CHECKPOINT / PHASE 5 IN PROGRESS`

## Installed identity

- App version: `0.5.3`
- App build: `236`
- App executable digest: `e3ae51ce3937d46c39792c0f3d62a8956d69b71c832c100f7d44e2228c1578ed`
- Bundled daemon digest: `dd1f3cd5f3b810d19b9e306f91b90ffd22459de4b0c4cca980c88699fb77ad41`
- Packaged and installed executable digests match exactly.
- Strict installed bundle signature verification passed.

## Live path

The installed App began in the restored Mission-linked, concluded RoundTable
context. Pressing `Command-N`:

- created a fresh isolated Conversation task;
- selected Conversation as the primary destination;
- closed the unpinned RoundTable inspector;
- preserved the executable Conversation Route and Model controls; and
- placed accessibility focus on `loom.conversation.composer`.

No extra App window was created.

## Automated gates

- Complete Swift package: 417 XCTest cases passed, two conditional skips.
- Swift Testing contracts: 20 passed.
- Complete Go repository, vet and focused race suites passed in the unchanged
  Build 235/236 daemon lineage.

This evidence contains no Prompt, Conversation content, Agent output,
credential or Provider response.
