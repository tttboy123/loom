package work

import (
	"strings"
	"testing"
)

func TestTeamFallbackDecisionScopeIsImmutableAndBindingComplete(t *testing.T) {
	input := TeamFallbackDecisionScopeInput{
		Version: 1, TeamInstanceID: "team-mixed-instance",
		PlanDigest: strings.Repeat("a", 64), LogicalNodeID: "main",
		SourceBindingDigest: strings.Repeat("b", 64),
		TargetBindingDigest: strings.Repeat("c", 64),
	}
	scope, err := NewTeamFallbackDecisionScope(input)
	if err != nil || !scope.Valid() || scope.Version() != 1 ||
		scope.TeamInstanceID() != input.TeamInstanceID ||
		scope.PlanDigest() != input.PlanDigest ||
		scope.LogicalNodeID() != input.LogicalNodeID ||
		scope.SourceBindingDigest() != input.SourceBindingDigest ||
		scope.TargetBindingDigest() != input.TargetBindingDigest ||
		len(scope.Digest()) != 64 {
		t.Fatalf("scope = %#v, err=%v", scope, err)
	}
	changed := input
	changed.PlanDigest = strings.Repeat("d", 64)
	other, err := NewTeamFallbackDecisionScope(changed)
	if err != nil || other.Digest() == scope.Digest() {
		t.Fatalf("plan drift scope = %#v, err=%v", other, err)
	}
	changed = input
	changed.TargetBindingDigest = input.SourceBindingDigest
	if _, err := NewTeamFallbackDecisionScope(changed); err == nil {
		t.Fatal("same source and target binding should fail closed")
	}
}
