package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/localipc"
)

func TestCOMP2DLocalIPCBundleOwnsHandlerLifecycle(t *testing.T) {
	slot := &productLocalIPCHandlerSlot{}
	factoryCalls := 0
	handlerCalls := 0
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest, nil,
		"incident-comp2d-local-ipc", nil,
		productCompatibilityConstruction{
			localIPCSlot: slot,
			localIPCFactory: func(context.Context) (localipc.Handler, error) {
				factoryCalls++
				return localipc.HandlerFunc(func(
					_ context.Context, request localipc.Request,
				) localipc.Response {
					handlerCalls++
					return localipc.Response{
						Version: request.Version, RequestID: request.RequestID, OK: true,
					}
				}), nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if factoryCalls != 1 || !slot.Ready() {
		t.Fatalf("factoryCalls=%d ready=%t", factoryCalls, slot.Ready())
	}
	admitted := facade.Handler()
	request := localipc.Request{Version: 1, RequestID: "request-local-ipc"}
	if response := admitted.Handle(context.Background(), request); !response.OK || handlerCalls != 1 {
		t.Fatalf("response=%#v handlerCalls=%d", response, handlerCalls)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if slot.Ready() {
		t.Fatal("local IPC slot remained ready after Composition close")
	}
	if response := admitted.Handle(context.Background(), request); response.OK ||
		response.Error == nil || response.Error.Code != "state_unavailable" {
		t.Fatalf("closed handler admitted request: %#v", response)
	}
}

func TestCOMP2DLocalIPCBundleFailsClosedWhenFactoryFails(t *testing.T) {
	slot := &productLocalIPCHandlerSlot{}
	privateFailure := errors.New("private local IPC construction failure")
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest, nil,
		"incident-comp2d-local-ipc-failure", nil,
		productCompatibilityConstruction{
			localIPCSlot: slot,
			localIPCFactory: func(context.Context) (localipc.Handler, error) {
				return nil, privateFailure
			},
		},
	)
	if err == nil || facade != nil || !errors.Is(err, privateFailure) || slot.Ready() {
		t.Fatalf("facade=%#v ready=%t err=%v", facade, slot.Ready(), err)
	}
	if bindErr := slot.Bind(localipc.HandlerFunc(func(
		context.Context, localipc.Request,
	) localipc.Response {
		return localipc.Response{OK: true}
	})); bindErr == nil {
		t.Fatal("failed construction left local IPC slot reusable")
	}
}

func TestCOMP2DLocalIPCBundleRejectsAmbiguousHandlerSource(t *testing.T) {
	slot := &productLocalIPCHandlerSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2d-local-ipc-ambiguous", nil,
		productCompatibilityConstruction{
			localIPCSlot: slot,
			localIPCFactory: func(context.Context) (localipc.Handler, error) {
				return slot, nil
			},
		},
	)
	if !errors.Is(err, composition.ErrInvalidComposition) || facade != nil {
		t.Fatalf("facade=%#v err=%v", facade, err)
	}
}

func TestCOMP2DLocalIPCSlotCloseQuiescesInFlightRequest(t *testing.T) {
	slot := &productLocalIPCHandlerSlot{}
	entered := make(chan struct{})
	release := make(chan struct{})
	if err := slot.Bind(localipc.HandlerFunc(func(
		context.Context, localipc.Request,
	) localipc.Response {
		close(entered)
		<-release
		return localipc.Response{OK: true}
	})); err != nil {
		t.Fatal(err)
	}
	done := make(chan localipc.Response, 1)
	go func() {
		done <- slot.Handle(context.Background(), localipc.Request{Version: 1})
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handler did not enter")
	}
	closed := make(chan error, 1)
	go func() { closed <- slot.Close(context.Background()) }()
	select {
	case err := <-closed:
		t.Fatalf("Close detached in-flight request: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if response := <-done; !response.OK {
		t.Fatalf("response=%#v", response)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
}
