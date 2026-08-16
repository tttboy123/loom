package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/toolbroker"
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
	configureProductEnrollment(
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
		view, productRemoteToolEnrollmentDeps(&productRemoteToolBrokerConfig{
			Search: productEnrollmentSearchFixture{},
		}),
	)
	if err != nil {
		t.Fatalf("materialize error = %v", err)
	}
	if executor == nil {
		t.Fatal("expected one materialized executor")
	}
	allowed := executor.AllowedRemoteTools()
	if len(allowed) != 1 || allowed[0] != permissions.ToolWebSearch {
		t.Fatalf("allowed = %#v", allowed)
	}
	content, err := executor.ExecuteProposalContent(
		context.Background(),
		permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "Loom governance"},
	)
	if err != nil || len(content) == 0 {
		t.Fatalf("execute content=%q error=%v", content, err)
	}

	// Without any injected port, the same Enrollments expose no capability.
	withoutPort, err := newProductRemoteToolExecutorsFromEnrollments(
		view, productRemoteToolEnrollmentDeps(nil),
	)
	if err != nil || withoutPort != nil {
		t.Fatalf("port-less materialization = %#v error=%v", withoutPort, err)
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
		readModel.GlobalReadView(),
		productRemoteToolEnrollmentDeps(&productRemoteToolBrokerConfig{
			Search: productEnrollmentSearchFixture{},
		}),
	)
	if err != nil || afterRevoke != nil {
		t.Fatalf("post-revoke materialization = %#v error=%v", afterRevoke, err)
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

type productMaterializedExecutorFixture struct {
	content []byte
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
