package main

import (
	"context"
	"sync"
	"time"

	"loom-pi-rebuild/internal/app"
)

type productNativeAuthSnapshotObserver struct {
	delegate app.NativeAuthObserver
	fallback app.NativeAuthObservation
	timeout  time.Duration
	freshTTL time.Duration
	retryTTL time.Duration

	mu          sync.Mutex
	cached      app.NativeAuthObservation
	cacheUntil  time.Time
	cacheExists bool
	refreshing  bool
	closed      bool
	generation  uint64
	cancel      context.CancelFunc
	wait        sync.WaitGroup
}

func newProductNativeAuthSnapshotObserver(
	delegate app.NativeAuthObserver,
	fallback app.NativeAuthObservation,
	timeout time.Duration,
	freshTTL time.Duration,
	retryTTL time.Duration,
) (*productNativeAuthSnapshotObserver, error) {
	if delegate == nil || !validProductNativeAuthSnapshot(fallback) ||
		timeout <= 0 || freshTTL <= 0 || retryTTL <= 0 {
		return nil, app.ErrInvalidLocalProductSetup
	}
	return &productNativeAuthSnapshotObserver{
		delegate: delegate,
		fallback: fallback,
		timeout:  timeout,
		freshTTL: freshTTL,
		retryTTL: retryTTL,
	}, nil
}

func (observer *productNativeAuthSnapshotObserver) ObserveNativeAuth(
	ctx context.Context,
) (app.NativeAuthObservation, error) {
	if observer == nil || ctx == nil || ctx.Err() != nil {
		return app.NativeAuthObservation{}, app.ErrInvalidLocalProductSetup
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if observer.closed {
		return app.NativeAuthObservation{}, app.ErrInvalidLocalProductSetup
	}
	now := time.Now()
	if observer.cacheExists && now.Before(observer.cacheUntil) {
		return observer.cached, nil
	}
	if !observer.refreshing {
		observer.startRefreshLocked()
	}
	if observer.cacheExists {
		return observer.cached, nil
	}
	return observer.fallback, nil
}

func (observer *productNativeAuthSnapshotObserver) Warm() {
	if observer == nil {
		return
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if observer.closed || observer.refreshing ||
		observer.cacheExists && time.Now().Before(observer.cacheUntil) {
		return
	}
	observer.startRefreshLocked()
}

func (observer *productNativeAuthSnapshotObserver) Invalidate() {
	if observer == nil {
		return
	}
	observer.mu.Lock()
	var cancel context.CancelFunc
	if !observer.closed {
		observer.generation++
		observer.cacheUntil = time.Time{}
		cancel = observer.cancel
	}
	observer.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (observer *productNativeAuthSnapshotObserver) Close() error {
	if observer == nil {
		return nil
	}
	observer.mu.Lock()
	if observer.closed {
		observer.mu.Unlock()
		return nil
	}
	observer.closed = true
	cancel := observer.cancel
	observer.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	observer.wait.Wait()
	return nil
}

func (observer *productNativeAuthSnapshotObserver) startRefreshLocked() {
	refreshContext, cancel := context.WithTimeout(context.Background(), observer.timeout)
	observer.refreshing = true
	observer.cancel = cancel
	generation := observer.generation
	observer.wait.Add(1)
	go observer.refresh(refreshContext, cancel, generation)
}

func (observer *productNativeAuthSnapshotObserver) refresh(
	ctx context.Context,
	cancel context.CancelFunc,
	generation uint64,
) {
	defer observer.wait.Done()
	defer cancel()
	observation, err := observer.delegate.ObserveNativeAuth(ctx)
	valid := err == nil && validProductNativeAuthSnapshot(observation)
	if !valid {
		observation = observer.fallback
	}

	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.refreshing = false
	observer.cancel = nil
	if observer.closed || generation != observer.generation {
		return
	}
	observer.cached = observation
	observer.cacheExists = true
	ttl := observer.retryTTL
	if valid {
		ttl = observer.freshTTL
	}
	observer.cacheUntil = time.Now().Add(ttl)
}

func validProductNativeAuthSnapshot(observation app.NativeAuthObservation) bool {
	if observation.AuthMode != "native_auth" {
		return false
	}
	switch observation.Status {
	case "available":
		return observation.Reason == ""
	case "not_logged_in":
		return observation.Reason == "not_logged_in"
	case "unsupported":
		return observation.Reason == "unknown_output"
	case "unavailable":
		return observation.Reason == "timeout" ||
			observation.Reason == "identity_changed" ||
			observation.Reason == "unavailable"
	default:
		return false
	}
}

var _ app.NativeAuthObserver = (*productNativeAuthSnapshotObserver)(nil)
var _ interface{ Close() error } = (*productNativeAuthSnapshotObserver)(nil)
