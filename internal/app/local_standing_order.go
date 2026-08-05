package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/rules"
)

var ErrInvalidStandingOrderRequest = errors.New("invalid local standing order request")

type StandingOrderSnapshotRequest struct {
	JourneyID string `json:"-"`
}

type StandingOrderView struct {
	Order      rules.StandingOrder `json:"order"`
	Dispatches int64               `json:"dispatches"`
}

type StandingOrderSnapshot struct {
	ViewVersion string              `json:"view_version"`
	Orders      []StandingOrderView `json:"orders"`
	Autopilot   bool                `json:"autopilot"`
}

type StandingOrderCommandRequest struct {
	JourneyID   string          `json:"-"`
	OperationID string          `json:"operation_id"`
	Action      string          `json:"action"`
	Input       json.RawMessage `json:"input"`
}

type StandingOrderCommandResult struct {
	OperationID string              `json:"operation_id"`
	Action      string              `json:"action"`
	ViewVersion string              `json:"view_version"`
	EventIDs    []string            `json:"event_ids,omitempty"`
	Order       rules.StandingOrder `json:"order,omitempty"`
	Note        string              `json:"note,omitempty"`
}

type LocalStandingOrderService struct {
	store       *journal.Store
	now         func() time.Time
	viewVersion func() string
	authority   *rules.StandingOrderAuthority
}

func NewLocalStandingOrderService(
	store *journal.Store,
	now func() time.Time,
	viewVersion func() string,
	authority *rules.StandingOrderAuthority,
) (*LocalStandingOrderService, error) {
	if store == nil || now == nil || viewVersion == nil || authority == nil {
		return nil, ErrInvalidStandingOrderRequest
	}
	return &LocalStandingOrderService{
		store: store, now: now, viewVersion: viewVersion, authority: authority,
	}, nil
}

func (service *LocalStandingOrderService) Snapshot(
	ctx context.Context,
	request StandingOrderSnapshotRequest,
) (StandingOrderSnapshot, error) {
	if service == nil || service.store == nil {
		return StandingOrderSnapshot{}, ErrInvalidStandingOrderRequest
	}
	projection, err := service.authority.Rebuild(ctx)
	if err != nil {
		return StandingOrderSnapshot{}, err
	}
	snapshot := StandingOrderSnapshot{ViewVersion: service.viewVersion()}
	for _, record := range projection.Orders() {
		snapshot.Orders = append(snapshot.Orders, StandingOrderView{
			Order: record.Order, Dispatches: record.Dispatches,
		})
	}
	return snapshot, nil
}

func (service *LocalStandingOrderService) Command(
	ctx context.Context,
	request StandingOrderCommandRequest,
) (StandingOrderCommandResult, error) {
	if service == nil || service.store == nil ||
		request.OperationID == "" {
		return StandingOrderCommandResult{}, ErrInvalidStandingOrderRequest
	}
	result := StandingOrderCommandResult{
		OperationID: request.OperationID, Action: request.Action,
		ViewVersion: service.viewVersion(),
	}
	switch request.Action {
	case "define":
		var input struct {
			OrderID       string `json:"order_id"`
			Scope         string `json:"scope"`
			ScopeID       string `json:"scope_id"`
			TriggerRuleID string `json:"trigger_rule_id"`
			Tool          string `json:"tool"`
			Pattern       string `json:"pattern"`
			BudgetRuleID  string `json:"budget_rule_id"`
			MaxIterations int64  `json:"max_iterations"`
		}
		if decodeStandingOrderInput(request.Input, &input) != nil {
			return StandingOrderCommandResult{}, ErrInvalidStandingOrderRequest
		}
		order := rules.StandingOrder{
			OrderID: input.OrderID, Scope: rules.StandingOrderScopeKind(input.Scope),
			ScopeID: input.ScopeID, TriggerRuleID: input.TriggerRuleID,
			Tool: permissions.ToolKind(input.Tool), Pattern: input.Pattern,
			BudgetRuleID: input.BudgetRuleID, MaxIterations: input.MaxIterations,
		}
		defined, err := service.authority.DefineStandingOrder(ctx, order,
			request.OperationID, request.JourneyID)
		if err != nil {
			return StandingOrderCommandResult{}, err
		}
		result.Order = defined
		result.Note = "defined; default inactive (Autopilot off)"
		return result, nil
	case "activate":
		var input struct {
			OrderID      string `json:"order_id"`
			AuthorizedBy string `json:"authorized_by"`
		}
		if decodeStandingOrderInput(request.Input, &input) != nil {
			return StandingOrderCommandResult{}, ErrInvalidStandingOrderRequest
		}
		activated, err := service.authority.ActivateStandingOrder(ctx,
			input.OrderID, input.AuthorizedBy, request.OperationID, request.JourneyID)
		if err != nil {
			return StandingOrderCommandResult{}, err
		}
		result.Order = activated
		result.Note = "activated by " + input.AuthorizedBy
		return result, nil
	case "revoke":
		var input struct {
			OrderID      string `json:"order_id"`
			AuthorizedBy string `json:"authorized_by"`
		}
		if decodeStandingOrderInput(request.Input, &input) != nil {
			return StandingOrderCommandResult{}, ErrInvalidStandingOrderRequest
		}
		if err := service.authority.RevokeStandingOrder(ctx,
			input.OrderID, input.AuthorizedBy, request.OperationID, request.JourneyID); err != nil {
			return StandingOrderCommandResult{}, err
		}
		result.Note = "revoked by " + input.AuthorizedBy
		return result, nil
	default:
		return StandingOrderCommandResult{}, fmt.Errorf("%w: unknown action %s",
			ErrInvalidStandingOrderRequest, request.Action)
	}
}

func decodeStandingOrderInput(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return ErrInvalidStandingOrderRequest
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	return nil
}
