package providerendpoint

import "testing"

func TestNewEndpointPolicyCanonicalizesHTTPSAndFingerprintsIt(t *testing.T) {
	policy, err := NewEndpointPolicy("HTTPS://API.Example.COM:443//v1/./chat/../models")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := policy.CanonicalEndpoint(), "https://api.example.com/v1/models"; got != want {
		t.Fatalf("CanonicalEndpoint() = %q, want %q", got, want)
	}
	if got, want := policy.EndpointFingerprint(), "bba3e614ba0be5e6bbe51e7ef6eb717e6e1dc5d74daefedf84eba21a76d64eec"; got != want {
		t.Fatalf("EndpointFingerprint() = %q, want %q", got, want)
	}
}

func TestEndpointPolicyDigestIsVersionedAndDomainSeparated(t *testing.T) {
	policy, err := NewEndpointPolicy("https://api.example.com/v1/models")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := policy.Version(), uint32(1); got != want {
		t.Fatalf("Version() = %d, want %d", got, want)
	}
	if got, want := policy.Digest(), "1af16a13ab65e66dac4a69cdd297d17c0d8585ce7002dbe8b25b69c65804339e"; got != want {
		t.Fatalf("Digest() = %q, want %q", got, want)
	}
	if policy.Digest() == policy.EndpointFingerprint() {
		t.Fatal("policy digest must be domain-separated from endpoint fingerprint")
	}
}

func TestNewEndpointPolicyRejectsUnsafeLiteralAddresses(t *testing.T) {
	for _, endpoint := range []string{
		"https://127.0.0.1/v1",
		"https://10.1.2.3/v1",
		"https://169.254.1.1/v1",
		"https://224.0.0.1/v1",
		"https://0.0.0.0/v1",
		"https://192.0.2.1/v1",
		"https://198.51.100.1/v1",
		"https://203.0.113.1/v1",
		"https://240.0.0.1/v1",
		"https://[::1]/v1",
		"https://[fc00::1]/v1",
		"https://[fe80::1]/v1",
		"https://[ff02::1]/v1",
		"https://[::]/v1",
		"https://[2001:db8::1]/v1",
	} {
		t.Run(endpoint, func(t *testing.T) {
			if _, err := NewEndpointPolicy(endpoint); err == nil {
				t.Fatalf("NewEndpointPolicy(%q) accepted an unsafe address", endpoint)
			}
		})
	}

	if _, err := NewEndpointPolicy("https://93.184.216.34/v1"); err != nil {
		t.Fatalf("public literal address rejected: %v", err)
	}
}

func TestNewEndpointPolicyRejectsNonCanonicalAuthorityFeatures(t *testing.T) {
	for _, endpoint := range []string{
		"http://api.example.com/v1",
		"https://user:password@api.example.com/v1",
		"https://api.example.com:8443/v1",
		"https://api.example.com/v1?region=local",
		"https://api.example.com/v1#fragment",
	} {
		if _, err := NewEndpointPolicy(endpoint); err == nil {
			t.Fatalf("NewEndpointPolicy(%q) accepted a forbidden URL feature", endpoint)
		}
	}
}
