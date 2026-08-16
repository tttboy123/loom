package provider

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const maxCodexConversationPromptBytes = 64 * 1024

var (
	ErrInvalidCodexConversationConfig = errors.New("invalid Codex conversation configuration")
	ErrCodexConversationUnavailable   = errors.New("Codex conversation unavailable")
	// ErrCodexConversationUsageLimit is returned when the Codex CLI reports the
	// account has hit its usage/credit limit (for example official OpenAI Codex
	// credits exhausted). It lets the responder surface an actionable message.
	ErrCodexConversationUsageLimit = errors.New("Codex conversation usage limit reached")
	// ErrCodexConversationAuth is returned when the Codex CLI fails to
	// authenticate with the selected provider.
	ErrCodexConversationAuth = errors.New("Codex conversation authentication failed")
)

type CodexConversationProcessRequest struct {
	ExecutablePath  string
	HomePath        string
	PrivateRoot     string
	ModelID         string
	ReasoningEffort string
	UseNativeAuth   bool
	Prompt          string
	MaxOutputBytes  int
}

type CodexConversationProcessRunner interface {
	RunCodexConversation(
		context.Context,
		CodexConversationProcessRequest,
	) ([]byte, error)
}

type CodexConversationConfig struct {
	ExecutablePath string
	HomePath       string
	PrivateRoot    string
	Timeout        time.Duration
	MaxOutputBytes int
	Runner         CodexConversationProcessRunner
}

type CodexConversationClient struct {
	executablePath string
	homePath       string
	privateRoot    string
	timeout        time.Duration
	maxOutputBytes int
	runner         CodexConversationProcessRunner
}

func NewCodexConversationClient(
	config CodexConversationConfig,
) (*CodexConversationClient, error) {
	if config.Timeout <= 0 || config.Timeout > 5*time.Minute ||
		config.MaxOutputBytes < 64 || config.MaxOutputBytes > 64*1024 ||
		interfaceIsNil(config.Runner) ||
		!cleanAbsolutePath(config.HomePath) ||
		!cleanAbsolutePath(config.PrivateRoot) {
		return nil, ErrInvalidCodexConversationConfig
	}
	if _, err := codexExecutableIdentity(config.ExecutablePath); err != nil {
		return nil, ErrInvalidCodexConversationConfig
	}
	if err := ensureCodexConversationDirectory(config.PrivateRoot); err != nil {
		return nil, ErrInvalidCodexConversationConfig
	}
	return &CodexConversationClient{
		executablePath: config.ExecutablePath,
		homePath:       config.HomePath,
		privateRoot:    config.PrivateRoot,
		timeout:        config.Timeout,
		maxOutputBytes: config.MaxOutputBytes,
		runner:         config.Runner,
	}, nil
}

func (client *CodexConversationClient) Respond(
	ctx context.Context,
	prompt string,
) (string, error) {
	return client.RespondConfigured(ctx, prompt, "", "")
}

// RespondConfigured runs one Codex conversation turn with an explicit model and
// optional reasoning effort. Empty modelID uses the Loom gateway default
// (gpt-5.5-codex); the native DeepSeek V4 models run through the user's own
// Codex configuration (cc-switch custom provider) without the gateway.
func (client *CodexConversationClient) RespondConfigured(
	ctx context.Context,
	prompt string,
	modelID string,
	reasoningEffort string,
) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if client == nil || ctx == nil || prompt == "" ||
		len(prompt) > maxCodexConversationPromptBytes ||
		!utf8.ValidString(prompt) || strings.IndexByte(prompt, 0) >= 0 {
		return "", ErrCodexConversationUnavailable
	}
	useNative := false
	if modelID != "" {
		model, err := ValidateConversationModel("openai", modelID)
		if err != nil {
			return "", ErrCodexConversationUnavailable
		}
		if reasoningEffort != "" {
			if err := ValidateConversationReasoningEffort(model, reasoningEffort); err != nil {
				return "", ErrCodexConversationUnavailable
			}
		}
		if strings.HasPrefix(modelID, "deepseek-v4-") {
			useNative = true
		}
	}
	runContext, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	output, err := client.runner.RunCodexConversation(
		runContext,
		CodexConversationProcessRequest{
			ExecutablePath:  client.executablePath,
			HomePath:        client.homePath,
			PrivateRoot:     client.privateRoot,
			ModelID:         modelID,
			ReasoningEffort: reasoningEffort,
			UseNativeAuth:   useNative,
			Prompt:          prompt,
			MaxOutputBytes:  client.maxOutputBytes,
		},
	)
	if err != nil {
		// Preserve specific, actionable Codex CLI failures (usage limit, auth)
		// so the responder can surface a clear message instead of a generic
		// runtime error.
		if errors.Is(err, ErrCodexConversationUsageLimit) ||
			errors.Is(err, ErrCodexConversationAuth) {
			return "", err
		}
		return "", ErrCodexConversationUnavailable
	}
	if len(output) > client.maxOutputBytes ||
		!utf8.Valid(output) || bytes.IndexByte(output, 0) >= 0 {
		return "", ErrCodexConversationUnavailable
	}
	response := strings.TrimSpace(string(output))
	if response == "" {
		return "", ErrCodexConversationUnavailable
	}
	return response, nil
}

type SystemCodexConversationRunner struct{}

func NewSystemCodexConversationRunner() SystemCodexConversationRunner {
	return SystemCodexConversationRunner{}
}

func (SystemCodexConversationRunner) RunCodexConversation(
	ctx context.Context,
	request CodexConversationProcessRequest,
) ([]byte, error) {
	if ctx == nil || request.Prompt == "" ||
		len(request.Prompt) > maxCodexConversationPromptBytes ||
		request.MaxOutputBytes < 64 || request.MaxOutputBytes > 64*1024 ||
		!cleanAbsolutePath(request.HomePath) ||
		!cleanAbsolutePath(request.PrivateRoot) {
		return nil, ErrInvalidCodexConversationConfig
	}
	before, err := codexExecutableIdentity(request.ExecutablePath)
	if err != nil {
		return nil, ErrCodexExecutableIdentityChanged
	}
	if err := ensureCodexConversationDirectory(request.PrivateRoot); err != nil {
		return nil, ErrCodexConversationUnavailable
	}
	output, err := os.CreateTemp(request.PrivateRoot, "response-*.txt")
	if err != nil {
		return nil, ErrCodexConversationUnavailable
	}
	outputPath := output.Name()
	if chmodErr := output.Chmod(0o600); chmodErr != nil {
		_ = output.Close()
		_ = os.Remove(outputPath)
		return nil, ErrCodexConversationUnavailable
	}
	if closeErr := output.Close(); closeErr != nil {
		_ = os.Remove(outputPath)
		return nil, ErrCodexConversationUnavailable
	}
	defer os.Remove(outputPath)

	arguments := codexConversationArguments(outputPath, request)
	command := exec.CommandContext(ctx, request.ExecutablePath, arguments...)
	command.Dir = request.PrivateRoot
	command.Env = []string{
		"HOME=" + request.HomePath,
		"TMPDIR=" + request.PrivateRoot,
		"PATH=" + filepath.Dir(request.ExecutablePath) + ":/usr/bin:/bin",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"NO_COLOR=1",
	}
	command.Stdin = strings.NewReader(request.Prompt)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	command.WaitDelay = 2 * time.Second
	stdout := &boundedCodexBuffer{maximum: 64 * 1024}
	stderr := &boundedCodexBuffer{maximum: 64 * 1024}
	command.Stdout = stdout
	command.Stderr = stderr
	launchIdentity, identityErr := codexExecutableIdentity(request.ExecutablePath)
	if identityErr != nil || !sameCodexExecutableIdentity(before, launchIdentity) {
		return nil, ErrCodexExecutableIdentityChanged
	}
	if runErr := command.Run(); runErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if diagnostic := stderr.buffer.String(); classifyCodexConversationFailure(diagnostic) {
			if strings.Contains(diagnostic, "usage limit") ||
				strings.Contains(diagnostic, "credits") ||
				strings.Contains(diagnostic, "quota") {
				return nil, ErrCodexConversationUsageLimit
			}
			return nil, ErrCodexConversationAuth
		}
		return nil, ErrCodexConversationUnavailable
	}
	after, identityErr := codexExecutableIdentity(request.ExecutablePath)
	if identityErr != nil || !sameCodexExecutableIdentity(before, after) {
		return nil, ErrCodexExecutableIdentityChanged
	}
	file, err := os.Open(outputPath)
	if err != nil {
		return nil, ErrCodexConversationUnavailable
	}
	defer file.Close()
	response, err := io.ReadAll(io.LimitReader(file, int64(request.MaxOutputBytes+1)))
	if err != nil || len(response) > request.MaxOutputBytes {
		return nil, ErrCodexConversationUnavailable
	}
	if len(response) == 0 {
		return nil, ErrCodexConversationUnavailable
	}
	return response, nil
}

func codexConversationArguments(
	outputPath string,
	request CodexConversationProcessRequest,
) []string {
	arguments := []string{
		"exec",
		"--ephemeral",
		"--sandbox", "read-only",
		"--skip-git-repo-check",
	}
	if !request.UseNativeAuth {
		// Gateway mode ignores the user config so the Loom OpenAI gateway
		// controls the model.
		arguments = append(arguments, "--ignore-user-config")
	}
	arguments = append(arguments, "--ignore-rules")
	if request.ModelID != "" && request.UseNativeAuth {
		arguments = append(arguments, "--model", request.ModelID)
	}
	if request.ReasoningEffort != "" {
		arguments = append(
			arguments,
			"-c", `model_reasoning_effort="`+request.ReasoningEffort+`"`,
		)
	}
	for _, feature := range []string{
		"shell_tool",
		"unified_exec",
		"shell_snapshot",
		"apps",
		"enable_mcp_apps",
		"in_app_browser",
		"browser_use",
		"browser_use_external",
		"computer_use",
		"multi_agent",
		"image_generation",
		"code_mode_host",
		"tool_suggest",
	} {
		arguments = append(arguments, "--disable", feature)
	}
	return append(arguments,
		"--color", "never",
		"-c", `approval_policy="never"`,
		"--output-last-message", outputPath,
		"-",
	)
}

func cleanAbsolutePath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path
}

func ensureCodexConversationDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidCodexConversationConfig
	}
	if info.Mode().Perm()&0o077 != 0 {
		return ErrInvalidCodexConversationConfig
	}
	return nil
}

// classifyCodexConversationFailure reports whether the Codex CLI stderr shows a
// known, actionable failure (usage limit, auth, or connection) rather than an
// opaque runtime error.
func classifyCodexConversationFailure(diagnostic string) bool {
	if diagnostic == "" {
		return false
	}
	lower := strings.ToLower(diagnostic)
	for _, marker := range []string{
		"usage limit", "credits", "quota",
		"unauthorized", "authentication", "invalid api key", "401",
		"connection refused", "connection error", "timed out",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
