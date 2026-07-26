package projection

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/journal"
	ruleauthority "loom-pi-rebuild/internal/rules"
)

type ProjectedRule struct {
	ID           string
	Action       string
	Risk         string
	EffectKind   string
	Marker       string
	ApproverRefs []string
	Timeout      time.Duration
	OnTimeout    string
}

type ProjectedRuleSet struct {
	StreamID            string
	ScopeKind           string
	ScopeID             string
	Revision            int
	Digest              string
	Rules               []ProjectedRule
	ActorRef            string
	AuthorizationDigest string
	CommandDigest       string
	ActivationEventID   string
}

type ProjectedRuleSetReference struct {
	StreamID  string
	ScopeKind string
	ScopeID   string
	Revision  int
	Digest    string
}

type ProjectedApprovalRequest struct {
	ID                  string
	Digest              string
	ProjectID           string
	TeamInstanceID      string
	WorkPackageID       string
	WorkItemID          string
	RunID               string
	AgentInstanceID     string
	LogicalNodeID       string
	AttemptNumber       int
	Action              string
	Risk                string
	ContractDigest      string
	ActionDigest        string
	ContinuationDigest  string
	DecisionKind        string
	DecisionDigest      string
	RuleSetReferences   []ProjectedRuleSetReference
	ApproverRefs        []string
	Timeout             time.Duration
	OnTimeout           string
	WarningMarkers      []string
	RecordMarkers       []string
	PreviousStatus      string
	Status              string
	RequestedAt         time.Time
	ExpiresAt           time.Time
	DecisionActorRef    string
	AuthorizationDigest string
	DecidedAt           time.Time
	RequestEventID      string
	TerminalEventID     string
}

type projectionScopePayload struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type projectionConditionPayload struct {
	Action string `json:"action"`
	Risk   string `json:"risk"`
}

type projectionEffectPayload struct {
	Kind         string    `json:"kind"`
	Marker       string    `json:"marker"`
	ApproverRefs *[]string `json:"approver_refs"`
	TimeoutNanos int64     `json:"timeout_nanos"`
	OnTimeout    string    `json:"on_timeout"`
}

type projectionRulePayload struct {
	ID        string                     `json:"id"`
	Condition projectionConditionPayload `json:"condition"`
	Effect    projectionEffectPayload    `json:"effect"`
}

type projectionRuleSetPayload struct {
	Scope               projectionScopePayload   `json:"scope"`
	Revision            int                      `json:"revision"`
	Digest              string                   `json:"digest"`
	Rules               *[]projectionRulePayload `json:"rules"`
	ActorRef            string                   `json:"actor_ref"`
	AuthorizationDigest string                   `json:"authorization_digest"`
	CommandDigest       string                   `json:"command_digest"`
}

type projectionActionPayload struct {
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

type projectionRuleSetReferencePayload struct {
	StreamID  string `json:"stream_id"`
	ScopeKind string `json:"scope_kind"`
	ScopeID   string `json:"scope_id"`
	Version   int    `json:"version"`
	Digest    string `json:"digest"`
}

type projectionDecisionPayload struct {
	Kind           string                               `json:"kind"`
	Digest         string                               `json:"digest"`
	ActionDigest   string                               `json:"action_digest"`
	RuleSetRefs    *[]projectionRuleSetReferencePayload `json:"rule_set_refs"`
	ApproverRefs   *[]string                            `json:"approver_refs"`
	TimeoutNanos   int64                                `json:"timeout_nanos"`
	OnTimeout      string                               `json:"on_timeout"`
	WarningMarkers *[]string                            `json:"warning_markers"`
	RecordMarkers  *[]string                            `json:"record_markers"`
}

type projectionApprovalRequestedPayload struct {
	ApprovalRequestID     string                    `json:"approval_request_id"`
	ApprovalRequestDigest string                    `json:"approval_request_digest"`
	Context               projectionActionPayload   `json:"context"`
	ContinuationDigest    string                    `json:"continuation_digest"`
	Decision              projectionDecisionPayload `json:"decision"`
	PreviousStatus        string                    `json:"previous_status"`
	RequestedAt           string                    `json:"requested_at"`
	ExpiresAt             string                    `json:"expires_at"`
}

type projectionApprovalResolutionPayload struct {
	ApprovalRequestID     string `json:"approval_request_id"`
	ApprovalRequestDigest string `json:"approval_request_digest"`
	Status                string `json:"status"`
	ActorRef              string `json:"actor_ref"`
	AuthorizationDigest   string `json:"authorization_digest"`
	DecidedAt             string `json:"decided_at"`
}

type projectionWorkApprovalPayload struct {
	WorkItemID            string `json:"work_item_id"`
	ApprovalRequestID     string `json:"approval_request_id"`
	ApprovalRequestDigest string `json:"approval_request_digest"`
	PreviousStatus        string `json:"previous_status"`
	Status                string `json:"status"`
}

func isApprovalProjectionEvent(event journal.Event) bool {
	switch event.Type {
	case "RuleSetActivated",
		"ApprovalRequested",
		"ApprovalDecided",
		"ApprovalExpired",
		"WorkItemApprovalPaused",
		"WorkItemApprovalResolved":
		return true
	default:
		return false
	}
}

func runAuthorityEventsWithoutApprovalFacts(
	events []journal.Event,
) []journal.Event {
	out := make([]journal.Event, 0, len(events))
	nextWorkSequence := make(map[string]int64)
	for _, event := range events {
		if event.Type == "WorkItemApprovalPaused" ||
			event.Type == "WorkItemApprovalResolved" {
			continue
		}
		cloned := event
		if strings.HasPrefix(cloned.StreamID, "work-item/") {
			nextWorkSequence[cloned.StreamID]++
			cloned.Seq = nextWorkSequence[cloned.StreamID]
		}
		out = append(out, cloned)
	}
	return out
}

func applyApprovalProjection(
	ctx context.Context,
	snapshot *Snapshot,
	approvalEvents []journal.Event,
	allEvents []journal.Event,
) error {
	if snapshot == nil {
		return ErrInvalidProjectionEvent
	}
	ruleSets, domainRuleSets, err := projectRuleSetEvents(ctx, approvalEvents)
	if err != nil {
		return fmt.Errorf("project RuleSets: %w", err)
	}
	requests, err := projectApprovalRequestEvents(
		ctx,
		approvalEvents,
		allEvents,
		ruleSets,
		domainRuleSets,
		snapshot.WorkItems,
	)
	if err != nil {
		return fmt.Errorf("project ApprovalRequests: %w", err)
	}
	for streamID, record := range ruleSets {
		snapshot.RuleSets[streamID] = cloneProjectedRuleSet(record)
	}
	for approvalID, record := range requests {
		snapshot.ApprovalRequests[approvalID] =
			cloneProjectedApprovalRequest(record)
	}
	return nil
}

func projectRuleSetEvents(
	ctx context.Context,
	events []journal.Event,
) (
	map[string]ProjectedRuleSet,
	map[string]ruleauthority.RuleSet,
	error,
) {
	byStream := make(map[string][]journal.Event)
	for _, event := range events {
		if event.Type == "RuleSetActivated" {
			if !strings.HasPrefix(event.StreamID, "rule-set/") {
				return nil, nil, ErrInvalidProjectionEvent
			}
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
	}
	projected := make(map[string]ProjectedRuleSet, len(byStream))
	domain := make(map[string]ruleauthority.RuleSet, len(byStream))
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		var previousEventID string
		for index, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			if !validRunProjectionEnvelope(event) ||
				event.Seq != int64(index+1) ||
				event.CausationID != previousEventID {
				return nil, nil, ErrInvalidProjectionEvent
			}
			var payload projectionRuleSetPayload
			if decodeExactProjectionPayload(event, &payload) != nil ||
				payload.Rules == nil ||
				payload.Revision != index+1 ||
				!validSHA256Digest(payload.Digest) ||
				!validProjectionText(payload.ActorRef) ||
				!validSHA256Digest(payload.AuthorizationDigest) ||
				!validSHA256Digest(payload.CommandDigest) {
				return nil, nil, ErrInvalidProjectionEvent
			}
			scope, err := ruleauthority.NewScope(
				payload.Scope.Kind,
				payload.Scope.ID,
			)
			if err != nil ||
				streamID != "rule-set/"+scope.Kind()+"/"+scope.ID() {
				return nil, nil, ErrInvalidProjectionEvent
			}
			rules := make([]ruleauthority.Rule, len(*payload.Rules))
			publicRules := make([]ProjectedRule, len(*payload.Rules))
			for ruleIndex, item := range *payload.Rules {
				if item.Effect.ApproverRefs == nil {
					return nil, nil, ErrInvalidProjectionEvent
				}
				condition, conditionErr := ruleauthority.NewCondition(
					item.Condition.Action,
					item.Condition.Risk,
				)
				effect, effectErr := ruleauthority.NewEffect(
					item.Effect.Kind,
					item.Effect.Marker,
					*item.Effect.ApproverRefs,
					time.Duration(item.Effect.TimeoutNanos),
					item.Effect.OnTimeout,
				)
				rule, ruleErr := ruleauthority.NewRule(
					item.ID,
					condition,
					effect,
				)
				if conditionErr != nil || effectErr != nil || ruleErr != nil {
					return nil, nil, ErrInvalidProjectionEvent
				}
				rules[ruleIndex] = rule
				publicRules[ruleIndex] = ProjectedRule{
					ID: rule.ID(), Action: condition.Action(),
					Risk: condition.Risk(), EffectKind: effect.Kind(),
					Marker:       effect.Marker(),
					ApproverRefs: effect.ApproverRefs(),
					Timeout:      effect.Timeout(), OnTimeout: effect.OnTimeout(),
				}
			}
			ruleSet, err := ruleauthority.NewRuleSet(
				scope,
				payload.Revision,
				rules,
			)
			if err != nil || ruleSet.Digest() != payload.Digest {
				return nil, nil, ErrInvalidProjectionEvent
			}
			projected[streamID] = ProjectedRuleSet{
				StreamID: streamID, ScopeKind: scope.Kind(),
				ScopeID: scope.ID(), Revision: payload.Revision,
				Digest: payload.Digest, Rules: publicRules,
				ActorRef:            payload.ActorRef,
				AuthorizationDigest: payload.AuthorizationDigest,
				CommandDigest:       payload.CommandDigest,
				ActivationEventID:   event.ID,
			}
			domain[streamID] = ruleSet
			previousEventID = event.ID
		}
	}
	return projected, domain, nil
}

func projectApprovalRequestEvents(
	ctx context.Context,
	approvalEvents []journal.Event,
	allEvents []journal.Event,
	ruleSets map[string]ProjectedRuleSet,
	domainRuleSets map[string]ruleauthority.RuleSet,
	workItems map[string]WorkItem,
) (map[string]ProjectedApprovalRequest, error) {
	byStream := make(map[string][]journal.Event)
	workApprovalEvents := make(map[string][]journal.Event)
	eventByID := make(map[string]journal.Event, len(allEvents))
	latestNonApprovalWorkSequence := make(map[string]int64)
	for _, event := range allEvents {
		eventByID[event.ID] = event
		if strings.HasPrefix(event.StreamID, "work-item/") &&
			event.Type != "WorkItemApprovalPaused" &&
			event.Type != "WorkItemApprovalResolved" &&
			event.Seq > latestNonApprovalWorkSequence[event.StreamID] {
			latestNonApprovalWorkSequence[event.StreamID] = event.Seq
		}
	}
	for _, event := range approvalEvents {
		switch event.Type {
		case "ApprovalRequested", "ApprovalDecided", "ApprovalExpired":
			if !strings.HasPrefix(event.StreamID, "approval/") {
				return nil, ErrInvalidProjectionEvent
			}
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		case "WorkItemApprovalPaused", "WorkItemApprovalResolved":
			var payload projectionWorkApprovalPayload
			if decodeExactProjectionPayload(event, &payload) != nil ||
				!validProjectionText(payload.WorkItemID) ||
				!validProjectionText(payload.ApprovalRequestID) ||
				!validSHA256Digest(payload.ApprovalRequestDigest) ||
				event.StreamID != "work-item/"+payload.WorkItemID ||
				!validRunProjectionEnvelope(event) {
				return nil, ErrInvalidProjectionEvent
			}
			workApprovalEvents[payload.ApprovalRequestID] =
				append(workApprovalEvents[payload.ApprovalRequestID], event)
		}
	}
	requests := make(map[string]ProjectedApprovalRequest, len(byStream))
	consumedWorkFacts := make(map[string]struct{})
	for streamID, streamEvents := range byStream {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		approvalID := strings.TrimPrefix(streamID, "approval/")
		if !validProjectionText(approvalID) ||
			len(streamEvents) < 1 || len(streamEvents) > 2 {
			return nil, ErrInvalidProjectionEvent
		}
		requestEvent := streamEvents[0]
		if !validRunProjectionEnvelope(requestEvent) ||
			requestEvent.Type != "ApprovalRequested" ||
			requestEvent.Seq != 1 {
			return nil, ErrInvalidProjectionEvent
		}
		var requested projectionApprovalRequestedPayload
		if decodeExactProjectionPayload(requestEvent, &requested) != nil ||
			requested.ApprovalRequestID != approvalID ||
			!validSHA256Digest(requested.ApprovalRequestDigest) ||
			!validSHA256Digest(requested.ContinuationDigest) ||
			requested.PreviousStatus != "assigned" {
			return nil, ErrInvalidProjectionEvent
		}
		record, err := projectedApprovalFromRequest(
			requested,
			requestEvent,
			ruleSets,
			domainRuleSets,
		)
		if err != nil {
			return nil, fmt.Errorf("request %s payload: %w", approvalID, err)
		}
		assignment, ok := eventByID[requestEvent.CausationID]
		workItem, workExists := workItems[record.WorkItemID]
		if !ok || assignment.Type != "WorkItemAssigned" ||
			assignment.StreamID != "work-item/"+record.WorkItemID ||
			!workExists ||
			workItem.RunID != record.RunID ||
			workItem.AgentInstanceID != record.AgentInstanceID {
			return nil, fmt.Errorf(
				"request %s assignment binding: %w",
				approvalID,
				ErrInvalidProjectionEvent,
			)
		}
		facts := workApprovalEvents[approvalID]
		sort.SliceStable(facts, func(i, j int) bool {
			return facts[i].Seq < facts[j].Seq
		})
		if len(facts) < 1 || len(facts) > 2 {
			return nil, ErrInvalidProjectionEvent
		}
		pauseEvent := facts[0]
		var pause projectionWorkApprovalPayload
		if decodeExactProjectionPayload(pauseEvent, &pause) != nil ||
			pauseEvent.Type != "WorkItemApprovalPaused" ||
			pauseEvent.CausationID != requestEvent.ID ||
			pause.WorkItemID != record.WorkItemID ||
			pause.ApprovalRequestID != approvalID ||
			pause.ApprovalRequestDigest != record.Digest ||
			pause.PreviousStatus != "assigned" ||
			pause.Status != "waiting_approval" ||
			pauseEvent.Seq != assignment.Seq+1 {
			return nil, fmt.Errorf(
				"request %s pause binding: %w",
				approvalID,
				ErrInvalidProjectionEvent,
			)
		}
		consumedWorkFacts[pauseEvent.ID] = struct{}{}
		record.Status = "pending"
		latestApprovalSequence := pauseEvent.Seq
		approvalWorkStatus := "waiting_approval"
		if len(streamEvents) == 2 {
			terminalEvent := streamEvents[1]
			if !validRunProjectionEnvelope(terminalEvent) ||
				terminalEvent.Seq != 2 ||
				terminalEvent.CausationID != requestEvent.ID ||
				(terminalEvent.Type != "ApprovalDecided" &&
					terminalEvent.Type != "ApprovalExpired") ||
				len(facts) != 2 {
				return nil, ErrInvalidProjectionEvent
			}
			var resolution projectionApprovalResolutionPayload
			if decodeExactProjectionPayload(terminalEvent, &resolution) != nil ||
				resolution.ApprovalRequestID != approvalID ||
				resolution.ApprovalRequestDigest != record.Digest ||
				!validApprovalProjectionTerminal(
					terminalEvent.Type,
					resolution.Status,
					resolution.ActorRef,
					resolution.AuthorizationDigest,
				) {
				return nil, ErrInvalidProjectionEvent
			}
			decidedAt, err := parseApprovalProjectionTime(
				resolution.DecidedAt,
			)
			if err != nil || decidedAt.Before(record.RequestedAt) {
				return nil, ErrInvalidProjectionEvent
			}
			resolvedEvent := facts[1]
			var resolved projectionWorkApprovalPayload
			if decodeExactProjectionPayload(resolvedEvent, &resolved) != nil ||
				resolvedEvent.Type != "WorkItemApprovalResolved" ||
				resolvedEvent.CausationID != terminalEvent.ID ||
				resolved.WorkItemID != record.WorkItemID ||
				resolved.ApprovalRequestID != approvalID ||
				resolved.ApprovalRequestDigest != record.Digest ||
				resolved.PreviousStatus != "waiting_approval" ||
				resolved.Status != approvalWorkStatusFor(
					resolution.Status,
					record.OnTimeout,
				) ||
				resolvedEvent.Seq != pauseEvent.Seq+1 {
				return nil, ErrInvalidProjectionEvent
			}
			consumedWorkFacts[resolvedEvent.ID] = struct{}{}
			record.Status = resolution.Status
			record.DecisionActorRef = resolution.ActorRef
			record.AuthorizationDigest = resolution.AuthorizationDigest
			record.DecidedAt = decidedAt
			record.TerminalEventID = terminalEvent.ID
			latestApprovalSequence = resolvedEvent.Seq
			approvalWorkStatus = resolved.Status
		} else if len(facts) != 1 {
			return nil, ErrInvalidProjectionEvent
		}
		workStream := "work-item/" + record.WorkItemID
		if latestApprovalSequence >
			latestNonApprovalWorkSequence[workStream] {
			workItem.Status = approvalWorkStatus
			workItems[record.WorkItemID] = workItem
		}
		requests[approvalID] = record
	}
	for _, facts := range workApprovalEvents {
		for _, event := range facts {
			if _, ok := consumedWorkFacts[event.ID]; !ok {
				return nil, ErrInvalidProjectionEvent
			}
		}
	}
	return requests, nil
}

func projectedApprovalFromRequest(
	payload projectionApprovalRequestedPayload,
	event journal.Event,
	ruleSets map[string]ProjectedRuleSet,
	domainRuleSets map[string]ruleauthority.RuleSet,
) (ProjectedApprovalRequest, error) {
	action, err := ruleauthority.NewActionContext(
		ruleauthority.ActionContextInput{
			ProjectID:       payload.Context.ProjectID,
			TeamInstanceID:  payload.Context.TeamInstanceID,
			WorkPackageID:   payload.Context.WorkPackageID,
			WorkItemID:      payload.Context.WorkItemID,
			RunID:           payload.Context.RunID,
			AgentInstanceID: payload.Context.AgentInstanceID,
			LogicalNodeID:   payload.Context.LogicalNodeID,
			AttemptNumber:   payload.Context.AttemptNumber,
			Action:          payload.Context.Action,
			Risk:            payload.Context.Risk,
			ClaimID:         payload.Context.ClaimID,
			ClaimGeneration: payload.Context.ClaimGeneration,
			ContractDigest:  payload.Context.ContractDigest,
		},
	)
	if err != nil || action.Digest() != payload.Context.Digest ||
		action.Action() != "start_run" ||
		action.ClaimID() != "" || action.ClaimGeneration() != 0 {
		return ProjectedApprovalRequest{}, fmt.Errorf(
			"invalid action context: %w",
			ErrInvalidProjectionEvent,
		)
	}
	if payload.Decision.RuleSetRefs == nil ||
		payload.Decision.ApproverRefs == nil {
		return ProjectedApprovalRequest{}, fmt.Errorf(
			"missing decision collection: %w",
			ErrInvalidProjectionEvent,
		)
	}
	if payload.Decision.Kind != "require_approval" {
		return ProjectedApprovalRequest{}, fmt.Errorf(
			"invalid decision kind: %w",
			ErrInvalidProjectionEvent,
		)
	}
	if payload.Decision.ActionDigest != action.Digest() {
		return ProjectedApprovalRequest{}, fmt.Errorf(
			"invalid decision action binding: %w",
			ErrInvalidProjectionEvent,
		)
	}
	if !validSHA256Digest(payload.Decision.Digest) {
		return ProjectedApprovalRequest{}, fmt.Errorf(
			"invalid decision digest: %w",
			ErrInvalidProjectionEvent,
		)
	}
	warningMarkers := []string{}
	if payload.Decision.WarningMarkers != nil {
		warningMarkers = append(
			warningMarkers,
			(*payload.Decision.WarningMarkers)...,
		)
	}
	recordMarkers := []string{}
	if payload.Decision.RecordMarkers != nil {
		recordMarkers = append(
			recordMarkers,
			(*payload.Decision.RecordMarkers)...,
		)
	}
	if !validSortedProjectionTexts(*payload.Decision.ApproverRefs, false) ||
		!validSortedProjectionTexts(warningMarkers, true) ||
		!validSortedProjectionTexts(recordMarkers, true) {
		return ProjectedApprovalRequest{}, fmt.Errorf(
			"invalid decision collections: %w",
			ErrInvalidProjectionEvent,
		)
	}
	requestedAt, err := parseApprovalProjectionTime(payload.RequestedAt)
	if err != nil {
		return ProjectedApprovalRequest{}, err
	}
	expiresAt, err := parseApprovalProjectionTime(payload.ExpiresAt)
	timeout := time.Duration(payload.Decision.TimeoutNanos)
	if err != nil || timeout < time.Second ||
		timeout > 30*24*time.Hour ||
		!expiresAt.Equal(requestedAt.Add(timeout)) ||
		(payload.Decision.OnTimeout != "reject" &&
			payload.Decision.OnTimeout != "cancel") {
		return ProjectedApprovalRequest{}, fmt.Errorf(
			"invalid approval timing: %w",
			ErrInvalidProjectionEvent,
		)
	}
	expectedID := canonicalApprovalProjectionID(
		action.Digest(),
		payload.ContinuationDigest,
		payload.Decision.Digest,
	)
	expectedDigest := canonicalApprovalProjectionDigest(
		expectedID,
		action.Digest(),
		payload.ContinuationDigest,
		payload.Decision.Digest,
		requestedAt,
		expiresAt,
	)
	if payload.ApprovalRequestID != expectedID ||
		payload.ApprovalRequestDigest != expectedDigest {
		return ProjectedApprovalRequest{}, fmt.Errorf(
			"invalid approval identity: %w",
			ErrInvalidProjectionEvent,
		)
	}
	domainSets := make([]ruleauthority.RuleSet, len(*payload.Decision.RuleSetRefs))
	references := make(
		[]ProjectedRuleSetReference,
		len(*payload.Decision.RuleSetRefs),
	)
	for index, reference := range *payload.Decision.RuleSetRefs {
		projected, ok := ruleSets[reference.StreamID]
		domainSet, domainOK := domainRuleSets[reference.StreamID]
		if !ok || !domainOK ||
			projected.ScopeKind != reference.ScopeKind ||
			projected.ScopeID != reference.ScopeID ||
			projected.Revision != reference.Version ||
			projected.Digest != reference.Digest {
			return ProjectedApprovalRequest{}, fmt.Errorf(
				"invalid RuleSet reference %s: %w",
				reference.StreamID,
				ErrInvalidProjectionEvent,
			)
		}
		domainSets[index] = domainSet
		references[index] = ProjectedRuleSetReference{
			StreamID:  reference.StreamID,
			ScopeKind: reference.ScopeKind,
			ScopeID:   reference.ScopeID,
			Revision:  reference.Version,
			Digest:    reference.Digest,
		}
	}
	evaluated, err := ruleauthority.Evaluate(domainSets, action)
	if err != nil ||
		evaluated.Kind() != payload.Decision.Kind ||
		evaluated.Digest() != payload.Decision.Digest ||
		evaluated.Timeout() != timeout ||
		evaluated.OnTimeout() != payload.Decision.OnTimeout ||
		!equalProjectionStrings(
			evaluated.ApproverRefs(),
			*payload.Decision.ApproverRefs,
		) ||
		!equalProjectionStrings(
			evaluated.WarningMarkers(),
			warningMarkers,
		) ||
		!equalProjectionStrings(
			evaluated.RecordMarkers(),
			recordMarkers,
		) {
		return ProjectedApprovalRequest{}, fmt.Errorf(
			"decision does not recompute: %w",
			ErrInvalidProjectionEvent,
		)
	}
	return ProjectedApprovalRequest{
		ID:                 payload.ApprovalRequestID,
		Digest:             payload.ApprovalRequestDigest,
		ProjectID:          action.ProjectID(),
		TeamInstanceID:     action.TeamInstanceID(),
		WorkPackageID:      action.WorkPackageID(),
		WorkItemID:         action.WorkItemID(),
		RunID:              action.RunID(),
		AgentInstanceID:    action.AgentInstanceID(),
		LogicalNodeID:      action.LogicalNodeID(),
		AttemptNumber:      action.AttemptNumber(),
		Action:             action.Action(),
		Risk:               action.Risk(),
		ContractDigest:     action.ContractDigest(),
		ActionDigest:       action.Digest(),
		ContinuationDigest: payload.ContinuationDigest,
		DecisionKind:       payload.Decision.Kind,
		DecisionDigest:     payload.Decision.Digest,
		RuleSetReferences:  references,
		ApproverRefs: append(
			[]string(nil),
			(*payload.Decision.ApproverRefs)...,
		),
		Timeout:   timeout,
		OnTimeout: payload.Decision.OnTimeout,
		WarningMarkers: append(
			[]string(nil),
			warningMarkers...,
		),
		RecordMarkers: append(
			[]string(nil),
			recordMarkers...,
		),
		PreviousStatus: "assigned",
		Status:         "pending",
		RequestedAt:    requestedAt,
		ExpiresAt:      expiresAt,
		RequestEventID: event.ID,
	}, nil
}

func approvalWorkStatusFor(status, onTimeout string) string {
	switch status {
	case "approved":
		return "assigned"
	case "cancelled":
		return "cancelled"
	case "expired":
		if onTimeout == "cancel" {
			return "cancelled"
		}
		return "blocked"
	default:
		return "blocked"
	}
}

func validApprovalProjectionTerminal(
	eventType, status, actorRef, authorizationDigest string,
) bool {
	if eventType == "ApprovalExpired" {
		return status == "expired" &&
			actorRef == "" &&
			authorizationDigest == ""
	}
	switch status {
	case "approved", "rejected", "cancelled":
		return eventType == "ApprovalDecided" &&
			validProjectionText(actorRef) &&
			validSHA256Digest(authorizationDigest)
	default:
		return false
	}
}

func parseApprovalProjectionTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.IsZero() || parsed.Location() != time.UTC {
		return time.Time{}, ErrInvalidProjectionEvent
	}
	return parsed, nil
}

func validProjectionText(value string) bool {
	if value == "" || len(value) > 128 ||
		!utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validSortedProjectionTexts(values []string, allowEmpty bool) bool {
	if !allowEmpty && len(values) == 0 {
		return false
	}
	for index, value := range values {
		if !validProjectionText(value) ||
			index > 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}

func equalProjectionStrings(left, right []string) bool {
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

func canonicalApprovalProjectionID(
	actionDigest, continuationDigest, decisionDigest string,
) string {
	sum := sha256.Sum256([]byte(canonicalApprovalProjectionFields(
		"loom.approval-request-id.v1",
		actionDigest,
		continuationDigest,
		decisionDigest,
	)))
	value := append([]byte(nil), sum[:16]...)
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		value[0:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:16],
	)
}

func canonicalApprovalProjectionDigest(
	id, actionDigest, continuationDigest, decisionDigest string,
	requestedAt, expiresAt time.Time,
) string {
	return canonicalApprovalProjectionFields(
		"loom.approval-request.v1",
		id,
		actionDigest,
		continuationDigest,
		decisionDigest,
		"assigned",
		requestedAt.Format(time.RFC3339Nano),
		expiresAt.Format(time.RFC3339Nano),
	)
}

func canonicalApprovalProjectionFields(fields ...string) string {
	hash := sha256.New()
	var length [8]byte
	for _, field := range fields {
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(field))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func cloneProjectedRuleSet(record ProjectedRuleSet) ProjectedRuleSet {
	record.Rules = append([]ProjectedRule(nil), record.Rules...)
	for index := range record.Rules {
		record.Rules[index].ApproverRefs = append(
			[]string(nil),
			record.Rules[index].ApproverRefs...,
		)
	}
	return record
}

func cloneProjectedApprovalRequest(
	record ProjectedApprovalRequest,
) ProjectedApprovalRequest {
	record.RuleSetReferences = append(
		[]ProjectedRuleSetReference(nil),
		record.RuleSetReferences...,
	)
	record.ApproverRefs = append([]string(nil), record.ApproverRefs...)
	record.WarningMarkers = append([]string(nil), record.WarningMarkers...)
	record.RecordMarkers = append([]string(nil), record.RecordMarkers...)
	return record
}
