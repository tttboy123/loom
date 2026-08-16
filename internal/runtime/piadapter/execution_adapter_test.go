package piadapter

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"

	_ "modernc.org/sqlite"
)

const piFastStderrBytes = 240 << 10

type piRecordingFrameSink struct {
	frames []bridgev1.Frame
	err    error
}

func (sink *piRecordingFrameSink) AcceptFrame(
	_ context.Context,
	frame bridgev1.Frame,
) error {
	sink.frames = append(sink.frames, frame)
	return sink.err
}

func TestPiExecutionAdapterConfigEnvironmentAndBinding(t *testing.T) { // s3_w4_adapter_config_env_binding
	t.Setenv("SHOULD_NOT_LEAK", "ambient-secret")
	fixture := newPiAdapterFixture(t, "success")
	adapter, err := NewPiExecutionAdapter(fixture.config())
	if err != nil {
		t.Fatalf("NewPiExecutionAdapter() error = %v", err)
	}
	if adapter.AdapterType() != "pi" ||
		adapter.RuntimeInstanceID() != fixture.binding.RuntimeInstanceID {
		t.Fatalf("adapter identity = %q/%q", adapter.AdapterType(), adapter.RuntimeInstanceID())
	}
	request := fixture.request(t)
	sink := request.FrameSink.(*piRecordingFrameSink)
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExitCode() != 0 ||
		!result.DispatchAcknowledged() ||
		!result.ResultAcknowledged() ||
		result.CancelAcknowledged() ||
		len(result.InboundFrames()) != 5 ||
		!validPiCoverageStderr(result.Stderr()) {
		t.Fatalf("adapter result = %#v", result)
	}
	if len(sink.frames) != len(result.InboundFrames()) ||
		!reflect.DeepEqual(sink.frames, result.InboundFrames()) {
		t.Fatalf(
			"incremental sink=%#v terminal batch=%#v",
			sink.frames,
			result.InboundFrames(),
		)
	}
	failingRequest := fixture.request(t)
	failingSink := failingRequest.FrameSink.(*piRecordingFrameSink)
	failingSink.err = errors.New("stop after first Frame")
	if failed, executeErr := adapter.Execute(
		context.Background(),
		failingRequest,
	); executeErr == nil ||
		len(failed.InboundFrames()) != 0 ||
		len(failingSink.frames) != 1 {
		t.Fatalf(
			"sink failure result=%#v frames=%d error=%v",
			failed,
			len(failingSink.frames),
			executeErr,
		)
	}
	arguments := fixture.arguments
	arguments[0] = "mutated"
	searchPaths := fixture.searchPaths
	searchPaths[0] = "mutated"
	second, err := adapter.Execute(context.Background(), fixture.request(t))
	if err != nil || second.ExitCode() != 0 {
		t.Fatalf("adapter aliased config: %#v, %v", second, err)
	}
	drifted := fixture.request(t)
	drifted.ExecutionBinding.ModelID = "silent-model-drift"
	if result, executeErr := adapter.Execute(
		context.Background(),
		drifted,
	); !errors.Is(executeErr, ErrPiExecutionBindingChanged) ||
		len(result.InboundFrames()) != 0 {
		t.Fatalf("drifted execution binding = %#v, %v", result, executeErr)
	}
	fastExit, err := adapter.Execute(
		context.Background(),
		fixture.requestForMode(t, "fast-stderr"),
	)
	if err != nil ||
		fastExit.ExitCode() != 0 ||
		!fastExit.DispatchAcknowledged() ||
		!fastExit.ResultAcknowledged() ||
		!validPiFastStderr(fastExit.Stderr()) {
		t.Fatalf("fast-exit stderr result = %#v, %v", fastExit, err)
	}

	invalid := fixture.config()
	invalid.ExecutablePath = "relative"
	if candidate, invalidErr := NewPiExecutionAdapter(invalid); !errors.Is(
		invalidErr,
		ErrInvalidPiExecutionAdapter,
	) || candidate != nil {
		t.Fatalf("relative adapter = %#v, %v", candidate, invalidErr)
	}
	symlinkSearch := filepath.Join(t.TempDir(), "search-link")
	if err := os.Symlink(fixture.searchPaths[0], symlinkSearch); err != nil {
		t.Fatal(err)
	}
	invalidCases := []struct {
		name   string
		mutate func(*PiExecutionAdapterConfig)
	}{
		{"nil random", func(config *PiExecutionAdapterConfig) { config.Random = nil }},
		{"zero grace", func(config *PiExecutionAdapterConfig) { config.CancelGrace = 0 }},
		{"long grace", func(config *PiExecutionAdapterConfig) { config.CancelGrace = 6 * time.Second }},
		{"non UTC clock", func(config *PiExecutionAdapterConfig) {
			config.Now = func() time.Time { return managedPiTestNow.In(time.FixedZone("fixture", 3600)) }
		}},
		{"invalid runtime ID", func(config *PiExecutionAdapterConfig) {
			config.RuntimeInstanceID = " runtime-1"
		}},
		{"empty search path", func(config *PiExecutionAdapterConfig) {
			config.RuntimeSearchPaths = nil
		}},
		{"relative search path", func(config *PiExecutionAdapterConfig) {
			config.RuntimeSearchPaths = []string{"relative"}
		}},
		{"symlink search path", func(config *PiExecutionAdapterConfig) {
			config.RuntimeSearchPaths = []string{symlinkSearch}
		}},
		{"NUL argument", func(config *PiExecutionAdapterConfig) {
			config.Arguments = []string{"bad\x00argument"}
		}},
	}
	for _, testCase := range invalidCases {
		t.Run("rejects "+testCase.name, func(t *testing.T) {
			candidateConfig := fixture.config()
			testCase.mutate(&candidateConfig)
			candidate, candidateErr := NewPiExecutionAdapter(candidateConfig)
			if !errors.Is(candidateErr, ErrInvalidPiExecutionAdapter) || candidate != nil {
				t.Fatalf("candidate = %#v, %v", candidate, candidateErr)
			}
		})
	}

	t.Run("executable identity is revalidated", func(t *testing.T) {
		mutable := newPiAdapterFixture(t, "success")
		bound, bindErr := NewPiExecutionAdapter(mutable.config())
		if bindErr != nil {
			t.Fatal(bindErr)
		}
		if err := os.Chmod(mutable.executablePath, 0o600); err != nil {
			t.Fatal(err)
		}
		if result, executeErr := bound.Execute(
			context.Background(),
			mutable.request(t),
		); !errors.Is(executeErr, ErrPiExecutionBindingChanged) ||
			len(result.InboundFrames()) != 0 {
			t.Fatalf("mutated executable = %#v, %v", result, executeErr)
		}
	})

	t.Run("search directory identity is revalidated", func(t *testing.T) {
		mutable := newPiAdapterFixture(t, "success")
		bound, bindErr := NewPiExecutionAdapter(mutable.config())
		if bindErr != nil {
			t.Fatal(bindErr)
		}
		original := mutable.searchPaths[0] + ".original"
		if err := os.Rename(mutable.searchPaths[0], original); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(mutable.searchPaths[0], 0o700); err != nil {
			t.Fatal(err)
		}
		if result, executeErr := bound.Execute(
			context.Background(),
			mutable.request(t),
		); !errors.Is(executeErr, ErrPiExecutionBindingChanged) ||
			len(result.InboundFrames()) != 0 {
			t.Fatalf("mutated search path = %#v, %v", result, executeErr)
		}
	})
}

func TestPiExecutionThroughSupervisorReopensExactTerminal(t *testing.T) {
	process := newPiAdapterFixture(t, "supervisor-success")
	adapter, err := NewPiExecutionAdapter(process.config())
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name           string
		mode           string
		wantWorkStatus string
		wantRunStatus  string
		wantRunReason  string
	}{
		{
			name:           "succeeded",
			mode:           "supervisor-success",
			wantWorkStatus: "ready_for_review",
			wantRunStatus:  "succeeded",
			wantRunReason:  "",
		},
		{
			name:           "failed",
			mode:           "supervisor-failed",
			wantWorkStatus: "failed",
			wantRunStatus:  "failed",
			wantRunReason:  "fixture_failure",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newPiSupervisorFixture(t)
			controller, err := supervisor.New(
				supervisor.Config{
					WorkspaceRoot:  fixture.workspaceRoot,
					CleanupTimeout: time.Second,
				},
				fixture.workAuthority,
				fixture.grantAuthority,
				adapter,
			)
			if err != nil {
				t.Fatal(err)
			}
			outcome, err := controller.Execute(
				context.Background(),
				fixture.input(
					process.binding,
					process.dispatchForMode(t, testCase.mode),
				),
			)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if outcome.WorkItem().Status() != testCase.wantWorkStatus ||
				outcome.Run().TerminalStatus() != testCase.wantRunStatus ||
				outcome.Run().TerminalReason() != testCase.wantRunReason ||
				!outcome.Stream().TerminalResultSeen() ||
				len(outcome.Stream().Frames()) != 5 {
				t.Fatalf("live outcome = %#v", outcome)
			}
			assertPiReopenedTerminal(
				t,
				fixture,
				testCase.wantWorkStatus,
				testCase.wantRunStatus,
				testCase.wantRunReason,
			)
		})
	}
}

func TestPiExecutionBridgeDispatchAckAndResult(t *testing.T) { // s3_w4_bridge_dispatch_ack_result
	fixture := newPiAdapterFixture(t, "success")
	adapter, err := NewPiExecutionAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{
		"malformed",
		"wrong-binding",
		"nonzero",
		"no-ack",
		"stderr-token",
		"stdout-token",
		"oversized-stdout",
		"missing-result",
		"out-of-order",
		"after-result",
		"second-result",
		"inbound-cancel",
	} {
		t.Run(mode, func(t *testing.T) {
			result, executeErr := adapter.Execute(
				context.Background(),
				fixture.requestForMode(t, mode),
			)
			if executeErr == nil {
				t.Fatalf("%s Execute() unexpectedly succeeded: %#v", mode, result)
			}
			if mode == "nonzero" {
				if !errors.Is(executeErr, ErrPiExecutionProcess) {
					t.Fatalf("nonzero error = %v", executeErr)
				}
			} else if mode == "oversized-stdout" {
				if !errors.Is(executeErr, ErrPiExecutionOutputTooLarge) {
					t.Fatalf("oversized stdout error = %v", executeErr)
				}
			} else if !errors.Is(executeErr, ErrPiExecutionProtocol) {
				t.Fatalf("%s error = %v", mode, executeErr)
			}
			if len(result.InboundFrames()) != 0 ||
				len(result.Stderr()) != 0 ||
				result.DispatchAcknowledged() ||
				result.ResultAcknowledged() {
				t.Fatalf("%s leaked partial result = %#v", mode, result)
			}
		})
	}
}

func TestPiExecutionCancellationTimeoutAndProcessGroupCleanup(t *testing.T) { // s3_w4_cancel_timeout_process_group
	if runtime.GOOS == "windows" {
		t.Skip("process-group proof is Unix-only")
	}
	fixture := newPiAdapterFixture(t, "wait-cancel")
	fixture.cancelGrace = 100 * time.Millisecond
	adapter, err := NewPiExecutionAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type executeAnswer struct {
		result supervisor.AdapterResult
		err    error
	}
	answer := make(chan executeAnswer, 1)
	go func() {
		result, executeErr := adapter.Execute(
			ctx,
			fixture.requestForMode(t, "wait-cancel"),
		)
		answer <- executeAnswer{result: result, err: executeErr}
	}()
	pidPath := filepath.Join(fixture.workspacePath, "grandchild.pid")
	readinessDeadline := time.NewTimer(10 * time.Second)
	defer readinessDeadline.Stop()
	readinessPoll := time.NewTicker(10 * time.Millisecond)
	defer readinessPoll.Stop()
	var grandchildPID int
	var lastPIDBytes []byte
	var lastPIDError error
readiness:
	for {
		select {
		case early := <-answer:
			t.Fatalf(
				"cancel helper exited before grandchild readiness: %#v, %v",
				early.result,
				early.err,
			)
		case <-readinessPoll.C:
			pidBytes, readErr := os.ReadFile(pidPath)
			if readErr == nil {
				parsedPID, parseErr := strconv.Atoi(
					strings.TrimSpace(string(pidBytes)),
				)
				lastPIDBytes = bytes.Clone(pidBytes)
				lastPIDError = parseErr
				if parseErr == nil && parsedPID > 0 {
					grandchildPID = parsedPID
					break readiness
				}
				continue
			}
			if !errors.Is(readErr, os.ErrNotExist) {
				cancel()
				select {
				case completed := <-answer:
					t.Fatalf(
						"grandchild PID readiness read failed: %v; cleanup=%#v, %v",
						readErr,
						completed.result,
						completed.err,
					)
				case <-time.After(5 * time.Second):
					t.Fatalf(
						"grandchild PID readiness read and cancellation timed out: %v",
						readErr,
					)
				}
			}
		case <-readinessDeadline.C:
			cancel()
			select {
			case early := <-answer:
				t.Fatalf(
					"grandchild PID readiness timed out: bytes=%q parse=%v; cleanup=%#v, %v",
					lastPIDBytes,
					lastPIDError,
					early.result,
					early.err,
				)
			case <-time.After(5 * time.Second):
				t.Fatal("grandchild readiness and cancellation both timed out")
			}
		}
	}
	cancel()
	cancelCompletion := time.NewTimer(5 * time.Second)
	defer cancelCompletion.Stop()
	var result supervisor.AdapterResult
	var executeErr error
	select {
	case completed := <-answer:
		result = completed.result
		executeErr = completed.err
	case <-cancelCompletion.C:
		t.Fatal("cancelled adapter did not complete")
	}
	if !errors.Is(executeErr, context.Canceled) ||
		!errors.Is(executeErr, ErrPiExecutionProcess) {
		t.Fatalf("cancel Execute() error = %v", executeErr)
	}
	if result.ExitCode() != 0 ||
		len(result.InboundFrames()) != 0 ||
		len(result.Stderr()) != 0 {
		t.Fatalf("cancel leaked partial result = %#v", result)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if !executionProcessExists(grandchildPID) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("grandchild %d remains after process-group cleanup", grandchildPID)
		}
		time.Sleep(10 * time.Millisecond)
	}

	afterResultContext, cancelAfterResult := context.WithTimeout(
		context.Background(),
		750*time.Millisecond,
	)
	defer cancelAfterResult()
	if result, afterResultErr := adapter.Execute(
		afterResultContext,
		fixture.requestForMode(t, "wait-after-result"),
	); !errors.Is(afterResultErr, context.DeadlineExceeded) ||
		!errors.Is(afterResultErr, ErrPiExecutionProcess) ||
		len(result.InboundFrames()) != 0 {
		t.Fatalf("after-result cancellation = %#v, %v", result, afterResultErr)
	}

	if result, overflowErr := adapter.Execute(
		context.Background(),
		fixture.requestForMode(t, "stderr-overflow"),
	); !errors.Is(overflowErr, ErrPiExecutionOutputTooLarge) ||
		len(result.Stderr()) != 0 {
		t.Fatalf("overflow result = %#v, %v", result, overflowErr)
	}
}

func TestPiExecutionHelperProcess(t *testing.T) {
	var helperArguments []string
	for index, value := range os.Args {
		if value == "--" && index+1 < len(os.Args) {
			helperArguments = os.Args[index+1:]
			break
		}
	}
	if len(helperArguments) < 5 {
		return
	}
	argumentMode := helperArguments[0]
	if argumentMode == "grandchild" {
		blockPiHelper()
	}
	if argumentMode != "bridge" {
		os.Exit(20)
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		os.Exit(21)
	}
	dispatch, err := bridgev1.DecodeLine(line)
	if err != nil || dispatch.Type() != bridgev1.MessageDispatch {
		os.Exit(22)
	}
	var dispatchPayload struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(dispatch.Payload(), &dispatchPayload); err != nil ||
		dispatchPayload.Mode == "" {
		os.Exit(28)
	}
	mode := dispatchPayload.Mode
	wantEnvironment := []string{
		"HOME=" + helperArguments[1],
		"TMPDIR=" + helperArguments[2],
		"PATH=" + helperArguments[3],
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"LOOM_AGENT_GRANT=" + piTestTokenValue,
	}
	currentDirectory, cwdErr := os.Getwd()
	currentInfo, currentInfoErr := os.Stat(currentDirectory)
	expectedInfo, expectedInfoErr := os.Stat(helperArguments[4])
	if strings.HasPrefix(mode, "supervisor-") {
		if len(os.Environ()) != 6 ||
			cwdErr != nil ||
			currentInfoErr != nil ||
			os.Getenv("HOME") == "" ||
			os.Getenv("TMPDIR") == "" ||
			os.Getenv("PATH") == "" ||
			os.Getenv("LANG") != "C.UTF-8" ||
			os.Getenv("LC_ALL") != "C.UTF-8" ||
			os.Getenv("LOOM_AGENT_GRANT") != piTestTokenValue ||
			os.Getenv("SHOULD_NOT_LEAK") != "" {
			os.Exit(23)
		}
	} else if !reflect.DeepEqual(os.Environ(), wantEnvironment) ||
		cwdErr != nil ||
		currentInfoErr != nil ||
		expectedInfoErr != nil ||
		!os.SameFile(currentInfo, expectedInfo) ||
		os.Getenv("SHOULD_NOT_LEAK") != "" {
		os.Exit(23)
	}
	if mode == "stderr-overflow" {
		_, _ = os.Stderr.Write(bytes.Repeat([]byte("x"), (256<<10)+1))
		os.Exit(0)
	}
	if mode == "fast-stderr" {
		_, _ = os.Stderr.Write(bytes.Repeat([]byte("s"), piFastStderrBytes))
	}
	if mode == "wait-cancel" {
		command := exec.Command(
			os.Args[0],
			"-test.run=^TestPiExecutionHelperProcess$",
			"--",
			"grandchild",
			helperArguments[1],
			helperArguments[2],
			helperArguments[3],
			helperArguments[4],
		)
		command.Env = os.Environ()
		command.Dir = helperArguments[4]
		if err := command.Start(); err != nil {
			os.Exit(26)
		}
		if err := publishPiGrandchildPID(
			helperArguments[4],
			command.Process.Pid,
		); err != nil {
			os.Exit(27)
		}
		_, _ = reader.ReadBytes('\n')
		blockPiHelper()
	}
	if mode == "malformed" {
		_, _ = os.Stdout.WriteString("{malformed}\n")
		os.Exit(0)
	}
	if mode == "oversized-stdout" {
		_, _ = os.Stdout.Write(bytes.Repeat([]byte("x"), bridgev1.MaxLineBytes+1))
		_, _ = os.Stdout.WriteString("\n")
		os.Exit(0)
	}
	binding := bridgev1.RunStreamBinding{
		WorkItemID:            dispatch.WorkItemID(),
		RunID:                 dispatch.RunID(),
		ClaimGeneration:       dispatch.ClaimGeneration(),
		RuntimeInstanceID:     dispatch.RuntimeInstanceID(),
		SenderAgentInstanceID: dispatch.SenderAgentInstanceID(),
	}
	if mode == "wrong-binding" {
		binding.RunID = "wrong-run"
	}
	ack := piHelperFrame(
		binding,
		dispatch.CorrelationID(),
		2,
		bridgev1.MessageAck,
		mustPiJSON(map[string]string{
			"message_id": dispatch.MessageID(),
		}),
	)
	if mode != "no-ack" {
		writePiFrame(ack)
	}
	if mode == "missing-result" {
		os.Exit(0)
	}
	if mode == "out-of-order" {
		writePiFrame(piHelperFrame(
			binding,
			dispatch.CorrelationID(),
			2,
			bridgev1.MessageEvent,
			[]byte(`{"event":"out-of-order"}`),
		))
		blockPiHelper()
	}
	if mode == "inbound-cancel" {
		writePiFrame(piHelperFrame(
			binding,
			dispatch.CorrelationID(),
			3,
			bridgev1.MessageCancel,
			[]byte(`{"reason":"cancelled"}`),
		))
		blockPiHelper()
	}
	if mode == "stderr-token" {
		_, _ = os.Stderr.WriteString(piTestTokenValue)
	}
	resultSequence := int64(6)
	if mode == "stdout-token" {
		event := piHelperFrame(
			binding,
			dispatch.CorrelationID(),
			3,
			bridgev1.MessageEvent,
			mustPiJSON(map[string]string{"value": piTestTokenValue}),
		)
		writePiFrame(event)
		resultSequence = 4
	} else {
		writePiFrame(piHelperFrame(
			binding,
			dispatch.CorrelationID(),
			3,
			bridgev1.MessageEvent,
			[]byte(`{"event":"worked"}`),
		))
		writePiFrame(piHelperFrame(
			binding,
			dispatch.CorrelationID(),
			4,
			bridgev1.MessageEvidence,
			[]byte(`{"evidence":"bounded"}`),
		))
		writePiFrame(piHelperFrame(
			binding,
			dispatch.CorrelationID(),
			5,
			bridgev1.MessageHeartbeat,
			[]byte(`{"alive":true}`),
		))
	}
	resultPayload := []byte(`{"status":"succeeded","reason":""}`)
	if mode == "supervisor-failed" {
		resultPayload = []byte(`{"status":"failed","reason":"fixture_failure"}`)
	}
	result := piHelperFrame(
		binding,
		dispatch.CorrelationID(),
		resultSequence,
		bridgev1.MessageResult,
		resultPayload,
	)
	writePiFrame(result)
	if mode == "after-result" {
		writePiFrame(piHelperFrame(
			binding,
			dispatch.CorrelationID(),
			resultSequence+1,
			bridgev1.MessageEvent,
			[]byte(`{"event":"too-late"}`),
		))
	}
	if mode == "second-result" {
		writePiFrame(piHelperFrame(
			binding,
			dispatch.CorrelationID(),
			resultSequence+1,
			bridgev1.MessageResult,
			[]byte(`{"status":"failed","reason":"second"}`),
		))
	}
	ackLine, err := reader.ReadBytes('\n')
	if err != nil {
		os.Exit(24)
	}
	resultAck, err := bridgev1.DecodeLine(ackLine)
	if err != nil ||
		resultAck.Type() != bridgev1.MessageAck ||
		!bytes.Contains(resultAck.Payload(), []byte(result.MessageID())) {
		os.Exit(25)
	}
	if mode == "nonzero" {
		os.Exit(7)
	}
	if mode == "wait-after-result" {
		blockPiHelper()
	}
	os.Exit(0)
}

func publishPiGrandchildPID(directory string, pid int) error {
	file, err := os.CreateTemp(directory, ".grandchild-pid-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	content := []byte(strconv.Itoa(pid))
	written, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return errors.Join(writeErr, closeErr)
	}
	if written != len(content) {
		return fmt.Errorf("grandchild PID short write: %d of %d", written, len(content))
	}
	return os.Rename(tempPath, filepath.Join(directory, "grandchild.pid"))
}

type piAdapterFixture struct {
	executablePath string
	arguments      []string
	searchPaths    []string
	cancelGrace    time.Duration
	workspacePath  string
	homePath       string
	tempPath       string
	binding        bridgev1.RunStreamBinding
	dispatch       bridgev1.Frame
	token          authorization.Token
}

type piSupervisorFixture struct {
	dsn            string
	workAuthority  *work.Authority
	grantAuthority *authorization.Authority
	grant          authorization.IssuedGrant
	generation     work.RunGenerationInput
	profile        loomruntime.RuntimeProfile
	instance       loomruntime.RuntimeInstance
	sourcePath     string
	workspaceRoot  string
}

func newPiSupervisorFixture(t testing.TB) *piSupervisorFixture {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	dsn := fmt.Sprintf("file:%s/supervisor.db?%s", t.TempDir(), values.Encode())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	seedPiSupervisorRuntime(t, store)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:          "profile-1",
		AdapterType: "pi",
		ProviderID:  "loom-local",
		ModelID:     "fixture-model",
		AuthMode:    loomruntime.AuthNative,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID:                "runtime-1",
		DeviceID:          "device-1",
		AdapterType:       "pi",
		DisplayName:       "Pi Integration",
		ExecutableVersion: "1.0.0",
		Status:            loomruntime.RuntimeOnline,
		Capacity:          1,
	})
	if err != nil {
		t.Fatal(err)
	}
	executionBinding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	workAuthority, err := work.NewAuthority(
		store,
		func() time.Time { return managedPiTestNow },
		bytes.NewReader(bytes.Repeat([]byte{0x31}, 128)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	workItem, run, err := workAuthority.CreateAndAssign(
		context.Background(),
		work.WorkItemAssignmentInput{
			WorkItemID:       "S3-W4",
			Title:            "real adapter integration",
			RunID:            "run-1",
			AgentInstanceID:  "agent-1",
			ExecutionBinding: executionBinding,
			CorrelationID:    "22222222-2222-4222-8222-222222222222",
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
			CorrelationID:        "22222222-2222-4222-8222-222222222222",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	grantMaterial := bytes.Repeat([]byte{0x11}, 48)
	grantMaterial[8] = 0x01
	for index := 16; index < len(grantMaterial); index++ {
		grantMaterial[index] = 0
	}
	grantAuthority, err := authorization.NewAuthority(
		store,
		workAuthority,
		func() time.Time { return managedPiTestNow },
		bytes.NewReader(grantMaterial),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(context.Background()); err != nil {
		t.Fatal(err)
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
			AllowedOperations: []authorization.Operation{
				authorization.OperationBridgeAck,
				authorization.OperationBridgeEvent,
				authorization.OperationBridgeEvidence,
				authorization.OperationBridgeHeartbeat,
				authorization.OperationBridgeResult,
			},
			Lifetime:      30 * time.Minute,
			CorrelationID: "22222222-2222-4222-8222-222222222222",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if grant.Token().Value() != piTestTokenValue {
		t.Fatalf("integration token = %q", grant.Token().Value())
	}
	sourcePath := piPrivateDirectory(t, "supervisor-source")
	if err := os.WriteFile(
		filepath.Join(sourcePath, "input.txt"),
		[]byte("integration\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	return &piSupervisorFixture{
		dsn:            dsn,
		workAuthority:  workAuthority,
		grantAuthority: grantAuthority,
		grant:          grant,
		generation: work.RunGenerationInput{
			WorkItemID:        workItem.ID(),
			RunID:             run.ID(),
			ClaimID:           run.ClaimID(),
			ClaimGeneration:   run.ClaimGeneration(),
			RuntimeInstanceID: run.RuntimeInstanceID(),
			AgentInstanceID:   run.AgentInstanceID(),
			CorrelationID:     "22222222-2222-4222-8222-222222222222",
		},
		profile:       profile,
		instance:      instance,
		sourcePath:    sourcePath,
		workspaceRoot: piPrivateDirectory(t, "supervisor-workspace-root"),
	}
}

func (fixture *piSupervisorFixture) input(
	binding bridgev1.RunStreamBinding,
	dispatch bridgev1.Frame,
) supervisor.ExecuteInput {
	if binding.WorkItemID != fixture.generation.WorkItemID ||
		binding.RunID != fixture.generation.RunID ||
		binding.ClaimGeneration != fixture.generation.ClaimGeneration ||
		binding.RuntimeInstanceID != fixture.generation.RuntimeInstanceID ||
		binding.SenderAgentInstanceID != fixture.generation.AgentInstanceID {
		panic("test fixture binding mismatch")
	}
	return supervisor.ExecuteInput{
		SourcePath: fixture.sourcePath,
		Profile:    fixture.profile,
		Instance:   fixture.instance,
		Generation: fixture.generation,
		Grant:      fixture.grant,
		Dispatch:   dispatch,
	}
}

func seedPiSupervisorRuntime(t testing.TB, store *journal.Store) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"discovery_digest": strings.Repeat("a", 64),
		"source_probe_id":  "probe-1",
		"instance": map[string]any{
			"id":                    "runtime-1",
			"device_id":             "device-1",
			"adapter_type":          "pi",
			"display_name":          "Pi Integration",
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
		EmittedAt:      managedPiTestNow.Add(-time.Minute),
		CorrelationID:  "22222222-2222-4222-8222-222222222222",
		PayloadJSON:    payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func assertPiReopenedTerminal(
	t testing.TB,
	fixture *piSupervisorFixture,
	wantWorkStatus string,
	wantRunStatus string,
	wantRunReason string,
) {
	t.Helper()
	db, err := sql.Open("sqlite", fixture.dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	reopenedWork, err := work.NewAuthority(
		store,
		func() time.Time { return managedPiTestNow },
		bytes.NewReader(bytes.Repeat([]byte{0x51}, 64)),
	)
	if err != nil {
		t.Fatal(err)
	}
	workSnapshot, err := reopenedWork.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	workItems := workSnapshot.WorkItems()
	runs := workSnapshot.Runs()
	if len(workItems) != 1 ||
		len(runs) != 1 ||
		workItems[0].Status() != wantWorkStatus ||
		runs[0].TerminalStatus() != wantRunStatus ||
		runs[0].TerminalReason() != wantRunReason {
		t.Fatalf("reopened work = %#v / %#v", workItems, runs)
	}
	reopenedGrant, err := authorization.NewAuthority(
		store,
		reopenedWork,
		func() time.Time { return managedPiTestNow },
		bytes.NewReader(bytes.Repeat([]byte{0x61}, 48)),
	)
	if err != nil {
		t.Fatal(err)
	}
	grantSnapshot, err := reopenedGrant.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	grants := grantSnapshot.Grants()
	if len(grants) != 1 ||
		grants[0].RevocationReason() != authorization.RevocationTerminal ||
		grants[0].RevokedAt().IsZero() {
		t.Fatalf("reopened grants = %#v", grants)
	}
	var payloads string
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT COALESCE(GROUP_CONCAT(payload_json, ''), '') FROM events`,
	).Scan(&payloads); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payloads, fixture.grant.Token().Value()) {
		t.Fatal("raw grant entered reopened SQLite")
	}
}

func newPiAdapterFixture(t testing.TB, mode string) *piAdapterFixture {
	t.Helper()
	originalExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	originalExecutable, err = filepath.EvalSymlinks(originalExecutable)
	if err != nil {
		t.Fatal(err)
	}
	root := piPrivateDirectory(t, "execution")
	executablePath := filepath.Join(root, "fixture-executable")
	executableBytes, err := os.ReadFile(originalExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executablePath, executableBytes, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(executablePath, 0o700); err != nil {
		t.Fatal(err)
	}
	workspacePath := piPrivateDirectoryAt(t, root, "workspace")
	homePath := piPrivateDirectoryAt(t, root, "home")
	tempPath := piPrivateDirectoryAt(t, root, "tmp")
	searchPath := piPrivateDirectoryAt(t, root, "search")
	binding := bridgev1.RunStreamBinding{
		WorkItemID:            "S3-W4",
		RunID:                 "run-1",
		ClaimGeneration:       1,
		RuntimeInstanceID:     "runtime-1",
		SenderAgentInstanceID: "agent-1",
	}
	dispatch := piTestFrame(
		t,
		binding,
		1,
		bridgev1.MessageDispatch,
		mustPiJSON(map[string]string{"mode": mode}),
	)
	token, err := authorization.ParseToken(
		piTestTokenValue,
	)
	if err != nil {
		t.Fatal(err)
	}
	return &piAdapterFixture{
		executablePath: executablePath,
		arguments: []string{
			"-test.run=^TestPiExecutionHelperProcess$",
			"--",
			"bridge",
			homePath,
			tempPath,
			searchPath,
			workspacePath,
		},
		searchPaths:   []string{searchPath},
		cancelGrace:   200 * time.Millisecond,
		workspacePath: workspacePath,
		homePath:      homePath,
		tempPath:      tempPath,
		binding:       binding,
		dispatch:      dispatch,
		token:         token,
	}
}

func (fixture *piAdapterFixture) config() PiExecutionAdapterConfig {
	return PiExecutionAdapterConfig{
		ExecutablePath:    fixture.executablePath,
		Arguments:         append([]string(nil), fixture.arguments...),
		RuntimeInstanceID: fixture.binding.RuntimeInstanceID,
		RuntimeSearchPaths: append(
			[]string(nil),
			fixture.searchPaths...,
		),
		CancelGrace: fixture.cancelGrace,
		Now:         func() time.Time { return managedPiTestNow },
		Random:      bytes.NewReader(bytes.Repeat([]byte{0x35}, 512)),
	}
}

func (fixture *piAdapterFixture) request(t testing.TB) supervisor.AdapterRequest {
	t.Helper()
	return supervisor.AdapterRequest{
		WorkspacePath: fixture.workspacePath,
		HomePath:      fixture.homePath,
		TempPath:      fixture.tempPath,
		Binding:       fixture.binding,
		ExecutionBinding: piFixtureExecutionBinding(
			t,
			"pi",
			"loom-local",
			"fixture-model",
			fixture.binding.RuntimeInstanceID,
		),
		Dispatch:  fixture.dispatch,
		Grant:     fixture.token,
		FrameSink: &piRecordingFrameSink{},
	}
}

func piFixtureExecutionBinding(
	t testing.TB,
	adapterType,
	providerID,
	modelID,
	runtimeInstanceID string,
) loomruntime.FrozenExecutionBinding {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:          "profile-" + adapterType,
		AdapterType: adapterType,
		ProviderID:  providerID,
		ModelID:     modelID,
		AuthMode:    loomruntime.AuthNative,
		Timeout:     time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID:                runtimeInstanceID,
		DeviceID:          "device-fixture",
		AdapterType:       adapterType,
		DisplayName:       "Pi fixture",
		ExecutableVersion: "1.0.0",
		Status:            loomruntime.RuntimeOnline,
		Capacity:          1,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func (fixture *piAdapterFixture) requestForMode(
	t testing.TB,
	mode string,
) supervisor.AdapterRequest {
	t.Helper()
	request := fixture.request(t)
	request.Dispatch = fixture.dispatchForMode(t, mode)
	return request
}

func (fixture *piAdapterFixture) dispatchForMode(
	t testing.TB,
	mode string,
) bridgev1.Frame {
	t.Helper()
	return piTestFrame(
		t,
		fixture.binding,
		1,
		bridgev1.MessageDispatch,
		mustPiJSON(map[string]string{"mode": mode}),
	)
}

var managedPiTestNow = time.Date(2026, 7, 26, 9, 10, 11, 0, time.UTC)

var piTestTokenValue = "loom_grant_v1.11111111-1111-4111-8111-111111111111." +
	strings.Repeat("A", 43)

func piTestFrame(
	t testing.TB,
	binding bridgev1.RunStreamBinding,
	sequence int64,
	messageType bridgev1.MessageType,
	payload []byte,
) bridgev1.Frame {
	t.Helper()
	frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: fmt.Sprintf(
			"10000000-0000-4000-8000-%012d",
			sequence,
		),
		CorrelationID:         "22222222-2222-4222-8222-222222222222",
		WorkItemID:            binding.WorkItemID,
		RunID:                 binding.RunID,
		ClaimGeneration:       binding.ClaimGeneration,
		RuntimeInstanceID:     binding.RuntimeInstanceID,
		SenderAgentInstanceID: binding.SenderAgentInstanceID,
		Sequence:              sequence,
		Type:                  messageType,
		EmittedAt:             managedPiTestNow.Add(time.Duration(sequence) * time.Millisecond),
		Payload:               payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func piHelperFrame(
	binding bridgev1.RunStreamBinding,
	correlationID string,
	sequence int64,
	messageType bridgev1.MessageType,
	payload []byte,
) bridgev1.Frame {
	frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: fmt.Sprintf(
			"20000000-0000-4000-8000-%012d",
			sequence,
		),
		CorrelationID:         correlationID,
		WorkItemID:            binding.WorkItemID,
		RunID:                 binding.RunID,
		ClaimGeneration:       binding.ClaimGeneration,
		RuntimeInstanceID:     binding.RuntimeInstanceID,
		SenderAgentInstanceID: binding.SenderAgentInstanceID,
		Sequence:              sequence,
		Type:                  messageType,
		EmittedAt:             managedPiTestNow.Add(time.Duration(sequence) * time.Millisecond),
		Payload:               payload,
	})
	if err != nil {
		os.Exit(31)
	}
	return frame
}

func writePiFrame(frame bridgev1.Frame) {
	line, err := bridgev1.EncodeLine(frame)
	if err != nil {
		os.Exit(32)
	}
	if _, err := os.Stdout.Write(line); err != nil {
		os.Exit(33)
	}
}

func mustPiJSON(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		os.Exit(34)
	}
	return payload
}

func blockPiHelper() {
	for {
		time.Sleep(time.Hour)
	}
}

func validPiCoverageStderr(stderr []byte) bool {
	return len(stderr) == 0 ||
		string(stderr) == "warning: GOCOVERDIR not set, no coverage data emitted\n"
}

func validPiFastStderr(stderr []byte) bool {
	expected := bytes.Repeat([]byte("s"), piFastStderrBytes)
	if bytes.Equal(stderr, expected) {
		return true
	}
	return bytes.Equal(
		stderr,
		append(
			expected,
			[]byte("warning: GOCOVERDIR not set, no coverage data emitted\n")...,
		),
	)
}

func piPrivateDirectory(t testing.TB, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func piPrivateDirectoryAt(t testing.TB, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
