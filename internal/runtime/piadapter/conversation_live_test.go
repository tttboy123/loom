package piadapter

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"loom-pi-rebuild/internal/controltool"
)

const (
	piConversationLiveGate          = "LOOM_PHASE7_PI_ADAPTER_PROBE"
	piConversationLiveExecutable    = "LOOM_PHASE7_PI_EXECUTABLE"
	piConversationLiveBaseURL       = "LOOM_PHASE7_PI_BASE_URL"
	piConversationLiveSearchPath    = "LOOM_PHASE7_PI_SEARCH_PATH"
	piConversationLiveToolLimit     = "LOOM_PHASE7_PI_TOOL_LIMIT"
	piConversationLiveToolOffset    = "LOOM_PHASE7_PI_TOOL_OFFSET"
	piConversationLiveObjectiveMark = "LOOM_PHASE7_PI_OBJECTIVE_MARKER"
	piConversationLiveExpectedReply = "Loom-P7-PI-ADAPTER-OK"
)

// TestLivePiRPCConversationAdapter isolates the real Pi RPC process and local
// model from daemon routing. Failure text contains protocol shape only; it
// never includes the prompt, model response, stderr, or generated catalog.
func TestLivePiRPCConversationAdapter(t *testing.T) {
	if os.Getenv(piConversationLiveGate) != "1" {
		t.Skip("Pi RPC adapter probe requires " + piConversationLiveGate + "=1")
	}
	executable := os.Getenv(piConversationLiveExecutable)
	baseURL := os.Getenv(piConversationLiveBaseURL)
	searchPaths := filepath.SplitList(os.Getenv(piConversationLiveSearchPath))
	if executable == "" || baseURL == "" || len(searchPaths) == 0 {
		t.Fatal("Pi RPC adapter probe requires executable, base URL, and search path")
	}
	privateRoot := t.TempDir()
	if err := os.Chmod(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewPiRPCConversationAdapter(PiRPCConversationAdapterConfig{
		Execution: PiExecutionAdapterConfig{
			ExecutablePath:     executable,
			RuntimeInstanceID:  "runtime.pi.phase7-live-probe",
			RuntimeSearchPaths: searchPaths,
			CancelGrace:        3 * time.Second,
			Now:                func() time.Time { return time.Now().UTC() },
			Random:             rand.Reader,
		},
		ProviderID:        piRPCProviderID,
		ModelID:           piRPCModelID,
		BaseURL:           baseURL,
		PrivateRoot:       privateRoot,
		MaxAssistantBytes: 4_096,
	})
	if err != nil {
		t.Fatalf("construct Pi RPC adapter: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	response, err := adapter.Respond(ctx, PiRPCConversationRequest{
		ThreadID: "phase7-pi-adapter-live-probe",
		Messages: []PiRPCConversationMessage{{
			Role: "user", Content: "Reply with exactly: " + piConversationLiveExpectedReply + ".",
		}},
	})
	if err != nil {
		t.Fatalf("Pi RPC adapter probe: %v", err)
	}
	if !strings.Contains(response.Content, piConversationLiveExpectedReply) {
		t.Fatalf("Pi RPC adapter reply missing marker; bytes=%d", len(response.Content))
	}
}

// TestLivePiRPCConversationControlAdapter adds the generated control extension
// while keeping the request text-only. This separates extension loading and
// control-aware RPC parsing from model tool-selection quality.
func TestLivePiRPCConversationControlAdapter(t *testing.T) {
	if os.Getenv(piConversationLiveGate) != "1" {
		t.Skip("Pi RPC adapter probe requires " + piConversationLiveGate + "=1")
	}
	controlServer := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		_ *http.Request,
	) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response,
			`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"ok"}]}}`)
	}))
	defer controlServer.Close()

	control := PiRPCConversationControlConfig{
		URL: controlServer.URL + "/mcp", Token: strings.Repeat("a", 64),
		Tools: []PiRPCConversationControlTool{{
			Name: "loom_runtimes_status", Description: "Read bounded Runtime status.",
			InputSchema: json.RawMessage(
				`{"type":"object","additionalProperties":false,"properties":{}}`,
			),
		}},
	}
	prepared, err := preparePiConversationControlConfig(control)
	if err != nil {
		t.Fatalf("prepare Pi RPC control probe: %v", err)
	}
	selectionPrompt, err := buildPiRPCConversationControlSystemPrompt(prepared.Tools)
	if err != nil || len(selectionPrompt) > piRPCMaxSystemPromptBytes {
		t.Fatalf(
			"build Pi RPC control probe prompt: bytes=%d error=%v",
			len(selectionPrompt), err,
		)
	}
	adapter := newLivePiRPCConversationAdapter(t, &control)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	response, err := adapter.Respond(ctx, PiRPCConversationRequest{
		ThreadID: "phase7-pi-control-live-probe",
		Messages: []PiRPCConversationMessage{{
			Role: "user", Content: "Reply briefly without using a tool.",
		}},
	})
	if err != nil {
		t.Fatalf("Pi RPC control adapter probe: %v", err)
	}
	if response.Content == "" {
		t.Fatal("Pi RPC control adapter returned an empty reply")
	}
}

// TestLivePiRPCFullControlRegistryAdapter reproduces the installed product
// shape: every built-in Loom control tool is registered for one Pi turn.
func TestLivePiRPCFullControlRegistryAdapter(t *testing.T) {
	if os.Getenv(piConversationLiveGate) != "1" {
		t.Skip("Pi RPC adapter probe requires " + piConversationLiveGate + "=1")
	}
	var controlCalls atomic.Int64
	controlServer := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		_ *http.Request,
	) {
		controlCalls.Add(1)
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response,
			`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"ok"}]}}`)
	}))
	defer controlServer.Close()
	upstream, err := url.Parse(os.Getenv(piConversationLiveBaseURL))
	if err != nil || upstream.Scheme != "http" || upstream.Host == "" {
		t.Fatal("Pi RPC full control Registry probe requires a valid local base URL")
	}
	target := *upstream
	target.Path = ""
	proxy := httputil.NewSingleHostReverseProxy(&target)
	var requestBytes atomic.Int64
	var responseStatus atomic.Int64
	proxy.ModifyResponse = func(response *http.Response) error {
		responseStatus.Store(int64(response.StatusCode))
		return nil
	}
	providerProxy := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		requestBytes.Store(request.ContentLength)
		proxy.ServeHTTP(response, request)
	}))
	defer providerProxy.Close()
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	definitions := registry.Definitions()
	offset := 0
	if rawOffset := os.Getenv(piConversationLiveToolOffset); rawOffset != "" {
		parsedOffset, parseErr := strconv.Atoi(rawOffset)
		if parseErr != nil || parsedOffset < 0 || parsedOffset >= len(definitions) {
			t.Fatal("invalid " + piConversationLiveToolOffset)
		}
		offset = parsedOffset
		definitions = definitions[offset:]
	}
	if rawLimit := os.Getenv(piConversationLiveToolLimit); rawLimit != "" {
		limit, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || limit < 1 || limit > len(definitions) {
			t.Fatal("invalid " + piConversationLiveToolLimit)
		}
		definitions = definitions[:limit]
	}
	tools := livePiControlTools(definitions)
	adapter := newLivePiRPCConversationAdapterAtBaseURL(
		t,
		&PiRPCConversationControlConfig{
			URL: controlServer.URL + "/mcp", Token: strings.Repeat("b", 64), Tools: tools,
		},
		providerProxy.URL+"/v1",
	)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	response, err := adapter.Respond(ctx, PiRPCConversationRequest{
		ThreadID: "phase7-pi-full-control-live-probe",
		Messages: []PiRPCConversationMessage{{
			Role: "user", Content: "Reply briefly without using a tool.",
		}},
	})
	if err != nil {
		t.Fatalf(
			"Pi RPC full control Registry probe: offset=%d tools=%d request_bytes=%d provider_status=%d: %v",
			offset, len(tools), requestBytes.Load(), responseStatus.Load(), err,
		)
	}
	if response.Content == "" {
		t.Fatal("Pi RPC full control Registry returned an empty reply")
	}
	if controlCalls.Load() != 0 {
		t.Fatalf("Pi direct reply called the product control Gateway %d times", controlCalls.Load())
	}
}

// TestLivePiRPCFullControlRegistrySelectsMissionTool proves that the local
// model can select a typed Loom tool after the compatibility projection.
func TestLivePiRPCFullControlRegistrySelectsMissionTool(t *testing.T) {
	objectiveMarker := strings.TrimSpace(os.Getenv(piConversationLiveObjectiveMark))
	if objectiveMarker == "" {
		objectiveMarker = "pi cancel governance check"
	}
	if !validPiRPCConversationText(objectiveMarker) || len(objectiveMarker) > 128 {
		t.Fatal("Pi RPC Mission selection requires a valid objective marker")
	}
	testLivePiRPCFullControlRegistrySelectsTool(t, piLiveControlSelectionProbe{
		threadID: "phase7-pi-full-control-mission-probe",
		prompt: "请实际使用 Loom 能力，准备一个目标为 P7 " + objectiveMarker +
			" verification 的 Mission 审阅提案。只准备这一项，不要执行。",
		expectedTool: "loom_missions_create_preview",
		argumentKey:  "objective",
		argumentMark: objectiveMarker,
	})
}

func TestLivePiRPCFullControlRegistrySelectsRouteTool(t *testing.T) {
	const profileID = "conversation-loom-local-pi-default-v1"
	testLivePiRPCFullControlRegistrySelectsTool(t, piLiveControlSelectionProbe{
		threadID:      "phase7-pi-full-control-route-probe",
		contextPrompt: `{"kind":"loom_role_context","conversation_id":"conversation:p7-route-distractor","items":[{"trust":"authoritative","content":"capsule-profile-distractor"}]}`,
		prompt: "请实际使用 Loom 能力，准备把当前会话 Route 选择为 " + profileID +
			" 的审阅提案。只准备这一项，不要执行。",
		expectedTool:  "loom_conversation_route_change_preview",
		argumentKey:   "profile_id",
		argumentMark:  profileID,
		argumentExact: true,
	})
}

func TestLivePiRPCFullControlRegistrySelectsTeamEditTool(t *testing.T) {
	const teamID = "team-instance-phase7-probe"
	testLivePiRPCFullControlRegistrySelectsTool(t, piLiveControlSelectionProbe{
		threadID: "phase7-pi-full-control-team-edit-probe",
		prompt: "请实际使用 Loom 能力，为 Agent Team " + teamID +
			" 准备编辑审阅提案，编辑指导必须精确为 Review role bindings。只准备这一项，不要执行。",
		expectedTool:  "loom_teams_edit_preview",
		argumentKey:   "team_instance_id",
		argumentMark:  teamID,
		argumentExact: true,
	})
}

func TestLivePiRPCFullControlRegistrySelectsRoundTableSkipTool(t *testing.T) {
	const (
		sessionID = "roundtable-phase7-probe"
		roundID   = "round-phase7-probe"
		seatID    = "seat-phase7-probe"
	)
	testLivePiRPCFullControlRegistrySelectsTool(t, piLiveControlSelectionProbe{
		threadID: "phase7-pi-full-control-roundtable-skip-probe",
		prompt: "请实际使用 Loom 能力，为 RoundTable Session " + sessionID + "、Round " + roundID + " 的 Seat " + seatID +
			" 准备 skip 审阅提案。只准备这一项，不要执行。",
		expectedTool:  "loom_roundtables_skip_preview",
		argumentKey:   "seat_id",
		argumentMark:  seatID,
		argumentExact: true,
		expectedArguments: map[string]string{
			"session_id": sessionID,
			"round_id":   roundID,
			"seat_id":    seatID,
		},
	})
}

type piLiveControlSelectionProbe struct {
	threadID          string
	contextPrompt     string
	prompt            string
	expectedTool      string
	argumentKey       string
	argumentMark      string
	argumentExact     bool
	expectedArguments map[string]string
}

func testLivePiRPCFullControlRegistrySelectsTool(
	t *testing.T,
	probe piLiveControlSelectionProbe,
) {
	t.Helper()
	if os.Getenv(piConversationLiveGate) != "1" {
		t.Skip("Pi RPC adapter probe requires " + piConversationLiveGate + "=1")
	}
	if !validPiRPCConversationThreadID(probe.threadID) ||
		!validPiRPCConversationText(probe.prompt) ||
		!validPiConversationControlName(probe.expectedTool) ||
		!validPiConversationControlName(probe.argumentKey) ||
		probe.argumentMark == "" {
		t.Fatal("invalid Pi live control selection probe")
	}
	type selectedCall struct {
		name      string
		argument  string
		arguments map[string]string
	}
	selected := make(chan selectedCall, 1)
	var controlCalls atomic.Int64
	controlServer := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		controlCalls.Add(1)
		var envelope struct {
			Method string `json:"method"`
			Params struct {
				Name      string                     `json:"name"`
				Arguments map[string]json.RawMessage `json:"arguments"`
			} `json:"params"`
		}
		if json.NewDecoder(request.Body).Decode(&envelope) == nil {
			var argument string
			_ = json.Unmarshal(envelope.Params.Arguments[probe.argumentKey], &argument)
			arguments := make(map[string]string, len(envelope.Params.Arguments))
			for field, raw := range envelope.Params.Arguments {
				var value string
				if json.Unmarshal(raw, &value) == nil {
					arguments[field] = value
				}
			}
			select {
			case selected <- selectedCall{
				name: envelope.Params.Name, argument: argument, arguments: arguments,
			}:
			default:
			}
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response,
			`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"Mission proposal prepared."}],"structuredContent":{"kind":"probe"},"isError":false}}`)
	}))
	defer controlServer.Close()
	upstream, err := url.Parse(os.Getenv(piConversationLiveBaseURL))
	if err != nil || upstream.Scheme != "http" || upstream.Host == "" {
		t.Fatal("Pi RPC Mission selection requires a valid local base URL")
	}
	target := *upstream
	target.Path = ""
	proxy := httputil.NewSingleHostReverseProxy(&target)
	var observedToolCount atomic.Int64
	var observedToolChoice atomic.Value
	var observedSamplingDrift atomic.Bool
	providerProxy := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		body, readErr := io.ReadAll(io.LimitReader(request.Body, 64<<10))
		if readErr != nil {
			http.Error(response, "proxy_unavailable", http.StatusBadGateway)
			return
		}
		defer zeroPiRPCBytes(body)
		var payload struct {
			ToolChoice  string            `json:"tool_choice"`
			Tools       []json.RawMessage `json:"tools"`
			Temperature *float64          `json:"temperature"`
		}
		if json.Unmarshal(body, &payload) == nil {
			observedToolCount.Store(int64(len(payload.Tools)))
			observedToolChoice.Store(payload.ToolChoice)
			if payload.Temperature == nil || *payload.Temperature != 0 {
				observedSamplingDrift.Store(true)
			}
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		request.ContentLength = int64(len(body))
		proxy.ServeHTTP(response, request)
	}))
	defer providerProxy.Close()
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	definitions := registry.Definitions()
	adapter := newLivePiRPCConversationAdapterAtBaseURL(
		t,
		&PiRPCConversationControlConfig{
			URL: controlServer.URL + "/mcp", Token: strings.Repeat("c", 64),
			Tools: livePiControlTools(definitions),
		},
		providerProxy.URL+"/v1",
	)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	response, err := adapter.Respond(ctx, PiRPCConversationRequest{
		ThreadID: probe.threadID, ContextPrompt: probe.contextPrompt,
		Messages: []PiRPCConversationMessage{{
			Role: "user", Content: probe.prompt,
		}},
	})
	if err != nil {
		selectedName := "none"
		select {
		case call := <-selected:
			selectedName = call.name
		default:
		}
		choice := ""
		if value := observedToolChoice.Load(); value != nil {
			choice, _ = value.(string)
		}
		t.Fatalf(
			"Pi RPC full Registry selection: selected=%s tool_choice=%s tools=%d: %v",
			selectedName, choice, observedToolCount.Load(), err,
		)
	}
	if response.Content == "" {
		t.Fatal("Pi RPC Mission selection returned an empty reply")
	}
	choice := ""
	if value := observedToolChoice.Load(); value != nil {
		choice, _ = value.(string)
	}
	if observedToolCount.Load() != 0 || choice != "" && choice != "none" {
		t.Fatalf(
			"Pi provider request retained native tool authority: tool_choice=%q tools=%d",
			choice, observedToolCount.Load(),
		)
	}
	select {
	case call := <-selected:
		argumentMatches := strings.EqualFold(call.argument, probe.argumentMark)
		if !probe.argumentExact {
			argumentMatches = strings.Contains(
				strings.ToLower(call.argument), strings.ToLower(probe.argumentMark),
			)
		}
		exactArguments := true
		for field, want := range probe.expectedArguments {
			if call.arguments[field] != want {
				exactArguments = false
				break
			}
		}
		if call.name != probe.expectedTool || !argumentMatches || !exactArguments {
			t.Fatalf(
				"Pi selected control tool %q with argument_match=%t exact_arguments=%t",
				call.name, argumentMatches, exactArguments,
			)
		}
	default:
		diagnosticContext, diagnosticCancel := context.WithTimeout(
			context.Background(), 90*time.Second,
		)
		defer diagnosticCancel()
		domain, exact, stage := diagnoseLivePiControlSelection(
			diagnosticContext, adapter, probe.threadID, probe.prompt,
		)
		t.Fatalf(
			"Pi did not select a Loom control tool; domain=%s exact=%s diagnostic_stage=%s",
			domain, exact, stage,
		)
	}
	if observedSamplingDrift.Load() {
		t.Fatal("Pi control selection retained nondeterministic sampling")
	}
	if controlCalls.Load() != 1 {
		t.Fatalf("Pi selected tool called the product control Gateway %d times", controlCalls.Load())
	}
}

// diagnoseLivePiControlSelection reports only the selected semantic names and
// failure stage. It deliberately excludes prompts, model output, and arguments.
func diagnoseLivePiControlSelection(
	ctx context.Context,
	adapter *PiRPCConversationAdapter,
	threadID string,
	promptText string,
) (domainName string, exactName string, failureStage string) {
	if adapter == nil || adapter.domains == nil || adapter.control == nil {
		return "none", "none", "adapter"
	}
	selectionPrompt, err := buildPiRPCConversationSelectionPrompt(
		"", []PiRPCConversationMessage{{Role: "user", Content: promptText}},
	)
	if err != nil {
		return "none", "none", "prompt"
	}
	domainContent, err := adapter.runConversationStage(
		ctx, selectionPrompt, piRPCConversationRunStage{
			systemPrompt: adapter.systemPrompt,
			selection:    adapter.domains,
		},
	)
	if err != nil {
		return "none", "none", "domain_run"
	}
	domain, err := adapter.resolveConversationControlSelection(
		domainContent, adapter.domains,
	)
	if err != nil {
		return "none", "none", "domain_resolve"
	}
	domainControl, err := piConversationControlToolsForDomain(
		*adapter.control, domain.Name,
	)
	if err != nil {
		return domain.Name, "none", "domain_tools"
	}
	selectionSystemPrompt, err := buildPiRPCConversationControlSystemPrompt(
		domainControl.Tools,
	)
	if err != nil {
		return domain.Name, "none", "exact_prompt"
	}
	exactContent, err := adapter.runConversationStage(
		ctx, selectionPrompt, piRPCConversationRunStage{
			systemPrompt: selectionSystemPrompt,
			selection:    &domainControl,
		},
	)
	if err != nil {
		return domain.Name, "none", "exact_run"
	}
	exact, err := adapter.resolveConversationControlSelection(
		exactContent, &domainControl,
	)
	if err != nil {
		return domain.Name, "none", "exact_resolve"
	}
	_ = threadID // The diagnostic intentionally starts a fresh stateless stage.
	return domain.Name, exact.Name, "none"
}

func livePiControlTools(
	definitions []controltool.Definition,
) []PiRPCConversationControlTool {
	tools := make([]PiRPCConversationControlTool, len(definitions))
	for index, definition := range definitions {
		tools[index] = PiRPCConversationControlTool{
			Name: definition.MCPName, Description: definition.Description,
			InputSchema: append(json.RawMessage(nil), definition.InputSchema...),
		}
	}
	return tools
}

func newLivePiRPCConversationAdapter(
	t *testing.T,
	control *PiRPCConversationControlConfig,
) *PiRPCConversationAdapter {
	t.Helper()
	return newLivePiRPCConversationAdapterAtBaseURL(
		t, control, os.Getenv(piConversationLiveBaseURL),
	)
}

func newLivePiRPCConversationAdapterAtBaseURL(
	t *testing.T,
	control *PiRPCConversationControlConfig,
	baseURL string,
) *PiRPCConversationAdapter {
	t.Helper()
	executable := os.Getenv(piConversationLiveExecutable)
	searchPaths := filepath.SplitList(os.Getenv(piConversationLiveSearchPath))
	if executable == "" || baseURL == "" || len(searchPaths) == 0 {
		t.Fatal("Pi RPC adapter probe requires executable, base URL, and search path")
	}
	privateRoot := t.TempDir()
	if err := os.Chmod(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewPiRPCConversationAdapter(PiRPCConversationAdapterConfig{
		Execution: PiExecutionAdapterConfig{
			ExecutablePath:     executable,
			RuntimeInstanceID:  "runtime.pi.phase7-live-probe",
			RuntimeSearchPaths: searchPaths,
			CancelGrace:        3 * time.Second,
			Now:                func() time.Time { return time.Now().UTC() },
			Random:             rand.Reader,
		},
		ProviderID:        piRPCProviderID,
		ModelID:           piRPCModelID,
		BaseURL:           baseURL,
		PrivateRoot:       privateRoot,
		MaxAssistantBytes: 4_096,
		Control:           control,
	})
	if err != nil {
		t.Fatalf("construct Pi RPC adapter: %v", err)
	}
	return adapter
}
