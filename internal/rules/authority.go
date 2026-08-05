package rules

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/journal"
)

var (
	ErrInvalidRuleInput              = errors.New("invalid Rule input")
	ErrRuleEffectConflict            = errors.New("Rule effect conflict")
	ErrRuleAuthorityConflict         = errors.New("Rule authority conflict")
	ErrCustomerAuthorizationRequired = errors.New("customer authorization required")
	ErrCustomerAuthorizationDenied   = errors.New("customer authorization denied")
	ErrInvalidApprovalInput          = errors.New("invalid approval input")
	ErrApprovalNotRequired           = errors.New("approval not required")
	ErrApprovalAlreadyPending        = errors.New("approval already pending")
	ErrApprovalNotPending            = errors.New("approval not pending")
	ErrApprovalAlreadyTerminal       = errors.New("approval already terminal")
	ErrApprovalExpired               = errors.New("approval expired")
	ErrApprovalStale                 = errors.New("approval stale")
	ErrApprovalRunAlreadyClaimed     = errors.New("approval Run already claimed")
)

const (
	maxRuleTextBytes          = 128
	maxRuleSetRules           = 32
	maxRuleSets               = 4
	maxMatchedRules           = 16
	maxApproverRefs           = 8
	maxDecisionMarkers        = 16
	maxAuthorizationBytes     = 4096
	maxApprovalTimeout        = 30 * 24 * time.Hour
	maxApprovalReadStreamHead = 7
)

type Scope struct {
	kind string
	id   string
}

func NewScope(kind, id string) (Scope, error) {
	kind = strings.TrimSpace(kind)
	id = strings.TrimSpace(id)
	if !validScopeKind(kind) || !validRuleText(id) {
		return Scope{}, ErrInvalidRuleInput
	}
	return Scope{kind: kind, id: id}, nil
}

func (scope Scope) Kind() string { return scope.kind }
func (scope Scope) ID() string   { return scope.id }

type Condition struct {
	action string
	risk   string
}

func NewCondition(action, risk string) (Condition, error) {
	action = strings.TrimSpace(action)
	risk = strings.TrimSpace(risk)
	if !validRuleText(action) ||
		(risk != "" && risk != "low" && risk != "medium" && risk != "high") {
		return Condition{}, ErrInvalidRuleInput
	}
	return Condition{action: action, risk: risk}, nil
}

func (condition Condition) Action() string { return condition.action }
func (condition Condition) Risk() string   { return condition.risk }

type Effect struct {
	kind         string
	marker       string
	approverRefs []string
	timeout      time.Duration
	onTimeout    string
}

func NewEffect(
	kind, marker string,
	approverRefs []string,
	timeout time.Duration,
	onTimeout string,
) (Effect, error) {
	kind = strings.TrimSpace(kind)
	marker = strings.TrimSpace(marker)
	onTimeout = strings.TrimSpace(onTimeout)
	approvers, ok := normalizedUnique(approverRefs, maxApproverRefs)
	if !ok {
		return Effect{}, ErrInvalidRuleInput
	}
	switch kind {
	case "record", "warn":
		if !validRuleText(marker) || len(approvers) != 0 ||
			timeout != 0 || onTimeout != "" {
			return Effect{}, ErrInvalidRuleInput
		}
	case "require_approval":
		if marker != "" || len(approvers) == 0 ||
			timeout < time.Second || timeout > maxApprovalTimeout ||
			(onTimeout != "reject" && onTimeout != "cancel") {
			return Effect{}, ErrInvalidRuleInput
		}
	case "reject":
		if marker != "" || len(approvers) != 0 ||
			timeout != 0 || onTimeout != "" {
			return Effect{}, ErrInvalidRuleInput
		}
	default:
		return Effect{}, ErrInvalidRuleInput
	}
	return Effect{
		kind: kind, marker: marker, approverRefs: approvers,
		timeout: timeout, onTimeout: onTimeout,
	}, nil
}

func (effect Effect) Kind() string           { return effect.kind }
func (effect Effect) Marker() string         { return effect.marker }
func (effect Effect) ApproverRefs() []string { return append([]string(nil), effect.approverRefs...) }
func (effect Effect) Timeout() time.Duration { return effect.timeout }
func (effect Effect) OnTimeout() string      { return effect.onTimeout }

type Rule struct {
	id        string
	condition Condition
	effect    Effect
}

func NewRule(id string, condition Condition, effect Effect) (Rule, error) {
	id = strings.TrimSpace(id)
	if !validRuleText(id) || !validCondition(condition) || !validEffect(effect) {
		return Rule{}, ErrInvalidRuleInput
	}
	return Rule{id: id, condition: condition, effect: cloneEffect(effect)}, nil
}

func (rule Rule) ID() string           { return rule.id }
func (rule Rule) Condition() Condition { return rule.condition }
func (rule Rule) Effect() Effect       { return cloneEffect(rule.effect) }

type RuleSet struct {
	scope   Scope
	version int
	rules   []Rule
	digest  string
}

func NewRuleSet(scope Scope, version int, rules []Rule) (RuleSet, error) {
	if !validScope(scope) || version <= 0 ||
		len(rules) == 0 || len(rules) > maxRuleSetRules {
		return RuleSet{}, ErrInvalidRuleInput
	}
	cloned := cloneRules(rules)
	sort.Slice(cloned, func(i, j int) bool { return cloned[i].id < cloned[j].id })
	for index, rule := range cloned {
		if !validRule(rule) || (index > 0 && cloned[index-1].id == rule.id) {
			return RuleSet{}, ErrInvalidRuleInput
		}
	}
	ruleSet := RuleSet{scope: scope, version: version, rules: cloned}
	ruleSet.digest = digestRuleSet(ruleSet)
	return ruleSet, nil
}

func (ruleSet RuleSet) Scope() Scope   { return ruleSet.scope }
func (ruleSet RuleSet) Version() int   { return ruleSet.version }
func (ruleSet RuleSet) Rules() []Rule  { return cloneRules(ruleSet.rules) }
func (ruleSet RuleSet) Digest() string { return ruleSet.digest }
func (ruleSet RuleSet) StreamID() string {
	return ruleSetStream(ruleSet.scope)
}

type ActionContextInput struct {
	ProjectID       string
	TeamInstanceID  string
	WorkPackageID   string
	WorkItemID      string
	RunID           string
	AgentInstanceID string
	LogicalNodeID   string
	AttemptNumber   int
	Action          string
	Risk            string
	ClaimID         string
	ClaimGeneration int64
	ContractDigest  string
}

type ActionContext struct {
	projectID       string
	teamInstanceID  string
	workPackageID   string
	workItemID      string
	runID           string
	agentInstanceID string
	logicalNodeID   string
	attemptNumber   int
	action          string
	risk            string
	claimID         string
	claimGeneration int64
	contractDigest  string
	digest          string
}

func NewActionContext(input ActionContextInput) (ActionContext, error) {
	input.ProjectID = strings.TrimSpace(input.ProjectID)
	input.TeamInstanceID = strings.TrimSpace(input.TeamInstanceID)
	input.WorkPackageID = strings.TrimSpace(input.WorkPackageID)
	input.WorkItemID = strings.TrimSpace(input.WorkItemID)
	input.RunID = strings.TrimSpace(input.RunID)
	input.AgentInstanceID = strings.TrimSpace(input.AgentInstanceID)
	input.LogicalNodeID = strings.TrimSpace(input.LogicalNodeID)
	input.Action = strings.TrimSpace(input.Action)
	input.Risk = strings.TrimSpace(input.Risk)
	input.ClaimID = strings.TrimSpace(input.ClaimID)
	input.ContractDigest = strings.TrimSpace(input.ContractDigest)
	if !validRuleText(input.ProjectID) ||
		!validRuleText(input.TeamInstanceID) ||
		!validRuleText(input.WorkPackageID) ||
		!validRuleText(input.WorkItemID) ||
		!validRuleText(input.RunID) ||
		!validRuleText(input.AgentInstanceID) ||
		!validRuleText(input.LogicalNodeID) ||
		input.AttemptNumber <= 0 ||
		!validRuleText(input.Action) ||
		(input.Risk != "low" && input.Risk != "medium" && input.Risk != "high") ||
		input.ClaimGeneration < 0 ||
		(input.ClaimGeneration == 0 && input.ClaimID != "") ||
		(input.ClaimGeneration > 0 && !validRuleText(input.ClaimID)) ||
		!validSHA256(input.ContractDigest) {
		return ActionContext{}, ErrInvalidRuleInput
	}
	action := ActionContext{
		projectID: input.ProjectID, teamInstanceID: input.TeamInstanceID,
		workPackageID: input.WorkPackageID, workItemID: input.WorkItemID,
		runID: input.RunID, agentInstanceID: input.AgentInstanceID,
		logicalNodeID: input.LogicalNodeID, attemptNumber: input.AttemptNumber,
		action: input.Action, risk: input.Risk, claimID: input.ClaimID,
		claimGeneration: input.ClaimGeneration,
		contractDigest:  input.ContractDigest,
	}
	action.digest = digestActionContext(action)
	return action, nil
}

func (action ActionContext) Digest() string          { return action.digest }
func (action ActionContext) ProjectID() string       { return action.projectID }
func (action ActionContext) TeamInstanceID() string  { return action.teamInstanceID }
func (action ActionContext) WorkPackageID() string   { return action.workPackageID }
func (action ActionContext) WorkItemID() string      { return action.workItemID }
func (action ActionContext) RunID() string           { return action.runID }
func (action ActionContext) AgentInstanceID() string { return action.agentInstanceID }
func (action ActionContext) LogicalNodeID() string   { return action.logicalNodeID }
func (action ActionContext) AttemptNumber() int      { return action.attemptNumber }
func (action ActionContext) Action() string          { return action.action }
func (action ActionContext) Risk() string            { return action.risk }
func (action ActionContext) ClaimID() string         { return action.claimID }
func (action ActionContext) ClaimGeneration() int64  { return action.claimGeneration }
func (action ActionContext) ContractDigest() string  { return action.contractDigest }

type RuleSetReference struct {
	StreamID string `json:"stream_id"`
	Kind     string `json:"scope_kind"`
	ScopeID  string `json:"scope_id"`
	Version  int    `json:"version"`
	Digest   string `json:"digest"`
}

type matchedRuleReference struct {
	scopeRank int
	ruleID    string
	version   int
}

type Decision struct {
	kind           string
	digest         string
	actionDigest   string
	ruleSetRefs    []RuleSetReference
	approverRefs   []string
	timeout        time.Duration
	onTimeout      string
	warningMarkers []string
	recordMarkers  []string
}

func (decision Decision) Kind() string   { return decision.kind }
func (decision Decision) Digest() string { return decision.digest }
func (decision Decision) RuleSetReferences() []RuleSetReference {
	return append([]RuleSetReference(nil), decision.ruleSetRefs...)
}
func (decision Decision) ApproverRefs() []string {
	return append([]string(nil), decision.approverRefs...)
}
func (decision Decision) Timeout() time.Duration { return decision.timeout }
func (decision Decision) OnTimeout() string      { return decision.onTimeout }
func (decision Decision) WarningMarkers() []string {
	return append([]string(nil), decision.warningMarkers...)
}
func (decision Decision) RecordMarkers() []string {
	return append([]string(nil), decision.recordMarkers...)
}

func Evaluate(ruleSets []RuleSet, action ActionContext) (Decision, error) {
	if !validActionContext(action) || len(ruleSets) > maxRuleSets {
		return Decision{}, ErrInvalidRuleInput
	}
	sets := cloneRuleSets(ruleSets)
	seenScopes := make(map[string]struct{}, len(sets))
	for _, ruleSet := range sets {
		if !validRuleSet(ruleSet) || !scopeMatches(ruleSet.scope, action) {
			return Decision{}, ErrInvalidRuleInput
		}
		scopeKey := ruleSet.scope.kind + "\x00" + ruleSet.scope.id
		if _, exists := seenScopes[scopeKey]; exists {
			return Decision{}, ErrInvalidRuleInput
		}
		seenScopes[scopeKey] = struct{}{}
	}
	sort.Slice(sets, func(i, j int) bool {
		left := scopeRank(sets[i].scope.kind)
		right := scopeRank(sets[j].scope.kind)
		if left != right {
			return left > right
		}
		return sets[i].StreamID() < sets[j].StreamID()
	})

	decision := Decision{kind: "allow", actionDigest: action.digest}
	var matched []matchedRuleReference
	var approvalPolicySet bool
	for _, ruleSet := range sets {
		decision.ruleSetRefs = append(decision.ruleSetRefs, RuleSetReference{
			StreamID: ruleSet.StreamID(), Kind: ruleSet.scope.kind,
			ScopeID: ruleSet.scope.id, Version: ruleSet.version,
			Digest: ruleSet.digest,
		})
		for _, rule := range ruleSet.rules {
			if rule.condition.action != action.action ||
				(rule.condition.risk != "" && rule.condition.risk != action.risk) {
				continue
			}
			matched = append(matched, matchedRuleReference{
				scopeRank: scopeRank(ruleSet.scope.kind),
				ruleID:    rule.id, version: ruleSet.version,
			})
			if len(matched) > maxMatchedRules {
				return Decision{}, ErrInvalidRuleInput
			}
			switch rule.effect.kind {
			case "reject":
				decision.kind = "reject"
			case "require_approval":
				if approvalPolicySet &&
					(decision.timeout != rule.effect.timeout ||
						decision.onTimeout != rule.effect.onTimeout) {
					return Decision{}, ErrRuleEffectConflict
				}
				approvalPolicySet = true
				decision.timeout = rule.effect.timeout
				decision.onTimeout = rule.effect.onTimeout
				decision.approverRefs = append(
					decision.approverRefs,
					rule.effect.approverRefs...,
				)
				if decision.kind != "reject" {
					decision.kind = "require_approval"
				}
			case "warn":
				decision.warningMarkers = append(
					decision.warningMarkers,
					rule.effect.marker,
				)
			case "record":
				decision.recordMarkers = append(
					decision.recordMarkers,
					rule.effect.marker,
				)
			}
		}
	}
	sort.Slice(decision.ruleSetRefs, func(i, j int) bool {
		return decision.ruleSetRefs[i].StreamID < decision.ruleSetRefs[j].StreamID
	})
	if decision.kind == "reject" {
		decision.approverRefs = nil
		decision.timeout = 0
		decision.onTimeout = ""
	}
	decision.approverRefs, _ = normalizedUnique(decision.approverRefs, maxApproverRefs)
	decision.warningMarkers, _ = normalizedUnique(decision.warningMarkers, maxDecisionMarkers)
	decision.recordMarkers, _ = normalizedUnique(decision.recordMarkers, maxDecisionMarkers)
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].scopeRank != matched[j].scopeRank {
			return matched[i].scopeRank > matched[j].scopeRank
		}
		if matched[i].ruleID != matched[j].ruleID {
			return matched[i].ruleID < matched[j].ruleID
		}
		return matched[i].version < matched[j].version
	})
	decision.digest = digestDecision(decision, matched)
	return decision, nil
}

type RuleSetActivationRequest struct {
	ruleSet                   RuleSet
	authorizationPresentation []byte
	correlationID             string
}

func NewRuleSetActivationRequest(
	ruleSet RuleSet,
	authorizationPresentation []byte,
) (RuleSetActivationRequest, error) {
	if !validRuleSet(ruleSet) ||
		len(authorizationPresentation) == 0 ||
		len(authorizationPresentation) > maxAuthorizationBytes {
		return RuleSetActivationRequest{}, ErrInvalidRuleInput
	}
	return RuleSetActivationRequest{
		ruleSet: ruleSet,
		authorizationPresentation: append(
			[]byte(nil),
			authorizationPresentation...,
		),
	}, nil
}

func (request RuleSetActivationRequest) RuleSet() RuleSet {
	return cloneRuleSets([]RuleSet{request.ruleSet})[0]
}

func (request RuleSetActivationRequest) AuthorizationPresentation() []byte {
	return append([]byte(nil), request.authorizationPresentation...)
}

func (request RuleSetActivationRequest) CorrelationID() string {
	return request.correlationID
}

type ApprovalDecisionRequest struct {
	approvalRequestID         string
	approvalRequestDigest     string
	decision                  string
	authorizationPresentation []byte
	correlationID             string
}

func NewApprovalDecisionRequest(
	approvalRequestID, approvalRequestDigest, decision string,
	authorizationPresentation []byte,
) (ApprovalDecisionRequest, error) {
	approvalRequestID = strings.TrimSpace(approvalRequestID)
	approvalRequestDigest = strings.TrimSpace(approvalRequestDigest)
	decision = strings.TrimSpace(decision)
	if !validRuleText(approvalRequestID) ||
		!validSHA256(approvalRequestDigest) ||
		!validApprovalDecision(decision) ||
		len(authorizationPresentation) == 0 ||
		len(authorizationPresentation) > maxAuthorizationBytes {
		return ApprovalDecisionRequest{}, ErrInvalidApprovalInput
	}
	return ApprovalDecisionRequest{
		approvalRequestID:     approvalRequestID,
		approvalRequestDigest: approvalRequestDigest,
		decision:              decision,
		authorizationPresentation: append(
			[]byte(nil),
			authorizationPresentation...,
		),
	}, nil
}

func (request ApprovalDecisionRequest) ApprovalRequestID() string {
	return request.approvalRequestID
}

func (request ApprovalDecisionRequest) ApprovalRequestDigest() string {
	return request.approvalRequestDigest
}

func (request ApprovalDecisionRequest) Decision() string {
	return request.decision
}

func (request ApprovalDecisionRequest) AuthorizationPresentation() []byte {
	return append([]byte(nil), request.authorizationPresentation...)
}

func (request ApprovalDecisionRequest) CorrelationID() string {
	return request.correlationID
}

type AuthorizedRuleSetActivation struct {
	scopeKind           string
	scopeID             string
	revision            int
	ruleSetDigest       string
	actorRef            string
	commandDigest       string
	authorizationDigest string
	issuedAt            time.Time
	expiresAt           time.Time
	correlationID       string
	requestID           string
	seal                [32]byte
}

type AuthorizedApprovalDecision struct {
	approvalRequestID     string
	approvalRequestDigest string
	decision              string
	actorRef              string
	commandDigest         string
	authorizationDigest   string
	issuedAt              time.Time
	expiresAt             time.Time
	correlationID         string
	requestID             string
	seal                  [32]byte
}

func NewAuthorizedRuleSetActivation(
	request RuleSetActivationRequest,
	actorRef, authorizationDigest string,
	issuedAt, expiresAt time.Time,
) (AuthorizedRuleSetActivation, error) {
	if !validRuleSet(request.ruleSet) ||
		!validRuleText(request.correlationID) ||
		!validRuleText(actorRef) ||
		!validSHA256(authorizationDigest) ||
		issuedAt.IsZero() || issuedAt.Location() != time.UTC ||
		expiresAt.IsZero() || expiresAt.Location() != time.UTC ||
		!issuedAt.Before(expiresAt) {
		return AuthorizedRuleSetActivation{}, ErrCustomerAuthorizationDenied
	}
	commandDigest := ruleSetActivationCommandDigest(
		request.ruleSet,
		request.correlationID,
	)
	return AuthorizedRuleSetActivation{
		scopeKind:           request.ruleSet.scope.kind,
		scopeID:             request.ruleSet.scope.id,
		revision:            request.ruleSet.version,
		ruleSetDigest:       request.ruleSet.digest,
		actorRef:            actorRef,
		commandDigest:       commandDigest,
		authorizationDigest: authorizationDigest,
		issuedAt:            issuedAt,
		expiresAt:           expiresAt,
		correlationID:       request.correlationID,
		requestID:           ruleSetActivationRequestID(request.ruleSet),
		seal:                authorizationSeal(commandDigest, authorizationDigest),
	}, nil
}

func NewAuthorizedApprovalDecision(
	request ApprovalDecisionRequest,
	actorRef, authorizationDigest string,
	issuedAt, expiresAt time.Time,
) (AuthorizedApprovalDecision, error) {
	if !validRuleText(request.approvalRequestID) ||
		!validSHA256(request.approvalRequestDigest) ||
		!validApprovalDecision(request.decision) ||
		!validRuleText(request.correlationID) ||
		!validRuleText(actorRef) ||
		!validSHA256(authorizationDigest) ||
		issuedAt.IsZero() || issuedAt.Location() != time.UTC ||
		expiresAt.IsZero() || expiresAt.Location() != time.UTC ||
		!issuedAt.Before(expiresAt) {
		return AuthorizedApprovalDecision{}, ErrCustomerAuthorizationDenied
	}
	commandDigest := approvalDecisionCommandDigest(
		request.approvalRequestID,
		request.approvalRequestDigest,
		request.decision,
		request.correlationID,
	)
	return AuthorizedApprovalDecision{
		approvalRequestID:     request.approvalRequestID,
		approvalRequestDigest: request.approvalRequestDigest,
		decision:              request.decision,
		actorRef:              actorRef,
		commandDigest:         commandDigest,
		authorizationDigest:   authorizationDigest,
		issuedAt:              issuedAt,
		expiresAt:             expiresAt,
		correlationID:         request.correlationID,
		requestID:             approvalDecisionRequestID(request),
		seal:                  authorizationSeal(commandDigest, authorizationDigest),
	}, nil
}

type CustomerAuthorizer interface {
	AuthorizeRuleSet(
		context.Context,
		RuleSetActivationRequest,
	) (AuthorizedRuleSetActivation, error)
	AuthorizeApprovalDecision(
		context.Context,
		ApprovalDecisionRequest,
	) (AuthorizedApprovalDecision, error)
}

type Authority struct {
	store      *journal.Store
	authorizer CustomerAuthorizer
	now        func() time.Time
}

func NewAuthority(
	store *journal.Store,
	authorizer CustomerAuthorizer,
	now func() time.Time,
) (*Authority, error) {
	if store == nil || now == nil {
		return nil, ErrInvalidRuleInput
	}
	if nilInterface(authorizer) {
		return nil, ErrCustomerAuthorizationRequired
	}
	return &Authority{store: store, authorizer: authorizer, now: now}, nil
}

type RuleSetRecord struct {
	ruleSet             RuleSet
	actorRef            string
	authorizationDigest string
	commandDigest       string
	correlationID       string
	eventID             string
}

func (record RuleSetRecord) Scope() Scope     { return record.ruleSet.scope }
func (record RuleSetRecord) Revision() int    { return record.ruleSet.version }
func (record RuleSetRecord) Digest() string   { return record.ruleSet.digest }
func (record RuleSetRecord) Rules() []Rule    { return record.ruleSet.Rules() }
func (record RuleSetRecord) ActorRef() string { return record.actorRef }
func (record RuleSetRecord) EventID() string  { return record.eventID }

func (authority *Authority) ActivateRuleSet(
	ctx context.Context,
	request RuleSetActivationRequest,
	correlationID string,
) (RuleSetRecord, error) {
	if ctx == nil || ctx.Err() != nil || !validRuleSet(request.ruleSet) ||
		!validCorrelationID(correlationID) {
		return RuleSetRecord{}, ErrInvalidRuleInput
	}
	request.correlationID = correlationID
	existing, found, err := authority.readRuleSet(ctx, request.ruleSet.StreamID())
	if err != nil {
		return RuleSetRecord{}, err
	}
	authorized, err := authority.authorizer.AuthorizeRuleSet(ctx, request)
	if err != nil {
		if errors.Is(err, ErrCustomerAuthorizationDenied) {
			return RuleSetRecord{}, ErrCustomerAuthorizationDenied
		}
		return RuleSetRecord{}, err
	}
	now, err := authority.operationTime()
	if err != nil {
		return RuleSetRecord{}, err
	}
	if found && existing.Revision() == request.ruleSet.version {
		if !validAuthorizedActivationBinding(authorized, request) {
			return RuleSetRecord{}, ErrCustomerAuthorizationDenied
		}
		if existing.Digest() == request.ruleSet.digest &&
			existing.actorRef == authorized.actorRef &&
			existing.authorizationDigest == authorized.authorizationDigest &&
			existing.commandDigest == authorized.commandDigest &&
			existing.correlationID == correlationID {
			return existing, nil
		}
		return RuleSetRecord{}, ErrRuleAuthorityConflict
	}
	if !validAuthorizedActivation(authorized, request, now) {
		return RuleSetRecord{}, ErrCustomerAuthorizationDenied
	}
	expectedRevision := 1
	expectedHead := int64(0)
	causationID := ""
	if found {
		expectedRevision = existing.Revision() + 1
		expectedHead = int64(existing.Revision())
		causationID = existing.EventID()
	}
	if request.ruleSet.version != expectedRevision {
		return RuleSetRecord{}, ErrRuleAuthorityConflict
	}
	payload := ruleSetEventPayloadFor(
		request.ruleSet,
		authorized.actorRef,
		authorized.authorizationDigest,
		authorized.commandDigest,
	)
	eventID := deterministicEventID(
		"RuleSetActivated",
		request.ruleSet.StreamID(),
		fmt.Sprint(request.ruleSet.version),
		request.ruleSet.digest,
		correlationID,
	)
	event := newRulesEvent(
		eventID,
		request.ruleSet.StreamID(),
		int64(request.ruleSet.version),
		"RuleSetActivated",
		now,
		correlationID,
		causationID,
		payload,
	)
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{
			StreamID: request.ruleSet.StreamID(),
			Sequence: expectedHead,
		}},
		[]journal.Event{event},
	); err != nil {
		return RuleSetRecord{}, mapRulesJournalError(err)
	}
	return RuleSetRecord{
		ruleSet: request.ruleSet, actorRef: authorized.actorRef,
		authorizationDigest: authorized.authorizationDigest,
		commandDigest:       authorized.commandDigest,
		correlationID:       correlationID,
		eventID:             eventID,
	}, nil
}

type ApprovalRequestInput struct {
	Context            ActionContext
	ContinuationDigest string
	Decision           Decision
	RequestedAt        time.Time
	CorrelationID      string
}

type ResumeCandidate struct {
	continuationDigest string
	heads              []journal.StreamHead
}

func (candidate ResumeCandidate) ContinuationDigest() string {
	return candidate.continuationDigest
}
func (candidate ResumeCandidate) Heads() []journal.StreamHead {
	return append([]journal.StreamHead(nil), candidate.heads...)
}

type ApprovalRequestRecord struct {
	id                    string
	digest                string
	context               ActionContext
	continuationDigest    string
	decision              Decision
	status                string
	previousStatus        string
	requestedAt           time.Time
	expiresAt             time.Time
	decisionActorRef      string
	authorizationDigest   string
	decidedAt             time.Time
	correlationID         string
	terminalCorrelationID string
	resumeCandidate       ResumeCandidate
	lastEventID           string
	streamSequence        int64
}

func (record ApprovalRequestRecord) ID() string                 { return record.id }
func (record ApprovalRequestRecord) Digest() string             { return record.digest }
func (record ApprovalRequestRecord) WorkItemID() string         { return record.context.workItemID }
func (record ApprovalRequestRecord) RunID() string              { return record.context.runID }
func (record ApprovalRequestRecord) ClaimGeneration() int64     { return record.context.claimGeneration }
func (record ApprovalRequestRecord) ContinuationDigest() string { return record.continuationDigest }
func (record ApprovalRequestRecord) Status() string             { return record.status }
func (record ApprovalRequestRecord) DecisionActorRef() string   { return record.decisionActorRef }
func (record ApprovalRequestRecord) ResumeCandidate() ResumeCandidate {
	return ResumeCandidate{
		continuationDigest: record.resumeCandidate.continuationDigest,
		heads:              record.resumeCandidate.Heads(),
	}
}

func (authority *Authority) RequestApproval(
	ctx context.Context,
	input ApprovalRequestInput,
) (ApprovalRequestRecord, error) {
	if ctx == nil || ctx.Err() != nil ||
		!validActionContext(input.Context) ||
		input.Context.action != "start_run" ||
		input.Context.claimID != "" ||
		input.Context.claimGeneration != 0 ||
		!validSHA256(input.ContinuationDigest) ||
		!validDecision(input.Decision) ||
		input.Decision.actionDigest != input.Context.digest ||
		!validCorrelationID(input.CorrelationID) {
		return ApprovalRequestRecord{}, ErrInvalidApprovalInput
	}
	if input.Decision.kind != "require_approval" {
		return ApprovalRequestRecord{}, ErrApprovalNotRequired
	}
	approvalID := approvalRequestID(
		input.Context.digest,
		input.ContinuationDigest,
		input.Decision.digest,
	)
	streamIDs := approvalRequestStreams(input.Context, approvalID)
	if len(streamIDs) > maxApprovalReadStreamHead {
		return ApprovalRequestRecord{}, ErrInvalidApprovalInput
	}
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return ApprovalRequestRecord{}, mapRulesJournalError(err)
	}
	if existing, found, replayErr := replayApprovalStream(
		approvalID,
		eventsForStream(snapshot.Events(), approvalStream(approvalID)),
	); replayErr != nil {
		return ApprovalRequestRecord{}, replayErr
	} else if found {
		if existing.context.digest == input.Context.digest &&
			existing.continuationDigest == input.ContinuationDigest &&
			existing.decision.digest == input.Decision.digest &&
			existing.requestedAt.Equal(input.RequestedAt) &&
			existing.correlationID == input.CorrelationID {
			if existing.status == "approved" {
				candidate, err := authority.rebuildApprovedResumeCandidate(
					ctx,
					existing,
				)
				if err != nil {
					return ApprovalRequestRecord{}, err
				}
				existing.resumeCandidate = candidate
			}
			return existing, nil
		}
		return ApprovalRequestRecord{}, ErrApprovalAlreadyPending
	}
	now, err := authority.operationTime()
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	if !input.RequestedAt.Equal(now) {
		return ApprovalRequestRecord{}, ErrInvalidApprovalInput
	}
	workState, err := replayApprovalWorkItem(
		input.Context.workItemID,
		eventsForStream(snapshot.Events(), workItemStream(input.Context.workItemID)),
	)
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	if workState.status != "assigned" ||
		workState.runID != input.Context.runID ||
		workState.agentInstanceID != input.Context.agentInstanceID {
		return ApprovalRequestRecord{}, ErrApprovalStale
	}
	runHead, _ := snapshot.Head(runStream(input.Context.runID))
	if runHead.Sequence != 0 || runHead.EventID != "" {
		return ApprovalRequestRecord{}, ErrApprovalRunAlreadyClaimed
	}
	ruleSets, err := ruleSetsForDecision(
		snapshot,
		input.Context,
		input.Decision,
	)
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	recomputed, err := Evaluate(ruleSets, input.Context)
	if err != nil || !equalDecision(recomputed, input.Decision) {
		return ApprovalRequestRecord{}, ErrApprovalStale
	}
	expiresAt := now.Add(input.Decision.timeout)
	record := ApprovalRequestRecord{
		id: approvalID, context: input.Context,
		continuationDigest: input.ContinuationDigest,
		decision:           input.Decision, status: "pending",
		previousStatus: workState.status, requestedAt: now,
		expiresAt: expiresAt, correlationID: input.CorrelationID,
	}
	record.digest = digestApprovalRecord(record)
	approvalEventID := deterministicEventID(
		"ApprovalRequested",
		approvalID,
		record.digest,
		input.CorrelationID,
	)
	workEventID := deterministicEventID(
		"WorkItemApprovalPaused",
		input.Context.workItemID,
		approvalID,
		approvalEventID,
	)
	approvalEvent := newRulesEvent(
		approvalEventID,
		approvalStream(approvalID),
		1,
		"ApprovalRequested",
		now,
		input.CorrelationID,
		workState.lastEventID,
		approvalRequestedPayloadFor(record),
	)
	workEvent := newRulesEvent(
		workEventID,
		workItemStream(input.Context.workItemID),
		workState.sequence+1,
		"WorkItemApprovalPaused",
		now,
		input.CorrelationID,
		approvalEventID,
		workItemApprovalPayload{
			WorkItemID:            input.Context.workItemID,
			ApprovalRequestID:     approvalID,
			ApprovalRequestDigest: record.digest,
			PreviousStatus:        workState.status,
			Status:                "waiting_approval",
		},
	)
	expectations := expectationsFor(snapshot, streamIDs)
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		expectations,
		[]journal.Event{approvalEvent, workEvent},
	); err != nil {
		return ApprovalRequestRecord{}, mapRulesJournalError(err)
	}
	record.lastEventID = approvalEventID
	record.streamSequence = 1
	return record, nil
}

func (authority *Authority) DecideApproval(
	ctx context.Context,
	request ApprovalDecisionRequest,
	correlationID string,
) (ApprovalRequestRecord, error) {
	if ctx == nil || ctx.Err() != nil ||
		!validRuleText(request.approvalRequestID) ||
		!validSHA256(request.approvalRequestDigest) ||
		!validApprovalDecision(request.decision) ||
		!validCorrelationID(correlationID) {
		return ApprovalRequestRecord{}, ErrInvalidApprovalInput
	}
	request.correlationID = correlationID
	approvalEvents, err := authority.store.ReadStream(
		ctx,
		approvalStream(request.approvalRequestID),
	)
	if err != nil {
		return ApprovalRequestRecord{}, mapRulesJournalError(err)
	}
	existing, found, err := replayApprovalStream(
		request.approvalRequestID,
		approvalEvents,
	)
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	if !found || existing.digest != request.approvalRequestDigest {
		return ApprovalRequestRecord{}, ErrApprovalNotPending
	}
	if existing.status != "pending" &&
		existing.status != request.decision {
		return ApprovalRequestRecord{}, ErrApprovalAlreadyTerminal
	}
	authorized, err := authority.authorizer.AuthorizeApprovalDecision(ctx, request)
	if err != nil {
		if errors.Is(err, ErrCustomerAuthorizationDenied) {
			return ApprovalRequestRecord{}, ErrCustomerAuthorizationDenied
		}
		return ApprovalRequestRecord{}, err
	}
	now, err := authority.operationTime()
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	if existing.status == request.decision {
		if !validAuthorizedDecisionBinding(authorized, request) {
			return ApprovalRequestRecord{}, ErrCustomerAuthorizationDenied
		}
		if request.decision == "approved" &&
			!contains(existing.decision.approverRefs, authorized.actorRef) {
			return ApprovalRequestRecord{}, ErrCustomerAuthorizationDenied
		}
		if existing.decisionActorRef != authorized.actorRef ||
			existing.authorizationDigest != authorized.authorizationDigest ||
			existing.terminalCorrelationID != correlationID {
			return ApprovalRequestRecord{}, ErrRuleAuthorityConflict
		}
		if existing.status == "approved" {
			candidate, err := authority.rebuildApprovedResumeCandidate(
				ctx,
				existing,
			)
			if err != nil {
				return ApprovalRequestRecord{}, err
			}
			existing.resumeCandidate = candidate
		}
		return existing, nil
	}
	if !validAuthorizedDecision(authorized, request, now) {
		return ApprovalRequestRecord{}, ErrCustomerAuthorizationDenied
	}
	if request.decision == "approved" &&
		!contains(existing.decision.approverRefs, authorized.actorRef) {
		return ApprovalRequestRecord{}, ErrCustomerAuthorizationDenied
	}
	return authority.commitApprovalResolution(
		ctx,
		existing,
		request.decision,
		authorized.actorRef,
		authorized.authorizationDigest,
		now,
		correlationID,
	)
}

func (authority *Authority) ExpireApproval(
	ctx context.Context,
	approvalRequestID, correlationID string,
) (ApprovalRequestRecord, error) {
	if ctx == nil || ctx.Err() != nil ||
		!validRuleText(approvalRequestID) ||
		!validCorrelationID(correlationID) {
		return ApprovalRequestRecord{}, ErrInvalidApprovalInput
	}
	events, err := authority.store.ReadStream(
		ctx,
		approvalStream(approvalRequestID),
	)
	if err != nil {
		return ApprovalRequestRecord{}, mapRulesJournalError(err)
	}
	record, found, err := replayApprovalStream(approvalRequestID, events)
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	if !found {
		return ApprovalRequestRecord{}, ErrApprovalNotPending
	}
	if record.status != "pending" {
		if record.status == "expired" &&
			record.terminalCorrelationID == correlationID {
			return record, nil
		}
		if record.status == "expired" {
			return ApprovalRequestRecord{}, ErrRuleAuthorityConflict
		}
		return ApprovalRequestRecord{}, ErrApprovalAlreadyTerminal
	}
	now, err := authority.operationTime()
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	if now.Before(record.expiresAt) {
		return ApprovalRequestRecord{}, ErrApprovalExpired
	}
	return authority.commitApprovalResolution(
		ctx,
		record,
		"expired",
		"",
		"",
		now,
		correlationID,
	)
}

// PermissionApprovalInput is the permission-layer ask. It reuses the full
// customer approval lifecycle (RuleSetActivated -> RequestApproval ->
// ApprovalRequested/WorkItemApprovalPaused -> ApprovalDecided/
// WorkItemApprovalResolved) by representing the ask as a temporary
// require_approval RuleSet scoped to the Queue Job's work item. The
// permission layer has already decided "ask"; the approval decision remains a
// human gate through the daemon's permission authorizer.
type PermissionApprovalInput struct {
	JobID         string
	CallDigest    string
	Tool          string
	Command       string
	Path          string
	Reason        string
	RequestedAt   time.Time
	CorrelationID string
}

const (
	permissionApprovalActor   = "permission"
	permissionApproverRef     = "approver:permission-owner"
	permissionApprovalTimeout = 24 * time.Hour
)

func (authority *Authority) RequestPermissionApproval(
	ctx context.Context,
	input PermissionApprovalInput,
) (ApprovalRequestRecord, error) {
	if ctx == nil || ctx.Err() != nil ||
		!validRuleText(input.JobID) ||
		!validSHA256(input.CallDigest) ||
		!validRuleText(input.Tool) ||
		input.RequestedAt.IsZero() ||
		!validCorrelationID(input.CorrelationID) {
		return ApprovalRequestRecord{}, ErrInvalidApprovalInput
	}
	action, err := NewActionContext(ActionContextInput{
		ProjectID: permissionApprovalActor, TeamInstanceID: permissionApprovalActor,
		WorkPackageID: permissionApprovalActor, WorkItemID: input.JobID,
		RunID:           "run-permission-" + input.CallDigest[:16],
		AgentInstanceID: permissionApprovalActor, LogicalNodeID: permissionApprovalActor,
		AttemptNumber: 1, Action: "start_run", Risk: "medium",
		ContractDigest: input.CallDigest,
	})
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	condition, err := NewCondition("start_run", "medium")
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	effect, err := NewEffect(
		"require_approval", "",
		[]string{permissionApproverRef},
		permissionApprovalTimeout,
		"reject",
	)
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	rule, err := NewRule("permission-ask", condition, effect)
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	scope, err := NewScope("work_item", input.JobID)
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	ruleSet, err := NewRuleSet(scope, 1, []Rule{rule})
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	decision, err := Evaluate([]RuleSet{ruleSet}, action)
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	if decision.Kind() != "require_approval" {
		return ApprovalRequestRecord{}, ErrApprovalNotRequired
	}
	approvalID := approvalRequestID(
		action.digest,
		input.CallDigest,
		decision.digest,
	)
	// Idempotency (RED A4-2/A4-6): an existing approval for the same ask is
	// returned before any work-state gate so restart/replay is safe.
	existingEvents, err := authority.store.ReadStream(ctx, approvalStream(approvalID))
	if err != nil {
		return ApprovalRequestRecord{}, mapRulesJournalError(err)
	}
	if existing, found, replayErr := replayApprovalStream(
		approvalID,
		existingEvents,
	); replayErr != nil {
		return ApprovalRequestRecord{}, replayErr
	} else if found {
		if existing.context.digest == action.digest &&
			existing.continuationDigest == input.CallDigest &&
			existing.decision.digest == decision.digest {
			return existing, nil
		}
		return ApprovalRequestRecord{}, ErrApprovalAlreadyPending
	}
	workStreamID := workItemStream(input.JobID)
	workEvents, err := authority.store.ReadStream(ctx, workStreamID)
	if err != nil {
		return ApprovalRequestRecord{}, mapRulesJournalError(err)
	}
	workState, workErr := replayApprovalWorkItem(input.JobID, workEvents)
	workFound := workErr == nil && workState.workItemID != ""
	workCausation := ""
	switch {
	case len(workEvents) == 0:
		// Bootstrap the rules work-item record for the Queue Job so the
		// approval state machine and projections see a complete lifecycle.
		createdID := deterministicEventID(
			"WorkItemCreated", input.JobID, "ready", input.CorrelationID,
		)
		assignedID := deterministicEventID(
			"WorkItemAssigned", input.JobID, action.runID, input.CorrelationID,
		)
		workCausation = assignedID
		now := input.RequestedAt.UTC()
		workPrefix := []journal.Event{
			newRulesEvent(createdID, workStreamID, 1, "WorkItemCreated", now,
				input.CorrelationID, "",
				struct {
					WorkItemID string `json:"work_item_id"`
					Title      string `json:"title"`
					Status     string `json:"status"`
				}{WorkItemID: input.JobID, Title: "permission-ask:" + input.Tool, Status: "ready"}),
			newRulesEvent(assignedID, workStreamID, 2, "WorkItemAssigned", now,
				input.CorrelationID, createdID,
				struct {
					WorkItemID      string `json:"work_item_id"`
					RunID           string `json:"run_id"`
					AgentInstanceID string `json:"agent_instance_id"`
					Status          string `json:"status"`
				}{WorkItemID: input.JobID, RunID: action.runID,
					AgentInstanceID: permissionApprovalActor, Status: "assigned"}),
		}
		if _, err := authority.store.AppendBatchIfStreamHeads(
			ctx,
			[]journal.StreamHeadExpectation{{StreamID: workStreamID, Sequence: 0}},
			workPrefix,
		); err != nil {
			return ApprovalRequestRecord{}, mapRulesJournalError(err)
		}
	case workErr != nil || !workFound:
		return ApprovalRequestRecord{}, ErrApprovalStale
	case workState.status == "waiting_approval":
		return ApprovalRequestRecord{}, ErrApprovalAlreadyPending
	case workState.status != "assigned":
		return ApprovalRequestRecord{}, ErrApprovalStale
	default:
		workCausation = workState.lastEventID
	}
	activation, err := NewRuleSetActivationRequest(
		ruleSet,
		[]byte("permission-ask:"+input.JobID+":"+input.CallDigest),
	)
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	if _, err := authority.ActivateRuleSet(ctx, activation, input.CorrelationID); err != nil {
		return ApprovalRequestRecord{}, err
	}
	now, err := authority.operationTime()
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	record := ApprovalRequestRecord{
		id: approvalID, context: action,
		continuationDigest: input.CallDigest,
		decision:           decision, status: "pending",
		previousStatus: "assigned",
		requestedAt:    now, expiresAt: now.Add(decision.timeout),
		correlationID: input.CorrelationID,
	}
	record.digest = digestApprovalRecord(record)
	approvalEventID := deterministicEventID(
		"ApprovalRequested", approvalID, record.digest, input.CorrelationID,
	)
	workEventID := deterministicEventID(
		"WorkItemApprovalPaused", input.JobID, approvalID, approvalEventID,
	)
	workHead, err := authority.currentWorkHead(ctx, workStreamID)
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	approvalEvent := newRulesEvent(
		approvalEventID, approvalStream(approvalID), 1, "ApprovalRequested",
		now, input.CorrelationID, workCausation, approvalRequestedPayloadFor(record),
	)
	workEvent := newRulesEvent(
		workEventID, workStreamID, workHead+1, "WorkItemApprovalPaused",
		now, input.CorrelationID, approvalEventID,
		workItemApprovalPayload{
			WorkItemID: input.JobID, ApprovalRequestID: approvalID,
			ApprovalRequestDigest: record.digest,
			PreviousStatus:        "assigned", Status: "waiting_approval",
		},
	)
	snapshot, err := authority.store.ReadStreamSet(
		ctx,
		[]string{approvalStream(approvalID), workStreamID},
	)
	if err != nil {
		return ApprovalRequestRecord{}, mapRulesJournalError(err)
	}
	streamIDs := []string{approvalStream(approvalID), workStreamID}
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		expectationsFor(snapshot, streamIDs),
		[]journal.Event{approvalEvent, workEvent},
	); err != nil {
		return ApprovalRequestRecord{}, mapRulesJournalError(err)
	}
	record.lastEventID = approvalEventID
	record.streamSequence = 1
	return record, nil
}

func (authority *Authority) currentWorkHead(
	ctx context.Context,
	streamID string,
) (int64, error) {
	events, err := authority.store.ReadStream(ctx, streamID)
	if err != nil {
		return 0, mapRulesJournalError(err)
	}
	var head int64
	for _, event := range events {
		if event.Seq > head {
			head = event.Seq
		}
	}
	return head, nil
}

func (authority *Authority) DecidePermissionApproval(
	ctx context.Context,
	approvalID, approvalDigest, decision, resolvedBy, correlationID string,
) (ApprovalRequestRecord, error) {
	if ctx == nil || ctx.Err() != nil ||
		!validRuleText(approvalID) ||
		!validSHA256(approvalDigest) ||
		(decision != "approved" && decision != "rejected") ||
		!validRuleText(resolvedBy) ||
		!validCorrelationID(correlationID) {
		return ApprovalRequestRecord{}, ErrInvalidApprovalInput
	}
	request, err := NewApprovalDecisionRequest(
		approvalID,
		approvalDigest,
		decision,
		[]byte("permission-decision:"+resolvedBy),
	)
	if err != nil {
		return ApprovalRequestRecord{}, err
	}
	return authority.DecideApproval(ctx, request, correlationID)
}

func (authority *Authority) commitApprovalResolution(
	ctx context.Context,
	record ApprovalRequestRecord,
	status, actorRef, authorizationDigest string,
	now time.Time,
	correlationID string,
) (ApprovalRequestRecord, error) {
	streamIDs := approvalRequestStreams(record.context, record.id)
	if len(streamIDs) > maxApprovalReadStreamHead {
		return ApprovalRequestRecord{}, ErrInvalidApprovalInput
	}
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return ApprovalRequestRecord{}, mapRulesJournalError(err)
	}
	current, found, err := replayApprovalStream(
		record.id,
		eventsForStream(snapshot.Events(), approvalStream(record.id)),
	)
	if err != nil || !found || current.status != "pending" ||
		current.digest != record.digest {
		if err != nil {
			return ApprovalRequestRecord{}, err
		}
		return ApprovalRequestRecord{}, ErrApprovalStale
	}
	workState, err := replayApprovalWorkItem(
		record.context.workItemID,
		eventsForStream(snapshot.Events(), workItemStream(record.context.workItemID)),
	)
	if err != nil || workState.status != "waiting_approval" ||
		workState.approvalRequestID != record.id {
		return ApprovalRequestRecord{}, ErrApprovalStale
	}
	runHead, _ := snapshot.Head(runStream(record.context.runID))
	if runHead.Sequence != 0 || runHead.EventID != "" {
		return ApprovalRequestRecord{}, ErrApprovalRunAlreadyClaimed
	}
	if _, err := ruleSetsForDecision(
		snapshot,
		record.context,
		record.decision,
	); err != nil {
		return ApprovalRequestRecord{}, err
	}
	nextWorkStatus := "blocked"
	if status == "approved" {
		nextWorkStatus = "assigned"
	} else if status == "cancelled" ||
		(status == "expired" && record.decision.onTimeout == "cancel") {
		nextWorkStatus = "cancelled"
	}
	terminalEventType := "ApprovalDecided"
	if status == "expired" {
		terminalEventType = "ApprovalExpired"
	}
	terminalID := deterministicEventID(
		terminalEventType,
		record.id,
		status,
		actorRef,
		correlationID,
	)
	workID := deterministicEventID(
		"WorkItemApprovalResolved",
		record.context.workItemID,
		record.id,
		status,
		terminalID,
	)
	terminalEvent := newRulesEvent(
		terminalID,
		approvalStream(record.id),
		current.streamSequence+1,
		terminalEventType,
		now,
		correlationID,
		current.lastEventID,
		approvalResolutionPayload{
			ApprovalRequestID:     record.id,
			ApprovalRequestDigest: record.digest,
			Status:                status,
			ActorRef:              actorRef,
			AuthorizationDigest:   authorizationDigest,
			DecidedAt:             now.Format(time.RFC3339Nano),
		},
	)
	workEvent := newRulesEvent(
		workID,
		workItemStream(record.context.workItemID),
		workState.sequence+1,
		"WorkItemApprovalResolved",
		now,
		correlationID,
		terminalID,
		workItemApprovalPayload{
			WorkItemID:            record.context.workItemID,
			ApprovalRequestID:     record.id,
			ApprovalRequestDigest: record.digest,
			PreviousStatus:        "waiting_approval",
			Status:                nextWorkStatus,
		},
	)
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		expectationsFor(snapshot, streamIDs),
		[]journal.Event{terminalEvent, workEvent},
	); err != nil {
		return ApprovalRequestRecord{}, mapRulesJournalError(err)
	}
	record.status = status
	record.decisionActorRef = actorRef
	record.authorizationDigest = authorizationDigest
	record.decidedAt = now
	record.terminalCorrelationID = correlationID
	record.lastEventID = terminalID
	record.streamSequence++
	if status == "approved" {
		record.resumeCandidate = ResumeCandidate{
			continuationDigest: record.continuationDigest,
			heads:              headsAfterResolution(snapshot, workEvent, terminalEvent),
		}
	}
	return record, nil
}

func (authority *Authority) rebuildApprovedResumeCandidate(
	ctx context.Context,
	record ApprovalRequestRecord,
) (ResumeCandidate, error) {
	if record.status != "approved" {
		return ResumeCandidate{}, ErrApprovalAlreadyTerminal
	}
	streamIDs := approvalRequestStreams(record.context, record.id)
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return ResumeCandidate{}, mapRulesJournalError(err)
	}
	replayed, found, err := replayApprovalStream(
		record.id,
		eventsForStream(snapshot.Events(), approvalStream(record.id)),
	)
	if err != nil || !found || replayed.status != "approved" ||
		replayed.digest != record.digest ||
		replayed.lastEventID != record.lastEventID {
		if err != nil {
			return ResumeCandidate{}, err
		}
		return ResumeCandidate{}, ErrApprovalStale
	}
	var resolvedHead journal.StreamHead
	for _, event := range eventsForStream(
		snapshot.Events(),
		workItemStream(record.context.workItemID),
	) {
		if event.Type != "WorkItemApprovalResolved" ||
			event.CausationID != record.lastEventID {
			continue
		}
		var payload workItemApprovalPayload
		if decodeExact(event.PayloadJSON, &payload) != nil ||
			payload.WorkItemID != record.context.workItemID ||
			payload.ApprovalRequestID != record.id ||
			payload.ApprovalRequestDigest != record.digest ||
			payload.PreviousStatus != "waiting_approval" ||
			payload.Status != "assigned" ||
			resolvedHead.Sequence != 0 {
			return ResumeCandidate{}, ErrApprovalStale
		}
		resolvedHead = journal.StreamHead{
			StreamID: event.StreamID,
			Sequence: event.Seq,
			EventID:  event.ID,
		}
	}
	if resolvedHead.Sequence == 0 {
		return ResumeCandidate{}, ErrApprovalStale
	}
	references := make(map[string]RuleSetReference)
	for _, reference := range record.decision.ruleSetRefs {
		references[reference.StreamID] = reference
	}
	heads := make([]journal.StreamHead, 0, len(streamIDs))
	for _, streamID := range streamIDs {
		switch streamID {
		case approvalStream(record.id):
			heads = append(heads, journal.StreamHead{
				StreamID: streamID,
				Sequence: record.streamSequence,
				EventID:  record.lastEventID,
			})
		case workItemStream(record.context.workItemID):
			heads = append(heads, resolvedHead)
		case runStream(record.context.runID):
			heads = append(heads, journal.StreamHead{StreamID: streamID})
		default:
			reference, referenced := references[streamID]
			if !referenced {
				heads = append(
					heads,
					journal.StreamHead{StreamID: streamID},
				)
				continue
			}
			streamEvents := eventsForStream(snapshot.Events(), streamID)
			if reference.Version <= 0 ||
				len(streamEvents) < reference.Version {
				return ResumeCandidate{}, ErrApprovalStale
			}
			recordAtVersion, found, replayErr := replayRuleSetStream(
				streamID,
				streamEvents[:reference.Version],
			)
			if replayErr != nil || !found ||
				recordAtVersion.Digest() != reference.Digest {
				return ResumeCandidate{}, ErrApprovalStale
			}
			event := streamEvents[reference.Version-1]
			heads = append(heads, journal.StreamHead{
				StreamID: streamID,
				Sequence: event.Seq,
				EventID:  event.ID,
			})
		}
	}
	return ResumeCandidate{
		continuationDigest: record.continuationDigest,
		heads:              heads,
	}, nil
}

func (authority *Authority) readRuleSet(
	ctx context.Context,
	streamID string,
) (RuleSetRecord, bool, error) {
	events, err := authority.store.ReadStream(ctx, streamID)
	if err != nil {
		return RuleSetRecord{}, false, mapRulesJournalError(err)
	}
	return replayRuleSetStream(streamID, events)
}

func (authority *Authority) operationTime() (time.Time, error) {
	now := authority.now()
	if now.IsZero() || now.Location() != time.UTC {
		return time.Time{}, ErrInvalidRuleInput
	}
	return now, nil
}

type scopePayload struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type conditionPayload struct {
	Action string `json:"action"`
	Risk   string `json:"risk"`
}

type effectPayload struct {
	Kind         string   `json:"kind"`
	Marker       string   `json:"marker"`
	ApproverRefs []string `json:"approver_refs"`
	TimeoutNanos int64    `json:"timeout_nanos"`
	OnTimeout    string   `json:"on_timeout"`
}

type rulePayload struct {
	ID        string           `json:"id"`
	Condition conditionPayload `json:"condition"`
	Effect    effectPayload    `json:"effect"`
}

type ruleSetEventPayload struct {
	Scope               scopePayload  `json:"scope"`
	Revision            int           `json:"revision"`
	Digest              string        `json:"digest"`
	Rules               []rulePayload `json:"rules"`
	ActorRef            string        `json:"actor_ref"`
	AuthorizationDigest string        `json:"authorization_digest"`
	CommandDigest       string        `json:"command_digest"`
}

type actionContextPayload struct {
	ProjectID       string `json:"project_id"`
	TeamInstanceID  string `json:"team_instance_id"`
	WorkPackageID   string `json:"work_package_id"`
	WorkItemID      string `json:"work_item_id"`
	RunID           string `json:"run_id"`
	AgentInstanceID string `json:"agent_instance_id"`
	LogicalNodeID   string `json:"logical_node_id"`
	AttemptNumber   int    `json:"attempt_number"`
	Action          string `json:"action"`
	Risk            string `json:"risk"`
	ClaimID         string `json:"claim_id"`
	ClaimGeneration int64  `json:"claim_generation"`
	ContractDigest  string `json:"contract_digest"`
	Digest          string `json:"digest"`
}

type decisionPayload struct {
	Kind           string             `json:"kind"`
	Digest         string             `json:"digest"`
	ActionDigest   string             `json:"action_digest"`
	RuleSetRefs    []RuleSetReference `json:"rule_set_refs"`
	ApproverRefs   []string           `json:"approver_refs"`
	TimeoutNanos   int64              `json:"timeout_nanos"`
	OnTimeout      string             `json:"on_timeout"`
	WarningMarkers []string           `json:"warning_markers"`
	RecordMarkers  []string           `json:"record_markers"`
}

type approvalRequestedPayload struct {
	ApprovalRequestID     string               `json:"approval_request_id"`
	ApprovalRequestDigest string               `json:"approval_request_digest"`
	Context               actionContextPayload `json:"context"`
	ContinuationDigest    string               `json:"continuation_digest"`
	Decision              decisionPayload      `json:"decision"`
	PreviousStatus        string               `json:"previous_status"`
	RequestedAt           string               `json:"requested_at"`
	ExpiresAt             string               `json:"expires_at"`
}

type approvalResolutionPayload struct {
	ApprovalRequestID     string `json:"approval_request_id"`
	ApprovalRequestDigest string `json:"approval_request_digest"`
	Status                string `json:"status"`
	ActorRef              string `json:"actor_ref"`
	AuthorizationDigest   string `json:"authorization_digest"`
	DecidedAt             string `json:"decided_at"`
}

type workItemApprovalPayload struct {
	WorkItemID            string `json:"work_item_id"`
	ApprovalRequestID     string `json:"approval_request_id"`
	ApprovalRequestDigest string `json:"approval_request_digest"`
	PreviousStatus        string `json:"previous_status"`
	Status                string `json:"status"`
}

type approvalWorkState struct {
	workItemID        string
	runID             string
	agentInstanceID   string
	status            string
	approvalRequestID string
	lastEventID       string
	sequence          int64
}

func replayRuleSetStream(
	streamID string,
	events []journal.Event,
) (RuleSetRecord, bool, error) {
	var record RuleSetRecord
	for index, event := range events {
		if event.StreamID != streamID || event.Seq != int64(index+1) ||
			event.SchemaVersion != 1 || event.Type != "RuleSetActivated" {
			return RuleSetRecord{}, false, ErrRuleAuthorityConflict
		}
		var payload ruleSetEventPayload
		if decodeExact(event.PayloadJSON, &payload) != nil ||
			payload.Revision != index+1 {
			return RuleSetRecord{}, false, ErrRuleAuthorityConflict
		}
		ruleSet, err := ruleSetFromPayload(payload)
		if err != nil || ruleSet.StreamID() != streamID ||
			event.CausationID != record.eventID ||
			!validCorrelationID(event.CorrelationID) ||
			payload.CommandDigest != ruleSetActivationCommandDigest(
				ruleSet,
				event.CorrelationID,
			) {
			return RuleSetRecord{}, false, ErrRuleAuthorityConflict
		}
		record = RuleSetRecord{
			ruleSet: ruleSet, actorRef: payload.ActorRef,
			authorizationDigest: payload.AuthorizationDigest,
			commandDigest:       payload.CommandDigest,
			correlationID:       event.CorrelationID,
			eventID:             event.ID,
		}
	}
	return record, len(events) > 0, nil
}

func replayApprovalStream(
	approvalID string,
	events []journal.Event,
) (ApprovalRequestRecord, bool, error) {
	if len(events) == 0 {
		return ApprovalRequestRecord{}, false, nil
	}
	if len(events) > 2 {
		return ApprovalRequestRecord{}, false, ErrRuleAuthorityConflict
	}
	first := events[0]
	if first.StreamID != approvalStream(approvalID) || first.Seq != 1 ||
		first.SchemaVersion != 1 || first.Type != "ApprovalRequested" ||
		!validCorrelationID(first.CorrelationID) {
		return ApprovalRequestRecord{}, false, ErrRuleAuthorityConflict
	}
	var requested approvalRequestedPayload
	if decodeExact(first.PayloadJSON, &requested) != nil ||
		requested.ApprovalRequestID != approvalID {
		return ApprovalRequestRecord{}, false, ErrRuleAuthorityConflict
	}
	record, err := approvalRecordFromPayload(requested)
	if err != nil {
		return ApprovalRequestRecord{}, false, err
	}
	record.lastEventID = first.ID
	record.streamSequence = 1
	record.correlationID = first.CorrelationID
	if len(events) == 1 {
		return record, true, nil
	}
	second := events[1]
	if second.StreamID != first.StreamID || second.Seq != 2 ||
		second.SchemaVersion != 1 ||
		(second.Type != "ApprovalDecided" && second.Type != "ApprovalExpired") ||
		second.CausationID != first.ID ||
		!validCorrelationID(second.CorrelationID) {
		return ApprovalRequestRecord{}, false, ErrRuleAuthorityConflict
	}
	var resolution approvalResolutionPayload
	if decodeExact(second.PayloadJSON, &resolution) != nil ||
		resolution.ApprovalRequestID != approvalID ||
		resolution.ApprovalRequestDigest != record.digest ||
		!validApprovalResolution(
			second.Type,
			resolution.Status,
			resolution.ActorRef,
			resolution.AuthorizationDigest,
		) {
		return ApprovalRequestRecord{}, false, ErrRuleAuthorityConflict
	}
	decidedAt, err := time.Parse(time.RFC3339Nano, resolution.DecidedAt)
	if err != nil {
		return ApprovalRequestRecord{}, false, ErrRuleAuthorityConflict
	}
	record.status = resolution.Status
	record.decisionActorRef = resolution.ActorRef
	record.authorizationDigest = resolution.AuthorizationDigest
	record.decidedAt = decidedAt
	record.terminalCorrelationID = second.CorrelationID
	record.lastEventID = second.ID
	record.streamSequence = 2
	if record.status == "approved" {
		record.resumeCandidate.continuationDigest = record.continuationDigest
	}
	return record, true, nil
}

func replayApprovalWorkItem(
	workItemID string,
	events []journal.Event,
) (approvalWorkState, error) {
	var state approvalWorkState
	for _, event := range events {
		if event.SchemaVersion != 1 ||
			event.StreamID != workItemStream(workItemID) {
			return approvalWorkState{}, ErrRuleAuthorityConflict
		}
		switch event.Type {
		case "WorkItemCreated":
			var payload struct {
				WorkItemID string `json:"work_item_id"`
				Title      string `json:"title"`
				Status     string `json:"status"`
			}
			if decodeExact(event.PayloadJSON, &payload) != nil ||
				payload.WorkItemID != workItemID ||
				payload.Status != "ready" ||
				state.workItemID != "" {
				return approvalWorkState{}, ErrRuleAuthorityConflict
			}
			state.workItemID = workItemID
			state.status = "ready"
		case "WorkItemAssigned":
			var payload struct {
				WorkItemID      string `json:"work_item_id"`
				RunID           string `json:"run_id"`
				AgentInstanceID string `json:"agent_instance_id"`
				Status          string `json:"status"`
			}
			if decodeExact(event.PayloadJSON, &payload) != nil ||
				state.status != "ready" ||
				payload.WorkItemID != workItemID ||
				payload.Status != "assigned" {
				return approvalWorkState{}, ErrRuleAuthorityConflict
			}
			state.runID = payload.RunID
			state.agentInstanceID = payload.AgentInstanceID
			state.status = "assigned"
		case "WorkItemApprovalPaused":
			var payload workItemApprovalPayload
			if decodeExact(event.PayloadJSON, &payload) != nil ||
				state.status != "assigned" ||
				payload.WorkItemID != workItemID ||
				payload.PreviousStatus != "assigned" ||
				payload.Status != "waiting_approval" ||
				event.CausationID == "" {
				return approvalWorkState{}, ErrRuleAuthorityConflict
			}
			state.status = "waiting_approval"
			state.approvalRequestID = payload.ApprovalRequestID
		case "WorkItemApprovalResolved":
			var payload workItemApprovalPayload
			if decodeExact(event.PayloadJSON, &payload) != nil ||
				state.status != "waiting_approval" ||
				payload.WorkItemID != workItemID ||
				payload.ApprovalRequestID != state.approvalRequestID ||
				payload.PreviousStatus != "waiting_approval" ||
				(payload.Status != "assigned" &&
					payload.Status != "blocked" &&
					payload.Status != "cancelled") ||
				event.CausationID == "" {
				return approvalWorkState{}, ErrRuleAuthorityConflict
			}
			state.status = payload.Status
			state.approvalRequestID = ""
		default:
			if event.Type == "WorkItemReadyForReview" ||
				event.Type == "WorkItemTerminal" {
				return approvalWorkState{}, ErrApprovalRunAlreadyClaimed
			}
			return approvalWorkState{}, ErrRuleAuthorityConflict
		}
		state.lastEventID = event.ID
		state.sequence = event.Seq
	}
	if state.workItemID == "" {
		return approvalWorkState{}, ErrApprovalStale
	}
	return state, nil
}

func ruleSetsForDecision(
	snapshot journal.StreamSetSnapshot,
	action ActionContext,
	decision Decision,
) ([]RuleSet, error) {
	streamIDs := ruleSetStreamsForAction(action)
	if len(streamIDs) != maxRuleSets {
		return nil, ErrApprovalStale
	}
	ruleSets := make([]RuleSet, 0, len(streamIDs))
	for _, streamID := range streamIDs {
		record, found, err := replayRuleSetStream(
			streamID,
			eventsForStream(snapshot.Events(), streamID),
		)
		if err != nil {
			return nil, ErrApprovalStale
		}
		if found {
			ruleSets = append(ruleSets, record.ruleSet)
		}
	}
	recomputed, err := Evaluate(ruleSets, action)
	if err != nil || !equalDecision(recomputed, decision) {
		return nil, ErrApprovalStale
	}
	return ruleSets, nil
}

func ruleSetEventPayloadFor(
	ruleSet RuleSet,
	actorRef, authorizationDigest, commandDigest string,
) ruleSetEventPayload {
	rules := make([]rulePayload, len(ruleSet.rules))
	for index, rule := range ruleSet.rules {
		rules[index] = ruleToPayload(rule)
	}
	return ruleSetEventPayload{
		Scope:    scopePayload{Kind: ruleSet.scope.kind, ID: ruleSet.scope.id},
		Revision: ruleSet.version, Digest: ruleSet.digest, Rules: rules,
		ActorRef: actorRef, AuthorizationDigest: authorizationDigest,
		CommandDigest: commandDigest,
	}
}

func ruleSetFromPayload(payload ruleSetEventPayload) (RuleSet, error) {
	scope, err := NewScope(payload.Scope.Kind, payload.Scope.ID)
	if err != nil || !validSHA256(payload.Digest) ||
		!validRuleText(payload.ActorRef) ||
		!validSHA256(payload.AuthorizationDigest) ||
		!validSHA256(payload.CommandDigest) {
		return RuleSet{}, ErrRuleAuthorityConflict
	}
	rules := make([]Rule, len(payload.Rules))
	for index, item := range payload.Rules {
		condition, conditionErr := NewCondition(
			item.Condition.Action,
			item.Condition.Risk,
		)
		effect, effectErr := NewEffect(
			item.Effect.Kind,
			item.Effect.Marker,
			item.Effect.ApproverRefs,
			time.Duration(item.Effect.TimeoutNanos),
			item.Effect.OnTimeout,
		)
		rule, ruleErr := NewRule(item.ID, condition, effect)
		if conditionErr != nil || effectErr != nil || ruleErr != nil {
			return RuleSet{}, ErrRuleAuthorityConflict
		}
		rules[index] = rule
	}
	ruleSet, err := NewRuleSet(scope, payload.Revision, rules)
	if err != nil || ruleSet.digest != payload.Digest {
		return RuleSet{}, ErrRuleAuthorityConflict
	}
	return ruleSet, nil
}

func approvalRequestedPayloadFor(
	record ApprovalRequestRecord,
) approvalRequestedPayload {
	return approvalRequestedPayload{
		ApprovalRequestID:     record.id,
		ApprovalRequestDigest: record.digest,
		Context:               actionToPayload(record.context),
		ContinuationDigest:    record.continuationDigest,
		Decision:              decisionToPayload(record.decision),
		PreviousStatus:        record.previousStatus,
		RequestedAt:           record.requestedAt.Format(time.RFC3339Nano),
		ExpiresAt:             record.expiresAt.Format(time.RFC3339Nano),
	}
}

func approvalRecordFromPayload(
	payload approvalRequestedPayload,
) (ApprovalRequestRecord, error) {
	action, err := actionFromPayload(payload.Context)
	if err != nil || !validSHA256(payload.ContinuationDigest) ||
		payload.PreviousStatus != "assigned" {
		return ApprovalRequestRecord{}, ErrRuleAuthorityConflict
	}
	decision, err := decisionFromPayload(payload.Decision)
	if err != nil || decision.kind != "require_approval" ||
		decision.actionDigest != action.digest {
		return ApprovalRequestRecord{}, ErrRuleAuthorityConflict
	}
	requestedAt, err := time.Parse(time.RFC3339Nano, payload.RequestedAt)
	if err != nil {
		return ApprovalRequestRecord{}, ErrRuleAuthorityConflict
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, payload.ExpiresAt)
	if err != nil || !expiresAt.Equal(requestedAt.Add(decision.timeout)) {
		return ApprovalRequestRecord{}, ErrRuleAuthorityConflict
	}
	record := ApprovalRequestRecord{
		id:                 payload.ApprovalRequestID,
		context:            action,
		continuationDigest: payload.ContinuationDigest,
		decision:           decision,
		status:             "pending",
		previousStatus:     "assigned",
		requestedAt:        requestedAt,
		expiresAt:          expiresAt,
	}
	record.digest = digestApprovalRecord(record)
	if record.id != approvalRequestID(
		action.digest,
		record.continuationDigest,
		decision.digest,
	) ||
		record.digest != payload.ApprovalRequestDigest {
		return ApprovalRequestRecord{}, ErrRuleAuthorityConflict
	}
	return record, nil
}

func actionToPayload(action ActionContext) actionContextPayload {
	return actionContextPayload{
		ProjectID:       action.projectID,
		TeamInstanceID:  action.teamInstanceID,
		WorkPackageID:   action.workPackageID,
		WorkItemID:      action.workItemID,
		RunID:           action.runID,
		AgentInstanceID: action.agentInstanceID,
		LogicalNodeID:   action.logicalNodeID,
		AttemptNumber:   action.attemptNumber,
		Action:          action.action,
		Risk:            action.risk,
		ClaimID:         action.claimID,
		ClaimGeneration: action.claimGeneration,
		ContractDigest:  action.contractDigest,
		Digest:          action.digest,
	}
}

func actionFromPayload(payload actionContextPayload) (ActionContext, error) {
	action, err := NewActionContext(ActionContextInput{
		ProjectID:       payload.ProjectID,
		TeamInstanceID:  payload.TeamInstanceID,
		WorkPackageID:   payload.WorkPackageID,
		WorkItemID:      payload.WorkItemID,
		RunID:           payload.RunID,
		AgentInstanceID: payload.AgentInstanceID,
		LogicalNodeID:   payload.LogicalNodeID,
		AttemptNumber:   payload.AttemptNumber,
		Action:          payload.Action,
		Risk:            payload.Risk,
		ClaimID:         payload.ClaimID,
		ClaimGeneration: payload.ClaimGeneration,
		ContractDigest:  payload.ContractDigest,
	})
	if err != nil || action.digest != payload.Digest {
		return ActionContext{}, ErrRuleAuthorityConflict
	}
	return action, nil
}

func decisionToPayload(decision Decision) decisionPayload {
	return decisionPayload{
		Kind: decision.kind, Digest: decision.digest,
		ActionDigest:   decision.actionDigest,
		RuleSetRefs:    decision.RuleSetReferences(),
		ApproverRefs:   decision.ApproverRefs(),
		TimeoutNanos:   int64(decision.timeout),
		OnTimeout:      decision.onTimeout,
		WarningMarkers: decision.WarningMarkers(),
		RecordMarkers:  decision.RecordMarkers(),
	}
}

func decisionFromPayload(payload decisionPayload) (Decision, error) {
	decision := Decision{
		kind: payload.Kind, digest: payload.Digest,
		actionDigest:   payload.ActionDigest,
		ruleSetRefs:    append([]RuleSetReference(nil), payload.RuleSetRefs...),
		approverRefs:   append([]string(nil), payload.ApproverRefs...),
		timeout:        time.Duration(payload.TimeoutNanos),
		onTimeout:      payload.OnTimeout,
		warningMarkers: append([]string(nil), payload.WarningMarkers...),
		recordMarkers:  append([]string(nil), payload.RecordMarkers...),
	}
	if !validDecision(decision) {
		return Decision{}, ErrRuleAuthorityConflict
	}
	return decision, nil
}

func validAuthorizedActivation(
	authorized AuthorizedRuleSetActivation,
	request RuleSetActivationRequest,
	now time.Time,
) bool {
	return validAuthorizedActivationBinding(authorized, request) &&
		validAuthorizationWindow(authorized.issuedAt, authorized.expiresAt, now)
}

func validAuthorizedActivationBinding(
	authorized AuthorizedRuleSetActivation,
	request RuleSetActivationRequest,
) bool {
	return authorized.scopeKind == request.ruleSet.scope.kind &&
		authorized.scopeID == request.ruleSet.scope.id &&
		authorized.revision == request.ruleSet.version &&
		authorized.ruleSetDigest == request.ruleSet.digest &&
		validRuleText(authorized.actorRef) &&
		authorized.commandDigest == ruleSetActivationCommandDigest(
			request.ruleSet,
			request.correlationID,
		) &&
		validSHA256(authorized.authorizationDigest) &&
		!zeroSeal(authorized.seal) &&
		authorized.correlationID == request.correlationID &&
		authorized.requestID == ruleSetActivationRequestID(request.ruleSet)
}

func validAuthorizedDecision(
	authorized AuthorizedApprovalDecision,
	request ApprovalDecisionRequest,
	now time.Time,
) bool {
	return validAuthorizedDecisionBinding(authorized, request) &&
		validAuthorizationWindow(authorized.issuedAt, authorized.expiresAt, now)
}

func validAuthorizedDecisionBinding(
	authorized AuthorizedApprovalDecision,
	request ApprovalDecisionRequest,
) bool {
	return authorized.approvalRequestID == request.approvalRequestID &&
		authorized.approvalRequestDigest == request.approvalRequestDigest &&
		authorized.decision == request.decision &&
		validRuleText(authorized.actorRef) &&
		authorized.commandDigest == approvalDecisionCommandDigest(
			request.approvalRequestID,
			request.approvalRequestDigest,
			request.decision,
			request.correlationID,
		) &&
		validSHA256(authorized.authorizationDigest) &&
		!zeroSeal(authorized.seal) &&
		authorized.correlationID == request.correlationID &&
		authorized.requestID == approvalDecisionRequestID(request)
}

func validAuthorizationWindow(issuedAt, expiresAt, now time.Time) bool {
	return !issuedAt.IsZero() && issuedAt.Location() == time.UTC &&
		!expiresAt.IsZero() && expiresAt.Location() == time.UTC &&
		!issuedAt.After(now) && now.Before(expiresAt) &&
		expiresAt.Sub(issuedAt) <= 24*time.Hour
}

func ruleSetActivationCommandDigest(
	ruleSet RuleSet,
	correlationID string,
) string {
	return digestFields(
		"loom.rule-set-activation-command.v1",
		ruleSet.StreamID(),
		fmt.Sprint(ruleSet.version),
		ruleSet.digest,
		correlationID,
	)
}

func ruleSetActivationRequestID(ruleSet RuleSet) string {
	return digestFields(
		"loom.rule-set-activation-request.v1",
		ruleSet.StreamID(),
		fmt.Sprint(ruleSet.version),
		ruleSet.digest,
	)
}

func approvalDecisionCommandDigest(
	approvalRequestID, approvalRequestDigest, decision, correlationID string,
) string {
	return digestFields(
		"loom.approval-decision-command.v1",
		approvalRequestID,
		approvalRequestDigest,
		decision,
		correlationID,
	)
}

func approvalDecisionRequestID(request ApprovalDecisionRequest) string {
	return digestFields(
		"loom.approval-decision-request.v1",
		request.approvalRequestID,
		request.approvalRequestDigest,
		request.decision,
	)
}

func approvalRequestID(
	actionDigest, continuationDigest, decisionDigest string,
) string {
	sum := sha256.Sum256([]byte(digestFields(
		"loom.approval-request-id.v1",
		actionDigest,
		continuationDigest,
		decisionDigest,
	)))
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

func digestRuleSet(ruleSet RuleSet) string {
	fields := []string{
		"loom.rule-set.v1",
		ruleSet.scope.kind,
		ruleSet.scope.id,
		fmt.Sprint(ruleSet.version),
	}
	for _, rule := range ruleSet.rules {
		fields = append(
			fields,
			rule.id,
			rule.condition.action,
			rule.condition.risk,
			rule.effect.kind,
			rule.effect.marker,
			fmt.Sprint(int64(rule.effect.timeout)),
			rule.effect.onTimeout,
		)
		fields = append(fields, rule.effect.approverRefs...)
	}
	return digestFields(fields...)
}

func digestActionContext(action ActionContext) string {
	return digestFields(
		"loom.rule-action-context.v1",
		action.projectID,
		action.teamInstanceID,
		action.workPackageID,
		action.workItemID,
		action.runID,
		action.agentInstanceID,
		action.logicalNodeID,
		fmt.Sprint(action.attemptNumber),
		action.action,
		action.risk,
		action.claimID,
		fmt.Sprint(action.claimGeneration),
		action.contractDigest,
	)
}

func digestDecision(
	decision Decision,
	matched []matchedRuleReference,
) string {
	fields := []string{
		"loom.rule-decision.v1",
		decision.kind,
		decision.actionDigest,
		fmt.Sprint(int64(decision.timeout)),
		decision.onTimeout,
	}
	for _, reference := range decision.ruleSetRefs {
		fields = append(
			fields,
			reference.StreamID,
			reference.Kind,
			reference.ScopeID,
			fmt.Sprint(reference.Version),
			reference.Digest,
		)
	}
	for _, match := range matched {
		fields = append(
			fields,
			fmt.Sprint(match.scopeRank),
			match.ruleID,
			fmt.Sprint(match.version),
		)
	}
	fields = append(fields, decision.approverRefs...)
	fields = append(fields, decision.warningMarkers...)
	fields = append(fields, decision.recordMarkers...)
	return digestFields(fields...)
}

func digestApprovalRecord(record ApprovalRequestRecord) string {
	fields := []string{
		"loom.approval-request.v1",
		record.id,
		record.context.digest,
		record.continuationDigest,
		record.decision.digest,
		record.previousStatus,
		record.requestedAt.Format(time.RFC3339Nano),
		record.expiresAt.Format(time.RFC3339Nano),
	}
	return digestFields(fields...)
}

func digestFields(fields ...string) string {
	hash := sha256.New()
	var length [8]byte
	for _, field := range fields {
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(field))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func approvalRequestStreams(action ActionContext, approvalID string) []string {
	streams := []string{
		approvalStream(approvalID),
		workItemStream(action.workItemID),
		runStream(action.runID),
	}
	streams = append(streams, ruleSetStreamsForAction(action)...)
	sort.Strings(streams)
	return streams
}

func ruleSetStreamsForAction(action ActionContext) []string {
	streams := []string{
		ruleSetStream(Scope{kind: "project", id: action.projectID}),
		ruleSetStream(Scope{kind: "team", id: action.teamInstanceID}),
		ruleSetStream(Scope{
			kind: "work_package",
			id:   action.workPackageID,
		}),
		ruleSetStream(Scope{kind: "work_item", id: action.workItemID}),
	}
	sort.Strings(streams)
	return streams
}

func expectationsFor(
	snapshot journal.StreamSetSnapshot,
	streamIDs []string,
) []journal.StreamHeadExpectation {
	expectations := make([]journal.StreamHeadExpectation, len(streamIDs))
	for index, streamID := range streamIDs {
		head, _ := snapshot.Head(streamID)
		expectations[index] = journal.StreamHeadExpectation{
			StreamID: streamID,
			Sequence: head.Sequence,
		}
	}
	return expectations
}

func headsAfterResolution(
	snapshot journal.StreamSetSnapshot,
	workEvent, approvalEvent journal.Event,
) []journal.StreamHead {
	heads := snapshot.Heads()
	for index := range heads {
		switch heads[index].StreamID {
		case workEvent.StreamID:
			heads[index].Sequence = workEvent.Seq
			heads[index].EventID = workEvent.ID
		case approvalEvent.StreamID:
			heads[index].Sequence = approvalEvent.Seq
			heads[index].EventID = approvalEvent.ID
		}
	}
	return heads
}

func eventsForStream(events []journal.Event, streamID string) []journal.Event {
	var selected []journal.Event
	for _, event := range events {
		if event.StreamID == streamID {
			selected = append(selected, event)
		}
	}
	return selected
}

func newRulesEvent(
	id, streamID string,
	sequence int64,
	eventType string,
	emittedAt time.Time,
	correlationID, causationID string,
	payload any,
) journal.Event {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return journal.Event{
		ID: id, StreamID: streamID, Seq: sequence, IdempotencyKey: id,
		Type: eventType, SchemaVersion: 1, EmittedAt: emittedAt,
		CorrelationID: correlationID, CausationID: causationID,
		PayloadJSON: body,
	}
}

func deterministicEventID(parts ...string) string {
	return "evt-" + digestFields(parts...)[:32]
}

func decodeExact(data []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON")
		}
		return err
	}
	return nil
}

func mapRulesJournalError(err error) error {
	if errors.Is(err, journal.ErrStreamHeadConflict) ||
		errors.Is(err, journal.ErrSequenceConflict) ||
		errors.Is(err, journal.ErrIdempotencyConflict) ||
		errors.Is(err, journal.ErrPartialEventBatchConflict) {
		return fmt.Errorf("%w: %v", ErrRuleAuthorityConflict, err)
	}
	return err
}

func ruleSetStream(scope Scope) string {
	return "rule-set/" + scope.kind + "/" + scope.id
}
func approvalStream(id string) string { return "approval/" + id }
func workItemStream(id string) string { return "work-item/" + id }
func runStream(id string) string      { return "run/" + id }

func validScopeKind(kind string) bool {
	return kind == "project" || kind == "team" ||
		kind == "work_package" || kind == "work_item"
}

func validScope(scope Scope) bool {
	return validScopeKind(scope.kind) && validRuleText(scope.id)
}

func validCondition(condition Condition) bool {
	return validRuleText(condition.action) &&
		(condition.risk == "" || condition.risk == "low" ||
			condition.risk == "medium" || condition.risk == "high")
}

func validEffect(effect Effect) bool {
	rebuilt, err := NewEffect(
		effect.kind,
		effect.marker,
		effect.approverRefs,
		effect.timeout,
		effect.onTimeout,
	)
	return err == nil && reflectEffectEqual(rebuilt, effect)
}

func validRule(rule Rule) bool {
	return validRuleText(rule.id) &&
		validCondition(rule.condition) &&
		validEffect(rule.effect)
}

func validRuleSet(ruleSet RuleSet) bool {
	rebuilt, err := NewRuleSet(ruleSet.scope, ruleSet.version, ruleSet.rules)
	return err == nil && rebuilt.digest == ruleSet.digest
}

func validActionContext(action ActionContext) bool {
	rebuilt, err := NewActionContext(ActionContextInput{
		ProjectID:       action.projectID,
		TeamInstanceID:  action.teamInstanceID,
		WorkPackageID:   action.workPackageID,
		WorkItemID:      action.workItemID,
		RunID:           action.runID,
		AgentInstanceID: action.agentInstanceID,
		LogicalNodeID:   action.logicalNodeID,
		AttemptNumber:   action.attemptNumber,
		Action:          action.action,
		Risk:            action.risk,
		ClaimID:         action.claimID,
		ClaimGeneration: action.claimGeneration,
		ContractDigest:  action.contractDigest,
	})
	return err == nil && rebuilt.digest == action.digest
}

func validDecision(decision Decision) bool {
	if (decision.kind != "allow" &&
		decision.kind != "record" &&
		decision.kind != "warn" &&
		decision.kind != "require_approval" &&
		decision.kind != "reject") ||
		!validSHA256(decision.digest) ||
		!validSHA256(decision.actionDigest) ||
		len(decision.ruleSetRefs) > maxRuleSets ||
		len(decision.approverRefs) > maxApproverRefs ||
		len(decision.warningMarkers) > maxDecisionMarkers ||
		len(decision.recordMarkers) > maxDecisionMarkers {
		return false
	}
	for index, reference := range decision.ruleSetRefs {
		if !validScopeKind(reference.Kind) ||
			!validRuleText(reference.ScopeID) ||
			reference.StreamID != "rule-set/"+
				reference.Kind+"/"+reference.ScopeID ||
			reference.Version <= 0 ||
			!validSHA256(reference.Digest) ||
			index > 0 &&
				decision.ruleSetRefs[index-1].StreamID >= reference.StreamID {
			return false
		}
	}
	approvers, approversValid := normalizedUnique(
		decision.approverRefs,
		maxApproverRefs,
	)
	warnings, warningsValid := normalizedUnique(
		decision.warningMarkers,
		maxDecisionMarkers,
	)
	records, recordsValid := normalizedUnique(
		decision.recordMarkers,
		maxDecisionMarkers,
	)
	if !approversValid || !warningsValid || !recordsValid ||
		!sameRuleStrings(approvers, decision.approverRefs) ||
		!sameRuleStrings(warnings, decision.warningMarkers) ||
		!sameRuleStrings(records, decision.recordMarkers) {
		return false
	}
	if decision.kind == "require_approval" {
		return len(decision.approverRefs) > 0 &&
			decision.timeout >= time.Second &&
			decision.timeout <= maxApprovalTimeout &&
			(decision.onTimeout == "reject" ||
				decision.onTimeout == "cancel")
	}
	return decision.timeout == 0 && decision.onTimeout == "" &&
		len(decision.approverRefs) == 0
}

func equalDecision(left, right Decision) bool {
	leftBody, _ := json.Marshal(decisionToPayload(left))
	rightBody, _ := json.Marshal(decisionToPayload(right))
	return string(leftBody) == string(rightBody)
}

func validApprovalDecision(value string) bool {
	return value == "approved" || value == "rejected" || value == "cancelled"
}

func validApprovalTerminal(value string) bool {
	return validApprovalDecision(value) || value == "expired"
}

func validApprovalResolution(
	eventType, status, actorRef, authorizationDigest string,
) bool {
	if eventType == "ApprovalExpired" {
		return status == "expired" &&
			actorRef == "" &&
			authorizationDigest == ""
	}
	return eventType == "ApprovalDecided" &&
		validApprovalDecision(status) &&
		validRuleText(actorRef) &&
		validSHA256(authorizationDigest)
}

func validCorrelationID(value string) bool {
	return validRuleText(strings.TrimSpace(value))
}

func validRuleText(value string) bool {
	if value == "" || len(value) > maxRuleTextBytes ||
		!utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') &&
			(character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func normalizedUnique(values []string, maximum int) ([]string, bool) {
	if len(values) > maximum {
		return nil, false
	}
	cloned := append([]string(nil), values...)
	sort.Strings(cloned)
	result := cloned[:0]
	for _, value := range cloned {
		value = strings.TrimSpace(value)
		if !validRuleText(value) {
			return nil, false
		}
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return append([]string(nil), result...), true
}

func cloneEffect(effect Effect) Effect {
	effect.approverRefs = append([]string(nil), effect.approverRefs...)
	return effect
}

func cloneRules(rules []Rule) []Rule {
	cloned := make([]Rule, len(rules))
	for index, rule := range rules {
		cloned[index] = rule
		cloned[index].effect = cloneEffect(rule.effect)
	}
	return cloned
}

func cloneRuleSets(ruleSets []RuleSet) []RuleSet {
	cloned := make([]RuleSet, len(ruleSets))
	for index, ruleSet := range ruleSets {
		cloned[index] = ruleSet
		cloned[index].rules = cloneRules(ruleSet.rules)
	}
	return cloned
}

func scopeMatches(scope Scope, action ActionContext) bool {
	switch scope.kind {
	case "project":
		return scope.id == action.projectID
	case "team":
		return scope.id == action.teamInstanceID
	case "work_package":
		return scope.id == action.workPackageID
	case "work_item":
		return scope.id == action.workItemID
	default:
		return false
	}
}

func scopeRank(kind string) int {
	switch kind {
	case "work_item":
		return 4
	case "work_package":
		return 3
	case "team":
		return 2
	case "project":
		return 1
	default:
		return 0
	}
}

func ruleToPayload(rule Rule) rulePayload {
	return rulePayload{
		ID: rule.id,
		Condition: conditionPayload{
			Action: rule.condition.action,
			Risk:   rule.condition.risk,
		},
		Effect: effectPayload{
			Kind:         rule.effect.kind,
			Marker:       rule.effect.marker,
			ApproverRefs: append([]string(nil), rule.effect.approverRefs...),
			TimeoutNanos: int64(rule.effect.timeout),
			OnTimeout:    rule.effect.onTimeout,
		},
	}
}

func reflectEffectEqual(left, right Effect) bool {
	if left.kind != right.kind || left.marker != right.marker ||
		left.timeout != right.timeout || left.onTimeout != right.onTimeout ||
		len(left.approverRefs) != len(right.approverRefs) {
		return false
	}
	for index := range left.approverRefs {
		if left.approverRefs[index] != right.approverRefs[index] {
			return false
		}
	}
	return true
}

func sameRuleStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func contains(values []string, target string) bool {
	index := sort.SearchStrings(values, target)
	return index < len(values) && values[index] == target
}

func zeroSeal(seal [32]byte) bool {
	return seal == [32]byte{}
}

func authorizationSeal(commandDigest, authorizationDigest string) [32]byte {
	return sha256.Sum256([]byte(
		"loom.customer-authorization.v1\x00" +
			commandDigest + "\x00" + authorizationDigest,
	))
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
