//go:build darwin

package localipc

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const swiftSnapshotFixture = `{
  "schema_version":1,
  "view_version":"view-1",
  "partial":false,
  "stale":false,
  "reason":"",
  "runtimes":[{
    "runtime_instance_id":"pi-local",
    "display_name":"Pi",
    "adapter_type":"pi",
    "executable_version":"0.82.1",
    "status":"online",
    "capacity":2,
    "model_ids":[],
    "observed_capabilities":[]
  }],
  "teams":[],
  "runs":[],
  "evidence":[],
  "attention":[],
  "runtime_page":{"next_cursor":"pi-local","has_more":false},
  "team_page":{"next_cursor":"","has_more":false},
  "run_page":{"next_cursor":"","has_more":false},
  "evidence_page":{"next_cursor":"","has_more":false}
}`

const swiftTimelineFixture = `{
  "schema_version":1,
  "team_instance_id":"team-1",
  "view_version":"view-1",
  "next_cursor":"",
  "has_more":false,
  "gap":null,
  "records":[],
  "board":{
    "schema_version":1,
    "team_instance_id":"team-1",
    "plan_digest":"",
    "status":"ready",
    "view_version":"view-1",
    "nodes":[],
    "cost":{"observed":false,"amount_microunits":null,"currency":""}
  },
  "attention":[]
}`

type swiftFixtureHandler struct {
	mu        sync.Mutex
	errorCode string
}

func (handler *swiftFixtureHandler) setErrorCode(code string) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	handler.errorCode = code
}

func (handler *swiftFixtureHandler) Handle(
	_ context.Context,
	request Request,
) Response {
	handler.mu.Lock()
	code := handler.errorCode
	handler.mu.Unlock()
	if code != "" {
		return Response{Error: safeProtocolError(code, errors.New("private"))}
	}
	switch request.Method {
	case "snapshot":
		return Response{OK: true, Result: json.RawMessage(swiftSnapshotFixture)}
	case "timeline_page":
		return Response{OK: true, Result: json.RawMessage(swiftTimelineFixture)}
	default:
		return Response{}
	}
}

func TestSwiftClientInteroperatesWithRealGoServer(t *testing.T) {
	probe := buildSwiftContractProbe(t)
	root, socketPath := swiftPrivateSocketRoot(t)
	defer os.RemoveAll(root)

	handler := &swiftFixtureHandler{}
	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "swift-contract-fixture",
		Handler:      handler,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	defer func() {
		_ = server.Close()
		select {
		case err := <-serveDone:
			if err != nil {
				t.Errorf("Serve() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("server did not close")
		}
	}()

	output, runErr := exec.Command(
		probe,
		"--socket",
		socketPath,
		"--team",
		"team-1",
	).CombinedOutput()
	if runErr != nil {
		t.Fatalf("Swift probe error = %v, output = %q", runErr, output)
	}
	var actual struct {
		Snapshot json.RawMessage `json:"snapshot"`
		Timeline json.RawMessage `json:"timeline"`
	}
	if err := json.Unmarshal(output, &actual); err != nil {
		t.Fatalf("probe output invalid: %v, output = %q", err, output)
	}
	assertJSONSemanticEqual(t, actual.Snapshot, []byte(swiftSnapshotFixture))
	assertJSONSemanticEqual(t, actual.Timeline, []byte(swiftTimelineFixture))

	for _, code := range []string{
		"invalid_request",
		"unsupported_version",
		"unknown_method",
		"unauthorized_peer",
		"unsupported_platform",
		"not_found",
		"cursor_conflict",
		"stream_gap",
		"state_unavailable",
		"timeout",
		"busy",
		"internal",
	} {
		handler.setErrorCode(code)
		output, runErr = exec.Command(
			probe,
			"--socket",
			socketPath,
		).CombinedOutput()
		if runErr == nil {
			t.Fatalf("Swift probe accepted %q error", code)
		}
		_, recoverable, ok := protocolErrorDefinition(code)
		if !ok {
			t.Fatalf("missing Go error definition for %q", code)
		}
		expected := "error:" + code + ":" +
			map[bool]string{true: "true", false: "false"}[recoverable]
		if strings.TrimSpace(string(output)) != expected {
			t.Fatalf(
				"Swift probe error output for %q = %q, want %q",
				code,
				output,
				expected,
			)
		}
	}
}

func TestSwiftClientRejectsMalformedLoopbackResponses(t *testing.T) {
	probe := buildSwiftContractProbe(t)
	cases := map[string]func(string) []byte{
		"null_model_ids": swiftInvalidSnapshotResponse(
			`"model_ids":[]`,
			`"model_ids":null`,
		),
		"null_observed_capabilities": swiftInvalidSnapshotResponse(
			`"observed_capabilities":[]`,
			`"observed_capabilities":null`,
		),
		"missing_model_ids": swiftInvalidSnapshotResponse(
			`    "model_ids":[],`+"\n",
			"",
		),
		"wrong_type_model_ids": swiftInvalidSnapshotResponse(
			`"model_ids":[]`,
			`"model_ids":"model-a"`,
		),
		"duplicate_model_ids": swiftInvalidSnapshotResponse(
			`"model_ids":[]`,
			`"model_ids":[],"model_ids":[]`,
		),
		"unknown_runtime_field": swiftInvalidSnapshotResponse(
			`"observed_capabilities":[]`,
			`"observed_capabilities":[],"extra":true`,
		),
		"duplicate": func(id string) []byte {
			return []byte(`{"version":1,"version":1,"request_id":"` + id +
				`","ok":true,"result":{},"error":null}`)
		},
		"unknown": func(id string) []byte {
			return []byte(`{"version":1,"request_id":"` + id +
				`","ok":true,"result":{},"error":null,"extra":1}`)
		},
		"trailing": func(id string) []byte {
			return []byte(`{"version":1,"request_id":"` + id +
				`","ok":true,"result":{},"error":null}x`)
		},
		"mismatched_id": func(string) []byte {
			return []byte(
				`{"version":1,"request_id":"other","ok":true,` +
					`"result":{},"error":null}`,
			)
		},
		"invalid_shape": func(id string) []byte {
			return []byte(`{"version":1,"request_id":"` + id +
				`","ok":true,"result":{},"error":` +
				`{"code":"internal","message":"internal error"}}`)
		},
	}
	for name, malformed := range cases {
		t.Run(name, func(t *testing.T) {
			root, socketPath := swiftPrivateSocketRoot(t)
			defer os.RemoveAll(root)
			done := startRawSwiftFixture(t, socketPath, func(
				id string,
				connection *net.UnixConn,
			) {
				writeRawFrame(t, connection, malformed(id))
			})
			output, err := exec.Command(
				probe,
				"--socket",
				socketPath,
			).CombinedOutput()
			if err == nil ||
				strings.TrimSpace(string(output)) != "error:invalid_response" {
				t.Fatalf("malformed response result = %v, %q", err, output)
			}
			<-done
		})
	}

	t.Run("oversized", func(t *testing.T) {
		root, socketPath := swiftPrivateSocketRoot(t)
		defer os.RemoveAll(root)
		done := startRawSwiftFixture(t, socketPath, func(
			_ string,
			connection *net.UnixConn,
		) {
			var length [4]byte
			binary.BigEndian.PutUint32(
				length[:],
				uint32(maxResponseBodyBytes+1),
			)
			if _, err := connection.Write(length[:]); err != nil {
				t.Errorf("write oversized length: %v", err)
			}
			_ = connection.Close()
		})
		output, err := exec.Command(
			probe,
			"--socket",
			socketPath,
		).CombinedOutput()
		if err == nil ||
			strings.TrimSpace(string(output)) != "error:invalid_response" {
			t.Fatalf("oversized response result = %v, %q", err, output)
		}
		<-done
	})
}

func swiftInvalidSnapshotResponse(
	old string,
	replacement string,
) func(string) []byte {
	return func(id string) []byte {
		snapshot := strings.Replace(
			swiftSnapshotFixture,
			old,
			replacement,
			1,
		)
		return []byte(
			`{"version":1,"request_id":"` + id +
				`","ok":true,"result":` + snapshot +
				`,"error":null}`,
		)
	}
}

func startRawSwiftFixture(
	t *testing.T,
	socketPath string,
	writeMalformed func(string, *net.UnixConn),
) <-chan struct{} {
	t.Helper()
	listener, err := net.ListenUnix(
		"unix",
		&net.UnixAddr{Name: socketPath, Net: "unix"},
	)
	if err != nil {
		t.Fatalf("ListenUnix() error = %v", err)
	}
	listener.SetUnlinkOnClose(true)
	if err := os.Chmod(socketPath, 0o600); err != nil {
		t.Fatalf("chmod socket: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer listener.Close()
		ping, err := listener.AcceptUnix()
		if err != nil {
			t.Errorf("accept ping: %v", err)
			return
		}
		pingBody, err := readFrame(ping, maxRequestBodyBytes)
		if err != nil {
			t.Errorf("read ping: %v", err)
			_ = ping.Close()
			return
		}
		pingID := requestIDFromUntrustedBody(pingBody)
		pingResult := json.RawMessage(
			`{"protocol_version":1,"available":true,"build_id":"raw-fixture"}`,
		)
		encoded, _ := encodeResponse(Response{
			Version:   protocolVersion,
			RequestID: pingID,
			OK:        true,
			Result:    pingResult,
		})
		if err := writeFrame(ping, encoded, maxResponseBodyBytes); err != nil {
			t.Errorf("write ping: %v", err)
		}
		_ = ping.Close()

		snapshot, err := listener.AcceptUnix()
		if err != nil {
			t.Errorf("accept snapshot: %v", err)
			return
		}
		body, err := readFrame(snapshot, maxRequestBodyBytes)
		if err != nil {
			t.Errorf("read snapshot: %v", err)
			_ = snapshot.Close()
			return
		}
		writeMalformed(requestIDFromUntrustedBody(body), snapshot)
	}()
	return done
}

func writeRawFrame(t *testing.T, connection *net.UnixConn, body []byte) {
	t.Helper()
	if err := writeFrame(connection, body, maxResponseBodyBytes); err != nil {
		t.Errorf("write malformed frame: %v", err)
	}
	_ = connection.Close()
}

func buildSwiftContractProbe(t *testing.T) string {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	packageRoot := filepath.Join(repoRoot, "apps", "macos")
	command := exec.Command(
		"/usr/bin/swift",
		"build",
		"--package-path",
		packageRoot,
		"-c",
		"release",
		"--arch",
		"arm64",
		"--product",
		"LoomLocalAppContractProbe",
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("swift build error = %v, output = %q", err, output)
	}
	command = exec.Command(
		"/usr/bin/swift",
		"build",
		"--package-path",
		packageRoot,
		"-c",
		"release",
		"--arch",
		"arm64",
		"--show-bin-path",
	)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("swift bin path error = %v", err)
	}
	return filepath.Join(
		strings.TrimSpace(string(output)),
		"LoomLocalAppContractProbe",
	)
}

func swiftPrivateSocketRoot(t *testing.T) (string, string) {
	t.Helper()
	root, err := os.MkdirTemp("/private/tmp", "loom-swift-ipc.")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		os.RemoveAll(root)
		t.Fatalf("chmod private root: %v", err)
	}
	return root, filepath.Join(root, "loomd.sock")
}

func assertJSONSemanticEqual(t *testing.T, actual, expected []byte) {
	t.Helper()
	var actualValue any
	var expectedValue any
	if err := json.Unmarshal(actual, &actualValue); err != nil {
		t.Fatalf("actual JSON invalid: %v", err)
	}
	if err := json.Unmarshal(expected, &expectedValue); err != nil {
		t.Fatalf("expected JSON invalid: %v", err)
	}
	actualBytes, _ := json.Marshal(actualValue)
	expectedBytes, _ := json.Marshal(expectedValue)
	if string(actualBytes) != string(expectedBytes) {
		t.Fatalf("JSON mismatch\nactual: %s\nexpected: %s", actualBytes, expectedBytes)
	}
}
