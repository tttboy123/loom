//go:build unix

package piadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const piRPCFixturePrompt = "Correct: func add(a, b int) int { return a - b }"

func TestPiRPCBridgeTranslatesCorrelatedTranscript(t *testing.T) {
	t.Setenv("SHOULD_NOT_LEAK", "ambient-secret")
	fixture := newPiRPCBridgeFixture(t, "success")
	adapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatalf("NewPiRPCBridgeAdapter() error = %v", err)
	}
	if adapter.AdapterType() != "pi-cli" ||
		adapter.RuntimeInstanceID() != fixture.binding.RuntimeInstanceID {
		t.Fatalf("adapter identity = %q/%q", adapter.AdapterType(), adapter.RuntimeInstanceID())
	}

	request := fixture.request(t)
	sink := request.FrameSink.(*piRecordingFrameSink)
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExitCode() != 0 ||
		!result.DispatchAcknowledged() ||
		!result.ResultAcknowledged() ||
		result.CancelAcknowledged() {
		t.Fatalf("adapter result = %#v", result)
	}
	frames := result.InboundFrames()
	if len(frames) != 5 || len(sink.frames) != len(frames) {
		t.Fatalf("frames = %d/%d, want 5 translated frames", len(frames), len(sink.frames))
	}
	wantTypes := []bridgev1.MessageType{
		bridgev1.MessageAck,
		bridgev1.MessageEvent,
		bridgev1.MessageEvent,
		bridgev1.MessageEvidence,
		bridgev1.MessageResult,
	}
	for index, frame := range frames {
		if frame.Type() != wantTypes[index] || frame.Sequence() != int64(index+2) {
			t.Fatalf("frame[%d] = %q seq %d", index, frame.Type(), frame.Sequence())
		}
	}
	if got := string(frames[1].Payload()); got != `{"delta":"Hello "}` {
		t.Fatalf("first delta = %s", got)
	}
	if got := string(frames[2].Payload()); got != `{"delta":"world"}` {
		t.Fatalf("second delta = %s", got)
	}
	if bytes.Contains(mustJSONFrames(t, frames), []byte(piTestTokenValue)) {
		t.Fatal("translated frames leaked the raw Grant")
	}
	modelsPath := filepath.Join(fixture.homePath, ".pi", "agent", "models.json")
	info, err := os.Lstat(modelsPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("models.json binding = %#v, %v", info, err)
	}
}

func TestPiRPCBridgeFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want error
	}{
		{name: "unknown record", mode: "unknown", want: ErrPiRPCProtocol},
		{name: "tool use", mode: "tool", want: ErrPiRPCProtocol},
		{name: "hidden reasoning", mode: "thinking", want: ErrPiRPCProtocol},
		{name: "length completion", mode: "length", want: ErrPiRPCProtocol},
		{name: "invented nested start", mode: "nested-start-event", want: ErrPiRPCProtocol},
		{name: "invented nested done", mode: "nested-done-event", want: ErrPiRPCProtocol},
		{name: "invented nested error", mode: "nested-error-event", want: ErrPiRPCProtocol},
		{name: "top-level partial identity mismatch", mode: "partial-identity-mismatch", want: ErrPiRPCProtocol},
		{name: "response model substitution", mode: "response-model-substitution", want: ErrPiRPCProtocol},
		{name: "retry", mode: "retry", want: ErrPiRPCProtocol},
		{name: "mismatched text", mode: "mismatch", want: ErrPiRPCProtocol},
		{name: "uncorrelated response", mode: "uncorrelated", want: ErrPiRPCProtocol},
		{name: "duplicate key", mode: "duplicate", want: ErrPiRPCProtocol},
		{name: "unknown key", mode: "extra", want: ErrPiRPCProtocol},
		{name: "nested duplicate key", mode: "nested-duplicate", want: ErrPiRPCProtocol},
		{name: "nested unknown key", mode: "nested-extra", want: ErrPiRPCProtocol},
		{name: "error assistant field", mode: "assistant-error", want: ErrPiRPCProtocol},
		{name: "unexpected agent end message", mode: "agent-end-extra", want: ErrPiRPCProtocol},
		{name: "record after settled", mode: "after-settled", want: ErrPiRPCProtocol},
		{name: "carriage return", mode: "carriage-return", want: ErrPiRPCProtocol},
		{name: "unicode separator", mode: "unicode-separator", want: ErrPiRPCProtocol},
		{name: "grant output", mode: "grant", want: ErrPiRPCProtocol},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, test.mode)
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			request := fixture.request(t)
			sink := request.FrameSink.(*piRecordingFrameSink)
			result, err := adapter.Execute(context.Background(), request)
			if !errors.Is(err, test.want) {
				t.Fatalf("Execute() error = %v, want %v", err, test.want)
			}
			if len(result.InboundFrames()) != 0 {
				t.Fatalf("Execute() leaked partial AdapterResult = %#v", result)
			}
			if test.mode == "after-settled" {
				for _, frame := range sink.frames {
					if frame.Type() == bridgev1.MessageEvidence ||
						frame.Type() == bridgev1.MessageResult {
						t.Fatalf("post-settled rejection leaked terminal frame %q", frame.Type())
					}
				}
			}
		})
	}
}

func TestPiRPCBridgeRejectsDispatchBeforeProcessStart(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	tests := [][]byte{
		[]byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"/help"}`),
		[]byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":""}`),
		[]byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"TOKEN=secret"}`),
		[]byte(`{"kind":"pi_rpc_prompt","schema_version":1,"prompt":"valid"}`),
		[]byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"valid","extra":true}`),
		[]byte(`{"schema_version":1,"schema_version":1,"kind":"pi_rpc_prompt","prompt":"valid"}`),
	}
	for index, payload := range tests {
		if _, err := parsePiRPCDispatch(payload, fixture.token.Value()); !errors.Is(err, ErrPiRPCProtocol) {
			t.Fatalf("dispatch[%d] error = %v, want ErrPiRPCProtocol", index, err)
		}
	}
}

func TestPiRPCBridgeCancellationAcknowledgementAndCleanup(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "wait-cancel")
	adapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	answer := make(chan error, 1)
	go func() {
		_, executeErr := adapter.Execute(ctx, fixture.request(t))
		answer <- executeErr
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-answer:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrPiRPCProtocol) {
			t.Fatalf("Execute() cancellation error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Pi RPC cancellation did not clean up within the bound")
	}
}

func TestPiRPCBridgeUTF8ChunkBoundaries(t *testing.T) {
	for offset := 0; offset < 100; offset++ {
		input := []byte(strings.Repeat("界", 1000) + strings.Repeat("x", offset))
		chunks := splitPiRPCDelta(input)
		var rebuilt []byte
		for index, chunk := range chunks {
			if len(chunk) == 0 || len(chunk) > piRPCMaxDeltaBytes || !json.Valid(mustDeltaJSON(t, chunk)) {
				t.Fatalf("offset %d chunk %d invalid: %d bytes", offset, index, len(chunk))
			}
			rebuilt = append(rebuilt, chunk...)
		}
		if !bytes.Equal(rebuilt, input) {
			t.Fatalf("offset %d changed UTF-8 content", offset)
		}
	}
}

func FuzzPiRPCObjectNoPanic(f *testing.F) {
	f.Add([]byte(`{"type":"agent_start"}`))
	f.Add([]byte(`{"type":"x","type":"y"}`))
	f.Fuzz(func(t *testing.T, value []byte) {
		_, _ = piRPCObject(value)
	})
}

type piRPCBridgeFixture struct {
	executablePath string
	searchPath     string
	workspacePath  string
	homePath       string
	tempPath       string
	binding        bridgev1.RunStreamBinding
	dispatch       bridgev1.Frame
	token          authorization.Token
}

func newPiRPCBridgeFixture(t testing.TB, mode string) *piRPCBridgeFixture {
	t.Helper()
	root := piPrivateDirectory(t, "rpc-bridge")
	executablePath := filepath.Join(root, "pi-fixture")
	script := piRPCFixtureScript(mode)
	if err := os.WriteFile(executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(executablePath, 0o700); err != nil {
		t.Fatal(err)
	}
	binding := bridgev1.RunStreamBinding{
		WorkItemID:            "S5-W2",
		RunID:                 "run-rpc-1",
		ClaimGeneration:       1,
		RuntimeInstanceID:     "runtime.pi.earendil-works.0.82.1",
		SenderAgentInstanceID: "agent-main-1",
	}
	payload, err := json.Marshal(struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Prompt        string `json:"prompt"`
	}{SchemaVersion: 1, Kind: "pi_rpc_prompt", Prompt: piRPCFixturePrompt})
	if err != nil {
		t.Fatal(err)
	}
	token, err := authorization.ParseToken(piTestTokenValue)
	if err != nil {
		t.Fatal(err)
	}
	return &piRPCBridgeFixture{
		executablePath: executablePath,
		searchPath:     piPrivateDirectoryAt(t, root, "search"),
		workspacePath:  piPrivateDirectoryAt(t, root, "workspace"),
		homePath:       piPrivateDirectoryAt(t, root, "home"),
		tempPath:       piPrivateDirectoryAt(t, root, "tmp"),
		binding:        binding,
		dispatch: piTestFrame(
			t,
			binding,
			1,
			bridgev1.MessageDispatch,
			payload,
		),
		token: token,
	}
}

func (fixture *piRPCBridgeFixture) config() PiRPCBridgeAdapterConfig {
	return PiRPCBridgeAdapterConfig{
		Execution: PiExecutionAdapterConfig{
			ExecutablePath:     fixture.executablePath,
			RuntimeInstanceID:  fixture.binding.RuntimeInstanceID,
			RuntimeSearchPaths: []string{fixture.searchPath},
			CancelGrace:        200 * time.Millisecond,
			Now:                func() time.Time { return managedPiTestNow },
			Random:             bytes.NewReader(bytes.Repeat([]byte{0x45}, 2048)),
		},
		ProviderID:        "loom-local",
		ModelID:           "qwen2.5-coder-1.5b-instruct-q4-k-m",
		BaseURL:           "http://127.0.0.1:18427/v1",
		MaxAssistantBytes: 16384,
	}
}

func (fixture *piRPCBridgeFixture) request(t testing.TB) supervisor.AdapterRequest {
	t.Helper()
	return supervisor.AdapterRequest{
		WorkspacePath: fixture.workspacePath,
		HomePath:      fixture.homePath,
		TempPath:      fixture.tempPath,
		Binding:       fixture.binding,
		Dispatch:      fixture.dispatch,
		Grant:         fixture.token,
		FrameSink:     &piRecordingFrameSink{},
	}
}

func piRPCFixtureScript(mode string) string {
	responseID := "10000000-0000-4000-8000-000000000001"
	expectedArguments := []string{
		"--mode", "rpc",
		"--offline",
		"--no-approve",
		"--no-session",
		"--no-tools",
		"--no-extensions",
		"--no-skills",
		"--no-prompt-templates",
		"--no-themes",
		"--no-context-files",
		"--provider", "loom-local",
		"--model", "loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m",
		"--thinking", "off",
		"--system-prompt", piRPCSystemPrompt,
	}
	header := "#!/bin/sh\n" +
		"[ -z \"${SHOULD_NOT_LEAK+x}\" ] || exit 41\n" +
		"[ -z \"${LOOM_AGENT_GRANT+x}\" ] || exit 42\n" +
		"[ \"$PI_OFFLINE\" = 1 ] || exit 43\n" +
		"[ \"$PI_SKIP_VERSION_CHECK\" = 1 ] || exit 44\n" +
		"[ \"$PI_TELEMETRY\" = 0 ] || exit 45\n" +
		"[ \"$#\" -eq " + fmt.Sprint(len(expectedArguments)) + " ] || exit 46\n"
	for _, argument := range expectedArguments {
		header += "[ \"$1\" = " + piRPCShellQuote(argument) + " ] || exit 47\nshift\n"
	}
	if mode == "wait-cancel" {
		return header +
			"read request || exit 48\n" +
			"read abort || exit 49\n" +
			"printf '%s\\n' '{\"id\":\"45454545-4545-4545-8545-454545454545\",\"type\":\"response\",\"command\":\"abort\",\"success\":true}'\n" +
			"exec /bin/sleep 30\n"
	}
	userMessage := piRPCFixtureUserMessage(piRPCFixturePrompt)
	emptyAssistant := piRPCFixtureAssistantMessage("")
	emptyTextAssistant := strings.Replace(
		emptyAssistant,
		`"content":[]`,
		`"content":[{"type":"text","text":""}]`,
		1,
	)
	firstAssistant := piRPCFixtureAssistantMessage("Hello ")
	finalAssistant := piRPCFixtureAssistantMessage("Hello world")
	lines := []string{
		`{"id":"` + responseID + `","type":"response","command":"prompt","success":true}`,
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"message_start","message":` + userMessage + `}`,
		`{"type":"message_end","message":` + userMessage + `}`,
		`{"type":"message_start","message":` + emptyAssistant + `}`,
		`{"type":"message_update","message":` + emptyTextAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + emptyTextAssistant + `}}`,
		`{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + firstAssistant + `}}`,
		`{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"world","partial":` + finalAssistant + `}}`,
		`{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"Hello world","partial":` + finalAssistant + `}}`,
		`{"type":"message_end","message":` + finalAssistant + `}`,
		`{"type":"turn_end","message":` + finalAssistant + `,"toolResults":[]}`,
		`{"type":"agent_end","messages":[` + userMessage + `,` + finalAssistant + `],"willRetry":false}`,
		`{"type":"agent_settled"}`,
	}
	switch mode {
	case "unknown":
		lines[3] = `{"type":"extension_error"}`
	case "tool":
		toolAssistant := strings.Replace(
			emptyAssistant,
			`"content":[]`,
			`"content":[{"type":"toolCall","id":"x","name":"bash","arguments":{}}]`,
			1,
		)
		lines[7] = `{"type":"message_update","message":` + toolAssistant + `,"assistantMessageEvent":{"type":"toolcall_start","contentIndex":0,"partial":` + toolAssistant + `}}`
	case "thinking":
		thinkingAssistant := strings.Replace(
			emptyAssistant,
			`"content":[]`,
			`"content":[{"type":"thinking","thinking":"hidden"}]`,
			1,
		)
		lines[7] = `{"type":"message_update","message":` + thinkingAssistant + `,"assistantMessageEvent":{"type":"thinking_start","contentIndex":0,"partial":` + thinkingAssistant + `}}`
	case "length":
		lengthAssistant := strings.ReplaceAll(finalAssistant, `"stopReason":"stop"`, `"stopReason":"length"`)
		lines[10] = `{"type":"message_end","message":` + lengthAssistant + `}`
	case "nested-start-event":
		lines[6] = `{"type":"message_update","message":` + emptyAssistant + `,"assistantMessageEvent":{"type":"start","partial":` + emptyAssistant + `}}`
	case "nested-done-event":
		lines[9] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"done","reason":"stop","message":` + finalAssistant + `}}`
	case "nested-error-event":
		lines[9] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"error","reason":"error","error":` + finalAssistant + `}}`
	case "partial-identity-mismatch":
		differentPartial := strings.Replace(firstAssistant, `"timestamp":1`, `"responseId":"different","timestamp":1`, 1)
		lines[7] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + differentPartial + `}}`
	case "response-model-substitution":
		substituted := strings.Replace(finalAssistant, `"timestamp":1`, `"responseModel":"other-model","timestamp":1`, 1)
		lines[10] = `{"type":"message_end","message":` + substituted + `}`
	case "retry":
		lines[12] = `{"type":"agent_end","messages":[` + userMessage + `,` + finalAssistant + `],"willRetry":true}`
	case "mismatch":
		differentAssistant := piRPCFixtureAssistantMessage("different")
		lines[9] = `{"type":"message_update","message":` + differentAssistant + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"different","partial":` + differentAssistant + `}}`
	case "uncorrelated":
		lines[0] = `{"id":"20000000-0000-4000-8000-000000000002","type":"response","command":"prompt","success":true}`
	case "duplicate":
		lines[0] = `{"id":"` + responseID + `","type":"response","type":"response","command":"prompt","success":true}`
	case "extra":
		lines[0] = `{"id":"` + responseID + `","type":"response","command":"prompt","success":true,"extra":true}`
	case "nested-duplicate":
		lines[5] = strings.Replace(emptyAssistant, `"role":"assistant"`, `"role":"assistant","role":"assistant"`, 1)
		lines[5] = `{"type":"message_start","message":` + lines[5] + `}`
	case "nested-extra":
		nested := strings.Replace(emptyAssistant, `"timestamp":1`, `"timestamp":1,"extra":true`, 1)
		lines[5] = `{"type":"message_start","message":` + nested + `}`
	case "assistant-error":
		nested := strings.Replace(emptyAssistant, `"timestamp":1`, `"timestamp":1,"errorMessage":"secret"`, 1)
		lines[5] = `{"type":"message_start","message":` + nested + `}`
	case "agent-end-extra":
		lines[12] = `{"type":"agent_end","messages":[` + userMessage + `,` + finalAssistant + `,` + userMessage + `],"willRetry":false}`
	case "after-settled":
		lines = append(lines, `{"type":"agent_start"}`)
	case "carriage-return":
		lines[0] += "\r"
	case "unicode-separator":
		lines[0] += "\u2028"
	case "grant":
		grantAssistant := piRPCFixtureAssistantMessage(piTestTokenValue)
		lines[7] = `{"type":"message_update","message":` + grantAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"` + piTestTokenValue + `","partial":` + grantAssistant + `}}`
	}
	var quoted []string
	for _, line := range lines {
		quoted = append(quoted, "'"+strings.ReplaceAll(line, "'", "'\\''")+"'")
	}
	return header +
		"read request || exit 42\n" +
		"printf '%s\\n' " + strings.Join(quoted, " ") + "\n"
}

func piRPCFixtureUserMessage(prompt string) string {
	value, err := json.Marshal(struct {
		Role      string `json:"role"`
		Content   string `json:"content"`
		Timestamp int64  `json:"timestamp"`
	}{
		Role:      "user",
		Content:   prompt,
		Timestamp: 1,
	})
	if err != nil {
		panic(err)
	}
	return string(value)
}

func piRPCFixtureAssistantMessage(text string) string {
	type content struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type cost struct {
		Input      int `json:"input"`
		Output     int `json:"output"`
		CacheRead  int `json:"cacheRead"`
		CacheWrite int `json:"cacheWrite"`
		Total      int `json:"total"`
	}
	type usage struct {
		Input       int  `json:"input"`
		Output      int  `json:"output"`
		CacheRead   int  `json:"cacheRead"`
		CacheWrite  int  `json:"cacheWrite"`
		TotalTokens int  `json:"totalTokens"`
		Cost        cost `json:"cost"`
	}
	message := struct {
		Role       string    `json:"role"`
		Content    []content `json:"content"`
		API        string    `json:"api"`
		Provider   string    `json:"provider"`
		Model      string    `json:"model"`
		Usage      usage     `json:"usage"`
		StopReason string    `json:"stopReason"`
		Timestamp  int64     `json:"timestamp"`
	}{
		Role:       "assistant",
		API:        "openai-completions",
		Provider:   piRPCProviderID,
		Model:      piRPCModelID,
		StopReason: "stop",
		Timestamp:  1,
	}
	if text != "" {
		message.Content = []content{{Type: "text", Text: text}}
	} else {
		message.Content = []content{}
	}
	value, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}
	return string(value)
}

func piRPCShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func mustDeltaJSON(t testing.TB, chunk []byte) []byte {
	t.Helper()
	payload, err := json.Marshal(struct {
		Delta string `json:"delta"`
	}{Delta: string(chunk)})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func mustJSONFrames(t testing.TB, frames []bridgev1.Frame) []byte {
	t.Helper()
	var result []byte
	for _, frame := range frames {
		line, err := bridgev1.EncodeLine(frame)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, line...)
	}
	return result
}
