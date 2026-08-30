package harnessadapter

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/controltool"
)

var ErrHarnessControlMCP = errors.New("Harness conversation control MCP unavailable")

const (
	harnessControlMCPTokenEnv        = "LOOM_CONTROL_TURN_TOKEN"
	harnessControlMCPMaxRequest      = 48 << 10
	harnessControlMCPMaxResponse     = 64 << 10
	harnessControlMCPMaxCalls        = 8
	harnessControlMCPShutdownBudget  = 2 * time.Second
	harnessControlMCPReadRetryDelay  = 100 * time.Millisecond
	harnessControlMCPCompletedMethod = "loom/control/completed"
	harnessControlMCPAckMethod       = "loom/control/acknowledged"
	harnessControlMCPURLEnv          = "LOOM_CONTROL_MCP_URL"
)

type HarnessControlMCPLease struct {
	URL       string
	Token     string
	ToolNames []string
}

// HarnessControlServer owns one private, turn-scoped Loom control MCP endpoint.
// Runtime adapters receive only its lease and cannot confirm or execute proposals.
type HarnessControlServer struct {
	service *harnessControlMCP
}

func OpenHarnessControlServer(
	ctx context.Context,
	registry *controltool.Registry,
	gateway controltool.Gateway,
) (*HarnessControlServer, error) {
	service, err := newHarnessControlMCP(ctx, registry, gateway)
	if err != nil {
		return nil, err
	}
	return &HarnessControlServer{service: service}, nil
}

func (server *HarnessControlServer) Lease() HarnessControlMCPLease {
	if server == nil {
		return HarnessControlMCPLease{}
	}
	return server.service.Lease()
}

func (server *HarnessControlServer) BeginTurn(turn controltool.TurnContext) error {
	if server == nil {
		return ErrHarnessControlMCP
	}
	return server.service.BeginTurn(turn)
}

func (server *HarnessControlServer) EndTurn() (controltool.ProposalBatch, error) {
	if server == nil {
		return controltool.ProposalBatch{}, ErrHarnessControlMCP
	}
	return server.service.EndTurn()
}

// TerminalProposalCompleted closes only after the Harness acknowledges that it
// consumed a proposal Tool response and the acknowledgement response is
// written. The signal carries no arguments, result content, or authority;
// callers must still validate EndTurn's batch.
func (server *HarnessControlServer) TerminalProposalCompleted() (<-chan struct{}, error) {
	if server == nil {
		return nil, ErrHarnessControlMCP
	}
	return server.service.terminalProposalCompleted()
}

func (server *HarnessControlServer) Close() {
	if server != nil {
		server.service.Close()
	}
}

type harnessControlMCP struct {
	registry *controltool.Registry
	gateway  controltool.Gateway
	lease    HarnessControlMCPLease
	listener net.Listener
	server   *http.Server

	mu              sync.Mutex
	turn            *controltool.TurnContext
	calls           int
	proposals       []controltool.SessionAlignmentProposal
	actionProposals []controltool.ConversationActionProposal
	completedCalls  []controltool.CompletedCall
	callsSuspended  bool
	terminalSignal  chan struct{}
	terminalSent    bool
	closed          bool
	closeOnce       sync.Once
}

func newHarnessControlMCP(
	ctx context.Context,
	registry *controltool.Registry,
	gateway controltool.Gateway,
) (*harnessControlMCP, error) {
	if ctx == nil || ctx.Err() != nil || registry == nil || registry.Digest() == "" ||
		nilHarnessInterface(gateway) {
		return nil, ErrHarnessControlMCP
	}
	var tokenBytes [32]byte
	if _, err := io.ReadFull(rand.Reader, tokenBytes[:]); err != nil {
		return nil, ErrHarnessControlMCP
	}
	token := hex.EncodeToString(tokenBytes[:])
	zeroHarnessBytes(tokenBytes[:])
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, ErrHarnessControlMCP
	}
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil || host != "127.0.0.1" || port == "" {
		_ = listener.Close()
		return nil, ErrHarnessControlMCP
	}
	toolNames := make([]string, 0, len(registry.Definitions()))
	for _, definition := range registry.Definitions() {
		toolNames = append(toolNames, definition.MCPName)
	}
	service := &harnessControlMCP{
		registry: registry, gateway: gateway,
		lease: HarnessControlMCPLease{
			URL: "http://127.0.0.1:" + port + "/mcp", Token: token,
			ToolNames: toolNames,
		},
		listener: listener,
	}
	service.server = &http.Server{
		Handler: service, ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second,
		IdleTimeout: 2 * time.Second, MaxHeaderBytes: 4096,
	}
	go func() { _ = service.server.Serve(listener) }()
	go func() {
		<-ctx.Done()
		service.close(false)
	}()
	return service, nil
}

func (service *harnessControlMCP) Lease() HarnessControlMCPLease {
	if service == nil {
		return HarnessControlMCPLease{}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	lease := service.lease
	lease.ToolNames = append([]string(nil), service.lease.ToolNames...)
	return lease
}

func (service *harnessControlMCP) BeginTurn(turn controltool.TurnContext) error {
	if service == nil || !validHarnessProtocolID(turn.ConversationID) ||
		!validHarnessProtocolID(turn.SegmentID) || !validHarnessProtocolID(turn.AttemptID) ||
		turn.IncidentID != "" && !validHarnessProtocolID(turn.IncidentID) ||
		len(turn.CatalogDigest) != 64 || len(turn.Catalog) > 1_024 ||
		turn.RegistryDigest != "" && turn.RegistryDigest != service.registry.Digest() ||
		!turn.Route.Empty() && !turn.Route.Valid() ||
		!turn.Workspace.Empty() && !turn.Workspace.Valid() {
		return ErrHarnessControlMCP
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.turn != nil {
		return ErrHarnessControlMCP
	}
	frozen := turn
	frozen.RegistryDigest = service.registry.Digest()
	frozen.Catalog = append([]controltool.SessionReference(nil), turn.Catalog...)
	service.turn = &frozen
	service.calls = 0
	service.proposals = nil
	service.actionProposals = nil
	service.completedCalls = nil
	service.callsSuspended = false
	service.terminalSignal = make(chan struct{})
	service.terminalSent = false
	return nil
}

func (service *harnessControlMCP) terminalProposalCompleted() (<-chan struct{}, error) {
	if service == nil {
		return nil, ErrHarnessControlMCP
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.turn == nil || service.terminalSignal == nil {
		return nil, ErrHarnessControlMCP
	}
	return service.terminalSignal, nil
}

func (service *harnessControlMCP) publishTerminalProposal() {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.turn == nil || !service.callsSuspended ||
		service.terminalSignal == nil || service.terminalSent {
		return
	}
	service.terminalSent = true
	close(service.terminalSignal)
}

func (service *harnessControlMCP) suspendTurnCalls() error {
	if service == nil {
		return ErrHarnessControlMCP
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.turn == nil {
		return ErrHarnessControlMCP
	}
	if service.callsSuspended {
		return nil
	}
	service.callsSuspended = true
	return nil
}

func (service *harnessControlMCP) completedToolNames() ([]string, error) {
	if service == nil {
		return nil, ErrHarnessControlMCP
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.turn == nil {
		return nil, ErrHarnessControlMCP
	}
	return service.completedToolNamesLocked()
}

func (service *harnessControlMCP) selectedToolAlreadyCompleted(
	toolName string,
) (bool, error) {
	if service == nil || !validHarnessControlToolName(toolName) {
		return false, ErrHarnessControlMCP
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.turn == nil {
		return false, ErrHarnessControlMCP
	}
	if len(service.completedCalls) == 0 {
		return false, nil
	}
	selected, found := service.registry.DefinitionByMCPName(toolName)
	if !found {
		return false, ErrHarnessControlMCP
	}
	for index, call := range service.completedCalls {
		definition, found := service.registry.Definition(call.ToolID)
		if !found || definition.Version != call.ToolVersion ||
			definition.Effect != call.Effect {
			return false, ErrHarnessControlMCP
		}
		if index < len(service.completedCalls)-1 {
			if definition.Effect != controltool.EffectRead || definition.ID == selected.ID {
				return false, ErrHarnessControlMCP
			}
			continue
		}
		if definition.ID != selected.ID {
			return false, ErrHarnessControlMCP
		}
	}
	return true, nil
}

func (service *harnessControlMCP) EndTurn() (
	controltool.ProposalBatch,
	error,
) {
	if service == nil {
		return controltool.ProposalBatch{}, ErrHarnessControlMCP
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.turn == nil {
		return controltool.ProposalBatch{}, ErrHarnessControlMCP
	}
	batch := controltool.ProposalBatch{
		SessionAlignments: controltool.CloneSessionAlignmentProposals(service.proposals),
		ConversationActions: controltool.CloneConversationActionProposals(
			service.actionProposals,
		),
		CompletedCalls: controltool.CloneCompletedCalls(service.completedCalls),
	}
	service.turn = nil
	service.calls = 0
	service.proposals = nil
	service.actionProposals = nil
	service.completedCalls = nil
	service.callsSuspended = false
	service.terminalSignal = nil
	service.terminalSent = false
	return batch, nil
}

func (service *harnessControlMCP) ServeHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "application/json")
	if service == nil || request == nil || request.Method != http.MethodPost ||
		request.URL.Path != "/mcp" || request.URL.RawQuery != "" ||
		!service.authorized(request.Header.Get("Authorization")) ||
		!strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, harnessControlMCPMaxRequest+1))
	if err != nil || len(body) == 0 || len(body) > harnessControlMCPMaxRequest ||
		!utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
		zeroHarnessBytes(body)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	outcome := service.handleOutcome(request.Context(), body)
	zeroHarnessBytes(body)
	response, status := outcome.response, outcome.status
	if len(response) > harnessControlMCPMaxResponse {
		zeroHarnessBytes(response)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(status)
	written := len(response) == 0
	if len(response) != 0 {
		count, writeErr := writer.Write(response)
		written = writeErr == nil && count == len(response)
		zeroHarnessBytes(response)
	}
	if written && outcome.terminalProposal {
		service.publishTerminalProposal()
	}
}

func (service *harnessControlMCP) authorized(header string) bool {
	if service == nil || !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	service.mu.Lock()
	defer service.mu.Unlock()
	return !service.closed && len(token) == len(service.lease.Token) &&
		subtle.ConstantTimeCompare([]byte(token), []byte(service.lease.Token)) == 1
}

func (service *harnessControlMCP) handle(
	ctx context.Context,
	body []byte,
) ([]byte, int) {
	outcome := service.handleOutcome(ctx, body)
	return outcome.response, outcome.status
}

type harnessControlMCPOutcome struct {
	response         []byte
	status           int
	terminalProposal bool
}

func (service *harnessControlMCP) handleOutcome(
	ctx context.Context,
	body []byte,
) harnessControlMCPOutcome {
	if rejectHarnessDuplicateJSONKeys(body) {
		return harnessControlMCPOutcome{status: http.StatusBadRequest}
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var request harnessContextMCPRequest
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		request.JSONRPC != "2.0" || request.Method == "" || !validHarnessMCPID(request.ID) {
		return harnessControlMCPOutcome{status: http.StatusBadRequest}
	}
	switch request.Method {
	case "initialize":
		protocolVersion, ok := harnessMCPInitializeProtocol(request.Params)
		if len(request.ID) == 0 || !ok {
			return harnessControlMCPOutcome{status: http.StatusBadRequest}
		}
		return harnessControlMCPOutcome{response: harnessMCPResult(request.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "loom-control", "version": "1"},
		}), status: http.StatusOK}
	case "notifications/initialized":
		if len(request.ID) != 0 || !emptyHarnessMCPParams(request.Params) {
			return harnessControlMCPOutcome{status: http.StatusBadRequest}
		}
		return harnessControlMCPOutcome{status: http.StatusAccepted}
	case "tools/list":
		if len(request.ID) == 0 || !validHarnessMCPListParams(request.Params) {
			return harnessControlMCPOutcome{status: http.StatusBadRequest}
		}
		return harnessControlMCPOutcome{
			response: harnessMCPResult(
				request.ID, map[string]any{"tools": service.mcpTools()},
			),
			status: http.StatusOK,
		}
	case "tools/call":
		if len(request.ID) == 0 {
			return harnessControlMCPOutcome{status: http.StatusBadRequest}
		}
		return service.callOutcome(ctx, request.ID, request.Params)
	case harnessControlMCPCompletedMethod:
		if len(request.ID) == 0 || !emptyHarnessMCPParams(request.Params) {
			return harnessControlMCPOutcome{status: http.StatusBadRequest}
		}
		response, status := service.completed(request.ID)
		return harnessControlMCPOutcome{response: response, status: status}
	case harnessControlMCPAckMethod:
		if len(request.ID) == 0 || !emptyHarnessMCPParams(request.Params) {
			return harnessControlMCPOutcome{status: http.StatusBadRequest}
		}
		response, status, terminal := service.acknowledged(request.ID)
		return harnessControlMCPOutcome{
			response: response, status: status, terminalProposal: terminal,
		}
	default:
		if len(request.ID) == 0 {
			return harnessControlMCPOutcome{status: http.StatusAccepted}
		}
		return harnessControlMCPOutcome{
			response: harnessMCPError(request.ID, -32601, "method_not_found"),
			status:   http.StatusOK,
		}
	}
}

func (service *harnessControlMCP) mcpTools() []map[string]any {
	definitions := service.registry.Definitions()
	result := make([]map[string]any, 0, len(definitions))
	for _, definition := range definitions {
		var schema map[string]any
		if json.Unmarshal(definition.InputSchema, &schema) != nil {
			continue
		}
		readOnly := definition.Effect == controltool.EffectRead
		result = append(result, map[string]any{
			"name": definition.MCPName, "description": definition.Description,
			"inputSchema": schema,
			"annotations": map[string]any{
				"readOnlyHint": readOnly, "destructiveHint": false,
				"idempotentHint": readOnly, "openWorldHint": false,
			},
			"_meta": map[string]any{
				"loom/tool_id":      string(definition.ID),
				"loom/tool_version": definition.Version,
				"loom/effect":       string(definition.Effect),
				"loom/confirmation": string(definition.Confirmation),
			},
		})
	}
	return result
}

func (service *harnessControlMCP) call(
	ctx context.Context,
	id json.RawMessage,
	params json.RawMessage,
) ([]byte, int) {
	outcome := service.callOutcome(ctx, id, params)
	return outcome.response, outcome.status
}

func (service *harnessControlMCP) callOutcome(
	ctx context.Context,
	id json.RawMessage,
	params json.RawMessage,
) harnessControlMCPOutcome {
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
		return harnessControlMCPOutcome{
			response: harnessMCPError(id, -32602, "invalid_request"), status: http.StatusOK,
		}
	}
	definition, found := service.registry.DefinitionByMCPName(call.Name)
	if !found || rejectHarnessDuplicateJSONKeys(call.Arguments) {
		return harnessControlMCPOutcome{
			response: harnessMCPError(id, -32602, "invalid_request"), status: http.StatusOK,
		}
	}
	service.mu.Lock()
	if service.closed || service.turn == nil || service.callsSuspended {
		service.mu.Unlock()
		return harnessControlMCPOutcome{
			response: harnessMCPError(id, -32001, "turn_unavailable"), status: http.StatusOK,
		}
	}
	if service.calls >= harnessControlMCPMaxCalls {
		service.mu.Unlock()
		return harnessControlMCPOutcome{
			response: harnessMCPError(id, -32002, "call_limit"), status: http.StatusOK,
		}
	}
	turn := *service.turn
	turn.Catalog = append([]controltool.SessionReference(nil), service.turn.Catalog...)
	service.calls++
	service.mu.Unlock()

	result, err := service.callGateway(ctx, turn, controltool.Call{
		ToolID: definition.ID, Arguments: append(json.RawMessage(nil), call.Arguments...),
	}, definition.Effect)
	if err != nil {
		if errors.Is(err, controltool.ErrInvalidCall) {
			return harnessControlMCPOutcome{
				response: harnessMCPError(id, -32602, "invalid_request"), status: http.StatusOK,
			}
		}
		return harnessControlMCPOutcome{
			response: harnessMCPError(id, -32003, "tool_unavailable"), status: http.StatusOK,
		}
	}
	if len(result.Content) == 0 ||
		len(result.Content) > harnessControlMCPMaxResponse ||
		!json.Valid(result.Content) {
		return harnessControlMCPOutcome{
			response: harnessMCPError(id, -32003, "tool_unavailable"), status: http.StatusOK,
		}
	}
	proposalCount := 0
	if result.Proposal != nil {
		proposalCount++
	}
	if result.ActionProposal != nil {
		proposalCount++
	}
	if definition.Effect == controltool.EffectProposal && proposalCount != 1 ||
		definition.Effect == controltool.EffectRead && proposalCount != 0 {
		return harnessControlMCPOutcome{
			response: harnessMCPError(id, -32003, "tool_unavailable"), status: http.StatusOK,
		}
	}
	if result.Proposal != nil {
		if !result.Proposal.Valid() || result.Proposal.ToolID != definition.ID {
			return harnessControlMCPOutcome{
				response: harnessMCPError(id, -32003, "tool_unavailable"), status: http.StatusOK,
			}
		}
	}
	if result.ActionProposal != nil {
		if !result.ActionProposal.Valid() || result.ActionProposal.ToolID != definition.ID {
			return harnessControlMCPOutcome{
				response: harnessMCPError(id, -32003, "tool_unavailable"), status: http.StatusOK,
			}
		}
	}
	var structured any
	if json.Unmarshal(result.Content, &structured) != nil {
		return harnessControlMCPOutcome{
			response: harnessMCPError(id, -32003, "tool_unavailable"), status: http.StatusOK,
		}
	}
	service.mu.Lock()
	if service.turn == nil || service.turn.AttemptID != turn.AttemptID {
		service.mu.Unlock()
		return harnessControlMCPOutcome{
			response: harnessMCPError(id, -32001, "turn_unavailable"), status: http.StatusOK,
		}
	}
	if result.Proposal != nil && service.proposalIdentityExists(
		result.Proposal.ProposalID, result.Proposal.ProposalDigest,
	) || result.ActionProposal != nil && service.proposalIdentityExists(
		result.ActionProposal.ProposalID, result.ActionProposal.ProposalDigest,
	) {
		service.mu.Unlock()
		return harnessControlMCPOutcome{
			response: harnessMCPError(id, -32003, "tool_unavailable"), status: http.StatusOK,
		}
	}
	if result.Proposal != nil {
		service.proposals = append(service.proposals, *result.Proposal)
	}
	if result.ActionProposal != nil {
		service.actionProposals = append(
			service.actionProposals, result.ActionProposal.Clone(),
		)
	}
	service.completedCalls = append(service.completedCalls, controltool.CompletedCall{
		ToolID: definition.ID, ToolVersion: definition.Version, Effect: definition.Effect,
	})
	terminalProposal := definition.Effect == controltool.EffectProposal
	if terminalProposal {
		service.callsSuspended = true
	}
	service.mu.Unlock()
	return harnessControlMCPOutcome{
		response: harnessMCPResult(id, map[string]any{
			"content":           []map[string]any{{"type": "text", "text": string(result.Content)}},
			"structuredContent": structured, "isError": false,
		}),
		status: http.StatusOK,
	}
}

func (service *harnessControlMCP) callGateway(
	ctx context.Context,
	turn controltool.TurnContext,
	call controltool.Call,
	effect controltool.Effect,
) (controltool.Result, error) {
	result, err := service.gateway.Call(ctx, turn, call)
	if err == nil || effect != controltool.EffectRead ||
		!errors.Is(err, controltool.ErrGatewayUnavailable) {
		return result, err
	}
	zeroHarnessBytes(result.Content)
	timer := time.NewTimer(harnessControlMCPReadRetryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return controltool.Result{}, ctx.Err()
	case <-timer.C:
		return service.gateway.Call(ctx, turn, call)
	}
}

func (service *harnessControlMCP) completed(id json.RawMessage) ([]byte, int) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.turn == nil {
		return harnessMCPError(id, -32001, "turn_unavailable"), http.StatusOK
	}
	toolNames, err := service.completedToolNamesLocked()
	if err != nil {
		return harnessMCPError(id, -32003, "tool_unavailable"), http.StatusOK
	}
	return harnessMCPResult(id, map[string]any{
		"tool_names": toolNames,
	}), http.StatusOK
}

func (service *harnessControlMCP) acknowledged(
	id json.RawMessage,
) ([]byte, int, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.turn == nil {
		return harnessMCPError(id, -32001, "turn_unavailable"), http.StatusOK, false
	}
	terminal := service.terminalProposalReadyLocked()
	return harnessMCPResult(id, map[string]any{"terminal": terminal}),
		http.StatusOK, terminal
}

func (service *harnessControlMCP) terminalProposalReadyLocked() bool {
	if !service.callsSuspended || len(service.completedCalls) == 0 ||
		len(service.proposals)+len(service.actionProposals) != 1 ||
		service.completedCalls[len(service.completedCalls)-1].Effect != controltool.EffectProposal {
		return false
	}
	proposalCalls := 0
	for _, call := range service.completedCalls {
		if call.Effect == controltool.EffectProposal {
			proposalCalls++
		}
	}
	return proposalCalls == 1
}

func (service *harnessControlMCP) completedToolNamesLocked() ([]string, error) {
	toolNames := make([]string, 0, len(service.completedCalls))
	for _, call := range service.completedCalls {
		definition, found := service.registry.Definition(call.ToolID)
		if !found || definition.Version != call.ToolVersion ||
			definition.Effect != call.Effect {
			return nil, ErrHarnessControlMCP
		}
		toolNames = append(toolNames, definition.MCPName)
	}
	return toolNames, nil
}

func (service *harnessControlMCP) proposalIdentityExists(id string, digest string) bool {
	for _, existing := range service.proposals {
		if existing.ProposalID == id || existing.ProposalDigest == digest {
			return true
		}
	}
	for _, existing := range service.actionProposals {
		if existing.ProposalID == id || existing.ProposalDigest == digest {
			return true
		}
	}
	return false
}

func (service *harnessControlMCP) Close() { service.close(true) }

func (service *harnessControlMCP) close(graceful bool) {
	if service == nil {
		return
	}
	service.closeOnce.Do(func() {
		service.mu.Lock()
		service.closed = true
		service.turn = nil
		service.proposals = nil
		service.actionProposals = nil
		service.completedCalls = nil
		service.callsSuspended = false
		service.terminalSignal = nil
		service.terminalSent = false
		service.lease.Token = ""
		service.lease.ToolNames = nil
		service.mu.Unlock()
		if graceful {
			ctx, cancel := context.WithTimeout(
				context.Background(), harnessControlMCPShutdownBudget,
			)
			_ = service.server.Shutdown(ctx)
			cancel()
		} else {
			_ = service.server.Close()
		}
		_ = service.listener.Close()
	})
}
