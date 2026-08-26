package failurelab

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"
)

const (
	testTargetAccount = "failurelab_tmp_0123456789abcdef0123456789abcdef"
	testPeerAccount   = "failurelab_tmp_fedcba9876543210fedcba9876543210"
)

func TestRunnerClassifiesClosedFailureScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		scenario   Scenario
		stage      Stage
		code       Code
		retryable  bool
		status     int
		retryAfter time.Duration
	}{
		{name: "auth", scenario: ScenarioAuth, stage: StageProviderAuth, code: CodeProviderAuth, status: http.StatusUnauthorized},
		{name: "rate limit", scenario: ScenarioRateLimit, stage: StageProviderRateLimit, code: CodeProviderRateLimit, retryable: true, status: http.StatusTooManyRequests, retryAfter: defaultRetryAfter},
		{name: "timeout", scenario: ScenarioTimeout, stage: StageProviderConnect, code: CodeTimeout, retryable: true},
		{name: "insufficient balance", scenario: ScenarioInsufficientBalance, stage: StageProviderHTTP, code: CodeProviderInsufficientBalance, status: http.StatusPaymentRequired},
		{name: "corrupt vault record", scenario: ScenarioCorruptVaultRecord, stage: StageVaultAADValidation, code: CodeCorruptVaultRecord},
		{name: "revision conflict", scenario: ScenarioRevisionConflict, stage: StageAgentAttemptDispatch, code: CodeCredentialRevisionConflict},
	}

	transport := NewTransport()
	runner := NewRunner(transport)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostic, err := runner.Run(context.Background(), Request{
				Scenario:        test.scenario,
				AccountOpaqueID: testTargetAccount,
				IncidentID:      "incident_91bd",
			})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if diagnostic.Stage != test.stage || diagnostic.Code != test.code ||
				diagnostic.Retryable != test.retryable ||
				diagnostic.AccountOpaqueID != testTargetAccount ||
				diagnostic.IncidentID != "incident_91bd" || diagnostic.Elapsed < 0 {
				t.Fatalf("Run() = %#v", diagnostic)
			}

			outcome, err := transport.execute(context.Background(), test.scenario)
			if err != nil {
				t.Fatalf("transport execute error = %v", err)
			}
			if outcome.httpStatus != test.status || outcome.retryAfter != test.retryAfter {
				t.Fatalf("transport outcome = %#v, want status=%d retry-after=%s", outcome, test.status, test.retryAfter)
			}
		})
	}
}

func TestLoopbackTimeoutIsBoundedWithoutCallerDeadline(t *testing.T) {
	t.Parallel()

	type result struct {
		outcome transportOutcome
		err     error
	}
	completed := make(chan result, 1)
	go func() {
		outcome, err := NewTransport().execute(context.Background(), ScenarioTimeout)
		completed <- result{outcome: outcome, err: err}
	}()

	select {
	case execution := <-completed:
		if execution.err != nil || execution.outcome.code != CodeTimeout ||
			!execution.outcome.retryable {
			t.Fatalf("timeout execution = %#v, error = %v", execution.outcome, execution.err)
		}
	case <-time.After(maxLoopbackExecution):
		t.Fatal("timeout execution exceeded its transport bound")
	}
}

func TestDiagnosticJSONHasOnlyApprovedTypedFields(t *testing.T) {
	t.Parallel()

	diagnostic, err := NewRunner(NewTransport()).Run(context.Background(), Request{
		Scenario:        ScenarioAuth,
		AccountOpaqueID: testTargetAccount,
		IncidentID:      "incident_opaque",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	encoded, err := json.Marshal(diagnostic)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	want := []string{"account_opaque_id", "code", "elapsed", "incident_id", "retryable", "stage"}
	got := make([]string, 0, len(fields))
	for _, field := range want {
		if _, ok := fields[field]; ok {
			got = append(got, field)
		}
	}
	if !reflect.DeepEqual(got, want) || len(fields) != len(want) {
		t.Fatalf("diagnostic fields = %v, want only %v; JSON=%s", fields, want, encoded)
	}
}

func TestRunnerRejectsNonTemporaryProviderAccount(t *testing.T) {
	t.Parallel()

	_, err := NewRunner(NewTransport()).Run(context.Background(), Request{
		Scenario:        ScenarioAuth,
		AccountOpaqueID: "deepseek.production",
		IncidentID:      "incident_opaque",
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Run() error = %v, want ErrInvalidRequest", err)
	}
}

func TestTemporaryProviderAccountsAreOpaqueAndDistinct(t *testing.T) {
	t.Parallel()

	first, err := NewTemporaryAccountOpaqueID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewTemporaryAccountOpaqueID()
	if err != nil {
		t.Fatal(err)
	}
	if !validTemporaryAccountOpaqueID(first) || !validTemporaryAccountOpaqueID(second) ||
		first == second {
		t.Fatalf("temporary accounts first=%q second=%q", first, second)
	}
}

func TestRunTeamProjectsFailureOnlyOntoTargetAgentAndAccount(t *testing.T) {
	t.Parallel()

	result, err := NewRunner(NewTransport()).RunTeam(context.Background(), TeamRequest{
		IncidentID: "incident_team_42",
		Agents: []AgentRequest{
			{AgentOpaqueID: "agent_target", AccountOpaqueID: testTargetAccount, Scenario: ScenarioRateLimit},
			{AgentOpaqueID: "agent_peer", AccountOpaqueID: testPeerAccount, Scenario: ScenarioSuccess},
		},
	})
	if err != nil {
		t.Fatalf("RunTeam() error = %v", err)
	}
	if result.IncidentID != "incident_team_42" || len(result.Agents) != 2 {
		t.Fatalf("RunTeam() = %#v", result)
	}
	target := result.Agents[0]
	if target.AgentOpaqueID != "agent_target" || target.AccountOpaqueID != testTargetAccount ||
		target.Status != AgentFailed || target.Diagnostic == nil {
		t.Fatalf("target = %#v", target)
	}
	if target.Diagnostic.AccountOpaqueID != testTargetAccount ||
		target.Diagnostic.IncidentID != "incident_team_42" ||
		target.Diagnostic.Code != CodeProviderRateLimit {
		t.Fatalf("target diagnostic = %#v", target.Diagnostic)
	}
	peer := result.Agents[1]
	if peer.AgentOpaqueID != "agent_peer" || peer.AccountOpaqueID != testPeerAccount ||
		peer.Status != AgentSucceeded || peer.Diagnostic != nil || peer.Elapsed <= 0 {
		t.Fatalf("peer inherited failure = %#v", peer)
	}
	if target.Elapsed <= 0 {
		t.Fatalf("target did not retain an execution receipt: %#v", target)
	}
}

func TestRunTeamRejectsSharedTemporaryAccount(t *testing.T) {
	t.Parallel()

	_, err := NewRunner(NewTransport()).RunTeam(context.Background(), TeamRequest{
		IncidentID: "incident_shared_account",
		Agents: []AgentRequest{
			{AgentOpaqueID: "agent_target", AccountOpaqueID: testTargetAccount, Scenario: ScenarioAuth},
			{AgentOpaqueID: "agent_peer", AccountOpaqueID: testTargetAccount, Scenario: ScenarioSuccess},
		},
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("RunTeam() error = %v, want ErrInvalidRequest", err)
	}
}

func TestRunnerIsSafeForConcurrentSyntheticTeams(t *testing.T) {
	t.Parallel()

	const runs = 64
	runner := NewRunner(NewTransport())
	start := make(chan struct{})
	errorsByRun := make(chan error, runs)
	var wait sync.WaitGroup
	for index := 0; index < runs; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			result, err := runner.RunTeam(context.Background(), TeamRequest{
				IncidentID: "incident_concurrent",
				Agents: []AgentRequest{
					{AgentOpaqueID: "agent_target", AccountOpaqueID: testTargetAccount, Scenario: ScenarioTimeout},
					{AgentOpaqueID: "agent_peer", AccountOpaqueID: testPeerAccount, Scenario: ScenarioSuccess},
				},
			})
			if err != nil {
				errorsByRun <- err
				return
			}
			if len(result.Agents) != 2 || result.Agents[0].Status != AgentFailed ||
				result.Agents[0].Diagnostic == nil || result.Agents[0].Diagnostic.Code != CodeTimeout ||
				result.Agents[1].Status != AgentSucceeded || result.Agents[1].Diagnostic != nil {
				errorsByRun <- errors.New("concurrent result lost Agent isolation")
			}
		}()
	}
	close(start)
	wait.Wait()
	close(errorsByRun)
	for err := range errorsByRun {
		t.Error(err)
	}
}
