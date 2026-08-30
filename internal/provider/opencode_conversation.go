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
	// when a profile does not pin one. It is OpenCode's own hosted free-tier
	// model (OpenCode CLI 1.18.3 catalog) so a profile starts from a usable
	// model that needs no injected Provider credential, instead of silently
	// routing a DeepSeek/MiniMax model through the OpenCode harness.
	OpenCodeConversationDefaultModel   = "opencode/big-pickle"
	maxOpenCodeConversationPromptBytes = 64 * 1024
	openCodeConversationInlineConfig   = `{"permission":"deny","share":"disabled"}`
)

var (
	ErrInvalidOpenCodeConversationConfig = errors.New("invalid OpenCode conversation configuration")
	ErrOpenCodeConversationUnavailable   = errors.New("OpenCode conversation unavailable")
	ErrOpenCodeConversationAuth          = errors.Join(
		ErrOpenCodeConversationUnavailable,
		errors.New("OpenCode Provider authentication failed"),
	)
	ErrOpenCodeConversationInsufficientBalance = errors.Join(
		ErrOpenCodeConversationUnavailable,
		errors.New("OpenCode Provider balance required"),
	)
	ErrOpenCodeConversationModelUnavailable = errors.Join(
		ErrOpenCodeConversationUnavailable,
		errors.New("OpenCode Provider model unavailable"),
	)
	ErrOpenCodeConversationRateLimit = errors.Join(
		ErrOpenCodeConversationUnavailable,
		errors.New("OpenCode Provider rate limit reached"),
	)
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
	if err := preparePrivateConversationScratchRoot(config.PrivateRoot); err != nil {
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
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, context.Canceled) {
			return "", err
		}
		if classifiedOpenCodeConversationFailure(err) {
			return "", err
		}
		return "", ErrOpenCodeConversationUnavailable
	}
	if len(output) > client.maxOutputBytes ||
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
) (responseBytes []byte, resultErr error) {
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
	processTemp, cleanupProcessTemp, err := preparePrivateConversationProcessTemp(
		request.PrivateRoot,
	)
	if err != nil {
		return nil, ErrOpenCodeConversationUnavailable
	}
	defer func() {
		resultErr = errors.Join(resultErr, cleanupProcessTemp())
	}()
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
	command := exec.CommandContext(ctx, request.ExecutablePath, arguments...)
	command.Dir = request.PrivateRoot
	// OpenCode reads the non-interactive message from stdin when no positional
	// message is present. Keep user content out of process argv.
	command.Stdin = strings.NewReader(request.Prompt)
	// Keep the Harness environment minimal. The exact Provider credential is
	// injected from one bounded Vault lease below; daemon/global credentials
	// and unrelated environment state must not cross this process boundary.
	environment := []string{
		"HOME=" + request.HomePath,
		"TMPDIR=" + processTemp,
		"PATH=" + filepath.Dir(request.ExecutablePath) + ":/usr/bin:/bin",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"NO_COLOR=1",
		"OPENCODE_CONFIG_CONTENT=" + openCodeConversationInlineConfig,
	}
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
		// OpenCode writes a structured error event to stdout and then exits
		// non-zero. Preserve only its privacy-safe classification before falling
		// back to the opaque process error.
		if stdout.err == nil {
			_, eventErr := decodeOpenCodeConversation(
				stdout.buffer.Bytes(), request.MaxOutputBytes,
			)
			if classifiedOpenCodeConversationFailure(eventErr) {
				return nil, eventErr
			}
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
	return decodeOpenCodeEventStream(payload, maximum, false, false)
}

// DecodeOpenCodeHarnessText accepts a completed, tool-only Agent run. Coding
// attempts may finish after governed workspace edits without emitting a final
// prose message; that is distinct from an empty/no-op conversation response.
func DecodeOpenCodeHarnessText(payload []byte, maximum int) ([]byte, error) {
	return decodeOpenCodeEventStream(payload, maximum, false, true)
}

// OpenCodeControlToolSelection is a model-selected Loom control tool. The
// Harness adapter still validates the name against the frozen turn lease and
// sends the arguments through the authenticated MCP server, which performs the
// authoritative Registry schema and Proposal checks.
type OpenCodeControlToolSelection struct {
	Name      string
	Arguments json.RawMessage
}

// OpenCodeControlHarnessCompletion is the strict terminal contract used only
// for OpenCode Conversation control turns. CompletedToolNames records direct
// OpenCode tool activity so the adapter does not replay a tool the model has
// already invoked before returning its structured terminal response.
type OpenCodeControlHarnessCompletion struct {
	Content            []byte
	SelectedTool       *OpenCodeControlToolSelection
	CompletedToolNames []string
	Structured         bool
}

// DecodeOpenCodeControlHarness parses an OpenCode JSON event stream whose
// terminal response either uses StructuredOutput or follows a direct Loom tool
// call. The adapter separately corroborates direct calls with the turn-scoped
// MCP server. Plain prose, duplicate tools and incomplete streams fail closed.
func DecodeOpenCodeControlHarness(
	payload []byte,
	maximum int,
) (OpenCodeControlHarnessCompletion, error) {
	if maximum < 256 || maximum > 1<<20 || len(payload) == 0 ||
		len(payload) > maximum || !utf8.Valid(payload) ||
		bytes.IndexByte(payload, 0) >= 0 {
		return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
	}
	var completion OpenCodeControlHarnessCompletion
	var directContent strings.Builder
	var structuredSeen, completed bool
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 4096), maximum)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if rejectConversationDuplicateJSONKeys(line) {
			return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
		}
		var event struct {
			Type       string          `json:"type"`
			Error      json.RawMessage `json:"error"`
			Part       json.RawMessage `json:"part"`
			Properties struct {
				Part json.RawMessage `json:"part"`
			} `json:"properties"`
		}
		decoder := json.NewDecoder(bytes.NewReader(line))
		if decoder.Decode(&event) != nil ||
			decoder.Decode(&struct{}{}) != io.EOF || event.Type == "" {
			return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
		}
		switch event.Type {
		case "step_finish", "session.idle":
			completed = true
		case "auth.error":
			return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationAuth
		case "error", "session.error":
			return OpenCodeControlHarnessCompletion{}, classifyOpenCodeEventError(event.Error)
		}
		partRaw := event.Part
		if len(partRaw) == 0 {
			partRaw = event.Properties.Part
		}
		if len(partRaw) == 0 {
			if event.Type == "tool" || event.Type == "tool_call" ||
				event.Type == "tool_use" {
				return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
			}
			continue
		}
		var part struct {
			Type  string `json:"type"`
			Text  string `json:"text"`
			Tool  string `json:"tool"`
			State struct {
				Status string          `json:"status"`
				Input  json.RawMessage `json:"input"`
			} `json:"state"`
		}
		partDecoder := json.NewDecoder(bytes.NewReader(partRaw))
		if partDecoder.Decode(&part) != nil ||
			partDecoder.Decode(&struct{}{}) != io.EOF {
			return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
		}
		if part.Type == "text" {
			if !utf8.ValidString(part.Text) || strings.IndexByte(part.Text, 0) >= 0 ||
				directContent.Len()+len(part.Text) > maximum {
				return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
			}
			directContent.WriteString(part.Text)
			continue
		}
		if part.Type != "tool" {
			if event.Type == "tool" || event.Type == "tool_call" ||
				event.Type == "tool_use" {
				return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
			}
			continue
		}
		if part.Tool == "" || len(part.Tool) > 256 ||
			!utf8.ValidString(part.Tool) || strings.IndexByte(part.Tool, 0) >= 0 ||
			part.State.Status != "completed" {
			return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
		}
		if part.Tool != "StructuredOutput" {
			if len(completion.CompletedToolNames) >= MaxConversationControlCalls {
				return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
			}
			completion.CompletedToolNames = append(
				completion.CompletedToolNames, part.Tool,
			)
			continue
		}
		if structuredSeen {
			return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
		}
		selected, content, err := decodeOpenCodeStructuredControlInput(
			part.State.Input, maximum,
		)
		if err != nil {
			return OpenCodeControlHarnessCompletion{}, err
		}
		completion.Content = content
		completion.SelectedTool = selected
		completion.Structured = true
		structuredSeen = true
	}
	if scanner.Err() != nil || !completed {
		return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
	}
	if structuredSeen {
		if len(completion.Content) == 0 || strings.TrimSpace(directContent.String()) != "" {
			return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
		}
		return completion, nil
	}
	content := strings.TrimSpace(directContent.String())
	if len(completion.CompletedToolNames) == 0 || content == "" ||
		len(content) > maximum || !utf8.ValidString(content) ||
		strings.IndexByte(content, 0) >= 0 {
		return OpenCodeControlHarnessCompletion{}, ErrOpenCodeConversationUnavailable
	}
	completion.Content = []byte(content)
	return completion, nil
}

func decodeOpenCodeStructuredControlInput(
	raw json.RawMessage,
	maximum int,
) (*OpenCodeControlToolSelection, []byte, error) {
	if len(raw) == 0 || len(raw) > maximum || !utf8.Valid(raw) ||
		bytes.IndexByte(raw, 0) >= 0 || rejectConversationDuplicateJSONKeys(raw) {
		return nil, nil, ErrOpenCodeConversationUnavailable
	}
	var input struct {
		Response      string          `json:"response"`
		ToolName      string          `json:"tool_name"`
		ToolArguments json.RawMessage `json:"tool_arguments"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, nil, ErrOpenCodeConversationUnavailable
	}
	response := strings.TrimSpace(input.Response)
	if response == "" || len(response) > maximum || !utf8.ValidString(response) ||
		strings.IndexByte(response, 0) >= 0 {
		return nil, nil, ErrOpenCodeConversationUnavailable
	}
	arguments, err := validateConversationControlArguments(input.ToolArguments)
	if err != nil {
		return nil, nil, ErrOpenCodeConversationUnavailable
	}
	if input.ToolName == "none" {
		var empty map[string]any
		argumentDecoder := json.NewDecoder(bytes.NewReader(arguments))
		argumentDecoder.UseNumber()
		if argumentDecoder.Decode(&empty) != nil || len(empty) != 0 ||
			argumentDecoder.Decode(&struct{}{}) != io.EOF {
			return nil, nil, ErrOpenCodeConversationUnavailable
		}
		return nil, []byte(response), nil
	}
	if !strings.HasPrefix(input.ToolName, "loom_") ||
		!validConversationControlName(input.ToolName) {
		return nil, nil, ErrOpenCodeConversationUnavailable
	}
	return &OpenCodeControlToolSelection{
		Name: input.ToolName, Arguments: arguments,
	}, []byte(response), nil
}

// decodeOpenCodeConversation parses the `opencode run --format json` newline-
// delimited event stream and returns the final assistant text. The schema is
// grounded in the installed OpenCode 1.18.3 CLI output: `text` events carry
// `part.type == "text"` with `part.text`; `step_finish` and `session.idle`
// mark completion. Top-level `error` events carry the exact 1.18.3
// `ProviderAuthError` or `APIError` union; unknown unions fail closed. The
// older `message.part.updated` / `properties.part` shape is also accepted.
func decodeOpenCodeConversation(payload []byte, maximum int) ([]byte, error) {
	return decodeOpenCodeEventStream(payload, maximum, true, false)
}

func decodeOpenCodeEventStream(
	payload []byte,
	maximum int,
	rejectToolEvents bool,
	allowCompletedToolOnly bool,
) ([]byte, error) {
	if len(payload) == 0 || len(payload) > maximum || !utf8.Valid(payload) ||
		bytes.IndexByte(payload, 0) >= 0 {
		return nil, ErrOpenCodeConversationUnavailable
	}
	var content strings.Builder
	completed := false
	toolActivity := false
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 4096), maximum)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event struct {
			Type  string          `json:"type"`
			Error json.RawMessage `json:"error"`
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
		case "tool", "tool_call", "tool_use":
			if rejectToolEvents {
				return nil, ErrOpenCodeConversationUnavailable
			}
			toolActivity = true
		case "step_finish", "session.idle":
			completed = true
		case "auth.error":
			return nil, ErrOpenCodeConversationAuth
		case "error", "session.error":
			return nil, classifyOpenCodeEventError(event.Error)
		}
		if rejectToolEvents && partType != "" && partType != "text" {
			return nil, ErrOpenCodeConversationUnavailable
		}
		if !rejectToolEvents && partType != "" && partType != "text" {
			toolActivity = true
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
	if response == "" && allowCompletedToolOnly && toolActivity {
		return []byte{}, nil
	}
	if response == "" || len(response) > maximum {
		return nil, ErrOpenCodeConversationUnavailable
	}
	return []byte(response), nil
}

type openCodeConversationFailure struct {
	sentinel error
	details  error
}

func (failure *openCodeConversationFailure) Error() string {
	return failure.sentinel.Error()
}

func (failure *openCodeConversationFailure) Unwrap() []error {
	return []error{failure.sentinel, failure.details}
}

func newOpenCodeConversationFailure(
	sentinel error,
	stage string,
	code string,
	retryable bool,
) error {
	return &openCodeConversationFailure{
		sentinel: sentinel,
		details:  newConversationFailure(stage, code, retryable),
	}
}

func newOpenCodeConversationHTTPFailure(
	sentinel error,
	stage string,
	code string,
	status int,
	retryable bool,
) error {
	return &openCodeConversationFailure{
		sentinel: sentinel,
		details: newConversationHTTPFailure(
			stage, code, status, "", retryable, 0,
		),
	}
}

func classifyOpenCodeEventError(raw json.RawMessage) error {
	var envelope struct {
		Name string          `json:"name"`
		Data json.RawMessage `json:"data"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &envelope) != nil ||
		!openCodeErrorDataObject(envelope.Data) {
		return ErrOpenCodeConversationUnavailable
	}
	switch envelope.Name {
	case "ProviderAuthError":
		var data struct {
			ProviderID string `json:"providerID"`
		}
		if json.Unmarshal(envelope.Data, &data) != nil ||
			!validOpenCodeProviderPart(data.ProviderID) {
			return ErrOpenCodeConversationUnavailable
		}
		return newOpenCodeConversationFailure(
			ErrOpenCodeConversationAuth,
			"provider_auth",
			"provider_auth",
			false,
		)
	case "APIError":
		return classifyOpenCodeAPIError(envelope.Data)
	default:
		return ErrOpenCodeConversationUnavailable
	}
}

func openCodeErrorDataObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return false
	}
	var object struct{}
	return json.Unmarshal(trimmed, &object) == nil
}

func classifyOpenCodeAPIError(raw json.RawMessage) error {
	var data struct {
		StatusCode  int    `json:"statusCode"`
		IsRetryable *bool  `json:"isRetryable"`
		Message     string `json:"message"`
	}
	if json.Unmarshal(raw, &data) != nil || data.IsRetryable == nil ||
		data.StatusCode < 100 || data.StatusCode > 599 {
		return ErrOpenCodeConversationUnavailable
	}
	status := data.StatusCode
	switch {
	case status == 401 || status == 403:
		return newOpenCodeConversationHTTPFailure(
			ErrOpenCodeConversationAuth,
			"provider_auth",
			"provider_auth",
			status,
			false,
		)
	case status == 402:
		return newOpenCodeConversationHTTPFailure(
			ErrOpenCodeConversationInsufficientBalance,
			"provider_http",
			"provider_insufficient_balance",
			status,
			false,
		)
	case status == 404:
		return newOpenCodeConversationHTTPFailure(
			ErrOpenCodeConversationModelUnavailable,
			"provider_http",
			"provider_model_unavailable",
			status,
			false,
		)
	case status == 429:
		return newOpenCodeConversationHTTPFailure(
			ErrOpenCodeConversationRateLimit,
			"provider_rate_limit",
			"provider_rate_limit",
			status,
			*data.IsRetryable,
		)
	case status == 400:
		// OpenCode Console reports retired or removed hosted models as HTTP 400.
		// Inspect only a bounded message for this closed classification and never
		// retain the Provider text in the returned error.
		lowerMessage := strings.ToLower(data.Message)
		if len(lowerMessage) <= 1024 &&
			(strings.Contains(lowerMessage, "model is unavailable") ||
				strings.Contains(lowerMessage, "model not found")) {
			return newOpenCodeConversationHTTPFailure(
				ErrOpenCodeConversationModelUnavailable,
				"provider_http",
				"provider_model_unavailable",
				status,
				false,
			)
		}
		return newOpenCodeConversationHTTPFailure(
			ErrOpenCodeConversationUnavailable,
			"provider_http",
			"provider_invalid_request",
			status,
			false,
		)
	case status >= 500:
		return newOpenCodeConversationHTTPFailure(
			ErrOpenCodeConversationUnavailable,
			"provider_http",
			"provider_unavailable",
			status,
			*data.IsRetryable,
		)
	default:
		return ErrOpenCodeConversationUnavailable
	}
}

func classifiedOpenCodeConversationFailure(err error) bool {
	if _, ok := ConversationFailureDetails(err); ok {
		return true
	}
	return errors.Is(err, ErrOpenCodeConversationAuth) ||
		errors.Is(err, ErrOpenCodeConversationInsufficientBalance) ||
		errors.Is(err, ErrOpenCodeConversationModelUnavailable) ||
		errors.Is(err, ErrOpenCodeConversationRateLimit)
}
