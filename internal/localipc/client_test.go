package localipc

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClientUsesExtendedTimeoutOnlyForLongOperations(t *testing.T) {
	client := Client{
		timeout:         5 * time.Second,
		extendedTimeout: 10 * time.Second,
	}
	for _, method := range []string{"credential_verify"} {
		if got := client.timeoutForMethod(method); got != 10*time.Second {
			t.Fatalf("timeoutForMethod(%q) = %s, want 10s", method, got)
		}
	}
	if got := client.timeoutForMethod("mission_execution"); got != 185*time.Second {
		t.Fatalf("timeoutForMethod(mission_execution) = %s, want 185s", got)
	}
	if got := client.timeoutForMethod("chat_message"); got != 1810*time.Second {
		t.Fatalf("timeoutForMethod(chat_message) = %s, want 1810s", got)
	}
	if got := client.timeoutForMethod("agent_attempt_recovery"); got != 55*time.Second {
		t.Fatalf("timeoutForMethod(agent_attempt_recovery) = %s, want 55s", got)
	}
	for _, method := range []string{
		"ping", "snapshot", "timeline_page", "chat_thread", "chat_context_disclosure",
	} {
		if got := client.timeoutForMethod(method); got != 5*time.Second {
			t.Fatalf("timeoutForMethod(%q) = %s, want 5s", method, got)
		}
	}
}

func TestClientConfigDefaultsAndValidatesExtendedTimeout(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	client, err := NewClient(ClientConfig{
		SocketPath: socketPath,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.extendedTimeout != 5*time.Second {
		t.Fatalf("default extended timeout = %s, want 5s", client.extendedTimeout)
	}
	if _, err := NewClient(ClientConfig{
		SocketPath:      socketPath,
		Timeout:         5 * time.Second,
		ExtendedTimeout: 4 * time.Second,
	}); !errors.Is(err, ErrInvalidSocketPath) {
		t.Fatalf("short extended timeout error = %v, want invalid input", err)
	}
}

func TestClientConfigRequiresResponseGraceBeyondServerDeadline(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	if _, err := NewClient(ClientConfig{
		SocketPath:      socketPath,
		Timeout:         5 * time.Second,
		ExtendedTimeout: extendedResponseDeadline,
	}); !errors.Is(err, ErrInvalidSocketPath) {
		t.Fatalf("equal extended deadline error = %v, want invalid input", err)
	}
	client, err := NewClient(ClientConfig{
		SocketPath:      socketPath,
		Timeout:         5 * time.Second,
		ExtendedTimeout: extendedRequestDeadline + 5*time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.extendedTimeout != 15*time.Second {
		t.Fatalf("extended timeout = %s, want 15s", client.extendedTimeout)
	}
}

func TestExtendedClientReceivesControlledServerDeadlineResponse(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "deadline-response-fixture",
		Handler: HandlerFunc(func(
			ctx context.Context,
			request Request,
		) Response {
			<-ctx.Done()
			return Response{
				Version:   1,
				RequestID: request.RequestID,
				Error: &ProtocolError{
					Code:        "state_unavailable",
					Message:     "controlled deadline",
					Recoverable: true,
				},
			}
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
		SocketPath:      socketPath,
		Timeout:         5 * time.Second,
		ExtendedTimeout: 15 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	var result map[string]any
	// Keep this bounded response test on the generic extended operation budget;
	// mission execution has a larger production budget because new-attempt
	// preflight may persist encrypted Context Capsules.
	err = client.Call(context.Background(), "credential_verify", struct{}{}, &result)
	if elapsed := time.Since(started); elapsed < extendedRequestDeadline ||
		elapsed >= 15*time.Second {
		t.Fatalf("controlled response elapsed = %s, want [10s, 15s)", elapsed)
	}
	var remote *RemoteError
	if !errors.As(err, &remote) || remote.Code != "state_unavailable" ||
		!remote.Recoverable {
		t.Fatalf("controlled response error = %#v", err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestClientAndServerRoundTripOneBoundedRequestAndCloseCleanly(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	handler := HandlerFunc(func(_ context.Context, request Request) Response {
		result, _ := json.Marshal(map[string]any{
			"protocol_version": 1,
			"available":        true,
			"build_id":         "fixture-build",
		})
		return Response{
			Version:   1,
			RequestID: request.RequestID,
			OK:        true,
			Result:    result,
		}
	})
	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "fixture-build",
		Handler:      handler,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	waitForSocket(t, socketPath)

	client, err := NewClient(ClientConfig{
		SocketPath: socketPath,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ping, err := client.Ping(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ping.ProtocolVersion != 1 || !ping.Available ||
		ping.BuildID != "fixture-build" {
		t.Fatalf("ping = %#v", ping)
	}
	var snapshot PingResult
	if err := client.Call(
		context.Background(),
		"snapshot",
		map[string]any{"limit": 64},
		&snapshot,
	); err != nil {
		t.Fatal(err)
	}
	if snapshot.BuildID != "fixture-build" {
		t.Fatalf("snapshot result = %#v", snapshot)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if _, err := os.Lstat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("socket remains after close: %v", err)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestP3ACallJourneyUsesProductionSocketAndRejectsResponseIdentityDrift(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	drift := false
	server, err := NewServer(ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(), BuildID: "fixture-build",
		Handler: HandlerFunc(func(_ context.Context, request Request) Response {
			result, _ := json.Marshal(map[string]any{"schema_version": 1, "records": []any{}})
			responseJourney := request.JourneyID
			if drift {
				responseJourney = "223e4567-e89b-42d3-a456-426614174000"
			}
			return Response{OK: true, JourneyID: responseJourney, Result: result}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	waitForSocket(t, socketPath)
	client, err := NewClient(ClientConfig{SocketPath: socketPath, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := client.CallJourney(
		context.Background(), journeyID, "evolution_asset_snapshot", struct{}{}, &result,
	); err != nil {
		t.Fatalf("CallJourney(valid) error = %v", err)
	}
	drift = true
	if err := client.CallJourney(
		context.Background(), journeyID, "evolution_asset_snapshot", struct{}{}, &result,
	); !errors.Is(err, ErrInvalidProtocol) {
		t.Fatalf("CallJourney(drift) error = %v, want invalid protocol", err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestClientValidatesInputMapsRemoteErrorAndBoundsTimeout(t *testing.T) {
	if _, err := NewClient(ClientConfig{}); err == nil {
		t.Fatal("NewClient(invalid) error = nil")
	}
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "fixture-build",
		Handler: HandlerFunc(func(
			_ context.Context,
			request Request,
		) Response {
			switch request.Method {
			case "snapshot":
				return Response{
					OK: false,
					Error: safeProtocolError(
						"state_unavailable",
						errors.New("private path"),
					),
				}
			case "timeline_page", "chat_message":
				time.Sleep(250 * time.Millisecond)
				result, _ := json.Marshal(map[string]any{"late": true})
				return Response{OK: true, Result: result}
			default:
				return Response{}
			}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	waitForSocket(t, socketPath)

	client, err := NewClient(ClientConfig{
		SocketPath:      socketPath,
		Timeout:         time.Second,
		ExtendedTimeout: 15 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	for _, call := range []struct {
		method string
		params any
		result any
	}{
		{"snapshot", nil, &result},
		{"snapshot", struct{}{}, nil},
		{"snapshot", make(chan int), &result},
	} {
		if err := client.Call(
			context.Background(),
			call.method,
			call.params,
			call.result,
		); !errors.Is(err, ErrInvalidProtocol) {
			t.Fatalf("invalid client input error = %v", err)
		}
	}
	err = client.Call(
		context.Background(),
		"snapshot",
		map[string]any{"limit": 64},
		&result,
	)
	var remote *RemoteError
	if !errors.As(err, &remote) ||
		remote.Code != "state_unavailable" ||
		remote.Error() !=
			"local product request failed: state_unavailable" {
		t.Fatalf("remote error = %#v, %v", remote, err)
	}
	if err := client.Call(
		context.Background(),
		"missing",
		struct{}{},
		&result,
	); !errors.Is(err, ErrInvalidProtocol) {
		t.Fatalf("invalid method error = %v", err)
	}
	cancelled, cancelCall := context.WithCancel(context.Background())
	cancelCall()
	if err := client.Call(
		cancelled,
		"snapshot",
		struct{}{},
		&result,
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled call error = %v", err)
	}
	inflight, cancelInflight := context.WithCancel(context.Background())
	inflightDone := make(chan error, 1)
	go func() {
		inflightDone <- client.Call(
			inflight,
			"chat_message",
			struct{}{},
			&result,
		)
	}()
	time.Sleep(20 * time.Millisecond)
	cancelInflight()
	select {
	case err := <-inflightDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("in-flight cancellation error = %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("in-flight client call ignored context cancellation")
	}

	shortClient, err := NewClient(ClientConfig{
		SocketPath: socketPath,
		Timeout:    15 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := shortClient.Call(
		context.Background(),
		"timeline_page",
		struct{}{},
		&result,
	); !errors.Is(err, ErrProtocolTimeout) {
		t.Fatalf("timeout error = %v", err)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
