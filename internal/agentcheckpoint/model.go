package agentcheckpoint

import "context"

const ContentTypeTextUTF8 = "text/plain; charset=utf-8"

// Binding freezes one encrypted model checkpoint to an exact Attempt position.
// Model content is deliberately absent.
type Binding struct {
	CheckpointID           string `json:"checkpoint_id"`
	ConversationID         string `json:"conversation_id"`
	SegmentID              string `json:"segment_id"`
	AttemptID              string `json:"attempt_id"`
	AgentInstanceID        string `json:"agent_instance_id"`
	WorkItemID             string `json:"work_item_id"`
	RunID                  string `json:"run_id"`
	ClaimGeneration        int64  `json:"claim_generation"`
	RuntimeInstanceID      string `json:"runtime_instance_id"`
	ExecutionBindingDigest string `json:"execution_binding_digest"`
	CapsuleDigest          string `json:"capsule_digest"`
	TurnID                 string `json:"turn_id"`
	TurnSequence           int    `json:"turn_sequence"`
	StepID                 string `json:"step_id"`
	StepSequence           int    `json:"step_sequence"`
	ContentType            string `json:"content_type"`
	ContentDigest          string `json:"content_digest"`
}

type Payload struct {
	Binding Binding
	Content []byte
}

// Query identifies a checkpoint without guessing its source Turn or Step.
// Resolution must return exactly one immutable record.
type Query struct {
	ConversationID         string
	SegmentID              string
	AttemptID              string
	AgentInstanceID        string
	WorkItemID             string
	RunID                  string
	ClaimGeneration        int64
	RuntimeInstanceID      string
	ExecutionBindingDigest string
	CapsuleDigest          string
	ContentDigest          string
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
	PutAgentCheckpoint(context.Context, Payload) error
	ReadAgentCheckpoint(context.Context, Binding) (Payload, error)
	ResolveAgentCheckpoint(context.Context, Query) (Payload, error)
	DeleteAgentCheckpoint(context.Context, Binding) error
}
