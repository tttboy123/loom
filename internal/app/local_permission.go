package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
)

var (
	ErrInvalidPermissionRequest   = errors.New("invalid local permission request")
	ErrPermissionStateUnavailable = errors.New("permission state unavailable")
)

type PermissionSnapshotRequest struct {
	JourneyID string `json:"-"`
}

type PermissionSnapshot struct {
	ViewVersion string                          `json:"view_version"`
	Profiles    []permissions.PermissionProfile `json:"profiles"`
	Bindings    []permissions.JobBinding        `json:"bindings"`
	Rules       []permissions.Rule              `json:"rules"`
	Grants      []permissions.Grant             `json:"grants"`
	Activations []permissions.Activation        `json:"activations"`
	AdminLock   bool                            `json:"admin_lock"`
}

type PermissionAttentionRequest struct {
	JourneyID string `json:"-"`
}

type PermissionDecisionView struct {
	JobID             string              `json:"job_id"`
	ApprovalID        string              `json:"approval_id,omitempty"`
	Verdict           permissions.Verdict `json:"verdict"`
	Reason            string              `json:"reason"`
	AuthorizationPath string              `json:"authorization_path"`
	RecordedAt        string              `json:"recorded_at"`
}

type PermissionAttention struct {
	ViewVersion string                   `json:"view_version"`
	Decisions   []PermissionDecisionView `json:"decisions"`
}

type PermissionCommandRequest struct {
	JourneyID   string          `json:"-"`
	OperationID string          `json:"operation_id"`
	Action      string          `json:"action"`
	Input       json.RawMessage `json:"input"`
}

type PermissionCommandResult struct {
	OperationID string              `json:"operation_id"`
	Action      string              `json:"action"`
	ViewVersion string              `json:"view_version"`
	EventIDs    []string            `json:"event_ids,omitempty"`
	Verdict     permissions.Verdict `json:"verdict,omitempty"`
	Denial      permissions.Denial  `json:"denial,omitempty"`
	Note        string              `json:"note,omitempty"`
}

type LocalPermissionService struct {
	store       *journal.Store
	now         func() time.Time
	viewVersion func() string
	authority   *permissions.Authority
}

func NewLocalPermissionService(
	store *journal.Store,
	now func() time.Time,
	viewVersion func() string,
) (*LocalPermissionService, error) {
	if store == nil || now == nil || viewVersion == nil {
		return nil, ErrInvalidPermissionRequest
	}
	authority, err := permissions.NewAuthority(store, now)
	if err != nil {
		return nil, err
	}
	return &LocalPermissionService{
		store: store, now: now, viewVersion: viewVersion, authority: authority,
	}, nil
}

func (service *LocalPermissionService) projection(ctx context.Context) (*permissions.Projection, []journal.Event, error) {
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return nil, nil, err
	}
	projection, err := permissions.Replay(events)
	if err != nil {
		return nil, nil, err
	}
	return projection, events, nil
}

func (service *LocalPermissionService) PermissionSnapshot(
	ctx context.Context,
	request PermissionSnapshotRequest,
) (PermissionSnapshot, error) {
	if service == nil || service.store == nil {
		return PermissionSnapshot{}, ErrInvalidPermissionRequest
	}
	projection, _, err := service.projection(ctx)
	if err != nil {
		return PermissionSnapshot{}, err
	}
	profiles := make([]permissions.PermissionProfile, 0, len(projection.Profiles))
	for _, profile := range projection.Profiles {
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ProfileID < profiles[j].ProfileID })
	bindings := make([]permissions.JobBinding, 0, len(projection.Bindings))
	for _, binding := range projection.Bindings {
		bindings = append(bindings, binding)
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].JobID < bindings[j].JobID })
	rules := make([]permissions.Rule, 0, len(projection.Rules))
	for _, rule := range projection.Rules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].RuleID < rules[j].RuleID })
	grants := make([]permissions.Grant, 0, len(projection.Grants))
	for _, grant := range projection.Grants {
		grants = append(grants, grant)
	}
	sort.Slice(grants, func(i, j int) bool { return grants[i].GrantID < grants[j].GrantID })
	activations := make([]permissions.Activation, 0, len(projection.Activations))
	for _, activation := range projection.Activations {
		activations = append(activations, activation)
	}
	sort.Slice(activations, func(i, j int) bool {
		left := string(activations[i].Scope) + activations[i].ScopeID
		right := string(activations[j].Scope) + activations[j].ScopeID
		return left < right
	})
	return PermissionSnapshot{
		ViewVersion: service.viewVersion(),
		Profiles:    profiles, Bindings: bindings, Rules: rules,
		Grants: grants, Activations: activations, AdminLock: projection.AdminLock,
	}, nil
}

func (service *LocalPermissionService) PermissionAttention(
	ctx context.Context,
	request PermissionAttentionRequest,
) (PermissionAttention, error) {
	if service == nil || service.store == nil {
		return PermissionAttention{}, ErrInvalidPermissionRequest
	}
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return PermissionAttention{}, err
	}
	var decisions []PermissionDecisionView
	for _, event := range events {
		if event.Type != "PermissionDecisionRecorded" {
			continue
		}
		var payload struct {
			JobID             string `json:"job_id"`
			ApprovalID        string `json:"approval_id,omitempty"`
			Verdict           string `json:"verdict"`
			Reason            string `json:"reason"`
			AuthorizationPath string `json:"authorization_path"`
			RecordedAt        string `json:"recorded_at"`
		}
		if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
			continue
		}
		if payload.Verdict != string(permissions.VerdictAsk) {
			continue
		}
		decisions = append(decisions, PermissionDecisionView{
			JobID: payload.JobID, ApprovalID: payload.ApprovalID,
			Verdict: permissions.VerdictAsk, Reason: payload.Reason,
			AuthorizationPath: payload.AuthorizationPath, RecordedAt: payload.RecordedAt,
		})
	}
	sort.Slice(decisions, func(i, j int) bool { return decisions[i].RecordedAt < decisions[j].RecordedAt })
	return PermissionAttention{ViewVersion: service.viewVersion(), Decisions: decisions}, nil
}

func (service *LocalPermissionService) PermissionCommand(
	ctx context.Context,
	request PermissionCommandRequest,
) (PermissionCommandResult, error) {
	if service == nil || service.authority == nil {
		return PermissionCommandResult{}, ErrInvalidPermissionRequest
	}
	if request.OperationID == "" || request.Action == "" {
		return PermissionCommandResult{}, ErrInvalidPermissionRequest
	}
	var events []journal.Event
	var err error
	switch request.Action {
	case "define_profile":
		var input permissions.ProfileInput
		if err = decodeExactPermissionParams(request.Input, &input); err != nil {
			return PermissionCommandResult{}, ErrInvalidPermissionRequest
		}
		events, err = service.authority.DefineProfile(ctx, input, request.OperationID, request.JourneyID)
	case "revise_profile":
		var input struct {
			ProfileID string                   `json:"profile_id"`
			Input     permissions.ProfileInput `json:"input"`
		}
		if err = decodeExactPermissionParams(request.Input, &input); err != nil {
			return PermissionCommandResult{}, ErrInvalidPermissionRequest
		}
		events, err = service.authority.ReviseProfile(ctx, input.ProfileID, input.Input, request.OperationID, request.JourneyID)
	case "retire_profile":
		var input struct {
			ProfileID string `json:"profile_id"`
		}
		if err = decodeExactPermissionParams(request.Input, &input); err != nil {
			return PermissionCommandResult{}, ErrInvalidPermissionRequest
		}
		events, err = service.authority.RetireProfile(ctx, input.ProfileID, request.OperationID, request.JourneyID)
	case "add_rule":
		var input struct {
			Rule         permissions.Rule `json:"rule"`
			AuthorizedBy string           `json:"authorized_by"`
		}
		if err = decodeExactPermissionParams(request.Input, &input); err != nil {
			return PermissionCommandResult{}, ErrInvalidPermissionRequest
		}
		events, err = service.authority.AddRule(ctx, input.Rule, input.AuthorizedBy, request.OperationID, request.JourneyID)
	case "revoke_rule":
		var input struct {
			RuleID       string `json:"rule_id"`
			AuthorizedBy string `json:"authorized_by"`
		}
		if err = decodeExactPermissionParams(request.Input, &input); err != nil {
			return PermissionCommandResult{}, ErrInvalidPermissionRequest
		}
		events, err = service.authority.RevokeRule(ctx, input.RuleID, input.AuthorizedBy, request.OperationID, request.JourneyID)
	case "bind_job":
		var input struct {
			JobID     string `json:"job_id"`
			ProfileID string `json:"profile_id"`
		}
		if err = decodeExactPermissionParams(request.Input, &input); err != nil {
			return PermissionCommandResult{}, ErrInvalidPermissionRequest
		}
		events, err = service.authority.BindJob(ctx, input.JobID, input.ProfileID, request.OperationID, request.JourneyID)
	case "issue_grant":
		var input permissions.Grant
		if err = decodeExactPermissionParams(request.Input, &input); err != nil {
			return PermissionCommandResult{}, ErrInvalidPermissionRequest
		}
		events, err = service.authority.IssueGrant(ctx, input, request.OperationID, request.JourneyID)
	case "revoke_grant":
		var input struct {
			GrantID string `json:"grant_id"`
		}
		if err = decodeExactPermissionParams(request.Input, &input); err != nil {
			return PermissionCommandResult{}, ErrInvalidPermissionRequest
		}
		events, err = service.authority.RevokeGrant(ctx, input.GrantID, request.OperationID, request.JourneyID)
	case "activate_mode":
		var input struct {
			Mode         permissions.Mode      `json:"mode"`
			Scope        permissions.ScopeKind `json:"scope"`
			ScopeID      string                `json:"scope_id"`
			AuthorizedBy string                `json:"authorized_by"`
		}
		if err = decodeExactPermissionParams(request.Input, &input); err != nil {
			return PermissionCommandResult{}, ErrInvalidPermissionRequest
		}
		events, err = service.authority.ActivateMode(ctx, input.Mode, input.Scope, input.ScopeID, input.AuthorizedBy, request.OperationID, request.JourneyID)
	case "set_admin_lock":
		var input struct {
			Enabled      bool   `json:"enabled"`
			AuthorizedBy string `json:"authorized_by"`
		}
		if err = decodeExactPermissionParams(request.Input, &input); err != nil {
			return PermissionCommandResult{}, ErrInvalidPermissionRequest
		}
		events, err = service.authority.SetAdminLock(ctx, input.Enabled, input.AuthorizedBy, request.OperationID, request.JourneyID)
	case "resolve_approval":
		var input struct {
			ApprovalID string `json:"approval_id"`
			Resolution string `json:"resolution"`
			ResolvedBy string `json:"resolved_by"`
		}
		if err = decodeExactPermissionParams(request.Input, &input); err != nil {
			return PermissionCommandResult{}, ErrInvalidPermissionRequest
		}
		_, err = service.authority.ResolveApproval(ctx, input.ApprovalID, input.Resolution, input.ResolvedBy, request.OperationID, request.JourneyID)
		if err != nil {
			return PermissionCommandResult{}, err
		}
	case "validate_call":
		return service.validateCall(ctx, request)
	default:
		return PermissionCommandResult{}, fmt.Errorf("%w: unknown action %q", ErrInvalidPermissionRequest, request.Action)
	}
	if err != nil {
		return PermissionCommandResult{}, err
	}
	eventIDs := make([]string, 0, len(events))
	for _, event := range events {
		eventIDs = append(eventIDs, event.ID)
	}
	return PermissionCommandResult{
		OperationID: request.OperationID, Action: request.Action,
		ViewVersion: service.viewVersion(), EventIDs: eventIDs,
	}, nil
}

func (service *LocalPermissionService) validateCall(
	ctx context.Context,
	request PermissionCommandRequest,
) (PermissionCommandResult, error) {
	var input struct {
		JobID string                   `json:"job_id"`
		Call  permissions.ProposedCall `json:"call"`
	}
	if err := decodeExactPermissionParams(request.Input, &input); err != nil {
		return PermissionCommandResult{}, ErrInvalidPermissionRequest
	}
	projection, _, err := service.projection(ctx)
	if err != nil {
		return PermissionCommandResult{}, err
	}
	effective, err := permissions.ResolveEffectiveProfile(projection, input.JobID)
	if err != nil {
		return PermissionCommandResult{}, err
	}
	verdict, denial, err := permissions.Evaluate(effective, input.Call)
	if err != nil {
		return PermissionCommandResult{}, err
	}
	result := PermissionCommandResult{
		OperationID: request.OperationID, Action: request.Action,
		ViewVersion: service.viewVersion(), Verdict: verdict, Denial: denial,
	}
	if verdict == permissions.VerdictAsk {
		events, err := service.authority.RecordDecision(
			ctx, input.JobID, verdict, denial, "", request.OperationID, request.JourneyID,
		)
		if err != nil {
			return PermissionCommandResult{}, err
		}
		for _, event := range events {
			result.EventIDs = append(result.EventIDs, event.ID)
		}
		result.Note = "approval required; resolution is forwarded to the existing rules authority"
	}
	return result, nil
}

func decodeExactPermissionParams(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing data")
	}
	return nil
}
