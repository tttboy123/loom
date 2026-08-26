package harnessadapter

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/permissions"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

var ErrHarnessContextMCP = errors.New("Harness scoped Context MCP unavailable")

const (
	harnessContextMCPTokenEnv       = "LOOM_CONTEXT_ATTEMPT_TOKEN"
	harnessContextMCPMaxRequest     = 72 << 10
	harnessContextMCPMaxContent     = 64 << 10
	harnessContextMCPMaxResponse    = 96 << 10
	harnessMCPToolMaxArguments      = 64 << 10
	harnessContextMCPMaxCalls       = 4
	harnessWorkspaceMCPMaxCalls     = 24
	harnessContextMCPShutdownBudget = 2 * time.Second
)

type HarnessContextMCPLease struct {
	URL              string
	Token            string
	ContextEnabled   bool
	ReadEnabled      bool
	GrepEnabled      bool
	EditEnabled      bool
	BashEnabled      bool
	WebSearchEnabled bool
	WebFetchEnabled  bool
	MCPToolEnabled   bool
}

type harnessPreparedDelivery struct {
	binding     attemptpayload.Binding
	context     bool
	toolBinding loomruntime.ToolCallBinding
	toolResult  loomruntime.ToolCallResult
}

type harnessAttemptMCPConfig struct {
	Retriever     contextcapsule.Retriever
	Delivery      contextcapsule.DeliveryBroker
	ToolGateway   loomruntime.AttemptToolGateway
	ToolBinding   loomruntime.ToolCallBinding
	AllowedTools  []permissions.ToolKind
	WorkspacePath string
}

type harnessContextMCP struct {
	lease          HarnessContextMCPLease
	attemptContext context.Context
	retriever      contextcapsule.Retriever
	delivery       contextcapsule.DeliveryBroker
	toolGateway    loomruntime.AttemptToolGateway
	toolBinding    loomruntime.ToolCallBinding
	workspacePath  string
	tools          map[string]permissions.ToolKind
	listener       net.Listener
	server         *http.Server
	mu             sync.Mutex
	calls          int
	maxCalls       int
	prepared       []harnessPreparedDelivery
	acknowledged   int
	proposals      map[string]struct{}
	inFlight       bool
	sealed         bool
	failed         bool
	acknowledging  bool
	initialized    int
	listed         int
	lastProtocol   string
	methods        []string
	closed         chan struct{}
	closeOnce      sync.Once
}

func (service *harnessContextMCP) metadata() string {
	if service == nil {
		return "unavailable"
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return fmt.Sprintf(
		"initialized=%d listed=%d protocol=%s calls=%d prepared=%d acknowledged=%d sealed=%t failed=%t methods=%s",
		service.initialized, service.listed, service.lastProtocol, service.calls,
		len(service.prepared), service.acknowledged, service.sealed, service.failed,
		strings.Join(service.methods, ","),
	)
}

type harnessContextMCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func newHarnessContextMCP(retriever contextcapsule.Retriever) (*harnessContextMCP, error) {
	return newHarnessContextMCPWithContext(context.Background(), retriever)
}

func newHarnessContextMCPWithContext(
	ctx context.Context,
	retriever contextcapsule.Retriever,
) (*harnessContextMCP, error) {
	if nilHarnessInterface(retriever) {
		return nil, ErrHarnessContextMCP
	}
	return newHarnessAttemptMCPWithContext(ctx, harnessAttemptMCPConfig{Retriever: retriever})
}

func newHarnessContextDeliveryMCP(
	delivery contextcapsule.DeliveryBroker,
) (*harnessContextMCP, error) {
	return newHarnessContextDeliveryMCPWithContext(
		context.Background(), delivery,
	)
}

func newHarnessContextDeliveryMCPWithContext(
	ctx context.Context,
	delivery contextcapsule.DeliveryBroker,
) (*harnessContextMCP, error) {
	if nilHarnessInterface(delivery) {
		return nil, ErrHarnessContextMCP
	}
	return newHarnessAttemptMCPWithContext(ctx, harnessAttemptMCPConfig{Delivery: delivery})
}

func newHarnessAttemptMCPWithContext(
	ctx context.Context,
	config harnessAttemptMCPConfig,
) (*harnessContextMCP, error) {
	retrieverPresent := config.Retriever != nil && !nilHarnessInterface(config.Retriever)
	deliveryPresent := config.Delivery != nil && !nilHarnessInterface(config.Delivery)
	gatewayPresent := config.ToolGateway != nil && !nilHarnessInterface(config.ToolGateway)
	contextPresent := retrieverPresent || deliveryPresent
	if ctx == nil || ctx.Err() != nil || retrieverPresent && deliveryPresent ||
		config.Retriever != nil && !retrieverPresent || config.Delivery != nil && !deliveryPresent ||
		config.ToolGateway != nil && !gatewayPresent || !contextPresent && !gatewayPresent {
		return nil, ErrHarnessContextMCP
	}
	tools := make(map[string]permissions.ToolKind)
	if gatewayPresent {
		_, readerOK := config.ToolGateway.(loomruntime.ToolCallResultContentReader)
		_, acknowledgerOK := config.ToolGateway.(loomruntime.ToolCallResultProofAcknowledger)
		if !readerOK || !acknowledgerOK || !validHarnessToolBinding(config.ToolBinding) {
			return nil, ErrHarnessContextMCP
		}
		allowedTools := config.AllowedTools
		if len(allowedTools) == 0 {
			provider, providerOK := config.ToolGateway.(loomruntime.ToolCallCapabilityProvider)
			if !providerOK {
				return nil, ErrHarnessContextMCP
			}
			allowedTools = provider.AllowedToolCalls()
		}
		for _, tool := range allowedTools {
			switch tool {
			case permissions.ToolEdit:
				tools["loom_edit_file"] = tool
			case permissions.ToolBash:
				tools["loom_run_command"] = tool
			case permissions.ToolRead:
				tools["loom_read_file"] = tool
			case permissions.ToolGrep:
				tools["loom_grep_files"] = tool
			case permissions.ToolWebSearch:
				tools["loom_web_search"] = tool
			case permissions.ToolWebFetch:
				tools["loom_web_fetch"] = tool
			case permissions.ToolMCPTool:
				tools["loom_mcp_call"] = tool
			}
		}
		if len(tools) == 0 {
			return nil, ErrHarnessContextMCP
		}
	}
	var tokenBytes [32]byte
	if _, err := io.ReadFull(rand.Reader, tokenBytes[:]); err != nil {
		return nil, ErrHarnessContextMCP
	}
	token := hex.EncodeToString(tokenBytes[:])
	for index := range tokenBytes {
		tokenBytes[index] = 0
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, ErrHarnessContextMCP
	}
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil || host != "127.0.0.1" || port == "" {
		_ = listener.Close()
		return nil, ErrHarnessContextMCP
	}
	maxCalls := harnessContextMCPMaxCalls
	if tools["loom_edit_file"] == permissions.ToolEdit ||
		tools["loom_run_command"] == permissions.ToolBash {
		maxCalls = harnessWorkspaceMCPMaxCalls
	}
	service := &harnessContextMCP{
		lease: HarnessContextMCPLease{
			URL: "http://127.0.0.1:" + port + "/mcp", Token: token,
			ContextEnabled:   contextPresent,
			ReadEnabled:      tools["loom_read_file"] == permissions.ToolRead,
			GrepEnabled:      tools["loom_grep_files"] == permissions.ToolGrep,
			EditEnabled:      tools["loom_edit_file"] == permissions.ToolEdit,
			BashEnabled:      tools["loom_run_command"] == permissions.ToolBash,
			WebSearchEnabled: tools["loom_web_search"] == permissions.ToolWebSearch,
			WebFetchEnabled:  tools["loom_web_fetch"] == permissions.ToolWebFetch,
			MCPToolEnabled:   tools["loom_mcp_call"] == permissions.ToolMCPTool,
		},
		attemptContext: ctx, retriever: config.Retriever, delivery: config.Delivery,
		toolGateway: config.ToolGateway, toolBinding: config.ToolBinding,
		workspacePath: config.WorkspacePath, tools: tools,
		listener: listener, proposals: make(map[string]struct{}), closed: make(chan struct{}),
		maxCalls: maxCalls,
	}
	service.server = &http.Server{
		Handler:           service,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       2 * time.Second,
		MaxHeaderBytes:    4096,
	}
	go func() { _ = service.server.Serve(listener) }()
	go func() {
		select {
		case <-ctx.Done():
			service.close(false)
		case <-service.closed:
		}
	}()
	return service, nil
}

func (service *harnessContextMCP) Lease() HarnessContextMCPLease {
	if service == nil {
		return HarnessContextMCPLease{}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.lease
}

func (service *harnessContextMCP) Close() {
	service.close(true)
}

func (service *harnessContextMCP) close(graceful bool) {
	if service == nil {
		return
	}
	service.closeOnce.Do(func() {
		if graceful {
			ctx, cancel := context.WithTimeout(
				context.Background(), harnessContextMCPShutdownBudget,
			)
			_ = service.server.Shutdown(ctx)
			cancel()
		} else {
			_ = service.server.Close()
		}
		_ = service.listener.Close()
		service.mu.Lock()
		service.lease.Token = ""
		service.mu.Unlock()
		close(service.closed)
	})
}

func (service *harnessContextMCP) Acknowledge(
	ctx context.Context,
	proof attemptpayload.DeliveryProof,
) error {
	if proof != attemptpayload.ProofHarnessFinalOutput {
		return ErrHarnessContextMCP
	}
	return service.acknowledgePrepared(ctx, proof, true)
}

func (service *harnessContextMCP) acknowledgeToolResponses(ctx context.Context) error {
	return service.acknowledgePrepared(
		ctx, attemptpayload.ProofHarnessToolResponse, false,
	)
}

func (service *harnessContextMCP) acknowledgePrepared(
	ctx context.Context,
	proof attemptpayload.DeliveryProof,
	seal bool,
) error {
	if service == nil || ctx == nil || ctx.Err() != nil ||
		(service.delivery == nil && service.toolGateway == nil) ||
		(seal && proof != attemptpayload.ProofHarnessFinalOutput) ||
		(!seal && proof != attemptpayload.ProofHarnessToolResponse) {
		return ErrHarnessContextMCP
	}
	service.mu.Lock()
	if service.failed || service.inFlight || service.acknowledging {
		service.mu.Unlock()
		return ErrHarnessContextMCP
	}
	if seal {
		service.sealed = true
	}
	service.acknowledging = true
	service.mu.Unlock()
	defer func() {
		service.mu.Lock()
		service.acknowledging = false
		service.mu.Unlock()
	}()
	for {
		service.mu.Lock()
		if service.acknowledged >= len(service.prepared) {
			service.mu.Unlock()
			return nil
		}
		index := service.acknowledged
		prepared := service.prepared[index]
		service.mu.Unlock()
		var err error
		if prepared.context {
			if service.delivery == nil {
				return ErrHarnessContextMCP
			}
			err = service.delivery.Acknowledge(ctx, prepared.binding, proof)
		} else {
			acknowledger, ok := service.toolGateway.(loomruntime.ToolCallResultProofAcknowledger)
			if !ok {
				return ErrHarnessContextMCP
			}
			err = acknowledger.AcknowledgeToolCallResultWithProof(
				ctx, prepared.toolBinding, prepared.toolResult, proof,
			)
		}
		if err != nil {
			return errors.Join(ErrHarnessContextMCP, err)
		}
		service.mu.Lock()
		if index >= len(service.prepared) ||
			service.prepared[index].binding != prepared.binding ||
			service.prepared[index].context != prepared.context || service.acknowledged != index {
			service.mu.Unlock()
			return ErrHarnessContextMCP
		}
		service.acknowledged++
		service.mu.Unlock()
	}
}

func (service *harnessContextMCP) hasPendingAcknowledgements() bool {
	if service == nil {
		return false
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.acknowledged < len(service.prepared)
}

func (service *harnessContextMCP) failDelivery() {
	if service == nil {
		return
	}
	service.mu.Lock()
	service.failed = true
	service.mu.Unlock()
}

func (service *harnessContextMCP) toolNames() []string {
	if service == nil {
		return nil
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	result := make([]string, 0, len(service.tools)+1)
	if service.retriever != nil || service.delivery != nil {
		result = append(result, "loom_read_context")
	}
	for name := range service.tools {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func (service *harnessContextMCP) mcpTools() []any {
	names := service.toolNames()
	result := make([]any, 0, len(names))
	for _, name := range names {
		var description string
		var properties map[string]any
		var required []string
		annotations := map[string]any{
			"readOnlyHint": true, "destructiveHint": false,
			"idempotentHint": false, "openWorldHint": false,
		}
		switch name {
		case "loom_read_context":
			description = "Read one exact Loom-authorized omitted Context item per call."
			properties = map[string]any{
				"item_id":        map[string]any{"type": "string"},
				"content_digest": map[string]any{"type": "string"},
				"artifact_ref":   map[string]any{"type": "string"},
			}
			required = []string{"item_id", "content_digest"}
		case "loom_read_file":
			description = "Read one file through the Loom-governed workspace gateway."
			properties = map[string]any{"path": map[string]any{"type": "string"}}
			required = []string{"path"}
		case "loom_grep_files":
			description = "Search workspace files with an RE2 regular expression through the Loom-governed workspace gateway. Use path . for the workspace root."
			properties = map[string]any{
				"path":    map[string]any{"type": "string"},
				"pattern": map[string]any{"type": "string"},
			}
			required = []string{"path", "pattern"}
		case "loom_edit_file":
			description = "Create or replace one workspace-relative text file through the Loom-governed workspace gateway."
			annotations = map[string]any{
				"readOnlyHint": false, "destructiveHint": true,
				"idempotentHint": true, "openWorldHint": false,
			}
			properties = map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			}
			required = []string{"path", "content"}
		case "loom_run_command":
			description = "Run one bounded command inside the Loom-governed workspace. Use it for local tests and inspection."
			annotations = map[string]any{
				"readOnlyHint": false, "destructiveHint": true,
				"idempotentHint": false, "openWorldHint": false,
			}
			properties = map[string]any{
				"command": map[string]any{"type": "string"},
			}
			required = []string{"command"}
		case "loom_mcp_call":
			description = "Call one tool through the Loom-governed MCP gateway."
			// Enrollment authorization is exact, but a generic MCP tool's
			// side-effect semantics are not known to the Harness adapter.
			annotations = map[string]any{
				"readOnlyHint": false, "destructiveHint": true,
				"idempotentHint": false, "openWorldHint": true,
			}
			properties = map[string]any{
				"server":    map[string]any{"type": "string"},
				"tool":      map[string]any{"type": "string"},
				"arguments": map[string]any{"type": "object"},
			}
			required = []string{"server", "tool", "arguments"}
		default:
			continue
		}
		result = append(result, map[string]any{
			"name": name, "description": description, "annotations": annotations,
			"inputSchema": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": properties, "required": required,
			},
		})
	}
	return result
}

func acknowledgeHarnessContextMCP(
	ctx context.Context,
	service *harnessContextMCP,
) error {
	if service == nil {
		return nil
	}
	return service.Acknowledge(ctx, attemptpayload.ProofHarnessFinalOutput)
}

func harnessContextMCPHasToolActivity(service *harnessContextMCP) bool {
	if service == nil {
		return false
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	for _, prepared := range service.prepared {
		if !prepared.context {
			return true
		}
	}
	return false
}

func (service *harnessContextMCP) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "application/json")
	if service == nil || request == nil || request.Method != http.MethodPost ||
		request.URL.Path != "/mcp" || request.URL.RawQuery != "" ||
		!service.authorized(request.Header.Get("Authorization")) ||
		!strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, harnessContextMCPMaxRequest+1))
	if err != nil || len(body) == 0 || len(body) > harnessContextMCPMaxRequest ||
		!utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
		zeroHarnessBytes(body)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	response, status := service.handle(request.Context(), body)
	zeroHarnessBytes(body)
	if status == http.StatusOK && service.hasPendingAcknowledgements() {
		if err := service.acknowledgeToolResponses(service.attemptContext); err != nil {
			zeroHarnessBytes(response)
			service.failDelivery()
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	if len(response) > harnessContextMCPMaxResponse {
		zeroHarnessBytes(response)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(status)
	if len(response) != 0 {
		responseLength := len(response)
		written, writeErr := writer.Write(response)
		zeroHarnessBytes(response)
		if writeErr != nil || written != responseLength {
			service.failDelivery()
			return
		}
	}
}

func (service *harnessContextMCP) authorized(header string) bool {
	if service == nil || !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	service.mu.Lock()
	defer service.mu.Unlock()
	return len(token) == len(service.lease.Token) &&
		subtle.ConstantTimeCompare([]byte(token), []byte(service.lease.Token)) == 1
}

func (service *harnessContextMCP) handle(ctx context.Context, body []byte) ([]byte, int) {
	if rejectHarnessDuplicateJSONKeys(body) {
		return nil, http.StatusBadRequest
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var request harnessContextMCPRequest
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		request.JSONRPC != "2.0" || request.Method == "" || !validHarnessMCPID(request.ID) {
		return nil, http.StatusBadRequest
	}
	service.recordMethod(request.Method)
	switch request.Method {
	case "initialize":
		protocolVersion, ok := harnessMCPInitializeProtocol(request.Params)
		if len(request.ID) == 0 || !ok {
			return nil, http.StatusBadRequest
		}
		service.mu.Lock()
		service.initialized++
		service.lastProtocol = protocolVersion
		service.mu.Unlock()
		return harnessMCPResult(request.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "loom-context", "version": "1"},
		}), http.StatusOK
	case "notifications/initialized":
		if len(request.ID) != 0 || !emptyHarnessMCPParams(request.Params) {
			return nil, http.StatusBadRequest
		}
		return nil, http.StatusAccepted
	case "tools/list":
		if len(request.ID) == 0 || !validHarnessMCPListParams(request.Params) {
			return nil, http.StatusBadRequest
		}
		service.mu.Lock()
		service.listed++
		service.mu.Unlock()
		return harnessMCPResult(request.ID, map[string]any{"tools": service.mcpTools()}),
			http.StatusOK
	case "tools/call":
		if len(request.ID) == 0 {
			return nil, http.StatusBadRequest
		}
		return service.call(ctx, request.ID, request.Params)
	default:
		if len(request.ID) == 0 {
			return nil, http.StatusAccepted
		}
		return harnessMCPError(request.ID, -32601, "method_not_found"), http.StatusOK
	}
}

func (service *harnessContextMCP) recordMethod(method string) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if len(service.methods) < 16 {
		service.methods = append(service.methods, method)
	}
}

func (service *harnessContextMCP) call(
	ctx context.Context,
	id json.RawMessage,
	params json.RawMessage,
) ([]byte, int) {
	decoder := json.NewDecoder(bytes.NewReader(params))
	decoder.DisallowUnknownFields()
	var call struct {
		Name      string                     `json:"name"`
		Meta      map[string]json.RawMessage `json:"_meta,omitempty"`
		Arguments json.RawMessage            `json:"arguments"`
	}
	if rejectHarnessDuplicateJSONKeys(params) || decoder.Decode(&call) != nil ||
		decoder.Decode(&struct{}{}) != io.EOF || call.Name == "" ||
		!validHarnessMCPCallMeta(call.Meta) || len(call.Arguments) == 0 {
		return harnessMCPError(id, -32602, "invalid_request"), http.StatusOK
	}
	if call.Name == "loom_read_context" {
		return service.callContext(ctx, id, call.Arguments)
	}
	if _, ok := service.tools[call.Name]; ok {
		return service.callTool(ctx, id, call.Name, call.Arguments)
	}
	return harnessMCPError(id, -32602, "invalid_request"), http.StatusOK
}

func (service *harnessContextMCP) callContext(
	ctx context.Context,
	id json.RawMessage,
	arguments json.RawMessage,
) ([]byte, int) {
	if service.retriever == nil && service.delivery == nil {
		return harnessMCPError(id, -32602, "invalid_request"), http.StatusOK
	}
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.DisallowUnknownFields()
	var value struct {
		ItemID        string `json:"item_id"`
		ContentDigest string `json:"content_digest"`
		ArtifactRef   string `json:"artifact_ref,omitempty"`
	}
	if rejectHarnessDuplicateJSONKeys(arguments) || decoder.Decode(&value) != nil ||
		decoder.Decode(&struct{}{}) != io.EOF ||
		!validHarnessContextIdentifier(value.ItemID, 256) ||
		!validHarnessDigest(value.ContentDigest) || value.ArtifactRef != "" &&
		!validHarnessContextIdentifier(value.ArtifactRef, 512) {
		return harnessMCPError(id, -32602, "invalid_request"), http.StatusOK
	}
	proposal := contextcapsule.RetrievalProposal{
		ItemID: value.ItemID, ContentDigest: value.ContentDigest,
		ArtifactRef: value.ArtifactRef,
	}
	sequence, ok := service.reserveCall(harnessContextProposalKey(proposal))
	if !ok {
		return harnessMCPError(id, -32602, "context_retrieval_denied"), http.StatusOK
	}
	text, binding, err := service.contextResult(ctx, proposal, sequence)
	if err != nil {
		service.failCall()
		return harnessMCPError(id, -32602, "context_retrieval_denied"), http.StatusOK
	}
	defer zeroHarnessBytes(text)
	if service.delivery != nil &&
		(binding == (attemptpayload.Binding{}) || binding.Sequence != sequence) {
		service.failCall()
		return harnessMCPError(id, -32603, "context_retrieval_denied"), http.StatusOK
	}
	prepared := harnessPreparedDelivery{binding: binding, context: true}
	if !service.finishCall(prepared, binding != (attemptpayload.Binding{})) {
		return harnessMCPError(id, -32603, "context_retrieval_denied"), http.StatusOK
	}
	response := harnessMCPResult(id, map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": string(text)}},
		"structuredContent": json.RawMessage(text), "isError": false,
	})
	return response, http.StatusOK
}

func (service *harnessContextMCP) callTool(
	ctx context.Context,
	id json.RawMessage,
	name string,
	arguments json.RawMessage,
) ([]byte, int) {
	tool, ok := service.tools[name]
	if !ok || service.toolGateway == nil {
		return harnessMCPError(id, -32602, "invalid_request"), http.StatusOK
	}
	call, ok := decodeHarnessToolCall(tool, arguments)
	if !ok {
		return harnessMCPError(id, -32602, "invalid_request"), http.StatusOK
	}
	call, ok = normalizeHarnessWorkspaceCall(call, service.workspacePath)
	if !ok {
		return harnessMCPError(id, -32602, "invalid_request"), http.StatusOK
	}
	sequence, reserved := service.reserveCall("tool:" + permissions.ProposedCallDigest(call))
	if !reserved {
		return harnessMCPError(id, -32602, "tool_call_denied"), http.StatusOK
	}
	authorityContext, cancel, ok := service.attemptCallContext(ctx)
	if !ok {
		service.failCall()
		return harnessMCPError(id, -32602, "tool_call_denied"), http.StatusOK
	}
	defer cancel()
	callContext, err := loomruntime.BindToolCallSequence(authorityContext, sequence)
	if err != nil {
		service.failCall()
		return harnessMCPError(id, -32602, "tool_call_denied"), http.StatusOK
	}
	result, err := service.toolGateway.ExecuteToolCall(
		callContext,
		loomruntime.ToolCallEnvelope{JobID: service.toolBinding.WorkItemID, Call: call},
		service.toolBinding,
	)
	if err != nil {
		service.failCall()
		return harnessMCPError(id, -32602, "tool_call_denied"), http.StatusOK
	}
	text, prepared, prepareErr := service.harnessToolResult(
		authorityContext, call, result, sequence,
	)
	if prepareErr != nil {
		zeroHarnessBytes(text)
		service.failCall()
		return harnessMCPError(id, -32603, "tool_result_unavailable"), http.StatusOK
	}
	defer zeroHarnessBytes(text)
	if !service.finishCall(prepared, prepared.binding != (attemptpayload.Binding{})) {
		return harnessMCPError(id, -32603, "tool_result_unavailable"), http.StatusOK
	}
	response := harnessMCPResult(id, map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": string(text)}},
		"structuredContent": json.RawMessage(text), "isError": false,
	})
	return response, http.StatusOK
}

func (service *harnessContextMCP) attemptCallContext(
	requestContext context.Context,
) (context.Context, context.CancelFunc, bool) {
	if service == nil || service.attemptContext == nil || requestContext == nil ||
		service.attemptContext.Err() != nil || requestContext.Err() != nil {
		return nil, func() {}, false
	}
	ctx, cancel := context.WithCancel(service.attemptContext)
	stop := context.AfterFunc(requestContext, cancel)
	return ctx, func() {
		stop()
		cancel()
	}, true
}

func (service *harnessContextMCP) reserveCall(key string) (int64, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if key == "" || service.sealed || service.failed || service.inFlight ||
		service.calls >= service.maxCalls {
		return 0, false
	}
	if _, duplicate := service.proposals[key]; duplicate {
		return 0, false
	}
	service.calls++
	service.proposals[key] = struct{}{}
	service.inFlight = true
	return int64(service.calls), true
}

func (service *harnessContextMCP) failCall() {
	service.mu.Lock()
	service.inFlight = false
	service.failed = true
	service.mu.Unlock()
}

func (service *harnessContextMCP) finishCall(
	prepared harnessPreparedDelivery,
	retain bool,
) bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	if !service.inFlight || service.failed ||
		retain && (prepared.binding == (attemptpayload.Binding{}) ||
			prepared.binding.Sequence != int64(service.calls)) {
		service.inFlight = false
		service.failed = true
		return false
	}
	if retain {
		for _, existing := range service.prepared {
			if existing.binding == prepared.binding {
				service.inFlight = false
				service.failed = true
				return false
			}
		}
		service.prepared = append(service.prepared, prepared)
	}
	service.inFlight = false
	return true
}

type harnessGovernedToolResult struct {
	SchemaVersion     int                  `json:"schema_version"`
	CallDigest        string               `json:"call_digest"`
	Tool              permissions.ToolKind `json:"tool"`
	Verdict           permissions.Verdict  `json:"verdict"`
	ApprovalID        string               `json:"approval_id,omitempty"`
	ApprovalDigest    string               `json:"approval_digest,omitempty"`
	ErrorCode         string               `json:"error_code,omitempty"`
	AuthorizationPath string               `json:"authorization_path,omitempty"`
	OutputDigest      string               `json:"output_digest,omitempty"`
	Content           string               `json:"content,omitempty"`
}

func (service *harnessContextMCP) harnessToolResult(
	ctx context.Context,
	call permissions.ProposedCall,
	result loomruntime.ToolCallResult,
	sequence int64,
) ([]byte, harnessPreparedDelivery, error) {
	callDigest := permissions.ProposedCallDigest(call)
	visible := harnessGovernedToolResult{
		SchemaVersion: 1, CallDigest: callDigest, Tool: call.Tool,
		Verdict: result.Verdict, ApprovalID: result.ApprovalID,
		ApprovalDigest: result.ApprovalDigest, ErrorCode: result.ErrorCode,
		AuthorizationPath: result.AuthorizationPath, OutputDigest: result.OutputDigest,
	}
	prepared := harnessPreparedDelivery{
		toolBinding: service.toolBinding, toolResult: result,
	}
	switch result.Verdict {
	case permissions.VerdictAllow:
		reader, readerOK := service.toolGateway.(loomruntime.ToolCallResultContentReader)
		if !readerOK || result.Delivery == nil || result.Delivery.Tool != call.Tool ||
			result.Delivery.CallDigest != callDigest ||
			result.Delivery.Binding.Sequence != sequence || result.ContentDigest == "" ||
			result.ContentDigest != "sha256:"+result.Delivery.Binding.ContentDigest {
			return nil, harnessPreparedDelivery{}, ErrHarnessContextMCP
		}
		content, err := reader.ReadToolCallResultContent(ctx, service.toolBinding, result)
		if err != nil || len(content) == 0 || len(content) > harnessContextMCPMaxContent ||
			!utf8.Valid(content) || harnessContextControls(content) ||
			result.ContentDigest != "sha256:"+harnessContextDigest(content) {
			zeroHarnessBytes(content)
			return nil, harnessPreparedDelivery{}, ErrHarnessContextMCP
		}
		visible.Content = string(content)
		zeroHarnessBytes(content)
		prepared.binding = result.Delivery.Binding
	case permissions.VerdictAsk:
		if result.Delivery != nil || !validHarnessContextIdentifier(result.ApprovalID, 256) ||
			!validHarnessDigest(result.ApprovalDigest) || result.OutputDigest != "" ||
			result.ContentDigest != "" {
			return nil, harnessPreparedDelivery{}, ErrHarnessContextMCP
		}
	case permissions.VerdictDeny:
		if result.Delivery != nil || result.OutputDigest != "" || result.ContentDigest != "" ||
			(result.ErrorCode != "" && !validHarnessContextIdentifier(result.ErrorCode, 128)) ||
			(result.AuthorizationPath != "" &&
				!validHarnessContextIdentifier(result.AuthorizationPath, 256)) {
			return nil, harnessPreparedDelivery{}, ErrHarnessContextMCP
		}
	default:
		return nil, harnessPreparedDelivery{}, ErrHarnessContextMCP
	}
	text, err := json.Marshal(visible)
	if err != nil || len(text) == 0 || len(text) > harnessContextMCPMaxResponse/2 {
		zeroHarnessBytes(text)
		return nil, harnessPreparedDelivery{}, ErrHarnessContextMCP
	}
	return text, prepared, nil
}

func decodeHarnessToolCall(
	tool permissions.ToolKind,
	arguments json.RawMessage,
) (permissions.ProposedCall, bool) {
	if len(arguments) == 0 || rejectHarnessDuplicateJSONKeys(arguments) {
		return permissions.ProposedCall{}, false
	}
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.DisallowUnknownFields()
	switch tool {
	case permissions.ToolRead:
		var value struct {
			Path string `json:"path"`
		}
		if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			!validHarnessContextIdentifier(value.Path, 2048) {
			return permissions.ProposedCall{}, false
		}
		return permissions.ProposedCall{Tool: tool, Path: value.Path}, true
	case permissions.ToolEdit:
		var value struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			!validHarnessContextIdentifier(value.Path, 2048) ||
			len(value.Content) > harnessMCPToolMaxArguments || !utf8.ValidString(value.Content) ||
			harnessContextControls([]byte(value.Content)) {
			return permissions.ProposedCall{}, false
		}
		return permissions.ProposedCall{
			Tool: tool, Path: value.Path, Command: value.Content,
		}, true
	case permissions.ToolBash:
		var value struct {
			Command string `json:"command"`
		}
		if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			!validHarnessToolArgument(value.Command, 8192) {
			return permissions.ProposedCall{}, false
		}
		return permissions.ProposedCall{Tool: tool, Command: value.Command}, true
	case permissions.ToolGrep:
		var value struct {
			Path    string `json:"path"`
			Pattern string `json:"pattern"`
		}
		if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			!validHarnessContextIdentifier(value.Path, 2048) ||
			!validHarnessToolArgument(value.Pattern, 2048) {
			return permissions.ProposedCall{}, false
		}
		return permissions.ProposedCall{Tool: tool, Path: value.Path, Pattern: value.Pattern}, true
	case permissions.ToolWebSearch:
		var value struct {
			Query string `json:"query"`
		}
		if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			!validHarnessToolArgument(value.Query, 512) {
			return permissions.ProposedCall{}, false
		}
		return permissions.ProposedCall{Tool: tool, Path: value.Query}, true
	case permissions.ToolWebFetch:
		var value struct {
			URL string `json:"url"`
		}
		if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			!validHarnessToolArgument(value.URL, 2048) {
			return permissions.ProposedCall{}, false
		}
		return permissions.ProposedCall{Tool: tool, Path: value.URL}, true
	case permissions.ToolMCPTool:
		var value struct {
			Server    string          `json:"server"`
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			!validHarnessMCPToolIdentifier(value.Server) ||
			!validHarnessMCPToolIdentifier(value.Tool) || len(value.Arguments) == 0 ||
			len(value.Arguments) > harnessMCPToolMaxArguments ||
			rejectHarnessDuplicateJSONKeys(value.Arguments) {
			return permissions.ProposedCall{}, false
		}
		argumentsDecoder := json.NewDecoder(bytes.NewReader(value.Arguments))
		argumentsDecoder.UseNumber()
		var arguments map[string]any
		if argumentsDecoder.Decode(&arguments) != nil || arguments == nil ||
			argumentsDecoder.Decode(&struct{}{}) != io.EOF ||
			!validHarnessMCPArguments(arguments, 0) {
			return permissions.ProposedCall{}, false
		}
		canonical, err := json.Marshal(arguments)
		if err != nil || len(canonical) == 0 || len(canonical) > harnessMCPToolMaxArguments {
			zeroHarnessBytes(canonical)
			return permissions.ProposedCall{}, false
		}
		call := permissions.ProposedCall{
			Tool: tool, Path: value.Server + "/" + value.Tool, Command: string(canonical),
		}
		zeroHarnessBytes(canonical)
		return call, true
	default:
		return permissions.ProposedCall{}, false
	}
}

func normalizeHarnessWorkspaceCall(
	call permissions.ProposedCall,
	workspacePath string,
) (permissions.ProposedCall, bool) {
	if call.Tool != permissions.ToolRead && call.Tool != permissions.ToolEdit &&
		call.Tool != permissions.ToolGrep {
		return call, true
	}
	path := call.Path
	if filepath.IsAbs(path) {
		if workspacePath == "" || !filepath.IsAbs(workspacePath) ||
			filepath.Clean(workspacePath) != workspacePath {
			return permissions.ProposedCall{}, false
		}
		relative, err := filepath.Rel(workspacePath, filepath.Clean(path))
		if err != nil || relative == ".." ||
			strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return permissions.ProposedCall{}, false
		}
		path = relative
	}
	path = filepath.Clean(path)
	if path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(path) || path == "." && call.Tool != permissions.ToolGrep {
		return permissions.ProposedCall{}, false
	}
	call.Path = path
	return call, true
}

func validHarnessMCPToolIdentifier(value string) bool {
	if value == "" || len(value) > 128 || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if !(unicode.IsLetter(character) || unicode.IsDigit(character) ||
			character == '-' || character == '_' || character == '.') {
			return false
		}
	}
	return true
}

func validHarnessMCPArguments(value any, depth int) bool {
	if depth > 64 {
		return false
	}
	switch typed := value.(type) {
	case nil, bool, json.Number:
		return true
	case string:
		return validHarnessMCPArgumentText(typed)
	case []any:
		for _, item := range typed {
			if !validHarnessMCPArguments(item, depth+1) {
				return false
			}
		}
		return true
	case map[string]any:
		for key, item := range typed {
			if !validHarnessMCPArgumentText(key) ||
				!validHarnessMCPArguments(item, depth+1) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func validHarnessMCPArgumentText(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func harnessContextProposalKey(proposal contextcapsule.RetrievalProposal) string {
	digest := sha256.Sum256([]byte(
		"loom/harness-context-proposal/v1\x00" + proposal.ItemID + "\x00" +
			proposal.ContentDigest + "\x00" + proposal.ArtifactRef,
	))
	return "context:" + hex.EncodeToString(digest[:])
}

func validHarnessToolBinding(binding loomruntime.ToolCallBinding) bool {
	return validHarnessContextIdentifier(binding.ConversationID, 256) &&
		validHarnessContextIdentifier(binding.WorkItemID, 256) &&
		validHarnessContextIdentifier(binding.RunID, 256) && binding.ClaimGeneration > 0 &&
		validHarnessContextIdentifier(binding.RuntimeInstanceID, 256) &&
		validHarnessContextIdentifier(binding.AgentInstanceID, 256) &&
		validHarnessDigest(binding.ExecutionBindingDigest) &&
		validHarnessDigest(binding.CapsuleDigest) &&
		validHarnessContextIdentifier(binding.ClaimID, 256) &&
		validHarnessContextIdentifier(binding.IncidentID, 256) &&
		binding.JourneyID == binding.IncidentID
}

func validHarnessToolArgument(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

type harnessContextToolResult struct {
	SchemaVersion int                       `json:"schema_version"`
	ItemID        string                    `json:"item_id"`
	Kind          contextcapsule.ItemKind   `json:"kind"`
	ContentDigest string                    `json:"content_digest"`
	Trust         contextcapsule.TrustClass `json:"trust"`
	Scope         contextcapsule.Scope      `json:"scope"`
	SourceType    contextcapsule.SourceType `json:"source_type"`
	SourceRef     string                    `json:"source_ref"`
	ArtifactRef   string                    `json:"artifact_ref,omitempty"`
	Content       string                    `json:"content"`
}

func (service *harnessContextMCP) contextResult(
	ctx context.Context,
	proposal contextcapsule.RetrievalProposal,
	sequence int64,
) ([]byte, attemptpayload.Binding, error) {
	if service.delivery != nil {
		payload, err := service.delivery.Prepare(
			ctx, proposal,
			contextcapsule.DeliveryRequest{Sequence: sequence, ContentType: "application/json"},
			marshalHarnessContextToolResult,
		)
		if err != nil {
			payload.Close()
			return nil, attemptpayload.Binding{}, err
		}
		defer payload.Close()
		if _, err := decodeHarnessContextToolResult(proposal, payload.Content); err != nil {
			return nil, attemptpayload.Binding{}, err
		}
		return append([]byte(nil), payload.Content...), payload.Binding, nil
	}
	item, err := service.retriever.Retrieve(ctx, proposal)
	if err != nil {
		item.Close()
		return nil, attemptpayload.Binding{}, err
	}
	defer item.Close()
	text, err := marshalHarnessContextToolResult(proposal, item)
	return text, attemptpayload.Binding{}, err
}

func marshalHarnessContextToolResult(
	proposal contextcapsule.RetrievalProposal,
	item contextcapsule.RetrievedItem,
) ([]byte, error) {
	if !validHarnessContextItem(
		proposal.ItemID, proposal.ContentDigest, proposal.ArtifactRef, item,
	) {
		return nil, ErrHarnessContextMCP
	}
	text, err := json.Marshal(harnessContextToolResult{
		SchemaVersion: 1, ItemID: item.ItemID, Kind: item.Kind,
		ContentDigest: item.ContentDigest,
		Trust:         item.Trust, Scope: item.Scope, SourceType: item.SourceType,
		SourceRef: item.SourceRef, ArtifactRef: item.ArtifactRef,
		Content: string(item.Content),
	})
	if err != nil || len(text) == 0 || len(text) > harnessContextMCPMaxResponse/2 {
		zeroHarnessBytes(text)
		return nil, ErrHarnessContextMCP
	}
	return text, nil
}

func decodeHarnessContextToolResult(
	proposal contextcapsule.RetrievalProposal,
	body []byte,
) (harnessContextToolResult, error) {
	if len(body) == 0 || len(body) > harnessContextMCPMaxResponse/2 ||
		rejectHarnessDuplicateJSONKeys(body) {
		return harnessContextToolResult{}, ErrHarnessContextMCP
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var result harnessContextToolResult
	if decoder.Decode(&result) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		result.SchemaVersion != 1 || !validHarnessContextItem(
		proposal.ItemID, proposal.ContentDigest, proposal.ArtifactRef,
		contextcapsule.RetrievedItem{
			ItemID: result.ItemID, Kind: result.Kind,
			Trust: result.Trust, Scope: result.Scope,
			ContentDigest: result.ContentDigest, SourceType: result.SourceType,
			SourceRef: result.SourceRef, ArtifactRef: result.ArtifactRef,
			Content: []byte(result.Content),
		},
	) {
		return harnessContextToolResult{}, ErrHarnessContextMCP
	}
	return result, nil
}

func harnessMCPResult(id json.RawMessage, result any) []byte {
	body, _ := json.Marshal(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result"`
	}{"2.0", id, result})
	return body
}

func harnessMCPError(id json.RawMessage, code int, message string) []byte {
	body, _ := json.Marshal(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{JSONRPC: "2.0", ID: id, Error: struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}{code, message}})
	return body
}

func validHarnessMCPID(id json.RawMessage) bool {
	if len(id) == 0 {
		return true
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(id))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return false
	}
	switch value.(type) {
	case string, json.Number:
		return true
	default:
		return false
	}
}

func harnessMCPInitializeProtocol(params json.RawMessage) (string, bool) {
	if len(params) == 0 || rejectHarnessDuplicateJSONKeys(params) {
		return "", false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(params, &fields) != nil {
		return "", false
	}
	for _, key := range []string{"protocolVersion", "capabilities", "clientInfo"} {
		if len(fields[key]) == 0 {
			return "", false
		}
	}
	if len(fields) != 3 || !validHarnessMCPObject(fields["capabilities"], false) ||
		!validHarnessMCPClientInfo(fields["clientInfo"]) {
		return "", false
	}
	var protocolVersion string
	if json.Unmarshal(fields["protocolVersion"], &protocolVersion) != nil {
		return "", false
	}
	switch protocolVersion {
	case "2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25":
		return protocolVersion, true
	default:
		return "", false
	}
}

func validHarnessMCPClientInfo(payload json.RawMessage) bool {
	if !validHarnessMCPObject(payload, true) {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(payload, &fields) != nil || len(fields) < 2 || len(fields) > 5 {
		return false
	}
	for key := range fields {
		if key != "name" && key != "version" && key != "title" &&
			key != "description" && key != "websiteUrl" {
			return false
		}
	}
	for _, key := range []string{"name", "version"} {
		var value string
		if json.Unmarshal(fields[key], &value) != nil ||
			!validHarnessContextIdentifier(value, 128) {
			return false
		}
	}
	if raw := fields["title"]; len(raw) != 0 {
		var title string
		if json.Unmarshal(raw, &title) != nil ||
			!validHarnessContextIdentifier(title, 256) {
			return false
		}
	}
	if raw := fields["description"]; len(raw) != 0 {
		var description string
		if json.Unmarshal(raw, &description) != nil ||
			!validHarnessContextIdentifier(description, 512) {
			return false
		}
	}
	if raw := fields["websiteUrl"]; len(raw) != 0 {
		var website string
		if json.Unmarshal(raw, &website) != nil {
			return false
		}
		parsed, err := url.Parse(website)
		if err != nil || parsed == nil ||
			parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
			parsed.RawQuery != "" || parsed.Fragment != "" ||
			!validHarnessContextIdentifier(website, 512) {
			return false
		}
	}
	return true
}

func validHarnessMCPObject(payload json.RawMessage, requireNonEmpty bool) bool {
	if len(payload) == 0 || rejectHarnessDuplicateJSONKeys(payload) {
		return false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(payload, &object) != nil || object == nil {
		return false
	}
	return !requireNonEmpty || len(object) > 0
}

func emptyHarnessMCPParams(params json.RawMessage) bool {
	return len(params) == 0 || bytes.Equal(bytes.TrimSpace(params), []byte("{}"))
}

func validHarnessMCPListParams(params json.RawMessage) bool {
	if emptyHarnessMCPParams(params) {
		return true
	}
	if rejectHarnessDuplicateJSONKeys(params) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(params))
	decoder.DisallowUnknownFields()
	var value struct {
		Cursor *string                    `json:"cursor,omitempty"`
		Meta   map[string]json.RawMessage `json:"_meta,omitempty"`
	}
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		value.Cursor != nil || value.Meta == nil {
		return false
	}
	for key, raw := range value.Meta {
		if key != "progressToken" || !validHarnessMCPID(raw) {
			return false
		}
	}
	return true
}

func validHarnessMCPCallMeta(meta map[string]json.RawMessage) bool {
	if meta == nil {
		return true
	}
	if len(meta) > 4 {
		return false
	}
	for key, raw := range meta {
		switch key {
		case "threadId", "codex_bridge_mcp_call_id", "claudecode/toolUseId":
			var value string
			if json.Unmarshal(raw, &value) != nil ||
				!validHarnessContextIdentifier(value, 128) {
				return false
			}
		case "progressToken":
			if !validHarnessMCPID(raw) {
				return false
			}
		case "x-codex-turn-metadata":
			if !validHarnessMCPMetadataValue(raw, 0) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func validHarnessMCPMetadataValue(raw json.RawMessage, depth int) bool {
	if len(raw) == 0 || len(raw) > 4096 || depth > 4 ||
		rejectHarnessDuplicateJSONKeys(raw) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return false
	}
	object, ok := value.(map[string]any)
	return ok && validHarnessMCPMetadataNode(object, depth)
}

func validHarnessMCPMetadataNode(value any, depth int) bool {
	if depth > 4 {
		return false
	}
	switch typed := value.(type) {
	case nil, bool, json.Number:
		return true
	case string:
		return len(typed) <= 1024 && utf8.ValidString(typed) &&
			!harnessContextControls([]byte(typed))
	case []any:
		if len(typed) > 32 {
			return false
		}
		for _, item := range typed {
			if !validHarnessMCPMetadataNode(item, depth+1) {
				return false
			}
		}
		return true
	case map[string]any:
		if len(typed) > 64 {
			return false
		}
		for key, item := range typed {
			if !validHarnessContextIdentifier(key, 128) || harnessMCPSecretMetadataKey(key) ||
				!validHarnessMCPMetadataNode(item, depth+1) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func harnessMCPSecretMetadataKey(key string) bool {
	canonical := strings.NewReplacer("-", "", "_", "", ".", "", " ", "").
		Replace(strings.ToLower(key))
	switch canonical {
	case "apikey", "authorization", "credential", "credentials", "password",
		"secret", "secrets", "accesstoken", "refreshtoken":
		return true
	default:
		return false
	}
}

func validHarnessContextItem(
	itemID, digest, artifact string,
	item contextcapsule.RetrievedItem,
) bool {
	if item.ItemID != itemID || item.ContentDigest != digest || item.ArtifactRef != artifact ||
		item.Kind == contextcapsule.KindCredentialReference ||
		item.Scope == contextcapsule.ScopeSecretReferenceOnly ||
		item.SourceType == contextcapsule.SourceCredentialReference ||
		len(item.Content) == 0 || len(item.Content) > harnessContextMCPMaxContent ||
		!utf8.Valid(item.Content) || harnessContextControls(item.Content) ||
		harnessContextDigest(item.Content) != item.ContentDigest ||
		!validHarnessContextIdentifier(item.SourceRef, 512) ||
		(item.Scope == contextcapsule.ScopeArtifactScoped) != (artifact != "") {
		return false
	}
	switch item.SourceType {
	case contextcapsule.SourceAuthority:
		return item.Trust == contextcapsule.TrustAuthoritative
	case contextcapsule.SourceObservation:
		return item.Trust == contextcapsule.TrustObserved
	case contextcapsule.SourceModelOutput:
		return item.Trust == contextcapsule.TrustUntrusted &&
			item.Kind == contextcapsule.KindPriorModelOutput
	default:
		return false
	}
}

func validHarnessContextIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value ||
		!utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validHarnessDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func harnessContextDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func harnessContextControls(content []byte) bool {
	for _, character := range string(content) {
		if unicode.IsControl(character) && character != '\n' && character != '\t' {
			return true
		}
	}
	return false
}

func zeroHarnessBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func rejectHarnessDuplicateJSONKeys(payload []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	var walk func() bool
	walk = func() bool {
		token, err := decoder.Token()
		if err != nil {
			return true
		}
		delimiter, composite := token.(json.Delim)
		if !composite {
			return false
		}
		switch delimiter {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				key, ok := keyToken.(string)
				if err != nil || !ok {
					return true
				}
				if _, duplicate := seen[key]; duplicate {
					return true
				}
				seen[key] = struct{}{}
				if walk() {
					return true
				}
			}
		case '[':
			for decoder.More() {
				if walk() {
					return true
				}
			}
		default:
			return true
		}
		_, err = decoder.Token()
		return err != nil
	}
	if walk() {
		return true
	}
	return decoder.Decode(&struct{}{}) != io.EOF
}
