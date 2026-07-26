# S5-W1 Contract-to-Test Traceability

Date: 2026-07-26
Baseline: `006db8c`

| Frozen boundary | Direct evidence |
|---|---|
| Journal exact head, merge, prefix, copy, gap, 96/128 bounds | `TestReadPageAfterHeadsReturnsDeterministicContiguousCopiedPrefix`; `TestReadPageAfterHeadsRejectsInvalidConflictGapBoundsAndCancellation` |
| Journal concurrent snapshot / current-or-next visibility | `TestReadPageAfterHeadsConcurrentAppendIsVisibleNowOrFromReturnedCursor` |
| Selective copied view access and old-view preservation | `TestGlobalReadViewReturnsOnlyRequestedTeamRecordsAsStableCopies`; `TestGlobalReadViewPublishesAtomicallyWithCanonicalVersionAndLocalCopies` |
| Cursor canonical JSON/base64, Team binding, own-scope digest | `TestTimelineCursorCanonicalRoundTripAndTeamBinding` |
| Cursor permutations, duplicate/unknown/unsorted/97-head/32-KiB rejection | `TestTimelineCursorRejectsNoncanonicalShapesAndEncodedOverflow` |
| Gap schema, two typed causes, copy, sanitized error | `TestTimelineGapErrorPreservesOnlySafeTypedCausesAndCopiedPage`; `TestReadPageRejectsMalformedCursorAsRecoverableGapWithoutJournalFallback` |
| Closed 20-kind authoritative vocabulary | `TestAuthoritativeKindVocabularyIsExact` |
| Fixed safe payload, duplicate/type/control rejection, raw-key exclusion | `TestAuthoritativeSafeFieldsRejectAmbiguityAndIgnoreRawKeys` |
| Journal source position and reconnect exact-once | `TestReadPageReturnsJournalAuthoritativeTimelineAndReconnectsWithoutDuplicate` performs 50 walks |
| Concurrent external append without position fabrication | `TestReadPageConcurrentAppendAdvancesOnlyOnCurrentOrNextCursor` under race repetition |
| Subscription Team existence, cursor position, cancellation, max 8 | `TestSubscribeRequiresTeamValidatesPositionAndBoundsSubscribers` |
| Coalescing only within exact Run/generation; 64 item/64-KiB overflow | `TestSubscriptionCoalescesTentativeTextAndSurfacesOverflowGapFirst` |
| 100 concurrent slow-consumer iterations and gap-first behavior | Same subscription test under `-race`; four producers per iteration |
| Known Journal milestone survives tentative overflow | Same subscription test performs a fresh authoritative page after every overflow set |
| Exact `{"delta":...}` format, 2048-byte/UTF-8/control bounds | `TestTentativeDeltaRequiresExactBoundedSafeJSON` |
| First-dispatch/rebound Projection freshness and zero-call failure | `TestTeamCoordinatorRefreshesProjectionBeforeAuthorizedObservation` |
| Capture-before-delivery, stale generation exclusion, non-persistence | `TestTeamExecutionStreamReceivesPostCaptureAuthorizedOutput` |
| Independent verifier execution and source-subscription exclusion | Same high-risk external integration executes distinct source/verifier Runtime adapters and rejects verifier tentative output from the source subscription |
| Source/verifier Run/Work/Evidence lineage, per-Run Grant cross-check, and board/Attention reconstruction | Same external integration requires all five source/verifier authoritative streams in the final page and succeeds only after exact Grant relation validation |
| CLI flags, exact wrapper, read-only SQLite, reconnect and gap exit 4 | `TestRunTimelineValidatesFlagsAndUsesFiniteInjectedRead`; `TestRunTimelineReadsAuthoritativeFactsReconnectsAndReturnsSafeGap` |
| CLI special path, cancellation, sanitization, no SQL/write | Same production CLI test plus `assertCLIDoesNotQueryJournalSQL` and `assertReadOnlyDBRejectsWrites` |
| No raw Grant/token/credential/hidden reasoning | closed mapper test, tentative non-persistence assertion, source/diff audit |
| No writer, migration, dependency, daemon, network, Runtime activation | scope/dependency/trust audit in `verification.md` |

The external integration uses high-risk acceptance and a distinct verifier
executor/Runtime. The Team observer is applied only to the source execution;
the verifier runs and produces private captured output, but its tentative
delta is neither published to the source subscription nor persisted in the
Journal.

VERDICT: PASS
