package roundtable

import (
	"encoding/json"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
)

const seatBindingDigestDomain = "loom.roundtable.seat-binding.v1"

// SessionContext binds a RoundTable to the product workflow that owns it.
// Legacy ledger-only sessions omit this value and remain read compatible.
type SessionContext struct {
	ConversationID string `json:"conversation_id"`
	MissionID      string `json:"mission_id"`
	TeamID         string `json:"team_id"`
	TeamVersion    int    `json:"team_version"`
	WorkspaceID    string `json:"workspace_id"`
}

// SessionLinkRequest is the non-authoritative product identity supplied by a
// client. The daemon resolves it against the projected Mission and Team before
// a SessionContext reaches Authority.
type SessionLinkRequest struct {
	ConversationID string `json:"conversation_id"`
	MissionID      string `json:"mission_id"`
	TeamInstanceID string `json:"team_instance_id"`
}

// SeatBindingRequest names one configured Team role. It intentionally omits
// Provider Account, credential revision, model and limits; those values are
// resolved and frozen by the daemon from authoritative Team state.
type SeatBindingRequest struct {
	AgentDefinitionID string `json:"agent_definition_id"`
	TeamRoleKind      string `json:"team_role_kind"`
	RuntimeProfileID  string `json:"runtime_profile_id"`
}

// SeatExecutionBinding is the strict, non-secret wire form of a validated
// runtime.FrozenExecutionBinding. CredentialReference is an opaque reference,
// never credential material.
type SeatExecutionBinding struct {
	ProfileID                  string               `json:"profile_id"`
	HarnessAdapter             string               `json:"harness_adapter"`
	RuntimeInstanceID          string               `json:"runtime_instance_id"`
	ProviderID                 string               `json:"provider_id"`
	ProviderAccountID          string               `json:"provider_account_id"`
	ModelID                    string               `json:"model_id"`
	AuthMode                   loomruntime.AuthMode `json:"auth_mode"`
	EndpointFingerprint        string               `json:"endpoint_fingerprint"`
	CredentialReference        string               `json:"credential_reference"`
	CredentialRevision         int64                `json:"credential_revision"`
	ReasoningEffort            string               `json:"reasoning_effort"`
	TimeoutNanoseconds         int64                `json:"timeout_nanoseconds"`
	Budget                     *int64               `json:"budget"`
	Capabilities               []string             `json:"capabilities"`
	RemoteToolEnrollmentID     string               `json:"remote_tool_enrollment_id"`
	RemoteToolEnrollmentDigest string               `json:"remote_tool_enrollment_digest"`
	BindingDigest              string               `json:"binding_digest"`
}

// FrozenSeatBinding binds one Team role and one immutable execution route to a
// specific Session membership revision.
type FrozenSeatBinding struct {
	AgentDefinitionID  string               `json:"agent_definition_id"`
	TeamRoleKind       string               `json:"team_role_kind"`
	RuntimeProfileID   string               `json:"runtime_profile_id"`
	ExecutionBinding   SeatExecutionBinding `json:"execution_binding"`
	MembershipRevision int                  `json:"membership_revision"`
	BindingDigest      string               `json:"binding_digest"`
}

// FreezeSeatBinding validates the runtime binding and content-addresses the
// complete Session, Team, Seat, role and execution identity.
func FreezeSeatBinding(
	sessionID string,
	seatID string,
	context SessionContext,
	agentDefinitionID string,
	teamRoleKind string,
	runtimeProfileID string,
	executionBinding loomruntime.FrozenExecutionBinding,
	membershipRevision int,
) (FrozenSeatBinding, error) {
	validated, err := loomruntime.ValidateFrozenExecutionBinding(executionBinding)
	if err != nil {
		return FrozenSeatBinding{}, ErrInvalidRoundtableSeatBinding
	}
	binding := FrozenSeatBinding{
		AgentDefinitionID:  agentDefinitionID,
		TeamRoleKind:       teamRoleKind,
		RuntimeProfileID:   runtimeProfileID,
		ExecutionBinding:   seatExecutionBindingFromRuntime(validated),
		MembershipRevision: membershipRevision,
	}
	if !validSessionContext(context) ||
		!validRoundtableID(sessionID, MaxSessionIDBytes) ||
		!validRoundtableID(seatID, MaxSeatIDBytes) ||
		!validRoundtableID(agentDefinitionID, MaxSeatIDBytes) ||
		!validRoundtableID(runtimeProfileID, MaxSessionIDBytes) ||
		(teamRoleKind != "main" && teamRoleKind != "subagent") ||
		membershipRevision <= 0 || validated.ProfileID != runtimeProfileID {
		return FrozenSeatBinding{}, ErrInvalidRoundtableSeatBinding
	}
	digest, err := digestSeatBinding(sessionID, seatID, context, binding)
	if err != nil {
		return FrozenSeatBinding{}, ErrInvalidRoundtableSeatBinding
	}
	binding.BindingDigest = digest
	return cloneFrozenSeatBinding(binding), nil
}

func validateFrozenSeatBinding(
	sessionID string,
	seatID string,
	context SessionContext,
	input FrozenSeatBinding,
) (FrozenSeatBinding, error) {
	expected, err := FreezeSeatBinding(
		sessionID, seatID, context, input.AgentDefinitionID,
		input.TeamRoleKind, input.RuntimeProfileID,
		input.ExecutionBinding.runtimeBinding(), input.MembershipRevision,
	)
	if err != nil || input.BindingDigest == "" ||
		input.BindingDigest != expected.BindingDigest ||
		!equalFrozenSeatBindingValue(input, expected) {
		return FrozenSeatBinding{}, ErrInvalidRoundtableSeatBinding
	}
	return expected, nil
}

func digestSeatBinding(
	sessionID string,
	seatID string,
	context SessionContext,
	binding FrozenSeatBinding,
) (string, error) {
	binding.BindingDigest = ""
	encoded, err := json.Marshal(struct {
		Domain    string            `json:"domain"`
		SessionID string            `json:"session_id"`
		SeatID    string            `json:"seat_id"`
		Context   SessionContext    `json:"context"`
		Binding   FrozenSeatBinding `json:"binding"`
	}{
		Domain: seatBindingDigestDomain, SessionID: sessionID, SeatID: seatID,
		Context: context, Binding: binding,
	})
	if err != nil {
		return "", err
	}
	return digestBytes(string(encoded)), nil
}

func validSessionContext(input SessionContext) bool {
	return validRoundtableID(input.ConversationID, MaxSessionIDBytes) &&
		validRoundtableID(input.MissionID, MaxSessionIDBytes) &&
		validRoundtableID(input.TeamID, MaxSessionIDBytes) &&
		input.TeamVersion > 0 &&
		validRoundtableID(input.WorkspaceID, MaxSessionIDBytes)
}

func cloneSessionContext(input *SessionContext) *SessionContext {
	if input == nil {
		return nil
	}
	clone := *input
	return &clone
}

func cloneFrozenSeatBinding(input FrozenSeatBinding) FrozenSeatBinding {
	clone := input
	clone.ExecutionBinding.Capabilities = append(
		[]string(nil), input.ExecutionBinding.Capabilities...,
	)
	if input.ExecutionBinding.Budget != nil {
		budget := *input.ExecutionBinding.Budget
		clone.ExecutionBinding.Budget = &budget
	}
	return clone
}

func cloneFrozenSeatBindingPointer(input *FrozenSeatBinding) *FrozenSeatBinding {
	if input == nil {
		return nil
	}
	clone := cloneFrozenSeatBinding(*input)
	return &clone
}

func equalFrozenSeatBindings(left, right *FrozenSeatBinding) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return equalFrozenSeatBindingValue(*left, *right)
}

func equalFrozenSeatBindingValue(left, right FrozenSeatBinding) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func seatExecutionBindingFromRuntime(
	input loomruntime.FrozenExecutionBinding,
) SeatExecutionBinding {
	output := SeatExecutionBinding{
		ProfileID: input.ProfileID, HarnessAdapter: input.HarnessAdapter,
		RuntimeInstanceID: input.RuntimeInstanceID, ProviderID: input.ProviderID,
		ProviderAccountID: input.ProviderAccountID, ModelID: input.ModelID,
		AuthMode: input.AuthMode, EndpointFingerprint: input.EndpointFingerprint,
		CredentialReference:        input.CredentialReference,
		CredentialRevision:         input.CredentialRevision,
		ReasoningEffort:            input.ReasoningEffort,
		TimeoutNanoseconds:         int64(input.Timeout),
		Capabilities:               append([]string(nil), input.Capabilities...),
		RemoteToolEnrollmentID:     input.RemoteToolEnrollmentID,
		RemoteToolEnrollmentDigest: input.RemoteToolEnrollmentDigest,
		BindingDigest:              input.BindingDigest,
	}
	if input.Budget != nil {
		budget := *input.Budget
		output.Budget = &budget
	}
	return output
}

func (input SeatExecutionBinding) runtimeBinding() loomruntime.FrozenExecutionBinding {
	output := loomruntime.FrozenExecutionBinding{
		ProfileID: input.ProfileID, HarnessAdapter: input.HarnessAdapter,
		RuntimeInstanceID: input.RuntimeInstanceID, ProviderID: input.ProviderID,
		ProviderAccountID: input.ProviderAccountID, ModelID: input.ModelID,
		AuthMode: input.AuthMode, EndpointFingerprint: input.EndpointFingerprint,
		CredentialReference:        input.CredentialReference,
		CredentialRevision:         input.CredentialRevision,
		ReasoningEffort:            input.ReasoningEffort,
		Timeout:                    time.Duration(input.TimeoutNanoseconds),
		Capabilities:               append([]string(nil), input.Capabilities...),
		RemoteToolEnrollmentID:     input.RemoteToolEnrollmentID,
		RemoteToolEnrollmentDigest: input.RemoteToolEnrollmentDigest,
		BindingDigest:              input.BindingDigest,
	}
	if input.Budget != nil {
		budget := *input.Budget
		output.Budget = &budget
	}
	return output
}

// RuntimeExecutionBinding returns the validated runtime binding frozen for
// this seat without exposing credential material.
func (input FrozenSeatBinding) RuntimeExecutionBinding() (
	loomruntime.FrozenExecutionBinding,
	error,
) {
	return loomruntime.ValidateFrozenExecutionBinding(input.ExecutionBinding.runtimeBinding())
}
