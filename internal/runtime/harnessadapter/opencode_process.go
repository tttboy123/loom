package harnessadapter

import (
	"context"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/provider"
)

// openCodeProcessRunner adapts OpenCode's multi-model CLI
// (`opencode run --format json --pure --model <provider/model>`) to the
// Harness process contract. Credentials are injected through the provider's
// well-known OpenCode environment variable; OpenCode may also use its own
// native auth store. Accounting stays unobserved (no invented numbers).
type openCodeProcessRunner struct {
	commands HarnessCommandRunner
}

type OpenCodeProcessRunnerConfig struct {
	Commands HarnessCommandRunner
}

func NewOpenCodeProcessRunner(
	config OpenCodeProcessRunnerConfig,
) (HarnessProcessRunner, error) {
	if nilHarnessInterface(config.Commands) {
		return nil, ErrInvalidOpenCodeAdapter
	}
	return &openCodeProcessRunner{commands: config.Commands}, nil
}

func (runner *openCodeProcessRunner) SupportsAgentInputs() bool {
	// OpenCode session continuation is not wired yet.
	return false
}

func (runner *openCodeProcessRunner) RunHarness(
	ctx context.Context,
	request HarnessProcessRequest,
	secret []byte,
) (HarnessProcessResult, error) {
	defer zeroHarnessBytes(request.Prompt)
	if runner == nil || ctx == nil || !validOpenCodeProcessRequest(request) ||
		request.RequiresCredential && (len(secret) == 0 || len(secret) > 8192) {
		return HarnessProcessResult{}, ErrInvalidOpenCodeAdapter
	}
	// The Loom system prompt is prepended to the model-visible message because
	// OpenCode has no separate system-prompt file flag.
	message := request.SystemPrompt
	if message != "" {
		message += "\n\n"
	}
	message += string(request.Prompt)
	if len(message) > maxHarnessPromptBytes || !utf8.ValidString(message) {
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	commandResult, commandErr := runner.commands.RunCommand(
		ctx,
		HarnessCommandRequest{
			ExecutablePath: request.ExecutablePath,
			Arguments:      openCodeArguments(request, message),
			Environment:    openCodeEnvironment(request, secret),
			Directory:      request.TempPath,
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
	if commandResult.ExitCode != 0 || len(commandResult.Stdout) > request.MaxOutputBytes ||
		len(commandResult.Stderr) > request.MaxOutputBytes {
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	decoded, decodeErr := provider.DecodeOpenCodeText(
		commandResult.Stdout, request.MaxOutputBytes,
	)
	if decodeErr != nil {
		return HarnessProcessResult{}, decodeErr
	}
	return HarnessProcessResult{
		Content: strings.TrimSpace(string(decoded)),
		Stderr:  []byte{},
	}, nil
}

func validOpenCodeProcessRequest(request HarnessProcessRequest) bool {
	return cleanHarnessAbsolutePath(request.ExecutablePath) &&
		cleanHarnessAbsolutePath(request.WorkspacePath) &&
		cleanHarnessAbsolutePath(request.HomePath) && cleanHarnessAbsolutePath(request.TempPath) &&
		len(request.Prompt) > 0 && len(request.Prompt) <= maxHarnessPromptBytes &&
		utf8.Valid(request.Prompt) &&
		validOpenCodeModelIdentityString(request.ModelID) &&
		request.Timeout > 0 && request.Timeout <= time.Hour &&
		request.MaxOutputBytes >= 256 && request.MaxOutputBytes <= 1<<20
}

func validOpenCodeModelIdentityString(identity string) bool {
	if identity == "" || len(identity) > 512 || !utf8.ValidString(identity) {
		return false
	}
	return provider.ValidOpenCodeModelIdentity(identity)
}

// openCodeArguments builds the OpenCode CLI arguments. The model identity is
// already adapted to "provider/model" by the adapter.
func openCodeArguments(request HarnessProcessRequest, message string) []string {
	return []string{
		"run",
		"--format", "json",
		"--pure",
		"--model", request.ModelID,
		"--dir", request.TempPath,
		message,
	}
}

// openCodeEnvironment injects the provider's well-known OpenCode credential
// environment variable from the Loom lease secret. OpenCode keeps its own
// native auth available through HOME.
func openCodeEnvironment(
	request HarnessProcessRequest,
	secret []byte,
) []string {
	environment := []string{
		"HOME=" + request.HomePath,
		"TMPDIR=" + request.TempPath,
		"PATH=" + filepath.Dir(request.ExecutablePath) + ":/usr/bin:/bin",
		"LANG=C.UTF-8", "LC_ALL=C.UTF-8", "NO_COLOR=1",
	}
	if providerID, ok := openCodeProviderPart(request.ModelID); ok {
		if envName, known := provider.OpenCodeCredentialEnv(providerID); known {
			environment = append(
				environment, envName+"="+string(secret),
			)
		}
	}
	return environment
}

func openCodeProviderPart(identity string) (string, bool) {
	index := strings.IndexByte(identity, '/')
	if index <= 0 || index >= len(identity)-1 {
		return "", false
	}
	return identity[:index], true
}
