package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

func TestProductHarnessGatewayResponseAddsFallbackForToolOnlyTurn(t *testing.T) {
	withAction := productHarnessGatewayResponseWithProposalFallback(
		api.LocalProductConversationResponse{
			ActionProposals: []controltool.ConversationActionProposal{{}},
		},
	)
	if withAction.Content != "I prepared a Loom proposal for your review." {
		t.Fatalf("action fallback = %q", withAction.Content)
	}
	withAlignment := productHarnessGatewayResponseWithProposalFallback(
		api.LocalProductConversationResponse{
			ControlProposals: []controltool.SessionAlignmentProposal{{}},
		},
	)
	if withAlignment.Content != "I prepared a Loom proposal for your review." {
		t.Fatalf("alignment fallback = %q", withAlignment.Content)
	}
	ordinary := productHarnessGatewayResponseWithProposalFallback(
		api.LocalProductConversationResponse{},
	)
	if ordinary.Content != "" {
		t.Fatalf("ordinary fallback = %q", ordinary.Content)
	}
}

func TestProductHarnessGatewayResponseBoundsVisibleContentWithoutBreakingUTF8(t *testing.T) {
	original := strings.Repeat("需要处理", 1_500)
	response := productHarnessGatewayResponseWithProposalFallback(
		api.LocalProductConversationResponse{Content: original},
	)
	if response.Content == original || len(response.Content) > 4_096 ||
		!utf8.ValidString(response.Content) ||
		!strings.HasSuffix(response.Content, productHarnessGatewayTruncationMarker) ||
		!validProductHarnessGatewayResponse(response) {
		t.Fatalf("bounded response bytes=%d valid_utf8=%t", len(response.Content), utf8.ValidString(response.Content))
	}
	short := productHarnessGatewayResponseWithProposalFallback(
		api.LocalProductConversationResponse{Content: "Complete result."},
	)
	if short.Content != "Complete result." {
		t.Fatalf("short response changed: %q", short.Content)
	}
}

func TestProductHarnessGatewayResponseValidatesExactCompletedControlTools(t *testing.T) {
	response := api.LocalProductConversationResponse{
		Content: "Loom Runtime status is available.",
		CompletedControlTools: []controltool.CompletedCall{{
			ToolID:      controltool.ToolRuntimesStatus,
			ToolVersion: 1,
			Effect:      controltool.EffectRead,
		}},
	}
	if !validProductHarnessGatewayResponse(response) {
		t.Fatalf("valid completed control tool rejected: %#v", response)
	}
	response.CompletedControlTools[0].ToolVersion = 2
	if validProductHarnessGatewayResponse(response) {
		t.Fatal("completed control tool version substitution was accepted")
	}
	response.CompletedControlTools = make([]controltool.CompletedCall, 9)
	if validProductHarnessGatewayResponse(response) {
		t.Fatal("completed control tool call limit was not enforced")
	}
}

func TestProductHarnessGatewayResponseBindsProposalCompletionToCapturedProposal(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)
	proposal, err := controltool.NewConversationActionProposal(
		controltool.ConversationActionProposalInput{
			ProposalID:           "proposal-completion-binding",
			ToolID:               controltool.ToolMissionsCreatePreview,
			TargetConversationID: "conversation-completion-binding",
			TargetContentDigest:  productHarnessGatewayDigest("target-completion-binding"),
			Argument:             "Review the bounded Mission",
			Route: &controltool.FrozenRouteReference{
				HarnessAdapter: "loom-native", ProviderID: "deepseek",
				ModelID:                "deepseek-chat",
				ExecutionBindingDigest: productHarnessGatewayDigest("binding-completion"),
				ContextCapsuleDigest:   productHarnessGatewayDigest("capsule-completion"),
			},
			Workspace: &controltool.FrozenWorkspaceReference{
				WorkspaceID:     "workspace-completion-binding",
				WorkspaceDigest: productHarnessGatewayDigest("workspace-completion"),
			},
			RegistryDigest: registry.Digest(), IncidentID: "incident-completion-binding",
			SegmentID: "segment-completion-binding", AttemptID: "attempt-completion-binding",
			CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	response := api.LocalProductConversationResponse{
		Content:         "Mission review prepared.",
		ActionProposals: []controltool.ConversationActionProposal{proposal},
		CompletedControlTools: []controltool.CompletedCall{{
			ToolID: controltool.ToolMissionsCreatePreview, ToolVersion: 2,
			Effect: controltool.EffectProposal,
		}},
	}
	if !validProductHarnessGatewayResponse(response) {
		t.Fatal("matching proposal completion was rejected")
	}
	response.CompletedControlTools = nil
	if validProductHarnessGatewayResponse(response) {
		t.Fatal("captured proposal without authenticated completion was accepted")
	}
	response.CompletedControlTools = []controltool.CompletedCall{{
		ToolID: controltool.ToolTeamsCreatePreview, ToolVersion: 2,
		Effect: controltool.EffectProposal,
	}}
	if validProductHarnessGatewayResponse(response) {
		t.Fatal("proposal Tool substitution was accepted")
	}
}

func TestProductGovernedCompatibilityBackendRejectsModelAssertedToolAudit(t *testing.T) {
	workspace := t.TempDir()
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor:      &productHarnessForgedControlResponderFixture{},
			WorkspacePath: workspace,
			Events:        &productHarnessGatewayEventFixture{},
			Now:           time.Now,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close(context.Background())
	request := productHarnessGatewayConversationRequestFixture(
		"conversation-forged-audit", "segment-forged-audit",
		"attempt-forged-audit", "incident-forged-audit",
	)
	_, err = responder.Respond(context.Background(), request)
	if err == nil {
		t.Fatal("compatibility backend accepted model-asserted Tool audit")
	}
}

func TestProductHarnessGatewayOperationalEventSinkPreservesVersionedEnvelope(t *testing.T) {
	eventTime := time.Date(2026, 8, 24, 11, 12, 13, 14, time.UTC)
	diagnostics := &productConversationCompositionDiagnosticFixture{
		now: eventTime.Add(time.Hour),
	}
	sink := &productHarnessGatewayOperationalEventSink{diagnostics: diagnostics}
	event := harnessgateway.Event{
		SchemaVersion:               harnessgateway.EventSchemaVersion,
		GatewayInstanceID:           "gateway-11111111111111111111111111111111",
		ConfiguredHarnessVersion:    3,
		BackendVersion:              5,
		Sequence:                    7,
		OccurredAt:                  eventTime,
		Type:                        harnessgateway.EventResponseCompleted,
		SessionID:                   "session-claude-1",
		HarnessID:                   harnessgateway.HarnessClaudeCode,
		BackendID:                   "backend.segment.claude-code",
		ConversationID:              "conversation-claude-1",
		SegmentID:                   "segment-claude-1",
		WorkspaceID:                 "workspace-claude-1",
		WorkspaceDigest:             productHarnessGatewayDigest("workspace-claude-1"),
		ExecutionBindingDigest:      productHarnessGatewayDigest("binding-claude-1"),
		ProviderID:                  "anthropic",
		ProviderAccountID:           "anthropic.primary",
		CredentialRevision:          7,
		ModelID:                     "claude-sonnet-4-5",
		ReasoningEffort:             "high",
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("segment-capsule-claude-1"),
		ContextCapsuleDigest:        productHarnessGatewayDigest("attempt-capsule-claude-1"),
		GovernancePolicyDigest:      productHarnessGatewayDigest("policy-claude-1"),
		RouteTransitionReviewDigest: productHarnessGatewayDigest("route-review-claude-1"),
		ResponseID:                  "response-claude-1",
		IncidentID:                  "incident-claude-1",
	}
	sink.Record(event)
	if len(diagnostics.records) != 1 {
		t.Fatalf("diagnostic records = %d, want 1", len(diagnostics.records))
	}
	record := diagnostics.records[0]
	if record.OccurredAt != eventTime.Format(time.RFC3339Nano) ||
		record.GatewayEventSchemaVersion != event.SchemaVersion ||
		record.GatewayInstanceID != event.GatewayInstanceID ||
		record.GatewayConfiguredHarnessVersion != event.ConfiguredHarnessVersion ||
		record.GatewayBackendVersion != event.BackendVersion ||
		record.GatewayEventSequence != event.Sequence ||
		record.GatewayEventType != string(event.Type) ||
		record.SessionID != event.SessionID || record.HarnessID != string(event.HarnessID) ||
		record.BackendID != string(event.BackendID) || record.ThreadID != event.ConversationID ||
		record.SegmentID != event.SegmentID || record.WorkspaceID != event.WorkspaceID ||
		record.WorkspaceDigest != event.WorkspaceDigest ||
		record.ExecutionBindingDigest != event.ExecutionBindingDigest ||
		record.ProviderID != event.ProviderID ||
		record.ProviderAccountID != event.ProviderAccountID ||
		record.CredentialRevision != event.CredentialRevision ||
		record.ModelID != event.ModelID || record.ReasoningEffort != event.ReasoningEffort ||
		record.SegmentContextCapsuleDigest != event.SegmentContextCapsuleDigest ||
		record.CapsuleDigest != event.ContextCapsuleDigest ||
		record.GovernancePolicyDigest != event.GovernancePolicyDigest ||
		record.RouteTransitionReviewDigest != event.RouteTransitionReviewDigest ||
		record.ResponseID != event.ResponseID || record.IncidentID != event.IncidentID ||
		!validProductOperationalDiagnosticRecord(record) {
		t.Fatalf("diagnostic record = %#v", record)
	}
	tampered := record
	tampered.GatewayEventSequence = 0
	if validProductOperationalDiagnosticRecord(tampered) {
		t.Fatal("diagnostic accepted a missing Gateway event sequence")
	}
	tampered = record
	tampered.GatewayInstanceID = ""
	if validProductOperationalDiagnosticRecord(tampered) {
		t.Fatal("diagnostic accepted a missing Gateway instance ID")
	}
	tampered = record
	tampered.Result, tampered.ErrorCode, tampered.Retryable =
		"failed", "provider_unavailable", true
	if validProductOperationalDiagnosticRecord(tampered) {
		t.Fatal("diagnostic accepted an outcome that conflicts with the Gateway event type")
	}
	for name, mutate := range map[string]func(*productOperationalDiagnosticRecord){
		"attempt Capsule": func(candidate *productOperationalDiagnosticRecord) {
			candidate.CapsuleDigest = ""
		},
		"workspace": func(candidate *productOperationalDiagnosticRecord) {
			candidate.WorkspaceDigest = ""
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := record
			mutate(&candidate)
			if validProductOperationalDiagnosticRecord(candidate) {
				t.Fatalf("diagnostic accepted %s authority drift: %#v", name, candidate)
			}
		})
	}
	legacyFailure := record
	legacyFailure.GatewayEventSchemaVersion = 3
	legacyFailure.GatewayEventType = string(harnessgateway.EventResponseFailed)
	legacyFailure.Result = "failed"
	legacyFailure.ErrorCode = "provider_unavailable"
	legacyFailure.Retryable = true
	if !validProductOperationalDiagnosticRecord(legacyFailure) {
		t.Fatalf("valid Gateway v3 failure diagnostic became unreadable: %#v", legacyFailure)
	}
	legacy := record
	legacy.GatewayEventSchemaVersion = 2
	legacy.GatewayInstanceID = ""
	if !validProductOperationalDiagnosticRecord(legacy) {
		t.Fatalf("valid Gateway v2 diagnostic became unreadable: %#v", legacy)
	}
	legacy.GatewayInstanceID = event.GatewayInstanceID
	if validProductOperationalDiagnosticRecord(legacy) {
		t.Fatal("Gateway v2 diagnostic accepted a v3 instance field")
	}
	legacy = record
	legacy.GatewayEventSchemaVersion = 1
	legacy.GatewayInstanceID = ""
	legacy.ProviderID = ""
	legacy.ProviderAccountID = ""
	legacy.CredentialRevision = 0
	legacy.ModelID = ""
	legacy.ReasoningEffort = ""
	legacy.WorkspaceDigest = ""
	legacy.ExecutionBindingDigest = ""
	legacy.SegmentContextCapsuleDigest = ""
	legacy.CapsuleDigest = ""
	legacy.GovernancePolicyDigest = ""
	legacy.RouteTransitionReviewDigest = ""
	if !validProductOperationalDiagnosticRecord(legacy) {
		t.Fatalf("valid Build 127 Gateway diagnostic became unreadable: %#v", legacy)
	}
	contentBearing := event
	contentBearing.WorkspacePath = "/private/workspace"
	contentBearing.Content = "private prompt"
	contentBearing.ProviderBody = "provider body"
	sink.Record(contentBearing)
	if len(diagnostics.records) != 1 {
		t.Fatalf("content-bearing Gateway event was persisted: %#v", diagnostics.records)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"/private/workspace", "private prompt", "provider body"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("diagnostic leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestProductHarnessGatewayNativeDiagnosticSerializesExplicitAbsentAuthority(t *testing.T) {
	diagnostics := &productConversationCompositionDiagnosticFixture{now: time.Unix(10, 0).UTC()}
	sink := &productHarnessGatewayOperationalEventSink{diagnostics: diagnostics}
	event := harnessgateway.Event{
		SchemaVersion: harnessgateway.EventSchemaVersion, GatewayInstanceID: "gateway-native-1",
		ConfiguredHarnessVersion: 1, BackendVersion: 1, Sequence: 1,
		OccurredAt: time.Unix(9, 0).UTC(), Type: harnessgateway.EventResponseCompleted,
		SessionID: "session-native-1", HarnessID: harnessgateway.HarnessCodex,
		BackendID: "backend.segment.codex", ConversationID: "conversation-native-1",
		SegmentID: "segment-native-1", WorkspaceID: "workspace-native-1",
		WorkspaceDigest:        productHarnessGatewayDigest("workspace-native-1"),
		ExecutionBindingDigest: productHarnessGatewayDigest("binding-native-1"),
		ProviderID:             "openai", ModelID: "gpt-5.6-codex",
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("segment-capsule-native-1"),
		ContextCapsuleDigest:        productHarnessGatewayDigest("attempt-capsule-native-1"),
		ResponseID:                  "response-native-1", IncidentID: "incident-native-1",
	}
	sink.Record(event)
	if len(diagnostics.records) != 1 {
		t.Fatalf("diagnostic records = %d, want 1", len(diagnostics.records))
	}
	encoded, err := json.Marshal(diagnostics.records[0])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(encoded, &fields) != nil ||
		string(fields["provider_account_id"]) != `""` ||
		string(fields["credential_revision"]) != "0" ||
		string(fields["governance_policy_digest"]) != `""` ||
		strings.Count(string(encoded), `"provider_account_id"`) != 1 ||
		strings.Count(string(encoded), `"credential_revision"`) != 1 ||
		strings.Count(string(encoded), `"governance_policy_digest"`) != 1 {
		t.Fatalf("native diagnostic omitted exact absent authority: %s", encoded)
	}
}

func TestProductHarnessGatewayConversationUsesPersistentCodexSegmentBackend(t *testing.T) {
	direct := &productHarnessConversationResponderFixture{}
	events := &productHarnessGatewayEventFixture{}
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "codex-private")
	if err := prepareProductHarnessGatewayWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	if err := prepareProductHarnessGatewayWorkspace(privateRoot); err != nil {
		t.Fatal(err)
	}
	runtime := &productCodexSegmentRuntimeFixture{}
	var opened []harnessadapter.CodexSegmentSessionConfig
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: direct, WorkspacePath: workspace,
			Events: events, Now: func() time.Time { return time.Unix(1, 0).UTC() },
			CodexSegment: &productCodexSegmentBackendConfig{
				ExecutablePath: "/opt/loom/bin/codex", HomePath: filepath.Join(root, "home"),
				PrivateRoot: privateRoot, Sessions: productHarnessSessionRunnerFixture{},
				Open: func(
					_ context.Context,
					config harnessadapter.CodexSegmentSessionConfig,
				) (productCodexSegmentRuntime, error) {
					opened = append(opened, config)
					return runtime, nil
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close(context.Background())
	for _, harnessID := range harnessgateway.BuiltInHarnessIDs() {
		want := harnessgateway.BackendID("backend.segment." + string(harnessID))
		if harnessID == harnessgateway.HarnessCodex {
			want = productCodexSegmentBackendID
		}
		if responder.backendIDs[harnessID] != want ||
			strings.Contains(string(responder.backendIDs[harnessID]), "compatibility") {
			t.Fatalf("Harness %s backend = %q, want %q", harnessID, responder.backendIDs[harnessID], want)
		}
	}

	binding := &api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "codex", ProviderID: "openai",
		ModelID: "gpt-5.6-sol",
	}
	base := api.LocalProductConversationRequest{
		ThreadID: "conversation-codex", SegmentID: "segment-1",
		ProfileID: "conversation-openai-codex-default-v1", ModelID: binding.ModelID,
		ReasoningEffort:             "high",
		ContextPrompt:               "capsule prompt",
		ContextCapsuleDigest:        productHarnessGatewayDigest("capsule:segment-1"),
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("capsule:segment-1"),
		ExecutionBinding:            binding,
		SegmentBindingDigest:        productHarnessGatewayDigest("binding:segment-1"),
	}
	for index, content := range []string{"first", "second"} {
		request := base
		request.AttemptID = "attempt-" + string(rune('1'+index))
		request.IncidentID = "incident-" + request.AttemptID
		request.BindingDigest = productHarnessGatewayDigest(request.AttemptID)
		request.Messages = []api.LocalProductChatMessage{{
			MessageID: "message-" + string(rune('1'+index)), SegmentID: "segment-1",
			Role: "user", Content: content,
		}}
		response, respondErr := responder.Respond(context.Background(), request)
		if respondErr != nil || response.Content != "persistent-reply-"+string(rune('1'+index)) {
			t.Fatalf("response=%#v error=%v", response, respondErr)
		}
	}
	if len(opened) != 1 {
		t.Fatalf("Codex Segment opens = %d, want 1", len(opened))
	}
	if opened[0].WorkspacePath != workspace || opened[0].ModelID != binding.ModelID ||
		opened[0].ReasoningEffort != "high" || opened[0].PrivateRoot == privateRoot ||
		!strings.HasPrefix(opened[0].PrivateRoot, privateRoot+string(filepath.Separator)) {
		t.Fatalf("Codex Segment config = %#v", opened[0])
	}
	prompts := runtime.promptsSnapshot()
	if len(prompts) != 2 || !strings.Contains(prompts[0], `"loom_context":"capsule prompt"`) ||
		!strings.Contains(prompts[0], `"content":"first"`) || prompts[1] != "second" {
		t.Fatalf("prompts = %#v", prompts)
	}
	if direct.calls() != 0 || events.count(harnessgateway.EventSessionOpening) != 1 {
		t.Fatalf("direct calls=%d events=%#v", direct.calls(), events.snapshot())
	}
	for _, event := range events.snapshot() {
		if event.ProviderAccountID != "" || event.CredentialRevision != 0 ||
			event.GovernancePolicyDigest != "" {
			t.Fatalf("native authority was synthesized: %#v", event)
		}
	}
}

func TestProductCodexSegmentBackendCarriesGovernedControlProposalAcrossRuntimeBoundary(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "codex-private")
	for _, directory := range []string{workspace, privateRoot} {
		if err := prepareProductHarnessGatewayWorkspace(directory); err != nil {
			t.Fatal(err)
		}
	}
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	route := &controltool.FrozenRouteReference{
		HarnessAdapter: "codex", ProviderID: "openai", ModelID: "gpt-5.6-sol",
		ReasoningEffort:        "high",
		ExecutionBindingDigest: productHarnessGatewayDigest("binding-segment"),
		ContextCapsuleDigest:   productHarnessGatewayDigest("capsule-attempt"),
	}
	workspaceDigest := productHarnessGatewayDigest(workspace)
	workspaceReference := &controltool.FrozenWorkspaceReference{
		WorkspaceID:     "workspace-" + workspaceDigest[:32],
		WorkspaceDigest: workspaceDigest,
	}
	proposal, err := controltool.NewSessionAlignmentProposal(
		controltool.SessionAlignmentProposalInput{
			ProposalID: "proposal-runtime-1", TargetConversationID: "conversation-target",
			TargetContentDigest: productHarnessGatewayDigest("target-content"),
			Sources: []controltool.SessionSource{{
				ConversationID: "conversation-source", Title: "Source session",
				ContentDigest: productHarnessGatewayDigest("source-content"), MessageCount: 2,
			}},
			ContextMode:   controltool.ContextModeSummaryOnly,
			CatalogDigest: productHarnessGatewayDigest("catalog"),
			Route:         route, Workspace: workspaceReference,
			RegistryDigest: registry.Digest(), IncidentID: "incident-control-runtime-1",
			SegmentID: "segment-1", AttemptID: "attempt-1",
			CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	actionProposal, err := controltool.NewConversationActionProposal(
		controltool.ConversationActionProposalInput{
			ProposalID:           "proposal-runtime-mission-1",
			ToolID:               controltool.ToolMissionsCreatePreview,
			TargetConversationID: "conversation-target",
			TargetContentDigest:  productHarnessGatewayDigest("target-content"),
			Argument:             "Ship the governed release", SegmentID: "segment-1",
			Route: route, Workspace: workspaceReference,
			RegistryDigest: registry.Digest(), IncidentID: "incident-control-runtime-1",
			AttemptID: "attempt-1", CreatedAt: createdAt,
			ExpiresAt: createdAt.Add(5 * time.Minute),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &productCodexSegmentRuntimeFixture{
		controlProposals: []controltool.SessionAlignmentProposal{proposal},
		actionProposals:  []controltool.ConversationActionProposal{actionProposal},
		completedCalls: []controltool.CompletedCall{
			{ToolID: controltool.ToolSessionsAlignPreview, ToolVersion: 2, Effect: controltool.EffectProposal},
			{ToolID: controltool.ToolMissionsCreatePreview, ToolVersion: 2, Effect: controltool.EffectProposal},
		},
	}
	controlGateway := controltool.NewGatewaySlot()
	var opened harnessadapter.CodexSegmentSessionConfig
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: &productHarnessConversationResponderFixture{}, WorkspacePath: workspace,
			Events: &productHarnessGatewayEventFixture{},
			Now:    func() time.Time { return createdAt },
			CodexSegment: &productCodexSegmentBackendConfig{
				ExecutablePath: "/opt/loom/bin/codex", HomePath: filepath.Join(root, "home"),
				PrivateRoot: privateRoot, Sessions: productHarnessSessionRunnerFixture{},
				ControlRegistry: registry, ControlGateway: controlGateway,
				Open: func(
					_ context.Context,
					config harnessadapter.CodexSegmentSessionConfig,
				) (productCodexSegmentRuntime, error) {
					opened = config
					return runtime, nil
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close(context.Background())

	request := api.LocalProductConversationRequest{
		ThreadID: "conversation-target", SegmentID: "segment-1", AttemptID: "attempt-1",
		IncidentID: "incident-control-runtime-1", ProfileID: "codex",
		ModelID: "gpt-5.6-sol", ReasoningEffort: "high",
		ContextPrompt:               "capsule prompt",
		ContextCapsuleDigest:        productHarnessGatewayDigest("capsule-attempt"),
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("capsule-segment"),
		SegmentBindingDigest:        productHarnessGatewayDigest("binding-segment"),
		BindingDigest:               productHarnessGatewayDigest("binding-attempt"),
		ExecutionBinding: &api.LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "codex", ProviderID: "openai",
			ModelID: "gpt-5.6-sol",
		},
		SessionCatalog: []controltool.SessionReference{
			{ConversationID: "conversation-source", Title: "Source session", UpdatedAt: createdAt},
			{ConversationID: "conversation-target", Title: "Target session", UpdatedAt: createdAt},
		},
		CatalogDigest: productHarnessGatewayDigest("catalog"),
		Messages: []api.LocalProductChatMessage{{
			MessageID: "message-1", SegmentID: "segment-1", Role: "user",
			Content: "Align the source session here",
		}},
	}
	response, err := responder.Respond(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "persistent-reply-1" || len(response.ControlProposals) != 1 ||
		len(response.ActionProposals) != 1 ||
		len(response.CompletedControlTools) != 2 ||
		response.ControlProposals[0].ProposalDigest != proposal.ProposalDigest ||
		response.ActionProposals[0].ProposalDigest != actionProposal.ProposalDigest {
		t.Fatalf("response = %#v", response)
	}
	assertProductAlignmentProposalFrozenTurn(
		t, response.ControlProposals[0], request, registry,
	)
	assertProductActionProposalFrozenTurn(
		t, response.ActionProposals[0], request, registry,
	)
	if opened.ControlRegistry != registry || opened.ControlGateway != controlGateway {
		t.Fatalf("control configuration was not preserved: %#v", opened)
	}
	if !strings.Contains(opened.SystemPrompt, "loom_sessions_align_preview") ||
		!strings.Contains(opened.SystemPrompt, "loom_missions_create_preview") ||
		!strings.Contains(opened.SystemPrompt, "harness=codex") {
		t.Fatalf("Codex system prompt did not freeze the control Registry: %q", opened.SystemPrompt)
	}
	turns, endCalls := runtime.controlSnapshot()
	if len(turns) != 1 || endCalls != 1 ||
		turns[0].ConversationID != request.ThreadID || turns[0].SegmentID != request.SegmentID ||
		turns[0].AttemptID != request.AttemptID || turns[0].IncidentID != request.IncidentID ||
		turns[0].CatalogDigest != request.CatalogDigest || len(turns[0].Catalog) != 2 {
		t.Fatalf("control turns = %#v, end calls = %d", turns, endCalls)
	}
}

func assertProductActionProposalFrozenTurn(
	t *testing.T,
	proposal controltool.ConversationActionProposal,
	request api.LocalProductConversationRequest,
	registry *controltool.Registry,
) {
	t.Helper()
	if proposal.SchemaVersion != 2 || proposal.ToolVersion != 2 ||
		proposal.Route == nil || proposal.Workspace == nil ||
		proposal.RegistryDigest != registry.Digest() ||
		proposal.IncidentID != request.IncidentID ||
		!productControlRouteMatchesRequest(*proposal.Route, request) ||
		!proposal.Workspace.Valid() {
		t.Fatalf("action proposal did not freeze the exact turn: %#v", proposal)
	}
}

func assertProductAlignmentProposalFrozenTurn(
	t *testing.T,
	proposal controltool.SessionAlignmentProposal,
	request api.LocalProductConversationRequest,
	registry *controltool.Registry,
) {
	t.Helper()
	if proposal.SchemaVersion != 2 || proposal.ToolVersion != 2 ||
		proposal.Route == nil || proposal.Workspace == nil ||
		proposal.RegistryDigest != registry.Digest() ||
		proposal.IncidentID != request.IncidentID ||
		!productControlRouteMatchesRequest(*proposal.Route, request) ||
		!proposal.Workspace.Valid() {
		t.Fatalf("alignment proposal did not freeze the exact turn: %#v", proposal)
	}
}

func productControlRouteMatchesRequest(
	route controltool.FrozenRouteReference,
	request api.LocalProductConversationRequest,
) bool {
	if request.ExecutionBinding == nil {
		return false
	}
	return route.HarnessAdapter == request.ExecutionBinding.HarnessAdapter &&
		route.ProviderID == request.ExecutionBinding.ProviderID &&
		route.ProviderAccountID == request.ExecutionBinding.ProviderAccountID &&
		route.CredentialRevision == request.ExecutionBinding.CredentialRevision &&
		route.ModelID == request.ExecutionBinding.ModelID &&
		route.ReasoningEffort == request.ReasoningEffort &&
		route.ExecutionBindingDigest == request.SegmentBindingDigest &&
		route.ContextCapsuleDigest == request.ContextCapsuleDigest
}

func TestProductCodexSegmentFailurePreservesStructuredRecoveryGuidance(t *testing.T) {
	err := productCodexSegmentFailure(errors.New("runtime stopped"))
	failure, ok := api.LocalProductConversationDispatchFailureDetails(err)
	if !ok || failure.Code != "conversation_unavailable" ||
		failure.Stage != "conversation_dispatch" || !failure.Retryable ||
		failure.UserMessage != "The Codex conversation runtime stopped. Retry to reopen this Segment session." {
		t.Fatalf("failure = %#v, ok=%t, error=%v", failure, ok, err)
	}
	if !errors.Is(err, api.ErrLocalProductChatUnavailable) {
		t.Fatalf("error = %v, want local chat unavailable", err)
	}
}

func TestProductCodexSegmentFailurePublishesPrivacySafeArbitrationSubcode(t *testing.T) {
	err := productCodexSegmentFailure(productCodexControlFailureFixture{
		stage: "selection_decode",
	})
	failure, ok := api.LocalProductConversationDispatchFailureDetails(err)
	if !ok || failure.Code != "conversation_unavailable" ||
		failure.Stage != "conversation_dispatch" ||
		failure.ProviderCode != "codex_control_selection_decode" ||
		failure.UserMessage != "Codex could not complete the Loom tool turn. Retry to reopen this Segment session." {
		t.Fatalf("failure = %#v, ok=%t, error=%v", failure, ok, err)
	}
}

func TestProductCodexSegmentFailurePublishesNativeLoginRecovery(t *testing.T) {
	err := productCodexSegmentFailure(harnessadapter.ErrHarnessProviderAuth)
	failure, ok := api.LocalProductConversationDispatchFailureDetails(err)
	if !ok || failure.Code != "provider_auth" || failure.Stage != "provider_auth" ||
		failure.Retryable || failure.ProviderCode != "codex_auth_required" ||
		failure.UserMessage != "Codex needs you to sign in again. Open Runtime & Providers, connect Codex, then retry this turn." {
		t.Fatalf("failure = %#v, ok=%t, error=%v", failure, ok, err)
	}
}

func TestProductCodexSegmentFailurePublishesClosedProviderRecovery(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		code         string
		stage        string
		providerCode string
	}{
		{
			name: "usage limit", err: harnessadapter.ErrHarnessProviderRateLimit,
			code: "provider_rate_limit", stage: "provider_rate_limit",
			providerCode: "codex_usage_limit",
		},
		{
			name: "temporary service failure", err: harnessadapter.ErrHarnessProviderUnavailable,
			code: "provider_unavailable", stage: "provider_connect",
			providerCode: "codex_server_unavailable",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := productCodexSegmentFailure(test.err)
			failure, ok := api.LocalProductConversationDispatchFailureDetails(err)
			if !ok || failure.Code != test.code || failure.Stage != test.stage ||
				!failure.Retryable || failure.ProviderCode != test.providerCode ||
				failure.UserMessage == "" {
				t.Fatalf("failure = %#v, ok=%t, error=%v", failure, ok, err)
			}
		})
	}
}

type productCodexControlFailureFixture struct {
	stage string
}

func (failure productCodexControlFailureFixture) Error() string {
	return "privacy-safe Codex control failure"
}

func (failure productCodexControlFailureFixture) CodexControlArbitrationFailureStage() string {
	return failure.stage
}

func TestProductCodexSegmentBackendReopensAfterNativeSessionBecomesUnhealthy(t *testing.T) {
	direct := &productHarnessConversationResponderFixture{}
	events := &productHarnessGatewayEventFixture{}
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "codex-private")
	for _, directory := range []string{workspace, privateRoot} {
		if err := prepareProductHarnessGatewayWorkspace(directory); err != nil {
			t.Fatal(err)
		}
	}
	runtimes := []*productCodexSegmentRuntimeFixture{
		{respondErr: errors.New("native protocol lost"), unhealthy: true},
		{},
	}
	opened := 0
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: direct, WorkspacePath: workspace,
			Events: events, Now: func() time.Time { return time.Unix(1, 0).UTC() },
			CodexSegment: &productCodexSegmentBackendConfig{
				ExecutablePath: "/opt/loom/bin/codex", HomePath: filepath.Join(root, "home"),
				PrivateRoot: privateRoot, Sessions: productHarnessSessionRunnerFixture{},
				Open: func(
					_ context.Context,
					_ harnessadapter.CodexSegmentSessionConfig,
				) (productCodexSegmentRuntime, error) {
					if opened >= len(runtimes) {
						return nil, errors.New("unexpected Session open")
					}
					runtime := runtimes[opened]
					opened++
					return runtime, nil
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close(context.Background())
	binding := &api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "codex", ProviderID: "openai",
		ProviderAccountID: "openai.native", CredentialRevision: 1,
		ModelID:                     "gpt-5.6-sol",
		ProviderAccountPolicyDigest: productHarnessGatewayDigest("policy:codex"),
	}
	request := api.LocalProductConversationRequest{
		ThreadID: "conversation-codex-unhealthy", SegmentID: "segment-1",
		ProfileID: "conversation-openai-codex-default-v1", ModelID: binding.ModelID,
		ContextPrompt: "capsule prompt", ExecutionBinding: binding,
		ContextCapsuleDigest:        productHarnessGatewayDigest("capsule:segment-1"),
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("capsule:segment-1"),
		SegmentBindingDigest:        productHarnessGatewayDigest("binding:segment-1"),
		Messages: []api.LocalProductChatMessage{{
			MessageID: "message-1", SegmentID: "segment-1", Role: "user", Content: "first",
		}},
	}
	request.AttemptID, request.IncidentID = "attempt-1", "incident-attempt-1"
	request.BindingDigest = productHarnessGatewayDigest(request.AttemptID)
	if _, err := responder.Respond(context.Background(), request); !errors.Is(err, harnessgateway.ErrSessionUnhealthy) {
		t.Fatalf("first response error = %v", err)
	}
	request.AttemptID, request.IncidentID = "attempt-2", "incident-attempt-2"
	request.BindingDigest = productHarnessGatewayDigest(request.AttemptID)
	request.Messages[0].MessageID = "message-2"
	request.Messages[0].Content = "second"
	response, err := responder.Respond(context.Background(), request)
	if err != nil || response.Content != "persistent-reply-1" {
		t.Fatalf("reopened response=%#v error=%v", response, err)
	}
	if opened != 2 || events.count(harnessgateway.EventSessionFailed) != 1 ||
		direct.calls() != 0 {
		t.Fatalf("opened=%d events=%#v direct=%d", opened, events.snapshot(), direct.calls())
	}
}

func TestProductHarnessGatewayConversationReusesExactSegmentSession(t *testing.T) {
	direct := &productHarnessConversationResponderFixture{}
	events := &productHarnessGatewayEventFixture{}
	workspace := t.TempDir()
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: direct, WorkspacePath: workspace,
			Events: events, Now: func() time.Time { return time.Unix(1, 0).UTC() },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close(context.Background())

	binding := &api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "codex", ProviderID: "openai",
		ProviderAccountID: "openai.native", CredentialRevision: 1,
		ModelID:                     "gpt-5.6-sol",
		ProviderAccountPolicyDigest: productHarnessGatewayDigest("policy:codex"),
	}
	base := api.LocalProductConversationRequest{
		ThreadID: "conversation-codex", SegmentID: "segment-1",
		ProfileID: "conversation-openai-codex-default-v1", ModelID: binding.ModelID,
		ContextCapsuleDigest:        productHarnessGatewayDigest("capsule:segment-1"),
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("capsule:segment-1"),
		ExecutionBinding:            binding,
		SegmentBindingDigest:        productHarnessGatewayDigest("binding:segment-1"),
		BindingDigest:               productHarnessGatewayDigest("attempt:1"),
		Messages: []api.LocalProductChatMessage{{
			MessageID: "message-1", SegmentID: "segment-1", Role: "user", Content: "first",
		}},
	}
	for index := range 2 {
		request := base
		request.AttemptID = "attempt-" + string(rune('1'+index))
		request.IncidentID = "incident-" + request.AttemptID
		request.BindingDigest = productHarnessGatewayDigest(request.AttemptID)
		request.Messages = []api.LocalProductChatMessage{{
			MessageID: "message-" + string(rune('1'+index)), SegmentID: "segment-1",
			Role: "user", Content: "turn-" + string(rune('1'+index)),
		}}
		response, respondErr := responder.Respond(context.Background(), request)
		if respondErr != nil || response.Content != "reply:"+request.AttemptID {
			t.Fatalf("response=%#v error=%v", response, respondErr)
		}
	}
	if direct.calls() != 2 {
		t.Fatalf("direct calls = %d", direct.calls())
	}
	if got := events.count(harnessgateway.EventSessionOpening); got != 1 {
		t.Fatalf("session openings = %d, want 1", got)
	}
	if got := events.count(harnessgateway.EventResponseCompleted); got != 2 {
		t.Fatalf("completed responses = %d, want 2", got)
	}
}

func TestProductHarnessGatewayConversationRegistersExactlyFiveBuiltIns(t *testing.T) {
	direct := &productHarnessConversationResponderFixture{}
	events := &productHarnessGatewayEventFixture{}
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: direct, WorkspacePath: t.TempDir(), Events: events,
			Now: func() time.Time { return time.Unix(2, 0).UTC() },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close(context.Background())
	for _, harnessID := range harnessgateway.BuiltInHarnessIDs() {
		want := harnessgateway.BackendID("backend.segment." + string(harnessID))
		if responder.backendIDs[harnessID] != want ||
			strings.Contains(string(responder.backendIDs[harnessID]), "compatibility") {
			t.Fatalf("Harness %s backend = %q, want %q", harnessID, responder.backendIDs[harnessID], want)
		}
	}

	for index, harnessID := range harnessgateway.BuiltInHarnessIDs() {
		providerID := "provider-" + string(harnessID)
		request := api.LocalProductConversationRequest{
			ThreadID:   "conversation-" + string(harnessID),
			AttemptID:  "attempt-" + string(harnessID),
			IncidentID: "incident-" + string(harnessID),
			ProfileID:  "profile-" + string(harnessID), SegmentID: "segment-1",
			ModelID:                     "model-" + string(harnessID),
			ContextCapsuleDigest:        productHarnessGatewayDigest("capsule:" + string(harnessID)),
			SegmentContextCapsuleDigest: productHarnessGatewayDigest("capsule:" + string(harnessID)),
			SegmentBindingDigest:        productHarnessGatewayDigest("binding:" + string(harnessID)),
			BindingDigest:               productHarnessGatewayDigest("attempt:" + string(harnessID)),
			ExecutionBinding: &api.LocalProductConversationExecutionBinding{
				SchemaVersion: 4, HarnessAdapter: string(harnessID), ProviderID: providerID,
				ProviderAccountID: providerID + ".primary", CredentialRevision: int64(index + 1),
				ModelID:                     "model-" + string(harnessID),
				ProviderAccountPolicyDigest: productHarnessGatewayDigest("policy:" + string(harnessID)),
			},
			Messages: []api.LocalProductChatMessage{{
				MessageID: "message-1", SegmentID: "segment-1", Role: "user", Content: "hello",
			}},
		}
		if _, err := responder.Respond(context.Background(), request); err != nil {
			t.Fatalf("Harness %s: %v", harnessID, err)
		}
	}
	if direct.calls() != len(harnessgateway.BuiltInHarnessIDs()) ||
		events.count(harnessgateway.EventSessionOpening) != len(harnessgateway.BuiltInHarnessIDs()) {
		t.Fatalf("calls=%d events=%#v", direct.calls(), events.snapshot())
	}
	requests := direct.requestsSnapshot()
	eventSnapshot := events.snapshot()
	for index, harnessID := range harnessgateway.BuiltInHarnessIDs() {
		request := requests[index]
		providerID := "provider-" + string(harnessID)
		if request.ExecutionBinding == nil ||
			request.ExecutionBinding.HarnessAdapter != string(harnessID) ||
			request.ExecutionBinding.ProviderID != providerID ||
			request.ExecutionBinding.ProviderAccountID != providerID+".primary" ||
			request.ExecutionBinding.CredentialRevision != int64(index+1) ||
			request.ExecutionBinding.ModelID != "model-"+string(harnessID) ||
			request.ContextCapsuleDigest != productHarnessGatewayDigest("capsule:"+string(harnessID)) {
			t.Fatalf("Harness %s frozen request = %#v", harnessID, request)
		}
		foundOpening := false
		for _, event := range eventSnapshot {
			if event.Type == harnessgateway.EventSessionOpening && event.HarnessID == harnessID {
				foundOpening = event.BackendID == responder.backendIDs[harnessID] &&
					event.ConfiguredHarnessVersion == productHarnessGatewayConfiguredVersion &&
					event.BackendVersion == productHarnessGatewayBackendVersion && event.Sequence > 0
				break
			}
		}
		if !foundOpening {
			t.Fatalf("Harness %s opening event missing from %#v", harnessID, eventSnapshot)
		}
	}
}

func TestProductHarnessGatewayConversationRejectsUnknownHarnessWithoutExecutorCall(
	t *testing.T,
) {
	direct := &productHarnessConversationResponderFixture{}
	events := &productHarnessGatewayEventFixture{}
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: direct, WorkspacePath: t.TempDir(), Events: events,
			Now: func() time.Time { return time.Unix(2, 0).UTC() },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close(context.Background())

	request := productHarnessGatewayConversationRequestFixture(
		"conversation-unknown", "segment-1", "attempt-1", "incident-1",
	)
	request.ExecutionBinding.HarnessAdapter = "unknown-harness"
	if _, err := responder.Respond(context.Background(), request); err == nil {
		t.Fatal("unknown Harness unexpectedly reached the Gateway executor")
	}
	if direct.calls() != 0 || len(events.snapshot()) != 0 {
		t.Fatalf("executor calls=%d events=%#v", direct.calls(), events.snapshot())
	}
}

func TestProductHarnessGatewayConversationCancelsOnlyExactActiveResponse(t *testing.T) {
	direct := newProductHarnessBlockingConversationResponderFixture()
	events := &productHarnessGatewayEventFixture{}
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: direct, WorkspacePath: t.TempDir(),
			Events: events,
			Now:    func() time.Time { return time.Unix(3, 0).UTC() },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close(context.Background())
	request := productHarnessGatewayConversationRequestFixture(
		"conversation-cancel", "segment-1", "attempt-1", "incident-1",
	)
	result := make(chan error, 1)
	go func() {
		_, respondErr := responder.Respond(context.Background(), request)
		result <- respondErr
	}()
	<-direct.started
	if err := responder.CancelChatResponse(
		context.Background(),
		api.LocalProductChatResponseCancelRequest{
			ThreadID: request.ThreadID, IncidentID: "incident-wrong",
		},
	); !errors.Is(err, harnessgateway.ErrResponseNotFound) {
		t.Fatalf("wrong Incident cancellation error = %v", err)
	}
	select {
	case err := <-result:
		t.Fatalf("wrong Incident cancelled response: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if err := responder.CancelChatResponse(
		context.Background(),
		api.LocalProductChatResponseCancelRequest{
			ThreadID: request.ThreadID, IncidentID: request.IncidentID,
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("response error = %v, want cancelled", err)
	}
	if events.count(harnessgateway.EventResponseCancelled) != 1 ||
		events.count(harnessgateway.EventResponseCompleted) != 0 {
		t.Fatalf("cancellation events = %#v", events.snapshot())
	}
	if err := responder.CancelChatResponse(
		context.Background(),
		api.LocalProductChatResponseCancelRequest{
			ThreadID: request.ThreadID, IncidentID: request.IncidentID,
		},
	); !errors.Is(err, harnessgateway.ErrResponseNotFound) {
		t.Fatalf("terminal response cancellation error = %v, want not found", err)
	}
}

func TestProductHarnessGatewayConversationCancelsDuringSessionOpening(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "codex-private")
	for _, directory := range []string{workspace, privateRoot} {
		if err := prepareProductHarnessGatewayWorkspace(directory); err != nil {
			t.Fatal(err)
		}
	}
	openStarted := make(chan struct{})
	openRelease := make(chan struct{})
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor:      &productHarnessConversationResponderFixture{},
			WorkspacePath: workspace, Events: &productHarnessGatewayEventFixture{},
			Now: func() time.Time { return time.Unix(4, 0).UTC() },
			CodexSegment: &productCodexSegmentBackendConfig{
				ExecutablePath: "/opt/loom/bin/codex", HomePath: filepath.Join(root, "home"),
				PrivateRoot: privateRoot, Sessions: productHarnessSessionRunnerFixture{},
				Open: func(ctx context.Context, _ harnessadapter.CodexSegmentSessionConfig) (productCodexSegmentRuntime, error) {
					close(openStarted)
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-openRelease:
						return &productCodexSegmentRuntimeFixture{}, nil
					}
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		select {
		case <-openRelease:
		default:
			close(openRelease)
		}
		if closeErr := responder.Close(context.Background()); closeErr != nil {
			t.Errorf("close Gateway responder: %v", closeErr)
		}
	})
	binding := &api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "codex", ProviderID: "openai", ModelID: "gpt-5.6-sol",
	}
	request := api.LocalProductConversationRequest{
		ThreadID: "conversation-opening-cancel", SegmentID: "segment-1",
		AttemptID: "attempt-opening-cancel", IncidentID: "incident-opening-cancel",
		ProfileID: "conversation-openai-codex-default-v1", ModelID: binding.ModelID,
		ContextCapsuleDigest:        productHarnessGatewayDigest("attempt-capsule-opening-cancel"),
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("segment-capsule-opening-cancel"),
		ExecutionBinding:            binding, SegmentBindingDigest: productHarnessGatewayDigest("binding-opening-cancel"),
		BindingDigest: productHarnessGatewayDigest("attempt-opening-cancel"),
		Messages: []api.LocalProductChatMessage{{
			MessageID: "message-opening-cancel", SegmentID: "segment-1", Role: "user", Content: "stop immediately",
		}},
	}
	done := make(chan error, 1)
	go func() {
		_, respondErr := responder.Respond(context.Background(), request)
		done <- respondErr
	}()
	<-openStarted
	if err := responder.CancelChatResponse(context.Background(), api.LocalProductChatResponseCancelRequest{
		ThreadID: request.ThreadID, IncidentID: request.IncidentID,
	}); err != nil {
		t.Fatalf("cancel opening response: %v", err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("opening response error = %v, want cancelled", err)
	}
	if err := responder.CancelChatResponse(context.Background(), api.LocalProductChatResponseCancelRequest{
		ThreadID: request.ThreadID, IncidentID: request.IncidentID,
	}); !errors.Is(err, harnessgateway.ErrResponseNotFound) {
		t.Fatalf("terminal opening response cancellation error = %v", err)
	}
	close(openRelease)
}

type productHarnessConversationResponderFixture struct {
	mu       sync.Mutex
	requests []api.LocalProductConversationRequest
}

type productHarnessForgedControlResponderFixture struct{}

func (*productHarnessForgedControlResponderFixture) Respond(
	context.Context,
	api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error) {
	return api.LocalProductConversationResponse{
		Content: "I used a Loom tool.", Tentative: true,
		CompletedControlTools: []controltool.CompletedCall{{
			ToolID: controltool.ToolRuntimesStatus, ToolVersion: 1,
			Effect: controltool.EffectRead,
		}},
	}, nil
}

func productHarnessGatewayConversationRequestFixture(
	threadID string,
	segmentID string,
	attemptID string,
	incidentID string,
) api.LocalProductConversationRequest {
	binding := &api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "loom-native", ProviderID: "deepseek",
		ProviderAccountID: "deepseek.primary", CredentialRevision: 1,
		ModelID:                     "deepseek-chat",
		ProviderAccountPolicyDigest: productHarnessGatewayDigest("policy:" + threadID),
	}
	return api.LocalProductConversationRequest{
		ThreadID: threadID, SegmentID: segmentID, AttemptID: attemptID,
		IncidentID: incidentID, ProfileID: "profile-deepseek", ModelID: binding.ModelID,
		ContextCapsuleDigest:        productHarnessGatewayDigest("capsule:" + segmentID),
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("capsule:" + segmentID),
		SegmentBindingDigest:        productHarnessGatewayDigest("binding:" + segmentID),
		BindingDigest:               productHarnessGatewayDigest("attempt:" + attemptID),
		ExecutionBinding:            binding,
		Messages: []api.LocalProductChatMessage{{
			MessageID: "message-1", SegmentID: segmentID, Role: "user", Content: "hello",
		}},
	}
}

type productHarnessBlockingConversationResponderFixture struct {
	started chan struct{}
	once    sync.Once
}

func newProductHarnessBlockingConversationResponderFixture() *productHarnessBlockingConversationResponderFixture {
	return &productHarnessBlockingConversationResponderFixture{started: make(chan struct{})}
}

func (fixture *productHarnessBlockingConversationResponderFixture) Respond(
	ctx context.Context,
	_ api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error) {
	fixture.once.Do(func() { close(fixture.started) })
	<-ctx.Done()
	return api.LocalProductConversationResponse{}, ctx.Err()
}

func (fixture *productHarnessConversationResponderFixture) Respond(
	_ context.Context,
	request api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error) {
	fixture.mu.Lock()
	fixture.requests = append(fixture.requests, request)
	fixture.mu.Unlock()
	return api.LocalProductConversationResponse{
		Content: "reply:" + request.AttemptID, Tentative: true,
	}, nil
}

func (fixture *productHarnessConversationResponderFixture) calls() int {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return len(fixture.requests)
}

func (fixture *productHarnessConversationResponderFixture) requestsSnapshot() []api.LocalProductConversationRequest {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return append([]api.LocalProductConversationRequest(nil), fixture.requests...)
}

type productHarnessGatewayEventFixture struct {
	mu     sync.Mutex
	events []harnessgateway.Event
}

func (fixture *productHarnessGatewayEventFixture) Record(event harnessgateway.Event) {
	fixture.mu.Lock()
	fixture.events = append(fixture.events, event)
	fixture.mu.Unlock()
}

func (fixture *productHarnessGatewayEventFixture) snapshot() []harnessgateway.Event {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return append([]harnessgateway.Event(nil), fixture.events...)
}

func (fixture *productHarnessGatewayEventFixture) count(want harnessgateway.EventType) int {
	count := 0
	for _, event := range fixture.snapshot() {
		if event.Type == want {
			count++
		}
	}
	return count
}

type productCodexSegmentRuntimeFixture struct {
	mu               sync.Mutex
	prompts          []string
	controlTurns     []controltool.TurnContext
	controlProposals []controltool.SessionAlignmentProposal
	actionProposals  []controltool.ConversationActionProposal
	completedCalls   []controltool.CompletedCall
	controlEndCalls  int
	closed           bool
	respondErr       error
	controlBeginErr  error
	controlEndErr    error
	unhealthy        bool
}

func (fixture *productCodexSegmentRuntimeFixture) BeginControlTurn(
	turn controltool.TurnContext,
) error {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	turn.Catalog = append([]controltool.SessionReference(nil), turn.Catalog...)
	fixture.controlTurns = append(fixture.controlTurns, turn)
	return fixture.controlBeginErr
}

func (fixture *productCodexSegmentRuntimeFixture) EndControlTurn() (
	controltool.ProposalBatch,
	error,
) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	fixture.controlEndCalls++
	return controltool.ProposalBatch{
		SessionAlignments: append(
			[]controltool.SessionAlignmentProposal(nil), fixture.controlProposals...,
		),
		ConversationActions: append(
			[]controltool.ConversationActionProposal(nil), fixture.actionProposals...,
		),
		CompletedCalls: controltool.CloneCompletedCalls(fixture.completedCalls),
	}, fixture.controlEndErr
}

func (fixture *productCodexSegmentRuntimeFixture) Respond(
	_ context.Context,
	prompt []byte,
) (harnessadapter.HarnessProcessResult, error) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	fixture.prompts = append(fixture.prompts, string(prompt))
	if fixture.respondErr != nil {
		return harnessadapter.HarnessProcessResult{}, fixture.respondErr
	}
	return harnessadapter.HarnessProcessResult{
		Content: "persistent-reply-" + string(rune('0'+len(fixture.prompts))),
	}, nil
}

func (fixture *productCodexSegmentRuntimeFixture) Healthy() bool {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return !fixture.closed && !fixture.unhealthy
}

func (fixture *productCodexSegmentRuntimeFixture) Close(context.Context) error {
	fixture.mu.Lock()
	fixture.closed = true
	fixture.mu.Unlock()
	return nil
}

func (fixture *productCodexSegmentRuntimeFixture) promptsSnapshot() []string {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return append([]string(nil), fixture.prompts...)
}

func (fixture *productCodexSegmentRuntimeFixture) controlSnapshot() (
	[]controltool.TurnContext,
	int,
) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	turns := make([]controltool.TurnContext, len(fixture.controlTurns))
	copy(turns, fixture.controlTurns)
	for index := range turns {
		turns[index].Catalog = append(
			[]controltool.SessionReference(nil), turns[index].Catalog...,
		)
	}
	return turns, fixture.controlEndCalls
}

type productHarnessSessionRunnerFixture struct{}

func (productHarnessSessionRunnerFixture) StartSession(
	context.Context,
	harnessadapter.HarnessSessionRequest,
) (harnessadapter.HarnessStreamSession, error) {
	return nil, nil
}
