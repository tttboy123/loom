package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/mode"
	"loom-pi-rebuild/internal/projection"

	_ "modernc.org/sqlite"
)

const (
	exitSuccess          = 0
	exitInvalidInput     = 2
	exitStateUnavailable = 3
	exitStreamGap        = 4
)

type runDeps struct {
	route    func(mode.Intent) mode.Decision
	status   func(context.Context, string) (projection.Snapshot, error)
	timeline func(context.Context, timelineInput) (api.TimelinePage, error)
}

func productionDeps() runDeps {
	return runDeps{
		route:    mode.Route,
		status:   readProductionStatus,
		timeline: readProductionTimeline,
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, deps runDeps) int {
	if deps.route == nil {
		deps.route = mode.Route
	}
	if deps.status == nil {
		deps.status = readProductionStatus
	}
	if deps.timeline == nil {
		deps.timeline = readProductionTimeline
	}
	if len(args) == 0 {
		return invalidInput(stderr, "missing command")
	}

	switch args[0] {
	case "route":
		return runRoute(args[1:], stdout, stderr, deps)
	case "status":
		return runStatus(ctx, args[1:], stdout, stderr, deps)
	case "timeline":
		return runTimeline(ctx, args[1:], stdout, stderr, deps)
	default:
		return invalidInput(stderr, "unknown command")
	}
}

type timelineInput struct {
	StatePath      string
	TeamInstanceID string
	Cursor         string
	Limit          int
}

func runTimeline(
	ctx context.Context,
	args []string,
	stdout,
	stderr io.Writer,
	deps runDeps,
) int {
	fs := flag.NewFlagSet("timeline", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	state := fs.String("state", "", "")
	team := fs.String("team", "", "")
	cursor := fs.String("cursor", "", "")
	limit := fs.Int("limit", journal.MaxReadPageEvents, "")
	if err := fs.Parse(args); err != nil {
		return invalidInput(stderr, err.Error())
	}
	if fs.NArg() != 0 {
		return invalidInput(stderr, "unexpected positional arguments")
	}
	if *state == "" {
		return invalidInput(stderr, "missing state")
	}
	if *team == "" {
		return invalidInput(stderr, "missing team")
	}
	if *limit < 1 || *limit > journal.MaxReadPageEvents {
		return invalidInput(stderr, "invalid limit")
	}
	page, err := deps.timeline(ctx, timelineInput{
		StatePath:      *state,
		TeamInstanceID: *team,
		Cursor:         *cursor,
		Limit:          *limit,
	})
	if err != nil {
		var gapErr *api.TimelineGapError
		if errors.As(err, &gapErr) {
			if writeJSON(stdout, stderr, timelineOutputFromPage(gapErr.Page())) != exitSuccess {
				return exitStateUnavailable
			}
			return exitStreamGap
		}
		if errors.Is(err, api.ErrInvalidTimelineRequest) ||
			errors.Is(err, api.ErrTeamTimelineNotFound) {
			return invalidInput(stderr, "invalid timeline request")
		}
		return stateUnavailable(stderr)
	}
	return writeJSON(stdout, stderr, timelineOutputFromPage(page))
}

func readProductionTimeline(
	ctx context.Context,
	input timelineInput,
) (api.TimelinePage, error) {
	db, err := openReadOnlyState(ctx, input.StatePath)
	if err != nil {
		return api.TimelinePage{}, err
	}
	readModel := projection.New(db)
	stream, err := api.NewTeamExecutionStream(api.TeamExecutionStreamConfig{
		TeamInstanceID: input.TeamInstanceID,
		Journal:        journal.NewStore(db),
		Projection:     readModel,
		Now: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		_ = db.Close()
		return api.TimelinePage{}, err
	}
	page, readErr := stream.ReadPage(ctx, input.Cursor, input.Limit)
	closeErr := db.Close()
	if readErr != nil {
		return page, readErr
	}
	if closeErr != nil {
		return api.TimelinePage{}, closeErr
	}
	return page, nil
}

func runRoute(args []string, stdout, stderr io.Writer, deps runDeps) int {
	fs := flag.NewFlagSet("route", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	trigger := fs.String("trigger", "", "")
	target := fs.String("target", "", "")
	text := fs.String("text", "", "")
	if err := fs.Parse(args); err != nil {
		return invalidInput(stderr, err.Error())
	}
	if fs.NArg() != 0 {
		return invalidInput(stderr, "unexpected positional arguments")
	}
	if *trigger == "" {
		return invalidInput(stderr, "missing trigger")
	}

	decision := deps.route(mode.Intent{
		Trigger:  mode.Trigger(*trigger),
		TargetID: *target,
		Text:     *text,
	})
	return writeJSON(stdout, stderr, routeOutput{
		Command: "route",
		Mode:    string(decision.Mode),
	})
}

func runStatus(ctx context.Context, args []string, stdout, stderr io.Writer, deps runDeps) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	state := fs.String("state", "", "")
	if err := fs.Parse(args); err != nil {
		return invalidInput(stderr, err.Error())
	}
	if fs.NArg() != 0 {
		return invalidInput(stderr, "unexpected positional arguments")
	}
	if *state == "" {
		return invalidInput(stderr, "missing state")
	}

	snapshot, err := deps.status(ctx, *state)
	if err != nil {
		return stateUnavailable(stderr)
	}
	return writeJSON(stdout, stderr, statusOutputFromSnapshot(snapshot))
}

func readProductionStatus(ctx context.Context, statePath string) (projection.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return projection.Snapshot{}, err
	}
	db, err := openReadOnlyState(ctx, statePath)
	if err != nil {
		return projection.Snapshot{}, err
	}

	proj := projection.New(db)
	if err := proj.Rebuild(ctx); err != nil {
		_ = db.Close()
		return projection.Snapshot{}, err
	}
	snapshot := proj.Snapshot()
	if err := db.Close(); err != nil {
		return projection.Snapshot{}, err
	}
	return snapshot, nil
}

func openReadOnlyState(ctx context.Context, statePath string) (*sql.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	absoluteStatePath, err := filepath.Abs(statePath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absoluteStatePath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("state is not a file")
	}

	values := url.Values{}
	values.Add("mode", "ro")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "query_only(1)")
	uri := url.URL{Scheme: "file", Path: absoluteStatePath}
	uri.RawQuery = values.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

type routeOutput struct {
	Command string `json:"command"`
	Mode    string `json:"mode"`
}

type statusOutput struct {
	Command   string           `json:"command"`
	Modes     []modeStatus     `json:"modes"`
	WorkItems []workItemStatus `json:"work_items"`
	Evidence  []evidenceStatus `json:"evidence"`
}

type modeStatus struct {
	Stream string `json:"stream"`
	Mode   string `json:"mode"`
}

type workItemStatus struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type evidenceStatus struct {
	ID         string `json:"id"`
	WorkItemID string `json:"work_item_id"`
	Digest     string `json:"digest"`
}

type timelineOutput struct {
	SchemaVersion  int                  `json:"schema_version"`
	Command        string               `json:"command"`
	TeamInstanceID string               `json:"team_instance_id"`
	ViewVersion    string               `json:"view_version"`
	NextCursor     string               `json:"next_cursor"`
	HasMore        bool                 `json:"has_more"`
	Gap            *api.StreamGap       `json:"gap"`
	Records        []api.DeliveryRecord `json:"records"`
	Board          api.TeamBoard        `json:"board"`
	Attention      []api.AttentionItem  `json:"attention"`
}

func timelineOutputFromPage(page api.TimelinePage) timelineOutput {
	var gap *api.StreamGap
	if current, ok := page.Gap(); ok {
		gap = &current
	}
	return timelineOutput{
		SchemaVersion:  1,
		Command:        "timeline",
		TeamInstanceID: page.TeamInstanceID(),
		ViewVersion:    page.ViewVersion(),
		NextCursor:     page.NextCursor(),
		HasMore:        page.HasMore(),
		Gap:            gap,
		Records:        page.Records(),
		Board:          page.Board(),
		Attention:      page.Attention(),
	}
}

func statusOutputFromSnapshot(snapshot projection.Snapshot) statusOutput {
	out := statusOutput{Command: "status"}
	for stream, selectedMode := range snapshot.Modes {
		out.Modes = append(out.Modes, modeStatus{Stream: stream, Mode: selectedMode})
	}
	for _, workItem := range snapshot.WorkItems {
		out.WorkItems = append(out.WorkItems, workItemStatus{
			ID:     workItem.ID,
			Title:  workItem.Title,
			Status: workItem.Status,
		})
	}
	for _, evidence := range snapshot.Evidence {
		out.Evidence = append(out.Evidence, evidenceStatus{
			ID:         evidence.ID,
			WorkItemID: evidence.WorkItemID,
			Digest:     evidence.Digest,
		})
	}

	sort.Slice(out.Modes, func(i, j int) bool {
		return out.Modes[i].Stream < out.Modes[j].Stream
	})
	sort.Slice(out.WorkItems, func(i, j int) bool {
		return out.WorkItems[i].ID < out.WorkItems[j].ID
	})
	sort.Slice(out.Evidence, func(i, j int) bool {
		return out.Evidence[i].ID < out.Evidence[j].ID
	})
	if out.Modes == nil {
		out.Modes = []modeStatus{}
	}
	if out.WorkItems == nil {
		out.WorkItems = []workItemStatus{}
	}
	if out.Evidence == nil {
		out.Evidence = []evidenceStatus{}
	}
	return out
}

func writeJSON(stdout, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	if err := encoder.Encode(value); err != nil {
		_, _ = fmt.Fprintln(stderr, "state unavailable: unavailable")
		return exitStateUnavailable
	}
	return exitSuccess
}

func invalidInput(stderr io.Writer, message string) int {
	_, _ = fmt.Fprintf(stderr, "invalid input: %s\n", message)
	return exitInvalidInput
}

func stateUnavailable(stderr io.Writer) int {
	_, _ = fmt.Fprintln(stderr, "state unavailable: unavailable")
	return exitStateUnavailable
}
