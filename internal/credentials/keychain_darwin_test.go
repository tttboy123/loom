//go:build darwin && cgo

package credentials

import "testing"

func TestKeychainStoreConfigurationIsFixedAndBounded(t *testing.T) {
	store, err := NewKeychainStore(KeychainStoreConfig{})
	if err != nil {
		t.Fatalf("NewKeychainStore() error = %v", err)
	}
	if store.serviceName != defaultKeychainServiceName {
		t.Fatalf("service name = %q", store.serviceName)
	}
	for _, invalid := range []string{
		"com.example.other-provider",
		" leading",
		"trailing ",
		"line\nbreak",
		string(make([]byte, 129)),
	} {
		if _, err := NewKeychainStore(KeychainStoreConfig{
			ServiceName: invalid,
		}); err == nil {
			t.Fatalf("accepted service name %q", invalid)
		}
	}
}
