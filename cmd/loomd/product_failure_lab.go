package main

import (
	"context"
	"errors"
	"time"

	"loom-pi-rebuild/internal/provider/failurelab"
)

var errProductFailureLabUnavailable = errors.New("Provider Failure Lab unavailable")

const (
	legacyFailureLabTargetAccount = "failure-lab.target"
	legacyFailureLabPeerAccount   = "failure-lab.healthy-peer"
)

type productFailureLabRequest struct {
	Scenario             string `json:"scenario"`
	ProviderAccountID    string `json:"provider_account_id"`
	HealthyPeerAccountID string `json:"healthy_peer_account_id"`
}

type productFailureLabAgentResult struct {
	AgentID           string `json:"agent_id"`
	ProviderAccountID string `json:"provider_account_id"`
	Status            string `json:"status"`
	Stage             string `json:"stage,omitempty"`
	Code              string `json:"code,omitempty"`
	Retryable         bool   `json:"retryable"`
	ElapsedMillis     int64  `json:"elapsed_milliseconds"`
}

type productFailureLabResult struct {
	SchemaVersion int                            `json:"schema_version"`
	Scenario      string                         `json:"scenario"`
	IncidentID    string                         `json:"incident_id"`
	Agents        []productFailureLabAgentResult `json:"agents"`
}

func runProductFailureLab(
	ctx context.Context,
	runner *failurelab.Runner,
	incidentID string,
	request productFailureLabRequest,
) (productFailureLabResult, error) {
	if ctx == nil || runner == nil || incidentID == "" ||
		!validFailureLabAccountHints(request) {
		return productFailureLabResult{}, errProductFailureLabUnavailable
	}
	targetAccountID, err := failurelab.NewTemporaryAccountOpaqueID()
	if err != nil {
		return productFailureLabResult{}, errProductFailureLabUnavailable
	}
	peerAccountID, err := failurelab.NewTemporaryAccountOpaqueID()
	if err != nil || peerAccountID == targetAccountID {
		return productFailureLabResult{}, errProductFailureLabUnavailable
	}
	result, err := runner.RunTeam(ctx, failurelab.TeamRequest{
		IncidentID: incidentID,
		Agents: []failurelab.AgentRequest{
			{AgentOpaqueID: "target-agent", AccountOpaqueID: targetAccountID,
				Scenario: failurelab.Scenario(request.Scenario)},
			{AgentOpaqueID: "healthy-peer", AccountOpaqueID: peerAccountID,
				Scenario: failurelab.ScenarioSuccess},
		},
	})
	if err != nil {
		return productFailureLabResult{}, errProductFailureLabUnavailable
	}
	projected := productFailureLabResult{
		SchemaVersion: 1, Scenario: request.Scenario,
		IncidentID: result.IncidentID,
		Agents:     make([]productFailureLabAgentResult, len(result.Agents)),
	}
	for index, agent := range result.Agents {
		item := productFailureLabAgentResult{
			AgentID: agent.AgentOpaqueID, ProviderAccountID: agent.AccountOpaqueID,
			Status:        string(agent.Status),
			ElapsedMillis: max(0, agent.Elapsed.Milliseconds()),
		}
		if agent.Elapsed > 0 && item.ElapsedMillis == 0 {
			item.ElapsedMillis = time.Millisecond.Milliseconds()
		}
		if agent.Diagnostic != nil {
			item.Stage = string(agent.Diagnostic.Stage)
			item.Code = string(agent.Diagnostic.Code)
			item.Retryable = agent.Diagnostic.Retryable
		}
		projected.Agents[index] = item
	}
	return projected, nil
}

func validFailureLabAccountHints(request productFailureLabRequest) bool {
	targetValid := request.ProviderAccountID == "" ||
		request.ProviderAccountID == legacyFailureLabTargetAccount
	peerValid := request.HealthyPeerAccountID == "" ||
		request.HealthyPeerAccountID == legacyFailureLabPeerAccount
	return targetValid && peerValid
}
