package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"sync"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
)

type productCapabilityScopeSlot struct {
	mu      sync.RWMutex
	manager *productCapabilityScopeManager
	bound   bool
	closed  bool
}

func (slot *productCapabilityScopeSlot) Bind(
	manager *productCapabilityScopeManager,
) error {
	if slot == nil || manager == nil {
		return composition.ErrInvalidComposition
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.bound || slot.closed {
		return composition.ErrCompositionConflict
	}
	slot.manager = manager
	slot.bound = true
	return nil
}

func (slot *productCapabilityScopeSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && slot.manager != nil
}

func (slot *productCapabilityScopeSlot) OpenConversationAttempt(
	ctx context.Context,
	request api.LocalProductConversationScopeRequest,
) (api.LocalProductConversationScopeLease, error) {
	if slot == nil {
		return nil, composition.ErrCompositionNotReady
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if !slot.bound || slot.closed || slot.manager == nil {
		return nil, composition.ErrCompositionNotReady
	}
	return slot.manager.OpenConversationAttempt(ctx, request)
}

func (slot *productCapabilityScopeSlot) OpenTeamExecution(
	ctx context.Context,
	teamID string,
	executionID string,
) (productTeamExecutionScope, error) {
	if slot == nil {
		return nil, composition.ErrCompositionNotReady
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if !slot.bound || slot.closed || slot.manager == nil {
		return nil, composition.ErrCompositionNotReady
	}
	return slot.manager.OpenTeamExecution(ctx, teamID, executionID)
}

func (slot *productCapabilityScopeSlot) Close() error {
	if slot == nil {
		return nil
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.closed {
		return nil
	}
	slot.closed = true
	manager := slot.manager
	slot.manager = nil
	if manager == nil {
		return nil
	}
	return manager.Close(context.Background())
}

type productConversationScopeSet struct {
	conversation *composition.CapabilityContext
	team         *composition.CapabilityContext
	agent        *composition.CapabilityContext
	teamID       string
	agentID      string
}

type productCapabilityScopeManager struct {
	mu                sync.Mutex
	product           *composition.CapabilityContext
	snapshotDigest    string
	conversations     map[string]productConversationScopeSet
	conversationOrder []string
	attempts          map[string]bool
	teamExecutions    map[string]*productTeamExecutionScopeLease
	teamOrder         []string
	closed            bool
}

func newProductCapabilityScopeManager(
	product *composition.CapabilityContext,
	snapshotDigest string,
) (*productCapabilityScopeManager, error) {
	if product == nil || product.Kind() != composition.ScopeProduct ||
		product.CompositionSnapshotDigest() != snapshotDigest || len(snapshotDigest) != 64 {
		return nil, composition.ErrInvalidComposition
	}
	return &productCapabilityScopeManager{
		product: product, snapshotDigest: snapshotDigest,
		conversations:  make(map[string]productConversationScopeSet),
		attempts:       make(map[string]bool),
		teamExecutions: make(map[string]*productTeamExecutionScopeLease),
	}, nil
}

type productAgentAttemptScopeRequest struct {
	AgentID                string
	AttemptID              string
	TurnID                 string
	TurnGeneration         int64
	ExecutionBindingDigest string
}

type productTeamExecutionScope interface {
	OpenAgentAttempt(
		context.Context,
		productAgentAttemptScopeRequest,
	) (productAgentAttemptScopeLease, error)
	Close(context.Context) error
}

type productAgentAttemptScopeLease interface {
	api.LocalProductConversationScopeLease
	ExecutionContext() context.Context
	AdvanceTurn(context.Context, int64) error
}

type productAttemptTurnScopeController interface {
	AdvanceTurn(context.Context, int64) error
}

type productAttemptTurnScopeContextKey struct{}

func bindProductAttemptTurnScope(
	ctx context.Context,
	controller productAttemptTurnScopeController,
) (context.Context, error) {
	if ctx == nil || controller == nil {
		return nil, composition.ErrInvalidComposition
	}
	return context.WithValue(ctx, productAttemptTurnScopeContextKey{}, controller), nil
}

func productAttemptTurnScopeFromContext(
	ctx context.Context,
) (productAttemptTurnScopeController, bool) {
	if ctx == nil {
		return nil, false
	}
	controller, ok := ctx.Value(productAttemptTurnScopeContextKey{}).(productAttemptTurnScopeController)
	return controller, ok && controller != nil
}

func (manager *productCapabilityScopeManager) OpenTeamExecution(
	ctx context.Context,
	teamID string,
	executionID string,
) (productTeamExecutionScope, error) {
	if manager == nil || ctx == nil || teamID == "" || executionID == "" {
		return nil, composition.ErrInvalidComposition
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.closed || manager.product == nil {
		return nil, composition.ErrCompositionNotReady
	}
	key := teamID + "\x00" + executionID
	if manager.teamExecutions[key] != nil {
		return nil, composition.ErrCompositionConflict
	}
	conversation, err := manager.product.OpenChild(
		ctx, composition.ScopeConversation,
		composition.ScopeOptions{ID: productCapabilityScopeID("mission", key)},
	)
	if err != nil {
		return nil, err
	}
	team, err := conversation.OpenChild(
		ctx, composition.ScopeTeam,
		composition.ScopeOptions{ID: productCapabilityScopeID("team", key)},
	)
	if err != nil {
		_ = conversation.Close(ctx)
		return nil, err
	}
	lease := &productTeamExecutionScopeLease{
		manager: manager, key: key, conversation: conversation, team: team,
		snapshotDigest: manager.snapshotDigest,
		agents:         make(map[string]*composition.CapabilityContext),
		attempts:       make(map[string]bool),
	}
	manager.teamExecutions[key] = lease
	manager.teamOrder = append(manager.teamOrder, key)
	return lease, nil
}

func (manager *productCapabilityScopeManager) OpenConversationAttempt(
	ctx context.Context,
	request api.LocalProductConversationScopeRequest,
) (api.LocalProductConversationScopeLease, error) {
	if manager == nil || ctx == nil || request.ConversationID == "" ||
		request.TeamID == "" || request.AgentID == "" || request.AttemptID == "" ||
		request.TurnID == "" || request.TurnGeneration < 1 ||
		len(request.ExecutionBindingDigest) != 64 {
		return nil, composition.ErrInvalidComposition
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.closed || manager.product == nil {
		return nil, composition.ErrCompositionNotReady
	}
	attemptKey := request.ConversationID + "/" + request.AttemptID
	if manager.attempts[attemptKey] {
		return nil, composition.ErrCompositionConflict
	}
	scopes, found := manager.conversations[request.ConversationID]
	if found && (scopes.teamID != request.TeamID || scopes.agentID != request.AgentID) {
		return nil, composition.ErrCompositionConflict
	}
	if !found {
		conversation, err := manager.product.OpenChild(
			ctx, composition.ScopeConversation,
			composition.ScopeOptions{ID: productCapabilityScopeID(
				"conversation", request.ConversationID,
			)},
		)
		if err != nil {
			return nil, err
		}
		team, err := conversation.OpenChild(
			ctx, composition.ScopeTeam,
			composition.ScopeOptions{ID: productCapabilityScopeID("team", request.TeamID)},
		)
		if err != nil {
			_ = conversation.Close(ctx)
			return nil, err
		}
		agent, err := team.OpenChild(
			ctx, composition.ScopeAgent,
			composition.ScopeOptions{ID: productCapabilityScopeID("agent", request.AgentID)},
		)
		if err != nil {
			_ = conversation.Close(ctx)
			return nil, err
		}
		scopes = productConversationScopeSet{
			conversation: conversation, team: team, agent: agent,
			teamID: request.TeamID, agentID: request.AgentID,
		}
		manager.conversations[request.ConversationID] = scopes
		manager.conversationOrder = append(manager.conversationOrder, request.ConversationID)
	}
	attempt, err := scopes.agent.OpenChild(
		ctx, composition.ScopeAttempt,
		composition.ScopeOptions{
			ID: productCapabilityScopeID(
				"attempt", request.ConversationID+"\x00"+request.AttemptID,
			),
			CompositionSnapshotDigest: manager.snapshotDigest,
			ExecutionBindingDigest:    request.ExecutionBindingDigest,
		},
	)
	if err != nil {
		return nil, err
	}
	executionContext, cancelExecution := context.WithCancelCause(ctx)
	if err := attempt.Own(composition.NewEffect(func(context.Context) error {
		cancelExecution(composition.ErrScopeClosed)
		return nil
	})); err != nil {
		cancelExecution(err)
		return nil, errors.Join(err, attempt.Close(ctx))
	}
	turn, err := attempt.OpenChild(
		ctx, composition.ScopeTurn,
		composition.ScopeOptions{
			ID: productCapabilityScopeID(
				"turn", request.ConversationID+"\x00"+request.TurnID,
			),
			Generation: request.TurnGeneration,
		},
	)
	if err != nil {
		cancelExecution(err)
		return nil, errors.Join(err, attempt.Close(ctx))
	}
	manager.attempts[attemptKey] = true
	return &productConversationAttemptScopeLease{
		attempt: attempt, turn: turn, executionContext: executionContext,
	}, nil
}

func (manager *productCapabilityScopeManager) Close(ctx context.Context) error {
	if manager == nil || ctx == nil {
		return composition.ErrInvalidComposition
	}
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return nil
	}
	manager.closed = true
	conversations := make([]*composition.CapabilityContext, 0, len(manager.conversationOrder))
	for _, conversationID := range manager.conversationOrder {
		conversations = append(
			conversations, manager.conversations[conversationID].conversation,
		)
	}
	teams := make([]*productTeamExecutionScopeLease, 0, len(manager.teamOrder))
	for _, key := range manager.teamOrder {
		if lease := manager.teamExecutions[key]; lease != nil {
			teams = append(teams, lease)
		}
	}
	manager.conversations = nil
	manager.conversationOrder = nil
	manager.attempts = nil
	manager.teamExecutions = nil
	manager.teamOrder = nil
	manager.product = nil
	manager.mu.Unlock()
	var closeErr error
	for index := len(teams) - 1; index >= 0; index-- {
		closeErr = errors.Join(closeErr, teams[index].closeWithoutRelease(ctx))
	}
	for index := len(conversations) - 1; index >= 0; index-- {
		closeErr = errors.Join(closeErr, conversations[index].Close(ctx))
	}
	return closeErr
}

type productTeamExecutionScopeLease struct {
	mu             sync.Mutex
	manager        *productCapabilityScopeManager
	key            string
	conversation   *composition.CapabilityContext
	team           *composition.CapabilityContext
	snapshotDigest string
	agents         map[string]*composition.CapabilityContext
	agentOrder     []string
	attempts       map[string]bool
	closed         bool
	closeErr       error
}

func (lease *productTeamExecutionScopeLease) OpenAgentAttempt(
	ctx context.Context,
	request productAgentAttemptScopeRequest,
) (productAgentAttemptScopeLease, error) {
	if lease == nil || ctx == nil || request.AgentID == "" || request.AttemptID == "" ||
		request.TurnID == "" || request.TurnGeneration < 1 ||
		len(request.ExecutionBindingDigest) != 64 {
		return nil, composition.ErrInvalidComposition
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.closed || lease.team == nil {
		return nil, composition.ErrScopeClosed
	}
	attemptKey := request.AgentID + "\x00" + request.AttemptID
	if lease.attempts[attemptKey] {
		return nil, composition.ErrCompositionConflict
	}
	agent := lease.agents[request.AgentID]
	if agent == nil {
		var err error
		agent, err = lease.team.OpenChild(
			ctx, composition.ScopeAgent,
			composition.ScopeOptions{ID: productCapabilityScopeID("agent", lease.key+"\x00"+request.AgentID)},
		)
		if err != nil {
			return nil, err
		}
		lease.agents[request.AgentID] = agent
		lease.agentOrder = append(lease.agentOrder, request.AgentID)
	}
	attempt, err := agent.OpenChild(
		ctx, composition.ScopeAttempt,
		composition.ScopeOptions{
			ID:                        productCapabilityScopeID("attempt", lease.key+"\x00"+attemptKey),
			CompositionSnapshotDigest: lease.snapshotDigest,
			ExecutionBindingDigest:    request.ExecutionBindingDigest,
		},
	)
	if err != nil {
		return nil, err
	}
	executionContext, cancelExecution := context.WithCancelCause(ctx)
	if err := attempt.Own(composition.NewEffect(func(context.Context) error {
		cancelExecution(composition.ErrScopeClosed)
		return nil
	})); err != nil {
		cancelExecution(err)
		return nil, errors.Join(err, attempt.Close(ctx))
	}
	turn, err := attempt.OpenChild(
		ctx, composition.ScopeTurn,
		composition.ScopeOptions{
			ID:         productCapabilityScopeID("turn", lease.key+"\x00"+request.TurnID),
			Generation: request.TurnGeneration,
		},
	)
	if err != nil {
		cancelExecution(err)
		return nil, errors.Join(err, attempt.Close(ctx))
	}
	lease.attempts[attemptKey] = true
	return &productAgentAttemptScopeLeaseValue{
		attempt: attempt, turn: turn,
		executionContext: executionContext,
		turnIdentity:     lease.key + "\x00" + request.AgentID + "\x00" + request.AttemptID,
		turnGeneration:   request.TurnGeneration,
	}, nil
}

func (lease *productTeamExecutionScopeLease) Close(ctx context.Context) error {
	if lease == nil || ctx == nil {
		return composition.ErrInvalidComposition
	}
	err := lease.closeWithoutRelease(ctx)
	if lease.manager != nil {
		lease.manager.releaseTeamExecution(lease.key, lease)
	}
	return err
}

func (lease *productTeamExecutionScopeLease) closeWithoutRelease(ctx context.Context) error {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.closed {
		return lease.closeErr
	}
	lease.closed = true
	if lease.conversation != nil {
		lease.closeErr = lease.conversation.Close(ctx)
	}
	lease.conversation = nil
	lease.team = nil
	lease.agents = nil
	lease.agentOrder = nil
	lease.attempts = nil
	return lease.closeErr
}

func (manager *productCapabilityScopeManager) releaseTeamExecution(
	key string,
	lease *productTeamExecutionScopeLease,
) {
	if manager == nil {
		return
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.teamExecutions != nil && manager.teamExecutions[key] == lease {
		delete(manager.teamExecutions, key)
	}
}

type productMissionScopedExecutor struct {
	delegate productMissionExecutorPort
	team     productTeamExecutionScope
}

func (executor *productMissionScopedExecutor) Execute(
	ctx context.Context,
	input supervisor.ExecuteInput,
) (result supervisor.Outcome, resultErr error) {
	if executor == nil || executor.delegate == nil || executor.team == nil ||
		ctx == nil || input.Generation.AgentInstanceID == "" ||
		input.Generation.RunID == "" {
		return supervisor.Outcome{}, app.ErrInvalidMissionExecution
	}
	binding, err := loomruntime.FreezeExecutionBinding(input.Profile, input.Instance)
	if err != nil {
		return supervisor.Outcome{}, errors.Join(app.ErrInvalidMissionExecution, err)
	}
	lease, err := executor.team.OpenAgentAttempt(
		ctx,
		productAgentAttemptScopeRequest{
			AgentID: input.Generation.AgentInstanceID, AttemptID: input.Generation.RunID,
			TurnID: input.Generation.RunID + ":turn-1", TurnGeneration: 1,
			ExecutionBindingDigest: binding.BindingDigest,
		},
	)
	if err != nil {
		return supervisor.Outcome{}, errors.Join(app.ErrInvalidMissionExecution, err)
	}
	defer func() {
		closeContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resultErr = errors.Join(resultErr, lease.Close(closeContext))
	}()
	executionContext := lease.ExecutionContext()
	if executionContext == nil {
		return supervisor.Outcome{}, app.ErrInvalidMissionExecution
	}
	executionContext, err = bindProductAttemptTurnScope(executionContext, lease)
	if err != nil {
		return supervisor.Outcome{}, errors.Join(app.ErrInvalidMissionExecution, err)
	}
	return executor.delegate.Execute(executionContext, input)
}

func (executor *productMissionScopedExecutor) Close(ctx context.Context) error {
	if executor == nil || executor.delegate == nil || ctx == nil {
		return app.ErrInvalidMissionExecution
	}
	return executor.delegate.Close(ctx)
}

type productConversationAttemptScopeLease struct {
	attempt          *composition.CapabilityContext
	turn             *composition.CapabilityContext
	executionContext context.Context
	once             sync.Once
	err              error
}

type productAgentAttemptScopeLeaseValue struct {
	mu               sync.Mutex
	attempt          *composition.CapabilityContext
	turn             *composition.CapabilityContext
	executionContext context.Context
	turnIdentity     string
	turnGeneration   int64
	closed           bool
	closeOnce        sync.Once
	closeErr         error
}

func (lease *productAgentAttemptScopeLeaseValue) ExecutionContext() context.Context {
	if lease == nil {
		return nil
	}
	return lease.executionContext
}

func (lease *productAgentAttemptScopeLeaseValue) CompositionSnapshotDigest() string {
	if lease == nil || lease.attempt == nil {
		return ""
	}
	return lease.attempt.CompositionSnapshotDigest()
}

func (lease *productAgentAttemptScopeLeaseValue) ExecutionBindingDigest() string {
	if lease == nil || lease.attempt == nil {
		return ""
	}
	return lease.attempt.ExecutionBindingDigest()
}

func (lease *productAgentAttemptScopeLeaseValue) AdvanceTurn(
	ctx context.Context,
	completedSequence int64,
) error {
	if lease == nil || ctx == nil || completedSequence < 1 {
		return composition.ErrInvalidComposition
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.closed || lease.attempt == nil || lease.turn == nil {
		return composition.ErrScopeClosed
	}
	if completedSequence != lease.turnGeneration {
		return composition.ErrCompositionConflict
	}
	if err := lease.turn.Close(ctx); err != nil {
		return err
	}
	nextGeneration := completedSequence + 1
	nextTurn, err := lease.attempt.OpenChild(
		ctx, composition.ScopeTurn,
		composition.ScopeOptions{
			ID: productCapabilityScopeID(
				"turn", lease.turnIdentity+"\x00"+strconv.FormatInt(nextGeneration, 10),
			),
			Generation: nextGeneration,
		},
	)
	if err != nil {
		lease.turn = nil
		return err
	}
	lease.turn = nextTurn
	lease.turnGeneration = nextGeneration
	return nil
}

func (lease *productAgentAttemptScopeLeaseValue) Close(ctx context.Context) error {
	if lease == nil || ctx == nil {
		return composition.ErrInvalidComposition
	}
	lease.closeOnce.Do(func() {
		lease.mu.Lock()
		lease.closed = true
		attempt := lease.attempt
		lease.mu.Unlock()
		if attempt == nil {
			lease.closeErr = composition.ErrInvalidComposition
			return
		}
		lease.closeErr = attempt.Close(ctx)
	})
	return lease.closeErr
}

func (lease *productConversationAttemptScopeLease) CompositionSnapshotDigest() string {
	if lease == nil || lease.attempt == nil {
		return ""
	}
	return lease.attempt.CompositionSnapshotDigest()
}

func (lease *productConversationAttemptScopeLease) ExecutionBindingDigest() string {
	if lease == nil || lease.attempt == nil {
		return ""
	}
	return lease.attempt.ExecutionBindingDigest()
}

func (lease *productConversationAttemptScopeLease) ExecutionContext() context.Context {
	if lease == nil {
		return nil
	}
	return lease.executionContext
}

func (lease *productConversationAttemptScopeLease) Close(ctx context.Context) error {
	if lease == nil || lease.attempt == nil || ctx == nil {
		return composition.ErrInvalidComposition
	}
	lease.once.Do(func() { lease.err = lease.attempt.Close(ctx) })
	return lease.err
}

var _ api.LocalProductConversationScopeManager = (*productCapabilityScopeSlot)(nil)
var _ api.LocalProductConversationScopeLease = (*productConversationAttemptScopeLease)(nil)

func productCapabilityScopeID(domain, value string) string {
	digest := sha256.Sum256([]byte("loom/product-scope/" + domain + "/v1\x00" + value))
	return domain + "-" + hex.EncodeToString(digest[:])
}
