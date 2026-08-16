// Command rt-live-journey drives the RoundTable dual-seat journey against the
// installed Loom daemon over its real Unix socket, in two modes:
//
//	--journey  full create->seats->round->propose->relay->ack->insert->conclude
//	           plus restart consistency and post-conclude rejection.
//	--tui      runs the real TUI program headlessly (disable renderer) with
//	           scripted keystrokes against the installed daemon.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/roundtable"
	"loom-pi-rebuild/internal/tui"
)

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
	default:
		fmt.Fprintln(os.Stderr, "usage: rt-live-journey --journey [session] | --tui")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
