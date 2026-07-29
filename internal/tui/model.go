package tui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/localipc"
)

type Screen string

const (
	ScreenHome        Screen = "Home"
	ScreenRuntimes    Screen = "Runtimes"
	ScreenTeamBuilder Screen = "Team Builder"
	ScreenTeams       Screen = "Teams"
	ScreenRuns        Screen = "Runs / History"
	ScreenEvidence    Screen = "Evidence"
	ScreenCompare     Screen = "Compare"
	ScreenAttention   Screen = "Attention"
	ScreenTimeline    Screen = "Team Timeline"
)

var screens = []Screen{
	ScreenHome,
	ScreenRuntimes,
	ScreenTeamBuilder,
	ScreenTeams,
	ScreenRuns,
	ScreenEvidence,
	ScreenCompare,
	ScreenAttention,
	ScreenTimeline,
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

type builderStartedMsg struct {
	session app.BuilderSessionView
}

type builderUpdatedMsg struct {
	session app.BuilderSessionView
}

type builderConfirmedMsg struct {
	confirmation app.BuilderConfirmation
}

type teamStatusUpdatedMsg struct {
	team app.SetupSavedTeamPreview
}

type credentialUpdatedMsg struct {
	result app.CredentialSetupResult
}

const (
	entryBuilderAnswer  = "builder_answer"
	entryEditName       = "edit_name"
	entryEditPurpose    = "edit_purpose"
	entryCredentialPut  = "credential_put"
	entryCredentialSwap = "credential_swap"
)

type Model struct {
	client      ReadClient
	setupClient SetupClient
	ctx         context.Context
	cancel      context.CancelFunc

	screenIndex  int
	width        int
	height       int
	selected     int
	help         bool
	loading      bool
	offline      bool
	lastError    string
	snapshot     api.LocalProductSnapshot
	timeline     api.LocalProductTimelinePage
	setup        app.SetupSnapshot
	builder      app.BuilderSessionView
	confirmation app.BuilderConfirmation
	credential   app.CredentialSetupResult
	entryMode    string
	entry        []byte
	compareRuns  []string
	currentTeam  string
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
	return Model{
		client:      client,
		setupClient: setupClient,
		ctx:         ctx,
		cancel:      cancel,
		width:       80,
		height:      24,
		loading:     true,
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
		model.snapshot = cloneSnapshot(message.snapshot)
		if len(model.snapshot.Teams) == 0 {
			model.currentTeam = ""
			model.timeline = api.LocalProductTimelinePage{}
		}
		model.clampSelection()
		return model, nil
	case snapshotFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
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
		model.confirmation = message.confirmation
		model.builder = app.BuilderSessionView{}
		return model, model.loadSetup()
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
	case setupFailedMsg:
		model.loading = false
		model.offline = errors.Is(
			message.err,
			localipc.ErrLocalProductUnavailable,
		)
		model.lastError = safeClientState(message.err)
		return model, nil
	case tea.KeyMsg:
		if model.entryMode != "" {
			return model.updateEntry(message)
		}
		switch message.String() {
		case "q", "ctrl+c":
			model.cancel()
			return model, tea.Quit
		case "tab", "right":
			model.screenIndex = (model.screenIndex + 1) % len(screens)
			model.selected = 0
			return model, nil
		case "shift+tab", "left":
			model.screenIndex--
			if model.screenIndex < 0 {
				model.screenIndex = len(screens) - 1
			}
			model.selected = 0
			return model, nil
		case "down", "j":
			model.selected++
			model.clampSelection()
			return model, nil
		case "up", "k":
			if model.selected > 0 {
				model.selected--
			}
			return model, nil
		case "r":
			model.loading = true
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
			if model.Screen() == ScreenTimeline {
				model.screenIndex = indexOfScreen(ScreenTeams)
			} else if model.Screen() == ScreenTeamBuilder {
				clearTUIBytes(model.entry)
				model.entry = nil
				model.entryMode = ""
				model.builder = app.BuilderSessionView{}
			}
			return model, nil
		case "enter":
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
				model.screenIndex = indexOfScreen(ScreenTimeline)
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
			if model.Screen() == ScreenTeamBuilder &&
				model.setupClient != nil &&
				model.builder.DraftID == "" {
				model.loading = true
				return model, model.startBlankBuilder()
			}
		case "c":
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.CanConfirm {
				model.loading = true
				return model, model.confirmBuilder()
			}
		case "a":
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.DraftID == "" &&
				model.selected < len(model.setup.SavedTeams) &&
				model.setup.SavedTeams[model.selected].Status == "active" {
				model.loading = true
				return model, model.setSelectedTeamStatus(false)
			}
		case "u":
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.DraftID == "" &&
				model.selected < len(model.setup.SavedTeams) &&
				model.setup.SavedTeams[model.selected].Status == "archived" {
				model.loading = true
				return model, model.setSelectedTeamStatus(true)
			}
		case "e":
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.CanConfirm {
				model.entryMode = entryEditName
				model.entry = []byte{}
				return model, nil
			}
		case "p":
			if model.Screen() == ScreenTeamBuilder &&
				model.builder.CanConfirm {
				model.entryMode = entryEditPurpose
				model.entry = []byte{}
				return model, nil
			}
		case "g":
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
		case "v":
			if model.Screen() == ScreenTeamBuilder &&
				model.setup.MiniMax.CredentialReference != "" &&
				model.setup.MiniMax.Revision > 0 {
				model.loading = true
				return model, model.verifyCredential()
			}
		case "x":
			if model.Screen() == ScreenTeamBuilder &&
				model.setup.MiniMax.CredentialReference != "" &&
				model.setup.MiniMax.Revision > 0 {
				model.loading = true
				return model, model.revokeCredential()
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
	builder.WriteString("Loom · ")
	builder.WriteString(string(model.Screen()))
	builder.WriteByte('\n')
	builder.WriteString(strings.Repeat("─", min(model.width, 72)))
	builder.WriteByte('\n')
	switch {
	case model.loading:
		builder.WriteString("Loading the local read model…\n")
	case model.offline:
		builder.WriteString("Daemon offline · read view unavailable\n")
	case model.lastError != "":
		builder.WriteString("State unavailable · ")
		builder.WriteString(model.lastError)
		builder.WriteByte('\n')
	default:
		builder.WriteString(model.screenBody())
	}
	if model.snapshot.Stale {
		builder.WriteString("\nstale · ")
		builder.WriteString(sanitizeCell(model.snapshot.Reason, 80))
		builder.WriteString(" · last view ")
		builder.WriteString(sanitizeCell(model.snapshot.ViewVersion, 64))
		builder.WriteByte('\n')
	}
	if model.snapshot.Partial {
		builder.WriteString(
			"\npartial · more records are available on another bounded page\n",
		)
	}
	if model.help {
		builder.WriteString(
			"\nKeys: tab/shift+tab screens · j/k select · enter open · r refresh · q quit\n",
		)
	}
	builder.WriteString(
		"\nPhase 2A · Team Draft confirmation never starts a Run\n",
	)
	return clipView(builder.String(), model.width, model.height)
}

func (model Model) Screen() Screen {
	if model.screenIndex < 0 || model.screenIndex >= len(screens) {
		return ScreenHome
	}
	return screens[model.screenIndex]
}

func (model Model) screenBody() string {
	switch model.Screen() {
	case ScreenHome:
		return fmt.Sprintf(
			"Home\nView %s\n%d runtimes · %d teams · %d runs · %d evidence\n%s",
			sanitizeCell(model.snapshot.ViewVersion, 64),
			len(model.snapshot.Runtimes),
			len(model.snapshot.Teams),
			len(model.snapshot.Runs),
			len(model.snapshot.Evidence),
			renderTeamNames(model.snapshot.Teams),
		)
	case ScreenRuntimes:
		lines := make([]string, 0, len(model.snapshot.Runtimes)+1)
		lines = append(lines, "Discovered local Runtimes")
		for _, runtime := range model.snapshot.Runtimes {
			lines = append(lines, fmt.Sprintf(
				"• %s · %s · %s",
				sanitizeCell(runtime.DisplayName, 48),
				sanitizeCell(runtime.Status, 24),
				sanitizeCell(runtime.ExecutableVersion, 24),
			))
		}
		return emptyOrLines(lines, len(model.snapshot.Runtimes))
	case ScreenTeamBuilder:
		return model.renderTeamBuilder()
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
				sanitizeCell(team.DisplayName, 48),
				sanitizeCell(team.SourceKind, 32),
			))
		}
		return emptyOrLines(lines, len(model.snapshot.Teams))
	case ScreenRuns:
		lines := []string{"Run history"}
		for index, run := range model.snapshot.Runs {
			marker := " "
			if index == model.selected {
				marker = "›"
			}
			selected := ""
			if model.isComparedRun(run.RunID) {
				selected = " · compare"
			}
			lines = append(lines, fmt.Sprintf(
				"%s %s · %s · %s%s",
				marker,
				sanitizeCell(run.RunID, 48),
				sanitizeCell(run.Phase, 24),
				sanitizeCell(run.TerminalStatus, 24),
				selected,
			))
		}
		return emptyOrLines(lines, len(model.snapshot.Runs))
	case ScreenEvidence:
		lines := []string{"Accepted Evidence references"}
		for _, evidence := range model.snapshot.Evidence {
			lines = append(lines, fmt.Sprintf(
				"• %s · %s",
				sanitizeCell(evidence.EvidenceID, 48),
				sanitizeCell(evidence.Digest, 64),
			))
		}
		return emptyOrLines(lines, len(model.snapshot.Evidence))
	case ScreenCompare:
		return model.renderCompare()
	case ScreenAttention:
		lines := []string{"Attention inbox"}
		for _, item := range model.snapshot.Attention {
			lines = append(lines, fmt.Sprintf(
				"• %s · %s",
				sanitizeCell(item.Kind, 32),
				sanitizeCell(item.ActionRequired, 64),
			))
		}
		return emptyOrLines(lines, len(model.snapshot.Attention))
	case ScreenTimeline:
		lines := []string{"Authoritative Team timeline"}
		if model.currentTeam == "" &&
			model.timeline.TeamInstanceID == "" &&
			model.timeline.Gap == nil &&
			model.timeline.Board.Status == "" &&
			len(model.timeline.Board.Nodes) == 0 &&
			len(model.timeline.Attention) == 0 &&
			len(model.timeline.Records) == 0 {
			return lines[0] +
				"\nSelect a Team from Teams to open its authoritative timeline.\n"
		}
		if model.timeline.Gap != nil {
			recovery := "reopen the Team timeline"
			if model.timeline.Gap.Recoverable {
				recovery = "press r to reconnect from the authoritative view"
			}
			lines = append(lines, fmt.Sprintf(
				"stream gap · %s · %s",
				sanitizeCell(model.timeline.Gap.Reason, 32),
				recovery,
			))
		}
		if model.timeline.Board.Status != "" {
			lines = append(lines, fmt.Sprintf(
				"Board · %s · %d nodes",
				sanitizeCell(model.timeline.Board.Status, 24),
				len(model.timeline.Board.Nodes),
			))
		}
		for _, item := range model.timeline.Attention {
			lines = append(lines, fmt.Sprintf(
				"Attention · %s · %s",
				sanitizeCell(item.Kind, 32),
				sanitizeCell(item.ActionRequired, 48),
			))
		}
		for _, record := range model.timeline.Records {
			lines = append(lines, fmt.Sprintf(
				"• %s · %s · %s",
				sanitizeCell(record.Kind, 32),
				sanitizeCell(record.Payload.Status, 24),
				sanitizeCell(record.Payload.WarningCode, 32),
			))
		}
		count := len(model.timeline.Records) +
			len(model.timeline.Attention)
		if model.timeline.Gap != nil || model.timeline.Board.Status != "" {
			count++
		}
		return emptyOrLines(lines, count)
	default:
		return ""
	}
}

func (model Model) renderTeamBuilder() string {
	lines := []string{"Team Builder"}
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
			sanitizeCell(runtime.DisplayName, 40),
			sanitizeCell(runtime.Status, 20),
			sanitizeCell(runtime.ExecutableVersion, 20),
		))
	}
	if model.builder.DraftID != "" {
		lines = append(lines, fmt.Sprintf(
			"Candidate · revision %d",
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
					sanitizeCell(option.Label, 64),
				))
			}
			if len(model.builder.Question.Options) == 0 {
				lines = append(lines, "Press enter to type one bounded answer")
			}
		} else if model.builder.CanConfirm {
			lines = append(
				lines,
				"Ready for explicit confirmation · no execution will start",
				"c confirm · e edit name · p edit purpose · esc cancel",
			)
		}
	} else {
		for index, team := range model.setup.SavedTeams {
			marker := " "
			if index == model.selected {
				marker = "›"
			}
			action := "enter open Candidate"
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
				"%s Template · %s · enter open Candidate",
				marker,
				sanitizeCell(template.Name, 32),
			))
		}
		lines = append(lines, "n start a blank Candidate Team Draft")
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
			"Saved TeamDefinition · active · no Run created",
		)
	}
	return strings.Join(lines, "\n") + "\n"
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
	if message.Type != tea.KeyRunes {
		return model, nil
	}
	maximum := 2048
	if model.entryMode == entryCredentialPut ||
		model.entryMode == entryCredentialSwap {
		maximum = 8192
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
			return setupFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		definitionID, err := newTUITeamDefinitionID()
		if err != nil {
			return setupFailedMsg{err: err}
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
			return setupFailedMsg{err: err}
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
			evidence := make([]string, 0)
			for _, record := range model.snapshot.Evidence {
				if record.WorkItemID == run.WorkItemID {
					evidence = append(
						evidence,
						sanitizeCell(record.EvidenceID, 32)+":"+
							sanitizeCell(record.Digest, 16),
					)
				}
			}
			evidenceText := "none"
			if len(evidence) > 0 {
				evidenceText = strings.Join(evidence, ",")
			}
			lines = append(lines, fmt.Sprintf(
				"%d. %s · %s · %s/%s",
				position+1,
				sanitizeCell(run.RunID, 32),
				sanitizeCell(run.Phase, 16),
				sanitizeCell(run.TerminalStatus, 16),
				sanitizeCell(run.TerminalReason, 24),
			))
			lines = append(lines, fmt.Sprintf(
				"   runtime=%s · generation=%d",
				sanitizeCell(run.RuntimeInstanceID, 24),
				run.ClaimGeneration,
			))
			lines = append(lines, fmt.Sprintf(
				"   evidence=%s",
				evidenceText,
			))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func (model *Model) clampSelection() {
	maximum := 0
	switch model.Screen() {
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
	}
	if maximum == 0 {
		model.selected = 0
	} else if model.selected >= maximum {
		model.selected = maximum - 1
	}
}

func (model Model) setupSelectableCount() int {
	return len(model.setup.SavedTeams) + len(model.setup.Templates)
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

func renderTeamNames(teams []api.LocalProductTeamSummary) string {
	lines := make([]string, 0, len(teams))
	for _, team := range teams {
		lines = append(lines, "• "+sanitizeCell(team.DisplayName, 48))
	}
	return strings.Join(lines, "\n")
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
		runes := []rune(line)
		if len(runes) > width {
			lines[index] = string(runes[:width])
		}
	}
	return strings.Join(lines, "\n")
}
