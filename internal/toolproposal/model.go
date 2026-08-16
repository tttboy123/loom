package toolproposal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

type Binding struct {
	ProposalID             string
	ConversationID         string
	WorkItemID             string
	RunID                  string
	ClaimGeneration        int64
	RuntimeInstanceID      string
	AgentInstanceID        string
	ExecutionBindingDigest string
	CapsuleDigest          string
	CallID                 string
	CallDigest             string
	ApprovalID             string
	ApprovalDigest         string
	Tool                   string
	OperationID            string
	IncidentID             string
	ContentDigest          string
}

type Record struct {
	Binding Binding
	Content []byte
}

type ApprovalLookup struct {
	ApprovalID     string
	ApprovalDigest string
	WorkItemID     string
	CallDigest     string
}

func (record *Record) Close() {
	if record == nil {
		return
	}
	for index := range record.Content {
		record.Content[index] = 0
	}
	record.Content = nil
}

func ContentDigest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

type Store interface {
	PutToolProposal(context.Context, Record) error
	ReadToolProposal(context.Context, Binding) (Record, error)
	LookupToolProposal(context.Context, ApprovalLookup) (Record, error)
	DeleteToolProposal(context.Context, Binding) error
}
