//go:build unix

package piadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/controltool"
)

func TestPiConversationControlExtensionRegistersExactToolsWithoutPersistingToken(t *testing.T) {
	root := piPrivateDirectory(t, "conversation-control")
	control := PiRPCConversationControlConfig{
		URL:   "http://127.0.0.1:18429/mcp",
		Token: strings.Repeat("a", 64),
		Tools: []PiRPCConversationControlTool{
			{
				Name: "loom_sessions_search", Description: "Search Loom conversations.",
				InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"query":{"type":"string"}},"required":["query"]}`),
			},
			{
				Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
				InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"objective":{"type":"string","minLength":1,"maxLength":4096}},"required":["objective"]}`),
			},
		},
	}
	prepared, err := preparePiConversationControlConfig(control)
	if err != nil {
		t.Fatalf("preparePiConversationControlConfig() error = %v", err)
	}
	systemPrompt, err := buildPiRPCConversationControlSystemPrompt(prepared.Tools)
	if err != nil {
		t.Fatalf("buildPiRPCConversationControlSystemPrompt() error = %v", err)
	}
	if len(systemPrompt) > piRPCMaxSystemPromptBytes {
		t.Fatalf("Pi system prompt bytes = %d", len(systemPrompt))
	}
	argumentPrompt, err := buildPiRPCConversationArgumentSystemPrompt(control.Tools[1])
	if err != nil || !strings.Contains(
		argumentPrompt, "loom_missions_create_preview={objective}",
	) {
		t.Fatalf("Pi argument prompt = %q, %v", argumentPrompt, err)
	}
	extension, err := newPiConversationControlExtension(
		root, "56565656-5656-4656-8656-565656565656", prepared,
	)
	if err != nil {
		t.Fatalf("newPiConversationControlExtension() error = %v", err)
	}
	path := extension.extensionPath
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("extension mode = %v", info.Mode())
	}
	for _, required := range []string{
		`name: "loom_sessions_search"`,
		`name: "loom_missions_create_preview"`,
		`additionalProperties`,
		`name: "loom_conversation_reply"`,
		`response_format`,
		`temperature: 0`,
		`loom_control_selection`,
		`tool_choice: "none"`,
		`throw new Error("selection_only")`,
	} {
		if !strings.Contains(string(source), required) {
			t.Fatalf("extension source missing %q: %s", required, source)
		}
	}
	if strings.Contains(string(source), `"minLength"`) ||
		strings.Contains(string(source), `"maxLength"`) ||
		!strings.Contains(string(control.Tools[1].InputSchema), `"minLength":1`) {
		t.Fatalf("extension did not project unsupported model schema safely: %s", source)
	}
	if len(control.Tools) != 2 || len(prepared.Tools) != 3 ||
		prepared.Tools[2].Name != "loom_conversation_reply" {
		t.Fatalf("control preparation mutated authority or omitted direct reply: original=%d prepared=%#v", len(control.Tools), prepared.Tools)
	}
	if strings.Contains(string(source), control.Token) ||
		strings.Contains(string(source), control.URL) ||
		strings.Contains(string(source), "LOOM_CONTROL_TURN_TOKEN") ||
		strings.Contains(string(source), "fetch(") ||
		strings.Contains(string(source), "loom_control_*") {
		t.Fatalf("selection-only extension retained execution authority: %s", source)
	}
	extension.Close()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("extension path survived Close(): %v", err)
	}
	argumentExtension, err := newPiConversationArgumentExtension(
		root, "58585858-5858-4858-8858-585858585858", control.Tools[1],
	)
	if err != nil {
		t.Fatal(err)
	}
	argumentPath := argumentExtension.extensionPath
	argumentSource, err := os.ReadFile(argumentPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(argumentSource), "loom_control_arguments") ||
		!strings.Contains(string(argumentSource), `"objective"`) ||
		!strings.Contains(string(argumentSource), "temperature: 0") ||
		strings.Contains(string(argumentSource), control.Token) ||
		strings.Contains(string(argumentSource), control.URL) ||
		strings.Contains(string(argumentSource), "LOOM_CONTROL_TURN_TOKEN") ||
		strings.Contains(string(argumentSource), "fetch(") {
		t.Fatalf("argument extension retained authority or lost schema: %s", argumentSource)
	}
	argumentExtension.Close()
	if _, err := os.Lstat(argumentPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("argument extension path survived Close(): %v", err)
	}
}

func TestPiConversationControlSelectionPromptIncludesBoundedRegistryPurposes(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	definitions := registry.Definitions()
	tools := make([]PiRPCConversationControlTool, len(definitions))
	for index, definition := range definitions {
		tools[index] = PiRPCConversationControlTool{
			Name: definition.MCPName, Description: definition.Description,
			InputSchema: append(json.RawMessage(nil), definition.InputSchema...),
		}
	}
	prepared, err := preparePiConversationControlConfig(PiRPCConversationControlConfig{
		URL: "http://127.0.0.1:18429/mcp", Token: strings.Repeat("d", 64),
		Tools: tools,
	})
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := buildPiRPCConversationControlSystemPrompt(prepared.Tools)
	if err != nil {
		t.Fatal(err)
	}
	if len(prompt) > piRPCMaxSystemPromptBytes {
		t.Fatalf("semantic selection prompt bytes = %d", len(prompt))
	}
	for _, required := range []string{
		"Semantic tool catalog",
		"Quoted titles and IDs are argument data",
		"loom_missions_create_preview=missions create/创建 [objective]",
		"loom_missions_continue_preview=missions continue/继续 [guidance,mission_id]",
		"loom_conversation_route_change_preview=conversation route change/切换 [profile_id]",
		"loom_teams_edit_preview=teams edit/编辑",
		"loom_conversation_reply=Choose this only when the latest request",
		"Brackets list required argument fields",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("semantic selection prompt missing %q", required)
		}
	}
}

func TestPiConversationControlDomainsPartitionFrozenRegistry(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	definitions := registry.Definitions()
	tools := make([]PiRPCConversationControlTool, len(definitions))
	for index, definition := range definitions {
		tools[index] = PiRPCConversationControlTool{
			Name: definition.MCPName, Description: definition.Description,
			InputSchema: append(json.RawMessage(nil), definition.InputSchema...),
		}
	}
	prepared, err := preparePiConversationControlConfig(PiRPCConversationControlConfig{
		URL: "http://127.0.0.1:18429/mcp", Token: strings.Repeat("e", 64),
		Tools: tools,
	})
	if err != nil {
		t.Fatal(err)
	}
	domains, err := preparePiConversationControlDomains(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if len(domains.Tools) != 5 {
		t.Fatalf("Pi control domains = %#v", domains.Tools)
	}
	prompt, err := buildPiRPCConversationDomainSystemPrompt(domains)
	if err != nil || len(prompt) > piRPCMaxSystemPromptBytes {
		t.Fatalf("Pi domain prompt bytes=%d error=%v", len(prompt), err)
	}
	for _, required := range []string{
		"loom_domain_conversation=Session search/align, route/model/reasoning/workspace",
		"loom_domain_missions=Mission search/create/continue blocked work",
		"Routing categories only; they have no product effect",
		"Conversation/Session/会话/对话 Route/model/模型/reasoning/推理/workspace -> loom_domain_conversation",
		"Agent Team/智能体团队/团队/team_instance_id with search/status/create/edit/编辑 -> loom_domain_teams",
		"Being requested from a Conversation does not make a Mission, Team, RoundTable, or environment operation a Conversation operation",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("Pi domain prompt missing %q: %s", required, prompt)
		}
	}
	seen := make(map[string]string, len(prepared.Tools)-1)
	for _, domain := range domains.Tools {
		subset, subsetErr := piConversationControlToolsForDomain(prepared, domain.Name)
		if subsetErr != nil || len(subset.Tools) < 2 ||
			subset.Tools[len(subset.Tools)-1].Name != "loom_conversation_reply" {
			t.Fatalf("Pi domain %s subset=%#v error=%v", domain.Name, subset.Tools, subsetErr)
		}
		source, sourceErr := piConversationControlExtensionSource(subset)
		if sourceErr != nil || !strings.Contains(string(source), "loom_conversation_reply") {
			t.Fatalf("Pi domain %s source lost bounded direct reply: %v", domain.Name, sourceErr)
		}
		for _, tool := range subset.Tools[:len(subset.Tools)-1] {
			if previous, duplicate := seen[tool.Name]; duplicate {
				t.Fatalf("Pi tool %s appears in %s and %s", tool.Name, previous, domain.Name)
			}
			seen[tool.Name] = domain.Name
		}
		if domain.Name == "loom_domain_missions" {
			missionPrompt, promptErr := buildPiRPCConversationControlSystemPrompt(subset.Tools)
			if promptErr != nil ||
				!strings.Contains(missionPrompt, "Mission create accepts a new objective") ||
				!strings.Contains(missionPrompt, "Mission create/创建 is required when the user asks to prepare or propose a new Mission") ||
				!strings.Contains(missionPrompt, "Mission search/搜索 is only for finding or listing existing Missions") ||
				!strings.Contains(missionPrompt, "Mission continue requires an explicit existing mission_id and guidance") {
				t.Fatalf("Pi Mission selection guidance unavailable: %v: %s", promptErr, missionPrompt)
			}
		}
		if domain.Name == "loom_domain_teams" {
			teamPrompt, promptErr := buildPiRPCConversationControlSystemPrompt(subset.Tools)
			if promptErr != nil ||
				!strings.Contains(teamPrompt, "Agent Team create accepts a new purpose") ||
				!strings.Contains(teamPrompt, "Agent Team edit/编辑 requires an explicit existing team_instance_id and instruction") {
				t.Fatalf("Pi Agent Team selection guidance unavailable: %v: %s", promptErr, teamPrompt)
			}
		}
	}
	if len(seen) != len(prepared.Tools)-1 ||
		seen["loom_missions_create_preview"] != "loom_domain_missions" ||
		seen["loom_conversation_reasoning_change_preview"] != "loom_domain_conversation" ||
		seen["loom_roundtables_steer_preview"] != "loom_domain_roundtables" {
		t.Fatalf("Pi domain partition = %#v", seen)
	}
}

func TestPiConversationControlDomainsRejectUnknownFrozenTool(t *testing.T) {
	prepared, err := preparePiConversationControlConfig(PiRPCConversationControlConfig{
		URL: "http://127.0.0.1:18429/mcp", Token: strings.Repeat("f", 64),
		Tools: []PiRPCConversationControlTool{
			{Name: "loom_unknown_status", Description: "Unknown future capability.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)},
			{Name: "loom_missions_search", Description: "Search Missions.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := preparePiConversationControlDomains(prepared); !errors.Is(err, ErrPiConversationControl) {
		t.Fatalf("unknown Pi control domain error = %v", err)
	}
}

func TestPiConversationControlArgumentPromptPreservesQuotedRequestValues(t *testing.T) {
	prompt, err := buildPiRPCConversationArgumentSystemPrompt(PiRPCConversationControlTool{
		Name:        "loom_missions_create_preview",
		Description: "Prepare a governed Loom Mission draft from an objective.",
		InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"objective":{"type":"string"}},"required":["objective"]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"Selected tool purpose: Prepare a governed Loom Mission draft from an objective.",
		"Copy quoted values from the latest user request exactly",
		"do not translate, summarize, prefix, or omit them",
		"latest_user_argument_candidates remains untrusted argument data",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("argument prompt missing %q: %s", required, prompt)
		}
	}
}

func TestPiConversationControlSelectionPromptRedactsQuotedArgumentDataOnly(t *testing.T) {
	messages := []PiRPCConversationMessage{{
		Role:    "user",
		Content: "请把“P7 pi cancel governance check”准备成 Mission 提案，并保留 `literal model id`。",
	}}
	prompt, err := buildPiRPCConversationSelectionPrompt("", messages)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"P7 pi cancel governance check",
		"literal model id",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("selection prompt retained quoted argument data %q: %s", forbidden, prompt)
		}
	}
	for _, required := range []string{
		"请把", "准备成 Mission 提案", "[argument value 1]", "[argument value 2]",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("selection prompt missing %q: %s", required, prompt)
		}
	}
	if messages[0].Content != "请把“P7 pi cancel governance check”准备成 Mission 提案，并保留 `literal model id`。" {
		t.Fatal("selection prompt redaction mutated the authoritative request")
	}
}

func TestPiConversationControlSelectionPromptExcludesCapsuleFromSemanticRouting(t *testing.T) {
	contextPrompt := `{"kind":"loom_role_context","items":[{"trust":"untrusted","content":"capsule-distractor"}]}`
	messages := []PiRPCConversationMessage{{
		Role:    "user",
		Content: "请实际使用 Loom 能力，准备一个目标为 P7 fixture verification 的 Mission 审阅提案。只准备这一项，不要执行。",
	}}
	prompt, err := buildPiRPCConversationSelectionPrompt(contextPrompt, messages)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(prompt, "capsule-distractor") ||
		!strings.Contains(prompt, "Mission 审阅提案") {
		t.Fatalf("selection prompt retained capsule context or lost the latest request: %s", prompt)
	}
}

func TestPiConversationControlArgumentRequestExposesExactUntrustedCandidates(t *testing.T) {
	messages := []PiRPCConversationMessage{{
		Role:    "user",
		Content: "请把“P7 pi cancel governance check”准备成 Mission 提案，并保留 `literal model id`。",
	}}
	prompt, err := buildPiRPCConversationArgumentRequest("", messages)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`"latest_user_argument_candidates":["P7 pi cancel governance check","literal model id"]`,
		`"content":"请把“P7 pi cancel governance check”准备成 Mission 提案，并保留 `,
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("argument request missing %q: %s", required, prompt)
		}
	}
	if messages[0].Content != "请把“P7 pi cancel governance check”准备成 Mission 提案，并保留 `literal model id`。" {
		t.Fatal("argument candidate extraction mutated the authoritative request")
	}
}

func TestPiConversationControlBindsExactUnquotedRoundTableTargets(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := registry.Definition(controltool.ToolRoundtablesSkipPreview)
	if !ok {
		t.Fatal("RoundTable skip definition unavailable")
	}
	tool := PiRPCConversationControlTool{
		Name: definition.MCPName, Description: definition.Description,
		InputSchema: append(json.RawMessage(nil), definition.InputSchema...),
	}
	bound, bindings, err := bindPiConversationExplicitArgumentTargets(
		tool,
		"请实际使用 Loom 能力，为 RoundTable Session rt-session-17、Round round-29 的 Seat seat-coder-3 准备 skip 审阅提案。只准备这一项，不要执行。",
	)
	if err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]string{
		"session_id": "rt-session-17",
		"round_id":   "round-29",
		"seat_id":    "seat-coder-3",
	} {
		values := bindings[field]
		if len(values) != 1 || values[0] != want {
			t.Fatalf("binding %s = %#v, want %q", field, values, want)
		}
	}
	if bytes.Equal(bound.InputSchema, tool.InputSchema) {
		t.Fatal("bound argument schema did not freeze explicit targets")
	}
	fields, err := piRPCObject(bound.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	properties, err := piRPCObject(fields["properties"])
	if err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]string{
		"session_id": "rt-session-17",
		"round_id":   "round-29",
		"seat_id":    "seat-coder-3",
	} {
		property, propertyErr := piRPCObject(properties[field])
		if propertyErr != nil {
			t.Fatal(propertyErr)
		}
		var enum []string
		if json.Unmarshal(property["enum"], &enum) != nil ||
			len(enum) != 1 || enum[0] != want {
			t.Fatalf("bound enum %s = %#v, want %q", field, enum, want)
		}
	}
	good, err := piRPCObject(json.RawMessage(
		`{"session_id":"rt-session-17","round_id":"round-29","seat_id":"seat-coder-3"}`,
	))
	if err != nil || validatePiConversationExplicitArgumentTargets(good, bindings) != nil {
		t.Fatalf("exact RoundTable targets rejected: %v", err)
	}
	bad, err := piRPCObject(json.RawMessage(
		`{"session_id":"rt-session-17","round_id":"rt-session-17","seat_id":"seat-coder-3"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	if validatePiConversationExplicitArgumentTargets(bad, bindings) == nil {
		t.Fatal("cross-field RoundTable target substitution was accepted")
	}
	if !bytes.Equal(tool.InputSchema, definition.InputSchema) {
		t.Fatal("explicit argument binding mutated the frozen Registry schema")
	}
}

func TestPiConversationControlArgumentRequestExcludesCapsuleFromArguments(t *testing.T) {
	contextPrompt := `{"kind":"loom_role_context","conversation_id":"conversation:p7-distractor","items":[{"trust":"authoritative","content":"capsule-profile-distractor"}]}`
	messages := []PiRPCConversationMessage{{
		Role:    "user",
		Content: "请准备把当前会话 Route 选择为 conversation-loom-local-pi-default-v1 的审阅提案。",
	}}
	prompt, err := buildPiRPCConversationArgumentRequest(contextPrompt, messages)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(prompt, "conversation:p7-distractor") ||
		strings.Contains(prompt, "capsule-profile-distractor") ||
		!strings.Contains(prompt, "conversation-loom-local-pi-default-v1") {
		t.Fatalf("argument request retained Capsule data or lost the latest request: %s", prompt)
	}
}

func TestPiConversationControlConfigFailsClosed(t *testing.T) {
	valid := PiRPCConversationControlConfig{
		URL:   "http://127.0.0.1:18429/mcp",
		Token: strings.Repeat("b", 64),
		Tools: []PiRPCConversationControlTool{{
			Name: "loom_sessions_search", Description: "Search Loom conversations.",
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
		}},
	}
	tests := []PiRPCConversationControlConfig{
		{},
		func() PiRPCConversationControlConfig {
			value := clonePiConversationControlConfig(valid)
			value.URL = "https://example.com/mcp"
			return value
		}(),
		func() PiRPCConversationControlConfig {
			value := clonePiConversationControlConfig(valid)
			value.Token += "c"
			return value
		}(),
		func() PiRPCConversationControlConfig {
			value := clonePiConversationControlConfig(valid)
			value.Tools[0].Name = "loom.sessions.search"
			return value
		}(),
		func() PiRPCConversationControlConfig {
			value := clonePiConversationControlConfig(valid)
			value.Tools = append(value.Tools, value.Tools[0])
			return value
		}(),
		func() PiRPCConversationControlConfig {
			value := clonePiConversationControlConfig(valid)
			value.Tools[0].InputSchema = json.RawMessage(`{"type":"object"}`)
			return value
		}(),
		func() PiRPCConversationControlConfig {
			value := clonePiConversationControlConfig(valid)
			value.Tools[0].Name = "loom_conversation_reply"
			return value
		}(),
	}
	for index, control := range tests {
		if _, err := preparePiConversationControlConfig(control); !errors.Is(err, ErrPiConversationControl) {
			t.Fatalf("invalid control[%d] error = %v", index, err)
		}
	}
	prepared, err := preparePiConversationControlConfig(valid)
	if err != nil {
		t.Fatal(err)
	}
	root := piPrivateDirectory(t, "conversation-control-invalid")
	if _, err := newPiConversationControlExtension(
		filepath.Clean(root), "not-a-uuid", prepared,
	); !errors.Is(err, ErrPiConversationControl) {
		t.Fatalf("invalid extension id error = %v", err)
	}
}

func TestPiRPCConversationAdapterRunsExactControlToolLoop(t *testing.T) {
	request := PiRPCConversationRequest{
		ThreadID: "thread-control",
		Messages: []PiRPCConversationMessage{{
			Role: "user", Content: "Create a Mission to ship the governed release.",
		}},
	}
	prompt, err := buildPiRPCConversationPrompt("", request.Messages)
	if err != nil {
		t.Fatal(err)
	}
	called := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		var envelope struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if json.NewDecoder(request.Body).Decode(&envelope) == nil {
			called <- envelope.Params.Name
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response,
			`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"Proposal ready."}],"structuredContent":{"requires_confirmation":true},"isError":false}}`)
	}))
	defer server.Close()
	control := PiRPCConversationControlConfig{
		URL: server.URL + "/mcp", Token: strings.Repeat("c", 64),
		Tools: []PiRPCConversationControlTool{{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"objective":{"type":"string"}},"required":["objective"]}`),
		}},
	}
	preparedControl, err := preparePiConversationControlConfig(control)
	if err != nil {
		t.Fatal(err)
	}
	systemPrompt, err := buildPiRPCConversationControlSystemPrompt(preparedControl.Tools)
	if err != nil {
		t.Fatal(err)
	}
	root := piPrivateDirectory(t, "rpc-conversation-control")
	executablePath := filepath.Join(root, "pi-control-fixture")
	script := piRPCConversationControlFixtureScript(
		t, prompt, systemPrompt, control, "loom_missions_create_preview",
	)
	if err := os.WriteFile(executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	privateRoot := piPrivateDirectoryAt(t, root, "private")
	adapter, err := NewPiRPCConversationAdapter(PiRPCConversationAdapterConfig{
		Execution: PiExecutionAdapterConfig{
			ExecutablePath: executablePath, RuntimeInstanceID: "runtime.pi.control",
			RuntimeSearchPaths: []string{piPrivateDirectoryAt(t, root, "search")},
			CancelGrace:        200 * time.Millisecond,
			Now:                func() time.Time { return managedPiTestNow },
			Random:             bytes.NewReader(bytes.Repeat([]byte{0x57}, 2048)),
		},
		ProviderID: piRPCProviderID, ModelID: piRPCModelID,
		BaseURL: "http://127.0.0.1:18427/v1", PrivateRoot: privateRoot,
		MaxAssistantBytes: 16_384, Control: &control,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := adapter.Respond(context.Background(), request)
	if err != nil {
		failure, _ := os.ReadFile(executablePath + ".failure")
		t.Fatalf("Respond() error = %v fixture_failure=%s", err, failure)
	}
	if response.Content != "I prepared a Loom proposal for your review." {
		t.Fatalf("Respond() content = %q", response.Content)
	}
	select {
	case name := <-called:
		if name != "loom_missions_create_preview" {
			t.Fatalf("MCP tool = %q", name)
		}
	default:
		t.Fatal("MCP tool was not called")
	}
	entries, err := os.ReadDir(privateRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("control invocation cleanup entries=%#v error=%v", entries, err)
	}
}

func TestPiConversationControlTransportAllowsTextSelectionEnvelope(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	runtimeAdapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	adapter := runtimeAdapter.(*piRPCBridgeAdapter)
	messageID := "direct-control-message-1"
	user := piRPCFixtureUserMessage(piRPCFixturePrompt)
	empty := piRPCFixtureAssistantMessage("")
	emptyText := piRPCFixtureWithResponseID(
		strings.Replace(empty, `"content":[]`, `"content":[{"type":"text","text":""}]`, 1),
		messageID,
	)
	first := piRPCFixtureWithResponseID(piRPCFixtureAssistantMessage("Hello "), messageID)
	final := piRPCFixtureWithResponseID(piRPCFixtureAssistantMessage("Hello world"), messageID)
	lines := []string{
		`{"id":"` + messageID + `","type":"response","command":"prompt","success":true}`,
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"message_start","message":` + user + `}`,
		`{"type":"message_end","message":` + user + `}`,
		`{"type":"message_start","message":` + empty + `}`,
		`{"type":"message_update","message":` + emptyText + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + emptyText + `}}`,
		`{"type":"message_update","message":` + first + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + first + `}}`,
		`{"type":"message_update","message":` + final + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"world","partial":` + final + `}}`,
		`{"type":"message_update","message":` + final + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"Hello world","partial":` + final + `}}`,
		`{"type":"message_end","message":` + final + `}`,
		`{"type":"turn_end","message":` + final + `,"toolResults":[]}`,
		`{"type":"agent_end","messages":[` + user + `,` + final + `],"willRetry":false}`,
		`{"type":"agent_settled"}`,
	}
	state := piRPCState{
		conversation: true, messageID: messageID,
		prompt: []byte(piRPCFixturePrompt),
	}
	for index, line := range lines {
		if err := adapter.acceptRPCLine(
			context.Background(), nil, &state, []byte(line),
		); err != nil {
			t.Fatalf("line[%d] error = %v: %s", index, err, line)
		}
	}
	if !state.settled || string(state.assistant) != "Hello world" {
		t.Fatalf("direct control state = %#v", state)
	}
}

func TestPiConversationControlTransportRejectsNativeToolCallBeforeBroker(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	runtimeAdapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	adapter := runtimeAdapter.(*piRPCBridgeAdapter)
	messageID := "control-message-1"
	user := piRPCFixtureUserMessage(piRPCFixturePrompt)
	empty := piRPCFixtureAssistantMessage("")
	unknownCall := piRPCConversationControlToolCallFixture(
		"unknown-call-1", "loom_unknown_preview", json.RawMessage(`{"value":"x"}`),
	)
	unknownAssistant := strings.Replace(
		empty, `"content":[]`, `"content":[`+unknownCall+`]`, 1,
	)
	unknownAssistant = strings.Replace(
		unknownAssistant, `"stopReason":"stop"`, `"stopReason":"toolUse"`, 1,
	)
	unknownAssistant = piRPCFixtureWithResponseID(unknownAssistant, "control-response-1")
	lines := []string{
		`{"id":"` + messageID + `","type":"response","command":"prompt","success":true}`,
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"message_start","message":` + user + `}`,
		`{"type":"message_end","message":` + user + `}`,
		`{"type":"message_start","message":` + empty + `}`,
		`{"type":"message_update","message":` + unknownAssistant + `,"assistantMessageEvent":{"type":"toolcall_start","contentIndex":0,"partial":` + unknownAssistant + `}}`,
		`{"type":"message_update","message":` + unknownAssistant + `,"assistantMessageEvent":{"type":"toolcall_end","contentIndex":0,"toolCall":` + unknownCall + `,"partial":` + unknownAssistant + `}}`,
	}
	state := piRPCState{
		conversation: true, messageID: messageID,
		prompt: []byte(piRPCFixturePrompt),
	}
	rejected := false
	for _, line := range lines {
		if err := adapter.acceptRPCLine(
			context.Background(), nil, &state, []byte(line),
		); err != nil {
			rejected = true
			break
		}
	}
	if !rejected {
		t.Fatal("native Pi control tool call reached the broker path")
	}
}

func piRPCConversationControlFixtureScript(
	t testing.TB,
	prompt string,
	systemPrompt string,
	control PiRPCConversationControlConfig,
	toolName string,
) string {
	t.Helper()
	messageID := "57575757-5757-4757-9757-575757575757"
	responseID := "control-response-1"
	var selectedTool PiRPCConversationControlTool
	for _, tool := range control.Tools {
		if tool.Name == toolName {
			selectedTool = tool
			break
		}
	}
	argumentPrompt, err := buildPiRPCConversationArgumentSystemPrompt(selectedTool)
	if err != nil {
		t.Fatal(err)
	}
	linesFor := func(content string) []string {
		empty := piRPCFixtureAssistantMessage("")
		user := piRPCFixtureUserMessage(prompt)
		emptyText := piRPCFixtureWithResponseID(
			strings.Replace(empty, `"content":[]`, `"content":[{"type":"text","text":""}]`, 1),
			responseID,
		)
		final := piRPCFixtureWithResponseID(piRPCFixtureAssistantMessage(content), responseID)
		return []string{
			`{"id":"` + messageID + `","type":"response","command":"prompt","success":true}`,
			`{"type":"agent_start"}`,
			`{"type":"turn_start"}`,
			`{"type":"message_start","message":` + user + `}`,
			`{"type":"message_end","message":` + user + `}`,
			`{"type":"message_start","message":` + empty + `}`,
			`{"type":"message_update","message":` + emptyText + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + emptyText + `}}`,
			`{"type":"message_update","message":` + final + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":` + mustPiJSONString(content) + `,"partial":` + final + `}}`,
			`{"type":"message_update","message":` + final + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":` + mustPiJSONString(content) + `,"partial":` + final + `}}`,
			`{"type":"message_end","message":` + final + `}`,
			`{"type":"turn_end","message":` + final + `,"toolResults":[]}`,
			`{"type":"agent_end","messages":[` + user + `,` + final + `],"willRetry":false}`,
			`{"type":"agent_settled"}`,
		}
	}
	selectionLines := linesFor(`{"tool_name":"` + toolName + `"}`)
	argumentLines := linesFor(`{"objective":"Ship the governed release"}`)
	expectedPrefix := []string{
		"--mode", "rpc", "--offline", "--no-approve", "--no-session", "--no-extensions",
		"--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files",
		"--provider", piRPCProviderID, "--model", piRPCProviderID + "/" + piRPCModelID,
		"--thinking", "off", "--system-prompt",
	}
	header := "#!/bin/sh\nfail() { printf '%s' \"$1\" > \"$0.failure\"; exit \"$1\"; }\n" +
		"[ -z \"${LOOM_CONTROL_TURN_TOKEN+x}\" ] || fail 61\n"
	for _, argument := range expectedPrefix {
		header += "[ \"$1\" = " + piRPCShellQuote(argument) + " ] || fail 62\nshift\n"
	}
	header += "system_prompt=$1\nshift\n" +
		"{ [ \"$system_prompt\" = " + piRPCShellQuote(systemPrompt) + " ] || [ \"$system_prompt\" = " + piRPCShellQuote(argumentPrompt) + " ]; } || fail 62\n" +
		"[ \"$1\" = --no-builtin-tools ] || fail 63\nshift\n" +
		"[ \"$1\" = --extension ] || fail 64\nshift\n" +
		"extension=$1\nshift\n[ \"$#\" -eq 0 ] || fail 65\n" +
		"[ -f \"$extension\" ] || fail 66\n" +
		"/usr/bin/grep -q " + piRPCShellQuote(`name: "`+toolName+`"`) + " \"$extension\" || fail 67\n" +
		"/usr/bin/grep -q " + piRPCShellQuote(control.Token) + " \"$extension\" && fail 68\n" +
		"/usr/bin/grep -q " + piRPCShellQuote(control.URL) + " \"$extension\" && fail 68\n"
	quoteLines := func(lines []string) string {
		quoted := make([]string, 0, len(lines))
		for _, line := range lines {
			quoted = append(quoted, piRPCShellQuote(line))
		}
		return strings.Join(quoted, " ")
	}
	return header + "read request || fail 69\n" +
		"if /usr/bin/grep -q loom_control_arguments \"$extension\"; then\n" +
		"  [ \"$system_prompt\" = " + piRPCShellQuote(argumentPrompt) + " ] || fail 70\n" +
		"  printf '%s\\n' " + quoteLines(argumentLines) + "\n" +
		"else\n" +
		"  [ \"$system_prompt\" = " + piRPCShellQuote(systemPrompt) + " ] || fail 71\n" +
		"  printf '%s\\n' " + quoteLines(selectionLines) + "\n" +
		"fi\n"
}

func piRPCConversationControlToolCallFixture(
	callID string,
	toolName string,
	arguments json.RawMessage,
) string {
	return `{"type":"toolCall","id":` + mustPiJSONString(callID) +
		`,"name":` + mustPiJSONString(toolName) + `,"arguments":` + string(arguments) + `}`
}

func mustPiJSONString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
