package supervisor

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

// RecoveryGrantBinding is the non-secret identity of the original execution
// grant. Recovery never reconstructs or accepts the original bearer token.
type RecoveryGrantBinding struct {
	GrantID           string
	WorkItemID        string
	RunID             string
	ClaimID           string
	ClaimGeneration   int64
	RuntimeInstanceID string
	AgentInstanceID   string
	AllowedOperations []authorization.Operation
}

// RecoveryFrameAuthority validates a recovered Runtime stream under an exact,
// consumed recovery grant. It does not commit Run state or revoke grants.
type RecoveryFrameAuthority struct {
	binding           bridgev1.RunStreamBinding
	correlationID     string
	dispatchMessageID string
	allowed           map[authorization.Operation]struct{}
	observer          AuthorizedFrameObserver
	stream            bridgev1.BoundRunStream
	frames            []bridgev1.Frame
	status            string
	reason            string
	resultSeen        bool
}

type RecoveryAdapterResultValidator interface {
	ValidateAdapterResult(
		context.Context,
		AdapterResult,
	) (bridgev1.BoundRunStream, string, string, error)
}

func NewRecoveryFrameAuthority(
	recoveryGrant work.AgentAttemptRecoveryDispatchGrant,
	original RecoveryGrantBinding,
	dispatchMessageID string,
	observer AuthorizedFrameObserver,
) (*RecoveryFrameAuthority, error) {
	outcome, _, err := work.ValidateAgentAttemptRecoveryDispatchGrant(recoveryGrant)
	if err != nil || dispatchMessageID == "" || original.GrantID == "" ||
		observer != nil && nilManagedInterface(observer) {
		return nil, errors.Join(ErrInvalidManagedExecution, err)
	}
	authority := outcome.Binding.PayloadAuthority
	if original.WorkItemID != authority.WorkItemID ||
		original.RunID != authority.RunID || original.ClaimID != authority.ClaimID ||
		original.ClaimGeneration != authority.ClaimGeneration ||
		original.RuntimeInstanceID != authority.RuntimeInstanceID ||
		original.AgentInstanceID != authority.AgentInstanceID {
		return nil, ErrInvalidManagedExecution
	}
	allowed := make(map[authorization.Operation]struct{}, len(original.AllowedOperations))
	for _, operation := range original.AllowedOperations {
		if _, duplicate := allowed[operation]; duplicate {
			return nil, ErrInvalidManagedExecution
		}
		allowed[operation] = struct{}{}
	}
	if !managedHasOperations(original.AllowedOperations) {
		return nil, ErrInvalidManagedExecution
	}
	binding := bridgev1.RunStreamBinding{
		WorkItemID: authority.WorkItemID, RunID: authority.RunID,
		ClaimGeneration:       authority.ClaimGeneration,
		RuntimeInstanceID:     authority.RuntimeInstanceID,
		SenderAgentInstanceID: authority.AgentInstanceID,
	}
	stream, err := bridgev1.NewBoundRunStream(binding)
	if err != nil {
		return nil, errors.Join(ErrBridgeSession, err)
	}
	return &RecoveryFrameAuthority{
		binding: binding, correlationID: recoveryGrant.IncidentID(),
		dispatchMessageID: dispatchMessageID, allowed: allowed,
		observer: observer, stream: stream,
	}, nil
}

func (authority *RecoveryFrameAuthority) AcceptFrame(
	ctx context.Context,
	frame bridgev1.Frame,
) error {
	if authority == nil || ctx == nil || authority.resultSeen {
		return ErrBridgeSession
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	operation, valid := managedFrameOperation(frame.Type())
	if !valid || frame.CorrelationID() != authority.correlationID {
		return ErrBridgeSession
	}
	if _, permitted := authority.allowed[operation]; !permitted {
		return ErrBridgeSession
	}
	index := len(authority.frames)
	if frame.Type() == bridgev1.MessageAck && index != 0 {
		return ErrBridgeSession
	}
	if index == 0 && (frame.Type() != bridgev1.MessageAck ||
		!exactManagedMessagePayload(frame.Payload(), authority.dispatchMessageID)) {
		return ErrBridgeSession
	}
	candidate, err := bridgev1.AdvanceBoundRunStream(authority.stream, frame)
	if err != nil {
		return errors.Join(ErrBridgeSession, err)
	}
	status := ""
	reason := ""
	if frame.Type() == bridgev1.MessageResult {
		status, reason, err = parseManagedResultPayload(frame.Payload())
		if err != nil {
			return err
		}
	}
	cloned, err := cloneManagedFrames([]bridgev1.Frame{frame})
	if err != nil {
		return ErrBridgeSession
	}
	authority.stream = candidate
	authority.frames = append(authority.frames, cloned[0])
	if frame.Type() == bridgev1.MessageResult {
		authority.status = status
		authority.reason = reason
		authority.resultSeen = true
	}
	if authority.observer != nil {
		observed, cloneErr := cloneManagedFrames([]bridgev1.Frame{frame})
		if cloneErr != nil {
			return ErrBridgeSession
		}
		if err := authority.observer.ObserveAuthorizedFrame(ctx, AuthorizedFrame{
			frame: observed[0], binding: authority.binding,
		}); err != nil {
			return errors.Join(ErrAuthorizedFrameObserver, err)
		}
	}
	return nil
}

// ValidateAdapterResult proves that the Runtime's returned transcript is the
// exact stream already admitted by AcceptFrame.
func (authority *RecoveryFrameAuthority) ValidateAdapterResult(
	ctx context.Context,
	result AdapterResult,
) (bridgev1.BoundRunStream, string, string, error) {
	if authority == nil || ctx == nil {
		return bridgev1.BoundRunStream{}, "", "", ErrBridgeSession
	}
	if err := ctx.Err(); err != nil {
		return bridgev1.BoundRunStream{}, "", "", err
	}
	if !result.valid || !result.DispatchAcknowledged() ||
		!result.ResultAcknowledged() || !authority.resultSeen {
		return cloneManagedStream(authority.stream), "", "", ErrBridgeSession
	}
	frames := result.InboundFrames()
	if len(frames) != len(authority.frames) || len(frames) < 2 {
		return cloneManagedStream(authority.stream), "", "", ErrBridgeSession
	}
	for index := range frames {
		if !sameManagedFrame(frames[index], authority.frames[index]) {
			return cloneManagedStream(authority.stream), "", "", ErrBridgeSession
		}
	}
	if frames[0].Type() != bridgev1.MessageAck ||
		!exactManagedMessagePayload(frames[0].Payload(), authority.dispatchMessageID) ||
		frames[len(frames)-1].Type() != bridgev1.MessageResult {
		return cloneManagedStream(authority.stream), "", "", ErrBridgeSession
	}
	return cloneManagedStream(authority.stream), authority.status, authority.reason, nil
}
