package rules

// W-AUTONOMY: Standing Orders / Autopilot default-off bounded form. An order
// is a durable, human-authorized automation surface — never execution
// authority. Every lifecycle step is a Journal fact on autonomy/<order_id>;
// dispatch gates on trigger/scope/budget/max_iterations and still funnels the
// call through the B-W1 permissions pipeline (deny/admin-lock/dangerous rules
// are never bypassed).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
)

var (
	ErrInvalidStandingOrder  = errors.New("invalid standing order")
	ErrStandingOrderNotFound = errors.New("standing order not found")
	ErrStandingOrderInactive = errors.New("standing order not active")
	ErrStandingOrderRevoked  = errors.New("standing order revoked")
	ErrStandingOrderExpired  = errors.New("standing order expired")
	ErrStandingOrderStale    = errors.New("standing order generation stale")
	ErrStandingOrderMaxHits  = errors.New("standing order max iterations reached")
	ErrStandingOrderDenied   = errors.New("standing order dispatch denied")
)

type StandingOrderScopeKind string

const (
	StandingScopeJob      StandingOrderScopeKind = "job"
	StandingScopeProject  StandingOrderScopeKind = "project"
	StandingScopePersonal StandingOrderScopeKind = "personal"
)

const (
	StandingEventDefined    = "StandingOrderDefined"
	StandingEventActivated  = "StandingOrderActivated"
	StandingEventDeactivate = "StandingOrderDeactivated"
	StandingEventRevoked    = "StandingOrderRevoked"
	StandingEventExpired    = "StandingOrderExpired"
	StandingEventDispatch   = "StandingOrderDispatch"
	StandingEventBlocked    = "StandingOrderBlocked"
)

const standingOrderPrefix = "autonomy"

// StandingOrder is the frozen durable automation surface.
type StandingOrder struct {
	OrderID       string                 `json:"order_id"`
	Scope         StandingOrderScopeKind `json:"scope"`
	ScopeID       string                 `json:"scope_id"`
	TriggerRuleID string                 `json:"trigger_rule_id"`
	Tool          permissions.ToolKind   `json:"tool"`
	Pattern       string                 `json:"pattern"`
	BudgetRuleID  string                 `json:"budget_rule_id"`
	MaxIterations int64                  `json:"max_iterations"`
	Active        bool                   `json:"active"`
	RevokedAt     string                 `json:"revoked_at,omitempty"`
	ExpiresAt     time.Time              `json:"expires_at,omitempty"`
	AuthorizedBy  string                 `json:"authorized_by,omitempty"`
	Digest        string                 `json:"digest"`
}

// StandingOrderBinding binds a dispatch candidate to a Job/Project lineage.
type StandingOrderBinding struct {
	JobID      string
	ProjectID  string
	Generation int64
	Call       permissions.ProposedCall
}

type standingOrderRecord struct {
	order      StandingOrder
	definedAt  time.Time
	dispatches int64
}

type standingOrderProjection struct {
	orders map[string]standingOrderRecord
}

func standingOrderStream(orderID string) string {
	return standingOrderPrefix + "/" + orderID
}

func validateStandingOrder(order StandingOrder) error {
	if !validRuleText(order.OrderID) || order.OrderID == "" ||
		!validRuleText(order.TriggerRuleID) ||
		!validRuleText(order.BudgetRuleID) ||
		!permissions.ValidToolKind(string(order.Tool)) ||
		order.MaxIterations <= 0 {
		return ErrInvalidStandingOrder
	}
	switch order.Scope {
	case StandingScopeJob, StandingScopeProject, StandingScopePersonal:
	default:
		return ErrInvalidStandingOrder
	}
	if order.Scope != StandingScopePersonal && !validRuleText(order.ScopeID) {
		return ErrInvalidStandingOrder
	}
	return nil
}

func digestStandingOrder(order StandingOrder) string {
	canonical, err := json.Marshal(struct {
		OrderID       string                 `json:"order_id"`
		Scope         StandingOrderScopeKind `json:"scope"`
		ScopeID       string                 `json:"scope_id"`
		TriggerRuleID string                 `json:"trigger_rule_id"`
		Tool          string                 `json:"tool"`
		Pattern       string                 `json:"pattern"`
		BudgetRuleID  string                 `json:"budget_rule_id"`
		MaxIterations int64                  `json:"max_iterations"`
	}{
		OrderID: order.OrderID, Scope: order.Scope, ScopeID: order.ScopeID,
		TriggerRuleID: order.TriggerRuleID, Tool: string(order.Tool),
		Pattern: order.Pattern, BudgetRuleID: order.BudgetRuleID,
		MaxIterations: order.MaxIterations,
	})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// StandingOrderAuthority owns the autonomy fact stream. It consumes the
// W-RULES budget (ConsumeBudget) and trigger rule states; it never decides
// execution — the B-W1 adapter does.
type StandingOrderAuthority struct {
	store     *journal.Store
	customers *Authority
	now       func() time.Time
}

func NewStandingOrderAuthority(
	store *journal.Store,
	customers *Authority,
	now func() time.Time,
) *StandingOrderAuthority {
	return &StandingOrderAuthority{store: store, customers: customers, now: now}
}

// DefineStandingOrder records a new order in the default inactive state.
func (a *StandingOrderAuthority) DefineStandingOrder(
	ctx context.Context,
	order StandingOrder,
	operationID, journeyID string,
) (StandingOrder, error) {
	if ctx == nil || operationID == "" || a == nil || a.store == nil || a.now == nil {
		return StandingOrder{}, ErrInvalidStandingOrder
	}
	if err := validateStandingOrder(order); err != nil {
		return StandingOrder{}, err
	}
	order.Active = false
	order.Digest = digestStandingOrder(order)
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return StandingOrder{}, mapRulesJournalError(err)
	}
	heads := standingStreamHeads(events)
	streamID := standingOrderStream(order.OrderID)
	event := newRulesEvent(
		deterministicEventID(StandingEventDefined, order.OrderID, operationID),
		streamID, heads[streamID]+1, StandingEventDefined,
		a.now().UTC(), journeyID, "",
		order,
	)
	if _, err := a.appendStandingCAS(ctx, heads, streamID, event); err != nil {
		return StandingOrder{}, err
	}
	return order, nil
}

// ActivateStandingOrder turns an order on. It requires an explicit human
// actor (authorizedBy); model/agent output is never a valid activator.
func (a *StandingOrderAuthority) ActivateStandingOrder(
	ctx context.Context,
	orderID, authorizedBy, operationID, journeyID string,
) (StandingOrder, error) {
	if !validRuleText(orderID) || authorizedBy == "" || operationID == "" {
		return StandingOrder{}, ErrInvalidStandingOrder
	}
	projection, err := a.Rebuild(ctx)
	if err != nil {
		return StandingOrder{}, err
	}
	record, ok := projection.orders[orderID]
	if !ok {
		return StandingOrder{}, ErrStandingOrderNotFound
	}
	if record.order.RevokedAt != "" {
		return StandingOrder{}, ErrStandingOrderRevoked
	}
	if !record.order.ExpiresAt.IsZero() && a.now().UTC().After(record.order.ExpiresAt) {
		_ = a.recordStandingFact(ctx, StandingEventExpired, orderID, journeyID,
			standingEventPayload{OrderID: orderID, At: a.now().UTC().Format(time.RFC3339Nano)})
		return StandingOrder{}, ErrStandingOrderExpired
	}
	updated := record.order
	updated.Active = true
	updated.AuthorizedBy = authorizedBy
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return StandingOrder{}, mapRulesJournalError(err)
	}
	heads := standingStreamHeads(events)
	streamID := standingOrderStream(orderID)
	event := newRulesEvent(
		deterministicEventID(StandingEventActivated, orderID, operationID),
		streamID, heads[streamID]+1, StandingEventActivated,
		a.now().UTC(), journeyID, updated.Digest,
		standingEventPayload{OrderID: orderID, AuthorizedBy: authorizedBy,
			At: a.now().UTC().Format(time.RFC3339Nano)},
	)
	if _, err := a.appendStandingCAS(ctx, heads, streamID, event); err != nil {
		return StandingOrder{}, err
	}
	return updated, nil
}

// RevokeStandingOrder immediately stops future dispatch. In-flight calls are
// bounded by the execution adapter, never replayed.
func (a *StandingOrderAuthority) RevokeStandingOrder(
	ctx context.Context,
	orderID, authorizedBy, operationID, journeyID string,
) error {
	if !validRuleText(orderID) || authorizedBy == "" || operationID == "" {
		return ErrInvalidStandingOrder
	}
	projection, err := a.Rebuild(ctx)
	if err != nil {
		return err
	}
	if _, ok := projection.orders[orderID]; !ok {
		return ErrStandingOrderNotFound
	}
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return mapRulesJournalError(err)
	}
	heads := standingStreamHeads(events)
	streamID := standingOrderStream(orderID)
	event := newRulesEvent(
		deterministicEventID(StandingEventRevoked, orderID, operationID),
		streamID, heads[streamID]+1, StandingEventRevoked,
		a.now().UTC(), journeyID, "",
		standingEventPayload{OrderID: orderID, AuthorizedBy: authorizedBy,
			At: a.now().UTC().Format(time.RFC3339Nano)},
	)
	_, err = a.appendStandingCAS(ctx, heads, streamID, event)
	return err
}

// CheckDispatch is the fail-closed gate: every condition must hold, with no
// side effects. Deny/admin-lock/dangerous evaluation belongs to the B-W1
// adapter, which runs after this gate.
func (a *StandingOrderAuthority) CheckDispatch(
	ctx context.Context,
	orderID string,
	binding StandingOrderBinding,
) error {
	projection, err := a.Rebuild(ctx)
	if err != nil {
		return err
	}
	record, ok := projection.orders[orderID]
	if !ok {
		return ErrStandingOrderNotFound
	}
	order := record.order
	if order.RevokedAt != "" {
		return ErrStandingOrderRevoked
	}
	if !order.ExpiresAt.IsZero() && a.now().UTC().After(order.ExpiresAt) {
		return ErrStandingOrderExpired
	}
	if !order.Active {
		return ErrStandingOrderInactive
	}
	if binding.Generation <= 0 {
		return fmt.Errorf("%w: non-positive generation %d", ErrStandingOrderStale, binding.Generation)
	}
	if !standingScopeMatches(order, binding) {
		return fmt.Errorf("%w: scope %s/%s vs job %s project %s",
			ErrStandingOrderDenied, order.Scope, order.ScopeID, binding.JobID, binding.ProjectID)
	}
	if !standingTriggerMatches(order, binding.Call) {
		return fmt.Errorf("%w: trigger rule %s does not match call", ErrStandingOrderDenied, order.TriggerRuleID)
	}
	if err := a.triggerRuleActive(ctx, order.TriggerRuleID); err != nil {
		return err
	}
	if record.dispatches >= order.MaxIterations {
		return ErrStandingOrderMaxHits
	}
	if a.customers != nil {
		available, availErr := a.budgetAvailable(ctx, order.BudgetRuleID, 1)
		if availErr != nil {
			return availErr
		}
		if !available {
			return ErrRuleBudgetExhausted
		}
	}
	return nil
}

// ConsumeDispatch atomically consumes one budget unit and records the
// dispatch iteration. It is called after the B-W1 adapter allowed the call.
func (a *StandingOrderAuthority) ConsumeDispatch(
	ctx context.Context,
	orderID string,
	binding StandingOrderBinding,
	operationID, journeyID string,
) error {
	if a.customers == nil {
		return ErrInvalidStandingOrder
	}
	projection, err := a.Rebuild(ctx)
	if err != nil {
		return err
	}
	record, ok := projection.orders[orderID]
	if !ok {
		return ErrStandingOrderNotFound
	}
	// Idempotent replay guard: ConsumeBudget's own CAS computes a fresh Seq on
	// replay (W-RULES has no key short-circuit), so detect the already-committed
	// budget fact and skip re-consumption.
	expectedKey := deterministicEventID("BudgetConsumed", record.order.BudgetRuleID, operationID)
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return mapRulesJournalError(err)
	}
	alreadyConsumed := false
	for _, event := range events {
		if event.IdempotencyKey == expectedKey {
			alreadyConsumed = true
			break
		}
	}
	if !alreadyConsumed {
		if _, err := a.customers.ConsumeBudget(ctx, record.order.BudgetRuleID, 1, operationID, journeyID); err != nil {
			return err
		}
	}
	heads := standingStreamHeads(events)
	streamID := standingOrderStream(orderID)
	event := newRulesEvent(
		deterministicEventID(StandingEventDispatch, orderID, operationID),
		streamID, heads[streamID]+1, StandingEventDispatch,
		a.now().UTC(), journeyID, record.order.Digest,
		standingDispatchPayload{
			OrderID: orderID, JobID: binding.JobID, Generation: binding.Generation,
			At: a.now().UTC().Format(time.RFC3339Nano),
		},
	)
	_, err = a.appendStandingCAS(ctx, heads, streamID, event)
	return err
}

// budgetAvailable is a read-only, side-effect-free budget check over the
// W-RULES projection (append-only BudgetConsumed facts).
func (a *StandingOrderAuthority) budgetAvailable(
	ctx context.Context,
	ruleID string,
	amount int64,
) (bool, error) {
	if a.customers == nil || amount <= 0 {
		return false, ErrInvalidStandingOrder
	}
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return false, mapRulesJournalError(err)
	}
	projection, err := replayCustomerRules(events)
	if err != nil {
		return false, err
	}
	record, ok := projection.rules[ruleID]
	if !ok {
		return false, ErrCustomerRuleNotFound
	}
	if record.status != "active" {
		return false, ErrCustomerRuleInactive
	}
	rule := record.rule
	window := customerRuleWindow(rule, record.definedAt, a.now().UTC())
	consumed := projection.budget[ruleID][window]
	if rule.BudgetLimit <= 0 {
		return true, nil
	}
	return consumed+amount <= rule.BudgetLimit, nil
}

// Rebuild projects every order from the Journal (read model, not authority).
func (a *StandingOrderAuthority) Rebuild(ctx context.Context) (standingOrderProjection, error) {
	projection := standingOrderProjection{orders: map[string]standingOrderRecord{}}
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return projection, mapRulesJournalError(err)
	}
	for _, event := range events {
		if !strings.HasPrefix(event.StreamID, standingOrderPrefix+"/") {
			continue
		}
		orderID := strings.TrimPrefix(event.StreamID, standingOrderPrefix+"/")
		record, ok := projection.orders[orderID]
		if !ok {
			record = standingOrderRecord{}
			record.order = StandingOrder{OrderID: orderID}
		}
		switch event.Type {
		case StandingEventDefined:
			var order StandingOrder
			if json.Unmarshal(event.PayloadJSON, &order) == nil {
				record.order = order
			}
			record.definedAt = event.EmittedAt
		case StandingEventActivated:
			record.order.Active = true
			var payload standingEventPayload
			if json.Unmarshal(event.PayloadJSON, &payload) == nil {
				record.order.AuthorizedBy = payload.AuthorizedBy
			}
		case StandingEventDeactivate:
			record.order.Active = false
		case StandingEventRevoked:
			record.order.RevokedAt = event.EmittedAt.Format(time.RFC3339Nano)
			record.order.Active = false
		case StandingEventExpired:
			record.order.Active = false
		case StandingEventDispatch:
			record.dispatches++
		}
		projection.orders[orderID] = record
	}
	return projection, nil
}

func (a *StandingOrderAuthority) triggerRuleActive(ctx context.Context, ruleID string) error {
	if a.customers == nil {
		return ErrInvalidStandingOrder
	}
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return mapRulesJournalError(err)
	}
	projection, err := replayCustomerRules(events)
	if err != nil {
		return err
	}
	record, ok := projection.rules[ruleID]
	if !ok {
		return ErrStandingOrderDenied
	}
	if record.status != "active" {
		return ErrStandingOrderDenied
	}
	return nil
}

func (a *StandingOrderAuthority) recordStandingFact(
	ctx context.Context,
	eventType, orderID, journeyID string,
	payload any,
) error {
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return mapRulesJournalError(err)
	}
	heads := standingStreamHeads(events)
	streamID := standingOrderStream(orderID)
	event := newRulesEvent(
		deterministicEventID(eventType, orderID, journeyID, fmt.Sprint(a.now().UTC().UnixNano())),
		streamID, heads[streamID]+1, eventType,
		a.now().UTC(), journeyID, "",
		payload,
	)
	_, err = a.appendStandingCAS(ctx, heads, streamID, event)
	return err
}

func (a *StandingOrderAuthority) appendStandingCAS(
	ctx context.Context,
	heads map[string]int64,
	streamID string,
	event journal.Event,
) ([]journal.Event, error) {
	// Idempotent replay: reuse the committed event verbatim (same Seq) so the
	// immutable event identity stays stable; otherwise the freshly computed
	// Seq would collide with the committed one.
	stream, readErr := a.store.ReadStream(ctx, streamID)
	if readErr != nil {
		return nil, mapRulesJournalError(readErr)
	}
	for _, existing := range stream {
		if existing.IdempotencyKey == event.IdempotencyKey {
			if existing.Type != event.Type ||
				!bytes.Equal(existing.PayloadJSON, event.PayloadJSON) {
				return nil, fmt.Errorf("%w: %s", ErrRuleAuthorityConflict, event.IdempotencyKey)
			}
			return []journal.Event{existing}, nil
		}
	}
	committed, err := a.store.AppendBatchIfStreamHeads(ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: heads[streamID]}},
		[]journal.Event{event},
	)
	if err != nil {
		return nil, mapRulesJournalError(err)
	}
	return committed, nil
}

func standingStreamHeads(events []journal.Event) map[string]int64 {
	heads := map[string]int64{}
	for _, event := range events {
		if event.Seq > heads[event.StreamID] {
			heads[event.StreamID] = event.Seq
		}
	}
	return heads
}

func standingScopeMatches(order StandingOrder, binding StandingOrderBinding) bool {
	switch order.Scope {
	case StandingScopeJob:
		return binding.JobID == order.ScopeID
	case StandingScopeProject:
		return binding.ProjectID == order.ScopeID
	case StandingScopePersonal:
		return true
	default:
		return false
	}
}

func standingTriggerMatches(order StandingOrder, call permissions.ProposedCall) bool {
	if call.Tool != order.Tool {
		return false
	}
	switch call.Tool {
	case permissions.ToolBash:
		return order.Pattern == "" || order.Pattern == "*" ||
			strings.Contains(call.Command, order.Pattern) || call.Command == order.Pattern
	case permissions.ToolEdit, permissions.ToolRead, permissions.ToolGrep:
		return order.Pattern == "" || order.Pattern == "*" ||
			strings.Contains(call.Path, order.Pattern) || call.Path == order.Pattern
	default:
		return false
	}
}

type standingEventPayload struct {
	OrderID      string `json:"order_id"`
	AuthorizedBy string `json:"authorized_by,omitempty"`
	At           string `json:"at"`
}

type standingDispatchPayload struct {
	OrderID    string `json:"order_id"`
	JobID      string `json:"job_id"`
	Generation int64  `json:"generation"`
	At         string `json:"at"`
}
