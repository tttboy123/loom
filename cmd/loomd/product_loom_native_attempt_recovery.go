package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"loom-pi-rebuild/internal/agentcheckpoint"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/contextcapsule"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
)

const productAttemptRecoveryCleanupTimeout = 5 * time.Second

type productRecoveryCapsuleAuthorityLister interface {
	ListRoleContextCapsuleAuthorities(
		context.Context,
		string,
	) ([]contextcapsule.AuthorityRecord, error)
}

type productLoomNativeAttemptRecovery struct {
	adapters     map[string]supervisor.RuntimeAdapter
	capsules     productRecoveryCapsuleAuthorityLister
	capsuleStore app.MissionContextCapsuleStore
	checkpoints  agentcheckpoint.Store
	inbox        *work.AgentInboxCoordinator
	loops        *work.AttemptLoopAuthority
}

type productLoomNativeAttemptRecoverySession struct {
	runtimeInstanceID string
	sessionDigest     string
	adapter           nativeadapter.LoomNativeAgentContinuationAdapter
	outcome           work.AgentAttemptRestartOutcome
	incidentID        string
	leaseID           string
	authority         contextcapsule.AuthorityRecord
	contextPayload    []byte
	checkpoint        agentcheckpoint.Payload
	inputSource       *productAttemptLoopAgentInputSource
	loops             *work.AttemptLoopAuthority
	continueMu        sync.Mutex
	continued         bool
	closed            bool
	closeOnce         sync.Once
}

func newProductLoomNativeAttemptRecovery(
	adapters []supervisor.RuntimeAdapter,
	capsules productRecoveryCapsuleAuthorityLister,
) (*productLoomNativeAttemptRecovery, error) {
	if len(adapters) == 0 || nilProductAgentInterface(capsules) {
		return nil, errProductInvalidAttemptRecoveryAttachment
	}
	indexed := make(map[string]supervisor.RuntimeAdapter, len(adapters))
	for _, adapter := range adapters {
		if nilProductAgentInterface(adapter) {
			return nil, errProductInvalidAttemptRecoveryAttachment
		}
		key := productMissionAdapterKey(adapter.AdapterType(), adapter.RuntimeInstanceID())
		if key == "" {
			return nil, errProductInvalidAttemptRecoveryAttachment
		}
		if _, duplicate := indexed[key]; duplicate {
			return nil, errProductInvalidAttemptRecoveryAttachment
		}
		indexed[key] = adapter
	}
	return &productLoomNativeAttemptRecovery{adapters: indexed, capsules: capsules}, nil
}

func newProductLoomNativeAttemptContinuation(
	adapters []supervisor.RuntimeAdapter,
	capsules app.MissionContextCapsuleStore,
	checkpoints agentcheckpoint.Store,
	inbox *work.AgentInboxCoordinator,
	loops *work.AttemptLoopAuthority,
) (*productLoomNativeAttemptRecovery, error) {
	recovery, err := newProductLoomNativeAttemptRecovery(adapters, capsules)
	if err != nil || nilProductAgentInterface(checkpoints) || inbox == nil || loops == nil {
		return nil, errors.Join(errProductInvalidAttemptRecoveryAttachment, err)
	}
	recovery.capsuleStore = capsules
	recovery.checkpoints = checkpoints
	recovery.inbox = inbox
	recovery.loops = loops
	return recovery, nil
}

func (recovery *productLoomNativeAttemptRecovery) ResolveAgentAttemptRestartCapability(
	ctx context.Context,
	outcome work.AgentAttemptRestartOutcome,
) (work.AgentAttemptRestartCapability, error) {
	validated, authority, contract, err := recovery.resolve(ctx, outcome)
	if err != nil {
		return work.AgentAttemptRestartCapability{}, err
	}
	sessionDigest, err := productLoomNativeRestartSessionDigest(validated, authority, contract)
	if err != nil {
		return work.AgentAttemptRestartCapability{}, errors.Join(
			errProductAttemptRecoveryReattach, err,
		)
	}
	capability := work.AgentAttemptRestartCapability{
		SchemaVersion:          1,
		AttemptID:              validated.Binding.AttemptID,
		RuntimeInstanceID:      validated.Binding.PayloadAuthority.RuntimeInstanceID,
		ExecutionBindingDigest: validated.Binding.PayloadAuthority.ExecutionBindingDigest,
		ContextCapsuleDigest:   validated.Binding.PayloadAuthority.CapsuleDigest,
		ResumeMode:             work.AgentAttemptRestartResumeFromCheckpoint,
		SessionBindingDigest:   sessionDigest,
	}
	capability.CapabilityDigest = work.AgentAttemptRestartCapabilityDigest(capability)
	return capability, nil
}

func (recovery *productLoomNativeAttemptRecovery) ReattachAgentAttempt(
	ctx context.Context,
	grant work.AgentAttemptRecoveryDispatchGrant,
) (productAgentAttemptRecoverySession, error) {
	outcome, capability, err := work.ValidateAgentAttemptRecoveryDispatchGrant(grant)
	if err != nil {
		return nil, errors.Join(errProductAttemptRecoveryReattach, err)
	}
	current, err := recovery.ResolveAgentAttemptRestartCapability(ctx, outcome)
	if err != nil || current != capability {
		return nil, errors.Join(errProductAttemptRecoveryReattach, err)
	}
	session := &productLoomNativeAttemptRecoverySession{
		runtimeInstanceID: current.RuntimeInstanceID,
		sessionDigest:     current.SessionBindingDigest,
	}
	if recovery.capsuleStore == nil && recovery.checkpoints == nil &&
		recovery.inbox == nil && recovery.loops == nil {
		return session, nil
	}
	if nilProductAgentInterface(recovery.capsuleStore) ||
		nilProductAgentInterface(recovery.checkpoints) ||
		recovery.inbox == nil || recovery.loops == nil {
		return nil, errProductAttemptRecoveryReattach
	}
	binding := outcome.ExecutionBinding
	adapter, ok := recovery.adapters[productMissionAdapterKey(
		binding.HarnessAdapter, binding.RuntimeInstanceID,
	)].(nativeadapter.LoomNativeAgentContinuationAdapter)
	if !ok || nilProductAgentInterface(adapter) {
		return nil, errProductAttemptRecoveryReattach
	}
	authority, err := recovery.resolveCapsuleAuthority(ctx, outcome, binding)
	if err != nil {
		return nil, err
	}
	capsule, contextPayload, err := recovery.capsuleStore.ReadRoleContextCapsule(ctx, authority)
	if err != nil || capsule.AuthorityRecord() != authority || len(contextPayload) == 0 {
		clearProductRecoveryBytes(contextPayload)
		return nil, errors.Join(errProductAttemptRecoveryReattach, err)
	}
	checkpoint, err := recovery.checkpoints.ResolveAgentCheckpoint(
		ctx, productAgentCheckpointQuery(outcome),
	)
	if err != nil {
		clearProductRecoveryBytes(contextPayload)
		return nil, errors.Join(errProductAttemptRecoveryReattach, err)
	}
	session.adapter = adapter
	session.outcome = outcome
	session.incidentID = grant.IncidentID()
	session.leaseID = grant.LeaseID()
	session.authority = authority
	session.contextPayload = contextPayload
	session.checkpoint = checkpoint
	session.loops = recovery.loops
	session.inputSource = &productAttemptLoopAgentInputSource{
		loops: recovery.loops, inbox: recovery.inbox, checkpoints: recovery.checkpoints,
		binding: outcome.Binding, segmentID: outcome.SegmentID,
		cursor: &productAttemptLoopCursor{
			turnID: outcome.TurnID, turnSequence: outcome.TurnSequence,
			stepID: outcome.StepID, stepSequence: outcome.StepSequence,
		},
	}
	return session, nil
}

func productAgentCheckpointQuery(
	outcome work.AgentAttemptRestartOutcome,
) agentcheckpoint.Query {
	authority := outcome.Binding.PayloadAuthority
	return agentcheckpoint.Query{
		ConversationID: authority.ConversationID, SegmentID: outcome.SegmentID,
		AttemptID: outcome.Binding.AttemptID, AgentInstanceID: authority.AgentInstanceID,
		WorkItemID: authority.WorkItemID, RunID: authority.RunID,
		ClaimGeneration:        authority.ClaimGeneration,
		RuntimeInstanceID:      authority.RuntimeInstanceID,
		ExecutionBindingDigest: authority.ExecutionBindingDigest,
		CapsuleDigest:          authority.CapsuleDigest, ContentDigest: outcome.CheckpointDigest,
	}
}

func (recovery *productLoomNativeAttemptRecovery) resolve(
	ctx context.Context,
	outcome work.AgentAttemptRestartOutcome,
) (work.AgentAttemptRestartOutcome, contextcapsule.AuthorityRecord, string, error) {
	if recovery == nil || ctx == nil || ctx.Err() != nil ||
		nilProductAgentInterface(recovery.capsules) {
		return work.AgentAttemptRestartOutcome{}, contextcapsule.AuthorityRecord{}, "",
			errProductAttemptRecoveryReattach
	}
	validated, err := work.ValidateAgentAttemptRestartOutcome(outcome)
	if err != nil {
		return work.AgentAttemptRestartOutcome{}, contextcapsule.AuthorityRecord{}, "",
			errors.Join(errProductAttemptRecoveryReattach, err)
	}
	binding, err := loomruntime.ValidateFrozenExecutionBinding(validated.ExecutionBinding)
	if err != nil || binding.HarnessAdapter != nativeadapter.LoomNativeAgentAdapterType ||
		binding.RuntimeInstanceID != validated.Binding.PayloadAuthority.RuntimeInstanceID {
		return work.AgentAttemptRestartOutcome{}, contextcapsule.AuthorityRecord{}, "",
			errProductAttemptRecoveryReattach
	}
	adapter, found := recovery.adapters[productMissionAdapterKey(
		binding.HarnessAdapter, binding.RuntimeInstanceID,
	)]
	if !found || nilProductAgentInterface(adapter) {
		return work.AgentAttemptRestartOutcome{}, contextcapsule.AuthorityRecord{}, "",
			errProductAttemptRecoveryReattach
	}
	conformance, ok := adapter.(loomruntime.AgentAttemptRestartConformance)
	if !ok || nilProductAgentInterface(conformance) ||
		conformance.AgentAttemptRestartContract() != loomruntime.AgentAttemptRestartLoomOwnedCheckpointV1 ||
		conformance.ValidateAgentAttemptRestartBinding(binding) != nil {
		return work.AgentAttemptRestartOutcome{}, contextcapsule.AuthorityRecord{}, "",
			errProductAttemptRecoveryReattach
	}
	authority, err := recovery.resolveCapsuleAuthority(ctx, validated, binding)
	if err != nil {
		return work.AgentAttemptRestartOutcome{}, contextcapsule.AuthorityRecord{}, "", err
	}
	return validated, authority, conformance.AgentAttemptRestartContract(), nil
}

func (recovery *productLoomNativeAttemptRecovery) resolveCapsuleAuthority(
	ctx context.Context,
	outcome work.AgentAttemptRestartOutcome,
	binding loomruntime.FrozenExecutionBinding,
) (contextcapsule.AuthorityRecord, error) {
	conversationID := outcome.Binding.PayloadAuthority.ConversationID
	authorities, err := recovery.capsules.ListRoleContextCapsuleAuthorities(ctx, conversationID)
	if err != nil {
		return contextcapsule.AuthorityRecord{}, errors.Join(errProductAttemptRecoveryReattach, err)
	}
	var matched contextcapsule.AuthorityRecord
	found := false
	for _, candidate := range authorities {
		if candidate.CapsuleDigest != outcome.Binding.PayloadAuthority.CapsuleDigest {
			continue
		}
		validated, validationErr := contextcapsule.ValidateAuthorityRecord(candidate)
		if validationErr != nil {
			return contextcapsule.AuthorityRecord{}, errors.Join(
				errProductAttemptRecoveryReattach, validationErr,
			)
		}
		if found || validated.ConversationID != conversationID ||
			validated.TeamID != outcome.Binding.TeamInstanceID ||
			validated.AgentID != outcome.Binding.PayloadAuthority.AgentInstanceID ||
			validated.ProviderID != binding.ProviderID ||
			validated.ProviderAccountID != binding.ProviderAccountID ||
			validated.ModelID != binding.ModelID || validated.AuthMode != string(binding.AuthMode) ||
			validated.ContextAdapterID != "context:loom-native:v1" {
			return contextcapsule.AuthorityRecord{}, errProductAttemptRecoveryReattach
		}
		matched, found = validated, true
	}
	if !found {
		return contextcapsule.AuthorityRecord{}, errProductAttemptRecoveryReattach
	}
	return matched, nil
}

func productLoomNativeRestartSessionDigest(
	outcome work.AgentAttemptRestartOutcome,
	authority contextcapsule.AuthorityRecord,
	contract string,
) (string, error) {
	inputIDs := append([]string(nil), outcome.InputIDs...)
	sort.Strings(inputIDs)
	canonical, err := json.Marshal(struct {
		Domain                  string   `json:"domain"`
		SchemaVersion           int      `json:"schema_version"`
		Contract                string   `json:"contract"`
		CandidateDigest         string   `json:"candidate_digest"`
		DisclosureReceiptDigest string   `json:"disclosure_receipt_digest"`
		SegmentID               string   `json:"segment_id"`
		CheckpointDigest        string   `json:"checkpoint_digest"`
		InputIDs                []string `json:"input_ids"`
	}{
		Domain:        "loom/product-agent-attempt/loom-native-restart-session/v1",
		SchemaVersion: 1, Contract: contract,
		CandidateDigest:         work.AgentAttemptRestartCandidateDigest(outcome),
		DisclosureReceiptDigest: authority.DisclosureReceiptDigest,
		SegmentID:               outcome.SegmentID, CheckpointDigest: outcome.CheckpointDigest,
		InputIDs: inputIDs,
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func (session *productLoomNativeAttemptRecoverySession) RuntimeInstanceID() string {
	if session == nil {
		return ""
	}
	return session.runtimeInstanceID
}

func (session *productLoomNativeAttemptRecoverySession) SessionBindingDigest() string {
	if session == nil {
		return ""
	}
	return session.sessionDigest
}

func (session *productLoomNativeAttemptRecoverySession) ContinueAgentAttempt(
	ctx context.Context,
	sink supervisor.FrameSink,
) (supervisor.AdapterResult, error) {
	if session == nil || ctx == nil || ctx.Err() != nil ||
		nilProductAgentInterface(sink) || nilProductAgentInterface(session.adapter) ||
		session.inputSource == nil || session.loops == nil ||
		len(session.contextPayload) == 0 || len(session.checkpoint.Content) == 0 {
		return supervisor.AdapterResult{}, errProductAttemptRecoveryReattach
	}
	session.continueMu.Lock()
	defer session.continueMu.Unlock()
	if session.continued || session.closed {
		return supervisor.AdapterResult{}, errProductInvalidAttemptRecoveryAttachment
	}
	session.continued = true
	batch, available, err := session.inputSource.NextAgentInput(
		ctx, loomruntime.AgentInputCheckpoint{
			OutputDigest: session.outcome.CheckpointDigest,
		},
	)
	if err != nil || !available || !productRecoveryBatchMatchesOutcome(batch, session.outcome) {
		batch.Close()
		return supervisor.AdapterResult{}, errors.Join(errProductAttemptRecoveryReattach, err)
	}
	defer batch.Close()
	authority := session.outcome.Binding.PayloadAuthority
	result, dispatchErr := session.adapter.ResumeAgentAttempt(
		ctx,
		nativeadapter.LoomNativeAgentContinuationRequest{
			DispatchMessageID: productDeterministicUUID(
				"agent-attempt-recovery-dispatch", session.leaseID,
			),
			IncidentID: session.incidentID, ClaimID: authority.ClaimID,
			WorkItemID: authority.WorkItemID, RunID: authority.RunID,
			ClaimGeneration:  authority.ClaimGeneration,
			AgentInstanceID:  authority.AgentInstanceID,
			SegmentID:        session.outcome.SegmentID,
			CheckpointDigest: session.outcome.CheckpointDigest,
			ExecutionBinding: session.outcome.ExecutionBinding,
			ContextCapsule:   session.authority,
			ContextPayload:   session.contextPayload,
			PreviousOutput:   session.checkpoint.Content,
			CurrentInputs:    batch, NextInputs: session.inputSource,
			FrameSink: sink,
		},
	)
	if dispatchErr == nil {
		if validator, ok := sink.(supervisor.RecoveryAdapterResultValidator); ok {
			_, _, _, validationErr := validator.ValidateAdapterResult(ctx, result)
			dispatchErr = errors.Join(dispatchErr, validationErr)
		}
	}
	turnID, _, stepID, _ := session.inputSource.cursor.Current()
	terminalErr := dispatchErr
	terminalized := false
	status, statusAvailable := productAttemptLoopTerminalStatus(result)
	if dispatchErr != nil || !statusAvailable || status != "succeeded" ||
		!result.DispatchAcknowledged() || !result.ResultAcknowledged() {
		endErr := (&productAttemptLoopRuntimeAdapter{loops: session.loops}).endFailed(
			context.WithoutCancel(ctx), session.outcome.Binding, turnID, stepID,
		)
		terminalized = endErr == nil
		terminalErr = errors.Join(
			terminalErr,
			endErr,
		)
	} else {
		outputDigest := productAttemptLoopResultDigest(result)
		if _, err := session.loops.EndStep(
			ctx, session.outcome.Binding, work.AttemptLoopStepEndInput{
				TurnID: turnID, StepID: stepID, Outcome: work.AttemptStepFinal,
				OutputDigest: outputDigest,
			},
		); err != nil {
			terminalErr = errors.Join(terminalErr, err)
		} else if _, err := session.loops.EndTurn(
			ctx, session.outcome.Binding, work.AttemptLoopTurnEndInput{
				TurnID: turnID, Outcome: work.AttemptTurnSucceeded,
			},
		); err != nil {
			terminalErr = errors.Join(terminalErr, err)
		} else {
			terminalized = true
		}
	}
	if terminalized {
		terminalErr = errors.Join(
			terminalErr,
			deleteProductRecoveryCheckpoint(
				ctx, session.inputSource.checkpoints, session.checkpoint.Binding,
			),
		)
	}
	return result, terminalErr
}

func deleteProductRecoveryCheckpoint(
	ctx context.Context,
	store agentcheckpoint.Store,
	binding agentcheckpoint.Binding,
) error {
	if ctx == nil || nilProductAgentInterface(store) {
		return errProductAttemptRecoveryReattach
	}
	cleanupContext, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), productAttemptRecoveryCleanupTimeout,
	)
	defer cancel()
	return store.DeleteAgentCheckpoint(cleanupContext, binding)
}

func productRecoveryBatchMatchesOutcome(
	batch loomruntime.AgentInputBatch,
	outcome work.AgentAttemptRestartOutcome,
) bool {
	if batch.TurnID != outcome.TurnID || batch.TurnSequence != outcome.TurnSequence ||
		len(batch.Inputs) != len(outcome.InputIDs) {
		return false
	}
	if outcome.StepID != "" &&
		(batch.StepID != outcome.StepID || batch.StepSequence != outcome.StepSequence) {
		return false
	}
	want := append([]string(nil), outcome.InputIDs...)
	got := make([]string, 0, len(batch.Inputs))
	for index := range batch.Inputs {
		got = append(got, batch.Inputs[index].Binding.InputID)
	}
	sort.Strings(want)
	sort.Strings(got)
	return len(want) == len(got) && equalProductRecoveryStrings(want, got)
}

func equalProductRecoveryStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (session *productLoomNativeAttemptRecoverySession) Close() error {
	if session != nil {
		session.closeOnce.Do(func() {
			session.continueMu.Lock()
			defer session.continueMu.Unlock()
			session.closed = true
			clearProductRecoveryBytes(session.contextPayload)
			session.contextPayload = nil
			session.checkpoint.Close()
			session.inputSource = nil
			session.adapter = nil
		})
	}
	return nil
}

func clearProductRecoveryBytes(content []byte) {
	for index := range content {
		content[index] = 0
	}
}
