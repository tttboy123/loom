package projection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

func TestRemoteToolBackendEnrollmentProjectionRebuildsAndIsolatesAccounts(t *testing.T) {
	database, store := openLocalProductSetupProjectionStore(t)
	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	authority, err := work.NewAuthority(
		store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0xc1}, 4096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	workPolicy, err := authority.ConfigureProviderAccountPolicy(
		ctx, projectionRemoteToolPolicyCommand("deepseek", "deepseek.work"),
	)
	if err != nil {
		t.Fatal(err)
	}
	reviewPolicy, err := authority.ConfigureProviderAccountPolicy(
		ctx, projectionRemoteToolPolicyCommand("deepseek", "deepseek.review"),
	)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	search, err := authority.ConfigureRemoteToolBackendEnrollment(
		ctx,
		projectionRemoteToolEnrollmentCommand(
			"search-work", "web_search", "deepseek", "deepseek.work", workPolicy,
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	mcpCommand := projectionRemoteToolEnrollmentCommand(
		"mcp-review", "mcp_server", "deepseek", "deepseek.review", reviewPolicy,
	)
	mcpCommand.AdapterID = "builtin.mcp.stdio.v1"
	mcpCommand.MCPServerID = "review-tools"
	mcpCommand.AllowedTools = []string{"get_issue", "search_docs"}
	if _, err := authority.ConfigureRemoteToolBackendEnrollment(ctx, mcpCommand); err != nil {
		t.Fatal(err)
	}

	readModel := New(database)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	workEnrollments := readModel.GlobalReadView().RemoteToolBackendEnrollments(
		"deepseek", "deepseek.work",
	)
	if len(workEnrollments) != 1 ||
		workEnrollments[0].EnrollmentID() != search.EnrollmentID() ||
		workEnrollments[0].Digest() != search.Digest() {
		t.Fatalf("work enrollments = %#v", workEnrollments)
	}
	reviewEnrollments := readModel.GlobalReadView().RemoteToolBackendEnrollments(
		"deepseek", "deepseek.review",
	)
	if len(reviewEnrollments) != 1 || reviewEnrollments[0].MCPServerID() != "review-tools" {
		t.Fatalf("review enrollments = %#v", reviewEnrollments)
	}
	if got := readModel.GlobalReadView().RemoteToolBackendEnrollments(
		"openai", "deepseek.work",
	); len(got) != 0 {
		t.Fatalf("cross-Provider enrollments = %#v", got)
	}
	catalog := readModel.GlobalReadView().RemoteToolBackendEnrollmentCatalog()
	if len(catalog) != 2 ||
		catalog[0].EnrollmentID() != "mcp-review" ||
		catalog[0].ProviderAccountID() != "deepseek.review" ||
		catalog[1].EnrollmentID() != "search-work" ||
		catalog[1].ProviderAccountID() != "deepseek.work" {
		t.Fatalf("enrollment catalog = %#v", catalog)
	}

	now = now.Add(time.Minute)
	if _, err := authority.RevokeRemoteToolBackendEnrollment(
		ctx,
		work.RemoteToolBackendEnrollmentRevokeCommand{
			CommandID: "revoke-search-work", EnrollmentID: "search-work",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			ExpectedRevision: 1, CorrelationID: "projection-enrollment-revoke",
		},
	); err != nil {
		t.Fatal(err)
	}
	restarted := New(database)
	if err := restarted.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	workEnrollments = restarted.GlobalReadView().RemoteToolBackendEnrollments(
		"deepseek", "deepseek.work",
	)
	if len(workEnrollments) != 1 ||
		workEnrollments[0].Status() != work.RemoteToolBackendEnrollmentRevoked ||
		workEnrollments[0].Revision() != 2 {
		t.Fatalf("restarted work enrollments = %#v", workEnrollments)
	}
	if review := restarted.GlobalReadView().RemoteToolBackendEnrollments(
		"deepseek", "deepseek.review",
	); len(review) != 1 || review[0].Status() != work.RemoteToolBackendEnrollmentActive {
		t.Fatalf("peer enrollment changed = %#v", review)
	}
	restartedCatalog := restarted.GlobalReadView().RemoteToolBackendEnrollmentCatalog()
	if len(restartedCatalog) != 2 {
		t.Fatalf("restarted enrollment catalog = %#v", restartedCatalog)
	}
}

func TestRemoteToolBackendEnrollmentProjectionFailurePreservesPriorView(t *testing.T) {
	database, store := openLocalProductSetupProjectionStore(t)
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	authority, err := work.NewAuthority(
		store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0xc4}, 4096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	policy, err := authority.ConfigureProviderAccountPolicy(
		ctx, projectionRemoteToolPolicyCommand("deepseek", "deepseek.work"),
	)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if _, err := authority.ConfigureRemoteToolBackendEnrollment(
		ctx,
		projectionRemoteToolEnrollmentCommand(
			"search-work", "web_search", "deepseek", "deepseek.work", policy,
		),
	); err != nil {
		t.Fatal(err)
	}
	readModel := New(database)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	before := readModel.GlobalReadView().RemoteToolBackendEnrollments(
		"deepseek", "deepseek.work",
	)
	if len(before) != 1 || before[0].Revision() != 1 {
		t.Fatalf("accepted enrollment = %#v", before)
	}
	events, err := store.ReadStream(ctx, "remote-tool-backend-enrollment/search-work")
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %d, err=%v", len(events), err)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[0].PayloadJSON, &payload); err != nil {
		t.Fatal(err)
	}
	payload["command_id"] = "malformed-search-work-v2"
	payload["revision"] = float64(2)
	payload["configured_at"] = now.Add(time.Minute).Format(time.RFC3339Nano)
	payload["enrollment_digest"] = strings.Repeat("f", 64)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(ctx, journal.Event{
		ID: "malformed-search-work-v2", IdempotencyKey: "malformed-search-work-v2",
		StreamID: "remote-tool-backend-enrollment/search-work", Seq: 2,
		Type: "RemoteToolBackendEnrollmentConfigured", SchemaVersion: 1,
		EmittedAt: now.Add(time.Minute), CorrelationID: "projection-enrollment-malformed",
		CausationID: events[0].ID, PayloadJSON: body,
	}); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(ctx); !errors.Is(err, ErrInvalidProjectionEvent) {
		t.Fatalf("malformed rebuild error = %v", err)
	}
	after := readModel.GlobalReadView().RemoteToolBackendEnrollments(
		"deepseek", "deepseek.work",
	)
	if len(after) != 1 || after[0].Digest() != before[0].Digest() {
		t.Fatalf("prior view changed: before=%#v after=%#v", before, after)
	}
}

func projectionRemoteToolPolicyCommand(
	providerID string,
	accountID string,
) work.ProviderAccountPolicyCommand {
	return work.ProviderAccountPolicyCommand{
		CommandID:  "policy-" + strings.ReplaceAll(accountID, ".", "-"),
		ProviderID: providerID, ProviderAccountID: accountID,
		ExpectedRevision: 0, MaximumConcurrentAttempts: 4,
		DispatchWindow: time.Minute, MaximumDispatchStarts: 40,
		MaximumAssignedBudgetUnits: 10_000,
		TrustDomain:                "external_provider", RetentionMode: "zero_data_retention",
		DataRegion: "apac", CorrelationID: "projection-enrollment-policy",
	}
}

func projectionRemoteToolEnrollmentCommand(
	enrollmentID string,
	kind string,
	providerID string,
	accountID string,
	policy work.ProviderAccountPolicy,
) work.RemoteToolBackendEnrollmentCommand {
	return work.RemoteToolBackendEnrollmentCommand{
		CommandID: "configure-" + enrollmentID, EnrollmentID: enrollmentID,
		BackendKind: kind, AdapterID: "builtin.search.v1",
		ProviderID: providerID, ProviderAccountID: accountID,
		ProviderAccountPolicyVersion:  policy.Version(),
		ProviderAccountPolicyRevision: policy.Revision(),
		ProviderAccountPolicyDigest:   policy.Digest(),
		EndpointFingerprint:           strings.Repeat("d", 64), ExpectedRevision: 0,
		MaximumConcurrentCalls: 2, MaximumCallsPerAttempt: 4,
		Timeout: 20 * time.Second, MaximumResultBytes: 16 * 1024,
		MaximumBudgetUnits: 2_000, CorrelationID: "projection-enrollment-configure",
	}
}
