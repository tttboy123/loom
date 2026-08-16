package app

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/work"
)

func TestProjectionMissionFallbackApprovalSourceResolvesOnlyLatestApproval(t *testing.T) {
	database := openTeamCanaryDB(t)
	store := journal.NewStore(database)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	profile := loomruntime.RuntimeProfile{
		ID: "profile-primary", AdapterType: "codex", ProviderID: "openai",
		ProviderAccountID: "openai.primary", ModelID: "gpt-5.5-codex",
		AuthMode:            loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("a", 64),
		CredentialReference: "credential-ref-openai-primary",
		CredentialRevision:  3, Timeout: time.Minute,
	}
	instance := loomruntime.RuntimeInstance{
		ID: "runtime-codex", DeviceID: "device-local", AdapterType: "codex",
		DisplayName: "Codex", ExecutableVersion: "1.0.0",
		Status: loomruntime.RuntimeOnline, Capacity: 1,
	}
	fallback := profile
	fallback.ID = "profile-fallback"
	fallback.AdapterType = "loom-native"
	fallback.ProviderID = "deepseek"
	fallback.ProviderAccountID = "deepseek.primary"
	fallback.ModelID = "deepseek-chat"
	fallback.EndpointFingerprint = strings.Repeat("b", 64)
	fallback.CredentialReference = "credential-ref-deepseek-primary"
	fallbackInstance := instance
	fallbackInstance.ID = "runtime-loom"
	fallbackInstance.AdapterType = "loom-native"
	sourceBinding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	targetBinding, err := loomruntime.FreezeExecutionBinding(fallback, fallbackInstance)
	if err != nil {
		t.Fatal(err)
	}
	query := MissionFallbackApprovalQuery{
		TeamInstanceID: "team-mixed-instance",
		PlanDigest:     strings.Repeat("c", 64), LogicalNodeID: "main",
		SourceBindingDigest: sourceBinding.BindingDigest,
		TargetBindingDigest: targetBinding.BindingDigest,
	}
	scope, err := work.NewTeamFallbackDecisionScope(
		work.TeamFallbackDecisionScopeInput{
			Version: 1, TeamInstanceID: query.TeamInstanceID,
			PlanDigest: query.PlanDigest, LogicalNodeID: query.LogicalNodeID,
			SourceBindingDigest: query.SourceBindingDigest,
			TargetBindingDigest: query.TargetBindingDigest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	if _, err := writer.CommitMissionFallbackDecision(
		context.Background(), state.MissionFallbackDecisionCommand{
			CommandID: "approve-source-v1", ExpectedRevision: 0,
			OccurredAt: now, CorrelationID: "fallback-source-1",
			ActorRef: "user:local-owner", Scope: scope,
			Decision: state.MissionFallbackApproved,
		},
	); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	source, err := NewProjectionMissionExecutionBindingSource(readModel)
	if err != nil {
		t.Fatal(err)
	}
	approval, found, err := source.ResolveMissionFallbackApproval(
		context.Background(), query,
	)
	if err != nil || !found || !approval.Valid() || approval.Version() != 1 {
		t.Fatalf("approval = %#v, found=%t, err=%v", approval, found, err)
	}
	foreign := query
	foreign.PlanDigest = strings.Repeat("d", 64)
	if approval, found, err := source.ResolveMissionFallbackApproval(
		context.Background(), foreign,
	); err != nil || found || approval.Valid() {
		t.Fatalf("foreign approval = %#v, found=%t, err=%v", approval, found, err)
	}
	if _, err := writer.CommitMissionFallbackDecision(
		context.Background(), state.MissionFallbackDecisionCommand{
			CommandID: "reject-source-v2", ExpectedRevision: 1,
			OccurredAt: now.Add(time.Second), CorrelationID: "fallback-source-2",
			ActorRef: "user:local-owner", Scope: scope,
			Decision: state.MissionFallbackRejected,
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if approval, found, err := source.ResolveMissionFallbackApproval(
		context.Background(), query,
	); err != nil || found || approval.Valid() {
		t.Fatalf("rejected approval = %#v, found=%t, err=%v", approval, found, err)
	}
}

func TestPreparedMissionFallbackDecisionWritesOnlyExplicitExactAction(t *testing.T) {
	for _, test := range []struct {
		name, action, status, projected string
		decision                        state.MissionFallbackDecision
		wantApproval                    bool
	}{
		{
			name: "approve", action: "approve_fallback", status: "approved",
			projected: "approved", decision: state.MissionFallbackApproved,
			wantApproval: true,
		},
		{
			name: "reject", action: "reject_fallback", status: "denied",
			projected: "rejected", decision: state.MissionFallbackRejected,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			database := openTeamCanaryDB(t)
			store := journal.NewStore(database)
			writer, err := state.NewLocalProductSetupWriter(store)
			if err != nil {
				t.Fatal(err)
			}
			readModel := projection.New(database)
			if err := readModel.Rebuild(context.Background()); err != nil {
				t.Fatal(err)
			}
			scope, err := work.NewTeamFallbackDecisionScope(
				work.TeamFallbackDecisionScopeInput{
					Version: 1, TeamInstanceID: "team-prepared-fallback",
					PlanDigest: strings.Repeat("1", 64), LogicalNodeID: "main",
					SourceBindingDigest: strings.Repeat("2", 64),
					TargetBindingDigest: strings.Repeat("3", 64),
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			approve := state.MissionFallbackDecisionCommand{
				CommandID: "prepared-approve-fallback", ExpectedRevision: 0,
				ActorRef: "user:local-owner", Scope: scope,
				Decision: state.MissionFallbackApproved,
			}
			reject := approve
			reject.CommandID = "prepared-reject-fallback"
			reject.Decision = state.MissionFallbackRejected
			sheet := MissionDecisionSheet{
				SchemaVersion: 1, Kind: "fallback",
				MissionID:      "mission/" + scope.TeamInstanceID(),
				TeamInstanceID: scope.TeamInstanceID(),
				ViewVersion:    readModel.GlobalReadView().Version(),
				DecisionID:     "fallback-" + scope.Digest()[:32],
				DecisionDigest: missionFallbackDecisionIntentDigest(scope, 0),
				Title:          "Fallback route decision",
				Summary:        "Choose whether this Agent may use its configured fallback route.",
				Requester:      "Loom", Target: "Main Agent fallback route",
				CommandType: "Versioned binding transition", NetworkAccess: "unchanged",
				CredentialAccess: "Exact target credential reference",
				PermissionScope:  "This Agent and attempt only", AttemptScope: "Attempt 2",
				ExpectedEvidence: "Journal approval bound to source and target bindings",
				TechnicalDetails: []string{},
				Actions:          []string{"not_now", "reject_fallback", "approve_fallback"},
				PreparedActions:  []string{"reject_fallback", "approve_fallback"},
				Prepared:         true, LogicalNodeID: scope.LogicalNodeID(),
				AttemptNumber: 2, ClaimGeneration: 0,
			}
			now := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
			prepared := PreparedMissionDecisions{
				Fallbacks: []PreparedMissionFallbackDecision{{
					Sheet: sheet, Authority: writer,
					Approve: &approve, Reject: &reject,
					Refresh: projectionMissionDecisionRefresh(readModel),
					Now:     func() time.Time { return now },
				}},
			}
			backend, err := NewPreparedMissionDecisionBackend(prepared)
			if err != nil {
				t.Fatal(err)
			}
			service, err := NewLocalProductDecisionService(
				MissionDecisionConfig{Backend: backend},
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := NewPreparedMissionExecutionDecisionRouter(
				backend, service, prepared,
			); err != nil {
				t.Fatalf("fallback decision router: %v", err)
			}
			deferred := missionDecisionCommandFromSheet(sheet, "defer", "not_now")
			deferredResult, err := service.DecideMission(
				context.Background(), deferred,
			)
			if err != nil || deferredResult.Authoritative {
				t.Fatalf("deferred = %#v, err=%v", deferredResult, err)
			}
			events, err := store.ReadStream(
				context.Background(), state.MissionFallbackDecisionStreamID(scope),
			)
			if err != nil || len(events) != 0 {
				t.Fatalf("deferred events = %#v, err=%v", events, err)
			}
			command := missionDecisionCommandFromSheet(sheet, "submit", test.action)
			command.CorrelationID = "fallback-decision-incident"
			result, err := service.DecideMission(context.Background(), command)
			if err != nil || result.Status != test.status ||
				!result.Authoritative || result.ViewVersion == sheet.ViewVersion {
				t.Fatalf("result = %#v, err=%v", result, err)
			}
			record, ok := readModel.MissionFallbackDecision(scope)
			if !ok || record.Decision != test.projected ||
				record.Approval.Valid() != test.wantApproval {
				t.Fatalf("record = %#v, ok=%t", record, ok)
			}
		})
	}
}

func TestProjectionMissionFallbackDecisionPreparerRegistersAndReplacesExactScope(
	t *testing.T,
) {
	database := openTeamCanaryDB(t)
	store := journal.NewStore(database)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	backend, err := NewPreparedMissionDecisionBackend(PreparedMissionDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewLocalProductDecisionService(
		MissionDecisionConfig{Backend: backend},
	)
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewPreparedMissionExecutionDecisionRouter(
		backend, service, PreparedMissionDecisions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 11, 11, 0, 0, 0, time.UTC)
	preparer, err := NewProjectionMissionFallbackDecisionPreparer(
		ProjectionMissionFallbackDecisionPreparerConfig{
			Backend: backend, Projection: readModel, Authority: writer,
			ActorRef: "user:local-owner", Now: func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	teamID := "team-dynamic-fallback"
	scope := mustMissionFallbackDecisionScope(
		t, teamID, strings.Repeat("1", 64), "main",
		strings.Repeat("2", 64), strings.Repeat("3", 64),
	)
	candidate := MissionFallbackDecisionCandidate{
		MissionID: "mission/" + teamID, Scope: scope,
		AgentTitle: "Main Agent", HarnessAdapter: "loom-native",
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		ModelID: "deepseek-chat", CredentialRevision: 6,
	}
	view := readModel.GlobalReadView().Version()
	if err := preparer.PrepareMissionFallbackDecisions(
		context.Background(), teamID, view,
		[]MissionFallbackDecisionCandidate{candidate},
	); err != nil {
		t.Fatal(err)
	}
	commands, err := backend.ListMissionDecisionCommands(
		context.Background(), MissionDecisionCommandQuery{
			ViewVersion: view, Mode: MissionDecisionCommandPreserveStale,
		},
	)
	if err != nil || len(commands) != 1 || commands[0].Kind != "fallback" {
		t.Fatalf("prepared commands = %#v, err=%v", commands, err)
	}
	first := commands[0]
	sheet, err := backend.ReadMissionDecision(context.Background(), first)
	if err != nil || sheet.Target != "Main Agent" ||
		!strings.Contains(sheet.CommandType, "deepseek.primary") ||
		strings.Contains(sheet.CommandType, "credential-ref") {
		t.Fatalf("prepared sheet = %#v, err=%v", sheet, err)
	}

	deferCommand := missionExecutionTestCommand(missionExecutionControl)
	deferCommand.WorkPackageID = ""
	deferCommand.WorkPackageDigest = ""
	deferCommand.Objective = ""
	deferCommand.TeamInstanceID = teamID
	deferCommand.MissionID = "mission/" + teamID
	deferCommand.ExpectedViewVersion = view
	deferCommand.ExecutionDigest = strings.Repeat("a", 64)
	deferCommand.ControlAction = "not_now"
	deferCommand.LogicalNodeID = "main"
	deferCommand.AttemptNumber = 2
	deferCommand.ClaimGeneration = 0
	if err := router.RouteMissionExecutionControl(
		context.Background(), deferCommand,
	); err != nil {
		t.Fatalf("not now: %v", err)
	}
	events, err := store.ReadStream(
		context.Background(), state.MissionFallbackDecisionStreamID(scope),
	)
	if err != nil || len(events) != 0 {
		t.Fatalf("not-now events = %#v, err=%v", events, err)
	}

	approveCommand := deferCommand
	approveCommand.ControlAction = "approve"
	approveCommand.CorrelationID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	if err := router.RouteMissionExecutionControl(
		context.Background(), approveCommand,
	); err != nil {
		t.Fatalf("approve: %v", err)
	}
	record, ok := readModel.MissionFallbackDecision(scope)
	if !ok || record.Decision != "approved" || record.Revision != 1 ||
		!record.Approval.Valid() {
		t.Fatalf("approved record = %#v, ok=%t", record, ok)
	}

	view = readModel.GlobalReadView().Version()
	if err := preparer.PrepareMissionFallbackDecisions(
		context.Background(), teamID, view,
		[]MissionFallbackDecisionCandidate{candidate},
	); err != nil {
		t.Fatal(err)
	}
	commands, err = backend.ListMissionDecisionCommands(
		context.Background(), MissionDecisionCommandQuery{
			ViewVersion: view, Mode: MissionDecisionCommandPreserveStale,
		},
	)
	if err != nil || len(commands) != 1 ||
		commands[0].DecisionDigest == first.DecisionDigest {
		t.Fatalf("revision-two prepared commands = %#v, err=%v", commands, err)
	}
	rejectCommand := approveCommand
	rejectCommand.ExpectedViewVersion = view
	rejectCommand.ControlAction = "reject"
	rejectCommand.CorrelationID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	now = now.Add(time.Second)
	if err := router.RouteMissionExecutionControl(
		context.Background(), rejectCommand,
	); err != nil {
		t.Fatalf("reject: %v", err)
	}
	record, ok = readModel.MissionFallbackDecision(scope)
	if !ok || record.Decision != "rejected" || record.Revision != 2 ||
		record.Approval.Valid() {
		t.Fatalf("rejected record = %#v, ok=%t", record, ok)
	}

	drifted := candidate
	drifted.Scope = mustMissionFallbackDecisionScope(
		t, teamID, strings.Repeat("4", 64), "main",
		strings.Repeat("2", 64), strings.Repeat("3", 64),
	)
	view = readModel.GlobalReadView().Version()
	if err := preparer.PrepareMissionFallbackDecisions(
		context.Background(), teamID, view,
		[]MissionFallbackDecisionCandidate{drifted},
	); err != nil {
		t.Fatal(err)
	}
	commands, err = backend.ListMissionDecisionCommands(
		context.Background(), MissionDecisionCommandQuery{
			ViewVersion: view, Mode: MissionDecisionCommandPreserveStale,
		},
	)
	if err != nil || len(commands) != 1 ||
		commands[0].DecisionID == first.DecisionID {
		t.Fatalf("drifted commands = %#v, err=%v", commands, err)
	}
	if _, err := backend.ReadMissionDecision(
		context.Background(), first,
	); !errors.Is(err, ErrMissionDecisionConflict) {
		t.Fatalf("stale decision error = %v", err)
	}
	peer := candidate
	peer.MissionID = "mission/team-peer-fallback"
	peer.Scope = mustMissionFallbackDecisionScope(
		t, "team-peer-fallback", strings.Repeat("5", 64), "review",
		strings.Repeat("6", 64), strings.Repeat("7", 64),
	)
	peer.AgentTitle = "Review Agent"
	if err := preparer.PrepareMissionFallbackDecisions(
		context.Background(), peer.Scope.TeamInstanceID(), view,
		[]MissionFallbackDecisionCandidate{peer},
	); err != nil {
		t.Fatal(err)
	}
	if err := preparer.PrepareMissionFallbackDecisions(
		context.Background(), teamID, view, nil,
	); err != nil {
		t.Fatal(err)
	}
	commands, err = backend.ListMissionDecisionCommands(
		context.Background(), MissionDecisionCommandQuery{
			ViewVersion: view, Mode: MissionDecisionCommandPreserveStale,
		},
	)
	if err != nil || len(commands) != 1 ||
		commands[0].TeamInstanceID != peer.Scope.TeamInstanceID() {
		t.Fatalf("peer decision isolation = %#v, err=%v", commands, err)
	}
}

func TestDynamicFallbackApprovalRecompilesAndReplaysIntoTeamBoardProjection(
	t *testing.T,
) {
	ctx := context.Background()
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	database := openTeamCanaryDB(t)
	store := journal.NewStore(database)
	seedTeamCanaryRuntime(t, store, "runtime-primary", 1, now)
	seedTeamCanaryRuntime(t, store, "runtime-fallback", 1, now)
	seedTeamCanaryRuntime(t, store, "runtime-review", 1, now)
	readModel := projection.New(database)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	decisionBackend, err := NewPreparedMissionDecisionBackend(
		PreparedMissionDecisions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	decisionService, err := NewLocalProductDecisionService(
		MissionDecisionConfig{Backend: decisionBackend},
	)
	if err != nil {
		t.Fatal(err)
	}
	preparer, err := NewProjectionMissionFallbackDecisionPreparer(
		ProjectionMissionFallbackDecisionPreparerConfig{
			Backend: decisionBackend, Projection: readModel, Authority: writer,
			ActorRef: "user:local-owner", Now: func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	approvalSource, err := NewProjectionMissionExecutionBindingSource(readModel)
	if err != nil {
		t.Fatal(err)
	}
	profile := func(
		id, adapter, provider, account, model, reference string,
		revision int64,
	) loomruntime.RuntimeProfile {
		t.Helper()
		value, profileErr := loomruntime.NewRuntimeProfile(
			loomruntime.RuntimeProfile{
				ID: id, AdapterType: adapter, ProviderID: provider,
				ProviderAccountID: account, ModelID: model,
				AuthMode:            loomruntime.AuthBrokered,
				EndpointFingerprint: strings.Repeat(id[len(id)-1:], 64),
				CredentialReference: reference, CredentialRevision: revision,
				Timeout: time.Minute,
			},
		)
		if profileErr != nil {
			t.Fatal(profileErr)
		}
		return value
	}
	instance := func(id, adapter string) loomruntime.RuntimeInstance {
		t.Helper()
		value, instanceErr := loomruntime.NewRuntimeInstance(
			loomruntime.RuntimeInstance{
				ID: id, DeviceID: "device-local", AdapterType: adapter,
				DisplayName: id, ExecutableVersion: "1.0.0",
				Status: loomruntime.RuntimeOnline, Capacity: 1,
			},
		)
		if instanceErr != nil {
			t.Fatal(instanceErr)
		}
		return value
	}
	primaryProfile := profile(
		"profile-primary-1", "codex", "openai", "openai.primary",
		"gpt-5.5-codex", "credential-ref-openai-primary", 4,
	)
	primaryInstance := instance("runtime-primary", "codex")
	fallbackProfile := profile(
		"profile-fallback-2", "loom-native", "deepseek", "deepseek.primary",
		"deepseek-chat", "credential-ref-deepseek-primary", 7,
	)
	fallbackInstance := instance("runtime-fallback", "loom-native")
	reviewProfile := profile(
		"profile-review-3", "loom-native", "minimax", "minimax.reviewer",
		"MiniMax-M2.1", "credential-ref-minimax-reviewer", 2,
	)
	reviewInstance := instance("runtime-review", "loom-native")
	teamID := "team-dynamic-board"
	bindingSource := &controlledMissionExecutionBindingSource{
		binding: MissionExecutionBinding{
			ViewVersion:    readModel.GlobalReadView().Version(),
			TeamInstanceID: teamID,
			Roles: []MissionExecutionRoleBinding{{
				LogicalNodeID: "main", Title: "Govern delivery",
				Role: teams.ExecutionRoleMain, DependsOn: []string{"review"},
				AgentInstanceID: "agent-main", Profile: primaryProfile,
				Instance: primaryInstance, CapacityAvailable: 1,
				FallbackConfigured: true, FallbackProfile: fallbackProfile,
				FallbackInstance: fallbackInstance, FallbackCapacityAvailable: 1,
				FallbackStatus: "ready", FallbackApprovalRequired: true,
			}, {
				LogicalNodeID: "review", Title: "Review delivery",
				Role: teams.ExecutionRoleSubAgent, DependsOn: []string{},
				AgentInstanceID: "agent-review", Profile: reviewProfile,
				Instance: reviewInstance, CapacityAvailable: 1, Status: "ready",
			}},
		},
	}
	compiler, err := NewBuiltInMissionExecutionCompiler(
		BuiltInMissionExecutionCompilerConfig{
			Bindings: bindingSource, FallbackApprovals: approvalSource,
			SourcePath: t.TempDir(), Now: func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	command := missionExecutionTestCommand(missionExecutionPreflight)
	command.MissionID = "mission/" + teamID
	command.TeamInstanceID = teamID
	command.Objective = "Govern delivery"
	command.ExpectedViewVersion = bindingSource.binding.ViewVersion
	command.CorrelationID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	initial, err := compiler.CompileMissionExecution(ctx, command)
	if err != nil || len(initial.FallbackDecisions) != 1 ||
		initial.Preflight.Nodes[0].FallbackApprovalAvailable {
		t.Fatalf("initial fallback compilation = %#v, err=%v", initial, err)
	}
	if err := preparer.PrepareMissionFallbackDecisions(
		ctx, teamID, command.ExpectedViewVersion, initial.FallbackDecisions,
	); err != nil {
		t.Fatal(err)
	}
	commands, err := decisionBackend.ListMissionDecisionCommands(
		ctx,
		MissionDecisionCommandQuery{
			ViewVersion: command.ExpectedViewVersion,
			Mode:        MissionDecisionCommandPreserveStale,
		},
	)
	if err != nil || len(commands) != 1 {
		t.Fatalf("prepared fallback command = %#v, err=%v", commands, err)
	}
	sheet, err := decisionBackend.ReadMissionDecision(ctx, commands[0])
	if err != nil {
		t.Fatal(err)
	}
	approve := missionDecisionCommandFromSheet(
		sheet, "submit", "approve_fallback",
	)
	approve.CorrelationID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	result, err := decisionService.DecideMission(ctx, approve)
	if err != nil || !result.Authoritative || result.Status != "approved" {
		t.Fatalf("fallback approval result = %#v, err=%v", result, err)
	}
	bindingSource.binding.ViewVersion = result.ViewVersion
	command.ExpectedViewVersion = result.ViewVersion
	approved, err := compiler.CompileMissionExecution(ctx, command)
	if err != nil || !approved.Preflight.Nodes[0].FallbackApprovalAvailable ||
		approved.Preflight.Nodes[0].FallbackApprovalVersion != 1 ||
		!approved.Request.Semantics[0].FallbackApproval.Valid() {
		t.Fatalf("approved fallback compilation = %#v, err=%v", approved, err)
	}
	var primary, fallback TeamNodeExecution
	for _, execution := range approved.Request.Nodes {
		if execution.LogicalNodeID != "main" {
			continue
		}
		switch execution.AttemptNumber {
		case 1:
			primary = execution
		case 2:
			fallback = execution
		}
	}
	if primary.Profile.ID != primaryProfile.ID ||
		fallback.Profile.ID != fallbackProfile.ID ||
		fallback.Profile.ProviderAccountID != "deepseek.primary" ||
		fallback.Profile.CredentialReference != "credential-ref-deepseek-primary" ||
		fallback.Profile.CredentialRevision != 7 {
		t.Fatalf("approved attempt bindings = %#v / %#v", primary, fallback)
	}
	primaryBinding, err := loomruntime.FreezeExecutionBinding(
		primary.Profile, primary.Instance,
	)
	if err != nil {
		t.Fatal(err)
	}
	workRandom := make([]byte, 4096)
	for index := range workRandom {
		workRandom[index] = 0x31 + byte(index/16)
	}
	workAuthority, err := work.NewAuthority(
		store,
		func() time.Time { return now },
		bytes.NewReader(workRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	grantRandom := make([]byte, 4096)
	for index := range grantRandom {
		grantRandom[index] = 0x51 + byte(index/48)
	}
	grantAuthority, err := authorization.NewAuthority(
		store,
		workAuthority,
		func() time.Time { return now },
		bytes.NewReader(grantRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	evidenceRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := evidence.NewStore(filepath.Join(evidenceRoot, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = artifacts.Close() })
	coordinator, err := NewTeamCoordinator(
		workAuthority, grantAuthority, readModel, artifacts,
	)
	if err != nil {
		t.Fatal(err)
	}
	var primaryCalls atomic.Int32
	var fallbackCalls atomic.Int32
	var reviewCalls atomic.Int32
	var verifierCalls atomic.Int32
	primaryExecutor := newTeamCanarySupervisor(
		t, workAuthority, grantAuthority,
		&teamCanaryAdapter{
			barrier:     &teamCanaryBarrier{release: make(chan struct{})},
			adapterType: "codex", runtimeID: primaryInstance.ID,
			calls: &primaryCalls, terminalStatus: "failed",
			terminalReason: "provider_rejected",
		},
	)
	fallbackExecutor := newTeamCanarySupervisor(
		t, workAuthority, grantAuthority,
		&teamCanaryAdapter{
			barrier:     &teamCanaryBarrier{release: make(chan struct{})},
			adapterType: "loom-native", runtimeID: fallbackInstance.ID,
			calls: &fallbackCalls,
		},
	)
	reviewExecutor := newTeamCanarySupervisor(
		t, workAuthority, grantAuthority,
		&teamCanaryAdapter{
			barrier:     &teamCanaryBarrier{release: make(chan struct{})},
			adapterType: "loom-native", runtimeID: reviewInstance.ID,
			calls: &reviewCalls,
		},
	)
	verifierExecutor := newTeamCanarySupervisor(
		t, workAuthority, grantAuthority,
		&teamCanaryAdapter{
			barrier:     &teamCanaryBarrier{release: make(chan struct{})},
			adapterType: "codex", runtimeID: primaryInstance.ID,
			calls: &verifierCalls,
		},
	)
	for index := range approved.Request.Nodes {
		execution := &approved.Request.Nodes[index]
		switch execution.LogicalNodeID {
		case "main":
			if execution.AttemptNumber == 1 {
				execution.Executor = primaryExecutor
			} else {
				execution.Executor = fallbackExecutor
			}
		case "review":
			execution.Executor = reviewExecutor
		}
	}
	for index := range approved.Request.Semantics {
		semantics := &approved.Request.Semantics[index]
		if semantics.LogicalNodeID == "main" {
			semantics.VerifierExecution.Executor = verifierExecutor
		} else {
			semantics.VerifierExecution.Executor = reviewExecutor
		}
	}
	executed, err := coordinator.Run(ctx, approved.Request)
	if err != nil || executed.Team().Status() != "succeeded" {
		t.Fatalf("dynamic fallback execution = %#v, err=%v", executed, err)
	}
	if primaryCalls.Load() != 1 || fallbackCalls.Load() != 1 ||
		verifierCalls.Load() != 1 || reviewCalls.Load() != 2 {
		t.Fatalf(
			"execution calls primary=%d fallback=%d verifier=%d review=%d",
			primaryCalls.Load(), fallbackCalls.Load(), verifierCalls.Load(),
			reviewCalls.Load(),
		)
	}
	teamNodes := executed.Team().Nodes()
	var mainRecord, reviewRecord work.TeamNodeRecord
	for _, candidate := range teamNodes {
		switch candidate.LogicalNodeID() {
		case "main":
			mainRecord = candidate
		case "review":
			reviewRecord = candidate
		}
	}
	mainAttempts := mainRecord.Attempts()
	reviewAttempts := reviewRecord.Attempts()
	if mainRecord.Status() != "succeeded" || len(mainAttempts) != 2 ||
		mainAttempts[0].ExecutionBinding().BindingDigest != primaryBinding.BindingDigest ||
		mainAttempts[1].ExecutionBinding().ProviderAccountID != "deepseek.primary" ||
		mainAttempts[1].ExecutionBinding().CredentialReference !=
			"credential-ref-deepseek-primary" ||
		mainAttempts[1].ExecutionBinding().CredentialRevision != 7 {
		t.Fatalf("dynamic fallback authority record = %#v", mainRecord)
	}
	if reviewRecord.Status() != "succeeded" || len(reviewAttempts) != 1 ||
		reviewAttempts[0].ExecutionBinding().ProviderAccountID != "minimax.reviewer" ||
		reviewAttempts[0].ExecutionBinding().CredentialReference !=
			"credential-ref-minimax-reviewer" ||
		reviewAttempts[0].ExecutionBinding().CredentialRevision != 2 {
		t.Fatalf("peer Agent isolation record = %#v", reviewRecord)
	}
	events, err := store.ReadStream(ctx, "team-execution/"+teamID)
	if err != nil {
		t.Fatal(err)
	}
	audited := false
	for _, event := range events {
		if event.Type == "TeamNodeRecoveryRecorded" &&
			strings.Contains(string(event.PayloadJSON), `"action":"fallback"`) &&
			strings.Contains(string(event.PayloadJSON),
				`"fallback_approval_digest":"`+
					approved.Request.Semantics[0].FallbackApproval.Digest()+`"`) {
			audited = true
		}
	}
	if !audited {
		t.Fatal("dynamic fallback recovery was not linked to its approval")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	projected, ok := readModel.GlobalReadView().TeamExecution(teamID)
	if !ok || len(projected.Nodes) != 2 {
		t.Fatalf("projected Team execution = %#v, ok=%t", projected, ok)
	}
	var mainBoard, reviewBoard projection.TeamExecutionNode
	for _, candidate := range projected.Nodes {
		switch candidate.LogicalNodeID {
		case "main":
			mainBoard = candidate
		case "review":
			reviewBoard = candidate
		}
	}
	if !mainBoard.FallbackApprovalAvailable ||
		mainBoard.FallbackApprovalVersion != 1 ||
		mainBoard.WorkflowFallbackKey != "builtin/mission-fallback-v1" ||
		mainBoard.FallbackRuntimeInstanceID != "runtime-fallback" ||
		mainBoard.RecoveryApprovalRequired || !mainBoard.FallbackConsumed ||
		mainBoard.Status != "succeeded" || len(mainBoard.Attempts) != 2 ||
		mainBoard.Attempts[1].ExecutionBinding.ProviderAccountID !=
			"deepseek.primary" ||
		mainBoard.Attempts[1].ExecutionBinding.CredentialRevision != 7 {
		t.Fatalf("fallback Board projection = %#v", mainBoard)
	}
	if reviewBoard.Status != "succeeded" || reviewBoard.FallbackConsumed ||
		reviewBoard.FallbackRuntimeInstanceID != "" ||
		len(reviewBoard.Attempts) != 1 ||
		reviewBoard.Attempts[0].ExecutionBinding.ProviderAccountID !=
			"minimax.reviewer" {
		t.Fatalf("peer Agent Board projection = %#v", reviewBoard)
	}
}
