package work

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRemoteToolBackendEnrollmentAuthorityBindsExactAccountPolicyAndIsolatesAccounts(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xb8)

	workPolicy := configureRemoteToolTestPolicy(
		t, ctx, authority, "deepseek", "deepseek.work", 0,
	)
	reviewPolicy := configureRemoteToolTestPolicy(
		t, ctx, authority, "deepseek", "deepseek.review", 0,
	)
	clock.Set(testNow.Add(time.Minute))

	searchCommand := remoteToolBackendEnrollmentTestCommand(
		"search-deepseek-work", "web_search", "deepseek", "deepseek.work",
		workPolicy,
	)
	search, err := authority.ConfigureRemoteToolBackendEnrollment(ctx, searchCommand)
	if err != nil {
		t.Fatal(err)
	}
	if !search.Valid() || search.Version() != 1 || search.Revision() != 1 ||
		search.Status() != RemoteToolBackendEnrollmentActive ||
		search.EnrollmentID() != searchCommand.EnrollmentID ||
		search.BackendKind() != RemoteToolBackendWebSearch ||
		search.ProviderID() != "deepseek" ||
		search.ProviderAccountID() != "deepseek.work" ||
		search.ProviderAccountPolicyRevision() != workPolicy.Revision() ||
		search.ProviderAccountPolicyDigest() != workPolicy.Digest() ||
		search.AdapterID() != "builtin.search.deepseek.v1" ||
		search.EndpointFingerprint() != strings.Repeat("a", 64) ||
		search.MCPServerID() != "" || len(search.AllowedTools()) != 0 ||
		search.MaximumConcurrentCalls() != 2 ||
		search.MaximumCallsPerAttempt() != 4 ||
		search.Timeout() != 30*time.Second ||
		search.MaximumResultBytes() != 32*1024 ||
		search.MaximumBudgetUnits() != 2_000 || len(search.Digest()) != 64 {
		t.Fatalf("search enrollment = %#v", search)
	}
	if retried, retryErr := authority.ConfigureRemoteToolBackendEnrollment(
		ctx, searchCommand,
	); retryErr != nil || retried.Digest() != search.Digest() ||
		retried.Revision() != search.Revision() {
		t.Fatalf("idempotent retry = %#v, err=%v", retried, retryErr)
	}

	mcpCommand := remoteToolBackendEnrollmentTestCommand(
		"mcp-deepseek-review", "mcp_server", "deepseek", "deepseek.review",
		reviewPolicy,
	)
	mcpCommand.AdapterID = "builtin.mcp.stdio.v1"
	mcpCommand.EndpointFingerprint = strings.Repeat("b", 64)
	mcpCommand.MCPServerID = "review-tools"
	mcpCommand.AllowedTools = []string{"get_issue", "search_docs"}
	mcp, err := authority.ConfigureRemoteToolBackendEnrollment(ctx, mcpCommand)
	if err != nil {
		t.Fatal(err)
	}
	if mcp.ProviderAccountID() != "deepseek.review" ||
		mcp.ProviderAccountPolicyDigest() != reviewPolicy.Digest() ||
		mcp.MCPServerID() != "review-tools" ||
		len(mcp.AllowedTools()) != 2 {
		t.Fatalf("MCP enrollment = %#v", mcp)
	}

	gotSearch, err := authority.RemoteToolBackendEnrollment(ctx, search.EnrollmentID())
	if err != nil || gotSearch.Digest() != search.Digest() {
		t.Fatalf("replayed search = %#v, err=%v", gotSearch, err)
	}
	gotMCP, err := authority.RemoteToolBackendEnrollment(ctx, mcp.EnrollmentID())
	if err != nil || gotMCP.Digest() != mcp.Digest() {
		t.Fatalf("replayed MCP = %#v, err=%v", gotMCP, err)
	}
}

func TestRemoteToolBackendEnrollmentRejectsPolicyDriftAndInvalidBackendShapes(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xb9)
	policy := configureRemoteToolTestPolicy(
		t, ctx, authority, "deepseek", "deepseek.work", 0,
	)
	clock.Set(testNow.Add(time.Minute))
	base := remoteToolBackendEnrollmentTestCommand(
		"search-deepseek-work", "web_search", "deepseek", "deepseek.work", policy,
	)

	invalid := map[string]func(*RemoteToolBackendEnrollmentCommand){
		"unknown kind": func(input *RemoteToolBackendEnrollmentCommand) {
			input.BackendKind = "browser"
		},
		"search MCP server": func(input *RemoteToolBackendEnrollmentCommand) {
			input.MCPServerID = "search-server"
		},
		"search tools": func(input *RemoteToolBackendEnrollmentCommand) {
			input.AllowedTools = []string{"search"}
		},
		"MCP missing server": func(input *RemoteToolBackendEnrollmentCommand) {
			input.BackendKind = RemoteToolBackendMCPServer
			input.AllowedTools = []string{"search"}
		},
		"MCP unsorted tools": func(input *RemoteToolBackendEnrollmentCommand) {
			input.BackendKind = RemoteToolBackendMCPServer
			input.MCPServerID = "docs"
			input.AllowedTools = []string{"zeta", "alpha"}
		},
		"raw endpoint": func(input *RemoteToolBackendEnrollmentCommand) {
			input.EndpointFingerprint = "https://secret.example.test/token"
		},
		"oversized result": func(input *RemoteToolBackendEnrollmentCommand) {
			input.MaximumResultBytes = 1<<20 + 1
		},
		"over policy budget": func(input *RemoteToolBackendEnrollmentCommand) {
			input.MaximumBudgetUnits = policy.MaximumAssignedBudgetUnits() + 1
		},
	}
	for name, mutate := range invalid {
		t.Run(name, func(t *testing.T) {
			candidate := base
			candidate.AllowedTools = append([]string(nil), base.AllowedTools...)
			mutate(&candidate)
			if _, err := authority.ConfigureRemoteToolBackendEnrollment(
				ctx, candidate,
			); !errors.Is(err, ErrInvalidRemoteToolBackendEnrollment) {
				t.Fatalf("error = %v", err)
			}
		})
	}

	clock.Set(testNow.Add(2 * time.Minute))
	updatedPolicyCommand := providerAccountPolicyTestInput("deepseek", "deepseek.work", 1)
	updatedPolicyCommand.CommandID = "update-deepseek-work-policy"
	updatedPolicyCommand.MaximumAssignedBudgetUnits = 9_000
	if _, err := authority.ConfigureProviderAccountPolicy(ctx, updatedPolicyCommand); err != nil {
		t.Fatal(err)
	}
	clock.Set(testNow.Add(3 * time.Minute))
	if _, err := authority.ConfigureRemoteToolBackendEnrollment(
		ctx, base,
	); !errors.Is(err, ErrRemoteToolBackendPolicyDrift) {
		t.Fatalf("stale policy error = %v", err)
	}
}

func TestRemoteToolBackendEnrollmentRevocationIsVersionedAndDoesNotRequirePolicyMutation(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xba)
	policy := configureRemoteToolTestPolicy(
		t, ctx, authority, "deepseek", "deepseek.work", 0,
	)
	clock.Set(testNow.Add(time.Minute))
	active, err := authority.ConfigureRemoteToolBackendEnrollment(
		ctx,
		remoteToolBackendEnrollmentTestCommand(
			"search-deepseek-work", "web_search", "deepseek", "deepseek.work", policy,
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	clock.Set(testNow.Add(2 * time.Minute))
	revoke := RemoteToolBackendEnrollmentRevokeCommand{
		CommandID: "revoke-search-deepseek-work", EnrollmentID: active.EnrollmentID(),
		ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		ExpectedRevision: 1, CorrelationID: "remote-tool-enrollment-revoke",
	}
	revoked, err := authority.RevokeRemoteToolBackendEnrollment(ctx, revoke)
	if err != nil {
		t.Fatal(err)
	}
	if !revoked.Valid() || revoked.Status() != RemoteToolBackendEnrollmentRevoked ||
		revoked.Revision() != 2 || revoked.Digest() == active.Digest() ||
		revoked.ProviderAccountPolicyDigest() != active.ProviderAccountPolicyDigest() {
		t.Fatalf("revoked enrollment = %#v", revoked)
	}
	if retried, retryErr := authority.RevokeRemoteToolBackendEnrollment(
		ctx, revoke,
	); retryErr != nil || retried.Digest() != revoked.Digest() ||
		retried.Revision() != revoked.Revision() {
		t.Fatalf("idempotent revoke = %#v, err=%v", retried, retryErr)
	}
	if _, err := authority.ConfigureRemoteToolBackendEnrollment(
		ctx,
		remoteToolBackendEnrollmentTestCommand(
			"search-deepseek-peer", "web_search", "deepseek", "deepseek.review", policy,
		),
	); !errors.Is(err, ErrRemoteToolBackendPolicyDrift) {
		t.Fatalf("cross-account enrollment error = %v", err)
	}
}

func configureRemoteToolTestPolicy(
	t *testing.T,
	ctx context.Context,
	authority *Authority,
	providerID string,
	accountID string,
	expectedRevision int64,
) ProviderAccountPolicy {
	t.Helper()
	command := providerAccountPolicyTestInput(providerID, accountID, expectedRevision)
	command.CommandID = "configure-policy-" + strings.ReplaceAll(accountID, ".", "-")
	policy, err := authority.ConfigureProviderAccountPolicy(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func remoteToolBackendEnrollmentTestCommand(
	enrollmentID string,
	backendKind string,
	providerID string,
	accountID string,
	policy ProviderAccountPolicy,
) RemoteToolBackendEnrollmentCommand {
	return RemoteToolBackendEnrollmentCommand{
		CommandID: "configure-" + enrollmentID, EnrollmentID: enrollmentID,
		BackendKind: backendKind, AdapterID: "builtin.search.deepseek.v1",
		ProviderID: providerID, ProviderAccountID: accountID,
		ProviderAccountPolicyVersion:  policy.Version(),
		ProviderAccountPolicyRevision: policy.Revision(),
		ProviderAccountPolicyDigest:   policy.Digest(),
		EndpointFingerprint:           strings.Repeat("a", 64),
		ExpectedRevision:              0,
		MaximumConcurrentCalls:        2,
		MaximumCallsPerAttempt:        4,
		Timeout:                       30 * time.Second,
		MaximumResultBytes:            32 * 1024,
		MaximumBudgetUnits:            2_000,
		CorrelationID:                 "remote-tool-enrollment-configure",
	}
}
