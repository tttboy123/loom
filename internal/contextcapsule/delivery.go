package contextcapsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"loom-pi-rebuild/internal/attemptpayload"
)

var (
	ErrInvalidContextDelivery  = errors.New("invalid Context Capsule delivery")
	ErrContextDeliveryComplete = errors.New("Context Capsule delivery already complete")
)

type DeliveryRequest struct {
	Sequence    int64
	ContentType string
}

type DeliveryEncoder func(RetrievalProposal, RetrievedItem) ([]byte, error)

type DeliveryBroker interface {
	Prepare(
		context.Context,
		RetrievalProposal,
		DeliveryRequest,
		DeliveryEncoder,
	) (attemptpayload.Payload, error)
	Acknowledge(
		context.Context,
		attemptpayload.Binding,
		attemptpayload.DeliveryProof,
	) error
}

type DeliveryCoordinator struct {
	retriever Retriever
	store     attemptpayload.Store
	facts     attemptpayload.FactAuthority
	authority attemptpayload.Authority
}

var _ DeliveryBroker = (*DeliveryCoordinator)(nil)

func NewDeliveryCoordinator(
	capsuleAuthority AuthorityRecord,
	attempt AttemptIdentity,
	retriever Retriever,
	store attemptpayload.Store,
	facts attemptpayload.FactAuthority,
) (*DeliveryCoordinator, error) {
	validated, err := ValidateAuthorityRecord(capsuleAuthority)
	if err != nil || !validAttemptIdentity(attempt) || retriever == nil ||
		store == nil || facts == nil {
		return nil, ErrInvalidContextDelivery
	}
	return &DeliveryCoordinator{
		retriever: retriever, store: store, facts: facts,
		authority: attemptpayload.Authority{
			Scope: attemptpayload.Scope{
				ConversationID: validated.ConversationID,
				WorkItemID:     attempt.WorkItemID, RunID: attempt.RunID,
				ClaimGeneration:        attempt.ClaimGeneration,
				RuntimeInstanceID:      attempt.RuntimeInstanceID,
				ExecutionBindingDigest: attempt.ExecutionBindingDigest,
				CapsuleDigest:          validated.CapsuleDigest,
			},
			ClaimID: attempt.ClaimID, AgentInstanceID: validated.AgentID,
			IncidentID: attempt.IncidentID,
		},
	}, nil
}

func (coordinator *DeliveryCoordinator) Prepare(
	ctx context.Context,
	proposal RetrievalProposal,
	request DeliveryRequest,
	encode DeliveryEncoder,
) (attemptpayload.Payload, error) {
	if coordinator == nil || ctx == nil || ctx.Err() != nil ||
		!validDeliveryProposal(proposal) || !validDeliveryRequest(request) || encode == nil {
		return attemptpayload.Payload{}, ErrInvalidContextDelivery
	}
	callID := DeliveryCallID(proposal, request.Sequence)
	fact, factFound, err := coordinator.facts.Lookup(
		ctx, coordinator.authority, callID, request.Sequence,
	)
	if err != nil {
		return attemptpayload.Payload{}, errors.Join(ErrInvalidContextDelivery, err)
	}
	pending, err := coordinator.store.ListPendingAttemptPayloads(
		ctx, coordinator.authority.Scope,
	)
	if err != nil {
		return attemptpayload.Payload{}, errors.Join(ErrInvalidContextDelivery, err)
	}
	payload, found, err := selectPendingDelivery(pending, callID, request)
	if err != nil {
		return attemptpayload.Payload{}, err
	}
	if found {
		if factFound && fact.Binding != payload.Binding {
			payload.Close()
			return attemptpayload.Payload{}, ErrInvalidContextDelivery
		}
		if factFound && fact.Status == attemptpayload.FactDelivered {
			markErr := coordinator.store.MarkAttemptPayloadDelivered(ctx, payload.Binding)
			payload.Close()
			if markErr != nil {
				return attemptpayload.Payload{}, errors.Join(ErrInvalidContextDelivery, markErr)
			}
			return attemptpayload.Payload{}, ErrContextDeliveryComplete
		}
		if !factFound {
			if err := coordinator.facts.Accept(ctx, coordinator.authority, payload.Binding); err != nil {
				payload.Close()
				return attemptpayload.Payload{}, errors.Join(ErrInvalidContextDelivery, err)
			}
		} else if fact.Status != attemptpayload.FactAccepted {
			payload.Close()
			return attemptpayload.Payload{}, ErrInvalidContextDelivery
		}
		return payload, nil
	}
	if factFound {
		if fact.Status == attemptpayload.FactDelivered &&
			validDeliveryProof(fact.Proof) {
			return attemptpayload.Payload{}, ErrContextDeliveryComplete
		}
		return attemptpayload.Payload{}, ErrInvalidContextDelivery
	}
	item, err := coordinator.retriever.Retrieve(ctx, proposal)
	if err != nil {
		item.Close()
		return attemptpayload.Payload{}, err
	}
	content, encodeErr := encode(proposal, item)
	item.Close()
	if encodeErr != nil || len(content) == 0 {
		clearDeliveryBytes(content)
		return attemptpayload.Payload{}, errors.Join(ErrInvalidContextDelivery, encodeErr)
	}
	digest := sha256.Sum256(content)
	binding := attemptpayload.Binding{
		PayloadID: deterministicDeliveryPayloadID(coordinator.authority.Scope, callID, request),
		Scope:     coordinator.authority.Scope,
		CallID:    callID, Sequence: request.Sequence,
		ContentType:   request.ContentType,
		ContentDigest: hex.EncodeToString(digest[:]),
	}
	payload = attemptpayload.Payload{
		Binding: binding, Status: attemptpayload.StatusPending, Content: content,
	}
	if err := coordinator.store.PutAttemptPayload(ctx, payload); err != nil {
		payload.Close()
		return attemptpayload.Payload{}, errors.Join(ErrInvalidContextDelivery, err)
	}
	if err := coordinator.facts.Accept(ctx, coordinator.authority, binding); err != nil {
		payload.Close()
		return attemptpayload.Payload{}, errors.Join(ErrInvalidContextDelivery, err)
	}
	return payload, nil
}

func (coordinator *DeliveryCoordinator) Acknowledge(
	ctx context.Context,
	binding attemptpayload.Binding,
	proof attemptpayload.DeliveryProof,
) error {
	if coordinator == nil || ctx == nil || ctx.Err() != nil ||
		binding.Scope != coordinator.authority.Scope ||
		!validDeliveryBinding(binding) || !validDeliveryProof(proof) {
		return ErrInvalidContextDelivery
	}
	if err := coordinator.facts.Deliver(
		ctx, coordinator.authority, binding, proof,
	); err != nil {
		return errors.Join(ErrInvalidContextDelivery, err)
	}
	if err := coordinator.store.MarkAttemptPayloadDelivered(ctx, binding); err != nil {
		return errors.Join(ErrInvalidContextDelivery, err)
	}
	return nil
}

func selectPendingDelivery(
	pending []attemptpayload.Payload,
	callID string,
	request DeliveryRequest,
) (attemptpayload.Payload, bool, error) {
	var selected attemptpayload.Payload
	found := false
	for index := range pending {
		candidate := pending[index]
		if candidate.Binding.CallID == callID &&
			candidate.Binding.Sequence == request.Sequence {
			if found || candidate.Status != attemptpayload.StatusPending ||
				candidate.Binding.ContentType != request.ContentType {
				candidate.Close()
				selected.Close()
				clearPendingDeliveries(pending[index+1:])
				return attemptpayload.Payload{}, false, ErrInvalidContextDelivery
			}
			selected = candidate
			found = true
			continue
		}
		candidate.Close()
	}
	return selected, found, nil
}

func clearPendingDeliveries(values []attemptpayload.Payload) {
	for index := range values {
		values[index].Close()
	}
}

func validDeliveryProposal(proposal RetrievalProposal) bool {
	return validIdentifier(proposal.ItemID) && validDigest(proposal.ContentDigest) &&
		(proposal.ArtifactRef == "" || validIdentifier(proposal.ArtifactRef))
}

func validDeliveryRequest(request DeliveryRequest) bool {
	return request.Sequence > 0 && request.ContentType == "application/json"
}

func validDeliveryBinding(binding attemptpayload.Binding) bool {
	return validIdentifier(binding.PayloadID) && validIdentifier(binding.ConversationID) &&
		validIdentifier(binding.WorkItemID) && validIdentifier(binding.RunID) &&
		binding.ClaimGeneration > 0 && validIdentifier(binding.RuntimeInstanceID) &&
		validDigest(binding.ExecutionBindingDigest) && validDigest(binding.CapsuleDigest) &&
		validIdentifier(binding.CallID) && binding.Sequence > 0 &&
		binding.ContentType == "application/json" && validDigest(binding.ContentDigest)
}

func validDeliveryProof(proof attemptpayload.DeliveryProof) bool {
	return proof == attemptpayload.ProofProviderContinuation ||
		proof == attemptpayload.ProofHarnessFinalOutput
}

func DeliveryCallID(proposal RetrievalProposal, sequence int64) string {
	if !validDeliveryProposal(proposal) || sequence <= 0 {
		return ""
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf(
		"loom/context-read-call/v1\x00%s\x00%s\x00%s\x00%d",
		proposal.ItemID, proposal.ContentDigest, proposal.ArtifactRef, sequence,
	)))
	return "context-read-" + hex.EncodeToString(digest[:16])
}

func deterministicDeliveryPayloadID(
	scope attemptpayload.Scope,
	callID string,
	request DeliveryRequest,
) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf(
		"loom/attempt-payload/v1\x00%s\x00%s\x00%d\x00%s\x00%s\x00%d",
		scope.RunID, scope.WorkItemID, scope.ClaimGeneration,
		scope.ExecutionBindingDigest, callID, request.Sequence,
	)))
	return "attempt-payload-" + hex.EncodeToString(digest[:16])
}

func clearDeliveryBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
