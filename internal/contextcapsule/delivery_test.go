package contextcapsule_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
)

type deliveryStoreFixture struct {
	payload attemptpayload.Payload
}

func (store *deliveryStoreFixture) PutAttemptPayload(
	context.Context,
	attemptpayload.Payload,
) error {
	return errors.New("unexpected payload write")
}

func (store *deliveryStoreFixture) ReadAttemptPayload(
	_ context.Context,
	binding attemptpayload.Binding,
) (attemptpayload.Payload, error) {
	if binding != store.payload.Binding {
		return attemptpayload.Payload{}, errors.New("unexpected payload binding")
	}
	return cloneDeliveryPayload(store.payload), nil
}

func (store *deliveryStoreFixture) ListPendingAttemptPayloads(
	_ context.Context,
	scope attemptpayload.Scope,
) ([]attemptpayload.Payload, error) {
	if scope != store.payload.Binding.Scope {
		return nil, errors.New("unexpected payload scope")
	}
	return []attemptpayload.Payload{cloneDeliveryPayload(store.payload)}, nil
}

func (*deliveryStoreFixture) MarkAttemptPayloadDelivered(
	context.Context,
	attemptpayload.Binding,
) error {
	return errors.New("unexpected delivered transition")
}

type writingDeliveryStoreFixture struct {
	payload attemptpayload.Payload
	writes  int
}

func (store *writingDeliveryStoreFixture) PutAttemptPayload(
	_ context.Context,
	payload attemptpayload.Payload,
) error {
	store.writes++
	store.payload = cloneDeliveryPayload(payload)
	return nil
}

func (store *writingDeliveryStoreFixture) ReadAttemptPayload(
	context.Context,
	attemptpayload.Binding,
) (attemptpayload.Payload, error) {
	return attemptpayload.Payload{}, errors.New("unexpected payload read")
}

func (*writingDeliveryStoreFixture) ListPendingAttemptPayloads(
	context.Context,
	attemptpayload.Scope,
) ([]attemptpayload.Payload, error) {
	return nil, nil
}

func (*writingDeliveryStoreFixture) MarkAttemptPayloadDelivered(
	context.Context,
	attemptpayload.Binding,
) error {
	return errors.New("unexpected delivered transition")
}

type deniedRetrieverFixture struct {
	calls int
	err   error
}

func (retriever *deniedRetrieverFixture) Retrieve(
	context.Context,
	contextcapsule.RetrievalProposal,
) (contextcapsule.RetrievedItem, error) {
	retriever.calls++
	return contextcapsule.RetrievedItem{}, retriever.err
}

type deliveryFactAuthorityFixture struct {
	accepted []attemptpayload.Binding
}

func (*deliveryFactAuthorityFixture) Lookup(
	context.Context,
	attemptpayload.Authority,
	string,
	int64,
) (attemptpayload.Fact, bool, error) {
	return attemptpayload.Fact{}, false, nil
}

func (authority *deliveryFactAuthorityFixture) Accept(
	_ context.Context,
	_ attemptpayload.Authority,
	binding attemptpayload.Binding,
) error {
	authority.accepted = append(authority.accepted, binding)
	return nil
}

func (*deliveryFactAuthorityFixture) Deliver(
	context.Context,
	attemptpayload.Authority,
	attemptpayload.Binding,
	attemptpayload.DeliveryProof,
) error {
	return errors.New("unexpected delivery acknowledgement")
}

type countingRetrieverFixture struct {
	calls int
	item  contextcapsule.RetrievedItem
}

func (retriever *countingRetrieverFixture) Retrieve(
	context.Context,
	contextcapsule.RetrievalProposal,
) (contextcapsule.RetrievedItem, error) {
	retriever.calls++
	if retriever.item.Content != nil {
		item := retriever.item
		item.Content = append([]byte(nil), item.Content...)
		return item, nil
	}
	return contextcapsule.RetrievedItem{}, errors.New("unexpected retrieval")
}

func TestDeliveryCoordinatorWritesBoundedDeniedPayloadOnUnretrievableItem(t *testing.T) {
	capsule := testRetrievalCapsule(t)
	omitted := capsule.Omitted()[0]
	store := &writingDeliveryStoreFixture{}
	facts := &deliveryFactAuthorityFixture{}
	retriever := &deniedRetrieverFixture{err: contextcapsule.ErrContextItemNotRetrievable}
	coordinator, err := contextcapsule.NewDeliveryCoordinator(
		capsule.AuthorityRecord(),
		contextcapsule.AttemptIdentity{
			WorkItemID: "work-1", RunID: "run-1", ClaimGeneration: 2,
			RuntimeInstanceID:      "runtime-1",
			ExecutionBindingDigest: strings.Repeat("a", 64),
			IncidentID:             "incident-1", ClaimID: "claim-1",
		},
		retriever, store, facts,
	)
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := coordinator.Prepare(
		context.Background(),
		contextcapsule.RetrievalProposal{
			ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
			ArtifactRef: omitted.ArtifactRef,
		},
		contextcapsule.DeliveryRequest{
			Sequence: 1, ContentType: "application/json",
		},
		func(contextcapsule.RetrievalProposal, contextcapsule.RetrievedItem) ([]byte, error) {
			t.Fatal("encoder must not run for a denied item")
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("bounded denial must produce a delivered payload, got error: %v", err)
	}
	defer delivery.Close()
	if delivery.Status != attemptpayload.StatusPending {
		t.Fatalf("delivery status = %v", delivery.Status)
	}
	if !bytes.Contains(delivery.Content, []byte("context_item_unavailable")) {
		t.Fatalf("denial payload missing bounded marker: %s", delivery.Content)
	}
	if store.writes != 1 {
		t.Fatalf("payload writes = %d, want 1", store.writes)
	}
	if len(facts.accepted) != 1 || facts.accepted[0] != delivery.Binding {
		t.Fatalf("accepted = %#v", facts.accepted)
	}
}

func TestDeliveryCoordinatorRecoversPendingPayloadWithoutReexecutingTool(t *testing.T) {
	capsule := testRetrievalCapsule(t)
	omitted := capsule.Omitted()[0]
	body := []byte(`{"schema_version":1,"content":"persisted result"}`)
	digest := sha256.Sum256(body)
	binding := attemptpayload.Binding{
		PayloadID: "payload-recovery-1",
		Scope: attemptpayload.Scope{
			ConversationID: capsule.AuthorityRecord().ConversationID,
			WorkItemID:     "work-1", RunID: "run-1", ClaimGeneration: 2,
			RuntimeInstanceID:      "runtime-1",
			ExecutionBindingDigest: strings.Repeat("a", 64),
			CapsuleDigest:          capsule.Digest(),
		},
		CallID: contextcapsule.DeliveryCallID(contextcapsule.RetrievalProposal{
			ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
			ArtifactRef: omitted.ArtifactRef,
		}, 1), Sequence: 1, ContentType: "application/json",
		ContentDigest: hex.EncodeToString(digest[:]),
	}
	store := &deliveryStoreFixture{payload: attemptpayload.Payload{
		Binding: binding, Status: attemptpayload.StatusPending,
		Content: append([]byte(nil), body...),
	}}
	facts := &deliveryFactAuthorityFixture{}
	retriever := &countingRetrieverFixture{}
	coordinator, err := contextcapsule.NewDeliveryCoordinator(
		capsule.AuthorityRecord(),
		contextcapsule.AttemptIdentity{
			WorkItemID: "work-1", RunID: "run-1", ClaimGeneration: 2,
			RuntimeInstanceID:      "runtime-1",
			ExecutionBindingDigest: strings.Repeat("a", 64),
			IncidentID:             "incident-1", ClaimID: "claim-1",
		},
		retriever, store, facts,
	)
	if err != nil {
		t.Fatal(err)
	}
	encodeCalls := 0
	delivery, err := coordinator.Prepare(
		context.Background(),
		contextcapsule.RetrievalProposal{
			ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
			ArtifactRef: omitted.ArtifactRef,
		},
		contextcapsule.DeliveryRequest{
			Sequence: 1, ContentType: "application/json",
		},
		func(contextcapsule.RetrievalProposal, contextcapsule.RetrievedItem) ([]byte, error) {
			encodeCalls++
			return nil, errors.New("unexpected encoder")
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer delivery.Close()
	if string(delivery.Content) != string(body) || delivery.Binding != binding {
		t.Fatalf("delivery = %#v", delivery)
	}
	if retriever.calls != 0 || encodeCalls != 0 {
		t.Fatalf("reexecuted retrieval=%d encode=%d", retriever.calls, encodeCalls)
	}
	if len(facts.accepted) != 1 || facts.accepted[0] != binding {
		t.Fatalf("accepted = %#v", facts.accepted)
	}
}

func cloneDeliveryPayload(payload attemptpayload.Payload) attemptpayload.Payload {
	payload.Content = append([]byte(nil), payload.Content...)
	return payload
}

type orderedDeliveryStoreFixture struct {
	order   *[]string
	payload attemptpayload.Payload
}

func (store *orderedDeliveryStoreFixture) PutAttemptPayload(
	_ context.Context,
	payload attemptpayload.Payload,
) error {
	*store.order = append(*store.order, "vault_pending")
	store.payload = cloneDeliveryPayload(payload)
	return nil
}

func (store *orderedDeliveryStoreFixture) ReadAttemptPayload(
	_ context.Context,
	binding attemptpayload.Binding,
) (attemptpayload.Payload, error) {
	if store.payload.Binding != binding {
		return attemptpayload.Payload{}, errors.New("payload not found")
	}
	return cloneDeliveryPayload(store.payload), nil
}

func (store *orderedDeliveryStoreFixture) ListPendingAttemptPayloads(
	_ context.Context,
	scope attemptpayload.Scope,
) ([]attemptpayload.Payload, error) {
	if store.payload.Binding.Scope == scope &&
		store.payload.Status == attemptpayload.StatusPending {
		return []attemptpayload.Payload{cloneDeliveryPayload(store.payload)}, nil
	}
	return []attemptpayload.Payload{}, nil
}

func (store *orderedDeliveryStoreFixture) MarkAttemptPayloadDelivered(
	_ context.Context,
	binding attemptpayload.Binding,
) error {
	if store.payload.Binding != binding || store.payload.Status != attemptpayload.StatusPending {
		return errors.New("unexpected delivered transition")
	}
	*store.order = append(*store.order, "vault_delivered")
	store.payload.Status = attemptpayload.StatusDelivered
	return nil
}

type orderedDeliveryFactAuthorityFixture struct {
	order *[]string
	fact  attemptpayload.Fact
	found bool
}

func (authority *orderedDeliveryFactAuthorityFixture) Lookup(
	context.Context,
	attemptpayload.Authority,
	string,
	int64,
) (attemptpayload.Fact, bool, error) {
	return authority.fact, authority.found, nil
}

func (authority *orderedDeliveryFactAuthorityFixture) Accept(
	_ context.Context,
	_ attemptpayload.Authority,
	binding attemptpayload.Binding,
) error {
	*authority.order = append(*authority.order, "journal_accepted")
	authority.fact = attemptpayload.Fact{
		Binding: binding, Status: attemptpayload.FactAccepted,
	}
	authority.found = true
	return nil
}

func (authority *orderedDeliveryFactAuthorityFixture) Deliver(
	_ context.Context,
	_ attemptpayload.Authority,
	binding attemptpayload.Binding,
	proof attemptpayload.DeliveryProof,
) error {
	if !authority.found || authority.fact.Binding != binding ||
		authority.fact.Status != attemptpayload.FactAccepted {
		return errors.New("unexpected delivery fact")
	}
	*authority.order = append(*authority.order, "journal_delivered")
	authority.fact.Status = attemptpayload.FactDelivered
	authority.fact.Proof = proof
	return nil
}

func TestDeliveryCoordinatorPersistsBeforeDeliveryAndRequiresStrongAcknowledgement(t *testing.T) {
	capsule := testRetrievalCapsule(t)
	omitted := capsule.Omitted()[0]
	order := []string{}
	store := &orderedDeliveryStoreFixture{order: &order}
	facts := &orderedDeliveryFactAuthorityFixture{order: &order}
	retriever := &countingRetrieverFixture{item: contextcapsule.RetrievedItem{
		ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
		ArtifactRef: omitted.ArtifactRef, Content: []byte("retrieved body"),
	}}
	coordinator, err := contextcapsule.NewDeliveryCoordinator(
		capsule.AuthorityRecord(),
		contextcapsule.AttemptIdentity{
			WorkItemID: "work-1", RunID: "run-1", ClaimID: "claim-1",
			ClaimGeneration: 2, RuntimeInstanceID: "runtime-1",
			ExecutionBindingDigest: strings.Repeat("a", 64),
			IncidentID:             "incident-1",
		},
		retriever, store, facts,
	)
	if err != nil {
		t.Fatal(err)
	}
	proposal := contextcapsule.RetrievalProposal{
		ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
		ArtifactRef: omitted.ArtifactRef,
	}
	request := contextcapsule.DeliveryRequest{
		Sequence: 1, ContentType: "application/json",
	}
	delivery, err := coordinator.Prepare(
		context.Background(), proposal, request,
		func(contextcapsule.RetrievalProposal, contextcapsule.RetrievedItem) ([]byte, error) {
			return []byte(`{"schema_version":1,"content":"retrieved body"}`), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer delivery.Close()
	if retriever.calls != 1 || !reflect.DeepEqual(
		order, []string{"vault_pending", "journal_accepted"},
	) {
		t.Fatalf("prepare order=%v retrievals=%d", order, retriever.calls)
	}
	if err := coordinator.Acknowledge(
		context.Background(), delivery.Binding, attemptpayload.DeliveryProof("http_write"),
	); !errors.Is(err, contextcapsule.ErrInvalidContextDelivery) {
		t.Fatalf("weak acknowledgement = %v", err)
	}
	if store.payload.Status != attemptpayload.StatusPending || len(order) != 2 {
		t.Fatalf("weak acknowledgement mutated state: %v %#v", order, store.payload)
	}
	if err := coordinator.Acknowledge(
		context.Background(), delivery.Binding, attemptpayload.ProofProviderContinuation,
	); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{
		"vault_pending", "journal_accepted", "journal_delivered", "vault_delivered",
	}) || store.payload.Status != attemptpayload.StatusDelivered {
		t.Fatalf("acknowledgement order=%v payload=%#v", order, store.payload)
	}
	if _, err := coordinator.Prepare(
		context.Background(), proposal, request,
		func(contextcapsule.RetrievalProposal, contextcapsule.RetrievedItem) ([]byte, error) {
			return nil, errors.New("must not reexecute")
		},
	); !errors.Is(err, contextcapsule.ErrContextDeliveryComplete) || retriever.calls != 1 {
		t.Fatalf("completed recovery = %v retrievals=%d", err, retriever.calls)
	}
}
