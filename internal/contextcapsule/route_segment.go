package contextcapsule

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

const routeSegmentBindingSchemaVersion = 1

var ErrInvalidRouteSegmentBinding = errors.New("invalid Route Segment binding")

// RouteSegmentBinding freezes one visible Conversation Segment to one Agent
// Attempt route. It carries identity and digests only; context content remains
// in the encrypted Capsule store.
type RouteSegmentBinding struct {
	SchemaVersion          int    `json:"schema_version"`
	SegmentID              string `json:"segment_id"`
	ConversationID         string `json:"conversation_id"`
	TeamID                 string `json:"team_id"`
	AgentID                string `json:"agent_id"`
	RoleID                 string `json:"role_id"`
	AttemptNumber          int    `json:"attempt_number"`
	CapsuleDigest          string `json:"capsule_digest"`
	ExecutionBindingDigest string `json:"execution_binding_digest"`
	Digest                 string `json:"digest"`
}

type RouteSegmentBindingInput struct {
	SegmentID              string
	ConversationID         string
	TeamID                 string
	AgentID                string
	RoleID                 string
	AttemptNumber          int
	CapsuleDigest          string
	ExecutionBindingDigest string
}

func NewRouteSegmentBinding(
	input RouteSegmentBindingInput,
) (RouteSegmentBinding, error) {
	binding := RouteSegmentBinding{
		SchemaVersion:          routeSegmentBindingSchemaVersion,
		SegmentID:              input.SegmentID,
		ConversationID:         input.ConversationID,
		TeamID:                 input.TeamID,
		AgentID:                input.AgentID,
		RoleID:                 input.RoleID,
		AttemptNumber:          input.AttemptNumber,
		CapsuleDigest:          input.CapsuleDigest,
		ExecutionBindingDigest: input.ExecutionBindingDigest,
	}
	if !validRouteSegmentBindingShape(binding) {
		return RouteSegmentBinding{}, ErrInvalidRouteSegmentBinding
	}
	digest, err := routeSegmentBindingDigest(binding)
	if err != nil {
		return RouteSegmentBinding{}, errors.Join(ErrInvalidRouteSegmentBinding, err)
	}
	binding.Digest = digest
	return binding, nil
}

func ValidateRouteSegmentBinding(
	binding RouteSegmentBinding,
) (RouteSegmentBinding, error) {
	if !validRouteSegmentBindingShape(binding) || !validDigest(binding.Digest) {
		return RouteSegmentBinding{}, ErrInvalidRouteSegmentBinding
	}
	expected, err := routeSegmentBindingDigest(binding)
	if err != nil || expected != binding.Digest {
		return RouteSegmentBinding{}, ErrInvalidRouteSegmentBinding
	}
	return binding, nil
}

func validRouteSegmentBindingShape(binding RouteSegmentBinding) bool {
	return binding.SchemaVersion == routeSegmentBindingSchemaVersion &&
		validIdentifier(binding.SegmentID) &&
		validIdentifier(binding.ConversationID) &&
		validIdentifier(binding.TeamID) &&
		validIdentifier(binding.AgentID) &&
		validIdentifier(binding.RoleID) &&
		binding.AttemptNumber > 0 && binding.AttemptNumber <= 64 &&
		validDigest(binding.CapsuleDigest) &&
		validDigest(binding.ExecutionBindingDigest)
}

func routeSegmentBindingDigest(binding RouteSegmentBinding) (string, error) {
	binding.Digest = ""
	encoded, err := json.Marshal(struct {
		Domain  string              `json:"domain"`
		Binding RouteSegmentBinding `json:"binding"`
	}{Domain: "loom/route-segment-binding/v1", Binding: binding})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
