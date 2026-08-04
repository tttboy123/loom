package tui

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/localipc"
)

func TestP3ATUIProductionClientExposesAssetJourneyMethods(t *testing.T) {
	clientType := reflect.TypeOf(&DaemonReadClient{})
	for _, method := range []string{
		"EvolutionAssetSnapshot",
		"EvolutionAssetDiff",
		"EvolutionAssetCommand",
	} {
		if _, found := clientType.MethodByName(method); !found {
			t.Fatalf("DaemonReadClient.%s is missing", method)
		}
	}
}

type fakeReadClient struct {
	snapshot      api.LocalProductSnapshot
	timeline      api.LocalProductTimelinePage
	err           error
	timelineCalls int
	execution     api.MissionExecutionEnvelope
	executionErr  error
	executions    []app.MissionExecutionCommand
	proposals     []app.SideTaskProposalRequest
	creates       []app.SideTaskCreateRequest
	decisions     []app.SideTaskDecisionRequest
	proposal      app.SideTaskProposalResult
	created       app.SideTaskCreateResult
	decided       app.SideTaskDecisionResult
}

type fakeAssetReadClient struct {
	fakeReadClient
	assetSnapshot api.EvolutionAssetSnapshot
	assetCommands []api.EvolutionAssetCommandRequest
}

func (client *fakeAssetReadClient) EvolutionAssetSnapshot(context.Context, api.EvolutionAssetSnapshotRequest) (api.EvolutionAssetSnapshot, error) {
	return client.assetSnapshot, nil
}
func (client *fakeAssetReadClient) EvolutionAssetDiff(context.Context, api.EvolutionAssetDiffRequest) (api.EvolutionAssetDiff, error) {
	return api.EvolutionAssetDiff{}, nil
}
func (client *fakeAssetReadClient) EvolutionAssetCommand(_ context.Context, request api.EvolutionAssetCommandRequest) (api.EvolutionAssetCommandResult, error) {
	client.assetCommands = append(client.assetCommands, request)
	return api.EvolutionAssetCommandResult{OperationID: request.OperationID, Action: request.Action, ViewVersion: strings.Repeat("b", 64), EventIDs: []string{"event-1"}}, nil
}

func TestP3ATUIAssetMutationRequiresExplicitConfirmationAndCancelWritesNothing(t *testing.T) {
	digest := strings.Repeat("a", 64)
	client := &fakeAssetReadClient{assetSnapshot: api.EvolutionAssetSnapshot{
		ViewVersion: digest, Definitions: []assets.SkillDefinition{{DefinitionID: "skill-1", Name: "Skill", LatestRevisionID: "revision-1", Lifecycle: assets.LifecycleCandidate}},
		Revisions:   []assets.SkillRevision{{AssetKind: assets.AssetKindSkill, DefinitionID: "skill-1", RevisionID: "revision-1", ArtifactDigest: digest, Lifecycle: assets.LifecycleCandidate}},
		Candidates:  []assets.EvolutionCandidate{{CandidateID: "candidate-1", AssetKind: assets.AssetKindSkill, DefinitionID: "skill-1", RevisionID: "revision-1", RequiredEvaluationIDs: []string{}}},
		Evaluations: []assets.EvaluationRecord{}, Bindings: []assets.EvolutionAssetBindingRecord{}, Materializations: []assets.RuntimeSkillMaterializationRecord{},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.switchScreen(indexOfScreen(ScreenAssets))
	updated, _ := model.Update(evolutionAssetsLoadedMsg{snapshot: client.assetSnapshot})
	model = updated.(Model)
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	model = updated.(Model)
	if command != nil || model.pendingEvolutionAction != "activate" || len(client.assetCommands) != 0 {
		t.Fatalf("unconfirmed state=%q calls=%d", model.pendingEvolutionAction, len(client.assetCommands))
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.pendingEvolutionAction != "" || len(client.assetCommands) != 0 {
		t.Fatalf("cancel wrote: pending=%q calls=%d", model.pendingEvolutionAction, len(client.assetCommands))
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	model = updated.(Model)
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = updated.(Model)
	if command == nil {
		t.Fatal("confirmed activation command = nil")
	}
	message := command()
	if _, ok := message.(evolutionAssetCommittedMsg); !ok {
		t.Fatalf("confirmed message = %#v", message)
	}
	if len(client.assetCommands) != 1 || client.assetCommands[0].Action != "activate" || client.assetCommands[0].ExpectedStreamHeads == nil {
		t.Fatalf("asset commands = %#v", client.assetCommands)
	}
	var input struct {
		EvaluationIDs []string `json:"evaluation_ids"`
	}
	if err := json.Unmarshal(client.assetCommands[0].Input, &input); err != nil || input.EvaluationIDs == nil {
		t.Fatalf("activation input=%s err=%v", client.assetCommands[0].Input, err)
	}
}

func TestP3ATUIEvaluationAndBindingUseClosedAuthoritativeCommands(t *testing.T) {
	digest := strings.Repeat("a", 64)
	subject := app.EvolutionAssetBindingSubject{
		SubjectIdentity: assets.SubjectIdentity{
			SubjectKind: "work_package", SubjectID: "work-package.coding",
			SubjectVersion: 1, SubjectDigest: digest, Scope: "builtin",
		},
		SubjectIdentityDigest: strings.Repeat("b", 64),
	}
	client := &fakeAssetReadClient{assetSnapshot: api.EvolutionAssetSnapshot{
		ViewVersion: digest,
		Definitions: []assets.SkillDefinition{{
			DefinitionID: "skill-1", Name: "Skill", LatestRevisionID: "revision-1",
			ActiveRevisionID: "revision-1", Lifecycle: assets.LifecycleActive,
		}},
		Revisions: []assets.SkillRevision{{
			AssetKind: assets.AssetKindSkill, DefinitionID: "skill-1",
			RevisionID: "revision-1", ArtifactDigest: digest,
			SourceScope: assets.SourceScopeLocal, Lifecycle: assets.LifecycleActive,
		}},
		Candidates: []assets.EvolutionCandidate{{
			CandidateID: "candidate-1", AssetKind: assets.AssetKindSkill,
			DefinitionID: "skill-1", RevisionID: "revision-1",
			RequiredEvaluationIDs: []string{},
		}},
		Evaluations: []assets.EvaluationRecord{}, Bindings: []assets.EvolutionAssetBindingRecord{},
		Materializations: []assets.RuntimeSkillMaterializationRecord{},
		BindingSubjects:  []app.EvolutionAssetBindingSubject{subject},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.evolutionAssets = client.assetSnapshot
	if message := model.commitSelectedEvolutionAction("evaluate")(); message == nil {
		t.Fatal("evaluate message = nil")
	}
	if len(client.assetCommands) != 1 || client.assetCommands[0].Action != "record_evaluation" {
		t.Fatalf("evaluation command = %#v", client.assetCommands)
	}
	var evaluation map[string]any
	if err := json.Unmarshal(client.assetCommands[0].Input, &evaluation); err != nil ||
		evaluation["fixture_digest"] == "" || evaluation["requested_case_ids"] == nil {
		t.Fatalf("evaluation input = %s, %v", client.assetCommands[0].Input, err)
	}
	if message := model.commitSelectedEvolutionAction("bind")(); message == nil {
		t.Fatal("bind message = nil")
	}
	if len(client.assetCommands) != 2 || client.assetCommands[1].Action != "set_binding" {
		t.Fatalf("binding command = %#v", client.assetCommands)
	}
	var binding map[string]any
	if err := json.Unmarshal(client.assetCommands[1].Input, &binding); err != nil ||
		binding["subject_identity_digest"] != subject.SubjectIdentityDigest ||
		binding["asset_revision_set_digest"] == "" {
		t.Fatalf("binding input = %s, %v", client.assetCommands[1].Input, err)
	}
}

func TestP3ATUIPromotionUsesExactAcceptedRunLineage(t *testing.T) {
	digest := strings.Repeat("a", 64)
	evidenceDigest := strings.Repeat("b", 64)
	client := &fakeAssetReadClient{assetSnapshot: api.EvolutionAssetSnapshot{
		ViewVersion: digest,
		Definitions: []assets.SkillDefinition{}, Revisions: []assets.SkillRevision{},
		Candidates: []assets.EvolutionCandidate{}, Evaluations: []assets.EvaluationRecord{},
		Bindings:         []assets.EvolutionAssetBindingRecord{},
		Materializations: []assets.RuntimeSkillMaterializationRecord{},
		BindingSubjects:  []app.EvolutionAssetBindingSubject{},
		PromotionSources: []app.EvolutionAssetPromotionSource{{
			RunID: "run-accepted", RunGeneration: 3, RunDigest: digest,
			EvidenceIDs: []string{"evidence-1"}, EvidenceDigests: []string{evidenceDigest},
		}},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.evolutionAssets = client.assetSnapshot
	message := model.commitEvolutionPromotion()()
	if _, ok := message.(evolutionAssetCommittedMsg); !ok {
		t.Fatalf("promotion message = %#v", message)
	}
	if len(client.assetCommands) != 1 || client.assetCommands[0].Action != "promote_run" {
		t.Fatalf("promotion command = %#v", client.assetCommands)
	}
	var input map[string]any
	if err := json.Unmarshal(client.assetCommands[0].Input, &input); err != nil {
		t.Fatal(err)
	}
	if input["source_run_id"] != "run-accepted" || input["source_run_digest"] != digest ||
		input["redacted_summary_digest"] == "" || input["candidate_id"] == "" {
		t.Fatalf("promotion input = %s", client.assetCommands[0].Input)
	}
}

func TestP3ATUICreatesReviewedImportAndAllTemplateKindsThroughClosedIPC(t *testing.T) {
	digest := strings.Repeat("a", 64)
	client := &fakeAssetReadClient{assetSnapshot: api.EvolutionAssetSnapshot{
		ViewVersion: digest, Definitions: []assets.SkillDefinition{}, Revisions: []assets.SkillRevision{},
		Candidates: []assets.EvolutionCandidate{}, Evaluations: []assets.EvaluationRecord{},
		Bindings: []assets.EvolutionAssetBindingRecord{}, Materializations: []assets.RuntimeSkillMaterializationRecord{},
		BindingSubjects: []app.EvolutionAssetBindingSubject{}, PromotionSources: []app.EvolutionAssetPromotionSource{},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.evolutionAssets = client.assetSnapshot
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "template.md")
	if err := os.WriteFile(sourcePath, []byte("# Reviewed source\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		mode       string
		wantAction string
		wantKind   string
	}{
		{"import_skill", "import_skill", ""},
		{"agent_template", "create_template", "agent_template"},
		{"team_template", "create_template", "team_template"},
		{"work_package_template", "create_template", "work_package_template"},
		{"recovery_strategy_template", "create_template", "recovery_strategy_template"},
	} {
		model.evolutionCreateMode = test.mode
		message := model.createEvolutionAsset(sourcePath)()
		if _, ok := message.(evolutionAssetCommittedMsg); !ok {
			t.Fatalf("mode %s message = %#v", test.mode, message)
		}
		request := client.assetCommands[len(client.assetCommands)-1]
		if request.Action != test.wantAction {
			t.Fatalf("mode %s action = %s", test.mode, request.Action)
		}
		var input map[string]any
		if err := json.Unmarshal(request.Input, &input); err != nil {
			t.Fatal(err)
		}
		if test.wantKind == "" {
			if input["candidate_id"] == "" || input["external_source_digest"] == "" {
				t.Fatalf("import input = %s", request.Input)
			}
		} else if input["asset_kind"] != test.wantKind || input["template_output"] == "" ||
			input["parameter_schema_digest"] == "" {
			t.Fatalf("template input = %s", request.Input)
		}
	}
}

func (client *fakeReadClient) ProposeSideTask(_ context.Context, request app.SideTaskProposalRequest) (app.SideTaskProposalResult, error) {
	client.proposals = append(client.proposals, request)
	return client.proposal, nil
}

func (client *fakeReadClient) CreateSideTask(_ context.Context, request app.SideTaskCreateRequest) (app.SideTaskCreateResult, error) {
	client.creates = append(client.creates, request)
	return client.created, nil
}

func (client *fakeReadClient) ReadSideTask(context.Context, app.SideTaskReadRequest) (app.SideTaskReadResult, error) {
	return app.SideTaskReadResult{}, nil
}

func (client *fakeReadClient) DecideSideTask(_ context.Context, request app.SideTaskDecisionRequest) (app.SideTaskDecisionResult, error) {
	client.decisions = append(client.decisions, request)
	return client.decided, nil
}

func (client *fakeReadClient) ExecuteMission(
	_ context.Context,
	command app.MissionExecutionCommand,
) (api.MissionExecutionEnvelope, error) {
	client.executions = append(client.executions, command)
	return client.execution, client.executionErr
}

func testMission(
	teamID string,
	title string,
	lane api.MissionLane,
	status string,
) api.LocalProductMissionSummary {
	return api.LocalProductMissionSummary{
		SchemaVersion:  1,
		MissionID:      "mission/" + teamID,
		TeamInstanceID: teamID,
		Title:          title,
		Lane:           lane,
		Status:         status,
		NodeCount:      1,
		TeamPulse:      []api.LocalProductMissionPulse{},
		Topology:       []api.LocalProductMissionNode{},
	}
}

func TestMissionWorkbenchStartsOnBoardAndUsesExactNavigation(t *testing.T) {
	if len(screens) == 0 || screens[0] != ScreenBoard {
		t.Fatalf("initial screen = %v, want %v", screens, ScreenBoard)
	}
	for _, screen := range screens {
		if screen == Screen("Tasks") {
			t.Fatalf("legacy Tasks screen remains: %q", screen)
		}
	}
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 2,
		ViewVersion:   strings.Repeat("a", 64),
		Teams: []api.LocalProductTeamSummary{{
			TeamInstanceID: "team-1",
			DisplayName:    "Release Team",
		}},
		Missions: []api.LocalProductMissionSummary{{
			MissionID:      "mission/team-1",
			TeamInstanceID: "team-1",
			Title:          "mission/team-1",
			SourceKind:     "saved_team",
			Lane:           api.MissionLaneOrchestrating,
			Status:         "human_required",
			Priority:       "normal",
			NodeCount:      1,
			AttentionCount: 1,
			CurrentNodeID:  "node-internal-9",
			LastMilestone:  "Human decision required",
			TeamPulse: []api.LocalProductMissionPulse{{
				Role:          "main",
				State:         "waiting",
				NodeID:        "node-internal-9",
				AttemptNumber: 1,
			}},
			Topology: []api.LocalProductMissionNode{{
				LogicalNodeID: "node-internal-9",
				Title:         "Verify release",
				Role:          "main",
			}},
		}},
		PreparedDecisions: []app.MissionDecisionCommand{{
			SchemaVersion:  1,
			Operation:      "read",
			Kind:           "authorization",
			Action:         "read",
			MissionID:      "mission/team-1",
			TeamInstanceID: "team-1",
			LogicalNodeID:  "main",
			AttemptNumber:  1,
		}},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: client.snapshot})
	model = updated.(Model)
	if view := model.View(); !strings.Contains(view, "Proposed") ||
		!strings.Contains(view, "Orchestrating") ||
		!strings.Contains(view, "Mission Detail · Release Team") ||
		!strings.Contains(view, "Team · Main · Waiting · Attempt 1") ||
		!strings.Contains(view, "Current node · Verify release") ||
		!strings.Contains(view, "Decision · Authorization prepared") ||
		strings.Contains(view, "› New Mission") ||
		strings.Contains(view, "Needs You lane") ||
		strings.Contains(view, "mission/team-1") ||
		strings.Contains(view, "node-internal-9") {
		t.Fatalf("Board lifecycle = %q", view)
	}
	exportTUISnapshotIfRequested(t, model.View())
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("g")},
		{Type: tea.KeyRunes, Runes: []rune("t")},
	} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	if model.Screen() != ScreenMission {
		t.Fatalf("g t screen = %q, want %q", model.Screen(), ScreenMission)
	}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("g")},
		{Type: tea.KeyRunes, Runes: []rune("b")},
	} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	if model.Screen() != ScreenBoard {
		t.Fatalf("g b screen = %q, want %q", model.Screen(), ScreenBoard)
	}
}

func TestMissionSideTaskUsesZeroWriteProposalExplicitConfirmAndTypedDecision(t *testing.T) {
	viewVersion := strings.Repeat("a", 64)
	handoffDigest := strings.Repeat("b", 64)
	client := &fakeReadClient{
		snapshot: api.LocalProductSnapshot{
			SchemaVersion: 3, ViewVersion: viewVersion,
			Missions: []api.LocalProductMissionSummary{testMission("team-1", "Release", api.MissionLaneComplete, "succeeded")},
			Runs:     []api.LocalProductRunSummary{{RunID: "run-1", WorkItemID: "work-1", ClaimGeneration: 2}},
			SideTasks: []api.LocalProductSideTaskSummary{{
				SideTaskID: "side-1", ParentMissionID: "mission/team-1",
				ParentTeamInstanceID: "team-1", ParentTaskID: "work-1", ParentRunID: "run-1",
				ParentClaimGeneration: 2, ParentExecutionDigest: strings.Repeat("f", 64),
				Purpose: "research", Mode: "decision_required",
				Title: "Check release", Status: "decision_required", SourceGeneration: 1,
				HandoffVersion: 1, HandoffDigest: handoffDigest,
				AvailableDecisions: []string{"discard"}, EffectStatus: "none",
			}},
		},
		timeline: api.LocalProductTimelinePage{
			TeamInstanceID: "team-1", ViewVersion: viewVersion,
			Board: api.LocalProductTeamBoard{SchemaVersion: 1, TeamInstanceID: "team-1", PlanDigest: strings.Repeat("c", 64), Status: "succeeded", ViewVersion: viewVersion, Nodes: []api.NodeBoardRow{{LogicalNodeID: "main", Status: "succeeded", CurrentAttempt: 1, WorkItemID: "work-1", RunID: "run-1"}}},
		},
		proposal: app.SideTaskProposalResult{SchemaVersion: 1, Status: "proposal", ProposalDigest: strings.Repeat("d", 64), ViewVersion: viewVersion, Purpose: "research", Mode: "report_only", Title: "Side task", PermissionScopes: []string{}, RequiresConfirmation: true},
		created:  app.SideTaskCreateResult{SchemaVersion: 1, SideTaskID: "side-created", Status: "report_delivered", ViewVersion: viewVersion, SideExecutionTeamInstanceID: "team-side", ProposalDigest: strings.Repeat("d", 64), HandoffVersion: 1, HandoffDigest: strings.Repeat("e", 64)},
		decided:  app.SideTaskDecisionResult{SchemaVersion: 1, SideTaskID: "side-1", Decision: "discard", Status: "decided", EffectStatus: "none", ViewVersion: viewVersion},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.snapshot = client.snapshot
	model.timeline = client.timeline
	model.currentMission = "mission/team-1"
	model.currentTeam = "team-1"
	model.missionResult = app.MissionExecutionResult{MissionID: "mission/team-1", TeamInstanceID: "team-1", ExecutionDigest: strings.Repeat("f", 64)}
	model.switchScreen(indexOfScreen(ScreenMission))

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Check one bounded fact")})
	model = updated.(Model)
	updated, proposalCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if proposalCommand == nil {
		t.Fatal("missing zero-write proposal command")
	}
	proposalMessage := proposalCommand()
	if len(client.proposals) != 1 || client.proposals[0].Operation != "propose" ||
		client.proposals[0].ParentTaskID != "work-1" ||
		client.proposals[0].ParentExecutionDigest != strings.Repeat("f", 64) {
		t.Fatalf("proposal calls=%#v", client.proposals)
	}
	updated, _ = model.Update(proposalMessage)
	model = updated.(Model)
	updated, createCommand := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	model = updated.(Model)
	if createCommand == nil {
		t.Fatal("explicit confirm did not create")
	}
	createdMessage := createCommand()
	if len(client.creates) != 1 || !client.creates[0].Confirmed || client.creates[0].ProposalDigest != strings.Repeat("d", 64) {
		t.Fatalf("create=%#v", client.creates)
	}
	updated, _ = model.Update(createdMessage)
	model = updated.(Model)
	model.snapshot = client.snapshot
	// A reconnected TUI has the authoritative Side-task projection and current
	// Run/board, but no transient start envelope in memory.
	model.missionResult = app.MissionExecutionResult{}
	updated, decisionCommand := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if decisionCommand == nil {
		t.Fatal("typed decision unavailable")
	}
	_ = updated
	_ = decisionCommand()
	if len(client.decisions) != 1 || client.decisions[0].Decision != "discard" || client.decisions[0].ParentExecutionDigest != strings.Repeat("f", 64) {
		t.Fatalf("decision=%#v", client.decisions)
	}
}

func TestTeamBuilderRuntimeDisplayNameFailsClosed(t *testing.T) {
	model, err := NewModel(&fakeSetupClient{})
	if err != nil {
		t.Fatal(err)
	}
	model.loading = false
	model.screenIndex = indexOfScreen(ScreenTeamBuilder)
	model.setup = app.SetupSnapshot{Runtimes: []app.SetupRuntimePreview{{
		RuntimeInstanceID: "runtime-internal-setup",
		DisplayName:       "runtime-internal-setup",
		ExecutableVersion: "0.82.1",
		Status:            "online",
	}}}
	view := model.View()
	if !strings.Contains(view, "Runtime · Runtime unavailable · Online · 0.82.1") ||
		strings.Contains(view, "runtime-internal-setup") {
		t.Fatalf("Team Builder exposed setup Runtime identity: %q", view)
	}
}

func TestNewMissionRequiresExactPreflightBeforeExplicitStart(t *testing.T) {
	viewVersion := strings.Repeat("a", 64)
	freshViewVersion := strings.Repeat("f", 64)
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 2,
		ViewVersion:   viewVersion,
		Teams: []api.LocalProductTeamSummary{{
			TeamInstanceID: "team-internal-1",
			DisplayName:    "Release Crew",
			SourceKind:     "saved_team",
			State:          "created",
			Confirmed:      true,
			Executable:     true,
		}},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: client.snapshot})
	model = updated.(Model)
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command != nil || model.Screen() != ScreenNewMission {
		t.Fatalf("new Mission screen=%q command=%v", model.Screen(), command)
	}
	view := model.View()
	for _, want := range []string{
		"New Mission", "Release Crew", "Coding", "Nothing runs before Start",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("new Mission missing %q: %q", want, view)
		}
	}
	if strings.Contains(view, "team-internal-1") {
		t.Fatalf("new Mission exposed internal Team ID: %q", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.entryMode != entryMissionObjective {
		t.Fatalf("objective entry mode = %q", model.entryMode)
	}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("Ship the reviewed release")},
		{Type: tea.KeyEnter},
	} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	if model.missionObjective != "Ship the reviewed release" {
		t.Fatalf("objective = %q", model.missionObjective)
	}
	client.snapshot.ViewVersion = freshViewVersion

	updated, command = model.Update(tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune("p"),
	})
	model = updated.(Model)
	if command == nil {
		t.Fatal("preflight did not call mission_execution")
	}
	message := command()
	if len(client.executions) != 1 {
		t.Fatalf("execution calls = %d, want 1", len(client.executions))
	}
	preflightCommand := client.executions[0]
	if preflightCommand.Operation != "preflight" ||
		preflightCommand.MissionID != "mission/team-internal-1" ||
		preflightCommand.TeamInstanceID != "team-internal-1" ||
		preflightCommand.ExpectedViewVersion != freshViewVersion ||
		preflightCommand.Objective != "Ship the reviewed release" ||
		preflightCommand.PreflightDigest != "" ||
		preflightCommand.WorkPackageID != "work-package.coding" ||
		len(preflightCommand.WorkPackageDigest) != 64 {
		t.Fatalf("preflight command = %#v", preflightCommand)
	}
	if len(preflightCommand.CorrelationID) != 36 {
		t.Fatalf("preflight correlation = %q", preflightCommand.CorrelationID)
	}
	client.execution = api.MissionExecutionEnvelope{}
	preflight := app.MissionExecutionPreflight{
		SchemaVersion:     1,
		MissionID:         preflightCommand.MissionID,
		TeamInstanceID:    preflightCommand.TeamInstanceID,
		WorkPackageID:     preflightCommand.WorkPackageID,
		WorkPackageDigest: preflightCommand.WorkPackageDigest,
		ViewVersion:       freshViewVersion,
		PlanDigest:        strings.Repeat("b", 64),
		PreflightDigest:   strings.Repeat("c", 64),
		RuntimeInstanceID: "runtime-internal-1",
		RuntimeProfileID:  "profile-internal-1",
		ModelID:           "model-1",
		AuthMode:          "native_auth",
		CapacityAvailable: 1,
		BudgetStatus:      "unavailable",
		PermissionScopes:  []string{"repo.read"},
		ApprovalPoints:    []string{"terminal_review"},
		Nodes: []app.MissionExecutionNodePreview{{
			LogicalNodeID: "main",
			Title:         "Ship the reviewed release",
			Role:          "main",
			DependsOn:     []string{},
			MaxAttempts:   2,
		}},
	}
	message = missionPreflightedMsg{preflight: preflight}
	updated, _ = model.Update(message)
	model = updated.(Model)
	view = model.View()
	for _, want := range []string{
		"Preflight ready", "model-1", "Native auth", "1 available",
		"Repo read", "Terminal review", "s Start",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("preflight missing %q: %q", want, view)
		}
	}

	client.execution = api.MissionExecutionEnvelope{
		SchemaVersion: 1,
		Operation:     "start",
		Result: &app.MissionExecutionResult{
			SchemaVersion:   1,
			MissionID:       preflight.MissionID,
			TeamInstanceID:  preflight.TeamInstanceID,
			Status:          "running",
			ViewVersion:     strings.Repeat("d", 64),
			ExecutionDigest: strings.Repeat("e", 64),
		},
	}
	updated, command = model.Update(tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune("s"),
	})
	model = updated.(Model)
	if command == nil {
		t.Fatal("explicit Start did not call mission_execution")
	}
	message = command()
	if len(client.executions) != 2 {
		t.Fatalf("execution calls = %d, want 2", len(client.executions))
	}
	startCommand := client.executions[1]
	if startCommand.Operation != "start" ||
		startCommand.MissionID != preflight.MissionID ||
		startCommand.PreflightDigest != preflight.PreflightDigest ||
		startCommand.CorrelationID == preflightCommand.CorrelationID {
		t.Fatalf("start command = %#v", startCommand)
	}
	updated, refresh := model.Update(message)
	model = updated.(Model)
	if refresh == nil || model.currentMission != preflight.MissionID ||
		model.currentTeam != preflight.TeamInstanceID {
		t.Fatalf("started model = %#v refresh=%v", model, refresh)
	}
}

func TestMissionTimelineLabelsTentativeOutputAndMilestones(t *testing.T) {
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 2,
		ViewVersion:   strings.Repeat("a", 64),
		Missions: []api.LocalProductMissionSummary{
			testMission("team-1", "Release", api.MissionLaneOrchestrating, "running"),
		},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.currentMission = "mission/team-1"
	model.currentTeam = "team-1"
	model.screenIndex = indexOfScreen(ScreenMission)
	model.loading = false
	model.snapshot = client.snapshot
	model.timeline = api.LocalProductTimelinePage{Records: []api.LocalProductTimelineRecord{
		{Kind: "tentative_output", Payload: api.LocalProductTimelinePayload{TextDelta: "Compiling checks"}},
		{Kind: "retry", Payload: api.LocalProductTimelinePayload{Status: "retry_scheduled"}},
		{Kind: "terminal", Payload: api.LocalProductTimelinePayload{Status: "succeeded"}},
	}}
	view := model.View()
	for _, want := range []string{
		"Tentative output · Compiling checks", "Retry scheduled", "Succeeded",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("Mission timeline missing %q: %q", want, view)
		}
	}
}

func TestMissionCancelUsesOnlyCurrentAuthoritativeLineage(t *testing.T) {
	viewVersion := strings.Repeat("a", 64)
	mission := testMission(
		"team-1", "Release", api.MissionLaneOrchestrating, "running",
	)
	mission.CurrentNodeID = "main"
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 2, ViewVersion: viewVersion,
		Missions: []api.LocalProductMissionSummary{mission},
		Runs: []api.LocalProductRunSummary{{
			RunID: "run-1", ClaimGeneration: 3,
		}},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.loading = false
	model.snapshot = client.snapshot
	model.currentMission = mission.MissionID
	model.currentTeam = mission.TeamInstanceID
	model.screenIndex = indexOfScreen(ScreenMission)
	model.missionResult = app.MissionExecutionResult{
		SchemaVersion: 1, MissionID: mission.MissionID,
		TeamInstanceID: mission.TeamInstanceID, Status: "running",
		ViewVersion: viewVersion, ExecutionDigest: strings.Repeat("e", 64),
	}
	model.timeline = api.LocalProductTimelinePage{
		SchemaVersion: 1, TeamInstanceID: mission.TeamInstanceID,
		ViewVersion: viewVersion,
		Board: api.LocalProductTeamBoard{
			SchemaVersion: 1, TeamInstanceID: mission.TeamInstanceID,
			Status: "running", ViewVersion: viewVersion,
			Nodes: []api.NodeBoardRow{{
				LogicalNodeID: "main", Status: "running",
				CurrentAttempt: 2, RunID: "run-1",
			}},
		},
	}
	if view := model.View(); !strings.Contains(view, "c Cancel Mission") {
		t.Fatalf("cancel control unavailable: %q", view)
	}
	client.execution = api.MissionExecutionEnvelope{
		SchemaVersion: 1, Operation: "control",
		Result: &app.MissionExecutionResult{
			SchemaVersion: 1, MissionID: mission.MissionID,
			TeamInstanceID: mission.TeamInstanceID, Status: "cancelled",
			ViewVersion: viewVersion, ExecutionDigest: strings.Repeat("e", 64),
		},
	}
	updated, command := model.Update(tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune("c"),
	})
	model = updated.(Model)
	if command == nil {
		t.Fatal("cancel did not call mission_execution")
	}
	message := command()
	if len(client.executions) != 1 {
		t.Fatalf("control calls = %d", len(client.executions))
	}
	control := client.executions[0]
	if control.Operation != "control" || control.ControlAction != "cancel" ||
		control.MissionID != mission.MissionID ||
		control.TeamInstanceID != mission.TeamInstanceID ||
		control.ExpectedViewVersion != viewVersion ||
		control.ExecutionDigest != model.missionResult.ExecutionDigest ||
		control.LogicalNodeID != "main" || control.AttemptNumber != 2 ||
		control.ClaimGeneration != 3 {
		t.Fatalf("cancel command = %#v", control)
	}
	updated, refresh := model.Update(message)
	if refresh == nil || updated.(Model).missionResult.Status != "cancelled" {
		t.Fatalf("cancel response = %#v refresh=%v", updated, refresh)
	}
}

func exportTUISnapshotIfRequested(t *testing.T, view string) {
	t.Helper()
	requested := os.Getenv("LOOM_TUI_SNAPSHOT_PATH")
	if requested == "" {
		return
	}
	absolute, err := filepath.Abs(requested)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.Abs(filepath.Join(
		"..",
		"..",
		".loom-evidence",
		"phase2a",
		"P2A-W2",
		"tui-mission-board.txt",
	))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(absolute) != filepath.Clean(expected) {
		t.Fatalf("TUI evidence path = %q", absolute)
	}
	if info, err := os.Lstat(filepath.Dir(absolute)); err != nil ||
		info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("TUI evidence parent is not a real directory: %v", err)
	}
	if err := os.WriteFile(absolute, []byte(view), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMissionApprovalKeyOpensReadOnlyDecisionWhenCommandIsNotPrepared(
	t *testing.T,
) {
	mission := testMission(
		"team-1",
		"Ship reviewed change",
		api.MissionLaneOrchestrating,
		"human_required",
	)
	mission.AttentionCount = 1
	mission.TeamPulse = []api.LocalProductMissionPulse{{
		Role:          "main",
		State:         "waiting",
		NodeID:        "main",
		AttemptNumber: 1,
	}}
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 2,
		ViewVersion:   strings.Repeat("a", 64),
		Missions:      []api.LocalProductMissionSummary{mission},
		Attention: []api.AttentionItem{{
			SchemaVersion:     1,
			AttentionID:       "attention-1",
			Kind:              "approval_required",
			Severity:          "warning",
			TeamInstanceID:    "team-1",
			LogicalNodeID:     "main",
			ApprovalRequestID: "approval-1",
			Status:            "pending",
			ActionRequired:    "approve or deny",
		}},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: client.snapshot})
	model = updated.(Model)
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyDown},
		{Type: tea.KeyRunes, Runes: []rune("g")},
		{Type: tea.KeyRunes, Runes: []rune("t")},
	} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	if view := model.View(); !strings.Contains(
		view,
		"Team Pulse · Main · Attempt 1 · Waiting",
	) || strings.Contains(
		view,
		"approval-1",
	) {
		t.Fatalf("Mission semantic view = %q", view)
	}
	updated, _ = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("a"),
	})
	model = updated.(Model)
	view := model.View()
	if !strings.Contains(view, "Authorization Decision") ||
		!strings.Contains(view, "mutation actions disabled") ||
		!strings.Contains(view, "Esc · Not now") {
		t.Fatalf("decision view = %q", view)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.Screen() != ScreenMission || model.decisionOpen {
		t.Fatalf(
			"Esc did not restore Mission: screen=%q open=%v",
			model.Screen(),
			model.decisionOpen,
		)
	}
}

func TestInteractionContinuityStartsWithBoardInsteadOfHome(t *testing.T) {
	for _, screen := range screens {
		switch screen {
		case ScreenHome, ScreenRuntimes, ScreenTeams:
			t.Fatalf("object/dashboard-first primary screen remains: %q", screen)
		}
	}
}

func TestTaskSelectionSurvivesPrimaryViewSwitches(t *testing.T) {
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 2,
		ViewVersion:   strings.Repeat("a", 64),
		Missions: []api.LocalProductMissionSummary{
			testMission(
				"team-1",
				"Release review",
				api.MissionLaneReady,
				"ready",
			),
		},
		Teams: []api.LocalProductTeamSummary{{
			TeamInstanceID: "team-1",
			DisplayName:    "Release review",
			State:          "ready",
		}},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: client.snapshot})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.selected != 1 {
		t.Fatalf("task selection=%d, want 1", model.selected)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model = updated.(Model)
	if model.Screen() != ScreenBoard || model.selected != 1 {
		t.Fatalf(
			"restored screen=%s selection=%d",
			model.Screen(),
			model.selected,
		)
	}
}

func TestTasksFilterWithoutReplacingCurrentSelection(t *testing.T) {
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 2,
		ViewVersion:   strings.Repeat("a", 64),
		Missions: []api.LocalProductMissionSummary{
			testMission(
				"team-alpha",
				"Alpha review",
				api.MissionLaneReady,
				"ready",
			),
			testMission(
				"team-beta",
				"Beta migration",
				api.MissionLaneReady,
				"ready",
			),
		},
		Teams: []api.LocalProductTeamSummary{
			{
				TeamInstanceID: "team-alpha",
				DisplayName:    "Alpha review",
				State:          "ready",
			},
			{
				TeamInstanceID: "team-beta",
				DisplayName:    "Beta migration",
				State:          "ready",
			},
		},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: client.snapshot})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune("/"),
	})
	model = updated.(Model)
	if model.entryMode != "task_search" {
		t.Fatalf("Tasks search mode = %q", model.entryMode)
	}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("beta")},
		{Type: tea.KeyEnter},
	} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	view := model.View()
	if !strings.Contains(view, "Filter · beta") ||
		!strings.Contains(view, "Alpha review") ||
		!strings.Contains(view, "Beta migration") {
		t.Fatalf("filtered Tasks did not preserve selection: %q", view)
	}
}

func TestTeamBuilderPreflightShowsBoundProviderModelAndLimits(t *testing.T) {
	client := &fakeSetupClient{}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.screenIndex = indexOfScreen(ScreenTeamBuilder)
	model.loading = false
	model.builder = app.BuilderSessionView{
		DraftID:    "draft-1",
		CanConfirm: true,
		Preview: app.BuilderPreview{
			Name:                 "Release team",
			Purpose:              "Review the release",
			Permissions:          []string{"repo.read"},
			MaximumBudgetCredits: 200,
			EstimatedMaximumCost: "Up to 2 credits",
			Roles: []app.BuilderRolePreview{{
				Kind:        "main",
				DisplayName: "Coordinator",
				Runtime: app.SetupRuntimePreview{
					DisplayName: "Pi Coding Agent",
				},
				ModelID:       "codex-model",
				AuthMode:      "native_auth",
				PermissionIDs: []string{"repo.read"},
				Compatible:    true,
			}},
		},
	}
	model.setup = app.SetupSnapshot{
		Runtimes: []app.SetupRuntimePreview{{
			RuntimeInstanceID: "runtime-pi",
			DisplayName:       "Pi Coding Agent",
			AdapterType:       "pi",
			ModelID:           "codex-model",
			ModelIDs:          []string{"codex-model"},
		}, {
			RuntimeInstanceID: "runtime-alt",
			DisplayName:       "Alternate Runtime",
			AdapterType:       "pi",
			ModelID:           "alternate-model",
			ModelIDs:          []string{"alternate-model"},
		}},
		RoleOptions: []app.SetupRoleOptionPreview{
			{
				ID:                "coordinator",
				Kind:              "main",
				AgentDefinitionID: "agent-main",
				RuntimeProfileID:  "profile",
				RuntimeInstanceID: "runtime-pi",
				Responsibility:    "Coordinate",
			},
			{
				ID:                "reviewer",
				Kind:              "main",
				AgentDefinitionID: "agent-main",
				RuntimeProfileID:  "profile-alt",
				RuntimeInstanceID: "runtime-alt",
				Responsibility:    "Review",
			},
		},
	}
	model.builder.Preview.Roles[0].AgentDefinitionID = "agent-main"
	model.builder.Preview.Roles[0].RuntimeProfileID = "profile"
	model.builder.Preview.Roles[0].Runtime.RuntimeInstanceID = "runtime-pi"
	view := model.View()
	for _, want := range []string{
		"Coordinator",
		"Codex",
		"codex-model",
		"Native auth",
		"Repo read",
		"Compatible",
		"200 credits",
		"Up to 2 credits",
		"Main role choices",
		"m change Main role",
		"Review · alternate-model · Select to review provider and sign-in",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("preflight missing %q: %q", want, view)
		}
	}
	if strings.Contains(
		view,
		"Review · Local provider · alternate-model · Native",
	) {
		t.Fatalf("unselected role invented provider/auth: %q", view)
	}

	updated, command := model.Update(tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune("m"),
	})
	if command == nil {
		t.Fatal("Main role change did not call builder_edit")
	}
	model = updated.(Model)
	_, _ = model.Update(command())
	if client.edits != 1 ||
		client.lastEdit.Field != "main_role" ||
		client.lastEdit.Value != "reviewer" {
		t.Fatalf(
			"role edits=%d command=%#v",
			client.edits,
			client.lastEdit,
		)
	}
}

func (client *fakeReadClient) Snapshot(
	context.Context,
	api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	return client.snapshot, client.err
}

func (client *fakeReadClient) TimelinePage(
	context.Context,
	api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	client.timelineCalls++
	return client.timeline, client.err
}

func TestModelTruthfullyHandlesJournalWithNoTeams(t *testing.T) {
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("a", 64),
		Runtimes: []api.LocalProductRuntimeSummary{{
			RuntimeInstanceID: "runtime.pi",
			DisplayName:       "Pi 0.82.1",
			Status:            "online",
		}},
		Teams:     []api.LocalProductTeamSummary{},
		Runs:      []api.LocalProductRunSummary{},
		Evidence:  []api.LocalProductEvidenceSummary{},
		Attention: []api.AttentionItem{},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, command := model.Update(snapshotLoadedMsg{
		snapshot: client.snapshot,
	})
	if command != nil {
		t.Fatal("snapshot load produced unexpected command")
	}
	model = updated.(Model)
	if view := model.View(); !strings.Contains(view, "New Mission") {
		t.Fatalf("Board did not offer a new Mission: %q", view)
	}

	model.screenIndex = indexOfScreen(ScreenTimeline)
	if view := model.View(); !strings.Contains(
		view,
		"Choose a task to inspect its activity.",
	) {
		t.Fatalf("direct Timeline empty state = %q", view)
	}
	if client.timelineCalls != 0 {
		t.Fatalf(
			"direct Timeline screen made %d requests",
			client.timelineCalls,
		)
	}

	transitionClient := &fakeReadClient{
		snapshot: api.LocalProductSnapshot{
			SchemaVersion: 2,
			ViewVersion:   strings.Repeat("b", 64),
			Missions: []api.LocalProductMissionSummary{
				testMission(
					"team-previous",
					"Previous Team",
					api.MissionLaneReady,
					"ready",
				),
			},
			Teams: []api.LocalProductTeamSummary{{
				TeamInstanceID: "team-previous",
				DisplayName:    "Previous Team",
				SourceKind:     "saved_team",
			}},
		},
		timeline: api.LocalProductTimelinePage{
			SchemaVersion:  1,
			TeamInstanceID: "team-previous",
			ViewVersion:    strings.Repeat("b", 64),
			Records: []api.LocalProductTimelineRecord{{
				Kind: "terminal",
			}},
		},
	}
	transition, err := NewModel(transitionClient)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ = transition.Update(snapshotLoadedMsg{
		snapshot: transitionClient.snapshot,
	})
	transition = updated.(Model)
	transition.screenIndex = indexOfScreen(ScreenBoard)
	transition.selected = 1
	updated, command = transition.Update(tea.KeyMsg{Type: tea.KeyEnter})
	transition = updated.(Model)
	if command == nil {
		t.Fatal("selected Team did not request its timeline")
	}
	updated, _ = transition.Update(command())
	transition = updated.(Model)
	if transitionClient.timelineCalls != 1 {
		t.Fatalf(
			"selected Team timeline calls = %d",
			transitionClient.timelineCalls,
		)
	}

	transitionClient.snapshot = client.snapshot
	updated, command = transition.Update(snapshotLoadedMsg{
		snapshot: transitionClient.snapshot,
	})
	transition = updated.(Model)
	if command != nil ||
		transition.currentTeam != "" ||
		transition.timeline.TeamInstanceID != "" ||
		!strings.Contains(
			transition.View(),
			"New Mission",
		) {
		t.Fatalf(
			"zero-Team refresh retained stale selection: %#v view=%q",
			transition,
			transition.View(),
		)
	}
	updated, command = transition.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("r"),
	})
	transition = updated.(Model)
	if command == nil {
		t.Fatal("zero-Team refresh did not request a snapshot")
	}
	updated, _ = transition.Update(command())
	transition = updated.(Model)
	if transitionClient.timelineCalls != 1 {
		t.Fatalf(
			"zero-Team refresh retried stale timeline: calls=%d",
			transitionClient.timelineCalls,
		)
	}
}

func TestModelRendersEveryBoundedScreenAndTeamTimelineInteraction(
	t *testing.T,
) {
	client := &fakeReadClient{
		snapshot: api.LocalProductSnapshot{
			SchemaVersion: 2,
			ViewVersion:   strings.Repeat("c", 64),
			Missions: []api.LocalProductMissionSummary{
				testMission(
					"team-1",
					"Delivery Team",
					api.MissionLaneComplete,
					"succeeded",
				),
			},
			Runtimes: []api.LocalProductRuntimeSummary{{
				RuntimeInstanceID: "runtime-1",
				DisplayName:       "Local Pi",
				Status:            "online",
				ExecutableVersion: "0.82.1",
			}},
			Teams: []api.LocalProductTeamSummary{{
				TeamInstanceID: "team-1",
				DisplayName:    "Delivery Team",
				SourceKind:     "saved_team",
			}},
			Runs: []api.LocalProductRunSummary{{
				RunID:          "run-1",
				Phase:          "terminal",
				TerminalStatus: "succeeded",
			}},
			Evidence: []api.LocalProductEvidenceSummary{{
				EvidenceID: "evidence-1",
				Digest:     strings.Repeat("d", 64),
			}},
			Attention: []api.AttentionItem{{
				Kind:           "human_required",
				ActionRequired: "review",
			}},
		},
		timeline: api.LocalProductTimelinePage{
			SchemaVersion:  1,
			TeamInstanceID: "team-1",
			ViewVersion:    strings.Repeat("e", 64),
			Records: []api.LocalProductTimelineRecord{{
				Kind: "terminal",
				Payload: api.LocalProductTimelinePayload{
					Status:      "succeeded",
					WarningCode: "none",
				},
			}},
			Board: api.LocalProductTeamBoard{
				Nodes: []api.NodeBoardRow{{LogicalNodeID: "main"}},
			},
			Attention: []api.AttentionItem{},
		},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	loaded := model.Init()()
	updated, command := model.Update(loaded)
	if command != nil {
		t.Fatal("snapshot load produced unexpected command")
	}
	model = updated.(Model)

	for _, test := range []struct {
		screen Screen
		text   string
	}{
		{ScreenBoard, "Delivery Team"},
		{ScreenTeamBuilder, "What would you like"},
		{ScreenRuns, "Work 1"},
		{ScreenAttention, "review"},
		{ScreenTimeline, "Choose a task"},
	} {
		model.screenIndex = indexOfScreen(test.screen)
		if view := model.View(); !strings.Contains(view, test.text) {
			t.Fatalf("%s view = %q, want %q", test.screen, view, test.text)
		}
	}

	model.screenIndex = indexOfScreen(ScreenBoard)
	model.selected = 1
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.Screen() != ScreenMission || command == nil {
		t.Fatalf(
			"enter screen=%s command=%v",
			model.Screen(),
			command,
		)
	}
	updated, command = model.Update(command())
	model = updated.(Model)
	if command != nil ||
		!strings.Contains(model.View(), "Succeeded") {
		t.Fatalf("loaded timeline view = %q", model.View())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.Screen() != ScreenBoard {
		t.Fatalf("escape screen = %s", model.Screen())
	}

	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyDown},
		{Type: tea.KeyRunes, Runes: []rune("j")},
		{Type: tea.KeyUp},
		{Type: tea.KeyRunes, Runes: []rune("k")},
		{Type: tea.KeyShiftTab},
		{Type: tea.KeyLeft},
		{Type: tea.KeyRight},
	} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	updated, command = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("?"),
	})
	model = updated.(Model)
	if command != nil || !strings.Contains(model.View(), "Keys:") {
		t.Fatalf("help view = %q", model.View())
	}
	updated, command = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("r"),
	})
	if command == nil {
		t.Fatal("refresh command = nil")
	}
	updated, command = updated.(Model).Update(command())
	if command != nil {
		t.Fatal("refresh result produced command")
	}
	if _, quit := updated.(Model).Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	}); quit == nil {
		t.Fatal("q did not produce quit command")
	}
}

func TestModelMapsRemoteFailuresAndStripsOSCAndIncompleteEscapes(t *testing.T) {
	model, err := NewModel(&fakeReadClient{})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotFailedMsg{err: &localipc.RemoteError{
		Code:        "cursor_conflict",
		Recoverable: true,
	}})
	if view := updated.(Model).View(); !strings.Contains(
		view,
		"cursor_conflict",
	) {
		t.Fatalf("remote failure view = %q", view)
	}
	for _, unsafe := range []string{
		"safe\x1b]2;private-title\x07visible",
		"safe\x1b]2;private-title\x1b\\visible",
		"safe\x1b[31",
		"safe\x1b",
	} {
		sanitized := sanitizeCell(unsafe, 80)
		if strings.Contains(sanitized, "private-title") ||
			strings.ContainsRune(sanitized, '\x1b') {
			t.Fatalf("sanitizeCell(%q) = %q", unsafe, sanitized)
		}
	}
}

func TestModelPresentsRecentWorkWithoutRawInternalIdentifiers(t *testing.T) {
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 2,
		ViewVersion:   strings.Repeat("a", 64),
		Missions: []api.LocalProductMissionSummary{
			testMission(
				"team-1",
				"Team One",
				api.MissionLaneReady,
				"ready",
			),
		},
		Runtimes: []api.LocalProductRuntimeSummary{
			{RuntimeInstanceID: "runtime-1", DisplayName: "Pi source"},
			{RuntimeInstanceID: "runtime-2", DisplayName: "Pi verifier"},
		},
		Runs: []api.LocalProductRunSummary{
			{
				RunID:             "run-1",
				WorkItemID:        "work-1",
				Phase:             "terminal",
				TerminalStatus:    "succeeded",
				RuntimeInstanceID: "runtime-1",
				ClaimGeneration:   1,
			},
			{
				RunID:             "run-2",
				WorkItemID:        "work-2",
				Phase:             "terminal",
				TerminalStatus:    "failed",
				TerminalReason:    "verification_failed",
				RuntimeInstanceID: "runtime-2",
				ClaimGeneration:   2,
			},
		},
		Evidence: []api.LocalProductEvidenceSummary{
			{
				EvidenceID: "evidence-1",
				WorkItemID: "work-1",
				Digest:     strings.Repeat("1", 64),
			},
			{
				EvidenceID: "evidence-2",
				WorkItemID: "work-2",
				Digest:     strings.Repeat("2", 64),
			},
		},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: client.snapshot})
	model = updated.(Model)
	model.screenIndex = indexOfScreen(ScreenRuns)
	view := model.View()
	for _, want := range []string{
		"Work 1",
		"Work 2",
		"Succeeded",
		"Failed",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("Recent work view missing %q: %q", want, view)
		}
	}
	for _, forbidden := range []string{
		"run-1", "run-2", "runtime-1", "runtime-2",
		"evidence-1", "evidence-2",
	} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("Recent work exposed %q: %q", forbidden, view)
		}
	}

	model.compareRuns = []string{"run-1", "run-2"}
	model.screenIndex = indexOfScreen(ScreenCompare)
	view = model.View()
	for _, want := range []string{"Compare", "Pi source", "Pi verifier", "Evidence · 1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("Compare view missing %q: %q", want, view)
		}
	}
	for _, forbidden := range []string{
		"run-1", "run-2", "runtime-1", "runtime-2",
		"evidence-1", "evidence-2", "work-1", "work-2",
	} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("Compare exposed %q: %q", forbidden, view)
		}
	}
}

type cancelAwareReadClient struct {
	entered  chan struct{}
	canceled chan struct{}
	once     sync.Once
}

func (client *cancelAwareReadClient) Snapshot(
	ctx context.Context,
	_ api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	client.once.Do(func() { close(client.entered) })
	<-ctx.Done()
	close(client.canceled)
	return api.LocalProductSnapshot{}, ctx.Err()
}

func (*cancelAwareReadClient) TimelinePage(
	context.Context,
	api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	return api.LocalProductTimelinePage{}, errors.New("unexpected timeline")
}

func TestModelSurfacesGapPartialProtocolMismatchAndCancelsInflightRead(
	t *testing.T,
) {
	model, err := NewModel(&fakeReadClient{})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("a", 64),
		Partial:       true,
	}})
	model = updated.(Model)
	if !strings.Contains(model.View(), "partial") {
		t.Fatalf("partial view = %q", model.View())
	}
	updated, _ = model.Update(snapshotFailedMsg{
		err: localipc.ErrInvalidProtocol,
	})
	model = updated.(Model)
	if strings.Contains(model.View(), "Daemon offline") ||
		!strings.Contains(model.View(), "fatal_protocol_mismatch") {
		t.Fatalf("protocol mismatch view = %q", model.View())
	}

	model.lastError = ""
	model.screenIndex = indexOfScreen(ScreenTimeline)
	updated, _ = model.Update(timelineLoadedMsg{
		page: api.LocalProductTimelinePage{
			SchemaVersion:  1,
			TeamInstanceID: "team-1",
			Gap: &api.LocalProductStreamGap{
				Reason:      "cursor_conflict",
				Recoverable: true,
			},
			Board: api.LocalProductTeamBoard{
				Status: "blocked",
				Nodes:  []api.NodeBoardRow{{LogicalNodeID: "main"}},
			},
			Attention: []api.AttentionItem{{
				Kind:           "human_required",
				ActionRequired: "provide_input",
			}},
		},
	})
	model = updated.(Model)
	for _, want := range []string{
		"Some activity is unavailable",
		"press r",
		"Blocked",
		"Provide input",
	} {
		if !strings.Contains(model.View(), want) {
			t.Fatalf("gap view missing %q: %q", want, model.View())
		}
	}

	cancelClient := &cancelAwareReadClient{
		entered:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
	cancelModel, err := NewModel(cancelClient)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- cancelModel.Init()() }()
	select {
	case <-cancelClient.entered:
	case <-time.After(time.Second):
		t.Fatal("snapshot read did not start")
	}
	if _, quit := cancelModel.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	}); quit == nil {
		t.Fatal("q did not return quit command")
	}
	select {
	case <-cancelClient.canceled:
	case <-time.After(time.Second):
		t.Fatal("q did not cancel in-flight read")
	}
	<-result
}

func TestModelNavigatesAllReadScreensAndNeverCreatesMutationCommand(t *testing.T) {
	client := &fakeReadClient{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("a", 64),
		Teams: []api.LocalProductTeamSummary{{
			TeamInstanceID: "team-1",
			DisplayName:    "Team One",
			SourceKind:     "saved_team",
		}},
	}}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	if cmd := model.Init(); cmd == nil {
		t.Fatal("Init() command = nil, want initial snapshot read")
	}
	updated, _ := model.Update(snapshotLoadedMsg{snapshot: client.snapshot})
	model = updated.(Model)
	if !strings.Contains(model.View(), "Missions") ||
		!strings.Contains(
			model.View(),
			"Saving a team never starts work",
		) {
		t.Fatalf("tasks view = %q", model.View())
	}

	wantScreens := []Screen{
		ScreenNewMission,
		ScreenMission,
		ScreenTeamBuilder,
		ScreenRuns,
		ScreenCompare,
		ScreenAttention,
		ScreenTimeline,
		ScreenAssets,
		ScreenBoard,
	}
	for _, want := range wantScreens {
		updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		if cmd != nil && want != ScreenAssets {
			t.Fatalf("screen navigation produced command for %s", want)
		}
		if cmd == nil && want == ScreenAssets {
			t.Fatal("Evolution Assets navigation omitted production IPC refresh")
		}
		model = updated.(Model)
		if model.Screen() != want {
			t.Fatalf("screen = %s, want %s", model.Screen(), want)
		}
	}
}

func TestModelRendersOfflineStaleResizeAndSanitizesUntrustedText(t *testing.T) {
	client := &fakeReadClient{}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	model = updated.(Model)
	if !strings.Contains(model.View(), "60") ||
		!strings.Contains(model.View(), "16") {
		t.Fatalf("small view = %q", model.View())
	}

	updated, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model = updated.(Model)
	updated, _ = model.Update(snapshotFailedMsg{
		err: localipc.ErrLocalProductUnavailable,
	})
	model = updated.(Model)
	if !strings.Contains(model.View(), "offline") {
		t.Fatalf("offline view = %q", model.View())
	}

	unsafe := "safe\x1b[31m-red\u202ehidden\nnext"
	updated, _ = model.Update(snapshotLoadedMsg{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 1,
		ViewVersion:   strings.Repeat("b", 64),
		Stale:         true,
		Reason:        "projection_refresh_failed",
		Teams: []api.LocalProductTeamSummary{{
			TeamInstanceID: "team-unsafe",
			DisplayName:    unsafe,
			SourceKind:     "saved_team",
		}},
	}})
	model = updated.(Model)
	view := model.View()
	if !strings.Contains(view, "stale") ||
		strings.Contains(view, "\x1b[31m") ||
		strings.Contains(view, "[31m") ||
		strings.Contains(view, "\u202e") ||
		strings.Contains(view, "hidden\nnext") {
		t.Fatalf("unsafe stale view = %q", view)
	}
}

type fakeSetupClient struct {
	fakeReadClient
	setup       app.SetupSnapshot
	session     app.BuilderSessionView
	starts      int
	answers     int
	credentials int
	statuses    int
	edits       int
	lastStart   app.BuilderStartCommand
	lastStatus  app.TeamStatusCommand
	lastEdit    app.BuilderEditCommand
}

func (client *fakeSetupClient) SetupSnapshot(
	context.Context,
) (app.SetupSnapshot, error) {
	return client.setup, client.err
}

func (client *fakeSetupClient) StartBuilder(
	_ context.Context,
	command app.BuilderStartCommand,
) (app.BuilderSessionView, error) {
	client.starts++
	client.lastStart = command
	return client.session, client.err
}

func (client *fakeSetupClient) AnswerBuilder(
	_ context.Context,
	_ app.BuilderAnswerCommand,
) (app.BuilderSessionView, error) {
	client.answers++
	return client.session, client.err
}

func (client *fakeSetupClient) EditBuilder(
	_ context.Context,
	command app.BuilderEditCommand,
) (app.BuilderSessionView, error) {
	client.edits++
	client.lastEdit = command
	return client.session, client.err
}

func (client *fakeSetupClient) ConfirmBuilder(
	_ context.Context,
	_ app.BuilderConfirmCommand,
) (app.BuilderConfirmation, error) {
	return app.BuilderConfirmation{
		TeamDefinitionID: "team-fixture",
		Status:           "active",
	}, client.err
}

func (client *fakeSetupClient) ArchiveTeam(
	_ context.Context,
	command app.TeamStatusCommand,
) (app.SetupSavedTeamPreview, error) {
	client.statuses++
	client.lastStatus = command
	return app.SetupSavedTeamPreview{
		ID:         command.DefinitionID,
		Status:     "archived",
		StreamHead: command.ExpectedHead + 1,
	}, client.err
}

func (client *fakeSetupClient) RestoreTeam(
	_ context.Context,
	command app.TeamStatusCommand,
) (app.SetupSavedTeamPreview, error) {
	client.statuses++
	client.lastStatus = command
	return app.SetupSavedTeamPreview{
		ID:         command.DefinitionID,
		Status:     "active",
		StreamHead: command.ExpectedHead + 1,
	}, client.err
}

func (client *fakeSetupClient) ConfigureCredential(
	_ context.Context,
	_ app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	client.credentials++
	return app.CredentialSetupResult{
		ProviderID: "minimax",
		Revision:   1,
		Status:     "configured",
	}, client.err
}

func (client *fakeSetupClient) VerifyCredential(
	_ context.Context,
	_ app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	client.credentials++
	return app.CredentialSetupResult{
		ProviderID: "minimax",
		Revision:   2,
		Status:     "verified",
	}, client.err
}

func (client *fakeSetupClient) ReplaceCredential(
	_ context.Context,
	_ app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	client.credentials++
	return app.CredentialSetupResult{
		ProviderID: "minimax",
		Revision:   2,
		Status:     "configured",
	}, client.err
}

func (client *fakeSetupClient) RevokeCredential(
	_ context.Context,
	_ app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	client.credentials++
	return app.CredentialSetupResult{
		ProviderID: "minimax",
		Revision:   2,
		Status:     "revoked",
	}, client.err
}

func TestModelConfirmedExecutableTeamRefreshesSetupAndAuthoritativeSnapshot(
	t *testing.T,
) {
	client := &fakeSetupClient{
		setup: app.SetupSnapshot{
			SchemaVersion: 1,
			ViewVersion:   strings.Repeat("a", 64),
			Runtimes:      []app.SetupRuntimePreview{},
			SavedTeams:    []app.SetupSavedTeamPreview{},
			Templates:     []app.SetupTeamTemplatePreview{},
			RoleOptions:   []app.SetupRoleOptionPreview{},
			Skills:        []app.SetupSkillRevision{},
			Permissions:   []string{},
			Resources:     []app.SetupResourcePointer{},
		},
		fakeReadClient: fakeReadClient{snapshot: api.LocalProductSnapshot{
			SchemaVersion: 2,
			ViewVersion:   strings.Repeat("b", 64),
			Teams: []api.LocalProductTeamSummary{{
				TeamInstanceID: "team-instance-fixture",
				DisplayName:    "Controlled Team",
				SourceKind:     "saved_team",
				State:          "created",
				Confirmed:      true,
				Executable:     true,
			}},
		}},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	updated, command := model.Update(builderConfirmedMsg{
		confirmation: app.BuilderConfirmation{
			TeamDefinitionID:    "team-fixture",
			Status:              "active",
			TeamInstanceCreated: true,
		},
	})
	model = updated.(Model)
	if command == nil {
		t.Fatal("confirmed executable Team did not refresh product state")
	}
	batch, ok := command().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("post-confirm command = %#v, want setup + snapshot batch", command())
	}
	seenSetup := false
	seenSnapshot := false
	for _, next := range batch {
		switch message := next().(type) {
		case setupLoadedMsg:
			seenSetup = true
			updated, _ = model.Update(message)
			model = updated.(Model)
		case snapshotLoadedMsg:
			seenSnapshot = true
			updated, _ = model.Update(message)
			model = updated.(Model)
		default:
			t.Fatalf("unexpected post-confirm message = %#v", message)
		}
	}
	if !seenSetup || !seenSnapshot || len(model.snapshot.Teams) != 1 ||
		model.snapshot.Teams[0].TeamInstanceID != "team-instance-fixture" {
		t.Fatalf(
			"post-confirm setup=%t snapshot=%t state=%#v",
			seenSetup,
			seenSnapshot,
			model.snapshot,
		)
	}
}

func TestModelTeamBuilderOpensSavedAndTemplateCandidatesAndArchivesByHead(
	t *testing.T,
) {
	digest := strings.Repeat("d", 64)
	client := &fakeSetupClient{
		setup: app.SetupSnapshot{
			SavedTeams: []app.SetupSavedTeamPreview{{
				ID:               "team-saved",
				Version:          1,
				Name:             "Saved Review Team",
				Status:           "active",
				DefinitionDigest: digest,
				StreamHead:       3,
			}},
			Templates: []app.SetupTeamTemplatePreview{{
				ID:      "template-review",
				Version: 2,
				Digest:  digest,
				Name:    "Template Review Team",
			}},
		},
		session: app.BuilderSessionView{
			DraftID: "draft-from-source",
			Source:  app.BuilderSourceSavedTeam,
		},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.screenIndex = indexOfScreen(ScreenTeamBuilder)
	model.setup = cloneSetupSnapshot(client.setup)
	model.loading = false

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("saved Team selection did not start a Candidate")
	}
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if client.lastStart.Source != app.BuilderSourceSavedTeam ||
		client.lastStart.SourceID != "team-saved" ||
		client.lastStart.SourceDigest != digest {
		t.Fatalf("saved start = %#v", client.lastStart)
	}

	model.builder = app.BuilderSessionView{}
	model.selected = 1
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("template selection did not start a Candidate")
	}
	model = updated.(Model)
	_, _ = model.Update(command())
	if client.lastStart.Source != app.BuilderSourceTemplate ||
		client.lastStart.SourceID != "template-review" {
		t.Fatalf("template start = %#v", client.lastStart)
	}

	model.builder = app.BuilderSessionView{}
	model.selected = 0
	model.loading = false
	updated, command = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("a"),
	})
	if command == nil {
		t.Fatal("archive action was not exposed")
	}
	model = updated.(Model)
	_, _ = model.Update(command())
	if client.statuses != 1 ||
		client.lastStatus.DefinitionID != "team-saved" ||
		client.lastStatus.ExpectedHead != 3 {
		t.Fatalf(
			"archive calls=%d command=%#v",
			client.statuses,
			client.lastStatus,
		)
	}
}

func TestModelTeamBuilderEnterDoesNotReplaceOpenCandidate(
	t *testing.T,
) {
	digest := strings.Repeat("d", 64)
	client := &fakeSetupClient{
		setup: app.SetupSnapshot{
			SavedTeams: []app.SetupSavedTeamPreview{{
				ID:               "team-saved",
				Version:          1,
				Name:             "Saved Review Team",
				Status:           "active",
				DefinitionDigest: digest,
				StreamHead:       1,
			}},
		},
		session: app.BuilderSessionView{DraftID: "unexpected"},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.screenIndex = indexOfScreen(ScreenTeamBuilder)
	model.setup = cloneSetupSnapshot(client.setup)
	model.builder = app.BuilderSessionView{
		DraftID:    "draft-open",
		Revision:   4,
		CanConfirm: true,
		Preview: app.BuilderPreview{
			Name: "Open Candidate",
		},
	}
	model.loading = false

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil {
		t.Fatal("enter on an open Candidate started another setup command")
	}
	model = updated.(Model)
	if client.starts != 0 || model.builder.DraftID != "draft-open" {
		t.Fatalf(
			"starts=%d builder=%#v",
			client.starts,
			model.builder,
		)
	}
}

func TestModelTeamBuilderUsesDaemonSetupClientWithoutTerminalInput(t *testing.T) {
	client := &fakeSetupClient{
		setup: app.SetupSnapshot{
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
			Runtimes: []app.SetupRuntimePreview{{
				RuntimeInstanceID: "runtime-pi",
				DisplayName:       "Pi Coding Agent",
				ExecutableVersion: "0.82.1",
				Status:            "online",
				ModelIDs:          []string{"model-a"},
			}},
			SavedTeams:  []app.SetupSavedTeamPreview{},
			Templates:   []app.SetupTeamTemplatePreview{},
			RoleOptions: []app.SetupRoleOptionPreview{},
			Skills:      []app.SetupSkillRevision{},
			Permissions: []string{},
			Resources:   []app.SetupResourcePointer{},
		},
		session: app.BuilderSessionView{
			SchemaVersion: 1,
			DraftID:       "draft-1",
			Revision:      1,
			Source:        app.BuilderSourceBlank,
			Question: app.BuilderQuestion{
				ID:      "team_name",
				Prompt:  "Name this team",
				Options: []app.BuilderQuestionOption{},
			},
		},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.screenIndex = indexOfScreen(ScreenTeamBuilder)
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("Team Builder enter did not call setup application client")
	}
	model = updated.(Model)
	updated, command = model.Update(command())
	if command != nil {
		t.Fatal("setup snapshot result produced unexpected command")
	}
	model = updated.(Model)
	view := model.View()
	for _, want := range []string{
		"Team Builder",
		"Codex",
		"native_auth",
		"MiniMax",
		"brokered",
		"Pi Coding Agent",
		"0.82.1",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("Team Builder view missing %q: %q", want, view)
		}
	}
	updated, command = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("n"),
	})
	if command == nil {
		t.Fatal("new Team action did not start Candidate builder")
	}
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if client.starts != 1 ||
		!strings.Contains(model.View(), "Name this team") ||
		strings.Contains(model.View(), "sqlite") ||
		strings.Contains(model.View(), "launchctl") {
		t.Fatalf(
			"builder starts=%d view=%q",
			client.starts,
			model.View(),
		)
	}
}

func TestModelTeamBuilderAnswersAndMasksCredentialEntry(t *testing.T) {
	client := &fakeSetupClient{
		setup: app.SetupSnapshot{
			SchemaVersion: 1,
			ViewVersion:   strings.Repeat("a", 64),
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
		},
		session: app.BuilderSessionView{
			SchemaVersion: 1,
			DraftID:       "draft-1",
			Revision:      1,
			Source:        app.BuilderSourceBlank,
			CatalogDigest: strings.Repeat("b", 64),
			Question: app.BuilderQuestion{
				ID:      "team_name",
				Prompt:  "Name this team",
				Options: []app.BuilderQuestionOption{},
			},
		},
	}
	model, err := NewModel(client)
	if err != nil {
		t.Fatal(err)
	}
	model.screenIndex = indexOfScreen(ScreenTeamBuilder)
	model.setup = cloneSetupSnapshot(client.setup)
	model.builder = cloneBuilderSession(client.session)
	model.loading = false

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil {
		t.Fatal("opening answer input produced an external command")
	}
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("Local Team"),
	})
	model = updated.(Model)
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("submitting an answer did not call the application service")
	}
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if client.answers != 1 {
		t.Fatalf("answer calls = %d", client.answers)
	}

	model.builder = app.BuilderSessionView{}
	updated, command = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("g"),
	})
	if command != nil {
		t.Fatal("opening masked credential entry produced a command")
	}
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("alpha"),
	})
	model = updated.(Model)
	if strings.Contains(model.View(), "alpha") ||
		!strings.Contains(model.View(), "•••••") {
		t.Fatalf("credential entry was not masked: %q", model.View())
	}
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("credential submit did not call the setup service")
	}
	model = updated.(Model)
	if len(model.entry) != 0 || model.entryMode != "" {
		t.Fatal("credential input remained in TUI state after submit")
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if client.credentials != 1 {
		t.Fatalf("credential calls = %d", client.credentials)
	}
}
