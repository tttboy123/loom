//go:build !darwin || !cgo

package credentials

import "context"

type KeychainStoreConfig struct {
	ServiceName string
}

type KeychainStore struct{}

func NewKeychainStore(KeychainStoreConfig) (*KeychainStore, error) {
	return nil, ErrCredentialStoreUnavailable
}

func (*KeychainStore) Put(context.Context, string, []byte) error {
	return ErrCredentialStoreUnavailable
}

func (*KeychainStore) Read(context.Context, string) ([]byte, error) {
	return nil, ErrCredentialStoreUnavailable
}

func (*KeychainStore) Delete(context.Context, string) error {
	return ErrCredentialStoreUnavailable
}
