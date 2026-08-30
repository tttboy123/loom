package harnessadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/work"
)

type CodexSegmentSessionConfig struct {
	ExecutablePath  string
	HomePath        string
	WorkspacePath   string
	PrivateRoot     string
	ModelID         string
	ReasoningEffort string
	SystemPrompt    string
	Timeout         time.Duration
	MaxOutputBytes  int
	Sessions        HarnessSessionRunner
	ControlRegistry *controltool.Registry
	ControlGateway  controltool.Gateway
}

type CodexSegmentSession struct {
	stream                 HarnessStreamSession
	threadID               string
	modelID                string
	reasoning              string
	maximum                int
	cleanupPrompt          func() error
	cleanupHome            func() error
	cancelLifetime         context.CancelFunc
	controlMCP             *harnessControlMCP
	controlSelectionSchema json.RawMessage
	controlFinalSchema     json.RawMessage
	mu                     sync.Mutex
	turnSequence           int
	closed                 bool
}

func OpenCodexSegmentSession(
	ctx context.Context,
	config CodexSegmentSessionConfig,
) (_ *CodexSegmentSession, resultErr error) {
	if ctx == nil || ctx.Err() != nil || nilHarnessInterface(config.Sessions) ||
		!cleanHarnessAbsolutePath(config.ExecutablePath) ||
		!cleanHarnessAbsolutePath(config.HomePath) ||
		!cleanHarnessAbsolutePath(config.WorkspacePath) ||
		!cleanHarnessAbsolutePath(config.PrivateRoot) ||
		!validHarnessProtocolID(config.ModelID) ||
		config.ReasoningEffort != "" && !validHarnessProtocolID(config.ReasoningEffort) ||
		config.SystemPrompt == "" || len(config.SystemPrompt) > maxHarnessPromptBytes ||
		!utf8.ValidString(config.SystemPrompt) ||
		strings.IndexByte(config.SystemPrompt, 0) >= 0 ||
		config.Timeout <= 0 || config.Timeout > 24*time.Hour ||
		config.MaxOutputBytes < 256 || config.MaxOutputBytes > 1<<20 ||
		(config.ControlRegistry == nil) != nilHarnessInterface(config.ControlGateway) {
		return nil, ErrInvalidCodexAdapter
	}
	systemPrompt := config.SystemPrompt
	if config.ControlRegistry != nil {
		var promptErr error
		systemPrompt, promptErr = appendCodexControlArbitrationSystemPrompt(systemPrompt)
		if promptErr != nil {
			return nil, promptErr
		}
	}
	systemPromptPath, cleanupPrompt, err := materializeHarnessSystemPrompt(
		config.PrivateRoot, systemPrompt,
	)
	if err != nil {
		return nil, err
	}
	cleaned := false
	defer func() {
		if !cleaned {
			resultErr = errors.Join(resultErr, cleanupPrompt())
		}
	}()
	codexHome, cleanupHome, err := prepareCodexNativeConversationHome(
		config.HomePath, config.PrivateRoot,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !cleaned {
			resultErr = errors.Join(resultErr, cleanupHome())
		}
	}()
	request := HarnessProcessRequest{
		ExecutablePath: config.ExecutablePath,
		WorkspacePath:  config.WorkspacePath,
		HomePath:       config.HomePath, TempPath: config.PrivateRoot,
		ModelID: config.ModelID, ReasoningEffort: config.ReasoningEffort,
		SystemPrompt: systemPrompt,
		Timeout:      config.Timeout, MaxOutputBytes: config.MaxOutputBytes,
	}
	lifetimeContext, cancelLifetime := context.WithCancel(context.Background())
	var controlMCP *harnessControlMCP
	if config.ControlRegistry != nil {
		controlMCP, err = newHarnessControlMCP(
			lifetimeContext, config.ControlRegistry, config.ControlGateway,
		)
		if err != nil {
			cancelLifetime()
			return nil, err
		}
	}
	defer func() {
		if !cleaned && controlMCP != nil {
			controlMCP.Close()
		}
	}()
	controlLease := HarnessControlMCPLease{}
	if controlMCP != nil {
		controlLease = controlMCP.Lease()
	}
	var controlSelectionSchema json.RawMessage
	var controlFinalSchema json.RawMessage
	if controlMCP != nil {
		controlSelectionSchema, err = codexControlSelectionOutputSchema(
			config.ControlRegistry, controlLease.ToolNames,
		)
		if err == nil {
			controlFinalSchema, err = codexControlFinalOutputSchema()
		}
		if err != nil {
			cancelLifetime()
			return nil, err
		}
	}
	stream, err := config.Sessions.StartSession(
		lifetimeContext,
		HarnessSessionRequest{
			ExecutablePath: config.ExecutablePath,
			Arguments: codexNativeConversationAppServerArguments(
				request, systemPromptPath, controlLease,
			),
			Environment: codexNativeConversationEnvironment(
				request, codexHome, controlLease,
			),
			Directory:      config.WorkspacePath,
			MaxOutputBytes: config.MaxOutputBytes, Timeout: config.Timeout,
		},
	)
	if err != nil || stream == nil {
		cancelLifetime()
		return nil, errors.Join(ErrHarnessProcessUnavailable, err)
	}
	abort := true
	defer func() {
		if abort {
			resultErr = errors.Join(resultErr, stream.Abort())
			cancelLifetime()
		}
	}()
	if err := codexAppServerInitialize(ctx, stream); err != nil {
		return nil, err
	}
	threadID, resolvedModelID, err := codexAppServerStartThread(ctx, stream, request)
	if err != nil {
		return nil, err
	}
	segment := &CodexSegmentSession{
		stream: stream, threadID: threadID, modelID: resolvedModelID,
		reasoning: config.ReasoningEffort, maximum: config.MaxOutputBytes,
		cleanupPrompt: cleanupPrompt, cleanupHome: cleanupHome,
		cancelLifetime: cancelLifetime, controlMCP: controlMCP,
		controlSelectionSchema: controlSelectionSchema,
		controlFinalSchema:     controlFinalSchema,
	}
	abort = false
	cleaned = true
	return segment, nil
}

func (session *CodexSegmentSession) BeginControlTurn(
	turn controltool.TurnContext,
) error {
	if session == nil || session.controlMCP == nil {
		return ErrHarnessControlMCP
	}
	return session.controlMCP.BeginTurn(turn)
}

func (session *CodexSegmentSession) EndControlTurn() (
	controltool.ProposalBatch,
	error,
) {
	if session == nil || session.controlMCP == nil {
		return controltool.ProposalBatch{}, ErrHarnessControlMCP
	}
	return session.controlMCP.EndTurn()
}

func (session *CodexSegmentSession) Respond(
	ctx context.Context,
	prompt []byte,
) (HarnessProcessResult, error) {
	defer zeroHarnessBytes(prompt)
	if session == nil || ctx == nil || ctx.Err() != nil || len(prompt) == 0 ||
		len(prompt) > maxHarnessPromptBytes || !utf8.Valid(prompt) ||
		bytes.IndexByte(prompt, 0) >= 0 {
		return HarnessProcessResult{}, ErrInvalidCodexAdapter
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed || session.stream == nil {
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	request := HarnessProcessRequest{
		ModelID: session.modelID, ReasoningEffort: session.reasoning,
		MaxOutputBytes: session.maximum,
	}
	options := []codexAppServerTurnOptions(nil)
	if session.controlMCP != nil {
		options = append(options, codexAppServerTurnOptions{
			OutputSchema: session.controlSelectionSchema,
		})
	}
	content, accounting, err := session.respondTurnLocked(ctx, request, prompt, options...)
	if err != nil {
		if errors.Is(err, errCodexAppServerTurnInterrupted) {
			return HarnessProcessResult{}, err
		}
		err = errors.Join(
			newCodexControlArbitrationFailure(codexControlFailureFirstTurn, err),
			session.failLocked(),
		)
		return HarnessProcessResult{}, err
	}
	if session.controlMCP == nil {
		return HarnessProcessResult{Content: content, Accounting: accounting}, nil
	}
	selection, err := decodeCodexControlSelection(
		content, session.controlMCP.Lease().ToolNames,
	)
	if err != nil {
		return HarnessProcessResult{}, errors.Join(
			newCodexControlArbitrationFailure(codexControlFailureSelectionDecode, err),
			session.failLocked(),
		)
	}
	defer zeroHarnessBytes(selection.Arguments)
	if selection.ToolName == codexControlNoTool {
		return HarnessProcessResult{Content: selection.Response, Accounting: accounting}, nil
	}
	alreadyCompleted, err := session.controlMCP.selectedToolAlreadyCompleted(selection.ToolName)
	if err != nil {
		return HarnessProcessResult{}, errors.Join(
			newCodexControlArbitrationFailure(codexControlFailureDirectMatch, err),
			session.failLocked(),
		)
	}
	if alreadyCompleted {
		return HarnessProcessResult{Content: selection.Response, Accounting: accounting}, nil
	}
	result, err := CallHarnessControlTool(
		ctx, session.controlMCP.Lease(), selection.ToolName, selection.Arguments,
	)
	if err != nil {
		zeroHarnessBytes(result)
		return HarnessProcessResult{}, errors.Join(
			newCodexControlArbitrationFailure(codexControlBrokerFailureStage(err), err),
			session.failLocked(),
		)
	}
	defer zeroHarnessBytes(result)
	if err := session.controlMCP.suspendTurnCalls(); err != nil {
		return HarnessProcessResult{}, errors.Join(
			newCodexControlArbitrationFailure(codexControlFailureBrokerSuspend, err),
			session.failLocked(),
		)
	}
	finalContent, finalAccounting, err := session.respondTurnLocked(
		ctx, request, []byte(codexControlResultFollowupPrompt),
		codexAppServerTurnOptions{
			OutputSchema:     session.controlFinalSchema,
			UntrustedContext: result,
		},
	)
	if err != nil {
		return HarnessProcessResult{}, errors.Join(
			newCodexControlArbitrationFailure(codexControlFailureResultSummary, err),
			session.failLocked(),
		)
	}
	accounting, err = combineHarnessAccounting(accounting, finalAccounting)
	if err != nil {
		return HarnessProcessResult{}, errors.Join(
			newCodexControlArbitrationFailure(codexControlFailureAccounting, err),
			session.failLocked(),
		)
	}
	finalResponse, err := decodeCodexControlFinal(finalContent)
	if err != nil {
		return HarnessProcessResult{}, errors.Join(
			newCodexControlArbitrationFailure(codexControlFailureResultDecode, err),
			session.failLocked(),
		)
	}
	return HarnessProcessResult{Content: finalResponse, Accounting: accounting}, nil
}

func (session *CodexSegmentSession) respondTurnLocked(
	ctx context.Context,
	request HarnessProcessRequest,
	prompt []byte,
	options ...codexAppServerTurnOptions,
) (string, *work.RunAccounting, error) {
	session.turnSequence++
	return codexAppServerTurn(
		ctx, session.stream, session.threadID, session.turnSequence, request, prompt,
		options...,
	)
}

func (session *CodexSegmentSession) Healthy() bool {
	if session == nil {
		return false
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return !session.closed && session.stream != nil
}

func (session *CodexSegmentSession) failLocked() error {
	session.closed = true
	var result error
	if session.stream != nil {
		result = session.stream.Abort()
		session.stream = nil
	}
	if session.cancelLifetime != nil {
		session.cancelLifetime()
		session.cancelLifetime = nil
	}
	if session.controlMCP != nil {
		session.controlMCP.Close()
		session.controlMCP = nil
	}
	if session.cleanupPrompt != nil {
		result = errors.Join(result, session.cleanupPrompt())
		session.cleanupPrompt = nil
	}
	if session.cleanupHome != nil {
		result = errors.Join(result, session.cleanupHome())
		session.cleanupHome = nil
	}
	return result
}

func (session *CodexSegmentSession) Close(ctx context.Context) error {
	if session == nil || ctx == nil {
		return ErrInvalidCodexAdapter
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return nil
	}
	session.closed = true
	var result error
	defer func() {
		if session.cancelLifetime != nil {
			session.cancelLifetime()
			session.cancelLifetime = nil
		}
	}()
	if session.stream != nil {
		if err := session.stream.CloseInput(); err != nil {
			result = errors.Join(result, err, session.stream.Abort())
		} else {
			command, err := session.stream.Wait(ctx)
			if err != nil || command.ExitCode != 0 || len(command.Stdout) != 0 ||
				len(command.Stderr) > session.maximum {
				result = errors.Join(result, ErrHarnessProcessUnavailable, err, session.stream.Abort())
			}
		}
	}
	if session.cleanupPrompt != nil {
		result = errors.Join(result, session.cleanupPrompt())
		session.cleanupPrompt = nil
	}
	if session.cleanupHome != nil {
		result = errors.Join(result, session.cleanupHome())
		session.cleanupHome = nil
	}
	if session.controlMCP != nil {
		session.controlMCP.Close()
		session.controlMCP = nil
	}
	return result
}

func codexNativeConversationAppServerArguments(
	request HarnessProcessRequest,
	systemPromptPath string,
	control ...HarnessControlMCPLease,
) []string {
	arguments := []string{
		"-c", `model_instructions_file=` + fmt.Sprintf("%q", systemPromptPath),
		"-c", `project_doc_max_bytes=0`,
		"-c", `mcp_servers={}`,
		"-c", `features.apps=false`,
		"-c", `features.plugins=false`,
		"-c", `features.remote_plugin=false`,
		"-c", `features.in_app_browser=false`,
		"-c", `features.shell_tool=false`,
		"-c", `features.unified_exec=false`,
		"-c", `features.apply_patch_freeform=false`,
		"-c", `features.tool_search=false`,
		"-c", `approval_policy="never"`,
		"-c", `sandbox_mode="read-only"`,
	}
	if len(control) == 1 && validHarnessControlMCPLease(control[0]) {
		enabledTools, _ := json.Marshal(control[0].ToolNames)
		arguments = append(arguments,
			"-c", `mcp_servers.loom_control.url=`+fmt.Sprintf("%q", control[0].URL),
			"-c", `mcp_servers.loom_control.bearer_token_env_var="`+harnessControlMCPTokenEnv+`"`,
			"-c", `mcp_servers.loom_control.enabled_tools=`+string(enabledTools),
		)
		for _, name := range control[0].ToolNames {
			arguments = append(
				arguments, "-c",
				`mcp_servers.loom_control.tools.`+name+`.approval_mode="approve"`,
			)
		}
		arguments = append(arguments,
			"-c", `mcp_servers.loom_control.startup_timeout_sec=5`,
			"-c", `mcp_servers.loom_control.tool_timeout_sec=15`,
		)
	}
	if request.ReasoningEffort != "" {
		arguments = append(
			arguments, "-c",
			`model_reasoning_effort="`+request.ReasoningEffort+`"`,
		)
	}
	return append(arguments, "app-server")
}

func codexNativeConversationEnvironment(
	request HarnessProcessRequest,
	codexHome string,
	control ...HarnessControlMCPLease,
) []string {
	environment := []string{
		"HOME=" + request.HomePath,
		"CODEX_HOME=" + codexHome,
		"TMPDIR=" + request.TempPath,
		"PATH=" + filepath.Dir(request.ExecutablePath) + ":/usr/bin:/bin",
		"LANG=C.UTF-8", "LC_ALL=C.UTF-8", "NO_COLOR=1",
		"CODEX_MANAGED_BY_LOOM=1",
	}
	if len(control) == 1 && validHarnessControlMCPLease(control[0]) {
		environment = append(
			environment, harnessControlMCPTokenEnv+"="+control[0].Token,
		)
	}
	return environment
}

func validHarnessControlMCPLease(lease HarnessControlMCPLease) bool {
	endpoint, err := url.Parse(lease.URL)
	if err != nil || !validHarnessControlLoopbackURL(endpoint) || len(lease.URL) > 256 ||
		len(lease.Token) != 64 || strings.IndexFunc(lease.Token, func(char rune) bool {
		return !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f')
	}) >= 0 || len(lease.ToolNames) == 0 || len(lease.ToolNames) > 64 {
		return false
	}
	seen := make(map[string]struct{}, len(lease.ToolNames))
	for _, name := range lease.ToolNames {
		if !validHarnessControlToolName(name) {
			return false
		}
		if _, duplicate := seen[name]; duplicate {
			return false
		}
		seen[name] = struct{}{}
	}
	return true
}

func validHarnessControlToolName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	// Tool names become exact CLI allow-list and configuration keys, so keep
	// their alphabet closed to prevent argument or configuration injection.
	return strings.IndexFunc(name, func(char rune) bool {
		return !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_')
	}) < 0
}

func prepareCodexNativeConversationHome(
	homePath string,
	privateRoot string,
) (string, func() error, error) {
	authPath := filepath.Join(homePath, ".codex", "auth.json")
	authInfo, err := os.Lstat(authPath)
	if err != nil || !authInfo.Mode().IsRegular() || authInfo.Mode().Perm()&0o077 != 0 {
		return "", nil, ErrHarnessProcessUnavailable
	}
	codexHome := filepath.Join(privateRoot, "codex-home")
	if err := os.Mkdir(codexHome, 0o700); err != nil {
		return "", nil, ErrHarnessProcessUnavailable
	}
	cleanup := func() error {
		info, statErr := os.Lstat(codexHome)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrHarnessProcessUnavailable
		}
		if err := os.RemoveAll(codexHome); err != nil {
			return ErrHarnessProcessUnavailable
		}
		return nil
	}
	if err := os.Symlink(authPath, filepath.Join(codexHome, "auth.json")); err != nil {
		_ = cleanup()
		return "", nil, ErrHarnessProcessUnavailable
	}
	config, err := os.OpenFile(
		filepath.Join(codexHome, "config.toml"),
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		_ = cleanup()
		return "", nil, ErrHarnessProcessUnavailable
	}
	if err := config.Sync(); err != nil {
		_ = config.Close()
		_ = cleanup()
		return "", nil, ErrHarnessProcessUnavailable
	}
	if err := config.Close(); err != nil {
		_ = cleanup()
		return "", nil, ErrHarnessProcessUnavailable
	}
	return codexHome, cleanup, nil
}
