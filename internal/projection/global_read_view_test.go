package projection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
)

type errorEventSource struct {
	err error
}

func (source errorEventSource) Events(context.Context) ([]journal.Event, error) {
	return nil, source.err
}

func TestGlobalReadViewPublishesAtomicallyWithCanonicalVersionAndLocalCopies(t *testing.T) {
	create := projectionEvent(
		"evt-work-create",
		"view-stream",
		1,
		"idem-work-create",
		"WorkItemCreated",
		map[string]string{
			"work_item_id": "work-1",
			"title":        "View boundary",
			"status":       "open",
		},
	)
	projection := newForTestSource(eventSliceSource{events: []journal.Event{create}})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	view := projection.GlobalReadView()
	sum := sha256.Sum256([]byte("view-stream\x001\x00evt-work-create\n"))
	if got, want := view.Version(), hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("Version() = %q, want %q", got, want)
	}
	head, ok := view.Head("view-stream")
	if !ok || head.Sequence != 1 || head.EventID != create.ID {
		t.Fatalf("Head() = %#v, %v", head, ok)
	}
	workItem, ok := view.WorkItem("work-1")
	if !ok || workItem.Title != "View boundary" {
		t.Fatalf("WorkItem() = %#v, %v", workItem, ok)
	}
	workItem.Title = "mutated"
	if again, _ := view.WorkItem("work-1"); again.Title != "View boundary" {
		t.Fatalf("view record mutation escaped: %#v", again)
	}

	failing := newForTestSource(errorEventSource{err: errors.New("source failed")})
	failing.snapshot = projection.Snapshot()
	failing.view = view
	if err := failing.Rebuild(context.Background()); err == nil {
		t.Fatal("failing Rebuild() error = nil")
	}
	if got := failing.GlobalReadView().Version(); got != view.Version() {
		t.Fatalf("failed rebuild replaced old view: %q != %q", got, view.Version())
	}
}

func TestEmptyGlobalReadViewUsesEmptySHA256(t *testing.T) {
	projection := newForTestSource(eventSliceSource{})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(nil)
	if got, want := projection.GlobalReadView().Version(), hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("empty version = %q, want %q", got, want)
	}
}

func TestGlobalReadViewReturnsLatestAgentGrantForRunBySequence(t *testing.T) {
	issuedAt := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	snapshot := emptySnapshot()
	snapshot.AgentGrants["grant-old"] = AgentGrant{
		ID:                "grant-old",
		RunID:             "run-1",
		AllowedOperations: []string{"bridge.event"},
		IssuedAt:          issuedAt,
	}
	snapshot.AgentGrants["grant-new"] = AgentGrant{
		ID:                "grant-new",
		RunID:             "run-1",
		AllowedOperations: []string{"bridge.event", "bridge.result"},
		IssuedAt:          issuedAt.Add(time.Second),
		RevokedAt:         issuedAt.Add(2 * time.Second),
		RevocationReason:  "operator",
	}
	issuePayload := func(grantID string, issued time.Time) map[string]any {
		return map[string]any{
			"grant_id":            grantID,
			"work_item_id":        "work-1",
			"run_id":              "run-1",
			"claim_id":            "11111111-1111-4111-8111-111111111111",
			"claim_generation":    1,
			"runtime_instance_id": "runtime-1",
			"agent_instance_id":   "agent-1",
			"allowed_operations":  []string{"bridge.event"},
			"token_hash":          strings.Repeat("a", 64),
			"issued_at":           issued.Format(time.RFC3339Nano),
			"expires_at":          issued.Add(time.Minute).Format(time.RFC3339Nano),
			"run_stream":          "run/run-1",
			"run_sequence":        1,
			"run_event_id":        "run-claimed",
		}
	}
	events := []journal.Event{
		teamProjectionEvent(
			t,
			"grant-old-issued",
			"agent-grant/run-1",
			1,
			"grant-old-issued",
			"AgentGrantIssued",
			issuePayload("grant-old", issuedAt),
		),
		teamProjectionEvent(
			t,
			"grant-new-issued",
			"agent-grant/run-1",
			2,
			"grant-new-issued",
			"AgentGrantIssued",
			issuePayload("grant-new", issuedAt.Add(time.Second)),
		),
	}
	view := buildGlobalReadView(snapshot, events, nil)
	latest, ok := view.LatestAgentGrantForRun("run-1")
	if !ok || latest.ID != "grant-new" ||
		latest.RevocationReason != "operator" {
		t.Fatalf("LatestAgentGrantForRun() = %#v, %v", latest, ok)
	}
	latest.AllowedOperations[0] = "mutated"
	again, _ := view.LatestAgentGrantForRun("run-1")
	if again.AllowedOperations[0] != "bridge.event" {
		t.Fatal("latest AgentGrant aliases caller mutation")
	}
}
