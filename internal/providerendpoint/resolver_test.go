package providerendpoint

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

type staticResolver struct {
	addresses []netip.Addr
	err       error
}

func (resolver staticResolver) LookupNetIP(
	context.Context,
	string,
	string,
) ([]netip.Addr, error) {
	return resolver.addresses, resolver.err
}

func TestEndpointPolicyResolveRejectsAnyUnsafeDNSAnswer(t *testing.T) {
	policy, err := NewEndpointPolicy("https://api.example.com/v1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = policy.Resolve(context.Background(), staticResolver{addresses: []netip.Addr{
		netip.MustParseAddr("93.184.216.34"),
		netip.MustParseAddr("10.0.0.8"),
	}})
	if !errors.Is(err, ErrUnsafeAddress) {
		t.Fatalf("Resolve() error = %v, want ErrUnsafeAddress", err)
	}
}

func TestEndpointPolicyResolveReturnsOnlyValidatedPublicAddresses(t *testing.T) {
	policy, err := NewEndpointPolicy("https://api.example.com/v1")
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Addr{
		netip.MustParseAddr("93.184.216.34"),
		netip.MustParseAddr("2606:2800:220:1:248:1893:25c8:1946"),
	}

	got, err := policy.Resolve(context.Background(), staticResolver{addresses: want})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Resolve() = %v, want %v", got, want)
	}
}
