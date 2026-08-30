package main

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
)

type productCoreConstructionConfig struct {
	StatePath string
	Prepared  app.PreparedMissionDecisions
}

type productCoreResources struct {
	database  *sql.DB
	store     *journal.Store
	readModel *projection.Projection
	prepared  app.PreparedMissionDecisions
}

func (resources productCoreResources) valid() bool {
	return resources.database != nil && resources.store != nil && resources.readModel != nil
}

// productCoreRouteSlot is a trusted construction port, not a CapabilityContext
// capability. It never crosses the built-in Bundle boundary or request path.
type productCoreRouteSlot struct {
	mu        sync.RWMutex
	resources productCoreResources
	bound     bool
	closed    bool
}

func (slot *productCoreRouteSlot) Bind(resources productCoreResources) error {
	if slot == nil || !resources.valid() {
		return composition.ErrInvalidComposition
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.bound || slot.closed {
		return composition.ErrCompositionConflict
	}
	slot.resources = resources
	slot.bound = true
	return nil
}

func (slot *productCoreRouteSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && slot.resources.valid()
}

func (slot *productCoreRouteSlot) Resources() (productCoreResources, error) {
	if slot == nil {
		return productCoreResources{}, composition.ErrCapabilityUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if !slot.bound || slot.closed || !slot.resources.valid() {
		return productCoreResources{}, composition.ErrCapabilityUnavailable
	}
	return slot.resources, nil
}

func (slot *productCoreRouteSlot) Close() error {
	if slot == nil {
		return nil
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.closed {
		return nil
	}
	slot.closed = true
	database := slot.resources.database
	slot.resources = productCoreResources{}
	if database == nil {
		return nil
	}
	return database.Close()
}

func newProductCoreRouteFactory(
	config productCoreConstructionConfig,
) func(context.Context) (productCoreResources, error) {
	return func(ctx context.Context) (_ productCoreResources, resultErr error) {
		if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(config.StatePath) {
			return productCoreResources{}, newDaemonBuildFailure(
				"build_state", composition.ErrInvalidComposition,
			)
		}
		database, err := openProductReadDatabase(config.StatePath)
		if err != nil {
			return productCoreResources{}, newDaemonBuildFailure("build_state", err)
		}
		defer func() {
			if resultErr != nil {
				resultErr = errors.Join(resultErr, database.Close())
			}
		}()
		readModel := projection.New(database)
		if err := readModel.Rebuild(ctx); err != nil {
			return productCoreResources{}, newDaemonBuildFailure("build_state", err)
		}
		store := journal.NewStore(database)
		if err := controlledRuntimeStatusFixtureFromEnvironment(
			ctx, config.StatePath, store, readModel,
		); err != nil {
			return productCoreResources{}, newDaemonBuildFailure("build_state", err)
		}
		prepared, err := controlledMissionFixtureFromEnvironment(
			ctx, database, config.StatePath, config.Prepared,
		)
		if err != nil {
			return productCoreResources{}, newDaemonBuildFailure("build_state", err)
		}
		now := time.Now().UTC()
		if err := ensureProductVerifiedNativeAgentRuntimes(
			ctx, store, readModel, now,
		); err != nil {
			return productCoreResources{}, newDaemonBuildFailure("build_execution", err)
		}
		return productCoreResources{
			database: database, store: store, readModel: readModel, prepared: prepared,
		}, nil
	}
}

func (construction productCompatibilityConstruction) startCore(
	ctx context.Context,
	_ *composition.BundleContext,
) (composition.Effect, error) {
	if !construction.valid() || construction.coreSlot == nil || ctx == nil {
		return nil, composition.ErrInvalidComposition
	}
	resources, err := construction.coreFactory(ctx)
	if err != nil || !resources.valid() {
		if resources.database != nil {
			_ = resources.database.Close()
		}
		return nil, errors.Join(composition.ErrCapabilityUnavailable, err)
	}
	if err := construction.coreSlot.Bind(resources); err != nil {
		_ = resources.database.Close()
		return nil, err
	}
	return composition.NewEffect(func(context.Context) error {
		return construction.coreSlot.Close()
	}), nil
}

func (construction productCompatibilityConstruction) coreReady(
	context.Context,
	*composition.BundleContext,
) error {
	if !construction.valid() || construction.coreSlot == nil ||
		!construction.coreSlot.Ready() {
		return composition.ErrCapabilityUnavailable
	}
	return nil
}
