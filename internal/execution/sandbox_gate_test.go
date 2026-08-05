package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	"loom-pi-rebuild/internal/permissions"
)

// stubSandboxGate is a deterministic Phase 3B policy gate for tests.
type stubSandboxGate struct {
	policy    SandboxPolicy
	policyErr error
	available bool
	availErr  error

	policyJobs []string
	availSeen  []string
}

func (gate *stubSandboxGate) ResolvePolicy(ctx context.Context, jobID string) (SandboxPolicy, error) {
	gate.policyJobs = append(gate.policyJobs, jobID)
	return gate.policy, gate.policyErr
}

func (gate *stubSandboxGate) BackendAvailable(ctx context.Context, backend string) (bool, error) {
	gate.availSeen = append(gate.availSeen, backend)
	return gate.available, gate.availErr
}

func mustSandboxAdapter(
	t testing.TB,
	gate SandboxGate,
) (*Adapter, *recordingExecutor, error) {
	t.Helper()
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-sb", nil)
	if err != nil {
		return nil, nil, err
	}
	executor := &recordingExecutor{}
	now := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, executor, now)
	if gate != nil {
		adapter = adapter.WithSandboxGate(gate)
	}
	return adapter, executor, nil
}

func sandboxProposal(operationID string) Proposal {
	return Proposal{
		JobID: execTestJobA, OperationID: operationID, JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "ls -la"},
	}
}

// TestRedSB2_RequiredSandboxUnavailableFailsClosed: a Required policy with an
// unavailable backend must deny with zero side effects and never fall back to
// local execution.
func TestRedSB2_RequiredSandboxUnavailableFailsClosed(t *testing.T) {
	gate := &stubSandboxGate{
		policy:    SandboxPolicy{Required: true, Backend: "gVisor"},
		available: false,
	}
	adapter, executor, err := mustSandboxAdapter(t, gate)
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), sandboxProposal("op-sb-deny"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictDeny {
		t.Fatalf("expected deny, got %+v", result)
	}
	if executor.runCount() != 0 {
		t.Fatalf("unavailable sandbox must never fall back to local execution, runs=%d", executor.runCount())
	}
	if len(gate.policyJobs) != 1 || gate.policyJobs[0] != execTestJobA {
		t.Fatalf("policy not resolved for job: %+v", gate.policyJobs)
	}
	if len(gate.availSeen) != 1 || gate.availSeen[0] != "gVisor" {
		t.Fatalf("backend availability not checked: %+v", gate.availSeen)
	}
	if result.Denial.Reason == "" || result.Denial.AuthorizationPath == "" {
		t.Fatalf("denial must carry actionable reason/path, got %+v", result.Denial)
	}
}

// TestRedSB2a_RequiredSandboxPolicyErrorFailsClosed: a policy resolution error
// must also fail closed.
func TestRedSB2a_RequiredSandboxPolicyErrorFailsClosed(t *testing.T) {
	gate := &stubSandboxGate{policyErr: errors.New("policy store unavailable")}
	adapter, executor, err := mustSandboxAdapter(t, gate)
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), sandboxProposal("op-sb-perr"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictDeny || executor.runCount() != 0 {
		t.Fatalf("policy error must fail closed: verdict=%s runs=%d", result.Verdict, executor.runCount())
	}
}

// TestRedSB2b_RequiredSandboxAvailableExecutes: with an available backend the
// allow path executes exactly once through the deterministic executor.
func TestRedSB2b_RequiredSandboxAvailableExecutes(t *testing.T) {
	gate := &stubSandboxGate{
		policy:    SandboxPolicy{Required: true, Backend: "gVisor"},
		available: true,
	}
	adapter, executor, err := mustSandboxAdapter(t, gate)
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), sandboxProposal("op-sb-ok"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictAllow || executor.runCount() != 1 {
		t.Fatalf("available sandbox must execute: result=%+v runs=%d", result, executor.runCount())
	}
	if len(gate.availSeen) != 1 {
		t.Fatalf("availability must be consulted once, got %+v", gate.availSeen)
	}
}

// TestRedSB2c_NilGatePreservesDefaultLocalExecution: no sandbox gate means no
// sandbox requirement; the default allow path still executes.
func TestRedSB2c_NilGatePreservesDefaultLocalExecution(t *testing.T) {
	adapter, executor, err := mustSandboxAdapter(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), sandboxProposal("op-sb-nil"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictAllow || executor.runCount() != 1 {
		t.Fatalf("nil gate must preserve default execution: result=%+v runs=%d", result, executor.runCount())
	}
}
