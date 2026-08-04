// Package schedule owns the v0.4.0 SF-W2 ephemeral worker pool, lease /
// generation fencing, Reconciler and lane routing. It is never an authority
// by itself: every worker/attempt fact is an Event Journal record written
// through the accepted AppendBatchIfStreamHeads CAS.
package schedule

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrLeaseExpired        = errors.New("attempt lease expired")
	ErrStaleGeneration     = errors.New("stale attempt generation")
	ErrCapacityOversold    = errors.New("worker pool capacity oversold")
	ErrDuplicateAttempt    = errors.New("duplicate attempt id")
	ErrInfiniteRetry       = errors.New("unbounded retry would exceed frozen policy")
	ErrWorkerStillActive   = errors.New("worker still holds a claim")
	ErrRepairStarved       = errors.New("repair lane starved by development")
	ErrReviewerWriteDenied = errors.New("reviewer may not write product state")
	ErrInvalidInput        = errors.New("invalid schedule input")
)

// Lane mirrors the frozen seven-class lane set relevant to worker routing.
type Lane string

const (
	LaneDevelopment Lane = "development"
	LaneRepair      Lane = "repair"
	LaneTest        Lane = "test"
	LaneReview      Lane = "review"
	LaneIntegration Lane = "integration"
)

// AttemptStatus mirrors SF-EXIT-CONTRACT.md §5 Attempt.status.
type AttemptStatus string

const (
	StatusClaimed       AttemptStatus = "claimed"
	StatusRunning       AttemptStatus = "running"
	StatusSucceeded     AttemptStatus = "succeeded"
	StatusFailed        AttemptStatus = "failed"
	StatusCrashed       AttemptStatus = "crashed"
	StatusStaleRejected AttemptStatus = "stale_rejected"
)

// CrashSeam mirrors the frozen before/after-CAS seam.
type CrashSeam string

const (
	CrashBeforeCAS CrashSeam = "before_cas"
	CrashAfterCAS  CrashSeam = "after_cas"
)

// FailureClass mirrors SF-EXIT-CONTRACT.md §2 (exactly seven classes).
type FailureClass string

const (
	ClassProductDefect       FailureClass = "product_defect"
	ClassTestDefect          FailureClass = "test_defect"
	ClassInfraTransient      FailureClass = "infra_transient"
	ClassContractDefect      FailureClass = "contract_defect"
	ClassIntegrationConflict FailureClass = "integration_conflict"
	ClassCrossLayer          FailureClass = "cross_layer"
	ClassHumanRequired       FailureClass = "human_required"
)

// Attempt is the frozen §5 record as projected from the Journal.
type Attempt struct {
	AttemptID              string        `json:"attempt_id"`
	JobID                  string        `json:"job_id"`
	Generation             int64         `json:"generation"`
	LeaseExpiresAt         string        `json:"lease_expires_at"`
	ClaimCAS               string        `json:"claim_cas"`
	Status                 AttemptStatus `json:"status"`
	CrashSeam              CrashSeam     `json:"crash_seam"`
	CrashEffectCardinality string        `json:"crash_effect_cardinality"`
	FailureClass           FailureClass  `json:"failure_class"`
	EvidenceDigests        []string      `json:"evidence_digests"`
	CandidateBranch        string        `json:"candidate_branch"`
	CandidateWorktree      string        `json:"candidate_worktree"`
	StartedAt              string        `json:"started_at"`
	FinishedAt             string        `json:"finished_at"`
	Lane                   Lane          `json:"lane"`
}

// Worker is a short-lived claim holder: one claim, fresh context, exits on
// completion.
type Worker struct {
	WorkerID   string
	Lane       Lane
	AttemptID  string
	JobID      string
	Generation int64
}

// Pool is an ephemeral per-lane worker pool with a hard capacity bound.
type Pool struct {
	Lane     Lane
	Capacity int
	Active   map[string]Worker // worker_id -> claim
}

func NewPool(lane Lane, capacity int) *Pool {
	if capacity < 1 {
		capacity = 1
	}
	return &Pool{Lane: lane, Capacity: capacity, Active: map[string]Worker{}}
}

// Acquire claims a worker slot if capacity allows. One claim per worker;
// capacity is never oversold.
func (pool *Pool) Acquire(workerID string, attempt Attempt) (Worker, error) {
	if len(pool.Active) >= pool.Capacity {
		return Worker{}, fmt.Errorf("%w: lane %s capacity %d", ErrCapacityOversold, pool.Lane, pool.Capacity)
	}
	if _, exists := pool.Active[workerID]; exists {
		return Worker{}, ErrWorkerStillActive
	}
	worker := Worker{
		WorkerID: workerID, Lane: pool.Lane, AttemptID: attempt.AttemptID,
		JobID: attempt.JobID, Generation: attempt.Generation,
	}
	pool.Active[workerID] = worker
	return worker, nil
}

// Release returns the worker's slot after completion/crash.
func (pool *Pool) Release(workerID string) {
	delete(pool.Active, workerID)
}

// Router routes a classified failure to its lane and applies
// aging/weighted fairness so Repair is never starved by Development.
type Router struct {
	developmentRuns int
	repairWaitAge   int // increments every routing cycle a repair waits
}

func NewRouter() *Router {
	return &Router{}
}

func (router *Router) RecordDevelopmentDispatch() {
	router.developmentRuns++
}

func (router *Router) RecordRepairWait() {
	router.repairWaitAge++
}

// Route determines the lane for a failed attempt. The seven-class taxonomy
// decides, never the Developer.
func (router *Router) Route(class FailureClass) Lane {
	switch class {
	case ClassProductDefect, ClassTestDefect, ClassContractDefect, ClassCrossLayer:
		return LaneRepair
	case ClassInfraTransient:
		return LaneRepair
	case ClassIntegrationConflict:
		return LaneRepair
	case ClassHumanRequired:
		return LaneIntegration // routed to the human lane surface via integration
	default:
		return LaneRepair
	}
}

// RepairDue returns true when repair priority (aging/weighted) outweighs new
// development. Weight grows with the wait age; development is capped.
func (router *Router) RepairDue() bool {
	if router.repairWaitAge <= 0 {
		return false
	}
	// Weighted aging: each repair wait cycle contributes weight 2 so repair
	// gains priority as its wait grows, while a steady flow of new
	// development cannot starve it indefinitely.
	return router.repairWaitAge*2 >= router.developmentRuns
}

// Backoff is the frozen bounded retry policy: exponential backoff with a cap
// and a hard max attempt count. There is no hidden infinite retry.
func Backoff(attemptCount int64, maxAttempts int64, now time.Time) (time.Time, error) {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if attemptCount >= maxAttempts {
		return time.Time{}, ErrInfiniteRetry
	}
	base := time.Second
	delay := base << uint(attemptCount)
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return now.Add(delay), nil
}

func ValidFailureClass(value string) bool {
	switch FailureClass(value) {
	case ClassProductDefect, ClassTestDefect, ClassInfraTransient,
		ClassContractDefect, ClassIntegrationConflict, ClassCrossLayer,
		ClassHumanRequired:
		return true
	default:
		return false
	}
}

func NormalizeStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := append([]string(nil), values...)
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}
