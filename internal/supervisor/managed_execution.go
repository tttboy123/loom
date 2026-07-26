package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/authorization"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

var (
	ErrInvalidManagedExecution = errors.New("invalid managed execution")
	ErrManagedWorkspace        = errors.New("managed workspace failure")
	ErrSourceChanged           = errors.New("source changed during managed execution")
	ErrRuntimeAdapter          = errors.New("runtime adapter failure")
	ErrBridgeSession           = errors.New("bridge session failure")
	ErrRuntimeTimeout          = errors.New("managed execution timeout")
	ErrRuntimeCancelled        = errors.New("managed execution cancelled")
	ErrProcessCleanup          = errors.New("managed process cleanup failed")
	ErrAuthorizedFrameObserver = errors.New("authorized Frame observer failed")
)

const (
	maxManagedStderrBytes = 256 << 10
	maxManagedReasonBytes = 1024
)

type WorkspaceChangeKind string

const (
	WorkspaceChangeAdded    WorkspaceChangeKind = "added"
	WorkspaceChangeModified WorkspaceChangeKind = "modified"
	WorkspaceChangeDeleted  WorkspaceChangeKind = "deleted"
)

type RuntimeAdapter interface {
	AdapterType() string
	RuntimeInstanceID() string
	Execute(context.Context, AdapterRequest) (AdapterResult, error)
}

type FrameSink interface {
	AcceptFrame(context.Context, bridgev1.Frame) error
}

type AuthorizedFrame struct {
	frame   bridgev1.Frame
	binding bridgev1.RunStreamBinding
}

func (frame AuthorizedFrame) Frame() bridgev1.Frame {
	cloned, _ := cloneManagedFrames([]bridgev1.Frame{frame.frame})
	if len(cloned) == 0 {
		return bridgev1.Frame{}
	}
	return cloned[0]
}

func (frame AuthorizedFrame) Binding() bridgev1.RunStreamBinding {
	return frame.binding
}

func (AuthorizedFrame) Tentative() bool { return true }

type AuthorizedFrameObserver interface {
	ObserveAuthorizedFrame(context.Context, AuthorizedFrame) error
}

type AdapterRequest struct {
	WorkspacePath string
	HomePath      string
	TempPath      string
	Binding       bridgev1.RunStreamBinding
	Dispatch      bridgev1.Frame
	Grant         authorization.Token
	FrameSink     FrameSink
}

type AdapterResultInput struct {
	InboundFrames        []bridgev1.Frame
	Stderr               []byte
	ExitCode             int
	DispatchAcknowledged bool
	ResultAcknowledged   bool
	CancelAcknowledged   bool
}

type AdapterResult struct {
	inboundFrames        []bridgev1.Frame
	stderr               []byte
	exitCode             int
	dispatchAcknowledged bool
	resultAcknowledged   bool
	cancelAcknowledged   bool
	valid                bool
}

type Config struct {
	WorkspaceRoot  string
	CleanupTimeout time.Duration
}

type ExecuteInput struct {
	SourcePath    string
	Profile       loomruntime.RuntimeProfile
	Instance      loomruntime.RuntimeInstance
	Generation    work.RunGenerationInput
	Grant         authorization.IssuedGrant
	Dispatch      bridgev1.Frame
	FrameObserver AuthorizedFrameObserver
}

type WorkspaceChange struct {
	path    string
	kind    WorkspaceChangeKind
	mode    fs.FileMode
	digest  string
	content []byte
}

type Outcome struct {
	workItem        work.WorkItemRecord
	run             work.RunRecord
	stream          bridgev1.BoundRunStream
	sourceDigest    string
	workspaceDigest string
	changes         []WorkspaceChange
	stderr          []byte
}

type Supervisor struct {
	config            Config
	workAuthority     *work.Authority
	grantAuthority    *authorization.Authority
	adapter           RuntimeAdapter
	workspaceRootInfo fs.FileInfo
	activeMu          sync.Mutex
	activeRuns        map[string]struct{}
}

type authorizedFrameSink struct {
	supervisor *Supervisor
	input      ExecuteInput
	stream     bridgev1.BoundRunStream
	frames     []bridgev1.Frame
	status     string
	reason     string
	resultSeen bool
}

func (sink *authorizedFrameSink) AcceptFrame(
	ctx context.Context,
	frame bridgev1.Frame,
) error {
	if sink == nil || sink.supervisor == nil || ctx == nil {
		return ErrBridgeSession
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if bytes.Contains(
		frame.Payload(),
		[]byte(sink.input.Grant.Token().Value()),
	) {
		return ErrBridgeSession
	}
	index := len(sink.frames)
	operation, valid := managedFrameOperation(frame.Type())
	if !valid ||
		frame.Type() == bridgev1.MessageAck && index != 0 ||
		sink.resultSeen {
		return ErrBridgeSession
	}
	if index == 0 &&
		(frame.Type() != bridgev1.MessageAck ||
			!exactManagedMessagePayload(
				frame.Payload(),
				sink.input.Dispatch.MessageID(),
			)) {
		return ErrBridgeSession
	}
	candidate, err := bridgev1.AdvanceBoundRunStream(sink.stream, frame)
	if err != nil {
		return errors.Join(ErrBridgeSession, err)
	}
	status := ""
	reason := ""
	if frame.Type() == bridgev1.MessageResult {
		status, reason, err = parseManagedResultPayload(frame.Payload())
		if err != nil {
			return errors.Join(ErrBridgeSession, err)
		}
	}
	if _, err := sink.supervisor.grantAuthority.Authorize(
		ctx,
		authorization.AuthorizeInput{
			Token:             sink.input.Grant.Token(),
			WorkItemID:        sink.input.Generation.WorkItemID,
			RunID:             sink.input.Generation.RunID,
			ClaimID:           sink.input.Generation.ClaimID,
			ClaimGeneration:   sink.input.Generation.ClaimGeneration,
			RuntimeInstanceID: sink.input.Generation.RuntimeInstanceID,
			AgentInstanceID:   sink.input.Generation.AgentInstanceID,
			Operation:         operation,
			RequestID:         frame.MessageID(),
			CorrelationID:     sink.input.Generation.CorrelationID,
		},
	); err != nil {
		return errors.Join(ErrBridgeSession, err)
	}
	cloned, err := cloneManagedFrames([]bridgev1.Frame{frame})
	if err != nil {
		return ErrBridgeSession
	}
	sink.stream = candidate
	sink.frames = append(sink.frames, cloned[0])
	if frame.Type() == bridgev1.MessageResult {
		sink.status = status
		sink.reason = reason
		sink.resultSeen = true
	}
	if sink.input.FrameObserver != nil {
		observed, cloneErr := cloneManagedFrames([]bridgev1.Frame{frame})
		if cloneErr != nil {
			return ErrBridgeSession
		}
		if err := sink.input.FrameObserver.ObserveAuthorizedFrame(
			ctx,
			AuthorizedFrame{
				frame:   observed[0],
				binding: managedBinding(sink.input.Generation),
			},
		); err != nil {
			return errors.Join(ErrAuthorizedFrameObserver, err)
		}
	}
	return nil
}

func NewAdapterResult(input AdapterResultInput) (AdapterResult, error) {
	if len(input.Stderr) > maxManagedStderrBytes ||
		len(input.InboundFrames) > bridgev1.MaxBufferedFrames ||
		input.ExitCode < -1 ||
		input.ExitCode > 255 {
		return AdapterResult{}, ErrInvalidManagedExecution
	}
	frames, err := cloneManagedFrames(input.InboundFrames)
	if err != nil {
		return AdapterResult{}, ErrInvalidManagedExecution
	}
	return AdapterResult{
		inboundFrames:        frames,
		stderr:               bytes.Clone(input.Stderr),
		exitCode:             input.ExitCode,
		dispatchAcknowledged: input.DispatchAcknowledged,
		resultAcknowledged:   input.ResultAcknowledged,
		cancelAcknowledged:   input.CancelAcknowledged,
		valid:                true,
	}, nil
}

func (result AdapterResult) InboundFrames() []bridgev1.Frame {
	frames, _ := cloneManagedFrames(result.inboundFrames)
	return frames
}
func (result AdapterResult) Stderr() []byte             { return bytes.Clone(result.stderr) }
func (result AdapterResult) ExitCode() int              { return result.exitCode }
func (result AdapterResult) DispatchAcknowledged() bool { return result.dispatchAcknowledged }
func (result AdapterResult) ResultAcknowledged() bool   { return result.resultAcknowledged }
func (result AdapterResult) CancelAcknowledged() bool   { return result.cancelAcknowledged }

func (change WorkspaceChange) Path() string              { return change.path }
func (change WorkspaceChange) Kind() WorkspaceChangeKind { return change.kind }
func (change WorkspaceChange) Mode() fs.FileMode         { return change.mode }
func (change WorkspaceChange) Digest() string            { return change.digest }
func (change WorkspaceChange) Content() []byte           { return bytes.Clone(change.content) }

func (outcome Outcome) WorkItem() work.WorkItemRecord   { return outcome.workItem }
func (outcome Outcome) Run() work.RunRecord             { return outcome.run }
func (outcome Outcome) Stream() bridgev1.BoundRunStream { return cloneManagedStream(outcome.stream) }
func (outcome Outcome) SourceDigest() string            { return outcome.sourceDigest }
func (outcome Outcome) WorkspaceDigest() string         { return outcome.workspaceDigest }
func (outcome Outcome) Changes() []WorkspaceChange      { return cloneWorkspaceChanges(outcome.changes) }
func (outcome Outcome) Stderr() []byte                  { return bytes.Clone(outcome.stderr) }

func New(
	config Config,
	workAuthority *work.Authority,
	grantAuthority *authorization.Authority,
	adapter RuntimeAdapter,
) (*Supervisor, error) {
	if workAuthority == nil ||
		grantAuthority == nil ||
		nilManagedInterface(adapter) ||
		config.CleanupTimeout <= 0 ||
		config.CleanupTimeout > 30*time.Second ||
		adapter.AdapterType() == "" ||
		adapter.RuntimeInstanceID() == "" {
		return nil, ErrInvalidManagedExecution
	}
	if err := validateAbsoluteCleanDirectory(config.WorkspaceRoot, true); err != nil {
		return nil, fmt.Errorf("%w: workspace root: %v", ErrInvalidManagedExecution, err)
	}
	rootInfo, err := os.Lstat(config.WorkspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: workspace root: %v", ErrInvalidManagedExecution, err)
	}
	return &Supervisor{
		config:            config,
		workAuthority:     workAuthority,
		grantAuthority:    grantAuthority,
		adapter:           adapter,
		workspaceRootInfo: rootInfo,
		activeRuns:        make(map[string]struct{}),
	}, nil
}

func (supervisor *Supervisor) Execute(
	ctx context.Context,
	input ExecuteInput,
) (outcome Outcome, returnErr error) {
	if supervisor == nil || ctx == nil {
		return Outcome{}, ErrInvalidManagedExecution
	}
	if input.Generation.RunID == "" {
		return Outcome{}, ErrInvalidManagedExecution
	}
	if !supervisor.acquireRun(input.Generation.RunID) {
		return Outcome{}, fmt.Errorf("%w: run already active", ErrInvalidManagedExecution)
	}
	defer supervisor.releaseRun(input.Generation.RunID)
	if err := supervisor.validateInput(ctx, input); err != nil {
		if errors.Is(err, work.ErrStaleClaimGeneration) {
			err = errors.Join(err, supervisor.revokeRejected(input))
		}
		return Outcome{}, err
	}

	if err := verifyManagedDirectoryIdentity(
		supervisor.config.WorkspaceRoot,
		supervisor.workspaceRootInfo,
		true,
	); err != nil {
		return supervisor.finishFailure(
			input, nil, bridgev1.BoundRunStream{},
			"", "", nil, nil,
			"failed", "workspace_failed",
			authorization.RevocationTerminal,
			errors.Join(ErrManagedWorkspace, err),
		)
	}
	workspace, err := prepareManagedWorkspace(
		supervisor.config.WorkspaceRoot,
		input.SourcePath,
	)
	if err != nil {
		return supervisor.finishFailure(
			input,
			nil,
			bridgev1.BoundRunStream{},
			"",
			"",
			nil,
			nil,
			"failed",
			"workspace_failed",
			authorization.RevocationTerminal,
			err,
		)
	}
	defer func() {
		if cleanupErr := workspace.cleanup(); cleanupErr != nil {
			returnErr = errors.Join(returnErr, cleanupErr)
		}
	}()
	outcome.sourceDigest = workspace.sourceDigest

	if ctxErr := ctx.Err(); ctxErr != nil {
		return supervisor.finishFailure(
			input, workspace, bridgev1.BoundRunStream{},
			"", "", nil, nil,
			"cancelled", "operator_cancelled",
			authorization.RevocationCancelled,
			errors.Join(ErrRuntimeCancelled, ctxErr),
		)
	}
	if workspace.containsBytes([]byte(input.Grant.Token().Value())) {
		return supervisor.finishFailure(
			input, workspace, bridgev1.BoundRunStream{},
			"", "", nil, nil,
			"failed", "workspace_failed",
			authorization.RevocationTerminal,
			ErrManagedWorkspace,
		)
	}
	if err := workspace.verifyPrivateBindings(); err != nil {
		return supervisor.finishFailure(
			input, workspace, bridgev1.BoundRunStream{},
			"", "", nil, nil,
			"failed", "workspace_failed",
			authorization.RevocationTerminal,
			err,
		)
	}
	if err := workspace.verifySourceUnchanged(); err != nil {
		reason := "workspace_failed"
		if errors.Is(err, ErrSourceChanged) {
			reason = "source_changed"
		}
		return supervisor.finishFailure(
			input, workspace, bridgev1.BoundRunStream{},
			"", "", nil, nil,
			"failed", reason, authorization.RevocationTerminal, err,
		)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return supervisor.finishFailure(
			input, workspace, bridgev1.BoundRunStream{},
			workspace.sourceDigest, "", nil, nil,
			"cancelled", "operator_cancelled",
			authorization.RevocationCancelled,
			errors.Join(ErrRuntimeCancelled, ctxErr),
		)
	}
	workItem, run, err := supervisor.workAuthority.Start(ctx, input.Generation)
	if err != nil {
		if ctx.Err() != nil {
			return supervisor.finishFailure(
				input, workspace, bridgev1.BoundRunStream{},
				workspace.sourceDigest, "", nil, nil,
				"cancelled", "operator_cancelled",
				authorization.RevocationCancelled,
				errors.Join(ErrRuntimeCancelled, ctx.Err(), err),
			)
		}
		if errors.Is(err, work.ErrRunLeaseExpired) ||
			errors.Is(err, work.ErrStaleClaimGeneration) ||
			errors.Is(err, work.ErrRunNotClaimable) {
			err = errors.Join(err, supervisor.revokeRejected(input))
		}
		return Outcome{}, err
	}
	outcome.workItem = workItem
	outcome.run = run

	initialStream, err := bridgev1.NewBoundRunStream(
		managedBinding(input.Generation),
	)
	if err != nil {
		return supervisor.finishFailure(
			input, workspace, bridgev1.BoundRunStream{},
			workspace.sourceDigest, "", nil, nil,
			"failed", "bridge_protocol_failed",
			authorization.RevocationTerminal,
			errors.Join(ErrBridgeSession, err),
		)
	}
	frameSink := &authorizedFrameSink{
		supervisor: supervisor,
		input:      input,
		stream:     initialStream,
	}
	executionContext, cancelExecution := context.WithTimeout(ctx, input.Profile.Timeout)
	result, adapterErr := supervisor.adapter.Execute(
		executionContext,
		AdapterRequest{
			WorkspacePath: workspace.workspacePath,
			HomePath:      workspace.homePath,
			TempPath:      workspace.tempPath,
			Binding:       managedBinding(input.Generation),
			Dispatch:      input.Dispatch,
			Grant:         input.Grant.Token(),
			FrameSink:     frameSink,
		},
	)
	executionContextErr := executionContext.Err()
	cancelExecution()
	if ctx.Err() != nil {
		return supervisor.finishFailure(
			input, workspace, bridgev1.BoundRunStream{},
			workspace.sourceDigest, "", nil, nil,
			"cancelled", "operator_cancelled",
			authorization.RevocationCancelled,
			errors.Join(ErrRuntimeCancelled, ctx.Err(), adapterErr),
		)
	}
	if errors.Is(executionContextErr, context.DeadlineExceeded) {
		return supervisor.finishFailure(
			input, workspace, bridgev1.BoundRunStream{},
			workspace.sourceDigest, "", nil, nil,
			"failed", "runtime_timeout",
			authorization.RevocationTimeout,
			errors.Join(ErrRuntimeTimeout, adapterErr),
		)
	}
	if err := workspace.verifySourceUnchanged(); err != nil {
		failureReason := "workspace_failed"
		if errors.Is(err, ErrSourceChanged) {
			failureReason = "source_changed"
		}
		return supervisor.finishFailure(
			input, workspace, bridgev1.BoundRunStream{},
			workspace.sourceDigest, "", nil, nil,
			"failed", failureReason,
			authorization.RevocationTerminal,
			err,
		)
	}
	if adapterErr != nil {
		failureReason := "runtime_process_failed"
		primary := errors.Join(ErrRuntimeAdapter, adapterErr)
		if errors.Is(adapterErr, ErrBridgeSession) ||
			errors.Is(adapterErr, ErrAuthorizedFrameObserver) {
			failureReason = "bridge_protocol_failed"
			primary = adapterErr
		}
		return supervisor.finishFailure(
			input, workspace, frameSink.stream,
			workspace.sourceDigest, "", nil, nil,
			"failed", failureReason,
			authorization.RevocationTerminal,
			primary,
		)
	}
	if !result.valid {
		return supervisor.finishFailure(
			input, workspace, bridgev1.BoundRunStream{},
			workspace.sourceDigest, "", nil, nil,
			"failed", "runtime_process_failed",
			authorization.RevocationTerminal,
			ErrRuntimeAdapter,
		)
	}
	stream, status, reason, err := supervisor.acceptBridgeResult(
		ctx,
		input,
		result,
		frameSink,
	)
	if err != nil {
		if ctx.Err() != nil {
			return supervisor.finishFailure(
				input, workspace, stream,
				workspace.sourceDigest, "", nil, nil,
				"cancelled", "operator_cancelled",
				authorization.RevocationCancelled,
				errors.Join(ErrRuntimeCancelled, ctx.Err(), err),
			)
		}
		return supervisor.finishFailure(
			input, workspace, stream,
			workspace.sourceDigest, "", nil, result.Stderr(),
			"failed", "bridge_protocol_failed",
			authorization.RevocationTerminal,
			err,
		)
	}
	if result.ExitCode() != 0 {
		return supervisor.finishFailure(
			input, workspace, stream,
			workspace.sourceDigest, "", nil, result.Stderr(),
			"failed", "runtime_process_failed",
			authorization.RevocationTerminal,
			fmt.Errorf("%w: exit %d", ErrRuntimeAdapter, result.ExitCode()),
		)
	}
	if err := workspace.verifySourceUnchanged(); err != nil {
		failureReason := "workspace_failed"
		if errors.Is(err, ErrSourceChanged) {
			failureReason = "source_changed"
		}
		return supervisor.finishFailure(
			input, workspace, stream,
			workspace.sourceDigest, "", nil, result.Stderr(),
			"failed", failureReason,
			authorization.RevocationTerminal,
			err,
		)
	}
	workspaceDigest, changes, err := workspace.collectChanges()
	if err != nil {
		return supervisor.finishFailure(
			input, workspace, stream,
			workspace.sourceDigest, "", nil, result.Stderr(),
			"failed", "workspace_failed",
			authorization.RevocationTerminal,
			err,
		)
	}
	if managedChangesContain(changes, []byte(input.Grant.Token().Value())) ||
		bytes.Contains(result.Stderr(), []byte(input.Grant.Token().Value())) {
		return supervisor.finishFailure(
			input, workspace, stream,
			workspace.sourceDigest, "", nil, nil,
			"failed", "workspace_failed",
			authorization.RevocationTerminal,
			ErrManagedWorkspace,
		)
	}
	return supervisor.finishTerminal(
		input,
		stream,
		workspace.sourceDigest,
		workspaceDigest,
		changes,
		result.Stderr(),
		status,
		reason,
		authorization.RevocationTerminal,
		nil,
	)
}

func (supervisor *Supervisor) validateInput(
	ctx context.Context,
	input ExecuteInput,
) error {
	if ctx == nil {
		return ErrInvalidManagedExecution
	}
	if input.FrameObserver != nil && nilManagedInterface(input.FrameObserver) {
		return ErrInvalidManagedExecution
	}
	profile, err := loomruntime.NewRuntimeProfile(input.Profile)
	if err != nil {
		return errors.Join(ErrInvalidManagedExecution, err)
	}
	instance, err := loomruntime.NewRuntimeInstance(input.Instance)
	if err != nil {
		return errors.Join(ErrInvalidManagedExecution, err)
	}
	if _, err := loomruntime.ValidateBinding(profile, instance); err != nil {
		return errors.Join(ErrInvalidManagedExecution, err)
	}
	generation := input.Generation
	grant := input.Grant.Record()
	if generation.WorkItemID == "" ||
		generation.RunID == "" ||
		generation.ClaimID == "" ||
		generation.ClaimGeneration <= 0 ||
		generation.RuntimeInstanceID == "" ||
		generation.AgentInstanceID == "" ||
		generation.CorrelationID == "" {
		return ErrInvalidManagedExecution
	}
	if grant.ClaimGeneration() != generation.ClaimGeneration {
		return work.ErrStaleClaimGeneration
	}
	if grant.ID() == "" ||
		input.Grant.Token().Value() == "" ||
		grant.WorkItemID() != generation.WorkItemID ||
		grant.RunID() != generation.RunID ||
		grant.ClaimID() != generation.ClaimID ||
		grant.RuntimeInstanceID() != generation.RuntimeInstanceID ||
		grant.AgentInstanceID() != generation.AgentInstanceID ||
		!grant.RevokedAt().IsZero() ||
		instance.ID != generation.RuntimeInstanceID ||
		instance.AdapterType != supervisor.adapter.AdapterType() ||
		instance.ID != supervisor.adapter.RuntimeInstanceID() {
		return ErrInvalidManagedExecution
	}
	if !managedHasOperations(grant.AllowedOperations()) {
		return ErrInvalidManagedExecution
	}
	if !managedDispatchMatches(input.Dispatch, generation) ||
		bytes.Contains(
			input.Dispatch.Payload(),
			[]byte(input.Grant.Token().Value()),
		) {
		return ErrInvalidManagedExecution
	}
	snapshot, err := supervisor.workAuthority.Snapshot(context.Background())
	if err != nil {
		return err
	}
	for _, run := range snapshot.Runs() {
		if run.ID() != generation.RunID {
			continue
		}
		if run.WorkItemID() != generation.WorkItemID ||
			run.Phase() != "claimed" ||
			run.ClaimID() != generation.ClaimID ||
			run.ClaimGeneration() != generation.ClaimGeneration ||
			run.RuntimeInstanceID() != generation.RuntimeInstanceID ||
			run.AgentInstanceID() != generation.AgentInstanceID {
			return work.ErrStaleClaimGeneration
		}
		return nil
	}
	return work.ErrRunNotClaimable
}

func (supervisor *Supervisor) acceptBridgeResult(
	ctx context.Context,
	input ExecuteInput,
	result AdapterResult,
	sink *authorizedFrameSink,
) (bridgev1.BoundRunStream, string, string, error) {
	if err := ctx.Err(); err != nil {
		return bridgev1.BoundRunStream{}, "", "", err
	}
	if sink == nil ||
		!result.DispatchAcknowledged() ||
		!result.ResultAcknowledged() ||
		!sink.resultSeen {
		return bridgev1.BoundRunStream{}, "", "", ErrBridgeSession
	}
	frames := result.InboundFrames()
	if len(frames) != len(sink.frames) || len(frames) < 2 {
		return sink.stream, "", "", ErrBridgeSession
	}
	for index := range frames {
		if !sameManagedFrame(frames[index], sink.frames[index]) {
			return sink.stream, "", "", ErrBridgeSession
		}
	}
	if frames[0].Type() != bridgev1.MessageAck ||
		!exactManagedMessagePayload(
			frames[0].Payload(),
			input.Dispatch.MessageID(),
		) ||
		frames[len(frames)-1].Type() != bridgev1.MessageResult {
		return sink.stream, "", "", ErrBridgeSession
	}
	return cloneManagedStream(sink.stream), sink.status, sink.reason, nil
}

func (supervisor *Supervisor) finishFailure(
	input ExecuteInput,
	_ *managedWorkspace,
	stream bridgev1.BoundRunStream,
	sourceDigest string,
	workspaceDigest string,
	changes []WorkspaceChange,
	stderr []byte,
	status string,
	reason string,
	revokeReason authorization.RevocationReason,
	primary error,
) (Outcome, error) {
	return supervisor.finishTerminal(
		input, stream, sourceDigest, workspaceDigest, changes, stderr,
		status, reason, revokeReason, primary,
	)
}

func (supervisor *Supervisor) finishTerminal(
	input ExecuteInput,
	stream bridgev1.BoundRunStream,
	sourceDigest string,
	workspaceDigest string,
	changes []WorkspaceChange,
	stderr []byte,
	status string,
	reason string,
	revokeReason authorization.RevocationReason,
	primary error,
) (Outcome, error) {
	terminalContext, cancelTerminal := context.WithTimeout(
		context.Background(),
		supervisor.config.CleanupTimeout,
	)
	defer cancelTerminal()
	workItem, run, terminalErr := supervisor.workAuthority.CommitTerminal(
		terminalContext,
		work.RunTerminalInput{
			RunGenerationInput: input.Generation,
			Status:             status,
			Reason:             reason,
		},
	)
	cancelTerminal()
	revokeContext, cancelRevoke := context.WithTimeout(
		context.Background(),
		supervisor.config.CleanupTimeout,
	)
	defer cancelRevoke()
	_, revokeErr := supervisor.grantAuthority.Revoke(
		revokeContext,
		authorization.RevokeInput{
			GrantID:       input.Grant.Record().ID(),
			Reason:        revokeReason,
			CorrelationID: input.Generation.CorrelationID,
		},
	)
	outcome := Outcome{
		workItem:        workItem,
		run:             run,
		stream:          cloneManagedStream(stream),
		sourceDigest:    sourceDigest,
		workspaceDigest: workspaceDigest,
		changes:         cloneWorkspaceChanges(changes),
		stderr:          bytes.Clone(stderr),
	}
	return outcome, errors.Join(primary, terminalErr, revokeErr)
}

func (supervisor *Supervisor) revokeRejected(input ExecuteInput) error {
	grantID := input.Grant.Record().ID()
	if grantID == "" {
		return nil
	}
	revokeContext, cancel := context.WithTimeout(
		context.Background(),
		supervisor.config.CleanupTimeout,
	)
	defer cancel()
	_, err := supervisor.grantAuthority.Revoke(
		revokeContext,
		authorization.RevokeInput{
			GrantID:       grantID,
			Reason:        authorization.RevocationOperator,
			CorrelationID: input.Generation.CorrelationID,
		},
	)
	return err
}

func (supervisor *Supervisor) acquireRun(runID string) bool {
	supervisor.activeMu.Lock()
	defer supervisor.activeMu.Unlock()
	if _, exists := supervisor.activeRuns[runID]; exists {
		return false
	}
	supervisor.activeRuns[runID] = struct{}{}
	return true
}

func (supervisor *Supervisor) releaseRun(runID string) {
	supervisor.activeMu.Lock()
	defer supervisor.activeMu.Unlock()
	delete(supervisor.activeRuns, runID)
}

func managedBinding(input work.RunGenerationInput) bridgev1.RunStreamBinding {
	return bridgev1.RunStreamBinding{
		WorkItemID:            input.WorkItemID,
		RunID:                 input.RunID,
		ClaimGeneration:       input.ClaimGeneration,
		RuntimeInstanceID:     input.RuntimeInstanceID,
		SenderAgentInstanceID: input.AgentInstanceID,
	}
}

func managedDispatchMatches(
	frame bridgev1.Frame,
	generation work.RunGenerationInput,
) bool {
	payload := frame.Payload()
	return frame.MessageID() != "" &&
		frame.CorrelationID() == generation.CorrelationID &&
		frame.WorkItemID() == generation.WorkItemID &&
		frame.RunID() == generation.RunID &&
		frame.ClaimGeneration() == generation.ClaimGeneration &&
		frame.RuntimeInstanceID() == generation.RuntimeInstanceID &&
		frame.SenderAgentInstanceID() == generation.AgentInstanceID &&
		frame.Sequence() == 1 &&
		frame.Type() == bridgev1.MessageDispatch &&
		len(payload) >= 2 &&
		payload[0] == '{' &&
		payload[len(payload)-1] == '}'
}

func managedHasOperations(operations []authorization.Operation) bool {
	required := map[authorization.Operation]bool{
		authorization.OperationBridgeAck:    false,
		authorization.OperationBridgeResult: false,
	}
	for _, operation := range operations {
		if _, exists := required[operation]; exists {
			required[operation] = true
		}
	}
	return required[authorization.OperationBridgeAck] &&
		required[authorization.OperationBridgeResult]
}

func managedFrameOperation(
	messageType bridgev1.MessageType,
) (authorization.Operation, bool) {
	switch messageType {
	case bridgev1.MessageAck:
		return authorization.OperationBridgeAck, true
	case bridgev1.MessageEvent:
		return authorization.OperationBridgeEvent, true
	case bridgev1.MessageEvidence:
		return authorization.OperationBridgeEvidence, true
	case bridgev1.MessageResult:
		return authorization.OperationBridgeResult, true
	case bridgev1.MessageHeartbeat:
		return authorization.OperationBridgeHeartbeat, true
	default:
		return "", false
	}
}

func exactManagedMessagePayload(payload []byte, messageID string) bool {
	fields, err := decodeManagedStringObject(payload)
	return err == nil &&
		len(fields) == 1 &&
		fields["message_id"] == messageID
}

func parseManagedResultPayload(payload []byte) (string, string, error) {
	fields, err := decodeManagedStringObject(payload)
	if err != nil || len(fields) != 2 {
		return "", "", ErrBridgeSession
	}
	status, hasStatus := fields["status"]
	reason, hasReason := fields["reason"]
	if !hasStatus || !hasReason {
		return "", "", ErrBridgeSession
	}
	switch status {
	case "succeeded":
		if reason != "" {
			return "", "", ErrBridgeSession
		}
	case "failed":
		if !validManagedReason(reason) {
			return "", "", ErrBridgeSession
		}
	default:
		return "", "", ErrBridgeSession
	}
	return status, reason, nil
}

func decodeManagedStringObject(payload []byte) (map[string]string, error) {
	if len(payload) == 0 || len(payload) > bridgev1.MaxPayloadBytes {
		return nil, ErrBridgeSession
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, ErrBridgeSession
	}
	fields := make(map[string]string)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, ErrBridgeSession
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, ErrBridgeSession
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, ErrBridgeSession
		}
		var value string
		if err := decoder.Decode(&value); err != nil {
			return nil, ErrBridgeSession
		}
		fields[key] = value
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') {
		return nil, ErrBridgeSession
	}
	if token, err = decoder.Token(); !errors.Is(err, io.EOF) || token != nil {
		return nil, ErrBridgeSession
	}
	return fields, nil
}

func validManagedReason(reason string) bool {
	if reason == "" ||
		!utf8.ValidString(reason) ||
		len(reason) > maxManagedReasonBytes ||
		strings.TrimSpace(reason) != reason {
		return false
	}
	for _, character := range reason {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func cloneManagedFrames(frames []bridgev1.Frame) ([]bridgev1.Frame, error) {
	if frames == nil {
		return nil, nil
	}
	output := make([]bridgev1.Frame, len(frames))
	for index, frame := range frames {
		line, err := bridgev1.EncodeLine(frame)
		if err != nil {
			return nil, err
		}
		output[index], err = bridgev1.DecodeLine(line)
		if err != nil {
			return nil, err
		}
	}
	return output, nil
}

func sameManagedFrame(left bridgev1.Frame, right bridgev1.Frame) bool {
	leftLine, leftErr := bridgev1.EncodeLine(left)
	rightLine, rightErr := bridgev1.EncodeLine(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftLine, rightLine)
}

func cloneManagedStream(stream bridgev1.BoundRunStream) bridgev1.BoundRunStream {
	binding := stream.Binding()
	if binding.WorkItemID == "" {
		return bridgev1.BoundRunStream{}
	}
	output, err := bridgev1.NewBoundRunStream(binding)
	if err != nil {
		return bridgev1.BoundRunStream{}
	}
	for _, frame := range stream.Frames() {
		output, err = bridgev1.AdvanceBoundRunStream(output, frame)
		if err != nil {
			return bridgev1.BoundRunStream{}
		}
	}
	return output
}

func cloneWorkspaceChanges(changes []WorkspaceChange) []WorkspaceChange {
	if changes == nil {
		return nil
	}
	output := make([]WorkspaceChange, len(changes))
	for index, change := range changes {
		output[index] = WorkspaceChange{
			path:    change.path,
			kind:    change.kind,
			mode:    change.mode,
			digest:  change.digest,
			content: bytes.Clone(change.content),
		}
	}
	return output
}

func managedChangesContain(changes []WorkspaceChange, value []byte) bool {
	if len(value) == 0 {
		return false
	}
	for _, change := range changes {
		if bytes.Contains([]byte(change.path), value) ||
			bytes.Contains(change.content, value) {
			return true
		}
	}
	return false
}

func nilManagedInterface(value any) bool {
	if value == nil {
		return true
	}
	reflection := reflect.ValueOf(value)
	switch reflection.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflection.IsNil()
	default:
		return false
	}
}
