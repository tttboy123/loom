package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"

	_ "modernc.org/sqlite"
)

type productFixedVerifierFixture struct{}

func (productFixedVerifierFixture) Verify(
	context.Context,
	string,
	[]byte,
) (credentials.VerificationResult, error) {
	return credentials.VerificationResult{
		Status: credentials.VerificationValid, SafeMessage: "credential verified",
	}, nil
}

func TestProductAccountCredentialVerifierSelectsLatestUnsupersededAccountApproval(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := journal.Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	verifier, err := newProductAccountCredentialVerifier(productFixedVerifierFixture{}, store)
	if err != nil {
		t.Fatal(err)
	}
	candidateA := productEndpointDigest("candidate-a")
	candidateB := productEndpointDigest("candidate-bb")
	approvalA := productEndpointDigest("approval-a")
	approvalB := productEndpointDigest("approval-bb")
	approvalOtherAccount := productEndpointDigest("approval-other-account")
	reviews := []app.EndpointReviewResult{
		productEndpointReview(candidateA, approvalA, "custom-openai.primary"),
		productEndpointReview(candidateB, approvalB, "custom-openai.primary"),
		productEndpointReview(candidateA, approvalOtherAccount, "custom-openai.secondary"),
	}
	for index, review := range reviews {
		appendProductEndpointReview(t, ctx, store, index, review)
	}
	if _, err := verifier.approvedReview(
		ctx, "custom-openai", "custom-openai.primary",
	); err == nil {
		t.Fatal("ambiguous account approvals did not fail closed")
	}
	appendProductEndpointSupersession(t, ctx, store, reviews[0])

	selected, err := verifier.approvedReview(
		ctx, "custom-openai", "custom-openai.primary",
	)
	if err != nil || selected.CandidateDigest != candidateB ||
		selected.ApprovalDigest != approvalB || selected.Revision != 2 {
		t.Fatalf("selected=%#v err=%v", selected, err)
	}
	selected, err = verifier.approvedReview(
		ctx, "custom-openai", "custom-openai.secondary",
	)
	if err != nil || selected.CandidateDigest != candidateA ||
		selected.ApprovalDigest != approvalOtherAccount || selected.Revision != 2 {
		t.Fatalf("selected=%#v err=%v", selected, err)
	}
	if _, err := verifier.approvedReview(
		ctx, "custom-openai", "custom-openai.other",
	); err == nil {
		t.Fatal("foreign Provider Account consumed endpoint approval")
	}
	if got, err := customEndpointModelsURL("https://gateway.example.com/v1"); err != nil || got != "https://gateway.example.com/v1/models" {
		t.Fatalf("models URL=%q err=%v", got, err)
	}
}

func appendProductEndpointSupersession(
	t *testing.T,
	ctx context.Context,
	store *journal.Store,
	review app.EndpointReviewResult,
) {
	t.Helper()
	payload, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(ctx, journal.Event{
		ID: "endpoint-review-superseded", StreamID: "provider-endpoint-review/supersession",
		Seq: 1, IdempotencyKey: "endpoint-review/supersession",
		Type: "ProviderEndpointReviewSuperseded", SchemaVersion: 1,
		EmittedAt: time.Now().UTC(), PayloadJSON: payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func productEndpointReview(
	candidateDigest, approvalDigest, accountID string,
) app.EndpointReviewResult {
	digest := productEndpointDigest("fixture")
	return app.EndpointReviewResult{
		CandidateID: digest, CandidateDigest: candidateDigest,
		AuthorityCandidateDigest: productEndpointDigest("authority-" + candidateDigest),
		ProviderID:               "custom-openai", ProviderAccountID: accountID,
		Protocol: "openai_responses", Endpoint: "https://gateway.example.com/v1",
		EndpointFingerprint: digest, ModelDigest: digest,
		ReviewPolicyVersion: 1, ReviewPolicyDigest: digest,
		Status: "approved", Revision: 2, ApprovalDigest: approvalDigest,
		ApprovedAt: time.Now().UTC().Format(time.RFC3339Nano),
		ExpiresAt:  time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339Nano),
	}
}

func appendProductEndpointReview(
	t *testing.T,
	ctx context.Context,
	store *journal.Store,
	index int,
	review app.EndpointReviewResult,
) {
	t.Helper()
	payload, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	id := "endpoint-review-event-" + string(rune('a'+index))
	if _, err := store.Append(ctx, journal.Event{
		ID: id, StreamID: "provider-endpoint-review/fixture-" + id,
		Seq: 1, IdempotencyKey: "endpoint-review/fixture-" + id,
		Type: "ProviderEndpointReviewApproved", SchemaVersion: 1,
		EmittedAt: time.Now().UTC(), PayloadJSON: payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func productEndpointDigest(seed string) string {
	const hexadecimal = "0123456789abcdef"
	result := make([]byte, 64)
	for index := range result {
		result[index] = hexadecimal[(index+len(seed))%len(hexadecimal)]
	}
	return string(result)
}
