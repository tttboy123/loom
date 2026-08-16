package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/roundtable"
)

func TestTUIRoundtableClientExposesJourneyMethods(t *testing.T) {
	clientType := reflect.TypeOf(&DaemonReadClient{})
	for _, method := range []string{
		"RoundtableCreateSession",
		"RoundtableAddSeat",
		"RoundtableOpenRound",
		"RoundtableProposeMessage",
		"RoundtableRelayMessage",
		"RoundtableAcknowledgeMessage",
		"RoundtableInsertMessage",
		"RoundtableConcludeSession",
		"RoundtableReadView",
	} {
		if _, found := clientType.MethodByName(method); !found {
			t.Fatalf("DaemonReadClient.%s is missing", method)
		}
	}
}

// stubRoundtableClient is a stateful test double that advances a minimal
// session as writes arrive, mirroring the authority's lifecycle.
type stubRoundtableClient struct {
	sessionID string
	seats     []string
	rounds    []string
	status    string
	concluded bool
	calls     []string
}

func (client *stubRoundtableClient) view() roundtable.View {
	seats := map[string]roundtable.Seat{
		roundtableModeratorSeat: {
			ID: roundtableModeratorSeat, DisplayName: "Moderator", Available: true,
		},
	}
	for _, id := range client.seats {
		seats[id] = roundtable.Seat{ID: id, DisplayName: id, Available: true}
	}
	view := roundtable.View{
		Session: roundtable.Session{
			ID: client.sessionID, ModeratorSeat: roundtableModeratorSeat,
			Title: roundtableDemoTitle, CreatedAt: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC),
			Concluded: client.concluded,
		},
		Seats:    seats,
		Messages: map[string]roundtable.Message{},
		Digest:   strings.Repeat("a", 64),
	}
	if len(client.rounds) > 0 {
		round := roundtable.Round{
			ID: roundtableRoundID, Sequence: 1,
		}
		if client.status != "" {
			round.MessageCount = 1
			message := roundtable.Message{
				ID: roundtableMessageID, RoundID: roundtableRoundID,
				WriterSeat: roundtableWriterSeat, TargetSeat: roundtableTargetSeat,
				Body: roundtableDemoBody, BodyDigest: strings.Repeat("b", 64),
				Status: client.status,
			}
			round.Messages = []roundtable.Message{message}
			view.Messages[roundtableMessageID] = message
		}
		view.Rounds = []roundtable.Round{round}
	}
	return view
}

func (client *stubRoundtableClient) RoundtableCreateSession(
	_ context.Context,
	request RoundtableSessionCreateRequest,
) (roundtable.View, error) {
	client.calls = append(client.calls, "create")
	client.sessionID = request.SessionID
	return client.view(), nil
}

func (client *stubRoundtableClient) RoundtableAddSeat(
	_ context.Context,
	request RoundtableAddSeatRequest,
) (roundtable.View, error) {
	client.calls = append(client.calls, "add_seat:"+request.SeatID)
	for _, existing := range client.seats {
		if existing == request.SeatID {
			return roundtable.View{}, roundtable.ErrRoundtableConflict
		}
	}
	client.seats = append(client.seats, request.SeatID)
	return client.view(), nil
}

func (client *stubRoundtableClient) RoundtableOpenRound(
	_ context.Context,
	_ RoundtableOpenRoundRequest,
) (roundtable.View, error) {
	client.calls = append(client.calls, "open_round")
	client.rounds = []string{roundtableRoundID}
	return client.view(), nil
}

func (client *stubRoundtableClient) RoundtableProposeMessage(
	_ context.Context,
	_ RoundtableProposeMessageRequest,
) (roundtable.View, error) {
	client.calls = append(client.calls, "propose")
	client.status = roundtable.MessagePending
	return client.view(), nil
}

func (client *stubRoundtableClient) RoundtableRelayMessage(
	_ context.Context,
	_ RoundtableRelayMessageRequest,
) (roundtable.View, error) {
	client.calls = append(client.calls, "relay")
	client.status = roundtable.MessageRelayed
	return client.view(), nil
}

func (client *stubRoundtableClient) RoundtableAcknowledgeMessage(
	_ context.Context,
	_ RoundtableAckMessageRequest,
) (roundtable.View, error) {
	client.calls = append(client.calls, "ack")
	client.status = roundtable.MessageAcknowledged
	return client.view(), nil
}

func (client *stubRoundtableClient) RoundtableInsertMessage(
	_ context.Context,
	_ RoundtableInsertMessageRequest,
) (roundtable.View, error) {
	client.calls = append(client.calls, "insert")
	client.status = roundtable.MessageInserted
	return client.view(), nil
}

func (client *stubRoundtableClient) RoundtableConcludeSession(
	_ context.Context,
	_ RoundtableConcludeRequest,
) (roundtable.View, error) {
	client.calls = append(client.calls, "conclude")
	client.concluded = true
	return client.view(), nil
}

func (client *stubRoundtableClient) RoundtableReadView(
	_ context.Context,
	sessionID string,
) (roundtable.View, error) {
	client.calls = append(client.calls, "read")
	client.sessionID = sessionID
	return client.view(), nil
}

func (client *stubRoundtableClient) Snapshot(
	context.Context,
	api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	return api.LocalProductSnapshot{}, nil
}

func (client *stubRoundtableClient) TimelinePage(
	context.Context,
	api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	return api.LocalProductTimelinePage{}, nil
}

func newStubRoundtableModel() (Model, *stubRoundtableClient) {
	client := &stubRoundtableClient{}
	model := Model{
		roundtableClient:    client,
		roundtableSessionID: "rt-tui",
	}
	return model, client
}

// roundtableWriteCalls filters authoritative reloads ("read") from the
// recorded call log, leaving only Journal writes.
func roundtableWriteCalls(calls []string) []string {
	writes := make([]string, 0, len(calls))
	for _, call := range calls {
		if call != "read" {
			writes = append(writes, call)
		}
	}
	return writes
}

// advanceRoundtableStep runs one journey hop and consumes the follow-up
// authoritative reload, returning the updated model.
func advanceRoundtableStep(t *testing.T, model Model) Model {
	t.Helper()
	cmd := model.roundtableAdvanceStep()
	if cmd == nil {
		t.Fatal("advance produced no command")
	}
	updated, reload := model.Update(cmd())
	model = updated.(Model)
	for i := 0; i < 4 && reload != nil; i++ {
		updated, reload = model.Update(reload())
		model = updated.(Model)
	}
	return model
}

func TestTUIAdvanceStepWalksFullDualSeatJourney(t *testing.T) {
	model, client := newStubRoundtableModel()
	step := roundtableCurrentStep(model.roundtableView, false)
	if step != roundtableStepCreateSession {
		t.Fatalf("initial step = %v", step)
	}
	for i := 0; i < 8; i++ {
		model = advanceRoundtableStep(t, model)
	}
	want := []string{
		"create", "add_seat:seat-writer", "add_seat:seat-target", "open_round",
		"propose", "relay", "ack", "insert", "conclude",
	}
	writes := roundtableWriteCalls(client.calls)
	if len(writes) != len(want) {
		t.Fatalf("writes = %v, want %v", writes, want)
	}
	for index := range want {
		if writes[index] != want[index] {
			t.Fatalf("writes = %v, want %v", writes, want)
		}
	}
	if !model.roundtableView.Session.Concluded {
		t.Fatal("session not concluded after full journey")
	}
}

func TestTUIAdvanceStepDoesNotDoubleContinueAfterConclude(t *testing.T) {
	model, client := newStubRoundtableModel()
	// Walk the full journey.
	for i := 0; i < 8; i++ {
		model = advanceRoundtableStep(t, model)
	}
	if !model.roundtableView.Session.Concluded {
		t.Fatal("session not concluded")
	}
	before := len(roundtableWriteCalls(client.calls))
	// Further advances must be no-ops: the step is concluded and no new writes
	// are issued even though the follow-up reload still runs.
	for i := 0; i < 3; i++ {
		model = advanceRoundtableStep(t, model)
	}
	if len(roundtableWriteCalls(client.calls)) != before {
		t.Fatalf("post-conclude advance wrote again: calls = %v", client.calls)
	}
	if !model.roundtableView.Session.Concluded {
		t.Fatal("session not concluded")
	}
}

func TestTUICurrentStepIsRestartConsistent(t *testing.T) {
	// A mid-journey view (message acknowledged) yields the same next step as
	// the live view, and a concluded view stays concluded.
	mid := roundtable.View{
		Session: roundtable.Session{ID: "rt-x", ModeratorSeat: roundtableModeratorSeat},
		Seats: map[string]roundtable.Seat{
			roundtableModeratorSeat: {ID: roundtableModeratorSeat},
			roundtableWriterSeat:    {ID: roundtableWriterSeat},
			roundtableTargetSeat:    {ID: roundtableTargetSeat},
		},
		Rounds: []roundtable.Round{{ID: roundtableRoundID, Sequence: 1, MessageCount: 1}},
		Messages: map[string]roundtable.Message{
			roundtableMessageID: {
				ID: roundtableMessageID, Status: roundtable.MessageAcknowledged,
			},
		},
	}
	if step := roundtableCurrentStep(mid, true); step != roundtableStepInsert {
		t.Fatalf("mid-journey step = %v, want insert", step)
	}
	mid.Session.Concluded = true
	if step := roundtableCurrentStep(mid, true); step != roundtableStepConcluded {
		t.Fatalf("concluded step = %v, want concluded", step)
	}
}

func TestTUIRoundtableScreenRendersSessionAndMessages(t *testing.T) {
	model, client := newStubRoundtableModel()
	model.roundtableSessionID = "rt-render"
	client.sessionID = "rt-render"
	client.seats = []string{roundtableWriterSeat, roundtableTargetSeat}
	client.rounds = []string{roundtableRoundID}
	client.status = roundtable.MessageAcknowledged
	model.roundtableView = client.view()
	model.screenIndex = indexOfScreen(ScreenRoundtable)
	body := model.renderRoundtableView()
	for _, want := range []string{
		"Roundtable", "rt-render", "seat-writer", "seat-target",
		"Acknowledged", "round-1", "msg-1", "body digest",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("roundtable view missing %q in:\n%s", want, body)
		}
	}
}

func TestTUIRunRoundtableLiveJourneyWalksDualSeatJourney(t *testing.T) {
	client := &stubRoundtableClient{}
	frames, view, err := RunRoundtableLiveJourney(
		context.Background(),
		client,
		"rt-live-test",
	)
	if err != nil {
		t.Fatalf("journey: %v", err)
	}
	if !view.Session.Concluded {
		t.Fatal("session not concluded")
	}
	writes := roundtableWriteCalls(client.calls)
	want := []string{
		"create", "add_seat:seat-writer", "add_seat:seat-target", "open_round",
		"propose", "relay", "ack", "insert", "conclude",
	}
	if len(writes) != len(want) {
		t.Fatalf("writes = %v, want %v", writes, want)
	}
	for index := range want {
		if writes[index] != want[index] {
			t.Fatalf("writes = %v, want %v", writes, want)
		}
	}
	if len(frames) == 0 || !strings.Contains(frames[0], "Roundtable") {
		t.Fatalf("journey frames missing header: %q", frames)
	}
	last := frames[len(frames)-1]
	if !strings.Contains(last, "concluded") {
		t.Fatalf("last frame missing conclusion: %q", last)
	}
}

func TestTUIAddSeatsIsIdempotentPerSeat(t *testing.T) {
	client := &stubRoundtableClient{
		sessionID: "rt-partial",
		seats:     []string{roundtableWriterSeat},
	}
	model := Model{
		roundtableClient:    client,
		roundtableSessionID: "rt-partial",
		roundtableView:      client.view(),
	}
	// View has moderator + writer only -> next step is addSeats; the writer
	// must NOT be re-added (it would conflict), only the target.
	step := roundtableCurrentStep(model.roundtableView, true)
	if step != roundtableStepAddSeats {
		t.Fatalf("step = %v, want add seats", step)
	}
	updated, reload := model.Update(model.roundtableAdvanceStep()())
	model = updated.(Model)
	for i := 0; i < 4 && reload != nil; i++ {
		updated, reload = model.Update(reload())
		model = updated.(Model)
	}
	writes := roundtableWriteCalls(client.calls)
	if len(writes) != 1 || writes[0] != "add_seat:seat-target" {
		t.Fatalf("writes = %v, want [add_seat:seat-target]", writes)
	}
}

func TestTUISessionSwitchClearsStaleView(t *testing.T) {
	client := &stubRoundtableClient{}
	model, _ := newStubRoundtableModel()
	model.roundtableClient = client
	client.sessionID = "rt-old"
	client.concluded = true
	model.roundtableSessionID = "rt-old"
	model.roundtableView = client.view()
	model.screenIndex = indexOfScreen(ScreenRoundtable)
	// User presses e (entry mode) and enters a new session id.
	model.entryMode = entryRoundtableSession
	model.entry = []byte("rt-new")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.roundtableSessionID != "rt-new" {
		t.Fatalf("session id = %q, want rt-new", model.roundtableSessionID)
	}
	if model.roundtableView.Session.ID != "" {
		t.Fatalf("stale view not cleared: %#v", model.roundtableView.Session)
	}
	if model.roundtableError != "" {
		t.Fatalf("stale error not cleared: %q", model.roundtableError)
	}
	if cmd == nil {
		t.Fatal("expected an advance command after session switch")
	}
	if step := roundtableCurrentStep(model.roundtableView, false); step != roundtableStepCreateSession {
		t.Fatalf("new session step = %v, want create", step)
	}
}

// TestTUIRoundtableReadViewWire validates the TUI client sends the exact
// roundtable_snapshot method over a real Local IPC socket.
func TestTUIRoundtableReadViewWire(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "loom-tui-roundtable-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	viewJSON := `{"session":{"id":"rt-wire","moderator_seat":"seat-moderator","title":"Wire","created_at":"2026-08-17T12:00:00Z","concluded":false},"seats":{"seat-moderator":{"id":"seat-moderator","display_name":"Moderator","available":true}},"rounds":[],"messages":{},"digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	handler := localipc.HandlerFunc(func(
		_ context.Context,
		request localipc.Request,
	) localipc.Response {
		if request.Method != "roundtable_snapshot" {
			return localipc.Response{Error: &localipc.ProtocolError{
				Code: "unknown_method", Message: "unknown method",
			}}
		}
		var params struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil ||
			params.SessionID != "rt-wire" {
			return localipc.Response{Error: &localipc.ProtocolError{
				Code: "invalid_request", Message: "invalid request",
			}}
		}
		return localipc.Response{OK: true, Result: json.RawMessage(viewJSON)}
	})
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath:   filepath.Join(root, "loomd.sock"),
		EffectiveUID: os.Geteuid(),
		BuildID:      "tui-roundtable-wire",
		Handler:      handler,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case err := <-done:
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("server close: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("server did not close")
		}
	}()
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: filepath.Join(root, "loomd.sock"), Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	readClient, err := NewDaemonReadClient(client)
	if err != nil {
		t.Fatal(err)
	}
	view, err := readClient.RoundtableReadView(context.Background(), "rt-wire")
	if err != nil {
		t.Fatal(err)
	}
	if view.Session.ID != "rt-wire" || len(view.Seats) != 1 {
		t.Fatalf("view = %#v", view)
	}
}
