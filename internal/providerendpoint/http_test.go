package providerendpoint

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"testing"
	"time"
)

type changingResolver struct {
	mu        sync.Mutex
	responses [][]netip.Addr
	calls     int
}

func (resolver *changingResolver) LookupNetIP(
	context.Context,
	string,
	string,
) ([]netip.Addr, error) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	index := resolver.calls
	resolver.calls++
	if index >= len(resolver.responses) {
		index = len(resolver.responses) - 1
	}
	return resolver.responses[index], nil
}

type recordingDialer struct {
	mu    sync.Mutex
	calls int
}

func (dialer *recordingDialer) DialContext(
	context.Context,
	string,
	string,
) (net.Conn, error) {
	dialer.mu.Lock()
	dialer.calls++
	dialer.mu.Unlock()
	return nil, errors.New("controlled dial failure")
}

func TestHTTPTransportRevalidatesDNSAtDialTime(t *testing.T) {
	policy, err := NewEndpointPolicy("https://api.example.com/v1")
	if err != nil {
		t.Fatal(err)
	}
	resolver := &changingResolver{responses: [][]netip.Addr{
		{netip.MustParseAddr("93.184.216.34")},
		{netip.MustParseAddr("127.0.0.1")},
	}}
	if _, err := policy.Resolve(context.Background(), resolver); err != nil {
		t.Fatalf("initial Resolve() failed: %v", err)
	}
	dialer := &recordingDialer{}
	transport, err := NewHTTPTransport(HTTPConfig{
		Policy: policy, Resolver: resolver, Dialer: dialer, Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = transport.DialContext(context.Background(), "tcp", "api.example.com:443")
	if !errors.Is(err, ErrUnsafeAddress) {
		t.Fatalf("DialContext() error = %v, want ErrUnsafeAddress", err)
	}
	if dialer.calls != 0 {
		t.Fatalf("underlying dialer called %d times after unsafe re-resolution", dialer.calls)
	}
}

func TestHTTPClientBuilderAppliesBoundedHardenedDefaults(t *testing.T) {
	policy, err := NewEndpointPolicy("https://93.184.216.34/v1")
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewHTTPClient(HTTPConfig{Policy: policy, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client transport type = %T", client.Transport)
	}
	if transport.Proxy != nil || !transport.DisableCompression || transport.DialContext == nil {
		t.Fatalf("transport hardening missing: %#v", transport)
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatalf("TLS minimum = %#v, want TLS 1.2+", transport.TLSClientConfig)
	}
	if transport.TLSHandshakeTimeout <= 0 || transport.TLSHandshakeTimeout > client.Timeout ||
		transport.ResponseHeaderTimeout <= 0 || transport.ResponseHeaderTimeout > client.Timeout {
		t.Fatalf("transport timeouts exceed client bound %s", client.Timeout)
	}
	if client.Timeout != 2*time.Second || client.CheckRedirect == nil {
		t.Fatalf("client bounds missing: %#v", client)
	}
	if err := client.CheckRedirect(&http.Request{}, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("CheckRedirect() error = %v, want http.ErrUseLastResponse", err)
	}

	for _, timeout := range []time.Duration{0, MaxRequestTimeout + time.Nanosecond} {
		if _, err := NewHTTPClient(HTTPConfig{Policy: policy, Timeout: timeout}); err == nil {
			t.Fatalf("NewHTTPClient() accepted timeout %s", timeout)
		}
	}
}
