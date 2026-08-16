package credentials

import "sync/atomic"

var productKeychainHelperSpawnAttempts atomic.Uint64

func recordProductKeychainHelperSpawnAttempt() {
	productKeychainHelperSpawnAttempts.Add(1)
}

// ProductKeychainHelperSpawnAttempts reports process-local helper spawn attempts.
func ProductKeychainHelperSpawnAttempts() uint64 {
	return productKeychainHelperSpawnAttempts.Load()
}
