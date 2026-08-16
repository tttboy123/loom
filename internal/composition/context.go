package composition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"time"
)

type ScopeKind string

const (
	ScopeRoot         ScopeKind = "root"
	ScopeProduct      ScopeKind = "product"
	ScopeConversation ScopeKind = "conversation"
	ScopeTeam         ScopeKind = "team"
	ScopeAgent        ScopeKind = "agent"
	ScopeAttempt      ScopeKind = "attempt"
	ScopeTurn         ScopeKind = "turn"
)

var scopeOrder = map[ScopeKind]int{
	ScopeRoot: 0, ScopeProduct: 1, ScopeConversation: 2, ScopeTeam: 3,
	ScopeAgent: 4, ScopeAttempt: 5, ScopeTurn: 6,
}

type ScopeOptions struct {
	ID                        string
	Generation                int64
	CompositionSnapshotDigest string
	ExecutionBindingDigest    string
}

type capabilityEntry struct {
	valueType reflect.Type
	value     any
	owner     string
}

// BundleContext is a read-only view containing only capabilities declared by
// one frozen Bundle descriptor. It is not an operational scope or content bag.
type BundleContext struct {
	root           *CapabilityContext
	bundleID       string
	snapshotDigest string
	admitted       map[CapabilityRef]bool
}

func newBundleContext(
	root *CapabilityContext,
	descriptor BundleDescriptor,
) (*BundleContext, error) {
	if root == nil || root.kind != ScopeRoot || !validIdentifier(descriptor.ID) {
		return nil, ErrInvalidComposition
	}
	admitted := make(map[CapabilityRef]bool)
	for _, set := range [][]CapabilityRef{
		descriptor.Requires, descriptor.OptionalRequires, descriptor.Provides,
	} {
		for _, ref := range set {
			admitted[ref] = true
		}
	}
	return &BundleContext{
		root: root, bundleID: descriptor.ID,
		snapshotDigest: root.compositionSnapshotDigest, admitted: admitted,
	}, nil
}

func (capabilities *BundleContext) BundleID() string {
	if capabilities == nil {
		return ""
	}
	return capabilities.bundleID
}

func (capabilities *BundleContext) CompositionSnapshotDigest() string {
	if capabilities == nil {
		return ""
	}
	return capabilities.snapshotDigest
}

func LookupBundle[T any](
	capabilities *BundleContext,
	key CapabilityKey[T],
) (T, error) {
	var zero T
	if capabilities == nil || !validCapabilityKey(key) || !capabilities.admitted[key.ref] {
		return zero, ErrCapabilityUnavailable
	}
	return Lookup(capabilities.root, key)
}

type CapabilityContext struct {
	mu sync.RWMutex

	kind      ScopeKind
	id        string
	digest    string
	parent    *CapabilityContext
	values    map[CapabilityRef]capabilityEntry
	masked    map[CapabilityRef]bool
	effects   []Effect
	children  []*CapabilityContext
	closed    bool
	closeOnce sync.Once
	closeErr  error

	compositionSnapshotDigest string
	executionBindingDigest    string
	generation                int64
	profileID                 ProfileID
	incidentID                string
	recorder                  DiagnosticRecorder
}

func newRootContext(
	ctx context.Context,
	snapshotDigest string,
	profileID ProfileID,
	incidentID string,
	recorder DiagnosticRecorder,
) (*CapabilityContext, error) {
	if ctx == nil || !validDigest(snapshotDigest) || !validIdentifier(incidentID) {
		return nil, ErrInvalidComposition
	}
	started := time.Now()
	root := &CapabilityContext{
		kind: ScopeRoot, id: "composition-root", values: make(map[CapabilityRef]capabilityEntry),
		masked: make(map[CapabilityRef]bool), compositionSnapshotDigest: snapshotDigest,
		profileID: profileID, incidentID: incidentID, recorder: recorder,
	}
	root.digest = scopeDigest("", root.kind, ScopeOptions{
		ID: root.id, CompositionSnapshotDigest: snapshotDigest,
	})
	root.recordScope(ctx, StageScopeOpen, started, nil)
	return root, nil
}

func (scope *CapabilityContext) Kind() ScopeKind {
	if scope == nil {
		return ""
	}
	return scope.kind
}

func (scope *CapabilityContext) ID() string {
	if scope == nil {
		return ""
	}
	return scope.id
}

func (scope *CapabilityContext) Digest() string {
	if scope == nil {
		return ""
	}
	return scope.digest
}

func (scope *CapabilityContext) CompositionSnapshotDigest() string {
	if scope == nil {
		return ""
	}
	return scope.compositionSnapshotDigest
}

func (scope *CapabilityContext) ExecutionBindingDigest() string {
	if scope == nil {
		return ""
	}
	return scope.executionBindingDigest
}

func (scope *CapabilityContext) OpenChild(
	ctx context.Context,
	kind ScopeKind,
	options ScopeOptions,
) (*CapabilityContext, error) {
	if scope == nil || ctx == nil || !validIdentifier(options.ID) ||
		scopeOrder[kind] != scopeOrder[scope.kind]+1 {
		return nil, ErrInvalidComposition
	}
	started := time.Now()
	if kind == ScopeAttempt {
		if !validDigest(options.CompositionSnapshotDigest) ||
			!validDigest(options.ExecutionBindingDigest) ||
			options.CompositionSnapshotDigest != scope.compositionSnapshotDigest {
			return nil, ErrInvalidComposition
		}
	} else if options.CompositionSnapshotDigest != "" || options.ExecutionBindingDigest != "" {
		return nil, ErrInvalidComposition
	}
	if kind == ScopeTurn {
		if options.Generation < 1 {
			return nil, ErrInvalidComposition
		}
	} else if options.Generation != 0 {
		return nil, ErrInvalidComposition
	}
	scope.mu.Lock()
	if scope.closed {
		scope.mu.Unlock()
		return nil, ErrScopeClosed
	}
	compositionDigest := scope.compositionSnapshotDigest
	executionDigest := scope.executionBindingDigest
	if kind == ScopeAttempt {
		compositionDigest = options.CompositionSnapshotDigest
		executionDigest = options.ExecutionBindingDigest
	}
	child := &CapabilityContext{
		kind: kind, id: options.ID, parent: scope,
		values: make(map[CapabilityRef]capabilityEntry), masked: make(map[CapabilityRef]bool),
		compositionSnapshotDigest: compositionDigest,
		executionBindingDigest:    executionDigest, generation: options.Generation,
		profileID: scope.profileID, incidentID: scope.incidentID, recorder: scope.recorder,
	}
	if kind == ScopeProduct {
		for _, ref := range ProtectedCapabilities() {
			child.masked[ref] = true
		}
	}
	child.digest = scopeDigest(scope.digest, kind, ScopeOptions{
		ID: options.ID, Generation: options.Generation,
		CompositionSnapshotDigest: compositionDigest,
		ExecutionBindingDigest:    executionDigest,
	})
	scope.children = append(scope.children, child)
	scope.mu.Unlock()
	child.recordScope(ctx, StageScopeOpen, started, nil)
	return child, nil
}

func BindScoped[T any](
	scope *CapabilityContext,
	key CapabilityKey[T],
	value T,
) error {
	if scope == nil || scope.kind == ScopeRoot || !validCapabilityKey(key) ||
		capabilityProtected(key.ref) || nilInterface(value) {
		return ErrCapabilityProtected
	}
	entry, found, err := scope.parentLookup(key.ref)
	if err != nil || !found || entry.valueType != key.valueType ||
		!reflect.TypeOf(value).AssignableTo(key.valueType) {
		return ErrCapabilityUnavailable
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.closed {
		return ErrScopeClosed
	}
	if _, exists := scope.values[key.ref]; exists || scope.masked[key.ref] {
		return ErrCompositionConflict
	}
	scope.values[key.ref] = capabilityEntry{
		valueType: key.valueType, value: value, owner: entry.owner,
	}
	return nil
}

func Lookup[T any](scope *CapabilityContext, key CapabilityKey[T]) (T, error) {
	var zero T
	if scope == nil || !validCapabilityKey(key) {
		return zero, ErrCapabilityUnavailable
	}
	entry, found, err := scope.lookup(key.ref)
	if err != nil || !found || entry.valueType != key.valueType {
		return zero, errors.Join(ErrCapabilityUnavailable, err)
	}
	value, ok := entry.value.(T)
	if !ok {
		return zero, ErrCapabilityUnavailable
	}
	return value, nil
}

func (scope *CapabilityContext) Mask(ref CapabilityRef) error {
	if scope == nil || scope.kind == ScopeRoot || !validCapabilityRef(ref) || capabilityProtected(ref) {
		return ErrCapabilityProtected
	}
	if _, found, err := scope.parentLookup(ref); err != nil || !found {
		return ErrCapabilityUnavailable
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.closed {
		return ErrScopeClosed
	}
	if _, exists := scope.values[ref]; exists || scope.masked[ref] {
		return ErrCompositionConflict
	}
	scope.masked[ref] = true
	return nil
}

func (scope *CapabilityContext) Own(effect Effect) error {
	if scope == nil || nilInterface(effect) {
		return ErrInvalidComposition
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.closed {
		return ErrScopeClosed
	}
	scope.effects = append(scope.effects, effect)
	return nil
}

func (scope *CapabilityContext) lookup(ref CapabilityRef) (capabilityEntry, bool, error) {
	if scope == nil {
		return capabilityEntry{}, false, ErrCapabilityUnavailable
	}
	scope.mu.RLock()
	if scope.closed {
		scope.mu.RUnlock()
		return capabilityEntry{}, false, ErrScopeClosed
	}
	if scope.masked[ref] {
		scope.mu.RUnlock()
		return capabilityEntry{}, false, nil
	}
	entry, found := scope.values[ref]
	parent := scope.parent
	scope.mu.RUnlock()
	if found {
		return entry, true, nil
	}
	if parent == nil {
		return capabilityEntry{}, false, nil
	}
	return parent.lookup(ref)
}

func (scope *CapabilityContext) parentLookup(ref CapabilityRef) (capabilityEntry, bool, error) {
	if scope == nil || scope.parent == nil {
		return capabilityEntry{}, false, ErrCapabilityUnavailable
	}
	return scope.parent.lookup(ref)
}

func (scope *CapabilityContext) bindRoot(
	ref CapabilityRef,
	valueType reflect.Type,
	value any,
	owner string,
) error {
	if scope == nil || scope.kind != ScopeRoot || !validCapabilityRef(ref) ||
		valueType == nil || nilInterface(value) || !validIdentifier(owner) ||
		!reflect.TypeOf(value).AssignableTo(valueType) {
		return ErrInvalidComposition
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.closed {
		return ErrScopeClosed
	}
	if _, exists := scope.values[ref]; exists {
		return ErrCompositionConflict
	}
	scope.values[ref] = capabilityEntry{valueType: valueType, value: value, owner: owner}
	return nil
}

func (scope *CapabilityContext) Close(ctx context.Context) error {
	if scope == nil || ctx == nil {
		return ErrInvalidComposition
	}
	scope.closeOnce.Do(func() {
		started := time.Now()
		scope.mu.Lock()
		scope.closed = true
		children := append([]*CapabilityContext(nil), scope.children...)
		effects := append([]Effect(nil), scope.effects...)
		scope.values = nil
		scope.masked = nil
		scope.mu.Unlock()
		var closeErr error
		for index := len(children) - 1; index >= 0; index-- {
			closeErr = errors.Join(closeErr, children[index].Close(ctx))
		}
		for index := len(effects) - 1; index >= 0; index-- {
			closeErr = errors.Join(closeErr, effects[index].Close(ctx))
		}
		scope.closeErr = closeErr
		scope.recordScope(ctx, StageScopeClose, started, closeErr)
	})
	return scope.closeErr
}

func (scope *CapabilityContext) recordScope(
	ctx context.Context,
	stage DiagnosticStage,
	started time.Time,
	err error,
) {
	if scope == nil || nilInterface(scope.recorder) {
		return
	}
	result, code := "succeeded", ""
	if err != nil {
		result, code = "failed", "composition_failed"
	}
	_ = scope.recorder.RecordCompositionDiagnostic(ctx, DiagnosticRecord{
		IncidentID: scope.incidentID, ProfileID: scope.profileID,
		SnapshotDigest: scope.compositionSnapshotDigest,
		ScopeKind:      scope.kind, ScopeID: scope.id, ScopeDigest: scope.digest,
		Stage: stage, ElapsedMillis: time.Since(started).Milliseconds(),
		Result: result, ErrorCode: code, Retryable: false,
	})
}

func validCapabilityKey[T any](key CapabilityKey[T]) bool {
	return validCapabilityRef(key.ref) && key.valueType != nil &&
		key.valueType.Kind() == reflect.Interface && key.valueType.NumMethod() > 0
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func scopeDigest(parent string, kind ScopeKind, options ScopeOptions) string {
	canonical, _ := json.Marshal(struct {
		Domain      string    `json:"domain"`
		Parent      string    `json:"parent,omitempty"`
		Kind        ScopeKind `json:"kind"`
		ID          string    `json:"id"`
		Generation  int64     `json:"generation,omitempty"`
		Composition string    `json:"composition,omitempty"`
		Execution   string    `json:"execution,omitempty"`
	}{
		"loom/composition-scope/v1", parent, kind, options.ID, options.Generation,
		options.CompositionSnapshotDigest, options.ExecutionBindingDigest,
	})
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}
