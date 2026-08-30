//go:build unix

package piadapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/permissions"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func TestPiRPCToolExecuteReturnsNativeResultBeforeFinalOutput(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec " + piRPCShellQuote(testBinary) +
		" -test.run '^TestPiRPCToolChildProcess$' -- \"$@\"\n"
	if err := os.WriteFile(fixture.executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	hook := &piToolSuspendingHookFixture{}
	config := fixture.config()
	config.ToolHook = hook
	audits := make(chan PiRPCTranscriptAudit, 1)
	config.TranscriptAudit = audits
	runtimeAdapter, err := NewPiRPCBridgeAdapter(config)
	if err != nil {
		t.Fatal(err)
	}
	request := piRPCToolExecuteRequest(t, fixture)
	result, err := runtimeAdapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if hook.calls != 2 || hook.acked != 1 ||
		hook.proof != attemptpayload.ProofHarnessFinalOutput {
		t.Fatalf("hook calls=%d acked=%d proof=%q", hook.calls, hook.acked, hook.proof)
	}
	var answer []byte
	for _, frame := range result.InboundFrames() {
		if bytes.Contains(frame.Payload(), []byte("private-tool-command")) {
			t.Fatal("tool command leaked to an authorized Bridge frame")
		}
		if frame.Type() == bridgev1.MessageEvent {
			var payload struct {
				Delta string `json:"delta"`
			}
			if json.Unmarshal(frame.Payload(), &payload) != nil {
				t.Fatal("invalid assistant event")
			}
			answer = append(answer, payload.Delta...)
		}
	}
	if string(answer) != "Used governed tool result." {
		t.Fatalf("final answer = %q", answer)
	}
	audit := <-audits
	if len(audit.ToolCallResults) != 1 ||
		audit.ToolCallResults[0].Tool != "Bash" ||
		audit.ToolCallResults[0].ExecutionID != "execution-tool-1" ||
		audit.ToolCallResults[0].ResultNote != "completed" {
		t.Fatalf("content-free tool audit = %#v", audit.ToolCallResults)
	}
	for _, root := range []string{fixture.homePath, fixture.tempPath} {
		entries, readErr := os.ReadDir(root)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, entry := range entries {
			if strings.Contains(entry.Name(), ".loom-tool-") {
				t.Fatalf("private tool extension residue survived: %s", entry.Name())
			}
		}
	}
}

type piSequentialToolHookFixture struct {
	mu        sync.Mutex
	sequences []int64
	acked     []string
	proof     attemptpayload.DeliveryProof
}

func (hook *piSequentialToolHookFixture) ExecuteToolCall(
	ctx context.Context,
	_ ToolCallEnvelope,
	_ ToolCallBinding,
) (ToolCallResult, error) {
	sequence, ok := ToolCallSequence(ctx)
	if !ok || sequence < 1 || sequence > 2 {
		return ToolCallResult{}, ErrPiToolExtension
	}
	hook.mu.Lock()
	hook.sequences = append(hook.sequences, sequence)
	hook.mu.Unlock()
	digestCharacter := "b"
	if sequence == 2 {
		digestCharacter = "c"
	}
	return ToolCallResult{
		Verdict:      permissions.VerdictAllow,
		ExecutionID:  fmt.Sprintf("execution-sequential-%d", sequence),
		ResultNote:   fmt.Sprintf("sequential %d completed", sequence),
		OutputDigest: strings.Repeat(digestCharacter, 64),
		Delivery: &ToolCallDelivery{Binding: attemptpayload.Binding{
			PayloadID: fmt.Sprintf("payload-sequential-%d", sequence),
			Scope: attemptpayload.Scope{
				ConversationID: "conversation-tool-1", WorkItemID: "S5-W2",
				RunID: "run-tool-1", ClaimGeneration: 1,
				RuntimeInstanceID:      "runtime-tool-1",
				ExecutionBindingDigest: strings.Repeat("c", 64),
				CapsuleDigest:          strings.Repeat("d", 64),
			},
			CallID:   fmt.Sprintf("call-sequential-%d", sequence),
			Sequence: sequence, ContentType: "application/json",
			ContentDigest: strings.Repeat("e", 64),
		}},
	}, nil
}

func (hook *piSequentialToolHookFixture) AcknowledgeToolCallResultWithProof(
	_ context.Context,
	_ ToolCallBinding,
	result ToolCallResult,
	proof attemptpayload.DeliveryProof,
) error {
	hook.mu.Lock()
	defer hook.mu.Unlock()
	hook.acked = append(hook.acked, result.ExecutionID)
	hook.proof = proof
	return nil
}

func TestPiRPCSequentialToolExecuteUsesOneManagedChildAndDistinctLineage(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec " + piRPCShellQuote(testBinary) +
		" -test.run '^TestPiRPCSequentialToolChildProcess$' -- \"$@\"\n"
	if err := os.WriteFile(fixture.executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	hook := &piSequentialToolHookFixture{}
	config := fixture.config()
	config.ToolHook = hook
	audits := make(chan PiRPCTranscriptAudit, 1)
	config.TranscriptAudit = audits
	runtimeAdapter, err := NewPiRPCBridgeAdapter(config)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtimeAdapter.Execute(
		context.Background(), piRPCToolExecuteRequest(t, fixture),
	)
	if err != nil {
		t.Fatal(err)
	}
	hook.mu.Lock()
	sequences := append([]int64(nil), hook.sequences...)
	acked := append([]string(nil), hook.acked...)
	proof := hook.proof
	hook.mu.Unlock()
	if !reflect.DeepEqual(sequences, []int64{1, 2}) ||
		!reflect.DeepEqual(acked, []string{"execution-sequential-1", "execution-sequential-2"}) ||
		proof != attemptpayload.ProofHarnessFinalOutput {
		t.Fatalf("sequences=%v acked=%v proof=%q", sequences, acked, proof)
	}
	var answer []byte
	for _, frame := range result.InboundFrames() {
		if bytes.Contains(frame.Payload(), []byte("printf first")) ||
			bytes.Contains(frame.Payload(), []byte("printf second")) ||
			bytes.Contains(frame.Payload(), []byte("payload-sequential")) {
			t.Fatal("sequential ToolCall authority or content leaked to Bridge")
		}
		if frame.Type() == bridgev1.MessageEvent {
			var payload struct {
				Delta string `json:"delta"`
			}
			if json.Unmarshal(frame.Payload(), &payload) != nil {
				t.Fatal("invalid sequential assistant event")
			}
			answer = append(answer, payload.Delta...)
		}
	}
	if string(answer) != "Used governed tool result." {
		t.Fatalf("sequential final answer=%q", answer)
	}
	audit := <-audits
	if len(audit.ToolCallResults) != 2 ||
		audit.ToolCallResults[0].ExecutionID != "execution-sequential-1" ||
		audit.ToolCallResults[1].ExecutionID != "execution-sequential-2" {
		t.Fatalf("sequential audit=%#v", audit.ToolCallResults)
	}
	for _, root := range []string{fixture.homePath, fixture.tempPath} {
		entries, readErr := os.ReadDir(root)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, entry := range entries {
			if strings.Contains(entry.Name(), ".loom-tool-") {
				t.Fatalf("sequential Tool extension residue survived: %s", entry.Name())
			}
		}
	}
}

func TestPiRPCReadToolExecuteReturnsPrivateContentToChildOnly(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec " + piRPCShellQuote(testBinary) +
		" -test.run '^TestPiRPCReadToolChildProcess$' -- \"$@\"\n"
	if err := os.WriteFile(fixture.executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	privateContent := []byte("private source delivered only to Pi child\n")
	digest := "sha256:" + piContextDigest(privateContent)
	hook := &piToolReadHookFixture{content: privateContent}
	hook.result = ToolCallResult{
		Verdict: permissions.VerdictAllow, ExecutionID: "execution-read-child-1",
		ResultNote: "read completed", OutputDigest: digest,
		Delivery: &ToolCallDelivery{Binding: attemptpayload.Binding{
			PayloadID: "payload-read-child-1",
			Scope: attemptpayload.Scope{
				ConversationID: "conversation-tool-1", WorkItemID: "S5-W2",
				RunID: "run-tool-1", ClaimGeneration: 1,
				RuntimeInstanceID:      "runtime-tool-1",
				ExecutionBindingDigest: strings.Repeat("c", 64),
				CapsuleDigest:          strings.Repeat("d", 64),
			},
			CallID: "call-read-child-1", Sequence: 1,
			ContentType:   attemptpayload.ContentTypeTextUTF8,
			ContentDigest: strings.TrimPrefix(digest, "sha256:"),
		}},
	}
	config := fixture.config()
	config.ToolHook = hook
	audits := make(chan PiRPCTranscriptAudit, 1)
	config.TranscriptAudit = audits
	runtimeAdapter, err := NewPiRPCBridgeAdapter(config)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtimeAdapter.Execute(
		context.Background(), piRPCToolExecuteRequest(t, fixture),
	)
	if err != nil {
		t.Fatal(err)
	}
	if hook.acked != 1 || hook.proof != attemptpayload.ProofHarnessFinalOutput {
		t.Fatalf("Read ack=%d proof=%q", hook.acked, hook.proof)
	}
	var answer []byte
	for _, frame := range result.InboundFrames() {
		if bytes.Contains(frame.Payload(), privateContent) ||
			bytes.Contains(frame.Payload(), []byte("src/private.go")) {
			t.Fatal("Read content or path leaked to an authorized Bridge frame")
		}
		if frame.Type() == bridgev1.MessageEvent {
			var payload struct {
				Delta string `json:"delta"`
			}
			if json.Unmarshal(frame.Payload(), &payload) != nil {
				t.Fatal("invalid assistant event")
			}
			answer = append(answer, payload.Delta...)
		}
	}
	if string(answer) != "Used governed tool result." {
		t.Fatalf("Read final answer=%q", answer)
	}
	audit := <-audits
	if len(audit.ToolCallResults) != 1 ||
		audit.ToolCallResults[0].Tool != string(permissions.ToolRead) ||
		audit.ToolCallResults[0].ExecutionID != "execution-read-child-1" ||
		audit.ToolCallResults[0].ResultNote != "read completed" {
		t.Fatalf("content-free Read audit=%#v", audit.ToolCallResults)
	}
}

func TestPiRPCToolAttemptCancellationReapsProcessAndExtensionResources(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	fallbackRoot := filepath.Join(
		"/tmp", ".loom-tool-45454545-4545-4545-8545-454545454545",
	)
	if err := os.RemoveAll(fallbackRoot); err != nil {
		t.Fatalf("clear test-owned fallback Tool root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(fallbackRoot)
	})
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(fixture.tempPath, "tool-child-started")
	script := "#!/bin/sh\nexec " + piRPCShellQuote(testBinary) +
		" -test.run '^TestPiRPCToolCancellationChildProcess$' -- \"$@\" " +
		"--loom-cancel-marker " + piRPCShellQuote(marker) + "\n"
	if err := os.WriteFile(fixture.executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	hook := &piToolPendingHookFixture{called: make(chan struct{})}
	config := fixture.config()
	config.ToolHook = hook
	runtimeAdapter, err := NewPiRPCBridgeAdapter(config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	request := piRPCToolExecuteRequest(t, fixture)
	answer := make(chan error, 1)
	go func() {
		_, executeErr := runtimeAdapter.Execute(ctx, request)
		answer <- executeErr
	}()
	select {
	case <-hook.called:
	case executeErr := <-answer:
		t.Fatalf("Pi child exited before entering the governed Tool channel: %v", executeErr)
	case <-time.After(piFixtureProcessStartupTimeout):
		t.Fatal("Pi child did not enter the governed Tool channel")
	}
	waitForFixtureReady(marker)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("Pi child start marker: %v", err)
	}
	cancel()
	select {
	case err := <-answer:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrPiRPCProtocol) {
			t.Fatalf("Attempt cancellation error=%v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Attempt cancellation did not reap the Pi process")
	}
	for _, root := range []string{fixture.homePath, fixture.tempPath} {
		entries, readErr := os.ReadDir(root)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, entry := range entries {
			if strings.Contains(entry.Name(), ".loom-tool-") ||
				strings.HasSuffix(entry.Name(), ".sock") {
				t.Fatalf("Attempt cleanup residue in %s: %s", root, entry.Name())
			}
		}
	}
	if _, err := os.Lstat(fallbackRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fallback Tool socket root survived: %v", err)
	}
}

func TestPiRPCToolChildProcess(t *testing.T) {
	extensionPath := piRPCTestExtensionArgument(os.Args, "loom-tool.mjs")
	if extensionPath == "" {
		t.Skip("Pi tool child helper is not active")
	}
	source, err := os.ReadFile(extensionPath)
	if err != nil {
		os.Exit(81)
	}
	socketPath, ok := piRPCTestSourceConstant(source, "socketPath")
	capability, capabilityOK := piRPCTestSourceConstant(source, "capability")
	zeroPiRPCBytes(source)
	if !ok || !capabilityOK {
		os.Exit(82)
	}
	requestLine, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		os.Exit(83)
	}
	var promptRequest struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if json.Unmarshal(requestLine, &promptRequest) != nil ||
		promptRequest.Type != "prompt" || promptRequest.ID == "" || promptRequest.Message == "" {
		zeroPiRPCBytes(requestLine)
		os.Exit(84)
	}
	zeroPiRPCBytes(requestLine)
	lines, envelope, wantResult := piRPCToolFixtureLinesForPrompt(
		t, promptRequest.ID, promptRequest.Message, "S5-W2",
	)
	connection, err := net.DialTimeout("unix", socketPath, 3*time.Second)
	if err != nil {
		os.Exit(85)
	}
	payload, err := json.Marshal(piToolExtensionRequest{
		SchemaVersion: 1, Capability: capability, Envelope: envelope,
	})
	if err != nil {
		os.Exit(86)
	}
	if _, err := connection.Write(append(payload, '\n')); err != nil {
		zeroPiRPCBytes(payload)
		os.Exit(87)
	}
	zeroPiRPCBytes(payload)
	response, err := bufio.NewReader(connection).ReadBytes('\n')
	_ = connection.Close()
	if err != nil || bytes.Contains(response, []byte(envelope.Call.Command)) {
		zeroPiRPCBytes(response)
		os.Exit(88)
	}
	_, gotResult, err := decodePiToolExtensionResponse(bytes.TrimSuffix(response, []byte{'\n'}))
	zeroPiRPCBytes(response)
	if err != nil || !samePiToolResult(gotResult, wantResult) {
		os.Exit(89)
	}
	for _, line := range lines {
		_, _ = fmt.Fprintln(os.Stdout, line)
	}
	os.Exit(0)
}

func TestPiRPCSequentialToolChildProcess(t *testing.T) {
	extensionPath := piRPCTestExtensionArgument(os.Args, "loom-tool.mjs")
	if extensionPath == "" {
		t.Skip("Pi sequential tool child helper is not active")
	}
	source, err := os.ReadFile(extensionPath)
	if err != nil {
		os.Exit(121)
	}
	socketPath, socketOK := piRPCTestSourceConstant(source, "socketPath")
	capability, capabilityOK := piRPCTestSourceConstant(source, "capability")
	zeroPiRPCBytes(source)
	if !socketOK || !capabilityOK {
		os.Exit(122)
	}
	requestLine, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		os.Exit(123)
	}
	var promptRequest struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if json.Unmarshal(requestLine, &promptRequest) != nil ||
		promptRequest.Type != "prompt" || promptRequest.ID == "" ||
		promptRequest.Message == "" {
		zeroPiRPCBytes(requestLine)
		os.Exit(124)
	}
	zeroPiRPCBytes(requestLine)
	first := ToolCallEnvelope{
		JobID: "S5-W2",
		Call:  permissions.ProposedCall{Tool: permissions.ToolBash, Command: "printf first"},
	}
	second := ToolCallEnvelope{
		JobID: "S5-W2",
		Call:  permissions.ProposedCall{Tool: permissions.ToolBash, Command: "printf second"},
	}
	results := make([]ToolCallResult, 0, 2)
	for _, envelope := range []ToolCallEnvelope{first, second} {
		connection, dialErr := net.DialTimeout("unix", socketPath, 3*time.Second)
		if dialErr != nil {
			os.Exit(125)
		}
		payload, marshalErr := json.Marshal(piToolExtensionRequest{
			SchemaVersion: 1, Capability: capability, Envelope: envelope,
		})
		if marshalErr != nil {
			_ = connection.Close()
			os.Exit(126)
		}
		if _, writeErr := connection.Write(append(payload, '\n')); writeErr != nil {
			zeroPiRPCBytes(payload)
			_ = connection.Close()
			os.Exit(127)
		}
		zeroPiRPCBytes(payload)
		response, readErr := bufio.NewReader(connection).ReadBytes('\n')
		_ = connection.Close()
		if readErr != nil || bytes.Contains(response, []byte(envelope.Call.Command)) ||
			bytes.Contains(response, []byte("payload-sequential")) {
			zeroPiRPCBytes(response)
			os.Exit(128)
		}
		_, result, decodeErr := decodePiToolExtensionResponse(
			bytes.TrimSuffix(response, []byte{'\n'}),
		)
		zeroPiRPCBytes(response)
		if decodeErr != nil || result.Verdict != permissions.VerdictAllow {
			os.Exit(129)
		}
		results = append(results, result)
	}
	if results[0].ExecutionID != "execution-sequential-1" ||
		results[1].ExecutionID != "execution-sequential-2" ||
		results[0].OutputDigest == results[1].OutputDigest {
		os.Exit(130)
	}
	lines := piRPCSequentialToolFixtureLinesForPrompt(
		t,
		promptRequest.ID,
		promptRequest.Message,
		first,
		results[0],
		second,
		results[1],
	)
	for _, line := range lines {
		_, _ = fmt.Fprintln(os.Stdout, line)
	}
	os.Exit(0)
}

func TestPiRPCReadToolChildProcess(t *testing.T) {
	extensionPath := piRPCTestExtensionArgument(os.Args, "loom-tool.mjs")
	if extensionPath == "" {
		t.Skip("Pi Read tool child helper is not active")
	}
	source, err := os.ReadFile(extensionPath)
	if err != nil {
		os.Exit(91)
	}
	socketPath, ok := piRPCTestSourceConstant(source, "socketPath")
	capability, capabilityOK := piRPCTestSourceConstant(source, "capability")
	zeroPiRPCBytes(source)
	if !ok || !capabilityOK {
		os.Exit(92)
	}
	requestLine, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		os.Exit(93)
	}
	var promptRequest struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if json.Unmarshal(requestLine, &promptRequest) != nil ||
		promptRequest.Type != "prompt" || promptRequest.ID == "" || promptRequest.Message == "" {
		zeroPiRPCBytes(requestLine)
		os.Exit(94)
	}
	zeroPiRPCBytes(requestLine)
	envelope := ToolCallEnvelope{
		JobID: "S5-W2",
		Call: permissions.ProposedCall{
			Tool: permissions.ToolRead, Path: "src/private.go",
		},
	}
	connection, err := net.DialTimeout("unix", socketPath, 3*time.Second)
	if err != nil {
		os.Exit(95)
	}
	payload, err := json.Marshal(piToolExtensionRequest{
		SchemaVersion: 1, Capability: capability, Envelope: envelope,
	})
	if err != nil {
		os.Exit(96)
	}
	if _, err := connection.Write(append(payload, '\n')); err != nil {
		zeroPiRPCBytes(payload)
		os.Exit(97)
	}
	zeroPiRPCBytes(payload)
	response, err := bufio.NewReader(connection).ReadBytes('\n')
	_ = connection.Close()
	privateContent := []byte("private source delivered only to Pi child\n")
	if err != nil || !bytes.Contains(response, bytes.TrimSuffix(privateContent, []byte{'\n'})) ||
		bytes.Contains(response, []byte("payload-read-child-1")) {
		zeroPiRPCBytes(response)
		os.Exit(98)
	}
	_, gotResult, err := decodePiToolExtensionResponse(
		bytes.TrimSuffix(response, []byte{'\n'}),
	)
	zeroPiRPCBytes(response)
	digest := "sha256:" + piContextDigest(privateContent)
	if err != nil || gotResult.Verdict != permissions.VerdictAllow ||
		gotResult.ExecutionID != "execution-read-child-1" ||
		gotResult.OutputDigest != digest || gotResult.ContentDigest != digest {
		os.Exit(99)
	}
	lines := piRPCToolFixtureLinesWithResult(
		t, promptRequest.ID, promptRequest.Message, envelope, gotResult, privateContent,
	)
	zeroPiRPCBytes(privateContent)
	for _, line := range lines {
		_, _ = fmt.Fprintln(os.Stdout, line)
	}
	os.Exit(0)
}

func TestPiRPCToolCancellationChildProcess(t *testing.T) {
	extensionPath := piRPCTestExtensionArgument(os.Args, "loom-tool.mjs")
	marker := piRPCTestArgumentValue(os.Args, "--loom-cancel-marker")
	if extensionPath == "" || marker == "" {
		t.Skip("Pi tool cancellation child helper is not active")
	}
	source, err := os.ReadFile(extensionPath)
	if err != nil {
		os.Exit(101)
	}
	socketPath, socketOK := piRPCTestSourceConstant(source, "socketPath")
	capability, capabilityOK := piRPCTestSourceConstant(source, "capability")
	zeroPiRPCBytes(source)
	if !socketOK || !capabilityOK {
		os.Exit(102)
	}
	requestLine, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		os.Exit(103)
	}
	var promptRequest struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if json.Unmarshal(requestLine, &promptRequest) != nil ||
		promptRequest.Type != "prompt" || promptRequest.ID == "" ||
		promptRequest.Message == "" {
		zeroPiRPCBytes(requestLine)
		os.Exit(104)
	}
	zeroPiRPCBytes(requestLine)
	connection, err := net.DialTimeout("unix", socketPath, 3*time.Second)
	if err != nil {
		os.Exit(105)
	}
	payload, err := json.Marshal(piToolExtensionRequest{
		SchemaVersion: 1,
		Capability:    capability,
		Envelope: ToolCallEnvelope{
			JobID: "S5-W2",
			Call: permissions.ProposedCall{
				Tool: permissions.ToolBash, Command: "printf private-cancelled-tool",
			},
		},
	})
	if err != nil {
		os.Exit(106)
	}
	if _, err := connection.Write(append(payload, '\n')); err != nil {
		zeroPiRPCBytes(payload)
		os.Exit(107)
	}
	zeroPiRPCBytes(payload)
	if err := os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		os.Exit(108)
	}
	toolResponse := make(chan []byte, 1)
	go func() {
		response, _ := bufio.NewReader(connection).ReadBytes('\n')
		_ = connection.Close()
		toolResponse <- response
	}()
	abortLine, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		os.Exit(109)
	}
	var abort struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if json.Unmarshal(abortLine, &abort) != nil || abort.ID == "" || abort.Type != "abort" {
		zeroPiRPCBytes(abortLine)
		os.Exit(110)
	}
	zeroPiRPCBytes(abortLine)
	ack, err := json.Marshal(struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Command string `json:"command"`
		Success bool   `json:"success"`
	}{abort.ID, "response", "abort", true})
	if err != nil {
		os.Exit(111)
	}
	_, _ = fmt.Fprintln(os.Stdout, string(ack))
	zeroPiRPCBytes(ack)
	select {
	case response := <-toolResponse:
		if !bytes.Contains(response, []byte(`"status":"denied"`)) ||
			bytes.Contains(response, []byte("private-cancelled-tool")) {
			zeroPiRPCBytes(response)
			os.Exit(112)
		}
		zeroPiRPCBytes(response)
	case <-time.After(3 * time.Second):
		os.Exit(113)
	}
	time.Sleep(30 * time.Second)
}

func piRPCTestArgumentValue(arguments []string, name string) string {
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == name {
			return arguments[index+1]
		}
	}
	return ""
}

func piRPCToolExecuteRequest(
	t testing.TB,
	fixture *piRPCBridgeFixture,
) supervisor.AdapterRequest {
	t.Helper()
	request := fixture.request(t)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "profile-pi-tool", AdapterType: "pi-cli",
		ProviderID: piRPCProviderID, ModelID: piRPCModelID,
		AuthMode: loomruntime.AuthNative, Timeout: time.Minute,
		RequiredCapabilities: []string{loomruntime.CapabilityGovernedToolLoop},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: fixture.binding.RuntimeInstanceID, DeviceID: "device.local",
		AdapterType: "pi-cli", DisplayName: "Pi governed tool fixture",
		ExecutableVersion: "0.82.1", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{loomruntime.CapabilityGovernedToolLoop}, Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	request.ExecutionBinding, err = loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "conversation-tool-1", TeamID: "team-tool-1",
			AgentID: fixture.binding.SenderAgentInstanceID, RoleID: "coder",
			ProviderID: piRPCProviderID, ModelID: piRPCModelID,
			AuthMode: string(loomruntime.AuthNative), ContextAdapterID: "context:pi-cli:v1",
			DisclosurePolicyID: "policy.local", DisclosurePolicyVersion: 1,
			TokenBudget: 4,
		},
		[]contextcapsule.ItemInput{{
			ItemID: "goal", Kind: contextcapsule.KindConversationGoal,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			Priority: contextcapsule.PrioritySystem, TokenCount: 4, Required: true,
			Content:    []byte("Use the governed tool once."),
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
	request.Dispatch = piTestFrame(t, fixture.binding, 1, bridgev1.MessageDispatch, payload)
	request.ContextCapsule = capsule.AuthorityRecord()
	request.ClaimID = "claim-tool-1"
	request.IncidentID = request.Dispatch.CorrelationID()
	return request
}

func piRPCTestExtensionArgument(arguments []string, base string) string {
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == "--extension" && filepath.Base(arguments[index+1]) == base {
			return arguments[index+1]
		}
	}
	return ""
}

func piRPCTestSourceConstant(source []byte, name string) (string, bool) {
	prefix := []byte("const " + name + " = ")
	for _, line := range bytes.Split(source, []byte{'\n'}) {
		if !bytes.HasPrefix(line, prefix) || !bytes.HasSuffix(line, []byte{';'}) {
			continue
		}
		value, err := strconv.Unquote(string(line[len(prefix) : len(line)-1]))
		return value, err == nil
	}
	return "", false
}
