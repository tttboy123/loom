package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/rules"
)

var (
	ErrInvalidCustomerRuleRequest = errors.New("invalid local customer rule request")
)

type CustomerRuleSnapshotRequest struct {
	JourneyID string `json:"-"`
}

type CustomerRuleView struct {
	Rule     rules.CustomerRule `json:"rule"`
	Status   string             `json:"status"`
	Consumed int64              `json:"consumed"`
}

type CustomerRuleSnapshot struct {
	ViewVersion string             `json:"view_version"`
	Rules       []CustomerRuleView `json:"rules"`
}

type CustomerRuleCommandRequest struct {
	JourneyID   string          `json:"-"`
	OperationID string          `json:"operation_id"`
	Action      string          `json:"action"`
	Input       json.RawMessage `json:"input"`
}

type CustomerRuleCommandResult struct {
	OperationID string                 `json:"operation_id"`
	Action      string                 `json:"action"`
	ViewVersion string                 `json:"view_version"`
	EventIDs    []string               `json:"event_ids,omitempty"`
	Decision    rules.CustomerDecision `json:"decision,omitempty"`
	Note        string                 `json:"note,omitempty"`
}

type LocalCustomerRuleService struct {
	store       *journal.Store
	now         func() time.Time
	viewVersion func() string
	authority   *rules.Authority
}

func NewLocalCustomerRuleService(
	store *journal.Store,
	now func() time.Time,
	viewVersion func() string,
	authority *rules.Authority,
) (*LocalCustomerRuleService, error) {
	if store == nil || now == nil || viewVersion == nil || authority == nil {
		return nil, ErrInvalidCustomerRuleRequest
	}
	return &LocalCustomerRuleService{
		store: store, now: now, viewVersion: viewVersion, authority: authority,
	}, nil
}

func (service *LocalCustomerRuleService) Snapshot(
	ctx context.Context,
	request CustomerRuleSnapshotRequest,
) (CustomerRuleSnapshot, error) {
	if service == nil || service.store == nil {
		return CustomerRuleSnapshot{}, ErrInvalidCustomerRuleRequest
	}
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return CustomerRuleSnapshot{}, err
	}
	records, err := rules.SnapshotCustomerRules(events)
	if err != nil {
		return CustomerRuleSnapshot{}, err
	}
	snapshot := CustomerRuleSnapshot{ViewVersion: service.viewVersion()}
	for _, record := range records {
		snapshot.Rules = append(snapshot.Rules, CustomerRuleView{
			Rule: record.Rule, Status: record.Status, Consumed: record.Consumed,
		})
	}
	return snapshot, nil
}

func (service *LocalCustomerRuleService) Command(
	ctx context.Context,
	request CustomerRuleCommandRequest,
) (CustomerRuleCommandResult, error) {
	if service == nil || service.authority == nil {
		return CustomerRuleCommandResult{}, ErrInvalidCustomerRuleRequest
	}
	if request.OperationID == "" || request.Action == "" {
		return CustomerRuleCommandResult{}, ErrInvalidCustomerRuleRequest
	}
	var events []journal.Event
	var err error
	switch request.Action {
	case "define":
		var input rules.CustomerRule
		if err = decodeStrictCustomerRuleInput(request.Input, &input); err != nil {
			return CustomerRuleCommandResult{}, ErrInvalidCustomerRuleRequest
		}
		events, err = service.authority.DefineCustomerRule(ctx, input, "tui-user", request.OperationID, request.JourneyID)
	case "revoke":
		var input struct {
			RuleID string `json:"rule_id"`
		}
		if err = decodeStrictCustomerRuleInput(request.Input, &input); err != nil {
			return CustomerRuleCommandResult{}, ErrInvalidCustomerRuleRequest
		}
		events, err = service.authority.RevokeCustomerRule(ctx, input.RuleID, "tui-user", request.OperationID, request.JourneyID)
	case "expire":
		var input struct {
			RuleID string `json:"rule_id"`
		}
		if err = decodeStrictCustomerRuleInput(request.Input, &input); err != nil {
			return CustomerRuleCommandResult{}, ErrInvalidCustomerRuleRequest
		}
		events, err = service.authority.ExpireCustomerRule(ctx, input.RuleID, request.OperationID, request.JourneyID)
	case "import":
		var input struct {
			Content string `json:"content"`
		}
		if err = decodeStrictCustomerRuleInput(request.Input, &input); err != nil {
			return CustomerRuleCommandResult{}, ErrInvalidCustomerRuleRequest
		}
		events, err = service.authority.ImportPermissionsTOML(ctx, []byte(input.Content), "tui-user", request.OperationID, request.JourneyID)
	case "evaluate":
		var input struct {
			Action rules.ActionContextInput `json:"action"`
		}
		if err = decodeStrictCustomerRuleInput(request.Input, &input); err != nil {
			return CustomerRuleCommandResult{}, ErrInvalidCustomerRuleRequest
		}
		action, actionErr := rules.NewActionContext(input.Action)
		if actionErr != nil {
			return CustomerRuleCommandResult{}, actionErr
		}
		decision, evalErr := service.authority.EvaluateCustomerRules(ctx, action, request.JourneyID)
		if evalErr != nil {
			return CustomerRuleCommandResult{}, evalErr
		}
		return CustomerRuleCommandResult{
			OperationID: request.OperationID, Action: request.Action,
			ViewVersion: service.viewVersion(), Decision: decision,
			Note: fmt.Sprintf("customer rule effect %s", decision.Effect),
		}, nil
	case "consume_budget":
		var input struct {
			RuleID string `json:"rule_id"`
			Amount int64  `json:"amount"`
		}
		if err = decodeStrictCustomerRuleInput(request.Input, &input); err != nil {
			return CustomerRuleCommandResult{}, ErrInvalidCustomerRuleRequest
		}
		events, err = service.authority.ConsumeBudget(ctx, input.RuleID, input.Amount, request.OperationID, request.JourneyID)
	default:
		return CustomerRuleCommandResult{}, fmt.Errorf("%w: unknown action %q", ErrInvalidCustomerRuleRequest, request.Action)
	}
	if err != nil {
		return CustomerRuleCommandResult{}, err
	}
	eventIDs := make([]string, 0, len(events))
	for _, event := range events {
		eventIDs = append(eventIDs, event.ID)
	}
	return CustomerRuleCommandResult{
		OperationID: request.OperationID, Action: request.Action,
		ViewVersion: service.viewVersion(), EventIDs: eventIDs,
	}, nil
}

func decodeStrictCustomerRuleInput(data []byte, target any) error {
	return decodeExactPermissionParams(data, target)
}
