package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"loom-pi-rebuild/internal/composition"
)

// productHarnessRuntimeRefresher serializes the post-ready catalog refresh
// with lazy Agent Runtime construction. Both paths mutate the same Journal and
// Projection, so they must share one coordinator.
type productHarnessRuntimeRefresher struct {
	mu        sync.Mutex
	admitOnce sync.Once
	admitted  chan struct{}
	refresh   func(context.Context) error
	ready     bool
}

func newProductHarnessRuntimeRefresher(
	core *productCoreRouteSlot,
	claudeExecutable string,
	codexExecutable string,
	openCodeExecutable string,
) *productHarnessRuntimeRefresher {
	if claudeExecutable == "" && codexExecutable == "" && openCodeExecutable == "" {
		return nil
	}
	return &productHarnessRuntimeRefresher{
		admitted: make(chan struct{}),
		refresh: func(ctx context.Context) error {
			resources, err := core.Resources()
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			if err := ensureProductVerifiedClaudeCodeAgentRuntime(
				ctx, resources.store, resources.readModel, now, claudeExecutable,
			); err != nil {
				return err
			}
			if err := ensureProductVerifiedCodexAgentRuntime(
				ctx, resources.store, resources.readModel, now, codexExecutable,
			); err != nil {
				return err
			}
			return ensureProductVerifiedOpenCodeAgentRuntime(
				ctx, resources.store, resources.readModel, now, openCodeExecutable,
			)
		},
	}
}

func (refresher *productHarnessRuntimeRefresher) Refresh(ctx context.Context) error {
	if refresher == nil || refresher.refresh == nil || ctx == nil {
		return errors.New("Harness runtime refresh unavailable")
	}
	if refresher.admitted != nil {
		select {
		case <-refresher.admitted:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	refresher.mu.Lock()
	defer refresher.mu.Unlock()
	return refresher.refreshLocked(ctx)
}

func (refresher *productHarnessRuntimeRefresher) RefreshAfterReady(
	ctx context.Context,
) error {
	if refresher == nil || refresher.refresh == nil || ctx == nil ||
		refresher.admitted == nil {
		return errors.New("Harness runtime refresh unavailable")
	}
	refresher.mu.Lock()
	defer refresher.mu.Unlock()
	refresher.admitOnce.Do(func() { close(refresher.admitted) })
	return refresher.refreshLocked(ctx)
}

func (refresher *productHarnessRuntimeRefresher) refreshLocked(
	ctx context.Context,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if refresher.ready {
		return nil
	}
	if err := refresher.refresh(ctx); err != nil {
		return err
	}
	refresher.ready = true
	return nil
}

func recordProductHarnessRuntimeRefreshOutcome(
	diagnostics productOperationalDiagnosticSink,
	err error,
	profileID composition.ProfileID,
	snapshotDigest string,
	elapsed time.Duration,
) error {
	if nilProductAssetPort(diagnostics) {
		return nil
	}
	now := diagnostics.operationalNow().UTC()
	record := productOperationalDiagnosticRecord{
		SchemaVersion: 1,
		OccurredAt:    now.Format(time.RFC3339Nano),
		IncidentID: "loom-runtime-catalog-" + productDeterministicUUID(
			"runtime-catalog-refresh", now.Format(time.RFC3339Nano), snapshotDigest,
		),
		Operation:                 "composition",
		CredentialRuntime:         diagnostics.credentialRuntimeValue(),
		ProfileID:                 string(profileID),
		CompositionSnapshotDigest: snapshotDigest,
		BundleID:                  "loom-runtime-catalog",
		BundleVersion:             "1.0.0",
		Stage:                     "bundle_start",
		ElapsedMS:                 max(elapsed.Milliseconds(), 0),
		Result:                    "succeeded",
	}
	if err != nil {
		record.Result = "failed"
		record.ErrorCode = "runtime_catalog_refresh_failed"
		record.Retryable = true
	}
	return diagnostics.append(record)
}
