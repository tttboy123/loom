package app

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/state"
)

func TestS2EXIT1Repair1MandatoryMarkers(t *testing.T) {
	source, err := os.ReadFile("runtime_daemon_test.go")
	if err != nil {
		t.Fatalf("read repair test source: %v", err)
	}
	markers := [][2]string{
		{"s2_exit_metadata_duplicate_", "identity_zero_append"},
		{"s2_exit_metadata_invalid_", "identity_zero_append"},
		{"s2_exit_metadata_non_utc_", "zero_append"},
		{"s2_exit_metadata_cancellation_", "zero_append"},
		{"s2_exit_metadata_sequence_", "overflow"},
		{"s2_exit_configuration_complete_", "rejection_matrix"},
		{"s2_exit_configuration_typed_", "nil_identity"},
	}
	for _, marker := range markers {
		full := marker[0] + marker[1]
		if count := strings.Count(string(source), full); count != 1 {
			t.Errorf("mandatory repair marker %q count = %d, want 1", full, count)
		}
	}
}

func TestLocalRuntimeObservationDaemonRealSQLiteRestart(t *testing.T) {
	config := daemonTestConfig(t, "1.0.0", "model-a")
	clock := newDaemonTestClock()
	identities := &daemonTestIdentitySource{}

	daemon, err := NewLocalRuntimeObservationDaemon(config, clock, identities)
	if err != nil {
		t.Fatalf("construct daemon: %v", err)
	}
	locked, lockErr := NewLocalRuntimeObservationDaemon(
		config, newDaemonTestClock(), &daemonTestIdentitySource{},
	)
	if lockErr == nil || locked != nil {
		t.Fatalf("second daemon must fail while state is locked: daemon=%v err=%v", locked, lockErr)
	}

	result, err := daemon.Run(context.Background())
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if result.CompletedCycles != 2 ||
		result.DiscoveryEvents != 1 ||
		result.StatusEvents != 0 ||
		result.NoWriteCycles != 1 {
		t.Fatalf("unexpected first result: %#v", result)
	}
	assertDaemonRuntimeFact(t, result.RuntimeFacts, "1.0.0", "provider/model-a", 1)
	if err := daemon.Close(); err != nil {
		t.Fatalf("close first daemon: %v", err)
	}

	restartRuntimeDir := filepath.Join(filepath.Dir(config.StatePath), "runtime-restart")
	if err := os.Mkdir(restartRuntimeDir, 0o700); err != nil {
		t.Fatalf("mkdir restart runtime: %v", err)
	}
	writeDaemonTestPi(t, restartRuntimeDir, "2.0.0", "model-b")
	config.RuntimeSearchPaths = []string{restartRuntimeDir}
	config.MaxCycles = 1
	restarted, err := NewLocalRuntimeObservationDaemon(
		config, newDaemonTestClock(), &daemonTestIdentitySource{next: 100},
	)
	if err != nil {
		t.Fatalf("restart daemon: %v", err)
	}
	restartedResult, err := restarted.Run(context.Background())
	if err != nil {
		t.Fatalf("restart run: %v", err)
	}
	if restartedResult.CompletedCycles != 1 ||
		restartedResult.DiscoveryEvents != 1 ||
		restartedResult.NoWriteCycles != 0 {
		t.Fatalf("unexpected restart result: %#v", restartedResult)
	}
	assertDaemonRuntimeFact(
		t, restartedResult.RuntimeFacts, "2.0.0", "provider/model-b", 2,
	)
	if err := restarted.Close(); err != nil {
		t.Fatalf("close restarted daemon: %v", err)
	}
	if info, err := os.Stat(config.StatePath); err != nil {
		t.Fatalf("stat state: %v", err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("state mode = %o, want 600", info.Mode().Perm())
	}
}

func TestLocalRuntimeObservationDaemonCancellationWhileWaiting(t *testing.T) {
	config := daemonTestConfig(t, "1.0.0", "model-a")
	config.MaxCycles = 0
	clock := newDaemonBlockingTestClock()
	daemon, err := NewLocalRuntimeObservationDaemon(
		config, clock, &daemonTestIdentitySource{},
	)
	if err != nil {
		t.Fatalf("construct daemon: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	clock.onWait = cancel

	result, err := daemon.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run error = %v, want context canceled", err)
	}
	if result.CompletedCycles != 1 || result.DiscoveryEvents != 1 {
		t.Fatalf("unexpected canceled result: %#v", result)
	}
	if clock.waits != 1 {
		t.Fatalf("wait calls = %d, want 1", clock.waits)
	}
	if err := daemon.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestLocalRuntimeObservationDaemonRejectsInvalidConfigBeforeStateCreation(t *testing.T) {
	config := daemonTestConfig(t, "1.0.0", "model-a")
	state := filepath.Join(filepath.Dir(config.StatePath), "must-not-exist.db")
	config.StatePath = state
	config.ProbeID = ""
	daemon, err := NewLocalRuntimeObservationDaemon(
		config, newDaemonTestClock(), &daemonTestIdentitySource{},
	)
	if err == nil || daemon != nil {
		t.Fatalf("invalid config accepted: daemon=%v err=%v", daemon, err)
	}
	if _, statErr := os.Stat(state); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("unexpected state side effect: %v", statErr)
	}

	valid := daemonTestConfig(t, "1.0.0", "model-a")
	var nilClock *daemonTestClock
	daemon, err = NewLocalRuntimeObservationDaemon(
		valid, nilClock, &daemonTestIdentitySource{},
	)
	if err == nil || daemon != nil {
		t.Fatalf("typed-nil clock accepted: daemon=%v err=%v", daemon, err)
	}

	duplicate := daemonTestConfig(t, "1.0.0", "model-a")
	duplicate.RuntimeSearchPaths = append(
		duplicate.RuntimeSearchPaths,
		duplicate.RuntimeSearchPaths[0],
	)
	daemon, err = NewLocalRuntimeObservationDaemon(
		duplicate, newDaemonTestClock(), &daemonTestIdentitySource{},
	)
	if err == nil || daemon != nil {
		t.Fatalf("duplicate runtime directory accepted: daemon=%v err=%v", daemon, err)
	}

	symlinked := daemonTestConfig(t, "1.0.0", "model-a")
	target := filepath.Join(filepath.Dir(symlinked.StatePath), "target.db")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatalf("write state target: %v", err)
	}
	if err := os.Symlink(target, symlinked.StatePath); err != nil {
		t.Fatalf("symlink state: %v", err)
	}
	daemon, err = NewLocalRuntimeObservationDaemon(
		symlinked, newDaemonTestClock(), &daemonTestIdentitySource{},
	)
	if err == nil || daemon != nil {
		t.Fatalf("symlinked state accepted: daemon=%v err=%v", daemon, err)
	}
}

func TestLocalRuntimeObservationDaemonRejectsDuplicateIdentityWithoutAppend(
	t *testing.T,
) {
	// s2_exit_metadata_duplicate_identity_zero_append
	config := daemonTestConfig(t, "1.0.0", "model-a")
	config.MaxCycles = 1
	daemon, err := NewLocalRuntimeObservationDaemon(
		config,
		newDaemonTestClock(),
		daemonFixedIdentitySource("00000000000000000000000000000001"),
	)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	result, err := daemon.Run(context.Background())
	if !errors.Is(err, ErrLocalRuntimeObservationDaemonMetadata) {
		t.Fatalf("run error = %v, want metadata error", err)
	}
	assertZeroDaemonResult(t, result)
	assertDaemonJournalEventCount(t, daemon, 0)
	if err := daemon.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestLocalRuntimeObservationDaemonRejectsInvalidIdentityWithoutAppend(
	t *testing.T,
) {
	// s2_exit_metadata_invalid_identity_zero_append
	config := daemonTestConfig(t, "1.0.0", "model-a")
	config.MaxCycles = 1
	daemon, err := NewLocalRuntimeObservationDaemon(
		config,
		newDaemonTestClock(),
		daemonFixedIdentitySource("not-hex"),
	)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	result, err := daemon.Run(context.Background())
	if !errors.Is(err, ErrLocalRuntimeObservationDaemonMetadata) {
		t.Fatalf("run error = %v, want metadata error", err)
	}
	assertZeroDaemonResult(t, result)
	assertDaemonJournalEventCount(t, daemon, 0)
	if err := daemon.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestLocalRuntimeObservationDaemonRejectsNonUTCClockWithoutAppend(
	t *testing.T,
) {
	// s2_exit_metadata_non_utc_zero_append
	config := daemonTestConfig(t, "1.0.0", "model-a")
	config.MaxCycles = 1
	daemon, err := NewLocalRuntimeObservationDaemon(
		config,
		daemonNonUTCClock{},
		&daemonTestIdentitySource{},
	)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	result, err := daemon.Run(context.Background())
	if !errors.Is(err, ErrLocalRuntimeObservationDaemonMetadata) {
		t.Fatalf("run error = %v, want metadata error", err)
	}
	assertZeroDaemonResult(t, result)
	assertDaemonJournalEventCount(t, daemon, 0)
	if err := daemon.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestLocalRuntimeObservationDaemonPreservesIdentityCancellationWithoutAppend(
	t *testing.T,
) {
	// s2_exit_metadata_cancellation_zero_append
	config := daemonTestConfig(t, "1.0.0", "model-a")
	config.MaxCycles = 1
	ctx, cancel := context.WithCancel(context.Background())
	daemon, err := NewLocalRuntimeObservationDaemon(
		config,
		newDaemonTestClock(),
		&daemonCancelingIdentitySource{cancel: cancel},
	)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	result, err := daemon.Run(ctx)
	if err != context.Canceled {
		t.Fatalf("run error = %v, want exact context.Canceled", err)
	}
	assertZeroDaemonResult(t, result)
	assertDaemonJournalEventCount(t, daemon, 0)
	if err := daemon.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestNextRuntimeObservationSequence(t *testing.T) {
	// s2_exit_metadata_sequence_overflow
	tests := []struct {
		name    string
		current projection.RuntimeInstance
		exists  bool
		want    int64
		wantErr bool
	}{
		{name: "new", want: 1},
		{
			name: "discovery latest",
			current: projection.RuntimeInstance{
				ID: "runtime-1", DiscoverySequence: 41, StatusSequence: 17,
			},
			exists: true,
			want:   42,
		},
		{
			name: "status latest",
			current: projection.RuntimeInstance{
				ID: "runtime-1", DiscoverySequence: 17, StatusSequence: 41,
			},
			exists: true,
			want:   42,
		},
		{
			name: "missing projected identity",
			current: projection.RuntimeInstance{
				DiscoverySequence: 1,
			},
			exists:  true,
			wantErr: true,
		},
		{
			name: "non-positive latest",
			current: projection.RuntimeInstance{
				ID: "runtime-1",
			},
			exists:  true,
			wantErr: true,
		},
		{
			name: "discovery overflow",
			current: projection.RuntimeInstance{
				ID: "runtime-1", DiscoverySequence: math.MaxInt64,
			},
			exists:  true,
			wantErr: true,
		},
		{
			name: "status overflow",
			current: projection.RuntimeInstance{
				ID: "runtime-1", DiscoverySequence: 1, StatusSequence: math.MaxInt64,
			},
			exists:  true,
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := nextRuntimeObservationSequence(test.current, test.exists)
			if test.wantErr {
				if !errors.Is(err, ErrLocalRuntimeObservationDaemonMetadata) {
					t.Fatalf("error = %v, want metadata error", err)
				}
				if got != 0 {
					t.Fatalf("sequence = %d, want zero on failure", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("sequence = %d, error = %v, want %d", got, err, test.want)
			}
		})
	}
}

func TestLocalRuntimeObservationDaemonCompleteConfigurationRejectionMatrix(
	t *testing.T,
) {
	// s2_exit_configuration_complete_rejection_matrix
	tests := []struct {
		name   string
		mutate func(*testing.T, *LocalRuntimeObservationDaemonConfig)
	}{
		{
			name: "absent state parent",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				config.StatePath = filepath.Join(
					filepath.Dir(config.StatePath), "absent-parent", "loom.db",
				)
			},
		},
		{
			name: "non-private state parent",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				if err := os.Chmod(filepath.Dir(config.StatePath), 0o755); err != nil {
					t.Fatalf("chmod state parent: %v", err)
				}
			},
		},
		{
			name: "non-directory state parent",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				parent := filepath.Join(filepath.Dir(config.StatePath), "parent-file")
				if err := os.WriteFile(parent, []byte("unchanged"), 0o600); err != nil {
					t.Fatalf("write state parent file: %v", err)
				}
				config.StatePath = filepath.Join(parent, "loom.db")
			},
		},
		{
			name: "symlink state parent",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				root := filepath.Dir(config.StatePath)
				target := filepath.Join(root, "state-parent-target")
				link := filepath.Join(root, "state-parent-link")
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatalf("mkdir state parent target: %v", err)
				}
				if err := os.Symlink(target, link); err != nil {
					t.Fatalf("symlink state parent: %v", err)
				}
				config.StatePath = filepath.Join(link, "loom.db")
			},
		},
		{
			name: "state path directory",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				if err := os.Mkdir(config.StatePath, 0o700); err != nil {
					t.Fatalf("mkdir state path: %v", err)
				}
			},
		},
		{
			name: "state path symlink",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				target := config.StatePath + ".target"
				if err := os.WriteFile(target, []byte("unchanged"), 0o600); err != nil {
					t.Fatalf("write state target: %v", err)
				}
				if err := os.Symlink(target, config.StatePath); err != nil {
					t.Fatalf("symlink state: %v", err)
				}
			},
		},
		{
			name: "state path wrong mode",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				if err := os.WriteFile(config.StatePath, []byte("unchanged"), 0o600); err != nil {
					t.Fatalf("write state: %v", err)
				}
				if err := os.Chmod(config.StatePath, 0o644); err != nil {
					t.Fatalf("chmod state: %v", err)
				}
			},
		},
		{
			name: "absent isolation root",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				config.IsolationRoot = filepath.Join(
					filepath.Dir(config.StatePath), "absent-isolation",
				)
			},
		},
		{
			name: "non-private isolation root",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				if err := os.Chmod(config.IsolationRoot, 0o755); err != nil {
					t.Fatalf("chmod isolation: %v", err)
				}
			},
		},
		{
			name: "non-directory isolation root",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				path := filepath.Join(filepath.Dir(config.StatePath), "isolation-file")
				if err := os.WriteFile(path, []byte("unchanged"), 0o600); err != nil {
					t.Fatalf("write isolation file: %v", err)
				}
				config.IsolationRoot = path
			},
		},
		{
			name: "symlink isolation root",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				root := filepath.Dir(config.StatePath)
				target := filepath.Join(root, "isolation-target")
				link := filepath.Join(root, "isolation-link")
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatalf("mkdir isolation target: %v", err)
				}
				if err := os.Symlink(target, link); err != nil {
					t.Fatalf("symlink isolation: %v", err)
				}
				config.IsolationRoot = link
			},
		},
		{
			name: "absent runtime directory",
			mutate: func(t *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				config.RuntimeSearchPaths = []string{filepath.Join(
					filepath.Dir(config.StatePath), "absent-runtime",
				)}
			},
		},
		{
			name: "duplicate runtime directory",
			mutate: func(_ *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				config.RuntimeSearchPaths = append(
					config.RuntimeSearchPaths, config.RuntimeSearchPaths[0],
				)
			},
		},
		{
			name: "interval below minimum",
			mutate: func(_ *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				config.ObservationInterval = minRuntimeObservationInterval - time.Nanosecond
			},
		},
		{
			name: "interval above maximum",
			mutate: func(_ *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				config.ObservationInterval = maxRuntimeObservationInterval + time.Nanosecond
			},
		},
		{
			name: "timeout at lower bound",
			mutate: func(_ *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				config.ProcessTimeout = 0
			},
		},
		{
			name: "timeout above maximum",
			mutate: func(_ *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				config.ProcessTimeout = maxRuntimeObservationTimeout + time.Nanosecond
			},
		},
		{
			name: "cycles below minimum",
			mutate: func(_ *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				config.MaxCycles = -1
			},
		},
		{
			name: "cycles above maximum",
			mutate: func(_ *testing.T, config *LocalRuntimeObservationDaemonConfig) {
				config.MaxCycles = maxRuntimeObservationCycles + 1
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := daemonTestConfig(t, "1.0.0", "model-a")
			test.mutate(t, &config)
			assertDaemonConfigRejectedWithoutStateSideEffects(
				t, config, newDaemonTestClock(), &daemonTestIdentitySource{},
			)
		})
	}
}

func TestLocalRuntimeObservationDaemonRejectsTypedNilIdentityWithoutStateSideEffects(
	t *testing.T,
) {
	// s2_exit_configuration_typed_nil_identity
	config := daemonTestConfig(t, "1.0.0", "model-a")
	var identities *daemonTypedNilIdentitySource
	assertDaemonConfigRejectedWithoutStateSideEffects(
		t, config, newDaemonTestClock(), identities,
	)
}

func TestLocalRuntimeObservationDaemonResultMutationIsolation(t *testing.T) {
	config := daemonTestConfig(t, "1.0.0", "model-a")
	config.MaxCycles = 1
	daemon, err := NewLocalRuntimeObservationDaemon(
		config, newDaemonTestClock(), &daemonTestIdentitySource{},
	)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	first, err := daemon.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	first.RuntimeFacts[0].ModelIDs[0] = "mutated"
	if err := daemon.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	restarted, err := NewLocalRuntimeObservationDaemon(
		config, newDaemonTestClock(), &daemonTestIdentitySource{next: 50},
	)
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	second, err := restarted.Run(context.Background())
	if err != nil {
		t.Fatalf("restart run: %v", err)
	}
	if reflect.DeepEqual(first.RuntimeFacts, second.RuntimeFacts) {
		t.Fatal("mutated returned facts contaminated persisted projection")
	}
	assertDaemonRuntimeFact(t, second.RuntimeFacts, "1.0.0", "provider/model-a", 1)
	if err := restarted.Close(); err != nil {
		t.Fatalf("close restarted: %v", err)
	}
}

func TestLocalRuntimeObservationDaemonRejectsConcurrentRunAndActiveClose(
	t *testing.T,
) {
	config := daemonTestConfig(t, "1.0.0", "model-a")
	config.MaxCycles = 0
	clock := newDaemonBlockingTestClock()
	waiting := make(chan struct{})
	var once sync.Once
	clock.onWait = func() { once.Do(func() { close(waiting) }) }
	daemon, err := NewLocalRuntimeObservationDaemon(
		config, clock, &daemonTestIdentitySource{},
	)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	type runResult struct {
		result LocalRuntimeObservationDaemonResult
		err    error
	}
	finished := make(chan runResult, 1)
	go func() {
		result, runErr := daemon.Run(ctx)
		finished <- runResult{result: result, err: runErr}
	}()
	<-waiting

	if _, err := daemon.Run(context.Background()); !errors.Is(
		err, ErrLocalRuntimeObservationDaemonRunning,
	) {
		t.Fatalf("concurrent run error = %v", err)
	}
	if err := daemon.Close(); !errors.Is(
		err, ErrLocalRuntimeObservationDaemonRunning,
	) {
		t.Fatalf("active close error = %v", err)
	}
	cancel()
	completed := <-finished
	if !errors.Is(completed.err, context.Canceled) ||
		completed.result.CompletedCycles != 1 {
		t.Fatalf("completed run = %#v err=%v", completed.result, completed.err)
	}
	if err := daemon.Close(); err != nil {
		t.Fatalf("final close: %v", err)
	}
	if err := daemon.Close(); err != nil {
		t.Fatalf("idempotent close: %v", err)
	}
}

func TestLocalRuntimeObservationDaemonFailureThenRestartRecovery(t *testing.T) {
	config := daemonTestConfig(t, "1.0.0", "model-a")
	config.MaxCycles = 1
	first, err := NewLocalRuntimeObservationDaemon(
		config, newDaemonTestClock(), &daemonTestIdentitySource{},
	)
	if err != nil {
		t.Fatalf("construct first: %v", err)
	}
	if _, err := first.Run(context.Background()); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	failingDir := daemonTestRuntimeDir(t, filepath.Dir(config.StatePath), "failing")
	writeDaemonFailingPi(t, failingDir)
	config.RuntimeSearchPaths = []string{failingDir}
	failing, err := NewLocalRuntimeObservationDaemon(
		config, newDaemonTestClock(), &daemonTestIdentitySource{next: 20},
	)
	if err != nil {
		t.Fatalf("construct failing: %v", err)
	}
	failedResult, err := failing.Run(context.Background())
	if !errors.Is(err, ErrLocalRuntimeObservationDaemonCycle) {
		t.Fatalf("failure error = %v", err)
	}
	if failedResult.CompletedCycles != 0 {
		t.Fatalf("failure committed a cycle: %#v", failedResult)
	}
	if err := failing.Close(); err != nil {
		t.Fatalf("close failing: %v", err)
	}

	recoveryDir := daemonTestRuntimeDir(t, filepath.Dir(config.StatePath), "recovery")
	writeDaemonTestPi(t, recoveryDir, "1.0.0", "model-a")
	config.RuntimeSearchPaths = []string{recoveryDir}
	recovered, err := NewLocalRuntimeObservationDaemon(
		config, newDaemonTestClock(), &daemonTestIdentitySource{next: 40},
	)
	if err != nil {
		t.Fatalf("construct recovery: %v", err)
	}
	recoveredResult, err := recovered.Run(context.Background())
	if err != nil {
		t.Fatalf("recovery run: %v", err)
	}
	if recoveredResult.NoWriteCycles != 1 ||
		recoveredResult.DiscoveryEvents != 0 ||
		recoveredResult.StatusEvents != 0 {
		t.Fatalf("recovery rewrote accepted facts: %#v", recoveredResult)
	}
	assertDaemonRuntimeFact(
		t, recoveredResult.RuntimeFacts, "1.0.0", "provider/model-a", 1,
	)
	if err := recovered.Close(); err != nil {
		t.Fatalf("close recovered: %v", err)
	}
}

func TestLocalRuntimeObservationDaemonStatusMetadataAndSequence(t *testing.T) {
	config := daemonTestConfig(t, "1.0.0", "model-a")
	config.MaxCycles = 1
	daemon, err := NewLocalRuntimeObservationDaemon(
		config, newDaemonTestClock(), &daemonTestIdentitySource{},
	)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}

	offline, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID:                   config.RuntimeInstanceID,
		DeviceID:             config.DeviceID,
		AdapterType:          "pi-cli",
		DisplayName:          config.DisplayName,
		ExecutableVersion:    "1.0.0",
		Status:               loomruntime.RuntimeOffline,
		ObservedCapabilities: []string{"pi.metadata.models", "pi.metadata.version"},
		Capacity:             1,
	})
	if err != nil {
		t.Fatalf("offline instance: %v", err)
	}
	source, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{daemonStaticRuntimeProbe{
			id: "pi-local",
			observations: []loomruntime.RuntimeObservation{{
				Instance:      offline,
				ModelIDs:      []string{"provider/model-a"},
				SourceProbeID: "pi-local",
			}},
		}},
	)
	if err != nil {
		t.Fatalf("offline discovery: %v", err)
	}
	if _, err := state.CommitRuntimeDiscoverySnapshot(
		context.Background(),
		journal.NewStore(daemon.db),
		source,
		state.RuntimeDiscoveryCommitInput{
			DiscoveryID: "seed-discovery",
			EmittedAt: time.Date(
				2026, 7, 25, 1, 0, 0, 0, time.UTC,
			),
			Events: []state.RuntimeDiscoveryEventInput{{
				RuntimeInstanceID: config.RuntimeInstanceID,
				EventID:           "seed-event",
				IdempotencyKey:    "seed-key",
				Seq:               1,
			}},
		},
	); err != nil {
		t.Fatalf("seed offline discovery: %v", err)
	}

	result, err := daemon.Run(context.Background())
	if err != nil {
		t.Fatalf("status run: %v", err)
	}
	if result.StatusEvents != 1 ||
		result.DiscoveryEvents != 0 ||
		result.NoWriteCycles != 0 {
		t.Fatalf("unexpected status result: %#v", result)
	}
	if len(result.RuntimeFacts) != 1 ||
		result.RuntimeFacts[0].Status != "online" ||
		result.RuntimeFacts[0].DiscoverySequence != 1 ||
		result.RuntimeFacts[0].StatusSequence != 2 {
		t.Fatalf("unexpected status fact: %#v", result.RuntimeFacts)
	}
	if err := daemon.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestLocalRuntimeObservationDaemonStaticBoundary(t *testing.T) {
	files := []string{
		"runtime_daemon.go",
		"runtime_daemon_lock_unix.go",
		"runtime_daemon_lock_unsupported.go",
	}
	for _, filename := range files {
		source, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("read %s: %v", filename, err)
		}
		for _, forbidden := range []string{
			"os/exec",
			"net/http",
			"AgentGrant",
			"claim_generation",
			"WorkItemCreated",
			"RunCreated",
			"ProviderID",
		} {
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s contains forbidden authority %q", filename, forbidden)
			}
		}
		parsed, err := parser.ParseFile(
			token.NewFileSet(), filename, source, parser.SkipObjectResolution,
		)
		if err != nil {
			t.Fatalf("parse %s: %v", filename, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			if _, ok := node.(*ast.GoStmt); ok {
				t.Errorf("%s contains hidden goroutine authority", filename)
			}
			return true
		})
	}
}

type daemonTestClock struct {
	mu     sync.Mutex
	now    time.Time
	waits  int
	block  bool
	onWait func()
}

func newDaemonTestClock() *daemonTestClock {
	return &daemonTestClock{now: time.Date(2026, 7, 25, 1, 2, 3, 0, time.UTC)}
}

func newDaemonBlockingTestClock() *daemonTestClock {
	clock := newDaemonTestClock()
	clock.block = true
	return clock
}

func (c *daemonTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(time.Second)
	return c.now
}

func (c *daemonTestClock) Wait(ctx context.Context, interval time.Duration) error {
	c.mu.Lock()
	c.waits++
	onWait := c.onWait
	block := c.block
	if !block {
		c.now = c.now.Add(interval)
	}
	c.mu.Unlock()
	if onWait != nil {
		onWait()
	}
	if block {
		<-ctx.Done()
		return ctx.Err()
	}
	return ctx.Err()
}

type daemonTestIdentitySource struct {
	mu   sync.Mutex
	next int
}

func (s *daemonTestIdentitySource) NextRuntimeObservationIdentity(
	ctx context.Context,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	return fmt.Sprintf("%032x", s.next), nil
}

type daemonFixedIdentitySource string

func (s daemonFixedIdentitySource) NextRuntimeObservationIdentity(
	ctx context.Context,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return string(s), nil
}

type daemonCancelingIdentitySource struct {
	once   sync.Once
	cancel context.CancelFunc
}

func (s *daemonCancelingIdentitySource) NextRuntimeObservationIdentity(
	ctx context.Context,
) (string, error) {
	s.once.Do(s.cancel)
	return "", ctx.Err()
}

type daemonTypedNilIdentitySource struct{}

func (*daemonTypedNilIdentitySource) NextRuntimeObservationIdentity(
	context.Context,
) (string, error) {
	return "00000000000000000000000000000001", nil
}

type daemonNonUTCClock struct{}

func (daemonNonUTCClock) Now() time.Time {
	return time.Date(
		2026, 7, 25, 1, 2, 3, 0,
		time.FixedZone("non-utc", 8*60*60),
	)
}

func (daemonNonUTCClock) Wait(
	ctx context.Context,
	_ time.Duration,
) error {
	return ctx.Err()
}

type daemonTestPathSnapshot struct {
	exists bool
	mode   os.FileMode
	link   string
	data   []byte
}

func captureDaemonTestPathSnapshot(
	t *testing.T,
	path string,
) daemonTestPathSnapshot {
	t.Helper()
	info, err := os.Lstat(path)
	if daemonTestPathIsMissing(err) {
		return daemonTestPathSnapshot{}
	}
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	snapshot := daemonTestPathSnapshot{
		exists: true,
		mode:   info.Mode(),
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		snapshot.link, err = os.Readlink(path)
	case info.Mode().IsRegular():
		snapshot.data, err = os.ReadFile(path)
	}
	if err != nil {
		t.Fatalf("snapshot %s: %v", path, err)
	}
	return snapshot
}

func assertDaemonConfigRejectedWithoutStateSideEffects(
	t *testing.T,
	config LocalRuntimeObservationDaemonConfig,
	clock RuntimeObservationDaemonClock,
	identities RuntimeObservationIdentitySource,
) {
	t.Helper()
	before := captureDaemonTestPathSnapshot(t, config.StatePath)
	daemon, err := NewLocalRuntimeObservationDaemon(config, clock, identities)
	if !errors.Is(err, ErrInvalidLocalRuntimeObservationDaemon) || daemon != nil {
		t.Fatalf("invalid config accepted: daemon=%v err=%v", daemon, err)
	}
	after := captureDaemonTestPathSnapshot(t, config.StatePath)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("state path changed: before=%#v after=%#v", before, after)
	}
	if _, err := os.Lstat(config.StatePath + ".lock"); !daemonTestPathIsMissing(err) {
		t.Fatalf("rejected config created state lock: %v", err)
	}
}

func daemonTestPathIsMissing(err error) bool {
	return os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR)
}

func assertZeroDaemonResult(
	t *testing.T,
	result LocalRuntimeObservationDaemonResult,
) {
	t.Helper()
	if result.CompletedCycles != 0 ||
		result.DiscoveryEvents != 0 ||
		result.StatusEvents != 0 ||
		result.NoWriteCycles != 0 ||
		len(result.RuntimeFacts) != 0 {
		t.Fatalf("failure returned non-zero result: %#v", result)
	}
}

func assertDaemonJournalEventCount(
	t *testing.T,
	daemon *LocalRuntimeObservationDaemon,
	want int,
) {
	t.Helper()
	var got int
	if err := daemon.db.QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM events`,
	).Scan(&got); err != nil {
		t.Fatalf("count daemon events: %v", err)
	}
	if got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
}

func daemonTestConfig(
	t *testing.T,
	version string,
	model string,
) LocalRuntimeObservationDaemonConfig {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatalf("chmod root: %v", err)
	}
	isolation := filepath.Join(root, "isolation")
	runtimeDir := filepath.Join(root, "runtime")
	for _, path := range []string{isolation, runtimeDir} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	writeDaemonTestPi(t, runtimeDir, version, model)
	return LocalRuntimeObservationDaemonConfig{
		StatePath:           filepath.Join(root, "loom.db"),
		IsolationRoot:       isolation,
		RuntimeSearchPaths:  []string{runtimeDir},
		ProbeID:             "pi-local",
		RuntimeInstanceID:   "pi-local-1",
		DeviceID:            "device-1",
		DisplayName:         "Local Pi",
		ObservationInterval: 10 * time.Millisecond,
		// These tests exercise daemon state and identity behavior, not the
		// production probe deadline. Leave enough room for a real subprocess to
		// be scheduled while the repository test suite is under parallel load.
		ProcessTimeout: 15 * time.Second,
		MaxCycles:      2,
	}
}

func TestLocalRuntimeObservationDaemonRetriesTypedPiTimeoutAndClearsHealth(
	t *testing.T,
) {
	config := daemonTestConfig(t, "0.82.1", "recovered-model")
	config.MaxCycles = 1
	config.ProcessTimeout = 500 * time.Millisecond
	config.ObservationInterval = 10 * time.Millisecond
	runtimeDir := config.RuntimeSearchPaths[0]
	piPath := filepath.Join(runtimeDir, "pi")
	script := `#!/bin/sh
set -eu
counter="${0%/*}/model-attempts"
if [ "$#" -eq 1 ] && [ "$1" = "--version" ]; then
  printf '0.82.1\n'
  exit 0
fi
if [ "$#" -eq 8 ] && [ "$8" = "--list-models" ]; then
  count=0
  if [ -f "$counter" ]; then count=$(/bin/cat "$counter"); fi
  count=$((count + 1))
  printf '%s' "$count" > "$counter"
  if [ "$count" -eq 1 ]; then
    /bin/sleep 2
    exit 0
  fi
  printf 'provider model context max-out thinking images\nprovider recovered-model 32K 256 no no\n'
  exit 0
fi
exit 83
`
	if err := os.WriteFile(piPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	reporter := &daemonRuntimeObservationHealthReporter{}
	daemon, err := NewLocalRuntimeObservationDaemon(
		config,
		NewSystemRuntimeObservationDaemonClock(),
		NewCryptographicRuntimeObservationIdentitySource(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Close()
	if err := daemon.SetRuntimeObservationHealthReporter(reporter); err != nil {
		t.Fatal(err)
	}
	if err := daemon.SetRuntimeObservationHealthReporter(reporter); !errors.Is(
		err,
		ErrInvalidLocalRuntimeObservationDaemon,
	) {
		t.Fatalf("duplicate health reporter error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := daemon.Run(ctx)
	if err != nil {
		t.Fatalf("recovered daemon run: %v", err)
	}
	if result.CompletedCycles != 1 {
		t.Fatalf("completed cycles = %d, want 1", result.CompletedCycles)
	}
	failures, modelFailures, recoveries := reporter.snapshot()
	if failures < 1 || modelFailures < 1 || recoveries != 1 {
		t.Fatalf(
			"health lifecycle = failures:%d model_failures:%d recoveries:%d",
			failures,
			modelFailures,
			recoveries,
		)
	}
	contents, err := os.ReadFile(filepath.Join(runtimeDir, "model-attempts"))
	if err != nil || string(contents) != "2" {
		t.Fatalf("model attempts = %q, error = %v", contents, err)
	}
}

type daemonRuntimeObservationHealthReporter struct {
	mu            sync.Mutex
	failures      int
	modelFailures int
	recoveries    int
}

type daemonTaggedPiMetadataFailure struct {
	command loomruntime.PiMetadataCommand
	cause   error
}

func (failure daemonTaggedPiMetadataFailure) Error() string { return "typed Pi metadata failure" }
func (failure daemonTaggedPiMetadataFailure) Unwrap() error { return failure.cause }
func (failure daemonTaggedPiMetadataFailure) PiMetadataFailureCommand() loomruntime.PiMetadataCommand {
	return failure.command
}

func TestRetryablePiMetadataTimeoutRejectsMixedOrUntaggedFailures(t *testing.T) {
	typed := daemonTaggedPiMetadataFailure{
		command: loomruntime.PiMetadataListModels,
		cause:   piadapter.ErrPiMetadataProcessTimeout,
	}
	wrapped := fmt.Errorf("%w: %w", loomruntime.ErrRuntimeDiscoveryFailed, typed)
	if !retryablePiMetadataTimeout(wrapped) {
		t.Fatal("exact typed Pi timeout was not retryable")
	}
	if retryablePiMetadataTimeout(errors.Join(wrapped, errors.New("unknown failure"))) {
		t.Fatal("mixed Pi timeout was retryable")
	}
	if retryablePiMetadataTimeout(piadapter.ErrPiMetadataProcessTimeout) {
		t.Fatal("untagged Pi timeout was retryable")
	}
}

func (reporter *daemonRuntimeObservationHealthReporter) RuntimeObservationFailed(
	err error,
) {
	reporter.mu.Lock()
	defer reporter.mu.Unlock()
	if errors.Is(err, piadapter.ErrPiMetadataProcessTimeout) {
		reporter.failures++
		if command, ok := loomruntime.PiMetadataFailureCommand(err); ok &&
			command == loomruntime.PiMetadataListModels {
			reporter.modelFailures++
		}
	}
}

func (reporter *daemonRuntimeObservationHealthReporter) RuntimeObservationRecovered() {
	reporter.mu.Lock()
	reporter.recoveries++
	reporter.mu.Unlock()
}

func (reporter *daemonRuntimeObservationHealthReporter) snapshot() (int, int, int) {
	reporter.mu.Lock()
	defer reporter.mu.Unlock()
	return reporter.failures, reporter.modelFailures, reporter.recoveries
}

func writeDaemonTestPi(
	t *testing.T,
	runtimeDir string,
	version string,
	model string,
) {
	t.Helper()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "--version" ]; then
  printf '%s\n'
  exit 0
fi
printf 'provider model context max-out thinking images\nprovider %s 200K 32K no no\n'
`, version, model)
	path := filepath.Join(runtimeDir, "pi")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write pi fixture: %v", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatalf("chmod pi fixture: %v", err)
	}
}

func daemonTestRuntimeDir(t *testing.T, root string, name string) string {
	t.Helper()
	path := filepath.Join(root, "runtime-"+name)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir runtime %s: %v", name, err)
	}
	return path
}

func writeDaemonFailingPi(t *testing.T, runtimeDir string) {
	t.Helper()
	path := filepath.Join(runtimeDir, "pi")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 9\n"), 0o700); err != nil {
		t.Fatalf("write failing pi: %v", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatalf("chmod failing pi: %v", err)
	}
}

type daemonStaticRuntimeProbe struct {
	id           string
	observations []loomruntime.RuntimeObservation
}

func (p daemonStaticRuntimeProbe) ID() string {
	return p.id
}

func (p daemonStaticRuntimeProbe) ObserveRuntime(
	context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	return append([]loomruntime.RuntimeObservation(nil), p.observations...), nil
}

func assertDaemonRuntimeFact(
	t *testing.T,
	facts []LocalRuntimeObservationFact,
	version string,
	model string,
	discoverySequence int64,
) {
	t.Helper()
	if len(facts) != 1 {
		t.Fatalf("runtime fact count = %d, want 1", len(facts))
	}
	fact := facts[0]
	if fact.RuntimeInstanceID != "pi-local-1" ||
		fact.ExecutableVersion != version ||
		fact.Status != "online" ||
		!reflect.DeepEqual(fact.ModelIDs, []string{model}) ||
		fact.DiscoverySequence != discoverySequence {
		t.Fatalf("unexpected runtime fact: %#v", fact)
	}
}
