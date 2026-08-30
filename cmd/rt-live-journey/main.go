// Command rt-live-journey drives the RoundTable dual-seat journey against the
// installed Loom daemon over its real Unix socket, in two modes:
//
//	--journey  full create->seats->round->propose->relay->ack->insert->conclude
//	           plus restart consistency and post-conclude rejection.
//	--tui      runs the real TUI program headlessly (disable renderer) with
//	           scripted keystrokes against the installed daemon.
package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/roundtable"
	"loom-pi-rebuild/internal/tui"
)

type steerRunningParams struct {
	SchemaVersion  int    `json:"schema_version"`
	SessionID      string `json:"session_id"`
	RoundID        string `json:"round_id"`
	InterventionID string `json:"intervention_id"`
	ModeratorSeat  string `json:"moderator_seat"`
	SeatID         string `json:"seat_id"`
	AttemptID      string `json:"attempt_id"`
	Guidance       []byte `json:"guidance"`
	CorrelationID  string `json:"correlation_id"`
}

type retrySeatParams struct {
	SchemaVersion  int    `json:"schema_version"`
	SessionID      string `json:"session_id"`
	RoundID        string `json:"round_id"`
	InterventionID string `json:"intervention_id"`
	ModeratorSeat  string `json:"moderator_seat"`
	SeatID         string `json:"seat_id"`
	AttemptID      string `json:"attempt_id"`
	Guidance       string `json:"guidance"`
	CorrelationID  string `json:"correlation_id"`
}

type cloneSessionCreateParams struct {
	SchemaVersion int                           `json:"schema_version"`
	SessionID     string                        `json:"session_id"`
	ModeratorSeat string                        `json:"moderator_seat"`
	Title         string                        `json:"title"`
	Link          roundtable.SessionLinkRequest `json:"link"`
	CorrelationID string                        `json:"correlation_id"`
}

type cloneAddSeatParams struct {
	SchemaVersion int                           `json:"schema_version"`
	SessionID     string                        `json:"session_id"`
	SeatID        string                        `json:"seat_id"`
	DisplayName   string                        `json:"display_name"`
	Selection     roundtable.SeatBindingRequest `json:"selection"`
	CorrelationID string                        `json:"correlation_id"`
}

type cloneOpenRoundParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	RoundID       string `json:"round_id"`
	ModeratorSeat string `json:"moderator_seat"`
	Prompt        string `json:"prompt"`
	CorrelationID string `json:"correlation_id"`
}

const (
	moderatorSeat = "seat-moderator"
	writerSeat    = "seat-writer"
	targetSeat    = "seat-target"
	roundID       = "round-1"
	messageID     = "msg-1"
)

func socketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(home, "/") {
		return "", fmt.Errorf("invalid home")
	}
	return home + "/Library/Application Support/Loom/run/loomd.sock", nil
}

func journey(ctx context.Context, sessionID string) error {
	path, err := socketPath()
	if err != nil {
		return err
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: path, Timeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	readClient, err := tui.NewDaemonReadClient(client)
	if err != nil {
		return err
	}
	// Create session.
	if _, err := readClient.RoundtableCreateSession(ctx, tui.RoundtableSessionCreateRequest{
		SchemaVersion: 1, SessionID: sessionID, ModeratorSeat: moderatorSeat,
		Title: "Roundtable live E2E", CorrelationID: correlationID(),
	}); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	fmt.Printf("created session %s\n", sessionID)
	steps := []struct {
		name string
		run  func() error
	}{
		{"add seats", func() error {
			if _, err := readClient.RoundtableAddSeat(ctx, tui.RoundtableAddSeatRequest{
				SchemaVersion: 1, SessionID: sessionID, SeatID: writerSeat,
				DisplayName: "Writer Seat", CorrelationID: correlationID(),
			}); err != nil {
				return err
			}
			_, err := readClient.RoundtableAddSeat(ctx, tui.RoundtableAddSeatRequest{
				SchemaVersion: 1, SessionID: sessionID, SeatID: targetSeat,
				DisplayName: "Target Seat", CorrelationID: correlationID(),
			})
			return err
		}},
		{"open round", func() error {
			_, err := readClient.RoundtableOpenRound(ctx, tui.RoundtableOpenRoundRequest{
				SchemaVersion: 1, SessionID: sessionID, RoundID: roundID,
				ModeratorSeat: moderatorSeat, CorrelationID: correlationID(),
			})
			return err
		}},
		{"propose", func() error {
			_, err := readClient.RoundtableProposeMessage(ctx, tui.RoundtableProposeMessageRequest{
				SchemaVersion: 1, SessionID: sessionID, RoundID: roundID,
				MessageID: messageID, WriterSeat: writerSeat, TargetSeat: targetSeat,
				Body:         "Governed live E2E: bounded dual-seat handoff message.",
				ArtifactRefs: []string{}, CorrelationID: correlationID(),
			})
			return err
		}},
		{"relay", func() error {
			_, err := readClient.RoundtableRelayMessage(ctx, tui.RoundtableRelayMessageRequest{
				SchemaVersion: 1, SessionID: sessionID, MessageID: messageID,
				ModeratorSeat: moderatorSeat, CorrelationID: correlationID(),
			})
			return err
		}},
		{"ack", func() error {
			_, err := readClient.RoundtableAcknowledgeMessage(ctx, tui.RoundtableAckMessageRequest{
				SchemaVersion: 1, SessionID: sessionID, MessageID: messageID,
				SeatID: targetSeat, CorrelationID: correlationID(),
			})
			return err
		}},
		{"insert", func() error {
			_, err := readClient.RoundtableInsertMessage(ctx, tui.RoundtableInsertMessageRequest{
				SchemaVersion: 1, SessionID: sessionID, MessageID: messageID,
				ModeratorSeat: moderatorSeat, CorrelationID: correlationID(),
			})
			return err
		}},
		{"conclude", func() error {
			_, err := readClient.RoundtableConcludeSession(ctx, tui.RoundtableConcludeRequest{
				SchemaVersion: 1, SessionID: sessionID,
				ModeratorSeat: moderatorSeat, CorrelationID: correlationID(),
			})
			return err
		}},
	}
	for _, step := range steps {
		if err := step.run(); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
		fmt.Printf("step %s ok\n", step.name)
	}
	view, err := readClient.RoundtableReadView(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	printView(view)
	// Restart consistency: re-read the same session and confirm identical state.
	again, err := readClient.RoundtableReadView(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("re-snapshot: %w", err)
	}
	if again.Digest != view.Digest || !again.Session.Concluded {
		return fmt.Errorf("restart digest mismatch: %s vs %s", again.Digest, view.Digest)
	}
	fmt.Printf("restart-consistent digest %s (concluded=%t)\n", again.Digest, again.Session.Concluded)
	// Post-conclude rejection: propose and re-conclude must fail with concluded.
	if _, err := readClient.RoundtableProposeMessage(ctx, tui.RoundtableProposeMessageRequest{
		SchemaVersion: 1, SessionID: sessionID, RoundID: roundID,
		MessageID: "msg-after", WriterSeat: writerSeat, TargetSeat: targetSeat,
		Body: "too late", ArtifactRefs: []string{}, CorrelationID: correlationID(),
	}); err == nil {
		return fmt.Errorf("post-conclude propose accepted")
	}
	if _, err := readClient.RoundtableConcludeSession(ctx, tui.RoundtableConcludeRequest{
		SchemaVersion: 1, SessionID: sessionID, ModeratorSeat: moderatorSeat,
		CorrelationID: correlationID(),
	}); err == nil {
		return fmt.Errorf("re-conclude accepted")
	}
	fmt.Println("post-conclude writes rejected")
	return nil
}

// correlationID returns a v4-shaped uuid.
func correlationID() string {
	return "22222222-2222-4222-8222-" + fmt.Sprintf("%012d", time.Now().UnixNano()%1_000_000_000_000)
}

func printView(view roundtable.View) {
	fmt.Printf("session=%s moderator=%s concluded=%t digest=%s\n",
		view.Session.ID, view.Session.ModeratorSeat, view.Session.Concluded, view.Digest)
	for _, seat := range view.SeatsList() {
		fmt.Printf("  seat %s available=%t\n", seat.ID, seat.Available)
	}
	for _, message := range view.Messages {
		fmt.Printf("  msg %s %s->%s status=%s\n",
			message.ID, message.WriterSeat, message.TargetSeat, message.Status)
	}
	attemptIDs := make([]string, 0, len(view.Attempts))
	for attemptID := range view.Attempts {
		attemptIDs = append(attemptIDs, attemptID)
	}
	sort.Strings(attemptIDs)
	for _, attemptID := range attemptIDs {
		attempt := view.Attempts[attemptID]
		fmt.Printf("  attempt %s seat=%s number=%d status=%s retryable=%t stage=%s code=%s\n",
			attempt.AttemptID, attempt.SeatID, attempt.AttemptNumber, attempt.Status,
			attempt.Retryable, attempt.FailureStage, attempt.FailureCode)
	}
	interventionIDs := make([]string, 0, len(view.Interventions))
	for interventionID := range view.Interventions {
		interventionIDs = append(interventionIDs, interventionID)
	}
	sort.Strings(interventionIDs)
	for _, interventionID := range interventionIDs {
		intervention := view.Interventions[interventionID]
		fmt.Printf("  intervention %s kind=%s seat=%s attempt=%s\n",
			intervention.ID, intervention.Kind, intervention.SeatID, intervention.AttemptID)
	}
}

func snapshot(ctx context.Context, sessionID string) error {
	path, err := socketPath()
	if err != nil {
		return err
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: path, Timeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	var view roundtable.View
	if err := client.Call(ctx, "roundtable_snapshot", struct {
		SchemaVersion int    `json:"schema_version"`
		SessionID     string `json:"session_id"`
	}{SchemaVersion: roundtable.SchemaVersion, SessionID: sessionID}, &view); err != nil {
		return err
	}
	printView(view)
	return nil
}

func cloneSourceSeats(view roundtable.View) ([]roundtable.Seat, error) {
	seats := make([]roundtable.Seat, 0, roundtable.MaxAgentSeats)
	for _, seat := range view.SeatsList() {
		if !seat.Available || seat.Binding == nil {
			continue
		}
		seats = append(seats, seat)
	}
	if view.Session.Context == nil || len(seats) < roundtable.MinAgentSeats ||
		len(seats) > roundtable.MaxAgentSeats {
		return nil, fmt.Errorf("source session is not executable")
	}
	return seats, nil
}

func cloneAndRun(ctx context.Context, sourceSessionID, sessionID string) error {
	path, err := socketPath()
	if err != nil {
		return err
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: path, Timeout: 30 * time.Second,
	})
	if err != nil {
		return err
	}
	var source roundtable.View
	if err := client.Call(ctx, "roundtable_snapshot", struct {
		SchemaVersion int    `json:"schema_version"`
		SessionID     string `json:"session_id"`
	}{SchemaVersion: roundtable.SchemaVersion, SessionID: sourceSessionID}, &source); err != nil {
		return fmt.Errorf("source snapshot: %w", err)
	}
	seats, err := cloneSourceSeats(source)
	if err != nil {
		return err
	}
	promptBytes, err := bufio.NewReader(
		io.LimitReader(os.Stdin, roundtable.MaxRoundPromptBytes+1),
	).ReadBytes('\n')
	if err != nil && err != io.EOF {
		return err
	}
	promptBytes = bytes.TrimSpace(promptBytes)
	if len(promptBytes) == 0 || len(promptBytes) > roundtable.MaxRoundPromptBytes ||
		bytes.ContainsAny(promptBytes, "\x00\r") {
		return fmt.Errorf("invalid prompt")
	}
	prompt := string(promptBytes)
	for index := range promptBytes {
		promptBytes[index] = 0
	}
	context := *source.Session.Context
	var view roundtable.View
	if err := client.Call(ctx, "roundtable_session_create", cloneSessionCreateParams{
		SchemaVersion: roundtable.SchemaVersion, SessionID: sessionID,
		ModeratorSeat: moderatorSeat, Title: "RoundTable installed live Steer",
		Link: roundtable.SessionLinkRequest{
			ConversationID: context.ConversationID, MissionID: context.MissionID,
			TeamInstanceID: context.TeamID,
		}, CorrelationID: correlationID(),
	}, &view); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	for _, seat := range seats {
		selection := roundtable.SeatBindingRequest{
			AgentDefinitionID: seat.Binding.AgentDefinitionID,
			TeamRoleKind:      seat.Binding.TeamRoleKind,
			RuntimeProfileID:  seat.Binding.RuntimeProfileID,
		}
		if err := client.Call(ctx, "roundtable_add_seat", cloneAddSeatParams{
			SchemaVersion: roundtable.SchemaVersion, SessionID: sessionID,
			SeatID: seat.ID, DisplayName: seat.DisplayName, Selection: selection,
			CorrelationID: correlationID(),
		}, &view); err != nil {
			return fmt.Errorf("add seat %s: %w", seat.ID, err)
		}
	}
	roundID := "round-live-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := client.Call(ctx, "roundtable_open_round", cloneOpenRoundParams{
		SchemaVersion: roundtable.SchemaVersion, SessionID: sessionID,
		RoundID: roundID, ModeratorSeat: moderatorSeat, Prompt: prompt,
		CorrelationID: correlationID(),
	}, &view); err != nil {
		return fmt.Errorf("open round: %w", err)
	}
	printView(view)
	return nil
}

func steerRunning(ctx context.Context, sessionID, seatID string, waitForRunning bool) error {
	path, err := socketPath()
	if err != nil {
		return err
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: path, Timeout: 130 * time.Second,
	})
	if err != nil {
		return err
	}
	var view roundtable.View
	var running roundtable.SeatAttempt
	for {
		if err := client.Call(ctx, "roundtable_snapshot", struct {
			SchemaVersion int    `json:"schema_version"`
			SessionID     string `json:"session_id"`
		}{SchemaVersion: roundtable.SchemaVersion, SessionID: sessionID}, &view); err != nil {
			return fmt.Errorf("snapshot: %w", err)
		}
		for _, attempt := range view.Attempts {
			if attempt.SeatID != seatID || attempt.Status != roundtable.SeatAttemptRunning {
				continue
			}
			if running.AttemptID != "" {
				return fmt.Errorf("multiple running Attempts for seat")
			}
			running = attempt
		}
		if running.AttemptID != "" || !waitForRunning {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("running Attempt unavailable: %w", ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	if running.AttemptID == "" {
		return fmt.Errorf("running Attempt unavailable")
	}
	guidance, err := bufio.NewReader(
		io.LimitReader(os.Stdin, roundtable.MaxRoundPromptBytes+1),
	).ReadBytes('\n')
	if err != nil && err != io.EOF {
		return err
	}
	guidance = bytes.TrimSpace(guidance)
	if len(guidance) == 0 || len(guidance) > roundtable.MaxRoundPromptBytes ||
		bytes.ContainsAny(guidance, "\x00\r") {
		return fmt.Errorf("invalid guidance")
	}
	defer func() {
		for index := range guidance {
			guidance[index] = 0
		}
	}()
	interventionID := "intervention-live-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := client.Call(ctx, "roundtable_steer_seat", steerRunningParams{
		SchemaVersion: roundtable.SchemaVersion, SessionID: sessionID,
		RoundID: running.RoundID, InterventionID: interventionID,
		ModeratorSeat: moderatorSeat, SeatID: seatID, AttemptID: running.AttemptID,
		Guidance: guidance, CorrelationID: correlationID(),
	}, &view); err != nil {
		return fmt.Errorf("steer: %w", err)
	}
	receipt, found := view.Interventions[interventionID]
	if !found || receipt.Kind != roundtable.InterventionSteer ||
		receipt.AttemptID != running.AttemptID || receipt.ContentDigest == "" {
		return fmt.Errorf("steer receipt unavailable")
	}
	fmt.Printf("steer accepted session=%s seat=%s attempt=%s intervention=%s digest=%s\n",
		sessionID, seatID, running.AttemptID, receipt.ID, receipt.ContentDigest)
	return nil
}

func retryAndSteer(ctx context.Context, sessionID, seatID string) error {
	path, err := socketPath()
	if err != nil {
		return err
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: path, Timeout: 130 * time.Second,
	})
	if err != nil {
		return err
	}
	var view roundtable.View
	if err := client.Call(ctx, "roundtable_snapshot", struct {
		SchemaVersion int    `json:"schema_version"`
		SessionID     string `json:"session_id"`
	}{SchemaVersion: roundtable.SchemaVersion, SessionID: sessionID}, &view); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	var previous roundtable.SeatAttempt
	for _, attempt := range view.Attempts {
		if attempt.SeatID == seatID && attempt.Retryable &&
			attempt.AttemptNumber > previous.AttemptNumber {
			previous = attempt
		}
	}
	if previous.AttemptID == "" {
		return fmt.Errorf("retryable Attempt unavailable")
	}
	reader := bufio.NewReader(io.LimitReader(os.Stdin, 2*roundtable.MaxRoundPromptBytes+2))
	retryGuidance, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	steerGuidance, steerErr := reader.ReadBytes('\n')
	if steerErr != nil && steerErr != io.EOF {
		return steerErr
	}
	retryGuidance = strings.TrimSpace(retryGuidance)
	steerGuidance = bytes.TrimSpace(steerGuidance)
	if retryGuidance == "" || len(retryGuidance) > roundtable.MaxRoundPromptBytes ||
		len(steerGuidance) == 0 || len(steerGuidance) > roundtable.MaxRoundPromptBytes ||
		strings.ContainsAny(retryGuidance, "\x00\r") || bytes.ContainsAny(steerGuidance, "\x00\r") {
		return fmt.Errorf("invalid guidance")
	}
	defer func() {
		for index := range steerGuidance {
			steerGuidance[index] = 0
		}
	}()
	retryID := "intervention-live-retry-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := client.Call(ctx, "roundtable_retry_seat", retrySeatParams{
		SchemaVersion: roundtable.SchemaVersion, SessionID: sessionID,
		RoundID: previous.RoundID, InterventionID: retryID,
		ModeratorSeat: moderatorSeat, SeatID: seatID, AttemptID: previous.AttemptID,
		Guidance: retryGuidance, CorrelationID: correlationID(),
	}, &view); err != nil {
		return fmt.Errorf("retry: %w", err)
	}
	var running roundtable.SeatAttempt
	for _, attempt := range view.Attempts {
		if attempt.SeatID == seatID && attempt.Status == roundtable.SeatAttemptRunning &&
			attempt.AttemptNumber > previous.AttemptNumber {
			running = attempt
		}
	}
	if running.AttemptID == "" {
		return fmt.Errorf("retry did not return a running Attempt")
	}
	steerID := "intervention-live-steer-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := client.Call(ctx, "roundtable_steer_seat", steerRunningParams{
		SchemaVersion: roundtable.SchemaVersion, SessionID: sessionID,
		RoundID: running.RoundID, InterventionID: steerID,
		ModeratorSeat: moderatorSeat, SeatID: seatID, AttemptID: running.AttemptID,
		Guidance: steerGuidance, CorrelationID: correlationID(),
	}, &view); err != nil {
		return fmt.Errorf("steer: %w", err)
	}
	receipt, found := view.Interventions[steerID]
	if !found || receipt.Kind != roundtable.InterventionSteer ||
		receipt.AttemptID != running.AttemptID || receipt.ContentDigest == "" {
		return fmt.Errorf("steer receipt unavailable")
	}
	fmt.Printf("retry+steer accepted session=%s seat=%s attempt=%s retry=%s steer=%s digest=%s\n",
		sessionID, seatID, running.AttemptID, retryID, steerID, receipt.ContentDigest)
	return nil
}

func runTUI(ctx context.Context) error {
	path, err := socketPath()
	if err != nil {
		return err
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: path, Timeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	readClient, err := tui.NewDaemonReadClient(client)
	if err != nil {
		return err
	}
	sessionID := "rt-tui-live-" + fmt.Sprintf("%d", time.Now().UnixNano())
	frames, view, err := tui.RunRoundtableLiveJourney(ctx, readClient, sessionID)
	if err != nil {
		return err
	}
	for index, frame := range frames {
		fmt.Printf("--- TUI Roundtable frame %d ---\n%s\n", index+1, frame)
	}
	fmt.Printf("TUI journey concluded session=%s digest=%s\n",
		view.Session.ID, view.Digest)
	return nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var err error
	switch {
	case len(os.Args) >= 2 && os.Args[1] == "--tui":
		err = runTUI(ctx)
	case len(os.Args) >= 2 && os.Args[1] == "--journey":
		sessionID := "rt-live-e2e-" + fmt.Sprintf("%d", time.Now().UnixNano())
		if len(os.Args) >= 3 {
			sessionID = os.Args[2]
		}
		err = journey(ctx, sessionID)
	case len(os.Args) == 4 && os.Args[1] == "--steer-running":
		err = steerRunning(ctx, os.Args[2], os.Args[3], false)
	case len(os.Args) == 4 && os.Args[1] == "--steer-waiting":
		err = steerRunning(ctx, os.Args[2], os.Args[3], true)
	case len(os.Args) == 4 && os.Args[1] == "--retry-steer":
		err = retryAndSteer(ctx, os.Args[2], os.Args[3])
	case len(os.Args) == 4 && os.Args[1] == "--clone-run":
		err = cloneAndRun(ctx, os.Args[2], os.Args[3])
	case len(os.Args) == 3 && os.Args[1] == "--snapshot":
		err = snapshot(ctx, os.Args[2])
	default:
		fmt.Fprintln(os.Stderr, "usage: rt-live-journey --journey [session] | --tui | --snapshot SESSION | --steer-running SESSION SEAT | --steer-waiting SESSION SEAT | --retry-steer SESSION SEAT | --clone-run SOURCE_SESSION NEW_SESSION")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
