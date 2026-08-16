package contextcapsule_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/contextcapsule"
)

type retrievalStoreFixture struct {
	want    contextcapsule.RetrievalRequest
	content []byte
	err     error
}

func (store *retrievalStoreFixture) RetrieveContextItem(
	_ context.Context,
	request contextcapsule.RetrievalRequest,
) (contextcapsule.RetrievedItem, error) {
	if !reflect.DeepEqual(request, store.want) {
		return contextcapsule.RetrievedItem{}, errors.New("unexpected retrieval request")
	}
	return contextcapsule.RetrievedItem{
		ItemID: request.ItemID, Trust: contextcapsule.TrustObserved,
		Scope:   contextcapsule.ScopeArtifactScoped,
		Content: append([]byte(nil), store.content...), ContentDigest: request.ContentDigest,
	}, store.err
}

type retrievalAuditorFixture struct {
	records []contextcapsule.RetrievalAudit
	err     error
}

func (auditor *retrievalAuditorFixture) RecordContextRetrieval(
	_ context.Context,
	record contextcapsule.RetrievalAudit,
) error {
	auditor.records = append(auditor.records, record)
	return auditor.err
}

func TestScopedRetrieverBindsFrozenAttemptAndWritesContentFreeAudit(t *testing.T) {
	capsule := testRetrievalCapsule(t)
	omitted := capsule.Omitted()[0]
	attempt := contextcapsule.AttemptIdentity{
		WorkItemID: "work-1", RunID: "run-1", ClaimID: "claim-1", ClaimGeneration: 2,
		RuntimeInstanceID: "runtime-1", ExecutionBindingDigest: strings.Repeat("a", 64),
		IncidentID: "incident-1",
	}
	want := contextcapsule.RetrievalRequest{
		Authority: capsule.AuthorityRecord(), ItemID: omitted.ItemID,
		ContentDigest:    omitted.ContentDigest,
		RequesterAgentID: capsule.Target().AgentID,
		RequesterRoleID:  capsule.Target().RoleID,
		ArtifactRef:      omitted.ArtifactRef,
	}
	store := &retrievalStoreFixture{want: want, content: []byte("scoped body")}
	auditor := &retrievalAuditorFixture{}
	retriever, err := contextcapsule.NewScopedRetriever(
		capsule.AuthorityRecord(), attempt, store, auditor,
	)
	if err != nil {
		t.Fatal(err)
	}
	item, err := retriever.Retrieve(context.Background(), contextcapsule.RetrievalProposal{
		ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
		ArtifactRef: omitted.ArtifactRef,
	})
	if err != nil || string(item.Content) != "scoped body" {
		t.Fatalf("item = %#v, %v", item, err)
	}
	defer item.Close()
	wantAudit := contextcapsule.RetrievalAudit{
		IncidentID: attempt.IncidentID, WorkItemID: attempt.WorkItemID,
		RunID: attempt.RunID, ClaimGeneration: attempt.ClaimGeneration,
		RuntimeInstanceID:      attempt.RuntimeInstanceID,
		ExecutionBindingDigest: attempt.ExecutionBindingDigest,
		CapsuleDigest:          capsule.Digest(), ItemID: omitted.ItemID,
		ContentDigest: omitted.ContentDigest, AgentID: capsule.Target().AgentID,
		RoleID: capsule.Target().RoleID, ArtifactRef: omitted.ArtifactRef,
		Result: "disclosed",
	}
	if !reflect.DeepEqual(auditor.records, []contextcapsule.RetrievalAudit{wantAudit}) {
		t.Fatalf("audits = %#v", auditor.records)
	}
}

func TestScopedRetrieverFailsClosedWhenAuditCannotCommit(t *testing.T) {
	capsule := testRetrievalCapsule(t)
	omitted := capsule.Omitted()[0]
	want := contextcapsule.RetrievalRequest{
		Authority: capsule.AuthorityRecord(), ItemID: omitted.ItemID,
		ContentDigest:    omitted.ContentDigest,
		RequesterAgentID: capsule.Target().AgentID,
		RequesterRoleID:  capsule.Target().RoleID, ArtifactRef: omitted.ArtifactRef,
	}
	retriever, err := contextcapsule.NewScopedRetriever(
		capsule.AuthorityRecord(),
		contextcapsule.AttemptIdentity{
			WorkItemID: "work-1", RunID: "run-1", ClaimID: "claim-1", ClaimGeneration: 1,
			RuntimeInstanceID: "runtime-1", ExecutionBindingDigest: strings.Repeat("b", 64),
			IncidentID: "incident-1",
		},
		&retrievalStoreFixture{want: want, content: []byte("must be cleared")},
		&retrievalAuditorFixture{err: errors.New("audit unavailable")},
	)
	if err != nil {
		t.Fatal(err)
	}
	if item, err := retriever.Retrieve(context.Background(), contextcapsule.RetrievalProposal{
		ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
		ArtifactRef: omitted.ArtifactRef,
	}); !errors.Is(err, contextcapsule.ErrInvalidContextRetriever) || item.Content != nil {
		t.Fatalf("item = %#v, error = %v", item, err)
	}
}

func testRetrievalCapsule(t *testing.T) contextcapsule.RoleContextCapsule {
	t.Helper()
	target := testRoleTarget("agent-reviewer", "reviewer", 3)
	target.ArtifactRefs = []string{"artifact:diff-1"}
	capsule, err := contextcapsule.BuildRoleContextCapsule(target, []contextcapsule.ItemInput{
		{
			ItemID: "goal", Kind: contextcapsule.KindConversationGoal,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			Priority: contextcapsule.PrioritySystem, TokenCount: 3, Required: true,
			Content: []byte("Review safely."), SourceType: contextcapsule.SourceAuthority,
			SourceRef: "goal:review",
		},
		{
			ItemID: "diff-detail", Kind: contextcapsule.KindArtifactReference,
			Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
			Priority: contextcapsule.PriorityRetrievable, TokenCount: 5,
			Content: []byte("encrypted diff detail"), SourceType: contextcapsule.SourceObservation,
			SourceRef: "evidence:diff-1", ArtifactRef: "artifact:diff-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return capsule
}
