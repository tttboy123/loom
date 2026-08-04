# P3A-W1 Mandatory RED

Date: `2026-08-03`

Status: `COMPLETE — SURFACE, LINEAGE AND BEHAVIORAL RED CONFIRMED`

Contract authority: Gate 1 Contract Repair 2 Re-review `PASS`, `P0=P1=P2=0`.

No product implementation, daemon, Runtime, GUI/TUI journey, staging or commit
was performed.

## Test integrity

An initial combined attempt found one test-authoring error: missing `reflect`
import in `internal/work/run_authority_test.go`. That compile failure is excluded
from RED evidence. The import was corrected and `internal/work` was rerun; both
P3A tests then compiled and failed on the intended missing lineage fields.

## Confirmed RED command 1

```text
go test ./internal/assets ./internal/localipc ./internal/teams \
  ./internal/work ./internal/projection ./internal/runtime \
  -run '^TestP3A' -count=1
```

Confirmed intended failures:

- `internal/assets`: reviewed asset model/authority/canonical/replay source does
  not exist;
- `internal/localipc`: Request lacks strict `JourneyID` and P3A methods;
- `internal/teams`: `ExecutionNodeInput` lacks exact asset bindings/digest;
- `internal/work`: `RunRecord`, `TeamDispatchInput` and
  `TeamNodeSemanticBinding` lack exact asset/materialization lineage;
- `internal/projection`: GlobalReadView lacks typed evolution-asset accessors;
- `internal/runtime`: Pi probe config lacks explicit materialization
  conformance.

Representative exact output:

```text
TestP3AAssetDomainDeclaresReviewedAuthoritySurface:
  open authority.go: no such file or directory
TestP3AJourneyWireAndMethodsAreStrictlyAvailable:
  Request JourneyID field ... found=false
TestP3AExecutionPlanInputFreezesExactAssetRevisionSet:
  AssetRevisionBindings ... found=false
TestP3ARunRecordCarriesImmutableAssetAndMaterializationLineage:
  RunRecord field AssetRevisionBindings is missing
TestP3ATeamDispatchAndSemanticBindingCarryExactAssetSet:
  TeamDispatchInput field AssetRevisionBindings is missing
TestP3AGlobalReadViewExposesTypedEvolutionAssetCopies:
  GlobalReadView.EvolutionAssetDefinition is missing
TestP3APiProbeHasExplicitSkillMaterializationConformance:
  SkillMaterializationConformance ... found=false
```

## Confirmed RED command 2

```text
go test ./internal/assets ./internal/app ./internal/api ./internal/localipc \
  ./internal/runtime/piadapter ./internal/tui \
  -run '^TestP3A' -count=1
```

Confirmed intended failures:

- complete asset lifecycle authority surface absent;
- LocalProduct asset application service absent;
- strict asset API models absent;
- Go journey wire and Swift journey/asset production sources absent;
- private atomic Pi materializer absent;
- production TUI asset journey methods absent.

Representative exact output:

```text
TestP3AAssetAuthorityDeclaresCompleteVerticalLifecycle:
  open authority.go: no such file or directory
TestP3AApplicationServiceDeclaresProductionAssetJourney:
  open local_product_assets.go: no such file or directory
TestP3AAPIModelsDeclareStrictAssetWireSurface:
  open local_product_assets.go: no such file or directory
TestP3ASwiftProductionClientHasStrictJourneyAndAssetSurface:
  LocalIPCClient.swift missing "journey_id"
TestP3APrivateSkillMaterializerDeclaresAtomicLifecycle:
  open skill_materialization.go: no such file or directory
TestP3ATUIProductionClientExposesAssetJourneyMethods:
  DaemonReadClient.EvolutionAssetSnapshot is missing
```

## Confirmed behavioral RED command 3

```text
go test ./internal/assets ./internal/teams ./internal/work \
  ./internal/projection ./internal/runtime/piadapter ./internal/localipc \
  -run '^TestP3A' -count=1
```

All packages compiled. There was no compile error, missing import or test harness
panic. The command failed only on the intended absent behavior:

- unconfirmed import remains inactive and non-user lifecycle actors are denied;
- nonterminal/unaccepted Run or Evidence promotion has zero writes;
- stale view/head/revision/generation and wrong digest/subject have zero writes;
- activate/activate and activate/rollback races have one CAS winner;
- archive/restore/rollback preserve exact revision history;
- byte-identical redelivery is exact-once and operation drift conflicts;
- every Template kind produces only its typed Draft/Candidate output;
- canonical binding/subject JSON and reviewed Event identity golden bytes match;
- ExecutionPlan copies the exact binding set rather than aliasing input;
- dispatch reads bound revision and activation authority streams;
- malformed/duplicate P3A facts fail rebuild without replacing the prior view;
- incompatible Runtime, existing user target, interrupted publish and stale
  cleanup fail closed;
- manifests and Journal facts disclose no secret, raw Grant, prompt, hidden
  reasoning or token delta;
- production Unix socket `CallJourney` requires exact response identity;
- missing, invalid, uppercase, duplicate and unknown journey fields fail closed;
- the strict Swift production asset surface is required rather than mocked.

Representative exact output:

```text
TestP3AConcurrentActivationHasExactlyOneCASWinner:
  concurrent activation error = evolution asset authority not implemented
TestP3AExecutionPlanFreezesBindingBytesAcrossLaterInputMutation:
  frozen bindings = nil, want exact revision-1 binding
TestP3ATeamDispatchCASReadsEveryBoundAssetAuthorityStream:
  missing evolution-asset-revision/skill/skill-1/revision-1
TestP3AMalformedEvolutionAssetFactPreservesPublishedGlobalReadView:
  malformed evolution asset Rebuild() error = nil
TestP3APartialMaterializationIsAtomicAndRestartRecoverable:
  interrupted Materialize() error = Pi skill materialization not implemented
TestP3ACallJourneyUsesProductionSocketAndRejectsResponseIdentityDrift:
  CallJourney(valid) error = invalid local IPC protocol
TestP3AJourneyWireRejectsMissingDuplicateUnknownAndInvalidIdentity:
  missing journey_id error = nil
```

Existing deterministic `TestTeamDispatchCASHasOneWinnerAndIndependentAttemptLineage`
already supplies the base dispatch CAS race. The new P3A stream-set RED proves
the missing bound-asset heads that must enter that same transaction; no second
dispatch authority is introduced.

## Confirmed client/application surface RED command 4

```text
go test ./internal/app ./internal/api ./internal/runtime ./internal/tui \
  -run '^TestP3A' -count=1
```

All packages compiled and failed on the intended missing vertical surfaces:

```text
TestP3AApplicationServiceDeclaresProductionAssetJourney:
  local_product_assets.go: no such file
TestP3AAPIModelsDeclareStrictAssetWireSurface:
  local_product_assets.go: no such file
TestP3APiProbeHasExplicitSkillMaterializationConformance:
  SkillMaterializationConformance missing
TestP3ATUIProductionClientExposesAssetJourneyMethods:
  DaemonReadClient.EvolutionAssetSnapshot is missing
```

Together with the real Unix socket journey RED and strict Swift production
source RED, this closes the GUI/TUI same-protocol parity harness boundary. The
final native-window and PTY journey remains an Exit Gate and is not claimed by
RED.

## RED integrity verdict

Every parent Mandatory RED category now has an identifiable compile-clean
failing test. Production scaffolding added solely to make the tests compile
returns `ErrNotImplemented`, performs no Journal write, publishes no file,
wires no daemon and starts no client or Runtime.

VERDICT: `COMPLETE RED — PRODUCT IMPLEMENTATION MAY BEGIN`
