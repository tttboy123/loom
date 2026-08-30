package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
)

type productBlockingNativeAuthObserver struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
	result  app.NativeAuthObservation
}

type productInvalidatedNativeAuthObserver struct {
	mu             sync.Mutex
	calls          int
	firstStarted   chan struct{}
	firstCancelled chan struct{}
}

func (observer *productInvalidatedNativeAuthObserver) ObserveNativeAuth(
	ctx context.Context,
) (app.NativeAuthObservation, error) {
	observer.mu.Lock()
	observer.calls++
	call := observer.calls
	observer.mu.Unlock()
	if call == 1 {
		close(observer.firstStarted)
		<-ctx.Done()
		close(observer.firstCancelled)
		return app.NativeAuthObservation{}, ctx.Err()
	}
	return app.NativeAuthObservation{
		Status: "available", AuthMode: "native_auth",
	}, nil
}

func (observer *productBlockingNativeAuthObserver) ObserveNativeAuth(
	ctx context.Context,
) (app.NativeAuthObservation, error) {
	observer.mu.Lock()
	observer.calls++
	if observer.calls == 1 {
		close(observer.started)
	}
	observer.mu.Unlock()
	select {
	case <-ctx.Done():
		return app.NativeAuthObservation{}, ctx.Err()
	case <-observer.release:
		return observer.result, nil
	}
}

func (observer *productBlockingNativeAuthObserver) callCount() int {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return observer.calls
}

func TestProductNativeAuthSnapshotObserverReturnsImmediatelyAndSingleFlightsRefresh(
	t *testing.T,
) {
	delegate := &productBlockingNativeAuthObserver{
		started: make(chan struct{}),
		release: make(chan struct{}),
		result: app.NativeAuthObservation{
			Status: "available", AuthMode: "native_auth",
		},
	}
	fallback := app.NativeAuthObservation{
		Status: "unavailable", AuthMode: "native_auth", Reason: "unavailable",
	}
	observer, err := newProductNativeAuthSnapshotObserver(
		delegate, fallback, time.Second, time.Minute, 20*time.Millisecond,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = observer.Close() })

	startedAt := time.Now()
	initial, err := observer.ObserveNativeAuth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(startedAt) > 100*time.Millisecond || initial != fallback {
		t.Fatalf("initial observation = %#v after %s", initial, time.Since(startedAt))
	}
	select {
	case <-delegate.started:
	case <-time.After(time.Second):
		t.Fatal("background native-auth refresh did not start")
	}

	for range 32 {
		observation, err := observer.ObserveNativeAuth(context.Background())
		if err != nil || observation != fallback {
			t.Fatalf("concurrent observation = %#v, %v", observation, err)
		}
	}
	if calls := delegate.callCount(); calls != 1 {
		t.Fatalf("single-flight delegate calls = %d", calls)
	}

	close(delegate.release)
	deadline := time.Now().Add(time.Second)
	for {
		observation, err := observer.ObserveNativeAuth(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if observation.Status == "available" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("refreshed native-auth observation was not published")
		}
		time.Sleep(time.Millisecond)
	}
	if calls := delegate.callCount(); calls != 1 {
		t.Fatalf("cached delegate calls = %d", calls)
	}
}

func TestProductNativeAuthSnapshotObserverRejectsInvalidConfiguration(t *testing.T) {
	fallback := app.NativeAuthObservation{
		Status: "unavailable", AuthMode: "native_auth", Reason: "unavailable",
	}
	if _, err := newProductNativeAuthSnapshotObserver(
		nil, fallback, time.Second, time.Minute, time.Second,
	); err == nil {
		t.Fatal("nil native-auth delegate was accepted")
	}
	invalid := fallback
	invalid.AuthMode = "brokered"
	if _, err := newProductNativeAuthSnapshotObserver(
		&productBlockingNativeAuthObserver{}, invalid,
		time.Second, time.Minute, time.Second,
	); err == nil {
		t.Fatal("invalid native-auth fallback was accepted")
	}
}

func TestProductNativeAuthSnapshotObserverInvalidationCancelsAndDiscardsInflightResult(
	t *testing.T,
) {
	delegate := &productInvalidatedNativeAuthObserver{
		firstStarted: make(chan struct{}), firstCancelled: make(chan struct{}),
	}
	fallback := app.NativeAuthObservation{
		Status: "unavailable", AuthMode: "native_auth", Reason: "unavailable",
	}
	observer, err := newProductNativeAuthSnapshotObserver(
		delegate, fallback, 30*time.Second, time.Minute, 20*time.Millisecond,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = observer.Close() })
	if observation, err := observer.ObserveNativeAuth(context.Background()); err != nil || observation != fallback {
		t.Fatalf("initial observation = %#v, %v", observation, err)
	}
	select {
	case <-delegate.firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first refresh did not start")
	}
	observer.Invalidate()
	select {
	case <-delegate.firstCancelled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("invalidated refresh was not cancelled")
	}

	deadline := time.Now().Add(time.Second)
	for {
		observation, err := observer.ObserveNativeAuth(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if observation.Status == "available" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fresh post-invalidation result was not published")
		}
		time.Sleep(time.Millisecond)
	}
	delegate.mu.Lock()
	calls := delegate.calls
	delegate.mu.Unlock()
	if calls != 2 {
		t.Fatalf("delegate calls after invalidation = %d", calls)
	}
}
