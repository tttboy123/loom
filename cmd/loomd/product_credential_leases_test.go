package main

import (
	"testing"
	"time"
)

func TestProductCredentialLeaseCoversConfiguredAgentWindow(t *testing.T) {
	if productCredentialLeaseTTL < 10*time.Minute ||
		productCredentialLeaseTTL > 15*time.Minute {
		t.Fatalf("product credential lease TTL = %s", productCredentialLeaseTTL)
	}
}
