package app

import (
	"context"
	"errors"
	"time"

	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/work"
)

var ErrRemoteToolBackendEnrollmentUnavailable = errors.New(
	"remote tool backend enrollment unavailable",
)

type RemoteToolBackendEnrollmentAuthority interface {
	ConfigureRemoteToolBackendEnrollment(
		context.Context,
		work.RemoteToolBackendEnrollmentCommand,
	) (work.RemoteToolBackendEnrollment, error)
	RevokeRemoteToolBackendEnrollment(
		context.Context,
		work.RemoteToolBackendEnrollmentRevokeCommand,
	) (work.RemoteToolBackendEnrollment, error)
}

type RemoteToolBackendEnrollmentCommand struct {
	EnrollmentID                  string   `json:"enrollment_id"`
	BackendKind                   string   `json:"backend_kind"`
	AdapterID                     string   `json:"adapter_id"`
	ProviderID                    string   `json:"provider_id"`
	ProviderAccountID             string   `json:"provider_account_id"`
	ProviderAccountPolicyVersion  int      `json:"provider_account_policy_version"`
	ProviderAccountPolicyRevision int64    `json:"provider_account_policy_revision"`
	ProviderAccountPolicyDigest   string   `json:"provider_account_policy_digest"`
	EndpointFingerprint           string   `json:"endpoint_fingerprint"`
	MCPServerID                   string   `json:"mcp_server_id"`
	AllowedTools                  []string `json:"allowed_tools"`
	ExpectedRevision              int64    `json:"expected_revision"`
	MaximumConcurrentCalls        int      `json:"maximum_concurrent_calls"`
	MaximumCallsPerAttempt        int      `json:"maximum_calls_per_attempt"`
	TimeoutSeconds                int64    `json:"timeout_seconds"`
	MaximumResultBytes            int      `json:"maximum_result_bytes"`
	MaximumBudgetUnits            int64    `json:"maximum_budget_units"`
	OperationID                   string   `json:"operation_id"`
	CorrelationID                 string   `json:"-"`
}

type RemoteToolBackendEnrollmentRevokeCommand struct {
	EnrollmentID      string `json:"enrollment_id"`
	ProviderID        string `json:"provider_id"`
	ProviderAccountID string `json:"provider_account_id"`
	ExpectedRevision  int64  `json:"expected_revision"`
	OperationID       string `json:"operation_id"`
	CorrelationID     string `json:"-"`
}

type RemoteToolBackendEnrollmentResult struct {
	EnrollmentAvailable           bool     `json:"enrollment_available"`
	EnrollmentVersion             int      `json:"enrollment_version"`
	EnrollmentID                  string   `json:"enrollment_id"`
	BackendKind                   string   `json:"backend_kind"`
	AdapterID                     string   `json:"adapter_id"`
	ProviderID                    string   `json:"provider_id"`
	ProviderAccountID             string   `json:"provider_account_id"`
	ProviderAccountPolicyVersion  int      `json:"provider_account_policy_version"`
	ProviderAccountPolicyRevision int64    `json:"provider_account_policy_revision"`
	ProviderAccountPolicyDigest   string   `json:"provider_account_policy_digest"`
	PolicyCurrent                 bool     `json:"policy_current"`
	EndpointFingerprint           string   `json:"endpoint_fingerprint"`
	MCPServerID                   string   `json:"mcp_server_id"`
	AllowedTools                  []string `json:"allowed_tools"`
	Revision                      int64    `json:"revision"`
	Status                        string   `json:"status"`
	MaximumConcurrentCalls        int      `json:"maximum_concurrent_calls"`
	MaximumCallsPerAttempt        int      `json:"maximum_calls_per_attempt"`
	TimeoutSeconds                int64    `json:"timeout_seconds"`
	MaximumResultBytes            int      `json:"maximum_result_bytes"`
	MaximumBudgetUnits            int64    `json:"maximum_budget_units"`
	ConfiguredAt                  string   `json:"configured_at"`
	EnrollmentDigest              string   `json:"enrollment_digest"`
}

func (service *LocalProductSetupService) ConfigureRemoteToolBackendEnrollment(
	ctx context.Context,
	command RemoteToolBackendEnrollmentCommand,
) (RemoteToolBackendEnrollmentResult, error) {
	if service == nil || ctx == nil || service.remoteToolBackendEnrollments == nil {
		return RemoteToolBackendEnrollmentResult{}, ErrRemoteToolBackendEnrollmentUnavailable
	}
	if command.TimeoutSeconds < 1 || command.TimeoutSeconds > 120 {
		return RemoteToolBackendEnrollmentResult{}, work.ErrInvalidRemoteToolBackendEnrollment
	}
	if !service.remoteToolBackendAccountAvailable(
		command.ProviderID, command.ProviderAccountID,
	) {
		return RemoteToolBackendEnrollmentResult{}, work.ErrInvalidRemoteToolBackendEnrollment
	}
	enrollment, err := service.remoteToolBackendEnrollments.ConfigureRemoteToolBackendEnrollment(
		ctx,
		work.RemoteToolBackendEnrollmentCommand{
			CommandID: command.OperationID, EnrollmentID: command.EnrollmentID,
			BackendKind: command.BackendKind, AdapterID: command.AdapterID,
			ProviderID: command.ProviderID, ProviderAccountID: command.ProviderAccountID,
			ProviderAccountPolicyVersion:  command.ProviderAccountPolicyVersion,
			ProviderAccountPolicyRevision: command.ProviderAccountPolicyRevision,
			ProviderAccountPolicyDigest:   command.ProviderAccountPolicyDigest,
			EndpointFingerprint:           command.EndpointFingerprint,
			MCPServerID:                   command.MCPServerID,
			AllowedTools:                  append([]string(nil), command.AllowedTools...),
			ExpectedRevision:              command.ExpectedRevision,
			MaximumConcurrentCalls:        command.MaximumConcurrentCalls,
			MaximumCallsPerAttempt:        command.MaximumCallsPerAttempt,
			Timeout:                       time.Duration(command.TimeoutSeconds) * time.Second,
			MaximumResultBytes:            command.MaximumResultBytes,
			MaximumBudgetUnits:            command.MaximumBudgetUnits,
			CorrelationID:                 command.CorrelationID,
		},
	)
	if err != nil {
		return RemoteToolBackendEnrollmentResult{}, err
	}
	return service.projectRemoteToolBackendEnrollment(ctx, enrollment)
}

func (service *LocalProductSetupService) remoteToolBackendAccountAvailable(
	providerID string,
	providerAccountID string,
) bool {
	if service == nil || service.projection == nil {
		return false
	}
	for _, account := range service.projection.GlobalReadView().ProviderAccountCredentials(providerID) {
		if account.ProviderID == providerID &&
			account.ProviderAccountID == providerAccountID &&
			account.CredentialReference != "" && account.Revision > 0 &&
			(account.Status == string(credentials.CredentialConfigured) ||
				account.Status == string(credentials.CredentialVerified)) {
			return true
		}
	}
	return false
}

func (service *LocalProductSetupService) RevokeRemoteToolBackendEnrollment(
	ctx context.Context,
	command RemoteToolBackendEnrollmentRevokeCommand,
) (RemoteToolBackendEnrollmentResult, error) {
	if service == nil || ctx == nil || service.remoteToolBackendEnrollments == nil {
		return RemoteToolBackendEnrollmentResult{}, ErrRemoteToolBackendEnrollmentUnavailable
	}
	enrollment, err := service.remoteToolBackendEnrollments.RevokeRemoteToolBackendEnrollment(
		ctx,
		work.RemoteToolBackendEnrollmentRevokeCommand{
			CommandID: command.OperationID, EnrollmentID: command.EnrollmentID,
			ProviderID: command.ProviderID, ProviderAccountID: command.ProviderAccountID,
			ExpectedRevision: command.ExpectedRevision,
			CorrelationID:    command.CorrelationID,
		},
	)
	if err != nil {
		return RemoteToolBackendEnrollmentResult{}, err
	}
	return service.projectRemoteToolBackendEnrollment(ctx, enrollment)
}

func (service *LocalProductSetupService) projectRemoteToolBackendEnrollment(
	ctx context.Context,
	enrollment work.RemoteToolBackendEnrollment,
) (RemoteToolBackendEnrollmentResult, error) {
	if err := service.projection.Rebuild(ctx); err != nil {
		return RemoteToolBackendEnrollmentResult{}, ErrRemoteToolBackendEnrollmentUnavailable
	}
	for _, projected := range service.projection.GlobalReadView().RemoteToolBackendEnrollments(
		enrollment.ProviderID(), enrollment.ProviderAccountID(),
	) {
		if projected.EnrollmentID() == enrollment.EnrollmentID() &&
			projected.Revision() == enrollment.Revision() &&
			projected.Digest() == enrollment.Digest() {
			policy, current := service.projection.GlobalReadView().ProviderAccountPolicy(
				projected.ProviderID(), projected.ProviderAccountID(),
			)
			return remoteToolBackendEnrollmentResult(
				projected,
				current && remoteToolBackendEnrollmentUsesPolicy(projected, policy),
			), nil
		}
	}
	return RemoteToolBackendEnrollmentResult{}, ErrRemoteToolBackendEnrollmentUnavailable
}

func remoteToolBackendEnrollmentResult(
	enrollment work.RemoteToolBackendEnrollment,
	policyCurrent bool,
) RemoteToolBackendEnrollmentResult {
	return RemoteToolBackendEnrollmentResult{
		EnrollmentAvailable: true, EnrollmentVersion: enrollment.Version(),
		EnrollmentID: enrollment.EnrollmentID(), BackendKind: enrollment.BackendKind(),
		AdapterID: enrollment.AdapterID(), ProviderID: enrollment.ProviderID(),
		ProviderAccountID:             enrollment.ProviderAccountID(),
		ProviderAccountPolicyVersion:  enrollment.ProviderAccountPolicyVersion(),
		ProviderAccountPolicyRevision: enrollment.ProviderAccountPolicyRevision(),
		ProviderAccountPolicyDigest:   enrollment.ProviderAccountPolicyDigest(),
		PolicyCurrent:                 policyCurrent,
		EndpointFingerprint:           enrollment.EndpointFingerprint(),
		MCPServerID:                   enrollment.MCPServerID(), AllowedTools: enrollment.AllowedTools(),
		Revision: enrollment.Revision(), Status: enrollment.Status(),
		MaximumConcurrentCalls: enrollment.MaximumConcurrentCalls(),
		MaximumCallsPerAttempt: enrollment.MaximumCallsPerAttempt(),
		TimeoutSeconds:         int64(enrollment.Timeout() / time.Second),
		MaximumResultBytes:     enrollment.MaximumResultBytes(),
		MaximumBudgetUnits:     enrollment.MaximumBudgetUnits(),
		ConfiguredAt:           enrollment.ConfiguredAt().Format(time.RFC3339Nano),
		EnrollmentDigest:       enrollment.Digest(),
	}
}

func remoteToolBackendDirectoryEntry(
	enrollment work.RemoteToolBackendEnrollment,
	policyCurrent bool,
) RemoteToolBackendDirectoryEntry {
	result := remoteToolBackendEnrollmentResult(enrollment, policyCurrent)
	return RemoteToolBackendDirectoryEntry{
		EnrollmentVersion: result.EnrollmentVersion,
		EnrollmentID:      result.EnrollmentID, BackendKind: result.BackendKind,
		AdapterID:                     result.AdapterID,
		ProviderAccountPolicyVersion:  result.ProviderAccountPolicyVersion,
		ProviderAccountPolicyRevision: result.ProviderAccountPolicyRevision,
		ProviderAccountPolicyDigest:   result.ProviderAccountPolicyDigest,
		PolicyCurrent:                 result.PolicyCurrent,
		EndpointFingerprint:           result.EndpointFingerprint,
		MCPServerID:                   result.MCPServerID,
		AllowedTools:                  append([]string(nil), result.AllowedTools...),
		Revision:                      result.Revision, Status: result.Status,
		MaximumConcurrentCalls: result.MaximumConcurrentCalls,
		MaximumCallsPerAttempt: result.MaximumCallsPerAttempt,
		TimeoutSeconds:         result.TimeoutSeconds,
		MaximumResultBytes:     result.MaximumResultBytes,
		MaximumBudgetUnits:     result.MaximumBudgetUnits,
		ConfiguredAt:           result.ConfiguredAt, EnrollmentDigest: result.EnrollmentDigest,
	}
}

func remoteToolBackendEnrollmentUsesPolicy(
	enrollment work.RemoteToolBackendEnrollment,
	policy work.ProviderAccountPolicy,
) bool {
	return policy.Valid() &&
		enrollment.ProviderID() == policy.ProviderID() &&
		enrollment.ProviderAccountID() == policy.ProviderAccountID() &&
		enrollment.ProviderAccountPolicyVersion() == policy.Version() &&
		enrollment.ProviderAccountPolicyRevision() == policy.Revision() &&
		enrollment.ProviderAccountPolicyDigest() == policy.Digest()
}
