package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/rules"
)

const adapterPrefix = "exec1"

// Adapter is the single execution authority. It evaluates every proposal with
// the permissions pipeline, writes Journal facts through CAS, executes only
// after an allow verdict, and publishes digest-only evidence.
type Adapter struct {
	store     *journal.Store
	evidence  *evidence.Store
	executor  Executor
	resolver  WorktreeResolver
	approvals ApprovalRequester
	consumer  ApprovalConsumer
	decisions DecisionRecorder
	sandbox   SandboxGate
	remote    RemoteToolExecutor
	now       func() time.Time

	mu        sync.Mutex
	inflight  map[string]bool
	completed map[string]ExecutionResult
}

// WithRemoteToolExecutor is a composition-time option. Call it before exposing
// the Adapter to concurrent requests; the remote broker is immutable at run time.
func (a *Adapter) WithRemoteToolExecutor(executor RemoteToolExecutor) *Adapter {
	if a != nil {
		a.remote = executor
	}
	return a
}

func (a *Adapter) RemoteToolsEnabled() bool {
	return len(a.RemoteToolKinds()) > 0
}

func (a *Adapter) RemoteToolKinds() []permissions.ToolKind {
	if a == nil || a.remote == nil {
		return nil
	}
	allowed := a.remote.AllowedRemoteTools()
	result := make([]permissions.ToolKind, 0, len(allowed))
	seen := make(map[permissions.ToolKind]bool, len(allowed))
	for _, tool := range allowed {
		if seen[tool] ||
			tool != permissions.ToolWebSearch &&
				tool != permissions.ToolWebFetch &&
				tool != permissions.ToolMCPTool {
			return nil
		}
		seen[tool] = true
		result = append(result, tool)
	}
	return result
}

func (a *Adapter) remoteToolEnabled(wanted permissions.ToolKind) bool {
	for _, tool := range a.RemoteToolKinds() {
		if tool == wanted {
			return true
		}
	}
	return false
}

// WithSandboxGate wires the Phase 3B sandbox policy gate. A nil gate (default)
// means no sandbox requirement; a non-nil gate enforces Required => fail closed.
func (a *Adapter) WithSandboxGate(gate SandboxGate) *Adapter {
	if a != nil {
		a.sandbox = gate
	}
	return a
}

func NewAdapter(
	store *journal.Store,
	evidenceStore *evidence.Store,
	executor Executor,
	resolver WorktreeResolver,
	approvals ApprovalRequester,
	decisions DecisionRecorder,
	now func() time.Time,
) (*Adapter, error) {
	if store == nil || evidenceStore == nil || executor == nil || resolver == nil || now == nil {
		return nil, ErrInvalidExecutionInput
	}
	adapter := &Adapter{
		store: store, evidence: evidenceStore, executor: executor,
		resolver: resolver, approvals: approvals, decisions: decisions,
		now: now, inflight: make(map[string]bool), completed: make(map[string]ExecutionResult),
	}
	if consumer, ok := approvals.(ApprovalConsumer); ok {
		adapter.consumer = consumer
	}
	return adapter, nil
}

// Execute proposes and (only when allow) executes one tool call. ask and
// deny always have zero side effects. Idempotent replay of the same
// operation returns the existing result or pending ask without re-executing.
func (a *Adapter) Execute(ctx context.Context, proposal Proposal) (ExecutionResult, error) {
	if a == nil || a.store == nil {
		return ExecutionResult{}, ErrInvalidExecutionInput
	}
	if proposal.JobID == "" || proposal.OperationID == "" ||
		!permissions.ValidToolKind(string(proposal.Call.Tool)) {
		return ExecutionResult{}, ErrInvalidExecutionInput
	}
	callDigest := callDigest(proposal.Call)
	executionID := executionID(proposal.JobID, callDigest, proposal.OperationID)

	a.mu.Lock()
	if result, ok := a.completed[executionID]; ok {
		a.mu.Unlock()
		return a.restoreReadOnlyContent(ctx, proposal, result)
	}
	if a.inflight[executionID] {
		a.mu.Unlock()
		return ExecutionResult{}, ErrExecutionInFlight
	}
	a.inflight[executionID] = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.inflight, executionID)
		a.mu.Unlock()
	}()

	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return ExecutionResult{}, err
	}
	permissionProjection, err := permissions.Replay(events)
	if err != nil {
		return ExecutionResult{}, err
	}
	snapshot, err := ReplaySnapshot(events)
	if err != nil {
		return ExecutionResult{}, err
	}
	existing, found := snapshot.Record(executionID)
	if found && isTerminal(existing) {
		result := resultFromRecord(existing)
		a.remember(executionID, result)
		return a.restoreReadOnlyContent(ctx, proposal, result)
	}
	if found && existing.AllowedAt != "" {
		// An Allowed fact with no terminal fact means the side effect may
		// already have happened. Re-executing would violate exactly-once;
		// ReplayPending is the only path that terminalizes it, and it never
		// executes.
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageRecovery,
			a.now().UTC(), ToolDiagnosticFailed, "side_effect_unknown", false,
		)
		return ExecutionResult{}, ErrExecutionInterrupted
	}

	authorizationStarted := a.now().UTC()
	effective, profileErr := permissions.ResolveEffectiveProfile(permissionProjection, proposal.JobID)
	verdict, denial, evalErr := permissions.VerdictDeny, permissions.Denial{}, error(nil)
	generation := int64(0)
	if profileErr == nil {
		generation = effective.Profile.Generation
		verdict, denial, evalErr = permissions.Evaluate(effective, proposal.Call)
	}
	if profileErr != nil {
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageAuthorization,
			authorizationStarted, ToolDiagnosticFailed, "permission_profile_unavailable", false,
		)
		return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
			Reason:            profileErr.Error(),
			AuthorizationPath: "bind a valid permission profile or fix the stale binding",
		})
	}
	if evalErr != nil {
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageAuthorization,
			authorizationStarted, ToolDiagnosticFailed, "permission_evaluation_failed", false,
		)
		return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
			Reason:            evalErr.Error(),
			AuthorizationPath: "fix the proposed call",
		})
	}

	switch verdict {
	case permissions.VerdictDeny:
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageAuthorization,
			authorizationStarted, ToolDiagnosticFailed, "permission_denied", false,
		)
		return a.deny(ctx, proposal, executionID, callDigest, generation, events, denial)
	case permissions.VerdictAsk:
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageAuthorization,
			authorizationStarted, ToolDiagnosticSucceeded, "", false,
		)
		return a.ask(ctx, proposal, executionID, callDigest, generation, events, denial, found)
	case permissions.VerdictAllow:
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageAuthorization,
			authorizationStarted, ToolDiagnosticSucceeded, "", false,
		)
		return a.allowAndExecute(
			ctx, proposal, executionID, callDigest, generation, events,
			existing, found, effective, "", "",
		)
	default:
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageAuthorization,
			authorizationStarted, ToolDiagnosticFailed, "permission_verdict_invalid", false,
		)
		return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
			Reason:            "unknown verdict",
			AuthorizationPath: "fix the permission pipeline",
		})
	}
}

func (a *Adapter) deny(
	ctx context.Context,
	proposal Proposal,
	executionID, callDigest string,
	generation int64,
	events []journal.Event,
	denial permissions.Denial,
) (ExecutionResult, error) {
	streamID := executionStreamID(proposal.JobID, executionID)
	snapshot, _ := ReplaySnapshot(events)
	existing, found := snapshot.Record(executionID)
	heads := streamHeads(events)
	now := a.now().UTC()
	var batch []journal.Event
	if !found || existing.Status == "" {
		proposed, err := a.buildEvent("proposed", streamID, proposal.JobID, executionID, callDigest,
			proposal, generation, now, proposal.JourneyID)
		if err != nil {
			return ExecutionResult{}, err
		}
		batch = append(batch, proposed)
	}
	if !found || (existing.Status != "denied" && existing.DeniedAt == "") {
		denied, err := a.buildDeniedEvent(streamID, executionID, denial, now)
		if err != nil {
			return ExecutionResult{}, err
		}
		batch = append(batch, denied)
	}
	if len(batch) > 0 {
		committed, err := a.appendCAS(ctx, events, heads, streamID, batch)
		if err != nil {
			return ExecutionResult{}, err
		}
		for _, event := range committed {
			_ = event
		}
	}
	result := ExecutionResult{
		ExecutionID: executionID, JobID: proposal.JobID,
		Verdict: permissions.VerdictDeny, Denial: denial,
		Note: "denied without execution",
	}
	a.remember(executionID, result)
	return result, nil
}

func (a *Adapter) ask(
	ctx context.Context,
	proposal Proposal,
	executionID, callDigest string,
	generation int64,
	events []journal.Event,
	denial permissions.Denial,
	existingFound bool,
) (ExecutionResult, error) {
	streamID := executionStreamID(proposal.JobID, executionID)
	heads := streamHeads(events)
	now := a.now().UTC()
	if !existingFound {
		proposed, err := a.buildEvent("proposed", streamID, proposal.JobID, executionID, callDigest,
			proposal, generation, now, proposal.JourneyID)
		if err != nil {
			return ExecutionResult{}, err
		}
		committed, err := a.appendCAS(ctx, events, heads, streamID, []journal.Event{proposed})
		if err != nil {
			return ExecutionResult{}, err
		}
		events = append(events, committed...)
		// The Proposed fact is now committed; the resume path must append only
		// Allowed + terminal facts (no duplicate Proposed => no partial batch
		// conflict).
		existingFound = true
	}
	if approval, found := existingApproval(events, proposal.JobID, callDigest); found {
		if approval.status == "consumed" {
			return ExecutionResult{}, ErrApprovalConsumed
		}
		if approval.status == "approved" {
			// Approval exists for this exact (job, call). The approval is the
			// one-time authorization for this call: resume executes when the
			// re-check yields allow OR ask (approved overrides the ask), and
			// still denies when the re-check yields deny or errors.
			permissionProjection, err := permissions.Replay(events)
			if err != nil {
				return ExecutionResult{}, err
			}
			effective, err := permissions.ResolveEffectiveProfile(permissionProjection, proposal.JobID)
			if err == nil {
				verdict, resumeDenial, evalErr := permissions.Evaluate(effective, proposal.Call)
				if evalErr == nil &&
					(verdict == permissions.VerdictAllow || verdict == permissions.VerdictAsk) {
					snapshot, _ := ReplaySnapshot(events)
					existing, _ := snapshot.Record(executionID)
					if existing.Status == "" {
						existing.Status = string(EventToolProposed)
					}
					if a.consumer == nil {
						return ExecutionResult{}, ErrApprovalConsumed
					}
					if _, consumeErr := a.consumer.ConsumePermissionApproval(
						ctx,
						rules.PermissionApprovalConsumptionInput{
							ApprovalID: approval.id, ApprovalDigest: approval.digest,
							JobID: proposal.JobID, CallDigest: callDigest,
							ConsumerID: executionID, OperationID: proposal.OperationID,
							CorrelationID: proposal.JourneyID,
						},
					); consumeErr != nil {
						if errors.Is(consumeErr, rules.ErrPermissionApprovalConsumed) {
							return ExecutionResult{}, ErrApprovalConsumed
						}
						return ExecutionResult{}, consumeErr
					}
					result, resumeErr := a.allowAndExecute(
						ctx, proposal, executionID, callDigest, generation,
						events, existing, existingFound, effective,
						approval.id, approval.digest,
					)
					if resumeErr != nil {
						return ExecutionResult{}, resumeErr
					}
					result.ApprovalID = approval.id
					result.ApprovalDigest = approval.digest
					a.remember(executionID, result)
					return result, nil
				}
				if evalErr != nil {
					return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
						Reason: evalErr.Error(), AuthorizationPath: "fix the proposed call",
					})
				}
				return a.deny(ctx, proposal, executionID, callDigest, generation, events, resumeDenial)
			}
		}
		if approval.status == "rejected" {
			return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
				Reason: "approval was rejected", AuthorizationPath: "request a new grant or allow rule",
			})
		}
		return ExecutionResult{
			ExecutionID: executionID, JobID: proposal.JobID,
			Verdict: permissions.VerdictAsk, Denial: denial,
			ApprovalID: approval.id, ApprovalDigest: approval.digest,
			Note: "approval pending; replay the same operation after resolution",
		}, nil
	}
	if a.decisions != nil {
		if err := a.decisions.RecordDecision(
			ctx, proposal.JobID, proposal.Call, permissions.VerdictAsk, denial, "",
			proposal.OperationID, proposal.JourneyID,
		); err != nil {
			return ExecutionResult{}, err
		}
	}
	if a.approvals == nil {
		return ExecutionResult{
			ExecutionID: executionID, JobID: proposal.JobID,
			Verdict: permissions.VerdictAsk, Denial: denial,
			Note: "approval required; resolution is forwarded to the existing rules authority",
		}, nil
	}
	record, err := a.approvals.RequestPermissionApproval(ctx, rules.PermissionApprovalInput{
		JobID: proposal.JobID, CallDigest: callDigest,
		Tool: string(proposal.Call.Tool), Command: proposal.Call.Command,
		Path: proposal.Call.Path, Reason: denial.Reason,
		RequestedAt: now, CorrelationID: proposal.JourneyID,
	})
	if err != nil {
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageApprovalWait,
			now, ToolDiagnosticFailed, "approval_unavailable", true,
		)
		return ExecutionResult{}, err
	}
	a.recordToolDiagnostic(
		ctx, proposal, executionID, callDigest, ToolStageApprovalWait,
		now, ToolDiagnosticSucceeded, "", false,
	)
	return ExecutionResult{
		ExecutionID: executionID, JobID: proposal.JobID,
		Verdict: permissions.VerdictAsk, Denial: denial,
		ApprovalID: record.ID(), ApprovalDigest: record.Digest(),
		Note: "approval required; replay the same operation after resolution",
	}, nil
}

func (a *Adapter) allowAndExecute(
	ctx context.Context,
	proposal Proposal,
	executionID, callDigest string,
	generation int64,
	events []journal.Event,
	existing ExecutionRecord,
	existingFound bool,
	effective permissions.EffectiveProfile,
	approvalID string,
	approvalDigest string,
) (ExecutionResult, error) {
	streamID := executionStreamID(proposal.JobID, executionID)
	heads := streamHeads(events)
	now := a.now().UTC()
	if !existingFound || existing.Status == "" {
		proposed, err := a.buildEvent("proposed", streamID, proposal.JobID, executionID, callDigest,
			proposal, generation, now, proposal.JourneyID)
		if err != nil {
			return ExecutionResult{}, err
		}
		allowed, err := a.buildAllowedEvent(streamID, executionID, now)
		if err != nil {
			return ExecutionResult{}, err
		}
		committed, err := a.appendCAS(ctx, events, heads, streamID, []journal.Event{proposed, allowed})
		if err != nil {
			return ExecutionResult{}, err
		}
		events = append(events, committed...)
	} else if existing.AllowedAt == "" {
		allowed, err := a.buildAllowedEvent(streamID, executionID, now)
		if err != nil {
			return ExecutionResult{}, err
		}
		committed, err := a.appendCAS(ctx, events, heads, streamID, []journal.Event{allowed})
		if err != nil {
			return ExecutionResult{}, err
		}
		events = append(events, committed...)
	}
	result, executeErr := a.executeApproved(
		ctx, proposal, executionID, callDigest, generation, events, effective,
		approvalID, approvalDigest,
	)
	if executeErr != nil {
		return ExecutionResult{}, executeErr
	}
	return result, nil
}

func (a *Adapter) executeApproved(
	ctx context.Context,
	proposal Proposal,
	executionID, callDigest string,
	generation int64,
	events []journal.Event,
	effective permissions.EffectiveProfile,
	approvalID, approvalDigest string,
) (ExecutionResult, error) {
	sandboxStarted := a.now().UTC()
	// Phase 3B sandbox policy gate: Required + unavailable backend => fail
	// closed with zero side effects (never local fallback).
	if a.sandbox != nil {
		policy, policyErr := a.sandbox.ResolvePolicy(ctx, proposal.JobID)
		if policyErr != nil {
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageSandboxPrepare,
				sandboxStarted, ToolDiagnosticFailed, "sandbox_policy_unavailable", false,
			)
			return a.deny(ctx, proposal, executionID, callDigest, generation, events,
				permissions.Denial{Reason: "sandbox policy error"})
		}
		if policy.Required {
			available, availErr := a.sandbox.BackendAvailable(ctx, policy.Backend)
			if availErr != nil || !available {
				a.recordToolDiagnostic(
					ctx, proposal, executionID, callDigest, ToolStageSandboxPrepare,
					sandboxStarted, ToolDiagnosticFailed, "sandbox_unavailable", false,
				)
				return a.deny(ctx, proposal, executionID, callDigest, generation, events,
					permissions.Denial{
						Reason:            "required sandbox backend unavailable",
						AuthorizationPath: "configure the sandbox backend",
					})
			}
		}
	}
	a.recordToolDiagnostic(
		ctx, proposal, executionID, callDigest, ToolStageSandboxPrepare,
		sandboxStarted, ToolDiagnosticSucceeded, "", false,
	)
	bindingStarted := a.now().UTC()
	// Remote (web/MCP) tool calls are brokered by the remote executor and do
	// not touch the local worktree; only workspace-bound tools need it.
	needsWorktree := proposal.Call.Tool != permissions.ToolWebSearch &&
		proposal.Call.Tool != permissions.ToolWebFetch &&
		proposal.Call.Tool != permissions.ToolMCPTool
	var worktree string
	if needsWorktree {
		resolved, resolveErr := a.resolver.Resolve(ctx, proposal.JobID)
		if resolveErr != nil {
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageBindingValidation,
				bindingStarted, ToolDiagnosticFailed, "worktree_unavailable", true,
			)
			return a.fail(ctx, proposal, executionID, callDigest, generation, events, "worktree resolution failed", "worktree_unavailable")
		}
		worktree = resolved
	}
	var editContent []byte
	switch proposal.Call.Tool {
	case permissions.ToolEdit:
		if !pathAllowed(effective, proposal.Call.Path) {
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageBindingValidation,
				bindingStarted, ToolDiagnosticFailed, "path_outside_owned_scope", false,
			)
			return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
				Reason:            "edit path is not inside profile owned paths",
				AuthorizationPath: "narrow the owned paths or add an allow rule",
			})
		}
		content, readErr := a.readProposalContent(proposal)
		if readErr != nil {
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageBindingValidation,
				bindingStarted, ToolDiagnosticFailed, "invalid_edit_content", false,
			)
			return a.fail(ctx, proposal, executionID, callDigest, generation, events, readErr.Error(), "invalid_edit_content")
		}
		editContent = content
	case permissions.ToolRead, permissions.ToolGrep:
		if _, pathErr := secureRelativePath(proposal.Call.Path); pathErr != nil {
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageBindingValidation,
				bindingStarted, ToolDiagnosticFailed, "path_outside_workspace", false,
			)
			return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
				Reason:            "read path is outside the workspace",
				AuthorizationPath: "use a workspace-relative path",
			})
		}
		if proposal.Call.Tool == permissions.ToolGrep {
			if _, patternErr := validateGrepPattern(proposal.Call.Pattern); patternErr != nil {
				a.recordToolDiagnostic(
					ctx, proposal, executionID, callDigest, ToolStageBindingValidation,
					bindingStarted, ToolDiagnosticFailed, "invalid_grep_pattern", false,
				)
				return a.fail(
					ctx, proposal, executionID, callDigest, generation, events,
					"invalid Grep pattern", "invalid_grep_pattern",
				)
			}
		}
	case permissions.ToolWebSearch, permissions.ToolWebFetch, permissions.ToolMCPTool:
		if !a.remoteToolEnabled(proposal.Call.Tool) || proposal.ResultCommitGate == nil {
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageBindingValidation,
				bindingStarted, ToolDiagnosticFailed, "remote_tool_unavailable", false,
			)
			return a.fail(
				ctx, proposal, executionID, callDigest, generation, events,
				"remote tool unavailable", "remote_tool_unavailable",
			)
		}
		if validateErr := a.remote.ValidateProposal(proposal.Call); validateErr != nil {
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageBindingValidation,
				bindingStarted, ToolDiagnosticFailed, "invalid_remote_tool_call", false,
			)
			return a.fail(
				ctx, proposal, executionID, callDigest, generation, events,
				"invalid remote tool call", "invalid_remote_tool_call",
			)
		}
	case permissions.ToolBash:
	default:
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageBindingValidation,
			bindingStarted, ToolDiagnosticFailed, "unsupported_tool", false,
		)
		return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
			Reason:            "tool is not executable by the adapter",
			AuthorizationPath: "use Edit or Bash through the execution adapter",
		})
	}
	a.recordToolDiagnostic(
		ctx, proposal, executionID, callDigest, ToolStageBindingValidation,
		bindingStarted, ToolDiagnosticSucceeded, "", false,
	)
	if proposal.DispatchGate != nil {
		dispatchStarted := a.now().UTC()
		if dispatchErr := proposal.DispatchGate.CommitExecutionDispatch(
			ctx,
			ExecutionDispatchInput{
				JobID: proposal.JobID, ExecutionID: executionID,
				CallDigest: callDigest, Tool: proposal.Call.Tool,
				OperationID: proposal.OperationID, CorrelationID: proposal.JourneyID,
				ApprovalID: approvalID, ApprovalDigest: approvalDigest,
			},
		); dispatchErr != nil {
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageDispatch,
				dispatchStarted, ToolDiagnosticFailed, "dispatch_not_committed", false,
			)
			return a.fail(
				ctx, proposal, executionID, callDigest, generation, events,
				"dispatch admission failed", "dispatch_not_committed",
			)
		}
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageDispatch,
			dispatchStarted, ToolDiagnosticSucceeded, "", false,
		)
	}
	started := a.now().UTC()
	var (
		result              ExecutionResult
		runErr              error
		retainResultContent bool
	)
	defer func() {
		if !retainResultContent {
			result.Close()
		}
	}()
	switch proposal.Call.Tool {
	case permissions.ToolEdit:
		contentSum := sha256.Sum256(editContent)
		editResult, editErr := a.executor.Edit(ctx, EditRequest{
			Worktree: worktree, RelativePath: proposal.Call.Path,
			NewContentDigest: hex.EncodeToString(contentSum[:]), NewContentBytes: editContent,
		})
		if editErr != nil {
			code := "edit_failed"
			if errors.Is(editErr, ErrExecutionLimit) {
				code = "limit_exceeded"
			}
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageResultValidation,
				started, ToolDiagnosticFailed, code, errors.Is(editErr, ErrExecutionLimit),
			)
			return a.fail(ctx, proposal, executionID, callDigest, generation, events, editErr.Error(), code)
		}
		result = ExecutionResult{
			ExecutionID: executionID, JobID: proposal.JobID,
			Verdict:            permissions.VerdictAllow,
			ChangedFilesDigest: editResult.ChangedFilesDigest,
			Note:               "edit applied",
		}
	case permissions.ToolBash:
		runResult, runErr := a.executor.Run(ctx, RunRequest{
			Worktree: worktree, Command: proposal.Call.Command,
		})
		if runErr != nil {
			code := "run_failed"
			if errors.Is(runErr, ErrExecutionLimit) {
				code = "limit_exceeded"
			}
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageResultValidation,
				started, ToolDiagnosticFailed, code, errors.Is(runErr, ErrExecutionLimit),
			)
			return a.fail(ctx, proposal, executionID, callDigest, generation, events, runErr.Error(), code)
		}
		result = ExecutionResult{
			ExecutionID: executionID, JobID: proposal.JobID,
			Verdict: permissions.VerdictAllow, ExitCode: runResult.ExitCode,
			OutputDigest:       runResult.OutputDigest,
			ChangedFilesDigest: runResult.ChangedFilesDigest,
			DurationMS:         runResult.DurationMS,
			Note:               "command completed",
		}
	case permissions.ToolRead:
		executor, ok := a.executor.(ReadExecutor)
		if !ok {
			return a.unsupportedTool(
				ctx, proposal, executionID, callDigest, generation, events, started,
			)
		}
		readResult, readErr := executor.Read(ctx, ReadRequest{
			Worktree: worktree, RelativePath: proposal.Call.Path,
		})
		if readErr != nil {
			readResult.Close()
			return a.readOnlyToolFailure(
				ctx, proposal, executionID, callDigest, generation, events, started, readErr,
			)
		}
		result = ExecutionResult{
			ExecutionID: executionID, JobID: proposal.JobID,
			Verdict: permissions.VerdictAllow, OutputDigest: readResult.ContentDigest,
			DurationMS: readResult.DurationMS, Note: "read completed",
			Content: readResult.Content,
		}
	case permissions.ToolGrep:
		executor, ok := a.executor.(GrepExecutor)
		if !ok {
			return a.unsupportedTool(
				ctx, proposal, executionID, callDigest, generation, events, started,
			)
		}
		grepResult, grepErr := executor.Grep(ctx, GrepRequest{
			Worktree: worktree, RelativePath: proposal.Call.Path,
			Pattern: proposal.Call.Pattern,
		})
		if grepErr != nil {
			grepResult.Close()
			return a.readOnlyToolFailure(
				ctx, proposal, executionID, callDigest, generation, events, started, grepErr,
			)
		}
		result = ExecutionResult{
			ExecutionID: executionID, JobID: proposal.JobID,
			Verdict: permissions.VerdictAllow, OutputDigest: grepResult.ContentDigest,
			DurationMS: grepResult.DurationMS, Note: "grep completed",
			Content: grepResult.Content,
		}
	case permissions.ToolWebSearch, permissions.ToolWebFetch, permissions.ToolMCPTool:
		content, remoteErr := a.remote.ExecuteProposalContent(ctx, proposal.Call)
		if remoteErr != nil {
			zeroExecutionBytes(content)
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageResultValidation,
				started, ToolDiagnosticFailed, "remote_tool_failed", true,
			)
			return a.fail(
				ctx, proposal, executionID, callDigest, generation, events,
				"remote tool failed", "remote_tool_failed",
			)
		}
		if len(content) == 0 || len(content) > piCompatibleRemoteResultLimit ||
			!utf8.Valid(content) || executionTextHasUnsafeControls(content) {
			zeroExecutionBytes(content)
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStageResultValidation,
				started, ToolDiagnosticFailed, "remote_result_invalid", false,
			)
			return a.fail(
				ctx, proposal, executionID, callDigest, generation, events,
				"remote result invalid", "remote_result_invalid",
			)
		}
		result = ExecutionResult{
			ExecutionID: executionID, JobID: proposal.JobID,
			Verdict: permissions.VerdictAllow, OutputDigest: digestBytes(content),
			DurationMS: a.now().UTC().Sub(started).Milliseconds(),
			Note:       "remote tool completed", Content: content,
		}
	}
	a.recordToolDiagnostic(
		ctx, proposal, executionID, callDigest, ToolStageResultValidation,
		started, ToolDiagnosticSucceeded, "", false,
	)
	_ = runErr
	if proposal.Call.Tool == permissions.ToolWebSearch ||
		proposal.Call.Tool == permissions.ToolWebFetch ||
		proposal.Call.Tool == permissions.ToolMCPTool {
		commitStarted := a.now().UTC()
		if commitErr := proposal.ResultCommitGate.CommitExecutionResult(
			ctx,
			ExecutionResultCommitInput{
				JobID: proposal.JobID, ExecutionID: executionID,
				CallDigest: callDigest, Tool: proposal.Call.Tool,
				OperationID: proposal.OperationID, CorrelationID: proposal.JourneyID,
				OutputDigest: result.OutputDigest, DurationMS: result.DurationMS,
				Content: result.Content,
			},
		); commitErr != nil {
			a.recordToolDiagnostic(
				ctx, proposal, executionID, callDigest, ToolStagePayloadCommit,
				commitStarted, ToolDiagnosticFailed, "result_persistence_failed", false,
			)
			return a.fail(
				ctx, proposal, executionID, callDigest, generation, events,
				"remote result persistence failed", "result_persistence_failed",
			)
		}
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStagePayloadCommit,
			commitStarted, ToolDiagnosticSucceeded, "", false,
		)
	}
	// Evidence metadata is digest-only, never raw output or secrets.
	metadata := map[string]any{
		"execution_id":         executionID,
		"job_id":               proposal.JobID,
		"tool":                 proposal.Call.Tool,
		"exit_code":            result.ExitCode,
		"duration_ms":          result.DurationMS,
		"output_digest":        result.OutputDigest,
		"changed_files_digest": result.ChangedFilesDigest,
		"evidence_kind":        "execution-metadata",
		"created_at":           started.Format(time.RFC3339Nano),
	}
	body, marshalErr := json.Marshal(metadata)
	if marshalErr != nil {
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageResultCommit,
			started, ToolDiagnosticFailed, "evidence_failed", false,
		)
		return a.fail(ctx, proposal, executionID, callDigest, generation, events, "evidence marshal failed", "evidence_failed")
	}
	sum := sha256.Sum256(body)
	expected := hex.EncodeToString(sum[:])
	artifact, publishErr := a.evidence.Publish(ctx, bytes.NewReader(body), expected)
	if publishErr != nil {
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageResultCommit,
			started, ToolDiagnosticFailed, "evidence_failed", true,
		)
		return a.fail(ctx, proposal, executionID, callDigest, generation, events, "evidence publish failed", "evidence_failed")
	}
	result.EvidenceID = artifact.Digest
	completedAt := a.now().UTC()
	streamID := executionStreamID(proposal.JobID, executionID)
	completed, err := a.buildCompletedEvent(streamID, executionID, result, started, completedAt)
	if err != nil {
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageResultCommit,
			started, ToolDiagnosticFailed, "result_commit_failed", false,
		)
		return ExecutionResult{}, err
	}
	heads := streamHeads(events)
	if _, err := a.appendCAS(ctx, events, heads, streamID, []journal.Event{completed}); err != nil {
		a.recordToolDiagnostic(
			ctx, proposal, executionID, callDigest, ToolStageResultCommit,
			started, ToolDiagnosticFailed, "result_commit_failed", true,
		)
		return ExecutionResult{}, err
	}
	a.recordToolDiagnostic(
		ctx, proposal, executionID, callDigest, ToolStageResultCommit,
		started, ToolDiagnosticSucceeded, "", false,
	)
	if approvalID != "" {
		result.ApprovalID = approvalID
		result.ApprovalDigest = approvalDigest
	}
	a.remember(executionID, result)
	retainResultContent = true
	return result, nil
}

func (a *Adapter) fail(
	ctx context.Context,
	proposal Proposal,
	executionID, callDigest string,
	generation int64,
	events []journal.Event,
	reason, errorCode string,
) (ExecutionResult, error) {
	streamID := executionStreamID(proposal.JobID, executionID)
	heads := streamHeads(events)
	now := a.now().UTC()
	snapshot, _ := ReplaySnapshot(events)
	existing, found := snapshot.Record(executionID)
	var batch []journal.Event
	if !found || existing.Status == "" {
		proposed, err := a.buildEvent("proposed", streamID, proposal.JobID, executionID, callDigest,
			proposal, generation, now, proposal.JourneyID)
		if err != nil {
			return ExecutionResult{}, err
		}
		batch = append(batch, proposed)
	}
	failed, err := a.buildFailedEvent(streamID, executionID, reason, errorCode, now)
	if err != nil {
		return ExecutionResult{}, err
	}
	batch = append(batch, failed)
	if _, err := a.appendCAS(ctx, events, heads, streamID, batch); err != nil {
		return ExecutionResult{}, err
	}
	result := ExecutionResult{
		ExecutionID: executionID, JobID: proposal.JobID,
		Verdict: permissions.VerdictDeny, Note: errorCode, ErrorCode: errorCode,
	}
	a.remember(executionID, result)
	return result, nil
}

// ReplayPending terminalizes every execution stream that has a Proposed or
// Allowed fact but no terminal fact. It writes facts only and never executes.
func (a *Adapter) ReplayPending(ctx context.Context) error {
	if a == nil || a.store == nil {
		return ErrInvalidExecutionInput
	}
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return err
	}
	heads := streamHeads(events)
	streams := make(map[string]string) // streamID -> executionID
	for _, event := range events {
		if !strings.HasPrefix(event.StreamID, executionStreamPrefix) {
			continue
		}
		switch event.Type {
		case EventToolProposed, EventToolAllowed:
			var payload struct {
				ExecutionID string `json:"execution_id"`
			}
			if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil || payload.ExecutionID == "" {
				return ErrInvalidExecutionEvent
			}
			streams[event.StreamID] = payload.ExecutionID
		}
	}
	snapshot, err := ReplaySnapshot(events)
	if err != nil {
		return err
	}
	for streamID, executionID := range streams {
		record, ok := snapshot.Record(executionID)
		if ok && isTerminal(record) {
			continue
		}
		now := a.now().UTC()
		// Proposed-only streams are pending approval/proposals and have no
		// dispatched-side-effect uncertainty. Only Allowed-without-terminal
		// streams enter explicit human recovery.
		if !ok || record.AllowedAt == "" {
			continue
		}
		recovery, err := a.buildRecoveryRequiredEvent(
			streamID, executionID, record.JourneyID, now,
		)
		if err != nil {
			return err
		}
		if _, err := a.appendCAS(ctx, events, heads, streamID, []journal.Event{recovery}); err != nil {
			return err
		}
		heads[streamID] = recovery.Seq
	}
	return nil
}

func (a *Adapter) buildEvent(kind, streamID, jobID, executionID, callDigest string,
	proposal Proposal, generation int64, now time.Time, journeyID string) (journal.Event, error) {
	var payload any
	switch kind {
	case "proposed":
		payload = proposedPayloadV2{
			JobID: jobID, ExecutionID: executionID, CallDigest: callDigest,
			Tool: string(proposal.Call.Tool), ProposedAt: now.Format(time.RFC3339Nano),
			Generation: generation, OperationID: proposal.OperationID, JourneyID: journeyID,
		}
	default:
		return journal.Event{}, ErrInvalidExecutionInput
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return journal.Event{}, err
	}
	id := deterministicEventID(adapterPrefix, EventToolProposed, streamID, proposal.OperationID)
	return journal.Event{
		ID: id, StreamID: streamID, IdempotencyKey: adapterPrefix + "/" + EventToolProposed + "/" + executionID,
		Type: EventToolProposed, SchemaVersion: 2, EmittedAt: now.UTC(),
		CorrelationID: journeyID, PayloadJSON: body,
	}, nil
}

func (a *Adapter) buildAllowedEvent(streamID, executionID string, now time.Time) (journal.Event, error) {
	body, err := json.Marshal(allowedPayload{ExecutionID: executionID, AllowedAt: now.Format(time.RFC3339Nano)})
	if err != nil {
		return journal.Event{}, err
	}
	key := adapterPrefix + "/" + EventToolAllowed + "/" + executionID
	return journal.Event{
		ID:       deterministicEventID(adapterPrefix, EventToolAllowed, streamID, executionID),
		StreamID: streamID, IdempotencyKey: key,
		Type: EventToolAllowed, SchemaVersion: 1, EmittedAt: now.UTC(),
		CorrelationID: "", PayloadJSON: body,
	}, nil
}

func (a *Adapter) buildDeniedEvent(streamID, executionID string, denial permissions.Denial, now time.Time) (journal.Event, error) {
	body, err := json.Marshal(deniedPayloadV2{
		ExecutionID: executionID, ReasonCode: "permission_denied",
		DeniedAt: now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return journal.Event{}, err
	}
	key := adapterPrefix + "/" + EventToolDenied + "/" + executionID
	return journal.Event{
		ID:       deterministicEventID(adapterPrefix, EventToolDenied, streamID, executionID),
		StreamID: streamID, IdempotencyKey: key,
		Type: EventToolDenied, SchemaVersion: 2, EmittedAt: now.UTC(),
		PayloadJSON: body,
	}, nil
}

func (a *Adapter) buildCompletedEvent(streamID, executionID string, result ExecutionResult, started, completedAt time.Time) (journal.Event, error) {
	body, err := json.Marshal(completedPayload{
		ExecutionID: executionID, ExitCode: result.ExitCode,
		OutputDigest: result.OutputDigest, ChangedFilesDigest: result.ChangedFilesDigest,
		DurationMS: result.DurationMS, EvidenceID: result.EvidenceID,
		CompletedAt: completedAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		return journal.Event{}, err
	}
	_ = started
	key := adapterPrefix + "/" + EventToolCompleted + "/" + executionID
	return journal.Event{
		ID:       deterministicEventID(adapterPrefix, EventToolCompleted, streamID, executionID),
		StreamID: streamID, IdempotencyKey: key,
		Type: EventToolCompleted, SchemaVersion: 1, EmittedAt: completedAt.UTC(),
		PayloadJSON: body,
	}, nil
}

func (a *Adapter) buildFailedEvent(streamID, executionID, reason, errorCode string, now time.Time) (journal.Event, error) {
	body, err := json.Marshal(failedPayloadV2{
		ExecutionID: executionID, ErrorCode: errorCode,
		FailedAt: now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return journal.Event{}, err
	}
	key := adapterPrefix + "/" + EventToolFailed + "/" + executionID
	return journal.Event{
		ID:       deterministicEventID(adapterPrefix, EventToolFailed, streamID, executionID),
		StreamID: streamID, IdempotencyKey: key,
		Type: EventToolFailed, SchemaVersion: 2, EmittedAt: now.UTC(),
		PayloadJSON: body,
	}, nil
}

func (a *Adapter) buildRecoveryRequiredEvent(
	streamID string,
	executionID string,
	correlationID string,
	now time.Time,
) (journal.Event, error) {
	body, err := json.Marshal(recoveryRequiredPayloadV2{
		ExecutionID: executionID, RecoveryCode: "side_effect_unknown",
		RecoveryAction: "resolve_tool_recovery",
		RequiredAt:     now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return journal.Event{}, err
	}
	key := adapterPrefix + "/" + EventToolRecoveryRequired + "/" + executionID
	return journal.Event{
		ID:       deterministicEventID(adapterPrefix, EventToolRecoveryRequired, streamID, executionID),
		StreamID: streamID, IdempotencyKey: key,
		Type: EventToolRecoveryRequired, SchemaVersion: 2, EmittedAt: now.UTC(),
		CorrelationID: correlationID, PayloadJSON: body,
	}, nil
}

func (a *Adapter) appendCAS(
	ctx context.Context,
	events []journal.Event,
	heads map[string]int64,
	streamID string,
	batch []journal.Event,
) ([]journal.Event, error) {
	if len(batch) == 0 {
		return nil, nil
	}
	sequence := heads[streamID]
	now := a.now().UTC()
	for index := range batch {
		if batch[index].Seq == 0 {
			batch[index].Seq = sequence + int64(index) + 1
			if batch[index].EmittedAt.IsZero() {
				batch[index].EmittedAt = now
			}
		}
	}
	for _, event := range batch {
		if existing, ok := findJournalEvent(events, event.IdempotencyKey); ok {
			if !sameImmutable(existing, event) {
				return nil, fmt.Errorf("%w: %s", journal.ErrIdempotencyConflict, event.IdempotencyKey)
			}
		}
	}
	committed, err := a.store.AppendBatchIfStreamHeads(ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: heads[streamID]}},
		batch,
	)
	if err != nil {
		return nil, err
	}
	for _, event := range committed {
		if event.Seq > heads[streamID] {
			heads[streamID] = event.Seq
		}
	}
	return committed, nil
}

func (a *Adapter) remember(executionID string, result ExecutionResult) {
	result.Content = nil
	a.mu.Lock()
	a.completed[executionID] = result
	a.mu.Unlock()
}

func (a *Adapter) restoreReadOnlyContent(
	ctx context.Context,
	proposal Proposal,
	result ExecutionResult,
) (ExecutionResult, error) {
	if result.Verdict != permissions.VerdictAllow ||
		(proposal.Call.Tool != permissions.ToolRead && proposal.Call.Tool != permissions.ToolGrep) {
		return result, nil
	}
	worktree, err := a.resolver.Resolve(ctx, proposal.JobID)
	if err != nil {
		return ExecutionResult{}, ErrExecutionContent
	}
	var restored ReadResult
	switch proposal.Call.Tool {
	case permissions.ToolRead:
		executor, ok := a.executor.(ReadExecutor)
		if !ok {
			return ExecutionResult{}, ErrUnsupportedTool
		}
		restored, err = executor.Read(ctx, ReadRequest{
			Worktree: worktree, RelativePath: proposal.Call.Path,
		})
	case permissions.ToolGrep:
		executor, ok := a.executor.(GrepExecutor)
		if !ok {
			return ExecutionResult{}, ErrUnsupportedTool
		}
		restored, err = executor.Grep(ctx, GrepRequest{
			Worktree: worktree, RelativePath: proposal.Call.Path,
			Pattern: proposal.Call.Pattern,
		})
	}
	if err != nil {
		restored.Close()
		return ExecutionResult{}, errors.Join(ErrExecutionContent, err)
	}
	if restored.ContentDigest != result.OutputDigest {
		restored.Close()
		return ExecutionResult{}, ErrExecutionContent
	}
	result.Content = restored.Content
	return result, nil
}

func (a *Adapter) unsupportedTool(
	ctx context.Context,
	proposal Proposal,
	executionID, callDigest string,
	generation int64,
	events []journal.Event,
	started time.Time,
) (ExecutionResult, error) {
	a.recordToolDiagnostic(
		ctx, proposal, executionID, callDigest, ToolStageResultValidation,
		started, ToolDiagnosticFailed, "unsupported_tool", false,
	)
	return a.fail(
		ctx, proposal, executionID, callDigest, generation, events,
		"tool unavailable", "unsupported_tool",
	)
}

func (a *Adapter) readOnlyToolFailure(
	ctx context.Context,
	proposal Proposal,
	executionID, callDigest string,
	generation int64,
	events []journal.Event,
	started time.Time,
	cause error,
) (ExecutionResult, error) {
	code := "read_failed"
	if errors.Is(cause, ErrExecutionLimit) {
		code = "limit_exceeded"
	} else if errors.Is(cause, ErrExecutionContent) {
		code = "content_unavailable"
	} else if errors.Is(cause, ErrExecutionPathOutside) {
		code = "path_outside_workspace"
	}
	a.recordToolDiagnostic(
		ctx, proposal, executionID, callDigest, ToolStageResultValidation,
		started, ToolDiagnosticFailed, code, false,
	)
	return a.fail(
		ctx, proposal, executionID, callDigest, generation, events,
		"read-only tool failed", code,
	)
}

func (a *Adapter) readProposalContent(proposal Proposal) ([]byte, error) {
	if proposal.Call.Path == "" {
		return nil, ErrInvalidExecutionInput
	}
	// The frozen wire carries edit content in Command for the probe and
	// future bridge; path carries the relative target. Content is not
	// secret and is bounded by the JSON request limit.
	return []byte(proposal.Call.Command), nil
}

func pathAllowed(effective permissions.EffectiveProfile, path string) bool {
	for _, owned := range effective.Profile.OwnedPaths {
		if executionPathMatch(owned, path) {
			return true
		}
	}
	return false
}

func executionPathMatch(pattern, path string) bool {
	patternSegments := strings.Split(pattern, "/")
	pathSegments := strings.Split(strings.TrimPrefix(path, "./"), "/")
	return executionMatchSegments(patternSegments, pathSegments)
}

func executionMatchSegments(pattern, segments []string) bool {
	if len(pattern) == 0 {
		return len(segments) == 0
	}
	if pattern[0] == "**" {
		for index := 0; index <= len(segments); index++ {
			if executionMatchSegments(pattern[1:], segments[index:]) {
				return true
			}
		}
		return false
	}
	if len(segments) == 0 {
		return false
	}
	if !executionMatchSingle(pattern[0], segments[0]) {
		return false
	}
	return executionMatchSegments(pattern[1:], segments[1:])
}

func executionMatchSingle(pattern, value string) bool {
	if pattern == "" {
		return value == ""
	}
	if pattern[0] == '*' {
		for index := 0; index <= len(value); index++ {
			if executionMatchSingle(pattern[1:], value[index:]) {
				return true
			}
		}
		return false
	}
	if value == "" {
		return false
	}
	if pattern[0] == '?' || pattern[0] == value[0] {
		return executionMatchSingle(pattern[1:], value[1:])
	}
	return false
}

func callDigest(call permissions.ProposedCall) string {
	return permissions.ProposedCallDigest(call)
}

func executionID(jobID, callDigest, operationID string) string {
	sum := sha256.Sum256([]byte("execution\n" + jobID + "\n" + callDigest + "\n" + operationID))
	return hex.EncodeToString(sum[:])[:32]
}

func deterministicEventID(prefix, eventType, streamID, operationID string) string {
	sum := sha256.Sum256([]byte(prefix + "\n" + eventType + "\n" + streamID + "\n" + operationID))
	return prefix + "-" + hex.EncodeToString(sum[:])[:32]
}

func streamHeads(events []journal.Event) map[string]int64 {
	heads := make(map[string]int64)
	for _, event := range events {
		if event.Seq > heads[event.StreamID] {
			heads[event.StreamID] = event.Seq
		}
	}
	return heads
}

func findJournalEvent(events []journal.Event, key string) (journal.Event, bool) {
	for _, event := range events {
		if event.IdempotencyKey == key {
			return event, true
		}
	}
	return journal.Event{}, false
}

func sameImmutable(left, right journal.Event) bool {
	return left.ID == right.ID &&
		left.StreamID == right.StreamID &&
		left.IdempotencyKey == right.IdempotencyKey &&
		left.Type == right.Type &&
		left.SchemaVersion == right.SchemaVersion &&
		left.EmittedAt.Equal(right.EmittedAt) &&
		left.CorrelationID == right.CorrelationID &&
		left.CausationID == right.CausationID &&
		bytes.Equal(left.PayloadJSON, right.PayloadJSON)
}

func (a *Adapter) recordToolDiagnostic(
	ctx context.Context,
	proposal Proposal,
	executionID string,
	callDigest string,
	stage ToolExecutionStage,
	started time.Time,
	result string,
	errorCode string,
	retryable bool,
) {
	if proposal.Diagnostics == nil {
		return
	}
	elapsed := a.now().UTC().Sub(started)
	if elapsed < 0 {
		elapsed = 0
	}
	_ = proposal.Diagnostics.RecordToolExecutionDiagnostic(
		ctx,
		ToolExecutionDiagnostic{
			ExecutionID: executionID, JobID: proposal.JobID,
			CallDigest: callDigest, Tool: proposal.Call.Tool,
			OperationID: proposal.OperationID, CorrelationID: proposal.JourneyID,
			Stage: stage, Elapsed: elapsed, Result: result,
			ErrorCode: errorCode, Retryable: retryable,
		},
	)
}

func isTerminal(record ExecutionRecord) bool {
	return record.Status == "completed" || record.Status == "failed" ||
		record.Status == "denied" || record.Status == "recovery_required" ||
		record.Status == "recovery_aborted" ||
		record.Status == "recovery_effect_accepted" ||
		record.Status == "recovery_retry_authorized"
}

func resultFromRecord(record ExecutionRecord) ExecutionResult {
	result := ExecutionResult{
		ExecutionID: record.ExecutionID, JobID: record.JobID,
		Verdict:  permissions.VerdictDeny,
		ExitCode: record.ExitCode, OutputDigest: record.OutputDigest,
		ChangedFilesDigest: record.ChangedFilesDigest,
		EvidenceID:         record.EvidenceID, DurationMS: record.DurationMS,
		Note:             record.FailureReason,
		ErrorCode:        record.ErrorCode,
		RecoveryRequired: record.Status == "recovery_required",
		RecoveryCode:     record.RecoveryCode, RecoveryAction: record.RecoveryAction,
	}
	if record.Status == "completed" {
		result.Verdict = permissions.VerdictAllow
		result.Note = "completed"
	} else if record.Status == "denied" {
		result.Denial = permissions.Denial{Reason: record.DenialReason}
		result.Note = "denied without execution"
	} else if record.Status == "recovery_required" {
		result.Note = "recovery required"
	} else if record.Status == "recovery_aborted" {
		result.Note = "recovery aborted"
	} else if record.Status == "recovery_effect_accepted" {
		result.Note = "observed effect accepted; current execution closed"
	} else if record.Status == "recovery_retry_authorized" {
		result.Note = "retry requires the authorized replacement Attempt"
	}
	return result
}

type existingApprovalRecord struct {
	id          string
	digest      string
	status      string
	consumerID  string
	operationID string
}

// existingApproval scans the Journal for a permission approval matching the
// job and call digest and returns its latest resolution or consumption state.
func existingApproval(
	events []journal.Event,
	jobID string,
	callDigest string,
) (existingApprovalRecord, bool) {
	status := make(map[string]string)
	meta := make(map[string][2]string)
	consumers := make(map[string][2]string)
	for _, event := range events {
		if event.Type == "ApprovalRequested" {
			var payload struct {
				ApprovalRequestID     string `json:"approval_request_id"`
				ApprovalRequestDigest string `json:"approval_request_digest"`
				Context               struct {
					ProjectID  string `json:"project_id"`
					WorkItemID string `json:"work_item_id"`
				} `json:"context"`
				ContinuationDigest string `json:"continuation_digest"`
			}
			if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
				continue
			}
			if payload.Context.ProjectID == rules.PermissionApprovalProjectID &&
				payload.Context.WorkItemID == jobID &&
				payload.ContinuationDigest == callDigest {
				status[payload.ApprovalRequestID] = "pending"
				meta[payload.ApprovalRequestID] = [2]string{payload.ApprovalRequestID, payload.ApprovalRequestDigest}
			}
		}
		if event.Type == "ApprovalDecided" || event.Type == "ApprovalExpired" {
			var payload struct {
				ApprovalRequestID string `json:"approval_request_id"`
				Status            string `json:"status"`
			}
			if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
				continue
			}
			if _, ok := status[payload.ApprovalRequestID]; ok {
				resolution := "rejected"
				if event.Type == "ApprovalExpired" {
					resolution = "rejected"
				} else if payload.Status == "approved" {
					resolution = "approved"
				}
				status[payload.ApprovalRequestID] = resolution
			}
		}
		if event.Type == "PermissionApprovalConsumed" {
			var payload struct {
				ApprovalRequestID string `json:"approval_request_id"`
				ConsumerID        string `json:"consumer_id"`
				OperationID       string `json:"operation_id"`
			}
			if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
				continue
			}
			if _, ok := status[payload.ApprovalRequestID]; ok {
				status[payload.ApprovalRequestID] = "consumed"
				consumers[payload.ApprovalRequestID] = [2]string{
					payload.ConsumerID, payload.OperationID,
				}
			}
		}
	}
	var ids []string
	for id := range status {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return existingApprovalRecord{}, false
	}
	id := ids[0]
	consumer := consumers[id]
	return existingApprovalRecord{
		id: id, digest: meta[id][1], status: status[id],
		consumerID: consumer[0], operationID: consumer[1],
	}, true
}

var _ = sort.Strings
