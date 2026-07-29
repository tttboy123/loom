package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/localipc"
)

type Screen string

const (
	ScreenHome      Screen = "Home"
	ScreenRuntimes  Screen = "Runtimes"
	ScreenTeams     Screen = "Teams"
	ScreenRuns      Screen = "Runs / History"
	ScreenEvidence  Screen = "Evidence"
	ScreenCompare   Screen = "Compare"
	ScreenAttention Screen = "Attention"
	ScreenTimeline  Screen = "Team Timeline"
)

var screens = []Screen{
	ScreenHome,
	ScreenRuntimes,
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

type Model struct {
	client ReadClient
	ctx    context.Context
	cancel context.CancelFunc

	screenIndex int
	width       int
	height      int
	selected    int
	help        bool
	loading     bool
	offline     bool
	lastError   string
	snapshot    api.LocalProductSnapshot
	timeline    api.LocalProductTimelinePage
	compareRuns []string
	currentTeam string
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
	return Model{
		client:  client,
		ctx:     ctx,
		cancel:  cancel,
		width:   80,
		height:  24,
		loading: true,
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
	case tea.KeyMsg:
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
			}
			return model, nil
		case "enter":
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
		"\nPhase 2A W1 · read-only · no Team, Provider, or Run mutation\n",
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
	}
	if maximum == 0 {
		model.selected = 0
	} else if model.selected >= maximum {
		model.selected = maximum - 1
	}
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
