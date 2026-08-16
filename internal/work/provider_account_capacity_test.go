package work

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestProviderAccountCapacityClaimFreezesPolicyAndReleasesOnTerminal(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 4)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xa1)
	policy := configureProviderAccountCapacityPolicy(
		t, authority, "deepseek", "deepseek.work", 1, 10, 100,
	)
	clock.Set(testNow.Add(time.Second))
	policyCommand := providerAccountPolicyTestInput("deepseek", "deepseek.work", 1)
	policyCommand.CommandID = "configure-deepseek-work-disclosure-v2"
	policyCommand.MaximumConcurrentAttempts = 1
	policyCommand.MaximumDispatchStarts = 10
	policyCommand.MaximumAssignedBudgetUnits = 100
	policyCommand.TrustDomain = "external_provider"
	policyCommand.RetentionMode = "zero_data_retention"
	policyCommand.DataRegion = "apac"
	policy, err := authority.ConfigureProviderAccountPolicy(ctx, policyCommand)
	if err != nil {
		t.Fatal(err)
	}
	first := providerAccountCapacityAssignment(
		t, "work-account-1", "run-account-1", "deepseek.work", 60,
	)
	second := providerAccountCapacityAssignment(
		t, "work-account-2", "run-account-2", "deepseek.work", 40,
	)
	if _, _, err := authority.CreateAndAssign(ctx, first); err != nil {
		t.Fatal(err)
	}
	if _, _, err := authority.CreateAndAssign(ctx, second); err != nil {
		t.Fatal(err)
	}
	_, claimed, err := authority.Claim(
		ctx, claim(first.WorkItemID, first.RunID, "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	revision, digest, governed := claimed.ProviderAccountPolicy()
	if !governed || revision != policy.Revision() || digest != policy.Digest() {
		t.Fatalf("claimed policy = %d %q governed=%t", revision, digest, governed)
	}
	version, trustDomain, retentionMode, dataRegion, governed :=
		claimed.ProviderAccountDisclosurePolicy()
	if !governed || version != 2 || trustDomain != "external_provider" ||
		retentionMode != "zero_data_retention" || dataRegion != "apac" {
		t.Fatalf(
			"claimed disclosure = v%d %q %q %q governed=%t",
			version, trustDomain, retentionMode, dataRegion, governed,
		)
	}
	if _, _, err := authority.Claim(
		ctx, claim(second.WorkItemID, second.RunID, "runtime-a"),
	); !errors.Is(err, ErrProviderAccountConcurrencyExhausted) {
		t.Fatalf("shared account concurrency error = %v", err)
	}

	if _, _, err := authority.CommitTerminal(ctx, RunTerminalInput{
		RunGenerationInput: generationInput(claimed),
		Status:             "cancelled", Reason: "operator_cancelled",
	}); err != nil {
		t.Fatal(err)
	}
	_, next, err := authority.Claim(
		ctx, claim(second.WorkItemID, second.RunID, "runtime-a"),
	)
	if err != nil {
		t.Fatalf("claim after exact release = %v", err)
	}
	if revision, digest, ok := next.ProviderAccountPolicy(); !ok ||
		revision != policy.Revision() || digest != policy.Digest() {
		t.Fatalf("second policy = %d %q %t", revision, digest, ok)
	}

	events, err := store.ReadStream(
		ctx, providerAccountCapacityStream("deepseek.work"),
	)
	if err != nil || len(events) != 3 ||
		events[0].Type != "ProviderAccountCapacityReserved" ||
		events[1].Type != "ProviderAccountCapacityReleased" ||
		events[2].Type != "ProviderAccountCapacityReserved" {
		t.Fatalf("account capacity events = %#v, err=%v", events, err)
	}
	for _, event := range events {
		body := strings.ToLower(string(event.PayloadJSON))
		for _, forbidden := range []string{
			"credential-ref-", "api_key", "authorization", "secret", "prompt",
		} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("account capacity payload contains %q: %s", forbidden, body)
			}
		}
	}
}

func TestProviderAccountCapacityEnforcesBudgetRateAndAccountIsolation(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 8)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xa2)
	configureProviderAccountCapacityPolicy(
		t, authority, "deepseek", "deepseek.work", 4, 1, 100,
	)
	configureProviderAccountCapacityPolicy(
		t, authority, "deepseek", "deepseek.review", 4, 10, 50,
	)

	work := providerAccountCapacityAssignment(
		t, "work-rate-1", "run-rate-1", "deepseek.work", 60,
	)
	workRate := providerAccountCapacityAssignment(
		t, "work-rate-2", "run-rate-2", "deepseek.work", 20,
	)
	review := providerAccountCapacityAssignment(
		t, "work-review-1", "run-review-1", "deepseek.review", 50,
	)
	reviewBudget := providerAccountCapacityAssignment(
		t, "work-review-2", "run-review-2", "deepseek.review", 1,
	)
	for _, input := range []WorkItemAssignmentInput{work, workRate, review, reviewBudget} {
		if _, _, err := authority.CreateAndAssign(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	_, workRun, err := authority.Claim(
		ctx, claim(work.WorkItemID, work.RunID, "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := authority.Claim(
		ctx, claim(workRate.WorkItemID, workRate.RunID, "runtime-a"),
	); !errors.Is(err, ErrProviderAccountDispatchRateExhausted) {
		t.Fatalf("dispatch rate error = %v", err)
	}
	_, reviewRun, err := authority.Claim(
		ctx, claim(review.WorkItemID, review.RunID, "runtime-a"),
	)
	if err != nil {
		t.Fatalf("peer account claim = %v", err)
	}
	if _, _, err := authority.Claim(
		ctx, claim(reviewBudget.WorkItemID, reviewBudget.RunID, "runtime-a"),
	); !errors.Is(err, ErrProviderAccountBudgetExhausted) {
		t.Fatalf("assigned budget error = %v", err)
	}
	if _, _, err := authority.CommitTerminal(ctx, RunTerminalInput{
		RunGenerationInput: generationInput(reviewRun),
		Status:             "cancelled", Reason: "operator_cancelled",
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := authority.Claim(
		ctx, claim(reviewBudget.WorkItemID, reviewBudget.RunID, "runtime-a"),
	); err != nil {
		t.Fatalf("budget release did not unblock peer Run: %v", err)
	}

	if _, _, err := authority.CommitTerminal(ctx, RunTerminalInput{
		RunGenerationInput: generationInput(workRun),
		Status:             "cancelled", Reason: "operator_cancelled",
	}); err != nil {
		t.Fatal(err)
	}
	clock.Set(testNow.Add(time.Minute + time.Second))
	if _, _, err := authority.Claim(
		ctx, claim(workRate.WorkItemID, workRate.RunID, "runtime-a"),
	); err != nil {
		t.Fatalf("expired dispatch window did not reopen: %v", err)
	}
}

func TestProviderAccountCapacityPolicyDriftAffectsOnlyFutureClaims(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 4)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xa3)
	configureProviderAccountCapacityPolicy(
		t, authority, "deepseek", "deepseek.work", 2, 10, 100,
	)
	clock.Set(testNow.Add(time.Second))
	firstCommand := providerAccountPolicyTestInput("deepseek", "deepseek.work", 1)
	firstCommand.CommandID = "configure-deepseek-work-disclosure-v2"
	firstCommand.MaximumConcurrentAttempts = 2
	firstCommand.MaximumDispatchStarts = 10
	firstCommand.MaximumAssignedBudgetUnits = 100
	firstCommand.TrustDomain = "external_provider"
	firstCommand.RetentionMode = "provider_default"
	firstCommand.DataRegion = "global"
	firstPolicy, err := authority.ConfigureProviderAccountPolicy(ctx, firstCommand)
	if err != nil {
		t.Fatal(err)
	}
	first := providerAccountCapacityAssignment(
		t, "work-policy-1", "run-policy-1", "deepseek.work", 40,
	)
	second := providerAccountCapacityAssignment(
		t, "work-policy-2", "run-policy-2", "deepseek.work", 40,
	)
	for _, input := range []WorkItemAssignmentInput{first, second} {
		if _, _, err := authority.CreateAndAssign(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	_, firstRun, err := authority.Claim(
		ctx, claim(first.WorkItemID, first.RunID, "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}

	clock.Set(testNow.Add(2 * time.Second))
	command := providerAccountPolicyTestInput(
		"deepseek", "deepseek.work", firstPolicy.Revision(),
	)
	command.CommandID = "tighten-deepseek-work-v2"
	command.MaximumConcurrentAttempts = 1
	command.MaximumDispatchStarts = 10
	command.MaximumAssignedBudgetUnits = 100
	command.TrustDomain = "enterprise_tenant"
	command.RetentionMode = "limited_retention"
	command.DataRegion = "eu"
	secondPolicy, err := authority.ConfigureProviderAccountPolicy(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := authority.Claim(
		ctx, claim(second.WorkItemID, second.RunID, "runtime-a"),
	); !errors.Is(err, ErrProviderAccountConcurrencyExhausted) {
		t.Fatalf("tightened future claim error = %v", err)
	}
	revision, digest, governed := firstRun.ProviderAccountPolicy()
	if !governed || revision != firstPolicy.Revision() ||
		digest != firstPolicy.Digest() || digest == secondPolicy.Digest() {
		t.Fatalf("existing Run policy drifted: %d %q %t", revision, digest, governed)
	}
	version, trustDomain, retentionMode, dataRegion, governed :=
		firstRun.ProviderAccountDisclosurePolicy()
	if !governed || version != firstPolicy.Version() ||
		trustDomain != "external_provider" ||
		retentionMode != "provider_default" || dataRegion != "global" ||
		trustDomain == secondPolicy.TrustDomain() ||
		retentionMode == secondPolicy.RetentionMode() ||
		dataRegion == secondPolicy.DataRegion() {
		t.Fatalf(
			"existing Run disclosure drifted: v%d %q %q %q governed=%t",
			version, trustDomain, retentionMode, dataRegion, governed,
		)
	}
	if _, _, err := authority.Start(ctx, generationInput(firstRun)); err != nil {
		t.Fatalf("policy drift blocked admitted Run start: %v", err)
	}
}

func TestProviderAccountCapacityCASLeavesNoPartialRuntimeReservation(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 1)
	seedRuntime(t, store, "runtime-b", "online", 1)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xa4)
	configureProviderAccountCapacityPolicy(
		t, authority, "deepseek", "deepseek.work", 1, 10, 100,
	)
	first := providerAccountCapacityAssignmentForRuntime(
		t, "work-cas-1", "run-cas-1", "runtime-a", "deepseek.work", 40,
	)
	second := providerAccountCapacityAssignmentForRuntime(
		t, "work-cas-2", "run-cas-2", "runtime-b", "deepseek.work", 40,
	)
	for _, input := range []WorkItemAssignmentInput{first, second} {
		if _, _, err := authority.CreateAndAssign(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	type result struct {
		input WorkItemAssignmentInput
		err   error
	}
	results := make(chan result, 2)
	var wait sync.WaitGroup
	for _, pair := range []struct {
		input     WorkItemAssignmentInput
		runtimeID string
	}{{first, "runtime-a"}, {second, "runtime-b"}} {
		wait.Add(1)
		go func(input WorkItemAssignmentInput, runtimeID string) {
			defer wait.Done()
			_, _, err := authority.Claim(
				ctx, claim(input.WorkItemID, input.RunID, runtimeID),
			)
			results <- result{input: input, err: err}
		}(pair.input, pair.runtimeID)
	}
	wait.Wait()
	close(results)
	var loser WorkItemAssignmentInput
	succeeded := 0
	conflicted := 0
	for current := range results {
		switch {
		case current.err == nil:
			succeeded++
		case errors.Is(current.err, ErrRunAuthorityConflict),
			errors.Is(current.err, ErrProviderAccountConcurrencyExhausted):
			conflicted++
			loser = current.input
		default:
			t.Fatalf("concurrent Claim error = %v", current.err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("CAS results succeeded=%d conflicted=%d", succeeded, conflicted)
	}
	loserRuntime := loser.ExecutionBinding.RuntimeInstanceID
	if _, _, err := authority.Claim(
		ctx, claim(loser.WorkItemID, loser.RunID, loserRuntime),
	); !errors.Is(err, ErrProviderAccountConcurrencyExhausted) {
		t.Fatalf("loser retry error = %v", err)
	}
	accountEvents, err := store.ReadStream(
		ctx, providerAccountCapacityStream("deepseek.work"),
	)
	if err != nil || len(accountEvents) != 1 {
		t.Fatalf("account events = %d, err=%v", len(accountEvents), err)
	}
	runtimeA, _ := store.ReadStream(ctx, runtimeCapacityStream("runtime-a"))
	runtimeB, _ := store.ReadStream(ctx, runtimeCapacityStream("runtime-b"))
	if len(runtimeA)+len(runtimeB) != 1 {
		t.Fatalf("partial Runtime reservations: a=%d b=%d", len(runtimeA), len(runtimeB))
	}
	loserRun, _ := store.ReadStream(ctx, runStream(loser.RunID))
	if len(loserRun) != 0 {
		t.Fatalf("loser Run mutated without account reservation: %#v", loserRun)
	}
}

func TestProviderAccountCapacityAdmissionRejectsClockRegression(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 4)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xa5)
	configureProviderAccountCapacityPolicy(
		t, authority, "deepseek", "deepseek.work", 4, 10, 100,
	)
	input := providerAccountCapacityAssignment(
		t, "work-clock-regression", "run-clock-regression", "deepseek.work", 20,
	)
	if _, _, err := authority.CreateAndAssign(ctx, input); err != nil {
		t.Fatal(err)
	}
	clock.Set(testNow.Add(-time.Second))
	if _, _, err := authority.Claim(
		ctx, claim(input.WorkItemID, input.RunID, "runtime-a"),
	); !errors.Is(err, ErrRunAuthorityConflict) {
		t.Fatalf("clock-regressed account policy error = %v", err)
	}
}

func TestProviderAccountCapacitySelectiveReplayRejectsForgedOrphanIdentity(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 4)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xa6)
	policy := configureProviderAccountCapacityPolicy(
		t, authority, "deepseek", "deepseek.work", 4, 10, 100,
	)
	peer := providerAccountCapacityAssignmentForRuntime(
		t, "work-forged-peer", "run-forged-peer", "runtime-b",
		"deepseek.work", 20,
	)
	_, peerRun, err := authority.CreateAndAssign(ctx, peer)
	if err != nil {
		t.Fatal(err)
	}
	peerBinding, err := providerAccountCapacityBindingFor(
		peerRun,
		"22222222-2222-4222-8222-222222222222",
		1,
		"runtime-b",
		policy,
	)
	if err != nil {
		t.Fatal(err)
	}
	streamID := providerAccountCapacityStream("deepseek.work")
	forged := newEvent(
		"evt-forged-account-capacity",
		streamID,
		1,
		"ProviderAccountCapacityReserved",
		testNow,
		"33333333-3333-4333-8333-333333333333",
		"evt-forged-cause",
		peerBinding.payload(),
	)
	if _, err := store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: 0}},
		[]journal.Event{forged},
	); err != nil {
		t.Fatal(err)
	}

	input := providerAccountCapacityAssignment(
		t, "work-after-forgery", "run-after-forgery", "deepseek.work", 20,
	)
	if _, _, err := authority.CreateAndAssign(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, _, err := authority.Claim(
		ctx, claim(input.WorkItemID, input.RunID, "runtime-a"),
	); !errors.Is(err, ErrRunAuthorityConflict) {
		t.Fatalf("forged orphan capacity identity error = %v", err)
	}
}

func configureProviderAccountCapacityPolicy(
	t *testing.T,
	authority *Authority,
	providerID string,
	accountID string,
	concurrency int,
	dispatchStarts int,
	budget int64,
) ProviderAccountPolicy {
	t.Helper()
	command := providerAccountPolicyTestInput(providerID, accountID, 0)
	command.MaximumConcurrentAttempts = concurrency
	command.MaximumDispatchStarts = dispatchStarts
	command.MaximumAssignedBudgetUnits = budget
	policy, err := authority.ConfigureProviderAccountPolicy(
		context.Background(), command,
	)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func providerAccountCapacityAssignment(
	t *testing.T,
	workItemID string,
	runID string,
	accountID string,
	budget int64,
) WorkItemAssignmentInput {
	return providerAccountCapacityAssignmentForRuntime(
		t, workItemID, runID, "runtime-a", accountID, budget,
	)
}

func providerAccountCapacityAssignmentForRuntime(
	t *testing.T,
	workItemID string,
	runID string,
	runtimeID string,
	accountID string,
	budget int64,
) WorkItemAssignmentInput {
	t.Helper()
	input := assignment(workItemID, runID)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "profile-" + runID, AdapterType: "loom-native",
		ProviderID: "deepseek", ProviderAccountID: accountID,
		ModelID: "deepseek-chat", AuthMode: loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("a", 64),
		CredentialReference: "credential-ref-" + strings.ReplaceAll(accountID, ".", "-"),
		CredentialRevision:  7, RequiredCapabilities: []string{"chat", "tools"},
		Timeout: 90 * time.Second, Budget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: runtimeID, DeviceID: "device.local", AdapterType: "loom-native",
		DisplayName: "Loom Native", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"chat", "tools"}, Capacity: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	input.ExecutionBinding, err = loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	return input
}
