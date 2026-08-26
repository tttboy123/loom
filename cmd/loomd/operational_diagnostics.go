package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
)

const (
	productOperationalDiagnosticsMaximum   = int64(512 << 10)
	productDiagnosticStageDaemonAdmission  = "daemon_admission"
	productCredentialRuntimeVault          = "vault"
	productCredentialRuntimeExplicitLegacy = "explicit_legacy"
)

type productOperationalDiagnosticRecord struct {
	SchemaVersion                   int    `json:"schema_version"`
	OccurredAt                      string `json:"occurred_at"`
	IncidentID                      string `json:"incident_id"`
	Operation                       string `json:"operation"`
	CredentialRuntime               string `json:"credential_runtime,omitempty"`
	CredentialHelperSpawnAttempts   uint64 `json:"credential_helper_spawn_attempts"`
	ProviderID                      string `json:"provider_id,omitempty"`
	ProviderAccountID               string `json:"provider_account_id,omitempty"`
	CredentialRevision              int64  `json:"credential_revision,omitempty"`
	ModelID                         string `json:"model_id,omitempty"`
	ReasoningEffort                 string `json:"reasoning_effort,omitempty"`
	ThreadID                        string `json:"thread_id,omitempty"`
	ProfileID                       string `json:"profile_id,omitempty"`
	GatewayEventSchemaVersion       int    `json:"gateway_event_schema_version,omitempty"`
	GatewayInstanceID               string `json:"gateway_instance_id,omitempty"`
	GatewayConfiguredHarnessVersion int    `json:"gateway_configured_harness_version,omitempty"`
	GatewayBackendVersion           int    `json:"gateway_backend_version,omitempty"`
	GatewayEventSequence            uint64 `json:"gateway_event_sequence,omitempty"`
	GatewayEventType                string `json:"gateway_event_type,omitempty"`
	SessionID                       string `json:"session_id,omitempty"`
	HarnessID                       string `json:"harness_id,omitempty"`
	BackendID                       string `json:"backend_id,omitempty"`
	SegmentID                       string `json:"segment_id,omitempty"`
	WorkspaceID                     string `json:"workspace_id,omitempty"`
	WorkspaceDigest                 string `json:"workspace_digest,omitempty"`
	SegmentContextCapsuleDigest     string `json:"segment_context_capsule_digest,omitempty"`
	GovernancePolicyDigest          string `json:"governance_policy_digest,omitempty"`
	RouteTransitionReviewDigest     string `json:"route_transition_review_digest,omitempty"`
	ResponseID                      string `json:"response_id,omitempty"`
	WorkItemID                      string `json:"work_item_id,omitempty"`
	RunID                           string `json:"run_id,omitempty"`
	ClaimGeneration                 int64  `json:"claim_generation,omitempty"`
	RuntimeInstanceID               string `json:"runtime_instance_id,omitempty"`
	ExecutionBindingDigest          string `json:"execution_binding_digest,omitempty"`
	CapsuleDigest                   string `json:"context_capsule_digest,omitempty"`
	ContextItemID                   string `json:"context_item_id,omitempty"`
	ContextItemDigest               string `json:"context_item_digest,omitempty"`
	AgentID                         string `json:"agent_id,omitempty"`
	RoleID                          string `json:"role_id,omitempty"`
	ArtifactRef                     string `json:"artifact_ref,omitempty"`
	ExecutionID                     string `json:"execution_id,omitempty"`
	ToolCallID                      string `json:"tool_call_id,omitempty"`
	Tool                            string `json:"tool,omitempty"`
	CallDigest                      string `json:"call_digest,omitempty"`
	ToolOperationID                 string `json:"tool_operation_id,omitempty"`
	CompositionSnapshotDigest       string `json:"composition_snapshot_digest,omitempty"`
	BundleID                        string `json:"bundle_id,omitempty"`
	BundleVersion                   string `json:"bundle_version,omitempty"`
	ScopeKind                       string `json:"scope_kind,omitempty"`
	ScopeID                         string `json:"scope_id,omitempty"`
	ScopeDigest                     string `json:"scope_digest,omitempty"`
	Stage                           string `json:"stage"`
	ElapsedMS                       int64  `json:"elapsed_ms"`
	Result                          string `json:"result"`
	ErrorCode                       string `json:"error_code,omitempty"`
	HTTPStatus                      int    `json:"http_status,omitempty"`
	ProviderErrorCode               string `json:"provider_error_code,omitempty"`
	RetryAfterSeconds               int64  `json:"retry_after_seconds,omitempty"`
	Retryable                       bool   `json:"retryable"`
}

func (record productOperationalDiagnosticRecord) MarshalJSON() ([]byte, error) {
	type diagnosticAlias productOperationalDiagnosticRecord
	if record.GatewayEventSchemaVersion != harnessgateway.EventSchemaVersion {
		return json.Marshal(diagnosticAlias(record))
	}
	return json.Marshal(struct {
		diagnosticAlias
		ProviderAccountID      string `json:"provider_account_id"`
		CredentialRevision     int64  `json:"credential_revision"`
		GovernancePolicyDigest string `json:"governance_policy_digest"`
	}{
		diagnosticAlias:        diagnosticAlias(record),
		ProviderAccountID:      record.ProviderAccountID,
		CredentialRevision:     record.CredentialRevision,
		GovernancePolicyDigest: record.GovernancePolicyDigest,
	})
}

type productAttemptToolDiagnostic struct {
	IncidentID             string
	ProviderID             string
	ProviderAccountID      string
	ModelID                string
	WorkItemID             string
	RunID                  string
	ClaimGeneration        int64
	RuntimeInstanceID      string
	AgentInstanceID        string
	ExecutionBindingDigest string
	CapsuleDigest          string
	CallID                 string
	Execution              execution.ToolExecutionDiagnostic
}

type productAttemptToolDiagnosticRecorder interface {
	RecordAttemptToolDiagnostic(context.Context, productAttemptToolDiagnostic) error
}

var _ productAttemptToolDiagnosticRecorder = (*productOperationalDiagnosticStore)(nil)

type productOperationalDiagnosticStore struct {
	path              string
	maximum           int64
	now               func() time.Time
	credentialRuntime string
	mu                sync.Mutex
}

type productOperationalDiagnosticSink interface {
	operationalNow() time.Time
	credentialRuntimeValue() string
	append(productOperationalDiagnosticRecord) error
}

const productOperationalDiagnosticBootstrapMaximum = 256

type productOperationalDiagnosticBootstrap struct {
	statePath string
	maximum   int64
	now       func() time.Time

	mu                sync.Mutex
	credentialRuntime string
	pending           []productOperationalDiagnosticRecord
	store             *productOperationalDiagnosticStore
	failed            bool
}

func newProductOperationalDiagnosticBootstrap(
	statePath string,
	maximum int64,
	now func() time.Time,
) (*productOperationalDiagnosticBootstrap, error) {
	if !filepath.IsAbs(statePath) || filepath.Clean(statePath) != statePath ||
		maximum < 256 || now == nil {
		return nil, errors.New("invalid operational diagnostics")
	}
	return &productOperationalDiagnosticBootstrap{
		statePath: statePath,
		maximum:   maximum,
		now:       now,
		pending:   make([]productOperationalDiagnosticRecord, 0, 32),
	}, nil
}

func (bootstrap *productOperationalDiagnosticBootstrap) setCredentialRuntime(
	value string,
) error {
	if bootstrap == nil || !validProductCredentialRuntime(value) {
		return errors.New("invalid credential runtime")
	}
	bootstrap.mu.Lock()
	defer bootstrap.mu.Unlock()
	if bootstrap.failed ||
		bootstrap.credentialRuntime != "" && bootstrap.credentialRuntime != value {
		return errors.New("credential runtime already bound")
	}
	if bootstrap.store != nil {
		return bootstrap.store.setCredentialRuntime(value)
	}
	bootstrap.credentialRuntime = value
	return nil
}

func (bootstrap *productOperationalDiagnosticBootstrap) bind(
	store *productOperationalDiagnosticStore,
) error {
	if bootstrap == nil || store == nil {
		return errors.New("operational diagnostics unavailable")
	}
	bootstrap.mu.Lock()
	defer bootstrap.mu.Unlock()
	if bootstrap.failed || bootstrap.store != nil {
		return errors.New("operational diagnostics already bound")
	}
	if bootstrap.credentialRuntime != "" {
		if err := store.setCredentialRuntime(bootstrap.credentialRuntime); err != nil {
			bootstrap.failed = true
			return err
		}
	}
	for _, record := range bootstrap.pending {
		if err := store.append(record); err != nil {
			bootstrap.failed = true
			return err
		}
	}
	clear(bootstrap.pending)
	bootstrap.pending = nil
	bootstrap.store = store
	return nil
}

func (bootstrap *productOperationalDiagnosticBootstrap) operationalNow() time.Time {
	if bootstrap == nil || bootstrap.now == nil {
		return time.Time{}
	}
	return bootstrap.now()
}

func (bootstrap *productOperationalDiagnosticBootstrap) credentialRuntimeValue() string {
	if bootstrap == nil {
		return ""
	}
	bootstrap.mu.Lock()
	defer bootstrap.mu.Unlock()
	if bootstrap.store != nil {
		return bootstrap.store.credentialRuntimeValue()
	}
	return bootstrap.credentialRuntime
}

func (bootstrap *productOperationalDiagnosticBootstrap) append(
	record productOperationalDiagnosticRecord,
) error {
	if bootstrap == nil || !validProductOperationalDiagnosticRecord(record) {
		return errors.New("invalid operational diagnostic")
	}
	bootstrap.mu.Lock()
	defer bootstrap.mu.Unlock()
	if bootstrap.failed {
		return errors.New("operational diagnostics unavailable")
	}
	if bootstrap.store != nil {
		return bootstrap.store.append(record)
	}
	if len(bootstrap.pending) >= productOperationalDiagnosticBootstrapMaximum {
		bootstrap.failed = true
		clear(bootstrap.pending)
		bootstrap.pending = nil
		return errors.New("operational diagnostics bootstrap full")
	}
	bootstrap.pending = append(bootstrap.pending, record)
	return nil
}

func (store *productOperationalDiagnosticStore) setCredentialRuntime(
	value string,
) error {
	if store == nil || !validProductCredentialRuntime(value) {
		return errors.New("invalid credential runtime")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.credentialRuntime != "" && store.credentialRuntime != value {
		return errors.New("credential runtime already bound")
	}
	store.credentialRuntime = value
	return nil
}

func (store *productOperationalDiagnosticStore) credentialRuntimeValue() string {
	if store == nil {
		return ""
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.credentialRuntime
}

func (store *productOperationalDiagnosticStore) operationalNow() time.Time {
	if store == nil || store.now == nil {
		return time.Time{}
	}
	return store.now()
}

type productChatMigrationDiagnosticRecorder struct {
	store      productOperationalDiagnosticSink
	incidentID string

	mu          sync.Mutex
	lastFailure *api.LocalProductChatAvailabilityFailure
}

var _ api.LocalProductChatMigrationDiagnosticRecorder = (*productChatMigrationDiagnosticRecorder)(nil)

func newProductOperationalDiagnosticStore(
	statePath string,
	maximum int64,
	now func() time.Time,
) (*productOperationalDiagnosticStore, error) {
	if !filepath.IsAbs(statePath) || filepath.Clean(statePath) != statePath ||
		maximum < 256 || now == nil {
		return nil, errors.New("invalid operational diagnostics")
	}
	stateDir := filepath.Dir(statePath)
	root := stateDir
	if filepath.Base(stateDir) == "state" {
		root = filepath.Dir(stateDir)
	}
	directory := filepath.Join(root, "diagnostics")
	if err := ensureProductOperationalDirectory(directory); err != nil {
		return nil, err
	}
	return &productOperationalDiagnosticStore{
		path:    filepath.Join(directory, "operational.jsonl"),
		maximum: maximum,
		now:     now,
	}, nil
}

func ensureProductOperationalDirectory(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("operational diagnostics unavailable")
		}
		return os.Chmod(path, 0o700)
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("operational diagnostics unavailable")
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		return errors.New("operational diagnostics unavailable")
	}
	return nil
}

func (store *productOperationalDiagnosticStore) wrap(
	next localipc.Handler,
) localipc.Handler {
	return localipc.HandlerFunc(func(
		ctx context.Context,
		request localipc.Request,
	) localipc.Response {
		started := time.Now()
		response := next.Handle(ctx, request)
		if productIPCOperationalDiagnosticMethod(request.Method) {
			record := productOperationalDiagnosticFromResponse(
				store.now(),
				request,
				response,
				time.Since(started),
			)
			record.CredentialRuntime = store.credentialRuntimeValue()
			_ = store.append(record)
		}
		return response
	})
}

func productCredentialDiagnosticMethod(method string) bool {
	switch method {
	case "credential_configure", "credential_verify", "credential_replace",
		"credential_revoke", "credential_vault_rotate", "credential_vault_lock",
		"credential_vault_unlock", "credential_vault_reset":
		return true
	case "credential_vault_export":
		return true
	default:
		return false
	}
}

func productIPCOperationalDiagnosticMethod(method string) bool {
	return productCredentialDiagnosticMethod(method) ||
		method == "provider_account_policy_configure" ||
		method == "provider_model_rate_card_configure" ||
		method == "remote_tool_backend_enrollment_configure" ||
		method == "remote_tool_backend_enrollment_revoke" || method == "chat_message" ||
		method == "chat_response_cancel" ||
		method == "agent_attempt_recovery" || method == "tool_recovery"
}

func productOperationalDiagnosticMethod(method string) bool {
	return productIPCOperationalDiagnosticMethod(method) ||
		method == "agent_attempt" || method == "conversation_migration" ||
		method == "context_retrieval" || method == "tool_call" ||
		method == "composition" || method == "authorization_reconcile"
}

type productCompositionDiagnosticRecorder struct {
	store productOperationalDiagnosticSink

	mu  sync.Mutex
	err error
}

var _ composition.DiagnosticRecorder = (*productCompositionDiagnosticRecorder)(nil)

func newProductCompositionDiagnosticRecorder(
	store productOperationalDiagnosticSink,
) (*productCompositionDiagnosticRecorder, error) {
	if nilProductAssetPort(store) {
		return nil, errors.New("operational diagnostics unavailable")
	}
	return &productCompositionDiagnosticRecorder{store: store}, nil
}

func (recorder *productCompositionDiagnosticRecorder) RecordCompositionDiagnostic(
	ctx context.Context,
	diagnostic composition.DiagnosticRecord,
) error {
	if recorder == nil || nilProductAssetPort(recorder.store) || ctx == nil {
		return errors.New("operational diagnostics unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	record := productOperationalDiagnosticRecord{
		SchemaVersion: 1, OccurredAt: recorder.store.operationalNow().UTC().Format(time.RFC3339Nano),
		IncidentID: diagnostic.IncidentID, Operation: "composition",
		CredentialRuntime:         recorder.store.credentialRuntimeValue(),
		ProfileID:                 string(diagnostic.ProfileID),
		CompositionSnapshotDigest: diagnostic.SnapshotDigest,
		BundleID:                  diagnostic.BundleID, BundleVersion: diagnostic.BundleVersion,
		ScopeKind: string(diagnostic.ScopeKind), ScopeID: diagnostic.ScopeID,
		ScopeDigest: diagnostic.ScopeDigest, Stage: string(diagnostic.Stage),
		ElapsedMS: max(diagnostic.ElapsedMillis, 0), Result: diagnostic.Result,
		ErrorCode: diagnostic.ErrorCode, Retryable: diagnostic.Retryable,
	}
	err := recorder.store.append(record)
	if err != nil {
		recorder.mu.Lock()
		recorder.err = errors.Join(recorder.err, err)
		recorder.mu.Unlock()
	}
	return err
}

func (recorder *productCompositionDiagnosticRecorder) CompositionDiagnosticError() error {
	if recorder == nil {
		return errors.New("operational diagnostics unavailable")
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.err
}

func (store *productOperationalDiagnosticStore) RecordContextRetrieval(
	ctx context.Context,
	audit contextcapsule.RetrievalAudit,
) error {
	if store == nil || ctx == nil {
		return errors.New("operational diagnostics unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	record := productOperationalDiagnosticRecord{
		SchemaVersion: 1, OccurredAt: store.now().UTC().Format(time.RFC3339Nano),
		IncidentID: audit.IncidentID, Operation: "context_retrieval",
		CredentialRuntime: store.credentialRuntimeValue(),
		WorkItemID:        audit.WorkItemID, RunID: audit.RunID,
		ClaimGeneration:        audit.ClaimGeneration,
		RuntimeInstanceID:      audit.RuntimeInstanceID,
		ExecutionBindingDigest: audit.ExecutionBindingDigest,
		CapsuleDigest:          audit.CapsuleDigest, ContextItemID: audit.ItemID,
		ContextItemDigest: audit.ContentDigest, AgentID: audit.AgentID,
		RoleID: audit.RoleID, ArtifactRef: audit.ArtifactRef,
		Stage: "context_retrieval",
	}
	switch audit.Result {
	case "disclosed":
		record.Result = "succeeded"
	case "denied":
		record.Result = "failed"
		record.ErrorCode = "context_access_denied"
	default:
		return errors.New("invalid operational diagnostic")
	}
	return store.append(record)
}

func (recorder *productChatMigrationDiagnosticRecorder) RecordLocalProductChatMigration(
	ctx context.Context,
	diagnostic api.LocalProductChatMigrationDiagnostic,
) error {
	if recorder == nil || nilProductAssetPort(recorder.store) || ctx == nil ||
		!validProductDiagnosticIncidentID(recorder.incidentID) ||
		(diagnostic.Stage != "migration_read" &&
			diagnostic.Stage != "migration_commit" &&
			diagnostic.Stage != "migration_cleanup") ||
		(diagnostic.Result != "succeeded" && diagnostic.Result != "failed") ||
		diagnostic.Elapsed < 0 ||
		(diagnostic.Result == "succeeded" &&
			(diagnostic.ErrorCode != "" || diagnostic.Retryable)) ||
		(diagnostic.Result == "failed" &&
			!validProductDiagnosticErrorCode(diagnostic.ErrorCode)) {
		return errors.New("invalid operational diagnostic")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	record := productOperationalDiagnosticRecord{
		SchemaVersion:     1,
		OccurredAt:        recorder.store.operationalNow().UTC().Format(time.RFC3339Nano),
		IncidentID:        recorder.incidentID,
		Operation:         "conversation_migration",
		CredentialRuntime: recorder.store.credentialRuntimeValue(),
		Stage:             diagnostic.Stage,
		ElapsedMS:         max(diagnostic.Elapsed.Milliseconds(), 0),
		Result:            diagnostic.Result,
		ErrorCode:         diagnostic.ErrorCode,
		Retryable:         diagnostic.Retryable,
	}
	if err := recorder.store.append(record); err != nil {
		return err
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if diagnostic.Result == "failed" {
		recorder.lastFailure = &api.LocalProductChatAvailabilityFailure{
			Code: "state_unavailable", Stage: diagnostic.Stage,
			IncidentID: recorder.incidentID, Retryable: false,
		}
	}
	return nil
}

func (recorder *productChatMigrationDiagnosticRecorder) availabilityFailure() (
	api.LocalProductChatAvailabilityFailure,
	bool,
) {
	if recorder == nil {
		return api.LocalProductChatAvailabilityFailure{}, false
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if recorder.lastFailure == nil {
		return api.LocalProductChatAvailabilityFailure{}, false
	}
	return *recorder.lastFailure, true
}

func (store *productOperationalDiagnosticStore) RecordAgentAttemptDiagnostic(
	ctx context.Context,
	diagnostic nativeadapter.AgentAttemptDiagnostic,
) error {
	if store == nil || ctx == nil {
		return errors.New("operational diagnostics unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	elapsed := diagnostic.Elapsed.Milliseconds()
	if elapsed < 0 {
		elapsed = 0
	}
	return store.append(productOperationalDiagnosticRecord{
		SchemaVersion:          1,
		OccurredAt:             diagnostic.OccurredAt.UTC().Format(time.RFC3339Nano),
		IncidentID:             diagnostic.IncidentID,
		Operation:              "agent_attempt",
		CredentialRuntime:      store.credentialRuntimeValue(),
		ProviderID:             diagnostic.ProviderID,
		ProviderAccountID:      diagnostic.ProviderAccountID,
		ModelID:                diagnostic.ModelID,
		WorkItemID:             diagnostic.WorkItemID,
		RunID:                  diagnostic.RunID,
		ClaimGeneration:        diagnostic.ClaimGeneration,
		RuntimeInstanceID:      diagnostic.RuntimeInstanceID,
		AgentID:                diagnostic.AgentInstanceID,
		ExecutionBindingDigest: diagnostic.ExecutionBindingDigest,
		CapsuleDigest:          diagnostic.ContextCapsuleDigest,
		Stage:                  diagnostic.Stage,
		ElapsedMS:              elapsed,
		Result:                 diagnostic.Result,
		ErrorCode:              diagnostic.ErrorCode,
		Retryable:              diagnostic.Retryable,
	})
}

func (store *productOperationalDiagnosticStore) RecordAttemptToolDiagnostic(
	ctx context.Context,
	diagnostic productAttemptToolDiagnostic,
) error {
	if store == nil || ctx == nil || diagnostic.Execution.CorrelationID != diagnostic.IncidentID ||
		diagnostic.Execution.JobID != diagnostic.WorkItemID {
		return errors.New("invalid operational diagnostic")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	elapsed := diagnostic.Execution.Elapsed.Milliseconds()
	if elapsed < 0 {
		elapsed = 0
	}
	return store.append(productOperationalDiagnosticRecord{
		SchemaVersion:          1,
		OccurredAt:             store.now().UTC().Format(time.RFC3339Nano),
		IncidentID:             diagnostic.IncidentID,
		Operation:              "tool_call",
		CredentialRuntime:      store.credentialRuntimeValue(),
		ProviderID:             diagnostic.ProviderID,
		ProviderAccountID:      diagnostic.ProviderAccountID,
		ModelID:                diagnostic.ModelID,
		WorkItemID:             diagnostic.WorkItemID,
		RunID:                  diagnostic.RunID,
		ClaimGeneration:        diagnostic.ClaimGeneration,
		RuntimeInstanceID:      diagnostic.RuntimeInstanceID,
		AgentID:                diagnostic.AgentInstanceID,
		ExecutionBindingDigest: diagnostic.ExecutionBindingDigest,
		CapsuleDigest:          diagnostic.CapsuleDigest,
		ExecutionID:            diagnostic.Execution.ExecutionID,
		ToolCallID:             diagnostic.CallID,
		Tool:                   string(diagnostic.Execution.Tool),
		CallDigest:             diagnostic.Execution.CallDigest,
		ToolOperationID:        diagnostic.Execution.OperationID,
		Stage:                  string(diagnostic.Execution.Stage),
		ElapsedMS:              elapsed,
		Result:                 diagnostic.Execution.Result,
		ErrorCode:              diagnostic.Execution.ErrorCode,
		Retryable:              diagnostic.Execution.Retryable,
	})
}

func (store *productOperationalDiagnosticStore) AgentAttemptDiagnostics(
	ctx context.Context,
	queries []api.AgentAttemptDiagnosticQuery,
) ([]api.AgentAttemptDiagnosticSummary, error) {
	if store == nil || ctx == nil || len(queries) > 256 {
		return nil, errors.New("operational diagnostics unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	wanted := make(map[string]api.AgentAttemptDiagnosticQuery, len(queries))
	orderedKeys := make([]string, 0, len(queries))
	for _, query := range queries {
		key := productAgentAttemptDiagnosticKey(
			query.IncidentID,
			query.ProviderID,
			query.ProviderAccountID,
			query.ModelID,
		)
		if key == "" {
			return nil, errors.New("invalid operational diagnostic query")
		}
		if _, duplicate := wanted[key]; duplicate {
			continue
		}
		wanted[key] = query
		orderedKeys = append(orderedKeys, key)
	}
	if len(wanted) == 0 {
		return []api.AgentAttemptDiagnosticSummary{}, nil
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	matched := make(map[string]api.AgentAttemptDiagnosticSummary, len(wanted))
	for _, path := range []string{store.path + ".1", store.path} {
		records, err := readProductOperationalDiagnosticFile(path, store.maximum)
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if record.Operation != "agent_attempt" {
				continue
			}
			key := productAgentAttemptDiagnosticKey(
				record.IncidentID,
				record.ProviderID,
				record.ProviderAccountID,
				record.ModelID,
			)
			if _, requested := wanted[key]; !requested {
				continue
			}
			if record.Result == "succeeded" {
				delete(matched, key)
				continue
			}
			matched[key] = api.AgentAttemptDiagnosticSummary{
				IncidentID: record.IncidentID, ProviderID: record.ProviderID,
				ProviderAccountID: record.ProviderAccountID, ModelID: record.ModelID,
				FailureStage: record.Stage, FailureCode: record.ErrorCode,
				Retryable: record.Retryable,
			}
		}
	}
	result := make([]api.AgentAttemptDiagnosticSummary, 0, len(matched))
	for _, key := range orderedKeys {
		if summary, exists := matched[key]; exists {
			result = append(result, summary)
		}
	}
	return result, nil
}

func productAgentAttemptDiagnosticKey(
	incidentID, providerID, providerAccountID, modelID string,
) string {
	if !validProductDiagnosticIncidentID(incidentID) ||
		!validProductDiagnosticProviderID(providerID) ||
		!validProductDiagnosticIdentifier(providerAccountID, 128) ||
		!validProductDiagnosticIdentifier(modelID, 256) {
		return ""
	}
	return incidentID + "\x00" + providerID + "\x00" +
		providerAccountID + "\x00" + modelID
}

func readProductOperationalDiagnosticFile(
	path string,
	maximum int64,
) ([]productOperationalDiagnosticRecord, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return []productOperationalDiagnosticRecord{}, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 || info.Size() < 0 || info.Size() > maximum {
		return nil, errors.New("operational diagnostics unavailable")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("operational diagnostics unavailable")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) ||
		!opened.Mode().IsRegular() || opened.Mode().Perm() != 0o600 {
		return nil, errors.New("operational diagnostics unavailable")
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), int(maximum)+1)
	records := make([]productOperationalDiagnosticRecord, 0)
	for scanner.Scan() {
		var record productOperationalDiagnosticRecord
		if json.Unmarshal(scanner.Bytes(), &record) != nil ||
			!validProductOperationalDiagnosticRecord(record) {
			return nil, errors.New("operational diagnostics unavailable")
		}
		records = append(records, record)
	}
	if scanner.Err() != nil {
		return nil, errors.New("operational diagnostics unavailable")
	}
	return records, nil
}

func productOperationalDiagnosticFromResponse(
	occurredAt time.Time,
	request localipc.Request,
	response localipc.Response,
	elapsed time.Duration,
) productOperationalDiagnosticRecord {
	record := productOperationalDiagnosticRecord{
		SchemaVersion: 1,
		OccurredAt:    occurredAt.UTC().Format(time.RFC3339Nano),
		IncidentID:    request.RequestID,
		Operation:     request.Method,
		Stage:         "projection_refresh",
		ElapsedMS:     max(elapsed.Milliseconds(), 0),
		Result:        "succeeded",
	}
	record.ProviderID, record.ProviderAccountID =
		productDiagnosticProviderIdentity(request.Params)
	if request.Method == "chat_message" {
		record.ThreadID, record.ProfileID =
			productDiagnosticConversationIdentity(request.Params)
		record.Stage = "conversation_dispatch"
		if response.OK {
			if attempt, ok := productDiagnosticConversationAttempt(
				response.Result,
				request.RequestID,
			); ok && attempt.Status == "failed" {
				record.Result = "failed"
				record.ErrorCode = attempt.FailureCode
				record.HTTPStatus = attempt.HTTPStatus
				record.ProviderErrorCode = attempt.ProviderCode
				record.RetryAfterSeconds = attempt.RetryAfterSeconds
				record.Retryable = attempt.Retryable
				if productOperationalDiagnosticStage(attempt.FailureStage) {
					record.Stage = attempt.FailureStage
				}
			}
		}
	}
	if request.Method == "chat_response_cancel" {
		record.ThreadID, _ = productDiagnosticConversationIdentity(request.Params)
		record.Stage = "conversation_dispatch"
	}
	if request.Method == "agent_attempt_recovery" {
		record.Stage = "agent_attempt_reconcile"
	}
	if request.Method == "tool_recovery" {
		record.Stage = "tool_recovery"
	}
	if request.Method == "credential_vault_rotate" {
		record.Stage = "vault_rotation"
	}
	if request.Method == "credential_vault_lock" {
		record.Stage = credentials.CredentialStageLeaseRevoke
	}
	if request.Method == "credential_vault_unlock" {
		record.Stage = credentials.CredentialStageVaultOpen
	}
	if request.Method == "credential_vault_reset" {
		record.Stage = credentials.CredentialStageVaultRecovery
	}
	if request.Method == "credential_vault_export" {
		record.Stage = credentials.CredentialStageVaultExport
	}
	if !response.OK {
		record.Result = "failed"
		record.Stage = productDiagnosticStageDaemonAdmission
		if response.Error != nil {
			record.ErrorCode = response.Error.Code
			record.Retryable = response.Error.Recoverable
			if productOperationalDiagnosticStage(response.Error.Stage) {
				record.Stage = response.Error.Stage
			}
		}
	}
	return record
}

func productDiagnosticConversationAttempt(
	data []byte,
	incidentID string,
) (api.LocalProductConversationAttempt, bool) {
	var result struct {
		Attempts []api.LocalProductConversationAttempt `json:"attempts"`
	}
	if !validProductDiagnosticIncidentID(incidentID) ||
		json.Unmarshal(data, &result) != nil {
		return api.LocalProductConversationAttempt{}, false
	}
	for index := len(result.Attempts) - 1; index >= 0; index-- {
		attempt := result.Attempts[index]
		if attempt.IncidentID == incidentID &&
			(attempt.Status == "succeeded" ||
				attempt.Status == "failed" &&
					validProductDiagnosticErrorCode(attempt.FailureCode)) {
			return attempt, true
		}
	}
	return api.LocalProductConversationAttempt{}, false
}

func productDiagnosticConversationIdentity(data []byte) (string, string) {
	var params struct {
		ThreadID  string `json:"thread_id"`
		ProfileID string `json:"profile_id"`
	}
	if json.Unmarshal(data, &params) != nil ||
		!validProductDiagnosticIdentifier(params.ThreadID, 256) ||
		(params.ProfileID != "" &&
			!validProductDiagnosticIdentifier(params.ProfileID, 256)) {
		return "", ""
	}
	return params.ThreadID, params.ProfileID
}

func productDiagnosticProviderIdentity(data []byte) (string, string) {
	var params struct {
		ProviderID        string `json:"provider_id"`
		ProviderAccountID string `json:"provider_account_id"`
	}
	if json.Unmarshal(data, &params) != nil ||
		!validProductDiagnosticProviderID(params.ProviderID) {
		return "", ""
	}
	if params.ProviderAccountID != "" &&
		!credentials.ValidProviderAccountIdentifier(
			params.ProviderID,
			params.ProviderAccountID,
		) {
		return "", ""
	}
	return params.ProviderID, params.ProviderAccountID
}

func validProductDiagnosticProviderID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func productOperationalDiagnosticStage(stage string) bool {
	switch stage {
	case "input_admission", "uds_transport", "daemon_admission",
		"helper_validation", "helper_start", "helper_authorization",
		"helper_request", "helper_timeout", "helper_response", "helper_exit",
		"keychain_access", "metadata_commit", "projection_refresh",
		"vault_key_load", "vault_open", "vault_encrypt", "vault_commit",
		"vault_decrypt", "vault_aad_validation", "vault_rotation", "vault_recovery",
		"vault_export",
		"credential_lease_issue", "credential_lease_expire",
		"credential_lease_revoke", "migration_read", "migration_commit",
		"migration_cleanup",
		"provider_dns", "provider_tls", "provider_connect", "provider_http",
		"provider_auth", "provider_rate_limit", "profile_publish",
		"conversation_dispatch", "agent_attempt_dispatch", "agent_input_admission",
		"agent_attempt_reconcile",
		"context_retrieval",
		"context_delivery_reconcile",
		"tool_input_admission", "tool_binding_validation", "tool_authorization",
		"tool_approval_wait", "tool_sandbox_prepare", "tool_dispatch",
		"tool_result_validation", "tool_result_commit", "tool_payload_commit",
		"tool_result_delivery", "tool_recovery",
		"composition_compile", "composition_validate", "bundle_register",
		"bundle_start", "bundle_ready", "bundle_stop", "bundle_dispose",
		"route_compile", "scope_open", "scope_close":
		return true
	default:
		return false
	}
}

func (store *productOperationalDiagnosticStore) append(
	record productOperationalDiagnosticRecord,
) error {
	record.CredentialHelperSpawnAttempts =
		credentials.ProductKeychainHelperSpawnAttempts()
	if store == nil || !validProductOperationalDiagnosticRecord(record) {
		return errors.New("invalid operational diagnostic")
	}
	line, err := json.Marshal(record)
	if err != nil {
		return errors.New("operational diagnostics unavailable")
	}
	line = append(line, '\n')
	if int64(len(line)) > store.maximum {
		return errors.New("operational diagnostic too large")
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.rotateIfNeeded(int64(len(line))); err != nil {
		return err
	}
	file, err := openProductOperationalFile(store.path)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(line)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return errors.New("operational diagnostics unavailable")
	}
	return nil
}

func validProductOperationalDiagnosticRecord(
	record productOperationalDiagnosticRecord,
) bool {
	base := record.SchemaVersion == 1 &&
		validProductDiagnosticIncidentID(record.IncidentID) &&
		productOperationalDiagnosticMethod(record.Operation) &&
		(record.CredentialRuntime == "" ||
			validProductCredentialRuntime(record.CredentialRuntime)) &&
		(record.ProviderID == "" || validProductDiagnosticProviderID(record.ProviderID)) &&
		(record.ProviderAccountID == "" || validProductDiagnosticIdentifier(record.ProviderAccountID, 128)) &&
		record.CredentialRevision >= 0 &&
		(record.ModelID == "" || validProductDiagnosticIdentifier(record.ModelID, 256)) &&
		(record.ReasoningEffort == "" || validProductDiagnosticIdentifier(record.ReasoningEffort, 128)) &&
		(record.ThreadID == "" || validProductDiagnosticIdentifier(record.ThreadID, 256)) &&
		(record.ProfileID == "" || validProductDiagnosticIdentifier(record.ProfileID, 256)) &&
		(record.WorkspaceDigest == "" || validProductHex(record.WorkspaceDigest, 64)) &&
		(record.SegmentContextCapsuleDigest == "" ||
			validProductHex(record.SegmentContextCapsuleDigest, 64)) &&
		(record.GovernancePolicyDigest == "" || validProductHex(record.GovernancePolicyDigest, 64)) &&
		(record.RouteTransitionReviewDigest == "" ||
			validProductHex(record.RouteTransitionReviewDigest, 64)) &&
		productOperationalDiagnosticStage(record.Stage) &&
		record.ElapsedMS >= 0 &&
		(record.HTTPStatus == 0 || record.HTTPStatus >= 100 && record.HTTPStatus <= 599) &&
		(record.ProviderErrorCode == "" ||
			validProductDiagnosticProviderErrorCode(record.ProviderErrorCode)) &&
		record.RetryAfterSeconds >= 0 && record.RetryAfterSeconds <= 24*60*60 &&
		(record.Result == "succeeded" && record.ErrorCode == "" &&
			record.HTTPStatus == 0 && record.ProviderErrorCode == "" &&
			record.RetryAfterSeconds == 0 && !record.Retryable ||
			record.Result == "failed" && validProductDiagnosticErrorCode(record.ErrorCode))
	if !base {
		return false
	}
	return validProductContextRetrievalDiagnostic(record)
}

func validProductContextRetrievalDiagnostic(record productOperationalDiagnosticRecord) bool {
	executionIdentityFieldsAbsent := record.WorkItemID == "" && record.RunID == "" &&
		record.ClaimGeneration == 0 && record.RuntimeInstanceID == "" &&
		record.ContextItemID == "" && record.ContextItemDigest == "" &&
		record.AgentID == "" && record.RoleID == "" && record.ArtifactRef == "" &&
		record.ExecutionID == "" && record.ToolCallID == "" && record.Tool == "" &&
		record.CallDigest == "" && record.ToolOperationID == ""
	executionFieldsAbsent := executionIdentityFieldsAbsent &&
		record.ExecutionBindingDigest == "" && record.CapsuleDigest == ""
	compositionFieldsAbsent := record.CompositionSnapshotDigest == "" &&
		record.BundleID == "" && record.BundleVersion == "" &&
		record.ScopeKind == "" && record.ScopeID == "" && record.ScopeDigest == ""
	gatewayFieldsAbsent := record.GatewayEventSchemaVersion == 0 &&
		record.GatewayInstanceID == "" &&
		record.GatewayConfiguredHarnessVersion == 0 && record.GatewayBackendVersion == 0 &&
		record.GatewayEventSequence == 0 && record.GatewayEventType == "" &&
		record.SessionID == "" && record.HarnessID == "" && record.BackendID == "" &&
		record.SegmentID == "" && record.WorkspaceID == "" && record.ResponseID == "" &&
		record.WorkspaceDigest == "" && record.CredentialRevision == 0 &&
		record.ReasoningEffort == "" && record.SegmentContextCapsuleDigest == "" &&
		record.GovernancePolicyDigest == "" &&
		record.RouteTransitionReviewDigest == ""
	fieldsAbsent := executionFieldsAbsent && compositionFieldsAbsent && gatewayFieldsAbsent
	if record.Operation == "chat_message" && !gatewayFieldsAbsent {
		return executionIdentityFieldsAbsent && compositionFieldsAbsent &&
			validProductHarnessGatewayDiagnostic(record)
	}
	if record.Operation == "composition" {
		return executionFieldsAbsent && gatewayFieldsAbsent &&
			validProductCompositionDiagnostic(record)
	}
	if record.Operation == "authorization_reconcile" {
		return compositionFieldsAbsent && gatewayFieldsAbsent &&
			record.ThreadID == "" && record.ProfileID == "" &&
			validProductDiagnosticProviderID(record.ProviderID) &&
			validProductDiagnosticIdentifier(record.ProviderAccountID, 128) &&
			validProductDiagnosticIdentifier(record.ModelID, 256) &&
			validProductDiagnosticIdentifier(record.WorkItemID, 256) &&
			validProductDiagnosticIdentifier(record.RunID, 256) &&
			record.ClaimGeneration > 0 &&
			validProductDiagnosticIdentifier(record.RuntimeInstanceID, 256) &&
			validProductHex(record.ExecutionBindingDigest, 64) &&
			record.CapsuleDigest == "" &&
			validProductDiagnosticIdentifier(record.AgentID, 512) &&
			record.ContextItemID == "" && record.ContextItemDigest == "" &&
			record.RoleID == "" && record.ArtifactRef == "" &&
			record.ExecutionID == "" && record.ToolCallID == "" &&
			record.Tool == "" && record.CallDigest == "" && record.ToolOperationID == "" &&
			record.Stage == "agent_attempt_reconcile" && record.Result == "succeeded"
	}
	if record.Operation == "agent_attempt" {
		if fieldsAbsent {
			return true
		}
		return compositionFieldsAbsent && gatewayFieldsAbsent &&
			validProductDiagnosticIdentifier(record.WorkItemID, 256) &&
			validProductDiagnosticIdentifier(record.RunID, 256) &&
			record.ClaimGeneration > 0 &&
			validProductDiagnosticIdentifier(record.RuntimeInstanceID, 256) &&
			validProductHex(record.ExecutionBindingDigest, 64) &&
			validProductHex(record.CapsuleDigest, 64) &&
			validProductDiagnosticIdentifier(record.AgentID, 512) &&
			record.ContextItemID == "" && record.ContextItemDigest == "" &&
			record.RoleID == "" && record.ArtifactRef == "" &&
			record.ExecutionID == "" && record.ToolCallID == "" &&
			record.Tool == "" && record.CallDigest == "" && record.ToolOperationID == ""
	}
	if record.Operation == "tool_call" {
		return compositionFieldsAbsent && gatewayFieldsAbsent &&
			validProductDiagnosticProviderID(record.ProviderID) &&
			validProductDiagnosticIdentifier(record.ProviderAccountID, 128) &&
			validProductDiagnosticIdentifier(record.ModelID, 256) &&
			validProductDiagnosticIdentifier(record.WorkItemID, 256) &&
			validProductDiagnosticIdentifier(record.RunID, 256) &&
			record.ClaimGeneration > 0 &&
			validProductDiagnosticIdentifier(record.RuntimeInstanceID, 256) &&
			validProductHex(record.ExecutionBindingDigest, 64) &&
			validProductHex(record.CapsuleDigest, 64) &&
			validProductDiagnosticIdentifier(record.AgentID, 512) &&
			validProductDiagnosticIdentifier(record.ExecutionID, 256) &&
			validProductDiagnosticIdentifier(record.ToolCallID, 256) &&
			permissions.ValidToolKind(record.Tool) &&
			validProductHex(record.CallDigest, 64) &&
			validProductDiagnosticIdentifier(record.ToolOperationID, 256) &&
			record.ContextItemID == "" && record.ContextItemDigest == "" &&
			record.RoleID == "" && record.ArtifactRef == ""
	}
	if record.Operation != "context_retrieval" {
		return fieldsAbsent
	}
	return compositionFieldsAbsent && gatewayFieldsAbsent &&
		record.Stage == "context_retrieval" &&
		validProductDiagnosticIdentifier(record.WorkItemID, 256) &&
		validProductDiagnosticIdentifier(record.RunID, 256) &&
		record.ClaimGeneration > 0 &&
		validProductDiagnosticIdentifier(record.RuntimeInstanceID, 256) &&
		validProductHex(record.ExecutionBindingDigest, 64) &&
		validProductHex(record.CapsuleDigest, 64) &&
		validProductDiagnosticIdentifier(record.ContextItemID, 512) &&
		validProductHex(record.ContextItemDigest, 64) &&
		validProductDiagnosticIdentifier(record.AgentID, 512) &&
		validProductDiagnosticIdentifier(record.RoleID, 512) &&
		(record.ArtifactRef == "" || validProductDiagnosticIdentifier(record.ArtifactRef, 512)) &&
		(record.Result == "succeeded" && record.ErrorCode == "" ||
			record.Result == "failed" && record.ErrorCode == "context_access_denied")
}

func validProductHarnessGatewayDiagnostic(record productOperationalDiagnosticRecord) bool {
	if record.GatewayEventSchemaVersion == 1 {
		return validLegacyProductHarnessGatewayDiagnosticV1(record)
	}
	if record.GatewayEventSchemaVersion == 2 {
		if record.GatewayInstanceID != "" {
			return false
		}
		record.GatewayEventSchemaVersion = harnessgateway.EventSchemaVersion
		record.GatewayInstanceID = "gateway-legacy-v2"
		return validProductHarnessGatewayDiagnostic(record)
	}
	if record.GatewayEventSchemaVersion != harnessgateway.EventSchemaVersion {
		return false
	}
	if record.Operation != "chat_message" || record.Stage != "conversation_dispatch" ||
		record.ProfileID != "" ||
		!validProductDiagnosticIdentifier(record.GatewayInstanceID, 255) ||
		!validProductDiagnosticProviderID(record.ProviderID) ||
		!validProductHarnessGatewayProviderAuthority(record) ||
		!validProductDiagnosticIdentifier(record.ModelID, 256) ||
		(record.ReasoningEffort != "" &&
			!validProductDiagnosticIdentifier(record.ReasoningEffort, 128)) ||
		!validProductHex(record.WorkspaceDigest, 64) ||
		!validProductHex(record.ExecutionBindingDigest, 64) ||
		!validProductHex(record.SegmentContextCapsuleDigest, 64) {
		return false
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, record.OccurredAt)
	if err != nil {
		return false
	}
	eventType := harnessgateway.EventType(record.GatewayEventType)
	incidentID := record.IncidentID
	result, errorCode, retryable, ok := productHarnessGatewayDiagnosticOutcome(eventType)
	if !ok || record.Result != result || record.ErrorCode != errorCode ||
		record.Retryable != retryable || record.HTTPStatus != 0 ||
		record.ProviderErrorCode != "" || record.RetryAfterSeconds != 0 {
		return false
	}
	switch eventType {
	case harnessgateway.EventSessionOpening, harnessgateway.EventSessionReady,
		harnessgateway.EventSessionClosing, harnessgateway.EventSessionClosed,
		harnessgateway.EventSessionFailed:
		if record.IncidentID != "loom-session-"+productHarnessGatewayDigest(record.SessionID)[:48] {
			return false
		}
		incidentID = ""
	}
	return (harnessgateway.Event{
		SchemaVersion:               record.GatewayEventSchemaVersion,
		GatewayInstanceID:           record.GatewayInstanceID,
		ConfiguredHarnessVersion:    record.GatewayConfiguredHarnessVersion,
		BackendVersion:              record.GatewayBackendVersion,
		Sequence:                    record.GatewayEventSequence,
		OccurredAt:                  occurredAt,
		Type:                        eventType,
		SessionID:                   record.SessionID,
		HarnessID:                   harnessgateway.HarnessID(record.HarnessID),
		BackendID:                   harnessgateway.BackendID(record.BackendID),
		ConversationID:              record.ThreadID,
		SegmentID:                   record.SegmentID,
		WorkspaceID:                 record.WorkspaceID,
		WorkspaceDigest:             record.WorkspaceDigest,
		ExecutionBindingDigest:      record.ExecutionBindingDigest,
		ProviderID:                  record.ProviderID,
		ProviderAccountID:           record.ProviderAccountID,
		CredentialRevision:          record.CredentialRevision,
		ModelID:                     record.ModelID,
		ReasoningEffort:             record.ReasoningEffort,
		SegmentContextCapsuleDigest: record.SegmentContextCapsuleDigest,
		ContextCapsuleDigest:        record.CapsuleDigest,
		GovernancePolicyDigest:      record.GovernancePolicyDigest,
		RouteTransitionReviewDigest: record.RouteTransitionReviewDigest,
		ResponseID:                  record.ResponseID,
		IncidentID:                  incidentID,
	}).Valid()
}

func validProductHarnessGatewayProviderAuthority(record productOperationalDiagnosticRecord) bool {
	if record.ProviderAccountID == "" {
		return record.CredentialRevision == 0 && record.GovernancePolicyDigest == ""
	}
	return credentials.ValidProviderAccountIdentifier(record.ProviderID, record.ProviderAccountID) &&
		record.CredentialRevision > 0 &&
		(record.GovernancePolicyDigest == "" ||
			validProductHex(record.GovernancePolicyDigest, 64))
}

func validLegacyProductHarnessGatewayDiagnosticV1(
	record productOperationalDiagnosticRecord,
) bool {
	if record.Operation != "chat_message" || record.Stage != "conversation_dispatch" ||
		record.ProviderID != "" || record.ProviderAccountID != "" ||
		record.CredentialRevision != 0 || record.ModelID != "" ||
		record.ReasoningEffort != "" || record.ProfileID != "" ||
		record.GatewayInstanceID != "" ||
		record.WorkspaceDigest != "" || record.ExecutionBindingDigest != "" ||
		record.SegmentContextCapsuleDigest != "" || record.CapsuleDigest != "" ||
		record.GovernancePolicyDigest != "" ||
		record.RouteTransitionReviewDigest != "" ||
		record.GatewayConfiguredHarnessVersion < 1 || record.GatewayBackendVersion < 1 ||
		record.GatewayEventSequence == 0 ||
		!validProductDiagnosticIdentifier(record.SessionID, 255) ||
		!validProductDiagnosticIdentifier(record.HarnessID, 255) ||
		!validProductDiagnosticIdentifier(record.BackendID, 255) ||
		!validProductDiagnosticIdentifier(record.ThreadID, 255) ||
		!validProductDiagnosticIdentifier(record.SegmentID, 255) ||
		!validProductDiagnosticIdentifier(record.WorkspaceID, 255) {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, record.OccurredAt); err != nil {
		return false
	}
	eventType := harnessgateway.EventType(record.GatewayEventType)
	result, errorCode, retryable, ok := productHarnessGatewayDiagnosticOutcome(eventType)
	if !ok || record.Result != result || record.ErrorCode != errorCode ||
		record.Retryable != retryable || record.HTTPStatus != 0 ||
		record.ProviderErrorCode != "" || record.RetryAfterSeconds != 0 {
		return false
	}
	switch eventType {
	case harnessgateway.EventSessionOpening, harnessgateway.EventSessionReady,
		harnessgateway.EventSessionClosing, harnessgateway.EventSessionClosed,
		harnessgateway.EventSessionFailed:
		return record.ResponseID == "" &&
			record.IncidentID == "loom-session-"+
				productHarnessGatewayDigest(record.SessionID)[:48]
	case harnessgateway.EventResponseStarted, harnessgateway.EventResponseCompleted,
		harnessgateway.EventResponseCancelled, harnessgateway.EventResponseFailed:
		return validProductDiagnosticIdentifier(record.ResponseID, 255) &&
			validProductDiagnosticIncidentID(record.IncidentID)
	default:
		return false
	}
}

func productHarnessGatewayDiagnosticOutcome(
	eventType harnessgateway.EventType,
) (string, string, bool, bool) {
	switch eventType {
	case harnessgateway.EventResponseFailed:
		return "failed", "provider_unavailable", true, true
	case harnessgateway.EventResponseCancelled:
		return "failed", "cancelled", false, true
	case harnessgateway.EventSessionFailed:
		return "failed", "conversation_unavailable", true, true
	case harnessgateway.EventSessionOpening, harnessgateway.EventSessionReady,
		harnessgateway.EventSessionClosing, harnessgateway.EventSessionClosed,
		harnessgateway.EventResponseStarted, harnessgateway.EventResponseCompleted:
		return "succeeded", "", false, true
	default:
		return "", "", false, false
	}
}

func validProductCompositionDiagnostic(record productOperationalDiagnosticRecord) bool {
	if record.ProviderID != "" || record.ProviderAccountID != "" || record.ModelID != "" ||
		record.ThreadID != "" || !validProductHex(record.CompositionSnapshotDigest, 64) ||
		!validProductDiagnosticIdentifier(record.ProfileID, 32) ||
		(record.BundleID == "") != (record.BundleVersion == "") ||
		(record.ScopeKind == "") != (record.ScopeID == "") ||
		(record.ScopeKind == "") != (record.ScopeDigest == "") {
		return false
	}
	if record.BundleID != "" &&
		(!validProductDiagnosticIdentifier(record.BundleID, 128) ||
			!validProductDiagnosticIdentifier(record.BundleVersion, 32)) {
		return false
	}
	if record.ScopeKind != "" {
		switch composition.ScopeKind(record.ScopeKind) {
		case composition.ScopeRoot, composition.ScopeProduct,
			composition.ScopeConversation, composition.ScopeTeam,
			composition.ScopeAgent, composition.ScopeAttempt, composition.ScopeTurn:
		default:
			return false
		}
		if !validProductDiagnosticIdentifier(record.ScopeID, 256) ||
			!validProductHex(record.ScopeDigest, 64) {
			return false
		}
	}
	switch record.Stage {
	case "bundle_register", "bundle_start", "bundle_ready", "bundle_stop", "bundle_dispose":
		return record.BundleID != "" && record.ScopeKind == ""
	case "scope_open", "scope_close":
		return record.BundleID == "" && record.ScopeKind != ""
	case "composition_compile", "composition_validate", "route_compile":
		return record.ScopeKind == ""
	default:
		return false
	}
}

func validProductCredentialRuntime(value string) bool {
	return value == productCredentialRuntimeVault ||
		value == productCredentialRuntimeExplicitLegacy
}

func validProductDiagnosticIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' ||
			character == '/' || character == ':' {
			continue
		}
		return false
	}
	return true
}

func validProductDiagnosticIncidentID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == ':' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validProductDiagnosticErrorCode(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validProductDiagnosticProviderErrorCode(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '_' || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func (store *productOperationalDiagnosticStore) rotateIfNeeded(incoming int64) error {
	info, err := os.Lstat(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("operational diagnostics unavailable")
	}
	if info.Mode().Perm() != 0o600 {
		if err := os.Chmod(store.path, 0o600); err != nil {
			return errors.New("operational diagnostics unavailable")
		}
	}
	if info.Size()+incoming <= store.maximum {
		return nil
	}
	rotated := store.path + ".1"
	if rotatedInfo, rotatedErr := os.Lstat(rotated); rotatedErr == nil {
		if !rotatedInfo.Mode().IsRegular() || rotatedInfo.Mode()&os.ModeSymlink != 0 ||
			os.Remove(rotated) != nil {
			return errors.New("operational diagnostics unavailable")
		}
	} else if !errors.Is(rotatedErr, os.ErrNotExist) {
		return errors.New("operational diagnostics unavailable")
	}
	if err := os.Rename(store.path, rotated); err != nil || os.Chmod(rotated, 0o600) != nil {
		return errors.New("operational diagnostics unavailable")
	}
	return nil
}

func openProductOperationalFile(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, errors.New("operational diagnostics unavailable")
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		_ = file.Close()
		return nil, errors.New("operational diagnostics unavailable")
	}
	return file, nil
}
