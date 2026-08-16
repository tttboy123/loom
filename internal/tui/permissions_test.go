package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/permissions"
)

type stubPermissionClient struct {
	snapshot  app.PermissionSnapshot
	attention app.PermissionAttention
	result    app.PermissionCommandResult
	commands  int
}

func (client *stubPermissionClient) PermissionSnapshot(
	context.Context,
	app.PermissionSnapshotRequest,
) (app.PermissionSnapshot, error) {
	return client.snapshot, nil
}

func (client *stubPermissionClient) PermissionAttention(
	context.Context,
	app.PermissionAttentionRequest,
) (app.PermissionAttention, error) {
	return client.attention, nil
}

func (client *stubPermissionClient) PermissionCommand(
	context.Context,
	app.PermissionCommandRequest,
) (app.PermissionCommandResult, error) {
	client.commands++
	return client.result, nil
}

type stubPermissionReadClient struct {
	permissions PermissionClient
}

func (client *stubPermissionReadClient) Snapshot(
	context.Context,
	api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	return api.LocalProductSnapshot{}, nil
}

func (client *stubPermissionReadClient) TimelinePage(
	context.Context,
	api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	return api.LocalProductTimelinePage{}, nil
}

var _ ReadClient = (*stubPermissionReadClient)(nil)

func TestPermissionsScreenRendersProfilesBindingsAndRules(t *testing.T) {
	permissionClient := &stubPermissionClient{
		snapshot: app.PermissionSnapshot{
			ViewVersion: "v1",
			Profiles: []permissions.PermissionProfile{
				{ProfileID: "profile-a", Generation: 1, Digest: "d1", Mode: permissions.ModeDefault},
			},
			Bindings: []permissions.JobBinding{
				{JobID: "job-a", ProfileID: "profile-a"},
			},
			Rules: []permissions.Rule{
				{RuleID: "r-1", Scope: permissions.ScopeProject, ScopeID: "p1",
					Action: permissions.ActionAllow, Tool: permissions.ToolBash, Pattern: "go test *"},
			},
		},
	}
	model, err := newModelWithContext(context.Background(), &stubPermissionReadClient{permissions: permissionClient})
	if err != nil {
		t.Fatal(err)
	}
	model.permissionClient = permissionClient
	model.permissionSnapshot = permissionClient.snapshot
	model.screenIndex = indexOfScreen(ScreenPermissions)
	body := model.renderPermissionsView()
	for _, want := range []string{
		"Permissions", "profile-a", "job-a -> profile-a",
		"r-1", "go test *",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("permissions view missing %q:\n%s", want, body)
		}
	}
}

func TestPermissionsScreenKeyDefineBindAndValidate(t *testing.T) {
	permissionClient := &stubPermissionClient{
		snapshot: app.PermissionSnapshot{
			Profiles: []permissions.PermissionProfile{
				{ProfileID: "profile-a", Generation: 1, Digest: "d1", Mode: permissions.ModeDefault},
			},
		},
		result: app.PermissionCommandResult{Action: "validate_call", Verdict: permissions.VerdictAllow},
	}
	model, err := newModelWithContext(context.Background(), &stubPermissionReadClient{permissions: permissionClient})
	if err != nil {
		t.Fatal(err)
	}
	model.permissionClient = permissionClient
	model.permissionSnapshot = permissionClient.snapshot
	model.screenIndex = indexOfScreen(ScreenPermissions)

	updated, _ := model.Update(teaKeyString("n"))
	model = updated.(Model)
	if model.entryMode != entryPermissionProfile {
		t.Fatalf("entryMode = %q, want permission_profile", model.entryMode)
	}
}

func TestP2DPermissionApprovalWithoutAuthenticatedDetailsFailsClosed(t *testing.T) {
	client := &stubPermissionClient{}
	model := Model{
		permissionClient: client,
		ctx:              context.Background(),
		permissionAttention: app.PermissionAttention{
			Approvals: []app.PermissionApprovalView{{
				ApprovalID: "approval-content-free", Digest: "digest",
				JobID: "job-content-free", Status: "pending",
			}},
		},
	}
	message, ok := model.permissionResolveDecision("allow")().(permissionCommandDoneMsg)
	if !ok || !errors.Is(message.err, app.ErrInvalidPermissionRequest) || client.commands != 0 {
		t.Fatalf("content-free approval result = %#v, commands=%d", message, client.commands)
	}
	message, ok = model.permissionResolveDecision("deny")().(permissionCommandDoneMsg)
	if !ok || message.err != nil || client.commands != 1 {
		t.Fatalf("content-free rejection result = %#v, commands=%d", message, client.commands)
	}
}

func teaKeyString(value string) tea.KeyMsg {
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
	return key
}
