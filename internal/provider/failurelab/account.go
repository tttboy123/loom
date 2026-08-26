package failurelab

import (
	"crypto/rand"
	"encoding/hex"
)

const (
	temporaryAccountPrefix      = "failurelab_tmp_"
	temporaryAccountRandomBytes = 16
)

// NewTemporaryAccountOpaqueID creates an account identifier owned solely by
// this process. It does not correspond to a Vault or Provider account.
func NewTemporaryAccountOpaqueID() (string, error) {
	var random [temporaryAccountRandomBytes]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return temporaryAccountPrefix + hex.EncodeToString(random[:]), nil
}

func validTemporaryAccountOpaqueID(value string) bool {
	if len(value) != len(temporaryAccountPrefix)+(temporaryAccountRandomBytes*2) ||
		value[:len(temporaryAccountPrefix)] != temporaryAccountPrefix {
		return false
	}
	for _, character := range value[len(temporaryAccountPrefix):] {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}
