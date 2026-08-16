package harnessadapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/work"
)

const codexAttemptTokenEnv = "LOOM_CODEX_ATTEMPT_TOKEN"

type CodexProcessRunnerConfig struct {
	Gateway  AttemptCredentialGateway
	Commands HarnessCommandRunner
	Sessions HarnessSessionRunner
}

type codexProcessRunner struct {
	gateway  AttemptCredentialGateway
	commands HarnessCommandRunner
	sessions HarnessSessionRunner
}

func NewCodexProcessRunner(
	config CodexProcessRunnerConfig,
) (HarnessProcessRunner, error) {
	if nilHarnessInterface(config.Gateway) || nilHarnessInterface(config.Commands) {
		return nil, ErrInvalidCodexAdapter
	}
	return &codexProcessRunner{
		gateway: config.Gateway, commands: config.Commands, sessions: config.Sessions,
	}, nil
}

func (runner *codexProcessRunner) SupportsAgentInputs() bool {
	return runner != nil && !nilHarnessInterface(runner.sessions)
}

func (runner *codexProcessRunner) RunHarness(
	ctx context.Context,
	request HarnessProcessRequest,
	secret []byte,
) (HarnessProcessResult, error) {
	defer zeroHarnessBytes(request.Prompt)
	if runner == nil || ctx == nil || !validCodexProcessRequest(request) ||
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
			defer func() {
				resultErr = errors.Join(resultErr, cleanupPrompt())
			}()
			commandResult, commandErr := runner.commands.RunCommand(
				ctx,
				HarnessCommandRequest{
					ExecutablePath: request.ExecutablePath,
					Arguments:      codexArguments(request, lease, systemPromptPath),
					Environment:    codexEnvironment(request, lease),
					Directory:      request.TempPath, Stdin: request.Prompt,
					MaxOutputBytes: request.MaxOutputBytes, Timeout: request.Timeout,
				},
			)
			if commandErr != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return ctxErr
				}
				return ErrHarnessProcessUnavailable
			}
			if commandResult.ExitCode != 0 || len(commandResult.Stdout) > request.MaxOutputBytes ||
				len(commandResult.Stderr) > request.MaxOutputBytes {
				return ErrHarnessProcessUnavailable
			}
			parsed, parseErr := decodeCodexResult(commandResult.Stdout, request.MaxOutputBytes)
			if parseErr != nil {
				return parseErr
			}
			parsed.Stderr = []byte{}
			result = parsed
			return nil
		},
	)
	if err != nil {
		return HarnessProcessResult{}, err
	}
	return result, nil
}

func validCodexProcessRequest(request HarnessProcessRequest) bool {
	return cleanHarnessAbsolutePath(request.ExecutablePath) &&
		cleanHarnessAbsolutePath(request.WorkspacePath) &&
		cleanHarnessAbsolutePath(request.HomePath) && cleanHarnessAbsolutePath(request.TempPath) &&
		request.ModelID == CodexModelID && len(request.Prompt) > 0 &&
		len(request.Prompt) <= maxHarnessPromptBytes && utf8.Valid(request.Prompt) &&
		bytes.IndexByte(request.Prompt, 0) < 0 && request.SystemPrompt != "" &&
		len(request.SystemPrompt) <= maxHarnessPromptBytes &&
		utf8.ValidString(request.SystemPrompt) &&
		strings.IndexByte(request.SystemPrompt, 0) < 0 && request.Timeout > 0 &&
		request.Timeout <= 15*time.Minute && request.MaxOutputBytes >= 256 &&
		request.MaxOutputBytes <= 1<<20 && validHarnessContextMCPLease(request.ContextMCP)
}

func codexArguments(
	request HarnessProcessRequest,
	lease AttemptGatewayLease,
	systemPromptPath string,
) []string {
	arguments := []string{
		"exec", "--ignore-user-config",
		"-c", `model_provider="loom_gateway"`,
		"-c", `model_providers.loom_gateway.name="Loom OpenAI Gateway"`,
		"-c", `model_providers.loom_gateway.base_url="` + lease.BaseURL + `/v1"`,
		"-c", `model_providers.loom_gateway.env_key="` + codexAttemptTokenEnv + `"`,
		"-c", `model_providers.loom_gateway.wire_api="responses"`,
		"-c", `model_providers.loom_gateway.request_max_retries=0`,
		"-c", `model_providers.loom_gateway.stream_max_retries=0`,
		"-c", `model_instructions_file=` + strconv.Quote(systemPromptPath),
		"-c", `mcp_servers={}`,
		"-c", `features.shell_tool=false`,
		"-c", `features.unified_exec=false`,
		"-c", `features.apply_patch_freeform=false`,
		"-c", `features.tool_search=false`,
		"-c", `approval_policy="never"`,
		"--strict-config", "--json", "--ephemeral", "--ignore-rules",
		"--skip-git-repo-check", "--color", "never",
		"--sandbox", "read-only", "--cd", request.TempPath,
		"--model", request.ModelID,
	}
	if request.ContextMCP.URL != "" {
		enabledTools, _ := json.Marshal(harnessMCPToolNames(request.ContextMCP))
		arguments = append(arguments,
			"-c", `mcp_servers.loom_context.url=`+strconv.Quote(request.ContextMCP.URL),
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
	return append(arguments, "-")
}

func codexEnvironment(
	request HarnessProcessRequest,
	lease AttemptGatewayLease,
) []string {
	environment := []string{
		"HOME=" + request.HomePath,
		"CODEX_HOME=" + request.HomePath,
		"TMPDIR=" + request.TempPath,
		"PATH=" + filepath.Dir(request.ExecutablePath) + ":/usr/bin:/bin",
		"LANG=C.UTF-8", "LC_ALL=C.UTF-8", "NO_COLOR=1",
		codexAttemptTokenEnv + "=" + lease.Token,
		"CODEX_MANAGED_BY_LOOM=1",
	}
	if request.ContextMCP.Token != "" {
		environment = append(environment, harnessContextMCPTokenEnv+"="+request.ContextMCP.Token)
	}
	return environment
}

func decodeCodexResult(payload []byte, maximum int) (HarnessProcessResult, error) {
	if len(payload) == 0 || len(payload) > maximum || !utf8.Valid(payload) ||
		bytes.IndexByte(payload, 0) >= 0 {
		return HarnessProcessResult{}, ErrHarnessProtocol
	}
	var (
		content       string
		completedTurn bool
		accounting    work.RunAccounting
	)
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 4096), maximum)
	for scanner.Scan() {
		line := scanner.Bytes()
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.UseNumber()
		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
			Usage struct {
				InputTokens           int64 `json:"input_tokens"`
				CachedInputTokens     int64 `json:"cached_input_tokens"`
				OutputTokens          int64 `json:"output_tokens"`
				ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
			} `json:"usage"`
		}
		if decoder.Decode(&event) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			return HarnessProcessResult{}, ErrHarnessProtocol
		}
		switch event.Type {
		case "item.completed":
			if event.Item.Type == "agent_message" {
				content = strings.TrimSpace(event.Item.Text)
			}
		case "turn.completed":
			if completedTurn {
				return HarnessProcessResult{}, ErrHarnessProtocol
			}
			if event.Usage.InputTokens < 0 || event.Usage.OutputTokens < 0 ||
				event.Usage.CachedInputTokens < 0 ||
				event.Usage.ReasoningOutputTokens < 0 {
				return HarnessProcessResult{}, ErrHarnessProtocol
			}
			completedTurn = true
			accounting = work.RunAccounting{
				UsageObserved: true, InputTokens: event.Usage.InputTokens,
				OutputTokens:    event.Usage.OutputTokens,
				CacheReadTokens: event.Usage.CachedInputTokens,
			}
		case "turn.failed", "error":
			return HarnessProcessResult{}, ErrHarnessProcessUnavailable
		}
	}
	if scanner.Err() != nil || !completedTurn || content == "" ||
		len(content) > maximum || accounting.InputTokens < 0 ||
		accounting.OutputTokens < 0 || accounting.CacheReadTokens < 0 ||
		accounting.CacheReadTokens > accounting.InputTokens ||
		accounting.InputTokens > math.MaxInt64-accounting.OutputTokens {
		return HarnessProcessResult{}, ErrHarnessProtocol
	}
	accounting.TotalTokens = accounting.InputTokens + accounting.OutputTokens
	if work.ValidateRunAccounting(accounting) != nil {
		return HarnessProcessResult{}, ErrHarnessProtocol
	}
	return HarnessProcessResult{Content: content, Accounting: &accounting}, nil
}
