package runtime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"loom-pi-rebuild/internal/agentinbox"
)

func TestNewAgentInputBatchTakesOrderedOwnershipAndZeroizes(t *testing.T) {
	first := runtimeAgentInputPayload("input-steer", 1, agentinbox.ModeSteer, "first private input")
	second := runtimeAgentInputPayload("input-inject", 2, agentinbox.ModeInject, "second private input")
	batch, err := NewAgentInputBatch(
		"turn-1", 1, "step-2", 2,
		[]agentinbox.Payload{first, second},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Inputs) != 2 || batch.Inputs[0].Binding.InputID != "input-steer" ||
		batch.Inputs[1].Binding.InputID != "input-inject" ||
		!bytes.Equal(batch.Inputs[0].Content, []byte("first private input")) ||
		!bytes.Equal(batch.Inputs[1].Content, []byte("second private input")) {
		t.Fatalf("batch = %#v", batch)
	}
	firstContent := batch.Inputs[0].Content
	secondContent := batch.Inputs[1].Content
	batch.Close()
	if !runtimeAgentInputAllZero(firstContent) || !runtimeAgentInputAllZero(secondContent) ||
		batch.Inputs != nil {
		t.Fatal("Agent input batch did not zeroize owned plaintext")
	}
}

func TestNewAgentInputBatchRejectsDigestAndTargetSubstitution(t *testing.T) {
	payload := runtimeAgentInputPayload("input-steer", 1, agentinbox.ModeSteer, "private input")
	payload.Binding.ContentDigest = hex.EncodeToString(bytes.Repeat([]byte{0x44}, 32))
	if batch, err := NewAgentInputBatch(
		"turn-1", 1, "step-2", 2, []agentinbox.Payload{payload},
	); err == nil {
		batch.Close()
		t.Fatal("content digest substitution was accepted")
	}
	payload = runtimeAgentInputPayload("input-steer", 1, agentinbox.ModeSteer, "private input")
	if batch, err := NewAgentInputBatch(
		"turn-1", 1, "step-other", 2, []agentinbox.Payload{payload},
	); err == nil {
		batch.Close()
		t.Fatal("target Step substitution was accepted")
	}
}

func TestRenderAgentInputUsesOneDeterministicMutableEnvelope(t *testing.T) {
	first := runtimeAgentInputPayload(
		"input-steer", 1, agentinbox.ModeSteer, "first private input",
	)
	second := runtimeAgentInputPayload(
		"input-inject", 2, agentinbox.ModeInject, "second private input",
	)
	batch, err := NewAgentInputBatch(
		"turn-1", 1, "step-2", 2, []agentinbox.Payload{second, first},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer batch.Close()
	rendered, err := RenderAgentInput(&batch)
	if err != nil {
		t.Fatal(err)
	}
	want := "Loom steer input (scope=agent_private):\nfirst private input\n\n" +
		"Loom inject input (scope=agent_private):\nsecond private input"
	if string(rendered) != want {
		t.Fatalf("rendered Agent input = %q", rendered)
	}
	for index := range rendered {
		rendered[index] = 0
	}
	if !runtimeAgentInputAllZero(rendered) {
		t.Fatal("rendered Agent input is not caller-zeroizable")
	}
}

func TestAgentInputCheckpointPayloadValidatesDigestAndZeroizes(t *testing.T) {
	content := []byte("private model checkpoint")
	digest := sha256.Sum256(content)
	payload := AgentInputCheckpointPayload{
		Checkpoint: AgentInputCheckpoint{OutputDigest: hex.EncodeToString(digest[:])},
		Content:    bytes.Clone(content),
	}
	if !ValidAgentInputCheckpointPayload(payload) {
		t.Fatal("valid checkpoint payload was rejected")
	}
	owned := payload.Content
	payload.Close()
	if !runtimeAgentInputAllZero(owned) || payload.Content != nil {
		t.Fatal("checkpoint payload did not zeroize owned plaintext")
	}
	changed := AgentInputCheckpointPayload{
		Checkpoint: AgentInputCheckpoint{OutputDigest: hex.EncodeToString(digest[:])},
		Content:    []byte("substituted model checkpoint"),
	}
	if ValidAgentInputCheckpointPayload(changed) {
		t.Fatal("checkpoint digest substitution was accepted")
	}
}

func runtimeAgentInputPayload(
	inputID string,
	order int64,
	mode agentinbox.Mode,
	content string,
) agentinbox.Payload {
	digest := sha256.Sum256([]byte(content))
	return agentinbox.Payload{
		Binding: agentinbox.Binding{
			PayloadID: "payload-" + inputID, InputID: inputID, Mode: mode,
			ContextScope:   agentinbox.ScopeAgentPrivate,
			ConversationID: "conversation-1", SegmentID: "segment-1",
			AgentInstanceID: "agent-1", WorkItemID: "work-1", RunID: "run-1",
			ClaimGeneration: 1, RuntimeInstanceID: "runtime-1",
			ExecutionBindingDigest: hex.EncodeToString(bytes.Repeat([]byte{0x11}, 32)),
			CapsuleDigest:          hex.EncodeToString(bytes.Repeat([]byte{0x22}, 32)),
			OrderKey:               order, TargetStepID: "step-2", TargetStepSequence: 2,
			ContentType: "text/plain", ContentDigest: hex.EncodeToString(digest[:]),
		},
		Status: agentinbox.StatusConsumed, Content: []byte(content),
	}
}

func runtimeAgentInputAllZero(content []byte) bool {
	for _, value := range content {
		if value != 0 {
			return false
		}
	}
	return true
}
