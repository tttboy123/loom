package piadapter

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPiRPCConversationSystemPromptFreezesLocalModelWithoutTools(t *testing.T) {
	for _, required := range []string{
		"provider=loom-local",
		"model=" + piRPCModelID,
		"Tools are unavailable in this conversation",
		"Do not infer or invent a different underlying model",
	} {
		if !strings.Contains(piRPCConversationSystemPrompt, required) {
			t.Fatalf("conversation system prompt missing %q: %s", required, piRPCConversationSystemPrompt)
		}
	}
}

const piRPCConversationFixtureID = "45454545-4545-4545-8545-454545454545"

func TestPiRPCConversationAdapterReturnsStrictToolDisabledText(t *testing.T) {
	request := PiRPCConversationRequest{
		ThreadID: "thread-one",
		Messages: []PiRPCConversationMessage{
			{Role: "user", Content: "Review this function for a race."},
		},
	}
	prompt, err := buildPiRPCConversationPrompt("", request.Messages)
	if err != nil {
		t.Fatal(err)
	}
	fixture := newPiRPCConversationFixture(t, "success", prompt)
	adapter, err := NewPiRPCConversationAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}

	response, err := adapter.Respond(context.Background(), request)
	if err != nil {
		t.Fatalf("Respond() error = %v", err)
	}
	if response.Content != "Hello world" {
		t.Fatalf("Respond() content = %q", response.Content)
	}
	entries, err := os.ReadDir(fixture.privateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("conversation invocation roots remain: %#v", entries)
	}
}

func TestPiRPCConversationAdapterRejectsToolCalls(t *testing.T) {
	request := PiRPCConversationRequest{
		ThreadID: "thread-tool",
		Messages: []PiRPCConversationMessage{
			{Role: "user", Content: "Do not use tools."},
		},
	}
	prompt, err := buildPiRPCConversationPrompt("", request.Messages)
	if err != nil {
		t.Fatal(err)
	}
	fixture := newPiRPCConversationFixture(t, "tool", prompt)
	adapter, err := NewPiRPCConversationAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	response, err := adapter.Respond(context.Background(), request)
	if !errors.Is(err, ErrPiRPCProtocol) || response.Content != "" {
		t.Fatalf("Respond() = %#v, %v; want closed protocol rejection", response, err)
	}
	diagnostic := err.Error()
	for _, required := range []string{
		"record=message_update",
		"role=assistant",
		"kind=toolcall_start",
		"keys=assistantMessageEvent,message,type",
	} {
		if !strings.Contains(diagnostic, required) {
			t.Fatalf("safe protocol diagnostic missing %q: %s", required, diagnostic)
		}
	}
	if strings.Contains(diagnostic, request.Messages[0].Content) {
		t.Fatalf("safe protocol diagnostic disclosed conversation content: %s", diagnostic)
	}
}

func TestPiRPCConversationPromptDropsOldestHistoryFirst(t *testing.T) {
	oldest := strings.Repeat("a", 4_096)
	middle := strings.Repeat("b", 4_096)
	latest := "latest user request"
	prompt, err := buildPiRPCConversationPrompt("", []PiRPCConversationMessage{
		{Role: "user", Content: oldest},
		{Role: "loom", Content: middle},
		{Role: "user", Content: latest},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prompt) > piRPCMaxPromptBytes || strings.Contains(prompt, oldest) ||
		!strings.Contains(prompt, middle) || !strings.Contains(prompt, latest) {
		t.Fatalf("bounded prompt bytes=%d oldest=%t middle=%t latest=%t",
			len(prompt), strings.Contains(prompt, oldest), strings.Contains(prompt, middle),
			strings.Contains(prompt, latest))
	}
}

func TestPiRPCConversationAdapterCancellationReapsProcess(t *testing.T) {
	request := PiRPCConversationRequest{
		ThreadID: "thread-cancel",
		Messages: []PiRPCConversationMessage{
			{Role: "user", Content: "Wait until cancelled."},
		},
	}
	prompt, err := buildPiRPCConversationPrompt("", request.Messages)
	if err != nil {
		t.Fatal(err)
	}
	fixture := newPiRPCConversationFixture(t, "wait-cancel", prompt)
	adapter, err := NewPiRPCConversationAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	response, err := adapter.Respond(ctx, request)
	if !errors.Is(err, context.DeadlineExceeded) || response.Content != "" {
		t.Fatalf("Respond() = %#v, %v; want cancellation", response, err)
	}
	if !strings.Contains(err.Error(), "response=false agent=false turns=0") ||
		strings.Contains(err.Error(), request.Messages[0].Content) {
		t.Fatalf("cancellation diagnostic is incomplete or disclosed content: %v", err)
	}
	entries, readErr := os.ReadDir(fixture.privateRoot)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("cancel cleanup entries=%#v error=%v", entries, readErr)
	}
}

type piRPCConversationFixture struct {
	executablePath string
	searchPath     string
	privateRoot    string
}

func newPiRPCConversationFixture(
	t testing.TB,
	mode string,
	prompt string,
) *piRPCConversationFixture {
	t.Helper()
	root := piPrivateDirectory(t, "rpc-conversation")
	executablePath := filepath.Join(root, "pi-fixture")
	script := piRPCFixtureScriptFor(
		mode,
		prompt,
		piRPCConversationSystemPrompt,
		piRPCConversationFixtureID,
		true,
	)
	if err := os.WriteFile(executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(executablePath, 0o700); err != nil {
		t.Fatal(err)
	}
	return &piRPCConversationFixture{
		executablePath: executablePath,
		searchPath:     piPrivateDirectoryAt(t, root, "search"),
		privateRoot:    piPrivateDirectoryAt(t, root, "private"),
	}
}

func (fixture *piRPCConversationFixture) config() PiRPCConversationAdapterConfig {
	return PiRPCConversationAdapterConfig{
		Execution: PiExecutionAdapterConfig{
			ExecutablePath:     fixture.executablePath,
			RuntimeInstanceID:  "runtime.pi.conversation",
			RuntimeSearchPaths: []string{fixture.searchPath},
			CancelGrace:        200 * time.Millisecond,
			Now:                func() time.Time { return managedPiTestNow },
			Random:             bytes.NewReader(bytes.Repeat([]byte{0x45}, 2048)),
		},
		ProviderID:        piRPCProviderID,
		ModelID:           piRPCModelID,
		BaseURL:           "http://127.0.0.1:18427/v1",
		PrivateRoot:       fixture.privateRoot,
		MaxAssistantBytes: 16_384,
	}
}
