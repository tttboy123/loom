package harnessadapter

import (
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

const claudeCodeDisallowedNativeTools = "Bash,Read,Edit,Write,Glob,Grep,WebFetch,WebSearch,Task,Skill,NotebookEdit,TodoWrite"

type AttemptGatewayLease struct {
	BaseURL string
	Token   string
}

type AttemptProviderToolPolicy struct {
	AllowedLoomTools []string
}

type AttemptCredentialGateway interface {
	WithCredential(
		context.Context,
		string,
		string,
		[]byte,
		AttemptProviderToolPolicy,
		func(AttemptGatewayLease) error,
	) error
}

type HarnessCommandRequest struct {
	ExecutablePath string
	Arguments      []string
	Environment    []string
	Directory      string
	Stdin          []byte
	MaxOutputBytes int
	Timeout        time.Duration
}

type HarnessCommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

type HarnessCommandRunner interface {
	RunCommand(context.Context, HarnessCommandRequest) (HarnessCommandResult, error)
}

type ClaudeCodeProcessRunnerConfig struct {
	Gateway  AttemptCredentialGateway
	Commands HarnessCommandRunner
	Sessions HarnessSessionRunner
}

type claudeCodeProcessRunner struct {
	gateway  AttemptCredentialGateway
	commands HarnessCommandRunner
	sessions HarnessSessionRunner
}

func NewClaudeCodeProcessRunner(
	config ClaudeCodeProcessRunnerConfig,
) (HarnessProcessRunner, error) {
	if nilHarnessInterface(config.Gateway) || nilHarnessInterface(config.Commands) {
		return nil, ErrInvalidClaudeCodeAdapter
	}
	return &claudeCodeProcessRunner{
		gateway: config.Gateway, commands: config.Commands, sessions: config.Sessions,
	}, nil
}

func (runner *claudeCodeProcessRunner) SupportsAgentInputs() bool {
	return runner != nil && !nilHarnessInterface(runner.sessions)
}

func (runner *claudeCodeProcessRunner) RunHarness(
	ctx context.Context,
	request HarnessProcessRequest,
	secret []byte,
) (HarnessProcessResult, error) {
	defer zeroHarnessBytes(request.Prompt)
	if runner == nil || ctx == nil || !validHarnessProcessRequest(request) ||
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
					Arguments:      claudeCodeArguments(request, systemPromptPath),
					Environment:    claudeCodeEnvironment(request, lease),
					Directory:      request.TempPath,
					Stdin:          request.Prompt,
					MaxOutputBytes: request.MaxOutputBytes,
					Timeout:        request.Timeout,
				},
			)
			if commandErr != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return ctxErr
				}
				return ErrHarnessProcessUnavailable
			}
			if commandResult.ExitCode != 0 ||
				len(commandResult.Stdout) > request.MaxOutputBytes ||
				len(commandResult.Stderr) > request.MaxOutputBytes {
				return ErrHarnessProcessUnavailable
			}
			parsed, parseErr := decodeClaudeCodeResult(
				commandResult.Stdout,
				request.MaxOutputBytes,
			)
			if parseErr != nil {
				return parseErr
			}
			// Raw CLI stderr can contain Provider or workspace content. The closed
			// operational diagnostic is emitted by the adapter instead.
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

func attemptProviderToolPolicy(lease HarnessContextMCPLease) AttemptProviderToolPolicy {
	return AttemptProviderToolPolicy{AllowedLoomTools: harnessMCPToolNames(lease)}
}

func validHarnessProcessRequest(request HarnessProcessRequest) bool {
	return cleanHarnessAbsolutePath(request.ExecutablePath) &&
		cleanHarnessAbsolutePath(request.WorkspacePath) &&
		cleanHarnessAbsolutePath(request.HomePath) &&
		cleanHarnessAbsolutePath(request.TempPath) &&
		request.ModelID == ClaudeCodeModelID && len(request.Prompt) > 0 &&
		len(request.Prompt) <= maxHarnessPromptBytes && utf8.Valid(request.Prompt) &&
		bytes.IndexByte(request.Prompt, 0) < 0 && request.SystemPrompt != "" &&
		len(request.SystemPrompt) <= maxHarnessPromptBytes &&
		utf8.ValidString(request.SystemPrompt) &&
		strings.IndexByte(request.SystemPrompt, 0) < 0 && request.Timeout > 0 &&
		request.Timeout <= 15*time.Minute && request.MaxOutputBytes >= 256 &&
		request.MaxOutputBytes <= 1<<20 && validHarnessContextMCPLease(request.ContextMCP)
}

func cleanHarnessAbsolutePath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path
}

func validAttemptGatewayLease(lease AttemptGatewayLease) bool {
	return strings.HasPrefix(lease.BaseURL, "http://127.0.0.1:") &&
		!strings.ContainsAny(lease.BaseURL, "\r\n\x00") && lease.Token != "" &&
		len(lease.Token) <= 256 && !strings.ContainsAny(lease.Token, "\r\n\x00")
}

func claudeCodeArguments(request HarnessProcessRequest, systemPromptPath string) []string {
	arguments := []string{
		"--print",
		"--bare",
		"--no-session-persistence",
		"--output-format", "json",
		"--model", request.ModelID,
		"--system-prompt-file", systemPromptPath,
	}
	if request.ReasoningEffort != "" {
		arguments = append(arguments, "--effort", request.ReasoningEffort)
	}
	tools := ""
	mcpConfig := `{"mcpServers":{}}`
	if request.ContextMCP.URL != "" {
		prefixedTools := make([]string, 0, 3)
		for _, name := range harnessMCPToolNames(request.ContextMCP) {
			prefixedTools = append(prefixedTools, "mcp__loom_context__"+name)
		}
		tools = strings.Join(prefixedTools, ",")
		encoded, err := json.Marshal(map[string]any{"mcpServers": map[string]any{
			"loom_context": map[string]any{
				"type": "http", "url": request.ContextMCP.URL,
				"headers": map[string]string{
					"Authorization": "Bearer ${" + harnessContextMCPTokenEnv + "}",
				},
			},
		}})
		if err == nil {
			mcpConfig = string(encoded)
		}
	}
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

func claudeCodeEnvironment(
	request HarnessProcessRequest,
	lease AttemptGatewayLease,
) []string {
	environment := []string{
		"HOME=" + request.HomePath,
		"TMPDIR=" + request.TempPath,
		"PATH=" + filepath.Dir(request.ExecutablePath) + ":/usr/bin:/bin",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"NO_COLOR=1",
		"ANTHROPIC_BASE_URL=" + lease.BaseURL,
		"ANTHROPIC_API_KEY=" + lease.Token,
		"CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST=1",
		"CLAUDE_CODE_SIMPLE=1",
		"DISABLE_TELEMETRY=1",
		"DISABLE_ERROR_REPORTING=1",
		"DISABLE_AUTOUPDATER=1",
	}
	if request.ContextMCP.Token != "" {
		environment = append(environment, harnessContextMCPTokenEnv+"="+request.ContextMCP.Token)
	}
	return environment
}

func validHarnessContextMCPLease(lease HarnessContextMCPLease) bool {
	if lease == (HarnessContextMCPLease{}) {
		return true
	}
	return strings.HasPrefix(lease.URL, "http://127.0.0.1:") &&
		strings.HasSuffix(lease.URL, "/mcp") &&
		!strings.ContainsAny(lease.URL, "\r\n\x00") &&
		len(lease.Token) == 64 && validHarnessDigest(lease.Token)
}

func harnessMCPToolNames(lease HarnessContextMCPLease) []string {
	if lease.URL == "" {
		return nil
	}
	if !lease.ContextEnabled && !lease.ReadEnabled && !lease.GrepEnabled {
		return []string{"loom_read_context"}
	}
	names := make([]string, 0, 3)
	if lease.GrepEnabled {
		names = append(names, "loom_grep_files")
	}
	if lease.ContextEnabled {
		names = append(names, "loom_read_context")
	}
	if lease.ReadEnabled {
		names = append(names, "loom_read_file")
	}
	return names
}

func decodeClaudeCodeResult(
	payload []byte,
	maximum int,
) (HarnessProcessResult, error) {
	if len(payload) == 0 || len(payload) > maximum || !utf8.Valid(payload) ||
		bytes.IndexByte(payload, 0) >= 0 {
		return HarnessProcessResult{}, ErrHarnessProtocol
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var decoded struct {
		Type         string      `json:"type"`
		Subtype      string      `json:"subtype"`
		IsError      bool        `json:"is_error"`
		Result       string      `json:"result"`
		TotalCostUSD json.Number `json:"total_cost_usd"`
		Usage        struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if decoder.Decode(&decoded) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		decoded.Type != "result" || decoded.Subtype != "success" || decoded.IsError {
		return HarnessProcessResult{}, ErrHarnessProtocol
	}
	content := strings.TrimSpace(decoded.Result)
	if content == "" || len(content) > maximum || !utf8.ValidString(content) ||
		strings.IndexByte(content, 0) >= 0 {
		return HarnessProcessResult{}, ErrHarnessProtocol
	}
	accounting := work.RunAccounting{
		UsageObserved: true,
		InputTokens:   decoded.Usage.InputTokens, OutputTokens: decoded.Usage.OutputTokens,
		CacheReadTokens:  decoded.Usage.CacheReadInputTokens,
		CacheWriteTokens: decoded.Usage.CacheCreationInputTokens,
	}
	if accounting.InputTokens < 0 || accounting.OutputTokens < 0 ||
		accounting.InputTokens > math.MaxInt64-accounting.OutputTokens {
		return HarnessProcessResult{}, ErrHarnessProtocol
	}
	accounting.TotalTokens = accounting.InputTokens + accounting.OutputTokens
	if decoded.TotalCostUSD != "" {
		cost, err := decimalUSDMicrounits(string(decoded.TotalCostUSD))
		if err != nil {
			return HarnessProcessResult{}, ErrHarnessProtocol
		}
		accounting.CostObserved = true
		accounting.CostMicrounits = cost
		accounting.CostCurrency = "USD"
		accounting.CostSource = work.CostSourceHarnessReported
	}
	if work.ValidateRunAccounting(accounting) != nil {
		return HarnessProcessResult{}, ErrHarnessProtocol
	}
	return HarnessProcessResult{Content: content, Accounting: &accounting}, nil
}

func decimalUSDMicrounits(value string) (int64, error) {
	if value == "" || strings.HasPrefix(value, "-") || strings.ContainsAny(value, "eE+") {
		return 0, ErrHarnessProtocol
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, ErrHarnessProtocol
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > (math.MaxInt64-999999)/1000000 {
		return 0, ErrHarnessProtocol
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 6 {
		return 0, ErrHarnessProtocol
	}
	for len(fraction) < 6 {
		fraction += "0"
	}
	fractionValue := int64(0)
	if fraction != "" {
		fractionValue, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, ErrHarnessProtocol
		}
	}
	return whole*1000000 + fractionValue, nil
}
