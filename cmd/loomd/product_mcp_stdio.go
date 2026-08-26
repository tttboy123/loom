package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/toolbroker"
)

const (
	productMCPStdioConfigFilename    = "mcp-stdio-v1.json"
	productMCPStdioMaximumBytes      = 64 << 10
	productMCPStdioMaximumServers    = 16
	productMCPStdioMaximumArgs       = 32
	productMCPStdioMaximumArgBytes   = 512
	productMCPStdioMaximumAllArgs    = 4 << 10
	productMCPStdioInitializeTimeout = 5 * time.Second
)

var errProductMCPStdioConfig = errors.New("invalid product MCP stdio config")
var errProductMCPStdioInitialize = errors.New("product MCP stdio initialization failed")

var productMCPStdioIncidentSequence atomic.Uint64

type productMCPStdioOperationalDiagnostic struct {
	SchemaVersion int    `json:"schema_version"`
	IncidentID    string `json:"incident_id"`
	Operation     string `json:"operation"`
	Stage         string `json:"stage"`
	Result        string `json:"result"`
	ErrorCode     string `json:"error_code"`
	Retryable     bool   `json:"retryable"`
}

func prepareProductMCPStdioStartup(
	ctx context.Context,
	isolationRoot string,
) (*productMCPStdioRegistry, *productMCPStdioOperationalDiagnostic) {
	registry, err := newProductMCPStdioRegistry(ctx, isolationRoot)
	if err == nil {
		return registry, nil
	}
	stage := "mcp_config"
	errorCode := "mcp_config_invalid"
	if errors.Is(err, errProductMCPStdioInitialize) {
		stage = "mcp_init"
		errorCode = "mcp_init_failed"
	}
	sequence := productMCPStdioIncidentSequence.Add(1)
	return nil, &productMCPStdioOperationalDiagnostic{
		SchemaVersion: 1,
		IncidentID: "loom-mcp-" + productDeterministicUUID(
			"product-mcp-stdio-startup",
			stage,
			time.Now().UTC().Format(time.RFC3339Nano),
			strconv.FormatUint(sequence, 10),
		),
		Operation: "mcp_stdio_startup",
		Stage:     stage,
		Result:    "failed",
		ErrorCode: errorCode,
		Retryable: stage == "mcp_init",
	}
}

type productMCPStdioFile struct {
	Version int                           `json:"version"`
	Servers []productMCPStdioServerConfig `json:"servers"`
}

type productMCPStdioServerConfig struct {
	ID         string   `json:"id"`
	Executable string   `json:"executable"`
	Args       []string `json:"args,omitempty"`

	executableInfo os.FileInfo
}

func productMCPStdioConfigPath(isolationRoot string) string {
	return filepath.Join(isolationRoot, productMCPStdioConfigFilename)
}

func readProductMCPStdioConfig(
	isolationRoot string,
) ([]productMCPStdioServerConfig, error) {
	if !productMCPStdioPrivateDirectory(isolationRoot) {
		return nil, errProductMCPStdioConfig
	}
	path := productMCPStdioConfigPath(isolationRoot)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errProductMCPStdioConfig
	}
	defer file.Close()
	info, err := file.Stat()
	linked, linkErr := os.Lstat(path)
	if err != nil || linkErr != nil || !os.SameFile(info, linked) ||
		!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 || !productMCPStdioOwnedByCurrentUser(info) ||
		info.Size() <= 0 || info.Size() > productMCPStdioMaximumBytes {
		return nil, errProductMCPStdioConfig
	}
	payload, err := io.ReadAll(io.LimitReader(file, productMCPStdioMaximumBytes+1))
	if err != nil || len(payload) > productMCPStdioMaximumBytes ||
		productMCPStdioRejectDuplicateJSONKeys(payload) != nil {
		return nil, errProductMCPStdioConfig
	}
	var decoded productMCPStdioFile
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&decoded) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		decoded.Version != 1 || decoded.Servers == nil ||
		len(decoded.Servers) > productMCPStdioMaximumServers {
		return nil, errProductMCPStdioConfig
	}
	seen := make(map[string]struct{}, len(decoded.Servers))
	for index := range decoded.Servers {
		server := &decoded.Servers[index]
		if !productMCPStdioValidIdentifier(server.ID) {
			return nil, errProductMCPStdioConfig
		}
		if _, duplicate := seen[server.ID]; duplicate {
			return nil, errProductMCPStdioConfig
		}
		seen[server.ID] = struct{}{}
		if !productMCPStdioValidArgs(server.Args) {
			return nil, errProductMCPStdioConfig
		}
		executableInfo, executableErr := productMCPStdioValidateExecutable(server.Executable)
		if executableErr != nil {
			return nil, errProductMCPStdioConfig
		}
		server.Args = append([]string(nil), server.Args...)
		server.executableInfo = executableInfo
	}
	return decoded.Servers, nil
}

func productMCPStdioPrivateDirectory(path string) bool {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 &&
		info.Mode().Perm() == 0o700 && productMCPStdioOwnedByCurrentUser(info)
}

func productMCPStdioOwnedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Geteuid()
}

func productMCPStdioValidateExecutable(path string) (os.FileInfo, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path ||
		strings.IndexByte(path, 0) >= 0 {
		return nil, errProductMCPStdioConfig
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path {
		return nil, errProductMCPStdioConfig
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm()&0o111 == 0 || info.Mode().Perm()&0o022 != 0 {
		return nil, errProductMCPStdioConfig
	}
	return info, nil
}

func productMCPStdioValidIdentifier(value string) bool {
	if value == "" || len(value) > 128 || value[0] == '.' || value[0] == '-' ||
		value[0] == '_' || value[len(value)-1] == '.' || value[len(value)-1] == '-' ||
		value[len(value)-1] == '_' {
		return false
	}
	previousSeparator := false
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			previousSeparator = false
		case (character == '.' || character == '-' || character == '_') && !previousSeparator:
			previousSeparator = true
		default:
			return false
		}
	}
	return true
}

func productMCPStdioValidArgs(arguments []string) bool {
	if len(arguments) > productMCPStdioMaximumArgs {
		return false
	}
	total := 0
	for _, argument := range arguments {
		total += len(argument)
		if argument == "" || len(argument) > productMCPStdioMaximumArgBytes ||
			total > productMCPStdioMaximumAllArgs || !utf8.ValidString(argument) ||
			strings.IndexByte(argument, 0) >= 0 {
			return false
		}
		for _, character := range argument {
			if character < 0x20 || character == 0x7f {
				return false
			}
		}
	}
	return true
}

func productMCPStdioRejectDuplicateJSONKeys(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := productMCPStdioWalkJSONValue(decoder); err != nil {
		return err
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) || token != nil {
		return errProductMCPStdioConfig
	}
	return nil
}

func productMCPStdioWalkJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			key, keyOK := keyToken.(string)
			if keyErr != nil || !keyOK {
				return errProductMCPStdioConfig
			}
			if _, duplicate := seen[key]; duplicate {
				return errProductMCPStdioConfig
			}
			seen[key] = struct{}{}
			if err := productMCPStdioWalkJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, closeErr := decoder.Token()
		if closeErr != nil || closing != json.Delim('}') {
			return errProductMCPStdioConfig
		}
	case '[':
		for decoder.More() {
			if err := productMCPStdioWalkJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, closeErr := decoder.Token()
		if closeErr != nil || closing != json.Delim(']') {
			return errProductMCPStdioConfig
		}
	default:
		return errProductMCPStdioConfig
	}
	return nil
}

type productMCPStdioRegistry struct {
	mu      sync.Mutex
	clients map[string]*mcpclient.Client
	closed  bool
}

func newProductMCPStdioRegistry(
	ctx context.Context,
	isolationRoot string,
) (*productMCPStdioRegistry, error) {
	if ctx == nil {
		return nil, errProductMCPStdioConfig
	}
	servers, err := readProductMCPStdioConfig(isolationRoot)
	if err != nil || len(servers) == 0 {
		return nil, err
	}
	registry := &productMCPStdioRegistry{clients: make(map[string]*mcpclient.Client)}
	for _, server := range servers {
		client, startErr := startProductMCPStdioClient(ctx, isolationRoot, server)
		if startErr != nil {
			continue
		}
		registry.clients[server.ID] = client
	}
	if len(registry.clients) == 0 {
		return nil, errProductMCPStdioInitialize
	}
	return registry, nil
}

func startProductMCPStdioClient(
	ctx context.Context,
	isolationRoot string,
	server productMCPStdioServerConfig,
) (*mcpclient.Client, error) {
	if ctx == nil || !productMCPStdioPrivateDirectory(isolationRoot) ||
		server.executableInfo == nil {
		return nil, errProductMCPStdioConfig
	}
	commandFactory := func(
		_ context.Context,
		command string,
		environment []string,
		arguments []string,
	) (*exec.Cmd, error) {
		current, err := productMCPStdioValidateExecutable(command)
		if err != nil || command != server.Executable || len(environment) != 0 ||
			!slices.Equal(arguments, server.Args) || !os.SameFile(current, server.executableInfo) {
			return nil, errProductMCPStdioConfig
		}
		child := exec.Command(command, arguments...)
		child.Env = []string{}
		child.Dir = isolationRoot
		return child, nil
	}
	transport := mcptransport.NewStdioWithOptions(
		server.Executable,
		[]string{},
		append([]string(nil), server.Args...),
		mcptransport.WithCommandFunc(commandFactory),
	)
	client := mcpclient.NewClient(transport)
	initializeContext, cancel := context.WithTimeout(ctx, productMCPStdioInitializeTimeout)
	defer cancel()
	if err := client.Start(initializeContext); err != nil {
		_ = client.Close()
		return nil, errProductMCPStdioConfig
	}
	result, err := client.Initialize(initializeContext, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "loomd", Version: "1"},
		},
	})
	if err != nil || result == nil || result.Capabilities.Tools == nil ||
		result.ServerInfo.Name == "" || len(result.ServerInfo.Name) > 128 ||
		result.ServerInfo.Version == "" || len(result.ServerInfo.Version) > 64 {
		_ = client.Close()
		return nil, errProductMCPStdioConfig
	}
	return client, nil
}

func (registry *productMCPStdioRegistry) Clients() map[string]toolbroker.MCPToolClient {
	if registry == nil {
		return nil
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.closed {
		return nil
	}
	clients := make(map[string]toolbroker.MCPToolClient, len(registry.clients))
	for id, client := range registry.clients {
		clients[id] = client
	}
	return clients
}

func (registry *productMCPStdioRegistry) Close() error {
	if registry == nil {
		return nil
	}
	registry.mu.Lock()
	if registry.closed {
		registry.mu.Unlock()
		return nil
	}
	registry.closed = true
	clients := registry.clients
	registry.clients = nil
	registry.mu.Unlock()
	var closeErr error
	for _, client := range clients {
		closeErr = errors.Join(closeErr, client.Close())
	}
	return closeErr
}

func applyProductMCPStdioClients(
	config *productRemoteToolBrokerConfig,
	clients map[string]toolbroker.MCPToolClient,
) {
	if config == nil || len(clients) == 0 {
		return
	}
	config.MCPClients = make(map[string]toolbroker.MCPToolClient, len(clients))
	for id, client := range clients {
		config.MCPClients[id] = client
	}
}

type productMCPStdioDaemonRunner struct {
	delegate   daemonRunner
	registry   *productMCPStdioRegistry
	diagnostic *productMCPStdioOperationalDiagnostic
	mu         sync.Mutex
	closed     bool
}

func newProductMCPStdioDaemonRunner(
	delegate daemonRunner,
	registry *productMCPStdioRegistry,
) daemonRunner {
	return newProductMCPStdioDaemonRunnerWithDiagnostics(delegate, registry, nil)
}

func newProductMCPStdioDaemonRunnerWithDiagnostics(
	delegate daemonRunner,
	registry *productMCPStdioRegistry,
	diagnostic *productMCPStdioOperationalDiagnostic,
) daemonRunner {
	if delegate == nil {
		return nil
	}
	if registry == nil && diagnostic == nil {
		return delegate
	}
	return &productMCPStdioDaemonRunner{
		delegate: delegate, registry: registry, diagnostic: diagnostic,
	}
}

func (runner *productMCPStdioDaemonRunner) StartupOperationalDiagnostics() []productMCPStdioOperationalDiagnostic {
	if runner == nil || runner.diagnostic == nil {
		return nil
	}
	return []productMCPStdioOperationalDiagnostic{*runner.diagnostic}
}

func (runner *productMCPStdioDaemonRunner) Run(
	ctx context.Context,
) (app.LocalRuntimeObservationDaemonResult, error) {
	return runner.delegate.Run(ctx)
}

func (runner *productMCPStdioDaemonRunner) Close() error {
	if runner == nil {
		return nil
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.closed {
		return nil
	}
	runner.closed = true
	return errors.Join(runner.delegate.Close(), runner.registry.Close())
}
