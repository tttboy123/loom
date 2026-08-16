package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"unicode/utf8"

	"loom-pi-rebuild/internal/agentinbox"
)

var ErrInvalidAgentInput = errors.New("invalid Runtime Agent input")

const AgentAttemptRestartLoomOwnedCheckpointV1 = "loom-owned-checkpoint/v1"

type AgentInputCheckpoint struct {
	OutputDigest string
}

type AgentInputCheckpointPayload struct {
	Checkpoint AgentInputCheckpoint
	Content    []byte
}

func (payload *AgentInputCheckpointPayload) Close() {
	if payload == nil {
		return
	}
	for index := range payload.Content {
		payload.Content[index] = 0
	}
	payload.Content = nil
}

type AgentInput struct {
	Binding agentinbox.Binding
	Content []byte
}

type AgentInputBatch struct {
	TurnID       string
	TurnSequence int
	StepID       string
	StepSequence int
	Inputs       []AgentInput
}

type AgentInputSource interface {
	NextAgentInput(
		context.Context,
		AgentInputCheckpoint,
	) (AgentInputBatch, bool, error)
}

// DurableAgentInputSource atomically persists the preceding model output before
// it may consume and return the next Agent input.
type DurableAgentInputSource interface {
	AgentInputSource
	NextAgentInputFromDurableCheckpoint(
		context.Context,
		AgentInputCheckpointPayload,
	) (AgentInputBatch, bool, error)
}

type AgentInputConsumer interface {
	AcceptsAgentInputs() bool
}

// AgentAttemptRestartConformance is an explicit, content-free statement that
// a Runtime can restart from Loom-owned checkpoint state. It does not authorize
// dispatch and does not expose a Provider-native session handle.
type AgentAttemptRestartConformance interface {
	AgentAttemptRestartContract() string
	ValidateAgentAttemptRestartBinding(FrozenExecutionBinding) error
}

func NewAgentInputBatch(
	turnID string,
	turnSequence int,
	stepID string,
	stepSequence int,
	payloads []agentinbox.Payload,
) (AgentInputBatch, error) {
	defer closeRuntimeAgentInputPayloads(payloads)
	if turnID == "" || turnSequence < 1 || stepID == "" || stepSequence < 1 ||
		len(payloads) == 0 || len(payloads) > 64 {
		return AgentInputBatch{}, ErrInvalidAgentInput
	}
	sort.Slice(payloads, func(left, right int) bool {
		return payloads[left].Binding.OrderKey < payloads[right].Binding.OrderKey
	})
	batch := AgentInputBatch{
		TurnID: turnID, TurnSequence: turnSequence,
		StepID: stepID, StepSequence: stepSequence,
		Inputs: make([]AgentInput, 0, len(payloads)),
	}
	queueMode := payloads[0].Binding.Mode == agentinbox.ModeQueue
	if queueMode && (len(payloads) != 1 || stepSequence != 1) {
		return AgentInputBatch{}, ErrInvalidAgentInput
	}
	seen := make(map[string]struct{}, len(payloads))
	totalBytes := 0
	for index := range payloads {
		payload := &payloads[index]
		binding := payload.Binding
		digest := sha256.Sum256(payload.Content)
		totalBytes += len(payload.Content)
		if payload.Status != agentinbox.StatusConsumed || len(payload.Content) == 0 ||
			totalBytes > 64<<10 || !utf8.Valid(payload.Content) ||
			bytes.IndexByte(payload.Content, 0) >= 0 ||
			binding.ContentType != "text/plain" || binding.InputID == "" ||
			binding.PayloadID == "" || binding.OrderKey < 1 ||
			hex.EncodeToString(digest[:]) != binding.ContentDigest {
			batch.Close()
			return AgentInputBatch{}, ErrInvalidAgentInput
		}
		if _, duplicate := seen[binding.InputID]; duplicate {
			batch.Close()
			return AgentInputBatch{}, ErrInvalidAgentInput
		}
		seen[binding.InputID] = struct{}{}
		switch binding.ContextScope {
		case agentinbox.ScopeConversationShared, agentinbox.ScopeTeamShared,
			agentinbox.ScopeAgentPrivate:
			if binding.ScopeTargetID != "" {
				batch.Close()
				return AgentInputBatch{}, ErrInvalidAgentInput
			}
		case agentinbox.ScopeRoleRestricted, agentinbox.ScopeArtifact,
			agentinbox.ScopeSecretReference:
			if binding.ScopeTargetID == "" {
				batch.Close()
				return AgentInputBatch{}, ErrInvalidAgentInput
			}
		default:
			batch.Close()
			return AgentInputBatch{}, ErrInvalidAgentInput
		}
		if queueMode {
			if binding.Mode != agentinbox.ModeQueue || binding.TargetTurnID != turnID ||
				binding.TargetTurnSequence != turnSequence || binding.TargetStepID != "" ||
				binding.TargetStepSequence != 0 {
				batch.Close()
				return AgentInputBatch{}, ErrInvalidAgentInput
			}
		} else if (binding.Mode != agentinbox.ModeSteer && binding.Mode != agentinbox.ModeInject) ||
			binding.TargetTurnID != "" || binding.TargetTurnSequence != 0 ||
			binding.TargetStepID != stepID || binding.TargetStepSequence != stepSequence {
			batch.Close()
			return AgentInputBatch{}, ErrInvalidAgentInput
		}
		batch.Inputs = append(batch.Inputs, AgentInput{
			Binding: binding,
			Content: bytes.Clone(payload.Content),
		})
	}
	return batch, nil
}

func (batch *AgentInputBatch) Close() {
	if batch == nil {
		return
	}
	for index := range batch.Inputs {
		for offset := range batch.Inputs[index].Content {
			batch.Inputs[index].Content[offset] = 0
		}
		batch.Inputs[index].Content = nil
	}
	batch.Inputs = nil
}

// RenderAgentInput returns owned mutable bytes; callers must clear them after use.
func RenderAgentInput(batch *AgentInputBatch) ([]byte, error) {
	if batch == nil || batch.TurnID == "" || batch.TurnSequence < 1 ||
		batch.StepID == "" || batch.StepSequence < 1 || len(batch.Inputs) == 0 {
		return nil, ErrInvalidAgentInput
	}
	var rendered bytes.Buffer
	defer func() {
		for index := range rendered.Bytes() {
			rendered.Bytes()[index] = 0
		}
	}()
	for index := range batch.Inputs {
		input := &batch.Inputs[index]
		if len(input.Content) == 0 || len(input.Content) > 64<<10 ||
			!utf8.Valid(input.Content) || bytes.IndexByte(input.Content, 0) >= 0 {
			return nil, ErrInvalidAgentInput
		}
		switch input.Binding.Mode {
		case agentinbox.ModeQueue, agentinbox.ModeSteer, agentinbox.ModeInject:
		default:
			return nil, ErrInvalidAgentInput
		}
		switch input.Binding.ContextScope {
		case agentinbox.ScopeConversationShared, agentinbox.ScopeTeamShared,
			agentinbox.ScopeAgentPrivate:
			if input.Binding.ScopeTargetID != "" {
				return nil, ErrInvalidAgentInput
			}
		case agentinbox.ScopeRoleRestricted, agentinbox.ScopeArtifact,
			agentinbox.ScopeSecretReference:
			if input.Binding.ScopeTargetID == "" ||
				len(input.Binding.ScopeTargetID) > 512 ||
				!utf8.ValidString(input.Binding.ScopeTargetID) ||
				bytes.IndexByte([]byte(input.Binding.ScopeTargetID), 0) >= 0 {
				return nil, ErrInvalidAgentInput
			}
		default:
			return nil, ErrInvalidAgentInput
		}
		if index != 0 {
			rendered.WriteString("\n\n")
		}
		rendered.WriteString("Loom ")
		rendered.WriteString(string(input.Binding.Mode))
		rendered.WriteString(" input (scope=")
		rendered.WriteString(string(input.Binding.ContextScope))
		if input.Binding.ScopeTargetID != "" {
			rendered.WriteString(", target=")
			rendered.WriteString(input.Binding.ScopeTargetID)
		}
		rendered.WriteString("):\n")
		rendered.Write(input.Content)
		if rendered.Len() > 64<<10 {
			return nil, ErrInvalidAgentInput
		}
	}
	if !utf8.Valid(rendered.Bytes()) || bytes.IndexByte(rendered.Bytes(), 0) >= 0 {
		return nil, ErrInvalidAgentInput
	}
	return bytes.Clone(rendered.Bytes()), nil
}

func ValidAgentInputCheckpoint(checkpoint AgentInputCheckpoint) bool {
	if len(checkpoint.OutputDigest) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(checkpoint.OutputDigest)
	return err == nil
}

func ValidAgentInputCheckpointPayload(payload AgentInputCheckpointPayload) bool {
	if !ValidAgentInputCheckpoint(payload.Checkpoint) || len(payload.Content) == 0 ||
		len(payload.Content) > 64<<10 || !utf8.Valid(payload.Content) ||
		bytes.IndexByte(payload.Content, 0) >= 0 {
		return false
	}
	digest := sha256.Sum256(payload.Content)
	return hex.EncodeToString(digest[:]) == payload.Checkpoint.OutputDigest
}

func closeRuntimeAgentInputPayloads(payloads []agentinbox.Payload) {
	for index := range payloads {
		payloads[index].Close()
	}
}
