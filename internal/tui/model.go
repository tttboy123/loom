package tui

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/production"
	"loom-pi-rebuild/internal/work"
)

type Screen string

const (
	ScreenBoard         Screen = "Board"
	ScreenNewMission    Screen = "New Mission"
	ScreenMission       Screen = "Mission"
	ScreenHome          Screen = "Home"
	ScreenRuntimes      Screen = "Runtimes"
	ScreenTeamBuilder   Screen = "Team Builder"
	ScreenTeams         Screen = "Teams"
	ScreenRuns          Screen = "Runs / History"
	ScreenEvidence      Screen = "Evidence"
	ScreenCompare       Screen = "Compare"
	ScreenAttention     Screen = "Attention"
	ScreenTimeline      Screen = "Team Timeline"
	ScreenAssets        Screen = "Evolution Assets"
	ScreenQueue         Screen = "Queue"
	ScreenWorkers       Screen = "Workers"
	ScreenIntegration   Screen = "Integration"
	ScreenPermissions   Screen = "Permissions"
	ScreenExecution     Screen = "Execution"
	ScreenProduction    Screen = "Production"
	ScreenCustomerRules Screen = "Customer Rules"
	ScreenAutonomy      Screen = "Autopilot"
)

var screens = []Screen{
	ScreenHome,
	ScreenBoard,
	ScreenTeams,
	ScreenEvidence,
	ScreenRuntimes,
	ScreenNewMission,
	ScreenMission,
	ScreenTeamBuilder,
	ScreenRuns,
	ScreenCompare,
	ScreenAttention,
	ScreenTimeline,
	ScreenAssets,
	ScreenQueue,
	ScreenWorkers,
	ScreenIntegration,
	ScreenPermissions,
	ScreenExecution,
	ScreenProduction,
	ScreenCustomerRules,
	ScreenAutonomy,
}

var governanceViews = []string{
	"Overview",
	"Board",
	"Teams",
	"Decisions",
	"Evidence",
	"Runtimes",
	"Attention",
}

type ReadClient interface {
	Snapshot(
		context.Context,
		api.LocalProductSnapshotRequest,
	) (api.LocalProductSnapshot, error)
	TimelinePage(
		context.Context,
		api.LocalProductTimelineRequest,
	) (api.LocalProductTimelinePage, error)
}

type ChatClient interface {
	ChatThread(context.Context, api.LocalProductChatThreadRequest) (api.LocalProductChatThread, error)
	SendChatMessage(context.Context, api.LocalProductChatMessageRequest) (api.LocalProductChatThread, error)
}

type SetupClient interface {
	SetupSnapshot(context.Context) (app.SetupSnapshot, error)
	StartBuilder(
		context.Context,
		app.BuilderStartCommand,
	) (app.BuilderSessionView, error)
	AnswerBuilder(
		context.Context,
		app.BuilderAnswerCommand,
	) (app.BuilderSessionView, error)
	EditBuilder(
		context.Context,
		app.BuilderEditCommand,
	) (app.BuilderSessionView, error)
	ConfirmBuilder(
		context.Context,
		app.BuilderConfirmCommand,
	) (app.BuilderConfirmation, error)
	ArchiveTeam(
		context.Context,
		app.TeamStatusCommand,
	) (app.SetupSavedTeamPreview, error)
	RestoreTeam(
		context.Context,
		app.TeamStatusCommand,
	) (app.SetupSavedTeamPreview, error)
	ConfigureCredential(
		context.Context,
		app.CredentialSetupCommand,
	) (app.CredentialSetupResult, error)
	VerifyCredential(
		context.Context,
		app.CredentialSetupCommand,
	) (app.CredentialSetupResult, error)
	ReplaceCredential(
		context.Context,
		app.CredentialSetupCommand,
	) (app.CredentialSetupResult, error)
	RevokeCredential(
		context.Context,
		app.CredentialSetupCommand,
	) (app.CredentialSetupResult, error)
}

type ExecutionClient interface {
	ExecuteMission(
		context.Context,
		app.MissionExecutionCommand,
	) (api.MissionExecutionEnvelope, error)
}

type MissionDecisionClient interface {
	ReadMissionDecision(
		context.Context,
		app.MissionDecisionCommand,
	) (app.MissionDecisionSheet, error)
	DecideMission(
		context.Context,
		app.MissionDecisionCommand,
	) (app.MissionDecisionResult, error)
}

type EvolutionAssetClient interface {
	EvolutionAssetSnapshot(context.Context, api.EvolutionAssetSnapshotRequest) (api.EvolutionAssetSnapshot, error)
	EvolutionAssetDiff(context.Context, api.EvolutionAssetDiffRequest) (api.EvolutionAssetDiff, error)
	EvolutionAssetCommand(context.Context, api.EvolutionAssetCommandRequest) (api.EvolutionAssetCommandResult, error)
}

type QueueClient interface {
	QueueSnapshot(context.Context, api.QueueSnapshotRequest) (api.QueueSnapshot, error)
	QueueCommand(context.Context, api.QueueCommandRequest) (api.QueueCommandResult, error)
}

type WorkersClient interface {
	WorkersSnapshot(context.Context, app.WorkersSnapshotRequest) (app.WorkersSnapshot, error)
	WorkersCommand(context.Context, app.WorkersCommandRequest) (app.WorkersCommandResult, error)
}

type IntegrationClient interface {
	IntegrationSnapshot(context.Context, string) (app.IntegrationSnapshot, error)
	IntegrationCommand(context.Context, app.IntegrationCommandRequest) (app.IntegrationCommandResult, error)
}

type ProductionClient interface {
	ProductionSnapshot(context.Context, app.ProductionSnapshotRequest) (production.ProductionSnapshot, error)
	ProductionCommand(context.Context, app.ProductionCommandRequest) (app.ProductionCommandResult, error)
}

type HandoffClient interface {
	ProposeSideTask(context.Context, app.SideTaskProposalRequest) (app.SideTaskProposalResult, error)
	CreateSideTask(context.Context, app.SideTaskCreateRequest) (app.SideTaskCreateResult, error)
	ReadSideTask(context.Context, app.SideTaskReadRequest) (app.SideTaskReadResult, error)
	DecideSideTask(context.Context, app.SideTaskDecisionRequest) (app.SideTaskDecisionResult, error)
}

type DaemonReadClient struct {
	client *localipc.Client
}

func NewDaemonReadClient(client *localipc.Client) (*DaemonReadClient, error) {
	if client == nil {
		return nil, errors.New("invalid daemon read client")
	}
	return &DaemonReadClient{client: client}, nil
}

func (client *DaemonReadClient) Snapshot(
	ctx context.Context,
	request api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	var snapshot api.LocalProductSnapshot
	err := client.client.Call(ctx, "snapshot", request, &snapshot)
	return snapshot, err
}

func (client *DaemonReadClient) TimelinePage(
	ctx context.Context,
	request api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	var page api.LocalProductTimelinePage
	err := client.client.Call(ctx, "timeline_page", request, &page)
	return page, err
}

func (client *DaemonReadClient) EvolutionAssetSnapshot(ctx context.Context, request api.EvolutionAssetSnapshotRequest) (api.EvolutionAssetSnapshot, error) {
	var snapshot api.EvolutionAssetSnapshot
	err := client.client.CallJourney(ctx, request.JourneyID, "evolution_asset_snapshot", request, &snapshot)
	return snapshot, err
}

func (client *DaemonReadClient) EvolutionAssetDiff(ctx context.Context, request api.EvolutionAssetDiffRequest) (api.EvolutionAssetDiff, error) {
	var diff api.EvolutionAssetDiff
	err := client.client.CallJourney(ctx, request.JourneyID, "evolution_asset_diff", request, &diff)
	return diff, err
}

func (client *DaemonReadClient) EvolutionAssetCommand(ctx context.Context, request api.EvolutionAssetCommandRequest) (api.EvolutionAssetCommandResult, error) {
	var result api.EvolutionAssetCommandResult
	err := client.client.CallJourney(ctx, request.JourneyID, "evolution_asset_command", request, &result)
	return result, err
}

func (client *DaemonReadClient) ExecuteMission(
	ctx context.Context,
	command app.MissionExecutionCommand,
) (api.MissionExecutionEnvelope, error) {
	var envelope api.MissionExecutionEnvelope
	err := client.client.Call(ctx, "mission_execution", command, &envelope)
	return envelope, err
}

func (client *DaemonReadClient) ChatThread(
	ctx context.Context,
	request api.LocalProductChatThreadRequest,
) (api.LocalProductChatThread, error) {
	var thread api.LocalProductChatThread
	err := client.client.Call(ctx, "chat_thread", request, &thread)
	return thread, err
}

func (client *DaemonReadClient) SendChatMessage(
	ctx context.Context,
	request api.LocalProductChatMessageRequest,
) (api.LocalProductChatThread, error) {
	var thread api.LocalProductChatThread
	err := client.client.Call(ctx, "chat_message", request, &thread)
	return thread, err
}

func (client *DaemonReadClient) ReadMissionDecision(
	ctx context.Context,
	command app.MissionDecisionCommand,
) (app.MissionDecisionSheet, error) {
	var sheet app.MissionDecisionSheet
	err := client.client.Call(ctx, "mission_decision", command, &sheet)
	return sheet, err
}

func (client *DaemonReadClient) DecideMission(
	ctx context.Context,
	command app.MissionDecisionCommand,
) (app.MissionDecisionResult, error) {
	var result app.MissionDecisionResult
	err := client.client.Call(ctx, "mission_decision", command, &result)
	return result, err
}

func (client *DaemonReadClient) ProposeSideTask(ctx context.Context, request app.SideTaskProposalRequest) (app.SideTaskProposalResult, error) {
	var result app.SideTaskProposalResult
	err := client.client.Call(ctx, "side_task_handoff", request, &result)
	return result, err
}
func (client *DaemonReadClient) CreateSideTask(ctx context.Context, request app.SideTaskCreateRequest) (app.SideTaskCreateResult, error) {
	var result app.SideTaskCreateResult
	err := client.client.Call(ctx, "side_task_handoff", request, &result)
	return result, err
}
func (client *DaemonReadClient) ReadSideTask(ctx context.Context, request app.SideTaskReadRequest) (app.SideTaskReadResult, error) {
	var result app.SideTaskReadResult
	err := client.client.Call(ctx, "side_task_handoff", request, &result)
	return result, err
}
func (client *DaemonReadClient) DecideSideTask(ctx context.Context, request app.SideTaskDecisionRequest) (app.SideTaskDecisionResult, error) {
	var result app.SideTaskDecisionResult
	err := client.client.Call(ctx, "side_task_handoff", request, &result)
	return result, err
}

func (client *DaemonReadClient) SetupSnapshot(
	ctx context.Context,
) (app.SetupSnapshot, error) {
	var snapshot app.SetupSnapshot
	err := client.client.Call(ctx, "setup_snapshot", struct{}{}, &snapshot)
	return snapshot, err
}

func (client *DaemonReadClient) StartBuilder(
	ctx context.Context,
	command app.BuilderStartCommand,
) (app.BuilderSessionView, error) {
	var session app.BuilderSessionView
	err := client.client.Call(ctx, "builder_start", command, &session)
	return session, err
}

func (client *DaemonReadClient) AnswerBuilder(
	ctx context.Context,
	command app.BuilderAnswerCommand,
) (app.BuilderSessionView, error) {
	var session app.BuilderSessionView
	err := client.client.Call(ctx, "builder_answer", command, &session)
	return session, err
}

func (client *DaemonReadClient) EditBuilder(
	ctx context.Context,
	command app.BuilderEditCommand,
) (app.BuilderSessionView, error) {
	var session app.BuilderSessionView
	err := client.client.Call(ctx, "builder_edit", command, &session)
	return session, err
}

func (client *DaemonReadClient) ConfirmBuilder(
	ctx context.Context,
	command app.BuilderConfirmCommand,
) (app.BuilderConfirmation, error) {
	var confirmation app.BuilderConfirmation
	err := client.client.Call(
		ctx,
		"builder_confirm",
		command,
		&confirmation,
	)
	return confirmation, err
}

func (client *DaemonReadClient) ArchiveTeam(
	ctx context.Context,
	command app.TeamStatusCommand,
) (app.SetupSavedTeamPreview, error) {
	var team app.SetupSavedTeamPreview
	err := client.client.Call(ctx, "team_archive", command, &team)
	return team, err
}

func (client *DaemonReadClient) RestoreTeam(
	ctx context.Context,
	command app.TeamStatusCommand,
) (app.SetupSavedTeamPreview, error) {
	var team app.SetupSavedTeamPreview
	err := client.client.Call(ctx, "team_restore", command, &team)
	return team, err
}

func (client *DaemonReadClient) ConfigureCredential(
	ctx context.Context,
	command app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	return client.mutateCredential(ctx, "credential_configure", command)
}

func (client *DaemonReadClient) VerifyCredential(
	ctx context.Context,
	command app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	return client.mutateCredential(ctx, "credential_verify", command)
}

func (client *DaemonReadClient) ReplaceCredential(
	ctx context.Context,
	command app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	return client.mutateCredential(ctx, "credential_replace", command)
}

func (client *DaemonReadClient) RevokeCredential(
	ctx context.Context,
	command app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	return client.mutateCredential(ctx, "credential_revoke", command)
}

func (client *DaemonReadClient) mutateCredential(
	ctx context.Context,
	method string,
	command app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	defer clearTUIBytes(command.Secret)
	var result app.CredentialSetupResult
	err := client.client.Call(ctx, method, struct {
		ProviderID          string `json:"provider_id"`
		CredentialReference string `json:"credential_reference"`
		ExpectedRevision    int64  `json:"expected_revision"`
		Secret              string `json:"secret"`
	}{
		ProviderID:          command.ProviderID,
		CredentialReference: command.CredentialReference,
		ExpectedRevision:    command.ExpectedRevision,
		Secret:              string(command.Secret),
	}, &result)
	return result, err
}

type snapshotLoadedMsg struct {
	snapshot api.LocalProductSnapshot
}

type snapshotFailedMsg struct {
	err error
}

type attentionRefreshedMsg struct {
	generation          uint64
	snapshot            api.LocalProductSnapshot
	snapshotErr         error
	permissionAttention app.PermissionAttention
	permissionErr       error
	hasPermission       bool
}

type timelineLoadedMsg struct {
	page api.LocalProductTimelinePage
}

type timelineFailedMsg struct {
	err error
}

type setupLoadedMsg struct {
	snapshot app.SetupSnapshot
}

type setupFailedMsg struct {
	err error
}

type evolutionAssetsLoadedMsg struct{ snapshot api.EvolutionAssetSnapshot }
type evolutionAssetsFailedMsg struct{ err error }
type evolutionAssetDiffLoadedMsg struct{ diff api.EvolutionAssetDiff }
type evolutionAssetCommittedMsg struct {
	result api.EvolutionAssetCommandResult
}

type queueLoadedMsg struct{ snapshot api.QueueSnapshot }
type queueFailedMsg struct{ err error }
type workersLoadedMsg struct{ snapshot app.WorkersSnapshot }
type workersFailedMsg struct{ err error }
type integrationLoadedMsg struct{ snapshot app.IntegrationSnapshot }
type integrationFailedMsg struct{ err error }
type queueCommandDoneMsg struct{ err error }
type workersCommandDoneMsg struct{ err error }
type integrationCommandDoneMsg struct{ err error }

type builderStartedMsg struct {
	session app.BuilderSessionView
}

type builderUpdatedMsg struct {
	session app.BuilderSessionView
}

type builderConfirmedMsg struct {
	confirmation app.BuilderConfirmation
}

type builderConfirmFailedMsg struct {
	err error
}

type teamStatusUpdatedMsg struct {
	team app.SetupSavedTeamPreview
}

type credentialUpdatedMsg struct {
	result app.CredentialSetupResult
}

type missionPreflightedMsg struct {
	preflight app.MissionExecutionPreflight
	snapshot  api.LocalProductSnapshot
}

type missionStartedMsg struct {
	result app.MissionExecutionResult
}

type missionExecutionFailedMsg struct {
	err error
}
type chatThreadLoadedMsg struct {
	thread api.LocalProductChatThread
}

type chatThreadFailedMsg struct {
	err error
}

type chatMessageSentMsg struct {
	thread api.LocalProductChatThread
}

type chatMessageFailedMsg struct {
	err error
}

type sideTaskProposedMsg struct {
	request app.SideTaskProposalRequest
	result  app.SideTaskProposalResult
}

type sideTaskCreatedMsg struct{ result app.SideTaskCreateResult }
type sideTaskDecidedMsg struct{ result app.SideTaskDecisionResult }
type sideTaskFailedMsg struct{ err error }
type missionDecisionLoadedMsg struct {
	command app.MissionDecisionCommand
	sheet   app.MissionDecisionSheet
}
type missionDecisionAppliedMsg struct{ result app.MissionDecisionResult }
type missionDecisionFailedMsg struct{ err error }

const (
	entryBuilderAnswer      = "builder_answer"
	entryChatDraft          = "chat_draft"
	entryFolderPath         = "folder_path"
	entryEditName           = "edit_name"
	entryEditPurpose        = "edit_purpose"
	entryCredentialPut      = "credential_put"
	entryCredentialSwap     = "credential_swap"
	entryTaskSearch         = "task_search"
	entryMissionObjective   = "mission_objective"
	entrySideTaskRequest    = "side_task_request"
	entryEvolutionAsset     = "evolution_asset_name"
	entryQueueJobPath       = "queue_job_path"
	entryEvolutionSearch    = "evolution_asset_search"
	entryPermissionProfile  = "permission_profile"
	entryPermissionCall     = "permission_call"
	entryCustomerRuleImport = "customer_rule_import"
)

type Model struct {
	client                 ReadClient
	setupClient            SetupClient
	boundedExecutionClient BoundedExecutionClient
	handoffClient          HandoffClient
	assetClient            EvolutionAssetClient
	queueClient            QueueClient
	workersClient          WorkersClient
	integrationClient      IntegrationClient
	permissionClient       PermissionClient
	productionClient       ProductionClient
	customerRuleClient     CustomerRuleClient
	standingOrderClient    AutonomyClient
	executionClient        ExecutionClient
	ctx                    context.Context
	cancel                 context.CancelFunc

	screenIndex            int
	selections             [24]int
	width                  int
	height                 int
	selected               int
	help                   bool
	loading                bool
	offline                bool
	lastError              string
	governancePanelVisible bool
	governancePanelPinned  bool
	governanceViewIndex    int
	snapshot               api.LocalProductSnapshot
	timeline               api.LocalProductTimelinePage
	setup                  app.SetupSnapshot
	builder                app.BuilderSessionView
	builderNotice          string
	confirmation           app.BuilderConfirmation
	credential             app.CredentialSetupResult
	entryMode              string
	entry                  []byte
	compareRuns            []string
	currentTeam            string
	currentMission         string
	missionID              string
	missionObjective       string
	missionTeamIndex       int
	missionPackageIndex    int
	missionPreflight       app.MissionExecutionPreflight
	missionResult          app.MissionExecutionResult
	decisionClient         MissionDecisionClient
	decisionCommand        app.MissionDecisionCommand
	decisionSheet          app.MissionDecisionSheet
	decisionActionIndex    int
	chatClient             ChatClient
	chatThread             api.LocalProductChatThread
	chatThreadID           string
	chatDraft              []byte
	workspaceFolder        string
	sideTaskPurpose        string
	sideTaskMode           string
	sideTaskTitle          string
	sideTaskRequest        string
	sideTaskProposalReq    app.SideTaskProposalRequest
	sideTaskProposal       app.SideTaskProposalResult
	sideTaskDecision       int
	decisionOpen           bool
	taskFilter             string
	navigationPrefix       bool
	evolutionAssets        api.EvolutionAssetSnapshot
	queueSnapshot          api.QueueSnapshot
	workersSnapshot        app.WorkersSnapshot
	integrationSnapshot    app.IntegrationSnapshot
	permissionSnapshot     app.PermissionSnapshot
	permissionAttention    app.PermissionAttention
	attentionGeneration    uint64
	executionSnapshot      app.ExecutionSnapshot
	productionSnapshot     production.ProductionSnapshot
	customerRuleSnapshot   app.CustomerRuleSnapshot
	customerRuleDetail     bool
	standingOrderSnapshot  app.StandingOrderSnapshot
	standingOrderSelected  int
	productionPreview      production.ActivationPreview
	productionPending      string
	permissionDetail       bool
	executionDetail        bool
	evolutionAssetDiff     api.EvolutionAssetDiff
	evolutionSearch        string
	evolutionJourneyID     string
	pendingEvolutionAction string
	evolutionCreateMode    string
}

func NewModel(client ReadClient) (Model, error) {
	return newModelWithContext(context.Background(), client)
}

func newModelWithContext(
	parent context.Context,
	client ReadClient,
) (Model, error) {
	if client == nil {
		return Model{}, errors.New("invalid TUI client")
	}
	if parent == nil {
		return Model{}, errors.New("invalid TUI context")
	}
	ctx, cancel := context.WithCancel(parent)
	setupClient, _ := client.(SetupClient)
	chatClient, _ := client.(ChatClient)
	executionClient, _ := client.(ExecutionClient)
	decisionClient, _ := client.(MissionDecisionClient)
	handoffClient, _ := client.(HandoffClient)
	assetClient, _ := client.(EvolutionAssetClient)
	journeyID, err := newTUIJourneyID()
	if err != nil {
		cancel()
		return Model{}, err
	}
	workspaceIdentity, err := os.Getwd()
	if err != nil {
		cancel()
		return Model{}, err
	}
	if physical, physicalErr := filepath.EvalSymlinks(workspaceIdentity); physicalErr == nil {
		workspaceIdentity = physical
	}
	return Model{
		client:                 client,
		setupClient:            setupClient,
		chatClient:             chatClient,
		chatThreadID:           newTUIChatThreadID(workspaceIdentity),
		executionClient:        executionClient,
		decisionClient:         decisionClient,
		handoffClient:          handoffClient,
		assetClient:            assetClient,
		queueClient:            queueClientFrom(client),
		workersClient:          workersClientFrom(client),
		integrationClient:      integrationClientFrom(client),
		permissionClient:       permissionClientFrom(client),
		boundedExecutionClient: boundedExecutionClientFrom(client),
		productionClient:       productionClientFrom(client),
		customerRuleClient:     customerRuleClientFrom(client),
		standingOrderClient:    standingOrderClientFrom(client),
		ctx:                    ctx,
		cancel:                 cancel,
		width:                  80,
		height:                 24,
		loading:                true,
		sideTaskPurpose:        "research",
		sideTaskMode:           "report_only",
		evolutionJourneyID:     journeyID,
	}, nil
}

func (model Model) Init() tea.Cmd {
	return model.loadSnapshot()
}

func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		return model, nil
	case snapshotLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.applySnapshot(message.snapshot)
		return model, nil
	case snapshotFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case attentionRefreshedMsg:
		if message.generation != model.attentionGeneration {
			return model, nil
		}
		model.loading = false
		if message.snapshotErr != nil {
			model.permissionAttention = app.PermissionAttention{}
			model.offline = errors.Is(
				message.snapshotErr,
				localipc.ErrLocalProductUnavailable,
			)
			model.lastError = safeClientState(message.snapshotErr)
			return model, nil
		}
		model.offline = false
		model.lastError = ""
		model.applySnapshot(message.snapshot)
		if !message.hasPermission {
			model.permissionAttention = app.PermissionAttention{}
			return model, nil
		}
		if message.permissionErr != nil {
			model.permissionAttention = app.PermissionAttention{}
			model.offline = errors.Is(
				message.permissionErr,
				localipc.ErrLocalProductUnavailable,
			)
			model.lastError = safeClientState(message.permissionErr)
			return model, nil
		}
		model.permissionAttention = message.permissionAttention
		return model, nil
	case timelineLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.timeline = cloneTimeline(message.page)
		return model, nil
	case timelineFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case chatThreadLoadedMsg:
		model.loading = false
		model.offline = false
		model.chatThread = cloneChatThread(message.thread)
		return model, nil
	case chatThreadFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case chatMessageSentMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.chatThread = cloneChatThread(message.thread)
		model.chatDraft = nil
		return model, nil
	case chatMessageFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case setupLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.setup = cloneSetupSnapshot(message.snapshot)
		return model, nil
	case builderStartedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.builderNotice = ""
		model.builder = cloneBuilderSession(message.session)
		return model, nil
	case builderUpdatedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.builder = cloneBuilderSession(message.session)
		model.selected = 0
		return model, nil
	case builderConfirmedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.builderNotice = ""
		model.confirmation = message.confirmation
		model.builder = app.BuilderSessionView{}
		if message.confirmation.TeamInstanceCreated {
			return model, tea.Batch(model.loadSetup(), model.loadSnapshot())
		}
		return model, model.loadSetup()
	case builderConfirmFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		if isRemoteConflict(message.err) || isRemoteNotFound(message.err) {
			model.builder = app.BuilderSessionView{}
			model.confirmation = app.BuilderConfirmation{}
			model.lastError = ""
			if isRemoteNotFound(message.err) {
				model.builderNotice = "This Agent Team draft is no longer available. Review the latest Teams, then press n to start a new draft."
			} else {
				model.builderNotice = "This Agent Team draft changed in another client. Review the latest Teams, then press n to start a new draft."
			}
			return model, model.loadSetup()
		}
		model.lastError = safeClientState(message.err)
		return model, nil
	case teamStatusUpdatedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		return model, model.loadSetup()
	case credentialUpdatedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.credential = message.result
		return model, model.loadSetup()
	case missionPreflightedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.snapshot = message.snapshot
		model.missionPreflight = message.preflight
		model.missionResult = app.MissionExecutionResult{}
		return model, nil
	case missionStartedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.missionPreflight = app.MissionExecutionPreflight{}
		model.missionResult = message.result
		model.currentMission = message.result.MissionID
		model.currentTeam = message.result.TeamInstanceID
		return model, model.loadSnapshot()
	case missionExecutionFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case missionDecisionLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.decisionCommand = message.command
		model.decisionSheet = message.sheet
		model.decisionActionIndex = 0
		model.decisionOpen = true
		return model, nil
	case missionDecisionAppliedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.decisionOpen = false
		model.decisionCommand = app.MissionDecisionCommand{}
		model.decisionSheet = app.MissionDecisionSheet{}
		model.decisionActionIndex = 0
		return model, model.loadSnapshot()
	case missionDecisionFailedMsg:
		model.loading = false
		model.decisionOpen = false
		model.decisionCommand = app.MissionDecisionCommand{}
		model.decisionSheet = app.MissionDecisionSheet{}
		model.decisionActionIndex = 0
		model.lastError = safeClientState(message.err)
		return model, nil
	case sideTaskProposedMsg:
		model.loading = false
		model.lastError = ""
		model.sideTaskProposalReq = message.request
		model.sideTaskProposal = message.result
		return model, nil
	case sideTaskCreatedMsg:
		model.loading = false
		model.lastError = ""
		model.sideTaskProposalReq = app.SideTaskProposalRequest{}
		model.sideTaskProposal = app.SideTaskProposalResult{}
		model.sideTaskRequest = ""
		model.sideTaskTitle = ""
		return model, model.loadSnapshot()
	case sideTaskDecidedMsg:
		model.loading = false
		model.lastError = ""
		return model, model.loadSnapshot()
	case sideTaskFailedMsg:
		model.loading = false
		model.lastError = safeClientState(message.err)
		return model, nil
	case setupFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case evolutionAssetsLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.evolutionAssets = message.snapshot
		model.clampSelection()
		return model, nil
	case evolutionAssetCommittedMsg:
		model.loading = true
		model.lastError = ""
		return model, tea.Batch(model.loadSnapshot(), model.loadEvolutionAssets(""))
	case evolutionAssetsFailedMsg:
		model.loading = false
		model.lastError = safeClientState(message.err)
		return model, nil
	case evolutionAssetDiffLoadedMsg:
		model.loading = false
		model.lastError = ""
		model.evolutionAssetDiff = message.diff
		return model, nil
	case queueLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.queueSnapshot = message.snapshot
		model.clampSelection()
		return model, nil
	case queueFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case workersLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.workersSnapshot = message.snapshot
		return model, nil
	case workersFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case integrationLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.integrationSnapshot = message.snapshot
		return model, nil
	case integrationFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case queueCommandDoneMsg:
		model.loading = false
		if message.err != nil {
			model.lastError = safeClientState(message.err)
			return model, nil
		}
		model.lastError = ""
		return model, model.loadQueue()
	case workersCommandDoneMsg:
		model.loading = false
		if message.err != nil {
			model.lastError = safeClientState(message.err)
			return model, nil
		}
		model.lastError = ""
		return model, tea.Batch(model.loadWorkers(), model.loadQueue())
	case integrationCommandDoneMsg:
		model.loading = false
		if message.err != nil {
			model.lastError = safeClientState(message.err)
			return model, nil
		}
		model.lastError = ""
		return model, model.loadIntegration()
	case permissionsLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.permissionSnapshot = message.snapshot
		model.clampSelection()
		return model, nil
	case permissionsFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case permissionAttentionLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.permissionAttention = message.attention
		return model, nil
	case permissionAttentionFailedMsg:
		model.loading = false
		model.lastError = safeClientState(message.err)
		return model, nil
	case customerRuleLoadedMsg:
		model.loading = false
		model.lastError = ""
		model.customerRuleSnapshot = message.snapshot
		return model, nil
	case customerRuleFailedMsg:
		model.loading = false
		model.lastError = safeClientState(message.err)
		return model, nil
	case customerRuleCommandDoneMsg:
		model.loading = false
		if message.err != nil {
			model.lastError = safeClientState(message.err)
			return model, nil
		}
		model.lastError = message.note
		return model, model.loadCustomerRules()
	case standingOrderLoadedMsg:
		model.loading = false
		model.standingOrderSnapshot = message.snapshot
		return model, nil
	case standingOrderFailedMsg:
		model.loading = false
		model.lastError = safeClientState(message.err)
		return model, nil
	case standingOrderCommandDoneMsg:
		model.loading = false
		if message.err != nil {
			model.lastError = safeClientState(message.err)
			return model, nil
		}
		model.lastError = message.note
		return model, model.loadStandingOrders()
	case permissionCommandDoneMsg:
		model.loading = false
		if message.err != nil {
			model.lastError = safeClientState(message.err)
			return model, nil
		}
		model.lastError = message.note
		return model, tea.Batch(
			model.loadPermissions(),
			model.loadPermissionAttention(),
		)
	case executionLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.executionSnapshot = message.snapshot
		model.clampSelection()
		return model, nil
	case executionFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case executionCommandDoneMsg:
		model.loading = false
		if message.err != nil {
			model.lastError = safeClientState(message.err)
			return model, nil
		}
		model.lastError = message.note
		return model, model.loadExecutions()
	case productionLoadedMsg:
		model.loading = false
		model.offline = false
		model.lastError = ""
		model.productionSnapshot = message.snapshot
		return model, nil
	case productionFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case productionCommandDoneMsg:
		model.loading = false
		if message.err != nil {
			model.lastError = safeClientState(message.err)
			return model, nil
		}
		model.productionPreview = message.preview
		model.lastError = message.note
		return model, model.loadProduction()
	case tea.KeyMsg:
		if model.entryMode != "" {
			return model.updateEntry(message)
		}
		key := message.String()
		if model.navigationPrefix {
			model.navigationPrefix = false
			switch key {
			case "b":
				model.switchScreen(indexOfScreen(ScreenBoard))
				return model, nil
			case "t":
				if model.currentMission == "" {
					if mission, ok := model.selectedMission(); ok {
						model.currentMission = mission.MissionID
					}
				}
				model.switchScreen(indexOfScreen(ScreenMission))
				return model, nil
			}
		}
		switch key {
		case "g":
			if (model.Screen() == ScreenPermissions ||
				model.Screen() == ScreenAttention) &&
				model.permissionClient != nil &&
				len(model.permissionAttention.Decisions) > 0 {
				model.loading = true
				return model, model.permissionGrantAlways()
			}
			if model.Screen() == ScreenTeamBuilder &&
				model.setupClient != nil {
				model.entryMode = entryCredentialPut
				if model.setup.MiniMax.CredentialReference != "" &&
					model.setup.MiniMax.Revision > 0 &&
					model.setup.MiniMax.Status != "revoked" {
					model.entryMode = entryCredentialSwap
				}
				model.entry = []byte{}
				return model, nil
			}
			model.navigationPrefix = true
			return model, nil
		case "q", "ctrl+c":
			model.cancel()
			return model, tea.Quit
		case "tab", "right":
			model.switchScreen(
				(model.screenIndex + 1) % len(screens),
			)
			if model.Screen() == ScreenAssets {
				model.loading = true
				return model, model.loadEvolutionAssets("")
			}
			if model.Screen() == ScreenQueue {
				model.loading = true
				return model, model.loadQueue()
			}
			if model.Screen() == ScreenWorkers {
				model.loading = true
				return model, model.loadWorkers()
			}
			if model.Screen() == ScreenIntegration {
				model.loading = true
				return model, model.loadIntegration()
			}
			if model.Screen() == ScreenPermissions && model.permissionClient != nil {
				model.loading = true
				return model, model.loadPermissions()
			}
			if model.Screen() == ScreenExecution && model.boundedExecutionClient != nil {
				model.loading = true
				return model, model.loadExecutions()
			}
			if model.Screen() == ScreenProduction && model.productionClient != nil {
				model.loading = true
				return model, model.loadProduction()
			}
			if model.Screen() == ScreenCustomerRules && model.customerRuleClient != nil {
				model.loading = true
				return model, model.loadCustomerRules()
			}
			if model.Screen() == ScreenAutonomy && model.standingOrderClient != nil {
				model.loading = true
				return model, model.loadStandingOrders()
			}
			if model.Screen() == ScreenAttention {
				return model.beginAttentionRefresh()
			}
			return model, nil
		case "shift+tab", "left":
			next := model.screenIndex - 1
			if next < 0 {
				next = len(screens) - 1
			}
			model.switchScreen(next)
			if model.Screen() == ScreenAssets {
				model.loading = true
				return model, model.loadEvolutionAssets("")
			}
			if model.Screen() == ScreenQueue {
				model.loading = true
				return model, model.loadQueue()
			}
			if model.Screen() == ScreenWorkers {
				model.loading = true
				return model, model.loadWorkers()
			}
			if model.Screen() == ScreenIntegration {
				model.loading = true
				return model, model.loadIntegration()
			}
			if model.Screen() == ScreenPermissions && model.permissionClient != nil {
				model.loading = true
				return model, model.loadPermissions()
			}
			if model.Screen() == ScreenExecution && model.boundedExecutionClient != nil {
				model.loading = true
				return model, model.loadExecutions()
			}
			if model.Screen() == ScreenProduction && model.productionClient != nil {
				model.loading = true
				return model, model.loadProduction()
			}
			if model.Screen() == ScreenCustomerRules && model.customerRuleClient != nil {
				model.loading = true
				return model, model.loadCustomerRules()
			}
			if model.Screen() == ScreenAutonomy && model.standingOrderClient != nil {
				model.loading = true
				return model, model.loadStandingOrders()
			}
			if model.Screen() == ScreenAttention {
				return model.beginAttentionRefresh()
			}
			return model, nil
		case "down", "j":
			if model.Screen() == ScreenBoard {
				model.moveTaskSelection(1)
				return model, nil
			}
			model.selected++
			model.clampSelection()
			return model, nil
		case "up", "k":
			if model.Screen() == ScreenBoard {
				model.moveTaskSelection(-1)
				return model, nil
			}
			if model.selected > 0 {
				model.selected--
			}
			return model, nil
		case "r":
			model.loading = true
			if model.Screen() == ScreenAssets {
				return model, model.loadEvolutionAssets("")
			}
			if model.Screen() == ScreenQueue {
				return model, model.loadQueue()
			}
			if model.Screen() == ScreenWorkers {
				return model, model.loadWorkers()
			}
			if model.Screen() == ScreenIntegration {
				return model, model.loadIntegration()
			}
			if model.Screen() == ScreenPermissions && model.permissionClient != nil {
				return model, model.loadPermissions()
			}
			if model.Screen() == ScreenExecution && model.boundedExecutionClient != nil {
				return model, model.loadExecutions()
			}
			if model.Screen() == ScreenProduction && model.productionClient != nil {
				return model, model.loadProduction()
			}
			if model.Screen() == ScreenCustomerRules && model.customerRuleClient != nil {
				return model, model.loadCustomerRules()
			}
			if model.Screen() == ScreenAutonomy && model.standingOrderClient != nil {
				return model, model.loadStandingOrders()
			}
			if model.Screen() == ScreenAttention {
				return model.beginAttentionRefresh()
			}
			if model.Screen() == ScreenTeamBuilder {
				return model, model.loadSetup()
			}
			if model.Screen() == ScreenTimeline &&
				model.currentTeam != "" {
				cursor := model.timeline.NextCursor
				if model.timeline.Gap != nil {
					cursor = ""
				}
				return model, model.loadTimeline(
					model.currentTeam,
					cursor,
				)
			}
			return model, model.loadSnapshot()
		case "?":
			model.help = !model.help
			return model, nil
		case "esc":
			if model.Screen() == ScreenAssets && model.pendingEvolutionAction != "" {
				model.pendingEvolutionAction = ""
				return model, nil
			}
			if model.Screen() == ScreenMission && model.sideTaskProposal.ProposalDigest != "" {
				model.sideTaskProposalReq = app.SideTaskProposalRequest{}
				model.sideTaskProposal = app.SideTaskProposalResult{}
				return model, nil
			}
			if model.Screen() == ScreenMission && model.decisionOpen {
				model.decisionOpen = false
				model.decisionCommand = app.MissionDecisionCommand{}
				model.decisionSheet = app.MissionDecisionSheet{}
				model.decisionActionIndex = 0
				return model, nil
			}
			if model.Screen() == ScreenMission ||
				model.Screen() == ScreenTimeline {
				model.switchScreen(indexOfScreen(ScreenBoard))
			} else if model.Screen() == ScreenNewMission {
				model.resetMissionDraft()
				model.switchScreen(indexOfScreen(ScreenBoard))
			} else if model.Screen() == ScreenTeamBuilder {
				clearTUIBytes(model.entry)
				model.entry = nil
				model.entryMode = ""
				model.builder = app.BuilderSessionView{}
				model.builderNotice = ""
			}
			return model, nil
		case "enter":
			if model.Screen() == ScreenBoard {
				if model.selected == 0 {
					model.resetMissionDraft()
					model.switchScreen(indexOfScreen(ScreenNewMission))
					return model, nil
				}
				if mission, ok := model.selectedMission(); ok {
					model.currentTeam = mission.TeamInstanceID
					model.currentMission = mission.MissionID
					model.switchScreen(indexOfScreen(ScreenMission))
					model.loading = true
					return model, model.loadTimeline(
						mission.TeamInstanceID,
						"",
					)
				}
			}
			if model.Screen() == ScreenNewMission {
				model.entryMode = entryMissionObjective
				model.entry = []byte(model.missionObjective)
				return model, nil
			}
			if model.Screen() == ScreenTeamBuilder {
				if model.builder.Question.ID != "" {
					if len(model.builder.Question.Options) == 0 {
						model.entryMode = entryBuilderAnswer
						model.entry = []byte{}
						return model, nil
					}
					optionIndex := min(
						model.selected,
						len(model.builder.Question.Options)-1,
					)
					option := model.builder.Question.Options[optionIndex]
					model.loading = true
					return model, model.answerBuilder(option.ID)
				}
				if model.builder.DraftID != "" {
					return model, nil
				}
				if model.setupSelectableCount() > 0 {
					model.loading = true
					return model, model.startSelectedSetupAsset()
				}
				model.loading = true
				return model, model.loadSetup()
			}
			if model.Screen() == ScreenTeams &&
				model.selected < len(model.snapshot.Teams) {
				teamID := model.snapshot.Teams[model.selected].TeamInstanceID
				model.currentTeam = teamID
				model.switchScreen(indexOfScreen(ScreenTimeline))
				model.loading = true
				return model, model.loadTimeline(teamID, "")
			}
			if (model.Screen() == ScreenRuns ||
				model.Screen() == ScreenCompare) &&
				model.selected < len(model.snapshot.Runs) {
				model.toggleComparedRun(
					model.snapshot.Runs[model.selected].RunID,
				)
				return model, nil
			}
		case "n":
			if model.Screen() == ScreenHome {
				model.resetMissionDraft()
				model.switchScreen(indexOfScreen(ScreenNewMission))
				return model, nil
			}
			if model.Screen() == ScreenCustomerRules && model.customerRuleClient != nil {
				model.loading = true
				return model, model.customerRuleDefineBuiltin()
			}
			if model.Screen() == ScreenPermissions && model.permissionClient != nil {
				model.entryMode = entryPermissionProfile
				model.entry = []byte{}
				return model, nil
			}
			if model.Screen() == ScreenQueue && model.queueClient != nil {
				model.entryMode = entryQueueJobPath
				model.entry = []byte{}
				return model, nil
			}
			if model.Screen() == ScreenAssets && model.assetClient != nil {
				model.evolutionCreateMode = "create_skill"
				model.entryMode = entryEvolutionAsset
				model.entry = []byte{}
				return model, nil
			}
			if model.Screen() == ScreenMission && model.handoffClient != nil &&
				model.sideTaskProposal.ProposalDigest == "" {
				model.entryMode = entrySideTaskRequest
				model.entry = []byte(model.sideTaskRequest)
				return model, nil
			}
			if model.Screen() == ScreenTeamBuilder &&
				model.setupClient != nil &&
				model.builder.DraftID == "" {
				model.loading = true
				return model, model.startBlankBuilder()
			}
		case "/":
			if model.Screen() == ScreenBoard {
				model.entryMode = entryTaskSearch
				model.entry = []byte(model.taskFilter)
				return model, nil
			}
			if model.Screen() == ScreenAssets {
				model.entryMode = entryEvolutionSearch
				model.entry = []byte(model.evolutionSearch)
				return model, nil
			}
		case "c":
			if model.Screen() == ScreenProduction &&
				model.productionClient != nil &&
				model.productionPreview.Digest != "" {
				model.loading = true
				return model, model.productionConfirmActivation()
			}
			if model.Screen() == ScreenWorkers && model.workersClient != nil {
				model.loading = true
				return model, model.claimFirstJob()
			}
			if model.Screen() == ScreenMission {
				if _, ok := model.currentMissionCancelBinding(); ok {
					model.loading = true
					return model, model.cancelMission()
				}
				return model, nil
			}
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.CanConfirm {
				model.loading = true
				return model, model.confirmBuilder()
			}
		case "a":
			if (model.Screen() == ScreenPermissions ||
				model.Screen() == ScreenAttention) &&
				model.permissionClient != nil &&
				model.hasInspectablePermissionApproval() {
				model.loading = true
				return model, model.permissionResolveDecision("allow")
			}
			if model.Screen() == ScreenIntegration && model.integrationClient != nil {
				model.loading = true
				return model, model.adoptFirstRelease()
			}
			if model.Screen() == ScreenAssets && len(model.evolutionAssets.Definitions) > 0 {
				model.pendingEvolutionAction = "activate"
				return model, nil
			}
			if model.Screen() == ScreenMission {
				if _, ok := model.preparedMissionDecision(model.currentMission); ok &&
					model.decisionClient != nil {
					model.loading = true
					return model, model.readMissionDecision()
				}
				return model, nil
			}
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.DraftID == "" &&
				model.selected < len(model.setup.SavedTeams) &&
				model.setup.SavedTeams[model.selected].Status == "active" {
				model.loading = true
				return model, model.setSelectedTeamStatus(false)
			}
		case "u":
			if model.Screen() == ScreenHome {
				model.switchScreen(indexOfScreen(ScreenTeamBuilder))
				model.loading = true
				return model, model.loadSetup()
			}
			if model.Screen() == ScreenAssets && len(model.evolutionAssets.Definitions) > 0 {
				model.pendingEvolutionAction = "rollback"
				return model, nil
			}
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.DraftID == "" &&
				model.selected < len(model.setup.SavedTeams) &&
				model.setup.SavedTeams[model.selected].Status == "archived" {
				model.loading = true
				return model, model.setSelectedTeamStatus(true)
			}
		case "K":
			if model.Screen() == ScreenAutonomy &&
				model.standingOrderClient != nil &&
				model.standingOrderSelected < len(model.standingOrderSnapshot.Orders) {
				orderID := model.standingOrderSnapshot.Orders[model.standingOrderSelected].Order.OrderID
				model.loading = true
				return model, model.standingOrderActivate(orderID)
			}
		case "L":
			if model.Screen() == ScreenAutonomy &&
				model.standingOrderClient != nil &&
				model.standingOrderSelected < len(model.standingOrderSnapshot.Orders) {
				orderID := model.standingOrderSnapshot.Orders[model.standingOrderSelected].Order.OrderID
				model.loading = true
				return model, model.standingOrderRevoke(orderID)
			}
		case "e":
			if model.Screen() == ScreenCustomerRules && model.customerRuleClient != nil {
				model.loading = true
				return model, model.customerRuleEvaluate()
			}
			if model.Screen() == ScreenAssets && len(model.evolutionAssets.Definitions) > 0 {
				model.pendingEvolutionAction = "evaluate"
				return model, nil
			}
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.CanConfirm {
				model.entryMode = entryEditName
				model.entry = []byte{}
				return model, nil
			}
		case "p":
			if model.Screen() == ScreenHome {
				model.governancePanelVisible = true
				model.governancePanelPinned = !model.governancePanelPinned
				return model, nil
			}
			if model.Screen() == ScreenExecution && model.boundedExecutionClient != nil {
				model.loading = true
				return model, model.proposeExecutionProbe()
			}
			if model.Screen() == ScreenProduction && model.productionClient != nil {
				model.loading = true
				return model, model.productionPreviewActivation()
			}
			if model.Screen() == ScreenAssets && len(model.evolutionAssets.PromotionSources) > 0 {
				model.pendingEvolutionAction = "promote"
				return model, nil
			}
			if model.Screen() == ScreenNewMission &&
				model.canPreflightMission() {
				model.loading = true
				return model, model.preflightMission()
			}
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.CanConfirm {
				model.entryMode = entryEditPurpose
				model.entry = []byte{}
				return model, nil
			}
		case "m":
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.CanConfirm {
				field, value, ok := model.nextRoleOption("main")
				if ok {
					model.loading = true
					return model, model.editBuilder(field, value)
				}
			}
		case "s":
			if model.Screen() == ScreenNewMission &&
				model.missionPreflight.PreflightDigest != "" {
				model.loading = true
				return model, model.startMission()
			}
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.CanConfirm {
				field, value, ok := model.nextRoleOption("subagent")
				if ok {
					model.loading = true
					return model, model.editBuilder(field, value)
				}
			}
		case "t":
			if model.Screen() == ScreenWorkers && model.workersClient != nil {
				model.loading = true
				return model, model.recordTestSuccess()
			}
			if model.Screen() == ScreenAssets && model.assetClient != nil {
				model.evolutionCreateMode = "team_template"
				model.entryMode = entryEvolutionAsset
				model.entry = []byte{}
				return model, nil
			}
			if model.Screen() == ScreenNewMission {
				model.cycleMissionTeam()
				return model, nil
			}
		case "w":
			if model.Screen() == ScreenNewMission {
				model.missionPackageIndex = (model.missionPackageIndex + 1) % 2
				model.invalidateMissionPreflight()
				return model, nil
			}
		case "v":
			if model.Screen() == ScreenHome && model.governancePanelVisible {
				model.governanceViewIndex =
					(model.governanceViewIndex + 1) % len(governanceViews)
				return model, nil
			}
			if (model.Screen() == ScreenPermissions ||
				model.Screen() == ScreenAttention) &&
				model.permissionClient != nil {
				model.permissionDetail = !model.permissionDetail
				return model, nil
			}
			if model.Screen() == ScreenWorkers && model.workersClient != nil {
				model.loading = true
				return model, model.recordReviewPass()
			}
			if model.Screen() == ScreenAssets && len(model.evolutionAssets.Definitions) > 0 {
				model.loading = true
				return model, model.compareSelectedEvolutionRevisions()
			}
			if model.Screen() == ScreenTeamBuilder &&
				model.setup.MiniMax.CredentialReference != "" &&
				model.setup.MiniMax.Revision > 0 {
				model.loading = true
				return model, model.verifyCredential()
			}
		case "x":
			if model.Screen() == ScreenProduction &&
				model.productionClient != nil &&
				model.productionPreview.Digest != "" &&
				model.productionPreview.Operation == "deactivation" {
				model.loading = true
				return model, model.productionConfirmDeactivation()
			}
			if model.Screen() == ScreenPermissions && model.permissionClient != nil {
				model.entryMode = entryPermissionCall
				model.entry = []byte{}
				return model, nil
			}
			if model.Screen() == ScreenAttention &&
				model.permissionClient != nil &&
				len(model.permissionAttention.Decisions) > 0 {
				model.loading = true
				return model, model.permissionResolveDecision("deny")
			}
			if model.Screen() == ScreenAssets && len(model.evolutionAssets.Definitions) > 0 {
				model.pendingEvolutionAction = "reject"
				return model, nil
			}
			if model.Screen() == ScreenMission && model.sideTaskProposal.ProposalDigest != "" {
				model.sideTaskProposalReq = app.SideTaskProposalRequest{}
				model.sideTaskProposal = app.SideTaskProposalResult{}
				return model, nil
			}
			if model.Screen() == ScreenTeamBuilder &&
				model.setup.MiniMax.CredentialReference != "" &&
				model.setup.MiniMax.Revision > 0 {
				model.loading = true
				return model, model.revokeCredential()
			}
		case "y":
			if model.Screen() == ScreenAssets && model.pendingEvolutionAction != "" {
				action := model.pendingEvolutionAction
				model.pendingEvolutionAction = ""
				model.loading = true
				if action == "promote" {
					return model, model.commitEvolutionPromotion()
				}
				return model, model.commitSelectedEvolutionAction(action)
			}
			if model.Screen() == ScreenMission && model.sideTaskProposal.ProposalDigest == "" {
				model.cycleSideTaskPurpose()
				return model, nil
			}
		case "z":
			if model.Screen() == ScreenAssets && len(model.evolutionAssets.Definitions) > 0 {
				model.pendingEvolutionAction = "instantiate"
				return model, nil
			}
		case "o":
			if model.Screen() == ScreenHome {
				model.entryMode = entryFolderPath
				model.entry = nil
				return model, nil
			}
			if model.Screen() == ScreenMission && model.sideTaskProposal.ProposalDigest == "" {
				model.cycleSideTaskMode()
				return model, nil
			}
		case "f":
			if model.Screen() == ScreenMission && model.sideTaskProposal.ProposalDigest != "" {
				model.loading = true
				return model, model.createSideTask()
			}
		case "[":
			if model.Screen() == ScreenMission && model.decisionOpen &&
				model.decisionActionIndex > 0 {
				model.decisionActionIndex--
				return model, nil
			}
			if model.Screen() == ScreenMission && model.sideTaskDecision > 0 {
				model.sideTaskDecision--
				return model, nil
			}
		case "]":
			if model.Screen() == ScreenHome {
				model.governancePanelVisible = !model.governancePanelVisible
				return model, nil
			}
			if model.Screen() == ScreenAssets && model.evolutionAssets.NextCursor != "" {
				model.loading = true
				return model, model.loadEvolutionAssets(model.evolutionAssets.NextCursor)
			}
			if model.Screen() == ScreenMission {
				if model.decisionOpen {
					choices := model.decisionSheet.PreparedActions
					if len(choices) > 0 {
						model.decisionActionIndex =
							(model.decisionActionIndex + 1) % len(choices)
					}
					return model, nil
				}
				if sideTask, ok := model.currentDecisionSideTask(); ok && len(sideTask.AvailableDecisions) > 0 {
					model.sideTaskDecision = (model.sideTaskDecision + 1) % len(sideTask.AvailableDecisions)
				}
				return model, nil
			}
		case "d":
			if model.Screen() == ScreenProduction && model.productionClient != nil {
				model.loading = true
				return model, model.productionPreviewDeactivation()
			}
			if model.Screen() == ScreenExecution {
				model.executionDetail = !model.executionDetail
				return model, nil
			}
			if model.Screen() == ScreenPermissions {
				model.permissionDetail = !model.permissionDetail
				return model, nil
			}
			if model.Screen() == ScreenAssets && len(model.evolutionAssets.Definitions) > 0 {
				model.pendingEvolutionAction = "archive_toggle"
				return model, nil
			}
			if model.Screen() == ScreenMission {
				if model.decisionOpen && len(model.decisionSheet.PreparedActions) > 0 {
					model.loading = true
					return model, model.submitMissionDecision()
				}
				if _, ok := model.currentDecisionSideTask(); ok {
					model.loading = true
					return model, model.decideSideTask()
				}
				return model, nil
			}
		case "h":
			if model.Screen() == ScreenAssets && len(model.evolutionAssets.Definitions) > 0 {
				model.pendingEvolutionAction = "retain"
				return model, nil
			}
		case "b":
			if model.Screen() == ScreenPermissions &&
				model.permissionClient != nil &&
				model.selected < len(model.queueSnapshot.Jobs) &&
				len(model.permissionSnapshot.Profiles) > 0 {
				model.loading = true
				return model, model.permissionBindSelected()
			}
			if model.Screen() == ScreenIntegration && model.integrationClient != nil {
				model.loading = true
				return model, model.rollbackFirstRelease()
			}
			if model.Screen() == ScreenAssets && len(model.evolutionAssets.Definitions) > 0 {
				model.pendingEvolutionAction = "bind"
				return model, nil
			}
		case "i":
			if model.Screen() == ScreenHome {
				model.entryMode = entryChatDraft
				model.entry = append([]byte(nil), model.chatDraft...)
				return model, nil
			}
			if model.Screen() == ScreenCustomerRules && model.customerRuleClient != nil {
				model.entryMode = entryCustomerRuleImport
				model.entry = []byte{}
				return model, nil
			}
			if model.Screen() == ScreenIntegration && model.integrationClient != nil {
				model.loading = true
				return model, model.integrateFirstCandidate()
			}
			if model.Screen() == ScreenAssets && model.assetClient != nil {
				model.evolutionCreateMode = "import_skill"
				model.entryMode = entryEvolutionAsset
				model.entry = []byte{}
				return model, nil
			}
		case "1", "2", "3", "4":
			if model.Screen() == ScreenAssets && model.assetClient != nil {
				modes := map[string]string{
					"1": "agent_template", "2": "team_template",
					"3": "work_package_template", "4": "recovery_strategy_template",
				}
				model.evolutionCreateMode = modes[key]
				model.entryMode = entryEvolutionAsset
				model.entry = []byte{}
				return model, nil
			}
		}
	}
	return model, nil
}

func (model Model) View() string {
	if model.width < 60 || model.height < 16 {
		return "Loom needs at least 60 columns × 16 rows. Resize the terminal.\n"
	}
	var builder strings.Builder
	builder.WriteString(styleHeader("Loom · " + string(model.Screen())))
	builder.WriteByte('\n')
	builder.WriteString(styleDivider(min(model.width, 72)))
	builder.WriteByte('\n')
	switch {
	case model.loading:
		builder.WriteString(styleLoading("Starting local service...") + "\n")
	case model.offline:
		builder.WriteString(styleError("Local service unavailable · press r to try again") + "\n")
	case model.lastError != "":
		builder.WriteString(styleError("State unavailable · "))
		builder.WriteString(styleError(sanitizeCell(model.lastError, 80)))
		builder.WriteByte('\n')
	default:
		body := model.screenBody()
		if model.Screen() == ScreenHome && model.governancePanelVisible {
			body = model.renderHomeGovernanceLayout(body)
		}
		builder.WriteString(body)
	}
	if model.snapshot.Stale {
		builder.WriteString("\n")
		builder.WriteString(styleWarningBanner("Showing last loaded state · press r to refresh"))
		builder.WriteByte('\n')
	}
	if model.snapshot.Partial {
		builder.WriteString(
			"\n" + styleWarningBanner("partial · more records are available on another bounded page") + "\n",
		)
	}
	if model.help {
		builder.WriteString(
			"\n" + styleHelp("Keys: tab/shift+tab views · j/k select · enter open · r refresh · q quit") + "\n",
		)
	}
	builder.WriteString("\n")
	builder.WriteString(styleKeyHint(screenKeyHint(model.Screen())))
	builder.WriteString("\n")
	return clipView(builder.String(), model.width, model.height)
}

func (model Model) renderHomeGovernanceLayout(conversation string) string {
	conversation = strings.Replace(conversation, styleSection("Chat"), styleSection("Conversation"), 1)
	governance := model.renderGovernanceSummary()
	if model.width < 120 {
		return strings.TrimRight(conversation, "\n") + "\n" +
			styleDivider(min(model.width, 72)) + "\n" + governance
	}

	leftWidth := model.width - 42
	rightWidth := 38
	left := lipgloss.NewStyle().
		Width(leftWidth).
		MaxWidth(leftWidth).
		PaddingRight(2).
		Render(strings.TrimRight(conversation, "\n"))
	right := lipgloss.NewStyle().
		Width(rightWidth).
		MaxWidth(rightWidth).
		BorderLeft(true).
		BorderForeground(loomTextMuted).
		PaddingLeft(2).
		Render(strings.TrimRight(governance, "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right) + "\n"
}

func (model Model) renderGovernanceSummary() string {
	viewIndex := model.governanceViewIndex
	if viewIndex < 0 || viewIndex >= len(governanceViews) {
		viewIndex = 0
	}
	active := 0
	for _, mission := range model.snapshot.Missions {
		if mission.Lane != api.MissionLaneComplete {
			active++
		}
	}
	lines := []string{styleSection("Governance · " + governanceViews[viewIndex])}
	switch governanceViews[viewIndex] {
	case "Overview":
		lines = append(
			lines,
			fmt.Sprintf("Active missions · %d", active),
			fmt.Sprintf("Needs you · %d", len(model.snapshot.Attention)),
			fmt.Sprintf("Teams · %d", len(model.snapshot.Teams)),
			fmt.Sprintf("Runtimes · %d", len(model.snapshot.Runtimes)),
		)
	case "Board":
		for _, mission := range model.snapshot.Missions[:min(6, len(model.snapshot.Missions))] {
			lines = append(lines, fmt.Sprintf(
				"· %s · %s",
				sanitizeCell(model.missionDisplayTitle(mission), 22),
				sanitizeCell(string(mission.Lane), 14),
			))
		}
		if len(model.snapshot.Missions) == 0 {
			lines = append(lines, "No missions yet")
		}
	case "Teams":
		for _, team := range model.snapshot.Teams[:min(8, len(model.snapshot.Teams))] {
			lines = append(lines, "· "+sanitizeCell(model.teamDisplayName(team), 28))
		}
		if len(model.snapshot.Teams) == 0 {
			lines = append(lines, "No Agent Teams yet")
		}
	case "Decisions":
		for _, decision := range model.snapshot.PreparedDecisions[:min(8, len(model.snapshot.PreparedDecisions))] {
			lines = append(lines, fmt.Sprintf(
				"· %s · %s",
				humanizeStatus(decision.Kind),
				humanizeStatus(decision.Action),
			))
		}
		if len(model.snapshot.PreparedDecisions) == 0 {
			lines = append(lines, "No decisions need review")
		}
	case "Evidence":
		for index, evidence := range model.snapshot.Evidence[:min(8, len(model.snapshot.Evidence))] {
			lines = append(lines, fmt.Sprintf(
				"· Evidence %d · %s",
				index+1,
				shortTUIDigest(evidence.Digest),
			))
		}
		if len(model.snapshot.Evidence) == 0 {
			lines = append(lines, "No accepted Evidence yet")
		}
	case "Runtimes":
		for _, runtime := range model.snapshot.Runtimes[:min(8, len(model.snapshot.Runtimes))] {
			lines = append(lines, fmt.Sprintf(
				"· %s · %s",
				sanitizeCell(model.runtimeDisplayName(runtime.RuntimeInstanceID), 22),
				humanizeStatus(runtime.Status),
			))
		}
		if len(model.snapshot.Runtimes) == 0 {
			lines = append(lines, "No runtimes discovered")
		}
	case "Attention":
		for _, item := range model.snapshot.Attention[:min(8, len(model.snapshot.Attention))] {
			lines = append(lines, "· "+sanitizeCell(item.ActionRequired, 30))
		}
		if len(model.snapshot.Attention) == 0 {
			lines = append(lines, "Nothing needs you")
		}
	}
	if governanceViews[viewIndex] == "Overview" && len(model.snapshot.Missions) > 0 {
		lines = append(lines, "", styleSection("Mission pulse"))
		for _, mission := range model.snapshot.Missions[:min(3, len(model.snapshot.Missions))] {
			lines = append(lines, fmt.Sprintf(
				"· %s · %s",
				sanitizeCell(model.missionDisplayTitle(mission), 22),
				sanitizeCell(string(mission.Lane), 14),
			))
		}
	}
	mode := "not pinned"
	if model.governancePanelPinned {
		mode = "pinned"
	}
	lines = append(lines, "", styleHelp("v next view · ] close · p "+mode+" · g b Board"))
	return strings.Join(lines, "\n") + "\n"
}

func (model Model) Screen() Screen {
	if model.screenIndex < 0 || model.screenIndex >= len(screens) {
		return ScreenBoard
	}
	return screens[model.screenIndex]
}

func (model Model) screenBody() string {
	switch model.Screen() {
	case ScreenHome:
		lines := []string{
			styleTitle("Start with a task"),
			"Describe the outcome you want. Loom stays in conversation until you choose to use an Agent Team.",
			"",
			"› New Mission · describe the outcome and choose a Team",
			"",
			styleSection("Chat"),
		}
		if len(model.chatThread.Messages) == 0 {
			lines = append(lines, "No messages yet. Press i to write a message.")
		} else {
			for _, message := range model.chatThread.Messages {
				prefix := "Loom"
				if message.Role == "user" {
					prefix = "You"
				} else if message.Tentative {
					prefix = "Loom proposal (untrusted)"
				}
				content := message.DisplayContent()
				if content == api.ToolShapedChatWarning {
					lines = append(
						lines,
						prefix+" · The conversation runtime returned tool-shaped text.",
						"  Loom did not execute it.",
					)
					continue
				}
				lines = append(lines, fmt.Sprintf("%s · %s", prefix, sanitizeCell(content, 72)))
			}
		}
		if model.entryMode == entryChatDraft {
			lines = append(lines, "", "Draft · "+sanitizeCell(string(model.entry), 72))
		}
		if model.entryMode == entryFolderPath {
			lines = append(lines, "", "Folder path · "+sanitizeCell(string(model.entry), 72))
		} else if model.workspaceFolder != "" {
			lines = append(lines, "", "Folder · "+sanitizeCell(model.workspaceFolder, 72))
		}
		lines = append(lines, "", styleHelp("i message · n New Mission · o folder · u Agent Team · g b Board · r refresh · q quit"))
		return strings.Join(lines, "\n") + "\n"
	case ScreenBoard:
		lines := []string{
			styleSection("Lanes | Missions | Mission Detail"),
			styleSection("Proposed · Ready · Orchestrating · Review · Complete"),
		}
		newMissionMarker := " "
		if model.selected == 0 {
			newMissionMarker = styleSelectedMarker("›")
		}
		newMissionLine := newMissionMarker + " New Mission · describe the outcome and choose a Team"
		if model.selected == 0 {
			newMissionLine = styleSelected(newMissionLine)
		}
		lines = append(
			lines,
			newMissionLine,
		)
		if model.taskFilter != "" {
			lines = append(
				lines,
				styleHelp("Filter · "+sanitizeCell(model.taskFilter, 48)),
			)
		}
		for index, mission := range model.filteredMissions() {
			marker := " "
			if index+1 == model.selected {
				marker = styleSelectedMarker("›")
			}
			row := fmt.Sprintf(
				"%s %s · %s · %s",
				marker,
				model.missionDisplayTitle(mission),
				sanitizeCell(string(mission.Lane), 18),
				styleStatus(humanizeStatus(mission.Status)),
			)
			if index+1 == model.selected {
				row = styleSelected(row)
			}
			lines = append(lines, row)
		}
		if mission, ok := model.selectedMission(); ok {
			lines = append(
				lines,
				"",
				fmt.Sprintf(
					styleTitle("Mission Detail")+" · %s · %s priority",
					model.missionDisplayTitle(mission),
					styleStatus(humanizeStatus(mission.Priority)),
				),
			)
			if len(mission.TeamPulse) == 0 {
				lines = append(lines, styleSection("Team · no active presence"))
			} else {
				pulse := mission.TeamPulse[0]
				lines = append(lines, fmt.Sprintf(
					styleSection("Team")+" · %s · %s · Attempt %d",
					styleStatus(humanizeStatus(pulse.Role)),
					styleStatus(humanizeStatus(pulse.State)),
					pulse.AttemptNumber,
				))
			}
			lines = append(lines, styleSection("Current node")+" · "+
				model.missionNodeDisplayTitle(mission, mission.CurrentNodeID))
			if decision, available :=
				model.preparedMissionDecision(mission.MissionID); available {
				action := ""
				if model.decisionClient != nil {
					action = " · a open"
				}
				lines = append(lines, fmt.Sprintf(
					styleSection("Decision")+" · %s prepared%s",
					styleStatus(humanizeStatus(decision.Kind)),
					action,
				))
			} else if mission.AttentionCount > 0 {
				lines = append(
					lines,
					styleSection("Decision · attention exists · no prepared mutation"),
				)
			} else {
				lines = append(lines, styleSection("Decision · none required"))
			}
			lines = append(
				lines,
				styleSection("Milestone")+" · "+sanitizeCell(mission.LastMilestone, 72),
			)
		}
		if len(model.snapshot.Runs) > 0 {
			lines = append(lines, fmt.Sprintf(
				styleSection("  Recent work")+" · %d item(s)",
				len(model.snapshot.Runs),
			))
		}
		if len(model.snapshot.Attention) > 0 {
			lines = append(lines, fmt.Sprintf(
				styleSection("  Needs your attention")+" · %d item(s)",
				len(model.snapshot.Attention),
			))
		}
		lines = append(
			lines,
			"",
			styleHelp("/ filters Missions · enter opens · g b Board · g t current Mission"),
		)
		return strings.Join(lines, "\n") + "\n"
	case ScreenNewMission:
		return model.renderNewMission()
	case ScreenMission:
		mission, ok := model.currentMissionRecord()
		if !ok {
			return "Mission Detail\nChoose a Mission from the Board.\n"
		}
		if model.decisionOpen {
			return model.renderMissionDecision(mission)
		}
		lines := []string{
			"Mission Detail",
			fmt.Sprintf(
				"%s · %s · %s",
				model.missionDisplayTitle(mission),
				sanitizeCell(string(mission.Lane), 18),
				humanizeStatus(mission.Status),
			),
			"Team | Plan | Changes | Evidence",
			fmt.Sprintf(
				"Nodes %d · Complete %d · Review %d · Active %d",
				mission.NodeCount,
				mission.CompletedNodeCount,
				mission.ReviewNodeCount,
				mission.ActiveNodeCount,
			),
		}
		if model.handoffClient != nil {
			if model.entryMode == entrySideTaskRequest {
				lines = append(lines,
					"Side-task request · "+sanitizeCell(string(model.entry), 72),
					"enter review zero-write proposal · esc clear",
				)
			}
			if model.sideTaskProposal.ProposalDigest == "" {
				lines = append(lines, fmt.Sprintf(
					"New Side-task · n request · y purpose %s · o mode %s",
					humanizeStatus(model.sideTaskPurpose), humanizeStatus(model.sideTaskMode),
				))
			} else {
				lines = append(lines,
					"Side-task proposal ready · zero writes so far · f Confirm and run · x Discard",
				)
			}
		}
		if _, prepared := model.preparedMissionDecision(mission.MissionID); prepared && model.decisionClient != nil {
			lines = append(
				lines,
				"Needs You · prepared decision required · a open Approval",
			)
		} else if mission.AttentionCount > 0 {
			lines = append(
				lines,
				"Needs You · review in a supported decision client",
			)
		}
		for _, pulse := range mission.TeamPulse {
			lines = append(lines, fmt.Sprintf(
				"Team Pulse · %s · Attempt %d · %s",
				humanizeStatus(pulse.Role),
				pulse.AttemptNumber,
				humanizeStatus(pulse.State),
			))
		}
		for _, node := range mission.Topology {
			if len(node.DependsOn) == 0 {
				continue
			}
			lines = append(lines, fmt.Sprintf(
				"Plan · %s waits for %s",
				model.missionNodeDisplayTitle(mission, node.LogicalNodeID),
				model.missionDependencyDisplayTitles(mission, node.DependsOn),
			))
		}
		sideTaskCount := 0
		for _, sideTask := range model.snapshot.SideTasks {
			if sideTask.ParentMissionID != mission.MissionID {
				continue
			}
			if sideTaskCount == 0 {
				lines = append(lines, "Side-tasks")
			}
			sideTaskCount++
			lines = append(lines, fmt.Sprintf(
				"  %s · %s · %s",
				sanitizeCell(sideTask.Title, 48),
				humanizeStatus(sideTask.Mode),
				humanizeStatus(sideTask.Status),
			))
			if sideTask.WhatHappened != "" {
				lines = append(lines, "    "+sanitizeCell(sideTask.WhatHappened, 72))
			}
			if sideTask.Risk != "" || len(sideTask.Uncertainties) > 0 {
				lines = append(lines, fmt.Sprintf(
					"    Risk %s · Uncertainty %d · Evidence %d · Artifacts %d",
					humanizeStatus(sideTask.Risk), len(sideTask.Uncertainties),
					len(sideTask.EvidenceReferences), len(sideTask.ArtifactReferences),
				))
			}
			for _, finding := range sideTask.AuthorizedFindings {
				lines = append(lines, "    Finding · "+sanitizeCell(finding, 64))
			}
			if len(sideTask.EvidenceReferences) > 0 {
				reference := sideTask.EvidenceReferences[0]
				lines = append(lines, "    Evidence ref · "+
					sanitizeCell(reference.Kind+" · "+reference.EvidenceID, 68))
			}
			if len(sideTask.ArtifactReferences) > 0 {
				reference := sideTask.ArtifactReferences[0]
				digest := reference.Digest
				if len(digest) > 12 {
					digest = digest[:12] + "…"
				}
				lines = append(lines, "    Artifact ref · "+reference.Kind+" · "+digest)
			}
			if sideTask.UsageObserved {
				lines = append(lines, fmt.Sprintf(
					"    Usage · %d microunits %s",
					sideTask.UsageMicrounits, sideTask.UsageCurrency,
				))
			} else {
				lines = append(lines, "    Usage · Not observed")
			}
		}
		if sideTask, ok := model.currentDecisionSideTask(); ok {
			index := model.sideTaskDecision
			if index >= len(sideTask.AvailableDecisions) {
				index = 0
			}
			lines = append(lines, fmt.Sprintf(
				"Side-task next action · %s · [ ] choose · d Apply exact action",
				humanizeStatus(sideTask.AvailableDecisions[index]),
			))
		}
		for _, record := range model.timeline.Records {
			if record.Payload.TextDelta != "" {
				lines = append(
					lines,
					"Tentative output · "+sanitizeCell(record.Payload.TextDelta, 72),
				)
			} else if record.Payload.Status != "" {
				lines = append(
					lines,
					"Timeline · "+humanizeStatus(record.Payload.Status),
				)
			} else if record.Kind != "" {
				lines = append(
					lines,
					"Timeline · "+humanizeStatus(record.Kind),
				)
			}
		}
		if _, ok := model.currentMissionCancelBinding(); ok {
			lines = append(lines, "c Cancel Mission · exact current Attempt only")
		}
		return strings.Join(lines, "\n") + "\n"
	case ScreenRuntimes:
		lines := make([]string, 0, len(model.snapshot.Runtimes)+1)
		lines = append(lines, "Discovered local Runtimes")
		for _, runtime := range model.snapshot.Runtimes {
			lines = append(lines, fmt.Sprintf(
				"• %s · %s · %s",
				model.runtimeDisplayName(runtime.RuntimeInstanceID),
				sanitizeCell(runtime.Status, 24),
				sanitizeCell(runtime.ExecutableVersion, 24),
			))
		}
		return emptyOrLines(lines, len(model.snapshot.Runtimes))
	case ScreenTeamBuilder:
		return model.renderTeamBuilder()
	case ScreenAssets:
		lines := []string{
			"Evolution Assets · exact revision lineage",
			"Journey · " + model.evolutionJourneyID,
			"/ search · n local Skill · i import Skill · 1-4 templates · p promote Run",
			"e evaluate · a activate · t Team template · z instantiate · b bind Coding · v diff",
			"x reject · h keep · d archive/restore · u rollback · ] next page · r refresh",
		}
		if model.evolutionSearch != "" {
			lines = append(lines, "Filter · "+sanitizeCell(model.evolutionSearch, 48))
		}
		if model.pendingEvolutionAction != "" {
			lines = append(lines, "Confirm "+model.pendingEvolutionAction+"? y confirm · esc cancel")
		}
		if len(model.evolutionAssets.Definitions) == 0 {
			lines = append(lines, "No Evolution Assets. Create a Candidate to begin.")
		}
		for _, source := range model.evolutionAssets.PromotionSources {
			lines = append(lines, fmt.Sprintf(
				"Promotion source · %s · generation %d · %s · Evidence %s",
				source.RunID, source.RunGeneration, shortTUIDigest(source.RunDigest),
				strings.Join(source.EvidenceIDs, ","),
			))
		}
		for index, definition := range model.evolutionAssets.Definitions {
			marker := " "
			if index == model.selected {
				marker = "›"
			}
			lines = append(lines, fmt.Sprintf("%s %s · %s", marker,
				sanitizeCell(definition.Name, 40),
				humanizeStatus(string(definition.Lifecycle))))
			for _, revision := range model.evolutionAssets.Revisions {
				if revision.DefinitionID != definition.DefinitionID {
					continue
				}
				digest := revision.ArtifactDigest
				if len(digest) > 12 {
					digest = digest[:12] + "…"
				}
				lines = append(lines, "    Exact revision · "+revision.RevisionID+" · "+digest,
					"    Risk · "+string(revision.Risk)+" · Materialization · not published")
			}
		}
		for _, evaluation := range model.evolutionAssets.Evaluations {
			usage, cost := "unknown", "unknown"
			if evaluation.UsageObserved {
				usage = "observed"
			}
			if evaluation.CostObserved {
				cost = "observed"
			}
			lines = append(lines, "Eval · "+evaluation.EvaluationID+" · "+evaluation.QualityResult+" · usage "+usage+" · cost "+cost)
		}
		for _, binding := range model.evolutionAssets.Bindings {
			lines = append(lines, "Binding · "+binding.SubjectKind+"/"+binding.SubjectID+" · "+shortTUIDigest(binding.AssetRevisionSetDigest))
		}
		for _, materialization := range model.evolutionAssets.Materializations {
			state := "published"
			if materialization.Cleaned {
				state = "cleaned"
			}
			lines = append(lines, fmt.Sprintf("Run pin · %s · attempt %d · generation %d · %s", materialization.RunID, materialization.AttemptNumber, materialization.Generation, state))
		}
		if model.evolutionAssetDiff.DefinitionID != "" {
			lines = append(lines, "Diff · "+model.evolutionAssetDiff.LeftRevisionID+" → "+model.evolutionAssetDiff.RightRevisionID)
			for _, change := range model.evolutionAssetDiff.Changes {
				lines = append(lines, "    "+change.Kind+" · "+change.RelativePath)
			}
		}
		return strings.Join(lines, "\n") + "\n"
	case ScreenTeams:
		lines := []string{"Confirmed and historical Teams"}
		if len(model.snapshot.Teams) == 0 {
			return lines[0] + "\nNo Teams exist in this Journal yet.\n"
		}
		for index, team := range model.snapshot.Teams {
			marker := " "
			if index == model.selected {
				marker = "›"
			}
			lines = append(lines, fmt.Sprintf(
				"%s %s · %s",
				marker,
				model.teamDisplayName(team),
				sanitizeCell(team.SourceKind, 32),
			))
		}
		return emptyOrLines(lines, len(model.snapshot.Teams))
	case ScreenRuns:
		lines := []string{"Recent work"}
		for index, run := range model.snapshot.Runs {
			marker := " "
			if index == model.selected {
				marker = "›"
			}
			lines = append(lines, fmt.Sprintf(
				"%s Work %d · %s · %s",
				marker,
				index+1,
				humanizeStatus(run.Phase),
				humanizeStatus(run.TerminalStatus),
			))
		}
		return emptyOrLines(lines, len(model.snapshot.Runs))
	case ScreenEvidence:
		lines := []string{"Accepted Evidence references"}
		for index, evidence := range model.snapshot.Evidence {
			lines = append(lines, fmt.Sprintf(
				"• Evidence %d · %s",
				index+1,
				sanitizeCell(evidence.Digest, 64),
			))
		}
		return emptyOrLines(lines, len(model.snapshot.Evidence))
	case ScreenCompare:
		return model.renderCompare()
	case ScreenAttention:
		lines := []string{"Needs your attention"}
		if model.permissionClient != nil && len(model.permissionAttention.Approvals) > 0 {
			lines[0] += " · x reject"
			if model.hasInspectablePermissionApproval() {
				lines[0] += " · a approve"
			}
			lines[0] += " · v view"
		}
		for _, item := range model.snapshot.Attention {
			lines = append(lines, fmt.Sprintf(
				"• %s",
				sanitizeCell(item.ActionRequired, 64),
			))
		}
		for _, decision := range model.permissionAttention.Decisions {
			lines = append(lines, fmt.Sprintf(
				"• Permission %s · %s · %s",
				sanitizeCell(decision.JobID, 24),
				sanitizeCell(decision.Reason, 40),
				sanitizeCell(decision.AuthorizationPath, 48),
			))
		}
		for _, approval := range model.permissionAttention.Approvals {
			lines = append(lines, fmt.Sprintf(
				"• Approval %s · %s · pending",
				sanitizeCell(approval.JobID, 24),
				sanitizeCell(approval.Command, 48),
			))
		}
		return emptyOrLines(
			lines,
			len(model.snapshot.Attention)+
				len(model.permissionAttention.Decisions)+
				len(model.permissionAttention.Approvals),
		)
	case ScreenPermissions:
		return model.renderPermissionsView()
	case ScreenCustomerRules:
		return model.renderCustomerRulesView()
	case ScreenAutonomy:
		return model.renderAutonomyView()
	case ScreenExecution:
		return model.renderExecutionsView()
	case ScreenProduction:
		return model.renderProductionView()
	case ScreenTimeline:
		lines := []string{"Team · Context · Changes · Evidence"}
		if model.currentTeam == "" &&
			model.timeline.TeamInstanceID == "" &&
			model.timeline.Gap == nil &&
			model.timeline.Board.Status == "" &&
			len(model.timeline.Board.Nodes) == 0 &&
			len(model.timeline.Attention) == 0 &&
			len(model.timeline.Records) == 0 {
			return lines[0] +
				"\nChoose a task to inspect its activity.\n"
		}
		if model.timeline.Gap != nil {
			recovery := "reopen the Team timeline"
			if model.timeline.Gap.Recoverable {
				recovery = "press r to reconnect"
			}
			lines = append(lines, fmt.Sprintf(
				"Some activity is unavailable · %s · %s",
				humanizeStatus(model.timeline.Gap.Reason),
				recovery,
			))
		}
		if model.timeline.Board.Status != "" {
			lines = append(lines, fmt.Sprintf(
				"Progress · %s · %d steps",
				humanizeStatus(model.timeline.Board.Status),
				len(model.timeline.Board.Nodes),
			))
		}
		for _, item := range model.timeline.Attention {
			lines = append(lines, fmt.Sprintf(
				"Needs your attention · %s",
				humanizeStatus(item.ActionRequired),
			))
		}
		for _, record := range model.timeline.Records {
			lines = append(lines, fmt.Sprintf(
				"• %s · %s · %s",
				humanizeStatus(record.Kind),
				humanizeStatus(record.Payload.Status),
				humanizeStatus(record.Payload.WarningCode),
			))
		}
		count := len(model.timeline.Records) +
			len(model.timeline.Attention)
		if model.timeline.Gap != nil || model.timeline.Board.Status != "" {
			count++
		}
		return emptyOrLines(lines, count)
	case ScreenQueue:
		return model.renderQueueView()
	case ScreenWorkers:
		return model.renderWorkersView()
	case ScreenIntegration:
		return model.renderIntegrationView()
	default:
		return ""
	}
}

func (model Model) hasInspectablePermissionApproval() bool {
	return len(model.permissionAttention.Approvals) > 0 &&
		model.permissionAttention.Approvals[0].DetailsAvailable
}

func (model Model) renderTeamBuilder() string {
	lines := []string{
		"What would you like Loom to help with?",
		"Build your team one step at a time.",
	}
	if model.builderNotice != "" {
		notice := "Draft changed in another client."
		if strings.Contains(model.builderNotice, "no longer available") {
			notice = "Agent Team draft no longer available."
		}
		lines = append(
			lines,
			"",
			notice,
			"Latest Teams refreshed. Press n to start a new draft.",
		)
	}
	if model.setupClient == nil {
		lines = append(lines, "Setup service unavailable")
		return strings.Join(lines, "\n") + "\n"
	}
	if model.setup.Codex.ProviderID != "" {
		lines = append(lines, fmt.Sprintf(
			"Codex · %s · %s",
			sanitizeCell(model.setup.Codex.AuthMode, 24),
			sanitizeCell(model.setup.Codex.Status, 24),
		))
	}
	if model.setup.MiniMax.ProviderID != "" {
		lines = append(lines, fmt.Sprintf(
			"MiniMax · %s · %s",
			sanitizeCell(model.setup.MiniMax.AuthMode, 24),
			sanitizeCell(model.setup.MiniMax.Status, 24),
		))
		if model.credential.Status != "" {
			lines = append(lines, fmt.Sprintf(
				"Credential result · %s",
				sanitizeCell(model.credential.Status, 24),
			))
		}
	}
	for _, runtime := range model.setup.Runtimes {
		lines = append(lines, fmt.Sprintf(
			"Runtime · %s · %s · %s",
			setupRuntimeDisplayName(runtime),
			humanizeStatus(sanitizeCell(runtime.Status, 20)),
			sanitizeCell(runtime.ExecutableVersion, 20),
		))
	}
	if model.builder.DraftID != "" {
		lines = append(lines, fmt.Sprintf(
			"Team setup · step %d",
			model.builder.Revision,
		))
		if model.builder.Question.Prompt != "" {
			lines = append(
				lines,
				sanitizeCell(model.builder.Question.Prompt, 72),
			)
			for index, option := range model.builder.Question.Options {
				marker := " "
				if index == model.selected {
					marker = "›"
				}
				lines = append(lines, fmt.Sprintf(
					"%s %s",
					marker,
					model.roleOptionSummary(
						option.ID,
						option.Label,
					),
				))
			}
			if len(model.builder.Question.Options) == 0 {
				lines = append(lines, "Press enter to type one bounded answer")
			}
		} else if model.builder.CanConfirm {
			lines = append(lines, renderBuilderPreflight(model.builder.Preview)...)
			lines = append(
				lines,
				renderBuilderRoleChoices(
					model.setup,
					model.builder.Preview,
				)...,
			)
			lines = append(
				lines,
				"Ready for explicit confirmation · no execution will start",
				"c confirm · m change Main role · s change SubAgent role",
				"e edit name · p edit purpose · esc cancel",
			)
		}
	} else {
		for index, team := range model.setup.SavedTeams {
			marker := " "
			if index == model.selected {
				marker = "›"
			}
			action := "enter open in Builder"
			if team.Status == "active" {
				action += " · a archive"
			} else {
				action = "u restore"
			}
			lines = append(lines, fmt.Sprintf(
				"%s Saved · %s · %s · %s",
				marker,
				sanitizeCell(team.Name, 32),
				sanitizeCell(team.Status, 16),
				action,
			))
		}
		for index, template := range model.setup.Templates {
			selection := len(model.setup.SavedTeams) + index
			marker := " "
			if selection == model.selected {
				marker = "›"
			}
			lines = append(lines, fmt.Sprintf(
				"%s Template · %s · enter open in Builder",
				marker,
				sanitizeCell(template.Name, 32),
			))
		}
		lines = append(lines, "n build a new team")
	}
	if model.setup.MiniMax.CredentialReference == "" ||
		model.setup.MiniMax.Status == "revoked" {
		lines = append(lines, "g store MiniMax credential securely")
	} else {
		lines = append(
			lines,
			"g replace · v test · x revoke MiniMax credential",
		)
	}
	if model.entryMode != "" {
		prompt := "Input"
		value := sanitizeCell(string(model.entry), 72)
		switch model.entryMode {
		case entryBuilderAnswer:
			prompt = "Answer"
		case entryEditName:
			prompt = "New team name"
		case entryEditPurpose:
			prompt = "New purpose"
		case entryCredentialPut:
			prompt = "MiniMax credential (masked)"
			value = strings.Repeat(
				"•",
				utf8.RuneCount(model.entry),
			)
		case entryCredentialSwap:
			prompt = "Replacement credential (masked)"
			value = strings.Repeat(
				"•",
				utf8.RuneCount(model.entry),
			)
		case entryTaskSearch:
			prompt = "Task filter"
		case entryEvolutionSearch:
			prompt = "Asset search"
		}
		lines = append(
			lines,
			prompt+" · "+value,
			"enter submit · esc clear",
		)
	}
	if model.confirmation.TeamDefinitionID != "" {
		lines = append(
			lines,
			"Your saved team is ready · no work has started",
		)
	}
	return strings.Join(lines, "\n") + "\n"
}

func (model Model) renderNewMission() string {
	lines := []string{
		"Describe the outcome, choose a confirmed Team, then review preflight.",
	}
	teamName := "No executable confirmed Team available"
	if team, ok := model.selectedMissionTeam(); ok {
		teamName = model.teamDisplayName(team)
	}
	packageName := "Coding"
	if model.missionPackageIndex == 1 {
		packageName = "Knowledge"
	}
	objective := "Not described yet · enter to describe"
	if model.missionObjective != "" {
		objective = sanitizeCell(model.missionObjective, 72)
	}
	lines = append(lines,
		"Outcome · "+objective,
		"Team · "+teamName+" · t change",
		"Work type · "+packageName+" · w change",
	)
	if model.entryMode == entryMissionObjective {
		lines = append(
			lines,
			"Outcome input · "+sanitizeCell(string(model.entry), 72),
			"enter save · esc clear",
		)
	}
	if model.missionPreflight.PreflightDigest == "" {
		if model.canPreflightMission() {
			lines = append(lines, "p Review preflight")
		} else {
			lines = append(lines, "Complete the outcome and choose a Team to review preflight")
		}
	} else {
		preflight := model.missionPreflight
		lines = append(lines,
			"",
			"Preflight ready · nothing has started",
			fmt.Sprintf(
				"Runtime · %s · %s · %d available",
				sanitizeCell(preflight.ModelID, 32),
				humanizeStatus(preflight.AuthMode),
				preflight.CapacityAvailable,
			),
			"Permissions · "+humanizeList(preflight.PermissionScopes),
			"Approval points · "+humanizeList(preflight.ApprovalPoints),
			fmt.Sprintf(
				"Plan · %d step(s) · budget %s",
				len(preflight.Nodes),
				humanizeStatus(preflight.BudgetStatus),
			),
			"s Start",
		)
	}
	if model.missionResult.Status != "" {
		lines = append(
			lines,
			"Execution accepted · "+humanizeStatus(model.missionResult.Status),
		)
	}
	lines = append(lines, "", "Nothing runs before Start · Esc returns to Board")
	return strings.Join(lines, "\n") + "\n"
}

func (model Model) eligibleMissionTeams() []api.LocalProductTeamSummary {
	teams := make([]api.LocalProductTeamSummary, 0, len(model.snapshot.Teams))
	for _, team := range model.snapshot.Teams {
		if team.Confirmed && team.Executable && !team.ReadOnly &&
			team.SourceKind == "saved_team" {
			teams = append(teams, team)
		}
	}
	return teams
}

func (model Model) selectedMissionTeam() (api.LocalProductTeamSummary, bool) {
	teams := model.eligibleMissionTeams()
	if len(teams) == 0 || model.missionTeamIndex < 0 ||
		model.missionTeamIndex >= len(teams) {
		return api.LocalProductTeamSummary{}, false
	}
	return teams[model.missionTeamIndex], true
}

func (model *Model) cycleMissionTeam() {
	teams := model.eligibleMissionTeams()
	if len(teams) == 0 {
		model.missionTeamIndex = 0
		return
	}
	model.missionTeamIndex = (model.missionTeamIndex + 1) % len(teams)
	model.invalidateMissionPreflight()
}

func (model Model) canPreflightMission() bool {
	_, ok := model.selectedMissionTeam()
	return ok && model.executionClient != nil &&
		strings.TrimSpace(model.missionObjective) != "" &&
		model.snapshot.ViewVersion != "" && !model.snapshot.Stale
}

func (model *Model) invalidateMissionPreflight() {
	model.missionID = ""
	model.missionPreflight = app.MissionExecutionPreflight{}
	model.missionResult = app.MissionExecutionResult{}
}

func (model *Model) resetMissionDraft() {
	clearTUIBytes(model.entry)
	model.entry = nil
	model.entryMode = ""
	model.missionID = ""
	model.missionObjective = ""
	model.missionTeamIndex = 0
	model.missionPackageIndex = 0
	model.missionPreflight = app.MissionExecutionPreflight{}
	model.missionResult = app.MissionExecutionResult{}
}

func (model Model) missionWorkPackage() (work.WorkPackage, error) {
	if model.missionPackageIndex == 1 {
		return work.KnowledgeWorkPackage()
	}
	return work.CodingWorkPackage()
}

func (model Model) preflightMission() tea.Cmd {
	client := model.executionClient
	readClient := model.client
	ctx := model.ctx
	team, teamOK := model.selectedMissionTeam()
	workPackage, packageErr := model.missionWorkPackage()
	missionID := "mission/" + team.TeamInstanceID
	objective := strings.TrimSpace(model.missionObjective)
	return func() tea.Msg {
		correlationID, correlationErr := newTUICorrelationID()
		if client == nil || !teamOK || packageErr != nil || correlationErr != nil {
			return missionExecutionFailedMsg{err: app.ErrInvalidMissionExecution}
		}
		snapshot, err := readClient.Snapshot(ctx, api.LocalProductSnapshotRequest{Limit: 64})
		if err != nil {
			return missionExecutionFailedMsg{err: err}
		}
		freshTeamFound := false
		for _, candidate := range snapshot.Teams {
			if candidate.TeamInstanceID == team.TeamInstanceID && candidate.Confirmed &&
				candidate.Executable && !candidate.ReadOnly {
				freshTeamFound = true
				break
			}
		}
		if !freshTeamFound {
			return missionExecutionFailedMsg{err: app.ErrMissionExecutionConflict}
		}
		envelope, err := client.ExecuteMission(ctx, app.MissionExecutionCommand{
			SchemaVersion:       app.MissionExecutionSchemaVersion,
			Operation:           "preflight",
			MissionID:           missionID,
			TeamInstanceID:      team.TeamInstanceID,
			WorkPackageID:       workPackage.ID(),
			WorkPackageDigest:   workPackage.Digest(),
			Objective:           objective,
			ExpectedViewVersion: snapshot.ViewVersion,
			CorrelationID:       correlationID,
		})
		if err != nil {
			return missionExecutionFailedMsg{err: err}
		}
		if envelope.Operation != "preflight" || envelope.Preflight == nil {
			return missionExecutionFailedMsg{err: localipc.ErrInvalidProtocol}
		}
		return missionPreflightedMsg{preflight: *envelope.Preflight, snapshot: snapshot}
	}
}

func (model Model) startMission() tea.Cmd {
	client := model.executionClient
	ctx := model.ctx
	preflight := model.missionPreflight
	objective := strings.TrimSpace(model.missionObjective)
	return func() tea.Msg {
		if client == nil || preflight.PreflightDigest == "" {
			return missionExecutionFailedMsg{err: app.ErrInvalidMissionExecution}
		}
		envelope, err := client.ExecuteMission(ctx, app.MissionExecutionCommand{
			SchemaVersion:       app.MissionExecutionSchemaVersion,
			Operation:           "start",
			MissionID:           preflight.MissionID,
			TeamInstanceID:      preflight.TeamInstanceID,
			WorkPackageID:       preflight.WorkPackageID,
			WorkPackageDigest:   preflight.WorkPackageDigest,
			Objective:           objective,
			ExpectedViewVersion: preflight.ViewVersion,
			PreflightDigest:     preflight.PreflightDigest,
			CorrelationID:       model.evolutionJourneyID,
		})
		if err != nil {
			return missionExecutionFailedMsg{err: err}
		}
		if envelope.Operation != "start" || envelope.Result == nil {
			return missionExecutionFailedMsg{err: localipc.ErrInvalidProtocol}
		}
		return missionStartedMsg{result: *envelope.Result}
	}
}

type missionCancelBinding struct {
	viewVersion     string
	logicalNodeID   string
	attemptNumber   int
	claimGeneration int64
}

type sideTaskParentBinding struct {
	teamInstanceID  string
	logicalNodeID   string
	attemptNumber   int
	workItemID      string
	runID           string
	claimGeneration int64
	executionDigest string
}

func (model Model) currentSideTaskParentBinding(expectedWorkItemID, expectedRunID string) (sideTaskParentBinding, bool) {
	result := model.missionResult
	if result.MissionID == "" || result.MissionID != model.currentMission ||
		result.TeamInstanceID == "" || result.TeamInstanceID != model.currentTeam ||
		len(result.ExecutionDigest) != 64 {
		return sideTaskParentBinding{}, false
	}
	return model.projectedSideTaskParentBinding(
		result.MissionID, result.TeamInstanceID, expectedWorkItemID, expectedRunID,
		result.ExecutionDigest,
	)
}

func (model Model) projectedSideTaskParentBinding(
	missionID, teamInstanceID, expectedWorkItemID, expectedRunID, executionDigest string,
) (sideTaskParentBinding, bool) {
	if missionID == "" || missionID != model.currentMission ||
		teamInstanceID == "" || teamInstanceID != model.currentTeam ||
		len(executionDigest) != 64 || len(model.snapshot.ViewVersion) != 64 ||
		model.timeline.ViewVersion != model.snapshot.ViewVersion ||
		model.timeline.Board.ViewVersion != model.snapshot.ViewVersion ||
		model.timeline.TeamInstanceID != teamInstanceID ||
		model.timeline.Board.TeamInstanceID != teamInstanceID {
		return sideTaskParentBinding{}, false
	}
	missionFound := false
	for _, mission := range model.snapshot.Missions {
		if mission.MissionID == missionID && mission.TeamInstanceID == teamInstanceID {
			missionFound = true
			break
		}
	}
	if !missionFound {
		return sideTaskParentBinding{}, false
	}
	for _, node := range model.timeline.Board.Nodes {
		if node.CurrentAttempt < 1 || node.RunID == "" || node.WorkItemID == "" {
			continue
		}
		if (expectedWorkItemID != "" && node.WorkItemID != expectedWorkItemID) ||
			(expectedRunID != "" && node.RunID != expectedRunID) {
			continue
		}
		for _, run := range model.snapshot.Runs {
			if run.RunID == node.RunID && run.WorkItemID == node.WorkItemID {
				return sideTaskParentBinding{
					teamInstanceID: teamInstanceID, logicalNodeID: node.LogicalNodeID,
					attemptNumber: node.CurrentAttempt, workItemID: node.WorkItemID,
					runID: node.RunID, claimGeneration: run.ClaimGeneration,
					executionDigest: executionDigest,
				}, true
			}
		}
	}
	return sideTaskParentBinding{}, false
}

func (model *Model) cycleSideTaskPurpose() {
	values := []string{"research", "comparison", "diagnosis", "verification", "read_only_review"}
	for index, value := range values {
		if value == model.sideTaskPurpose {
			model.sideTaskPurpose = values[(index+1)%len(values)]
			return
		}
	}
	model.sideTaskPurpose = values[0]
}

func (model *Model) cycleSideTaskMode() {
	values := []string{"report_only", "decision_required", "merge_candidate"}
	for index, value := range values {
		if value == model.sideTaskMode {
			model.sideTaskMode = values[(index+1)%len(values)]
			return
		}
	}
	model.sideTaskMode = values[0]
}

func (model Model) proposeSideTask() tea.Cmd {
	client := model.handoffClient
	ctx := model.ctx
	binding, ok := model.currentSideTaskParentBinding("", "")
	requestText := strings.TrimSpace(model.sideTaskRequest)
	title := strings.TrimSpace(model.sideTaskTitle)
	if title == "" {
		title = "Side task"
	}
	correlationID, correlationErr := newTUICorrelationID()
	request := app.SideTaskProposalRequest{
		SchemaVersion: 1, Operation: "propose", ParentMissionID: model.currentMission,
		ParentTeamInstanceID: binding.teamInstanceID, ParentTaskID: binding.workItemID,
		ParentRunID: binding.runID, ParentClaimGeneration: binding.claimGeneration,
		ParentExecutionDigest: binding.executionDigest,
		Purpose:               model.sideTaskPurpose, Mode: model.sideTaskMode, Title: title,
		AuthorizedRequest: requestText, PermissionScopes: []string{},
		ExpectedViewVersion: model.snapshot.ViewVersion, CorrelationID: correlationID,
	}
	if request.Mode != "report_only" {
		request.DecisionTimeoutSeconds = 900
	}
	return func() tea.Msg {
		if client == nil || !ok || correlationErr != nil || requestText == "" {
			return sideTaskFailedMsg{err: app.ErrInvalidSideTaskProduct}
		}
		result, err := client.ProposeSideTask(ctx, request)
		if err != nil {
			return sideTaskFailedMsg{err: err}
		}
		return sideTaskProposedMsg{request: request, result: result}
	}
}

func (model Model) createSideTask() tea.Cmd {
	client := model.handoffClient
	ctx := model.ctx
	request := model.sideTaskProposalReq
	proposal := model.sideTaskProposal
	request.Operation = "create"
	return func() tea.Msg {
		if client == nil || proposal.ProposalDigest == "" {
			return sideTaskFailedMsg{err: app.ErrInvalidSideTaskProduct}
		}
		result, err := client.CreateSideTask(ctx, app.SideTaskCreateRequest{
			SideTaskProposalRequest: request, ProposalDigest: proposal.ProposalDigest,
			Confirmed: true,
		})
		if err != nil {
			return sideTaskFailedMsg{err: err}
		}
		return sideTaskCreatedMsg{result: result}
	}
}

func (model Model) currentDecisionSideTask() (api.LocalProductSideTaskSummary, bool) {
	for _, sideTask := range model.snapshot.SideTasks {
		if sideTask.ParentMissionID == model.currentMission && len(sideTask.AvailableDecisions) > 0 {
			return sideTask, true
		}
	}
	return api.LocalProductSideTaskSummary{}, false
}

func (model Model) decideSideTask() tea.Cmd {
	client := model.handoffClient
	ctx := model.ctx
	sideTask, sideOK := model.currentDecisionSideTask()
	binding, bindingOK := model.projectedSideTaskParentBinding(
		sideTask.ParentMissionID, sideTask.ParentTeamInstanceID,
		sideTask.ParentTaskID, sideTask.ParentRunID, sideTask.ParentExecutionDigest,
	)
	index := model.sideTaskDecision
	if index >= len(sideTask.AvailableDecisions) {
		index = 0
	}
	decision := ""
	if len(sideTask.AvailableDecisions) > 0 {
		decision = sideTask.AvailableDecisions[index]
	}
	effect := sha256.Sum256([]byte(strings.Join(
		[]string{
			"parent-effect-v1", sideTask.SideTaskID, sideTask.ParentTeamInstanceID,
			sideTask.ParentTaskID, sideTask.ParentRunID,
			fmt.Sprint(sideTask.ParentClaimGeneration), sideTask.ParentExecutionDigest,
			sideTask.HandoffDigest, decision,
		}, "\x00",
	)))
	correlationID, correlationErr := newTUICorrelationID()
	request := app.SideTaskDecisionRequest{
		SchemaVersion: 1, Operation: "decide", SideTaskID: sideTask.SideTaskID,
		ParentMissionID: sideTask.ParentMissionID, ParentTeamInstanceID: sideTask.ParentTeamInstanceID,
		ParentTaskID: sideTask.ParentTaskID, ParentRunID: sideTask.ParentRunID,
		ParentLogicalNodeID: binding.logicalNodeID, ParentAttemptNumber: binding.attemptNumber,
		ParentClaimGeneration: sideTask.ParentClaimGeneration,
		ParentExecutionDigest: sideTask.ParentExecutionDigest, SideTaskGeneration: sideTask.SourceGeneration,
		HandoffVersion: sideTask.HandoffVersion, HandoffDigest: sideTask.HandoffDigest,
		Decision: decision, EffectDigest: hex.EncodeToString(effect[:]),
		ExpectedViewVersion: model.snapshot.ViewVersion, CorrelationID: correlationID,
	}
	return func() tea.Msg {
		if client == nil || !sideOK || !bindingOK || correlationErr != nil ||
			binding.workItemID != sideTask.ParentTaskID || binding.runID != sideTask.ParentRunID ||
			binding.claimGeneration != sideTask.ParentClaimGeneration ||
			binding.executionDigest != sideTask.ParentExecutionDigest {
			return sideTaskFailedMsg{err: app.ErrSideTaskProductConflict}
		}
		result, err := client.DecideSideTask(ctx, request)
		if err != nil {
			return sideTaskFailedMsg{err: err}
		}
		return sideTaskDecidedMsg{result: result}
	}
}

func (model Model) currentMissionCancelBinding() (missionCancelBinding, bool) {
	result := model.missionResult
	if result.MissionID == "" || result.MissionID != model.currentMission ||
		result.TeamInstanceID == "" || result.TeamInstanceID != model.currentTeam ||
		len(result.ExecutionDigest) != 64 ||
		model.snapshot.ViewVersion == "" ||
		model.timeline.ViewVersion != model.snapshot.ViewVersion ||
		model.timeline.Board.ViewVersion != model.snapshot.ViewVersion ||
		model.timeline.TeamInstanceID != model.currentTeam ||
		model.timeline.Board.TeamInstanceID != model.currentTeam {
		return missionCancelBinding{}, false
	}
	for _, node := range model.timeline.Board.Nodes {
		if node.CurrentAttempt < 1 || node.RunID == "" ||
			node.Status == "succeeded" || node.Status == "failed" ||
			node.Status == "cancelled" {
			continue
		}
		for _, run := range model.snapshot.Runs {
			if run.RunID == node.RunID && run.ClaimGeneration > 0 &&
				run.TerminalStatus == "" {
				return missionCancelBinding{
					viewVersion:     model.snapshot.ViewVersion,
					logicalNodeID:   node.LogicalNodeID,
					attemptNumber:   node.CurrentAttempt,
					claimGeneration: run.ClaimGeneration,
				}, true
			}
		}
	}
	return missionCancelBinding{}, false
}

func (model Model) cancelMission() tea.Cmd {
	client := model.executionClient
	ctx := model.ctx
	result := model.missionResult
	binding, ok := model.currentMissionCancelBinding()
	return func() tea.Msg {
		if client == nil || !ok {
			return missionExecutionFailedMsg{err: app.ErrMissionExecutionConflict}
		}
		correlationID, err := newTUICorrelationID()
		if err != nil {
			return missionExecutionFailedMsg{err: err}
		}
		envelope, err := client.ExecuteMission(ctx, app.MissionExecutionCommand{
			SchemaVersion: app.MissionExecutionSchemaVersion,
			Operation:     "control", MissionID: result.MissionID,
			TeamInstanceID:      result.TeamInstanceID,
			ExpectedViewVersion: binding.viewVersion,
			ControlAction:       "cancel", ExecutionDigest: result.ExecutionDigest,
			LogicalNodeID:   binding.logicalNodeID,
			AttemptNumber:   binding.attemptNumber,
			ClaimGeneration: binding.claimGeneration,
			CorrelationID:   correlationID,
		})
		if err != nil {
			return missionExecutionFailedMsg{err: err}
		}
		if envelope.Operation != "control" || envelope.Result == nil {
			return missionExecutionFailedMsg{err: localipc.ErrInvalidProtocol}
		}
		return missionStartedMsg{result: *envelope.Result}
	}
}

func (model Model) readMissionDecision() tea.Cmd {
	client := model.decisionClient
	ctx := model.ctx
	command, ok := model.preparedMissionDecision(model.currentMission)
	return func() tea.Msg {
		if client == nil || !ok || command.Operation != "read" ||
			command.Action != "read" || command.ViewVersion != model.snapshot.ViewVersion {
			return missionDecisionFailedMsg{err: app.ErrMissionDecisionConflict}
		}
		sheet, err := client.ReadMissionDecision(ctx, command)
		if err != nil {
			return missionDecisionFailedMsg{err: err}
		}
		if sheet.SchemaVersion != 1 || !sheet.Prepared ||
			sheet.MissionID != command.MissionID ||
			sheet.TeamInstanceID != command.TeamInstanceID ||
			sheet.ViewVersion != command.ViewVersion ||
			sheet.DecisionID != command.DecisionID ||
			sheet.DecisionDigest != command.DecisionDigest ||
			sheet.LogicalNodeID != command.LogicalNodeID ||
			sheet.AttemptNumber != command.AttemptNumber ||
			sheet.ClaimGeneration != command.ClaimGeneration {
			return missionDecisionFailedMsg{err: localipc.ErrInvalidProtocol}
		}
		return missionDecisionLoadedMsg{command: command, sheet: sheet}
	}
}

func (model Model) submitMissionDecision() tea.Cmd {
	client := model.decisionClient
	ctx := model.ctx
	command := model.decisionCommand
	sheet := model.decisionSheet
	index := model.decisionActionIndex
	return func() tea.Msg {
		if client == nil || index < 0 || index >= len(sheet.PreparedActions) ||
			command.DecisionID == "" || command.ViewVersion != sheet.ViewVersion {
			return missionDecisionFailedMsg{err: app.ErrMissionDecisionConflict}
		}
		action := sheet.PreparedActions[index]
		if !containsTUIString(sheet.Actions, action) {
			return missionDecisionFailedMsg{err: localipc.ErrInvalidProtocol}
		}
		correlationID, err := newTUICorrelationID()
		if err != nil {
			return missionDecisionFailedMsg{err: err}
		}
		command.Operation = "submit"
		command.Action = action
		command.CorrelationID = correlationID
		result, err := client.DecideMission(ctx, command)
		if err != nil {
			return missionDecisionFailedMsg{err: err}
		}
		if result.SchemaVersion != 1 || !result.Authoritative ||
			result.MissionID != command.MissionID ||
			result.DecisionID != command.DecisionID ||
			len(result.ViewVersion) != 64 {
			return missionDecisionFailedMsg{err: localipc.ErrInvalidProtocol}
		}
		return missionDecisionAppliedMsg{result: result}
	}
}

func (model Model) renderMissionDecision(
	mission api.LocalProductMissionSummary,
) string {
	sheet := model.decisionSheet
	lines := []string{
		humanizeStatus(sheet.Kind) + " Decision",
		"Mission · " + model.missionDisplayTitle(mission),
		"Request · " + sanitizeCell(sheet.Summary, 72),
		"Expected Evidence · " + sanitizeCell(sheet.ExpectedEvidence, 72),
	}
	if len(sheet.PreparedActions) == 0 {
		lines = append(
			lines,
			"No authoritative action is prepared",
			"Esc · Not now",
		)
		return strings.Join(lines, "\n") + "\n"
	}
	index := model.decisionActionIndex
	if index < 0 || index >= len(sheet.PreparedActions) {
		index = 0
	}
	for actionIndex, action := range sheet.PreparedActions {
		marker := " "
		if actionIndex == index {
			marker = "›"
		}
		lines = append(lines, marker+" "+humanizeStatus(action))
	}
	lines = append(lines, "[ ] choose · d Apply exact action · Esc Not now")
	return strings.Join(lines, "\n") + "\n"
}

func containsTUIString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (model *Model) applySnapshot(snapshot api.LocalProductSnapshot) {
	model.snapshot = cloneSnapshot(snapshot)
	if model.missionPreflight.PreflightDigest != "" &&
		model.missionPreflight.ViewVersion != model.snapshot.ViewVersion {
		model.missionPreflight = app.MissionExecutionPreflight{}
		model.missionResult = app.MissionExecutionResult{}
		model.lastError = "preflight_expired"
	}
	if model.currentMission != "" {
		if _, ok := model.currentMissionRecord(); !ok {
			model.currentMission = ""
			if model.Screen() == ScreenMission {
				model.switchScreen(indexOfScreen(ScreenBoard))
			}
		}
	}
	if len(model.snapshot.Teams) == 0 {
		model.currentTeam = ""
		model.timeline = api.LocalProductTimelinePage{}
	}
	if model.Screen() == ScreenBoard &&
		model.selected == 0 &&
		len(model.snapshot.Missions) > 0 {
		model.selected = 1
	}
	model.clampSelection()
}

func (model Model) loadSnapshot() tea.Cmd {
	client := model.client
	ctx := model.ctx
	return func() tea.Msg {
		snapshot, err := client.Snapshot(
			ctx,
			api.LocalProductSnapshotRequest{Limit: 64},
		)
		if err != nil {
			return snapshotFailedMsg{err: err}
		}
		return snapshotLoadedMsg{snapshot: snapshot}
	}
}

func (model Model) beginAttentionRefresh() (Model, tea.Cmd) {
	model.loading = true
	model.permissionAttention = app.PermissionAttention{}
	model.attentionGeneration++
	return model, model.loadAttention(model.attentionGeneration)
}

func (model Model) loadAttention(generation uint64) tea.Cmd {
	client, permissionClient, ctx := model.client, model.permissionClient, model.ctx
	journeyID := model.evolutionJourneyID
	return func() tea.Msg {
		snapshot, snapshotErr := client.Snapshot(
			ctx,
			api.LocalProductSnapshotRequest{Limit: 64},
		)
		message := attentionRefreshedMsg{
			generation:    generation,
			snapshot:      snapshot,
			snapshotErr:   snapshotErr,
			hasPermission: permissionClient != nil,
		}
		if permissionClient != nil {
			message.permissionAttention, message.permissionErr =
				permissionClient.PermissionAttention(
					ctx,
					app.PermissionAttentionRequest{JourneyID: journeyID},
				)
		}
		return message
	}
}

func (model Model) loadTimeline(teamID, cursor string) tea.Cmd {
	client := model.client
	ctx := model.ctx
	return func() tea.Msg {
		page, err := client.TimelinePage(
			ctx,
			api.LocalProductTimelineRequest{
				TeamInstanceID: teamID,
				Cursor:         cursor,
				Limit:          128,
			},
		)
		if err != nil {
			return timelineFailedMsg{err: err}
		}
		return timelineLoadedMsg{page: page}
	}
}

func (model Model) loadEvolutionAssets(cursor string) tea.Cmd {
	client, ctx, journeyID := model.assetClient, model.ctx, model.evolutionJourneyID
	searchText := model.evolutionSearch
	return func() tea.Msg {
		if client == nil {
			return evolutionAssetsFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		snapshot, err := client.EvolutionAssetSnapshot(ctx, api.EvolutionAssetSnapshotRequest{
			JourneyID: journeyID, Cursor: cursor, Limit: 64, SearchText: searchText,
		})
		if err != nil {
			return evolutionAssetsFailedMsg{err: err}
		}
		return evolutionAssetsLoadedMsg{snapshot: snapshot}
	}
}

func (model Model) createEvolutionAsset(sourcePath string) tea.Cmd {
	client, ctx, journeyID := model.assetClient, model.ctx, model.evolutionJourneyID
	viewVersion := model.evolutionAssets.ViewVersion
	createMode := model.evolutionCreateMode
	return func() tea.Msg {
		if client == nil || viewVersion == "" {
			return evolutionAssetsFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		if !filepath.IsAbs(sourcePath) {
			return evolutionAssetsFailedMsg{err: app.ErrInvalidLocalProductAsset}
		}
		data, readErr := os.ReadFile(sourcePath)
		if readErr != nil || len(data) == 0 || len(data) > 1<<20 {
			return evolutionAssetsFailedMsg{err: app.ErrInvalidLocalProductAsset}
		}
		name := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
		sum := sha256.Sum256(data)
		identity := hex.EncodeToString(sum[:])[:16]
		if createMode == "" {
			createMode = "create_skill"
		}
		action := createMode
		definitionID := "skill-" + identity
		revisionID := "revision-1"
		var input []byte
		var err error
		if createMode == "create_skill" || createMode == "import_skill" {
			_, artifactDigest, contentDigest, readErr := app.CanonicalEvolutionAssetArtifact(
				assets.AssetKindSkill, definitionID, revisionID, sourcePath,
			)
			if readErr != nil {
				return evolutionAssetsFailedMsg{err: app.ErrInvalidLocalProductAsset}
			}
			if createMode == "create_skill" {
				input, err = json.Marshal(struct {
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
				}{definitionID, revisionID, name, "Reviewed local Candidate", "project", sourcePath, artifactDigest, contentDigest, []string{}, []string{}, string(assets.RiskLow)})
			} else {
				input, err = json.Marshal(struct {
					CandidateID                   string   `json:"candidate_id"`
					DefinitionID                  string   `json:"definition_id"`
					RevisionID                    string   `json:"revision_id"`
					Name                          string   `json:"name"`
					Description                   string   `json:"description"`
					SubjectScope                  string   `json:"subject_scope"`
					SourcePath                    string   `json:"source_path"`
					SuppliedArtifactDigest        string   `json:"supplied_artifact_digest"`
					SuppliedContentDigest         string   `json:"supplied_content_digest"`
					ExternalSourceDigest          string   `json:"external_source_digest"`
					ProvenanceDigest              string   `json:"provenance_digest"`
					Dependencies                  []string `json:"dependencies"`
					CompatibleRuntimeCapabilities []string `json:"compatible_runtime_capabilities"`
					Risk                          string   `json:"risk"`
				}{"candidate-import-" + identity, definitionID, revisionID, name, "Reviewed import Candidate", "project", sourcePath, artifactDigest, contentDigest, artifactDigest, artifactDigest, []string{}, []string{}, string(assets.RiskMedium)})
			}
		} else {
			config, found := tuiTemplateConfig(createMode)
			if !found {
				return evolutionAssetsFailedMsg{err: app.ErrInvalidLocalProductAsset}
			}
			definitionID = createMode + "-" + identity
			parameterDigest := tuiSHA256(`{"schema_version":1,"parameters":[]}`)
			permissionDigest := tuiSHA256("loom.template.permission.none.v1")
			scopeDigest := tuiSHA256("loom.template.scope.project.v1")
			_, artifactDigest, contentDigest, readErr := app.CanonicalEvolutionTemplateArtifact(
				config.kind, definitionID, revisionID, sourcePath, config.output,
				parameterDigest, permissionDigest, scopeDigest,
			)
			if readErr != nil {
				return evolutionAssetsFailedMsg{err: app.ErrInvalidLocalProductAsset}
			}
			action = "create_template"
			input, err = json.Marshal(struct {
				AssetKind                     assets.AssetKind      `json:"asset_kind"`
				DefinitionID                  string                `json:"definition_id"`
				RevisionID                    string                `json:"revision_id"`
				Name                          string                `json:"name"`
				Description                   string                `json:"description"`
				SubjectScope                  string                `json:"subject_scope"`
				SourcePath                    string                `json:"source_path"`
				SuppliedArtifactDigest        string                `json:"supplied_artifact_digest"`
				SuppliedContentDigest         string                `json:"supplied_content_digest"`
				TemplateOutput                assets.TemplateOutput `json:"template_output"`
				ParameterSchemaDigest         string                `json:"parameter_schema_digest"`
				PermissionCeilingDigest       string                `json:"permission_ceiling_digest"`
				ScopeCeilingDigest            string                `json:"scope_ceiling_digest"`
				CompatibleRuntimeCapabilities []string              `json:"compatible_runtime_capabilities"`
				Risk                          string                `json:"risk"`
			}{config.kind, definitionID, revisionID, name, "Candidate-only template", "project", sourcePath, artifactDigest, contentDigest, config.output, parameterDigest, permissionDigest, scopeDigest, []string{}, string(assets.RiskMedium)})
		}
		if err != nil {
			return evolutionAssetsFailedMsg{err: err}
		}
		operationID := "asset-" + action + "-" + identity
		result, err := client.EvolutionAssetCommand(ctx, api.EvolutionAssetCommandRequest{
			JourneyID: journeyID, OperationID: operationID, Action: action,
			ExpectedViewVersion: viewVersion,
			ExpectedStreamHeads: []journal.StreamHead{}, Input: input,
		})
		if err != nil {
			return evolutionAssetsFailedMsg{err: err}
		}
		if result.OperationID != operationID || result.Action != action || len(result.EventIDs) == 0 {
			return evolutionAssetsFailedMsg{err: localipc.ErrInvalidProtocol}
		}
		return evolutionAssetCommittedMsg{result: result}
	}
}

type tuiEvolutionTemplateConfig struct {
	kind   assets.AssetKind
	output assets.TemplateOutput
}

func tuiTemplateConfig(mode string) (tuiEvolutionTemplateConfig, bool) {
	values := map[string]tuiEvolutionTemplateConfig{
		"agent_template":             {assets.AssetKindAgentTemplate, assets.TemplateOutputAgentCandidate},
		"team_template":              {assets.AssetKindTeamTemplate, assets.TemplateOutputTeamDraft},
		"work_package_template":      {assets.AssetKindWorkPackageTemplate, assets.TemplateOutputWorkPackageCandidate},
		"recovery_strategy_template": {assets.AssetKindRecoveryStrategyTemplate, assets.TemplateOutputRecoveryStrategyCandidate},
	}
	value, ok := values[mode]
	return value, ok
}

func tuiSHA256(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func (model Model) commitSelectedEvolutionAction(action string) tea.Cmd {
	client, ctx, journeyID := model.assetClient, model.ctx, model.evolutionJourneyID
	snapshot, selected := model.evolutionAssets, model.selected
	return func() tea.Msg {
		if client == nil || selected < 0 || selected >= len(snapshot.Definitions) {
			return evolutionAssetsFailedMsg{err: app.ErrInvalidLocalProductAsset}
		}
		definition := snapshot.Definitions[selected]
		findRevision := func(id string) (assets.SkillRevision, bool) {
			for _, revision := range snapshot.Revisions {
				if revision.DefinitionID == definition.DefinitionID && revision.RevisionID == id {
					return revision, true
				}
			}
			return assets.SkillRevision{}, false
		}
		var input []byte
		var err error
		operationID, operationErr := newTUICorrelationID()
		if operationErr != nil {
			return evolutionAssetsFailedMsg{err: operationErr}
		}
		switch action {
		case "instantiate":
			revision, found := findRevision(definition.LatestRevisionID)
			if !found || revision.AssetKind == assets.AssetKindSkill || revision.Lifecycle == assets.LifecycleArchived {
				return evolutionAssetsFailedMsg{err: assets.ErrDenied}
			}
			parameterDigest := tuiSHA256("[]")
			input, err = json.Marshal(struct {
				AssetKind       assets.AssetKind `json:"asset_kind"`
				DefinitionID    string           `json:"definition_id"`
				RevisionID      string           `json:"revision_id"`
				RevisionDigest  string           `json:"revision_digest"`
				ParameterValues []struct{}       `json:"parameter_values"`
				ParameterDigest string           `json:"parameter_digest"`
			}{revision.AssetKind, revision.DefinitionID, revision.RevisionID, revision.ArtifactDigest, []struct{}{}, parameterDigest})
			action = "instantiate_template"
		case "evaluate":
			var candidate assets.EvolutionCandidate
			found := false
			for _, value := range snapshot.Candidates {
				if value.DefinitionID == definition.DefinitionID && value.Decision == "" {
					candidate, found = value, true
					break
				}
			}
			revision, revisionFound := findRevision(candidate.RevisionID)
			if !found || !revisionFound {
				return evolutionAssetsFailedMsg{err: assets.ErrNotFound}
			}
			baseline := revision
			if active, ok := findRevision(definition.ActiveRevisionID); ok {
				baseline = active
			}
			caseIDs := []string{"artifact_digest", "runtime_compatibility", "security_boundary"}
			fixture, fixtureErr := app.CanonicalEvolutionEvaluationFixture("synthetic", caseIDs)
			if fixtureErr != nil {
				return evolutionAssetsFailedMsg{err: fixtureErr}
			}
			fixtureSum := sha256.Sum256(fixture)
			evaluationID, idErr := newTUICorrelationID()
			if idErr != nil {
				return evolutionAssetsFailedMsg{err: idErr}
			}
			input, err = json.Marshal(struct {
				EvaluationID       string   `json:"evaluation_id"`
				CandidateID        string   `json:"candidate_id"`
				FixtureKind        string   `json:"fixture_kind"`
				FixtureDigest      string   `json:"fixture_digest"`
				BaselineRevisionID string   `json:"baseline_revision_id"`
				BaselineDigest     string   `json:"baseline_digest"`
				CandidateRevision  string   `json:"candidate_revision_id"`
				CandidateDigest    string   `json:"candidate_digest"`
				RequestedCaseIDs   []string `json:"requested_case_ids"`
			}{evaluationID, candidate.CandidateID, "synthetic", hex.EncodeToString(fixtureSum[:]), baseline.RevisionID, baseline.ArtifactDigest, revision.RevisionID, revision.ArtifactDigest, caseIDs})
			action = "record_evaluation"
		case "bind":
			revision, found := findRevision(definition.ActiveRevisionID)
			if !found || revision.Lifecycle != assets.LifecycleActive {
				return evolutionAssetsFailedMsg{err: assets.ErrDenied}
			}
			var subject app.EvolutionAssetBindingSubject
			for _, value := range snapshot.BindingSubjects {
				if value.SubjectKind == "work_package" && value.SubjectID == "work-package.coding" {
					subject = value
					break
				}
			}
			if subject.SubjectID == "" {
				return evolutionAssetsFailedMsg{err: assets.ErrNotFound}
			}
			binding := assets.ExactAssetRevisionBinding{
				AssetKind: revision.AssetKind, DefinitionID: revision.DefinitionID,
				RevisionID: revision.RevisionID, SHA256Digest: revision.ArtifactDigest,
				SourceScope: revision.SourceScope,
			}
			setDigest, digestErr := assets.CanonicalAssetRevisionSetDigest([]assets.ExactAssetRevisionBinding{binding})
			if digestErr != nil {
				return evolutionAssetsFailedMsg{err: digestErr}
			}
			input, err = json.Marshal(struct {
				SubjectKind            string                             `json:"subject_kind"`
				SubjectID              string                             `json:"subject_id"`
				SubjectVersion         int64                              `json:"subject_version"`
				SubjectDigest          string                             `json:"subject_digest"`
				SubjectScope           string                             `json:"subject_scope"`
				SubjectProjectID       string                             `json:"subject_project_id"`
				SubjectGenerationID    string                             `json:"subject_generation_id"`
				SubjectIdentityDigest  string                             `json:"subject_identity_digest"`
				AssetRevisionBindings  []assets.ExactAssetRevisionBinding `json:"asset_revision_bindings"`
				AssetRevisionSetDigest string                             `json:"asset_revision_set_digest"`
			}{subject.SubjectKind, subject.SubjectID, subject.SubjectVersion, subject.SubjectDigest, subject.Scope, subject.ProjectID, subject.GenerationID, subject.SubjectIdentityDigest, []assets.ExactAssetRevisionBinding{binding}, setDigest})
			action = "set_binding"
		case "activate", "reject", "retain":
			var candidate assets.EvolutionCandidate
			found := false
			for _, value := range snapshot.Candidates {
				if value.DefinitionID == definition.DefinitionID && value.Decision == "" {
					candidate, found = value, true
					break
				}
			}
			revision, revisionFound := findRevision(candidate.RevisionID)
			if !found || !revisionFound {
				return evolutionAssetsFailedMsg{err: assets.ErrNotFound}
			}
			if action == "activate" {
				input, err = json.Marshal(struct {
					CandidateID                string           `json:"candidate_id"`
					AssetKind                  assets.AssetKind `json:"asset_kind"`
					DefinitionID               string           `json:"definition_id"`
					RevisionID                 string           `json:"revision_id"`
					RevisionDigest             string           `json:"revision_digest"`
					ExpectedPreviousRevisionID string           `json:"expected_previous_revision_id"`
					EvaluationIDs              []string         `json:"evaluation_ids"`
				}{candidate.CandidateID, candidate.AssetKind, candidate.DefinitionID, candidate.RevisionID, revision.ArtifactDigest, definition.ActiveRevisionID, nonNilTUIStrings(candidate.RequiredEvaluationIDs)})
			} else {
				reason := "user_rejected"
				if action == "retain" {
					reason = "keep_for_later"
				}
				input, err = json.Marshal(struct {
					CandidateID    string           `json:"candidate_id"`
					AssetKind      assets.AssetKind `json:"asset_kind"`
					DefinitionID   string           `json:"definition_id"`
					RevisionID     string           `json:"revision_id"`
					RevisionDigest string           `json:"revision_digest"`
					ReasonCode     string           `json:"reason_code"`
				}{candidate.CandidateID, candidate.AssetKind, candidate.DefinitionID, candidate.RevisionID, revision.ArtifactDigest, reason})
			}
		case "archive_toggle":
			revision, found := findRevision(definition.LatestRevisionID)
			if !found {
				return evolutionAssetsFailedMsg{err: assets.ErrNotFound}
			}
			action = "archive"
			reason := "archive_requested"
			if revision.Lifecycle == assets.LifecycleArchived {
				action, reason = "restore", "restore_requested"
			}
			input, err = json.Marshal(struct {
				AssetKind      assets.AssetKind `json:"asset_kind"`
				DefinitionID   string           `json:"definition_id"`
				RevisionID     string           `json:"revision_id"`
				RevisionDigest string           `json:"revision_digest"`
				ReasonCode     string           `json:"reason_code"`
			}{revision.AssetKind, revision.DefinitionID, revision.RevisionID, revision.ArtifactDigest, reason})
		case "rollback":
			from, found := findRevision(definition.ActiveRevisionID)
			if !found {
				return evolutionAssetsFailedMsg{err: assets.ErrNotFound}
			}
			var target assets.SkillRevision
			for _, revision := range snapshot.Revisions {
				if revision.DefinitionID == definition.DefinitionID && revision.RevisionID != from.RevisionID && revision.Lifecycle != assets.LifecycleArchived {
					target = revision
					break
				}
			}
			if target.RevisionID == "" {
				return evolutionAssetsFailedMsg{err: assets.ErrNotFound}
			}
			input, err = json.Marshal(struct {
				AssetKind      assets.AssetKind `json:"asset_kind"`
				DefinitionID   string           `json:"definition_id"`
				FromRevisionID string           `json:"from_revision_id"`
				FromDigest     string           `json:"from_digest"`
				ToRevisionID   string           `json:"to_revision_id"`
				ToDigest       string           `json:"to_digest"`
				EvaluationIDs  []string         `json:"evaluation_ids"`
				ReasonCode     string           `json:"reason_code"`
			}{target.AssetKind, definition.DefinitionID, from.RevisionID, from.ArtifactDigest, target.RevisionID, target.ArtifactDigest, []string{}, "rollback_requested"})
		default:
			return evolutionAssetsFailedMsg{err: app.ErrInvalidLocalProductAsset}
		}
		if err != nil {
			return evolutionAssetsFailedMsg{err: err}
		}
		result, err := client.EvolutionAssetCommand(ctx, api.EvolutionAssetCommandRequest{JourneyID: journeyID, OperationID: operationID, Action: action, ExpectedViewVersion: snapshot.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: input})
		if err != nil {
			return evolutionAssetsFailedMsg{err: err}
		}
		if result.OperationID != operationID || result.Action != action || len(result.EventIDs) == 0 {
			return evolutionAssetsFailedMsg{err: localipc.ErrInvalidProtocol}
		}
		return evolutionAssetCommittedMsg{result: result}
	}
}

func (model Model) commitEvolutionPromotion() tea.Cmd {
	client, ctx, journeyID := model.assetClient, model.ctx, model.evolutionJourneyID
	snapshot := model.evolutionAssets
	return func() tea.Msg {
		if client == nil || len(snapshot.PromotionSources) == 0 {
			return evolutionAssetsFailedMsg{err: app.ErrInvalidLocalProductAsset}
		}
		source := snapshot.PromotionSources[0]
		if source.RunGeneration < 1 || len(source.RunDigest) != 64 ||
			len(source.EvidenceIDs) == 0 || len(source.EvidenceIDs) != len(source.EvidenceDigests) {
			return evolutionAssetsFailedMsg{err: assets.ErrDenied}
		}
		operationID, err := newTUICorrelationID()
		if err != nil {
			return evolutionAssetsFailedMsg{err: err}
		}
		candidateID, err := newTUICorrelationID()
		if err != nil {
			return evolutionAssetsFailedMsg{err: err}
		}
		summary := "Accepted terminal Run " + source.RunID + " promoted by explicit user action."
		summaryDigest := sha256.Sum256([]byte(summary))
		input, err := json.Marshal(struct {
			CandidateID           string           `json:"candidate_id"`
			AssetKind             assets.AssetKind `json:"asset_kind"`
			DefinitionID          string           `json:"definition_id"`
			RevisionID            string           `json:"revision_id"`
			SourceRunID           string           `json:"source_run_id"`
			SourceRunGeneration   int64            `json:"source_run_generation"`
			SourceRunDigest       string           `json:"source_run_digest"`
			SourceEvidenceIDs     []string         `json:"source_evidence_ids"`
			SourceEvidenceDigests []string         `json:"source_evidence_digests"`
			RedactedSummary       string           `json:"redacted_summary"`
			RedactedSummaryDigest string           `json:"redacted_summary_digest"`
			ScopeDifference       string           `json:"scope_difference"`
			ExpectedBenefit       string           `json:"expected_benefit"`
			Risk                  assets.Risk      `json:"risk"`
		}{
			candidateID, assets.AssetKindSkill, "skill.promoted." + source.RunDigest[:16],
			"revision.1", source.RunID, source.RunGeneration, source.RunDigest,
			nonNilTUIStrings(source.EvidenceIDs), nonNilTUIStrings(source.EvidenceDigests),
			summary, hex.EncodeToString(summaryDigest[:]), "new project-scoped Candidate",
			"reuse accepted terminal behavior", assets.RiskMedium,
		})
		if err != nil {
			return evolutionAssetsFailedMsg{err: err}
		}
		result, err := client.EvolutionAssetCommand(ctx, api.EvolutionAssetCommandRequest{
			JourneyID: journeyID, OperationID: operationID, Action: "promote_run",
			ExpectedViewVersion: snapshot.ViewVersion,
			ExpectedStreamHeads: []journal.StreamHead{}, Input: input,
		})
		if err != nil {
			return evolutionAssetsFailedMsg{err: err}
		}
		if result.OperationID != operationID || result.Action != "promote_run" || len(result.EventIDs) == 0 {
			return evolutionAssetsFailedMsg{err: localipc.ErrInvalidProtocol}
		}
		return evolutionAssetCommittedMsg{result: result}
	}
}

func (model Model) compareSelectedEvolutionRevisions() tea.Cmd {
	client, ctx, journeyID := model.assetClient, model.ctx, model.evolutionJourneyID
	snapshot, selected := model.evolutionAssets, model.selected
	return func() tea.Msg {
		if client == nil || selected < 0 || selected >= len(snapshot.Definitions) {
			return evolutionAssetsFailedMsg{err: app.ErrInvalidLocalProductAsset}
		}
		definition := snapshot.Definitions[selected]
		revisions := make([]assets.SkillRevision, 0)
		for _, revision := range snapshot.Revisions {
			if revision.DefinitionID == definition.DefinitionID {
				revisions = append(revisions, revision)
			}
		}
		if len(revisions) < 2 {
			return evolutionAssetsFailedMsg{err: assets.ErrNotFound}
		}
		diff, err := client.EvolutionAssetDiff(ctx, api.EvolutionAssetDiffRequest{
			JourneyID: journeyID, DefinitionID: definition.DefinitionID,
			LeftRevisionID: revisions[0].RevisionID, LeftDigest: revisions[0].ArtifactDigest,
			RightRevisionID: revisions[1].RevisionID, RightDigest: revisions[1].ArtifactDigest,
		})
		if err != nil {
			return evolutionAssetsFailedMsg{err: err}
		}
		return evolutionAssetDiffLoadedMsg{diff: diff}
	}
}

func shortTUIDigest(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12] + "…"
}

func nonNilTUIStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return append([]string{}, values...)
}

func (model Model) loadSetup() tea.Cmd {
	client := model.setupClient
	ctx := model.ctx
	return func() tea.Msg {
		if client == nil {
			return setupFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		snapshot, err := client.SetupSnapshot(ctx)
		if err != nil {
			return setupFailedMsg{err: err}
		}
		return setupLoadedMsg{snapshot: snapshot}
	}
}

func (model Model) startBlankBuilder() tea.Cmd {
	client := model.setupClient
	ctx := model.ctx
	return func() tea.Msg {
		if client == nil {
			return setupFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		session, err := client.StartBuilder(
			ctx,
			app.BuilderStartCommand{Source: app.BuilderSourceBlank},
		)
		if err != nil {
			return setupFailedMsg{err: err}
		}
		return builderStartedMsg{session: session}
	}
}

func (model Model) loadChatThread() tea.Cmd {
	client := model.chatClient
	ctx := model.ctx
	threadID := model.currentChatThreadID()
	return func() tea.Msg {
		if client == nil {
			return chatThreadFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		thread, err := client.ChatThread(ctx, api.LocalProductChatThreadRequest{ThreadID: threadID})
		if err != nil {
			return chatThreadFailedMsg{err: err}
		}
		return chatThreadLoadedMsg{thread: thread}
	}
}

func (model Model) sendChatMessage(content string) tea.Cmd {
	client := model.chatClient
	ctx := model.ctx
	threadID := model.currentChatThreadID()
	return func() tea.Msg {
		if client == nil {
			return chatMessageFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		thread, err := client.SendChatMessage(ctx, api.LocalProductChatMessageRequest{ThreadID: threadID, Content: content})
		if err != nil {
			return chatMessageFailedMsg{err: err}
		}
		return chatMessageSentMsg{thread: thread}
	}
}

func (model Model) currentChatThreadID() string {
	return model.chatThreadID
}

func newTUIChatThreadID(workspaceIdentity string) string {
	digest := sha256.Sum256([]byte(filepath.Clean(workspaceIdentity)))
	return "thread-" + hex.EncodeToString(digest[:16])
}

func (model Model) startSelectedSetupAsset() tea.Cmd {
	client := model.setupClient
	ctx := model.ctx
	selected := model.selected
	saved := append([]app.SetupSavedTeamPreview{}, model.setup.SavedTeams...)
	templates := append(
		[]app.SetupTeamTemplatePreview{},
		model.setup.Templates...,
	)
	return func() tea.Msg {
		if client == nil {
			return setupFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		command := app.BuilderStartCommand{}
		switch {
		case selected < len(saved):
			team := saved[selected]
			if team.Status != "active" {
				return setupFailedMsg{err: app.ErrBuilderIncompatible}
			}
			command = app.BuilderStartCommand{
				Source:        app.BuilderSourceSavedTeam,
				SourceID:      team.ID,
				SourceVersion: team.Version,
				SourceDigest:  team.DefinitionDigest,
			}
		case selected-len(saved) < len(templates):
			template := templates[selected-len(saved)]
			command = app.BuilderStartCommand{
				Source:        app.BuilderSourceTemplate,
				SourceID:      template.ID,
				SourceVersion: template.Version,
				SourceDigest:  template.Digest,
			}
		default:
			return setupFailedMsg{err: app.ErrInvalidLocalProductSetup}
		}
		session, err := client.StartBuilder(ctx, command)
		if err != nil {
			return setupFailedMsg{err: err}
		}
		return builderStartedMsg{session: session}
	}
}

func (model Model) setSelectedTeamStatus(restoring bool) tea.Cmd {
	client := model.setupClient
	ctx := model.ctx
	team := model.setup.SavedTeams[model.selected]
	return func() tea.Msg {
		if client == nil {
			return setupFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		command := app.TeamStatusCommand{
			DefinitionID: team.ID,
			ExpectedHead: team.StreamHead,
		}
		var (
			updated app.SetupSavedTeamPreview
			err     error
		)
		if restoring {
			updated, err = client.RestoreTeam(ctx, command)
		} else {
			updated, err = client.ArchiveTeam(ctx, command)
		}
		if err != nil {
			return setupFailedMsg{err: err}
		}
		return teamStatusUpdatedMsg{team: updated}
	}
}

func (model Model) updateEntry(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "esc", "ctrl+c":
		clearTUIBytes(model.entry)
		model.entry = nil
		model.entryMode = ""
		return model, nil
	case "backspace", "ctrl+h":
		_, size := utf8.DecodeLastRune(model.entry)
		if size > 0 {
			for index := len(model.entry) - size; index < len(model.entry); index++ {
				model.entry[index] = 0
			}
			model.entry = model.entry[:len(model.entry)-size]
		}
		return model, nil
	case "enter":
		if model.entryMode == entryTaskSearch {
			model.taskFilter = sanitizeCell(string(model.entry), 96)
			clearTUIBytes(model.entry)
			model.entry = nil
			model.entryMode = ""
			model.clampTaskSelection()
			return model, nil
		}
		if model.entryMode == entryEvolutionSearch {
			model.evolutionSearch = sanitizeCell(string(model.entry), 96)
			clearTUIBytes(model.entry)
			model.entry = nil
			model.entryMode = ""
			model.loading = true
			return model, model.loadEvolutionAssets("")
		}
		if model.entryMode == entryMissionObjective {
			objective := strings.TrimSpace(sanitizeCell(string(model.entry), 4096))
			clearTUIBytes(model.entry)
			model.entry = nil
			model.entryMode = ""
			if objective == "" {
				return model, nil
			}
			model.missionObjective = objective
			model.invalidateMissionPreflight()
			return model, nil
		}
		if model.entryMode == entrySideTaskRequest {
			request := strings.TrimSpace(sanitizeCell(string(model.entry), 4096))
			clearTUIBytes(model.entry)
			model.entry = nil
			model.entryMode = ""
			if request == "" {
				return model, nil
			}
			model.sideTaskRequest = request
			model.loading = true
			return model, model.proposeSideTask()
		}
		if model.entryMode == entryEvolutionAsset {
			name := strings.TrimSpace(sanitizeCell(string(model.entry), 128))
			clearTUIBytes(model.entry)
			model.entry = nil
			model.entryMode = ""
			if name == "" {
				return model, nil
			}
			model.loading = true
			return model, model.createEvolutionAsset(name)
		}
		if model.entryMode == entryQueueJobPath {
			path := strings.TrimSpace(sanitizeCell(string(model.entry), 256))
			clearTUIBytes(model.entry)
			model.entry = nil
			model.entryMode = ""
			if path == "" {
				return model, nil
			}
			model.loading = true
			return model, model.createQueueJob(path)
		}
		if model.entryMode == entryPermissionProfile {
			profileID := strings.TrimSpace(sanitizeCell(string(model.entry), 128))
			clearTUIBytes(model.entry)
			model.entry = nil
			model.entryMode = ""
			if profileID == "" {
				return model, nil
			}
			model.loading = true
			return model, model.permissionDefineDefault(profileID)
		}
		if model.entryMode == entryCustomerRuleImport {
			content := strings.TrimSpace(sanitizeCell(string(model.entry), 4096))
			clearTUIBytes(model.entry)
			model.entry = nil
			model.entryMode = ""
			if content == "" {
				return model, nil
			}
			model.loading = true
			return model, model.customerRuleImport(content)
		}
		if model.entryMode == entryPermissionCall {
			command := strings.TrimSpace(sanitizeCell(string(model.entry), 512))
			clearTUIBytes(model.entry)
			model.entry = nil
			model.entryMode = ""
			if command == "" {
				return model, nil
			}
			model.loading = true
			return model, model.permissionValidateCall(command)
		}
		if model.entryMode == entryChatDraft {
			content := strings.TrimSpace(sanitizeCell(string(model.entry), 4096))
			clearTUIBytes(model.entry)
			model.entry = nil
			model.entryMode = ""
			if content == "" {
				return model, nil
			}
			model.loading = true
			return model, model.sendChatMessage(content)
		}
		if model.entryMode == entryFolderPath {
			path := strings.TrimSpace(string(model.entry))
			clearTUIBytes(model.entry)
			model.entry = nil
			model.entryMode = ""
			absolute, err := filepath.Abs(path)
			if err != nil || path == "" {
				model.lastError = "folder_unavailable"
				return model, nil
			}
			info, err := os.Lstat(absolute)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				model.lastError = "folder_unavailable"
				return model, nil
			}
			model.workspaceFolder = sanitizeCell(filepath.Base(absolute), 128)
			model.chatThreadID = newTUIChatThreadID(absolute)
			model.chatThread = api.LocalProductChatThread{}
			model.lastError = ""
			return model, model.loadChatThread()
		}
		if len(model.entry) == 0 {
			return model, nil
		}
		mode := model.entryMode
		value := append([]byte(nil), model.entry...)
		clearTUIBytes(model.entry)
		model.entry = nil
		model.entryMode = ""
		model.loading = true
		switch mode {
		case entryBuilderAnswer:
			defer clearTUIBytes(value)
			return model, model.answerBuilder(string(value))
		case entryEditName:
			defer clearTUIBytes(value)
			return model, model.editBuilder("team_name", string(value))
		case entryEditPurpose:
			defer clearTUIBytes(value)
			return model, model.editBuilder("purpose", string(value))
		case entryCredentialPut:
			return model, model.configureCredential(value)
		case entryCredentialSwap:
			return model, model.replaceCredential(value)
		default:
			clearTUIBytes(value)
			model.loading = false
			return model, nil
		}
	}
	maximum := 2048
	if model.entryMode == entryFolderPath {
		maximum = 1024
	}
	if model.entryMode == entryMissionObjective || model.entryMode == entrySideTaskRequest {
		maximum = 4096
	}
	if model.entryMode == entryCredentialPut ||
		model.entryMode == entryCredentialSwap {
		maximum = 8192
	}
	if message.Type == tea.KeySpace {
		if len(model.entry) < maximum {
			model.entry = append(model.entry, ' ')
		}
		return model, nil
	}
	if message.Type != tea.KeyRunes {
		return model, nil
	}
	for _, character := range message.Runes {
		if unicode.IsControl(character) {
			continue
		}
		var encoded [utf8.UTFMax]byte
		size := utf8.EncodeRune(encoded[:], character)
		if len(model.entry)+size > maximum {
			break
		}
		model.entry = append(model.entry, encoded[:size]...)
	}
	return model, nil
}

func (model Model) answerBuilder(answer string) tea.Cmd {
	client := model.setupClient
	ctx := model.ctx
	session := cloneBuilderSession(model.builder)
	return func() tea.Msg {
		if client == nil {
			return setupFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		updated, err := client.AnswerBuilder(
			ctx,
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
			return setupFailedMsg{err: err}
		}
		return builderUpdatedMsg{session: updated}
	}
}

func (model Model) editBuilder(field, value string) tea.Cmd {
	client := model.setupClient
	ctx := model.ctx
	session := cloneBuilderSession(model.builder)
	return func() tea.Msg {
		if client == nil {
			return setupFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		updated, err := client.EditBuilder(
			ctx,
			app.BuilderEditCommand{
				DraftID:          session.DraftID,
				ExpectedRevision: session.Revision,
				CatalogDigest:    session.CatalogDigest,
				ViewVersion:      session.ViewVersion,
				Field:            field,
				Value:            value,
			},
		)
		if err != nil {
			return setupFailedMsg{err: err}
		}
		return builderUpdatedMsg{session: updated}
	}
}

func (model Model) confirmBuilder() tea.Cmd {
	client := model.setupClient
	ctx := model.ctx
	session := cloneBuilderSession(model.builder)
	return func() tea.Msg {
		if client == nil {
			return builderConfirmFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		definitionID, err := newTUITeamDefinitionID()
		if err != nil {
			return builderConfirmFailedMsg{err: err}
		}
		confirmation, err := client.ConfirmBuilder(
			ctx,
			app.BuilderConfirmCommand{
				DraftID:          session.DraftID,
				ExpectedRevision: session.Revision,
				CatalogDigest:    session.CatalogDigest,
				ViewVersion:      session.ViewVersion,
				BindingDigest:    session.BindingDigest,
				DefinitionID:     definitionID,
				Scope:            "reusable",
				Confirm:          true,
			},
		)
		if err != nil {
			return builderConfirmFailedMsg{err: err}
		}
		return builderConfirmedMsg{confirmation: confirmation}
	}
}

func (model Model) configureCredential(secret []byte) tea.Cmd {
	client := model.setupClient
	ctx := model.ctx
	return func() tea.Msg {
		defer clearTUIBytes(secret)
		if client == nil {
			return setupFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		result, err := client.ConfigureCredential(
			ctx,
			app.CredentialSetupCommand{
				ProviderID: "minimax",
				Secret:     secret,
			},
		)
		if err != nil {
			return setupFailedMsg{err: err}
		}
		return credentialUpdatedMsg{result: result}
	}
}

func (model Model) replaceCredential(secret []byte) tea.Cmd {
	client := model.setupClient
	ctx := model.ctx
	status := model.setup.MiniMax
	return func() tea.Msg {
		defer clearTUIBytes(secret)
		if client == nil {
			return setupFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		result, err := client.ReplaceCredential(
			ctx,
			app.CredentialSetupCommand{
				ProviderID:          "minimax",
				CredentialReference: status.CredentialReference,
				ExpectedRevision:    status.Revision,
				Secret:              secret,
			},
		)
		if err != nil {
			return setupFailedMsg{err: err}
		}
		return credentialUpdatedMsg{result: result}
	}
}

func (model Model) verifyCredential() tea.Cmd {
	return model.credentialWithoutSecret("verify")
}

func (model Model) revokeCredential() tea.Cmd {
	return model.credentialWithoutSecret("revoke")
}

func (model Model) credentialWithoutSecret(action string) tea.Cmd {
	client := model.setupClient
	ctx := model.ctx
	status := model.setup.MiniMax
	return func() tea.Msg {
		if client == nil {
			return setupFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		command := app.CredentialSetupCommand{
			ProviderID:          "minimax",
			CredentialReference: status.CredentialReference,
			ExpectedRevision:    status.Revision,
		}
		var (
			result app.CredentialSetupResult
			err    error
		)
		if action == "verify" {
			result, err = client.VerifyCredential(ctx, command)
		} else {
			result, err = client.RevokeCredential(ctx, command)
		}
		if err != nil {
			return setupFailedMsg{err: err}
		}
		return credentialUpdatedMsg{result: result}
	}
}

func newTUITeamDefinitionID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", errors.New("Team identifier unavailable")
	}
	return "team-" + hex.EncodeToString(value[:]), nil
}

func newTUICorrelationID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", errors.New("Request identifier unavailable")
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" +
		encoded[12:16] + "-" + encoded[16:20] + "-" +
		encoded[20:32], nil
}

func newTUIJourneyID() (string, error) {
	if supplied := os.Getenv("LOOM_JOURNEY_ID"); validTUIJourneyID(supplied) {
		return supplied, nil
	}
	return newTUICorrelationID()
}

func validTUIJourneyID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' ||
		value[18] != '-' || value[23] != '-' || value[14] != '4' ||
		!strings.ContainsRune("89ab", rune(value[19])) {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func clearTUIBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func (model *Model) toggleComparedRun(runID string) {
	for index, selected := range model.compareRuns {
		if selected == runID {
			model.compareRuns = append(
				model.compareRuns[:index],
				model.compareRuns[index+1:]...,
			)
			return
		}
	}
	if len(model.compareRuns) == 2 {
		model.compareRuns = append(model.compareRuns[:0], model.compareRuns[1])
	}
	model.compareRuns = append(model.compareRuns, runID)
}

func (model Model) isComparedRun(runID string) bool {
	for _, selected := range model.compareRuns {
		if selected == runID {
			return true
		}
	}
	return false
}

func (model Model) renderCompare() string {
	lines := []string{"Compare"}
	if len(model.compareRuns) < 2 {
		lines = append(
			lines,
			"Select two bounded Run summaries from History with enter.",
		)
	}
	for position, runID := range model.compareRuns {
		for _, run := range model.snapshot.Runs {
			if run.RunID != runID {
				continue
			}
			evidenceCount := 0
			for _, record := range model.snapshot.Evidence {
				if record.WorkItemID == run.WorkItemID {
					evidenceCount++
				}
			}
			label := "Previous"
			if position == 1 {
				label = "Current"
			}
			lines = append(lines, fmt.Sprintf(
				"%s · %s · %s",
				label,
				humanizeStatus(run.Phase),
				humanizeStatus(run.TerminalStatus),
			))
			if run.TerminalReason != "" {
				lines = append(lines, "   Reason · "+humanizeStatus(run.TerminalReason))
			}
			lines = append(lines, fmt.Sprintf(
				"   Runtime · %s · Attempt generation %d",
				model.runtimeDisplayName(run.RuntimeInstanceID),
				run.ClaimGeneration,
			))
			lines = append(lines, fmt.Sprintf(
				"   Evidence · %d",
				evidenceCount,
			))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func (model *Model) clampSelection() {
	maximum := 0
	switch model.Screen() {
	case ScreenHome:
		maximum = len(model.snapshot.Missions)
	case ScreenBoard:
		model.clampTaskSelection()
		return
	case ScreenTeams:
		maximum = len(model.snapshot.Teams)
	case ScreenRuns, ScreenCompare:
		maximum = len(model.snapshot.Runs)
	case ScreenEvidence:
		maximum = len(model.snapshot.Evidence)
	case ScreenRuntimes:
		maximum = len(model.snapshot.Runtimes)
	case ScreenAttention:
		maximum = len(model.snapshot.Attention)
	case ScreenTeamBuilder:
		if model.builder.DraftID != "" {
			maximum = len(model.builder.Question.Options)
		} else {
			maximum = model.setupSelectableCount()
		}
	case ScreenAssets:
		maximum = len(model.evolutionAssets.Definitions)
	}
	if maximum == 0 {
		model.selected = 0
	} else if model.selected >= maximum {
		model.selected = maximum - 1
	}
}

func humanizeStatus(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "_", " "))
	if value == "" {
		return "Ready"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func (model Model) missionDisplayTitle(
	mission api.LocalProductMissionSummary,
) string {
	candidate := sanitizeCell(mission.Title, 48)
	missionID := sanitizeCell(mission.MissionID, 96)
	teamID := sanitizeCell(mission.TeamInstanceID, 96)
	if candidate != "" && candidate != missionID && candidate != teamID {
		return candidate
	}
	for _, team := range model.snapshot.Teams {
		if team.TeamInstanceID != mission.TeamInstanceID {
			continue
		}
		name := sanitizeCell(team.DisplayName, 48)
		if name != "" && name != sanitizeCell(team.TeamInstanceID, 96) {
			return name
		}
	}
	return "Mission"
}

func (model Model) missionNodeDisplayTitle(
	mission api.LocalProductMissionSummary,
	nodeID string,
) string {
	for _, node := range mission.Topology {
		if node.LogicalNodeID != nodeID {
			continue
		}
		title := sanitizeCell(node.Title, 48)
		if title != "" && title != sanitizeCell(node.LogicalNodeID, 96) {
			return title
		}
		role := sanitizeCell(node.Role, 32)
		if role != "" && role != sanitizeCell(node.LogicalNodeID, 96) {
			return humanizeStatus(role)
		}
	}
	for _, pulse := range mission.TeamPulse {
		if pulse.NodeID == nodeID && sanitizeCell(pulse.Role, 32) != "" {
			return humanizeStatus(sanitizeCell(pulse.Role, 32))
		}
	}
	if strings.TrimSpace(nodeID) == "" {
		return "No current step"
	}
	return "Mission step"
}

func (model Model) missionDependencyDisplayTitles(
	mission api.LocalProductMissionSummary,
	nodeIDs []string,
) string {
	titles := make([]string, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		titles = append(titles, model.missionNodeDisplayTitle(mission, nodeID))
	}
	return sanitizeCell(strings.Join(titles, ", "), 48)
}

func (model Model) runtimeDisplayName(runtimeID string) string {
	for _, runtime := range model.snapshot.Runtimes {
		if runtime.RuntimeInstanceID != runtimeID {
			continue
		}
		name := sanitizeCell(runtime.DisplayName, 48)
		if name != "" && name != sanitizeCell(runtime.RuntimeInstanceID, 96) {
			return name
		}
	}
	return "Runtime unavailable"
}

func setupRuntimeDisplayName(runtime app.SetupRuntimePreview) string {
	name := sanitizeCell(runtime.DisplayName, 40)
	if name != "" && name != sanitizeCell(runtime.RuntimeInstanceID, 96) {
		return name
	}
	return "Runtime unavailable"
}

func (model Model) teamDisplayName(team api.LocalProductTeamSummary) string {
	name := sanitizeCell(team.DisplayName, 48)
	if name != "" && name != sanitizeCell(team.TeamInstanceID, 96) {
		return name
	}
	return "Confirmed Team"
}

func (model Model) renderTeamNames() string {
	lines := make([]string, 0, len(model.snapshot.Teams))
	for _, team := range model.snapshot.Teams {
		lines = append(lines, "• "+model.teamDisplayName(team))
	}
	return strings.Join(lines, "\n")
}

func (model Model) setupSelectableCount() int {
	return len(model.setup.SavedTeams) + len(model.setup.Templates)
}

func (model Model) filteredMissions() []api.LocalProductMissionSummary {
	filter := strings.ToLower(strings.TrimSpace(model.taskFilter))
	rows := make(
		[]api.LocalProductMissionSummary,
		0,
		len(model.snapshot.Missions),
	)
	for index, mission := range model.snapshot.Missions {
		selection := index + 1
		name := strings.ToLower(sanitizeCell(mission.Title, 96))
		state := strings.ToLower(sanitizeCell(mission.Status, 48))
		lane := strings.ToLower(sanitizeCell(string(mission.Lane), 24))
		if filter == "" || selection == model.selected ||
			strings.Contains(name, filter) ||
			strings.Contains(state, filter) ||
			strings.Contains(lane, filter) {
			rows = append(rows, mission)
		}
	}
	return rows
}

func (model Model) selectedMission() (
	api.LocalProductMissionSummary,
	bool,
) {
	if model.selected < 1 || model.selected > len(model.snapshot.Missions) {
		return api.LocalProductMissionSummary{}, false
	}
	return model.snapshot.Missions[model.selected-1], true
}

func (model Model) currentMissionRecord() (
	api.LocalProductMissionSummary,
	bool,
) {
	for _, mission := range model.snapshot.Missions {
		if mission.MissionID == model.currentMission {
			return mission, true
		}
	}
	return api.LocalProductMissionSummary{}, false
}

func (model Model) currentMissionAttention() (api.AttentionItem, bool) {
	mission, ok := model.currentMissionRecord()
	if !ok {
		return api.AttentionItem{}, false
	}
	for _, attention := range model.snapshot.Attention {
		if attention.TeamInstanceID == mission.TeamInstanceID &&
			attention.ApprovalRequestID != "" {
			return attention, true
		}
	}
	return api.AttentionItem{}, false
}

func (model Model) preparedMissionDecision(
	missionID string,
) (app.MissionDecisionCommand, bool) {
	for _, decision := range model.snapshot.PreparedDecisions {
		if decision.MissionID == missionID {
			return decision, true
		}
	}
	return app.MissionDecisionCommand{}, false
}

func (model *Model) taskSelections() []int {
	selections := []int{0}
	for index := range model.filteredMissions() {
		selections = append(selections, index+1)
	}
	return selections
}

func (model *Model) moveTaskSelection(delta int) {
	selections := model.taskSelections()
	position := 0
	for index, selection := range selections {
		if selection == model.selected {
			position = index
			break
		}
	}
	position += delta
	if position < 0 {
		position = 0
	}
	if position >= len(selections) {
		position = len(selections) - 1
	}
	model.selected = selections[position]
}

func (model *Model) clampTaskSelection() {
	for _, selection := range model.taskSelections() {
		if model.selected == selection {
			return
		}
	}
	model.selected = 0
}

func renderBuilderPreflight(preview app.BuilderPreview) []string {
	lines := []string{"Review before saving"}
	for _, role := range preview.Roles {
		lines = append(lines, fmt.Sprintf(
			"%s · %s · %s · %s · %s · %s",
			humanizeStatus(role.Kind),
			sanitizeCell(role.DisplayName, 32),
			providerForAuthMode(role.AuthMode),
			sanitizeCell(role.ModelID, 48),
			humanizeStatus(role.AuthMode),
			compatibilityLabel(role),
		))
		if len(role.PermissionIDs) > 0 {
			lines = append(
				lines,
				"Permissions · "+
					humanizeList(role.PermissionIDs),
			)
		}
	}
	if len(preview.Permissions) > 0 {
		lines = append(
			lines,
			"Team permissions · "+humanizeList(preview.Permissions),
		)
	}
	compatibility := "Compatible"
	if len(preview.CompatibilityGaps) > 0 {
		compatibility = "Needs review"
	}
	lines = append(
		lines,
		"Compatibility · "+compatibility,
		fmt.Sprintf(
			"Maximum budget · %d credits · %s",
			preview.MaximumBudgetCredits,
			sanitizeCell(preview.EstimatedMaximumCost, 48),
		),
	)
	return lines
}

func renderBuilderRoleChoices(
	setup app.SetupSnapshot,
	preview app.BuilderPreview,
) []string {
	lines := make([]string, 0)
	for _, kind := range []string{"main", "subagent"} {
		label := "Main role choices"
		if kind == "subagent" {
			label = "SubAgent role choices"
		}
		added := false
		for _, option := range setup.RoleOptions {
			if option.Kind != kind {
				continue
			}
			if !added {
				lines = append(lines, label)
				added = true
			}
			marker := " "
			if roleOptionCurrent(option, preview) {
				marker = "✓"
			}
			lines = append(lines, fmt.Sprintf(
				"%s %s",
				marker,
				roleOptionSummary(option, setup, preview),
			))
		}
	}
	return lines
}

func (model Model) roleOptionSummary(id, fallback string) string {
	for _, option := range model.setup.RoleOptions {
		if option.ID == id {
			return roleOptionSummary(
				option,
				model.setup,
				model.builder.Preview,
			)
		}
	}
	return sanitizeCell(fallback, 64)
}

func roleOptionSummary(
	option app.SetupRoleOptionPreview,
	setup app.SetupSnapshot,
	preview app.BuilderPreview,
) string {
	responsibility := sanitizeCell(option.Responsibility, 40)
	model := "Configured model"
	provider := ""
	auth := ""
	current := false
	for _, role := range preview.Roles {
		if roleOptionMatches(option, role) {
			current = true
			provider = providerForAuthMode(role.AuthMode)
			model = sanitizeCell(role.ModelID, 40)
			auth = humanizeStatus(role.AuthMode)
			break
		}
	}
	for _, runtime := range setup.Runtimes {
		if runtime.RuntimeInstanceID != option.RuntimeInstanceID {
			continue
		}
		if model == "Configured model" {
			if runtime.ModelID != "" {
				model = sanitizeCell(runtime.ModelID, 40)
			} else if len(runtime.ModelIDs) > 0 {
				model = sanitizeCell(runtime.ModelIDs[0], 40)
			}
		}
		break
	}
	if !current {
		return fmt.Sprintf(
			"%s · %s · Select to review provider and sign-in",
			responsibility,
			model,
		)
	}
	return fmt.Sprintf(
		"%s · %s · %s · %s",
		responsibility,
		provider,
		model,
		auth,
	)
}

func roleOptionCurrent(
	option app.SetupRoleOptionPreview,
	preview app.BuilderPreview,
) bool {
	for _, role := range preview.Roles {
		if roleOptionMatches(option, role) {
			return true
		}
	}
	return false
}

func roleOptionMatches(
	option app.SetupRoleOptionPreview,
	role app.BuilderRolePreview,
) bool {
	return role.Kind == option.Kind &&
		role.AgentDefinitionID == option.AgentDefinitionID &&
		role.RuntimeProfileID == option.RuntimeProfileID &&
		role.Runtime.RuntimeInstanceID == option.RuntimeInstanceID
}

func (model Model) nextRoleOption(kind string) (string, string, bool) {
	options := make([]app.SetupRoleOptionPreview, 0)
	current := -1
	for _, option := range model.setup.RoleOptions {
		if option.Kind != kind {
			continue
		}
		if roleOptionCurrent(option, model.builder.Preview) {
			current = len(options)
		}
		options = append(options, option)
	}
	if len(options) == 0 {
		return "", "", false
	}
	next := 0
	if current >= 0 {
		next = (current + 1) % len(options)
	}
	field := "main_role"
	if kind == "subagent" {
		field = "subagent_role"
	}
	return field, options[next].ID, true
}

func providerForAuthMode(authMode string) string {
	switch authMode {
	case "native_auth":
		return "Codex"
	case "brokered":
		return "MiniMax"
	default:
		return "Local provider"
	}
}

func compatibilityLabel(role app.BuilderRolePreview) string {
	if role.Compatible {
		return "Compatible"
	}
	return humanizeStatus(role.CompatibilityReason)
}

func humanizeList(values []string) string {
	human := make([]string, 0, len(values))
	for _, value := range values {
		human = append(human, humanizeStatus(strings.ReplaceAll(value, ".", "_")))
	}
	return strings.Join(human, ", ")
}

func (model *Model) switchScreen(next int) {
	if next < 0 || next >= len(screens) {
		return
	}
	if model.screenIndex >= 0 && model.screenIndex < len(model.selections) {
		model.selections[model.screenIndex] = model.selected
	}
	model.screenIndex = next
	model.selected = model.selections[next]
	model.clampSelection()
}

func indexOfScreen(screen Screen) int {
	for index, candidate := range screens {
		if candidate == screen {
			return index
		}
	}
	return 0
}

func sanitizeCell(value string, maximum int) string {
	if maximum < 1 {
		return ""
	}
	value = stripTerminalSequences(value)
	value = strings.ToValidUTF8(value, "�")
	var builder strings.Builder
	spacePending := false
	for _, character := range value {
		if character == '\x1b' || isBidiControl(character) ||
			unicode.IsControl(character) {
			spacePending = builder.Len() > 0
			continue
		}
		if unicode.IsSpace(character) {
			spacePending = builder.Len() > 0
			continue
		}
		if spacePending {
			builder.WriteByte(' ')
			spacePending = false
		}
		builder.WriteRune(character)
		if utf8.RuneCountInString(builder.String()) >= maximum {
			break
		}
	}
	return strings.TrimSpace(builder.String())
}

func stripTerminalSequences(value string) string {
	data := []byte(value)
	result := make([]byte, 0, len(data))
	for index := 0; index < len(data); {
		if data[index] != 0x1b {
			result = append(result, data[index])
			index++
			continue
		}
		index++
		if index >= len(data) {
			break
		}
		switch data[index] {
		case '[':
			index++
			for index < len(data) {
				final := data[index]
				index++
				if final >= 0x40 && final <= 0x7e {
					break
				}
			}
		case ']':
			index++
			for index < len(data) {
				if data[index] == 0x07 {
					index++
					break
				}
				if data[index] == 0x1b &&
					index+1 < len(data) &&
					data[index+1] == '\\' {
					index += 2
					break
				}
				index++
			}
		default:
			index++
		}
	}
	return string(result)
}

func isBidiControl(character rune) bool {
	return character >= '\u202a' && character <= '\u202e' ||
		character >= '\u2066' && character <= '\u2069'
}

func safeClientState(err error) string {
	if err == nil {
		return ""
	}
	var remote *localipc.RemoteError
	if errors.As(err, &remote) {
		return sanitizeCell(remote.Code, 32)
	}
	if errors.Is(err, localipc.ErrLocalProductUnavailable) {
		return "offline"
	}
	if errors.Is(err, localipc.ErrInvalidProtocol) {
		return "fatal_protocol_mismatch"
	}
	if errors.Is(err, localipc.ErrProtocolTimeout) ||
		errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "state_unavailable"
}

func isRemoteConflict(err error) bool {
	var remote *localipc.RemoteError
	return errors.As(err, &remote) && remote.Code == "conflict"
}

func isRemoteNotFound(err error) bool {
	var remote *localipc.RemoteError
	return errors.As(err, &remote) && remote.Code == "not_found"
}

func cloneChatThread(thread api.LocalProductChatThread) api.LocalProductChatThread {
	copied := thread
	copied.Messages = append([]api.LocalProductChatMessage(nil), thread.Messages...)
	return copied
}

func cloneSnapshot(snapshot api.LocalProductSnapshot) api.LocalProductSnapshot {
	snapshot.Runtimes = append(
		[]api.LocalProductRuntimeSummary(nil),
		snapshot.Runtimes...,
	)
	for index := range snapshot.Runtimes {
		snapshot.Runtimes[index].ModelIDs = append(
			[]string(nil),
			snapshot.Runtimes[index].ModelIDs...,
		)
		snapshot.Runtimes[index].ObservedCapabilities = append(
			[]string(nil),
			snapshot.Runtimes[index].ObservedCapabilities...,
		)
	}
	snapshot.Teams = append(
		[]api.LocalProductTeamSummary(nil),
		snapshot.Teams...,
	)
	snapshot.Missions = append(
		[]api.LocalProductMissionSummary(nil),
		snapshot.Missions...,
	)
	for index := range snapshot.Missions {
		snapshot.Missions[index].TeamPulse = append(
			[]api.LocalProductMissionPulse(nil),
			snapshot.Missions[index].TeamPulse...,
		)
		snapshot.Missions[index].Topology = append(
			[]api.LocalProductMissionNode(nil),
			snapshot.Missions[index].Topology...,
		)
		for nodeIndex := range snapshot.Missions[index].Topology {
			snapshot.Missions[index].Topology[nodeIndex].DependsOn = append(
				[]string(nil),
				snapshot.Missions[index].Topology[nodeIndex].DependsOn...,
			)
		}
	}
	snapshot.Runs = append(
		[]api.LocalProductRunSummary(nil),
		snapshot.Runs...,
	)
	snapshot.Evidence = append(
		[]api.LocalProductEvidenceSummary(nil),
		snapshot.Evidence...,
	)
	snapshot.Attention = append(
		[]api.AttentionItem(nil),
		snapshot.Attention...,
	)
	return snapshot
}

func cloneTimeline(
	page api.LocalProductTimelinePage,
) api.LocalProductTimelinePage {
	page.Records = append(
		[]api.LocalProductTimelineRecord(nil),
		page.Records...,
	)
	page.Board.Nodes = append([]api.NodeBoardRow(nil), page.Board.Nodes...)
	page.Attention = append([]api.AttentionItem(nil), page.Attention...)
	if page.Gap != nil {
		copied := *page.Gap
		page.Gap = &copied
	}
	return page
}

func cloneSetupSnapshot(snapshot app.SetupSnapshot) app.SetupSnapshot {
	snapshot.Runtimes = append(
		[]app.SetupRuntimePreview{},
		snapshot.Runtimes...,
	)
	for index := range snapshot.Runtimes {
		snapshot.Runtimes[index].ModelIDs = append(
			[]string{},
			snapshot.Runtimes[index].ModelIDs...,
		)
		snapshot.Runtimes[index].ObservedCapabilities = append(
			[]string{},
			snapshot.Runtimes[index].ObservedCapabilities...,
		)
	}
	snapshot.SavedTeams = append(
		[]app.SetupSavedTeamPreview{},
		snapshot.SavedTeams...,
	)
	snapshot.Templates = append(
		[]app.SetupTeamTemplatePreview{},
		snapshot.Templates...,
	)
	snapshot.RoleOptions = append(
		[]app.SetupRoleOptionPreview{},
		snapshot.RoleOptions...,
	)
	snapshot.Skills = append(
		[]app.SetupSkillRevision{},
		snapshot.Skills...,
	)
	snapshot.Permissions = append([]string{}, snapshot.Permissions...)
	snapshot.Resources = append(
		[]app.SetupResourcePointer{},
		snapshot.Resources...,
	)
	return snapshot
}

func cloneBuilderSession(
	session app.BuilderSessionView,
) app.BuilderSessionView {
	session.Question.Options = append(
		[]app.BuilderQuestionOption{},
		session.Question.Options...,
	)
	session.Preview.Roles = append(
		[]app.BuilderRolePreview{},
		session.Preview.Roles...,
	)
	session.Preview.Permissions = append(
		[]string{},
		session.Preview.Permissions...,
	)
	session.Preview.Resources = append(
		[]string{},
		session.Preview.Resources...,
	)
	session.Preview.CompatibilityGaps = append(
		[]string{},
		session.Preview.CompatibilityGaps...,
	)
	return session
}

func emptyOrLines(lines []string, count int) string {
	if count == 0 {
		return lines[0] + "\nNo records in this bounded page.\n"
	}
	return strings.Join(lines, "\n") + "\n"
}

func clipView(value string, width, height int) string {
	lines := strings.Split(value, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for index, line := range lines {
		if ansi.StringWidth(line) > width {
			lines[index] = ansi.Truncate(line, width, "")
		}
	}
	return strings.Join(lines, "\n")
}
