package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/toolbroker"
)

const productMCPStdioTestHelperArgument = "loom-product-mcp-stdio-test-helper"

func TestProductMCPStdioConfigAcceptsExactPrivateVersionedFile(t *testing.T) {
	root := productMCPStdioTestRoot(t)
	executable := productMCPStdioTestExecutable(t)
	productMCPStdioWriteTestConfig(t, root, fmt.Sprintf(
		`{"version":1,"servers":[{"id":"review-tools","executable":%q,"args":["-test.run=^TestProductMCPStdioHelperProcess$","--","%s"]}]}`,
		executable, productMCPStdioTestHelperArgument,
	))

	servers, err := readProductMCPStdioConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].ID != "review-tools" ||
		servers[0].Executable != executable || len(servers[0].Args) != 3 {
		t.Fatalf("servers = %#v", servers)
	}
}

func TestProductMCPStdioConfigRejectsUntrustedOrAmbiguousInput(t *testing.T) {
	executable := productMCPStdioTestExecutable(t)
	tests := []struct {
		name    string
		payload func(string) string
		mutate  func(*testing.T, string)
	}{
		{
			name: "unsupported version",
			payload: func(path string) string {
				return fmt.Sprintf(`{"version":2,"servers":[{"id":"review","executable":%q}]}`, path)
			},
		},
		{
			name: "duplicate json key",
			payload: func(path string) string {
				return fmt.Sprintf(`{"version":1,"version":1,"servers":[{"id":"review","executable":%q}]}`, path)
			},
		},
		{
			name: "duplicate server",
			payload: func(path string) string {
				return fmt.Sprintf(`{"version":1,"servers":[{"id":"review","executable":%q},{"id":"review","executable":%q}]}`, path, path)
			},
		},
		{
			name: "invalid server",
			payload: func(path string) string {
				return fmt.Sprintf(`{"version":1,"servers":[{"id":"../review","executable":%q}]}`, path)
			},
		},
		{
			name: "relative executable",
			payload: func(string) string {
				return `{"version":1,"servers":[{"id":"review","executable":"bin/server"}]}`
			},
		},
		{
			name: "unbounded args",
			payload: func(path string) string {
				arguments := make([]string, productMCPStdioMaximumArgs+1)
				for index := range arguments {
					arguments[index] = "bounded"
				}
				encoded, err := json.Marshal(arguments)
				if err != nil {
					t.Fatal(err)
				}
				return fmt.Sprintf(`{"version":1,"servers":[{"id":"review","executable":%q,"args":%s}]}`, path, encoded)
			},
		},
		{
			name: "environment injection",
			payload: func(path string) string {
				return fmt.Sprintf(`{"version":1,"servers":[{"id":"review","executable":%q,"env":{"TOKEN":"secret"}}]}`, path)
			},
		},
		{
			name: "non private file",
			payload: func(path string) string {
				return fmt.Sprintf(`{"version":1,"servers":[{"id":"review","executable":%q}]}`, path)
			},
			mutate: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Chmod(productMCPStdioConfigPath(root), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := productMCPStdioTestRoot(t)
			productMCPStdioWriteTestConfig(t, root, test.payload(executable))
			if test.mutate != nil {
				test.mutate(t, root)
			}
			if _, err := readProductMCPStdioConfig(root); !errors.Is(err, errProductMCPStdioConfig) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestProductMCPStdioRegistryInitializesCallsWithoutEnvironmentAndClosesChild(t *testing.T) {
	t.Setenv("LOOM_TEST_SECRET", "must-not-reach-child")
	root := productMCPStdioTestRoot(t)
	exitMarker := filepath.Join(root, "helper-exited")
	productMCPStdioWriteHelperConfig(t, root, "review-tools", exitMarker)

	registry, err := newProductMCPStdioRegistry(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if registry == nil || len(registry.Clients()) != 1 {
		t.Fatalf("registry = %#v", registry)
	}
	clientRegistry, err := toolbroker.NewMCPRegistry(registry.Clients())
	if err != nil {
		t.Fatal(err)
	}
	content, err := clientRegistry.CallTool(
		context.Background(), "review-tools", "echo", json.RawMessage(`{"value":"hello"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if content != "initialized=true env=0 secret= value=hello" {
		t.Fatalf("content = %q", content)
	}
	if err := registry.Close(); err != nil {
		t.Fatal(err)
	}
	productMCPStdioWaitForFile(t, exitMarker)
	if err := registry.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestProductMCPStdioRegistryKeepsInflightCallBlockedUntilRelease(t *testing.T) {
	root := productMCPStdioTestRoot(t)
	exitMarker := filepath.Join(root, "helper-exited")
	startedMarker := filepath.Join(root, "call-started")
	releaseMarker := filepath.Join(root, "call-release")
	productMCPStdioWriteTestConfig(t, root, fmt.Sprintf(
		`{"version":1,"servers":[{"id":"review-tools","executable":%q,"args":["-test.run=^TestProductMCPStdioHelperProcess$","--","%s",%q,%q,%q]}]}`,
		productMCPStdioTestExecutable(t), productMCPStdioTestHelperArgument,
		exitMarker, startedMarker, releaseMarker,
	))
	registry, err := newProductMCPStdioRegistry(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = registry.Close() }()
	clientRegistry, err := toolbroker.NewMCPRegistry(registry.Clients())
	if err != nil {
		t.Fatal(err)
	}
	type callResult struct {
		content string
		err     error
	}
	result := make(chan callResult, 1)
	go func() {
		content, callErr := clientRegistry.CallTool(
			context.Background(), "review-tools", "echo",
			json.RawMessage(`{"value":"wait-for-revoke"}`),
		)
		result <- callResult{content: content, err: callErr}
	}()
	productMCPStdioWaitForFile(t, startedMarker)
	select {
	case got := <-result:
		t.Fatalf("call completed before release: %#v", got)
	default:
	}
	if err := os.WriteFile(releaseMarker, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-result:
		if got.err != nil || !strings.Contains(got.content, "value=wait-for-revoke") {
			t.Fatalf("released call = %#v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("call did not complete after release")
	}
}

func TestProductMCPStdioRegistryContainsFailedServerAndKeepsPeer(t *testing.T) {
	root := productMCPStdioTestRoot(t)
	executable := productMCPStdioTestExecutable(t)
	exitMarker := filepath.Join(root, "peer-exited")
	productMCPStdioWriteTestConfig(t, root, fmt.Sprintf(
		`{"version":1,"servers":[{"id":"broken","executable":"/usr/bin/false"},{"id":"peer","executable":%q,"args":["-test.run=^TestProductMCPStdioHelperProcess$","--","%s",%q]}]}`,
		executable, productMCPStdioTestHelperArgument, exitMarker,
	))

	registry, err := newProductMCPStdioRegistry(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = registry.Close() }()
	clients := registry.Clients()
	if len(clients) != 1 || clients["peer"] == nil || clients["broken"] != nil {
		t.Fatalf("clients = %#v", clients)
	}

	executionConfig := missionExecutionConfigFromDaemonBuildWithMCP(
		daemonBuildConfig{Observer: app.LocalRuntimeObservationDaemonConfig{
			RuntimeInstanceID: "runtime-1", RuntimeSearchPaths: []string{"/opt/runtime"},
		}},
		registry,
	)
	if executionConfig == nil || executionConfig.RemoteToolBroker == nil {
		t.Fatalf("execution config = %#v", executionConfig)
	}
	brokerConfig := executionConfig.RemoteToolBroker
	if len(brokerConfig.MCPClients) != 1 || brokerConfig.MCPClients["peer"] == nil ||
		len(brokerConfig.MCPAllowlist) != 0 || !brokerConfig.EnrollmentOnly {
		t.Fatalf("broker config = %#v", brokerConfig)
	}
}

func TestProductMCPStdioStartupFailuresAreStagedCorrelatedAndPrivacySafe(t *testing.T) {
	const secret = "mcp-secret-must-not-enter-diagnostics"
	tests := []struct {
		name      string
		stage     string
		errorCode string
		retryable bool
		prepare   func(*testing.T, string)
	}{
		{
			name:      "malformed config",
			stage:     "mcp_config",
			errorCode: "mcp_config_invalid",
			prepare: func(t *testing.T, root string) {
				productMCPStdioWriteTestConfig(t, root, `{"version":1,"secret":"`+secret+`"`)
			},
		},
		{
			name:      "non-private config",
			stage:     "mcp_config",
			errorCode: "mcp_config_invalid",
			prepare: func(t *testing.T, root string) {
				productMCPStdioWriteTestConfig(t, root, fmt.Sprintf(
					`{"version":1,"servers":[{"id":"review","executable":"/usr/bin/false","args":[%q]}]}`,
					secret,
				))
				if err := os.Chmod(productMCPStdioConfigPath(root), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:      "all server initialization failed",
			stage:     "mcp_init",
			errorCode: "mcp_init_failed",
			retryable: true,
			prepare: func(t *testing.T, root string) {
				productMCPStdioWriteTestConfig(t, root, fmt.Sprintf(
					`{"version":1,"servers":[{"id":"broken","executable":"/usr/bin/false","args":[%q]}]}`,
					secret,
				))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := productMCPStdioTestRoot(t)
			test.prepare(t, root)

			registry, diagnostic := prepareProductMCPStdioStartup(
				context.Background(), root,
			)
			if registry != nil {
				_ = registry.Close()
				t.Fatal("failed startup published an MCP registry")
			}
			if diagnostic == nil || diagnostic.Stage != test.stage ||
				diagnostic.ErrorCode != test.errorCode ||
				diagnostic.IncidentID == "" || diagnostic.Result != "failed" ||
				diagnostic.Operation != "mcp_stdio_startup" ||
				diagnostic.Retryable != test.retryable {
				t.Fatalf("diagnostic = %#v", diagnostic)
			}
			encoded, err := json.Marshal(diagnostic)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{
				secret, root, productMCPStdioConfigPath(root),
				`"args"`, `"env"`, `"content"`, `"executable"`,
			} {
				if bytes.Contains(encoded, []byte(forbidden)) {
					t.Fatalf("diagnostic disclosed %q: %s", forbidden, encoded)
				}
			}

			delegate := &fakeDaemonRunner{}
			runner := newProductMCPStdioDaemonRunnerWithDiagnostics(
				delegate, registry, diagnostic,
			)
			if _, err := runner.Run(context.Background()); err != nil {
				t.Fatalf("isolated daemon run: %v", err)
			}
			if err := runner.Close(); err != nil {
				t.Fatalf("isolated daemon close: %v", err)
			}
			if delegate.runCalls != 1 || delegate.closeCalls != 1 {
				t.Fatalf(
					"isolated daemon calls run=%d close=%d",
					delegate.runCalls, delegate.closeCalls,
				)
			}
		})
	}
}

func TestProductionDaemonBuilderIsolatesMCPStartupFailures(t *testing.T) {
	tests := []struct {
		name    string
		stage   string
		prepare func(*testing.T, string)
	}{
		{
			name:  "malformed config",
			stage: "mcp_config",
			prepare: func(t *testing.T, root string) {
				productMCPStdioWriteTestConfig(t, root, `{"version":1`)
			},
		},
		{
			name:  "non-private config",
			stage: "mcp_config",
			prepare: func(t *testing.T, root string) {
				productMCPStdioWriteTestConfig(
					t, root,
					`{"version":1,"servers":[{"id":"review","executable":"/usr/bin/false"}]}`,
				)
				if err := os.Chmod(productMCPStdioConfigPath(root), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:  "all server initialization failed",
			stage: "mcp_init",
			prepare: func(t *testing.T, root string) {
				productMCPStdioWriteTestConfig(
					t, root,
					`{"version":1,"servers":[{"id":"broken","executable":"/usr/bin/false"}]}`,
				)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, statePath := productDaemonFailureState(t)
			isolationRoot := filepath.Join(root, "isolation")
			runtimeRoot := filepath.Join(root, "runtime")
			for _, directory := range []string{isolationRoot, runtimeRoot} {
				if err := os.Mkdir(directory, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			test.prepare(t, isolationRoot)

			runner, err := productionDaemonBuilder(daemonBuildConfig{
				Observer: app.LocalRuntimeObservationDaemonConfig{
					StatePath: statePath, IsolationRoot: isolationRoot,
					RuntimeSearchPaths: []string{runtimeRoot}, ProbeID: "mcp-failure-probe",
					RuntimeInstanceID: "mcp-failure-runtime", DeviceID: "mcp-failure-device",
					DisplayName: "MCP failure", ObservationInterval: time.Hour,
					ProcessTimeout: time.Second, MaxCycles: 1,
				},
				SocketPath: filepath.Join(root, "loomd.sock"),
			})
			if err != nil || runner == nil {
				t.Fatalf("isolated production build runner=%T err=%v", runner, err)
			}
			defer func() {
				if err := runner.Close(); err != nil {
					t.Errorf("close production runner: %v", err)
				}
			}()
			source, ok := runner.(daemonStartupOperationalDiagnosticSource)
			if !ok {
				t.Fatalf("production runner %T has no startup diagnostics", runner)
			}
			diagnostics := source.StartupOperationalDiagnostics()
			if len(diagnostics) != 1 || diagnostics[0].Stage != test.stage ||
				diagnostics[0].IncidentID == "" {
				t.Fatalf("diagnostics = %#v", diagnostics)
			}
		})
	}
}

func TestProductMCPStdioDefaultHasNoCapability(t *testing.T) {
	root := productMCPStdioTestRoot(t)
	registry, err := newProductMCPStdioRegistry(context.Background(), root)
	if err != nil || registry != nil {
		t.Fatalf("default registry = %#v, %v", registry, err)
	}
	config := missionExecutionConfigFromDaemonBuild(daemonBuildConfig{
		Observer: app.LocalRuntimeObservationDaemonConfig{
			RuntimeInstanceID: "runtime-1", RuntimeSearchPaths: []string{"/opt/runtime"},
		},
	})
	if config == nil || config.RemoteToolBroker == nil ||
		len(config.RemoteToolBroker.MCPClients) != 0 {
		t.Fatalf("default execution config = %#v", config)
	}
}

func TestProductMCPStdioDaemonRunnerClosesRegistry(t *testing.T) {
	root := productMCPStdioTestRoot(t)
	exitMarker := filepath.Join(root, "runner-helper-exited")
	productMCPStdioWriteHelperConfig(t, root, "runner-peer", exitMarker)
	registry, err := newProductMCPStdioRegistry(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	delegate := &fakeDaemonRunner{}
	runner := newProductMCPStdioDaemonRunner(delegate, registry)
	if runner == nil {
		t.Fatal("missing wrapped runner")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	if delegate.closeCalls != 1 {
		t.Fatalf("delegate close calls = %d", delegate.closeCalls)
	}
	productMCPStdioWaitForFile(t, exitMarker)
}

func TestPrepareInstalledMCPMissionFixture(t *testing.T) {
	if os.Getenv("LOOM_PREPARE_INSTALLED_MCP_FIXTURE") != "1" {
		t.Skip("installed MCP fixture preparation requires explicit opt-in")
	}
	root := os.Getenv("LOOM_LIVE_MCP_ISOLATION_ROOT")
	helper := os.Getenv("LOOM_LIVE_MCP_HELPER")
	startedMarker := os.Getenv("LOOM_LIVE_MCP_STARTED_MARKER")
	releaseMarker := os.Getenv("LOOM_LIVE_MCP_RELEASE_MARKER")
	exitMarker := os.Getenv("LOOM_LIVE_MCP_EXIT_MARKER")
	if !productMCPStdioPrivateDirectory(root) {
		t.Fatalf("installed MCP isolation root is not private: %q", root)
	}
	if _, err := productMCPStdioValidateExecutable(helper); err != nil {
		t.Fatalf("installed MCP helper is not trusted: %v", err)
	}
	for name, path := range map[string]string{
		"started": startedMarker,
		"release": releaseMarker,
		"exit":    exitMarker,
	} {
		if !productMCPStdioFixtureMarker(root, path) {
			t.Fatalf("installed MCP %s marker is outside isolation root: %q", name, path)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	payload, err := json.Marshal(productMCPStdioFile{
		Version: 1,
		Servers: []productMCPStdioServerConfig{{
			ID:         "mcp.live.probe",
			Executable: helper,
			Args: []string{
				"-test.run=^TestProductMCPStdioHelperProcess$",
				"--",
				productMCPStdioTestHelperArgument,
				exitMarker,
				startedMarker,
				releaseMarker,
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := productMCPStdioConfigPath(root)
	temporary := filepath.Join(root, fmt.Sprintf(".%s.prepare-%d", productMCPStdioConfigFilename, os.Getpid()))
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(temporary)
		}
	}()
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(temporary, path); err != nil {
		t.Fatal(err)
	}
	cleanup = false
	directory, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		t.Fatal(err)
	}
	if err := directory.Close(); err != nil {
		t.Fatal(err)
	}
	servers, err := readProductMCPStdioConfig(root)
	if err != nil || len(servers) != 1 || servers[0].ID != "mcp.live.probe" {
		t.Fatalf("installed MCP fixture verification servers=%#v err=%v", servers, err)
	}
}

func productMCPStdioFixtureMarker(root string, path string) bool {
	return root != "" && path != "" && filepath.IsAbs(path) &&
		filepath.Clean(path) == path && filepath.Dir(path) == root
}

func TestProductMCPStdioHelperProcess(t *testing.T) {
	if !productMCPStdioHasArgument(productMCPStdioTestHelperArgument) {
		return
	}
	exitMarker := ""
	startedMarker := ""
	releaseMarker := ""
	for index, value := range os.Args {
		if value == productMCPStdioTestHelperArgument && index+1 < len(os.Args) {
			exitMarker = os.Args[index+1]
			if index+2 < len(os.Args) {
				startedMarker = os.Args[index+2]
			}
			if index+3 < len(os.Args) {
				releaseMarker = os.Args[index+3]
			}
		}
	}
	defer func() {
		if exitMarker != "" {
			_ = os.WriteFile(exitMarker, []byte("closed"), 0o600)
		}
	}()
	initialized := false
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			return
		}
		switch request.Method {
		case "initialize":
			response := fmt.Sprintf(
				`{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":%q,"capabilities":{"tools":{}},"serverInfo":{"name":"loom-test-helper","version":"1"}}}`,
				request.ID, mcp.LATEST_PROTOCOL_VERSION,
			)
			fmt.Fprintln(os.Stdout, response)
		case "notifications/initialized":
			initialized = true
		case "tools/call":
			value, _ := request.Params.Arguments["value"].(string)
			if value == "wait-for-revoke" && startedMarker != "" && releaseMarker != "" {
				if err := os.WriteFile(startedMarker, []byte("started"), 0o600); err != nil {
					return
				}
				deadline := time.Now().Add(2 * time.Minute)
				for {
					if _, err := os.Stat(releaseMarker); err == nil {
						break
					}
					if time.Now().After(deadline) {
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
			text := fmt.Sprintf(
				"initialized=%t env=%d secret=%s value=%s",
				initialized, len(os.Environ()), os.Getenv("LOOM_TEST_SECRET"), value,
			)
			encoded, _ := json.Marshal(text)
			fmt.Fprintf(
				os.Stdout,
				`{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":%s}]}}`+"\n",
				request.ID, encoded,
			)
		}
	}
}

func productMCPStdioTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func productMCPStdioTestExecutable(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func productMCPStdioWriteHelperConfig(t *testing.T, root, id, exitMarker string) {
	t.Helper()
	productMCPStdioWriteTestConfig(t, root, fmt.Sprintf(
		`{"version":1,"servers":[{"id":%q,"executable":%q,"args":["-test.run=^TestProductMCPStdioHelperProcess$","--","%s",%q]}]}`,
		id, productMCPStdioTestExecutable(t), productMCPStdioTestHelperArgument, exitMarker,
	))
}

func productMCPStdioWriteTestConfig(t *testing.T, root, payload string) {
	t.Helper()
	if err := os.WriteFile(productMCPStdioConfigPath(root), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
}

func productMCPStdioWaitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func productMCPStdioHasArgument(want string) bool {
	return bytes.Contains([]byte(strings.Join(os.Args, "\x00")), []byte(want))
}
