package providerendpoint

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

const MaxRequestTimeout = 30 * time.Second

var (
	ErrInvalidHTTPConfig = errors.New("invalid Provider endpoint HTTP configuration")
	ErrDialFailed        = errors.New("Provider endpoint dial failed")
)

type Dialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type HTTPConfig struct {
	Policy   EndpointPolicy
	Resolver Resolver
	Dialer   Dialer
	Timeout  time.Duration
}

func NewHTTPTransport(config HTTPConfig) (*http.Transport, error) {
	if config.Timeout <= 0 || config.Timeout > MaxRequestTimeout || !config.Policy.valid() {
		return nil, ErrInvalidHTTPConfig
	}
	resolver := config.Resolver
	if nilInterface(resolver) {
		resolver = net.DefaultResolver
	}
	dialer := config.Dialer
	if nilInterface(dialer) {
		dialer = &net.Dialer{Timeout: config.Timeout, KeepAlive: config.Timeout}
	}

	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, ErrInvalidHTTPConfig
	}
	transport := base.Clone()
	transport.Proxy = nil
	transport.DialContext = policyDialContext(
		config.Policy, resolver, dialer, config.Timeout,
	)
	transport.DialTLSContext = nil
	transport.DisableCompression = true
	transport.TLSClientConfig = cloneTLSConfig(transport.TLSClientConfig)
	transport.TLSClientConfig.MinVersion = tls.VersionTLS12
	transport.TLSHandshakeTimeout = config.Timeout
	transport.ResponseHeaderTimeout = config.Timeout
	transport.ExpectContinueTimeout = min(config.Timeout, time.Second)
	transport.IdleConnTimeout = config.Timeout
	transport.MaxResponseHeaderBytes = 64 << 10
	transport.MaxIdleConns = 8
	transport.MaxIdleConnsPerHost = 2
	transport.MaxConnsPerHost = 4
	return transport, nil
}

func NewHTTPClient(config HTTPConfig) (*http.Client, error) {
	transport, err := NewHTTPTransport(config)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

func policyDialContext(
	policy EndpointPolicy,
	resolver Resolver,
	dialer Dialer,
	timeout time.Duration,
) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network string, target string) (net.Conn, error) {
		if ctx == nil || network != "tcp" && network != "tcp4" && network != "tcp6" {
			return nil, ErrDialFailed
		}
		host, port, err := net.SplitHostPort(target)
		if err != nil || port != "443" ||
			!strings.EqualFold(strings.TrimSuffix(host, "."), policy.hostname) {
			return nil, ErrDialFailed
		}
		dialContext, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		addresses, err := policy.Resolve(dialContext, resolver)
		if err != nil {
			return nil, err
		}
		for _, address := range addresses {
			if network == "tcp4" && !address.Is4() || network == "tcp6" && !address.Is6() {
				continue
			}
			connection, dialErr := dialer.DialContext(
				dialContext, network, net.JoinHostPort(address.String(), "443"),
			)
			if dialErr == nil && connection != nil {
				return connection, nil
			}
		}
		return nil, ErrDialFailed
	}
}

func cloneTLSConfig(config *tls.Config) *tls.Config {
	if config == nil {
		return &tls.Config{}
	}
	return config.Clone()
}
