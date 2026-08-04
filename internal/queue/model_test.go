package queue

import (
	"encoding/json"
	"testing"
)

func TestQueueJobSubmissionValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   JobSubmission
		wantErr bool
	}{
		{
			name: "valid submission",
			input: JobSubmission{
				JobID:                "123e4567-e89b-42d3-a456-426614174000",
				Source:               "user_queued",
				DAGNodeID:            "node-a",
				OwnedPaths:           []string{"internal/queue/model.go"},
				MaxAttempts:          3,
				CapabilityKind:       "feature",
				ExitConditions:       []string{"focused tests pass"},
				VerificationStrategy: "focused + race",
				IntegrationStrategy:  "single-integrator",
				EligibilityAuthority: "user",
			},
		},
		{
			name: "missing job id",
			input: JobSubmission{
				Source: "user_queued", DAGNodeID: "node-a",
				OwnedPaths: []string{"internal/queue/model.go"}, MaxAttempts: 3,
			},
			wantErr: true,
		},
		{
			name: "missing owned paths",
			input: JobSubmission{
				JobID: "123e4567-e89b-42d3-a456-426614174000", Source: "user_queued",
				DAGNodeID: "node-a", MaxAttempts: 3,
			},
			wantErr: true,
		},
		{
			name: "invalid source",
			input: JobSubmission{
				JobID: "123e4567-e89b-42d3-a456-426614174000", Source: "bogus",
				DAGNodeID: "node-a", OwnedPaths: []string{"internal/queue/model.go"},
				MaxAttempts: 3,
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateJobSubmission(test.input)
			if test.wantErr && err == nil {
				t.Fatalf("ValidateJobSubmission(%+v) = nil, want error", test.input)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("ValidateJobSubmission(%+v) = %v, want nil", test.input, err)
			}
		})
	}
}

func TestQueueJobJSONRoundTrip(t *testing.T) {
	job := QueueJob{
		JobID:  "123e4567-e89b-42d3-a456-426614174000",
		Source: "user_queued", DAGNodeID: "node-a",
		Status: StatusQueued, Lane: LaneAdmission,
		OwnedPaths:     []string{"internal/queue/model.go"},
		ResourceClaims: ResourceClaims{Runtime: "pi", Slots: 1, Model: "qwen2.5-coder-1.5b"},
		ExitConditions: []string{"focused tests pass"},
		CapabilityKind: "feature",
	}
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	var decoded QueueJob
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.JobID != job.JobID || decoded.Status != StatusQueued || decoded.Lane != LaneAdmission {
		t.Fatalf("round trip mismatch: %+v", decoded)
	}
	if len(decoded.OwnedPaths) != 1 || decoded.OwnedPaths[0] != "internal/queue/model.go" {
		t.Fatalf("owned paths mismatch: %+v", decoded.OwnedPaths)
	}
}
