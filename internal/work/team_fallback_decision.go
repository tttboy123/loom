package work

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
)

var ErrInvalidTeamFallbackDecisionScope = errors.New(
	"invalid Team fallback decision scope",
)

type TeamFallbackDecisionScopeInput struct {
	Version             int
	TeamInstanceID      string
	PlanDigest          string
	LogicalNodeID       string
	SourceBindingDigest string
	TargetBindingDigest string
}

type TeamFallbackDecisionScope struct {
	version             int
	teamInstanceID      string
	planDigest          string
	logicalNodeID       string
	sourceBindingDigest string
	targetBindingDigest string
	digest              string
}

func NewTeamFallbackDecisionScope(
	input TeamFallbackDecisionScopeInput,
) (TeamFallbackDecisionScope, error) {
	if input.Version != 1 ||
		!validOpaqueID(input.TeamInstanceID) ||
		!validSHA256Hex(input.PlanDigest) ||
		!validOpaqueID(input.LogicalNodeID) ||
		!validSHA256Hex(input.SourceBindingDigest) ||
		!validSHA256Hex(input.TargetBindingDigest) ||
		input.SourceBindingDigest == input.TargetBindingDigest {
		return TeamFallbackDecisionScope{}, ErrInvalidTeamFallbackDecisionScope
	}
	scope := TeamFallbackDecisionScope{
		version:             input.Version,
		teamInstanceID:      input.TeamInstanceID,
		planDigest:          input.PlanDigest,
		logicalNodeID:       input.LogicalNodeID,
		sourceBindingDigest: input.SourceBindingDigest,
		targetBindingDigest: input.TargetBindingDigest,
	}
	scope.digest = canonicalTeamFallbackDecisionScopeDigest(scope)
	return scope, nil
}

func (scope TeamFallbackDecisionScope) Version() int { return scope.version }
func (scope TeamFallbackDecisionScope) TeamInstanceID() string {
	return scope.teamInstanceID
}
func (scope TeamFallbackDecisionScope) PlanDigest() string { return scope.planDigest }
func (scope TeamFallbackDecisionScope) LogicalNodeID() string {
	return scope.logicalNodeID
}
func (scope TeamFallbackDecisionScope) SourceBindingDigest() string {
	return scope.sourceBindingDigest
}
func (scope TeamFallbackDecisionScope) TargetBindingDigest() string {
	return scope.targetBindingDigest
}
func (scope TeamFallbackDecisionScope) Digest() string { return scope.digest }
func (scope TeamFallbackDecisionScope) Valid() bool {
	rebuilt, err := NewTeamFallbackDecisionScope(TeamFallbackDecisionScopeInput{
		Version: scope.version, TeamInstanceID: scope.teamInstanceID,
		PlanDigest: scope.planDigest, LogicalNodeID: scope.logicalNodeID,
		SourceBindingDigest: scope.sourceBindingDigest,
		TargetBindingDigest: scope.targetBindingDigest,
	})
	return err == nil && rebuilt.digest == scope.digest
}

func canonicalTeamFallbackDecisionScopeDigest(
	scope TeamFallbackDecisionScope,
) string {
	digest := sha256.New()
	for _, field := range []string{
		"loom.team-fallback-decision-scope.v1",
		scope.teamInstanceID,
		scope.planDigest,
		scope.logicalNodeID,
		scope.sourceBindingDigest,
		scope.targetBindingDigest,
	} {
		writeTeamFallbackDecisionScopeField(digest, field)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func writeTeamFallbackDecisionScopeField(target hash.Hash, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = target.Write(length[:])
	_, _ = target.Write([]byte(value))
}
