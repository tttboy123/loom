package credentials

import "testing"

func TestProductKeychainHelperSpawnAttemptsAreMonotonic(t *testing.T) {
	before := ProductKeychainHelperSpawnAttempts()
	recordProductKeychainHelperSpawnAttempt()
	recordProductKeychainHelperSpawnAttempt()
	if got := ProductKeychainHelperSpawnAttempts(); got-before != 2 {
		t.Fatalf("spawn attempt delta = %d, want 2", got-before)
	}
}
