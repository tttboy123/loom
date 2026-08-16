package app

import (
	"context"
	"errors"
	"time"

	"loom-pi-rebuild/internal/work"
)

var ErrProviderAccountPolicyUnavailable = errors.New(
	"Provider Account policy unavailable",
)

type ProviderAccountPolicyAuthority interface {
	ConfigureProviderAccountPolicy(
		context.Context,
		work.ProviderAccountPolicyCommand,
	) (work.ProviderAccountPolicy, error)
}

type ProviderAccountPolicyCommand struct {
	ProviderID                 string `json:"provider_id"`
	ProviderAccountID          string `json:"provider_account_id"`
	ExpectedRevision           int64  `json:"expected_revision"`
	MaximumConcurrentAttempts  int    `json:"maximum_concurrent_attempts"`
	DispatchWindowSeconds      int64  `json:"dispatch_window_seconds"`
	MaximumDispatchStarts      int    `json:"maximum_dispatch_starts"`
	MaximumAssignedBudgetUnits int64  `json:"maximum_assigned_budget_units"`
	TrustDomain                string `json:"trust_domain"`
	RetentionMode              string `json:"retention_mode"`
	DataRegion                 string `json:"data_region"`
	OperationID                string `json:"operation_id"`
	CorrelationID              string `json:"-"`
}

type ProviderAccountPolicyResult struct {
	PolicyAvailable            bool   `json:"policy_available"`
	PolicyVersion              int    `json:"policy_version"`
	ProviderID                 string `json:"provider_id"`
	ProviderAccountID          string `json:"provider_account_id"`
	Revision                   int64  `json:"revision"`
	PolicyDigest               string `json:"policy_digest"`
	MaximumConcurrentAttempts  int    `json:"maximum_concurrent_attempts"`
	DispatchWindowSeconds      int64  `json:"dispatch_window_seconds"`
	MaximumDispatchStarts      int    `json:"maximum_dispatch_starts"`
	MaximumAssignedBudgetUnits int64  `json:"maximum_assigned_budget_units"`
	TrustDomain                string `json:"trust_domain"`
	RetentionMode              string `json:"retention_mode"`
	DataRegion                 string `json:"data_region"`
	ConfiguredAt               string `json:"configured_at"`
}

func (service *LocalProductSetupService) ConfigureProviderAccountPolicy(
	ctx context.Context,
	command ProviderAccountPolicyCommand,
) (ProviderAccountPolicyResult, error) {
	if service == nil || ctx == nil || service.providerAccountPolicies == nil {
		return ProviderAccountPolicyResult{}, ErrProviderAccountPolicyUnavailable
	}
	if command.DispatchWindowSeconds < 1 || command.DispatchWindowSeconds > 86_400 {
		return ProviderAccountPolicyResult{}, work.ErrInvalidProviderAccountPolicy
	}
	policy, err := service.providerAccountPolicies.ConfigureProviderAccountPolicy(
		ctx,
		work.ProviderAccountPolicyCommand{
			CommandID:                  command.OperationID,
			ProviderID:                 command.ProviderID,
			ProviderAccountID:          command.ProviderAccountID,
			ExpectedRevision:           command.ExpectedRevision,
			MaximumConcurrentAttempts:  command.MaximumConcurrentAttempts,
			DispatchWindow:             time.Duration(command.DispatchWindowSeconds) * time.Second,
			MaximumDispatchStarts:      command.MaximumDispatchStarts,
			MaximumAssignedBudgetUnits: command.MaximumAssignedBudgetUnits,
			TrustDomain:                command.TrustDomain,
			RetentionMode:              command.RetentionMode,
			DataRegion:                 command.DataRegion,
			CorrelationID:              command.CorrelationID,
		},
	)
	if err != nil {
		return ProviderAccountPolicyResult{}, err
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return ProviderAccountPolicyResult{}, ErrProviderAccountPolicyUnavailable
	}
	projected, ok := service.projection.GlobalReadView().ProviderAccountPolicy(
		policy.ProviderID(), policy.ProviderAccountID(),
	)
	if !ok || projected.Revision() != policy.Revision() ||
		projected.Digest() != policy.Digest() {
		return ProviderAccountPolicyResult{}, ErrProviderAccountPolicyUnavailable
	}
	return providerAccountPolicyResult(policy), nil
}

func providerAccountPolicyResult(
	policy work.ProviderAccountPolicy,
) ProviderAccountPolicyResult {
	return ProviderAccountPolicyResult{
		PolicyAvailable:            true,
		PolicyVersion:              policy.Version(),
		ProviderID:                 policy.ProviderID(),
		ProviderAccountID:          policy.ProviderAccountID(),
		Revision:                   policy.Revision(),
		PolicyDigest:               policy.Digest(),
		MaximumConcurrentAttempts:  policy.MaximumConcurrentAttempts(),
		DispatchWindowSeconds:      int64(policy.DispatchWindow() / time.Second),
		MaximumDispatchStarts:      policy.MaximumDispatchStarts(),
		MaximumAssignedBudgetUnits: policy.MaximumAssignedBudgetUnits(),
		TrustDomain:                policy.TrustDomain(),
		RetentionMode:              policy.RetentionMode(),
		DataRegion:                 policy.DataRegion(),
		ConfiguredAt:               policy.ConfiguredAt().Format(time.RFC3339Nano),
	}
}
