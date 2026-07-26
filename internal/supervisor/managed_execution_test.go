package supervisor

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"

	_ "modernc.org/sqlite"
)

var managedTestNow = time.Date(2026, 7, 26, 8, 9, 10, 0, time.UTC)

type managedTestClock struct {
	mu  sync.RWMutex
	now time.Time
}

func (clock *managedTestClock) Now() time.Time {
	clock.mu.RLock()
	defer clock.mu.RUnlock()
	return clock.now
}

func (clock *managedTestClock) Set(now time.Time) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = now
}

type managedExecutionFixture struct {
	store          *journal.Store
	workAuthority  *work.Authority
	grantAuthority *authorization.Authority
	workItem       work.WorkItemRecord
	run            work.RunRecord
	grant          authorization.IssuedGrant
	generation     work.RunGenerationInput
	profile        loomruntime.RuntimeProfile
	instance       loomruntime.RuntimeInstance
	dispatch       bridgev1.Frame
	sourcePath     string
	workspaceRoot  string
	clock          *managedTestClock
	dsn            string
}

type fakeRuntimeAdapter struct {
	mu              sync.Mutex
	adapterType     string
	instanceID      string
	result          AdapterResult
	err             error
	hook            func(AdapterRequest) error
	requests        []AdapterRequest
	lastWorkspace   string
	lastGrantValue  string
	executionCalled bool
	waitForContext  bool
	started         chan struct{}
	release         chan struct{}
}

func (adapter *fakeRuntimeAdapter) AdapterType() string {
	return adapter.adapterType
}

func (adapter *fakeRuntimeAdapter) RuntimeInstanceID() string {
	return adapter.instanceID
}

func (adapter *fakeRuntimeAdapter) Execute(
	ctx context.Context,
	request AdapterRequest,
) (AdapterResult, error) {
	adapter.mu.Lock()
	adapter.executionCalled = true
	adapter.lastWorkspace = request.WorkspacePath
	adapter.lastGrantValue = request.Grant.Value()
	adapter.requests = append(adapter.requests, request)
	hook := adapter.hook
	result := adapter.result
	err := adapter.err
	waitForContext := adapter.waitForContext
	started := adapter.started
	release := adapter.release
	adapter.mu.Unlock()
	if started != nil {
		close(started)
	}
	if waitForContext {
		<-ctx.Done()
		return AdapterResult{}, ctx.Err()
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return AdapterResult{}, ctx.Err()
		}
	}
	if hook != nil {
		if hookErr := hook(request); hookErr != nil {
			return AdapterResult{}, hookErr
		}
	}
	return result, err
}

func TestSupervisorManagedSuccessAndFrameAuthorization(t *testing.T) { // s3_w4_grant_frame_authorization
	fixture := newManagedExecutionFixture(t, "success")
	frames := managedInboundFrames(t, fixture, "succeeded", "")
	diagnostic := []byte("bounded diagnostic")
	result, err := NewAdapterResult(AdapterResultInput{
		InboundFrames:        frames,
		Stderr:               diagnostic,
		ExitCode:             0,
		DispatchAcknowledged: true,
		ResultAcknowledged:   true,
	})
	if err != nil {
		t.Fatalf("NewAdapterResult() error = %v", err)
	}
	diagnostic[0] = 'X'
	resultFrames := result.InboundFrames()
	resultFrames[0] = bridgev1.Frame{}
	if string(result.Stderr()) != "bounded diagnostic" ||
		result.InboundFrames()[0].Type() != bridgev1.MessageAck {
		t.Fatal("AdapterResult aliases input or accessor")
	}
	adapter := &fakeRuntimeAdapter{
		adapterType: fixture.instance.AdapterType,
		instanceID:  fixture.instance.ID,
		result:      result,
		hook: func(request AdapterRequest) error {
			return os.WriteFile(
				filepath.Join(request.WorkspacePath, "result.txt"),
				[]byte("managed output\n"),
				0o600,
			)
		},
	}
	controller, err := New(
		Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
		fixture.workAuthority,
		fixture.grantAuthority,
		adapter,
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	outcome, err := controller.Execute(context.Background(), fixture.input())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if outcome.WorkItem().Status() != "ready_for_review" ||
		outcome.Run().TerminalStatus() != "succeeded" ||
		outcome.Run().TerminalReason() != "" {
		t.Fatalf(
			"terminal = work %q run %q/%q",
			outcome.WorkItem().Status(),
			outcome.Run().TerminalStatus(),
			outcome.Run().TerminalReason(),
		)
	}
	if !outcome.Stream().TerminalResultSeen() ||
		len(outcome.Stream().Frames()) != len(frames) {
		t.Fatalf("stream = %#v", outcome.Stream())
	}
	if outcome.SourceDigest() == "" || outcome.WorkspaceDigest() == "" {
		t.Fatalf("empty digests: %#v", outcome)
	}
	if got := sortedManagedChangePaths(outcome.Changes()); !equalManagedStrings(
		got,
		[]string{"result.txt"},
	) {
		t.Fatalf("changes = %#v", got)
	}
	if string(outcome.Changes()[0].Content()) != "managed output\n" ||
		outcome.Changes()[0].Mode() != 0o600 ||
		outcome.Changes()[0].Digest() == "" ||
		string(outcome.Stderr()) != "bounded diagnostic" {
		t.Fatalf("outcome content = %#v / %q", outcome.Changes(), outcome.Stderr())
	}
	outcomeChanges := outcome.Changes()
	outcomeChanges[0] = WorkspaceChange{}
	outcomeStderr := outcome.Stderr()
	outcomeStderr[0] = 'X'
	if outcome.Changes()[0].Path() != "result.txt" ||
		string(outcome.Stderr()) != "bounded diagnostic" {
		t.Fatal("Outcome aliases accessors")
	}
	if adapter.lastGrantValue != fixture.grant.Token().Value() {
		t.Fatal("adapter did not receive exact grant")
	}
	if _, statErr := os.Lstat(adapter.lastWorkspace); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("managed workspace remains after Execute: %v", statErr)
	}
	assertManagedGrantRevoked(t, fixture, authorization.RevocationTerminal)
	assertManagedAuthorizations(t, fixture, frames)
	assertManagedTokenAbsentFromJournal(t, fixture)

	t.Run("one active execution per run", func(t *testing.T) {
		active := newManagedExecutionFixture(t, "active")
		activeFrames := managedInboundFrames(t, active, "succeeded", "")
		activeResult, resultErr := NewAdapterResult(AdapterResultInput{
			InboundFrames:        activeFrames,
			ExitCode:             0,
			DispatchAcknowledged: true,
			ResultAcknowledged:   true,
		})
		if resultErr != nil {
			t.Fatal(resultErr)
		}
		started := make(chan struct{})
		release := make(chan struct{})
		activeAdapter := &fakeRuntimeAdapter{
			adapterType: active.instance.AdapterType,
			instanceID:  active.instance.ID,
			result:      activeResult,
			started:     started,
			release:     release,
		}
		activeController, newErr := New(
			Config{WorkspaceRoot: active.workspaceRoot, CleanupTimeout: time.Second},
			active.workAuthority,
			active.grantAuthority,
			activeAdapter,
		)
		if newErr != nil {
			t.Fatal(newErr)
		}
		type executionAnswer struct {
			outcome Outcome
			err     error
		}
		answer := make(chan executionAnswer, 1)
		go func() {
			value, executeErr := activeController.Execute(
				context.Background(),
				active.input(),
			)
			answer <- executionAnswer{outcome: value, err: executeErr}
		}()
		<-started
		if duplicate, duplicateErr := activeController.Execute(
			context.Background(),
			active.input(),
		); !errors.Is(duplicateErr, ErrInvalidManagedExecution) ||
			duplicate.Run().ID() != "" {
			t.Fatalf("duplicate execution = %#v, %v", duplicate, duplicateErr)
		}
		close(release)
		first := <-answer
		if first.err != nil || first.outcome.Run().TerminalStatus() != "succeeded" {
			t.Fatalf("first execution = %#v, %v", first.outcome, first.err)
		}
		activeAdapter.mu.Lock()
		requestCount := len(activeAdapter.requests)
		activeAdapter.mu.Unlock()
		if requestCount != 1 {
			t.Fatalf("adapter request count = %d", requestCount)
		}
		if !activeController.acquireRun("unrelated-a") ||
			!activeController.acquireRun("unrelated-b") {
			t.Fatal("unrelated Run IDs blocked each other")
		}
		activeController.releaseRun("unrelated-a")
		activeController.releaseRun("unrelated-b")
	})

	t.Run("child failed is an exact failure terminal", func(t *testing.T) {
		failed := newManagedExecutionFixture(t, "child-failed")
		failedFrames := managedInboundFrames(t, failed, "failed", "agent_failure")
		failedResult, resultErr := NewAdapterResult(AdapterResultInput{
			InboundFrames:        failedFrames,
			ExitCode:             0,
			DispatchAcknowledged: true,
			ResultAcknowledged:   true,
		})
		if resultErr != nil {
			t.Fatal(resultErr)
		}
		failedAdapter := &fakeRuntimeAdapter{
			adapterType: failed.instance.AdapterType,
			instanceID:  failed.instance.ID,
			result:      failedResult,
		}
		failedController, newErr := New(
			Config{WorkspaceRoot: failed.workspaceRoot, CleanupTimeout: time.Second},
			failed.workAuthority,
			failed.grantAuthority,
			failedAdapter,
		)
		if newErr != nil {
			t.Fatal(newErr)
		}
		failedOutcome, executeErr := failedController.Execute(
			context.Background(),
			failed.input(),
		)
		if executeErr != nil {
			t.Fatalf("failed child Execute() error = %v", executeErr)
		}
		if failedOutcome.WorkItem().Status() != "failed" ||
			failedOutcome.Run().TerminalStatus() != "failed" ||
			failedOutcome.Run().TerminalReason() != "agent_failure" {
			t.Fatalf("failed terminal = %#v", failedOutcome)
		}
		assertManagedGrantRevoked(t, failed, authorization.RevocationTerminal)
	})
}

func TestManagedExecutionValidationMatrix(t *testing.T) {
	valid, err := NewAdapterResult(AdapterResultInput{
		Stderr:             []byte("diagnostic"),
		ExitCode:           7,
		CancelAcknowledged: true,
	})
	if err != nil ||
		valid.ExitCode() != 7 ||
		!valid.CancelAcknowledged() ||
		string(valid.Stderr()) != "diagnostic" {
		t.Fatalf("valid AdapterResult = %#v, %v", valid, err)
	}
	tooManyFrames := make([]bridgev1.Frame, bridgev1.MaxBufferedFrames+1)
	for _, testCase := range []struct {
		name  string
		input AdapterResultInput
	}{
		{"stderr bound", AdapterResultInput{Stderr: make([]byte, maxManagedStderrBytes+1)}},
		{"frame bound", AdapterResultInput{InboundFrames: tooManyFrames}},
		{"negative exit", AdapterResultInput{ExitCode: -2}},
		{"large exit", AdapterResultInput{ExitCode: 256}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			result, resultErr := NewAdapterResult(testCase.input)
			if !errors.Is(resultErr, ErrInvalidManagedExecution) ||
				len(result.InboundFrames()) != 0 ||
				len(result.Stderr()) != 0 {
				t.Fatalf("invalid result = %#v, %v", result, resultErr)
			}
		})
	}

	resultPayloads := []struct {
		name    string
		payload string
	}{
		{"missing field", `{"status":"succeeded"}`},
		{"unknown field", `{"status":"succeeded","reason":"","extra":"x"}`},
		{"duplicate field", `{"status":"succeeded","status":"failed","reason":""}`},
		{"success reason", `{"status":"succeeded","reason":"not-empty"}`},
		{"failed empty reason", `{"status":"failed","reason":""}`},
		{"failed control reason", "{\"status\":\"failed\",\"reason\":\"bad\\nreason\"}"},
		{"unknown status", `{"status":"done","reason":""}`},
		{"trailing value", `{"status":"succeeded","reason":""} {}`},
	}
	for _, testCase := range resultPayloads {
		t.Run(testCase.name, func(t *testing.T) {
			status, reason, parseErr := parseManagedResultPayload([]byte(testCase.payload))
			if !errors.Is(parseErr, ErrBridgeSession) || status != "" || reason != "" {
				t.Fatalf("parse result = %q/%q, %v", status, reason, parseErr)
			}
		})
	}

	fixture := newManagedExecutionFixture(t, "constructor-validation")
	adapter := &fakeRuntimeAdapter{
		adapterType: fixture.instance.AdapterType,
		instanceID:  fixture.instance.ID,
	}
	for _, testCase := range []struct {
		name    string
		config  Config
		work    *work.Authority
		grant   *authorization.Authority
		adapter RuntimeAdapter
	}{
		{
			name:   "zero cleanup",
			config: Config{WorkspaceRoot: fixture.workspaceRoot},
			work:   fixture.workAuthority, grant: fixture.grantAuthority, adapter: adapter,
		},
		{
			name:   "long cleanup",
			config: Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: 31 * time.Second},
			work:   fixture.workAuthority, grant: fixture.grantAuthority, adapter: adapter,
		},
		{
			name:   "nil work",
			config: Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			grant:  fixture.grantAuthority, adapter: adapter,
		},
		{
			name:   "nil grant",
			config: Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			work:   fixture.workAuthority, adapter: adapter,
		},
		{
			name:   "nil adapter",
			config: Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			work:   fixture.workAuthority, grant: fixture.grantAuthority,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			controller, newErr := New(
				testCase.config,
				testCase.work,
				testCase.grant,
				testCase.adapter,
			)
			if !errors.Is(newErr, ErrInvalidManagedExecution) || controller != nil {
				t.Fatalf("invalid Supervisor = %#v, %v", controller, newErr)
			}
		})
	}
}

func TestSupervisorRejectsInvalidBridgeSessions(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(testing.TB, *managedExecutionFixture, []bridgev1.Frame) []bridgev1.Frame
	}{
		{
			name: "wrong dispatch acknowledgement",
			mutate: func(
				t testing.TB,
				fixture *managedExecutionFixture,
				frames []bridgev1.Frame,
			) []bridgev1.Frame {
				frames[0] = managedFrame(
					t,
					fixture.generation,
					2,
					bridgev1.MessageAck,
					[]byte(`{"message_id":"33333333-3333-4333-8333-333333333333"}`),
				)
				return frames
			},
		},
		{
			name: "missing result",
			mutate: func(
				_ testing.TB,
				_ *managedExecutionFixture,
				frames []bridgev1.Frame,
			) []bridgev1.Frame {
				return frames[:len(frames)-1]
			},
		},
		{
			name: "unknown result field",
			mutate: func(
				t testing.TB,
				fixture *managedExecutionFixture,
				frames []bridgev1.Frame,
			) []bridgev1.Frame {
				frames[len(frames)-1] = managedFrame(
					t,
					fixture.generation,
					6,
					bridgev1.MessageResult,
					[]byte(`{"status":"succeeded","reason":"","extra":"x"}`),
				)
				return frames
			},
		},
		{
			name: "frame after result",
			mutate: func(
				t testing.TB,
				fixture *managedExecutionFixture,
				frames []bridgev1.Frame,
			) []bridgev1.Frame {
				return append(frames, managedFrame(
					t,
					fixture.generation,
					7,
					bridgev1.MessageEvent,
					[]byte(`{"event":"late"}`),
				))
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newManagedExecutionFixture(
				t,
				"bridge-"+strings.ReplaceAll(testCase.name, " ", "-"),
			)
			frames := testCase.mutate(
				t,
				fixture,
				managedInboundFrames(t, fixture, "succeeded", ""),
			)
			result, err := NewAdapterResult(AdapterResultInput{
				InboundFrames:        frames,
				ExitCode:             0,
				DispatchAcknowledged: true,
				ResultAcknowledged:   true,
			})
			if err != nil {
				t.Fatal(err)
			}
			adapter := &fakeRuntimeAdapter{
				adapterType: fixture.instance.AdapterType,
				instanceID:  fixture.instance.ID,
				result:      result,
			}
			controller, err := New(
				Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
				fixture.workAuthority,
				fixture.grantAuthority,
				adapter,
			)
			if err != nil {
				t.Fatal(err)
			}
			outcome, executeErr := controller.Execute(context.Background(), fixture.input())
			if !errors.Is(executeErr, ErrBridgeSession) ||
				outcome.Run().TerminalStatus() != "failed" ||
				outcome.Run().TerminalReason() != "bridge_protocol_failed" ||
				len(outcome.Changes()) != 0 {
				t.Fatalf("bridge rejection = %#v, %v", outcome, executeErr)
			}
			assertManagedGrantRevoked(t, fixture, authorization.RevocationTerminal)
		})
	}
}

func TestSupervisorTerminalFailureAlwaysRevokes(t *testing.T) { // s3_w4_terminal_failure_revoke
	testCases := []struct {
		name           string
		adapterErr     error
		cancel         bool
		wait           bool
		callerTimeout  time.Duration
		profileTimeout time.Duration
		wantStatus     string
		wantReason     string
		wantError      error
		wantRevoke     authorization.RevocationReason
	}{
		{
			name:       "process failure",
			adapterErr: ErrRuntimeAdapter,
			wantStatus: "failed",
			wantReason: "runtime_process_failed",
			wantError:  ErrRuntimeAdapter,
			wantRevoke: authorization.RevocationTerminal,
		},
		{
			name:       "caller cancellation",
			cancel:     true,
			wantStatus: "cancelled",
			wantReason: "operator_cancelled",
			wantError:  ErrRuntimeCancelled,
			wantRevoke: authorization.RevocationCancelled,
		},
		{
			name:          "caller cancellation after start",
			wait:          true,
			callerTimeout: time.Second,
			wantStatus:    "cancelled",
			wantReason:    "operator_cancelled",
			wantError:     ErrRuntimeCancelled,
			wantRevoke:    authorization.RevocationCancelled,
		},
		{
			name:           "profile timeout",
			wait:           true,
			profileTimeout: 25 * time.Millisecond,
			wantStatus:     "failed",
			wantReason:     "runtime_timeout",
			wantError:      ErrRuntimeTimeout,
			wantRevoke:     authorization.RevocationTimeout,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newManagedExecutionFixture(t, strings.ReplaceAll(testCase.name, " ", "-"))
			adapter := &fakeRuntimeAdapter{
				adapterType:    fixture.instance.AdapterType,
				instanceID:     fixture.instance.ID,
				err:            testCase.adapterErr,
				waitForContext: testCase.wait,
			}
			ctx := context.Background()
			if testCase.cancel {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			if testCase.callerTimeout > 0 {
				bounded, cancel := context.WithTimeout(ctx, testCase.callerTimeout)
				defer cancel()
				ctx = bounded
			}
			if testCase.profileTimeout > 0 {
				fixture.profile.Timeout = testCase.profileTimeout
			}
			controller, err := New(
				Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
				fixture.workAuthority,
				fixture.grantAuthority,
				adapter,
			)
			if err != nil {
				t.Fatal(err)
			}
			outcome, executeErr := controller.Execute(ctx, fixture.input())
			if !errors.Is(executeErr, testCase.wantError) {
				t.Fatalf("Execute() error = %v, want %v", executeErr, testCase.wantError)
			}
			if outcome.Run().TerminalStatus() != testCase.wantStatus ||
				outcome.Run().TerminalReason() != testCase.wantReason {
				t.Fatalf("terminal outcome = %#v", outcome)
			}
			if testCase.wait && !adapter.executionCalled {
				t.Fatal("cancellation/timeout did not reach child execution")
			}
			assertManagedGrantRevoked(t, fixture, testCase.wantRevoke)
			if testCase.wait {
				assertManagedReopenedTerminal(
					t,
					fixture,
					testCase.wantStatus,
					testCase.wantReason,
					testCase.wantRevoke,
				)
			}
			if adapter.lastWorkspace != "" {
				if _, statErr := os.Lstat(adapter.lastWorkspace); !errors.Is(
					statErr,
					os.ErrNotExist,
				) {
					t.Fatalf("workspace remains after failure: %v", statErr)
				}
			}
		})
	}

	t.Run("raw grant in source is rejected before adapter", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "source-token")
		if err := os.WriteFile(
			filepath.Join(fixture.sourcePath, "input.txt"),
			[]byte(fixture.grant.Token().Value()),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		adapter := &fakeRuntimeAdapter{
			adapterType: fixture.instance.AdapterType,
			instanceID:  fixture.instance.ID,
		}
		controller, err := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			adapter,
		)
		if err != nil {
			t.Fatal(err)
		}
		outcome, executeErr := controller.Execute(context.Background(), fixture.input())
		if !errors.Is(executeErr, ErrManagedWorkspace) ||
			adapter.executionCalled ||
			outcome.Run().TerminalReason() != "workspace_failed" {
			t.Fatalf("source token outcome = %#v, %v, adapter=%#v", outcome, executeErr, adapter)
		}
		assertManagedGrantRevoked(t, fixture, authorization.RevocationTerminal)
	})

	t.Run("raw grant in source path is rejected before adapter", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "source-token-path")
		if err := os.WriteFile(
			filepath.Join(
				fixture.sourcePath,
				"leak-"+fixture.grant.Token().Value(),
			),
			[]byte("safe-content"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		adapter := &fakeRuntimeAdapter{
			adapterType: fixture.instance.AdapterType,
			instanceID:  fixture.instance.ID,
		}
		controller, err := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			adapter,
		)
		if err != nil {
			t.Fatal(err)
		}
		outcome, executeErr := controller.Execute(context.Background(), fixture.input())
		if !errors.Is(executeErr, ErrManagedWorkspace) ||
			adapter.executionCalled ||
			outcome.Run().TerminalReason() != "workspace_failed" {
			t.Fatalf(
				"source token path outcome = %#v, %v, adapter=%#v",
				outcome,
				executeErr,
				adapter,
			)
		}
		assertManagedGrantRevoked(t, fixture, authorization.RevocationTerminal)
	})

	t.Run("cancellation during workspace preparation", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "cancel-workspace")
		adapter := &fakeRuntimeAdapter{
			adapterType: fixture.instance.AdapterType,
			instanceID:  fixture.instance.ID,
		}
		controller, err := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			adapter,
		)
		if err != nil {
			t.Fatal(err)
		}
		originalHook := managedTraversalBeforeOpen
		reached := make(chan struct{})
		release := make(chan struct{})
		var block sync.Once
		managedTraversalBeforeOpen = func(relative string) {
			if relative == "input.txt" {
				block.Do(func() {
					close(reached)
					<-release
				})
			}
		}
		defer func() { managedTraversalBeforeOpen = originalHook }()

		ctx, cancel := context.WithCancel(context.Background())
		type cancellationAnswer struct {
			outcome Outcome
			err     error
		}
		answer := make(chan cancellationAnswer, 1)
		go func() {
			value, executeErr := controller.Execute(ctx, fixture.input())
			answer <- cancellationAnswer{outcome: value, err: executeErr}
		}()
		<-reached
		cancel()
		close(release)
		cancelled := <-answer
		if !errors.Is(cancelled.err, ErrRuntimeCancelled) ||
			cancelled.outcome.Run().TerminalStatus() != "cancelled" ||
			cancelled.outcome.Run().TerminalReason() != "operator_cancelled" ||
			adapter.executionCalled {
			t.Fatalf(
				"workspace cancellation = %#v, %v, adapter=%#v",
				cancelled.outcome,
				cancelled.err,
				adapter,
			)
		}
		assertManagedGrantRevoked(t, fixture, authorization.RevocationCancelled)
	})

	t.Run("raw grant in workspace or stderr is never returned", func(t *testing.T) {
		for _, surface := range []string{"workspace-content", "workspace-path", "stderr"} {
			t.Run(surface, func(t *testing.T) {
				fixture := newManagedExecutionFixture(t, "token-"+surface)
				frames := managedInboundFrames(t, fixture, "succeeded", "")
				resultInput := AdapterResultInput{
					InboundFrames:        frames,
					ExitCode:             0,
					DispatchAcknowledged: true,
					ResultAcknowledged:   true,
				}
				if surface == "stderr" {
					resultInput.Stderr = []byte(fixture.grant.Token().Value())
				}
				result, resultErr := NewAdapterResult(resultInput)
				if resultErr != nil {
					t.Fatal(resultErr)
				}
				adapter := &fakeRuntimeAdapter{
					adapterType: fixture.instance.AdapterType,
					instanceID:  fixture.instance.ID,
					result:      result,
				}
				if strings.HasPrefix(surface, "workspace-") {
					adapter.hook = func(request AdapterRequest) error {
						name := "leak"
						content := []byte(fixture.grant.Token().Value())
						if surface == "workspace-path" {
							name = "leak-" + fixture.grant.Token().Value()
							content = []byte("safe-content")
						}
						return os.WriteFile(
							filepath.Join(request.WorkspacePath, name),
							content,
							0o600,
						)
					}
				}
				controller, newErr := New(
					Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
					fixture.workAuthority,
					fixture.grantAuthority,
					adapter,
				)
				if newErr != nil {
					t.Fatal(newErr)
				}
				outcome, executeErr := controller.Execute(
					context.Background(),
					fixture.input(),
				)
				if !errors.Is(executeErr, ErrManagedWorkspace) ||
					outcome.Run().TerminalReason() != "workspace_failed" ||
					len(outcome.Changes()) != 0 ||
					len(outcome.Stderr()) != 0 {
					t.Fatalf("%s token outcome = %#v, %v", surface, outcome, executeErr)
				}
				assertManagedGrantRevoked(t, fixture, authorization.RevocationTerminal)
			})
		}
	})

	t.Run("raw grant in Bridge payload is never accepted", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "token-frame")
		frames := managedInboundFrames(t, fixture, "succeeded", "")
		payload, err := json.Marshal(map[string]string{
			"value": fixture.grant.Token().Value(),
		})
		if err != nil {
			t.Fatal(err)
		}
		frames[1] = managedFrame(
			t,
			fixture.generation,
			3,
			bridgev1.MessageEvent,
			payload,
		)
		result, err := NewAdapterResult(AdapterResultInput{
			InboundFrames:        frames,
			ExitCode:             0,
			DispatchAcknowledged: true,
			ResultAcknowledged:   true,
		})
		if err != nil {
			t.Fatal(err)
		}
		adapter := &fakeRuntimeAdapter{
			adapterType: fixture.instance.AdapterType,
			instanceID:  fixture.instance.ID,
			result:      result,
		}
		controller, err := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			adapter,
		)
		if err != nil {
			t.Fatal(err)
		}
		outcome, executeErr := controller.Execute(context.Background(), fixture.input())
		if !errors.Is(executeErr, ErrBridgeSession) ||
			outcome.Run().TerminalReason() != "bridge_protocol_failed" ||
			len(outcome.Changes()) != 0 ||
			len(outcome.Stderr()) != 0 {
			t.Fatalf("frame token outcome = %#v, %v", outcome, executeErr)
		}
		assertManagedGrantRevoked(t, fixture, authorization.RevocationTerminal)
	})

	t.Run("raw grant in dispatch is validation-only", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "token-dispatch")
		input := fixture.input()
		payload, err := json.Marshal(map[string]string{
			"token": fixture.grant.Token().Value(),
		})
		if err != nil {
			t.Fatal(err)
		}
		input.Dispatch = managedFrame(
			t,
			fixture.generation,
			1,
			bridgev1.MessageDispatch,
			payload,
		)
		adapter := &fakeRuntimeAdapter{
			adapterType: fixture.instance.AdapterType,
			instanceID:  fixture.instance.ID,
		}
		controller, err := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			adapter,
		)
		if err != nil {
			t.Fatal(err)
		}
		outcome, executeErr := controller.Execute(context.Background(), input)
		if !errors.Is(executeErr, ErrInvalidManagedExecution) ||
			outcome.Run().ID() != "" ||
			adapter.executionCalled {
			t.Fatalf("dispatch token outcome = %#v, %v", outcome, executeErr)
		}
	})

	t.Run("terminal commit survives later revoke conflict", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "revoke-conflict")
		controller, err := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			&fakeRuntimeAdapter{
				adapterType: fixture.instance.AdapterType,
				instanceID:  fixture.instance.ID,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := fixture.workAuthority.Start(
			context.Background(),
			fixture.generation,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.grantAuthority.Revoke(
			context.Background(),
			authorization.RevokeInput{
				GrantID:       fixture.grant.Record().ID(),
				Reason:        authorization.RevocationOperator,
				CorrelationID: fixture.generation.CorrelationID,
			},
		); err != nil {
			t.Fatal(err)
		}
		outcome, finishErr := controller.finishTerminal(
			fixture.input(),
			bridgev1.BoundRunStream{},
			"",
			"",
			nil,
			nil,
			"failed",
			"external_failure",
			authorization.RevocationTerminal,
			nil,
		)
		if !errors.Is(finishErr, authorization.ErrGrantAlreadyRevoked) ||
			outcome.Run().TerminalStatus() != "failed" ||
			outcome.Run().TerminalReason() != "external_failure" {
			t.Fatalf("revoke conflict outcome = %#v, %v", outcome, finishErr)
		}
	})

	t.Run("terminal conflict still revokes grant", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "terminal-conflict")
		controller, err := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			&fakeRuntimeAdapter{
				adapterType: fixture.instance.AdapterType,
				instanceID:  fixture.instance.ID,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := fixture.workAuthority.Start(
			context.Background(),
			fixture.generation,
		); err != nil {
			t.Fatal(err)
		}
		if _, _, err := fixture.workAuthority.CommitTerminal(
			context.Background(),
			work.RunTerminalInput{
				RunGenerationInput: fixture.generation,
				Status:             "succeeded",
				Reason:             "",
			},
		); err != nil {
			t.Fatal(err)
		}
		outcome, finishErr := controller.finishTerminal(
			fixture.input(),
			bridgev1.BoundRunStream{},
			"",
			"",
			nil,
			nil,
			"failed",
			"bridge_protocol_failed",
			authorization.RevocationTerminal,
			ErrBridgeSession,
		)
		if !errors.Is(finishErr, work.ErrRunAlreadyTerminal) ||
			!errors.Is(finishErr, ErrBridgeSession) ||
			outcome.Run().ID() != "" {
			t.Fatalf("terminal conflict outcome = %#v, %v", outcome, finishErr)
		}
		assertManagedGrantRevoked(t, fixture, authorization.RevocationTerminal)
	})
}

func TestSupervisorStaleGenerationAndSourceChanged(t *testing.T) { // s3_w4_stale_generation_source_changed
	t.Run("stale generation never invokes adapter", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "stale")
		adapter := &fakeRuntimeAdapter{
			adapterType: fixture.instance.AdapterType,
			instanceID:  fixture.instance.ID,
		}
		controller, err := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			adapter,
		)
		if err != nil {
			t.Fatal(err)
		}
		input := fixture.input()
		input.Generation.ClaimGeneration++
		outcome, executeErr := controller.Execute(context.Background(), input)
		if !errors.Is(executeErr, work.ErrStaleClaimGeneration) {
			t.Fatalf("stale Execute() error = %v", executeErr)
		}
		if outcome.Run().ID() != "" || adapter.executionCalled {
			t.Fatalf("stale execution produced authority: %#v / %#v", outcome, adapter)
		}
		assertManagedGrantRevoked(t, fixture, authorization.RevocationOperator)
	})

	t.Run("expired lease never starts and revokes safely", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "expired")
		fixture.clock.Set(managedTestNow.Add(6 * time.Minute))
		adapter := &fakeRuntimeAdapter{
			adapterType: fixture.instance.AdapterType,
			instanceID:  fixture.instance.ID,
		}
		controller, err := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			adapter,
		)
		if err != nil {
			t.Fatal(err)
		}
		outcome, executeErr := controller.Execute(context.Background(), fixture.input())
		if !errors.Is(executeErr, work.ErrRunLeaseExpired) ||
			outcome.Run().ID() != "" ||
			adapter.executionCalled {
			t.Fatalf("expired execution = %#v, %v, adapter=%#v", outcome, executeErr, adapter)
		}
		assertManagedGrantRevoked(t, fixture, authorization.RevocationOperator)
	})

	t.Run("offline runtime and adapter mismatch are validation-only", func(t *testing.T) {
		for _, testCase := range []struct {
			name    string
			mutate  func(*ExecuteInput)
			adapter *fakeRuntimeAdapter
			want    error
		}{
			{
				name: "offline runtime",
				mutate: func(input *ExecuteInput) {
					input.Instance.Status = loomruntime.RuntimeOffline
				},
				adapter: &fakeRuntimeAdapter{adapterType: "pi", instanceID: "runtime-1"},
				want:    loomruntime.ErrRuntimeOffline,
			},
			{
				name:    "adapter type mismatch",
				mutate:  func(*ExecuteInput) {},
				adapter: &fakeRuntimeAdapter{adapterType: "other", instanceID: "runtime-1"},
				want:    ErrInvalidManagedExecution,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newManagedExecutionFixture(t, "validation-"+strings.ReplaceAll(testCase.name, " ", "-"))
				controller, err := New(
					Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
					fixture.workAuthority,
					fixture.grantAuthority,
					testCase.adapter,
				)
				if err != nil {
					t.Fatal(err)
				}
				input := fixture.input()
				testCase.mutate(&input)
				outcome, executeErr := controller.Execute(context.Background(), input)
				if !errors.Is(executeErr, testCase.want) ||
					outcome.Run().ID() != "" ||
					testCase.adapter.executionCalled {
					t.Fatalf("validation outcome = %#v, %v, adapter=%#v", outcome, executeErr, testCase.adapter)
				}
				snapshot, snapshotErr := fixture.grantAuthority.Snapshot(context.Background())
				if snapshotErr != nil {
					t.Fatal(snapshotErr)
				}
				if grants := snapshot.Grants(); len(grants) != 1 ||
					grants[0].RevocationReason() != "" {
					t.Fatalf("validation mutated grant = %#v", grants)
				}
			})
		}
	})

	t.Run("configured workspace root replacement fails closed", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "root-replaced")
		adapter := &fakeRuntimeAdapter{
			adapterType: fixture.instance.AdapterType,
			instanceID:  fixture.instance.ID,
		}
		controller, err := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			adapter,
		)
		if err != nil {
			t.Fatal(err)
		}
		original := fixture.workspaceRoot + ".original"
		if err := os.Rename(fixture.workspaceRoot, original); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(fixture.workspaceRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		outcome, executeErr := controller.Execute(context.Background(), fixture.input())
		if !errors.Is(executeErr, ErrManagedWorkspace) ||
			adapter.executionCalled ||
			outcome.Run().TerminalReason() != "workspace_failed" {
			t.Fatalf("root replacement = %#v, %v, adapter=%#v", outcome, executeErr, adapter)
		}
		assertManagedGrantRevoked(t, fixture, authorization.RevocationTerminal)
	})

	t.Run("source mutation becomes failed terminal", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "source-change")
		result, err := NewAdapterResult(AdapterResultInput{
			InboundFrames:        managedInboundFrames(t, fixture, "succeeded", ""),
			ExitCode:             0,
			DispatchAcknowledged: true,
			ResultAcknowledged:   true,
		})
		if err != nil {
			t.Fatal(err)
		}
		adapter := &fakeRuntimeAdapter{
			adapterType: fixture.instance.AdapterType,
			instanceID:  fixture.instance.ID,
			result:      result,
			hook: func(AdapterRequest) error {
				return os.WriteFile(
					filepath.Join(fixture.sourcePath, "input.txt"),
					[]byte("mutated source\n"),
					0o600,
				)
			},
		}
		controller, newErr := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			adapter,
		)
		if newErr != nil {
			t.Fatal(newErr)
		}
		outcome, executeErr := controller.Execute(context.Background(), fixture.input())
		if !errors.Is(executeErr, ErrSourceChanged) {
			t.Fatalf("source changed Execute() error = %v", executeErr)
		}
		if outcome.Run().TerminalStatus() != "failed" ||
			outcome.Run().TerminalReason() != "source_changed" ||
			len(outcome.Changes()) != 0 {
			t.Fatalf("source changed outcome = %#v", outcome)
		}
		assertManagedGrantRevoked(t, fixture, authorization.RevocationTerminal)
	})

	t.Run("same-content source root replacement is detected", func(t *testing.T) {
		fixture := newManagedExecutionFixture(t, "source-root-change")
		result, err := NewAdapterResult(AdapterResultInput{
			InboundFrames:        managedInboundFrames(t, fixture, "succeeded", ""),
			ExitCode:             0,
			DispatchAcknowledged: true,
			ResultAcknowledged:   true,
		})
		if err != nil {
			t.Fatal(err)
		}
		adapter := &fakeRuntimeAdapter{
			adapterType: fixture.instance.AdapterType,
			instanceID:  fixture.instance.ID,
			result:      result,
			hook: func(AdapterRequest) error {
				original := fixture.sourcePath + ".original"
				if renameErr := os.Rename(fixture.sourcePath, original); renameErr != nil {
					return renameErr
				}
				if mkdirErr := os.Mkdir(fixture.sourcePath, 0o700); mkdirErr != nil {
					return mkdirErr
				}
				return os.WriteFile(
					filepath.Join(fixture.sourcePath, "input.txt"),
					[]byte("input\n"),
					0o600,
				)
			},
		}
		controller, newErr := New(
			Config{WorkspaceRoot: fixture.workspaceRoot, CleanupTimeout: time.Second},
			fixture.workAuthority,
			fixture.grantAuthority,
			adapter,
		)
		if newErr != nil {
			t.Fatal(newErr)
		}
		outcome, executeErr := controller.Execute(context.Background(), fixture.input())
		if !errors.Is(executeErr, ErrSourceChanged) ||
			outcome.Run().TerminalReason() != "source_changed" {
			t.Fatalf("source root replacement = %#v, %v", outcome, executeErr)
		}
		assertManagedGrantRevoked(t, fixture, authorization.RevocationTerminal)
	})
}

func newManagedExecutionFixture(t testing.TB, suffix string) *managedExecutionFixture {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	dsn := fmt.Sprintf(
		"file:%s/%s.db?%s",
		t.TempDir(),
		suffix,
		values.Encode(),
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	seedManagedRuntime(t, store)
	clock := &managedTestClock{now: managedTestNow}
	workAuthority, err := work.NewAuthority(
		store,
		clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x31}, 128)),
	)
	if err != nil {
		t.Fatal(err)
	}
	workItem, run, err := workAuthority.CreateAndAssign(
		context.Background(),
		work.WorkItemAssignmentInput{
			WorkItemID:      "S3-W4-" + suffix,
			Title:           "managed execution " + suffix,
			RunID:           "run-" + suffix,
			AgentInstanceID: "agent-1",
			CorrelationID:   "11111111-1111-4111-8111-111111111111",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	workItem, run, err = workAuthority.Claim(
		context.Background(),
		work.RunClaimInput{
			WorkItemID:           workItem.ID(),
			RunID:                run.ID(),
			RuntimeInstanceID:    "runtime-1",
			AgentInstanceID:      "agent-1",
			PrepareLeaseDuration: 5 * time.Minute,
			CorrelationID:        "11111111-1111-4111-8111-111111111111",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	grantAuthority, err := authorization.NewAuthority(
		store,
		workAuthority,
		clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x42}, 96)),
	)
	if err != nil {
		t.Fatal(err)
	}
	operations := []authorization.Operation{
		authorization.OperationBridgeAck,
		authorization.OperationBridgeEvent,
		authorization.OperationBridgeEvidence,
		authorization.OperationBridgeResult,
		authorization.OperationBridgeHeartbeat,
	}
	grant, err := grantAuthority.Issue(
		context.Background(),
		authorization.IssueInput{
			WorkItemID:        workItem.ID(),
			RunID:             run.ID(),
			ClaimID:           run.ClaimID(),
			ClaimGeneration:   run.ClaimGeneration(),
			RuntimeInstanceID: run.RuntimeInstanceID(),
			AgentInstanceID:   run.AgentInstanceID(),
			AllowedOperations: operations,
			Lifetime:          30 * time.Minute,
			CorrelationID:     "11111111-1111-4111-8111-111111111111",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:          "profile-1",
		AdapterType: "pi",
		AuthMode:    loomruntime.AuthBrokered,
		Timeout:     3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID:                "runtime-1",
		DeviceID:          "device-1",
		AdapterType:       "pi",
		DisplayName:       "Pi Fixture",
		ExecutableVersion: "1.0.0",
		Status:            loomruntime.RuntimeOnline,
		Capacity:          1,
	})
	if err != nil {
		t.Fatal(err)
	}
	generation := work.RunGenerationInput{
		WorkItemID:        workItem.ID(),
		RunID:             run.ID(),
		ClaimID:           run.ClaimID(),
		ClaimGeneration:   run.ClaimGeneration(),
		RuntimeInstanceID: run.RuntimeInstanceID(),
		AgentInstanceID:   run.AgentInstanceID(),
		CorrelationID:     "11111111-1111-4111-8111-111111111111",
	}
	dispatch := managedFrame(t, generation, 1, bridgev1.MessageDispatch, []byte(`{"task":"fixture"}`))
	sourcePath := privateDirectory(t, "source")
	writeManagedTestFile(t, filepath.Join(sourcePath, "input.txt"), []byte("input\n"), 0o600)
	return &managedExecutionFixture{
		store:          store,
		workAuthority:  workAuthority,
		grantAuthority: grantAuthority,
		workItem:       workItem,
		run:            run,
		grant:          grant,
		generation:     generation,
		profile:        profile,
		instance:       instance,
		dispatch:       dispatch,
		sourcePath:     sourcePath,
		workspaceRoot:  privateDirectory(t, "workspace-root"),
		clock:          clock,
		dsn:            dsn,
	}
}

func (fixture *managedExecutionFixture) input() ExecuteInput {
	return ExecuteInput{
		SourcePath: fixture.sourcePath,
		Profile:    fixture.profile,
		Instance:   fixture.instance,
		Generation: fixture.generation,
		Grant:      fixture.grant,
		Dispatch:   fixture.dispatch,
	}
}

func seedManagedRuntime(t testing.TB, store *journal.Store) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"discovery_digest": strings.Repeat("a", 64),
		"source_probe_id":  "probe-1",
		"instance": map[string]any{
			"id":                    "runtime-1",
			"device_id":             "device-1",
			"adapter_type":          "pi",
			"display_name":          "Pi Fixture",
			"executable_version":    "1.0.0",
			"status":                "online",
			"observed_capabilities": []string{},
			"capacity":              1,
		},
		"model_ids": []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID:             "runtime-discovered-runtime-1",
		StreamID:       "runtime_instance:runtime-1",
		Seq:            1,
		IdempotencyKey: "runtime-discovered-runtime-1",
		Type:           "RuntimeInstanceDiscovered",
		SchemaVersion:  1,
		EmittedAt:      managedTestNow.Add(-time.Minute),
		CorrelationID:  "22222222-2222-4222-8222-222222222222",
		PayloadJSON:    payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func managedInboundFrames(
	t testing.TB,
	fixture *managedExecutionFixture,
	status string,
	reason string,
) []bridgev1.Frame {
	t.Helper()
	ackPayload, err := json.Marshal(map[string]string{
		"message_id": fixture.dispatch.MessageID(),
	})
	if err != nil {
		t.Fatal(err)
	}
	resultPayload, err := json.Marshal(map[string]string{
		"status": status,
		"reason": reason,
	})
	if err != nil {
		t.Fatal(err)
	}
	return []bridgev1.Frame{
		managedFrame(t, fixture.generation, 2, bridgev1.MessageAck, ackPayload),
		managedFrame(t, fixture.generation, 3, bridgev1.MessageEvent, []byte(`{"event":"worked"}`)),
		managedFrame(t, fixture.generation, 4, bridgev1.MessageEvidence, []byte(`{"evidence":"bounded"}`)),
		managedFrame(t, fixture.generation, 5, bridgev1.MessageHeartbeat, []byte(`{"alive":true}`)),
		managedFrame(t, fixture.generation, 6, bridgev1.MessageResult, resultPayload),
	}
}

func managedFrame(
	t testing.TB,
	generation work.RunGenerationInput,
	sequence int64,
	messageType bridgev1.MessageType,
	payload []byte,
) bridgev1.Frame {
	t.Helper()
	frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: fmt.Sprintf(
			"00000000-0000-4000-8000-%012d",
			sequence,
		),
		CorrelationID:         generation.CorrelationID,
		WorkItemID:            generation.WorkItemID,
		RunID:                 generation.RunID,
		ClaimGeneration:       generation.ClaimGeneration,
		RuntimeInstanceID:     generation.RuntimeInstanceID,
		SenderAgentInstanceID: generation.AgentInstanceID,
		Sequence:              sequence,
		Type:                  messageType,
		EmittedAt:             managedTestNow.Add(time.Duration(sequence) * time.Millisecond),
		Payload:               payload,
	})
	if err != nil {
		t.Fatalf("NewFrame(%d, %s) error = %v", sequence, messageType, err)
	}
	return frame
}

func assertManagedGrantRevoked(
	t testing.TB,
	fixture *managedExecutionFixture,
	reason authorization.RevocationReason,
) {
	t.Helper()
	snapshot, err := fixture.grantAuthority.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	grants := snapshot.Grants()
	if len(grants) != 1 ||
		grants[0].ID() != fixture.grant.Record().ID() ||
		grants[0].RevocationReason() != reason ||
		grants[0].RevokedAt().IsZero() {
		t.Fatalf("grants = %#v, want revoked %q", grants, reason)
	}
}

func assertManagedReopenedTerminal(
	t testing.TB,
	fixture *managedExecutionFixture,
	wantStatus string,
	wantReason string,
	wantRevoke authorization.RevocationReason,
) {
	t.Helper()
	db, err := sql.Open("sqlite", fixture.dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := journal.NewStore(db)
	reopenedWork, err := work.NewAuthority(
		store,
		fixture.clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x71}, 64)),
	)
	if err != nil {
		t.Fatal(err)
	}
	workSnapshot, err := reopenedWork.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runs := workSnapshot.Runs()
	if len(runs) != 1 ||
		runs[0].TerminalStatus() != wantStatus ||
		runs[0].TerminalReason() != wantReason {
		t.Fatalf("reopened runs = %#v", runs)
	}
	reopenedGrant, err := authorization.NewAuthority(
		store,
		reopenedWork,
		fixture.clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x72}, 48)),
	)
	if err != nil {
		t.Fatal(err)
	}
	grantSnapshot, err := reopenedGrant.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	grants := grantSnapshot.Grants()
	if len(grants) != 1 || grants[0].RevocationReason() != wantRevoke {
		t.Fatalf("reopened grants = %#v", grants)
	}
}

func assertManagedAuthorizations(
	t testing.TB,
	fixture *managedExecutionFixture,
	frames []bridgev1.Frame,
) {
	t.Helper()
	events, err := fixture.store.ReadStream(
		context.Background(),
		"agent-grant/"+fixture.run.ID(),
	)
	if err != nil {
		t.Fatal(err)
	}
	authorized := 0
	for _, event := range events {
		if event.Type == "AgentGrantAuthorized" {
			authorized++
		}
	}
	if authorized != len(frames) {
		t.Fatalf("authorized frames = %d, want %d", authorized, len(frames))
	}
}

func assertManagedTokenAbsentFromJournal(
	t testing.TB,
	fixture *managedExecutionFixture,
) {
	t.Helper()
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	token := []byte(fixture.grant.Token().Value())
	for _, event := range events {
		assertManagedBytesAbsent(t, token, event.PayloadJSON, []byte(event.Type))
	}
}

func equalManagedStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
