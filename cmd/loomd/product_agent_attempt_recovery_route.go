package main

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
)

const productAgentAttemptRecoverySchemaVersion = 1

var errProductInvalidAttemptRecoveryRequest = errors.New("invalid Agent Attempt recovery request")

type productAgentAttemptRecoveryRequest struct {
	SchemaVersion    int    `json:"schema_version"`
	Operation        string `json:"operation"`
	DecisionID       string `json:"decision_id,omitempty"`
	PrincipalID      string `json:"principal_id,omitempty"`
	CandidateDigest  string `json:"candidate_digest,omitempty"`
	CapabilityDigest string `json:"capability_digest,omitempty"`
	IncidentID       string `json:"-"`
}

type productAgentAttemptRecoveryCandidate struct {
	SchemaVersion      int                                      `json:"schema_version"`
	Status             work.AgentAttemptRecoveryCandidateStatus `json:"status"`
	Action             work.AgentAttemptRecoveryAction          `json:"action"`
	DecisionID         string                                   `json:"decision_id,omitempty"`
	CandidateDigest    string                                   `json:"candidate_digest"`
	CapabilityDigest   string                                   `json:"capability_digest"`
	AttemptID          string                                   `json:"attempt_id"`
	TeamInstanceID     string                                   `json:"team_instance_id"`
	SegmentID          string                                   `json:"segment_id"`
	WorkItemID         string                                   `json:"work_item_id"`
	RunID              string                                   `json:"run_id"`
	ClaimGeneration    int64                                    `json:"claim_generation"`
	RuntimeInstanceID  string                                   `json:"runtime_instance_id"`
	AgentInstanceID    string                                   `json:"agent_instance_id"`
	HarnessAdapter     string                                   `json:"harness_adapter"`
	ProviderID         string                                   `json:"provider_id"`
	ProviderAccountID  string                                   `json:"provider_account_id"`
	ModelID            string                                   `json:"model_id"`
	CredentialRevision int64                                    `json:"credential_revision"`
}

type productAgentAttemptRecoveryDecision struct {
	SchemaVersion    int                             `json:"schema_version"`
	DecisionID       string                          `json:"decision_id"`
	Action           work.AgentAttemptRecoveryAction `json:"action"`
	CandidateDigest  string                          `json:"candidate_digest"`
	CapabilityDigest string                          `json:"capability_digest"`
}

type productAgentAttemptRecoveryResume struct {
	SchemaVersion     int    `json:"schema_version"`
	Status            string `json:"status"`
	DecisionID        string `json:"decision_id"`
	CandidateDigest   string `json:"candidate_digest"`
	CapabilityDigest  string `json:"capability_digest"`
	AttemptID         string `json:"attempt_id,omitempty"`
	RuntimeInstanceID string `json:"runtime_instance_id,omitempty"`
}

type productAgentAttemptRecoveryResponse struct {
	SchemaVersion int                                    `json:"schema_version"`
	IncidentID    string                                 `json:"incident_id"`
	Operation     string                                 `json:"operation"`
	Candidates    []productAgentAttemptRecoveryCandidate `json:"candidates,omitempty"`
	Decision      *productAgentAttemptRecoveryDecision   `json:"decision,omitempty"`
	Resume        *productAgentAttemptRecoveryResume     `json:"resume,omitempty"`
}

type productAgentAttemptRecoveryRouteAuthority interface {
	Preview(context.Context) (work.AgentAttemptRecoveryPreview, error)
	Authorize(
		context.Context,
		work.AgentAttemptRecoveryDecisionInput,
	) (work.AgentAttemptRecoveryDecision, error)
	Consume(
		context.Context,
		work.AgentAttemptRecoveryConsumeInput,
	) (*work.AgentAttemptRecoveryDispatchLease, error)
}

type productAgentAttemptRecoveryRouteCompletion interface {
	Resume(
		context.Context,
		*work.AgentAttemptRecoveryDispatchLease,
	) (supervisor.AdapterResult, error)
}

type productAgentAttemptRecoveryRoute interface {
	RecoverAgentAttempt(
		context.Context,
		productAgentAttemptRecoveryRequest,
	) (productAgentAttemptRecoveryResponse, error)
}

type productAgentAttemptRecoveryService struct {
	authority  productAgentAttemptRecoveryRouteAuthority
	completion productAgentAttemptRecoveryRouteCompletion
}

func newProductAgentAttemptRecoveryRoute(
	authority productAgentAttemptRecoveryRouteAuthority,
	completion productAgentAttemptRecoveryRouteCompletion,
) (*productAgentAttemptRecoveryService, error) {
	if nilProductAgentInterface(authority) || nilProductAgentInterface(completion) {
		return nil, errProductInvalidAttemptRecoveryRequest
	}
	return &productAgentAttemptRecoveryService{
		authority: authority, completion: completion,
	}, nil
}

func (service *productAgentAttemptRecoveryService) RecoverAgentAttempt(
	ctx context.Context,
	request productAgentAttemptRecoveryRequest,
) (productAgentAttemptRecoveryResponse, error) {
	if service == nil || nilProductAgentInterface(service.authority) ||
		nilProductAgentInterface(service.completion) || ctx == nil || ctx.Err() != nil ||
		!validProductAgentAttemptRecoveryRequest(request) {
		return productAgentAttemptRecoveryResponse{}, errProductInvalidAttemptRecoveryRequest
	}
	response := productAgentAttemptRecoveryResponse{
		SchemaVersion: productAgentAttemptRecoverySchemaVersion,
		IncidentID:    request.IncidentID, Operation: request.Operation,
	}
	switch request.Operation {
	case "preview":
		preview, err := service.authority.Preview(ctx)
		if err != nil {
			return productAgentAttemptRecoveryResponse{}, err
		}
		if preview.SchemaVersion != productAgentAttemptRecoverySchemaVersion {
			return productAgentAttemptRecoveryResponse{}, errProductInvalidAttemptRecoveryRequest
		}
		response.Candidates = make(
			[]productAgentAttemptRecoveryCandidate, len(preview.Candidates),
		)
		for index, candidate := range preview.Candidates {
			response.Candidates[index] = productAgentAttemptRecoveryCandidateFrom(candidate)
		}
	case "confirm":
		decision, err := service.authority.Authorize(
			ctx,
			work.AgentAttemptRecoveryDecisionInput{
				SchemaVersion: productAgentAttemptRecoverySchemaVersion,
				DecisionID:    request.DecisionID, CorrelationID: request.IncidentID,
				PrincipalID:      request.PrincipalID,
				Action:           work.AgentAttemptRecoveryResumePreModel,
				CandidateDigest:  request.CandidateDigest,
				CapabilityDigest: request.CapabilityDigest,
			},
		)
		if err != nil {
			return productAgentAttemptRecoveryResponse{}, err
		}
		response.Decision = &productAgentAttemptRecoveryDecision{
			SchemaVersion: decision.SchemaVersion, DecisionID: decision.DecisionID,
			Action: decision.Action, CandidateDigest: decision.CandidateDigest,
			CapabilityDigest: decision.CapabilityDigest,
		}
	case "resume":
		lease, err := service.authority.Consume(
			ctx,
			work.AgentAttemptRecoveryConsumeInput{
				SchemaVersion: productAgentAttemptRecoverySchemaVersion,
				CorrelationID: request.IncidentID, DecisionID: request.DecisionID,
				CandidateDigest:  request.CandidateDigest,
				CapabilityDigest: request.CapabilityDigest,
			},
		)
		if err != nil {
			return productAgentAttemptRecoveryResponse{}, err
		}
		attemptID, runtimeID := lease.AttemptID(), lease.RuntimeInstanceID()
		if _, err := service.completion.Resume(ctx, lease); err != nil {
			return productAgentAttemptRecoveryResponse{}, err
		}
		response.Resume = &productAgentAttemptRecoveryResume{
			SchemaVersion: productAgentAttemptRecoverySchemaVersion, Status: "completed",
			DecisionID: request.DecisionID, CandidateDigest: request.CandidateDigest,
			CapabilityDigest: request.CapabilityDigest,
			AttemptID:        attemptID, RuntimeInstanceID: runtimeID,
		}
	default:
		return productAgentAttemptRecoveryResponse{}, errProductInvalidAttemptRecoveryRequest
	}
	return response, nil
}

func validProductAgentAttemptRecoveryRequest(
	request productAgentAttemptRecoveryRequest,
) bool {
	if request.SchemaVersion != productAgentAttemptRecoverySchemaVersion ||
		!validProductDiagnosticIncidentID(request.IncidentID) {
		return false
	}
	switch request.Operation {
	case "preview":
		return request.DecisionID == "" && request.PrincipalID == "" &&
			request.CandidateDigest == "" && request.CapabilityDigest == ""
	case "confirm":
		return validProductJourneyID(request.DecisionID) &&
			validProductDiagnosticIdentifier(request.PrincipalID, 128) &&
			validProductHex(request.CandidateDigest, 64) &&
			validProductHex(request.CapabilityDigest, 64)
	case "resume":
		return validProductJourneyID(request.DecisionID) && request.PrincipalID == "" &&
			validProductHex(request.CandidateDigest, 64) &&
			validProductHex(request.CapabilityDigest, 64)
	default:
		return false
	}
}

func productAgentAttemptRecoveryCandidateFrom(
	candidate work.AgentAttemptRecoveryCandidate,
) productAgentAttemptRecoveryCandidate {
	return productAgentAttemptRecoveryCandidate{
		SchemaVersion: candidate.SchemaVersion, Status: candidate.Status,
		Action: candidate.Action, DecisionID: candidate.DecisionID,
		CandidateDigest:  candidate.CandidateDigest,
		CapabilityDigest: candidate.CapabilityDigest,
		AttemptID:        candidate.AttemptID, TeamInstanceID: candidate.TeamInstanceID,
		SegmentID: candidate.SegmentID, WorkItemID: candidate.WorkItemID,
		RunID: candidate.RunID, ClaimGeneration: candidate.ClaimGeneration,
		RuntimeInstanceID: candidate.RuntimeInstanceID,
		AgentInstanceID:   candidate.AgentInstanceID,
		HarnessAdapter:    candidate.HarnessAdapter, ProviderID: candidate.ProviderID,
		ProviderAccountID: candidate.ProviderAccountID, ModelID: candidate.ModelID,
		CredentialRevision: candidate.CredentialRevision,
	}
}

var _ productAgentAttemptRecoveryRouteAuthority = (*work.AgentAttemptRecoveryAuthority)(nil)
var _ productAgentAttemptRecoveryRouteCompletion = (*productAgentAttemptRecoveryCompletion)(nil)
var _ productAgentAttemptRecoveryRoute = (*productAgentAttemptRecoveryService)(nil)
