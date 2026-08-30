package harnessadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/work"
)

func (runner *claudeCodeProcessRunner) RunHarnessWithAgentInputs(
	ctx context.Context,
	request HarnessProcessRequest,
	secret []byte,
	inputs loomruntime.AgentInputSource,
) (HarnessProcessResult, error) {
	defer zeroHarnessBytes(request.Prompt)
	if runner == nil || ctx == nil || !runner.SupportsAgentInputs() ||
		nilHarnessInterface(inputs) || !validHarnessProcessRequest(request) ||
		len(secret) == 0 || len(secret) > 8192 {
		return HarnessProcessResult{}, ErrInvalidClaudeCodeAdapter
	}
	var result HarnessProcessResult
	err := runner.gateway.WithCredential(
		ctx,
		ClaudeCodeProviderID,
		request.ModelID,
		secret,
		attemptProviderToolPolicy(request.ContextMCP),
		func(lease AttemptGatewayLease) (resultErr error) {
			if !validAttemptGatewayLease(lease) {
				return ErrHarnessProcessUnavailable
			}
			systemPromptPath, cleanupPrompt, promptErr := materializeHarnessSystemPrompt(
				request.TempPath, request.SystemPrompt,
			)
			if promptErr != nil {
				return promptErr
			}
			defer func() { resultErr = errors.Join(resultErr, cleanupPrompt()) }()
			session, startErr := runner.sessions.StartSession(
				ctx,
				HarnessSessionRequest{
					ExecutablePath: request.ExecutablePath,
					Arguments: claudeCodeStreamArguments(
						request, systemPromptPath,
					),
					Environment: claudeCodeEnvironment(request, lease),
					Directory:   request.TempPath, MaxOutputBytes: request.MaxOutputBytes,
					Timeout: request.Timeout,
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
			candidate, runErr := runClaudeStreamJSON(ctx, session, request, inputs)
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

func claudeCodeStreamArguments(
	request HarnessProcessRequest,
	systemPromptPath string,
) []string {
	arguments := []string{
		"--print", "--bare", "--no-session-persistence",
		"--input-format", "stream-json", "--output-format", "stream-json",
		"--replay-user-messages", "--verbose",
		"--model", request.ModelID,
		"--system-prompt-file", systemPromptPath,
	}
	if request.ReasoningEffort != "" {
		arguments = append(arguments, "--effort", request.ReasoningEffort)
	}
	tools, mcpConfig := claudeCodeMCPConfiguration(request)
	return append(arguments,
		"--permission-mode", "dontAsk",
		"--tools", tools,
		"--allowedTools", tools,
		"--disallowedTools", claudeCodeDisallowedNativeTools,
		"--disable-slash-commands",
		"--strict-mcp-config",
		"--mcp-config", mcpConfig,
	)
}

func runClaudeStreamJSON(
	ctx context.Context,
	session HarnessStreamSession,
	request HarnessProcessRequest,
	inputs loomruntime.AgentInputSource,
) (HarnessProcessResult, error) {
	prompt := request.Prompt
	sessionID := ""
	var accounting *work.RunAccounting
	for sequence := 1; sequence <= 65; sequence++ {
		payload, err := marshalClaudeStreamUser(prompt)
		if err != nil {
			if sequence > 1 {
				zeroHarnessBytes(prompt)
			}
			return HarnessProcessResult{}, err
		}
		if err := session.WriteLine(ctx, payload); err != nil {
			zeroHarnessBytes(payload)
			if sequence > 1 {
				zeroHarnessBytes(prompt)
			}
			return HarnessProcessResult{}, ErrHarnessProcessUnavailable
		}
		zeroHarnessBytes(payload)
		content, roundAccounting, lockedSessionID, roundErr := readClaudeStreamRound(
			ctx, session, prompt, sessionID, sequence == 1, request.MaxOutputBytes,
		)
		if sequence > 1 {
			zeroHarnessBytes(prompt)
		}
		if roundErr != nil {
			return HarnessProcessResult{}, roundErr
		}
		sessionID = lockedSessionID
		accounting, err = combineHarnessAccounting(accounting, roundAccounting)
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

func marshalClaudeStreamUser(prompt []byte) ([]byte, error) {
	if len(prompt) == 0 || len(prompt) > maxHarnessPromptBytes ||
		!utf8.Valid(prompt) || bytes.IndexByte(prompt, 0) >= 0 {
		return nil, ErrHarnessProtocol
	}
	payload := []byte(`{"type":"user","message":{"role":"user","content":`)
	payload = appendHarnessJSONString(payload, prompt)
	payload = append(payload, "}}"...)
	return payload, nil
}

func readClaudeStreamRound(
	ctx context.Context,
	session HarnessStreamSession,
	prompt []byte,
	sessionID string,
	wantInitialize bool,
	maximum int,
) (string, *work.RunAccounting, string, error) {
	initialized := !wantInitialize
	seenUser := false
	assistantEvents := 0
	pendingToolUses := make(map[string]struct{})
	for count := 0; count < 4096; count++ {
		line, err := session.ReadLine(ctx)
		if err != nil {
			zeroHarnessBytes(line)
			return "", nil, "", errors.Join(ErrHarnessProcessUnavailable, err)
		}
		if len(line) == 0 || len(line) > maximum || !utf8.Valid(line) ||
			bytes.IndexByte(line, 0) >= 0 {
			zeroHarnessBytes(line)
			return "", nil, "", ErrHarnessProtocol
		}
		var event struct {
			Type      string          `json:"type"`
			Subtype   string          `json:"subtype"`
			SessionID string          `json:"session_id"`
			Model     string          `json:"model"`
			Version   string          `json:"claude_code_version"`
			Message   json.RawMessage `json:"message"`
			Result    string          `json:"result"`
		}
		if json.Unmarshal(line, &event) != nil ||
			!validHarnessProtocolID(event.SessionID) {
			zeroHarnessBytes(line)
			return "", nil, "", ErrHarnessProtocol
		}
		if sessionID == "" {
			sessionID = event.SessionID
		} else if event.SessionID != sessionID {
			zeroHarnessBytes(line)
			return "", nil, "", ErrHarnessProtocol
		}
		switch event.Type {
		case "system":
			if event.Subtype == "compact_boundary" {
				if !initialized || !seenUser {
					zeroHarnessBytes(line)
					return "", nil, "", ErrHarnessProtocol
				}
				break
			}
			if initialized || event.Subtype != "init" ||
				event.Model != ClaudeCodeModelID || event.Version != "2.1.196" ||
				seenUser || assistantEvents != 0 {
				zeroHarnessBytes(line)
				return "", nil, "", ErrHarnessProtocol
			}
			initialized = true
		case "user":
			if !initialized {
				zeroHarnessBytes(line)
				return "", nil, "", ErrHarnessProtocol
			}
			if !seenUser {
				role, content, decodeErr := decodeClaudeStreamTextMessage(
					event.Message, maximum,
				)
				if decodeErr != nil || role != "user" ||
					!bytes.Equal([]byte(content), prompt) {
					zeroHarnessBytes(line)
					return "", nil, "", ErrHarnessProtocol
				}
				seenUser = true
				break
			}
			if assistantEvents == 0 ||
				validateClaudeStreamToolResults(
					event.Message, pendingToolUses, maximum,
				) != nil {
				zeroHarnessBytes(line)
				return "", nil, "", ErrHarnessProtocol
			}
		case "assistant":
			if !initialized || !seenUser || validateClaudeStreamAssistant(
				event.Message, pendingToolUses, maximum,
			) != nil {
				zeroHarnessBytes(line)
				return "", nil, "", ErrHarnessProtocol
			}
			assistantEvents++
		case "result":
			if !initialized || !seenUser || assistantEvents == 0 ||
				len(pendingToolUses) != 0 {
				zeroHarnessBytes(line)
				return "", nil, "", ErrHarnessProtocol
			}
			result, decodeErr := decodeClaudeCodeResult(line, maximum)
			zeroHarnessBytes(line)
			if decodeErr != nil || result.Accounting == nil {
				return "", nil, "", ErrHarnessProtocol
			}
			return result.Content, result.Accounting, sessionID, nil
		default:
			zeroHarnessBytes(line)
			return "", nil, "", ErrHarnessProtocol
		}
		zeroHarnessBytes(line)
	}
	return "", nil, "", ErrHarnessProtocol
}

func decodeClaudeStreamTextMessage(
	payload json.RawMessage,
	maximum int,
) (string, string, error) {
	var message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(payload, &message) != nil || len(message.Content) == 0 {
		return "", "", ErrHarnessProtocol
	}
	var content string
	if json.Unmarshal(message.Content, &content) != nil {
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(message.Content, &blocks) != nil || len(blocks) == 0 {
			return "", "", ErrHarnessProtocol
		}
		var combined strings.Builder
		for _, block := range blocks {
			if block.Type != "text" || block.Text == "" {
				return "", "", ErrHarnessProtocol
			}
			combined.WriteString(block.Text)
			if combined.Len() > maximum {
				return "", "", ErrHarnessProtocol
			}
		}
		content = combined.String()
	}
	if content == "" || len(content) > maximum || !utf8.ValidString(content) ||
		strings.IndexByte(content, 0) >= 0 {
		return "", "", ErrHarnessProtocol
	}
	return message.Role, content, nil
}

func validateClaudeStreamAssistant(
	payload json.RawMessage,
	pendingToolUses map[string]struct{},
	maximum int,
) error {
	var message struct {
		Role    string `json:"role"`
		Content []struct {
			Type  string          `json:"type"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Text  string          `json:"text"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	}
	if json.Unmarshal(payload, &message) != nil || message.Role != "assistant" ||
		len(message.Content) == 0 || len(message.Content) > 128 {
		return ErrHarnessProtocol
	}
	for _, block := range message.Content {
		switch block.Type {
		case "text":
			if block.Text == "" || len(block.Text) > maximum ||
				!utf8.ValidString(block.Text) || strings.IndexByte(block.Text, 0) >= 0 {
				return ErrHarnessProtocol
			}
		case "tool_use":
			if !validHarnessProtocolID(block.ID) || !validHarnessProtocolID(block.Name) ||
				len(block.Input) == 0 || len(block.Input) > maximum {
				return ErrHarnessProtocol
			}
			if _, duplicate := pendingToolUses[block.ID]; duplicate {
				return ErrHarnessProtocol
			}
			pendingToolUses[block.ID] = struct{}{}
		case "thinking", "redacted_thinking":
			if len(payload) > maximum {
				return ErrHarnessProtocol
			}
		default:
			return ErrHarnessProtocol
		}
	}
	return nil
}

func validateClaudeStreamToolResults(
	payload json.RawMessage,
	pendingToolUses map[string]struct{},
	maximum int,
) error {
	var message struct {
		Role    string `json:"role"`
		Content []struct {
			Type      string          `json:"type"`
			ToolUseID string          `json:"tool_use_id"`
			Content   json.RawMessage `json:"content"`
		} `json:"content"`
	}
	if json.Unmarshal(payload, &message) != nil || message.Role != "user" ||
		len(message.Content) == 0 || len(message.Content) > 128 {
		return ErrHarnessProtocol
	}
	for _, block := range message.Content {
		if block.Type != "tool_result" || !validHarnessProtocolID(block.ToolUseID) ||
			len(block.Content) == 0 || len(block.Content) > maximum {
			return ErrHarnessProtocol
		}
		if _, present := pendingToolUses[block.ToolUseID]; !present {
			return ErrHarnessProtocol
		}
		delete(pendingToolUses, block.ToolUseID)
	}
	return nil
}

func combineHarnessAccounting(
	combined *work.RunAccounting,
	current *work.RunAccounting,
) (*work.RunAccounting, error) {
	if current == nil || work.ValidateRunAccounting(*current) != nil {
		return nil, ErrHarnessProtocol
	}
	if combined == nil {
		copy := *current
		return &copy, nil
	}
	result := *combined
	add := func(left, right int64) (int64, bool) {
		return left + right, right >= 0 && left <= math.MaxInt64-right
	}
	var ok bool
	if result.InputTokens, ok = add(result.InputTokens, current.InputTokens); !ok {
		return nil, ErrHarnessProtocol
	}
	if result.OutputTokens, ok = add(result.OutputTokens, current.OutputTokens); !ok {
		return nil, ErrHarnessProtocol
	}
	if result.CacheReadTokens, ok = add(result.CacheReadTokens, current.CacheReadTokens); !ok {
		return nil, ErrHarnessProtocol
	}
	if result.CacheWriteTokens, ok = add(result.CacheWriteTokens, current.CacheWriteTokens); !ok {
		return nil, ErrHarnessProtocol
	}
	if result.TotalTokens, ok = add(result.TotalTokens, current.TotalTokens); !ok {
		return nil, ErrHarnessProtocol
	}
	if result.CostMicrounits, ok = add(result.CostMicrounits, current.CostMicrounits); !ok {
		return nil, ErrHarnessProtocol
	}
	result.UsageObserved = result.UsageObserved || current.UsageObserved
	result.CostObserved = result.CostObserved || current.CostObserved
	if result.CostCurrency == "" {
		result.CostCurrency = current.CostCurrency
	} else if current.CostCurrency != result.CostCurrency {
		return nil, ErrHarnessProtocol
	}
	if result.CostSource == "" {
		result.CostSource = current.CostSource
	} else if current.CostSource != result.CostSource {
		return nil, ErrHarnessProtocol
	}
	if work.ValidateRunAccounting(result) != nil {
		return nil, ErrHarnessProtocol
	}
	return &result, nil
}
