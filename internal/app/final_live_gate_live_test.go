package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"

	_ "modernc.org/sqlite"
)

const (
	finalLiveRuntimeID                 = "runtime.pi.earendil-works.0.82.1"
	finalLiveInstalledPiSHA256         = "af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca"
	finalLiveModelSHA256               = "cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046"
	finalLivePi0821EventStreamSHA256   = "44a2498660ca61efa952ad6a3f10cc0491883411bd2b4572c9a392ec4e9553ec"
	finalLivePi0821OpenAISHA256        = "0d50250fe2931e66e2078279a397814202e1ecddee58faf4b8bc04c278da177a"
	finalLivePi0821SimpleOptionsSHA256 = "74dfde37adbd00a6af1fd707c1c5c876577793b078da9fbbd6d40bb75bfb4749"
	finalLivePrompt                    = "Return only the corrected one-line Go function: func add(a, b int) int { return a - b }"
	finalLiveResolvedManifestRel       = ".loom-evidence/phase1-final-live-gate/resolved-live-manifest-pi-0821-transcript-closure-reopen1-repair2-canary.json"
)

type finalLiveResolvedManifestInput struct {
	Destination         string
	PrivateRoot         string
	PiExecutable        string
	LlamaExecutable     string
	ModelPath           string
	ExpectedPiSHA256    string
	ExpectedLlamaSHA256 string
	ExpectedModelSHA256 string
	CreatedAt           time.Time
}

func TestFinalLiveFreshAttemptIsolationAndPrivateSQLite(t *testing.T) {
	privateRoot := t.TempDir()
	if err := os.Chmod(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	first, err := createFinalLiveFreshAttemptRoot(privateRoot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := createFinalLiveFreshAttemptRoot(privateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if first == second ||
		filepath.Dir(first) != privateRoot ||
		filepath.Dir(second) != privateRoot {
		t.Fatal("fresh attempt roots are not distinct direct children")
	}
	for _, root := range []string{first, second} {
		info, err := os.Lstat(root)
		if err != nil ||
			!info.IsDir() ||
			info.Mode()&os.ModeSymlink != 0 ||
			info.Mode().Perm() != 0o700 {
			t.Fatal("fresh attempt root is not an exact private directory")
		}
	}

	databasePath, err := createFinalLivePrivateSQLite(first)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(databasePath)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 {
		t.Fatal("fresh SQLite leaf is not an exact private regular file")
	}
	if _, err := createFinalLivePrivateSQLite(first); err == nil {
		t.Fatal("fresh SQLite helper reused an existing leaf")
	}
	infoAfter, err := os.Lstat(databasePath)
	if err != nil ||
		!infoAfter.Mode().IsRegular() ||
		infoAfter.Mode().Perm() != 0o600 {
		t.Fatal("SQLite collision changed the existing leaf")
	}
}

func TestFinalLiveManifestDoesNotAliasPriorEvidence(t *testing.T) {
	prior := ".loom-evidence/phase1-final-live-gate/resolved-live-manifest.json"
	additional := ".loom-evidence/phase1-final-live-gate/resolved-live-manifest-additional-canary.json"
	progressive := ".loom-evidence/phase1-final-live-gate/resolved-live-manifest-progressive-identity-canary.json"
	diagnostic := ".loom-evidence/phase1-final-live-gate/resolved-live-manifest-rejection-diagnostic-canary.json"
	closure := ".loom-evidence/phase1-final-live-gate/resolved-live-manifest-pi-0821-transcript-closure-canary.json"
	reopen1 := ".loom-evidence/phase1-final-live-gate/resolved-live-manifest-pi-0821-transcript-closure-reopen1-canary.json"
	want := ".loom-evidence/phase1-final-live-gate/resolved-live-manifest-pi-0821-transcript-closure-reopen1-repair2-canary.json"
	if finalLiveResolvedManifestRel == prior ||
		finalLiveResolvedManifestRel == additional ||
		finalLiveResolvedManifestRel == progressive ||
		finalLiveResolvedManifestRel == diagnostic ||
		finalLiveResolvedManifestRel == closure ||
		finalLiveResolvedManifestRel == reopen1 {
		t.Fatal("transcript closure manifest aliases prior evidence")
	}
	if finalLiveResolvedManifestRel != want ||
		filepath.Base(finalLiveResolvedManifestRel) !=
			"resolved-live-manifest-pi-0821-transcript-closure-reopen1-repair2-canary.json" {
		t.Fatal("transcript closure manifest name is not frozen")
	}
	if filepath.Dir(finalLiveResolvedManifestRel) !=
		filepath.Dir(prior) {
		t.Fatal("transcript closure manifest escaped final-live evidence")
	}
}

func TestFinalLiveRejectionDiagnosticCanaryRemainsHistorical(t *testing.T) {
	diagnostic := ".loom-evidence/phase1-final-live-gate/" +
		"resolved-live-manifest-rejection-diagnostic-canary.json"
	if diagnostic == finalLiveResolvedManifestRel ||
		filepath.Dir(diagnostic) != filepath.Dir(finalLiveResolvedManifestRel) {
		t.Fatal("historical rejection diagnostic evidence was aliased")
	}
}

func TestFinalLivePi0821TranscriptClosureCanaryIsolation(t *testing.T) {
	t.Run("independent manifest", func(t *testing.T) {
		want := ".loom-evidence/phase1-final-live-gate/" +
			"resolved-live-manifest-pi-0821-transcript-closure-reopen1-repair2-canary.json"
		if finalLiveResolvedManifestRel != want {
			t.Fatalf("transcript closure manifest = %q", finalLiveResolvedManifestRel)
		}
	})

	t.Run("independent attempt prefix", func(t *testing.T) {
		privateRoot := t.TempDir()
		if err := os.Chmod(privateRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		attemptRoot, err := createFinalLiveFreshAttemptRoot(privateRoot)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(
			filepath.Base(attemptRoot),
			"controlled-canary-pi-0821-transcript-closure-reopen1-repair2-",
		) {
			t.Fatalf("transcript closure attempt root = %q", filepath.Base(attemptRoot))
		}
	})
}

func TestFinalLiveAuthoritativeClockBinding(t *testing.T) {
	snapshot := time.Date(2026, time.July, 28, 12, 34, 56, 789, time.UTC)

	t.Run("fixed UTC snapshot", func(t *testing.T) {
		clock, err := newFinalLiveAuthoritativeClock(snapshot, func() time.Time {
			return snapshot
		})
		if err != nil {
			t.Fatal(err)
		}
		for range 3 {
			if got := clock(); got != snapshot {
				t.Fatalf("authoritative clock = %v, want exact snapshot", got)
			}
		}
	})

	t.Run("zero snapshot", func(t *testing.T) {
		if _, err := newFinalLiveAuthoritativeClock(time.Time{}, func() time.Time {
			return time.Time{}
		}); err == nil {
			t.Fatal("zero authoritative snapshot was accepted")
		}
	})

	t.Run("non UTC snapshot", func(t *testing.T) {
		nonUTC := time.Date(
			2026,
			time.July,
			28,
			12,
			34,
			56,
			789,
			time.FixedZone("not-UTC", 0),
		)
		if _, err := newFinalLiveAuthoritativeClock(nonUTC, func() time.Time {
			return nonUTC
		}); err == nil {
			t.Fatal("non-UTC authoritative snapshot was accepted")
		}
	})

	t.Run("drifting source", func(t *testing.T) {
		call := 0
		if _, err := newFinalLiveAuthoritativeClock(snapshot, func() time.Time {
			call++
			return snapshot.Add(time.Duration(call-1) * time.Nanosecond)
		}); err == nil {
			t.Fatal("drifting authoritative clock source was accepted")
		}
	})
}

func TestFinalLiveGatePiRPCOfflineModel(t *testing.T) {
	if os.Getenv("LOOM_FINAL_LIVE_GATE") != "1" {
		t.Skip("opt-in final live gate is disabled")
	}
	if os.Getenv("LOOM_FINAL_APPROVAL") != "approved" ||
		os.Getenv("LOOM_FINAL_EXECUTION_AUTH") != "local" ||
		os.Getenv("LOOM_FINAL_RUNTIME_ID") != finalLiveRuntimeID {
		t.Fatal("final live gate authorization binding is absent")
	}
	privateRoot := os.Getenv("LOOM_FINAL_PRIVATE_ROOT")
	piExecutable := os.Getenv("LOOM_FINAL_PI_EXECUTABLE")
	llamaExecutable := os.Getenv("LOOM_FINAL_LLAMA_EXECUTABLE")
	modelPath := os.Getenv("LOOM_FINAL_MODEL_PATH")
	searchPaths := filepath.SplitList(os.Getenv("LOOM_FINAL_RUNTIME_SEARCH_PATH"))
	assertFinalLivePrivateBinding(
		t,
		privateRoot,
		llamaExecutable,
		modelPath,
	)
	if piExecutable == "" ||
		!filepath.IsAbs(piExecutable) ||
		filepath.Clean(piExecutable) != piExecutable ||
		len(searchPaths) == 0 {
		t.Fatal("invalid installed Pi Runtime binding")
	}
	if err := bindFinalLivePi0821Sources(piExecutable); err != nil {
		t.Fatal("locked Pi 0.82.1 source binding failed")
	}
	authoritativeSnapshot := time.Now().UTC()
	authoritativeClock, err := newFinalLiveAuthoritativeClock(
		authoritativeSnapshot,
		func() time.Time { return authoritativeSnapshot },
	)
	if err != nil {
		t.Fatal("final live authoritative clock binding failed")
	}
	llamaExecutableSHA256 := os.Getenv("LOOM_FINAL_LLAMA_EXECUTABLE_SHA256")
	if err := writeFinalLiveResolvedManifest(finalLiveResolvedManifestInput{
		Destination:         finalLiveResolvedManifestPath(t),
		PrivateRoot:         privateRoot,
		PiExecutable:        piExecutable,
		LlamaExecutable:     llamaExecutable,
		ModelPath:           modelPath,
		ExpectedPiSHA256:    finalLiveInstalledPiSHA256,
		ExpectedLlamaSHA256: llamaExecutableSHA256,
		ExpectedModelSHA256: finalLiveModelSHA256,
		CreatedAt:           time.Now().UTC(),
	}); err != nil {
		t.Fatal("sanitized resolved live manifest was not created before execution")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	runRoot, err := createFinalLiveFreshAttemptRoot(privateRoot)
	if err != nil {
		t.Fatal("cannot create fresh private canary root")
	}
	isolationRoot := finalLivePrivateDirectory(t, runRoot, "metadata")
	metadataRunner, err := piadapter.NewPiMetadataProcessRunner(
		piadapter.PiMetadataProcessRunnerConfig{
			ExecutablePath:     piExecutable,
			IsolationRoot:      isolationRoot,
			RuntimeSearchPaths: searchPaths,
			Timeout:            15 * time.Second,
		},
	)
	if err != nil {
		t.Fatal("Pi metadata runner construction failed")
	}
	probe, err := loomruntime.NewPiRuntimeProbe(loomruntime.PiRuntimeProbeConfig{
		ProbeID:     "probe.pi.final-live",
		InstanceID:  finalLiveRuntimeID,
		DeviceID:    "device.local",
		DisplayName: "Pi 0.82.1 Final Live Gate",
		Runner:      metadataRunner,
	})
	if err != nil {
		t.Fatal("Pi Runtime probe construction failed")
	}
	observations, err := probe.ObserveRuntime(ctx)
	if err != nil ||
		len(observations) != 1 ||
		observations[0].Instance.ID != finalLiveRuntimeID ||
		observations[0].Instance.AdapterType != "pi-cli" ||
		observations[0].Instance.Status != loomruntime.RuntimeOnline {
		t.Fatal("installed Pi Runtime discovery failed")
	}

	modelServer, err := piadapter.StartPiLocalModelServer(
		ctx,
		piadapter.PiLocalModelServerConfig{
			PrivateRoot:    privateRoot,
			ExecutablePath: llamaExecutable,
			ModelPath:      modelPath,
			Host:           "127.0.0.1",
			Port:           18427,
			StartupTimeout: 60 * time.Second,
			CancelGrace:    3 * time.Second,
		},
	)
	if err != nil {
		t.Fatal("controlled local model startup failed")
	}
	defer func() {
		closeContext, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		if err := modelServer.Close(closeContext); err != nil {
			t.Errorf("controlled local model cleanup failed: %v", err)
		}
	}()

	adapter, err := piadapter.NewPiRPCBridgeAdapter(
		piadapter.PiRPCBridgeAdapterConfig{
			Execution: piadapter.PiExecutionAdapterConfig{
				ExecutablePath:     piExecutable,
				RuntimeInstanceID:  finalLiveRuntimeID,
				RuntimeSearchPaths: searchPaths,
				CancelGrace:        3 * time.Second,
				Now:                authoritativeClock,
				Random:             rand.Reader,
			},
			ProviderID:        "loom-local",
			ModelID:           "qwen2.5-coder-1.5b-instruct-q4-k-m",
			BaseURL:           modelServer.BaseURL(),
			MaxAssistantBytes: 16384,
		},
	)
	if err != nil {
		t.Fatal("Pi RPC Bridge adapter construction failed")
	}

	databasePath, err := createFinalLivePrivateSQLite(runRoot)
	if err != nil {
		t.Fatal("cannot create fresh private canary database")
	}
	db, err := sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := journal.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	seedFinalLiveRuntime(t, store, authoritativeClock())
	workAuthority, err := work.NewAuthority(store, authoritativeClock, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	grantAuthority, err := authorization.NewAuthority(
		store,
		workAuthority,
		authoritativeClock,
		rand.Reader,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	artifactStore, err := evidence.NewStore(filepath.Join(runRoot, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	defer artifactStore.Close()
	coordinator, err := NewTeamCoordinator(
		workAuthority,
		grantAuthority,
		readModel,
		artifactStore,
	)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-final-live-gate",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID:     "main",
			Title:             "Correct bounded Go snippet",
			AgentInstanceID:   "agent-main-final-live",
			RuntimeInstanceID: finalLiveRuntimeID,
			Role:              teams.ExecutionRoleMain,
			MaxAttempts:       1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	workItemID := appTeamAttemptIdentity("work", plan, "main", 1)
	runID := appTeamAttemptIdentity("run", plan, "main", 1)
	promptPayload, err := json.Marshal(struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Prompt        string `json:"prompt"`
	}{
		SchemaVersion: 1,
		Kind:          "pi_rpc_prompt",
		Prompt:        finalLivePrompt,
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             "71000000-0000-4000-8000-000000000001",
		CorrelationID:         "72000000-0000-4000-8000-000000000001",
		WorkItemID:            workItemID,
		RunID:                 runID,
		ClaimGeneration:       1,
		RuntimeInstanceID:     finalLiveRuntimeID,
		SenderAgentInstanceID: "agent-main-final-live",
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt:             authoritativeClock(),
		Payload:               promptPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:          "profile-pi-final-live",
		AdapterType: "pi-cli",
		AuthMode:    loomruntime.AuthBrokered,
		Timeout:     120 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID:                finalLiveRuntimeID,
		DeviceID:          "device.local",
		AdapterType:       "pi-cli",
		DisplayName:       "Pi 0.82.1 Final Live Gate",
		ExecutableVersion: "0.82.1",
		Status:            loomruntime.RuntimeOnline,
		Capacity:          1,
	})
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := finalLivePrivateDirectory(t, runRoot, "source")
	if err := os.WriteFile(
		filepath.Join(sourcePath, "bounded.go"),
		[]byte("package bounded\nfunc add(a, b int) int { return a - b }\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	observer := &finalLiveOutputObserver{}
	result, err := coordinator.Run(ctx, TeamExecutionRequest{
		Plan: plan,
		Nodes: []TeamNodeExecution{{
			LogicalNodeID: "main",
			AttemptNumber: 1,
			WorkflowPath:  "primary",
			SourcePath:    sourcePath,
			Profile:       profile,
			Instance:      instance,
			Dispatch:      dispatch,
			Executor: newTeamCanarySupervisor(
				t,
				workAuthority,
				grantAuthority,
				adapter,
			),
		}},
		Semantics:            testTeamNodeSemantics(t, plan, time.Second, ""),
		AuthoritativeTime:    authoritativeClock(),
		PrepareLeaseDuration: time.Minute,
		GrantLifetime:        2 * time.Minute,
		CorrelationID:        dispatch.CorrelationID(),
		OutputObserver:       observer,
	})
	if err != nil ||
		result.Team().Status() != "succeeded" ||
		len(result.ExecutedNodeIDs()) != 1 ||
		result.ExecutedNodeIDs()[0] != "main" {
		t.Fatalf("final live Team execution failed: status=%q err=%v", result.Team().Status(), err)
	}
	if !observer.validSequence() {
		t.Fatal("authorized tentative output was not observed before terminal")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	projected, ok := readModel.GlobalReadView().TeamExecution(plan.TeamInstanceID())
	if !ok || projected.Status != "succeeded" {
		t.Fatal("final live Team terminal was not projected")
	}
	assertFinalLiveJournal(t, ctx, store, observer, privateRoot)
}

func TestFinalLiveResolvedManifestIsSanitizedAndFailClosed(t *testing.T) {
	privateRoot := filepath.Join(t.TempDir(), "private-source")
	if err := os.Mkdir(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	piPath := filepath.Join(privateRoot, "pi")
	llamaPath := filepath.Join(privateRoot, "llama-server")
	modelPath := filepath.Join(privateRoot, "model.gguf")
	for path, fixture := range map[string]struct {
		content []byte
		mode    os.FileMode
	}{
		piPath:    {content: []byte("pi"), mode: 0o600},
		llamaPath: {content: []byte("llama"), mode: 0o700},
		modelPath: {content: []byte("model"), mode: 0o600},
	} {
		if err := os.WriteFile(path, fixture.content, fixture.mode); err != nil {
			t.Fatal(err)
		}
	}
	destination := filepath.Join(t.TempDir(), "resolved-live-manifest.json")
	input := finalLiveResolvedManifestInput{
		Destination:         destination,
		PrivateRoot:         privateRoot,
		PiExecutable:        piPath,
		LlamaExecutable:     llamaPath,
		ModelPath:           modelPath,
		ExpectedPiSHA256:    finalLiveTestDigest([]byte("pi")),
		ExpectedLlamaSHA256: finalLiveTestDigest([]byte("llama")),
		ExpectedModelSHA256: finalLiveTestDigest([]byte("model")),
		CreatedAt:           time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC),
	}
	inspectorCalls := 0
	inspector := func(
		config piadapter.PiLocalModelServerConfig,
	) (piadapter.PiLocalModelServerBinding, error) {
		inspectorCalls++
		if config.PrivateRoot != privateRoot ||
			config.ExecutablePath != llamaPath ||
			config.ModelPath != modelPath ||
			config.Host != "127.0.0.1" ||
			config.Port != 18427 ||
			config.StartupTimeout != 60*time.Second ||
			config.CancelGrace != 3*time.Second {
			return piadapter.PiLocalModelServerBinding{}, errors.New("unexpected local-model binding request")
		}
		return piadapter.PiLocalModelServerBinding{
			ExecutableSHA256: finalLiveTestDigest([]byte("llama")),
			ModelSHA256:      finalLiveTestDigest([]byte("model")),
		}, nil
	}
	if err := writeFinalLiveResolvedManifestWithInspector(input, inspector); err != nil {
		t.Fatalf("writeFinalLiveResolvedManifest() error = %v", err)
	}
	if inspectorCalls != 1 {
		t.Fatalf("binding inspector calls = %d, want 1", inspectorCalls)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{
		privateRoot,
		piPath,
		llamaPath,
		modelPath,
		finalLivePrompt,
	} {
		if bytes.Contains(content, []byte(secret)) {
			t.Fatalf("resolved manifest leaked private value %q", secret)
		}
	}
	info, err := os.Lstat(destination)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatal("resolved manifest is not a private regular file")
	}

	badDestination := filepath.Join(t.TempDir(), "must-not-exist.json")
	input.Destination = badDestination
	input.ExpectedModelSHA256 = strings.Repeat("0", 64)
	if err := writeFinalLiveResolvedManifestWithInspector(input, inspector); err == nil {
		t.Fatal("writeFinalLiveResolvedManifest() accepted a model digest mismatch")
	}
	if _, err := os.Lstat(badDestination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed manifest validation left a destination file")
	}
}

func writeFinalLiveResolvedManifest(input finalLiveResolvedManifestInput) error {
	return writeFinalLiveResolvedManifestWithInspector(
		input,
		piadapter.InspectPiLocalModelServerBinding,
	)
}

func writeFinalLiveResolvedManifestWithInspector(
	input finalLiveResolvedManifestInput,
	inspect func(
		piadapter.PiLocalModelServerConfig,
	) (piadapter.PiLocalModelServerBinding, error),
) error {
	if input.Destination == "" ||
		!filepath.IsAbs(input.Destination) ||
		filepath.Clean(input.Destination) != input.Destination ||
		input.PrivateRoot == "" ||
		!filepath.IsAbs(input.PrivateRoot) ||
		filepath.Clean(input.PrivateRoot) != input.PrivateRoot ||
		input.CreatedAt.IsZero() ||
		input.CreatedAt.Location() != time.UTC ||
		inspect == nil {
		return errors.New("invalid final live manifest binding")
	}
	rootInfo, err := os.Lstat(input.PrivateRoot)
	if err != nil ||
		!rootInfo.IsDir() ||
		rootInfo.Mode()&os.ModeSymlink != 0 ||
		rootInfo.Mode().Perm() != 0o700 {
		return errors.New("invalid final live manifest private root")
	}
	localBinding, err := inspect(piadapter.PiLocalModelServerConfig{
		PrivateRoot:    input.PrivateRoot,
		ExecutablePath: input.LlamaExecutable,
		ModelPath:      input.ModelPath,
		Host:           "127.0.0.1",
		Port:           18427,
		StartupTimeout: 60 * time.Second,
		CancelGrace:    3 * time.Second,
	})
	if err != nil ||
		!validFinalLiveDigest(localBinding.ExecutableSHA256) ||
		!validFinalLiveDigest(localBinding.ModelSHA256) ||
		localBinding.ExecutableSHA256 != input.ExpectedLlamaSHA256 ||
		localBinding.ModelSHA256 != input.ExpectedModelSHA256 {
		return errors.New("invalid inspected final live local-model binding")
	}
	piDigest, err := finalLiveBoundFileDigest(
		input.PiExecutable,
		input.ExpectedPiSHA256,
	)
	if err != nil {
		return err
	}
	if !validFinalLiveDigest(piDigest) {
		return errors.New("invalid final live Pi digest")
	}
	promptDigest := sha256.Sum256([]byte(finalLivePrompt))
	manifest := struct {
		SchemaVersion int    `json:"schema_version"`
		Status        string `json:"status"`
		Runtime       struct {
			InstanceID       string `json:"instance_id"`
			AdapterType      string `json:"adapter_type"`
			Version          string `json:"version"`
			GitHead          string `json:"git_head"`
			PackageIntegrity string `json:"package_integrity"`
			ExecutableName   string `json:"executable_name"`
			ExecutableSHA    string `json:"executable_sha256"`
		} `json:"runtime"`
		LocalModelServer struct {
			Release            string `json:"release"`
			Commit             string `json:"commit"`
			ReleaseAssetSHA256 string `json:"release_asset_sha256"`
			ExecutableName     string `json:"executable_name"`
			ExecutableSHA      string `json:"executable_sha256"`
			Host               string `json:"host"`
			Port               int    `json:"port"`
		} `json:"local_model_server"`
		Model struct {
			Repository string `json:"repository"`
			Revision   string `json:"revision"`
			File       string `json:"file"`
			SHA256     string `json:"sha256"`
		} `json:"model"`
		Canary struct {
			TeamInstanceID             string `json:"team_instance_id"`
			LogicalNodeID              string `json:"logical_node_id"`
			AttemptNumber              int    `json:"attempt_number"`
			PromptSHA256               string `json:"prompt_sha256"`
			RunDeadlineSeconds         int    `json:"run_deadline_seconds"`
			WholeCanaryDeadlineSeconds int    `json:"whole_canary_deadline_seconds"`
		} `json:"canary"`
		Authorization struct {
			ApprovalDecision string `json:"approval_decision"`
			ExecutionScope   string `json:"execution_scope"`
		} `json:"authorization"`
		CreatedAt string `json:"created_at"`
	}{
		SchemaVersion: 1,
		Status:        "resolved_pre_live",
		CreatedAt:     input.CreatedAt.Format(time.RFC3339Nano),
	}
	manifest.Runtime.InstanceID = finalLiveRuntimeID
	manifest.Runtime.AdapterType = "pi-cli"
	manifest.Runtime.Version = "0.82.1"
	manifest.Runtime.GitHead = "b4f293684bba718d59cc1157679bcf6157b3a7f5"
	manifest.Runtime.PackageIntegrity = "sha512-zbkAhoIuDPMF3pKuja0ajZabrMWU29FUMV9A/XMXT/XC1yXs5xt6t6t13GogQFsDrDqbFP4DkZQO1w8rWRAzYA=="
	manifest.Runtime.ExecutableName = filepath.Base(input.PiExecutable)
	manifest.Runtime.ExecutableSHA = piDigest
	manifest.LocalModelServer.Release = "b10107"
	manifest.LocalModelServer.Commit = "c0bc8591e8815c63cb01dd3f051a8b0df02501c9"
	manifest.LocalModelServer.ReleaseAssetSHA256 = "b9554ab4c9f6e91199f48387cb4ab27466fb1d724881f81463ef03f6370cfa32"
	manifest.LocalModelServer.ExecutableName = filepath.Base(input.LlamaExecutable)
	manifest.LocalModelServer.ExecutableSHA = localBinding.ExecutableSHA256
	manifest.LocalModelServer.Host = "127.0.0.1"
	manifest.LocalModelServer.Port = 18427
	manifest.Model.Repository = "Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF"
	manifest.Model.Revision = "2ab9f8f42af02fc212effaef7c4850c885e965f4"
	manifest.Model.File = filepath.Base(input.ModelPath)
	manifest.Model.SHA256 = localBinding.ModelSHA256
	manifest.Canary.TeamInstanceID = "team-final-live-gate"
	manifest.Canary.LogicalNodeID = "main"
	manifest.Canary.AttemptNumber = 1
	manifest.Canary.PromptSHA256 = hex.EncodeToString(promptDigest[:])
	manifest.Canary.RunDeadlineSeconds = 120
	manifest.Canary.WholeCanaryDeadlineSeconds = 180
	manifest.Authorization.ApprovalDecision = "approved"
	manifest.Authorization.ExecutionScope = "controlled-local"
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	for _, forbidden := range []string{
		input.PrivateRoot,
		input.PiExecutable,
		input.LlamaExecutable,
		input.ModelPath,
		finalLivePrompt,
	} {
		if bytes.Contains(content, []byte(forbidden)) {
			return errors.New("resolved live manifest contains private material")
		}
	}
	parent := filepath.Dir(input.Destination)
	parentInfo, err := os.Lstat(parent)
	if err != nil ||
		!parentInfo.IsDir() ||
		parentInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid final live manifest destination")
	}
	temporary, err := os.CreateTemp(parent, ".resolved-live-manifest-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, input.Destination); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func validFinalLiveDigest(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func TestFinalLiveBoundFilesRequireCurrentUserOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owned")
	if err := os.WriteFile(path, []byte("owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	current, err := user.Current()
	if err != nil || current.Uid == "" {
		t.Fatal("current user UID is unavailable")
	}
	if !finalLiveUIDMatchesCurrent(info, current.Uid) {
		t.Fatal("current-user-owned file was rejected")
	}
	if finalLiveUIDMatchesCurrent(info, current.Uid+"0") {
		t.Fatal("wrong-owner file was accepted")
	}
}

func finalLiveUIDMatchesCurrent(info os.FileInfo, expectedUID string) bool {
	if info == nil || expectedUID == "" {
		return false
	}
	expected, err := strconv.ParseUint(expectedUID, 10, 64)
	if err != nil {
		return false
	}
	stat := reflect.ValueOf(info.Sys())
	if !stat.IsValid() {
		return false
	}
	if stat.Kind() == reflect.Pointer {
		if stat.IsNil() {
			return false
		}
		stat = stat.Elem()
	}
	if stat.Kind() != reflect.Struct {
		return false
	}
	uid := stat.FieldByName("Uid")
	if !uid.IsValid() {
		return false
	}
	switch uid.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		return uid.Uint() == expected
	default:
		return false
	}
}

func finalLiveBoundFileDigest(path string, expected string) (string, error) {
	if path == "" ||
		!filepath.IsAbs(path) ||
		filepath.Clean(path) != path ||
		len(expected) != sha256.Size*2 {
		return "", errors.New("invalid final live file binding")
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return "", errors.New("invalid final live file digest")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(resolved)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Size() <= 0 ||
		info.Mode().Perm()&0o022 != 0 {
		return "", errors.New("unsafe final live file binding")
	}
	current, err := user.Current()
	if err != nil ||
		!finalLiveUIDMatchesCurrent(info, current.Uid) {
		return "", errors.New("unsafe final live file ownership")
	}
	file, err := os.Open(resolved)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		return "", errors.Join(copyErr, closeErr)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if digest != expected {
		return "", errors.New("final live file digest mismatch")
	}
	return digest, nil
}

func bindFinalLivePi0821Sources(piExecutable string) error {
	if piExecutable == "" ||
		!filepath.IsAbs(piExecutable) ||
		filepath.Clean(piExecutable) != piExecutable {
		return errors.New("invalid Pi source binding")
	}
	resolvedPi, err := filepath.EvalSymlinks(piExecutable)
	if err != nil {
		return errors.New("invalid Pi source binding")
	}
	packageRoot := filepath.Dir(filepath.Dir(resolvedPi))
	piAI := filepath.Join(
		packageRoot,
		"node_modules",
		"@earendil-works",
		"pi-ai",
		"dist",
	)
	for _, binding := range []struct {
		path   string
		digest string
	}{
		{
			path:   filepath.Join(piAI, "utils", "event-stream.js"),
			digest: finalLivePi0821EventStreamSHA256,
		},
		{
			path:   filepath.Join(piAI, "api", "openai-completions.js"),
			digest: finalLivePi0821OpenAISHA256,
		},
		{
			path:   filepath.Join(piAI, "api", "simple-options.js"),
			digest: finalLivePi0821SimpleOptionsSHA256,
		},
	} {
		if _, err := finalLiveBoundFileDigest(binding.path, binding.digest); err != nil {
			return errors.New("invalid Pi source binding")
		}
	}
	return nil
}

func finalLiveResolvedManifestPath(t testing.TB) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve final live evidence root")
	}
	repositoryRoot := filepath.Dir(filepath.Dir(filepath.Dir(sourceFile)))
	destination := filepath.Join(repositoryRoot, finalLiveResolvedManifestRel)
	contractPath := filepath.Join(
		repositoryRoot,
		".loom-evidence",
		"phase1-final-live-gate",
		"contract.md",
	)
	if _, err := os.Lstat(contractPath); err != nil {
		t.Fatal("final live contract is absent")
	}
	return destination
}

func finalLiveTestDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

type finalLiveOutputObserver struct {
	mu     sync.Mutex
	frames []bridgev1.Frame
}

func (observer *finalLiveOutputObserver) ObserveNodeOutput(
	_ context.Context,
	output NodeOutput,
) error {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if output.LogicalNodeID() != "main" || output.AttemptNumber() != 1 {
		return errors.New("unexpected final live output binding")
	}
	observer.frames = append(observer.frames, output.AuthorizedFrame().Frame())
	return nil
}

func (observer *finalLiveOutputObserver) validSequence() bool {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	eventIndex := -1
	resultIndex := -1
	evidenceCount := 0
	for index, frame := range observer.frames {
		switch frame.Type() {
		case bridgev1.MessageEvent:
			if eventIndex < 0 {
				eventIndex = index
			}
		case bridgev1.MessageEvidence:
			evidenceCount++
		case bridgev1.MessageResult:
			resultIndex = index
		}
	}
	return eventIndex >= 0 &&
		resultIndex > eventIndex &&
		evidenceCount == 1
}

func assertFinalLivePrivateBinding(
	t testing.TB,
	privateRoot string,
	paths ...string,
) {
	t.Helper()
	if privateRoot == "" ||
		!filepath.IsAbs(privateRoot) ||
		filepath.Clean(privateRoot) != privateRoot {
		t.Fatal("invalid final live private root")
	}
	info, err := os.Lstat(privateRoot)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatal("final live private root is not 0700")
	}
	for _, value := range paths {
		if value == "" || !strings.HasPrefix(value, privateRoot+string(filepath.Separator)) {
			t.Fatal("final live path escaped private root")
		}
	}
}

func finalLivePrivateDirectory(t testing.TB, root string, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func newFinalLiveAuthoritativeClock(
	snapshot time.Time,
	source func() time.Time,
) (func() time.Time, error) {
	if snapshot.IsZero() || snapshot.Location() != time.UTC || source == nil {
		return nil, errors.New("invalid final live authoritative clock")
	}
	if source() != snapshot || source() != snapshot {
		return nil, errors.New("invalid final live authoritative clock")
	}
	return func() time.Time {
		return snapshot
	}, nil
}

func createFinalLiveFreshAttemptRoot(privateRoot string) (string, error) {
	if privateRoot == "" ||
		!filepath.IsAbs(privateRoot) ||
		filepath.Clean(privateRoot) != privateRoot {
		return "", errors.New("invalid final live private root")
	}
	rootInfo, err := os.Lstat(privateRoot)
	if err != nil ||
		!rootInfo.IsDir() ||
		rootInfo.Mode()&os.ModeSymlink != 0 ||
		rootInfo.Mode().Perm() != 0o700 {
		return "", errors.New("invalid final live private root")
	}
	attemptRoot, err := os.MkdirTemp(
		privateRoot,
		"controlled-canary-pi-0821-transcript-closure-reopen1-repair2-",
	)
	if err != nil {
		return "", errors.New("cannot create final live attempt root")
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(attemptRoot)
		}
	}()
	relative, err := filepath.Rel(privateRoot, attemptRoot)
	if err != nil ||
		relative == "." ||
		filepath.IsAbs(relative) ||
		filepath.Dir(relative) != "." {
		return "", errors.New("final live attempt root escaped private root")
	}
	info, err := os.Lstat(attemptRoot)
	if err != nil ||
		!info.IsDir() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o700 {
		return "", errors.New("invalid final live attempt root")
	}
	cleanup = false
	return attemptRoot, nil
}

func createFinalLivePrivateSQLite(attemptRoot string) (string, error) {
	if attemptRoot == "" ||
		!filepath.IsAbs(attemptRoot) ||
		filepath.Clean(attemptRoot) != attemptRoot {
		return "", errors.New("invalid final live attempt root")
	}
	rootInfo, err := os.Lstat(attemptRoot)
	if err != nil ||
		!rootInfo.IsDir() ||
		rootInfo.Mode()&os.ModeSymlink != 0 ||
		rootInfo.Mode().Perm() != 0o700 {
		return "", errors.New("invalid final live attempt root")
	}
	databasePath := filepath.Join(attemptRoot, "canary.sqlite")
	file, err := os.OpenFile(
		databasePath,
		os.O_RDWR|os.O_CREATE|os.O_EXCL,
		0o600,
	)
	if err != nil {
		return "", errors.New("cannot create final live database")
	}
	created := true
	defer func() {
		if created {
			_ = os.Remove(databasePath)
		}
	}()
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", errors.New("cannot sync final live database")
	}
	if err := file.Close(); err != nil {
		return "", errors.New("cannot close final live database")
	}
	info, err := os.Lstat(databasePath)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 {
		return "", errors.New("invalid final live database")
	}
	created = false
	return databasePath, nil
}

func seedFinalLiveRuntime(t testing.TB, store *journal.Store, now time.Time) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"discovery_digest": strings.Repeat("b", 64),
		"source_probe_id":  "probe.pi.final-live",
		"instance": map[string]any{
			"id":                    finalLiveRuntimeID,
			"device_id":             "device.local",
			"adapter_type":          "pi-cli",
			"display_name":          "Pi 0.82.1 Final Live Gate",
			"executable_version":    "0.82.1",
			"status":                "online",
			"observed_capabilities": []string{"pi.metadata.models", "pi.metadata.version"},
			"capacity":              1,
		},
		"model_ids": []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID:             "runtime-final-live-1",
		StreamID:       "runtime_instance:" + finalLiveRuntimeID,
		Seq:            1,
		IdempotencyKey: "runtime-final-live-1",
		Type:           "RuntimeInstanceDiscovered",
		SchemaVersion:  1,
		EmittedAt:      now.Add(-time.Minute),
		CorrelationID:  "72000000-0000-4000-8000-000000000001",
		PayloadJSON:    payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func assertFinalLiveJournal(
	t testing.TB,
	ctx context.Context,
	store *journal.Store,
	observer *finalLiveOutputObserver,
	privateRoot string,
) {
	t.Helper()
	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var terminals, grants, revocations int
	observer.mu.Lock()
	var tentative bytes.Buffer
	for _, frame := range observer.frames {
		if frame.Type() == bridgev1.MessageEvent {
			tentative.Write(frame.Payload())
		}
	}
	observer.mu.Unlock()
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, tentative.Bytes()) ||
			bytes.Contains(event.PayloadJSON, []byte(privateRoot)) ||
			bytes.Contains(event.PayloadJSON, []byte("loom_grant_v1.")) {
			t.Fatal("private or tentative final live material entered Journal")
		}
		switch event.Type {
		case "RunTerminalCommitted":
			terminals++
		case "AgentGrantIssued":
			grants++
		case "AgentGrantRevoked":
			revocations++
		}
	}
	if terminals != 1 || grants != 1 || revocations != 1 {
		t.Fatalf(
			"final live lineage counts terminal=%d grant=%d revoke=%d",
			terminals,
			grants,
			revocations,
		)
	}
}
