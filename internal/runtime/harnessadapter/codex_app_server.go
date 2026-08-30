package harnessadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/work"
)

const (
	codexAppServerClientVersion    = "phase2d-v11"
	codexAppServerInterruptTimeout = 5 * time.Second
	codexDefaultModelAlias         = "codex-default"
)

var errCodexAppServerTurnInterrupted = errors.New("codex app-server turn interrupted")

func (runner *codexProcessRunner) RunHarnessWithAgentInputs(
	ctx context.Context,
	request HarnessProcessRequest,
	secret []byte,
	inputs loomruntime.AgentInputSource,
) (HarnessProcessResult, error) {
	defer zeroHarnessBytes(request.Prompt)
	if runner == nil || ctx == nil || !runner.SupportsAgentInputs() ||
		nilHarnessInterface(inputs) || !validCodexProcessRequest(request) ||
		len(secret) == 0 || len(secret) > 8192 {
		return HarnessProcessResult{}, ErrInvalidCodexAdapter
	}
	var result HarnessProcessResult
	err := runner.gateway.WithCredential(
		ctx,
		CodexProviderID,
		request.ModelID,
		secret,
		attemptProviderToolPolicy(request.ContextMCP),
		func(lease AttemptGatewayLease) (resultErr error) {
			if !validAttemptGatewayLease(lease) {
				return ErrHarnessProcessUnavailable
			}
			systemPromptPath, cleanupPrompt, promptErr := materializeHarnessSystemPrompt(
				request.TempPath,
				request.SystemPrompt,
			)
			if promptErr != nil {
				return promptErr
			}
			defer func() { resultErr = errors.Join(resultErr, cleanupPrompt()) }()
			session, startErr := runner.sessions.StartSession(
				ctx,
				HarnessSessionRequest{
					ExecutablePath: request.ExecutablePath,
					Arguments: codexAppServerArguments(
						request, lease, systemPromptPath,
					),
					Environment:    codexEnvironment(request, lease),
					Directory:      request.WorkspacePath,
					MaxOutputBytes: request.MaxOutputBytes,
					Timeout:        request.Timeout,
				},
			)
			if startErr != nil {
				return ErrHarnessProcessUnavailable
			}
			clean := false
			defer func() {
				if !clean {
					resultErr = errors.Join(resultErr, session.Abort())
				}
			}()
			candidate, runErr := runCodexAppServer(
				ctx, session, request, inputs,
			)
			if runErr != nil {
				return runErr
			}
			if closeErr := session.CloseInput(); closeErr != nil {
				return ErrHarnessProcessUnavailable
			}
			commandResult, waitErr := session.Wait(ctx)
			if waitErr != nil || commandResult.ExitCode != 0 ||
				len(commandResult.Stdout) != 0 ||
				len(commandResult.Stderr) > request.MaxOutputBytes {
				return ErrHarnessProcessUnavailable
			}
			candidate.Stderr = []byte{}
			result = candidate
			clean = true
			return nil
		},
	)
	if err != nil {
		return HarnessProcessResult{}, err
	}
	return result, nil
}

func codexAppServerArguments(
	request HarnessProcessRequest,
	lease AttemptGatewayLease,
	systemPromptPath string,
) []string {
	arguments := []string{
		"-c", `model_provider="loom_gateway"`,
		"-c", `model_providers.loom_gateway.name="Loom OpenAI Gateway"`,
		"-c", `model_providers.loom_gateway.base_url="` + lease.BaseURL + `/v1"`,
		"-c", `model_providers.loom_gateway.env_key="` + codexAttemptTokenEnv + `"`,
		"-c", `model_providers.loom_gateway.wire_api="responses"`,
		"-c", `model_providers.loom_gateway.request_max_retries=0`,
		"-c", `model_providers.loom_gateway.stream_max_retries=0`,
		"-c", `model_instructions_file=` + fmt.Sprintf("%q", systemPromptPath),
		"-c", `mcp_servers={}`,
		"-c", `features.shell_tool=false`,
		"-c", `features.unified_exec=false`,
		"-c", `features.apply_patch_freeform=false`,
		"-c", `features.tool_search=false`,
		"-c", `approval_policy="never"`,
		"-c", `sandbox_mode="read-only"`,
	}
	if request.ContextMCP.URL != "" {
		enabledTools, _ := json.Marshal(harnessMCPToolNames(request.ContextMCP))
		arguments = append(arguments,
			"-c", `mcp_servers.loom_context.url=`+fmt.Sprintf("%q", request.ContextMCP.URL),
			"-c", `mcp_servers.loom_context.bearer_token_env_var="`+harnessContextMCPTokenEnv+`"`,
			"-c", `mcp_servers.loom_context.enabled_tools=`+string(enabledTools),
		)
		for _, name := range harnessMCPToolNames(request.ContextMCP) {
			arguments = append(
				arguments, "-c",
				`mcp_servers.loom_context.tools.`+name+`.approval_mode="approve"`,
			)
		}
		arguments = append(arguments,
			"-c", `mcp_servers.loom_context.startup_timeout_sec=5`,
			"-c", `mcp_servers.loom_context.tool_timeout_sec=15`,
		)
	}
	if request.ReasoningEffort != "" {
		arguments = append(
			arguments,
			"-c", `model_reasoning_effort="`+request.ReasoningEffort+`"`,
		)
	}
	return append(arguments, "app-server")
}

func runCodexAppServer(
	ctx context.Context,
	session HarnessStreamSession,
	request HarnessProcessRequest,
	inputs loomruntime.AgentInputSource,
) (HarnessProcessResult, error) {
	if err := codexAppServerInitialize(ctx, session); err != nil {
		return HarnessProcessResult{}, err
	}
	threadID, resolvedModelID, err := codexAppServerStartThread(ctx, session, request)
	if err != nil {
		return HarnessProcessResult{}, err
	}
	request.ModelID = resolvedModelID
	prompt := request.Prompt
	var accounting *work.RunAccounting
	for sequence := 1; sequence <= 65; sequence++ {
		content, turnAccounting, turnErr := codexAppServerTurn(
			ctx, session, threadID, sequence, request, prompt,
		)
		if sequence > 1 {
			zeroHarnessBytes(prompt)
		}
		if turnErr != nil {
			return HarnessProcessResult{}, turnErr
		}
		accounting, err = combineHarnessAccounting(accounting, turnAccounting)
		if err != nil {
			return HarnessProcessResult{}, err
		}
		outputDigest := sha256.Sum256([]byte(content))
		batch, available, inputErr := inputs.NextAgentInput(
			ctx,
			loomruntime.AgentInputCheckpoint{
				OutputDigest: hex.EncodeToString(outputDigest[:]),
			},
		)
		if inputErr != nil {
			batch.Close()
			return HarnessProcessResult{}, ErrHarnessProtocol
		}
		if !available {
			batch.Close()
			return HarnessProcessResult{Content: content, Accounting: accounting}, nil
		}
		nextPrompt, renderErr := loomruntime.RenderAgentInput(&batch)
		batch.Close()
		if renderErr != nil {
			zeroHarnessBytes(nextPrompt)
			return HarnessProcessResult{}, ErrHarnessProtocol
		}
		prompt = nextPrompt
	}
	zeroHarnessBytes(prompt)
	return HarnessProcessResult{}, ErrHarnessProtocol
}

func codexAppServerInitialize(
	ctx context.Context,
	session HarnessStreamSession,
) error {
	request := struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params struct {
			ClientInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"clientInfo"`
			Capabilities struct {
				ExperimentalAPI bool `json:"experimentalApi"`
			} `json:"capabilities"`
		} `json:"params"`
	}{ID: "loom-initialize-v1", Method: "initialize"}
	request.Params.ClientInfo.Name = "loom"
	request.Params.ClientInfo.Version = codexAppServerClientVersion
	request.Params.Capabilities.ExperimentalAPI = true
	if err := writeHarnessJSONLine(ctx, session, request); err != nil {
		return err
	}
	response, err := readCodexAppServerResponse(ctx, session, request.ID, nil)
	if err != nil {
		return err
	}
	var result struct {
		UserAgent      string `json:"userAgent"`
		CodexHome      string `json:"codexHome"`
		PlatformFamily string `json:"platformFamily"`
		PlatformOS     string `json:"platformOs"`
	}
	if json.Unmarshal(response, &result) != nil ||
		!strings.Contains(result.UserAgent, "0.144.1") ||
		result.CodexHome == "" || result.PlatformFamily == "" || result.PlatformOS == "" {
		return ErrHarnessProtocol
	}
	return nil
}

func codexAppServerStartThread(
	ctx context.Context,
	session HarnessStreamSession,
	request HarnessProcessRequest,
) (string, string, error) {
	payload := struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params struct {
			Model          string `json:"model,omitempty"`
			CWD            string `json:"cwd"`
			ApprovalPolicy string `json:"approvalPolicy"`
			Ephemeral      bool   `json:"ephemeral"`
			Sandbox        string `json:"sandbox"`
		} `json:"params"`
	}{ID: "loom-thread-start-v1", Method: "thread/start"}
	if request.ModelID != codexDefaultModelAlias {
		payload.Params.Model = request.ModelID
	}
	payload.Params.CWD = request.WorkspacePath
	payload.Params.ApprovalPolicy = "never"
	payload.Params.Ephemeral = true
	payload.Params.Sandbox = "read-only"
	if err := writeHarnessJSONLine(ctx, session, payload); err != nil {
		return "", "", err
	}
	preStartedThreadID := ""
	response, err := readCodexAppServerResponse(
		ctx, session, payload.ID,
		func(method string, params json.RawMessage) error {
			if handled, lifecycleErr := acceptCodexAppServerLifecycleNotification(
				method, params, "",
			); handled {
				return lifecycleErr
			}
			if method != "thread/started" {
				return ErrHarnessProtocol
			}
			var notification struct {
				Thread struct {
					ID string `json:"id"`
				} `json:"thread"`
			}
			if json.Unmarshal(params, &notification) != nil ||
				!validHarnessProtocolID(notification.Thread.ID) ||
				preStartedThreadID != "" {
				return ErrHarnessProtocol
			}
			preStartedThreadID = notification.Thread.ID
			return nil
		},
	)
	if err != nil {
		return "", "", err
	}
	var result struct {
		Model  string `json:"model"`
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if json.Unmarshal(response, &result) != nil || !validHarnessProtocolID(result.Thread.ID) ||
		!validHarnessProtocolID(result.Model) ||
		request.ModelID != codexDefaultModelAlias && result.Model != request.ModelID ||
		preStartedThreadID != "" && preStartedThreadID != result.Thread.ID {
		return "", "", ErrHarnessProtocol
	}
	return result.Thread.ID, result.Model, nil
}

func codexAppServerTurn(
	ctx context.Context,
	session HarnessStreamSession,
	threadID string,
	sequence int,
	request HarnessProcessRequest,
	prompt []byte,
	options ...codexAppServerTurnOptions,
) (string, *work.RunAccounting, error) {
	requestID := fmt.Sprintf("loom-turn-start-%d", sequence)
	payload, err := marshalCodexTurnStart(
		requestID, threadID, request.ModelID, request.ReasoningEffort, prompt, options...,
	)
	if err != nil {
		return "", nil, err
	}
	if err := session.WriteLine(ctx, payload); err != nil {
		zeroHarnessBytes(payload)
		return "", nil, ErrHarnessProcessUnavailable
	}
	zeroHarnessBytes(payload)
	preStartedTurnID := ""
	acceptStartNotification := func(method string, params json.RawMessage) error {
		if handled, lifecycleErr := acceptCodexAppServerLifecycleNotification(
			method, params, threadID,
		); handled {
			return lifecycleErr
		}
		return acceptCodexTurnStartNotification(
			method, params, threadID, &preStartedTurnID,
		)
	}
	response, err := readCodexAppServerResponse(
		ctx, session, requestID, acceptStartNotification,
	)
	if err != nil {
		if ctx.Err() == nil || errors.Is(err, ErrHarnessProtocol) {
			return "", nil, err
		}
		cleanupCtx, cancelCleanup := context.WithTimeout(
			context.Background(), codexAppServerInterruptTimeout,
		)
		defer cancelCleanup()
		response, err = readCodexAppServerResponse(
			cleanupCtx, session, requestID, acceptStartNotification,
		)
		if err != nil {
			return "", nil, errors.Join(ctx.Err(), err)
		}
		turnID, decodeErr := decodeCodexStartedTurn(response, preStartedTurnID)
		zeroHarnessBytes(response)
		if decodeErr != nil {
			return "", nil, errors.Join(ctx.Err(), decodeErr)
		}
		if interruptErr := interruptCodexAppServerTurnWithContext(
			cleanupCtx, session, threadID, turnID, sequence,
		); interruptErr != nil {
			return "", nil, errors.Join(ctx.Err(), interruptErr)
		}
		return "", nil, errors.Join(errCodexAppServerTurnInterrupted, ctx.Err())
	}
	turnID, err := decodeCodexStartedTurn(response, preStartedTurnID)
	zeroHarnessBytes(response)
	if err != nil {
		return "", nil, err
	}
	content, accounting, readErr := readCodexAppServerTurn(
		ctx, session, threadID, turnID, request.MaxOutputBytes,
	)
	if readErr == nil {
		return content, accounting, nil
	}
	if ctx.Err() == nil || errors.Is(readErr, ErrHarnessProtocol) {
		return "", nil, readErr
	}
	if interruptErr := interruptCodexAppServerTurn(
		session, threadID, turnID, sequence,
	); interruptErr != nil {
		return "", nil, errors.Join(ctx.Err(), interruptErr)
	}
	return "", nil, errors.Join(errCodexAppServerTurnInterrupted, ctx.Err())
}

type codexAppServerTurnOptions struct {
	OutputSchema     json.RawMessage
	UntrustedContext []byte
}

func acceptCodexTurnStartNotification(
	method string,
	params json.RawMessage,
	threadID string,
	preStartedTurnID *string,
) error {
	if preStartedTurnID == nil {
		return ErrHarnessProtocol
	}
	switch method {
	case "thread/started":
		var notification struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		}
		if json.Unmarshal(params, &notification) != nil ||
			notification.Thread.ID != threadID {
			return ErrHarnessProtocol
		}
		return nil
	case "turn/started":
		var notification struct {
			ThreadID string `json:"threadId"`
			Turn     struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"turn"`
		}
		if json.Unmarshal(params, &notification) != nil ||
			notification.ThreadID != threadID ||
			!validHarnessProtocolID(notification.Turn.ID) ||
			notification.Turn.Status != "inProgress" || *preStartedTurnID != "" {
			return ErrHarnessProtocol
		}
		*preStartedTurnID = notification.Turn.ID
		return nil
	default:
		return ErrHarnessProtocol
	}
}

func decodeCodexStartedTurn(response json.RawMessage, preStartedTurnID string) (string, error) {
	var started struct {
		Turn struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"turn"`
	}
	if json.Unmarshal(response, &started) != nil ||
		!validHarnessProtocolID(started.Turn.ID) || started.Turn.Status != "inProgress" ||
		preStartedTurnID != "" && preStartedTurnID != started.Turn.ID {
		return "", ErrHarnessProtocol
	}
	return started.Turn.ID, nil
}

func marshalCodexTurnStart(
	requestID, threadID, modelID, effort string,
	prompt []byte,
	options ...codexAppServerTurnOptions,
) ([]byte, error) {
	if !validHarnessProtocolID(requestID) || !validHarnessProtocolID(threadID) ||
		!validHarnessProtocolID(modelID) || len(prompt) == 0 ||
		len(prompt) > maxHarnessPromptBytes ||
		!utf8.Valid(prompt) || bytes.IndexByte(prompt, 0) >= 0 || len(options) > 1 {
		return nil, ErrHarnessProtocol
	}
	option := codexAppServerTurnOptions{}
	if len(options) == 1 {
		option = options[0]
	}
	if !validCodexAppServerTurnOptions(option) {
		return nil, ErrHarnessProtocol
	}
	metadata, err := json.Marshal(struct {
		ID     string `json:"id"`
		Method string `json:"method"`
	}{ID: requestID, Method: "turn/start"})
	if err != nil || len(metadata) < 1 || metadata[len(metadata)-1] != '}' {
		return nil, ErrHarnessProtocol
	}
	payload := append([]byte(nil), metadata[:len(metadata)-1]...)
	payload = append(payload, `,"params":{"threadId":`...)
	payload = appendHarnessJSONString(payload, []byte(threadID))
	payload = append(payload, `,"input":[{"type":"text","text":`...)
	payload = appendHarnessJSONString(payload, prompt)
	payload = append(payload, `,"text_elements":[]}],"model":`...)
	payload = appendHarnessJSONString(payload, []byte(modelID))
	if effort != "" {
		payload = append(payload, `,"effort":`...)
		payload = appendHarnessJSONString(payload, []byte(effort))
	}
	if len(option.OutputSchema) != 0 {
		payload = append(payload, `,"outputSchema":`...)
		payload = append(payload, option.OutputSchema...)
	}
	if len(option.UntrustedContext) != 0 {
		payload = append(payload, `,"additionalContext":{"`...)
		payload = append(payload, codexControlResultContextID...)
		payload = append(payload, `":{"kind":"untrusted","value":`...)
		payload = appendHarnessJSONString(payload, option.UntrustedContext)
		payload = append(payload, "}}"...)
	}
	payload = append(payload, "}}"...)
	return payload, nil
}

func validCodexAppServerTurnOptions(option codexAppServerTurnOptions) bool {
	if len(option.OutputSchema) == 0 {
		return len(option.UntrustedContext) == 0
	}
	if len(option.OutputSchema) > codexControlOutputSchemaMaximumBytes ||
		!json.Valid(option.OutputSchema) || rejectHarnessDuplicateJSONKeys(option.OutputSchema) ||
		len(option.UntrustedContext) > maxHarnessPromptBytes ||
		!utf8.Valid(option.UntrustedContext) || bytes.IndexByte(option.UntrustedContext, 0) >= 0 {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(option.OutputSchema))
	var object map[string]any
	return decoder.Decode(&object) == nil && object != nil &&
		decoder.Decode(&struct{}{}) == io.EOF
}

func interruptCodexAppServerTurn(
	session HarnessStreamSession,
	threadID, turnID string,
	sequence int,
) error {
	if nilHarnessInterface(session) || !validHarnessProtocolID(threadID) ||
		!validHarnessProtocolID(turnID) || sequence < 1 {
		return ErrHarnessProtocol
	}
	ctx, cancel := context.WithTimeout(
		context.Background(), codexAppServerInterruptTimeout,
	)
	defer cancel()
	return interruptCodexAppServerTurnWithContext(
		ctx, session, threadID, turnID, sequence,
	)
}

func interruptCodexAppServerTurnWithContext(
	ctx context.Context,
	session HarnessStreamSession,
	threadID, turnID string,
	sequence int,
) error {
	if ctx == nil || nilHarnessInterface(session) || !validHarnessProtocolID(threadID) ||
		!validHarnessProtocolID(turnID) || sequence < 1 {
		return ErrHarnessProtocol
	}
	request := struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params struct {
			ThreadID string `json:"threadId"`
			TurnID   string `json:"turnId"`
		} `json:"params"`
	}{
		ID:     fmt.Sprintf("loom-turn-interrupt-%d", sequence),
		Method: "turn/interrupt",
	}
	request.Params.ThreadID = threadID
	request.Params.TurnID = turnID
	if err := writeHarnessJSONLine(ctx, session, request); err != nil {
		return err
	}
	completed := false
	result, err := readCodexAppServerResponse(
		ctx, session, request.ID,
		func(method string, params json.RawMessage) error {
			return acceptCodexInterruptNotification(
				method, params, threadID, turnID, &completed,
			)
		},
	)
	if err != nil {
		return err
	}
	defer zeroHarnessBytes(result)
	var resultObject map[string]json.RawMessage
	if json.Unmarshal(result, &resultObject) != nil || resultObject == nil {
		return ErrHarnessProtocol
	}
	if completed {
		return nil
	}
	return readCodexInterruptedCompletion(ctx, session, threadID, turnID)
}

func readCodexInterruptedCompletion(
	ctx context.Context,
	session HarnessStreamSession,
	threadID, turnID string,
) error {
	completed := false
	for count := 0; count < 4096; count++ {
		line, err := session.ReadLine(ctx)
		if err != nil {
			zeroHarnessBytes(line)
			return errors.Join(ErrHarnessProcessUnavailable, err)
		}
		var notification struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		decodeErr := json.Unmarshal(line, &notification)
		if decodeErr != nil || len(notification.ID) != 0 ||
			notification.Method == "" || len(notification.Params) == 0 {
			zeroHarnessBytes(line)
			return ErrHarnessProtocol
		}
		acceptErr := acceptCodexInterruptNotification(
			notification.Method, notification.Params, threadID, turnID, &completed,
		)
		zeroHarnessBytes(line)
		if acceptErr != nil {
			return ErrHarnessProtocol
		}
		if completed {
			return nil
		}
	}
	return ErrHarnessProtocol
}

func acceptCodexInterruptNotification(
	method string,
	params json.RawMessage,
	threadID, turnID string,
	completed *bool,
) error {
	if completed == nil {
		return ErrHarnessProtocol
	}
	if *completed {
		return ErrHarnessProtocol
	}
	if handled, lifecycleErr := acceptCodexAppServerLifecycleNotification(
		method, params, threadID,
	); handled {
		return lifecycleErr
	}
	var notification struct {
		ThreadID string `json:"threadId"`
		TurnID   string `json:"turnId"`
		Turn     struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"turn"`
	}
	if json.Unmarshal(params, &notification) != nil ||
		notification.ThreadID != threadID {
		return ErrHarnessProtocol
	}
	switch method {
	case "turn/completed":
		if notification.Turn.ID != turnID ||
			notification.Turn.Status != "interrupted" {
			return ErrHarnessProtocol
		}
		*completed = true
		return nil
	case "turn/started":
		if notification.Turn.ID != turnID ||
			notification.Turn.Status != "inProgress" {
			return ErrHarnessProtocol
		}
		return nil
	case "item/mcpToolCall/progress":
		return acceptCodexMCPToolCallProgressNotification(
			params, threadID, turnID,
		)
	case "item/started", "item/completed", "turn/diff/updated",
		"item/agentMessage/delta", "item/plan/delta",
		"item/reasoning/summaryPartAdded", "item/reasoning/summaryTextDelta",
		"item/reasoning/textDelta", "turn/plan/updated",
		"turn/moderationMetadata", "thread/tokenUsage/updated":
		if notification.TurnID != turnID {
			return ErrHarnessProtocol
		}
		return nil
	default:
		return ErrHarnessProtocol
	}
}

func acceptCodexMCPToolCallProgressNotification(
	params json.RawMessage,
	threadID string,
	turnID string,
) error {
	if len(params) == 0 || len(params) > 64<<10 ||
		!validHarnessProtocolID(threadID) || !validHarnessProtocolID(turnID) {
		return ErrHarnessProtocol
	}
	var notification struct {
		ThreadID string `json:"threadId"`
		TurnID   string `json:"turnId"`
		ItemID   string `json:"itemId"`
		Message  string `json:"message"`
	}
	if json.Unmarshal(params, &notification) != nil ||
		notification.ThreadID != threadID || notification.TurnID != turnID ||
		!validHarnessProtocolID(notification.ItemID) ||
		len(notification.Message) > 16<<10 || !utf8.ValidString(notification.Message) ||
		strings.IndexByte(notification.Message, 0) >= 0 {
		return ErrHarnessProtocol
	}
	return nil
}

func appendHarnessJSONString(destination, content []byte) []byte {
	const hexDigits = "0123456789abcdef"
	destination = append(destination, '"')
	for _, value := range content {
		switch value {
		case '"', '\\':
			destination = append(destination, '\\', value)
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
		default:
			if value < 0x20 {
				destination = append(
					destination, '\\', 'u', '0', '0',
					hexDigits[value>>4], hexDigits[value&0x0f],
				)
			} else {
				destination = append(destination, value)
			}
		}
	}
	return append(destination, '"')
}

func readCodexAppServerResponse(
	ctx context.Context,
	session HarnessStreamSession,
	requestID string,
	acceptNotification func(string, json.RawMessage) error,
) (json.RawMessage, error) {
	for count := 0; count < 4096; count++ {
		line, err := session.ReadLine(ctx)
		if err != nil {
			zeroHarnessBytes(line)
			return nil, errors.Join(ErrHarnessProcessUnavailable, err)
		}
		var envelope struct {
			ID     string          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		decodeErr := json.Unmarshal(line, &envelope)
		zeroHarnessBytes(line)
		if decodeErr != nil {
			return nil, ErrHarnessProtocol
		}
		if envelope.ID == "" && envelope.Method != "" &&
			len(envelope.Result) == 0 && len(envelope.Error) == 0 &&
			len(envelope.Params) != 0 {
			if acceptNotification == nil ||
				acceptNotification(envelope.Method, envelope.Params) != nil {
				return nil, ErrHarnessProtocol
			}
			continue
		}
		if envelope.ID != requestID || envelope.Method != "" ||
			len(envelope.Params) != 0 || len(envelope.Error) != 0 ||
			len(envelope.Result) == 0 {
			return nil, ErrHarnessProtocol
		}
		return append(json.RawMessage(nil), envelope.Result...), nil
	}
	return nil, ErrHarnessProtocol
}

func acceptCodexAppServerLifecycleNotification(
	method string,
	params json.RawMessage,
	threadID string,
) (bool, error) {
	switch method {
	case "configWarning":
		var notification struct {
			Summary string          `json:"summary"`
			Details json.RawMessage `json:"details"`
		}
		if json.Unmarshal(params, &notification) != nil || notification.Summary == "" ||
			len(notification.Summary) > 16*1024 || !utf8.ValidString(notification.Summary) ||
			strings.IndexByte(notification.Summary, 0) >= 0 || len(notification.Details) == 0 {
			return true, ErrHarnessProtocol
		}
		return true, nil
	case "thread/status/changed":
		if threadID == "" {
			return true, ErrHarnessProtocol
		}
		var notification struct {
			ThreadID string `json:"threadId"`
			Status   struct {
				Type        string    `json:"type"`
				ActiveFlags *[]string `json:"activeFlags"`
			} `json:"status"`
		}
		if json.Unmarshal(params, &notification) != nil ||
			notification.ThreadID != threadID {
			return true, ErrHarnessProtocol
		}
		switch notification.Status.Type {
		case "active":
			if notification.Status.ActiveFlags == nil ||
				len(*notification.Status.ActiveFlags) > 2 {
				return true, ErrHarnessProtocol
			}
			seen := map[string]bool{}
			for _, flag := range *notification.Status.ActiveFlags {
				if flag != "waitingOnApproval" && flag != "waitingOnUserInput" || seen[flag] {
					return true, ErrHarnessProtocol
				}
				seen[flag] = true
			}
		case "idle", "notLoaded", "systemError":
			if notification.Status.ActiveFlags != nil {
				return true, ErrHarnessProtocol
			}
		default:
			return true, ErrHarnessProtocol
		}
		return true, nil
	case "warning":
		var notification struct {
			ThreadID *string `json:"threadId"`
			Message  string  `json:"message"`
		}
		if json.Unmarshal(params, &notification) != nil || notification.Message == "" ||
			len(notification.Message) > 16*1024 || !utf8.ValidString(notification.Message) ||
			strings.IndexByte(notification.Message, 0) >= 0 ||
			notification.ThreadID != nil &&
				(threadID == "" || *notification.ThreadID != threadID) {
			return true, ErrHarnessProtocol
		}
		return true, nil
	case "account/rateLimits/updated":
		var notification struct {
			RateLimits json.RawMessage `json:"rateLimits"`
		}
		if len(params) > 64*1024 || json.Unmarshal(params, &notification) != nil {
			return true, ErrHarnessProtocol
		}
		rateLimits := bytes.TrimSpace(notification.RateLimits)
		if len(rateLimits) < 2 || rateLimits[0] != '{' || rateLimits[len(rateLimits)-1] != '}' {
			return true, ErrHarnessProtocol
		}
		return true, nil
	case "remoteControl/status/changed":
		var notification struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(params, &notification) != nil ||
			!validHarnessProtocolID(notification.Status) {
			return true, ErrHarnessProtocol
		}
		return true, nil
	case "mcpServer/startupStatus/updated":
		var notification struct {
			Name          string  `json:"name"`
			Status        string  `json:"status"`
			ThreadID      *string `json:"threadId"`
			Error         *string `json:"error"`
			FailureReason *string `json:"failureReason"`
		}
		if len(params) > 64*1024 || json.Unmarshal(params, &notification) != nil ||
			!validHarnessProtocolID(notification.Name) ||
			notification.ThreadID != nil &&
				(threadID == "" || *notification.ThreadID != threadID) ||
			notification.Error != nil &&
				(len(*notification.Error) > 16*1024 || !utf8.ValidString(*notification.Error) ||
					strings.IndexByte(*notification.Error, 0) >= 0) ||
			notification.FailureReason != nil &&
				*notification.FailureReason != "reauthenticationRequired" {
			return true, ErrHarnessProtocol
		}
		switch notification.Status {
		case "starting", "ready", "failed", "cancelled":
			return true, nil
		default:
			return true, ErrHarnessProtocol
		}
	case "thread/started":
		if threadID == "" {
			return false, nil
		}
		var notification struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		}
		if json.Unmarshal(params, &notification) != nil || notification.Thread.ID != threadID {
			return true, ErrHarnessProtocol
		}
		return true, nil
	default:
		return false, nil
	}
}

func readCodexAppServerTurn(
	ctx context.Context,
	session HarnessStreamSession,
	threadID, turnID string,
	maximum int,
) (string, *work.RunAccounting, error) {
	content := ""
	var accounting *work.RunAccounting
	for count := 0; count < 4096; count++ {
		line, err := session.ReadLine(ctx)
		if err != nil {
			zeroHarnessBytes(line)
			return "", nil, errors.Join(ErrHarnessProcessUnavailable, err)
		}
		var notification struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				ThreadID string `json:"threadId"`
				TurnID   string `json:"turnId"`
				Item     struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"item"`
				Turn struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"turn"`
			} `json:"params"`
		}
		decodeErr := json.Unmarshal(line, &notification)
		var rawEnvelope struct {
			Params json.RawMessage `json:"params"`
		}
		rawDecodeErr := json.Unmarshal(line, &rawEnvelope)
		zeroHarnessBytes(line)
		if decodeErr != nil || rawDecodeErr != nil || len(notification.ID) != 0 ||
			notification.Method == "" || len(rawEnvelope.Params) == 0 {
			zeroHarnessBytes(rawEnvelope.Params)
			return "", nil, ErrHarnessProtocol
		}
		if notification.Method == "error" {
			failure := codexAppServerTurnFailure(rawEnvelope.Params, threadID, turnID)
			zeroHarnessBytes(rawEnvelope.Params)
			if failure == nil {
				continue
			}
			return "", nil, failure
		}
		if handled, lifecycleErr := acceptCodexAppServerLifecycleNotification(
			notification.Method, rawEnvelope.Params, threadID,
		); handled {
			zeroHarnessBytes(rawEnvelope.Params)
			if lifecycleErr != nil {
				return "", nil, lifecycleErr
			}
			continue
		}
		if notification.Method == "item/mcpToolCall/progress" {
			progressErr := acceptCodexMCPToolCallProgressNotification(
				rawEnvelope.Params, threadID, turnID,
			)
			zeroHarnessBytes(rawEnvelope.Params)
			if progressErr != nil {
				return "", nil, progressErr
			}
			continue
		}
		if notification.Method != "thread/tokenUsage/updated" {
			zeroHarnessBytes(rawEnvelope.Params)
		}
		switch notification.Method {
		case "item/completed":
			if notification.Params.ThreadID != threadID ||
				notification.Params.TurnID != turnID {
				return "", nil, ErrHarnessProtocol
			}
			if notification.Params.Item.Type == "agentMessage" {
				candidate := strings.TrimSpace(notification.Params.Item.Text)
				if candidate == "" || len(candidate) > maximum ||
					!utf8.ValidString(candidate) || strings.IndexByte(candidate, 0) >= 0 {
					return "", nil, ErrHarnessProtocol
				}
				content = candidate
			}
		case "turn/completed":
			if notification.Params.ThreadID != threadID ||
				notification.Params.Turn.ID != turnID ||
				notification.Params.Turn.Status != "completed" || content == "" ||
				accounting == nil {
				return "", nil, ErrHarnessProtocol
			}
			return content, accounting, nil
		case "turn/started":
			if notification.Params.ThreadID != threadID ||
				notification.Params.Turn.ID != turnID ||
				notification.Params.Turn.Status != "inProgress" {
				return "", nil, ErrHarnessProtocol
			}
		case "item/started", "turn/diff/updated", "item/agentMessage/delta",
			"item/plan/delta", "item/reasoning/summaryPartAdded",
			"item/reasoning/summaryTextDelta", "item/reasoning/textDelta",
			"turn/plan/updated", "turn/moderationMetadata":
			if notification.Params.ThreadID != threadID ||
				notification.Params.TurnID != turnID {
				return "", nil, ErrHarnessProtocol
			}
		case "thread/tokenUsage/updated":
			var usageNotification struct {
				ThreadID   string `json:"threadId"`
				TurnID     string `json:"turnId"`
				TokenUsage struct {
					Last struct {
						InputTokens           int64 `json:"inputTokens"`
						CachedInputTokens     int64 `json:"cachedInputTokens"`
						OutputTokens          int64 `json:"outputTokens"`
						ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
						TotalTokens           int64 `json:"totalTokens"`
					} `json:"last"`
				} `json:"tokenUsage"`
			}
			usageDecodeErr := json.Unmarshal(rawEnvelope.Params, &usageNotification)
			zeroHarnessBytes(rawEnvelope.Params)
			if usageDecodeErr != nil ||
				usageNotification.ThreadID != threadID ||
				usageNotification.TurnID != turnID {
				return "", nil, ErrHarnessProtocol
			}
			usage := usageNotification.TokenUsage.Last
			candidate := work.RunAccounting{
				UsageObserved: true, InputTokens: usage.InputTokens,
				OutputTokens:    usage.OutputTokens,
				CacheReadTokens: usage.CachedInputTokens,
				TotalTokens:     usage.TotalTokens,
			}
			if usage.ReasoningOutputTokens < 0 ||
				work.ValidateRunAccounting(candidate) != nil {
				return "", nil, ErrHarnessProtocol
			}
			accounting = &candidate
		default:
			return "", nil, ErrHarnessProtocol
		}
	}
	return "", nil, ErrHarnessProtocol
}

func codexAppServerTurnFailure(
	params json.RawMessage,
	threadID, turnID string,
) error {
	var notification struct {
		ThreadID  string `json:"threadId"`
		TurnID    string `json:"turnId"`
		WillRetry *bool  `json:"willRetry"`
		Error     struct {
			Message        json.RawMessage `json:"message"`
			CodexErrorInfo json.RawMessage `json:"codexErrorInfo"`
		} `json:"error"`
	}
	if json.Unmarshal(params, &notification) != nil ||
		notification.ThreadID != threadID || notification.TurnID != turnID ||
		notification.WillRetry == nil || len(notification.Error.Message) < 2 ||
		len(notification.Error.Message) > 16*1024 || !json.Valid(notification.Error.Message) ||
		notification.Error.Message[0] != '"' {
		zeroHarnessBytes(notification.Error.Message)
		zeroHarnessBytes(notification.Error.CodexErrorInfo)
		return ErrHarnessProtocol
	}
	defer zeroHarnessBytes(notification.Error.Message)
	defer zeroHarnessBytes(notification.Error.CodexErrorInfo)
	switch string(bytes.TrimSpace(notification.Error.CodexErrorInfo)) {
	case `"unauthorized"`:
		return ErrHarnessProviderAuth
	case `"usageLimitExceeded"`:
		return ErrHarnessProviderRateLimit
	case `"serverOverloaded"`, `"internalServerError"`:
		if *notification.WillRetry {
			return nil
		}
		return ErrHarnessProviderUnavailable
	default:
		if *notification.WillRetry {
			return nil
		}
		return ErrHarnessProtocol
	}
}

func writeHarnessJSONLine(
	ctx context.Context,
	session HarnessStreamSession,
	value any,
) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return ErrHarnessProtocol
	}
	defer zeroHarnessBytes(payload)
	if err := session.WriteLine(ctx, payload); err != nil {
		return ErrHarnessProcessUnavailable
	}
	return nil
}

func validHarnessProtocolID(value string) bool {
	return value != "" && len(value) <= 512 && utf8.ValidString(value) &&
		strings.IndexByte(value, 0) < 0 && strings.TrimSpace(value) == value
}
