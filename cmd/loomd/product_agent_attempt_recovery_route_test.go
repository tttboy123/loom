package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
)

func TestProductAgentAttemptRecoveryRouteRequiresPreviewConfirmThenResume(t *testing.T) {
	ctx := context.Background()
	candidate := work.AgentAttemptRecoveryCandidate{
		SchemaVersion: 1, Status: work.AgentAttemptRecoveryCandidateAvailable,
		Action:           work.AgentAttemptRecoveryResumePreModel,
		CandidateDigest:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CapabilityDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		AttemptID:        "attempt-recovery", TeamInstanceID: "team-recovery",
		SegmentID: "segment-recovery", WorkItemID: "work-recovery",
		RunID: "run-recovery", ClaimGeneration: 3,
		RuntimeInstanceID: "runtime-recovery", AgentInstanceID: "agent-recovery",
		HarnessAdapter: "loom-native", ProviderID: "deepseek",
		ProviderAccountID: "deepseek.primary", ModelID: "deepseek-chat",
		CredentialRevision: 4,
	}
	authority := &productAttemptRecoveryRouteAuthorityFixture{
		preview: work.AgentAttemptRecoveryPreview{
			SchemaVersion: 1, Candidates: []work.AgentAttemptRecoveryCandidate{candidate},
		},
		decision: work.AgentAttemptRecoveryDecision{
			SchemaVersion: 1, DecisionID: "11111111-1111-4111-8111-111111111111",
			Action:           work.AgentAttemptRecoveryResumePreModel,
			CandidateDigest:  candidate.CandidateDigest,
			CapabilityDigest: candidate.CapabilityDigest,
		},
		lease: &work.AgentAttemptRecoveryDispatchLease{},
	}
	completion := &productAttemptRecoveryRouteCompletionFixture{}
	route, err := newProductAgentAttemptRecoveryRoute(authority, completion)
	if err != nil {
		t.Fatal(err)
	}

	preview, err := route.RecoverAgentAttempt(ctx, productAgentAttemptRecoveryRequest{
		SchemaVersion: 1, Operation: "preview",
		IncidentID: "22222222-2222-4222-8222-222222222222",
	})
	if err != nil || preview.Operation != "preview" ||
		preview.IncidentID != "22222222-2222-4222-8222-222222222222" ||
		len(preview.Candidates) != 1 || preview.Candidates[0].ProviderAccountID != "deepseek.primary" ||
		preview.Candidates[0].CredentialRevision != 4 || authority.previewCalls != 1 {
		t.Fatalf("preview = %#v, calls=%d, err=%v", preview, authority.previewCalls, err)
	}

	confirmed, err := route.RecoverAgentAttempt(ctx, productAgentAttemptRecoveryRequest{
		SchemaVersion: 1, Operation: "confirm",
		DecisionID:  "11111111-1111-4111-8111-111111111111",
		PrincipalID: "local-user", CandidateDigest: candidate.CandidateDigest,
		CapabilityDigest: candidate.CapabilityDigest,
		IncidentID:       "33333333-3333-4333-8333-333333333333",
	})
	if err != nil || confirmed.Operation != "confirm" || confirmed.Decision == nil ||
		confirmed.Decision.DecisionID != authority.decision.DecisionID ||
		authority.authorizeCalls != 1 ||
		authority.authorizeInput.CorrelationID != confirmed.IncidentID ||
		authority.authorizeInput.PrincipalID != "local-user" {
		t.Fatalf("confirm = %#v, authority=%#v, err=%v", confirmed, authority, err)
	}

	resumed, err := route.RecoverAgentAttempt(ctx, productAgentAttemptRecoveryRequest{
		SchemaVersion: 1, Operation: "resume",
		DecisionID:       authority.decision.DecisionID,
		CandidateDigest:  candidate.CandidateDigest,
		CapabilityDigest: candidate.CapabilityDigest,
		IncidentID:       "44444444-4444-4444-8444-444444444444",
	})
	if err != nil || resumed.Operation != "resume" || resumed.Resume == nil ||
		resumed.Resume.Status != "completed" ||
		resumed.Resume.DecisionID != authority.decision.DecisionID ||
		authority.consumeCalls != 1 || completion.calls != 1 ||
		authority.consumeInput.CorrelationID != resumed.IncidentID {
		t.Fatalf("resume = %#v, authority=%#v completion=%#v err=%v",
			resumed, authority, completion, err)
	}
}

func TestProductAgentAttemptRecoveryRouteRejectsMixedOperationFields(t *testing.T) {
	route, err := newProductAgentAttemptRecoveryRoute(
		&productAttemptRecoveryRouteAuthorityFixture{},
		&productAttemptRecoveryRouteCompletionFixture{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := route.RecoverAgentAttempt(context.Background(), productAgentAttemptRecoveryRequest{
		SchemaVersion: 1, Operation: "preview", PrincipalID: "local-user",
		IncidentID: "55555555-5555-4555-8555-555555555555",
	}); err != errProductInvalidAttemptRecoveryRequest {
		t.Fatalf("mixed preview fields error = %v", err)
	}
}

func TestProductDaemonRoutesStrictAgentAttemptRecoveryWithRequestIncident(t *testing.T) {
	authority := &productAttemptRecoveryRouteAuthorityFixture{
		preview: work.AgentAttemptRecoveryPreview{SchemaVersion: 1, Candidates: []work.AgentAttemptRecoveryCandidate{}},
	}
	route, err := newProductAgentAttemptRecoveryRoute(
		authority, &productAttemptRecoveryRouteCompletionFixture{},
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := newProductRouteHandler(productRouteServices{agentRecovery: route})
	params, err := json.Marshal(productAgentAttemptRecoveryRequest{
		SchemaVersion: 1, Operation: "preview",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "66666666-6666-4666-8666-666666666666",
		Method: "agent_attempt_recovery", Params: params,
	})
	if !response.OK || response.Error != nil || authority.previewCalls != 1 {
		t.Fatalf("response/authority = %#v / %#v", response, authority)
	}
	var decoded productAgentAttemptRecoveryResponse
	if err := json.Unmarshal(response.Result, &decoded); err != nil ||
		decoded.IncidentID != "66666666-6666-4666-8666-666666666666" ||
		decoded.Operation != "preview" {
		t.Fatalf("decoded response = %#v, %v", decoded, err)
	}

	invalid := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "77777777-7777-4777-8777-777777777777",
		Method: "agent_attempt_recovery",
		Params: append(params[:len(params)-1], []byte(`,"extra":true}`)...),
	})
	if invalid.OK || invalid.Error == nil || invalid.Error.Code != "invalid_request" ||
		invalid.Error.Stage != "agent_attempt_reconcile" {
		t.Fatalf("invalid response = %#v", invalid)
	}
}

func TestProductAgentAttemptRecoveryTraversesAuthenticatedLocalIPC(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "loom-agent-recovery-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	authority := &productAttemptRecoveryRouteAuthorityFixture{
		preview: work.AgentAttemptRecoveryPreview{SchemaVersion: 1, Candidates: []work.AgentAttemptRecoveryCandidate{}},
	}
	route, err := newProductAgentAttemptRecoveryRoute(
		authority, &productAttemptRecoveryRouteCompletionFixture{},
	)
	if err != nil {
		t.Fatal(err)
	}
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: filepath.Join(root, "loomd.sock"), EffectiveUID: os.Geteuid(),
		BuildID: "agent-recovery-ipc-fixture",
		Handler: localipc.HandlerFunc(newProductRouteHandler(productRouteServices{
			agentRecovery: route,
		})),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case err := <-done:
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	defer func() {
		cancel()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("server close: %v", err)
		}
	}()
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: filepath.Join(root, "loomd.sock"), Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var response productAgentAttemptRecoveryResponse
	if err := client.Call(
		context.Background(), "agent_attempt_recovery",
		productAgentAttemptRecoveryRequest{SchemaVersion: 1, Operation: "preview"},
		&response,
	); err != nil {
		t.Fatal(err)
	}
	if response.Operation != "preview" || response.IncidentID == "" ||
		authority.previewCalls != 1 {
		t.Fatalf("response/authority = %#v / %#v", response, authority)
	}
}

type productAttemptRecoveryRouteAuthorityFixture struct {
	preview        work.AgentAttemptRecoveryPreview
	decision       work.AgentAttemptRecoveryDecision
	lease          *work.AgentAttemptRecoveryDispatchLease
	previewCalls   int
	authorizeCalls int
	consumeCalls   int
	authorizeInput work.AgentAttemptRecoveryDecisionInput
	consumeInput   work.AgentAttemptRecoveryConsumeInput
}

func (fixture *productAttemptRecoveryRouteAuthorityFixture) Preview(
	context.Context,
) (work.AgentAttemptRecoveryPreview, error) {
	fixture.previewCalls++
	return fixture.preview, nil
}

func (fixture *productAttemptRecoveryRouteAuthorityFixture) Authorize(
	_ context.Context,
	input work.AgentAttemptRecoveryDecisionInput,
) (work.AgentAttemptRecoveryDecision, error) {
	fixture.authorizeCalls++
	fixture.authorizeInput = input
	return fixture.decision, nil
}

func (fixture *productAttemptRecoveryRouteAuthorityFixture) Consume(
	_ context.Context,
	input work.AgentAttemptRecoveryConsumeInput,
) (*work.AgentAttemptRecoveryDispatchLease, error) {
	fixture.consumeCalls++
	fixture.consumeInput = input
	return fixture.lease, nil
}

type productAttemptRecoveryRouteCompletionFixture struct {
	calls int
}

func (fixture *productAttemptRecoveryRouteCompletionFixture) Resume(
	context.Context,
	*work.AgentAttemptRecoveryDispatchLease,
) (supervisor.AdapterResult, error) {
	fixture.calls++
	return supervisor.AdapterResult{}, nil
}
