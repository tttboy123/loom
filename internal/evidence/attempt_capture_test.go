//go:build !windows

package evidence

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

var attemptCaptureNow = time.Date(2026, 7, 26, 13, 14, 15, 0, time.UTC)

func TestAttemptCapturePersistsAuthorizedFramesFinalizesAndReopens(t *testing.T) {
	root := testAttemptEvidenceRoot(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	input := testAttemptCaptureInput()
	if err := store.BeginAttemptCapture(
		context.Background(),
		input,
	); err != nil {
		t.Fatal(err)
	}
	state, exists, err := store.AttemptCapture(
		context.Background(),
		input.EvidenceID,
	)
	if err != nil || !exists ||
		state.Binding() != input ||
		state.FrameCount() != 0 ||
		state.ResultObserved() {
		t.Fatalf("initial capture = %#v, %v, %v", state, exists, err)
	}
	lines := testAttemptCaptureLines(t, input, "succeeded", "")
	for _, line := range lines {
		if err := store.AppendAttemptFrame(
			context.Background(),
			input.EvidenceID,
			line,
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.AppendAttemptFrame(
		context.Background(),
		input.EvidenceID,
		lines[1],
	); err != nil {
		t.Fatalf("exact duplicate append error = %v", err)
	}
	state, exists, err = store.AttemptCapture(
		context.Background(),
		input.EvidenceID,
	)
	if err != nil || !exists ||
		state.FrameCount() != 3 ||
		!state.ResultObserved() {
		t.Fatalf("complete capture = %#v, %v, %v", state, exists, err)
	}
	terminal := AttemptTerminal{Status: "succeeded"}
	receipt, err := store.FinalizeAttemptCapture(
		context.Background(),
		input.EvidenceID,
		terminal,
	)
	if err != nil ||
		receipt.EvidenceID() != input.EvidenceID ||
		len(receipt.Digest()) != 64 {
		t.Fatalf("FinalizeAttemptCapture() = %#v, %v", receipt, err)
	}
	exact, err := store.FinalizeAttemptCapture(
		context.Background(),
		input.EvidenceID,
		terminal,
	)
	if err != nil || exact.Digest() != receipt.Digest() {
		t.Fatalf("exact Finalize retry = %#v, %v", exact, err)
	}
	content, err := os.ReadFile(artifactPathForRoot(root, receipt.Digest()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte(input.EvidenceID)) ||
		!bytes.Contains(content, []byte(`"child_result_observed":true`)) ||
		!bytes.Contains(content, []byte(`"status":"succeeded"`)) {
		t.Fatalf("canonical attempt artifact = %s", content)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	reopenedReceipt, exists, err := reopened.AttemptReceipt(
		context.Background(),
		input.EvidenceID,
	)
	if err != nil || !exists ||
		reopenedReceipt.Digest() != receipt.Digest() {
		t.Fatalf("reopened receipt = %#v, %v, %v", reopenedReceipt, exists, err)
	}
}

func TestAttemptCaptureRebindIsEmptyOnlyAndExactIdempotent(t *testing.T) {
	store, err := NewStore(testAttemptEvidenceRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	input := testAttemptCaptureInput()
	if err := store.BeginAttemptCapture(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	rebound := input
	rebound.ClaimID = "22222222-2222-4222-8222-222222222222"
	rebound.ClaimGeneration = 2
	if err := store.RebindAttemptCapture(
		context.Background(),
		rebound,
	); err != nil {
		t.Fatal(err)
	}
	if err := store.RebindAttemptCapture(
		context.Background(),
		rebound,
	); err != nil {
		t.Fatalf("exact Rebind retry error = %v", err)
	}
	state, exists, err := store.AttemptCapture(
		context.Background(),
		input.EvidenceID,
	)
	if err != nil || !exists || state.Binding() != rebound {
		t.Fatalf("rebound capture = %#v, %v, %v", state, exists, err)
	}
	if err := store.AppendAttemptFrame(
		context.Background(),
		input.EvidenceID,
		testAttemptCaptureLines(t, rebound, "succeeded", "")[0],
	); err != nil {
		t.Fatal(err)
	}
	third := rebound
	third.ClaimID = "33333333-3333-4333-8333-333333333333"
	third.ClaimGeneration = 3
	if err := store.RebindAttemptCapture(
		context.Background(),
		third,
	); !errors.Is(err, ErrAttemptCaptureConflict) {
		t.Fatalf("nonempty Rebind error = %v", err)
	}
}

func TestAttemptCaptureRejectsDivergenceTokensAndInvalidTerminal(t *testing.T) {
	store, err := NewStore(testAttemptEvidenceRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	input := testAttemptCaptureInput()
	if err := store.BeginAttemptCapture(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	lines := testAttemptCaptureLines(t, input, "succeeded", "")
	if err := store.AppendAttemptFrame(
		context.Background(),
		input.EvidenceID,
		lines[0],
	); err != nil {
		t.Fatal(err)
	}
	divergent := testAttemptCaptureFrame(
		t,
		input,
		2,
		bridgev1.MessageAck,
		[]byte(`{"message_id":"99999999-9999-4999-8999-999999999999"}`),
	)
	divergentLine, err := bridgev1.EncodeLine(divergent)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendAttemptFrame(
		context.Background(),
		input.EvidenceID,
		divergentLine,
	); !errors.Is(err, ErrAttemptCaptureConflict) {
		t.Fatalf("divergent duplicate error = %v", err)
	}
	tokenFrame := testAttemptCaptureFrame(
		t,
		input,
		3,
		bridgev1.MessageEvent,
		[]byte(`{"delta":"loom_grant_v1.forbidden"}`),
	)
	tokenLine, err := bridgev1.EncodeLine(tokenFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendAttemptFrame(
		context.Background(),
		input.EvidenceID,
		tokenLine,
	); !errors.Is(err, ErrInvalidAttemptCapture) {
		t.Fatalf("raw token marker error = %v", err)
	}
	if _, err := store.FinalizeAttemptCapture(
		context.Background(),
		input.EvidenceID,
		AttemptTerminal{Status: "succeeded"},
	); !errors.Is(err, ErrAttemptCaptureIncomplete) {
		t.Fatalf("success without result error = %v", err)
	}
	if _, err := store.FinalizeAttemptCapture(
		context.Background(),
		input.EvidenceID,
		AttemptTerminal{Status: "failed", Reason: "adapter_failed"},
	); err != nil {
		t.Fatalf("Supervisor terminal without result error = %v", err)
	}
}

func TestAttemptCaptureRejectsSymlinkStateDirectory(t *testing.T) {
	root := testAttemptEvidenceRoot(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, "attempts")); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginAttemptCapture(
		context.Background(),
		testAttemptCaptureInput(),
	); !errors.Is(err, ErrStateSymlink) {
		t.Fatalf("symlink attempt state error = %v", err)
	}
}

func testAttemptCaptureInput() AttemptCaptureInput {
	return AttemptCaptureInput{
		EvidenceID:        "team-evidence-1234567890abcdef1234567890abcdef",
		TeamInstanceID:    "team-1",
		PlanDigest:        strings.Repeat("a", 64),
		LogicalNodeID:     "main",
		AttemptNumber:     1,
		WorkItemID:        "team-work-1234567890abcdef1234567890abcdef",
		RunID:             "team-run-1234567890abcdef1234567890abcdef",
		ClaimID:           "11111111-1111-4111-8111-111111111111",
		ClaimGeneration:   1,
		RuntimeInstanceID: "runtime-a",
		AgentInstanceID:   "agent-main",
	}
}

func testAttemptEvidenceRoot(t testing.TB) string {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(parent, "evidence")
}

func testAttemptCaptureLines(
	t testing.TB,
	input AttemptCaptureInput,
	status string,
	reason string,
) [][]byte {
	t.Helper()
	frames := []bridgev1.Frame{
		testAttemptCaptureFrame(
			t,
			input,
			2,
			bridgev1.MessageAck,
			[]byte(`{"message_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}`),
		),
		testAttemptCaptureFrame(
			t,
			input,
			3,
			bridgev1.MessageEvent,
			[]byte(`{"delta":"authorized output"}`),
		),
		testAttemptCaptureFrame(
			t,
			input,
			4,
			bridgev1.MessageResult,
			[]byte(`{"status":"`+status+`","reason":"`+reason+`"}`),
		),
	}
	lines := make([][]byte, len(frames))
	for index, frame := range frames {
		line, err := bridgev1.EncodeLine(frame)
		if err != nil {
			t.Fatal(err)
		}
		lines[index] = line
	}
	return lines
}

func testAttemptCaptureFrame(
	t testing.TB,
	input AttemptCaptureInput,
	sequence int64,
	messageType bridgev1.MessageType,
	payload []byte,
) bridgev1.Frame {
	t.Helper()
	messageIDs := map[int64]string{
		2: "22222222-2222-4222-8222-222222222222",
		3: "33333333-3333-4333-8333-333333333333",
		4: "44444444-4444-4444-8444-444444444444",
	}
	frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             messageIDs[sequence],
		CorrelationID:         "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		WorkItemID:            input.WorkItemID,
		RunID:                 input.RunID,
		ClaimGeneration:       input.ClaimGeneration,
		RuntimeInstanceID:     input.RuntimeInstanceID,
		SenderAgentInstanceID: input.AgentInstanceID,
		Sequence:              sequence,
		Type:                  messageType,
		EmittedAt:             attemptCaptureNow,
		Payload:               payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	return frame
}
