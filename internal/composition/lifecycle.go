package composition

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

type Effect interface {
	Close(context.Context) error
}

type effectFunc struct {
	close func(context.Context) error
	once  sync.Once
	err   error
}

func NewEffect(close func(context.Context) error) Effect {
	if close == nil {
		return nil
	}
	return &effectFunc{close: close}
}

func (effect *effectFunc) Close(ctx context.Context) error {
	if effect == nil || ctx == nil {
		return ErrInvalidComposition
	}
	effect.once.Do(func() { effect.err = effect.close(ctx) })
	return effect.err
}

type Registration struct {
	descriptor BundleDescriptor
	root       *CapabilityContext
	provided   map[CapabilityRef]bool
	required   map[CapabilityRef]bool
}

func Provide[T any](
	registration *Registration,
	key CapabilityKey[T],
	value T,
) error {
	if registration == nil || !validCapabilityKey(key) || nilInterface(value) ||
		!containsCapability(registration.descriptor.Provides, key.ref) ||
		registration.provided[key.ref] ||
		capabilityProtected(key.ref) && !registration.descriptor.CoreProtected {
		return ErrInvalidComposition
	}
	if !reflect.TypeOf(value).AssignableTo(key.valueType) {
		return ErrInvalidComposition
	}
	if err := registration.root.bindRoot(
		key.ref, key.valueType, value, registration.descriptor.ID,
	); err != nil {
		return err
	}
	registration.provided[key.ref] = true
	return nil
}

func Require[T any](
	registration *Registration,
	key CapabilityKey[T],
) (T, error) {
	var zero T
	if registration == nil || !validCapabilityKey(key) ||
		!containsCapability(registration.descriptor.Requires, key.ref) &&
			!containsCapability(registration.descriptor.OptionalRequires, key.ref) ||
		capabilityProtected(key.ref) && !registration.descriptor.CoreProtected {
		return zero, ErrCapabilityUnavailable
	}
	value, err := Lookup(registration.root, key)
	if err != nil {
		return zero, err
	}
	registration.required[key.ref] = true
	return value, nil
}

func (registration *Registration) validate() error {
	if registration == nil {
		return ErrInvalidComposition
	}
	for _, provided := range registration.descriptor.Provides {
		if !registration.provided[provided] {
			return ErrInvalidComposition
		}
	}
	for _, required := range registration.descriptor.Requires {
		if !registration.required[required] {
			return ErrInvalidComposition
		}
	}
	return nil
}

type DiagnosticStage string

const (
	StageCompositionCompile  DiagnosticStage = "composition_compile"
	StageCompositionValidate DiagnosticStage = "composition_validate"
	StageBundleRegister      DiagnosticStage = "bundle_register"
	StageBundleStart         DiagnosticStage = "bundle_start"
	StageBundleReady         DiagnosticStage = "bundle_ready"
	StageBundleStop          DiagnosticStage = "bundle_stop"
	StageBundleDispose       DiagnosticStage = "bundle_dispose"
	StageRouteCompile        DiagnosticStage = "route_compile"
	StageScopeOpen           DiagnosticStage = "scope_open"
	StageScopeClose          DiagnosticStage = "scope_close"
)

type DiagnosticRecord struct {
	IncidentID     string          `json:"incident_id"`
	ProfileID      ProfileID       `json:"profile_id"`
	SnapshotDigest string          `json:"snapshot_digest"`
	BundleID       string          `json:"bundle_id,omitempty"`
	BundleVersion  string          `json:"bundle_version,omitempty"`
	ScopeKind      ScopeKind       `json:"scope_kind,omitempty"`
	ScopeID        string          `json:"scope_id,omitempty"`
	ScopeDigest    string          `json:"scope_digest,omitempty"`
	Stage          DiagnosticStage `json:"stage"`
	ElapsedMillis  int64           `json:"elapsed_millis"`
	Result         string          `json:"result"`
	ErrorCode      string          `json:"error_code,omitempty"`
	Retryable      bool            `json:"retryable"`
}

type lifecycleEffect struct {
	effect     Effect
	descriptor BundleDescriptor
}

type activeBundle struct {
	bundle       Bundle
	descriptor   BundleDescriptor
	capabilities *BundleContext
}

type DiagnosticRecorder interface {
	RecordCompositionDiagnostic(context.Context, DiagnosticRecord) error
}

type Activation struct {
	plan       *Plan
	root       *CapabilityContext
	effects    []lifecycleEffect
	started    []activeBundle
	ready      atomic.Bool
	admitting  atomic.Bool
	closeOnce  sync.Once
	closeErr   error
	incidentID string
	recorder   DiagnosticRecorder
}

func (plan *Plan) Activate(
	ctx context.Context,
	incidentID string,
	recorder DiagnosticRecorder,
) (*Activation, error) {
	if plan == nil || ctx == nil || !validIdentifier(incidentID) ||
		len(plan.bundles) != len(plan.snapshot.Bundles) {
		return nil, ErrInvalidComposition
	}
	root, err := newRootContext(
		ctx, plan.snapshot.Digest, plan.snapshot.Profile.ID, incidentID, recorder,
	)
	if err != nil {
		return nil, err
	}
	activation := &Activation{
		plan: plan, root: root, incidentID: incidentID, recorder: recorder,
	}
	activation.admitting.Store(false)
	registrations := make([]*Registration, 0, len(plan.bundles))
	for index, bundle := range plan.bundles {
		descriptor := plan.snapshot.Bundles[index]
		registration := &Registration{
			descriptor: descriptor, root: root,
			provided: make(map[CapabilityRef]bool), required: make(map[CapabilityRef]bool),
		}
		startedAt := time.Now()
		effect, registerErr := bundle.Register(ctx, registration)
		activation.record(ctx, descriptor, StageBundleRegister, startedAt, registerErr)
		if !nilInterface(effect) {
			activation.effects = append(activation.effects, lifecycleEffect{
				effect: effect, descriptor: descriptor,
			})
		}
		if registerErr != nil || nilInterface(effect) {
			activation.disposeEffects(ctx)
			_ = root.Close(ctx)
			return nil, compositionError("register", descriptor.ID, errors.Join(registerErr, ErrInvalidComposition))
		}
		registrations = append(registrations, registration)
	}
	for index, registration := range registrations {
		if err := registration.validate(); err != nil {
			activation.record(ctx, plan.snapshot.Bundles[index], StageCompositionValidate, time.Now(), err)
			activation.disposeEffects(ctx)
			_ = root.Close(ctx)
			return nil, compositionError("validate", plan.snapshot.Bundles[index].ID, err)
		}
	}
	for index, bundle := range plan.bundles {
		descriptor := plan.snapshot.Bundles[index]
		capabilities, contextErr := newBundleContext(root, descriptor)
		if contextErr != nil {
			activation.stopStarted(ctx)
			activation.disposeEffects(ctx)
			_ = root.Close(ctx)
			return nil, compositionError("start", descriptor.ID, contextErr)
		}
		startedAt := time.Now()
		effect, startErr := bundle.Start(ctx, capabilities)
		activation.record(ctx, descriptor, StageBundleStart, startedAt, startErr)
		if !nilInterface(effect) {
			activation.effects = append(activation.effects, lifecycleEffect{
				effect: effect, descriptor: descriptor,
			})
		}
		if startErr != nil || nilInterface(effect) {
			activation.stopStarted(ctx)
			activation.disposeEffects(ctx)
			_ = root.Close(ctx)
			return nil, compositionError("start", descriptor.ID, errors.Join(startErr, ErrInvalidComposition))
		}
		activation.started = append(activation.started, activeBundle{
			bundle: bundle, descriptor: descriptor, capabilities: capabilities,
		})
	}
	for _, active := range activation.started {
		startedAt := time.Now()
		readyErr := active.bundle.Ready(ctx, active.capabilities)
		activation.record(ctx, active.descriptor, StageBundleReady, startedAt, readyErr)
		if readyErr != nil {
			activation.stopStarted(ctx)
			activation.disposeEffects(ctx)
			_ = root.Close(ctx)
			return nil, compositionError("ready", active.descriptor.ID, readyErr)
		}
	}
	activation.ready.Store(true)
	activation.admitting.Store(true)
	return activation, nil
}

func (activation *Activation) Ready() bool {
	return activation != nil && activation.ready.Load() && activation.admitting.Load()
}

func (activation *Activation) Root() *CapabilityContext {
	if activation == nil || !activation.Ready() {
		return nil
	}
	return activation.root
}

func (activation *Activation) SnapshotDigest() string {
	if activation == nil || activation.plan == nil {
		return ""
	}
	return activation.plan.snapshot.Digest
}

func (activation *Activation) Close(ctx context.Context) error {
	if activation == nil || ctx == nil {
		return ErrInvalidComposition
	}
	activation.closeOnce.Do(func() {
		activation.admitting.Store(false)
		activation.ready.Store(false)
		activation.closeErr = errors.Join(
			activation.closeErr,
			activation.stopStarted(ctx),
			activation.root.Close(ctx),
			activation.disposeEffects(ctx),
		)
	})
	return activation.closeErr
}

func (activation *Activation) stopStarted(ctx context.Context) error {
	var stopErr error
	for _, active := range activation.started {
		startedAt := time.Now()
		err := active.bundle.Stop(ctx, active.capabilities)
		activation.record(ctx, active.descriptor, StageBundleStop, startedAt, err)
		stopErr = errors.Join(stopErr, err)
	}
	activation.started = nil
	return stopErr
}

func (activation *Activation) disposeEffects(ctx context.Context) error {
	var disposeErr error
	for index := len(activation.effects) - 1; index >= 0; index-- {
		tracked := activation.effects[index]
		startedAt := time.Now()
		err := tracked.effect.Close(ctx)
		activation.record(ctx, tracked.descriptor, StageBundleDispose, startedAt, err)
		disposeErr = errors.Join(disposeErr, err)
	}
	activation.effects = nil
	return disposeErr
}

func (activation *Activation) record(
	ctx context.Context,
	descriptor BundleDescriptor,
	stage DiagnosticStage,
	started time.Time,
	err error,
) {
	if activation == nil || nilInterface(activation.recorder) {
		return
	}
	result, code := "succeeded", ""
	if err != nil {
		result, code = "failed", "composition_failed"
	}
	_ = activation.recorder.RecordCompositionDiagnostic(ctx, DiagnosticRecord{
		IncidentID: activation.incidentID, ProfileID: activation.plan.snapshot.Profile.ID,
		SnapshotDigest: activation.plan.snapshot.Digest,
		BundleID:       descriptor.ID, BundleVersion: descriptor.Version,
		Stage: stage, ElapsedMillis: time.Since(started).Milliseconds(),
		Result: result, ErrorCode: code, Retryable: false,
	})
}

func containsCapability(values []CapabilityRef, wanted CapabilityRef) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
