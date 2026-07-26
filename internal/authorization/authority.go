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
	"sync"
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
	ErrGrantIdentityIndexRequired = errors.New("Grant identity index required")
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
	tokenPrefix           = "loom_grant_v1."
	grantStreamPrefix     = "agent-grant/"
	grantIdentityStreamID = "agent-grant-identity/v1"
	maxGrantLifetime      = time.Hour
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
	store         *journal.Store
	runAuthority  *work.Authority
	now           func() time.Time
	random        io.Reader
	identityMu    sync.RWMutex
	identityReady bool
	identityByID  map[string]grantIdentityReservation
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
		identityByID: make(map[string]grantIdentityReservation),
	}, nil
}

func (authority *Authority) InitializeGrantIdentityIndex(ctx context.Context) error {
	if authority == nil || ctx == nil {
		return ErrInvalidGrantAuthorityInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	now, err := authority.operationTime()
	if err != nil {
		return err
	}
	events, err := authority.store.ReadAll(ctx)
	if err != nil {
		return err
	}
	state, err := authority.readState(ctx)
	if err != nil {
		return err
	}
	expected := make([]grantIdentityReservation, 0)
	for _, event := range events {
		if event.Type != "AgentGrantIssued" ||
			!strings.HasPrefix(event.StreamID, grantStreamPrefix) {
			continue
		}
		var payload issuePayload
		if decodeExactGrantPayload(event.PayloadJSON, &payload) != nil ||
			!payload.valid() {
			return ErrInvalidGrantAuthorityInput
		}
		expected = append(expected, grantIdentityReservation{
			grantID: payload.GrantID, tokenHash: payload.TokenHash,
			runID: payload.RunID, workItemID: payload.WorkItemID,
			runtimeInstanceID: payload.RuntimeInstanceID,
			issueEventID:      event.ID,
			grantStreamID:     event.StreamID, grantSequence: event.Seq,
		})
	}
	sort.Slice(expected, func(i, j int) bool {
		if expected[i].grantID != expected[j].grantID {
			return expected[i].grantID < expected[j].grantID
		}
		return expected[i].issueEventID < expected[j].issueEventID
	})
	seenIDs := make(map[string]grantIdentityReservation, len(expected))
	seenHashes := make(map[string]grantIdentityReservation, len(expected))
	issueIDs := make([]string, len(expected))
	for index, reservation := range expected {
		if existing, exists := seenIDs[reservation.grantID]; exists &&
			existing != reservation {
			return ErrGrantIDCollision
		}
		if existing, exists := seenHashes[reservation.tokenHash]; exists &&
			existing != reservation {
			return ErrGrantTokenCollision
		}
		seenIDs[reservation.grantID] = reservation
		seenHashes[reservation.tokenHash] = reservation
		issueIDs[index] = reservation.issueEventID
	}
	if state.identityInitialized {
		if !grantIdentityMatches(state.identityByID, seenIDs) ||
			!grantIdentityMatches(state.identityByHash, seenHashes) {
			return ErrGrantAuthorityConflict
		}
		authority.setGrantIdentityReady(state.identityByID)
		return nil
	}
	identityHead := state.heads[grantIdentityStreamID]
	for _, reservation := range expected {
		existing, exists := state.identityByID[reservation.grantID]
		if exists {
			if existing != reservation ||
				state.identityByHash[reservation.tokenHash] != reservation {
				return ErrGrantAuthorityConflict
			}
			continue
		}
		identityHead++
		reservationEvent := newGrantEvent(
			deterministicGrantEventID(
				"AgentGrantIdentityReserved",
				reservation.grantID,
				reservation.tokenHash,
				reservation.issueEventID,
			),
			grantIdentityStreamID,
			identityHead,
			"AgentGrantIdentityReserved",
			now,
			"00000000-0000-4000-8000-000000000002",
			reservation.issueEventID,
			grantIdentityPayload(reservation),
		)
		if _, err := authority.store.AppendBatchIfStreamHeads(
			ctx,
			[]journal.StreamHeadExpectation{
				{StreamID: grantIdentityStreamID, Sequence: identityHead - 1},
				{StreamID: reservation.grantStreamID, Sequence: state.heads[reservation.grantStreamID]},
			},
			[]journal.Event{reservationEvent},
		); err != nil {
			return mapGrantWriteError(err)
		}
	}
	digest := sha256.Sum256([]byte(strings.Join(issueIDs, "\n")))
	issueDigest := hex.EncodeToString(digest[:])
	marker := newGrantEvent(
		deterministicGrantEventID(
			"AgentGrantIdentityIndexInitialized",
			issueDigest,
		),
		grantIdentityStreamID,
		identityHead+1,
		"AgentGrantIdentityIndexInitialized",
		now,
		"00000000-0000-4000-8000-000000000002",
		"",
		struct {
			IssueDigest string `json:"issue_digest"`
			IssueCount  int    `json:"issue_count"`
		}{issueDigest, len(expected)},
	)
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{
			StreamID: grantIdentityStreamID,
			Sequence: identityHead,
		}},
		[]journal.Event{marker},
	); err != nil {
		return mapGrantWriteError(err)
	}
	authority.setGrantIdentityReady(seenIDs)
	return nil
}

func (authority *Authority) Issue(
	ctx context.Context,
	input IssueInput,
) (IssuedGrant, error) {
	operations, err := validateIssueInput(ctx, input)
	if err != nil {
		return IssuedGrant{}, err
	}
	if !authority.grantIdentityReady() {
		return IssuedGrant{}, ErrGrantIdentityIndexRequired
	}
	now, err := authority.operationTime()
	if err != nil {
		return IssuedGrant{}, err
	}
	command, err := authority.readCommandState(
		ctx,
		input.WorkItemID,
		input.RunID,
		input.RuntimeInstanceID,
	)
	if err != nil {
		return IssuedGrant{}, err
	}
	runHead := command.runHead
	run := command.run
	if !issueBindingMatches(input, run) ||
		run.phase != "claimed" ||
		!now.Before(run.prepareLeaseExpiresAt) {
		return IssuedGrant{}, ErrRunNotGrantable
	}
	state := command.state
	if !state.identityInitialized {
		return IssuedGrant{}, ErrGrantIdentityIndexRequired
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
	if _, exists := state.identityByID[grantID]; exists {
		return IssuedGrant{}, ErrGrantIDCollision
	}
	if _, exists := state.identityByHash[tokenHash]; exists {
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
		for _, previous := range state.byID {
			if previous.runID == input.RunID &&
				previous.streamSequence == streamSequence-1 {
				previousEventID = previous.lastEventID
				break
			}
		}
		if previousEventID == "" {
			previousEventID = runHead.ID
		}
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
	identitySequence := state.heads[grantIdentityStreamID] + 1
	events = append(events, newGrantEvent(
		deterministicGrantEventID(
			"AgentGrantIdentityReserved",
			grantID,
			tokenHash,
			issueID,
		),
		grantIdentityStreamID,
		identitySequence,
		"AgentGrantIdentityReserved",
		now,
		input.CorrelationID,
		issueID,
		grantIdentityPayload(grantIdentityReservation{
			grantID: grantID, tokenHash: tokenHash,
			runID: input.RunID, workItemID: input.WorkItemID,
			runtimeInstanceID: input.RuntimeInstanceID,
			issueEventID:      issueID,
			grantStreamID:     streamID, grantSequence: streamSequence,
		}),
	))
	if err := ctx.Err(); err != nil {
		return IssuedGrant{}, err
	}
	_, err = authority.store.AppendBatchIfStreamHeads(
		ctx,
		grantCommandExpectations(command.heads),
		events,
	)
	if err != nil {
		return IssuedGrant{}, mapGrantWriteError(err)
	}
	authority.cacheGrantIdentity(grantIdentityReservation{
		grantID: grantID, tokenHash: tokenHash,
		runID: input.RunID, workItemID: input.WorkItemID,
		runtimeInstanceID: input.RuntimeInstanceID,
		issueEventID:      issueID, grantStreamID: streamID,
		grantSequence: streamSequence,
	})
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
	if !authority.grantIdentityReady() {
		return GrantRecord{}, ErrGrantIdentityIndexRequired
	}
	now, err := authority.operationTime()
	if err != nil {
		return GrantRecord{}, err
	}
	command, err := authority.readCommandState(
		ctx,
		input.WorkItemID,
		input.RunID,
		input.RuntimeInstanceID,
	)
	if err != nil {
		return GrantRecord{}, err
	}
	runHead := command.runHead
	run := command.run
	state := command.state
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
		grantCommandExpectations(command.heads),
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
	if !authority.grantIdentityReady() {
		return GrantRecord{}, ErrGrantIdentityIndexRequired
	}
	now, err := authority.operationTime()
	if err != nil {
		return GrantRecord{}, err
	}
	reservation, exists := authority.grantIdentity(input.GrantID)
	if !exists {
		return GrantRecord{}, ErrGrantNotFound
	}
	command, err := authority.readCommandState(
		ctx,
		reservation.workItemID,
		reservation.runID,
		reservation.runtimeInstanceID,
	)
	if err != nil {
		return GrantRecord{}, err
	}
	state := command.state
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
	runHead := command.runHead
	run := command.run
	if run.id != record.runID ||
		run.workItemID != record.workItemID ||
		run.claimGeneration < record.claimGeneration {
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
		grantCommandExpectations(command.heads),
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
	byID                map[string]*GrantRecord
	byHash              map[string]*GrantRecord
	active              map[string]*GrantRecord
	requests            map[string]map[string]authorizationDecision
	heads               map[string]int64
	runReferences       map[string]runReference
	identityByID        map[string]grantIdentityReservation
	identityByHash      map[string]grantIdentityReservation
	identityInitialized bool
}

type grantIdentityReservation struct {
	grantID           string
	tokenHash         string
	runID             string
	workItemID        string
	runtimeInstanceID string
	issueEventID      string
	grantStreamID     string
	grantSequence     int64
}

func (authority *Authority) readState(ctx context.Context) (*grantState, error) {
	events, err := authority.store.ReadAll(ctx)
	if err != nil {
		return nil, err
	}
	return replayGrantState(events)
}

func (authority *Authority) readStateFor(
	ctx context.Context,
	streamIDs []string,
) (*grantState, error) {
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return nil, err
	}
	return replayGrantState(snapshot.Events())
}

type grantCommandState struct {
	state   *grantState
	runHead runReference
	run     currentGrantRun
	heads   map[string]journal.StreamHead
}

func (authority *Authority) readCommandState(
	ctx context.Context,
	workItemID string,
	runID string,
	runtimeInstanceID string,
) (grantCommandState, error) {
	streamIDs := grantCommandStreams(
		workItemID,
		runID,
		runtimeInstanceID,
	)
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return grantCommandState{}, err
	}
	events := snapshot.Events()
	state, err := replayGrantState(events)
	if err != nil {
		return grantCommandState{}, err
	}
	runHead, run, err := currentRunFromEvents(events, runID)
	if err != nil {
		return grantCommandState{}, err
	}
	heads := make(map[string]journal.StreamHead, len(streamIDs))
	for _, head := range snapshot.Heads() {
		heads[head.StreamID] = head
	}
	return grantCommandState{
		state: state, runHead: runHead, run: run, heads: heads,
	}, nil
}

func replayGrantState(events []journal.Event) (*grantState, error) {
	runReferences, err := indexGrantRunReferences(events)
	if err != nil {
		return nil, err
	}
	state := &grantState{
		byID:           make(map[string]*GrantRecord),
		byHash:         make(map[string]*GrantRecord),
		active:         make(map[string]*GrantRecord),
		requests:       make(map[string]map[string]authorizationDecision),
		heads:          make(map[string]int64),
		runReferences:  runReferences,
		identityByID:   make(map[string]grantIdentityReservation),
		identityByHash: make(map[string]grantIdentityReservation),
	}
	for _, event := range events {
		if event.StreamID == grantIdentityStreamID {
			if err := applyGrantIdentityEvent(state, event); err != nil {
				return nil, err
			}
			continue
		}
		if !strings.HasPrefix(event.StreamID, grantStreamPrefix) {
			continue
		}
		if err := applyGrantEvent(state, event); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func applyGrantIdentityEvent(state *grantState, event journal.Event) error {
	if event.SchemaVersion != 1 ||
		event.Seq != state.heads[event.StreamID]+1 ||
		event.EmittedAt.IsZero() ||
		event.EmittedAt.Location() != time.UTC ||
		!validCanonicalUUID(event.CorrelationID) {
		return ErrInvalidGrantAuthorityInput
	}
	switch event.Type {
	case "AgentGrantIdentityReserved":
		var payload struct {
			GrantID           string `json:"grant_id"`
			TokenHash         string `json:"token_hash"`
			RunID             string `json:"run_id"`
			WorkItemID        string `json:"work_item_id"`
			RuntimeInstanceID string `json:"runtime_instance_id"`
			IssueEventID      string `json:"issue_event_id"`
			GrantStreamID     string `json:"grant_stream_id"`
			GrantSequence     int64  `json:"grant_sequence"`
		}
		if decodeExactGrantPayload(event.PayloadJSON, &payload) != nil ||
			!validCanonicalUUID(payload.GrantID) ||
			!validTokenHash(payload.TokenHash) ||
			!validOpaqueID(payload.RunID) ||
			!validOpaqueID(payload.WorkItemID) ||
			!validOpaqueID(payload.RuntimeInstanceID) ||
			payload.IssueEventID == "" ||
			payload.GrantStreamID != grantStream(payload.RunID) ||
			payload.GrantSequence <= 0 ||
			event.CausationID != payload.IssueEventID {
			return ErrInvalidGrantAuthorityInput
		}
		reservation := grantIdentityReservation{
			grantID: payload.GrantID, tokenHash: payload.TokenHash,
			runID: payload.RunID, workItemID: payload.WorkItemID,
			runtimeInstanceID: payload.RuntimeInstanceID,
			issueEventID:      payload.IssueEventID,
			grantStreamID:     payload.GrantStreamID,
			grantSequence:     payload.GrantSequence,
		}
		if existing, exists := state.identityByID[payload.GrantID]; exists &&
			existing != reservation {
			return ErrGrantIDCollision
		}
		if existing, exists := state.identityByHash[payload.TokenHash]; exists &&
			existing != reservation {
			return ErrGrantTokenCollision
		}
		state.identityByID[payload.GrantID] = reservation
		state.identityByHash[payload.TokenHash] = reservation
	case "AgentGrantIdentityIndexInitialized":
		var payload struct {
			IssueDigest string `json:"issue_digest"`
			IssueCount  int    `json:"issue_count"`
		}
		if state.identityInitialized ||
			decodeExactGrantPayload(event.PayloadJSON, &payload) != nil ||
			!validTokenHash(payload.IssueDigest) ||
			payload.IssueCount < 0 {
			return ErrInvalidGrantAuthorityInput
		}
		state.identityInitialized = true
	default:
		return ErrInvalidGrantAuthorityInput
	}
	state.heads[event.StreamID] = event.Seq
	return nil
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

type currentGrantRun struct {
	id                    string
	workItemID            string
	phase                 string
	claimID               string
	claimGeneration       int64
	runtimeInstanceID     string
	agentInstanceID       string
	prepareLeaseExpiresAt time.Time
	terminalStatus        string
}

func currentRunFromEvents(
	events []journal.Event,
	runID string,
) (runReference, currentGrantRun, error) {
	streamID := runStream(runID)
	var current currentGrantRun
	var head runReference
	var sequence int64
	for _, event := range events {
		if event.StreamID != streamID {
			continue
		}
		if event.Seq != sequence+1 {
			return runReference{}, currentGrantRun{}, ErrInvalidGrantAuthorityInput
		}
		sequence = event.Seq
		head = runReference{
			StreamID: event.StreamID, Seq: event.Seq,
			ID: event.ID, EmittedAt: event.EmittedAt,
		}
		switch event.Type {
		case "RunClaimed":
			var payload struct {
				WorkItemID            string `json:"work_item_id"`
				RunID                 string `json:"run_id"`
				ClaimID               string `json:"claim_id"`
				ClaimGeneration       int64  `json:"claim_generation"`
				RuntimeInstanceID     string `json:"runtime_instance_id"`
				AgentInstanceID       string `json:"agent_instance_id"`
				PrepareLeaseExpiresAt string `json:"prepare_lease_expires_at"`
				RuntimeStatusStreamID string `json:"runtime_status_stream_id"`
				RuntimeStatusSequence int64  `json:"runtime_status_sequence"`
				RuntimeStatusEventID  string `json:"runtime_status_event_id"`
			}
			if decodeExactGrantPayload(event.PayloadJSON, &payload) != nil {
				return runReference{}, currentGrantRun{}, ErrInvalidGrantAuthorityInput
			}
			expiresAt, err := time.Parse(time.RFC3339Nano, payload.PrepareLeaseExpiresAt)
			if err != nil || expiresAt.Location() != time.UTC ||
				payload.RunID != runID ||
				payload.WorkItemID == "" || payload.ClaimID == "" ||
				payload.ClaimGeneration <= 0 ||
				payload.RuntimeInstanceID == "" ||
				payload.AgentInstanceID == "" {
				return runReference{}, currentGrantRun{}, ErrInvalidGrantAuthorityInput
			}
			current = currentGrantRun{
				id: runID, workItemID: payload.WorkItemID, phase: "claimed",
				claimID:               payload.ClaimID,
				claimGeneration:       payload.ClaimGeneration,
				runtimeInstanceID:     payload.RuntimeInstanceID,
				agentInstanceID:       payload.AgentInstanceID,
				prepareLeaseExpiresAt: expiresAt,
			}
		case "RunPrepareLeaseExtended":
			var payload struct {
				WorkItemID             string `json:"work_item_id"`
				RunID                  string `json:"run_id"`
				ClaimID                string `json:"claim_id"`
				ClaimGeneration        int64  `json:"claim_generation"`
				RuntimeInstanceID      string `json:"runtime_instance_id"`
				AgentInstanceID        string `json:"agent_instance_id"`
				PreviousLeaseExpiresAt string `json:"previous_lease_expires_at"`
				PrepareLeaseExpiresAt  string `json:"prepare_lease_expires_at"`
			}
			if decodeExactGrantPayload(event.PayloadJSON, &payload) != nil ||
				!grantRunPayloadMatches(current, payload.WorkItemID, payload.RunID,
					payload.ClaimID, payload.ClaimGeneration,
					payload.RuntimeInstanceID, payload.AgentInstanceID) {
				return runReference{}, currentGrantRun{}, ErrInvalidGrantAuthorityInput
			}
			expiresAt, err := time.Parse(time.RFC3339Nano, payload.PrepareLeaseExpiresAt)
			if err != nil || expiresAt.Location() != time.UTC ||
				!expiresAt.After(current.prepareLeaseExpiresAt) {
				return runReference{}, currentGrantRun{}, ErrInvalidGrantAuthorityInput
			}
			current.prepareLeaseExpiresAt = expiresAt
		case "RunStarted":
			var payload struct {
				WorkItemID            string `json:"work_item_id"`
				RunID                 string `json:"run_id"`
				ClaimID               string `json:"claim_id"`
				ClaimGeneration       int64  `json:"claim_generation"`
				RuntimeInstanceID     string `json:"runtime_instance_id"`
				AgentInstanceID       string `json:"agent_instance_id"`
				RuntimeStatusStreamID string `json:"runtime_status_stream_id"`
				RuntimeStatusSequence int64  `json:"runtime_status_sequence"`
				RuntimeStatusEventID  string `json:"runtime_status_event_id"`
			}
			if decodeExactGrantPayload(event.PayloadJSON, &payload) != nil ||
				!grantRunPayloadMatches(current, payload.WorkItemID, payload.RunID,
					payload.ClaimID, payload.ClaimGeneration,
					payload.RuntimeInstanceID, payload.AgentInstanceID) {
				return runReference{}, currentGrantRun{}, ErrInvalidGrantAuthorityInput
			}
			current.phase = "running"
		case "RunTerminalCommitted":
			var payload struct {
				WorkItemID            string `json:"work_item_id"`
				RunID                 string `json:"run_id"`
				ClaimID               string `json:"claim_id"`
				ClaimGeneration       int64  `json:"claim_generation"`
				RuntimeInstanceID     string `json:"runtime_instance_id"`
				AgentInstanceID       string `json:"agent_instance_id"`
				Status                string `json:"status"`
				Reason                string `json:"reason"`
				RuntimeStatusStreamID string `json:"runtime_status_stream_id"`
				RuntimeStatusSequence int64  `json:"runtime_status_sequence"`
				RuntimeStatusEventID  string `json:"runtime_status_event_id"`
			}
			if decodeExactGrantPayload(event.PayloadJSON, &payload) != nil ||
				!grantRunPayloadMatches(current, payload.WorkItemID, payload.RunID,
					payload.ClaimID, payload.ClaimGeneration,
					payload.RuntimeInstanceID, payload.AgentInstanceID) ||
				payload.Status == "" {
				return runReference{}, currentGrantRun{}, ErrInvalidGrantAuthorityInput
			}
			current.phase = "terminal"
			current.terminalStatus = payload.Status
		default:
			return runReference{}, currentGrantRun{}, ErrInvalidGrantAuthorityInput
		}
	}
	if current.id == "" || head.ID == "" {
		return runReference{}, currentGrantRun{}, ErrRunNotGrantable
	}
	return head, current, nil
}

func grantRunPayloadMatches(
	run currentGrantRun,
	workItemID string,
	runID string,
	claimID string,
	generation int64,
	runtimeInstanceID string,
	agentInstanceID string,
) bool {
	return run.id == runID && run.workItemID == workItemID &&
		run.claimID == claimID && run.claimGeneration == generation &&
		run.runtimeInstanceID == runtimeInstanceID &&
		run.agentInstanceID == agentInstanceID
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

func issueBindingMatches(input IssueInput, run currentGrantRun) bool {
	return run.id == input.RunID &&
		run.workItemID == input.WorkItemID &&
		run.claimID == input.ClaimID &&
		run.claimGeneration == input.ClaimGeneration &&
		run.runtimeInstanceID == input.RuntimeInstanceID &&
		run.agentInstanceID == input.AgentInstanceID
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
	run currentGrantRun,
	now time.Time,
) bool {
	if run.id != input.RunID ||
		run.workItemID != input.WorkItemID ||
		run.claimID != input.ClaimID ||
		run.claimGeneration != input.ClaimGeneration ||
		run.runtimeInstanceID != input.RuntimeInstanceID ||
		run.agentInstanceID != input.AgentInstanceID ||
		run.terminalStatus != "" {
		return false
	}
	switch run.phase {
	case "claimed":
		return now.Before(run.prepareLeaseExpiresAt)
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

func grantCommandStreams(
	workItemID string,
	runID string,
	runtimeInstanceID string,
) []string {
	return []string{
		"work-item/" + workItemID,
		runStream(runID),
		"runtime_instance:" + runtimeInstanceID,
		"runtime_capacity:" + runtimeInstanceID,
		grantStream(runID),
		grantIdentityStreamID,
	}
}

func grantCommandExpectations(
	heads map[string]journal.StreamHead,
) []journal.StreamHeadExpectation {
	streamIDs := make([]string, 0, len(heads))
	for streamID := range heads {
		streamIDs = append(streamIDs, streamID)
	}
	sort.Strings(streamIDs)
	expectations := make([]journal.StreamHeadExpectation, len(streamIDs))
	for index, streamID := range streamIDs {
		expectations[index] = journal.StreamHeadExpectation{
			StreamID: streamID,
			Sequence: heads[streamID].Sequence,
		}
	}
	return expectations
}

func (authority *Authority) grantIdentityReady() bool {
	authority.identityMu.RLock()
	defer authority.identityMu.RUnlock()
	return authority.identityReady
}

func (authority *Authority) setGrantIdentityReady(
	reservations map[string]grantIdentityReservation,
) {
	authority.identityMu.Lock()
	defer authority.identityMu.Unlock()
	authority.identityByID = make(map[string]grantIdentityReservation, len(reservations))
	for grantID, reservation := range reservations {
		authority.identityByID[grantID] = reservation
	}
	authority.identityReady = true
}

func (authority *Authority) cacheGrantIdentity(
	reservation grantIdentityReservation,
) {
	authority.identityMu.Lock()
	defer authority.identityMu.Unlock()
	authority.identityByID[reservation.grantID] = reservation
}

func (authority *Authority) grantIdentity(
	grantID string,
) (grantIdentityReservation, bool) {
	authority.identityMu.RLock()
	defer authority.identityMu.RUnlock()
	reservation, exists := authority.identityByID[grantID]
	return reservation, exists
}

func grantIdentityPayload(reservation grantIdentityReservation) struct {
	GrantID           string `json:"grant_id"`
	TokenHash         string `json:"token_hash"`
	RunID             string `json:"run_id"`
	WorkItemID        string `json:"work_item_id"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
	IssueEventID      string `json:"issue_event_id"`
	GrantStreamID     string `json:"grant_stream_id"`
	GrantSequence     int64  `json:"grant_sequence"`
} {
	return struct {
		GrantID           string `json:"grant_id"`
		TokenHash         string `json:"token_hash"`
		RunID             string `json:"run_id"`
		WorkItemID        string `json:"work_item_id"`
		RuntimeInstanceID string `json:"runtime_instance_id"`
		IssueEventID      string `json:"issue_event_id"`
		GrantStreamID     string `json:"grant_stream_id"`
		GrantSequence     int64  `json:"grant_sequence"`
	}{
		reservation.grantID,
		reservation.tokenHash,
		reservation.runID,
		reservation.workItemID,
		reservation.runtimeInstanceID,
		reservation.issueEventID,
		reservation.grantStreamID,
		reservation.grantSequence,
	}
}

func grantIdentityMatches(
	indexed map[string]grantIdentityReservation,
	expected map[string]grantIdentityReservation,
) bool {
	if len(indexed) != len(expected) {
		return false
	}
	for key, reservation := range expected {
		if indexed[key] != reservation {
			return false
		}
	}
	return true
}

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
