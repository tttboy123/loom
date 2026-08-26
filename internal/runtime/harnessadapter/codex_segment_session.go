package harnessadapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
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
}

type CodexSegmentSession struct {
	stream         HarnessStreamSession
	threadID       string
	modelID        string
	reasoning      string
	maximum        int
	cleanupPrompt  func() error
	cleanupHome    func() error
	cancelLifetime context.CancelFunc
	mu             sync.Mutex
	turnSequence   int
	closed         bool
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
		config.MaxOutputBytes < 256 || config.MaxOutputBytes > 1<<20 {
		return nil, ErrInvalidCodexAdapter
	}
	systemPromptPath, cleanupPrompt, err := materializeHarnessSystemPrompt(
		config.PrivateRoot, config.SystemPrompt,
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
		SystemPrompt: config.SystemPrompt,
		Timeout:      config.Timeout, MaxOutputBytes: config.MaxOutputBytes,
	}
	lifetimeContext, cancelLifetime := context.WithCancel(context.Background())
	stream, err := config.Sessions.StartSession(
		lifetimeContext,
		HarnessSessionRequest{
			ExecutablePath: config.ExecutablePath,
			Arguments: codexNativeConversationAppServerArguments(
				request, systemPromptPath,
			),
			Environment:    codexNativeConversationEnvironment(request, codexHome),
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
		cancelLifetime: cancelLifetime,
	}
	abort = false
	cleaned = true
	return segment, nil
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
	session.turnSequence++
	request := HarnessProcessRequest{
		ModelID: session.modelID, ReasoningEffort: session.reasoning,
		MaxOutputBytes: session.maximum,
	}
	content, accounting, err := codexAppServerTurn(
		ctx, session.stream, session.threadID, session.turnSequence, request, prompt,
	)
	if err != nil {
		if errors.Is(err, errCodexAppServerTurnInterrupted) {
			return HarnessProcessResult{}, err
		}
		err = errors.Join(err, session.failLocked())
		return HarnessProcessResult{}, err
	}
	return HarnessProcessResult{Content: content, Accounting: accounting}, nil
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
	return result
}

func codexNativeConversationAppServerArguments(
	request HarnessProcessRequest,
	systemPromptPath string,
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
) []string {
	return []string{
		"HOME=" + request.HomePath,
		"CODEX_HOME=" + codexHome,
		"TMPDIR=" + request.TempPath,
		"PATH=" + filepath.Dir(request.ExecutablePath) + ":/usr/bin:/bin",
		"LANG=C.UTF-8", "LC_ALL=C.UTF-8", "NO_COLOR=1",
		"CODEX_MANAGED_BY_LOOM=1",
	}
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
