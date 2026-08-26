package execution

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/toolbroker"
)

// liveRemoteToolExecutor wraps the real Loom tool broker (governed web search
// + web fetch) behind the RemoteToolExecutor contract the execution adapter
// consumes. It is the same engine the daemon's productRemoteToolExecutor uses.
type liveRemoteToolExecutor struct {
	broker *toolbroker.Broker
}

func newLiveRemoteToolExecutor(t *testing.T) *liveRemoteToolExecutor {
	t.Helper()
	search, err := toolbroker.NewDDGSearchClient(20*time.Second, 5, 48<<10)
	if err != nil {
		t.Fatal(err)
	}
	httpClient, err := toolbroker.NewSystemHTTPClient(20 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	broker, err := toolbroker.New(toolbroker.Config{
		Search: search, HTTP: httpClient, Timeout: 20 * time.Second,
		MaxResultBytes: 32 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &liveRemoteToolExecutor{broker: broker}
}

func (executor *liveRemoteToolExecutor) AllowedRemoteTools() []permissions.ToolKind {
	return executor.broker.AllowedRemoteTools()
}

func (executor *liveRemoteToolExecutor) ValidateProposal(
	proposal permissions.ProposedCall,
) error {
	return executor.broker.ValidateProposal(proposal)
}

func (executor *liveRemoteToolExecutor) ExecuteProposalContent(
	ctx context.Context,
	proposal permissions.ProposedCall,
) ([]byte, error) {
	return executor.broker.ExecuteProposalContent(ctx, proposal)
}

// TestExecutionAdapterWebSearchLive proves the authoritative execution adapter
// runs a real governed web search (real internet, no Codex) and commits the
// result through the dispatch/result gates.
func TestExecutionAdapterWebSearchLive(t *testing.T) {
	if os.Getenv("LOOM_LIVE_NET") != "1" {
		t.Skip("set LOOM_LIVE_NET=1 to run the live network E2E")
	}
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-web-live-e2e", nil)
	if err != nil {
		t.Fatal(err)
	}
	dispatch := &recordingDispatchGate{}
	resultGate := &resultCommitGateFixture{store: store}
	remote := newLiveRemoteToolExecutor(t)
	adapter := mustAdapter(
		t, store, projection, &recordingExecutor{},
		func() time.Time { return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC) },
	).WithRemoteToolExecutor(remote)
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-live-web-search",
		JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{
			Tool: permissions.ToolWebSearch, Path: "Loom governed handoff",
		},
		DispatchGate: dispatch, ResultCommitGate: resultGate,
	})
	if err != nil {
		if errors.Is(err, toolbroker.ErrToolFailed) {
			t.Skipf("external search endpoint blocked from this host: %v", err)
		}
		t.Fatalf("adapter live web search: %v", err)
	}
	if result.ErrorCode == "remote_tool_failed" {
		t.Skip("external search endpoint blocked from this host")
	}
	if result.Verdict != permissions.VerdictAllow || len(result.Content) == 0 {
		t.Fatalf("result = %#v", result)
	}
	if !dispatch.wasCommitted() || resultGate.calls != 1 ||
		!bytes.Equal(resultGate.contentCopy, result.Content) {
		t.Fatalf("gates not committed: dispatch=%t gateCalls=%d", dispatch.wasCommitted(), resultGate.calls)
	}
	if !strings.Contains(strings.ToLower(string(result.Content)), "http") {
		t.Fatalf("live content missing source URLs: %s", truncateExec(result.Content, 160))
	}
	t.Logf("adapter live web search ok: %d bytes committed (%s)", len(result.Content), truncateExec(result.Content, 120))
}

// TestExecutionAdapterWebFetchIsGoverned proves WebFetch is not treated as a
// read-only tool by default: the authoritative adapter returns an approval
// ask (governed remote capability), while WebSearch executes freely.
func TestExecutionAdapterWebFetchIsGoverned(t *testing.T) {
	if os.Getenv("LOOM_LIVE_NET") != "1" {
		t.Skip("set LOOM_LIVE_NET=1 to run the live network E2E")
	}
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-web-live-e2e-fetch", nil)
	if err != nil {
		t.Fatal(err)
	}
	dispatch := &recordingDispatchGate{}
	resultGate := &resultCommitGateFixture{store: store}
	remote := newLiveRemoteToolExecutor(t)
	adapter := mustAdapter(
		t, store, projection, &recordingExecutor{},
		func() time.Time { return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC) },
	).WithRemoteToolExecutor(remote)
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-live-web-fetch",
		JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{
			Tool: permissions.ToolWebFetch, Path: "https://example.com/",
		},
		DispatchGate: dispatch, ResultCommitGate: resultGate,
	})
	if err != nil {
		t.Fatalf("adapter live web fetch: %v", err)
	}
	if result.Verdict != permissions.VerdictAsk || result.ErrorCode != "" {
		t.Fatalf("web fetch not approval-gated: %#v", result)
	}
	t.Logf("adapter web fetch governed ok: verdict=%s reason=%s", result.Verdict, result.Denial.Reason)
}

func truncateExec(content []byte, maximum int) string {
	value := string(content)
	if len(value) > maximum {
		return value[:maximum] + "…"
	}
	return value
}
