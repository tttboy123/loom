// Package failurelab provides deterministic, credential-free Provider failure
// simulations for diagnostics and failure-isolation tests.
package failurelab

import (
	"context"
	"errors"
	"io"
	"net"
	"time"
)

const (
	defaultRetryAfter       = 30 * time.Second
	syntheticTimeoutDelay   = 2 * time.Millisecond
	maxLoopbackExecution    = 250 * time.Millisecond
	loopbackProtocolVersion = byte(1)
	loopbackFrameSize       = 2
)

var (
	ErrInvalidRequest    = errors.New("failurelab: invalid request")
	ErrRunCancelled      = errors.New("failurelab: run cancelled")
	errLoopbackExecution = errors.New("failurelab: loopback execution failed")
)

type Scenario string

const (
	ScenarioSuccess             Scenario = "success"
	ScenarioAuth                Scenario = "auth"
	ScenarioRateLimit           Scenario = "rate_limit"
	ScenarioTimeout             Scenario = "timeout"
	ScenarioInsufficientBalance Scenario = "insufficient_balance"
	ScenarioCorruptVaultRecord  Scenario = "corrupt_vault_record"
	ScenarioRevisionConflict    Scenario = "revision_conflict"
)

type Stage string

const (
	StageProviderAuth         Stage = "provider_auth"
	StageProviderRateLimit    Stage = "provider_rate_limit"
	StageProviderConnect      Stage = "provider_connect"
	StageProviderHTTP         Stage = "provider_http"
	StageVaultAADValidation   Stage = "vault_aad_validation"
	StageAgentAttemptDispatch Stage = "agent_attempt_dispatch"
)

type Code string

const (
	CodeProviderAuth                Code = "provider_auth"
	CodeProviderRateLimit           Code = "provider_rate_limit"
	CodeTimeout                     Code = "timeout"
	CodeProviderInsufficientBalance Code = "provider_insufficient_balance"
	CodeCorruptVaultRecord          Code = "corrupt_vault_record"
	CodeCredentialRevisionConflict  Code = "credential_revision_conflict"
)

type Request struct {
	Scenario        Scenario
	AccountOpaqueID string
	IncidentID      string
}

// Diagnostic is the complete observable output of a failure simulation.
// Provider request and response content cannot be represented by this type.
type Diagnostic struct {
	Stage           Stage         `json:"stage"`
	Code            Code          `json:"code"`
	Retryable       bool          `json:"retryable"`
	AccountOpaqueID string        `json:"account_opaque_id"`
	IncidentID      string        `json:"incident_id"`
	Elapsed         time.Duration `json:"elapsed"`
}

// Transport uses net.Pipe exclusively. It cannot dial a network, carry an
// endpoint, or represent credentials, prompts, or Provider bodies.
type Transport struct{}

func NewTransport() *Transport {
	return &Transport{}
}

type transportOutcome struct {
	succeeded  bool
	stage      Stage
	code       Code
	retryable  bool
	httpStatus int
	retryAfter time.Duration
}

type wireScenario byte

const (
	wireSuccess wireScenario = iota + 1
	wireAuth
	wireRateLimit
	wireTimeout
	wireInsufficientBalance
	wireCorruptVaultRecord
	wireRevisionConflict
)

type wireOutcome byte

const (
	wireSucceeded wireOutcome = iota + 1
	wireProviderAuth
	wireProviderRateLimit
	wireProviderInsufficientBalance
	wireCorruptVaultRecordOutcome
	wireCredentialRevisionConflict
)

func (*Transport) execute(ctx context.Context, scenario Scenario) (transportOutcome, error) {
	wireValue, ok := encodeScenario(scenario)
	if ctx == nil || !ok {
		return transportOutcome{}, ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return transportOutcome{}, ErrRunCancelled
	}

	limit := maxLoopbackExecution
	if scenario == ScenarioTimeout {
		limit = syntheticTimeoutDelay
	}
	executionCtx, cancel := context.WithTimeout(ctx, limit)
	client, server := net.Pipe()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serveLoopback(executionCtx, server)
	}()

	deadline, _ := executionCtx.Deadline()
	_ = client.SetDeadline(deadline)
	_, writeErr := client.Write([]byte{loopbackProtocolVersion, byte(wireValue)})
	var response [loopbackFrameSize]byte
	_, readErr := io.ReadFull(client, response[:])

	cancel()
	_ = client.Close()
	_ = server.Close()
	<-serverDone

	if writeErr != nil {
		if ctx.Err() != nil {
			return transportOutcome{}, ErrRunCancelled
		}
		if scenario == ScenarioTimeout {
			return timeoutOutcome(), nil
		}
		return transportOutcome{}, errLoopbackExecution
	}
	if readErr != nil {
		if ctx.Err() != nil {
			return transportOutcome{}, ErrRunCancelled
		}
		if scenario == ScenarioTimeout {
			return timeoutOutcome(), nil
		}
		return transportOutcome{}, errLoopbackExecution
	}
	if response[0] != loopbackProtocolVersion {
		return transportOutcome{}, errLoopbackExecution
	}
	return decodeOutcome(wireOutcome(response[1]))
}

func serveLoopback(ctx context.Context, connection net.Conn) error {
	defer connection.Close()
	var request [loopbackFrameSize]byte
	if _, err := io.ReadFull(connection, request[:]); err != nil {
		return err
	}
	if request[0] != loopbackProtocolVersion {
		return errLoopbackExecution
	}
	scenario, ok := decodeScenario(wireScenario(request[1]))
	if !ok {
		return errLoopbackExecution
	}
	if scenario == ScenarioTimeout {
		<-ctx.Done()
		return ctx.Err()
	}
	outcome, ok := injectFault(scenario)
	if !ok {
		return errLoopbackExecution
	}
	_, err := connection.Write([]byte{loopbackProtocolVersion, byte(outcome)})
	return err
}

func encodeScenario(scenario Scenario) (wireScenario, bool) {
	switch scenario {
	case ScenarioSuccess:
		return wireSuccess, true
	case ScenarioAuth:
		return wireAuth, true
	case ScenarioRateLimit:
		return wireRateLimit, true
	case ScenarioTimeout:
		return wireTimeout, true
	case ScenarioInsufficientBalance:
		return wireInsufficientBalance, true
	case ScenarioCorruptVaultRecord:
		return wireCorruptVaultRecord, true
	case ScenarioRevisionConflict:
		return wireRevisionConflict, true
	default:
		return 0, false
	}
}

func decodeScenario(scenario wireScenario) (Scenario, bool) {
	switch scenario {
	case wireSuccess:
		return ScenarioSuccess, true
	case wireAuth:
		return ScenarioAuth, true
	case wireRateLimit:
		return ScenarioRateLimit, true
	case wireTimeout:
		return ScenarioTimeout, true
	case wireInsufficientBalance:
		return ScenarioInsufficientBalance, true
	case wireCorruptVaultRecord:
		return ScenarioCorruptVaultRecord, true
	case wireRevisionConflict:
		return ScenarioRevisionConflict, true
	default:
		return "", false
	}
}

func injectFault(scenario Scenario) (wireOutcome, bool) {
	switch scenario {
	case ScenarioSuccess:
		return wireSucceeded, true
	case ScenarioAuth:
		return wireProviderAuth, true
	case ScenarioRateLimit:
		return wireProviderRateLimit, true
	case ScenarioInsufficientBalance:
		return wireProviderInsufficientBalance, true
	case ScenarioCorruptVaultRecord:
		return wireCorruptVaultRecordOutcome, true
	case ScenarioRevisionConflict:
		return wireCredentialRevisionConflict, true
	default:
		return 0, false
	}
}

func decodeOutcome(outcome wireOutcome) (transportOutcome, error) {
	switch outcome {
	case wireSucceeded:
		return transportOutcome{succeeded: true}, nil
	case wireProviderAuth:
		return transportOutcome{
			stage: StageProviderAuth, code: CodeProviderAuth, httpStatus: 401,
		}, nil
	case wireProviderRateLimit:
		return transportOutcome{
			stage: StageProviderRateLimit, code: CodeProviderRateLimit,
			retryable: true, httpStatus: 429, retryAfter: defaultRetryAfter,
		}, nil
	case wireProviderInsufficientBalance:
		return transportOutcome{
			stage: StageProviderHTTP, code: CodeProviderInsufficientBalance,
			httpStatus: 402,
		}, nil
	case wireCorruptVaultRecordOutcome:
		return transportOutcome{stage: StageVaultAADValidation, code: CodeCorruptVaultRecord}, nil
	case wireCredentialRevisionConflict:
		return transportOutcome{
			stage: StageAgentAttemptDispatch, code: CodeCredentialRevisionConflict,
		}, nil
	default:
		return transportOutcome{}, errLoopbackExecution
	}
}

func timeoutOutcome() transportOutcome {
	return transportOutcome{
		stage: StageProviderConnect, code: CodeTimeout, retryable: true,
	}
}

type Runner struct {
	transport *Transport
}

func NewRunner(transport *Transport) *Runner {
	if transport == nil {
		transport = NewTransport()
	}
	return &Runner{transport: transport}
}

func (runner *Runner) Run(ctx context.Context, request Request) (Diagnostic, error) {
	if !validScenario(request.Scenario) {
		return Diagnostic{}, ErrInvalidRequest
	}
	outcome, elapsed, err := runner.executeRequest(ctx, request)
	if err != nil {
		return Diagnostic{}, err
	}
	return diagnosticFor(request, outcome, elapsed), nil
}

func (runner *Runner) executeRequest(
	ctx context.Context,
	request Request,
) (transportOutcome, time.Duration, error) {
	if runner == nil || runner.transport == nil || ctx == nil ||
		!validTeamScenario(request.Scenario) ||
		!validTemporaryAccountOpaqueID(request.AccountOpaqueID) ||
		!validOpaqueID(request.IncidentID) {
		return transportOutcome{}, 0, ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return transportOutcome{}, 0, ErrRunCancelled
	}

	started := time.Now()
	outcome, err := runner.transport.execute(ctx, request.Scenario)
	elapsed := time.Since(started)
	if err != nil {
		return transportOutcome{}, 0, err
	}
	if ctx.Err() != nil {
		return transportOutcome{}, 0, ErrRunCancelled
	}
	return outcome, elapsed, nil
}

func diagnosticFor(request Request, outcome transportOutcome, elapsed time.Duration) Diagnostic {
	return Diagnostic{
		Stage:           outcome.stage,
		Code:            outcome.code,
		Retryable:       outcome.retryable,
		AccountOpaqueID: request.AccountOpaqueID,
		IncidentID:      request.IncidentID,
		Elapsed:         elapsed,
	}
}

func validScenario(scenario Scenario) bool {
	switch scenario {
	case ScenarioAuth, ScenarioRateLimit, ScenarioTimeout,
		ScenarioInsufficientBalance, ScenarioCorruptVaultRecord,
		ScenarioRevisionConflict:
		return true
	default:
		return false
	}
}

func validOpaqueID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == ':' ||
			character == '-' {
			continue
		}
		return false
	}
	return true
}
