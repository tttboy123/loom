package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
)

func TestProductOperationalDiagnosticsAcceptsClosedVaultStages(t *testing.T) {
	stages := []string{
		credentials.CredentialStageVaultKeyLoad,
		credentials.CredentialStageVaultOpen,
		credentials.CredentialStageVaultEncrypt,
		credentials.CredentialStageVaultCommit,
		credentials.CredentialStageVaultDecrypt,
		credentials.CredentialStageVaultAADValidation,
		credentials.CredentialStageVaultRotation,
		credentials.CredentialStageVaultRecovery,
		credentials.CredentialStageVaultExport,
		credentials.CredentialStageLeaseIssue,
		credentials.CredentialStageLeaseExpire,
		credentials.CredentialStageLeaseRevoke,
		credentials.CredentialStageMigrationRead,
		credentials.CredentialStageMigrationCommit,
		credentials.CredentialStageMigrationCleanup,
		productAgentInputStage,
		"agent_attempt_reconcile",
	}
	for _, stage := range stages {
		if !productOperationalDiagnosticStage(stage) {
			t.Fatalf("Vault stage %q rejected", stage)
		}
	}
}

func TestProductAttemptRecoveryTerminalReconciliationPersistsContentFreeAuthorizationClosure(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 14, 13, 0, 0, 0, time.UTC)
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(root, "state", "loom.db"),
		productOperationalDiagnosticsMaximum,
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	report := productAttemptRecoveryTerminalReport{
		TerminalRuns:  1,
		RevokedGrants: 1,
		Outcomes: []productAttemptRecoveryTerminalOutcome{{
			WorkItemID: "work-recovered", RunID: "run-recovered",
			ClaimGeneration: 4, RuntimeInstanceID: "runtime-recovered",
			AgentInstanceID: "agent-recovered", ProviderID: "deepseek",
			ProviderAccountID: "deepseek.primary", ModelID: "deepseek-chat",
			ExecutionBindingDigest: strings.Repeat("a", 64),
		}},
	}
	incidentID := "55555555-5555-4555-8555-555555555555"
	if err := recordProductAttemptRecoveryTerminalReconciliation(
		context.Background(), store, func() time.Time { return now }, incidentID, report,
	); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, "diagnostics", "operational.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"api-key", "authorization_header", "bearer ", "prompt", "provider_response",
		"ciphertext",
	} {
		if strings.Contains(strings.ToLower(string(contents)), forbidden) {
			t.Fatalf("authorization reconciliation diagnostic contains %q: %s", forbidden, contents)
		}
	}
	var record productOperationalDiagnosticRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.Operation != "authorization_reconcile" ||
		record.IncidentID != incidentID || record.Stage != "agent_attempt_reconcile" ||
		record.Result != "succeeded" || record.ErrorCode != "" || record.Retryable ||
		record.ProviderID != "deepseek" ||
		record.ProviderAccountID != "deepseek.primary" ||
		record.ModelID != "deepseek-chat" || record.WorkItemID != "work-recovered" ||
		record.RunID != "run-recovered" || record.ClaimGeneration != 4 ||
		record.RuntimeInstanceID != "runtime-recovered" ||
		record.AgentID != "agent-recovered" ||
		record.ExecutionBindingDigest != strings.Repeat("a", 64) ||
		record.CapsuleDigest != "" || record.OccurredAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("record = %#v", record)
	}
}

func TestProductOperationalDiagnosticsRecordsContentFreeContextRetrieval(t *testing.T) {
	root := t.TempDir()
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(root, "state", "loom.db"),
		productOperationalDiagnosticsMaximum,
		func() time.Time { return time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	audit := contextcapsule.RetrievalAudit{
		IncidentID: "incident-context-1", WorkItemID: "work-1", RunID: "run-1",
		ClaimGeneration: 2, RuntimeInstanceID: "runtime-1",
		ExecutionBindingDigest: strings.Repeat("a", 64),
		CapsuleDigest:          strings.Repeat("b", 64), ItemID: "diff-detail",
		ContentDigest: strings.Repeat("c", 64), AgentID: "agent-reviewer",
		RoleID: "reviewer", ArtifactRef: "artifact:diff-1", Result: "disclosed",
	}
	if err := store.RecordContextRetrieval(context.Background(), audit); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, "diagnostics", "operational.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"private diff body", "prompt", "provider_response", "authorization",
	} {
		if strings.Contains(strings.ToLower(string(contents)), forbidden) {
			t.Fatalf("context retrieval diagnostic contains forbidden content %q: %s", forbidden, contents)
		}
	}
	var record productOperationalDiagnosticRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.Operation != "context_retrieval" || record.Stage != "context_retrieval" ||
		record.Result != "succeeded" || record.CapsuleDigest != audit.CapsuleDigest ||
		record.ContextItemDigest != audit.ContentDigest || record.AgentID != audit.AgentID ||
		record.ExecutionBindingDigest != audit.ExecutionBindingDigest {
		t.Fatalf("record = %#v", record)
	}

	audit.Result = "denied"
	audit.IncidentID = "incident-context-2"
	if err := store.RecordContextRetrieval(context.Background(), audit); err != nil {
		t.Fatal(err)
	}
}

func TestProductOperationalDiagnosticsRecordsContentFreeToolCallIdentity(t *testing.T) {
	root := t.TempDir()
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(root, "state", "loom.db"),
		productOperationalDiagnosticsMaximum,
		func() time.Time { return time.Date(2026, 8, 14, 10, 30, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	privateCommand := "printf must-not-enter-operational-diagnostics"
	diagnostic := productAttemptToolDiagnostic{
		IncidentID: "incident-tool-1", ProviderID: "deepseek",
		ProviderAccountID: "deepseek.primary", ModelID: "deepseek-chat",
		WorkItemID: "work-tool-1", RunID: "run-tool-1", ClaimGeneration: 3,
		RuntimeInstanceID: "runtime-tool-1", AgentInstanceID: "agent-tool-1",
		ExecutionBindingDigest: strings.Repeat("a", 64),
		CapsuleDigest:          strings.Repeat("b", 64),
		CallID:                 "call-tool-1",
		Execution: execution.ToolExecutionDiagnostic{
			ExecutionID: "execution-tool-1", JobID: "work-tool-1",
			CallDigest: strings.Repeat("c", 64), Tool: permissions.ToolBash,
			OperationID: "operation-tool-1", CorrelationID: "incident-tool-1",
			Stage: execution.ToolStageDispatch, Elapsed: 25 * time.Millisecond,
			Result: execution.ToolDiagnosticSucceeded,
		},
	}
	if err := store.RecordAttemptToolDiagnostic(context.Background(), diagnostic); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, "diagnostics", "operational.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(contents, []byte(privateCommand)) ||
		bytes.Contains(bytes.ToLower(contents), []byte("command")) ||
		bytes.Contains(bytes.ToLower(contents), []byte("prompt")) {
		t.Fatalf("tool diagnostic leaked private content: %s", contents)
	}
	var record productOperationalDiagnosticRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.Operation != "tool_call" || record.IncidentID != diagnostic.IncidentID ||
		record.ProviderAccountID != diagnostic.ProviderAccountID ||
		record.WorkItemID != diagnostic.WorkItemID || record.RunID != diagnostic.RunID ||
		record.AgentID != diagnostic.AgentInstanceID ||
		record.ExecutionBindingDigest != diagnostic.ExecutionBindingDigest ||
		record.CapsuleDigest != diagnostic.CapsuleDigest ||
		record.ToolCallID != diagnostic.CallID ||
		record.ExecutionID != diagnostic.Execution.ExecutionID ||
		record.CallDigest != diagnostic.Execution.CallDigest ||
		record.Tool != string(permissions.ToolBash) ||
		record.Stage != string(execution.ToolStageDispatch) ||
		record.ElapsedMS != 25 || record.Result != "succeeded" {
		t.Fatalf("tool diagnostic record=%#v", record)
	}
}

func TestProductOperationalDiagnosticsClassifiesVaultRotation(t *testing.T) {
	record := productOperationalDiagnosticFromResponse(
		time.Date(2026, 8, 10, 15, 0, 0, 0, time.UTC),
		localipc.Request{
			RequestID: "vault-rotation-incident",
			Method:    "credential_vault_rotate",
			Params:    json.RawMessage(`{}`),
		},
		localipc.Response{OK: true, Result: json.RawMessage(`{}`)},
		25*time.Millisecond,
	)
	if record.Operation != "credential_vault_rotate" ||
		record.Stage != credentials.CredentialStageVaultRotation ||
		record.Result != "succeeded" || record.IncidentID != "vault-rotation-incident" {
		t.Fatalf("rotation diagnostic = %#v", record)
	}
}

func TestProductOperationalDiagnosticsClassifiesVaultLockAndUnlock(t *testing.T) {
	for _, test := range []struct {
		method string
		stage  string
	}{
		{method: "credential_vault_lock", stage: credentials.CredentialStageLeaseRevoke},
		{method: "credential_vault_unlock", stage: credentials.CredentialStageVaultOpen},
	} {
		record := productOperationalDiagnosticFromResponse(
			time.Date(2026, 8, 10, 15, 0, 0, 0, time.UTC),
			localipc.Request{
				RequestID: "vault-lifecycle-incident",
				Method:    test.method,
				Params:    json.RawMessage(`{}`),
			},
			localipc.Response{OK: true, Result: json.RawMessage(`{}`)},
			25*time.Millisecond,
		)
		if record.Operation != test.method || record.Stage != test.stage ||
			record.Result != "succeeded" {
			t.Fatalf("%s diagnostic = %#v", test.method, record)
		}
	}
}

func TestProductOperationalDiagnosticsClassifiesVaultRecovery(t *testing.T) {
	record := productOperationalDiagnosticFromResponse(
		time.Unix(1, 0).UTC(),
		localipc.Request{
			RequestID: "vault-reset-incident-1",
			Method:    "credential_vault_reset",
			Params:    json.RawMessage(`{"confirmation":"reset_recovery_vault"}`),
		},
		localipc.Response{OK: true},
		time.Millisecond,
	)
	if record.Operation != "credential_vault_reset" ||
		record.Stage != credentials.CredentialStageVaultRecovery ||
		record.Result != "succeeded" {
		t.Fatalf("Vault recovery diagnostic = %#v", record)
	}
}

func TestProductOperationalDiagnosticsClassifiesVaultExport(t *testing.T) {
	record := productOperationalDiagnosticFromResponse(
		time.Unix(1, 0).UTC(),
		localipc.Request{
			RequestID: "vault-export-incident-1",
			Method:    "credential_vault_export",
			Params: json.RawMessage(
				`{"passphrase":"cmVkYWN0ZWQ=","destination":"/tmp/backup.loomvault"}`,
			),
		},
		localipc.Response{OK: true},
		time.Millisecond,
	)
	if record.Operation != "credential_vault_export" ||
		record.Stage != credentials.CredentialStageVaultExport ||
		record.ProviderID != "" || record.Result != "succeeded" {
		t.Fatalf("Vault export diagnostic = %#v", record)
	}
	encoded, err := json.Marshal(record)
	if err != nil || bytes.Contains(encoded, []byte("cmVkYWN0ZWQ=")) ||
		bytes.Contains(encoded, []byte("/tmp/backup.loomvault")) {
		t.Fatalf("Vault export diagnostic leaked request data: %s, %v", encoded, err)
	}
}

func TestProductOperationalDiagnosticsRecordsSafeCredentialTerminalEvent(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(stateDir, "loom.db")
	store, err := newProductOperationalDiagnosticStore(
		statePath,
		4<<10,
		func() time.Time { return time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := store.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		return localipc.Response{
			OK: false,
			Error: &localipc.ProtocolError{
				Code:        "credential_unavailable",
				Message:     "credential unavailable",
				Recoverable: true,
				Stage:       "helper_authorization",
			},
		}
	}))
	request := localipc.Request{
		RequestID: "loom-swift-11111111-1111-4111-8111-111111111111",
		Method:    "credential_configure",
		Params: json.RawMessage(
			`{"provider_id":"deepseek","credential_reference":"","expected_revision":0,"secret":"sk-must-never-appear"}`,
		),
	}
	response := handler.Handle(context.Background(), request)
	if response.OK || response.Error == nil {
		t.Fatalf("response = %#v", response)
	}

	path := filepath.Join(root, "diagnostics", "operational.jsonl")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "sk-must-never-appear") ||
		strings.Contains(string(contents), "credential_reference") {
		t.Fatalf("diagnostics contain forbidden credential data: %s", contents)
	}
	var record productOperationalDiagnosticRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.IncidentID != request.RequestID ||
		record.Operation != request.Method ||
		record.ProviderID != "deepseek" ||
		record.Stage != "helper_authorization" ||
		record.Result != "failed" ||
		record.ErrorCode != "credential_unavailable" ||
		!record.Retryable || record.OccurredAt != "2026-08-09T12:00:00Z" {
		t.Fatalf("record = %#v", record)
	}
}

func TestProductOperationalDiagnosticsRecordsSafeProviderAccountPolicyEvent(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(stateDir, "loom.db"),
		4<<10,
		func() time.Time { return time.Date(2026, 8, 11, 11, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := store.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		return localipc.Response{OK: true, Result: []byte(`{"policy_available":true}`)}
	}))
	request := localipc.Request{
		RequestID: "loom-swift-policy-11111111-1111-4111-8111-111111111111",
		Method:    "provider_account_policy_configure",
		Params: json.RawMessage(
			`{"provider_id":"deepseek","provider_account_id":"deepseek.work","expected_revision":2,"maximum_concurrent_attempts":4,"dispatch_window_seconds":60,"maximum_dispatch_starts":20,"maximum_assigned_budget_units":12000,"operation_id":"must-not-enter-diagnostics"}`,
		),
	}
	response := handler.Handle(context.Background(), request)
	if !response.OK {
		t.Fatalf("response = %#v", response)
	}

	contents, err := os.ReadFile(filepath.Join(root, "diagnostics", "operational.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, forbidden := range []string{
		"must-not-enter-diagnostics", "maximum_concurrent_attempts",
		"maximum_assigned_budget_units", "operation_id",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("diagnostics contain forbidden policy input %q: %s", forbidden, text)
		}
	}
	var record productOperationalDiagnosticRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.IncidentID != request.RequestID ||
		record.Operation != request.Method ||
		record.ProviderID != "deepseek" ||
		record.ProviderAccountID != "deepseek.work" ||
		record.Stage != "projection_refresh" ||
		record.Result != "succeeded" || record.ErrorCode != "" || record.Retryable {
		t.Fatalf("record = %#v", record)
	}
}

func TestProductOperationalDiagnosticsRecordsSafeRemoteToolEnrollmentEvent(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(stateDir, "loom.db"),
		4<<10,
		func() time.Time { return time.Date(2026, 8, 15, 11, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := store.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		return localipc.Response{OK: true, Result: []byte(`{"enrollment_available":true}`)}
	}))
	request := localipc.Request{
		RequestID: "loom-swift-enrollment-11111111-1111-4111-8111-111111111111",
		Method:    "remote_tool_backend_enrollment_configure",
		Params: json.RawMessage(`{
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "enrollment_id":"search-deepseek-work",
          "endpoint_fingerprint":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "allowed_tools":["private_search_tool"],
          "operation_id":"must-not-enter-enrollment-diagnostics"
        }`),
	}
	response := handler.Handle(context.Background(), request)
	if !response.OK {
		t.Fatalf("response = %#v", response)
	}

	contents, err := os.ReadFile(filepath.Join(root, "diagnostics", "operational.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, forbidden := range []string{
		"search-deepseek-work", "aaaaaaaaaaaaaaaa",
		"private_search_tool", "must-not-enter-enrollment-diagnostics",
		"endpoint_fingerprint", "allowed_tools", "operation_id",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("diagnostics contain forbidden enrollment input %q: %s", forbidden, text)
		}
	}
	var record productOperationalDiagnosticRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.IncidentID != request.RequestID ||
		record.Operation != request.Method ||
		record.ProviderID != "deepseek" ||
		record.ProviderAccountID != "deepseek.work" ||
		record.Stage != "projection_refresh" ||
		record.Result != "succeeded" || record.ErrorCode != "" || record.Retryable {
		t.Fatalf("record = %#v", record)
	}
}

func TestProductOperationalDiagnosticsRecordsSafeConversationConflict(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(stateDir, "loom.db"),
		4<<10,
		func() time.Time { return time.Date(2026, 8, 10, 17, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := store.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		return localipc.Response{OK: false, Error: &localipc.ProtocolError{
			Code: "conflict", Message: "conflict", Recoverable: true,
			Stage: "conversation_dispatch",
		}}
	}))
	request := localipc.Request{
		RequestID: "loom-chat-11111111-1111-4111-8111-111111111111",
		Method:    "chat_message",
		Params: json.RawMessage(
			`{"thread_id":"thread-codex","profile_id":"conversation-deepseek-r3","content":"must never appear"}`,
		),
	}
	handler.Handle(context.Background(), request)
	contents, err := os.ReadFile(filepath.Join(root, "diagnostics", "operational.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "must never appear") ||
		strings.Contains(string(contents), `"content"`) {
		t.Fatalf("diagnostics contain conversation content: %s", contents)
	}
	var record productOperationalDiagnosticRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.Operation != "chat_message" ||
		record.ThreadID != "thread-codex" ||
		record.ProfileID != "conversation-deepseek-r3" ||
		record.Stage != "conversation_dispatch" ||
		record.Result != "failed" || record.ErrorCode != "conflict" ||
		!record.Retryable {
		t.Fatalf("record = %#v", record)
	}
}

func TestProductOperationalDiagnosticsRecordsContentFreeAttemptRecoveryRequest(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(stateDir, "loom.db"),
		4<<10,
		func() time.Time { return time.Date(2026, 8, 14, 18, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := store.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		return localipc.Response{OK: false, Error: &localipc.ProtocolError{
			Code: "conflict", Message: "conflict", Recoverable: true,
			Stage: "agent_attempt_reconcile",
		}}
	}))
	const incidentID = "loom-agent-recovery-11111111-1111-4111-8111-111111111111"
	handler.Handle(context.Background(), localipc.Request{
		RequestID: incidentID,
		Method:    "agent_attempt_recovery",
		Params: json.RawMessage(
			`{"schema_version":1,"operation":"confirm","decision_id":"22222222-2222-4222-8222-222222222222","principal_id":"local-user","candidate_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","capability_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`,
		),
	})
	contents, err := os.ReadFile(filepath.Join(root, "diagnostics", "operational.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"local-user", "candidate_digest", "capability_digest", "prompt",
		"provider_response", "credential_reference", "authorization",
	} {
		if strings.Contains(strings.ToLower(string(contents)), forbidden) {
			t.Fatalf("attempt recovery diagnostic contains %q: %s", forbidden, contents)
		}
	}
	var record productOperationalDiagnosticRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.Operation != "agent_attempt_recovery" ||
		record.IncidentID != incidentID || record.Stage != "agent_attempt_reconcile" ||
		record.Result != "failed" || record.ErrorCode != "conflict" ||
		!record.Retryable || record.ProviderID != "" || record.WorkItemID != "" ||
		record.ExecutionBindingDigest != "" || record.CapsuleDigest != "" {
		t.Fatalf("record = %#v", record)
	}
}

func TestProductOperationalDiagnosticsRecordsContentFreeToolRecoveryRequest(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(stateDir, "loom.db"),
		4<<10,
		func() time.Time { return time.Date(2026, 8, 14, 20, 30, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := store.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		return localipc.Response{OK: false, Error: &localipc.ProtocolError{
			Code: "conflict", Message: "conflict", Recoverable: true,
			Stage: "tool_recovery",
		}}
	}))
	const incidentID = "loom-tool-recovery-11111111-1111-4111-8111-111111111111"
	handler.Handle(context.Background(), localipc.Request{
		RequestID: incidentID,
		Method:    "tool_recovery",
		Params: json.RawMessage(
			`{"schema_version":1,"operation":"resolve","decision_id":"22222222-2222-4222-8222-222222222222","principal_id":"local-user","action":"abort_attempt","candidate_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
		),
	})
	contents, err := os.ReadFile(filepath.Join(root, "diagnostics", "operational.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"local-user", "candidate_digest", "decision_id", "prompt",
		"provider_response", "credential_reference", "authorization",
	} {
		if strings.Contains(strings.ToLower(string(contents)), forbidden) {
			t.Fatalf("tool recovery diagnostic contains %q: %s", forbidden, contents)
		}
	}
	var record productOperationalDiagnosticRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.Operation != "tool_recovery" || record.IncidentID != incidentID ||
		record.Stage != "tool_recovery" || record.Result != "failed" ||
		record.ErrorCode != "conflict" || !record.Retryable ||
		record.ExecutionID != "" || record.CallDigest != "" {
		t.Fatalf("record = %#v", record)
	}
}

func TestProductOperationalDiagnosticsUsesConversationAttemptTerminalFailure(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(stateDir, "loom.db"),
		4<<10,
		func() time.Time { return time.Date(2026, 8, 11, 4, 0, 1, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.setCredentialRuntime("vault"); err != nil {
		t.Fatal(err)
	}
	const incidentID = "loom-chat-22222222-2222-4222-8222-222222222222"
	result, err := json.Marshal(api.LocalProductChatThread{
		ThreadID: "thread-deepseek", ProfileID: "conversation-deepseek-r6",
		Attempts: []api.LocalProductConversationAttempt{{
			AttemptID: "attempt-1", IncidentID: incidentID,
			Status: "failed", FailureCode: "provider_rate_limit",
			FailureStage: "provider_rate_limit", HTTPStatus: 429,
			ProviderCode:      "rate_limit_exceeded",
			FailureMessage:    "Provider rate limit reached.",
			RetryAfterSeconds: 18, Retryable: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := store.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		return localipc.Response{OK: true, Result: result}
	}))
	handler.Handle(context.Background(), localipc.Request{
		RequestID: incidentID, Method: "chat_message",
		Params: json.RawMessage(
			`{"thread_id":"thread-deepseek","profile_id":"conversation-deepseek-r6","content":"must never appear"}`,
		),
	})
	contents, err := os.ReadFile(filepath.Join(root, "diagnostics", "operational.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "must never appear") ||
		strings.Contains(string(contents), `"content"`) ||
		strings.Contains(string(contents), "Provider rate limit reached") {
		t.Fatalf("diagnostics contain conversation content: %s", contents)
	}
	var record productOperationalDiagnosticRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.CredentialRuntime != "vault" ||
		record.CredentialHelperSpawnAttempts != credentials.ProductKeychainHelperSpawnAttempts() ||
		record.Result != "failed" || record.Stage != "provider_rate_limit" ||
		record.ErrorCode != "provider_rate_limit" || record.HTTPStatus != 429 ||
		record.ProviderErrorCode != "rate_limit_exceeded" ||
		record.RetryAfterSeconds != 18 || !record.Retryable {
		t.Fatalf("record = %#v", record)
	}
	if !strings.Contains(string(contents), `"credential_helper_spawn_attempts":`) {
		t.Fatalf("helper spawn attempt count omitted: %s", contents)
	}
}

func TestProductOperationalDiagnosticsRotatesBoundedFile(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(stateDir, "loom.db"),
		512,
		func() time.Time { return time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := store.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		return localipc.Response{OK: false, Error: &localipc.ProtocolError{
			Code: "invalid_request", Message: "invalid request",
		}}
	}))
	for index := 0; index < 8; index++ {
		handler.Handle(context.Background(), localipc.Request{
			RequestID: "loom-swift-11111111-1111-4111-8111-11111111111" + string(rune('0'+index)),
			Method:    "credential_configure",
			Params:    json.RawMessage(`{"provider_id":"deepseek","secret":"x"}`),
		})
	}
	path := filepath.Join(root, "diagnostics", "operational.jsonl")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 512 {
		t.Fatalf("active diagnostics size = %d", info.Size())
	}
	rotated, err := os.Stat(path + ".1")
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Mode().Perm() != 0o600 || rotated.Size() > 512 {
		t.Fatalf("rotated diagnostics = mode %o size %d", rotated.Mode().Perm(), rotated.Size())
	}
}

func TestProductOperationalDiagnosticsRecordsSafeAgentAttempt(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(stateDir, "loom.db"),
		4<<10,
		func() time.Time { return time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.setCredentialRuntime("vault"); err != nil {
		t.Fatal(err)
	}
	err = store.RecordAgentAttemptDiagnostic(
		context.Background(),
		nativeadapter.AgentAttemptDiagnostic{
			OccurredAt: time.Date(2026, 8, 10, 11, 59, 59, 0, time.UTC),
			IncidentID: "22222222-2222-4222-8222-222222222222",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			ModelID: "deepseek-chat", WorkItemID: "work-safe",
			RunID: "run-safe", ClaimGeneration: 3,
			RuntimeInstanceID: "runtime-safe", AgentInstanceID: "agent-safe",
			ExecutionBindingDigest: strings.Repeat("a", 64),
			ContextCapsuleDigest:   strings.Repeat("b", 64),
			Stage:                  "context_delivery_reconcile",
			Elapsed:                125 * time.Millisecond, Result: "failed",
			ErrorCode: "attempt_payload_unavailable", Retryable: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, "diagnostics", "operational.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "credential-ref") ||
		strings.Contains(string(contents), "prompt") ||
		strings.Contains(string(contents), "private") {
		t.Fatalf("unsafe Agent diagnostic = %s", contents)
	}
	var record productOperationalDiagnosticRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.CredentialRuntime != "vault" ||
		record.CredentialHelperSpawnAttempts != credentials.ProductKeychainHelperSpawnAttempts() ||
		record.Operation != "agent_attempt" || record.ProviderID != "deepseek" ||
		record.ProviderAccountID != "deepseek.primary" ||
		record.ModelID != "deepseek-chat" || record.WorkItemID != "work-safe" ||
		record.RunID != "run-safe" || record.ClaimGeneration != 3 ||
		record.RuntimeInstanceID != "runtime-safe" || record.AgentID != "agent-safe" ||
		record.ExecutionBindingDigest != strings.Repeat("a", 64) ||
		record.CapsuleDigest != strings.Repeat("b", 64) ||
		record.Stage != "context_delivery_reconcile" ||
		record.ErrorCode != "attempt_payload_unavailable" ||
		record.ElapsedMS != 125 || !record.Retryable {
		t.Fatalf("record = %#v", record)
	}
}

func TestProductOperationalDiagnosticsRejectsUnknownCredentialRuntime(t *testing.T) {
	root := t.TempDir()
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(root, "state", "loom.db"),
		productOperationalDiagnosticsMaximum,
		func() time.Time { return time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.setCredentialRuntime("keychain"); err == nil {
		t.Fatal("unknown credential runtime accepted")
	}
}

func TestProductOperationalDiagnosticsQueriesExactAgentAttemptIdentity(t *testing.T) {
	root := t.TempDir()
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(root, "state", "loom.db"),
		productOperationalDiagnosticsMaximum,
		func() time.Time { return time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range []productOperationalDiagnosticRecord{
		{
			SchemaVersion: 1, OccurredAt: "2026-08-11T10:00:00Z",
			IncidentID: "incident-shared-1", Operation: "agent_attempt",
			ProviderID: "anthropic", ProviderAccountID: "anthropic.production",
			ModelID: "claude-sonnet", Stage: "provider_auth",
			ElapsedMS: 40, Result: "failed", ErrorCode: "provider_rejected",
		},
		{
			SchemaVersion: 1, OccurredAt: "2026-08-11T10:00:01Z",
			IncidentID: "incident-shared-1", Operation: "agent_attempt",
			ProviderID: "anthropic", ProviderAccountID: "anthropic.staging",
			ModelID: "claude-sonnet", Stage: "provider_rate_limit",
			ElapsedMS: 41, Result: "failed", ErrorCode: "rate_limited", Retryable: true,
		},
		{
			SchemaVersion: 1, OccurredAt: "2026-08-11T10:00:02Z",
			IncidentID: "incident-deepseek-timeout-1", Operation: "agent_attempt",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			ModelID: "deepseek-chat", Stage: "provider_connect",
			ElapsedMS: 45_000, Result: "failed", ErrorCode: "timeout", Retryable: true,
		},
	} {
		if err := store.append(record); err != nil {
			t.Fatal(err)
		}
	}

	summaries, err := store.AgentAttemptDiagnostics(
		context.Background(),
		[]api.AgentAttemptDiagnosticQuery{
			{
				IncidentID: "incident-shared-1", ProviderID: "anthropic",
				ProviderAccountID: "anthropic.production", ModelID: "claude-sonnet",
			},
			{
				IncidentID: "incident-missing-1", ProviderID: "deepseek",
				ProviderAccountID: "deepseek.primary", ModelID: "deepseek-chat",
			},
			{
				IncidentID: "incident-deepseek-timeout-1", ProviderID: "deepseek",
				ProviderAccountID: "deepseek.primary", ModelID: "deepseek-chat",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 {
		t.Fatalf("summaries = %#v", summaries)
	}
	want := api.AgentAttemptDiagnosticSummary{
		IncidentID: "incident-shared-1", ProviderID: "anthropic",
		ProviderAccountID: "anthropic.production", ModelID: "claude-sonnet",
		FailureStage: "provider_auth", FailureCode: "provider_rejected",
	}
	if summaries[0] != want {
		t.Fatalf("summary = %#v, want %#v", summaries[0], want)
	}
	timeoutWant := api.AgentAttemptDiagnosticSummary{
		IncidentID: "incident-deepseek-timeout-1", ProviderID: "deepseek",
		ProviderAccountID: "deepseek.primary", ModelID: "deepseek-chat",
		FailureStage: "provider_connect", FailureCode: "timeout", Retryable: true,
	}
	if summaries[1] != timeoutWant {
		t.Fatalf("timeout summary = %#v, want %#v", summaries[1], timeoutWant)
	}
	if err := store.append(productOperationalDiagnosticRecord{
		SchemaVersion: 1, OccurredAt: "2026-08-11T10:00:02Z",
		IncidentID: "incident-shared-1", Operation: "agent_attempt",
		ProviderID: "anthropic", ProviderAccountID: "anthropic.production",
		ModelID: "claude-sonnet", Stage: "agent_attempt_dispatch",
		ElapsedMS: 42, Result: "succeeded",
	}); err != nil {
		t.Fatal(err)
	}
	summaries, err = store.AgentAttemptDiagnostics(
		context.Background(),
		[]api.AgentAttemptDiagnosticQuery{{
			IncidentID: "incident-shared-1", ProviderID: "anthropic",
			ProviderAccountID: "anthropic.production", ModelID: "claude-sonnet",
		}},
	)
	if err != nil || len(summaries) != 0 {
		t.Fatalf("latest successful diagnostic retained stale failure = %#v, %v", summaries, err)
	}
}
