package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
)

// permissionDecisionRecorder adapts the permissions authority decision facts
// to the execution adapter's DecisionRecorder port.
type permissionDecisionRecorder struct {
	authority *permissions.Authority
}

func (recorder *permissionDecisionRecorder) RecordDecision(
	ctx context.Context,
	jobID string,
	call permissions.ProposedCall,
	verdict permissions.Verdict,
	denial permissions.Denial,
	approvalID, operationID, journeyID string,
) error {
	if recorder == nil || recorder.authority == nil {
		return execution.ErrInvalidExecutionInput
	}
	_, err := recorder.authority.RecordDecision(
		ctx, jobID, call, verdict, denial, approvalID, operationID, journeyID,
	)
	return err
}

// productWorktreeResolver maps a Queue Job to the candidate worktree of its
// most recently claimed attempt. It is the daemon's only resolver; a missing
// claim is fail-closed.
type productWorktreeResolver struct {
	store *journal.Store
}

func (resolver *productWorktreeResolver) Resolve(ctx context.Context, jobID string) (string, error) {
	if resolver == nil || resolver.store == nil || jobID == "" {
		return "", execution.ErrInvalidExecutionInput
	}
	events, err := resolver.store.ReadAll(ctx)
	if err != nil {
		return "", err
	}
	latest := ""
	latestSeq := int64(-1)
	for _, event := range events {
		if event.Type != "AttemptClaimed" {
			continue
		}
		var payload struct {
			JobID             string `json:"job_id"`
			CandidateWorktree string `json:"candidate_worktree"`
		}
		if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil ||
			payload.JobID != jobID || payload.CandidateWorktree == "" {
			continue
		}
		if event.Seq > latestSeq {
			latest = payload.CandidateWorktree
			latestSeq = event.Seq
		}
	}
	if latest == "" {
		return "", errors.New("no claimed candidate worktree for job " + jobID)
	}
	return latest, nil
}

func newExecutionDecisionRecorder(store *journal.Store, now func() time.Time) (*permissionDecisionRecorder, error) {
	authority, err := permissions.NewAuthority(store, now)
	if err != nil {
		return nil, err
	}
	return &permissionDecisionRecorder{authority: authority}, nil
}

var _ = time.Now
