package attemptpayload

import (
	"context"
	"errors"
)

var ErrPayloadNotFound = errors.New("Attempt payload not found")

const (
	ContentTypeJSON     = "application/json"
	ContentTypeTextUTF8 = "text/plain; charset=utf-8"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusDelivered Status = "delivered"
)

type Scope struct {
	ConversationID         string
	WorkItemID             string
	RunID                  string
	ClaimGeneration        int64
	RuntimeInstanceID      string
	ExecutionBindingDigest string
	CapsuleDigest          string
}

type Binding struct {
	PayloadID string
	Scope
	CallID        string
	Sequence      int64
	ContentType   string
	ContentDigest string
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
	PutAttemptPayload(context.Context, Payload) error
	ReadAttemptPayload(context.Context, Binding) (Payload, error)
	ListPendingAttemptPayloads(context.Context, Scope) ([]Payload, error)
	MarkAttemptPayloadDelivered(context.Context, Binding) error
}

type Authority struct {
	Scope
	ClaimID         string
	AgentInstanceID string
	IncidentID      string
}

type FactStatus string

const (
	FactAccepted  FactStatus = "accepted"
	FactDelivered FactStatus = "delivered"
)

type DeliveryProof string

const (
	ProofProviderContinuation DeliveryProof = "provider_continuation"
	ProofHarnessFinalOutput   DeliveryProof = "harness_final_output"
	ProofRunStreamToolResult  DeliveryProof = "run_stream_tool_result"
)

type Fact struct {
	Binding Binding
	Status  FactStatus
	Proof   DeliveryProof
}

type FactAuthority interface {
	Lookup(context.Context, Authority, string, int64) (Fact, bool, error)
	Accept(context.Context, Authority, Binding) error
	Deliver(context.Context, Authority, Binding, DeliveryProof) error
}
