package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/rules"
	"loom-pi-rebuild/internal/toolproposal"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

const (
	rulesTestContractHex     = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	rulesTestContinuationHex = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
)

func openPermAppStore(t testing.TB) *journal.Store {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s/perm-app.db?%s",
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

func mustPermService(t testing.TB, store *journal.Store) *LocalPermissionService {
	t.Helper()
	clock := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	service, err := NewLocalPermissionService(store, clock, func() string { return "view-v1" }, nil)
	if err != nil {
		t.Fatalf("NewLocalPermissionService() error = %v", err)
	}
	return service
}

func permissionCommand(
	t testing.TB,
	service *LocalPermissionService,
	action string,
	input any,
) (PermissionCommandResult, error) {
	t.Helper()
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return service.PermissionCommand(context.Background(), PermissionCommandRequest{
		JourneyID:   "11111111-1111-4111-8111-111111111111",
		OperationID: "op-" + action + "-" + fmt.Sprint(time.Now().UnixNano()),
		Action:      action, Input: body,
	})
}

func TestRed14_SnapshotAndAttentionAreReadOnly(t *testing.T) {
	store := openPermAppStore(t)
	service := mustPermService(t, store)
	input := permissions.ProfileInput{
		ProfileID: "profile-readonly", Mode: permissions.ModeDefault,
		OwnedPaths: []string{"src/**"},
	}
	if _, err := permissionCommand(t, service, "define_profile", input); err != nil {
		t.Fatalf("define_profile error = %v", err)
	}
	before := len(allAppEvents(t, store))

	snapshot, err := service.PermissionSnapshot(context.Background(), PermissionSnapshotRequest{})
	if err != nil {
		t.Fatalf("PermissionSnapshot() error = %v", err)
	}
	if len(snapshot.Profiles) != 1 || snapshot.Profiles[0].ProfileID != "profile-readonly" {
		t.Fatalf("snapshot profiles = %+v", snapshot.Profiles)
	}
	attention, err := service.PermissionAttention(context.Background(), PermissionAttentionRequest{})
	if err != nil {
		t.Fatalf("PermissionAttention() error = %v", err)
	}
	if len(attention.Decisions) != 0 {
		t.Fatalf("attention decisions = %+v, want none", attention.Decisions)
	}
	after := len(allAppEvents(t, store))
	if after != before {
		t.Fatalf("snapshot/attention wrote Journal: before=%d after=%d", before, after)
	}
}

func TestValidateCallAllowAndAskDecisionFact(t *testing.T) {
	store := openPermAppStore(t)
	service := mustPermService(t, store)
	profile := permissions.ProfileInput{
		ProfileID: "profile-vc", Mode: permissions.ModeDefault,
		OwnedPaths: []string{"src/**"},
	}
	if _, err := permissionCommand(t, service, "define_profile", profile); err != nil {
		t.Fatalf("define_profile error = %v", err)
	}
	if _, err := permissionCommand(t, service, "bind_job", struct {
		JobID     string `json:"job_id"`
		ProfileID string `json:"profile_id"`
	}{JobID: "job-vc", ProfileID: "profile-vc"}); err != nil {
		t.Fatalf("bind_job error = %v", err)
	}
	if _, err := permissionCommand(t, service, "add_rule", struct {
		Rule         permissions.Rule `json:"rule"`
		AuthorizedBy string           `json:"authorized_by"`
	}{Rule: permissions.Rule{
		RuleID: "r-vc-allow", Scope: permissions.ScopeJob, ScopeID: "job-vc",
		Action: permissions.ActionAllow, Tool: permissions.ToolBash, Pattern: "go test *",
	}, AuthorizedBy: "user-1"}); err != nil {
		t.Fatalf("add_rule error = %v", err)
	}

	allow, err := permissionCommand(t, service, "validate_call", struct {
		JobID string                   `json:"job_id"`
		Call  permissions.ProposedCall `json:"call"`
	}{JobID: "job-vc", Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "go test ./..."}})
	if err != nil {
		t.Fatalf("validate_call allow error = %v", err)
	}
	if allow.Verdict != permissions.VerdictAllow {
		t.Fatalf("verdict = %s, want allow", allow.Verdict)
	}
	if len(allAppEvents(t, store)) != 3 {
		t.Fatalf("allow path must not record a decision fact, events = %d", len(allAppEvents(t, store)))
	}

	ask, err := permissionCommand(t, service, "validate_call", struct {
		JobID string                   `json:"job_id"`
		Call  permissions.ProposedCall `json:"call"`
	}{JobID: "job-vc", Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "curl https://example.com"}})
	if err != nil {
		t.Fatalf("validate_call ask error = %v", err)
	}
	if ask.Verdict != permissions.VerdictAsk {
		t.Fatalf("verdict = %s, want ask", ask.Verdict)
	}
	if len(ask.EventIDs) != 1 {
		t.Fatalf("ask path must record one decision fact, got %d", len(ask.EventIDs))
	}
	attention, err := service.PermissionAttention(context.Background(), PermissionAttentionRequest{})
	if err != nil {
		t.Fatalf("PermissionAttention() error = %v", err)
	}
	if len(attention.Decisions) != 1 || attention.Decisions[0].JobID != "job-vc" {
		t.Fatalf("attention decisions = %+v", attention.Decisions)
	}
	if attention.Decisions[0].Command != "" ||
		attention.Decisions[0].Tool != permissions.ToolBash ||
		attention.Decisions[0].CallDigest != permissions.ProposedCallDigest(
			permissions.ProposedCall{Tool: permissions.ToolBash, Command: "curl https://example.com"},
		) ||
		attention.Decisions[0].Reason != "approval_required" ||
		attention.Decisions[0].AuthorizationPath != "review_permission_request" {
		t.Fatalf("decision fact contains unsafe or invalid details: %+v", attention.Decisions[0])
	}
}

func TestA48ApprovalLifecycleWiredThroughRulesPort(t *testing.T) {
	store := openPermAppStore(t)
	clock := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	port := mustAppApprovalPort(t, store, clock)
	service, err := NewLocalPermissionService(store, clock, func() string { return "view-v1" }, port)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := permissionCommand(t, service, "define_profile", permissions.ProfileInput{
		ProfileID: "profile-a48", Mode: permissions.ModeDefault,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := permissionCommand(t, service, "bind_job", struct {
		JobID     string `json:"job_id"`
		ProfileID string `json:"profile_id"`
	}{JobID: "job-a48", ProfileID: "profile-a48"}); err != nil {
		t.Fatal(err)
	}
	if _, err := permissionCommand(t, service, "add_rule", struct {
		Rule         permissions.Rule `json:"rule"`
		AuthorizedBy string           `json:"authorized_by"`
	}{Rule: permissions.Rule{
		RuleID: "r-a48-ask", Scope: permissions.ScopeJob, ScopeID: "job-a48",
		Action: permissions.ActionAsk, Tool: permissions.ToolBash, Pattern: "curl *",
	}, AuthorizedBy: "user-1"}); err != nil {
		t.Fatal(err)
	}
	ask, err := permissionCommand(t, service, "validate_call", struct {
		JobID string                   `json:"job_id"`
		Call  permissions.ProposedCall `json:"call"`
	}{JobID: "job-a48", Call: permissions.ProposedCall{
		Tool: permissions.ToolBash, Command: "curl https://example.com",
	}})
	if err != nil {
		t.Fatalf("validate_call error = %v", err)
	}
	if ask.Verdict != permissions.VerdictAsk || ask.ApprovalID == "" || ask.ApprovalDigest == "" {
		t.Fatalf("ask result missing approval: %+v", ask)
	}
	attention, err := service.PermissionAttention(context.Background(), PermissionAttentionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(attention.Approvals) != 1 || attention.Approvals[0].JobID != "job-a48" ||
		attention.Approvals[0].Status != "pending" {
		t.Fatalf("pending approval missing from attention: %+v", attention.Approvals)
	}
	approved, err := permissionCommand(t, service, "resolve_approval", struct {
		ApprovalID     string `json:"approval_id"`
		ApprovalDigest string `json:"approval_digest"`
		Resolution     string `json:"resolution"`
		ResolvedBy     string `json:"resolved_by"`
	}{ApprovalID: ask.ApprovalID, ApprovalDigest: ask.ApprovalDigest,
		Resolution: "allow", ResolvedBy: "user-1"})
	if err != nil {
		t.Fatalf("resolve_approval error = %v", err)
	}
	if approved.Note == "" || approved.ApprovalID == "" {
		t.Fatalf("resolve result = %+v", approved)
	}
	after, err := service.PermissionAttention(context.Background(), PermissionAttentionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Approvals) != 0 {
		t.Fatalf("resolved approval must leave attention: %+v", after.Approvals)
	}
}

type permissionProposalStoreFixture struct {
	lookup toolproposal.ApprovalLookup
	record toolproposal.Record
}

func (*permissionProposalStoreFixture) PutToolProposal(context.Context, toolproposal.Record) error {
	return fmt.Errorf("not implemented")
}

func (*permissionProposalStoreFixture) ReadToolProposal(
	context.Context,
	toolproposal.Binding,
) (toolproposal.Record, error) {
	return toolproposal.Record{}, fmt.Errorf("not implemented")
}

func (fixture *permissionProposalStoreFixture) LookupToolProposal(
	_ context.Context,
	lookup toolproposal.ApprovalLookup,
) (toolproposal.Record, error) {
	if lookup != fixture.lookup {
		return toolproposal.Record{}, fmt.Errorf("binding mismatch")
	}
	return toolproposal.Record{
		Binding: fixture.record.Binding,
		Content: append([]byte(nil), fixture.record.Content...),
	}, nil
}

func (*permissionProposalStoreFixture) DeleteToolProposal(
	context.Context,
	toolproposal.Binding,
) error {
	return fmt.Errorf("not implemented")
}

func TestP2DPermissionAttentionDecryptsExactProposalDetail(t *testing.T) {
	store := openPermAppStore(t)
	clock := func() time.Time { return time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC) }
	port := mustAppApprovalPort(t, store, clock)
	service, err := NewLocalPermissionService(store, clock, func() string { return "view-vault" }, port)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := permissionCommand(t, service, "define_profile", permissions.ProfileInput{
		ProfileID: "profile-vault-detail", Mode: permissions.ModeDefault,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := permissionCommand(t, service, "bind_job", struct {
		JobID     string `json:"job_id"`
		ProfileID string `json:"profile_id"`
	}{JobID: "job-vault-detail", ProfileID: "profile-vault-detail"}); err != nil {
		t.Fatal(err)
	}
	call := permissions.ProposedCall{
		Tool: permissions.ToolBash, Command: "printf inspect-before-approve",
	}
	ask, err := permissionCommand(t, service, "validate_call", struct {
		JobID string                   `json:"job_id"`
		Call  permissions.ProposedCall `json:"call"`
	}{JobID: "job-vault-detail", Call: call})
	if err != nil || ask.ApprovalID == "" {
		t.Fatalf("ask = %#v, %v", ask, err)
	}
	if err := service.SetToolProposalStore(&permissionProposalStoreFixture{}); err != nil {
		t.Fatal(err)
	}
	if _, err := permissionCommand(t, service, "resolve_approval", struct {
		ApprovalID     string `json:"approval_id"`
		ApprovalDigest string `json:"approval_digest"`
		Resolution     string `json:"resolution"`
		ResolvedBy     string `json:"resolved_by"`
	}{
		ApprovalID: ask.ApprovalID, ApprovalDigest: ask.ApprovalDigest,
		Resolution: "allow", ResolvedBy: "user-vault",
	}); !errors.Is(err, ErrInvalidPermissionRequest) {
		t.Fatalf("approval without authenticated detail = %v", err)
	}
	content, err := json.Marshal(call)
	if err != nil {
		t.Fatal(err)
	}
	digest := permissions.ProposedCallDigest(call)
	proposalStore := &permissionProposalStoreFixture{
		lookup: toolproposal.ApprovalLookup{
			ApprovalID: ask.ApprovalID, ApprovalDigest: ask.ApprovalDigest,
			WorkItemID: "job-vault-detail", CallDigest: digest,
		},
		record: toolproposal.Record{
			Binding: toolproposal.Binding{Tool: string(call.Tool), CallDigest: digest},
			Content: content,
		},
	}
	if err := service.SetToolProposalStore(proposalStore); err != nil {
		t.Fatal(err)
	}
	attention, err := service.PermissionAttention(context.Background(), PermissionAttentionRequest{})
	if err != nil || len(attention.Approvals) != 1 {
		t.Fatalf("attention = %#v, %v", attention, err)
	}
	approval := attention.Approvals[0]
	if !approval.DetailsAvailable || approval.DetailStatus != "encrypted_vault" ||
		approval.CallDigest != digest || approval.Command != call.Command || approval.Tool != call.Tool {
		t.Fatalf("approval detail = %#v", approval)
	}
	resolved, err := permissionCommand(t, service, "resolve_approval", struct {
		ApprovalID     string `json:"approval_id"`
		ApprovalDigest string `json:"approval_digest"`
		Resolution     string `json:"resolution"`
		ResolvedBy     string `json:"resolved_by"`
	}{
		ApprovalID: ask.ApprovalID, ApprovalDigest: ask.ApprovalDigest,
		Resolution: "allow", ResolvedBy: "user-vault",
	})
	if err != nil || resolved.Note != "approval approved" {
		t.Fatalf("resolved exact encrypted proposal = %#v, %v", resolved, err)
	}
}

func TestA410AttentionAndResolveArePermissionScoped(t *testing.T) {
	store := openPermAppStore(t)
	clock := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	authority, err := rules.NewAuthority(store, &appTestAuthorizer{clock: clock}, clock)
	if err != nil {
		t.Fatal(err)
	}
	port := &appTestApprovalPort{authority: authority, clock: clock}
	service, err := NewLocalPermissionService(store, clock, func() string { return "view-v1" }, port)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// Foreign (non-permission) pending approval in the same journal.
	workAuthority, err := work.NewAuthority(
		store,
		clock,
		bytes.NewReader(bytes.Repeat([]byte{0x31}, 1024)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := workAuthority.CreateAndAssign(ctx, work.WorkItemAssignmentInput{
		WorkItemID: "work-foreign", Title: "Approval gated work",
		RunID: "run-foreign", AgentInstanceID: "agent-1",
		CorrelationID: "11111111-1111-4111-8111-111111111111",
	}); err != nil {
		t.Fatal(err)
	}
	foreignScope, err := rules.NewScope("work_item", "work-foreign")
	if err != nil {
		t.Fatal(err)
	}
	foreignCondition, err := rules.NewCondition("start_run", "high")
	if err != nil {
		t.Fatal(err)
	}
	foreignEffect, err := rules.NewEffect(
		"require_approval", "", []string{"local-owner"}, 2*time.Hour, "reject",
	)
	if err != nil {
		t.Fatal(err)
	}
	foreignRule, err := rules.NewRule("approve-start", foreignCondition, foreignEffect)
	if err != nil {
		t.Fatal(err)
	}
	foreignRuleSet, err := rules.NewRuleSet(foreignScope, 1, []rules.Rule{foreignRule})
	if err != nil {
		t.Fatal(err)
	}
	foreignActivation, err := rules.NewRuleSetActivationRequest(
		foreignRuleSet, []byte("foreign-approval"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ActivateRuleSet(ctx, foreignActivation, "11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatal(err)
	}
	foreignAction, err := rules.NewActionContext(rules.ActionContextInput{
		ProjectID: "project-1", TeamInstanceID: "team-1",
		WorkPackageID: "package-1", WorkItemID: "work-foreign",
		RunID: "run-foreign", AgentInstanceID: "agent-1",
		LogicalNodeID: "node-1", AttemptNumber: 1,
		Action: "start_run", Risk: "high", ContractDigest: rulesTestContractHex,
	})
	if err != nil {
		t.Fatal(err)
	}
	foreignDecision, err := rules.Evaluate([]rules.RuleSet{foreignRuleSet}, foreignAction)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := authority.RequestApproval(ctx, rules.ApprovalRequestInput{
		Context: foreignAction, ContinuationDigest: rulesTestContinuationHex,
		Decision: foreignDecision, RequestedAt: clock(), CorrelationID: "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatalf("foreign RequestApproval() error = %v", err)
	}
	// Permission ask for a separate job.
	if _, err := permissionCommand(t, service, "define_profile", permissions.ProfileInput{
		ProfileID: "profile-a410", Mode: permissions.ModeDefault,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := permissionCommand(t, service, "bind_job", struct {
		JobID     string `json:"job_id"`
		ProfileID string `json:"profile_id"`
	}{JobID: "job-a410", ProfileID: "profile-a410"}); err != nil {
		t.Fatal(err)
	}
	if _, err := permissionCommand(t, service, "add_rule", struct {
		Rule         permissions.Rule `json:"rule"`
		AuthorizedBy string           `json:"authorized_by"`
	}{Rule: permissions.Rule{
		RuleID: "r-a410-ask", Scope: permissions.ScopeJob, ScopeID: "job-a410",
		Action: permissions.ActionAsk, Tool: permissions.ToolBash, Pattern: "curl *",
	}, AuthorizedBy: "user-1"}); err != nil {
		t.Fatal(err)
	}
	ask, err := permissionCommand(t, service, "validate_call", struct {
		JobID string                   `json:"job_id"`
		Call  permissions.ProposedCall `json:"call"`
	}{JobID: "job-a410", Call: permissions.ProposedCall{
		Tool: permissions.ToolBash, Command: "curl https://example.com",
	}})
	if err != nil {
		t.Fatalf("validate_call error = %v", err)
	}
	attention, err := service.PermissionAttention(ctx, PermissionAttentionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(attention.Approvals) != 1 || attention.Approvals[0].JobID != "job-a410" {
		t.Fatalf("attention must show only the permission approval, got %+v", attention.Approvals)
	}
	if _, err := permissionCommand(t, service, "resolve_approval", struct {
		ApprovalID     string `json:"approval_id"`
		ApprovalDigest string `json:"approval_digest"`
		Resolution     string `json:"resolution"`
		ResolvedBy     string `json:"resolved_by"`
	}{ApprovalID: foreign.ID(), ApprovalDigest: foreign.Digest(),
		Resolution: "deny", ResolvedBy: "user-1"}); err == nil {
		t.Fatal("resolve_approval must reject a foreign approval")
	}
	if ask.ApprovalID == "" {
		t.Fatal("permission ask must carry an approval id")
	}
}

type appTestApprovalPort struct {
	authority *rules.Authority
	clock     func() time.Time
}

func mustAppApprovalPort(t testing.TB, store *journal.Store, clock func() time.Time) ApprovalPort {
	t.Helper()
	authority, err := rules.NewAuthority(store, &appTestAuthorizer{clock: clock}, clock)
	if err != nil {
		t.Fatal(err)
	}
	return &appTestApprovalPort{authority: authority, clock: clock}
}

func (port *appTestApprovalPort) RequestPermissionApproval(
	ctx context.Context,
	input rules.PermissionApprovalInput,
) (rules.ApprovalRequestRecord, error) {
	return port.authority.RequestPermissionApproval(ctx, input)
}

func (port *appTestApprovalPort) DecidePermissionApproval(
	ctx context.Context,
	approvalID, approvalDigest, decision, resolvedBy, correlationID string,
) (rules.ApprovalRequestRecord, error) {
	return port.authority.DecidePermissionApproval(
		ctx, approvalID, approvalDigest, decision, resolvedBy, correlationID,
	)
}

type appTestAuthorizer struct {
	clock func() time.Time
}

func (authorizer *appTestAuthorizer) AuthorizeRuleSet(
	ctx context.Context,
	request rules.RuleSetActivationRequest,
) (rules.AuthorizedRuleSetActivation, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedRuleSetActivation{}, err
	}
	digest := sha256.Sum256([]byte("app-rule-auth"))
	now := authorizer.clock()
	return rules.NewAuthorizedRuleSetActivation(
		request, "approver:permission-owner", fmt.Sprintf("%x", digest[:]),
		now, now.Add(time.Hour),
	)
}

func (authorizer *appTestAuthorizer) AuthorizeApprovalDecision(
	ctx context.Context,
	request rules.ApprovalDecisionRequest,
) (rules.AuthorizedApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedApprovalDecision{}, err
	}
	digest := sha256.Sum256([]byte("app-decision-auth"))
	now := authorizer.clock()
	return rules.NewAuthorizedApprovalDecision(
		request, "approver:permission-owner", fmt.Sprintf("%x", digest[:]),
		now, now.Add(time.Hour),
	)
}

func TestPermissionCommandUnknownActionAndUnboundJob(t *testing.T) {
	store := openPermAppStore(t)
	service := mustPermService(t, store)
	if _, err := permissionCommand(t, service, "bogus_action", struct{}{}); err == nil {
		t.Fatal("unknown action must error")
	}
	if _, err := permissionCommand(t, service, "validate_call", struct {
		JobID string                   `json:"job_id"`
		Call  permissions.ProposedCall `json:"call"`
	}{JobID: "unbound", Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "cat x"}}); err != nil {
		t.Fatalf("unbound validate_call error = %v", err)
	}
}

func allAppEvents(t testing.TB, store *journal.Store) []journal.Event {
	t.Helper()
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return events
}
