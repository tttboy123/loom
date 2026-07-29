package localipc

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestFrameRoundTripIsBoundedAndRejectsTrailingOrTruncatedInput(t *testing.T) {
	body := []byte(`{"version":1,"request_id":"request-1","method":"ping","params":{}}`)
	var buffer bytes.Buffer
	if err := writeFrame(&buffer, body, maxRequestBodyBytes); err != nil {
		t.Fatal(err)
	}
	decoded, err := readFrame(&buffer, maxRequestBodyBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, body) {
		t.Fatalf("decoded = %q, want %q", decoded, body)
	}
	if buffer.Len() != 0 {
		t.Fatalf("frame left %d bytes", buffer.Len())
	}
	if err := writeFrame(&buffer, nil, maxRequestBodyBytes); err == nil {
		t.Fatal("empty frame write error = nil")
	}
	if err := writeFrame(
		&buffer,
		make([]byte, maxRequestBodyBytes+1),
		maxRequestBodyBytes,
	); err == nil {
		t.Fatal("oversized frame write error = nil")
	}

	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "zero-length", data: []byte{0, 0, 0, 0}},
		{name: "truncated", data: []byte{0, 0, 0, 4, '{'}},
		{name: "oversized", data: []byte{0, 1, 0, 1}},
		{name: "trailing-frame", data: append(append([]byte{0, 0, 0, 2}, []byte(`{}`)...), 0)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := readSingleFrame(bytes.NewReader(test.data), maxRequestBodyBytes); err == nil {
				t.Fatal("invalid frame error = nil")
			}
		})
	}
}

func TestDecodeRequestRequiresExactVersionMethodIDAndJSONShape(t *testing.T) {
	valid := []byte(`{"version":1,"request_id":"request-1","method":"snapshot","params":{"limit":64}}`)
	request, err := decodeRequest(valid)
	if err != nil {
		t.Fatal(err)
	}
	if request.Version != 1 ||
		request.RequestID != "request-1" ||
		request.Method != "snapshot" ||
		!json.Valid(request.Params) {
		t.Fatalf("request = %#v", request)
	}

	invalid := []string{
		`{}`,
		`{"version":2,"request_id":"request-1","method":"ping","params":{}}`,
		`{"version":1,"version":1,"request_id":"request-1","method":"ping","params":{}}`,
		`{"version":1,"request_id":"","method":"ping","params":{}}`,
		`{"version":1,"request_id":"bad id","method":"ping","params":{}}`,
		`{"version":1,"request_id":"request-1","method":"missing","params":{}}`,
		`{"version":1,"request_id":"request-1","method":"ping","params":{},"extra":true}`,
		`{"version":1,"request_id":"request-1","method":"ping","params":null}`,
		`{"version":1.0,"request_id":"request-1","method":"ping","params":{}}`,
	}
	for _, input := range invalid {
		if _, err := decodeRequest([]byte(input)); err == nil {
			t.Fatalf("decodeRequest(%s) error = nil", input)
		}
	}
}

func TestProtocolErrorIsClosedAndNeverIncludesCause(t *testing.T) {
	cause := errors.New("private /Users/name/state.db token=secret")
	protocolErr := safeProtocolError("state_unavailable", cause)
	decodeErr := &requestDecodeError{code: "unknown_method"}
	if decodeErr.Error() != "invalid local IPC request: unknown_method" {
		t.Fatalf("request decode error string = %q", decodeErr.Error())
	}
	encoded, err := json.Marshal(protocolErr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"code":"state_unavailable"`) {
		t.Fatalf("encoded error = %s", encoded)
	}
	for _, forbidden := range []string{"Users", "state.db", "token", "secret"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("encoded error leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestProtocolRejectsNestedDuplicatesAndInvalidResponses(t *testing.T) {
	if _, err := decodeRequest([]byte(
		`{"version":1,"request_id":"request-1","method":"snapshot","params":{"items":[1,{"key":2}]}}`,
	)); err != nil {
		t.Fatalf("valid nested request error = %v", err)
	}
	for _, input := range []string{
		`{"version":1,"request_id":"request-1","method":"snapshot","params":{"nested":{"key":1,"key":2}}}`,
		`{"version":1,"request_id":"request-1","method":"snapshot","params":{"items":[{"key":1,"key":2}]}}`,
	} {
		if _, err := decodeRequest([]byte(input)); err == nil {
			t.Fatalf("nested duplicate accepted: %s", input)
		}
	}
	if _, err := decodeRequest([]byte(
		`{"version":1,"request_id":"request-1","method":"not-real","params":{}}`,
	)); requestErrorCode(err) != "unknown_method" {
		t.Fatalf("unknown method error = %v", err)
	}
	for _, response := range []Response{
		{},
		{Version: 1, RequestID: "request-1", OK: true},
		{
			Version:   1,
			RequestID: "request-1",
			OK:        true,
			Result:    json.RawMessage(`{}`),
			Error:     safeProtocolError("internal", nil),
		},
	} {
		if _, err := encodeResponse(response); err == nil {
			t.Fatalf("invalid response encoded: %#v", response)
		}
	}
	if errorResponse := safeProtocolError("not-a-code", errors.New(
		"secret",
	)); errorResponse.Code != "internal" {
		t.Fatalf("unknown code response = %#v", errorResponse)
	}
}
