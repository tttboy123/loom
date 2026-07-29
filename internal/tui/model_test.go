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
	"loom-pi-rebuild/internal/localipc"
)

type fakeReadClient struct {
	snapshot      api.LocalProductSnapshot
	timeline      api.LocalProductTimelinePage
	err           error
	timelineCalls int
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
	if view := model.View(); !strings.Contains(view, "0 teams") {
		t.Fatalf("Home did not report zero Teams: %q", view)
	}

	model.screenIndex = indexOfScreen(ScreenTeams)
	if view := model.View(); !strings.Contains(
		view,
		"No Teams exist in this Journal yet.",
	) {
		t.Fatalf("Teams empty state = %q", view)
	}
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command != nil ||
		model.Screen() != ScreenTeams ||
		client.timelineCalls != 0 {
		t.Fatalf(
			"empty Team enter command=%v screen=%s timeline_calls=%d",
			command,
			model.Screen(),
			client.timelineCalls,
		)
	}

	model.screenIndex = indexOfScreen(ScreenTimeline)
	if view := model.View(); !strings.Contains(
		view,
		"Select a Team from Teams to open its authoritative timeline.",
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
	transition.screenIndex = indexOfScreen(ScreenTeams)
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
			"Select a Team from Teams to open its authoritative timeline.",
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
		{ScreenHome, "Delivery Team"},
		{ScreenRuntimes, "Local Pi"},
		{ScreenTeams, "Delivery Team"},
		{ScreenRuns, "run-1"},
		{ScreenEvidence, "evidence-1"},
		{ScreenCompare, "Select two"},
		{ScreenAttention, "human_required"},
		{ScreenTimeline, "Select a Team"},
	} {
		model.screenIndex = indexOfScreen(test.screen)
		if view := model.View(); !strings.Contains(view, test.text) {
			t.Fatalf("%s view = %q, want %q", test.screen, view, test.text)
		}
	}

	model.screenIndex = indexOfScreen(ScreenTeams)
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
		!strings.Contains(model.View(), "succeeded") {
		t.Fatalf("loaded timeline view = %q", model.View())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.Screen() != ScreenTeams {
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

func TestModelSelectsTwoRunsForExactSummaryAndEvidenceCompare(t *testing.T) {
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
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil {
		t.Fatal("Run compare selection dispatched I/O")
	}
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil {
		t.Fatal("second Run compare selection dispatched I/O")
	}
	model = updated.(Model)
	model.screenIndex = indexOfScreen(ScreenCompare)
	view := model.View()
	for _, want := range []string{
		"run-1",
		"run-2",
		"succeeded",
		"verification_failed",
		"evidence-1",
		"evidence-2",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("Compare view missing %q: %q", want, view)
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
		"stream gap",
		"press r",
		"blocked",
		"human_required",
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
	if !strings.Contains(model.View(), "Home") ||
		!strings.Contains(model.View(), "read-only") {
		t.Fatalf("home view = %q", model.View())
	}

	wantScreens := []Screen{
		ScreenRuntimes,
		ScreenTeams,
		ScreenRuns,
		ScreenEvidence,
		ScreenCompare,
		ScreenAttention,
		ScreenTimeline,
		ScreenHome,
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
