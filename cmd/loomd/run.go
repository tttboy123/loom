package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"loom-pi-rebuild/internal/app"
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

type daemonBuildConfig struct {
	Observer   app.LocalRuntimeObservationDaemonConfig
	SocketPath string
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
		return nil, err
	}
	if config.SocketPath == "" {
		return observer, nil
	}
	return newProductDaemonRunner(
		observer,
		config.Observer.StatePath,
		config.SocketPath,
	)
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
		},
		SocketPath: *socketPath,
	}
	daemon, err := builder(config)
	if err != nil || daemon == nil {
		return writeDaemonError(stderr, exitUnavailable, "daemon unavailable")
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
			return "daemon failed: observer"
		case "local_ipc":
			return "daemon failed: local_ipc"
		case "shutdown":
			return "daemon failed: shutdown"
		}
	}
	return "daemon failed: shutdown"
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
