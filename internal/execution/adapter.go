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
	decisions DecisionRecorder
	sandbox   SandboxGate
	now       func() time.Time

	mu        sync.Mutex
	inflight  map[string]bool
	completed map[string]ExecutionResult
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
	return &Adapter{
		store: store, evidence: evidenceStore, executor: executor,
		resolver: resolver, approvals: approvals, decisions: decisions,
		now: now, inflight: make(map[string]bool), completed: make(map[string]ExecutionResult),
	}, nil
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
		return result, nil
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
		a.mu.Lock()
		a.completed[executionID] = result
		a.mu.Unlock()
		return result, nil
	}
	if found && existing.AllowedAt != "" {
		// An Allowed fact with no terminal fact means the side effect may
		// already have happened. Re-executing would violate exactly-once;
		// ReplayPending is the only path that terminalizes it, and it never
		// executes.
		return ExecutionResult{}, ErrExecutionInterrupted
	}

	effective, profileErr := permissions.ResolveEffectiveProfile(permissionProjection, proposal.JobID)
	verdict, denial, evalErr := permissions.VerdictDeny, permissions.Denial{}, error(nil)
	generation := int64(0)
	if profileErr == nil {
		generation = effective.Profile.Generation
		verdict, denial, evalErr = permissions.Evaluate(effective, proposal.Call)
	}
	if profileErr != nil {
		return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
			Reason:            profileErr.Error(),
			AuthorizationPath: "bind a valid permission profile or fix the stale binding",
		})
	}
	if evalErr != nil {
		return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
			Reason:            evalErr.Error(),
			AuthorizationPath: "fix the proposed call",
		})
	}

	switch verdict {
	case permissions.VerdictDeny:
		return a.deny(ctx, proposal, executionID, callDigest, generation, events, denial)
	case permissions.VerdictAsk:
		return a.ask(ctx, proposal, executionID, callDigest, generation, events, denial, found)
	case permissions.VerdictAllow:
		return a.allowAndExecute(ctx, proposal, executionID, callDigest, generation, events, existing, found, effective, callDigest)
	default:
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
	if approvalID, digest, status, found := existingApproval(events, proposal.JobID, callDigest); found {
		if status == "approved" {
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
					result, resumeErr := a.allowAndExecute(
						ctx, proposal, executionID, callDigest, generation,
						events, existing, existingFound, effective, callDigest,
					)
					if resumeErr != nil {
						return ExecutionResult{}, resumeErr
					}
					result.ApprovalID = approvalID
					result.ApprovalDigest = digest
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
		if status == "rejected" {
			return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
				Reason: "approval was rejected", AuthorizationPath: "request a new grant or allow rule",
			})
		}
		return ExecutionResult{
			ExecutionID: executionID, JobID: proposal.JobID,
			Verdict: permissions.VerdictAsk, Denial: denial,
			ApprovalID: approvalID, ApprovalDigest: digest,
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
		return ExecutionResult{}, err
	}
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
	_ string,
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
	result, executeErr := a.executeApproved(ctx, proposal, executionID, callDigest, generation, events, effective, callDigest, "", "")
	if executeErr != nil {
		return ExecutionResult{}, executeErr
	}
	if result.ApprovalID == "" {
		if approvalID, digest, status, ok := existingApproval(events, proposal.JobID, callDigest); ok && status == "approved" {
			result.ApprovalID = approvalID
			result.ApprovalDigest = digest
			a.remember(executionID, result)
		}
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
	_ string,
	approvalID, approvalDigest string,
) (ExecutionResult, error) {
	// Phase 3B sandbox policy gate: Required + unavailable backend => fail
	// closed with zero side effects (never local fallback).
	if a.sandbox != nil {
		policy, policyErr := a.sandbox.ResolvePolicy(ctx, proposal.JobID)
		if policyErr != nil {
			return a.deny(ctx, proposal, executionID, callDigest, generation, events,
				permissions.Denial{Reason: "sandbox policy error"})
		}
		if policy.Required {
			available, availErr := a.sandbox.BackendAvailable(ctx, policy.Backend)
			if availErr != nil || !available {
				return a.deny(ctx, proposal, executionID, callDigest, generation, events,
					permissions.Denial{
						Reason:            "required sandbox backend unavailable",
						AuthorizationPath: "configure the sandbox backend",
					})
			}
		}
	}
	worktree, err := a.resolver.Resolve(ctx, proposal.JobID)
	if err != nil {
		return a.fail(ctx, proposal, executionID, callDigest, generation, events, "worktree resolution failed", "worktree_unavailable")
	}
	started := a.now().UTC()
	var (
		result ExecutionResult
		runErr error
	)
	switch proposal.Call.Tool {
	case permissions.ToolEdit:
		if !pathAllowed(effective, proposal.Call.Path) {
			return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
				Reason:            "edit path is not inside profile owned paths",
				AuthorizationPath: "narrow the owned paths or add an allow rule",
			})
		}
		content, readErr := a.readProposalContent(proposal)
		if readErr != nil {
			return a.fail(ctx, proposal, executionID, callDigest, generation, events, readErr.Error(), "invalid_edit_content")
		}
		contentSum := sha256.Sum256(content)
		editResult, editErr := a.executor.Edit(ctx, EditRequest{
			Worktree: worktree, RelativePath: proposal.Call.Path,
			NewContentDigest: hex.EncodeToString(contentSum[:]), NewContentBytes: content,
		})
		if editErr != nil {
			code := "edit_failed"
			if errors.Is(editErr, ErrExecutionLimit) {
				code = "limit_exceeded"
			}
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
	default:
		return a.deny(ctx, proposal, executionID, callDigest, generation, events, permissions.Denial{
			Reason:            "tool is not executable by the adapter",
			AuthorizationPath: "use Edit or Bash through the execution adapter",
		})
	}
	_ = runErr
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
		return a.fail(ctx, proposal, executionID, callDigest, generation, events, "evidence marshal failed", "evidence_failed")
	}
	sum := sha256.Sum256(body)
	expected := hex.EncodeToString(sum[:])
	artifact, publishErr := a.evidence.Publish(ctx, bytes.NewReader(body), expected)
	if publishErr != nil {
		return a.fail(ctx, proposal, executionID, callDigest, generation, events, "evidence publish failed", "evidence_failed")
	}
	result.EvidenceID = artifact.Digest
	completedAt := a.now().UTC()
	streamID := executionStreamID(proposal.JobID, executionID)
	completed, err := a.buildCompletedEvent(streamID, executionID, result, started, completedAt)
	if err != nil {
		return ExecutionResult{}, err
	}
	heads := streamHeads(events)
	if _, err := a.appendCAS(ctx, events, heads, streamID, []journal.Event{completed}); err != nil {
		return ExecutionResult{}, err
	}
	if approvalID != "" {
		result.ApprovalID = approvalID
		result.ApprovalDigest = approvalDigest
	}
	a.remember(executionID, result)
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
		Verdict: permissions.VerdictDeny, Note: reason,
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
		failed, err := a.buildFailedEvent(streamID, executionID, "interrupted before terminal fact", "interrupted", now)
		if err != nil {
			return err
		}
		if _, err := a.appendCAS(ctx, events, heads, streamID, []journal.Event{failed}); err != nil {
			return err
		}
		heads[streamID] = failed.Seq
	}
	return nil
}

func (a *Adapter) buildEvent(kind, streamID, jobID, executionID, callDigest string,
	proposal Proposal, generation int64, now time.Time, journeyID string) (journal.Event, error) {
	var payload any
	switch kind {
	case "proposed":
		payload = proposedPayload{
			JobID: jobID, ExecutionID: executionID, CallDigest: callDigest,
			Tool: string(proposal.Call.Tool), Command: proposal.Call.Command,
			Path: proposal.Call.Path, ProposedAt: now.Format(time.RFC3339Nano),
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
		Type: EventToolProposed, SchemaVersion: 1, EmittedAt: now.UTC(),
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
	body, err := json.Marshal(deniedPayload{
		ExecutionID: executionID, Denial: denial,
		DeniedAt: now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return journal.Event{}, err
	}
	key := adapterPrefix + "/" + EventToolDenied + "/" + executionID
	return journal.Event{
		ID:       deterministicEventID(adapterPrefix, EventToolDenied, streamID, executionID),
		StreamID: streamID, IdempotencyKey: key,
		Type: EventToolDenied, SchemaVersion: 1, EmittedAt: now.UTC(),
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
	body, err := json.Marshal(failedPayload{
		ExecutionID: executionID, Reason: reason, ErrorCode: errorCode,
		FailedAt: now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return journal.Event{}, err
	}
	key := adapterPrefix + "/" + EventToolFailed + "/" + executionID
	return journal.Event{
		ID:       deterministicEventID(adapterPrefix, EventToolFailed, streamID, executionID),
		StreamID: streamID, IdempotencyKey: key,
		Type: EventToolFailed, SchemaVersion: 1, EmittedAt: now.UTC(),
		PayloadJSON: body,
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
	a.mu.Lock()
	a.completed[executionID] = result
	a.mu.Unlock()
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
	body, _ := json.Marshal(call)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
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

func isTerminal(record ExecutionRecord) bool {
	return record.Status == "completed" || record.Status == "failed" || record.Status == "denied"
}

func resultFromRecord(record ExecutionRecord) ExecutionResult {
	result := ExecutionResult{
		ExecutionID: record.ExecutionID, JobID: record.JobID,
		Verdict:  permissions.VerdictDeny,
		ExitCode: record.ExitCode, OutputDigest: record.OutputDigest,
		ChangedFilesDigest: record.ChangedFilesDigest,
		EvidenceID:         record.EvidenceID, DurationMS: record.DurationMS,
		Note: record.FailureReason,
	}
	if record.Status == "completed" {
		result.Verdict = permissions.VerdictAllow
		result.Note = "completed"
	} else if record.Status == "denied" {
		result.Denial = permissions.Denial{Reason: record.DenialReason}
		result.Note = "denied without execution"
	}
	return result
}

// existingApproval scans the Journal for a permission approval matching the
// job and call digest and returns its latest resolution.
func existingApproval(events []journal.Event, jobID, callDigest string) (string, string, string, bool) {
	status := make(map[string]string)
	meta := make(map[string][2]string)
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
	}
	var ids []string
	for id := range status {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return "", "", "", false
	}
	id := ids[0]
	digest := meta[id][1]
	return id, digest, status[id], true
}

var _ = sort.Strings
