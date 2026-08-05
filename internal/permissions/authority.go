package permissions

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"loom-pi-rebuild/internal/journal"
)

// Authority is the state-writing authority for permission facts. Every method
// rebuilds the latest projection from the Journal and appends through
// AppendBatchIfStreamHeads CAS, mirroring internal/authorization and
// internal/rules. This layer never writes approval lifecycle events.
type Authority struct {
	store *journal.Store
	now   func() time.Time
}

func NewAuthority(store *journal.Store, now func() time.Time) (*Authority, error) {
	if store == nil || now == nil {
		return nil, ErrInvalidAuthorityInput
	}
	return &Authority{store: store, now: now}, nil
}

func (a *Authority) latestProjection(ctx context.Context) (*Projection, []journal.Event, map[string]int64, error) {
	events, err := a.store.ReadAll(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	projection, err := Replay(events)
	if err != nil {
		return nil, nil, nil, err
	}
	return projection, events, headsOf(events), nil
}

func headsOf(events []journal.Event) map[string]int64 {
	heads := make(map[string]int64)
	for _, event := range events {
		if event.Seq > heads[event.StreamID] {
			heads[event.StreamID] = event.Seq
		}
	}
	return heads
}

func newPermEvent(
	eventType, streamID string,
	sequence int64,
	emittedAt time.Time,
	correlationID, operationID string,
	payload any,
) journal.Event {
	id := "perm1-" + hex.EncodeToString(func() []byte {
		sum := sha256.Sum256([]byte("perm1\n" + eventType + "\n" + streamID + "\n" + operationID))
		return sum[:]
	}())[:32]
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return journal.Event{
		ID: id, StreamID: streamID, Seq: sequence,
		IdempotencyKey: "perm1/" + eventType + "/" + operationID,
		Type:           eventType, SchemaVersion: 1, EmittedAt: emittedAt,
		CorrelationID: correlationID, PayloadJSON: body,
	}
}

func (a *Authority) appendCAS(
	ctx context.Context,
	events []journal.Event,
	heads map[string]int64,
	streamID string,
	event journal.Event,
) ([]journal.Event, error) {
	if existing, ok := findByIdempotencyKey(events, event.IdempotencyKey); ok {
		if sameImmutableIgnoringSeq(existing, event) {
			return []journal.Event{existing}, nil
		}
		return nil, fmt.Errorf("%w: %s", journal.ErrIdempotencyConflict, event.IdempotencyKey)
	}
	committed, err := a.store.AppendBatchIfStreamHeads(ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: heads[streamID]}},
		[]journal.Event{event},
	)
	if err != nil {
		return nil, err
	}
	return committed, nil
}

func findByIdempotencyKey(events []journal.Event, key string) (journal.Event, bool) {
	for _, event := range events {
		if event.IdempotencyKey == key {
			return event, true
		}
	}
	return journal.Event{}, false
}

func sameImmutableIgnoringSeq(left, right journal.Event) bool {
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

func (a *Authority) DefineProfile(ctx context.Context, input ProfileInput, operationID, journeyID string) ([]journal.Event, error) {
	profile, err := CompileProfile(input)
	if err != nil {
		return nil, err
	}
	projection, events, heads, err := a.latestProjection(ctx)
	if err != nil {
		return nil, err
	}
	if _, exists := projection.Profiles[profile.ProfileID]; exists {
		return nil, ErrProfileExists
	}
	streamID := streamProfile + profile.ProfileID
	event := newPermEvent("PermissionProfileDefined", streamID, heads[streamID]+1,
		a.now(), journeyID, operationID, profileEventPayload{
			ProfileID: profile.ProfileID, Generation: profile.Generation, Digest: profile.Digest,
			Mode: profile.Mode, Rules: profile.Rules, OwnedPaths: profile.OwnedPaths,
		})
	return a.appendCAS(ctx, events, heads, streamID, event)
}

func (a *Authority) ReviseProfile(ctx context.Context, profileID string, input ProfileInput, operationID, journeyID string) ([]journal.Event, error) {
	if input.ProfileID != profileID {
		return nil, ErrInvalidInput
	}
	if !validID(profileID) {
		return nil, ErrInvalidInput
	}
	compiled, err := CompileProfile(input)
	if err != nil {
		return nil, err
	}
	projection, events, heads, err := a.latestProjection(ctx)
	if err != nil {
		return nil, err
	}
	existing, ok := projection.Profiles[profileID]
	if !ok {
		return nil, ErrProfileNotFound
	}
	if projection.retired[profileID] {
		return nil, ErrProfileRetired
	}
	compiled.Generation = existing.Generation + 1
	compiled.Digest, err = digestProfile(compiled.ProfileID, compiled.Mode, compiled.Rules, compiled.OwnedPaths)
	if err != nil {
		return nil, err
	}
	streamID := streamProfile + profileID
	event := newPermEvent("PermissionProfileRevised", streamID, heads[streamID]+1,
		a.now(), journeyID, operationID, profileEventPayload{
			ProfileID: compiled.ProfileID, Generation: compiled.Generation, Digest: compiled.Digest,
			Mode: compiled.Mode, Rules: compiled.Rules, OwnedPaths: compiled.OwnedPaths,
		})
	return a.appendCAS(ctx, events, heads, streamID, event)
}

func (a *Authority) RetireProfile(ctx context.Context, profileID, operationID, journeyID string) ([]journal.Event, error) {
	if !validID(profileID) {
		return nil, ErrInvalidInput
	}
	projection, events, heads, err := a.latestProjection(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := projection.Profiles[profileID]; !ok {
		return nil, ErrProfileNotFound
	}
	if projection.retired[profileID] {
		return nil, ErrProfileRetired
	}
	streamID := streamProfile + profileID
	event := newPermEvent("PermissionProfileRetired", streamID, heads[streamID]+1,
		a.now(), journeyID, operationID, retiredPayload{ProfileID: profileID, RetiredAt: isoNow(a.now)})
	return a.appendCAS(ctx, events, heads, streamID, event)
}

func (a *Authority) AddRule(ctx context.Context, rule Rule, authorizedBy, operationID, journeyID string) ([]journal.Event, error) {
	if err := validateRule(rule); err != nil {
		return nil, err
	}
	if rule.Scope == ScopeRoot && authorizedBy == "" {
		return nil, ErrRootRequiresAuth
	}
	projection, events, heads, err := a.latestProjection(ctx)
	if err != nil {
		return nil, err
	}
	if _, exists := projection.Rules[rule.RuleID]; exists {
		return nil, ErrRuleExists
	}
	streamID := streamRule + rule.RuleID
	event := newPermEvent("PermissionRuleAdded", streamID, heads[streamID]+1,
		a.now(), journeyID, operationID, ruleEventPayload{
			RuleID: rule.RuleID, Scope: rule.Scope, ScopeID: rule.ScopeID,
			Action: rule.Action, Tool: rule.Tool, Pattern: rule.Pattern,
		})
	return a.appendCAS(ctx, events, heads, streamID, event)
}

func (a *Authority) RevokeRule(ctx context.Context, ruleID, authorizedBy, operationID, journeyID string) ([]journal.Event, error) {
	if !validID(ruleID) {
		return nil, ErrInvalidInput
	}
	projection, events, heads, err := a.latestProjection(ctx)
	if err != nil {
		return nil, err
	}
	rule, ok := projection.Rules[ruleID]
	if !ok {
		return nil, ErrRuleNotFound
	}
	if rule.Scope == ScopeRoot && authorizedBy == "" {
		return nil, ErrRootRequiresAuth
	}
	streamID := streamRule + ruleID
	event := newPermEvent("PermissionRuleRevoked", streamID, heads[streamID]+1,
		a.now(), journeyID, operationID, ruleRevokedPayload{RuleID: ruleID, RevokedAt: isoNow(a.now)})
	return a.appendCAS(ctx, events, heads, streamID, event)
}

func (a *Authority) BindJob(ctx context.Context, jobID, profileID, operationID, journeyID string) ([]journal.Event, error) {
	if !validID(jobID) || !validID(profileID) {
		return nil, ErrInvalidInput
	}
	projection, events, heads, err := a.latestProjection(ctx)
	if err != nil {
		return nil, err
	}
	profile, ok := projection.Profiles[profileID]
	if !ok {
		return nil, ErrProfileNotFound
	}
	if projection.retired[profileID] {
		return nil, ErrProfileRetired
	}
	streamID := streamBinding + jobID
	event := newPermEvent("JobPermissionBound", streamID, heads[streamID]+1,
		a.now(), journeyID, operationID, bindingPayload{
			JobID: jobID, ProfileID: profileID,
			ProfileDigest: profile.Digest, ProfileGeneration: profile.Generation,
			BoundAt: isoNow(a.now),
		})
	return a.appendCAS(ctx, events, heads, streamID, event)
}

func (a *Authority) IssueGrant(ctx context.Context, grant Grant, operationID, journeyID string) ([]journal.Event, error) {
	if err := validateGrant(grant); err != nil {
		return nil, err
	}
	projection, events, heads, err := a.latestProjection(ctx)
	if err != nil {
		return nil, err
	}
	if existing, ok := projection.Grants[grant.GrantID]; ok && existing.RevokedAt == "" {
		return nil, ErrGrantExists
	}
	streamID := streamGrant + grant.GrantID
	event := newPermEvent("PermissionGrantIssued", streamID, heads[streamID]+1,
		a.now(), journeyID, operationID, grantEventPayload{
			GrantID: grant.GrantID, Scope: grant.Scope, ScopeID: grant.ScopeID,
			Tool: grant.Tool, Pattern: grant.Pattern, IssuedAt: grant.IssuedAt,
		})
	return a.appendCAS(ctx, events, heads, streamID, event)
}

func (a *Authority) RevokeGrant(ctx context.Context, grantID, operationID, journeyID string) ([]journal.Event, error) {
	if !validID(grantID) {
		return nil, ErrInvalidInput
	}
	projection, events, heads, err := a.latestProjection(ctx)
	if err != nil {
		return nil, err
	}
	grant, ok := projection.Grants[grantID]
	if !ok || grant.RevokedAt != "" {
		return nil, ErrGrantNotFound
	}
	streamID := streamGrant + grantID
	event := newPermEvent("PermissionGrantRevoked", streamID, heads[streamID]+1,
		a.now(), journeyID, operationID, grantRevokedPayload{GrantID: grantID, RevokedAt: isoNow(a.now)})
	return a.appendCAS(ctx, events, heads, streamID, event)
}

func (a *Authority) ActivateMode(ctx context.Context, mode Mode, scope ScopeKind, scopeID, authorizedBy, operationID, journeyID string) ([]journal.Event, error) {
	if !ValidMode(string(mode)) || !ValidScopeKind(string(scope)) || !validID(scopeID) {
		return nil, ErrInvalidInput
	}
	if mode == ModeBypassPermissions && authorizedBy == "" {
		return nil, ErrBypassRequiresAuth
	}
	projection, events, heads, err := a.latestProjection(ctx)
	if err != nil {
		return nil, err
	}
	if projection.AdminLock && mode == ModeBypassPermissions {
		return nil, ErrAdminLockEnabled
	}
	streamID := streamActivation
	event := newPermEvent("PermissionActivationActivated", streamID, heads[streamID]+1,
		a.now(), journeyID, operationID, activationPayload{
			Mode: mode, Scope: scope, ScopeID: scopeID,
			ActivatedAt: isoNow(a.now), AuthorizedBy: authorizedBy,
		})
	return a.appendCAS(ctx, events, heads, streamID, event)
}

func (a *Authority) SetAdminLock(ctx context.Context, enabled bool, authorizedBy, operationID, journeyID string) ([]journal.Event, error) {
	if authorizedBy == "" {
		return nil, ErrInvalidInput
	}
	projection, events, heads, err := a.latestProjection(ctx)
	if err != nil {
		return nil, err
	}
	eventType := "PermissionAdminLockDisabled"
	payload := adminLockPayload{DisabledAt: isoNow(a.now), AuthorizedBy: authorizedBy}
	if enabled {
		eventType = "PermissionAdminLockEnabled"
		payload = adminLockPayload{EnabledAt: isoNow(a.now), AuthorizedBy: authorizedBy}
	}
	_ = projection
	streamID := streamAdminLock
	event := newPermEvent(eventType, streamID, heads[streamID]+1,
		a.now(), journeyID, operationID, payload)
	return a.appendCAS(ctx, events, heads, streamID, event)
}

// ResolveApproval is a typed forward-only boundary: the product service routes
// approval resolution to the existing internal/rules authority. This layer
// never writes approval lifecycle events.
func (a *Authority) ResolveApproval(ctx context.Context, approvalID, resolution, resolvedBy, operationID, journeyID string) ([]journal.Event, error) {
	if approvalID == "" || (resolution != "allow" && resolution != "deny") || resolvedBy == "" {
		return nil, ErrInvalidInput
	}
	_ = operationID
	_ = journeyID
	return nil, ErrApprovalForwardOnly
}

func (a *Authority) RecordDecision(ctx context.Context, jobID string, call ProposedCall, verdict Verdict, denial Denial, approvalID, operationID, journeyID string) ([]journal.Event, error) {
	if !validID(jobID) || !ValidToolKind(string(call.Tool)) ||
		(verdict != VerdictAllow && verdict != VerdictAsk && verdict != VerdictDeny) {
		return nil, ErrInvalidInput
	}
	projection, events, heads, err := a.latestProjection(ctx)
	if err != nil {
		return nil, err
	}
	_ = projection
	streamID := streamDecision + jobID
	event := newPermEvent("PermissionDecisionRecorded", streamID, heads[streamID]+1,
		a.now(), journeyID, operationID, decisionPayload{
			JobID: jobID, ApprovalID: approvalID, Verdict: verdict,
			Tool: call.Tool, Command: call.Command, Path: call.Path,
			Reason: denial.Reason, AuthorizationPath: denial.AuthorizationPath,
			RecordedAt: isoNow(a.now),
		})
	return a.appendCAS(ctx, events, heads, streamID, event)
}
