package harnessadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/provider"
)

const (
	openCodeGovernedAgentID          = "loom-governed"
	openCodeControlPluginDirectory   = ".loom-opencode-control"
	openCodeControlPluginFilename    = "loom-control-arbitration.mjs"
	openCodeControlHomeDirectory     = ".loom-opencode-home"
	openCodeControlXDGDirectory      = ".loom-opencode-xdg"
	openCodeControlPluginMaximumSize = 32 << 10
	openCodeNativeAuthMaximumSize    = 256 << 10
)

type openCodeControlArbitration struct {
	pluginPath  string
	rootPath    string
	homePath    string
	xdgPath     string
	configPath  string
	source      []byte
	identity    os.FileInfo
	directories []openCodeControlDirectoryIdentity
}

type openCodeControlDirectoryIdentity struct {
	path string
	mode os.FileMode
	info os.FileInfo
}

// openCodeProcessRunner adapts OpenCode's multi-model CLI
// (`opencode run --format json --model <provider/model>`) to the Harness
// process contract. Ordinary turns use --pure. Control turns load one private,
// per-turn Loom plugin under isolated config paths. Credentials are injected
// through the provider's well-known OpenCode environment variable; OpenCode
// may also use its native auth store. Accounting stays unobserved.
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
) (_ HarnessProcessResult, resultErr error) {
	defer zeroHarnessBytes(request.Prompt)
	if runner == nil || ctx == nil || !validOpenCodeProcessRequest(request) ||
		request.RequiresCredential && (len(secret) == 0 || len(secret) > 8192) {
		return HarnessProcessResult{}, ErrInvalidOpenCodeAdapter
	}
	// OpenCode reads a non-interactive message from stdin when no positional
	// message is present. Keep the governed context out of process argv, where
	// other same-user processes could observe it.
	message := append([]byte(nil), request.Prompt...)
	defer zeroHarnessBytes(message)
	if len(message) > maxHarnessPromptBytes || !utf8.Valid(message) {
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	var arbitration *openCodeControlArbitration
	if request.ControlMCP.URL != "" {
		var err error
		arbitration, err = materializeOpenCodeControlArbitration(
			request.TempPath, request.ControlMCP.ToolNames,
		)
		if err != nil {
			return HarnessProcessResult{}, ErrHarnessProcessUnavailable
		}
		defer func() {
			if err := arbitration.close(); err != nil {
				resultErr = errors.Join(resultErr, ErrHarnessProcessUnavailable)
			}
		}()
		if !arbitration.valid() {
			return HarnessProcessResult{}, ErrHarnessProcessUnavailable
		}
	}
	pluginPath := ""
	if arbitration != nil {
		pluginPath = arbitration.pluginPath
	}
	environment, err := openCodeEnvironment(request, secret, pluginPath)
	if err != nil {
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	commandResult, commandErr := runner.commands.RunCommand(
		ctx,
		HarnessCommandRequest{
			ExecutablePath: request.ExecutablePath,
			Arguments:      openCodeArguments(request),
			Environment:    environment,
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
		if errors.Is(commandErr, context.DeadlineExceeded) {
			return HarnessProcessResult{}, ErrHarnessProviderTimeout
		}
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	if arbitration != nil && !arbitration.valid() {
		return HarnessProcessResult{}, ErrHarnessProcessUnavailable
	}
	if commandResult.ExitCode != 0 {
		decoded, decodeErr := decodeOpenCodeProcessCompletion(
			ctx, request, commandResult.Stdout,
		)
		// The OpenCode launcher can report a cleanup failure after its structured
		// stream has already reached session.idle. Preserve that completed
		// candidate; the adapter still requires authenticated governed tool
		// activity before accepting an empty, tool-only result.
		if decodeErr == nil {
			return HarnessProcessResult{
				Content: decoded,
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
	decoded, decodeErr := decodeOpenCodeProcessCompletion(
		ctx, request, commandResult.Stdout,
	)
	if decodeErr != nil {
		return HarnessProcessResult{}, decodeErr
	}
	return HarnessProcessResult{
		Content: decoded,
		Stderr:  []byte{},
	}, nil
}

func decodeOpenCodeProcessCompletion(
	ctx context.Context,
	request HarnessProcessRequest,
	payload []byte,
) (string, error) {
	if request.ControlMCP.URL != "" {
		completion, err := provider.DecodeOpenCodeControlHarness(
			payload, request.MaxOutputBytes,
		)
		if err != nil {
			return "", err
		}
		completed := make(map[string]struct{}, len(completion.CompletedToolNames))
		completedInOrder := make([]string, 0, len(completion.CompletedToolNames))
		for _, rawName := range completion.CompletedToolNames {
			name, found := openCodeCompletedControlToolName(
				rawName, request.ControlMCP,
			)
			if !found {
				if openCodeCompletedContextToolName(rawName, request.ContextMCP) {
					continue
				}
				return "", ErrHarnessControlMCP
			}
			completed[name] = struct{}{}
			completedInOrder = append(completedInOrder, name)
		}
		if !completion.Structured && len(completed) == 0 {
			return "", ErrHarnessControlMCP
		}
		if len(completedInOrder) != 0 {
			if err := VerifyHarnessControlToolCompletions(
				ctx, request.ControlMCP, completedInOrder,
			); err != nil {
				return "", err
			}
		}
		if completion.SelectedTool != nil {
			defer zeroHarnessBytes(completion.SelectedTool.Arguments)
			if _, alreadyCompleted := completed[completion.SelectedTool.Name]; alreadyCompleted {
				return strings.TrimSpace(string(completion.Content)), nil
			}
			if len(completed) != 0 {
				return "", ErrHarnessControlMCP
			}
			result, err := CallHarnessControlTool(
				ctx, request.ControlMCP, completion.SelectedTool.Name,
				completion.SelectedTool.Arguments,
			)
			zeroHarnessBytes(result)
			if err != nil {
				return "", err
			}
		}
		return strings.TrimSpace(string(completion.Content)), nil
	}
	decoded, err := provider.DecodeOpenCodeHarnessText(
		payload, request.MaxOutputBytes,
	)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(decoded)), nil
}

func openCodeCompletedControlToolName(
	rawName string,
	lease HarnessControlMCPLease,
) (string, bool) {
	for _, name := range lease.ToolNames {
		if rawName == name || rawName == "loom_control_"+name {
			return name, true
		}
	}
	return "", false
}

func openCodeCompletedContextToolName(
	rawName string,
	lease HarnessContextMCPLease,
) bool {
	for _, name := range harnessMCPToolNames(lease) {
		if rawName == name || rawName == "loom_context_"+name {
			return true
		}
	}
	return false
}

func materializeOpenCodeControlArbitration(
	tempPath string,
	toolNames []string,
) (*openCodeControlArbitration, error) {
	if !cleanHarnessAbsolutePath(tempPath) || len(toolNames) == 0 ||
		len(toolNames) > 64 {
		return nil, ErrHarnessProtocol
	}
	tempInfo, err := os.Lstat(tempPath)
	if err != nil || !tempInfo.IsDir() || tempInfo.Mode()&os.ModeSymlink != 0 ||
		tempInfo.Mode().Perm() != 0o700 {
		return nil, ErrHarnessProtocol
	}
	rootPath := filepath.Join(tempPath, openCodeControlPluginDirectory)
	homePath := filepath.Join(tempPath, openCodeControlHomeDirectory)
	xdgPath := filepath.Join(tempPath, openCodeControlXDGDirectory)
	created := make([]string, 0, 3)
	directories := make([]openCodeControlDirectoryIdentity, 0, 5)
	cleanup := func() {
		for index := len(created) - 1; index >= 0; index-- {
			_ = os.RemoveAll(created[index])
		}
	}
	for _, path := range []string{rootPath, homePath, xdgPath} {
		if err := os.Mkdir(path, 0o700); err != nil {
			cleanup()
			return nil, ErrHarnessProtocol
		}
		created = append(created, path)
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
			info.Mode().Perm() != 0o700 {
			cleanup()
			return nil, ErrHarnessProtocol
		}
		directories = append(directories, openCodeControlDirectoryIdentity{
			path: path, mode: 0o700, info: info,
		})
	}
	configRoot := filepath.Join(xdgPath, "config")
	configPath := filepath.Join(configRoot, "opencode")
	if err := os.Mkdir(configRoot, 0o700); err != nil {
		cleanup()
		return nil, ErrHarnessProtocol
	}
	configRootInfo, err := os.Lstat(configRoot)
	if err != nil || !configRootInfo.IsDir() ||
		configRootInfo.Mode()&os.ModeSymlink != 0 ||
		configRootInfo.Mode().Perm() != 0o700 {
		cleanup()
		return nil, ErrHarnessProtocol
	}
	directories = append(directories, openCodeControlDirectoryIdentity{
		path: configRoot, mode: 0o700, info: configRootInfo,
	})
	if err := os.Mkdir(configPath, 0o500); err != nil {
		cleanup()
		return nil, ErrHarnessProtocol
	}
	configInfo, err := os.Lstat(configPath)
	if err != nil || !configInfo.IsDir() ||
		configInfo.Mode()&os.ModeSymlink != 0 || configInfo.Mode().Perm() != 0o500 {
		cleanup()
		return nil, ErrHarnessProtocol
	}
	directories = append(directories, openCodeControlDirectoryIdentity{
		path: configPath, mode: 0o500, info: configInfo,
	})
	source, err := openCodeControlPluginSource(toolNames)
	if err != nil {
		cleanup()
		return nil, err
	}
	pluginPath := filepath.Join(rootPath, openCodeControlPluginFilename)
	file, err := os.OpenFile(
		pluginPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600,
	)
	if err != nil {
		zeroHarnessBytes(source)
		cleanup()
		return nil, ErrHarnessProtocol
	}
	writeErr := error(nil)
	if written, err := file.Write(source); err != nil || written != len(source) {
		writeErr = errors.Join(writeErr, err, io.ErrShortWrite)
	}
	if err := file.Sync(); err != nil {
		writeErr = errors.Join(writeErr, err)
	}
	if err := file.Close(); err != nil {
		writeErr = errors.Join(writeErr, err)
	}
	if directory, err := os.Open(rootPath); err != nil {
		writeErr = errors.Join(writeErr, err)
	} else {
		writeErr = errors.Join(writeErr, directory.Sync(), directory.Close())
	}
	identity, statErr := os.Lstat(pluginPath)
	if writeErr != nil || statErr != nil || !identity.Mode().IsRegular() ||
		identity.Mode()&os.ModeSymlink != 0 || identity.Mode().Perm() != 0o600 ||
		identity.Size() != int64(len(source)) {
		zeroHarnessBytes(source)
		cleanup()
		return nil, ErrHarnessProtocol
	}
	return &openCodeControlArbitration{
		pluginPath: pluginPath, rootPath: rootPath, homePath: homePath,
		xdgPath: xdgPath, configPath: configPath,
		source: source, identity: identity, directories: directories,
	}, nil
}

func openCodeControlPluginSource(toolNames []string) ([]byte, error) {
	seen := make(map[string]struct{}, len(toolNames))
	allowed := make([]string, 0, len(toolNames)+1)
	allowed = append(allowed, "none")
	for _, name := range toolNames {
		if !validHarnessControlToolName(name) || !strings.HasPrefix(name, "loom_") {
			return nil, ErrHarnessProtocol
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, ErrHarnessProtocol
		}
		seen[name] = struct{}{}
		allowed = append(allowed, name)
	}
	encodedNames, err := json.Marshal(allowed)
	if err != nil {
		return nil, ErrHarnessProtocol
	}
	source := []byte(fmt.Sprintf(`const allowedTools = %s;
	const directControlTools = new Set(
	  allowedTools.filter((name) => name !== "none").map((name) => "loom_control_" + name)
	);
	const controlURL = process.env[%q];
	const controlToken = process.env[%q];
	const outputFormatClass = "~effect/Schema/Class/OutputFormatJsonSchema";
	const textFormatClass = "~effect/Schema/Class/OutputFormatText";
	const acknowledgeControlCompletion = async () => {
	  if (!controlURL || !controlToken) {
	    throw new Error("Loom control acknowledgement is unavailable");
	  }
	  const response = await fetch(controlURL, {
	    method: "POST",
	    headers: {
	      "Authorization": "Bearer " + controlToken,
	      "Content-Type": "application/json"
	    },
	    body: JSON.stringify({
	      jsonrpc: "2.0", id: 3, method: %q, params: {}
	    }),
	    signal: AbortSignal.timeout(5000)
	  });
	  if (!response.ok) {
	    throw new Error("Loom control acknowledgement failed");
	  }
	  await response.arrayBuffer();
	};

	export const LoomControlArbitration = async () => {
  let controlCompleted = false;
  return {
  "chat.message": async (_input, output) => {
	if (!output || !output.message || typeof output.message !== "object") {
	  throw new Error("Loom control arbitration requires a message object");
    }
    const format = {
      type: "json_schema",
      retryCount: 2,
      schema: {
        type: "object",
        additionalProperties: false,
        required: ["response", "tool_name", "tool_arguments"],
        properties: {
          response: {
            type: "string",
            minLength: 1,
            maxLength: 32768,
            description: "Final user-visible response. Never include hidden reasoning or secrets."
          },
          tool_name: {
            type: "string",
            enum: allowedTools,
            description: "Choose the exact Loom tool required by the latest user request when it has not already completed. Use none only for ordinary conversation or after the needed Loom tool completed."
          },
          tool_arguments: {
            type: "object",
            description: "Exact JSON arguments for tool_name. Use an empty object with none. Loom validates this against the frozen Tool Registry."
          }
        }
      }
    };
    Object.defineProperty(format, outputFormatClass, {
      value: outputFormatClass,
      enumerable: false,
      configurable: false,
      writable: false
    });
    output.message.format = format;
  },
	  "tool.execute.after": async (input) => {
	    if (!input || !directControlTools.has(input.tool)) return;
	    controlCompleted = true;
	    await acknowledgeControlCompletion();
	  },
  "experimental.chat.messages.transform": async (_input, output) => {
    if (!controlCompleted) return;
    if (!output || !Array.isArray(output.messages)) {
      throw new Error("Loom control arbitration requires message history");
    }
    for (let index = output.messages.length - 1; index >= 0; index--) {
      const message = output.messages[index] && output.messages[index].info;
      if (!message || message.role !== "user") continue;
      const format = { type: "text" };
      Object.defineProperty(format, textFormatClass, {
        value: textFormatClass,
        enumerable: false,
        configurable: false,
        writable: false
      });
      message.format = format;
      return;
    }
    throw new Error("Loom control arbitration could not find the user message");
  }
  };
};
	`, encodedNames, harnessControlMCPURLEnv, harnessControlMCPTokenEnv,
		harnessControlMCPAckMethod))
	zeroHarnessBytes(encodedNames)
	if len(source) == 0 || len(source) > openCodeControlPluginMaximumSize ||
		!utf8.Valid(source) || bytes.IndexByte(source, 0) >= 0 {
		zeroHarnessBytes(source)
		return nil, ErrHarnessProtocol
	}
	return source, nil
}

func (arbitration *openCodeControlArbitration) valid() bool {
	if arbitration == nil || arbitration.identity == nil ||
		!cleanHarnessAbsolutePath(arbitration.pluginPath) ||
		len(arbitration.source) == 0 ||
		len(arbitration.source) > openCodeControlPluginMaximumSize {
		return false
	}
	for _, directory := range arbitration.directories {
		info, err := os.Lstat(directory.path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
			info.Mode().Perm() != directory.mode || directory.info == nil ||
			!os.SameFile(directory.info, info) {
			return false
		}
	}
	info, err := os.Lstat(arbitration.pluginPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 || !os.SameFile(arbitration.identity, info) ||
		info.Size() != int64(len(arbitration.source)) {
		return false
	}
	data, err := os.ReadFile(arbitration.pluginPath)
	valid := err == nil && bytes.Equal(data, arbitration.source)
	zeroHarnessBytes(data)
	return valid
}

func (arbitration *openCodeControlArbitration) close() error {
	if arbitration == nil {
		return nil
	}
	valid := arbitration.valid()
	zeroHarnessBytes(arbitration.source)
	arbitration.source = nil
	var result error
	for _, path := range []string{
		arbitration.rootPath, arbitration.homePath, arbitration.xdgPath,
	} {
		if err := os.RemoveAll(path); err != nil {
			result = errors.Join(result, err)
		}
	}
	if !valid || result != nil {
		return ErrHarnessProtocol
	}
	return nil
}

func classifiedOpenCodeProcessFailure(err error) bool {
	if _, ok := provider.ConversationFailureDetails(err); ok {
		return true
	}
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
		validOptionalOpenCodeSystemPrompt(request.SystemPrompt) &&
		request.Timeout > 0 && request.Timeout <= time.Hour &&
		validHarnessContextMCPLease(request.ContextMCP) &&
		validOptionalHarnessControlMCPLease(request.ControlMCP) &&
		request.MaxOutputBytes >= 256 && request.MaxOutputBytes <= 1<<20
}

func validOptionalOpenCodeSystemPrompt(prompt string) bool {
	return prompt == "" || len(prompt) <= maxHarnessPromptBytes &&
		utf8.ValidString(prompt) && strings.IndexByte(prompt, 0) < 0
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
	}
	if request.ControlMCP.URL == "" {
		arguments = append(arguments, "--pure")
	}
	arguments = append(arguments, "--model", request.ModelID)
	if request.SystemPrompt != "" {
		arguments = append(arguments, "--agent", openCodeGovernedAgentID)
	}
	arguments = append(arguments, "--dir", request.WorkspacePath)
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
	controlPluginPath string,
) ([]string, error) {
	homePath := request.HomePath
	if controlPluginPath != "" {
		homePath = filepath.Join(request.TempPath, openCodeControlHomeDirectory)
	}
	environment := []string{
		"HOME=" + homePath,
		"TMPDIR=" + request.TempPath,
		"PATH=" + filepath.Dir(request.ExecutablePath) + ":/usr/bin:/bin",
		"LANG=C.UTF-8", "LC_ALL=C.UTF-8", "NO_COLOR=1",
	}
	permission := map[string]string{"*": "deny"}
	config := map[string]any{
		"permission": permission,
		"share":      "disabled",
	}
	if controlPluginPath != "" {
		xdgRoot := filepath.Join(request.TempPath, openCodeControlXDGDirectory)
		environment = append(environment,
			"XDG_CONFIG_HOME="+filepath.Join(xdgRoot, "config"),
			"XDG_CACHE_HOME="+filepath.Join(xdgRoot, "cache"),
			"XDG_STATE_HOME="+filepath.Join(xdgRoot, "state"),
			"XDG_DATA_HOME="+filepath.Join(xdgRoot, "data"),
			"OPENCODE_DISABLE_PROJECT_CONFIG=1",
			"OPENCODE_DISABLE_EXTERNAL_SKILLS=1",
			"OPENCODE_DISABLE_LSP_DOWNLOAD=1",
			"OPENCODE_DISABLE_AUTOUPDATE=1",
			"NPM_CONFIG_OFFLINE=true",
		)
		pluginURL := (&url.URL{Scheme: "file", Path: controlPluginPath}).String()
		config["plugin"] = []string{pluginURL}
		if !request.RequiresCredential {
			authContent, found, err := openCodeNativeAuthProjection(
				request.HomePath, request.ModelID,
			)
			if err != nil {
				return nil, err
			}
			if found {
				environment = append(
					environment, "OPENCODE_AUTH_CONTENT="+string(authContent),
				)
			}
			zeroHarnessBytes(authContent)
		}
	}
	if request.SystemPrompt != "" {
		config["agent"] = map[string]any{
			openCodeGovernedAgentID: map[string]any{
				"description": "Loom governed conversation adapter",
				"mode":        "primary",
				"prompt":      request.SystemPrompt,
				"temperature": 0,
			},
		}
	}
	mcp := make(map[string]any, 2)
	if request.ContextMCP.URL != "" {
		permission["loom_context_*"] = "allow"
		mcp["loom_context"] = map[string]any{
			"type": "remote", "url": request.ContextMCP.URL,
			"enabled": true, "timeout": 15000,
			"headers": map[string]string{
				"Authorization": "Bearer {env:" + harnessContextMCPTokenEnv + "}",
			},
		}
		if request.ContextMCP.Token != "" {
			environment = append(environment, harnessContextMCPTokenEnv+"="+request.ContextMCP.Token)
		}
	}
	if request.ControlMCP.URL != "" {
		for _, name := range request.ControlMCP.ToolNames {
			permission["loom_control_"+name] = "allow"
		}
		mcp["loom_control"] = map[string]any{
			"type": "remote", "url": request.ControlMCP.URL,
			"enabled": true, "timeout": 15000,
			"headers": map[string]string{
				"Authorization": "Bearer {env:" + harnessControlMCPTokenEnv + "}",
			},
		}
		environment = append(
			environment,
			harnessControlMCPTokenEnv+"="+request.ControlMCP.Token,
			harnessControlMCPURLEnv+"="+request.ControlMCP.URL,
		)
	}
	if len(mcp) != 0 {
		config["mcp"] = mcp
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
	return environment, nil
}

func openCodeNativeAuthProjection(
	homePath string,
	modelID string,
) ([]byte, bool, error) {
	providerID, ok := openCodeProviderPart(modelID)
	if !ok || providerID != "opencode" {
		return nil, false, nil
	}
	authPath := filepath.Join(homePath, ".local", "share", "opencode", "auth.json")
	before, err := os.Lstat(authPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 ||
		before.Mode().Perm() != 0o600 || before.Size() <= 0 ||
		before.Size() > openCodeNativeAuthMaximumSize {
		return nil, false, ErrHarnessProtocol
	}
	file, err := os.Open(authPath)
	if err != nil {
		return nil, false, ErrHarnessProtocol
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, openCodeNativeAuthMaximumSize+1))
	closeErr := file.Close()
	after, afterErr := os.Lstat(authPath)
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil ||
		opened == nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) ||
		len(data) == 0 || len(data) > openCodeNativeAuthMaximumSize ||
		!utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 ||
		rejectHarnessDuplicateJSONKeys(data) {
		zeroHarnessBytes(data)
		return nil, false, ErrHarnessProtocol
	}
	defer zeroHarnessBytes(data)
	decoder := json.NewDecoder(bytes.NewReader(data))
	var accounts map[string]json.RawMessage
	if decoder.Decode(&accounts) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, false, ErrHarnessProtocol
	}
	selected, found := accounts[providerID]
	if !found || len(selected) == 0 || !json.Valid(selected) ||
		rejectHarnessDuplicateJSONKeys(selected) {
		return nil, false, nil
	}
	var selectedObject map[string]any
	selectedDecoder := json.NewDecoder(bytes.NewReader(selected))
	selectedDecoder.UseNumber()
	if selectedDecoder.Decode(&selectedObject) != nil || selectedObject == nil ||
		selectedDecoder.Decode(&struct{}{}) != io.EOF {
		return nil, false, ErrHarnessProtocol
	}
	projection, err := json.Marshal(map[string]json.RawMessage{
		providerID: append(json.RawMessage(nil), selected...),
	})
	if err != nil || len(projection) == 0 || len(projection) > openCodeNativeAuthMaximumSize {
		zeroHarnessBytes(projection)
		return nil, false, ErrHarnessProtocol
	}
	return projection, true, nil
}

func openCodeProviderPart(identity string) (string, bool) {
	index := strings.IndexByte(identity, '/')
	if index <= 0 || index >= len(identity)-1 {
		return "", false
	}
	return identity[:index], true
}
