package contextcapsule

import (
	"context"
	"errors"
)

var ErrInvalidContextRetriever = errors.New("invalid scoped Context Capsule retriever")

type RetrievalStore interface {
	RetrieveContextItem(context.Context, RetrievalRequest) (RetrievedItem, error)
}

type AttemptIdentity struct {
	WorkItemID             string
	RunID                  string
	ClaimID                string
	ClaimGeneration        int64
	RuntimeInstanceID      string
	ExecutionBindingDigest string
	IncidentID             string
}

type RetrievalProposal struct {
	ItemID        string
	ContentDigest string
	ArtifactRef   string
}

type RetrievalAudit struct {
	IncidentID             string
	WorkItemID             string
	RunID                  string
	ClaimGeneration        int64
	RuntimeInstanceID      string
	ExecutionBindingDigest string
	CapsuleDigest          string
	ItemID                 string
	ContentDigest          string
	AgentID                string
	RoleID                 string
	ArtifactRef            string
	Result                 string
}

type RetrievalAuditor interface {
	RecordContextRetrieval(context.Context, RetrievalAudit) error
}

type Retriever interface {
	Retrieve(context.Context, RetrievalProposal) (RetrievedItem, error)
}

var _ Retriever = (*ScopedRetriever)(nil)

type ScopedRetriever struct {
	authority AuthorityRecord
	attempt   AttemptIdentity
	store     RetrievalStore
	auditor   RetrievalAuditor
}

func NewScopedRetriever(
	authority AuthorityRecord,
	attempt AttemptIdentity,
	store RetrievalStore,
	auditor RetrievalAuditor,
) (*ScopedRetriever, error) {
	validated, err := ValidateAuthorityRecord(authority)
	if err != nil || !validAttemptIdentity(attempt) || store == nil || auditor == nil {
		return nil, ErrInvalidContextRetriever
	}
	return &ScopedRetriever{
		authority: validated, attempt: attempt, store: store, auditor: auditor,
	}, nil
}

func (retriever *ScopedRetriever) Retrieve(
	ctx context.Context,
	proposal RetrievalProposal,
) (RetrievedItem, error) {
	if retriever == nil || ctx == nil || ctx.Err() != nil ||
		!validIdentifier(proposal.ItemID) || !validDigest(proposal.ContentDigest) ||
		(proposal.ArtifactRef != "" && !validIdentifier(proposal.ArtifactRef)) {
		return RetrievedItem{}, ErrInvalidContextRetriever
	}
	audit := RetrievalAudit{
		IncidentID: retriever.attempt.IncidentID,
		WorkItemID: retriever.attempt.WorkItemID, RunID: retriever.attempt.RunID,
		ClaimGeneration:        retriever.attempt.ClaimGeneration,
		RuntimeInstanceID:      retriever.attempt.RuntimeInstanceID,
		ExecutionBindingDigest: retriever.attempt.ExecutionBindingDigest,
		CapsuleDigest:          retriever.authority.CapsuleDigest,
		ItemID:                 proposal.ItemID, ContentDigest: proposal.ContentDigest,
		AgentID: retriever.authority.AgentID, RoleID: retriever.authority.RoleID,
		ArtifactRef: proposal.ArtifactRef,
	}
	item, readErr := retriever.store.RetrieveContextItem(ctx, RetrievalRequest{
		Authority: retriever.authority, ItemID: proposal.ItemID,
		ContentDigest:    proposal.ContentDigest,
		RequesterAgentID: retriever.authority.AgentID,
		RequesterRoleID:  retriever.authority.RoleID,
		ArtifactRef:      proposal.ArtifactRef,
	})
	if readErr == nil {
		audit.Result = "disclosed"
	} else {
		audit.Result = "denied"
	}
	if auditErr := retriever.auditor.RecordContextRetrieval(ctx, audit); auditErr != nil {
		item.Close()
		return RetrievedItem{}, errors.Join(ErrInvalidContextRetriever, auditErr)
	}
	if readErr != nil {
		item.Close()
		return RetrievedItem{}, readErr
	}
	return item, nil
}

func validAttemptIdentity(value AttemptIdentity) bool {
	return validIdentifier(value.WorkItemID) && validIdentifier(value.RunID) &&
		validIdentifier(value.ClaimID) && value.ClaimGeneration > 0 &&
		validIdentifier(value.RuntimeInstanceID) &&
		validDigest(value.ExecutionBindingDigest) &&
		validIdentifier(value.IncidentID)
}
