package work

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/attemptpayload"
)

func TestAttemptPayloadAuthorityFreezesAcceptedAndDeliveredFacts(t *testing.T) {
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 1)
	clock := &mutableClock{now: testNow}
	runAuthority := newAuthority(t, store, clock, 0x61)
	assignmentInput := assignment("work-payload", "run-payload")
	assignmentInput.ExecutionBinding = testFrozenExecutionBinding(
		t, "profile.payload", "deepseek", "deepseek.primary", "deepseek-chat",
		"credential-ref-deepseek-primary", 4,
	)
	_, _, err := runAuthority.CreateAndAssign(context.Background(), assignmentInput)
	if err != nil {
		t.Fatal(err)
	}
	claimInput := claim("work-payload", "run-payload", "runtime-a")
	_, run, err := runAuthority.Claim(context.Background(), claimInput)
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runAuthority.Start(context.Background(), generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAttemptPayloadAuthority(runAuthority)
	if err != nil {
		t.Fatal(err)
	}
	frozen := attemptPayloadAuthorityFixture(run, assignmentInput.ExecutionBinding)
	binding := attemptPayloadBindingFixture(frozen)
	foreignIncident := frozen
	foreignIncident.IncidentID = "22222222-2222-4222-8222-222222222222"
	if err := authority.Accept(
		context.Background(), foreignIncident, binding,
	); !errors.Is(err, ErrAttemptPayloadFactAuthority) {
		t.Fatalf("foreign Incident accepted = %v", err)
	}
	if err := authority.Accept(context.Background(), frozen, binding); err != nil {
		t.Fatal(err)
	}
	if err := authority.Accept(context.Background(), frozen, binding); err != nil {
		t.Fatalf("idempotent accept: %v", err)
	}
	fact, found, err := authority.Lookup(
		context.Background(), frozen, binding.CallID, binding.Sequence,
	)
	if err != nil || !found || fact.Binding != binding ||
		fact.Status != attemptpayload.FactAccepted || fact.Proof != "" {
		t.Fatalf("accepted fact = %#v, %t, %v", fact, found, err)
	}
	if err := authority.Deliver(
		context.Background(), frozen, binding, attemptpayload.DeliveryProof("http_write"),
	); !errors.Is(err, ErrInvalidAttemptPayloadFact) {
		t.Fatalf("weak proof = %v", err)
	}
	if err := authority.Deliver(
		context.Background(), frozen, binding, attemptpayload.ProofProviderContinuation,
	); err != nil {
		t.Fatal(err)
	}
	if err := authority.Deliver(
		context.Background(), frozen, binding, attemptpayload.ProofProviderContinuation,
	); err != nil {
		t.Fatalf("idempotent deliver: %v", err)
	}
	fact, found, err = authority.Lookup(
		context.Background(), frozen, binding.CallID, binding.Sequence,
	)
	if err != nil || !found || fact.Status != attemptpayload.FactDelivered ||
		fact.Proof != attemptpayload.ProofProviderContinuation {
		t.Fatalf("delivered fact = %#v, %t, %v", fact, found, err)
	}
	if _, _, err := authority.Lookup(
		nil, frozen, binding.CallID, binding.Sequence,
	); !errors.Is(err, ErrAttemptPayloadFactAuthority) {
		t.Fatalf("nil context lookup = %v", err)
	}
	if _, found, err := authority.Lookup(
		context.Background(), frozen, "call-context-2", 2,
	); err != nil || found {
		t.Fatalf("second call lookup = found=%t err=%v", found, err)
	}
	events, err := store.ReadStream(
		context.Background(), attemptPayloadFactStream(frozen),
	)
	if err != nil || len(events) != 2 || events[0].Type != "ToolResultAccepted" ||
		events[1].Type != "ToolResultDelivered" ||
		events[1].CausationID != events[0].ID {
		t.Fatalf("payload facts = %#v, %v", events, err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "persisted tool result body") {
		t.Fatal("Journal leaked Attempt payload content")
	}

	changed := binding
	changed.ContentDigest = strings.Repeat("e", 64)
	if err := authority.Accept(context.Background(), frozen, changed); !errors.Is(err, ErrAttemptPayloadFactConflict) {
		t.Fatalf("changed accepted fact = %v", err)
	}
	stale := frozen
	stale.ClaimGeneration++
	stale.Scope.ClaimGeneration++
	if _, _, err := authority.Lookup(
		context.Background(), stale, binding.CallID, binding.Sequence,
	); !errors.Is(err, ErrAttemptPayloadFactAuthority) {
		t.Fatalf("stale lookup = %v", err)
	}
}

func TestAttemptPayloadBindingContentTypesAreClosed(t *testing.T) {
	binding := attemptpayload.Binding{
		PayloadID: "payload-content-type",
		Scope: attemptpayload.Scope{
			ConversationID:         "conversation-content-type",
			WorkItemID:             "work-content-type",
			RunID:                  "run-content-type",
			ClaimGeneration:        1,
			RuntimeInstanceID:      "runtime-content-type",
			ExecutionBindingDigest: strings.Repeat("a", 64),
			CapsuleDigest:          strings.Repeat("b", 64),
		},
		CallID:        "call-content-type",
		Sequence:      1,
		ContentDigest: strings.Repeat("c", 64),
	}
	for _, contentType := range []string{
		attemptpayload.ContentTypeJSON,
		attemptpayload.ContentTypeTextUTF8,
	} {
		candidate := binding
		candidate.ContentType = contentType
		if !validAttemptPayloadBinding(candidate) {
			t.Fatalf("valid content type rejected: %q", contentType)
		}
	}
	for _, contentType := range []string{
		"",
		"text/plain",
		"text/plain; charset=UTF-8",
		"application/octet-stream",
		"*/*",
	} {
		candidate := binding
		candidate.ContentType = contentType
		if validAttemptPayloadBinding(candidate) {
			t.Fatalf("unexpected content type accepted: %q", contentType)
		}
	}
}

func TestAttemptPayloadAuthoritySupportsOrderedMultiCallFacts(t *testing.T) {
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 1)
	runAuthority := newAuthority(t, store, &mutableClock{now: testNow}, 0x62)
	assignmentInput := assignment("work-multi-payload", "run-multi-payload")
	assignmentInput.ExecutionBinding = testFrozenExecutionBinding(
		t, "profile.multi-payload", "deepseek", "deepseek.primary", "deepseek-chat",
		"credential-ref-deepseek-primary", 4,
	)
	if _, _, err := runAuthority.CreateAndAssign(context.Background(), assignmentInput); err != nil {
		t.Fatal(err)
	}
	_, run, err := runAuthority.Claim(
		context.Background(), claim("work-multi-payload", "run-multi-payload", "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runAuthority.Start(context.Background(), generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAttemptPayloadAuthority(runAuthority)
	if err != nil {
		t.Fatal(err)
	}
	frozen := attemptPayloadAuthorityFixture(run, assignmentInput.ExecutionBinding)
	first := attemptPayloadBindingFixture(frozen)
	second := first
	second.PayloadID = "payload-journal-2"
	second.CallID = "call-context-2"
	second.Sequence = 2
	second.ContentDigest = strings.Repeat("e", 64)

	for _, binding := range []attemptpayload.Binding{first, second} {
		if err := authority.Accept(context.Background(), frozen, binding); err != nil {
			t.Fatalf("Accept(%s) error = %v", binding.CallID, err)
		}
		if err := authority.Deliver(
			context.Background(), frozen, binding, attemptpayload.ProofProviderContinuation,
		); err != nil {
			t.Fatalf("Deliver(%s) error = %v", binding.CallID, err)
		}
	}
	changedSecond := second
	changedSecond.ContentDigest = strings.Repeat("f", 64)
	if err := authority.Accept(
		context.Background(), frozen, changedSecond,
	); !errors.Is(err, ErrAttemptPayloadFactConflict) {
		t.Fatalf("second-call digest substitution = %v", err)
	}
	changedSequence := second
	changedSequence.CallID = "call-context-substituted"
	if err := authority.Accept(
		context.Background(), frozen, changedSequence,
	); !errors.Is(err, ErrAttemptPayloadFactConflict) {
		t.Fatalf("second-call sequence substitution = %v", err)
	}
	for _, binding := range []attemptpayload.Binding{first, second} {
		fact, found, lookupErr := authority.Lookup(
			context.Background(), frozen, binding.CallID, binding.Sequence,
		)
		if lookupErr != nil || !found || fact.Binding != binding ||
			fact.Status != attemptpayload.FactDelivered {
			t.Fatalf("Lookup(%s) = %#v, %t, %v", binding.CallID, fact, found, lookupErr)
		}
	}
	if attemptPayloadFactStreamFor(frozen, first.CallID, first.Sequence) ==
		attemptPayloadFactStreamFor(frozen, second.CallID, second.Sequence) {
		t.Fatal("ordered ToolCalls share one authority stream")
	}
	report, err := authority.deliveredFacts(context.Background())
	if err != nil || len(report) != 2 {
		t.Fatalf("delivered facts = %#v, %v", report, err)
	}
}

type attemptPayloadReconciliationStore struct {
	payload attemptpayload.Payload
	marks   int
	readErr error
}

type cancellingAttemptPayloadReconciliationStore struct {
	payloads map[attemptpayload.Binding]attemptpayload.Payload
	cancel   context.CancelFunc
	reads    int
}

func (*cancellingAttemptPayloadReconciliationStore) PutAttemptPayload(
	context.Context,
	attemptpayload.Payload,
) error {
	return errors.New("unexpected payload write")
}

func (store *cancellingAttemptPayloadReconciliationStore) ReadAttemptPayload(
	_ context.Context,
	binding attemptpayload.Binding,
) (attemptpayload.Payload, error) {
	payload, found := store.payloads[binding]
	if !found {
		return attemptpayload.Payload{}, attemptpayload.ErrPayloadNotFound
	}
	store.reads++
	if store.reads == 1 {
		store.cancel()
	}
	payload.Content = append([]byte(nil), payload.Content...)
	return payload, nil
}

func (*cancellingAttemptPayloadReconciliationStore) ListPendingAttemptPayloads(
	context.Context,
	attemptpayload.Scope,
) ([]attemptpayload.Payload, error) {
	return nil, errors.New("unexpected pending payload list")
}

func (*cancellingAttemptPayloadReconciliationStore) MarkAttemptPayloadDelivered(
	context.Context,
	attemptpayload.Binding,
) error {
	return nil
}

func (*attemptPayloadReconciliationStore) PutAttemptPayload(
	context.Context,
	attemptpayload.Payload,
) error {
	return errors.New("unexpected payload write")
}

func (store *attemptPayloadReconciliationStore) ReadAttemptPayload(
	_ context.Context,
	binding attemptpayload.Binding,
) (attemptpayload.Payload, error) {
	if store.readErr != nil {
		return attemptpayload.Payload{}, store.readErr
	}
	if store.payload.Binding != binding {
		return attemptpayload.Payload{}, attemptpayload.ErrPayloadNotFound
	}
	payload := store.payload
	payload.Content = append([]byte(nil), payload.Content...)
	return payload, nil
}

func (*attemptPayloadReconciliationStore) ListPendingAttemptPayloads(
	context.Context,
	attemptpayload.Scope,
) ([]attemptpayload.Payload, error) {
	return nil, errors.New("unexpected pending payload list")
}

func (store *attemptPayloadReconciliationStore) MarkAttemptPayloadDelivered(
	_ context.Context,
	binding attemptpayload.Binding,
) error {
	if store.payload.Binding != binding ||
		store.payload.Status != attemptpayload.StatusPending {
		return errors.New("unexpected delivered transition")
	}
	store.marks++
	store.payload.Status = attemptpayload.StatusDelivered
	return nil
}

func TestAttemptPayloadAuthorityReconcilesDeliveredFactAfterRunTerminal(t *testing.T) {
	ctx := context.Background()
	journalStore := openAuthorityStore(t)
	seedRuntime(t, journalStore, "runtime-a", "online", 1)
	clock := &mutableClock{now: testNow}
	runAuthority := newAuthority(t, journalStore, clock, 0x71)
	assignmentInput := assignment("work-reconcile", "run-reconcile")
	assignmentInput.ExecutionBinding = testFrozenExecutionBinding(
		t, "profile.reconcile", "deepseek", "deepseek.primary", "deepseek-chat",
		"credential-ref-deepseek-primary", 4,
	)
	if _, _, err := runAuthority.CreateAndAssign(ctx, assignmentInput); err != nil {
		t.Fatal(err)
	}
	_, run, err := runAuthority.Claim(
		ctx, claim("work-reconcile", "run-reconcile", "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runAuthority.Start(ctx, generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAttemptPayloadAuthority(runAuthority)
	if err != nil {
		t.Fatal(err)
	}
	frozen := attemptPayloadAuthorityFixture(run, assignmentInput.ExecutionBinding)
	binding := attemptPayloadBindingFixture(frozen)
	if err := authority.Accept(ctx, frozen, binding); err != nil {
		t.Fatal(err)
	}
	if err := authority.Deliver(
		ctx, frozen, binding, attemptpayload.ProofProviderContinuation,
	); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runAuthority.CommitTerminal(ctx, RunTerminalInput{
		RunGenerationInput: generationInput(run),
		Status:             "cancelled",
		Reason:             "operator_cancelled",
	}); err != nil {
		t.Fatal(err)
	}
	payloadStore := &attemptPayloadReconciliationStore{payload: attemptpayload.Payload{
		Binding: binding, Status: attemptpayload.StatusPending,
		Content: []byte("persisted result body"),
	}}
	report, err := authority.ReconcileDelivered(ctx, payloadStore)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Outcomes) != 1 ||
		report.Outcomes[0].Result != AttemptPayloadReconcileRepaired ||
		report.Outcomes[0].Binding != binding ||
		report.Outcomes[0].Authority != frozen ||
		report.Outcomes[0].ExecutionBinding.BindingDigest !=
			assignmentInput.ExecutionBinding.BindingDigest ||
		payloadStore.marks != 1 ||
		payloadStore.payload.Status != attemptpayload.StatusDelivered {
		t.Fatalf("reconciliation = %#v store=%#v", report, payloadStore)
	}
	report, err = authority.ReconcileDelivered(ctx, payloadStore)
	if err != nil || len(report.Outcomes) != 1 ||
		report.Outcomes[0].Result != AttemptPayloadReconcileAlreadyDelivered ||
		payloadStore.marks != 1 {
		t.Fatalf("idempotent reconciliation = %#v marks=%d err=%v", report, payloadStore.marks, err)
	}

	payloadStore.readErr = errors.New("one encrypted row is unavailable")
	report, err = authority.ReconcileDelivered(ctx, payloadStore)
	if err != nil || len(report.Outcomes) != 1 ||
		report.Outcomes[0].Result != AttemptPayloadReconcileBlocked ||
		report.Outcomes[0].ErrorCode != "attempt_payload_unavailable" {
		t.Fatalf("isolated reconciliation failure = %#v err=%v", report, err)
	}
}

func TestAttemptPayloadReconciliationValidatesOneFrozenAuthorityOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	journalStore := openAuthorityStore(t)
	seedRuntime(t, journalStore, "runtime-a", "online", 1)
	runAuthority := newAuthority(t, journalStore, &mutableClock{now: testNow}, 0x73)
	input := assignment("work-reconcile-shared", "run-reconcile-shared")
	input.ExecutionBinding = testFrozenExecutionBinding(
		t, "profile.reconcile-shared", "deepseek", "deepseek.primary",
		"deepseek-chat", "credential-ref-deepseek-primary", 4,
	)
	if _, _, err := runAuthority.CreateAndAssign(ctx, input); err != nil {
		t.Fatal(err)
	}
	_, run, err := runAuthority.Claim(
		ctx, claim("work-reconcile-shared", "run-reconcile-shared", "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runAuthority.Start(ctx, generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAttemptPayloadAuthority(runAuthority)
	if err != nil {
		t.Fatal(err)
	}
	frozen := attemptPayloadAuthorityFixture(run, input.ExecutionBinding)
	first := attemptPayloadBindingFixture(frozen)
	second := first
	second.PayloadID = "payload-journal-2"
	second.CallID = "call-context-2"
	second.Sequence = 2
	second.ContentDigest = strings.Repeat("e", 64)
	for _, binding := range []attemptpayload.Binding{first, second} {
		if err := authority.Accept(ctx, frozen, binding); err != nil {
			t.Fatal(err)
		}
		if err := authority.Deliver(
			ctx, frozen, binding, attemptpayload.ProofProviderContinuation,
		); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := runAuthority.CommitTerminal(ctx, RunTerminalInput{
		RunGenerationInput: generationInput(run),
		Status:             "cancelled",
		Reason:             "operator_cancelled",
	}); err != nil {
		t.Fatal(err)
	}
	store := &cancellingAttemptPayloadReconciliationStore{
		payloads: map[attemptpayload.Binding]attemptpayload.Payload{
			first:  {Binding: first, Status: attemptpayload.StatusDelivered},
			second: {Binding: second, Status: attemptpayload.StatusDelivered},
		},
		cancel: cancel,
	}
	report, err := authority.ReconcileDelivered(ctx, store)
	if err != nil || len(report.Outcomes) != 2 || store.reads != 2 {
		t.Fatalf("shared-authority reconciliation = %#v reads=%d err=%v", report, store.reads, err)
	}
	for _, outcome := range report.Outcomes {
		if outcome.Authority != frozen ||
			outcome.Result != AttemptPayloadReconcileAlreadyDelivered {
			t.Fatalf("shared-authority outcome = %#v", outcome)
		}
	}
}

type isolatedAttemptPayloadReconciliationStore struct {
	payloads   map[attemptpayload.Binding]attemptpayload.Payload
	readErrors map[attemptpayload.Binding]error
	marks      map[attemptpayload.Binding]int
}

func (*isolatedAttemptPayloadReconciliationStore) PutAttemptPayload(
	context.Context,
	attemptpayload.Payload,
) error {
	return errors.New("unexpected payload write")
}

func (store *isolatedAttemptPayloadReconciliationStore) ReadAttemptPayload(
	_ context.Context,
	binding attemptpayload.Binding,
) (attemptpayload.Payload, error) {
	if err := store.readErrors[binding]; err != nil {
		return attemptpayload.Payload{}, err
	}
	payload, found := store.payloads[binding]
	if !found {
		return attemptpayload.Payload{}, attemptpayload.ErrPayloadNotFound
	}
	payload.Content = append([]byte(nil), payload.Content...)
	return payload, nil
}

func (*isolatedAttemptPayloadReconciliationStore) ListPendingAttemptPayloads(
	context.Context,
	attemptpayload.Scope,
) ([]attemptpayload.Payload, error) {
	return nil, errors.New("unexpected pending payload list")
}

func (store *isolatedAttemptPayloadReconciliationStore) MarkAttemptPayloadDelivered(
	_ context.Context,
	binding attemptpayload.Binding,
) error {
	payload, found := store.payloads[binding]
	if !found {
		return attemptpayload.ErrPayloadNotFound
	}
	if payload.Status != attemptpayload.StatusPending {
		return errors.New("unexpected delivered transition")
	}
	store.marks[binding]++
	payload.Status = attemptpayload.StatusDelivered
	store.payloads[binding] = payload
	return nil
}

func TestAttemptPayloadReconciliationIsolatesOneCorruptPayload(t *testing.T) {
	ctx := context.Background()
	journalStore := openAuthorityStore(t)
	seedRuntime(t, journalStore, "runtime-a", "online", 1)
	clock := &mutableClock{now: testNow}
	runAuthority := newAuthority(t, journalStore, clock, 0x72)
	facts, err := NewAttemptPayloadAuthority(runAuthority)
	if err != nil {
		t.Fatal(err)
	}
	firstAuthority, firstBinding := terminalAttemptPayloadFactFixture(
		t, ctx, runAuthority, facts, "work-reconcile-a", "run-reconcile-a",
		"profile.reconcile-a", "deepseek.primary",
	)
	secondAuthority, secondBinding := terminalAttemptPayloadFactFixture(
		t, ctx, runAuthority, facts, "work-reconcile-b", "run-reconcile-b",
		"profile.reconcile-b", "deepseek.secondary",
	)
	store := &isolatedAttemptPayloadReconciliationStore{
		payloads: map[attemptpayload.Binding]attemptpayload.Payload{
			firstBinding: {
				Binding: firstBinding, Status: attemptpayload.StatusPending,
				Content: []byte("first encrypted result"),
			},
			secondBinding: {
				Binding: secondBinding, Status: attemptpayload.StatusPending,
				Content: []byte("second encrypted result"),
			},
		},
		readErrors: map[attemptpayload.Binding]error{
			firstBinding: errors.New("first encrypted row failed authentication"),
		},
		marks: make(map[attemptpayload.Binding]int),
	}
	report, err := facts.ReconcileDelivered(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Outcomes) != 2 || store.marks[firstBinding] != 0 ||
		store.marks[secondBinding] != 1 {
		t.Fatalf("isolated report=%#v marks=%#v", report, store.marks)
	}
	results := make(map[string]AttemptPayloadReconciliation)
	for _, outcome := range report.Outcomes {
		results[outcome.Binding.RunID] = outcome
	}
	if results[firstBinding.RunID].Result != AttemptPayloadReconcileBlocked ||
		results[firstBinding.RunID].ErrorCode != "attempt_payload_unavailable" ||
		results[firstBinding.RunID].Authority != firstAuthority ||
		results[secondBinding.RunID].Result != AttemptPayloadReconcileRepaired ||
		results[secondBinding.RunID].Authority != secondAuthority {
		t.Fatalf("isolated outcomes = %#v", results)
	}
}

func terminalAttemptPayloadFactFixture(
	t *testing.T,
	ctx context.Context,
	runs *Authority,
	facts *AttemptPayloadAuthority,
	workItemID,
	runID,
	profileID,
	providerAccountID string,
) (attemptpayload.Authority, attemptpayload.Binding) {
	t.Helper()
	input := assignment(workItemID, runID)
	input.ExecutionBinding = testFrozenExecutionBinding(
		t, profileID, "deepseek", providerAccountID, "deepseek-chat",
		"credential-ref-"+strings.ReplaceAll(providerAccountID, ".", "-"), 4,
	)
	if _, _, err := runs.CreateAndAssign(ctx, input); err != nil {
		t.Fatal(err)
	}
	_, run, err := runs.Claim(ctx, claim(workItemID, runID, "runtime-a"))
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runs.Start(ctx, generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	frozen := attemptPayloadAuthorityFixture(run, input.ExecutionBinding)
	binding := attemptPayloadBindingFixture(frozen)
	binding.PayloadID = "payload-" + runID
	binding.CallID = "call-" + runID
	if err := facts.Accept(ctx, frozen, binding); err != nil {
		t.Fatal(err)
	}
	if err := facts.Deliver(
		ctx, frozen, binding, attemptpayload.ProofProviderContinuation,
	); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runs.CommitTerminal(ctx, RunTerminalInput{
		RunGenerationInput: generationInput(run),
		Status:             "cancelled",
		Reason:             "operator_cancelled",
	}); err != nil {
		t.Fatal(err)
	}
	return frozen, binding
}

func attemptPayloadAuthorityFixture(
	run RunRecord,
	binding FrozenExecutionBinding,
) attemptpayload.Authority {
	return attemptpayload.Authority{
		Scope: attemptpayload.Scope{
			ConversationID: "mission:team-payload", WorkItemID: run.WorkItemID(),
			RunID: run.ID(), ClaimGeneration: run.ClaimGeneration(),
			RuntimeInstanceID:      run.RuntimeInstanceID(),
			ExecutionBindingDigest: binding.BindingDigest,
			CapsuleDigest:          strings.Repeat("c", 64),
		},
		ClaimID: run.ClaimID(), AgentInstanceID: run.AgentInstanceID(),
		IncidentID: testCorrelation,
	}
}

func attemptPayloadBindingFixture(authority attemptpayload.Authority) attemptpayload.Binding {
	return attemptpayload.Binding{
		PayloadID: "payload-journal-1", Scope: authority.Scope,
		CallID: "call-context-1", Sequence: 1, ContentType: attemptpayload.ContentTypeJSON,
		ContentDigest: strings.Repeat("d", 64),
	}
}
