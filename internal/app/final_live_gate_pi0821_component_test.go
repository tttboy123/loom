//go:build unix

package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"

	_ "modernc.org/sqlite"
)

const (
	pi0821OutputBudget = 256
)

type pi0821LoopbackObservation struct {
	mu           sync.Mutex
	requests     int
	valid        bool
	reason       string
	outputBudget int
}

type pi0821RecordingAdapter struct {
	delegate supervisor.RuntimeAdapter
	mu       sync.Mutex
	err      error
	calls    int
}

func (adapter *pi0821RecordingAdapter) AdapterType() string {
	return adapter.delegate.AdapterType()
}

func (adapter *pi0821RecordingAdapter) RuntimeInstanceID() string {
	return adapter.delegate.RuntimeInstanceID()
}

func (adapter *pi0821RecordingAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	result, err := adapter.delegate.Execute(ctx, request)
	adapter.mu.Lock()
	adapter.err = err
	adapter.calls++
	adapter.mu.Unlock()
	return result, err
}

func (adapter *pi0821RecordingAdapter) failureCode() string {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	switch {
	case adapter.err == nil:
		return ""
	case errors.Is(adapter.err, piadapter.ErrPiRPCOutputTooLarge):
		return "output_too_large"
	case errors.Is(adapter.err, piadapter.ErrPiRPCCleanup):
		return "cleanup"
	case errors.Is(adapter.err, piadapter.ErrPiRPCProtocol):
		return adapter.err.Error()
	default:
		return "runtime_adapter"
	}
}

func (adapter *pi0821RecordingAdapter) callCount() int {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	return adapter.calls
}

type pi0821RejectingObserver struct {
	mu             sync.Mutex
	rejectedEvents int
	acceptedOutput int
}

func (observer *pi0821RejectingObserver) ObserveNodeOutput(
	_ context.Context,
	output NodeOutput,
) error {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if output.AuthorizedFrame().Frame().Type() == bridgev1.MessageEvent {
		observer.rejectedEvents++
		return errors.New("controlled observer rejection")
	}
	if output.AuthorizedFrame().Frame().Type() == bridgev1.MessageEvidence ||
		output.AuthorizedFrame().Frame().Type() == bridgev1.MessageResult {
		observer.acceptedOutput++
	}
	return nil
}

func (observer *pi0821RejectingObserver) counts() (int, int) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return observer.rejectedEvents, observer.acceptedOutput
}

func (observation *pi0821LoopbackObservation) record(
	valid bool,
	reason string,
	outputBudget int,
) {
	observation.mu.Lock()
	defer observation.mu.Unlock()
	observation.requests++
	observation.valid = observation.valid || valid
	observation.outputBudget = outputBudget
	if !valid && observation.reason == "" {
		observation.reason = reason
	}
}

func (observation *pi0821LoopbackObservation) snapshot() (
	int,
	bool,
	string,
	int,
) {
	observation.mu.Lock()
	defer observation.mu.Unlock()
	return observation.requests,
		observation.valid,
		observation.reason,
		observation.outputBudget
}

func TestPi0821DeterministicSSETeamExecutionClosure(t *testing.T) {
	runPi0821DeterministicSSETeamExecution(t, false)
}

func TestPi0821DeterministicSSETeamExecutionFailureClosure(t *testing.T) {
	runPi0821DeterministicSSETeamExecution(t, true)
}

func runPi0821DeterministicSSETeamExecution(
	t *testing.T,
	rejectOutput bool,
) {
	if os.Getenv("LOOM_PI_0821_COMPONENT") != "1" {
		t.Skip("opt-in Pi 0.82.1 component is disabled")
	}
	piExecutable := os.Getenv("LOOM_PI_0821_EXECUTABLE")
	searchPaths := filepath.SplitList(os.Getenv("LOOM_PI_0821_RUNTIME_SEARCH_PATH"))
	if os.Getenv("LOOM_PI_0821_RUNTIME_INSTANCE_ID") != finalLiveRuntimeID ||
		piExecutable == "" ||
		!filepath.IsAbs(piExecutable) ||
		filepath.Clean(piExecutable) != piExecutable ||
		len(searchPaths) == 0 {
		t.Fatal("locked Pi component binding is absent")
	}
	if _, err := finalLiveBoundFileDigest(
		piExecutable,
		finalLiveInstalledPiSHA256,
	); err != nil {
		t.Fatal("locked Pi executable binding failed")
	}
	if err := bindFinalLivePi0821Sources(piExecutable); err != nil {
		t.Fatal("locked Pi source binding failed")
	}

	firstDelta := string([]byte{0x61, 0x62, 0x63})
	secondDelta := string([]byte{0x64, 0x65, 0x66})
	wantOutput := []byte(firstDelta + secondDelta)
	observation := &pi0821LoopbackObservation{}
	server := newPi0821LoopbackServer(
		t,
		observation,
		firstDelta,
		secondDelta,
	)
	defer server.Close()

	privateRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	runRoot := finalLivePrivateDirectory(t, privateRoot, "component-attempt")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	metadataRunner, err := piadapter.NewPiMetadataProcessRunner(
		piadapter.PiMetadataProcessRunnerConfig{
			ExecutablePath:     piExecutable,
			IsolationRoot:      finalLivePrivateDirectory(t, runRoot, "metadata"),
			RuntimeSearchPaths: searchPaths,
			Timeout:            15 * time.Second,
		},
	)
	if err != nil {
		t.Fatal("Pi metadata runner construction failed")
	}
	probe, err := loomruntime.NewPiRuntimeProbe(loomruntime.PiRuntimeProbeConfig{
		ProbeID:     "probe.pi.component",
		InstanceID:  finalLiveRuntimeID,
		DeviceID:    "device.local",
		DisplayName: "Pi 0.82.1 Component",
		Runner:      metadataRunner,
	})
	if err != nil {
		t.Fatal("Pi Runtime probe construction failed")
	}
	observations, err := probe.ObserveRuntime(ctx)
	if err != nil ||
		len(observations) != 1 ||
		observations[0].Instance.ID != finalLiveRuntimeID ||
		observations[0].Instance.Status != loomruntime.RuntimeOnline {
		t.Fatal("locked Pi Runtime discovery failed")
	}

	transcriptAudit := make(chan piadapter.PiRPCTranscriptAudit, 1)
	adapter, err := piadapter.NewPiRPCBridgeAdapter(
		piadapter.PiRPCBridgeAdapterConfig{
			Execution: piadapter.PiExecutionAdapterConfig{
				ExecutablePath:     piExecutable,
				RuntimeInstanceID:  finalLiveRuntimeID,
				RuntimeSearchPaths: searchPaths,
				CancelGrace:        3 * time.Second,
				Now:                func() time.Time { return time.Now().UTC() },
				Random:             rand.Reader,
			},
			ProviderID:        "loom-local",
			ModelID:           "qwen2.5-coder-1.5b-instruct-q4-k-m",
			BaseURL:           server.URL + "/v1",
			MaxAssistantBytes: 16384,
			TranscriptAudit:   transcriptAudit,
		},
	)
	if err != nil {
		t.Fatal("Pi RPC Bridge adapter construction failed")
	}
	recordingAdapter := &pi0821RecordingAdapter{delegate: adapter}

	databasePath, err := createFinalLivePrivateSQLite(runRoot)
	if err != nil {
		t.Fatal("cannot create component database")
	}
	db, err := sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := journal.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	now := time.Now().UTC()
	seedFinalLiveRuntime(t, store, now)
	workAuthority, err := work.NewAuthority(
		store,
		func() time.Time { return now },
		rand.Reader,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	grantAuthority, err := authorization.NewAuthority(
		store,
		workAuthority,
		func() time.Time { return now },
		rand.Reader,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	artifactStore, err := evidence.NewStore(filepath.Join(runRoot, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	defer artifactStore.Close()
	coordinator, err := NewTeamCoordinator(
		workAuthority,
		grantAuthority,
		readModel,
		artifactStore,
	)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-pi-0821-component",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID:     "main",
			Title:             "Bounded loopback closure",
			AgentInstanceID:   "agent-main-pi-0821-component",
			RuntimeInstanceID: finalLiveRuntimeID,
			Role:              teams.ExecutionRoleMain,
			MaxAttempts:       1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	workItemID := appTeamAttemptIdentity("work", plan, "main", 1)
	runID := appTeamAttemptIdentity("run", plan, "main", 1)
	promptPayload, err := json.Marshal(struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Prompt        string `json:"prompt"`
	}{
		SchemaVersion: 1,
		Kind:          "pi_rpc_prompt",
		Prompt:        finalLivePrompt,
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             "81000000-0000-4000-8000-000000000001",
		CorrelationID:         "82000000-0000-4000-8000-000000000001",
		WorkItemID:            workItemID,
		RunID:                 runID,
		ClaimGeneration:       1,
		RuntimeInstanceID:     finalLiveRuntimeID,
		SenderAgentInstanceID: "agent-main-pi-0821-component",
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt:             now,
		Payload:               promptPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:          "profile-pi-0821-component",
		AdapterType: "pi-cli",
		AuthMode:    loomruntime.AuthBrokered,
		Timeout:     30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID:                finalLiveRuntimeID,
		DeviceID:          "device.local",
		AdapterType:       "pi-cli",
		DisplayName:       "Pi 0.82.1 Component",
		ExecutableVersion: "0.82.1",
		Status:            loomruntime.RuntimeOnline,
		Capacity:          1,
	})
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := finalLivePrivateDirectory(t, runRoot, "source")
	if err := os.WriteFile(
		filepath.Join(sourcePath, "bounded.go"),
		[]byte("package bounded\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	successObserver := &finalLiveOutputObserver{}
	failureObserver := &pi0821RejectingObserver{}
	var outputObserver NodeOutputObserver = successObserver
	if rejectOutput {
		outputObserver = failureObserver
	}
	result, err := coordinator.Run(ctx, TeamExecutionRequest{
		Plan: plan,
		Nodes: []TeamNodeExecution{{
			LogicalNodeID: "main",
			AttemptNumber: 1,
			WorkflowPath:  "primary",
			SourcePath:    sourcePath,
			Profile:       profile,
			Instance:      instance,
			Dispatch:      dispatch,
			Executor: newTeamCanarySupervisor(
				t,
				workAuthority,
				grantAuthority,
				recordingAdapter,
			),
		}},
		Semantics:            testTeamNodeSemantics(t, plan, time.Second, ""),
		AuthoritativeTime:    now,
		PrepareLeaseDuration: time.Minute,
		GrantLifetime:        2 * time.Minute,
		CorrelationID:        dispatch.CorrelationID(),
		OutputObserver:       outputObserver,
	})
	if rejectOutput {
		if err != nil ||
			result.Team().Status() != "blocked" ||
			len(result.ExecutedNodeIDs()) != 1 ||
			result.ExecutedNodeIDs()[0] != "main" {
			t.Fatalf(
				"component failure closure status=%q err=%v",
				result.Team().Status(),
				err,
			)
		}
		requests, validRequest, _, outputBudget := observation.snapshot()
		rejectedEvents, acceptedOutput := failureObserver.counts()
		if requests != 1 ||
			!validRequest ||
			outputBudget != pi0821OutputBudget ||
			recordingAdapter.callCount() != 1 ||
			rejectedEvents != 1 ||
			acceptedOutput != 0 {
			t.Fatal("component observer failure accounting did not close")
		}
		select {
		case <-transcriptAudit:
			t.Fatal("failed AdapterResult published a transcript audit")
		default:
		}
		assertPi0821ComponentEvidence(
			t,
			ctx,
			artifactStore,
			runRoot,
			result.Team(),
			false,
			nil,
		)
		assertPi0821ComponentJournal(
			t,
			ctx,
			store,
			privateRoot,
			nil,
			false,
		)
		if err := readModel.Rebuild(ctx); err != nil {
			t.Fatal(err)
		}
		projected, ok := readModel.GlobalReadView().TeamExecution(
			plan.TeamInstanceID(),
		)
		if !ok || projected.Status != "blocked" {
			t.Fatal("failed component Team terminal was not projected")
		}
		return
	}
	if err != nil ||
		result.Team().Status() != "succeeded" ||
		len(result.ExecutedNodeIDs()) != 1 ||
		result.ExecutedNodeIDs()[0] != "main" {
		requests, validRequest, reason, outputBudget := observation.snapshot()
		nodeStatus := ""
		attemptStatus := ""
		classification := ""
		recoveryTrigger := ""
		terminalReason := ""
		nodes := result.Team().Nodes()
		if len(nodes) == 1 {
			nodeStatus = nodes[0].Status()
			recoveryTrigger = nodes[0].RecoveryTrigger()
			attempts := nodes[0].Attempts()
			if len(attempts) == 1 {
				attemptStatus = attempts[0].Status()
				classification = string(attempts[0].OutputClassification())
			}
		}
		auditPublished := false
		select {
		case <-transcriptAudit:
			auditPublished = true
		default:
		}
		if snapshot, snapshotErr := workAuthority.Snapshot(ctx); snapshotErr == nil {
			for _, run := range snapshot.Runs() {
				if run.ID() == runID {
					terminalReason = run.TerminalReason()
				}
			}
		}
		t.Fatalf(
			"component Team execution failed: status=%q node=%q attempt=%q classification=%q recovery=%q terminal_reason=%q adapter=%q audit=%t requests=%d valid=%t reason=%s output_budget=%d err=%v",
			result.Team().Status(),
			nodeStatus,
			attemptStatus,
			classification,
			recoveryTrigger,
			terminalReason,
			recordingAdapter.failureCode(),
			auditPublished,
			requests,
			validRequest,
			reason,
			outputBudget,
			err,
		)
	}
	requests, validRequest, _, outputBudget := observation.snapshot()
	if requests != 1 ||
		!validRequest ||
		outputBudget != pi0821OutputBudget {
		t.Fatal("loopback SSE request accounting failed")
	}
	successFrames := assertPi0821ComponentFrames(
		t,
		successObserver,
		wantOutput,
	)
	select {
	case audit := <-transcriptAudit:
		if !audit.ForwardPartialObserved ||
			audit.TextStartSnapshotBytes <= audit.AcceptedBytesAtTextStart ||
			audit.AcceptedBytesAtTextStart != 0 ||
			audit.AcceptedDeltaCount < 2 ||
			audit.FinalSnapshotBytes != audit.FinalAcceptedDeltaBytes ||
			!audit.DeltaClosure {
			t.Fatal("sanitized forward-partial audit did not close")
		}
	default:
		t.Fatal("sanitized forward-partial audit was not published")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	projected, ok := readModel.GlobalReadView().TeamExecution(plan.TeamInstanceID())
	if !ok || projected.Status != "succeeded" {
		t.Fatal("component Team terminal was not projected")
	}
	assertPi0821ComponentEvidence(
		t,
		ctx,
		artifactStore,
		runRoot,
		result.Team(),
		true,
		wantOutput,
		successFrames,
	)
	assertPi0821ComponentJournal(
		t,
		ctx,
		store,
		privateRoot,
		wantOutput,
		true,
	)
	t.Log("component_proof locked_pi=true forward_partial=true sse_requests=1 team_terminal=succeeded evidence=1")
}

func newPi0821LoopbackServer(
	t testing.TB,
	observation *pi0821LoopbackObservation,
	firstDelta string,
	secondDelta string,
) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		valid, reason, outputBudget := validPi0821LoopbackRequest(request)
		observation.record(valid, reason, outputBudget)
		if !valid {
			http.Error(writer, "invalid", http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		writer.WriteHeader(http.StatusOK)
		flusher, ok := writer.(http.Flusher)
		if !ok {
			t.Error("loopback response is not flushable")
			return
		}
		finishReason := "length"
		if outputBudget == pi0821OutputBudget {
			finishReason = "stop"
		}
		chunks := []any{
			pi0821SSERoleChunk(),
			pi0821SSEContentChunk(firstDelta),
			pi0821SSEContentChunk(secondDelta),
			pi0821SSEFinishChunk(finishReason),
			pi0821SSEUsageChunk(),
		}
		for _, chunk := range chunks {
			content, err := json.Marshal(chunk)
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := writer.Write(append(append([]byte("data: "), content...), '\n', '\n')); err != nil {
				return
			}
			flusher.Flush()
		}
		_, _ = writer.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	})
	server := httptest.NewUnstartedServer(handler)
	address, ok := server.Listener.Addr().(*net.TCPAddr)
	if !ok || !address.IP.IsLoopback() {
		t.Fatal("loopback server did not bind loopback")
	}
	server.Start()
	return server
}

func validPi0821LoopbackRequest(request *http.Request) (
	bool,
	string,
	int,
) {
	if request.RemoteAddr == "" {
		return false, "remote", 0
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return false, "remote", 0
	}
	if request.Method != http.MethodPost {
		return false, "method", 0
	}
	if request.URL.Path != "/v1/chat/completions" {
		return false, "path", 0
	}
	if request.Header.Get("Content-Type") != "application/json" {
		return false, "content_type", 0
	}
	if request.Header.Get("Authorization") != "Bearer loom-local-offline" {
		return false, "auth", 0
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, 64<<10))
	if err != nil || len(body) == 0 || len(body) >= 64<<10 {
		return false, "body", 0
	}
	var value struct {
		Model               string `json:"model"`
		Stream              bool   `json:"stream"`
		MaxTokens           *int   `json:"max_tokens"`
		MaxCompletionTokens *int   `json:"max_completion_tokens"`
		Messages            []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		Tools json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		return false, "json", 0
	}
	if value.Model != "qwen2.5-coder-1.5b-instruct-q4-k-m" {
		return false, "model", 0
	}
	if !value.Stream {
		return false, "stream", 0
	}
	if len(value.Messages) != 2 {
		return false, "messages", 0
	}
	if value.Messages[0].Role != "system" || value.Messages[1].Role != "user" {
		return false, "roles", 0
	}
	userText, ok := pi0821RequestText(value.Messages[1].Content)
	if !ok || userText != finalLivePrompt {
		return false, "prompt", 0
	}
	if !pi0821NoTools(value.Tools) {
		return false, "tools", 0
	}
	outputBudget := 0
	switch {
	case value.MaxTokens != nil && value.MaxCompletionTokens == nil:
		outputBudget = *value.MaxTokens
	case value.MaxTokens == nil && value.MaxCompletionTokens != nil:
		outputBudget = *value.MaxCompletionTokens
	default:
		return false, "output_budget_field", 0
	}
	if outputBudget < 1 || outputBudget > 4096 {
		return false, "output_budget", outputBudget
	}
	return true, "", outputBudget
}

func pi0821RequestText(raw json.RawMessage) (string, bool) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, true
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil ||
		len(blocks) != 1 ||
		blocks[0].Type != "text" {
		return "", false
	}
	return blocks[0].Text, true
}

func pi0821NoTools(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) == 0 ||
		bytes.Equal(raw, []byte("null")) ||
		bytes.Equal(raw, []byte("[]"))
}

func pi0821SSERoleChunk() any {
	return map[string]any{
		"object":  "chat.completion.chunk",
		"created": 1,
		"model":   "qwen2.5-coder-1.5b-instruct-q4-k-m",
		"choices": []any{map[string]any{
			"index": 0,
			"delta": map[string]any{
				"role": "assistant",
			},
			"finish_reason": nil,
		}},
	}
}

func pi0821SSEContentChunk(delta string) any {
	return struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int    `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index int `json:"index"`
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
			FinishReason any `json:"finish_reason"`
		} `json:"choices"`
	}{
		ID:      "loopback-response",
		Object:  "chat.completion.chunk",
		Created: 1,
		Model:   "qwen2.5-coder-1.5b-instruct-q4-k-m",
		Choices: []struct {
			Index int `json:"index"`
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
			FinishReason any `json:"finish_reason"`
		}{{Index: 0, Delta: struct {
			Content string `json:"content"`
		}{Content: delta}}},
	}
}

func pi0821SSEFinishChunk(reason string) any {
	return map[string]any{
		"id":      "loopback-response",
		"object":  "chat.completion.chunk",
		"created": 1,
		"model":   "qwen2.5-coder-1.5b-instruct-q4-k-m",
		"choices": []any{map[string]any{
			"index":         0,
			"delta":         map[string]any{},
			"finish_reason": reason,
		}},
	}
}

func pi0821SSEUsageChunk() any {
	return map[string]any{
		"id":      "loopback-response",
		"object":  "chat.completion.chunk",
		"created": 1,
		"model":   "qwen2.5-coder-1.5b-instruct-q4-k-m",
		"choices": []any{},
		"usage": map[string]any{
			"prompt_tokens":     3,
			"completion_tokens": 2,
			"total_tokens":      5,
		},
	}
}

func assertPi0821ComponentFrames(
	t testing.TB,
	observer *finalLiveOutputObserver,
	want []byte,
) []bridgev1.Frame {
	t.Helper()
	observer.mu.Lock()
	defer observer.mu.Unlock()
	frames := append([]bridgev1.Frame(nil), observer.frames...)
	wantTypes := []bridgev1.MessageType{
		bridgev1.MessageAck,
		bridgev1.MessageEvent,
		bridgev1.MessageEvent,
		bridgev1.MessageEvidence,
		bridgev1.MessageResult,
	}
	if len(frames) != len(wantTypes) {
		t.Fatalf("authorized Frame count = %d", len(frames))
	}
	var got []byte
	for index, frame := range frames {
		if frame.Type() != wantTypes[index] {
			t.Fatalf(
				"authorized Frame %d type = %q",
				index,
				frame.Type(),
			)
		}
		if index > 0 &&
			frame.Sequence() != frames[index-1].Sequence()+1 {
			t.Fatal("authorized Frame sequence is not contiguous")
		}
		switch frame.Type() {
		case bridgev1.MessageEvent:
			var fields map[string]json.RawMessage
			var payload struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(frame.Payload(), &fields); err != nil ||
				len(fields) != 1 {
				t.Fatal("invalid authorized Event payload")
			}
			if err := json.Unmarshal(frame.Payload(), &payload); err != nil ||
				payload.Delta == "" {
				t.Fatal("invalid authorized Event delta")
			}
			got = append(got, payload.Delta...)
		case bridgev1.MessageEvidence:
			var fields map[string]json.RawMessage
			var payload struct {
				Kind   string `json:"kind"`
				SHA256 string `json:"sha256"`
				Bytes  int    `json:"bytes"`
			}
			if err := json.Unmarshal(frame.Payload(), &fields); err != nil ||
				len(fields) != 3 ||
				json.Unmarshal(frame.Payload(), &payload) != nil ||
				payload.Kind != "assistant_text_digest" ||
				payload.SHA256 != finalLiveTestDigest(want) ||
				payload.Bytes != len(want) {
				t.Fatal("Evidence Frame did not bind exact output digest")
			}
		case bridgev1.MessageResult:
			if !bytes.Equal(
				frame.Payload(),
				[]byte(`{"status":"succeeded","reason":""}`),
			) {
				t.Fatal("Result Frame was not exact succeeded terminal")
			}
		}
	}
	if !bytes.Equal(got, want) {
		t.Fatal("authorized Event bytes did not match ordered SSE deltas")
	}
	return frames
}

func assertPi0821ComponentEvidence(
	t testing.TB,
	ctx context.Context,
	store *evidence.Store,
	runRoot string,
	team work.TeamExecutionRecord,
	succeeded bool,
	wantOutput []byte,
	frames ...[]bridgev1.Frame,
) {
	t.Helper()
	nodes := team.Nodes()
	if len(nodes) != 1 {
		t.Fatal("component Team attempt lineage is not singular")
	}
	attempts := nodes[0].Attempts()
	if len(attempts) != 1 ||
		attempts[0].EvidenceID() == "" ||
		attempts[0].EvidenceDigest() == "" {
		t.Fatal("component Evidence lineage is not singular")
	}
	attempt := attempts[0]
	wantStatus := "failed"
	if succeeded {
		wantStatus = "succeeded"
	}
	if attempt.Status() != wantStatus {
		t.Fatalf("component attempt status = %q", attempt.Status())
	}
	receipt, found, err := store.AttemptReceipt(ctx, attempt.EvidenceID())
	if err != nil || !found ||
		receipt.EvidenceID() != attempt.EvidenceID() ||
		receipt.Digest() != attempt.EvidenceDigest() {
		t.Fatal("component Evidence receipt binding failed")
	}
	capture, found, err := store.AttemptCapture(ctx, attempt.EvidenceID())
	if err != nil || !found {
		t.Fatal("component Evidence capture binding failed")
	}
	binding := capture.Binding()
	if binding.EvidenceID != attempt.EvidenceID() ||
		binding.TeamInstanceID != team.TeamInstanceID() ||
		binding.PlanDigest != team.PlanDigest() ||
		binding.LogicalNodeID != nodes[0].LogicalNodeID() ||
		binding.AttemptNumber != attempt.AttemptNumber() ||
		binding.WorkItemID != attempt.WorkItemID() ||
		binding.RunID != attempt.RunID() ||
		binding.ClaimID != attempt.ClaimID() ||
		binding.ClaimGeneration != attempt.ClaimGeneration() ||
		binding.RuntimeInstanceID != attempt.RuntimeInstanceID() ||
		binding.AgentInstanceID != attempt.AgentInstanceID() {
		t.Fatal("component Evidence capture lineage diverged")
	}
	summary := receipt.OutputSummary()
	if summary.EvidenceID() != receipt.EvidenceID() ||
		summary.EvidenceDigest() != receipt.Digest() ||
		summary.TerminalStatus() != wantStatus {
		t.Fatal("component Evidence summary binding diverged")
	}
	if succeeded {
		if len(frames) != 1 {
			t.Fatal("missing successful authorized Frame proof")
		}
		outputFrames := 0
		outputBytes := 0
		for _, frame := range frames[0] {
			if frame.Type() == bridgev1.MessageEvent ||
				frame.Type() == bridgev1.MessageEvidence {
				outputFrames++
				outputBytes += len(frame.Payload())
			}
		}
		if summary.AuthorizedFrameCount() != len(frames[0]) ||
			summary.OutputFrameCount() != outputFrames ||
			summary.OutputPayloadBytes() != outputBytes ||
			!summary.ResultObserved() ||
			capture.FrameCount() != len(frames[0]) ||
			!capture.ResultObserved() {
			t.Fatal("successful Evidence receipt/capture counts diverged")
		}
	} else {
		firstDeltaPayload, err := json.Marshal(struct {
			Delta string `json:"delta"`
		}{Delta: "abc"})
		if err != nil {
			t.Fatal(err)
		}
		if summary.AuthorizedFrameCount() != 2 ||
			summary.OutputFrameCount() != 1 ||
			summary.OutputPayloadBytes() != len(firstDeltaPayload) ||
			summary.ResultObserved() ||
			capture.FrameCount() != 2 ||
			capture.ResultObserved() {
			t.Fatal("failed Evidence receipt/capture counts diverged")
		}
	}
	artifactPath := filepath.Join(
		runRoot,
		"evidence",
		"artifacts",
		"sha256",
		receipt.Digest()[:2],
		receipt.Digest(),
	)
	info, err := os.Lstat(artifactPath)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 {
		t.Fatal("component immutable Evidence artifact is invalid")
	}
	artifactBytes, err := os.ReadFile(artifactPath)
	if err != nil ||
		finalLiveTestDigest(artifactBytes) != receipt.Digest() {
		t.Fatal("component immutable Evidence artifact digest diverged")
	}
	for _, forbidden := range [][]byte{
		[]byte(runRoot),
		[]byte(finalLivePrompt),
		[]byte("loom_grant_v1."),
		[]byte("loom-local-offline"),
	} {
		if bytes.Contains(artifactBytes, forbidden) {
			t.Fatal("forbidden material entered private component Evidence")
		}
	}
	var artifact struct {
		AuthorizedFrames []string `json:"authorized_frames"`
	}
	if json.Unmarshal(artifactBytes, &artifact) != nil ||
		len(artifact.AuthorizedFrames) != capture.FrameCount() {
		t.Fatal("private component Evidence frame encoding diverged")
	}
	artifactFrames := make([]bridgev1.Frame, 0, len(artifact.AuthorizedFrames))
	for _, line := range artifact.AuthorizedFrames {
		frame, err := bridgev1.DecodeLine([]byte(line))
		if err != nil {
			t.Fatal("private component Evidence contains a non-canonical Frame")
		}
		canonical, err := bridgev1.EncodeLine(frame)
		if err != nil || !bytes.Equal(canonical, []byte(line)) {
			t.Fatal("private component Evidence Frame is not canonical")
		}
		artifactFrames = append(artifactFrames, frame)
	}
	if succeeded {
		if len(frames) != 1 ||
			len(artifactFrames) != len(frames[0]) {
			t.Fatal("successful private Evidence Frame count diverged")
		}
		for index, frame := range frames[0] {
			wantLine, err := bridgev1.EncodeLine(frame)
			if err != nil ||
				!bytes.Equal(
					wantLine,
					[]byte(artifact.AuthorizedFrames[index]),
				) {
				t.Fatal("successful private Evidence Frame bytes diverged")
			}
		}
	} else {
		if len(artifactFrames) != 2 ||
			artifactFrames[0].Type() != bridgev1.MessageAck ||
			artifactFrames[1].Type() != bridgev1.MessageEvent ||
			!bytes.Equal(
				artifactFrames[1].Payload(),
				[]byte(`{"delta":"abc"}`),
			) {
			t.Fatal("failed private Evidence did not close at rejected Event")
		}
	}
}

func assertPi0821ComponentJournal(
	t testing.TB,
	ctx context.Context,
	store *journal.Store,
	privateRoot string,
	wantOutput []byte,
	succeeded bool,
) {
	t.Helper()
	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	eventIDs := make(map[string]struct{})
	streamSequences := make(map[string]struct{})
	for _, event := range events {
		if _, exists := eventIDs[event.ID]; exists {
			t.Fatal("duplicate component Journal Event ID")
		}
		eventIDs[event.ID] = struct{}{}
		streamSequence := event.StreamID + "/" + fmt.Sprint(event.Seq)
		if _, exists := streamSequences[streamSequence]; exists {
			t.Fatal("duplicate component Journal stream sequence")
		}
		streamSequences[streamSequence] = struct{}{}
		if bytes.Contains(event.PayloadJSON, []byte(privateRoot)) ||
			bytes.Contains(event.PayloadJSON, []byte(finalLivePrompt)) ||
			bytes.Contains(event.PayloadJSON, []byte("loom_grant_v1.")) ||
			len(wantOutput) > 0 &&
				bytes.Contains(event.PayloadJSON, wantOutput) {
			t.Fatal("private or tentative component material entered Journal")
		}
		counts[event.Type]++
	}
	required := map[string]int{
		"AgentGrantIssued":        1,
		"AgentGrantRevoked":       1,
		"EvidenceSubmitted":       1,
		"RunTerminalCommitted":    1,
		"RuntimeCapacityReserved": 1,
		"RuntimeCapacityReleased": 1,
		"TeamNodeAttemptTerminal": 1,
		"TeamExecutionTerminal":   1,
	}
	if succeeded {
		required["WorkItemReadyForReview"] = 1
		required["WorkItemVerificationCommitted"] = 1
		required["WorkItemDone"] = 1
		required["TeamNodeAcceptanceCommitted"] = 1
		required["AgentGrantAuthorized"] = 5
	} else {
		required["WorkItemTerminal"] = 1
		required["AgentGrantAuthorized"] = 2
	}
	for eventType, want := range required {
		if counts[eventType] != want {
			t.Fatalf(
				"component Journal %s count = %d, want %d",
				eventType,
				counts[eventType],
				want,
			)
		}
	}
}
