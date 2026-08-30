package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/roundtable"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const productRoundtableOutputLimit = 30 << 10

const (
	productRoundtableAgentInputPending     = "pending"
	productRoundtableAgentInputAvailable   = "available"
	productRoundtableAgentInputUnavailable = "unavailable"
)

type productRoundtableExecutionRunner interface {
	RunRound(
		context.Context,
		app.TeamExecutionRequest,
	) (map[string]productRoundtableSeatOutcome, error)
}

type productRoundtableAgentInputCapabilityResolver interface {
	Resolve(productActiveAttemptQuery) (productActiveAttempt, error)
}

type productRoundtableSeatOutcome struct {
	status string
	reason string
}

type productRoundtableMissionRunner struct {
	delegate *productMissionExecutionRunner
}

func (runner productRoundtableMissionRunner) RunRound(
	ctx context.Context,
	request app.TeamExecutionRequest,
) (map[string]productRoundtableSeatOutcome, error) {
	if runner.delegate == nil {
		return nil, app.ErrInvalidMissionExecution
	}
	result, err := runner.delegate.Run(ctx, request)
	records := make(map[string]productRoundtableSeatOutcome)
	for _, node := range result.Team().Nodes() {
		attempts := node.Attempts()
		if len(attempts) == 0 {
			continue
		}
		attempt := attempts[len(attempts)-1]
		records[node.LogicalNodeID()] = productRoundtableSeatOutcome{
			status: node.Status(), reason: attempt.TerminalReason(),
		}
	}
	return records, err
}

type productRoundtableExecution struct {
	authority       *roundtable.Authority
	runner          productRoundtableExecutionRunner
	contextCapsules app.MissionContextCapsuleStore
	payloads        attemptpayload.Store
	activeAttempts  productRoundtableAgentInputCapabilityResolver
	missionContext  productRoundtableMissionContextSource
	sourcePath      string
	now             func() time.Time

	mu         sync.RWMutex
	deliveries map[string]map[string]roundtable.SeatDelivery
	active     map[string]productRoundtableActiveExecution
	closed     bool
	wait       sync.WaitGroup
}

type productRoundtableActiveExecution struct {
	cancel context.CancelFunc
}

func newProductRoundtableExecution(
	authority *roundtable.Authority,
	runner productRoundtableExecutionRunner,
	contextCapsules app.MissionContextCapsuleStore,
	payloads attemptpayload.Store,
	activeAttempts productRoundtableAgentInputCapabilityResolver,
	missionContext productRoundtableMissionContextSource,
	sourcePath string,
	now func() time.Time,
) (*productRoundtableExecution, error) {
	if authority == nil || runner == nil || contextCapsules == nil || payloads == nil ||
		missionContext == nil || sourcePath == "" || now == nil {
		return nil, app.ErrInvalidMissionExecution
	}
	return &productRoundtableExecution{
		authority: authority, runner: runner, contextCapsules: contextCapsules,
		payloads: payloads, activeAttempts: activeAttempts, sourcePath: sourcePath, now: now,
		missionContext: missionContext,
		deliveries:     make(map[string]map[string]roundtable.SeatDelivery),
		active:         make(map[string]productRoundtableActiveExecution),
	}, nil
}

func (execution *productRoundtableExecution) StartRound(
	ctx context.Context,
	view roundtable.View,
	prompt string,
	correlationID string,
) (roundtable.View, error) {
	if execution == nil || ctx == nil || view.Session.Context == nil ||
		len(view.Rounds) == 0 || strings.TrimSpace(prompt) == "" {
		return roundtable.View{}, roundtable.ErrInvalidRoundtableSeatAttempt
	}
	round := view.Rounds[len(view.Rounds)-1]
	priorContributions, err := execution.readRoundtablePriorContributions(
		ctx, view, round.ID, false,
	)
	if err != nil {
		return roundtable.View{}, err
	}
	defer clearProductRoundtablePriorContributions(priorContributions)
	missionContext, err := execution.missionContext.ResolveRoundtableMissionContext(
		ctx, view.Session.Context.MissionID,
	)
	if err != nil {
		return roundtable.View{}, err
	}
	key := view.Session.ID + "\x00" + round.ID
	execution.mu.Lock()
	if execution.closed {
		execution.mu.Unlock()
		return roundtable.View{}, app.ErrInvalidMissionExecution
	}
	if _, exists := execution.active[key]; exists {
		execution.mu.Unlock()
		return execution.Project(ctx, view), nil
	}
	runContext, cancel := context.WithTimeout(context.Background(), time.Hour)
	execution.wait.Add(1)
	execution.active[key] = productRoundtableActiveExecution{cancel: cancel}
	execution.mu.Unlock()

	seats := make([]app.RoundtableSeatExecutionInput, 0, len(view.Seats))
	for _, seat := range view.SeatsList() {
		if !seat.Available || seat.Binding == nil {
			continue
		}
		binding, err := seat.Binding.RuntimeExecutionBinding()
		if err != nil {
			cancel()
			execution.release(key)
			execution.wait.Done()
			return roundtable.View{}, err
		}
		seats = append(seats, app.RoundtableSeatExecutionInput{
			SeatID: seat.ID, DisplayName: seat.DisplayName,
			TeamRoleKind:       seat.Binding.TeamRoleKind,
			AgentDefinitionID:  seat.Binding.AgentDefinitionID,
			MembershipRevision: seat.Binding.MembershipRevision,
			SeatBindingDigest:  seat.Binding.BindingDigest,
			ExecutionBinding:   binding,
		})
	}
	observer := &productRoundtableOutputObserver{
		execution: execution, sessionID: view.Session.ID,
		attemptBySeat: make(map[string]string),
	}
	compilation, err := app.CompileRoundtableExecution(ctx, app.RoundtableExecutionInput{
		SessionID: view.Session.ID, RoundID: round.ID,
		ConversationID: view.Session.Context.ConversationID,
		MissionID:      view.Session.Context.MissionID, MissionContext: missionContext,
		Title:  view.Session.Title,
		Prompt: strings.TrimSpace(prompt), CorrelationID: correlationID,
		SourcePath: execution.sourcePath, AuthoritativeTime: execution.now().UTC(),
		Seats: seats, ContextCapsules: execution.contextCapsules, OutputObserver: observer,
		PriorContributions: priorContributions,
	})
	if err != nil {
		execution.recordDispatchFailure(ctx, view, round.ID, correlationID, "execution_compile_failed")
		cancel()
		execution.release(key)
		execution.wait.Done()
		return roundtable.View{}, err
	}
	latest := view
	started := make([]app.RoundtableAttemptExecution, 0, len(compilation.Attempts))
	for _, attempt := range compilation.Attempts {
		observer.attemptBySeat[attempt.SeatID] = attempt.AttemptID
		latest, err = execution.authority.StartSeatAttempt(ctx, roundtable.StartSeatAttemptCommand{
			SessionID: view.Session.ID, RoundID: round.ID, SeatID: attempt.SeatID,
			AttemptID: attempt.AttemptID, AttemptNumber: attempt.AttemptNumber,
			ExecutionTeamID: compilation.ExecutionTeamID,
			WorkItemID:      attempt.WorkItemID, RunID: attempt.RunID,
			SegmentID:              attempt.SegmentID,
			ClaimGeneration:        attempt.ClaimGeneration,
			RuntimeInstanceID:      attempt.RuntimeInstanceID,
			AgentInstanceID:        attempt.AgentInstanceID,
			MembershipRevision:     attempt.MembershipRevision,
			SeatBindingDigest:      attempt.SeatBindingDigest,
			ExecutionBindingDigest: attempt.ExecutionBindingDigest,
			ContextCapsuleDigest:   attempt.ContextCapsuleDigest,
			EmittedAt:              execution.now().UTC(), CorrelationID: correlationID,
		})
		if err != nil {
			if len(latest.Attempts) == 0 {
				execution.recordDispatchFailure(ctx, view, round.ID, correlationID, "attempt_start_failed")
			}
			execution.cancelAdmittedAttempts(view.Session.ID, round.ID, started, correlationID)
			cancel()
			execution.release(key)
			execution.wait.Done()
			return roundtable.View{}, err
		}
		execution.setDelivery(view.Session.ID, roundtable.SeatDelivery{
			AttemptID: attempt.AttemptID, SeatID: attempt.SeatID,
			Status: roundtable.SeatAttemptRunning, UpdatedAt: execution.now().UTC(),
		})
		started = append(started, attempt)
	}
	go execution.runRound(runContext, key, compilation, observer, correlationID)
	return execution.Project(ctx, latest), nil
}

func (execution *productRoundtableExecution) cancelAdmittedAttempts(
	sessionID string,
	roundID string,
	attempts []app.RoundtableAttemptExecution,
	correlationID string,
) {
	if len(attempts) == 0 {
		return
	}
	terminalContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, attempt := range attempts {
		if _, err := execution.authority.CancelSeatAttempt(
			terminalContext,
			roundtable.CancelSeatAttemptCommand{
				SessionID: sessionID, RoundID: roundID, SeatID: attempt.SeatID,
				AttemptID: attempt.AttemptID, EmittedAt: execution.now().UTC(),
				CorrelationID: correlationID,
			},
		); err == nil {
			execution.setDelivery(sessionID, roundtable.SeatDelivery{
				AttemptID: attempt.AttemptID, SeatID: attempt.SeatID,
				Status: roundtable.SeatAttemptCancelled, UpdatedAt: execution.now().UTC(),
			})
		}
	}
}

func (execution *productRoundtableExecution) StartRetry(
	ctx context.Context,
	view roundtable.View,
	roundID string,
	seatID string,
	attemptNumber int,
	priorAttempt roundtable.SeatAttempt,
	guidance string,
	interventionID string,
	correlationID string,
) (roundtable.View, error) {
	if execution == nil || ctx == nil || view.Session.Context == nil || attemptNumber < 2 {
		return roundtable.View{}, roundtable.ErrInvalidRoundtableSeatAttempt
	}
	seat, found := view.Seats[seatID]
	if !found || !seat.Available || seat.Binding == nil {
		return roundtable.View{}, roundtable.ErrInvalidRoundtableSeatAttempt
	}
	if priorAttempt.RoundID != roundID || priorAttempt.SeatID != seatID ||
		priorAttempt.AttemptNumber+1 != attemptNumber ||
		priorAttempt.MembershipRevision != seat.Binding.MembershipRevision ||
		priorAttempt.SeatBindingDigest != seat.Binding.BindingDigest ||
		priorAttempt.ContextCapsuleDigest == "" {
		return roundtable.View{}, roundtable.ErrInvalidRoundtableSeatAttempt
	}
	prompt, err := execution.readRoundtablePrompt(ctx, view, priorAttempt)
	if err != nil {
		return roundtable.View{}, err
	}
	priorContributions, err := execution.readRoundtablePriorContributions(
		ctx, view, roundID, true,
	)
	if err != nil {
		return roundtable.View{}, err
	}
	defer clearProductRoundtablePriorContributions(priorContributions)
	missionContext, err := execution.missionContext.ResolveRoundtableMissionContext(
		ctx, view.Session.Context.MissionID,
	)
	if err != nil {
		return roundtable.View{}, err
	}
	binding, err := seat.Binding.RuntimeExecutionBinding()
	if err != nil {
		return roundtable.View{}, err
	}
	key := view.Session.ID + "\x00" + roundID + "\x00" + seatID + "\x00" + strconv.Itoa(attemptNumber)
	execution.mu.Lock()
	if execution.closed {
		execution.mu.Unlock()
		return roundtable.View{}, app.ErrInvalidMissionExecution
	}
	if _, exists := execution.active[key]; exists {
		execution.mu.Unlock()
		return execution.Project(ctx, view), nil
	}
	runContext, cancel := context.WithTimeout(context.Background(), time.Hour)
	execution.wait.Add(1)
	execution.active[key] = productRoundtableActiveExecution{cancel: cancel}
	execution.mu.Unlock()
	observer := &productRoundtableOutputObserver{
		execution: execution, sessionID: view.Session.ID,
		attemptBySeat: make(map[string]string),
	}
	compilation, err := app.CompileRoundtableExecution(ctx, app.RoundtableExecutionInput{
		Mode:      app.RoundtableExecutionRetry,
		SessionID: view.Session.ID, RoundID: roundID,
		ConversationID: view.Session.Context.ConversationID,
		MissionID:      view.Session.Context.MissionID, MissionContext: missionContext,
		Title:  view.Session.Title,
		Prompt: prompt, RetryGuidance: guidance, RetryInterventionID: interventionID,
		CorrelationID: correlationID, SourcePath: execution.sourcePath,
		AuthoritativeTime: execution.now().UTC(), ContextCapsules: execution.contextCapsules,
		OutputObserver: observer, PriorContributions: priorContributions,
		Seats: []app.RoundtableSeatExecutionInput{{
			SeatID: seat.ID, DisplayName: seat.DisplayName,
			TeamRoleKind: seat.Binding.TeamRoleKind, AgentDefinitionID: seat.Binding.AgentDefinitionID,
			MembershipRevision: seat.Binding.MembershipRevision,
			SeatBindingDigest:  seat.Binding.BindingDigest, ExecutionBinding: binding,
			AttemptNumber: attemptNumber,
		}},
	})
	if err != nil {
		cancel()
		execution.release(key)
		execution.wait.Done()
		return roundtable.View{}, err
	}
	attempt := compilation.Attempts[0]
	observer.attemptBySeat[seatID] = attempt.AttemptID
	view, err = execution.authority.StartSeatAttempt(ctx, roundtable.StartSeatAttemptCommand{
		SessionID: view.Session.ID, RoundID: roundID, SeatID: seatID,
		AttemptID: attempt.AttemptID, AttemptNumber: attempt.AttemptNumber,
		ExecutionTeamID: compilation.ExecutionTeamID, WorkItemID: attempt.WorkItemID,
		RunID: attempt.RunID, SegmentID: attempt.SegmentID,
		ClaimGeneration: attempt.ClaimGeneration, RuntimeInstanceID: attempt.RuntimeInstanceID,
		AgentInstanceID: attempt.AgentInstanceID, MembershipRevision: attempt.MembershipRevision,
		SeatBindingDigest:      attempt.SeatBindingDigest,
		ExecutionBindingDigest: attempt.ExecutionBindingDigest,
		ContextCapsuleDigest:   attempt.ContextCapsuleDigest,
		EmittedAt:              execution.now().UTC(), CorrelationID: correlationID,
	})
	if err != nil {
		cancel()
		execution.release(key)
		execution.wait.Done()
		return roundtable.View{}, err
	}
	execution.setDelivery(view.Session.ID, roundtable.SeatDelivery{
		AttemptID: attempt.AttemptID, SeatID: seatID,
		Status: roundtable.SeatAttemptRunning, UpdatedAt: execution.now().UTC(),
	})
	go execution.runRound(runContext, key, compilation, observer, correlationID)
	return execution.Project(ctx, view), nil
}

func (execution *productRoundtableExecution) readRoundtablePrompt(
	ctx context.Context,
	view roundtable.View,
	attempt roundtable.SeatAttempt,
) (string, error) {
	if execution == nil || execution.contextCapsules == nil || ctx == nil ||
		view.Session.Context == nil {
		return "", roundtable.ErrInvalidRoundtableSeatAttempt
	}
	authorities, err := execution.contextCapsules.ListRoleContextCapsuleAuthorities(
		ctx, view.Session.Context.ConversationID,
	)
	if err != nil {
		return "", errors.Join(roundtable.ErrInvalidRoundtableSeatAttempt, err)
	}
	var authority contextcapsule.AuthorityRecord
	matches := 0
	for _, candidate := range authorities {
		if candidate.CapsuleDigest == attempt.ContextCapsuleDigest &&
			candidate.ConversationID == view.Session.Context.ConversationID &&
			candidate.TeamID == attempt.ExecutionTeamID &&
			candidate.AgentID == attempt.AgentInstanceID &&
			candidate.RoleID == attempt.SeatID {
			authority = candidate
			matches++
		}
	}
	if matches != 1 {
		return "", roundtable.ErrInvalidRoundtableSeatAttempt
	}
	capsule, payload, err := execution.contextCapsules.ReadRoleContextCapsule(ctx, authority)
	clearProductRoundtableBytes(payload)
	if err != nil || capsule.AuthorityRecord() != authority {
		return "", errors.Join(roundtable.ErrInvalidRoundtableSeatAttempt, err)
	}
	items := capsule.Disclosed()
	defer func() {
		for index := range items {
			clearProductRoundtableBytes(items[index].Content)
		}
	}()
	var prompt string
	matches = 0
	for _, item := range items {
		if item.ItemID != "roundtable-prompt" {
			continue
		}
		if item.Kind != contextcapsule.KindConversationGoal ||
			item.Trust != contextcapsule.TrustAuthoritative ||
			item.Scope != contextcapsule.ScopeTeamShared ||
			item.Priority != contextcapsule.PrioritySystem || !item.Required ||
			item.SourceType != contextcapsule.SourceAuthority ||
			item.SourceRef != "roundtable:"+view.Session.ID+":"+attempt.RoundID {
			return "", roundtable.ErrInvalidRoundtableSeatAttempt
		}
		prompt = string(item.Content)
		matches++
	}
	trimmed := strings.TrimSpace(prompt)
	if matches != 1 || prompt != trimmed || prompt == "" ||
		len(prompt) > roundtable.MaxRoundPromptBytes || !utf8.ValidString(prompt) ||
		strings.ContainsAny(prompt, "\x00\r") {
		return "", roundtable.ErrInvalidRoundtableSeatAttempt
	}
	return prompt, nil
}

func (execution *productRoundtableExecution) readRoundtablePriorContributions(
	ctx context.Context,
	view roundtable.View,
	targetRoundID string,
	includeTargetRound bool,
) ([]app.RoundtablePriorContribution, error) {
	if execution == nil || execution.payloads == nil || ctx == nil ||
		view.Session.Context == nil {
		return nil, roundtable.ErrInvalidRoundtableSeatAttempt
	}
	rounds := append([]roundtable.Round(nil), view.Rounds...)
	sort.Slice(rounds, func(i, j int) bool { return rounds[i].Sequence < rounds[j].Sequence })
	sourceRoundID := ""
	for index, candidate := range rounds {
		if candidate.ID != targetRoundID {
			continue
		}
		if includeTargetRound {
			sourceRoundID = candidate.ID
		} else if index > 0 {
			sourceRoundID = rounds[index-1].ID
		}
		break
	}
	if sourceRoundID == "" {
		return []app.RoundtablePriorContribution{}, nil
	}
	latestBySeat := make(map[string]roundtable.SeatAttempt)
	for _, attempt := range view.Attempts {
		if attempt.RoundID != sourceRoundID ||
			attempt.Status != roundtable.SeatAttemptSucceeded {
			continue
		}
		current, found := latestBySeat[attempt.SeatID]
		if !found || attempt.AttemptNumber > current.AttemptNumber ||
			attempt.AttemptNumber == current.AttemptNumber &&
				attempt.CompletedAt.After(current.CompletedAt) {
			latestBySeat[attempt.SeatID] = attempt
		}
	}
	seatIDs := make([]string, 0, len(latestBySeat))
	for seatID := range latestBySeat {
		seatIDs = append(seatIDs, seatID)
	}
	sort.Strings(seatIDs)
	contributions := make([]app.RoundtablePriorContribution, 0, len(seatIDs))
	failed := true
	defer func() {
		if failed {
			clearProductRoundtablePriorContributions(contributions)
		}
	}()
	for _, seatID := range seatIDs {
		attempt := latestBySeat[seatID]
		binding, ok := productRoundtableAttemptPayloadBinding(view, attempt)
		if !ok {
			return nil, roundtable.ErrInvalidRoundtableSeatAttempt
		}
		payload, err := execution.payloads.ReadAttemptPayload(ctx, binding)
		if err != nil || payload.Binding != binding {
			payload.Close()
			return nil, errors.Join(roundtable.ErrInvalidRoundtableSeatAttempt, err)
		}
		digest := sha256.Sum256(payload.Content)
		if len(payload.Content) == 0 ||
			hex.EncodeToString(digest[:]) != attempt.OutputDigest {
			payload.Close()
			return nil, roundtable.ErrInvalidRoundtableSeatAttempt
		}
		contributions = append(contributions, app.RoundtablePriorContribution{
			AttemptID: attempt.AttemptID, RoundID: attempt.RoundID,
			SeatID: attempt.SeatID, OutputDigest: attempt.OutputDigest,
			Content: append([]byte(nil), payload.Content...),
		})
		payload.Close()
	}
	failed = false
	return contributions, nil
}

func productRoundtableAttemptPayloadBinding(
	view roundtable.View,
	attempt roundtable.SeatAttempt,
) (attemptpayload.Binding, bool) {
	if view.Session.Context == nil || attempt.PayloadReference == "" ||
		attempt.Status != roundtable.SeatAttemptSucceeded || attempt.OutputDigest == "" {
		return attemptpayload.Binding{}, false
	}
	return attemptpayload.Binding{
		PayloadID: attempt.PayloadReference,
		Scope: attemptpayload.Scope{
			ConversationID: view.Session.Context.ConversationID,
			WorkItemID:     attempt.WorkItemID, RunID: attempt.RunID,
			ClaimGeneration:        attempt.ClaimGeneration,
			RuntimeInstanceID:      attempt.RuntimeInstanceID,
			ExecutionBindingDigest: attempt.ExecutionBindingDigest,
			CapsuleDigest:          attempt.ContextCapsuleDigest,
		},
		CallID: attempt.AttemptID, Sequence: 1,
		ContentType:   attemptpayload.ContentTypeTextUTF8,
		ContentDigest: attempt.OutputDigest,
	}, true
}

func clearProductRoundtablePriorContributions(
	values []app.RoundtablePriorContribution,
) {
	for index := range values {
		clearProductRoundtableBytes(values[index].Content)
		values[index].Content = nil
	}
}

func clearProductRoundtableBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func (execution *productRoundtableExecution) CancelRound(sessionID, roundID string) {
	if execution == nil {
		return
	}
	prefix := sessionID + "\x00" + roundID
	execution.mu.RLock()
	cancels := make([]context.CancelFunc, 0)
	for key, active := range execution.active {
		if (key == prefix || strings.HasPrefix(key, prefix+"\x00")) && active.cancel != nil {
			cancels = append(cancels, active.cancel)
		}
	}
	execution.mu.RUnlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (execution *productRoundtableExecution) runRound(
	runContext context.Context,
	key string,
	compilation app.RoundtableExecutionCompilation,
	observer *productRoundtableOutputObserver,
	correlationID string,
) {
	defer execution.wait.Done()
	defer execution.release(key)
	records, runErr := execution.runner.RunRound(runContext, compilation.Request)
	terminalContext, terminalCancel := context.WithTimeout(context.WithoutCancel(runContext), 10*time.Second)
	defer terminalCancel()
	for _, attempt := range compilation.Attempts {
		record, found := records[attempt.SeatID]
		body := observer.output(attempt.SeatID)
		if found && record.status == "succeeded" && len(body) > 0 {
			if err := execution.succeedAttempt(terminalContext, compilation, attempt, body, correlationID); err == nil {
				continue
			}
			runErr = errors.New("roundtable payload commit failed")
		}
		if runContext.Err() != nil {
			_, _ = execution.authority.CancelSeatAttempt(terminalContext, roundtable.CancelSeatAttemptCommand{
				SessionID: compilation.SessionID, RoundID: compilation.RoundID,
				SeatID: attempt.SeatID, AttemptID: attempt.AttemptID,
				EmittedAt: execution.now().UTC(), CorrelationID: correlationID,
			})
			execution.setDelivery(compilation.SessionID, roundtable.SeatDelivery{
				AttemptID: attempt.AttemptID, SeatID: attempt.SeatID,
				Status: roundtable.SeatAttemptCancelled, Body: string(body), UpdatedAt: execution.now().UTC(),
			})
			continue
		}
		reason := "agent_attempt_failed"
		if found && record.reason != "" {
			reason = safeRoundtableFailureCode(record.reason)
		}
		if !found && runErr != nil {
			reason = "execution_unavailable"
		}
		incidentID := "loom-roundtable-" + productDeterministicUUID(
			"roundtable-attempt-failure", attempt.AttemptID, reason,
		)
		_, _ = execution.authority.FailSeatAttempt(terminalContext, roundtable.FailSeatAttemptCommand{
			SessionID: compilation.SessionID, AttemptID: attempt.AttemptID,
			IncidentID: incidentID, FailureCode: reason,
			FailureStage: "agent_attempt_dispatch", Retryable: true,
			EmittedAt: execution.now().UTC(), CorrelationID: correlationID,
		})
		execution.setDelivery(compilation.SessionID, roundtable.SeatDelivery{
			AttemptID: attempt.AttemptID, SeatID: attempt.SeatID,
			Status: roundtable.SeatAttemptFailed, Body: string(body), UpdatedAt: execution.now().UTC(),
		})
	}
}

func (execution *productRoundtableExecution) recordDispatchFailure(
	ctx context.Context,
	view roundtable.View,
	roundID string,
	correlationID string,
	failureCode string,
) {
	incidentID := "loom-roundtable-" + productDeterministicUUID(
		"roundtable-dispatch-failure", view.Session.ID, roundID, failureCode,
	)
	_, _ = execution.authority.RecordRoundDispatchFailure(ctx, roundtable.RecordRoundDispatchFailureCommand{
		SessionID: view.Session.ID, RoundID: roundID,
		InterventionID: "intervention-" + productDeterministicUUID(
			"roundtable-dispatch-intervention", view.Session.ID, roundID, failureCode,
		),
		ModeratorSeat: view.Session.ModeratorSeat, IncidentID: incidentID,
		FailureCode: failureCode, FailureStage: "agent_attempt_dispatch", Retryable: true,
		EmittedAt: execution.now().UTC(), CorrelationID: correlationID,
	})
}

func (execution *productRoundtableExecution) succeedAttempt(
	ctx context.Context,
	compilation app.RoundtableExecutionCompilation,
	attempt app.RoundtableAttemptExecution,
	body []byte,
	correlationID string,
) error {
	sessionID := compilation.SessionID
	digestBytes := sha256.Sum256(body)
	digest := hex.EncodeToString(digestBytes[:])
	payloadID := "roundtable-payload-" + productDeterministicUUID(
		"roundtable-output", attempt.AttemptID, digest,
	)
	binding := attemptpayload.Binding{
		PayloadID: payloadID,
		Scope: attemptpayload.Scope{
			ConversationID: compilation.ConversationID, WorkItemID: attempt.WorkItemID,
			RunID: attempt.RunID, ClaimGeneration: attempt.ClaimGeneration,
			RuntimeInstanceID:      attempt.RuntimeInstanceID,
			ExecutionBindingDigest: attempt.ExecutionBindingDigest,
			CapsuleDigest:          attempt.ContextCapsuleDigest,
		},
		CallID: attempt.AttemptID, Sequence: 1,
		ContentType: attemptpayload.ContentTypeTextUTF8, ContentDigest: digest,
	}
	if err := execution.payloads.PutAttemptPayload(ctx, attemptpayload.Payload{
		Binding: binding, Status: attemptpayload.StatusPending,
		Content: append([]byte(nil), body...),
	}); err != nil {
		return err
	}
	if _, err := execution.authority.SucceedSeatAttempt(ctx, roundtable.SucceedSeatAttemptCommand{
		SessionID: sessionID, AttemptID: attempt.AttemptID,
		PayloadReference: payloadID, OutputDigest: digest,
		EmittedAt: execution.now().UTC(), CorrelationID: correlationID,
	}); err != nil {
		return err
	}
	execution.setDelivery(sessionID, roundtable.SeatDelivery{
		AttemptID: attempt.AttemptID, SeatID: attempt.SeatID,
		Status: roundtable.SeatAttemptSucceeded, Body: string(body), UpdatedAt: execution.now().UTC(),
	})
	// The RoundTable success fact already binds the encrypted payload digest.
	// A delivery-marker repair must not rewrite that successful seat as failed.
	_ = execution.payloads.MarkAttemptPayloadDelivered(ctx, binding)
	return nil
}

func (execution *productRoundtableExecution) Project(
	ctx context.Context,
	view roundtable.View,
) roundtable.View {
	if execution == nil {
		return view
	}
	execution.mu.RLock()
	for attemptID, delivery := range execution.deliveries[view.Session.ID] {
		if view.Deliveries == nil {
			view.Deliveries = make(map[string]roundtable.SeatDelivery)
		}
		view.Deliveries[attemptID] = delivery
	}
	execution.mu.RUnlock()
	if view.Session.Context != nil {
		for attemptID, attempt := range view.Attempts {
			if attempt.Status != roundtable.SeatAttemptRunning {
				continue
			}
			delivery, found := view.Deliveries[attemptID]
			if !found {
				continue
			}
			delivery.AgentInputCapability = productRoundtableAgentInputPending
			if execution.activeAttempts != nil {
				active, resolveErr := execution.activeAttempts.Resolve(productActiveAttemptQuery{
					ConversationID:  view.Session.Context.ConversationID,
					SegmentID:       attempt.SegmentID,
					AgentInstanceID: attempt.AgentInstanceID,
					WorkItemID:      attempt.WorkItemID,
					RunID:           attempt.RunID,
					ClaimGeneration: attempt.ClaimGeneration,
				})
				if resolveErr == nil {
					if active.AcceptsAgentInputs {
						delivery.AgentInputCapability = productRoundtableAgentInputAvailable
					} else {
						delivery.AgentInputCapability = productRoundtableAgentInputUnavailable
					}
				}
			}
			view.Deliveries[attemptID] = delivery
		}
	}
	for _, attempt := range view.Attempts {
		if attempt.Status != roundtable.SeatAttemptSucceeded || attempt.PayloadReference == "" {
			continue
		}
		if _, found := view.Deliveries[attempt.AttemptID]; found {
			continue
		}
		binding, ok := productRoundtableAttemptPayloadBinding(view, attempt)
		if !ok {
			continue
		}
		payload, err := execution.payloads.ReadAttemptPayload(ctx, binding)
		if err != nil {
			continue
		}
		if view.Deliveries == nil {
			view.Deliveries = make(map[string]roundtable.SeatDelivery)
		}
		view.Deliveries[attempt.AttemptID] = roundtable.SeatDelivery{
			AttemptID: attempt.AttemptID, SeatID: attempt.SeatID,
			Status: attempt.Status, Body: string(payload.Content), UpdatedAt: attempt.CompletedAt,
		}
		payload.Close()
	}
	return view
}

func (execution *productRoundtableExecution) setDelivery(
	sessionID string,
	delivery roundtable.SeatDelivery,
) {
	execution.mu.Lock()
	defer execution.mu.Unlock()
	if execution.deliveries[sessionID] == nil {
		execution.deliveries[sessionID] = make(map[string]roundtable.SeatDelivery)
	}
	execution.deliveries[sessionID][delivery.AttemptID] = delivery
}

func (execution *productRoundtableExecution) release(key string) {
	execution.mu.Lock()
	active, found := execution.active[key]
	delete(execution.active, key)
	execution.mu.Unlock()
	if found && active.cancel != nil {
		active.cancel()
	}
}

func (execution *productRoundtableExecution) Close() error {
	if execution == nil {
		return nil
	}
	execution.mu.Lock()
	if execution.closed {
		execution.mu.Unlock()
		return nil
	}
	execution.closed = true
	cancels := make([]context.CancelFunc, 0, len(execution.active))
	for _, active := range execution.active {
		if active.cancel != nil {
			cancels = append(cancels, active.cancel)
		}
	}
	execution.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		execution.wait.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		return context.DeadlineExceeded
	}
}

type productRoundtableOutputObserver struct {
	execution     *productRoundtableExecution
	sessionID     string
	attemptBySeat map[string]string

	mu      sync.Mutex
	outputs map[string][]byte
}

func (observer *productRoundtableOutputObserver) ObserveNodeOutput(
	ctx context.Context,
	output app.NodeOutput,
) error {
	if observer == nil || observer.execution == nil || ctx == nil {
		return app.ErrInvalidMissionExecution
	}
	frame := output.AuthorizedFrame().Frame()
	if frame.Type() != bridgev1.MessageEvent {
		return nil
	}
	var event struct {
		Delta string `json:"delta"`
	}
	if err := json.Unmarshal(frame.Payload(), &event); err != nil || event.Delta == "" ||
		!utf8.ValidString(event.Delta) {
		return nil
	}
	seatID := output.LogicalNodeID()
	observer.mu.Lock()
	if observer.outputs == nil {
		observer.outputs = make(map[string][]byte)
	}
	remaining := productRoundtableOutputLimit - len(observer.outputs[seatID])
	if remaining > 0 {
		delta := []byte(event.Delta)
		if len(delta) > remaining {
			delta = delta[:remaining]
			for len(delta) > 0 && !utf8.Valid(delta) {
				delta = delta[:len(delta)-1]
			}
		}
		observer.outputs[seatID] = append(observer.outputs[seatID], delta...)
	}
	body := string(observer.outputs[seatID])
	attemptID := observer.attemptBySeat[seatID]
	observer.mu.Unlock()
	if attemptID != "" {
		observer.execution.setDelivery(observer.sessionID, roundtable.SeatDelivery{
			AttemptID: attemptID, SeatID: seatID, Status: roundtable.SeatAttemptRunning,
			Body: body, UpdatedAt: observer.execution.now().UTC(),
		})
	}
	return nil
}

func (observer *productRoundtableOutputObserver) output(seatID string) []byte {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return append([]byte(nil), observer.outputs[seatID]...)
}

func safeRoundtableFailureCode(reason string) string {
	reason = strings.ToLower(strings.TrimSpace(reason))
	var output strings.Builder
	for _, character := range reason {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' ||
			character == '_' || character == '-' {
			output.WriteRune(character)
		}
		if output.Len() >= 64 {
			break
		}
	}
	if output.Len() == 0 {
		return "agent_attempt_failed"
	}
	return output.String()
}
