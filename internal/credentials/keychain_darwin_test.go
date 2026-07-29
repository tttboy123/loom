//go:build darwin && cgo

package credentials

import (
	"os"
	"strings"
	"testing"
)

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

func TestKeychainReadExplicitlyDisablesAuthenticationUI(t *testing.T) {
	source, err := os.ReadFile("keychain_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(
		string(source),
		"kSecUseAuthenticationUIFail",
	) {
		t.Fatal("Keychain query does not explicitly fail closed on auth UI")
	}
}
