package nativeadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/prompting"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/verification"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const (
	LoomNativeAgentAdapterType       = "loom-native"
	DeepSeekAgentAdapterType         = LoomNativeAgentAdapterType
	DeepSeekAgentProviderID          = "deepseek"
	DeepSeekAgentModelID             = "deepseek-chat"
	DeepSeekAgentEndpoint            = "https://api.deepseek.com/chat/completions"
	DeepSeekAgentEndpointFingerprint = "948f1ecb6b48f91adc4e110d0351cd172b16450e9936d358992e0dfad7b863f3"
	KimiAgentProviderID              = "kimi"
	KimiAgentModelID                 = "kimi-k2.6"
	KimiAgentEndpoint                = "https://api.moonshot.cn/v1/chat/completions"
	KimiAgentEndpointFingerprint     = "ff7dad4f0b167f53ac644aadc2028891f6c496db1e0ad7b6c0734c114e72bfd4"
	MiniMaxAgentProviderID           = "minimax"
	MiniMaxAgentModelID              = "MiniMax-M3"
	MiniMaxAgentEndpoint             = "https://api.minimaxi.com/v1/chat/completions"
	MiniMaxAgentEndpointFingerprint  = "e06a7ee6786ad3f758a129ef7f6c214e9a17ff88c4326bb1e0e82b93c94ca561"

	piVerifierPromptKind           = "pi_verifier_prompt"
	deepSeekAgentMaxPromptBytes    = 64 * 1024
	deepSeekAgentMaxContentBytes   = 4096
	contextToolMaxArgumentsBytes   = 2048
	contextToolMaxResultBytes      = 32 << 10
	contextToolMaxCallsPerExchange = 4
	// contextReadExhaustionDirective tells the model, once the bounded
	// context-read budget for an exchange is exhausted, to stop calling
	// tools and answer from its objective and instructions. A denied
	// Context item must never become a terminal attempt failure.
	contextReadExhaustionDirective = "The context-read limit for this attempt is exhausted and no further tools are available. Do not call any tools. Produce your best final answer now based on the objective and instructions in this conversation."
	contextReadBoundedDenial       = `{"error":"context_item_unavailable","message":"The requested context item is not available to this attempt. Answer directly using the objective and instructions already provided in this conversation."}`
)

var (
	ErrInvalidOpenAICompatibleAgentAdapter = errors.New("invalid OpenAI-compatible Agent adapter")
	ErrInvalidDeepSeekAgentAdapter         = ErrInvalidOpenAICompatibleAgentAdapter
	ErrAgentExecutionBindingChanged        = errors.New("Agent execution binding changed")
	ErrAgentCredentialUnavailable          = errors.New("Agent credential unavailable")
	ErrAgentDiagnosticsUnavailable         = errors.New("Agent diagnostics unavailable")
	ErrOpenAICompatibleAgentProtocol       = errors.New("OpenAI-compatible Agent protocol failure")
	ErrDeepSeekAgentProtocol               = ErrOpenAICompatibleAgentProtocol
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// CredentialAccess resolves only the exact frozen Provider Account and
// credential revision, bounds secret lifetime to use, and clears its copy.
type CredentialAccess interface {
	UseCredential(
		context.Context,
		loomruntime.FrozenExecutionBinding,
		func(context.Context, []byte) error,
	) error
}

type AgentAttemptDiagnostic struct {
	OccurredAt             time.Time
	IncidentID             string
	ProviderID             string
	ProviderAccountID      string
	ModelID                string
	WorkItemID             string
	RunID                  string
	ClaimGeneration        int64
	RuntimeInstanceID      string
	AgentInstanceID        string
	ExecutionBindingDigest string
	ContextCapsuleDigest   string
	Stage                  string
	Elapsed                time.Duration
	Result                 string
	ErrorCode              string
	Retryable              bool
}

type AgentAttemptDiagnosticRecorder interface {
	RecordAgentAttemptDiagnostic(context.Context, AgentAttemptDiagnostic) error
}

type OpenAICompatibleAgentAdapterConfig struct {
	RuntimeInstanceID string
	CredentialAccess  CredentialAccess
	Diagnostics       AgentAttemptDiagnosticRecorder
	Client            HTTPDoer
	Now               func() time.Time
	MaxResponseBytes  int64
	ToolGateway       loomruntime.AttemptToolGateway
}

type DeepSeekAgentAdapterConfig = OpenAICompatibleAgentAdapterConfig

type openAICompatibleAgentProvider struct {
	providerID           string
	modelID              string
	endpoint             string
	endpointFingerprint  string
	completionTokenField string
}

type deepSeekAgentAdapter struct {
	provider          openAICompatibleAgentProvider
	runtimeInstanceID string
	credentialAccess  CredentialAccess
	diagnostics       AgentAttemptDiagnosticRecorder
	client            HTTPDoer
	now               func() time.Time
	maxResponseBytes  int64
	toolGateway       loomruntime.AttemptToolGateway
}

type deepSeekAgentDispatch struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	Prompt        string `json:"prompt"`
	// verifier is set only for the governed independent-verifier dispatch
	// kind; it is not part of the wire payload.
	verifier bool
}

type deepSeekAgentProviderFailure struct {
	reason string
	stage  string
}

func (failure *deepSeekAgentProviderFailure) Error() string {
	return "Agent provider request failed"
}

func NewDeepSeekAgentAdapter(
	config DeepSeekAgentAdapterConfig,
) (supervisor.RuntimeAdapter, error) {
	return newOpenAICompatibleAgentAdapter(config, deepSeekAgentProvider())
}

func NewKimiAgentAdapter(
	config OpenAICompatibleAgentAdapterConfig,
) (supervisor.RuntimeAdapter, error) {
	return newOpenAICompatibleAgentAdapter(config, kimiAgentProvider())
}

func NewMiniMaxAgentAdapter(
	config OpenAICompatibleAgentAdapterConfig,
) (supervisor.RuntimeAdapter, error) {
	return newOpenAICompatibleAgentAdapter(config, miniMaxAgentProvider())
}

func newOpenAICompatibleAgentAdapter(
	config OpenAICompatibleAgentAdapterConfig,
	provider openAICompatibleAgentProvider,
) (supervisor.RuntimeAdapter, error) {
	if !validAdapterID(config.RuntimeInstanceID) ||
		nilInterface(config.CredentialAccess) || nilInterface(config.Diagnostics) ||
		nilInterface(config.Client) ||
		config.Now == nil || config.MaxResponseBytes < 256 ||
		config.MaxResponseBytes > 1<<20 || !validOpenAICompatibleAgentProvider(provider) {
		return nil, ErrInvalidOpenAICompatibleAgentAdapter
	}
	return &deepSeekAgentAdapter{
		provider:          provider,
		runtimeInstanceID: config.RuntimeInstanceID,
		credentialAccess:  config.CredentialAccess,
		diagnostics:       config.Diagnostics,
		client:            config.Client,
		now:               config.Now,
		maxResponseBytes:  config.MaxResponseBytes,
		toolGateway:       config.ToolGateway,
	}, nil
}

func NewSystemDeepSeekAgentAdapter(
	runtimeInstanceID string,
	credentialAccess CredentialAccess,
	diagnostics AgentAttemptDiagnosticRecorder,
	now func() time.Time,
	timeout time.Duration,
	maxResponseBytes int64,
	toolGateway loomruntime.AttemptToolGateway,
) (supervisor.RuntimeAdapter, error) {
	return newSystemOpenAICompatibleAgentAdapter(
		runtimeInstanceID,
		credentialAccess,
		diagnostics,
		now,
		timeout,
		maxResponseBytes,
		deepSeekAgentProvider(),
		toolGateway,
	)
}

func NewSystemKimiAgentAdapter(
	runtimeInstanceID string,
	credentialAccess CredentialAccess,
	diagnostics AgentAttemptDiagnosticRecorder,
	now func() time.Time,
	timeout time.Duration,
	maxResponseBytes int64,
	toolGateway loomruntime.AttemptToolGateway,
) (supervisor.RuntimeAdapter, error) {
	return newSystemOpenAICompatibleAgentAdapter(
		runtimeInstanceID, credentialAccess, diagnostics, now, timeout,
		maxResponseBytes, kimiAgentProvider(), toolGateway,
	)
}

func NewSystemMiniMaxAgentAdapter(
	runtimeInstanceID string,
	credentialAccess CredentialAccess,
	diagnostics AgentAttemptDiagnosticRecorder,
	now func() time.Time,
	timeout time.Duration,
	maxResponseBytes int64,
	toolGateway loomruntime.AttemptToolGateway,
) (supervisor.RuntimeAdapter, error) {
	return newSystemOpenAICompatibleAgentAdapter(
		runtimeInstanceID, credentialAccess, diagnostics, now, timeout,
		maxResponseBytes, miniMaxAgentProvider(), toolGateway,
	)
}

func newSystemOpenAICompatibleAgentAdapter(
	runtimeInstanceID string,
	credentialAccess CredentialAccess,
	diagnostics AgentAttemptDiagnosticRecorder,
	now func() time.Time,
	timeout time.Duration,
	maxResponseBytes int64,
	provider openAICompatibleAgentProvider,
	toolGateway loomruntime.AttemptToolGateway,
) (supervisor.RuntimeAdapter, error) {
	if timeout <= 0 || timeout > 2*time.Minute {
		return nil, ErrInvalidOpenAICompatibleAgentAdapter
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, ErrInvalidOpenAICompatibleAgentAdapter
	}
	privateTransport := transport.Clone()
	privateTransport.Proxy = nil
	privateTransport.DisableCompression = true
	return newOpenAICompatibleAgentAdapter(OpenAICompatibleAgentAdapterConfig{
		RuntimeInstanceID: runtimeInstanceID,
		CredentialAccess:  credentialAccess,
		Diagnostics:       diagnostics,
		Client: &http.Client{
			Transport: privateTransport,
			Timeout:   timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Now:              now,
		MaxResponseBytes: maxResponseBytes,
		ToolGateway:      toolGateway,
	}, provider)
}

func (*deepSeekAgentAdapter) AdapterType() string {
	return LoomNativeAgentAdapterType
}

func (*deepSeekAgentAdapter) AcceptsAgentInputs() bool { return true }

func (*deepSeekAgentAdapter) AgentAttemptRestartContract() string {
	return loomruntime.AgentAttemptRestartLoomOwnedCheckpointV1
}

func (adapter *deepSeekAgentAdapter) ValidateAgentAttemptRestartBinding(
	binding loomruntime.FrozenExecutionBinding,
) error {
	if adapter == nil {
		return ErrAgentExecutionBindingChanged
	}
	validated, err := loomruntime.ValidateFrozenExecutionBinding(binding)
	if err != nil || validated.HarnessAdapter != LoomNativeAgentAdapterType ||
		validated.RuntimeInstanceID != adapter.runtimeInstanceID ||
		validated.ProviderID != adapter.provider.providerID ||
		validated.ProviderAccountID == "" ||
		validated.ModelID != adapter.provider.modelID ||
		validated.AuthMode != loomruntime.AuthBrokered ||
		validated.EndpointFingerprint != adapter.provider.endpointFingerprint ||
		validated.CredentialReference == "" || validated.CredentialRevision <= 0 {
		return ErrAgentExecutionBindingChanged
	}
	return nil
}

func (adapter *deepSeekAgentAdapter) RuntimeInstanceID() string {
	if adapter == nil {
		return ""
	}
	return adapter.runtimeInstanceID
}

func (adapter *deepSeekAgentAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	if adapter == nil || ctx == nil || nilInterface(request.FrameSink) {
		return supervisor.AdapterResult{}, ErrInvalidDeepSeekAgentAdapter
	}
	started := time.Now()
	if err := adapter.validateRequest(request); err != nil {
		diagnosticErr := adapter.recordDiagnostic(
			ctx, request, started, "agent_attempt_dispatch", "failed",
			"binding_changed", false,
		)
		if diagnosticErr != nil {
			return supervisor.AdapterResult{}, errors.Join(err, diagnosticErr)
		}
		return supervisor.AdapterResult{}, err
	}
	dispatch, err := decodeDeepSeekAgentDispatch(request.Dispatch.Payload())
	if err != nil {
		return supervisor.AdapterResult{}, err
	}

	var response deepSeekAgentResponse
	credentialErr := adapter.credentialAccess.UseCredential(
		ctx,
		request.ExecutionBinding,
		func(leaseContext context.Context, secret []byte) error {
			if len(secret) == 0 || len(secret) > 8192 {
				return ErrAgentCredentialUnavailable
			}
			toolBinding := loomruntime.ToolCallBinding{
				ConversationID:         request.ContextCapsule.ConversationID,
				WorkItemID:             request.Binding.WorkItemID,
				RunID:                  request.Binding.RunID,
				ClaimGeneration:        request.Binding.ClaimGeneration,
				RuntimeInstanceID:      request.Binding.RuntimeInstanceID,
				AgentInstanceID:        request.Binding.SenderAgentInstanceID,
				ExecutionBindingDigest: request.ExecutionBinding.BindingDigest,
				CapsuleDigest:          request.ContextCapsule.CapsuleDigest,
				ClaimID:                request.ClaimID,
				IncidentID:             request.IncidentID,
				JourneyID:              request.IncidentID,
			}
			candidate, callErr := adapter.callProvider(
				leaseContext, dispatch.Prompt, secret, request.ContextDelivery,
				request.AgentInputs, toolBinding,
			)
			if callErr == nil {
				response = candidate
			}
			return callErr
		},
	)
	if credentialErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			_ = adapter.recordDiagnostic(
				context.WithoutCancel(ctx), request, started,
				"agent_attempt_dispatch", "failed", "timeout", true,
			)
			return supervisor.AdapterResult{}, ctxErr
		}
		reason := "credential_unavailable"
		var providerFailure *deepSeekAgentProviderFailure
		if errors.As(credentialErr, &providerFailure) {
			reason = providerFailure.reason
		}
		stage, retryable := deepSeekAgentDiagnosticFailure(reason, credentialErr)
		if providerFailure != nil && providerFailure.stage != "" {
			stage = providerFailure.stage
		}
		if err := adapter.recordDiagnostic(
			ctx, request, started, stage, "failed", reason, retryable,
		); err != nil {
			return supervisor.AdapterResult{}, err
		}
		return adapter.publish(ctx, request, "", "failed", reason, nil)
	}
	if err := adapter.recordDiagnostic(
		ctx, request, started, "agent_attempt_dispatch", "succeeded", "", false,
	); err != nil {
		return supervisor.AdapterResult{}, err
	}
	status := "succeeded"
	reason := ""
	if dispatch.verifier {
		status, reason = verifierTerminalFromVerdict(response.content)
	}
	return adapter.publish(
		ctx,
		request,
		response.content,
		status,
		reason,
		response.accounting,
	)
}

func (adapter *deepSeekAgentAdapter) recordDiagnostic(
	ctx context.Context,
	request supervisor.AdapterRequest,
	started time.Time,
	stage,
	result,
	errorCode string,
	retryable bool,
) error {
	if adapter == nil || nilInterface(adapter.diagnostics) {
		return ErrAgentDiagnosticsUnavailable
	}
	occurredAt := adapter.now()
	if occurredAt.IsZero() || occurredAt.Location() != time.UTC {
		return ErrAgentDiagnosticsUnavailable
	}
	elapsed := time.Since(started)
	if elapsed < 0 {
		elapsed = 0
	}
	if err := adapter.diagnostics.RecordAgentAttemptDiagnostic(
		ctx,
		AgentAttemptDiagnostic{
			OccurredAt:        occurredAt,
			IncidentID:        request.Dispatch.CorrelationID(),
			ProviderID:        request.ExecutionBinding.ProviderID,
			ProviderAccountID: request.ExecutionBinding.ProviderAccountID,
			ModelID:           request.ExecutionBinding.ModelID,
			Stage:             stage,
			Elapsed:           elapsed,
			Result:            result,
			ErrorCode:         errorCode,
			Retryable:         retryable,
		},
	); err != nil {
		return ErrAgentDiagnosticsUnavailable
	}
	return nil
}

func (adapter *deepSeekAgentAdapter) validateRequest(
	request supervisor.AdapterRequest,
) error {
	if err := adapter.ValidateAgentAttemptRestartBinding(request.ExecutionBinding); err != nil {
		return err
	}
	binding := request.ExecutionBinding
	if binding.RuntimeInstanceID != adapter.runtimeInstanceID ||
		binding.RuntimeInstanceID != request.Binding.RuntimeInstanceID ||
		binding.HarnessAdapter != LoomNativeAgentAdapterType {
		return ErrAgentExecutionBindingChanged
	}
	if request.Dispatch.Type() != bridgev1.MessageDispatch ||
		request.Dispatch.Sequence() != 1 ||
		request.Dispatch.WorkItemID() != request.Binding.WorkItemID ||
		request.Dispatch.RunID() != request.Binding.RunID ||
		request.Dispatch.ClaimGeneration() != request.Binding.ClaimGeneration ||
		request.Dispatch.RuntimeInstanceID() != request.Binding.RuntimeInstanceID ||
		request.Dispatch.SenderAgentInstanceID() != request.Binding.SenderAgentInstanceID {
		return errors.Join(ErrDeepSeekAgentProtocol, supervisor.ErrBridgeSession)
	}
	if request.ContextRetriever != nil {
		if nilInterface(request.ContextRetriever) ||
			request.ContextDelivery == nil || nilInterface(request.ContextDelivery) ||
			request.ContextCapsule == (contextcapsule.AuthorityRecord{}) ||
			!containsNativeCapability(
				binding.Capabilities,
				loomruntime.CapabilityContextRetrieval,
			) {
			return ErrAgentExecutionBindingChanged
		}
		authority, authorityErr := contextcapsule.ValidateAuthorityRecord(request.ContextCapsule)
		dispatch, dispatchErr := contextcapsule.DecodeDispatchPayload(request.Dispatch.Payload())
		if authorityErr != nil || dispatchErr != nil ||
			authority.AgentID != request.Binding.SenderAgentInstanceID ||
			authority.ProviderID != binding.ProviderID ||
			authority.ProviderAccountID != binding.ProviderAccountID ||
			authority.ModelID != binding.ModelID || authority.AuthMode != string(binding.AuthMode) ||
			dispatch.CapsuleDigest != authority.CapsuleDigest ||
			dispatch.DisclosureReceiptDigest != authority.DisclosureReceiptDigest {
			return ErrAgentExecutionBindingChanged
		}
	} else if request.ContextDelivery != nil {
		// A delivery without a retriever is inconsistent and rejected. A
		// completely context-free Execute (no retriever, no delivery, empty
		// capsule) is valid even when the binding lists the context_retrieval
		// capability: the independent verifier runs exactly this shape, and
		// governed attempts always carry retriever+delivery together.
		return ErrAgentExecutionBindingChanged
	}
	return nil
}

func containsNativeCapability(capabilities []string, target string) bool {
	for _, capability := range capabilities {
		if capability == target {
			return true
		}
	}
	return false
}

type deepSeekAgentResponse struct {
	content    string
	toolCalls  []nativeAgentToolCall
	accounting *work.RunAccounting
}

type nativeAgentToolCall struct {
	ID      string
	Kind    permissions.ToolKind
	Context *contextToolCall
	Web     *webAgentToolCall
}

type contextToolCall struct {
	ID       string
	Proposal contextcapsule.RetrievalProposal
}

type webAgentToolCall struct {
	ID   string
	Kind permissions.ToolKind
	Call permissions.ProposedCall
}

type openAICompatibleToolCallWire struct {
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAICompatibleMessage struct {
	Role           string
	Content        string
	MutableContent []byte
	ToolCalls      []openAICompatibleToolCallWire
	ToolCallID     string
}

type openAICompatibleContextToolResult struct {
	SchemaVersion int                       `json:"schema_version"`
	ItemID        string                    `json:"item_id"`
	ContentDigest string                    `json:"content_digest"`
	Trust         contextcapsule.TrustClass `json:"trust"`
	Scope         contextcapsule.Scope      `json:"scope"`
	SourceType    contextcapsule.SourceType `json:"source_type"`
	SourceRef     string                    `json:"source_ref"`
	ArtifactRef   string                    `json:"artifact_ref,omitempty"`
	Content       string                    `json:"content"`
}

func (adapter *deepSeekAgentAdapter) callProvider(
	ctx context.Context,
	prompt string,
	secret []byte,
	delivery contextcapsule.DeliveryBroker,
	inputs loomruntime.AgentInputSource,
	binding loomruntime.ToolCallBinding,
) (deepSeekAgentResponse, error) {
	tools := []prompting.ToolCapability(nil)
	if delivery != nil {
		tools = []prompting.ToolCapability{prompting.ToolContextRead}
	}
	for _, kind := range adapter.allowedWebTools() {
		switch kind {
		case permissions.ToolWebSearch:
			tools = append(tools, prompting.ToolWebSearch)
		case permissions.ToolWebFetch:
			tools = append(tools, prompting.ToolWebFetch)
		}
	}
	systemPrompt, err := prompting.BuildSystemPrompt(prompting.Profile{
		Mode: prompting.ModeAgent, ProviderID: adapter.provider.providerID,
		ModelID: adapter.provider.modelID, HarnessAdapter: LoomNativeAgentAdapterType,
		Tools: tools,
	})
	if err != nil {
		return deepSeekAgentResponse{}, ErrDeepSeekAgentProtocol
	}
	messages := []openAICompatibleMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}
	return adapter.callProviderMessages(ctx, messages, secret, delivery, inputs, binding)
}

func (adapter *deepSeekAgentAdapter) callProviderMessages(
	ctx context.Context,
	messages []openAICompatibleMessage,
	secret []byte,
	delivery contextcapsule.DeliveryBroker,
	inputs loomruntime.AgentInputSource,
	binding loomruntime.ToolCallBinding,
) (deepSeekAgentResponse, error) {
	defer func() { clearOpenAICompatibleMessages(messages) }()
	var total *work.RunAccounting
	contextSequence := int64(1)
	for {
		response, err := adapter.callProviderExchange(
			ctx, messages, secret, delivery, contextSequence, binding,
		)
		if err != nil {
			return deepSeekAgentResponse{}, err
		}
		if response.contextDeliveries > 0 {
			contextSequence += response.contextDeliveries
		}
		total, err = combineAgentAccounting(total, response.accounting)
		if err != nil {
			return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{
				reason: "provider_http", stage: "provider_http",
			}
		}
		if inputs == nil {
			response.accounting = total
			return response.deepSeekAgentResponse, nil
		}
		outputDigest := sha256.Sum256([]byte(response.content))
		checkpoint := loomruntime.AgentInputCheckpoint{
			OutputDigest: hex.EncodeToString(outputDigest[:]),
		}
		var batch loomruntime.AgentInputBatch
		var available bool
		var inputErr error
		if durable, ok := inputs.(loomruntime.DurableAgentInputSource); ok {
			payload := loomruntime.AgentInputCheckpointPayload{
				Checkpoint: checkpoint, Content: []byte(response.content),
			}
			batch, available, inputErr = durable.NextAgentInputFromDurableCheckpoint(
				ctx, payload,
			)
			payload.Close()
		} else {
			batch, available, inputErr = inputs.NextAgentInput(ctx, checkpoint)
		}
		if inputErr != nil {
			batch.Close()
			return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{
				reason: "agent_input_unavailable", stage: "agent_attempt_dispatch",
			}
		}
		if !available {
			batch.Close()
			response.accounting = total
			return response.deepSeekAgentResponse, nil
		}
		inputPrompt, promptErr := loomruntime.RenderAgentInput(&batch)
		batch.Close()
		if promptErr != nil {
			return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{
				reason: "agent_input_unavailable", stage: "agent_attempt_dispatch",
			}
		}
		messages = append(messages,
			openAICompatibleMessage{Role: "assistant", Content: response.content},
			openAICompatibleMessage{Role: "user", MutableContent: inputPrompt},
		)
	}
}

type openAICompatibleExchange struct {
	deepSeekAgentResponse
	contextDeliveries int64
}

func (adapter *deepSeekAgentAdapter) callProviderExchange(
	ctx context.Context,
	messages []openAICompatibleMessage,
	secret []byte,
	delivery contextcapsule.DeliveryBroker,
	contextSequence int64,
	binding loomruntime.ToolCallBinding,
) (openAICompatibleExchange, error) {
	history := append([]openAICompatibleMessage(nil), messages...)
	ownedStart := len(history)
	defer func() { clearOpenAICompatibleMessages(history[ownedStart:]) }()
	var total *work.RunAccounting
	var pending *attemptpayload.Binding
	var exhaustionPrompted bool
	var bestContent string
	deliveries := int64(0)
	for {
		response, err := adapter.callProviderRound(
			ctx, history, secret,
			delivery != nil && deliveries < contextToolMaxCallsPerExchange,
		)
		if err != nil {
			return openAICompatibleExchange{}, err
		}
		if response.content != "" {
			bestContent = response.content
		}
		if pending != nil {
			if err := delivery.Acknowledge(
				ctx, *pending, attemptpayload.ProofProviderContinuation,
			); err != nil {
				return openAICompatibleExchange{}, &deepSeekAgentProviderFailure{
					reason: "context_delivery_unavailable", stage: "context_delivery",
				}
			}
			pending = nil
		}
		total, err = combineAgentAccounting(total, response.accounting)
		if err != nil {
			return openAICompatibleExchange{}, &deepSeekAgentProviderFailure{
				reason: "provider_http", stage: "provider_http",
			}
		}
		if len(response.toolCalls) == 0 {
			if response.content == "" {
				response.content = "No further information was produced for this attempt after the available tools were used."
			}
			response.accounting = total
			return openAICompatibleExchange{
				deepSeekAgentResponse: response, contextDeliveries: deliveries,
			}, nil
		}
		if deliveries >= contextToolMaxCallsPerExchange {
			if !exhaustionPrompted {
				exhaustionPrompted = true
				history = append(history, openAICompatibleMessage{
					Role:           "user",
					MutableContent: []byte(contextReadExhaustionDirective),
				})
				continue
			}
			// The model still requested tools after the exhaustion directive;
			// end the exchange cleanly with the best content produced so far
			// so the attempt can complete instead of failing terminally.
			content := bestContent
			if content == "" {
				content = "No further information was produced for this attempt after the available tools were used."
			}
			response.content = content
			response.accounting = total
			return openAICompatibleExchange{
				deepSeekAgentResponse: response, contextDeliveries: deliveries,
			}, nil
		}
		assistantWires := make([]openAICompatibleToolCallWire, 0, len(response.toolCalls))
		results := make([]openAICompatibleMessage, 0, len(response.toolCalls))
		for _, toolCall := range response.toolCalls {
			if toolCall.Web != nil {
				webResult, webErr := adapter.executeWebToolCall(
					ctx, binding, *toolCall.Web, int64(deliveries)+1,
				)
				if webErr != nil {
					return openAICompatibleExchange{}, webErr
				}
				assistantWires = append(assistantWires, webToolCallWire(*toolCall.Web))
				results = append(results, openAICompatibleMessage{
					Role: "tool", ToolCallID: toolCall.Web.ID,
					MutableContent: webResult,
				})
				deliveries++
				continue
			}
			if toolCall.Context == nil || delivery == nil {
				return openAICompatibleExchange{}, &deepSeekAgentProviderFailure{
					reason: "context_retrieval_denied", stage: "context_retrieval",
				}
			}
			payload, retrievalErr := delivery.Prepare(
				ctx,
				toolCall.Context.Proposal,
				contextcapsule.DeliveryRequest{
					Sequence:    contextSequence + deliveries,
					ContentType: "application/json",
				},
				marshalContextToolResult,
			)
			if retrievalErr != nil {
				payload.Close()
				if ctxErr := ctx.Err(); ctxErr != nil {
					return openAICompatibleExchange{}, ctxErr
				}
				// A denied or unretrievable Context item is a bounded tool
				// result, not a terminal attempt failure: the model may
				// recover with a different item or another tool (e.g. the
				// governed web gateway). Never leak the retrieval reason.
				if errors.Is(retrievalErr, contextcapsule.ErrContextRetrievalDenied) ||
					errors.Is(retrievalErr, contextcapsule.ErrContextItemNotRetrievable) {
					assistantWires = append(assistantWires, contextToolCallWire(*toolCall.Context))
					results = append(results, openAICompatibleMessage{
						Role: "tool", ToolCallID: toolCall.Context.ID,
						MutableContent: []byte(contextReadBoundedDenial),
					})
					deliveries++
					continue
				}
				// A second Context-read in the same step cannot be dispatched
				// again (native adapters run Context reads exclusively). The
				// step already carries the first dispatched call, so surface
				// the same bounded denial inline (no new dispatch, no fact)
				// and let the model recover and answer. This keeps the step
				// finalizable as FINAL (all dispatched calls delivered) instead
				// of failing terminally or leaving an undelivered call that
				// stalls the attempt-loop on a StepEnd conflict.
				assistantWires = append(assistantWires, contextToolCallWire(*toolCall.Context))
				results = append(results, openAICompatibleMessage{
					Role: "tool", ToolCallID: toolCall.Context.ID,
					MutableContent: []byte(contextReadBoundedDenial),
				})
				deliveries++
				continue
			}
			assistantWires = append(assistantWires, contextToolCallWire(*toolCall.Context))
			results = append(results, openAICompatibleMessage{
				Role: "tool", ToolCallID: toolCall.Context.ID,
				MutableContent: bytes.Clone(payload.Content),
			})
			bindingRef := payload.Binding
			payload.Close()
			pending = &bindingRef
			deliveries++
		}
		history = append(history,
			openAICompatibleMessage{Role: "assistant", ToolCalls: assistantWires},
		)
		history = append(history, results...)
	}
}

func (adapter *deepSeekAgentAdapter) callProviderRound(
	ctx context.Context,
	messages []openAICompatibleMessage,
	secret []byte,
	allowContextTool bool,
) (deepSeekAgentResponse, error) {
	wireMessages, err := marshalOpenAICompatibleMessages(messages)
	if err != nil {
		return deepSeekAgentResponse{}, ErrDeepSeekAgentProtocol
	}
	defer clearOpenAICompatibleWireMessages(wireMessages)
	requestBody := struct {
		Model               string            `json:"model"`
		Messages            []json.RawMessage `json:"messages"`
		Tools               []any             `json:"tools,omitempty"`
		ToolChoice          string            `json:"tool_choice,omitempty"`
		Stream              bool              `json:"stream"`
		ParallelToolCalls   bool              `json:"parallel_tool_calls,omitempty"`
		MaxTokens           int               `json:"max_tokens,omitempty"`
		MaxCompletionTokens int               `json:"max_completion_tokens,omitempty"`
	}{
		Model:    adapter.provider.modelID,
		Messages: wireMessages,
		Stream:   false,
	}
	if allowContextTool || len(adapter.allowedWebTools()) > 0 {
		requestBody.Tools = []any{}
		requestBody.ToolChoice = "auto"
		// The exchange loop executes one governed tool call per round; ask
		// the provider for a single tool call at a time.
		requestBody.ParallelToolCalls = false
	}
	if allowContextTool {
		requestBody.Tools = append(requestBody.Tools, contextToolDefinition())
	}
	for _, kind := range adapter.allowedWebTools() {
		switch kind {
		case permissions.ToolWebSearch:
			requestBody.Tools = append(requestBody.Tools, webSearchToolDefinition())
		case permissions.ToolWebFetch:
			requestBody.Tools = append(requestBody.Tools, webFetchToolDefinition())
		}
	}
	if adapter.provider.completionTokenField == "max_tokens" {
		requestBody.MaxTokens = 2048
	} else {
		requestBody.MaxCompletionTokens = 2048
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return deepSeekAgentResponse{}, ErrDeepSeekAgentProtocol
	}
	defer zeroNativeAgentBytes(payload)
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		adapter.provider.endpoint,
		bytes.NewReader(payload),
	)
	if err != nil || !exactOpenAICompatibleAgentURL(request.URL, adapter.provider.endpoint) {
		return deepSeekAgentResponse{}, ErrDeepSeekAgentProtocol
	}
	request.Header.Set("Authorization", "Bearer "+string(secret))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, requestErr := adapter.client.Do(request)
	request.Header.Del("Authorization")
	if requestErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return deepSeekAgentResponse{}, ctxErr
		}
		if transportTimeout(requestErr) {
			return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{
				reason: "timeout", stage: "provider_connect",
			}
		}
		return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_unavailable"}
	}
	if response == nil || response.Body == nil {
		return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_unavailable"}
	}
	defer response.Body.Close()
	if response.Request != nil && !exactOpenAICompatibleAgentURL(
		response.Request.URL,
		adapter.provider.endpoint,
	) {
		return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_http"}
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, adapter.maxResponseBytes+1))
	if readErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return deepSeekAgentResponse{}, ctxErr
		}
		if transportTimeout(readErr) {
			return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{
				reason: "timeout", stage: "provider_http",
			}
		}
		return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_http"}
	}
	if int64(len(body)) > adapter.maxResponseBytes {
		return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_http"}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{
			reason: deepSeekAgentHTTPFailureReason(response.StatusCode),
		}
	}
	return decodeDeepSeekAgentResponse(body, adapter.provider.modelID)
}

func clearOpenAICompatibleMessages(messages []openAICompatibleMessage) {
	for index := range messages {
		messages[index].Content = ""
		zeroNativeAgentBytes(messages[index].MutableContent)
		messages[index].MutableContent = nil
		messages[index].ToolCalls = nil
		messages[index].ToolCallID = ""
	}
}

func marshalOpenAICompatibleMessages(
	messages []openAICompatibleMessage,
) ([]json.RawMessage, error) {
	encoded := make([]json.RawMessage, 0, len(messages))
	for _, message := range messages {
		content := message.MutableContent
		ownedContent := false
		if content == nil && message.Content != "" {
			content = []byte(message.Content)
			ownedContent = true
		}
		var contentJSON json.RawMessage
		if len(content) != 0 {
			if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
				clearOpenAICompatibleWireMessages(encoded)
				return nil, ErrDeepSeekAgentProtocol
			}
			contentJSON = appendOpenAICompatibleJSONString(nil, content)
		}
		if ownedContent {
			zeroNativeAgentBytes(content)
		}
		wire, err := json.Marshal(struct {
			Role       string                         `json:"role"`
			Content    json.RawMessage                `json:"content,omitempty"`
			ToolCalls  []openAICompatibleToolCallWire `json:"tool_calls,omitempty"`
			ToolCallID string                         `json:"tool_call_id,omitempty"`
		}{
			Role: message.Role, Content: contentJSON,
			ToolCalls: message.ToolCalls, ToolCallID: message.ToolCallID,
		})
		zeroNativeAgentBytes(contentJSON)
		if err != nil {
			clearOpenAICompatibleWireMessages(encoded)
			return nil, ErrDeepSeekAgentProtocol
		}
		encoded = append(encoded, wire)
	}
	return encoded, nil
}

func appendOpenAICompatibleJSONString(target, content []byte) []byte {
	const hexadecimal = "0123456789abcdef"
	target = append(target, '"')
	for _, value := range content {
		switch value {
		case '"', '\\':
			target = append(target, '\\', value)
		case '\b':
			target = append(target, '\\', 'b')
		case '\f':
			target = append(target, '\\', 'f')
		case '\n':
			target = append(target, '\\', 'n')
		case '\r':
			target = append(target, '\\', 'r')
		case '\t':
			target = append(target, '\\', 't')
		default:
			if value < 0x20 {
				target = append(
					target, '\\', 'u', '0', '0',
					hexadecimal[value>>4], hexadecimal[value&0x0f],
				)
				continue
			}
			target = append(target, value)
		}
	}
	return append(target, '"')
}

func clearOpenAICompatibleWireMessages(messages []json.RawMessage) {
	for index := range messages {
		zeroNativeAgentBytes(messages[index])
		messages[index] = nil
	}
}

func zeroNativeAgentBytes(content []byte) {
	for index := range content {
		content[index] = 0
	}
}

func decodeDeepSeekAgentResponse(
	body []byte,
	expectedModel string,
) (deepSeekAgentResponse, error) {
	var decoded struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role      string            `json:"role"`
				Content   string            `json:"content"`
				ToolCalls []json.RawMessage `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens       int64 `json:"prompt_tokens"`
			CompletionTokens   int64 `json:"completion_tokens"`
			TotalTokens        int64 `json:"total_tokens"`
			PromptTokenDetails *struct {
				CachedTokens int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if rejectNativeDuplicateJSONKeys(body) != nil ||
		json.Unmarshal(body, &decoded) != nil ||
		!validNativeAgentResponseModel(decoded.Model) ||
		len(decoded.Choices) != 1 ||
		decoded.Choices[0].Message.Role != "assistant" {
		return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_http"}
	}
	message := decoded.Choices[0].Message
	content := strings.TrimSpace(message.Content)
	// A tool call may be accompanied by a short content preamble (DeepSeek
	// often emits "I'll search..." before the function call). A response with
	// no tool call may carry empty content; the exchange loop substitutes a
	// bounded fallback so an exhausted model still completes the attempt.
	if len(message.ToolCalls) > contextToolMaxCallsPerExchange ||
		len(content) > deepSeekAgentMaxContentBytes ||
		!utf8.ValidString(content) || strings.IndexByte(content, 0) >= 0 {
		return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_http"}
	}
	response := deepSeekAgentResponse{content: content}
	response.toolCalls = make([]nativeAgentToolCall, 0, len(message.ToolCalls))
	for _, raw := range message.ToolCalls {
		wire, err := decodeContextToolCallWire(raw)
		if err != nil {
			return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_http"}
		}
		switch wire.Function.Name {
		case "loom_read_context":
			toolCall, decodeErr := decodeContextToolCall(wire)
			if decodeErr != nil {
				return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_http"}
			}
			response.toolCalls = append(response.toolCalls, nativeAgentToolCall{
				ID: wire.ID, Kind: permissions.ToolMCPTool, Context: &toolCall,
			})
		case "loom_web_search", "loom_web_fetch":
			webCall, decodeErr := decodeWebToolCall(wire)
			if decodeErr != nil {
				return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_http"}
			}
			response.toolCalls = append(response.toolCalls, nativeAgentToolCall{
				ID: webCall.ID, Kind: webCall.Kind, Web: &webCall,
			})
		default:
			return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_http"}
		}
	}
	if decoded.Usage == nil {
		return response, nil
	}
	accounting := work.RunAccounting{
		UsageObserved: true,
		InputTokens:   decoded.Usage.PromptTokens,
		OutputTokens:  decoded.Usage.CompletionTokens,
		TotalTokens:   decoded.Usage.TotalTokens,
	}
	if decoded.Usage.PromptTokenDetails != nil {
		accounting.CacheReadTokens = decoded.Usage.PromptTokenDetails.CachedTokens
	}
	if work.ValidateRunAccounting(accounting) != nil {
		return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{reason: "provider_http"}
	}
	response.accounting = &accounting
	return response, nil
}

func decodeContextToolCallWire(raw json.RawMessage) (openAICompatibleToolCallWire, error) {
	if len(raw) == 0 || len(raw) > contextToolMaxArgumentsBytes*2 ||
		rejectNativeDuplicateJSONKeys(raw) != nil {
		return openAICompatibleToolCallWire{}, ErrDeepSeekAgentProtocol
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var wire openAICompatibleToolCallWire
	if decoder.Decode(&wire) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return openAICompatibleToolCallWire{}, ErrDeepSeekAgentProtocol
	}
	return wire, nil
}

func (adapter *deepSeekAgentAdapter) allowedWebTools() []permissions.ToolKind {
	if adapter == nil || adapter.toolGateway == nil {
		return nil
	}
	provider, ok := adapter.toolGateway.(loomruntime.ToolCallCapabilityProvider)
	if !ok {
		return nil
	}
	allowed := []permissions.ToolKind(nil)
	for _, kind := range provider.AllowedToolCalls() {
		switch kind {
		case permissions.ToolWebSearch, permissions.ToolWebFetch:
			allowed = append(allowed, kind)
		}
	}
	return allowed
}

func validNativeAgentResponseModel(value string) bool {
	if value == "" || len(value) > 128 || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '_' || character == '-' || character == '.' ||
			character == ':' || character == '/' {
			continue
		}
		return false
	}
	return true
}

func webSearchToolDefinition() any {
	return struct {
		Type     string `json:"type"`
		Function any    `json:"function"`
	}{
		Type: "function",
		Function: struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Strict      bool   `json:"strict"`
			Parameters  any    `json:"parameters"`
		}{
			Name:        "loom_web_search",
			Description: "Search the live internet through the Loom-governed web gateway and return concise results.",
			Strict:      true,
			Parameters: struct {
				Type                 string         `json:"type"`
				Properties           map[string]any `json:"properties"`
				Required             []string       `json:"required"`
				AdditionalProperties bool           `json:"additionalProperties"`
			}{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{"type": "string"},
				},
				Required:             []string{"query"},
				AdditionalProperties: false,
			},
		},
	}
}

func webFetchToolDefinition() any {
	return struct {
		Type     string `json:"type"`
		Function any    `json:"function"`
	}{
		Type: "function",
		Function: struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Strict      bool   `json:"strict"`
			Parameters  any    `json:"parameters"`
		}{
			Name:        "loom_web_fetch",
			Description: "Fetch one public URL through the Loom-governed web gateway and return its text content.",
			Strict:      true,
			Parameters: struct {
				Type                 string         `json:"type"`
				Properties           map[string]any `json:"properties"`
				Required             []string       `json:"required"`
				AdditionalProperties bool           `json:"additionalProperties"`
			}{
				Type: "object",
				Properties: map[string]any{
					"url": map[string]any{"type": "string"},
				},
				Required:             []string{"url"},
				AdditionalProperties: false,
			},
		},
	}
}

func decodeWebToolCall(wire openAICompatibleToolCallWire) (webAgentToolCall, error) {
	if !validContextToolIdentifier(wire.ID, 128) || wire.Type != "function" ||
		wire.Function.Arguments == "" ||
		len(wire.Function.Arguments) > contextToolMaxArgumentsBytes ||
		rejectNativeDuplicateJSONKeys([]byte(wire.Function.Arguments)) != nil {
		return webAgentToolCall{}, ErrDeepSeekAgentProtocol
	}
	decoder := json.NewDecoder(strings.NewReader(wire.Function.Arguments))
	decoder.DisallowUnknownFields()
	switch wire.Function.Name {
	case "loom_web_search":
		var arguments struct {
			Query string `json:"query"`
		}
		if decoder.Decode(&arguments) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			!validWebToolArgument(arguments.Query, 512) {
			return webAgentToolCall{}, ErrDeepSeekAgentProtocol
		}
		return webAgentToolCall{
			ID: wire.ID, Kind: permissions.ToolWebSearch,
			Call: permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: arguments.Query},
		}, nil
	case "loom_web_fetch":
		var arguments struct {
			URL string `json:"url"`
		}
		if decoder.Decode(&arguments) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			!validWebToolArgument(arguments.URL, 2048) {
			return webAgentToolCall{}, ErrDeepSeekAgentProtocol
		}
		return webAgentToolCall{
			ID: wire.ID, Kind: permissions.ToolWebFetch,
			Call: permissions.ProposedCall{Tool: permissions.ToolWebFetch, Path: arguments.URL},
		}, nil
	default:
		return webAgentToolCall{}, ErrDeepSeekAgentProtocol
	}
}

func validWebToolArgument(value string, max int) bool {
	if value == "" || len(value) > max || !utf8.ValidString(value) ||
		strings.TrimSpace(value) != value || strings.IndexByte(value, 0) >= 0 {
		return false
	}
	return true
}

func webToolCallWire(call webAgentToolCall) openAICompatibleToolCallWire {
	wire := openAICompatibleToolCallWire{ID: call.ID, Type: "function"}
	switch call.Kind {
	case permissions.ToolWebSearch:
		wire.Function.Name = "loom_web_search"
		arguments, _ := json.Marshal(struct {
			Query string `json:"query"`
		}{call.Call.Path})
		wire.Function.Arguments = string(arguments)
	case permissions.ToolWebFetch:
		wire.Function.Name = "loom_web_fetch"
		arguments, _ := json.Marshal(struct {
			URL string `json:"url"`
		}{call.Call.Path})
		wire.Function.Arguments = string(arguments)
	}
	return wire
}

func (adapter *deepSeekAgentAdapter) executeWebToolCall(
	ctx context.Context,
	binding loomruntime.ToolCallBinding,
	call webAgentToolCall,
	sequence int64,
) ([]byte, error) {
	if adapter == nil || adapter.toolGateway == nil {
		return nil, &deepSeekAgentProviderFailure{
			reason: "context_retrieval_denied", stage: "context_retrieval",
		}
	}
	// Each governed tool call needs a distinct 1-based sequence so the
	// attempt loop can admit multiple calls in one step sequentially.
	callContext, err := loomruntime.BindToolCallSequence(ctx, sequence)
	if err != nil {
		return nil, &deepSeekAgentProviderFailure{
			reason: "context_retrieval_denied", stage: "context_retrieval",
		}
	}
	gateway, ok := adapter.toolGateway.(loomruntime.AttemptToolGateway)
	if !ok {
		return nil, &deepSeekAgentProviderFailure{
			reason: "context_retrieval_denied", stage: "context_retrieval",
		}
	}
	reader, readerOK := adapter.toolGateway.(loomruntime.ToolCallResultContentReader)
	if !readerOK {
		return nil, &deepSeekAgentProviderFailure{
			reason: "context_retrieval_denied", stage: "context_retrieval",
		}
	}
	result, err := gateway.ExecuteToolCall(callContext, loomruntime.ToolCallEnvelope{
		JobID: binding.WorkItemID, Call: call.Call,
	}, binding)
	if err != nil {
		if ctxErr := callContext.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, &deepSeekAgentProviderFailure{
			reason: "context_retrieval_denied", stage: "context_retrieval",
		}
	}
	if result.Verdict != permissions.VerdictAllow {
		// A governance denial/ask is a bounded tool result, not a terminal
		// attempt failure: the model may recover with a different query or
		// tool. Never leak the denial reason.
		return []byte(`{"error":"tool_denied","message":"The Loom governance gate did not allow this web tool call for this attempt. Use a different query or tool."}`), nil
	}
	content, err := reader.ReadToolCallResultContent(callContext, binding, result)
	if err != nil {
		return []byte(`{"error":"tool_unavailable","message":"The web tool result could not be read for this attempt. Try a different query or tool."}`), nil
	}
	if len(content) == 0 || len(content) > contextToolMaxResultBytes {
		return []byte(`{"error":"tool_unavailable","message":"The web tool result was empty or too large for this attempt."}`), nil
	}
	// Deliver the result to the attempt loop so the call is marked delivered;
	// otherwise the call stays active and a subsequent (Exclusive) tool call
	// in the same step is rejected as a conflict.
	if acknowledger, ok := adapter.toolGateway.(loomruntime.ToolCallResultProofAcknowledger); ok {
		if ackErr := acknowledger.AcknowledgeToolCallResultWithProof(
			callContext, binding, result, attemptpayload.ProofRunStreamToolResult,
		); ackErr != nil && callContext.Err() == nil {
			return []byte(`{"error":"tool_result_delivery_failed","message":"The web tool result could not be delivered to the attempt. Try again."}`), nil
		}
	}
	return content, nil
}

func contextToolDefinition() any {
	return struct {
		Type     string `json:"type"`
		Function any    `json:"function"`
	}{
		Type: "function",
		Function: struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Strict      bool   `json:"strict"`
			Parameters  any    `json:"parameters"`
		}{
			Name:        "loom_read_context",
			Description: "Read one Loom-authorized Context Capsule omission by exact identity.",
			Strict:      true,
			Parameters: struct {
				Type                 string         `json:"type"`
				Properties           map[string]any `json:"properties"`
				Required             []string       `json:"required"`
				AdditionalProperties bool           `json:"additionalProperties"`
			}{
				Type: "object",
				Properties: map[string]any{
					"item_id":        map[string]any{"type": "string"},
					"content_digest": map[string]any{"type": "string", "pattern": "^[0-9a-f]{64}$"},
					"artifact_ref":   map[string]any{"type": "string"},
				},
				Required:             []string{"item_id", "content_digest"},
				AdditionalProperties: false,
			},
		},
	}
}

func decodeContextToolCall(wire openAICompatibleToolCallWire) (contextToolCall, error) {
	if !validContextToolIdentifier(wire.ID, 128) || wire.Type != "function" ||
		wire.Function.Name != "loom_read_context" || wire.Function.Arguments == "" ||
		len(wire.Function.Arguments) > contextToolMaxArgumentsBytes ||
		rejectNativeDuplicateJSONKeys([]byte(wire.Function.Arguments)) != nil {
		return contextToolCall{}, ErrDeepSeekAgentProtocol
	}
	decoder := json.NewDecoder(strings.NewReader(wire.Function.Arguments))
	decoder.DisallowUnknownFields()
	var arguments struct {
		ItemID        string `json:"item_id"`
		ContentDigest string `json:"content_digest"`
		ArtifactRef   string `json:"artifact_ref"`
	}
	if decoder.Decode(&arguments) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		!validContextToolIdentifier(arguments.ItemID, 512) ||
		!validContextToolDigest(arguments.ContentDigest) ||
		arguments.ArtifactRef != "" && !validContextToolIdentifier(arguments.ArtifactRef, 512) {
		return contextToolCall{}, ErrDeepSeekAgentProtocol
	}
	proposal := contextcapsule.RetrievalProposal{
		ItemID: arguments.ItemID, ContentDigest: arguments.ContentDigest,
		ArtifactRef: arguments.ArtifactRef,
	}
	return contextToolCall{ID: wire.ID, Proposal: proposal}, nil
}

func contextToolCallWire(call contextToolCall) openAICompatibleToolCallWire {
	wire := openAICompatibleToolCallWire{ID: call.ID, Type: "function"}
	wire.Function.Name = "loom_read_context"
	arguments, _ := json.Marshal(struct {
		ItemID        string `json:"item_id"`
		ContentDigest string `json:"content_digest"`
		ArtifactRef   string `json:"artifact_ref"`
	}{call.Proposal.ItemID, call.Proposal.ContentDigest, call.Proposal.ArtifactRef})
	wire.Function.Arguments = string(arguments)
	return wire
}

func marshalContextToolResult(
	proposal contextcapsule.RetrievalProposal,
	item contextcapsule.RetrievedItem,
) ([]byte, error) {
	if item.ItemID != proposal.ItemID || item.ContentDigest != proposal.ContentDigest ||
		item.ArtifactRef != proposal.ArtifactRef || item.Kind == contextcapsule.KindCredentialReference ||
		item.Scope == contextcapsule.ScopeSecretReferenceOnly ||
		item.SourceType == contextcapsule.SourceCredentialReference ||
		len(item.Content) == 0 || len(item.Content) > contextToolMaxResultBytes ||
		!validContextToolText(item.Content) || digestContextToolContent(item.Content) != item.ContentDigest ||
		!validContextToolClassification(item) {
		return nil, ErrDeepSeekAgentProtocol
	}
	payload, err := json.Marshal(openAICompatibleContextToolResult{
		SchemaVersion: 1, ItemID: item.ItemID, ContentDigest: item.ContentDigest,
		Trust: item.Trust, Scope: item.Scope, SourceType: item.SourceType,
		SourceRef: item.SourceRef, ArtifactRef: item.ArtifactRef,
		Content: string(item.Content),
	})
	if err != nil || len(payload) > contextToolMaxResultBytes {
		return nil, ErrDeepSeekAgentProtocol
	}
	return payload, nil
}

func digestContextToolContent(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func validContextToolClassification(item contextcapsule.RetrievedItem) bool {
	if !validContextToolIdentifier(item.SourceRef, 512) ||
		(item.Scope == contextcapsule.ScopeArtifactScoped) != (item.ArtifactRef != "") {
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

func combineAgentAccounting(
	first *work.RunAccounting,
	second *work.RunAccounting,
) (*work.RunAccounting, error) {
	if first == nil && second == nil {
		return nil, nil
	}
	if first == nil {
		copy := *second
		return &copy, nil
	}
	if second == nil {
		copy := *first
		return &copy, nil
	}
	if first.CostObserved || second.CostObserved {
		return nil, ErrDeepSeekAgentProtocol
	}
	combined := work.RunAccounting{
		UsageObserved:    first.UsageObserved || second.UsageObserved,
		InputTokens:      first.InputTokens + second.InputTokens,
		OutputTokens:     first.OutputTokens + second.OutputTokens,
		CacheReadTokens:  first.CacheReadTokens + second.CacheReadTokens,
		CacheWriteTokens: first.CacheWriteTokens + second.CacheWriteTokens,
	}
	combined.TotalTokens = combined.InputTokens + combined.OutputTokens
	if work.ValidateRunAccounting(combined) != nil ||
		combined.InputTokens < first.InputTokens || combined.OutputTokens < first.OutputTokens ||
		combined.CacheReadTokens < first.CacheReadTokens ||
		combined.CacheWriteTokens < first.CacheWriteTokens {
		return nil, ErrDeepSeekAgentProtocol
	}
	return &combined, nil
}

func validContextToolIdentifier(value string, maximum int) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= maximum &&
		utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n")
}

func validContextToolDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func validContextToolText(content []byte) bool {
	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return false
	}
	for _, character := range string(content) {
		if character != '\n' && character != '\t' && unicode.IsControl(character) {
			return false
		}
	}
	upper := strings.ToUpper(string(content))
	for _, marker := range []string{
		"API_KEY=", "APIKEY=", "TOKEN=", "PASSWORD=", "SECRET=",
		"AUTHORIZATION: BEARER ", "BEGIN PRIVATE KEY",
	} {
		if strings.Contains(upper, marker) {
			return false
		}
	}
	return true
}

func rejectNativeDuplicateJSONKeys(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := walkNativeJSONValue(decoder); err != nil {
		return err
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) || token != nil {
		return ErrDeepSeekAgentProtocol
	}
	return nil
}

func walkNativeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok || seen[key] {
				return ErrDeepSeekAgentProtocol
			}
			seen[key] = true
			if err := walkNativeJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return ErrDeepSeekAgentProtocol
		}
	case '[':
		for decoder.More() {
			if err := walkNativeJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return ErrDeepSeekAgentProtocol
		}
	default:
		return ErrDeepSeekAgentProtocol
	}
	return nil
}

func (adapter *deepSeekAgentAdapter) publish(
	ctx context.Context,
	request supervisor.AdapterRequest,
	content string,
	status string,
	reason string,
	accounting *work.RunAccounting,
) (supervisor.AdapterResult, error) {
	records := []struct {
		kind    bridgev1.MessageType
		payload any
	}{
		{kind: bridgev1.MessageAck, payload: struct {
			MessageID string `json:"message_id"`
		}{MessageID: request.Dispatch.MessageID()}},
	}
	if status == "succeeded" {
		records = append(records, struct {
			kind    bridgev1.MessageType
			payload any
		}{kind: bridgev1.MessageEvent, payload: struct {
			Delta string `json:"delta"`
		}{Delta: content}})
	}
	records = append(records, struct {
		kind    bridgev1.MessageType
		payload any
	}{kind: bridgev1.MessageResult, payload: struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}{Status: status, Reason: reason}})

	frames := make([]bridgev1.Frame, 0, len(records))
	for index, record := range records {
		payload, err := json.Marshal(record.payload)
		if err != nil {
			return supervisor.AdapterResult{}, ErrDeepSeekAgentProtocol
		}
		now := adapter.now()
		if now.IsZero() || now.Location() != time.UTC {
			return supervisor.AdapterResult{}, ErrInvalidDeepSeekAgentAdapter
		}
		frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
			MessageID: deterministicUUID(
				request.Dispatch.MessageID(),
				string(record.kind),
				fmt.Sprintf("%d", index+2),
			),
			CorrelationID:         request.Dispatch.CorrelationID(),
			WorkItemID:            request.Binding.WorkItemID,
			RunID:                 request.Binding.RunID,
			ClaimGeneration:       request.Binding.ClaimGeneration,
			RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
			SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
			Sequence:              int64(index + 2),
			Type:                  record.kind,
			EmittedAt:             now,
			Payload:               payload,
		})
		if err != nil {
			return supervisor.AdapterResult{}, errors.Join(ErrDeepSeekAgentProtocol, supervisor.ErrBridgeSession)
		}
		if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
			return supervisor.AdapterResult{}, errors.Join(ErrDeepSeekAgentProtocol, supervisor.ErrBridgeSession, err)
		}
		frames = append(frames, frame)
	}
	result, err := supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames:        frames,
		ExitCode:             0,
		DispatchAcknowledged: true,
		ResultAcknowledged:   true,
		Accounting:           accounting,
	})
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrDeepSeekAgentProtocol, err)
	}
	return result, nil
}

func decodeDeepSeekAgentDispatch(payload []byte) (deepSeekAgentDispatch, error) {
	if contextDispatch, err := contextcapsule.DecodeDispatchPayload(payload); err == nil {
		if len(contextDispatch.Prompt) > deepSeekAgentMaxPromptBytes {
			return deepSeekAgentDispatch{}, errors.Join(
				ErrDeepSeekAgentProtocol,
				supervisor.ErrBridgeSession,
			)
		}
		return deepSeekAgentDispatch{
			SchemaVersion: 2,
			Kind:          "loom_context_capsule_prompt",
			Prompt:        contextDispatch.Prompt,
		}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var dispatch deepSeekAgentDispatch
	if decoder.Decode(&dispatch) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		dispatch.SchemaVersion != 1 ||
		(dispatch.Kind != "pi_rpc_prompt" && dispatch.Kind != piVerifierPromptKind) ||
		dispatch.Prompt == "" || len(dispatch.Prompt) > deepSeekAgentMaxPromptBytes ||
		!utf8.ValidString(dispatch.Prompt) || strings.IndexByte(dispatch.Prompt, 0) >= 0 {
		return deepSeekAgentDispatch{}, errors.Join(ErrDeepSeekAgentProtocol, supervisor.ErrBridgeSession)
	}
	canonical, err := json.Marshal(dispatch)
	if err != nil || !bytes.Equal(canonical, payload) {
		return deepSeekAgentDispatch{}, errors.Join(ErrDeepSeekAgentProtocol, supervisor.ErrBridgeSession)
	}
	dispatch.verifier = dispatch.Kind == piVerifierPromptKind
	return dispatch, nil
}

// verifierTerminalFromVerdict maps the independent verifier model response to
// the terminal status/reason consumed by the acceptance authority. An
// unparseable or missing verdict fails closed as insufficient evidence so the
// acceptance cannot silently pass without a verifier judgment.
func verifierTerminalFromVerdict(content string) (string, string) {
	verdict, ok := parseVerifierVerdict(content)
	switch {
	case ok && verdict == "satisfied":
		return "succeeded", ""
	case ok && verdict == "not_satisfied":
		return "failed", string(verification.VerifierReasonCriteriaNotSatisfied)
	default:
		return "failed", string(verification.VerifierReasonInsufficientEvidence)
	}
}

// parseVerifierVerdict extracts the verifier decision from the model
// response. The verifier prompt asks for exactly one allowed reason code, so
// a bare reason code is accepted directly; a JSON object with a "verdict"
// field (with optional surrounding prose) is accepted too. A missing or
// ambiguous verdict is never accepted.
func parseVerifierVerdict(content string) (string, bool) {
	if content == "" {
		return "", false
	}
	trimmed := strings.TrimSpace(content)
	switch trimmed {
	case "satisfied", "criteria_satisfied",
		"not_satisfied", "criteria_not_satisfied",
		"insufficient_evidence":
		return normalizeVerifierVerdict(trimmed), true
	}
	index := strings.Index(content, `"verdict"`)
	if index < 0 {
		return "", false
	}
	remainder := content[index+len(`"verdict"`):]
	colon := strings.Index(remainder, ":")
	if colon < 0 {
		return "", false
	}
	value := strings.TrimSpace(remainder[colon+1:])
	if len(value) < 2 || value[0] != '"' {
		return "", false
	}
	end := strings.Index(value[1:], `"`)
	if end < 0 {
		return "", false
	}
	verdict := value[1 : 1+end]
	normalized := normalizeVerifierVerdict(verdict)
	if normalized == "" {
		return "", false
	}
	return normalized, true
}

func normalizeVerifierVerdict(value string) string {
	switch value {
	case "satisfied", "criteria_satisfied":
		return "satisfied"
	case "not_satisfied", "criteria_not_satisfied":
		return "not_satisfied"
	case "insufficient_evidence":
		return "insufficient_evidence"
	default:
		return ""
	}
}

func deepSeekAgentHTTPFailureReason(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "provider_auth"
	case http.StatusTooManyRequests:
		return "provider_rate_limit"
	default:
		if status >= http.StatusInternalServerError {
			return "provider_unavailable"
		}
		return "provider_rejected"
	}
}

func deepSeekAgentDiagnosticFailure(reason string, err error) (string, bool) {
	switch reason {
	case "credential_unavailable":
		stage := credentials.CredentialFailureStage(err)
		if stage == "" {
			stage = credentials.CredentialStageLeaseIssue
		}
		return stage, credentials.CredentialFailureRetryable(stage)
	case "provider_auth":
		return "provider_auth", false
	case "provider_rate_limit":
		return "provider_rate_limit", true
	case "provider_unavailable":
		return "provider_http", true
	case "timeout":
		return "provider_connect", true
	case "context_delivery_unavailable":
		return "context_delivery", true
	default:
		return "provider_http", false
	}
}

func transportTimeout(err error) bool {
	var timeout interface{ Timeout() bool }
	return errors.Is(err, context.DeadlineExceeded) ||
		errors.As(err, &timeout) && timeout.Timeout()
}

func exactOpenAICompatibleAgentURL(candidate *url.URL, endpoint string) bool {
	reference, err := url.Parse(endpoint)
	return err == nil && candidate != nil && candidate.Scheme == reference.Scheme &&
		candidate.Host == reference.Host && candidate.Path == reference.Path &&
		candidate.RawQuery == "" && candidate.Fragment == ""
}

func deepSeekAgentProvider() openAICompatibleAgentProvider {
	return openAICompatibleAgentProvider{
		providerID: DeepSeekAgentProviderID, modelID: DeepSeekAgentModelID,
		endpoint:             DeepSeekAgentEndpoint,
		endpointFingerprint:  DeepSeekAgentEndpointFingerprint,
		completionTokenField: "max_tokens",
	}
}

func kimiAgentProvider() openAICompatibleAgentProvider {
	return openAICompatibleAgentProvider{
		providerID: KimiAgentProviderID, modelID: KimiAgentModelID,
		endpoint:             KimiAgentEndpoint,
		endpointFingerprint:  KimiAgentEndpointFingerprint,
		completionTokenField: "max_tokens",
	}
}

func miniMaxAgentProvider() openAICompatibleAgentProvider {
	return openAICompatibleAgentProvider{
		providerID: MiniMaxAgentProviderID, modelID: MiniMaxAgentModelID,
		endpoint:             MiniMaxAgentEndpoint,
		endpointFingerprint:  MiniMaxAgentEndpointFingerprint,
		completionTokenField: "max_completion_tokens",
	}
}

func validOpenAICompatibleAgentProvider(provider openAICompatibleAgentProvider) bool {
	return validAdapterID(provider.providerID) && provider.modelID != "" &&
		len(provider.modelID) <= 128 &&
		provider.endpointFingerprint != "" &&
		len(provider.endpointFingerprint) == sha256.Size*2 &&
		(provider.completionTokenField == "max_tokens" ||
			provider.completionTokenField == "max_completion_tokens") &&
		exactOpenAICompatibleAgentURLMustParse(provider.endpoint)
}

func exactOpenAICompatibleAgentURLMustParse(endpoint string) bool {
	parsed, err := url.Parse(endpoint)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" &&
		parsed.RawQuery == "" && parsed.Fragment == "" &&
		parsed.String() == endpoint
}

func deterministicUUID(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	digest[6] = digest[6]&0x0f | 0x40
	digest[8] = digest[8]&0x3f | 0x80
	encoded := hex.EncodeToString(digest[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" +
		encoded[16:20] + "-" + encoded[20:32]
}

func validAdapterID(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
