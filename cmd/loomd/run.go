package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/runtime/piadapter"
)

const (
	exitSuccess        = 0
	exitInvalidInput   = 2
	exitUnavailable    = 3
	exitRuntimeFailure = 4
)

type daemonRunner interface {
	Run(context.Context) (app.LocalRuntimeObservationDaemonResult, error)
	Close() error
}

type daemonFailureCoder interface {
	DaemonFailureCode() string
}

type daemonFailureReasoner interface {
	DaemonFailureReason() string
}

type daemonBuildFailure struct {
	reason string
	err    error
}

func (failure *daemonBuildFailure) Error() string                    { return "daemon unavailable" }
func (failure *daemonBuildFailure) Unwrap() error                    { return failure.err }
func (failure *daemonBuildFailure) DaemonBuildFailureReason() string { return failure.reason }

func newDaemonBuildFailure(reason string, err error) error {
	if err == nil || !validDaemonBuildFailureReason(reason) {
		return &daemonBuildFailure{reason: "build_unknown", err: err}
	}
	return &daemonBuildFailure{reason: reason, err: err}
}

func validDaemonBuildFailureReason(reason string) bool {
	switch reason {
	case "build_observer", "build_state", "build_setup_runtime",
		"build_setup_credential", "build_setup_provider",
		"build_setup_native_auth", "build_decision", "build_execution",
		"build_assets", "build_queue", "build_workers", "build_ipc", "build_unknown":
		return true
	default:
		return false
	}
}

func daemonBuildFailureReason(err error) string {
	reasons := map[string]struct{}{}
	var visit func(error)
	visit = func(current error) {
		if current == nil {
			return
		}
		if reasoner, ok := current.(interface{ DaemonBuildFailureReason() string }); ok {
			reason := reasoner.DaemonBuildFailureReason()
			if !validDaemonBuildFailureReason(reason) {
				reason = "build_unknown"
			}
			reasons[reason] = struct{}{}
		}
		if joined, ok := current.(interface{ Unwrap() []error }); ok {
			for _, child := range joined.Unwrap() {
				visit(child)
			}
			return
		}
		if wrapped := errors.Unwrap(current); wrapped != nil {
			visit(wrapped)
		}
	}
	visit(err)
	if len(reasons) != 1 {
		return "build_unknown"
	}
	for reason := range reasons {
		return reason
	}
	return "build_unknown"
}

type daemonBuildConfig struct {
	Observer        app.LocalRuntimeObservationDaemonConfig
	SocketPath      string
	CodexExecutable string
}

type daemonBuilder func(daemonBuildConfig) (daemonRunner, error)

type repeatedStrings []string

func (values *repeatedStrings) String() string {
	return fmt.Sprint([]string(*values))
}

func (values *repeatedStrings) Set(value string) error {
	if value == "" {
		return errors.New("empty value")
	}
	*values = append(*values, value)
	return nil
}

func productionDaemonBuilder(
	config daemonBuildConfig,
) (daemonRunner, error) {
	observer, err := app.NewLocalRuntimeObservationDaemon(
		config.Observer,
		app.NewSystemRuntimeObservationDaemonClock(),
		app.NewCryptographicRuntimeObservationIdentitySource(),
	)
	if err != nil {
		return nil, newDaemonBuildFailure("build_observer", err)
	}
	if config.SocketPath == "" {
		return observer, nil
	}
	return newProductDaemonRunner(
		observer,
		config.Observer.StatePath,
		config.SocketPath,
		productSetupRuntimeConfig{
			CodexExecutable: config.CodexExecutable,
			Execution:       missionExecutionConfigFromDaemonBuild(config),
		},
	)
}

func missionExecutionConfigFromDaemonBuild(
	config daemonBuildConfig,
) *productMissionExecutionRuntimeConfig {
	if config.Observer.LocalModelCatalog == nil {
		return nil
	}
	catalog := *config.Observer.LocalModelCatalog
	return &productMissionExecutionRuntimeConfig{
		RuntimeSearchPaths: append(
			[]string(nil),
			config.Observer.RuntimeSearchPaths...,
		),
		RuntimeInstanceID: config.Observer.RuntimeInstanceID,
		LocalModelCatalog: &catalog,
	}
}

func run(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	builder daemonBuilder,
) int {
	if ctx == nil {
		return writeDaemonError(stderr, exitInvalidInput, "invalid input")
	}
	if builder == nil {
		builder = productionDaemonBuilder
	}

	fs := flag.NewFlagSet("loomd", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	statePath := fs.String("state", "", "")
	isolationRoot := fs.String("isolation-root", "", "")
	var runtimeDirs repeatedStrings
	fs.Var(&runtimeDirs, "runtime-dir", "")
	probeID := fs.String("probe-id", "", "")
	instanceID := fs.String("instance-id", "", "")
	deviceID := fs.String("device-id", "", "")
	displayName := fs.String("display-name", "", "")
	interval := fs.Duration("interval", 0, "")
	processTimeout := fs.Duration("process-timeout", 0, "")
	maxCycles := fs.Int("max-cycles", 0, "")
	socketPath := fs.String("socket", "", "")
	codexExecutable := fs.String("codex-executable", "", "")
	localModelPrivateRoot := fs.String("local-model-private-root", "", "")
	localModelExecutable := fs.String("local-model-executable", "", "")
	localModelPath := fs.String("local-model-path", "", "")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return writeDaemonError(stderr, exitInvalidInput, "invalid input")
	}
	if *statePath == "" ||
		*isolationRoot == "" ||
		len(runtimeDirs) == 0 ||
		*probeID == "" ||
		*instanceID == "" ||
		*deviceID == "" ||
		*displayName == "" ||
		*interval <= 0 ||
		*processTimeout <= 0 {
		return writeDaemonError(stderr, exitInvalidInput, "invalid input")
	}
	localModelValues := 0
	for _, value := range []string{
		*localModelPrivateRoot,
		*localModelExecutable,
		*localModelPath,
	} {
		if value != "" {
			localModelValues++
		}
	}
	if localModelValues != 0 && localModelValues != 3 {
		return writeDaemonError(stderr, exitInvalidInput, "invalid input")
	}
	var localModelCatalog *piadapter.PiLocalModelCatalogConfig
	if localModelValues == 3 {
		localModelCatalog = &piadapter.PiLocalModelCatalogConfig{
			PrivateRoot:    *localModelPrivateRoot,
			ExecutablePath: *localModelExecutable,
			ModelPath:      *localModelPath,
		}
	}

	config := daemonBuildConfig{
		Observer: app.LocalRuntimeObservationDaemonConfig{
			StatePath:           *statePath,
			IsolationRoot:       *isolationRoot,
			RuntimeSearchPaths:  append([]string(nil), runtimeDirs...),
			ProbeID:             *probeID,
			RuntimeInstanceID:   *instanceID,
			DeviceID:            *deviceID,
			DisplayName:         *displayName,
			ObservationInterval: *interval,
			ProcessTimeout:      *processTimeout,
			MaxCycles:           *maxCycles,
			LocalModelCatalog:   localModelCatalog,
		},
		SocketPath:      *socketPath,
		CodexExecutable: *codexExecutable,
	}
	daemon, err := builder(config)
	if err != nil || daemon == nil {
		return writeDaemonError(
			stderr, exitUnavailable,
			"daemon unavailable: "+daemonBuildFailureReason(err),
		)
	}

	result, runErr := daemon.Run(ctx)
	closeErr := daemon.Close()
	if runErr != nil &&
		!errors.Is(runErr, context.Canceled) &&
		!errors.Is(runErr, context.DeadlineExceeded) {
		return writeDaemonError(
			stderr,
			exitRuntimeFailure,
			daemonFailureMessage(runErr),
		)
	}
	if closeErr != nil {
		return writeDaemonError(
			stderr,
			exitRuntimeFailure,
			"daemon failed: shutdown",
		)
	}
	if err := writeDaemonResult(stdout, result); err != nil {
		return writeDaemonError(
			stderr,
			exitRuntimeFailure,
			"daemon failed: result",
		)
	}
	return exitSuccess
}

func daemonFailureMessage(err error) string {
	var failure daemonFailureCoder
	if errors.As(err, &failure) {
		switch failure.DaemonFailureCode() {
		case "observer":
			reason := "observer_unknown"
			var reasoner daemonFailureReasoner
			if errors.As(err, &reasoner) &&
				validObserverFailureReason(
					reasoner.DaemonFailureReason(),
				) {
				reason = reasoner.DaemonFailureReason()
			}
			return "daemon failed: " + reason
		case "local_ipc":
			return "daemon failed: local_ipc"
		case "shutdown":
			return "daemon failed: shutdown"
		}
	}
	return "daemon failed: shutdown"
}

func validObserverFailureReason(reason string) bool {
	switch reason {
	case "observer_probe_factory",
		"observer_probe_candidate",
		"observer_probe_construction",
		"observer_metadata_binding",
		"observer_version_process",
		"observer_version_timeout",
		"observer_version_output_limit",
		"observer_version_stderr",
		"observer_version_output",
		"observer_models_process",
		"observer_models_timeout",
		"observer_models_output_limit",
		"observer_models_stderr",
		"observer_models_output",
		"observer_models_duplicate",
		"observer_inventory",
		"observer_projection",
		"observer_plan",
		"observer_identity_metadata",
		"observer_write",
		"observer_unknown":
		return true
	default:
		return false
	}
}

type daemonOutput struct {
	CompletedCycles int                 `json:"completed_cycles"`
	DiscoveryEvents int                 `json:"discovery_events"`
	StatusEvents    int                 `json:"status_events"`
	NoWriteCycles   int                 `json:"no_write_cycles"`
	RuntimeFacts    []daemonRuntimeFact `json:"runtime_facts"`
}

type daemonRuntimeFact struct {
	RuntimeInstanceID string   `json:"runtime_instance_id"`
	ExecutableVersion string   `json:"executable_version"`
	Status            string   `json:"status"`
	ModelIDs          []string `json:"model_ids"`
	DiscoverySequence int64    `json:"discovery_sequence"`
	StatusSequence    int64    `json:"status_sequence"`
}

func writeDaemonResult(
	writer io.Writer,
	result app.LocalRuntimeObservationDaemonResult,
) error {
	output := daemonOutput{
		CompletedCycles: result.CompletedCycles,
		DiscoveryEvents: result.DiscoveryEvents,
		StatusEvents:    result.StatusEvents,
		NoWriteCycles:   result.NoWriteCycles,
		RuntimeFacts:    make([]daemonRuntimeFact, len(result.RuntimeFacts)),
	}
	for index, fact := range result.RuntimeFacts {
		output.RuntimeFacts[index] = daemonRuntimeFact{
			RuntimeInstanceID: fact.RuntimeInstanceID,
			ExecutableVersion: fact.ExecutableVersion,
			Status:            fact.Status,
			ModelIDs:          append([]string(nil), fact.ModelIDs...),
			DiscoverySequence: fact.DiscoverySequence,
			StatusSequence:    fact.StatusSequence,
		}
	}
	if output.RuntimeFacts == nil {
		output.RuntimeFacts = []daemonRuntimeFact{}
	}
	return json.NewEncoder(writer).Encode(output)
}

func writeDaemonError(writer io.Writer, code int, message string) int {
	_, _ = fmt.Fprintln(writer, message)
	return code
}
