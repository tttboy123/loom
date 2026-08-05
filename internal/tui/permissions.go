package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/permissions"
)

type PermissionClient interface {
	PermissionSnapshot(context.Context, app.PermissionSnapshotRequest) (app.PermissionSnapshot, error)
	PermissionAttention(context.Context, app.PermissionAttentionRequest) (app.PermissionAttention, error)
	PermissionCommand(context.Context, app.PermissionCommandRequest) (app.PermissionCommandResult, error)
}

func permissionClientFrom(client ReadClient) PermissionClient {
	permissionClient, _ := client.(PermissionClient)
	return permissionClient
}

func (client *DaemonReadClient) PermissionSnapshot(
	ctx context.Context,
	request app.PermissionSnapshotRequest,
) (app.PermissionSnapshot, error) {
	var snapshot app.PermissionSnapshot
	err := client.client.CallJourney(
		ctx, request.JourneyID, "permissions_snapshot", request, &snapshot,
	)
	return snapshot, err
}

func (client *DaemonReadClient) PermissionAttention(
	ctx context.Context,
	request app.PermissionAttentionRequest,
) (app.PermissionAttention, error) {
	var attention app.PermissionAttention
	err := client.client.CallJourney(
		ctx, request.JourneyID, "permissions_attention", request, &attention,
	)
	return attention, err
}

func (client *DaemonReadClient) PermissionCommand(
	ctx context.Context,
	request app.PermissionCommandRequest,
) (app.PermissionCommandResult, error) {
	var result app.PermissionCommandResult
	err := client.client.CallJourney(
		ctx, request.JourneyID, "permissions_command", request, &result,
	)
	return result, err
}

type permissionsLoadedMsg struct {
	snapshot app.PermissionSnapshot
}

type permissionsFailedMsg struct {
	err error
}

type permissionAttentionLoadedMsg struct {
	attention app.PermissionAttention
}

type permissionAttentionFailedMsg struct {
	err error
}

type permissionCommandDoneMsg struct {
	err  error
	note string
}

func (model Model) loadPermissions() tea.Cmd {
	client, ctx, journeyID := model.permissionClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return permissionsFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		snapshot, err := client.PermissionSnapshot(ctx, app.PermissionSnapshotRequest{
			JourneyID: journeyID,
		})
		if err != nil {
			return permissionsFailedMsg{err: err}
		}
		return permissionsLoadedMsg{snapshot: snapshot}
	}
}

func (model Model) loadPermissionAttention() tea.Cmd {
	client, ctx, journeyID := model.permissionClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return permissionAttentionFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		attention, err := client.PermissionAttention(ctx, app.PermissionAttentionRequest{
			JourneyID: journeyID,
		})
		if err != nil {
			return permissionAttentionFailedMsg{err: err}
		}
		return permissionAttentionLoadedMsg{attention: attention}
	}
}

func (model Model) renderPermissionsView() string {
	lines := []string{
		"Permissions · tool-call authorization pipeline",
		"j/k select · n define profile · b bind job · x validate call · d detail · r refresh · q quit",
	}
	if model.permissionClient == nil {
		lines = append(lines, "Permission service unavailable")
		return strings.Join(lines, "\n") + "\n"
	}
	if len(model.permissionSnapshot.Profiles) == 0 {
		lines = append(lines, "No permission profiles. Press n to define the default project template.")
	}
	for index, profile := range model.permissionSnapshot.Profiles {
		marker := " "
		if index == model.selected {
			marker = ">"
		}
		lines = append(lines, fmt.Sprintf(
			"%s Profile %s · gen %d · %s · %s",
			marker,
			sanitizeCell(profile.ProfileID, 28),
			profile.Generation,
			humanizeStatus(string(profile.Mode)),
			sanitizeCell(profile.Digest, 16),
		))
	}
	for _, binding := range model.permissionSnapshot.Bindings {
		lines = append(lines, fmt.Sprintf(
			"  Binding %s -> %s",
			sanitizeCell(binding.JobID, 24),
			sanitizeCell(binding.ProfileID, 24),
		))
	}
	for _, rule := range model.permissionSnapshot.Rules {
		lines = append(lines, fmt.Sprintf(
			"  Rule %s · %s · %s · %s",
			sanitizeCell(rule.RuleID, 22),
			humanizeStatus(string(rule.Action)),
			sanitizeCell(string(rule.Tool), 12),
			sanitizeCell(rule.Pattern, 32),
		))
	}
	if model.permissionDetail && model.selected < len(model.permissionSnapshot.Profiles) {
		profile := model.permissionSnapshot.Profiles[model.selected]
		lines = append(lines, "Detail:")
		for _, rule := range profile.Rules {
			lines = append(lines, fmt.Sprintf(
				"  %s · %s · %s",
				humanizeStatus(string(rule.Action)),
				sanitizeCell(string(rule.Tool), 12),
				sanitizeCell(rule.Pattern, 48),
			))
		}
		lines = append(lines, fmt.Sprintf("  Owned paths: %s", strings.Join(profile.OwnedPaths, ", ")))
	}
	return strings.Join(lines, "\n") + "\n"
}

// permissionDefineDefault defines the default project template under the
// entered profile ID through the production permissions_command.
func (model Model) permissionDefineDefault(profileID string) tea.Cmd {
	client, ctx, journeyID := model.permissionClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return permissionCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		if profileID == "" {
			return permissionCommandDoneMsg{err: app.ErrInvalidPermissionRequest}
		}
		template, err := permissions.DefaultProjectProfileTemplate()
		if err != nil {
			return permissionCommandDoneMsg{err: err}
		}
		input, _ := json.Marshal(permissions.ProfileInput{
			ProfileID: profileID, Mode: template.Mode,
			Rules: template.Rules, OwnedPaths: template.OwnedPaths,
		})
		_, err = client.PermissionCommand(ctx, app.PermissionCommandRequest{
			JourneyID: journeyID, OperationID: "tui-define-" + profileID,
			Action: "define_profile", Input: input,
		})
		return permissionCommandDoneMsg{err: err}
	}
}

// permissionBindSelected binds the selected Queue job to the first profile.
func (model Model) permissionBindSelected() tea.Cmd {
	client, ctx, journeyID := model.permissionClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return permissionCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		if model.selected >= len(model.queueSnapshot.Jobs) ||
			len(model.permissionSnapshot.Profiles) == 0 {
			return permissionCommandDoneMsg{err: app.ErrInvalidPermissionRequest}
		}
		jobID := model.queueSnapshot.Jobs[model.selected].JobID
		profileID := model.permissionSnapshot.Profiles[0].ProfileID
		input, _ := json.Marshal(map[string]any{
			"job_id": jobID, "profile_id": profileID,
		})
		_, err := client.PermissionCommand(ctx, app.PermissionCommandRequest{
			JourneyID: journeyID, OperationID: "tui-bind-" + jobID,
			Action: "bind_job", Input: input,
		})
		return permissionCommandDoneMsg{err: err}
	}
}

// permissionValidateCall runs validate_call for the selected job.
func (model Model) permissionValidateCall(command string) tea.Cmd {
	client, ctx, journeyID := model.permissionClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return permissionCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		if model.selected >= len(model.queueSnapshot.Jobs) || command == "" {
			return permissionCommandDoneMsg{err: app.ErrInvalidPermissionRequest}
		}
		jobID := model.queueSnapshot.Jobs[model.selected].JobID
		input, _ := json.Marshal(map[string]any{
			"job_id": jobID,
			"call":   map[string]any{"tool": "Bash", "command": command, "path": ""},
		})
		result, err := client.PermissionCommand(ctx, app.PermissionCommandRequest{
			JourneyID: journeyID, OperationID: "tui-validate-" + jobID,
			Action: "validate_call", Input: input,
		})
		if err != nil {
			return permissionCommandDoneMsg{err: err}
		}
		return permissionCommandDoneMsg{
			note: fmt.Sprintf("verdict %s: %s", result.Verdict, result.Denial.Reason),
		}
	}
}

// permissionResolveDecision resolves a pending permission approval through the
// existing rules authority (allow -> approved, deny -> rejected).
func (model Model) permissionResolveDecision(resolution string) tea.Cmd {
	client, ctx, journeyID := model.permissionClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return permissionCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		if len(model.permissionAttention.Approvals) == 0 {
			return permissionCommandDoneMsg{err: app.ErrInvalidPermissionRequest}
		}
		approval := model.permissionAttention.Approvals[0]
		input, _ := json.Marshal(map[string]any{
			"approval_id":     approval.ApprovalID,
			"approval_digest": approval.Digest,
			"resolution":      resolution,
			"resolved_by":     "tui-user",
		})
		_, err := client.PermissionCommand(ctx, app.PermissionCommandRequest{
			JourneyID: journeyID, OperationID: "tui-resolve-" + approval.JobID,
			Action: "resolve_approval", Input: input,
		})
		return permissionCommandDoneMsg{err: err}
	}
}

// permissionGrantAlways issues a Job-scoped grant for the first decision's
// command (first command word + " *"), never covering dangerous patterns.
func (model Model) permissionGrantAlways() tea.Cmd {
	client, ctx, journeyID := model.permissionClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return permissionCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		if len(model.permissionAttention.Decisions) == 0 {
			return permissionCommandDoneMsg{err: app.ErrInvalidPermissionRequest}
		}
		decision := model.permissionAttention.Decisions[0]
		command := strings.TrimSpace(decision.Command)
		firstWord := ""
		if command != "" {
			firstWord = strings.Fields(command)[0]
		}
		if firstWord == "" {
			return permissionCommandDoneMsg{err: app.ErrInvalidPermissionRequest}
		}
		input, _ := json.Marshal(map[string]any{
			"grant_id":  "tui-grant-" + decision.JobID + "-" + firstWord,
			"scope":     "job",
			"scope_id":  decision.JobID,
			"tool":      "Bash",
			"pattern":   firstWord + " *",
			"issued_at": time.Now().UTC().Format(time.RFC3339),
		})
		_, err := client.PermissionCommand(ctx, app.PermissionCommandRequest{
			JourneyID: journeyID, OperationID: "tui-grant-" + decision.JobID,
			Action: "issue_grant", Input: input,
		})
		return permissionCommandDoneMsg{err: err}
	}
}
