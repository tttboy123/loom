package harnessadapter

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agentinbox"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestCodexAppServerContinuesAgentInputOnOneThread(t *testing.T) {
	t.Parallel()
	session := newCodexAppServerSessionFixture()
	sessions := &codexContinuationSessionRunnerFixture{session: session}
	runner, err := NewCodexProcessRunner(CodexProcessRunnerConfig{
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
			ExecutablePath:  "/opt/loom/bin/codex",
			WorkspacePath:   root + "/workspace",
			HomePath:        root,
			TempPath:        root,
			ModelID:         CodexModelID,
			ReasoningEffort: "high",
			Prompt:          prompt,
			SystemPrompt:    "Loom governed system prompt",
			Timeout:         time.Minute,
			MaxOutputBytes:  1 << 16,
		},
		[]byte("private-provider-key"),
		&codexAgentInputSourceFixture{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "second reply" || result.Accounting == nil ||
		result.Accounting.InputTokens != 3 || result.Accounting.OutputTokens != 5 ||
		result.Accounting.TotalTokens != 8 {
		t.Fatalf("result = %#v", result)
	}
	if !allHarnessBytesZero(prompt) {
		t.Fatal("initial prompt was not zeroized")
	}
	requests := session.requestsSnapshot()
	if len(requests) != 4 {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0].Method != "initialize" || requests[1].Method != "thread/start" ||
		requests[2].Method != "turn/start" || requests[3].Method != "turn/start" {
		t.Fatalf("methods = %#v", requests)
	}
	if sessions.request.Directory != root || requests[1].CWD != root ||
		requests[1].ApprovalPolicy != "never" || requests[1].SandboxType != "readOnly" ||
		len(requests[1].WritableRoots) != 0 {
		t.Fatalf("ungoverned app-server boundary command=%#v thread=%#v",
			sessions.request, requests[1])
	}
	arguments := strings.Join(sessions.request.Arguments, "\n")
	for _, expected := range []string{
		`sandbox_mode="read-only"`, `features.shell_tool=false`,
		`features.unified_exec=false`, `features.apply_patch_freeform=false`,
		`features.tool_search=false`,
	} {
		if !strings.Contains(arguments, expected) {
			t.Fatalf("app-server arguments missing %q: %s", expected, arguments)
		}
	}
	if strings.Contains(arguments, "workspace-write") ||
		strings.Contains(arguments, root+"/workspace") {
		t.Fatalf("unsafe app-server arguments = %s", arguments)
	}
	if requests[2].ThreadID != "thread-locked-1" ||
		requests[3].ThreadID != "thread-locked-1" ||
		requests[2].Text != "initial private prompt" ||
		requests[3].Text != "Loom steer input (scope=agent_private):\ncontinue privately" {
		t.Fatalf("turn requests = %#v", requests[2:])
	}
	if !session.closed || !session.waited || session.aborted {
		t.Fatalf("session lifecycle closed=%t waited=%t aborted=%t", session.closed, session.waited, session.aborted)
	}
}

func TestCodexAppServerRejectsThreadOrTurnSubstitution(t *testing.T) {
	t.Parallel()
	for _, substitution := range []string{"thread", "turn"} {
		t.Run(substitution, func(t *testing.T) {
			root := t.TempDir()
			session := newCodexAppServerSessionFixture()
			session.substitution = substitution
			runner, err := NewCodexProcessRunner(CodexProcessRunnerConfig{
				Gateway:  codexContinuationGatewayFixture{},
				Commands: codexContinuationCommandFixture{},
				Sessions: &codexContinuationSessionRunnerFixture{session: session},
			})
			if err != nil {
				t.Fatal(err)
			}
			prompt := []byte("private prompt")
			_, err = runner.(HarnessAgentInputRunner).RunHarnessWithAgentInputs(
				context.Background(),
				HarnessProcessRequest{
					ExecutablePath: "/opt/loom/bin/codex",
					WorkspacePath:  root,
					HomePath:       root,
					TempPath:       root,
					ModelID:        CodexModelID,
					Prompt:         prompt,
					SystemPrompt:   "Loom governed system prompt",
					Timeout:        time.Minute,
					MaxOutputBytes: 1 << 16,
				},
				[]byte("private-provider-key"),
				&codexNoAgentInputSourceFixture{},
			)
			if err == nil {
				t.Fatal("substitution accepted")
			}
			if !session.aborted {
				t.Fatal("failed session was not aborted")
			}
			if !allHarnessBytesZero(prompt) {
				t.Fatal("prompt was not zeroized on failure")
			}
		})
	}
}

func TestCodexAppServerAcceptsBoundTurnStartedBeforeTurnStartResponse(t *testing.T) {
	t.Parallel()
	session := newCodexAppServerSessionFixture()
	session.turnStartedBeforeResponse = true
	runner, err := NewCodexProcessRunner(CodexProcessRunnerConfig{
		Gateway:  codexContinuationGatewayFixture{},
		Commands: codexContinuationCommandFixture{},
		Sessions: &codexContinuationSessionRunnerFixture{session: session},
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	result, err := runner.(HarnessAgentInputRunner).RunHarnessWithAgentInputs(
		context.Background(),
		HarnessProcessRequest{
			ExecutablePath: "/opt/loom/bin/codex", WorkspacePath: root,
			HomePath: root, TempPath: root, ModelID: CodexModelID,
			Prompt: []byte("private prompt"), SystemPrompt: "governed system prompt",
			Timeout: time.Minute, MaxOutputBytes: 1 << 16,
		},
		[]byte("private-provider-key"),
		&codexNoAgentInputSourceFixture{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "first reply" {
		t.Fatalf("result = %#v", result)
	}
}

type codexAppServerRequestSnapshot struct {
	Method         string
	ThreadID       string
	Text           string
	CWD            string
	ApprovalPolicy string
	SandboxType    string
	WritableRoots  []string
}

type codexAppServerSessionFixture struct {
	mu                        sync.Mutex
	lines                     chan []byte
	requests                  []codexAppServerRequestSnapshot
	turnSequence              int
	substitution              string
	turnStartedBeforeResponse bool
	closed                    bool
	waited                    bool
	aborted                   bool
}

func newCodexAppServerSessionFixture() *codexAppServerSessionFixture {
	return &codexAppServerSessionFixture{lines: make(chan []byte, 16)}
}

func (session *codexAppServerSessionFixture) WriteLine(
	_ context.Context,
	payload []byte,
) error {
	var request struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params struct {
			ThreadID       string `json:"threadId"`
			CWD            string `json:"cwd"`
			ApprovalPolicy string `json:"approvalPolicy"`
			Sandbox        struct {
				Type          string   `json:"type"`
				WritableRoots []string `json:"writableRoots"`
			} `json:"sandbox"`
			Input []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"input"`
		} `json:"params"`
	}
	if json.Unmarshal(payload, &request) != nil || request.ID == "" || request.Method == "" {
		return ErrHarnessProtocol
	}
	snapshot := codexAppServerRequestSnapshot{
		Method: request.Method, ThreadID: request.Params.ThreadID,
		CWD: request.Params.CWD, ApprovalPolicy: request.Params.ApprovalPolicy,
		SandboxType:   request.Params.Sandbox.Type,
		WritableRoots: append([]string(nil), request.Params.Sandbox.WritableRoots...),
	}
	if len(request.Params.Input) == 1 {
		snapshot.Text = request.Params.Input[0].Text
	}
	session.mu.Lock()
	session.requests = append(session.requests, snapshot)
	if request.Method == "turn/start" {
		session.turnSequence++
	}
	turnSequence := session.turnSequence
	session.mu.Unlock()

	switch request.Method {
	case "initialize":
		session.enqueue(map[string]any{
			"id": request.ID,
			"result": map[string]any{
				"userAgent":      "codex_cli_rs/0.144.1",
				"codexHome":      "/private/tmp/home",
				"platformFamily": "unix",
				"platformOs":     "macos",
			},
		})
	case "thread/start":
		threadID := "thread-locked-1"
		if session.substitution == "thread" {
			threadID = ""
		}
		session.enqueue(map[string]any{
			"id":     request.ID,
			"result": map[string]any{"thread": map[string]any{"id": threadID}},
		})
	case "turn/start":
		turnID := "turn-locked-" + string(rune('0'+turnSequence))
		if session.turnStartedBeforeResponse {
			session.enqueue(map[string]any{
				"method": "turn/started",
				"params": map[string]any{
					"threadId": request.Params.ThreadID,
					"turn":     map[string]any{"id": turnID, "status": "inProgress"},
				},
			})
		}
		session.enqueue(map[string]any{
			"id": request.ID,
			"result": map[string]any{"turn": map[string]any{
				"id": turnID, "status": "inProgress",
			}},
		})
		notificationTurnID := turnID
		if session.substitution == "turn" {
			notificationTurnID = "turn-substituted"
		}
		content := "first reply"
		if turnSequence == 2 {
			content = "second reply"
		}
		session.enqueue(map[string]any{
			"method": "item/completed",
			"params": map[string]any{
				"threadId": request.Params.ThreadID,
				"turnId":   notificationTurnID,
				"item":     map[string]any{"type": "agentMessage", "text": content},
			},
		})
		session.enqueue(map[string]any{
			"method": "thread/tokenUsage/updated",
			"params": map[string]any{
				"threadId": request.Params.ThreadID,
				"turnId":   notificationTurnID,
				"tokenUsage": map[string]any{
					"last": map[string]any{
						"inputTokens":           sequenceOrOne(turnSequence),
						"cachedInputTokens":     0,
						"outputTokens":          sequenceOrOne(turnSequence) + 1,
						"reasoningOutputTokens": 0,
						"totalTokens":           sequenceOrOne(turnSequence)*2 + 1,
					},
				},
			},
		})
		session.enqueue(map[string]any{
			"method": "turn/completed",
			"params": map[string]any{
				"threadId": request.Params.ThreadID,
				"turn":     map[string]any{"id": notificationTurnID, "status": "completed"},
			},
		})
	default:
		return ErrHarnessProtocol
	}
	return nil
}

func sequenceOrOne(sequence int) int {
	if sequence < 1 {
		return 1
	}
	return sequence
}

func (session *codexAppServerSessionFixture) enqueue(value any) {
	payload, _ := json.Marshal(value)
	session.lines <- payload
}

func (session *codexAppServerSessionFixture) ReadLine(context.Context) ([]byte, error) {
	line, ok := <-session.lines
	if !ok {
		return nil, io.EOF
	}
	return line, nil
}

func (session *codexAppServerSessionFixture) CloseInput() error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.closed {
		session.closed = true
		close(session.lines)
	}
	return nil
}

func (session *codexAppServerSessionFixture) Wait(context.Context) (HarnessCommandResult, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.waited = true
	return HarnessCommandResult{ExitCode: 0}, nil
}

func (session *codexAppServerSessionFixture) Abort() error {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.aborted = true
	return nil
}

func (session *codexAppServerSessionFixture) requestsSnapshot() []codexAppServerRequestSnapshot {
	session.mu.Lock()
	defer session.mu.Unlock()
	return append([]codexAppServerRequestSnapshot(nil), session.requests...)
}

type codexContinuationSessionRunnerFixture struct {
	session HarnessStreamSession
	request HarnessSessionRequest
}

func (runner *codexContinuationSessionRunnerFixture) StartSession(
	_ context.Context,
	request HarnessSessionRequest,
) (HarnessStreamSession, error) {
	runner.request = request
	return runner.session, nil
}

type codexContinuationGatewayFixture struct{}

func (codexContinuationGatewayFixture) WithCredential(
	ctx context.Context,
	_, _ string,
	_ []byte,
	_ AttemptProviderToolPolicy,
	callback func(AttemptGatewayLease) error,
) error {
	return callback(AttemptGatewayLease{BaseURL: "http://127.0.0.1:42100", Token: "attempt-token"})
}

type codexContinuationCommandFixture struct{}

func (codexContinuationCommandFixture) RunCommand(
	context.Context,
	HarnessCommandRequest,
) (HarnessCommandResult, error) {
	return HarnessCommandResult{}, ErrHarnessProcessUnavailable
}

type codexAgentInputSourceFixture struct {
	called bool
}

func (source *codexAgentInputSourceFixture) NextAgentInput(
	_ context.Context,
	checkpoint loomruntime.AgentInputCheckpoint,
) (loomruntime.AgentInputBatch, bool, error) {
	if !loomruntime.ValidAgentInputCheckpoint(checkpoint) {
		return loomruntime.AgentInputBatch{}, false, ErrHarnessProtocol
	}
	if source.called {
		return loomruntime.AgentInputBatch{}, false, nil
	}
	source.called = true
	return loomruntime.AgentInputBatch{
		TurnID: "turn-2", TurnSequence: 2,
		StepID: "step-2", StepSequence: 1,
		Inputs: []loomruntime.AgentInput{{
			Binding: agentinbox.Binding{
				Mode: agentinbox.ModeSteer, ContextScope: agentinbox.ScopeAgentPrivate,
			},
			Content: []byte("continue privately"),
		}},
	}, true, nil
}

type codexNoAgentInputSourceFixture struct{}

func (*codexNoAgentInputSourceFixture) NextAgentInput(
	context.Context,
	loomruntime.AgentInputCheckpoint,
) (loomruntime.AgentInputBatch, bool, error) {
	return loomruntime.AgentInputBatch{}, false, nil
}
