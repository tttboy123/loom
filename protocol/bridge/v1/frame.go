package bridgev1

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const ProtocolVersion = "loom.bridge.v1"

const (
	MaxLineBytes      = 1 << 20
	MaxPayloadBytes   = 768 << 10
	MaxOpaqueIDBytes  = 128
	MaxBufferedFrames = 1024
	MaxBufferedBytes  = 8 << 20
)

type MessageType string

const (
	MessageDispatch  MessageType = "dispatch"
	MessageEvent     MessageType = "event"
	MessageEvidence  MessageType = "evidence"
	MessageResult    MessageType = "result"
	MessageCancel    MessageType = "cancel"
	MessageAck       MessageType = "ack"
	MessageHeartbeat MessageType = "heartbeat"
)

var (
	ErrInvalidBridgeV1Frame         = errors.New("invalid loom.bridge.v1 frame")
	ErrInvalidBridgeV1Line          = errors.New("invalid loom.bridge.v1 line")
	ErrBridgeV1LineTooLarge         = errors.New("loom.bridge.v1 line too large")
	ErrUnsupportedBridgeProtocol    = errors.New("unsupported bridge protocol")
	ErrUnknownBridgeMessageType     = errors.New("unknown bridge message type")
	ErrDuplicateBridgeJSONKey       = errors.New("duplicate bridge JSON key")
	ErrInvalidBridgeV1RunStream     = errors.New("invalid loom.bridge.v1 run stream")
	ErrBridgeV1RunStreamBufferLimit = errors.New("loom.bridge.v1 run stream buffer limit")
	errTrailingJSONValue            = errors.New("trailing JSON value")
)

type FrameInput struct {
	MessageID             string
	CorrelationID         string
	WorkItemID            string
	RunID                 string
	ClaimGeneration       int64
	RuntimeInstanceID     string
	SenderAgentInstanceID string
	Sequence              int64
	Type                  MessageType
	EmittedAt             time.Time
	Payload               []byte
}

type Frame struct {
	messageID             string
	correlationID         string
	workItemID            string
	runID                 string
	claimGeneration       int64
	runtimeInstanceID     string
	senderAgentInstanceID string
	sequence              int64
	messageType           MessageType
	emittedAt             time.Time
	payload               []byte
}

type RunStreamBinding struct {
	WorkItemID            string
	RunID                 string
	ClaimGeneration       int64
	RuntimeInstanceID     string
	SenderAgentInstanceID string
}

type BoundRunStream struct {
	binding        RunStreamBinding
	frames         []Frame
	lastSequence   int64
	terminalResult bool
	bufferedBytes  int
}

type wireFrame struct {
	ProtocolVersion       string          `json:"protocol_version"`
	MessageID             string          `json:"message_id"`
	CorrelationID         string          `json:"correlation_id"`
	WorkItemID            string          `json:"work_item_id"`
	RunID                 string          `json:"run_id"`
	ClaimGeneration       int64           `json:"claim_generation"`
	RuntimeInstanceID     string          `json:"runtime_instance_id"`
	SenderAgentInstanceID string          `json:"sender_agent_instance_id"`
	Sequence              int64           `json:"seq"`
	Type                  MessageType     `json:"type"`
	EmittedAt             string          `json:"emitted_at"`
	Payload               json.RawMessage `json:"payload"`
}

type decodedWireFrame struct {
	ProtocolVersion       *string         `json:"protocol_version"`
	MessageID             *string         `json:"message_id"`
	CorrelationID         *string         `json:"correlation_id"`
	WorkItemID            *string         `json:"work_item_id"`
	RunID                 *string         `json:"run_id"`
	ClaimGeneration       *int64          `json:"claim_generation"`
	RuntimeInstanceID     *string         `json:"runtime_instance_id"`
	SenderAgentInstanceID *string         `json:"sender_agent_instance_id"`
	Sequence              *int64          `json:"seq"`
	Type                  *MessageType    `json:"type"`
	EmittedAt             *string         `json:"emitted_at"`
	Payload               json.RawMessage `json:"payload"`
}

func NewFrame(input FrameInput) (Frame, error) {
	if !validCanonicalUUID(input.MessageID) ||
		!validCanonicalUUID(input.CorrelationID) ||
		!validOpaqueID(input.WorkItemID) ||
		!validOpaqueID(input.RunID) ||
		input.ClaimGeneration <= 0 ||
		!validOpaqueID(input.RuntimeInstanceID) ||
		!validOpaqueID(input.SenderAgentInstanceID) ||
		input.Sequence <= 0 ||
		input.EmittedAt.IsZero() ||
		input.EmittedAt.Location() != time.UTC {
		return Frame{}, ErrInvalidBridgeV1Frame
	}
	if !validMessageType(input.Type) {
		return Frame{}, ErrUnknownBridgeMessageType
	}

	payload, err := validatedPayload(input.Payload)
	if err != nil {
		return Frame{}, err
	}

	frame := Frame{
		messageID:             input.MessageID,
		correlationID:         input.CorrelationID,
		workItemID:            input.WorkItemID,
		runID:                 input.RunID,
		claimGeneration:       input.ClaimGeneration,
		runtimeInstanceID:     input.RuntimeInstanceID,
		senderAgentInstanceID: input.SenderAgentInstanceID,
		sequence:              input.Sequence,
		messageType:           input.Type,
		emittedAt:             input.EmittedAt,
		payload:               payload,
	}
	line, err := encodeValidatedFrame(frame)
	if err != nil {
		return Frame{}, err
	}
	if len(line) > MaxLineBytes {
		return Frame{}, ErrBridgeV1LineTooLarge
	}
	return frame, nil
}

func DecodeLine(line []byte) (Frame, error) {
	if len(line) > MaxLineBytes {
		return Frame{}, ErrBridgeV1LineTooLarge
	}
	if len(line) < 2 ||
		line[len(line)-1] != '\n' ||
		bytes.Count(line, []byte{'\n'}) != 1 ||
		bytes.IndexByte(line, '\r') >= 0 ||
		!utf8.Valid(line) {
		return Frame{}, ErrInvalidBridgeV1Line
	}

	body := line[:len(line)-1]
	if len(body) > 0 && (body[0] == ' ' || body[0] == '\t' ||
		body[len(body)-1] == ' ' || body[len(body)-1] == '\t') {
		return Frame{}, ErrInvalidBridgeV1Line
	}
	if len(body) < 2 || body[0] != '{' || body[len(body)-1] != '}' {
		return Frame{}, ErrInvalidBridgeV1Frame
	}
	if hasTrailingJSONValue(body) {
		return Frame{}, ErrInvalidBridgeV1Line
	}
	if err := rejectDuplicateJSONKeys(body); err != nil {
		if errors.Is(err, errTrailingJSONValue) {
			return Frame{}, ErrInvalidBridgeV1Line
		}
		return Frame{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var decoded decodedWireFrame
	if err := decoder.Decode(&decoded); err != nil {
		return Frame{}, fmt.Errorf("%w: %v", ErrInvalidBridgeV1Frame, err)
	}
	if err := requireDecoderEOF(decoder); err != nil {
		return Frame{}, fmt.Errorf("%w: %v", ErrInvalidBridgeV1Frame, err)
	}
	if decoded.ProtocolVersion == nil ||
		decoded.MessageID == nil ||
		decoded.CorrelationID == nil ||
		decoded.WorkItemID == nil ||
		decoded.RunID == nil ||
		decoded.ClaimGeneration == nil ||
		decoded.RuntimeInstanceID == nil ||
		decoded.SenderAgentInstanceID == nil ||
		decoded.Sequence == nil ||
		decoded.Type == nil ||
		decoded.EmittedAt == nil ||
		decoded.Payload == nil {
		return Frame{}, ErrInvalidBridgeV1Frame
	}
	if *decoded.ProtocolVersion != ProtocolVersion {
		return Frame{}, ErrUnsupportedBridgeProtocol
	}
	if !strings.HasSuffix(*decoded.EmittedAt, "Z") {
		return Frame{}, ErrInvalidBridgeV1Frame
	}
	emittedAt, err := time.Parse(time.RFC3339Nano, *decoded.EmittedAt)
	if err != nil || emittedAt.IsZero() || emittedAt.Location() != time.UTC {
		return Frame{}, ErrInvalidBridgeV1Frame
	}

	return NewFrame(FrameInput{
		MessageID:             *decoded.MessageID,
		CorrelationID:         *decoded.CorrelationID,
		WorkItemID:            *decoded.WorkItemID,
		RunID:                 *decoded.RunID,
		ClaimGeneration:       *decoded.ClaimGeneration,
		RuntimeInstanceID:     *decoded.RuntimeInstanceID,
		SenderAgentInstanceID: *decoded.SenderAgentInstanceID,
		Sequence:              *decoded.Sequence,
		Type:                  *decoded.Type,
		EmittedAt:             emittedAt,
		Payload:               decoded.Payload,
	})
}

func EncodeLine(frame Frame) ([]byte, error) {
	validated, err := NewFrame(FrameInput{
		MessageID:             frame.messageID,
		CorrelationID:         frame.correlationID,
		WorkItemID:            frame.workItemID,
		RunID:                 frame.runID,
		ClaimGeneration:       frame.claimGeneration,
		RuntimeInstanceID:     frame.runtimeInstanceID,
		SenderAgentInstanceID: frame.senderAgentInstanceID,
		Sequence:              frame.sequence,
		Type:                  frame.messageType,
		EmittedAt:             frame.emittedAt,
		Payload:               frame.payload,
	})
	if err != nil {
		return nil, err
	}
	return encodeValidatedFrame(validated)
}

func (Frame) ProtocolVersion() string {
	return ProtocolVersion
}

func (frame Frame) MessageID() string {
	return frame.messageID
}

func (frame Frame) CorrelationID() string {
	return frame.correlationID
}

func (frame Frame) WorkItemID() string {
	return frame.workItemID
}

func (frame Frame) RunID() string {
	return frame.runID
}

func (frame Frame) ClaimGeneration() int64 {
	return frame.claimGeneration
}

func (frame Frame) RuntimeInstanceID() string {
	return frame.runtimeInstanceID
}

func (frame Frame) SenderAgentInstanceID() string {
	return frame.senderAgentInstanceID
}

func (frame Frame) Sequence() int64 {
	return frame.sequence
}

func (frame Frame) Type() MessageType {
	return frame.messageType
}

func (frame Frame) EmittedAt() time.Time {
	return frame.emittedAt
}

func (frame Frame) Payload() []byte {
	return bytes.Clone(frame.payload)
}

func NewBoundRunStream(binding RunStreamBinding) (BoundRunStream, error) {
	if !validRunStreamBinding(binding) {
		return BoundRunStream{}, ErrInvalidBridgeV1RunStream
	}
	return BoundRunStream{binding: binding}, nil
}

func AdvanceBoundRunStream(stream BoundRunStream, frame Frame) (BoundRunStream, error) {
	if !validRunStreamBinding(stream.binding) ||
		(len(stream.frames) == 0 && (stream.lastSequence != 0 ||
			stream.terminalResult || stream.bufferedBytes != 0)) ||
		(len(stream.frames) > 0 && stream.lastSequence <= 0) {
		return BoundRunStream{}, ErrInvalidBridgeV1RunStream
	}

	line, err := EncodeLine(frame)
	if err != nil {
		return BoundRunStream{}, ErrInvalidBridgeV1RunStream
	}
	if frame.workItemID != stream.binding.WorkItemID ||
		frame.runID != stream.binding.RunID ||
		frame.claimGeneration != stream.binding.ClaimGeneration ||
		frame.runtimeInstanceID != stream.binding.RuntimeInstanceID ||
		frame.senderAgentInstanceID != stream.binding.SenderAgentInstanceID ||
		frame.sequence <= stream.lastSequence {
		return BoundRunStream{}, ErrInvalidBridgeV1RunStream
	}
	for _, existing := range stream.frames {
		if existing.messageID == frame.messageID {
			return BoundRunStream{}, ErrInvalidBridgeV1RunStream
		}
	}
	if frame.messageType == MessageResult && stream.terminalResult {
		return BoundRunStream{}, ErrInvalidBridgeV1RunStream
	}
	if len(stream.frames) >= MaxBufferedFrames ||
		len(line) > MaxBufferedBytes-stream.bufferedBytes {
		return BoundRunStream{}, ErrBridgeV1RunStreamBufferLimit
	}

	frames := make([]Frame, len(stream.frames)+1)
	copy(frames, stream.frames)
	frames[len(stream.frames)] = cloneFrame(frame)
	return BoundRunStream{
		binding:        stream.binding,
		frames:         frames,
		lastSequence:   frame.sequence,
		terminalResult: stream.terminalResult || frame.messageType == MessageResult,
		bufferedBytes:  stream.bufferedBytes + len(line),
	}, nil
}

func (stream BoundRunStream) Binding() RunStreamBinding {
	return stream.binding
}

func (stream BoundRunStream) Frames() []Frame {
	if stream.frames == nil {
		return nil
	}
	frames := make([]Frame, len(stream.frames))
	for index := range stream.frames {
		frames[index] = cloneFrame(stream.frames[index])
	}
	return frames
}

func (stream BoundRunStream) LastSequence() int64 {
	return stream.lastSequence
}

func (stream BoundRunStream) TerminalResultSeen() bool {
	return stream.terminalResult
}

func (stream BoundRunStream) BufferedBytes() int {
	return stream.bufferedBytes
}

func validCanonicalUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index := 0; index < len(value); index++ {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if value[index] != '-' {
				return false
			}
			continue
		}
		if !((value[index] >= '0' && value[index] <= '9') ||
			(value[index] >= 'a' && value[index] <= 'f')) {
			return false
		}
	}
	return true
}

func validOpaqueID(value string) bool {
	if value == "" ||
		len(value) > MaxOpaqueIDBytes ||
		!utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validMessageType(messageType MessageType) bool {
	switch messageType {
	case MessageDispatch,
		MessageEvent,
		MessageEvidence,
		MessageResult,
		MessageCancel,
		MessageAck,
		MessageHeartbeat:
		return true
	default:
		return false
	}
}

func validatedPayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 || !utf8.Valid(payload) {
		return nil, ErrInvalidBridgeV1Frame
	}
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return nil, err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, payload); err != nil {
		return nil, ErrInvalidBridgeV1Frame
	}
	validated := compact.Bytes()
	if len(validated) < 2 ||
		validated[0] != '{' ||
		validated[len(validated)-1] != '}' ||
		len(validated) > MaxPayloadBytes {
		return nil, ErrInvalidBridgeV1Frame
	}
	return bytes.Clone(validated), nil
}

func encodeValidatedFrame(frame Frame) ([]byte, error) {
	body, err := json.Marshal(wireFrame{
		ProtocolVersion:       ProtocolVersion,
		MessageID:             frame.messageID,
		CorrelationID:         frame.correlationID,
		WorkItemID:            frame.workItemID,
		RunID:                 frame.runID,
		ClaimGeneration:       frame.claimGeneration,
		RuntimeInstanceID:     frame.runtimeInstanceID,
		SenderAgentInstanceID: frame.senderAgentInstanceID,
		Sequence:              frame.sequence,
		Type:                  frame.messageType,
		EmittedAt:             frame.emittedAt.Format(time.RFC3339Nano),
		Payload:               json.RawMessage(frame.payload),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBridgeV1Frame, err)
	}
	line := append(body, '\n')
	if len(line) > MaxLineBytes {
		return nil, ErrBridgeV1LineTooLarge
	}
	return line, nil
}

func cloneFrame(frame Frame) Frame {
	frame.payload = bytes.Clone(frame.payload)
	return frame
}

func validRunStreamBinding(binding RunStreamBinding) bool {
	return validOpaqueID(binding.WorkItemID) &&
		validOpaqueID(binding.RunID) &&
		binding.ClaimGeneration > 0 &&
		validOpaqueID(binding.RuntimeInstanceID) &&
		validOpaqueID(binding.SenderAgentInstanceID)
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if err := requireDecoderEOF(decoder); err != nil {
		return ErrInvalidBridgeV1Frame
	}
	return nil
}

func hasTrailingJSONValue(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var first json.RawMessage
	if err := decoder.Decode(&first); err != nil {
		return false
	}
	var second json.RawMessage
	return decoder.Decode(&second) == nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return ErrInvalidBridgeV1Frame
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return ErrInvalidBridgeV1Frame
			}
			key, ok := keyToken.(string)
			if !ok {
				return ErrInvalidBridgeV1Frame
			}
			if _, exists := keys[key]; exists {
				return ErrDuplicateBridgeJSONKey
			}
			keys[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return ErrInvalidBridgeV1Frame
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return ErrInvalidBridgeV1Frame
		}
	default:
		return ErrInvalidBridgeV1Frame
	}
	return nil
}

func requireDecoderEOF(decoder *json.Decoder) error {
	_, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errTrailingJSONValue
	}
	return err
}
