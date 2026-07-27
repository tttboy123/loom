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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

var (
	ErrInvalidPiRPCBridgeAdapter = errors.New("invalid Pi RPC bridge adapter")
	ErrPiRPCProtocol             = errors.New("Pi RPC protocol failed")
	ErrPiRPCOutputTooLarge       = errors.New("Pi RPC output too large")
	ErrPiRPCCleanup              = errors.New("Pi RPC cleanup failed")
)

const (
	piRPCProviderID      = "loom-local"
	piRPCModelID         = "qwen2.5-coder-1.5b-instruct-q4-k-m"
	piRPCMaxPromptBytes  = 8192
	piRPCMaxLineBytes    = 1 << 20
	piRPCMaxStdoutBytes  = 8 << 20
	piRPCMaxStderrBytes  = 256 << 10
	piRPCMaxRecords      = 1024
	piRPCMaxBridgeFrames = 1024
	piRPCMaxDeltaBytes   = 2048
	piRPCSystemPrompt    = "Answer only the supplied bounded task. Use no tools. Do not expose hidden reasoning, credentials, or filesystem paths. Return concise plain text."
	piRPCSettingsJSON    = `{"compaction":{"enabled":false},"retry":{"enabled":false,"maxRetries":0,"baseDelayMs":0,"provider":{"maxRetries":0,"maxRetryDelayMs":0}}}` + "\n"
)

type PiRPCBridgeAdapterConfig struct {
	Execution         PiExecutionAdapterConfig
	ProviderID        string
	ModelID           string
	BaseURL           string
	MaxAssistantBytes int
}

type piRPCBridgeAdapter struct {
	execution         *piExecutionAdapter
	providerID        string
	modelID           string
	baseURL           string
	maxAssistantBytes int
}

type piRPCLineResult struct {
	line []byte
	err  error
}

type piRPCState struct {
	responseSeen   bool
	agentStarted   bool
	turnOpen       bool
	turnCount      int
	messageOpen    bool
	messageRole    string
	userSeen       bool
	assistantSeen  bool
	assistantID    string
	finalAssistant json.RawMessage
	textStarted    bool
	textEnded      bool
	doneSeen       bool
	agentEnded     bool
	settled        bool
	assistant      []byte
	frames         []bridgev1.Frame
	nextSequence   int64
	recordCount    int
	prompt         string
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
		len(piRPCSystemPrompt) > 1024 {
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
	}, nil
}

func (*piRPCBridgeAdapter) AdapterType() string {
	return "pi-cli"
}

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
	if err := adapter.execution.validateRequest(request); err != nil {
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

	command := exec.Command(adapter.execution.executable.path, adapter.arguments()...)
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

	requestLine, err := json.Marshal(struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}{
		ID:      request.Dispatch.MessageID(),
		Type:    "prompt",
		Message: prompt,
	})
	if err != nil {
		return supervisor.AdapterResult{}, adapter.failRPC(command, wait, stdin, ErrPiRPCProtocol)
	}
	requestLine = append(requestLine, '\n')
	if _, err := stdin.Write(requestLine); err != nil {
		return supervisor.AdapterResult{}, adapter.failRPC(command, wait, stdin, ErrPiRPCProtocol)
	}

	state := piRPCState{nextSequence: 2, prompt: prompt}
	for !state.settled {
		select {
		case <-ctx.Done():
			return supervisor.AdapterResult{}, adapter.cancelRPC(
				command,
				wait,
				stdin,
				lines,
				request,
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
			if bytes.Contains(lineResult.line, []byte(request.Grant.Value())) {
				return supervisor.AdapterResult{}, adapter.failRPC(command, wait, stdin, ErrPiRPCProtocol)
			}
			if err := adapter.acceptRPCLine(ctx, request, &state, lineResult.line); err != nil {
				return supervisor.AdapterResult{}, adapter.failRPC(
					command,
					wait,
					stdin,
					errors.Join(err, errors.New(piRPCDebugState(state))),
				)
			}
		}
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
	if err := adapter.finishRPCFrames(ctx, request, &state); err != nil {
		return supervisor.AdapterResult{}, err
	}
	result, err := supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames:        state.frames,
		Stderr:               capturedStderr.content,
		ExitCode:             0,
		DispatchAcknowledged: true,
		ResultAcknowledged:   true,
	})
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiRPCProtocol, err)
	}
	return result, nil
}

func (adapter *piRPCBridgeAdapter) arguments() []string {
	return []string{
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
		"--provider", adapter.providerID,
		"--model", adapter.providerID + "/" + adapter.modelID,
		"--thinking", "off",
		"--system-prompt", piRPCSystemPrompt,
	}
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
	type cost struct {
		Input      int `json:"input"`
		Output     int `json:"output"`
		CacheRead  int `json:"cacheRead"`
		CacheWrite int `json:"cacheWrite"`
	}
	type model struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		Reasoning     bool     `json:"reasoning"`
		Input         []string `json:"input"`
		ContextWindow int      `json:"contextWindow"`
		MaxTokens     int      `json:"maxTokens"`
		Cost          cost     `json:"cost"`
	}
	type compatibility struct {
		SupportsDeveloperRole   bool `json:"supportsDeveloperRole"`
		SupportsReasoningEffort bool `json:"supportsReasoningEffort"`
	}
	type provider struct {
		BaseURL string        `json:"baseUrl"`
		API     string        `json:"api"`
		APIKey  string        `json:"apiKey"`
		Compat  compatibility `json:"compat"`
		Models  []model       `json:"models"`
	}
	value := struct {
		Providers map[string]provider `json:"providers"`
	}{
		Providers: map[string]provider{
			adapter.providerID: {
				BaseURL: adapter.baseURL,
				API:     "openai-completions",
				APIKey:  "loom-local-offline",
				Compat:  compatibility{},
				Models: []model{{
					ID:            adapter.modelID,
					Name:          "Loom Local Qwen 2.5 Coder 1.5B",
					Reasoning:     false,
					Input:         []string{"text"},
					ContextWindow: 4096,
					MaxTokens:     256,
					Cost:          cost{},
				}},
			},
		},
	}
	return json.Marshal(value)
}

func (adapter *piRPCBridgeAdapter) acceptRPCLine(
	ctx context.Context,
	request supervisor.AdapterRequest,
	state *piRPCState,
	line []byte,
) error {
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
			id != request.Dispatch.MessageID() ||
			command != "prompt" ||
			!success {
			return ErrPiRPCProtocol
		}
		frame, err := adapter.execution.outboundFrame(
			request,
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
			identity, ok := piRPCAssistantIdentity(fields["message"])
			if !ok {
				return ErrPiRPCProtocol
			}
			state.assistantID = identity
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
			return ErrPiRPCProtocol
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
		if !piRPCAssistantTextMessage(
			fields["message"],
			string(state.assistant),
			state.textStarted,
		) {
			return ErrPiRPCProtocol
		}
	case "message_end":
		if !state.messageOpen ||
			!piRPCExactKeys(fields, "type", "message") {
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
				!state.textEnded ||
				len(state.assistant) == 0 ||
				!piRPCAssistantTextMessage(
					fields["message"],
					string(state.assistant),
					true,
				) ||
				!piRPCAssistantIdentityMatches(
					state.assistantID,
					fields["message"],
				) {
				return ErrPiRPCProtocol
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
			!state.assistantSeen || !state.doneSeen ||
			!piRPCExactKeys(fields, "type", "message", "toolResults") ||
			!piRPCAssistantTextMessage(
				fields["message"],
				string(state.assistant),
				true,
			) ||
			!piRPCSemanticEqual(fields["message"], state.finalAssistant) ||
			!piRPCEmptyArray(fields["toolResults"]) {
			return ErrPiRPCProtocol
		}
		state.turnOpen = false
	case "agent_end":
		if !state.agentStarted || state.turnOpen || state.messageOpen ||
			!state.doneSeen || state.agentEnded ||
			!piRPCExactKeys(fields, "type", "messages", "willRetry") ||
			!piRPCAgentMessages(
				fields["messages"],
				state.prompt,
				state.finalAssistant,
			) {
			return ErrPiRPCProtocol
		}
		willRetry, ok := piRPCBool(fields, "willRetry")
		if !ok || willRetry {
			return ErrPiRPCProtocol
		}
		state.agentEnded = true
	case "agent_settled":
		if !state.agentEnded || state.settled || state.turnCount == 0 ||
			!state.textEnded || len(state.assistant) == 0 ||
			!piRPCExactKeys(fields, "type") {
			return ErrPiRPCProtocol
		}
		state.settled = true
	default:
		return ErrPiRPCProtocol
	}
	return nil
}

func (adapter *piRPCBridgeAdapter) acceptAssistantEvent(
	ctx context.Context,
	request supervisor.AdapterRequest,
	state *piRPCState,
	message json.RawMessage,
	raw json.RawMessage,
) error {
	fields, err := piRPCObject(raw)
	if err != nil {
		return err
	}
	eventType, ok := piRPCString(fields, "type")
	if !ok {
		return ErrPiRPCProtocol
	}
	switch eventType {
	case "text_start":
		if state.textStarted ||
			!piRPCExactKeys(fields, "type", "contentIndex", "partial") ||
			!piRPCZero(fields["contentIndex"]) ||
			!piRPCSemanticEqual(message, fields["partial"]) ||
			!piRPCAssistantTextMessage(fields["partial"], "", true) ||
			!piRPCAssistantIdentityMatches(
				state.assistantID,
				fields["partial"],
			) {
			return ErrPiRPCProtocol
		}
		state.textStarted = true
	case "text_delta":
		if !state.textStarted || state.textEnded || state.doneSeen ||
			!piRPCExactKeys(fields, "type", "contentIndex", "delta", "partial") ||
			!piRPCZero(fields["contentIndex"]) {
			return ErrPiRPCProtocol
		}
		delta, ok := piRPCString(fields, "delta")
		if !ok || delta == "" || !utf8.ValidString(delta) ||
			bytes.Contains([]byte(delta), []byte(request.Grant.Value())) ||
			len(state.assistant)+len(delta) > adapter.maxAssistantBytes {
			if len(state.assistant)+len(delta) > adapter.maxAssistantBytes {
				return ErrPiRPCOutputTooLarge
			}
			return ErrPiRPCProtocol
		}
		state.assistant = append(state.assistant, delta...)
		if !piRPCSemanticEqual(message, fields["partial"]) ||
			!piRPCAssistantTextMessage(
				fields["partial"],
				string(state.assistant),
				true,
			) ||
			!piRPCAssistantIdentityMatches(
				state.assistantID,
				fields["partial"],
			) {
			return ErrPiRPCProtocol
		}
		for _, chunk := range splitPiRPCDelta([]byte(delta)) {
			payload, err := json.Marshal(struct {
				Delta string `json:"delta"`
			}{Delta: string(chunk)})
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
			if len(state.frames) >= piRPCMaxBridgeFrames {
				return ErrPiRPCOutputTooLarge
			}
			if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
				return errors.Join(ErrPiRPCProtocol, err)
			}
			state.frames = append(state.frames, frame)
			state.nextSequence++
		}
	case "text_end":
		if !state.textStarted || state.textEnded || state.doneSeen ||
			!piRPCExactKeys(fields, "type", "contentIndex", "content", "partial") ||
			!piRPCZero(fields["contentIndex"]) {
			return ErrPiRPCProtocol
		}
		content, ok := piRPCString(fields, "content")
		if !ok ||
			content != string(state.assistant) ||
			!piRPCSemanticEqual(message, fields["partial"]) ||
			!piRPCAssistantTextMessage(fields["partial"], content, true) ||
			!piRPCAssistantIdentityMatches(
				state.assistantID,
				fields["partial"],
			) {
			return ErrPiRPCProtocol
		}
		state.textEnded = true
	default:
		return ErrPiRPCProtocol
	}
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
	request supervisor.AdapterRequest,
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
		info.Mode().Perm() != 0o700 {
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

func piRPCUserMessage(raw json.RawMessage, expectedPrompt string) bool {
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
	return typeOK &&
		textOK &&
		blockType == "text" &&
		text == expectedPrompt
}

func piRPCAgentMessages(
	raw json.RawMessage,
	expectedPrompt string,
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

func piRPCAssistantIdentity(raw json.RawMessage) (string, bool) {
	fields, err := piRPCObject(raw)
	if err != nil {
		return "", false
	}
	api, apiOK := piRPCString(fields, "api")
	provider, providerOK := piRPCString(fields, "provider")
	model, modelOK := piRPCString(fields, "model")
	if !apiOK || !providerOK || !modelOK ||
		api != "openai-completions" ||
		provider != piRPCProviderID ||
		model != piRPCModelID ||
		!piRPCUsage(fields["usage"]) {
		return "", false
	}
	var timestamp float64
	if json.Unmarshal(fields["timestamp"], &timestamp) != nil ||
		timestamp < 0 ||
		math.IsInf(timestamp, 0) ||
		math.IsNaN(timestamp) {
		return "", false
	}
	responseID := ""
	if _, present := fields["responseId"]; present {
		value, ok := piRPCString(fields, "responseId")
		if !ok || value == "" || len(value) > 512 {
			return "", false
		}
		responseID = value
	}
	responseModel := ""
	if _, present := fields["responseModel"]; present {
		value, ok := piRPCString(fields, "responseModel")
		if !ok || value != piRPCModelID {
			return "", false
		}
		responseModel = value
	}
	usage, err := piRPCObject(fields["usage"])
	if err != nil {
		return "", false
	}
	_, hasCacheWrite1h := usage["cacheWrite1h"]
	_, hasReasoning := usage["reasoning"]
	return fmt.Sprintf(
		"%s\x00%s\x00%s\x00%g\x00%s\x00%s\x00%t\x00%t",
		api,
		provider,
		model,
		timestamp,
		responseID,
		responseModel,
		hasCacheWrite1h,
		hasReasoning,
	), true
}

func piRPCAssistantIdentityMatches(
	expected string,
	raw json.RawMessage,
) bool {
	current, ok := piRPCAssistantIdentity(raw)
	return ok && current == expected
}

func piRPCAssistantTextMessage(
	raw json.RawMessage,
	expectedText string,
	requireContent bool,
) bool {
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
		return false
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
		return false
	}
	for _, optional := range []string{"responseId", "responseModel"} {
		if _, present := fields[optional]; present &&
			!piRPCBoundedString(fields, optional, 512) {
			return false
		}
	}
	contentRaw, ok := fields["content"]
	var content []json.RawMessage
	if !ok ||
		json.Unmarshal(contentRaw, &content) != nil ||
		len(content) > 1 {
		return false
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
			return false
		}
		blockType, typeOK := piRPCString(block, "type")
		blockText, textOK := piRPCString(block, "text")
		if !typeOK || !textOK || blockType != "text" {
			return false
		}
		if _, present := block["textSignature"]; present &&
			!piRPCBoundedString(block, "textSignature", 8192) {
			return false
		}
		text.WriteString(blockText)
	}
	if text.String() != expectedText {
		return false
	}
	if requireContent && len(content) != 1 {
		return false
	}
	if !requireContent && len(content) != 0 {
		return false
	}
	return true
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

func piRPCDebugState(state piRPCState) string {
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

func piRPCExitCode(command *exec.Cmd) int {
	if command == nil || command.ProcessState == nil {
		return -1
	}
	return command.ProcessState.ExitCode()
}
