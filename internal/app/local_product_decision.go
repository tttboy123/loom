package app

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/rules"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/verification"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

var (
	ErrInvalidMissionDecision  = errors.New("invalid mission decision")
	ErrMissionDecisionConflict = errors.New("mission decision conflict")
)

type MissionDecisionCommand struct {
	SchemaVersion   int    `json:"schema_version"`
	Operation       string `json:"operation"`
	Kind            string `json:"kind"`
	Action          string `json:"action"`
	MissionID       string `json:"mission_id"`
	TeamInstanceID  string `json:"team_instance_id"`
	ViewVersion     string `json:"view_version"`
	DecisionID      string `json:"decision_id"`
	DecisionDigest  string `json:"decision_digest"`
	LogicalNodeID   string `json:"logical_node_id"`
	AttemptNumber   int    `json:"attempt_number"`
	ClaimGeneration int64  `json:"claim_generation"`
	CorrelationID   string `json:"correlation_id"`
}

type MissionDecisionResult struct {
	SchemaVersion int    `json:"schema_version"`
	MissionID     string `json:"mission_id"`
	DecisionID    string `json:"decision_id"`
	Status        string `json:"status"`
	Authoritative bool   `json:"authoritative"`
	ViewVersion   string `json:"view_version"`
}

type MissionDecisionSheet struct {
	SchemaVersion    int      `json:"schema_version"`
	Kind             string   `json:"kind"`
	MissionID        string   `json:"mission_id"`
	TeamInstanceID   string   `json:"team_instance_id"`
	ViewVersion      string   `json:"view_version"`
	DecisionID       string   `json:"decision_id"`
	DecisionDigest   string   `json:"decision_digest"`
	Title            string   `json:"title"`
	Summary          string   `json:"summary"`
	Requester        string   `json:"requester"`
	Target           string   `json:"target"`
	CommandType      string   `json:"command_type"`
	NetworkAccess    string   `json:"network_access"`
	CredentialAccess string   `json:"credential_access"`
	PermissionScope  string   `json:"permission_scope"`
	AttemptScope     string   `json:"attempt_scope"`
	ExpectedEvidence string   `json:"expected_evidence"`
	TechnicalDetails []string `json:"technical_details"`
	Actions          []string `json:"actions"`
	PreparedActions  []string `json:"prepared_actions"`
	Prepared         bool     `json:"prepared"`
	LogicalNodeID    string   `json:"logical_node_id"`
	AttemptNumber    int      `json:"attempt_number"`
	ClaimGeneration  int64    `json:"claim_generation"`
}

type ApprovalDecisionAuthority interface {
	DecideApproval(
		context.Context,
		rules.ApprovalDecisionRequest,
		string,
	) (rules.ApprovalRequestRecord, error)
}

type TeamNodeAcceptanceAuthority interface {
	CommitTeamNodeAcceptance(
		context.Context,
		work.TeamNodeAcceptanceInput,
	) (work.TeamExecutionRecord, error)
}

type TeamRecoveryAuthority interface {
	ScheduleTeamNodeRecovery(
		context.Context,
		work.TeamRecoveryInput,
	) (work.TeamExecutionRecord, error)
}

type MissionDecisionViewRefresher interface {
	RefreshMissionDecisionView(context.Context) (string, error)
}

type MissionDecisionViewRefreshFunc func(context.Context) (string, error)

func (refresh MissionDecisionViewRefreshFunc) RefreshMissionDecisionView(
	ctx context.Context,
) (string, error) {
	return refresh(ctx)
}

type PreparedAuthorizationDecision struct {
	Sheet           MissionDecisionSheet
	Pending         rules.ApprovalRequestRecord
	Context         rules.ActionContext
	Decision        rules.Decision
	Authority       ApprovalDecisionAuthority
	Deny            rules.ApprovalDecisionRequest
	AllowOnce       rules.ApprovalDecisionRequest
	AllowForMission rules.ApprovalDecisionRequest
	Refresh         MissionDecisionViewRefresher
}

type PreparedReviewDecision struct {
	Sheet          MissionDecisionSheet
	Authority      TeamNodeAcceptanceAuthority
	RequestChanges *work.TeamNodeAcceptanceInput
	AcceptResult   *work.TeamNodeAcceptanceInput
	Refresh        MissionDecisionViewRefresher
}

type PreparedRecoveryDecision struct {
	Sheet           MissionDecisionSheet
	Attempt         work.TeamAttemptRecord
	Authority       TeamRecoveryAuthority
	StopMission     *work.TeamRecoveryInput
	StartNewAttempt *work.TeamRecoveryInput
	Refresh         MissionDecisionViewRefresher
}

type PreparedMissionDecisions struct {
	Authorizations []PreparedAuthorizationDecision
	Reviews        []PreparedReviewDecision
	Recoveries     []PreparedRecoveryDecision
}

type ControlledMissionDecisionFixtureConfig struct {
	Database          *sql.DB
	ArtifactRoot      string
	AuthoritativeTime time.Time
	FixtureID         string
}

type controlledMissionDecisionFixtureClock struct {
	mu    sync.RWMutex
	fixed time.Time
	live  bool
}

func (clock *controlledMissionDecisionFixtureClock) Now() time.Time {
	clock.mu.RLock()
	defer clock.mu.RUnlock()
	if clock.live {
		now := time.Now().UTC()
		if !now.After(clock.fixed) {
			return clock.fixed.Add(time.Second)
		}
		return now
	}
	return clock.fixed
}

func (clock *controlledMissionDecisionFixtureClock) EnableLive() {
	clock.mu.Lock()
	clock.live = true
	clock.mu.Unlock()
}

type controlledMissionDecisionAuthorizer struct {
	clock *controlledMissionDecisionFixtureClock
	seed  string
}

func (authorizer *controlledMissionDecisionAuthorizer) AuthorizeRuleSet(
	ctx context.Context,
	request rules.RuleSetActivationRequest,
) (rules.AuthorizedRuleSetActivation, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedRuleSetActivation{}, err
	}
	issuedAt := authorizer.clock.Now()
	return rules.NewAuthorizedRuleSetActivation(
		request,
		"local-owner",
		missionDecisionDigestFields(
			authorizer.seed,
			"rule-set",
			request.RuleSet().Digest(),
			request.CorrelationID(),
		),
		issuedAt,
		issuedAt.Add(5*time.Minute),
	)
}

func (authorizer *controlledMissionDecisionAuthorizer) AuthorizeApprovalDecision(
	ctx context.Context,
	request rules.ApprovalDecisionRequest,
) (rules.AuthorizedApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedApprovalDecision{}, err
	}
	issuedAt := authorizer.clock.Now()
	return rules.NewAuthorizedApprovalDecision(
		request,
		"local-owner",
		missionDecisionDigestFields(
			authorizer.seed,
			"approval",
			request.ApprovalRequestID(),
			request.ApprovalRequestDigest(),
			request.Decision(),
			request.CorrelationID(),
		),
		issuedAt,
		issuedAt.Add(5*time.Minute),
	)
}

type controlledMissionRuntimeProbe struct {
	fixtureID string
	instance  loomruntime.RuntimeInstance
}

func (probe controlledMissionRuntimeProbe) ID() string {
	return "probe." + probe.fixtureID
}

func (probe controlledMissionRuntimeProbe) ObserveRuntime(
	ctx context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []loomruntime.RuntimeObservation{{
		SourceProbeID: probe.ID(),
		Instance:      probe.instance,
		ModelIDs:      []string{"fixture/no-provider"},
	}}, nil
}

type controlledMissionScenario struct {
	plan           teams.ExecutionPlan
	semantics      TeamNodeSemantics
	attempt        work.TeamAttemptRecord
	receipt        evidence.AttemptReceipt
	classification verification.Classification
}

func BuildControlledMissionDecisionFixture(
	ctx context.Context,
	config ControlledMissionDecisionFixtureConfig,
) (PreparedMissionDecisions, error) {
	if ctx == nil || ctx.Err() != nil ||
		config.Database == nil ||
		config.ArtifactRoot == "" ||
		!validDecisionID(config.FixtureID) ||
		config.AuthoritativeTime.IsZero() ||
		config.AuthoritativeTime.Location() != time.UTC {
		return PreparedMissionDecisions{}, ErrInvalidMissionDecision
	}
	store := journal.NewStore(config.Database)
	events, err := store.ReadAll(ctx)
	if err != nil || len(events) != 0 {
		return PreparedMissionDecisions{}, ErrInvalidMissionDecision
	}
	clock := &controlledMissionDecisionFixtureClock{
		fixed: config.AuthoritativeTime,
	}
	runtimeID := "runtime." + config.FixtureID
	if err := commitControlledMissionRuntime(
		ctx,
		store,
		runtimeID,
		config.FixtureID,
		config.AuthoritativeTime,
	); err != nil {
		return PreparedMissionDecisions{}, err
	}
	workAuthority, err := work.NewAuthority(
		store,
		clock.Now,
		cryptorand.Reader,
	)
	if err != nil {
		return PreparedMissionDecisions{}, err
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		return PreparedMissionDecisions{}, err
	}
	readModel := projection.New(config.Database)
	if err := readModel.Rebuild(ctx); err != nil {
		return PreparedMissionDecisions{}, err
	}
	artifacts, err := evidence.NewStore(config.ArtifactRoot)
	if err != nil {
		return PreparedMissionDecisions{}, err
	}
	defer artifacts.Close()

	authDeny, err := buildControlledAuthorizationMission(
		ctx,
		store,
		workAuthority,
		readModel,
		clock,
		runtimeID,
		config.FixtureID,
		"team-auth-deny",
	)
	if err != nil {
		return PreparedMissionDecisions{}, err
	}
	authAllow, err := buildControlledAuthorizationMission(
		ctx,
		store,
		workAuthority,
		readModel,
		clock,
		runtimeID,
		config.FixtureID,
		"team-auth-allow",
	)
	if err != nil {
		return PreparedMissionDecisions{}, err
	}
	reviewMissing, err := buildControlledTerminalMission(
		ctx,
		workAuthority,
		readModel,
		artifacts,
		clock.Now(),
		runtimeID,
		"team-review-missing",
		2,
		verification.AcceptanceRiskHigh,
		"succeeded",
	)
	if err != nil {
		return PreparedMissionDecisions{}, err
	}
	if reviewMissing.attempt.EvidenceID() == "" {
		return PreparedMissionDecisions{}, ErrInvalidMissionDecision
	}
	reviewReady, err := buildControlledTerminalMission(
		ctx,
		workAuthority,
		readModel,
		artifacts,
		clock.Now(),
		runtimeID,
		"team-review-ready",
		1,
		verification.AcceptanceRiskLow,
		"succeeded",
	)
	if err != nil {
		return PreparedMissionDecisions{}, err
	}
	recoveryReady, err := buildControlledTerminalMission(
		ctx,
		workAuthority,
		readModel,
		artifacts,
		clock.Now(),
		runtimeID,
		"team-recovery",
		2,
		verification.AcceptanceRiskLow,
		"failed",
	)
	if err != nil {
		return PreparedMissionDecisions{}, err
	}
	reviewInput, err := controlledReviewInput(
		reviewReady,
		config.AuthoritativeTime,
	)
	if err != nil {
		return PreparedMissionDecisions{}, err
	}
	recoveryInput, err := controlledRecoveryInput(
		recoveryReady,
		config.AuthoritativeTime,
	)
	if err != nil {
		return PreparedMissionDecisions{}, err
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return PreparedMissionDecisions{}, err
	}
	viewVersion := readModel.GlobalReadView().Version()
	refresh := MissionDecisionViewRefreshFunc(func(ctx context.Context) (
		string,
		error,
	) {
		if err := readModel.Rebuild(ctx); err != nil {
			return "", err
		}
		return readModel.GlobalReadView().Version(), nil
	})
	authDeny.Sheet.ViewVersion = viewVersion
	authDeny.Refresh = refresh
	authAllow.Sheet.ViewVersion = viewVersion
	authAllow.Refresh = refresh
	reviewSheet := controlledWorkDecisionSheet(
		"review",
		reviewReady,
		viewVersion,
		reviewInput.Decision.Digest(),
		[]string{"not_now", "request_changes", "accept_result"},
		[]string{"accept_result"},
	)
	recoverySheet := controlledWorkDecisionSheet(
		"recovery",
		recoveryReady,
		viewVersion,
		recoveryInput.Decision.Digest(),
		[]string{
			"not_now",
			"stop_mission",
			"edit_scope",
			"start_new_attempt",
		},
		[]string{"start_new_attempt"},
	)
	clock.EnableLive()
	return PreparedMissionDecisions{
		Authorizations: []PreparedAuthorizationDecision{
			authDeny,
			authAllow,
		},
		Reviews: []PreparedReviewDecision{{
			Sheet:        reviewSheet,
			Authority:    workAuthority,
			AcceptResult: &reviewInput,
			Refresh:      refresh,
		}},
		Recoveries: []PreparedRecoveryDecision{{
			Sheet:           recoverySheet,
			Attempt:         recoveryReady.attempt,
			Authority:       workAuthority,
			StartNewAttempt: &recoveryInput,
			Refresh:         refresh,
		}},
	}, nil
}

func commitControlledMissionRuntime(
	ctx context.Context,
	store *journal.Store,
	runtimeID, fixtureID string,
	now time.Time,
) error {
	instance, err := loomruntime.NewRuntimeInstance(
		loomruntime.RuntimeInstance{
			ID:                   runtimeID,
			DeviceID:             "device." + fixtureID,
			AdapterType:          "fixture",
			DisplayName:          "Controlled Decision Fixture",
			ExecutableVersion:    "1.0.0",
			Status:               loomruntime.RuntimeOnline,
			ObservedCapabilities: []string{"models"},
			Capacity:             8,
		},
	)
	if err != nil {
		return err
	}
	snapshot, err := loomruntime.DiscoverRuntime(
		ctx,
		[]loomruntime.RuntimeProbe{controlledMissionRuntimeProbe{
			fixtureID: fixtureID,
			instance:  instance,
		}},
	)
	if err != nil {
		return err
	}
	_, err = state.CommitRuntimeDiscoverySnapshot(
		ctx,
		store,
		snapshot,
		state.RuntimeDiscoveryCommitInput{
			DiscoveryID: "discovery." + fixtureID,
			EmittedAt:   now,
			Events: []state.RuntimeDiscoveryEventInput{{
				RuntimeInstanceID: runtimeID,
				EventID:           "event.runtime." + fixtureID,
				IdempotencyKey:    "runtime." + fixtureID,
				Seq:               1,
			}},
		},
	)
	return err
}

func buildControlledAuthorizationMission(
	ctx context.Context,
	store *journal.Store,
	workAuthority *work.Authority,
	readModel *projection.Projection,
	clock *controlledMissionDecisionFixtureClock,
	runtimeID, fixtureID, teamID string,
) (PreparedAuthorizationDecision, error) {
	if _, err := dispatchControlledMission(
		ctx,
		workAuthority,
		readModel,
		clock.Now(),
		runtimeID,
		teamID,
		1,
		verification.AcceptanceRiskLow,
	); err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	workItemID := "approval-work-" + teamID
	runID := "approval-run-" + teamID
	agentID := "agent-" + teamID
	correlationID := appVerifierUUID(
		"mission-approval-correlation",
		fixtureID,
		teamID,
	)
	if _, _, err := workAuthority.CreateAndAssign(
		ctx,
		work.WorkItemAssignmentInput{
			WorkItemID:      workItemID,
			Title:           "Authorize " + teamID,
			RunID:           runID,
			AgentInstanceID: agentID,
			CorrelationID:   correlationID,
		},
	); err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	authorizer := &controlledMissionDecisionAuthorizer{
		clock: clock,
		seed:  fixtureID,
	}
	ruleAuthority, err := rules.NewAuthority(
		store,
		authorizer,
		clock.Now,
	)
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	scope, err := rules.NewScope("work_item", workItemID)
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	condition, err := rules.NewCondition("start_run", "high")
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	effect, err := rules.NewEffect(
		"require_approval",
		"",
		[]string{"local-owner"},
		2*time.Hour,
		"reject",
	)
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	rule, err := rules.NewRule("controlled-start", condition, effect)
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	ruleSet, err := rules.NewRuleSet(scope, 1, []rules.Rule{rule})
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	activation, err := rules.NewRuleSetActivationRequest(
		ruleSet,
		[]byte("controlled native decision fixture"),
	)
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	if _, err := ruleAuthority.ActivateRuleSet(
		ctx,
		activation,
		correlationID,
	); err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	action, err := rules.NewActionContext(rules.ActionContextInput{
		ProjectID:       "project.phase2a",
		TeamInstanceID:  teamID,
		WorkPackageID:   "package." + fixtureID,
		WorkItemID:      workItemID,
		RunID:           runID,
		AgentInstanceID: agentID,
		LogicalNodeID:   "main",
		AttemptNumber:   1,
		Action:          "start_run",
		Risk:            "high",
		ClaimGeneration: 0,
		ContractDigest: missionDecisionDigestFields(
			"controlled-mission-contract",
			fixtureID,
			teamID,
		),
	})
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	decision, err := rules.Evaluate([]rules.RuleSet{ruleSet}, action)
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	pending, err := ruleAuthority.RequestApproval(
		ctx,
		rules.ApprovalRequestInput{
			Context: action,
			ContinuationDigest: missionDecisionDigestFields(
				"controlled-mission-continuation",
				fixtureID,
				teamID,
			),
			Decision:      decision,
			RequestedAt:   clock.Now(),
			CorrelationID: correlationID,
		},
	)
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	presentation := []byte("controlled native owner decision")
	deny, err := rules.NewApprovalDecisionRequest(
		pending.ID(),
		pending.Digest(),
		"rejected",
		presentation,
	)
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	allow, err := rules.NewApprovalDecisionRequest(
		pending.ID(),
		pending.Digest(),
		"approved",
		presentation,
	)
	if err != nil {
		return PreparedAuthorizationDecision{}, err
	}
	return PreparedAuthorizationDecision{
		Sheet: MissionDecisionSheet{
			SchemaVersion:    1,
			Kind:             "authorization",
			MissionID:        "mission/" + teamID,
			TeamInstanceID:   teamID,
			DecisionID:       pending.ID(),
			DecisionDigest:   pending.Digest(),
			Title:            "Authorization required",
			Summary:          "Run the controlled local verification command.",
			Requester:        "Main Agent",
			Target:           "controlled local workspace",
			CommandType:      "local process",
			NetworkAccess:    "none",
			CredentialAccess: "none",
			PermissionScope:  "this Mission",
			AttemptScope:     "Attempt 1",
			ExpectedEvidence: "authoritative approval fact",
			TechnicalDetails: []string{},
			Actions: []string{
				"not_now",
				"deny",
				"edit_scope",
				"allow_once",
			},
			PreparedActions: []string{"deny", "allow_once"},
			Prepared:        true,
			LogicalNodeID:   "main",
			AttemptNumber:   1,
			ClaimGeneration: 0,
		},
		Pending:   pending,
		Context:   action,
		Decision:  decision,
		Authority: ruleAuthority,
		Deny:      deny,
		AllowOnce: allow,
	}, nil
}

func buildControlledTerminalMission(
	ctx context.Context,
	workAuthority *work.Authority,
	readModel *projection.Projection,
	artifacts *evidence.Store,
	now time.Time,
	runtimeID, teamID string,
	maxAttempts int,
	risk verification.AcceptanceRisk,
	terminalStatus string,
) (controlledMissionScenario, error) {
	scenario, err := dispatchControlledMission(
		ctx,
		workAuthority,
		readModel,
		now,
		runtimeID,
		teamID,
		maxAttempts,
		risk,
	)
	if err != nil {
		return controlledMissionScenario{}, err
	}
	generation := work.RunGenerationInput{
		WorkItemID:        scenario.attempt.WorkItemID(),
		RunID:             scenario.attempt.RunID(),
		ClaimID:           scenario.attempt.ClaimID(),
		ClaimGeneration:   scenario.attempt.ClaimGeneration(),
		RuntimeInstanceID: scenario.attempt.RuntimeInstanceID(),
		AgentInstanceID:   scenario.attempt.AgentInstanceID(),
		CorrelationID: appVerifierUUID(
			"mission-run-correlation",
			teamID,
		),
	}
	if _, _, err := workAuthority.Start(ctx, generation); err != nil {
		return controlledMissionScenario{}, err
	}
	reason := ""
	if terminalStatus == "failed" {
		reason = "controlled_failure"
	}
	if _, _, err := workAuthority.CommitTerminal(
		ctx,
		work.RunTerminalInput{
			RunGenerationInput: generation,
			Status:             terminalStatus,
			Reason:             reason,
		},
	); err != nil {
		return controlledMissionScenario{}, err
	}
	evidenceID := "evidence-" + teamID
	capture := evidence.AttemptCaptureInput{
		EvidenceID:        evidenceID,
		TeamInstanceID:    scenario.plan.TeamInstanceID(),
		PlanDigest:        scenario.plan.Digest(),
		LogicalNodeID:     "main",
		AttemptNumber:     scenario.attempt.AttemptNumber(),
		WorkItemID:        scenario.attempt.WorkItemID(),
		RunID:             scenario.attempt.RunID(),
		ClaimID:           scenario.attempt.ClaimID(),
		ClaimGeneration:   scenario.attempt.ClaimGeneration(),
		RuntimeInstanceID: scenario.attempt.RuntimeInstanceID(),
		AgentInstanceID:   scenario.attempt.AgentInstanceID(),
	}
	if err := artifacts.BeginAttemptCapture(ctx, capture); err != nil {
		return controlledMissionScenario{}, err
	}
	lines, err := controlledMissionEvidenceLines(
		capture,
		now,
		terminalStatus,
		reason,
	)
	if err != nil {
		return controlledMissionScenario{}, err
	}
	for _, line := range lines {
		if err := artifacts.AppendAttemptFrame(
			ctx,
			evidenceID,
			line,
		); err != nil {
			return controlledMissionScenario{}, err
		}
	}
	receipt, err := artifacts.FinalizeAttemptCapture(
		ctx,
		evidenceID,
		evidence.AttemptTerminal{Status: terminalStatus, Reason: reason},
	)
	if err != nil {
		return controlledMissionScenario{}, err
	}
	summary := receipt.OutputSummary()
	observation, err := verification.NewOutputObservation(
		receipt.EvidenceID(),
		receipt.Digest(),
		summary.Digest(),
		summary.AuthorizedFrameCount(),
		summary.OutputFrameCount(),
		summary.OutputPayloadBytes(),
		summary.ResultObserved(),
		summary.TerminalStatus(),
	)
	if err != nil {
		return controlledMissionScenario{}, err
	}
	classification, err := verification.Classify(
		scenario.semantics.OutputContract,
		observation,
	)
	if err != nil {
		return controlledMissionScenario{}, err
	}
	if _, err := workAuthority.CommitTeamAttemptEvidence(
		ctx,
		work.TeamAttemptEvidenceInput{
			TeamInstanceID:    scenario.plan.TeamInstanceID(),
			PlanDigest:        scenario.plan.Digest(),
			LogicalNodeID:     "main",
			AttemptNumber:     scenario.attempt.AttemptNumber(),
			WorkItemID:        scenario.attempt.WorkItemID(),
			RunID:             scenario.attempt.RunID(),
			ClaimID:           scenario.attempt.ClaimID(),
			ClaimGeneration:   scenario.attempt.ClaimGeneration(),
			RuntimeInstanceID: scenario.attempt.RuntimeInstanceID(),
			AgentInstanceID:   scenario.attempt.AgentInstanceID(),
			Receipt:           receipt,
			Classification:    classification,
			CorrelationID:     generation.CorrelationID,
		},
	); err != nil {
		return controlledMissionScenario{}, err
	}
	record, err := workAuthority.TeamExecution(
		ctx,
		scenario.plan.TeamInstanceID(),
	)
	if err != nil {
		return controlledMissionScenario{}, err
	}
	for _, node := range record.Nodes() {
		if node.LogicalNodeID() == "main" &&
			len(node.Attempts()) > 0 {
			scenario.attempt = node.Attempts()[len(node.Attempts())-1]
			break
		}
	}
	scenario.receipt = receipt
	scenario.classification = classification
	return scenario, nil
}

func dispatchControlledMission(
	ctx context.Context,
	workAuthority *work.Authority,
	readModel *projection.Projection,
	now time.Time,
	runtimeID, teamID string,
	maxAttempts int,
	risk verification.AcceptanceRisk,
) (controlledMissionScenario, error) {
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: teamID,
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID:     "main",
			Title:             "Controlled mission " + teamID,
			AgentInstanceID:   "agent-" + teamID,
			RuntimeInstanceID: runtimeID,
			Role:              teams.ExecutionRoleMain,
			MaxAttempts:       maxAttempts,
		}},
	})
	if err != nil {
		return controlledMissionScenario{}, err
	}
	output, err := verification.NewOutputContract(
		1,
		verification.EmptyOutputInvalid,
	)
	if err != nil {
		return controlledMissionScenario{}, err
	}
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:          1,
		AttemptCredits:   maxAttempts - 1,
		ExhaustionAction: rules.ExhaustionBlocked,
		RetryInvalid:     true,
	})
	if err != nil {
		return controlledMissionScenario{}, err
	}
	acceptance, err := verification.NewAcceptanceContract(
		1,
		[]string{"controlled output is accepted"},
		risk,
	)
	if err != nil {
		return controlledMissionScenario{}, err
	}
	semantics := TeamNodeSemantics{
		LogicalNodeID:       "main",
		OutputContract:      output,
		RecoveryPolicy:      policy,
		AcceptanceContract:  acceptance,
		PrimaryWorkflowPath: "controlled",
	}
	if acceptance.IndependentVerifierRequired() {
		semantics.VerifierAgentInstanceID = "agent-verifier-" + teamID
		semantics.VerifierRuntimeInstanceID = runtimeID
		semantics.VerifierWorkflowPath = "controlled-verifier"
	}
	request := TeamExecutionRequest{
		Plan:              plan,
		Semantics:         []TeamNodeSemantics{semantics},
		AuthoritativeTime: now,
		CorrelationID: appVerifierUUID(
			"mission-dispatch-correlation",
			teamID,
		),
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return controlledMissionScenario{}, err
	}
	view := readModel.GlobalReadView()
	selections := []work.TeamAttemptSelection{{
		LogicalNodeID: "main",
		AttemptNumber: 1,
	}}
	dispatched, err := workAuthority.DispatchTeamReadySet(
		ctx,
		work.TeamDispatchInput{
			Plan:                 plan,
			ReadyAttempts:        selections,
			SemanticBindings:     appSemanticBindings(request),
			ViewVersion:          view.Version(),
			ExpectedHeads:        appDispatchHeads(view, plan, selections, plan.Nodes()),
			AuthoritativeTime:    now,
			PrepareLeaseDuration: 5 * time.Minute,
			CorrelationID:        request.CorrelationID,
		},
	)
	if err != nil {
		return controlledMissionScenario{}, err
	}
	nodes := dispatched.Nodes()
	if len(nodes) != 1 {
		return controlledMissionScenario{}, ErrInvalidMissionDecision
	}
	return controlledMissionScenario{
		plan:      plan,
		semantics: semantics,
		attempt:   nodes[0].Attempt(),
	}, nil
}

func controlledMissionEvidenceLines(
	input evidence.AttemptCaptureInput,
	now time.Time,
	status, reason string,
) ([][]byte, error) {
	specs := []struct {
		sequence int64
		kind     bridgev1.MessageType
		payload  []byte
	}{
		{
			2,
			bridgev1.MessageAck,
			[]byte(`{"message_id":"11111111-1111-4111-8111-111111111111"}`),
		},
		{
			3,
			bridgev1.MessageEvent,
			[]byte(`{"delta":"authorized controlled output"}`),
		},
		{
			4,
			bridgev1.MessageResult,
			[]byte(fmt.Sprintf(
				`{"status":%q,"reason":%q}`,
				status,
				reason,
			)),
		},
	}
	lines := make([][]byte, 0, len(specs))
	for _, spec := range specs {
		frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
			MessageID: appVerifierUUID(
				"mission-evidence-frame",
				input.TeamInstanceID,
				fmt.Sprint(spec.sequence),
			),
			CorrelationID: appVerifierUUID(
				"mission-evidence-correlation",
				input.TeamInstanceID,
			),
			WorkItemID:            input.WorkItemID,
			RunID:                 input.RunID,
			ClaimGeneration:       input.ClaimGeneration,
			RuntimeInstanceID:     input.RuntimeInstanceID,
			SenderAgentInstanceID: input.AgentInstanceID,
			Sequence:              spec.sequence,
			Type:                  spec.kind,
			EmittedAt:             now,
			Payload:               spec.payload,
		})
		if err != nil {
			return nil, err
		}
		line, err := bridgev1.EncodeLine(frame)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func controlledReviewInput(
	scenario controlledMissionScenario,
	now time.Time,
) (work.TeamNodeAcceptanceInput, error) {
	summary := scenario.receipt.OutputSummary()
	result, err := verification.VerifyDeterministic(
		scenario.semantics.AcceptanceContract,
		verification.DeterministicVerificationInput{
			TeamInstanceID:             scenario.plan.TeamInstanceID(),
			PlanDigest:                 scenario.plan.Digest(),
			LogicalNodeID:              "main",
			AttemptNumber:              scenario.attempt.AttemptNumber(),
			WorkItemID:                 scenario.attempt.WorkItemID(),
			RunID:                      scenario.attempt.RunID(),
			ClaimID:                    scenario.attempt.ClaimID(),
			ClaimGeneration:            scenario.attempt.ClaimGeneration(),
			SourceEvidenceID:           scenario.receipt.EvidenceID(),
			SourceEvidenceDigest:       scenario.receipt.Digest(),
			OutputSummaryDigest:        summary.Digest(),
			OutputContractVersion:      scenario.semantics.OutputContract.Version(),
			OutputContractDigest:       scenario.semantics.OutputContract.Digest(),
			OutputClassification:       scenario.classification.Kind(),
			OutputClassificationDigest: scenario.classification.Digest(),
			AcceptanceContractDigest: scenario.semantics.
				AcceptanceContract.Digest(),
			TerminalStatus: summary.TerminalStatus(),
		},
	)
	if err != nil {
		return work.TeamNodeAcceptanceInput{}, err
	}
	decision, err := verification.DecideAcceptance(
		verification.AcceptanceDecisionInput{
			Contract:            scenario.semantics.AcceptanceContract,
			DeterministicResult: result,
			DecisionTime:        now,
		},
	)
	if err != nil {
		return work.TeamNodeAcceptanceInput{}, err
	}
	return work.TeamNodeAcceptanceInput{
		TeamInstanceID:      scenario.plan.TeamInstanceID(),
		PlanDigest:          scenario.plan.Digest(),
		LogicalNodeID:       "main",
		AttemptNumber:       scenario.attempt.AttemptNumber(),
		SourceReceipt:       scenario.receipt,
		AcceptanceContract:  scenario.semantics.AcceptanceContract,
		DeterministicResult: result,
		Decision:            decision,
		RecoveryPolicy:      scenario.semantics.RecoveryPolicy,
		MaxAttempts:         scenario.plan.Nodes()[0].MaxAttempts(),
		CreditsBefore: scenario.semantics.RecoveryPolicy.
			AttemptCredits(),
		CorrelationID: appVerifierUUID(
			"mission-review-correlation",
			scenario.plan.TeamInstanceID(),
		),
	}, nil
}

func controlledRecoveryInput(
	scenario controlledMissionScenario,
	now time.Time,
) (work.TeamRecoveryInput, error) {
	decision, err := rules.DecideRecovery(
		scenario.semantics.RecoveryPolicy,
		rules.RecoveryInput{
			TeamInstanceID:      scenario.plan.TeamInstanceID(),
			PlanDigest:          scenario.plan.Digest(),
			LogicalNodeID:       "main",
			AttemptNumber:       scenario.attempt.AttemptNumber(),
			MaxAttempts:         scenario.plan.Nodes()[0].MaxAttempts(),
			AgentInstanceID:     scenario.attempt.AgentInstanceID(),
			RuntimeInstanceID:   scenario.attempt.RuntimeInstanceID(),
			EvidenceID:          scenario.receipt.EvidenceID(),
			EvidenceDigest:      scenario.receipt.Digest(),
			OutputSummaryDigest: scenario.receipt.OutputSummary().Digest(),
			Classification:      scenario.classification,
			RemainingCredits:    1,
			FallbackConsumed:    false,
			DecisionTime:        now,
			Trigger:             rules.RecoveryTriggerOutput,
		},
	)
	if err != nil {
		return work.TeamRecoveryInput{}, err
	}
	return work.TeamRecoveryInput{
		Decision: decision,
		CorrelationID: appVerifierUUID(
			"mission-recovery-correlation",
			scenario.plan.TeamInstanceID(),
		),
	}, nil
}

func controlledWorkDecisionSheet(
	kind string,
	scenario controlledMissionScenario,
	viewVersion, decisionDigest string,
	actions, preparedActions []string,
) MissionDecisionSheet {
	title := "Review result"
	summary := "Focused and repository verification passed."
	target := "Exact prepared node result"
	commandType := "Deterministic verification: PASS"
	networkAccess := "Independent Reviewer: PASS"
	permissionScope := "No new permission"
	attemptScope := "Attempt 1"
	expectedEvidence := "Accepted Evidence receipt bound to this attempt"
	if kind == "recovery" {
		title = "Recovery decision"
		summary = "Attempt 1 stopped after deterministic verification failed."
		target = "No Team or Provider change"
		commandType = "Fresh attempt with generation fencing"
		networkAccess = "none"
		permissionScope = "No permission or budget increase"
		attemptScope = "Attempt 2 · fresh generation"
		expectedEvidence =
			"Attempt 1 output and Evidence retained by digest"
	}
	return MissionDecisionSheet{
		SchemaVersion:    1,
		Kind:             kind,
		MissionID:        "mission/" + scenario.plan.TeamInstanceID(),
		TeamInstanceID:   scenario.plan.TeamInstanceID(),
		ViewVersion:      viewVersion,
		DecisionID:       kind + "-" + decisionDigest[:32],
		DecisionDigest:   decisionDigest,
		Title:            title,
		Summary:          summary,
		Requester:        "Main Agent",
		Target:           target,
		CommandType:      commandType,
		NetworkAccess:    networkAccess,
		CredentialAccess: "none",
		PermissionScope:  permissionScope,
		AttemptScope:     attemptScope,
		ExpectedEvidence: expectedEvidence,
		TechnicalDetails: []string{},
		Actions:          append([]string(nil), actions...),
		PreparedActions:  append([]string(nil), preparedActions...),
		Prepared:         true,
		LogicalNodeID:    "main",
		AttemptNumber:    scenario.attempt.AttemptNumber(),
		ClaimGeneration:  scenario.attempt.ClaimGeneration(),
	}
}

type preparedMissionDecisionEntry struct {
	sheet   MissionDecisionSheet
	refresh MissionDecisionViewRefresher
	execute func(
		context.Context,
		MissionDecisionCommand,
	) (string, string, error)
	inFlight bool
	consumed bool
}

type PreparedMissionDecisionBackend struct {
	mu      sync.Mutex
	entries map[string]*preparedMissionDecisionEntry
}

func NewPreparedMissionDecisionBackend(
	prepared PreparedMissionDecisions,
) (*PreparedMissionDecisionBackend, error) {
	backend := &PreparedMissionDecisionBackend{
		entries: make(map[string]*preparedMissionDecisionEntry),
	}
	add := func(
		sheet MissionDecisionSheet,
		refresh MissionDecisionViewRefresher,
		execute func(
			context.Context,
			MissionDecisionCommand,
		) (string, string, error),
	) error {
		command := missionDecisionCommandFromSheet(sheet, "read", "read")
		if !validMissionDecisionCommand(command) ||
			!validMissionDecisionSheet(command, sheet) ||
			refresh == nil ||
			execute == nil {
			return ErrInvalidMissionDecision
		}
		if _, duplicate := backend.entries[sheet.DecisionID]; duplicate {
			return ErrInvalidMissionDecision
		}
		backend.entries[sheet.DecisionID] = &preparedMissionDecisionEntry{
			sheet:   cloneMissionDecisionSheet(sheet),
			refresh: refresh,
			execute: execute,
		}
		return nil
	}
	for _, candidate := range prepared.Authorizations {
		if !validPreparedAuthorizationDecision(candidate) {
			return nil, ErrInvalidMissionDecision
		}
		candidate := candidate
		if err := add(candidate.Sheet, candidate.Refresh, func(
			ctx context.Context,
			command MissionDecisionCommand,
		) (string, string, error) {
			return executePreparedAuthorization(ctx, command, candidate)
		}); err != nil {
			return nil, ErrInvalidMissionDecision
		}
	}
	for _, candidate := range prepared.Reviews {
		if !validPreparedReviewDecision(candidate) {
			return nil, ErrInvalidMissionDecision
		}
		candidate := candidate
		if err := add(candidate.Sheet, candidate.Refresh, func(
			ctx context.Context,
			command MissionDecisionCommand,
		) (string, string, error) {
			return executePreparedReview(ctx, command, candidate)
		}); err != nil {
			return nil, ErrInvalidMissionDecision
		}
	}
	for _, candidate := range prepared.Recoveries {
		if !validPreparedRecoveryDecision(candidate) {
			return nil, ErrInvalidMissionDecision
		}
		candidate := candidate
		if err := add(candidate.Sheet, candidate.Refresh, func(
			ctx context.Context,
			command MissionDecisionCommand,
		) (string, string, error) {
			return executePreparedRecovery(ctx, command, candidate)
		}); err != nil {
			return nil, ErrInvalidMissionDecision
		}
	}
	return backend, nil
}

func (backend *PreparedMissionDecisionBackend) ReadMissionDecision(
	ctx context.Context,
	command MissionDecisionCommand,
) (MissionDecisionSheet, error) {
	if backend == nil || ctx == nil || ctx.Err() != nil {
		return MissionDecisionSheet{}, ErrInvalidMissionDecision
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	entry, ok := backend.entries[command.DecisionID]
	if !ok || entry.consumed ||
		!commandMatchesMissionDecisionSheet(command, entry.sheet) {
		return MissionDecisionSheet{}, ErrMissionDecisionConflict
	}
	return cloneMissionDecisionSheet(entry.sheet), nil
}

func (backend *PreparedMissionDecisionBackend) ListMissionDecisionCommands(
	ctx context.Context,
) ([]MissionDecisionCommand, error) {
	if backend == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrInvalidMissionDecision
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	commands := make([]MissionDecisionCommand, 0, len(backend.entries))
	for _, entry := range backend.entries {
		if entry.consumed {
			continue
		}
		commands = append(
			commands,
			missionDecisionCommandFromSheet(entry.sheet, "read", "read"),
		)
	}
	sort.Slice(commands, func(i, j int) bool {
		left := commands[i]
		right := commands[j]
		for _, pair := range [][2]string{
			{left.TeamInstanceID, right.TeamInstanceID},
			{left.LogicalNodeID, right.LogicalNodeID},
			{left.Kind, right.Kind},
			{left.DecisionID, right.DecisionID},
		} {
			if pair[0] != pair[1] {
				return pair[0] < pair[1]
			}
		}
		return left.AttemptNumber < right.AttemptNumber
	})
	return commands, nil
}

func (backend *PreparedMissionDecisionBackend) ExecuteMissionDecision(
	ctx context.Context,
	command MissionDecisionCommand,
) (MissionDecisionResult, error) {
	if backend == nil || ctx == nil || ctx.Err() != nil {
		return MissionDecisionResult{}, ErrInvalidMissionDecision
	}
	backend.mu.Lock()
	entry, ok := backend.entries[command.DecisionID]
	if !ok || entry.consumed ||
		!commandMatchesMissionDecisionSheet(command, entry.sheet) ||
		!containsDecisionAction(entry.sheet.Actions, command.Action) ||
		!containsDecisionAction(entry.sheet.PreparedActions, command.Action) {
		backend.mu.Unlock()
		return MissionDecisionResult{}, ErrMissionDecisionConflict
	}
	if entry.inFlight {
		backend.mu.Unlock()
		return MissionDecisionResult{}, ErrMissionDecisionConflict
	}
	entry.inFlight = true
	refresh := entry.refresh
	execute := entry.execute
	backend.mu.Unlock()

	currentView, err := refresh.RefreshMissionDecisionView(ctx)
	if err != nil || currentView != command.ViewVersion {
		backend.mu.Lock()
		entry.inFlight = false
		backend.mu.Unlock()
		return MissionDecisionResult{}, errors.Join(
			ErrMissionDecisionConflict,
			err,
		)
	}
	status, viewVersion, err := execute(ctx, command)

	backend.mu.Lock()
	entry.inFlight = false
	if err == nil {
		entry.consumed = true
		for _, pending := range backend.entries {
			if pending.consumed {
				continue
			}
			pending.sheet.ViewVersion = viewVersion
		}
	}
	backend.mu.Unlock()
	if err != nil {
		return MissionDecisionResult{}, err
	}
	return MissionDecisionResult{
		SchemaVersion: 1,
		MissionID:     command.MissionID,
		DecisionID:    command.DecisionID,
		Status:        status,
		Authoritative: true,
		ViewVersion:   viewVersion,
	}, nil
}

func validPreparedAuthorizationDecision(
	candidate PreparedAuthorizationDecision,
) bool {
	if candidate.Sheet.Kind != "authorization" ||
		!candidate.Sheet.Prepared ||
		!containsDecisionAction(candidate.Sheet.PreparedActions, "deny") ||
		!containsDecisionAction(candidate.Sheet.PreparedActions, "allow_once") ||
		candidate.Authority == nil ||
		candidate.Refresh == nil ||
		candidate.Pending.Status() != "pending" ||
		candidate.Pending.ID() != candidate.Sheet.DecisionID ||
		candidate.Pending.Digest() != candidate.Sheet.DecisionDigest ||
		candidate.Pending.ID() != missionApprovalRequestID(
			candidate.Context.Digest(),
			candidate.Pending.ContinuationDigest(),
			candidate.Decision.Digest(),
		) ||
		candidate.Decision.Kind() != "require_approval" ||
		candidate.Pending.WorkItemID() != candidate.Context.WorkItemID() ||
		candidate.Pending.RunID() != candidate.Context.RunID() ||
		candidate.Context.TeamInstanceID() !=
			candidate.Sheet.TeamInstanceID ||
		candidate.Context.LogicalNodeID() !=
			candidate.Sheet.LogicalNodeID ||
		candidate.Context.AttemptNumber() !=
			candidate.Sheet.AttemptNumber ||
		candidate.Context.ClaimGeneration() !=
			candidate.Sheet.ClaimGeneration ||
		candidate.Pending.ClaimGeneration() !=
			candidate.Sheet.ClaimGeneration ||
		!validApprovalDecisionBinding(
			candidate.Pending,
			candidate.Deny,
			"rejected",
		) ||
		!validApprovalDecisionBinding(
			candidate.Pending,
			candidate.AllowOnce,
			"approved",
		) {
		return false
	}
	if containsDecisionAction(
		candidate.Sheet.Actions,
		"allow_for_mission",
	) {
		prepared := containsDecisionAction(
			candidate.Sheet.PreparedActions,
			"allow_for_mission",
		)
		valid := validApprovalDecisionBinding(
			candidate.Pending,
			candidate.AllowForMission,
			"approved",
		)
		if prepared != valid {
			return false
		}
	}
	return true
}

func validApprovalDecisionBinding(
	pending rules.ApprovalRequestRecord,
	request rules.ApprovalDecisionRequest,
	decision string,
) bool {
	return request.ApprovalRequestID() == pending.ID() &&
		request.ApprovalRequestDigest() == pending.Digest() &&
		request.Decision() == decision
}

func missionApprovalRequestID(
	actionDigest, continuationDigest, decisionDigest string,
) string {
	inner := missionDecisionDigestFields(
		"loom.approval-request-id.v1",
		actionDigest,
		continuationDigest,
		decisionDigest,
	)
	sum := sha256.Sum256([]byte(inner))
	bytes := sum[:16]
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		bytes[0:4],
		bytes[4:6],
		bytes[6:8],
		bytes[8:10],
		bytes[10:16],
	)
}

func missionDecisionDigestFields(fields ...string) string {
	hash := sha256.New()
	var length [8]byte
	for _, field := range fields {
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(field))
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func validPreparedReviewDecision(candidate PreparedReviewDecision) bool {
	if candidate.Sheet.Kind != "review" ||
		!candidate.Sheet.Prepared ||
		candidate.Authority == nil ||
		candidate.Refresh == nil ||
		candidate.RequestChanges == nil &&
			candidate.AcceptResult == nil {
		return false
	}
	if containsDecisionAction(
		candidate.Sheet.PreparedActions,
		"request_changes",
	) != (candidate.RequestChanges != nil) ||
		containsDecisionAction(
			candidate.Sheet.PreparedActions,
			"accept_result",
		) != (candidate.AcceptResult != nil) {
		return false
	}
	return (candidate.RequestChanges == nil ||
		validReviewAcceptanceBinding(
			candidate.Sheet,
			*candidate.RequestChanges,
			verification.AcceptanceRejected,
		)) &&
		(candidate.AcceptResult == nil ||
			validReviewAcceptanceBinding(
				candidate.Sheet,
				*candidate.AcceptResult,
				verification.AcceptanceAccepted,
			))
}

func validReviewAcceptanceBinding(
	sheet MissionDecisionSheet,
	input work.TeamNodeAcceptanceInput,
	kind verification.AcceptanceDecisionKind,
) bool {
	result := input.DeterministicResult.Input()
	return input.Decision.Valid() &&
		input.DeterministicResult.Valid() &&
		input.Decision.Digest() == sheet.DecisionDigest &&
		input.Decision.DeterministicResultDigest() ==
			input.DeterministicResult.Digest() &&
		input.TeamInstanceID == sheet.TeamInstanceID &&
		input.LogicalNodeID == sheet.LogicalNodeID &&
		input.AttemptNumber == sheet.AttemptNumber &&
		result.TeamInstanceID == input.TeamInstanceID &&
		result.PlanDigest == input.PlanDigest &&
		result.LogicalNodeID == input.LogicalNodeID &&
		result.AttemptNumber == input.AttemptNumber &&
		result.ClaimGeneration == sheet.ClaimGeneration &&
		input.Decision.Kind() == kind
}

func validPreparedRecoveryDecision(candidate PreparedRecoveryDecision) bool {
	if candidate.Sheet.Kind != "recovery" ||
		!candidate.Sheet.Prepared ||
		candidate.Authority == nil ||
		candidate.Refresh == nil {
		return false
	}
	if candidate.StopMission == nil &&
		candidate.StartNewAttempt == nil {
		return false
	}
	if containsDecisionAction(
		candidate.Sheet.PreparedActions,
		"stop_mission",
	) != (candidate.StopMission != nil) ||
		containsDecisionAction(
			candidate.Sheet.PreparedActions,
			"start_new_attempt",
		) != (candidate.StartNewAttempt != nil) {
		return false
	}
	return (candidate.StopMission == nil ||
		validRecoveryBinding(
			candidate.Sheet,
			candidate.Attempt,
			*candidate.StopMission,
			false,
		)) &&
		(candidate.StartNewAttempt == nil ||
			validRecoveryBinding(
				candidate.Sheet,
				candidate.Attempt,
				*candidate.StartNewAttempt,
				true,
			))
}

func validRecoveryBinding(
	sheet MissionDecisionSheet,
	attempt work.TeamAttemptRecord,
	input work.TeamRecoveryInput,
	startNewAttempt bool,
) bool {
	if input.Decision == nil ||
		!input.Decision.Valid() ||
		input.Decision.Digest() != sheet.DecisionDigest ||
		input.Decision.TeamInstanceID() != sheet.TeamInstanceID ||
		input.Decision.LogicalNodeID() != sheet.LogicalNodeID ||
		input.Decision.AttemptNumber() != sheet.AttemptNumber ||
		recoveryAttemptMismatch(sheet, attempt, input) {
		return false
	}
	action := input.Decision.ActionValue()
	if startNewAttempt {
		return action == string(rules.RecoveryRetry) ||
			action == string(rules.RecoveryFallback)
	}
	return action == string(rules.RecoveryBlocked) ||
		action == string(rules.RecoveryHumanRequired)
}

func recoveryAttemptMismatch(
	sheet MissionDecisionSheet,
	attempt work.TeamAttemptRecord,
	input work.TeamRecoveryInput,
) bool {
	decision := input.Decision
	return decision == nil ||
		attempt.AttemptNumber() != sheet.AttemptNumber ||
		attempt.ClaimGeneration() != sheet.ClaimGeneration ||
		attempt.AgentInstanceID() != decision.AgentInstanceID() ||
		attempt.RuntimeInstanceID() != decision.RuntimeInstanceID() ||
		attempt.EvidenceID() != decision.EvidenceID() ||
		attempt.EvidenceDigest() != decision.EvidenceDigest() ||
		attempt.OutputSummaryDigest() != decision.OutputSummaryDigest() ||
		string(attempt.OutputClassification()) !=
			decision.ClassificationValue() ||
		attempt.OutputClassificationDigest() !=
			decision.ClassificationDigest()
}

func executePreparedAuthorization(
	ctx context.Context,
	command MissionDecisionCommand,
	candidate PreparedAuthorizationDecision,
) (string, string, error) {
	var request rules.ApprovalDecisionRequest
	switch command.Action {
	case "deny":
		request = candidate.Deny
	case "allow_once":
		request = candidate.AllowOnce
	case "allow_for_mission":
		request = candidate.AllowForMission
	default:
		return "", "", ErrMissionDecisionConflict
	}
	record, err := candidate.Authority.DecideApproval(
		ctx,
		request,
		command.CorrelationID,
	)
	if err != nil {
		return "", "", errors.Join(ErrMissionDecisionConflict, err)
	}
	status := record.Status()
	if status == "rejected" {
		status = "denied"
	}
	if status != "approved" && status != "denied" {
		return "", "", ErrMissionDecisionConflict
	}
	viewVersion, err := refreshMissionDecisionView(
		ctx,
		command.ViewVersion,
		candidate.Refresh,
	)
	return status, viewVersion, err
}

func executePreparedReview(
	ctx context.Context,
	command MissionDecisionCommand,
	candidate PreparedReviewDecision,
) (string, string, error) {
	var input work.TeamNodeAcceptanceInput
	status := ""
	switch command.Action {
	case "request_changes":
		if candidate.RequestChanges == nil {
			return "", "", ErrMissionDecisionConflict
		}
		input = *candidate.RequestChanges
		status = "changes_requested"
	case "accept_result":
		if candidate.AcceptResult == nil {
			return "", "", ErrMissionDecisionConflict
		}
		input = *candidate.AcceptResult
		status = "accepted"
	default:
		return "", "", ErrMissionDecisionConflict
	}
	if _, err := candidate.Authority.CommitTeamNodeAcceptance(
		ctx,
		input,
	); err != nil {
		return "", "", errors.Join(ErrMissionDecisionConflict, err)
	}
	viewVersion, err := refreshMissionDecisionView(
		ctx,
		command.ViewVersion,
		candidate.Refresh,
	)
	return status, viewVersion, err
}

func executePreparedRecovery(
	ctx context.Context,
	command MissionDecisionCommand,
	candidate PreparedRecoveryDecision,
) (string, string, error) {
	var input work.TeamRecoveryInput
	status := ""
	switch command.Action {
	case "stop_mission":
		if candidate.StopMission == nil {
			return "", "", ErrMissionDecisionConflict
		}
		input = *candidate.StopMission
		status = "stopped"
	case "start_new_attempt":
		if candidate.StartNewAttempt == nil {
			return "", "", ErrMissionDecisionConflict
		}
		input = *candidate.StartNewAttempt
		status = "recovery_started"
	default:
		return "", "", ErrMissionDecisionConflict
	}
	if _, err := candidate.Authority.ScheduleTeamNodeRecovery(
		ctx,
		input,
	); err != nil {
		return "", "", errors.Join(ErrMissionDecisionConflict, err)
	}
	viewVersion, err := refreshMissionDecisionView(
		ctx,
		command.ViewVersion,
		candidate.Refresh,
	)
	return status, viewVersion, err
}

func refreshMissionDecisionView(
	ctx context.Context,
	previous string,
	refresh MissionDecisionViewRefresher,
) (string, error) {
	next, err := refresh.RefreshMissionDecisionView(ctx)
	if err != nil {
		return "", errors.Join(ErrMissionDecisionConflict, err)
	}
	if !validDecisionDigest(next) || next == previous {
		return "", ErrMissionDecisionConflict
	}
	return next, nil
}

type MissionDecisionConfig struct {
	Backend *PreparedMissionDecisionBackend
}

type LocalProductDecisionService struct {
	backend *PreparedMissionDecisionBackend
}

func NewLocalProductDecisionService(
	config MissionDecisionConfig,
) (*LocalProductDecisionService, error) {
	if config.Backend == nil {
		return nil, ErrInvalidMissionDecision
	}
	return &LocalProductDecisionService{backend: config.Backend}, nil
}

func (service *LocalProductDecisionService) DecideMission(
	ctx context.Context,
	command MissionDecisionCommand,
) (MissionDecisionResult, error) {
	if service == nil || service.backend == nil ||
		ctx == nil || ctx.Err() != nil ||
		!validMissionDecisionCommand(command) ||
		command.Operation == "read" {
		return MissionDecisionResult{}, ErrInvalidMissionDecision
	}
	if command.Operation == "defer" {
		return MissionDecisionResult{
			SchemaVersion: 1,
			MissionID:     command.MissionID,
			DecisionID:    command.DecisionID,
			Status:        "pending",
			Authoritative: false,
			ViewVersion:   command.ViewVersion,
		}, nil
	}
	result, err := service.backend.ExecuteMissionDecision(ctx, command)
	if err != nil {
		return MissionDecisionResult{}, err
	}
	if !validMissionDecisionResult(command, result) {
		return MissionDecisionResult{}, ErrInvalidMissionDecision
	}
	return result, nil
}

func (service *LocalProductDecisionService) ReadMissionDecision(
	ctx context.Context,
	command MissionDecisionCommand,
) (MissionDecisionSheet, error) {
	if service == nil || service.backend == nil ||
		ctx == nil || ctx.Err() != nil ||
		!validMissionDecisionCommand(command) ||
		command.Operation != "read" {
		return MissionDecisionSheet{}, ErrInvalidMissionDecision
	}
	sheet, err := service.backend.ReadMissionDecision(ctx, command)
	if err != nil {
		return MissionDecisionSheet{}, err
	}
	if !validMissionDecisionSheet(command, sheet) {
		return MissionDecisionSheet{}, ErrInvalidMissionDecision
	}
	return cloneMissionDecisionSheet(sheet), nil
}

func validMissionDecisionCommand(command MissionDecisionCommand) bool {
	if command.SchemaVersion != 1 ||
		!validDecisionKind(command.Kind) ||
		command.MissionID != "mission/"+command.TeamInstanceID ||
		!validDecisionID(command.TeamInstanceID) ||
		!validDecisionID(command.LogicalNodeID) ||
		!validDecisionID(command.DecisionID) ||
		!validDecisionID(command.CorrelationID) ||
		!validDecisionDigest(command.ViewVersion) ||
		!validDecisionDigest(command.DecisionDigest) ||
		command.AttemptNumber < 1 {
		return false
	}
	if command.Kind == "authorization" {
		if command.ClaimGeneration < 0 {
			return false
		}
	} else if command.ClaimGeneration < 1 {
		return false
	}
	switch command.Operation {
	case "read":
		return command.Action == "read"
	case "defer":
		return command.Action == "not_now" || command.Action == "edit_scope"
	case "submit":
		return validDecisionAction(command.Kind, command.Action) &&
			command.Action != "not_now" &&
			command.Action != "edit_scope"
	default:
		return false
	}
}

func validMissionDecisionSheet(
	command MissionDecisionCommand,
	sheet MissionDecisionSheet,
) bool {
	if sheet.SchemaVersion != 1 ||
		sheet.Kind != command.Kind ||
		sheet.MissionID != command.MissionID ||
		sheet.TeamInstanceID != command.TeamInstanceID ||
		sheet.ViewVersion != command.ViewVersion ||
		sheet.DecisionID != command.DecisionID ||
		sheet.DecisionDigest != command.DecisionDigest ||
		sheet.LogicalNodeID != command.LogicalNodeID ||
		sheet.AttemptNumber != command.AttemptNumber ||
		sheet.ClaimGeneration != command.ClaimGeneration ||
		!validDecisionSheetText(sheet.Title) ||
		!validDecisionSheetText(sheet.Summary) ||
		!validDecisionSheetText(sheet.Requester) ||
		!validDecisionSheetText(sheet.Target) ||
		!validDecisionSheetText(sheet.CommandType) ||
		!validDecisionSheetText(sheet.NetworkAccess) ||
		!validDecisionSheetText(sheet.CredentialAccess) ||
		!validDecisionSheetText(sheet.PermissionScope) ||
		!validDecisionSheetText(sheet.AttemptScope) ||
		!validDecisionSheetText(sheet.ExpectedEvidence) ||
		len(sheet.TechnicalDetails) > 32 ||
		len(sheet.Actions) == 0 ||
		len(sheet.Actions) > 5 ||
		len(sheet.PreparedActions) > len(sheet.Actions) ||
		sheet.Prepared != (len(sheet.PreparedActions) > 0) {
		return false
	}
	seen := make(map[string]struct{}, len(sheet.Actions))
	for _, action := range sheet.Actions {
		if !validDecisionAction(sheet.Kind, action) {
			return false
		}
		if _, duplicate := seen[action]; duplicate {
			return false
		}
		seen[action] = struct{}{}
	}
	for _, required := range requiredDecisionActions(sheet.Kind) {
		if _, ok := seen[required]; !ok {
			return false
		}
	}
	preparedSeen := make(map[string]struct{}, len(sheet.PreparedActions))
	for _, action := range sheet.PreparedActions {
		if action == "not_now" || action == "edit_scope" {
			return false
		}
		if _, ok := seen[action]; !ok {
			return false
		}
		if _, duplicate := preparedSeen[action]; duplicate {
			return false
		}
		preparedSeen[action] = struct{}{}
	}
	for _, detail := range sheet.TechnicalDetails {
		if !validDecisionSheetText(detail) {
			return false
		}
	}
	return true
}

func requiredDecisionActions(kind string) []string {
	switch kind {
	case "authorization":
		return []string{"not_now", "deny", "edit_scope", "allow_once"}
	case "review":
		return []string{"not_now", "request_changes", "accept_result"}
	case "recovery":
		return []string{
			"not_now",
			"stop_mission",
			"edit_scope",
			"start_new_attempt",
		}
	default:
		return nil
	}
}

func missionDecisionCommandFromSheet(
	sheet MissionDecisionSheet,
	operation, action string,
) MissionDecisionCommand {
	return MissionDecisionCommand{
		SchemaVersion:   1,
		Operation:       operation,
		Kind:            sheet.Kind,
		Action:          action,
		MissionID:       sheet.MissionID,
		TeamInstanceID:  sheet.TeamInstanceID,
		ViewVersion:     sheet.ViewVersion,
		DecisionID:      sheet.DecisionID,
		DecisionDigest:  sheet.DecisionDigest,
		LogicalNodeID:   sheet.LogicalNodeID,
		AttemptNumber:   sheet.AttemptNumber,
		ClaimGeneration: sheet.ClaimGeneration,
		CorrelationID:   "00000000-0000-4000-8000-000000000000",
	}
}

func commandMatchesMissionDecisionSheet(
	command MissionDecisionCommand,
	sheet MissionDecisionSheet,
) bool {
	return command.Kind == sheet.Kind &&
		command.MissionID == sheet.MissionID &&
		command.TeamInstanceID == sheet.TeamInstanceID &&
		command.ViewVersion == sheet.ViewVersion &&
		command.DecisionID == sheet.DecisionID &&
		command.DecisionDigest == sheet.DecisionDigest &&
		command.LogicalNodeID == sheet.LogicalNodeID &&
		command.AttemptNumber == sheet.AttemptNumber &&
		command.ClaimGeneration == sheet.ClaimGeneration
}

func containsDecisionAction(actions []string, target string) bool {
	for _, action := range actions {
		if action == target {
			return true
		}
	}
	return false
}

func cloneMissionDecisionSheet(sheet MissionDecisionSheet) MissionDecisionSheet {
	sheet.TechnicalDetails = append(
		make([]string, 0, len(sheet.TechnicalDetails)),
		sheet.TechnicalDetails...,
	)
	sheet.Actions = append(
		make([]string, 0, len(sheet.Actions)),
		sheet.Actions...,
	)
	sheet.PreparedActions = append(
		make([]string, 0, len(sheet.PreparedActions)),
		sheet.PreparedActions...,
	)
	return sheet
}

func validDecisionSheetText(value string) bool {
	return value != "" &&
		len(value) <= 4_096 &&
		strings.TrimSpace(value) == value
}

func validMissionDecisionResult(
	command MissionDecisionCommand,
	result MissionDecisionResult,
) bool {
	if result.SchemaVersion != 1 ||
		result.MissionID != command.MissionID ||
		result.DecisionID != command.DecisionID ||
		!validDecisionDigest(result.ViewVersion) ||
		result.ViewVersion == command.ViewVersion {
		return false
	}
	switch result.Status {
	case "approved", "denied", "accepted", "changes_requested",
		"recovery_started", "stopped":
		return result.Authoritative
	default:
		return false
	}
}

func validDecisionKind(kind string) bool {
	switch kind {
	case "authorization", "review", "recovery":
		return true
	default:
		return false
	}
}

func validDecisionAction(kind, action string) bool {
	if action == "not_now" || action == "edit_scope" {
		return true
	}
	switch kind {
	case "authorization":
		return action == "deny" ||
			action == "allow_once" ||
			action == "allow_for_mission"
	case "review":
		return action == "request_changes" ||
			action == "accept_result"
	case "recovery":
		return action == "stop_mission" ||
			action == "start_new_attempt"
	default:
		return false
	}
}

func validDecisionDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !unicode.IsDigit(character) &&
			(character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validDecisionID(value string) bool {
	if value == "" || len(value) > 128 ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) ||
			character == '.' || character == '_' || character == ':' ||
			character == '-' {
			continue
		}
		return false
	}
	return true
}
