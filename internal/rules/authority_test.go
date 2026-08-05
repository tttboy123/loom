package rules

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

const (
	rulesTestCorrelation  = "11111111-1111-4111-8111-111111111111"
	rulesOtherCorrelation = "22222222-2222-4222-8222-222222222222"
	rulesTestContract     = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	rulesTestContinuation = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
)

var rulesTestNow = time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)

type rulesTestClock struct {
	now time.Time
}

func (clock *rulesTestClock) Now() time.Time {
	return clock.now
}

type rulesTestAuthorizer struct {
	mu        sync.Mutex
	now       time.Time
	actor     string
	deny      bool
	corrupt   bool
	activates int
	decisions int
}

func (authorizer *rulesTestAuthorizer) AuthorizeRuleSet(
	ctx context.Context,
	request RuleSetActivationRequest,
) (AuthorizedRuleSetActivation, error) {
	authorizer.mu.Lock()
	defer authorizer.mu.Unlock()
	authorizer.activates++
	if err := ctx.Err(); err != nil {
		return AuthorizedRuleSetActivation{}, err
	}
	if authorizer.deny {
		return AuthorizedRuleSetActivation{}, ErrCustomerAuthorizationDenied
	}
	commandDigest := ruleSetActivationCommandDigest(
		request.RuleSet(),
		request.CorrelationID(),
	)
	authorizationDigest := rulesTestAuthorizationDigest(
		"activate",
		commandDigest+rulesTestPresentationDigest(
			request.AuthorizationPresentation(),
		),
	)
	authorized, err := NewAuthorizedRuleSetActivation(
		request,
		authorizer.actor,
		authorizationDigest,
		authorizer.now,
		authorizer.now.Add(time.Minute),
	)
	if err != nil {
		return AuthorizedRuleSetActivation{}, err
	}
	if authorizer.corrupt {
		authorized.commandDigest = rulesTestContract
	}
	return authorized, nil
}

func (authorizer *rulesTestAuthorizer) AuthorizeApprovalDecision(
	ctx context.Context,
	request ApprovalDecisionRequest,
) (AuthorizedApprovalDecision, error) {
	authorizer.mu.Lock()
	defer authorizer.mu.Unlock()
	authorizer.decisions++
	if err := ctx.Err(); err != nil {
		return AuthorizedApprovalDecision{}, err
	}
	if authorizer.deny {
		return AuthorizedApprovalDecision{}, ErrCustomerAuthorizationDenied
	}
	commandDigest := approvalDecisionCommandDigest(
		request.ApprovalRequestID(),
		request.ApprovalRequestDigest(),
		request.Decision(),
		request.CorrelationID(),
	)
	authorizationDigest := rulesTestAuthorizationDigest(
		"decision",
		commandDigest+rulesTestPresentationDigest(
			request.AuthorizationPresentation(),
		),
	)
	authorized, err := NewAuthorizedApprovalDecision(
		request,
		authorizer.actor,
		authorizationDigest,
		authorizer.now,
		authorizer.now.Add(time.Minute),
	)
	if err != nil {
		return AuthorizedApprovalDecision{}, err
	}
	if authorizer.corrupt {
		authorized.commandDigest = rulesTestContract
	}
	return authorized, nil
}

func TestRuleSetEvaluationIsCanonicalImmutableAndFailClosed(t *testing.T) {
	project := mustRulesScope(t, "project", "project-1")
	workItem := mustRulesScope(t, "work_item", "work-1")
	projectSet := mustRuleSet(t, project, 1, []Rule{
		mustRule(t, "warn", "start_run", "high", "warn", "project-warning", nil, 0, ""),
		mustRule(t, "record", "start_run", "", "record", "audit-start", nil, 0, ""),
	})
	workSet := mustRuleSet(t, workItem, 1, []Rule{
		mustRule(
			t,
			"approval",
			"start_run",
			"high",
			"require_approval",
			"",
			[]string{"owner-b", "owner-a"},
			5*time.Minute,
			"reject",
		),
	})
	context := mustActionContext(t, "work-1", "run-1", "high")

	first, err := Evaluate([]RuleSet{projectSet, workSet}, context)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	second, err := Evaluate([]RuleSet{workSet, projectSet}, context)
	if err != nil {
		t.Fatalf("reordered Evaluate() error = %v", err)
	}
	if first.Kind() != "require_approval" ||
		first.Digest() != second.Digest() ||
		!reflect.DeepEqual(first.RuleSetReferences(), second.RuleSetReferences()) ||
		!reflect.DeepEqual(first.ApproverRefs(), []string{"owner-a", "owner-b"}) ||
		first.Timeout() != 5*time.Minute ||
		first.OnTimeout() != "reject" ||
		!reflect.DeepEqual(first.WarningMarkers(), []string{"project-warning"}) ||
		!reflect.DeepEqual(first.RecordMarkers(), []string{"audit-start"}) {
		t.Fatalf("decision first=%#v second=%#v", first, second)
	}

	approvers := first.ApproverRefs()
	approvers[0] = "mutated"
	references := first.RuleSetReferences()
	references[0].Digest = rulesTestContract
	if !reflect.DeepEqual(first.ApproverRefs(), []string{"owner-a", "owner-b"}) ||
		first.RuleSetReferences()[0].Digest == rulesTestContract {
		t.Fatal("Decision accessors exposed mutable state")
	}

	if _, err := Evaluate([]RuleSet{workSet, workSet}, context); !errors.Is(err, ErrInvalidRuleInput) {
		t.Fatalf("duplicate scope Evaluate() error = %v", err)
	}
	conflicting := mustRuleSet(t, workItem, 2, []Rule{
		mustRule(
			t,
			"approval",
			"start_run",
			"high",
			"require_approval",
			"",
			[]string{"owner-a"},
			time.Minute,
			"cancel",
		),
	})
	if _, err := Evaluate(
		[]RuleSet{projectSet, workSet, conflicting},
		context,
	); !errors.Is(err, ErrInvalidRuleInput) {
		t.Fatalf("stale duplicate scope Evaluate() error = %v", err)
	}

	team := mustRulesScope(t, "team", "team-1")
	workPackage := mustRulesScope(t, "work_package", "package-1")
	four := []RuleSet{
		projectSet,
		mustRuleSet(t, team, 1, []Rule{
			mustRule(t, "team-record", "start_run", "", "record", "team", nil, 0, ""),
		}),
		mustRuleSet(t, workPackage, 1, []Rule{
			mustRule(t, "package-record", "start_run", "", "record", "package", nil, 0, ""),
		}),
		workSet,
	}
	if _, err := Evaluate(four, context); err != nil {
		t.Fatalf("four-scope Evaluate() error = %v", err)
	}
	if _, err := Evaluate(append(four, projectSet), context); !errors.Is(err, ErrInvalidRuleInput) {
		t.Fatalf("five RuleSets Evaluate() error = %v", err)
	}

	conflictingApproval := mustRuleSet(t, team, 2, []Rule{
		mustRule(
			t,
			"team-approval",
			"start_run",
			"high",
			"require_approval",
			"",
			[]string{"owner-a"},
			time.Minute,
			"cancel",
		),
	})
	if _, err := Evaluate(
		[]RuleSet{workSet, conflictingApproval},
		context,
	); !errors.Is(err, ErrRuleEffectConflict) {
		t.Fatalf("conflicting approval policy error = %v", err)
	}

	rejecting := mustRuleSet(t, team, 3, []Rule{
		mustRule(
			t,
			"reject-start",
			"start_run",
			"high",
			"reject",
			"",
			nil,
			0,
			"",
		),
	})
	rejected, err := Evaluate([]RuleSet{workSet, rejecting}, context)
	if err != nil || rejected.Kind() != "reject" ||
		len(rejected.ApproverRefs()) != 0 ||
		rejected.Timeout() != 0 ||
		rejected.OnTimeout() != "" {
		t.Fatalf("reject precedence = %#v, %v", rejected, err)
	}
}

func TestRuleAuthorityActivationRequiresAuthorizationAndMonotonicCAS(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: rulesTestNow}
	authorizer := &rulesTestAuthorizer{
		now: rulesTestNow, actor: "local-owner",
	}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	ruleSet := mustRuleSet(t, mustRulesScope(t, "project", "project-1"), 1, []Rule{
		mustRule(t, "record", "start_run", "", "record", "audit", nil, 0, ""),
	})
	request := mustRuleSetActivationRequest(t, ruleSet)

	record, err := authority.ActivateRuleSet(
		context.Background(),
		request,
		rulesTestCorrelation,
	)
	if err != nil {
		t.Fatalf("ActivateRuleSet() error = %v", err)
	}
	if record.Digest() != ruleSet.Digest() ||
		record.ActorRef() != "local-owner" ||
		record.Revision() != 1 {
		t.Fatalf("activated record = %#v", record)
	}
	retry, err := authority.ActivateRuleSet(
		context.Background(),
		request,
		rulesTestCorrelation,
	)
	if err != nil || retry.Digest() != record.Digest() {
		t.Fatalf("exact retry = %#v, %v", retry, err)
	}
	clock.now = rulesTestNow.Add(2 * time.Minute)
	if expiredAuthorizationRetry, err := authority.ActivateRuleSet(
		context.Background(),
		request,
		rulesTestCorrelation,
	); err != nil ||
		expiredAuthorizationRetry.Digest() != record.Digest() {
		t.Fatalf(
			"expired-authorization exact retry = %#v, %v",
			expiredAuthorizationRetry,
			err,
		)
	}
	clock.now = rulesTestNow
	if authorizer.activates != 3 {
		t.Fatalf("authorizer activation calls = %d, want 3", authorizer.activates)
	}
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		request,
		rulesOtherCorrelation,
	); !errors.Is(err, ErrRuleAuthorityConflict) {
		t.Fatalf("divergent correlation retry error = %v", err)
	}
	differentPresentation, err := NewRuleSetActivationRequest(
		ruleSet,
		[]byte("different-opaque-authorization"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		differentPresentation,
		rulesTestCorrelation,
	); !errors.Is(err, ErrRuleAuthorityConflict) {
		t.Fatalf("divergent presentation retry error = %v", err)
	}

	authorizer.deny = true
	versionTwo := mustRuleSet(t, ruleSet.Scope(), 2, []Rule{
		mustRule(t, "warn", "start_run", "", "warn", "review", nil, 0, ""),
	})
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, versionTwo),
		rulesTestCorrelation,
	); !errors.Is(err, ErrCustomerAuthorizationDenied) {
		t.Fatalf("denied activation error = %v", err)
	}
	if events, err := store.ReadStream(context.Background(), ruleSet.StreamID()); err != nil ||
		len(events) != 1 {
		t.Fatalf("denied activation Events = %d, %v", len(events), err)
	}

	authorizer.deny = false
	authorizer.corrupt = true
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, versionTwo),
		rulesTestCorrelation,
	); !errors.Is(err, ErrCustomerAuthorizationDenied) {
		t.Fatalf("corrupt binding activation error = %v", err)
	}
	authorizer.corrupt = false

	secondRecord, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, versionTwo),
		rulesTestCorrelation,
	)
	if err != nil || secondRecord.Revision() != 2 {
		t.Fatalf("version-two activation = %#v, %v", secondRecord, err)
	}

	skipped := mustRuleSet(t, ruleSet.Scope(), 4, versionTwo.Rules())
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, skipped),
		rulesTestCorrelation,
	); !errors.Is(err, ErrRuleAuthorityConflict) {
		t.Fatalf("skipped revision error = %v", err)
	}
}

func TestApprovalLifecycleIsPreClaimAtomicRestartSafeAndTerminalOnce(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: rulesTestNow}
	authorizer := &rulesTestAuthorizer{
		now: rulesTestNow, actor: "local-owner",
	}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	workAuthority := seedRulesWorkItem(t, store, clock, "work-1", "run-1")

	ruleSet := mustRuleSet(t, mustRulesScope(t, "work_item", "work-1"), 1, []Rule{
		mustRule(
			t,
			"approve-start",
			"start_run",
			"high",
			"require_approval",
			"",
			[]string{"local-owner"},
			5*time.Minute,
			"reject",
		),
	})
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, ruleSet),
		rulesTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	action := mustActionContext(t, "work-1", "run-1", "high")
	decision, err := Evaluate([]RuleSet{ruleSet}, action)
	if err != nil {
		t.Fatal(err)
	}
	input := ApprovalRequestInput{
		Context:            action,
		ContinuationDigest: rulesTestContinuation,
		Decision:           decision,
		RequestedAt:        rulesTestNow,
		CorrelationID:      rulesTestCorrelation,
	}
	request, err := authority.RequestApproval(context.Background(), input)
	if err != nil {
		t.Fatalf("RequestApproval() error = %v", err)
	}
	if request.Status() != "pending" ||
		request.WorkItemID() != "work-1" ||
		request.ClaimGeneration() != 0 ||
		request.ContinuationDigest() != rulesTestContinuation {
		t.Fatalf("approval request = %#v", request)
	}
	retry, err := authority.RequestApproval(context.Background(), input)
	if err != nil || retry.ID() != request.ID() {
		t.Fatalf("RequestApproval() retry = %#v, %v", retry, err)
	}
	clock.now = rulesTestNow.Add(2 * time.Minute)
	lateRetry, err := authority.RequestApproval(context.Background(), input)
	clock.now = rulesTestNow
	if err != nil || lateRetry.ID() != request.ID() {
		t.Fatalf("late RequestApproval() retry = %#v, %v", lateRetry, err)
	}
	divergentInput := input
	divergentInput.CorrelationID = rulesOtherCorrelation
	if _, err := authority.RequestApproval(
		context.Background(),
		divergentInput,
	); !errors.Is(err, ErrApprovalAlreadyPending) {
		t.Fatalf("divergent RequestApproval() retry error = %v", err)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := countRulesEventTypes(events, "ApprovalRequested", "WorkItemApprovalPaused"); got != 2 {
		t.Fatalf("request lifecycle Event count = %d, want 2", got)
	}

	if _, _, err := workAuthority.Claim(
		context.Background(),
		work.RunClaimInput{
			WorkItemID:           "work-1",
			RunID:                "run-1",
			RuntimeInstanceID:    "runtime-1",
			AgentInstanceID:      "agent-1",
			PrepareLeaseDuration: time.Minute,
			CorrelationID:        rulesTestCorrelation,
		},
	); !errors.Is(err, work.ErrRunNotClaimable) {
		t.Fatalf("paused Claim() error = %v", err)
	}

	reopened := mustRulesAuthority(t, store, authorizer, clock)
	decisionRequest := mustApprovalDecisionRequest(
		t,
		request,
		"approved",
	)
	approved, err := reopened.DecideApproval(
		context.Background(),
		decisionRequest,
		rulesTestCorrelation,
	)
	if err != nil {
		t.Fatalf("DecideApproval() error = %v", err)
	}
	if approved.Status() != "approved" ||
		approved.DecisionActorRef() != "local-owner" ||
		approved.ResumeCandidate().ContinuationDigest() != rulesTestContinuation ||
		len(approved.ResumeCandidate().Heads()) != 7 {
		t.Fatalf("approved record = %#v", approved)
	}
	approvedHeads := approved.ResumeCandidate().Heads()
	var sawEmptyRunHead bool
	for index, head := range approvedHeads {
		if index > 0 &&
			approvedHeads[index-1].StreamID >= head.StreamID {
			t.Fatalf("resume heads not sorted: %#v", approvedHeads)
		}
		if head.StreamID == "run/run-1" &&
			head.Sequence == 0 &&
			head.EventID == "" {
			sawEmptyRunHead = true
		}
	}
	if !sawEmptyRunHead {
		t.Fatalf("resume heads lack frozen empty Run: %#v", approvedHeads)
	}
	approvedHeads[0].EventID = "mutated"
	if approved.ResumeCandidate().Heads()[0].EventID == "mutated" {
		t.Fatal("resume Candidate heads alias caller mutation")
	}
	decisionRetry, err := reopened.DecideApproval(
		context.Background(),
		decisionRequest,
		rulesTestCorrelation,
	)
	if err != nil || decisionRetry.Status() != "approved" {
		t.Fatalf("decision retry = %#v, %v", decisionRetry, err)
	}
	clock.now = rulesTestNow.Add(2 * time.Minute)
	expiredAuthorizationRetry, err := reopened.DecideApproval(
		context.Background(),
		decisionRequest,
		rulesTestCorrelation,
	)
	clock.now = rulesTestNow
	if err != nil || expiredAuthorizationRetry.Status() != "approved" {
		t.Fatalf(
			"expired-authorization decision retry = %#v, %v",
			expiredAuthorizationRetry,
			err,
		)
	}
	if !reflect.DeepEqual(
		expiredAuthorizationRetry.ResumeCandidate().Heads(),
		approved.ResumeCandidate().Heads(),
	) {
		t.Fatal("expired-authorization retry lost exact resume heads")
	}
	requestRetryAfterTerminal, err := reopened.RequestApproval(
		context.Background(),
		input,
	)
	if err != nil || !reflect.DeepEqual(
		requestRetryAfterTerminal.ResumeCandidate().Heads(),
		approved.ResumeCandidate().Heads(),
	) {
		t.Fatalf(
			"request retry after terminal = %#v, %v",
			requestRetryAfterTerminal,
			err,
		)
	}
	if !reflect.DeepEqual(
		decisionRetry.ResumeCandidate().Heads(),
		approved.ResumeCandidate().Heads(),
	) {
		t.Fatalf(
			"decision retry heads = %#v, want %#v",
			decisionRetry.ResumeCandidate().Heads(),
			approved.ResumeCandidate().Heads(),
		)
	}
	if _, err := reopened.DecideApproval(
		context.Background(),
		decisionRequest,
		rulesOtherCorrelation,
	); !errors.Is(err, ErrRuleAuthorityConflict) {
		t.Fatalf("divergent decision retry error = %v", err)
	}
	differentDecisionPresentation, err := NewApprovalDecisionRequest(
		request.ID(),
		request.Digest(),
		"approved",
		[]byte("different-opaque-authorization"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.DecideApproval(
		context.Background(),
		differentDecisionPresentation,
		rulesTestCorrelation,
	); !errors.Is(err, ErrRuleAuthorityConflict) {
		t.Fatalf("divergent decision presentation error = %v", err)
	}
	if authorizer.decisions != 5 {
		t.Fatalf("authorizer decision calls = %d, want 5", authorizer.decisions)
	}
	rejected := mustApprovalDecisionRequest(t, request, "rejected")
	if _, err := reopened.DecideApproval(
		context.Background(),
		rejected,
		rulesTestCorrelation,
	); !errors.Is(err, ErrApprovalAlreadyTerminal) {
		t.Fatalf("divergent terminal decision error = %v", err)
	}
}

func TestApprovalRequestRejectsClaimedRunAndConcurrentRequestClaimHasOneWinner(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: rulesTestNow}
	authorizer := &rulesTestAuthorizer{
		now: rulesTestNow, actor: "local-owner",
	}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	workAuthority := seedRulesWorkItem(t, store, clock, "work-2", "run-2")
	seedRulesRuntime(t, store, "runtime-1")

	ruleSet := mustRuleSet(t, mustRulesScope(t, "work_item", "work-2"), 1, []Rule{
		mustRule(
			t,
			"approve-start",
			"start_run",
			"high",
			"require_approval",
			"",
			[]string{"local-owner"},
			time.Minute,
			"cancel",
		),
	})
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, ruleSet),
		rulesTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	action := mustActionContext(t, "work-2", "run-2", "high")
	decision, err := Evaluate([]RuleSet{ruleSet}, action)
	if err != nil {
		t.Fatal(err)
	}
	input := ApprovalRequestInput{
		Context:            action,
		ContinuationDigest: rulesTestContinuation,
		Decision:           decision,
		RequestedAt:        rulesTestNow,
		CorrelationID:      rulesTestCorrelation,
	}

	if _, _, err := workAuthority.Claim(
		context.Background(),
		work.RunClaimInput{
			WorkItemID:           "work-2",
			RunID:                "run-2",
			RuntimeInstanceID:    "runtime-1",
			AgentInstanceID:      "agent-1",
			PrepareLeaseDuration: time.Minute,
			CorrelationID:        rulesTestCorrelation,
		},
	); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if _, err := authority.RequestApproval(
		context.Background(),
		input,
	); !errors.Is(err, ErrApprovalRunAlreadyClaimed) {
		t.Fatalf("post-claim RequestApproval() error = %v", err)
	}
}

func TestApprovalRequestCannotOmitCurrentHigherScopeRuleSet(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: rulesTestNow}
	authorizer := &rulesTestAuthorizer{
		now: rulesTestNow, actor: "local-owner",
	}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	seedRulesWorkItem(t, store, clock, "work-omission", "run-omission")
	projectReject := mustRuleSet(
		t,
		mustRulesScope(t, "project", "project-1"),
		1,
		[]Rule{mustRule(
			t,
			"reject-start",
			"start_run",
			"high",
			"reject",
			"",
			nil,
			0,
			"",
		)},
	)
	workApproval := mustRuleSet(
		t,
		mustRulesScope(t, "work_item", "work-omission"),
		1,
		[]Rule{mustRule(
			t,
			"approve-start",
			"start_run",
			"high",
			"require_approval",
			"",
			[]string{"local-owner"},
			time.Minute,
			"reject",
		)},
	)
	for _, ruleSet := range []RuleSet{projectReject, workApproval} {
		if _, err := authority.ActivateRuleSet(
			context.Background(),
			mustRuleSetActivationRequest(t, ruleSet),
			rulesTestCorrelation,
		); err != nil {
			t.Fatal(err)
		}
	}
	action := mustActionContext(
		t,
		"work-omission",
		"run-omission",
		"high",
	)
	omittingDecision, err := Evaluate([]RuleSet{workApproval}, action)
	if err != nil || omittingDecision.Kind() != "require_approval" {
		t.Fatalf("omitting Decision = %#v, %v", omittingDecision, err)
	}
	if _, err := authority.RequestApproval(
		context.Background(),
		ApprovalRequestInput{
			Context:            action,
			ContinuationDigest: rulesTestContinuation,
			Decision:           omittingDecision,
			RequestedAt:        rulesTestNow,
			CorrelationID:      rulesTestCorrelation,
		},
	); !errors.Is(err, ErrApprovalStale) {
		t.Fatalf("omitted current RuleSet error = %v", err)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := countRulesEventTypes(
		events,
		"ApprovalRequested",
		"WorkItemApprovalPaused",
	); got != 0 {
		t.Fatalf("omitted RuleSet wrote %d approval Events", got)
	}
}

func TestApprovalDecisionRejectsRuleSetChangedAfterRequest(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: rulesTestNow}
	authorizer := &rulesTestAuthorizer{
		now: rulesTestNow, actor: "local-owner",
	}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	seedRulesWorkItem(t, store, clock, "work-stale-rule", "run-stale-rule")
	scope := mustRulesScope(t, "work_item", "work-stale-rule")
	firstRuleSet := mustRuleSet(
		t,
		scope,
		1,
		[]Rule{mustRule(
			t,
			"approve-start",
			"start_run",
			"high",
			"require_approval",
			"",
			[]string{"local-owner"},
			time.Minute,
			"reject",
		)},
	)
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, firstRuleSet),
		rulesTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	action := mustActionContext(
		t,
		"work-stale-rule",
		"run-stale-rule",
		"high",
	)
	decision, err := Evaluate([]RuleSet{firstRuleSet}, action)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := authority.RequestApproval(
		context.Background(),
		ApprovalRequestInput{
			Context:            action,
			ContinuationDigest: rulesTestContinuation,
			Decision:           decision,
			RequestedAt:        rulesTestNow,
			CorrelationID:      rulesTestCorrelation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	secondRuleSet := mustRuleSet(t, scope, 2, firstRuleSet.Rules())
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, secondRuleSet),
		rulesTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.DecideApproval(
		context.Background(),
		mustApprovalDecisionRequest(t, pending, "approved"),
		rulesTestCorrelation,
	); !errors.Is(err, ErrApprovalStale) {
		t.Fatalf("changed RuleSet decision error = %v", err)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := countRulesEventTypes(
		events,
		"ApprovalDecided",
		"WorkItemApprovalResolved",
	); got != 0 {
		t.Fatalf("stale RuleSet decision wrote %d terminal Events", got)
	}
}

func TestConcurrentApprovalRequestAndClaimHasExactlyOneCASWinner(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: rulesTestNow}
	authorizer := &rulesTestAuthorizer{
		now: rulesTestNow, actor: "local-owner",
	}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	workAuthority := seedRulesWorkItem(
		t,
		store,
		clock,
		"work-race",
		"run-race",
	)
	seedRulesRuntime(t, store, "runtime-race")
	ruleSet := mustRuleSet(
		t,
		mustRulesScope(t, "work_item", "work-race"),
		1,
		[]Rule{mustRule(
			t,
			"approve-start",
			"start_run",
			"high",
			"require_approval",
			"",
			[]string{"local-owner"},
			time.Minute,
			"reject",
		)},
	)
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, ruleSet),
		rulesTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	action := mustActionContext(t, "work-race", "run-race", "high")
	decision, err := Evaluate([]RuleSet{ruleSet}, action)
	if err != nil {
		t.Fatal(err)
	}
	request := ApprovalRequestInput{
		Context:            action,
		ContinuationDigest: rulesTestContinuation,
		Decision:           decision,
		RequestedAt:        rulesTestNow,
		CorrelationID:      rulesTestCorrelation,
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, requestErr := authority.RequestApproval(
			context.Background(),
			request,
		)
		results <- requestErr
	}()
	go func() {
		defer wait.Done()
		<-start
		_, _, claimErr := workAuthority.Claim(
			context.Background(),
			work.RunClaimInput{
				WorkItemID:           "work-race",
				RunID:                "run-race",
				RuntimeInstanceID:    "runtime-race",
				AgentInstanceID:      "agent-1",
				PrepareLeaseDuration: time.Minute,
				CorrelationID:        rulesTestCorrelation,
			},
		)
		results <- claimErr
	}()
	close(start)
	wait.Wait()
	close(results)
	var successes int
	for result := range results {
		if result == nil {
			successes++
			continue
		}
		if !errors.Is(result, ErrRuleAuthorityConflict) &&
			!errors.Is(result, ErrApprovalRunAlreadyClaimed) &&
			!errors.Is(result, work.ErrRunAuthorityConflict) &&
			!errors.Is(result, work.ErrRunNotClaimable) {
			t.Fatalf("unexpected race loser error = %v", result)
		}
	}
	if successes != 1 {
		t.Fatalf("race successes = %d, want exactly 1", successes)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	approvalWon := countRulesEventTypes(
		events,
		"ApprovalRequested",
		"WorkItemApprovalPaused",
	) == 2
	claimWon := countRulesEventTypes(
		events,
		"RunClaimed",
		"RuntimeCapacityReserved",
	) == 2
	if approvalWon == claimWon {
		t.Fatalf(
			"race facts: approval_won=%v claim_won=%v",
			approvalWon,
			claimWon,
		)
	}
}

func TestConcurrentApprovalDecisionsHaveOneTerminalWinner(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: rulesTestNow}
	authorizer := &rulesTestAuthorizer{
		now: rulesTestNow, actor: "local-owner",
	}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	seedRulesWorkItem(
		t,
		store,
		clock,
		"work-decision-race",
		"run-decision-race",
	)
	ruleSet := mustRuleSet(
		t,
		mustRulesScope(t, "work_item", "work-decision-race"),
		1,
		[]Rule{mustRule(
			t,
			"approve-start",
			"start_run",
			"high",
			"require_approval",
			"",
			[]string{"local-owner"},
			time.Minute,
			"reject",
		)},
	)
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, ruleSet),
		rulesTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	action := mustActionContext(
		t,
		"work-decision-race",
		"run-decision-race",
		"high",
	)
	decision, err := Evaluate([]RuleSet{ruleSet}, action)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := authority.RequestApproval(
		context.Background(),
		ApprovalRequestInput{
			Context:            action,
			ContinuationDigest: rulesTestContinuation,
			Decision:           decision,
			RequestedAt:        rulesTestNow,
			CorrelationID:      rulesTestCorrelation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	commands := []ApprovalDecisionRequest{
		mustApprovalDecisionRequest(t, pending, "approved"),
		mustApprovalDecisionRequest(t, pending, "rejected"),
	}
	start := make(chan struct{})
	results := make(chan error, len(commands))
	var wait sync.WaitGroup
	for _, command := range commands {
		command := command
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, decisionErr := authority.DecideApproval(
				context.Background(),
				command,
				rulesTestCorrelation,
			)
			results <- decisionErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	var successes int
	for result := range results {
		if result == nil {
			successes++
			continue
		}
		if !errors.Is(result, ErrRuleAuthorityConflict) &&
			!errors.Is(result, ErrApprovalAlreadyTerminal) &&
			!errors.Is(result, ErrApprovalStale) {
			t.Fatalf("unexpected decision race loser error = %v", result)
		}
	}
	if successes != 1 {
		t.Fatalf("decision race successes = %d, want exactly 1", successes)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := countRulesEventTypes(
		events,
		"ApprovalDecided",
		"WorkItemApprovalResolved",
	); got != 2 {
		t.Fatalf("decision terminal Event count = %d, want 2", got)
	}
}

func TestApprovalExpiryAppliesBoundedTimeoutPolicyOnce(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: rulesTestNow}
	authorizer := &rulesTestAuthorizer{
		now: rulesTestNow, actor: "local-owner",
	}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	workAuthority := seedRulesWorkItem(
		t,
		store,
		clock,
		"work-expiry",
		"run-expiry",
	)
	ruleSet := mustRuleSet(
		t,
		mustRulesScope(t, "work_item", "work-expiry"),
		1,
		[]Rule{mustRule(
			t,
			"approve-start",
			"start_run",
			"high",
			"require_approval",
			"",
			[]string{"local-owner"},
			time.Minute,
			"cancel",
		)},
	)
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, ruleSet),
		rulesTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	action := mustActionContext(t, "work-expiry", "run-expiry", "high")
	decision, err := Evaluate([]RuleSet{ruleSet}, action)
	if err != nil {
		t.Fatal(err)
	}
	request, err := authority.RequestApproval(
		context.Background(),
		ApprovalRequestInput{
			Context:            action,
			ContinuationDigest: rulesTestContinuation,
			Decision:           decision,
			RequestedAt:        rulesTestNow,
			CorrelationID:      rulesTestCorrelation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ExpireApproval(
		context.Background(),
		request.ID(),
		rulesTestCorrelation,
	); !errors.Is(err, ErrApprovalExpired) {
		t.Fatalf("early ExpireApproval() error = %v", err)
	}
	clock.now = rulesTestNow.Add(time.Minute)
	expired, err := authority.ExpireApproval(
		context.Background(),
		request.ID(),
		rulesTestCorrelation,
	)
	if err != nil || expired.Status() != "expired" {
		t.Fatalf("ExpireApproval() = %#v, %v", expired, err)
	}
	retry, err := authority.ExpireApproval(
		context.Background(),
		request.ID(),
		rulesTestCorrelation,
	)
	if err != nil || retry.Status() != "expired" {
		t.Fatalf("ExpireApproval() retry = %#v, %v", retry, err)
	}
	if _, err := authority.ExpireApproval(
		context.Background(),
		request.ID(),
		rulesOtherCorrelation,
	); !errors.Is(err, ErrRuleAuthorityConflict) {
		t.Fatalf("divergent ExpireApproval() retry error = %v", err)
	}
	snapshot, err := workAuthority.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.WorkItems()) != 1 ||
		snapshot.WorkItems()[0].Status() != "cancelled" {
		t.Fatalf("expired WorkItem snapshot = %#v", snapshot.WorkItems())
	}
}

func TestRuleAuthorityRejectsTypedNilAndNonExactJSON(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: rulesTestNow}
	var typedNil *rulesTestAuthorizer
	if authority, err := NewAuthority(
		store,
		typedNil,
		clock.Now,
	); !errors.Is(err, ErrCustomerAuthorizationRequired) || authority != nil {
		t.Fatalf("typed-nil NewAuthority() = %v, %v", authority, err)
	}
	var target map[string]any
	if err := decodeExact([]byte(`{} {`), &target); err == nil {
		t.Fatal("decodeExact() accepted malformed trailing JSON")
	}

	authorizer := &rulesTestAuthorizer{
		now:   rulesTestNow,
		actor: "local-owner",
	}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	action := mustActionContext(t, "work-no-approval", "run-no-approval", "low")
	allow, err := Evaluate(nil, action)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.RequestApproval(
		context.Background(),
		ApprovalRequestInput{
			Context:            action,
			ContinuationDigest: rulesTestContinuation,
			Decision:           allow,
			RequestedAt:        rulesTestNow,
			CorrelationID:      rulesTestCorrelation,
		},
	); !errors.Is(err, ErrApprovalNotRequired) {
		t.Fatalf("allow RequestApproval() error = %v", err)
	}
}

func TestCustomerAuthorizerPortExposesBoundedCopiesAndBuildsExactResponses(t *testing.T) {
	ruleSet := mustRuleSet(
		t,
		mustRulesScope(t, "project", "project-1"),
		1,
		[]Rule{mustRule(
			t,
			"audit",
			"start_run",
			"",
			"record",
			"audit",
			nil,
			0,
			"",
		)},
	)
	activation := mustRuleSetActivationRequest(t, ruleSet)
	activation.correlationID = rulesTestCorrelation
	presentation := activation.AuthorizationPresentation()
	presentation[0] = 'X'
	if activation.RuleSet().Digest() != ruleSet.Digest() ||
		activation.CorrelationID() != rulesTestCorrelation ||
		string(activation.AuthorizationPresentation()) !=
			"opaque-local-authorization" {
		t.Fatalf("activation request accessors leaked mutation: %#v", activation)
	}
	authorizedActivation, err := NewAuthorizedRuleSetActivation(
		activation,
		"local-owner",
		rulesTestContract,
		rulesTestNow,
		rulesTestNow.Add(time.Minute),
	)
	if err != nil || !validAuthorizedActivation(
		authorizedActivation,
		activation,
		rulesTestNow,
	) {
		t.Fatalf(
			"NewAuthorizedRuleSetActivation() = %#v, %v",
			authorizedActivation,
			err,
		)
	}

	decisionRequest, err := NewApprovalDecisionRequest(
		"11111111-1111-4111-8111-111111111111",
		rulesTestContract,
		"approved",
		[]byte("opaque-local-authorization"),
	)
	if err != nil {
		t.Fatal(err)
	}
	decisionRequest.correlationID = rulesTestCorrelation
	decisionPresentation := decisionRequest.AuthorizationPresentation()
	decisionPresentation[0] = 'X'
	if decisionRequest.ApprovalRequestID() == "" ||
		decisionRequest.ApprovalRequestDigest() != rulesTestContract ||
		decisionRequest.Decision() != "approved" ||
		decisionRequest.CorrelationID() != rulesTestCorrelation ||
		string(decisionRequest.AuthorizationPresentation()) !=
			"opaque-local-authorization" {
		t.Fatalf("decision request accessors leaked mutation: %#v", decisionRequest)
	}
	authorizedDecision, err := NewAuthorizedApprovalDecision(
		decisionRequest,
		"local-owner",
		rulesTestContract,
		rulesTestNow,
		rulesTestNow.Add(time.Minute),
	)
	if err != nil || !validAuthorizedDecision(
		authorizedDecision,
		decisionRequest,
		rulesTestNow,
	) {
		t.Fatalf(
			"NewAuthorizedApprovalDecision() = %#v, %v",
			authorizedDecision,
			err,
		)
	}
}

func mustRulesScope(t testing.TB, kind, id string) Scope {
	t.Helper()
	scope, err := NewScope(kind, id)
	if err != nil {
		t.Fatalf("NewScope() error = %v", err)
	}
	return scope
}

func mustRule(
	t testing.TB,
	id, action, risk, kind, marker string,
	approvers []string,
	timeout time.Duration,
	onTimeout string,
) Rule {
	t.Helper()
	condition, err := NewCondition(action, risk)
	if err != nil {
		t.Fatalf("NewCondition() error = %v", err)
	}
	effect, err := NewEffect(kind, marker, approvers, timeout, onTimeout)
	if err != nil {
		t.Fatalf("NewEffect() error = %v", err)
	}
	rule, err := NewRule(id, condition, effect)
	if err != nil {
		t.Fatalf("NewRule() error = %v", err)
	}
	return rule
}

func mustRuleSet(
	t testing.TB,
	scope Scope,
	version int,
	rules []Rule,
) RuleSet {
	t.Helper()
	ruleSet, err := NewRuleSet(scope, version, rules)
	if err != nil {
		t.Fatalf("NewRuleSet() error = %v", err)
	}
	return ruleSet
}

func mustActionContext(
	t testing.TB,
	workItemID, runID, risk string,
) ActionContext {
	t.Helper()
	context, err := NewActionContext(ActionContextInput{
		ProjectID:       "project-1",
		TeamInstanceID:  "team-1",
		WorkPackageID:   "package-1",
		WorkItemID:      workItemID,
		RunID:           runID,
		AgentInstanceID: "agent-1",
		LogicalNodeID:   "node-1",
		AttemptNumber:   1,
		Action:          "start_run",
		Risk:            risk,
		ClaimID:         "",
		ClaimGeneration: 0,
		ContractDigest:  rulesTestContract,
	})
	if err != nil {
		t.Fatalf("NewActionContext() error = %v", err)
	}
	return context
}

func mustRuleSetActivationRequest(
	t testing.TB,
	ruleSet RuleSet,
) RuleSetActivationRequest {
	t.Helper()
	request, err := NewRuleSetActivationRequest(
		ruleSet,
		[]byte("opaque-local-authorization"),
	)
	if err != nil {
		t.Fatalf("NewRuleSetActivationRequest() error = %v", err)
	}
	return request
}

func mustApprovalDecisionRequest(
	t testing.TB,
	request ApprovalRequestRecord,
	decision string,
) ApprovalDecisionRequest {
	t.Helper()
	command, err := NewApprovalDecisionRequest(
		request.ID(),
		request.Digest(),
		decision,
		[]byte("opaque-local-authorization"),
	)
	if err != nil {
		t.Fatalf("NewApprovalDecisionRequest() error = %v", err)
	}
	return command
}

func openRulesStore(t testing.TB) *journal.Store {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s/rules.db?%s",
		t.TempDir(),
		values.Encode(),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return journal.NewStore(db)
}

func mustRulesAuthority(
	t testing.TB,
	store *journal.Store,
	authorizer CustomerAuthorizer,
	clock *rulesTestClock,
) *Authority {
	t.Helper()
	authority, err := NewAuthority(store, authorizer, clock.Now)
	if err != nil {
		t.Fatalf("NewAuthority() error = %v", err)
	}
	return authority
}

func seedRulesWorkItem(
	t testing.TB,
	store *journal.Store,
	clock *rulesTestClock,
	workItemID, runID string,
) *work.Authority {
	t.Helper()
	authority, err := work.NewAuthority(
		store,
		clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x31}, 1024)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.InitializeRunIdentityIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := authority.CreateAndAssign(
		context.Background(),
		work.WorkItemAssignmentInput{
			WorkItemID:      workItemID,
			Title:           "Approval gated work",
			RunID:           runID,
			AgentInstanceID: "agent-1",
			CorrelationID:   rulesTestCorrelation,
		},
	); err != nil {
		t.Fatal(err)
	}
	return authority
}

func seedRulesRuntime(t testing.TB, store *journal.Store, runtimeID string) {
	t.Helper()
	event := journal.Event{
		ID:             "evt-runtime-" + runtimeID,
		StreamID:       "runtime_instance:" + runtimeID,
		Seq:            1,
		IdempotencyKey: "evt-runtime-" + runtimeID,
		Type:           "RuntimeInstanceDiscovered",
		SchemaVersion:  1,
		EmittedAt:      rulesTestNow.Add(-time.Minute),
		CorrelationID:  rulesTestCorrelation,
		PayloadJSON: []byte(fmt.Sprintf(
			`{"discovery_digest":"%s","source_probe_id":"probe-1","instance":{"id":"%s","device_id":"device-1","adapter_type":"pi","display_name":"Pi","executable_version":"1.0.0","status":"online","observed_capabilities":["execution"],"capacity":1},"model_ids":[]}`,
			rulesTestContract,
			runtimeID,
		)),
	}
	if _, err := store.Append(context.Background(), event); err != nil {
		t.Fatalf("seed Runtime error = %v", err)
	}
}

func rulesTestAuthorizationDigest(kind, commandDigest string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + commandDigest))
	return fmt.Sprintf("%x", sum[:])
}

func rulesTestPresentationDigest(presentation []byte) string {
	sum := sha256.Sum256(presentation)
	return fmt.Sprintf("%x", sum[:])
}

func countRulesEventTypes(events []journal.Event, types ...string) int {
	allowed := make(map[string]struct{}, len(types))
	for _, eventType := range types {
		allowed[eventType] = struct{}{}
	}
	var count int
	for _, event := range events {
		if _, ok := allowed[event.Type]; ok {
			count++
		}
	}
	return count
}

func TestA41PermissionApprovalRequestedPendingWithRulesFacts(t *testing.T) {
	store := openRulesStore(t)
	authorizer := &rulesTestAuthorizer{
		actor: "approver:permission-owner",
		now:   time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC),
	}
	clock := &rulesTestClock{now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	record, err := authority.RequestPermissionApproval(
		context.Background(),
		PermissionApprovalInput{
			JobID: "job-perm-a4", CallDigest: rulesTestContract,
			Tool: "Bash", Command: "curl https://example.com",
			RequestedAt: clock.now, CorrelationID: rulesTestCorrelation,
		},
	)
	if err != nil {
		t.Fatalf("RequestPermissionApproval() error = %v", err)
	}
	if record.Status() != "pending" || record.WorkItemID() != "job-perm-a4" {
		t.Fatalf("record = %+v", record)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"WorkItemCreated", "WorkItemAssigned",
		"ApprovalRequested", "WorkItemApprovalPaused",
	} {
		if countRulesEventTypes(events, want) != 1 {
			t.Fatalf("event %s count != 1: %+v", want, events)
		}
	}
	for _, event := range events {
		if event.Type == "PermissionApproval" ||
			strings.Contains(event.Type, "PermissionApproval") {
			t.Fatalf("permission layer must reuse rules approval events, got %s", event.Type)
		}
	}
}

func TestA42PermissionApprovalIdempotent(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	authorizer := &rulesTestAuthorizer{actor: "approver:permission-owner", now: clock.now}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	input := PermissionApprovalInput{
		JobID: "job-perm-a4", CallDigest: rulesTestContract,
		Tool: "Bash", Command: "curl https://example.com",
		RequestedAt: clock.now, CorrelationID: rulesTestCorrelation,
	}
	first, err := authority.RequestPermissionApproval(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	before := len(mustRulesReadAll(t, store))
	second, err := authority.RequestPermissionApproval(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID() != first.ID() {
		t.Fatalf("idempotent approval id mismatch: %s vs %s", second.ID(), first.ID())
	}
	if after := len(mustRulesReadAll(t, store)); after != before {
		t.Fatalf("idempotent request duplicated facts: %d -> %d", before, after)
	}
}

func TestA43PermissionApprovalResolveApproved(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	authorizer := &rulesTestAuthorizer{actor: "approver:permission-owner", now: clock.now}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	record, err := authority.RequestPermissionApproval(context.Background(), PermissionApprovalInput{
		JobID: "job-perm-a4", CallDigest: rulesTestContract,
		Tool: "Bash", Command: "curl https://example.com",
		RequestedAt: clock.now, CorrelationID: rulesTestCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	decided, err := authority.DecidePermissionApproval(
		context.Background(), record.ID(), record.Digest(), "approved",
		"user-1", rulesTestCorrelation,
	)
	if err != nil {
		t.Fatalf("DecidePermissionApproval() error = %v", err)
	}
	if decided.Status() != "approved" ||
		decided.DecisionActorRef() != "approver:permission-owner" {
		t.Fatalf("decided = %+v", decided)
	}
	events, _ := store.ReadAll(context.Background())
	if countRulesEventTypes(events, "ApprovalDecided") != 1 ||
		countRulesEventTypes(events, "WorkItemApprovalResolved") != 1 {
		t.Fatalf("resolution facts missing: %+v", events)
	}
}

func TestA44PermissionApprovalResolveRejected(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	authorizer := &rulesTestAuthorizer{actor: "approver:permission-owner", now: clock.now}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	record, err := authority.RequestPermissionApproval(context.Background(), PermissionApprovalInput{
		JobID: "job-perm-a4", CallDigest: rulesTestContract,
		Tool: "Bash", Command: "curl https://example.com",
		RequestedAt: clock.now, CorrelationID: rulesTestCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	decided, err := authority.DecidePermissionApproval(
		context.Background(), record.ID(), record.Digest(), "rejected",
		"user-1", rulesTestCorrelation,
	)
	if err != nil {
		t.Fatal(err)
	}
	if decided.Status() != "rejected" {
		t.Fatalf("status = %s, want rejected", decided.Status())
	}
}

func TestA45PermissionApprovalRejectsEmptyActorAndDoubleResolve(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	authorizer := &rulesTestAuthorizer{actor: "approver:permission-owner", now: clock.now}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	record, err := authority.RequestPermissionApproval(context.Background(), PermissionApprovalInput{
		JobID: "job-perm-a4", CallDigest: rulesTestContract,
		Tool: "Bash", Command: "curl https://example.com",
		RequestedAt: clock.now, CorrelationID: rulesTestCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.DecidePermissionApproval(
		context.Background(), record.ID(), record.Digest(), "approved", "", rulesTestCorrelation,
	); err == nil {
		t.Fatal("empty resolvedBy must error")
	}
	if _, err := authority.DecidePermissionApproval(
		context.Background(), record.ID(), record.Digest(), "approved",
		"user-1", rulesTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.DecidePermissionApproval(
		context.Background(), record.ID(), record.Digest(), "rejected",
		"user-1", rulesTestCorrelation,
	); err == nil {
		t.Fatal("double resolve must error")
	}
}

func TestA46PermissionApprovalSurvivesRestart(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	authorizer := &rulesTestAuthorizer{actor: "approver:permission-owner", now: clock.now}
	first := mustRulesAuthority(t, store, authorizer, clock)
	input := PermissionApprovalInput{
		JobID: "job-perm-a4", CallDigest: rulesTestContract,
		Tool: "Bash", Command: "curl https://example.com",
		RequestedAt: clock.now, CorrelationID: rulesTestCorrelation,
	}
	record, err := first.RequestPermissionApproval(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	restarted := mustRulesAuthority(t, store, authorizer, clock)
	again, err := restarted.RequestPermissionApproval(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID() != record.ID() || again.Status() != "pending" {
		t.Fatalf("restart lost approval: %s %s", again.ID(), again.Status())
	}
	decided, err := restarted.DecidePermissionApproval(
		context.Background(), again.ID(), again.Digest(), "approved",
		"user-1", rulesTestCorrelation,
	)
	if err != nil || decided.Status() != "approved" {
		t.Fatalf("restart resolve error=%v record=%+v", err, decided)
	}
}

func TestA49DecidePermissionApprovalRejectsForeignApproval(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	authorizer := &rulesTestAuthorizer{actor: "approver:permission-owner", now: clock.now}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	seedRulesWorkItem(t, store, clock, "work-foreign", "run-foreign")
	foreignRuleSet := mustRuleSet(
		t,
		mustRulesScope(t, "work_item", "work-foreign"),
		1,
		[]Rule{mustRule(
			t,
			"approve-start",
			"start_run",
			"high",
			"require_approval",
			"",
			[]string{"local-owner"},
			time.Minute,
			"reject",
		)},
	)
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		mustRuleSetActivationRequest(t, foreignRuleSet),
		rulesTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	foreignAction := mustActionContext(t, "work-foreign", "run-foreign", "high")
	foreignDecision, err := Evaluate([]RuleSet{foreignRuleSet}, foreignAction)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := authority.RequestApproval(
		context.Background(),
		ApprovalRequestInput{
			Context:            foreignAction,
			ContinuationDigest: rulesTestContinuation,
			Decision:           foreignDecision,
			RequestedAt:        clock.now,
			CorrelationID:      rulesTestCorrelation,
		},
	)
	if err != nil {
		t.Fatalf("foreign RequestApproval() error = %v", err)
	}
	for _, decision := range []string{"approved", "rejected"} {
		if _, err := authority.DecidePermissionApproval(
			context.Background(),
			foreign.ID(), foreign.Digest(), decision,
			"user-1", rulesTestCorrelation,
		); err == nil {
			t.Fatalf("DecidePermissionApproval(%s) must reject a foreign approval", decision)
		}
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if countRulesEventTypes(events, "ApprovalDecided", "WorkItemApprovalResolved") != 0 {
		t.Fatal("foreign approval must not be resolved through the permission channel")
	}
}

func TestA411SecondAskSameJobDifferentCorrelationReusesActivation(t *testing.T) {
	store := openRulesStore(t)
	clock := &rulesTestClock{now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	authorizer := &rulesTestAuthorizer{actor: "approver:permission-owner", now: clock.now}
	authority := mustRulesAuthority(t, store, authorizer, clock)
	ctx := context.Background()
	first, err := authority.RequestPermissionApproval(ctx, PermissionApprovalInput{
		JobID: "job-a4-11", CallDigest: rulesTestContract,
		Tool: "Bash", Command: "curl https://one.example.com",
		RequestedAt: clock.now, CorrelationID: rulesTestCorrelation,
	})
	if err != nil {
		t.Fatalf("first ask error = %v", err)
	}
	if _, err := authority.DecidePermissionApproval(
		ctx, first.ID(), first.Digest(), "approved", "user-1", rulesTestCorrelation,
	); err != nil {
		t.Fatalf("resolve first error = %v", err)
	}
	second, err := authority.RequestPermissionApproval(ctx, PermissionApprovalInput{
		JobID: "job-a4-11", CallDigest: rulesTestContinuation,
		Tool: "Bash", Command: "curl https://two.example.com",
		RequestedAt: clock.now, CorrelationID: rulesOtherCorrelation,
	})
	if err != nil {
		t.Fatalf("second ask (different call, different correlation) error = %v", err)
	}
	if second.ID() == first.ID() || second.Status() != "pending" {
		t.Fatalf("second ask must create a new pending approval, got %s %s",
			second.ID(), second.Status())
	}
	if got := countRulesEventTypes(mustRulesReadAll(t, store), "RuleSetActivated"); got != 1 {
		t.Fatalf("permission ruleset must be activated exactly once, got %d", got)
	}
}

func mustRulesReadAll(t testing.TB, store *journal.Store) []journal.Event {
	t.Helper()
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return events
}
