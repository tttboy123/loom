package contextcapsule_test

import (
	"errors"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/contextcapsule"
)

func TestRouteSegmentBindingFreezesExactCapsuleAndExecutionRoute(t *testing.T) {
	t.Parallel()
	input := contextcapsule.RouteSegmentBindingInput{
		SegmentID: "segment-attempt-1", ConversationID: "mission:team-1",
		TeamID: "team-1", AgentID: "agent-1", RoleID: "coder", AttemptNumber: 1,
		CapsuleDigest:          strings.Repeat("a", 64),
		ExecutionBindingDigest: strings.Repeat("b", 64),
	}
	binding, err := contextcapsule.NewRouteSegmentBinding(input)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := contextcapsule.ValidateRouteSegmentBinding(binding)
	if err != nil || validated != binding || binding.SchemaVersion != 1 ||
		len(binding.Digest) != 64 {
		t.Fatalf("validated=%#v binding=%#v err=%v", validated, binding, err)
	}

	for name, mutate := range map[string]func(*contextcapsule.RouteSegmentBinding){
		"conversation": func(value *contextcapsule.RouteSegmentBinding) { value.ConversationID = "mission:other" },
		"agent":        func(value *contextcapsule.RouteSegmentBinding) { value.AgentID = "agent-other" },
		"attempt":      func(value *contextcapsule.RouteSegmentBinding) { value.AttemptNumber = 2 },
		"capsule":      func(value *contextcapsule.RouteSegmentBinding) { value.CapsuleDigest = strings.Repeat("c", 64) },
		"execution": func(value *contextcapsule.RouteSegmentBinding) {
			value.ExecutionBindingDigest = strings.Repeat("d", 64)
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := binding
			mutate(&changed)
			if _, err := contextcapsule.ValidateRouteSegmentBinding(changed); !errors.Is(err, contextcapsule.ErrInvalidRouteSegmentBinding) {
				t.Fatalf("error = %v", err)
			}
		})
	}

	secondInput := input
	secondInput.SegmentID = "segment-attempt-2"
	secondInput.AttemptNumber = 2
	second, err := contextcapsule.NewRouteSegmentBinding(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	if second.Digest == binding.Digest || second.SegmentID == binding.SegmentID {
		t.Fatalf("attempt segments collided: first=%#v second=%#v", binding, second)
	}
}
