package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/verification"
)

var (
	ErrInvalidAttemptLoop   = errors.New("invalid Attempt loop")
	ErrAttemptLoopAuthority = errors.New("Attempt loop authority unavailable")
	ErrAttemptLoopConflict  = errors.New("Attempt loop conflict")
)

type ToolExecutionMode string

const (
	ToolExecutionParallel  ToolExecutionMode = "parallel"
	ToolExecutionExclusive ToolExecutionMode = "exclusive"
)

type AttemptLoopStatus string

const (
	AttemptLoopRunning   AttemptLoopStatus = "running"
	AttemptLoopFailed    AttemptLoopStatus = "failed"
	AttemptLoopCancelled AttemptLoopStatus = "cancelled"
)

type AttemptStepOutcome string

const (
	AttemptStepContinue AttemptStepOutcome = "continue"
	AttemptStepFinal    AttemptStepOutcome = "final"
	AttemptStepFailed   AttemptStepOutcome = "failed"
)

type AttemptTurnOutcome string

const (
	AttemptTurnSucceeded AttemptTurnOutcome = "succeeded"
	AttemptTurnFailed    AttemptTurnOutcome = "failed"
	AttemptTurnCancelled AttemptTurnOutcome = "cancelled"
)

type AttemptLoopBinding struct {
	SchemaVersion               int
	AttemptID                   string
	TeamInstanceID              string
	PayloadAuthority            attemptpayload.Authority
	PermissionProfileID         string
	PermissionProfileGeneration int64
	PermissionProfileDigest     string
	CapabilitySetDigest         string
	ToolSchemaSetDigest         string
	BudgetPolicyVersion         int
}

type AttemptLoopBudget struct {
	MaxTurns             int   `json:"max_turns"`
	MaxStepsPerTurn      int   `json:"max_steps_per_turn"`
	MaxToolCalls         int   `json:"max_tool_calls"`
	MaxParallelToolCalls int   `json:"max_parallel_tool_calls"`
	MaxResultBytes       int64 `json:"max_result_bytes"`
	ToolTimeoutMillis    int64 `json:"tool_timeout_millis"`
}

type AttemptLoopTurnInput struct {
	TurnID      string
	Sequence    int
	InputDigest string
}

type AttemptLoopStepInput struct {
	TurnID           string
	StepID           string
	Sequence         int
	ModelInputDigest string
}

type AttemptLoopModelRequestInput struct {
	TurnID        string
	StepID        string
	RequestID     string
	RequestDigest string
}

type AttemptLoopToolCallInput struct {
	TurnID              string
	StepID              string
	CallID              string
	Sequence            int64
	Tool                permissions.ToolKind
	TargetID            string
	ProposalDigest      string
	ArgumentsDigest     string
	ToolSchemaDigest    string
	ExecutionMode       ToolExecutionMode
	ConflictScopeDigest string
}

type AttemptLoopToolDispatchInput struct {
	TurnID              string
	StepID              string
	CallID              string
	AuthorizationDigest string
	ApprovalID          string
	DispatchDigest      string
}

type AttemptLoopStepEndInput struct {
	TurnID       string
	StepID       string
	Outcome      AttemptStepOutcome
	OutputDigest string
	ErrorCode    string
	Retryable    bool
}

type AttemptLoopTurnEndInput struct {
	TurnID  string
	Outcome AttemptTurnOutcome
}

type AttemptLoopToolCallRecord struct {
	CallID              string
	Sequence            int64
	Tool                permissions.ToolKind
	TargetID            string
	ProposalDigest      string
	ArgumentsDigest     string
	ToolSchemaDigest    string
	ExecutionMode       ToolExecutionMode
	ConflictScopeDigest string
	Dispatched          bool
	AuthorizationDigest string
	ApprovalID          string
	DispatchDigest      string
	DispatchEventID     string
	ResultStatus        attemptpayload.FactStatus
	DeliveryProof       attemptpayload.DeliveryProof
}

type AttemptLoopGovernedTestReportRecord struct {
	Report         verification.GovernedTestReport
	PayloadBinding attemptpayload.Binding
	EventID        string
}

type AttemptReportQuery struct {
	TeamInstanceID string
	Authority      attemptpayload.Authority
}

type AttemptLoopStepRecord struct {
	StepID             string
	Sequence           int
	ModelInputDigest   string
	ModelRequestID     string
	ModelRequestDigest string
	ToolCalls          []AttemptLoopToolCallRecord
	Outcome            AttemptStepOutcome
	OutputDigest       string
	ErrorCode          string
	Retryable          bool
}

type AttemptLoopTurnRecord struct {
	TurnID      string
	Sequence    int
	InputDigest string
	Status      AttemptTurnOutcome
	Steps       []AttemptLoopStepRecord
}

type AttemptLoopSnapshot struct {
	Binding             AttemptLoopBinding
	Budget              AttemptLoopBudget
	Status              AttemptLoopStatus
	Turns               []AttemptLoopTurnRecord
	GovernedTestReports []AttemptLoopGovernedTestReportRecord
	LastEventID         string
	Sequence            int64
}

type AttemptLoopAuthority struct {
	runs     *Authority
	payloads *AttemptPayloadAuthority
}

func NewAttemptLoopAuthority(
	runs *Authority,
	payloads *AttemptPayloadAuthority,
) (*AttemptLoopAuthority, error) {
	if runs == nil || runs.store == nil || payloads == nil || payloads.runs != runs {
		return nil, ErrInvalidAttemptLoop
	}
	return &AttemptLoopAuthority{runs: runs, payloads: payloads}, nil
}

func (authority *AttemptLoopAuthority) StartTurn(
	ctx context.Context,
	binding AttemptLoopBinding,
	budget AttemptLoopBudget,
	input AttemptLoopTurnInput,
) (AttemptLoopSnapshot, error) {
	if !validAttemptLoopBudget(budget) || !validAttemptLoopTurnInput(input) {
		return AttemptLoopSnapshot{}, ErrInvalidAttemptLoop
	}
	run, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	if existing, found := attemptLoopTurnByID(state, input.TurnID); found {
		if existing.Sequence == input.Sequence && existing.InputDigest == input.InputDigest &&
			state.Budget == budget {
			return authority.snapshotWithResults(ctx, state)
		}
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	if state.Sequence == 0 {
		if input.Sequence != 1 {
			return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
		}
		now, timeErr := authority.runs.operationTime()
		if timeErr != nil {
			return AttemptLoopSnapshot{}, errors.Join(ErrAttemptLoopAuthority, timeErr)
		}
		startPayload := attemptLoopPayload(binding)
		startPayload.Budget = &budget
		startID := attemptLoopEventID("AttemptLoopStarted", startPayload)
		turnPayload := attemptLoopPayload(binding)
		turnPayload.TurnID = input.TurnID
		turnPayload.TurnSequence = input.Sequence
		turnPayload.TurnInputDigest = input.InputDigest
		turnID := attemptLoopEventID("TurnStarted", turnPayload)
		events := []journal.Event{
			newEvent(
				startID, attemptLoopStream(binding), 1, "AttemptLoopStarted", now,
				binding.PayloadAuthority.IncidentID, run.lastEventID, startPayload,
			),
			newEvent(
				turnID, attemptLoopStream(binding), 2, "TurnStarted", now,
				binding.PayloadAuthority.IncidentID, startID, turnPayload,
			),
		}
		if err := authority.append(ctx, run, state, events); err != nil {
			return AttemptLoopSnapshot{}, err
		}
		return authority.Snapshot(ctx, binding)
	}
	if state.Budget != budget || state.Status != AttemptLoopRunning ||
		len(state.Turns) >= budget.MaxTurns || attemptLoopHasOpenTurn(state) ||
		input.Sequence != len(state.Turns)+1 ||
		state.Turns[len(state.Turns)-1].Status != AttemptTurnSucceeded {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	payload := attemptLoopPayload(binding)
	payload.TurnID = input.TurnID
	payload.TurnSequence = input.Sequence
	payload.TurnInputDigest = input.InputDigest
	if err := authority.appendOne(ctx, run, state, "TurnStarted", payload); err != nil {
		return AttemptLoopSnapshot{}, err
	}
	return authority.Snapshot(ctx, binding)
}

func (authority *AttemptLoopAuthority) StartStep(
	ctx context.Context,
	binding AttemptLoopBinding,
	input AttemptLoopStepInput,
) (AttemptLoopSnapshot, error) {
	if !validAttemptLoopStepInput(input) {
		return AttemptLoopSnapshot{}, ErrInvalidAttemptLoop
	}
	run, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	if existing, found := attemptLoopStepByID(state, input.StepID); found {
		if existing.Sequence == input.Sequence &&
			existing.ModelInputDigest == input.ModelInputDigest {
			return authority.snapshotWithResults(ctx, state)
		}
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	turn, ok := attemptLoopOpenTurn(state, input.TurnID)
	if !ok || len(turn.Steps) >= state.Budget.MaxStepsPerTurn ||
		input.Sequence != len(turn.Steps)+1 || attemptLoopHasOpenStep(*turn) ||
		(input.Sequence > 1 && turn.Steps[len(turn.Steps)-1].Outcome != AttemptStepContinue) {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	payload := attemptLoopPayload(binding)
	payload.TurnID = input.TurnID
	payload.StepID = input.StepID
	payload.StepSequence = input.Sequence
	payload.ModelInputDigest = input.ModelInputDigest
	if err := authority.appendOne(ctx, run, state, "StepStarted", payload); err != nil {
		return AttemptLoopSnapshot{}, err
	}
	return authority.Snapshot(ctx, binding)
}

func (authority *AttemptLoopAuthority) AdmitModelRequest(
	ctx context.Context,
	binding AttemptLoopBinding,
	input AttemptLoopModelRequestInput,
) (AttemptLoopSnapshot, error) {
	if !validAttemptLoopModelRequestInput(input) {
		return AttemptLoopSnapshot{}, ErrInvalidAttemptLoop
	}
	run, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	step, ok := attemptLoopOpenStep(state, input.TurnID, input.StepID)
	if !ok {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	if step.ModelRequestID != "" {
		if step.ModelRequestID == input.RequestID &&
			step.ModelRequestDigest == input.RequestDigest {
			return authority.snapshotWithResults(ctx, state)
		}
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	if len(step.ToolCalls) != 0 {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	payload := attemptLoopPayload(binding)
	payload.TurnID = input.TurnID
	payload.StepID = input.StepID
	payload.ModelRequestID = input.RequestID
	payload.ModelRequestDigest = input.RequestDigest
	if err := authority.appendOne(ctx, run, state, "ModelRequestAdmitted", payload); err != nil {
		return AttemptLoopSnapshot{}, err
	}
	return authority.Snapshot(ctx, binding)
}

func (authority *AttemptLoopAuthority) AdmitToolCall(
	ctx context.Context,
	binding AttemptLoopBinding,
	input AttemptLoopToolCallInput,
) (AttemptLoopSnapshot, error) {
	if !validAttemptLoopToolCallInput(input) {
		return AttemptLoopSnapshot{}, ErrInvalidAttemptLoop
	}
	run, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	if existing, found := attemptLoopCallByID(state, input.CallID); found {
		if sameAttemptLoopCallInput(existing, input) {
			return authority.snapshotWithResults(ctx, state)
		}
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	state, err = authority.snapshotWithResults(ctx, state)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	step, ok := attemptLoopOpenStep(state, input.TurnID, input.StepID)
	if !ok || step.ModelRequestID == "" ||
		attemptLoopCallCount(state) >= state.Budget.MaxToolCalls ||
		input.Sequence != int64(attemptLoopCallCount(state)+1) ||
		attemptLoopActiveCallCount(*step) >= state.Budget.MaxParallelToolCalls {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	for _, call := range step.ToolCalls {
		if call.ResultStatus == attemptpayload.FactDelivered {
			continue
		}
		if input.ExecutionMode == ToolExecutionExclusive ||
			call.ExecutionMode == ToolExecutionExclusive ||
			call.ConflictScopeDigest == input.ConflictScopeDigest {
			return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
		}
	}
	payload := attemptLoopPayload(binding)
	payload.TurnID = input.TurnID
	payload.StepID = input.StepID
	payload.CallID = input.CallID
	payload.CallSequence = input.Sequence
	payload.Tool = input.Tool
	payload.TargetID = input.TargetID
	payload.ProposalDigest = input.ProposalDigest
	payload.ArgumentsDigest = input.ArgumentsDigest
	payload.ToolSchemaDigest = input.ToolSchemaDigest
	payload.ExecutionMode = input.ExecutionMode
	payload.ConflictScopeDigest = input.ConflictScopeDigest
	if err := authority.appendOne(ctx, run, state, "ToolCallAdmitted", payload); err != nil {
		return AttemptLoopSnapshot{}, err
	}
	return authority.Snapshot(ctx, binding)
}

func (authority *AttemptLoopAuthority) CommitToolDispatch(
	ctx context.Context,
	binding AttemptLoopBinding,
	input AttemptLoopToolDispatchInput,
) (AttemptLoopSnapshot, error) {
	if !validAttemptLoopToolDispatchInput(input) {
		return AttemptLoopSnapshot{}, ErrInvalidAttemptLoop
	}
	run, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	step, ok := attemptLoopOpenStep(state, input.TurnID, input.StepID)
	if !ok {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	call, found := attemptLoopCallInStep(step, input.CallID)
	if !found {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	if call.Dispatched {
		if call.AuthorizationDigest == input.AuthorizationDigest &&
			call.ApprovalID == input.ApprovalID && call.DispatchDigest == input.DispatchDigest {
			return authority.snapshotWithResults(ctx, state)
		}
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	payload := attemptLoopPayload(binding)
	payload.TurnID = input.TurnID
	payload.StepID = input.StepID
	payload.CallID = input.CallID
	payload.AuthorizationDigest = input.AuthorizationDigest
	payload.ApprovalID = input.ApprovalID
	payload.DispatchDigest = input.DispatchDigest
	if err := authority.appendOne(ctx, run, state, "ToolDispatchCommitted", payload); err != nil {
		return AttemptLoopSnapshot{}, err
	}
	return authority.Snapshot(ctx, binding)
}

func (authority *AttemptLoopAuthority) AcceptToolResult(
	ctx context.Context,
	binding AttemptLoopBinding,
	payloadBinding attemptpayload.Binding,
) (AttemptLoopSnapshot, error) {
	_, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	call, found := attemptLoopCallByID(state, payloadBinding.CallID)
	if !found || !call.Dispatched || call.DispatchEventID == "" ||
		payloadBinding.Scope != binding.PayloadAuthority.Scope ||
		payloadBinding.Sequence != call.Sequence {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	if err := authority.payloads.accept(
		ctx, binding.PayloadAuthority, payloadBinding, call.DispatchEventID,
	); err != nil {
		if errors.Is(err, ErrAttemptPayloadFactConflict) {
			return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
		}
		return AttemptLoopSnapshot{}, errors.Join(ErrAttemptLoopAuthority, err)
	}
	return authority.Snapshot(ctx, binding)
}

func (authority *AttemptLoopAuthority) DeliverToolResult(
	ctx context.Context,
	binding AttemptLoopBinding,
	payloadBinding attemptpayload.Binding,
	proof attemptpayload.DeliveryProof,
) (AttemptLoopSnapshot, error) {
	_, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	call, found := attemptLoopCallByID(state, payloadBinding.CallID)
	if !found || !call.Dispatched ||
		payloadBinding.Scope != binding.PayloadAuthority.Scope ||
		payloadBinding.Sequence != call.Sequence {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	facts, err := authority.payloads.readFacts(
		ctx, binding.PayloadAuthority, payloadBinding.CallID, payloadBinding.Sequence,
	)
	if err != nil {
		return AttemptLoopSnapshot{}, errors.Join(ErrAttemptLoopAuthority, err)
	}
	if !facts.found || facts.binding != payloadBinding ||
		facts.acceptedCausationID != call.DispatchEventID {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	if err := authority.payloads.Deliver(
		ctx, binding.PayloadAuthority, payloadBinding, proof,
	); err != nil {
		if errors.Is(err, ErrAttemptPayloadFactConflict) {
			return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
		}
		return AttemptLoopSnapshot{}, errors.Join(ErrAttemptLoopAuthority, err)
	}
	return authority.Snapshot(ctx, binding)
}

func (authority *AttemptLoopAuthority) CommitGovernedTestReport(
	ctx context.Context,
	binding AttemptLoopBinding,
	payloadBinding attemptpayload.Binding,
	report verification.GovernedTestReport,
) (AttemptLoopSnapshot, error) {
	if !report.Valid() || !validAttemptPayloadBinding(payloadBinding) {
		return AttemptLoopSnapshot{}, ErrInvalidAttemptLoop
	}
	run, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	call, found := attemptLoopCallByID(state, report.CallID())
	if !found || !call.Dispatched || call.DispatchEventID == "" ||
		call.Tool != permissions.ToolBash || call.Sequence != report.CallSequence() ||
		call.ArgumentsDigest != report.ArgumentsDigest() ||
		payloadBinding.Scope != binding.PayloadAuthority.Scope ||
		payloadBinding.CallID != call.CallID ||
		payloadBinding.Sequence != call.Sequence {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	facts, err := authority.payloads.readFacts(
		ctx, binding.PayloadAuthority, call.CallID, call.Sequence,
	)
	if err != nil {
		return AttemptLoopSnapshot{}, errors.Join(ErrAttemptLoopAuthority, err)
	}
	if !facts.found || facts.binding != payloadBinding ||
		facts.acceptedCausationID != call.DispatchEventID ||
		(facts.status != attemptpayload.FactAccepted &&
			facts.status != attemptpayload.FactDelivered) {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	for _, existing := range state.GovernedTestReports {
		if existing.Report.CallID() != report.CallID() {
			continue
		}
		if existing.Report.Digest() == report.Digest() &&
			existing.PayloadBinding == payloadBinding {
			return authority.snapshotWithResults(ctx, state)
		}
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	reportJSON, err := report.MarshalCanonicalJSON()
	if err != nil {
		return AttemptLoopSnapshot{}, ErrInvalidAttemptLoop
	}
	payload := attemptLoopPayload(binding)
	payload.TurnID = attemptLoopCallTurnID(state, call.CallID)
	payload.StepID = attemptLoopCallStepID(state, call.CallID)
	payload.CallID = call.CallID
	payload.CallSequence = call.Sequence
	payload.TestReport = reportJSON
	payload.ResultPayloadID = payloadBinding.PayloadID
	payload.ResultContentType = payloadBinding.ContentType
	payload.ResultContentDigest = payloadBinding.ContentDigest
	if err := authority.appendOne(
		ctx, run, state, "GovernedTestReportCommitted", payload,
	); err != nil {
		return AttemptLoopSnapshot{}, err
	}
	return authority.Snapshot(ctx, binding)
}

func (authority *AttemptLoopAuthority) GovernedTestReportsForAttempt(
	ctx context.Context,
	query AttemptReportQuery,
) ([]verification.GovernedTestReport, error) {
	if authority == nil || ctx == nil || !validOpaqueID(query.TeamInstanceID) ||
		!validAttemptPayloadAuthority(query.Authority) {
		return nil, ErrInvalidAttemptLoop
	}
	if _, err := authority.payloads.exactRun(ctx, query.Authority); err != nil {
		return nil, errors.Join(ErrAttemptLoopAuthority, err)
	}
	events, err := authority.runs.store.ReadAll(ctx)
	if err != nil {
		return nil, errors.Join(ErrAttemptLoopAuthority, err)
	}
	streams := make(map[string][]journal.Event)
	relatedMismatch := false
	for _, event := range events {
		if !strings.HasPrefix(event.StreamID, "attempt-loop/") {
			continue
		}
		streams[event.StreamID] = append(streams[event.StreamID], event)
	}
	var matched AttemptLoopBinding
	for _, streamEvents := range streams {
		if len(streamEvents) == 0 || streamEvents[0].Type != "AttemptLoopStarted" {
			return nil, ErrAttemptLoopConflict
		}
		var payload attemptLoopEventPayload
		if decodeExactPayload(streamEvents[0].PayloadJSON, &payload) != nil {
			return nil, ErrAttemptLoopConflict
		}
		candidate, ok := attemptLoopBindingFromStartPayload(
			payload,
			streamEvents[0].CorrelationID,
		)
		if !ok || attemptLoopStream(candidate) != streamEvents[0].StreamID {
			return nil, ErrAttemptLoopConflict
		}
		if !attemptReportQueryRelated(query, candidate) {
			continue
		}
		if candidate.TeamInstanceID != query.TeamInstanceID ||
			candidate.PayloadAuthority != query.Authority ||
			streamEvents[0].CorrelationID != query.Authority.IncidentID {
			relatedMismatch = true
			continue
		}
		if matched.AttemptID != "" {
			return nil, ErrAttemptLoopConflict
		}
		matched = candidate
	}
	if matched.AttemptID == "" {
		if relatedMismatch {
			return nil, ErrAttemptLoopAuthority
		}
		return nil, nil
	}
	snapshot, err := authority.Snapshot(ctx, matched)
	if err != nil {
		return nil, err
	}
	reports := make([]verification.GovernedTestReport, 0, len(snapshot.GovernedTestReports))
	for _, record := range snapshot.GovernedTestReports {
		call, found := attemptLoopCallByID(snapshot, record.Report.CallID())
		if !found || call.ResultStatus != attemptpayload.FactDelivered ||
			call.Sequence != record.Report.CallSequence() {
			return nil, ErrAttemptLoopConflict
		}
		reports = append(reports, record.Report)
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].CallSequence() < reports[j].CallSequence()
	})
	return reports, nil
}

// LookupToolResult exposes the exact payload fact lookup needed by a runtime
// delivery broker without allowing that broker to bypass Attempt-loop binding
// validation.
func (authority *AttemptLoopAuthority) LookupToolResult(
	ctx context.Context,
	binding AttemptLoopBinding,
	callID string,
	sequence int64,
) (attemptpayload.Fact, bool, error) {
	_, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return attemptpayload.Fact{}, false, err
	}
	call, found := attemptLoopCallByID(state, callID)
	if !found || call.Sequence != sequence || !call.Dispatched {
		return attemptpayload.Fact{}, false, nil
	}
	fact, factFound, err := authority.payloads.Lookup(
		ctx, binding.PayloadAuthority, callID, sequence,
	)
	if err != nil {
		return attemptpayload.Fact{}, false, errors.Join(ErrAttemptLoopAuthority, err)
	}
	return fact, factFound, nil
}

func (authority *AttemptLoopAuthority) EndStep(
	ctx context.Context,
	binding AttemptLoopBinding,
	input AttemptLoopStepEndInput,
) (AttemptLoopSnapshot, error) {
	if !validAttemptLoopStepEndInput(input) {
		return AttemptLoopSnapshot{}, ErrInvalidAttemptLoop
	}
	run, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	step, ok := attemptLoopOpenStep(state, input.TurnID, input.StepID)
	if !ok || step.ModelRequestID == "" {
		if existing, found := attemptLoopStepByID(state, input.StepID); found &&
			existing.Outcome == input.Outcome && existing.OutputDigest == input.OutputDigest &&
			existing.ErrorCode == input.ErrorCode && existing.Retryable == input.Retryable {
			return authority.snapshotWithResults(ctx, state)
		}
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	for _, call := range step.ToolCalls {
		if !call.Dispatched {
			// A failed step may carry an admitted but undispatched tool call
			// (for example the provider round failed before dispatch); that
			// must not prevent the step from terminating as failed.
			if input.Outcome == AttemptStepFailed {
				continue
			}
			return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
		}
		facts, lookupErr := authority.payloads.readFacts(
			ctx, binding.PayloadAuthority, call.CallID, call.Sequence,
		)
		if lookupErr != nil {
			return AttemptLoopSnapshot{}, errors.Join(ErrAttemptLoopAuthority, lookupErr)
		}
		if !facts.found || facts.status != attemptpayload.FactDelivered ||
			facts.acceptedCausationID != call.DispatchEventID {
			// A dispatched Context call whose result was never delivered (for
			// example the provider round after a bounded denial failed) must
			// still allow the step to finalize as failed; otherwise the
			// Attempt loop stays open forever and the Mission appears running.
			if input.Outcome == AttemptStepFailed {
				continue
			}
			return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
		}
	}
	payload := attemptLoopPayload(binding)
	payload.TurnID = input.TurnID
	payload.StepID = input.StepID
	payload.StepOutcome = input.Outcome
	payload.OutputDigest = input.OutputDigest
	payload.ErrorCode = input.ErrorCode
	payload.Retryable = input.Retryable
	if err := authority.appendOne(ctx, run, state, "StepEnded", payload); err != nil {
		return AttemptLoopSnapshot{}, err
	}
	return authority.Snapshot(ctx, binding)
}

func (authority *AttemptLoopAuthority) EndTurn(
	ctx context.Context,
	binding AttemptLoopBinding,
	input AttemptLoopTurnEndInput,
) (AttemptLoopSnapshot, error) {
	if !validAttemptLoopTurnEndInput(input) {
		return AttemptLoopSnapshot{}, ErrInvalidAttemptLoop
	}
	run, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	turn, ok := attemptLoopOpenTurn(state, input.TurnID)
	if !ok {
		if existing, found := attemptLoopTurnByID(state, input.TurnID); found &&
			existing.Status == input.Outcome {
			return authority.snapshotWithResults(ctx, state)
		}
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	if len(turn.Steps) == 0 || attemptLoopHasOpenStep(*turn) {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	lastOutcome := turn.Steps[len(turn.Steps)-1].Outcome
	if (input.Outcome == AttemptTurnSucceeded && lastOutcome != AttemptStepFinal) ||
		(input.Outcome == AttemptTurnFailed && lastOutcome != AttemptStepFailed) {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	payload := attemptLoopPayload(binding)
	payload.TurnID = input.TurnID
	payload.TurnOutcome = input.Outcome
	if err := authority.appendOne(ctx, run, state, "TurnEnded", payload); err != nil {
		return AttemptLoopSnapshot{}, err
	}
	return authority.Snapshot(ctx, binding)
}

func (authority *AttemptLoopAuthority) Snapshot(
	ctx context.Context,
	binding AttemptLoopBinding,
) (AttemptLoopSnapshot, error) {
	if authority == nil || !validAttemptLoopBinding(binding) || ctx == nil {
		return AttemptLoopSnapshot{}, ErrInvalidAttemptLoop
	}
	run, err := authority.payloads.exactRun(ctx, binding.PayloadAuthority)
	if err != nil {
		return AttemptLoopSnapshot{}, errors.Join(ErrAttemptLoopAuthority, err)
	}
	if err := validateAttemptLoopBindingRun(binding, run); err != nil {
		return AttemptLoopSnapshot{}, err
	}
	events, err := authority.runs.store.ReadStream(ctx, attemptLoopStream(binding))
	if err != nil {
		return AttemptLoopSnapshot{}, errors.Join(ErrAttemptLoopAuthority, err)
	}
	state, err := replayAttemptLoop(binding, events)
	if err != nil {
		return AttemptLoopSnapshot{}, err
	}
	return authority.snapshotWithResults(ctx, state)
}

func (authority *AttemptLoopAuthority) commandState(
	ctx context.Context,
	binding AttemptLoopBinding,
) (RunRecord, AttemptLoopSnapshot, error) {
	if authority == nil || !validAttemptLoopBinding(binding) || ctx == nil {
		return RunRecord{}, AttemptLoopSnapshot{}, ErrInvalidAttemptLoop
	}
	run, err := authority.payloads.currentRun(ctx, binding.PayloadAuthority)
	if err != nil {
		return RunRecord{}, AttemptLoopSnapshot{}, errors.Join(ErrAttemptLoopAuthority, err)
	}
	if err := validateAttemptLoopBindingRun(binding, run); err != nil {
		return RunRecord{}, AttemptLoopSnapshot{}, err
	}
	events, err := authority.runs.store.ReadStream(ctx, attemptLoopStream(binding))
	if err != nil {
		return RunRecord{}, AttemptLoopSnapshot{}, errors.Join(ErrAttemptLoopAuthority, err)
	}
	state, err := replayAttemptLoop(binding, events)
	if err != nil {
		return RunRecord{}, AttemptLoopSnapshot{}, err
	}
	return run, state, nil
}

func validateAttemptLoopBindingRun(binding AttemptLoopBinding, run RunRecord) error {
	if run.ID() != binding.AttemptID || run.ID() != binding.PayloadAuthority.RunID ||
		run.WorkItemID() != binding.PayloadAuthority.WorkItemID ||
		run.ClaimID() != binding.PayloadAuthority.ClaimID ||
		run.ClaimGeneration() != binding.PayloadAuthority.ClaimGeneration ||
		run.RuntimeInstanceID() != binding.PayloadAuthority.RuntimeInstanceID ||
		run.AgentInstanceID() != binding.PayloadAuthority.AgentInstanceID ||
		run.ExecutionBinding().BindingDigest !=
			binding.PayloadAuthority.ExecutionBindingDigest {
		return ErrAttemptLoopAuthority
	}
	if attemptLoopCapabilitySetDigest(run.ExecutionBinding().Capabilities) !=
		binding.CapabilitySetDigest {
		return ErrInvalidAttemptLoop
	}
	return nil
}

func (authority *AttemptLoopAuthority) appendOne(
	ctx context.Context,
	run RunRecord,
	state AttemptLoopSnapshot,
	eventType string,
	payload attemptLoopEventPayload,
) error {
	now, err := authority.runs.operationTime()
	if err != nil {
		return errors.Join(ErrAttemptLoopAuthority, err)
	}
	eventID := attemptLoopEventID(eventType, payload)
	event := newEvent(
		eventID, attemptLoopStream(state.Binding), state.Sequence+1,
		eventType, now, state.Binding.PayloadAuthority.IncidentID,
		state.LastEventID, payload,
	)
	return authority.append(ctx, run, state, []journal.Event{event})
}

func (authority *AttemptLoopAuthority) append(
	ctx context.Context,
	run RunRecord,
	state AttemptLoopSnapshot,
	events []journal.Event,
) error {
	_, err := authority.runs.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{
			{StreamID: runStream(run.id), Sequence: run.streamSequence},
			{StreamID: attemptLoopStream(state.Binding), Sequence: state.Sequence},
		},
		events,
	)
	if err == nil {
		return nil
	}
	if errors.Is(err, journal.ErrStreamHeadConflict) ||
		errors.Is(err, journal.ErrIdempotencyConflict) ||
		errors.Is(err, journal.ErrSequenceConflict) ||
		errors.Is(err, journal.ErrPartialEventBatchConflict) {
		return ErrAttemptLoopConflict
	}
	return errors.Join(ErrAttemptLoopAuthority, err)
}

func (authority *AttemptLoopAuthority) snapshotWithResults(
	ctx context.Context,
	state AttemptLoopSnapshot,
) (AttemptLoopSnapshot, error) {
	state = cloneAttemptLoopSnapshot(state)
	for turnIndex := range state.Turns {
		for stepIndex := range state.Turns[turnIndex].Steps {
			for callIndex := range state.Turns[turnIndex].Steps[stepIndex].ToolCalls {
				call := &state.Turns[turnIndex].Steps[stepIndex].ToolCalls[callIndex]
				fact, found, err := authority.payloads.Lookup(
					ctx, state.Binding.PayloadAuthority, call.CallID, call.Sequence,
				)
				if err != nil {
					return AttemptLoopSnapshot{}, errors.Join(ErrAttemptLoopAuthority, err)
				}
				if found {
					call.ResultStatus = fact.Status
					call.DeliveryProof = fact.Proof
				}
			}
		}
	}
	return state, nil
}

type attemptLoopEventPayload struct {
	SchemaVersion               int                  `json:"schema_version"`
	AttemptID                   string               `json:"attempt_id"`
	TeamInstanceID              string               `json:"team_instance_id,omitempty"`
	ConversationID              string               `json:"conversation_id"`
	WorkItemID                  string               `json:"work_item_id"`
	RunID                       string               `json:"run_id"`
	ClaimID                     string               `json:"claim_id"`
	ClaimGeneration             int64                `json:"claim_generation"`
	RuntimeInstanceID           string               `json:"runtime_instance_id"`
	AgentInstanceID             string               `json:"agent_instance_id"`
	ExecutionBindingDigest      string               `json:"execution_binding_digest"`
	ContextCapsuleDigest        string               `json:"context_capsule_digest"`
	PermissionProfileID         string               `json:"permission_profile_id"`
	PermissionProfileGeneration int64                `json:"permission_profile_generation"`
	PermissionProfileDigest     string               `json:"permission_profile_digest"`
	CapabilitySetDigest         string               `json:"capability_set_digest"`
	ToolSchemaSetDigest         string               `json:"tool_schema_set_digest"`
	BudgetPolicyVersion         int                  `json:"budget_policy_version"`
	Budget                      *AttemptLoopBudget   `json:"budget,omitempty"`
	TurnID                      string               `json:"turn_id,omitempty"`
	TurnSequence                int                  `json:"turn_sequence,omitempty"`
	TurnInputDigest             string               `json:"turn_input_digest,omitempty"`
	StepID                      string               `json:"step_id,omitempty"`
	StepSequence                int                  `json:"step_sequence,omitempty"`
	ModelInputDigest            string               `json:"model_input_digest,omitempty"`
	ModelRequestID              string               `json:"model_request_id,omitempty"`
	ModelRequestDigest          string               `json:"model_request_digest,omitempty"`
	CallID                      string               `json:"call_id,omitempty"`
	CallSequence                int64                `json:"call_sequence,omitempty"`
	Tool                        permissions.ToolKind `json:"tool,omitempty"`
	TargetID                    string               `json:"target_id,omitempty"`
	ProposalDigest              string               `json:"proposal_digest,omitempty"`
	ArgumentsDigest             string               `json:"arguments_digest,omitempty"`
	ToolSchemaDigest            string               `json:"tool_schema_digest,omitempty"`
	ExecutionMode               ToolExecutionMode    `json:"execution_mode,omitempty"`
	ConflictScopeDigest         string               `json:"conflict_scope_digest,omitempty"`
	AuthorizationDigest         string               `json:"authorization_digest,omitempty"`
	ApprovalID                  string               `json:"approval_id,omitempty"`
	DispatchDigest              string               `json:"dispatch_digest,omitempty"`
	StepOutcome                 AttemptStepOutcome   `json:"step_outcome,omitempty"`
	OutputDigest                string               `json:"output_digest,omitempty"`
	ErrorCode                   string               `json:"error_code,omitempty"`
	Retryable                   bool                 `json:"retryable,omitempty"`
	TurnOutcome                 AttemptTurnOutcome   `json:"turn_outcome,omitempty"`
	TestReport                  json.RawMessage      `json:"governed_test_report,omitempty"`
	ResultPayloadID             string               `json:"result_payload_id,omitempty"`
	ResultContentType           string               `json:"result_content_type,omitempty"`
	ResultContentDigest         string               `json:"result_content_digest,omitempty"`
}

func attemptLoopPayload(binding AttemptLoopBinding) attemptLoopEventPayload {
	authority := binding.PayloadAuthority
	return attemptLoopEventPayload{
		SchemaVersion: binding.SchemaVersion, AttemptID: binding.AttemptID,
		TeamInstanceID: binding.TeamInstanceID,
		ConversationID: authority.ConversationID, WorkItemID: authority.WorkItemID,
		RunID: authority.RunID, ClaimID: authority.ClaimID,
		ClaimGeneration:             authority.ClaimGeneration,
		RuntimeInstanceID:           authority.RuntimeInstanceID,
		AgentInstanceID:             authority.AgentInstanceID,
		ExecutionBindingDigest:      authority.ExecutionBindingDigest,
		ContextCapsuleDigest:        authority.CapsuleDigest,
		PermissionProfileID:         binding.PermissionProfileID,
		PermissionProfileGeneration: binding.PermissionProfileGeneration,
		PermissionProfileDigest:     binding.PermissionProfileDigest,
		CapabilitySetDigest:         binding.CapabilitySetDigest,
		ToolSchemaSetDigest:         binding.ToolSchemaSetDigest,
		BudgetPolicyVersion:         binding.BudgetPolicyVersion,
	}
}

func replayAttemptLoop(
	binding AttemptLoopBinding,
	events []journal.Event,
) (AttemptLoopSnapshot, error) {
	state := AttemptLoopSnapshot{Binding: binding}
	streamID := attemptLoopStream(binding)
	for index, event := range events {
		var payload attemptLoopEventPayload
		if event.StreamID != streamID || event.Seq != int64(index+1) ||
			event.SchemaVersion != 1 ||
			event.CorrelationID != binding.PayloadAuthority.IncidentID ||
			event.ID != event.IdempotencyKey || event.CausationID == "" ||
			decodeExactPayload(event.PayloadJSON, &payload) != nil ||
			!sameAttemptLoopPayloadBinding(payload, binding) ||
			event.ID != attemptLoopEventID(event.Type, payload) ||
			(index > 0 && event.CausationID != state.LastEventID) {
			return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
		}
		if err := applyAttemptLoopEvent(&state, event.ID, event.Type, payload); err != nil {
			return AttemptLoopSnapshot{}, err
		}
		state.LastEventID = event.ID
		state.Sequence = event.Seq
	}
	return cloneAttemptLoopSnapshot(state), nil
}

func applyAttemptLoopEvent(
	state *AttemptLoopSnapshot,
	eventID string,
	eventType string,
	payload attemptLoopEventPayload,
) error {
	base := attemptLoopPayload(state.Binding)
	switch eventType {
	case "AttemptLoopStarted":
		expected := base
		expected.Budget = payload.Budget
		if state.Sequence != 0 || payload.Budget == nil ||
			!validAttemptLoopBudget(*payload.Budget) || !reflect.DeepEqual(payload, expected) {
			return ErrAttemptLoopConflict
		}
		state.Budget = *payload.Budget
		state.Status = AttemptLoopRunning
	case "TurnStarted":
		expected := base
		expected.TurnID = payload.TurnID
		expected.TurnSequence = payload.TurnSequence
		expected.TurnInputDigest = payload.TurnInputDigest
		input := AttemptLoopTurnInput{
			TurnID: payload.TurnID, Sequence: payload.TurnSequence,
			InputDigest: payload.TurnInputDigest,
		}
		if state.Status != AttemptLoopRunning || !validAttemptLoopTurnInput(input) ||
			!reflect.DeepEqual(payload, expected) || attemptLoopHasOpenTurn(*state) ||
			payload.TurnSequence != len(state.Turns)+1 ||
			len(state.Turns) >= state.Budget.MaxTurns {
			return ErrAttemptLoopConflict
		}
		state.Turns = append(state.Turns, AttemptLoopTurnRecord{
			TurnID: input.TurnID, Sequence: input.Sequence, InputDigest: input.InputDigest,
		})
	case "StepStarted":
		expected := base
		expected.TurnID, expected.StepID = payload.TurnID, payload.StepID
		expected.StepSequence = payload.StepSequence
		expected.ModelInputDigest = payload.ModelInputDigest
		input := AttemptLoopStepInput{
			TurnID: payload.TurnID, StepID: payload.StepID,
			Sequence: payload.StepSequence, ModelInputDigest: payload.ModelInputDigest,
		}
		turn, ok := attemptLoopOpenTurn(*state, payload.TurnID)
		if !ok || !validAttemptLoopStepInput(input) || !reflect.DeepEqual(payload, expected) ||
			attemptLoopHasOpenStep(*turn) || payload.StepSequence != len(turn.Steps)+1 ||
			len(turn.Steps) >= state.Budget.MaxStepsPerTurn ||
			(payload.StepSequence > 1 && turn.Steps[len(turn.Steps)-1].Outcome != AttemptStepContinue) {
			return ErrAttemptLoopConflict
		}
		turn.Steps = append(turn.Steps, AttemptLoopStepRecord{
			StepID: input.StepID, Sequence: input.Sequence,
			ModelInputDigest: input.ModelInputDigest,
		})
	case "ModelRequestAdmitted":
		expected := base
		expected.TurnID, expected.StepID = payload.TurnID, payload.StepID
		expected.ModelRequestID = payload.ModelRequestID
		expected.ModelRequestDigest = payload.ModelRequestDigest
		input := AttemptLoopModelRequestInput{
			TurnID: payload.TurnID, StepID: payload.StepID,
			RequestID: payload.ModelRequestID, RequestDigest: payload.ModelRequestDigest,
		}
		step, ok := attemptLoopOpenStep(*state, payload.TurnID, payload.StepID)
		if !ok || !validAttemptLoopModelRequestInput(input) ||
			!reflect.DeepEqual(payload, expected) || step.ModelRequestID != "" ||
			len(step.ToolCalls) != 0 {
			return ErrAttemptLoopConflict
		}
		step.ModelRequestID = input.RequestID
		step.ModelRequestDigest = input.RequestDigest
	case "ToolCallAdmitted":
		// Delivery facts live in separate streams. Stream-local replay enforces
		// total sequence and budget; command admission enforces live parallel and
		// conflict slots against those exact facts before this event is appended.
		expected := base
		expected.TurnID, expected.StepID = payload.TurnID, payload.StepID
		expected.CallID, expected.CallSequence = payload.CallID, payload.CallSequence
		expected.Tool, expected.TargetID = payload.Tool, payload.TargetID
		expected.ProposalDigest, expected.ArgumentsDigest = payload.ProposalDigest, payload.ArgumentsDigest
		expected.ToolSchemaDigest, expected.ExecutionMode = payload.ToolSchemaDigest, payload.ExecutionMode
		expected.ConflictScopeDigest = payload.ConflictScopeDigest
		input := AttemptLoopToolCallInput{
			TurnID: payload.TurnID, StepID: payload.StepID,
			CallID: payload.CallID, Sequence: payload.CallSequence,
			Tool: payload.Tool, TargetID: payload.TargetID,
			ProposalDigest: payload.ProposalDigest, ArgumentsDigest: payload.ArgumentsDigest,
			ToolSchemaDigest: payload.ToolSchemaDigest, ExecutionMode: payload.ExecutionMode,
			ConflictScopeDigest: payload.ConflictScopeDigest,
		}
		step, ok := attemptLoopOpenStep(*state, payload.TurnID, payload.StepID)
		if !ok || step.ModelRequestID == "" || !validAttemptLoopToolCallInput(input) ||
			!reflect.DeepEqual(payload, expected) ||
			payload.CallSequence != int64(attemptLoopCallCount(*state)+1) ||
			attemptLoopCallCount(*state) >= state.Budget.MaxToolCalls {
			return ErrAttemptLoopConflict
		}
		step.ToolCalls = append(step.ToolCalls, AttemptLoopToolCallRecord{
			CallID: input.CallID, Sequence: input.Sequence, Tool: input.Tool,
			TargetID: input.TargetID, ProposalDigest: input.ProposalDigest,
			ArgumentsDigest: input.ArgumentsDigest, ToolSchemaDigest: input.ToolSchemaDigest,
			ExecutionMode: input.ExecutionMode, ConflictScopeDigest: input.ConflictScopeDigest,
		})
	case "ToolDispatchCommitted":
		expected := base
		expected.TurnID, expected.StepID, expected.CallID = payload.TurnID, payload.StepID, payload.CallID
		expected.AuthorizationDigest = payload.AuthorizationDigest
		expected.ApprovalID, expected.DispatchDigest = payload.ApprovalID, payload.DispatchDigest
		input := AttemptLoopToolDispatchInput{
			TurnID: payload.TurnID, StepID: payload.StepID, CallID: payload.CallID,
			AuthorizationDigest: payload.AuthorizationDigest,
			ApprovalID:          payload.ApprovalID, DispatchDigest: payload.DispatchDigest,
		}
		step, ok := attemptLoopOpenStep(*state, payload.TurnID, payload.StepID)
		if !ok || !validAttemptLoopToolDispatchInput(input) ||
			!reflect.DeepEqual(payload, expected) {
			return ErrAttemptLoopConflict
		}
		call, found := attemptLoopCallInStep(step, payload.CallID)
		if !found || call.Dispatched {
			return ErrAttemptLoopConflict
		}
		call.Dispatched = true
		call.AuthorizationDigest = input.AuthorizationDigest
		call.ApprovalID = input.ApprovalID
		call.DispatchDigest = input.DispatchDigest
		call.DispatchEventID = eventID
	case "GovernedTestReportCommitted":
		expected := base
		expected.TurnID, expected.StepID = payload.TurnID, payload.StepID
		expected.CallID, expected.CallSequence = payload.CallID, payload.CallSequence
		expected.TestReport = payload.TestReport
		expected.ResultPayloadID = payload.ResultPayloadID
		expected.ResultContentType = payload.ResultContentType
		expected.ResultContentDigest = payload.ResultContentDigest
		report, err := verification.DecodeGovernedTestReport(payload.TestReport)
		if err != nil || !reflect.DeepEqual(payload, expected) ||
			!validOpaqueID(payload.ResultPayloadID) ||
			!validAttemptPayloadContentType(payload.ResultContentType) ||
			!validSHA256Hex(payload.ResultContentDigest) {
			return ErrAttemptLoopConflict
		}
		step, ok := attemptLoopOpenStep(*state, payload.TurnID, payload.StepID)
		if !ok {
			return ErrAttemptLoopConflict
		}
		call, found := attemptLoopCallInStep(step, payload.CallID)
		if !found || !call.Dispatched || call.Tool != permissions.ToolBash ||
			call.Sequence != payload.CallSequence ||
			report.CallID() != call.CallID ||
			report.CallSequence() != call.Sequence ||
			report.ArgumentsDigest() != call.ArgumentsDigest {
			return ErrAttemptLoopConflict
		}
		for _, existing := range state.GovernedTestReports {
			if existing.Report.CallID() == report.CallID() {
				return ErrAttemptLoopConflict
			}
		}
		state.GovernedTestReports = append(
			state.GovernedTestReports,
			AttemptLoopGovernedTestReportRecord{
				Report: report,
				PayloadBinding: attemptpayload.Binding{
					PayloadID: payload.ResultPayloadID,
					Scope:     state.Binding.PayloadAuthority.Scope,
					CallID:    report.CallID(), Sequence: report.CallSequence(),
					ContentType:   payload.ResultContentType,
					ContentDigest: payload.ResultContentDigest,
				},
				EventID: eventID,
			},
		)
	case "StepEnded":
		expected := base
		expected.TurnID, expected.StepID = payload.TurnID, payload.StepID
		expected.StepOutcome, expected.OutputDigest = payload.StepOutcome, payload.OutputDigest
		expected.ErrorCode, expected.Retryable = payload.ErrorCode, payload.Retryable
		input := AttemptLoopStepEndInput{
			TurnID: payload.TurnID, StepID: payload.StepID, Outcome: payload.StepOutcome,
			OutputDigest: payload.OutputDigest, ErrorCode: payload.ErrorCode,
			Retryable: payload.Retryable,
		}
		step, ok := attemptLoopOpenStep(*state, payload.TurnID, payload.StepID)
		if !ok || step.ModelRequestID == "" || !validAttemptLoopStepEndInput(input) ||
			!reflect.DeepEqual(payload, expected) {
			return ErrAttemptLoopConflict
		}
		for _, call := range step.ToolCalls {
			if !call.Dispatched {
				return ErrAttemptLoopConflict
			}
		}
		step.Outcome, step.OutputDigest = input.Outcome, input.OutputDigest
		step.ErrorCode, step.Retryable = input.ErrorCode, input.Retryable
	case "TurnEnded":
		expected := base
		expected.TurnID, expected.TurnOutcome = payload.TurnID, payload.TurnOutcome
		input := AttemptLoopTurnEndInput{TurnID: payload.TurnID, Outcome: payload.TurnOutcome}
		turn, ok := attemptLoopOpenTurn(*state, payload.TurnID)
		if !ok || !validAttemptLoopTurnEndInput(input) ||
			!reflect.DeepEqual(payload, expected) || len(turn.Steps) == 0 ||
			attemptLoopHasOpenStep(*turn) {
			return ErrAttemptLoopConflict
		}
		lastOutcome := turn.Steps[len(turn.Steps)-1].Outcome
		if (input.Outcome == AttemptTurnSucceeded && lastOutcome != AttemptStepFinal) ||
			(input.Outcome == AttemptTurnFailed && lastOutcome != AttemptStepFailed) {
			return ErrAttemptLoopConflict
		}
		turn.Status = input.Outcome
		if input.Outcome == AttemptTurnFailed {
			state.Status = AttemptLoopFailed
		} else if input.Outcome == AttemptTurnCancelled {
			state.Status = AttemptLoopCancelled
		}
	default:
		return ErrAttemptLoopConflict
	}
	return nil
}

func attemptLoopStream(binding AttemptLoopBinding) string {
	authority := binding.PayloadAuthority
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"loom/attempt-loop/v1", binding.AttemptID, binding.TeamInstanceID,
		authority.ConversationID, authority.WorkItemID, authority.RunID,
		authority.ClaimID, fmt.Sprint(authority.ClaimGeneration),
		authority.RuntimeInstanceID, authority.AgentInstanceID,
		authority.ExecutionBindingDigest, authority.CapsuleDigest,
		binding.PermissionProfileDigest, binding.CapabilitySetDigest,
		binding.ToolSchemaSetDigest, fmt.Sprint(binding.BudgetPolicyVersion),
	}, "\x00")))
	return "attempt-loop/" + hex.EncodeToString(digest[:16])
}

func attemptLoopEventID(eventType string, payload attemptLoopEventPayload) string {
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(encoded)
	return deterministicEventID(eventType, hex.EncodeToString(digest[:]))
}

func attemptLoopCapabilitySetDigest(capabilities []string) string {
	values := append([]string(nil), capabilities...)
	sort.Strings(values)
	digest := sha256.Sum256([]byte(strings.Join(
		append([]string{"loom/attempt-loop-capabilities/v1"}, values...), "\x00",
	)))
	return hex.EncodeToString(digest[:])
}

// AttemptLoopCapabilitySetDigest returns the canonical capability-set digest
// used by an AttemptLoopBinding.
func AttemptLoopCapabilitySetDigest(capabilities []string) string {
	return attemptLoopCapabilitySetDigest(capabilities)
}

func validAttemptLoopBinding(binding AttemptLoopBinding) bool {
	return binding.SchemaVersion == 1 && validOpaqueID(binding.AttemptID) &&
		binding.AttemptID == binding.PayloadAuthority.RunID &&
		(binding.TeamInstanceID == "" || validOpaqueID(binding.TeamInstanceID)) &&
		validAttemptPayloadAuthority(binding.PayloadAuthority) &&
		validOpaqueID(binding.PermissionProfileID) &&
		binding.PermissionProfileGeneration > 0 &&
		validSHA256Hex(binding.PermissionProfileDigest) &&
		validSHA256Hex(binding.CapabilitySetDigest) &&
		validSHA256Hex(binding.ToolSchemaSetDigest) &&
		binding.BudgetPolicyVersion == 1
}

func validAttemptLoopBudget(value AttemptLoopBudget) bool {
	return value.MaxTurns > 0 && value.MaxTurns <= 64 &&
		value.MaxStepsPerTurn > 0 && value.MaxStepsPerTurn <= 64 &&
		value.MaxToolCalls > 0 && value.MaxToolCalls <= 256 &&
		value.MaxParallelToolCalls > 0 && value.MaxParallelToolCalls <= 16 &&
		value.MaxParallelToolCalls <= value.MaxToolCalls &&
		value.MaxResultBytes > 0 && value.MaxResultBytes <= 16*1024*1024 &&
		value.ToolTimeoutMillis > 0 && value.ToolTimeoutMillis <= 3_600_000
}

func validAttemptLoopTurnInput(value AttemptLoopTurnInput) bool {
	return validOpaqueID(value.TurnID) && value.Sequence > 0 &&
		validSHA256Hex(value.InputDigest)
}

func validAttemptLoopStepInput(value AttemptLoopStepInput) bool {
	return validOpaqueID(value.TurnID) && validOpaqueID(value.StepID) &&
		value.Sequence > 0 && validSHA256Hex(value.ModelInputDigest)
}

func validAttemptLoopModelRequestInput(value AttemptLoopModelRequestInput) bool {
	return validOpaqueID(value.TurnID) && validOpaqueID(value.StepID) &&
		validOpaqueID(value.RequestID) && validSHA256Hex(value.RequestDigest)
}

func validAttemptLoopToolCallInput(value AttemptLoopToolCallInput) bool {
	return validOpaqueID(value.TurnID) && validOpaqueID(value.StepID) &&
		validOpaqueID(value.CallID) && value.Sequence > 0 &&
		validAttemptLoopTool(value.Tool) && validOpaqueID(value.TargetID) &&
		validSHA256Hex(value.ProposalDigest) && validSHA256Hex(value.ArgumentsDigest) &&
		validSHA256Hex(value.ToolSchemaDigest) &&
		(value.ExecutionMode == ToolExecutionParallel ||
			value.ExecutionMode == ToolExecutionExclusive) &&
		validSHA256Hex(value.ConflictScopeDigest)
}

func validAttemptLoopTool(value permissions.ToolKind) bool {
	switch value {
	case permissions.ToolBash, permissions.ToolRead, permissions.ToolEdit,
		permissions.ToolGrep, permissions.ToolMCPTool, permissions.ToolWebFetch,
		permissions.ToolWebSearch:
		return true
	default:
		return false
	}
}

func validAttemptLoopToolDispatchInput(value AttemptLoopToolDispatchInput) bool {
	return validOpaqueID(value.TurnID) && validOpaqueID(value.StepID) &&
		validOpaqueID(value.CallID) && validSHA256Hex(value.AuthorizationDigest) &&
		(value.ApprovalID == "" || validOpaqueID(value.ApprovalID)) &&
		validSHA256Hex(value.DispatchDigest)
}

func validAttemptLoopStepEndInput(value AttemptLoopStepEndInput) bool {
	if !validOpaqueID(value.TurnID) || !validOpaqueID(value.StepID) ||
		!validSHA256Hex(value.OutputDigest) {
		return false
	}
	switch value.Outcome {
	case AttemptStepContinue, AttemptStepFinal:
		return value.ErrorCode == "" && !value.Retryable
	case AttemptStepFailed:
		return validOpaqueID(value.ErrorCode)
	default:
		return false
	}
}

func validAttemptLoopTurnEndInput(value AttemptLoopTurnEndInput) bool {
	if !validOpaqueID(value.TurnID) {
		return false
	}
	return value.Outcome == AttemptTurnSucceeded ||
		value.Outcome == AttemptTurnFailed
}

func sameAttemptLoopPayloadBinding(
	payload attemptLoopEventPayload,
	binding AttemptLoopBinding,
) bool {
	base := attemptLoopPayload(binding)
	payload.Budget = nil
	payload.TurnID, payload.TurnSequence, payload.TurnInputDigest = "", 0, ""
	payload.StepID, payload.StepSequence, payload.ModelInputDigest = "", 0, ""
	payload.ModelRequestID, payload.ModelRequestDigest = "", ""
	payload.CallID, payload.CallSequence, payload.Tool, payload.TargetID = "", 0, "", ""
	payload.ProposalDigest, payload.ArgumentsDigest, payload.ToolSchemaDigest = "", "", ""
	payload.ExecutionMode, payload.ConflictScopeDigest = "", ""
	payload.AuthorizationDigest, payload.ApprovalID, payload.DispatchDigest = "", "", ""
	payload.StepOutcome, payload.OutputDigest, payload.ErrorCode, payload.Retryable = "", "", "", false
	payload.TurnOutcome = ""
	payload.TestReport = nil
	payload.ResultPayloadID, payload.ResultContentType = "", ""
	payload.ResultContentDigest = ""
	return reflect.DeepEqual(payload, base)
}

func attemptLoopOpenTurn(
	state AttemptLoopSnapshot,
	turnID string,
) (*AttemptLoopTurnRecord, bool) {
	if len(state.Turns) == 0 {
		return nil, false
	}
	turn := &state.Turns[len(state.Turns)-1]
	return turn, turn.TurnID == turnID && turn.Status == ""
}

func attemptLoopOpenStep(
	state AttemptLoopSnapshot,
	turnID string,
	stepID string,
) (*AttemptLoopStepRecord, bool) {
	turn, ok := attemptLoopOpenTurn(state, turnID)
	if !ok || len(turn.Steps) == 0 {
		return nil, false
	}
	step := &turn.Steps[len(turn.Steps)-1]
	return step, step.StepID == stepID && step.Outcome == ""
}

func attemptLoopHasOpenTurn(state AttemptLoopSnapshot) bool {
	if len(state.Turns) == 0 {
		return false
	}
	return state.Turns[len(state.Turns)-1].Status == ""
}

func attemptLoopHasOpenStep(turn AttemptLoopTurnRecord) bool {
	if len(turn.Steps) == 0 {
		return false
	}
	return turn.Steps[len(turn.Steps)-1].Outcome == ""
}

func attemptLoopTurnByID(
	state AttemptLoopSnapshot,
	id string,
) (AttemptLoopTurnRecord, bool) {
	for _, turn := range state.Turns {
		if turn.TurnID == id {
			return turn, true
		}
	}
	return AttemptLoopTurnRecord{}, false
}

func attemptLoopStepByID(
	state AttemptLoopSnapshot,
	id string,
) (AttemptLoopStepRecord, bool) {
	for _, turn := range state.Turns {
		for _, step := range turn.Steps {
			if step.StepID == id {
				return step, true
			}
		}
	}
	return AttemptLoopStepRecord{}, false
}

func attemptLoopCallByID(
	state AttemptLoopSnapshot,
	id string,
) (AttemptLoopToolCallRecord, bool) {
	for _, turn := range state.Turns {
		for _, step := range turn.Steps {
			for _, call := range step.ToolCalls {
				if call.CallID == id {
					return call, true
				}
			}
		}
	}
	return AttemptLoopToolCallRecord{}, false
}

func attemptLoopCallTurnID(state AttemptLoopSnapshot, id string) string {
	for _, turn := range state.Turns {
		for _, step := range turn.Steps {
			for _, call := range step.ToolCalls {
				if call.CallID == id {
					return turn.TurnID
				}
			}
		}
	}
	return ""
}

func attemptLoopCallStepID(state AttemptLoopSnapshot, id string) string {
	for _, turn := range state.Turns {
		for _, step := range turn.Steps {
			for _, call := range step.ToolCalls {
				if call.CallID == id {
					return step.StepID
				}
			}
		}
	}
	return ""
}

func attemptLoopCallInStep(
	step *AttemptLoopStepRecord,
	id string,
) (*AttemptLoopToolCallRecord, bool) {
	for index := range step.ToolCalls {
		if step.ToolCalls[index].CallID == id {
			return &step.ToolCalls[index], true
		}
	}
	return nil, false
}

func attemptLoopCallCount(state AttemptLoopSnapshot) int {
	count := 0
	for _, turn := range state.Turns {
		for _, step := range turn.Steps {
			count += len(step.ToolCalls)
		}
	}
	return count
}

func attemptLoopActiveCallCount(step AttemptLoopStepRecord) int {
	count := 0
	for _, call := range step.ToolCalls {
		if call.ResultStatus != attemptpayload.FactDelivered {
			count++
		}
	}
	return count
}

func sameAttemptLoopCallInput(
	record AttemptLoopToolCallRecord,
	input AttemptLoopToolCallInput,
) bool {
	return record.CallID == input.CallID && record.Sequence == input.Sequence &&
		record.Tool == input.Tool && record.TargetID == input.TargetID &&
		record.ProposalDigest == input.ProposalDigest &&
		record.ArgumentsDigest == input.ArgumentsDigest &&
		record.ToolSchemaDigest == input.ToolSchemaDigest &&
		record.ExecutionMode == input.ExecutionMode &&
		record.ConflictScopeDigest == input.ConflictScopeDigest
}

func cloneAttemptLoopSnapshot(input AttemptLoopSnapshot) AttemptLoopSnapshot {
	clone := input
	clone.GovernedTestReports = append(
		[]AttemptLoopGovernedTestReportRecord(nil),
		input.GovernedTestReports...,
	)
	clone.Turns = make([]AttemptLoopTurnRecord, len(input.Turns))
	for turnIndex, turn := range input.Turns {
		clone.Turns[turnIndex] = turn
		clone.Turns[turnIndex].Steps = make([]AttemptLoopStepRecord, len(turn.Steps))
		for stepIndex, step := range turn.Steps {
			clone.Turns[turnIndex].Steps[stepIndex] = step
			clone.Turns[turnIndex].Steps[stepIndex].ToolCalls = append(
				[]AttemptLoopToolCallRecord(nil), step.ToolCalls...,
			)
		}
	}
	return clone
}

func validAttemptPayloadContentType(value string) bool {
	return value == attemptpayload.ContentTypeJSON ||
		value == attemptpayload.ContentTypeTextUTF8
}

func attemptLoopBindingFromStartPayload(
	payload attemptLoopEventPayload,
	incidentID string,
) (AttemptLoopBinding, bool) {
	if payload.Budget == nil {
		return AttemptLoopBinding{}, false
	}
	binding := AttemptLoopBinding{
		SchemaVersion:  payload.SchemaVersion,
		AttemptID:      payload.AttemptID,
		TeamInstanceID: payload.TeamInstanceID,
		PayloadAuthority: attemptpayload.Authority{
			Scope: attemptpayload.Scope{
				ConversationID: payload.ConversationID,
				WorkItemID:     payload.WorkItemID, RunID: payload.RunID,
				ClaimGeneration:        payload.ClaimGeneration,
				RuntimeInstanceID:      payload.RuntimeInstanceID,
				ExecutionBindingDigest: payload.ExecutionBindingDigest,
				CapsuleDigest:          payload.ContextCapsuleDigest,
			},
			ClaimID: payload.ClaimID, AgentInstanceID: payload.AgentInstanceID,
			IncidentID: incidentID,
		},
		PermissionProfileID:         payload.PermissionProfileID,
		PermissionProfileGeneration: payload.PermissionProfileGeneration,
		PermissionProfileDigest:     payload.PermissionProfileDigest,
		CapabilitySetDigest:         payload.CapabilitySetDigest,
		ToolSchemaSetDigest:         payload.ToolSchemaSetDigest,
		BudgetPolicyVersion:         payload.BudgetPolicyVersion,
	}
	return binding, validAttemptLoopBinding(binding) &&
		validAttemptLoopBudget(*payload.Budget)
}

func attemptReportQueryRelated(
	query AttemptReportQuery,
	binding AttemptLoopBinding,
) bool {
	left, right := query.Authority, binding.PayloadAuthority
	return query.TeamInstanceID == binding.TeamInstanceID &&
		left.WorkItemID == right.WorkItemID && left.RunID == right.RunID &&
		left.ClaimID == right.ClaimID &&
		left.ClaimGeneration == right.ClaimGeneration &&
		left.RuntimeInstanceID == right.RuntimeInstanceID &&
		left.AgentInstanceID == right.AgentInstanceID
}
