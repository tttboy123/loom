package agentinbox

import "context"

type Mode string

const (
	ModeQueue  Mode = "queue"
	ModeSteer  Mode = "steer"
	ModeInject Mode = "inject"
)

type ContextScope string

const (
	ScopeConversationShared ContextScope = "conversation_shared"
	ScopeTeamShared         ContextScope = "team_shared"
	ScopeAgentPrivate       ContextScope = "agent_private"
	ScopeRoleRestricted     ContextScope = "role_restricted"
	ScopeArtifact           ContextScope = "artifact_scoped"
	ScopeSecretReference    ContextScope = "secret_reference_only"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusConsumed Status = "consumed"
)

// Binding freezes one encrypted input to one Attempt generation and one
// immutable Turn or Step target. Content is deliberately absent.
type Binding struct {
	PayloadID              string       `json:"payload_id"`
	InputID                string       `json:"input_id"`
	Mode                   Mode         `json:"mode"`
	ContextScope           ContextScope `json:"context_scope"`
	ScopeTargetID          string       `json:"scope_target_id,omitempty"`
	ConversationID         string       `json:"conversation_id"`
	SegmentID              string       `json:"segment_id"`
	AgentInstanceID        string       `json:"agent_instance_id"`
	WorkItemID             string       `json:"work_item_id"`
	RunID                  string       `json:"run_id"`
	ClaimGeneration        int64        `json:"claim_generation"`
	RuntimeInstanceID      string       `json:"runtime_instance_id"`
	ExecutionBindingDigest string       `json:"execution_binding_digest"`
	CapsuleDigest          string       `json:"capsule_digest"`
	OrderKey               int64        `json:"order_key"`
	TargetTurnID           string       `json:"target_turn_id,omitempty"`
	TargetTurnSequence     int          `json:"target_turn_sequence,omitempty"`
	TargetStepID           string       `json:"target_step_id,omitempty"`
	TargetStepSequence     int          `json:"target_step_sequence,omitempty"`
	ContentType            string       `json:"content_type"`
	ContentDigest          string       `json:"content_digest"`
}

type Payload struct {
	Binding Binding
	Status  Status
	Content []byte
}

func (payload *Payload) Close() {
	if payload == nil {
		return
	}
	for index := range payload.Content {
		payload.Content[index] = 0
	}
	payload.Content = nil
}

type Store interface {
	PutAgentInput(context.Context, Payload) error
	ReadAgentInput(context.Context, Binding) (Payload, error)
	ListPendingAgentInputs(context.Context, string, string, string, int64) ([]Payload, error)
	MarkAgentInputConsumed(context.Context, Binding) error
	DeleteAgentInput(context.Context, Binding) error
}
