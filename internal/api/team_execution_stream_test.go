package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/verification"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

type apiTestViewSource struct {
	view       projection.GlobalReadView
	rebuildErr error
	rebuilds   int
}

type apiTestAgentAttemptDiagnosticSource struct {
	queries   []AgentAttemptDiagnosticQuery
	summaries []AgentAttemptDiagnosticSummary
	err       error
}

type apiTestGovernedTestReportSource struct {
	queries []work.AttemptReportQuery
	reports []verification.GovernedTestReport
	err     error
}

func (source *apiTestGovernedTestReportSource) GovernedTestReportsForAttempt(
	_ context.Context,
	query work.AttemptReportQuery,
) ([]verification.GovernedTestReport, error) {
	source.queries = append(source.queries, query)
	return append([]verification.GovernedTestReport(nil), source.reports...), source.err
}

func (source *apiTestAgentAttemptDiagnosticSource) AgentAttemptDiagnostics(
	_ context.Context,
	queries []AgentAttemptDiagnosticQuery,
) ([]AgentAttemptDiagnosticSummary, error) {
	source.queries = append([]AgentAttemptDiagnosticQuery(nil), queries...)
	return append([]AgentAttemptDiagnosticSummary(nil), source.summaries...), source.err
}

type apiTestTimelineLineageView struct {
	version   string
	execution projection.TeamExecution
	workItems map[string]projection.WorkItem
	runs      map[string]projection.Run
	evidence  map[string]projection.Evidence
	approvals map[string]projection.ProjectedApprovalRequest
}

func (view apiTestTimelineLineageView) Version() string { return view.version }

func (view apiTestTimelineLineageView) TeamExecution(
	id string,
) (projection.TeamExecution, bool) {
	return view.execution, view.execution.TeamInstanceID == id
}

func (view apiTestTimelineLineageView) WorkItem(
	id string,
) (projection.WorkItem, bool) {
	record, ok := view.workItems[id]
	return record, ok
}

func (view apiTestTimelineLineageView) WorkItemsForTeam(
	teamID string,
) []projection.WorkItem {
	records := make([]projection.WorkItem, 0)
	for _, record := range view.workItems {
		if record.TeamInstanceID == teamID {
			records = append(records, record)
		}
	}
	return records
}

func (view apiTestTimelineLineageView) Run(
	id string,
) (projection.Run, bool) {
	record, ok := view.runs[id]
	return record, ok
}

func (view apiTestTimelineLineageView) Evidence(
	id string,
) (projection.Evidence, bool) {
	record, ok := view.evidence[id]
	return record, ok
}

func (view apiTestTimelineLineageView) ApprovalRequest(
	id string,
) (projection.ProjectedApprovalRequest, bool) {
	record, ok := view.approvals[id]
	return record, ok
}

func (source *apiTestViewSource) Rebuild(context.Context) error {
	source.rebuilds++
	return source.rebuildErr
}

func (source *apiTestViewSource) GlobalReadView() projection.GlobalReadView {
	return source.view
}

func TestNewTeamExecutionStreamRejectsIncompleteConfiguration(t *testing.T) {
	now := func() time.Time {
		return time.Date(2026, 7, 26, 14, 0, 0, 0, time.UTC)
	}
	tests := []TeamExecutionStreamConfig{
		{},
		{TeamInstanceID: "team-1"},
		{TeamInstanceID: "team-1", Journal: &journal.Store{}},
		{
			TeamInstanceID: "team-1",
			Journal:        &journal.Store{},
			Projection:     &apiTestViewSource{},
		},
		{
			TeamInstanceID: "bad\nteam",
			Journal:        &journal.Store{},
			Projection:     &apiTestViewSource{},
			Now:            now,
		},
	}
	for index, config := range tests {
		if _, err := NewTeamExecutionStream(config); !errors.Is(err, ErrInvalidTimelineRequest) {
			t.Fatalf("case %d error = %v, want ErrInvalidTimelineRequest", index, err)
		}
	}
}

func TestPhase2DBoardRowProjectsSafeAgentBindingAndFailureReason(t *testing.T) {
	bindingBudget := int64(1_200)
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "conversation-1", TeamID: "team-1",
			AgentID: "agent-claude", RoleID: "reviewer",
			ProviderID: "anthropic", ProviderAccountID: "anthropic.production",
			ModelID: "claude-sonnet", AuthMode: "brokered",
			ContextAdapterID:   "context:claude-code:v1",
			DisclosurePolicyID: "policy.production", DisclosurePolicyVersion: 3,
			TokenBudget: 128,
		},
		[]contextcapsule.ItemInput{{
			ItemID: "goal-1", Kind: contextcapsule.KindConversationGoal,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			Priority: contextcapsule.PrioritySystem, TokenCount: 8, Required: true,
			Content:    []byte("Private Capsule content must not reach the board."),
			SourceType: contextcapsule.SourceAuthority, SourceRef: "goal:phase-2d",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	segment, err := contextcapsule.NewRouteSegmentBinding(
		contextcapsule.RouteSegmentBindingInput{
			SegmentID: "segment-reviewer-1", ConversationID: "conversation-1",
			TeamID: "team-1", AgentID: "agent-claude", RoleID: "reviewer",
			AttemptNumber: 1, CapsuleDigest: capsule.Digest(),
			ExecutionBindingDigest: strings.Repeat("b", 64),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	row := NodeBoardRow{}
	applyAttemptBindingToBoardRow(&row, projection.TeamExecutionAttempt{
		TerminalReason:            "provider_rejected",
		IncidentID:                "22222222-2222-4222-8222-222222222222",
		ExecutionBindingAvailable: true,
		ExecutionBinding: loomruntime.FrozenExecutionBinding{
			HarnessAdapter:      "claude-code",
			ProviderID:          "anthropic",
			ProviderAccountID:   "anthropic.production",
			ModelID:             "claude-sonnet",
			ReasoningEffort:     "high",
			Timeout:             5 * time.Minute,
			Budget:              &bindingBudget,
			Capabilities:        []string{"reasoning_effort", "workspace_edit"},
			CredentialReference: "credential-ref-private",
			CredentialRevision:  7,
			EndpointFingerprint: strings.Repeat("a", 64),
		},
		ContextCapsuleAvailable:            true,
		ContextCapsule:                     capsule.AuthorityRecord(),
		RouteSegmentAvailable:              true,
		RouteSegment:                       segment,
		AccountingAvailable:                true,
		ProviderAccountPolicyAvailable:     true,
		ProviderAccountPolicyVersion:       2,
		ProviderAccountPolicyRevision:      3,
		ProviderAccountPolicyDigest:        strings.Repeat("c", 64),
		ProviderAccountTrustDomain:         "external_provider",
		ProviderAccountRetentionMode:       "limited_retention",
		ProviderAccountDataRegion:          "eu",
		ProviderAccountAssignedBudgetUnits: 750,
		ProviderModelRateCardAvailable:     true,
		ProviderModelRateCard: projection.ProjectedProviderModelRateCard{
			Revision: 4, Digest: strings.Repeat("d", 64),
			Currency: "USD", InputTokenBasis: "input_excludes_cache",
		},
		Accounting: projection.RunAccounting{
			UsageObserved:  true,
			InputTokens:    90,
			OutputTokens:   10,
			TotalTokens:    100,
			CostObserved:   true,
			CostMicrounits: 450,
			CostCurrency:   "USD",
			CostSource:     "harness_reported",
		},
	})

	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, want := range []string{
		`"harness_adapter":"claude-code"`,
		`"provider_id":"anthropic"`,
		`"provider_account_id":"anthropic.production"`,
		`"model_id":"claude-sonnet"`,
		`"reasoning_effort":"high"`,
		`"timeout_nanoseconds":300000000000`,
		`"binding_budget_credits":1200`,
		`"capabilities":["reasoning_effort","workspace_edit"]`,
		`"credential_revision":7`,
		`"context_capsule_available":true`,
		`"context_capsule_digest":"` + capsule.Digest() + `"`,
		`"route_segment_available":true`,
		`"route_segment_id":"segment-reviewer-1"`,
		`"route_segment_digest":"` + segment.Digest + `"`,
		`"context_disclosure_receipt_digest":"` + capsule.DisclosureReceiptDigest() + `"`,
		`"context_adapter_id":"context:claude-code:v1"`,
		`"disclosure_policy_id":"policy.production"`,
		`"disclosure_policy_version":3`,
		`"context_token_count":8`,
		`"context_omission_count":0`,
		`"provider_account_policy_available":true`,
		`"provider_account_policy_version":2`,
		`"provider_account_policy_revision":3`,
		`"provider_account_trust_domain":"external_provider"`,
		`"provider_account_retention_mode":"limited_retention"`,
		`"provider_account_data_region":"eu"`,
		`"provider_account_assigned_budget_units":750`,
		`"provider_model_rate_card_available":true`,
		`"provider_model_rate_card_revision":4`,
		`"provider_model_rate_card_digest":"` + strings.Repeat("d", 64) + `"`,
		`"provider_model_rate_card_currency":"USD"`,
		`"provider_model_rate_card_input_basis":"input_excludes_cache"`,
		`"terminal_reason":"provider_rejected"`,
		`"incident_id":"22222222-2222-4222-8222-222222222222"`,
		`"accounting_available":true`,
		`"total_tokens":100`,
		`"cost_microunits":450`,
		`"cost_currency":"USD"`,
		`"cost_source":"harness_reported"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("board row missing %s: %s", want, text)
		}
	}
	for _, forbidden := range []string{
		"credential-ref-private",
		strings.Repeat("a", 64),
		"Private Capsule content must not reach the board.",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("board row exposed private binding data: %s", text)
		}
	}
	unsafe := NodeBoardRow{}
	applyAttemptBindingToBoardRow(&unsafe, projection.TeamExecutionAttempt{
		IncidentID: "unsafe\nincident",
	})
	if unsafe.IncidentID != "" {
		t.Fatalf("unsafe incident reached Board: %#v", unsafe)
	}
}

func TestPhase2DBoardEnrichesOnlyExactSafeAttemptDiagnostics(t *testing.T) {
	board := TeamBoard{nodes: []NodeBoardRow{
		{
			LogicalNodeID: "anthropic-reviewer", CurrentAttempt: 1,
			ExecutionBindingAvailable: true,
			ProviderID:                "anthropic", ProviderAccountID: "anthropic.production",
			ModelID: "claude-sonnet", IncidentID: "incident-anthropic-1",
		},
		{
			LogicalNodeID: "deepseek-coder", CurrentAttempt: 1,
			ExecutionBindingAvailable: true,
			ProviderID:                "deepseek", ProviderAccountID: "deepseek.primary",
			ModelID: "deepseek-chat", IncidentID: "incident-deepseek-1",
		},
		{
			LogicalNodeID: "openai-reviewer", CurrentAttempt: 1,
			ExecutionBindingAvailable: true,
			ProviderID:                "openai", ProviderAccountID: "openai.primary",
			ModelID: "gpt-5.5-codex", IncidentID: "incident-openai-1",
		},
		{
			LogicalNodeID: "minimax-coder", CurrentAttempt: 1,
			ExecutionBindingAvailable: true,
			ProviderID:                "minimax", ProviderAccountID: "minimax.primary",
			ModelID: "MiniMax-M3", IncidentID: "incident-minimax-1",
		},
		{
			LogicalNodeID: "kimi-researcher", CurrentAttempt: 1,
			ExecutionBindingAvailable: true,
			ProviderID:                "kimi", ProviderAccountID: "kimi.primary",
			ModelID: "kimi-k2.6", IncidentID: "incident-kimi-1",
		},
	}}
	source := &apiTestAgentAttemptDiagnosticSource{
		summaries: []AgentAttemptDiagnosticSummary{
			{
				IncidentID: "incident-anthropic-1",
				ProviderID: "anthropic", ProviderAccountID: "anthropic.production",
				ModelID: "claude-sonnet", FailureStage: "provider_auth",
				FailureCode: "provider_rejected", Retryable: false,
			},
			{
				IncidentID: "incident-deepseek-1",
				ProviderID: "deepseek", ProviderAccountID: "deepseek.other",
				ModelID: "deepseek-chat", FailureStage: "provider_rate_limit",
				FailureCode: "rate_limited", Retryable: true,
			},
			{
				IncidentID: "incident-deepseek-1",
				ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
				ModelID: "deepseek-chat", FailureStage: "provider_secret_body",
				FailureCode: "provider_unavailable", Retryable: true,
			},
			{
				IncidentID: "incident-deepseek-1",
				ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
				ModelID: "deepseek-chat", FailureStage: "provider_connect",
				FailureCode: "timeout", Retryable: true,
			},
			{
				IncidentID: "incident-minimax-1",
				ProviderID: "minimax", ProviderAccountID: "minimax.primary",
				ModelID: "MiniMax-M3", FailureStage: "credential_lease_revoke",
				FailureCode: "credential_unavailable", Retryable: true,
			},
			{
				IncidentID: "incident-kimi-1",
				ProviderID: "kimi", ProviderAccountID: "kimi.primary",
				ModelID: "kimi-k2.6", FailureStage: "vault_decrypt",
				FailureCode: "credential_unavailable", Retryable: false,
			},
		},
	}

	enrichBoardFailureDiagnostics(context.Background(), &board, source)

	if len(source.queries) != 5 {
		t.Fatalf("diagnostic queries = %#v", source.queries)
	}
	first := board.nodes[0]
	if !first.FailureDiagnosticAvailable || first.FailureStage != "provider_auth" ||
		first.FailureCode != "provider_rejected" || first.FailureRetryable {
		t.Fatalf("exact diagnostic not projected = %#v", first)
	}
	second := board.nodes[1]
	if !second.FailureDiagnosticAvailable || second.FailureStage != "provider_connect" ||
		second.FailureCode != "timeout" || !second.FailureRetryable {
		t.Fatalf("exact timeout diagnostic not projected = %#v", second)
	}
	third := board.nodes[2]
	if third.FailureDiagnosticAvailable || third.FailureStage != "" ||
		third.FailureCode != "" || third.FailureRetryable {
		t.Fatalf("peer inherited another account diagnostic = %#v", third)
	}
	fourth := board.nodes[3]
	if !fourth.FailureDiagnosticAvailable ||
		fourth.FailureStage != "credential_lease_revoke" ||
		fourth.FailureCode != "credential_unavailable" ||
		!fourth.FailureRetryable {
		t.Fatalf("revoked credential diagnostic not projected = %#v", fourth)
	}
	fifth := board.nodes[4]
	if !fifth.FailureDiagnosticAvailable || fifth.FailureStage != "vault_decrypt" ||
		fifth.FailureCode != "credential_unavailable" || fifth.FailureRetryable {
		t.Fatalf("corrupt Vault diagnostic not projected = %#v", fifth)
	}

	failing := &apiTestAgentAttemptDiagnosticSource{err: errors.New("diagnostics unavailable")}
	enrichBoardFailureDiagnostics(context.Background(), &board, failing)
	if !board.nodes[0].FailureDiagnosticAvailable ||
		!board.nodes[1].FailureDiagnosticAvailable ||
		board.nodes[2].FailureDiagnosticAvailable ||
		!board.nodes[3].FailureDiagnosticAvailable ||
		!board.nodes[4].FailureDiagnosticAvailable {
		t.Fatalf("diagnostic read failure mutated prior safe projection = %#v", board.nodes)
	}
}

func TestPhase2DBoardProjectsOnlyExactGovernedTestReportSummary(t *testing.T) {
	command, err := verification.RecognizeGovernedTestCommand("go test ./... -count=1")
	if err != nil {
		t.Fatal(err)
	}
	makeReport := func(callID string, sequence int64, exitCode int) verification.GovernedTestReport {
		report, reportErr := verification.NewGovernedTestReport(
			command,
			verification.GovernedTestReportInput{
				CallID: callID, CallSequence: sequence,
				ArgumentsDigest: strings.Repeat(string(rune('a'+sequence)), 64),
				ExecutionID:     "execution-" + callID, ExitCode: exitCode,
				OutputDigest: "sha256:" + strings.Repeat(string(rune('d'+sequence)), 64),
				DurationMS:   sequence * 10,
			},
		)
		if reportErr != nil {
			t.Fatal(reportErr)
		}
		return report
	}
	failed := makeReport("call-test-1", 1, 1)
	passed := makeReport("call-test-2", 2, 0)
	source := &apiTestGovernedTestReportSource{
		reports: []verification.GovernedTestReport{passed, failed},
	}
	execution := projection.TeamExecution{
		TeamInstanceID: "team-1",
		Nodes: []projection.TeamExecutionNode{{
			LogicalNodeID: "reviewer", CurrentAttempt: 1,
			Attempts: []projection.TeamExecutionAttempt{{
				AttemptNumber: 1, IncidentID: "22222222-2222-4222-8222-222222222222",
				WorkItemID: "work-reviewer", RunID: "run-reviewer",
				ClaimID: "33333333-3333-4333-8333-333333333333", ClaimGeneration: 2,
				RuntimeInstanceID: "runtime-reviewer", AgentInstanceID: "agent-reviewer",
				ExecutionBindingAvailable: true,
				ExecutionBinding: loomruntime.FrozenExecutionBinding{
					BindingDigest: strings.Repeat("b", 64),
				},
				ContextCapsuleAvailable: true,
				ContextCapsule: contextcapsule.AuthorityRecord{
					ConversationID: "conversation-1", CapsuleDigest: strings.Repeat("c", 64),
				},
			}},
		}},
	}
	board := TeamBoard{
		teamInstanceID: "team-1",
		nodes:          []NodeBoardRow{{LogicalNodeID: "reviewer", CurrentAttempt: 1}},
	}
	enrichBoardGovernedTestReports(context.Background(), &board, execution, source)
	if len(source.queries) != 1 {
		t.Fatalf("report queries = %#v", source.queries)
	}
	wantAuthority := attemptpayload.Authority{
		Scope: attemptpayload.Scope{
			ConversationID: "conversation-1", WorkItemID: "work-reviewer",
			RunID: "run-reviewer", ClaimGeneration: 2,
			RuntimeInstanceID:      "runtime-reviewer",
			ExecutionBindingDigest: strings.Repeat("b", 64),
			CapsuleDigest:          strings.Repeat("c", 64),
		},
		ClaimID:         "33333333-3333-4333-8333-333333333333",
		AgentInstanceID: "agent-reviewer",
		IncidentID:      "22222222-2222-4222-8222-222222222222",
	}
	if source.queries[0] != (work.AttemptReportQuery{
		TeamInstanceID: "team-1", Authority: wantAuthority,
	}) {
		t.Fatalf("report query = %#v", source.queries[0])
	}
	setDigest, err := verification.GovernedTestReportSetDigest(source.reports)
	if err != nil {
		t.Fatal(err)
	}
	row := board.nodes[0]
	if !row.TestReportAvailable || row.TestReportCount != 2 ||
		row.TestReportPassedCount != 1 || row.TestReportFailedCount != 1 ||
		row.TestReportSetDigest != setDigest || row.LatestTestRunner != "go_test" ||
		row.LatestTestScope != "all" || row.LatestTestOutcome != "passed" ||
		row.LatestTestReportDigest != passed.Digest() {
		t.Fatalf("test report Board summary = %#v", row)
	}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("go test")) || bytes.Contains(encoded, []byte("./...")) {
		t.Fatalf("test command entered Board: %s", encoded)
	}

	duplicate := &apiTestGovernedTestReportSource{
		reports: []verification.GovernedTestReport{passed, passed},
	}
	board.nodes[0] = NodeBoardRow{LogicalNodeID: "reviewer", CurrentAttempt: 1}
	enrichBoardGovernedTestReports(context.Background(), &board, execution, duplicate)
	if board.nodes[0].TestReportAvailable {
		t.Fatalf("duplicate report set reached Board: %#v", board.nodes[0])
	}
}

func TestPhase2DBoardProjectsInitialBlockWithoutAttempt(t *testing.T) {
	incidentID := "11111111-1111-4111-8111-111111111111"
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	digest := strings.Repeat("b", 64)
	now := time.Date(2026, 8, 12, 2, 0, 0, 0, time.UTC)
	events := []struct {
		id, kind, causation string
		payload             map[string]any
	}{
		{"plan-initial", "TeamExecutionPlanned", "", map[string]any{
			"team_instance_id": "team-initial", "plan_digest": digest, "view_version": strings.Repeat("a", 64),
			"nodes": []map[string]any{{"logical_node_id": "agent-kimi", "title": "Kimi", "agent_instance_id": "agent-kimi", "runtime_instance_id": "runtime-kimi", "role": "main", "depends_on": []string{}, "max_attempts": 1}},
			"route_summaries": []map[string]any{{
				"logical_node_id": "agent-kimi", "harness_adapter": "loom-native",
				"provider_id": "moonshot", "provider_account_id": "moonshot.primary",
				"model_id": "kimi-k2", "timeout_nanoseconds": int64(time.Minute),
				"budget": int64(25), "capabilities": []string{"chat"}, "credential_revision": int64(4),
			}},
		}},
		{"blocked-initial", "TeamNodeInitiallyBlocked", "plan-initial", map[string]any{
			"logical_node_id": "agent-kimi", "code": "credential_unavailable", "stage": "credential_lease_issue",
			"reason": "Credential is not verified.", "retryable": true, "source_logical_node_id": "",
		}},
		{"terminal-initial", "TeamExecutionTerminal", "blocked-initial", map[string]any{
			"team_instance_id": "team-initial", "plan_digest": digest, "status": "blocked", "reason": "node_agent-kimi_blocked",
		}},
	}
	for index, fixture := range events {
		payload, err := json.Marshal(fixture.payload)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Append(context.Background(), journal.Event{
			ID: fixture.id, StreamID: "team-execution/team-initial", Seq: int64(index + 1),
			IdempotencyKey: fixture.id, Type: fixture.kind, SchemaVersion: 1,
			EmittedAt: now, CorrelationID: incidentID, CausationID: fixture.causation, PayloadJSON: payload,
		}); err != nil {
			t.Fatal(err)
		}
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	board, attention := deriveBoardAndAttention(readModel.GlobalReadView(), "team-initial")
	if len(board.nodes) != 1 || len(attention) != 1 {
		t.Fatalf("board=%#v attention=%#v", board, attention)
	}
	row := board.nodes[0]
	if row.CurrentAttempt != 0 || !row.ExecutionBindingAvailable ||
		row.HarnessAdapter != "loom-native" || row.ProviderID != "moonshot" ||
		row.ProviderAccountID != "moonshot.primary" || row.ModelID != "kimi-k2" ||
		row.CredentialRevision != 4 || row.BindingBudgetCredits == nil || *row.BindingBudgetCredits != 25 ||
		row.IncidentID != incidentID || !row.FailureDiagnosticAvailable ||
		row.FailureStage != "credential_lease_issue" || row.FailureCode != "credential_unavailable" ||
		!row.FailureRetryable || row.TerminalReason != "Credential is not verified." {
		t.Fatalf("initial block row = %#v", row)
	}
}

func TestPhase2DBoardProjectsAgentAttemptRestartRecoveryWithoutRetryAuthority(t *testing.T) {
	board := TeamBoard{nodes: []NodeBoardRow{{
		LogicalNodeID: "deepseek-coder", CurrentAttempt: 1,
		ExecutionBindingAvailable: true,
		ProviderID:                "deepseek", ProviderAccountID: "deepseek.primary",
		ModelID: "deepseek-chat", IncidentID: "incident-deepseek-restart-1",
	}}}
	source := &apiTestAgentAttemptDiagnosticSource{
		summaries: []AgentAttemptDiagnosticSummary{{
			IncidentID: "incident-deepseek-restart-1",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			ModelID: "deepseek-chat", FailureStage: "agent_attempt_reconcile",
			FailureCode: "agent_input_resume_required", Retryable: false,
		}},
	}

	enrichBoardFailureDiagnostics(context.Background(), &board, source)

	if len(board.nodes) != 1 || !board.nodes[0].FailureDiagnosticAvailable ||
		board.nodes[0].FailureStage != "agent_attempt_reconcile" ||
		board.nodes[0].FailureCode != "agent_input_resume_required" ||
		board.nodes[0].FailureRetryable {
		t.Fatalf("restart recovery diagnostic = %#v", board.nodes)
	}
}

func TestPhase2DBoardProjectsParallelRouteAndAggregationIdentity(t *testing.T) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	payload, err := json.Marshal(map[string]any{
		"team_instance_id": "team-parallel-board",
		"plan_digest":      strings.Repeat("b", 64),
		"view_version":     strings.Repeat("a", 64),
		"nodes": []map[string]any{
			{"logical_node_id": "main", "title": "Synthesis", "agent_instance_id": "agent-main", "runtime_instance_id": "runtime-main", "role": "main", "kind": "aggregation", "route_group_id": "route-group-main", "depends_on": []string{"route-a", "route-b"}, "max_attempts": 1},
			{"logical_node_id": "route-a", "title": "Route A", "agent_instance_id": "agent-main", "runtime_instance_id": "runtime-a", "role": "main", "kind": "route_sibling", "route_group_id": "route-group-main", "depends_on": []string{}, "max_attempts": 1},
			{"logical_node_id": "route-b", "title": "Route B", "agent_instance_id": "agent-main", "runtime_instance_id": "runtime-b", "role": "main", "kind": "route_sibling", "route_group_id": "route-group-main", "depends_on": []string{}, "max_attempts": 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID: "plan-parallel-board", StreamID: "team-execution/team-parallel-board",
		Seq: 1, IdempotencyKey: "plan-parallel-board", Type: "TeamExecutionPlanned",
		SchemaVersion: 1, EmittedAt: time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC),
		CorrelationID: "11111111-1111-4111-8111-111111111111", PayloadJSON: payload,
	}); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	board, _ := deriveBoardAndAttention(readModel.GlobalReadView(), "team-parallel-board")
	if len(board.nodes) != 3 ||
		board.nodes[0].NodeKind != "aggregation" || board.nodes[0].RouteGroupID != "route-group-main" ||
		board.nodes[1].NodeKind != "route_sibling" || board.nodes[2].NodeKind != "route_sibling" {
		t.Fatalf("parallel board rows = %#v", board.nodes)
	}
	encoded, err := json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"node_kind":"aggregation"`)) ||
		!bytes.Contains(encoded, []byte(`"route_group_id":"route-group-main"`)) {
		t.Fatalf("parallel board JSON = %s", encoded)
	}
}

func TestPhase2DBoardRowProjectsFallbackGovernanceWithoutAuthorityDigests(t *testing.T) {
	row := NodeBoardRow{}
	applyFallbackGovernanceToBoardRow(&row, projection.TeamExecutionNode{
		WorkflowFallbackKey:       "fallback-private-route",
		RecoveryApprovalRequired:  true,
		FallbackApprovalAvailable: true,
		FallbackApprovalVersion:   3,
		FallbackApprovalID:        "fallback-approval-private",
		FallbackApprovalDigest:    strings.Repeat("b", 64),
		FallbackConsumed:          true,
	})

	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, want := range []string{
		`"fallback_configured":true`,
		`"recovery_approval_required":true`,
		`"fallback_approval_available":true`,
		`"fallback_approval_version":3`,
		`"fallback_consumed":true`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("board row missing %s: %s", want, text)
		}
	}
	for _, forbidden := range []string{
		"fallback-private-route",
		"fallback-approval-private",
		strings.Repeat("b", 64),
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("board row exposed fallback authority data: %s", text)
		}
	}
}

func TestPhase2DProviderAccountAccountingNeverUsesProviderGlobalBucket(t *testing.T) {
	budgetA := int64(1_000)
	budgetB := int64(2_000)
	execution := projection.TeamExecution{Nodes: []projection.TeamExecutionNode{
		{
			LogicalNodeID: "main",
			Attempts: []projection.TeamExecutionAttempt{{
				Status: "failed", TerminalReason: "provider_rate_limited",
				ExecutionBindingAvailable: true,
				ExecutionBinding: loomruntime.FrozenExecutionBinding{
					ProviderID: "openai", ProviderAccountID: "openai.work",
					Budget: &budgetA,
				},
				AccountingAvailable: true,
				Accounting: projection.RunAccounting{
					UsageObserved: true, InputTokens: 80, OutputTokens: 20,
					TotalTokens: 100, CostObserved: true,
					CostMicrounits: 500, CostCurrency: "USD",
					CostSource: "provider_reported",
				},
			}},
		},
		{
			LogicalNodeID: "review",
			Attempts: []projection.TeamExecutionAttempt{{
				Status: "running", ExecutionBindingAvailable: true,
				ExecutionBinding: loomruntime.FrozenExecutionBinding{
					ProviderID: "openai", ProviderAccountID: "openai.personal",
					Budget: &budgetB,
				},
				ProviderAccountPolicyAvailable:     true,
				ProviderAccountPolicyRevision:      2,
				ProviderAccountPolicyDigest:        strings.Repeat("d", 64),
				ProviderAccountAssignedBudgetUnits: 2_000,
			}},
		},
	}}

	accounts := aggregateProviderAccountBoardRows(projection.GlobalReadView{}, execution)
	if len(accounts) != 2 ||
		accounts[0].ProviderAccountID != "openai.personal" ||
		accounts[0].ActiveAttempts != 1 ||
		accounts[0].BudgetUnits != 2_000 ||
		accounts[0].ActiveAssignedBudgetUnits != 2_000 ||
		accounts[1].ProviderAccountID != "openai.work" ||
		accounts[1].FailedAttempts != 1 ||
		accounts[1].RateLimitedAttempts != 1 ||
		accounts[1].ErrorRateBasisPoints != 10_000 ||
		accounts[1].TotalTokens != 100 ||
		len(accounts[1].Costs) != 1 ||
		accounts[1].Costs[0].AmountMicrounits != 500 {
		t.Fatalf("Provider Account rows = %#v", accounts)
	}
	if accounts[1].Costs[0].Source != "provider_reported" {
		t.Fatalf("Provider Account cost source = %#v", accounts[1].Costs)
	}
}

func TestPhase2DProviderAccountCostsRemainSeparatedBySource(t *testing.T) {
	execution := projection.TeamExecution{Nodes: []projection.TeamExecutionNode{{
		LogicalNodeID: "main",
		Attempts: []projection.TeamExecutionAttempt{
			{
				Status: "succeeded", ExecutionBindingAvailable: true,
				ExecutionBinding: loomruntime.FrozenExecutionBinding{
					ProviderID: "openai", ProviderAccountID: "openai.work",
				},
				AccountingAvailable: true,
				Accounting: projection.RunAccounting{
					CostObserved: true, CostMicrounits: 500, CostCurrency: "USD",
					CostSource: "provider_reported",
				},
			},
			{
				Status: "succeeded", ExecutionBindingAvailable: true,
				ExecutionBinding: loomruntime.FrozenExecutionBinding{
					ProviderID: "openai", ProviderAccountID: "openai.work",
				},
				AccountingAvailable: true,
				Accounting: projection.RunAccounting{
					CostObserved: true, CostMicrounits: 700, CostCurrency: "USD",
					CostSource: "harness_reported",
				},
			},
		},
	}}}

	accounts := aggregateProviderAccountBoardRows(projection.GlobalReadView{}, execution)
	if len(accounts) != 1 || accounts[0].CostAttemptCount != 2 ||
		len(accounts[0].Costs) != 2 ||
		accounts[0].Costs[0] != (ProviderAccountCostRow{
			Currency: "USD", Source: "harness_reported", AmountMicrounits: 700,
		}) ||
		accounts[0].Costs[1] != (ProviderAccountCostRow{
			Currency: "USD", Source: "provider_reported", AmountMicrounits: 500,
		}) {
		t.Fatalf("Provider Account source-separated costs = %#v", accounts)
	}
}

func TestTimelineCursorCanonicalRoundTripAndTeamBinding(t *testing.T) {
	viewVersion := strings.Repeat("a", 64)
	cursor := timelineCursor{
		SchemaVersion:  1,
		TeamInstanceID: "team-1",
		ViewVersion:    viewVersion,
		Heads: []journal.StreamHead{
			{StreamID: "run/run-1", Sequence: 2, EventID: "run-event-2"},
			{StreamID: "team-execution/team-1"},
		},
	}
	cursor.ScopeDigest = timelineScopeDigest(cursor.Heads)
	encoded, err := encodeTimelineCursor(cursor)
	if err != nil {
		t.Fatalf("encodeTimelineCursor() error = %v", err)
	}
	decoded, err := decodeTimelineCursor(encoded, "team-1")
	if err != nil {
		t.Fatalf("decodeTimelineCursor() error = %v", err)
	}
	if decoded.TeamInstanceID != cursor.TeamInstanceID ||
		decoded.ScopeDigest != cursor.ScopeDigest ||
		decoded.ViewVersion != viewVersion ||
		len(decoded.Heads) != 2 ||
		decoded.Heads[0].StreamID != "run/run-1" {
		t.Fatalf("decoded cursor = %#v", decoded)
	}

	if _, err := decodeTimelineCursor(encoded, "team-2"); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("wrong-Team cursor error = %v", err)
	}
	padded := encoded + "="
	if _, err := decodeTimelineCursor(padded, "team-1"); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("padded cursor error = %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	noncanonical := base64.RawURLEncoding.EncodeToString(append(raw, ' '))
	if _, err := decodeTimelineCursor(noncanonical, "team-1"); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("noncanonical JSON cursor error = %v", err)
	}
}

func TestTimelineCursorRejectsNoncanonicalShapesAndEncodedOverflow(t *testing.T) {
	viewVersion := strings.Repeat("a", 64)
	baseHeads := []journal.StreamHead{
		{StreamID: "run/run-1", Sequence: 2, EventID: "run-event-2"},
		{StreamID: "team-execution/team-1"},
		{StreamID: "work-item/work-1", Sequence: 1, EventID: "work-event-1"},
	}
	var canonical string
	for iteration := 0; iteration < 50; iteration++ {
		permuted := append([]journal.StreamHead(nil), baseHeads...)
		offset := iteration % len(permuted)
		permuted = append(permuted[offset:], permuted[:offset]...)
		if iteration%2 == 1 {
			for left, right := 0, len(permuted)-1; left < right; left, right = left+1, right-1 {
				permuted[left], permuted[right] = permuted[right], permuted[left]
			}
		}
		cursor := timelineCursor{
			SchemaVersion:  1,
			TeamInstanceID: "team-1",
			ViewVersion:    viewVersion,
			Heads:          permuted,
		}
		cursor.ScopeDigest = timelineScopeDigest(
			append([]journal.StreamHead(nil), baseHeads...),
		)
		encoded, err := encodeTimelineCursor(cursor)
		if err != nil {
			t.Fatalf("permutation %d encode error = %v", iteration, err)
		}
		if canonical == "" {
			canonical = encoded
		} else if encoded != canonical {
			t.Fatalf("permutation %d cursor is noncanonical", iteration)
		}
	}

	wire := timelineCursorWire{
		SchemaVersion:  1,
		TeamInstanceID: "team-1",
		ViewVersion:    viewVersion,
		Heads: []cursorHeadWire{
			{StreamID: "work-item/work-1", Sequence: 1, EventID: "work-event-1"},
			{StreamID: "run/run-1", Sequence: 2, EventID: "run-event-2"},
		},
	}
	wire.ScopeDigest = timelineScopeDigest([]journal.StreamHead{
		{StreamID: "run/run-1", Sequence: 2, EventID: "run-event-2"},
		{StreamID: "work-item/work-1", Sequence: 1, EventID: "work-event-1"},
	})
	encodeWire := func(value any) string {
		data, err := marshalCompact(value)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(data)
	}
	if _, err := decodeTimelineCursor(
		encodeWire(wire),
		"team-1",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("unsorted heads error = %v", err)
	}

	duplicate := wire
	duplicate.Heads = []cursorHeadWire{
		{StreamID: "run/run-1"},
		{StreamID: "run/run-1"},
	}
	duplicate.ScopeDigest = strings.Repeat("b", 64)
	if _, err := decodeTimelineCursor(
		encodeWire(duplicate),
		"team-1",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("duplicate heads error = %v", err)
	}

	tooMany := wire
	tooMany.Heads = make([]cursorHeadWire, journal.MaxCursorStreams+1)
	headValues := make([]journal.StreamHead, len(tooMany.Heads))
	for index := range tooMany.Heads {
		streamID := fmt.Sprintf("stream/%03d", index)
		tooMany.Heads[index] = cursorHeadWire{StreamID: streamID}
		headValues[index] = journal.StreamHead{StreamID: streamID}
	}
	tooMany.ScopeDigest = timelineScopeDigest(headValues)
	if _, err := decodeTimelineCursor(
		encodeWire(tooMany),
		"team-1",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("97-head cursor error = %v", err)
	}

	canonicalJSON, err := base64.RawURLEncoding.DecodeString(canonical)
	if err != nil {
		t.Fatal(err)
	}
	unknown := strings.TrimSuffix(string(canonicalJSON), "}") +
		`,"unknown":true}`
	if _, err := decodeTimelineCursor(
		base64.RawURLEncoding.EncodeToString([]byte(unknown)),
		"team-1",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("unknown field error = %v", err)
	}
	duplicateField := strings.Replace(
		string(canonicalJSON),
		`"schema_version":1`,
		`"schema_version":1,"schema_version":1`,
		1,
	)
	if _, err := decodeTimelineCursor(
		base64.RawURLEncoding.EncodeToString([]byte(duplicateField)),
		"team-1",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("duplicate field error = %v", err)
	}

	longHeads := make([]journal.StreamHead, journal.MaxCursorStreams)
	for index := range longHeads {
		longHeads[index] = journal.StreamHead{
			StreamID: fmt.Sprintf(
				"%03d-%s",
				index,
				strings.Repeat("x", 508),
			),
		}
	}
	overflow := timelineCursor{
		SchemaVersion:  1,
		TeamInstanceID: "team-1",
		ViewVersion:    viewVersion,
		Heads:          longHeads,
	}
	overflow.ScopeDigest = timelineScopeDigest(longHeads)
	if _, err := encodeTimelineCursor(overflow); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("encoded overflow error = %v", err)
	}
}

func TestTimelineGapErrorPreservesOnlySafeTypedCausesAndCopiedPage(t *testing.T) {
	occurredAt := time.Date(2026, 7, 26, 14, 1, 0, 0, time.UTC)
	gap, err := newStreamGap(streamGapInput{
		TeamInstanceID:       "team-1",
		Reason:               "cursor_conflict",
		PreviousCursorDigest: strings.Repeat("b", 64),
		CurrentViewVersion:   strings.Repeat("c", 64),
		ArtifactDigest:       strings.Repeat("d", 64),
		OccurredAt:           occurredAt,
	})
	if err != nil {
		t.Fatalf("newStreamGap() error = %v", err)
	}
	page := newTimelineGapPage("team-1", gap)
	gapErr := newTimelineGapError(page, ErrTimelineCursorConflict)
	if !errors.Is(gapErr, ErrStreamGap) ||
		!errors.Is(gapErr, ErrTimelineCursorConflict) {
		t.Fatalf("gap error causes = %v", gapErr)
	}
	if strings.Contains(gapErr.Error(), "team-1") ||
		strings.Contains(gapErr.Error(), strings.Repeat("d", 64)) {
		t.Fatalf("gap error leaks safe payload details: %q", gapErr.Error())
	}
	first := gapErr.Page()
	firstGap, ok := first.Gap()
	if !ok {
		t.Fatal("gap page has no gap")
	}
	data, err := json.Marshal(firstGap)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":1,"delivery_id":"` +
		firstGap.DeliveryID() +
		`","kind":"stream_gap","team_instance_id":"team-1","reason":"cursor_conflict","previous_cursor_digest":"` +
		strings.Repeat("b", 64) +
		`","current_view_version":"` + strings.Repeat("c", 64) +
		`","artifact_available":true,"artifact_digest":"` +
		strings.Repeat("d", 64) +
		`","recoverable":true,"occurred_at":"2026-07-26T14:01:00Z"}`
	if string(data) != want {
		t.Fatalf("gap JSON = %s, want %s", data, want)
	}
}

func TestReadPageRejectsMalformedCursorAsRecoverableGapWithoutJournalFallback(t *testing.T) {
	source := &apiTestViewSource{}
	stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: "team-1",
		Journal:        &journal.Store{},
		Projection:     source,
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 14, 2, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewTeamExecutionStream() error = %v", err)
	}
	page, err := stream.ReadPage(context.Background(), "not-base64!", 128)
	if !errors.Is(err, ErrStreamGap) ||
		!errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("ReadPage() error = %v", err)
	}
	gap, ok := page.Gap()
	if !ok || gap.Reason() != "invalid_cursor" || !gap.Recoverable() {
		t.Fatalf("gap = %#v, %v", gap, ok)
	}
	attention := page.Attention()
	if len(attention) != 1 ||
		attention[0].Kind != "stream_gap" ||
		attention[0].ActionRequired != "reconnect" {
		t.Fatalf("gap Attention = %#v", attention)
	}
	if source.rebuilds > 1 {
		t.Fatalf("malformed cursor rebuilds = %d, want at most 1", source.rebuilds)
	}
}

func TestReadPageReturnsJournalAuthoritativeTimelineAndReconnectsWithoutDuplicate(t *testing.T) {
	ctx := context.Background()
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: "team-instance.one",
		Journal:        store,
		Projection:     readModel,
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 15, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := stream.ReadPage(ctx, "", 1)
	if err != nil {
		t.Fatalf("first ReadPage() error = %v", err)
	}
	if first.TeamInstanceID() != "team-instance.one" ||
		first.ViewVersion() == "" ||
		first.NextCursor() == "" ||
		first.HasMore() {
		t.Fatalf("first page = %#v", first)
	}
	records := first.Records()
	if len(records) != 1 ||
		records[0].kind != "team_planned" ||
		records[0].authority != "journal" ||
		records[0].sourceStreamID != "team-execution/team-instance.one" ||
		records[0].sourceSequence != 1 {
		t.Fatalf("first records = %#v", records)
	}
	data, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "source_record_set_digest") ||
		strings.Contains(string(data), "team_definition_digest") {
		t.Fatalf("timeline leaked raw Team payload: %s", data)
	}

	reconnected, err := stream.ReadPage(ctx, first.NextCursor(), 1)
	if err != nil {
		t.Fatalf("reconnected ReadPage() error = %v", err)
	}
	if len(reconnected.Records()) != 0 ||
		reconnected.NextCursor() == "" ||
		reconnected.HasMore() {
		t.Fatalf("reconnected page = %#v", reconnected)
	}

	for iteration := 0; iteration < 50; iteration++ {
		start, err := stream.ReadPage(ctx, "", 1)
		if err != nil {
			t.Fatalf("walk %d initial page error = %v", iteration, err)
		}
		seen := make(map[string]struct{})
		for _, record := range start.Records() {
			if _, duplicate := seen[record.deliveryID]; duplicate {
				t.Fatalf("walk %d duplicate delivery %s", iteration, record.deliveryID)
			}
			seen[record.deliveryID] = struct{}{}
		}
		resumed, err := stream.ReadPage(ctx, start.NextCursor(), 1)
		if err != nil {
			t.Fatalf("walk %d resumed page error = %v", iteration, err)
		}
		for _, record := range resumed.Records() {
			if _, duplicate := seen[record.deliveryID]; duplicate {
				t.Fatalf("walk %d duplicate delivery %s", iteration, record.deliveryID)
			}
			seen[record.deliveryID] = struct{}{}
		}
		if len(seen) != 1 {
			t.Fatalf("walk %d delivery count = %d, want 1", iteration, len(seen))
		}
	}
}

func TestReadPageConcurrentAppendAdvancesOnlyOnCurrentOrNextCursor(t *testing.T) {
	ctx := context.Background()
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: "team-instance.one",
		Journal:        store,
		Projection:     readModel,
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 15, 30, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := stream.ReadPage(ctx, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	startingCursor, err := decodeTimelineCursor(
		initial.NextCursor(),
		"team-instance.one",
	)
	if err != nil {
		t.Fatal(err)
	}
	startingCursor.Heads = append(
		startingCursor.Heads,
		journal.StreamHead{StreamID: "safe-extra"},
	)
	startingCursor.Heads, err = normalizeTimelineHeads(startingCursor.Heads)
	if err != nil {
		t.Fatal(err)
	}
	startingCursor.ScopeDigest = timelineScopeDigest(startingCursor.Heads)
	startingEncoded, err := encodeTimelineCursor(startingCursor)
	if err != nil {
		t.Fatal(err)
	}
	appended := journal.Event{
		ID:             "event.team.safe-unknown",
		StreamID:       "safe-extra",
		Seq:            1,
		IdempotencyKey: "key.team.safe-unknown",
		Type:           "SafeVocabularyUnknown",
		SchemaVersion:  1,
		EmittedAt:      time.Date(2026, 7, 26, 15, 30, 1, 0, time.UTC),
		PayloadJSON:    []byte(`{"safe":"value"}`),
	}
	start := make(chan struct{})
	appendResult := make(chan error, 1)
	go func() {
		<-start
		_, err := store.Append(ctx, appended)
		appendResult <- err
	}()
	close(start)
	current, err := stream.ReadPage(ctx, startingEncoded, 128)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-appendResult; err != nil {
		t.Fatal(err)
	}
	cursor := current.NextCursor()
	decoded, err := decodeTimelineCursor(cursor, "team-instance.one")
	if err != nil {
		t.Fatal(err)
	}
	headSequence := func(value timelineCursor) int64 {
		for _, head := range value.Heads {
			if head.StreamID == appended.StreamID {
				if head.EventID != "" && head.EventID != appended.ID &&
					head.Sequence == appended.Seq {
					t.Fatalf("fabricated event ID at sequence %d", head.Sequence)
				}
				return head.Sequence
			}
		}
		return -1
	}
	sequence := headSequence(decoded)
	if sequence == 0 {
		next, err := stream.ReadPage(ctx, cursor, 128)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err = decodeTimelineCursor(
			next.NextCursor(),
			"team-instance.one",
		)
		if err != nil {
			t.Fatal(err)
		}
		sequence = headSequence(decoded)
	}
	if sequence != 1 || len(current.Records()) != 0 {
		t.Fatalf(
			"concurrent append sequence = %d current records = %#v",
			sequence,
			current.Records(),
		)
	}
}

func TestSubscriptionCoalescesTentativeTextAndSurfacesOverflowGapFirst(t *testing.T) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	teamID := "team-instance.one"
	stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: teamID,
		Journal:        store,
		Projection:     projection.New(db),
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 16, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	subscription, err := stream.Subscribe(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Close()
	makeRecord := func(node, text string, sequence int64) DeliveryRecord {
		return DeliveryRecord{
			schemaVersion:   1,
			deliveryID:      strings.Repeat("a", 63) + strconv.FormatInt(sequence%10, 10),
			kind:            "node_output_delta",
			authority:       "tentative",
			teamInstanceID:  teamID,
			logicalNodeID:   node,
			attemptNumber:   1,
			sourceSequence:  sequence,
			sourceEventID:   "frame-" + strconv.FormatInt(sequence, 10),
			occurredAt:      time.Date(2026, 7, 26, 16, 0, int(sequence%60), 0, time.UTC),
			payload:         DeliveryPayload{textDelta: text},
			runID:           "run-1",
			claimGeneration: 1,
		}
	}
	subscription.enqueue(makeRecord("main", "hello ", 1))
	subscription.enqueue(makeRecord("main", "world", 2))
	item, err := subscription.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	delivery, ok := item.Delivery()
	if !ok ||
		delivery.payload.textDelta != "hello world" ||
		delivery.sourceSequence != 2 {
		t.Fatalf("coalesced delivery = %#v, %v", delivery, ok)
	}

	lineage, err := stream.Subscribe(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer lineage.Close()
	lineage.enqueue(makeRecord("main", "generation-1", 3))
	nextGeneration := makeRecord("main", "generation-2", 4)
	nextGeneration.claimGeneration = 2
	lineage.enqueue(nextGeneration)
	firstLineage, err := lineage.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	secondLineage, err := lineage.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	firstDelivery, firstOK := firstLineage.Delivery()
	secondDelivery, secondOK := secondLineage.Delivery()
	if !firstOK || !secondOK ||
		firstDelivery.payload.textDelta != "generation-1" ||
		secondDelivery.payload.textDelta != "generation-2" {
		t.Fatalf(
			"cross-generation deliveries = %#v %#v",
			firstDelivery,
			secondDelivery,
		)
	}

	for iteration := 0; iteration < 100; iteration++ {
		overflow, err := stream.Subscribe(context.Background(), "")
		if err != nil {
			t.Fatal(err)
		}
		var producers sync.WaitGroup
		for producer := 0; producer < 4; producer++ {
			producers.Add(1)
			go func(producer int) {
				defer producers.Done()
				for index := 0; index < 17; index++ {
					sequence := int64(producer*17 + index + 1)
					overflow.enqueue(makeRecord(
						fmt.Sprintf("node-%d-%02d", producer, index),
						"x",
						sequence,
					))
				}
			}(producer)
		}
		producers.Wait()
		first, err := overflow.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		gap, ok := first.Gap()
		if !ok || gap.Reason() != "tentative_overflow" {
			t.Fatalf(
				"iteration %d overflow first item = %#v gap=%#v, %v",
				iteration,
				first,
				gap,
				ok,
			)
		}
		if err := overflow.Close(); err != nil {
			t.Fatal(err)
		}
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := overflow.Next(cancelled); !errors.Is(err, context.Canceled) {
			t.Fatalf("iteration %d closed Next() error = %v", iteration, err)
		}
	}
	authoritative, err := stream.ReadPage(context.Background(), "", 128)
	if err != nil {
		t.Fatal(err)
	}
	records := authoritative.Records()
	if len(records) != 1 ||
		records[0].kind != "team_planned" ||
		records[0].authority != "journal" {
		t.Fatalf("authoritative recovery after overflow = %#v", records)
	}
}

func TestSubscribeRequiresTeamValidatesPositionAndBoundsSubscribers(t *testing.T) {
	ctx := context.Background()
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: "team-instance.one",
		Journal:        store,
		Projection:     readModel,
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 16, 30, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := stream.ReadPage(ctx, "", 128)
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := decodeTimelineCursor(
		page.NextCursor(),
		"team-instance.one",
	)
	if err != nil {
		t.Fatal(err)
	}
	for index := range cursor.Heads {
		if cursor.Heads[index].Sequence > 0 {
			cursor.Heads[index].EventID = "conflicting-event"
			break
		}
	}
	cursor.ScopeDigest = timelineScopeDigest(cursor.Heads)
	conflicting, err := encodeTimelineCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Subscribe(
		ctx,
		conflicting,
	); !errors.Is(err, ErrTimelineCursorConflict) {
		t.Fatalf("conflicting cursor error = %v", err)
	}
	if _, err := stream.Subscribe(
		ctx,
		"not-base64!",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("invalid cursor error = %v", err)
	}

	subscriptions := make([]*Subscription, 0, maxSubscribers)
	for index := 0; index < maxSubscribers; index++ {
		subscription, err := stream.Subscribe(ctx, "")
		if err != nil {
			t.Fatalf("subscription %d error = %v", index, err)
		}
		subscriptions = append(subscriptions, subscription)
	}
	if _, err := stream.Subscribe(
		ctx,
		"",
	); !errors.Is(err, ErrTooManySubscribers) {
		t.Fatalf("ninth subscription error = %v", err)
	}
	if err := subscriptions[0].Close(); err != nil {
		t.Fatal(err)
	}
	replacement, err := stream.Subscribe(ctx, "")
	if err != nil {
		t.Fatalf("replacement subscription error = %v", err)
	}
	subscriptions[0] = replacement
	for _, subscription := range subscriptions {
		if err := subscription.Close(); err != nil {
			t.Fatal(err)
		}
	}

	missing, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: "team-missing",
		Journal:        store,
		Projection:     readModel,
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 16, 30, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missing.Subscribe(
		ctx,
		"",
	); !errors.Is(err, ErrTeamTimelineNotFound) {
		t.Fatalf("missing Team subscription error = %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := stream.Subscribe(
		cancelled,
		"",
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled subscription error = %v", err)
	}
}

func TestTentativeDeltaRequiresExactBoundedSafeJSON(t *testing.T) {
	valid, err := decodeTentativeDelta([]byte(`{"delta":"line one\nline two\tok"}`))
	if err != nil || valid != "line one\nline two\tok" {
		t.Fatalf("valid delta = %q, %v", valid, err)
	}
	tests := [][]byte{
		[]byte(`{}`),
		[]byte(`{"delta":""}`),
		[]byte(`{"delta":1}`),
		[]byte(`{"delta":"ok","extra":true}`),
		[]byte(`{"delta":"one","delta":"two"}`),
		[]byte(`{"delta":"bad\u0000control"}`),
		[]byte(`{"delta":"` + strings.Repeat("x", maxTentativeDelta+1) + `"}`),
		{0xff, 0xfe},
	}
	for index, payload := range tests {
		if _, err := decodeTentativeDelta(
			payload,
		); !errors.Is(err, ErrInvalidNodeOutput) {
			t.Fatalf("case %d error = %v", index, err)
		}
	}
}

func TestAuthoritativeSafeFieldsRejectAmbiguityAndIgnoreRawKeys(t *testing.T) {
	fields, err := safeEventFields([]byte(
		`{"logical_node_id":"main","attempt_number":1,` +
			`"status":"running","retry_at":"2026-07-26T16:30:00Z",` +
			`"evidence_digest":"` + strings.Repeat("a", 64) + `",` +
			`"raw_grant":"never-publish"}`,
	))
	if err != nil ||
		fields["logical_node_id"] != "main" ||
		fields["attempt_number"] != "1" ||
		fields["status"] != "running" ||
		fields["retry_at"] != "2026-07-26T16:30:00Z" ||
		fields["evidence_digest"] != strings.Repeat("a", 64) {
		t.Fatalf("safe fields = %#v, %v", fields, err)
	}
	if _, exists := fields["raw_grant"]; exists {
		t.Fatalf("raw key escaped safe mapping: %#v", fields)
	}
	for index, payload := range [][]byte{
		[]byte(`{"status":"one","status":"two"}`),
		[]byte(`{"attempt_number":"1"}`),
		[]byte(`{"status":{"nested":true}}`),
		[]byte(`{"reason":"bad\u0000control"}`),
		[]byte(`{"retry_at":"tomorrow"}`),
		[]byte(`{"retry_at":"2026-07-26T16:30:00+08:00"}`),
		[]byte(`{"evidence_digest":"not-a-digest"}`),
		[]byte(`{"source_evidence_digest":"` + strings.Repeat("A", 64) + `"}`),
	} {
		if _, err := safeEventFields(
			payload,
		); !errors.Is(err, ErrInvalidDeliveryRecord) {
			t.Fatalf("case %d error = %v", index, err)
		}
	}
}

func TestDeliveryLineageFieldsAcceptOnlyIndependentlyCorroboratedPartialMetadata(
	t *testing.T,
) {
	for name, fields := range map[string]map[string]string{
		"logical node only": {"logical_node_id": "main"},
		"attempt only":      {"attempt_number": "1"},
		"both": {
			"logical_node_id": "main",
			"attempt_number":  "1",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateDeliveryLineageFields(
				true,
				"main",
				1,
				fields,
			); err != nil {
				t.Fatalf("matching partial lineage error = %v", err)
			}
		})
	}

	for name, fields := range map[string]map[string]string{
		"logical node mismatch": {"logical_node_id": "other"},
		"attempt mismatch":      {"attempt_number": "2"},
		"zero attempt":          {"attempt_number": "0"},
		"malformed attempt":     {"attempt_number": "one"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateDeliveryLineageFields(
				true,
				"main",
				1,
				fields,
			); !errors.Is(err, ErrInvalidDeliveryRecord) {
				t.Fatalf("mismatched partial lineage error = %v", err)
			}
		})
	}

	if err := validateDeliveryLineageFields(
		false,
		"",
		0,
		map[string]string{"attempt_number": "1"},
	); !errors.Is(err, ErrInvalidDeliveryRecord) {
		t.Fatalf("unbound partial lineage error = %v", err)
	}
}

func TestAuthoritativeMappingAcceptsAttemptOnlyRejectionAndRejectsMalformedPresence(
	t *testing.T,
) {
	teamID := "team-1"
	workItemID := "work-1"
	view := apiTestTimelineLineageView{
		version: strings.Repeat("a", 64),
		execution: projection.TeamExecution{
			TeamInstanceID: teamID,
			Nodes: []projection.TeamExecutionNode{{
				LogicalNodeID: "main",
				Attempts: []projection.TeamExecutionAttempt{{
					AttemptNumber: 1,
					WorkItemID:    workItemID,
				}},
			}},
		},
		workItems: map[string]projection.WorkItem{
			workItemID: {
				ID:             workItemID,
				TeamInstanceID: teamID,
				LogicalNodeID:  "main",
				AttemptNumber:  1,
			},
		},
	}
	heads := []journal.StreamHead{{StreamID: "work-item/" + workItemID}}
	mapPayload := func(payload string, selected apiTestTimelineLineageView) (
		[]DeliveryRecord,
		error,
	) {
		return mapAuthoritativeRecords(
			teamID,
			selected,
			heads,
			[]journal.Event{{
				ID:             "event-rejected",
				StreamID:       "work-item/" + workItemID,
				Seq:            1,
				IdempotencyKey: "event-rejected",
				Type:           "WorkItemRejected",
				SchemaVersion:  1,
				EmittedAt: time.Date(
					2026, 8, 3, 0, 0, 0, 0, time.UTC,
				),
				PayloadJSON: []byte(payload),
			}},
		)
	}

	records, err := mapPayload(
		`{"attempt_number":1,"status":"rejected"}`,
		view,
	)
	if err != nil || len(records) != 1 ||
		records[0].kind != "verification_rejected" ||
		records[0].logicalNodeID != "main" ||
		records[0].attemptNumber != 1 {
		t.Fatalf("attempt-only rejection records = %#v, %v", records, err)
	}

	for name, payload := range map[string]string{
		"present empty node": `{"logical_node_id":"","attempt_number":1}`,
		"node mismatch":      `{"logical_node_id":"other","attempt_number":1}`,
		"attempt mismatch":   `{"attempt_number":2}`,
		"zero attempt":       `{"attempt_number":0}`,
		"malformed attempt":  `{"attempt_number":"one"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := mapPayload(payload, view); !errors.Is(
				err,
				ErrInvalidDeliveryRecord,
			) {
				t.Fatalf("malformed mapping error = %v", err)
			}
		})
	}

	unbound := view
	unbound.workItems = map[string]projection.WorkItem{}
	if _, err := mapPayload(
		`{"attempt_number":1}`,
		unbound,
	); !errors.Is(err, ErrInvalidDeliveryRecord) {
		t.Fatalf("unbound mapping error = %v", err)
	}
}

func TestAuthoritativeKindVocabularyIsExact(t *testing.T) {
	want := map[string]string{
		"TeamExecutionPlanned":          "team_planned",
		"TeamNodeInitiallyBlocked":      "node_initially_blocked",
		"TeamNodeAttemptScheduled":      "node_scheduled",
		"TeamReadySetDispatched":        "ready_set_dispatched",
		"TeamNodeAttemptRebound":        "node_rebound",
		"RunStarted":                    "run_started",
		"RunTerminalCommitted":          "run_terminal",
		"WorkItemApprovalPaused":        "approval_required",
		"ApprovalRequested":             "approval_requested",
		"ApprovalDecided":               "approval_decided",
		"ApprovalExpired":               "approval_expired",
		"WorkItemApprovalResolved":      "approval_resolved",
		"TeamNodeAttemptTerminal":       "node_attempt_terminal",
		"WorkItemReadyForReview":        "ready_for_review",
		"WorkItemVerificationCommitted": "verification_recorded",
		"WorkItemDone":                  "work_item_done",
		"WorkItemRejected":              "verification_rejected",
		"TeamNodeAcceptanceCommitted":   "node_acceptance",
		"TeamNodeRecoveryRecorded":      "node_recovery",
		"EvidenceSubmitted":             "evidence_available",
		"TeamExecutionTerminal":         "team_terminal",
	}
	if !reflect.DeepEqual(authoritativeKinds, want) {
		t.Fatalf("authoritativeKinds = %#v, want %#v", authoritativeKinds, want)
	}
}

func openAPITimelineDB(t *testing.T) *sql.DB {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s?%s",
		filepath.Join(t.TempDir(), "timeline.db"),
		values.Encode(),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func appendAPITimelineFixture(t *testing.T, store *journal.Store) {
	t.Helper()
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	teamPayload := map[string]any{
		"team": map[string]any{
			"id":                      "team-instance.one",
			"work_request_id":         "request.one",
			"source_kind":             "saved_team",
			"team_definition_id":      "team.delivery",
			"team_definition_version": 1,
			"team_definition_scope":   "project",
			"scope_identity": map[string]any{
				"project_id":    "project.one",
				"generation_id": "",
			},
			"team_definition_digest": digestA,
			"source_plan_digest":     digestB,
			"state":                  "created",
			"created_at":             int64(1_721_865_600),
		},
		"dormant_sub_agents":       []any{},
		"source_plan_digest":       digestB,
		"source_record_set_digest": digestA,
		"team_instance_count":      1,
		"agent_instance_count":     1,
		"active_sub_agent_count":   0,
		"work_item_count":          0,
	}
	planPayload := map[string]any{
		"team_instance_id": "team-instance.one",
		"plan_digest":      digestA,
		"view_version":     digestB,
		"nodes": []map[string]any{{
			"logical_node_id":     "main",
			"title":               "Main",
			"agent_instance_id":   "agent-instance.main",
			"runtime_instance_id": "runtime.shared",
			"role":                "main",
			"depends_on":          []string{},
			"max_attempts":        2,
		}},
	}
	agentPayload := map[string]any{
		"main_agent": map[string]any{
			"id":                       "agent-instance.main",
			"team_instance_id":         "team-instance.one",
			"agent_definition_id":      "agent.main",
			"agent_definition_version": 1,
			"agent_definition_scope":   "project",
			"scope_identity": map[string]any{
				"project_id":    "project.one",
				"generation_id": "",
			},
			"runtime_profile_id":  "profile.main",
			"runtime_instance_id": "runtime.shared",
			"is_main":             true,
			"state":               "created",
		},
		"runtime_binding": map[string]any{
			"accepted":    true,
			"profile_id":  "profile.main",
			"instance_id": "runtime.shared",
		},
		"source_plan_digest":       digestB,
		"source_record_set_digest": digestA,
		"team_created_at":          int64(1_721_865_600),
		"binding_digest":           digestB,
		"runtime_discovery_digest": digestA,
	}
	encode := func(value any) []byte {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	events := []journal.Event{
		{
			ID:             "event.agent.created",
			StreamID:       "agent_instance:agent-instance.main",
			Seq:            1,
			IdempotencyKey: "key.agent.created",
			Type:           "AgentInstanceCreated",
			SchemaVersion:  1,
			EmittedAt:      time.Date(2026, 7, 25, 2, 30, 0, 0, time.UTC),
			CorrelationID:  "request.one",
			CausationID:    "event.team.created",
			PayloadJSON:    encode(agentPayload),
		},
		{
			ID:             "event.team.created",
			StreamID:       "team_instance:team-instance.one",
			Seq:            1,
			IdempotencyKey: "key.team.created",
			Type:           "TeamInstanceCreated",
			SchemaVersion:  1,
			EmittedAt:      time.Date(2026, 7, 25, 2, 30, 0, 0, time.UTC),
			CorrelationID:  "request.one",
			PayloadJSON:    encode(teamPayload),
		},
		{
			ID:             "event.team.planned",
			StreamID:       "team-execution/team-instance.one",
			Seq:            1,
			IdempotencyKey: "key.team.planned",
			Type:           "TeamExecutionPlanned",
			SchemaVersion:  1,
			EmittedAt:      time.Date(2026, 7, 26, 1, 2, 3, 0, time.UTC),
			PayloadJSON:    encode(planPayload),
		},
	}
	for _, event := range events {
		if _, err := store.Append(context.Background(), event); err != nil {
			t.Fatalf("Append(%s) error = %v", event.ID, err)
		}
	}
}
