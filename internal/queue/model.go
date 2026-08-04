// Package queue implements the Journal-authoritative development queue:
// QueueJob records, the rebuildable queue projection, admission/eligibility
// compilation, the Conflict Arbiter, and digest-bound Gap Proposal /
// Successor compilation. It creates no second authority, database, or
// writer; all state is a projection over the Event Journal.
package queue

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrDenied        = errors.New("queue admission denied")
	ErrInvalidInput  = errors.New("invalid queue input")
	ErrDAGCycle      = errors.New("queue DAG cycle")
	ErrDuplicateWork = errors.New("duplicate active work")
)

type JobStatus string

const (
	StatusQueued         JobStatus = "queued"
	StatusAdmitted       JobStatus = "admitted"
	StatusDispatched     JobStatus = "dispatched"
	StatusInProgress     JobStatus = "in_progress"
	StatusWaitingReview  JobStatus = "waiting_review"
	StatusReadyIntegrate JobStatus = "ready_integrate"
	StatusIntegrated     JobStatus = "integrated"
	StatusRejected       JobStatus = "rejected"
	StatusCancelled      JobStatus = "cancelled"
	StatusHumanRequired  JobStatus = "human_required"
)

type Lane string

const (
	LaneAdmission   Lane = "admission"
	LaneDevelopment Lane = "development"
	LaneRepair      Lane = "repair"
	LaneTest        Lane = "test"
	LaneReview      Lane = "review"
	LaneIntegration Lane = "integration"
	LaneHuman       Lane = "human"
)

type ResourceClaims struct {
	Runtime string `json:"runtime"`
	Slots   int    `json:"slots"`
	Model   string `json:"model"`
}

// QueueJob is the frozen §5 record (SF-EXIT-CONTRACT.md, amended by
// SF-W1-SCHEMA-AMENDMENT-1) as projected from the Event Journal.
type QueueJob struct {
	JobID                   string         `json:"job_id"`
	Source                  string         `json:"source"`
	DAGNodeID               string         `json:"dag_node_id"`
	Dependencies            []string       `json:"dependencies"`
	Status                  JobStatus      `json:"status"`
	Lane                    Lane           `json:"lane"`
	OwnedPaths              []string       `json:"owned_paths"`
	MutexKeys               []string       `json:"mutex_keys"`
	ResourceClaims          ResourceClaims `json:"resource_claims"`
	AttemptCount            int            `json:"attempt_count"`
	MaxAttempts             int            `json:"max_attempts"`
	CapabilityKind          string         `json:"capability_kind"`
	ExitConditions          []string       `json:"exit_conditions"`
	VerificationStrategy    string         `json:"verification_strategy"`
	IntegrationStrategy     string         `json:"integration_strategy"`
	ProtectedAuthorityPaths []string       `json:"protected_authority_paths"`
	CreatedAt               string         `json:"created_at"`
	CorrelationID           string         `json:"correlation_id"`
}

// JobSubmission is the user/bounded-policy admission input.
type JobSubmission struct {
	JobID                   string         `json:"job_id"`
	Source                  string         `json:"source"`
	DAGNodeID               string         `json:"dag_node_id"`
	Dependencies            []string       `json:"dependencies"`
	OwnedPaths              []string       `json:"owned_paths"`
	MutexKeys               []string       `json:"mutex_keys"`
	ResourceClaims          ResourceClaims `json:"resource_claims"`
	MaxAttempts             int            `json:"max_attempts"`
	CapabilityKind          string         `json:"capability_kind"`
	ExitConditions          []string       `json:"exit_conditions"`
	VerificationStrategy    string         `json:"verification_strategy"`
	IntegrationStrategy     string         `json:"integration_strategy"`
	ProtectedAuthorityPaths []string       `json:"protected_authority_paths"`
	EligibilityAuthority    string         `json:"eligibility_authority"`
}

// CompiledJob is the admission-validated projection record fields.
type CompiledJob struct {
	JobID                   string
	Source                  string
	DAGNodeID               string
	Dependencies            []string
	OwnedPaths              []string
	MutexKeys               []string
	ResourceClaims          ResourceClaims
	MaxAttempts             int
	CapabilityKind          string
	ExitConditions          []string
	VerificationStrategy    string
	IntegrationStrategy     string
	ProtectedAuthorityPaths []string
}

func ValidateJobSubmission(input JobSubmission) error {
	if input.JobID == "" || input.DAGNodeID == "" {
		return fmt.Errorf("%w: job_id and dag_node_id are required", ErrInvalidInput)
	}
	switch input.Source {
	case "gap_proposal", "mission", "user_queued", "repair", "test_result",
		"review_verdict":
	default:
		return fmt.Errorf("%w: unknown source %q", ErrInvalidInput, input.Source)
	}
	if len(input.OwnedPaths) == 0 {
		return fmt.Errorf("%w: at least one owned_path is required", ErrInvalidInput)
	}
	if input.MaxAttempts < 1 || input.MaxAttempts > 10 {
		return fmt.Errorf("%w: max_attempts out of range", ErrInvalidInput)
	}
	return nil
}

func (input JobSubmission) compiled() CompiledJob {
	normalize := func(values []string) []string {
		if len(values) == 0 {
			return []string{}
		}
		return append([]string(nil), values...)
	}
	return CompiledJob{
		JobID: input.JobID, Source: input.Source, DAGNodeID: input.DAGNodeID,
		Dependencies:            normalize(input.Dependencies),
		OwnedPaths:              normalize(input.OwnedPaths),
		MutexKeys:               normalize(input.MutexKeys),
		ResourceClaims:          input.ResourceClaims,
		MaxAttempts:             input.MaxAttempts,
		CapabilityKind:          input.CapabilityKind,
		ExitConditions:          normalize(input.ExitConditions),
		VerificationStrategy:    input.VerificationStrategy,
		IntegrationStrategy:     input.IntegrationStrategy,
		ProtectedAuthorityPaths: normalize(input.ProtectedAuthorityPaths),
	}
}

func terminalStatus(status JobStatus) bool {
	switch status {
	case StatusIntegrated, StatusRejected, StatusCancelled, StatusHumanRequired:
		return true
	default:
		return false
	}
}

func thinCapability(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "wrapper", "wrapper-only", "adapter", "adapter-only",
		"coordinator", "coordinator-only", "visual", "visual-only":
		return true
	default:
		return false
	}
}

var protectedAuthorityPrefixes = []string{
	"internal/journal/",
	"internal/assets/authority",
	"internal/assets/validators",
	"internal/work/run_authority",
	"internal/work/team_execution_authority",
	"internal/projection/queue.go",
	"cmd/loomd/",
	"internal/localipc/",
	"internal/queue/",
}

func protectedAuthorityClaim(path string) bool {
	for _, prefix := range protectedAuthorityPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
