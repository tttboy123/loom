package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/runtime/piadapter"
	loomtui "loom-pi-rebuild/internal/tui"

	_ "modernc.org/sqlite"
)

type blockingObserverRunner struct {
	closed bool
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
			err: fmt.Errorf(
				"%w: %w",
				discoveryscan.ErrRuntimeProbeFactoryFailed,
				private,
			),
			want: "observer_probe_factory",
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
			err: fmt.Errorf(
				"%w: %w",
				app.ErrRuntimeObservationProjectionRefresh,
				private,
			),
			want: "observer_projection",
		},
		{
			name: "write",
			err: fmt.Errorf(
				"%w: %w",
				app.ErrRuntimeDiscoveryCommitInputFailed,
				private,
			),
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
	root, err := os.MkdirTemp("", "loom-p2a-failure-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove failure-state root: %v", err)
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
		productSetupRuntimeConfig{},
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
	if snapshot.SchemaVersion != 1 ||
		snapshot.ViewVersion == "" ||
		len(snapshot.Teams) != 1 ||
		len(snapshot.Runtimes) != 1 ||
		snapshot.Runtimes[0].ModelIDs == nil ||
		snapshot.Runtimes[0].ObservedCapabilities == nil ||
		len(snapshot.Runs) != 1 ||
		len(snapshot.Evidence) != 1 ||
		snapshot.Attention == nil {
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
		!strings.Contains(updated.View(), snapshot.ViewVersion) {
		t.Fatalf(
			"headless TUI view does not match daemon view %q: %q",
			snapshot.ViewVersion,
			updated.View(),
		)
	}
	model = updated.(loomtui.Model)
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyTab},
		{Type: tea.KeyTab},
		{Type: tea.KeyTab},
		{Type: tea.KeyEnter},
	} {
		updatedModel, next := model.Update(key)
		model = updatedModel.(loomtui.Model)
		if next != nil {
			updatedModel, _ = model.Update(next())
			model = updatedModel.(loomtui.Model)
		}
	}
	if !strings.Contains(model.View(), "team_planned") {
		t.Fatalf("TUI timeline did not render authoritative record: %q", model.View())
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
		{api.ErrTimelineCursorConflict, "cursor_conflict"},
		{api.ErrStreamGap, "stream_gap"},
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
	closed     *bool
	connectErr error
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
	if err := runner.Close(); !errors.Is(
		err,
		api.ErrInvalidLocalProductSetupAPI,
	) {
		t.Fatalf("Close() error = %v", err)
	}
	if !observer.closed {
		t.Fatal("Close() did not continue closing the observer")
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
