package providerendpoint

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
)

var ErrResolutionFailed = errors.New("Provider endpoint resolution failed")

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

func (policy EndpointPolicy) Resolve(
	ctx context.Context,
	resolver Resolver,
) ([]netip.Addr, error) {
	if ctx == nil || policy.hostname == "" {
		return nil, ErrResolutionFailed
	}
	if literal, err := netip.ParseAddr(policy.hostname); err == nil {
		literal = literal.Unmap()
		if !allowedAddress(literal) {
			return nil, ErrUnsafeAddress
		}
		return []netip.Addr{literal}, nil
	}
	if nilInterface(resolver) {
		return nil, ErrResolutionFailed
	}

	addresses, err := resolver.LookupNetIP(ctx, "ip", policy.hostname)
	if err != nil || len(addresses) == 0 {
		return nil, ErrResolutionFailed
	}
	validated := make([]netip.Addr, 0, len(addresses))
	seen := make(map[netip.Addr]struct{}, len(addresses))
	for _, address := range addresses {
		address = address.Unmap()
		if !allowedAddress(address) {
			return nil, ErrUnsafeAddress
		}
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		validated = append(validated, address)
	}
	if len(validated) == 0 {
		return nil, ErrResolutionFailed
	}
	return validated, nil
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
