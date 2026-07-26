package projection

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

var (
	grantProjectionNow = time.Date(2026, 7, 26, 2, 3, 4, 0, time.UTC)
	grantProjectionCID = "88888888-8888-4888-8888-888888888888"
)

type grantProjectionClock struct {
	mu  sync.RWMutex
	now time.Time
}

func (clock *grantProjectionClock) Now() time.Time {
	clock.mu.RLock()
	defer clock.mu.RUnlock()
	return clock.now
}

type grantProjectionFixture struct {
	dsn            string
	db             *sql.DB
	store          *journal.Store
	clock          *grantProjectionClock
	workAuthority  *work.Authority
	run            work.RunRecord
	grantAuthority *authorization.Authority
	issued         authorization.IssuedGrant
}

func newGrantProjectionFixture(t testing.TB) *grantProjectionFixture {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	dsn := fmt.Sprintf(
		"file:%s/grant-projection.db?%s",
		t.TempDir(),
		values.Encode(),
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	clock := &grantProjectionClock{now: grantProjectionNow}
	seedGrantProjectionRuntime(t, store)
	workAuthority, err := work.NewAuthority(
		store,
		clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x21}, 64)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	workItem, run, err := workAuthority.CreateAndAssign(
		context.Background(),
		work.WorkItemAssignmentInput{
			WorkItemID:      "projection-work",
			Title:           "grant projection",
			RunID:           "projection-run",
			AgentInstanceID: "projection-agent",
			CorrelationID:   grantProjectionCID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = workAuthority.Claim(
		context.Background(),
		work.RunClaimInput{
			WorkItemID:           workItem.ID(),
			RunID:                run.ID(),
			RuntimeInstanceID:    "projection-runtime",
			AgentInstanceID:      "projection-agent",
			PrepareLeaseDuration: 5 * time.Minute,
			CorrelationID:        grantProjectionCID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	random := make([]byte, 48)
	for index := range random {
		random[index] = 0x41 + byte(index)
	}
	grantAuthority, err := authorization.NewAuthority(
		store,
		workAuthority,
		clock.Now,
		bytes.NewReader(random),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	issued, err := grantAuthority.Issue(
		context.Background(),
		authorization.IssueInput{
			WorkItemID:        workItem.ID(),
			RunID:             run.ID(),
			ClaimID:           run.ClaimID(),
			ClaimGeneration:   run.ClaimGeneration(),
			RuntimeInstanceID: run.RuntimeInstanceID(),
			AgentInstanceID:   run.AgentInstanceID(),
			AllowedOperations: []authorization.Operation{
				authorization.OperationBridgeEvent,
				authorization.OperationBridgeResult,
				authorization.OperationEvidenceStage,
			},
			Lifetime:      time.Minute,
			CorrelationID: grantProjectionCID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = grantAuthority.Authorize(
		context.Background(),
		authorization.AuthorizeInput{
			Token:             issued.Token(),
			WorkItemID:        workItem.ID(),
			RunID:             run.ID(),
			ClaimID:           run.ClaimID(),
			ClaimGeneration:   run.ClaimGeneration(),
			RuntimeInstanceID: run.RuntimeInstanceID(),
			AgentInstanceID:   run.AgentInstanceID(),
			Operation:         authorization.OperationBridgeEvent,
			RequestID:         "99999999-9999-4999-8999-999999999999",
			CorrelationID:     grantProjectionCID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return &grantProjectionFixture{
		dsn:            dsn,
		db:             db,
		store:          store,
		clock:          clock,
		workAuthority:  workAuthority,
		run:            run,
		grantAuthority: grantAuthority,
		issued:         issued,
	}
}

func seedGrantProjectionRuntime(t testing.TB, store *journal.Store) {
	t.Helper()
	payload := map[string]any{
		"discovery_digest": strings.Repeat("b", 64),
		"source_probe_id":  "projection-probe",
		"instance": map[string]any{
			"id":                    "projection-runtime",
			"device_id":             "projection-device",
			"adapter_type":          "pi-cli",
			"display_name":          "Projection Fixture",
			"executable_version":    "1.0.0",
			"status":                "online",
			"observed_capabilities": []string{"models"},
			"capacity":              1,
		},
		"model_ids": []string{},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Append(context.Background(), journal.Event{
		ID:             "projection-runtime-discovered",
		StreamID:       "runtime_instance:projection-runtime",
		Seq:            1,
		IdempotencyKey: "projection-runtime-discovered",
		Type:           "RuntimeInstanceDiscovered",
		SchemaVersion:  1,
		EmittedAt:      grantProjectionNow.Add(-time.Minute),
		CorrelationID:  grantProjectionCID,
		PayloadJSON:    body,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGrantProjectionRebuildFailureIsolation(t *testing.T) { // s3_w3_projection_rebuild_failure_isolation
	fixture := newGrantProjectionFixture(t)
	readModel := New(fixture.db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	snapshot := readModel.Snapshot()
	if len(snapshot.AgentGrants) != 1 {
		t.Fatalf("AgentGrants = %#v", snapshot.AgentGrants)
	}
	record, ok := snapshot.AgentGrants[fixture.issued.Record().ID()]
	if !ok {
		t.Fatalf("missing grant %q", fixture.issued.Record().ID())
	}
	if record.ID != fixture.issued.Record().ID() ||
		record.WorkItemID != "projection-work" ||
		record.RunID != "projection-run" ||
		record.ClaimID != fixture.run.ClaimID() ||
		record.ClaimGeneration != fixture.run.ClaimGeneration() ||
		record.RuntimeInstanceID != "projection-runtime" ||
		record.AgentInstanceID != "projection-agent" ||
		!record.IssuedAt.Equal(grantProjectionNow) ||
		!record.ExpiresAt.Equal(grantProjectionNow.Add(time.Minute)) ||
		!record.RevokedAt.IsZero() ||
		record.RevocationReason != "" {
		t.Fatalf("projected Grant = %#v", record)
	}
	if !reflect.DeepEqual(record.AllowedOperations, []string{
		"bridge.event",
		"bridge.result",
		"evidence.stage",
	}) {
		t.Fatalf("projected operations = %#v", record.AllowedOperations)
	}
	serialized, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(serialized, []byte(fixture.issued.Token().Value())) ||
		bytes.Contains(serialized, []byte("token_hash")) {
		t.Fatalf("projection disclosed token material: %s", serialized)
	}

	mutated := snapshot.AgentGrants[record.ID]
	mutated.AllowedOperations[0] = "mutated"
	snapshot.AgentGrants[record.ID] = mutated
	delete(snapshot.AgentGrants, record.ID)
	fresh := readModel.Snapshot()
	if fresh.AgentGrants[record.ID].AllowedOperations[0] != "bridge.event" {
		t.Fatal("Snapshot AgentGrant aliases caller")
	}

	before := readModel.Snapshot()
	allEvents, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	badReference := cloneProjectionEvents(allEvents)
	for index := range badReference {
		if badReference[index].Type != "AgentGrantIssued" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(badReference[index].PayloadJSON, &payload); err != nil {
			t.Fatal(err)
		}
		payload["run_sequence"] = float64(99)
		badReference[index].PayloadJSON = projectionPayload(t, payload)
		break
	}
	if _, err := replay(context.Background(), badReference); err == nil {
		t.Fatal("fabricated Grant Run reference replay succeeded")
	}

	_, err = fixture.store.Append(context.Background(), journal.Event{
		ID:             "corrupt-grant-event",
		StreamID:       "agent-grant/projection-run",
		Seq:            3,
		IdempotencyKey: "corrupt-grant-event",
		Type:           "AgentGrantAuthorized",
		SchemaVersion:  1,
		EmittedAt:      grantProjectionNow.Add(time.Second),
		CorrelationID:  grantProjectionCID,
		CausationID:    "missing",
		PayloadJSON:    []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err == nil {
		t.Fatal("malformed Grant rebuild succeeded")
	}
	after := readModel.Snapshot()
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("failed rebuild changed snapshot: before=%#v after=%#v", before, after)
	}
}

func TestGrantProjectionRealJournalRestartAndRevocation(t *testing.T) {
	fixture := newGrantProjectionFixture(t)
	revoked, err := fixture.grantAuthority.Revoke(
		context.Background(),
		authorization.RevokeInput{
			GrantID:       fixture.issued.Record().ID(),
			Reason:        authorization.RevocationTerminal,
			CorrelationID: grantProjectionCID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sql.Open("sqlite", fixture.dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	readModel := New(reopened)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatalf("restart Rebuild() error = %v", err)
	}
	record := readModel.Snapshot().AgentGrants[revoked.ID()]
	if record.ID != revoked.ID() ||
		record.RevocationReason != string(authorization.RevocationTerminal) ||
		record.RevokedAt.IsZero() {
		t.Fatalf("restarted Grant = %#v", record)
	}
}

func TestGrantProjectionRejectsDuplicateAndTrailingJSON(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			"identical duplicate root key",
			`{"run_id":"projection-run","run_id":"projection-run"}`,
		},
		{
			"identical duplicate nested key",
			`{"binding":{"run_id":"projection-run","run_id":"projection-run"}}`,
		},
		{
			"trailing top-level value",
			`{"run_id":"projection-run"}{"run_id":"projection-run"}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var target any
			err := decodeExactProjectionPayload(
				journal.Event{PayloadJSON: []byte(test.payload)},
				&target,
			)
			if err == nil {
				t.Fatalf("decodeExactProjectionPayload accepted %s", test.name)
			}
		})
	}
}
