package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/localipc"
)

type remoteToolBackendEnrollmentWireBackend struct {
	configureCommand app.RemoteToolBackendEnrollmentCommand
	revokeCommand    app.RemoteToolBackendEnrollmentRevokeCommand
	result           app.RemoteToolBackendEnrollmentResult
}

func (backend *remoteToolBackendEnrollmentWireBackend) SetupSnapshot(
	context.Context,
) (app.SetupSnapshot, error) {
	return app.SetupSnapshot{}, nil
}

func (backend *remoteToolBackendEnrollmentWireBackend) StartBuilder(
	context.Context,
	app.BuilderStartCommand,
) (app.BuilderSessionView, error) {
	return app.BuilderSessionView{}, nil
}

func (backend *remoteToolBackendEnrollmentWireBackend) ConfigureRemoteToolBackendEnrollment(
	_ context.Context,
	command app.RemoteToolBackendEnrollmentCommand,
) (app.RemoteToolBackendEnrollmentResult, error) {
	backend.configureCommand = command
	return backend.result, nil
}

func (backend *remoteToolBackendEnrollmentWireBackend) RevokeRemoteToolBackendEnrollment(
	_ context.Context,
	command app.RemoteToolBackendEnrollmentRevokeCommand,
) (app.RemoteToolBackendEnrollmentResult, error) {
	backend.revokeCommand = command
	result := backend.result
	result.Revision = command.ExpectedRevision + 1
	result.Status = "revoked"
	return result, nil
}

func TestRemoteToolBackendEnrollmentWireInjectsTrustedCorrelationAndRejectsUnknownFields(t *testing.T) {
	backend := &remoteToolBackendEnrollmentWireBackend{
		result: app.RemoteToolBackendEnrollmentResult{
			EnrollmentAvailable: true, EnrollmentVersion: 1,
			EnrollmentID: "search-deepseek-work", BackendKind: "web_search",
			AdapterID: "builtin.search.v1", ProviderID: "deepseek",
			ProviderAccountID:            "deepseek.work",
			ProviderAccountPolicyVersion: 2, ProviderAccountPolicyRevision: 3,
			ProviderAccountPolicyDigest: strings.Repeat("a", 64),
			PolicyCurrent:               true,
			EndpointFingerprint:         strings.Repeat("b", 64),
			AllowedTools:                []string{}, Revision: 1, Status: "active",
			MaximumConcurrentCalls: 2, MaximumCallsPerAttempt: 4,
			TimeoutSeconds: 30, MaximumResultBytes: 32768,
			MaximumBudgetUnits: 2000,
			ConfiguredAt:       "2026-08-15T10:00:00Z",
			EnrollmentDigest:   strings.Repeat("c", 64),
		},
	}
	setup, err := api.NewLocalProductSetupAPI(backend)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandler(nil, setup)
	request := localipc.Request{
		Version: 1, RequestID: "loom-enrollment-wire-incident-1",
		Method: "remote_tool_backend_enrollment_configure",
		Params: json.RawMessage(`{
          "enrollment_id":"search-deepseek-work",
          "backend_kind":"web_search",
          "adapter_id":"builtin.search.v1",
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "provider_account_policy_version":2,
          "provider_account_policy_revision":3,
          "provider_account_policy_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "endpoint_fingerprint":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          "mcp_server_id":"",
          "allowed_tools":[],
          "expected_revision":0,
          "maximum_concurrent_calls":2,
          "maximum_calls_per_attempt":4,
          "timeout_seconds":30,
          "maximum_result_bytes":32768,
          "maximum_budget_units":2000,
          "operation_id":"configure-search-deepseek-work"
        }`),
	}
	response := handler(context.Background(), request)
	if !response.OK || response.Error != nil {
		t.Fatalf("configure response = %#v", response)
	}
	if backend.configureCommand.CorrelationID != request.RequestID ||
		backend.configureCommand.ProviderAccountID != "deepseek.work" ||
		backend.configureCommand.OperationID != "configure-search-deepseek-work" ||
		backend.configureCommand.ProviderAccountPolicyRevision != 3 {
		t.Fatalf("configure command = %#v", backend.configureCommand)
	}
	var configured app.RemoteToolBackendEnrollmentResult
	if err := json.Unmarshal(response.Result, &configured); err != nil ||
		configured.EnrollmentDigest != backend.result.EnrollmentDigest {
		t.Fatalf("configure wire result = %#v, err=%v", configured, err)
	}

	revoke := localipc.Request{
		Version: 1, RequestID: "loom-enrollment-wire-incident-2",
		Method: "remote_tool_backend_enrollment_revoke",
		Params: json.RawMessage(`{
          "enrollment_id":"search-deepseek-work",
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "expected_revision":1,
          "operation_id":"revoke-search-deepseek-work"
        }`),
	}
	revokedResponse := handler(context.Background(), revoke)
	if !revokedResponse.OK || revokedResponse.Error != nil ||
		backend.revokeCommand.CorrelationID != revoke.RequestID ||
		backend.revokeCommand.ExpectedRevision != 1 {
		t.Fatalf("revoke response = %#v, command=%#v", revokedResponse, backend.revokeCommand)
	}

	request.RequestID = "loom-enrollment-wire-incident-3"
	request.Params = json.RawMessage(`{
      "enrollment_id":"search-deepseek-work",
      "backend_kind":"web_search",
      "adapter_id":"builtin.search.v1",
      "provider_id":"deepseek",
      "provider_account_id":"deepseek.work",
      "provider_account_policy_version":2,
      "provider_account_policy_revision":3,
      "provider_account_policy_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "endpoint_fingerprint":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "mcp_server_id":"",
      "allowed_tools":[],
      "expected_revision":0,
      "maximum_concurrent_calls":2,
      "maximum_calls_per_attempt":4,
      "timeout_seconds":30,
      "maximum_result_bytes":32768,
      "maximum_budget_units":2000,
      "operation_id":"configure-search-deepseek-work",
      "secret":"must-not-be-accepted"
    }`)
	rejected := handler(context.Background(), request)
	if rejected.OK || rejected.Error == nil || rejected.Error.Code != "invalid_request" {
		t.Fatalf("unknown field response = %#v", rejected)
	}
}
