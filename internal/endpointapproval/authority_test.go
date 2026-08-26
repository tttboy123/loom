package endpointapproval

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestApprovalLifecycleBindsCandidateAndIsIdempotent(t *testing.T) {
	t.Parallel()

	candidate := testCandidate(t, "endpoint-a", "account-a", "model-a")
	requestedAt := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC)
	expiresAt := requestedAt.Add(30 * time.Minute)
	request := testCommand(t, candidate, ApprovalCommandInput{
		Kind:             CommandRequest,
		Actor:            "user:reviewer-a",
		OperationID:      "operation-request-a",
		ExpectedRevision: 0,
		OccurredAt:       requestedAt,
		ExpiresAt:        expiresAt,
	})

	record, err := Apply(candidate, nil, request)
	if err != nil {
		t.Fatalf("request approval: %v", err)
	}
	if record.Status() != StatusRequested || record.Revision() != 1 {
		t.Fatalf("request state = (%q, %d), want (%q, 1)", record.Status(), record.Revision(), StatusRequested)
	}
	assertRecordBinding(t, record, candidate)
	if record.RequestedBy() != "user:reviewer-a" ||
		record.RequestOperationID() != "operation-request-a" ||
		!record.RequestedAt().Equal(requestedAt) ||
		!record.ExpiresAt().Equal(expiresAt) {
		t.Fatalf("request authority metadata was not frozen: %#v", record)
	}

	retried, err := Apply(candidate, &record, request)
	if err != nil {
		t.Fatalf("retry request: %v", err)
	}
	if !reflect.DeepEqual(retried, record) {
		t.Fatalf("idempotent request changed record\n got: %#v\nwant: %#v", retried, record)
	}

	approvedAt := requestedAt.Add(5 * time.Minute)
	approve := testCommand(t, candidate, ApprovalCommandInput{
		Kind:                 CommandApprove,
		Actor:                "user:approver-b",
		OperationID:          "operation-approve-a",
		ExpectedRevision:     1,
		ExpectedRecordDigest: record.Digest(),
		OccurredAt:           approvedAt,
	})
	approved, err := Apply(candidate, &record, approve)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status() != StatusApproved || approved.Revision() != 2 {
		t.Fatalf("approved state = (%q, %d), want (%q, 2)", approved.Status(), approved.Revision(), StatusApproved)
	}
	assertRecordBinding(t, approved, candidate)
	if approved.DecidedBy() != "user:approver-b" ||
		approved.TerminalOperationID() != "operation-approve-a" ||
		!approved.DecidedAt().Equal(approvedAt) {
		t.Fatalf("approval authority metadata was not frozen: %#v", approved)
	}

	retried, err = Apply(candidate, &approved, approve)
	if err != nil {
		t.Fatalf("retry approval: %v", err)
	}
	if !reflect.DeepEqual(retried, approved) {
		t.Fatalf("idempotent approval changed record\n got: %#v\nwant: %#v", retried, approved)
	}
}

func TestApprovalRejectAndExpireTransitions(t *testing.T) {
	t.Parallel()

	candidate := testCandidate(t, "endpoint-b", "account-b", "model-b")
	requestedAt := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	t.Run("reject", func(t *testing.T) {
		record := requestRecord(t, candidate, requestedAt, requestedAt.Add(time.Hour), "reject")
		command := testCommand(t, candidate, ApprovalCommandInput{
			Kind:                 CommandReject,
			Actor:                "user:reviewer-b",
			OperationID:          "operation-reject-b",
			ExpectedRevision:     record.Revision(),
			ExpectedRecordDigest: record.Digest(),
			OccurredAt:           requestedAt.Add(time.Minute),
		})
		got, err := Apply(candidate, &record, command)
		if err != nil {
			t.Fatalf("reject: %v", err)
		}
		if got.Status() != StatusRejected || got.Revision() != 2 {
			t.Fatalf("reject state = (%q, %d)", got.Status(), got.Revision())
		}
	})

	t.Run("expire", func(t *testing.T) {
		expiresAt := requestedAt.Add(10 * time.Minute)
		record := requestRecord(t, candidate, requestedAt, expiresAt, "expire")
		command := testCommand(t, candidate, ApprovalCommandInput{
			Kind:                 CommandExpire,
			Actor:                "system:endpoint-review-expirer",
			OperationID:          "operation-expire-b",
			ExpectedRevision:     record.Revision(),
			ExpectedRecordDigest: record.Digest(),
			OccurredAt:           expiresAt,
		})
		got, err := Apply(candidate, &record, command)
		if err != nil {
			t.Fatalf("expire: %v", err)
		}
		if got.Status() != StatusExpired || got.Revision() != 2 {
			t.Fatalf("expiry state = (%q, %d)", got.Status(), got.Revision())
		}
	})
}

func TestApprovalFailsClosedForForeignReplayFutureAndDigestMismatch(t *testing.T) {
	t.Parallel()

	candidate := testCandidate(t, "endpoint-c", "account-c", "model-c")
	requestedAt := time.Date(2026, 8, 23, 11, 0, 0, 0, time.UTC)
	record := requestRecord(t, candidate, requestedAt, requestedAt.Add(time.Hour), "closed")

	foreignCases := []struct {
		name      string
		candidate ReviewCandidate
	}{
		{name: "endpoint fingerprint", candidate: testCandidate(t, "endpoint-foreign", "account-c", "model-c")},
		{name: "account", candidate: testCandidate(t, "endpoint-c", "account-foreign", "model-c")},
		{name: "model digest", candidate: testCandidate(t, "endpoint-c", "account-c", "model-foreign")},
	}
	for _, tc := range foreignCases {
		t.Run("foreign "+tc.name, func(t *testing.T) {
			command := testCommand(t, tc.candidate, ApprovalCommandInput{
				Kind:                 CommandApprove,
				Actor:                "user:approver-c",
				OperationID:          "operation-foreign-" + strings.ReplaceAll(tc.name, " ", "-"),
				ExpectedRevision:     record.Revision(),
				ExpectedRecordDigest: record.Digest(),
				OccurredAt:           requestedAt.Add(time.Minute),
			})
			if _, err := Apply(candidate, &record, command); !errors.Is(err, ErrForeignCandidate) {
				t.Fatalf("error = %v, want ErrForeignCandidate", err)
			}
		})
	}

	tests := []struct {
		name           string
		expected       uint64
		expectedDigest string
		want           error
	}{
		{name: "replay", expected: 0, expectedDigest: record.Digest(), want: ErrRevisionReplay},
		{name: "future", expected: 2, expectedDigest: record.Digest(), want: ErrFutureRevision},
		{name: "record digest mismatch", expected: 1, expectedDigest: testDigest("wrong-record"), want: ErrDigestMismatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			command := testCommand(t, candidate, ApprovalCommandInput{
				Kind:                 CommandApprove,
				Actor:                "user:approver-c",
				OperationID:          "operation-" + strings.ReplaceAll(tc.name, " ", "-"),
				ExpectedRevision:     tc.expected,
				ExpectedRecordDigest: tc.expectedDigest,
				OccurredAt:           requestedAt.Add(time.Minute),
			})
			if _, err := Apply(candidate, &record, command); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}

	approve := testCommand(t, candidate, ApprovalCommandInput{
		Kind:                 CommandApprove,
		Actor:                "user:approver-c",
		OperationID:          "operation-terminal-c",
		ExpectedRevision:     record.Revision(),
		ExpectedRecordDigest: record.Digest(),
		OccurredAt:           requestedAt.Add(time.Minute),
	})
	approved, err := Apply(candidate, &record, approve)
	if err != nil {
		t.Fatalf("approve setup: %v", err)
	}
	replayedRequest := testCommand(t, candidate, ApprovalCommandInput{
		Kind:             CommandRequest,
		Actor:            record.RequestedBy(),
		OperationID:      record.RequestOperationID(),
		ExpectedRevision: 0,
		OccurredAt:       record.RequestedAt(),
		ExpiresAt:        record.ExpiresAt(),
	})
	if _, err := Apply(candidate, &approved, replayedRequest); !errors.Is(err, ErrOperationReplay) {
		t.Fatalf("old exact operation error = %v, want ErrOperationReplay", err)
	}
}

func TestApprovalTimeAndTerminalRulesFailClosed(t *testing.T) {
	t.Parallel()

	candidate := testCandidate(t, "endpoint-d", "account-d", "model-d")
	requestedAt := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	expiresAt := requestedAt.Add(10 * time.Minute)
	record := requestRecord(t, candidate, requestedAt, expiresAt, "time")

	approveLate := testCommand(t, candidate, ApprovalCommandInput{
		Kind:                 CommandApprove,
		Actor:                "user:approver-d",
		OperationID:          "operation-approve-late",
		ExpectedRevision:     1,
		ExpectedRecordDigest: record.Digest(),
		OccurredAt:           expiresAt,
	})
	if _, err := Apply(candidate, &record, approveLate); !errors.Is(err, ErrApprovalExpired) {
		t.Fatalf("late approval error = %v, want ErrApprovalExpired", err)
	}

	expireEarly := testCommand(t, candidate, ApprovalCommandInput{
		Kind:                 CommandExpire,
		Actor:                "system:endpoint-review-expirer",
		OperationID:          "operation-expire-early",
		ExpectedRevision:     1,
		ExpectedRecordDigest: record.Digest(),
		OccurredAt:           expiresAt.Add(-time.Nanosecond),
	})
	if _, err := Apply(candidate, &record, expireEarly); !errors.Is(err, ErrApprovalNotExpired) {
		t.Fatalf("early expiry error = %v, want ErrApprovalNotExpired", err)
	}

	reject := testCommand(t, candidate, ApprovalCommandInput{
		Kind:                 CommandReject,
		Actor:                "user:reviewer-d",
		OperationID:          "operation-reject-d",
		ExpectedRevision:     1,
		ExpectedRecordDigest: record.Digest(),
		OccurredAt:           requestedAt.Add(time.Minute),
	})
	rejected, err := Apply(candidate, &record, reject)
	if err != nil {
		t.Fatalf("reject setup: %v", err)
	}
	secondDecision := testCommand(t, candidate, ApprovalCommandInput{
		Kind:                 CommandApprove,
		Actor:                "user:approver-d",
		OperationID:          "operation-second-decision",
		ExpectedRevision:     rejected.Revision(),
		ExpectedRecordDigest: rejected.Digest(),
		OccurredAt:           requestedAt.Add(2 * time.Minute),
	})
	if _, err := Apply(candidate, &rejected, secondDecision); !errors.Is(err, ErrTerminalApproval) {
		t.Fatalf("second terminal decision error = %v, want ErrTerminalApproval", err)
	}
}

func TestCanonicalDigestsAndStructuresExcludeSensitiveProviderData(t *testing.T) {
	t.Parallel()

	candidateA := testCandidate(t, "endpoint-e", "account-e", "model-e")
	candidateB := testCandidate(t, "endpoint-e", "account-e", "model-e")
	if candidateA.Digest() != candidateB.Digest() {
		t.Fatalf("candidate digest is not deterministic: %q != %q", candidateA.Digest(), candidateB.Digest())
	}
	if candidateA.Digest() != "cda8c169e1c064ab4be62ba0c24cda2ea3a04e5e31944a2eec8788b1f319a74e" {
		t.Fatalf("candidate canonical digest = %q", candidateA.Digest())
	}

	for _, typ := range []reflect.Type{
		reflect.TypeOf(ReviewCandidate{}),
		reflect.TypeOf(ApprovalCommand{}),
		reflect.TypeOf(ApprovalRecord{}),
	} {
		for index := 0; index < typ.NumField(); index++ {
			name := strings.ToLower(typ.Field(index).Name)
			for _, forbidden := range []string{
				"secret", "credential", "password", "apikey", "accesskey",
				"providertoken", "providerbody", "requestbody", "responsebody",
				"rawbody", "payload",
			} {
				if strings.Contains(name, forbidden) {
					t.Fatalf("%s contains forbidden field %q", typ.Name(), typ.Field(index).Name)
				}
			}
		}
	}
}

func testCandidate(t *testing.T, endpointSeed, accountID, modelSeed string) ReviewCandidate {
	t.Helper()
	candidate, err := NewReviewCandidate(ReviewCandidateInput{
		EndpointFingerprint: testDigest(endpointSeed),
		ProviderID:          "provider-openai-compatible",
		AccountID:           accountID,
		Protocol:            "openai-compatible",
		ModelDigest:         testDigest(modelSeed),
		ReviewPolicyVersion: 7,
		ReviewPolicyDigest:  testDigest("review-policy-v7"),
	})
	if err != nil {
		t.Fatalf("new candidate: %v", err)
	}
	return candidate
}

func testCommand(t *testing.T, candidate ReviewCandidate, input ApprovalCommandInput) ApprovalCommand {
	t.Helper()
	command, err := NewApprovalCommand(candidate, input)
	if err != nil {
		t.Fatalf("new command: %v", err)
	}
	return command
}

func requestRecord(
	t *testing.T,
	candidate ReviewCandidate,
	requestedAt, expiresAt time.Time,
	suffix string,
) ApprovalRecord {
	t.Helper()
	command := testCommand(t, candidate, ApprovalCommandInput{
		Kind:             CommandRequest,
		Actor:            "user:requester-" + suffix,
		OperationID:      "operation-request-" + suffix,
		ExpectedRevision: 0,
		OccurredAt:       requestedAt,
		ExpiresAt:        expiresAt,
	})
	record, err := Apply(candidate, nil, command)
	if err != nil {
		t.Fatalf("request record: %v", err)
	}
	return record
}

func assertRecordBinding(t *testing.T, record ApprovalRecord, candidate ReviewCandidate) {
	t.Helper()
	if record.CandidateDigest() != candidate.Digest() ||
		record.EndpointFingerprint() != candidate.EndpointFingerprint() ||
		record.ProviderID() != candidate.ProviderID() ||
		record.AccountID() != candidate.AccountID() ||
		record.Protocol() != candidate.Protocol() ||
		record.ModelDigest() != candidate.ModelDigest() ||
		record.ReviewPolicyVersion() != candidate.ReviewPolicyVersion() ||
		record.ReviewPolicyDigest() != candidate.ReviewPolicyDigest() {
		t.Fatalf("record binding does not match candidate\nrecord: %#v\ncandidate: %#v", record, candidate)
	}
}

func testDigest(seed string) string {
	const hex = "0123456789abcdef"
	result := make([]byte, 64)
	for index := range result {
		result[index] = hex[(index+len(seed))%len(hex)]
	}
	return string(result)
}
