package main

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/execution"
)

const productToolRecoverySchemaVersion = 1

var errProductInvalidToolRecoveryRequest = errors.New("invalid ToolCall recovery request")

type productToolRecoveryRequest struct {
	SchemaVersion   int                          `json:"schema_version"`
	Operation       string                       `json:"operation"`
	DecisionID      string                       `json:"decision_id,omitempty"`
	PrincipalID     string                       `json:"principal_id,omitempty"`
	Action          execution.ToolRecoveryAction `json:"action,omitempty"`
	CandidateDigest string                       `json:"candidate_digest,omitempty"`
	IncidentID      string                       `json:"-"`
}

type productToolRecoveryCandidate struct {
	SchemaVersion      int                                   `json:"schema_version"`
	Status             execution.ToolRecoveryCandidateStatus `json:"status"`
	CandidateDigest    string                                `json:"candidate_digest"`
	DecisionID         string                                `json:"decision_id,omitempty"`
	Decision           execution.ToolRecoveryAction          `json:"decision,omitempty"`
	ExecutionID        string                                `json:"execution_id"`
	JobID              string                                `json:"job_id"`
	CallDigest         string                                `json:"call_digest"`
	Tool               string                                `json:"tool"`
	Generation         int64                                 `json:"generation"`
	OperationID        string                                `json:"operation_id"`
	IncidentID         string                                `json:"incident_id"`
	RecoveryCode       string                                `json:"recovery_code"`
	RecoveryRequiredAt string                                `json:"recovery_required_at"`
	AvailableActions   []execution.ToolRecoveryAction        `json:"available_actions,omitempty"`
}

type productToolRecoveryDecision struct {
	SchemaVersion          int                          `json:"schema_version"`
	DecisionID             string                       `json:"decision_id"`
	Action                 execution.ToolRecoveryAction `json:"action"`
	CandidateDigest        string                       `json:"candidate_digest"`
	ExecutionID            string                       `json:"execution_id"`
	EvidenceID             string                       `json:"evidence_id,omitempty"`
	ObservationDigest      string                       `json:"observation_digest,omitempty"`
	OutputDigest           string                       `json:"output_digest,omitempty"`
	ChangedFilesDigest     string                       `json:"changed_files_digest,omitempty"`
	ReplacementAttemptID   string                       `json:"replacement_attempt_id,omitempty"`
	ReplacementRunID       string                       `json:"replacement_run_id,omitempty"`
	ExecutionBindingDigest string                       `json:"execution_binding_digest,omitempty"`
	ContextCapsuleDigest   string                       `json:"context_capsule_digest,omitempty"`
}

type productToolRecoveryResponse struct {
	SchemaVersion int                            `json:"schema_version"`
	IncidentID    string                         `json:"incident_id"`
	Operation     string                         `json:"operation"`
	Candidates    []productToolRecoveryCandidate `json:"candidates,omitempty"`
	Decision      *productToolRecoveryDecision   `json:"decision,omitempty"`
}

type productToolRecoveryRouteAuthority interface {
	Preview(context.Context) (execution.ToolRecoveryPreview, error)
	Resolve(context.Context, execution.ToolRecoveryDecisionInput) (execution.ToolRecoveryDecision, error)
}

type productToolRecoveryRoute interface {
	RecoverToolCall(context.Context, productToolRecoveryRequest) (productToolRecoveryResponse, error)
}

type productToolRecoveryService struct {
	authority productToolRecoveryRouteAuthority
}

func newProductToolRecoveryRoute(
	authority productToolRecoveryRouteAuthority,
) (*productToolRecoveryService, error) {
	if nilProductAgentInterface(authority) {
		return nil, errProductInvalidToolRecoveryRequest
	}
	return &productToolRecoveryService{authority: authority}, nil
}

func (service *productToolRecoveryService) RecoverToolCall(
	ctx context.Context,
	request productToolRecoveryRequest,
) (productToolRecoveryResponse, error) {
	if service == nil || nilProductAgentInterface(service.authority) ||
		ctx == nil || ctx.Err() != nil || !validProductToolRecoveryRequest(request) {
		return productToolRecoveryResponse{}, errProductInvalidToolRecoveryRequest
	}
	response := productToolRecoveryResponse{
		SchemaVersion: productToolRecoverySchemaVersion,
		IncidentID:    request.IncidentID, Operation: request.Operation,
	}
	switch request.Operation {
	case "preview":
		preview, err := service.authority.Preview(ctx)
		if err != nil {
			return productToolRecoveryResponse{}, err
		}
		if preview.SchemaVersion != productToolRecoverySchemaVersion {
			return productToolRecoveryResponse{}, errProductInvalidToolRecoveryRequest
		}
		response.Candidates = make([]productToolRecoveryCandidate, len(preview.Candidates))
		for index, candidate := range preview.Candidates {
			response.Candidates[index] = productToolRecoveryCandidateFrom(candidate)
		}
	case "resolve":
		decision, err := service.authority.Resolve(ctx, execution.ToolRecoveryDecisionInput{
			SchemaVersion: productToolRecoverySchemaVersion,
			DecisionID:    request.DecisionID, CorrelationID: request.IncidentID,
			PrincipalID: request.PrincipalID, Action: request.Action,
			CandidateDigest: request.CandidateDigest,
		})
		if err != nil {
			return productToolRecoveryResponse{}, err
		}
		response.Decision = productToolRecoveryDecisionFrom(decision)
	default:
		return productToolRecoveryResponse{}, errProductInvalidToolRecoveryRequest
	}
	return response, nil
}

func validProductToolRecoveryRequest(request productToolRecoveryRequest) bool {
	if request.SchemaVersion != productToolRecoverySchemaVersion ||
		!validProductDiagnosticIncidentID(request.IncidentID) {
		return false
	}
	switch request.Operation {
	case "preview":
		return request.DecisionID == "" && request.PrincipalID == "" &&
			request.Action == "" && request.CandidateDigest == ""
	case "resolve":
		return validProductJourneyID(request.DecisionID) &&
			validProductDiagnosticIdentifier(request.PrincipalID, 128) &&
			validProductHex(request.CandidateDigest, 64) &&
			(request.Action == execution.ToolRecoveryAbortAttempt ||
				request.Action == execution.ToolRecoveryAcceptObservedEffect ||
				request.Action == execution.ToolRecoveryRetryInNewAttempt)
	default:
		return false
	}
}

func productToolRecoveryCandidateFrom(
	candidate execution.ToolRecoveryCandidate,
) productToolRecoveryCandidate {
	actions := append([]execution.ToolRecoveryAction(nil), candidate.AvailableActions...)
	return productToolRecoveryCandidate{
		SchemaVersion: candidate.SchemaVersion, Status: candidate.Status,
		CandidateDigest: candidate.CandidateDigest,
		DecisionID:      candidate.DecisionID, Decision: candidate.Decision,
		ExecutionID: candidate.ExecutionID, JobID: candidate.JobID,
		CallDigest: candidate.CallDigest, Tool: string(candidate.Tool),
		Generation: candidate.Generation, OperationID: candidate.OperationID,
		IncidentID: candidate.IncidentID, RecoveryCode: candidate.RecoveryCode,
		RecoveryRequiredAt: candidate.RecoveryRequiredAt,
		AvailableActions:   actions,
	}
}

func productToolRecoveryDecisionFrom(
	decision execution.ToolRecoveryDecision,
) *productToolRecoveryDecision {
	return &productToolRecoveryDecision{
		SchemaVersion: decision.SchemaVersion, DecisionID: decision.DecisionID,
		Action: decision.Action, CandidateDigest: decision.CandidateDigest,
		ExecutionID: decision.ExecutionID, EvidenceID: decision.EvidenceID,
		ObservationDigest:      decision.ObservationDigest,
		OutputDigest:           decision.OutputDigest,
		ChangedFilesDigest:     decision.ChangedFilesDigest,
		ReplacementAttemptID:   decision.ReplacementAttemptID,
		ReplacementRunID:       decision.ReplacementRunID,
		ExecutionBindingDigest: decision.ExecutionBindingDigest,
		ContextCapsuleDigest:   decision.ContextCapsuleDigest,
	}
}

var _ productToolRecoveryRouteAuthority = (*execution.ToolRecoveryAuthority)(nil)
var _ productToolRecoveryRoute = (*productToolRecoveryService)(nil)
