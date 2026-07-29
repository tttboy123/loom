//go:build !darwin || !cgo

package credentials

import (
	"context"
	"errors"
	"testing"
)

func TestUnsupportedKeychainStoreFailsClosed(t *testing.T) {
	store, err := NewKeychainStore(KeychainStoreConfig{})
	if store != nil || !errors.Is(err, ErrCredentialStoreUnavailable) {
		t.Fatalf("NewKeychainStore() = %#v, %v", store, err)
	}
	var zero KeychainStore
	if err := zero.Put(
		context.Background(),
		"credential-ref",
		[]byte{1},
	); !errors.Is(err, ErrCredentialStoreUnavailable) {
		t.Fatalf("Put() error = %v", err)
	}
}
