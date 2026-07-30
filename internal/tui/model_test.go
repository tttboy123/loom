package tui

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/localipc"
)

type fakeReadClient struct {
	snapshot      api.LocalProductSnapshot
	timeline      api.LocalProductTimelinePage
	err           error
	timelineCalls int
}

func TestInteractionContinuityStartsWithTasksInsteadOfHome(t *testing.T) {
	if len(screens) == 0 || screens[0] != ScreenTasks {
		t.Fatalf("initial screen = %v, want %v", screens, ScreenTasks)
	}
	for _, screen := range screens {
		switch screen {
		case ScreenHome, ScreenRuntimes, ScreenTeams:
			t.Fatalf("object/dashboard-first primary screen remains: %q", screen)
		}
	}
}

func TestTaskSelectionSurvivesPrimaryViewSwitches(t *testing.T) {
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("a", 64),
		Teams: []api.LocalProductTeamSummary{{
			TeamInstanceID: "team-1",
			DisplayName:    "Release review",
			State:          "ready",
		}},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: client.snapshot})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.selected != 1 {
		t.Fatalf("task selection=%d, want 1", model.selected)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model = updated.(Model)
	if model.Screen() != ScreenTasks || model.selected != 1 {
		t.Fatalf(
			"restored screen=%s selection=%d",
			model.Screen(),
			model.selected,
		)
	}
}

func TestTasksFilterWithoutReplacingCurrentSelection(t *testing.T) {
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("a", 64),
		Teams: []api.LocalProductTeamSummary{
			{
				TeamInstanceID: "team-alpha",
				DisplayName:    "Alpha review",
				State:          "ready",
			},
			{
				TeamInstanceID: "team-beta",
				DisplayName:    "Beta migration",
				State:          "ready",
			},
		},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: client.snapshot})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune("/"),
	})
	model = updated.(Model)
	if model.entryMode != "task_search" {
		t.Fatalf("Tasks search mode = %q", model.entryMode)
	}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("beta")},
		{Type: tea.KeyEnter},
	} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	view := model.View()
	if !strings.Contains(view, "Filter · beta") ||
		!strings.Contains(view, "Alpha review") ||
		!strings.Contains(view, "Beta migration") {
		t.Fatalf("filtered Tasks did not preserve selection: %q", view)
	}
}

func TestTeamBuilderPreflightShowsBoundProviderModelAndLimits(t *testing.T) {
	client := &fakeSetupClient{}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.screenIndex = indexOfScreen(ScreenTeamBuilder)
	model.loading = false
	model.builder = app.BuilderSessionView{
		DraftID:    "draft-1",
		CanConfirm: true,
		Preview: app.BuilderPreview{
			Name:                 "Release team",
			Purpose:              "Review the release",
			Permissions:          []string{"repo.read"},
			MaximumBudgetCredits: 200,
			EstimatedMaximumCost: "Up to 2 credits",
			Roles: []app.BuilderRolePreview{{
				Kind:        "main",
				DisplayName: "Coordinator",
				Runtime: app.SetupRuntimePreview{
					DisplayName: "Pi Coding Agent",
				},
				ModelID:       "codex-model",
				AuthMode:      "native_auth",
				PermissionIDs: []string{"repo.read"},
				Compatible:    true,
			}},
		},
	}
	model.setup = app.SetupSnapshot{
		Runtimes: []app.SetupRuntimePreview{{
			RuntimeInstanceID: "runtime-pi",
			DisplayName:       "Pi Coding Agent",
			AdapterType:       "pi",
			ModelID:           "codex-model",
			ModelIDs:          []string{"codex-model"},
		}, {
			RuntimeInstanceID: "runtime-alt",
			DisplayName:       "Alternate Runtime",
			AdapterType:       "pi",
			ModelID:           "alternate-model",
			ModelIDs:          []string{"alternate-model"},
		}},
		RoleOptions: []app.SetupRoleOptionPreview{
			{
				ID:                "coordinator",
				Kind:              "main",
				AgentDefinitionID: "agent-main",
				RuntimeProfileID:  "profile",
				RuntimeInstanceID: "runtime-pi",
				Responsibility:    "Coordinate",
			},
			{
				ID:                "reviewer",
				Kind:              "main",
				AgentDefinitionID: "agent-main",
				RuntimeProfileID:  "profile-alt",
				RuntimeInstanceID: "runtime-alt",
				Responsibility:    "Review",
			},
		},
	}
	model.builder.Preview.Roles[0].AgentDefinitionID = "agent-main"
	model.builder.Preview.Roles[0].RuntimeProfileID = "profile"
	model.builder.Preview.Roles[0].Runtime.RuntimeInstanceID = "runtime-pi"
	view := model.View()
	for _, want := range []string{
		"Coordinator",
		"Codex",
		"codex-model",
		"Native auth",
		"Repo read",
		"Compatible",
		"200 credits",
		"Up to 2 credits",
		"Main role choices",
		"m change Main role",
		"Review · alternate-model · Select to review provider and sign-in",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("preflight missing %q: %q", want, view)
		}
	}
	if strings.Contains(
		view,
		"Review · Local provider · alternate-model · Native",
	) {
		t.Fatalf("unselected role invented provider/auth: %q", view)
	}

	updated, command := model.Update(tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune("m"),
	})
	if command == nil {
		t.Fatal("Main role change did not call builder_edit")
	}
	model = updated.(Model)
	_, _ = model.Update(command())
	if client.edits != 1 ||
		client.lastEdit.Field != "main_role" ||
		client.lastEdit.Value != "reviewer" {
		t.Fatalf(
			"role edits=%d command=%#v",
			client.edits,
			client.lastEdit,
		)
	}
}

func (client *fakeReadClient) Snapshot(
	context.Context,
	api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	return client.snapshot, client.err
}

func (client *fakeReadClient) TimelinePage(
	context.Context,
	api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	client.timelineCalls++
	return client.timeline, client.err
}

func TestModelTruthfullyHandlesJournalWithNoTeams(t *testing.T) {
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("a", 64),
		Runtimes: []api.LocalProductRuntimeSummary{{
			RuntimeInstanceID: "runtime.pi",
			DisplayName:       "Pi 0.82.1",
			Status:            "online",
		}},
		Teams:     []api.LocalProductTeamSummary{},
		Runs:      []api.LocalProductRunSummary{},
		Evidence:  []api.LocalProductEvidenceSummary{},
		Attention: []api.AttentionItem{},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, command := model.Update(snapshotLoadedMsg{
		snapshot: client.snapshot,
	})
	if command != nil {
		t.Fatal("snapshot load produced unexpected command")
	}
	model = updated.(Model)
	if view := model.View(); !strings.Contains(view, "New task") {
		t.Fatalf("Tasks did not offer a new task: %q", view)
	}

	model.screenIndex = indexOfScreen(ScreenTimeline)
	if view := model.View(); !strings.Contains(
		view,
		"Choose a task to inspect its activity.",
	) {
		t.Fatalf("direct Timeline empty state = %q", view)
	}
	if client.timelineCalls != 0 {
		t.Fatalf(
			"direct Timeline screen made %d requests",
			client.timelineCalls,
		)
	}

	transitionClient := &fakeReadClient{
		snapshot: api.LocalProductSnapshot{
			SchemaVersion: 1,
			ViewVersion:   strings.Repeat("b", 64),
			Teams: []api.LocalProductTeamSummary{{
				TeamInstanceID: "team-previous",
				DisplayName:    "Previous Team",
				SourceKind:     "saved_team",
			}},
		},
		timeline: api.LocalProductTimelinePage{
			SchemaVersion:  1,
			TeamInstanceID: "team-previous",
			ViewVersion:    strings.Repeat("b", 64),
			Records: []api.LocalProductTimelineRecord{{
				Kind: "terminal",
			}},
		},
	}
	transition, err := NewModel(transitionClient)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ = transition.Update(snapshotLoadedMsg{
		snapshot: transitionClient.snapshot,
	})
	transition = updated.(Model)
	transition.screenIndex = indexOfScreen(ScreenTasks)
	transition.selected = 1
	updated, command = transition.Update(tea.KeyMsg{Type: tea.KeyEnter})
	transition = updated.(Model)
	if command == nil {
		t.Fatal("selected Team did not request its timeline")
	}
	updated, _ = transition.Update(command())
	transition = updated.(Model)
	if transitionClient.timelineCalls != 1 {
		t.Fatalf(
			"selected Team timeline calls = %d",
			transitionClient.timelineCalls,
		)
	}

	transitionClient.snapshot = client.snapshot
	updated, command = transition.Update(snapshotLoadedMsg{
		snapshot: transitionClient.snapshot,
	})
	transition = updated.(Model)
	if command != nil ||
		transition.currentTeam != "" ||
		transition.timeline.TeamInstanceID != "" ||
		!strings.Contains(
			transition.View(),
			"Choose a task to inspect its activity.",
		) {
		t.Fatalf(
			"zero-Team refresh retained stale selection: %#v view=%q",
			transition,
			transition.View(),
		)
	}
	updated, command = transition.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("r"),
	})
	transition = updated.(Model)
	if command == nil {
		t.Fatal("zero-Team refresh did not request a snapshot")
	}
	updated, _ = transition.Update(command())
	transition = updated.(Model)
	if transitionClient.timelineCalls != 1 {
		t.Fatalf(
			"zero-Team refresh retried stale timeline: calls=%d",
			transitionClient.timelineCalls,
		)
	}
}

func TestModelRendersEveryBoundedScreenAndTeamTimelineInteraction(
	t *testing.T,
) {
	client := &fakeReadClient{
		snapshot: api.LocalProductSnapshot{
			SchemaVersion: 1,
			ViewVersion:   strings.Repeat("c", 64),
			Runtimes: []api.LocalProductRuntimeSummary{{
				RuntimeInstanceID: "runtime-1",
				DisplayName:       "Local Pi",
				Status:            "online",
				ExecutableVersion: "0.82.1",
			}},
			Teams: []api.LocalProductTeamSummary{{
				TeamInstanceID: "team-1",
				DisplayName:    "Delivery Team",
				SourceKind:     "saved_team",
			}},
			Runs: []api.LocalProductRunSummary{{
				RunID:          "run-1",
				Phase:          "terminal",
				TerminalStatus: "succeeded",
			}},
			Evidence: []api.LocalProductEvidenceSummary{{
				EvidenceID: "evidence-1",
				Digest:     strings.Repeat("d", 64),
			}},
			Attention: []api.AttentionItem{{
				Kind:           "human_required",
				ActionRequired: "review",
			}},
		},
		timeline: api.LocalProductTimelinePage{
			SchemaVersion:  1,
			TeamInstanceID: "team-1",
			ViewVersion:    strings.Repeat("e", 64),
			Records: []api.LocalProductTimelineRecord{{
				Kind: "terminal",
				Payload: api.LocalProductTimelinePayload{
					Status:      "succeeded",
					WarningCode: "none",
				},
			}},
			Board: api.LocalProductTeamBoard{
				Nodes: []api.NodeBoardRow{{LogicalNodeID: "main"}},
			},
			Attention: []api.AttentionItem{},
		},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	loaded := model.Init()()
	updated, command := model.Update(loaded)
	if command != nil {
		t.Fatal("snapshot load produced unexpected command")
	}
	model = updated.(Model)

	for _, test := range []struct {
		screen Screen
		text   string
	}{
		{ScreenTasks, "Delivery Team"},
		{ScreenTeamBuilder, "What would you like"},
		{ScreenRuns, "Work 1"},
		{ScreenAttention, "review"},
		{ScreenTimeline, "Choose a task"},
	} {
		model.screenIndex = indexOfScreen(test.screen)
		if view := model.View(); !strings.Contains(view, test.text) {
			t.Fatalf("%s view = %q, want %q", test.screen, view, test.text)
		}
	}

	model.screenIndex = indexOfScreen(ScreenTasks)
	model.selected = 1
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.Screen() != ScreenTimeline || command == nil {
		t.Fatalf(
			"enter screen=%s command=%v",
			model.Screen(),
			command,
		)
	}
	updated, command = model.Update(command())
	model = updated.(Model)
	if command != nil ||
		!strings.Contains(model.View(), "Succeeded") {
		t.Fatalf("loaded timeline view = %q", model.View())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.Screen() != ScreenTasks {
		t.Fatalf("escape screen = %s", model.Screen())
	}

	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyDown},
		{Type: tea.KeyRunes, Runes: []rune("j")},
		{Type: tea.KeyUp},
		{Type: tea.KeyRunes, Runes: []rune("k")},
		{Type: tea.KeyShiftTab},
		{Type: tea.KeyLeft},
		{Type: tea.KeyRight},
	} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	updated, command = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("?"),
	})
	model = updated.(Model)
	if command != nil || !strings.Contains(model.View(), "Keys:") {
		t.Fatalf("help view = %q", model.View())
	}
	updated, command = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("r"),
	})
	if command == nil {
		t.Fatal("refresh command = nil")
	}
	updated, command = updated.(Model).Update(command())
	if command != nil {
		t.Fatal("refresh result produced command")
	}
	if _, quit := updated.(Model).Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	}); quit == nil {
		t.Fatal("q did not produce quit command")
	}
}

func TestModelMapsRemoteFailuresAndStripsOSCAndIncompleteEscapes(t *testing.T) {
	model, err := NewModel(&fakeReadClient{})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotFailedMsg{err: &localipc.RemoteError{
		Code:        "cursor_conflict",
		Recoverable: true,
	}})
	if view := updated.(Model).View(); !strings.Contains(
		view,
		"cursor_conflict",
	) {
		t.Fatalf("remote failure view = %q", view)
	}
	for _, unsafe := range []string{
		"safe\x1b]2;private-title\x07visible",
		"safe\x1b]2;private-title\x1b\\visible",
		"safe\x1b[31",
		"safe\x1b",
	} {
		sanitized := sanitizeCell(unsafe, 80)
		if strings.Contains(sanitized, "private-title") ||
			strings.ContainsRune(sanitized, '\x1b') {
			t.Fatalf("sanitizeCell(%q) = %q", unsafe, sanitized)
		}
	}
}

func TestModelPresentsRecentWorkWithoutRawInternalIdentifiers(t *testing.T) {
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("a", 64),
		Runs: []api.LocalProductRunSummary{
			{
				RunID:             "run-1",
				WorkItemID:        "work-1",
				Phase:             "terminal",
				TerminalStatus:    "succeeded",
				RuntimeInstanceID: "runtime-1",
				ClaimGeneration:   1,
			},
			{
				RunID:             "run-2",
				WorkItemID:        "work-2",
				Phase:             "terminal",
				TerminalStatus:    "failed",
				TerminalReason:    "verification_failed",
				RuntimeInstanceID: "runtime-2",
				ClaimGeneration:   2,
			},
		},
		Evidence: []api.LocalProductEvidenceSummary{
			{
				EvidenceID: "evidence-1",
				WorkItemID: "work-1",
				Digest:     strings.Repeat("1", 64),
			},
			{
				EvidenceID: "evidence-2",
				WorkItemID: "work-2",
				Digest:     strings.Repeat("2", 64),
			},
		},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: client.snapshot})
	model = updated.(Model)
	model.screenIndex = indexOfScreen(ScreenRuns)
	view := model.View()
	for _, want := range []string{
		"Work 1",
		"Work 2",
		"Succeeded",
		"Failed",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("Recent work view missing %q: %q", want, view)
		}
	}
	for _, forbidden := range []string{
		"run-1", "run-2", "runtime-1", "runtime-2",
		"evidence-1", "evidence-2",
	} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("Recent work exposed %q: %q", forbidden, view)
		}
	}
}

type cancelAwareReadClient struct {
	entered  chan struct{}
	canceled chan struct{}
	once     sync.Once
}

func (client *cancelAwareReadClient) Snapshot(
	ctx context.Context,
	_ api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	client.once.Do(func() { close(client.entered) })
	<-ctx.Done()
	close(client.canceled)
	return api.LocalProductSnapshot{}, ctx.Err()
}

func (*cancelAwareReadClient) TimelinePage(
	context.Context,
	api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	return api.LocalProductTimelinePage{}, errors.New("unexpected timeline")
}

func TestModelSurfacesGapPartialProtocolMismatchAndCancelsInflightRead(
	t *testing.T,
) {
	model, err := NewModel(&fakeReadClient{})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("a", 64),
		Partial:       true,
	}})
	model = updated.(Model)
	if !strings.Contains(model.View(), "partial") {
		t.Fatalf("partial view = %q", model.View())
	}
	updated, _ = model.Update(snapshotFailedMsg{
		err: localipc.ErrInvalidProtocol,
	})
	model = updated.(Model)
	if strings.Contains(model.View(), "Daemon offline") ||
		!strings.Contains(model.View(), "fatal_protocol_mismatch") {
		t.Fatalf("protocol mismatch view = %q", model.View())
	}

	model.lastError = ""
	model.screenIndex = indexOfScreen(ScreenTimeline)
	updated, _ = model.Update(timelineLoadedMsg{
		page: api.LocalProductTimelinePage{
			SchemaVersion:  1,
			TeamInstanceID: "team-1",
			Gap: &api.LocalProductStreamGap{
				Reason:      "cursor_conflict",
				Recoverable: true,
			},
			Board: api.LocalProductTeamBoard{
				Status: "blocked",
				Nodes:  []api.NodeBoardRow{{LogicalNodeID: "main"}},
			},
			Attention: []api.AttentionItem{{
				Kind:           "human_required",
				ActionRequired: "provide_input",
			}},
		},
	})
	model = updated.(Model)
	for _, want := range []string{
		"Some activity is unavailable",
		"press r",
		"Blocked",
		"Provide input",
	} {
		if !strings.Contains(model.View(), want) {
			t.Fatalf("gap view missing %q: %q", want, model.View())
		}
	}

	cancelClient := &cancelAwareReadClient{
		entered:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
	cancelModel, err := NewModel(cancelClient)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- cancelModel.Init()() }()
	select {
	case <-cancelClient.entered:
	case <-time.After(time.Second):
		t.Fatal("snapshot read did not start")
	}
	if _, quit := cancelModel.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	}); quit == nil {
		t.Fatal("q did not return quit command")
	}
	select {
	case <-cancelClient.canceled:
	case <-time.After(time.Second):
		t.Fatal("q did not cancel in-flight read")
	}
	<-result
}

func TestModelNavigatesAllReadScreensAndNeverCreatesMutationCommand(t *testing.T) {
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("a", 64),
		Teams: []api.LocalProductTeamSummary{{
			TeamInstanceID: "team-1",
			DisplayName:    "Team One",
			SourceKind:     "saved_team",
		}},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	if cmd := model.Init(); cmd == nil {
		t.Fatal("Init() command = nil, want initial snapshot read")
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: client.snapshot})
	model = updated.(Model)
	if !strings.Contains(model.View(), "Tasks") ||
		!strings.Contains(
			model.View(),
			"Saving a team never starts work",
		) {
		t.Fatalf("tasks view = %q", model.View())
	}

	wantScreens := []Screen{
		ScreenTeamBuilder,
		ScreenRuns,
		ScreenAttention,
		ScreenTimeline,
		ScreenTasks,
	}
	for _, want := range wantScreens {
		updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		if cmd != nil {
			t.Fatalf("screen navigation produced command for %s", want)
		}
		model = updated.(Model)
		if model.Screen() != want {
			t.Fatalf("screen = %s, want %s", model.Screen(), want)
		}
	}
}

func TestModelRendersOfflineStaleResizeAndSanitizesUntrustedText(t *testing.T) {
	client := &fakeReadClient{}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	model = updated.(Model)
	if !strings.Contains(model.View(), "60") ||
		!strings.Contains(model.View(), "16") {
		t.Fatalf("small view = %q", model.View())
	}

	updated, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model = updated.(Model)
	updated, _ = model.Update(snapshotFailedMsg{
		err: localipc.ErrLocalProductUnavailable,
	})
	model = updated.(Model)
	if !strings.Contains(model.View(), "offline") {
		t.Fatalf("offline view = %q", model.View())
	}

	unsafe := "safe\x1b[31m-red\u202ehidden\nnext"
	updated, _ = model.Update(snapshotLoadedMsg{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("b", 64),
		Stale:         true,
		Reason:        "projection_refresh_failed",
		Teams: []api.LocalProductTeamSummary{{
			TeamInstanceID: "team-unsafe",
			DisplayName:    unsafe,
			SourceKind:     "saved_team",
		}},
	}})
	model = updated.(Model)
	view := model.View()
	if !strings.Contains(view, "stale") ||
		strings.Contains(view, "\x1b[31m") ||
		strings.Contains(view, "[31m") ||
		strings.Contains(view, "\u202e") ||
		strings.Contains(view, "hidden\nnext") {
		t.Fatalf("unsafe stale view = %q", view)
	}
}

type fakeSetupClient struct {
	fakeReadClient
	setup       app.SetupSnapshot
	session     app.BuilderSessionView
	starts      int
	answers     int
	credentials int
	statuses    int
	edits       int
	lastStart   app.BuilderStartCommand
	lastStatus  app.TeamStatusCommand
	lastEdit    app.BuilderEditCommand
}

func (client *fakeSetupClient) SetupSnapshot(
	context.Context,
) (app.SetupSnapshot, error) {
	return client.setup, client.err
}

func (client *fakeSetupClient) StartBuilder(
	_ context.Context,
	command app.BuilderStartCommand,
) (app.BuilderSessionView, error) {
	client.starts++
	client.lastStart = command
	return client.session, client.err
}

func (client *fakeSetupClient) AnswerBuilder(
	_ context.Context,
	_ app.BuilderAnswerCommand,
) (app.BuilderSessionView, error) {
	client.answers++
	return client.session, client.err
}

func (client *fakeSetupClient) EditBuilder(
	_ context.Context,
	command app.BuilderEditCommand,
) (app.BuilderSessionView, error) {
	client.edits++
	client.lastEdit = command
	return client.session, client.err
}

func (client *fakeSetupClient) ConfirmBuilder(
	_ context.Context,
	_ app.BuilderConfirmCommand,
) (app.BuilderConfirmation, error) {
	return app.BuilderConfirmation{
		TeamDefinitionID: "team-fixture",
		Status:           "active",
	}, client.err
}

func (client *fakeSetupClient) ArchiveTeam(
	_ context.Context,
	command app.TeamStatusCommand,
) (app.SetupSavedTeamPreview, error) {
	client.statuses++
	client.lastStatus = command
	return app.SetupSavedTeamPreview{
		ID:         command.DefinitionID,
		Status:     "archived",
		StreamHead: command.ExpectedHead + 1,
	}, client.err
}

func (client *fakeSetupClient) RestoreTeam(
	_ context.Context,
	command app.TeamStatusCommand,
) (app.SetupSavedTeamPreview, error) {
	client.statuses++
	client.lastStatus = command
	return app.SetupSavedTeamPreview{
		ID:         command.DefinitionID,
		Status:     "active",
		StreamHead: command.ExpectedHead + 1,
	}, client.err
}

func (client *fakeSetupClient) ConfigureCredential(
	_ context.Context,
	_ app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	client.credentials++
	return app.CredentialSetupResult{
		ProviderID: "minimax",
		Revision:   1,
		Status:     "configured",
	}, client.err
}

func (client *fakeSetupClient) VerifyCredential(
	_ context.Context,
	_ app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	client.credentials++
	return app.CredentialSetupResult{
		ProviderID: "minimax",
		Revision:   2,
		Status:     "verified",
	}, client.err
}

func (client *fakeSetupClient) ReplaceCredential(
	_ context.Context,
	_ app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	client.credentials++
	return app.CredentialSetupResult{
		ProviderID: "minimax",
		Revision:   2,
		Status:     "configured",
	}, client.err
}

func (client *fakeSetupClient) RevokeCredential(
	_ context.Context,
	_ app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	client.credentials++
	return app.CredentialSetupResult{
		ProviderID: "minimax",
		Revision:   2,
		Status:     "revoked",
	}, client.err
}

func TestModelTeamBuilderOpensSavedAndTemplateCandidatesAndArchivesByHead(
	t *testing.T,
) {
	digest := strings.Repeat("d", 64)
	client := &fakeSetupClient{
		setup: app.SetupSnapshot{
			SavedTeams: []app.SetupSavedTeamPreview{{
				ID:               "team-saved",
				Version:          1,
				Name:             "Saved Review Team",
				Status:           "active",
				DefinitionDigest: digest,
				StreamHead:       3,
			}},
			Templates: []app.SetupTeamTemplatePreview{{
				ID:      "template-review",
				Version: 2,
				Digest:  digest,
				Name:    "Template Review Team",
			}},
		},
		session: app.BuilderSessionView{
			DraftID: "draft-from-source",
			Source:  app.BuilderSourceSavedTeam,
		},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.screenIndex = indexOfScreen(ScreenTeamBuilder)
	model.setup = cloneSetupSnapshot(client.setup)
	model.loading = false

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("saved Team selection did not start a Candidate")
	}
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if client.lastStart.Source != app.BuilderSourceSavedTeam ||
		client.lastStart.SourceID != "team-saved" ||
		client.lastStart.SourceDigest != digest {
		t.Fatalf("saved start = %#v", client.lastStart)
	}

	model.builder = app.BuilderSessionView{}
	model.selected = 1
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("template selection did not start a Candidate")
	}
	model = updated.(Model)
	_, _ = model.Update(command())
	if client.lastStart.Source != app.BuilderSourceTemplate ||
		client.lastStart.SourceID != "template-review" {
		t.Fatalf("template start = %#v", client.lastStart)
	}

	model.builder = app.BuilderSessionView{}
	model.selected = 0
	model.loading = false
	updated, command = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("a"),
	})
	if command == nil {
		t.Fatal("archive action was not exposed")
	}
	model = updated.(Model)
	_, _ = model.Update(command())
	if client.statuses != 1 ||
		client.lastStatus.DefinitionID != "team-saved" ||
		client.lastStatus.ExpectedHead != 3 {
		t.Fatalf(
			"archive calls=%d command=%#v",
			client.statuses,
			client.lastStatus,
		)
	}
}

func TestModelTeamBuilderEnterDoesNotReplaceOpenCandidate(
	t *testing.T,
) {
	digest := strings.Repeat("d", 64)
	client := &fakeSetupClient{
		setup: app.SetupSnapshot{
			SavedTeams: []app.SetupSavedTeamPreview{{
				ID:               "team-saved",
				Version:          1,
				Name:             "Saved Review Team",
				Status:           "active",
				DefinitionDigest: digest,
				StreamHead:       1,
			}},
		},
		session: app.BuilderSessionView{DraftID: "unexpected"},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.screenIndex = indexOfScreen(ScreenTeamBuilder)
	model.setup = cloneSetupSnapshot(client.setup)
	model.builder = app.BuilderSessionView{
		DraftID:    "draft-open",
		Revision:   4,
		CanConfirm: true,
		Preview: app.BuilderPreview{
			Name: "Open Candidate",
		},
	}
	model.loading = false

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil {
		t.Fatal("enter on an open Candidate started another setup command")
	}
	model = updated.(Model)
	if client.starts != 0 || model.builder.DraftID != "draft-open" {
		t.Fatalf(
			"starts=%d builder=%#v",
			client.starts,
			model.builder,
		)
	}
}

func TestModelTeamBuilderUsesDaemonSetupClientWithoutTerminalInput(t *testing.T) {
	client := &fakeSetupClient{
		setup: app.SetupSnapshot{
			SchemaVersion: 1,
			ViewVersion:   strings.Repeat("a", 64),
			Codex: app.ProviderSetupStatus{
				ProviderID: "codex",
				AuthMode:   "native_auth",
				Status:     "available",
			},
			MiniMax: app.ProviderSetupStatus{
				ProviderID: "minimax",
				AuthMode:   "brokered",
				Status:     "unconfigured",
			},
			Runtimes: []app.SetupRuntimePreview{{
				RuntimeInstanceID: "runtime-pi",
				DisplayName:       "Pi Coding Agent",
				ExecutableVersion: "0.82.1",
				Status:            "online",
				ModelIDs:          []string{"model-a"},
			}},
			SavedTeams:  []app.SetupSavedTeamPreview{},
			Templates:   []app.SetupTeamTemplatePreview{},
			RoleOptions: []app.SetupRoleOptionPreview{},
			Skills:      []app.SetupSkillRevision{},
			Permissions: []string{},
			Resources:   []app.SetupResourcePointer{},
		},
		session: app.BuilderSessionView{
			SchemaVersion: 1,
			DraftID:       "draft-1",
			Revision:      1,
			Source:        app.BuilderSourceBlank,
			Question: app.BuilderQuestion{
				ID:      "team_name",
				Prompt:  "Name this team",
				Options: []app.BuilderQuestionOption{},
			},
		},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.screenIndex = indexOfScreen(ScreenTeamBuilder)
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("Team Builder enter did not call setup application client")
	}
	model = updated.(Model)
	updated, command = model.Update(command())
	if command != nil {
		t.Fatal("setup snapshot result produced unexpected command")
	}
	model = updated.(Model)
	view := model.View()
	for _, want := range []string{
		"Team Builder",
		"Codex",
		"native_auth",
		"MiniMax",
		"brokered",
		"Pi Coding Agent",
		"0.82.1",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("Team Builder view missing %q: %q", want, view)
		}
	}
	updated, command = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("n"),
	})
	if command == nil {
		t.Fatal("new Team action did not start Candidate builder")
	}
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if client.starts != 1 ||
		!strings.Contains(model.View(), "Name this team") ||
		strings.Contains(model.View(), "sqlite") ||
		strings.Contains(model.View(), "launchctl") {
		t.Fatalf(
			"builder starts=%d view=%q",
			client.starts,
			model.View(),
		)
	}
}

func TestModelTeamBuilderAnswersAndMasksCredentialEntry(t *testing.T) {
	client := &fakeSetupClient{
		setup: app.SetupSnapshot{
			SchemaVersion: 1,
			ViewVersion:   strings.Repeat("a", 64),
			MiniMax: app.ProviderSetupStatus{
				ProviderID: "minimax",
				AuthMode:   "brokered",
				Status:     "unconfigured",
			},
			Runtimes:    []app.SetupRuntimePreview{},
			SavedTeams:  []app.SetupSavedTeamPreview{},
			Templates:   []app.SetupTeamTemplatePreview{},
			RoleOptions: []app.SetupRoleOptionPreview{},
			Skills:      []app.SetupSkillRevision{},
			Permissions: []string{},
			Resources:   []app.SetupResourcePointer{},
		},
		session: app.BuilderSessionView{
			SchemaVersion: 1,
			DraftID:       "draft-1",
			Revision:      1,
			Source:        app.BuilderSourceBlank,
			CatalogDigest: strings.Repeat("b", 64),
			Question: app.BuilderQuestion{
				ID:      "team_name",
				Prompt:  "Name this team",
				Options: []app.BuilderQuestionOption{},
			},
		},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.screenIndex = indexOfScreen(ScreenTeamBuilder)
	model.setup = cloneSetupSnapshot(client.setup)
	model.builder = cloneBuilderSession(client.session)
	model.loading = false

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil {
		t.Fatal("opening answer input produced an external command")
	}
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("Local Team"),
	})
	model = updated.(Model)
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("submitting an answer did not call the application service")
	}
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if client.answers != 1 {
		t.Fatalf("answer calls = %d", client.answers)
	}

	model.builder = app.BuilderSessionView{}
	updated, command = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("g"),
	})
	if command != nil {
		t.Fatal("opening masked credential entry produced a command")
	}
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("alpha"),
	})
	model = updated.(Model)
	if strings.Contains(model.View(), "alpha") ||
		!strings.Contains(model.View(), "•••••") {
		t.Fatalf("credential entry was not masked: %q", model.View())
	}
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("credential submit did not call the setup service")
	}
	model = updated.(Model)
	if len(model.entry) != 0 || model.entryMode != "" {
		t.Fatal("credential input remained in TUI state after submit")
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if client.credentials != 1 {
		t.Fatalf("credential calls = %d", client.credentials)
	}
}
