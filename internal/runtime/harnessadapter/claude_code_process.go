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
	if nilHarnessInterface(config.Commands) {
		return nil, ErrInvalidClaudeCodeAdapter
	}
	return &claudeCodeProcessRunner{
		gateway: config.Gateway, commands: config.Commands, sessions: config.Sessions,
	}, nil
}

func (runner *claudeCodeProcessRunner) SupportsAgentInputs() bool {
	return runner != nil && !nilHarnessInterface(runner.gateway) &&
		!nilHarnessInterface(runner.sessions)
}

func (runner *claudeCodeProcessRunner) RunHarness(
	ctx context.Context,
	request HarnessProcessRequest,
	secret []byte,
) (HarnessProcessResult, error) {
	defer zeroHarnessBytes(request.Prompt)
	if runner == nil || ctx == nil || nilHarnessInterface(runner.commands) ||
		!validHarnessProcessRequest(request) {
		return HarnessProcessResult{}, ErrInvalidClaudeCodeAdapter
	}
	if !request.RequiresCredential {
		if len(secret) != 0 {
			return HarnessProcessResult{}, ErrInvalidClaudeCodeAdapter
		}
		return runner.runClaudeCodeCommand(
			ctx, request, claudeCodeNativeEnvironment(request),
		)
	}
	if nilHarnessInterface(runner.gateway) || len(secret) == 0 || len(secret) > 8192 {
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
			var runErr error
			result, runErr = runner.runClaudeCodeCommand(
				ctx, request, claudeCodeEnvironment(request, lease),
			)
			return runErr
		},
	)
	if err != nil {
		return HarnessProcessResult{}, err
	}
	return result, nil
}

func (runner *claudeCodeProcessRunner) runClaudeCodeCommand(
	ctx context.Context,
	request HarnessProcessRequest,
	environment []string,
) (result HarnessProcessResult, resultErr error) {
	systemPromptPath, cleanupPrompt, promptErr := materializeHarnessSystemPrompt(
		request.TempPath,
		request.SystemPrompt,
	)
	if promptErr != nil {
		return HarnessProcessResult{}, promptErr
	}
	defer func() {
		resultErr = errors.Join(resultErr, cleanupPrompt())
	}()
	commandResult, commandErr := runner.commands.RunCommand(
		ctx,
		HarnessCommandRequest{
			ExecutablePath: request.ExecutablePath,
			Arguments:      claudeCodeArguments(request, systemPromptPath),
			Environment:    environment,
			Directory:      request.TempPath,
			Stdin:          request.Prompt,
			MaxOutputBytes: request.MaxOutputBytes,
			Timeout:        request.Timeout,
		},
	)
	if commandErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return HarnessProcessResult{}, ctxErr
		}
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	if failure := closedClaudeCodeCommandFailure(commandResult); failure != nil {
		return HarnessProcessResult{}, failure
	}
	if commandResult.ExitCode != 0 ||
		len(commandResult.Stdout) > request.MaxOutputBytes ||
		len(commandResult.Stderr) > request.MaxOutputBytes {
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	parsed, parseErr := decodeClaudeCodeResult(
		commandResult.Stdout,
		request.MaxOutputBytes,
	)
	if parseErr != nil {
		return HarnessProcessResult{}, parseErr
	}
	if request.NativeSessionID != "" {
		resultSessionID, sessionErr := decodeClaudeCodeResultSessionID(
			commandResult.Stdout,
		)
		if sessionErr != nil || resultSessionID != request.NativeSessionID {
			return HarnessProcessResult{}, ErrHarnessProtocol
		}
	}
	// Raw CLI stderr can contain Provider or workspace content. The closed
	// operational diagnostic is emitted by the adapter instead.
	parsed.Stderr = []byte{}
	return parsed, nil
}

// ObserveClaudeCodeNativeAuth asks the attested CLI for a closed boolean
// status. Account metadata and raw CLI output are discarded in memory.
func ObserveClaudeCodeNativeAuth(
	ctx context.Context,
	executablePath string,
	homePath string,
	tempPath string,
	commands HarnessCommandRunner,
	timeout time.Duration,
) (ready bool, resultErr error) {
	if ctx == nil || nilHarnessInterface(commands) ||
		!cleanHarnessAbsolutePath(executablePath) ||
		!cleanHarnessAbsolutePath(homePath) ||
		!cleanHarnessAbsolutePath(tempPath) || timeout <= 0 || timeout > time.Minute {
		return false, ErrHarnessProcessUnavailable
	}
	result, err := commands.RunCommand(ctx, HarnessCommandRequest{
		ExecutablePath: executablePath,
		Arguments:      []string{"auth", "status", "--json"},
		Environment: claudeCodeBaseEnvironmentFor(
			executablePath, homePath, tempPath,
		),
		Directory: homePath, MaxOutputBytes: 4096, Timeout: timeout,
	})
	defer func() {
		zeroHarnessBytes(result.Stdout)
		zeroHarnessBytes(result.Stderr)
	}()
	if err != nil {
		return false, ErrHarnessProcessUnavailable
	}
	if len(result.Stdout) == 0 || len(result.Stdout) > 4096 ||
		!utf8.Valid(result.Stdout) || bytes.IndexByte(result.Stdout, 0) >= 0 {
		return false, ErrHarnessProtocol
	}
	decoder := json.NewDecoder(bytes.NewReader(result.Stdout))
	var status struct {
		LoggedIn *bool `json:"loggedIn"`
	}
	if decoder.Decode(&status) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		status.LoggedIn == nil {
		return false, ErrHarnessProtocol
	}
	if *status.LoggedIn {
		if result.ExitCode != 0 {
			return false, ErrHarnessProtocol
		}
		return true, nil
	}
	if result.ExitCode != 0 && result.ExitCode != 1 {
		return false, ErrHarnessProcessUnavailable
	}
	return false, nil
}

func closedClaudeCodeCommandFailure(result HarnessCommandResult) error {
	if len(result.Stdout) > 0 && len(result.Stdout) <= 1<<20 &&
		utf8.Valid(result.Stdout) && bytes.IndexByte(result.Stdout, 0) < 0 {
		decoder := json.NewDecoder(bytes.NewReader(result.Stdout))
		var decoded struct {
			IsError        bool   `json:"is_error"`
			Result         string `json:"result"`
			APIErrorStatus string `json:"api_error_status"`
		}
		if decoder.Decode(&decoded) == nil && decoder.Decode(&struct{}{}) == io.EOF &&
			decoded.IsError {
			if failure := closedClaudeCodeFailureText(
				decoded.Result + "\n" + decoded.APIErrorStatus,
			); failure != nil {
				return failure
			}
			return ErrHarnessProviderRejected
		}
	}
	if result.ExitCode != 0 {
		if failure := closedClaudeCodeFailureText(string(result.Stderr)); failure != nil {
			return failure
		}
	}
	return nil
}

func closedClaudeCodeFailureText(value string) error {
	value = strings.ToLower(value)
	switch {
	case strings.Contains(value, "not logged in"),
		strings.Contains(value, "run /login"),
		strings.Contains(value, "authentication"),
		strings.Contains(value, "unauthorized"),
		strings.Contains(value, "invalid api key"),
		strings.Contains(value, "oauth"):
		return ErrHarnessProviderAuth
	case strings.Contains(value, "rate limit"),
		strings.Contains(value, "too many requests"):
		return ErrHarnessProviderRateLimit
	case strings.Contains(value, "model not found"),
		strings.Contains(value, "invalid model"),
		strings.Contains(value, "model is not available"):
		return ErrHarnessProviderRejected
	default:
		return nil
	}
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
		request.MaxOutputBytes <= 1<<20 && validHarnessContextMCPLease(request.ContextMCP) &&
		validOptionalHarnessControlMCPLease(request.ControlMCP) &&
		validClaudeCodeNativeSessionRequest(request)
}

func validOptionalHarnessControlMCPLease(lease HarnessControlMCPLease) bool {
	if lease.URL == "" && lease.Token == "" && len(lease.ToolNames) == 0 {
		return true
	}
	return validHarnessControlMCPLease(lease)
}

func validClaudeCodeNativeSessionRequest(request HarnessProcessRequest) bool {
	if request.NativeSessionID == "" {
		return !request.ResumeNativeSession
	}
	return !request.RequiresCredential &&
		validClaudeCodeNativeSessionID(request.NativeSessionID)
}

func validClaudeCodeNativeSessionID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' ||
		value[18] != '-' || value[23] != '-' || value[14] != '4' ||
		!strings.ContainsRune("89ab", rune(value[19])) {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
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
	}
	if request.NativeSessionID == "" {
		arguments = append(arguments, "--no-session-persistence")
	} else if request.ResumeNativeSession {
		arguments = append(arguments, "--resume", request.NativeSessionID)
	} else {
		arguments = append(arguments, "--session-id", request.NativeSessionID)
	}
	arguments = append(arguments,
		"--output-format", "json",
		"--model", request.ModelID,
		"--system-prompt-file", systemPromptPath,
	)
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

func claudeCodeEnvironment(
	request HarnessProcessRequest,
	lease AttemptGatewayLease,
) []string {
	environment := claudeCodeBaseEnvironment(request)
	environment = append(environment,
		"ANTHROPIC_BASE_URL="+lease.BaseURL,
		"ANTHROPIC_API_KEY="+lease.Token,
		"CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST=1",
	)
	return appendClaudeCodeMCPEnvironment(environment, request)
}

func claudeCodeNativeEnvironment(request HarnessProcessRequest) []string {
	return appendClaudeCodeMCPEnvironment(claudeCodeBaseEnvironment(request), request)
}

func claudeCodeBaseEnvironment(request HarnessProcessRequest) []string {
	return claudeCodeBaseEnvironmentFor(
		request.ExecutablePath, request.HomePath, request.TempPath,
	)
}

func claudeCodeBaseEnvironmentFor(
	executablePath string,
	homePath string,
	tempPath string,
) []string {
	return []string{
		"HOME=" + homePath,
		"TMPDIR=" + tempPath,
		"PATH=" + filepath.Dir(executablePath) + ":/usr/bin:/bin",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"NO_COLOR=1",
		"CLAUDE_CODE_SIMPLE=1",
		"DISABLE_TELEMETRY=1",
		"DISABLE_ERROR_REPORTING=1",
		"DISABLE_AUTOUPDATER=1",
	}
}

func appendClaudeCodeMCPEnvironment(
	environment []string,
	request HarnessProcessRequest,
) []string {
	if request.ContextMCP.Token != "" {
		environment = append(
			environment, harnessContextMCPTokenEnv+"="+request.ContextMCP.Token,
		)
	}
	if request.ControlMCP.Token != "" {
		environment = append(
			environment, harnessControlMCPTokenEnv+"="+request.ControlMCP.Token,
		)
	}
	return environment
}

func claudeCodeMCPConfiguration(request HarnessProcessRequest) (string, string) {
	servers := make(map[string]any, 2)
	tools := make([]string, 0, len(harnessMCPToolNames(request.ContextMCP))+len(request.ControlMCP.ToolNames))
	if request.ContextMCP.URL != "" {
		for _, name := range harnessMCPToolNames(request.ContextMCP) {
			tools = append(tools, "mcp__loom_context__"+name)
		}
		servers["loom_context"] = map[string]any{
			"type": "http", "url": request.ContextMCP.URL,
			"headers": map[string]string{
				"Authorization": "Bearer ${" + harnessContextMCPTokenEnv + "}",
			},
		}
	}
	if request.ControlMCP.URL != "" {
		for _, name := range request.ControlMCP.ToolNames {
			tools = append(tools, "mcp__loom_control__"+name)
		}
		servers["loom_control"] = map[string]any{
			"type": "http", "url": request.ControlMCP.URL,
			"headers": map[string]string{
				"Authorization": "Bearer ${" + harnessControlMCPTokenEnv + "}",
			},
		}
	}
	encoded, err := json.Marshal(map[string]any{"mcpServers": servers})
	if err != nil {
		return "", `{"mcpServers":{}}`
	}
	return strings.Join(tools, ","), string(encoded)
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
	if !lease.ContextEnabled && !lease.ReadEnabled && !lease.GrepEnabled &&
		!lease.EditEnabled && !lease.BashEnabled &&
		!lease.WebSearchEnabled && !lease.WebFetchEnabled && !lease.MCPToolEnabled {
		return []string{"loom_read_context"}
	}
	names := make([]string, 0, 8)
	if lease.EditEnabled {
		names = append(names, "loom_edit_file")
	}
	if lease.GrepEnabled {
		names = append(names, "loom_grep_files")
	}
	if lease.MCPToolEnabled {
		names = append(names, "loom_mcp_call")
	}
	if lease.ContextEnabled {
		names = append(names, "loom_read_context")
	}
	if lease.ReadEnabled {
		names = append(names, "loom_read_file")
	}
	if lease.BashEnabled {
		names = append(names, "loom_run_command")
	}
	if lease.WebSearchEnabled {
		names = append(names, "loom_web_search")
	}
	if lease.WebFetchEnabled {
		names = append(names, "loom_web_fetch")
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

func decodeClaudeCodeResultSessionID(payload []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	var decoded struct {
		SessionID string `json:"session_id"`
	}
	if decoder.Decode(&decoded) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		!validClaudeCodeNativeSessionID(decoded.SessionID) {
		return "", ErrHarnessProtocol
	}
	return decoded.SessionID, nil
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
