package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	_ "modernc.org/sqlite"
)

func newFixture(t *testing.T) *IntegrationService {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	service, err := NewIntegrationService(
		journal.NewStore(database),
		func() time.Time { return time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func jid() string { return "123e4567-e89b-42d3-a456-426614174000" }

func TestTwoIntegratorsOneCASWinner(t *testing.T) {
	service := newFixture(t)
	ctx := context.Background()
	input := IntegrateInput{
		JourneyID: jid(), OperationID: "op-1",
		CandidateID: "candidate-1", TargetBranch: "main",
		BaseCommit: "base", SourceDigest: "src", EvidenceDigest: "ev",
	}
	if _, err := service.Integrate(ctx, input); err != nil {
		t.Fatal(err)
	}
	// Competing stale integration on the same target branch must lose.
	competing := input
	competing.OperationID = "op-2"
	if _, err := service.Integrate(ctx, competing); !errors.Is(err, ErrStaleIntegration) {
		t.Fatalf("competing integration = %v, want ErrStaleIntegration", err)
	}
}

func TestCanaryDoubleRunBlocked(t *testing.T) {
	service := newFixture(t)
	ctx := context.Background()
	input := CanaryInput{
		JourneyID: jid(), OperationID: "op-canary-1", RunID: "run-1",
		RuntimeInstanceID: "runtime.pi.earendil-works.0.82.1",
		ModelID:           "loom-local/qwen2.5", SkillDigest: "skill",
	}
	if _, err := service.StartCanary(ctx, input); err != nil {
		t.Fatal(err)
	}
	dup := input
	dup.OperationID = "op-canary-2"
	if _, err := service.StartCanary(ctx, dup); !errors.Is(err, ErrDuplicateCanary) {
		t.Fatalf("duplicate canary = %v, want ErrDuplicateCanary", err)
	}
}

func TestAdoptAndRollback(t *testing.T) {
	service := newFixture(t)
	ctx := context.Background()
	integrated, err := service.Integrate(ctx, IntegrateInput{
		JourneyID: jid(), OperationID: "op-1",
		CandidateID: "candidate-1", TargetBranch: "main",
		BaseCommit: "base", SourceDigest: "src", EvidenceDigest: "ev",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AdoptByLaterRun(ctx, AdoptInput{
		JourneyID: jid(), OperationID: "op-adopt",
		ReleaseID: integrated.ReleaseID, RunID: "run-later",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Rollback(ctx, RollbackInput{
		JourneyID: jid(), OperationID: "op-rollback",
		ReleaseID: integrated.ReleaseID, RollbackToID: "release-prior",
	}); err != nil {
		t.Fatal(err)
	}
	releases, err := service.ReleaseProjection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 || releases[0].AdoptedByRunID != "run-later" ||
		releases[0].Status != "rolled_back" {
		t.Fatalf("releases = %+v", releases)
	}
}

func TestRollbackUnavailableForSameRelease(t *testing.T) {
	service := newFixture(t)
	integrated, err := service.Integrate(context.Background(), IntegrateInput{
		JourneyID: jid(), OperationID: "op-1",
		CandidateID: "candidate-1", TargetBranch: "main",
		BaseCommit: "base", SourceDigest: "src", EvidenceDigest: "ev",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Rollback(context.Background(), RollbackInput{
		JourneyID: jid(), OperationID: "op-rb",
		ReleaseID: integrated.ReleaseID, RollbackToID: integrated.ReleaseID,
	}); !errors.Is(err, ErrRollbackUnavailable) {
		t.Fatalf("rollback to self = %v", err)
	}
}
