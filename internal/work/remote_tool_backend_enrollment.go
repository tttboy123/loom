package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
)

var (
	ErrInvalidRemoteToolBackendEnrollment  = errors.New("invalid remote tool backend enrollment")
	ErrRemoteToolBackendEnrollmentConflict = errors.New("remote tool backend enrollment conflict")
	ErrRemoteToolBackendEnrollmentNotFound = errors.New("remote tool backend enrollment not found")
	ErrRemoteToolBackendPolicyDrift        = errors.New("remote tool backend Provider Account policy drift")
)

const (
	RemoteToolBackendWebSearch = "web_search"
	RemoteToolBackendMCPServer = "mcp_server"

	RemoteToolBackendEnrollmentActive  = "active"
	RemoteToolBackendEnrollmentRevoked = "revoked"

	remoteToolBackendEnrollmentVersion = 1
	maximumRemoteToolCalls             = 16
	maximumRemoteToolResultBytes       = 1 << 20
)

type RemoteToolBackendEnrollmentCommand struct {
	CommandID                     string
	EnrollmentID                  string
	BackendKind                   string
	AdapterID                     string
	ProviderID                    string
	ProviderAccountID             string
	ProviderAccountPolicyVersion  int
	ProviderAccountPolicyRevision int64
	ProviderAccountPolicyDigest   string
	EndpointFingerprint           string
	MCPServerID                   string
	AllowedTools                  []string
	ExpectedRevision              int64
	MaximumConcurrentCalls        int
	MaximumCallsPerAttempt        int
	Timeout                       time.Duration
	MaximumResultBytes            int
	MaximumBudgetUnits            int64
	CorrelationID                 string
}

type RemoteToolBackendEnrollmentRevokeCommand struct {
	CommandID         string
	EnrollmentID      string
	ProviderID        string
	ProviderAccountID string
	ExpectedRevision  int64
	CorrelationID     string
}

type RemoteToolBackendEnrollmentInput struct {
	Version                       int
	EnrollmentID                  string
	BackendKind                   string
	AdapterID                     string
	ProviderID                    string
	ProviderAccountID             string
	ProviderAccountPolicyVersion  int
	ProviderAccountPolicyRevision int64
	ProviderAccountPolicyDigest   string
	EndpointFingerprint           string
	MCPServerID                   string
	AllowedTools                  []string
	Revision                      int64
	Status                        string
	MaximumConcurrentCalls        int
	MaximumCallsPerAttempt        int
	Timeout                       time.Duration
	MaximumResultBytes            int
	MaximumBudgetUnits            int64
	ConfiguredAt                  time.Time
}

type RemoteToolBackendEnrollment struct {
	version                       int
	enrollmentID                  string
	backendKind                   string
	adapterID                     string
	providerID                    string
	providerAccountID             string
	providerAccountPolicyVersion  int
	providerAccountPolicyRevision int64
	providerAccountPolicyDigest   string
	endpointFingerprint           string
	mcpServerID                   string
	allowedTools                  []string
	revision                      int64
	status                        string
	maximumConcurrentCalls        int
	maximumCallsPerAttempt        int
	timeout                       time.Duration
	maximumResultBytes            int
	maximumBudgetUnits            int64
	configuredAt                  time.Time
	digest                        string
}

type remoteToolBackendEnrollmentState struct {
	enrollment RemoteToolBackendEnrollment
	commandID  string
	eventID    string
}

type remoteToolBackendEnrollmentPayload struct {
	CommandID                     string   `json:"command_id"`
	EnrollmentVersion             int      `json:"enrollment_version"`
	EnrollmentID                  string   `json:"enrollment_id"`
	BackendKind                   string   `json:"backend_kind"`
	AdapterID                     string   `json:"adapter_id"`
	ProviderID                    string   `json:"provider_id"`
	ProviderAccountID             string   `json:"provider_account_id"`
	ProviderAccountPolicyVersion  int      `json:"provider_account_policy_version"`
	ProviderAccountPolicyRevision int64    `json:"provider_account_policy_revision"`
	ProviderAccountPolicyDigest   string   `json:"provider_account_policy_digest"`
	EndpointFingerprint           string   `json:"endpoint_fingerprint"`
	MCPServerID                   string   `json:"mcp_server_id,omitempty"`
	AllowedTools                  []string `json:"allowed_tools"`
	Revision                      int64    `json:"revision"`
	Status                        string   `json:"status"`
	MaximumConcurrentCalls        int      `json:"maximum_concurrent_calls"`
	MaximumCallsPerAttempt        int      `json:"maximum_calls_per_attempt"`
	TimeoutNanoseconds            int64    `json:"timeout_nanoseconds"`
	MaximumResultBytes            int      `json:"maximum_result_bytes"`
	MaximumBudgetUnits            int64    `json:"maximum_budget_units"`
	ConfiguredAt                  string   `json:"configured_at"`
	EnrollmentDigest              string   `json:"enrollment_digest"`
}

func (enrollment RemoteToolBackendEnrollment) Version() int { return enrollment.version }
func (enrollment RemoteToolBackendEnrollment) EnrollmentID() string {
	return enrollment.enrollmentID
}
func (enrollment RemoteToolBackendEnrollment) BackendKind() string {
	return enrollment.backendKind
}
func (enrollment RemoteToolBackendEnrollment) AdapterID() string  { return enrollment.adapterID }
func (enrollment RemoteToolBackendEnrollment) ProviderID() string { return enrollment.providerID }
func (enrollment RemoteToolBackendEnrollment) ProviderAccountID() string {
	return enrollment.providerAccountID
}
func (enrollment RemoteToolBackendEnrollment) ProviderAccountPolicyVersion() int {
	return enrollment.providerAccountPolicyVersion
}
func (enrollment RemoteToolBackendEnrollment) ProviderAccountPolicyRevision() int64 {
	return enrollment.providerAccountPolicyRevision
}
func (enrollment RemoteToolBackendEnrollment) ProviderAccountPolicyDigest() string {
	return enrollment.providerAccountPolicyDigest
}
func (enrollment RemoteToolBackendEnrollment) EndpointFingerprint() string {
	return enrollment.endpointFingerprint
}
func (enrollment RemoteToolBackendEnrollment) MCPServerID() string { return enrollment.mcpServerID }
func (enrollment RemoteToolBackendEnrollment) AllowedTools() []string {
	return append([]string(nil), enrollment.allowedTools...)
}
func (enrollment RemoteToolBackendEnrollment) Revision() int64 { return enrollment.revision }
func (enrollment RemoteToolBackendEnrollment) Status() string  { return enrollment.status }
func (enrollment RemoteToolBackendEnrollment) MaximumConcurrentCalls() int {
	return enrollment.maximumConcurrentCalls
}
func (enrollment RemoteToolBackendEnrollment) MaximumCallsPerAttempt() int {
	return enrollment.maximumCallsPerAttempt
}
func (enrollment RemoteToolBackendEnrollment) Timeout() time.Duration { return enrollment.timeout }
func (enrollment RemoteToolBackendEnrollment) MaximumResultBytes() int {
	return enrollment.maximumResultBytes
}
func (enrollment RemoteToolBackendEnrollment) MaximumBudgetUnits() int64 {
	return enrollment.maximumBudgetUnits
}
func (enrollment RemoteToolBackendEnrollment) ConfiguredAt() time.Time {
	return enrollment.configuredAt
}
func (enrollment RemoteToolBackendEnrollment) Digest() string { return enrollment.digest }

func NewRemoteToolBackendEnrollment(
	input RemoteToolBackendEnrollmentInput,
) (RemoteToolBackendEnrollment, error) {
	enrollment := RemoteToolBackendEnrollment{
		version: input.Version, enrollmentID: input.EnrollmentID,
		backendKind: input.BackendKind, adapterID: input.AdapterID,
		providerID: input.ProviderID, providerAccountID: input.ProviderAccountID,
		providerAccountPolicyVersion:  input.ProviderAccountPolicyVersion,
		providerAccountPolicyRevision: input.ProviderAccountPolicyRevision,
		providerAccountPolicyDigest:   input.ProviderAccountPolicyDigest,
		endpointFingerprint:           input.EndpointFingerprint, mcpServerID: input.MCPServerID,
		allowedTools: append([]string(nil), input.AllowedTools...),
		revision:     input.Revision, status: input.Status,
		maximumConcurrentCalls: input.MaximumConcurrentCalls,
		maximumCallsPerAttempt: input.MaximumCallsPerAttempt,
		timeout:                input.Timeout, maximumResultBytes: input.MaximumResultBytes,
		maximumBudgetUnits: input.MaximumBudgetUnits, configuredAt: input.ConfiguredAt,
	}
	if !validRemoteToolBackendEnrollmentShape(enrollment) {
		return RemoteToolBackendEnrollment{}, ErrInvalidRemoteToolBackendEnrollment
	}
	enrollment.digest = remoteToolBackendEnrollmentDigest(enrollment)
	return enrollment, nil
}

func (enrollment RemoteToolBackendEnrollment) Valid() bool {
	return validRemoteToolBackendEnrollmentShape(enrollment) &&
		enrollment.digest == remoteToolBackendEnrollmentDigest(enrollment)
}

func (authority *Authority) ConfigureRemoteToolBackendEnrollment(
	ctx context.Context,
	command RemoteToolBackendEnrollmentCommand,
) (RemoteToolBackendEnrollment, error) {
	if authority == nil || ctx == nil || !validRemoteToolBackendEnrollmentCommand(command) {
		return RemoteToolBackendEnrollment{}, ErrInvalidRemoteToolBackendEnrollment
	}
	if err := ctx.Err(); err != nil {
		return RemoteToolBackendEnrollment{}, err
	}
	policy, err := authority.ProviderAccountPolicy(
		ctx, command.ProviderID, command.ProviderAccountID,
	)
	if err != nil || !remoteToolBackendEnrollmentPolicyMatches(command, policy) {
		return RemoteToolBackendEnrollment{}, ErrRemoteToolBackendPolicyDrift
	}
	if (policy.MaximumAssignedBudgetUnits() > 0 &&
		command.MaximumBudgetUnits > policy.MaximumAssignedBudgetUnits()) ||
		command.MaximumConcurrentCalls > policy.MaximumConcurrentAttempts() {
		return RemoteToolBackendEnrollment{}, ErrInvalidRemoteToolBackendEnrollment
	}
	return authority.configureRemoteToolBackendEnrollment(ctx, command)
}

func (authority *Authority) configureRemoteToolBackendEnrollment(
	ctx context.Context,
	command RemoteToolBackendEnrollmentCommand,
) (RemoteToolBackendEnrollment, error) {
	now := authority.now()
	if now.IsZero() || now.Location() != time.UTC {
		return RemoteToolBackendEnrollment{}, ErrInvalidRemoteToolBackendEnrollment
	}
	streamID := remoteToolBackendEnrollmentStream(command.EnrollmentID)
	events, err := authority.store.ReadStream(ctx, streamID)
	if err != nil {
		return RemoteToolBackendEnrollment{}, err
	}
	current, found, err := replayRemoteToolBackendEnrollment(command.EnrollmentID, events)
	if err != nil {
		return RemoteToolBackendEnrollment{}, err
	}
	if found && current.commandID == command.CommandID {
		if current.enrollment.revision == command.ExpectedRevision+1 &&
			current.enrollment.status == RemoteToolBackendEnrollmentActive &&
			remoteToolBackendEnrollmentMatchesCommand(current.enrollment, command) {
			return cloneRemoteToolBackendEnrollment(current.enrollment), nil
		}
		return RemoteToolBackendEnrollment{}, ErrRemoteToolBackendEnrollmentConflict
	}
	actualRevision := int64(0)
	causationID := ""
	if found {
		actualRevision = current.enrollment.revision
		causationID = current.eventID
		if !now.After(current.enrollment.configuredAt) {
			return RemoteToolBackendEnrollment{}, ErrRemoteToolBackendEnrollmentConflict
		}
	}
	if actualRevision != command.ExpectedRevision {
		return RemoteToolBackendEnrollment{}, ErrRemoteToolBackendEnrollmentConflict
	}
	enrollment, err := NewRemoteToolBackendEnrollment(RemoteToolBackendEnrollmentInput{
		Version: remoteToolBackendEnrollmentVersion, EnrollmentID: command.EnrollmentID,
		BackendKind: command.BackendKind, AdapterID: command.AdapterID,
		ProviderID: command.ProviderID, ProviderAccountID: command.ProviderAccountID,
		ProviderAccountPolicyVersion:  command.ProviderAccountPolicyVersion,
		ProviderAccountPolicyRevision: command.ProviderAccountPolicyRevision,
		ProviderAccountPolicyDigest:   command.ProviderAccountPolicyDigest,
		EndpointFingerprint:           command.EndpointFingerprint, MCPServerID: command.MCPServerID,
		AllowedTools: command.AllowedTools, Revision: actualRevision + 1,
		Status:                 RemoteToolBackendEnrollmentActive,
		MaximumConcurrentCalls: command.MaximumConcurrentCalls,
		MaximumCallsPerAttempt: command.MaximumCallsPerAttempt,
		Timeout:                command.Timeout, MaximumResultBytes: command.MaximumResultBytes,
		MaximumBudgetUnits: command.MaximumBudgetUnits, ConfiguredAt: now,
	})
	if err != nil {
		return RemoteToolBackendEnrollment{}, err
	}
	eventType := "RemoteToolBackendEnrollmentConfigured"
	eventID := deterministicEventID(
		eventType, streamID, strconv.FormatInt(enrollment.revision, 10),
		command.CommandID, enrollment.digest,
	)
	event := newEvent(
		eventID, streamID, enrollment.revision, eventType, now,
		command.CorrelationID, causationID,
		remoteToolBackendEnrollmentPayloadFrom(enrollment, command.CommandID),
	)
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{
			{StreamID: streamID, Sequence: command.ExpectedRevision},
			{
				StreamID: providerAccountPolicyStream(command.ProviderAccountID),
				Sequence: command.ProviderAccountPolicyRevision,
			},
		},
		[]journal.Event{event},
	); err != nil {
		return RemoteToolBackendEnrollment{}, mapRemoteToolBackendEnrollmentWriteError(err)
	}
	return cloneRemoteToolBackendEnrollment(enrollment), nil
}

func (authority *Authority) RevokeRemoteToolBackendEnrollment(
	ctx context.Context,
	command RemoteToolBackendEnrollmentRevokeCommand,
) (RemoteToolBackendEnrollment, error) {
	if authority == nil || ctx == nil || !validRemoteToolBackendEnrollmentRevokeCommand(command) {
		return RemoteToolBackendEnrollment{}, ErrInvalidRemoteToolBackendEnrollment
	}
	if err := ctx.Err(); err != nil {
		return RemoteToolBackendEnrollment{}, err
	}
	now := authority.now()
	if now.IsZero() || now.Location() != time.UTC {
		return RemoteToolBackendEnrollment{}, ErrInvalidRemoteToolBackendEnrollment
	}
	streamID := remoteToolBackendEnrollmentStream(command.EnrollmentID)
	events, err := authority.store.ReadStream(ctx, streamID)
	if err != nil {
		return RemoteToolBackendEnrollment{}, err
	}
	current, found, err := replayRemoteToolBackendEnrollment(command.EnrollmentID, events)
	if err != nil {
		return RemoteToolBackendEnrollment{}, err
	}
	if !found {
		return RemoteToolBackendEnrollment{}, ErrRemoteToolBackendEnrollmentNotFound
	}
	if current.commandID == command.CommandID {
		if current.enrollment.revision == command.ExpectedRevision+1 &&
			current.enrollment.status == RemoteToolBackendEnrollmentRevoked &&
			current.enrollment.providerID == command.ProviderID &&
			current.enrollment.providerAccountID == command.ProviderAccountID {
			return cloneRemoteToolBackendEnrollment(current.enrollment), nil
		}
		return RemoteToolBackendEnrollment{}, ErrRemoteToolBackendEnrollmentConflict
	}
	if current.enrollment.revision != command.ExpectedRevision ||
		current.enrollment.providerID != command.ProviderID ||
		current.enrollment.providerAccountID != command.ProviderAccountID ||
		current.enrollment.status != RemoteToolBackendEnrollmentActive ||
		!now.After(current.enrollment.configuredAt) {
		return RemoteToolBackendEnrollment{}, ErrRemoteToolBackendEnrollmentConflict
	}
	revoked, err := NewRemoteToolBackendEnrollment(RemoteToolBackendEnrollmentInput{
		Version: current.enrollment.version, EnrollmentID: current.enrollment.enrollmentID,
		BackendKind: current.enrollment.backendKind, AdapterID: current.enrollment.adapterID,
		ProviderID:                    current.enrollment.providerID,
		ProviderAccountID:             current.enrollment.providerAccountID,
		ProviderAccountPolicyVersion:  current.enrollment.providerAccountPolicyVersion,
		ProviderAccountPolicyRevision: current.enrollment.providerAccountPolicyRevision,
		ProviderAccountPolicyDigest:   current.enrollment.providerAccountPolicyDigest,
		EndpointFingerprint:           current.enrollment.endpointFingerprint,
		MCPServerID:                   current.enrollment.mcpServerID,
		AllowedTools:                  current.enrollment.allowedTools,
		Revision:                      current.enrollment.revision + 1,
		Status:                        RemoteToolBackendEnrollmentRevoked,
		MaximumConcurrentCalls:        current.enrollment.maximumConcurrentCalls,
		MaximumCallsPerAttempt:        current.enrollment.maximumCallsPerAttempt,
		Timeout:                       current.enrollment.timeout,
		MaximumResultBytes:            current.enrollment.maximumResultBytes,
		MaximumBudgetUnits:            current.enrollment.maximumBudgetUnits,
		ConfiguredAt:                  now,
	})
	if err != nil {
		return RemoteToolBackendEnrollment{}, err
	}
	eventType := "RemoteToolBackendEnrollmentRevoked"
	eventID := deterministicEventID(
		eventType, streamID, strconv.FormatInt(revoked.revision, 10),
		command.CommandID, revoked.digest,
	)
	event := newEvent(
		eventID, streamID, revoked.revision, eventType, now,
		command.CorrelationID, current.eventID,
		remoteToolBackendEnrollmentPayloadFrom(revoked, command.CommandID),
	)
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: command.ExpectedRevision}},
		[]journal.Event{event},
	); err != nil {
		return RemoteToolBackendEnrollment{}, mapRemoteToolBackendEnrollmentWriteError(err)
	}
	return cloneRemoteToolBackendEnrollment(revoked), nil
}

func (authority *Authority) RemoteToolBackendEnrollment(
	ctx context.Context,
	enrollmentID string,
) (RemoteToolBackendEnrollment, error) {
	if authority == nil || ctx == nil || !validRemoteToolBackendIdentifier(enrollmentID) {
		return RemoteToolBackendEnrollment{}, ErrInvalidRemoteToolBackendEnrollment
	}
	if err := ctx.Err(); err != nil {
		return RemoteToolBackendEnrollment{}, err
	}
	events, err := authority.store.ReadStream(ctx, remoteToolBackendEnrollmentStream(enrollmentID))
	if err != nil {
		return RemoteToolBackendEnrollment{}, err
	}
	current, found, err := replayRemoteToolBackendEnrollment(enrollmentID, events)
	if err != nil {
		return RemoteToolBackendEnrollment{}, err
	}
	if !found {
		return RemoteToolBackendEnrollment{}, ErrRemoteToolBackendEnrollmentNotFound
	}
	return cloneRemoteToolBackendEnrollment(current.enrollment), nil
}

func DecodeRemoteToolBackendEnrollmentEvent(
	event journal.Event,
	previousEventID string,
) (RemoteToolBackendEnrollment, string, error) {
	if event.SchemaVersion != 1 ||
		(event.Type != "RemoteToolBackendEnrollmentConfigured" &&
			event.Type != "RemoteToolBackendEnrollmentRevoked") ||
		!validOpaqueID(event.CorrelationID) ||
		!strings.HasPrefix(event.StreamID, "remote-tool-backend-enrollment/") ||
		event.Seq <= 0 {
		return RemoteToolBackendEnrollment{}, "", ErrRemoteToolBackendEnrollmentConflict
	}
	var payload remoteToolBackendEnrollmentPayload
	if decodeExactPayload(event.PayloadJSON, &payload) != nil {
		return RemoteToolBackendEnrollment{}, "", ErrRemoteToolBackendEnrollmentConflict
	}
	configuredAt, err := parseUTC(payload.ConfiguredAt)
	enrollmentID := strings.TrimPrefix(event.StreamID, "remote-tool-backend-enrollment/")
	expectedStatus := RemoteToolBackendEnrollmentActive
	if event.Type == "RemoteToolBackendEnrollmentRevoked" {
		expectedStatus = RemoteToolBackendEnrollmentRevoked
	}
	if err != nil || payload.EnrollmentID != enrollmentID ||
		payload.Revision != event.Seq || payload.Status != expectedStatus ||
		!configuredAt.Equal(event.EmittedAt) ||
		(event.Seq == 1 && event.Type != "RemoteToolBackendEnrollmentConfigured") ||
		(event.Seq == 1 && previousEventID != "") ||
		(event.Seq == 1 && event.CausationID != "") ||
		(event.Seq > 1 && event.CausationID != previousEventID) ||
		!validOpaqueID(payload.CommandID) {
		return RemoteToolBackendEnrollment{}, "", ErrRemoteToolBackendEnrollmentConflict
	}
	enrollment, err := NewRemoteToolBackendEnrollment(RemoteToolBackendEnrollmentInput{
		Version: payload.EnrollmentVersion, EnrollmentID: payload.EnrollmentID,
		BackendKind: payload.BackendKind, AdapterID: payload.AdapterID,
		ProviderID: payload.ProviderID, ProviderAccountID: payload.ProviderAccountID,
		ProviderAccountPolicyVersion:  payload.ProviderAccountPolicyVersion,
		ProviderAccountPolicyRevision: payload.ProviderAccountPolicyRevision,
		ProviderAccountPolicyDigest:   payload.ProviderAccountPolicyDigest,
		EndpointFingerprint:           payload.EndpointFingerprint,
		MCPServerID:                   payload.MCPServerID,
		AllowedTools:                  payload.AllowedTools,
		Revision:                      payload.Revision, Status: payload.Status,
		MaximumConcurrentCalls: payload.MaximumConcurrentCalls,
		MaximumCallsPerAttempt: payload.MaximumCallsPerAttempt,
		Timeout:                time.Duration(payload.TimeoutNanoseconds),
		MaximumResultBytes:     payload.MaximumResultBytes,
		MaximumBudgetUnits:     payload.MaximumBudgetUnits,
		ConfiguredAt:           configuredAt,
	})
	if err != nil || enrollment.digest != payload.EnrollmentDigest {
		return RemoteToolBackendEnrollment{}, "", ErrRemoteToolBackendEnrollmentConflict
	}
	expectedEventID := deterministicEventID(
		event.Type, event.StreamID, strconv.FormatInt(enrollment.revision, 10),
		payload.CommandID, enrollment.digest,
	)
	if event.ID != expectedEventID || event.IdempotencyKey != event.ID {
		return RemoteToolBackendEnrollment{}, "", ErrRemoteToolBackendEnrollmentConflict
	}
	return cloneRemoteToolBackendEnrollment(enrollment), payload.CommandID, nil
}

func replayRemoteToolBackendEnrollment(
	enrollmentID string,
	events []journal.Event,
) (remoteToolBackendEnrollmentState, bool, error) {
	if !validRemoteToolBackendIdentifier(enrollmentID) {
		return remoteToolBackendEnrollmentState{}, false, ErrInvalidRemoteToolBackendEnrollment
	}
	streamID := remoteToolBackendEnrollmentStream(enrollmentID)
	var current remoteToolBackendEnrollmentState
	for index, event := range events {
		if event.StreamID != streamID || event.Seq != int64(index+1) {
			return remoteToolBackendEnrollmentState{}, false, ErrRemoteToolBackendEnrollmentConflict
		}
		candidate, commandID, err := DecodeRemoteToolBackendEnrollmentEvent(event, current.eventID)
		if err != nil || candidate.enrollmentID != enrollmentID ||
			(index > 0 && (candidate.providerID != current.enrollment.providerID ||
				candidate.providerAccountID != current.enrollment.providerAccountID ||
				candidate.backendKind != current.enrollment.backendKind)) {
			return remoteToolBackendEnrollmentState{}, false, ErrRemoteToolBackendEnrollmentConflict
		}
		current = remoteToolBackendEnrollmentState{
			enrollment: candidate, commandID: commandID, eventID: event.ID,
		}
	}
	return current, len(events) > 0, nil
}

func remoteToolBackendEnrollmentPayloadFrom(
	enrollment RemoteToolBackendEnrollment,
	commandID string,
) remoteToolBackendEnrollmentPayload {
	return remoteToolBackendEnrollmentPayload{
		CommandID: commandID, EnrollmentVersion: enrollment.version,
		EnrollmentID: enrollment.enrollmentID, BackendKind: enrollment.backendKind,
		AdapterID: enrollment.adapterID, ProviderID: enrollment.providerID,
		ProviderAccountID:             enrollment.providerAccountID,
		ProviderAccountPolicyVersion:  enrollment.providerAccountPolicyVersion,
		ProviderAccountPolicyRevision: enrollment.providerAccountPolicyRevision,
		ProviderAccountPolicyDigest:   enrollment.providerAccountPolicyDigest,
		EndpointFingerprint:           enrollment.endpointFingerprint,
		MCPServerID:                   enrollment.mcpServerID,
		AllowedTools:                  append([]string{}, enrollment.allowedTools...),
		Revision:                      enrollment.revision, Status: enrollment.status,
		MaximumConcurrentCalls: enrollment.maximumConcurrentCalls,
		MaximumCallsPerAttempt: enrollment.maximumCallsPerAttempt,
		TimeoutNanoseconds:     int64(enrollment.timeout),
		MaximumResultBytes:     enrollment.maximumResultBytes,
		MaximumBudgetUnits:     enrollment.maximumBudgetUnits,
		ConfiguredAt:           enrollment.configuredAt.Format(time.RFC3339Nano),
		EnrollmentDigest:       enrollment.digest,
	}
}

func validRemoteToolBackendEnrollmentCommand(
	command RemoteToolBackendEnrollmentCommand,
) bool {
	return validOpaqueID(command.CommandID) &&
		validRemoteToolBackendIdentifier(command.EnrollmentID) &&
		validRemoteToolBackendIdentifier(command.AdapterID) &&
		credentials.ValidProviderAccountIdentifier(command.ProviderID, command.ProviderAccountID) &&
		command.ProviderAccountPolicyVersion > 0 &&
		command.ProviderAccountPolicyRevision > 0 &&
		validSHA256Hex(command.ProviderAccountPolicyDigest) &&
		validSHA256Hex(command.EndpointFingerprint) &&
		command.ExpectedRevision >= 0 && command.ExpectedRevision < int64(^uint32(0)) &&
		validRemoteToolBackendShape(command.BackendKind, command.MCPServerID, command.AllowedTools) &&
		validRemoteToolBackendLimits(
			command.MaximumConcurrentCalls, command.MaximumCallsPerAttempt,
			command.Timeout, command.MaximumResultBytes, command.MaximumBudgetUnits,
		) && validOpaqueID(command.CorrelationID)
}

func validRemoteToolBackendEnrollmentRevokeCommand(
	command RemoteToolBackendEnrollmentRevokeCommand,
) bool {
	return validOpaqueID(command.CommandID) &&
		validRemoteToolBackendIdentifier(command.EnrollmentID) &&
		credentials.ValidProviderAccountIdentifier(command.ProviderID, command.ProviderAccountID) &&
		command.ExpectedRevision > 0 && command.ExpectedRevision < int64(^uint32(0)) &&
		validOpaqueID(command.CorrelationID)
}

func validRemoteToolBackendEnrollmentShape(enrollment RemoteToolBackendEnrollment) bool {
	return enrollment.version == remoteToolBackendEnrollmentVersion &&
		validRemoteToolBackendIdentifier(enrollment.enrollmentID) &&
		validRemoteToolBackendIdentifier(enrollment.adapterID) &&
		credentials.ValidProviderAccountIdentifier(enrollment.providerID, enrollment.providerAccountID) &&
		enrollment.providerAccountPolicyVersion > 0 &&
		enrollment.providerAccountPolicyRevision > 0 &&
		validSHA256Hex(enrollment.providerAccountPolicyDigest) &&
		validSHA256Hex(enrollment.endpointFingerprint) &&
		enrollment.revision > 0 &&
		(enrollment.status == RemoteToolBackendEnrollmentActive ||
			enrollment.status == RemoteToolBackendEnrollmentRevoked) &&
		validRemoteToolBackendShape(enrollment.backendKind, enrollment.mcpServerID, enrollment.allowedTools) &&
		validRemoteToolBackendLimits(
			enrollment.maximumConcurrentCalls, enrollment.maximumCallsPerAttempt,
			enrollment.timeout, enrollment.maximumResultBytes, enrollment.maximumBudgetUnits,
		) && !enrollment.configuredAt.IsZero() && enrollment.configuredAt.Location() == time.UTC
}

func validRemoteToolBackendShape(kind string, mcpServerID string, allowedTools []string) bool {
	switch kind {
	case RemoteToolBackendWebSearch:
		return mcpServerID == "" && len(allowedTools) == 0
	case RemoteToolBackendMCPServer:
		if !validRemoteToolBackendIdentifier(mcpServerID) ||
			len(allowedTools) == 0 || len(allowedTools) > 128 ||
			!sort.StringsAreSorted(allowedTools) {
			return false
		}
		for index, tool := range allowedTools {
			if !validRemoteToolBackendIdentifier(tool) ||
				(index > 0 && tool == allowedTools[index-1]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func validRemoteToolBackendLimits(
	maximumConcurrentCalls int,
	maximumCallsPerAttempt int,
	timeout time.Duration,
	maximumResultBytes int,
	maximumBudgetUnits int64,
) bool {
	return maximumConcurrentCalls > 0 && maximumConcurrentCalls <= maximumRemoteToolCalls &&
		maximumCallsPerAttempt > 0 && maximumCallsPerAttempt <= maximumRemoteToolCalls &&
		timeout >= time.Second && timeout <= 2*time.Minute &&
		maximumResultBytes >= 256 && maximumResultBytes <= maximumRemoteToolResultBytes &&
		maximumBudgetUnits >= 0
}

func validRemoteToolBackendIdentifier(value string) bool {
	if value == "" || len(value) > maxAuthorityIDBytes ||
		value[0] == '.' || value[0] == '-' || value[0] == '_' ||
		value[len(value)-1] == '.' || value[len(value)-1] == '-' ||
		value[len(value)-1] == '_' {
		return false
	}
	previousSeparator := false
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z',
			character >= '0' && character <= '9':
			previousSeparator = false
		case (character == '.' || character == '-' || character == '_') && !previousSeparator:
			previousSeparator = true
		default:
			return false
		}
	}
	return true
}

func remoteToolBackendEnrollmentPolicyMatches(
	command RemoteToolBackendEnrollmentCommand,
	policy ProviderAccountPolicy,
) bool {
	return policy.Valid() && policy.ProviderID() == command.ProviderID &&
		policy.ProviderAccountID() == command.ProviderAccountID &&
		policy.Version() == command.ProviderAccountPolicyVersion &&
		policy.Revision() == command.ProviderAccountPolicyRevision &&
		policy.Digest() == command.ProviderAccountPolicyDigest
}

func remoteToolBackendEnrollmentMatchesCommand(
	enrollment RemoteToolBackendEnrollment,
	command RemoteToolBackendEnrollmentCommand,
) bool {
	return enrollment.enrollmentID == command.EnrollmentID &&
		enrollment.backendKind == command.BackendKind && enrollment.adapterID == command.AdapterID &&
		enrollment.providerID == command.ProviderID &&
		enrollment.providerAccountID == command.ProviderAccountID &&
		enrollment.providerAccountPolicyVersion == command.ProviderAccountPolicyVersion &&
		enrollment.providerAccountPolicyRevision == command.ProviderAccountPolicyRevision &&
		enrollment.providerAccountPolicyDigest == command.ProviderAccountPolicyDigest &&
		enrollment.endpointFingerprint == command.EndpointFingerprint &&
		enrollment.mcpServerID == command.MCPServerID &&
		equalRemoteToolBackendStrings(enrollment.allowedTools, command.AllowedTools) &&
		enrollment.maximumConcurrentCalls == command.MaximumConcurrentCalls &&
		enrollment.maximumCallsPerAttempt == command.MaximumCallsPerAttempt &&
		enrollment.timeout == command.Timeout &&
		enrollment.maximumResultBytes == command.MaximumResultBytes &&
		enrollment.maximumBudgetUnits == command.MaximumBudgetUnits
}

func remoteToolBackendEnrollmentDigest(enrollment RemoteToolBackendEnrollment) string {
	digest := sha256.New()
	fields := []string{
		"loom.remote-tool-backend-enrollment.v1",
		strconv.Itoa(enrollment.version), enrollment.enrollmentID,
		enrollment.backendKind, enrollment.adapterID, enrollment.providerID,
		enrollment.providerAccountID,
		strconv.Itoa(enrollment.providerAccountPolicyVersion),
		strconv.FormatInt(enrollment.providerAccountPolicyRevision, 10),
		enrollment.providerAccountPolicyDigest, enrollment.endpointFingerprint,
		enrollment.mcpServerID, strconv.FormatInt(enrollment.revision, 10),
		enrollment.status, strconv.Itoa(enrollment.maximumConcurrentCalls),
		strconv.Itoa(enrollment.maximumCallsPerAttempt),
		strconv.FormatInt(int64(enrollment.timeout), 10),
		strconv.Itoa(enrollment.maximumResultBytes),
		strconv.FormatInt(enrollment.maximumBudgetUnits, 10),
		enrollment.configuredAt.Format(time.RFC3339Nano),
	}
	for _, field := range fields {
		writeProviderAccountPolicyField(digest, field)
	}
	for _, tool := range enrollment.allowedTools {
		writeProviderAccountPolicyField(digest, tool)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func cloneRemoteToolBackendEnrollment(
	enrollment RemoteToolBackendEnrollment,
) RemoteToolBackendEnrollment {
	enrollment.allowedTools = append([]string(nil), enrollment.allowedTools...)
	return enrollment
}

func equalRemoteToolBackendStrings(left, right []string) bool {
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

func remoteToolBackendEnrollmentStream(enrollmentID string) string {
	return "remote-tool-backend-enrollment/" + enrollmentID
}

func mapRemoteToolBackendEnrollmentWriteError(err error) error {
	switch {
	case errors.Is(err, journal.ErrStreamHeadConflict),
		errors.Is(err, journal.ErrSequenceConflict),
		errors.Is(err, journal.ErrIdempotencyConflict),
		errors.Is(err, journal.ErrPartialEventBatchConflict):
		return fmt.Errorf("%w: %v", ErrRemoteToolBackendEnrollmentConflict, err)
	default:
		return err
	}
}
