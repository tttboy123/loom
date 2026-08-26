package providerendpoint

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/netip"
	"net/url"
	"path"
	"strings"
)

var (
	ErrInvalidEndpoint = errors.New("invalid Provider endpoint")
	ErrUnsafeAddress   = errors.New("unsafe Provider endpoint address")
)

const CurrentPolicyVersion uint32 = 1

type EndpointPolicy struct {
	canonicalEndpoint   string
	hostname            string
	endpointFingerprint string
	policyDigest        string
}

func NewEndpointPolicy(rawEndpoint string) (EndpointPolicy, error) {
	canonical, err := canonicalizeEndpoint(rawEndpoint)
	if err != nil {
		return EndpointPolicy{}, err
	}
	sum := sha256.Sum256([]byte(canonical))
	policySum := sha256.Sum256([]byte("loom.provider-endpoint-policy.v1\x00" + canonical))
	return EndpointPolicy{
		canonicalEndpoint:   canonical,
		hostname:            endpointHostname(canonical),
		endpointFingerprint: hex.EncodeToString(sum[:]),
		policyDigest:        hex.EncodeToString(policySum[:]),
	}, nil
}

func endpointHostname(canonical string) string {
	parsed, err := url.Parse(canonical)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func (policy EndpointPolicy) CanonicalEndpoint() string {
	return policy.canonicalEndpoint
}

func (policy EndpointPolicy) EndpointFingerprint() string {
	return policy.endpointFingerprint
}

func (policy EndpointPolicy) Version() uint32 {
	return CurrentPolicyVersion
}

func (policy EndpointPolicy) Digest() string {
	return policy.policyDigest
}

func (policy EndpointPolicy) valid() bool {
	rebuilt, err := NewEndpointPolicy(policy.canonicalEndpoint)
	return err == nil && rebuilt.canonicalEndpoint == policy.canonicalEndpoint &&
		rebuilt.hostname == policy.hostname &&
		rebuilt.endpointFingerprint == policy.endpointFingerprint &&
		rebuilt.policyDigest == policy.policyDigest
}

func canonicalizeEndpoint(rawEndpoint string) (string, error) {
	if rawEndpoint == "" || strings.TrimSpace(rawEndpoint) != rawEndpoint {
		return "", ErrInvalidEndpoint
	}
	parsed, err := url.Parse(rawEndpoint)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Opaque != "" ||
		parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery ||
		parsed.Fragment != "" || parsed.Port() != "" && parsed.Port() != "443" {
		return "", ErrInvalidEndpoint
	}

	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if !validEndpointHost(host) {
		return "", ErrInvalidEndpoint
	}
	if address, addressErr := netip.ParseAddr(host); addressErr == nil && !allowedAddress(address) {
		return "", ErrUnsafeAddress
	}
	cleanPath, err := canonicalPath(parsed.Path)
	if err != nil {
		return "", err
	}

	parsed.Scheme = "https"
	parsed.Host = host
	if address, addressErr := netip.ParseAddr(host); addressErr == nil && address.Is6() {
		parsed.Host = "[" + host + "]"
	}
	parsed.Path = cleanPath
	parsed.RawPath = ""
	return parsed.String(), nil
}

var nonPublicEndpointPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("fec0::/10"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func allowedAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() ||
		address.IsLoopback() || address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range nonPublicEndpointPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func canonicalPath(value string) (string, error) {
	if strings.Contains(value, "\\") || strings.IndexByte(value, 0) >= 0 {
		return "", ErrInvalidEndpoint
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(value, "/"))
	if cleaned == "." {
		cleaned = "/"
	}
	return cleaned, nil
}

func validEndpointHost(host string) bool {
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "%/\\") {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if character < 'a' || character > 'z' {
				if character < '0' || character > '9' {
					if character != '-' {
						return false
					}
				}
			}
		}
	}
	return true
}
