package toolbroker

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"
)

// NewSystemHTTPClient creates the production WebFetch transport. DNS results
// are checked at dial time so a public hostname cannot redirect the broker to
// loopback, link-local, or private address space.
func NewSystemHTTPClient(timeout time.Duration) (*http.Client, error) {
	if timeout <= 0 || timeout > 2*time.Minute {
		return nil, ErrInvalidConfig
	}
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableCompression = true
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || port != strconv.Itoa(443) {
			return nil, ErrToolDenied
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addresses) == 0 {
			return nil, errors.Join(ErrToolFailed, err)
		}
		for _, candidate := range addresses {
			if !publicIP(candidate.IP) {
				return nil, ErrToolDenied
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 || request == nil || request.URL == nil ||
				!validPublicHTTPS(request.URL.String()) {
				return ErrToolDenied
			}
			return nil
		},
	}, nil
}

func publicIP(ip net.IP) bool {
	return ip != nil && !ip.IsPrivate() && !ip.IsLoopback() &&
		!ip.IsUnspecified() && !ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() && !ip.IsMulticast()
}
