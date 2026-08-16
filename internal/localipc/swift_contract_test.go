//go:build darwin

package localipc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/rules"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"

	_ "modernc.org/sqlite"
)

const swiftSnapshotFixture = `{
  "schema_version":1,
  "view_version":"view-1",
  "partial":false,
  "stale":false,
  "reason":"",
  "runtimes":[{
    "runtime_instance_id":"pi-local",
    "display_name":"Pi",
    "adapter_type":"pi",
    "executable_version":"0.82.1",
    "status":"online",
    "capacity":2,
    "model_ids":[],
    "observed_capabilities":[]
  }],
  "teams":[],
  "runs":[],
  "evidence":[],
  "attention":[],
  "runtime_page":{"next_cursor":"pi-local","has_more":false},
  "team_page":{"next_cursor":"","has_more":false},
  "run_page":{"next_cursor":"","has_more":false},
  "evidence_page":{"next_cursor":"","has_more":false}
}`

func TestP3ASwiftProductionClientHasStrictJourneyAndAssetSurface(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "apps", "macos", "Sources"))
	files := map[string][]string{
		filepath.Join(root, "LoomLocalAppCore", "LocalProductAssetModels.swift"): {
			"EvolutionAssetSnapshot", "EvolutionAssetCommand", "journeyID",
		},
		filepath.Join(root, "LoomLocalAppCore", "LocalIPCClient.swift"): {
			"journey_id", "evolution_asset_snapshot", "evolution_asset_diff",
			"evolution_asset_command",
		},
		filepath.Join(root, "LoomLocalAppUI", "MissionWorkbench.swift"): {
			"Assets", "Activate", "Rollback", "Materialization",
		},
	}
	for file, tokens := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read required Swift product source %s: %v", file, err)
		}
		for _, token := range tokens {
			if !bytes.Contains(content, []byte(token)) {
				t.Fatalf("Swift source %s missing %q", file, token)
			}
		}
	}
}

type swiftP3AAssetHandler struct{ service *api.LocalProductAssetAPI }

func (handler swiftP3AAssetHandler) Handle(ctx context.Context, request Request) Response {
	if request.Method != "evolution_asset_snapshot" || request.JourneyID == "" {
		return Response{JourneyID: request.JourneyID, Error: safeProtocolError("invalid_request", errors.New("method"))}
	}
	var input api.EvolutionAssetSnapshotRequest
	decoder := json.NewDecoder(bytes.NewReader(request.Params))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		return Response{JourneyID: request.JourneyID, Error: safeProtocolError("invalid_request", errors.New("params"))}
	}
	input.JourneyID = request.JourneyID
	result, err := handler.service.EvolutionAssetSnapshot(ctx, input)
	if err != nil {
		return Response{JourneyID: request.JourneyID, Error: safeProtocolError("internal", err)}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return Response{JourneyID: request.JourneyID, Error: safeProtocolError("internal", err)}
	}
	return Response{JourneyID: request.JourneyID, OK: true, Result: encoded}
}

func TestP3ARealGoAssetServiceAndIPCDecodeInStrictSwiftClient(t *testing.T) {
	probe := buildSwiftContractProbe(t)
	root, socketPath := swiftPrivateSocketRoot(t)
	defer os.RemoveAll(root)
	database, err := sql.Open("sqlite", filepath.Join(root, "p3a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	authority, err := assets.NewAuthority(assets.AuthorityConfig{Store: journal.NewStore(database), Now: func() time.Time { return time.Unix(2_000, 0).UTC() }, ViewVersion: func() string { return readModel.GlobalReadView().Version() }})
	if err != nil {
		t.Fatal(err)
	}
	artifactStore, err := evidence.NewStore(filepath.Join(root, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	defer artifactStore.Close()
	service, err := app.NewLocalProductAssetService(readModel, authority, artifactStore)
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	if _, err := authority.CreateSkill(context.Background(), assets.Command{
		OperationID:         "p3a-swift-nil-collection-fixture",
		JourneyID:           "123e4567-e89b-42d3-a456-426614174000",
		ExpectedViewVersion: readModel.GlobalReadView().Version(),
		AssetKind:           assets.AssetKindSkill,
		DefinitionID:        "skill-swift-wire", RevisionID: "revision-1",
		CandidateID: "candidate-swift-wire", Name: "Swift Wire Skill",
		Description: "strict production wire fixture", Scope: "project",
		ArtifactDigest: digest, ContentDigest: digest,
		SourceScope: assets.SourceScopeLocal, SourceReferenceDigest: digest,
		ProvenanceDigest: digest, Dependencies: []string{},
		CompatibleCapabilities: []string{}, SourceEvidenceIDs: []string{},
		SourceEvidenceDigests: []string{}, RequiredEvaluationIDs: []string{},
		Risk: assets.RiskLow,
	}); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	assetAPI, err := api.NewLocalProductAssetAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(ServerConfig{SocketPath: socketPath, EffectiveUID: os.Geteuid(), BuildID: "p3a-swift-fixture", Handler: swiftP3AAssetHandler{service: assetAPI}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("server not ready")
	}
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	output, runErr := exec.Command(probe, "--socket", socketPath, "--assets", journeyID).CombinedOutput()
	cancel()
	_ = server.Close()
	<-done
	if runErr != nil {
		t.Fatalf("Swift P3A probe: %v output=%q", runErr, output)
	}
	var actual struct {
		ViewVersion string `json:"viewVersion"`
		Definitions int    `json:"definitions"`
		Revisions   int    `json:"revisions"`
	}
	if err := json.Unmarshal(output, &actual); err != nil || actual.ViewVersion == "" || actual.Definitions != 1 || actual.Revisions != 1 {
		t.Fatalf("Swift P3A output=%q actual=%#v err=%v", output, actual, err)
	}
}

type swiftBP1PermissionHandler struct {
	api *api.LocalPermissionAPI
}

func (handler swiftBP1PermissionHandler) Handle(ctx context.Context, request Request) Response {
	if request.JourneyID == "" || !validJourneyID(request.JourneyID) {
		return Response{JourneyID: request.JourneyID, Error: safeProtocolError("invalid_request", errors.New("journey"))}
	}
	switch request.Method {
	case "permissions_snapshot":
		var input app.PermissionSnapshotRequest
		decoder := json.NewDecoder(bytes.NewReader(request.Params))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil {
			return Response{JourneyID: request.JourneyID, Error: safeProtocolError("invalid_request", errors.New("params"))}
		}
		input.JourneyID = request.JourneyID
		result, err := handler.api.PermissionSnapshot(ctx, input)
		if err != nil {
			return Response{JourneyID: request.JourneyID, Error: safeProtocolError("internal", err)}
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return Response{JourneyID: request.JourneyID, Error: safeProtocolError("internal", err)}
		}
		return Response{JourneyID: request.JourneyID, OK: true, Result: encoded}
	case "permissions_attention":
		var input app.PermissionAttentionRequest
		decoder := json.NewDecoder(bytes.NewReader(request.Params))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil {
			return Response{JourneyID: request.JourneyID, Error: safeProtocolError("invalid_request", errors.New("params"))}
		}
		input.JourneyID = request.JourneyID
		result, err := handler.api.PermissionAttention(ctx, input)
		if err != nil {
			return Response{JourneyID: request.JourneyID, Error: safeProtocolError("internal", err)}
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return Response{JourneyID: request.JourneyID, Error: safeProtocolError("internal", err)}
		}
		return Response{JourneyID: request.JourneyID, OK: true, Result: encoded}
	case "permissions_command":
		var input app.PermissionCommandRequest
		decoder := json.NewDecoder(bytes.NewReader(request.Params))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil {
			return Response{JourneyID: request.JourneyID, Error: safeProtocolError("invalid_request", errors.New("params"))}
		}
		input.JourneyID = request.JourneyID
		result, err := handler.api.PermissionCommand(ctx, input)
		if err != nil {
			return Response{JourneyID: request.JourneyID, Error: safeProtocolError("internal", err)}
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return Response{JourneyID: request.JourneyID, Error: safeProtocolError("internal", err)}
		}
		return Response{JourneyID: request.JourneyID, OK: true, Result: encoded}
	default:
		return Response{JourneyID: request.JourneyID, Error: safeProtocolError("invalid_request", errors.New("method"))}
	}
}

type swiftTestPermissionAuthorizer struct {
	now func() time.Time
}

func (authorizer *swiftTestPermissionAuthorizer) AuthorizeRuleSet(
	ctx context.Context,
	request rules.RuleSetActivationRequest,
) (rules.AuthorizedRuleSetActivation, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedRuleSetActivation{}, err
	}
	digest := sha256.Sum256([]byte("swift-permission-rule-auth"))
	now := authorizer.now()
	return rules.NewAuthorizedRuleSetActivation(
		request,
		"approver:permission-owner",
		fmt.Sprintf("%x", digest[:]),
		now,
		now.Add(time.Hour),
	)
}

func (authorizer *swiftTestPermissionAuthorizer) AuthorizeApprovalDecision(
	ctx context.Context,
	request rules.ApprovalDecisionRequest,
) (rules.AuthorizedApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedApprovalDecision{}, err
	}
	digest := sha256.Sum256([]byte("swift-permission-decision-auth"))
	now := authorizer.now()
	return rules.NewAuthorizedApprovalDecision(
		request,
		"approver:permission-owner",
		fmt.Sprintf("%x", digest[:]),
		now,
		now.Add(time.Hour),
	)
}

type swiftTestApprovalPort struct {
	authority *rules.Authority
}

func (port swiftTestApprovalPort) RequestPermissionApproval(
	ctx context.Context,
	input rules.PermissionApprovalInput,
) (rules.ApprovalRequestRecord, error) {
	return port.authority.RequestPermissionApproval(ctx, input)
}

func (port swiftTestApprovalPort) DecidePermissionApproval(
	ctx context.Context,
	approvalID, approvalDigest, decision, resolvedBy, correlationID string,
) (rules.ApprovalRequestRecord, error) {
	return port.authority.DecidePermissionApproval(
		ctx, approvalID, approvalDigest, decision, resolvedBy, correlationID,
	)
}

func TestBp1SwiftProbeDecodesRealPermissionAttentionWithApprovals(t *testing.T) {
	probe := buildSwiftContractProbe(t)
	root, socketPath := swiftPrivateSocketRoot(t)
	defer os.RemoveAll(root)
	database, err := sql.Open("sqlite", filepath.Join(root, "bp1.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	now := func() time.Time { return time.Now().UTC() }
	rulesAuthority, err := rules.NewAuthority(store, &swiftTestPermissionAuthorizer{now: now}, now)
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewLocalPermissionService(store, now, func() string { return "v1" }, swiftTestApprovalPort{authority: rulesAuthority})
	if err != nil {
		t.Fatal(err)
	}
	permissionAPI, err := api.NewLocalPermissionAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(), BuildID: "bp1-swift-fixture",
		Handler: swiftBP1PermissionHandler{api: permissionAPI},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case <-time.After(10 * time.Second):
		t.Fatal("server not ready")
	}
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	template, err := permissions.DefaultProjectProfileTemplate()
	if err != nil {
		t.Fatal(err)
	}
	define, _ := json.Marshal(permissions.ProfileInput{
		ProfileID: "swift-profile", Mode: template.Mode,
		Rules: template.Rules, OwnedPaths: template.OwnedPaths,
	})
	if _, err := service.PermissionCommand(ctx, app.PermissionCommandRequest{
		JourneyID: journeyID, OperationID: "swift-op-define", Action: "define_profile", Input: define,
	}); err != nil {
		t.Fatal(err)
	}
	bind, _ := json.Marshal(map[string]any{"job_id": "swift-job", "profile_id": "swift-profile"})
	if _, err := service.PermissionCommand(ctx, app.PermissionCommandRequest{
		JourneyID: journeyID, OperationID: "swift-op-bind", Action: "bind_job", Input: bind,
	}); err != nil {
		t.Fatal(err)
	}
	call, _ := json.Marshal(map[string]any{
		"job_id": "swift-job",
		"call":   map[string]any{"tool": "Bash", "command": "curl https://example.com", "path": ""},
	})
	result, err := service.PermissionCommand(ctx, app.PermissionCommandRequest{
		JourneyID: journeyID, OperationID: "swift-op-validate", Action: "validate_call", Input: call,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictAsk || result.ApprovalID == "" {
		t.Fatalf("validate_call result = %+v", result)
	}
	output, runErr := exec.Command(probe, "--socket", socketPath, "--permissions-attention", journeyID).CombinedOutput()
	cancel()
	_ = server.Close()
	<-done
	if runErr != nil {
		t.Fatalf("Swift B-P1 attention probe: %v output=%q", runErr, output)
	}
	var actual struct {
		ViewVersion string `json:"viewVersion"`
		Decisions   int    `json:"decisions"`
		Approvals   int    `json:"approvals"`
	}
	if err := json.Unmarshal(output, &actual); err != nil {
		t.Fatalf("probe output=%q err=%v", output, err)
	}
	if actual.Decisions < 1 || actual.Approvals < 1 {
		t.Fatalf("Swift attention must see the same pending decision+approval, got %+v", actual)
	}
}

const swiftPartialSnapshotFixture = `{
  "schema_version":2,
  "view_version":"view-partial-1",
  "partial":true,
  "stale":false,
  "reason":"observer_models_timeout",
  "health":{
    "daemon":"serving_request",
    "journal":"available",
    "projection":"current"
  },
  "runtimes":[],
  "teams":[],
  "missions":[],
  "runs":[],
  "evidence":[],
  "attention":[],
  "prepared_decisions":[],
  "runtime_page":{"next_cursor":"","has_more":false},
  "team_page":{"next_cursor":"","has_more":false},
  "mission_page":{"next_cursor":"","has_more":false},
  "run_page":{"next_cursor":"","has_more":false},
  "evidence_page":{"next_cursor":"","has_more":false}
}`

const swiftTimelineFixture = `{
  "schema_version":1,
  "team_instance_id":"team-1",
  "view_version":"view-1",
  "next_cursor":"",
  "has_more":false,
  "gap":null,
  "records":[],
  "board":{
    "schema_version":1,
    "team_instance_id":"team-1",
    "plan_digest":"",
    "status":"ready",
    "view_version":"view-1",
    "nodes":[],
    "provider_accounts":[],
    "cost":{"observed":false,"amount_microunits":null,"currency":""}
  },
  "attention":[]
}`

const swiftSetupFixture = `{
  "schema_version":1,
  "view_version":"view-setup-1",
  "codex":{
    "provider_id":"codex",
    "auth_mode":"native_auth",
    "credential_reference":"",
    "revision":0,
    "status":"available",
    "reason":""
  },
  "minimax":{
    "provider_id":"minimax",
    "auth_mode":"brokered",
    "credential_reference":"credential-ref-1",
    "revision":1,
    "status":"configured",
    "reason":""
  },
  "runtimes":[],
  "saved_teams":[],
  "templates":[],
  "role_options":[],
  "skills":[],
  "permissions":[],
  "resources":[]
}`

const swiftCredentialVerificationFixture = `{
  "provider_id":"minimax",
  "revision":2,
  "status":"verified",
  "reason":""
}`

const swiftBuilderFixture = `{
  "schema_version":1,
  "draft_id":"draft-swift-1",
  "revision":1,
  "source":"blank",
  "catalog_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "view_version":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "content_digest":"",
  "binding_digest":"",
  "question":{"id":"team_name","prompt":"Name this team","options":[]},
  "preview":{
    "name":"",
    "purpose":"",
    "roles":[],
    "permissions":[],
    "resources":[],
    "compatibility_gaps":[],
    "requested_concurrency":0,
    "maximum_budget_credits":0,
    "estimated_maximum_cost":""
  },
  "can_confirm":false
}`

const swiftCodexConnectFixture = `{
  "provider_id":"codex",
  "auth_mode":"native_auth",
  "status":"already_connected"
}`

const swiftDecisionFixture = `{
  "schema_version":1,
  "kind":"authorization",
  "mission_id":"mission/team-1",
  "team_instance_id":"team-1",
  "view_version":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "decision_id":"decision-1",
  "decision_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "title":"Authorization required",
  "summary":"Review the prepared command.",
  "requester":"Release Team",
  "target":"repository",
  "command_type":"local process",
  "network_access":"none",
  "credential_access":"none",
  "permission_scope":"this Mission",
  "attempt_scope":"Attempt 1",
  "expected_evidence":"accepted Evidence",
  "technical_details":[],
  "actions":["not_now","deny","edit_scope","allow_once"],
  "prepared_actions":["deny","allow_once"],
  "prepared":true,
  "logical_node_id":"main",
  "attempt_number":1,
  "claim_generation":1
}`

const swiftExecutionPreflightFixture = `{
  "schema_version":1,
  "operation":"preflight",
  "preflight":{
    "schema_version":1,
    "mission_id":"mission/team-swift-contract",
    "team_instance_id":"team-swift-contract",
    "work_package_id":"work-package.coding",
    "work_package_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "view_version":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
    "plan_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
    "preflight_digest":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
    "expires_at":"2026-08-01T12:05:00Z",
    "runtime_instance_id":"runtime-pi",
    "runtime_profile_id":"pi-default",
    "model_id":"qwen",
    "auth_mode":"native",
    "capacity_available":1,
    "budget_status":"unavailable",
    "side_effects":[],
    "permission_scopes":[],
    "approval_points":[],
    "nodes":[{
      "logical_node_id":"main",
      "title":"Verify the strict Swift execution contract",
      "role":"main",
      "depends_on":[],
      "max_attempts":1,
      "harness_adapter":"pi",
      "provider_id":"loom-local",
      "provider_account_id":"",
      "model_id":"qwen",
      "auth_mode":"native_auth",
      "credential_revision":0,
      "reasoning_effort":"",
      "timeout_seconds":60,
      "budget_credits":null,
      "capabilities":[],
      "status":"ready",
      "block_reason":"",
      "fallback_configured":false,
      "fallback_harness_adapter":"",
      "fallback_provider_id":"",
      "fallback_provider_account_id":"",
      "fallback_model_id":"",
      "fallback_auth_mode":"",
      "fallback_credential_revision":0,
      "fallback_reasoning_effort":"",
      "fallback_timeout_seconds":0,
      "fallback_budget_credits":null,
      "fallback_capabilities":[],
      "fallback_status":"",
      "fallback_block_reason":"",
      "fallback_approval_required":false,
      "fallback_approval_available":false,
      "fallback_approval_version":0
    }]
  }
}`

const swiftExecutionStartFixture = `{
  "schema_version":1,
  "operation":"start",
  "result":{
    "schema_version":1,
    "mission_id":"mission/team-swift-contract",
    "team_instance_id":"team-swift-contract",
    "status":"running",
    "view_version":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
    "execution_digest":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
  }
}`

type swiftFixtureHandler struct {
	mu                           sync.Mutex
	errorCode                    string
	snapshot                     json.RawMessage
	executionOperations          []string
	executionIDs                 []string
	credentialOperations         []string
	credentialConfigureProviders []string
	credentialConfigureDigests   [][sha256.Size]byte
	timelinePages                map[string]json.RawMessage
	timelineCursors              []string
}

func (handler *swiftFixtureHandler) setTimelinePages(
	pages map[string]json.RawMessage,
) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	handler.timelinePages = make(map[string]json.RawMessage, len(pages))
	for cursor, page := range pages {
		handler.timelinePages[cursor] = append(json.RawMessage(nil), page...)
	}
}

func (handler *swiftFixtureHandler) setErrorCode(code string) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	handler.errorCode = code
}

func (handler *swiftFixtureHandler) setSnapshot(snapshot json.RawMessage) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	handler.snapshot = append(json.RawMessage(nil), snapshot...)
}

func (handler *swiftFixtureHandler) Handle(
	_ context.Context,
	request Request,
) Response {
	handler.mu.Lock()
	code := handler.errorCode
	snapshot := append(json.RawMessage(nil), handler.snapshot...)
	handler.mu.Unlock()
	if code != "" {
		return Response{Error: safeProtocolError(code, errors.New("private"))}
	}
	switch request.Method {
	case "snapshot":
		if len(snapshot) != 0 {
			return Response{OK: true, Result: snapshot}
		}
		return Response{OK: true, Result: json.RawMessage(swiftSnapshotFixture)}
	case "timeline_page":
		var input struct {
			Cursor string `json:"cursor"`
		}
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return Response{Error: safeProtocolError("invalid_request", err)}
		}
		handler.mu.Lock()
		handler.timelineCursors = append(handler.timelineCursors, input.Cursor)
		page := append(json.RawMessage(nil), handler.timelinePages[input.Cursor]...)
		handler.mu.Unlock()
		if len(page) != 0 {
			return Response{OK: true, Result: page}
		}
		return Response{OK: true, Result: json.RawMessage(swiftTimelineFixture)}
	case "setup_snapshot":
		return Response{OK: true, Result: json.RawMessage(swiftSetupFixture)}
	case "codex_connect":
		return Response{
			OK:     true,
			Result: json.RawMessage(swiftCodexConnectFixture),
		}
	case "builder_start":
		return Response{OK: true, Result: json.RawMessage(swiftBuilderFixture)}
	case "credential_configure":
		var input struct {
			ProviderID          string `json:"provider_id"`
			ProviderAccountID   string `json:"provider_account_id"`
			CredentialReference string `json:"credential_reference"`
			ExpectedRevision    int64  `json:"expected_revision"`
			OperationID         string `json:"operation_id"`
			Secret              string `json:"secret"`
		}
		if err := json.Unmarshal(request.Params, &input); err != nil ||
			input.ProviderID != "deepseek" ||
			input.ProviderAccountID != "deepseek.primary" ||
			input.CredentialReference != "" ||
			input.ExpectedRevision != 0 || input.OperationID != "" ||
			input.Secret != "sk-deepseek-contract" {
			return Response{Error: safeProtocolError(
				"invalid_request",
				errors.New("credential configure operation"),
			)}
		}
		digest := sha256.Sum256([]byte(input.Secret))
		input.Secret = ""
		handler.mu.Lock()
		handler.credentialConfigureProviders = append(
			handler.credentialConfigureProviders,
			input.ProviderID,
		)
		handler.credentialConfigureDigests = append(
			handler.credentialConfigureDigests,
			digest,
		)
		handler.mu.Unlock()
		return Response{OK: true, Result: json.RawMessage(`{
  "provider_id":"deepseek",
  "revision":1,
  "status":"configured",
  "reason":""
}`)}
	case "credential_verify":
		var input struct {
			ProviderID          string `json:"provider_id"`
			CredentialReference string `json:"credential_reference"`
			ExpectedRevision    int64  `json:"expected_revision"`
			OperationID         string `json:"operation_id"`
			Secret              string `json:"secret"`
		}
		if err := json.Unmarshal(request.Params, &input); err != nil ||
			input.ProviderID != "minimax" ||
			input.CredentialReference != "credential-ref-1" ||
			input.ExpectedRevision != 1 || input.Secret != "" ||
			!validSwiftOperationID(input.OperationID) {
			return Response{Error: safeProtocolError(
				"invalid_request",
				errors.New("credential operation"),
			)}
		}
		handler.mu.Lock()
		handler.credentialOperations = append(
			handler.credentialOperations,
			input.OperationID,
		)
		handler.mu.Unlock()
		return Response{
			OK:     true,
			Result: json.RawMessage(swiftCredentialVerificationFixture),
		}
	case "mission_decision":
		return Response{OK: true, Result: json.RawMessage(swiftDecisionFixture)}
	case "mission_execution":
		var input struct {
			Operation     string `json:"operation"`
			CorrelationID string `json:"correlation_id"`
		}
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return Response{Error: safeProtocolError("invalid_request", err)}
		}
		handler.mu.Lock()
		handler.executionOperations = append(
			handler.executionOperations,
			input.Operation,
		)
		handler.executionIDs = append(handler.executionIDs, input.CorrelationID)
		handler.mu.Unlock()
		if input.Operation == "preflight" {
			return Response{OK: true, Result: json.RawMessage(swiftExecutionPreflightFixture)}
		}
		if input.Operation == "start" {
			return Response{OK: true, Result: json.RawMessage(swiftExecutionStartFixture)}
		}
		return Response{Error: safeProtocolError("invalid_request", errors.New("operation"))}
	default:
		return Response{}
	}
}

func validSwiftOperationID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' ||
		value[18] != '-' || value[23] != '-' || value[14] != '4' ||
		!strings.Contains("89ab", string(value[19])) {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

type swiftCredentialIdentity struct{}

type swiftCredentialProbe struct{}

func (swiftCredentialProbe) ID() string { return "swift-credential-probe" }

func (swiftCredentialProbe) ObserveRuntime(
	context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	return []loomruntime.RuntimeObservation{{
		Instance: loomruntime.RuntimeInstance{
			ID: "runtime-swift", DeviceID: "device-swift", AdapterType: "pi",
			DisplayName: "Swift Fixture Runtime", ExecutableVersion: "0.82.1",
			Status:               loomruntime.RuntimeOnline,
			ObservedCapabilities: []string{"rpc"}, Capacity: 1,
		},
		ModelIDs: []string{"model-swift"},
	}}, nil
}

func (swiftCredentialIdentity) NextSetupID(kind string) (string, error) {
	return kind + "-swift-live", nil
}

type swiftCredentialNativeAuth struct{}

func (swiftCredentialNativeAuth) ObserveNativeAuth(
	context.Context,
) (app.NativeAuthObservation, error) {
	return app.NativeAuthObservation{
		Status: "available", AuthMode: "native_auth",
	}, nil
}

type swiftCredentialStore struct {
	secret  []byte
	readErr error
}

func (store *swiftCredentialStore) Put(
	context.Context,
	string,
	[]byte,
) error {
	return errors.New("unexpected put")
}

func (store *swiftCredentialStore) Read(
	context.Context,
	string,
) ([]byte, error) {
	if store.readErr != nil {
		return nil, store.readErr
	}
	return append([]byte(nil), store.secret...), nil
}

func (store *swiftCredentialStore) Delete(
	context.Context,
	string,
) error {
	return errors.New("unexpected delete")
}

type swiftCredentialVerifier struct {
	mu    sync.Mutex
	calls int
}

func (verifier *swiftCredentialVerifier) Verify(
	context.Context,
	string,
	[]byte,
) (credentials.VerificationResult, error) {
	verifier.mu.Lock()
	verifier.calls++
	verifier.mu.Unlock()
	return credentials.VerificationResult{
		Status: credentials.VerificationValid,
		Reason: credentials.VerificationReasonNone,
	}, nil
}

type swiftCredentialStatusSource struct {
	projection *projection.Projection
}

func (source swiftCredentialStatusSource) CredentialStatus(
	ctx context.Context,
	providerID string,
) (credentials.MetadataResult, error) {
	if err := source.projection.Rebuild(ctx); err != nil {
		return credentials.MetadataResult{}, err
	}
	record, ok := source.projection.GlobalReadView().ProviderCredential(providerID)
	if !ok {
		return credentials.MetadataResult{}, credentials.ErrCredentialNotFound
	}
	return credentials.MetadataResult{
		ProviderID: record.ProviderID, CredentialReference: record.CredentialReference,
		Revision: record.Revision, Status: credentials.CredentialStatus(record.Status),
		Reason: credentials.VerificationReason(record.Reason),
	}, nil
}

type swiftCredentialLiveHandler struct {
	setup       *api.LocalProductSetupAPI
	mu          sync.Mutex
	operationID string
	lastErr     error
}

func swiftCredentialCatalog(t *testing.T) app.LocalProductSetupCatalog {
	t.Helper()
	discovery, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{swiftCredentialProbe{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	definitions := []agents.AgentDefinition{
		{
			ID: "agent-swift-main", Version: 1, Scope: agents.ScopeReusable,
			Name: "Swift Main", RoleSpec: "Coordinate bounded work",
			Status: agents.DefinitionActive,
		},
		{
			ID: "agent-swift-sub", Version: 1, Scope: agents.ScopeReusable,
			Name: "Swift Sub", RoleSpec: "Review bounded work",
			Status: agents.DefinitionActive,
		},
	}
	mainProfile := loomruntime.RuntimeProfile{
		ID: "profile-swift-main", AdapterType: "pi", ProviderID: "local",
		ModelID: "model-swift", AuthMode: loomruntime.AuthNative,
		RequiredCapabilities: []string{"rpc"}, Timeout: time.Minute,
	}
	subProfile := mainProfile
	subProfile.ID = "profile-swift-sub"
	return app.LocalProductSetupCatalog{
		CatalogDigest: strings.Repeat("a", 64), AgentDefinitions: definitions,
		RuntimeProfiles:  []loomruntime.RuntimeProfile{mainProfile, subProfile},
		RuntimeDiscovery: discovery,
		RoleOptions: []app.SetupRoleOption{
			{
				ID: "role-swift-main", Kind: "main",
				AgentDefinitionID: definitions[0].ID, RuntimeProfileID: mainProfile.ID,
				RuntimeInstanceID: "runtime-swift", SkillRevisionIDs: []string{},
				PermissionIDs: []string{}, ResourceIDs: []string{},
				Responsibility: "Coordinate",
			},
			{
				ID: "role-swift-sub", Kind: "subagent",
				AgentDefinitionID: definitions[1].ID, RuntimeProfileID: subProfile.ID,
				RuntimeInstanceID: "runtime-swift", SkillRevisionIDs: []string{},
				PermissionIDs: []string{}, ResourceIDs: []string{},
				Responsibility: "Review",
			},
		},
		Templates: []app.SetupTeamTemplate{{
			ID: "template-swift", Version: 1, Digest: strings.Repeat("b", 64),
			Name: "Swift Team", Purpose: "Verify one credential operation",
			MainRoleID: "role-swift-main", SubagentRoleIDs: []string{"role-swift-sub"},
			RequestedConcurrency: 1, MaximumBudgetCredits: 10,
		}},
		BudgetCeiling: 10, ConcurrencyCeiling: 1,
	}
}

func (handler *swiftCredentialLiveHandler) Handle(
	ctx context.Context,
	request Request,
) Response {
	if request.Method == "setup_snapshot" {
		result, err := handler.setup.SetupSnapshot(ctx)
		if err != nil {
			return Response{Error: safeProtocolError("state_unavailable", err)}
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return Response{Error: safeProtocolError("internal", err)}
		}
		return Response{OK: true, Result: encoded}
	}
	if request.Method != "credential_verify" {
		return Response{Error: safeProtocolError(
			"unknown_method",
			errors.New("unknown method"),
		)}
	}
	var input struct {
		ProviderID          string `json:"provider_id"`
		ProviderAccountID   string `json:"provider_account_id"`
		CredentialReference string `json:"credential_reference"`
		ExpectedRevision    int64  `json:"expected_revision"`
		OperationID         string `json:"operation_id"`
		Secret              string `json:"secret"`
	}
	if err := json.Unmarshal(request.Params, &input); err != nil ||
		input.ProviderAccountID != input.ProviderID+".primary" ||
		input.Secret != "" || !validSwiftOperationID(input.OperationID) {
		return Response{Error: safeProtocolError(
			"invalid_request",
			errors.New("credential params"),
		)}
	}
	result, err := handler.setup.VerifyCredential(
		ctx,
		app.CredentialSetupCommand{
			ProviderID: input.ProviderID, ProviderAccountID: input.ProviderAccountID,
			CredentialReference: input.CredentialReference,
			ExpectedRevision:    input.ExpectedRevision, OperationID: input.OperationID,
		},
	)
	if err != nil {
		handler.mu.Lock()
		handler.lastErr = err
		handler.mu.Unlock()
		return Response{Error: safeProtocolError("conflict", err)}
	}
	handler.mu.Lock()
	handler.operationID = input.OperationID
	handler.mu.Unlock()
	encoded, err := json.Marshal(result)
	if err != nil {
		return Response{Error: safeProtocolError("internal", err)}
	}
	return Response{OK: true, Result: encoded}
}

func TestStrictSwiftContractProbeExecutesMissionThroughRealGoServer(t *testing.T) {
	probe := buildSwiftContractProbe(t)
	root, socketPath := swiftPrivateSocketRoot(t)
	defer os.RemoveAll(root)
	handler := &swiftFixtureHandler{}
	server, err := NewServer(ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(),
		BuildID: "swift-execution-contract-fixture", Handler: handler,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case err := <-serveDone:
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	output, runErr := exec.Command(
		probe, "--socket", socketPath, "--execution",
	).CombinedOutput()
	cancel()
	_ = server.Close()
	<-serveDone
	if runErr != nil {
		t.Fatalf("Swift execution probe error = %v, output = %q", runErr, output)
	}
	var actual struct {
		PreflightDigest string `json:"preflightDigest"`
		ExpiresAt       string `json:"expiresAt"`
		Status          string `json:"status"`
		ExecutionDigest string `json:"executionDigest"`
	}
	if err := json.Unmarshal(output, &actual); err != nil ||
		actual.PreflightDigest != strings.Repeat("d", 64) ||
		actual.ExpiresAt != "2026-08-01T12:05:00Z" ||
		actual.Status != "running" ||
		actual.ExecutionDigest != strings.Repeat("f", 64) {
		t.Fatalf("Swift execution output = %#v, %v, %q", actual, err, output)
	}
	handler.mu.Lock()
	operations := append([]string(nil), handler.executionOperations...)
	correlations := append([]string(nil), handler.executionIDs...)
	handler.mu.Unlock()
	if strings.Join(operations, ",") != "preflight,start" ||
		len(correlations) != 2 || correlations[0] == correlations[1] {
		t.Fatalf("execution calls = %v %v", operations, correlations)
	}
}

func TestStrictSwiftExecutionProbeRejectsMalformedPreflightWire(t *testing.T) {
	probe := buildSwiftContractProbe(t)
	cases := map[string]string{
		"unknown_field": strings.Replace(
			swiftExecutionPreflightFixture,
			`"operation":"preflight",`,
			`"operation":"preflight","unknown":true,`,
			1,
		),
		"duplicate_key": strings.Replace(
			swiftExecutionPreflightFixture,
			`"operation":"preflight",`,
			`"operation":"preflight","operation":"preflight",`,
			1,
		),
		"null_collection": strings.Replace(
			swiftExecutionPreflightFixture,
			`"permission_scopes":[],`,
			`"permission_scopes":null,`,
			1,
		),
		"identity_drift": strings.Replace(
			swiftExecutionPreflightFixture,
			`"mission_id":"mission/team-swift-contract"`,
			`"mission_id":"mission-drift"`,
			1,
		),
	}
	for name, malformed := range cases {
		t.Run(name, func(t *testing.T) {
			root, socketPath := swiftPrivateSocketRoot(t)
			defer os.RemoveAll(root)
			done := startRawSwiftFixture(t, socketPath, func(
				id string,
				connection *net.UnixConn,
			) {
				body := []byte(
					`{"version":1,"request_id":"` + id +
						`","ok":true,"result":` + malformed +
						`,"error":null}`,
				)
				writeRawFrame(t, connection, body)
			})
			output, err := exec.Command(
				probe, "--socket", socketPath, "--execution",
			).CombinedOutput()
			if err == nil ||
				strings.TrimSpace(string(output)) != "error:invalid_response" {
				t.Fatalf("malformed execution result = %v, %q", err, output)
			}
			<-done
		})
	}
}

func TestStrictSwiftClientReadsPreparedDecisionFromRealGoServer(t *testing.T) {
	probe := buildSwiftContractProbe(t)
	root, socketPath := swiftPrivateSocketRoot(t)
	defer os.RemoveAll(root)

	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "swift-decision-contract-fixture",
		Handler:      &swiftFixtureHandler{},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case err := <-serveDone:
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	defer func() {
		_ = server.Close()
		<-serveDone
	}()

	output, runErr := exec.Command(
		probe,
		"--socket",
		socketPath,
		"--decision",
		"authorization",
	).CombinedOutput()
	if runErr != nil {
		t.Fatalf("Swift decision probe error = %v, output = %q", runErr, output)
	}
	var actual struct {
		Kind      string   `json:"kind"`
		MissionID string   `json:"missionID"`
		Actions   []string `json:"actions"`
		Prepared  bool     `json:"prepared"`
	}
	if err := json.Unmarshal(output, &actual); err != nil {
		t.Fatalf("decision output invalid: %v, output = %q", err, output)
	}
	if actual.Kind != "authorization" ||
		actual.MissionID != "mission/team-1" ||
		!actual.Prepared ||
		!equalStrings(
			actual.Actions,
			[]string{"not_now", "deny", "edit_scope", "allow_once"},
		) {
		t.Fatalf("decision output = %#v", actual)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestSwiftClientInteroperatesWithRealGoServer(t *testing.T) {
	probe := buildSwiftContractProbe(t)
	root, socketPath := swiftPrivateSocketRoot(t)
	defer os.RemoveAll(root)

	handler := &swiftFixtureHandler{}
	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "swift-contract-fixture",
		Handler:      handler,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	defer func() {
		_ = server.Close()
		select {
		case err := <-serveDone:
			if err != nil {
				t.Errorf("Serve() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("server did not close")
		}
	}()

	output, runErr := exec.Command(
		probe,
		"--socket",
		socketPath,
		"--team",
		"team-1",
	).CombinedOutput()
	if runErr != nil {
		t.Fatalf("Swift probe error = %v, output = %q", runErr, output)
	}
	var actual struct {
		Snapshot json.RawMessage `json:"snapshot"`
		Timeline json.RawMessage `json:"timeline"`
	}
	if err := json.Unmarshal(output, &actual); err != nil {
		t.Fatalf("probe output invalid: %v, output = %q", err, output)
	}
	assertJSONSemanticEqual(t, actual.Snapshot, []byte(swiftSnapshotFixture))
	assertJSONSemanticEqual(t, actual.Timeline, []byte(swiftTimelineFixture))

	handler.setSnapshot(json.RawMessage(swiftPartialSnapshotFixture))
	output, runErr = exec.Command(
		probe,
		"--socket",
		socketPath,
	).CombinedOutput()
	if runErr != nil {
		t.Fatalf("Swift partial-health probe error = %v, output = %q", runErr, output)
	}
	actual = struct {
		Snapshot json.RawMessage `json:"snapshot"`
		Timeline json.RawMessage `json:"timeline"`
	}{}
	if err := json.Unmarshal(output, &actual); err != nil {
		t.Fatalf("partial-health output invalid: %v, output = %q", err, output)
	}
	assertJSONSemanticEqual(t, actual.Snapshot, []byte(swiftPartialSnapshotFixture))
	handler.setSnapshot(nil)

	for _, code := range []string{
		"invalid_request",
		"unsupported_version",
		"unknown_method",
		"unauthorized_peer",
		"unsupported_platform",
		"not_found",
		"cursor_conflict",
		"stream_gap",
		"state_unavailable",
		"timeout",
		"busy",
		"internal",
	} {
		handler.setErrorCode(code)
		output, runErr = exec.Command(
			probe,
			"--socket",
			socketPath,
		).CombinedOutput()
		if runErr == nil {
			t.Fatalf("Swift probe accepted %q error", code)
		}
		_, recoverable, ok := protocolErrorDefinition(code)
		if !ok {
			t.Fatalf("missing Go error definition for %q", code)
		}
		expected := "error:" + code + ":" +
			map[bool]string{true: "true", false: "false"}[recoverable]
		if strings.TrimSpace(string(output)) != expected {
			t.Fatalf(
				"Swift probe error output for %q = %q, want %q",
				code,
				output,
				expected,
			)
		}
	}
}

func TestStrictSwiftStoreAggregatesBoundedTimelinePagesThroughRealGoServer(
	t *testing.T,
) {
	probe := buildSwiftContractProbe(t)
	root, socketPath := swiftPrivateSocketRoot(t)
	defer os.RemoveAll(root)
	page := func(next string, more bool, record string) json.RawMessage {
		return json.RawMessage(`{
  "schema_version":1,"team_instance_id":"team-1","view_version":"view-1",
  "next_cursor":"` + next + `","has_more":` + strconv.FormatBool(more) + `,
  "gap":null,"records":[` + record + `],
  "board":{"schema_version":1,"team_instance_id":"team-1","plan_digest":"",
    "status":"succeeded","view_version":"view-1","nodes":[],
    "cost":{"observed":false,"amount_microunits":null,"currency":""}},
  "attention":[]}`)
	}
	record := func(id, kind, status string) string {
		return `{"schema_version":1,"delivery_id":"` + id + `","kind":"` + kind +
			`","authority":"journal","team_instance_id":"team-1",` +
			`"logical_node_id":"main","attempt_number":2,` +
			`"source_stream_id":"work-item/work-1","source_sequence":1,` +
			`"source_event_id":"event-1","occurred_at":"2026-08-03T00:00:00Z",` +
			`"cursor":"record-cursor","payload":{"status":"` + status +
			`","reason_code":"","action":"","warning_code":"","retry_at":"",` +
			`"text_delta":"","evidence_digest":"` + strings.Repeat("a", 64) +
			`","cost":{"observed":false,"amount_microunits":null,"currency":""}}}`
	}
	handler := &swiftFixtureHandler{}
	handler.setSnapshot(json.RawMessage(strings.Replace(
		swiftSnapshotFixture,
		`"teams":[]`,
		`"teams":[{"team_instance_id":"team-1","display_name":"Review Team",`+
			`"source_kind":"saved","state":"ready","confirmed":true,`+
			`"executable":true,"read_only":false}]`,
		1,
	)))
	handler.setTimelinePages(map[string]json.RawMessage{
		"": page("Y3Vyc29yLTE", true,
			record("delivery-source", "evidence_available", "")),
		"Y3Vyc29yLTE": page("Y3Vyc29yLTI", false,
			record("delivery-verifier", "evidence_available", "")+","+
				record("delivery-terminal", "team_terminal", "succeeded")),
	})
	server, err := NewServer(ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(),
		BuildID: "swift-store-pagination-fixture", Handler: handler,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case err := <-serveDone:
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	output, runErr := exec.Command(
		probe, "--socket", socketPath, "--team-all", "team-1",
	).CombinedOutput()
	cancel()
	_ = server.Close()
	<-serveDone
	if runErr != nil {
		t.Fatalf("Swift Store probe error = %v, output = %q", runErr, output)
	}
	var actual struct {
		Timeline struct {
			HasMore bool `json:"has_more"`
			Records []struct {
				DeliveryID string `json:"delivery_id"`
			} `json:"records"`
		} `json:"timeline"`
	}
	if err := json.Unmarshal(output, &actual); err != nil {
		t.Fatalf("probe output invalid: %v, output = %q", err, output)
	}
	handler.mu.Lock()
	cursors := append([]string(nil), handler.timelineCursors...)
	handler.mu.Unlock()
	if actual.Timeline.HasMore ||
		!equalStrings(cursors, []string{"", "Y3Vyc29yLTE"}) ||
		len(actual.Timeline.Records) != 3 ||
		actual.Timeline.Records[2].DeliveryID != "delivery-terminal" {
		t.Fatalf("timeline=%#v cursors=%#v", actual.Timeline, cursors)
	}
}

func TestStrictSwiftClientReadsSetupAndStartsCandidateFromRealGoServer(
	t *testing.T,
) {
	probe := buildSwiftSetupContractProbe(t)
	root, socketPath := swiftPrivateSocketRoot(t)
	defer os.RemoveAll(root)
	handler := &swiftFixtureHandler{}
	server, err := NewServer(ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "swift-setup-contract-fixture",
		Handler:      handler,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case err := <-serveDone:
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	output, runErr := exec.Command(
		probe,
		"--socket",
		socketPath,
	).CombinedOutput()
	deepSeekOutput, deepSeekErr := exec.Command(
		probe,
		"--socket",
		socketPath,
		"--deepseek-configure",
	).CombinedOutput()
	cancel()
	if closeErr := server.Close(); closeErr != nil {
		t.Errorf("Close() error = %v", closeErr)
	}
	select {
	case err := <-serveDone:
		if err != nil {
			t.Errorf("Serve() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("server did not close")
	}
	if runErr != nil {
		t.Fatalf("Swift setup probe error = %v, output = %q", runErr, output)
	}
	if deepSeekErr != nil {
		t.Fatalf("Swift DeepSeek configure probe error = %v, output = %q",
			deepSeekErr, deepSeekOutput)
	}
	var actual struct {
		SchemaVersion int    `json:"schema_version"`
		CodexAuthMode string `json:"codex_auth_mode"`
		MiniMaxMode   string `json:"minimax_auth_mode"`
		RuntimeCount  int    `json:"runtime_count"`
		ConnectStatus string `json:"connect_status"`
		DraftID       string `json:"draft_id"`
		QuestionID    string `json:"question_id"`
		CanConfirm    bool   `json:"can_confirm"`
		VerifyStatus  string `json:"verify_status"`
	}
	if err := json.Unmarshal(output, &actual); err != nil {
		t.Fatalf("Swift setup output invalid: %v, output = %q", err, output)
	}
	if actual.SchemaVersion != 1 ||
		actual.CodexAuthMode != "native_auth" ||
		actual.MiniMaxMode != "brokered" ||
		actual.RuntimeCount != 0 ||
		actual.ConnectStatus != "already_connected" ||
		actual.DraftID != "draft-swift-1" ||
		actual.QuestionID != "team_name" ||
		actual.CanConfirm {
		t.Fatalf("Swift setup output = %#v", actual)
	}
	if actual.VerifyStatus != "verified" {
		t.Fatalf("Swift verification output = %#v", actual)
	}
	handler.mu.Lock()
	operations := append([]string(nil), handler.credentialOperations...)
	configureProviders := append(
		[]string(nil),
		handler.credentialConfigureProviders...,
	)
	configureDigests := append(
		[][sha256.Size]byte(nil),
		handler.credentialConfigureDigests...,
	)
	handler.mu.Unlock()
	if len(operations) != 2 || operations[0] == operations[1] ||
		!validSwiftOperationID(operations[0]) ||
		!validSwiftOperationID(operations[1]) {
		t.Fatalf("Swift credential operation IDs = %#v", operations)
	}
	var deepSeekResult struct {
		ProviderID string `json:"provider_id"`
		Revision   int64  `json:"revision"`
		Status     string `json:"status"`
	}
	wantDigest := sha256.Sum256([]byte("sk-deepseek-contract"))
	if err := json.Unmarshal(deepSeekOutput, &deepSeekResult); err != nil ||
		deepSeekResult.ProviderID != "deepseek" ||
		deepSeekResult.Revision != 1 ||
		deepSeekResult.Status != "configured" ||
		!equalStrings(configureProviders, []string{"deepseek"}) ||
		len(configureDigests) != 1 || configureDigests[0] != wantDigest {
		t.Fatalf("DeepSeek result=%#v providers=%#v digests=%d error=%v",
			deepSeekResult, configureProviders, len(configureDigests), err)
	}
	assertStrictSwiftCredentialClosesThroughRealServiceAndBroker(t, probe)
}

func assertStrictSwiftCredentialClosesThroughRealServiceAndBroker(
	t *testing.T,
	probe string,
) {
	t.Helper()
	for _, scenario := range []struct {
		name            string
		initialStatus   credentials.CredentialStatus
		store           *swiftCredentialStore
		wantStatus      string
		wantReason      string
		wantVerifyCalls int
	}{
		{
			name: "provider_verified", initialStatus: credentials.CredentialConfigured,
			store:      &swiftCredentialStore{secret: []byte("private-swift-secret")},
			wantStatus: "verified", wantVerifyCalls: 1,
		},
		{
			name: "store_unavailable", initialStatus: credentials.CredentialVerified,
			store:      &swiftCredentialStore{readErr: credentials.ErrCredentialStoreDenied},
			wantStatus: "rejected", wantReason: "unavailable", wantVerifyCalls: 0,
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			runStrictSwiftCredentialScenario(t, probe, scenario.initialStatus,
				scenario.store, scenario.wantStatus, scenario.wantReason,
				scenario.wantVerifyCalls)
		})
	}
}

func runStrictSwiftCredentialScenario(
	t *testing.T,
	probe string,
	initialStatus credentials.CredentialStatus,
	secretStore *swiftCredentialStore,
	wantStatus string,
	wantReason string,
	wantVerifyCalls int,
) {
	t.Helper()
	root, socketPath := swiftPrivateSocketRoot(t)
	defer os.RemoveAll(root)
	database, err := sql.Open("sqlite", filepath.Join(root, "credential.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID: "configure-credential-swift-live", ProviderID: "minimax",
			CredentialReference: "credential-ref-1", ExpectedRevision: 0,
			OccurredAt: time.Unix(1_800, 0).UTC(),
			Status:     credentials.CredentialConfigured,
		},
	); err != nil {
		t.Fatal(err)
	}
	initialRevision := int64(1)
	if initialStatus == credentials.CredentialVerified {
		if _, err := writer.CommitCredentialMetadata(
			context.Background(),
			credentials.MetadataCommand{
				CommandID: "verify-credential-swift-inherited", ProviderID: "minimax",
				CredentialReference: "credential-ref-1", ExpectedRevision: 1,
				OccurredAt: time.Unix(1_801, 0).UTC(),
				Status:     credentials.CredentialVerified,
			},
		); err != nil {
			t.Fatal(err)
		}
		initialRevision = 2
	}
	readModel := projection.New(database)
	status := swiftCredentialStatusSource{projection: readModel}
	verifier := &swiftCredentialVerifier{}
	broker, err := credentials.NewCredentialBroker(credentials.CredentialBrokerConfig{
		Store:    secretStore,
		Verifier: verifier, Committer: writer,
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewLocalProductSetupService(app.LocalProductSetupConfig{
		Journal: store, Projection: readModel, Writer: writer,
		Catalog:    swiftCredentialCatalog(t),
		Identity:   swiftCredentialIdentity{},
		Now:        func() time.Time { return time.Unix(1_801, 0).UTC() },
		NativeAuth: swiftCredentialNativeAuth{}, Credentials: status,
		CredentialMutator: broker,
	})
	if err != nil {
		t.Fatal(err)
	}
	setupAPI, err := api.NewLocalProductSetupAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	handler := &swiftCredentialLiveHandler{setup: setupAPI}
	server, err := NewServer(ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(),
		BuildID: "swift-live-credential-service", Handler: handler,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case err := <-serveDone:
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	output, runErr := exec.Command(
		probe, "--socket", socketPath, "--credential",
	).CombinedOutput()
	cancel()
	_ = server.Close()
	<-serveDone
	if runErr != nil {
		handler.mu.Lock()
		handlerErr := handler.lastErr
		handler.mu.Unlock()
		t.Fatalf(
			"Swift credential probe error = %v, handler_error=%v, output = %q",
			runErr, handlerErr, output,
		)
	}
	var result struct {
		Revision int64  `json:"revision"`
		Status   string `json:"status"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(output, &result); err != nil ||
		result.Revision != initialRevision+1 || result.Status != wantStatus ||
		result.Reason != wantReason {
		t.Fatalf("Swift credential result=%#v error=%v output=%q", result, err, output)
	}
	verifier.mu.Lock()
	verifyCalls := verifier.calls
	verifier.mu.Unlock()
	handler.mu.Lock()
	operationID := handler.operationID
	handler.mu.Unlock()
	events, err := store.ReadStream(
		context.Background(),
		"provider-credential/minimax",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != int(initialRevision+1) {
		t.Fatalf("events=%#v want_count=%d", events, initialRevision+1)
	}
	terminal := events[len(events)-1]
	if verifyCalls != wantVerifyCalls || !validSwiftOperationID(operationID) ||
		terminal.Type != "ProviderCredentialVerified" ||
		terminal.Seq != initialRevision+1 ||
		terminal.IdempotencyKey !=
			"local-product-setup/verify-credential-"+operationID ||
		bytes.Contains(terminal.PayloadJSON, []byte("private-swift-secret")) {
		t.Fatalf(
			"calls=%d operation=%q events=%#v",
			verifyCalls,
			operationID,
			events,
		)
	}
	if wantReason != "" &&
		!bytes.Contains(terminal.PayloadJSON, []byte(`"reason":"`+wantReason+`"`)) {
		t.Fatalf("terminal reason missing from event: %s", terminal.PayloadJSON)
	}
}

func TestSwiftClientRejectsMalformedLoopbackResponses(t *testing.T) {
	probe := buildSwiftContractProbe(t)
	cases := map[string]func(string) []byte{
		"null_model_ids": swiftInvalidSnapshotResponse(
			`"model_ids":[]`,
			`"model_ids":null`,
		),
		"null_observed_capabilities": swiftInvalidSnapshotResponse(
			`"observed_capabilities":[]`,
			`"observed_capabilities":null`,
		),
		"missing_model_ids": swiftInvalidSnapshotResponse(
			`    "model_ids":[],`+"\n",
			"",
		),
		"wrong_type_model_ids": swiftInvalidSnapshotResponse(
			`"model_ids":[]`,
			`"model_ids":"model-a"`,
		),
		"duplicate_model_ids": swiftInvalidSnapshotResponse(
			`"model_ids":[]`,
			`"model_ids":[],"model_ids":[]`,
		),
		"unknown_runtime_field": swiftInvalidSnapshotResponse(
			`"observed_capabilities":[]`,
			`"observed_capabilities":[],"extra":true`,
		),
		"duplicate": func(id string) []byte {
			return []byte(`{"version":1,"version":1,"request_id":"` + id +
				`","ok":true,"result":{},"error":null}`)
		},
		"unknown": func(id string) []byte {
			return []byte(`{"version":1,"request_id":"` + id +
				`","ok":true,"result":{},"error":null,"extra":1}`)
		},
		"trailing": func(id string) []byte {
			return []byte(`{"version":1,"request_id":"` + id +
				`","ok":true,"result":{},"error":null}x`)
		},
		"mismatched_id": func(string) []byte {
			return []byte(
				`{"version":1,"request_id":"other","ok":true,` +
					`"result":{},"error":null}`,
			)
		},
		"invalid_shape": func(id string) []byte {
			return []byte(`{"version":1,"request_id":"` + id +
				`","ok":true,"result":{},"error":` +
				`{"code":"internal","message":"internal error"}}`)
		},
	}
	for name, malformed := range cases {
		t.Run(name, func(t *testing.T) {
			root, socketPath := swiftPrivateSocketRoot(t)
			defer os.RemoveAll(root)
			done := startRawSwiftFixture(t, socketPath, func(
				id string,
				connection *net.UnixConn,
			) {
				writeRawFrame(t, connection, malformed(id))
			})
			output, err := exec.Command(
				probe,
				"--socket",
				socketPath,
			).CombinedOutput()
			if err == nil ||
				strings.TrimSpace(string(output)) != "error:invalid_response" {
				t.Fatalf("malformed response result = %v, %q", err, output)
			}
			<-done
		})
	}

	t.Run("oversized", func(t *testing.T) {
		root, socketPath := swiftPrivateSocketRoot(t)
		defer os.RemoveAll(root)
		done := startRawSwiftFixture(t, socketPath, func(
			_ string,
			connection *net.UnixConn,
		) {
			var length [4]byte
			binary.BigEndian.PutUint32(
				length[:],
				uint32(maxResponseBodyBytes+1),
			)
			if _, err := connection.Write(length[:]); err != nil {
				t.Errorf("write oversized length: %v", err)
			}
			_ = connection.Close()
		})
		output, err := exec.Command(
			probe,
			"--socket",
			socketPath,
		).CombinedOutput()
		if err == nil ||
			strings.TrimSpace(string(output)) != "error:invalid_response" {
			t.Fatalf("oversized response result = %v, %q", err, output)
		}
		<-done
	})
}

func swiftInvalidSnapshotResponse(
	old string,
	replacement string,
) func(string) []byte {
	return func(id string) []byte {
		snapshot := strings.Replace(
			swiftSnapshotFixture,
			old,
			replacement,
			1,
		)
		return []byte(
			`{"version":1,"request_id":"` + id +
				`","ok":true,"result":` + snapshot +
				`,"error":null}`,
		)
	}
}

func startRawSwiftFixture(
	t *testing.T,
	socketPath string,
	writeMalformed func(string, *net.UnixConn),
) <-chan struct{} {
	t.Helper()
	listener, err := net.ListenUnix(
		"unix",
		&net.UnixAddr{Name: socketPath, Net: "unix"},
	)
	if err != nil {
		t.Fatalf("ListenUnix() error = %v", err)
	}
	listener.SetUnlinkOnClose(true)
	if err := os.Chmod(socketPath, 0o600); err != nil {
		t.Fatalf("chmod socket: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer listener.Close()
		ping, err := listener.AcceptUnix()
		if err != nil {
			t.Errorf("accept ping: %v", err)
			return
		}
		pingBody, err := readFrame(ping, maxRequestBodyBytes)
		if err != nil {
			t.Errorf("read ping: %v", err)
			_ = ping.Close()
			return
		}
		pingID := requestIDFromUntrustedBody(pingBody)
		pingResult := json.RawMessage(
			`{"protocol_version":1,"available":true,"build_id":"raw-fixture"}`,
		)
		encoded, _ := encodeResponse(Response{
			Version:   protocolVersion,
			RequestID: pingID,
			OK:        true,
			Result:    pingResult,
		})
		if err := writeFrame(ping, encoded, maxResponseBodyBytes); err != nil {
			t.Errorf("write ping: %v", err)
		}
		_ = ping.Close()

		snapshot, err := listener.AcceptUnix()
		if err != nil {
			t.Errorf("accept snapshot: %v", err)
			return
		}
		body, err := readFrame(snapshot, maxRequestBodyBytes)
		if err != nil {
			t.Errorf("read snapshot: %v", err)
			_ = snapshot.Close()
			return
		}
		writeMalformed(requestIDFromUntrustedBody(body), snapshot)
	}()
	return done
}

func writeRawFrame(t *testing.T, connection *net.UnixConn, body []byte) {
	t.Helper()
	if err := writeFrame(connection, body, maxResponseBodyBytes); err != nil {
		t.Errorf("write malformed frame: %v", err)
	}
	_ = connection.Close()
}

func buildSwiftContractProbe(t *testing.T) string {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	packageRoot := filepath.Join(repoRoot, "apps", "macos")
	command := exec.Command(
		"/usr/bin/swift",
		"build",
		"--package-path",
		packageRoot,
		"-c",
		"release",
		"--arch",
		"arm64",
		"--product",
		"LoomLocalAppContractProbe",
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("swift build error = %v, output = %q", err, output)
	}
	command = exec.Command(
		"/usr/bin/swift",
		"build",
		"--package-path",
		packageRoot,
		"-c",
		"release",
		"--arch",
		"arm64",
		"--show-bin-path",
	)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("swift bin path error = %v", err)
	}
	return filepath.Join(
		strings.TrimSpace(string(output)),
		"LoomLocalAppContractProbe",
	)
}

func buildSwiftSetupContractProbe(t *testing.T) string {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	sourceRoot := filepath.Join(
		repoRoot,
		"apps",
		"macos",
		"Sources",
		"LoomLocalAppCore",
	)
	buildRoot := t.TempDir()
	mainPath := filepath.Join(buildRoot, "SetupContractProbe.swift")
	source := `import Darwin
import Foundation

@main
struct SetupContractProbe {
    static func main() async {
        do {
            let arguments = CommandLine.arguments
            if arguments.count == 4,
                arguments[1] == "--socket",
                arguments[3] == "--deepseek-configure"
            {
                let client = try LocalIPCClient(socketPath: arguments[2])
                let secret = "sk-" + ["deepseek", "contract"].joined(separator: "-") + "\n"
                let configured = try await client.configureCredential(
                    providerID: "deepseek",
                    secret: secret
                )
                let result: [String: Any] = [
                    "provider_id": configured.providerID,
                    "revision": configured.revision,
                    "status": configured.status,
                ]
                let encoded = try JSONSerialization.data(
                    withJSONObject: result,
                    options: [.sortedKeys]
                )
                FileHandle.standardOutput.write(encoded)
                return
            }
            if arguments.count == 4,
                arguments[1] == "--socket",
                arguments[3] == "--credential"
            {
                let client = try LocalIPCClient(socketPath: arguments[2])
                let setup = try await client.setupSnapshot()
                let verified = try await client.verifyMiniMax(
                    reference: setup.miniMax.credentialReference,
                    revision: setup.miniMax.revision
                )
                let result: [String: Any] = [
                    "revision": verified.revision,
                    "status": verified.status,
					"reason": verified.reason,
                ]
                let encoded = try JSONSerialization.data(
                    withJSONObject: result,
                    options: [.sortedKeys]
                )
                FileHandle.standardOutput.write(encoded)
                return
            }
            guard arguments.count == 3, arguments[1] == "--socket" else {
                throw LocalProductClientError.invalidRequest
            }
            let client = try LocalIPCClient(socketPath: arguments[2])
            let setup = try await client.setupSnapshot()
            let connection = try await client.connectCodex()
            let candidate = try await client.startBuilder()
            let firstVerification = try await client.verifyMiniMax(
                reference: "credential-ref-1",
                revision: 1
            )
            _ = try await client.verifyMiniMax(
                reference: "credential-ref-1",
                revision: 1
            )
            let result: [String: Any] = [
                "schema_version": setup.schemaVersion,
                "codex_auth_mode": setup.codex.authMode,
                "minimax_auth_mode": setup.miniMax.authMode,
                "runtime_count": setup.runtimes.count,
                "connect_status": connection.status,
                "draft_id": candidate.draftID,
                "question_id": candidate.question.id,
                "can_confirm": candidate.canConfirm,
                "verify_status": firstVerification.status,
            ]
            let encoded = try JSONSerialization.data(
                withJSONObject: result,
                options: [.sortedKeys]
            )
            FileHandle.standardOutput.write(encoded)
        } catch {
            FileHandle.standardError.write(Data("error:setup_probe\n".utf8))
            exit(1)
        }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(source), 0o600); err != nil {
		t.Fatalf("write Swift setup probe: %v", err)
	}
	probe := filepath.Join(buildRoot, "swift-setup-contract-probe")
	command := exec.Command(
		"/usr/bin/swiftc",
		"-O",
		"-parse-as-library",
		filepath.Join(sourceRoot, "LocalProductModels.swift"),
		filepath.Join(sourceRoot, "LocalProductHandoffModels.swift"),
		filepath.Join(sourceRoot, "MissionOrchestration.swift"),
		filepath.Join(sourceRoot, "LocalProductDecisionModels.swift"),
		filepath.Join(sourceRoot, "LocalProductSetupModels.swift"),
		filepath.Join(sourceRoot, "LocalProductExecutionModels.swift"),
		filepath.Join(sourceRoot, "LocalProductAssetModels.swift"),
		filepath.Join(sourceRoot, "LocalPermissionModels.swift"),
		filepath.Join(sourceRoot, "LocalExecutionModels.swift"),
		filepath.Join(sourceRoot, "LocalProductionModels.swift"),
		filepath.Join(sourceRoot, "LocalProductAgentInputModels.swift"),
		filepath.Join(sourceRoot, "LocalProductAgentRecoveryModels.swift"),
		filepath.Join(sourceRoot, "LocalProductToolRecoveryModels.swift"),
		filepath.Join(sourceRoot, "LocalOperationalDiagnostics.swift"),
		filepath.Join(sourceRoot, "LocalProductStore.swift"),
		filepath.Join(sourceRoot, "LocalIPCClient.swift"),
		mainPath,
		"-framework",
		"SwiftUI",
		"-o",
		probe,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("swiftc setup probe error = %v, output = %q", err, output)
	}
	return probe
}

func swiftPrivateSocketRoot(t *testing.T) (string, string) {
	t.Helper()
	root, err := os.MkdirTemp("/private/tmp", "loom-swift-ipc.")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		os.RemoveAll(root)
		t.Fatalf("chmod private root: %v", err)
	}
	return root, filepath.Join(root, "loomd.sock")
}

func assertJSONSemanticEqual(t *testing.T, actual, expected []byte) {
	t.Helper()
	var actualValue any
	var expectedValue any
	if err := json.Unmarshal(actual, &actualValue); err != nil {
		t.Fatalf("actual JSON invalid: %v", err)
	}
	if err := json.Unmarshal(expected, &expectedValue); err != nil {
		t.Fatalf("expected JSON invalid: %v", err)
	}
	actualBytes, _ := json.Marshal(actualValue)
	expectedBytes, _ := json.Marshal(expectedValue)
	if string(actualBytes) != string(expectedBytes) {
		t.Fatalf("JSON mismatch\nactual: %s\nexpected: %s", actualBytes, expectedBytes)
	}
}
