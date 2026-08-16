package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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

const (
	// OpenCodeConversationDefaultModel is the default model identity used only
	// when a profile does not pin one. It is a real OpenCode CLI 1.18.3 model
	// (deepseek/deepseek-chat) so a profile with no verified Provider account
	// still starts from a resolvable model instead of an unavailable one.
	OpenCodeConversationDefaultModel   = "deepseek/deepseek-chat"
	maxOpenCodeConversationPromptBytes = 64 * 1024
)

var (
	ErrInvalidOpenCodeConversationConfig = errors.New("invalid OpenCode conversation configuration")
	ErrOpenCodeConversationUnavailable   = errors.New("OpenCode conversation unavailable")
	ErrOpenCodeExecutableIdentityChanged = errors.New("OpenCode executable identity changed")
)

// OpenCodeConversationProcessRequest is the non-secret input to the injected
// process runner. The prompt is model-visible text; it is never persisted or
// committed to the Journal by this package.
type OpenCodeConversationProcessRequest struct {
	ExecutablePath    string
	HomePath          string
	PrivateRoot       string
	ModelID           string
	ReasoningEffort   string
	CredentialEnvName string
	Secret            []byte
	Prompt            string
	MaxOutputBytes    int
}

// OpenCodeConversationProcessRunner runs one bounded `opencode run` process.
// Authorization and credentials are OpenCode's own native auth; this package
// never receives or stores an API key.
type OpenCodeConversationProcessRunner interface {
	RunOpenCodeConversation(
		context.Context,
		OpenCodeConversationProcessRequest,
	) ([]byte, error)
}

type OpenCodeConversationConfig struct {
	ExecutablePath string
	HomePath       string
	PrivateRoot    string
	ModelID        string
	Timeout        time.Duration
	MaxOutputBytes int
	Runner         OpenCodeConversationProcessRunner
}

type OpenCodeConversationClient struct {
	executablePath string
	homePath       string
	privateRoot    string
	modelID        string
	timeout        time.Duration
	maxOutputBytes int
	runner         OpenCodeConversationProcessRunner
}

func NewOpenCodeConversationClient(
	config OpenCodeConversationConfig,
) (*OpenCodeConversationClient, error) {
	if config.Timeout <= 0 || config.Timeout > 5*time.Minute ||
		config.MaxOutputBytes < 64 || config.MaxOutputBytes > 64*1024 ||
		interfaceIsNil(config.Runner) ||
		!cleanAbsolutePath(config.HomePath) ||
		!cleanAbsolutePath(config.PrivateRoot) ||
		!validOpenCodeModelID(config.ModelID) {
		return nil, ErrInvalidOpenCodeConversationConfig
	}
	if _, err := codexExecutableIdentity(config.ExecutablePath); err != nil {
		return nil, ErrInvalidOpenCodeConversationConfig
	}
	if err := ensureCodexConversationDirectory(config.PrivateRoot); err != nil {
		return nil, ErrInvalidOpenCodeConversationConfig
	}
	return &OpenCodeConversationClient{
		executablePath: config.ExecutablePath,
		homePath:       config.HomePath,
		privateRoot:    config.PrivateRoot,
		modelID:        config.ModelID,
		timeout:        config.Timeout,
		maxOutputBytes: config.MaxOutputBytes,
		runner:         config.Runner,
	}, nil
}

// ResolveOpenCodeNativeExecutable validates and resolves the OpenCode CLI
// executable (regular file or symlink wrapper) to an absolute canonical path.
// OpenCode ships as a native binary (for example .../opencode-ai/bin/opencode.exe)
// behind a symlink; unlike Codex there is no package-layout requirement, only
// a regular, executable, absolute target.
func ResolveOpenCodeNativeExecutable(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) || cleaned != path {
		return "", ErrInvalidOpenCodeConversationConfig
	}
	info, err := os.Lstat(cleaned)
	if err != nil {
		return "", ErrInvalidOpenCodeConversationConfig
	}
	if info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
		return cleaned, nil
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", ErrInvalidOpenCodeConversationConfig
	}
	wrapper, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", ErrInvalidOpenCodeConversationConfig
	}
	wrapper = filepath.Clean(wrapper)
	wrapperInfo, err := os.Lstat(wrapper)
	if err != nil || !wrapperInfo.Mode().IsRegular() ||
		wrapperInfo.Mode()&0o111 == 0 {
		return "", ErrInvalidOpenCodeConversationConfig
	}
	return wrapper, nil
}

func validOpenCodeModelID(value string) bool {
	if value == "" || len(value) > 256 ||
		!utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	// provider/model identity: allow letters, digits, dots, dashes, slashes,
	// underscores and colons; reject control characters and quotes.
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			strings.ContainsRune("./-_:", r) {
			continue
		}
		return false
	}
	return true
}

// Respond runs one bounded OpenCode conversation turn and returns the trimmed
// final text. It never persists the Prompt and never commits conversation
// content to any authority.
func (client *OpenCodeConversationClient) Respond(
	ctx context.Context,
	prompt string,
) (string, error) {
	return client.RespondConfigured(ctx, prompt, "", "", "", nil)
}

// RespondConfigured runs one OpenCode conversation turn with an explicit model
// and optional reasoning effort (OpenCode `--variant`). Empty modelID selects
// the client default; OpenCode accepts any valid "provider/model" identity.
func (client *OpenCodeConversationClient) RespondConfigured(
	ctx context.Context,
	prompt string,
	modelID string,
	reasoningEffort string,
	credentialEnv string,
	secret []byte,
) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if client == nil || ctx == nil || prompt == "" ||
		len(prompt) > maxOpenCodeConversationPromptBytes ||
		!utf8.ValidString(prompt) || strings.IndexByte(prompt, 0) >= 0 {
		return "", ErrOpenCodeConversationUnavailable
	}
	if modelID == "" {
		modelID = client.modelID
	}
	model, err := ValidateConversationModel("opencode", modelID)
	if err != nil {
		return "", ErrOpenCodeConversationUnavailable
	}
	if reasoningEffort != "" {
		if err := ValidateConversationReasoningEffort(model, reasoningEffort); err != nil {
			return "", ErrOpenCodeConversationUnavailable
		}
	}
	runContext, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	output, err := client.runner.RunOpenCodeConversation(
		runContext,
		OpenCodeConversationProcessRequest{
			ExecutablePath:    client.executablePath,
			HomePath:          client.homePath,
			PrivateRoot:       client.privateRoot,
			ModelID:           modelID,
			ReasoningEffort:   reasoningEffort,
			CredentialEnvName: credentialEnv,
			Secret:            secret,
			Prompt:            prompt,
			MaxOutputBytes:    client.maxOutputBytes,
		},
	)
	if err != nil || len(output) > client.maxOutputBytes ||
		!utf8.Valid(output) || bytes.IndexByte(output, 0) >= 0 {
		return "", ErrOpenCodeConversationUnavailable
	}
	response := strings.TrimSpace(string(output))
	if response == "" {
		return "", ErrOpenCodeConversationUnavailable
	}
	return response, nil
}

type SystemOpenCodeConversationRunner struct{}

func NewSystemOpenCodeConversationRunner() SystemOpenCodeConversationRunner {
	return SystemOpenCodeConversationRunner{}
}

func (SystemOpenCodeConversationRunner) RunOpenCodeConversation(
	ctx context.Context,
	request OpenCodeConversationProcessRequest,
) ([]byte, error) {
	if ctx == nil || request.Prompt == "" ||
		len(request.Prompt) > maxOpenCodeConversationPromptBytes ||
		request.MaxOutputBytes < 64 || request.MaxOutputBytes > 64*1024 ||
		!cleanAbsolutePath(request.HomePath) ||
		!cleanAbsolutePath(request.PrivateRoot) ||
		!validOpenCodeModelID(request.ModelID) {
		return nil, ErrInvalidOpenCodeConversationConfig
	}
	before, err := codexExecutableIdentity(request.ExecutablePath)
	if err != nil {
		return nil, ErrOpenCodeExecutableIdentityChanged
	}
	if err := ensureCodexConversationDirectory(request.PrivateRoot); err != nil {
		return nil, ErrOpenCodeConversationUnavailable
	}
	arguments := []string{
		"run",
		"--format", "json",
		"--pure",
		"--model", request.ModelID,
		"--dir", request.PrivateRoot,
	}
	if request.ReasoningEffort != "" {
		arguments = append(arguments, "--variant", request.ReasoningEffort)
	}
	arguments = append(arguments, request.Prompt)
	command := exec.CommandContext(ctx, request.ExecutablePath, arguments...)
	command.Dir = request.PrivateRoot
	// Inherit the parent environment (Provider credential variables such as
	// DEEPSEEK_API_KEY / MINIMAX_API_KEY / ZHIPU_API_KEY must reach OpenCode's
	// native auth) while pinning the Loom-controlled variables.
	environment := append([]string{}, os.Environ()...)
	environment = append(environment,
		"HOME="+request.HomePath,
		"TMPDIR="+request.PrivateRoot,
		"PATH="+filepath.Dir(request.ExecutablePath)+":/usr/bin:/bin",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"NO_COLOR=1",
	)
	if request.CredentialEnvName != "" && len(request.Secret) > 0 {
		environment = append(
			environment,
			request.CredentialEnvName+"="+string(request.Secret),
		)
	}
	command.Env = environment
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
	stdout := &boundedCodexBuffer{maximum: request.MaxOutputBytes}
	stderr := &boundedCodexBuffer{maximum: request.MaxOutputBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	launchIdentity, identityErr := codexExecutableIdentity(request.ExecutablePath)
	if identityErr != nil || !sameCodexExecutableIdentity(before, launchIdentity) {
		return nil, ErrOpenCodeExecutableIdentityChanged
	}
	if runErr := command.Run(); runErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrOpenCodeConversationUnavailable
	}
	after, identityErr := codexExecutableIdentity(request.ExecutablePath)
	if identityErr != nil || !sameCodexExecutableIdentity(before, after) {
		return nil, ErrOpenCodeExecutableIdentityChanged
	}
	if stdout.err != nil || len(stdout.buffer.Bytes()) > request.MaxOutputBytes {
		return nil, ErrOpenCodeConversationUnavailable
	}
	decoded, decodeErr := decodeOpenCodeConversation(
		stdout.buffer.Bytes(), request.MaxOutputBytes,
	)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return decoded, nil
}

// DecodeOpenCodeText parses the `opencode run --format json` newline-delimited
// event stream and returns the final assistant text. It is the shared decoder
// for the conversation client and the team-attempt Harness adapter.
func DecodeOpenCodeText(payload []byte, maximum int) ([]byte, error) {
	return decodeOpenCodeConversation(payload, maximum)
}

// decodeOpenCodeConversation parses the `opencode run --format json` newline-
// delimited event stream and returns the final assistant text. The schema is
// grounded in the installed OpenCode 1.18.3 CLI output: `text` events carry
// `part.type == "text"` with `part.text`; `step_finish` and `session.idle`
// mark completion; `session.error` and `auth.error` fail closed. The older
// `message.part.updated` / `properties.part` shape is also accepted.
func decodeOpenCodeConversation(payload []byte, maximum int) ([]byte, error) {
	if len(payload) == 0 || len(payload) > maximum || !utf8.Valid(payload) ||
		bytes.IndexByte(payload, 0) >= 0 {
		return nil, ErrOpenCodeConversationUnavailable
	}
	var content strings.Builder
	completed := false
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 4096), maximum)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event struct {
			Type  string `json:"type"`
			Error string `json:"error"`
			Part  struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"part"`
			Properties struct {
				Part struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"part"`
			} `json:"properties"`
		}
		decoder := json.NewDecoder(bytes.NewReader(line))
		if decoder.Decode(&event) != nil ||
			decoder.Decode(&struct{}{}) != io.EOF {
			return nil, ErrOpenCodeConversationUnavailable
		}
		var partType, partText string
		switch event.Type {
		case "text":
			partType, partText = event.Part.Type, event.Part.Text
		case "message.part.updated":
			partType, partText =
				event.Properties.Part.Type, event.Properties.Part.Text
		case "step_finish", "session.idle":
			completed = true
		case "session.error", "auth.error":
			return nil, ErrOpenCodeConversationUnavailable
		}
		if partType == "text" {
			if !utf8.ValidString(partText) ||
				content.Len()+len(partText) > maximum {
				return nil, ErrOpenCodeConversationUnavailable
			}
			content.WriteString(partText)
		}
	}
	if scanner.Err() != nil || !completed {
		return nil, ErrOpenCodeConversationUnavailable
	}
	response := strings.TrimSpace(content.String())
	if response == "" || len(response) > maximum {
		return nil, ErrOpenCodeConversationUnavailable
	}
	return []byte(response), nil
}
