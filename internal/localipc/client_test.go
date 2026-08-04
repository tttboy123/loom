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
			case "timeline_page":
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
		SocketPath: socketPath,
		Timeout:    time.Second,
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
			"timeline_page",
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
