# P7-HT1 installed acceptance - Build 259

Status: `PARTIAL / INSTALLED / LIVE MODEL GATE PENDING`

Date: 2026-08-28

## Bundle identity

- Installed path: `/Users/lune/Applications/Loom.app`
- Version: `0.5.5`
- Build: `259`
- Transactional rollback bundle: Build `258`
- Strict deep code-sign verification: passed
- Candidate and installed executable byte comparison: equal
- App executable SHA-256:
  `1497dc023909db3186772aadea8506cae63581f17c76b74df820d4ebaa67a417`
- Bundled daemon SHA-256:
  `461c320e1a928f72e51d8b443d28101984510dfb400ce61b94d6dbe2a645c967`

## Source gates

- Repository-wide `go test ./... -count=1`: passed for the Harness tool slice.
- Final `cmd/loomd` package after Runtime-label migration: passed in 227.369s.
- Affected `controltool`, `harnessadapter`, `api` and `harnessgateway` packages:
  passed.
- macOS suite: 447 XCTest cases, two conditional skips, zero failures.
- Strict Swift contract suite: 20 tests, zero failures.
- `go vet ./...` and `git diff --check`: passed.
- Deterministic native bundle fixture: passed.
- Transactional installer fixture and candidate dry-run: passed.

## Installed checks

1. Opening the App started its bundled daemon automatically. The daemon's real
   argv contains canonical state, isolation root, runtime dirs, private Socket,
   Codex/OpenCode/Claude executables and the App managed-parent PID.
2. Read-only real UDS `setup_snapshot` completed in 313ms and returned seven
   Conversation Profiles, 24 Providers, seven online Runtimes and five
   credential import candidates.
3. Profiles include Loom Native DeepSeek and MiniMax, OpenCode DeepSeek and
   MiniMax, OpenCode native auth, Claude Code native auth and Codex native auth.
4. Provider projection retains verified DeepSeek and MiniMax. No Provider or
   Runtime collection disappeared after adding the new IPC route.
5. Persisted native Runtime records migrated safely to `Loom Native (DeepSeek)`,
   `Loom Native (Kimi)` and `Loom Native (MiniMax)`. Codex, Claude Code,
   OpenCode and Pi remain independently visible and online.
6. The installed window keeps Conversation as the center, restores the existing
   Runtime & Providers inspector and leaves the composer usable while the
   governance panel is open.

## Visual evidence

- [Installed Runtime labels](P7-HT1-installed-build259-runtimes.jpeg), SHA-256
  `d20bad8dbd68202f6d73b721c50553d2560dd8ff8abc09d06b34b21714c1652d`
- [Alignment review at 560px](P7-HT1-control-proposal-560.png), SHA-256
  `4b4b0bf99c1b90b40a8bc0cf8b9d9f16276ad237288c12634085415e6095ed46`
- [Alignment review at 360px accessibility size](P7-HT1-control-proposal-360.png),
  SHA-256
  `e0103e791a581f61019728b9a42ff645726d49786d85f26fde005d076e978f4f`

## Remaining live gate

No real Provider request, credential mutation or paid model call was performed.
Build 259 is therefore not accepted as a complete Phase 7 Goal. Completion
requires an explicitly authorized installed Codex run proving that varied
natural-language alignment wording causes the model to select the Loom tools,
the App displays the Proposal, user confirmation creates a new Segment, and the
next response uses the approved Context Capsule. OpenCode, Claude Code, Pi,
Loom Native and broader product tools remain target work.

## Privacy

Evidence contains no API key, Authorization header, Prompt, transcript,
Provider body, hidden reasoning, MCP token, ciphertext, nonce or wrapped key.
The installed probe was metadata-only and did not read or change credentials.
