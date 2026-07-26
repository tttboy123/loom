package authorization

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

var (
	grantTestNow         = time.Date(2026, 7, 26, 1, 2, 3, 456000000, time.UTC)
	grantTestCorrelation = "11111111-1111-4111-8111-111111111111"
	grantTestRequest     = "22222222-2222-4222-8222-222222222222"
	grantTestDigest      = strings.Repeat("a", 64)
)

type grantTestClock struct {
	mu  sync.RWMutex
	now time.Time
}

func (clock *grantTestClock) Now() time.Time {
	clock.mu.RLock()
	defer clock.mu.RUnlock()
	return clock.now
}

func (clock *grantTestClock) Set(now time.Time) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = now
}

type countingGrantReader struct {
	mu    sync.Mutex
	bytes []byte
	read  int
}

func newCountingGrantReader(seed byte) *countingGrantReader {
	material := make([]byte, 48)
	for index := range material {
		material[index] = seed + byte(index)
	}
	return &countingGrantReader{bytes: material}
}

func (reader *countingGrantReader) Read(output []byte) (int, error) {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if reader.read >= len(reader.bytes) {
		return 0, errors.New("grant random exhausted")
	}
	count := copy(output, reader.bytes[reader.read:])
	reader.read += count
	return count, nil
}

func (reader *countingGrantReader) Count() int {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	return reader.read
}

type grantFixture struct {
	store         *journal.Store
	db            *sql.DB
	clock         *grantTestClock
	workAuthority *work.Authority
	workItem      work.WorkItemRecord
	run           work.RunRecord
}

func newGrantFixture(t testing.TB, suffix string) *grantFixture {
	t.Helper()
	return newGrantFixtureWithIDs(
		t,
		suffix,
		"work-1",
		"grant boundary",
		"run-1",
		"agent-1",
	)
}

func newGrantFixtureWithIDs(
	t testing.TB,
	suffix string,
	workItemID string,
	title string,
	runID string,
	agentInstanceID string,
) *grantFixture {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s/%s.db?%s",
		t.TempDir(),
		suffix,
		values.Encode(),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	store := journal.NewStore(db)
	clock := &grantTestClock{now: grantTestNow}
	workAuthority, err := work.NewAuthority(
		store,
		clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x31}, 64)),
	)
	if err != nil {
		t.Fatalf("work.NewAuthority() error = %v", err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(context.Background()); err != nil {
		t.Fatalf("InitializeRunIdentityIndex() error = %v", err)
	}
	seedGrantRuntime(t, store, "runtime-1", "online", 2)
	workItem, run, err := workAuthority.CreateAndAssign(
		context.Background(),
		work.WorkItemAssignmentInput{
			WorkItemID:      workItemID,
			Title:           title,
			RunID:           runID,
			AgentInstanceID: agentInstanceID,
			CorrelationID:   grantTestCorrelation,
		},
	)
	if err != nil {
		t.Fatalf("CreateAndAssign() error = %v", err)
	}
	workItem, run, err = workAuthority.Claim(
		context.Background(),
		work.RunClaimInput{
			WorkItemID:           workItem.ID(),
			RunID:                run.ID(),
			RuntimeInstanceID:    "runtime-1",
			AgentInstanceID:      agentInstanceID,
			PrepareLeaseDuration: 5 * time.Minute,
			CorrelationID:        grantTestCorrelation,
		},
	)
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	return &grantFixture{
		store:         store,
		db:            db,
		clock:         clock,
		workAuthority: workAuthority,
		workItem:      workItem,
		run:           run,
	}
}

func seedGrantRuntime(
	t testing.TB,
	store *journal.Store,
	runtimeID string,
	status string,
	capacity int,
) {
	t.Helper()
	payload := map[string]any{
		"discovery_digest": grantTestDigest,
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
	_, err = store.Append(context.Background(), journal.Event{
		ID:             "runtime-discovered-" + runtimeID,
		StreamID:       "runtime_instance:" + runtimeID,
		Seq:            1,
		IdempotencyKey: "runtime-discovered-" + runtimeID,
		Type:           "RuntimeInstanceDiscovered",
		SchemaVersion:  1,
		EmittedAt:      grantTestNow.Add(-time.Minute),
		CorrelationID:  grantTestRequest,
		PayloadJSON:    body,
	})
	if err != nil {
		t.Fatalf("seed runtime: %v", err)
	}
}

func newGrantTestAuthority(
	t testing.TB,
	fixture *grantFixture,
	reader *countingGrantReader,
) *Authority {
	t.Helper()
	authority, err := NewAuthority(
		fixture.store,
		fixture.workAuthority,
		fixture.clock.Now,
		reader,
	)
	if err != nil {
		t.Fatalf("NewAuthority() error = %v", err)
	}
	if err := authority.InitializeGrantIdentityIndex(context.Background()); err != nil {
		t.Fatalf("InitializeGrantIdentityIndex() error = %v", err)
	}
	return authority
}

func TestGrantIdentityIndexIsExplicit(t *testing.T) {
	fixture := newGrantFixture(t, "identity-explicit")
	authority, err := NewAuthority(
		fixture.store,
		fixture.workAuthority,
		fixture.clock.Now,
		newCountingGrantReader(0x41),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.Issue(
		context.Background(),
		grantIssueInput(fixture, time.Minute),
	); !errors.Is(err, ErrGrantIdentityIndexRequired) {
		t.Fatalf("uninitialized Issue() error = %v", err)
	}
	if err := authority.InitializeGrantIdentityIndex(context.Background()); err != nil {
		t.Fatalf("InitializeGrantIdentityIndex() error = %v", err)
	}
	if _, err := authority.Issue(
		context.Background(),
		grantIssueInput(fixture, time.Minute),
	); err != nil {
		t.Fatalf("initialized Issue() error = %v", err)
	}
}

func grantIssueInput(fixture *grantFixture, lifetime time.Duration) IssueInput {
	return IssueInput{
		WorkItemID:        fixture.workItem.ID(),
		RunID:             fixture.run.ID(),
		ClaimID:           fixture.run.ClaimID(),
		ClaimGeneration:   fixture.run.ClaimGeneration(),
		RuntimeInstanceID: fixture.run.RuntimeInstanceID(),
		AgentInstanceID:   fixture.run.AgentInstanceID(),
		AllowedOperations: []Operation{
			OperationBridgeAck,
			OperationBridgeEvent,
			OperationBridgeResult,
			OperationContextRead,
			OperationEvidenceStage,
		},
		Lifetime:      lifetime,
		CorrelationID: grantTestCorrelation,
	}
}

func grantAuthorizeInput(
	fixture *grantFixture,
	token Token,
	operation Operation,
	requestID string,
) AuthorizeInput {
	return AuthorizeInput{
		Token:             token,
		WorkItemID:        fixture.workItem.ID(),
		RunID:             fixture.run.ID(),
		ClaimID:           fixture.run.ClaimID(),
		ClaimGeneration:   fixture.run.ClaimGeneration(),
		RuntimeInstanceID: fixture.run.RuntimeInstanceID(),
		AgentInstanceID:   fixture.run.AgentInstanceID(),
		Operation:         operation,
		RequestID:         requestID,
		CorrelationID:     grantTestCorrelation,
	}
}

func TestIssueHashOnlyRunCAS(t *testing.T) { // s3_w3_issue_hash_only_run_cas
	fixture := newGrantFixture(t, "issue")
	random := newCountingGrantReader(0x10)
	authority := newGrantTestAuthority(t, fixture, random)

	issued, err := authority.Issue(
		context.Background(),
		grantIssueInput(fixture, 30*time.Minute),
	)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if random.Count() != 48 {
		t.Fatalf("random bytes = %d, want 48", random.Count())
	}
	token := issued.Token()
	if token.String() != "[REDACTED]" || token.GoString() != "[REDACTED]" {
		t.Fatalf("token formatting leaked: %s / %#v", token.String(), token)
	}
	parsed, err := ParseToken(token.Value())
	if err != nil || parsed.Value() != token.Value() {
		t.Fatalf("ParseToken() = %q, %v", parsed.Value(), err)
	}
	if _, err := ParseToken(token.Value() + "="); !errors.Is(err, ErrInvalidGrantAuthorityInput) {
		t.Fatalf("noncanonical ParseToken() error = %v", err)
	}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		formattedIssued := fmt.Sprintf(format, issued)
		if strings.Contains(formattedIssued, token.Value()) {
			t.Fatalf("nested token formatting leaked for %s: %s", format, formattedIssued)
		}
	}
	serializedToken, err := json.Marshal(token)
	if err != nil || bytes.Contains(serializedToken, []byte(token.Value())) {
		t.Fatalf("token JSON leaked: %s, %v", serializedToken, err)
	}

	record := issued.Record()
	if record.ID() == "" || record.WorkItemID() != fixture.workItem.ID() ||
		record.RunID() != fixture.run.ID() ||
		record.ClaimID() != fixture.run.ClaimID() ||
		record.ClaimGeneration() != fixture.run.ClaimGeneration() ||
		record.RuntimeInstanceID() != fixture.run.RuntimeInstanceID() ||
		record.AgentInstanceID() != fixture.run.AgentInstanceID() ||
		!record.IssuedAt().Equal(grantTestNow) ||
		!record.ExpiresAt().Equal(grantTestNow.Add(30*time.Minute)) ||
		!record.RevokedAt().IsZero() {
		t.Fatalf("issued record = %#v", record)
	}
	wantOperations := []Operation{
		OperationBridgeAck,
		OperationBridgeEvent,
		OperationBridgeResult,
		OperationContextRead,
		OperationEvidenceStage,
	}
	if !reflect.DeepEqual(record.AllowedOperations(), wantOperations) {
		t.Fatalf("operations = %#v", record.AllowedOperations())
	}

	events, err := fixture.store.ReadStream(context.Background(), "agent-grant/run-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "AgentGrantIssued" ||
		events[0].Seq != 1 || events[0].SchemaVersion != 1 {
		t.Fatalf("issue events = %#v", events)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[0].PayloadJSON, &payload); err != nil {
		t.Fatal(err)
	}
	assertGrantJSONKeys(t, payload,
		"grant_id", "work_item_id", "run_id", "claim_id",
		"claim_generation", "runtime_instance_id", "agent_instance_id",
		"allowed_operations", "token_hash", "issued_at", "expires_at",
		"run_stream", "run_sequence", "run_event_id",
	)
	digest := sha256.Sum256([]byte(token.Value()))
	if payload["token_hash"] != hex.EncodeToString(digest[:]) {
		t.Fatalf("token hash = %#v", payload["token_hash"])
	}
	if bytes.Contains(events[0].PayloadJSON, []byte(token.Value())) {
		t.Fatal("raw token entered Journal payload")
	}
	var persistedToken string
	err = fixture.db.QueryRowContext(
		context.Background(),
		`SELECT COALESCE(GROUP_CONCAT(payload_json, ''), '') FROM events`,
	).Scan(&persistedToken)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(persistedToken, token.Value()) {
		t.Fatal("raw token entered SQLite")
	}

	second, err := authority.Issue(
		context.Background(),
		grantIssueInput(fixture, 30*time.Minute),
	)
	assertZeroIssuedGrant(t, second)
	if !errors.Is(err, ErrGrantAlreadyActive) {
		t.Fatalf("second Issue() error = %v", err)
	}
	if random.Count() != 48 {
		t.Fatalf("failed Issue consumed randomness: %d", random.Count())
	}
}

func TestAuthorizeBindingOperationExpiry(t *testing.T) { // s3_w3_authorize_binding_operation_expiry
	fixture := newGrantFixture(t, "authorize")
	authority := newGrantTestAuthority(t, fixture, newCountingGrantReader(0x20))
	issued, err := authority.Issue(
		context.Background(),
		grantIssueInput(fixture, time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}

	input := grantAuthorizeInput(
		fixture,
		issued.Token(),
		OperationBridgeEvent,
		grantTestRequest,
	)
	authorized, err := authority.Authorize(context.Background(), input)
	if err != nil || authorized.ID() != issued.Record().ID() {
		t.Fatalf("Authorize() = %#v, %v", authorized, err)
	}
	retry, err := authority.Authorize(context.Background(), input)
	if err != nil || retry.ID() != authorized.ID() {
		t.Fatalf("Authorize() retry = %#v, %v", retry, err)
	}
	events, err := fixture.store.ReadStream(context.Background(), "agent-grant/run-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].Type != "AgentGrantAuthorized" {
		t.Fatalf("authorized events = %#v", events)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[1].PayloadJSON, &payload); err != nil {
		t.Fatal(err)
	}
	assertGrantJSONKeys(t, payload,
		"grant_id", "work_item_id", "run_id", "claim_id",
		"claim_generation", "runtime_instance_id", "agent_instance_id",
		"operation", "request_id", "authorized_at",
		"run_stream", "run_sequence", "run_event_id",
	)
	if bytes.Contains(events[1].PayloadJSON, []byte(issued.Token().Value())) ||
		payload["token_hash"] != nil {
		t.Fatal("authorization fact disclosed token material")
	}

	tests := []struct {
		name  string
		edit  func(*AuthorizeInput)
		match error
	}{
		{"wrong operation", func(value *AuthorizeInput) {
			value.Operation = OperationBridgeEvidence
		}, ErrGrantOperationDenied},
		{"wrong work item", func(value *AuthorizeInput) {
			value.WorkItemID = "work-other"
		}, ErrGrantBindingMismatch},
		{"wrong agent", func(value *AuthorizeInput) {
			value.AgentInstanceID = "agent-other"
		}, ErrGrantBindingMismatch},
		{"stale generation", func(value *AuthorizeInput) {
			value.ClaimGeneration++
		}, ErrGrantBindingMismatch},
		{"request reuse other operation", func(value *AuthorizeInput) {
			value.Operation = OperationContextRead
		}, ErrGrantAuthorityConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := input
			test.edit(&candidate)
			got, gotErr := authority.Authorize(context.Background(), candidate)
			assertZeroGrantRecord(t, got)
			if !errors.Is(gotErr, test.match) {
				t.Fatalf("Authorize() error = %v, want %v", gotErr, test.match)
			}
		})
	}
	afterFailures, err := fixture.store.ReadStream(context.Background(), "agent-grant/run-1")
	if err != nil || len(afterFailures) != 2 {
		t.Fatalf("failure event count = %d, %v", len(afterFailures), err)
	}

	fixture.clock.Set(grantTestNow.Add(time.Minute))
	expired, err := authority.Authorize(
		context.Background(),
		grantAuthorizeInput(
			fixture,
			issued.Token(),
			OperationBridgeEvent,
			"33333333-3333-4333-8333-333333333333",
		),
	)
	assertZeroGrantRecord(t, expired)
	if !errors.Is(err, ErrGrantExpired) {
		t.Fatalf("expired Authorize() error = %v", err)
	}
}

func TestRotateRevokeGeneration(t *testing.T) { // s3_w3_rotate_revoke_generation
	fixture := newGrantFixture(t, "rotate")
	firstAuthority := newGrantTestAuthority(t, fixture, newCountingGrantReader(0x30))
	first, err := firstAuthority.Issue(
		context.Background(),
		grantIssueInput(fixture, 10*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}

	fixture.clock.Set(grantTestNow.Add(11 * time.Second))
	secondAuthority := newGrantTestAuthority(t, fixture, newCountingGrantReader(0x40))
	second, err := secondAuthority.Issue(
		context.Background(),
		grantIssueInput(fixture, time.Minute),
	)
	if err != nil {
		t.Fatalf("same-generation rotation error = %v", err)
	}
	snapshot, err := secondAuthority.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	records := snapshot.Grants()
	if len(records) != 2 ||
		records[0].RevocationReason() != RevocationExpired ||
		records[0].RevokedAt().IsZero() ||
		records[1].ID() != second.Record().ID() {
		t.Fatalf("same-generation rotation = %#v", records)
	}
	oldAuthorized, err := secondAuthority.Authorize(
		context.Background(),
		grantAuthorizeInput(
			fixture,
			first.Token(),
			OperationBridgeEvent,
			"44444444-4444-4444-8444-444444444444",
		),
	)
	assertZeroGrantRecord(t, oldAuthorized)
	if !errors.Is(err, ErrGrantRevoked) {
		t.Fatalf("old token error = %v", err)
	}

	revoked, err := secondAuthority.Revoke(
		context.Background(),
		RevokeInput{
			GrantID:       second.Record().ID(),
			Reason:        RevocationOperator,
			CorrelationID: grantTestCorrelation,
		},
	)
	if err != nil || revoked.RevocationReason() != RevocationOperator {
		t.Fatalf("Revoke() = %#v, %v", revoked, err)
	}
	retry, err := secondAuthority.Revoke(
		context.Background(),
		RevokeInput{
			GrantID:       second.Record().ID(),
			Reason:        RevocationOperator,
			CorrelationID: grantTestCorrelation,
		},
	)
	if err != nil || retry.ID() != revoked.ID() {
		t.Fatalf("Revoke() retry = %#v, %v", retry, err)
	}
	conflict, err := secondAuthority.Revoke(
		context.Background(),
		RevokeInput{
			GrantID:       second.Record().ID(),
			Reason:        RevocationTerminal,
			CorrelationID: grantTestCorrelation,
		},
	)
	assertZeroGrantRecord(t, conflict)
	if !errors.Is(err, ErrGrantAlreadyRevoked) {
		t.Fatalf("second reason error = %v", err)
	}

	fixture.clock.Set(grantTestNow.Add(5*time.Minute + time.Nanosecond))
	_, reclaimed, err := fixture.workAuthority.Claim(
		context.Background(),
		work.RunClaimInput{
			WorkItemID:           fixture.workItem.ID(),
			RunID:                fixture.run.ID(),
			RuntimeInstanceID:    fixture.run.RuntimeInstanceID(),
			AgentInstanceID:      fixture.run.AgentInstanceID(),
			PrepareLeaseDuration: 5 * time.Minute,
			CorrelationID:        grantTestCorrelation,
		},
	)
	if err != nil {
		t.Fatalf("reclaim error = %v", err)
	}
	fixture.run = reclaimed
	thirdAuthority := newGrantTestAuthority(t, fixture, newCountingGrantReader(0x50))
	third, err := thirdAuthority.Issue(
		context.Background(),
		grantIssueInput(fixture, time.Minute),
	)
	if err != nil || third.Record().ClaimGeneration() != 2 {
		t.Fatalf("generation rotation = %#v, %v", third, err)
	}
	grantEvents, err := fixture.store.ReadStream(
		context.Background(),
		"agent-grant/"+fixture.run.ID(),
	)
	if err != nil {
		t.Fatal(err)
	}
	last := grantEvents[len(grantEvents)-1]
	previous := grantEvents[len(grantEvents)-2]
	if last.Type != "AgentGrantIssued" ||
		last.CausationID != previous.ID {
		t.Fatalf(
			"post-revocation issue causation = %s, want %s",
			last.CausationID,
			previous.ID,
		)
	}

	t.Run("unrevoked prior generation is replaced atomically", func(t *testing.T) {
		replacementFixture := newGrantFixture(t, "generation-replacement")
		firstAuthority := newGrantTestAuthority(
			t,
			replacementFixture,
			newCountingGrantReader(0x51),
		)
		first, issueErr := firstAuthority.Issue(
			context.Background(),
			grantIssueInput(replacementFixture, time.Hour),
		)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		replacementFixture.clock.Set(grantTestNow.Add(5*time.Minute + time.Nanosecond))
		_, nextRun, claimErr := replacementFixture.workAuthority.Claim(
			context.Background(),
			work.RunClaimInput{
				WorkItemID:           replacementFixture.workItem.ID(),
				RunID:                replacementFixture.run.ID(),
				RuntimeInstanceID:    replacementFixture.run.RuntimeInstanceID(),
				AgentInstanceID:      replacementFixture.run.AgentInstanceID(),
				PrepareLeaseDuration: 5 * time.Minute,
				CorrelationID:        grantTestCorrelation,
			},
		)
		if claimErr != nil {
			t.Fatal(claimErr)
		}
		replacementFixture.run = nextRun
		nextAuthority := newGrantTestAuthority(
			t,
			replacementFixture,
			newCountingGrantReader(0x52),
		)
		next, issueErr := nextAuthority.Issue(
			context.Background(),
			grantIssueInput(replacementFixture, time.Minute),
		)
		if issueErr != nil || next.Record().ClaimGeneration() != 2 {
			t.Fatalf("replacement Issue() = %#v, %v", next, issueErr)
		}
		replaced := nextAuthority.mustSnapshotForTest(t).Grants()
		if len(replaced) != 2 ||
			replaced[0].RevocationReason() != RevocationReplaced {
			t.Fatalf("replacement records = %#v", replaced)
		}
		late, lateErr := nextAuthority.Authorize(
			context.Background(),
			grantAuthorizeInput(
				replacementFixture,
				first.Token(),
				OperationBridgeEvent,
				"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			),
		)
		assertZeroGrantRecord(t, late)
		if !errors.Is(lateErr, ErrGrantRevoked) {
			t.Fatalf("replaced token error = %v", lateErr)
		}
	})
}

func TestRepairOpaqueIDCompatibility(t *testing.T) { // s3_w3_repair_opaque_id_compatibility
	fixture := newGrantFixtureWithIDs(
		t,
		"opaque-ids",
		"work item 世界",
		"grant title",
		"run @ 世界",
		"agent @ 世界",
	)
	authority := newGrantTestAuthority(t, fixture, newCountingGrantReader(0x5a))
	issued, err := authority.Issue(
		context.Background(),
		grantIssueInput(fixture, time.Minute),
	)
	if err != nil {
		t.Fatalf("Issue() rejected accepted S3-W2 opaque IDs: %v", err)
	}
	authorized, err := authority.Authorize(
		context.Background(),
		grantAuthorizeInput(
			fixture,
			issued.Token(),
			OperationBridgeEvent,
			grantTestRequest,
		),
	)
	if err != nil || authorized.ID() != issued.Record().ID() {
		t.Fatalf("Authorize() = %#v, %v", authorized, err)
	}
}

func TestRepairHistoricalRunReference(t *testing.T) { // s3_w3_repair_historical_run_reference
	tests := []struct {
		name  string
		field string
		value any
	}{
		{"event id", "run_event_id", "missing-run-event"},
		{
			"referenced stream",
			"referenced_stream",
			"runtime-discovered-runtime-1",
		},
		{"sequence", "run_sequence", float64(99)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newGrantFixture(
				t,
				"historical-"+strings.ReplaceAll(test.name, " ", "-"),
			)
			authority := newGrantTestAuthority(
				t,
				fixture,
				newCountingGrantReader(0x5b),
			)
			runEvents, err := fixture.store.ReadStream(
				context.Background(),
				"run/run-1",
			)
			if err != nil || len(runEvents) == 0 {
				t.Fatalf("Run events = %#v, %v", runEvents, err)
			}
			runHead := runEvents[len(runEvents)-1]
			payload := issuePayload{
				GrantID:           "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
				WorkItemID:        fixture.workItem.ID(),
				RunID:             fixture.run.ID(),
				ClaimID:           fixture.run.ClaimID(),
				ClaimGeneration:   fixture.run.ClaimGeneration(),
				RuntimeInstanceID: fixture.run.RuntimeInstanceID(),
				AgentInstanceID:   fixture.run.AgentInstanceID(),
				AllowedOperations: []Operation{OperationBridgeEvent},
				TokenHash:         strings.Repeat("a", 64),
				IssuedAt:          grantTestNow.Format(time.RFC3339Nano),
				ExpiresAt: grantTestNow.Add(time.Minute).
					Format(time.RFC3339Nano),
				RunStream:   runHead.StreamID,
				RunSequence: runHead.Seq,
				RunEventID:  runHead.ID,
			}
			switch test.field {
			case "run_event_id":
				payload.RunEventID = test.value.(string)
			case "referenced_stream":
				payload.RunEventID = test.value.(string)
			case "run_sequence":
				payload.RunSequence = int64(test.value.(float64))
			}
			event := newGrantEvent(
				"fabricated-grant-"+strings.ReplaceAll(test.name, " ", "-"),
				"agent-grant/run-1",
				1,
				"AgentGrantIssued",
				grantTestNow,
				grantTestCorrelation,
				runHead.ID,
				payload,
			)
			if _, err := fixture.store.Append(
				context.Background(),
				event,
			); err != nil {
				t.Fatal(err)
			}
			snapshot, err := authority.Snapshot(context.Background())
			if err == nil || len(snapshot.Grants()) != 0 {
				t.Fatalf(
					"Snapshot() accepted fabricated %s: %#v, %v",
					test.field,
					snapshot,
					err,
				)
			}
		})
	}
}

func TestRepairConflictNormalization(t *testing.T) { // s3_w3_repair_conflict_normalization
	for _, collision := range []error{
		journal.ErrStreamHeadConflict,
		journal.ErrSequenceConflict,
		journal.ErrIdempotencyConflict,
		journal.ErrPartialEventBatchConflict,
	} {
		if err := mapGrantWriteError(collision); !errors.Is(
			err,
			ErrGrantAuthorityConflict,
		) {
			t.Fatalf("mapGrantWriteError(%v) = %v", collision, err)
		}
	}

	fixture := newGrantFixture(t, "request-collision")
	authority := newGrantTestAuthority(t, fixture, newCountingGrantReader(0x5c))
	issued, err := authority.Issue(
		context.Background(),
		grantIssueInput(fixture, time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	eventID := deterministicGrantEventID(
		"AgentGrantAuthorized",
		fixture.run.ID(),
		grantTestRequest,
	)
	if _, err := fixture.store.Append(context.Background(), journal.Event{
		ID:             eventID,
		StreamID:       "fixture/request-collision",
		Seq:            1,
		IdempotencyKey: eventID,
		Type:           "FixtureAuthorizationContender",
		SchemaVersion:  1,
		EmittedAt:      grantTestNow,
		CorrelationID:  grantTestCorrelation,
		PayloadJSON:    []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	record, err := authority.Authorize(
		context.Background(),
		grantAuthorizeInput(
			fixture,
			issued.Token(),
			OperationBridgeEvent,
			grantTestRequest,
		),
	)
	assertZeroGrantRecord(t, record)
	if !errors.Is(err, ErrGrantAuthorityConflict) {
		t.Fatalf("conflicting RequestID error = %v", err)
	}
}

func TestRepairDuplicateJSONKeys(t *testing.T) { // s3_w3_repair_duplicate_json_keys
	var root struct {
		RunID string `json:"run_id"`
	}
	if err := decodeExactGrantPayload(
		[]byte(`{"run_id":"run-1","run_id":"run-1"}`),
		&root,
	); err == nil {
		t.Fatal("identical duplicate root key accepted")
	}
	var nested struct {
		Binding struct {
			RunID string `json:"run_id"`
		} `json:"binding"`
	}
	if err := decodeExactGrantPayload(
		[]byte(`{"binding":{"run_id":"run-1","run_id":"run-1"}}`),
		&nested,
	); err == nil {
		t.Fatal("identical duplicate nested key accepted")
	}
}

func TestConcurrencyCollisionMutation(t *testing.T) { // s3_w3_concurrency_collision_mutation
	fixture := newGrantFixture(t, "concurrency")
	authorities := []*Authority{
		newGrantTestAuthority(t, fixture, newCountingGrantReader(0x60)),
		newGrantTestAuthority(t, fixture, newCountingGrantReader(0x70)),
	}
	input := grantIssueInput(fixture, time.Minute)
	type result struct {
		issued IssuedGrant
		err    error
	}
	start := make(chan struct{})
	results := make(chan result, len(authorities))
	for _, authority := range authorities {
		go func(authority *Authority) {
			<-start
			issued, err := authority.Issue(context.Background(), input)
			results <- result{issued, err}
		}(authority)
	}
	close(start)
	var winner IssuedGrant
	successes := 0
	for range authorities {
		got := <-results
		if got.err == nil {
			successes++
			winner = got.issued
			continue
		}
		if !errors.Is(got.err, ErrGrantAuthorityConflict) &&
			!errors.Is(got.err, ErrGrantAlreadyActive) {
			t.Fatalf("contender error = %v", got.err)
		}
		assertZeroIssuedGrant(t, got.issued)
	}
	if successes != 1 {
		t.Fatalf("successful issues = %d, want 1", successes)
	}
	events, err := fixture.store.ReadStream(context.Background(), "agent-grant/run-1")
	if err != nil || len(events) != 1 {
		t.Fatalf("issue event count = %d, %v", len(events), err)
	}

	operations := winner.Record().AllowedOperations()
	operations[0] = OperationBridgeHeartbeat
	if winner.Record().AllowedOperations()[0] != OperationBridgeAck {
		t.Fatal("record operation accessor aliases caller")
	}
	snapshot, err := authorities[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	grants := snapshot.Grants()
	grants[0] = GrantRecord{}
	if authorities[0].mustSnapshotForTest(t).Grants()[0].ID() == "" {
		t.Fatal("snapshot aliases caller")
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := authorities[0].Authorize(
		cancelled,
		grantAuthorizeInput(
			fixture,
			winner.Token(),
			OperationBridgeEvent,
			"55555555-5555-4555-8555-555555555555",
		),
	)
	assertZeroGrantRecord(t, got)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Authorize() error = %v", err)
	}
}

func (authority *Authority) mustSnapshotForTest(t testing.TB) AuthoritySnapshot {
	t.Helper()
	snapshot, err := authority.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestGrantAuthorityInvalidInputAndTerminal(t *testing.T) {
	fixture := newGrantFixture(t, "invalid")
	random := newCountingGrantReader(0x80)
	authority := newGrantTestAuthority(t, fixture, random)

	if _, err := NewAuthority(nil, fixture.workAuthority, fixture.clock.Now, random); !errors.Is(
		err,
		ErrInvalidGrantAuthorityInput,
	) {
		t.Fatalf("nil store error = %v", err)
	}
	if _, err := NewAuthority(fixture.store, nil, fixture.clock.Now, random); !errors.Is(
		err,
		ErrInvalidGrantAuthorityInput,
	) {
		t.Fatalf("nil work authority error = %v", err)
	}
	if _, err := NewAuthority(fixture.store, fixture.workAuthority, nil, random); !errors.Is(
		err,
		ErrInvalidGrantAuthorityInput,
	) {
		t.Fatalf("nil clock error = %v", err)
	}
	if _, err := NewAuthority(fixture.store, fixture.workAuthority, fixture.clock.Now, nil); !errors.Is(
		err,
		ErrInvalidGrantAuthorityInput,
	) {
		t.Fatalf("nil random error = %v", err)
	}

	invalidIssues := []IssueInput{
		{},
		func() IssueInput {
			value := grantIssueInput(fixture, time.Minute)
			value.AllowedOperations = nil
			return value
		}(),
		func() IssueInput {
			value := grantIssueInput(fixture, time.Minute)
			value.AllowedOperations = append(
				value.AllowedOperations,
				OperationBridgeAck,
			)
			return value
		}(),
		func() IssueInput {
			value := grantIssueInput(fixture, 0)
			return value
		}(),
		func() IssueInput {
			value := grantIssueInput(fixture, time.Hour+time.Nanosecond)
			return value
		}(),
	}
	for index, input := range invalidIssues {
		issued, err := authority.Issue(context.Background(), input)
		assertZeroIssuedGrant(t, issued)
		if !errors.Is(err, ErrInvalidGrantAuthorityInput) {
			t.Fatalf("invalid Issue %d error = %v", index, err)
		}
	}
	if random.Count() != 0 {
		t.Fatalf("invalid inputs consumed random = %d", random.Count())
	}

	issued, err := authority.Issue(
		context.Background(),
		grantIssueInput(fixture, time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	generation := work.RunGenerationInput{
		WorkItemID:        fixture.run.WorkItemID(),
		RunID:             fixture.run.ID(),
		ClaimID:           fixture.run.ClaimID(),
		ClaimGeneration:   fixture.run.ClaimGeneration(),
		RuntimeInstanceID: fixture.run.RuntimeInstanceID(),
		AgentInstanceID:   fixture.run.AgentInstanceID(),
		CorrelationID:     grantTestCorrelation,
	}
	_, running, err := fixture.workAuthority.Start(context.Background(), generation)
	if err != nil {
		t.Fatal(err)
	}
	fixture.run = running
	runningAuthorization, err := authority.Authorize(
		context.Background(),
		grantAuthorizeInput(
			fixture,
			issued.Token(),
			OperationBridgeEvent,
			"66666666-6666-4666-8666-666666666666",
		),
	)
	if err != nil || runningAuthorization.ID() != issued.Record().ID() {
		t.Fatalf("running Authorize() = %#v, %v", runningAuthorization, err)
	}
	_, terminal, err := fixture.workAuthority.CommitTerminal(
		context.Background(),
		work.RunTerminalInput{
			RunGenerationInput: generation,
			Status:             "failed",
			Reason:             "fixture-exit",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.run = terminal
	late, err := authority.Authorize(
		context.Background(),
		grantAuthorizeInput(
			fixture,
			issued.Token(),
			OperationBridgeEvent,
			"77777777-7777-4777-8777-777777777777",
		),
	)
	assertZeroGrantRecord(t, late)
	if !errors.Is(err, ErrRunNotGrantable) {
		t.Fatalf("terminal Authorize() error = %v", err)
	}
	revoked, err := authority.Revoke(
		context.Background(),
		RevokeInput{
			GrantID:       issued.Record().ID(),
			Reason:        RevocationTerminal,
			CorrelationID: grantTestCorrelation,
		},
	)
	if err != nil || revoked.RevocationReason() != RevocationTerminal {
		t.Fatalf("post-terminal Revoke() = %#v, %v", revoked, err)
	}

	t.Run("Run-first rejects Issue and Grant-first permits Start", func(t *testing.T) {
		runFirst := newGrantFixture(t, "run-first")
		generation := work.RunGenerationInput{
			WorkItemID:        runFirst.run.WorkItemID(),
			RunID:             runFirst.run.ID(),
			ClaimID:           runFirst.run.ClaimID(),
			ClaimGeneration:   runFirst.run.ClaimGeneration(),
			RuntimeInstanceID: runFirst.run.RuntimeInstanceID(),
			AgentInstanceID:   runFirst.run.AgentInstanceID(),
			CorrelationID:     grantTestCorrelation,
		}
		_, running, startErr := runFirst.workAuthority.Start(
			context.Background(),
			generation,
		)
		if startErr != nil {
			t.Fatal(startErr)
		}
		runFirst.run = running
		runFirstAuthority := newGrantTestAuthority(
			t,
			runFirst,
			newCountingGrantReader(0x91),
		)
		notIssued, issueErr := runFirstAuthority.Issue(
			context.Background(),
			grantIssueInput(runFirst, time.Minute),
		)
		assertZeroIssuedGrant(t, notIssued)
		if !errors.Is(issueErr, ErrRunNotGrantable) {
			t.Fatalf("Run-first Issue() error = %v", issueErr)
		}

		grantFirst := newGrantFixture(t, "grant-first")
		grantFirstAuthority := newGrantTestAuthority(
			t,
			grantFirst,
			newCountingGrantReader(0x92),
		)
		if _, issueErr := grantFirstAuthority.Issue(
			context.Background(),
			grantIssueInput(grantFirst, time.Minute),
		); issueErr != nil {
			t.Fatal(issueErr)
		}
		_, _, startErr = grantFirst.workAuthority.Start(
			context.Background(),
			work.RunGenerationInput{
				WorkItemID:        grantFirst.run.WorkItemID(),
				RunID:             grantFirst.run.ID(),
				ClaimID:           grantFirst.run.ClaimID(),
				ClaimGeneration:   grantFirst.run.ClaimGeneration(),
				RuntimeInstanceID: grantFirst.run.RuntimeInstanceID(),
				AgentInstanceID:   grantFirst.run.AgentInstanceID(),
				CorrelationID:     grantTestCorrelation,
			},
		)
		if startErr != nil {
			t.Fatalf("Grant-first Start() error = %v", startErr)
		}
	})
}

func TestTokenRedactionAndSourceFailure(t *testing.T) { // s3_w3_token_redaction_fuzz_static
	fixture := newGrantFixture(t, "source")
	shortReader := &countingGrantReader{bytes: bytes.Repeat([]byte{0x99}, 47)}
	authority := newGrantTestAuthority(t, fixture, shortReader)
	issued, err := authority.Issue(
		context.Background(),
		grantIssueInput(fixture, time.Minute),
	)
	assertZeroIssuedGrant(t, issued)
	if err == nil || strings.Contains(err.Error(), "99") {
		t.Fatalf("short random error = %v", err)
	}
	events, readErr := fixture.store.ReadStream(
		context.Background(),
		"agent-grant/run-1",
	)
	if readErr != nil || len(events) != 0 {
		t.Fatalf("short random appended events = %d, %v", len(events), readErr)
	}

	for _, raw := range []string{
		"",
		"loom_grant_v1",
		"loom_grant_v1.not-a-uuid.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"loom_grant_v1.11111111-1111-4111-8111-111111111111.short",
		strings.Repeat("x", 1024),
	} {
		token, parseErr := ParseToken(raw)
		if !errors.Is(parseErr, ErrInvalidGrantAuthorityInput) ||
			token.Value() != "" {
			t.Fatalf("ParseToken(%q) = %q, %v", raw, token.Value(), parseErr)
		}
		if parseErr != nil && strings.Contains(parseErr.Error(), raw) && raw != "" {
			t.Fatalf("parse error disclosed input: %v", parseErr)
		}
	}

	fixture.clock.Set(grantTestNow)
	zeroClockAuthority := newGrantTestAuthority(
		t,
		fixture,
		newCountingGrantReader(0xa0),
	)
	fixture.clock.Set(time.Time{})
	zeroClock, zeroClockErr := zeroClockAuthority.Issue(
		context.Background(),
		grantIssueInput(fixture, time.Minute),
	)
	assertZeroIssuedGrant(t, zeroClock)
	if !errors.Is(zeroClockErr, ErrInvalidGrantAuthorityInput) {
		t.Fatalf("zero clock error = %v", zeroClockErr)
	}
}

func TestGrantAuthorityAppendAndSourceFailuresAreAtomic(t *testing.T) {
	fixture := newGrantFixture(t, "append-failure")
	firstAuthority := newGrantTestAuthority(
		t,
		fixture,
		newCountingGrantReader(0xb0),
	)
	first, err := firstAuthority.Issue(
		context.Background(),
		grantIssueInput(fixture, 10*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.clock.Set(grantTestNow.Add(11 * time.Second))
	if _, err := fixture.db.ExecContext(
		context.Background(),
		`CREATE TRIGGER reject_agent_grant
		BEFORE INSERT ON events
		WHEN NEW.event_type IN (
			'AgentGrantIssued',
			'AgentGrantAuthorized',
			'AgentGrantRevoked'
		)
		BEGIN
			SELECT RAISE(ABORT, 'fixture grant rejection');
		END`,
	); err != nil {
		t.Fatal(err)
	}
	failingAuthority := newGrantTestAuthority(
		t,
		fixture,
		newCountingGrantReader(0xc0),
	)
	failed, err := failingAuthority.Issue(
		context.Background(),
		grantIssueInput(fixture, time.Minute),
	)
	assertZeroIssuedGrant(t, failed)
	if err == nil {
		t.Fatal("trigger-rejected Issue() succeeded")
	}
	events, readErr := fixture.store.ReadStream(
		context.Background(),
		"agent-grant/run-1",
	)
	if readErr != nil || len(events) != 1 ||
		events[0].Type != "AgentGrantIssued" {
		t.Fatalf("failed rotation facts = %#v, %v", events, readErr)
	}
	if _, err := fixture.db.ExecContext(
		context.Background(),
		`DROP TRIGGER reject_agent_grant`,
	); err != nil {
		t.Fatal(err)
	}
	active, err := firstAuthority.Authorize(
		context.Background(),
		grantAuthorizeInput(
			fixture,
			first.Token(),
			OperationBridgeEvent,
			"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		),
	)
	assertZeroGrantRecord(t, active)
	if !errors.Is(err, ErrGrantExpired) {
		t.Fatalf("failed rotation changed old Grant state: %v", err)
	}

	if err := fixture.db.Close(); err != nil {
		t.Fatal(err)
	}
	closedSnapshot, err := firstAuthority.Snapshot(context.Background())
	if err == nil || len(closedSnapshot.Grants()) != 0 {
		t.Fatalf("closed Snapshot() = %#v, %v", closedSnapshot, err)
	}
	closedAuthorization, err := firstAuthority.Authorize(
		context.Background(),
		grantAuthorizeInput(
			fixture,
			first.Token(),
			OperationBridgeEvent,
			"cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		),
	)
	assertZeroGrantRecord(t, closedAuthorization)
	if err == nil {
		t.Fatal("closed Authorize() succeeded")
	}
}

func FuzzGrantTokenAndReplayNeverPanic(f *testing.F) {
	for _, seed := range []string{
		"",
		"loom_grant_v1.11111111-1111-4111-8111-111111111111.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"{",
		strings.Repeat("x", 256),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		token, _ := ParseToken(input)
		_ = token.String()
		_ = token.GoString()
		_ = token.Value()
		state := &grantState{
			byID:     make(map[string]*GrantRecord),
			byHash:   make(map[string]*GrantRecord),
			active:   make(map[string]*GrantRecord),
			requests: make(map[string]map[string]authorizationDecision),
			heads:    make(map[string]int64),
		}
		_ = applyGrantEvent(state, journal.Event{
			ID:             "fuzz-grant-event",
			StreamID:       "agent-grant/fuzz-run",
			Seq:            1,
			IdempotencyKey: "fuzz-grant-event",
			Type:           "AgentGrantIssued",
			SchemaVersion:  1,
			EmittedAt:      grantTestNow,
			CorrelationID:  grantTestCorrelation,
			CausationID:    "fuzz-run-event",
			PayloadJSON:    []byte(input),
		})
	})
}

func assertGrantJSONKeys(t testing.TB, payload map[string]any, keys ...string) {
	t.Helper()
	got := make([]string, 0, len(payload))
	for key := range payload {
		got = append(got, key)
	}
	sort.Strings(got)
	sort.Strings(keys)
	if !reflect.DeepEqual(got, keys) {
		t.Fatalf("payload keys = %#v, want %#v", got, keys)
	}
}

func assertZeroGrantRecord(t testing.TB, record GrantRecord) {
	t.Helper()
	if record.ID() != "" || record.WorkItemID() != "" || record.RunID() != "" ||
		record.ClaimID() != "" || record.ClaimGeneration() != 0 ||
		record.RuntimeInstanceID() != "" || record.AgentInstanceID() != "" ||
		record.AllowedOperations() != nil || !record.IssuedAt().IsZero() ||
		!record.ExpiresAt().IsZero() || !record.RevokedAt().IsZero() ||
		record.RevocationReason() != "" {
		t.Fatalf("non-zero GrantRecord = %#v", record)
	}
}

func assertZeroIssuedGrant(t testing.TB, issued IssuedGrant) {
	t.Helper()
	assertZeroGrantRecord(t, issued.Record())
	if issued.Token().Value() != "" {
		t.Fatalf("non-zero token = %q", issued.Token().Value())
	}
}
