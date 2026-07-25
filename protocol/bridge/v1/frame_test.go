package bridgev1

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestS3W1MandatoryMarkers(t *testing.T) {
	t.Parallel()

	markers := []string{
		"s3_w1_" + "frame_exact_round_trip",
		"s3_w1_" + "frame_rejection_matrix",
		"s3_w1_" + "json_duplicate_key_rejection",
		"s3_w1_" + "frame_bounds",
		"s3_w1_" + "stream_binding_sequence_uniqueness",
		"s3_w1_" + "stream_terminal_and_buffer_bounds",
		"s3_w1_" + "mutation_concurrency_fuzz_static",
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	source, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read test source: %v", err)
	}

	for _, marker := range markers {
		if count := bytes.Count(source, []byte(marker)); count != 1 {
			t.Errorf("mandatory marker %q count = %d, want 1", marker, count)
		}
	}
}

var (
	testMessageTypes = []MessageType{
		MessageDispatch,
		MessageEvent,
		MessageEvidence,
		MessageResult,
		MessageCancel,
		MessageAck,
		MessageHeartbeat,
	}
	testEmittedAt = time.Date(2026, 7, 25, 12, 34, 56, 123456789, time.UTC)
)

func validFrameInput(sequence int64, messageType MessageType) FrameInput {
	return FrameInput{
		MessageID:             fmt.Sprintf("00000000-0000-4000-8000-%012x", sequence),
		CorrelationID:         "11111111-1111-4111-8111-111111111111",
		WorkItemID:            "S3-W1",
		RunID:                 "run-1",
		ClaimGeneration:       7,
		RuntimeInstanceID:     "runtime-1",
		SenderAgentInstanceID: "agent-1",
		Sequence:              sequence,
		Type:                  messageType,
		EmittedAt:             testEmittedAt,
		Payload:               []byte(`{"kind":"fixture","nested":{"ok":true}}`),
	}
}

func mustFrame(t testing.TB, input FrameInput) Frame {
	t.Helper()
	frame, err := NewFrame(input)
	if err != nil {
		t.Fatalf("NewFrame() error = %v", err)
	}
	return frame
}

func mustLine(t testing.TB, frame Frame) []byte {
	t.Helper()
	line, err := EncodeLine(frame)
	if err != nil {
		t.Fatalf("EncodeLine() error = %v", err)
	}
	return line
}

func assertZeroStream(t testing.TB, stream BoundRunStream) {
	t.Helper()
	if got := stream.Binding(); got != (RunStreamBinding{}) {
		t.Fatalf("zero stream binding = %#v", got)
	}
	if got := stream.Frames(); got != nil {
		t.Fatalf("zero stream frames = %#v, want nil", got)
	}
	if stream.LastSequence() != 0 || stream.TerminalResultSeen() || stream.BufferedBytes() != 0 {
		t.Fatalf("non-zero stream metadata")
	}
}

func TestFrameExactRoundTrip(t *testing.T) { // s3_w1_frame_exact_round_trip
	t.Parallel()

	for index, messageType := range testMessageTypes {
		messageType := messageType
		t.Run(string(messageType), func(t *testing.T) {
			t.Parallel()
			input := validFrameInput(int64(index+1), messageType)
			frame := mustFrame(t, input)

			input.Payload[2] = 'X'
			if got := string(frame.Payload()); got != `{"kind":"fixture","nested":{"ok":true}}` {
				t.Fatalf("payload after input mutation = %q", got)
			}

			line1 := mustLine(t, frame)
			line2 := mustLine(t, frame)
			if !bytes.Equal(line1, line2) {
				t.Fatal("encoding is not deterministic")
			}
			if len(line1) == 0 || line1[len(line1)-1] != '\n' ||
				bytes.Count(line1, []byte{'\n'}) != 1 {
				t.Fatalf("invalid JSON Lines envelope %q", line1)
			}

			decoded, err := DecodeLine(line1)
			if err != nil {
				t.Fatalf("DecodeLine() error = %v", err)
			}
			if decoded.ProtocolVersion() != ProtocolVersion ||
				decoded.MessageID() != frame.MessageID() ||
				decoded.CorrelationID() != frame.CorrelationID() ||
				decoded.WorkItemID() != frame.WorkItemID() ||
				decoded.RunID() != frame.RunID() ||
				decoded.ClaimGeneration() != frame.ClaimGeneration() ||
				decoded.RuntimeInstanceID() != frame.RuntimeInstanceID() ||
				decoded.SenderAgentInstanceID() != frame.SenderAgentInstanceID() ||
				decoded.Sequence() != frame.Sequence() ||
				decoded.Type() != frame.Type() ||
				!decoded.EmittedAt().Equal(frame.EmittedAt()) ||
				!bytes.Equal(decoded.Payload(), frame.Payload()) {
				t.Fatalf("round trip mismatch: %#v", decoded)
			}

			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(line1[:len(line1)-1], &envelope); err != nil {
				t.Fatalf("decode encoded envelope: %v", err)
			}
			wantKeys := []string{
				"protocol_version", "message_id", "correlation_id", "work_item_id",
				"run_id", "claim_generation", "runtime_instance_id",
				"sender_agent_instance_id", "seq", "type", "emitted_at", "payload",
			}
			if len(envelope) != len(wantKeys) {
				t.Fatalf("top-level key count = %d, want 12", len(envelope))
			}
			for _, key := range wantKeys {
				if _, ok := envelope[key]; !ok {
					t.Errorf("missing encoded key %q", key)
				}
			}
		})
	}
}

func TestFrameRejectionMatrix(t *testing.T) { // s3_w1_frame_rejection_matrix
	t.Parallel()

	validLine := mustLine(t, mustFrame(t, validFrameInput(1, MessageEvent)))
	var validEnvelope map[string]json.RawMessage
	if err := json.Unmarshal(validLine[:len(validLine)-1], &validEnvelope); err != nil {
		t.Fatal(err)
	}

	t.Run("every required field", func(t *testing.T) {
		for key := range validEnvelope {
			envelope := make(map[string]json.RawMessage, len(validEnvelope)-1)
			for candidateKey, value := range validEnvelope {
				if candidateKey != key {
					envelope[candidateKey] = value
				}
			}
			body, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			_, err = DecodeLine(append(body, '\n'))
			if !errors.Is(err, ErrInvalidBridgeV1Frame) {
				t.Errorf("remove %q error = %v, want ErrInvalidBridgeV1Frame", key, err)
			}
		}
	})

	t.Run("thirteenth field", func(t *testing.T) {
		body := append([]byte(nil), validLine[:len(validLine)-2]...)
		body = append(body, []byte(`,"extra":true}`+"\n")...)
		_, err := DecodeLine(body)
		if !errors.Is(err, ErrInvalidBridgeV1Frame) {
			t.Fatalf("error = %v, want ErrInvalidBridgeV1Frame", err)
		}
	})

	lineCases := []struct {
		name string
		line []byte
		want error
	}{
		{"empty", nil, ErrInvalidBridgeV1Line},
		{"only LF", []byte("\n"), ErrInvalidBridgeV1Line},
		{"missing LF", append([]byte(nil), validLine[:len(validLine)-1]...), ErrInvalidBridgeV1Line},
		{"CRLF", append(append([]byte(nil), validLine[:len(validLine)-1]...), '\r', '\n'), ErrInvalidBridgeV1Line},
		{"leading bytes", append([]byte(" "), validLine...), ErrInvalidBridgeV1Line},
		{"trailing bytes", append(append([]byte(nil), validLine...), 'x'), ErrInvalidBridgeV1Line},
		{"embedded literal newline", []byte("{\n}\n"), ErrInvalidBridgeV1Line},
		{"multiple values", []byte("{} {}\n"), ErrInvalidBridgeV1Line},
		{"invalid UTF-8", []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}', '\n'}, ErrInvalidBridgeV1Line},
		{"array", []byte("[]\n"), ErrInvalidBridgeV1Frame},
		{"null", []byte("null\n"), ErrInvalidBridgeV1Frame},
		{"truncated", []byte("{\n"), ErrInvalidBridgeV1Frame},
	}
	for _, testCase := range lineCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			_, err := DecodeLine(testCase.line)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
		})
	}

	inputCases := []struct {
		name   string
		mutate func(*FrameInput)
		want   error
	}{
		{"uppercase message UUID", func(input *FrameInput) {
			input.MessageID = strings.ToUpper("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
		}, ErrInvalidBridgeV1Frame},
		{"short message UUID", func(input *FrameInput) { input.MessageID = "not-a-uuid" }, ErrInvalidBridgeV1Frame},
		{"uppercase correlation UUID", func(input *FrameInput) {
			input.CorrelationID = strings.ToUpper("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
		}, ErrInvalidBridgeV1Frame},
		{"empty work item", func(input *FrameInput) { input.WorkItemID = "" }, ErrInvalidBridgeV1Frame},
		{"leading space ID", func(input *FrameInput) { input.RunID = " run" }, ErrInvalidBridgeV1Frame},
		{"trailing space ID", func(input *FrameInput) { input.RuntimeInstanceID = "runtime " }, ErrInvalidBridgeV1Frame},
		{"control ID", func(input *FrameInput) { input.SenderAgentInstanceID = "agent\n1" }, ErrInvalidBridgeV1Frame},
		{"zero generation", func(input *FrameInput) { input.ClaimGeneration = 0 }, ErrInvalidBridgeV1Frame},
		{"negative generation", func(input *FrameInput) { input.ClaimGeneration = -1 }, ErrInvalidBridgeV1Frame},
		{"zero sequence", func(input *FrameInput) { input.Sequence = 0 }, ErrInvalidBridgeV1Frame},
		{"unknown type", func(input *FrameInput) { input.Type = "other" }, ErrUnknownBridgeMessageType},
		{"zero time", func(input *FrameInput) { input.EmittedAt = time.Time{} }, ErrInvalidBridgeV1Frame},
		{"non-UTC time", func(input *FrameInput) { input.EmittedAt = testEmittedAt.In(time.FixedZone("offset", 3600)) }, ErrInvalidBridgeV1Frame},
		{"nil payload", func(input *FrameInput) { input.Payload = nil }, ErrInvalidBridgeV1Frame},
		{"null payload", func(input *FrameInput) { input.Payload = []byte("null") }, ErrInvalidBridgeV1Frame},
		{"array payload", func(input *FrameInput) { input.Payload = []byte("[]") }, ErrInvalidBridgeV1Frame},
		{"scalar payload", func(input *FrameInput) { input.Payload = []byte("true") }, ErrInvalidBridgeV1Frame},
		{"invalid payload", func(input *FrameInput) { input.Payload = []byte("{") }, ErrInvalidBridgeV1Frame},
	}
	for _, testCase := range inputCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			input := validFrameInput(1, MessageEvent)
			testCase.mutate(&input)
			_, err := NewFrame(input)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
		})
	}

	t.Run("wire protocol and type sentinels", func(t *testing.T) {
		for _, testCase := range []struct {
			old  string
			new  string
			want error
		}{
			{`"protocol_version":"loom.bridge.v1"`, `"protocol_version":"loom.bridge.v2"`, ErrUnsupportedBridgeProtocol},
			{`"type":"event"`, `"type":"other"`, ErrUnknownBridgeMessageType},
			{`"emitted_at":"2026-07-25T12:34:56.123456789Z"`, `"emitted_at":"2026-07-25T13:34:56.123456789+01:00"`, ErrInvalidBridgeV1Frame},
		} {
			line := bytes.Replace(validLine, []byte(testCase.old), []byte(testCase.new), 1)
			_, err := DecodeLine(line)
			if !errors.Is(err, testCase.want) {
				t.Errorf("replace %q error = %v, want %v", testCase.old, err, testCase.want)
			}
		}
	})
}

func TestJSONDuplicateKeyRejection(t *testing.T) { // s3_w1_json_duplicate_key_rejection
	t.Parallel()

	validLine := mustLine(t, mustFrame(t, validFrameInput(1, MessageEvent)))
	topLevelDuplicate := append([]byte(nil), validLine[:len(validLine)-2]...)
	topLevelDuplicate = append(topLevelDuplicate, []byte(`,"message_id":"22222222-2222-4222-8222-222222222222"}`+"\n")...)

	tests := [][]byte{
		topLevelDuplicate,
		bytes.Replace(validLine, []byte(`"nested":{"ok":true}`), []byte(`"nested":{"ok":true,"ok":false}`), 1),
		bytes.Replace(validLine, []byte(`"nested":{"ok":true}`), []byte(`"nested":{"a":true,"\u0061":false}`), 1),
		bytes.Replace(validLine, []byte(`"nested":{"ok":true}`), []byte(`"nested":{"deep":{"x":1,"x":2}}`), 1),
	}
	for index, line := range tests {
		_, err := DecodeLine(line)
		if !errors.Is(err, ErrDuplicateBridgeJSONKey) {
			t.Errorf("case %d error = %v, want ErrDuplicateBridgeJSONKey", index, err)
		}
	}

	input := validFrameInput(1, MessageEvent)
	input.Payload = []byte(`{"a":1,"\u0061":2}`)
	_, err := NewFrame(input)
	if !errors.Is(err, ErrDuplicateBridgeJSONKey) {
		t.Fatalf("NewFrame duplicate error = %v, want ErrDuplicateBridgeJSONKey", err)
	}
}

func payloadWithCompactSize(t testing.TB, size int) []byte {
	t.Helper()
	if size < len(`{"x":""}`) {
		t.Fatalf("payload size %d is too small", size)
	}
	return []byte(`{"x":"` + strings.Repeat("a", size-len(`{"x":""}`)) + `"}`)
}

func TestFrameBounds(t *testing.T) { // s3_w1_frame_bounds
	t.Parallel()

	for _, size := range []int{MaxOpaqueIDBytes - 1, MaxOpaqueIDBytes} {
		input := validFrameInput(1, MessageEvent)
		input.WorkItemID = strings.Repeat("w", size)
		if _, err := NewFrame(input); err != nil {
			t.Fatalf("opaque ID size %d error = %v", size, err)
		}
	}
	input := validFrameInput(1, MessageEvent)
	input.WorkItemID = strings.Repeat("w", MaxOpaqueIDBytes+1)
	if _, err := NewFrame(input); !errors.Is(err, ErrInvalidBridgeV1Frame) {
		t.Fatalf("oversize opaque ID error = %v", err)
	}

	for _, size := range []int{MaxPayloadBytes - 1, MaxPayloadBytes} {
		input := validFrameInput(1, MessageEvent)
		input.Payload = payloadWithCompactSize(t, size)
		frame, err := NewFrame(input)
		if err != nil {
			t.Fatalf("payload size %d error = %v", size, err)
		}
		if len(frame.Payload()) != size {
			t.Fatalf("stored payload size = %d, want %d", len(frame.Payload()), size)
		}
	}
	input = validFrameInput(1, MessageEvent)
	input.Payload = payloadWithCompactSize(t, MaxPayloadBytes+1)
	if _, err := NewFrame(input); !errors.Is(err, ErrInvalidBridgeV1Frame) {
		t.Fatalf("oversize payload error = %v", err)
	}

	base := mustLine(t, mustFrame(t, validFrameInput(1, MessageEvent)))
	makeSizedLine := func(size int) []byte {
		t.Helper()
		padding := size - len(base)
		if padding < 0 {
			t.Fatalf("requested line size %d below base %d", size, len(base))
		}
		line := append([]byte(nil), base[:len(base)-2]...)
		line = append(line, bytes.Repeat([]byte{' '}, padding)...)
		line = append(line, '}', '\n')
		return line
	}
	for _, size := range []int{MaxLineBytes - 1, MaxLineBytes} {
		line := makeSizedLine(size)
		if _, err := DecodeLine(line); err != nil {
			t.Fatalf("line size %d error = %v", size, err)
		}
	}
	if _, err := DecodeLine(makeSizedLine(MaxLineBytes + 1)); !errors.Is(err, ErrBridgeV1LineTooLarge) {
		t.Fatalf("oversize line error = %v, want ErrBridgeV1LineTooLarge", err)
	}

	whitespacePayload := []byte("{  \"x\"  :  \"ok\"  }")
	input = validFrameInput(1, MessageEvent)
	input.Payload = whitespacePayload
	frame := mustFrame(t, input)
	if got := string(frame.Payload()); got != `{"x":"ok"}` {
		t.Fatalf("compact payload = %q", got)
	}
}

func validBinding() RunStreamBinding {
	return RunStreamBinding{
		WorkItemID:            "S3-W1",
		RunID:                 "run-1",
		ClaimGeneration:       7,
		RuntimeInstanceID:     "runtime-1",
		SenderAgentInstanceID: "agent-1",
	}
}

func TestStreamBindingSequenceUniqueness(t *testing.T) { // s3_w1_stream_binding_sequence_uniqueness
	t.Parallel()

	stream, err := NewBoundRunStream(validBinding())
	if err != nil {
		t.Fatal(err)
	}
	first := mustFrame(t, validFrameInput(1, MessageEvent))
	stream, err = AdvanceBoundRunStream(stream, first)
	if err != nil {
		t.Fatal(err)
	}
	thirdInput := validFrameInput(3, MessageEvidence)
	thirdInput.MessageID = "33333333-3333-4333-8333-333333333333"
	third := mustFrame(t, thirdInput)
	stream, err = AdvanceBoundRunStream(stream, third)
	if err != nil {
		t.Fatalf("non-contiguous increasing sequence error = %v", err)
	}
	if stream.LastSequence() != 3 || len(stream.Frames()) != 2 {
		t.Fatalf("stream metadata = last %d frames %d", stream.LastSequence(), len(stream.Frames()))
	}

	invalidBindings := []RunStreamBinding{
		{},
		{WorkItemID: " S3-W1", RunID: "run-1", ClaimGeneration: 7, RuntimeInstanceID: "runtime-1", SenderAgentInstanceID: "agent-1"},
		{WorkItemID: "S3-W1", RunID: "run-1", ClaimGeneration: 0, RuntimeInstanceID: "runtime-1", SenderAgentInstanceID: "agent-1"},
	}
	for _, binding := range invalidBindings {
		candidate, err := NewBoundRunStream(binding)
		if !errors.Is(err, ErrInvalidBridgeV1RunStream) {
			t.Errorf("binding %#v error = %v", binding, err)
		}
		assertZeroStream(t, candidate)
	}

	failures := []struct {
		name   string
		mutate func(*FrameInput)
	}{
		{"work item mismatch", func(input *FrameInput) { input.WorkItemID = "S3-W2" }},
		{"run mismatch", func(input *FrameInput) { input.RunID = "run-2" }},
		{"generation mismatch", func(input *FrameInput) { input.ClaimGeneration++ }},
		{"runtime mismatch", func(input *FrameInput) { input.RuntimeInstanceID = "runtime-2" }},
		{"sender mismatch", func(input *FrameInput) { input.SenderAgentInstanceID = "agent-2" }},
		{"non-increasing sequence", func(input *FrameInput) { input.Sequence = 3 }},
		{"duplicate message", func(input *FrameInput) {
			input.MessageID = first.MessageID()
			input.Sequence = 4
		}},
	}
	beforeFrames := stream.Frames()
	beforeBytes := stream.BufferedBytes()
	for _, testCase := range failures {
		input := validFrameInput(4, MessageEvent)
		testCase.mutate(&input)
		frame := mustFrame(t, input)
		candidate, err := AdvanceBoundRunStream(stream, frame)
		if !errors.Is(err, ErrInvalidBridgeV1RunStream) {
			t.Errorf("%s error = %v", testCase.name, err)
		}
		assertZeroStream(t, candidate)
		if !reflect.DeepEqual(stream.Frames(), beforeFrames) ||
			stream.BufferedBytes() != beforeBytes {
			t.Fatalf("%s mutated prior stream", testCase.name)
		}
	}
}

func frameWithEncodedSize(t testing.TB, sequence int64, size int) Frame {
	t.Helper()
	input := validFrameInput(sequence, MessageEvent)
	input.Payload = payloadWithCompactSize(t, len(`{"x":""}`))
	base := mustFrame(t, input)
	baseSize := len(mustLine(t, base))
	targetPayloadSize := len(input.Payload) + size - baseSize
	if targetPayloadSize < len(`{"x":""}`) || targetPayloadSize > MaxPayloadBytes {
		t.Fatalf("encoded size %d cannot be constructed (base %d)", size, baseSize)
	}
	input.Payload = payloadWithCompactSize(t, targetPayloadSize)
	frame := mustFrame(t, input)
	if got := len(mustLine(t, frame)); got != size {
		t.Fatalf("encoded frame size = %d, want %d", got, size)
	}
	return frame
}

func TestStreamTerminalAndBufferBounds(t *testing.T) { // s3_w1_stream_terminal_and_buffer_bounds
	t.Parallel()

	t.Run("terminal once", func(t *testing.T) {
		stream, err := NewBoundRunStream(validBinding())
		if err != nil {
			t.Fatal(err)
		}
		result := mustFrame(t, validFrameInput(1, MessageResult))
		stream, err = AdvanceBoundRunStream(stream, result)
		if err != nil || !stream.TerminalResultSeen() {
			t.Fatalf("first result: terminal=%v error=%v", stream.TerminalResultSeen(), err)
		}
		secondInput := validFrameInput(2, MessageResult)
		secondInput.MessageID = "22222222-2222-4222-8222-222222222222"
		candidate, err := AdvanceBoundRunStream(stream, mustFrame(t, secondInput))
		if !errors.Is(err, ErrInvalidBridgeV1RunStream) {
			t.Fatalf("second result error = %v", err)
		}
		assertZeroStream(t, candidate)
	})

	t.Run("frame count exact and plus one", func(t *testing.T) {
		stream, err := NewBoundRunStream(validBinding())
		if err != nil {
			t.Fatal(err)
		}
		for sequence := 1; sequence <= MaxBufferedFrames; sequence++ {
			stream, err = AdvanceBoundRunStream(stream, mustFrame(t, validFrameInput(int64(sequence), MessageEvent)))
			if err != nil {
				t.Fatalf("advance %d: %v", sequence, err)
			}
		}
		if len(stream.Frames()) != MaxBufferedFrames {
			t.Fatalf("frame count = %d", len(stream.Frames()))
		}
		candidate, err := AdvanceBoundRunStream(stream, mustFrame(t, validFrameInput(MaxBufferedFrames+1, MessageEvent)))
		if !errors.Is(err, ErrBridgeV1RunStreamBufferLimit) {
			t.Fatalf("plus one error = %v", err)
		}
		assertZeroStream(t, candidate)
	})

	t.Run("byte count exact and plus one", func(t *testing.T) {
		stream, err := NewBoundRunStream(validBinding())
		if err != nil {
			t.Fatal(err)
		}
		sequence := int64(1)
		remaining := MaxBufferedBytes
		minSize := len(mustLine(t, mustFrame(t, validFrameInput(sequence, MessageEvent))))
		maxFrame := frameWithEncodedSize(t, sequence, minSize+MaxPayloadBytes-len(`{"kind":"fixture","nested":{"ok":true}}`))
		maxSize := len(mustLine(t, maxFrame))
		for remaining > maxSize {
			size := maxSize
			if remaining-size < minSize {
				size = remaining - minSize
			}
			frame := frameWithEncodedSize(t, sequence, size)
			stream, err = AdvanceBoundRunStream(stream, frame)
			if err != nil {
				t.Fatalf("advance size %d: %v", size, err)
			}
			remaining -= size
			sequence++
		}
		frame := frameWithEncodedSize(t, sequence, remaining)
		stream, err = AdvanceBoundRunStream(stream, frame)
		if err != nil {
			t.Fatalf("final exact advance: %v", err)
		}
		if stream.BufferedBytes() != MaxBufferedBytes {
			t.Fatalf("buffered bytes = %d, want %d", stream.BufferedBytes(), MaxBufferedBytes)
		}
		candidate, err := AdvanceBoundRunStream(stream, mustFrame(t, validFrameInput(sequence+1, MessageEvent)))
		if !errors.Is(err, ErrBridgeV1RunStreamBufferLimit) {
			t.Fatalf("byte plus one error = %v", err)
		}
		assertZeroStream(t, candidate)
	})
}

func TestMutationConcurrencyFuzzStatic(t *testing.T) { // s3_w1_mutation_concurrency_fuzz_static
	t.Parallel()

	input := validFrameInput(1, MessageEvent)
	frame := mustFrame(t, input)
	payload := frame.Payload()
	payload[0] = '['
	if string(frame.Payload()) != `{"kind":"fixture","nested":{"ok":true}}` {
		t.Fatal("payload accessor aliases internal storage")
	}

	stream, err := NewBoundRunStream(validBinding())
	if err != nil {
		t.Fatal(err)
	}
	stream, err = AdvanceBoundRunStream(stream, frame)
	if err != nil {
		t.Fatal(err)
	}
	frames := stream.Frames()
	frames[0] = Frame{}
	framesAgain := stream.Frames()
	if len(framesAgain) != 1 || framesAgain[0].MessageID() != frame.MessageID() {
		t.Fatal("frames accessor aliases internal storage")
	}
	nestedPayload := framesAgain[0].Payload()
	nestedPayload[0] = '['
	if stream.Frames()[0].Payload()[0] != '{' {
		t.Fatal("stream frame payload aliases internal storage")
	}

	var waitGroup sync.WaitGroup
	for index := 0; index < 32; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for iteration := 0; iteration < 100; iteration++ {
				_ = frame.Payload()
				_ = mustLine(t, frame)
				_ = stream.Frames()
				_ = stream.Binding()
				_ = stream.BufferedBytes()
			}
		}()
	}
	waitGroup.Wait()
}

func TestStaticAuthorityBoundary(t *testing.T) {
	t.Parallel()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "frame.go", nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse frame.go: %v", err)
	}
	allowedImports := map[string]bool{
		"bytes": true, "encoding/json": true, "errors": true, "fmt": true,
		"io": true, "strings": true, "time": true, "unicode": true,
		"unicode/utf8": true,
	}
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		if !allowedImports[path] {
			t.Errorf("forbidden import %q", path)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if _, ok := node.(*ast.GoStmt); ok {
			t.Error("goroutine authority is forbidden")
		}
		return true
	})
}

func FuzzDecodeLineNeverPanics(f *testing.F) {
	valid := mustLine(f, mustFrame(f, validFrameInput(1, MessageEvent)))
	for _, seed := range [][]byte{
		nil,
		[]byte("\n"),
		[]byte("{\n"),
		[]byte(`{"x":{"a":1,"a":2}}` + "\n"),
		append([]byte(nil), valid...),
		append(bytes.Repeat([]byte{' '}, MaxLineBytes+1), '\n'),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, line []byte) {
		frame, err := DecodeLine(line)
		if err != nil {
			return
		}
		encoded, err := EncodeLine(frame)
		if err != nil {
			t.Fatalf("accepted frame cannot encode: %v", err)
		}
		if _, err := DecodeLine(encoded); err != nil {
			t.Fatalf("encoded accepted frame cannot decode: %v", err)
		}
	})
}
