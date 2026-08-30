package harnessgateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type EventType string

const EventSchemaVersion = 4

const (
	defaultSessionOpenTimeout  = 30 * time.Second
	defaultSessionCloseTimeout = 30 * time.Second
)

const (
	EventSessionOpening    EventType = "session_opening"
	EventSessionReady      EventType = "session_ready"
	EventSessionClosing    EventType = "session_closing"
	EventSessionClosed     EventType = "session_closed"
	EventSessionFailed     EventType = "session_failed"
	EventResponseStarted   EventType = "response_started"
	EventResponseCompleted EventType = "response_completed"
	EventResponseCancelled EventType = "response_cancelled"
	EventResponseFailed    EventType = "response_failed"
)

type Event struct {
	SchemaVersion               int
	GatewayInstanceID           string
	ConfiguredHarnessVersion    int
	BackendVersion              int
	Sequence                    uint64
	OccurredAt                  time.Time
	Type                        EventType
	SessionID                   string
	HarnessID                   HarnessID
	BackendID                   BackendID
	ConversationID              string
	SegmentID                   string
	WorkspaceID                 string
	WorkspaceDigest             string
	ExecutionBindingDigest      string
	ProviderID                  string
	ProviderAccountID           string
	CredentialRevision          int64
	ModelID                     string
	ReasoningEffort             string
	SegmentContextCapsuleDigest string
	ContextCapsuleDigest        string
	GovernancePolicyDigest      string
	RouteTransitionReviewDigest string
	ContextAlignmentDigest      string
	ResponseID                  string
	IncidentID                  string
	Failure                     ResponseFailure
	// Forbidden content-bearing fields remain zero and exist only as a
	// regression tripwire for event sinks and compatibility projections.
	WorkspacePath string
	Content       string
	ProviderBody  string
}

type ResponseFailure struct {
	Code              string
	Stage             string
	HTTPStatus        int
	ProviderCode      string
	RetryAfterSeconds int64
	Retryable         bool
}

func (failure ResponseFailure) Valid() bool {
	return validIdentifier(failure.Code) && validIdentifier(failure.Stage) &&
		(failure.HTTPStatus == 0 || failure.HTTPStatus >= 100 && failure.HTTPStatus <= 599) &&
		(failure.ProviderCode == "" || validIdentifier(failure.ProviderCode)) &&
		failure.RetryAfterSeconds >= 0 && failure.RetryAfterSeconds <= 24*60*60
}

func (failure ResponseFailure) zero() bool {
	return failure == (ResponseFailure{})
}

type ResponseFailureClassifier func(error) (ResponseFailure, bool)

func (event Event) Valid() bool {
	if event.SchemaVersion != EventSchemaVersion || event.ConfiguredHarnessVersion < 1 ||
		!validIdentifier(event.GatewayInstanceID) || event.BackendVersion < 1 ||
		event.Sequence == 0 || event.OccurredAt.IsZero() ||
		!validIdentifier(event.SessionID) || !validIdentifier(string(event.HarnessID)) ||
		!validIdentifier(string(event.BackendID)) || !validIdentifier(event.ConversationID) ||
		!validIdentifier(event.SegmentID) || !validIdentifier(event.WorkspaceID) ||
		!validDigest(event.WorkspaceDigest) || !validDigest(event.ExecutionBindingDigest) ||
		!validIdentifier(event.ProviderID) ||
		!validProviderAuthority(event.ProviderAccountID, event.CredentialRevision) ||
		!validIdentifier(event.ModelID) ||
		(event.ReasoningEffort != "" && !validIdentifier(event.ReasoningEffort)) ||
		!validDigest(event.SegmentContextCapsuleDigest) ||
		!optionalDigest(event.GovernancePolicyDigest) ||
		!optionalDigest(event.RouteTransitionReviewDigest) ||
		!optionalDigest(event.ContextAlignmentDigest) ||
		(event.ProviderAccountID == "" && event.GovernancePolicyDigest != "") ||
		event.WorkspacePath != "" || event.Content != "" || event.ProviderBody != "" {
		return false
	}
	switch event.Type {
	case EventSessionOpening, EventSessionReady, EventSessionClosing, EventSessionClosed,
		EventSessionFailed:
		return event.ResponseID == "" && event.IncidentID == "" &&
			event.ContextCapsuleDigest == "" && event.Failure.zero()
	case EventResponseStarted, EventResponseCompleted, EventResponseCancelled:
		return validIdentifier(event.ResponseID) && validIdentifier(event.IncidentID) &&
			validDigest(event.ContextCapsuleDigest) && event.Failure.zero()
	case EventResponseFailed:
		return validIdentifier(event.ResponseID) && validIdentifier(event.IncidentID) &&
			validDigest(event.ContextCapsuleDigest) && event.Failure.Valid()
	default:
		return false
	}
}

type EventSink interface{ Record(Event) }

type Config struct {
	Registry            *BackendRegistry
	Events              EventSink
	Now                 func() time.Time
	SessionOpenTimeout  time.Duration
	SessionCloseTimeout time.Duration
	ClassifyFailure     ResponseFailureClassifier
}

type sessionState struct {
	binding    SegmentSessionBinding
	workspace  Workspace
	configured ConfiguredHarness
	session    BackendSession
	response   chan struct{}
	createdAt  time.Time
	lastUsedAt time.Time
	responding bool
	draining   bool
	failed     bool
	closeOnce  sync.Once
	closeDone  chan struct{}
	closeErr   error
}

type sessionOpening struct {
	binding   SegmentSessionBinding
	workspace Workspace
	ready     chan struct{}
	cancel    context.CancelFunc
	state     *sessionState
	err       error
}

type activeResponse struct {
	incidentID string
	cancel     context.CancelFunc
}

type configuredHarnessKey struct {
	id      HarnessID
	version int
}

var errSessionRetry = errors.New("retry Segment Session acquisition")

type Gateway struct {
	registry        *BackendRegistry
	events          EventSink
	now             func() time.Time
	instanceID      string
	sequence        atomic.Uint64
	eventMu         sync.Mutex
	openTimeout     time.Duration
	closeTimeout    time.Duration
	classifyFailure ResponseFailureClassifier
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc

	mu         sync.Mutex
	closed     bool
	sessions   map[string]*sessionState
	opening    map[string]*sessionOpening
	closing    map[*sessionState]struct{}
	active     map[string]activeResponse
	used       map[string]struct{}
	counts     map[configuredHarnessKey]int
	operations int
	drained    chan struct{}
	drainOnce  sync.Once
	closeDone  chan struct{}
	closeErr   error
}

func New(config Config) (*Gateway, error) {
	if config.Registry == nil || config.Events == nil || config.Now == nil ||
		config.SessionOpenTimeout < 0 || config.SessionCloseTimeout < 0 {
		return nil, ErrInvalidGateway
	}
	openTimeout := config.SessionOpenTimeout
	if openTimeout == 0 {
		openTimeout = defaultSessionOpenTimeout
	}
	closeTimeout := config.SessionCloseTimeout
	if closeTimeout == 0 {
		closeTimeout = defaultSessionCloseTimeout
	}
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	instanceID, err := newGatewayInstanceID()
	if err != nil {
		lifecycleCancel()
		return nil, ErrInvalidGateway
	}
	return &Gateway{
		registry: config.Registry, events: config.Events, now: config.Now,
		instanceID:      instanceID,
		openTimeout:     openTimeout,
		closeTimeout:    closeTimeout,
		classifyFailure: config.ClassifyFailure,
		lifecycleCtx:    lifecycleCtx, lifecycleCancel: lifecycleCancel,
		sessions: make(map[string]*sessionState), opening: make(map[string]*sessionOpening),
		closing: make(map[*sessionState]struct{}), active: make(map[string]activeResponse),
		used: make(map[string]struct{}), counts: make(map[configuredHarnessKey]int),
		drained: make(chan struct{}), closeDone: make(chan struct{}),
	}, nil
}

func newGatewayInstanceID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return "gateway-" + hex.EncodeToString(random[:]), nil
}

func (gateway *Gateway) Respond(
	ctx context.Context,
	binding SegmentSessionBinding,
	workspace Workspace,
	request ResponseRequest,
) (Response, error) {
	if gateway == nil || ctx == nil || ctx.Err() != nil || !binding.valid() ||
		!workspace.validFor(binding) || !request.Authority.validFor(binding) ||
		len(request.Input) == 0 || len(request.Input) > maxGatewayInputBytes {
		if binding.valid() && !request.Authority.validFor(binding) {
			return Response{}, ErrAuthorityConflict
		}
		return Response{}, ErrInvalidGateway
	}
	configured, backend, ok := gateway.registry.Resolve(binding.ConfiguredHarnessID)
	if !ok || configured.Version != binding.ConfiguredHarnessVersion ||
		configured.BackendID != binding.BackendID || configured.BackendVersion != binding.BackendVersion {
		return Response{}, ErrAuthorityConflict
	}
	if !gateway.beginOperation() {
		return Response{}, ErrInvalidGateway
	}
	defer gateway.endOperation()

	sessionID := binding.SessionID()
	responseKey := sessionID + ":" + request.Authority.ResponseID
	responseCtx, cancel := context.WithCancel(ctx)
	if err := gateway.admitResponse(responseKey, request.Authority.IncidentID, cancel); err != nil {
		cancel()
		return Response{}, err
	}
	var state *sessionState
	responseAcquired := false
	defer func() {
		cancel()
		closeDrained := gateway.deactivate(state, responseKey)
		if state == nil || !responseAcquired {
			return
		}
		if closeDrained {
			_ = gateway.closeSession(context.WithoutCancel(ctx), state)
		}
		state.releaseResponse()
	}()
	for {
		var err error
		state, err = gateway.session(responseCtx, sessionID, configured, backend, binding, workspace)
		if err != nil {
			return Response{}, err
		}
		if err := state.acquireResponse(responseCtx); err != nil {
			return Response{}, err
		}
		responseAcquired = true
		closeExpired, err := gateway.activate(state, responseKey)
		if err == nil {
			break
		}
		if closeExpired {
			_ = gateway.closeSession(context.WithoutCancel(responseCtx), state)
		}
		state.releaseResponse()
		responseAcquired = false
		state = nil
		if !errors.Is(err, errSessionRetry) {
			return Response{}, err
		}
	}
	gateway.record(EventResponseStarted, binding, sessionID, request.Authority)

	ownedInput := append([]byte(nil), request.Input...)
	request.Input = ownedInput
	response, respondErr := state.session.Respond(responseCtx, request)
	for index := range ownedInput {
		ownedInput[index] = 0
	}
	if respondErr != nil {
		if errors.Is(respondErr, context.Canceled) {
			gateway.record(EventResponseCancelled, binding, sessionID, request.Authority)
		} else {
			gateway.recordResponseFailure(
				binding, sessionID, request.Authority,
				gateway.responseFailure(respondErr),
			)
		}
		if errors.Is(respondErr, ErrSessionUnhealthy) {
			gateway.failSession(state)
		}
		return Response{}, respondErr
	}
	gateway.record(EventResponseCompleted, binding, sessionID, request.Authority)
	return response, nil
}

func (gateway *Gateway) session(
	ctx context.Context,
	sessionID string,
	configured ConfiguredHarness,
	backend Backend,
	binding SegmentSessionBinding,
	workspace Workspace,
) (*sessionState, error) {
	key := configuredHarnessKey{id: configured.HarnessID, version: configured.Version}
	for {
		gateway.mu.Lock()
		if gateway.closed {
			gateway.mu.Unlock()
			return nil, ErrInvalidGateway
		}
		now := gateway.now()
		if current := gateway.sessions[sessionID]; current != nil {
			if current.binding != binding || current.workspace != workspace {
				gateway.mu.Unlock()
				return nil, ErrAuthorityConflict
			}
			if !gateway.expiredLocked(current, now) && !current.draining {
				gateway.mu.Unlock()
				return current, nil
			}
			gateway.beginDrainLocked(sessionID, current)
			done := current.closeDone
			responding := current.responding
			gateway.mu.Unlock()
			if !responding {
				_ = gateway.closeSession(context.WithoutCancel(ctx), current)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-done:
				continue
			}
		}
		var priorClose <-chan struct{}
		for candidate := range gateway.closing {
			if candidate.binding.SessionID() == sessionID {
				priorClose = candidate.closeDone
				break
			}
		}
		if priorClose != nil {
			gateway.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-priorClose:
				continue
			}
		}
		if current := gateway.opening[sessionID]; current != nil {
			if current.binding != binding || current.workspace != workspace {
				gateway.mu.Unlock()
				return nil, ErrAuthorityConflict
			}
			gateway.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-current.ready:
				return current.state, current.err
			}
		}

		var idleExpired []*sessionState
		for candidateID, candidate := range gateway.sessions {
			if gateway.keyFor(candidate.configured) != key || candidate.draining ||
				!gateway.expiredLocked(candidate, now) {
				continue
			}
			gateway.beginDrainLocked(candidateID, candidate)
			if !candidate.responding {
				idleExpired = append(idleExpired, candidate)
			}
		}
		if len(idleExpired) > 0 {
			gateway.mu.Unlock()
			for _, expired := range idleExpired {
				_ = gateway.closeSession(context.WithoutCancel(ctx), expired)
			}
			continue
		}
		if gateway.counts[key] >= configured.MaxConcurrentSessions {
			var closingDone <-chan struct{}
			for candidate := range gateway.closing {
				if gateway.keyFor(candidate.configured) == key {
					closingDone = candidate.closeDone
					break
				}
			}
			gateway.mu.Unlock()
			if closingDone == nil {
				return nil, ErrInvalidGateway
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-closingDone:
				continue
			}
		}

		openCtx, cancel := context.WithTimeout(gateway.lifecycleCtx, gateway.openTimeout)
		opening := &sessionOpening{
			binding: binding, workspace: workspace, ready: make(chan struct{}), cancel: cancel,
		}
		gateway.opening[sessionID] = opening
		gateway.counts[key]++
		gateway.operations++
		gateway.mu.Unlock()
		go gateway.runSessionOpening(openCtx, sessionID, key, configured, backend, binding, workspace, opening)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-opening.ready:
			return opening.state, opening.err
		}
	}
}

func (gateway *Gateway) runSessionOpening(
	ctx context.Context,
	sessionID string,
	key configuredHarnessKey,
	configured ConfiguredHarness,
	backend Backend,
	binding SegmentSessionBinding,
	workspace Workspace,
	opening *sessionOpening,
) {
	defer gateway.endOperation()
	gateway.record(EventSessionOpening, binding, sessionID, ResponseAuthority{})
	opened, err := backend.OpenSession(ctx, configured, binding, workspace)
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	opening.cancel()
	if err == nil && opened == nil {
		err = ErrInvalidGateway
	}
	var state *sessionState
	if opened != nil {
		createdAt := gateway.now()
		state = &sessionState{
			binding: binding, workspace: workspace, configured: configured, session: opened,
			response: make(chan struct{}, 1), createdAt: createdAt, lastUsedAt: createdAt,
			closeDone: make(chan struct{}),
		}
		state.response <- struct{}{}
		state.failed = err != nil
	}

	gateway.mu.Lock()
	delete(gateway.opening, sessionID)
	closed := gateway.closed
	ready := err == nil && !closed
	if ready {
		gateway.sessions[sessionID] = state
		opening.state = state
	} else {
		if err == nil {
			err = ErrInvalidGateway
		}
		opening.err = err
		if state == nil {
			gateway.counts[key]--
		} else {
			state.draining = true
			gateway.closing[state] = struct{}{}
		}
	}
	gateway.mu.Unlock()

	if ready {
		gateway.record(EventSessionReady, binding, sessionID, ResponseAuthority{})
	} else if state == nil {
		gateway.record(EventSessionFailed, binding, sessionID, ResponseAuthority{})
	}
	close(opening.ready)
	if !ready && state != nil {
		_ = gateway.closeSession(context.WithoutCancel(ctx), state)
	}
}

func (state *sessionState) acquireResponse(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-state.response:
		if err := ctx.Err(); err != nil {
			state.releaseResponse()
			return err
		}
		return nil
	}
}

func (state *sessionState) releaseResponse() {
	state.response <- struct{}{}
}

func (gateway *Gateway) admitResponse(
	key string,
	incidentID string,
	cancel context.CancelFunc,
) error {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	if gateway.closed || cancel == nil {
		return ErrInvalidGateway
	}
	if _, duplicate := gateway.used[key]; duplicate {
		return ErrAuthorityConflict
	}
	gateway.used[key] = struct{}{}
	gateway.active[key] = activeResponse{incidentID: incidentID, cancel: cancel}
	return nil
}

func (gateway *Gateway) activate(state *sessionState, key string) (bool, error) {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	if gateway.closed {
		return false, ErrInvalidGateway
	}
	sessionID := state.binding.SessionID()
	if gateway.sessions[sessionID] != state || state.draining {
		return false, errSessionRetry
	}
	if gateway.expiredLocked(state, gateway.now()) {
		gateway.beginDrainLocked(sessionID, state)
		return true, errSessionRetry
	}
	if _, admitted := gateway.active[key]; !admitted {
		return false, ErrResponseNotFound
	}
	state.responding = true
	state.lastUsedAt = gateway.now()
	return false, nil
}

func (gateway *Gateway) deactivate(state *sessionState, key string) bool {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	delete(gateway.active, key)
	if state == nil {
		return false
	}
	state.responding = false
	state.lastUsedAt = gateway.now()
	if !state.draining {
		return false
	}
	gateway.beginDrainLocked(state.binding.SessionID(), state)
	return true
}

type CancelRequest struct {
	SessionID  string
	ResponseID string
	IncidentID string
}

func (gateway *Gateway) CancelResponse(ctx context.Context, request CancelRequest) error {
	if gateway == nil || ctx == nil || ctx.Err() != nil ||
		!validIdentifier(request.SessionID) || !validIdentifier(request.ResponseID) ||
		!validIdentifier(request.IncidentID) {
		return ErrInvalidGateway
	}
	gateway.mu.Lock()
	active, ok := gateway.active[request.SessionID+":"+request.ResponseID]
	gateway.mu.Unlock()
	if !ok || active.incidentID != request.IncidentID {
		return ErrResponseNotFound
	}
	active.cancel()
	return nil
}

func (gateway *Gateway) Close(ctx context.Context) error {
	if gateway == nil || ctx == nil {
		return ErrInvalidGateway
	}
	gateway.mu.Lock()
	startClosing := !gateway.closed
	if startClosing {
		gateway.closed = true
		gateway.lifecycleCancel()
		for _, opening := range gateway.opening {
			opening.cancel()
		}
		for _, active := range gateway.active {
			active.cancel()
		}
		if gateway.operations == 0 {
			gateway.drainOnce.Do(func() { close(gateway.drained) })
		}
	}
	gateway.mu.Unlock()
	if startClosing {
		go gateway.finishClose()
	}

	select {
	case <-gateway.closeDone:
		gateway.mu.Lock()
		result := gateway.closeErr
		gateway.mu.Unlock()
		return result
	default:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-gateway.closeDone:
		gateway.mu.Lock()
		result := gateway.closeErr
		gateway.mu.Unlock()
		return result
	}
}

func (gateway *Gateway) finishClose() {
	<-gateway.drained

	gateway.mu.Lock()
	sessions := make([]*sessionState, 0, len(gateway.sessions))
	for sessionID, state := range gateway.sessions {
		gateway.beginDrainLocked(sessionID, state)
		sessions = append(sessions, state)
	}
	gateway.mu.Unlock()
	var result error
	for _, state := range sessions {
		result = errors.Join(result, gateway.closeSession(context.Background(), state))
	}
	gateway.mu.Lock()
	gateway.closeErr = result
	close(gateway.closeDone)
	gateway.mu.Unlock()
}

func (gateway *Gateway) beginOperation() bool {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	if gateway.closed {
		return false
	}
	gateway.operations++
	return true
}

func (gateway *Gateway) endOperation() {
	gateway.mu.Lock()
	gateway.operations--
	if gateway.closed && gateway.operations == 0 {
		gateway.drainOnce.Do(func() { close(gateway.drained) })
	}
	gateway.mu.Unlock()
}

func (gateway *Gateway) keyFor(configured ConfiguredHarness) configuredHarnessKey {
	return configuredHarnessKey{id: configured.HarnessID, version: configured.Version}
}

func (gateway *Gateway) expiredLocked(state *sessionState, now time.Time) bool {
	age := now.Sub(state.createdAt)
	if age >= state.configured.MaxSessionAge {
		return true
	}
	idle := now.Sub(state.lastUsedAt)
	return !state.responding && idle >= state.configured.IdleTimeout
}

func (gateway *Gateway) beginDrainLocked(sessionID string, state *sessionState) {
	state.draining = true
	if gateway.sessions[sessionID] == state {
		delete(gateway.sessions, sessionID)
	}
	gateway.closing[state] = struct{}{}
}

func (gateway *Gateway) failSession(state *sessionState) {
	if state == nil {
		return
	}
	gateway.mu.Lock()
	state.failed = true
	state.draining = true
	gateway.mu.Unlock()
}

func (gateway *Gateway) closeSession(ctx context.Context, state *sessionState) error {
	state.closeOnce.Do(func() {
		sessionID := state.binding.SessionID()
		gateway.record(EventSessionClosing, state.binding, sessionID, ResponseAuthority{})
		closeCtx, cancelClose := context.WithTimeout(ctx, gateway.closeTimeout)
		state.closeErr = state.session.Close(closeCtx)
		cancelClose()
		gateway.mu.Lock()
		failed := state.failed
		gateway.mu.Unlock()
		if failed || state.closeErr != nil {
			gateway.record(EventSessionFailed, state.binding, sessionID, ResponseAuthority{})
		} else {
			gateway.record(EventSessionClosed, state.binding, sessionID, ResponseAuthority{})
		}
		gateway.mu.Lock()
		delete(gateway.closing, state)
		key := gateway.keyFor(state.configured)
		gateway.counts[key]--
		close(state.closeDone)
		gateway.mu.Unlock()
	})
	<-state.closeDone
	return state.closeErr
}

func (gateway *Gateway) record(
	eventType EventType,
	binding SegmentSessionBinding,
	sessionID string,
	authority ResponseAuthority,
) {
	gateway.recordEvent(eventType, binding, sessionID, authority, ResponseFailure{})
}

func (gateway *Gateway) recordResponseFailure(
	binding SegmentSessionBinding,
	sessionID string,
	authority ResponseAuthority,
	failure ResponseFailure,
) {
	gateway.recordEvent(EventResponseFailed, binding, sessionID, authority, failure)
}

func (gateway *Gateway) responseFailure(err error) ResponseFailure {
	fallback := ResponseFailure{
		Code: "provider_unavailable", Stage: "conversation_dispatch", Retryable: true,
	}
	if gateway == nil || gateway.classifyFailure == nil {
		return fallback
	}
	classified, ok := gateway.classifyFailure(err)
	if !ok || !classified.Valid() {
		return fallback
	}
	return classified
}

func (gateway *Gateway) recordEvent(
	eventType EventType,
	binding SegmentSessionBinding,
	sessionID string,
	authority ResponseAuthority,
	failure ResponseFailure,
) {
	gateway.eventMu.Lock()
	defer gateway.eventMu.Unlock()
	event := Event{
		SchemaVersion: EventSchemaVersion, GatewayInstanceID: gateway.instanceID,
		ConfiguredHarnessVersion: binding.ConfiguredHarnessVersion,
		BackendVersion:           binding.BackendVersion, Sequence: gateway.sequence.Add(1),
		OccurredAt: gateway.now(), Type: eventType,
		SessionID: sessionID, HarnessID: binding.ConfiguredHarnessID, BackendID: binding.BackendID,
		ConversationID: binding.ConversationID, SegmentID: binding.SegmentID,
		WorkspaceID: binding.WorkspaceID, WorkspaceDigest: binding.WorkspaceDigest,
		ExecutionBindingDigest: binding.ExecutionBindingDigest,
		ProviderID:             binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
		CredentialRevision: binding.CredentialRevision, ModelID: binding.ModelID,
		ReasoningEffort:             binding.ReasoningEffort,
		SegmentContextCapsuleDigest: binding.SegmentContextCapsuleDigest,
		ContextCapsuleDigest:        authority.ContextCapsuleDigest,
		GovernancePolicyDigest:      binding.GovernancePolicyDigest,
		RouteTransitionReviewDigest: binding.RouteTransitionReviewDigest,
		ContextAlignmentDigest:      binding.ContextAlignmentDigest,
		ResponseID:                  authority.ResponseID,
		IncidentID:                  authority.IncidentID,
		Failure:                     failure,
	}
	if event.Valid() {
		gateway.events.Record(event)
	}
}
