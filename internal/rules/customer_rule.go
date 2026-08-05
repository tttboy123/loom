package rules

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
)

var (
	ErrInvalidCustomerRule  = errors.New("invalid customer rule")
	ErrCustomerRuleNotFound = errors.New("customer rule not found")
	ErrCustomerRuleInactive = errors.New("customer rule not active")
	ErrRuleBudgetExhausted  = errors.New("customer rule budget exhausted")
	ErrCustomerRuleImport   = errors.New("invalid customer rule import")
)

type CustomerRuleEffectKind string

const (
	EffectRequireApproval CustomerRuleEffectKind = "require_approval"
	EffectReportOnly      CustomerRuleEffectKind = "report_only"
	EffectReject          CustomerRuleEffectKind = "reject"
)

const (
	CustomerScopeProject     = "project"
	CustomerScopeTeam        = "team"
	CustomerScopeWorkPackage = "work_package"
	CustomerScopeWorkItem    = "work_item"
)

type CustomerRule struct {
	RuleID       string                 `json:"rule_id"`
	Scope        string                 `json:"scope"`
	ScopeID      string                 `json:"scope_id"`
	Action       string                 `json:"action,omitempty"`
	Risk         string                 `json:"risk,omitempty"`
	Effect       CustomerRuleEffectKind `json:"effect"`
	ApproverRefs []string               `json:"approver_refs,omitempty"`
	Timeout      time.Duration          `json:"timeout_nanos,omitempty"`
	OnTimeout    string                 `json:"on_timeout,omitempty"`
	BudgetUnit   string                 `json:"budget_unit,omitempty"`
	BudgetLimit  int64                  `json:"budget_limit,omitempty"`
	BudgetPeriod time.Duration          `json:"budget_period_nanos,omitempty"`
	ExpiresAt    time.Time              `json:"expires_at,omitempty"`
	Version      int64                  `json:"version"`
	Digest       string                 `json:"digest"`
}

type CustomerDecision struct {
	Effect          CustomerRuleEffectKind
	RuleID          string
	Matched         bool
	BudgetExhausted bool
}

type customerRuleRecord struct {
	rule      CustomerRule
	status    string // active | revoked | expired
	definedAt time.Time
}

type customerRuleProjection struct {
	rules  map[string]customerRuleRecord
	budget map[string]map[int64]int64 // ruleID -> window -> consumed
}

func customerRuleStream(ruleID string) string { return "customer-rule/" + ruleID }

func validateCustomerRule(rule CustomerRule) error {
	if !validRuleText(rule.RuleID) || !validRuleText(rule.Scope) ||
		!validRuleText(rule.ScopeID) {
		return ErrInvalidCustomerRule
	}
	switch rule.Scope {
	case CustomerScopeProject, CustomerScopeTeam, CustomerScopeWorkPackage, CustomerScopeWorkItem:
	default:
		return ErrInvalidCustomerRule
	}
	if rule.Action != "" && !validRuleText(rule.Action) {
		return ErrInvalidCustomerRule
	}
	if rule.Risk != "" && rule.Risk != "low" && rule.Risk != "medium" && rule.Risk != "high" {
		return ErrInvalidCustomerRule
	}
	switch rule.Effect {
	case EffectRequireApproval:
		if len(rule.ApproverRefs) == 0 ||
			rule.Timeout < time.Second ||
			(rule.OnTimeout != "reject" && rule.OnTimeout != "cancel") {
			return ErrInvalidCustomerRule
		}
	case EffectReportOnly, EffectReject:
		if len(rule.ApproverRefs) != 0 || rule.Timeout != 0 || rule.OnTimeout != "" {
			return ErrInvalidCustomerRule
		}
	default:
		return ErrInvalidCustomerRule
	}
	approvers, ok := normalizedUnique(rule.ApproverRefs, maxApproverRefs)
	if !ok {
		return ErrInvalidCustomerRule
	}
	rule.ApproverRefs = approvers
	if rule.BudgetLimit < 0 || rule.BudgetPeriod < 0 {
		return ErrInvalidCustomerRule
	}
	return nil
}

func digestCustomerRule(rule CustomerRule) string {
	body, _ := json.Marshal(struct {
		RuleID            string                 `json:"rule_id"`
		Scope             string                 `json:"scope"`
		ScopeID           string                 `json:"scope_id"`
		Action            string                 `json:"action"`
		Risk              string                 `json:"risk"`
		Effect            CustomerRuleEffectKind `json:"effect"`
		ApproverRefs      []string               `json:"approver_refs"`
		TimeoutNanos      int64                  `json:"timeout_nanos"`
		OnTimeout         string                 `json:"on_timeout"`
		BudgetUnit        string                 `json:"budget_unit"`
		BudgetLimit       int64                  `json:"budget_limit"`
		BudgetPeriodNanos int64                  `json:"budget_period_nanos"`
		ExpiresAt         string                 `json:"expires_at"`
	}{
		RuleID: rule.RuleID, Scope: rule.Scope, ScopeID: rule.ScopeID,
		Action: rule.Action, Risk: rule.Risk, Effect: rule.Effect,
		ApproverRefs: rule.ApproverRefs, TimeoutNanos: int64(rule.Timeout),
		OnTimeout: rule.OnTimeout, BudgetUnit: rule.BudgetUnit,
		BudgetLimit: rule.BudgetLimit, BudgetPeriodNanos: int64(rule.BudgetPeriod),
		ExpiresAt: rule.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// CustomerRuleRecord is a public snapshot row: rule, lifecycle status, and
// budget consumed in the current window (0 = lifetime).
type CustomerRuleRecord struct {
	Rule     CustomerRule
	Status   string
	Consumed int64
}

// SnapshotCustomerRules rebuilds the customer rule snapshot from Journal
// events (read-only projection; never an authority).
func SnapshotCustomerRules(events []journal.Event) ([]CustomerRuleRecord, error) {
	projection, err := replayCustomerRules(events)
	if err != nil {
		return nil, err
	}
	var records []CustomerRuleRecord
	for ruleID, record := range projection.rules {
		records = append(records, CustomerRuleRecord{
			Rule: record.rule, Status: record.status,
			Consumed: projection.budget[ruleID][0],
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Rule.RuleID < records[j].Rule.RuleID
	})
	return records, nil
}

func customerStreamHeads(events []journal.Event) map[string]int64 {
	heads := make(map[string]int64)
	for _, event := range events {
		if event.Seq > heads[event.StreamID] {
			heads[event.StreamID] = event.Seq
		}
	}
	return heads
}

func replayCustomerRules(events []journal.Event) (*customerRuleProjection, error) {
	projection := &customerRuleProjection{
		rules:  make(map[string]customerRuleRecord),
		budget: make(map[string]map[int64]int64),
	}
	for _, event := range events {
		if !strings.HasPrefix(event.StreamID, "customer-rule/") {
			continue
		}
		ruleID := strings.TrimPrefix(event.StreamID, "customer-rule/")
		switch event.Type {
		case "CustomerRuleDefined", "CustomerRuleRevised":
			var payload struct {
				Rule         CustomerRule `json:"rule"`
				AuthorizedBy string       `json:"authorized_by"`
			}
			if err := decodeExact(event.PayloadJSON, &payload); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidCustomerRule, err)
			}
			projection.rules[ruleID] = customerRuleRecord{rule: payload.Rule, status: "active", definedAt: event.EmittedAt}
		case "CustomerRuleRevoked":
			if record, ok := projection.rules[ruleID]; ok && record.status == "active" {
				record.status = "revoked"
				projection.rules[ruleID] = record
			}
		case "CustomerRuleExpired":
			if record, ok := projection.rules[ruleID]; ok && record.status == "active" {
				record.status = "expired"
				projection.rules[ruleID] = record
			}
		case "RuleMatched", "RuleDenied":
			// audit facts; do not mutate rule state
		case "BudgetConsumed":
			var payload struct {
				RuleID string `json:"rule_id"`
				Window int64  `json:"window"`
				Amount int64  `json:"amount"`
			}
			if err := decodeExact(event.PayloadJSON, &payload); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidCustomerRule, err)
			}
			if projection.budget[payload.RuleID] == nil {
				projection.budget[payload.RuleID] = make(map[int64]int64)
			}
			projection.budget[payload.RuleID][payload.Window] += payload.Amount
		case "RuleBudgetExhausted":
			// marker fact; consumed is derived from BudgetConsumed
		default:
			return nil, fmt.Errorf("%w: unknown event %s", ErrInvalidCustomerRule, event.Type)
		}
	}
	return projection, nil
}

func (a *Authority) latestCustomerRules(ctx context.Context) (*customerRuleProjection, map[string]int64, error) {
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return nil, nil, mapRulesJournalError(err)
	}
	projection, err := replayCustomerRules(events)
	if err != nil {
		return nil, nil, err
	}
	return projection, customerStreamHeads(events), nil
}

func (a *Authority) DefineCustomerRule(
	ctx context.Context,
	rule CustomerRule,
	authorizedBy, operationID, journeyID string,
) ([]journal.Event, error) {
	if rule.Version == 0 {
		rule.Version = 1
	}
	if rule.Effect == EffectRequireApproval && rule.OnTimeout == "" {
		rule.OnTimeout = "reject"
	}
	if err := validateCustomerRule(rule); err != nil {
		return nil, err
	}
	if rule.Scope == CustomerScopeProject && authorizedBy == "" {
		return nil, ErrCustomerAuthorizationRequired
	}
	rule.Digest = digestCustomerRule(rule)
	projection, heads, err := a.latestCustomerRules(ctx)
	if err != nil {
		return nil, err
	}
	streamID := customerRuleStream(rule.RuleID)
	if existing, ok := projection.rules[rule.RuleID]; ok && existing.status == "active" {
		if existing.rule.Digest == rule.Digest && existing.rule.Version == rule.Version {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: rule already defined", ErrInvalidCustomerRule)
	}
	event := newRulesEvent(
		deterministicEventID("CustomerRuleDefined", rule.RuleID, rule.Digest, operationID),
		streamID, heads[streamID]+1, "CustomerRuleDefined",
		a.now().UTC(), journeyID, "",
		struct {
			Rule         CustomerRule `json:"rule"`
			AuthorizedBy string       `json:"authorized_by"`
		}{Rule: rule, AuthorizedBy: authorizedBy},
	)
	return a.appendCustomerRuleCAS(ctx, heads, streamID, event)
}

func (a *Authority) RevokeCustomerRule(
	ctx context.Context,
	ruleID, authorizedBy, operationID, journeyID string,
) ([]journal.Event, error) {
	projection, heads, err := a.latestCustomerRules(ctx)
	if err != nil {
		return nil, err
	}
	record, ok := projection.rules[ruleID]
	if !ok {
		return nil, ErrCustomerRuleNotFound
	}
	if record.status != "active" {
		return nil, ErrCustomerRuleInactive
	}
	if record.rule.Scope == CustomerScopeProject && authorizedBy == "" {
		return nil, ErrCustomerAuthorizationRequired
	}
	streamID := customerRuleStream(ruleID)
	event := newRulesEvent(
		deterministicEventID("CustomerRuleRevoked", ruleID, operationID),
		streamID, heads[streamID]+1, "CustomerRuleRevoked",
		a.now().UTC(), journeyID, record.rule.Digest,
		struct {
			RuleID       string `json:"rule_id"`
			RevokedAt    string `json:"revoked_at"`
			AuthorizedBy string `json:"authorized_by"`
		}{RuleID: ruleID, RevokedAt: a.now().UTC().Format(time.RFC3339Nano), AuthorizedBy: authorizedBy},
	)
	return a.appendCustomerRuleCAS(ctx, heads, streamID, event)
}

func (a *Authority) ExpireCustomerRule(
	ctx context.Context,
	ruleID, operationID, journeyID string,
) ([]journal.Event, error) {
	projection, heads, err := a.latestCustomerRules(ctx)
	if err != nil {
		return nil, err
	}
	record, ok := projection.rules[ruleID]
	if !ok {
		return nil, ErrCustomerRuleNotFound
	}
	if record.status != "active" {
		return nil, ErrCustomerRuleInactive
	}
	streamID := customerRuleStream(ruleID)
	event := newRulesEvent(
		deterministicEventID("CustomerRuleExpired", ruleID, operationID),
		streamID, heads[streamID]+1, "CustomerRuleExpired",
		a.now().UTC(), journeyID, record.rule.Digest,
		struct {
			RuleID    string `json:"rule_id"`
			ExpiredAt string `json:"expired_at"`
		}{RuleID: ruleID, ExpiredAt: a.now().UTC().Format(time.RFC3339Nano)},
	)
	return a.appendCustomerRuleCAS(ctx, heads, streamID, event)
}

func (a *Authority) appendCustomerRuleCAS(
	ctx context.Context,
	heads map[string]int64,
	streamID string,
	event journal.Event,
) ([]journal.Event, error) {
	committed, err := a.store.AppendBatchIfStreamHeads(ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: heads[streamID]}},
		[]journal.Event{event},
	)
	if err != nil {
		return nil, mapRulesJournalError(err)
	}
	return committed, nil
}

func customerRuleWindow(rule CustomerRule, definedAt, now time.Time) int64 {
	if rule.BudgetPeriod <= 0 {
		return 0
	}
	return int64(now.Sub(definedAt) / rule.BudgetPeriod)
}

func (a *Authority) EvaluateCustomerRules(
	ctx context.Context,
	action ActionContext,
	journeyID string,
) (CustomerDecision, error) {
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return CustomerDecision{}, mapRulesJournalError(err)
	}
	projection, err := replayCustomerRules(events)
	if err != nil {
		return CustomerDecision{}, err
	}
	now := a.now().UTC()
	decision := CustomerDecision{}
	type match struct {
		record customerRuleRecord
		window int64
	}
	var matches []match
	for _, record := range projection.rules {
		if record.status != "active" {
			continue
		}
		rule := record.rule
		if !rule.ExpiresAt.IsZero() && now.After(rule.ExpiresAt) {
			continue
		}
		if !customerRuleScopeMatches(rule, action) {
			continue
		}
		if rule.Action != "" && rule.Action != action.Action() {
			continue
		}
		if rule.Risk != "" && rule.Risk != action.Risk() {
			continue
		}
		matches = append(matches, match{record: record, window: customerRuleWindow(rule, record.definedAt, now)})
	}
	sort.Slice(matches, func(i, j int) bool {
		left := customerScopeRank(matches[i].record.rule.Scope)
		right := customerScopeRank(matches[j].record.rule.Scope)
		if left == right {
			return matches[i].record.rule.RuleID < matches[j].record.rule.RuleID
		}
		return left < right
	})
	// budget first: any matched rule over budget => fail-closed reject.
	for _, match := range matches {
		rule := match.record.rule
		consumed := projection.budget[rule.RuleID][match.window]
		if rule.BudgetLimit > 0 && consumed >= rule.BudgetLimit {
			_ = a.recordCustomerRuleFact(ctx, "RuleBudgetExhausted", rule.RuleID, journeyID,
				struct {
					RuleID string `json:"rule_id"`
					Window int64  `json:"window"`
					Limit  int64  `json:"limit"`
				}{RuleID: rule.RuleID, Window: match.window, Limit: rule.BudgetLimit})
			return CustomerDecision{Effect: EffectReject, RuleID: rule.RuleID, Matched: true, BudgetExhausted: true}, nil
		}
	}
	for _, match := range matches {
		rule := match.record.rule
		_ = a.recordCustomerRuleFact(ctx, "RuleMatched", rule.RuleID, journeyID,
			struct {
				RuleID        string                 `json:"rule_id"`
				ContextDigest string                 `json:"context_digest"`
				Effect        CustomerRuleEffectKind `json:"effect"`
				MatchedAt     string                 `json:"matched_at"`
			}{RuleID: rule.RuleID, ContextDigest: action.Digest(), Effect: rule.Effect,
				MatchedAt: now.Format(time.RFC3339Nano)})
		if rule.Effect == EffectReject {
			_ = a.recordCustomerRuleFact(ctx, "RuleDenied", rule.RuleID, journeyID,
				struct {
					RuleID        string `json:"rule_id"`
					ContextDigest string `json:"context_digest"`
					DeniedAt      string `json:"denied_at"`
				}{RuleID: rule.RuleID, ContextDigest: action.Digest(), DeniedAt: now.Format(time.RFC3339Nano)})
			return CustomerDecision{Effect: EffectReject, RuleID: rule.RuleID, Matched: true}, nil
		}
		if rule.Effect == EffectRequireApproval {
			decision = CustomerDecision{Effect: EffectRequireApproval, RuleID: rule.RuleID, Matched: true}
			return decision, nil
		}
		decision = CustomerDecision{Effect: EffectReportOnly, RuleID: rule.RuleID, Matched: true}
	}
	return decision, nil
}

func customerScopeRank(scope string) int {
	switch scope {
	case CustomerScopeWorkItem:
		return 0
	case CustomerScopeWorkPackage:
		return 1
	case CustomerScopeTeam:
		return 2
	case CustomerScopeProject:
		return 3
	default:
		return 4
	}
}

func customerRuleScopeMatches(rule CustomerRule, action ActionContext) bool {
	switch rule.Scope {
	case CustomerScopeProject:
		return rule.ScopeID == action.ProjectID()
	case CustomerScopeTeam:
		return rule.ScopeID == action.TeamInstanceID()
	case CustomerScopeWorkPackage:
		return rule.ScopeID == action.WorkPackageID()
	case CustomerScopeWorkItem:
		return rule.ScopeID == action.WorkItemID()
	default:
		return false
	}
}

func (a *Authority) ConsumeBudget(
	ctx context.Context,
	ruleID string,
	amount int64,
	operationID, journeyID string,
) ([]journal.Event, error) {
	if amount <= 0 {
		return nil, ErrInvalidCustomerRule
	}
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return nil, mapRulesJournalError(err)
	}
	projection, err := replayCustomerRules(events)
	if err != nil {
		return nil, err
	}
	record, ok := projection.rules[ruleID]
	if !ok {
		return nil, ErrCustomerRuleNotFound
	}
	if record.status != "active" {
		return nil, ErrCustomerRuleInactive
	}
	rule := record.rule
	window := customerRuleWindow(rule, record.definedAt, a.now().UTC())
	consumed := projection.budget[ruleID][window]
	if rule.BudgetLimit > 0 && consumed+amount > rule.BudgetLimit {
		_ = a.recordCustomerRuleFact(ctx, "RuleBudgetExhausted", ruleID, journeyID,
			struct {
				RuleID string `json:"rule_id"`
				Window int64  `json:"window"`
				Limit  int64  `json:"limit"`
			}{RuleID: ruleID, Window: window, Limit: rule.BudgetLimit})
		return nil, ErrRuleBudgetExhausted
	}
	heads := customerStreamHeads(events)
	streamID := customerRuleStream(ruleID)
	event := newRulesEvent(
		deterministicEventID("BudgetConsumed", ruleID, operationID),
		streamID, heads[streamID]+1, "BudgetConsumed",
		a.now().UTC(), journeyID, record.rule.Digest,
		struct {
			RuleID string `json:"rule_id"`
			Window int64  `json:"window"`
			Amount int64  `json:"amount"`
		}{RuleID: ruleID, Window: window, Amount: amount},
	)
	return a.appendCustomerRuleCAS(ctx, heads, streamID, event)
}

func (a *Authority) recordCustomerRuleFact(
	ctx context.Context,
	eventType, ruleID, journeyID string,
	payload any,
) error {
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return err
	}
	heads := customerStreamHeads(events)
	streamID := customerRuleStream(ruleID)
	event := newRulesEvent(
		deterministicEventID(eventType, ruleID, journeyID, strconv.FormatInt(time.Now().UnixNano(), 10)),
		streamID, heads[streamID]+1, eventType,
		a.now().UTC(), journeyID, "",
		payload,
	)
	_, err = a.appendCustomerRuleCAS(ctx, heads, streamID, event)
	return err
}

// ImportPermissionsTOML parses a bounded TOML subset and defines each rule as
// a Journal fact. Idempotent per (operationID, ruleID, digest).
func (a *Authority) ImportPermissionsTOML(
	ctx context.Context,
	content []byte,
	authorizedBy, operationID, journeyID string,
) ([]journal.Event, error) {
	rules, err := parsePermissionsTOML(content)
	if err != nil {
		return nil, err
	}
	var all []journal.Event
	for index, rule := range rules {
		rule.RuleID = strings.TrimSpace(rule.RuleID)
		if rule.RuleID == "" {
			rule.RuleID = fmt.Sprintf("import-%s-%d", operationID, index)
		}
		events, err := a.DefineCustomerRule(ctx, rule, authorizedBy, operationID+"-"+strconv.Itoa(index), journeyID)
		if err != nil {
			return nil, err
		}
		all = append(all, events...)
	}
	return all, nil
}

func parsePermissionsTOML(content []byte) ([]CustomerRule, error) {
	var rules []CustomerRule
	var current *CustomerRule
	finish := func() {
		if current != nil {
			rules = append(rules, *current)
		}
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key, value := fields[0], strings.Join(fields[1:], " ")
		value = strings.Trim(value, `"'`)
		switch key {
		case "rule":
			finish()
			if len(fields) < 4 {
				return nil, ErrCustomerRuleImport
			}
			current = &CustomerRule{RuleID: strings.Trim(fields[1], `"'`),
				Scope: strings.Trim(fields[2], `"'`), ScopeID: strings.Trim(fields[3], `"'`)}
		case "action":
			if current == nil {
				return nil, ErrCustomerRuleImport
			}
			current.Action = value
		case "risk":
			if current == nil {
				return nil, ErrCustomerRuleImport
			}
			current.Risk = value
		case "effect":
			if current == nil {
				return nil, ErrCustomerRuleImport
			}
			current.Effect = CustomerRuleEffectKind(value)
		case "approver":
			if current == nil {
				return nil, ErrCustomerRuleImport
			}
			current.ApproverRefs = append(current.ApproverRefs, value)
		case "timeout":
			if current == nil {
				return nil, ErrCustomerRuleImport
			}
			seconds, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, ErrCustomerRuleImport
			}
			current.Timeout = time.Duration(seconds) * time.Second
		case "on_timeout":
			if current == nil {
				return nil, ErrCustomerRuleImport
			}
			current.OnTimeout = value
		case "budget":
			if current == nil || len(fields) < 3 {
				return nil, ErrCustomerRuleImport
			}
			current.BudgetUnit = strings.Trim(fields[1], `"'`)
			limit, err := strconv.ParseInt(strings.Trim(fields[2], `"'`), 10, 64)
			if err != nil {
				return nil, ErrCustomerRuleImport
			}
			current.BudgetLimit = limit
			if len(fields) >= 5 && fields[3] == "period" {
				seconds, err := strconv.ParseInt(strings.Trim(fields[4], `"'`), 10, 64)
				if err != nil {
					return nil, ErrCustomerRuleImport
				}
				current.BudgetPeriod = time.Duration(seconds) * time.Second
			}
		case "expires":
			if current == nil {
				return nil, ErrCustomerRuleImport
			}
			unix, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, ErrCustomerRuleImport
			}
			current.ExpiresAt = time.Unix(unix, 0).UTC()
		}
	}
	finish()
	if len(rules) == 0 {
		return nil, ErrCustomerRuleImport
	}
	return rules, nil
}
