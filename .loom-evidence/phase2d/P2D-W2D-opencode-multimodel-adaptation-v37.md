# P2D-W2D OpenCode Multi-Model Adaptation V37

Status: `SOURCE VERIFIED / MULTI-MODEL ADAPTED / INSTALLED LIVE OPEN`

Date: 2026-08-16

## Acceptance boundary

V37 solves the OpenCode multi-model adaptation problem: one OpenCode runtime
now serves many Providers and models. Loom's Provider + Model binding is mapped
to OpenCode's `provider/model` identity, the bound Provider's API key is
injected through the exact environment variable OpenCode reads, and both the
conversation client and the team-attempt Harness adapter consume the same
contract. No live Provider call, App change, credential or user workspace was
used; the installed App remains operator-driven.

## Implemented source

### Provider adaptation (`internal/provider/opencode_adapt.go`)

- `OpenCodeModelIdentity(providerID, modelID)` maps a Loom binding to
  OpenCode's `provider/model` identity. Already-qualified models must match
  the Provider (no cross-Provider substitution); OpenRouter is a gateway and
  accepts `openrouter/<upstream>/<model>`.
- `OpenCodeCredentialEnv(providerID)` returns the exact env variable OpenCode
  1.18 reads per Provider, grounded in the installed binary:
  `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `DEEPSEEK_API_KEY`,
  `MOONSHOT_API_KEY` (kimi), `MINIMAX_API_KEY`, `XAI_API_KEY`,
  `ZHIPU_API_KEY`, `STEPFUN_API_KEY`, `OPENROUTER_API_KEY`,
  `GOOGLE_GENERATIVE_AI_API_KEY`, `DASHSCOPE_API_KEY` (alibaba-bailian),
  `OLLAMA_API_KEY`, `LMSTUDIO_API_KEY`.
- `ValidOpenCodeModelIdentity` + `DecodeOpenCodeText` (shared JSON event
  decoder for conversation and Harness).

### Team-attempt Harness adapter (`internal/runtime/harnessadapter`)

- `opencode_adapter.go`: `OpenCodeAdapterType`, `NewOpenCodeAdapter` and the
  `Execute` flow mirroring Codex (binding validation, context MCP, tool
  gateway, system prompt, diagnostics, publish). The binding's ProviderID +
  ModelID are adapted to `provider/model` before the process runner; the
  runtime instance is `runtime.opencode.local`; cross-Provider models fail
  closed before any credential or process access.
- `opencode_process.go`: `openCodeProcessRunner` runs
  `opencode run --format json --pure --model <provider/model> --dir <temp>`
  with the system prompt prepended to the message, injects the bound
  Provider's key through `OpenCodeCredentialEnv`, and decodes the event stream
  with `provider.DecodeOpenCodeText`. Accounting stays unobserved.
- `system_opencode.go`: `NewSystemOpenCodeAgentAdapter` builds the runner +
  adapter for the installed OpenCode CLI.

### Daemon wiring (`cmd/loomd`, `internal/app`)

- `--opencode-executable` flag; OpenCode executable resolution and threading
  through setup, core, execution and conversation composition.
- Runtime discovery observation `runtime.opencode.local`
  (`ensureProductVerifiedOpenCodeAgentRuntime`, probe
  `probe.opencode.local`, adapterType `opencode`, capacity 1).
- Attempt-loop adapter construction alongside Codex/Claude.
- Conversation: `productOpenCodeConversationResponder`, native binding for
  `conversation-opencode-default-v1` in the profile router, and the OpenCode
  native conversation profile in the setup snapshot (Swift model accepts the
  generic protocol/adapter with no change).

## Verification

Passed:

- `go test ./internal/provider` (adaptation + conversation + decoder tests).
- `go test ./internal/runtime/harnessadapter` (adapter multi-model + env
  injection + process-runner tests) and race.
- `go test ./internal/app` (setup profile list updated) and `./cmd/loomd`
  (complete daemon suite with the new wiring).
- `go vet` on touched packages, `gofmt -l` clean on touched files,
  `git diff --check` clean, `go build ./...`.
- Full `go test ./...` with no new failures; complete macOS package suite
  (`227` XCTest with `1` intentional skip + `15` Swift Testing) unchanged.

The adapter tests prove: `deepseek/deepseek-chat` adaptation, exact
`DEEPSEEK_API_KEY` env injection from the lease secret, cross-provider model
rejection before credential/process, and bounded OpenCode event decoding.

## Live verification (discovery, no paid call)

- Rebuilt and reinstalled `/Users/lune/Applications/Loom.app` with the V37
  wiring; the daemon now serves requests with
  `runtime.opencode.local | opencode | online` discovered alongside
  `runtime.loom-native.local` (deepseek-chat) and Pi.
- `opencode providers list` shows the environment supplies
  `DEEPSEEK_API_KEY`, `MINIMAX_API_KEY` and `ZHIPU_API_KEY` (existence only;
  values never read or recorded) — the exact env variables V37 injects from a
  Loom lease, so DeepSeek / MiniMax / Zhipu bindings are live-ready without
  OpenCode's own auth store.

## Privacy and live status

No key value, prompt, conversation content, Provider body or user workspace
entered source, logs or evidence. The installed App is running; OpenCode is now
a bindable multi-model Provider for conversation and team attempts. A paid live
`opencode run` validation was intentionally not executed without explicit
approval and remains operator-driven.
