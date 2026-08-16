package harnessadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/work"
)

const codexAppServerClientVersion = "phase2d-v11"

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
					Directory:      request.TempPath,
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
	threadID, err := codexAppServerStartThread(ctx, session, request)
	if err != nil {
		return HarnessProcessResult{}, err
	}
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
) (string, error) {
	payload := struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params struct {
			Model          string `json:"model"`
			CWD            string `json:"cwd"`
			ApprovalPolicy string `json:"approvalPolicy"`
			Ephemeral      bool   `json:"ephemeral"`
			Sandbox        struct {
				Type          string   `json:"type"`
				WritableRoots []string `json:"writableRoots"`
				NetworkAccess bool     `json:"networkAccess"`
			} `json:"sandbox"`
		} `json:"params"`
	}{ID: "loom-thread-start-v1", Method: "thread/start"}
	payload.Params.Model = request.ModelID
	payload.Params.CWD = request.TempPath
	payload.Params.ApprovalPolicy = "never"
	payload.Params.Ephemeral = true
	payload.Params.Sandbox.Type = "readOnly"
	payload.Params.Sandbox.WritableRoots = nil
	if err := writeHarnessJSONLine(ctx, session, payload); err != nil {
		return "", err
	}
	preStartedThreadID := ""
	response, err := readCodexAppServerResponse(
		ctx, session, payload.ID,
		func(method string, params json.RawMessage) error {
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
		return "", err
	}
	var result struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if json.Unmarshal(response, &result) != nil || !validHarnessProtocolID(result.Thread.ID) ||
		preStartedThreadID != "" && preStartedThreadID != result.Thread.ID {
		return "", ErrHarnessProtocol
	}
	return result.Thread.ID, nil
}

func codexAppServerTurn(
	ctx context.Context,
	session HarnessStreamSession,
	threadID string,
	sequence int,
	request HarnessProcessRequest,
	prompt []byte,
) (string, *work.RunAccounting, error) {
	requestID := fmt.Sprintf("loom-turn-start-%d", sequence)
	payload, err := marshalCodexTurnStart(
		requestID, threadID, request.ModelID, request.ReasoningEffort, prompt,
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
	response, err := readCodexAppServerResponse(
		ctx, session, requestID,
		func(method string, params json.RawMessage) error {
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
					notification.Turn.Status != "inProgress" || preStartedTurnID != "" {
					return ErrHarnessProtocol
				}
				preStartedTurnID = notification.Turn.ID
				return nil
			default:
				return ErrHarnessProtocol
			}
		},
	)
	if err != nil {
		return "", nil, err
	}
	var started struct {
		Turn struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"turn"`
	}
	if json.Unmarshal(response, &started) != nil ||
		!validHarnessProtocolID(started.Turn.ID) || started.Turn.Status != "inProgress" ||
		preStartedTurnID != "" && preStartedTurnID != started.Turn.ID {
		return "", nil, ErrHarnessProtocol
	}
	return readCodexAppServerTurn(
		ctx, session, threadID, started.Turn.ID, request.MaxOutputBytes,
	)
}

func marshalCodexTurnStart(
	requestID, threadID, modelID, effort string,
	prompt []byte,
) ([]byte, error) {
	if !validHarnessProtocolID(requestID) || !validHarnessProtocolID(threadID) ||
		modelID != CodexModelID || len(prompt) == 0 || len(prompt) > maxHarnessPromptBytes ||
		!utf8.Valid(prompt) || bytes.IndexByte(prompt, 0) >= 0 {
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
	payload = append(payload, "}}"...)
	return payload, nil
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
	for count := 0; count < 64; count++ {
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
		case "turn/started", "item/started", "turn/diff/updated",
			"item/agentMessage/delta":
			if notification.Params.ThreadID != threadID {
				return "", nil, ErrHarnessProtocol
			}
			if notification.Method == "turn/started" &&
				(notification.Params.Turn.ID != turnID ||
					notification.Params.Turn.Status != "inProgress") {
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
