package harnessadapter

import (
	"context"
	"encoding/json"
	"errors"
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
	// OpenCode reads a non-interactive message from stdin when no positional
	// message is present. Keep the governed context out of process argv, where
	// other same-user processes could observe it.
	message := make([]byte, 0, len(request.SystemPrompt)+2+len(request.Prompt))
	if request.SystemPrompt != "" {
		message = append(message, request.SystemPrompt...)
		message = append(message, '\n', '\n')
	}
	message = append(message, request.Prompt...)
	defer zeroHarnessBytes(message)
	if len(message) > maxHarnessPromptBytes || !utf8.Valid(message) {
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	commandResult, commandErr := runner.commands.RunCommand(
		ctx,
		HarnessCommandRequest{
			ExecutablePath: request.ExecutablePath,
			Arguments:      openCodeArguments(request),
			Environment:    openCodeEnvironment(request, secret),
			Directory:      request.WorkspacePath,
			Stdin:          message,
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
	if commandResult.ExitCode != 0 {
		decoded, decodeErr := provider.DecodeOpenCodeHarnessText(
			commandResult.Stdout, request.MaxOutputBytes,
		)
		// The OpenCode launcher can report a cleanup failure after its structured
		// stream has already reached session.idle. Preserve that completed
		// candidate; the adapter still requires authenticated governed tool
		// activity before accepting an empty, tool-only result.
		if decodeErr == nil {
			return HarnessProcessResult{
				Content: strings.TrimSpace(string(decoded)),
				Stderr:  []byte{},
			}, nil
		}
		if classifiedOpenCodeProcessFailure(decodeErr) {
			return HarnessProcessResult{}, decodeErr
		}
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	if len(commandResult.Stdout) > request.MaxOutputBytes ||
		len(commandResult.Stderr) > request.MaxOutputBytes {
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	decoded, decodeErr := provider.DecodeOpenCodeHarnessText(
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

func classifiedOpenCodeProcessFailure(err error) bool {
	return errors.Is(err, provider.ErrOpenCodeConversationAuth) ||
		errors.Is(err, provider.ErrOpenCodeConversationInsufficientBalance) ||
		errors.Is(err, provider.ErrOpenCodeConversationModelUnavailable) ||
		errors.Is(err, provider.ErrOpenCodeConversationRateLimit)
}

func validOpenCodeProcessRequest(request HarnessProcessRequest) bool {
	return cleanHarnessAbsolutePath(request.ExecutablePath) &&
		cleanHarnessAbsolutePath(request.WorkspacePath) &&
		cleanHarnessAbsolutePath(request.HomePath) && cleanHarnessAbsolutePath(request.TempPath) &&
		len(request.Prompt) > 0 && len(request.Prompt) <= maxHarnessPromptBytes &&
		utf8.Valid(request.Prompt) &&
		validOpenCodeModelIdentityString(request.ModelID) &&
		validOpenCodeReasoningEffort(request.ModelID, request.ReasoningEffort) &&
		request.Timeout > 0 && request.Timeout <= time.Hour &&
		validHarnessContextMCPLease(request.ContextMCP) &&
		request.MaxOutputBytes >= 256 && request.MaxOutputBytes <= 1<<20
}

func validOpenCodeModelIdentityString(identity string) bool {
	if identity == "" || len(identity) > 512 || !utf8.ValidString(identity) {
		return false
	}
	return provider.ValidOpenCodeModelIdentity(identity)
}

func validOpenCodeReasoningEffort(modelID, reasoningEffort string) bool {
	if reasoningEffort != strings.TrimSpace(reasoningEffort) {
		return false
	}
	model, err := provider.ValidateConversationModel("opencode", modelID)
	if err != nil {
		return false
	}
	return provider.ValidateConversationReasoningEffort(model, reasoningEffort) == nil
}

// openCodeArguments builds the OpenCode CLI arguments. The model identity is
// already adapted to "provider/model" by the adapter.
func openCodeArguments(request HarnessProcessRequest) []string {
	arguments := []string{
		"run",
		"--format", "json",
		"--pure",
		"--model", request.ModelID,
		"--dir", request.WorkspacePath,
	}
	if request.ReasoningEffort != "" {
		arguments = append(arguments, "--variant", request.ReasoningEffort)
	}
	return arguments
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
	permission := map[string]string{"*": "deny"}
	config := map[string]any{
		"permission": permission,
		"share":      "disabled",
	}
	if request.ContextMCP.URL != "" {
		permission["loom_context_*"] = "allow"
		config["mcp"] = map[string]any{
			"loom_context": map[string]any{
				"type": "remote", "url": request.ContextMCP.URL,
				"enabled": true, "timeout": 15000,
				"headers": map[string]string{
					"Authorization": "Bearer {env:" + harnessContextMCPTokenEnv + "}",
				},
			},
		}
		if request.ContextMCP.Token != "" {
			environment = append(environment, harnessContextMCPTokenEnv+"="+request.ContextMCP.Token)
		}
	}
	if encodedConfig, err := json.Marshal(config); err == nil {
		environment = append(
			environment, "OPENCODE_CONFIG_CONTENT="+string(encodedConfig),
		)
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
