package work

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"

	_ "modernc.org/sqlite"
)

const (
	testCorrelation = "11111111-1111-4111-8111-111111111111"
	testDigest      = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

var testNow = time.Date(2026, 7, 26, 1, 2, 3, 0, time.UTC)

type mutableClock struct {
	mu  sync.Mutex
	now time.Time
}

func (clock *mutableClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *mutableClock) Set(now time.Time) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = now
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}

func openAuthorityStore(t testing.TB) *journal.Store {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s/authority.db?%s",
		t.TempDir(),
		values.Encode(),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return journal.NewStore(db)
}

func newAuthority(t testing.TB, store *journal.Store, clock *mutableClock, seed byte) *Authority {
	t.Helper()
	randomBytes := make([]byte, 16*32)
	for index := range randomBytes {
		randomBytes[index] = seed + byte(index/16)
	}
	random := bytes.NewReader(randomBytes)
	authority, err := NewAuthority(store, clock.Now, random)
	if err != nil {
		t.Fatalf("NewAuthority() error = %v", err)
	}
	if err := authority.InitializeRunIdentityIndex(context.Background()); err != nil {
		t.Fatalf("InitializeRunIdentityIndex() error = %v", err)
	}
	return authority
}

func TestRunIdentityIndexIsExplicitAndRejectsCrossWorkItemCollision(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	uninitialized, err := NewAuthority(
		store,
		clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x21}, 64)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := uninitialized.CreateAndAssign(
		context.Background(),
		assignment("work-before-index", "run-shared"),
	); !errors.Is(err, ErrRunIdentityIndexRequired) {
		t.Fatalf("uninitialized CreateAndAssign() error = %v", err)
	}
	if err := uninitialized.InitializeRunIdentityIndex(context.Background()); err != nil {
		t.Fatalf("InitializeRunIdentityIndex() error = %v", err)
	}
	if _, _, err := uninitialized.CreateAndAssign(
		context.Background(),
		assignment("work-first", "run-shared"),
	); err != nil {
		t.Fatalf("first CreateAndAssign() error = %v", err)
	}
	if _, _, err := uninitialized.CreateAndAssign(
		context.Background(),
		assignment("work-second", "run-shared"),
	); !errors.Is(err, ErrRunAuthorityConflict) {
		t.Fatalf("cross-work collision error = %v", err)
	}
}

func seedRuntime(t testing.TB, store *journal.Store, runtimeID, status string, capacity int) {
	t.Helper()
	payload := map[string]any{
		"discovery_digest": testDigest,
		"source_probe_id":  "probe-1",
		"instance": map[string]any{
			"id":                    runtimeID,
			"device_id":             "device-1",
			"adapter_type":          "fixture",
			"display_name":          "Fixture Runtime",
			"executable_version":    "1.0.0",
			"status":                status,
			"observed_capabilities": []string{},
			"capacity":              capacity,
		},
		"model_ids": []string{},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	event := journal.Event{
		ID:             "runtime-discovered-" + runtimeID,
		StreamID:       "runtime_instance:" + runtimeID,
		Seq:            1,
		IdempotencyKey: "runtime-discovered-" + runtimeID,
		Type:           "RuntimeInstanceDiscovered",
		SchemaVersion:  1,
		EmittedAt:      testNow.Add(-time.Minute),
		CorrelationID:  "22222222-2222-4222-8222-222222222222",
		PayloadJSON:    body,
	}
	if _, err := store.Append(context.Background(), event); err != nil {
		t.Fatalf("seed runtime: %v", err)
	}
}

func appendRuntimeStatus(
	t testing.TB,
	store *journal.Store,
	runtimeID string,
	fromStatus string,
	toStatus string,
) journal.Event {
	t.Helper()
	event := runtimeStatusEvent(t, runtimeID, fromStatus, toStatus)
	if _, err := store.Append(context.Background(), event); err != nil {
		t.Fatalf("append runtime status: %v", err)
	}
	return event
}

func runtimeStatusEvent(
	t testing.TB,
	runtimeID string,
	fromStatus string,
	toStatus string,
) journal.Event {
	t.Helper()
	previousID := "runtime-discovered-" + runtimeID
	payload := map[string]any{
		"reconciliation_digest":   testDigest,
		"baseline_digest":         testDigest,
		"source_discovery_digest": testDigest,
		"source_probe_id":         "probe-1",
		"runtime_instance_id":     runtimeID,
		"device_id":               "device-1",
		"adapter_type":            "fixture",
		"from_status":             fromStatus,
		"to_status":               toStatus,
		"previous_event_id":       previousID,
		"previous_sequence":       1,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	event := journal.Event{
		ID:             "runtime-status-" + runtimeID + "-" + toStatus,
		StreamID:       "runtime_instance:" + runtimeID,
		Seq:            2,
		IdempotencyKey: "runtime-status-" + runtimeID + "-" + toStatus,
		Type:           "RuntimeInstanceStatusChanged",
		SchemaVersion:  1,
		EmittedAt:      testNow.Add(-30 * time.Second),
		CorrelationID:  "33333333-3333-4333-8333-333333333333",
		CausationID:    previousID,
		PayloadJSON:    body,
	}
	return event
}

func assertRuntimeStatusReference(
	t testing.TB,
	payloadJSON []byte,
	runtimeID string,
	sequence int64,
	eventID string,
) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		t.Fatalf("decode status reference payload: %v", err)
	}
	if payload["runtime_status_stream_id"] != "runtime_instance:"+runtimeID ||
		payload["runtime_status_sequence"] != float64(sequence) ||
		payload["runtime_status_event_id"] != eventID {
		t.Fatalf(
			"runtime status reference = stream=%v sequence=%v event=%v",
			payload["runtime_status_stream_id"],
			payload["runtime_status_sequence"],
			payload["runtime_status_event_id"],
		)
	}
}

func assignment(workItemID, runID string) WorkItemAssignmentInput {
	return WorkItemAssignmentInput{
		WorkItemID:      workItemID,
		Title:           "Implement " + workItemID,
		RunID:           runID,
		AgentInstanceID: "agent-1",
		CorrelationID:   testCorrelation,
	}
}

func claim(workItemID, runID, runtimeID string) RunClaimInput {
	return RunClaimInput{
		WorkItemID:           workItemID,
		RunID:                runID,
		RuntimeInstanceID:    runtimeID,
		AgentInstanceID:      "agent-1",
		PrepareLeaseDuration: time.Minute,
		CorrelationID:        testCorrelation,
	}
}

func generationInput(run RunRecord) RunGenerationInput {
	return RunGenerationInput{
		WorkItemID:        run.WorkItemID(),
		RunID:             run.ID(),
		ClaimID:           run.ClaimID(),
		ClaimGeneration:   run.ClaimGeneration(),
		RuntimeInstanceID: run.RuntimeInstanceID(),
		AgentInstanceID:   run.AgentInstanceID(),
		CorrelationID:     testCorrelation,
	}
}

func assertZeroWorkItem(t testing.TB, record WorkItemRecord) {
	t.Helper()
	if record.ID() != "" || record.Title() != "" || record.Status() != "" ||
		record.RunID() != "" || record.AgentInstanceID() != "" {
		t.Fatalf("non-zero WorkItemRecord: %#v", record)
	}
}

func assertZeroRun(t testing.TB, record RunRecord) {
	t.Helper()
	if record.ID() != "" || record.WorkItemID() != "" || record.Phase() != "" ||
		record.ClaimID() != "" || record.ClaimGeneration() != 0 ||
		record.RuntimeInstanceID() != "" || record.AgentInstanceID() != "" ||
		!record.PrepareLeaseExpiresAt().IsZero() ||
		record.TerminalStatus() != "" || record.TerminalReason() != "" {
		t.Fatalf("non-zero RunRecord: %#v", record)
	}
}

func TestCreateAndAssignExactRetryConflict(t *testing.T) { // s3_w2_create_assign_exact_retry_conflict
	t.Parallel()
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 1)
	ctx := context.Background()
	input := assignment("work-1", "run-1")

	workItem, run, err := authority.CreateAndAssign(ctx, input)
	if err != nil {
		t.Fatalf("CreateAndAssign() error = %v", err)
	}
	if workItem.ID() != "work-1" || workItem.Title() != "Implement work-1" ||
		workItem.Status() != "assigned" || workItem.RunID() != "run-1" ||
		workItem.AgentInstanceID() != "agent-1" {
		t.Fatalf("work item = %#v", workItem)
	}
	if run.ID() != "run-1" || run.WorkItemID() != "work-1" ||
		run.Phase() != "unclaimed" || run.AgentInstanceID() != "agent-1" {
		t.Fatalf("run = %#v", run)
	}
	events, err := store.ReadStream(ctx, "work-item/work-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 ||
		events[0].Type != "WorkItemCreated" || events[0].Seq != 1 ||
		events[1].Type != "WorkItemAssigned" || events[1].Seq != 2 ||
		events[0].CorrelationID != testCorrelation ||
		events[0].CausationID != "" ||
		events[1].CausationID != events[0].ID {
		t.Fatalf("assignment events = %#v", events)
	}
	assertExactJSON(t, events[0].PayloadJSON, map[string]any{
		"work_item_id": "work-1", "title": "Implement work-1", "status": "ready",
	})
	assertExactJSON(t, events[1].PayloadJSON, map[string]any{
		"work_item_id": "work-1", "run_id": "run-1",
		"agent_instance_id": "agent-1", "status": "assigned",
	})

	retryWork, retryRun, err := authority.CreateAndAssign(ctx, input)
	if err != nil {
		t.Fatalf("exact retry error = %v", err)
	}
	if retryWork.ID() != workItem.ID() || retryRun.ID() != run.ID() {
		t.Fatalf("retry records changed")
	}
	retried, _ := store.ReadStream(ctx, "work-item/work-1")
	if len(retried) != 2 {
		t.Fatalf("retry event count = %d, want 2", len(retried))
	}

	conflict := input
	conflict.Title = "Different"
	gotWork, gotRun, err := authority.CreateAndAssign(ctx, conflict)
	if !errors.Is(err, ErrRunAuthorityConflict) {
		t.Fatalf("conflict error = %v, want ErrRunAuthorityConflict", err)
	}
	assertZeroWorkItem(t, gotWork)
	assertZeroRun(t, gotRun)
}

func TestClaimRuntimeStatusCapacityAtomicity(t *testing.T) { // s3_w2_claim_runtime_status_capacity_atomicity
	t.Parallel()

	t.Run("claim writes matched run and runtime facts", func(t *testing.T) {
		store := openAuthorityStore(t)
		seedRuntime(t, store, "runtime-1", "online", 1)
		clock := &mutableClock{now: testNow}
		authority := newAuthority(t, store, clock, 2)
		workItem, run, err := authority.CreateAndAssign(context.Background(), assignment("work-1", "run-1"))
		if err != nil {
			t.Fatal(err)
		}
		workItem, run, err = authority.Claim(context.Background(), claim("work-1", "run-1", "runtime-1"))
		if err != nil {
			t.Fatalf("Claim() error = %v", err)
		}
		if workItem.Status() != "assigned" || run.Phase() != "claimed" ||
			run.ClaimGeneration() != 1 || run.ClaimID() == "" ||
			run.RuntimeInstanceID() != "runtime-1" ||
			!run.PrepareLeaseExpiresAt().Equal(testNow.Add(time.Minute)) {
			t.Fatalf("claimed records: work=%#v run=%#v", workItem, run)
		}
		runEvents, _ := store.ReadStream(context.Background(), "run/run-1")
		statusEvents, _ := store.ReadStream(context.Background(), "runtime_instance:runtime-1")
		capacityEvents, _ := store.ReadStream(context.Background(), "runtime_capacity:runtime-1")
		if len(runEvents) != 1 || runEvents[0].Type != "RunClaimed" ||
			len(statusEvents) != 1 || statusEvents[0].Type != "RuntimeInstanceDiscovered" ||
			len(capacityEvents) != 1 || capacityEvents[0].Type != "RuntimeCapacityReserved" ||
			capacityEvents[0].CausationID != runEvents[0].ID {
			t.Fatalf(
				"claim facts: run=%#v status=%#v capacity=%#v",
				runEvents, statusEvents, capacityEvents,
			)
		}
		assertExactJSON(t, runEvents[0].PayloadJSON, map[string]any{
			"work_item_id": "work-1", "run_id": "run-1",
			"claim_id": run.ClaimID(), "claim_generation": float64(1),
			"runtime_instance_id": "runtime-1", "agent_instance_id": "agent-1",
			"prepare_lease_expires_at": testNow.Add(time.Minute).Format(time.RFC3339Nano),
			"runtime_status_stream_id": "runtime_instance:runtime-1",
			"runtime_status_sequence":  float64(1),
			"runtime_status_event_id":  "runtime-discovered-runtime-1",
		})
		assertExactJSON(t, capacityEvents[0].PayloadJSON, map[string]any{
			"work_item_id": "work-1", "run_id": "run-1",
			"claim_id": run.ClaimID(), "claim_generation": float64(1),
			"runtime_instance_id": "runtime-1", "agent_instance_id": "agent-1",
			"runtime_status_stream_id": "runtime_instance:runtime-1",
			"runtime_status_sequence":  float64(1),
			"runtime_status_event_id":  "runtime-discovered-runtime-1",
		})
	})

	t.Run("missing offline and full runtimes fail closed", func(t *testing.T) {
		for _, testCase := range []struct {
			name     string
			seed     bool
			status   string
			capacity int
			want     error
		}{
			{"missing", false, "", 0, ErrRuntimeUnavailable},
			{"offline", true, "offline", 1, ErrRuntimeUnavailable},
			{"zero capacity", true, "online", 0, ErrRuntimeUnavailable},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				store := openAuthorityStore(t)
				if testCase.seed {
					seedRuntime(t, store, "runtime-1", testCase.status, testCase.capacity)
				}
				clock := &mutableClock{now: testNow}
				authority := newAuthority(t, store, clock, 3)
				if _, _, err := authority.CreateAndAssign(context.Background(), assignment("work-1", "run-1")); err != nil {
					t.Fatal(err)
				}
				workItem, run, err := authority.Claim(context.Background(), claim("work-1", "run-1", "runtime-1"))
				if !errors.Is(err, testCase.want) {
					t.Fatalf("error = %v, want %v", err, testCase.want)
				}
				assertZeroWorkItem(t, workItem)
				assertZeroRun(t, run)
				runEvents, _ := store.ReadStream(context.Background(), "run/run-1")
				if len(runEvents) != 0 {
					t.Fatalf("failed claim wrote %d run events", len(runEvents))
				}
			})
		}
	})

	t.Run("capacity one concurrent claims commit exactly one", func(t *testing.T) {
		store := openAuthorityStore(t)
		seedRuntime(t, store, "runtime-1", "online", 1)
		clock := &mutableClock{now: testNow}
		left := newAuthority(t, store, clock, 4)
		right := newAuthority(t, store, clock, 5)
		if _, _, err := left.CreateAndAssign(context.Background(), assignment("work-left", "run-left")); err != nil {
			t.Fatal(err)
		}
		if _, _, err := right.CreateAndAssign(context.Background(), assignment("work-right", "run-right")); err != nil {
			t.Fatal(err)
		}

		start := make(chan struct{})
		results := make(chan error, 2)
		go func() {
			<-start
			_, _, err := left.Claim(context.Background(), claim("work-left", "run-left", "runtime-1"))
			results <- err
		}()
		go func() {
			<-start
			_, _, err := right.Claim(context.Background(), claim("work-right", "run-right", "runtime-1"))
			results <- err
		}()
		close(start)
		var success, rejected int
		for index := 0; index < 2; index++ {
			err := <-results
			if err == nil {
				success++
			} else if errors.Is(err, ErrRunAuthorityConflict) ||
				errors.Is(err, ErrRuntimeCapacityExhausted) {
				rejected++
			} else {
				t.Fatalf("unexpected claim error = %v", err)
			}
		}
		if success != 1 || rejected != 1 {
			t.Fatalf("success=%d rejected=%d, want 1/1", success, rejected)
		}
		statusEvents, _ := store.ReadStream(context.Background(), "runtime_instance:runtime-1")
		capacityEvents, _ := store.ReadStream(context.Background(), "runtime_capacity:runtime-1")
		leftEvents, _ := store.ReadStream(context.Background(), "run/run-left")
		rightEvents, _ := store.ReadStream(context.Background(), "run/run-right")
		if len(statusEvents) != 1 || len(capacityEvents) != 1 ||
			len(leftEvents)+len(rightEvents) != 1 {
			t.Fatalf(
				"partial/extra facts: status=%d capacity=%d left=%d right=%d",
				len(statusEvents), len(capacityEvents), len(leftEvents), len(rightEvents),
			)
		}
	})

	t.Run("status transition races serialize without partial capacity facts", func(t *testing.T) {
		for attempt := 0; attempt < 20; attempt++ {
			store := openAuthorityStore(t)
			seedRuntime(t, store, "runtime-1", "online", 1)
			clock := &mutableClock{now: testNow}
			authority := newAuthority(t, store, clock, byte(20+attempt))
			if _, _, err := authority.CreateAndAssign(
				context.Background(),
				assignment("work-1", "run-1"),
			); err != nil {
				t.Fatal(err)
			}

			start := make(chan struct{})
			claimResult := make(chan error, 1)
			statusResult := make(chan error, 1)
			statusEvent := runtimeStatusEvent(
				t, "runtime-1", "online", "offline",
			)
			go func() {
				<-start
				_, _, err := authority.Claim(
					context.Background(),
					claim("work-1", "run-1", "runtime-1"),
				)
				claimResult <- err
			}()
			go func() {
				<-start
				_, err := store.Append(context.Background(), statusEvent)
				statusResult <- err
			}()
			close(start)

			claimErr := <-claimResult
			if statusErr := <-statusResult; statusErr != nil {
				t.Fatalf("attempt %d status append error = %v", attempt, statusErr)
			}
			runEvents, _ := store.ReadStream(context.Background(), "run/run-1")
			capacityEvents, _ := store.ReadStream(
				context.Background(), "runtime_capacity:runtime-1",
			)
			switch {
			case claimErr == nil:
				if len(runEvents) != 1 || len(capacityEvents) != 1 {
					t.Fatalf(
						"attempt %d successful claim partial facts: run=%d capacity=%d",
						attempt, len(runEvents), len(capacityEvents),
					)
				}
				assertRuntimeStatusReference(
					t, runEvents[0].PayloadJSON, "runtime-1", 1,
					"runtime-discovered-runtime-1",
				)
			case errors.Is(claimErr, ErrRunAuthorityConflict),
				errors.Is(claimErr, ErrRuntimeUnavailable):
				if len(runEvents) != 0 || len(capacityEvents) != 0 {
					t.Fatalf(
						"attempt %d rejected claim wrote facts: run=%d capacity=%d",
						attempt, len(runEvents), len(capacityEvents),
					)
				}
			default:
				t.Fatalf("attempt %d unexpected claim error = %v", attempt, claimErr)
			}
		}
	})
}

func TestLeaseExtendExpireReclaimGeneration(t *testing.T) { // s3_w2_lease_extend_expire_reclaim_generation
	t.Parallel()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-1", "online", 1)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 6)
	if _, _, err := authority.CreateAndAssign(context.Background(), assignment("work-1", "run-1")); err != nil {
		t.Fatal(err)
	}
	_, first, err := authority.Claim(context.Background(), claim("work-1", "run-1", "runtime-1"))
	if err != nil {
		t.Fatal(err)
	}

	if workItem, run, err := authority.Claim(context.Background(), claim("work-1", "run-1", "runtime-1")); !errors.Is(err, ErrRunLeaseActive) {
		t.Fatalf("active reclaim error = %v, want ErrRunLeaseActive", err)
	} else {
		assertZeroWorkItem(t, workItem)
		assertZeroRun(t, run)
	}

	clock.Set(testNow.Add(30 * time.Second))
	extended, err := authority.ExtendPrepareLease(context.Background(), generationInput(first), 2*time.Minute)
	if err != nil {
		t.Fatalf("ExtendPrepareLease() error = %v", err)
	}
	wantExpiry := testNow.Add(150 * time.Second)
	if !extended.PrepareLeaseExpiresAt().Equal(wantExpiry) {
		t.Fatalf("expiry = %s, want %s", extended.PrepareLeaseExpiresAt(), wantExpiry)
	}
	leaseEvents, _ := store.ReadStream(context.Background(), "run/run-1")
	if len(leaseEvents) != 2 ||
		leaseEvents[1].Type != "RunPrepareLeaseExtended" ||
		leaseEvents[1].CausationID != leaseEvents[0].ID {
		t.Fatalf("lease events = %#v", leaseEvents)
	}
	assertExactJSON(t, leaseEvents[1].PayloadJSON, map[string]any{
		"work_item_id": "work-1", "run_id": "run-1",
		"claim_id": first.ClaimID(), "claim_generation": float64(1),
		"runtime_instance_id": "runtime-1", "agent_instance_id": "agent-1",
		"previous_lease_expires_at": testNow.Add(time.Minute).Format(time.RFC3339Nano),
		"prepare_lease_expires_at":  wantExpiry.Format(time.RFC3339Nano),
	})

	clock.Set(wantExpiry)
	if run, err := authority.ExtendPrepareLease(context.Background(), generationInput(extended), time.Minute); !errors.Is(err, ErrRunLeaseExpired) {
		t.Fatalf("exact-expiry extend error = %v, want ErrRunLeaseExpired", err)
	} else {
		assertZeroRun(t, run)
	}

	reclaimedWork, second, err := authority.Claim(context.Background(), claim("work-1", "run-1", "runtime-1"))
	if err != nil {
		t.Fatalf("expired reclaim error = %v", err)
	}
	if reclaimedWork.ID() != "work-1" || second.ClaimGeneration() != 2 ||
		second.ClaimID() == first.ClaimID() || second.Phase() != "claimed" {
		t.Fatalf("reclaimed records: work=%#v run=%#v", reclaimedWork, second)
	}
	if run, err := authority.ExtendPrepareLease(context.Background(), generationInput(first), time.Minute); !errors.Is(err, ErrStaleClaimGeneration) {
		t.Fatalf("old generation error = %v, want ErrStaleClaimGeneration", err)
	} else {
		assertZeroRun(t, run)
	}
	statusEvents, _ := store.ReadStream(context.Background(), "runtime_instance:runtime-1")
	capacityEvents, _ := store.ReadStream(context.Background(), "runtime_capacity:runtime-1")
	if len(statusEvents) != 1 || len(capacityEvents) != 3 ||
		capacityEvents[1].Type != "RuntimeCapacityReleased" ||
		capacityEvents[2].Type != "RuntimeCapacityReserved" {
		t.Fatalf(
			"reclaim facts: status=%#v capacity=%#v",
			statusEvents, capacityEvents,
		)
	}
	runEvents, _ := store.ReadStream(context.Background(), "run/run-1")
	if len(runEvents) != 3 ||
		runEvents[2].Type != "RunClaimed" ||
		runEvents[2].CausationID != runEvents[1].ID ||
		capacityEvents[1].CausationID != runEvents[2].ID ||
		capacityEvents[2].CausationID != runEvents[2].ID {
		t.Fatalf(
			"reclaim causal order: run=%#v capacity=%#v",
			runEvents, capacityEvents,
		)
	}
	for _, event := range []journal.Event{
		runEvents[2], capacityEvents[1], capacityEvents[2],
	} {
		assertRuntimeStatusReference(
			t, event.PayloadJSON, "runtime-1", 1,
			"runtime-discovered-runtime-1",
		)
	}
}

func TestStartTerminalOnceLateGeneration(t *testing.T) { // s3_w2_start_terminal_once_late_generation
	t.Parallel()

	t.Run("start and succeeded terminal stop at ready for review", func(t *testing.T) {
		store := openAuthorityStore(t)
		seedRuntime(t, store, "runtime-1", "online", 1)
		clock := &mutableClock{now: testNow}
		authority := newAuthority(t, store, clock, 7)
		if _, _, err := authority.CreateAndAssign(context.Background(), assignment("work-1", "run-1")); err != nil {
			t.Fatal(err)
		}
		_, claimed, err := authority.Claim(context.Background(), claim("work-1", "run-1", "runtime-1"))
		if err != nil {
			t.Fatal(err)
		}
		clock.Set(testNow.Add(30 * time.Second))
		workItem, running, err := authority.Start(context.Background(), generationInput(claimed))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if workItem.Status() != "running" || running.Phase() != "running" {
			t.Fatalf("start records: work=%#v run=%#v", workItem, running)
		}

		clock.Set(testNow.Add(10 * time.Minute))
		if workItem, run, err := authority.Claim(context.Background(), claim("work-1", "run-1", "runtime-1")); !errors.Is(err, ErrRunNotClaimable) {
			t.Fatalf("running reclaim error = %v, want ErrRunNotClaimable", err)
		} else {
			assertZeroWorkItem(t, workItem)
			assertZeroRun(t, run)
		}

		terminalInput := RunTerminalInput{
			RunGenerationInput: generationInput(running),
			Status:             "succeeded",
		}
		workItem, terminal, err := authority.CommitTerminal(context.Background(), terminalInput)
		if err != nil {
			t.Fatalf("CommitTerminal() error = %v", err)
		}
		if workItem.Status() != "ready_for_review" || terminal.Phase() != "terminal" ||
			terminal.TerminalStatus() != "succeeded" || terminal.TerminalReason() != "" {
			t.Fatalf("terminal records: work=%#v run=%#v", workItem, terminal)
		}
		workEvents, _ := store.ReadStream(context.Background(), "work-item/work-1")
		runEvents, _ := store.ReadStream(context.Background(), "run/run-1")
		statusEvents, _ := store.ReadStream(context.Background(), "runtime_instance:runtime-1")
		capacityEvents, _ := store.ReadStream(context.Background(), "runtime_capacity:runtime-1")
		if workEvents[len(workEvents)-1].Type != "WorkItemReadyForReview" ||
			runEvents[len(runEvents)-1].Type != "RunTerminalCommitted" ||
			len(statusEvents) != 1 ||
			capacityEvents[len(capacityEvents)-1].Type != "RuntimeCapacityReleased" {
			t.Fatalf("terminal fact types: work=%s run=%s status=%#v capacity=%#v",
				workEvents[len(workEvents)-1].Type,
				runEvents[len(runEvents)-1].Type,
				statusEvents,
				capacityEvents,
			)
		}
		if len(runEvents) != 3 || len(capacityEvents) != 2 ||
			runEvents[1].Type != "RunStarted" ||
			runEvents[1].CausationID != runEvents[0].ID ||
			runEvents[2].CausationID != runEvents[1].ID ||
			capacityEvents[1].CausationID != runEvents[2].ID ||
			workEvents[len(workEvents)-1].CausationID != runEvents[2].ID {
			t.Fatalf(
				"terminal causal order: work=%#v run=%#v capacity=%#v",
				workEvents, runEvents, capacityEvents,
			)
		}
		assertExactJSON(t, runEvents[1].PayloadJSON, map[string]any{
			"work_item_id": "work-1", "run_id": "run-1",
			"claim_id": claimed.ClaimID(), "claim_generation": float64(1),
			"runtime_instance_id": "runtime-1", "agent_instance_id": "agent-1",
			"runtime_status_stream_id": "runtime_instance:runtime-1",
			"runtime_status_sequence":  float64(1),
			"runtime_status_event_id":  "runtime-discovered-runtime-1",
		})
		assertExactJSON(t, runEvents[2].PayloadJSON, map[string]any{
			"work_item_id": "work-1", "run_id": "run-1",
			"claim_id": claimed.ClaimID(), "claim_generation": float64(1),
			"runtime_instance_id": "runtime-1", "agent_instance_id": "agent-1",
			"status": "succeeded", "reason": "",
			"runtime_status_stream_id": "runtime_instance:runtime-1",
			"runtime_status_sequence":  float64(1),
			"runtime_status_event_id":  "runtime-discovered-runtime-1",
		})
		assertExactJSON(t, capacityEvents[1].PayloadJSON, map[string]any{
			"work_item_id": "work-1", "run_id": "run-1",
			"claim_id": claimed.ClaimID(), "claim_generation": float64(1),
			"runtime_instance_id": "runtime-1", "agent_instance_id": "agent-1",
			"runtime_status_stream_id": "runtime_instance:runtime-1",
			"runtime_status_sequence":  float64(1),
			"runtime_status_event_id":  "runtime-discovered-runtime-1",
		})

		retryWork, retryRun, err := authority.CommitTerminal(context.Background(), terminalInput)
		if err != nil || retryWork.Status() != "ready_for_review" ||
			retryRun.TerminalStatus() != "succeeded" {
			t.Fatalf("terminal retry: work=%#v run=%#v error=%v", retryWork, retryRun, err)
		}
		different := terminalInput
		different.Status = "failed"
		different.Reason = "late"
		if workItem, run, err := authority.CommitTerminal(context.Background(), different); !errors.Is(err, ErrRunAlreadyTerminal) {
			t.Fatalf("different terminal error = %v, want ErrRunAlreadyTerminal", err)
		} else {
			assertZeroWorkItem(t, workItem)
			assertZeroRun(t, run)
		}
	})

	t.Run("terminal cleanup binds the current offline status head", func(t *testing.T) {
		store := openAuthorityStore(t)
		seedRuntime(t, store, "runtime-1", "online", 1)
		clock := &mutableClock{now: testNow}
		authority := newAuthority(t, store, clock, 18)
		if _, _, err := authority.CreateAndAssign(
			context.Background(),
			assignment("work-1", "run-1"),
		); err != nil {
			t.Fatal(err)
		}
		_, claimed, err := authority.Claim(
			context.Background(),
			claim("work-1", "run-1", "runtime-1"),
		)
		if err != nil {
			t.Fatal(err)
		}
		clock.Set(testNow.Add(10 * time.Second))
		_, running, err := authority.Start(
			context.Background(), generationInput(claimed),
		)
		if err != nil {
			t.Fatal(err)
		}
		statusEvent := appendRuntimeStatus(
			t, store, "runtime-1", "online", "offline",
		)

		terminalInput := RunTerminalInput{
			RunGenerationInput: generationInput(running),
			Status:             "cancelled",
			Reason:             "runtime unavailable",
		}
		workItem, terminal, err := authority.CommitTerminal(
			context.Background(), terminalInput,
		)
		if err != nil {
			t.Fatalf("offline CommitTerminal() error = %v", err)
		}
		if workItem.Status() != "cancelled" ||
			terminal.TerminalStatus() != "cancelled" {
			t.Fatalf("offline terminal = work=%#v run=%#v", workItem, terminal)
		}

		runEvents, _ := store.ReadStream(context.Background(), "run/run-1")
		capacityEvents, _ := store.ReadStream(
			context.Background(), "runtime_capacity:runtime-1",
		)
		assertRuntimeStatusReference(
			t, runEvents[len(runEvents)-1].PayloadJSON,
			"runtime-1", 2, statusEvent.ID,
		)
		assertRuntimeStatusReference(
			t, capacityEvents[len(capacityEvents)-1].PayloadJSON,
			"runtime-1", 2, statusEvent.ID,
		)
	})

	t.Run("late generation and invalid terminal inputs are zero-write", func(t *testing.T) {
		store := openAuthorityStore(t)
		seedRuntime(t, store, "runtime-1", "online", 1)
		clock := &mutableClock{now: testNow}
		authority := newAuthority(t, store, clock, 8)
		if _, _, err := authority.CreateAndAssign(context.Background(), assignment("work-1", "run-1")); err != nil {
			t.Fatal(err)
		}
		_, claimed, err := authority.Claim(context.Background(), claim("work-1", "run-1", "runtime-1"))
		if err != nil {
			t.Fatal(err)
		}
		stale := generationInput(claimed)
		stale.ClaimID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
		if workItem, run, err := authority.Start(context.Background(), stale); !errors.Is(err, ErrStaleClaimGeneration) {
			t.Fatalf("stale start error = %v", err)
		} else {
			assertZeroWorkItem(t, workItem)
			assertZeroRun(t, run)
		}
		for _, terminal := range []RunTerminalInput{
			{RunGenerationInput: generationInput(claimed), Status: "done"},
			{RunGenerationInput: generationInput(claimed), Status: "succeeded", Reason: "not empty"},
			{RunGenerationInput: generationInput(claimed), Status: "failed"},
			{RunGenerationInput: generationInput(claimed), Status: "cancelled"},
		} {
			if workItem, run, err := authority.CommitTerminal(context.Background(), terminal); !errors.Is(err, ErrInvalidRunAuthorityInput) {
				t.Errorf("terminal %#v error = %v", terminal, err)
			} else {
				assertZeroWorkItem(t, workItem)
				assertZeroRun(t, run)
			}
		}
	})
}

func TestRunAuthorityInputSnapshotMutationAndStaticBoundary(t *testing.T) { // s3_w2_concurrency_mutation_fuzz_static
	t.Parallel()

	t.Run("constructor and input failures precede writes", func(t *testing.T) {
		store := openAuthorityStore(t)
		if authority, err := NewAuthority(nil, time.Now, bytes.NewReader(make([]byte, 16))); !errors.Is(err, ErrInvalidRunAuthorityInput) || authority != nil {
			t.Fatalf("nil store: authority=%v error=%v", authority, err)
		}
		if authority, err := NewAuthority(store, nil, bytes.NewReader(make([]byte, 16))); !errors.Is(err, ErrInvalidRunAuthorityInput) || authority != nil {
			t.Fatalf("nil clock: authority=%v error=%v", authority, err)
		}
		if authority, err := NewAuthority(store, time.Now, nil); !errors.Is(err, ErrInvalidRunAuthorityInput) || authority != nil {
			t.Fatalf("nil random: authority=%v error=%v", authority, err)
		}

		clock := &mutableClock{now: testNow}
		authority, err := NewAuthority(store, clock.Now, errorReader{err: io.ErrUnexpectedEOF})
		if err != nil {
			t.Fatal(err)
		}
		if err := authority.InitializeRunIdentityIndex(context.Background()); err != nil {
			t.Fatal(err)
		}
		if workItem, run, err := authority.CreateAndAssign(context.Background(), assignment("work-1", "run-1")); err != nil {
			t.Fatalf("assignment unexpectedly needs randomness: %v", err)
		} else {
			assertZeroRunClaim(t, run)
			_ = workItem
		}
		seedRuntime(t, store, "runtime-1", "online", 1)
		if workItem, run, err := authority.Claim(context.Background(), claim("work-1", "run-1", "runtime-1")); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("random error = %v, want io.ErrUnexpectedEOF", err)
		} else {
			assertZeroWorkItem(t, workItem)
			assertZeroRun(t, run)
		}
	})

	t.Run("snapshot is sorted isolated and concurrent-readable", func(t *testing.T) {
		store := openAuthorityStore(t)
		clock := &mutableClock{now: testNow}
		authority := newAuthority(t, store, clock, 9)
		for _, pair := range [][2]string{{"work-b", "run-b"}, {"work-a", "run-a"}} {
			if _, _, err := authority.CreateAndAssign(context.Background(), assignment(pair[0], pair[1])); err != nil {
				t.Fatal(err)
			}
		}
		snapshot, err := authority.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		workItems := snapshot.WorkItems()
		runs := snapshot.Runs()
		if len(workItems) != 2 || workItems[0].ID() != "work-a" || workItems[1].ID() != "work-b" ||
			len(runs) != 2 || runs[0].ID() != "run-a" || runs[1].ID() != "run-b" {
			t.Fatalf("snapshot order: work=%#v runs=%#v", workItems, runs)
		}
		workItems[0] = WorkItemRecord{}
		runs[0] = RunRecord{}
		if snapshot.WorkItems()[0].ID() != "work-a" || snapshot.Runs()[0].ID() != "run-a" {
			t.Fatal("snapshot accessors alias internal slices")
		}
		var waitGroup sync.WaitGroup
		for index := 0; index < 32; index++ {
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				for iteration := 0; iteration < 100; iteration++ {
					_, _ = authority.Snapshot(context.Background())
					_ = snapshot.WorkItems()
					_ = snapshot.Runs()
				}
			}()
		}
		waitGroup.Wait()
	})

	t.Run("unrelated facts are ignored and relevant corruption fails closed", func(t *testing.T) {
		store := openAuthorityStore(t)
		unrelated := journal.Event{
			ID:             "mode-event",
			StreamID:       "mode/session-1",
			Seq:            1,
			IdempotencyKey: "mode-event",
			Type:           "ModeSelected",
			SchemaVersion:  1,
			EmittedAt:      testNow,
			PayloadJSON:    []byte(`{"mode":"agent"}`),
		}
		if _, err := store.Append(context.Background(), unrelated); err != nil {
			t.Fatal(err)
		}
		clock := &mutableClock{now: testNow}
		authority := newAuthority(t, store, clock, 19)
		if _, _, err := authority.CreateAndAssign(
			context.Background(),
			assignment("work-1", "run-1"),
		); err != nil {
			t.Fatalf("unrelated fact blocked assignment: %v", err)
		}
		workEvents, _ := store.ReadStream(
			context.Background(), "work-item/work-1",
		)
		corrupt := journal.Event{
			ID:             "corrupt-run-event",
			StreamID:       "run/run-1",
			Seq:            1,
			IdempotencyKey: "corrupt-run-event",
			Type:           "RunAuthorityEscalated",
			SchemaVersion:  1,
			EmittedAt:      testNow,
			CorrelationID:  testCorrelation,
			CausationID:    workEvents[len(workEvents)-1].ID,
			PayloadJSON:    []byte(`{}`),
		}
		if _, err := store.Append(context.Background(), corrupt); err != nil {
			t.Fatal(err)
		}
		if snapshot, err := authority.Snapshot(context.Background()); !errors.Is(
			err, ErrRunAuthorityConflict,
		) || len(snapshot.WorkItems()) != 0 || len(snapshot.Runs()) != 0 {
			t.Fatalf("corrupt snapshot = %#v error=%v", snapshot, err)
		}
	})

	t.Run("production source has no forbidden authority", func(t *testing.T) {
		file, err := parser.ParseFile(token.NewFileSet(), "run_authority.go", nil, parser.AllErrors)
		if err != nil {
			t.Fatalf("parse production file: %v", err)
		}
		allowedImports := map[string]bool{
			"bytes": true, "context": true, "crypto/rand": true,
			"crypto/sha256": true, "encoding/hex": true, "encoding/json": true,
			"errors": true, "fmt": true, "io": true, "sort": true,
			"reflect": true, "strings": true, "sync": true, "time": true, "unicode": true,
			"unicode/utf8": true, "loom-pi-rebuild/internal/journal": true,
		}
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if !allowedImports[path] {
				t.Errorf("forbidden import %q", path)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if _, ok := node.(*ast.GoStmt); ok {
				t.Error("goroutine authority is forbidden")
			}
			return true
		})
	})
}

func TestApprovalPauseAndResolutionFenceClaimAndReplayStrictly(t *testing.T) {
	t.Run("pending approval fences claim and approval restores assignment", func(t *testing.T) {
		store := openAuthorityStore(t)
		clock := &mutableClock{now: testNow}
		authority := newAuthority(t, store, clock, 17)
		if _, _, err := authority.CreateAndAssign(
			context.Background(),
			assignment("work-approval", "run-approval"),
		); err != nil {
			t.Fatal(err)
		}
		workEvents, err := store.ReadStream(
			context.Background(),
			workItemStream("work-approval"),
		)
		if err != nil {
			t.Fatal(err)
		}
		approvalID := "approval-request-1"
		approvalDigest := testDigest
		pause := journal.Event{
			ID:             "approval-pause-event",
			StreamID:       workItemStream("work-approval"),
			Seq:            3,
			IdempotencyKey: "approval-pause-event",
			Type:           "WorkItemApprovalPaused",
			SchemaVersion:  1,
			EmittedAt:      testNow,
			CorrelationID:  testCorrelation,
			CausationID:    "approval-request-event",
			PayloadJSON: mustJSON(t, map[string]any{
				"work_item_id":            "work-approval",
				"approval_request_id":     approvalID,
				"approval_request_digest": approvalDigest,
				"previous_status":         "assigned",
				"status":                  "waiting_approval",
			}),
		}
		if _, err := store.Append(context.Background(), pause); err != nil {
			t.Fatal(err)
		}
		seedRuntime(t, store, "runtime-approval", "online", 1)
		if workItem, run, err := authority.Claim(
			context.Background(),
			claim("work-approval", "run-approval", "runtime-approval"),
		); !errors.Is(err, ErrRunNotClaimable) {
			t.Fatalf("paused Claim() = work=%#v run=%#v error=%v", workItem, run, err)
		}

		resolved := journal.Event{
			ID:             "approval-resolved-event",
			StreamID:       workItemStream("work-approval"),
			Seq:            4,
			IdempotencyKey: "approval-resolved-event",
			Type:           "WorkItemApprovalResolved",
			SchemaVersion:  1,
			EmittedAt:      testNow.Add(time.Second),
			CorrelationID:  testCorrelation,
			CausationID:    "approval-decision-event",
			PayloadJSON: mustJSON(t, map[string]any{
				"work_item_id":            "work-approval",
				"approval_request_id":     approvalID,
				"approval_request_digest": approvalDigest,
				"previous_status":         "waiting_approval",
				"status":                  "assigned",
			}),
		}
		if _, err := store.Append(context.Background(), resolved); err != nil {
			t.Fatal(err)
		}
		if _, _, err := authority.Claim(
			context.Background(),
			claim("work-approval", "run-approval", "runtime-approval"),
		); err != nil {
			t.Fatalf("approved Claim() error = %v", err)
		}
		if len(workEvents) != 2 {
			t.Fatalf("assignment Event count = %d, want 2", len(workEvents))
		}
	})

	t.Run("malformed approval event fails closed", func(t *testing.T) {
		store := openAuthorityStore(t)
		clock := &mutableClock{now: testNow}
		authority := newAuthority(t, store, clock, 18)
		if _, _, err := authority.CreateAndAssign(
			context.Background(),
			assignment("work-corrupt-approval", "run-corrupt-approval"),
		); err != nil {
			t.Fatal(err)
		}
		corrupt := journal.Event{
			ID:             "approval-corrupt-event",
			StreamID:       workItemStream("work-corrupt-approval"),
			Seq:            3,
			IdempotencyKey: "approval-corrupt-event",
			Type:           "WorkItemApprovalPaused",
			SchemaVersion:  1,
			EmittedAt:      testNow,
			CorrelationID:  testCorrelation,
			CausationID:    "approval-request-event",
			PayloadJSON:    []byte(`{"work_item_id":"work-corrupt-approval"}`),
		}
		if _, err := store.Append(context.Background(), corrupt); err != nil {
			t.Fatal(err)
		}
		if snapshot, err := authority.Snapshot(context.Background()); !errors.Is(
			err,
			ErrRunAuthorityConflict,
		) || len(snapshot.WorkItems()) != 0 || len(snapshot.Runs()) != 0 {
			t.Fatalf("corrupt approval snapshot = %#v error=%v", snapshot, err)
		}
	})
}

func assertZeroRunClaim(t testing.TB, run RunRecord) {
	t.Helper()
	if run.ID() == "" || run.Phase() != "unclaimed" || run.ClaimID() != "" ||
		run.ClaimGeneration() != 0 {
		t.Fatalf("assignment run = %#v", run)
	}
}

func assertExactJSON(t testing.TB, payload []byte, want map[string]any) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("payload keys = %d, want %d: %s", len(got), len(want), payload)
	}
	for key, wantValue := range want {
		if fmt.Sprint(got[key]) != fmt.Sprint(wantValue) {
			t.Fatalf("payload[%q] = %#v, want %#v", key, got[key], wantValue)
		}
	}
}

func mustJSON(t testing.TB, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func FuzzRunAuthorityReplayNeverPanics(f *testing.F) {
	seeds := [][]byte{
		nil,
		[]byte(`[]`),
		[]byte(`[{"Type":"RunClaimed"}]`),
		[]byte(`{"truncated":`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var events []journal.Event
		_ = json.Unmarshal(data, &events)
		_, _ = replayAuthorityEvents(context.Background(), events)
	})
}
