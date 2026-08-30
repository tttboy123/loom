# P7-HT2 installed Build 313 verification

Status: `PARTIAL ACCEPTANCE / FOUR RUNTIMES GREEN / CLAUDE CODE PROFILE BLOCKED`

Date: 2026-08-30

## Installed identity

Loom `0.5.6` Build 313 was the exact installed and running bundle for this
gate. Strict code-sign verification passed before and after the managed restart.

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `e2300168d2a0eaef7ae5f351f03c1cbf3e1409d82ec194166cf71988f0e08fa0` |
| `Contents/Library/Helpers/loomd` | `9c1b24a46136a65ef52e1150ed6ea5192ee4ac520f77befde84be848924c9964` |
| `Contents/Info.plist` | `bd4fbf6003842db4343833a1bcf51162c1446e5cf0deaf5432b01ce26906690e` |

The read-only identity preflight observed 24 Providers, seven Runtimes and six
executable Conversation Profiles. It used the explicit exact acceptance anchor
`mission/team-instance-643ace1b858f20a1d7b60f2ccd90d6a1` /
`team-instance-643ace1b858f20a1d7b60f2ccd90d6a1`; no fuzzy historical selection was
permitted.

## Real-model result

The explicitly authorized partial installed gate passed in 431.45 seconds for:

- Codex with OpenAI native authentication;
- OpenCode with the exact DeepSeek account Route;
- Pi with the locked local Qwen Route;
- Loom Native with the exact MiniMax account Route.

The run made real model requests but did not read, import, replace, revoke or
otherwise mutate credentials. It logged no Prompt, transcript, Provider body,
Authorization value or hidden reasoning.

Each Runtime completed the synthetic Mission Proposal lifecycle. Across the
four Runtimes, 28 ordinary-language turns selected the exact 13 read Tools and
15 Proposal Tools once each. The gate verified:

- exact Tool audit with no unrequested read or mutation Tool;
- user confirmation plus immutable decision receipt;
- replay rejection;
- user cancellation;
- five-minute expiry;
- encrypted Proposal and receipt restoration after App restart;
- fail-closed Route, Workspace, Registry, Segment, Attempt and target binding;
- action-valid RoundTable Pause, Steer, Retry, Skip and Replace targets.

The RoundTable acceptance path created only synthetic `p7-` Sessions. It did
not reuse the concluded historical Session as writable authority. The three
created streams ended with the authoritative terminal fact
`RoundtableConcluded`:

```text
roundtable/session/p7-rt-status-pause-*          RoundtableConcluded
roundtable/session/p7-rt-steer-1-*               RoundtableConcluded
roundtable/session/p7-rt-retry-skip-replace-*    RoundtableConcluded
```

The Mission navigation registry remained on its pre-existing Session; the test
fixtures did not replace the user's visible RoundTable entry.

## Source and UI regression

```text
go test ./... -count=1
PASS

go vet ./...
PASS

go test -race ./internal/controltool ./internal/api ./internal/roundtable \
  ./internal/harnessgateway ./internal/provider \
  ./internal/runtime/harnessadapter -count=1
PASS

swift test --package-path apps/macos
483 XCTest cases, 2 conditional skips, 0 failures
20 strict Swift Testing contract cases, 0 failures

git diff --check
PASS
```

The App and its managed bundled daemon remained running after verification.

## Remaining blocker

The formal five-Runtime installed gate stopped before any additional model
request or state mutation with:

```text
installed Phase 7 matrix missing executable claude-code profile
```

Claude Code source parity remains green, but the installed Setup Snapshot does
not publish an executable Claude Code Conversation Profile. The discovered
Claude Code `2.1.196` Runtime is online with capacity three, while the CLI
reports `loggedIn=false`, `authMethod=none`, `apiProvider=firstParty`. Setup has
neither an Anthropic Provider Account nor an Anthropic credential-import
candidate. This result cannot be promoted to five-Runtime acceptance. Phase 7
stays `PARTIAL` until the user completes native Claude authentication or
explicitly configures an Anthropic account, then the same installed matrix
passes all five Runtimes.
