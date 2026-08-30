# P7-HT2 installed Build 324 verification

Status: `ACCEPTED / COMPLETE / AVAILABLE-RUNTIME LIVE GREEN / CLAUDE LIVE N/A BY USER`

Date: 2026-08-31

## Installed identity

Loom `0.5.6` Build 324 is the exact installed and running bundle at
`/Users/lune/Applications/Loom.app`. Candidate and installed bytes are
identical, strict deep code-sign verification passed, and Build 323 is retained
as the transactional rollback bundle.

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `2989b152b3cfb505414290220b1cab2b802e522f515cc20e3a2bbc488bc68deb` |
| `Contents/Library/Helpers/loomd` | `c851c9c7495f6c895e726daab203902324098befcbe724654d4f65eb9ce1d57a` |
| `Contents/Info.plist` | `e4a96f098f800adc5ffdda2b2fb927d9925f96a5a64a829fc18634fcad27e0f2` |

After the acceptance restart, the App runs as PID `17991`; its bundled managed
daemon runs as PID `18029` with canonical state, isolation, Socket, Harness,
local-model and managed-parent arguments. No standalone daemon was started.

## Closed blockers

Build 318 had exposed a Codex auth false positive: legacy `login status`
reported success while a real refresh failed. The final observer uses the
attested Codex App Server `account/read` contract and preserves actionable
mid-Session `provider_auth` failures. Its real protocol semantics treat an
authenticated account whose active provider requires OpenAI auth as available;
an unauthenticated account remains unavailable.

Build 323 completed the four-Runtime tool and restart sequence but failed its
final privacy gate. Codex and OpenCode child CLIs had created default-umask
temporary directories and files under persistent scratch roots. Build 324
repairs validated legacy roots to directory mode `0700` and file mode `0600`,
uses a canonical owner-only temporary directory for every child call, and
removes it on success, failure and cancellation. The Build 323 failure remains
historical failed evidence and is not counted as acceptance.

## Source and package verification

```text
go test ./... -count=1
PASS (all packages)

go vet ./...
PASS

go test -race -p=1 ./internal/provider -count=1
PASS

swift test --package-path apps/macos
496 XCTest cases, 2 conditional skips, 0 failures
21 strict Swift Testing contract cases, 0 failures

scripts/test-build-loom-local-app.sh
PASS (two independent deterministic release builds)

scripts/test-install-loom-local-app.sh
PASS

scripts/install-loom-local-app.sh --dry-run --app .dist-build324/Loom.app \
  --destination /Users/lune/Applications/Loom.app
PASS

codesign --verify --deep --strict /Users/lune/Applications/Loom.app
PASS
```

The installed post-restart identity preflight passed with 24 Providers, seven
Runtimes, six executable Conversation Profiles, the exact Phase 7 Mission/Team
governance anchor and seven RoundTable navigation records. Claude Code Runtime
is discovered online with capacity three, but no executable Claude Profile is
published.

## Installed live result

Before the complete matrix, two exact Build 324 Codex probes passed:

- ordinary text response: 8.10 seconds;
- Mission Proposal plus immutable cancel receipt: 11.33 seconds.

The explicitly authorized installed available-Runtime matrix then passed in
407.66 seconds:

```text
TestLivePhase7AvailableRuntimeControlToolsE2E
PASS
Codex, OpenCode, Pi and Loom Native accepted under the explicit Claude waiver
```

The run used OpenCode with the existing DeepSeek account and Loom Native with
the existing MiniMax account. Across Codex, OpenCode, Pi and Loom Native it
proved:

- all 28 Registry Tools selected through ordinary-language turns;
- exact least-privilege read and Proposal Tool sets;
- user-owned confirmation and cancellation receipts;
- digest-bound expiry and replay rejection;
- current, action-eligible RoundTable Pause, Steer, Retry, Skip and Replace
  targets;
- managed App restart and exact bundle/daemon identity revalidation;
- restored confirmed authority without restoring stale or receipt-less
  authority;
- bounded owner-only state and diagnostics scans with no plaintext acceptance
  marker.

The acceptance harness did not inspect or print credentials and did not
replace, migrate or revoke them. Normal Provider calls used the daemon's
existing short-lived credential leases. Prompt, transcript, Provider body,
Authorization value and hidden reasoning were excluded from logs and evidence.

## Claude waiver

The user explicitly stated that Claude Code is unavailable and waived only its
installed login and paid live call. Build 324 still passes Claude source
adapter parity, exact Profile admission, cancellation, privacy and
failure-isolation contracts. The installed gate independently confirms that no
executable Claude Profile exists and would reject the waiver if one appeared.
Claude is therefore `N/A`, not passed and not silently substituted.

## Acceptance

Build 324 satisfies P7-HT2 and closes Phase 7 for the Runtimes available to this
user. The model-driven Harness surface, macOS review/intervention experience,
authority boundaries, restart recovery and installed privacy gate are accepted.
