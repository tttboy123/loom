package discoveryscan

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	loomruntime "loom-pi-rebuild/internal/runtime"
)

const MaxProbeFactories = 32

var (
	ErrInvalidConfiguredRuntimeDiscovery = errors.New("invalid configured runtime discovery")
	ErrRuntimeProbeFactoryFailed         = errors.New("runtime probe factory failed")
)

type ProbeFactory interface {
	BuildProbe(context.Context) (loomruntime.RuntimeProbe, bool, error)
}

func DiscoverConfiguredRuntimes(
	ctx context.Context,
	factories []ProbeFactory,
) (loomruntime.RuntimeDiscoverySnapshot, error) {
	if ctx == nil || len(factories) > MaxProbeFactories {
		return loomruntime.RuntimeDiscoverySnapshot{}, ErrInvalidConfiguredRuntimeDiscovery
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{}, err
	}

	configured := append([]ProbeFactory(nil), factories...)
	for _, factory := range configured {
		if nilInterface(factory) {
			return loomruntime.RuntimeDiscoverySnapshot{}, ErrInvalidConfiguredRuntimeDiscovery
		}
	}

	probes := make([]loomruntime.RuntimeProbe, 0, len(configured))
	for _, factory := range configured {
		if err := ctx.Err(); err != nil {
			return loomruntime.RuntimeDiscoverySnapshot{}, err
		}

		probe, present, err := factory.BuildProbe(ctx)
		if err != nil {
			switch {
			case errors.Is(err, context.Canceled):
				return loomruntime.RuntimeDiscoverySnapshot{}, context.Canceled
			case errors.Is(err, context.DeadlineExceeded):
				return loomruntime.RuntimeDiscoverySnapshot{}, context.DeadlineExceeded
			default:
				return loomruntime.RuntimeDiscoverySnapshot{},
					fmt.Errorf("%w: %w", ErrRuntimeProbeFactoryFailed, err)
			}
		}
		if err := ctx.Err(); err != nil {
			return loomruntime.RuntimeDiscoverySnapshot{}, err
		}

		switch {
		case present && !nilInterface(probe):
			probes = append(probes, probe)
		case !present && probe == nil:
		default:
			return loomruntime.RuntimeDiscoverySnapshot{}, ErrInvalidConfiguredRuntimeDiscovery
		}
	}

	return loomruntime.DiscoverRuntime(ctx, probes)
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
