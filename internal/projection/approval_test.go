package projection

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	ruleauthority "loom-pi-rebuild/internal/rules"

	_ "modernc.org/sqlite"
)

const (
	approvalProjectionCorrelation = "11111111-1111-4111-8111-111111111111"
	approvalProjectionDigest      = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func TestApprovalProjectionPublishesRuleRequestAndWorkStateAtomically(t *testing.T) {
	events := approvalProjectionEvents(t, false)
	approvalID, _ := approvalProjectionIdentity(t)
	projection := newForTestSource(eventSliceSource{events: events})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	snapshot := projection.Snapshot()
	domainRuleSet, _, _ := approvalProjectionDomain(t)
	ruleSet, ok := snapshot.RuleSets["rule-set/work_item/work-approval"]
	if !ok || ruleSet.ScopeKind != "work_item" ||
		ruleSet.ScopeID != "work-approval" ||
		ruleSet.Revision != 1 ||
		ruleSet.Digest != domainRuleSet.Digest() ||
		len(ruleSet.Rules) != 1 ||
		len(ruleSet.Rules[0].ApproverRefs) != 1 {
		t.Fatalf("projected RuleSet = %#v, %v", ruleSet, ok)
	}
	request, ok := snapshot.ApprovalRequests[approvalID]
	if !ok || request.Status != "pending" ||
		request.WorkItemID != "work-approval" ||
		request.RunID != "run-approval" ||
		request.ContinuationDigest != approvalProjectionDigest ||
		len(request.ApproverRefs) != 1 {
		t.Fatalf("projected ApprovalRequest = %#v, %v", request, ok)
	}
	if got := snapshot.WorkItems["work-approval"].Status; got != "waiting_approval" {
		t.Fatalf("WorkItem status = %q, want waiting_approval", got)
	}

	view := projection.GlobalReadView()
	viewRuleSet, ok := view.RuleSet("rule-set/work_item/work-approval")
	if !ok {
		t.Fatal("GlobalReadView RuleSet missing")
	}
	viewRequest, ok := view.ApprovalRequest(approvalID)
	if !ok {
		t.Fatal("GlobalReadView ApprovalRequest missing")
	}
	viewRuleSet.Rules[0].ApproverRefs[0] = "mutated"
	viewRequest.ApproverRefs[0] = "mutated"
	againRuleSet, _ := view.RuleSet("rule-set/work_item/work-approval")
	againRequest, _ := view.ApprovalRequest(approvalID)
	if againRuleSet.Rules[0].ApproverRefs[0] != "local-owner" ||
		againRequest.ApproverRefs[0] != "local-owner" {
		t.Fatal("typed accessors alias mutable projection storage")
	}
}

func TestApprovalProjectionResolvesAndFailedRebuildPreservesPublishedState(t *testing.T) {
	valid := approvalProjectionEvents(t, true)
	approvalID, approvalDigest := approvalProjectionIdentity(t)
	projection := newForTestSource(eventSliceSource{events: valid})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("valid Rebuild() error = %v", err)
	}
	beforeSnapshot := projection.Snapshot()
	beforeView := projection.GlobalReadView()
	request, ok := beforeView.ApprovalRequest(approvalID)
	if !ok || request.Status != "approved" ||
		request.DecisionActorRef != "local-owner" ||
		beforeSnapshot.WorkItems["work-approval"].Status != "assigned" {
		t.Fatalf(
			"resolved state = request %#v work %#v",
			request,
			beforeSnapshot.WorkItems["work-approval"],
		)
	}

	corrupt := append([]journal.Event(nil), valid...)
	for index := range corrupt {
		if corrupt[index].Type == "WorkItemApprovalResolved" {
			corrupt[index].PayloadJSON = projectionPayload(t, map[string]any{
				"work_item_id":            "work-approval",
				"approval_request_id":     "different-approval",
				"approval_request_digest": approvalDigest,
				"previous_status":         "waiting_approval",
				"status":                  "assigned",
			})
		}
	}
	projection.source = eventSliceSource{events: corrupt}
	if err := projection.Rebuild(context.Background()); !errors.Is(
		err,
		ErrInvalidProjectionEvent,
	) {
		t.Fatalf("corrupt Rebuild() error = %v", err)
	}
	afterSnapshot := projection.Snapshot()
	afterView := projection.GlobalReadView()
	if afterSnapshot.WorkItems["work-approval"].Status != "assigned" ||
		afterView.Version() != beforeView.Version() {
		t.Fatalf(
			"failed rebuild replaced published state: snapshot=%#v version=%q want=%q",
			afterSnapshot.WorkItems["work-approval"],
			afterView.Version(),
			beforeView.Version(),
		)
	}
}

func TestApprovalProjectionCompatibleWithPermissionApprovalFacts(t *testing.T) {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	database, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s/perm-approval.db?%s",
		t.TempDir(),
		values.Encode(),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	clock := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	authority, err := ruleauthority.NewAuthority(store, &permissionProjectionAuthorizer{now: clock}, func() time.Time {
		return clock
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err := authority.RequestPermissionApproval(context.Background(),
		ruleauthority.PermissionApprovalInput{
			JobID: "job-perm-proj", CallDigest: approvalProjectionDigest,
			Tool: "Bash", Command: "curl https://example.com",
			RequestedAt: clock, CorrelationID: approvalProjectionCorrelation,
		})
	if err != nil {
		t.Fatal(err)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	projection := newForTestSource(eventSliceSource{events: events})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("projection must accept permission approval facts: %v", err)
	}
	snapshot := projection.Snapshot()
	request, ok := snapshot.ApprovalRequests[record.ID()]
	if !ok || request.Status != "pending" ||
		request.WorkItemID != "job-perm-proj" {
		t.Fatalf("projected permission approval = %#v %v", request, ok)
	}
	if work, exists := snapshot.WorkItems["job-perm-proj"]; !exists || work.Status != "waiting_approval" {
		t.Fatalf("projected work item = %#v %v", work, exists)
	}
}

type permissionProjectionAuthorizer struct {
	now time.Time
}

func (authorizer *permissionProjectionAuthorizer) AuthorizeRuleSet(
	ctx context.Context,
	request ruleauthority.RuleSetActivationRequest,
) (ruleauthority.AuthorizedRuleSetActivation, error) {
	if err := ctx.Err(); err != nil {
		return ruleauthority.AuthorizedRuleSetActivation{}, err
	}
	digest := sha256.Sum256([]byte("permission-rule-auth"))
	return ruleauthority.NewAuthorizedRuleSetActivation(
		request,
		"approver:permission-owner",
		fmt.Sprintf("%x", digest[:]),
		authorizer.now,
		authorizer.now.Add(time.Hour),
	)
}

func (authorizer *permissionProjectionAuthorizer) AuthorizeApprovalDecision(
	ctx context.Context,
	request ruleauthority.ApprovalDecisionRequest,
) (ruleauthority.AuthorizedApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return ruleauthority.AuthorizedApprovalDecision{}, err
	}
	digest := sha256.Sum256([]byte("permission-decision-auth"))
	return ruleauthority.NewAuthorizedApprovalDecision(
		request,
		"approver:permission-owner",
		fmt.Sprintf("%x", digest[:]),
		authorizer.now,
		authorizer.now.Add(time.Hour),
	)
}

func TestApprovalProjectionSupportsExpiryAndRenumbersRunAuthorityView(t *testing.T) {
	expired := approvalProjectionEvents(t, true)
	approvalID, approvalDigest := approvalProjectionIdentity(t)
	for index := range expired {
		switch expired[index].Type {
		case "ApprovalDecided":
			expired[index].Type = "ApprovalExpired"
			expired[index].PayloadJSON = projectionPayload(t, map[string]any{
				"approval_request_id":     approvalID,
				"approval_request_digest": approvalDigest,
				"status":                  "expired",
				"actor_ref":               "",
				"authorization_digest":    "",
				"decided_at": time.Date(
					2026,
					7,
					26,
					12,
					1,
					0,
					0,
					time.UTC,
				).Format(time.RFC3339Nano),
			})
		case "WorkItemApprovalResolved":
			expired[index].PayloadJSON = projectionPayload(t, map[string]any{
				"work_item_id":            "work-approval",
				"approval_request_id":     approvalID,
				"approval_request_digest": approvalDigest,
				"previous_status":         "waiting_approval",
				"status":                  "blocked",
			})
		}
	}
	snapshot, err := replay(context.Background(), expired)
	if err != nil {
		t.Fatalf("expired replay() error = %v", err)
	}
	if snapshot.ApprovalRequests[approvalID].Status != "expired" ||
		snapshot.WorkItems["work-approval"].Status != "blocked" {
		t.Fatalf(
			"expired projection = approval %#v work %#v",
			snapshot.ApprovalRequests[approvalID],
			snapshot.WorkItems["work-approval"],
		)
	}

	renumbered := runAuthorityEventsWithoutApprovalFacts([]journal.Event{
		{StreamID: "work-item/work-1", Seq: 1, Type: "WorkItemCreated"},
		{StreamID: "work-item/work-1", Seq: 2, Type: "WorkItemAssigned"},
		{StreamID: "work-item/work-1", Seq: 3, Type: "WorkItemApprovalPaused"},
		{StreamID: "work-item/work-1", Seq: 4, Type: "WorkItemApprovalResolved"},
		{StreamID: "work-item/work-1", Seq: 5, Type: "WorkItemReadyForReview"},
	})
	if len(renumbered) != 3 ||
		renumbered[2].Type != "WorkItemReadyForReview" ||
		renumbered[2].Seq != 3 {
		t.Fatalf("renumbered Run authority Events = %#v", renumbered)
	}
}

func approvalProjectionEvents(t testing.TB, resolved bool) []journal.Event {
	t.Helper()
	requestedAt := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	ruleSet, action, decision := approvalProjectionDomain(t)
	ruleSetReference := decision.RuleSetReferences()[0]
	approvalID := canonicalApprovalProjectionID(
		action.Digest(),
		approvalProjectionDigest,
		decision.Digest(),
	)
	approvalDigest := canonicalApprovalProjectionDigest(
		approvalID,
		action.Digest(),
		approvalProjectionDigest,
		decision.Digest(),
		requestedAt,
		requestedAt.Add(time.Minute),
	)
	events := []journal.Event{
		runAuthorityEvent(
			"work-created",
			"work-item/work-approval",
			1,
			"WorkItemCreated",
			approvalProjectionCorrelation,
			"",
			map[string]any{
				"work_item_id": "work-approval",
				"title":        "Approval work",
				"status":       "ready",
			},
		),
		runAuthorityEvent(
			"work-assigned",
			"work-item/work-approval",
			2,
			"WorkItemAssigned",
			approvalProjectionCorrelation,
			"work-created",
			map[string]any{
				"work_item_id":      "work-approval",
				"run_id":            "run-approval",
				"agent_instance_id": "agent-approval",
				"status":            "assigned",
			},
		),
		runAuthorityEvent(
			"rule-activated",
			"rule-set/work_item/work-approval",
			1,
			"RuleSetActivated",
			approvalProjectionCorrelation,
			"",
			map[string]any{
				"scope": map[string]any{
					"kind": "work_item",
					"id":   "work-approval",
				},
				"revision": 1,
				"digest":   ruleSet.Digest(),
				"rules": []map[string]any{{
					"id": "approve-start",
					"condition": map[string]any{
						"action": "start_run",
						"risk":   "high",
					},
					"effect": map[string]any{
						"kind":          "require_approval",
						"marker":        "",
						"approver_refs": []string{"local-owner"},
						"timeout_nanos": int64(time.Minute),
						"on_timeout":    "reject",
					},
				}},
				"actor_ref":            "local-owner",
				"authorization_digest": approvalProjectionDigest,
				"command_digest":       strings.Repeat("a", 64),
			},
		),
		runAuthorityEvent(
			"approval-requested",
			"approval/"+approvalID,
			1,
			"ApprovalRequested",
			approvalProjectionCorrelation,
			"work-assigned",
			map[string]any{
				"approval_request_id":     approvalID,
				"approval_request_digest": approvalDigest,
				"context": map[string]any{
					"project_id":        "project-1",
					"team_instance_id":  "team-1",
					"work_package_id":   "package-1",
					"work_item_id":      "work-approval",
					"run_id":            "run-approval",
					"agent_instance_id": "agent-approval",
					"logical_node_id":   "node-1",
					"attempt_number":    1,
					"action":            "start_run",
					"risk":              "high",
					"claim_id":          "",
					"claim_generation":  0,
					"contract_digest":   approvalProjectionDigest,
					"digest":            action.Digest(),
				},
				"continuation_digest": approvalProjectionDigest,
				"decision": map[string]any{
					"kind":          "require_approval",
					"digest":        decision.Digest(),
					"action_digest": action.Digest(),
					"rule_set_refs": []map[string]any{{
						"stream_id":  ruleSetReference.StreamID,
						"scope_kind": ruleSetReference.Kind,
						"scope_id":   ruleSetReference.ScopeID,
						"version":    ruleSetReference.Version,
						"digest":     ruleSetReference.Digest,
					}},
					"approver_refs":   decision.ApproverRefs(),
					"timeout_nanos":   int64(decision.Timeout()),
					"on_timeout":      decision.OnTimeout(),
					"warning_markers": decision.WarningMarkers(),
					"record_markers":  decision.RecordMarkers(),
				},
				"previous_status": "assigned",
				"requested_at":    requestedAt.Format(time.RFC3339Nano),
				"expires_at":      requestedAt.Add(time.Minute).Format(time.RFC3339Nano),
			},
		),
		runAuthorityEvent(
			"approval-paused",
			"work-item/work-approval",
			3,
			"WorkItemApprovalPaused",
			approvalProjectionCorrelation,
			"approval-requested",
			map[string]any{
				"work_item_id":            "work-approval",
				"approval_request_id":     approvalID,
				"approval_request_digest": approvalDigest,
				"previous_status":         "assigned",
				"status":                  "waiting_approval",
			},
		),
	}
	if !resolved {
		return events
	}
	return append(
		events,
		runAuthorityEvent(
			"approval-decided",
			"approval/"+approvalID,
			2,
			"ApprovalDecided",
			approvalProjectionCorrelation,
			"approval-requested",
			map[string]any{
				"approval_request_id":     approvalID,
				"approval_request_digest": approvalDigest,
				"status":                  "approved",
				"actor_ref":               "local-owner",
				"authorization_digest":    strings.Repeat("d", 64),
				"decided_at":              requestedAt.Add(10 * time.Second).Format(time.RFC3339Nano),
			},
		),
		runAuthorityEvent(
			"approval-resolved",
			"work-item/work-approval",
			4,
			"WorkItemApprovalResolved",
			approvalProjectionCorrelation,
			"approval-decided",
			map[string]any{
				"work_item_id":            "work-approval",
				"approval_request_id":     approvalID,
				"approval_request_digest": approvalDigest,
				"previous_status":         "waiting_approval",
				"status":                  "assigned",
			},
		),
	)
}

func approvalProjectionDomain(
	t testing.TB,
) (ruleauthority.RuleSet, ruleauthority.ActionContext, ruleauthority.Decision) {
	t.Helper()
	scope, err := ruleauthority.NewScope("work_item", "work-approval")
	if err != nil {
		t.Fatal(err)
	}
	condition, err := ruleauthority.NewCondition("start_run", "high")
	if err != nil {
		t.Fatal(err)
	}
	effect, err := ruleauthority.NewEffect(
		"require_approval",
		"",
		[]string{"local-owner"},
		time.Minute,
		"reject",
	)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := ruleauthority.NewRule("approve-start", condition, effect)
	if err != nil {
		t.Fatal(err)
	}
	ruleSet, err := ruleauthority.NewRuleSet(
		scope,
		1,
		[]ruleauthority.Rule{rule},
	)
	if err != nil {
		t.Fatal(err)
	}
	action, err := ruleauthority.NewActionContext(
		ruleauthority.ActionContextInput{
			ProjectID:       "project-1",
			TeamInstanceID:  "team-1",
			WorkPackageID:   "package-1",
			WorkItemID:      "work-approval",
			RunID:           "run-approval",
			AgentInstanceID: "agent-approval",
			LogicalNodeID:   "node-1",
			AttemptNumber:   1,
			Action:          "start_run",
			Risk:            "high",
			ContractDigest:  approvalProjectionDigest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := ruleauthority.Evaluate(
		[]ruleauthority.RuleSet{ruleSet},
		action,
	)
	if err != nil {
		t.Fatal(err)
	}
	return ruleSet, action, decision
}

func approvalProjectionIdentity(t testing.TB) (string, string) {
	t.Helper()
	_, action, decision := approvalProjectionDomain(t)
	requestedAt := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	id := canonicalApprovalProjectionID(
		action.Digest(),
		approvalProjectionDigest,
		decision.Digest(),
	)
	return id, canonicalApprovalProjectionDigest(
		id,
		action.Digest(),
		approvalProjectionDigest,
		decision.Digest(),
		requestedAt,
		requestedAt.Add(time.Minute),
	)
}
