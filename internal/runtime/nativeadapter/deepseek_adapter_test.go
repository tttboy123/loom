package nativeadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/permissions"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func TestLoomNativeDispatchAcceptsCanonicalRoleContextPayload(t *testing.T) {
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "conversation-1", TeamID: "team-1",
			AgentID: "agent-deepseek", RoleID: "researcher",
			ProviderID:        DeepSeekAgentProviderID,
			ProviderAccountID: "deepseek.primary", ModelID: DeepSeekAgentModelID,
			AuthMode: "brokered", ContextAdapterID: "context:loom-native:v1",
			DisclosurePolicyID: "policy.test", DisclosurePolicyVersion: 1,
			TokenBudget: 64,
		},
		[]contextcapsule.ItemInput{{
			ItemID: "goal-1", Kind: contextcapsule.KindConversationGoal,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			Priority: contextcapsule.PrioritySystem, TokenCount: 4, Required: true,
			Content:    []byte("Research the admitted Provider behavior."),
			SourceType: contextcapsule.SourceAuthority, SourceRef: "goal:phase-2d",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := decodeDeepSeekAgentDispatch(payload)
	if err != nil || !strings.Contains(dispatch.Prompt, "Research the admitted Provider behavior.") {
		t.Fatalf("context dispatch = %#v, %v", dispatch, err)
	}
}

const deepSeekEndpointFingerprintFixture = "948f1ecb6b48f91adc4e110d0351cd172b16450e9936d358992e0dfad7b863f3"

type credentialAccessFixture struct {
	secret  []byte
	err     error
	uses    int
	binding loomruntime.FrozenExecutionBinding
}

func (access *credentialAccessFixture) UseCredential(
	ctx context.Context,
	binding loomruntime.FrozenExecutionBinding,
	use func(context.Context, []byte) error,
) error {
	access.uses++
	access.binding = binding
	if access.err != nil {
		return access.err
	}
	secret := append([]byte(nil), access.secret...)
	defer func() {
		for index := range secret {
			secret[index] = 0
		}
	}()
	return use(ctx, secret)
}

type httpDoerFixture struct {
	response *http.Response
	err      error
	requests int
	request  *http.Request
	body     []byte
	auth     string
}

type contextRetrievalHTTPDoerFixture struct {
	responses []*http.Response
	bodies    [][]byte
	auth      []string
	beforeDo  func(int) error
}

type agentInputSourceFixture struct {
	batches     []loomruntime.AgentInputBatch
	checkpoints []loomruntime.AgentInputCheckpoint
}

func (source *agentInputSourceFixture) NextAgentInput(
	_ context.Context,
	checkpoint loomruntime.AgentInputCheckpoint,
) (loomruntime.AgentInputBatch, bool, error) {
	source.checkpoints = append(source.checkpoints, checkpoint)
	if len(source.batches) == 0 {
		return loomruntime.AgentInputBatch{}, false, nil
	}
	batch := source.batches[0]
	source.batches = source.batches[1:]
	return batch, true, nil
}

type durableAgentInputSourceFixture struct {
	agentInputSourceFixture
	persisted [][]byte
	events    []string
	err       error
}

func (source *durableAgentInputSourceFixture) NextAgentInputFromDurableCheckpoint(
	ctx context.Context,
	payload loomruntime.AgentInputCheckpointPayload,
) (loomruntime.AgentInputBatch, bool, error) {
	source.events = append(source.events, "persist")
	if source.err != nil {
		return loomruntime.AgentInputBatch{}, false, source.err
	}
	source.persisted = append(source.persisted, bytes.Clone(payload.Content))
	source.events = append(source.events, "next")
	return source.NextAgentInput(ctx, payload.Checkpoint)
}

func (doer *contextRetrievalHTTPDoerFixture) Do(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	doer.bodies = append(doer.bodies, body)
	doer.auth = append(doer.auth, request.Header.Get("Authorization"))
	request.Header.Del("Authorization")
	if doer.beforeDo != nil {
		if err := doer.beforeDo(len(doer.bodies)); err != nil {
			return nil, err
		}
	}
	if len(doer.responses) == 0 {
		return nil, errors.New("unexpected Provider request")
	}
	response := doer.responses[0]
	doer.responses = doer.responses[1:]
	response.Request = request
	return response, nil
}

type contextRetrieverFixture struct {
	want    contextcapsule.RetrievalProposal
	content []byte
	err     error
	calls   int
}

// conflictingContextDeliveryFixture reproduces the production failure where the
// first context-read is a bounded denial (ErrContextItemNotRetrievable) and a
// second context-read in the same step cannot be dispatched because the
// Attempt-loop admits only one tool dispatch per step (ErrInvalidContextDelivery).
type conflictingContextDeliveryFixture struct {
	retriever contextcapsule.Retriever
	prepare   int
}

func (delivery *conflictingContextDeliveryFixture) Prepare(
	ctx context.Context,
	proposal contextcapsule.RetrievalProposal,
	request contextcapsule.DeliveryRequest,
	encode contextcapsule.DeliveryEncoder,
) (attemptpayload.Payload, error) {
	delivery.prepare++
	if delivery.prepare == 1 {
		item, err := delivery.retriever.Retrieve(ctx, proposal)
		item.Close()
		if err != nil {
			return attemptpayload.Payload{}, err
		}
		return attemptpayload.Payload{}, contextcapsule.ErrContextItemNotRetrievable
	}
	return attemptpayload.Payload{}, contextcapsule.ErrInvalidContextDelivery
}

func (delivery *conflictingContextDeliveryFixture) Acknowledge(
	context.Context,
	attemptpayload.Binding,
	attemptpayload.DeliveryProof,
) error {
	return nil
}

type contextDeliveryFixture struct {
	retriever contextcapsule.Retriever
	payload   attemptpayload.Payload
	payloads  map[string]attemptpayload.Payload
	proof     attemptpayload.DeliveryProof
	prepare   int
	acks      int
}

func (delivery *contextDeliveryFixture) Prepare(
	ctx context.Context,
	proposal contextcapsule.RetrievalProposal,
	request contextcapsule.DeliveryRequest,
	encode contextcapsule.DeliveryEncoder,
) (attemptpayload.Payload, error) {
	delivery.prepare++
	callID := contextcapsule.DeliveryCallID(proposal, request.Sequence)
	if delivery.payloads == nil {
		delivery.payloads = make(map[string]attemptpayload.Payload)
	}
	if existing, found := delivery.payloads[callID]; found {
		if existing.Binding.Sequence != request.Sequence ||
			existing.Status != attemptpayload.StatusPending {
			return attemptpayload.Payload{}, contextcapsule.ErrInvalidContextDelivery
		}
		delivery.payload = existing
		payload := existing
		payload.Content = append([]byte(nil), payload.Content...)
		return payload, nil
	}
	item, err := delivery.retriever.Retrieve(ctx, proposal)
	if err != nil {
		item.Close()
		return attemptpayload.Payload{}, err
	}
	content, err := encode(proposal, item)
	item.Close()
	if err != nil {
		return attemptpayload.Payload{}, err
	}
	digest := sha256.Sum256(content)
	delivery.payload = attemptpayload.Payload{
		Binding: attemptpayload.Binding{
			PayloadID: fmt.Sprintf("payload-native-context-%d", request.Sequence),
			CallID:    callID,
			Sequence:  request.Sequence, ContentType: request.ContentType,
			ContentDigest: hex.EncodeToString(digest[:]),
		},
		Status: attemptpayload.StatusPending, Content: append([]byte(nil), content...),
	}
	delivery.payloads[callID] = delivery.payload
	payload := delivery.payload
	payload.Content = append([]byte(nil), payload.Content...)
	return payload, nil
}

func (delivery *contextDeliveryFixture) Acknowledge(
	_ context.Context,
	binding attemptpayload.Binding,
	proof attemptpayload.DeliveryProof,
) error {
	delivery.acks++
	payload, found := delivery.payloads[binding.CallID]
	if !found || payload.Binding != binding ||
		proof != attemptpayload.ProofProviderContinuation {
		return contextcapsule.ErrInvalidContextDelivery
	}
	delivery.proof = proof
	payload.Status = attemptpayload.StatusDelivered
	delivery.payloads[binding.CallID] = payload
	delivery.payload = payload
	return nil
}

func (retriever *contextRetrieverFixture) Retrieve(
	_ context.Context,
	proposal contextcapsule.RetrievalProposal,
) (contextcapsule.RetrievedItem, error) {
	retriever.calls++
	if proposal != retriever.want {
		return contextcapsule.RetrievedItem{}, errors.New("retrieval proposal drifted")
	}
	if retriever.err != nil {
		return contextcapsule.RetrievedItem{}, retriever.err
	}
	return contextcapsule.RetrievedItem{
		ItemID: proposal.ItemID, Kind: contextcapsule.KindArtifactReference,
		Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
		Priority: contextcapsule.PriorityRetrievable, TokenCount: 5,
		Content:       append([]byte(nil), retriever.content...),
		ContentDigest: proposal.ContentDigest,
		SourceType:    contextcapsule.SourceObservation, SourceRef: "evidence:diff-1",
		ArtifactRef: proposal.ArtifactRef,
	}, nil
}

type agentProviderTimeoutFixture struct{}

func (agentProviderTimeoutFixture) Error() string { return "controlled transport timeout" }
func (agentProviderTimeoutFixture) Timeout() bool { return true }

type agentTimeoutBodyFixture struct{}

func (agentTimeoutBodyFixture) Read([]byte) (int, error) {
	return 0, agentProviderTimeoutFixture{}
}
func (agentTimeoutBodyFixture) Close() error { return nil }

func (doer *httpDoerFixture) Do(request *http.Request) (*http.Response, error) {
	doer.requests++
	doer.request = request
	doer.auth = request.Header.Get("Authorization")
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	doer.body = body
	if doer.response != nil {
		doer.response.Request = request
	}
	return doer.response, doer.err
}

type frameSinkFixture struct {
	frames []bridgev1.Frame
}

type agentDiagnosticRecorderFixture struct {
	records []AgentAttemptDiagnostic
	err     error
}

func (recorder *agentDiagnosticRecorderFixture) RecordAgentAttemptDiagnostic(
	_ context.Context,
	record AgentAttemptDiagnostic,
) error {
	recorder.records = append(recorder.records, record)
	return recorder.err
}

func (sink *frameSinkFixture) AcceptFrame(
	_ context.Context,
	frame bridgev1.Frame,
) error {
	sink.frames = append(sink.frames, frame)
	return nil
}

func TestDeepSeekAgentAdapterConsumesExactFrozenBinding(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("private-deepseek-key")}
	diagnostics := &agentDiagnosticRecorderFixture{}
	doer := &httpDoerFixture{response: &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Implemented safely"}}],` +
				`"usage":{"prompt_tokens":120,"completion_tokens":31,"total_tokens":151,` +
				`"prompt_tokens_details":{"cached_tokens":20}}}`,
		)),
	}}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local",
		CredentialAccess:  access,
		Diagnostics:       diagnostics,
		Client:            doer,
		Now: func() time.Time {
			return time.Date(2026, 8, 10, 8, 30, 0, 0, time.UTC)
		},
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := deepSeekAdapterRequest(t, "deepseek-chat")
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}

	if access.uses != 1 || !reflect.DeepEqual(access.binding, request.ExecutionBinding) {
		t.Fatalf("credential access = uses %d binding %#v", access.uses, access.binding)
	}
	if doer.requests != 1 || doer.request == nil ||
		doer.request.Method != http.MethodPost ||
		doer.request.URL.String() != "https://api.deepseek.com/chat/completions" ||
		doer.auth != "Bearer private-deepseek-key" ||
		doer.request.Header.Get("Authorization") != "" ||
		!bytes.Contains(doer.body, []byte(`"model":"deepseek-chat"`)) ||
		!bytes.Contains(doer.body, []byte("provider=deepseek")) ||
		!bytes.Contains(doer.body, []byte("model=deepseek-chat")) ||
		!bytes.Contains(doer.body, []byte("No tools are available for this Agent attempt")) ||
		bytes.Contains(doer.body, []byte(`"tools"`)) ||
		bytes.Contains(doer.body, []byte("built on OpenAI")) ||
		!bytes.Contains(doer.body, []byte(`"stream":false`)) ||
		bytes.Contains(doer.body, []byte("private-deepseek-key")) {
		t.Fatalf("request = %#v auth=%q body=%s", doer.request, doer.auth, doer.body)
	}
	if !result.DispatchAcknowledged() || !result.ResultAcknowledged() ||
		result.ExitCode() != 0 || len(request.FrameSink.(*frameSinkFixture).frames) != 3 {
		t.Fatalf("result = %#v frames=%#v", result, request.FrameSink.(*frameSinkFixture).frames)
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	if frames[0].Type() != bridgev1.MessageAck ||
		string(frames[0].Payload()) != `{"message_id":"11111111-1111-4111-8111-111111111111"}` ||
		frames[1].Type() != bridgev1.MessageEvent ||
		string(frames[1].Payload()) != `{"delta":"Implemented safely"}` ||
		frames[2].Type() != bridgev1.MessageResult ||
		string(frames[2].Payload()) != `{"status":"succeeded","reason":""}` {
		t.Fatalf("frames = %#v", frames)
	}
	accounting, ok := result.Accounting()
	if !ok || !accounting.UsageObserved || accounting.InputTokens != 120 ||
		accounting.OutputTokens != 31 || accounting.TotalTokens != 151 ||
		accounting.CacheReadTokens != 20 || accounting.CostObserved {
		t.Fatalf("accounting = %#v, %t", accounting, ok)
	}
	if len(diagnostics.records) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
	diagnostic := diagnostics.records[0]
	if diagnostic.IncidentID != request.Dispatch.CorrelationID() ||
		diagnostic.ProviderID != "deepseek" ||
		diagnostic.ProviderAccountID != "deepseek-primary" ||
		diagnostic.Stage != "agent_attempt_dispatch" ||
		diagnostic.Result != "succeeded" || diagnostic.ErrorCode != "" ||
		diagnostic.Retryable || diagnostic.Elapsed < 0 {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}

func TestLoomNativeConsumesStepAndQueueInputsWithinOneFrozenAttempt(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("private-deepseek-key")}
	diagnostics := &agentDiagnosticRecorderFixture{}
	doer := &contextRetrievalHTTPDoerFixture{responses: []*http.Response{
		deepSeekAgentTextResponse("First bounded answer", 10, 2),
		deepSeekAgentTextResponse("Second bounded answer", 12, 3),
		deepSeekAgentTextResponse("Final bounded answer", 14, 4),
	}}
	step := runtimeAgentInputBatchFixture(
		t, agentinbox.ModeSteer, "input-steer", "narrow the implementation boundary",
		"turn-1", 1, "step-2", 2,
	)
	queue := runtimeAgentInputBatchFixture(
		t, agentinbox.ModeQueue, "input-queue", "continue with the accepted tests",
		"turn-2", 2, "step-1-turn-2", 1,
	)
	stepPlaintext := step.Inputs[0].Content
	queuePlaintext := queue.Inputs[0].Content
	source := &agentInputSourceFixture{batches: []loomruntime.AgentInputBatch{step, queue}}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local", CredentialAccess: access,
		Diagnostics: diagnostics, Client: doer,
		Now: func() time.Time {
			return time.Date(2026, 8, 14, 18, 0, 0, 0, time.UTC)
		},
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := deepSeekAdapterRequest(t, DeepSeekAgentModelID)
	request.AgentInputs = source
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if access.uses != 1 || len(doer.bodies) != 3 || len(source.checkpoints) != 3 ||
		!bytes.Contains(doer.bodies[0], []byte("Implement the bounded change")) ||
		bytes.Contains(doer.bodies[0], []byte("narrow the implementation boundary")) ||
		!bytes.Contains(doer.bodies[1], []byte("First bounded answer")) ||
		!bytes.Contains(doer.bodies[1], []byte("narrow the implementation boundary")) ||
		!bytes.Contains(doer.bodies[2], []byte("Second bounded answer")) ||
		!bytes.Contains(doer.bodies[2], []byte("continue with the accepted tests")) {
		t.Fatalf("Provider rounds/checkpoints = bodies=%q checkpoints=%#v", doer.bodies, source.checkpoints)
	}
	if !runtimeAgentInputAllZero(stepPlaintext) || !runtimeAgentInputAllZero(queuePlaintext) {
		t.Fatal("Runtime Agent input plaintext remained after Provider dispatch")
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	if len(frames) != 3 || frames[1].Type() != bridgev1.MessageEvent ||
		string(frames[1].Payload()) != `{"delta":"Final bounded answer"}` ||
		frames[2].Type() != bridgev1.MessageResult || !result.ResultAcknowledged() {
		t.Fatalf("single terminal frames/result = %#v / %#v", frames, result)
	}
	accounting, ok := result.Accounting()
	if !ok || accounting.InputTokens != 36 || accounting.OutputTokens != 9 ||
		accounting.TotalTokens != 45 {
		t.Fatalf("combined accounting = %#v, %t", accounting, ok)
	}
}

func TestLoomNativePersistsCheckpointBeforeConsumingAgentInput(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("private-deepseek-key")}
	doer := &contextRetrievalHTTPDoerFixture{responses: []*http.Response{
		deepSeekAgentTextResponse("First bounded answer", 10, 2),
		deepSeekAgentTextResponse("Final bounded answer", 12, 3),
	}}
	batch := runtimeAgentInputBatchFixture(
		t, agentinbox.ModeSteer, "input-steer", "continue safely",
		"turn-1", 1, "step-2", 2,
	)
	source := &durableAgentInputSourceFixture{
		agentInputSourceFixture: agentInputSourceFixture{
			batches: []loomruntime.AgentInputBatch{batch},
		},
	}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local", CredentialAccess: access,
		Diagnostics: &agentDiagnosticRecorderFixture{}, Client: doer,
		Now:              func() time.Time { return time.Date(2026, 8, 14, 18, 30, 0, 0, time.UTC) },
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := deepSeekAdapterRequest(t, DeepSeekAgentModelID)
	request.AgentInputs = source
	if _, err := adapter.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(source.persisted) != 2 || string(source.persisted[0]) != "First bounded answer" ||
		string(source.persisted[1]) != "Final bounded answer" ||
		strings.Join(source.events, ",") != "persist,next,persist,next" ||
		len(source.checkpoints) != 2 || len(doer.bodies) != 2 {
		t.Fatalf("durable checkpoints = persisted=%q events=%q checkpoints=%#v bodies=%d",
			source.persisted, source.events, source.checkpoints, len(doer.bodies))
	}
}

func TestLoomNativeCheckpointFailurePrecedesInputConsumptionAndNextProviderCall(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("private-deepseek-key")}
	doer := &contextRetrievalHTTPDoerFixture{responses: []*http.Response{
		deepSeekAgentTextResponse("First bounded answer", 10, 2),
		deepSeekAgentTextResponse("must not be requested", 12, 3),
	}}
	source := &durableAgentInputSourceFixture{
		agentInputSourceFixture: agentInputSourceFixture{batches: []loomruntime.AgentInputBatch{
			runtimeAgentInputBatchFixture(
				t, agentinbox.ModeSteer, "input-steer", "must remain pending",
				"turn-1", 1, "step-2", 2,
			),
		}},
		err: errors.New("checkpoint unavailable"),
	}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local", CredentialAccess: access,
		Diagnostics: &agentDiagnosticRecorderFixture{}, Client: doer,
		Now:              func() time.Time { return time.Date(2026, 8, 14, 18, 35, 0, 0, time.UTC) },
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := deepSeekAdapterRequest(t, DeepSeekAgentModelID)
	request.AgentInputs = source
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	if !result.ResultAcknowledged() || len(frames) != 2 ||
		!bytes.Contains(frames[1].Payload(), []byte(`"status":"failed"`)) ||
		len(doer.bodies) != 1 || len(source.checkpoints) != 0 ||
		strings.Join(source.events, ",") != "persist" {
		t.Fatalf("checkpoint failure = result=%#v bodies=%d checkpoints=%#v events=%q",
			result, len(doer.bodies), source.checkpoints, source.events)
	}
}

func TestLoomNativeResumesExactCheckpointAndConsumedInputWithoutReplayingInitialDispatch(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("private-deepseek-key")}
	doer := &contextRetrievalHTTPDoerFixture{responses: []*http.Response{
		deepSeekAgentTextResponse("Recovered final answer", 15, 4),
	}}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local", CredentialAccess: access,
		Diagnostics: &agentDiagnosticRecorderFixture{}, Client: doer,
		Now:              func() time.Time { return time.Date(2026, 8, 14, 20, 0, 0, 0, time.UTC) },
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	original, _ := deepSeekRetrievalAdapterRequest(t)
	original.ClaimID = "claim-recovery-1"
	original.IncidentID = original.Dispatch.CorrelationID()
	segmentID := "segment-recovery-1"
	batch := runtimeAgentInputBatchFixture(
		t, agentinbox.ModeQueue, "input-recovered", "continue from recovered input",
		"turn-2", 2, "step-2-1", 1,
	)
	for index := range batch.Inputs {
		binding := &batch.Inputs[index].Binding
		binding.ConversationID = original.ContextCapsule.ConversationID
		binding.SegmentID = segmentID
		binding.AgentInstanceID = original.Binding.SenderAgentInstanceID
		binding.WorkItemID = original.Binding.WorkItemID
		binding.RunID = original.Binding.RunID
		binding.ClaimGeneration = original.Binding.ClaimGeneration
		binding.RuntimeInstanceID = original.Binding.RuntimeInstanceID
		binding.ExecutionBindingDigest = original.ExecutionBinding.BindingDigest
		binding.CapsuleDigest = original.ContextCapsule.CapsuleDigest
	}
	defer batch.Close()
	previous := []byte("Previous encrypted checkpoint answer")
	digest := sha256.Sum256(previous)
	result, err := adapter.(LoomNativeAgentContinuationAdapter).ResumeAgentAttempt(
		context.Background(),
		LoomNativeAgentContinuationRequest{
			DispatchMessageID: "77777777-7777-4777-8777-777777777777",
			IncidentID:        original.IncidentID, ClaimID: original.ClaimID,
			WorkItemID: original.Binding.WorkItemID, RunID: original.Binding.RunID,
			ClaimGeneration:  original.Binding.ClaimGeneration,
			AgentInstanceID:  original.Binding.SenderAgentInstanceID,
			SegmentID:        segmentID,
			CheckpointDigest: hex.EncodeToString(digest[:]),
			ExecutionBinding: original.ExecutionBinding,
			ContextCapsule:   original.ContextCapsule,
			ContextPayload:   original.Dispatch.Payload(), PreviousOutput: previous,
			CurrentInputs: batch, FrameSink: original.FrameSink,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if access.uses != 1 || len(doer.bodies) != 1 ||
		!bytes.Contains(doer.bodies[0], []byte("Previous encrypted checkpoint answer")) ||
		!bytes.Contains(doer.bodies[0], []byte("continue from recovered input")) ||
		!bytes.Contains(doer.bodies[0], []byte(original.ContextCapsule.CapsuleDigest)) ||
		!result.ResultAcknowledged() {
		t.Fatalf("continued request = uses=%d bodies=%q result=%#v", access.uses, doer.bodies, result)
	}
	frames := original.FrameSink.(*frameSinkFixture).frames
	if len(frames) != 3 || string(frames[1].Payload()) != `{"delta":"Recovered final answer"}` {
		t.Fatalf("continued frames = %#v", frames)
	}
}

func TestLoomNativeResumeRejectsCheckpointAndSegmentSubstitutionBeforeCredentialOrHTTP(t *testing.T) {
	for name, mutate := range map[string]func(*LoomNativeAgentContinuationRequest){
		"checkpoint": func(request *LoomNativeAgentContinuationRequest) {
			request.PreviousOutput = []byte("substituted checkpoint")
		},
		"segment": func(request *LoomNativeAgentContinuationRequest) {
			request.SegmentID = "segment-substituted"
		},
	} {
		t.Run(name, func(t *testing.T) {
			access := &credentialAccessFixture{secret: []byte("private-deepseek-key")}
			doer := &contextRetrievalHTTPDoerFixture{}
			adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
				RuntimeInstanceID: "loom-native-local", CredentialAccess: access,
				Diagnostics: &agentDiagnosticRecorderFixture{}, Client: doer,
				Now:              func() time.Time { return time.Date(2026, 8, 14, 20, 5, 0, 0, time.UTC) },
				MaxResponseBytes: 64 << 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			original, _ := deepSeekRetrievalAdapterRequest(t)
			original.ClaimID = "claim-recovery-1"
			original.IncidentID = original.Dispatch.CorrelationID()
			segmentID := "segment-recovery-1"
			batch := runtimeAgentInputBatchFixture(
				t, agentinbox.ModeQueue, "input-recovered", "continue safely",
				"turn-2", 2, "step-2-1", 1,
			)
			for index := range batch.Inputs {
				binding := &batch.Inputs[index].Binding
				binding.ConversationID = original.ContextCapsule.ConversationID
				binding.SegmentID = segmentID
				binding.AgentInstanceID = original.Binding.SenderAgentInstanceID
				binding.WorkItemID = original.Binding.WorkItemID
				binding.RunID = original.Binding.RunID
				binding.ClaimGeneration = original.Binding.ClaimGeneration
				binding.RuntimeInstanceID = original.Binding.RuntimeInstanceID
				binding.ExecutionBindingDigest = original.ExecutionBinding.BindingDigest
				binding.CapsuleDigest = original.ContextCapsule.CapsuleDigest
			}
			defer batch.Close()
			previous := []byte("Previous encrypted checkpoint answer")
			digest := sha256.Sum256(previous)
			continuation := LoomNativeAgentContinuationRequest{
				DispatchMessageID: "77777777-7777-4777-8777-777777777777",
				IncidentID:        original.IncidentID, ClaimID: original.ClaimID,
				WorkItemID: original.Binding.WorkItemID, RunID: original.Binding.RunID,
				ClaimGeneration:  original.Binding.ClaimGeneration,
				AgentInstanceID:  original.Binding.SenderAgentInstanceID,
				SegmentID:        segmentID,
				CheckpointDigest: hex.EncodeToString(digest[:]),
				ExecutionBinding: original.ExecutionBinding,
				ContextCapsule:   original.ContextCapsule,
				ContextPayload:   original.Dispatch.Payload(), PreviousOutput: previous,
				CurrentInputs: batch, FrameSink: original.FrameSink,
			}
			mutate(&continuation)
			if _, err := adapter.(LoomNativeAgentContinuationAdapter).ResumeAgentAttempt(context.Background(), continuation); !errors.Is(err, ErrAgentExecutionBindingChanged) {
				t.Fatalf("substitution error = %v", err)
			}
			if access.uses != 0 || len(doer.bodies) != 0 {
				t.Fatalf("substitution reached credential/provider: %d/%d", access.uses, len(doer.bodies))
			}
		})
	}
}

func TestLoomNativeDeepSeekReturnsScopedContextOnlyToSameProviderAttempt(t *testing.T) {
	request, omitted := deepSeekRetrievalAdapterRequest(t)
	retrievedBody := "scoped omitted context must only enter the second Provider request"
	retriever := &contextRetrieverFixture{
		want: contextcapsule.RetrievalProposal{
			ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
			ArtifactRef: omitted.ArtifactRef,
		},
		content: []byte(retrievedBody),
	}
	request.ContextRetriever = retriever
	delivery := &contextDeliveryFixture{retriever: retriever}
	request.ContextDelivery = delivery
	doer := &contextRetrievalHTTPDoerFixture{responses: []*http.Response{
		{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-context-1","type":"function","function":{"name":"loom_read_context","arguments":"{\"item_id\":\"` + omitted.ItemID + `\",\"content_digest\":\"` + omitted.ContentDigest + `\",\"artifact_ref\":\"` + omitted.ArtifactRef + `\"}"}}]}}],` +
					`"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
			)),
		},
		{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Final answer uses the scoped evidence"}}],` +
					`"usage":{"prompt_tokens":15,"completion_tokens":4,"total_tokens":19}}`,
			)),
		},
	}, beforeDo: func(round int) error {
		if round == 2 && delivery.payload.Status != attemptpayload.StatusPending {
			return errors.New("tool result was not pending before Provider continuation")
		}
		return nil
	}}
	diagnostics := &agentDiagnosticRecorderFixture{}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-deepseek-key")},
		Diagnostics:       diagnostics, Client: doer,
		Now:              func() time.Time { return time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC) },
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if retriever.calls != 1 || len(doer.bodies) != 2 || len(doer.auth) != 2 {
		t.Fatalf("retrieval/provider calls = %d/%d auth=%#v", retriever.calls, len(doer.bodies), doer.auth)
	}
	if delivery.prepare != 1 || delivery.acks != 1 ||
		delivery.proof != attemptpayload.ProofProviderContinuation ||
		delivery.payload.Status != attemptpayload.StatusDelivered {
		t.Fatalf("delivery = %#v", delivery)
	}
	if !bytes.Contains(doer.bodies[0], []byte(`"name":"loom_read_context"`)) ||
		bytes.Contains(doer.bodies[0], []byte(retrievedBody)) ||
		!bytes.Contains(doer.bodies[1], []byte(`"role":"tool"`)) ||
		!bytes.Contains(doer.bodies[1], []byte(retrievedBody)) ||
		!bytes.Contains(doer.bodies[1], []byte(omitted.ContentDigest)) {
		t.Fatalf("Provider rounds = first=%s second=%s", doer.bodies[0], doer.bodies[1])
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	if len(frames) != 3 || string(frames[1].Payload()) !=
		`{"delta":"Final answer uses the scoped evidence"}` {
		t.Fatalf("frames = %#v", frames)
	}
	for _, frame := range frames {
		if bytes.Contains(frame.Payload(), []byte(retrievedBody)) {
			t.Fatalf("retrieved context leaked to Bridge frame: %s", frame.Payload())
		}
	}
	accounting, ok := result.Accounting()
	if !ok || !accounting.UsageObserved || accounting.InputTokens != 25 ||
		accounting.OutputTokens != 6 || accounting.TotalTokens != 31 {
		t.Fatalf("combined accounting = %#v, %t", accounting, ok)
	}
	if len(diagnostics.records) != 1 || diagnostics.records[0].Result != "succeeded" {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
}

func TestLoomNativeDeepSeekContextRetrievalDenialIsContentFree(t *testing.T) {
	request, omitted := deepSeekRetrievalAdapterRequest(t)
	retriever := &contextRetrieverFixture{
		want: contextcapsule.RetrievalProposal{
			ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
			ArtifactRef: omitted.ArtifactRef,
		},
		err: contextcapsule.ErrContextRetrievalDenied,
	}
	request.ContextRetriever = retriever
	request.ContextDelivery = &contextDeliveryFixture{retriever: retriever}
	doer := &contextRetrievalHTTPDoerFixture{responses: []*http.Response{{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-context-1","type":"function","function":{"name":"loom_read_context","arguments":"{\"item_id\":\"` + omitted.ItemID + `\",\"content_digest\":\"` + omitted.ContentDigest + `\",\"artifact_ref\":\"` + omitted.ArtifactRef + `\"}"}}]}}]}`,
		)),
	}, {
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"final answer"}}]}`,
		)),
	}}}
	diagnostics := &agentDiagnosticRecorderFixture{}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-deepseek-key")},
		Diagnostics:       diagnostics, Client: doer,
		Now:              func() time.Time { return time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC) },
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	// The denied retrieval is a bounded, content-free tool result: the model
	// recovers and produces a final answer (never a terminal denial leak).
	if len(frames) != 3 || string(frames[2].Payload()) !=
		`{"status":"succeeded","reason":""}` ||
		!result.DispatchAcknowledged() || !result.ResultAcknowledged() {
		t.Fatalf("result=%#v frames=%#v", result, frames)
	}
	// The denial tool result sent back to the provider must be the generic
	// content-free marker (the item id/digest legitimately appear only in the
	// echoed tool-call arguments, never in the denial message).
	if len(doer.bodies) != 2 ||
		!bytes.Contains(doer.bodies[1], []byte(`context_item_unavailable`)) {
		t.Fatalf("denial not surfaced as a bounded tool result: %d bodies", len(doer.bodies))
	}
}

func TestLoomNativeDeepSeekCompletesWhenSecondContextReadCannotDispatch(t *testing.T) {
	request, omitted := deepSeekRetrievalAdapterRequest(t)
	retriever := &contextRetrieverFixture{
		want: contextcapsule.RetrievalProposal{
			ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
			ArtifactRef: omitted.ArtifactRef,
		},
		err: contextcapsule.ErrContextItemNotRetrievable,
	}
	delivery := &conflictingContextDeliveryFixture{retriever: retriever}
	request.ContextRetriever = retriever
	request.ContextDelivery = delivery
	toolResponse := func(id string) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"` + id + `","type":"function","function":{"name":"loom_read_context","arguments":"{\"item_id\":\"` + omitted.ItemID + `\",\"content_digest\":\"` + omitted.ContentDigest + `\",\"artifact_ref\":\"` + omitted.ArtifactRef + `\"}"}}]}}]}`,
		))}
	}
	doer := &contextRetrievalHTTPDoerFixture{responses: []*http.Response{
		toolResponse("call-context-1"),
		toolResponse("call-context-2"),
		{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"final answer after bounded context denials"}}]}`,
		))},
	}}
	diagnostics := &agentDiagnosticRecorderFixture{}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-deepseek-key")},
		Diagnostics:       diagnostics, Client: doer,
		Now:              func() time.Time { return time.Date(2026, 8, 14, 13, 0, 0, 0, time.UTC) },
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("second undispatchable context-read must not fail the attempt: %v", err)
	}
	if delivery.prepare != 2 {
		t.Fatalf("delivery.prepare = %d, want 2", delivery.prepare)
	}
	// A second Context-read in the same step cannot be dispatched again, so
	// the adapter surfaces the same bounded denial inline and lets the model
	// answer. The attempt must complete as SUCCEEDED with the model's answer
	// (the step stays finalizable because every DISPATCHED call was
	// delivered); it must never hang or fail terminally.
	if !result.ResultAcknowledged() {
		t.Fatalf("result not acknowledged: %#v", result)
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	if len(frames) != 3 {
		t.Fatalf("frames = %#v", frames)
	}
	if !bytes.Contains(frames[1].Payload(), []byte("final answer after bounded context denials")) {
		t.Fatalf("unexpected completion frame: %s", frames[1].Payload())
	}
	if string(frames[2].Payload()) != `{"status":"succeeded","reason":""}` {
		t.Fatalf("unexpected result frame: %s", frames[2].Payload())
	}
}

func TestLoomNativeDeepSeekResumesPendingContextAcrossProviderCallIDDrift(t *testing.T) {
	request, omitted := deepSeekRetrievalAdapterRequest(t)
	retriever := &contextRetrieverFixture{
		want: contextcapsule.RetrievalProposal{
			ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
			ArtifactRef: omitted.ArtifactRef,
		},
		content: []byte("scoped omitted context must only enter the second Provider request"),
	}
	delivery := &contextDeliveryFixture{retriever: retriever}
	request.ContextRetriever = retriever
	request.ContextDelivery = delivery
	toolResponse := func(id string) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"` + id + `","type":"function","function":{"name":"loom_read_context","arguments":"{\"item_id\":\"` + omitted.ItemID + `\",\"content_digest\":\"` + omitted.ContentDigest + `\",\"artifact_ref\":\"` + omitted.ArtifactRef + `\"}"}}]}}]}`,
		))}
	}
	doer := &contextRetrievalHTTPDoerFixture{responses: []*http.Response{
		toolResponse("provider-call-before-crash"),
		{StatusCode: http.StatusOK, Body: io.NopCloser(agentTimeoutBodyFixture{})},
		toolResponse("provider-call-after-restart"),
		{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"resumed final"}}]}`,
		))},
	}}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-deepseek-key")},
		Diagnostics:       &agentDiagnosticRecorderFixture{}, Client: doer,
		Now:              func() time.Time { return time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC) },
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if retriever.calls != 1 || delivery.prepare != 1 || delivery.acks != 0 ||
		delivery.payload.Status != attemptpayload.StatusPending {
		t.Fatalf("after timeout retriever=%d delivery=%#v", retriever.calls, delivery)
	}
	request.FrameSink = &frameSinkFixture{}
	if _, err := adapter.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if retriever.calls != 1 || delivery.prepare != 2 || delivery.acks != 1 ||
		delivery.payload.Status != attemptpayload.StatusDelivered {
		t.Fatalf("after resume retriever=%d delivery=%#v", retriever.calls, delivery)
	}
	if len(doer.bodies) != 4 ||
		!bytes.Contains(doer.bodies[1], []byte(`"tool_call_id":"provider-call-before-crash"`)) ||
		!bytes.Contains(doer.bodies[3], []byte(`"tool_call_id":"provider-call-after-restart"`)) ||
		!bytes.Contains(doer.bodies[3], []byte("scoped omitted context must only enter the second Provider request")) {
		t.Fatalf("Provider resume bodies = %#v", doer.bodies)
	}
}

func TestDecodeContextToolCallRejectsProtocolDrift(t *testing.T) {
	digest := strings.Repeat("a", 64)
	tests := []struct {
		name      string
		id        string
		toolType  string
		toolName  string
		arguments string
	}{
		{name: "empty id", toolType: "function", toolName: "loom_read_context", arguments: `{"item_id":"x","content_digest":"` + digest + `"}`},
		{name: "wrong type", id: "call-1", toolType: "native", toolName: "loom_read_context", arguments: `{"item_id":"x","content_digest":"` + digest + `"}`},
		{name: "wrong name", id: "call-1", toolType: "function", toolName: "Read", arguments: `{"item_id":"x","content_digest":"` + digest + `"}`},
		{name: "duplicate key", id: "call-1", toolType: "function", toolName: "loom_read_context", arguments: `{"item_id":"x","item_id":"y","content_digest":"` + digest + `"}`},
		{name: "unknown key", id: "call-1", toolType: "function", toolName: "loom_read_context", arguments: `{"item_id":"x","content_digest":"` + digest + `","authority":"forged"}`},
		{name: "bad digest", id: "call-1", toolType: "function", toolName: "loom_read_context", arguments: `{"item_id":"x","content_digest":"not-a-digest"}`},
		{name: "control character", id: "call-1", toolType: "function", toolName: "loom_read_context", arguments: "{\"item_id\":\"x\\ny\",\"content_digest\":\"" + digest + "\"}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire := openAICompatibleToolCallWire{ID: test.id, Type: test.toolType}
			wire.Function.Name = test.toolName
			wire.Function.Arguments = test.arguments
			if _, err := decodeContextToolCall(wire); !errors.Is(err, ErrDeepSeekAgentProtocol) {
				t.Fatalf("decode error = %v", err)
			}
		})
	}

	wire := openAICompatibleToolCallWire{ID: "call-1", Type: "function"}
	wire.Function.Name = "loom_read_context"
	wire.Function.Arguments = `{"item_id":"team-state","content_digest":"` + digest + `"}`
	call, err := decodeContextToolCall(wire)
	if err != nil || call.Proposal.ArtifactRef != "" {
		t.Fatalf("shared-scope call = %#v, %v", call, err)
	}
}

func TestDecodeProviderContextToolRejectsDuplicateOuterKeys(t *testing.T) {
	body := []byte(`{"model":"deepseek-chat","model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"answer"}}]}`)
	if _, err := decodeDeepSeekAgentResponse(body, DeepSeekAgentModelID); err == nil {
		t.Fatal("duplicate Provider response key was accepted")
	}
}

func TestDecodeProviderContextToolRejectsUnknownToolCallFields(t *testing.T) {
	body := []byte(`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"loom_read_context","arguments":"{\"item_id\":\"x\",\"content_digest\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"}"},"provider_extension":true}]}}]}`)
	if _, err := decodeDeepSeekAgentResponse(body, DeepSeekAgentModelID); err == nil {
		t.Fatal("unknown Provider tool-call field was accepted")
	}
}

func TestMarshalContextToolResultRejectsBodyAndClassificationDrift(t *testing.T) {
	content := []byte("exact scoped context")
	digest := digestContextToolContent(content)
	proposal := contextcapsule.RetrievalProposal{
		ItemID: "context-1", ContentDigest: digest, ArtifactRef: "artifact:1",
	}
	valid := contextcapsule.RetrievedItem{
		ItemID: proposal.ItemID, Kind: contextcapsule.KindArtifactReference,
		Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
		Content: append([]byte(nil), content...), ContentDigest: digest,
		SourceType: contextcapsule.SourceObservation, SourceRef: "evidence:1",
		ArtifactRef: proposal.ArtifactRef,
	}
	if payload, err := marshalContextToolResult(proposal, valid); err != nil ||
		!bytes.Contains(payload, content) {
		t.Fatalf("valid result = %s, %v", payload, err)
	}
	tests := map[string]contextcapsule.RetrievedItem{
		"body digest drift": func() contextcapsule.RetrievedItem {
			item := valid
			item.Content = []byte("different body")
			return item
		}(),
		"trust escalation": func() contextcapsule.RetrievedItem {
			item := valid
			item.Trust = contextcapsule.TrustAuthoritative
			return item
		}(),
		"scope artifact mismatch": func() contextcapsule.RetrievedItem {
			item := valid
			item.Scope = contextcapsule.ScopeTeamShared
			return item
		}(),
		"credential kind": func() contextcapsule.RetrievedItem {
			item := valid
			item.Kind = contextcapsule.KindCredentialReference
			return item
		}(),
		"secret marker": func() contextcapsule.RetrievedItem {
			item := valid
			item.Content = []byte("TOKEN=must-not-enter-provider")
			item.ContentDigest = digestContextToolContent(item.Content)
			return item
		}(),
	}
	for name, item := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := proposal
			candidate.ContentDigest = item.ContentDigest
			if _, err := marshalContextToolResult(candidate, item); !errors.Is(err, ErrDeepSeekAgentProtocol) {
				t.Fatalf("marshal error = %v", err)
			}
		})
	}
}

func TestLoomNativeDeepSeekSupportsBoundedSequentialContextToolCalls(t *testing.T) {
	request, omitted := deepSeekRetrievalAdapterRequest(t)
	retriever := &contextRetrieverFixture{
		want: contextcapsule.RetrievalProposal{
			ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
			ArtifactRef: omitted.ArtifactRef,
		},
		content: []byte("scoped omitted context must only enter the second Provider request"),
	}
	request.ContextRetriever = retriever
	delivery := &contextDeliveryFixture{retriever: retriever}
	request.ContextDelivery = delivery
	toolResponse := func(id string, inputTokens int, outputTokens int) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			fmt.Sprintf(
				`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"%s","type":"function","function":{"name":"loom_read_context","arguments":"{\"item_id\":\"%s\",\"content_digest\":\"%s\",\"artifact_ref\":\"%s\"}"}}]}}],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d}}`,
				id, omitted.ItemID, omitted.ContentDigest, omitted.ArtifactRef,
				inputTokens, outputTokens, inputTokens+outputTokens,
			),
		))}
	}
	doer := &contextRetrievalHTTPDoerFixture{responses: []*http.Response{
		toolResponse("call-context-1", 10, 2), toolResponse("call-context-2", 15, 3),
		{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"final after two context calls"}}],"usage":{"prompt_tokens":30,"completion_tokens":5,"total_tokens":35}}`,
		))},
	}}
	diagnostics := &agentDiagnosticRecorderFixture{}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-deepseek-key")},
		Diagnostics:       diagnostics, Client: doer,
		Now:              func() time.Time { return time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC) },
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if retriever.calls != 2 || delivery.prepare != 2 || delivery.acks != 2 ||
		len(doer.bodies) != 3 || len(doer.auth) != 3 {
		t.Fatalf("retrieval/delivery/provider = %d/%d/%d/%d", retriever.calls, delivery.prepare, delivery.acks, len(doer.bodies))
	}
	for index, authorization := range doer.auth {
		if authorization != "Bearer private-deepseek-key" {
			t.Fatalf("Provider request %d Authorization = %q", index, authorization)
		}
		if bytes.Contains(doer.bodies[index], []byte("private-deepseek-key")) {
			t.Fatalf("Provider request %d body contains credential", index)
		}
	}
	if !bytes.Contains(doer.bodies[1], []byte(`"tool_call_id":"call-context-1"`)) ||
		!bytes.Contains(doer.bodies[2], []byte(`"tool_call_id":"call-context-2"`)) ||
		!bytes.Contains(doer.bodies[2], []byte(`"name":"loom_read_context"`)) {
		t.Fatalf("sequential Provider bodies = %#v", doer.bodies)
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	if len(frames) != 3 || string(frames[1].Payload()) !=
		`{"delta":"final after two context calls"}` ||
		!result.DispatchAcknowledged() || !result.ResultAcknowledged() {
		t.Fatalf("sequential result=%#v frames=%#v", result, frames)
	}
	for _, frame := range frames {
		if bytes.Contains(frame.Payload(), retriever.content) {
			t.Fatalf("sequential Context content leaked to Bridge frame: %s", frame.Payload())
		}
	}
	accounting, ok := result.Accounting()
	if !ok || !accounting.UsageObserved || accounting.InputTokens != 55 ||
		accounting.OutputTokens != 10 || accounting.TotalTokens != 65 {
		t.Fatalf("sequential accounting = %#v, %t", accounting, ok)
	}
	if len(diagnostics.records) != 1 || diagnostics.records[0].Result != "succeeded" {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
}

func TestLoomNativeDeepSeekContextReadExhaustionCompletesCleanly(t *testing.T) {
	request, omitted := deepSeekRetrievalAdapterRequest(t)
	retriever := &contextRetrieverFixture{
		want: contextcapsule.RetrievalProposal{
			ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
			ArtifactRef: omitted.ArtifactRef,
		},
		content: []byte("scoped omitted context must only enter the second Provider request"),
	}
	delivery := &contextDeliveryFixture{retriever: retriever}
	request.ContextRetriever = retriever
	request.ContextDelivery = delivery
	toolResponse := func(index int) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			fmt.Sprintf(
				`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-context-%d","type":"function","function":{"name":"loom_read_context","arguments":"{\"item_id\":\"%s\",\"content_digest\":\"%s\",\"artifact_ref\":\"%s\"}"}}]}}]}`,
				index, omitted.ItemID, omitted.ContentDigest, omitted.ArtifactRef,
			),
		))}
	}
	doer := &contextRetrievalHTTPDoerFixture{}
	for index := 1; index <= contextToolMaxCallsPerExchange+1; index++ {
		doer.responses = append(doer.responses, toolResponse(index))
	}
	// After the exhaustion directive the model answers without further tools.
	doer.responses = append(doer.responses, &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"final answer after exhaustion"}}]}`,
		)),
	})
	diagnostics := &agentDiagnosticRecorderFixture{}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-deepseek-key")},
		Diagnostics:       diagnostics, Client: doer,
		Now:              func() time.Time { return time.Date(2026, 8, 14, 13, 0, 0, 0, time.UTC) },
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if retriever.calls != contextToolMaxCallsPerExchange ||
		delivery.prepare != contextToolMaxCallsPerExchange ||
		delivery.acks != contextToolMaxCallsPerExchange ||
		len(doer.bodies) != contextToolMaxCallsPerExchange+2 {
		t.Fatalf("bounded calls retriever=%d prepare=%d ack=%d provider=%d", retriever.calls, delivery.prepare, delivery.acks, len(doer.bodies))
	}
	// The final Provider request must NOT advertise the context tool and must
	// surface the exhaustion directive in the conversation history.
	last := doer.bodies[len(doer.bodies)-1]
	if bytes.Contains(last, []byte(`"tools"`)) {
		t.Fatal("final Provider request after exhaustion still advertised a tool")
	}
	if !bytes.Contains(last, []byte("context-read limit for this attempt is exhausted")) {
		t.Fatal("exhaustion directive was not surfaced to the Provider in the final request")
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	if len(frames) != 3 || string(frames[2].Payload()) !=
		`{"status":"succeeded","reason":""}` ||
		!bytes.Contains(frames[1].Payload(), []byte("final answer after exhaustion")) ||
		!result.DispatchAcknowledged() || !result.ResultAcknowledged() {
		t.Fatalf("bounded result=%#v frames=%#v", result, frames)
	}
	if len(diagnostics.records) != 1 ||
		diagnostics.records[0].Result != "succeeded" {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
}

func TestLoomNativeDeepSeekContextReadExhaustionEndsCleanlyWhenModelIgnoresDirective(t *testing.T) {
	request, omitted := deepSeekRetrievalAdapterRequest(t)
	retriever := &contextRetrieverFixture{
		want: contextcapsule.RetrievalProposal{
			ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
			ArtifactRef: omitted.ArtifactRef,
		},
		content: []byte("scoped omitted context must only enter the second Provider request"),
	}
	delivery := &contextDeliveryFixture{retriever: retriever}
	request.ContextRetriever = retriever
	request.ContextDelivery = delivery
	toolResponse := func(index int) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			fmt.Sprintf(
				`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-context-%d","type":"function","function":{"name":"loom_read_context","arguments":"{\"item_id\":\"%s\",\"content_digest\":\"%s\",\"artifact_ref\":\"%s\"}"}}]}}]}`,
				index, omitted.ItemID, omitted.ContentDigest, omitted.ArtifactRef,
			),
		))}
	}
	doer := &contextRetrievalHTTPDoerFixture{}
	for index := 1; index <= contextToolMaxCallsPerExchange+2; index++ {
		doer.responses = append(doer.responses, toolResponse(index))
	}
	diagnostics := &agentDiagnosticRecorderFixture{}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-deepseek-key")},
		Diagnostics:       diagnostics, Client: doer,
		Now:              func() time.Time { return time.Date(2026, 8, 14, 13, 0, 0, 0, time.UTC) },
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	// The exchange must end cleanly (succeeded with the bounded fallback) even
	// when the model keeps calling the tool after the exhaustion directive:
	// a denied context item must never become a terminal attempt failure.
	frames := request.FrameSink.(*frameSinkFixture).frames
	if len(frames) != 3 || string(frames[2].Payload()) !=
		`{"status":"succeeded","reason":""}` ||
		!bytes.Contains(frames[1].Payload(), []byte("No further information was produced")) ||
		!result.DispatchAcknowledged() || !result.ResultAcknowledged() {
		t.Fatalf("bounded result=%#v frames=%#v", result, frames)
	}
	if len(diagnostics.records) != 1 ||
		diagnostics.records[0].Result != "succeeded" {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
}

func TestDeepSeekAgentAdapterRejectsUnsupportedFrozenBindingBeforeCredentialAccess(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
	doer := &httpDoerFixture{}
	diagnostics := &agentDiagnosticRecorderFixture{}
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local",
		CredentialAccess:  access,
		Diagnostics:       diagnostics,
		Client:            doer,
		Now:               func() time.Time { return time.Now().UTC() },
		MaxResponseBytes:  64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := deepSeekAdapterRequest(t, "deepseek-reasoner")
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(
		err,
		ErrAgentExecutionBindingChanged,
	) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || doer.requests != 0 {
		t.Fatalf("unsupported binding used credential=%d requests=%d", access.uses, doer.requests)
	}
	if len(diagnostics.records) != 1 ||
		diagnostics.records[0].Stage != "agent_attempt_dispatch" ||
		diagnostics.records[0].ErrorCode != "binding_changed" {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
}

func TestDeepSeekAgentAdapterRequiresFrozenCapabilityAndBrokerTogether(t *testing.T) {
	tests := []struct {
		name            string
		request         func(testing.TB) supervisor.AdapterRequest
		attachRetriever bool
	}{
		{
			name: "retriever without frozen capability",
			request: func(t testing.TB) supervisor.AdapterRequest {
				request := deepSeekAdapterRequest(t, DeepSeekAgentModelID)
				request, _ = retrievalAdapterRequest(t, request)
				return request
			},
			attachRetriever: true,
		},
		{
			name: "frozen capability without broker",
			request: func(t testing.TB) supervisor.AdapterRequest {
				request, _ := deepSeekRetrievalAdapterRequest(t)
				return request
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
			doer := &httpDoerFixture{}
			diagnostics := &agentDiagnosticRecorderFixture{}
			adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
				RuntimeInstanceID: "loom-native-local", CredentialAccess: access,
				Diagnostics: diagnostics, Client: doer,
				Now: func() time.Time { return time.Now().UTC() }, MaxResponseBytes: 64 << 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			request := test.request(t)
			if test.attachRetriever {
				request.ContextRetriever = &contextRetrieverFixture{}
			}
			_, err = adapter.Execute(context.Background(), request)
			if test.name == "frozen capability without broker" {
				// A completely context-free Execute (no retriever, no
				// delivery) is valid even when the binding lists the
				// context_retrieval capability: the independent verifier
				// runs exactly this shape. It must not be rejected as a
				// binding change.
				if errors.Is(err, ErrAgentExecutionBindingChanged) {
					t.Fatalf("context-free Execute() error = %v", err)
				}
				return
			}
			if !errors.Is(err, ErrAgentExecutionBindingChanged) {
				t.Fatalf("Execute() error = %v", err)
			}
			if access.uses != 0 || doer.requests != 0 || len(diagnostics.records) != 1 ||
				diagnostics.records[0].ErrorCode != "binding_changed" {
				t.Fatalf(
					"credential=%d Provider=%d diagnostics=%#v",
					access.uses, doer.requests, diagnostics.records,
				)
			}
		})
	}
}

func TestDeepSeekAgentAdapterProjectsCredentialAndProviderFailuresToThisAttempt(t *testing.T) {
	tests := []struct {
		name       string
		accessErr  error
		doerErr    error
		body       io.ReadCloser
		statusCode int
		reason     string
		stage      string
		retryable  bool
	}{
		{
			name: "credential revision unavailable", accessErr: ErrAgentCredentialUnavailable,
			reason: "credential_unavailable", retryable: true,
		},
		{
			name: "Vault AAD validation failed",
			accessErr: credentials.WithCredentialFailureStage(
				credentials.CredentialStageVaultAADValidation,
				ErrAgentCredentialUnavailable,
			),
			reason: "credential_unavailable",
			stage:  credentials.CredentialStageVaultAADValidation,
		},
		{name: "provider rejected credential", statusCode: http.StatusUnauthorized, reason: "provider_auth"},
		{
			name: "provider rate limited account", statusCode: http.StatusTooManyRequests,
			reason: "provider_rate_limit", retryable: true,
		},
		{
			name: "provider unavailable", statusCode: http.StatusServiceUnavailable,
			reason: "provider_unavailable", retryable: true,
		},
		{
			name: "Provider transport timed out", doerErr: agentProviderTimeoutFixture{},
			reason: "timeout", stage: "provider_connect", retryable: true,
		},
		{
			name: "Provider response body timed out", body: agentTimeoutBodyFixture{},
			statusCode: http.StatusOK, reason: "timeout", stage: "provider_http", retryable: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			access := &credentialAccessFixture{
				secret: []byte("private-deepseek-key"), err: test.accessErr,
			}
			doer := &httpDoerFixture{err: test.doerErr}
			diagnostics := &agentDiagnosticRecorderFixture{}
			if test.statusCode != 0 {
				body := test.body
				if body == nil {
					body = io.NopCloser(strings.NewReader(
						`{"error":{"message":"must never escape"}}`,
					))
				}
				doer.response = &http.Response{
					StatusCode: test.statusCode,
					Body:       body,
				}
			}
			adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
				RuntimeInstanceID: "loom-native-local",
				CredentialAccess:  access,
				Diagnostics:       diagnostics,
				Client:            doer,
				Now: func() time.Time {
					return time.Date(2026, 8, 10, 8, 30, 0, 0, time.UTC)
				},
				MaxResponseBytes: 64 << 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			request := deepSeekAdapterRequest(t, "deepseek-chat")
			result, err := adapter.Execute(context.Background(), request)
			if err != nil {
				t.Fatalf("Execute() leaked operational error: %v", err)
			}
			frames := request.FrameSink.(*frameSinkFixture).frames
			if len(frames) != 2 || frames[0].Type() != bridgev1.MessageAck ||
				frames[1].Type() != bridgev1.MessageResult ||
				string(frames[1].Payload()) != `{"status":"failed","reason":"`+test.reason+`"}` ||
				!result.DispatchAcknowledged() || !result.ResultAcknowledged() {
				t.Fatalf("result=%#v frames=%#v", result, frames)
			}
			forbidden := "private-deepseek-keymust never escape"
			for _, frame := range frames {
				for _, value := range []string{"private-deepseek-key", "must never escape"} {
					if strings.Contains(string(frame.Payload()), value) || strings.Contains(errString(err), value) {
						t.Fatalf("forbidden value leaked from %q: %q", forbidden, value)
					}
				}
			}
			wantStage := test.stage
			if wantStage == "" {
				wantStage = expectedDiagnosticStage(test.reason)
			}
			if len(diagnostics.records) != 1 ||
				diagnostics.records[0].ErrorCode != test.reason ||
				diagnostics.records[0].Stage != wantStage ||
				diagnostics.records[0].Retryable != test.retryable {
				t.Fatalf("diagnostics = %#v", diagnostics.records)
			}
		})
	}
}

func expectedDiagnosticStage(reason string) string {
	switch reason {
	case "credential_unavailable":
		return "credential_lease_issue"
	case "provider_auth":
		return "provider_auth"
	case "provider_rate_limit":
		return "provider_rate_limit"
	default:
		return "provider_http"
	}
}

func deepSeekRetrievalAdapterRequest(
	t testing.TB,
) (supervisor.AdapterRequest, contextcapsule.OmittedItem) {
	t.Helper()
	request := deepSeekAdapterRequestWithCapabilities(
		t,
		DeepSeekAgentModelID,
		[]string{loomruntime.CapabilityContextRetrieval},
	)
	return retrievalAdapterRequest(t, request)
}

func retrievalAdapterRequest(
	t testing.TB,
	request supervisor.AdapterRequest,
) (supervisor.AdapterRequest, contextcapsule.OmittedItem) {
	t.Helper()
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "conversation-retrieval-1", TeamID: "team-retrieval-1",
			AgentID: request.Binding.SenderAgentInstanceID, RoleID: "researcher",
			ProviderID:        request.ExecutionBinding.ProviderID,
			ProviderAccountID: request.ExecutionBinding.ProviderAccountID,
			ModelID:           request.ExecutionBinding.ModelID, AuthMode: "brokered",
			ContextAdapterID:   "context:loom-native:v1",
			DisclosurePolicyID: "policy.test", DisclosurePolicyVersion: 1,
			ArtifactRefs: []string{"artifact:diff-1"}, TokenBudget: 4,
		},
		[]contextcapsule.ItemInput{
			{
				ItemID: "goal-1", Kind: contextcapsule.KindConversationGoal,
				Trust:    contextcapsule.TrustAuthoritative,
				Scope:    contextcapsule.ScopeTeamShared,
				Priority: contextcapsule.PrioritySystem, TokenCount: 4, Required: true,
				Content:    []byte("Review the exact bounded diff."),
				SourceType: contextcapsule.SourceAuthority, SourceRef: "goal:retrieval-1",
			},
			{
				ItemID: "diff-detail", Kind: contextcapsule.KindArtifactReference,
				Trust:    contextcapsule.TrustObserved,
				Scope:    contextcapsule.ScopeArtifactScoped,
				Priority: contextcapsule.PriorityRetrievable, TokenCount: 5,
				Content:    []byte("scoped omitted context must only enter the second Provider request"),
				SourceType: contextcapsule.SourceObservation, SourceRef: "evidence:diff-1",
				ArtifactRef: "artifact:diff-1",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:     request.Dispatch.MessageID(),
		CorrelationID: request.Dispatch.CorrelationID(),
		WorkItemID:    request.Binding.WorkItemID, RunID: request.Binding.RunID,
		ClaimGeneration:       request.Binding.ClaimGeneration,
		RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
		SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
		Sequence:              1, Type: bridgev1.MessageDispatch,
		EmittedAt: time.Date(2026, 8, 12, 12, 59, 59, 0, time.UTC),
		Payload:   payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	request.Dispatch = dispatch
	request.ContextCapsule = capsule.AuthorityRecord()
	omitted := capsule.Omitted()
	if len(omitted) != 1 || omitted[0].Reason != contextcapsule.OmissionBudgetExceeded {
		t.Fatalf("omissions = %#v", omitted)
	}
	return request, omitted[0]
}

func deepSeekAdapterRequest(t testing.TB, model string) supervisor.AdapterRequest {
	t.Helper()
	return deepSeekAdapterRequestWithCapabilities(t, model, nil)
}

func TestLoomNativeAdapterAdvertisesExactRestartCheckpointConformance(t *testing.T) {
	request := deepSeekAdapterRequest(t, DeepSeekAgentModelID)
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: request.ExecutionBinding.RuntimeInstanceID,
		CredentialAccess:  &credentialAccessFixture{secret: []byte("unused-secret")},
		Diagnostics:       &agentDiagnosticRecorderFixture{},
		Client:            &httpDoerFixture{},
		Now:               func() time.Time { return time.Now().UTC() },
		MaxResponseBytes:  64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	conformance, ok := adapter.(loomruntime.AgentAttemptRestartConformance)
	if !ok {
		t.Fatal("Loom Native adapter does not advertise restart conformance")
	}
	if conformance.AgentAttemptRestartContract() != loomruntime.AgentAttemptRestartLoomOwnedCheckpointV1 {
		t.Fatalf("restart contract = %q", conformance.AgentAttemptRestartContract())
	}
	if err := conformance.ValidateAgentAttemptRestartBinding(request.ExecutionBinding); err != nil {
		t.Fatalf("exact binding rejected: %v", err)
	}

	changed := request.ExecutionBinding
	changed.ProviderAccountID = "deepseek.secondary"
	if err := conformance.ValidateAgentAttemptRestartBinding(changed); !errors.Is(err, ErrAgentExecutionBindingChanged) {
		t.Fatalf("substituted binding error = %v", err)
	}
}

func deepSeekAdapterRequestWithCapabilities(
	t testing.TB,
	model string,
	capabilities []string,
) supervisor.AdapterRequest {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:                   "deepseek-agent-primary-v1",
		AdapterType:          "loom-native",
		ProviderID:           "deepseek",
		ProviderAccountID:    "deepseek-primary",
		ModelID:              model,
		AuthMode:             loomruntime.AuthBrokered,
		EndpointFingerprint:  deepSeekEndpointFingerprintFixture,
		CredentialReference:  "credential-ref-deepseek-primary",
		CredentialRevision:   7,
		RequiredCapabilities: append([]string(nil), capabilities...),
		Timeout:              30 * time.Second,
		Budget:               int64Pointer(2500),
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "loom-native-local", DeviceID: "device-local",
		AdapterType: "loom-native", DisplayName: "Loom Native",
		Status:               loomruntime.RuntimeOnline,
		ObservedCapabilities: append([]string(nil), capabilities...),
		Capacity:             3,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             "11111111-1111-4111-8111-111111111111",
		CorrelationID:         "22222222-2222-4222-8222-222222222222",
		WorkItemID:            "work-main-1",
		RunID:                 "run-main-1",
		ClaimGeneration:       1,
		RuntimeInstanceID:     instance.ID,
		SenderAgentInstanceID: "agent-main-1",
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt:             time.Date(2026, 8, 10, 8, 29, 59, 0, time.UTC),
		Payload:               []byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"Implement the bounded change"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	return supervisor.AdapterRequest{
		Binding: bridgev1.RunStreamBinding{
			WorkItemID: "work-main-1", RunID: "run-main-1",
			ClaimGeneration: 1, RuntimeInstanceID: instance.ID,
			SenderAgentInstanceID: "agent-main-1",
		},
		ExecutionBinding: binding,
		Dispatch:         dispatch,
		FrameSink:        &frameSinkFixture{},
	}
}

func int64Pointer(value int64) *int64 { return &value }

func deepSeekAgentTextResponse(content string, inputTokens, outputTokens int64) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(fmt.Sprintf(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":%q}}],`+
				`"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d}}`,
			content, inputTokens, outputTokens, inputTokens+outputTokens,
		))),
	}
}

func runtimeAgentInputBatchFixture(
	t *testing.T,
	mode agentinbox.Mode,
	inputID string,
	content string,
	turnID string,
	turnSequence int,
	stepID string,
	stepSequence int,
) loomruntime.AgentInputBatch {
	t.Helper()
	digest := sha256.Sum256([]byte(content))
	binding := agentinbox.Binding{
		PayloadID: "payload-" + inputID, InputID: inputID, Mode: mode,
		ContextScope:   agentinbox.ScopeAgentPrivate,
		ConversationID: "conversation-1", SegmentID: "segment-1",
		AgentInstanceID: "agent-main-1", WorkItemID: "work-main-1",
		RunID: "run-main-1", ClaimGeneration: 1,
		RuntimeInstanceID:      "loom-native-local",
		ExecutionBindingDigest: strings.Repeat("1", 64),
		CapsuleDigest:          strings.Repeat("2", 64), OrderKey: 1,
		ContentType: "text/plain", ContentDigest: hex.EncodeToString(digest[:]),
	}
	if mode == agentinbox.ModeQueue {
		binding.TargetTurnID, binding.TargetTurnSequence = turnID, turnSequence
	} else {
		binding.TargetStepID, binding.TargetStepSequence = stepID, stepSequence
	}
	batch, err := loomruntime.NewAgentInputBatch(
		turnID, turnSequence, stepID, stepSequence,
		[]agentinbox.Payload{{
			Binding: binding, Status: agentinbox.StatusConsumed, Content: []byte(content),
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func runtimeAgentInputAllZero(content []byte) bool {
	for _, value := range content {
		if value != 0 {
			return false
		}
	}
	return true
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestLoomNativeDispatchAcceptsLargeMissionRoleContextPrompt(t *testing.T) {
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "conversation-mission", TeamID: "team-mixed",
			AgentID: "agent-deepseek", RoleID: "coordinator",
			ProviderID:        DeepSeekAgentProviderID,
			ProviderAccountID: "deepseek.primary", ModelID: DeepSeekAgentModelID,
			AuthMode: "brokered", ContextAdapterID: "context:loom-native:v1",
			DisclosurePolicyID: "policy.test", DisclosurePolicyVersion: 1,
			TokenBudget: 1024,
		},
		[]contextcapsule.ItemInput{{
			ItemID: "objective-1", Kind: contextcapsule.KindConversationGoal,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			Priority: contextcapsule.PrioritySystem, TokenCount: 256, Required: true,
			Content: []byte(strings.Repeat(
				"Verify the mixed-provider Team runs end to end with real provider calls. ",
				160,
			)),
			SourceType: contextcapsule.SourceAuthority, SourceRef: "goal:phase-2d",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture prompt exceeds the legacy 4 KiB bound that rejected real
	// mission prompts, while staying inside the 32 KiB loom-native capsule cap.
	const legacyPromptBound = 4096
	if len(payload) <= legacyPromptBound {
		t.Fatalf("fixture prompt is not large: %d bytes", len(payload))
	}
	dispatch, err := decodeDeepSeekAgentDispatch(payload)
	if err != nil || !strings.Contains(dispatch.Prompt, "mixed-provider Team") {
		t.Fatalf("large context dispatch = %#v, %v", dispatch, err)
	}
}

func TestNativeAdapterWebToolDecodeAndModelFormat(t *testing.T) {
	// web_search decode
	search, err := decodeWebToolCall(openAICompatibleToolCallWire{
		ID: "call-1", Type: "function",
		Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "loom_web_search", Arguments: `{"query":"multica github"}`},
	})
	if err != nil || search.Kind != permissions.ToolWebSearch ||
		search.Call.Path != "multica github" {
		t.Fatalf("web_search decode = %#v, %v", search, err)
	}
	// web_fetch decode
	fetch, err := decodeWebToolCall(openAICompatibleToolCallWire{
		ID: "call-2", Type: "function",
		Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "loom_web_fetch", Arguments: `{"url":"https://example.com"}`},
	})
	if err != nil || fetch.Kind != permissions.ToolWebFetch ||
		fetch.Call.Path != "https://example.com" {
		t.Fatalf("web_fetch decode = %#v, %v", fetch, err)
	}
	// unknown tool rejected
	if _, err := decodeWebToolCall(openAICompatibleToolCallWire{
		ID: "call-3", Type: "function",
		Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "loom_read_context", Arguments: `{}`},
	}); err == nil {
		t.Fatal("unknown tool accepted")
	}
	// tool definitions are well-formed function schemas
	for _, definition := range []any{webSearchToolDefinition(), webFetchToolDefinition()} {
		if _, err := json.Marshal(definition); err != nil {
			t.Fatalf("tool definition marshal: %v", err)
		}
	}
	// relaxed response-model format check (DeepSeek returns deepseek-v4-flash
	// for a deepseek-chat request; exact-equality would reject every turn)
	if !validNativeAgentResponseModel("deepseek-v4-flash") ||
		!validNativeAgentResponseModel("deepseek-chat") ||
		validNativeAgentResponseModel("") ||
		validNativeAgentResponseModel("bad model!") {
		t.Fatal("response model format validation incorrect")
	}
}

func TestVerifierPromptDispatchDecodesStrictly(t *testing.T) {
	payload := []byte(`{"schema_version":1,"kind":"pi_verifier_prompt","prompt":"Verify the source attempt output against the criteria and reply with a JSON verdict."}`)
	dispatch, err := decodeDeepSeekAgentDispatch(payload)
	if err != nil {
		t.Fatalf("verifier dispatch decode = %v", err)
	}
	if !dispatch.verifier || dispatch.Kind != piVerifierPromptKind ||
		!strings.Contains(dispatch.Prompt, "Verify the source attempt") {
		t.Fatalf("verifier dispatch = %#v", dispatch)
	}
	// A plain mission dispatch must stay non-verifier.
	mission, err := decodeDeepSeekAgentDispatch([]byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"Implement the bounded change"}`))
	if err != nil || mission.verifier {
		t.Fatalf("mission dispatch verifier flag = %#v, %v", mission, err)
	}
	// Unknown fields and wrong kinds are rejected.
	for _, bad := range []string{
		`{"schema_version":1,"kind":"pi_verifier_prompt","prompt":"x","extra":1}`,
		`{"schema_version":2,"kind":"pi_verifier_prompt","prompt":"x"}`,
		`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":""}`,
	} {
		if _, err := decodeDeepSeekAgentDispatch([]byte(bad)); err == nil {
			t.Fatalf("accepted malformed dispatch %s", bad)
		}
	}
}

func TestVerifierTerminalFromVerdict(t *testing.T) {
	for _, test := range []struct {
		content string
		status  string
		reason  string
	}{
		{`{"verdict":"satisfied","summary":"ok"}`, "succeeded", ""},
		{`Here is my review: {"verdict":"satisfied"}`, "succeeded", ""},
		{`{"verdict":"criteria_satisfied"}`, "succeeded", ""},
		{"criteria_satisfied", "succeeded", ""},
		{"criteria_not_satisfied", "failed", "criteria_not_satisfied"},
		{"insufficient_evidence", "failed", "insufficient_evidence"},
		{`{"verdict":"not_satisfied","summary":"missing source URL"}`, "failed", "criteria_not_satisfied"},
		{`{"verdict":"insufficient_evidence"}`, "failed", "insufficient_evidence"},
		{`I cannot verify this.`, "failed", "insufficient_evidence"},
		{``, "failed", "insufficient_evidence"},
		{`{"verdict":"maybe"}`, "failed", "insufficient_evidence"},
	} {
		status, reason := verifierTerminalFromVerdict(test.content)
		if status != test.status || reason != test.reason {
			t.Fatalf("verdict %q = (%q,%q), want (%q,%q)",
				test.content, status, reason, test.status, test.reason)
		}
	}
}
