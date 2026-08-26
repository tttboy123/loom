package failurelab

import (
	"context"
	"sync"
	"time"
)

const maxSyntheticTeamAgents = 64

type AgentStatus string

const (
	AgentSucceeded AgentStatus = "succeeded"
	AgentFailed    AgentStatus = "failed"
)

type AgentRequest struct {
	AgentOpaqueID   string
	AccountOpaqueID string
	Scenario        Scenario
}

type TeamRequest struct {
	IncidentID string
	Agents     []AgentRequest
}

type AgentResult struct {
	AgentOpaqueID   string        `json:"agent_opaque_id"`
	AccountOpaqueID string        `json:"account_opaque_id"`
	Status          AgentStatus   `json:"status"`
	Diagnostic      *Diagnostic   `json:"diagnostic,omitempty"`
	Elapsed         time.Duration `json:"-"`
}

// TeamResult is a deterministic projection: Agents retain request order, and
// each failure diagnostic is attached only to the Agent and account that ran it.
type TeamResult struct {
	IncidentID string        `json:"incident_id"`
	Agents     []AgentResult `json:"agents"`
}

func (runner *Runner) RunTeam(ctx context.Context, request TeamRequest) (TeamResult, error) {
	if runner == nil || runner.transport == nil || ctx == nil ||
		!validOpaqueID(request.IncidentID) || len(request.Agents) < 2 ||
		len(request.Agents) > maxSyntheticTeamAgents || !validAgents(request.Agents) {
		return TeamResult{}, ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return TeamResult{}, ErrRunCancelled
	}

	result := TeamResult{
		IncidentID: request.IncidentID,
		Agents:     make([]AgentResult, len(request.Agents)),
	}
	errorsByAgent := make([]error, len(request.Agents))
	var wait sync.WaitGroup
	for index, agent := range request.Agents {
		index, agent := index, agent
		wait.Add(1)
		go func() {
			defer wait.Done()
			projected := AgentResult{
				AgentOpaqueID: agent.AgentOpaqueID, AccountOpaqueID: agent.AccountOpaqueID,
			}
			outcome, elapsed, err := runner.executeRequest(ctx, Request{
				Scenario: agent.Scenario, AccountOpaqueID: agent.AccountOpaqueID,
				IncidentID: request.IncidentID,
			})
			if err != nil {
				errorsByAgent[index] = err
				return
			}
			projected.Elapsed = elapsed
			if outcome.succeeded {
				projected.Status = AgentSucceeded
				result.Agents[index] = projected
				return
			}
			diagnostic := diagnosticFor(Request{
				Scenario: agent.Scenario, AccountOpaqueID: agent.AccountOpaqueID,
				IncidentID: request.IncidentID,
			}, outcome, elapsed)
			projected.Status = AgentFailed
			projected.Diagnostic = &diagnostic
			result.Agents[index] = projected
		}()
	}
	wait.Wait()
	for _, err := range errorsByAgent {
		if err != nil {
			return TeamResult{}, err
		}
	}
	return result, nil
}

func validAgents(agents []AgentRequest) bool {
	seenAgents := make(map[string]struct{}, len(agents))
	seenAccounts := make(map[string]struct{}, len(agents))
	for _, agent := range agents {
		if !validOpaqueID(agent.AgentOpaqueID) ||
			!validTemporaryAccountOpaqueID(agent.AccountOpaqueID) ||
			!validTeamScenario(agent.Scenario) {
			return false
		}
		if _, duplicate := seenAgents[agent.AgentOpaqueID]; duplicate {
			return false
		}
		if _, duplicate := seenAccounts[agent.AccountOpaqueID]; duplicate {
			return false
		}
		seenAgents[agent.AgentOpaqueID] = struct{}{}
		seenAccounts[agent.AccountOpaqueID] = struct{}{}
	}
	return true
}

func validTeamScenario(scenario Scenario) bool {
	return scenario == ScenarioSuccess || validScenario(scenario)
}
