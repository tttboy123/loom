package roundtable

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
)

func roundtableExecutionBinding(
	t *testing.T,
	profileID string,
	providerAccountID string,
	modelID string,
	revision int64,
) loomruntime.FrozenExecutionBinding {
	t.Helper()
	profile := loomruntime.RuntimeProfile{
		ID: profileID, AdapterType: "loom-native", ProviderID: "deepseek",
		ProviderAccountID: providerAccountID, ModelID: modelID,
		AuthMode:            loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("a", 64),
		CredentialReference: "credential-ref-" + strings.ReplaceAll(providerAccountID, ".", "-"),
		CredentialRevision:  revision, Timeout: time.Minute,
		RequiredCapabilities: []string{"text"},
	}
	instance := loomruntime.RuntimeInstance{
		ID: "runtime-" + profileID, DeviceID: "device-local",
		AdapterType: "loom-native", DisplayName: "Loom Native",
		Status:               loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"text"}, Capacity: 4,
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatalf("freeze execution binding: %v", err)
	}
	return binding
}

func roundtableMissionContext() SessionContext {
	return SessionContext{
		ConversationID: "conversation-42",
		MissionID:      "mission-42",
		TeamID:         "team-review",
		TeamVersion:    7,
		WorkspaceID:    "workspace-local",
	}
}

func TestMissionLinkedSessionFreezesIndependentAgentSeatsAndReplays(t *testing.T) {
	authority, _, _ := newRoundtableFixture(t)
	ctx := context.Background()
	correlation := "44444444-4444-4444-8444-444444444444"
	sessionContext := roundtableMissionContext()

	view, err := authority.CreateSession(ctx, CreateSessionCommand{
		SessionID: "session-mission-42", ModeratorSeat: "seat-moderator",
		Title: "Choose the implementation", Context: &sessionContext,
		EmittedAt: utcTime(1), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("create Mission-linked session: %v", err)
	}
	if view.Session.Context == nil || !reflect.DeepEqual(*view.Session.Context, sessionContext) {
		t.Fatalf("session context = %#v, want %#v", view.Session.Context, sessionContext)
	}

	inputs := []struct {
		seatID    string
		name      string
		agentID   string
		roleKind  string
		profileID string
		accountID string
		modelID   string
		revision  int64
	}{
		{"seat-planner", "Planner", "agent.planner", "main", "profile.planner", "deepseek.planning", "deepseek-reasoner", 3},
		{"seat-reviewer", "Reviewer", "agent.reviewer", "subagent", "profile.reviewer", "deepseek.review", "deepseek-chat", 5},
	}
	for index, input := range inputs {
		executionBinding := roundtableExecutionBinding(
			t, input.profileID, input.accountID, input.modelID, input.revision,
		)
		seatBinding, err := FreezeSeatBinding(
			"session-mission-42", input.seatID, sessionContext,
			input.agentID, input.roleKind, input.profileID, executionBinding, 1,
		)
		if err != nil {
			t.Fatalf("freeze seat binding: %v", err)
		}
		view, err = authority.AddSeat(ctx, AddSeatCommand{
			SessionID: "session-mission-42", SeatID: input.seatID,
			DisplayName: input.name, Binding: &seatBinding,
			EmittedAt: utcTime(index + 2), CorrelationID: correlation,
		})
		if err != nil {
			t.Fatalf("add frozen seat: %v", err)
		}
		seat := view.Seats[input.seatID]
		if seat.Binding == nil || seat.Binding.BindingDigest == "" ||
			seat.Binding.ExecutionBinding.BindingDigest != executionBinding.BindingDigest ||
			seat.Binding.ExecutionBinding.ProviderAccountID != input.accountID ||
			seat.Binding.ExecutionBinding.ModelID != input.modelID {
			t.Fatalf("seat binding = %#v", seat.Binding)
		}
	}

	replayed, err := authority.ReadView(ctx, "session-mission-42")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !reflect.DeepEqual(view, replayed) {
		t.Fatalf("replayed view drifted:\ncreated=%#v\nreplayed=%#v", view, replayed)
	}
	if replayed.Seats["seat-planner"].Binding.ExecutionBinding.ProviderAccountID ==
		replayed.Seats["seat-reviewer"].Binding.ExecutionBinding.ProviderAccountID {
		t.Fatal("independent seats collapsed to one Provider Account")
	}
}

func TestMissionLinkedSeatRejectsBindingSubstitutionWithoutJournalMutation(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	correlation := "55555555-5555-4555-8555-555555555555"
	sessionContext := roundtableMissionContext()
	if _, err := authority.CreateSession(ctx, CreateSessionCommand{
		SessionID: "session-binding-tamper", ModeratorSeat: "seat-moderator",
		Title: "Tamper check", Context: &sessionContext,
		EmittedAt: utcTime(1), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	executionBinding := roundtableExecutionBinding(
		t, "profile.planner", "deepseek.primary", "deepseek-chat", 3,
	)
	seatBinding, err := FreezeSeatBinding(
		"session-binding-tamper", "seat-planner", sessionContext,
		"agent.planner", "main", "profile.planner", executionBinding, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	seatBinding.ExecutionBinding.ProviderAccountID = "deepseek.substituted"
	if _, err := authority.AddSeat(ctx, AddSeatCommand{
		SessionID: "session-binding-tamper", SeatID: "seat-planner",
		DisplayName: "Planner", Binding: &seatBinding,
		EmittedAt: utcTime(2), CorrelationID: correlation,
	}); !errors.Is(err, ErrInvalidRoundtableSeatBinding) {
		t.Fatalf("tampered binding error = %v", err)
	}
	events, err := journalStore.ReadStream(ctx, sessionStream("session-binding-tamper"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != FactSessionCreated {
		t.Fatalf("tampered binding mutated Journal: %#v", events)
	}
}

func TestLegacyRoundtableSessionRemainsReadableAndUnbound(t *testing.T) {
	authority, _, _ := newRoundtableFixture(t)
	ctx := context.Background()
	correlation := "66666666-6666-4666-8666-666666666666"
	if _, err := authority.CreateSession(ctx, CreateSessionCommand{
		SessionID: "session-legacy-v1", ModeratorSeat: "seat-moderator",
		Title: "Legacy ledger", EmittedAt: utcTime(1), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.AddSeat(ctx, AddSeatCommand{
		SessionID: "session-legacy-v1", SeatID: "seat-writer",
		DisplayName: "Writer", EmittedAt: utcTime(2), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	replayed, err := authority.ReadView(ctx, "session-legacy-v1")
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Session.Context != nil || replayed.Seats["seat-writer"].Binding != nil {
		t.Fatalf("legacy session gained invented authority: %#v", replayed)
	}
}

func TestMissionLinkedRoundtableEnforcesTwoToSixFrozenSeatsAndMembershipLock(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	correlation := "77777777-7777-4777-8777-777777777777"
	sessionContext := roundtableMissionContext()
	const sessionID = "session-seat-bounds"
	if _, err := authority.CreateSession(ctx, CreateSessionCommand{
		SessionID: sessionID, ModeratorSeat: "seat-moderator",
		Title: "Seat bounds", Context: &sessionContext,
		EmittedAt: utcTime(1), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	addSeat := func(index int) error {
		seatID := fmt.Sprintf("seat-agent-%d", index)
		profileID := fmt.Sprintf("profile-agent-%d", index)
		execution := roundtableExecutionBinding(
			t, profileID, fmt.Sprintf("deepseek.account-%d", index),
			"deepseek-chat", int64(index+1),
		)
		binding, err := FreezeSeatBinding(
			sessionID, seatID, sessionContext, fmt.Sprintf("agent.%d", index),
			map[bool]string{true: "main", false: "subagent"}[index == 1],
			profileID, execution, 1,
		)
		if err != nil {
			return err
		}
		_, err = authority.AddSeat(ctx, AddSeatCommand{
			SessionID: sessionID, SeatID: seatID,
			DisplayName: fmt.Sprintf("Agent %d", index), Binding: &binding,
			EmittedAt: utcTime(index + 1), CorrelationID: correlation,
		})
		return err
	}
	if err := addSeat(1); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.OpenRound(ctx, OpenRoundCommand{
		SessionID: sessionID, RoundID: "round-too-early",
		ModeratorSeat: "seat-moderator", EmittedAt: utcTime(3),
		CorrelationID: correlation,
	}); !errors.Is(err, ErrRoundtableAgentSeatCount) {
		t.Fatalf("one-seat round error = %v", err)
	}
	events, err := journalStore.ReadStream(ctx, sessionStream(sessionID))
	if err != nil || len(events) != 2 {
		t.Fatalf("invalid round mutated Journal: events=%#v err=%v", events, err)
	}
	for index := 2; index <= MaxAgentSeats; index++ {
		if err := addSeat(index); err != nil {
			t.Fatalf("add seat %d: %v", index, err)
		}
	}
	if err := addSeat(MaxAgentSeats + 1); !errors.Is(err, ErrRoundtableTooManySeats) {
		t.Fatalf("seventh Agent error = %v", err)
	}
	if _, err := authority.OpenRound(ctx, OpenRoundCommand{
		SessionID: sessionID, RoundID: "round-1", ModeratorSeat: "seat-moderator",
		EmittedAt: utcTime(9), CorrelationID: correlation,
	}); err != nil {
		t.Fatalf("six-seat round: %v", err)
	}
	if err := addSeat(MaxAgentSeats + 2); !errors.Is(err, ErrRoundtableConflict) {
		t.Fatalf("post-round membership change error = %v", err)
	}
	if _, err := authority.RetireSeat(ctx, RetireSeatCommand{
		SessionID: sessionID, SeatID: "seat-agent-1", ModeratorSeat: "seat-moderator",
		EmittedAt: utcTime(10), CorrelationID: correlation,
	}); !errors.Is(err, ErrRoundtableConflict) {
		t.Fatalf("post-round retire error = %v", err)
	}
}
