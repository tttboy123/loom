package localipc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"unicode/utf8"
)

const (
	protocolVersion      = 1
	maxRequestBodyBytes  = 65536
	maxResponseBodyBytes = 524288
)

var (
	ErrInvalidProtocol = errors.New("invalid local IPC protocol")
	ErrProtocolTimeout = errors.New("local IPC timeout")
)

type requestDecodeError struct {
	code string
}

func (err *requestDecodeError) Error() string {
	return "invalid local IPC request: " + err.code
}

type Request struct {
	Version   int             `json:"version"`
	RequestID string          `json:"request_id"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params"`
}

type ProtocolError struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable,omitempty"`
}

type Response struct {
	Version   int             `json:"version"`
	RequestID string          `json:"request_id"`
	OK        bool            `json:"ok"`
	Result    json.RawMessage `json:"result"`
	Error     *ProtocolError  `json:"error"`
}

type PingResult struct {
	ProtocolVersion int    `json:"protocol_version"`
	Available       bool   `json:"available"`
	BuildID         string `json:"build_id"`
}

func writeFrame(writer io.Writer, body []byte, maximum uint32) error {
	if len(body) == 0 || uint64(len(body)) > uint64(maximum) {
		return ErrInvalidProtocol
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(body)))
	if err := writeAll(writer, length[:]); err != nil {
		return err
	}
	return writeAll(writer, body)
}

func readFrame(reader io.Reader, maximum uint32) ([]byte, error) {
	var length [4]byte
	if _, err := io.ReadFull(reader, length[:]); err != nil {
		var networkErr net.Error
		if errors.As(err, &networkErr) && networkErr.Timeout() {
			return nil, err
		}
		return nil, ErrInvalidProtocol
	}
	size := binary.BigEndian.Uint32(length[:])
	if size == 0 || size > maximum {
		return nil, ErrInvalidProtocol
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(reader, body); err != nil {
		var networkErr net.Error
		if errors.As(err, &networkErr) && networkErr.Timeout() {
			return nil, err
		}
		return nil, ErrInvalidProtocol
	}
	return body, nil
}

func readSingleFrame(reader io.Reader, maximum uint32) ([]byte, error) {
	body, err := readFrame(reader, maximum)
	if err != nil {
		return nil, err
	}
	var trailing [1]byte
	count, trailingErr := reader.Read(trailing[:])
	if count != 0 || trailingErr == nil || !errors.Is(trailingErr, io.EOF) {
		return nil, ErrInvalidProtocol
	}
	return body, nil
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written <= 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

func decodeRequest(data []byte) (Request, error) {
	if len(data) == 0 || len(data) > maxRequestBodyBytes ||
		!utf8.Valid(data) ||
		hasDuplicateJSONKeys(data) {
		return Request{}, ErrInvalidProtocol
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var request Request
	if err := decoder.Decode(&request); err != nil {
		return Request{}, ErrInvalidProtocol
	}
	if decoder.Decode(&struct{}{}) != io.EOF ||
		!validRequestID(request.RequestID) ||
		len(request.Params) == 0 ||
		string(request.Params) == "null" ||
		request.Params[0] != '{' {
		return Request{}, ErrInvalidProtocol
	}
	if request.Version != protocolVersion {
		return request, &requestDecodeError{code: "unsupported_version"}
	}
	if !validMethod(request.Method) {
		return request, &requestDecodeError{code: "unknown_method"}
	}
	var params map[string]json.RawMessage
	paramsDecoder := json.NewDecoder(bytes.NewReader(request.Params))
	paramsDecoder.UseNumber()
	if err := paramsDecoder.Decode(&params); err != nil ||
		params == nil ||
		paramsDecoder.Decode(&struct{}{}) != io.EOF {
		return Request{}, ErrInvalidProtocol
	}
	return request, nil
}

func requestErrorCode(err error) string {
	var decodeErr *requestDecodeError
	if errors.As(err, &decodeErr) {
		return decodeErr.code
	}
	return "invalid_request"
}

func encodeResponse(response Response) ([]byte, error) {
	if response.Version != protocolVersion ||
		!validRequestID(response.RequestID) ||
		response.OK == (response.Error != nil) ||
		response.OK && len(response.Result) == 0 ||
		!response.OK && len(response.Result) != 0 {
		return nil, ErrInvalidProtocol
	}
	data, err := json.Marshal(response)
	if err != nil || len(data) > maxResponseBodyBytes {
		return nil, ErrInvalidProtocol
	}
	return data, nil
}

func safeProtocolError(code string, _ error) *ProtocolError {
	message, recoverable, ok := protocolErrorDefinition(code)
	if !ok {
		code = "internal"
		message, recoverable, _ = protocolErrorDefinition(code)
	}
	return &ProtocolError{
		Code:        code,
		Message:     message,
		Recoverable: recoverable,
	}
}

func protocolErrorDefinition(code string) (string, bool, bool) {
	definitions := map[string]struct {
		message     string
		recoverable bool
	}{
		"invalid_request":      {"invalid request", false},
		"unsupported_version":  {"unsupported protocol version", false},
		"unknown_method":       {"unknown method", false},
		"unauthorized_peer":    {"unauthorized peer", false},
		"unsupported_platform": {"unsupported platform", false},
		"not_found":            {"not found", false},
		"conflict":             {"conflict", true},
		"incompatible":         {"incompatible", false},
		"denied":               {"denied", false},
		"credential_unavailable": {
			"credential unavailable",
			true,
		},
		"credential_rejected": {
			"credential rejected",
			false,
		},
		"credential_rollback_failed": {
			"credential rollback failed",
			false,
		},
		"cursor_conflict":   {"cursor conflict", true},
		"stream_gap":        {"stream gap", true},
		"state_unavailable": {"state unavailable", true},
		"timeout":           {"request timed out", true},
		"busy":              {"daemon busy", true},
		"internal":          {"internal error", true},
	}
	definition, ok := definitions[code]
	return definition.message, definition.recoverable, ok
}

func validRequestID(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' ||
			character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' ||
			character == ':' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validMethod(method string) bool {
	switch method {
	case "ping",
		"snapshot",
		"timeline_page",
		"setup_snapshot",
		"builder_start",
		"builder_answer",
		"builder_edit",
		"builder_validate",
		"builder_confirm",
		"team_archive",
		"team_restore",
		"credential_configure",
		"credential_verify",
		"credential_replace",
		"credential_revoke":
		return true
	default:
		return false
	}
}

func hasDuplicateJSONKeys(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	duplicate, err := scanJSONValue(decoder)
	if err != nil || duplicate {
		return true
	}
	return decoder.Decode(&struct{}{}) != io.EOF
}

func scanJSONValue(decoder *json.Decoder) (bool, error) {
	token, err := decoder.Token()
	if err != nil {
		return false, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return false, nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return false, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return false, ErrInvalidProtocol
			}
			if _, exists := seen[key]; exists {
				return true, nil
			}
			seen[key] = struct{}{}
			duplicate, err := scanJSONValue(decoder)
			if err != nil || duplicate {
				return duplicate, err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return false, fmt.Errorf("%w: object", ErrInvalidProtocol)
		}
	case '[':
		for decoder.More() {
			duplicate, err := scanJSONValue(decoder)
			if err != nil || duplicate {
				return duplicate, err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return false, fmt.Errorf("%w: array", ErrInvalidProtocol)
		}
	default:
		return false, ErrInvalidProtocol
	}
	return false, nil
}
