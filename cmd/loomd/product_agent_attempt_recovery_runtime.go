package main

import (
	"context"
	"errors"
	"sync"

	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
)

var (
	errProductInvalidAttemptRecoveryAttachment = errors.New("invalid Agent Attempt recovery attachment")
	errProductAttemptRecoveryReattach          = errors.New("Agent Attempt Runtime reattachment failed")
)

// productAgentAttemptRecoverySession is an already reattached, version-locked
// Runtime session. It carries no Provider dispatch method.
type productAgentAttemptRecoverySession interface {
	RuntimeInstanceID() string
	SessionBindingDigest() string
	Close() error
}

// productAgentAttemptRecoveryReattacher is trusted product composition. A
// production implementation must attest the Runtime before returning a session.
type productAgentAttemptRecoveryReattacher interface {
	ReattachAgentAttempt(
		context.Context,
		work.AgentAttemptRecoveryDispatchGrant,
	) (productAgentAttemptRecoverySession, error)
}

type productAgentAttemptRecoveryContinuationSession interface {
	productAgentAttemptRecoverySession
	ContinueAgentAttempt(
		context.Context,
		supervisor.FrameSink,
	) (supervisor.AdapterResult, error)
}

type productAgentAttemptRecoveryRuntime struct {
	active     *productActiveAttemptRegistry
	reattacher productAgentAttemptRecoveryReattacher
}

type productAgentAttemptRecoveryAttachment struct {
	registration *productActiveAttemptRegistration
	session      productAgentAttemptRecoverySession
	closeOnce    sync.Once
	closeErr     error
}

func newProductAgentAttemptRecoveryRuntime(
	active *productActiveAttemptRegistry,
	reattacher productAgentAttemptRecoveryReattacher,
) (*productAgentAttemptRecoveryRuntime, error) {
	if active == nil || nilProductAgentInterface(reattacher) {
		return nil, errProductInvalidAttemptRecoveryAttachment
	}
	return &productAgentAttemptRecoveryRuntime{active: active, reattacher: reattacher}, nil
}

func (runtime *productAgentAttemptRecoveryRuntime) Attach(
	ctx context.Context,
	lease *work.AgentAttemptRecoveryDispatchLease,
) (*productAgentAttemptRecoveryAttachment, productActiveAttempt, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil || lease == nil {
		return nil, productActiveAttempt{}, errProductInvalidAttemptRecoveryAttachment
	}
	grant, err := lease.Take()
	if err != nil {
		return nil, productActiveAttempt{}, errors.Join(
			errProductInvalidAttemptRecoveryAttachment, err,
		)
	}
	return runtime.attachGrant(ctx, grant)
}

func (runtime *productAgentAttemptRecoveryRuntime) Resume(
	ctx context.Context,
	lease *work.AgentAttemptRecoveryDispatchLease,
	sink supervisor.FrameSink,
) (supervisor.AdapterResult, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil || lease == nil ||
		nilProductAgentInterface(sink) {
		return supervisor.AdapterResult{}, errProductInvalidAttemptRecoveryAttachment
	}
	attachment, _, err := runtime.Attach(ctx, lease)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	defer attachment.Close()
	continuation, ok := attachment.session.(productAgentAttemptRecoveryContinuationSession)
	if !ok || nilProductAgentInterface(continuation) {
		return supervisor.AdapterResult{}, errProductInvalidAttemptRecoveryAttachment
	}
	return continuation.ContinueAgentAttempt(ctx, sink)
}

func (runtime *productAgentAttemptRecoveryRuntime) attachGrant(
	ctx context.Context,
	grant work.AgentAttemptRecoveryDispatchGrant,
) (*productAgentAttemptRecoveryAttachment, productActiveAttempt, error) {
	if runtime == nil || runtime.active == nil || nilProductAgentInterface(runtime.reattacher) ||
		ctx == nil || ctx.Err() != nil {
		return nil, productActiveAttempt{}, errProductInvalidAttemptRecoveryAttachment
	}
	_, capability, err := work.ValidateAgentAttemptRecoveryDispatchGrant(grant)
	if err != nil {
		return nil, productActiveAttempt{}, errors.Join(
			errProductInvalidAttemptRecoveryAttachment, err,
		)
	}
	session, err := runtime.reattacher.ReattachAgentAttempt(ctx, grant)
	if err != nil || nilProductAgentInterface(session) {
		return nil, productActiveAttempt{}, errors.Join(errProductAttemptRecoveryReattach, err)
	}
	closeOnFailure := func(cause error) (*productAgentAttemptRecoveryAttachment, productActiveAttempt, error) {
		return nil, productActiveAttempt{}, errors.Join(cause, session.Close())
	}
	if ctx.Err() != nil {
		return closeOnFailure(errors.Join(errProductAttemptRecoveryReattach, ctx.Err()))
	}
	if session.RuntimeInstanceID() != capability.RuntimeInstanceID ||
		session.SessionBindingDigest() != capability.SessionBindingDigest {
		return closeOnFailure(errProductInvalidAttemptRecoveryAttachment)
	}
	registration, active, err := runtime.active.RegisterRecovered(grant)
	if err != nil {
		return closeOnFailure(err)
	}
	return &productAgentAttemptRecoveryAttachment{
		registration: registration,
		session:      session,
	}, active, nil
}

func (attachment *productAgentAttemptRecoveryAttachment) Close() error {
	if attachment == nil {
		return nil
	}
	attachment.closeOnce.Do(func() {
		if attachment.registration != nil {
			attachment.registration.Close()
		}
		if !nilProductAgentInterface(attachment.session) {
			attachment.closeErr = attachment.session.Close()
		}
	})
	return attachment.closeErr
}
