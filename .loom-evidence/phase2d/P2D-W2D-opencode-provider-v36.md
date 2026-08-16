# P2D-W2D OpenCode Provider V36

Status: `SOURCE VERIFIED / PROVIDER CORE PRESENT / DAEMON WIRING OPEN (V37)`

Date: 2026-08-16

## Acceptance boundary

V36 adds OpenCode as a first-class Loom Provider: catalog identity, a bounded
native conversation client with the real OpenCode 1.18 CLI contract, and the
trusted JSON event decoder. It does not yet wire the daemon conversation
profile/router responder, runtime discovery or team-attempt harness adapter;
those form one interlocked surface (V37) that should land with live validation
against the real OpenCode CLI. No network call, App change, credential or user
workspace was used.

## Implemented source

- `internal/provider/catalog.go`: `opencode` descriptor
  (`DisplayName "OpenCode"`, `Category "official"`, `Protocol
  "opencode_agent"`, `AuthMode "native_auth"`, `ConnectionKind
  "native_runtime"`, model discovery supported) plus
  `OpenCodeConversationProfileID = "conversation-opencode-default-v1"`.
- `internal/provider/opencode_conversation.go`:
  - `OpenCodeConversationClient` (config validation, bounded `Respond`),
    `OpenCodeConversationProcessRunner`, and the system runner that spawns
    `opencode run --format json --pure --model <provider/model>` with the
    prompt as a message argument, private TMPDIR, real HOME (OpenCode native
    auth), process-group cancellation and executable identity checks.
  - `decodeOpenCodeConversation` parses the OpenCode 1.18 JSON event stream
    (`message.part.updated` text parts, `session.idle`, `session.error`,
    `auth.error`), fails closed on malformed or non-idle output, and never
    persists prompts or conversation content.
  - `ResolveOpenCodeNativeExecutable` canonicalizes the OpenCode CLI path.

## Verification

Passed:

- `go test ./internal/provider` (complete, including new
  `TestOpenCodeConversationClientRespondsBounded`,
  `TestOpenCodeConversationClientFailsClosed`,
  `TestDecodeOpenCodeConversationEventStream`, `TestProviderCatalog*`).
- `go test -race ./internal/provider`.
- `go test ./internal/runtime/harnessadapter ./cmd/loomd` focused.
- `go vet ./internal/provider`, `gofmt -l` clean, `git diff --check` clean,
  `go build ./...`, full `go test ./...` with no new failures.

## V37 boundary (next slice)

- Daemon `OpenCodeExecutable` config + resolution threading.
- Conversation profile in the setup snapshot (`conversation-opencode-default-v1`)
  and profile-router native binding for provider `opencode`.
- OpenCode conversation responder wiring in the daemon composition (G2).
- Runtime discovery observation (adapterType `opencode`) and the team-attempt
  harness adapter + process runner with the OpenCode JSON contract and a
  brokered credential env mapping (G3).

## Privacy and live status

No key, prompt, conversation content, Provider body or user workspace entered
source, logs or evidence. The installed App is running with the fresh Phase 2D
state; provider-key import and the live gates remain operator-driven.
