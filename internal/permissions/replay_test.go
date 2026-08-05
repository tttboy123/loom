package permissions

import (
	"encoding/json"
	"testing"

	"loom-pi-rebuild/internal/journal"
)

func TestRed10_ReplayRebuildsAndRejectsUnknown(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	mustProfile(t, auth, ProfileInput{ProfileID: permTestProfileA, Mode: ModeAuto, OwnedPaths: []string{"src/**"}})
	mustBind(t, auth, permTestJobA, permTestProfileA)
	events := allEvents(t, store)

	first := replayOf(t, events)
	second := replayOf(t, events)
	profile := first.Profiles[permTestProfileA]
	if profile.Generation != 1 || profile.Mode != ModeAuto {
		t.Fatalf("replayed profile = %+v", profile)
	}
	if second.Profiles[permTestProfileA].Digest != profile.Digest {
		t.Fatal("digest mismatch across replays")
	}

	bad := append([]journal.Event{}, events...)
	bad = append(bad, journal.Event{
		ID: "bad-1", StreamID: "permission-profile/" + permTestProfileA, Seq: 99,
		Type: "PermissionProfileMystery", SchemaVersion: 1,
	})
	if _, err := Replay(bad); err == nil {
		t.Fatal("unknown event type must error")
	}
}

func TestRed15_ProjectionFailureKeepsOldView(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	mustProfile(t, auth, ProfileInput{ProfileID: permTestProfileA, Mode: ModeDefault})
	mustBind(t, auth, permTestJobA, permTestProfileA)
	good := allEvents(t, store)

	goodView := replayOf(t, good)
	if goodView.Profiles[permTestProfileA].ProfileID == "" {
		t.Fatal("expected a populated view")
	}
	corrupt := append([]journal.Event{}, good...)
	corrupt = append(corrupt, journal.Event{
		ID: "corrupt-1", StreamID: "permission-binding/" + permTestJobA, Seq: 999,
		Type: "JobPermissionBound", SchemaVersion: 1,
		PayloadJSON: []byte(`{"job_id":"","profile_id":""}`),
	})
	if _, err := Replay(corrupt); err == nil {
		t.Fatal("corrupt event must error")
	}
	// The old view remains usable: zero mutation, zero side effects.
	if goodView.Profiles[permTestProfileA].Mode != ModeDefault {
		t.Fatal("old view must be preserved after a failed replay")
	}
}

func TestRed20_EffectiveModeDeterministicAcrossActivations(t *testing.T) {
	projection := &Projection{
		Profiles:     make(map[string]PermissionProfile),
		Bindings:     make(map[string]JobBinding),
		Rules:        make(map[string]Rule),
		Grants:       make(map[string]Grant),
		Activations:  make(map[string]Activation),
		ApprovalView: make(map[string]Approval),
		retired:      make(map[string]bool),
	}
	projection.Activations["personal/zz"] = Activation{
		Mode: ModeAuto, Scope: ScopePersonal, ScopeID: "zz", ActivatedAt: "2026-08-05T12:00:00Z",
	}
	projection.Activations["personal/aa"] = Activation{
		Mode: ModeDontAsk, Scope: ScopePersonal, ScopeID: "aa", ActivatedAt: "2026-08-05T12:00:00Z",
	}
	mode, err := EffectiveMode(projection, "job-x")
	if err != nil {
		t.Fatal(err)
	}
	if mode != ModeDontAsk {
		t.Fatalf("EffectiveMode = %s, want deterministic dont_ask (lexicographically smallest scopeID)", mode)
	}
}

func TestRed20_ActivationStreamMismatchRejected(t *testing.T) {
	payload, err := json.Marshal(activationPayload{
		Mode: ModeAuto, Scope: ScopePersonal, ScopeID: "global",
		ActivatedAt: "2026-08-05T12:00:00Z", AuthorizedBy: "human-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	event := journal.Event{
		ID: "perm1-wrong-stream", StreamID: "permission-rule/r-1",
		Type: "PermissionActivationActivated", SchemaVersion: 1,
		PayloadJSON: payload,
	}
	if _, err := Replay([]journal.Event{event}); err == nil {
		t.Fatal("activation on a non-activation stream must fail replay")
	}
}

func TestReplayRejectsInvalidProfileEventPayload(t *testing.T) {
	store := openPermStore(t)
	events := allEvents(t, store)
	events = append(events, journal.Event{
		ID: "inv-1", StreamID: "permission-profile/" + permTestProfileA, Seq: 1,
		Type: "PermissionProfileDefined", SchemaVersion: 1,
		PayloadJSON: json.RawMessage(`{"profile_id":"","generation":1,"digest":"x","mode":"default"}`),
	})
	if _, err := Replay(events); err == nil {
		t.Fatal("invalid profile payload must error")
	}
}

func TestEffectiveModePrecedence(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	mustProfile(t, auth, ProfileInput{ProfileID: permTestProfileA, Mode: ModeAuto})
	mustBind(t, auth, permTestJobA, permTestProfileA)
	projection := replayOf(t, allEvents(t, store))
	mode, err := EffectiveMode(projection, permTestJobA)
	if err != nil {
		t.Fatalf("EffectiveMode() error = %v", err)
	}
	if mode != ModeAuto {
		t.Fatalf("mode = %s, want profile mode auto", mode)
	}
	mode, err = EffectiveMode(projection, "unbound-job")
	if err != nil {
		t.Fatalf("EffectiveMode(unbound) error = %v", err)
	}
	if mode != ModeDefault {
		t.Fatalf("unbound mode = %s, want default", mode)
	}
}
