package harnessadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/permissions"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

type harnessContextRetrieverFixture struct {
	item          contextcapsule.RetrievedItem
	want          contextcapsule.RetrievalProposal
	issuedContent []byte
	calls         int
}

func TestHarnessContextMCPCancellationRevokesListenerAndToken(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	service, err := newHarnessContextMCPWithContext(
		ctx, &harnessContextRetrieverFixture{},
	)
	if err != nil {
		t.Fatal(err)
	}
	lease := service.Lease()
	address := strings.TrimSuffix(
		strings.TrimPrefix(lease.URL, "http://"), "/mcp",
	)
	connection, err := net.DialTimeout("tcp4", address, time.Second)
	if err != nil {
		t.Fatalf("Context MCP did not listen before cancellation: %v", err)
	}
	_ = connection.Close()
	cancel()
	deadline := time.Now().Add(time.Second)
	for {
		connection, dialErr := net.DialTimeout("tcp4", address, 50*time.Millisecond)
		if connection != nil {
			_ = connection.Close()
		}
		if dialErr != nil && service.Lease().Token == "" {
			break
		}
		if time.Now().After(deadline) {
			service.Close()
			t.Fatalf(
				"Context MCP survived cancellation: dial=%v token_present=%t",
				dialErr, service.Lease().Token != "",
			)
		}
		time.Sleep(10 * time.Millisecond)
	}
	service.Close()
}

func (fixture *harnessContextRetrieverFixture) Retrieve(
	_ context.Context,
	proposal contextcapsule.RetrievalProposal,
) (contextcapsule.RetrievedItem, error) {
	fixture.calls++
	if proposal != fixture.want {
		return contextcapsule.RetrievedItem{}, contextcapsule.ErrContextRetrievalDenied
	}
	item := fixture.item
	item.Content = bytes.Clone(fixture.item.Content)
	fixture.issuedContent = item.Content
	return item, nil
}

type harnessContextDeliveryFixture struct {
	retriever    *harnessContextRetrieverFixture
	payload      attemptpayload.Payload
	payloads     []attemptpayload.Payload
	prepares     int
	acks         int
	ackSequences []int64
	ackFailures  map[int64]int
	proof        attemptpayload.DeliveryProof
}

func (fixture *harnessContextDeliveryFixture) Prepare(
	ctx context.Context,
	proposal contextcapsule.RetrievalProposal,
	request contextcapsule.DeliveryRequest,
	encode contextcapsule.DeliveryEncoder,
) (attemptpayload.Payload, error) {
	fixture.prepares++
	item, err := fixture.retriever.Retrieve(ctx, proposal)
	if err != nil {
		return attemptpayload.Payload{}, err
	}
	content, err := encode(proposal, item)
	item.Close()
	if err != nil {
		return attemptpayload.Payload{}, err
	}
	digest := sha256.Sum256(content)
	fixture.payload = attemptpayload.Payload{
		Binding: attemptpayload.Binding{
			PayloadID: "payload-harness-context", CallID: contextcapsule.DeliveryCallID(proposal, request.Sequence),
			Sequence: request.Sequence, ContentType: request.ContentType,
			ContentDigest: hex.EncodeToString(digest[:]),
		},
		Status: attemptpayload.StatusPending, Content: append([]byte(nil), content...),
	}
	fixture.payloads = append(fixture.payloads, fixture.payload)
	payload := fixture.payload
	payload.Content = append([]byte(nil), fixture.payload.Content...)
	return payload, nil
}

func (fixture *harnessContextDeliveryFixture) Acknowledge(
	_ context.Context,
	binding attemptpayload.Binding,
	proof attemptpayload.DeliveryProof,
) error {
	matched := -1
	for index := range fixture.payloads {
		if fixture.payloads[index].Binding == binding {
			matched = index
			break
		}
	}
	if matched < 0 || proof != attemptpayload.ProofHarnessFinalOutput {
		return contextcapsule.ErrInvalidContextDelivery
	}
	if fixture.ackFailures != nil && fixture.ackFailures[binding.Sequence] > 0 {
		fixture.ackFailures[binding.Sequence]--
		return errors.New("fixture acknowledgement failure")
	}
	fixture.acks++
	fixture.ackSequences = append(fixture.ackSequences, binding.Sequence)
	fixture.proof = proof
	fixture.payloads[matched].Status = attemptpayload.StatusDelivered
	if fixture.payload.Binding == binding {
		fixture.payload.Status = attemptpayload.StatusDelivered
	}
	return nil
}

type harnessMultiContextDeliveryFixture struct {
	items          map[contextcapsule.RetrievalProposal]contextcapsule.RetrievedItem
	payloads       []attemptpayload.Payload
	ackSequences   []int64
	ackFailures    map[int64]int
	prepareStarted chan struct{}
	prepareRelease chan struct{}
}

type harnessToolGatewayFixture struct {
	allowed      []permissions.ToolKind
	contents     map[permissions.ToolKind][]byte
	envelopes    []loomruntime.ToolCallEnvelope
	bindings     []loomruntime.ToolCallBinding
	sequences    []int64
	ackSequences []int64
}

func (fixture *harnessToolGatewayFixture) AllowedToolCalls() []permissions.ToolKind {
	return append([]permissions.ToolKind(nil), fixture.allowed...)
}

func (fixture *harnessToolGatewayFixture) ExecuteToolCall(
	ctx context.Context,
	envelope loomruntime.ToolCallEnvelope,
	binding loomruntime.ToolCallBinding,
) (loomruntime.ToolCallResult, error) {
	sequence, ok := loomruntime.ToolCallSequence(ctx)
	if !ok {
		return loomruntime.ToolCallResult{}, errors.New("missing ToolCall sequence")
	}
	content := bytes.Clone(fixture.contents[envelope.Call.Tool])
	if len(content) == 0 {
		return loomruntime.ToolCallResult{}, errors.New("unexpected tool")
	}
	fixture.envelopes = append(fixture.envelopes, envelope)
	fixture.bindings = append(fixture.bindings, binding)
	fixture.sequences = append(fixture.sequences, sequence)
	digest := sha256.Sum256(content)
	contentDigest := "sha256:" + hex.EncodeToString(digest[:])
	return loomruntime.ToolCallResult{
		Verdict: permissions.VerdictAllow, ExecutionID: fmt.Sprintf("execution-%d", sequence),
		OutputDigest: contentDigest, ContentDigest: contentDigest,
		Delivery: &loomruntime.ToolCallDelivery{
			Binding: attemptpayload.Binding{
				PayloadID: fmt.Sprintf("payload-tool-%d", sequence),
				CallID:    fmt.Sprintf("call-tool-%d", sequence), Sequence: sequence,
				ContentType:   attemptpayload.ContentTypeTextUTF8,
				ContentDigest: hex.EncodeToString(digest[:]),
			},
			CallDigest: permissions.ProposedCallDigest(envelope.Call),
			Tool:       envelope.Call.Tool, OperationID: fmt.Sprintf("operation-%d", sequence),
		},
	}, nil
}

func (fixture *harnessToolGatewayFixture) ReadToolCallResultContent(
	_ context.Context,
	_ loomruntime.ToolCallBinding,
	result loomruntime.ToolCallResult,
) ([]byte, error) {
	if result.Delivery == nil {
		return nil, errors.New("missing delivery")
	}
	return bytes.Clone(fixture.contents[result.Delivery.Tool]), nil
}

func (fixture *harnessToolGatewayFixture) AcknowledgeToolCallResultWithProof(
	_ context.Context,
	_ loomruntime.ToolCallBinding,
	result loomruntime.ToolCallResult,
	proof attemptpayload.DeliveryProof,
) error {
	if result.Delivery == nil || proof != attemptpayload.ProofHarnessFinalOutput {
		return errors.New("invalid Harness ToolCall acknowledgement")
	}
	fixture.ackSequences = append(
		fixture.ackSequences, result.Delivery.Binding.Sequence,
	)
	return nil
}

func TestHarnessAttemptMCPExecutesOnlyGovernedReadAndGrepUntilFinalOutput(t *testing.T) {
	gateway := &harnessToolGatewayFixture{
		allowed: []permissions.ToolKind{
			permissions.ToolBash, permissions.ToolRead, permissions.ToolGrep,
			permissions.ToolEdit,
		},
		contents: map[permissions.ToolKind][]byte{
			permissions.ToolRead: []byte("bounded source content\n"),
			permissions.ToolGrep: []byte("src/main.go:12: bounded match\n"),
		},
	}
	binding := loomruntime.ToolCallBinding{
		ConversationID: "conversation-tools", WorkItemID: "work-tools",
		RunID: "run-tools", ClaimGeneration: 3, RuntimeInstanceID: "runtime-codex",
		AgentInstanceID: "agent-codex", ExecutionBindingDigest: strings.Repeat("1", 64),
		CapsuleDigest: strings.Repeat("2", 64), ClaimID: "claim-tools",
		IncidentID: "11111111-1111-4111-8111-111111111111",
		JourneyID:  "11111111-1111-4111-8111-111111111111",
	}
	service, err := newHarnessAttemptMCPWithContext(
		context.Background(), harnessAttemptMCPConfig{
			ToolGateway: gateway, ToolBinding: binding,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	tools := service.toolNames()
	if !reflect.DeepEqual(tools, []string{"loom_grep_files", "loom_read_file"}) {
		t.Fatalf("published Harness tools = %v", tools)
	}
	calls := []struct {
		name      string
		arguments map[string]any
		content   []byte
	}{
		{
			name: "loom_read_file", arguments: map[string]any{"path": "src/main.go"},
			content: gateway.contents[permissions.ToolRead],
		},
		{
			name:      "loom_grep_files",
			arguments: map[string]any{"path": "src", "pattern": "governed"},
			content:   gateway.contents[permissions.ToolGrep],
		},
	}
	for index, call := range calls {
		params, marshalErr := json.Marshal(map[string]any{
			"name": call.name, "arguments": call.arguments,
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		response, status := service.call(
			context.Background(), json.RawMessage(strconv.Itoa(index+1)), params,
		)
		if status != http.StatusOK || !bytes.Contains(response, bytes.TrimSpace(call.content)) ||
			bytes.Contains(response, []byte("context_retrieval_denied")) {
			t.Fatalf("%s status=%d response=%s", call.name, status, response)
		}
		if len(gateway.ackSequences) != 0 {
			t.Fatalf("%s acknowledged before final output", call.name)
		}
	}
	if !reflect.DeepEqual(gateway.sequences, []int64{1, 2}) ||
		gateway.envelopes[0].JobID != binding.WorkItemID ||
		gateway.envelopes[0].Call != (permissions.ProposedCall{
			Tool: permissions.ToolRead, Path: "src/main.go",
		}) ||
		gateway.envelopes[1].Call != (permissions.ProposedCall{
			Tool: permissions.ToolGrep, Path: "src", Pattern: "governed",
		}) {
		t.Fatalf("gateway calls = %#v / %v", gateway.envelopes, gateway.sequences)
	}
	if err := service.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gateway.ackSequences, []int64{1, 2}) {
		t.Fatalf("Harness ToolCall acknowledgements = %v", gateway.ackSequences)
	}
}

func TestHarnessAttemptMCPRejectsUnpublishedAndMalformedTools(t *testing.T) {
	gateway := &harnessToolGatewayFixture{
		allowed: []permissions.ToolKind{permissions.ToolRead, permissions.ToolGrep},
		contents: map[permissions.ToolKind][]byte{
			permissions.ToolRead: []byte("content"), permissions.ToolGrep: []byte("match"),
		},
	}
	service, err := newHarnessAttemptMCPWithContext(
		context.Background(), harnessAttemptMCPConfig{
			ToolGateway: gateway,
			ToolBinding: loomruntime.ToolCallBinding{
				ConversationID: "conversation-tools", WorkItemID: "work-tools",
				RunID: "run-tools", ClaimGeneration: 1, RuntimeInstanceID: "runtime-codex",
				AgentInstanceID: "agent-codex", ExecutionBindingDigest: strings.Repeat("1", 64),
				CapsuleDigest: strings.Repeat("2", 64), ClaimID: "claim-tools",
				IncidentID: "11111111-1111-4111-8111-111111111111",
				JourneyID:  "11111111-1111-4111-8111-111111111111",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	for index, params := range []string{
		`{"name":"loom_read_file","arguments":{"path":"src/main.go","command":"cat"}}`,
		`{"name":"loom_grep_files","arguments":{"path":"src","pattern":""}}`,
		`{"name":"loom_bash","arguments":{"command":"id"}}`,
		`{"name":"loom_web_search","arguments":{"query":"secret"}}`,
		`{"name":"loom_read_context","arguments":{"item_id":"x","content_digest":"` + strings.Repeat("1", 64) + `"}}`,
	} {
		response, status := service.call(
			context.Background(), json.RawMessage(strconv.Itoa(index+1)), json.RawMessage(params),
		)
		if status != http.StatusOK || !bytes.Contains(response, []byte("invalid_request")) {
			t.Fatalf("case %d status=%d response=%s", index, status, response)
		}
	}
	if len(gateway.envelopes) != 0 {
		t.Fatalf("rejected calls reached gateway: %#v", gateway.envelopes)
	}
}

func (fixture *harnessMultiContextDeliveryFixture) Prepare(
	_ context.Context,
	proposal contextcapsule.RetrievalProposal,
	request contextcapsule.DeliveryRequest,
	encode contextcapsule.DeliveryEncoder,
) (attemptpayload.Payload, error) {
	if fixture.prepareStarted != nil {
		close(fixture.prepareStarted)
		<-fixture.prepareRelease
	}
	item, ok := fixture.items[proposal]
	if !ok {
		return attemptpayload.Payload{}, contextcapsule.ErrContextRetrievalDenied
	}
	item.Content = bytes.Clone(item.Content)
	content, err := encode(proposal, item)
	item.Close()
	if err != nil {
		return attemptpayload.Payload{}, err
	}
	digest := sha256.Sum256(content)
	payload := attemptpayload.Payload{
		Binding: attemptpayload.Binding{
			PayloadID: "payload-harness-context-" + strconv.FormatInt(request.Sequence, 10),
			CallID:    contextcapsule.DeliveryCallID(proposal, request.Sequence),
			Sequence:  request.Sequence, ContentType: request.ContentType,
			ContentDigest: hex.EncodeToString(digest[:]),
		},
		Status: attemptpayload.StatusPending, Content: bytes.Clone(content),
	}
	zeroHarnessBytes(content)
	fixture.payloads = append(fixture.payloads, payload)
	result := payload
	result.Content = bytes.Clone(payload.Content)
	return result, nil
}

func TestHarnessContextMCPRejectsFinalOutputAckDuringPreparation(t *testing.T) {
	content := []byte("concurrently prepared context")
	proposal := contextcapsule.RetrievalProposal{
		ItemID: "concurrent-context", ContentDigest: harnessContextDigest(content),
	}
	delivery := &harnessMultiContextDeliveryFixture{
		items: map[contextcapsule.RetrievalProposal]contextcapsule.RetrievedItem{
			proposal: {
				ItemID: proposal.ItemID, Kind: contextcapsule.KindCurrentTaskState,
				Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeConversationShared,
				ContentDigest: proposal.ContentDigest, SourceType: contextcapsule.SourceAuthority,
				SourceRef: "authority:" + proposal.ItemID, Content: content,
			},
		},
		prepareStarted: make(chan struct{}), prepareRelease: make(chan struct{}),
	}
	service, err := newHarnessContextDeliveryMCP(delivery)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	params, err := json.Marshal(map[string]any{
		"name": "loom_read_context", "arguments": map[string]any{
			"item_id": proposal.ItemID, "content_digest": proposal.ContentDigest,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan []byte, 1)
	go func() {
		response, _ := service.call(context.Background(), json.RawMessage(`1`), params)
		finished <- response
	}()
	<-delivery.prepareStarted
	if err := service.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); !errors.Is(err, ErrHarnessContextMCP) {
		t.Fatalf("in-flight acknowledgement error = %v", err)
	}
	close(delivery.prepareRelease)
	if response := <-finished; bytes.Contains(response, []byte("context_retrieval_denied")) {
		t.Fatalf("prepared Context call failed after rejected early ack: %s", response)
	}
	if err := service.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(delivery.ackSequences, []int64{1}) {
		t.Fatalf("final acknowledgement order = %v", delivery.ackSequences)
	}
}

func (fixture *harnessMultiContextDeliveryFixture) Acknowledge(
	_ context.Context,
	binding attemptpayload.Binding,
	proof attemptpayload.DeliveryProof,
) error {
	if proof != attemptpayload.ProofHarnessFinalOutput {
		return contextcapsule.ErrInvalidContextDelivery
	}
	for index := range fixture.payloads {
		if fixture.payloads[index].Binding != binding {
			continue
		}
		if fixture.ackFailures != nil && fixture.ackFailures[binding.Sequence] > 0 {
			fixture.ackFailures[binding.Sequence]--
			return errors.New("fixture acknowledgement failure")
		}
		fixture.payloads[index].Status = attemptpayload.StatusDelivered
		fixture.ackSequences = append(fixture.ackSequences, binding.Sequence)
		return nil
	}
	return contextcapsule.ErrInvalidContextDelivery
}

func TestHarnessContextMCPBatchesFourDistinctReadsUntilHarnessFinalOutput(t *testing.T) {
	delivery := &harnessMultiContextDeliveryFixture{
		items: make(map[contextcapsule.RetrievalProposal]contextcapsule.RetrievedItem),
	}
	proposals := make([]contextcapsule.RetrievalProposal, 5)
	for index := range proposals {
		content := []byte(fmt.Sprintf("governed context %d", index+1))
		proposal := contextcapsule.RetrievalProposal{
			ItemID:        fmt.Sprintf("context-%d", index+1),
			ContentDigest: harnessContextDigest(content),
		}
		proposals[index] = proposal
		delivery.items[proposal] = contextcapsule.RetrievedItem{
			ItemID: proposal.ItemID, Kind: contextcapsule.KindCurrentTaskState,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeConversationShared,
			ContentDigest: proposal.ContentDigest, SourceType: contextcapsule.SourceAuthority,
			SourceRef: "authority:" + proposal.ItemID, Content: content,
		}
	}
	service, err := newHarnessContextDeliveryMCP(delivery)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	call := func(id int, proposal contextcapsule.RetrievalProposal) []byte {
		params, marshalErr := json.Marshal(map[string]any{
			"name": "loom_read_context",
			"arguments": map[string]any{
				"item_id": proposal.ItemID, "content_digest": proposal.ContentDigest,
			},
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		response, status := service.call(
			context.Background(), json.RawMessage(strconv.Itoa(id)), params,
		)
		if status != http.StatusOK {
			t.Fatalf("call %d status = %d", id, status)
		}
		return response
	}
	for index := 0; index < 2; index++ {
		if response := call(index+1, proposals[index]); bytes.Contains(response, []byte("context_retrieval_denied")) {
			t.Fatalf("call %d denied: %s", index+1, response)
		}
	}
	if response := call(20, proposals[0]); !bytes.Contains(response, []byte("context_retrieval_denied")) {
		t.Fatalf("duplicate proposal was admitted: %s", response)
	}
	for index := 2; index < 4; index++ {
		if response := call(index+1, proposals[index]); bytes.Contains(response, []byte("context_retrieval_denied")) {
			t.Fatalf("call %d denied: %s", index+1, response)
		}
	}
	if response := call(5, proposals[4]); !bytes.Contains(response, []byte("context_retrieval_denied")) {
		t.Fatalf("fifth proposal was admitted: %s", response)
	}
	if len(delivery.payloads) != 4 || len(delivery.ackSequences) != 0 {
		t.Fatalf("before final output payloads=%d acks=%v", len(delivery.payloads), delivery.ackSequences)
	}
	for index, payload := range delivery.payloads {
		if payload.Binding.Sequence != int64(index+1) || payload.Status != attemptpayload.StatusPending {
			t.Fatalf("pending payload %d = %#v", index, payload)
		}
	}
	if err := service.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(delivery.ackSequences, []int64{1, 2, 3, 4}) {
		t.Fatalf("acknowledgement order = %v", delivery.ackSequences)
	}
}

func TestHarnessContextMCPAcknowledgementRetryResumesAtFailedBinding(t *testing.T) {
	delivery := &harnessMultiContextDeliveryFixture{
		items:       make(map[contextcapsule.RetrievalProposal]contextcapsule.RetrievedItem),
		ackFailures: map[int64]int{2: 1},
	}
	service, err := newHarnessContextDeliveryMCP(delivery)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	for index := 1; index <= 3; index++ {
		content := []byte(fmt.Sprintf("retry context %d", index))
		proposal := contextcapsule.RetrievalProposal{
			ItemID:        fmt.Sprintf("retry-context-%d", index),
			ContentDigest: harnessContextDigest(content),
		}
		delivery.items[proposal] = contextcapsule.RetrievedItem{
			ItemID: proposal.ItemID, Kind: contextcapsule.KindCurrentTaskState,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeConversationShared,
			ContentDigest: proposal.ContentDigest, SourceType: contextcapsule.SourceAuthority,
			SourceRef: "authority:" + proposal.ItemID, Content: content,
		}
		params, marshalErr := json.Marshal(map[string]any{
			"name": "loom_read_context", "arguments": map[string]any{
				"item_id": proposal.ItemID, "content_digest": proposal.ContentDigest,
			},
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		response, status := service.call(
			context.Background(), json.RawMessage(strconv.Itoa(index)), params,
		)
		if status != http.StatusOK || bytes.Contains(response, []byte("context_retrieval_denied")) {
			t.Fatalf("call %d status=%d response=%s", index, status, response)
		}
	}
	if err := service.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); !errors.Is(err, ErrHarnessContextMCP) {
		t.Fatalf("first acknowledgement error = %v", err)
	}
	if !reflect.DeepEqual(delivery.ackSequences, []int64{1}) {
		t.Fatalf("first acknowledgement order = %v", delivery.ackSequences)
	}
	if err := service.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(delivery.ackSequences, []int64{1, 2, 3}) {
		t.Fatalf("resumed acknowledgement order = %v", delivery.ackSequences)
	}
}

func TestHarnessContextMCPDefersDeliveryAckUntilValidatedHarnessFinalOutput(t *testing.T) {
	content := []byte("persisted Harness Context")
	proposal := contextcapsule.RetrievalProposal{
		ItemID: "current-task", ContentDigest: harnessContextDigest(content),
	}
	retriever := &harnessContextRetrieverFixture{
		want: proposal,
		item: contextcapsule.RetrievedItem{
			ItemID: proposal.ItemID, Kind: contextcapsule.KindCurrentTaskState,
			Trust:         contextcapsule.TrustAuthoritative,
			Scope:         contextcapsule.ScopeConversationShared,
			ContentDigest: proposal.ContentDigest, SourceType: contextcapsule.SourceAuthority,
			SourceRef: "authority:current-task", Content: content,
		},
	}
	delivery := &harnessContextDeliveryFixture{retriever: retriever}
	service, err := newHarnessContextDeliveryMCP(delivery)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	params, err := json.Marshal(map[string]any{
		"name": "loom_read_context",
		"arguments": map[string]any{
			"item_id": proposal.ItemID, "content_digest": proposal.ContentDigest,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response, status := service.call(context.Background(), json.RawMessage(`1`), params)
	if status != http.StatusOK || !bytes.Contains(response, content) ||
		delivery.prepares != 1 || delivery.acks != 0 ||
		delivery.payload.Status != attemptpayload.StatusPending {
		t.Fatalf("HTTP delivery status=%d response=%s fixture=%#v", status, response, delivery)
	}
	if err := service.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); err != nil {
		t.Fatal(err)
	}
	if delivery.acks != 1 || delivery.proof != attemptpayload.ProofHarnessFinalOutput ||
		delivery.payload.Status != attemptpayload.StatusDelivered {
		t.Fatalf("final output acknowledgement = %#v", delivery)
	}
}

func TestHarnessContextMCPServesOneAuthorizedReadAndCloses(t *testing.T) {
	content := []byte("bounded observed Harness Context")
	proposal := contextcapsule.RetrievalProposal{
		ItemID: "test-output", ContentDigest: harnessContextDigest(content),
		ArtifactRef: "artifact:test-output",
	}
	retriever := &harnessContextRetrieverFixture{
		want: proposal,
		item: contextcapsule.RetrievedItem{
			ItemID: proposal.ItemID, Kind: contextcapsule.KindArtifactReference,
			Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
			ContentDigest: proposal.ContentDigest, SourceType: contextcapsule.SourceObservation,
			SourceRef: "test:test-output", ArtifactRef: proposal.ArtifactRef,
			Content: content,
		},
	}
	service, err := newHarnessContextMCP(retriever)
	if err != nil {
		t.Fatal(err)
	}
	lease := service.Lease()
	if !validHarnessContextMCPLease(lease) || strings.Contains(lease.URL, lease.Token) {
		t.Fatalf("invalid private MCP lease = %#v", lease)
	}
	client := &http.Client{}
	post := func(payload []byte) (int, []byte) {
		request, requestErr := http.NewRequest(http.MethodPost, lease.URL, bytes.NewReader(payload))
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		request.Header.Set("Authorization", "Bearer "+lease.Token)
		response, requestErr := client.Do(request)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		defer response.Body.Close()
		body, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			t.Fatal(readErr)
		}
		return response.StatusCode, body
	}
	status, body := post([]byte(`{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"loom-test","version":"1"}}}`))
	if status != http.StatusOK ||
		!bytes.Contains(body, []byte(`"protocolVersion":"2025-03-26"`)) {
		t.Fatalf("initialize status=%d body=%s", status, body)
	}
	status, body = post([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`))
	if status != http.StatusAccepted || len(body) != 0 {
		t.Fatalf("initialized status=%d body=%s", status, body)
	}
	status, body = post([]byte(`{"jsonrpc":"2.0","id":"list","method":"tools/list","params":{}}`))
	if status != http.StatusOK ||
		!bytes.Contains(body, []byte(`"name":"loom_read_context"`)) ||
		!bytes.Contains(body, []byte(`"readOnlyHint":true`)) ||
		!bytes.Contains(body, []byte(`"openWorldHint":false`)) {
		t.Fatalf("tools/list status=%d body=%s", status, body)
	}
	status, body = post([]byte(`{"jsonrpc":"2.0","id":"list-meta","method":"tools/list","params":{"_meta":{"progressToken":"progress-1"}}}`))
	if status != http.StatusOK ||
		!bytes.Contains(body, []byte(`"name":"loom_read_context"`)) {
		t.Fatalf("tools/list metadata status=%d body=%s", status, body)
	}
	call := func(id int) (int, []byte) {
		payload, marshalErr := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": id, "method": "tools/call",
			"params": map[string]any{
				"name": "loom_read_context",
				"_meta": map[string]any{
					"threadId": "019fe932-beab-7c03-9282-790a0bb0becd",
					"x-codex-turn-metadata": map[string]any{
						"model": "gpt-5.4", "reasoning_effort": "high",
					},
					"codex_bridge_mcp_call_id": "mcp-call-1",
					"claudecode/toolUseId":     "toolu-context-1",
				},
				"arguments": map[string]any{
					"item_id": proposal.ItemID, "content_digest": proposal.ContentDigest,
					"artifact_ref": proposal.ArtifactRef,
				},
			},
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return post(payload)
	}
	status, body = call(1)
	if status != http.StatusOK || !bytes.Contains(body, content) || retriever.calls != 1 {
		t.Fatalf("first call status=%d body=%s calls=%d", status, body, retriever.calls)
	}
	if len(retriever.issuedContent) == 0 ||
		!bytes.Equal(retriever.issuedContent, make([]byte, len(retriever.issuedContent))) {
		t.Fatalf("retrieved mutable content was not zeroized = %q", retriever.issuedContent)
	}
	status, body = call(2)
	if status != http.StatusOK || !bytes.Contains(body, []byte("context_retrieval_denied")) ||
		retriever.calls != 1 {
		t.Fatalf("second call status=%d body=%s calls=%d", status, body, retriever.calls)
	}
	service.Close()
	request, _ := http.NewRequest(http.MethodPost, lease.URL, strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+lease.Token)
	if _, err := client.Do(request); err == nil {
		t.Fatal("closed Attempt MCP remained reachable")
	}
}

func TestHarnessContextMCPRejectsAuthAndProtocolDrift(t *testing.T) {
	service, err := newHarnessContextMCP(&harnessContextRetrieverFixture{})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	for _, test := range []struct {
		name, token, payload string
		status               int
	}{
		{"wrong token", "wrong", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, 401},
		{"duplicate", service.Lease().Token, `{"jsonrpc":"2.0","id":1,"id":2,"method":"tools/list"}`, 400},
		{"batch", service.Lease().Token, `[]`, 400},
		{"unknown field", service.Lease().Token, `{"jsonrpc":"2.0","id":1,"method":"tools/list","extra":true}`, 400},
		{"unknown protocol", service.Lease().Token, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2099-01-01","capabilities":{},"clientInfo":{"name":"client","version":"1"}}}`, 400},
		{"invalid capabilities", service.Lease().Token, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":[],"clientInfo":{"name":"client","version":"1"}}}`, 400},
		{"invalid client info", service.Lease().Token, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"client","version":1}}}`, 400},
		{"claude client info", service.Lease().Token, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{"elicitation":{},"roots":{}},"clientInfo":{"name":"claude-code","version":"2.1.196","title":"Claude Code","description":"Anthropic CLI","websiteUrl":"https://claude.ai/code"}}}`, 200},
		{"client info drift", service.Lease().Token, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"client","version":"1","secret":"forbidden"}}}`, 400},
		{"client website credentials", service.Lease().Token, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"client","version":"1","websiteUrl":"https://user:secret@example.com"}}}`, 400},
		{"list cursor", service.Lease().Token, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"cursor":"next"}}`, 400},
		{"list metadata drift", service.Lease().Token, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"secret":"forbidden"}}}`, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, _ := http.NewRequest(http.MethodPost, service.Lease().URL, strings.NewReader(test.payload))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+test.token)
			response, requestErr := http.DefaultClient.Do(request)
			if requestErr != nil {
				t.Fatal(requestErr)
			}
			defer response.Body.Close()
			if response.StatusCode != test.status {
				t.Fatalf("status=%d, want %d", response.StatusCode, test.status)
			}
		})
	}
}

func TestHarnessContextMCPRejectsUntrustedCallMetadataDrift(t *testing.T) {
	content := []byte("bounded context")
	proposal := contextcapsule.RetrievalProposal{
		ItemID: "context-item", ContentDigest: harnessContextDigest(content),
	}
	for _, test := range []struct {
		name string
		meta any
	}{
		{"unknown metadata field", map[string]any{"unknown": "value"}},
		{"unknown Claude metadata", map[string]any{"claudecode/secret": "forbidden"}},
		{"secret metadata", map[string]any{"x-codex-turn-metadata": map[string]any{"api_key": "forbidden"}}},
		{"control character", map[string]any{"threadId": "thread\u0000id"}},
		{"nested too deeply", map[string]any{
			"x-codex-turn-metadata": map[string]any{
				"a": map[string]any{
					"b": map[string]any{
						"c": map[string]any{
							"d": map[string]any{"e": true},
						},
					},
				},
			},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			retriever := &harnessContextRetrieverFixture{
				want: proposal,
				item: contextcapsule.RetrievedItem{
					ItemID: proposal.ItemID, Kind: contextcapsule.KindCurrentTaskState,
					Trust:         contextcapsule.TrustAuthoritative,
					Scope:         contextcapsule.ScopeConversationShared,
					ContentDigest: proposal.ContentDigest,
					SourceType:    contextcapsule.SourceAuthority,
					SourceRef:     "authority:task", Content: content,
				},
			}
			service, err := newHarnessContextMCP(retriever)
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			payload, err := json.Marshal(map[string]any{
				"jsonrpc": "2.0", "id": 1, "method": "tools/call",
				"params": map[string]any{
					"name": "loom_read_context", "_meta": test.meta,
					"arguments": map[string]any{
						"item_id":        proposal.ItemID,
						"content_digest": proposal.ContentDigest,
					},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			request, _ := http.NewRequest(http.MethodPost, service.Lease().URL, bytes.NewReader(payload))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+service.Lease().Token)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK ||
				!bytes.Contains(body, []byte("invalid_request")) || retriever.calls != 0 {
				t.Fatalf("status=%d body=%s calls=%d", response.StatusCode, body, retriever.calls)
			}
		})
	}
}
