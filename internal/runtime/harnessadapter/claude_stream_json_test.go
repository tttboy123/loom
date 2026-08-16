package harnessadapter

import (
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClaudeStreamJSONContinuesAgentInputInOneSession(t *testing.T) {
	t.Parallel()
	session := newClaudeStreamJSONSessionFixture()
	session.includeToolLoop = true
	session.includeCompactBoundary = true
	sessions := &claudeContinuationSessionRunnerFixture{session: session}
	runner, err := NewClaudeCodeProcessRunner(ClaudeCodeProcessRunnerConfig{
		Gateway:  codexContinuationGatewayFixture{},
		Commands: codexContinuationCommandFixture{},
		Sessions: sessions,
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	prompt := []byte("initial private prompt")
	result, err := runner.(HarnessAgentInputRunner).RunHarnessWithAgentInputs(
		context.Background(),
		HarnessProcessRequest{
			ExecutablePath: "/opt/loom/bin/claude", WorkspacePath: root + "/workspace",
			HomePath: root, TempPath: root, ModelID: ClaudeCodeModelID,
			ReasoningEffort: "high", Prompt: prompt,
			SystemPrompt: "Loom governed system prompt",
			Timeout:      time.Minute, MaxOutputBytes: 1 << 16,
		},
		[]byte("private-provider-key"),
		&codexAgentInputSourceFixture{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "second reply" || result.Accounting == nil ||
		result.Accounting.InputTokens != 3 || result.Accounting.OutputTokens != 5 ||
		result.Accounting.TotalTokens != 8 ||
		result.Accounting.CostMicrounits != 3000 {
		t.Fatalf("result = %#v", result)
	}
	if !allHarnessBytesZero(prompt) {
		t.Fatal("initial prompt was not zeroized")
	}
	if got := session.promptsSnapshot(); !reflect.DeepEqual(got, []string{
		"initial private prompt",
		"Loom steer input (scope=agent_private):\ncontinue privately",
	}) {
		t.Fatalf("prompts = %#v", got)
	}
	arguments := sessions.request.Arguments
	for _, expected := range []string{
		"--print", "--bare", "--no-session-persistence",
		"--input-format", "stream-json", "--output-format", "stream-json",
		"--replay-user-messages", "--verbose", "dontAsk", "--disallowedTools",
	} {
		if !containsHarnessArgument(arguments, expected) {
			t.Fatalf("arguments missing %q: %#v", expected, arguments)
		}
	}
	joinedArguments := strings.Join(arguments, "\n")
	if sessions.request.Directory != root || strings.Contains(joinedArguments, "acceptEdits") ||
		strings.Contains(joinedArguments, root+"/workspace") {
		t.Fatalf("unsafe stream boundary request=%#v", sessions.request)
	}
	for index, argument := range arguments {
		if (argument == "--tools" || argument == "--allowedTools") &&
			(index+1 >= len(arguments) || arguments[index+1] != "") {
			t.Fatalf("native stream tools enabled: %#v", arguments)
		}
	}
	if !session.closed || !session.waited || session.aborted {
		t.Fatalf("session lifecycle closed=%t waited=%t aborted=%t",
			session.closed, session.waited, session.aborted)
	}
}

func TestClaudeStreamJSONRejectsSessionSubstitution(t *testing.T) {
	t.Parallel()
	session := newClaudeStreamJSONSessionFixture()
	session.substituteSession = true
	runner, err := NewClaudeCodeProcessRunner(ClaudeCodeProcessRunnerConfig{
		Gateway:  codexContinuationGatewayFixture{},
		Commands: codexContinuationCommandFixture{},
		Sessions: &claudeContinuationSessionRunnerFixture{session: session},
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	prompt := []byte("private prompt")
	_, err = runner.(HarnessAgentInputRunner).RunHarnessWithAgentInputs(
		context.Background(),
		HarnessProcessRequest{
			ExecutablePath: "/opt/loom/bin/claude", WorkspacePath: root,
			HomePath: root, TempPath: root, ModelID: ClaudeCodeModelID,
			Prompt: prompt, SystemPrompt: "Loom governed system prompt",
			Timeout: time.Minute, MaxOutputBytes: 1 << 16,
		},
		[]byte("private-provider-key"),
		&codexNoAgentInputSourceFixture{},
	)
	if err == nil || !session.aborted {
		t.Fatalf("session substitution err=%v aborted=%t", err, session.aborted)
	}
	if !allHarnessBytesZero(prompt) {
		t.Fatal("prompt was not zeroized on failure")
	}
}

type claudeContinuationSessionRunnerFixture struct {
	session HarnessStreamSession
	request HarnessSessionRequest
}

func (runner *claudeContinuationSessionRunnerFixture) StartSession(
	_ context.Context,
	request HarnessSessionRequest,
) (HarnessStreamSession, error) {
	runner.request = request
	return runner.session, nil
}

type claudeStreamJSONSessionFixture struct {
	mu                     sync.Mutex
	lines                  chan []byte
	prompts                []string
	sequence               int
	substituteSession      bool
	includeToolLoop        bool
	includeCompactBoundary bool
	closed                 bool
	waited                 bool
	aborted                bool
}

func newClaudeStreamJSONSessionFixture() *claudeStreamJSONSessionFixture {
	return &claudeStreamJSONSessionFixture{lines: make(chan []byte, 32)}
}

func (session *claudeStreamJSONSessionFixture) WriteLine(
	_ context.Context,
	payload []byte,
) error {
	var request struct {
		Type    string `json:"type"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(payload, &request) != nil || request.Type != "user" ||
		request.Message.Role != "user" || request.Message.Content == "" {
		return ErrHarnessProtocol
	}
	session.mu.Lock()
	session.prompts = append(session.prompts, request.Message.Content)
	session.sequence++
	sequence := session.sequence
	session.mu.Unlock()
	sessionID := "claude-session-locked-1"
	if sequence == 1 {
		session.enqueue(map[string]any{
			"type": "system", "subtype": "init", "session_id": sessionID,
			"model": ClaudeCodeModelID, "claude_code_version": "2.1.196",
		})
	}
	session.enqueue(map[string]any{
		"type": "user", "session_id": sessionID,
		"message": map[string]any{"role": "user", "content": request.Message.Content},
	})
	if session.includeCompactBoundary && sequence == 1 {
		session.enqueue(map[string]any{
			"type": "system", "subtype": "compact_boundary", "session_id": sessionID,
			"compact_metadata": map[string]any{"trigger": "auto"},
		})
	}
	reply := "first reply"
	if sequence == 2 {
		reply = "second reply"
	}
	if session.includeToolLoop && sequence == 1 {
		session.enqueue(map[string]any{
			"type": "assistant", "session_id": sessionID,
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{{
					"type": "tool_use", "id": "tool-call-1", "name": "Read",
					"input": map[string]any{"file_path": "/private/tmp/workspace/file.go"},
				}},
			},
		})
		session.enqueue(map[string]any{
			"type": "user", "session_id": sessionID,
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{{
					"type": "tool_result", "tool_use_id": "tool-call-1",
					"content": "bounded tool result",
				}},
			},
		})
	}
	session.enqueue(map[string]any{
		"type": "assistant", "session_id": sessionID,
		"message": map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"type": "text", "text": reply}},
		},
	})
	resultSessionID := sessionID
	if session.substituteSession {
		resultSessionID = "claude-session-substituted"
	}
	session.enqueue(map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"session_id": resultSessionID, "result": reply,
		"total_cost_usd": float64(sequence) / 1000,
		"usage": map[string]any{
			"input_tokens": sequence, "output_tokens": sequence + 1,
			"cache_read_input_tokens": 0, "cache_creation_input_tokens": 0,
		},
	})
	return nil
}

func (session *claudeStreamJSONSessionFixture) enqueue(value any) {
	payload, _ := json.Marshal(value)
	session.lines <- payload
}

func (session *claudeStreamJSONSessionFixture) ReadLine(context.Context) ([]byte, error) {
	line, ok := <-session.lines
	if !ok {
		return nil, io.EOF
	}
	return line, nil
}

func (session *claudeStreamJSONSessionFixture) CloseInput() error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.closed {
		session.closed = true
		close(session.lines)
	}
	return nil
}

func (session *claudeStreamJSONSessionFixture) Wait(context.Context) (HarnessCommandResult, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.waited = true
	return HarnessCommandResult{ExitCode: 0}, nil
}

func (session *claudeStreamJSONSessionFixture) Abort() error {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.aborted = true
	return nil
}

func (session *claudeStreamJSONSessionFixture) promptsSnapshot() []string {
	session.mu.Lock()
	defer session.mu.Unlock()
	return append([]string(nil), session.prompts...)
}

func containsHarnessArgument(arguments []string, expected string) bool {
	for _, argument := range arguments {
		if argument == expected {
			return true
		}
	}
	return false
}
