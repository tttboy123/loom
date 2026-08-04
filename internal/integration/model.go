// Package integration owns the v0.4.0 SF-W3 single-writer Integrator,
// controlled self-host canary and versioned release lifecycle. Every fact is
// an Event Journal record appended through the accepted
// AppendBatchIfStreamHeads CAS; this package is never an authority by itself.
package integration

import (
	"errors"
)

var (
	ErrStaleIntegration    = errors.New("stale integration rejected")
	ErrDuplicateCanary     = errors.New("canary already ran")
	ErrCapacityOversold    = errors.New("canary capacity oversold")
	ErrUnauthorizedFrame   = errors.New("unauthorized streaming frame")
	ErrStaleGeneration     = errors.New("stale generation frame")
	ErrMalformedFrame      = errors.New("malformed streaming frame")
	ErrInvalidInput        = errors.New("invalid integration input")
	ErrRollbackUnavailable = errors.New("no prior eligible release for rollback")
)

// ReleaseCandidate is the versioned release record.
type ReleaseCandidate struct {
	ReleaseID         string   `json:"release_id"`
	CandidateID       string   `json:"candidate_id"`
	TargetBranch      string   `json:"target_branch"`
	BaseCommit        string   `json:"base_commit"`
	SourceDigest      string   `json:"source_digest"`
	EvidenceDigest    string   `json:"evidence_digest"`
	DependencyDigests []string `json:"dependency_digests"`
	Status            string   `json:"status"`
	AdoptedByRunID    string   `json:"adopted_by_run_id,omitempty"`
	CreatedAt         string   `json:"created_at"`
	CorrelationID     string   `json:"correlation_id"`
}

// CanaryRun is the one-shot offline canary record.
type CanaryRun struct {
	CanaryID          string `json:"canary_id"`
	RunID             string `json:"run_id"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
	ModelID           string `json:"model_id"`
	SkillDigest       string `json:"skill_digest"`
	Status            string `json:"status"`
	EvidenceDigest    string `json:"evidence_digest,omitempty"`
	StartedAt         string `json:"started_at"`
	FinishedAt        string `json:"finished_at,omitempty"`
	CorrelationID     string `json:"correlation_id"`
}

// NodeOutputFrame is the authorized, generation-bound streaming frame.
type NodeOutputFrame struct {
	FrameID       string `json:"frame_id"`
	AttemptID     string `json:"attempt_id"`
	Generation    int64  `json:"generation"`
	NodeID        string `json:"node_id"`
	Kind          string `json:"kind"`
	Content       string `json:"content"`
	Authorized    bool   `json:"authorized"`
	PublishedAt   string `json:"published_at"`
	CorrelationID string `json:"correlation_id"`
}
