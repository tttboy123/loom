package sandbox

// Governed sandbox controller (Phase 3B). The backend is never a state
// authority: every lifecycle step is an Event Journal fact on the stream
// sandbox/<instance_id>, and Loom rebuilds and reconciles from the Journal.

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
	"time"

	"loom-pi-rebuild/internal/journal"
)

const (
	EventCreated   = "SandboxInstanceCreated"
	EventExecReq   = "SandboxExecRequested"
	EventExecDone  = "SandboxExecCompleted"
	EventExecFail  = "SandboxExecFailed"
	EventCancelled = "SandboxCancelled"
	EventPaused    = "SandboxPaused"
	EventResumed   = "SandboxResumed"
	EventDestroyed = "SandboxDestroyed"

	streamPrefix = "sandbox/"
	controllerID = "sb1"
)

var (
	ErrInvalidInput      = errors.New("invalid sandbox input")
	ErrUnknownInstance   = errors.New("unknown sandbox instance")
	ErrStaleGeneration   = errors.New("stale sandbox generation")
	ErrInstanceDestroyed = errors.New("sandbox instance already destroyed")
)

// InstanceState is the Journal-derived projection of one instance. It is a
// read model only; the Journal remains the single authority.
type InstanceState struct {
	InstanceID      string
	JobID           string
	RunID           string
	Generation      int64
	WorkspaceDigest string
	State           string
	ExecCount       int
	Destroyed       bool
}

// ReconcileResult describes one instance cleaned during Reconcile.
type ReconcileResult struct {
	InstanceID string
	Destroyed  bool
	Err        error
}

type createdPayload struct {
	InstanceID      string `json:"instance_id"`
	JobID           string `json:"job_id"`
	RunID           string `json:"run_id"`
	Generation      int64  `json:"generation"`
	WorkspaceDigest string `json:"workspace_digest"`
	CreatedAt       string `json:"created_at"`
}

type execRequestedPayload struct {
	InstanceID  string `json:"instance_id"`
	Command     string `json:"command"`
	TimeoutMS   int64  `json:"timeout_ms"`
	EnvDigest   string `json:"env_digest"`
	RequestedAt string `json:"requested_at"`
}

type execCompletedPayload struct {
	InstanceID   string `json:"instance_id"`
	ExitCode     int    `json:"exit_code"`
	OutputDigest string `json:"output_digest"`
	CompletedAt  string `json:"completed_at"`
}

type execFailedPayload struct {
	InstanceID string `json:"instance_id"`
	Reason     string `json:"reason"`
	ErrorCode  string `json:"error_code"`
	FailedAt   string `json:"failed_at"`
}

type instanceEventPayload struct {
	InstanceID string `json:"instance_id"`
	At         string `json:"at"`
	Reason     string `json:"reason,omitempty"`
}

// Controller owns the Journal lifecycle for one sandbox backend. One backend
// per implementation; the Controller never stores provider credentials or
// raw environment values.
type Controller struct {
	store   *journal.Store
	backend SandboxBackend
	now     func() time.Time
}

func NewController(store *journal.Store, backend SandboxBackend, now func() time.Time) *Controller {
	return &Controller{store: store, backend: backend, now: now}
}

// Create writes SandboxInstanceCreated, creates the backend instance, and on
// backend failure closes the stream with a Destroyed(create_failed) fact.
func (c *Controller) Create(
	ctx context.Context,
	request CreateRequest,
	operationID, correlationID string,
) (Instance, error) {
	if request.JobID == "" || request.RunID == "" || operationID == "" {
		return Instance{}, ErrInvalidInput
	}
	instanceID := deterministicInstanceID(request.JobID, request.RunID, request.Generation)
	streamID := streamPrefix + instanceID
	now := c.now().UTC()
	body, err := json.Marshal(createdPayload{
		InstanceID: instanceID, JobID: request.JobID, RunID: request.RunID,
		Generation: request.Generation, WorkspaceDigest: request.WorkspaceDigest,
		CreatedAt: now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return Instance{}, err
	}
	created := journal.Event{
		ID: eventID(EventCreated, streamID, operationID), StreamID: streamID,
		IdempotencyKey: controllerID + "/" + EventCreated + "/" + instanceID + "/" + operationID,
		Type:           EventCreated, SchemaVersion: 1, EmittedAt: now,
		CorrelationID: correlationID, PayloadJSON: body,
	}
	if _, err := c.append(ctx, streamID, []journal.Event{created}); err != nil {
		return Instance{}, err
	}
	instance, backendErr := c.backend.Create(ctx, request)
	if backendErr != nil {
		_ = c.recordDestroyed(ctx, instanceID, "create_failed", operationID, correlationID)
		return Instance{}, backendErr
	}
	if instance.InstanceID != "" && instance.InstanceID != instanceID {
		_ = c.recordDestroyed(ctx, instanceID, "instance_id_mismatch", operationID, correlationID)
		return Instance{}, ErrInvalidInput
	}
	return Instance{InstanceID: instanceID}, nil
}

// Exec fences generation, records ExecRequested, runs the backend, then
// records ExecCompleted or ExecFailed. Replay of the same operation never
// re-executes: the request fact is idempotent and the completed/failed fact
// is keyed by the same operation.
func (c *Controller) Exec(
	ctx context.Context,
	request ExecRequest,
	generation int64,
	operationID, correlationID string,
) (ExecResult, error) {
	if request.InstanceID == "" || request.Command == "" || operationID == "" {
		return ExecResult{}, ErrInvalidInput
	}
	streamID := streamPrefix + request.InstanceID
	if err := c.fence(ctx, request.InstanceID, generation); err != nil {
		return ExecResult{}, err
	}
	now := c.now().UTC()
	requestBody, err := json.Marshal(execRequestedPayload{
		InstanceID: request.InstanceID, Command: request.Command,
		TimeoutMS: request.Timeout.Milliseconds(), EnvDigest: request.EnvDigest,
		RequestedAt: now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return ExecResult{}, err
	}
	requested := journal.Event{
		ID: eventID(EventExecReq, streamID, operationID), StreamID: streamID,
		IdempotencyKey: controllerID + "/" + EventExecReq + "/" + request.InstanceID + "/" + operationID,
		Type:           EventExecReq, SchemaVersion: 1, EmittedAt: now,
		CorrelationID: correlationID, PayloadJSON: requestBody,
	}
	if _, err := c.append(ctx, streamID, []journal.Event{requested}); err != nil {
		return ExecResult{}, err
	}
	if terminal, ok, terminalErr := c.terminalForOp(ctx, request.InstanceID, operationID); terminalErr != nil {
		return ExecResult{}, terminalErr
	} else if ok {
		// Idempotent replay of the same operation: the terminal fact already
		// exists, so the backend must never run twice.
		if terminal.Type == EventExecDone {
			var payload execCompletedPayload
			if json.Unmarshal(terminal.PayloadJSON, &payload) == nil {
				return ExecResult{ExitCode: payload.ExitCode, OutputDigest: payload.OutputDigest}, nil
			}
			return ExecResult{}, ErrInvalidInput
		}
		var payload execFailedPayload
		_ = json.Unmarshal(terminal.PayloadJSON, &payload)
		return ExecResult{}, errors.New(payload.Reason)
	}
	result, execErr := c.backend.Exec(ctx, request)
	if execErr != nil {
		failBody, marshalErr := json.Marshal(execFailedPayload{
			InstanceID: request.InstanceID, Reason: execErr.Error(),
			ErrorCode: "exec_failed", FailedAt: c.now().UTC().Format(time.RFC3339Nano),
		})
		if marshalErr != nil {
			return ExecResult{}, marshalErr
		}
		failed := journal.Event{
			ID: eventID(EventExecFail, streamID, operationID), StreamID: streamID,
			IdempotencyKey: controllerID + "/" + EventExecFail + "/" + request.InstanceID + "/" + operationID,
			Type:           EventExecFail, SchemaVersion: 1,
			EmittedAt: c.now().UTC(), CorrelationID: correlationID, PayloadJSON: failBody,
		}
		if _, appendErr := c.append(ctx, streamID, []journal.Event{failed}); appendErr != nil {
			return ExecResult{}, appendErr
		}
		return ExecResult{}, execErr
	}
	doneBody, err := json.Marshal(execCompletedPayload{
		InstanceID: request.InstanceID, ExitCode: result.ExitCode,
		OutputDigest: result.OutputDigest, CompletedAt: c.now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return ExecResult{}, err
	}
	completed := journal.Event{
		ID: eventID(EventExecDone, streamID, operationID), StreamID: streamID,
		IdempotencyKey: controllerID + "/" + EventExecDone + "/" + request.InstanceID + "/" + operationID,
		Type:           EventExecDone, SchemaVersion: 1,
		EmittedAt: c.now().UTC(), CorrelationID: correlationID, PayloadJSON: doneBody,
	}
	if _, err := c.append(ctx, streamID, []journal.Event{completed}); err != nil {
		return ExecResult{}, err
	}
	return result, nil
}

func (c *Controller) Cancel(ctx context.Context, instanceID, operationID, correlationID string) error {
	return c.instanceOp(ctx, EventCancelled, instanceID, operationID, correlationID, c.backend.Cancel)
}

func (c *Controller) Pause(ctx context.Context, instanceID, operationID, correlationID string) error {
	return c.instanceOp(ctx, EventPaused, instanceID, operationID, correlationID, c.backend.Pause)
}

func (c *Controller) Resume(ctx context.Context, instanceID, operationID, correlationID string) error {
	return c.instanceOp(ctx, EventResumed, instanceID, operationID, correlationID, c.backend.Resume)
}

func (c *Controller) Destroy(ctx context.Context, instanceID, operationID, correlationID string) error {
	return c.instanceOp(ctx, EventDestroyed, instanceID, operationID, correlationID, c.backend.Destroy)
}

// Rebuild projects every sandbox instance from the Journal.
func (c *Controller) Rebuild(ctx context.Context) ([]InstanceState, error) {
	events, err := c.store.ReadAll(ctx)
	if err != nil {
		return nil, err
	}
	instances := make(map[string]*InstanceState)
	var order []string
	for _, event := range events {
		if !strings.HasPrefix(event.StreamID, streamPrefix) {
			continue
		}
		instanceID := strings.TrimPrefix(event.StreamID, streamPrefix)
		state, ok := instances[instanceID]
		if !ok {
			state = &InstanceState{InstanceID: instanceID}
			instances[instanceID] = state
			order = append(order, instanceID)
		}
		switch event.Type {
		case EventCreated:
			var payload createdPayload
			if json.Unmarshal(event.PayloadJSON, &payload) == nil {
				state.JobID = payload.JobID
				state.RunID = payload.RunID
				state.Generation = payload.Generation
				state.WorkspaceDigest = payload.WorkspaceDigest
			}
			state.State = EventCreated
		case EventExecReq:
			state.ExecCount++
			state.State = EventExecReq
		case EventExecDone:
			state.State = EventExecDone
		case EventExecFail:
			state.State = EventExecFail
		case EventCancelled, EventPaused, EventResumed:
			state.State = event.Type
		case EventDestroyed:
			state.State = EventDestroyed
			state.Destroyed = true
		}
	}
	result := make([]InstanceState, 0, len(order))
	for _, instanceID := range order {
		result = append(result, *instances[instanceID])
	}
	return result, nil
}

// Reconcile closes the loop after a restart/replay: every instance that is
// not Destroyed per the Journal is destroyed on the backend and marked
// Destroyed. It never re-executes and never touches instances already
// destroyed in the Journal.
func (c *Controller) Reconcile(ctx context.Context) ([]ReconcileResult, error) {
	instances, err := c.Rebuild(ctx)
	if err != nil {
		return nil, err
	}
	var results []ReconcileResult
	for _, instance := range instances {
		if instance.Destroyed {
			results = append(results, ReconcileResult{InstanceID: instance.InstanceID, Destroyed: true})
			continue
		}
		operationID := "reconcile-" + instance.InstanceID
		if backendErr := c.backend.Destroy(ctx, instance.InstanceID); backendErr != nil {
			results = append(results, ReconcileResult{
				InstanceID: instance.InstanceID, Err: backendErr,
			})
			continue
		}
		if recordErr := c.recordDestroyed(ctx, instance.InstanceID, "reconcile-cleanup", operationID, ""); recordErr != nil {
			results = append(results, ReconcileResult{
				InstanceID: instance.InstanceID, Err: recordErr,
			})
			continue
		}
		results = append(results, ReconcileResult{InstanceID: instance.InstanceID, Destroyed: true})
	}
	return results, nil
}

func (c *Controller) instanceOp(
	ctx context.Context,
	eventType, instanceID, operationID, correlationID string,
	backendOp func(context.Context, string) error,
) error {
	if instanceID == "" || operationID == "" {
		return ErrInvalidInput
	}
	streamID := streamPrefix + instanceID
	state, err := c.rebuildOne(ctx, instanceID)
	if err != nil {
		return err
	}
	if state.Destroyed {
		return ErrInstanceDestroyed
	}
	if err := backendOp(ctx, instanceID); err != nil {
		return err
	}
	now := c.now().UTC()
	body, err := json.Marshal(instanceEventPayload{
		InstanceID: instanceID, At: now.Format(time.RFC3339Nano),
		Reason: "instance_op",
	})
	if err != nil {
		return err
	}
	event := journal.Event{
		ID: eventID(eventType, streamID, operationID), StreamID: streamID,
		IdempotencyKey: controllerID + "/" + eventType + "/" + instanceID + "/" + operationID,
		Type:           eventType, SchemaVersion: 1, EmittedAt: now,
		CorrelationID: correlationID, PayloadJSON: body,
	}
	if _, err := c.append(ctx, streamID, []journal.Event{event}); err != nil {
		return err
	}
	return nil
}

func (c *Controller) terminalForOp(
	ctx context.Context,
	instanceID, operationID string,
) (journal.Event, bool, error) {
	streamID := streamPrefix + instanceID
	events, err := c.store.ReadStream(ctx, streamID)
	if err != nil {
		return journal.Event{}, false, err
	}
	doneKey := controllerID + "/" + EventExecDone + "/" + instanceID + "/" + operationID
	failKey := controllerID + "/" + EventExecFail + "/" + instanceID + "/" + operationID
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		if event.IdempotencyKey == doneKey || event.IdempotencyKey == failKey {
			return event, true, nil
		}
	}
	return journal.Event{}, false, nil
}

func (c *Controller) recordDestroyed(ctx context.Context, instanceID, reason, operationID, correlationID string) error {
	if operationID == "" {
		operationID = "destroy-" + instanceID
	}
	streamID := streamPrefix + instanceID
	now := c.now().UTC()
	body, err := json.Marshal(instanceEventPayload{
		InstanceID: instanceID, At: now.Format(time.RFC3339Nano), Reason: reason,
	})
	if err != nil {
		return err
	}
	event := journal.Event{
		ID: eventID(EventDestroyed, streamID, operationID), StreamID: streamID,
		IdempotencyKey: controllerID + "/" + EventDestroyed + "/" + instanceID + "/" + operationID,
		Type:           EventDestroyed, SchemaVersion: 1, EmittedAt: now,
		CorrelationID: correlationID, PayloadJSON: body,
	}
	_, err = c.append(ctx, streamID, []journal.Event{event})
	return err
}

func (c *Controller) fence(ctx context.Context, instanceID string, expectedGeneration int64) error {
	state, err := c.rebuildOne(ctx, instanceID)
	if err != nil {
		return err
	}
	if state.Destroyed {
		return ErrInstanceDestroyed
	}
	if state.Generation != expectedGeneration {
		return fmt.Errorf("%w: want %d have %d", ErrStaleGeneration, expectedGeneration, state.Generation)
	}
	return nil
}

func (c *Controller) rebuildOne(ctx context.Context, instanceID string) (InstanceState, error) {
	streamID := streamPrefix + instanceID
	events, err := c.store.ReadStream(ctx, streamID)
	if err != nil {
		return InstanceState{}, err
	}
	if len(events) == 0 {
		return InstanceState{}, fmt.Errorf("%w: %s", ErrUnknownInstance, instanceID)
	}
	state := InstanceState{InstanceID: instanceID}
	for _, event := range events {
		switch event.Type {
		case EventCreated:
			var payload createdPayload
			if json.Unmarshal(event.PayloadJSON, &payload) == nil {
				state.JobID = payload.JobID
				state.RunID = payload.RunID
				state.Generation = payload.Generation
				state.WorkspaceDigest = payload.WorkspaceDigest
			}
			state.State = EventCreated
		case EventExecReq:
			state.ExecCount++
			state.State = EventExecReq
		case EventExecDone:
			state.State = EventExecDone
		case EventExecFail:
			state.State = EventExecFail
		case EventCancelled, EventPaused, EventResumed:
			state.State = event.Type
		case EventDestroyed:
			state.State = EventDestroyed
			state.Destroyed = true
		}
	}
	return state, nil
}

func (c *Controller) append(ctx context.Context, streamID string, events []journal.Event) ([]journal.Event, error) {
	if len(events) == 0 {
		return nil, nil
	}
	stream, err := c.store.ReadStream(ctx, streamID)
	if err != nil {
		return nil, err
	}
	head := int64(0)
	if len(stream) > 0 {
		head = stream[len(stream)-1].Seq
	}
	existing := make(map[string]journal.Event, len(stream))
	for _, event := range stream {
		existing[event.IdempotencyKey] = event
	}
	committed := make([]journal.Event, 0, len(events))
	fresh := make([]journal.Event, 0, len(events))
	sequence := head
	for _, event := range events {
		if previous, ok := existing[event.IdempotencyKey]; ok {
			// Idempotent replay: reuse the committed event verbatim so the
			// immutable Seq/EmittedAt/content stay identical.
			if previous.Type != event.Type || !bytes.Equal(previous.PayloadJSON, event.PayloadJSON) {
				return nil, fmt.Errorf("%w: %s", journal.ErrIdempotencyConflict, event.IdempotencyKey)
			}
			committed = append(committed, previous)
			continue
		}
		sequence++
		event.Seq = sequence
		if event.EmittedAt.IsZero() {
			event.EmittedAt = c.now().UTC()
		}
		fresh = append(fresh, event)
		committed = append(committed, event)
	}
	if len(fresh) == 0 {
		return committed, nil
	}
	inserted, err := c.store.AppendBatchIfStreamHeads(ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: head}},
		fresh,
	)
	if err != nil {
		return nil, err
	}
	merged := make([]journal.Event, 0, len(committed))
	insertedIndex := 0
	for _, event := range committed {
		if _, ok := existing[event.IdempotencyKey]; ok {
			merged = append(merged, event)
			continue
		}
		merged = append(merged, inserted[insertedIndex])
		insertedIndex++
	}
	return merged, nil
}

func deterministicInstanceID(jobID, runID string, generation int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", jobID, runID, generation)))
	return "sb-" + hex.EncodeToString(sum[:8])
}

func eventID(eventType, streamID, operationID string) string {
	sum := sha256.Sum256([]byte(controllerID + "|" + eventType + "|" + streamID + "|" + operationID))
	return controllerID + "-" + eventType + "-" + hex.EncodeToString(sum[:8])
}

// EnvDigest returns a digest of the environment map with no raw values: the
// digest is a salted sha256 over sorted key=value pairs, and the raw
// environment never enters the Journal, evidence, or logs.
func EnvDigest(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(env[key])
		builder.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}
