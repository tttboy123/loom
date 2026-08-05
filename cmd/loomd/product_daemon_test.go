package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/rules"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/teams"
	loomtui "loom-pi-rebuild/internal/tui"
	"loom-pi-rebuild/internal/verification"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"

	_ "modernc.org/sqlite"
)

type blockingObserverRunner struct {
	closed bool
}

func TestP3ATemplateArtifactResolverAcceptsExactContractPlusBoundSource(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "loom-p3a-diagnostic-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	artifactStore, err := evidence.NewStore(filepath.Join(root, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	defer artifactStore.Close()
	sourcePath := filepath.Join(root, "template.md")
	if err := os.WriteFile(sourcePath, []byte("# Team template\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sourcePath, err = filepath.EvalSymlinks(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	parameterDigest := strings.Repeat("a", 64)
	permissionDigest := strings.Repeat("b", 64)
	scopeDigest := strings.Repeat("c", 64)
	artifact, digest, _, err := app.CanonicalEvolutionTemplateArtifact(
		assets.AssetKindTeamTemplate, "team-template-1", "revision-1", sourcePath,
		assets.TemplateOutputTeamDraft, parameterDigest, permissionDigest, scopeDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := artifactStore.Publish(context.Background(), bytes.NewReader(artifact), digest); err != nil {
		t.Fatal(err)
	}
	contract, err := (productTemplateArtifactResolver{artifacts: artifactStore}).ResolveTemplateArtifact(
		context.Background(), assets.TemplateArtifactRequest{
			AssetKind: assets.AssetKindTeamTemplate, DefinitionID: "team-template-1",
			RevisionID: "revision-1", ArtifactDigest: digest,
		},
	)
	if err != nil || contract.TemplateOutput != assets.TemplateOutputTeamDraft ||
		contract.ParameterSchemaDigest != parameterDigest ||
		contract.PermissionCeilingDigest != permissionDigest ||
		contract.ScopeCeilingDigest != scopeDigest {
		t.Fatalf("template contract = %#v, %v", contract, err)
	}
}

func TestP3AProductionDaemonAssetJourneyUsesRealSocketAndAuthoritativeProjection(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "loom-p3a-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, "state.db")
	stateFile, err := os.OpenFile(statePath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateFile.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := openProductReadDatabase(statePath)
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
	store := journal.NewStore(database)
	authority, err := assets.NewAuthority(assets.AuthorityConfig{
		Store: store,
		Now: func() time.Time {
			return time.Date(2026, 8, 3, 16, 0, 0, 0, time.UTC)
		},
		ViewVersion: func() string { return readModel.GlobalReadView().Version() },
	})
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
	assetAPI, err := api.NewLocalProductAssetAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(root, "loomd.sock")
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(), BuildID: "p3a-fixture",
		Handler: localipc.HandlerFunc(localProductHandlerWithComposition(nil, nil, nil, nil, nil, nil, assetAPI, nil, nil, nil, nil, nil, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("P3A IPC server did not become ready")
	}
	client, err := localipc.NewClient(localipc.ClientConfig{SocketPath: socketPath, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	var before api.EvolutionAssetSnapshot
	if err := client.CallJourney(context.Background(), journeyID, "evolution_asset_snapshot", api.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64}, &before); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "socket-skill.md")
	if err := os.WriteFile(sourcePath, []byte("# Socket Skill\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, sourceArtifactDigest, sourceContentDigest, err := app.CanonicalEvolutionAssetArtifact(assets.AssetKindSkill, "skill-socket", "revision-1", sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	command := struct {
		DefinitionID                  string   `json:"definition_id"`
		RevisionID                    string   `json:"revision_id"`
		Name                          string   `json:"name"`
		Description                   string   `json:"description"`
		SubjectScope                  string   `json:"subject_scope"`
		SourcePath                    string   `json:"source_path"`
		SuppliedArtifactDigest        string   `json:"supplied_artifact_digest"`
		SuppliedContentDigest         string   `json:"supplied_content_digest"`
		Dependencies                  []string `json:"dependencies"`
		CompatibleRuntimeCapabilities []string `json:"compatible_runtime_capabilities"`
		Risk                          string   `json:"risk"`
	}{"skill-socket", "revision-1", "Socket Skill", "fixture", "project", sourcePath, sourceArtifactDigest, sourceContentDigest, []string{}, []string{}, "low"}
	var receipt api.EvolutionAssetCommandResult
	input, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CallJourney(context.Background(), journeyID, "evolution_asset_command", api.EvolutionAssetCommandRequest{
		JourneyID: journeyID, OperationID: "p3a-socket-create", Action: "create_skill",
		ExpectedViewVersion: before.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{},
		Input: input,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.OperationID != "p3a-socket-create" || receipt.Action != "create_skill" || len(receipt.EventIDs) != 3 {
		t.Fatalf("asset receipt = %#v", receipt)
	}
	var after api.EvolutionAssetSnapshot
	if err := client.CallJourney(context.Background(), journeyID, "evolution_asset_snapshot", api.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64}, &after); err != nil {
		t.Fatal(err)
	}
	if len(after.Definitions) != 1 || len(after.Candidates) != 1 || after.Definitions[0].DefinitionID != "skill-socket" {
		t.Fatalf("authoritative asset snapshot = %#v", after)
	}
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("server stop error = %v", err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestP3AProductMaterializerStartupRemovesVerifiedPreCASOrphan(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "loom-p3a-recover-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", filepath.Join(root, "state.db"))
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
	authority, err := assets.NewAuthority(assets.AuthorityConfig{
		Store: journal.NewStore(database), Now: func() time.Time {
			return time.Date(2026, 8, 3, 16, 0, 0, 0, time.UTC)
		},
		ViewVersion: func() string { return readModel.GlobalReadView().Version() },
	})
	if err != nil {
		t.Fatal(err)
	}
	artifactStore, err := evidence.NewStore(filepath.Join(root, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	defer artifactStore.Close()
	skillMaterializer, err := piadapter.NewSkillMaterializer(piadapter.MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("# exact orphan\n")
	contentSum := sha256.Sum256(content)
	digest := hex.EncodeToString(contentSum[:])
	exact := assets.ExactAssetRevisionBinding{
		AssetKind: assets.AssetKindSkill, DefinitionID: "skill-orphan",
		RevisionID: "revision-1", SHA256Digest: digest,
		SourceScope: assets.SourceScopeLocal,
	}
	setDigest, err := assets.CanonicalAssetRevisionSetDigest([]assets.ExactAssetRevisionBinding{exact})
	if err != nil {
		t.Fatal(err)
	}
	isolatedRoot := filepath.Join(root, "materialization")
	plan := piadapter.MaterializationPlan{
		IsolatedRoot: isolatedRoot, RunID: "run-orphan", AttemptNumber: 1,
		Generation: 1, RuntimeInstanceID: "pi-runtime-1",
		RuntimeIdentityDigest:  digest,
		RuntimeCapabilities:    []string{piadapter.SkillMaterializationCapability},
		AssetRevisionSetDigest: setDigest,
		JourneyID:              "123e4567-e89b-42d3-a456-426614174000",
		OperationID:            "materialize:run-orphan:1:1",
		Bindings: []piadapter.MaterializationBinding{{
			AssetKind: string(assets.AssetKindSkill), DefinitionID: "skill-orphan",
			RevisionID: "revision-1", Digest: digest, ContentDigest: digest,
			SourceScope: string(assets.SourceScopeLocal), Files: []piadapter.MaterializationFile{{
				RelativePath: "SKILL.md", Bytes: content, Digest: digest,
			}},
		}},
	}
	result, err := skillMaterializer.Materialize(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	productMaterializer := &productTeamAssetMaterializer{
		authority: authority, projection: readModel, artifacts: artifactStore,
		materializer: skillMaterializer, isolatedRoot: isolatedRoot,
		plans: make(map[string]piadapter.MaterializationPlan),
	}
	if err := productMaterializer.RecoverStartup(context.Background()); err != nil {
		t.Fatalf("RecoverStartup() error = %v", err)
	}
	if _, err := os.Lstat(result.Root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("verified pre-CAS orphan remains: %v", err)
	}
	if len(productMaterializer.plans) != 0 {
		t.Fatalf("uncommitted plan retained: %#v", productMaterializer.plans)
	}
}

type productHandoffStub struct{ proposed bool }

func (stub *productHandoffStub) ProposeSideTask(_ context.Context, request app.SideTaskProposalRequest) (app.SideTaskProposalResult, error) {
	stub.proposed = true
	return app.SideTaskProposalResult{SchemaVersion: 1, Status: "proposal", ProposalDigest: strings.Repeat("a", 64), ViewVersion: request.ExpectedViewVersion, Purpose: request.Purpose, Mode: request.Mode, Title: request.Title, PermissionScopes: []string{}, RequiresConfirmation: true}, nil
}
func (*productHandoffStub) CreateSideTask(context.Context, app.SideTaskCreateRequest) (app.SideTaskCreateResult, error) {
	return app.SideTaskCreateResult{}, nil
}
func (*productHandoffStub) ReadSideTask(context.Context, app.SideTaskReadRequest) (app.SideTaskReadResult, error) {
	return app.SideTaskReadResult{}, nil
}
func (*productHandoffStub) DecideSideTask(context.Context, app.SideTaskDecisionRequest) (app.SideTaskDecisionResult, error) {
	return app.SideTaskDecisionResult{}, nil
}

func TestProductHandlerDispatchesStrictSideTaskOperation(t *testing.T) {
	stub := &productHandoffStub{}
	handoff, err := api.NewLocalProductHandoffAPI(stub)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandlerWithComposition(nil, nil, nil, nil, handoff, nil, nil, nil, nil, nil, nil, nil, nil)
	params, err := json.Marshal(app.SideTaskProposalRequest{SchemaVersion: 1, Operation: "propose", ParentMissionID: "mission/team-1", ParentTeamInstanceID: "team-1", ParentTaskID: "work-1", ParentRunID: "run-1", ParentClaimGeneration: 1, ParentExecutionDigest: strings.Repeat("a", 64), Purpose: "research", Mode: "report_only", Title: "Research", AuthorizedRequest: "Find facts", PermissionScopes: []string{}, ExpectedViewVersion: strings.Repeat("b", 64), CorrelationID: "11111111-1111-4111-8111-111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	response := handler(context.Background(), localipc.Request{Method: "side_task_handoff", Params: params})
	if !response.OK || !stub.proposed {
		t.Fatalf("response=%#v proposed=%v", response, stub.proposed)
	}
	bad := handler(context.Background(), localipc.Request{Method: "side_task_handoff", Params: json.RawMessage(`{"operation":"unknown"}`)})
	if bad.OK || bad.Error == nil || bad.Error.Code != "invalid_request" {
		t.Fatalf("bad=%#v", bad)
	}
	unavailable := localProductHandlerWithComposition(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)(
		context.Background(), localipc.Request{Method: "side_task_handoff", Params: params},
	)
	if unavailable.OK || unavailable.Error == nil || unavailable.Error.Code != "internal" {
		t.Fatalf("unavailable=%#v", unavailable)
	}
}

func TestSideTaskHandlerErrorsStayInsideClosedCodeSet(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{app.ErrInvalidSideTaskProduct, "invalid_request"},
		{app.ErrSideTaskProductCapabilityGap, "capability_gap"},
		{work.ErrSideTaskNotFound, "not_found"},
		{app.ErrSideTaskProductConflict, "conflict"},
		{app.ErrSideTaskProductStaleView, "stale_view"},
		{app.ErrSideTaskProductStaleGeneration, "stale_generation"},
		{app.ErrSideTaskProductDigestMismatch, "digest_mismatch"},
		{app.ErrSideTaskProductHumanRequired, "human_required"},
		{context.DeadlineExceeded, "internal"},
		{app.ErrMissionExecutionBusy, "internal"},
		{errors.New("unexpected unavailable failure"), "internal"},
	}
	closed := map[string]bool{
		"invalid_request": true, "capability_gap": true, "not_found": true,
		"conflict": true, "stale_view": true, "stale_generation": true,
		"digest_mismatch": true, "human_required": true, "internal": true,
	}
	for _, test := range tests {
		response := sideTaskServiceError(test.err)
		if response.OK || response.Error == nil || response.Error.Code != test.want ||
			!closed[response.Error.Code] {
			t.Fatalf("err=%v response=%#v want=%q", test.err, response, test.want)
		}
	}
}

func (runner *blockingObserverRunner) Run(
	ctx context.Context,
) (app.LocalRuntimeObservationDaemonResult, error) {
	<-ctx.Done()
	return app.LocalRuntimeObservationDaemonResult{}, ctx.Err()
}

func (runner *blockingObserverRunner) Close() error {
	runner.closed = true
	return nil
}

type failingObserverRunner struct {
	err    error
	closed bool
}

type productDaemonRuntimeProbe struct {
	id       string
	instance loomruntime.RuntimeInstance
	models   []string
}

type productExecutionServiceStub struct {
	preflight app.MissionExecutionPreflight
}

func (stub *productExecutionServiceStub) PreflightMission(
	context.Context,
	app.MissionExecutionCommand,
) (app.MissionExecutionPreflight, error) {
	return stub.preflight, nil
}

func (stub *productExecutionServiceStub) StartMission(
	context.Context,
	app.MissionExecutionCommand,
) (app.MissionExecutionResult, error) {
	return app.MissionExecutionResult{}, app.ErrInvalidMissionExecution
}

func (stub *productExecutionServiceStub) ControlMission(
	context.Context,
	app.MissionExecutionCommand,
) (app.MissionExecutionResult, error) {
	return app.MissionExecutionResult{}, app.ErrInvalidMissionExecution
}

func (probe productDaemonRuntimeProbe) ID() string {
	return probe.id
}

func (probe productDaemonRuntimeProbe) ObserveRuntime(
	ctx context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []loomruntime.RuntimeObservation{{
		Instance: probe.instance,
		ModelIDs: append([]string(nil), probe.models...),
	}}, nil
}

func (runner *failingObserverRunner) Run(
	context.Context,
) (app.LocalRuntimeObservationDaemonResult, error) {
	return app.LocalRuntimeObservationDaemonResult{}, runner.err
}

func (runner *failingObserverRunner) Close() error {
	runner.closed = true
	return nil
}

func TestProductDaemonClassifiesLifecycleFailureBoundaries(t *testing.T) {
	t.Run("server_before_ready", func(t *testing.T) {
		root, statePath := productDaemonFailureState(t)
		socketPath := filepath.Join(root, "loomd.sock")
		observer := &blockingObserverRunner{}
		runner, err := newProductDaemonRunner(observer, statePath, socketPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(root, 0); err != nil {
			t.Fatal(err)
		}
		_, runErr := runner.Run(context.Background())
		if err := os.Chmod(root, 0o700); err != nil {
			t.Fatal(err)
		}
		assertDaemonFailureCode(t, runErr, "local_ipc")
		if err := runner.Close(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("observer_after_ready", func(t *testing.T) {
		root, statePath := productDaemonFailureState(t)
		observer := &failingObserverRunner{
			err: errors.New("private observer failure"),
		}
		runner, err := newProductDaemonRunner(
			observer,
			statePath,
			filepath.Join(root, "loomd.sock"),
		)
		if err != nil {
			t.Fatal(err)
		}
		_, runErr := runner.Run(context.Background())
		assertDaemonFailureCode(t, runErr, "observer")
		assertDaemonFailureReason(t, runErr, "observer_unknown")
		if err := runner.Close(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("observer_binding_integrity_remains_fatal", func(t *testing.T) {
		root, statePath := productDaemonFailureState(t)
		observer := &failingObserverRunner{err: testPiMetadataFailure{
			command: loomruntime.PiMetadataVersion,
			cause:   piadapter.ErrPiMetadataBindingChanged,
		}}
		runner, err := newProductDaemonRunner(
			observer,
			statePath,
			filepath.Join(root, "loomd.sock"),
		)
		if err != nil {
			t.Fatal(err)
		}
		_, runErr := runner.Run(context.Background())
		assertDaemonFailureCode(t, runErr, "observer")
		assertDaemonFailureReason(t, runErr, "observer_metadata_binding")
		if err := runner.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestProductDaemonClassifiesConstructionIPCFailure(t *testing.T) {
	root, statePath := productDaemonFailureState(t)
	observer := &blockingObserverRunner{}
	runner, err := newProductDaemonRunner(
		observer,
		statePath,
		filepath.Join(root, "missing", "loomd.sock"),
		productSetupRuntimeConfig{
			CredentialStore: &productCredentialTestStore{},
		},
	)
	if runner != nil {
		t.Fatal("runner constructed for invalid IPC parent")
	}
	if got := daemonBuildFailureReason(err); got != "build_ipc" {
		t.Fatalf("build reason = %q, want build_ipc; error=%v", got, err)
	}
	if !observer.closed {
		t.Fatal("observer was not closed after IPC construction failure")
	}
}

func TestProductDaemonPreservesSetupConstructionFailureReason(t *testing.T) {
	root, statePath := productDaemonFailureState(t)
	observer := &blockingObserverRunner{}
	runner, err := newProductDaemonRunner(
		observer,
		statePath,
		filepath.Join(root, "loomd.sock"),
		productSetupRuntimeConfig{
			CredentialStore: &productCredentialTestStore{},
			CodexExecutable: "relative-codex",
		},
	)
	if runner != nil {
		t.Fatal("runner constructed for invalid Codex executable")
	}
	if got := daemonBuildFailureReason(err); got != "build_setup_native_auth" {
		t.Fatalf(
			"build reason = %q, want build_setup_native_auth; error=%v",
			got,
			err,
		)
	}
	if !observer.closed {
		t.Fatal("observer was not closed after setup construction failure")
	}
}

func TestProductDaemonContainsExactPiMetadataTimeoutWithoutStoppingIPC(
	t *testing.T,
) {
	root, statePath := productDaemonFailureState(t)
	socketPath := filepath.Join(root, "loomd.sock")
	runtimePath := filepath.Join(root, "runtime")
	isolationPath := filepath.Join(root, "isolation")
	for _, directory := range []string{runtimePath, isolationPath} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	piPath := filepath.Join(runtimePath, "pi")
	piScript := `#!/bin/sh
set -eu
counter="${0%/*}/list-models-count"
if [ "$#" -eq 1 ] && [ "$1" = "--version" ]; then
  printf '0.82.1\n'
  exit 0
fi
if [ "$#" -eq 8 ] && [ "$8" = "--list-models" ]; then
  count=0
  if [ -f "$counter" ]; then count=$(/bin/cat "$counter"); fi
  count=$((count + 1))
  printf '%s' "$count" > "$counter"
  /bin/sleep 30
  exit 0
fi
exit 83
`
	if err := os.WriteFile(piPath, []byte(piScript), 0o700); err != nil {
		t.Fatal(err)
	}
	codexPath := filepath.Join(root, "codex")
	if err := os.WriteFile(
		codexPath,
		[]byte("#!/bin/sh\nprintf 'Logged in using ChatGPT\\n'\n"),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	runner, err := productionDaemonBuilder(daemonBuildConfig{
		Observer: app.LocalRuntimeObservationDaemonConfig{
			StatePath:           statePath,
			IsolationRoot:       isolationPath,
			RuntimeSearchPaths:  []string{runtimePath},
			ProbeID:             "pi-timeout-probe",
			RuntimeInstanceID:   "pi-timeout-runtime",
			DeviceID:            "device-timeout",
			DisplayName:         "Pi timeout fixture",
			ObservationInterval: 10 * time.Millisecond,
			// The model fixture sleeps for thirty seconds, so ten seconds still
			// proves the product timeout while avoiding a false version-probe
			// timeout when the full repository matrix is CPU constrained.
			ProcessTimeout: 10 * time.Second,
			MaxCycles:      1,
		},
		SocketPath:      socketPath,
		CodexExecutable: codexPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		done <- runErr
	}()
	waitForProductSocket(t, socketPath)
	counterPath := filepath.Join(runtimePath, "list-models-count")
	// The repository matrix concurrently runs Swift probes and race-sensitive
	// process fixtures. Keep the product assertion bounded while allowing the
	// observer goroutine to receive CPU after the socket becomes ready. The
	// production metadata process remains bounded by the injected two-second
	// ProcessTimeout above; this deadline only bounds test synchronization.
	deadline := time.Now().Add(90 * time.Second)
	for {
		select {
		case runErr := <-done:
			reason := ""
			if classified, ok := runErr.(daemonFailureReasoner); ok {
				reason = classified.DaemonFailureReason()
			}
			t.Fatalf(
				"real Pi observer exited before list-models: %v reason=%s",
				runErr,
				reason,
			)
		default:
		}
		contents, readErr := os.ReadFile(counterPath)
		if readErr == nil && string(contents) == "1" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("real Pi list-models was not invoked once: %q %v", contents, readErr)
		}
		time.Sleep(time.Millisecond)
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Ping(context.Background()); err != nil {
		t.Fatalf("ping after real Pi timeout: %v", err)
	}
	var snapshot api.LocalProductSnapshot
	healthDeadline := time.Now().Add(15 * time.Second)
	for {
		select {
		case runErr := <-done:
			reason := ""
			if classified, ok := runErr.(daemonFailureReasoner); ok {
				reason = classified.DaemonFailureReason()
			}
			t.Fatalf("metadata timeout stopped product IPC: %v reason=%s", runErr, reason)
		default:
		}
		if err := client.Call(
			context.Background(),
			"snapshot",
			api.LocalProductSnapshotRequest{Limit: 64},
			&snapshot,
		); err != nil {
			t.Fatal(err)
		}
		if snapshot.Partial && snapshot.Reason == "observer_models_timeout" {
			break
		}
		if time.Now().After(healthDeadline) {
			t.Fatalf("metadata timeout health was not projected: %#v", snapshot)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !snapshot.Partial || snapshot.Reason != "observer_models_timeout" ||
		snapshot.Health != (api.LocalProductHealth{
			Daemon: "serving_request", Journal: "available", Projection: "current",
		}) {
		t.Fatalf("snapshot health = %#v", snapshot.Health)
	}
	setupClient, err := loomtui.NewDaemonReadClient(client)
	if err != nil {
		t.Fatal(err)
	}
	setup, err := setupClient.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatalf("setup/Codex read after real Pi timeout: %v", err)
	}
	if setup.Codex.Status != "available" || setup.Codex.AuthMode != "native_auth" ||
		setup.MiniMax.Status != "unconfigured" || setup.SavedTeams == nil {
		t.Fatalf("setup after real Pi timeout = %#v", setup)
	}
	time.Sleep(100 * time.Millisecond)
	contents, err := os.ReadFile(counterPath)
	if err != nil || string(contents) != "1" {
		t.Fatalf("Pi metadata hidden retry count = %q error=%v", contents, err)
	}
	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("controlled stop error = %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("contained product daemon did not stop")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProductDaemonCopiedRuntimeStateObservesPi0821WithoutRewrite(
	t *testing.T,
) {
	const retainedStateSHA256 = "677624b624b68ddd908939766752177a802bd90b657075133ef845e67e5461a2"
	root, statePath := productDaemonFailureState(t)
	socketPath := filepath.Join(root, "loomd.sock")
	runtimePath := filepath.Join(root, "runtime")
	isolationPath := filepath.Join(root, "isolation")
	for _, directory := range []string{runtimePath, isolationPath} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	contents := retainedPiSixFactStateFixture(t)
	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != retainedStateSHA256 {
		t.Fatalf("retained state digest=%x", digest)
	}
	if err := os.WriteFile(statePath, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	beforeEvents, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := map[string]int{
		"AgentGrantIdentityIndexInitialized": 1,
		"WorkRunIdentityIndexInitialized":    1,
		"RuntimeInstanceDiscovered":          1,
		"TeamDefinitionSaved":                1,
		"TeamInstanceCreated":                1,
		"AgentInstanceCreated":               1,
	}
	gotTypes := make(map[string]int, len(wantTypes))
	for _, event := range beforeEvents {
		gotTypes[event.Type]++
	}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("retained event types=%v", gotTypes)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	piPath := filepath.Join(runtimePath, "pi")
	piScript := `#!/bin/sh
set -eu
counter="${0%/*}/metadata-count"
count=0
if [ -f "$counter" ]; then count=$(/bin/cat "$counter"); fi
count=$((count + 1))
printf '%s' "$count" > "$counter"
if [ "$#" -eq 1 ] && [ "$1" = "--version" ]; then
  printf '0.82.1\n'
  exit 0
fi
if [ "$#" -eq 8 ] && [ "$8" = "--list-models" ]; then
  printf 'provider model context max-out thinking images\nloom-local qwen2.5-coder-1.5b-instruct-q4-k-m 32K 256 no no\n'
  exit 0
fi
exit 83
`
	if err := os.WriteFile(piPath, []byte(piScript), 0o700); err != nil {
		t.Fatal(err)
	}
	packageRoot := filepath.Join(root, "npm", "lib", "node_modules", "@openai", "codex")
	wrapperPath := filepath.Join(packageRoot, "bin", "codex.js")
	nativeCodexPath := filepath.Join(packageRoot, "node_modules", "@openai", "codex-darwin-arm64", "vendor", "aarch64-apple-darwin", "bin", "codex")
	for _, directory := range []string{filepath.Dir(wrapperPath), filepath.Dir(nativeCodexPath)} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(wrapperPath, []byte("#!/usr/bin/env node\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nativeCodexPath, []byte("#!/bin/sh\nprintf 'Logged in using ChatGPT\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	codexPath := filepath.Join(root, "npm", "bin", "codex")
	if err := os.MkdirAll(filepath.Dir(codexPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(wrapperPath, codexPath); err != nil {
		t.Fatal(err)
	}
	var localModelCatalog *piadapter.PiLocalModelCatalogConfig
	lockedModelRoot := os.Getenv("LOOM_P2A_W3_LOCKED_MODEL_ROOT")
	if lockedModelRoot != "" {
		if !filepath.IsAbs(lockedModelRoot) || filepath.Clean(lockedModelRoot) != lockedModelRoot {
			t.Fatalf("invalid locked model root %q", lockedModelRoot)
		}
		modelExecutable := filepath.Join(lockedModelRoot, "runtime", "llama-b10107", "llama-server")
		modelPath := filepath.Join(lockedModelRoot, "models", "qwen2.5-coder-1.5b-instruct-q4_k_m.gguf")
		assertLockedProductComponent(t, modelExecutable, 0o700, 33472,
			"a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b")
		assertLockedProductComponent(t, modelPath, 0o600, 1117320768,
			"cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046")
		localModelCatalog = &piadapter.PiLocalModelCatalogConfig{
			PrivateRoot: lockedModelRoot, ExecutablePath: modelExecutable, ModelPath: modelPath,
		}
	}
	runner, err := productionDaemonBuilder(daemonBuildConfig{
		Observer: app.LocalRuntimeObservationDaemonConfig{
			StatePath: statePath, IsolationRoot: isolationPath,
			RuntimeSearchPaths: []string{runtimePath},
			ProbeID:            "p2a-w3-pi-probe", RuntimeInstanceID: "pi-0.82.1-p2a-w3-pi",
			DeviceID:            "local-mac-p2a-w3",
			DisplayName:         "Pi 0.82.1 P2A-W3 Execution Canary",
			ObservationInterval: time.Hour, ProcessTimeout: 10 * time.Second,
			MaxCycles:         0,
			LocalModelCatalog: localModelCatalog,
		},
		SocketPath: socketPath, CodexExecutable: codexPath,
	})
	if err != nil {
		t.Fatalf("build reason=%s leaf=%v", daemonBuildFailureReason(err), errors.Unwrap(err))
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		done <- runErr
	}()
	waitForProductSocket(t, socketPath)
	counterPath := filepath.Join(runtimePath, "metadata-count")
	// This bounds only the test's wait for the observer goroutine to be
	// scheduled. The product metadata calls retain their injected two-second
	// ProcessTimeout and exact assertions below.
	metadataDeadline := 30 * time.Second
	if lockedModelRoot != "" {
		metadataDeadline = 45 * time.Second
	}
	deadline := time.Now().Add(metadataDeadline)
	for {
		contents, readErr := os.ReadFile(counterPath)
		if readErr == nil && string(contents) == "2" {
			break
		}
		select {
		case runErr := <-done:
			t.Fatalf("copied-state observer exited: %v", runErr)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("metadata calls=%q error=%v", contents, readErr)
		}
		time.Sleep(time.Millisecond)
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath, Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	var snapshot api.LocalProductSnapshot
	if err := client.Call(
		context.Background(), "snapshot",
		api.LocalProductSnapshotRequest{Limit: 64}, &snapshot,
	); err != nil {
		t.Fatal(err)
	}
	if snapshot.Partial || len(snapshot.Runtimes) != 1 ||
		snapshot.Runtimes[0].RuntimeInstanceID != "pi-0.82.1-p2a-w3-pi" ||
		snapshot.Runtimes[0].ExecutableVersion != "0.82.1" ||
		!reflect.DeepEqual(snapshot.Runtimes[0].ModelIDs, []string{
			"loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m",
		}) || len(snapshot.Teams) != 1 {
		t.Fatalf("copied-state snapshot=%#v", snapshot)
	}
	if lockedModelRoot != "" {
		if _, err := os.Stat(filepath.Join(filepath.Dir(statePath), "execution")); err != nil {
			t.Fatalf("execution composition missing: %v", err)
		}
		workPackage, err := work.CodingWorkPackage()
		if err != nil {
			t.Fatal(err)
		}
		var envelope api.MissionExecutionEnvelope
		teamID := "team-instance-8f2f4f51416eac8a915927fde5da6420"
		if err := client.Call(context.Background(), "mission_execution", app.MissionExecutionCommand{
			SchemaVersion: app.MissionExecutionSchemaVersion,
			Operation:     "preflight", MissionID: "mission/" + teamID,
			TeamInstanceID: teamID,
			WorkPackageID:  workPackage.ID(), WorkPackageDigest: workPackage.Digest(),
			Objective:           "Prove zero-write product construction",
			ExpectedViewVersion: snapshot.ViewVersion,
			CorrelationID:       "77777777-7777-4777-8777-777777777777",
		}, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Operation != "preflight" || envelope.Preflight == nil || envelope.Preflight.PreflightDigest == "" {
			t.Fatalf("preflight envelope=%#v", envelope)
		}
	}
	verifyDB, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	afterEvents, err := journal.NewStore(verifyDB).ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_ = verifyDB.Close()
	if !reflect.DeepEqual(afterEvents, beforeEvents) {
		t.Fatalf("copied authority changed: before=%#v after=%#v", beforeEvents, afterEvents)
	}
	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("controlled stop error=%v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("copied-state daemon did not stop")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err = os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	digest = sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != retainedStateSHA256 {
		t.Fatalf("post-observation state digest=%x", digest)
	}
	assertRetainedObserverFailureMatrixDoesNotWrite(t, statePath, beforeEvents)
}

func assertLockedProductComponent(t *testing.T, path string, mode os.FileMode, size int64, digest string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != mode || info.Size() != size {
		t.Fatalf("locked component identity %q: info=%v err=%v", path, info, err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		t.Fatal(copyErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != digest {
		t.Fatalf("locked component digest %q=%s want %s", path, got, digest)
	}
}

func retainedPiSixFactStateFixture(t *testing.T) []byte {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve retained Pi fixture source")
	}
	fixturePath := filepath.Join(
		filepath.Dir(currentFile),
		"..", "..", ".loom-evidence", "phase2a", "P2A-W3",
		"retained-pi-six-fact-state.sqlite.gz.b64",
	)
	encoded, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := base64.StdEncoding.DecodeString(
		strings.TrimSpace(string(encoded)),
	)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	contents, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return contents
}

func assertRetainedObserverFailureMatrixDoesNotWrite(
	t *testing.T,
	statePath string,
	wantEvents []journal.Event,
) {
	t.Helper()
	private := errors.New("private observer detail")
	cases := []struct {
		name   string
		err    error
		reason string
	}{
		{"probe_factory", discoveryscan.ErrRuntimeProbeFactoryFailed, "observer_probe_factory"},
		{"probe_candidate", piadapter.ErrPiLocalRuntimeCandidateInvalid, "observer_probe_candidate"},
		{"probe_construction", piadapter.ErrPiLocalRuntimeProbeConstructionFailed, "observer_probe_construction"},
		{"binding", testPiMetadataFailure{loomruntime.PiMetadataVersion, piadapter.ErrPiMetadataBindingChanged}, "observer_metadata_binding"},
		{"version_process", testPiMetadataFailure{loomruntime.PiMetadataVersion, piadapter.ErrPiMetadataProcessFailed}, "observer_version_process"},
		{"version_timeout", testPiMetadataFailure{loomruntime.PiMetadataVersion, piadapter.ErrPiMetadataProcessTimeout}, "observer_version_timeout"},
		{"version_limit", testPiMetadataFailure{loomruntime.PiMetadataVersion, piadapter.ErrPiMetadataProcessOutputTooLarge}, "observer_version_output_limit"},
		{"version_stderr", testPiMetadataFailure{loomruntime.PiMetadataVersion, loomruntime.ErrPiMetadataStderr}, "observer_version_stderr"},
		{"version_output", testPiMetadataFailure{loomruntime.PiMetadataVersion, loomruntime.ErrInvalidPiMetadataOutput}, "observer_version_output"},
		{"models_process", testPiMetadataFailure{loomruntime.PiMetadataListModels, piadapter.ErrPiMetadataProcessFailed}, "observer_models_process"},
		{"models_timeout", testPiMetadataFailure{loomruntime.PiMetadataListModels, piadapter.ErrPiMetadataProcessTimeout}, "observer_models_timeout"},
		{"models_limit", testPiMetadataFailure{loomruntime.PiMetadataListModels, piadapter.ErrPiMetadataProcessOutputTooLarge}, "observer_models_output_limit"},
		{"models_stderr", testPiMetadataFailure{loomruntime.PiMetadataListModels, loomruntime.ErrPiMetadataStderr}, "observer_models_stderr"},
		{"models_output", testPiMetadataFailure{loomruntime.PiMetadataListModels, loomruntime.ErrInvalidPiMetadataOutput}, "observer_models_output"},
		{"models_duplicate", testPiMetadataFailure{loomruntime.PiMetadataListModels, loomruntime.ErrDuplicatePiRuntimeModel}, "observer_models_duplicate"},
		{"inventory", loomruntime.ErrInvalidRuntimeInstance, "observer_inventory"},
		{"projection", app.ErrRuntimeObservationProjectionRefresh, "observer_projection"},
		{"plan", app.ErrInvalidRuntimeObservationWritePlan, "observer_plan"},
		{"identity", app.ErrLocalRuntimeObservationDaemonMetadata, "observer_identity_metadata"},
		{"write", journal.ErrPartialEventBatchConflict, "observer_write"},
		{"known_plus_unknown", errors.Join(journal.ErrPartialEventBatchConflict, private), "observer_unknown"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			failure := classifyProductDaemonFailure("observer", test.err)
			assertDaemonFailureReason(t, failure, test.reason)
			if daemonFailureMessage(failure) != "daemon failed: "+test.reason {
				t.Fatalf("public failure=%q", daemonFailureMessage(failure))
			}
			database, err := sql.Open("sqlite", statePath)
			if err != nil {
				t.Fatal(err)
			}
			gotEvents, readErr := journal.NewStore(database).ReadAll(context.Background())
			_ = database.Close()
			if readErr != nil || !reflect.DeepEqual(gotEvents, wantEvents) {
				t.Fatalf("failure attribution changed authority: events=%#v err=%v", gotEvents, readErr)
			}
		})
	}
}

func TestContainableProductObserverTimeoutRejectsMixedFailureChains(
	t *testing.T,
) {
	timeout := testPiMetadataFailure{
		command: loomruntime.PiMetadataListModels,
		cause:   piadapter.ErrPiMetadataProcessTimeout,
	}
	if !containableProductObserverTimeout(fmt.Errorf("observe: %w", timeout)) {
		t.Fatal("pure typed metadata timeout was not containable")
	}
	for _, mixed := range []error{
		errors.Join(timeout, piadapter.ErrPiMetadataBindingChanged),
		errors.Join(timeout, piadapter.ErrPiMetadataProcessFailed),
		errors.Join(timeout, errors.New("unknown cleanup failure")),
	} {
		if containableProductObserverTimeout(mixed) {
			t.Fatalf("mixed observer chain was containable: %v", mixed)
		}
	}
}

func TestProductDaemonReclaimsAbandonedOwnedLockAndCleansOwnedPair(
	t *testing.T,
) {
	root, statePath := productDaemonFailureState(t)
	socketPath := filepath.Join(root, "loomd.sock")
	lockPath := socketPath + ".lock"
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	observer := &blockingObserverRunner{}
	runner, err := newProductDaemonRunner(observer, statePath, socketPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		done <- runErr
	}()
	waitForProductSocket(t, socketPath)
	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("product daemon stop error = %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("product daemon did not stop")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{socketPath, lockPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("owned path remains after close %q: %v", path, err)
		}
	}
}

func TestObserverFailureReasonIsClosedTypedAndNonDisclosing(t *testing.T) {
	private := errors.New("private observer detail")
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "probe factory",
			err:  discoveryscan.ErrRuntimeProbeFactoryFailed,
			want: "observer_probe_factory",
		},
		{
			name: "probe factory plus unknown",
			err: fmt.Errorf(
				"%w: %w",
				discoveryscan.ErrRuntimeProbeFactoryFailed,
				private,
			),
			want: "observer_unknown",
		},
		{
			name: "probe candidate",
			err: fmt.Errorf(
				"%w: %w",
				discoveryscan.ErrRuntimeProbeFactoryFailed,
				piadapter.ErrPiLocalRuntimeCandidateInvalid,
			),
			want: "observer_probe_candidate",
		},
		{
			name: "probe construction",
			err: fmt.Errorf(
				"%w: %w",
				discoveryscan.ErrRuntimeProbeFactoryFailed,
				piadapter.ErrPiLocalRuntimeProbeConstructionFailed,
			),
			want: "observer_probe_construction",
		},
		{
			name: "metadata binding",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataVersion,
				cause:   piadapter.ErrPiMetadataBindingChanged,
			},
			want: "observer_metadata_binding",
		},
		{
			name: "version process",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataVersion,
				cause:   piadapter.ErrPiMetadataProcessFailed,
			},
			want: "observer_version_process",
		},
		{
			name: "version timeout",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataVersion,
				cause:   piadapter.ErrPiMetadataProcessTimeout,
			},
			want: "observer_version_timeout",
		},
		{
			name: "version output limit",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataVersion,
				cause:   piadapter.ErrPiMetadataProcessOutputTooLarge,
			},
			want: "observer_version_output_limit",
		},
		{
			name: "version stderr",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataVersion,
				cause:   loomruntime.ErrPiMetadataStderr,
			},
			want: "observer_version_stderr",
		},
		{
			name: "version output",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataVersion,
				cause:   loomruntime.ErrInvalidPiMetadataOutput,
			},
			want: "observer_version_output",
		},
		{
			name: "models process",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataListModels,
				cause:   piadapter.ErrPiMetadataProcessFailed,
			},
			want: "observer_models_process",
		},
		{
			name: "models timeout",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataListModels,
				cause:   piadapter.ErrPiMetadataProcessTimeout,
			},
			want: "observer_models_timeout",
		},
		{
			name: "models output limit",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataListModels,
				cause:   piadapter.ErrPiMetadataProcessOutputTooLarge,
			},
			want: "observer_models_output_limit",
		},
		{
			name: "models stderr",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataListModels,
				cause:   loomruntime.ErrPiMetadataStderr,
			},
			want: "observer_models_stderr",
		},
		{
			name: "models output",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataListModels,
				cause:   loomruntime.ErrInvalidPiMetadataOutput,
			},
			want: "observer_models_output",
		},
		{
			name: "models duplicate",
			err: testPiMetadataFailure{
				command: loomruntime.PiMetadataListModels,
				cause:   loomruntime.ErrDuplicatePiRuntimeModel,
			},
			want: "observer_models_duplicate",
		},
		{
			name: "projection",
			err:  app.ErrRuntimeObservationProjectionRefresh,
			want: "observer_projection",
		},
		{
			name: "projection plus unknown",
			err: fmt.Errorf(
				"%w: %w",
				app.ErrRuntimeObservationProjectionRefresh,
				private,
			),
			want: "observer_unknown",
		},
		{
			name: "write",
			err:  app.ErrRuntimeDiscoveryCommitInputFailed,
			want: "observer_write",
		},
		{
			name: "write plus unknown",
			err: fmt.Errorf(
				"%w: %w",
				app.ErrRuntimeDiscoveryCommitInputFailed,
				private,
			),
			want: "observer_unknown",
		},
		{
			name: "inventory",
			err: fmt.Errorf(
				"%w: %w",
				loomruntime.ErrRuntimeDiscoveryFailed,
				loomruntime.ErrInvalidRuntimeInstance,
			),
			want: "observer_inventory",
		},
		{
			name: "plan",
			err:  app.ErrInvalidRuntimeObservationWritePlan,
			want: "observer_plan",
		},
		{
			name: "identity metadata",
			err: fmt.Errorf(
				"%w: %w",
				app.ErrRuntimeDiscoveryCommitInputFailed,
				app.ErrLocalRuntimeObservationDaemonMetadata,
			),
			want: "observer_identity_metadata",
		},
		{
			name: "identity metadata plus unknown",
			err: errors.Join(
				app.ErrLocalRuntimeObservationDaemonMetadata,
				private,
			),
			want: "observer_unknown",
		},
		{
			name: "journal write",
			err:  journal.ErrPartialEventBatchConflict,
			want: "observer_write",
		},
		{
			name: "unknown",
			err:  private,
			want: "observer_unknown",
		},
		{
			name: "joined conflicting",
			err: errors.Join(
				testPiMetadataFailure{
					command: loomruntime.PiMetadataVersion,
					cause:   piadapter.ErrPiMetadataProcessFailed,
				},
				testPiMetadataFailure{
					command: loomruntime.PiMetadataListModels,
					cause:   loomruntime.ErrPiMetadataStderr,
				},
			),
			want: "observer_unknown",
		},
		{
			name: "same command incompatible known leaves",
			err: errors.Join(
				testPiMetadataFailure{
					command: loomruntime.PiMetadataVersion,
					cause:   piadapter.ErrPiMetadataProcessFailed,
				},
				testPiMetadataFailure{
					command: loomruntime.PiMetadataVersion,
					cause:   loomruntime.ErrPiMetadataStderr,
				},
			),
			want: "observer_unknown",
		},
		{
			name: "repeated same typed leaf",
			err: errors.Join(
				testPiMetadataFailure{
					command: loomruntime.PiMetadataVersion,
					cause:   piadapter.ErrPiMetadataProcessFailed,
				},
				testPiMetadataFailure{
					command: loomruntime.PiMetadataVersion,
					cause:   piadapter.ErrPiMetadataProcessFailed,
				},
			),
			want: "observer_version_process",
		},
		{
			name: "known plus unknown",
			err: errors.Join(
				testPiMetadataFailure{
					command: loomruntime.PiMetadataVersion,
					cause:   piadapter.ErrPiMetadataProcessFailed,
				},
				private,
			),
			want: "observer_unknown",
		},
		{
			name: "incompatible known leaves",
			err: errors.Join(
				piadapter.ErrPiLocalRuntimeCandidateInvalid,
				piadapter.ErrPiLocalRuntimeProbeConstructionFailed,
			),
			want: "observer_unknown",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := observerFailureReason(test.err)
			if got != test.want || strings.Contains(got, "private") {
				t.Fatalf(
					"observerFailureReason(%T) = %q, want %q",
					test.err,
					got,
					test.want,
				)
			}
		})
	}
}

func productDaemonFailureState(t *testing.T) (string, string) {
	t.Helper()
	root := ""
	var err error
	if canaryRoot := os.Getenv("LOOM_P2B_CANARY_ROOT"); canaryRoot != "" {
		if !filepath.IsAbs(canaryRoot) {
			t.Fatal("LOOM_P2B_CANARY_ROOT must be absolute")
		}
		root = filepath.Join(canaryRoot, "vertical-fixture")
		if err = os.Mkdir(root, 0o700); err != nil {
			t.Fatal(err)
		}
	} else {
		root, err = os.MkdirTemp("", "loom-p2a-failure-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.RemoveAll(root); err != nil {
				t.Errorf("remove failure-state root: %v", err)
			}
		})
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, "state.db")
	stateFile, err := os.OpenFile(
		statePath,
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateFile.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Migrate(context.Background(), database); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	return root, statePath
}

func assertDaemonFailureCode(t *testing.T, err error, want string) {
	t.Helper()
	var classified interface {
		DaemonFailureCode() string
	}
	if !errors.As(err, &classified) ||
		classified.DaemonFailureCode() != want {
		t.Fatalf("failure=%T %v code=%q", err, err, want)
	}
}

func assertDaemonFailureReason(t *testing.T, err error, want string) {
	t.Helper()
	var classified interface {
		DaemonFailureReason() string
	}
	if !errors.As(err, &classified) ||
		classified.DaemonFailureReason() != want {
		t.Fatalf("failure=%T %v reason=%q", err, err, want)
	}
}

type testPiMetadataFailure struct {
	command loomruntime.PiMetadataCommand
	cause   error
}

func (failure testPiMetadataFailure) Error() string {
	return "safe test Pi metadata failure"
}

func (failure testPiMetadataFailure) Unwrap() error {
	return failure.cause
}

func (failure testPiMetadataFailure) PiMetadataFailureCommand() loomruntime.PiMetadataCommand {
	return failure.command
}

func TestProductDaemonServesAuthoritativeNilCollectionsToStrictSwiftClient(
	t *testing.T,
) {
	t.Setenv("TMPDIR", "/private/tmp")
	root, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	appendProductDaemonFixture(t, database)

	service, err := api.NewLocalProductReadService(api.LocalProductReadConfig{
		Journal:    journal.NewStore(database),
		Projection: projection.New(database),
		Now: func() time.Time {
			return time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := localipc.HandlerFunc(localProductHandler(service))

	snapshotResponse := handler.Handle(context.Background(), localipc.Request{
		Method: "snapshot",
		Params: []byte(`{"limit":64}`),
	})
	if !snapshotResponse.OK {
		t.Fatalf("snapshot handler response = %#v", snapshotResponse)
	}
	if bytes.Contains(snapshotResponse.Result, []byte(`"model_ids":null`)) ||
		!bytes.Contains(snapshotResponse.Result, []byte(`"model_ids":[]`)) ||
		!bytes.Contains(
			snapshotResponse.Result,
			[]byte(`"observed_capabilities":[`),
		) {
		t.Fatalf("snapshot wire collections = %s", snapshotResponse.Result)
	}

	timelineResponse := handler.Handle(context.Background(), localipc.Request{
		Method: "timeline_page",
		Params: []byte(
			`{"team_instance_id":"team-instance.one","cursor":"","limit":64}`,
		),
	})
	if !timelineResponse.OK {
		t.Fatalf("timeline handler response = %#v", timelineResponse)
	}
	for _, requiredArray := range [][]byte{
		[]byte(`"records":[`),
		[]byte(`"nodes":[`),
		[]byte(`"attention":[`),
	} {
		if !bytes.Contains(timelineResponse.Result, requiredArray) {
			t.Fatalf(
				"timeline collection %s is not an array: %s",
				requiredArray,
				timelineResponse.Result,
			)
		}
	}

	socketPath := filepath.Join(root, "loomd.sock")
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "authoritative-collection-wire-fixture",
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
		entries, _ := os.ReadDir(root)
		t.Fatalf(
			"Go IPC server failed before ready: %v root=%q entries=%v",
			err,
			root,
			entries,
		)
	case <-time.After(5 * time.Second):
		t.Fatal("Go IPC server did not become ready")
	}
	defer func() {
		cancel()
		if err := server.Close(); err != nil {
			t.Errorf("close Go IPC server: %v", err)
		}
		select {
		case err := <-serveDone:
			if err != nil {
				t.Errorf("Go IPC Serve() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Go IPC server did not close")
		}
	}()

	probe := buildProductDaemonSwiftContractProbe(t, root)
	command := exec.Command(
		probe,
		"--socket",
		socketPath,
		"--team",
		"team-instance.one",
	)
	command.Env = productCLITestEnvironment(t, root)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("strict Swift client error=%v output=%q", err, output)
	}
	var decoded struct {
		Snapshot api.LocalProductSnapshot     `json:"snapshot"`
		Timeline api.LocalProductTimelinePage `json:"timeline"`
	}
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("strict Swift output invalid: %v, output=%q", err, output)
	}
	if len(decoded.Snapshot.Runtimes) != 1 ||
		decoded.Snapshot.Runtimes[0].ModelIDs == nil ||
		len(decoded.Snapshot.Runtimes[0].ModelIDs) != 0 ||
		decoded.Snapshot.Teams == nil ||
		decoded.Snapshot.Runs == nil ||
		decoded.Snapshot.Evidence == nil ||
		decoded.Snapshot.Attention == nil ||
		decoded.Timeline.Records == nil ||
		decoded.Timeline.Board.Nodes == nil ||
		decoded.Timeline.Attention == nil {
		t.Fatalf("strict Swift decoded collections = %#v", decoded)
	}
}

func buildProductDaemonSwiftContractProbe(t *testing.T, root string) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate product daemon test source")
	}
	repositoryRoot := filepath.Dir(filepath.Dir(filepath.Dir(sourceFile)))
	packageRoot := filepath.Join(repositoryRoot, "apps", "macos")
	scratchPath := filepath.Join(root, "swift-contract-build")
	build := exec.Command(
		"/usr/bin/swift",
		"build",
		"--package-path",
		packageRoot,
		"--scratch-path",
		scratchPath,
		"-c",
		"release",
		"--arch",
		"arm64",
		"--product",
		"LoomLocalAppContractProbe",
	)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build strict Swift contract probe: %v, output=%q", err, output)
	}
	showBinPath := exec.Command(
		"/usr/bin/swift",
		"build",
		"--package-path",
		packageRoot,
		"--scratch-path",
		scratchPath,
		"-c",
		"release",
		"--arch",
		"arm64",
		"--show-bin-path",
	)
	output, err := showBinPath.Output()
	if err != nil {
		t.Fatalf("strict Swift probe bin path: %v", err)
	}
	return filepath.Join(
		strings.TrimSpace(string(output)),
		"LoomLocalAppContractProbe",
	)
}

func TestProductDaemonRealSetupServiceConfirmsCandidateOverPrivateUDS(
	t *testing.T,
) {
	root, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	appendProductDaemonFixture(t, database, []string{"model-a"})
	beforeEvents, err := journal.NewStore(database).ReadAll(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	beforeExecutionFacts := productSetupExecutionFactCount(beforeEvents)
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(root, "loomd.sock")
	observer := &blockingObserverRunner{}
	runner, err := newProductDaemonRunner(
		observer,
		statePath,
		socketPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		runDone <- runErr
	}()
	waitForProductSocket(t, socketPath)
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	setupClient, err := loomtui.NewDaemonReadClient(client)
	if err != nil {
		t.Fatal(err)
	}
	setup, err := setupClient.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if setup.SchemaVersion != 1 ||
		len(setup.Runtimes) != 1 ||
		setup.Runtimes[0].ModelIDs == nil ||
		setup.Codex.AuthMode != "native_auth" ||
		setup.MiniMax.AuthMode != "brokered" {
		t.Fatalf("setup snapshot = %#v", setup)
	}
	session, err := setupClient.StartBuilder(
		context.Background(),
		app.BuilderStartCommand{Source: app.BuilderSourceBlank},
	)
	if err != nil {
		t.Fatal(err)
	}
	answers := map[string]string{
		"team_name":     "Private UDS Team",
		"purpose":       "Confirm one Candidate through the daemon",
		"main_role":     "coordinator",
		"subagent_role": "bounded-worker",
	}
	for session.Question.ID != "" {
		answer, ok := answers[session.Question.ID]
		if !ok {
			t.Fatalf("unexpected Builder question = %#v", session.Question)
		}
		session, err = setupClient.AnswerBuilder(
			context.Background(),
			app.BuilderAnswerCommand{
				DraftID:          session.DraftID,
				ExpectedRevision: session.Revision,
				CatalogDigest:    session.CatalogDigest,
				ViewVersion:      session.ViewVersion,
				QuestionID:       session.Question.ID,
				Answer:           answer,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !session.CanConfirm || session.BindingDigest == "" {
		t.Fatalf("Candidate session = %#v", session)
	}
	confirmation, err := setupClient.ConfirmBuilder(
		context.Background(),
		app.BuilderConfirmCommand{
			DraftID:          session.DraftID,
			ExpectedRevision: session.Revision,
			CatalogDigest:    session.CatalogDigest,
			ViewVersion:      session.ViewVersion,
			BindingDigest:    session.BindingDigest,
			DefinitionID:     "team-private-uds",
			Scope:            "reusable",
			Confirm:          true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if confirmation.TeamDefinitionID != "team-private-uds" ||
		confirmation.Status != "active" ||
		confirmation.TeamInstanceCreated ||
		confirmation.RunCreated {
		t.Fatalf("confirmation = %#v", confirmation)
	}
	cancel()
	select {
	case runErr := <-runDone:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("product daemon stop error = %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("product daemon did not stop")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	database, err = sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	events, err := journal.NewStore(database).ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	saved := 0
	for _, event := range events {
		if event.Type == "TeamDefinitionSaved" &&
			event.StreamID == "team-definition/team-private-uds" {
			saved++
		}
	}
	if saved != 1 {
		t.Fatalf("TeamDefinitionSaved events = %d", saved)
	}
	if after := productSetupExecutionFactCount(events); after != beforeExecutionFacts {
		t.Fatalf(
			"confirmation changed execution facts: before=%d after=%d",
			beforeExecutionFacts,
			after,
		)
	}
}

func TestProductDaemonSetupOnlyMiniMaxTestOverRealIPCDoesNotInitializeExecution(
	t *testing.T,
) {
	root, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	const credentialReference = "credential-ref-setup-only"
	if _, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:           "setup-only-minimax-configured",
			ProviderID:          "minimax",
			CredentialReference: credentialReference,
			ExpectedRevision:    0,
			OccurredAt:          time.Unix(750, 0).UTC(),
			Status:              credentials.CredentialConfigured,
			Reason:              credentials.VerificationReasonNone,
		},
	); err != nil {
		t.Fatal(err)
	}
	beforeEvents, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := productSetupExecutionFactCount(beforeEvents); got != 0 {
		t.Fatalf("initial execution facts = %d", got)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	executionRoot := filepath.Join(filepath.Dir(statePath), "execution")
	if _, err := os.Lstat(executionRoot); !os.IsNotExist(err) {
		t.Fatalf("execution root exists before setup-only construction: %v", err)
	}
	socketPath := filepath.Join(root, "loomd.sock")
	observer := &blockingObserverRunner{}
	runner, err := newProductDaemonRunner(
		observer,
		statePath,
		socketPath,
		productSetupRuntimeConfig{
			CredentialStore: &productCredentialTestStore{},
			Execution:       nil,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if runner.execution != nil {
		t.Fatal("setup-only construction initialized execution bundle")
	}
	if _, err := os.Lstat(executionRoot); !os.IsNotExist(err) {
		t.Fatalf("setup-only construction created execution root: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		runDone <- runErr
	}()
	waitForProductSocket(t, socketPath)
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	setupClient, err := loomtui.NewDaemonReadClient(client)
	if err != nil {
		t.Fatal(err)
	}
	setup, err := setupClient.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if setup.MiniMax.Status != "configured" ||
		setup.MiniMax.Revision != 1 ||
		setup.MiniMax.CredentialReference != credentialReference {
		t.Fatalf("setup-only MiniMax snapshot = %#v", setup.MiniMax)
	}

	var result app.CredentialSetupResult
	err = client.Call(
		context.Background(),
		"credential_verify",
		struct {
			ProviderID          string `json:"provider_id"`
			CredentialReference string `json:"credential_reference"`
			ExpectedRevision    int64  `json:"expected_revision"`
			OperationID         string `json:"operation_id"`
			Secret              string `json:"secret"`
		}{
			ProviderID:          "minimax",
			CredentialReference: credentialReference,
			ExpectedRevision:    1,
			OperationID:         "22222222-2222-4222-8222-222222222222",
			Secret:              "",
		},
		&result,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderID != "minimax" || result.Revision != 2 ||
		result.Status != "rejected" || result.Reason != "unavailable" {
		t.Fatalf("setup-only MiniMax terminal = %#v", result)
	}
	if _, err := os.Lstat(executionRoot); !os.IsNotExist(err) {
		t.Fatalf("credential Test created execution root: %v", err)
	}

	cancel()
	select {
	case runErr := <-runDone:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("product daemon stop error = %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("product daemon did not stop")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("product socket remains after close: %v", err)
	}

	database, err = sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	afterEvents, err := journal.NewStore(database).ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(afterEvents) != len(beforeEvents)+1 ||
		afterEvents[len(afterEvents)-1].Type != "ProviderCredentialVerified" ||
		productSetupExecutionFactCount(afterEvents) != 0 {
		t.Fatalf("setup-only postflight events = %#v", afterEvents)
	}
	if _, err := os.Lstat(executionRoot); !os.IsNotExist(err) {
		t.Fatalf("setup-only postflight execution root: %v", err)
	}
}

func TestProductDaemonExecutionCompositionMaterializesConfirmedTeamForPreflight(
	t *testing.T,
) {
	root, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	appendProductExecutionRuntimeFixture(t, database)
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	privateRoot := filepath.Join(root, "local-model")
	if err := os.Mkdir(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(root, "loomd.sock")
	runner, err := newProductDaemonRunner(
		&blockingObserverRunner{},
		statePath,
		socketPath,
		productSetupRuntimeConfig{Execution: &productMissionExecutionRuntimeConfig{
			RuntimeSearchPaths: []string{root},
			RuntimeInstanceID:  "runtime-1",
			LocalModelCatalog: &piadapter.PiLocalModelCatalogConfig{
				PrivateRoot:    privateRoot,
				ExecutablePath: filepath.Join(privateRoot, "not-started-llama"),
				ModelPath:      filepath.Join(privateRoot, "not-read-model.gguf"),
			},
			Now: func() time.Time {
				return time.Date(2026, 8, 1, 10, 5, 0, 0, time.UTC)
			},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		done <- runErr
	}()
	waitForProductSocket(t, socketPath)
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	setupClient, err := loomtui.NewDaemonReadClient(client)
	if err != nil {
		t.Fatal(err)
	}
	session, err := setupClient.StartBuilder(
		context.Background(),
		app.BuilderStartCommand{Source: app.BuilderSourceBlank},
	)
	if err != nil {
		t.Fatal(err)
	}
	answers := map[string]string{
		"team_name":     "Controlled Execution Team",
		"purpose":       "Run one bounded verified Mission",
		"main_role":     "coordinator",
		"subagent_role": "bounded-worker",
	}
	for session.Question.ID != "" {
		answer, ok := answers[session.Question.ID]
		if !ok {
			t.Fatalf("unexpected Builder question = %#v", session.Question)
		}
		session, err = setupClient.AnswerBuilder(
			context.Background(),
			app.BuilderAnswerCommand{
				DraftID: session.DraftID, ExpectedRevision: session.Revision,
				CatalogDigest: session.CatalogDigest,
				ViewVersion:   session.ViewVersion,
				QuestionID:    session.Question.ID,
				Answer:        answer,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	confirmation, err := setupClient.ConfirmBuilder(
		context.Background(),
		app.BuilderConfirmCommand{
			DraftID: session.DraftID, ExpectedRevision: session.Revision,
			CatalogDigest: session.CatalogDigest,
			ViewVersion:   session.ViewVersion,
			BindingDigest: session.BindingDigest,
			DefinitionID:  "team-controlled-execution",
			Scope:         "reusable",
			Confirm:       true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if confirmation.TeamDefinitionID != "team-controlled-execution" ||
		!confirmation.TeamInstanceCreated || confirmation.RunCreated {
		t.Fatalf("confirmation = %#v", confirmation)
	}
	var snapshot api.LocalProductSnapshot
	if err := client.Call(
		context.Background(),
		"snapshot",
		api.LocalProductSnapshotRequest{Limit: 64},
		&snapshot,
	); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Teams) != 1 || !snapshot.Teams[0].Confirmed ||
		!snapshot.Teams[0].Executable || snapshot.Teams[0].ReadOnly {
		t.Fatalf("post-confirm snapshot Teams = %#v", snapshot.Teams)
	}
	teamID := snapshot.Teams[0].TeamInstanceID
	diagnosticDB, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	diagnosticProjection := projection.New(diagnosticDB)
	if err := diagnosticProjection.Rebuild(context.Background()); err != nil {
		_ = diagnosticDB.Close()
		t.Fatal(err)
	}
	diagnosticBinding, err := app.NewProjectionMissionExecutionBindingSource(
		diagnosticProjection,
	)
	if err != nil {
		_ = diagnosticDB.Close()
		t.Fatal(err)
	}
	if _, err := diagnosticBinding.ResolveMissionExecutionBinding(
		context.Background(),
		teamID,
	); err != nil {
		_ = diagnosticDB.Close()
		t.Fatalf("materialized binding = %v snapshot=%#v", err, diagnosticProjection.Snapshot())
	}
	if err := diagnosticDB.Close(); err != nil {
		t.Fatal(err)
	}
	workPackage, err := work.CodingWorkPackage()
	if err != nil {
		t.Fatal(err)
	}
	var envelope api.MissionExecutionEnvelope
	if err := client.Call(
		context.Background(),
		"mission_execution",
		app.MissionExecutionCommand{
			SchemaVersion:  app.MissionExecutionSchemaVersion,
			Operation:      "preflight",
			MissionID:      "mission/" + teamID,
			TeamInstanceID: teamID,
			WorkPackageID:  workPackage.ID(), WorkPackageDigest: workPackage.Digest(),
			Objective:           "Produce one bounded verified result",
			ExpectedViewVersion: snapshot.ViewVersion,
			CorrelationID:       "33333333-3333-4333-8333-333333333333",
		},
		&envelope,
	); err != nil {
		t.Fatal(err)
	}
	if envelope.Operation != "preflight" || envelope.Preflight == nil ||
		envelope.Preflight.TeamInstanceID != teamID {
		t.Fatalf("preflight envelope = %#v", envelope)
	}
	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("product daemon stop error = %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("product daemon did not stop")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	database, err = sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	events, err := journal.NewStore(database).ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, event := range events {
		counts[event.Type]++
	}
	if counts["TeamDefinitionSaved"] != 1 ||
		counts["TeamInstanceCreated"] != 1 ||
		counts["AgentInstanceCreated"] != 1 ||
		counts["WorkItemCreated"] != 0 || counts["RunCreated"] != 0 {
		t.Fatalf("post-confirm authority facts = %#v", counts)
	}
}

func TestProductSetupRemainsAvailableWhenNoRuntimeCanBuildATeam(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	setup, err := buildProductSetupService(
		database,
		journal.NewStore(database),
		readModel,
		productSetupRuntimeConfig{
			CredentialStore: &productCredentialTestStore{},
		},
	)
	if err != nil || setup == nil {
		t.Fatalf("buildProductSetupService() = %#v, %v", setup, err)
	}
	snapshot, err := setup.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Runtimes == nil ||
		len(snapshot.Runtimes) != 0 ||
		snapshot.Codex.AuthMode != "native_auth" ||
		snapshot.MiniMax.AuthMode != "brokered" {
		t.Fatalf("provider-only setup snapshot = %#v", snapshot)
	}
	if _, err := setup.StartBuilder(
		context.Background(),
		app.BuilderStartCommand{Source: app.BuilderSourceBlank},
	); !errors.Is(err, app.ErrBuilderIncompatible) {
		t.Fatalf("StartBuilder() without a Runtime error = %v", err)
	}
}

func TestProductSetupFailsClosedWhenProcessKeychainCannotBeConstructed(
	t *testing.T,
) {
	root, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	setup, err := buildProductSetupService(
		database,
		journal.NewStore(database),
		readModel,
		productSetupRuntimeConfig{
			SocketPath: filepath.Join(
				filepath.Base(root),
				"relative.sock",
			),
		},
	)
	if err == nil || setup != nil {
		t.Fatalf(
			"buildProductSetupService() = %#v, %v; want fail closed",
			setup,
			err,
		)
	}
}

type productCredentialTestSource struct {
	result    credentials.MetadataResult
	commandID string
	err       error
	calls     int
}

type productCredentialTestStore struct{}

func (*productCredentialTestStore) Put(
	context.Context,
	string,
	[]byte,
) error {
	return credentials.ErrCredentialStoreUnavailable
}

func (source *productCredentialTestSource) CredentialOperationStatus(
	_ context.Context,
	_ string,
) (productCredentialOperationStatus, error) {
	source.calls++
	return productCredentialOperationStatus{
		metadata: source.result, commandID: source.commandID,
	}, source.err
}

func (*productCredentialTestStore) Read(
	context.Context,
	string,
) ([]byte, error) {
	return nil, credentials.ErrCredentialNotFound
}

func (*productCredentialTestStore) Delete(
	context.Context,
	string,
) error {
	return credentials.ErrCredentialNotFound
}

func (source *productCredentialTestSource) CredentialStatus(
	_ context.Context,
	_ string,
) (credentials.MetadataResult, error) {
	source.calls++
	return source.result, source.err
}

type productCredentialTestMutator struct {
	verifyResult credentials.MetadataResult
	verifyErr    error
	verifyCalls  int
}

func (mutator *productCredentialTestMutator) Configure(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return credentials.MetadataResult{}, nil
}

func (mutator *productCredentialTestMutator) Verify(
	_ context.Context,
	_ credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	mutator.verifyCalls++
	return mutator.verifyResult, mutator.verifyErr
}

func (mutator *productCredentialTestMutator) Replace(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return credentials.MetadataResult{}, nil
}

func (mutator *productCredentialTestMutator) Revoke(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return credentials.MetadataResult{}, nil
}

func TestProductCredentialVerifyRecoversLostTerminalResponseWithoutRetry(
	t *testing.T,
) {
	terminal := credentials.MetadataResult{
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-recovery",
		Revision:            8,
		Status:              credentials.CredentialVerified,
	}
	source := &productCredentialTestSource{
		result: terminal, commandID: "verify-credential-recovery",
	}
	delegate := &productCredentialTestMutator{}
	mutator := productCredentialMutator{
		status:   source,
		delegate: delegate,
	}

	result, err := mutator.Verify(
		context.Background(),
		credentials.CredentialCommand{
			CommandID:           "verify-credential-recovery",
			ProviderID:          "minimax",
			CredentialReference: "credential-ref-recovery",
			ExpectedRevision:    7,
			OccurredAt:          time.Unix(700, 0).UTC(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != terminal ||
		source.calls != 1 ||
		delegate.verifyCalls != 0 {
		t.Fatalf(
			"result=%#v source_calls=%d delegate_calls=%d",
			result,
			source.calls,
			delegate.verifyCalls,
		)
	}
}

func TestProductCredentialVerifyAllowsOnlyExactCurrentRevisionToBroker(
	t *testing.T,
) {
	current := credentials.MetadataResult{
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-current",
		Revision:            4,
		Status:              credentials.CredentialConfigured,
	}
	delegated := credentials.MetadataResult{
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-current",
		Revision:            5,
		Status:              credentials.CredentialVerified,
	}
	source := &productCredentialTestSource{result: current}
	delegate := &productCredentialTestMutator{verifyResult: delegated}
	mutator := productCredentialMutator{
		status:   source,
		delegate: delegate,
	}

	result, err := mutator.Verify(
		context.Background(),
		credentials.CredentialCommand{
			ProviderID:          "minimax",
			CredentialReference: "credential-ref-current",
			ExpectedRevision:    4,
			OccurredAt:          time.Unix(701, 0).UTC(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != delegated ||
		source.calls != 1 ||
		delegate.verifyCalls != 1 {
		t.Fatalf(
			"result=%#v source_calls=%d delegate_calls=%d",
			result,
			source.calls,
			delegate.verifyCalls,
		)
	}
}

func TestProductCredentialVerifyRejectsEveryOtherStaleShapeBeforeBroker(
	t *testing.T,
) {
	tests := []struct {
		name    string
		current credentials.MetadataResult
	}{
		{
			name: "reference drift",
			current: credentials.MetadataResult{
				ProviderID:          "minimax",
				CredentialReference: "credential-ref-other",
				Revision:            4,
				Status:              credentials.CredentialConfigured,
			},
		},
		{
			name: "revision gap",
			current: credentials.MetadataResult{
				ProviderID:          "minimax",
				CredentialReference: "credential-ref-current",
				Revision:            6,
				Status:              credentials.CredentialVerified,
			},
		},
		{
			name: "next revision nonterminal",
			current: credentials.MetadataResult{
				ProviderID:          "minimax",
				CredentialReference: "credential-ref-current",
				Revision:            5,
				Status:              credentials.CredentialConfigured,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := &productCredentialTestSource{result: test.current}
			delegate := &productCredentialTestMutator{}
			mutator := productCredentialMutator{
				status:   source,
				delegate: delegate,
			}
			_, err := mutator.Verify(
				context.Background(),
				credentials.CredentialCommand{
					ProviderID:          "minimax",
					CredentialReference: "credential-ref-current",
					ExpectedRevision:    4,
					OccurredAt:          time.Unix(702, 0).UTC(),
				},
			)
			if !errors.Is(
				err,
				credentials.ErrCredentialMetadataConflict,
			) {
				t.Fatalf("Verify() error = %v", err)
			}
			if source.calls != 1 || delegate.verifyCalls != 0 {
				t.Fatalf(
					"source_calls=%d delegate_calls=%d",
					source.calls,
					delegate.verifyCalls,
				)
			}
		})
	}
}

type productConcurrentCredentialFixture struct {
	mu          sync.Mutex
	current     credentials.MetadataResult
	commandID   string
	verifyCalls int
}

func (fixture *productConcurrentCredentialFixture) CredentialStatus(
	context.Context,
	string,
) (credentials.MetadataResult, error) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.current, nil
}

func (fixture *productConcurrentCredentialFixture) CredentialOperationStatus(
	_ context.Context,
	_ string,
) (productCredentialOperationStatus, error) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return productCredentialOperationStatus{
		metadata: fixture.current, commandID: fixture.commandID,
	}, nil
}

func (fixture *productConcurrentCredentialFixture) Configure(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return credentials.MetadataResult{}, errors.New("unexpected configure")
}

func (fixture *productConcurrentCredentialFixture) Verify(
	_ context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	fixture.verifyCalls++
	fixture.commandID = command.CommandID
	fixture.current = credentials.MetadataResult{
		ProviderID:          command.ProviderID,
		CredentialReference: command.CredentialReference,
		Revision:            command.ExpectedRevision + 1,
		Status:              credentials.CredentialVerified,
	}
	return fixture.current, nil
}

func (fixture *productConcurrentCredentialFixture) Replace(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return credentials.MetadataResult{}, errors.New("unexpected replace")
}

func (fixture *productConcurrentCredentialFixture) Revoke(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return credentials.MetadataResult{}, errors.New("unexpected revoke")
}

func TestProductCredentialConcurrentLostResponseRecoveryObservesProviderOnce(
	t *testing.T,
) {
	fixture := &productConcurrentCredentialFixture{
		current: credentials.MetadataResult{
			ProviderID:          "minimax",
			CredentialReference: "credential-ref-concurrent",
			Revision:            9,
			Status:              credentials.CredentialConfigured,
		},
	}
	mutator := &productCredentialMutator{
		status:   fixture,
		delegate: fixture,
	}
	command := credentials.CredentialCommand{
		CommandID:           "verify-credential-operation-a",
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-concurrent",
		ExpectedRevision:    9,
		OccurredAt:          time.Unix(703, 0).UTC(),
	}
	results := make(chan credentials.MetadataResult, 2)
	failures := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := mutator.Verify(context.Background(), command)
			if err != nil {
				failures <- err
				return
			}
			results <- result
		}()
	}
	wait.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatalf("concurrent Verify() error = %v", err)
	}
	count := 0
	for result := range results {
		count++
		if result.Revision != 10 ||
			result.Status != credentials.CredentialVerified {
			t.Fatalf("concurrent result = %#v", result)
		}
	}
	fixture.mu.Lock()
	verifyCalls := fixture.verifyCalls
	fixture.mu.Unlock()
	if count != 2 || verifyCalls != 1 {
		t.Fatalf("results=%d Provider observations=%d", count, verifyCalls)
	}
	_, err := mutator.Verify(context.Background(), credentials.CredentialCommand{
		CommandID:           "verify-credential-operation-b",
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-concurrent",
		ExpectedRevision:    9,
		OccurredAt:          time.Unix(704, 0).UTC(),
	})
	if !errors.Is(err, credentials.ErrCredentialMetadataConflict) {
		t.Fatalf("different stale operation error = %v", err)
	}
	fixture.mu.Lock()
	verifyCalls = fixture.verifyCalls
	fixture.mu.Unlock()
	if verifyCalls != 1 {
		t.Fatalf("different stale operation observed Provider: %d", verifyCalls)
	}
}

func TestProductCredentialOperationStatusBindsLatestJournalCommand(
	t *testing.T,
) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID: "configure-credential-source", ProviderID: "minimax",
			CredentialReference: "credential-ref-source", ExpectedRevision: 0,
			OccurredAt: time.Unix(710, 0).UTC(),
			Status:     credentials.CredentialConfigured,
		},
	); err != nil {
		t.Fatal(err)
	}
	verifyCommandID := "verify-credential-11111111-1111-4111-8111-111111111111"
	if _, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID: verifyCommandID, ProviderID: "minimax",
			CredentialReference: "credential-ref-source", ExpectedRevision: 1,
			OccurredAt: time.Unix(711, 0).UTC(),
			Status:     credentials.CredentialVerified,
		},
	); err != nil {
		t.Fatal(err)
	}
	source := productCredentialStatusSource{
		projection: projection.New(database), store: store,
	}
	status, err := source.CredentialOperationStatus(
		context.Background(),
		"minimax",
	)
	if err != nil {
		t.Fatal(err)
	}
	if status.commandID != verifyCommandID || status.metadata.Revision != 2 ||
		status.metadata.Status != credentials.CredentialVerified {
		t.Fatalf("operation status = %#v", status)
	}
}

func TestLoomdCredentialHelperDirectAndPipeOnlyActivationFailClosed(
	t *testing.T,
) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate loomd source")
	}
	repositoryRoot := filepath.Dir(filepath.Dir(filepath.Dir(sourceFile)))
	binaryPath := filepath.Join(t.TempDir(), "loomd-helper-fixture")
	build := exec.Command(
		"go",
		"build",
		"-o",
		binaryPath,
		"./cmd/loomd",
	)
	build.Dir = repositoryRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build loomd helper fixture: %v output=%q", err, output)
	}

	direct := exec.Command(binaryPath, "--credential-helper")
	direct.Env = []string{}
	if output, err := direct.CombinedOutput(); err == nil ||
		len(output) != 0 {
		t.Fatalf(
			"direct helper activation error=%v output=%q",
			err,
			output,
		)
	}

	requestRead, requestWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer requestRead.Close()
	defer requestWrite.Close()
	responseRead, responseWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer responseRead.Close()
	defer responseWrite.Close()
	pipeOnly := exec.Command(binaryPath, "--credential-helper")
	pipeOnly.Env = []string{}
	pipeOnly.ExtraFiles = []*os.File{requestRead, responseWrite}
	var stdout, stderr bytes.Buffer
	pipeOnly.Stdout = &stdout
	pipeOnly.Stderr = &stderr
	if err := pipeOnly.Run(); err == nil {
		t.Fatal("pipe-only helper activation unexpectedly succeeded")
	}
	_ = requestRead.Close()
	_ = responseWrite.Close()
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf(
			"pipe-only helper disclosed output stdout=%q stderr=%q",
			stdout.Bytes(),
			stderr.Bytes(),
		)
	}
}

func productSetupExecutionFactCount(events []journal.Event) int {
	count := 0
	for _, event := range events {
		switch event.Type {
		case "TeamInstanceCreated", "AgentInstanceCreated", "WorkItemCreated",
			"RunCreated", "AgentGrantIssued", "EvidenceSubmitted",
			"TeamExecutionPlanned", "TeamReadySetDispatched":
			count++
		}
	}
	return count
}

func TestProductDaemonServesRealReadOnlySQLiteOverPrivateUDSAndCleansUp(
	t *testing.T,
) {
	root, err := os.MkdirTemp("", "loom-p2a-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove product daemon root: %v", err)
		}
	})
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, "state.db")
	stateFile, err := os.OpenFile(
		statePath,
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateFile.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	appendProductDaemonFixture(t, database)
	fixtureProjection := projection.New(database)
	if err := fixtureProjection.Rebuild(context.Background()); err != nil {
		t.Fatalf("non-empty fixture projection: %v", err)
	}
	beforeHeads := productDaemonHeadsDigest(t, database)
	var beforeEventCount int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM events`,
	).Scan(&beforeEventCount); err != nil {
		t.Fatal(err)
	}
	if beforeEventCount == 0 {
		t.Fatal("non-empty product fixture has zero Events")
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(root, "loomd.sock")
	observer := &blockingObserverRunner{}
	runner, err := newProductDaemonRunner(
		observer,
		statePath,
		socketPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		done <- runErr
	}()
	waitForProductSocket(t, socketPath)

	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot api.LocalProductSnapshot
	if err := client.Call(
		context.Background(),
		"snapshot",
		api.LocalProductSnapshotRequest{Limit: 64},
		&snapshot,
	); err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != 3 ||
		snapshot.ViewVersion == "" ||
		len(snapshot.Teams) != 1 ||
		len(snapshot.Missions) != 1 ||
		snapshot.Missions[0].Title != "Saved team" ||
		len(snapshot.Runtimes) != 1 ||
		snapshot.Runtimes[0].ModelIDs == nil ||
		snapshot.Runtimes[0].ObservedCapabilities == nil ||
		len(snapshot.Runs) != 1 ||
		len(snapshot.Evidence) != 1 ||
		snapshot.Attention == nil || snapshot.SideTasks == nil {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	readClient, err := loomtui.NewDaemonReadClient(client)
	if err != nil {
		t.Fatal(err)
	}
	model, err := loomtui.NewModel(readClient)
	if err != nil {
		t.Fatal(err)
	}
	message := model.Init()()
	updated, command := model.Update(message)
	if command != nil ||
		!strings.Contains(updated.View(), "Saved team") ||
		strings.Contains(updated.View(), "team.delivery") ||
		strings.Contains(updated.View(), snapshot.ViewVersion) {
		t.Fatalf(
			"headless TUI does not present the daemon view safely %q: %q",
			snapshot.ViewVersion,
			updated.View(),
		)
	}
	model = updated.(loomtui.Model)
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyDown},
		{Type: tea.KeyEnter},
	} {
		updatedModel, next := model.Update(key)
		model = updatedModel.(loomtui.Model)
		if next != nil {
			updatedModel, _ = model.Update(next())
			model = updatedModel.(loomtui.Model)
		}
	}
	if !strings.Contains(model.View(), "Team planned") {
		t.Fatalf("TUI timeline did not render the activity record: %q", model.View())
	}
	var firstPage api.LocalProductTimelinePage
	if err := client.Call(
		context.Background(),
		"timeline_page",
		api.LocalProductTimelineRequest{
			TeamInstanceID: "team-instance.one",
			Limit:          1,
		},
		&firstPage,
	); err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Records) != 1 || firstPage.NextCursor == "" {
		t.Fatalf("first timeline page = %#v", firstPage)
	}
	var resumedPage api.LocalProductTimelinePage
	if err := client.Call(
		context.Background(),
		"timeline_page",
		api.LocalProductTimelineRequest{
			TeamInstanceID: "team-instance.one",
			Cursor:         firstPage.NextCursor,
			Limit:          1,
		},
		&resumedPage,
	); err != nil {
		t.Fatal(err)
	}
	if len(resumedPage.Records) != 0 ||
		resumedPage.NextCursor == "" {
		t.Fatalf("resumed timeline page = %#v", resumedPage)
	}

	cliPath := buildProductCLI(t, root)
	statusCommand := exec.Command(
		cliPath,
		"status",
		"--socket",
		socketPath,
	)
	statusCommand.Env = productCLITestEnvironment(t, root)
	var statusStdout, statusStderr bytes.Buffer
	statusCommand.Stdout = &statusStdout
	statusCommand.Stderr = &statusStderr
	if err := statusCommand.Run(); err != nil {
		t.Fatalf(
			"real CLI status error=%v stdout=%q stderr=%q",
			err,
			statusStdout.String(),
			statusStderr.String(),
		)
	}
	var cliSnapshot struct {
		Command    string `json:"command"`
		SourceMode string `json:"source_mode"`
		api.LocalProductSnapshot
	}
	if err := json.Unmarshal(statusStdout.Bytes(), &cliSnapshot); err != nil ||
		cliSnapshot.Command != "status" ||
		cliSnapshot.SourceMode != "daemon_api" ||
		cliSnapshot.ViewVersion != snapshot.ViewVersion ||
		len(cliSnapshot.Teams) != 1 ||
		cliSnapshot.Teams[0].TeamInstanceID != "team-instance.one" ||
		len(cliSnapshot.Runtimes) != 1 ||
		len(cliSnapshot.Runs) != 1 ||
		cliSnapshot.Runs[0].RunID != "run-1" ||
		len(cliSnapshot.Evidence) != 1 ||
		cliSnapshot.Evidence[0].EvidenceID != "evidence-1" {
		t.Fatalf(
			"real CLI status=%q decode_error=%v",
			statusStdout.String(),
			err,
		)
	}

	timelineCommand := exec.Command(
		cliPath,
		"timeline",
		"--socket",
		socketPath,
		"--team",
		"team-instance.one",
		"--limit",
		"1",
	)
	timelineCommand.Env = productCLITestEnvironment(t, root)
	var timelineStdout, timelineStderr bytes.Buffer
	timelineCommand.Stdout = &timelineStdout
	timelineCommand.Stderr = &timelineStderr
	if err := timelineCommand.Run(); err != nil {
		t.Fatalf(
			"real CLI timeline error=%v stdout=%q stderr=%q",
			err,
			timelineStdout.String(),
			timelineStderr.String(),
		)
	}
	var cliTimeline struct {
		Command    string `json:"command"`
		SourceMode string `json:"source_mode"`
		api.LocalProductTimelinePage
	}
	if err := json.Unmarshal(
		timelineStdout.Bytes(),
		&cliTimeline,
	); err != nil ||
		cliTimeline.Command != "timeline" ||
		cliTimeline.SourceMode != "daemon_api" ||
		cliTimeline.TeamInstanceID != firstPage.TeamInstanceID ||
		cliTimeline.ViewVersion != firstPage.ViewVersion ||
		cliTimeline.NextCursor != firstPage.NextCursor ||
		!reflect.DeepEqual(cliTimeline.Records, firstPage.Records) {
		t.Fatalf(
			"real CLI timeline=%q decode_error=%v",
			timelineStdout.String(),
			err,
		)
	}

	cancel()
	if runErr := <-done; !errors.Is(runErr, context.Canceled) {
		t.Fatalf("Run() error = %v", runErr)
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runner.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if !observer.closed {
		t.Fatal("observer was not closed")
	}
	if _, err := os.Lstat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("socket remains after close: %v", err)
	}

	restartObserver := &blockingObserverRunner{}
	restarted, err := newProductDaemonRunner(
		restartObserver,
		statePath,
		socketPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	restartCtx, restartCancel := context.WithCancel(context.Background())
	restartDone := make(chan error, 1)
	go func() {
		_, runErr := restarted.Run(restartCtx)
		restartDone <- runErr
	}()
	waitForProductSocket(t, socketPath)
	restartClient, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var restartedSnapshot api.LocalProductSnapshot
	if err := restartClient.Call(
		context.Background(),
		"snapshot",
		api.LocalProductSnapshotRequest{Limit: 64},
		&restartedSnapshot,
	); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot, restartedSnapshot) {
		t.Fatalf(
			"restart snapshot changed\nbefore=%#v\nafter=%#v",
			snapshot,
			restartedSnapshot,
		)
	}
	restartCancel()
	if runErr := <-restartDone; !errors.Is(runErr, context.Canceled) {
		t.Fatalf("restarted Run() error = %v", runErr)
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
	if !restartObserver.closed {
		t.Fatal("restart observer was not closed")
	}

	verificationDB, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer verificationDB.Close()
	var eventCount int
	if err := verificationDB.QueryRow(
		`SELECT COUNT(*) FROM events`,
	).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != beforeEventCount {
		t.Fatalf(
			"read product mutated Journal: event count = %d, want %d",
			eventCount,
			beforeEventCount,
		)
	}
	if afterHeads := productDaemonHeadsDigest(
		t,
		verificationDB,
	); afterHeads != beforeHeads {
		t.Fatalf(
			"read product changed stream heads: %s, want %s",
			afterHeads,
			beforeHeads,
		)
	}
}

func buildProductCLI(t *testing.T, root string) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate product daemon test source")
	}
	repositoryRoot := filepath.Dir(filepath.Dir(filepath.Dir(sourceFile)))
	cliPath := filepath.Join(root, "loom")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	command := exec.Command(
		goBinary,
		"build",
		"-trimpath",
		"-o",
		cliPath,
		"./cmd/loom",
	)
	command.Dir = repositoryRoot
	command.Env = []string{
		"GOENV=off",
		"GOPROXY=off",
		"GOTOOLCHAIN=local",
		"HOME=" + home,
		"LANG=C",
		"LC_ALL=C",
		"PATH=" + filepath.Join(runtime.GOROOT(), "bin") +
			":/usr/bin:/bin",
		"TMPDIR=" + root,
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build real CLI error=%v output=%q", err, output)
	}
	return cliPath
}

func productCLITestEnvironment(t *testing.T, root string) []string {
	t.Helper()
	return []string{
		"HOME=" + root,
		"LANG=C",
		"LC_ALL=C",
		"PATH=/usr/bin:/bin",
		"TERM=dumb",
		"TMPDIR=" + root,
	}
}

func TestProductDaemonHelpersFailClosedWithSafeCodes(t *testing.T) {
	if _, err := newProductDaemonRunner(
		nil,
		"missing.db",
		"/tmp/loomd.sock",
	); err == nil {
		t.Fatal("nil observer error = nil")
	}
	if _, err := openProductReadDatabase(
		filepath.Join(t.TempDir(), "missing.db"),
	); err == nil {
		t.Fatal("missing state error = nil")
	}
	if err := decodeExactProductParams(
		[]byte(`{"limit":64} trailing`),
		&api.LocalProductSnapshotRequest{},
	); err == nil {
		t.Fatal("trailing params error = nil")
	}
	if response := productResultResponse(make(chan int)); response.OK ||
		response.Error == nil ||
		response.Error.Code != "internal" {
		t.Fatalf("marshal failure response = %#v", response)
	}
	for _, test := range []struct {
		err      error
		wantCode string
	}{
		{api.ErrInvalidLocalProductRequest, "invalid_request"},
		{api.ErrInvalidTimelineRequest, "invalid_request"},
		{api.ErrTeamTimelineNotFound, "not_found"},
		{work.ErrSideTaskNotFound, "not_found"},
		{api.ErrTimelineCursorConflict, "cursor_conflict"},
		{api.ErrStreamGap, "stream_gap"},
		{app.ErrMissionExecutionConflict, "conflict"},
		{app.ErrMissionExecutionBusy, "busy"},
		{context.DeadlineExceeded, "timeout"},
		{errors.New("private database path"), "state_unavailable"},
	} {
		response := productServiceError(test.err)
		if response.OK || response.Error == nil ||
			response.Error.Code != test.wantCode ||
			strings.Contains(response.Error.Message, "private") {
			t.Fatalf(
				"productServiceError(%v) = %#v",
				test.err,
				response,
			)
		}
	}
	if protocolErr := localipcSafeError(
		"not-real",
		errors.New("secret"),
	); protocolErr.Code != "internal" ||
		strings.Contains(protocolErr.Message, "secret") {
		t.Fatalf("fallback protocol error = %#v", protocolErr)
	}
	handler := localProductHandler(nil)
	for _, request := range []localipc.Request{
		{
			Method: "snapshot",
			Params: []byte(`{"limit":64,"unexpected":true}`),
		},
		{
			Method: "timeline_page",
			Params: []byte(`{"team_instance_id":"team-1","limit":128,"unexpected":true}`),
		},
		{Method: "missing", Params: []byte(`{}`)},
	} {
		response := handler(context.Background(), request)
		if response.OK || response.Error == nil {
			t.Fatalf("invalid handler response = %#v", response)
		}
	}
}

func waitForProductSocket(t *testing.T, socketPath string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		info, err := os.Lstat(socketPath)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("product socket not ready: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
}

func appendProductDaemonFixture(
	t *testing.T,
	database *sql.DB,
	runtimeModels ...[]string,
) {
	t.Helper()
	const (
		digest      = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		digestB     = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
		correlation = "11111111-1111-4111-8111-111111111111"
		claimID     = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	encode := func(value any) []byte {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	event := func(
		id, streamID string,
		sequence int64,
		eventType, causation string,
		payload any,
	) journal.Event {
		correlationID := correlation
		if eventType == "TeamInstanceCreated" ||
			eventType == "AgentInstanceCreated" {
			correlationID = "request.one"
		}
		return journal.Event{
			ID:             id,
			StreamID:       streamID,
			Seq:            sequence,
			IdempotencyKey: "idem-" + id,
			Type:           eventType,
			SchemaVersion:  1,
			EmittedAt: time.Date(
				2026,
				7,
				28,
				12,
				0,
				int(sequence),
				0,
				time.UTC,
			),
			CorrelationID: correlationID,
			CausationID:   causation,
			PayloadJSON:   encode(payload),
		}
	}
	lease := time.Date(
		2026,
		7,
		28,
		12,
		5,
		0,
		0,
		time.UTC,
	).Format(time.RFC3339Nano)
	var modelIDs any
	if len(runtimeModels) == 1 {
		modelIDs = append([]string{}, runtimeModels[0]...)
	}
	events := []journal.Event{
		event(
			"runtime-discovery",
			"runtime_instance:runtime-1",
			1,
			"RuntimeInstanceDiscovered",
			"",
			map[string]any{
				"discovery_digest": digest,
				"source_probe_id":  "probe-1",
				"instance": map[string]any{
					"id":                    "runtime-1",
					"device_id":             "device-1",
					"adapter_type":          "pi-cli",
					"display_name":          "Local Pi",
					"executable_version":    "0.82.1",
					"status":                "online",
					"observed_capabilities": []string{"models"},
					"capacity":              1,
				},
				"model_ids": modelIDs,
			},
		),
		event(
			"team-created",
			"team_instance:team-instance.one",
			1,
			"TeamInstanceCreated",
			"",
			map[string]any{
				"team": map[string]any{
					"id":                      "team-instance.one",
					"work_request_id":         "request.one",
					"source_kind":             "saved_team",
					"team_definition_id":      "team.delivery",
					"team_definition_version": 1,
					"team_definition_scope":   "project",
					"scope_identity": map[string]any{
						"project_id":    "project.one",
						"generation_id": "",
					},
					"team_definition_digest": digest,
					"source_plan_digest":     digestB,
					"state":                  "created",
					"created_at":             int64(1_721_865_600),
				},
				"dormant_sub_agents":       []any{},
				"source_plan_digest":       digestB,
				"source_record_set_digest": digest,
				"team_instance_count":      1,
				"agent_instance_count":     1,
				"active_sub_agent_count":   0,
				"work_item_count":          0,
			},
		),
		event(
			"agent-created",
			"agent_instance:agent-instance.main",
			1,
			"AgentInstanceCreated",
			"team-created",
			map[string]any{
				"main_agent": map[string]any{
					"id":                       "agent-instance.main",
					"team_instance_id":         "team-instance.one",
					"agent_definition_id":      "agent.main",
					"agent_definition_version": 1,
					"agent_definition_scope":   "project",
					"scope_identity": map[string]any{
						"project_id":    "project.one",
						"generation_id": "",
					},
					"runtime_profile_id":  "profile.main",
					"runtime_instance_id": "runtime-1",
					"is_main":             true,
					"state":               "created",
				},
				"runtime_binding": map[string]any{
					"accepted":    true,
					"profile_id":  "profile.main",
					"instance_id": "runtime-1",
				},
				"source_plan_digest":       digestB,
				"source_record_set_digest": digest,
				"team_created_at":          int64(1_721_865_600),
				"binding_digest":           digestB,
				"runtime_discovery_digest": digest,
			},
		),
		event(
			"team-planned",
			"team-execution/team-instance.one",
			1,
			"TeamExecutionPlanned",
			"team-created",
			map[string]any{
				"team_instance_id": "team-instance.one",
				"plan_digest":      digest,
				"view_version":     digestB,
				"nodes": []map[string]any{{
					"logical_node_id":     "main",
					"title":               "Main",
					"agent_instance_id":   "agent-instance.main",
					"runtime_instance_id": "runtime-1",
					"role":                "main",
					"depends_on":          []string{},
					"max_attempts":        1,
				}},
			},
		),
		event(
			"work-created",
			"work-item/work-1",
			1,
			"WorkItemCreated",
			"",
			map[string]any{
				"work_item_id": "work-1",
				"title":        "Fixture work",
				"status":       "ready",
			},
		),
		event(
			"work-assigned",
			"work-item/work-1",
			2,
			"WorkItemAssigned",
			"work-created",
			map[string]any{
				"work_item_id":      "work-1",
				"run_id":            "run-1",
				"agent_instance_id": "agent-instance.main",
				"status":            "assigned",
			},
		),
		event(
			"work-ready",
			"work-item/work-1",
			3,
			"WorkItemReadyForReview",
			"run-terminal",
			map[string]any{
				"work_item_id":     "work-1",
				"run_id":           "run-1",
				"claim_generation": 1,
				"status":           "ready_for_review",
			},
		),
		event(
			"run-claimed",
			"run/run-1",
			1,
			"RunClaimed",
			"work-assigned",
			map[string]any{
				"work_item_id":             "work-1",
				"run_id":                   "run-1",
				"claim_id":                 claimID,
				"claim_generation":         1,
				"runtime_instance_id":      "runtime-1",
				"agent_instance_id":        "agent-instance.main",
				"prepare_lease_expires_at": lease,
				"runtime_status_stream_id": "runtime_instance:runtime-1",
				"runtime_status_sequence":  1,
				"runtime_status_event_id":  "runtime-discovery",
			},
		),
		event(
			"run-started",
			"run/run-1",
			2,
			"RunStarted",
			"run-claimed",
			map[string]any{
				"work_item_id":             "work-1",
				"run_id":                   "run-1",
				"claim_id":                 claimID,
				"claim_generation":         1,
				"runtime_instance_id":      "runtime-1",
				"agent_instance_id":        "agent-instance.main",
				"runtime_status_stream_id": "runtime_instance:runtime-1",
				"runtime_status_sequence":  1,
				"runtime_status_event_id":  "runtime-discovery",
			},
		),
		event(
			"run-terminal",
			"run/run-1",
			3,
			"RunTerminalCommitted",
			"run-started",
			map[string]any{
				"work_item_id":             "work-1",
				"run_id":                   "run-1",
				"claim_id":                 claimID,
				"claim_generation":         1,
				"runtime_instance_id":      "runtime-1",
				"agent_instance_id":        "agent-instance.main",
				"status":                   "succeeded",
				"reason":                   "",
				"runtime_status_stream_id": "runtime_instance:runtime-1",
				"runtime_status_sequence":  1,
				"runtime_status_event_id":  "runtime-discovery",
			},
		),
		event(
			"capacity-reserved",
			"runtime_capacity:runtime-1",
			1,
			"RuntimeCapacityReserved",
			"run-claimed",
			map[string]any{
				"work_item_id":             "work-1",
				"run_id":                   "run-1",
				"claim_id":                 claimID,
				"claim_generation":         1,
				"runtime_instance_id":      "runtime-1",
				"agent_instance_id":        "agent-instance.main",
				"runtime_status_stream_id": "runtime_instance:runtime-1",
				"runtime_status_sequence":  1,
				"runtime_status_event_id":  "runtime-discovery",
			},
		),
		event(
			"capacity-released",
			"runtime_capacity:runtime-1",
			2,
			"RuntimeCapacityReleased",
			"run-terminal",
			map[string]any{
				"work_item_id":             "work-1",
				"run_id":                   "run-1",
				"claim_id":                 claimID,
				"claim_generation":         1,
				"runtime_instance_id":      "runtime-1",
				"agent_instance_id":        "agent-instance.main",
				"runtime_status_stream_id": "runtime_instance:runtime-1",
				"runtime_status_sequence":  1,
				"runtime_status_event_id":  "runtime-discovery",
			},
		),
		event(
			"evidence-work-created",
			"evidence-work",
			1,
			"WorkItemCreated",
			"",
			map[string]any{
				"work_item_id": "work-evidence",
				"title":        "Evidence fixture",
				"status":       "accepted",
			},
		),
		event(
			"evidence-submitted",
			"evidence-work",
			2,
			"EvidenceSubmitted",
			"evidence-work-created",
			map[string]any{
				"evidence_id":  "evidence-1",
				"work_item_id": "work-evidence",
				"digest":       digest,
			},
		),
	}
	store := journal.NewStore(database)
	for index, record := range events {
		if _, err := store.Append(context.Background(), record); err != nil {
			t.Fatalf("Append(%s) error = %v", record.ID, err)
		}
		if index == 3 || index == len(events)-2 ||
			index == len(events)-1 {
			check := projection.New(database)
			if err := check.Rebuild(context.Background()); err != nil {
				t.Fatalf(
					"fixture projection after %s: %v",
					record.ID,
					err,
				)
			}
		}
	}
}

func appendProductExecutionTeamFixture(t *testing.T, database *sql.DB) {
	t.Helper()
	const (
		digestA       = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		digestB       = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
		runtimeDigest = "aec48c95126693e8303f204b591d4af3508d2670a9f658602231b319ce988b45"
		correlation   = "22222222-2222-4222-8222-222222222222"
	)
	encode := func(value any) []byte {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	store := journal.NewStore(database)
	for _, event := range []journal.Event{
		{
			ID:       "execution-team-created",
			StreamID: "team_instance:team-execution-ready", Seq: 1,
			IdempotencyKey: "idem-execution-team-created",
			Type:           "TeamInstanceCreated", SchemaVersion: 1,
			EmittedAt:     time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
			CorrelationID: correlation,
			PayloadJSON: encode(map[string]any{
				"team": map[string]any{
					"id":                      "team-execution-ready",
					"work_request_id":         correlation,
					"source_kind":             "saved_team",
					"team_definition_id":      "team.delivery",
					"team_definition_version": 1,
					"team_definition_scope":   "project",
					"scope_identity": map[string]any{
						"project_id": "project.one", "generation_id": "",
					},
					"team_definition_digest": digestA,
					"source_plan_digest":     digestB,
					"state":                  "created", "created_at": int64(1_722_508_400),
				},
				"dormant_sub_agents":       []any{},
				"source_plan_digest":       digestB,
				"source_record_set_digest": digestA,
				"team_instance_count":      1,
				"agent_instance_count":     1,
				"active_sub_agent_count":   0,
				"work_item_count":          0,
			}),
		},
		{
			ID:       "execution-agent-created",
			StreamID: "agent_instance:execution-agent-main", Seq: 1,
			IdempotencyKey: "idem-execution-agent-created",
			Type:           "AgentInstanceCreated", SchemaVersion: 1,
			EmittedAt:     time.Date(2026, 8, 1, 10, 0, 1, 0, time.UTC),
			CorrelationID: correlation,
			CausationID:   "execution-team-created",
			PayloadJSON: encode(map[string]any{
				"main_agent": map[string]any{
					"id":                       "execution-agent-main",
					"team_instance_id":         "team-execution-ready",
					"agent_definition_id":      "loom-main-coordinator",
					"agent_definition_version": 1,
					"agent_definition_scope":   "project",
					"scope_identity": map[string]any{
						"project_id": "project.one", "generation_id": "",
					},
					"runtime_profile_id":  "loom-main-native",
					"runtime_instance_id": "runtime-1",
					"is_main":             true, "state": "created",
				},
				"runtime_binding": map[string]any{
					"accepted":    true,
					"profile_id":  "loom-main-native",
					"instance_id": "runtime-1",
				},
				"source_plan_digest":       digestB,
				"source_record_set_digest": digestA,
				"team_created_at":          int64(1_722_508_400),
				"binding_digest":           digestB,
				"runtime_discovery_digest": runtimeDigest,
			}),
		},
	} {
		if _, err := store.Append(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
}

func appendProductExecutionRuntimeFixture(t *testing.T, database *sql.DB) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"discovery_digest": "aec48c95126693e8303f204b591d4af3508d2670a9f658602231b319ce988b45",
		"source_probe_id":  "probe-1",
		"instance": map[string]any{
			"id":                    "runtime-1",
			"device_id":             "device-1",
			"adapter_type":          "pi-cli",
			"display_name":          "Local Pi",
			"executable_version":    "0.82.1",
			"status":                "online",
			"observed_capabilities": []string{"models"},
			"capacity":              1,
		},
		"model_ids": []string{"loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m"},
	})
	if err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	if _, err := store.Append(context.Background(), journal.Event{
		ID:             "execution-runtime-discovery",
		StreamID:       "runtime_instance:runtime-1",
		Seq:            1,
		IdempotencyKey: "idem-execution-runtime-discovery",
		Type:           "RuntimeInstanceDiscovered",
		SchemaVersion:  1,
		EmittedAt: time.Date(
			2026, 8, 1, 10, 0, 0, 0, time.UTC,
		),
		CorrelationID: "22222222-2222-4222-8222-222222222222",
		PayloadJSON:   payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProductMissionExecutionCompositionPreflightIsZeroWriteAndLazy(
	t *testing.T,
) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	appendProductExecutionRuntimeFixture(t, database)
	appendProductExecutionTeamFixture(t, database)
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	readService, err := api.NewLocalProductReadService(
		api.LocalProductReadConfig{
			Journal: store, Projection: readModel,
			Now: func() time.Time { return time.Now().UTC() },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	privateRoot := t.TempDir()
	executionAPI, closer, err := buildProductMissionExecutionAPI(
		context.Background(),
		store,
		readModel,
		readService,
		statePath,
		productMissionExecutionRuntimeConfig{
			RuntimeSearchPaths: []string{t.TempDir()},
			RuntimeInstanceID:  "runtime-1",
			LocalModelCatalog: &piadapter.PiLocalModelCatalogConfig{
				PrivateRoot:    privateRoot,
				ExecutablePath: filepath.Join(privateRoot, "not-started-llama"),
				ModelPath:      filepath.Join(privateRoot, "not-read-model.gguf"),
			},
			Now: func() time.Time {
				return time.Date(2026, 8, 1, 10, 5, 0, 0, time.UTC)
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()
	before, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	workPackage, err := work.CodingWorkPackage()
	if err != nil {
		t.Fatal(err)
	}
	command := app.MissionExecutionCommand{
		SchemaVersion: app.MissionExecutionSchemaVersion,
		Operation:     "preflight", MissionID: "mission/team-execution-ready",
		TeamInstanceID:      "team-execution-ready",
		WorkPackageID:       workPackage.ID(),
		WorkPackageDigest:   workPackage.Digest(),
		Objective:           "Produce one bounded verified result",
		ExpectedViewVersion: readModel.GlobalReadView().Version(),
		CorrelationID:       "33333333-3333-4333-8333-333333333333",
	}
	envelope, err := executionAPI.ExecuteMission(
		context.Background(),
		command,
	)
	if err != nil || envelope.Preflight == nil ||
		envelope.Preflight.PreflightDigest == "" {
		t.Fatalf("preflight = %#v, %v", envelope, err)
	}
	after, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("preflight wrote Journal facts: before=%d after=%d", len(before), len(after))
	}
}

func TestProductMissionExecutionVerticalLoopbackClosesAuthorizedLineage(
	t *testing.T,
) {
	root, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	defer database.Close()
	appendProductExecutionRuntimeFixture(t, database)
	appendProductExecutionTeamFixture(t, database)
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	var clockMu sync.Mutex
	clockValue := time.Date(2026, 8, 1, 10, 5, 0, 0, time.UTC)
	now := func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		current := clockValue
		clockValue = clockValue.Add(time.Millisecond)
		return current
	}
	readService, err := api.NewLocalProductReadService(
		api.LocalProductReadConfig{
			Journal: store, Projection: readModel, Now: now,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	privateRoot := t.TempDir()
	sourceStarted := make(chan struct{}, 1)
	releaseSource := make(chan struct{})
	delegate := &productPiAdapterFixture{
		verifierOutput: "criteria_satisfied",
		sourceStarted:  sourceStarted,
		releaseSource:  releaseSource,
	}
	executionAPI, closer, err := buildProductMissionExecutionAPI(
		context.Background(),
		store,
		readModel,
		readService,
		statePath,
		productMissionExecutionRuntimeConfig{
			RuntimeSearchPaths: []string{t.TempDir()},
			RuntimeInstanceID:  "runtime-1",
			LocalModelCatalog: &piadapter.PiLocalModelCatalogConfig{
				PrivateRoot:    privateRoot,
				ExecutablePath: filepath.Join(privateRoot, "not-started-llama"),
				ModelPath:      filepath.Join(privateRoot, "not-read-model.gguf"),
			},
			Now: now,
			ExecutorFactory: func(
				_ context.Context,
				workspaceRoot string,
				workAuthority *work.Authority,
				grantAuthority *authorization.Authority,
			) (productMissionExecutorPort, error) {
				adapter, factoryErr := newProductPiRuntimeAdapter(delegate)
				if factoryErr != nil {
					return nil, factoryErr
				}
				managed, factoryErr := supervisor.New(
					supervisor.Config{
						WorkspaceRoot:  workspaceRoot,
						CleanupTimeout: 5 * time.Second,
					},
					workAuthority,
					grantAuthority,
					adapter,
				)
				if factoryErr != nil {
					return nil, factoryErr
				}
				return &productMissionExecutor{supervisor: managed}, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()
	bundle, ok := closer.(*productMissionExecutionBundle)
	if !ok || bundle.handoff == nil {
		t.Fatalf("handoff composition = %#v", closer)
	}

	workPackage, err := work.CodingWorkPackage()
	if err != nil {
		t.Fatal(err)
	}
	preflightCommand := app.MissionExecutionCommand{
		SchemaVersion:       app.MissionExecutionSchemaVersion,
		Operation:           "preflight",
		MissionID:           "mission/team-execution-ready",
		TeamInstanceID:      "team-execution-ready",
		WorkPackageID:       workPackage.ID(),
		WorkPackageDigest:   workPackage.Digest(),
		Objective:           "Produce one bounded verified result",
		ExpectedViewVersion: readModel.GlobalReadView().Version(),
		CorrelationID:       "33333333-3333-4333-8333-333333333333",
	}
	preflightEnvelope, err := executionAPI.ExecuteMission(
		context.Background(),
		preflightCommand,
	)
	if err != nil || preflightEnvelope.Preflight == nil {
		t.Fatalf("preflight = %#v, %v", preflightEnvelope, err)
	}
	startCommand := preflightCommand
	startCommand.Operation = "start"
	startCommand.PreflightDigest = preflightEnvelope.Preflight.PreflightDigest
	startCommand.CorrelationID = "44444444-4444-4444-8444-444444444444"
	startEnvelope, err := executionAPI.ExecuteMission(
		context.Background(),
		startCommand,
	)
	if err != nil || startEnvelope.Result == nil ||
		startEnvelope.Result.Status != "running" {
		t.Fatalf("start = %#v, %v", startEnvelope, err)
	}
	select {
	case <-sourceStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("authorized source output was not observed")
	}
	activeTimeline, err := readService.ReadLocalProductTimeline(
		context.Background(),
		api.LocalProductTimelineRequest{
			TeamInstanceID: "team-execution-ready", Limit: 64,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	foundTentative := false
	for _, record := range activeTimeline.Records {
		if record.Kind == "node_output_delta" &&
			record.Authority == "tentative" &&
			record.Payload.TextDelta == "authorized source result" {
			foundTentative = true
		}
	}
	if !foundTentative || activeTimeline.Board.Status != "running" {
		t.Fatalf("active timeline = %#v", activeTimeline)
	}
	close(releaseSource)

	deadline := time.Now().Add(5 * time.Second)
	terminalCommitted := false
	for {
		events, readErr := store.ReadAll(context.Background())
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, event := range events {
			if event.Type == "TeamExecutionTerminal" {
				terminalCommitted = true
				break
			}
		}
		if terminalCommitted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("terminal Journal fact not committed: %v", events)
		}
		time.Sleep(5 * time.Millisecond)
	}
	terminal, err := readService.ReadLocalProductSnapshot(
		context.Background(),
		api.LocalProductSnapshotRequest{Limit: 64},
	)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Stale || len(terminal.Missions) != 1 ||
		terminal.Missions[0].Status != "succeeded" ||
		terminal.Missions[0].MissionID != startEnvelope.Result.MissionID {
		t.Fatalf("terminal snapshot = %#v", terminal)
	}
	if len(terminal.Runs) != 2 || len(terminal.Evidence) != 2 ||
		terminal.Missions[0].CompletedNodeCount != 1 {
		t.Fatalf("terminal lineage = %#v", terminal)
	}
	promotionAuthority, err := assets.NewAuthority(assets.AuthorityConfig{
		Store: store, Now: func() time.Time { return time.Now().UTC() },
		ViewVersion: func() string { return readModel.GlobalReadView().Version() },
		Promotion: &productAssetPromotionResolver{
			projection: readModel, artifacts: bundle.evidence,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	promotionService, err := app.NewLocalProductAssetService(
		readModel, promotionAuthority, bundle.evidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	promotionSnapshot, err := promotionService.ReadEvolutionAssetSnapshot(
		context.Background(), app.EvolutionAssetSnapshotRequest{
			JourneyID: "55555555-5555-4555-8555-555555555555", Limit: 64,
		},
	)
	if err != nil || len(promotionSnapshot.PromotionSources) != 1 ||
		promotionSnapshot.PromotionSources[0].RunID == "" ||
		promotionSnapshot.PromotionSources[0].RunGeneration < 1 ||
		len(promotionSnapshot.PromotionSources[0].EvidenceIDs) == 0 ||
		len(promotionSnapshot.PromotionSources[0].EvidenceIDs) != len(promotionSnapshot.PromotionSources[0].EvidenceDigests) {
		t.Fatalf("accepted promotion sources = %#v, %v", promotionSnapshot.PromotionSources, err)
	}
	terminalTimeline, err := readService.ReadLocalProductTimeline(
		context.Background(),
		api.LocalProductTimelineRequest{
			TeamInstanceID: "team-execution-ready", Limit: 64,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	evidenceRecords := 0
	terminalRecords := 0
	for _, record := range terminalTimeline.Records {
		switch record.Kind {
		case "evidence_available":
			if record.Authority == "journal" &&
				record.Payload.EvidenceDigest != "" {
				evidenceRecords++
			}
		case "team_terminal":
			if record.Authority == "journal" &&
				record.Payload.Status == "succeeded" {
				terminalRecords++
			}
		}
	}
	if evidenceRecords != 2 || terminalRecords != 1 ||
		terminalTimeline.Board.Status != "succeeded" {
		t.Fatalf("terminal timeline = %#v", terminalTimeline)
	}

	beforeReconnect, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	var plannedAt time.Time
	acceptanceTimes := make([]time.Time, 0, 4)
	for _, event := range beforeReconnect {
		counts[event.Type]++
		if event.Type == "TeamExecutionPlanned" {
			plannedAt = event.EmittedAt
		}
		switch event.Type {
		case "WorkItemVerificationCommitted", "WorkItemDone",
			"TeamNodeAcceptanceCommitted", "TeamExecutionTerminal":
			acceptanceTimes = append(acceptanceTimes, event.EmittedAt)
		}
	}
	for eventType, want := range map[string]int{
		"TeamExecutionPlanned":          1,
		"TeamReadySetDispatched":        1,
		"WorkItemCreated":               2,
		"WorkItemAssigned":              2,
		"RunStarted":                    2,
		"RunTerminalCommitted":          2,
		"AgentGrantIssued":              2,
		"EvidenceSubmitted":             2,
		"WorkItemVerificationCommitted": 1,
		"WorkItemDone":                  1,
		"TeamNodeAcceptanceCommitted":   1,
		"TeamExecutionTerminal":         1,
	} {
		if counts[eventType] != want {
			t.Fatalf("%s count = %d, want %d; all=%v", eventType, counts[eventType], want, counts)
		}
	}
	if plannedAt.IsZero() || len(acceptanceTimes) != 4 ||
		!acceptanceTimes[0].After(plannedAt) {
		t.Fatalf(
			"authority acceptance times planned=%s batch=%v",
			plannedAt,
			acceptanceTimes,
		)
	}
	for _, current := range acceptanceTimes[1:] {
		if !current.Equal(acceptanceTimes[0]) {
			t.Fatalf("acceptance batch times = %v", acceptanceTimes)
		}
	}
	if len(delegate.prompts) != 2 {
		t.Fatalf("adapter prompts = %#v", delegate.prompts)
	}
	if _, err := readService.ReadLocalProductSnapshot(
		context.Background(),
		api.LocalProductSnapshotRequest{Limit: 64},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := readService.ReadLocalProductTimeline(
		context.Background(),
		api.LocalProductTimelineRequest{
			TeamInstanceID: "team-execution-ready", Limit: 64,
		},
	); err != nil {
		t.Fatal(err)
	}
	afterReconnect, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(afterReconnect) != len(beforeReconnect) || len(delegate.prompts) != 2 {
		t.Fatalf(
			"read-only reconnect wrote events=%d prompts=%d",
			len(afterReconnect)-len(beforeReconnect),
			len(delegate.prompts),
		)
	}

	if len(terminal.Evidence) < 1 {
		t.Fatal("terminal Evidence is required for pagination fixture")
	}
	paddingStream := "evidence/" + terminal.Evidence[0].EvidenceID
	paddingEvents, err := store.ReadStream(context.Background(), paddingStream)
	if err != nil || len(paddingEvents) == 0 {
		t.Fatalf("pagination Evidence stream = %v, %v", paddingEvents, err)
	}
	lastPaddingEvent := paddingEvents[len(paddingEvents)-1]
	for index := 0; index < 70; index++ {
		sequence := lastPaddingEvent.Seq + int64(index) + 1
		if _, err := store.Append(context.Background(), journal.Event{
			ID:       fmt.Sprintf("timeline-pagination-padding-%03d", index),
			StreamID: paddingStream, Seq: sequence,
			IdempotencyKey: fmt.Sprintf("idem-timeline-pagination-padding-%03d", index),
			Type:           "TimelinePaginationFixtureObserved", SchemaVersion: 1,
			EmittedAt:     time.Date(2026, 8, 1, 11, 0, index, 0, time.UTC),
			CorrelationID: "77777777-7777-4777-8777-777777777777",
			CausationID: func() string {
				if index == 0 {
					return lastPaddingEvent.ID
				}
				return fmt.Sprintf("timeline-pagination-padding-%03d", index-1)
			}(),
			PayloadJSON: []byte(`{"team_instance_id":"team-execution-ready"}`),
		}); err != nil {
			t.Fatalf("append pagination padding %d: %v", index, err)
		}
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatalf("projection after ignored pagination fixture: %v", err)
	}
	journalHeads := func(events []journal.Event) []journal.StreamHead {
		byStream := make(map[string]journal.StreamHead)
		for _, event := range events {
			byStream[event.StreamID] = journal.StreamHead{
				StreamID: event.StreamID,
				Sequence: event.Seq,
				EventID:  event.ID,
			}
		}
		heads := make([]journal.StreamHead, 0, len(byStream))
		for _, head := range byStream {
			heads = append(heads, head)
		}
		sort.Slice(heads, func(i, j int) bool {
			return heads[i].StreamID < heads[j].StreamID
		})
		return heads
	}
	beforeProbeEvents, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("read Journal before authoritative Swift Store probe: %v", err)
	}
	beforeProbeHeads := journalHeads(beforeProbeEvents)
	firstAuthoritativePage, err := readService.ReadLocalProductTimeline(
		context.Background(),
		api.LocalProductTimelineRequest{
			TeamInstanceID: "team-execution-ready", Limit: 64,
		},
	)
	if err != nil || !firstAuthoritativePage.HasMore ||
		len(firstAuthoritativePage.NextCursor) <= 256 {
		t.Fatalf("authoritative first page = %#v, %v", firstAuthoritativePage, err)
	}
	secondAuthoritativePage, err := readService.ReadLocalProductTimeline(
		context.Background(),
		api.LocalProductTimelineRequest{
			TeamInstanceID: "team-execution-ready",
			Cursor:         firstAuthoritativePage.NextCursor,
			Limit:          64,
		},
	)
	if err != nil || secondAuthoritativePage.HasMore || secondAuthoritativePage.Gap != nil {
		t.Fatalf("authoritative second page = %#v, %v", secondAuthoritativePage, err)
	}
	expectedDeliveryIDs := make([]string, 0,
		len(firstAuthoritativePage.Records)+len(secondAuthoritativePage.Records))
	for _, page := range []api.LocalProductTimelinePage{
		firstAuthoritativePage, secondAuthoritativePage,
	} {
		for _, record := range page.Records {
			expectedDeliveryIDs = append(expectedDeliveryIDs, record.DeliveryID)
		}
	}

	baseHandler := localProductHandler(readService)
	var cursorMu sync.Mutex
	var requestedCursors []string
	handler := localipc.HandlerFunc(func(
		ctx context.Context,
		request localipc.Request,
	) localipc.Response {
		if request.Method == "timeline_page" {
			var input api.LocalProductTimelineRequest
			if err := json.Unmarshal(request.Params, &input); err == nil {
				cursorMu.Lock()
				requestedCursors = append(requestedCursors, input.Cursor)
				cursorMu.Unlock()
			}
		}
		return baseHandler(ctx, request)
	})
	socketRoot, err := os.MkdirTemp("/private/tmp", "loom-pg-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(socketRoot)
	if err := os.Chmod(socketRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(socketRoot, "loomd.sock")
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(),
		BuildID: "authoritative-swift-store-pagination", Handler: handler,
	})
	if err != nil {
		t.Fatal(err)
	}
	serverContext, stopServer := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(serverContext) }()
	select {
	case <-server.Ready():
	case err := <-serverDone:
		t.Fatalf("pagination server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("pagination server did not become ready")
	}
	probe := buildProductDaemonSwiftContractProbe(t, root)
	probeCommand := exec.Command(
		probe,
		"--socket",
		socketPath,
		"--team-all",
		"team-execution-ready",
	)
	probeCommand.Env = productCLITestEnvironment(t, root)
	probeOutput, probeErr := probeCommand.CombinedOutput()
	stopServer()
	_ = server.Close()
	if serveErr := <-serverDone; serveErr != nil {
		t.Fatalf("pagination server close: %v", serveErr)
	}
	if probeErr != nil {
		t.Fatalf("authoritative Swift Store probe = %v, %q", probeErr, probeOutput)
	}
	var strictOutput struct {
		Timeline api.LocalProductTimelinePage `json:"timeline"`
	}
	if err := json.Unmarshal(probeOutput, &strictOutput); err != nil {
		t.Fatalf("authoritative Swift Store output = %v, %q", err, probeOutput)
	}
	cursorMu.Lock()
	cursors := append([]string(nil), requestedCursors...)
	cursorMu.Unlock()
	strictEvidence := 0
	strictTerminal := 0
	strictDeliveryIDs := make([]string, 0, len(strictOutput.Timeline.Records))
	uniqueDeliveryIDs := make(map[string]struct{}, len(strictOutput.Timeline.Records))
	for _, record := range strictOutput.Timeline.Records {
		strictDeliveryIDs = append(strictDeliveryIDs, record.DeliveryID)
		uniqueDeliveryIDs[record.DeliveryID] = struct{}{}
		switch record.Kind {
		case "evidence_available":
			strictEvidence++
		case "team_terminal":
			strictTerminal++
		}
	}
	if strictOutput.Timeline.HasMore || strictOutput.Timeline.Gap != nil ||
		strictEvidence != 2 || strictTerminal != 1 ||
		!reflect.DeepEqual(strictDeliveryIDs, expectedDeliveryIDs) ||
		len(uniqueDeliveryIDs) != len(strictDeliveryIDs) ||
		len(cursors) != 2 || cursors[0] != "" ||
		cursors[1] != firstAuthoritativePage.NextCursor ||
		len(cursors[1]) <= 256 {
		t.Fatalf(
			"strict timeline=%#v cursors=%#v first=%q",
			strictOutput.Timeline,
			cursors,
			firstAuthoritativePage.NextCursor,
		)
	}
	afterProbeEvents, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("read Journal after authoritative Swift Store probe: %v", err)
	}
	if !reflect.DeepEqual(afterProbeEvents, beforeProbeEvents) ||
		!reflect.DeepEqual(journalHeads(afterProbeEvents), beforeProbeHeads) {
		t.Fatalf(
			"authoritative Swift Store probe mutated Journal: events %d -> %d, heads %#v -> %#v",
			len(beforeProbeEvents), len(afterProbeEvents),
			beforeProbeHeads, journalHeads(afterProbeEvents),
		)
	}
	currentSnapshot, err := readService.ReadLocalProductSnapshot(context.Background(), api.LocalProductSnapshotRequest{Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	parentRun := currentSnapshot.Runs[0]
	proposalRequest := app.SideTaskProposalRequest{SchemaVersion: 1, Operation: "propose", ParentMissionID: "mission/team-execution-ready", ParentTeamInstanceID: "team-execution-ready", ParentTaskID: parentRun.WorkItemID, ParentRunID: parentRun.RunID, ParentClaimGeneration: parentRun.ClaimGeneration, ParentExecutionDigest: startEnvelope.Result.ExecutionDigest, Purpose: "verification", Mode: "report_only", Title: "Verify the completed Mission", AuthorizedRequest: "Return one bounded authorized verification result", PermissionScopes: []string{}, DecisionTimeoutSeconds: 0, ExpectedViewVersion: currentSnapshot.ViewVersion, CorrelationID: "88888888-8888-4888-8888-888888888888"}
	beforeBadParentDigest, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	badParentDigest := proposalRequest
	badParentDigest.ParentExecutionDigest = strings.Repeat("f", 64)
	if _, err = bundle.handoff.ProposeSideTask(context.Background(), badParentDigest); !errors.Is(err, app.ErrSideTaskProductDigestMismatch) {
		t.Fatalf("bad parent execution proposal error = %v", err)
	}
	badProposalBytes, err := json.Marshal(badParentDigest)
	if err != nil {
		t.Fatal(err)
	}
	badProposalSum := sha256.Sum256(badProposalBytes)
	badParentDigest.Operation = "create"
	if _, err = bundle.handoff.CreateSideTask(context.Background(), app.SideTaskCreateRequest{
		SideTaskProposalRequest: badParentDigest,
		ProposalDigest:          hex.EncodeToString(badProposalSum[:]),
		Confirmed:               true,
	}); !errors.Is(err, app.ErrSideTaskProductDigestMismatch) {
		t.Fatalf("bad parent execution create error = %v", err)
	}
	afterBadParentDigest, err := store.ReadAll(context.Background())
	if err != nil || len(afterBadParentDigest) != len(beforeBadParentDigest) {
		t.Fatalf("bad parent execution digest wrote Events: %d -> %d err=%v",
			len(beforeBadParentDigest), len(afterBadParentDigest), err)
	}
	proposal, err := bundle.handoff.ProposeSideTask(context.Background(), proposalRequest)
	if err != nil || !proposal.RequiresConfirmation || proposal.PolicyAvailable {
		t.Fatalf("proposal=%#v err=%v", proposal, err)
	}
	proposalRequest.Operation = "create"
	created, err := bundle.handoff.CreateSideTask(context.Background(), app.SideTaskCreateRequest{SideTaskProposalRequest: proposalRequest, ProposalDigest: proposal.ProposalDigest, Confirmed: true})
	if err != nil || created.Status != "report_delivered" || created.SideExecutionTeamInstanceID == "team-execution-ready" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	withSideTask, err := readService.ReadLocalProductSnapshot(context.Background(), api.LocalProductSnapshotRequest{Limit: 64})
	if err != nil || len(withSideTask.SideTasks) != 1 || withSideTask.SideTasks[0].ParentMissionID != "mission/team-execution-ready" || len(withSideTask.Missions) != 1 || withSideTask.Missions[0].Status != "succeeded" {
		t.Fatalf("side-task snapshot=%#v err=%v", withSideTask, err)
	}
	decisionProposal := proposalRequest
	decisionProposal.Operation = "propose"
	decisionProposal.Mode = "decision_required"
	decisionProposal.Title = "Decide how to use verified context"
	decisionProposal.DecisionTimeoutSeconds = 60
	decisionProposal.ExpectedViewVersion = withSideTask.ViewVersion
	decisionProposal.CorrelationID = "99999999-9999-4999-8999-999999999999"
	proposedDecision, err := bundle.handoff.ProposeSideTask(context.Background(), decisionProposal)
	if err != nil {
		t.Fatal(err)
	}
	decisionProposal.Operation = "create"
	createdDecision, err := bundle.handoff.CreateSideTask(context.Background(), app.SideTaskCreateRequest{SideTaskProposalRequest: decisionProposal, ProposalDigest: proposedDecision.ProposalDigest, Confirmed: true})
	if err != nil || createdDecision.Status != "decision_required" {
		t.Fatalf("decision handoff=%#v err=%v", createdDecision, err)
	}
	decisionSnapshot, err := readService.ReadLocalProductSnapshot(context.Background(), api.LocalProductSnapshotRequest{Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	var pending api.LocalProductSideTaskSummary
	for _, sideTask := range decisionSnapshot.SideTasks {
		if sideTask.SideTaskID == createdDecision.SideTaskID {
			pending = sideTask
		}
	}
	effectBytes := sha256.Sum256([]byte(strings.Join([]string{
		"parent-effect-v1", pending.SideTaskID, pending.ParentTeamInstanceID,
		pending.ParentTaskID, pending.ParentRunID,
		fmt.Sprint(pending.ParentClaimGeneration), startEnvelope.Result.ExecutionDigest,
		pending.HandoffDigest, "absorb",
	}, "\x00")))
	beforeStaleDigest, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.handoff.DecideSideTask(context.Background(), app.SideTaskDecisionRequest{SchemaVersion: 1, Operation: "decide", SideTaskID: pending.SideTaskID, ParentMissionID: pending.ParentMissionID, ParentTeamInstanceID: pending.ParentTeamInstanceID, ParentTaskID: pending.ParentTaskID, ParentRunID: pending.ParentRunID, ParentLogicalNodeID: "main", ParentAttemptNumber: 1, ParentClaimGeneration: pending.ParentClaimGeneration, ParentExecutionDigest: strings.Repeat("f", 64), SideTaskGeneration: pending.SourceGeneration, HandoffVersion: pending.HandoffVersion, HandoffDigest: pending.HandoffDigest, Decision: "absorb", EffectDigest: hex.EncodeToString(effectBytes[:]), ExpectedViewVersion: decisionSnapshot.ViewVersion, CorrelationID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}); !errors.Is(err, app.ErrSideTaskProductDigestMismatch) {
		t.Fatalf("stale parent execution digest error = %v", err)
	}
	afterStaleDigest, err := store.ReadAll(context.Background())
	if err != nil || len(afterStaleDigest) != len(beforeStaleDigest) {
		t.Fatalf("stale digest wrote Events: %d -> %d err=%v", len(beforeStaleDigest), len(afterStaleDigest), err)
	}
	decisionResult, err := bundle.handoff.DecideSideTask(context.Background(), app.SideTaskDecisionRequest{SchemaVersion: 1, Operation: "decide", SideTaskID: pending.SideTaskID, ParentMissionID: pending.ParentMissionID, ParentTeamInstanceID: pending.ParentTeamInstanceID, ParentTaskID: pending.ParentTaskID, ParentRunID: pending.ParentRunID, ParentLogicalNodeID: "main", ParentAttemptNumber: 1, ParentClaimGeneration: pending.ParentClaimGeneration, ParentExecutionDigest: startEnvelope.Result.ExecutionDigest, SideTaskGeneration: pending.SourceGeneration, HandoffVersion: pending.HandoffVersion, HandoffDigest: pending.HandoffDigest, Decision: "absorb", EffectDigest: hex.EncodeToString(effectBytes[:]), ExpectedViewVersion: decisionSnapshot.ViewVersion, CorrelationID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"})
	if err != nil || decisionResult.EffectStatus != "completed" || decisionResult.ContextPacketDigest == "" {
		t.Fatalf("absorb=%#v err=%v", decisionResult, err)
	}
	afterDecision, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	packets, continuations, completed := 0, 0, 0
	for _, event := range afterDecision {
		switch event.Type {
		case "ContextPacketCommitted":
			packets++
		case "ParentContinuationAuthorized":
			continuations++
		case "ParentHandoffEffectCompleted":
			completed++
		}
	}
	if packets != 1 || continuations != 1 || completed != 1 {
		t.Fatalf("parent effects packet=%d continuation=%d completed=%d", packets, continuations, completed)
	}

	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	recoveryProposalRequest := app.SideTaskProposalRequest{
		SchemaVersion: 1, Operation: "propose",
		ParentMissionID:      decisionProposal.ParentMissionID,
		ParentTeamInstanceID: decisionProposal.ParentTeamInstanceID,
		ParentTaskID:         decisionProposal.ParentTaskID, ParentRunID: decisionProposal.ParentRunID,
		ParentClaimGeneration: decisionProposal.ParentClaimGeneration,
		ParentExecutionDigest: decisionProposal.ParentExecutionDigest,
		Purpose:               "diagnosis", Mode: "decision_required", Title: "Recover admitted child",
		AuthorizedRequest: "Return the exact recovered child result", PermissionScopes: []string{},
		DecisionTimeoutSeconds: 900, ExpectedViewVersion: readModel.GlobalReadView().Version(),
		CorrelationID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
	}
	recoveryProposal, err := bundle.handoff.ProposeSideTask(context.Background(), recoveryProposalRequest)
	if err != nil {
		t.Fatal(err)
	}
	deterministicID := func(parts ...string) string {
		sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
		encoded := hex.EncodeToString(sum[:16])
		return encoded[:8] + "-" + encoded[8:12] + "-4" + encoded[13:16] + "-8" + encoded[17:20] + "-" + encoded[20:32]
	}
	recoverySideTaskID := deterministicID("side-task", recoveryProposalRequest.ParentMissionID, recoveryProposal.ProposalDigest)
	recoveryInput := app.SideTaskInputArtifact{
		SchemaVersion: 2, SideTaskID: recoverySideTaskID,
		ParentMissionID: recoveryProposalRequest.ParentMissionID,
		ParentTaskID:    recoveryProposalRequest.ParentTaskID, ParentRunID: recoveryProposalRequest.ParentRunID,
		ParentClaimGeneration: recoveryProposalRequest.ParentClaimGeneration,
		ParentExecutionDigest: recoveryProposalRequest.ParentExecutionDigest,
		Purpose:               recoveryProposalRequest.Purpose, Mode: recoveryProposalRequest.Mode,
		Title: recoveryProposalRequest.Title, AuthorizedRequest: recoveryProposalRequest.AuthorizedRequest,
		PermissionScopes: []string{}, ProposalDigest: recoveryProposal.ProposalDigest,
		DecisionTimeoutSeconds: 900, CreatedAt: now().Format(time.RFC3339Nano),
	}
	recoveryInputBytes, err := json.Marshal(recoveryInput)
	if err != nil {
		t.Fatal(err)
	}
	recoveryInputHash := sha256.Sum256(recoveryInputBytes)
	recoveryInputDigest := hex.EncodeToString(recoveryInputHash[:])
	if _, err := bundle.evidence.Publish(context.Background(), bytes.NewReader(recoveryInputBytes), recoveryInputDigest); err != nil {
		t.Fatal(err)
	}
	recoveryChildID := deterministicID("side-execution", recoverySideTaskID, recoveryProposal.ProposalDigest, recoveryInputDigest)
	if _, err := bundle.workAuthority.AdmitSideTask(context.Background(), work.SideTaskAdmissionInput{
		SideTaskID: recoverySideTaskID, ParentMissionID: recoveryProposalRequest.ParentMissionID,
		ParentTeamInstanceID: recoveryProposalRequest.ParentTeamInstanceID,
		ParentTaskID:         recoveryProposalRequest.ParentTaskID, ParentRunID: recoveryProposalRequest.ParentRunID,
		ParentClaimGeneration:       recoveryProposalRequest.ParentClaimGeneration,
		ParentExecutionDigest:       recoveryProposalRequest.ParentExecutionDigest,
		SideExecutionTeamInstanceID: recoveryChildID,
		Purpose:                     recoveryProposalRequest.Purpose, Mode: recoveryProposalRequest.Mode,
		Title: recoveryProposalRequest.Title, ProposalDigest: recoveryProposal.ProposalDigest,
		InputArtifactDigest: recoveryInputDigest, ExpectedViewVersion: recoveryProposalRequest.ExpectedViewVersion,
		PermissionScopes: []string{}, Confirmed: true, CorrelationID: recoveryProposalRequest.CorrelationID,
	}); err != nil {
		t.Fatal(err)
	}
	promptsBeforeRecovery := len(delegate.prompts)
	if err := bundle.handoffService.ReconcileSideTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	eventsAfterRecovery, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	promptsAfterRecovery := len(delegate.prompts)
	if promptsAfterRecovery <= promptsBeforeRecovery {
		t.Fatalf("admitted recovery did not execute child: prompts %d -> %d", promptsBeforeRecovery, promptsAfterRecovery)
	}
	if err := bundle.handoffService.ReconcileSideTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	eventsAfterReplay, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(eventsAfterReplay) != len(eventsAfterRecovery) || len(delegate.prompts) != promptsAfterRecovery {
		t.Fatalf("recovered terminal redispatched: events %d -> %d prompts %d -> %d", len(eventsAfterRecovery), len(eventsAfterReplay), promptsAfterRecovery, len(delegate.prompts))
	}
	timeoutBound := false
	for _, event := range eventsAfterRecovery {
		if event.StreamID != "side-task/"+recoverySideTaskID || event.Type != "SideTaskHandoffCommitted" {
			continue
		}
		var payload struct {
			DecisionTimeoutSeconds int64 `json:"decision_timeout_seconds"`
		}
		if err := json.Unmarshal(event.PayloadJSON, &payload); err == nil && payload.DecisionTimeoutSeconds == 900 {
			timeoutBound = true
		}
	}
	if !timeoutBound {
		t.Fatal("recovered handoff did not preserve the exact 900 second timeout")
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	strictReadViewVersion := readModel.GlobalReadView().Version()

	handoffServer, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(),
		BuildID: "side-task-strict-swift-read",
		Handler: localipc.HandlerFunc(localProductHandlerWithComposition(
			readService, nil, nil, executionAPI, bundle.handoff, nil, nil, nil, nil, nil, nil, nil, nil,
		)),
	})
	if err != nil {
		t.Fatal(err)
	}
	handoffContext, stopHandoffServer := context.WithCancel(context.Background())
	handoffDone := make(chan error, 1)
	go func() { handoffDone <- handoffServer.Serve(handoffContext) }()
	select {
	case <-handoffServer.Ready():
	case err := <-handoffDone:
		t.Fatalf("handoff Swift server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("handoff Swift server did not become ready")
	}
	strictRead := exec.Command(
		probe, "--socket", socketPath, "--side-task-read",
		pending.SideTaskID, strictReadViewVersion,
	)
	strictRead.Env = productCLITestEnvironment(t, root)
	strictReadOutput, strictReadErr := strictRead.CombinedOutput()
	stopHandoffServer()
	_ = handoffServer.Close()
	if serveErr := <-handoffDone; serveErr != nil {
		t.Fatalf("handoff Swift server close: %v", serveErr)
	}
	if strictReadErr != nil {
		t.Fatalf("strict Swift Side-task read = %v, %q", strictReadErr, strictReadOutput)
	}
	var strictSideTask struct {
		SideTaskID         string   `json:"sideTaskID"`
		Status             string   `json:"status"`
		ViewVersion        string   `json:"viewVersion"`
		AvailableDecisions []string `json:"availableDecisions"`
	}
	if err := json.Unmarshal(strictReadOutput, &strictSideTask); err != nil ||
		strictSideTask.SideTaskID != pending.SideTaskID ||
		strictSideTask.Status != "decided" ||
		strictSideTask.ViewVersion != strictReadViewVersion ||
		strictSideTask.AvailableDecisions == nil {
		t.Fatalf("strict Swift Side-task output=%q decoded=%#v err=%v", strictReadOutput, strictSideTask, err)
	}

	legacyProposalRequest := recoveryProposalRequest
	legacyProposalRequest.Title = "Reject legacy restart input"
	legacyProposalRequest.AuthorizedRequest = "This legacy input must not dispatch"
	legacyProposalRequest.ExpectedViewVersion = strictReadViewVersion
	legacyProposalRequest.CorrelationID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	legacyProposal, err := bundle.handoff.ProposeSideTask(context.Background(), legacyProposalRequest)
	if err != nil {
		t.Fatal(err)
	}
	legacySideTaskID := deterministicID("side-task", legacyProposalRequest.ParentMissionID, legacyProposal.ProposalDigest)
	legacyInput := struct {
		SchemaVersion         int      `json:"schema_version"`
		SideTaskID            string   `json:"side_task_id"`
		ParentMissionID       string   `json:"parent_mission_id"`
		ParentTaskID          string   `json:"parent_task_id"`
		ParentRunID           string   `json:"parent_run_id"`
		ParentClaimGeneration int64    `json:"parent_claim_generation"`
		Purpose               string   `json:"purpose"`
		Mode                  string   `json:"mode"`
		Title                 string   `json:"title"`
		AuthorizedRequest     string   `json:"authorized_request"`
		PermissionScopes      []string `json:"permission_scopes"`
		ProposalDigest        string   `json:"proposal_digest"`
		CreatedAt             string   `json:"created_at"`
	}{1, legacySideTaskID, legacyProposalRequest.ParentMissionID,
		legacyProposalRequest.ParentTaskID, legacyProposalRequest.ParentRunID,
		legacyProposalRequest.ParentClaimGeneration, legacyProposalRequest.Purpose,
		legacyProposalRequest.Mode, legacyProposalRequest.Title,
		legacyProposalRequest.AuthorizedRequest, []string{}, legacyProposal.ProposalDigest,
		now().Format(time.RFC3339Nano)}
	legacyBytes, err := json.Marshal(legacyInput)
	if err != nil {
		t.Fatal(err)
	}
	legacyHash := sha256.Sum256(legacyBytes)
	legacyDigest := hex.EncodeToString(legacyHash[:])
	if _, err := bundle.evidence.Publish(context.Background(), bytes.NewReader(legacyBytes), legacyDigest); err != nil {
		t.Fatal(err)
	}
	legacyChildID := deterministicID("side-execution", legacySideTaskID, legacyProposal.ProposalDigest, legacyDigest)
	if _, err := bundle.workAuthority.AdmitSideTask(context.Background(), work.SideTaskAdmissionInput{
		SideTaskID: legacySideTaskID, ParentMissionID: legacyProposalRequest.ParentMissionID,
		ParentTeamInstanceID: legacyProposalRequest.ParentTeamInstanceID,
		ParentTaskID:         legacyProposalRequest.ParentTaskID, ParentRunID: legacyProposalRequest.ParentRunID,
		ParentClaimGeneration:       legacyProposalRequest.ParentClaimGeneration,
		ParentExecutionDigest:       legacyProposalRequest.ParentExecutionDigest,
		SideExecutionTeamInstanceID: legacyChildID, Purpose: legacyProposalRequest.Purpose,
		Mode: legacyProposalRequest.Mode, Title: legacyProposalRequest.Title,
		ProposalDigest: legacyProposal.ProposalDigest, InputArtifactDigest: legacyDigest,
		ExpectedViewVersion: legacyProposalRequest.ExpectedViewVersion,
		PermissionScopes:    []string{}, Confirmed: true, CorrelationID: legacyProposalRequest.CorrelationID,
	}); err != nil {
		t.Fatal(err)
	}
	legacyPrompts := len(delegate.prompts)
	if err := bundle.handoffService.ReconcileSideTasks(context.Background()); !errors.Is(err, app.ErrSideTaskProductUnavailable) {
		t.Fatalf("legacy recovery error=%v", err)
	}
	if len(delegate.prompts) != legacyPrompts {
		t.Fatal("legacy input dispatched a child")
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	legacyRecord, ok := readModel.GlobalReadView().SideTaskHandoff(legacySideTaskID)
	if !ok || legacyRecord.Status != "admitted" || legacyRecord.HandoffVersion != 0 {
		t.Fatalf("legacy durable state=%#v found=%v", legacyRecord, ok)
	}
}

func TestProductMissionExecutionCompositionReconcilesExactJournalLineageBeforeIPC(
	t *testing.T,
) {
	now := time.Date(2026, 8, 1, 10, 5, 0, 0, time.UTC)
	build := func(workflowPath string) (*productDaemonRunner, error) {
		t.Helper()
		root, statePath := productDaemonFailureState(t)
		database, err := sql.Open("sqlite", statePath)
		if err != nil {
			t.Fatal(err)
		}
		appendProductExecutionRuntimeFixture(t, database)
		appendProductExecutionTeamFixture(t, database)
		appendProductExecutionInFlightFixture(t, database, now, workflowPath)
		if err := database.Close(); err != nil {
			t.Fatal(err)
		}
		privateRoot := filepath.Join(root, "local-model")
		if err := os.Mkdir(privateRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		return newProductDaemonRunner(
			&blockingObserverRunner{},
			statePath,
			filepath.Join(root, "loomd.sock"),
			productSetupRuntimeConfig{
				Execution: &productMissionExecutionRuntimeConfig{
					RuntimeSearchPaths: []string{root},
					RuntimeInstanceID:  "runtime-1",
					LocalModelCatalog: &piadapter.PiLocalModelCatalogConfig{
						PrivateRoot: privateRoot,
						ExecutablePath: filepath.Join(
							privateRoot,
							"not-started-llama",
						),
						ModelPath: filepath.Join(
							privateRoot,
							"not-read-model.gguf",
						),
					},
					Now: func() time.Time { return now },
				},
			},
		)
	}

	runner, err := build("builtin/mission-primary-v1")
	if err != nil {
		t.Fatalf("exact Journal lineage did not reconcile: %v", err)
	}
	if err := runner.Close(); err != nil {
		t.Fatalf("close reconciled daemon: %v", err)
	}

	unknown, err := build("unknown/mission-recipe")
	if unknown != nil {
		_ = unknown.Close()
	}
	if !errors.Is(err, app.ErrMissionExecutionConflict) {
		t.Fatalf("unknown Journal recipe error = %v", err)
	}
}

func appendProductExecutionInFlightFixture(
	t *testing.T,
	database *sql.DB,
	now time.Time,
	workflowPath string,
) {
	t.Helper()
	store := journal.NewStore(database)
	authority, err := work.NewAuthority(
		store,
		func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x42}, 4096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := authority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-execution-ready",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID:     "main",
			Title:             "Produce one bounded verified result",
			AgentInstanceID:   "execution-agent-main",
			RuntimeInstanceID: "runtime-1",
			Role:              teams.ExecutionRoleMain,
			DependsOn:         []string{},
			MaxAttempts:       2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	outputContract, err := verification.NewOutputContract(
		1,
		verification.EmptyOutputTransient,
	)
	if err != nil {
		t.Fatal(err)
	}
	recoveryPolicy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version: 1, RetryDelay: 0, AttemptCredits: 1,
		ExhaustionAction: rules.ExhaustionBlocked,
		RetryInvalid:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	acceptanceContract, err := verification.NewAcceptanceContract(
		1,
		[]string{
			"authorized output is non-empty",
			"result satisfies the confirmed Mission objective",
		},
		verification.AcceptanceRiskMedium,
	)
	if err != nil {
		t.Fatal(err)
	}
	verifierAgentID := productVerifierIdentity(
		"mission-verifier-agent",
		plan.TeamInstanceID(),
		plan.Digest(),
	)
	view := readModel.GlobalReadView()
	workItemID := productMissionAttemptIdentity("work", plan, "main", 1)
	runID := productMissionAttemptIdentity("run", plan, "main", 1)
	streamIDs := []string{
		"team-execution/" + plan.TeamInstanceID(),
		"work-run-identity/v1",
		"work-item/" + workItemID,
		"run/" + runID,
		"runtime_instance:runtime-1",
		"runtime_capacity:runtime-1",
	}
	sort.Strings(streamIDs)
	heads := make([]journal.StreamHead, len(streamIDs))
	for index, streamID := range streamIDs {
		head, ok := view.Head(streamID)
		if !ok {
			head = journal.StreamHead{StreamID: streamID}
		}
		heads[index] = head
	}
	_, err = authority.DispatchTeamReadySet(ctx, work.TeamDispatchInput{
		Plan: plan,
		ReadyAttempts: []work.TeamAttemptSelection{{
			LogicalNodeID: "main", AttemptNumber: 1,
		}},
		SemanticBindings: []work.TeamNodeSemanticBinding{{
			LogicalNodeID:               "main",
			OutputContractVersion:       outputContract.Version(),
			OutputContractDigest:        outputContract.Digest(),
			RecoveryPolicyVersion:       recoveryPolicy.Version(),
			RecoveryPolicyDigest:        recoveryPolicy.Digest(),
			AttemptCredits:              recoveryPolicy.AttemptCredits(),
			PrimaryWorkflowPath:         workflowPath,
			WorkflowFallbackKey:         recoveryPolicy.WorkflowFallbackKey(),
			RecoveryApprovalRequired:    recoveryPolicy.RecoveryApprovalRequired(),
			AcceptanceContractVersion:   acceptanceContract.Version(),
			AcceptanceContractDigest:    acceptanceContract.Digest(),
			AcceptanceRisk:              string(acceptanceContract.Risk()),
			IndependentVerifierRequired: acceptanceContract.IndependentVerifierRequired(),
			VerifierAgentInstanceID:     verifierAgentID,
			VerifierRuntimeInstanceID:   "runtime-1",
			VerifierWorkflowPath:        "builtin/mission-verifier-v1",
		}},
		ViewVersion:          view.Version(),
		ExpectedHeads:        heads,
		AuthoritativeTime:    now,
		PrepareLeaseDuration: time.Minute,
		CorrelationID:        "33333333-3333-4333-8333-333333333333",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	projected, ok := readModel.GlobalReadView().TeamExecution(plan.TeamInstanceID())
	if !ok || projected.Status != "running" || projected.PlanDigest != plan.Digest() {
		t.Fatalf("in-flight fixture projection = %#v", projected)
	}
}

func productMissionAttemptIdentity(
	label string,
	plan teams.ExecutionPlan,
	logicalNodeID string,
	attemptNumber int,
) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		label,
		plan.TeamInstanceID(),
		plan.Digest(),
		logicalNodeID,
		fmt.Sprint(attemptNumber),
	}, "\x00")))
	return "team-" + label + "-" + hex.EncodeToString(digest[:16])
}

func productVerifierIdentity(label string, fields ...string) string {
	hash := sha256.New()
	write := func(value string) {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(value))
	}
	write("loom." + label + ".v1")
	for _, field := range fields {
		write(field)
	}
	return label + "-" + hex.EncodeToString(hash.Sum(nil))
}

type productPiAdapterFixture struct {
	verifierOutput string
	prompts        []string
	sourceStarted  chan<- struct{}
	releaseSource  <-chan struct{}
}

func (*productPiAdapterFixture) AdapterType() string { return "pi-cli" }

func (*productPiAdapterFixture) RuntimeInstanceID() string { return "runtime-1" }

func (fixture *productPiAdapterFixture) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	var dispatch struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Prompt        string `json:"prompt"`
	}
	if err := json.Unmarshal(request.Dispatch.Payload(), &dispatch); err != nil ||
		dispatch.SchemaVersion != 1 || dispatch.Kind != "pi_rpc_prompt" {
		return supervisor.AdapterResult{}, errors.New("invalid fixture dispatch")
	}
	fixture.prompts = append(fixture.prompts, dispatch.Prompt)
	output := "authorized source result"
	if strings.Contains(dispatch.Prompt, "allowed verifier reason code") {
		output = fixture.verifierOutput
	}
	frames := make([]bridgev1.Frame, 0, 4)
	for index, record := range []struct {
		typeName bridgev1.MessageType
		payload  any
	}{
		{bridgev1.MessageAck, map[string]string{
			"message_id": request.Dispatch.MessageID(),
		}},
		{bridgev1.MessageEvent, map[string]string{"delta": output}},
		{bridgev1.MessageEvidence, map[string]any{
			"kind": "assistant_text_digest", "sha256": strings.Repeat("a", 64),
			"bytes": len(output),
		}},
		{bridgev1.MessageResult, struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}{"succeeded", ""}},
	} {
		payload, err := json.Marshal(record.payload)
		if err != nil {
			return supervisor.AdapterResult{}, err
		}
		frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
			MessageID: fmt.Sprintf(
				"10000000-0000-4000-8000-%012d", index+2,
			),
			CorrelationID:         request.Dispatch.CorrelationID(),
			WorkItemID:            request.Binding.WorkItemID,
			RunID:                 request.Binding.RunID,
			ClaimGeneration:       request.Binding.ClaimGeneration,
			RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
			SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
			Sequence:              int64(index + 2),
			Type:                  record.typeName,
			EmittedAt:             request.Dispatch.EmittedAt(),
			Payload:               payload,
		})
		if err != nil {
			return supervisor.AdapterResult{}, err
		}
		if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
			return supervisor.AdapterResult{}, err
		}
		if record.typeName == bridgev1.MessageEvent &&
			!strings.Contains(dispatch.Prompt, "allowed verifier reason code") &&
			fixture.releaseSource != nil {
			if fixture.sourceStarted != nil {
				select {
				case fixture.sourceStarted <- struct{}{}:
				default:
				}
			}
			select {
			case <-fixture.releaseSource:
			case <-ctx.Done():
				return supervisor.AdapterResult{}, ctx.Err()
			}
		}
		frames = append(frames, frame)
	}
	return supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames: frames, ExitCode: 0,
		DispatchAcknowledged: true, ResultAcknowledged: true,
	})
}

type productRecordingFrameSink struct {
	frames []bridgev1.Frame
	err    error
}

func (sink *productRecordingFrameSink) AcceptFrame(
	_ context.Context,
	frame bridgev1.Frame,
) error {
	if sink.err != nil {
		return sink.err
	}
	sink.frames = append(sink.frames, frame)
	return nil
}

func productAdapterRequest(
	t *testing.T,
	runID string,
	payload any,
	sink supervisor.FrameSink,
) supervisor.AdapterRequest {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             "20000000-0000-4000-8000-000000000001",
		CorrelationID:         "33333333-3333-4333-8333-333333333333",
		WorkItemID:            "work-" + runID,
		RunID:                 runID,
		ClaimGeneration:       1,
		RuntimeInstanceID:     "runtime-1",
		SenderAgentInstanceID: "agent-1",
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt: time.Date(
			2026, 8, 1, 10, 0, 0, 0, time.UTC,
		),
		Payload: body,
	})
	if err != nil {
		t.Fatal(err)
	}
	return supervisor.AdapterRequest{
		Binding: bridgev1.RunStreamBinding{
			WorkItemID:            dispatch.WorkItemID(),
			RunID:                 dispatch.RunID(),
			ClaimGeneration:       dispatch.ClaimGeneration(),
			RuntimeInstanceID:     dispatch.RuntimeInstanceID(),
			SenderAgentInstanceID: dispatch.SenderAgentInstanceID(),
		},
		Dispatch:  dispatch,
		FrameSink: sink,
	}
}

func productPromptFixture(prompt string) productPiPromptDispatch {
	return productPiPromptDispatch{
		SchemaVersion: 1,
		Kind:          "pi_rpc_prompt",
		Prompt:        prompt,
	}
}

func productVerifierFixture(sourceRunID string) productPiVerifierDispatch {
	return productPiVerifierDispatch{
		TeamInstanceID:       "team-1",
		PlanDigest:           strings.Repeat("a", 64),
		LogicalNodeID:        "main",
		SourceAttemptNumber:  1,
		SourceWorkItemID:     "work-" + sourceRunID,
		SourceRunID:          sourceRunID,
		SourceEvidenceDigest: strings.Repeat("b", 64),
		SourceOutputSummaryDigest: strings.Repeat(
			"c", 64,
		),
		AcceptanceContractDigest: strings.Repeat("d", 64),
		Risk:                     "medium",
		Criteria:                 []string{"bounded result is present"},
		AllowedReasonCodes: []string{
			"criteria_satisfied",
			"criteria_not_satisfied",
			"insufficient_evidence",
		},
	}
}

func TestProductPiRuntimeAdapterAuthorizesSourceBeforeIndependentVerifier(
	t *testing.T,
) {
	delegate := &productPiAdapterFixture{verifierOutput: "criteria_satisfied"}
	adapter, err := newProductPiRuntimeAdapter(delegate)
	if err != nil {
		t.Fatal(err)
	}
	sourceSink := &productRecordingFrameSink{}
	source := productAdapterRequest(
		t, "source-run", productPromptFixture("bounded source task"), sourceSink,
	)
	if _, err := adapter.Execute(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	verifierSink := &productRecordingFrameSink{}
	verifier := productAdapterRequest(
		t, "verifier-run", productVerifierFixture("source-run"), verifierSink,
	)
	result, err := adapter.Execute(context.Background(), verifier)
	if err != nil {
		t.Fatal(err)
	}
	if len(delegate.prompts) != 2 ||
		!strings.Contains(delegate.prompts[1], "authorized source result") ||
		len(verifierSink.frames) != 4 ||
		len(result.InboundFrames()) != 4 {
		t.Fatalf(
			"prompts=%#v frames=%d result=%d",
			delegate.prompts,
			len(verifierSink.frames),
			len(result.InboundFrames()),
		)
	}
	terminal := verifierSink.frames[3]
	if terminal.Type() != bridgev1.MessageResult ||
		string(terminal.Payload()) != `{"reason":"","status":"succeeded"}` {
		t.Fatalf("verifier terminal = %s %s", terminal.Type(), terminal.Payload())
	}
}

func TestProductPiRuntimeAdapterRejectsUnacceptedSourceAndFailsClosedVerifier(
	t *testing.T,
) {
	delegate := &productPiAdapterFixture{verifierOutput: "unexpected prose"}
	adapter, err := newProductPiRuntimeAdapter(delegate)
	if err != nil {
		t.Fatal(err)
	}
	rejected := errors.New("frame rejected before authorization")
	request := productAdapterRequest(
		t,
		"source-run",
		productPromptFixture("bounded source task"),
		&productRecordingFrameSink{err: rejected},
	)
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(err, rejected) {
		t.Fatalf("source rejection = %v", err)
	}
	verifier := productAdapterRequest(
		t,
		"verifier-run",
		productVerifierFixture("source-run"),
		&productRecordingFrameSink{},
	)
	if _, err := adapter.Execute(context.Background(), verifier); err == nil {
		t.Fatal("verifier used output rejected by authoritative sink")
	}

	accepted := productAdapterRequest(
		t,
		"source-two",
		productPromptFixture("bounded source task"),
		&productRecordingFrameSink{},
	)
	if _, err := adapter.Execute(context.Background(), accepted); err != nil {
		t.Fatal(err)
	}
	verifier = productAdapterRequest(
		t,
		"verifier-two",
		productVerifierFixture("source-two"),
		&productRecordingFrameSink{},
	)
	result, err := adapter.Execute(context.Background(), verifier)
	if err != nil {
		t.Fatal(err)
	}
	terminal := result.InboundFrames()[3]
	if string(terminal.Payload()) !=
		`{"reason":"insufficient_evidence","status":"failed"}` {
		t.Fatalf("fail-closed terminal = %s", terminal.Payload())
	}
}

func TestProductDaemonProductionRunnerServesAuthoritativeMissionPreflight(
	t *testing.T,
) {
	root, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	appendProductExecutionRuntimeFixture(t, database)
	appendProductExecutionTeamFixture(t, database)
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	privateRoot := filepath.Join(root, "local-model")
	if err := os.Mkdir(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	runner, err := newProductDaemonRunner(
		&blockingObserverRunner{},
		statePath,
		filepath.Join(root, "loomd.sock"),
		productSetupRuntimeConfig{Execution: &productMissionExecutionRuntimeConfig{
			RuntimeSearchPaths: []string{root},
			RuntimeInstanceID:  "runtime-1",
			LocalModelCatalog: &piadapter.PiLocalModelCatalogConfig{
				PrivateRoot:    privateRoot,
				ExecutablePath: filepath.Join(privateRoot, "not-started-llama"),
				ModelPath:      filepath.Join(privateRoot, "not-read-model.gguf"),
			},
			Now: func() time.Time {
				return time.Date(2026, 8, 1, 10, 5, 0, 0, time.UTC)
			},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		done <- runErr
	}()
	socketPath := filepath.Join(root, "loomd.sock")
	waitForProductSocket(t, socketPath)
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot api.LocalProductSnapshot
	if err := client.Call(
		context.Background(),
		"snapshot",
		api.LocalProductSnapshotRequest{Limit: 64},
		&snapshot,
	); err != nil {
		t.Fatal(err)
	}
	workPackage, err := work.CodingWorkPackage()
	if err != nil {
		t.Fatal(err)
	}
	var envelope api.MissionExecutionEnvelope
	if err := client.Call(
		context.Background(),
		"mission_execution",
		app.MissionExecutionCommand{
			SchemaVersion: app.MissionExecutionSchemaVersion,
			Operation:     "preflight", MissionID: "mission/team-execution-ready",
			TeamInstanceID:      "team-execution-ready",
			WorkPackageID:       workPackage.ID(),
			WorkPackageDigest:   workPackage.Digest(),
			Objective:           "Produce one bounded verified result",
			ExpectedViewVersion: snapshot.ViewVersion,
			CorrelationID:       "33333333-3333-4333-8333-333333333333",
		},
		&envelope,
	); err != nil {
		t.Fatal(err)
	}
	if envelope.Operation != "preflight" || envelope.Preflight == nil ||
		envelope.Preflight.PreflightDigest == "" {
		t.Fatalf("preflight envelope = %#v", envelope)
	}
	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("product daemon stop error = %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("product daemon did not stop")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
}

func productDaemonHeadsDigest(t *testing.T, database *sql.DB) string {
	t.Helper()
	rows, err := database.Query(
		`SELECT stream_id, seq, id
		   FROM events
		  WHERE (stream_id, seq) IN (
		        SELECT stream_id, MAX(seq) FROM events GROUP BY stream_id
		  )
		  ORDER BY stream_id`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	hasher := sha256.New()
	for rows.Next() {
		var streamID, eventID string
		var sequence int64
		if err := rows.Scan(&streamID, &sequence, &eventID); err != nil {
			t.Fatal(err)
		}
		_, _ = fmt.Fprintf(
			hasher,
			"%s\x00%d\x00%s\n",
			streamID,
			sequence,
			eventID,
		)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

type productSetupFixtureBackend struct {
	closed            *bool
	connectErr        error
	credentialCommand *app.CredentialSetupCommand
}

func (backend productSetupFixtureBackend) ConnectCodex(
	context.Context,
) (app.ProviderConnectResult, error) {
	if backend.connectErr != nil {
		return app.ProviderConnectResult{}, backend.connectErr
	}
	return app.ProviderConnectResult{
		ProviderID: "codex",
		AuthMode:   "native_auth",
		Status:     "started",
	}, nil
}

func (productSetupFixtureBackend) SetupSnapshot(
	context.Context,
) (app.SetupSnapshot, error) {
	return app.SetupSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("a", 64),
		Codex: app.ProviderSetupStatus{
			ProviderID: "codex",
			AuthMode:   "native_auth",
			Status:     "available",
		},
		MiniMax: app.ProviderSetupStatus{
			ProviderID: "minimax",
			AuthMode:   "brokered",
			Status:     "unconfigured",
		},
		Runtimes:    []app.SetupRuntimePreview{},
		SavedTeams:  []app.SetupSavedTeamPreview{},
		Templates:   []app.SetupTeamTemplatePreview{},
		RoleOptions: []app.SetupRoleOptionPreview{},
		Skills:      []app.SetupSkillRevision{},
		Permissions: []string{},
		Resources:   []app.SetupResourcePointer{},
	}, nil
}

func (productSetupFixtureBackend) StartBuilder(
	context.Context,
	app.BuilderStartCommand,
) (app.BuilderSessionView, error) {
	return app.BuilderSessionView{}, errors.New("unexpected builder start")
}

func (backend productSetupFixtureBackend) VerifyCredential(
	_ context.Context,
	command app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	if backend.credentialCommand == nil {
		return app.CredentialSetupResult{}, errors.New("unexpected credential verify")
	}
	*backend.credentialCommand = command
	return app.CredentialSetupResult{
		ProviderID: "minimax", Revision: command.ExpectedRevision + 1,
		Status: "verified",
	}, nil
}

func (backend productSetupFixtureBackend) Close() error {
	if backend.closed != nil {
		*backend.closed = true
	}
	return nil
}

type productCodexLoginFixture struct {
	status provider.CodexLoginStatus
	err    error
	closed bool
}

func (fixture *productCodexLoginFixture) Start(
	context.Context,
) (provider.CodexLoginStartResult, error) {
	return provider.CodexLoginStartResult{
		Status: fixture.status,
	}, fixture.err
}

func (fixture *productCodexLoginFixture) Close() error {
	fixture.closed = true
	return nil
}

func TestProductNativeAuthConnectorMapsOnlyClosedErrors(t *testing.T) {
	tests := []struct {
		name   string
		status provider.CodexLoginStatus
		err    error
		want   error
	}{
		{
			name:   "busy",
			status: provider.CodexLoginStarted,
			err:    provider.ErrCodexLoginBusy,
			want:   app.ErrNativeAuthConnectBusy,
		},
		{
			name:   "unavailable",
			status: provider.CodexLoginStarted,
			err:    provider.ErrCodexLoginUnavailable,
			want:   app.ErrNativeAuthConnectUnavailable,
		},
		{
			name:   "identity_changed",
			status: provider.CodexLoginStarted,
			err:    provider.ErrCodexExecutableIdentityChanged,
			want:   app.ErrNativeAuthConnectUnavailable,
		},
		{
			name: "invalid_success_status",
			want: app.ErrNativeAuthConnectUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := &productCodexLoginFixture{
				status: test.status,
				err:    test.err,
			}
			connector := productNativeAuthConnector{
				controller: fixture,
			}
			if err := connector.StartNativeAuth(
				context.Background(),
			); !errors.Is(err, test.want) {
				t.Fatalf("StartNativeAuth() error = %v", err)
			}
			if err := connector.Close(); err != nil {
				t.Fatal(err)
			}
			if !fixture.closed {
				t.Fatal("Close() did not reach controller")
			}
		})
	}
}

func TestProductDaemonClosePropagatesToSetupProcessOwner(t *testing.T) {
	root, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	setup, err := api.NewLocalProductSetupAPI(productSetupFixtureBackend{
		closed: &closed,
	})
	if err != nil {
		t.Fatal(err)
	}
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath:   filepath.Join(root, "loomd.sock"),
		EffectiveUID: os.Geteuid(),
		BuildID:      "close-fixture",
		Handler: localipc.HandlerFunc(
			localProductHandler(nil, setup),
		),
	})
	if err != nil {
		t.Fatal(err)
	}
	observer := &blockingObserverRunner{}
	runner := &productDaemonRunner{
		observer: observer,
		server:   server,
		database: database,
		setup:    setup,
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	if !closed || !observer.closed {
		t.Fatalf(
			"Close() setup=%t observer=%t",
			closed,
			observer.closed,
		)
	}
}

type productLifecycleCloser struct {
	name    string
	order   *[]string
	entered chan struct{}
	release <-chan struct{}
	err     error
}

func (closer *productLifecycleCloser) Close() error {
	if closer.order != nil {
		*closer.order = append(*closer.order, closer.name)
	}
	if closer.entered != nil {
		close(closer.entered)
	}
	if closer.release != nil {
		<-closer.release
	}
	return closer.err
}

type productLifecycleServer struct {
	*productLifecycleCloser
	ready chan struct{}
}

func (server *productLifecycleServer) Serve(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (server *productLifecycleServer) Ready() <-chan struct{} {
	return server.ready
}

type productLifecycleObserver struct {
	*productLifecycleCloser
	canceled chan struct{}
}

func (observer *productLifecycleObserver) Run(
	ctx context.Context,
) (app.LocalRuntimeObservationDaemonResult, error) {
	<-ctx.Done()
	if observer.canceled != nil {
		close(observer.canceled)
	}
	return app.LocalRuntimeObservationDaemonResult{}, ctx.Err()
}

type cancellationProcessFailureObserver struct {
	*productLifecycleCloser
	started chan struct{}
}

type cancellationCleanServer struct {
	*productLifecycleCloser
	ready chan struct{}
}

func (server *cancellationCleanServer) Serve(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (server *cancellationCleanServer) Ready() <-chan struct{} {
	return server.ready
}

func (observer *cancellationProcessFailureObserver) Run(
	ctx context.Context,
) (app.LocalRuntimeObservationDaemonResult, error) {
	close(observer.started)
	<-ctx.Done()
	return app.LocalRuntimeObservationDaemonResult{},
		errors.New("metadata subprocess terminated after cancellation")
}

func TestProductDaemonCancellationOwnsObserverFailureProducedAfterCancel(
	t *testing.T,
) {
	server := &cancellationCleanServer{
		productLifecycleCloser: &productLifecycleCloser{},
		ready:                  make(chan struct{}),
	}
	close(server.ready)
	runner := &productDaemonRunner{
		server: server,
		observer: &cancellationProcessFailureObserver{
			productLifecycleCloser: &productLifecycleCloser{},
			started:                make(chan struct{}),
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		done <- runErr
	}()
	select {
	case <-runner.observer.(*cancellationProcessFailureObserver).started:
	case <-time.After(time.Second):
		t.Fatal("observer did not start")
	}
	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("cancellation-owned observer failure = %v", runErr)
		}
	case <-time.After(time.Second):
		t.Fatal("product daemon did not stop")
	}
}

func TestP3AControlledJourneyHarnessWritesSanitizedRequestAndDaemonLogs(
	t *testing.T,
) {
	if !productJourneyPathWithin("/journey/root", "/journey/root") {
		t.Fatal("evidence root equal to journey root must be within the root")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	harness, err := newProductJourneyHarness(
		productJourneyHarnessManifest{
			SchemaVersion: 1,
			Purpose:       "phase3a-cross-client-e2e",
			JourneyID:     "123e4567-e89b-42d3-a456-426614174000",
			EvidenceRoot:  root,
			FaultKind:     "none",
			FaultAction:   "none",
			DelayMillis:   0,
		},
		func(int) { t.Fatal("unexpected termination") },
	)
	if err != nil {
		t.Fatal(err)
	}
	eventIDs := []string{"event-1", "event-2"}
	encoded, err := json.Marshal(struct {
		EventIDs []string `json:"event_ids"`
	}{eventIDs})
	if err != nil {
		t.Fatal(err)
	}
	handler := harness.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		return localipc.Response{OK: true, Result: encoded}
	}))
	params := json.RawMessage(`{"operation_id":"op-1","action":"activate","secret":"must-not-log"}`)
	response := handler.Handle(context.Background(), localipc.Request{
		Version: 1, RequestID: "loom-swift-request-1",
		JourneyID: "123e4567-e89b-42d3-a456-426614174000",
		Method:    "evolution_asset_command", Params: params,
	})
	if !response.OK {
		t.Fatalf("response = %#v", response)
	}
	if err := harness.Close(); err != nil {
		t.Fatal(err)
	}
	ipcBytes, err := os.ReadFile(filepath.Join(root, "ipc", "request-response-summary.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	daemonBytes, err := os.ReadFile(filepath.Join(root, "daemon", "structured-log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string][]byte{"ipc": ipcBytes, "daemon": daemonBytes} {
		if bytes.Contains(contents, []byte("must-not-log")) ||
			bytes.Contains(contents, []byte("secret")) {
			t.Fatalf("%s log disclosed request params: %s", name, contents)
		}
	}
	var ipcRecord productJourneyIPCRecord
	if err := json.Unmarshal(bytes.TrimSpace(ipcBytes), &ipcRecord); err != nil {
		t.Fatal(err)
	}
	if ipcRecord.Sequence != 1 || ipcRecord.ClientKind != "gui" ||
		ipcRecord.RequestID != "loom-swift-request-1" ||
		ipcRecord.JourneyID != "123e4567-e89b-42d3-a456-426614174000" ||
		ipcRecord.Method != "evolution_asset_command" ||
		ipcRecord.Action != "activate" || !ipcRecord.ResponseOK ||
		ipcRecord.ErrorCode != "" || len(ipcRecord.RequestDigest) != 64 ||
		len(ipcRecord.ResponseDigest) != 64 {
		t.Fatalf("IPC record = %#v", ipcRecord)
	}
	lines := bytes.Split(bytes.TrimSpace(daemonBytes), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("daemon log lines = %d: %s", len(lines), daemonBytes)
	}
	var completed productJourneyDaemonRecord
	if err := json.Unmarshal(lines[1], &completed); err != nil {
		t.Fatal(err)
	}
	if completed.Phase != "response" || completed.Outcome != "pass" ||
		!reflect.DeepEqual(completed.AuthorityEventIDs, eventIDs) {
		t.Fatalf("completed daemon record = %#v", completed)
	}
	reopened, err := newProductJourneyHarness(
		productJourneyHarnessManifest{
			SchemaVersion: 1, Purpose: "phase3a-cross-client-e2e",
			JourneyID:    "123e4567-e89b-42d3-a456-426614174000",
			EvidenceRoot: root, FaultKind: "none", FaultAction: "none",
		},
		func(int) { t.Fatal("unexpected termination") },
	)
	if err != nil {
		t.Fatal(err)
	}
	reopened.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		return localipc.Response{OK: true, Result: json.RawMessage(`{}`)}
	})).Handle(context.Background(), localipc.Request{
		Version: 1, RequestID: "loom-client-1",
		JourneyID: "123e4567-e89b-42d3-a456-426614174000",
		Method:    "evolution_asset_snapshot", Params: json.RawMessage(`{}`),
	})
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	ipcBytes, err = os.ReadFile(filepath.Join(root, "ipc", "request-response-summary.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	ipcLines := bytes.Split(bytes.TrimSpace(ipcBytes), []byte("\n"))
	if len(ipcLines) != 2 {
		t.Fatalf("restarted IPC lines = %d: %s", len(ipcLines), ipcBytes)
	}
	var restarted productJourneyIPCRecord
	if err := json.Unmarshal(ipcLines[1], &restarted); err != nil ||
		restarted.Sequence != 2 || restarted.ClientKind != "tui" {
		t.Fatalf("restarted IPC record = %#v, %v", restarted, err)
	}
}

func TestP3AProductJourneyIsolationRootRecoveryRemovesStaleProbeRoots(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "state")
	isolation := filepath.Join(root, "isolation")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(isolation, 0o700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(state, "loom.db")
	if err := os.WriteFile(statePath, []byte("journey"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(isolation, "loom-pi-metadata-123")
	conformance := filepath.Join(isolation, "loom-pi-skill-conformance-456")
	unrelated := filepath.Join(isolation, "loom-pi-other")
	for _, dir := range []string{stale, conformance, unrelated} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := recoverProductJourneyIsolationRoot(root); err != nil {
		t.Fatalf("recovery error = %v", err)
	}
	if _, err := os.Lstat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale metadata root not removed: %v", err)
	}
	if _, err := os.Lstat(conformance); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale conformance root not removed: %v", err)
	}
	if _, err := os.Lstat(unrelated); err != nil {
		t.Fatalf("unrelated isolation dir removed: %v", err)
	}
}

func TestP3AProductJourneyIsolationRootRecoveryMissingIsolationIsNoop(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := recoverProductJourneyIsolationRoot(root); err != nil {
		t.Fatalf("missing isolation dir should be a no-op, got %v", err)
	}
}

func TestP3AControlledJourneyHarnessFaultsAreOneShotAndPhaseExact(
	t *testing.T,
) {
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	newHarness := func(kind string, terminate func(int)) *productJourneyHarness {
		t.Helper()
		root := t.TempDir()
		if err := os.Chmod(root, 0o700); err != nil {
			t.Fatal(err)
		}
		harness, err := newProductJourneyHarness(
			productJourneyHarnessManifest{
				SchemaVersion: 1, Purpose: "phase3a-cross-client-e2e",
				JourneyID: journeyID, EvidenceRoot: root,
				FaultKind: kind, FaultAction: "activate",
			},
			terminate,
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = harness.Close() })
		return harness
	}

	projection := newHarness("projection_failure", func(int) {
		t.Fatal("projection fault terminated process")
	})
	rebuilds := 0
	refresh := projection.projectionRefresh(func(context.Context, string) error {
		rebuilds++
		return nil
	})
	if err := refresh(context.Background(), "activate"); !errors.Is(err, errProductJourneyProjectionFailure) {
		t.Fatalf("first projection refresh error = %v", err)
	}
	if err := refresh(context.Background(), "activate"); err != nil || rebuilds != 1 {
		t.Fatalf("second projection refresh = %v, rebuilds=%d", err, rebuilds)
	}

	request := localipc.Request{
		Version: 1, RequestID: "loom-client-1", JourneyID: journeyID,
		Method: "evolution_asset_command",
		Params: json.RawMessage(`{"operation_id":"op-1","action":"activate"}`),
	}
	beforeTerminations, beforeCalls := 0, 0
	before := newHarness("crash_before_cas", func(code int) {
		if code != productJourneyCrashBeforeCASExitCode {
			t.Fatalf("before-CAS exit code = %d", code)
		}
		beforeTerminations++
	})
	beforeHandler := before.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		beforeCalls++
		return localipc.Response{OK: true, Result: json.RawMessage(`{}`)}
	}))
	if response := beforeHandler.Handle(context.Background(), request); response.OK || beforeCalls != 0 || beforeTerminations != 1 {
		t.Fatalf("before-CAS response=%#v calls=%d terminations=%d", response, beforeCalls, beforeTerminations)
	}
	if response := beforeHandler.Handle(context.Background(), request); !response.OK || beforeCalls != 1 || beforeTerminations != 1 {
		t.Fatalf("before-CAS redelivery=%#v calls=%d terminations=%d", response, beforeCalls, beforeTerminations)
	}

	afterTerminations, afterCalls := 0, 0
	after := newHarness("crash_after_cas_before_response", func(code int) {
		if code != productJourneyCrashAfterCASExitCode {
			t.Fatalf("after-CAS exit code = %d", code)
		}
		afterTerminations++
	})
	afterHandler := after.wrap(localipc.HandlerFunc(func(
		context.Context,
		localipc.Request,
	) localipc.Response {
		afterCalls++
		return localipc.Response{OK: true, Result: json.RawMessage(`{"event_ids":["event-1"]}`)}
	}))
	if response := afterHandler.Handle(context.Background(), request); response.OK || afterCalls != 1 || afterTerminations != 1 {
		t.Fatalf("after-CAS response=%#v calls=%d terminations=%d", response, afterCalls, afterTerminations)
	}
	if response := afterHandler.Handle(context.Background(), request); !response.OK || afterCalls != 2 || afterTerminations != 1 {
		t.Fatalf("after-CAS redelivery=%#v calls=%d terminations=%d", response, afterCalls, afterTerminations)
	}
}

func TestP3AControlledJourneyManifestIsPrivateExactAndBoundToDaemonPaths(
	t *testing.T,
) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	manifestRoot := filepath.Join(root, "manifest")
	evidenceRoot := filepath.Join(root, "journey-evidence")
	stateRoot := filepath.Join(root, "state")
	for _, path := range []string{manifestRoot, evidenceRoot, stateRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	statePath := filepath.Join(stateRoot, "loom.db")
	if err := os.WriteFile(statePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(root, "loomd.sock")
	manifestPath := filepath.Join(manifestRoot, "journey-harness.json")
	manifest := productJourneyHarnessManifest{
		SchemaVersion: 1, Purpose: "phase3a-cross-client-e2e",
		JourneyID: "123e4567-e89b-42d3-a456-426614174000",
		StatePath: statePath, SocketPath: socketPath, EvidenceRoot: evidenceRoot,
		FaultKind: "none", FaultAction: "none", DelayMillis: 0,
	}
	writeManifest := func(value any) {
		t.Helper()
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifestPath, encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(manifestPath, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeManifest(manifest)
	t.Setenv(controlledProductJourneyManifestEnvironment, manifestPath)
	harness, err := controlledProductJourneyHarnessFromEnvironment(
		statePath,
		socketPath,
	)
	if err != nil || harness == nil {
		t.Fatalf("valid controlled journey manifest = %v, %v", harness, err)
	}
	if err := harness.Close(); err != nil {
		t.Fatal(err)
	}

	manifest.SocketPath = filepath.Join(root, "other.sock")
	writeManifest(manifest)
	if harness, err := controlledProductJourneyHarnessFromEnvironment(
		statePath,
		socketPath,
	); err == nil || harness != nil {
		t.Fatalf("mismatched socket manifest accepted: %v, %v", harness, err)
	}

	manifest.SocketPath = socketPath
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var unknown map[string]any
	if err := json.Unmarshal(encoded, &unknown); err != nil {
		t.Fatal(err)
	}
	unknown["unknown"] = true
	writeManifest(unknown)
	if harness, err := controlledProductJourneyHarnessFromEnvironment(
		statePath,
		socketPath,
	); err == nil || harness != nil {
		t.Fatalf("unknown manifest field accepted: %v, %v", harness, err)
	}
}

func TestP3AProductionRunnerWiresControlledJourneyAuditOverRealSocket(
	t *testing.T,
) {
	root, statePath := productDaemonFailureState(t)
	socketPath := filepath.Join(root, "loomd.sock")
	manifestRoot := filepath.Join(root, "manifest")
	evidenceRoot := filepath.Join(root, "journey-evidence")
	for _, path := range []string{manifestRoot, evidenceRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	manifestPath := filepath.Join(manifestRoot, "journey-harness.json")
	encoded, err := json.Marshal(productJourneyHarnessManifest{
		SchemaVersion: 1, Purpose: "phase3a-cross-client-e2e",
		JourneyID: "123e4567-e89b-42d3-a456-426614174000",
		StatePath: statePath, SocketPath: socketPath, EvidenceRoot: evidenceRoot,
		FaultKind: "none", FaultAction: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(controlledProductJourneyManifestEnvironment, manifestPath)
	runner, err := newProductDaemonRunner(
		&blockingObserverRunner{},
		statePath,
		socketPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		done <- runErr
	}()
	waitForProductSocket(t, socketPath)
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot api.EvolutionAssetSnapshot
	if err := client.CallJourney(
		context.Background(),
		"123e4567-e89b-42d3-a456-426614174000",
		"evolution_asset_snapshot",
		api.EvolutionAssetSnapshotRequest{Limit: 64},
		&snapshot,
	); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("runner stop error = %v", runErr)
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not stop")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	ipcBytes, err := os.ReadFile(filepath.Join(
		evidenceRoot,
		"ipc",
		"request-response-summary.jsonl",
	))
	if err != nil || !bytes.Contains(ipcBytes, []byte(`"method":"evolution_asset_snapshot"`)) ||
		!bytes.Contains(ipcBytes, []byte(`"client_kind":"tui"`)) {
		t.Fatalf("production journey audit = %s, %v", ipcBytes, err)
	}
}

func TestProductDaemonCloseIsOrderedJoinedAndExactlyOnce(t *testing.T) {
	order := []string{}
	setupEntered := make(chan struct{})
	setupRelease := make(chan struct{})
	runner := &productDaemonRunner{
		server: &productLifecycleServer{
			productLifecycleCloser: &productLifecycleCloser{
				name:  "local_ipc",
				order: &order,
			},
			ready: make(chan struct{}),
		},
		observer: &productLifecycleObserver{
			productLifecycleCloser: &productLifecycleCloser{
				name:  "observer",
				order: &order,
			},
		},
		setup: &productLifecycleCloser{
			name:    "setup",
			order:   &order,
			entered: setupEntered,
			release: setupRelease,
		},
		database: &productLifecycleCloser{
			name:  "database",
			order: &order,
		},
	}
	done := make(chan error, 1)
	go func() { done <- runner.Close() }()
	select {
	case <-setupEntered:
	case <-time.After(time.Second):
		t.Fatal("Close() did not reach setup stage")
	}
	if !reflect.DeepEqual(order, []string{
		"local_ipc",
		"observer",
		"setup",
	}) {
		t.Fatalf("order before setup release = %v", order)
	}
	select {
	case err := <-done:
		t.Fatalf("Close() detached blocked setup: %v", err)
	default:
	}
	close(setupRelease)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{
		"local_ipc",
		"observer",
		"setup",
		"database",
	}) {
		t.Fatalf("close order = %v", order)
	}
	if err := runner.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("shutdown owners closed more than once: %v", order)
	}
}

func TestProductDaemonCancellationJoinsHandlerAndObserver(t *testing.T) {
	root, _ := productDaemonFailureState(t)
	socketPath := filepath.Join(root, "loomd.sock")
	handlerEntered := make(chan struct{})
	handlerCanceled := make(chan struct{})
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "product-lifecycle-fixture",
		Handler: localipc.HandlerFunc(func(
			ctx context.Context,
			_ localipc.Request,
		) localipc.Response {
			close(handlerEntered)
			<-ctx.Done()
			close(handlerCanceled)
			return localipc.Response{OK: false}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	observerCanceled := make(chan struct{})
	runner := &productDaemonRunner{
		server: server,
		observer: &productLifecycleObserver{
			productLifecycleCloser: &productLifecycleCloser{},
			canceled:               observerCanceled,
		},
		setup:    &productLifecycleCloser{},
		database: &productLifecycleCloser{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		runDone <- runErr
	}()
	waitForProductSocket(t, socketPath)
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	callDone := make(chan error, 1)
	go func() {
		var result map[string]any
		callDone <- client.Call(
			context.Background(),
			"snapshot",
			struct{}{},
			&result,
		)
	}()
	select {
	case <-handlerEntered:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	cancel()
	for name, signal := range map[string]<-chan struct{}{
		"handler":  handlerCanceled,
		"observer": observerCanceled,
	} {
		select {
		case <-signal:
		case <-time.After(time.Second):
			t.Fatalf("%s was not canceled", name)
		}
	}
	select {
	case err := <-runDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not join handler and observer")
	}
	select {
	case <-callDone:
	case <-time.After(time.Second):
		t.Fatal("client remained blocked after cancellation")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("socket remains after cancellation: %v", err)
	}
}

func TestProductDaemonCloseFailsClosedWhenSetupOwnerIsMissing(t *testing.T) {
	root, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath:   filepath.Join(root, "loomd.sock"),
		EffectiveUID: os.Geteuid(),
		BuildID:      "missing-setup-fixture",
		Handler: localipc.HandlerFunc(
			localProductHandler(nil),
		),
	})
	if err != nil {
		t.Fatal(err)
	}
	observer := &blockingObserverRunner{}
	runner := &productDaemonRunner{
		observer: observer,
		server:   server,
		database: database,
	}
	var shutdownErr *productShutdownError
	if err := runner.Close(); !errors.As(err, &shutdownErr) ||
		shutdownErr.stage != "setup" {
		t.Fatalf("Close() error = %v", err)
	}
	if !observer.closed {
		t.Fatal("Close() did not continue closing the observer")
	}
}

func TestProductSetupRefreshesRuntimeCatalogAfterServiceConstruction(
	t *testing.T,
) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	for revision := int64(0); revision < 4; revision++ {
		status := credentials.CredentialVerified
		if revision == 0 {
			status = credentials.CredentialConfigured
		}
		_, err := writer.CommitCredentialMetadata(
			context.Background(),
			credentials.MetadataCommand{
				CommandID: fmt.Sprintf(
					"attempt-004-credential-%d",
					revision+1,
				),
				ProviderID:          "minimax",
				CredentialReference: "credential-ref-attempt-004",
				ExpectedRevision:    revision,
				OccurredAt:          time.Unix(100+revision, 0).UTC(),
				Status:              status,
				Reason:              credentials.VerificationReasonNone,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	appendProductSetupRuntimeEvent(
		t,
		store,
		"runtime-legacy",
		"legacy-discovery",
		nil,
	)
	if events, err := store.ReadAll(context.Background()); err != nil ||
		len(events) != 5 {
		t.Fatalf("pre-construction fixture events=%d error=%v", len(events), err)
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	setup, err := buildProductSetupService(
		database,
		store,
		readModel,
		productSetupRuntimeConfig{
			CredentialStore: &productCredentialTestStore{},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if setup == nil {
		t.Fatal("setup service unavailable")
	}
	defer setup.Close()

	appendProductSetupRuntimeEvent(
		t,
		store,
		"runtime-model-capable",
		"model-discovery",
		[]string{"loom-local/model-a"},
	)
	if events, err := store.ReadAll(context.Background()); err != nil ||
		len(events) != 6 {
		t.Fatalf("attempt-004 fixture events=%d error=%v", len(events), err)
	}
	snapshot, err := setup.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatalf("SetupSnapshot() error = %v", err)
	}
	if len(snapshot.Runtimes) != 2 ||
		len(snapshot.RoleOptions) != 2 ||
		snapshot.RoleOptions[0].RuntimeInstanceID !=
			"runtime-model-capable" ||
		snapshot.RoleOptions[1].RuntimeInstanceID !=
			"runtime-model-capable" {
		t.Fatalf("refreshed snapshot = %#v", snapshot)
	}
	session, err := setup.StartBuilder(
		context.Background(),
		app.BuilderStartCommand{Source: app.BuilderSourceBlank},
	)
	if err != nil {
		t.Fatalf("StartBuilder() error = %v", err)
	}
	if session.Question.ID != "team_name" ||
		session.CatalogDigest == "" ||
		session.ViewVersion != snapshot.ViewVersion {
		t.Fatalf("builder session = %#v, snapshot = %#v", session, snapshot)
	}
}

func appendProductSetupRuntimeEvent(
	t *testing.T,
	store *journal.Store,
	runtimeID,
	eventID string,
	modelIDs []string,
) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"discovery_digest": strings.Repeat(
			map[bool]string{true: "a", false: "b"}[modelIDs == nil],
			64,
		),
		"source_probe_id": "probe-" + runtimeID,
		"instance": map[string]any{
			"id":                    runtimeID,
			"device_id":             "device-local",
			"adapter_type":          "pi",
			"display_name":          "Local Pi",
			"executable_version":    "0.82.1",
			"status":                "online",
			"observed_capabilities": []string{"rpc"},
			"capacity":              1,
		},
		"model_ids": modelIDs,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Append(context.Background(), journal.Event{
		ID:             eventID,
		StreamID:       "runtime_instance:" + runtimeID,
		Seq:            1,
		IdempotencyKey: "idem-" + eventID,
		Type:           "RuntimeInstanceDiscovered",
		SchemaVersion:  1,
		EmittedAt:      time.Unix(10, 0).UTC(),
		CorrelationID:  "correlation-" + runtimeID,
		PayloadJSON:    payload,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestProductDaemonServesStrictSetupSnapshotWithoutCLIOrSQLiteClient(
	t *testing.T,
) {
	unavailable := localProductHandler(nil)(
		context.Background(),
		localipc.Request{
			Version:   1,
			RequestID: "setup-unavailable",
			Method:    "setup_snapshot",
			Params:    json.RawMessage(`{}`),
		},
	)
	if unavailable.OK ||
		unavailable.Error == nil ||
		unavailable.Error.Code != "state_unavailable" {
		t.Fatalf("missing setup service response = %#v", unavailable)
	}
	setup, err := api.NewLocalProductSetupAPI(productSetupFixtureBackend{})
	if err != nil {
		t.Fatalf("NewLocalProductSetupAPI() error = %v", err)
	}
	handler := localProductHandler(nil, setup)
	response := handler(context.Background(), localipc.Request{
		Version:   1,
		RequestID: "setup-request-1",
		Method:    "setup_snapshot",
		Params:    json.RawMessage(`{}`),
	})
	if !response.OK || response.Error != nil {
		t.Fatalf("setup snapshot response = %#v", response)
	}
	var snapshot app.SetupSnapshot
	if err := json.Unmarshal(response.Result, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Codex.AuthMode != "native_auth" ||
		snapshot.MiniMax.AuthMode != "brokered" ||
		snapshot.Runtimes == nil ||
		snapshot.SavedTeams == nil ||
		snapshot.Templates == nil {
		t.Fatalf("setup snapshot = %#v", snapshot)
	}

	connect := handler(context.Background(), localipc.Request{
		Version:   1,
		RequestID: "setup-connect-1",
		Method:    "codex_connect",
		Params:    json.RawMessage(`{}`),
	})
	if !connect.OK || connect.Error != nil ||
		string(connect.Result) !=
			`{"provider_id":"codex","auth_mode":"native_auth","status":"started"}` {
		t.Fatalf("codex connect response = %#v", connect)
	}
	rejectedConnect := handler(context.Background(), localipc.Request{
		Version:   1,
		RequestID: "setup-connect-2",
		Method:    "codex_connect",
		Params:    json.RawMessage(`{"provider_id":"codex"}`),
	})
	if rejectedConnect.OK ||
		rejectedConnect.Error == nil ||
		rejectedConnect.Error.Code != "invalid_request" {
		t.Fatalf("unknown codex connect field response = %#v", rejectedConnect)
	}
	busySetup, err := api.NewLocalProductSetupAPI(
		productSetupFixtureBackend{
			connectErr: app.ErrNativeAuthConnectBusy,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	busy := localProductHandler(nil, busySetup)(
		context.Background(),
		localipc.Request{
			Version:   1,
			RequestID: "setup-connect-busy",
			Method:    "codex_connect",
			Params:    json.RawMessage(`{}`),
		},
	)
	if busy.OK || busy.Error == nil ||
		busy.Error.Code != "busy" ||
		!busy.Error.Recoverable {
		t.Fatalf("busy codex connect response = %#v", busy)
	}

	rejected := handler(context.Background(), localipc.Request{
		Version:   1,
		RequestID: "setup-request-2",
		Method:    "setup_snapshot",
		Params:    json.RawMessage(`{"unexpected":true}`),
	})
	if rejected.OK ||
		rejected.Error == nil ||
		rejected.Error.Code != "invalid_request" {
		t.Fatalf("unknown setup field response = %#v", rejected)
	}
}

func TestProductDaemonCredentialVerifyRequiresOneStrictOperationID(
	t *testing.T,
) {
	var captured app.CredentialSetupCommand
	setup, err := api.NewLocalProductSetupAPI(productSetupFixtureBackend{
		credentialCommand: &captured,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandler(nil, setup)
	operationID := "11111111-1111-4111-8111-111111111111"
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "verify-operation", Method: "credential_verify",
		Params: json.RawMessage(`{"provider_id":"minimax","credential_reference":"credential-ref-1","expected_revision":1,"operation_id":"` + operationID + `","secret":""}`),
	})
	if !response.OK || response.Error != nil || captured.OperationID != operationID {
		t.Fatalf("response=%#v command=%#v", response, captured)
	}

	invalid := []json.RawMessage{
		json.RawMessage(`{"provider_id":"minimax","credential_reference":"credential-ref-1","expected_revision":1,"secret":""}`),
		json.RawMessage(`{"provider_id":"minimax","credential_reference":"credential-ref-1","expected_revision":1,"operation_id":"INVALID","secret":""}`),
		json.RawMessage(`{"provider_id":"minimax","credential_reference":"credential-ref-1","expected_revision":1,"operation_id":"` + operationID + `","operation_id":"` + operationID + `","secret":""}`),
		json.RawMessage(`{"provider_id":"minimax","credential_reference":"credential-ref-1","expected_revision":1,"operation_id":"` + operationID + `","secret":"","extra":true}`),
	}
	for index, params := range invalid {
		captured = app.CredentialSetupCommand{}
		response := handler(context.Background(), localipc.Request{
			Version: 1, RequestID: fmt.Sprintf("verify-invalid-%d", index),
			Method: "credential_verify", Params: params,
		})
		if response.OK || response.Error == nil ||
			response.Error.Code != "invalid_request" || captured.OperationID != "" {
			t.Fatalf("case=%d response=%#v command=%#v", index, response, captured)
		}
	}
}

func TestProductDaemonRoutesOneStrictMissionDecisionMethod(t *testing.T) {
	backend, err := app.NewPreparedMissionDecisionBackend(
		app.PreparedMissionDecisions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewLocalProductDecisionService(
		app.MissionDecisionConfig{Backend: backend},
	)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := api.NewLocalProductDecisionAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandlerWithDecision(nil, nil, decision)
	command := app.MissionDecisionCommand{
		SchemaVersion:   1,
		Operation:       "read",
		Kind:            "authorization",
		Action:          "read",
		MissionID:       "mission/team-1",
		TeamInstanceID:  "team-1",
		ViewVersion:     strings.Repeat("a", 64),
		DecisionID:      "decision-1",
		DecisionDigest:  strings.Repeat("b", 64),
		LogicalNodeID:   "main",
		AttemptNumber:   1,
		ClaimGeneration: 0,
		CorrelationID:   "11111111-1111-4111-8111-111111111111",
	}
	params, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	read := handler(context.Background(), localipc.Request{
		Version:   1,
		RequestID: "mission-read-1",
		Method:    "mission_decision",
		Params:    params,
	})
	if read.OK || read.Error == nil || read.Error.Code != "conflict" {
		t.Fatalf("unprepared read response=%#v", read)
	}

	command.Operation = "defer"
	command.Action = "not_now"
	params, _ = json.Marshal(command)
	deferred := handler(context.Background(), localipc.Request{
		Version:   1,
		RequestID: "mission-defer-1",
		Method:    "mission_decision",
		Params:    params,
	})
	if !deferred.OK || deferred.Error != nil {
		t.Fatalf("defer response=%#v", deferred)
	}

	command.Operation = "submit"
	command.Action = "deny"
	params, _ = json.Marshal(command)
	submitted := handler(context.Background(), localipc.Request{
		Version:   1,
		RequestID: "mission-submit-1",
		Method:    "mission_decision",
		Params:    params,
	})
	if submitted.OK ||
		submitted.Error == nil ||
		submitted.Error.Code != "conflict" {
		t.Fatalf("unprepared submit response=%#v", submitted)
	}
}

func TestProductDaemonRoutesOneStrictMissionExecutionMethod(t *testing.T) {
	workPackage, err := work.CodingWorkPackage()
	if err != nil {
		t.Fatal(err)
	}
	want := app.MissionExecutionPreflight{
		SchemaVersion:  app.MissionExecutionSchemaVersion,
		MissionID:      "mission/team-1",
		TeamInstanceID: "team-1",
		Nodes:          []app.MissionExecutionNodePreview{},
	}
	executionAPI, err := api.NewLocalProductExecutionAPI(
		&productExecutionServiceStub{preflight: want},
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandlerWithDecision(nil, nil, nil, executionAPI)
	params, err := json.Marshal(app.MissionExecutionCommand{
		SchemaVersion:       app.MissionExecutionSchemaVersion,
		Operation:           "preflight",
		MissionID:           "mission/team-1",
		TeamInstanceID:      "team-1",
		WorkPackageID:       workPackage.ID(),
		WorkPackageDigest:   workPackage.Digest(),
		Objective:           "bounded mission",
		ExpectedViewVersion: strings.Repeat("b", 64),
		CorrelationID:       "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := handler(context.Background(), localipc.Request{
		Version:   1,
		RequestID: "execution-1",
		Method:    "mission_execution",
		Params:    params,
	})
	if !response.OK || response.Error != nil {
		t.Fatalf("execution response = %#v", response)
	}
	var envelope api.MissionExecutionEnvelope
	if err := json.Unmarshal(response.Result, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Operation != "preflight" || envelope.Preflight == nil ||
		!reflect.DeepEqual(*envelope.Preflight, want) || envelope.Result != nil {
		t.Fatalf("execution envelope = %#v", envelope)
	}

	unavailable := localProductHandlerWithDecision(nil, nil, nil)(
		context.Background(),
		localipc.Request{
			Version:   1,
			RequestID: "execution-unavailable",
			Method:    "mission_execution",
			Params:    params,
		},
	)
	if unavailable.OK || unavailable.Error == nil ||
		unavailable.Error.Code != "state_unavailable" {
		t.Fatalf("unavailable execution response = %#v", unavailable)
	}

	unknown := append([]byte(nil), params[:len(params)-1]...)
	unknown = append(unknown, []byte(`,"unknown":true}`)...)
	invalid := handler(context.Background(), localipc.Request{
		Version:   1,
		RequestID: "execution-invalid",
		Method:    "mission_execution",
		Params:    unknown,
	})
	if invalid.OK || invalid.Error == nil || invalid.Error.Code != "invalid_request" {
		t.Fatalf("invalid execution response = %#v", invalid)
	}
}

func TestProductDaemonSnapshotRebindsPreparedDecisionsAfterRuntimeDiscovery(
	t *testing.T,
) {
	t.Setenv("TMPDIR", "/private/tmp")
	root, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	prepared, err := app.BuildControlledMissionDecisionFixture(
		ctx,
		app.ControlledMissionDecisionFixtureConfig{
			Database:          database,
			ArtifactRoot:      filepath.Join(root, "decision-artifacts"),
			AuthoritativeTime: time.Date(2026, 7, 30, 23, 0, 0, 0, time.UTC),
			FixtureID:         "daemon-view-lifecycle",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	backend, err := app.NewPreparedMissionDecisionBackend(prepared)
	if err != nil {
		t.Fatal(err)
	}
	decisionService, err := app.NewLocalProductDecisionService(
		app.MissionDecisionConfig{Backend: backend},
	)
	if err != nil {
		t.Fatal(err)
	}
	decisionAPI, err := api.NewLocalProductDecisionAPI(decisionService)
	if err != nil {
		t.Fatal(err)
	}
	readService, err := api.NewLocalProductReadService(
		api.LocalProductReadConfig{
			Journal:    journal.NewStore(database),
			Projection: projection.New(database),
			Now: func() time.Time {
				return time.Date(2026, 7, 30, 23, 1, 0, 0, time.UTC)
			},
			Decisions: backend,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandlerWithDecision(
		readService,
		nil,
		decisionAPI,
	)
	socketPath := filepath.Join(root, "loomd.sock")
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      "decision-view-lifecycle-fixture",
		Handler:      localipc.HandlerFunc(handler),
	})
	if err != nil {
		t.Fatal(err)
	}
	serverCtx, cancelServer := context.WithCancel(ctx)
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(serverCtx) }()
	select {
	case <-server.Ready():
	case err := <-serveDone:
		t.Fatalf("Go IPC server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Go IPC server did not become ready")
	}
	defer func() {
		cancelServer()
		if err := server.Close(); err != nil {
			t.Errorf("close Go IPC server: %v", err)
		}
		select {
		case err := <-serveDone:
			if err != nil {
				t.Errorf("Go IPC Serve() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Go IPC server did not close")
		}
	}()
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	readSnapshot := func(requestID string) api.LocalProductSnapshot {
		t.Helper()
		var snapshot api.LocalProductSnapshot
		if err := client.Call(
			ctx,
			"snapshot",
			api.LocalProductSnapshotRequest{Limit: 64},
			&snapshot,
		); err != nil {
			t.Fatalf("%s snapshot call: %v", requestID, err)
		}
		return snapshot
	}

	before := readSnapshot("snapshot-before-discovery")
	if len(before.PreparedDecisions) != 4 {
		t.Fatalf("prepared decisions before discovery = %#v", before)
	}
	oldCommand := before.PreparedDecisions[0]

	instance, err := loomruntime.NewRuntimeInstance(
		loomruntime.RuntimeInstance{
			ID:                   "runtime-after-prepared-snapshot",
			DeviceID:             "device-after-prepared-snapshot",
			AdapterType:          "fixture",
			DisplayName:          "Runtime after prepared snapshot",
			ExecutableVersion:    "1.0.0",
			Status:               loomruntime.RuntimeOnline,
			ObservedCapabilities: []string{"models"},
			Capacity:             1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := loomruntime.DiscoverRuntime(
		ctx,
		[]loomruntime.RuntimeProbe{productDaemonRuntimeProbe{
			id:       "probe-after-prepared-snapshot",
			instance: instance,
			models:   []string{"controlled-model"},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.CommitRuntimeDiscoverySnapshot(
		ctx,
		journal.NewStore(database),
		discovery,
		state.RuntimeDiscoveryCommitInput{
			DiscoveryID: "discovery-after-prepared-snapshot",
			EmittedAt:   time.Date(2026, 7, 30, 23, 2, 0, 0, time.UTC),
			Events: []state.RuntimeDiscoveryEventInput{{
				RuntimeInstanceID: instance.ID,
				EventID:           "event-after-prepared-snapshot",
				IdempotencyKey:    "runtime-after-prepared-snapshot",
				Seq:               1,
			}},
		},
	); err != nil {
		t.Fatal(err)
	}
	var eventCountAfterDiscovery int
	if err := database.QueryRow(`SELECT COUNT(*) FROM events`).Scan(
		&eventCountAfterDiscovery,
	); err != nil {
		t.Fatal(err)
	}

	after := readSnapshot("snapshot-after-discovery")
	if after.ViewVersion == before.ViewVersion {
		t.Fatal("Runtime discovery did not advance the snapshot view")
	}
	if len(after.PreparedDecisions) != len(before.PreparedDecisions) {
		t.Fatalf("prepared decisions after discovery = %#v", after)
	}
	for _, command := range after.PreparedDecisions {
		if command.ViewVersion != after.ViewVersion {
			t.Fatalf(
				"snapshot view = %s, prepared command = %#v",
				after.ViewVersion,
				command,
			)
		}
	}

	var oldSheet app.MissionDecisionSheet
	err = client.Call(
		ctx,
		"mission_decision",
		oldCommand,
		&oldSheet,
	)
	var remoteError *localipc.RemoteError
	if !errors.As(err, &remoteError) || remoteError.Code != "conflict" {
		t.Fatalf("old command error = %v", err)
	}
	var currentSheet app.MissionDecisionSheet
	if err := client.Call(
		ctx,
		"mission_decision",
		after.PreparedDecisions[0],
		&currentSheet,
	); err != nil {
		t.Fatalf("current command call = %v", err)
	}
	if currentSheet.ViewVersion != after.ViewVersion ||
		currentSheet.DecisionID != after.PreparedDecisions[0].DecisionID {
		t.Fatalf("current command sheet = %#v", currentSheet)
	}
	var finalEventCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM events`).Scan(
		&finalEventCount,
	); err != nil {
		t.Fatal(err)
	}
	if finalEventCount != eventCountAfterDiscovery {
		t.Fatalf(
			"read-only IPC changed Events: after discovery=%d final=%d",
			eventCountAfterDiscovery,
			finalEventCount,
		)
	}
}

func TestProductDaemonProductionRunnerWiresFailClosedDecisionRegistry(
	t *testing.T,
) {
	root, statePath := productDaemonFailureState(t)
	socketPath := filepath.Join(root, "loomd.sock")
	runner, err := newProductDaemonRunner(
		&blockingObserverRunner{},
		statePath,
		socketPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		done <- runErr
	}()
	waitForProductSocket(t, socketPath)
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	command := app.MissionDecisionCommand{
		SchemaVersion:   1,
		Operation:       "read",
		Kind:            "authorization",
		Action:          "read",
		MissionID:       "mission/team-1",
		TeamInstanceID:  "team-1",
		ViewVersion:     strings.Repeat("a", 64),
		DecisionID:      "decision-1",
		DecisionDigest:  strings.Repeat("b", 64),
		LogicalNodeID:   "main",
		AttemptNumber:   1,
		ClaimGeneration: 0,
		CorrelationID:   "11111111-1111-4111-8111-111111111111",
	}
	var sheet app.MissionDecisionSheet
	err = client.Call(
		context.Background(),
		"mission_decision",
		command,
		&sheet,
	)
	var remote *localipc.RemoteError
	if !errors.As(err, &remote) || remote.Code != "conflict" {
		t.Fatalf("production unprepared decision error = %v", err)
	}
	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("product daemon stop error = %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("product daemon did not stop")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProductDaemonProductionRunnerLoadsControlledMissionFixture(
	t *testing.T,
) {
	root, err := os.MkdirTemp("/tmp", "lw2-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove controlled fixture root: %v", err)
		}
	})
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	attemptID := filepath.Base(root)
	stateDirectory := filepath.Join(root, "state")
	manifestDirectory := filepath.Join(root, "manifest")
	artifactDirectory := filepath.Join(root, "artifacts")
	for _, directory := range []string{
		stateDirectory,
		manifestDirectory,
		artifactDirectory,
	} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	statePath := filepath.Join(stateDirectory, "loom.db")
	stateFile, err := os.OpenFile(
		statePath,
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateFile.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Migrate(context.Background(), database); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(
		manifestDirectory,
		"mission-fixture.json",
	)
	manifestBody, err := json.Marshal(controlledMissionFixtureManifest{
		SchemaVersion: 1,
		Purpose:       "p2a-w2-mission-workbench-controlled-live",
		AttemptID:     attemptID,
		StatePath:     statePath,
		ArtifactRoot: filepath.Join(
			artifactDirectory,
			"mission-fixture",
		),
		SourceCommit:      strings.Repeat("a", 40),
		AuthoritativeTime: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
		FixtureID:         "controlled-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, manifestBody, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(
		controlledMissionFixtureManifestEnvironment,
		manifestPath,
	)

	socketPath := filepath.Join(root, "loomd.sock")
	runner, err := newProductDaemonRunner(
		&blockingObserverRunner{},
		statePath,
		socketPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx)
		done <- runErr
	}()
	waitForProductSocket(t, socketPath)
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot api.LocalProductSnapshot
	if err := client.Call(
		context.Background(),
		"snapshot",
		api.LocalProductSnapshotRequest{Limit: 64},
		&snapshot,
	); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Missions) != 5 ||
		len(snapshot.PreparedDecisions) != 4 {
		t.Fatalf(
			"controlled snapshot missions=%d prepared=%d: %#v",
			len(snapshot.Missions),
			len(snapshot.PreparedDecisions),
			snapshot,
		)
	}
	var sheet app.MissionDecisionSheet
	if err := client.Call(
		context.Background(),
		"mission_decision",
		snapshot.PreparedDecisions[0],
		&sheet,
	); err != nil {
		t.Fatal(err)
	}
	if !sheet.Prepared ||
		sheet.MissionID != snapshot.PreparedDecisions[0].MissionID {
		t.Fatalf("controlled decision sheet=%#v", sheet)
	}

	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("product daemon stop error = %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("product daemon did not stop")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{socketPath, socketPath + ".lock"} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("owned IPC path remains after close %q: %v", path, err)
		}
	}
}
