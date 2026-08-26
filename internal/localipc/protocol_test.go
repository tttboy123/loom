package localipc

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestP3AJourneyWireAndMethodsAreStrictlyAvailable(t *testing.T) {
	requestType := reflect.TypeOf(Request{})
	requestJourney, found := requestType.FieldByName("JourneyID")
	if !found || requestJourney.Tag.Get("json") != "journey_id,omitempty" {
		t.Fatalf("Request JourneyID field = %#v, %v", requestJourney, found)
	}
	responseType := reflect.TypeOf(Response{})
	responseJourney, found := responseType.FieldByName("JourneyID")
	if !found || responseJourney.Tag.Get("json") != "journey_id,omitempty" {
		t.Fatalf("Response JourneyID field = %#v, %v", responseJourney, found)
	}
	for _, method := range []string{
		"evolution_asset_snapshot",
		"evolution_asset_diff",
		"evolution_asset_command",
	} {
		if !validMethod(method) {
			t.Fatalf("validMethod(%q) = false", method)
		}
	}
	request, err := decodeRequest([]byte(
		`{"version":1,"request_id":"request-p3a-red","journey_id":"11111111-1111-4111-8111-111111111111","method":"evolution_asset_snapshot","params":{"cursor":"","limit":1,"asset_kind":"","lifecycle":"","search_text":""}}`,
	))
	if err != nil {
		t.Fatalf("decodeRequest(P3A) error = %v", err)
	}
	journey := reflect.ValueOf(request).FieldByName("JourneyID")
	if !journey.IsValid() || journey.String() != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("decoded journey = %v", journey)
	}
}

func TestCredentialVaultLifecycleMethodsAreStrictAndExtended(t *testing.T) {
	for _, method := range []string{
		"credential_vault_rotate",
		"credential_vault_lock",
		"credential_vault_unlock",
		"credential_vault_reset",
		"credential_vault_export",
	} {
		if !validMethod(method) {
			t.Fatalf("%s method unavailable", method)
		}
		if !usesExtendedRequestDeadline(method) {
			t.Fatalf("%s lacks extended deadline", method)
		}
	}
	request, err := decodeRequest([]byte(
		`{"version":1,"request_id":"vault-rotation-1","method":"credential_vault_rotate","params":{}}`,
	))
	if err != nil || request.Method != "credential_vault_rotate" {
		t.Fatalf("decoded rotation request = %+v, %v", request, err)
	}
}

func TestCredentialImportMethodIsStrictAndUsesVerificationDeadline(t *testing.T) {
	const method = "credential_import"
	if !validMethod(method) {
		t.Fatalf("%s method unavailable", method)
	}
	if !usesExtendedRequestDeadline(method) {
		t.Fatalf("%s lacks Provider verification deadline", method)
	}
	request, err := decodeRequest([]byte(
		`{"version":1,"request_id":"credential-import-1","method":"credential_import","params":{"candidate_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","provider_id":"deepseek","provider_account_id":"deepseek.primary","confirm":true}}`,
	))
	if err != nil || request.Method != method {
		t.Fatalf("decoded import request = %+v, %v", request, err)
	}
}

func TestChatContextDisclosureMethodIsStrictAndUsesReadDeadline(t *testing.T) {
	const method = "chat_context_disclosure"
	if !validMethod(method) {
		t.Fatalf("%s method unavailable", method)
	}
	if requiresJourney(method) || usesExtendedRequestDeadline(method) {
		t.Fatalf("%s has non-read routing policy", method)
	}
	request, err := decodeRequest([]byte(
		`{"version":1,"request_id":"context-disclosure-1","method":"chat_context_disclosure","params":{"thread_id":"thread-1","segment_id":"segment-1"}}`,
	))
	if err != nil || request.Method != method {
		t.Fatalf("decoded context disclosure request = %+v, %v", request, err)
	}
}

func TestAgentAttemptRecoveryMethodIsStrictAndUsesProviderDeadline(t *testing.T) {
	method := "agent_attempt_recovery"
	if !validMethod(method) {
		t.Fatalf("%s method unavailable", method)
	}
	if requiresJourney(method) {
		t.Fatalf("%s unexpectedly requires a journey id", method)
	}
	if !usesExtendedRequestDeadline(method) {
		t.Fatalf("%s lacks extended deadline", method)
	}
	request, err := decodeRequest([]byte(
		`{"version":1,"request_id":"recovery-preview-1","method":"agent_attempt_recovery","params":{"schema_version":1,"operation":"preview"}}`,
	))
	if err != nil || request.Method != method {
		t.Fatalf("decoded recovery request = %+v, %v", request, err)
	}
}

func TestToolRecoveryMethodIsStrictAndUsesExtendedDeadline(t *testing.T) {
	method := "tool_recovery"
	if !validMethod(method) {
		t.Fatalf("%s method unavailable", method)
	}
	if requiresJourney(method) {
		t.Fatalf("%s unexpectedly requires a journey id", method)
	}
	if !usesExtendedRequestDeadline(method) {
		t.Fatalf("%s lacks extended deadline", method)
	}
	request, err := decodeRequest([]byte(
		`{"version":1,"request_id":"tool-recovery-preview-1","method":"tool_recovery","params":{"schema_version":1,"operation":"preview"}}`,
	))
	if err != nil || request.Method != method {
		t.Fatalf("decoded recovery request = %+v, %v", request, err)
	}
}

func TestQueueJourneyWireAndMethodsAreStrictlyAvailable(t *testing.T) {
	for _, method := range []string{"queue_snapshot", "queue_command"} {
		if !validMethod(method) {
			t.Fatalf("validMethod(%q) = false", method)
		}
		if !requiresJourney(method) {
			t.Fatalf("requiresJourney(%q) = false", method)
		}
	}
	request, err := decodeRequest([]byte(
		`{"version":1,"request_id":"request-queue-1","journey_id":"11111111-1111-4111-8111-111111111111","method":"queue_snapshot","params":{"cursor":"","limit":1}}`,
	))
	if err != nil {
		t.Fatalf("decodeRequest(queue_snapshot) error = %v", err)
	}
	if request.Method != "queue_snapshot" ||
		request.JourneyID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("decoded queue request = %+v", request)
	}
}

func TestP3AJourneyWireRejectsMissingDuplicateUnknownAndInvalidIdentity(t *testing.T) {
	validJourney := "123e4567-e89b-42d3-a456-426614174000"
	valid := []byte(`{"version":1,"request_id":"asset-1","journey_id":"` + validJourney + `","method":"evolution_asset_snapshot","params":{}}`)
	request, err := decodeRequest(valid)
	if err != nil {
		t.Fatalf("valid P3A request error = %v", err)
	}
	value := reflect.ValueOf(request)
	journey := value.FieldByName("JourneyID")
	if !journey.IsValid() || journey.String() != validJourney {
		t.Fatalf("decoded journey = %v, want %s", journey, validJourney)
	}
	for _, input := range []string{
		`{"version":1,"request_id":"asset-1","method":"evolution_asset_snapshot","params":{}}`,
		`{"version":1,"request_id":"asset-1","journey_id":"not-a-uuid","method":"evolution_asset_snapshot","params":{}}`,
		`{"version":1,"request_id":"asset-1","journey_id":"123E4567-E89B-42D3-A456-426614174000","method":"evolution_asset_snapshot","params":{}}`,
		`{"version":1,"request_id":"asset-1","journey_id":"123e4567-e89b-42d3-a456-426614174000","journey_id":"123e4567-e89b-42d3-a456-426614174000","method":"evolution_asset_snapshot","params":{}}`,
		`{"version":1,"request_id":"asset-1","journey_id":"123e4567-e89b-42d3-a456-426614174000","method":"evolution_asset_snapshot","params":{},"unexpected":true}`,
	} {
		if _, err := decodeRequest([]byte(input)); err == nil {
			t.Fatalf("decodeRequest(%s) error = nil", input)
		}
	}
}

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

func TestMissionDecisionIsTheOnlyAcceptedMissionMutationMethod(t *testing.T) {
	valid := []byte(
		`{"version":1,"request_id":"decision-1","method":"mission_decision","params":{"operation":"read"}}`,
	)
	request, err := decodeRequest(valid)
	if err != nil {
		t.Fatalf("mission_decision decode error = %v", err)
	}
	if request.Method != "mission_decision" {
		t.Fatalf("method = %q", request.Method)
	}
	for _, method := range []string{
		"decision_authorization",
		"decision_review",
		"decision_recovery",
		"mission_decide",
		"mission_decision_v2",
	} {
		input := []byte(
			`{"version":1,"request_id":"decision-1","method":"` +
				method + `","params":{}}`,
		)
		if _, err := decodeRequest(input); err == nil {
			t.Fatalf("decodeRequest accepted alias %q", method)
		}
	}
}

func TestMissionExecutionIsTheOnlyAcceptedExecutionMethod(t *testing.T) {
	valid := []byte(
		`{"version":1,"request_id":"execution-1","method":"mission_execution","params":{"operation":"preflight"}}`,
	)
	request, err := decodeRequest(valid)
	if err != nil {
		t.Fatalf("mission_execution decode error = %v", err)
	}
	if request.Method != "mission_execution" {
		t.Fatalf("method = %q", request.Method)
	}
	for _, method := range []string{
		"execution_preflight",
		"execution_start",
		"execution_control",
		"mission_execute",
		"mission_execution_v2",
	} {
		input := []byte(
			`{"version":1,"request_id":"execution-1","method":"` +
				method + `","params":{}}`,
		)
		if _, err := decodeRequest(input); err == nil {
			t.Fatalf("decodeRequest accepted alias %q", method)
		}
	}
}

func TestSideTaskHandoffIsTheOnlyAcceptedSideTaskMethod(t *testing.T) {
	valid := []byte(
		`{"version":1,"request_id":"side-task-1","method":"side_task_handoff","params":{"schema_version":1,"operation":"read"}}`,
	)
	request, err := decodeRequest(valid)
	if err != nil {
		t.Fatalf("side_task_handoff decode error = %v", err)
	}
	if request.Method != "side_task_handoff" {
		t.Fatalf("method = %q", request.Method)
	}
	for _, method := range []string{
		"side_task",
		"side_task_create",
		"side_task_decide",
		"side_task_handoff_v2",
	} {
		input := []byte(
			`{"version":1,"request_id":"side-task-1","method":"` +
				method + `","params":{}}`,
		)
		if _, err := decodeRequest(input); err == nil {
			t.Fatalf("decodeRequest accepted alias %q", method)
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
		`{"version":1,"request_id":"request-connect","method":"codex_connect","params":{}}`,
	)); err != nil {
		t.Fatalf("valid codex_connect request error = %v", err)
	}
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

func TestGovernanceConfigureMethodsAreStrictlyAvailable(t *testing.T) {
	for _, method := range []string{
		"provider_model_rate_card_configure",
		"remote_tool_backend_enrollment_configure",
		"remote_tool_backend_enrollment_revoke",
	} {
		if !validMethod(method) {
			t.Fatalf("validMethod(%q) = false", method)
		}
		if requiresJourney(method) {
			t.Fatalf("requiresJourney(%q) = true", method)
		}
	}
	request, err := decodeRequest([]byte(
		`{"version":1,"request_id":"governance-configure-1","method":"provider_model_rate_card_configure","params":{"provider_id":"deepseek","provider_account_id":"deepseek.primary"}}`,
	))
	if err != nil || request.Method != "provider_model_rate_card_configure" {
		t.Fatalf("decoded rate-card request = %+v, %v", request, err)
	}
	request, err = decodeRequest([]byte(
		`{"version":1,"request_id":"enrollment-configure-1","method":"remote_tool_backend_enrollment_configure","params":{"enrollment_id":"enroll-1"}}`,
	))
	if err != nil || request.Method != "remote_tool_backend_enrollment_configure" {
		t.Fatalf("decoded enrollment request = %+v, %v", request, err)
	}
	request, err = decodeRequest([]byte(
		`{"version":1,"request_id":"enrollment-revoke-1","method":"remote_tool_backend_enrollment_revoke","params":{"enrollment_id":"enroll-1"}}`,
	))
	if err != nil || request.Method != "remote_tool_backend_enrollment_revoke" {
		t.Fatalf("decoded enrollment revoke request = %+v, %v", request, err)
	}
}
