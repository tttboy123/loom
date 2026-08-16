//go:build !windows

package evidence

import (
	"bytes"
	"context"
	"testing"

	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func TestReadAggregationOutputDisclosesOnlyAuthorizedOutputEvents(t *testing.T) {
	store, err := NewStore(testAttemptEvidenceRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	input := testAttemptCaptureInput()
	if err := store.BeginAttemptCapture(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	frames := []bridgev1.Frame{
		testAttemptCaptureFrame(t, input, 2, bridgev1.MessageAck,
			[]byte(`{"message_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}`)),
		testAttemptCaptureFrame(t, input, 3, bridgev1.MessageEvent,
			[]byte(`{"delta":"authorized output"}`)),
		testAttemptCaptureFrame(t, input, 4, bridgev1.MessageEvidence,
			[]byte(`{"private_evidence":"must-not-disclose"}`)),
		testAttemptCaptureFrame(t, input, 5, bridgev1.MessageResult,
			[]byte(`{"status":"succeeded","reason":""}`)),
	}
	for _, frame := range frames {
		line, encodeErr := bridgev1.EncodeLine(frame)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		if err := store.AppendAttemptFrame(context.Background(), input.EvidenceID, line); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := store.FinalizeAttemptCapture(context.Background(), input.EvidenceID,
		AttemptTerminal{Status: "succeeded", Reason: ""})
	if err != nil {
		t.Fatal(err)
	}
	output, err := store.ReadAggregationOutput(context.Background(), receipt, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	content := output.Content()
	if output.Binding() != input || output.EvidenceDigest() != receipt.Digest() ||
		output.OutputSummaryDigest() != receipt.OutputSummary().Digest() ||
		!bytes.Contains(content, []byte(`"authorized output"`)) ||
		bytes.Contains(content, []byte("must-not-disclose")) ||
		bytes.Contains(content, []byte(`"status"`)) {
		t.Fatalf("aggregation output authority=%#v content=%s", output.Binding(), content)
	}
	output.Close()
	if output.Content() != nil {
		t.Fatalf("closed aggregation output = %q", output.Content())
	}
}

func TestReadAggregationOutputRejectsNonSucceededAndReceiptSubstitution(t *testing.T) {
	store, err := NewStore(testAttemptEvidenceRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	input := testAttemptCaptureInput()
	if err := store.BeginAttemptCapture(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	for _, line := range testAttemptCaptureLines(t, input, "failed", "provider_failed") {
		if err := store.AppendAttemptFrame(context.Background(), input.EvidenceID, line); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := store.FinalizeAttemptCapture(context.Background(), input.EvidenceID,
		AttemptTerminal{Status: "failed", Reason: "provider_failed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAggregationOutput(context.Background(), receipt, 1<<20); err == nil {
		t.Fatal("failed Attempt was disclosed to aggregation")
	}
	changed := receipt
	changed.summary.digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := store.ReadAggregationOutput(context.Background(), changed, 1<<20); err == nil {
		t.Fatal("substituted output summary was accepted")
	}
}
