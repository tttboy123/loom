package piadapter

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/permissions"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

var (
	ErrInvalidPiRPCBridgeAdapter = errors.New("invalid Pi RPC bridge adapter")
	ErrPiRPCProtocol             = errors.New("Pi RPC protocol failed")
	ErrPiRPCOutputTooLarge       = errors.New("Pi RPC output too large")
	ErrPiRPCCleanup              = errors.New("Pi RPC cleanup failed")
)

const (
	piRPCProviderID           = "loom-local"
	piRPCModelID              = "qwen2.5-coder-1.5b-instruct-q4-k-m"
	piRPCMaxPromptBytes       = 8192
	piRPCMaxLineBytes         = 1 << 20
	piRPCMaxStdoutBytes       = 8 << 20
	piRPCMaxStderrBytes       = 256 << 10
	piRPCMaxRecords           = 1024
	piRPCMaxBridgeFrames      = 1024
	piRPCMaxDeltaBytes        = 2048
	piRPCMaxSystemPromptBytes = 4096
	piRPCSettingsJSON         = `{"compaction":{"enabled":false},"retry":{"enabled":false,"maxRetries":0,"baseDelayMs":0,"provider":{"maxRetries":0,"maxRetryDelayMs":0}}}` + "\n"
)

// piRPCSystemPrompt 由 ToolCallSystemPrompt() 提供：W-BRIDGE 启用工具面后，
// 模型只能通过单一信封提议工具调用（契约 §3.1/§3.3）。
var piRPCSystemPrompt = ToolCallSystemPrompt()

var piRPCContextReadSystemPrompt = ContextReadSystemPrompt()

var piRPCContextAndToolSystemPrompt = ContextAndToolSystemPrompt()

type piRPCLocalRoute string

const (
	piRPCLocalRouteContext piRPCLocalRoute = "context"
	piRPCLocalRouteTool    piRPCLocalRoute = "tool"
)

type PiRPCBridgeAdapterConfig struct {
	Execution         PiExecutionAdapterConfig
	ProviderID        string
	ModelID           string
	BaseURL           string
	MaxAssistantBytes int
	TranscriptAudit   chan<- PiRPCTranscriptAudit
	ToolHook          ToolCallHook
}

type PiRPCTranscriptAudit struct {
	ForwardPartialObserved   bool
	TextStartSnapshotBytes   int
	AcceptedBytesAtTextStart int
	AcceptedDeltaCount       int
	FinalSnapshotBytes       int
	FinalAcceptedDeltaBytes  int
	DeltaClosure             bool
	ToolCallResults          []ToolCallAuditEntry
}

// ToolCallAuditEntry 是 W-BRIDGE 的 audit 记录：只有已批准执行的结果才
// 携带 ExecutionID/ResultNote；ask/deny 只记录 verdict 与 reason 摘要
// （契约 §3.3：不写完整 tool call payload/结果内容）。
type ToolCallAuditEntry struct {
	Verdict      permissions.Verdict `json:"verdict"`
	Tool         string              `json:"tool"`
	ExecutionID  string              `json:"execution_id,omitempty"`
	ResultNote   string              `json:"result_note,omitempty"`
	DenialReason string              `json:"denial_reason,omitempty"`
}

type piRPCBridgeAdapter struct {
	execution         *piExecutionAdapter
	providerID        string
	modelID           string
	baseURL           string
	maxAssistantBytes int
	transcriptAudit   chan<- PiRPCTranscriptAudit
	toolHook          ToolCallHook
}

type piRPCLineResult struct {
	line []byte
	err  error
}

type piRPCState struct {
	conversation         bool
	contextMode          bool
	toolMode             bool
	contextStage         piRPCContextStage
	contextProposal      contextcapsule.RetrievalProposal
	contextToolCallID    string
	contextAssistant     json.RawMessage
	contextResult        piRPCContextResultReceipt
	contextRecordType    string
	contextRecordRole    string
	contextRecordKeys    string
	contextRecordEvent   string
	contextRejectPoint   string
	contextInitialRID    string
	toolCallID           string
	toolEnvelope         ToolCallEnvelope
	toolWireResult       ToolCallResult
	toolAssistant        json.RawMessage
	toolExtension        *piToolExtension
	localRoute           piRPCLocalRoute
	messageID            string
	forbidden            []byte
	responseSeen         bool
	dispatchAcknowledged bool
	agentStarted         bool
	turnOpen             bool
	turnCount            int
	messageOpen          bool
	messageRole          string
	userSeen             bool
	assistantSeen        bool
	assistantIdentity    piRPCAssistantIdentityState
	finalAssistant       json.RawMessage
	textStarted          bool
	textEnded            bool
	toolStarted          bool
	toolEnded            bool
	toolCallBytes        []byte
	toolResultSent       bool
	toolResults          []ToolCallAuditEntry
	toolCallIDs          []string
	toolEnvelopes        []ToolCallEnvelope
	toolWireResults      []ToolCallResult
	toolAssistants       []json.RawMessage
	toolResultMessages   []json.RawMessage
	doneSeen             bool
	agentEnded           bool
	settled              bool
	assistant            []byte
	lastPartial          []byte
	forwardPartial       bool
	textStartBytes       int
	acceptedAtStart      int
	deltaCount           int
	frames               []bridgev1.Frame
	nextSequence         int64
	recordCount          int
	prompt               []byte
}

type piRPCAssistantIdentityState struct {
	timestamp     string
	responseID    string
	responseBound bool
	reasoningSeen bool
}

type piRPCAssistantIdentitySnapshot struct {
	timestamp       string
	responseID      string
	responsePresent bool
	responseModel   bool
	cacheWrite1h    bool
	reasoning       bool
}

type piRPCDiagnosticPhase string

const (
	piRPCPhaseAssistantUpdate     piRPCDiagnosticPhase = "assistant_update"
	piRPCPhaseAssistantMessageEnd piRPCDiagnosticPhase = "assistant_message_end"
	piRPCPhaseTurnEnd             piRPCDiagnosticPhase = "turn_end"
	piRPCPhaseAgentEnd            piRPCDiagnosticPhase = "agent_end"
	piRPCPhaseAgentSettled        piRPCDiagnosticPhase = "agent_settled"
)

type piRPCDiagnosticEvent string

const (
	piRPCEventTextStart     piRPCDiagnosticEvent = "text_start"
	piRPCEventTextDelta     piRPCDiagnosticEvent = "text_delta"
	piRPCEventTextEnd       piRPCDiagnosticEvent = "text_end"
	piRPCEventThinkingStart piRPCDiagnosticEvent = "thinking_start"
	piRPCEventThinkingDelta piRPCDiagnosticEvent = "thinking_delta"
	piRPCEventThinkingEnd   piRPCDiagnosticEvent = "thinking_end"
	piRPCEventToolCallStart piRPCDiagnosticEvent = "toolcall_start"
	piRPCEventToolCallDelta piRPCDiagnosticEvent = "toolcall_delta"
	piRPCEventToolCallEnd   piRPCDiagnosticEvent = "toolcall_end"
	piRPCEventMessageEnd    piRPCDiagnosticEvent = "message_end"
	piRPCEventTurnEnd       piRPCDiagnosticEvent = "turn_end"
	piRPCEventAgentEnd      piRPCDiagnosticEvent = "agent_end"
	piRPCEventAgentSettled  piRPCDiagnosticEvent = "agent_settled"
	piRPCEventUnknown       piRPCDiagnosticEvent = "unknown"
)

type piRPCDiagnosticReason string

const (
	piRPCReasonEventShape             piRPCDiagnosticReason = "event_shape"
	piRPCReasonEventKindUnsupported   piRPCDiagnosticReason = "event_kind_unsupported"
	piRPCReasonContentIndex           piRPCDiagnosticReason = "content_index"
	piRPCReasonMessagePartialMismatch piRPCDiagnosticReason = "message_partial_mismatch"
	piRPCReasonAssistantMessageSchema piRPCDiagnosticReason = "assistant_message_schema"
	piRPCReasonResponseIDTransition   piRPCDiagnosticReason = "response_id_transition"
	piRPCReasonResponseModelPresent   piRPCDiagnosticReason = "response_model_present"
	piRPCReasonTimestampIdentity      piRPCDiagnosticReason = "timestamp_identity"
	piRPCReasonUsageSchema            piRPCDiagnosticReason = "usage_schema"
	piRPCReasonUsageProgression       piRPCDiagnosticReason = "usage_progression"
	piRPCReasonTextContentProgression piRPCDiagnosticReason = "text_content_progression"
	piRPCReasonToolCallProgression    piRPCDiagnosticReason = "toolcall_content_progression"
	piRPCReasonToolCallInvalid        piRPCDiagnosticReason = "toolcall_invalid"
	piRPCReasonToolCallHookFailed     piRPCDiagnosticReason = "toolcall_hook_failed"
	piRPCReasonDeltaPolicy            piRPCDiagnosticReason = "delta_policy"
	piRPCReasonTerminalStopReason     piRPCDiagnosticReason = "terminal_stop_reason"
	piRPCReasonTerminalIdentity       piRPCDiagnosticReason = "terminal_identity"
	piRPCReasonFrameSink              piRPCDiagnosticReason = "frame_sink"
)

type piRPCRejection struct {
	phase  piRPCDiagnosticPhase
	event  piRPCDiagnosticEvent
	reason piRPCDiagnosticReason
}

func (rejection piRPCRejection) Error() string {
	return fmt.Sprintf(
		"%s: phase=%s event=%s reason=%s",
		ErrPiRPCProtocol,
		rejection.phase,
		rejection.event,
		rejection.reason,
	)
}

func (piRPCRejection) Unwrap() error {
	return ErrPiRPCProtocol
}

func rejectPiRPC(
	phase piRPCDiagnosticPhase,
	event piRPCDiagnosticEvent,
	reason piRPCDiagnosticReason,
) error {
	return piRPCRejection{phase: phase, event: event, reason: reason}
}

func NewPiRPCBridgeAdapter(
	config PiRPCBridgeAdapterConfig,
) (supervisor.RuntimeAdapter, error) {
	if len(config.Execution.Arguments) != 0 ||
		config.ProviderID != piRPCProviderID ||
		config.ModelID != piRPCModelID ||
		config.MaxAssistantBytes < 1 ||
		config.MaxAssistantBytes > 65536 ||
		!validPiRPCBaseURL(config.BaseURL) ||
		len(piRPCSystemPrompt) > piRPCMaxSystemPromptBytes ||
		len(piRPCContextReadSystemPrompt) > piRPCMaxSystemPromptBytes ||
		len(piRPCContextAndToolSystemPrompt) > piRPCMaxSystemPromptBytes {
		return nil, ErrInvalidPiRPCBridgeAdapter
	}
	baseAdapter, err := NewPiExecutionAdapter(config.Execution)
	if err != nil {
		return nil, errors.Join(ErrInvalidPiRPCBridgeAdapter, err)
	}
	execution, ok := baseAdapter.(*piExecutionAdapter)
	if !ok {
		return nil, ErrInvalidPiRPCBridgeAdapter
	}
	return &piRPCBridgeAdapter{
		execution:         execution,
		providerID:        config.ProviderID,
		modelID:           config.ModelID,
		baseURL:           config.BaseURL,
		maxAssistantBytes: config.MaxAssistantBytes,
		transcriptAudit:   config.TranscriptAudit,
		toolHook:          config.ToolHook,
	}, nil
}

func (*piRPCBridgeAdapter) AdapterType() string {
	return "pi-cli"
}

func (*piRPCBridgeAdapter) AcceptsAgentInputs() bool { return true }

func (adapter *piRPCBridgeAdapter) RuntimeInstanceID() string {
	if adapter == nil || adapter.execution == nil {
		return ""
	}
	return adapter.execution.instanceID
}

func (adapter *piRPCBridgeAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	if adapter == nil || adapter.execution == nil || ctx == nil {
		return supervisor.AdapterResult{}, ErrInvalidPiRPCBridgeAdapter
	}
	if err := adapter.execution.validateRequest(request, adapter.AdapterType()); err != nil ||
		request.ExecutionBinding.ProviderID != adapter.providerID ||
		request.ExecutionBinding.ModelID != adapter.modelID {
		return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, err)
	}
	prompt, err := parsePiRPCDispatch(request.Dispatch.Payload(), request.Grant.Value())
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, err)
	}
	if err := adapter.execution.revalidateBindings(); err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, err)
	}
	agentPath, sessionPath, err := adapter.preparePrivatePiHome(request.HomePath)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}

	contextCapability := piRPCContainsCapability(
		request.ExecutionBinding.Capabilities,
		loomruntime.CapabilityContextRetrieval,
	)
	toolCapability := piRPCContainsCapability(
		request.ExecutionBinding.Capabilities,
		loomruntime.CapabilityGovernedToolLoop,
	)
	toolHookPresent := adapter.toolHook != nil && !nilPiInterface(adapter.toolHook)
	if toolCapability != toolHookPresent {
		return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, ErrPiToolExtension)
	}
	retrieverPresent := piRPCContextRetrieverPresent(request.ContextRetriever)
	deliveryPresent := piRPCContextDeliveryPresent(request.ContextDelivery)
	if contextCapability != retrieverPresent || contextCapability != deliveryPresent ||
		request.ContextRetriever != nil && !retrieverPresent ||
		request.ContextDelivery != nil && !deliveryPresent {
		return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, ErrPiContextExtension)
	}
	var contextExtension *piContextExtension
	if contextCapability {
		authority, authorityErr := contextcapsule.ValidateAuthorityRecord(request.ContextCapsule)
		if authorityErr != nil ||
			authority.AgentID != request.Binding.SenderAgentInstanceID ||
			authority.ProviderID != request.ExecutionBinding.ProviderID ||
			authority.ProviderAccountID != request.ExecutionBinding.ProviderAccountID ||
			authority.ModelID != request.ExecutionBinding.ModelID ||
			authority.AuthMode != string(request.ExecutionBinding.AuthMode) {
			return supervisor.AdapterResult{}, errors.Join(
				ErrPiRPCProtocol, ErrPiContextExtension, authorityErr,
			)
		}
		extensionID, idErr := adapter.execution.randomUUID()
		if idErr != nil {
			return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, idErr)
		}
		contextExtension, err = newPiContextDeliveryExtensionWithContext(
			ctx,
			request.HomePath,
			request.TempPath,
			extensionID,
			request.ContextDelivery,
		)
		if err != nil {
			return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, err)
		}
		defer contextExtension.Close()
	}
	var toolExtension *piToolExtension
	if toolCapability {
		extensionID, idErr := adapter.execution.randomUUID()
		if idErr != nil {
			return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, idErr)
		}
		toolExtension, err = newPiToolExtension(
			ctx,
			request.HomePath,
			request.TempPath,
			extensionID,
			adapter.toolHook,
			piRPCToolBinding(request),
		)
		if err != nil {
			return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, err)
		}
		defer toolExtension.Close()
	}
	arguments, err := adapter.arguments(request, contextExtension, toolExtension)
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, err)
	}
	command := exec.Command(adapter.execution.executable.path, arguments...)
	command.Dir = request.WorkspacePath
	command.Env = []string{
		"HOME=" + request.HomePath,
		"TMPDIR=" + request.TempPath,
		"PATH=" + adapter.execution.searchPathValue(),
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"PI_CODING_AGENT_DIR=" + agentPath,
		"PI_CODING_AGENT_SESSION_DIR=" + sessionPath,
		"PI_OFFLINE=1",
		"PI_SKIP_VERSION_CHECK=1",
		"PI_TELEMETRY=0",
		"NO_COLOR=1",
		"TERM=dumb",
		"NO_PROXY=127.0.0.1",
		"no_proxy=127.0.0.1",
	}
	if err := configureExecutionProcess(command); err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiRPCCleanup, err)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		return supervisor.AdapterResult{}, ErrPiRPCProtocol
	}
	stdout, childStdout, err := os.Pipe()
	if err != nil {
		_ = stdin.Close()
		return supervisor.AdapterResult{}, ErrPiRPCProtocol
	}
	stderr, childStderr, err := os.Pipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = childStdout.Close()
		return supervisor.AdapterResult{}, ErrPiRPCProtocol
	}
	command.Stdout = childStdout
	command.Stderr = childStderr
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		_ = childStdout.Close()
		_ = childStderr.Close()
		return supervisor.AdapterResult{}, ErrPiRPCProtocol
	}
	defer stdout.Close()
	defer stderr.Close()
	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	if contextExtension != nil {
		if err := contextExtension.BindProcess(command.Process.Pid); err != nil {
			return supervisor.AdapterResult{}, adapter.failRPC(
				command, wait, stdin, errors.Join(ErrPiRPCProtocol, err),
			)
		}
	}
	if toolExtension != nil {
		if err := toolExtension.BindProcess(command.Process.Pid); err != nil {
			return supervisor.AdapterResult{}, adapter.failRPC(
				command, wait, stdin, errors.Join(ErrPiRPCProtocol, err),
			)
		}
	}
	if closeErr := errors.Join(childStdout.Close(), childStderr.Close()); closeErr != nil {
		return supervisor.AdapterResult{}, adapter.failRPC(
			command,
			wait,
			stdin,
			errors.Join(ErrPiRPCCleanup, closeErr),
		)
	}
	lineContext, cancelLines := context.WithCancel(context.Background())
	defer cancelLines()
	lines := make(chan piRPCLineResult, 1)
	go scanPiRPCLines(lineContext, stdout, lines)
	stderrResult := make(chan piStderrResult, 1)
	go readPiRPCStderr(stderr, stderrResult)

	if err := writePiRPCPrompt(stdin, request.Dispatch.MessageID(), []byte(prompt)); err != nil {
		return supervisor.AdapterResult{}, adapter.failRPC(command, wait, stdin, ErrPiRPCProtocol)
	}

	state := piRPCState{
		messageID:     request.Dispatch.MessageID(),
		forbidden:     []byte(request.Grant.Value()),
		nextSequence:  2,
		prompt:        []byte(prompt),
		contextMode:   contextCapability,
		toolMode:      toolCapability,
		toolExtension: toolExtension,
	}
	defer clearPiRPCState(&state)
	accountingMessages := make([]json.RawMessage, 0, 4)
	defer func() { clearPiRPCRawMessages(accountingMessages) }()
	audit := PiRPCTranscriptAudit{DeltaClosure: true}
	for {
		for !state.settled {
			select {
			case <-ctx.Done():
				return supervisor.AdapterResult{}, adapter.cancelRPC(
					command,
					wait,
					stdin,
					lines,
					ctx.Err(),
				)
			case lineResult := <-lines:
				if lineResult.err != nil {
					primary := ErrPiRPCProtocol
					if errors.Is(lineResult.err, ErrPiRPCOutputTooLarge) {
						primary = ErrPiRPCOutputTooLarge
					}
					return supervisor.AdapterResult{}, adapter.failRPC(command, wait, stdin, primary)
				}
				line := lineResult.line
				if bytes.Contains(line, []byte(request.Grant.Value())) {
					zeroPiRPCBytes(line)
					return supervisor.AdapterResult{}, adapter.failRPC(command, wait, stdin, ErrPiRPCProtocol)
				}
				acceptErr := adapter.acceptRPCLine(ctx, &request, &state, line)
				zeroPiRPCBytes(line)
				if acceptErr != nil {
					return supervisor.AdapterResult{}, adapter.failRPC(
						command,
						wait,
						stdin,
						errors.Join(acceptErr, errors.New(piRPCDebugState(state))),
					)
				}
			}
		}
		accountingMessages = append(accountingMessages, piRPCRoundAccountingMessages(&state)...)
		mergePiRPCRoundAudit(&audit, &state)
		if request.AgentInputs == nil {
			break
		}
		outputDigest := sha256.Sum256(state.assistant)
		batch, available, inputErr := request.AgentInputs.NextAgentInput(
			ctx,
			loomruntime.AgentInputCheckpoint{OutputDigest: hex.EncodeToString(outputDigest[:])},
		)
		if inputErr != nil {
			batch.Close()
			return supervisor.AdapterResult{}, adapter.failRPC(command, wait, stdin, ErrPiRPCProtocol)
		}
		if !available {
			batch.Close()
			break
		}
		nextMessageID := batch.StepID
		nextPrompt, renderErr := loomruntime.RenderAgentInput(&batch)
		batch.Close()
		if renderErr != nil {
			zeroPiRPCBytes(nextPrompt)
			return supervisor.AdapterResult{}, adapter.failRPC(command, wait, stdin, ErrPiRPCProtocol)
		}
		if resetErr := resetPiRPCRound(&state, nextMessageID, nextPrompt); resetErr != nil {
			zeroPiRPCBytes(nextPrompt)
			return supervisor.AdapterResult{}, adapter.failRPC(command, wait, stdin, ErrPiRPCProtocol)
		}
		if writeErr := writePiRPCPrompt(stdin, nextMessageID, nextPrompt); writeErr != nil {
			zeroPiRPCBytes(nextPrompt)
			return supervisor.AdapterResult{}, adapter.failRPC(command, wait, stdin, ErrPiRPCProtocol)
		}
		zeroPiRPCBytes(nextPrompt)
	}
	if err := stdin.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return supervisor.AdapterResult{}, adapter.failRPC(command, wait, nil, ErrPiRPCCleanup)
	}
	for {
		select {
		case <-ctx.Done():
			return supervisor.AdapterResult{}, adapter.failRPC(
				command,
				wait,
				nil,
				errors.Join(ErrPiRPCProtocol, ctx.Err()),
			)
		case lineResult := <-lines:
			if errors.Is(lineResult.err, io.EOF) {
				goto stdoutDrained
			}
			zeroPiRPCBytes(lineResult.line)
			if lineResult.err == nil {
				return supervisor.AdapterResult{}, adapter.failRPC(
					command, wait, nil, ErrPiRPCProtocol,
				)
			}
			primary := ErrPiRPCProtocol
			if errors.Is(lineResult.err, ErrPiRPCOutputTooLarge) {
				primary = ErrPiRPCOutputTooLarge
			}
			return supervisor.AdapterResult{}, adapter.failRPC(command, wait, nil, primary)
		}
	}

stdoutDrained:
	waitErr, cleanupErr := adapter.waitForRPCExit(command, wait)
	if cleanupErr != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiRPCCleanup, cleanupErr)
	}
	capturedStderr := <-stderrResult
	if capturedStderr.err != nil {
		primary := ErrPiRPCProtocol
		if errors.Is(capturedStderr.err, ErrPiRPCOutputTooLarge) {
			primary = ErrPiRPCOutputTooLarge
		}
		return supervisor.AdapterResult{}, errors.Join(primary, capturedStderr.err)
	}
	if bytes.Contains(capturedStderr.content, []byte(request.Grant.Value())) ||
		waitErr != nil ||
		command.ProcessState == nil ||
		command.ProcessState.ExitCode() != 0 {
		return supervisor.AdapterResult{}, errors.Join(
			ErrPiRPCProtocol,
			fmt.Errorf(
				"process exit invalid: wait_error=%t state_present=%t exit=%d",
				waitErr != nil,
				command.ProcessState != nil,
				piRPCExitCode(command),
			),
		)
	}
	accounting, err := piRPCMessagesAccounting(accountingMessages...)
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, err)
	}
	if contextExtension != nil {
		if err := contextExtension.Acknowledge(
			ctx, attemptpayload.ProofHarnessFinalOutput,
		); err != nil {
			return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, err)
		}
	}
	if toolExtension != nil {
		if err := toolExtension.Acknowledge(
			ctx, attemptpayload.ProofHarnessFinalOutput,
		); err != nil {
			return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, err)
		}
	}
	if err := adapter.finishRPCFrames(ctx, request, &state); err != nil {
		return supervisor.AdapterResult{}, err
	}
	result, err := supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames:        state.frames,
		Stderr:               capturedStderr.content,
		ExitCode:             0,
		DispatchAcknowledged: true,
		ResultAcknowledged:   true,
		Accounting:           &accounting,
	})
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, err)
	}
	publishPiRPCTranscriptAudit(adapter.transcriptAudit, audit)
	return result, nil
}

func writePiRPCPrompt(writer io.Writer, messageID string, prompt []byte) error {
	if writer == nil || messageID == "" || len(messageID) > 512 ||
		!utf8.ValidString(messageID) || strings.IndexByte(messageID, 0) >= 0 ||
		len(prompt) == 0 || len(prompt) > 64<<10 || !utf8.Valid(prompt) ||
		bytes.IndexByte(prompt, 0) >= 0 {
		return ErrPiRPCProtocol
	}
	line := make([]byte, 0, len(prompt)+len(messageID)+48)
	line = append(line, `{"id":`...)
	line = appendPiRPCJSONString(line, []byte(messageID))
	line = append(line, `,"type":"prompt","message":`...)
	line = appendPiRPCJSONString(line, prompt)
	line = append(line, '}', '\n')
	defer zeroPiRPCBytes(line)
	written, err := writer.Write(line)
	if err == nil && written != len(line) {
		return io.ErrShortWrite
	}
	return err
}

func appendPiRPCJSONString(destination []byte, content []byte) []byte {
	const hexDigits = "0123456789abcdef"
	destination = append(destination, '"')
	for offset := 0; offset < len(content); {
		character, size := utf8.DecodeRune(content[offset:])
		if character == utf8.RuneError && size == 1 {
			return append(destination, '"')
		}
		switch character {
		case '"', '\\':
			destination = append(destination, '\\', byte(character))
		case '\b':
			destination = append(destination, `\b`...)
		case '\f':
			destination = append(destination, `\f`...)
		case '\n':
			destination = append(destination, `\n`...)
		case '\r':
			destination = append(destination, `\r`...)
		case '\t':
			destination = append(destination, `\t`...)
		case '\u2028', '\u2029':
			destination = append(destination, `\u202`...)
			destination = append(destination, hexDigits[character&0xf])
		default:
			if character < 0x20 {
				destination = append(destination, `\u00`...)
				destination = append(
					destination,
					hexDigits[(character>>4)&0xf],
					hexDigits[character&0xf],
				)
			} else {
				destination = append(destination, content[offset:offset+size]...)
			}
		}
		offset += size
	}
	return append(destination, '"')
}

func piRPCRoundAccountingMessages(state *piRPCState) []json.RawMessage {
	if state == nil {
		return nil
	}
	messages := make([]json.RawMessage, 0, len(state.toolAssistants)+2)
	switch {
	case state.localRoute == piRPCLocalRouteContext ||
		state.contextMode && !state.toolMode:
		messages = append(messages, bytes.Clone(state.contextAssistant))
	case state.localRoute == piRPCLocalRouteTool ||
		state.toolMode && !state.contextMode:
		for _, message := range state.toolAssistants {
			messages = append(messages, bytes.Clone(message))
		}
	}
	messages = append(messages, bytes.Clone(state.finalAssistant))
	return messages
}

func mergePiRPCRoundAudit(audit *PiRPCTranscriptAudit, state *piRPCState) {
	if audit == nil || state == nil {
		return
	}
	audit.ForwardPartialObserved = audit.ForwardPartialObserved || state.forwardPartial
	audit.TextStartSnapshotBytes += state.textStartBytes
	audit.AcceptedBytesAtTextStart += state.acceptedAtStart
	audit.AcceptedDeltaCount += state.deltaCount
	audit.FinalSnapshotBytes += len(state.lastPartial)
	audit.FinalAcceptedDeltaBytes += len(state.assistant)
	audit.DeltaClosure = audit.DeltaClosure && bytes.Equal(state.lastPartial, state.assistant)
	audit.ToolCallResults = append(audit.ToolCallResults, state.toolResults...)
}

func resetPiRPCRound(state *piRPCState, messageID string, prompt []byte) error {
	if state == nil || !state.settled || messageID == "" || len(prompt) == 0 ||
		len(prompt) > 64<<10 || !utf8.Valid(prompt) || bytes.IndexByte(prompt, 0) >= 0 {
		return ErrPiRPCProtocol
	}
	frames := state.frames
	nextSequence := state.nextSequence
	recordCount := state.recordCount
	dispatchAcknowledged := state.dispatchAcknowledged
	forbidden := state.forbidden
	state.forbidden = nil
	clearPiRPCState(state)
	*state = piRPCState{
		messageID: messageID, prompt: bytes.Clone(prompt), forbidden: forbidden,
		frames: frames, nextSequence: nextSequence, recordCount: recordCount,
		dispatchAcknowledged: dispatchAcknowledged,
	}
	return nil
}

func clearPiRPCState(state *piRPCState) {
	if state == nil {
		return
	}
	for _, content := range [][]byte{
		state.prompt, state.forbidden, state.assistant, state.lastPartial,
		state.toolCallBytes, state.contextAssistant, state.toolAssistant,
		state.finalAssistant,
	} {
		zeroPiRPCBytes(content)
	}
	clearPiRPCRawMessages(state.toolAssistants)
	clearPiRPCRawMessages(state.toolResultMessages)
	state.prompt = nil
	state.forbidden = nil
	state.assistant = nil
	state.lastPartial = nil
	state.toolCallBytes = nil
	state.contextAssistant = nil
	state.toolAssistant = nil
	state.finalAssistant = nil
	state.toolAssistants = nil
	state.toolResultMessages = nil
}

func clearPiRPCRawMessages(messages []json.RawMessage) {
	for index := range messages {
		zeroPiRPCBytes(messages[index])
		messages[index] = nil
	}
}

func publishPiRPCTranscriptAudit(
	sink chan<- PiRPCTranscriptAudit,
	audit PiRPCTranscriptAudit,
) {
	defer func() {
		_ = recover()
	}()
	select {
	case sink <- audit:
	default:
	}
}

func (adapter *piRPCBridgeAdapter) arguments(
	request supervisor.AdapterRequest,
	contextExtension *piContextExtension,
	toolExtension *piToolExtension,
) ([]string, error) {
	systemPrompt := piRPCSystemPrompt
	if toolExtension != nil {
		systemPrompt = toolCallSystemPrompt(
			toolExtension.governedTools(),
			contextExtension != nil,
		)
	} else if contextExtension != nil {
		systemPrompt = piRPCContextReadSystemPrompt
	}
	if len(systemPrompt) > piRPCMaxSystemPromptBytes {
		return nil, ErrPiRPCProtocol
	}
	arguments := []string{
		"--mode", "rpc",
		"--offline",
		"--no-approve",
		"--no-session",
		"--no-extensions",
		"--no-skills",
		"--no-prompt-templates",
		"--no-themes",
		"--no-context-files",
		"--provider", adapter.providerID,
		"--model", adapter.providerID + "/" + adapter.modelID,
		"--thinking", "off",
		"--system-prompt", systemPrompt,
	}
	if contextExtension != nil || toolExtension != nil {
		arguments = append(arguments, "--no-builtin-tools")
		if contextExtension != nil {
			arguments = append(arguments, "--extension", contextExtension.extensionPath)
		}
		if toolExtension != nil {
			arguments = append(arguments, "--extension", toolExtension.extensionPath)
		}
	}
	skillRoot, err := executionMaterializationSkillRoot(
		request.WorkspacePath,
		request.Binding.RunID,
		request.Binding.ClaimGeneration,
		request.Binding.RuntimeInstanceID,
	)
	if err != nil {
		return nil, err
	}
	if skillRoot != "" {
		arguments = append(arguments, "--skill", skillRoot)
	}
	return arguments, nil
}

func piRPCToolBinding(request supervisor.AdapterRequest) ToolCallBinding {
	return ToolCallBinding{
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
		JourneyID:              request.Dispatch.CorrelationID(),
	}
}

func piRPCContainsCapability(capabilities []string, target string) bool {
	for _, capability := range capabilities {
		if capability == target {
			return true
		}
	}
	return false
}

func piRPCContextRetrieverPresent(retriever contextcapsule.Retriever) bool {
	return retriever != nil && !nilPiInterface(retriever)
}

func piRPCContextDeliveryPresent(delivery contextcapsule.DeliveryBroker) bool {
	return delivery != nil && !nilPiInterface(delivery)
}

func (adapter *piRPCBridgeAdapter) preparePrivatePiHome(
	homePath string,
) (string, string, error) {
	root := filepath.Join(homePath, ".pi")
	agentPath := filepath.Join(root, "agent")
	sessionPath := filepath.Join(root, "sessions")
	for _, path := range []string{root, agentPath, sessionPath} {
		if err := ensurePiRPCPrivateDirectory(path); err != nil {
			return "", "", err
		}
	}
	modelsJSON, err := adapter.modelsJSON()
	if err != nil {
		return "", "", ErrPiRPCProtocol
	}
	temporaryID, err := adapter.execution.randomUUID()
	if err != nil {
		return "", "", errors.Join(ErrPiRPCProtocol, err)
	}
	temporaryPath := filepath.Join(agentPath, ".models-"+temporaryID)
	modelsPath := filepath.Join(agentPath, "models.json")
	file, err := os.OpenFile(temporaryPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", "", ErrPiRPCProtocol
	}
	writeErr := error(nil)
	if _, err := file.Write(modelsJSON); err != nil {
		writeErr = err
	}
	if err := file.Sync(); err != nil {
		writeErr = errors.Join(writeErr, err)
	}
	if err := file.Close(); err != nil {
		writeErr = errors.Join(writeErr, err)
	}
	if writeErr != nil {
		_ = os.Remove(temporaryPath)
		return "", "", ErrPiRPCProtocol
	}
	if err := os.Rename(temporaryPath, modelsPath); err != nil {
		_ = os.Remove(temporaryPath)
		return "", "", ErrPiRPCProtocol
	}
	info, err := os.Lstat(modelsPath)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 {
		return "", "", ErrPiRPCProtocol
	}
	if err := materializePiRPCSettings(agentPath); err != nil {
		return "", "", err
	}
	return agentPath, sessionPath, nil
}

func materializePiRPCSettings(agentPath string) error {
	settingsPath := filepath.Join(agentPath, "settings.json")
	file, err := os.OpenFile(
		settingsPath,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0o600,
	)
	if err != nil {
		return ErrPiRPCProtocol
	}
	created := true
	defer func() {
		if created {
			_ = os.Remove(settingsPath)
		}
	}()
	if written, err := file.Write([]byte(piRPCSettingsJSON)); err != nil ||
		written != len(piRPCSettingsJSON) {
		_ = file.Close()
		return ErrPiRPCProtocol
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return ErrPiRPCProtocol
	}
	if err := file.Close(); err != nil {
		return ErrPiRPCProtocol
	}
	info, err := os.Lstat(settingsPath)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 {
		return ErrPiRPCProtocol
	}
	content, err := os.ReadFile(settingsPath)
	if err != nil || !bytes.Equal(content, []byte(piRPCSettingsJSON)) {
		return ErrPiRPCProtocol
	}
	created = false
	return nil
}

func (adapter *piRPCBridgeAdapter) modelsJSON() ([]byte, error) {
	return piModelCatalogJSON(adapter.providerID, adapter.modelID, adapter.baseURL)
}

func (adapter *piRPCBridgeAdapter) acceptRPCLine(
	ctx context.Context,
	request *supervisor.AdapterRequest,
	state *piRPCState,
	line []byte,
) error {
	if request == nil && !state.conversation {
		return ErrPiRPCProtocol
	}
	state.recordCount++
	if state.recordCount > piRPCMaxRecords {
		return ErrPiRPCOutputTooLarge
	}
	fields, err := piRPCObject(line)
	if err != nil {
		return err
	}
	recordType, ok := piRPCString(fields, "type")
	if !ok {
		return ErrPiRPCProtocol
	}
	if state.toolMode && state.contextMode {
		return adapter.acceptHybridRPCRecord(ctx, request, state, fields, recordType)
	}
	if state.toolMode {
		return adapter.acceptToolRPCRecord(ctx, request, state, fields, recordType)
	}
	if state.contextMode {
		state.contextRecordType = recordType
		if message, present := fields["message"]; present {
			if messageFields, messageErr := piRPCObject(message); messageErr == nil {
				state.contextRecordRole, _ = piRPCString(messageFields, "role")
				keys := make([]string, 0, len(messageFields))
				for key := range messageFields {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				state.contextRecordKeys = strings.Join(keys, ",")
			}
		}
		if rawEvent, present := fields["assistantMessageEvent"]; present {
			if eventFields, eventErr := piRPCObject(rawEvent); eventErr == nil {
				state.contextRecordEvent, _ = piRPCString(eventFields, "type")
			}
		}
		return adapter.acceptContextRPCRecord(ctx, request, state, fields, recordType)
	}
	switch recordType {
	case "response":
		if state.responseSeen || state.agentStarted ||
			!piRPCExactKeys(fields, "id", "type", "command", "success") {
			return ErrPiRPCProtocol
		}
		id, idOK := piRPCString(fields, "id")
		command, commandOK := piRPCString(fields, "command")
		success, successOK := piRPCBool(fields, "success")
		if !idOK || !commandOK || !successOK ||
			id != state.messageID ||
			command != "prompt" ||
			!success {
			return ErrPiRPCProtocol
		}
		if state.conversation {
			state.responseSeen = true
			break
		}
		if !state.dispatchAcknowledged {
			frame, err := adapter.execution.outboundFrame(
				*request,
				state.nextSequence,
				bridgev1.MessageAck,
				mustPiPayload(map[string]string{"message_id": request.Dispatch.MessageID()}),
			)
			if err != nil {
				return errors.Join(ErrPiRPCProtocol, err)
			}
			if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
				return errors.Join(ErrPiRPCProtocol, err)
			}
			state.frames = append(state.frames, frame)
			state.nextSequence++
			state.dispatchAcknowledged = true
		}
		state.responseSeen = true
	case "agent_start":
		if !state.responseSeen || state.agentStarted ||
			!piRPCExactKeys(fields, "type") {
			return ErrPiRPCProtocol
		}
		state.agentStarted = true
	case "turn_start":
		if !state.agentStarted || state.agentEnded || state.turnOpen ||
			state.turnCount != 0 ||
			state.messageOpen ||
			!piRPCExactKeys(fields, "type") {
			return ErrPiRPCProtocol
		}
		state.turnOpen = true
		state.turnCount++
	case "message_start":
		if !state.turnOpen || state.messageOpen ||
			!piRPCExactKeys(fields, "type", "message") {
			return ErrPiRPCProtocol
		}
		switch {
		case !state.userSeen:
			if !piRPCUserMessage(fields["message"], state.prompt) {
				return ErrPiRPCProtocol
			}
			state.messageRole = "user"
			state.userSeen = true
		case !state.assistantSeen:
			if !piRPCAssistantTextMessage(fields["message"], "", false) {
				return ErrPiRPCProtocol
			}
			identity, ok := piRPCInitialAssistantIdentity(fields["message"])
			if !ok {
				return ErrPiRPCProtocol
			}
			state.assistantIdentity = identity
			state.messageRole = "assistant"
			state.assistantSeen = true
		default:
			return ErrPiRPCProtocol
		}
		state.messageOpen = true
	case "message_update":
		if !state.messageOpen || state.messageRole != "assistant" ||
			!piRPCExactKeys(fields, "type", "message", "assistantMessageEvent") ||
			state.doneSeen {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				piRPCEventUnknown,
				piRPCReasonEventShape,
			)
		}
		if err := adapter.acceptAssistantEvent(
			ctx,
			request,
			state,
			fields["message"],
			fields["assistantMessageEvent"],
		); err != nil {
			return err
		}
	case "message_end":
		if !state.messageOpen ||
			!piRPCExactKeys(fields, "type", "message") {
			if state.assistantSeen {
				return rejectPiRPC(
					piRPCPhaseAssistantMessageEnd,
					piRPCEventMessageEnd,
					piRPCReasonEventShape,
				)
			}
			return ErrPiRPCProtocol
		}
		switch state.messageRole {
		case "user":
			if state.assistantSeen ||
				!piRPCUserMessage(fields["message"], state.prompt) {
				return ErrPiRPCProtocol
			}
		case "assistant":
			if state.doneSeen ||
				(!state.textEnded && !state.toolEnded) {
				return rejectPiRPC(
					piRPCPhaseAssistantMessageEnd,
					piRPCEventMessageEnd,
					piRPCAssistantRejectionReason(
						fields["message"],
						string(state.assistant),
						true,
						state.assistantIdentity,
						false,
						true,
					),
				)
			}
			if state.textEnded {
				if len(state.assistant) == 0 ||
					!piRPCAssistantTextMessage(
						fields["message"],
						string(state.assistant),
						true,
					) {
					return rejectPiRPC(
						piRPCPhaseAssistantMessageEnd,
						piRPCEventMessageEnd,
						piRPCAssistantRejectionReason(
							fields["message"],
							string(state.assistant),
							true,
							state.assistantIdentity,
							false,
							true,
						),
					)
				}
			} else if _, err := piRPCObject(fields["message"]); err != nil {
				return rejectPiRPC(
					piRPCPhaseAssistantMessageEnd,
					piRPCEventMessageEnd,
					piRPCReasonAssistantMessageSchema,
				)
			}
			if !state.assistantIdentity.acceptTerminal(fields["message"]) {
				return rejectPiRPC(
					piRPCPhaseAssistantMessageEnd,
					piRPCEventMessageEnd,
					piRPCAssistantRejectionReason(
						fields["message"],
						string(state.assistant),
						true,
						state.assistantIdentity,
						false,
						true,
					),
				)
			}
			state.doneSeen = true
			state.finalAssistant = bytes.Clone(fields["message"])
		default:
			return ErrPiRPCProtocol
		}
		state.messageOpen = false
		state.messageRole = ""
	case "turn_end":
		if !state.turnOpen || state.messageOpen || !state.userSeen ||
			!state.assistantSeen || !state.doneSeen {
			return rejectPiRPC(
				piRPCPhaseTurnEnd,
				piRPCEventTurnEnd,
				piRPCReasonTerminalIdentity,
			)
		}
		if !piRPCExactKeys(fields, "type", "message", "toolResults") {
			return rejectPiRPC(
				piRPCPhaseTurnEnd,
				piRPCEventTurnEnd,
				piRPCReasonEventShape,
			)
		}
		if state.textEnded {
			if !piRPCAssistantTextMessage(
				fields["message"],
				string(state.assistant),
				true,
			) {
				return rejectPiRPC(
					piRPCPhaseTurnEnd,
					piRPCEventTurnEnd,
					piRPCAssistantRejectionReason(
						fields["message"],
						string(state.assistant),
						true,
						state.assistantIdentity,
						false,
						true,
					),
				)
			}
		} else if _, err := piRPCObject(fields["message"]); err != nil {
			return rejectPiRPC(
				piRPCPhaseTurnEnd,
				piRPCEventTurnEnd,
				piRPCReasonAssistantMessageSchema,
			)
		}
		if !piRPCSemanticEqual(fields["message"], state.finalAssistant) ||
			!piRPCEmptyArray(fields["toolResults"]) {
			return rejectPiRPC(
				piRPCPhaseTurnEnd,
				piRPCEventTurnEnd,
				piRPCReasonTerminalIdentity,
			)
		}
		state.turnOpen = false
	case "agent_end":
		if !state.agentStarted || state.turnOpen || state.messageOpen ||
			!state.doneSeen || state.agentEnded {
			return rejectPiRPC(
				piRPCPhaseAgentEnd,
				piRPCEventAgentEnd,
				piRPCReasonTerminalIdentity,
			)
		}
		if !piRPCExactKeys(fields, "type", "messages", "willRetry") {
			return rejectPiRPC(
				piRPCPhaseAgentEnd,
				piRPCEventAgentEnd,
				piRPCReasonEventShape,
			)
		}
		if !piRPCAgentMessages(
			fields["messages"],
			state.prompt,
			state.finalAssistant,
		) {
			return rejectPiRPC(
				piRPCPhaseAgentEnd,
				piRPCEventAgentEnd,
				piRPCReasonTerminalIdentity,
			)
		}
		willRetry, ok := piRPCBool(fields, "willRetry")
		if !ok || willRetry {
			return rejectPiRPC(
				piRPCPhaseAgentEnd,
				piRPCEventAgentEnd,
				piRPCReasonTerminalIdentity,
			)
		}
		state.agentEnded = true
	case "agent_settled":
		if !piRPCExactKeys(fields, "type") {
			return rejectPiRPC(
				piRPCPhaseAgentSettled,
				piRPCEventAgentSettled,
				piRPCReasonEventShape,
			)
		}
		if !state.agentEnded || state.settled || state.turnCount == 0 ||
			(!state.textEnded && !state.toolEnded) ||
			(state.textEnded &&
				(len(state.assistant) == 0 ||
					!bytes.Equal(state.lastPartial, state.assistant))) {
			return rejectPiRPC(
				piRPCPhaseAgentSettled,
				piRPCEventAgentSettled,
				piRPCReasonTerminalIdentity,
			)
		}
		state.settled = true
	default:
		return ErrPiRPCProtocol
	}
	return nil
}

func (adapter *piRPCBridgeAdapter) acceptAssistantEvent(
	ctx context.Context,
	request *supervisor.AdapterRequest,
	state *piRPCState,
	message json.RawMessage,
	raw json.RawMessage,
) error {
	fields, err := piRPCObject(raw)
	if err != nil {
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			piRPCEventUnknown,
			piRPCReasonEventShape,
		)
	}
	eventType, ok := piRPCString(fields, "type")
	if !ok {
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			piRPCEventUnknown,
			piRPCReasonEventShape,
		)
	}
	event := piRPCDiagnosticEventOf(eventType)
	if !piRPCAssistantEventShape(fields, event) {
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			event,
			piRPCReasonEventShape,
		)
	}
	switch event {
	case piRPCEventTextStart:
		if state.textStarted || state.textEnded || state.doneSeen {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonTextContentProgression,
			)
		}
	case piRPCEventTextDelta:
		if !state.textStarted || state.textEnded || state.doneSeen {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonTextContentProgression,
			)
		}
	case piRPCEventTextEnd:
		if !state.textStarted || state.textEnded || state.doneSeen {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonTextContentProgression,
			)
		}
	case piRPCEventToolCallStart:
		if state.conversation {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonEventKindUnsupported,
			)
		}
		if state.toolStarted || state.toolEnded || state.doneSeen {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonToolCallProgression,
			)
		}
	case piRPCEventToolCallDelta:
		if state.conversation {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonEventKindUnsupported,
			)
		}
		if !state.toolStarted || state.toolEnded || state.doneSeen {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonToolCallProgression,
			)
		}
	case piRPCEventToolCallEnd:
		if state.conversation {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonEventKindUnsupported,
			)
		}
		if !state.toolStarted || state.toolEnded || state.doneSeen {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonToolCallProgression,
			)
		}
	default:
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			event,
			piRPCReasonEventKindUnsupported,
		)
	}

	if event == piRPCEventToolCallStart ||
		event == piRPCEventToolCallDelta ||
		event == piRPCEventToolCallEnd {
		if request == nil {
			return ErrPiRPCProtocol
		}
		return adapter.acceptToolCallEvent(ctx, *request, state, fields, event)
	}

	if !piRPCZero(fields["contentIndex"]) {
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			event,
			piRPCReasonContentIndex,
		)
	}
	partialText, ok := piRPCAssistantText(fields["partial"], true)
	if !ok {
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			event,
			piRPCAssistantRejectionReason(
				fields["partial"],
				"",
				true,
				state.assistantIdentity,
				event == piRPCEventTextStart,
				false,
			),
		)
	}
	messageText, ok := piRPCAssistantText(message, true)
	if !ok {
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			event,
			piRPCAssistantRejectionReason(
				message,
				"",
				true,
				state.assistantIdentity,
				event == piRPCEventTextStart,
				false,
			),
		)
	}
	witnessText, ok := piRPCAssistantTextWitness(
		message,
		messageText,
		fields["partial"],
		partialText,
		event != piRPCEventTextEnd,
	)
	if !ok {
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			event,
			piRPCReasonMessagePartialMismatch,
		)
	}
	messageSnapshot := []byte(messageText)
	partial := []byte(partialText)
	witness := []byte(witnessText)
	forbidden := state.forbidden
	if len(messageSnapshot) > adapter.maxAssistantBytes ||
		len(partial) > adapter.maxAssistantBytes {
		return ErrPiRPCOutputTooLarge
	}
	if len(forbidden) > 0 &&
		(bytes.Contains(messageSnapshot, forbidden) ||
			bytes.Contains(partial, forbidden)) ||
		!bytes.HasPrefix(witness, state.lastPartial) {
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			event,
			piRPCReasonTextContentProgression,
		)
	}
	identity := state.assistantIdentity
	if !identity.acceptUpdate(
		fields["partial"],
		event == piRPCEventTextStart,
	) {
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			event,
			piRPCAssistantRejectionReason(
				fields["partial"],
				partialText,
				true,
				state.assistantIdentity,
				event == piRPCEventTextStart,
				false,
			),
		)
	}

	switch event {
	case piRPCEventTextStart:
		if !bytes.HasPrefix(messageSnapshot, state.assistant) ||
			!bytes.HasPrefix(partial, state.assistant) {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonTextContentProgression,
			)
		}
		state.assistantIdentity = identity
		state.lastPartial = bytes.Clone(witness)
		state.forwardPartial = len(witness) > len(state.assistant)
		state.textStartBytes = len(witness)
		state.acceptedAtStart = len(state.assistant)
		state.textStarted = true
	case piRPCEventTextDelta:
		delta, ok := piRPCString(fields, "delta")
		if !ok ||
			!utf8.Valid(fields["delta"]) ||
			delta == "" ||
			!utf8.ValidString(delta) ||
			(len(forbidden) > 0 && bytes.Contains([]byte(delta), forbidden)) {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonDeltaPolicy,
			)
		}
		if len(state.assistant)+len(delta) > adapter.maxAssistantBytes {
			return ErrPiRPCOutputTooLarge
		}
		candidate := make([]byte, 0, len(state.assistant)+len(delta))
		candidate = append(candidate, state.assistant...)
		candidate = append(candidate, delta...)
		if !bytes.HasPrefix(messageSnapshot, candidate) ||
			!bytes.HasPrefix(partial, candidate) {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonTextContentProgression,
			)
		}
		chunks := splitPiRPCDelta([]byte(delta))
		if state.conversation {
			state.assistantIdentity = identity
			state.assistant = candidate
			state.lastPartial = bytes.Clone(witness)
			state.forwardPartial = state.forwardPartial ||
				len(witness) > len(candidate)
			state.deltaCount++
			break
		}
		if len(state.frames)+len(chunks) > piRPCMaxBridgeFrames {
			return ErrPiRPCOutputTooLarge
		}
		frames := make([]bridgev1.Frame, 0, len(chunks))
		for offset, chunk := range chunks {
			payload, err := json.Marshal(struct {
				Delta string `json:"delta"`
			}{Delta: string(chunk)})
			if err != nil {
				return rejectPiRPC(
					piRPCPhaseAssistantUpdate,
					event,
					piRPCReasonDeltaPolicy,
				)
			}
			frame, err := adapter.execution.outboundFrame(
				*request,
				state.nextSequence+int64(offset),
				bridgev1.MessageEvent,
				payload,
			)
			if err != nil {
				return rejectPiRPC(
					piRPCPhaseAssistantUpdate,
					event,
					piRPCReasonFrameSink,
				)
			}
			frames = append(frames, frame)
		}
		for _, frame := range frames {
			if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
				return rejectPiRPC(
					piRPCPhaseAssistantUpdate,
					event,
					piRPCReasonFrameSink,
				)
			}
		}
		state.assistantIdentity = identity
		state.assistant = candidate
		state.lastPartial = bytes.Clone(witness)
		state.forwardPartial = state.forwardPartial ||
			len(witness) > len(candidate)
		state.deltaCount++
		state.frames = append(state.frames, frames...)
		state.nextSequence += int64(len(frames))
	case piRPCEventTextEnd:
		content, ok := piRPCString(fields, "content")
		if !ok ||
			!utf8.Valid(fields["content"]) ||
			!utf8.ValidString(content) ||
			content != messageText ||
			content != partialText ||
			!bytes.Equal(witness, state.lastPartial) ||
			!bytes.Equal(witness, state.assistant) {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonTextContentProgression,
			)
		}
		state.assistantIdentity = identity
		state.lastPartial = bytes.Clone(witness)
		state.textEnded = true
	}
	return nil
}

func (adapter *piRPCBridgeAdapter) acceptToolCallEvent(
	ctx context.Context,
	request supervisor.AdapterRequest,
	state *piRPCState,
	fields map[string]json.RawMessage,
	event piRPCDiagnosticEvent,
) error {
	if !piRPCZero(fields["contentIndex"]) {
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			event,
			piRPCReasonContentIndex,
		)
	}
	switch event {
	case piRPCEventToolCallStart:
		state.toolStarted = true
		state.toolCallBytes = nil
		if !state.assistantIdentity.acceptUpdate(fields["partial"], true) {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCAssistantRejectionReason(
					fields["partial"],
					"",
					true,
					state.assistantIdentity,
					true,
					false,
				),
			)
		}
		return nil
	case piRPCEventToolCallDelta:
		partial, ok := fields["partial"]
		if !ok {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonEventShape,
			)
		}
		state.toolCallBytes = bytes.Clone(partial)
		if !state.assistantIdentity.acceptUpdate(partial, false) {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCAssistantRejectionReason(
					partial,
					"",
					true,
					state.assistantIdentity,
					false,
					false,
				),
			)
		}
		return nil
	case piRPCEventToolCallEnd:
		state.toolEnded = true
		if !state.assistantIdentity.acceptUpdate(fields["partial"], false) {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCAssistantRejectionReason(
					fields["partial"],
					"",
					true,
					state.assistantIdentity,
					false,
					false,
				),
			)
		}
		envelopeBytes, err := piRPCToolCallEnvelope(fields["toolCall"])
		if err != nil {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonToolCallInvalid,
			)
		}
		envelope, err := DecodeToolCallEnvelope(envelopeBytes)
		if err != nil {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonToolCallInvalid,
			)
		}
		if adapter.toolHook == nil {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonEventKindUnsupported,
			)
		}
		binding := ToolCallBinding{
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
			// The RunStreamBinding has no journey field; the dispatch frame's
			// CorrelationID is the journey/correlation lineage that the hook
			// must carry into execution.Proposal for approval facts.
			JourneyID: request.Dispatch.CorrelationID(),
		}
		result, err := adapter.toolHook.ExecuteToolCall(ctx, envelope, binding)
		if err != nil {
			return rejectPiRPC(
				piRPCPhaseAssistantUpdate,
				event,
				piRPCReasonToolCallHookFailed,
			)
		}
		if err := adapter.emitToolCallResult(ctx, request, state, envelope, result); err != nil {
			return err
		}
		if acknowledger, ok := adapter.toolHook.(ToolCallResultAcknowledger); ok {
			if err := acknowledger.AcknowledgeToolCallResult(ctx, binding, result); err != nil {
				return rejectPiRPC(
					piRPCPhaseAssistantUpdate,
					event,
					piRPCReasonToolCallHookFailed,
				)
			}
		}
		state.toolResultSent = true
		return nil
	default:
		return rejectPiRPC(
			piRPCPhaseAssistantUpdate,
			event,
			piRPCReasonEventKindUnsupported,
		)
	}
}

const piRPCBridgeToolName = "loom_tool"

// piRPCToolCallEnvelope 从 toolcall_end 的 toolCall 对象提取信封 JSON：
// 必须恰好 {type:"toolCall", name:"loom_tool", arguments:<对象>}；
// arguments 必须是 JSON 对象（拒绝字符串化嵌套，防注入）。
func piRPCToolCallEnvelope(raw json.RawMessage) ([]byte, error) {
	if len(raw) == 0 {
		return nil, ErrPiRPCProtocol
	}
	fields, err := piRPCObject(raw)
	if err != nil {
		return nil, err
	}
	if !piRPCExactKeys(fields, "type", "name", "arguments") {
		return nil, ErrPiRPCProtocol
	}
	if kind, ok := piRPCString(fields, "type"); !ok || kind != "toolCall" {
		return nil, ErrPiRPCProtocol
	}
	if name, ok := piRPCString(fields, "name"); !ok || name != piRPCBridgeToolName {
		return nil, ErrPiRPCProtocol
	}
	arguments := fields["arguments"]
	if len(arguments) == 0 {
		return nil, ErrPiRPCProtocol
	}
	if _, err := piRPCObject(arguments); err != nil {
		return nil, ErrPiRPCProtocol
	}
	return bytes.Clone(arguments), nil
}

// emitToolCallResult 把裁决结果写入 audit 摘要（契约 §3.3）并作为 event
// frame 推给客户端观察；结果只含已批准执行的内容，ask/deny 只有 verdict
// 与 reason 摘要。
func (adapter *piRPCBridgeAdapter) emitToolCallResult(
	ctx context.Context,
	request supervisor.AdapterRequest,
	state *piRPCState,
	envelope ToolCallEnvelope,
	result ToolCallResult,
) error {
	entry := ToolCallAuditEntry{
		Verdict:      result.Verdict,
		Tool:         string(envelope.Call.Tool),
		ExecutionID:  result.ExecutionID,
		ResultNote:   result.ResultNote,
		DenialReason: result.DenialReason,
	}
	state.toolResults = append(state.toolResults, entry)
	payload, err := marshalToolCallResultPayload(envelope, result)
	if err != nil {
		return ErrPiRPCProtocol
	}
	frame, err := adapter.execution.outboundFrame(
		request,
		state.nextSequence,
		bridgev1.MessageEvent,
		payload,
	)
	if err != nil {
		return errors.Join(ErrPiRPCProtocol, err)
	}
	if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
		return errors.Join(ErrPiRPCProtocol, err)
	}
	state.frames = append(state.frames, frame)
	state.nextSequence++
	return nil
}

func (adapter *piRPCBridgeAdapter) finishRPCFrames(
	ctx context.Context,
	request supervisor.AdapterRequest,
	state *piRPCState,
) error {
	digest := sha256.Sum256(state.assistant)
	evidencePayload, err := json.Marshal(struct {
		Kind   string `json:"kind"`
		SHA256 string `json:"sha256"`
		Bytes  int    `json:"bytes"`
	}{
		Kind:   "assistant_text_digest",
		SHA256: hex.EncodeToString(digest[:]),
		Bytes:  len(state.assistant),
	})
	if err != nil {
		return ErrPiRPCProtocol
	}
	resultPayload := []byte(`{"status":"succeeded","reason":""}`)
	for _, output := range []struct {
		messageType bridgev1.MessageType
		payload     []byte
	}{
		{messageType: bridgev1.MessageEvidence, payload: evidencePayload},
		{messageType: bridgev1.MessageResult, payload: resultPayload},
	} {
		if len(state.frames) >= piRPCMaxBridgeFrames {
			return ErrPiRPCOutputTooLarge
		}
		frame, err := adapter.execution.outboundFrame(
			request,
			state.nextSequence,
			output.messageType,
			output.payload,
		)
		if err != nil {
			return errors.Join(ErrPiRPCProtocol, err)
		}
		if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
			return errors.Join(ErrPiRPCProtocol, err)
		}
		state.frames = append(state.frames, frame)
		state.nextSequence++
	}
	return nil
}

func (adapter *piRPCBridgeAdapter) cancelRPC(
	command *exec.Cmd,
	wait <-chan error,
	stdin io.WriteCloser,
	lines <-chan piRPCLineResult,
	contextErr error,
) error {
	abortID, err := adapter.execution.randomUUID()
	if err != nil {
		return adapter.failRPC(command, wait, stdin, errors.Join(ErrPiRPCProtocol, contextErr))
	}
	abortLine, err := json.Marshal(struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}{ID: abortID, Type: "abort"})
	if err != nil {
		return adapter.failRPC(command, wait, stdin, errors.Join(ErrPiRPCProtocol, contextErr))
	}
	abortLine = append(abortLine, '\n')
	if _, err := stdin.Write(abortLine); err != nil {
		return adapter.failRPC(command, wait, stdin, errors.Join(ErrPiRPCProtocol, contextErr))
	}
	timer := time.NewTimer(adapter.execution.cancelGrace)
	defer timer.Stop()
	acknowledged := false
	for !acknowledged {
		select {
		case lineResult := <-lines:
			if lineResult.err != nil {
				return adapter.failRPC(command, wait, stdin, errors.Join(ErrPiRPCProtocol, contextErr))
			}
			fields, err := piRPCObject(lineResult.line)
			if err != nil || !piRPCExactKeys(fields, "id", "type", "command", "success") {
				continue
			}
			id, idOK := piRPCString(fields, "id")
			recordType, typeOK := piRPCString(fields, "type")
			rpcCommand, commandOK := piRPCString(fields, "command")
			success, successOK := piRPCBool(fields, "success")
			acknowledged = idOK && typeOK && commandOK && successOK &&
				id == abortID && recordType == "response" &&
				rpcCommand == "abort" && success
		case <-timer.C:
			return adapter.failRPC(command, wait, stdin, errors.Join(ErrPiRPCProtocol, contextErr))
		}
	}
	_ = stdin.Close()
	cleanupErr := terminateExecutionProcess(command, wait, adapter.execution.cancelGrace)
	if cleanupErr != nil {
		cleanupErr = errors.Join(ErrPiRPCCleanup, cleanupErr)
	}
	return errors.Join(ErrPiRPCProtocol, contextErr, cleanupErr)
}

func (adapter *piRPCBridgeAdapter) failRPC(
	command *exec.Cmd,
	wait <-chan error,
	stdin io.WriteCloser,
	primary error,
) error {
	if stdin != nil {
		_ = stdin.Close()
	}
	cleanupErr := terminateExecutionProcess(command, wait, adapter.execution.cancelGrace)
	if cleanupErr != nil {
		cleanupErr = errors.Join(ErrPiRPCCleanup, cleanupErr)
	}
	return errors.Join(primary, cleanupErr)
}

func (adapter *piRPCBridgeAdapter) waitForRPCExit(
	command *exec.Cmd,
	wait <-chan error,
) (error, error) {
	timer := time.NewTimer(adapter.execution.cancelGrace)
	defer timer.Stop()
	select {
	case err := <-wait:
		return err, nil
	case <-timer.C:
		return nil, terminateExecutionProcess(command, wait, adapter.execution.cancelGrace)
	}
}

func parsePiRPCDispatch(payload []byte, grant string) (string, error) {
	if contextDispatch, err := contextcapsule.DecodeDispatchPayload(payload); err == nil {
		if validPiRPCPrompt(contextDispatch.Prompt, grant) {
			return contextDispatch.Prompt, nil
		}
		return "", ErrPiRPCProtocol
	}
	if !utf8.Valid(payload) || bytes.ContainsRune(payload, '\x00') {
		return "", ErrPiRPCProtocol
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, payload); err != nil ||
		!bytes.Equal(compact.Bytes(), payload) {
		return "", ErrPiRPCProtocol
	}
	if err := rejectPiRPCDuplicateKeys(payload); err != nil {
		return "", ErrPiRPCProtocol
	}
	var dispatch struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Prompt        string `json:"prompt"`
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dispatch); err != nil ||
		dispatch.SchemaVersion != 1 ||
		dispatch.Kind != "pi_rpc_prompt" ||
		!validPiRPCPrompt(dispatch.Prompt, grant) {
		return "", ErrPiRPCProtocol
	}
	canonical, err := json.Marshal(dispatch)
	if err != nil || !bytes.Equal(canonical, payload) {
		return "", ErrPiRPCProtocol
	}
	return dispatch.Prompt, nil
}

func validPiRPCPrompt(prompt string, grant string) bool {
	if prompt == "" ||
		len(prompt) > piRPCMaxPromptBytes ||
		!utf8.ValidString(prompt) ||
		strings.HasPrefix(prompt, "/") ||
		grant == "" ||
		strings.Contains(prompt, grant) {
		return false
	}
	for _, character := range prompt {
		if character == '\n' || character == '\t' {
			continue
		}
		if unicode.IsControl(character) {
			return false
		}
	}
	upper := strings.ToUpper(prompt)
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

func validPiRPCBaseURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil ||
		parsed.Scheme != "http" ||
		parsed.Hostname() != "127.0.0.1" ||
		parsed.Path != "/v1" ||
		parsed.RawPath != "" ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" ||
		parsed.String() != value {
		return false
	}
	port, err := strconv.Atoi(parsed.Port())
	return err == nil && port >= 1024 && port <= 65535 &&
		parsed.Host == "127.0.0.1:"+strconv.Itoa(port)
}

func ensurePiRPCPrivateDirectory(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return ErrPiRPCProtocol
	}
	info, err := os.Lstat(path)
	if err != nil ||
		!info.IsDir() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o700 ||
		!piLocalCurrentUserOwns(info) {
		return ErrPiRPCProtocol
	}
	return nil
}

func scanPiRPCLines(
	ctx context.Context,
	reader io.Reader,
	output chan<- piRPCLineResult,
) {
	buffer := bufio.NewReaderSize(reader, piRPCMaxLineBytes+1)
	total := 0
	for {
		line, err := buffer.ReadSlice('\n')
		total += len(line)
		switch {
		case errors.Is(err, bufio.ErrBufferFull),
			len(line) > piRPCMaxLineBytes,
			total > piRPCMaxStdoutBytes:
			err = ErrPiRPCOutputTooLarge
		case err == nil &&
			(bytes.ContainsRune(line, '\r') ||
				bytes.Contains(line, []byte{0xe2, 0x80, 0xa8}) ||
				bytes.Contains(line, []byte{0xe2, 0x80, 0xa9})):
			err = ErrPiRPCProtocol
		case err == nil:
			line = bytes.Clone(bytes.TrimSuffix(line, []byte{'\n'}))
		case len(line) > 0:
			err = ErrPiRPCProtocol
		}
		select {
		case output <- piRPCLineResult{line: line, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func readPiRPCStderr(reader io.Reader, output chan<- piStderrResult) {
	content, err := io.ReadAll(io.LimitReader(reader, piRPCMaxStderrBytes+1))
	if err == nil && len(content) > piRPCMaxStderrBytes {
		content = nil
		err = ErrPiRPCOutputTooLarge
	}
	output <- piStderrResult{content: content, err: err}
}

func splitPiRPCDelta(delta []byte) [][]byte {
	var chunks [][]byte
	for len(delta) > 0 {
		size := len(delta)
		if size > piRPCMaxDeltaBytes {
			size = piRPCMaxDeltaBytes
			for size > 0 && !utf8.RuneStart(delta[size]) {
				size--
			}
		}
		chunks = append(chunks, bytes.Clone(delta[:size]))
		delta = delta[size:]
	}
	return chunks
}

func piRPCObject(value []byte) (map[string]json.RawMessage, error) {
	if err := rejectPiRPCDuplicateKeys(value); err != nil {
		return nil, ErrPiRPCProtocol
	}
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(value))
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return nil, ErrPiRPCProtocol
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) || token != nil {
		return nil, ErrPiRPCProtocol
	}
	return fields, nil
}

func rejectPiRPCDuplicateKeys(value []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(value))
	if err := scanPiRPCJSONValue(decoder); err != nil {
		return err
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) || token != nil {
		return ErrPiRPCProtocol
	}
	return nil
}

func scanPiRPCJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return ErrPiRPCProtocol
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return ErrPiRPCProtocol
			}
			key, ok := keyToken.(string)
			if !ok {
				return ErrPiRPCProtocol
			}
			if _, duplicate := seen[key]; duplicate {
				return ErrPiRPCProtocol
			}
			seen[key] = struct{}{}
			if err := scanPiRPCJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return ErrPiRPCProtocol
		}
	case '[':
		for decoder.More() {
			if err := scanPiRPCJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return ErrPiRPCProtocol
		}
	default:
		return ErrPiRPCProtocol
	}
	return nil
}

func piRPCExactKeys(fields map[string]json.RawMessage, expected ...string) bool {
	if len(fields) != len(expected) {
		return false
	}
	for _, key := range expected {
		if _, ok := fields[key]; !ok {
			return false
		}
	}
	return true
}

func piRPCString(fields map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := fields[key]
	if !ok {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil || !utf8.ValidString(value) {
		return "", false
	}
	return value, true
}

func piRPCBool(fields map[string]json.RawMessage, key string) (bool, bool) {
	raw, ok := fields[key]
	if !ok {
		return false, false
	}
	var value bool
	if json.Unmarshal(raw, &value) != nil {
		return false, false
	}
	return value, true
}

func piRPCZero(raw json.RawMessage) bool {
	return bytes.Equal(raw, []byte("0"))
}

func piRPCEmptyArray(raw json.RawMessage) bool {
	var value []json.RawMessage
	return json.Unmarshal(raw, &value) == nil && len(value) == 0
}

func piRPCAllowedKeys(
	fields map[string]json.RawMessage,
	required []string,
	optional ...string,
) bool {
	if len(fields) < len(required) ||
		len(fields) > len(required)+len(optional) {
		return false
	}
	allowed := make(map[string]struct{}, len(required)+len(optional))
	for _, key := range required {
		if _, ok := fields[key]; !ok {
			return false
		}
		allowed[key] = struct{}{}
	}
	for _, key := range optional {
		allowed[key] = struct{}{}
	}
	for key := range fields {
		if _, ok := allowed[key]; !ok {
			return false
		}
	}
	return true
}

func piRPCDiagnosticEventOf(value string) piRPCDiagnosticEvent {
	switch value {
	case string(piRPCEventTextStart):
		return piRPCEventTextStart
	case string(piRPCEventTextDelta):
		return piRPCEventTextDelta
	case string(piRPCEventTextEnd):
		return piRPCEventTextEnd
	case string(piRPCEventThinkingStart):
		return piRPCEventThinkingStart
	case string(piRPCEventThinkingDelta):
		return piRPCEventThinkingDelta
	case string(piRPCEventThinkingEnd):
		return piRPCEventThinkingEnd
	case string(piRPCEventToolCallStart):
		return piRPCEventToolCallStart
	case string(piRPCEventToolCallDelta):
		return piRPCEventToolCallDelta
	case string(piRPCEventToolCallEnd):
		return piRPCEventToolCallEnd
	default:
		return piRPCEventUnknown
	}
}

func piRPCDiagnosticEventFromRaw(raw json.RawMessage) piRPCDiagnosticEvent {
	fields, err := piRPCObject(raw)
	if err != nil {
		return piRPCEventUnknown
	}
	value, ok := piRPCString(fields, "type")
	if !ok {
		return piRPCEventUnknown
	}
	return piRPCDiagnosticEventOf(value)
}

func piRPCAssistantEventShape(
	fields map[string]json.RawMessage,
	event piRPCDiagnosticEvent,
) bool {
	switch event {
	case piRPCEventTextStart, piRPCEventThinkingStart,
		piRPCEventToolCallStart:
		return piRPCExactKeys(fields, "type", "contentIndex", "partial")
	case piRPCEventTextDelta, piRPCEventThinkingDelta,
		piRPCEventToolCallDelta:
		return piRPCExactKeys(
			fields,
			"type",
			"contentIndex",
			"delta",
			"partial",
		)
	case piRPCEventTextEnd, piRPCEventThinkingEnd:
		return piRPCExactKeys(
			fields,
			"type",
			"contentIndex",
			"content",
			"partial",
		)
	case piRPCEventToolCallEnd:
		return piRPCExactKeys(
			fields,
			"type",
			"contentIndex",
			"toolCall",
			"partial",
		)
	case piRPCEventUnknown:
		return true
	default:
		return false
	}
}

func piRPCNonNegativeNumber(raw json.RawMessage) bool {
	if bytes.Equal(raw, []byte("null")) {
		return false
	}
	var value float64
	return json.Unmarshal(raw, &value) == nil &&
		value >= 0 &&
		!math.IsInf(value, 0) &&
		!math.IsNaN(value)
}

func piRPCBoundedString(
	fields map[string]json.RawMessage,
	key string,
	maxBytes int,
) bool {
	value, ok := piRPCString(fields, key)
	if !ok || value == "" || len(value) > maxBytes {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func piRPCAssistantRejectionReason(
	raw json.RawMessage,
	expectedText string,
	requireContent bool,
	identity piRPCAssistantIdentityState,
	bindResponseID bool,
	terminal bool,
) piRPCDiagnosticReason {
	fields, err := piRPCObject(raw)
	if err != nil ||
		!piRPCAllowedKeys(
			fields,
			[]string{
				"role",
				"content",
				"api",
				"provider",
				"model",
				"usage",
				"stopReason",
				"timestamp",
			},
			"responseId",
			"responseModel",
		) {
		return piRPCReasonAssistantMessageSchema
	}
	role, roleOK := piRPCString(fields, "role")
	api, apiOK := piRPCString(fields, "api")
	provider, providerOK := piRPCString(fields, "provider")
	model, modelOK := piRPCString(fields, "model")
	stopReason, stopOK := piRPCString(fields, "stopReason")
	if !roleOK || !apiOK || !providerOK || !modelOK || !stopOK ||
		role != "assistant" ||
		api != "openai-completions" ||
		provider != piRPCProviderID ||
		model != piRPCModelID {
		return piRPCReasonAssistantMessageSchema
	}
	if stopReason != "stop" {
		return piRPCReasonTerminalStopReason
	}
	if _, present := fields["responseModel"]; present {
		return piRPCReasonResponseModelPresent
	}
	responseID, responsePresent := piRPCString(fields, "responseId")
	if responsePresent &&
		(responseID == "" || len(responseID) > 512) {
		return piRPCReasonResponseIDTransition
	}
	switch {
	case bindResponseID &&
		(identity.responseBound || !responsePresent):
		return piRPCReasonResponseIDTransition
	case !bindResponseID &&
		(!identity.responseBound ||
			!responsePresent ||
			responseID != identity.responseID):
		return piRPCReasonResponseIDTransition
	}
	if !piRPCNonNegativeNumber(fields["timestamp"]) ||
		string(fields["timestamp"]) != identity.timestamp {
		return piRPCReasonTimestampIdentity
	}
	if !piRPCUsage(fields["usage"]) {
		return piRPCReasonUsageSchema
	}
	usage, err := piRPCObject(fields["usage"])
	if err != nil {
		return piRPCReasonUsageSchema
	}
	_, cacheWrite1h := usage["cacheWrite1h"]
	_, reasoning := usage["reasoning"]
	if cacheWrite1h ||
		(identity.reasoningSeen && !reasoning) ||
		(terminal && reasoning != identity.reasoningSeen) {
		return piRPCReasonUsageProgression
	}
	contentRaw, present := fields["content"]
	var content []json.RawMessage
	if !present ||
		json.Unmarshal(contentRaw, &content) != nil ||
		len(content) > 1 {
		return piRPCReasonAssistantMessageSchema
	}
	var text strings.Builder
	for _, blockRaw := range content {
		block, err := piRPCObject(blockRaw)
		if err != nil ||
			!piRPCAllowedKeys(
				block,
				[]string{"type", "text"},
				"textSignature",
			) {
			return piRPCReasonAssistantMessageSchema
		}
		blockType, typeOK := piRPCString(block, "type")
		blockText, textOK := piRPCString(block, "text")
		if !typeOK || !textOK || blockType != "text" {
			return piRPCReasonAssistantMessageSchema
		}
		if _, present := block["textSignature"]; present &&
			!piRPCBoundedString(block, "textSignature", 8192) {
			return piRPCReasonAssistantMessageSchema
		}
		text.WriteString(blockText)
	}
	if text.String() != expectedText ||
		(requireContent && len(content) != 1) ||
		(!requireContent && len(content) != 0) {
		return piRPCReasonTextContentProgression
	}
	return piRPCReasonAssistantMessageSchema
}

func piRPCUserMessage(raw json.RawMessage, expectedPrompt []byte) bool {
	fields, err := piRPCObject(raw)
	if err != nil || !piRPCExactKeys(fields, "role", "content", "timestamp") {
		return false
	}
	role, ok := piRPCString(fields, "role")
	if !ok || role != "user" ||
		!piRPCNonNegativeNumber(fields["timestamp"]) {
		return false
	}
	var content []json.RawMessage
	if json.Unmarshal(fields["content"], &content) != nil ||
		len(content) != 1 {
		return false
	}
	block, err := piRPCObject(content[0])
	if err != nil || !piRPCExactKeys(block, "type", "text") {
		return false
	}
	blockType, typeOK := piRPCString(block, "type")
	text, textOK := piRPCString(block, "text")
	return typeOK && textOK && blockType == "text" &&
		bytes.Equal([]byte(text), expectedPrompt)
}

func piRPCAgentMessages(
	raw json.RawMessage,
	expectedPrompt []byte,
	expectedAssistant json.RawMessage,
) bool {
	var messages []json.RawMessage
	if json.Unmarshal(raw, &messages) != nil ||
		len(messages) != 2 {
		return false
	}
	return piRPCUserMessage(messages[0], expectedPrompt) &&
		piRPCSemanticEqual(messages[1], expectedAssistant)
}

func piRPCSemanticEqual(first json.RawMessage, second json.RawMessage) bool {
	if len(first) == 0 || len(second) == 0 {
		return false
	}
	if rejectPiRPCDuplicateKeys(first) != nil ||
		rejectPiRPCDuplicateKeys(second) != nil {
		return false
	}
	var firstValue any
	var secondValue any
	if json.Unmarshal(first, &firstValue) != nil ||
		json.Unmarshal(second, &secondValue) != nil {
		return false
	}
	return reflect.DeepEqual(firstValue, secondValue)
}

func piRPCAssistantTextWitness(
	message json.RawMessage,
	messageText string,
	partial json.RawMessage,
	partialText string,
	allowUsageProjection bool,
) (string, bool) {
	var messageValue map[string]any
	var partialValue map[string]any
	if json.Unmarshal(message, &messageValue) != nil ||
		json.Unmarshal(partial, &partialValue) != nil ||
		!piRPCClearAssistantText(messageValue) ||
		!piRPCClearAssistantText(partialValue) {
		return "", false
	}
	if !reflect.DeepEqual(messageValue, partialValue) &&
		(!allowUsageProjection ||
			!piRPCAssistantUsageProjectsForward(
				messageValue,
				partialValue,
			)) {
		return "", false
	}
	switch {
	case bytes.HasPrefix([]byte(messageText), []byte(partialText)):
		return messageText, true
	case bytes.HasPrefix([]byte(partialText), []byte(messageText)):
		return partialText, true
	default:
		return "", false
	}
}

func piRPCClearAssistantText(value map[string]any) bool {
	content, ok := value["content"].([]any)
	if !ok || len(content) != 1 {
		return false
	}
	block, ok := content[0].(map[string]any)
	if !ok {
		return false
	}
	if _, ok := block["text"].(string); !ok {
		return false
	}
	block["text"] = ""
	return true
}

func piRPCAssistantUsageProjectsForward(
	message map[string]any,
	partial map[string]any,
) bool {
	messageUsage, messageOK := message["usage"].(map[string]any)
	partialUsage, partialOK := partial["usage"].(map[string]any)
	if !messageOK ||
		!partialOK ||
		!piRPCUsageMapProjectsForward(messageUsage, partialUsage) {
		return false
	}
	delete(message, "usage")
	delete(partial, "usage")
	return reflect.DeepEqual(message, partial)
}

func piRPCUsageMapProjectsForward(
	message map[string]any,
	partial map[string]any,
) bool {
	if _, present := message["cacheWrite1h"]; present {
		return false
	}
	if _, present := partial["cacheWrite1h"]; present {
		return false
	}
	for _, key := range []string{
		"input",
		"output",
		"cacheRead",
		"cacheWrite",
		"totalTokens",
	} {
		if !piRPCNumericFieldProjectsForward(message, partial, key) {
			return false
		}
	}
	messageCost, messageOK := message["cost"].(map[string]any)
	partialCost, partialOK := partial["cost"].(map[string]any)
	if !messageOK || !partialOK {
		return false
	}
	for _, key := range []string{
		"input",
		"output",
		"cacheRead",
		"cacheWrite",
		"total",
	} {
		if !piRPCNumericFieldProjectsForward(
			messageCost,
			partialCost,
			key,
		) {
			return false
		}
	}
	messageReasoning, messageReasoningPresent := message["reasoning"]
	partialReasoning, partialReasoningPresent := partial["reasoning"]
	if messageReasoningPresent && !partialReasoningPresent {
		return false
	}
	if !messageReasoningPresent {
		return true
	}
	messageNumber, messageOK := messageReasoning.(float64)
	partialNumber, partialOK := partialReasoning.(float64)
	return messageOK && partialOK && messageNumber <= partialNumber
}

func piRPCNumericFieldProjectsForward(
	message map[string]any,
	partial map[string]any,
	key string,
) bool {
	messageNumber, messageOK := message[key].(float64)
	partialNumber, partialOK := partial[key].(float64)
	return messageOK && partialOK && messageNumber <= partialNumber
}

func piRPCAssistantIdentitySnapshotOf(
	raw json.RawMessage,
) (piRPCAssistantIdentitySnapshot, bool) {
	var result piRPCAssistantIdentitySnapshot
	fields, err := piRPCObject(raw)
	if err != nil {
		return result, false
	}
	api, apiOK := piRPCString(fields, "api")
	provider, providerOK := piRPCString(fields, "provider")
	model, modelOK := piRPCString(fields, "model")
	if !apiOK || !providerOK || !modelOK ||
		api != "openai-completions" ||
		provider != piRPCProviderID ||
		model != piRPCModelID ||
		!piRPCUsage(fields["usage"]) {
		return result, false
	}
	if !piRPCNonNegativeNumber(fields["timestamp"]) {
		return result, false
	}
	result.timestamp = string(fields["timestamp"])
	if _, present := fields["responseId"]; present {
		value, ok := piRPCString(fields, "responseId")
		if !ok || value == "" || len(value) > 512 {
			return result, false
		}
		result.responseID = value
		result.responsePresent = true
	}
	if _, present := fields["responseModel"]; present {
		if !piRPCBoundedString(fields, "responseModel", 512) {
			return result, false
		}
		result.responseModel = true
	}
	usage, err := piRPCObject(fields["usage"])
	if err != nil {
		return result, false
	}
	_, result.cacheWrite1h = usage["cacheWrite1h"]
	_, result.reasoning = usage["reasoning"]
	return result, true
}

func piRPCInitialAssistantIdentity(
	raw json.RawMessage,
) (piRPCAssistantIdentityState, bool) {
	var result piRPCAssistantIdentityState
	snapshot, ok := piRPCAssistantIdentitySnapshotOf(raw)
	if !ok ||
		snapshot.responsePresent ||
		snapshot.responseModel ||
		snapshot.cacheWrite1h ||
		snapshot.reasoning {
		return result, false
	}
	result.timestamp = snapshot.timestamp
	return result, true
}

func (state *piRPCAssistantIdentityState) acceptUpdate(
	raw json.RawMessage,
	bindResponseID bool,
) bool {
	snapshot, ok := piRPCAssistantIdentitySnapshotOf(raw)
	if !ok ||
		snapshot.timestamp != state.timestamp ||
		snapshot.responseModel ||
		snapshot.cacheWrite1h {
		return false
	}
	if !state.responseBound {
		if !bindResponseID || !snapshot.responsePresent {
			return false
		}
		state.responseID = snapshot.responseID
		state.responseBound = true
	} else if bindResponseID ||
		!snapshot.responsePresent ||
		snapshot.responseID != state.responseID {
		return false
	}
	if state.reasoningSeen && !snapshot.reasoning {
		return false
	}
	if snapshot.reasoning {
		state.reasoningSeen = true
	}
	return true
}

func (state *piRPCAssistantIdentityState) acceptTerminal(
	raw json.RawMessage,
) bool {
	snapshot, ok := piRPCAssistantIdentitySnapshotOf(raw)
	return ok &&
		state.responseBound &&
		snapshot.timestamp == state.timestamp &&
		snapshot.responsePresent &&
		snapshot.responseID == state.responseID &&
		!snapshot.responseModel &&
		!snapshot.cacheWrite1h &&
		snapshot.reasoning == state.reasoningSeen
}

func piRPCAssistantTextMessage(
	raw json.RawMessage,
	expectedText string,
	requireContent bool,
) bool {
	text, ok := piRPCAssistantText(raw, requireContent)
	return ok && text == expectedText
}

func piRPCAssistantText(
	raw json.RawMessage,
	requireContent bool,
) (string, bool) {
	if !utf8.Valid(raw) {
		return "", false
	}
	fields, err := piRPCObject(raw)
	if err != nil ||
		!piRPCAllowedKeys(
			fields,
			[]string{
				"role",
				"content",
				"api",
				"provider",
				"model",
				"usage",
				"stopReason",
				"timestamp",
			},
			"responseId",
			"responseModel",
		) {
		return "", false
	}
	role, ok := piRPCString(fields, "role")
	api, apiOK := piRPCString(fields, "api")
	provider, providerOK := piRPCString(fields, "provider")
	model, modelOK := piRPCString(fields, "model")
	stopReason, stopOK := piRPCString(fields, "stopReason")
	if !ok ||
		!apiOK ||
		!providerOK ||
		!modelOK ||
		!stopOK ||
		role != "assistant" ||
		api != "openai-completions" ||
		provider != piRPCProviderID ||
		model != piRPCModelID ||
		stopReason != "stop" ||
		!piRPCUsage(fields["usage"]) ||
		!piRPCNonNegativeNumber(fields["timestamp"]) {
		return "", false
	}
	for _, optional := range []string{"responseId", "responseModel"} {
		if _, present := fields[optional]; present &&
			!piRPCBoundedString(fields, optional, 512) {
			return "", false
		}
	}
	contentRaw, ok := fields["content"]
	var content []json.RawMessage
	if !ok ||
		json.Unmarshal(contentRaw, &content) != nil ||
		len(content) > 1 {
		return "", false
	}
	var text strings.Builder
	for _, blockRaw := range content {
		block, err := piRPCObject(blockRaw)
		if err != nil ||
			!piRPCAllowedKeys(
				block,
				[]string{"type", "text"},
				"textSignature",
			) {
			return "", false
		}
		blockType, typeOK := piRPCString(block, "type")
		blockText, textOK := piRPCString(block, "text")
		if !typeOK ||
			!textOK ||
			blockType != "text" ||
			!utf8.Valid(block["text"]) ||
			!utf8.ValidString(blockText) {
			return "", false
		}
		if _, present := block["textSignature"]; present &&
			!piRPCBoundedString(block, "textSignature", 8192) {
			return "", false
		}
		text.WriteString(blockText)
	}
	if requireContent && len(content) != 1 {
		return "", false
	}
	if !requireContent && len(content) != 0 {
		return "", false
	}
	return text.String(), true
}

func piRPCUsage(raw json.RawMessage) bool {
	fields, err := piRPCObject(raw)
	if err != nil ||
		!piRPCAllowedKeys(
			fields,
			[]string{
				"input",
				"output",
				"cacheRead",
				"cacheWrite",
				"totalTokens",
				"cost",
			},
			"cacheWrite1h",
			"reasoning",
		) {
		return false
	}
	for _, key := range []string{
		"input",
		"output",
		"cacheRead",
		"cacheWrite",
		"totalTokens",
	} {
		if !piRPCNonNegativeNumber(fields[key]) {
			return false
		}
	}
	for _, optional := range []string{"cacheWrite1h", "reasoning"} {
		if rawValue, present := fields[optional]; present &&
			!piRPCNonNegativeNumber(rawValue) {
			return false
		}
	}
	cost, err := piRPCObject(fields["cost"])
	if err != nil ||
		!piRPCExactKeys(
			cost,
			"input",
			"output",
			"cacheRead",
			"cacheWrite",
			"total",
		) {
		return false
	}
	for _, key := range []string{
		"input",
		"output",
		"cacheRead",
		"cacheWrite",
		"total",
	} {
		if !piRPCNonNegativeNumber(cost[key]) {
			return false
		}
	}
	return true
}

func piRPCFinalAccounting(raw json.RawMessage) (work.RunAccounting, error) {
	fields, err := piRPCObject(raw)
	if err != nil {
		return work.RunAccounting{}, err
	}
	usageFields, err := piRPCObject(fields["usage"])
	if err != nil {
		return work.RunAccounting{}, err
	}
	input, inputOK := piRPCNonNegativeInt64(usageFields["input"])
	output, outputOK := piRPCNonNegativeInt64(usageFields["output"])
	cacheRead, cacheReadOK := piRPCNonNegativeInt64(usageFields["cacheRead"])
	cacheWrite, cacheWriteOK := piRPCNonNegativeInt64(usageFields["cacheWrite"])
	total, totalOK := piRPCNonNegativeInt64(usageFields["totalTokens"])
	costFields, costErr := piRPCObject(usageFields["cost"])
	cost, costOK := piRPCMicrounits(costFields["total"])
	if !inputOK || !outputOK || !cacheReadOK || !cacheWriteOK || !totalOK ||
		costErr != nil || !costOK {
		return work.RunAccounting{}, ErrPiRPCProtocol
	}
	accounting := work.RunAccounting{
		UsageObserved:    true,
		InputTokens:      input,
		OutputTokens:     output,
		CacheReadTokens:  cacheRead,
		CacheWriteTokens: cacheWrite,
		TotalTokens:      total,
		CostObserved:     true,
		CostMicrounits:   cost,
		CostCurrency:     "USD",
		CostSource:       work.CostSourceHarnessReported,
	}
	if err := work.ValidateRunAccounting(accounting); err != nil {
		return work.RunAccounting{}, ErrPiRPCProtocol
	}
	return accounting, nil
}

func piRPCNonNegativeInt64(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	value, err := strconv.ParseInt(string(raw), 10, 64)
	return value, err == nil && value >= 0
}

func piRPCMicrounits(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	value, ok := new(big.Rat).SetString(string(raw))
	if !ok || value.Sign() < 0 {
		return 0, false
	}
	value.Mul(value, big.NewRat(1_000_000, 1))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(value.Num(), value.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(value.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, false
	}
	return quotient.Int64(), true
}

func piRPCDebugState(state piRPCState) string {
	if !state.contextMode {
		return fmt.Sprintf(
			"response=%t agent=%t turns=%d message=%t done=%t settled=%t",
			state.responseSeen,
			state.agentStarted,
			state.turnCount,
			state.assistantSeen,
			state.doneSeen,
			state.settled,
		)
	}
	return fmt.Sprintf(
		"response=%t agent=%t turns=%d message=%t done=%t settled=%t context_stage=%d record=%s role=%s event=%s keys=%s reject=%s",
		state.responseSeen,
		state.agentStarted,
		state.turnCount,
		state.assistantSeen,
		state.doneSeen,
		state.settled,
		state.contextStage,
		state.contextRecordType,
		state.contextRecordRole,
		state.contextRecordEvent,
		state.contextRecordKeys,
		state.contextRejectPoint,
	)
}

func piRPCExitCode(command *exec.Cmd) int {
	if command == nil || command.ProcessState == nil {
		return -1
	}
	return command.ProcessState.ExitCode()
}
