# P2A-W1 Native Product Experience Vertical Reopen Contract

**Date**: 2026-07-29
**Status**: `FROZEN - FRESH INDEPENDENT CONTRACT RE-REVIEW PASS`
**Parent**: P2A-W1 Local App Shell and Read Experience Contract plus Native App Host
**WorkItem count**: unchanged; this remains P2A-W1
**Live authority**: none
**Bounded Product Owner authority**: the user's 2026-07-29 request invoking
`design-taste-frontend` and `ui-ux-pro-max` to optimize the current project UI

## 1. Why this reopen exists

The current native shell exposes Home, Runtimes, Teams, Runs / History,
Evidence, Compare, Attention, and Team Timeline as eight equal primary
destinations. Its Home surface is a read-model counter dashboard. The result is
technically legible but presents Loom as an operator console rather than a
product for people delegating and following work.

Current first-party Multica product material demonstrates a different product
hierarchy:

- Issues, Inbox, projects, chat, and the work timeline are the everyday
  collaboration surface;
- Agents appear as teammates inside assignment and discussion flows;
- Runtime inventory and usage are a supporting machine/settings surface;
- a user begins from a unit of work and watches progress rather than choosing
  an internal persistence or evidence entity first.

Loom will adopt that task-first information architecture without copying
Multica branding, assets, source, terminology where Loom has a different
authority model, or behavior that P2A-W1 does not provide.

This is one vertical product-experience reopen. It is not P2A-W4, a new
WorkItem, a chain of point Amendments, or authority to retry the failed live
canary.

That explicit Product Owner request supersedes only the earlier prohibition on
changing the exact Swift presentation files owned in section 4. It does not
supersede the failed live result, consumed allowance, P2A-W2 lock, no-retry
boundary, or any non-UI authority prohibition. Contract Review cannot invent
this authority; it can only decide whether the bounded reopen is safe and
complete.

## 2. Design read and system

The product is a nontechnical-user-facing SwiftUI local Agent workspace. It
uses a task-first and conversation-ready product mental model, Apple platform
conventions, and Loom-owned semantic tokens.

Frozen design dials:

```text
DESIGN_VARIANCE = 6
MOTION_INTENSITY = 4
VISUAL_DENSITY = 5
```

The visual system is:

- native San Francisco typography with Dynamic Type;
- macOS semantic backgrounds, separators, labels, materials, and one system
  accent color;
- panels use one 16-point radius; compact controls use the platform default;
- hierarchy comes from spacing, typography, lists, and disclosure, not a grid
  of equal metric cards;
- light and dark appearance use the same semantic hierarchy;
- motion communicates loading, selection, or state transition only and obeys
  Reduce Motion;
- SF Symbols are the only icon family;
- no AI-purple palette, pink generation accent, decorative status dots,
  generic glass card wall, fake precision, invented activity, or copied
  Multica visual asset is allowed.

The `ui-ux-pro-max` generated palette is explicitly rejected where it conflicts
with the invoked `design-taste-frontend` anti-default rules and native Apple
semantics.

## 3. Vertical capability

This reopen changes the existing read-only native app into one coherent
workspace:

1. The sidebar has exactly five primary destinations:
   `Home`, `Work`, `Teams`, `Inbox`, and `System`.
2. Home answers three user questions in order:
   what needs attention, what work activity is present in the authoritative
   snapshot order, and whether a local team can be observed.
3. Work owns Run history and accepted Evidence. Evidence is contextual to work,
   not a primary destination. Compare is absent in every state because the
   current schema contains no authoritative comparison result. No comparison
   metric or unavailable Compare destination is fabricated.
4. Teams owns Team selection and the selected Team timeline in one flow. Team
   Timeline is no longer an equal primary destination.
5. Inbox owns attention items and makes the required human action the dominant
   text.
6. System owns daemon connection state and Runtime inventory. Runtime detail is
   available without competing with everyday work navigation.
7. Loading, connected, partial, stale, offline, fatal, empty, and populated
   states each have distinct, plain-language treatment.
8. Refresh remains the sole native action. It is available from the toolbar and
   relevant recovery surfaces, uses the existing bounded snapshot read, and
   never becomes a hidden retry loop.
9. The interface never exposes raw internal identifiers, credentials, hidden
   reasoning, cursor values, provider configuration, or terminal control
   sequences.
10. The app remains a typed direct client of the existing private IPC API. It
    does not parse CLI output or create a second state authority.

P2A-W1 remains read-only. A composer, New Task button, Team creation,
assignment, confirmation, approval, retry, Provider configuration, or Runtime
execution control must not be simulated, enabled, or advertised as already
available. The navigation and layout may reserve space for later accepted
capabilities, but disabled or fake product actions are prohibited.

## 4. Exact owned files

This reopen may modify only:

```text
apps/macos/Package.swift
apps/macos/Sources/LoomLocalApp/ContentView.swift
apps/macos/Sources/LoomLocalApp/LoomLocalApp.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift
docs/CURRENT.md
```

It may create only:

```text
apps/macos/Sources/LoomLocalAppCore/LocalProductExperience.swift
apps/macos/Sources/LoomLocalAppUI/ContentView.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift
.loom-evidence/phase2a/P2A-W1/native-product-experience-reopen-contract.md
.loom-evidence/phase2a/P2A-W1/native-product-experience-reopen-contract-review.md
.loom-evidence/phase2a/P2A-W1/native-product-experience-reopen-contract-review-2.md
.loom-evidence/phase2a/P2A-W1/native-product-experience-red.md
.loom-evidence/phase2a/P2A-W1/native-product-experience-green.md
.loom-evidence/phase2a/P2A-W1/native-product-experience-implementation-review.md
.loom-evidence/phase2a/P2A-W1/native-product-experience-visual-audit.md
.loom-evidence/phase2a/P2A-W1/native-product-experience-preview.png
```

No other file is owned. The new core file may contain only deterministic
presentation grouping, navigation semantics, safe visible copy, and derived
read-only counts or collections. It cannot add persistence, networking,
commands, state authority, or an execution decision.

`Package.swift` may change only to create a `LoomLocalAppUI` library target,
move the existing `ContentView` implementation into it, make the executable
depend on that target, and let the existing test target import it. The old
executable-target `ContentView.swift` must be deleted after the move. No package,
remote dependency, deployment target, product command, or application entry
semantics may otherwise change.

## 5. Explicit exclusions

This reopen does not own or change:

- Go product source, IPC schema, Journal facts, Projection semantics, daemon,
  service manager, installer, bundle builder, plist, Runtime adapters, Provider
  access, credential handling, Scheduler, Supervisor, Grant, Evidence
  authority, or accepted ADRs;
- the diagnosed `model_ids:null` cross-language normalization defect;
- the exact rollback evidence, consumed canary allowance, original resident
  observer, installed files, Journal bytes, or crash inventory;
- P2A-W2 or P2A-W3 behavior;
- localization, web UI, TUI parity, mobile UI, notification delivery, or a
  public/network listener.

The existing terminal state remains historically true:

```text
P2A-W1: FAIL - ROLLED_BACK - HUMAN_REQUIRED
live allowance: 0
P2A-W2: LOCKED
```

Design acceptance cannot reinterpret or repair that live result.

## 6. Deterministic presentation contract

`LocalProductExperience` must be a pure projection from the existing snapshot,
timeline, selection, and closed connection state. Its output must:

- preserve source ordering for authoritative records;
- use no current time, random value, network, file system, environment, or
  global mutable state;
- derive the same output for the same input;
- distinguish no snapshot from a loaded empty snapshot;
- preserve a prior snapshot when refresh fails, matching the accepted store
  behavior;
- never invent a Run, Team, attention item, Runtime, status, or Evidence;
- keep all raw identifier fallback behavior generic and safe;
- expose semantic navigation labels and accessibility descriptions for direct
  tests.

The source-ordered Run collection must be named `workActivity` in presentation
code and "Work activity" in visible copy. It must not be sorted, timestamped, or
described as recent.

## 7. Interaction and accessibility contract

The native interface must implement:

- keyboard traversal and native sidebar selection;
- Command-R refresh with a visible accessible label and hint;
- no icon-only action without an accessibility label;
- a minimum 44-point effective target for explicit recovery actions;
- text that remains readable at larger accessibility sizes without clipping or
  one-line truncation of essential state;
- semantic headings and combined row descriptions;
- status communicated by text and symbol, never color alone;
- system focus rings and selection states;
- no perpetual animation;
- loading indicators and any transition honor Reduce Motion;
- contrast derives from semantic system colors in both light and dark mode.

Deterministic proof is limited to what the owned Swift surface can establish:

- `LoomLocalAppUI` is imported by tests, so the real `ContentView` type and its
  shared design constants compile under test;
- every render test supplies an in-memory `LocalProductClientProtocol` stub and
  performs no file, socket, daemon, service-manager, Provider, Runtime, Journal,
  or network access;
- a shared minimum-action-target constant is at least 44 points and is used by
  every explicit recovery action;
- the actual view renders nonempty output for loading, connected-empty,
  connected-populated, partial, stale-with-preserved-view,
  offline-without-view, offline-with-preserved-view, and fatal states in both
  light and dark appearances;
- one accessibility-size render proves essential headings and actions do not
  rely on fixed one-line frames;
- source audit proves the real toolbar owns Command-R, explicit accessibility
  labels/hints, system semantic colors, SF Symbols, native controls, and no
  explicit animation or focus suppression.

This reopen does not claim a full VoiceOver, switch-control, or keyboard E2E
canary. Those remain a later installed-product verification after the separate
schema defect is governed. The present contract still requires accessibility
semantics in source and deterministic native rendering; it labels the missing
assistive-technology E2E proof honestly.

Visible product copy must use plain functional sentences. It must not contain
an em dash, internal code name, raw ID, provider secret, fake roadmap promise,
or unsupported claim that Loom can create or execute work from this screen.

## 8. RED-first implementation

Before product behavior changes, tests must fail for missing frozen behavior:

1. exactly five primary destinations in the frozen order;
2. Team selection remains inside `Teams`;
3. Home grouping orders attention before source-ordered Work activity and
   system readiness;
4. Work groups Runs and accepted Evidence without inventing Compare data;
5. Inbox promotes `action_required` over internal kind;
6. System owns Runtime inventory;
7. empty, loading, stale/offline-with-preserved-view, and populated
   presentation states remain distinct;
8. visible frozen copy contains no forbidden em dash.
9. the actual `ContentView` renders through a non-connecting stub in the frozen
   state and appearance matrix;
10. the shared explicit-action target used by the actual UI is at least 44
    points.

RED must fail for missing behavior, not compilation unrelated to this contract.

## 9. Verification matrix

After GREEN and after every repair that changes owned product behavior:

```text
cd apps/macos && swift test
cd apps/macos && swift build -c release
cd apps/macos && swift test --sanitize=thread
git diff --check
owned-file scope audit
Swift dependency/import audit
forbidden-visible-copy audit
raw-ID/credential/terminal-control negative audit
light/dark semantic-color source audit
keyboard/accessibility identifier audit
non-connecting UI render matrix
```

The complete existing Go repository, race, vet, module, installer, and live
transaction matrices are not repeated for a Swift-only presentation change
unless impact analysis finds a changed cross-language or packaging edge.

## 10. Non-connecting visual audit

No app executable or native window may be launched by this contract.

After deterministic GREEN and Implementation Review `PASS`, the real
`ContentView` from `LoomLocalAppUI` is rendered offscreen through the same
in-memory stub boundary used by tests. It cannot construct
`LocalIPCClient.defaultClient()` and cannot reach any socket. One representative
populated Home render may be saved as the owned preview PNG and inspected
visually. The complete state and appearance matrix remains deterministic test
evidence rather than a folder of golden screenshots.

The audit checks hierarchy, clipping, contrast intent, density, source-ordered
content, and absence of copied branding or fake actions. It is not called
Computer Use, a live canary, installed-product proof, or end-to-end
accessibility proof.

## 11. Review and exit

1. Fresh independent Contract Review must return `PASS`.
2. The contract status becomes `FROZEN`.
3. RED is captured.
4. Minimal owned implementation and deterministic matrix become GREEN.
5. Fresh independent Implementation Review returns `PASS`.
6. The bounded non-connecting visual audit passes.

This exits only the native product-experience reopen. It does not accept
P2A-W1 live delivery, permit a P2A-W1 commit, unlock P2A-W2, or authorize a new
live canary.

Stop for a reviewed replacement of this single contract, not another point
Amendment, if an unowned file or authority boundary is required. Stop
`HUMAN_REQUIRED` if the same blocker survives three governed attempts.
