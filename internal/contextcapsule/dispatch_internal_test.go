package contextcapsule

import "testing"

func TestDispatchAcceptsOpenCodeContextAdapter(t *testing.T) {
	// Mixed Teams can bind an Agent to the OpenCode harness; the context
	// capsule dispatch must accept the opencode context adapter instead of
	// failing closed on an unknown adapter.
	limit, ok := contextAdapterPromptLimit("context:opencode:v1")
	if !ok || limit <= 0 {
		t.Fatalf("context:opencode:v1 prompt limit = %d, %t", limit, ok)
	}
	if _, ok := contextAdapterPromptLimit("context:unknown:v1"); ok {
		t.Fatal("unknown context adapter must stay rejected")
	}
}
