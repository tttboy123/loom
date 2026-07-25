package authorization

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

var (
	ErrInvalidGrantAuthorityInput = errors.New("invalid grant authority input")
	ErrGrantAuthorityConflict     = errors.New("grant authority conflict")
	ErrRunNotGrantable            = errors.New("run not grantable")
	ErrGrantAlreadyActive         = errors.New("grant already active")
	ErrGrantNotFound              = errors.New("grant not found")
	ErrGrantBindingMismatch       = errors.New("grant binding mismatch")
	ErrGrantOperationDenied       = errors.New("grant operation denied")
	ErrGrantExpired               = errors.New("grant expired")
	ErrGrantRevoked               = errors.New("grant revoked")
	ErrGrantAlreadyRevoked        = errors.New("grant already revoked")
	ErrGrantIDCollision           = errors.New("grant id collision")
	ErrGrantTokenCollision        = errors.New("grant token collision")
)

type Operation string

const (
	OperationBridgeAck       Operation = "bridge.ack"
	OperationBridgeEvent     Operation = "bridge.event"
	OperationBridgeEvidence  Operation = "bridge.evidence"
	OperationBridgeResult    Operation = "bridge.result"
	OperationBridgeHeartbeat Operation = "bridge.heartbeat"
	OperationContextRead     Operation = "context.read"
	OperationEvidenceStage   Operation = "evidence.stage"
)

type RevocationReason string

const (
	RevocationReplaced  RevocationReason = "replaced"
	RevocationExpired   RevocationReason = "expired"
	RevocationTerminal  RevocationReason = "terminal"
	RevocationCancelled RevocationReason = "cancelled"
	RevocationTimeout   RevocationReason = "timeout"
	RevocationOperator  RevocationReason = "operator"
)

const (
	tokenPrefix       = "loom_grant_v1."
	grantStreamPrefix = "agent-grant/"
	maxGrantLifetime  = time.Hour
)

type Token struct {
	value string
}

func ParseToken(raw string) (Token, error) {
	if !validTokenWire(raw) {
		return Token{}, ErrInvalidGrantAuthorityInput
	}
	return Token{value: raw}, nil
}

func (token Token) Value() string { return token.value }
func (token Token) String() string {
	return "[REDACTED]"
}
func (token Token) GoString() string {
	return "[REDACTED]"
}

type IssueInput struct {
	WorkItemID        string
	RunID             string
	ClaimID           string
	ClaimGeneration   int64
	RuntimeInstanceID string
	AgentInstanceID   string
	AllowedOperations []Operation
	Lifetime          time.Duration
	CorrelationID     string
}

type AuthorizeInput struct {
	Token             Token
	WorkItemID        string
	RunID             string
	ClaimID           string
	ClaimGeneration   int64
	RuntimeInstanceID string
	AgentInstanceID   string
	Operation         Operation
	RequestID         string
	CorrelationID     string
}

type RevokeInput struct {
	GrantID       string
	Reason        RevocationReason
	CorrelationID string
}

type GrantRecord struct {
	id                string
	workItemID        string
	runID             string
	claimID           string
	claimGeneration   int64
	runtimeInstanceID string
	agentInstanceID   string
	allowedOperations []Operation
	issuedAt          time.Time
	expiresAt         time.Time
	revokedAt         time.Time
	revocationReason  RevocationReason
	tokenHash         string
	issueEventID      string
	lastEventID       string
	streamSequence    int64
	revokeCorrelation string
	authorizations    map[string]authorizationDecision
}

type authorizationDecision struct {
	grantID           string
	workItemID        string
	runID             string
	claimID           string
	claimGeneration   int64
	runtimeInstanceID string
	agentInstanceID   string
	operation         Operation
	requestID         string
	authorizedAt      time.Time
	eventID           string
}

type IssuedGrant struct {
	record GrantRecord
	token  *redactedTokenBox
}

type redactedTokenBox struct {
	token Token
}

type AuthoritySnapshot struct {
	grants []GrantRecord
}

type Authority struct {
	store        *journal.Store
	runAuthority *work.Authority
	now          func() time.Time
	random       io.Reader
}

func NewAuthority(
	store *journal.Store,
	runAuthority *work.Authority,
	now func() time.Time,
	random io.Reader,
) (*Authority, error) {
	if store == nil || runAuthority == nil || now == nil || isNilReader(random) {
		return nil, ErrInvalidGrantAuthorityInput
	}
	return &Authority{
		store:        store,
		runAuthority: runAuthority,
		now:          now,
		random:       random,
	}, nil
}

func (authority *Authority) Issue(
	ctx context.Context,
	input IssueInput,
) (IssuedGrant, error) {
	operations, err := validateIssueInput(ctx, input)
	if err != nil {
		return IssuedGrant{}, err
	}
	now, err := authority.operationTime()
	if err != nil {
		return IssuedGrant{}, err
	}
	runHead, run, err := authority.currentRun(ctx, input.RunID)
	if err != nil {
		return IssuedGrant{}, err
	}
	if !issueBindingMatches(input, run) ||
		run.Phase() != "claimed" ||
		!now.Before(run.PrepareLeaseExpiresAt()) {
		return IssuedGrant{}, ErrRunNotGrantable
	}
	state, err := authority.readState(ctx)
	if err != nil {
		return IssuedGrant{}, err
	}
	streamID := grantStream(input.RunID)
	streamSequence := state.heads[streamID]
	events := make([]journal.Event, 0, 3)
	var previousEventID string
	if active := state.active[input.RunID]; active != nil {
		if active.claimGeneration == input.ClaimGeneration &&
			now.Before(active.expiresAt) {
			return IssuedGrant{}, ErrGrantAlreadyActive
		}
		reason := RevocationReplaced
		if active.claimGeneration == input.ClaimGeneration {
			reason = RevocationExpired
		}
		streamSequence++
		revokeEvent := newGrantEvent(
			deterministicGrantEventID(
				"AgentGrantRevoked",
				active.id,
				string(reason),
				runHead.ID,
			),
			streamID,
			streamSequence,
			"AgentGrantRevoked",
			now,
			input.CorrelationID,
			active.lastEventID,
			revokePayload{
				GrantID:     active.id,
				Reason:      reason,
				RevokedAt:   now.Format(time.RFC3339Nano),
				RunStream:   runHead.StreamID,
				RunSequence: runHead.Seq,
				RunEventID:  runHead.ID,
			},
		)
		events = append(events, revokeEvent)
		previousEventID = revokeEvent.ID
	}

	material := make([]byte, 48)
	if _, err := io.ReadFull(authority.random, material); err != nil {
		return IssuedGrant{}, fmt.Errorf("%w: random source", ErrInvalidGrantAuthorityInput)
	}
	grantID := grantUUID(material[:16])
	token := Token{value: tokenPrefix + grantID + "." +
		base64.RawURLEncoding.EncodeToString(material[16:])}
	tokenDigest := sha256.Sum256([]byte(token.value))
	tokenHash := hex.EncodeToString(tokenDigest[:])
	if _, exists := state.byID[grantID]; exists {
		return IssuedGrant{}, ErrGrantIDCollision
	}
	if _, exists := state.byHash[tokenHash]; exists {
		return IssuedGrant{}, ErrGrantTokenCollision
	}
	expiresAt := now.Add(input.Lifetime)
	if !expiresAt.After(now) {
		return IssuedGrant{}, ErrInvalidGrantAuthorityInput
	}
	streamSequence++
	issueID := deterministicGrantEventID(
		"AgentGrantIssued",
		grantID,
		input.RunID,
		fmt.Sprint(input.ClaimGeneration),
		input.CorrelationID,
	)
	if previousEventID == "" {
		previousEventID = runHead.ID
	}
	issueEvent := newGrantEvent(
		issueID,
		streamID,
		streamSequence,
		"AgentGrantIssued",
		now,
		input.CorrelationID,
		previousEventID,
		issuePayload{
			GrantID:           grantID,
			WorkItemID:        input.WorkItemID,
			RunID:             input.RunID,
			ClaimID:           input.ClaimID,
			ClaimGeneration:   input.ClaimGeneration,
			RuntimeInstanceID: input.RuntimeInstanceID,
			AgentInstanceID:   input.AgentInstanceID,
			AllowedOperations: operations,
			TokenHash:         tokenHash,
			IssuedAt:          now.Format(time.RFC3339Nano),
			ExpiresAt:         expiresAt.Format(time.RFC3339Nano),
			RunStream:         runHead.StreamID,
			RunSequence:       runHead.Seq,
			RunEventID:        runHead.ID,
		},
	)
	events = append(events, issueEvent)
	if err := ctx.Err(); err != nil {
		return IssuedGrant{}, err
	}
	_, err = authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{
			{StreamID: runHead.StreamID, Sequence: runHead.Seq},
			{StreamID: streamID, Sequence: state.heads[streamID]},
		},
		events,
	)
	if err != nil {
		return IssuedGrant{}, mapGrantWriteError(err)
	}
	record := GrantRecord{
		id:                grantID,
		workItemID:        input.WorkItemID,
		runID:             input.RunID,
		claimID:           input.ClaimID,
		claimGeneration:   input.ClaimGeneration,
		runtimeInstanceID: input.RuntimeInstanceID,
		agentInstanceID:   input.AgentInstanceID,
		allowedOperations: cloneOperations(operations),
		issuedAt:          now,
		expiresAt:         expiresAt,
		tokenHash:         tokenHash,
		issueEventID:      issueID,
		lastEventID:       issueID,
		streamSequence:    streamSequence,
		authorizations:    make(map[string]authorizationDecision),
	}
	return IssuedGrant{
		record: record.public(),
		token:  &redactedTokenBox{token: token},
	}, nil
}

func (authority *Authority) Authorize(
	ctx context.Context,
	input AuthorizeInput,
) (GrantRecord, error) {
	if err := validateAuthorizeInput(ctx, input); err != nil {
		return GrantRecord{}, err
	}
	now, err := authority.operationTime()
	if err != nil {
		return GrantRecord{}, err
	}
	runHead, run, err := authority.currentRun(ctx, input.RunID)
	if err != nil {
		return GrantRecord{}, err
	}
	state, err := authority.readState(ctx)
	if err != nil {
		return GrantRecord{}, err
	}
	digest := sha256.Sum256([]byte(input.Token.value))
	tokenHash := hex.EncodeToString(digest[:])
	record := state.byHash[tokenHash]
	if record == nil || !constantTimeHashEqual(record.tokenHash, tokenHash) {
		return GrantRecord{}, ErrGrantNotFound
	}
	if record.revocationReason != "" {
		return GrantRecord{}, ErrGrantRevoked
	}
	if !now.Before(record.expiresAt) {
		return GrantRecord{}, ErrGrantExpired
	}
	if !authorizeBindingMatches(input, record) {
		return GrantRecord{}, ErrGrantBindingMismatch
	}
	if !hasOperation(record.allowedOperations, input.Operation) {
		return GrantRecord{}, ErrGrantOperationDenied
	}
	if !authorizeRunMatches(input, run, now) {
		return GrantRecord{}, ErrRunNotGrantable
	}
	if decision, exists := state.requests[input.RunID][input.RequestID]; exists {
		if sameAuthorization(input, record, decision) {
			return record.public(), nil
		}
		return GrantRecord{}, ErrGrantAuthorityConflict
	}
	streamID := grantStream(input.RunID)
	eventID := deterministicGrantEventID(
		"AgentGrantAuthorized",
		input.RunID,
		input.RequestID,
	)
	event := newGrantEvent(
		eventID,
		streamID,
		state.heads[streamID]+1,
		"AgentGrantAuthorized",
		now,
		input.CorrelationID,
		record.lastEventID,
		authorizePayload{
			GrantID:           record.id,
			WorkItemID:        input.WorkItemID,
			RunID:             input.RunID,
			ClaimID:           input.ClaimID,
			ClaimGeneration:   input.ClaimGeneration,
			RuntimeInstanceID: input.RuntimeInstanceID,
			AgentInstanceID:   input.AgentInstanceID,
			Operation:         input.Operation,
			RequestID:         input.RequestID,
			AuthorizedAt:      now.Format(time.RFC3339Nano),
			RunStream:         runHead.StreamID,
			RunSequence:       runHead.Seq,
			RunEventID:        runHead.ID,
		},
	)
	if err := ctx.Err(); err != nil {
		return GrantRecord{}, err
	}
	_, err = authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{
			{StreamID: runHead.StreamID, Sequence: runHead.Seq},
			{StreamID: streamID, Sequence: state.heads[streamID]},
		},
		[]journal.Event{event},
	)
	if err != nil {
		return GrantRecord{}, mapGrantWriteError(err)
	}
	return record.public(), nil
}

func (authority *Authority) Revoke(
	ctx context.Context,
	input RevokeInput,
) (GrantRecord, error) {
	if err := validateRevokeInput(ctx, input); err != nil {
		return GrantRecord{}, err
	}
	now, err := authority.operationTime()
	if err != nil {
		return GrantRecord{}, err
	}
	state, err := authority.readState(ctx)
	if err != nil {
		return GrantRecord{}, err
	}
	record := state.byID[input.GrantID]
	if record == nil {
		return GrantRecord{}, ErrGrantNotFound
	}
	if record.revocationReason != "" {
		if record.revocationReason == input.Reason &&
			record.revokeCorrelation == input.CorrelationID {
			return record.public(), nil
		}
		return GrantRecord{}, ErrGrantAlreadyRevoked
	}
	runHead, run, err := authority.currentRun(ctx, record.runID)
	if err != nil {
		return GrantRecord{}, err
	}
	if run.ID() != record.runID ||
		run.WorkItemID() != record.workItemID ||
		run.ClaimGeneration() < record.claimGeneration {
		return GrantRecord{}, ErrGrantBindingMismatch
	}
	streamID := grantStream(record.runID)
	eventID := deterministicGrantEventID(
		"AgentGrantRevoked",
		record.id,
		string(input.Reason),
		input.CorrelationID,
	)
	event := newGrantEvent(
		eventID,
		streamID,
		state.heads[streamID]+1,
		"AgentGrantRevoked",
		now,
		input.CorrelationID,
		record.lastEventID,
		revokePayload{
			GrantID:     record.id,
			Reason:      input.Reason,
			RevokedAt:   now.Format(time.RFC3339Nano),
			RunStream:   runHead.StreamID,
			RunSequence: runHead.Seq,
			RunEventID:  runHead.ID,
		},
	)
	if err := ctx.Err(); err != nil {
		return GrantRecord{}, err
	}
	_, err = authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{
			{StreamID: runHead.StreamID, Sequence: runHead.Seq},
			{StreamID: streamID, Sequence: state.heads[streamID]},
		},
		[]journal.Event{event},
	)
	if err != nil {
		return GrantRecord{}, mapGrantWriteError(err)
	}
	updated := record.clone()
	updated.revokedAt = now
	updated.revocationReason = input.Reason
	updated.revokeCorrelation = input.CorrelationID
	updated.lastEventID = eventID
	updated.streamSequence++
	return updated.public(), nil
}

func (authority *Authority) Snapshot(ctx context.Context) (AuthoritySnapshot, error) {
	if ctx == nil {
		return AuthoritySnapshot{}, ErrInvalidGrantAuthorityInput
	}
	if err := ctx.Err(); err != nil {
		return AuthoritySnapshot{}, err
	}
	state, err := authority.readState(ctx)
	if err != nil {
		return AuthoritySnapshot{}, err
	}
	grants := make([]GrantRecord, 0, len(state.byID))
	for _, record := range state.byID {
		grants = append(grants, record.public())
	}
	sort.Slice(grants, func(left, right int) bool {
		return grants[left].id < grants[right].id
	})
	return AuthoritySnapshot{grants: grants}, nil
}

func (record GrantRecord) ID() string                { return record.id }
func (record GrantRecord) WorkItemID() string        { return record.workItemID }
func (record GrantRecord) RunID() string             { return record.runID }
func (record GrantRecord) ClaimID() string           { return record.claimID }
func (record GrantRecord) ClaimGeneration() int64    { return record.claimGeneration }
func (record GrantRecord) RuntimeInstanceID() string { return record.runtimeInstanceID }
func (record GrantRecord) AgentInstanceID() string   { return record.agentInstanceID }
func (record GrantRecord) AllowedOperations() []Operation {
	return cloneOperations(record.allowedOperations)
}
func (record GrantRecord) IssuedAt() time.Time                { return record.issuedAt }
func (record GrantRecord) ExpiresAt() time.Time               { return record.expiresAt }
func (record GrantRecord) RevokedAt() time.Time               { return record.revokedAt }
func (record GrantRecord) RevocationReason() RevocationReason { return record.revocationReason }
func (issued IssuedGrant) Record() GrantRecord                { return issued.record.public() }
func (issued IssuedGrant) Token() Token {
	if issued.token == nil {
		return Token{}
	}
	return issued.token.token
}
func (snapshot AuthoritySnapshot) Grants() []GrantRecord {
	output := make([]GrantRecord, len(snapshot.grants))
	for index := range snapshot.grants {
		output[index] = snapshot.grants[index].public()
	}
	return output
}

type grantState struct {
	byID          map[string]*GrantRecord
	byHash        map[string]*GrantRecord
	active        map[string]*GrantRecord
	requests      map[string]map[string]authorizationDecision
	heads         map[string]int64
	runReferences map[string]runReference
}

func (authority *Authority) readState(ctx context.Context) (*grantState, error) {
	events, err := authority.store.ReadAll(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := authority.runAuthority.Snapshot(ctx); err != nil {
		return nil, err
	}
	runReferences, err := indexGrantRunReferences(events)
	if err != nil {
		return nil, err
	}
	state := &grantState{
		byID:          make(map[string]*GrantRecord),
		byHash:        make(map[string]*GrantRecord),
		active:        make(map[string]*GrantRecord),
		requests:      make(map[string]map[string]authorizationDecision),
		heads:         make(map[string]int64),
		runReferences: runReferences,
	}
	for _, event := range events {
		if !strings.HasPrefix(event.StreamID, grantStreamPrefix) {
			continue
		}
		if err := applyGrantEvent(state, event); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func applyGrantEvent(state *grantState, event journal.Event) error {
	if event.SchemaVersion != 1 ||
		event.Seq != state.heads[event.StreamID]+1 ||
		event.EmittedAt.IsZero() ||
		event.EmittedAt.Location() != time.UTC ||
		!validCanonicalUUID(event.CorrelationID) {
		return ErrInvalidGrantAuthorityInput
	}
	runID := strings.TrimPrefix(event.StreamID, grantStreamPrefix)
	if !validOpaqueID(runID) || grantStream(runID) != event.StreamID {
		return ErrInvalidGrantAuthorityInput
	}
	switch event.Type {
	case "AgentGrantIssued":
		var payload issuePayload
		if decodeExactGrantPayload(event.PayloadJSON, &payload) != nil ||
			!payload.valid() ||
			payload.RunID != runID ||
			payload.RunStream != runStream(runID) ||
			payload.RunSequence <= 0 ||
			!validGrantRunReference(
				state.runReferences,
				payload.RunStream,
				payload.RunSequence,
				payload.RunEventID,
				event.EmittedAt,
			) ||
			event.EmittedAt.Format(time.RFC3339Nano) != payload.IssuedAt {
			return ErrInvalidGrantAuthorityInput
		}
		if _, exists := state.byID[payload.GrantID]; exists {
			return ErrGrantIDCollision
		}
		if _, exists := state.byHash[payload.TokenHash]; exists {
			return ErrGrantTokenCollision
		}
		if active := state.active[runID]; active != nil {
			return ErrInvalidGrantAuthorityInput
		}
		issuedAt, _ := time.Parse(time.RFC3339Nano, payload.IssuedAt)
		expiresAt, _ := time.Parse(time.RFC3339Nano, payload.ExpiresAt)
		record := &GrantRecord{
			id:                payload.GrantID,
			workItemID:        payload.WorkItemID,
			runID:             payload.RunID,
			claimID:           payload.ClaimID,
			claimGeneration:   payload.ClaimGeneration,
			runtimeInstanceID: payload.RuntimeInstanceID,
			agentInstanceID:   payload.AgentInstanceID,
			allowedOperations: cloneOperations(payload.AllowedOperations),
			issuedAt:          issuedAt,
			expiresAt:         expiresAt,
			tokenHash:         payload.TokenHash,
			issueEventID:      event.ID,
			lastEventID:       event.ID,
			streamSequence:    event.Seq,
			authorizations:    make(map[string]authorizationDecision),
		}
		state.byID[record.id] = record
		state.byHash[record.tokenHash] = record
		state.active[runID] = record
	case "AgentGrantAuthorized":
		var payload authorizePayload
		if decodeExactGrantPayload(event.PayloadJSON, &payload) != nil ||
			!payload.valid() ||
			payload.RunID != runID ||
			payload.RunStream != runStream(runID) ||
			payload.RunSequence <= 0 ||
			!validGrantRunReference(
				state.runReferences,
				payload.RunStream,
				payload.RunSequence,
				payload.RunEventID,
				event.EmittedAt,
			) ||
			event.EmittedAt.Format(time.RFC3339Nano) != payload.AuthorizedAt {
			return ErrInvalidGrantAuthorityInput
		}
		record := state.byID[payload.GrantID]
		if record == nil || record.lastEventID != event.CausationID ||
			record.revocationReason != "" ||
			!bindingPayloadMatches(record, payload.binding()) ||
			!hasOperation(record.allowedOperations, payload.Operation) {
			return ErrInvalidGrantAuthorityInput
		}
		authorizedAt, _ := time.Parse(time.RFC3339Nano, payload.AuthorizedAt)
		if !authorizedAt.Before(record.expiresAt) {
			return ErrInvalidGrantAuthorityInput
		}
		if state.requests[runID] == nil {
			state.requests[runID] = make(map[string]authorizationDecision)
		}
		if _, exists := state.requests[runID][payload.RequestID]; exists {
			return ErrInvalidGrantAuthorityInput
		}
		decision := authorizationDecision{
			grantID:           record.id,
			workItemID:        record.workItemID,
			runID:             record.runID,
			claimID:           record.claimID,
			claimGeneration:   record.claimGeneration,
			runtimeInstanceID: record.runtimeInstanceID,
			agentInstanceID:   record.agentInstanceID,
			operation:         payload.Operation,
			requestID:         payload.RequestID,
			authorizedAt:      authorizedAt,
			eventID:           event.ID,
		}
		state.requests[runID][payload.RequestID] = decision
		record.authorizations[payload.RequestID] = decision
		record.lastEventID = event.ID
		record.streamSequence = event.Seq
	case "AgentGrantRevoked":
		var payload revokePayload
		if decodeExactGrantPayload(event.PayloadJSON, &payload) != nil ||
			!payload.valid() ||
			payload.RunStream != runStream(runID) ||
			payload.RunSequence <= 0 ||
			!validGrantRunReference(
				state.runReferences,
				payload.RunStream,
				payload.RunSequence,
				payload.RunEventID,
				event.EmittedAt,
			) ||
			event.EmittedAt.Format(time.RFC3339Nano) != payload.RevokedAt {
			return ErrInvalidGrantAuthorityInput
		}
		record := state.byID[payload.GrantID]
		if record == nil || record.runID != runID ||
			record.lastEventID != event.CausationID ||
			record.revocationReason != "" {
			return ErrInvalidGrantAuthorityInput
		}
		revokedAt, _ := time.Parse(time.RFC3339Nano, payload.RevokedAt)
		if revokedAt.Before(record.issuedAt) {
			return ErrInvalidGrantAuthorityInput
		}
		record.revokedAt = revokedAt
		record.revocationReason = payload.Reason
		record.revokeCorrelation = event.CorrelationID
		record.lastEventID = event.ID
		record.streamSequence = event.Seq
		delete(state.active, runID)
	default:
		return ErrInvalidGrantAuthorityInput
	}
	state.heads[event.StreamID] = event.Seq
	return nil
}

type runReference struct {
	StreamID  string
	Seq       int64
	ID        string
	EmittedAt time.Time
}

func indexGrantRunReferences(
	events []journal.Event,
) (map[string]runReference, error) {
	references := make(map[string]runReference)
	for _, event := range events {
		if !strings.HasPrefix(event.StreamID, "run/") {
			continue
		}
		runID := strings.TrimPrefix(event.StreamID, "run/")
		if !validOpaqueID(runID) ||
			runStream(runID) != event.StreamID ||
			event.ID == "" ||
			event.Seq <= 0 ||
			event.EmittedAt.IsZero() ||
			event.EmittedAt.Location() != time.UTC {
			return nil, ErrInvalidGrantAuthorityInput
		}
		if _, exists := references[event.ID]; exists {
			return nil, ErrInvalidGrantAuthorityInput
		}
		references[event.ID] = runReference{
			StreamID:  event.StreamID,
			Seq:       event.Seq,
			ID:        event.ID,
			EmittedAt: event.EmittedAt,
		}
	}
	return references, nil
}

func validGrantRunReference(
	references map[string]runReference,
	streamID string,
	sequence int64,
	eventID string,
	grantTime time.Time,
) bool {
	reference, exists := references[eventID]
	return exists &&
		reference.StreamID == streamID &&
		reference.Seq == sequence &&
		reference.ID == eventID &&
		!grantTime.Before(reference.EmittedAt)
}

func (authority *Authority) currentRun(
	ctx context.Context,
	runID string,
) (runReference, work.RunRecord, error) {
	events, err := authority.store.ReadStream(ctx, runStream(runID))
	if err != nil {
		return runReference{}, work.RunRecord{}, err
	}
	if len(events) == 0 {
		return runReference{}, work.RunRecord{}, ErrRunNotGrantable
	}
	head := events[len(events)-1]
	snapshot, err := authority.runAuthority.Snapshot(ctx)
	if err != nil {
		return runReference{}, work.RunRecord{}, err
	}
	for _, run := range snapshot.Runs() {
		if run.ID() == runID {
			return runReference{
				StreamID:  head.StreamID,
				Seq:       head.Seq,
				ID:        head.ID,
				EmittedAt: head.EmittedAt,
			}, run, nil
		}
	}
	return runReference{}, work.RunRecord{}, ErrRunNotGrantable
}

type issuePayload struct {
	GrantID           string      `json:"grant_id"`
	WorkItemID        string      `json:"work_item_id"`
	RunID             string      `json:"run_id"`
	ClaimID           string      `json:"claim_id"`
	ClaimGeneration   int64       `json:"claim_generation"`
	RuntimeInstanceID string      `json:"runtime_instance_id"`
	AgentInstanceID   string      `json:"agent_instance_id"`
	AllowedOperations []Operation `json:"allowed_operations"`
	TokenHash         string      `json:"token_hash"`
	IssuedAt          string      `json:"issued_at"`
	ExpiresAt         string      `json:"expires_at"`
	RunStream         string      `json:"run_stream"`
	RunSequence       int64       `json:"run_sequence"`
	RunEventID        string      `json:"run_event_id"`
}

type authorizePayload struct {
	GrantID           string    `json:"grant_id"`
	WorkItemID        string    `json:"work_item_id"`
	RunID             string    `json:"run_id"`
	ClaimID           string    `json:"claim_id"`
	ClaimGeneration   int64     `json:"claim_generation"`
	RuntimeInstanceID string    `json:"runtime_instance_id"`
	AgentInstanceID   string    `json:"agent_instance_id"`
	Operation         Operation `json:"operation"`
	RequestID         string    `json:"request_id"`
	AuthorizedAt      string    `json:"authorized_at"`
	RunStream         string    `json:"run_stream"`
	RunSequence       int64     `json:"run_sequence"`
	RunEventID        string    `json:"run_event_id"`
}

type revokePayload struct {
	GrantID     string           `json:"grant_id"`
	Reason      RevocationReason `json:"reason"`
	RevokedAt   string           `json:"revoked_at"`
	RunStream   string           `json:"run_stream"`
	RunSequence int64            `json:"run_sequence"`
	RunEventID  string           `json:"run_event_id"`
}

type binding struct {
	workItemID        string
	runID             string
	claimID           string
	claimGeneration   int64
	runtimeInstanceID string
	agentInstanceID   string
}

func (payload authorizePayload) binding() binding {
	return binding{
		workItemID:        payload.WorkItemID,
		runID:             payload.RunID,
		claimID:           payload.ClaimID,
		claimGeneration:   payload.ClaimGeneration,
		runtimeInstanceID: payload.RuntimeInstanceID,
		agentInstanceID:   payload.AgentInstanceID,
	}
}

func (payload issuePayload) valid() bool {
	issuedAt, issuedErr := time.Parse(time.RFC3339Nano, payload.IssuedAt)
	expiresAt, expiresErr := time.Parse(time.RFC3339Nano, payload.ExpiresAt)
	return validCanonicalUUID(payload.GrantID) &&
		validOpaqueID(payload.WorkItemID) &&
		validOpaqueID(payload.RunID) &&
		validCanonicalUUID(payload.ClaimID) &&
		payload.ClaimGeneration > 0 &&
		validOpaqueID(payload.RuntimeInstanceID) &&
		validOpaqueID(payload.AgentInstanceID) &&
		validCanonicalOperations(payload.AllowedOperations) &&
		validTokenHash(payload.TokenHash) &&
		issuedErr == nil && issuedAt.Location() == time.UTC &&
		expiresErr == nil && expiresAt.Location() == time.UTC &&
		expiresAt.After(issuedAt) &&
		expiresAt.Sub(issuedAt) <= maxGrantLifetime &&
		payload.RunSequence > 0 &&
		validOpaqueID(payload.RunEventID)
}

func (payload authorizePayload) valid() bool {
	authorizedAt, err := time.Parse(time.RFC3339Nano, payload.AuthorizedAt)
	return validCanonicalUUID(payload.GrantID) &&
		validOpaqueID(payload.WorkItemID) &&
		validOpaqueID(payload.RunID) &&
		validCanonicalUUID(payload.ClaimID) &&
		payload.ClaimGeneration > 0 &&
		validOpaqueID(payload.RuntimeInstanceID) &&
		validOpaqueID(payload.AgentInstanceID) &&
		validOperation(payload.Operation) &&
		validCanonicalUUID(payload.RequestID) &&
		err == nil && authorizedAt.Location() == time.UTC &&
		payload.RunSequence > 0 &&
		validOpaqueID(payload.RunEventID)
}

func (payload revokePayload) valid() bool {
	revokedAt, err := time.Parse(time.RFC3339Nano, payload.RevokedAt)
	return validCanonicalUUID(payload.GrantID) &&
		validRevocationReason(payload.Reason) &&
		err == nil && revokedAt.Location() == time.UTC &&
		payload.RunSequence > 0 &&
		validOpaqueID(payload.RunEventID)
}

func (authority *Authority) operationTime() (time.Time, error) {
	now := authority.now()
	if now.IsZero() || now.Location() != time.UTC {
		return time.Time{}, ErrInvalidGrantAuthorityInput
	}
	return now, nil
}

func validateIssueInput(ctx context.Context, input IssueInput) ([]Operation, error) {
	if ctx == nil {
		return nil, ErrInvalidGrantAuthorityInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validOpaqueID(input.WorkItemID) ||
		!validOpaqueID(input.RunID) ||
		!validCanonicalUUID(input.ClaimID) ||
		input.ClaimGeneration <= 0 ||
		!validOpaqueID(input.RuntimeInstanceID) ||
		!validOpaqueID(input.AgentInstanceID) ||
		input.Lifetime <= 0 ||
		input.Lifetime > maxGrantLifetime ||
		!validCanonicalUUID(input.CorrelationID) {
		return nil, ErrInvalidGrantAuthorityInput
	}
	operations := cloneOperations(input.AllowedOperations)
	sort.Slice(operations, func(left, right int) bool {
		return operations[left] < operations[right]
	})
	if !validCanonicalOperations(operations) {
		return nil, ErrInvalidGrantAuthorityInput
	}
	return operations, nil
}

func validateAuthorizeInput(ctx context.Context, input AuthorizeInput) error {
	if ctx == nil {
		return ErrInvalidGrantAuthorityInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validTokenWire(input.Token.value) ||
		!validOpaqueID(input.WorkItemID) ||
		!validOpaqueID(input.RunID) ||
		!validCanonicalUUID(input.ClaimID) ||
		input.ClaimGeneration <= 0 ||
		!validOpaqueID(input.RuntimeInstanceID) ||
		!validOpaqueID(input.AgentInstanceID) ||
		!validOperation(input.Operation) ||
		!validCanonicalUUID(input.RequestID) ||
		!validCanonicalUUID(input.CorrelationID) {
		return ErrInvalidGrantAuthorityInput
	}
	return nil
}

func validateRevokeInput(ctx context.Context, input RevokeInput) error {
	if ctx == nil {
		return ErrInvalidGrantAuthorityInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validCanonicalUUID(input.GrantID) ||
		!validRevocationReason(input.Reason) ||
		!validCanonicalUUID(input.CorrelationID) {
		return ErrInvalidGrantAuthorityInput
	}
	return nil
}

func issueBindingMatches(input IssueInput, run work.RunRecord) bool {
	return run.ID() == input.RunID &&
		run.WorkItemID() == input.WorkItemID &&
		run.ClaimID() == input.ClaimID &&
		run.ClaimGeneration() == input.ClaimGeneration &&
		run.RuntimeInstanceID() == input.RuntimeInstanceID &&
		run.AgentInstanceID() == input.AgentInstanceID
}

func authorizeBindingMatches(input AuthorizeInput, record *GrantRecord) bool {
	return record.workItemID == input.WorkItemID &&
		record.runID == input.RunID &&
		record.claimID == input.ClaimID &&
		record.claimGeneration == input.ClaimGeneration &&
		record.runtimeInstanceID == input.RuntimeInstanceID &&
		record.agentInstanceID == input.AgentInstanceID
}

func authorizeRunMatches(
	input AuthorizeInput,
	run work.RunRecord,
	now time.Time,
) bool {
	if run.ID() != input.RunID ||
		run.WorkItemID() != input.WorkItemID ||
		run.ClaimID() != input.ClaimID ||
		run.ClaimGeneration() != input.ClaimGeneration ||
		run.RuntimeInstanceID() != input.RuntimeInstanceID ||
		run.AgentInstanceID() != input.AgentInstanceID ||
		run.TerminalStatus() != "" {
		return false
	}
	switch run.Phase() {
	case "claimed":
		return now.Before(run.PrepareLeaseExpiresAt())
	case "running":
		return true
	default:
		return false
	}
}

func sameAuthorization(
	input AuthorizeInput,
	record *GrantRecord,
	decision authorizationDecision,
) bool {
	return decision.grantID == record.id &&
		decision.workItemID == input.WorkItemID &&
		decision.runID == input.RunID &&
		decision.claimID == input.ClaimID &&
		decision.claimGeneration == input.ClaimGeneration &&
		decision.runtimeInstanceID == input.RuntimeInstanceID &&
		decision.agentInstanceID == input.AgentInstanceID &&
		decision.operation == input.Operation &&
		decision.requestID == input.RequestID
}

func bindingPayloadMatches(record *GrantRecord, value binding) bool {
	return record.workItemID == value.workItemID &&
		record.runID == value.runID &&
		record.claimID == value.claimID &&
		record.claimGeneration == value.claimGeneration &&
		record.runtimeInstanceID == value.runtimeInstanceID &&
		record.agentInstanceID == value.agentInstanceID
}

func (record GrantRecord) clone() GrantRecord {
	record.allowedOperations = cloneOperations(record.allowedOperations)
	record.authorizations = cloneAuthorizations(record.authorizations)
	return record
}

func (record GrantRecord) public() GrantRecord {
	output := record.clone()
	output.tokenHash = ""
	output.issueEventID = ""
	output.lastEventID = ""
	output.streamSequence = 0
	output.revokeCorrelation = ""
	output.authorizations = nil
	return output
}

func cloneOperations(input []Operation) []Operation {
	if input == nil {
		return nil
	}
	return append([]Operation(nil), input...)
}

func cloneAuthorizations(
	input map[string]authorizationDecision,
) map[string]authorizationDecision {
	if input == nil {
		return nil
	}
	output := make(map[string]authorizationDecision, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func hasOperation(operations []Operation, wanted Operation) bool {
	index := sort.Search(len(operations), func(index int) bool {
		return operations[index] >= wanted
	})
	return index < len(operations) && operations[index] == wanted
}

func validCanonicalOperations(operations []Operation) bool {
	if len(operations) == 0 || len(operations) > 7 {
		return false
	}
	for index, operation := range operations {
		if !validOperation(operation) ||
			index > 0 && operations[index-1] >= operation {
			return false
		}
	}
	return true
}

func validOperation(operation Operation) bool {
	switch operation {
	case OperationBridgeAck,
		OperationBridgeEvent,
		OperationBridgeEvidence,
		OperationBridgeResult,
		OperationBridgeHeartbeat,
		OperationContextRead,
		OperationEvidenceStage:
		return true
	default:
		return false
	}
}

func validRevocationReason(reason RevocationReason) bool {
	switch reason {
	case RevocationReplaced,
		RevocationExpired,
		RevocationTerminal,
		RevocationCancelled,
		RevocationTimeout,
		RevocationOperator:
		return true
	default:
		return false
	}
}

func validTokenWire(raw string) bool {
	if len(raw) != len(tokenPrefix)+36+1+43 ||
		!strings.HasPrefix(raw, tokenPrefix) {
		return false
	}
	remainder := strings.TrimPrefix(raw, tokenPrefix)
	separator := strings.IndexByte(remainder, '.')
	if separator != 36 ||
		strings.IndexByte(remainder[separator+1:], '.') >= 0 {
		return false
	}
	grantID := remainder[:separator]
	secretText := remainder[separator+1:]
	if !validCanonicalUUID(grantID) || len(secretText) != 43 {
		return false
	}
	secret, err := base64.RawURLEncoding.DecodeString(secretText)
	return err == nil &&
		len(secret) == 32 &&
		base64.RawURLEncoding.EncodeToString(secret) == secretText
}

func validTokenHash(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func validCanonicalUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range []byte(value) {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			if !((character >= '0' && character <= '9') ||
				(character >= 'a' && character <= 'f')) {
				return false
			}
		}
	}
	return value[14] == '4' &&
		value[19] >= '8' && value[19] <= 'b'
}

func validOpaqueID(value string) bool {
	if value == "" || len(value) > 128 ||
		!utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func grantUUID(input []byte) string {
	material := append([]byte(nil), input...)
	material[6] = material[6]&0x0f | 0x40
	material[8] = material[8]&0x3f | 0x80
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		material[0:4],
		material[4:6],
		material[6:8],
		material[8:10],
		material[10:16],
	)
}

func deterministicGrantEventID(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}

func newGrantEvent(
	id string,
	streamID string,
	sequence int64,
	eventType string,
	emittedAt time.Time,
	correlationID string,
	causationID string,
	payload any,
) journal.Event {
	body, _ := json.Marshal(payload)
	return journal.Event{
		ID:             id,
		StreamID:       streamID,
		Seq:            sequence,
		IdempotencyKey: id,
		Type:           eventType,
		SchemaVersion:  1,
		EmittedAt:      emittedAt,
		CorrelationID:  correlationID,
		CausationID:    causationID,
		PayloadJSON:    body,
	}
}

func decodeExactGrantPayload(body []byte, destination any) error {
	if !json.Valid(body) || hasDuplicateGrantJSONKeys(body) {
		return ErrInvalidGrantAuthorityInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ErrInvalidGrantAuthorityInput
	}
	return nil
}

func hasDuplicateGrantJSONKeys(body []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	return scanDuplicateGrantJSONValue(decoder)
}

func scanDuplicateGrantJSONValue(decoder *json.Decoder) bool {
	token, err := decoder.Token()
	if err != nil {
		return true
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return false
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return true
			}
			key, ok := keyToken.(string)
			if !ok {
				return true
			}
			if _, duplicate := seen[key]; duplicate {
				return true
			}
			seen[key] = struct{}{}
			if scanDuplicateGrantJSONValue(decoder) {
				return true
			}
		}
		_, err = decoder.Token()
		return err != nil
	case '[':
		for decoder.More() {
			if scanDuplicateGrantJSONValue(decoder) {
				return true
			}
		}
		_, err = decoder.Token()
		return err != nil
	default:
		return true
	}
}

func mapGrantWriteError(err error) error {
	switch {
	case errors.Is(err, journal.ErrStreamHeadConflict),
		errors.Is(err, journal.ErrSequenceConflict),
		errors.Is(err, journal.ErrIdempotencyConflict),
		errors.Is(err, journal.ErrPartialEventBatchConflict):
		return fmt.Errorf("%w: %v", ErrGrantAuthorityConflict, err)
	default:
		return err
	}
}

func grantStream(runID string) string { return grantStreamPrefix + runID }
func runStream(runID string) string   { return "run/" + runID }

func isNilReader(reader io.Reader) bool {
	if reader == nil {
		return true
	}
	value := reflect.ValueOf(reader)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func constantTimeHashEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
