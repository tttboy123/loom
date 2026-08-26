package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/toolbroker"
	"loom-pi-rebuild/internal/toolbroker/enrollment"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

type productEnrollmentSearchFixture struct{}

func (productEnrollmentSearchFixture) Search(
	context.Context, string, int,
) ([]toolbroker.SearchResult, error) {
	return []toolbroker.SearchResult{{Title: "t", URL: "https://example.com"}}, nil
}

func openProductEnrollmentDB(t *testing.T) *sql.DB {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open(
		"sqlite", fmt.Sprintf("file:%s/enrollment.db?%s", t.TempDir(), values.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func configureProductEnrollmentPolicy(
	t *testing.T,
	authority *work.Authority,
	providerID string,
	accountID string,
	revision int64,
) work.ProviderAccountPolicy {
	t.Helper()
	policy, err := authority.ConfigureProviderAccountPolicy(
		context.Background(),
		work.ProviderAccountPolicyCommand{
			CommandID:                  "policy-" + accountID,
			ProviderID:                 providerID,
			ProviderAccountID:          accountID,
			ExpectedRevision:           revision,
			MaximumConcurrentAttempts:  4,
			DispatchWindow:             60 * time.Second,
			MaximumDispatchStarts:      20,
			MaximumAssignedBudgetUnits: 10_000,
			TrustDomain:                "external_provider",
			RetentionMode:              "zero_data_retention",
			DataRegion:                 "apac",
			CorrelationID:              "enrollment-policy",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func configureProductEnrollment(
	t *testing.T,
	authority *work.Authority,
	policy work.ProviderAccountPolicy,
	enrollmentID string,
	adapterID string,
	backendKind string,
	accountID string,
	status string,
) work.RemoteToolBackendEnrollment {
	t.Helper()
	now := time.Now().UTC()
	command := work.RemoteToolBackendEnrollmentCommand{
		CommandID:                     "configure-" + enrollmentID,
		EnrollmentID:                  enrollmentID,
		BackendKind:                   backendKind,
		AdapterID:                     adapterID,
		ProviderID:                    policy.ProviderID(),
		ProviderAccountID:             accountID,
		ProviderAccountPolicyVersion:  policy.Version(),
		ProviderAccountPolicyRevision: policy.Revision(),
		ProviderAccountPolicyDigest:   policy.Digest(),
		EndpointFingerprint:           strings.Repeat("e", 64),
		ExpectedRevision:              0,
		MaximumConcurrentCalls:        2,
		MaximumCallsPerAttempt:        4,
		Timeout:                       30 * time.Second,
		MaximumResultBytes:            1 << 16,
		MaximumBudgetUnits:            2_000,
		CorrelationID:                 "enrollment-configure",
	}
	enrollment, err := authority.ConfigureRemoteToolBackendEnrollment(
		context.Background(), command,
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = now
	return enrollment
}

func TestProductRemoteToolExecutorsFromEnrollmentsMaterializesOnlyTrustedActive(t *testing.T) {
	db := openProductEnrollmentDB(t)
	store := journal.NewStore(db)
	now := time.Unix(3_101, 0).UTC()
	authority, err := work.NewAuthority(
		store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0xc2}, 4_096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	policy := configureProductEnrollmentPolicy(t, authority, "deepseek", "deepseek.primary", 0)
	boundEnrollment := configureProductEnrollment(
		t, authority, policy, "enr-search", work.BuiltInSearchAdapterID,
		work.RemoteToolBackendWebSearch, "deepseek.primary", work.RemoteToolBackendEnrollmentActive,
	)
	configureProductEnrollment(
		t, authority, policy, "enr-unsupported", "arbitrary.adapter.v9",
		work.RemoteToolBackendWebSearch, "deepseek.primary", work.RemoteToolBackendEnrollmentActive,
	)
	// Advance the clock, configure a third Enrollment under a second account,
	// then revoke it to prove revocation records are skipped.
	now = now.Add(time.Minute)
	revokedPolicy := configureProductEnrollmentPolicy(t, authority, "deepseek", "deepseek.review", 0)
	configureProductEnrollment(
		t, authority, revokedPolicy, "enr-revoked", work.BuiltInSearchAdapterID,
		work.RemoteToolBackendWebSearch, "deepseek.review", work.RemoteToolBackendEnrollmentActive,
	)
	now = now.Add(time.Minute)
	if _, err := authority.RevokeRemoteToolBackendEnrollment(
		context.Background(),
		work.RemoteToolBackendEnrollmentRevokeCommand{
			CommandID: "revoke-enr-revoked", EnrollmentID: "enr-revoked",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.review",
			ExpectedRevision: 1, CorrelationID: "enrollment-revoke",
		},
	); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	view := readModel.GlobalReadView()
	catalog := view.RemoteToolBackendEnrollmentCatalog()
	if len(catalog) != 3 {
		t.Fatalf("enrollment catalog = %#v", catalog)
	}

	// With the Search port injected, exactly the trusted active Enrollment
	// materializes (the unsupported adapter is skipped; revoked is skipped).
	executor, err := newProductRemoteToolExecutorsFromEnrollments(
		func() projection.GlobalReadView { return view },
		productRemoteToolEnrollmentDeps(&productRemoteToolBrokerConfig{
			Search: productEnrollmentSearchFixture{},
		}),
	)
	if err != nil {
		t.Fatalf("materialize error = %v", err)
	}
	if executor == nil {
		t.Fatal("expected one materialized executor")
	}
	if allowed := executor.AllowedRemoteTools(); len(allowed) != 0 {
		t.Fatalf("unbound Enrollment executor exposed tools = %#v", allowed)
	}
	if _, err := executor.ExecuteProposalContent(
		context.Background(),
		permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "Loom governance"},
	); !errors.Is(err, execution.ErrUnsupportedTool) {
		t.Fatalf("unbound Enrollment execute error = %v", err)
	}
	scoped, ok := executor.(execution.ScopedRemoteToolExecutor)
	if !ok {
		t.Fatal("Enrollment executor does not expose exact scoped capability")
	}
	scope := execution.RemoteToolScope{
		EnrollmentID: boundEnrollment.EnrollmentID(), EnrollmentDigest: boundEnrollment.Digest(),
	}
	allowed := scoped.AllowedRemoteToolsForScope(scope)
	if len(allowed) != 1 || allowed[0] != permissions.ToolWebSearch {
		t.Fatalf("exact scoped allowed tools = %#v", allowed)
	}
	content, err := scoped.ExecuteProposalContentForScope(
		context.Background(), scope,
		permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "Loom governance"},
	)
	if err != nil || len(content) == 0 {
		t.Fatalf("scoped execute content=%q error=%v", content, err)
	}
	if allowed := scoped.AllowedRemoteToolsForScope(execution.RemoteToolScope{
		EnrollmentID: boundEnrollment.EnrollmentID(), EnrollmentDigest: strings.Repeat("f", 64),
	}); len(allowed) != 0 {
		t.Fatalf("wrong Enrollment digest exposed tools = %#v", allowed)
	}
	capabilityPort := &productEnrollmentCapabilityPortFixture{remote: executor}
	hook, err := newBridgeExecutionHook(capabilityPort, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if containsProductToolKind(hook.AllowedToolCalls(), permissions.ToolWebSearch) {
		t.Fatal("unbound bridge advertised enrollment-backed WebSearch")
	}
	if !containsProductToolKind(hook.AllowedToolCallsForBinding(
		loomruntime.FrozenExecutionBinding{
			RemoteToolEnrollmentID:     boundEnrollment.EnrollmentID(),
			RemoteToolEnrollmentDigest: boundEnrollment.Digest(),
		},
	), permissions.ToolWebSearch) {
		t.Fatal("exact frozen Enrollment did not advertise WebSearch")
	}

	// Without any injected port, the same Enrollments expose no capability.
	withoutPort, err := newProductRemoteToolExecutorsFromEnrollments(
		func() projection.GlobalReadView { return view },
		productRemoteToolEnrollmentDeps(nil),
	)
	if err != nil || withoutPort != nil && len(withoutPort.AllowedRemoteTools()) != 0 {
		t.Fatalf("port-less materialization exposes tools = %#v error=%v", withoutPort, err)
	}
	withoutPortScoped := withoutPort.(execution.ScopedRemoteToolExecutor)
	if allowed := withoutPortScoped.AllowedRemoteToolsForScope(scope); len(allowed) != 0 {
		t.Fatalf("port-less exact scope exposed tools = %#v", allowed)
	}

	// Revoking the remaining active Enrollment proves revocation makes the
	// same record unavailable for materialization.
	if _, err := authority.RevokeRemoteToolBackendEnrollment(
		context.Background(),
		work.RemoteToolBackendEnrollmentRevokeCommand{
			CommandID: "revoke-enr-search", EnrollmentID: "enr-search",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			ExpectedRevision: 1, CorrelationID: "enrollment-revoke",
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	afterRevoke, err := newProductRemoteToolExecutorsFromEnrollments(
		func() projection.GlobalReadView { return readModel.GlobalReadView() },
		productRemoteToolEnrollmentDeps(&productRemoteToolBrokerConfig{
			Search: productEnrollmentSearchFixture{},
		}),
	)
	if err != nil || afterRevoke != nil && len(afterRevoke.AllowedRemoteTools()) != 0 {
		t.Fatalf("post-revoke materialization exposes tools = %#v error=%v", afterRevoke, err)
	}
	afterRevokeScoped := afterRevoke.(execution.ScopedRemoteToolExecutor)
	if allowed := afterRevokeScoped.AllowedRemoteToolsForScope(scope); len(allowed) != 0 {
		t.Fatalf("revoked exact scope exposed tools = %#v", allowed)
	}
	if err := afterRevokeScoped.ValidateProposalForScope(
		context.Background(),
		scope,
		permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "Loom governance"},
	); !errors.Is(err, execution.ErrRemoteToolBindingRevoked) ||
		!errors.Is(err, enrollment.ErrToolEnrollmentRevoked) {
		t.Fatalf("revoked exact scope validation error = %v", err)
	}
}

func TestProductDynamicRemoteToolExecutorSuppressesInflightRevokedEnrollmentResult(t *testing.T) {
	db := openProductEnrollmentDB(t)
	store := journal.NewStore(db)
	now := time.Unix(3_201, 0).UTC()
	authority, err := work.NewAuthority(
		store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0xd2}, 4_096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	policy := configureProductEnrollmentPolicy(
		t, authority, "deepseek", "deepseek.primary", 0,
	)
	boundEnrollment := configureProductEnrollment(
		t, authority, policy, "enr-inflight", work.BuiltInSearchAdapterID,
		work.RemoteToolBackendWebSearch, "deepseek.primary",
		work.RemoteToolBackendEnrollmentActive,
	)
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	search := &productBlockingEnrollmentSearchFixture{
		started: make(chan struct{}), release: make(chan struct{}),
	}
	executor, err := newProductRemoteToolExecutorsFromEnrollments(
		func() projection.GlobalReadView { return readModel.GlobalReadView() },
		productRemoteToolEnrollmentDeps(&productRemoteToolBrokerConfig{Search: search}),
	)
	if err != nil {
		t.Fatal(err)
	}
	scoped, ok := executor.(execution.ScopedRemoteToolExecutor)
	if !ok {
		t.Fatal("dynamic Enrollment executor is not Attempt-scoped")
	}
	boundScope := execution.RemoteToolScope{
		EnrollmentID:     boundEnrollment.EnrollmentID(),
		EnrollmentDigest: boundEnrollment.Digest(),
	}
	type executeResult struct {
		content []byte
		err     error
	}
	result := make(chan executeResult, 1)
	go func() {
		content, executeErr := scoped.ExecuteProposalContentForScope(
			context.Background(), boundScope,
			permissions.ProposedCall{
				Tool: permissions.ToolWebSearch, Path: "private inflight query",
			},
		)
		result <- executeResult{content: content, err: executeErr}
	}()
	<-search.started

	// Add a healthy peer while the first call is blocked. The dynamic executor
	// must pick it up after the bound Enrollment is revoked.
	now = now.Add(time.Minute)
	peerPolicy := configureProductEnrollmentPolicy(
		t, authority, "deepseek", "deepseek.peer", 0,
	)
	peerEnrollment := configureProductEnrollment(
		t, authority, peerPolicy, "enr-peer", work.BuiltInSearchAdapterID,
		work.RemoteToolBackendWebSearch, "deepseek.peer",
		work.RemoteToolBackendEnrollmentActive,
	)
	now = now.Add(time.Minute)
	if _, err := authority.RevokeRemoteToolBackendEnrollment(
		context.Background(),
		work.RemoteToolBackendEnrollmentRevokeCommand{
			CommandID: "revoke-enr-inflight", EnrollmentID: "enr-inflight",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			ExpectedRevision: 1, CorrelationID: "enrollment-revoke",
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	var got executeResult
	select {
	case got = <-result:
	case <-time.After(2 * time.Second):
		close(search.release)
		t.Fatal("revoked Enrollment did not cancel the in-flight backend call")
	}
	close(search.release)
	if len(got.content) != 0 {
		t.Fatalf("revoked result content was delivered: %q", got.content)
	}
	if !errors.Is(got.err, toolbroker.ErrToolFailed) ||
		!errors.Is(got.err, enrollment.ErrToolEnrollmentRevoked) {
		t.Fatalf("revoked result error = %v", got.err)
	}
	if strings.Contains(got.err.Error(), productBlockedEnrollmentResultMarker) {
		t.Fatalf("revoked result body leaked through error: %v", got.err)
	}

	peerContent, err := scoped.ExecuteProposalContentForScope(
		context.Background(), execution.RemoteToolScope{
			EnrollmentID:     peerEnrollment.EnrollmentID(),
			EnrollmentDigest: peerEnrollment.Digest(),
		},
		permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "peer query"},
	)
	if err != nil || !bytes.Contains(peerContent, []byte(productBlockedEnrollmentResultMarker)) {
		t.Fatalf("peer execute content=%q error=%v", peerContent, err)
	}

	defaultExecutor, err := newProductRemoteToolExecutorsFromEnrollments(
		func() projection.GlobalReadView { return readModel.GlobalReadView() },
		productRemoteToolEnrollmentDeps(nil),
	)
	if err != nil {
		t.Fatal(err)
	}
	if allowed := defaultExecutor.AllowedRemoteTools(); len(allowed) != 0 {
		t.Fatalf("port-less default exposed tools = %#v", allowed)
	}
	if _, err := defaultExecutor.ExecuteProposalContent(
		context.Background(),
		permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "default query"},
	); !errors.Is(err, execution.ErrUnsupportedTool) {
		t.Fatalf("port-less default execute error = %v", err)
	}
}

func TestProductDynamicRemoteToolExecutorSuppressesInflightPolicyDriftResult(t *testing.T) {
	db := openProductEnrollmentDB(t)
	store := journal.NewStore(db)
	now := time.Unix(3_301, 0).UTC()
	authority, err := work.NewAuthority(
		store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0xe2}, 4_096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	policy := configureProductEnrollmentPolicy(
		t, authority, "deepseek", "deepseek.primary", 0,
	)
	boundEnrollment := configureProductEnrollment(
		t, authority, policy, "enr-policy-drift", work.BuiltInSearchAdapterID,
		work.RemoteToolBackendWebSearch, "deepseek.primary",
		work.RemoteToolBackendEnrollmentActive,
	)
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	search := &productBlockingEnrollmentSearchFixture{
		started: make(chan struct{}), release: make(chan struct{}),
	}
	executor, err := newProductRemoteToolExecutorsFromEnrollments(
		func() projection.GlobalReadView { return readModel.GlobalReadView() },
		productRemoteToolEnrollmentDeps(&productRemoteToolBrokerConfig{Search: search}),
	)
	if err != nil {
		t.Fatal(err)
	}
	type executeResult struct {
		content []byte
		err     error
	}
	result := make(chan executeResult, 1)
	go func() {
		scoped := executor.(execution.ScopedRemoteToolExecutor)
		content, executeErr := scoped.ExecuteProposalContentForScope(
			context.Background(),
			execution.RemoteToolScope{
				EnrollmentID:     boundEnrollment.EnrollmentID(),
				EnrollmentDigest: boundEnrollment.Digest(),
			},
			permissions.ProposedCall{
				Tool: permissions.ToolWebSearch, Path: "policy drift query",
			},
		)
		result <- executeResult{content: content, err: executeErr}
	}()
	<-search.started

	now = now.Add(time.Minute)
	if _, err := authority.ConfigureProviderAccountPolicy(
		context.Background(),
		work.ProviderAccountPolicyCommand{
			CommandID:  "drift-policy-deepseek-primary",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			ExpectedRevision:           1,
			MaximumConcurrentAttempts:  4,
			DispatchWindow:             60 * time.Second,
			MaximumDispatchStarts:      20,
			MaximumAssignedBudgetUnits: 9_000,
			TrustDomain:                "external_provider",
			RetentionMode:              "zero_data_retention",
			DataRegion:                 "apac",
			CorrelationID:              "enrollment-policy-drift",
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	scoped := executor.(execution.ScopedRemoteToolExecutor)
	driftedScope := execution.RemoteToolScope{
		EnrollmentID:     boundEnrollment.EnrollmentID(),
		EnrollmentDigest: boundEnrollment.Digest(),
	}
	if allowed := scoped.AllowedRemoteToolsForScope(driftedScope); len(allowed) != 0 {
		t.Fatalf("policy-drifted frozen scope exposed tools = %#v", allowed)
	}
	if err := scoped.ValidateProposalForScope(
		context.Background(),
		driftedScope,
		permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "policy drift query"},
	); !errors.Is(err, execution.ErrRemoteToolBindingPolicyDrift) ||
		!errors.Is(err, enrollment.ErrToolEnrollmentPolicyDrift) {
		t.Fatalf("policy-drifted scope validation error = %v", err)
	}
	close(search.release)

	got := <-result
	if len(got.content) != 0 {
		t.Fatalf("policy-drifted result content was delivered: %q", got.content)
	}
	if !errors.Is(got.err, toolbroker.ErrToolFailed) ||
		!errors.Is(got.err, enrollment.ErrToolEnrollmentPolicyDrift) {
		t.Fatalf("policy-drifted result error = %v", got.err)
	}
	if strings.Contains(got.err.Error(), productBlockedEnrollmentResultMarker) {
		t.Fatalf("policy-drifted result body leaked through error: %v", got.err)
	}
}

func TestProductCompositeRemoteToolExecutorCloseIsIdempotent(t *testing.T) {
	store := openProductEnrollmentDB(t)
	_ = store
	// A composite over one fake executor must validate, execute and close.
	fake := &productMaterializedExecutorFixture{content: []byte("bounded")}
	composite, err := newProductCompositeRemoteToolExecutor(
		[]execution.RemoteToolExecutor{fake},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := composite.ValidateProposal(permissions.ProposedCall{
		Tool: permissions.ToolWebSearch, Path: "x",
	}); err != nil {
		t.Fatalf("ValidateProposal error = %v", err)
	}
	content, err := composite.ExecuteProposalContent(
		context.Background(),
		permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "x"},
	)
	if err != nil || !bytes.Equal(content, []byte("bounded")) {
		t.Fatalf("execute = %q error=%v", content, err)
	}
	if err := composite.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}
	if err := composite.Close(); err != nil {
		t.Fatalf("second Close error = %v", err)
	}
	if len(composite.AllowedRemoteTools()) != 0 {
		t.Fatalf("closed composite still exposes tools")
	}
	if _, err := composite.ExecuteProposalContent(
		context.Background(),
		permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "x"},
	); !errors.Is(err, execution.ErrUnsupportedTool) {
		t.Fatalf("closed composite executed: %v", err)
	}
}

func TestProductCompositeRemoteToolExecutorKeepsScopedCallsOnScopedMembers(t *testing.T) {
	unscoped := &productUnscopedMCPExecutorFixture{}
	scoped := &productScopedMCPExecutorFixture{}
	composite, err := newProductCompositeRemoteToolExecutor(
		[]execution.RemoteToolExecutor{unscoped, scoped},
	)
	if err != nil {
		t.Fatal(err)
	}
	scope := execution.RemoteToolScope{
		EnrollmentID: "enr-mcp-exact", EnrollmentDigest: strings.Repeat("a", 64),
	}
	proposal := permissions.ProposedCall{
		Tool: permissions.ToolMCPTool, Path: "mcp.live.probe/lookup", Command: `{}`,
	}
	if allowed := composite.AllowedRemoteToolsForScope(scope); !reflect.DeepEqual(
		allowed, []permissions.ToolKind{permissions.ToolMCPTool},
	) {
		t.Fatalf("scoped allowed tools = %#v", allowed)
	}
	if err := composite.ValidateProposalForScope(
		context.Background(), scope, proposal,
	); err != nil {
		t.Fatal(err)
	}
	content, err := composite.ExecuteProposalContentForScope(
		context.Background(), scope, proposal,
	)
	if err != nil || string(content) != "scoped-result" {
		t.Fatalf("scoped content=%q err=%v", content, err)
	}
	if unscoped.calls != 0 || scoped.calls != 1 {
		t.Fatalf("unscoped calls=%d scoped calls=%d", unscoped.calls, scoped.calls)
	}
}

func TestProductDynamicRemoteToolExecutorCloseCancelsScopedInflightResult(t *testing.T) {
	db := openProductEnrollmentDB(t)
	store := journal.NewStore(db)
	now := time.Unix(3_301, 0).UTC()
	authority, err := work.NewAuthority(
		store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0xe3}, 4_096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	policy := configureProductEnrollmentPolicy(
		t, authority, "deepseek", "deepseek.primary", 0,
	)
	bound := configureProductEnrollment(
		t, authority, policy, "enr-close-scope", work.BuiltInSearchAdapterID,
		work.RemoteToolBackendWebSearch, "deepseek.primary",
		work.RemoteToolBackendEnrollmentActive,
	)
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	search := &productBlockingEnrollmentSearchFixture{
		started: make(chan struct{}), release: make(chan struct{}),
	}
	executor, err := newProductDynamicRemoteToolExecutor(
		func() projection.GlobalReadView { return readModel.GlobalReadView() },
		productRemoteToolEnrollmentDeps(&productRemoteToolBrokerConfig{Search: search}),
	)
	if err != nil {
		t.Fatal(err)
	}
	type executeResult struct {
		content []byte
		err     error
	}
	result := make(chan executeResult, 1)
	go func() {
		content, executeErr := executor.ExecuteProposalContentForScope(
			context.Background(),
			execution.RemoteToolScope{
				EnrollmentID: bound.EnrollmentID(), EnrollmentDigest: bound.Digest(),
			},
			permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "close race"},
		)
		result <- executeResult{content: content, err: executeErr}
	}()
	<-search.started
	if err := executor.Close(); err != nil {
		t.Fatal(err)
	}
	close(search.release)
	got := <-result
	if len(got.content) != 0 || !errors.Is(got.err, execution.ErrUnsupportedTool) {
		t.Fatalf("closed scoped result content=%q err=%v", got.content, got.err)
	}
}

type productMaterializedExecutorFixture struct {
	content []byte
}

type productEnrollmentCapabilityPortFixture struct {
	remote execution.RemoteToolExecutor
}

func (*productEnrollmentCapabilityPortFixture) Execute(
	context.Context,
	execution.Proposal,
) (execution.ExecutionResult, error) {
	return execution.ExecutionResult{}, execution.ErrUnsupportedTool
}

func (fixture *productEnrollmentCapabilityPortFixture) RemoteToolKinds() []permissions.ToolKind {
	return fixture.remote.AllowedRemoteTools()
}

func (fixture *productEnrollmentCapabilityPortFixture) RemoteToolKindsForScope(
	scope execution.RemoteToolScope,
) []permissions.ToolKind {
	remote, ok := fixture.remote.(execution.ScopedRemoteToolExecutor)
	if !ok {
		return nil
	}
	return remote.AllowedRemoteToolsForScope(scope)
}

func (fixture *productEnrollmentCapabilityPortFixture) RevalidateRemoteToolScope(
	ctx context.Context,
	scope execution.RemoteToolScope,
	proposal permissions.ProposedCall,
) error {
	remote, ok := fixture.remote.(execution.ScopedRemoteToolExecutor)
	if !ok {
		return execution.ErrUnsupportedTool
	}
	return remote.ValidateProposalForScope(ctx, scope, proposal)
}

func containsProductToolKind(
	tools []permissions.ToolKind,
	wanted permissions.ToolKind,
) bool {
	for _, tool := range tools {
		if tool == wanted {
			return true
		}
	}
	return false
}

type productUnscopedMCPExecutorFixture struct {
	calls int
}

func (fixture *productUnscopedMCPExecutorFixture) AllowedRemoteTools() []permissions.ToolKind {
	return []permissions.ToolKind{permissions.ToolMCPTool}
}

func (fixture *productUnscopedMCPExecutorFixture) ValidateProposal(
	permissions.ProposedCall,
) error {
	return nil
}

func (fixture *productUnscopedMCPExecutorFixture) ExecuteProposalContent(
	context.Context,
	permissions.ProposedCall,
) ([]byte, error) {
	fixture.calls++
	return []byte("unscoped-result"), nil
}

type productScopedMCPExecutorFixture struct {
	calls int
}

func (fixture *productScopedMCPExecutorFixture) AllowedRemoteTools() []permissions.ToolKind {
	return []permissions.ToolKind{permissions.ToolMCPTool}
}

func (fixture *productScopedMCPExecutorFixture) ValidateProposal(
	permissions.ProposedCall,
) error {
	return execution.ErrUnsupportedTool
}

func (fixture *productScopedMCPExecutorFixture) ExecuteProposalContent(
	context.Context,
	permissions.ProposedCall,
) ([]byte, error) {
	return nil, execution.ErrUnsupportedTool
}

func (fixture *productScopedMCPExecutorFixture) AllowedRemoteToolsForScope(
	execution.RemoteToolScope,
) []permissions.ToolKind {
	return []permissions.ToolKind{permissions.ToolMCPTool}
}

func (fixture *productScopedMCPExecutorFixture) ValidateProposalForScope(
	_ context.Context,
	_ execution.RemoteToolScope,
	proposal permissions.ProposedCall,
) error {
	if proposal.Tool != permissions.ToolMCPTool || proposal.Path != "mcp.live.probe/lookup" {
		return execution.ErrUnsupportedTool
	}
	return nil
}

func (fixture *productScopedMCPExecutorFixture) ExecuteProposalContentForScope(
	_ context.Context,
	_ execution.RemoteToolScope,
	proposal permissions.ProposedCall,
) ([]byte, error) {
	if err := fixture.ValidateProposalForScope(context.Background(), execution.RemoteToolScope{}, proposal); err != nil {
		return nil, err
	}
	fixture.calls++
	return []byte("scoped-result"), nil
}

const productBlockedEnrollmentResultMarker = "SENSITIVE-RETURNED-CONTENT"

type productBlockingEnrollmentSearchFixture struct {
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func (fixture *productBlockingEnrollmentSearchFixture) Search(
	ctx context.Context,
	_ string,
	_ int,
) ([]toolbroker.SearchResult, error) {
	blocked := false
	fixture.once.Do(func() {
		blocked = true
		close(fixture.started)
	})
	if blocked {
		select {
		case <-fixture.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return []toolbroker.SearchResult{{
		Title: productBlockedEnrollmentResultMarker,
		URL:   "https://example.com/result",
	}}, nil
}

func (fixture *productMaterializedExecutorFixture) AllowedRemoteTools() []permissions.ToolKind {
	return []permissions.ToolKind{permissions.ToolWebSearch}
}

func (fixture *productMaterializedExecutorFixture) ValidateProposal(
	proposal permissions.ProposedCall,
) error {
	if proposal.Tool != permissions.ToolWebSearch || proposal.Path == "" {
		return execution.ErrUnsupportedTool
	}
	return nil
}

func (fixture *productMaterializedExecutorFixture) ExecuteProposalContent(
	_ context.Context,
	proposal permissions.ProposedCall,
) ([]byte, error) {
	if err := fixture.ValidateProposal(proposal); err != nil {
		return nil, err
	}
	return append([]byte(nil), fixture.content...), nil
}
