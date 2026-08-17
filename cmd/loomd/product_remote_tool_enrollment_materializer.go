package main

import (
	"context"
	"errors"
	"sync"

	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/toolbroker/enrollment"
	"loom-pi-rebuild/internal/work"
)

// productCompositeRemoteToolExecutor fans a tool proposal out over an explicit
// set of already-materialized Loom-owned executors. Authorization stays
// outside; every member was validated at materialization time.
type productCompositeRemoteToolExecutor struct {
	mu        sync.RWMutex
	executors []execution.RemoteToolExecutor
	lifecycle context.Context
	cancel    context.CancelCauseFunc
	closed    bool
}

func newProductCompositeRemoteToolExecutor(
	executors []execution.RemoteToolExecutor,
) (*productCompositeRemoteToolExecutor, error) {
	if len(executors) == 0 {
		return nil, execution.ErrUnsupportedTool
	}
	for _, executor := range executors {
		if executor == nil {
			return nil, execution.ErrUnsupportedTool
		}
	}
	lifecycle, cancel := context.WithCancelCause(context.Background())
	return &productCompositeRemoteToolExecutor{
		executors: append([]execution.RemoteToolExecutor(nil), executors...),
		lifecycle: lifecycle, cancel: cancel,
	}, nil
}

func (executor *productCompositeRemoteToolExecutor) AllowedRemoteTools() []permissions.ToolKind {
	if executor == nil {
		return nil
	}
	executor.mu.RLock()
	defer executor.mu.RUnlock()
	if executor.closed {
		return nil
	}
	allowed := make([]permissions.ToolKind, 0, 4)
	seen := make(map[permissions.ToolKind]struct{})
	for _, member := range executor.executors {
		for _, kind := range member.AllowedRemoteTools() {
			if _, exists := seen[kind]; exists {
				continue
			}
			seen[kind] = struct{}{}
			allowed = append(allowed, kind)
		}
	}
	return allowed
}

func (executor *productCompositeRemoteToolExecutor) ValidateProposal(
	proposal permissions.ProposedCall,
) error {
	if executor == nil {
		return execution.ErrUnsupportedTool
	}
	executor.mu.RLock()
	defer executor.mu.RUnlock()
	if executor.closed {
		return execution.ErrUnsupportedTool
	}
	var lastErr error
	for _, member := range executor.executors {
		if err := member.ValidateProposal(proposal); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		return execution.ErrUnsupportedTool
	}
	return errors.Join(execution.ErrUnsupportedTool, lastErr)
}

func (executor *productCompositeRemoteToolExecutor) ExecuteProposalContent(
	ctx context.Context,
	proposal permissions.ProposedCall,
) ([]byte, error) {
	if executor == nil || ctx == nil {
		return nil, execution.ErrUnsupportedTool
	}
	executor.mu.RLock()
	if executor.closed {
		executor.mu.RUnlock()
		return nil, execution.ErrUnsupportedTool
	}
	members := append([]execution.RemoteToolExecutor(nil), executor.executors...)
	lifecycle := executor.lifecycle
	executor.mu.RUnlock()
	for _, member := range members {
		if err := member.ValidateProposal(proposal); err != nil {
			continue
		}
		callContext, cancel := context.WithCancelCause(ctx)
		stop := context.AfterFunc(lifecycle, func() {
			cancel(context.Cause(lifecycle))
		})
		content, err := member.ExecuteProposalContent(callContext, proposal)
		stop()
		cancel(nil)
		if err != nil {
			return nil, err
		}
		if cause := context.Cause(lifecycle); cause != nil {
			for index := range content {
				content[index] = 0
			}
			return nil, execution.ErrUnsupportedTool
		}
		return content, nil
	}
	return nil, execution.ErrUnsupportedTool
}

// Close cancels the shared lifecycle and zeroes the member set. It is safe to
// call multiple times.
func (executor *productCompositeRemoteToolExecutor) Close() error {
	if executor == nil {
		return nil
	}
	executor.mu.Lock()
	if executor.closed {
		executor.mu.Unlock()
		return nil
	}
	executor.closed = true
	executor.cancel(execution.ErrUnsupportedTool)
	executor.executors = nil
	executor.mu.Unlock()
	return nil
}

func productRemoteToolEnrollmentDeps(
	config *productRemoteToolBrokerConfig,
) enrollment.MaterializeDeps {
	deps := enrollment.MaterializeDeps{}
	if config != nil {
		deps.Search = config.Search
		deps.MCPClients = config.MCPClients
	}
	return deps
}

// newProductRemoteToolExecutorsFromEnrollments materializes every persisted
// active + policy-current Enrollment in the trusted built-in catalog through
// the Work Bundle materialization boundary. Revoked, policy-drifted,
// unsupported-adapter and port-less Enrollments are skipped (their bound
// Agents are already blocked by per-Agent preflight) and never produce a
// capability. It returns nil when nothing materializes, so default production
// without injected ports still exposes no remote tool capability.
// productDynamicRemoteToolExecutor re-materializes the persisted active +
// policy-current Enrollment set from the live projection on every operation,
// so an Enrollment configured after daemon startup is picked up without a
// restart. Materialization is cheap: members share the injected Search/MCP
// ports. Fail-closed per Enrollment (unsupported adapter, missing port,
// invalid limits) and fail-closed as a whole when nothing materializes.
type productDynamicRemoteToolExecutor struct {
	mu       sync.RWMutex
	view     func() projection.GlobalReadView
	deps     enrollment.MaterializeDeps
	lifecycle context.Context
	cancel    context.CancelCauseFunc
	closed    bool
}

func newProductDynamicRemoteToolExecutor(
	view func() projection.GlobalReadView,
	deps enrollment.MaterializeDeps,
) (*productDynamicRemoteToolExecutor, error) {
	if view == nil {
		return nil, execution.ErrUnsupportedTool
	}
	lifecycle, cancel := context.WithCancelCause(context.Background())
	return &productDynamicRemoteToolExecutor{
		view: view, deps: deps, lifecycle: lifecycle, cancel: cancel,
	}, nil
}

func (executor *productDynamicRemoteToolExecutor) materialize() []execution.RemoteToolExecutor {
	if executor == nil {
		return nil
	}
	executor.mu.RLock()
	closed := executor.closed
	executor.mu.RUnlock()
	if closed {
		return nil
	}
	view := executor.view()
	materialized := make([]execution.RemoteToolExecutor, 0, 2)
	for _, candidate := range view.RemoteToolBackendEnrollmentCatalog() {
		if candidate.Status() != work.RemoteToolBackendEnrollmentActive {
			continue
		}
		policy, ok := view.ProviderAccountPolicy(
			candidate.ProviderID(), candidate.ProviderAccountID(),
		)
		if !ok || !remoteToolEnrollmentPolicyCurrent(candidate, policy) {
			continue
		}
		member, err := enrollment.Materialize(candidate, true, executor.deps)
		if err != nil {
			continue
		}
		materialized = append(materialized, member)
	}
	return materialized
}

func (executor *productDynamicRemoteToolExecutor) AllowedRemoteTools() []permissions.ToolKind {
	allowed := make([]permissions.ToolKind, 0, 4)
	seen := make(map[permissions.ToolKind]struct{})
	for _, member := range executor.materialize() {
		for _, kind := range member.AllowedRemoteTools() {
			if _, exists := seen[kind]; exists {
				continue
			}
			seen[kind] = struct{}{}
			allowed = append(allowed, kind)
		}
	}
	return allowed
}

func (executor *productDynamicRemoteToolExecutor) ValidateProposal(
	proposal permissions.ProposedCall,
) error {
	if executor == nil {
		return execution.ErrUnsupportedTool
	}
	var lastErr error
	for _, member := range executor.materialize() {
		if err := member.ValidateProposal(proposal); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		return execution.ErrUnsupportedTool
	}
	return errors.Join(execution.ErrUnsupportedTool, lastErr)
}

func (executor *productDynamicRemoteToolExecutor) ExecuteProposalContent(
	ctx context.Context,
	proposal permissions.ProposedCall,
) ([]byte, error) {
	if executor == nil || ctx == nil {
		return nil, execution.ErrUnsupportedTool
	}
	executor.mu.RLock()
	if executor.closed {
		executor.mu.RUnlock()
		return nil, execution.ErrUnsupportedTool
	}
	lifecycle := executor.lifecycle
	executor.mu.RUnlock()
	for _, member := range executor.materialize() {
		if err := member.ValidateProposal(proposal); err != nil {
			continue
		}
		callContext, cancel := context.WithCancelCause(ctx)
		stop := context.AfterFunc(lifecycle, func() {
			cancel(context.Cause(lifecycle))
		})
		content, err := member.ExecuteProposalContent(callContext, proposal)
		stop()
		cancel(nil)
		if err != nil {
			return nil, err
		}
		if cause := context.Cause(lifecycle); cause != nil {
			for index := range content {
				content[index] = 0
			}
			return nil, execution.ErrUnsupportedTool
		}
		return content, nil
	}
	return nil, execution.ErrUnsupportedTool
}

func (executor *productDynamicRemoteToolExecutor) Close() error {
	if executor == nil {
		return nil
	}
	executor.mu.Lock()
	defer executor.mu.Unlock()
	if executor.closed {
		return nil
	}
	executor.closed = true
	executor.cancel(execution.ErrUnsupportedTool)
	return nil
}

func newProductRemoteToolExecutorsFromEnrollments(
	view func() projection.GlobalReadView,
	deps enrollment.MaterializeDeps,
) (execution.RemoteToolExecutor, error) {
	return newProductDynamicRemoteToolExecutor(view, deps)
}

func remoteToolEnrollmentPolicyCurrent(
	enrollment work.RemoteToolBackendEnrollment,
	policy work.ProviderAccountPolicy,
) bool {
	return policy.Valid() &&
		enrollment.ProviderID() == policy.ProviderID() &&
		enrollment.ProviderAccountID() == policy.ProviderAccountID() &&
		enrollment.ProviderAccountPolicyVersion() == policy.Version() &&
		enrollment.ProviderAccountPolicyRevision() == policy.Revision() &&
		enrollment.ProviderAccountPolicyDigest() == policy.Digest()
}
