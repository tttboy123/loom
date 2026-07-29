package localipc

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServerRejectsOversizedWrongVersionAndTrailingRequestsWithoutLeak(t *testing.T) {
	if _, err := NewServer(ServerConfig{}); err == nil {
		t.Fatal("NewServer(invalid) error = nil")
	}
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "fixture-build",
		Handler: HandlerFunc(func(context.Context, Request) Response {
			t.Fatal("invalid request reached handler")
			return Response{}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if server.Ready() == nil {
		t.Fatal("Ready() channel = nil")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	waitForServerReady(t, server)

	for _, test := range []struct {
		payload  []byte
		wantCode string
	}{
		{
			payload:  []byte(`{"version":2,"request_id":"request-1","method":"ping","params":{}}`),
			wantCode: "unsupported_version",
		},
		{
			payload: append(
				[]byte(`{"version":1,"request_id":"request-1","method":"ping","params":{}}`),
				0,
			),
			wantCode: "invalid_request",
		},
	} {
		conn, err := net.DialTimeout("unix", socketPath, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeFrame(
			conn,
			test.payload,
			maxRequestBodyBytes,
		); err != nil {
			t.Fatal(err)
		}
		unixConnection, ok := conn.(*net.UnixConn)
		if !ok {
			t.Fatalf("connection type = %T", conn)
		}
		if err := unixConnection.CloseWrite(); err != nil {
			t.Fatal(err)
		}
		responseBytes, err := readFrame(conn, maxResponseBodyBytes)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(responseBytes, []byte(root)) ||
			bytes.Contains(responseBytes, []byte("payload")) {
			t.Fatalf("response leaked internals: %s", responseBytes)
		}
		var response Response
		if err := json.Unmarshal(responseBytes, &response); err != nil ||
			response.Error == nil ||
			response.Error.Code != test.wantCode {
			t.Fatalf(
				"response=%s error=%v, want code %q",
				responseBytes,
				err,
				test.wantCode,
			)
		}
		_ = conn.Close()
	}

	conn, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], maxRequestBodyBytes+1)
	if _, err := conn.Write(length[:]); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	responseBytes, err := readFrame(conn, maxResponseBodyBytes)
	if err == nil && !strings.Contains(string(responseBytes), "invalid_request") {
		t.Fatalf("oversized response = %s, err=%v", responseBytes, err)
	}
	_ = conn.Close()

	halfOpen, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFrame(
		halfOpen,
		[]byte(
			`{"version":1,"request_id":"request-half-open","method":"snapshot","params":{}}`,
		),
		maxRequestBodyBytes,
	); err != nil {
		t.Fatal(err)
	}
	// Deliberately do not CloseWrite. A complete request is exactly one frame
	// followed by EOF, so a half-open peer must fail closed before dispatch.
	_ = halfOpen.SetReadDeadline(time.Now().Add(time.Second))
	responseBytes, err = readFrame(halfOpen, maxResponseBodyBytes)
	if err != nil {
		t.Fatal(err)
	}
	var halfOpenResponse Response
	if err := json.Unmarshal(responseBytes, &halfOpenResponse); err != nil ||
		halfOpenResponse.Error == nil ||
		halfOpenResponse.Error.Code != "invalid_request" {
		t.Fatalf(
			"half-open response=%s error=%v",
			responseBytes,
			err,
		)
	}
	_ = halfOpen.Close()

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServerLifecycleRejectsCanceledClosedAndSecondServe(t *testing.T) {
	newFixture := func(t *testing.T) (*Server, string) {
		t.Helper()
		root := shortPrivateSocketRoot(t)
		socketPath := filepath.Join(root, "loomd.sock")
		server, err := NewServer(ServerConfig{
			SocketPath:   socketPath,
			EffectiveUID: os.Geteuid(),
			BuildID:      "fixture-build",
			Handler: HandlerFunc(func(
				context.Context,
				Request,
			) Response {
				return Response{}
			}),
		})
		if err != nil {
			t.Fatal(err)
		}
		return server, socketPath
	}

	canceledServer, _ := newFixture(t)
	canceled, cancelCanceled := context.WithCancel(context.Background())
	cancelCanceled()
	if err := canceledServer.Serve(canceled); !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf("pre-canceled Serve() error = %v", err)
	}

	closedServer, _ := newFixture(t)
	if err := closedServer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := closedServer.Serve(context.Background()); !errors.Is(
		err,
		ErrInvalidProtocol,
	) {
		t.Fatalf("closed Serve() error = %v", err)
	}

	activeServer, _ := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- activeServer.Serve(ctx) }()
	waitForServerReady(t, activeServer)
	if err := activeServer.Serve(context.Background()); !errors.Is(
		err,
		ErrInvalidProtocol,
	) {
		t.Fatalf("second Serve() error = %v", err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServerReclaimsOnlyStaleSocketAndPreservesReplacement(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	stale, err := net.ListenUnix(
		"unix",
		&net.UnixAddr{Name: socketPath, Net: "unix"},
	)
	if err != nil {
		t.Fatal(err)
	}
	stale.SetUnlinkOnClose(false)
	if err := os.Chmod(socketPath, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}

	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "fixture-build",
		Handler: HandlerFunc(func(
			context.Context,
			Request,
		) Response {
			return Response{}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	waitForServerReady(t, server)

	if err := os.Remove(socketPath); err != nil {
		t.Fatal(err)
	}
	replacement := []byte("do-not-remove")
	if err := os.WriteFile(socketPath, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := <-done; !errors.Is(err, ErrInvalidSocketPath) {
		t.Fatalf("Serve() replacement error = %v", err)
	}
	data, err := os.ReadFile(socketPath)
	if err != nil || !bytes.Equal(data, replacement) {
		t.Fatalf("replacement data=%q error=%v", data, err)
	}
}

func TestServerBoundsConcurrentConnectionsAndReturnsBusy(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "fixture-build",
		Handler: HandlerFunc(func(
			context.Context,
			Request,
		) Response {
			return Response{}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	waitForServerReady(t, server)

	connections := make([]net.Conn, 0, maxConnections)
	for index := 0; index < maxConnections; index++ {
		connection, err := net.DialTimeout(
			"unix",
			socketPath,
			time.Second,
		)
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, connection)
	}
	deadline := time.Now().Add(time.Second)
	for len(server.slots) != maxConnections {
		if time.Now().After(deadline) {
			t.Fatalf(
				"active slots = %d, want %d",
				len(server.slots),
				maxConnections,
			)
		}
		time.Sleep(time.Millisecond)
	}
	excess, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = excess.SetReadDeadline(time.Now().Add(time.Second))
	responseBytes, err := readFrame(excess, maxResponseBodyBytes)
	if err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := json.Unmarshal(responseBytes, &response); err != nil ||
		response.Error == nil ||
		response.Error.Code != "busy" {
		t.Fatalf("busy response=%s error=%v", responseBytes, err)
	}
	_ = excess.Close()
	for _, connection := range connections {
		_ = connection.Close()
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServerBoundsHandlerContextAndCloseCannotMissAcceptedConnection(
	t *testing.T,
) {
	for iteration := 0; iteration < 50; iteration++ {
		root := shortPrivateSocketRoot(t)
		socketPath := filepath.Join(root, "loomd.sock")
		handlerEntered := make(chan struct{})
		handlerCanceled := make(chan struct{})
		server, err := NewServer(ServerConfig{
			SocketPath:   socketPath,
			EffectiveUID: os.Geteuid(),
			BuildID:      "fixture-build",
			Handler: HandlerFunc(func(
				ctx context.Context,
				_ Request,
			) Response {
				if _, ok := ctx.Deadline(); !ok {
					t.Error("handler context has no deadline")
				}
				close(handlerEntered)
				<-ctx.Done()
				close(handlerCanceled)
				return Response{OK: false}
			}),
		})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- server.Serve(ctx) }()
		waitForServerReady(t, server)
		client, err := NewClient(ClientConfig{
			SocketPath: socketPath,
			Timeout:    time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		callDone := make(chan error, 1)
		go func() {
			var result map[string]any
			callDone <- client.Call(
				context.Background(),
				"snapshot",
				struct{}{},
				&result,
			)
		}()
		select {
		case <-handlerEntered:
		case <-time.After(time.Second):
			t.Fatal("handler did not start")
		}
		cancel()
		select {
		case <-handlerCanceled:
		case <-time.After(time.Second):
			t.Fatal("server cancellation did not cancel handler")
		}
		select {
		case <-callDone:
		case <-time.After(time.Second):
			t.Fatal("client remained blocked after server close")
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Serve() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Serve() remained blocked after accepted connection")
		}
	}
}

func TestServerCloseCancelsAndJoinsTrackedHandler(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	handlerEntered := make(chan struct{})
	handlerCanceled := make(chan struct{})
	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "fixture-build",
		Handler: HandlerFunc(func(
			ctx context.Context,
			_ Request,
		) Response {
			close(handlerEntered)
			<-ctx.Done()
			close(handlerCanceled)
			return Response{OK: false}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background()) }()
	waitForServerReady(t, server)
	client, err := NewClient(ClientConfig{
		SocketPath: socketPath,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	callDone := make(chan error, 1)
	go func() {
		var result map[string]any
		callDone <- client.Call(
			context.Background(),
			"snapshot",
			struct{}{},
			&result,
		)
	}()
	select {
	case <-handlerEntered:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	closeDone := make(chan error, 1)
	go func() { closeDone <- server.Close() }()
	select {
	case <-handlerCanceled:
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel tracked handler")
	}
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not join tracked handler")
	}
	select {
	case <-callDone:
	case <-time.After(time.Second):
		t.Fatal("client remained blocked after Close")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return after Close")
	}
}

func TestServerUsesExtendedDeadlineOnlyForCredentialVerify(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	type observation struct {
		method    string
		remaining time.Duration
	}
	observations := make(chan observation, 2)
	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "deadline-fixture",
		Handler: HandlerFunc(func(
			ctx context.Context,
			request Request,
		) Response {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Errorf("%s handler has no deadline", request.Method)
			}
			observations <- observation{
				method:    request.Method,
				remaining: time.Until(deadline),
			}
			return Response{OK: true, Result: json.RawMessage(`{}`)}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	waitForServerReady(t, server)
	client, err := NewClient(ClientConfig{
		SocketPath: socketPath,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"snapshot", "credential_verify"} {
		var result map[string]any
		if err := client.Call(
			context.Background(),
			method,
			struct{}{},
			&result,
		); err != nil {
			t.Fatalf("Call(%s) error = %v", method, err)
		}
	}
	first := <-observations
	second := <-observations
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	byMethod := map[string]time.Duration{
		first.method:  first.remaining,
		second.method: second.remaining,
	}
	if got := byMethod["snapshot"]; got <= 4*time.Second ||
		got > 5*time.Second {
		t.Fatalf("snapshot deadline remaining = %s, want (4s, 5s]", got)
	}
	if got := byMethod["credential_verify"]; got <= 9*time.Second ||
		got > 10*time.Second {
		t.Fatalf(
			"credential_verify deadline remaining = %s, want (9s, 10s]",
			got,
		)
	}
}

func waitForServerReady(t *testing.T, server *Server) {
	t.Helper()
	select {
	case <-server.Ready():
	case <-time.After(time.Second):
		t.Fatal("server did not become ready")
	}
}

func waitForSocket(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if info, err := os.Lstat(path); err == nil &&
			info.Mode()&os.ModeSocket != 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("socket %q not ready", path)
		}
		time.Sleep(time.Millisecond)
	}
}

func shortPrivateSocketRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("", "loom-w1-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove private socket root: %v", err)
		}
	})
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
