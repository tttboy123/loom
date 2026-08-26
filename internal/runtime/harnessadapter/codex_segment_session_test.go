package harnessadapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexSegmentSessionReusesOneAppServerThreadAcrossResponses(t *testing.T) {
	session := newCodexAppServerSessionFixture()
	sessions := &codexContinuationSessionRunnerFixture{session: session}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "private")
	for _, directory := range []string{home, workspace, privateRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	prepareCodexAuthFixture(t, home)
	segment, err := OpenCodexSegmentSession(
		context.Background(),
		CodexSegmentSessionConfig{
			ExecutablePath: "/opt/loom/bin/codex", HomePath: home,
			WorkspacePath: workspace, PrivateRoot: privateRoot,
			ModelID: "codex-default", ReasoningEffort: "high",
			SystemPrompt: "Loom governed conversation policy",
			Timeout:      time.Hour, MaxOutputBytes: 1 << 16, Sessions: sessions,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !segment.Healthy() {
		t.Fatal("new Codex Segment Session is not healthy")
	}
	codexHome := filepath.Join(privateRoot, "codex-home")
	authInfo, err := os.Lstat(filepath.Join(codexHome, "auth.json"))
	if err != nil || authInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("isolated auth reference = %v, %v", authInfo, err)
	}
	configInfo, err := os.Lstat(filepath.Join(codexHome, "config.toml"))
	if err != nil || !configInfo.Mode().IsRegular() || configInfo.Mode().Perm() != 0o600 {
		t.Fatalf("isolated config = %v, %v", configInfo, err)
	}
	for index, prompt := range []string{"first private turn", "second private turn"} {
		response, respondErr := segment.Respond(
			context.Background(), []byte(prompt),
		)
		if respondErr != nil {
			t.Fatal(respondErr)
		}
		want := "first reply"
		if index == 1 {
			want = "second reply"
		}
		if response.Content != want || response.Accounting == nil {
			t.Fatalf("response[%d]=%#v", index, response)
		}
	}
	if err := segment.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if segment.Healthy() {
		t.Fatal("closed Codex Segment Session remained healthy")
	}
	if _, err := os.Lstat(codexHome); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("isolated Codex home survived close: %v", err)
	}
	requests := session.requestsSnapshot()
	if len(requests) != 4 || requests[0].Method != "initialize" ||
		requests[1].Method != "thread/start" ||
		requests[2].Method != "turn/start" || requests[3].Method != "turn/start" ||
		requests[2].ThreadID != "thread-locked-1" ||
		requests[3].ThreadID != "thread-locked-1" ||
		requests[2].Text != "first private turn" ||
		requests[3].Text != "second private turn" {
		t.Fatalf("requests=%#v", requests)
	}
	if requests[1].Model != "" || requests[2].Model != "gpt-5.6-sol" ||
		requests[3].Model != "gpt-5.6-sol" {
		t.Fatalf("default model was not resolved and frozen once: %#v", requests)
	}
	if sessions.request.Directory != workspace || requests[1].CWD != workspace {
		t.Fatalf("workspace command=%#v thread=%#v", sessions.request, requests[1])
	}
	arguments := strings.Join(sessions.request.Arguments, "\n")
	for _, forbidden := range []string{"workspace-write", workspace} {
		if strings.Contains(arguments, forbidden) {
			t.Fatalf("workspace leaked into argv: %s", arguments)
		}
	}
	if !session.closed || !session.waited || session.aborted {
		t.Fatalf("closed=%t waited=%t aborted=%t",
			session.closed, session.waited, session.aborted)
	}
}

func TestCodexSegmentSessionRejectsExplicitThreadModelDrift(t *testing.T) {
	session := newCodexAppServerSessionFixture()
	session.modelDrift = true
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "private")
	for _, directory := range []string{home, workspace, privateRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	prepareCodexAuthFixture(t, home)
	_, err := OpenCodexSegmentSession(
		context.Background(),
		CodexSegmentSessionConfig{
			ExecutablePath: "/opt/loom/bin/codex", HomePath: home,
			WorkspacePath: workspace, PrivateRoot: privateRoot,
			ModelID: CodexModelID, SystemPrompt: "Loom governed conversation policy",
			Timeout: time.Hour, MaxOutputBytes: 1 << 16,
			Sessions: &codexContinuationSessionRunnerFixture{session: session},
		},
	)
	if !errors.Is(err, ErrHarnessProtocol) {
		t.Fatalf("explicit model drift error = %v", err)
	}
	if !session.aborted {
		t.Fatal("drifted session was not aborted")
	}
}

func TestCodexNativeConversationAppServerArgumentsUseSupportedAppServerSurface(t *testing.T) {
	arguments := codexNativeConversationAppServerArguments(
		HarnessProcessRequest{
			ModelID: CodexModelID, ReasoningEffort: "high",
		},
		"/private/tmp/loom-system-prompt.txt",
	)
	joined := strings.Join(arguments, "\n")
	if strings.Contains(joined, "--ignore-user-config") {
		t.Fatalf("unsupported app-server flag returned: %s", joined)
	}
	if arguments[len(arguments)-1] != "app-server" {
		t.Fatalf("app-server command missing: %#v", arguments)
	}
	for _, required := range []string{
		`project_doc_max_bytes=0`,
		`mcp_servers={}`,
		`features.apps=false`,
		`features.plugins=false`,
		`features.remote_plugin=false`,
		`features.in_app_browser=false`,
		`features.shell_tool=false`,
		`features.unified_exec=false`,
		`features.apply_patch_freeform=false`,
		`features.tool_search=false`,
		`approval_policy="never"`,
		`sandbox_mode="read-only"`,
		`model_reasoning_effort="high"`,
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("governed app-server arguments missing %q: %s", required, joined)
		}
	}
}

func TestCodexSegmentSessionLifetimeDoesNotInheritOpeningResponseCancellation(t *testing.T) {
	session := newCodexAppServerSessionFixture()
	sessions := &codexContinuationSessionRunnerFixture{session: session}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "private")
	for _, directory := range []string{home, workspace, privateRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	prepareCodexAuthFixture(t, home)
	openingContext, cancelOpening := context.WithCancel(context.Background())
	segment, err := OpenCodexSegmentSession(
		openingContext,
		CodexSegmentSessionConfig{
			ExecutablePath: "/opt/loom/bin/codex", HomePath: home,
			WorkspacePath: workspace, PrivateRoot: privateRoot,
			ModelID: CodexModelID, SystemPrompt: "Loom governed conversation policy",
			Timeout: time.Hour, MaxOutputBytes: 1 << 16, Sessions: sessions,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	cancelOpening()
	if sessions.startContext == nil || sessions.startContext.Err() != nil {
		t.Fatalf("persistent process inherited opening cancellation: %v", sessions.startContext)
	}
	if err := segment.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(sessions.startContext.Err(), context.Canceled) {
		t.Fatalf("session lifetime was not cancelled on close: %v", sessions.startContext.Err())
	}
}

func TestCodexSegmentSessionInterruptsCancelledResponseAndReusesThread(t *testing.T) {
	for _, test := range []struct {
		name                     string
		completionBeforeResponse bool
		opaqueCancellationError  bool
	}{
		{name: "response_before_completion"},
		{name: "completion_before_response", completionBeforeResponse: true},
		{name: "opaque_transport_cancellation", opaqueCancellationError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := newCodexAppServerSessionFixture()
			session.holdFirstTurn = true
			session.interruptBeforeResponse = test.completionBeforeResponse
			session.opaqueCancellationError = test.opaqueCancellationError
			segment := openCodexSegmentSessionForTest(t, session)

			responseContext, cancelResponse := context.WithCancel(context.Background())
			responseDone := make(chan error, 1)
			go func() {
				_, err := segment.Respond(responseContext, []byte("private cancelled turn"))
				responseDone <- err
			}()
			<-session.firstTurnResponseRead
			cancelResponse()
			if err := <-responseDone; !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled response error = %v", err)
			}
			if !segment.Healthy() {
				t.Fatal("cleanly interrupted Segment Session became unhealthy")
			}

			response, err := segment.Respond(context.Background(), []byte("private next turn"))
			if err != nil {
				t.Fatal(err)
			}
			if response.Content != "second reply" || response.Accounting == nil {
				t.Fatal("next response metadata was incomplete")
			}
			if err := segment.Close(context.Background()); err != nil {
				t.Fatal(err)
			}
			requests := session.requestsSnapshot()
			if len(requests) != 5 || requests[2].Method != "turn/start" ||
				requests[3].Method != "turn/interrupt" ||
				requests[3].ThreadID != "thread-locked-1" ||
				requests[3].TurnID != "turn-locked-1" ||
				requests[4].Method != "turn/start" ||
				requests[4].ThreadID != "thread-locked-1" {
				t.Fatalf("unexpected request metadata: count=%d", len(requests))
			}
			if session.aborted {
				t.Fatal("reusable session was aborted")
			}
		})
	}
}

func TestCodexSegmentSessionRecoversTurnIDWhenCancelledBeforeStartResponse(t *testing.T) {
	session := newCodexAppServerSessionFixture()
	session.cancelBeforeTurnResponse = true
	session.turnStartedBeforeResponse = true
	session.holdFirstTurn = true
	segment := openCodexSegmentSessionForTest(t, session)

	responseContext, cancelResponse := context.WithCancel(context.Background())
	responseDone := make(chan error, 1)
	go func() {
		_, err := segment.Respond(responseContext, []byte("private cancelled turn"))
		responseDone <- err
	}()
	<-session.turnStartWritten
	cancelResponse()
	if err := <-responseDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled response error = %v", err)
	}
	if !segment.Healthy() {
		t.Fatal("recovered pre-response cancellation made Segment Session unhealthy")
	}
	if !session.turnStartResponseBounded {
		t.Fatal("turn ID recovery read had no deadline")
	}

	response, err := segment.Respond(context.Background(), []byte("private next turn"))
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "second reply" || response.Accounting == nil {
		t.Fatal("next response metadata was incomplete")
	}
	if err := segment.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	requests := session.requestsSnapshot()
	if len(requests) != 5 || requests[2].Method != "turn/start" ||
		requests[3].Method != "turn/interrupt" ||
		requests[3].ThreadID != "thread-locked-1" ||
		requests[3].TurnID != "turn-locked-1" ||
		requests[4].Method != "turn/start" ||
		requests[4].ThreadID != "thread-locked-1" {
		t.Fatalf("unexpected request metadata: count=%d", len(requests))
	}
	if session.aborted {
		t.Fatal("reusable session was aborted")
	}
}

func TestCodexSegmentSessionFailsWhenCancelledTurnIDCannotBeRecovered(t *testing.T) {
	session := newCodexAppServerSessionFixture()
	session.cancelBeforeTurnResponse = true
	session.dropTurnStartResponse = true
	segment := openCodexSegmentSessionForTest(t, session)

	responseContext, cancelResponse := context.WithCancel(context.Background())
	responseDone := make(chan error, 1)
	go func() {
		_, err := segment.Respond(responseContext, []byte("private cancelled turn"))
		responseDone <- err
	}()
	<-session.turnStartWritten
	cancelResponse()
	if err := <-responseDone; err == nil {
		t.Fatal("missing turn ID recovery failure accepted")
	}
	if segment.Healthy() {
		t.Fatal("unrecoverable native turn left Segment Session healthy")
	}
	requests := session.requestsSnapshot()
	if len(requests) != 3 || requests[2].Method != "turn/start" ||
		!session.turnStartRecoveryBounded || !session.aborted {
		t.Fatalf("unsafe failed recovery: requests=%d bounded=%t aborted=%t",
			len(requests), session.turnStartRecoveryBounded, session.aborted)
	}
}

func TestCodexSegmentSessionInterruptFailureClosesSession(t *testing.T) {
	for _, behavior := range []string{
		"response-mismatch", "mismatch", "status-mismatch", "timeout",
	} {
		t.Run(behavior, func(t *testing.T) {
			session := newCodexAppServerSessionFixture()
			session.holdFirstTurn = true
			session.interruptBehavior = behavior
			segment := openCodexSegmentSessionForTest(t, session)

			responseContext, cancelResponse := context.WithCancel(context.Background())
			responseDone := make(chan error, 1)
			go func() {
				_, err := segment.Respond(responseContext, []byte("private cancelled turn"))
				responseDone <- err
			}()
			<-session.firstTurnResponseRead
			cancelResponse()
			if err := <-responseDone; err == nil {
				t.Fatal("interrupt failure accepted")
			}
			if segment.Healthy() {
				t.Fatal("failed interrupt left Segment Session healthy")
			}
			requestCount := len(session.requestsSnapshot())
			if _, err := segment.Respond(context.Background(), []byte("must not run")); !errors.Is(err, ErrHarnessProcessUnavailable) {
				t.Fatalf("failed session reuse error = %v", err)
			}
			if len(session.requestsSnapshot()) != requestCount || !session.aborted {
				t.Fatalf("failed session remained reusable: requests=%d aborted=%t",
					len(session.requestsSnapshot()), session.aborted)
			}
			if behavior == "timeout" && !session.interruptReadBounded {
				t.Fatal("interrupt cleanup read had no deadline")
			}
		})
	}
}

func openCodexSegmentSessionForTest(
	t *testing.T,
	stream HarnessStreamSession,
) *CodexSegmentSession {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "private")
	for _, directory := range []string{home, workspace, privateRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	prepareCodexAuthFixture(t, home)
	segment, err := OpenCodexSegmentSession(
		context.Background(),
		CodexSegmentSessionConfig{
			ExecutablePath: "/opt/loom/bin/codex", HomePath: home,
			WorkspacePath: workspace, PrivateRoot: privateRoot,
			ModelID: CodexModelID, SystemPrompt: "Loom governed conversation policy",
			Timeout: time.Hour, MaxOutputBytes: 1 << 16,
			Sessions: &codexContinuationSessionRunnerFixture{session: stream},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return segment
}

func prepareCodexAuthFixture(t *testing.T, home string) {
	t.Helper()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
}
