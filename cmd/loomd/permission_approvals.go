package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/rules"
)

// permissionApprovalPort adapts the internal/rules authority for permission
// asks. It is the daemon's only rules consumer today, so its authorizer grants
// permission-scoped activations and decisions with the permission actor; the
// human gate is enforced upstream by the user command routing (resolvedBy).
type permissionApprovalPort struct {
	authority *rules.Authority
	now       func() time.Time
}

func newPermissionApprovalPort(store *journal.Store, now func() time.Time) (*permissionApprovalPort, error) {
	authority, err := rules.NewAuthority(store, &permissionAuthorizer{now: now}, now)
	if err != nil {
		return nil, err
	}
	return &permissionApprovalPort{authority: authority, now: now}, nil
}

func (port *permissionApprovalPort) RequestPermissionApproval(
	ctx context.Context,
	input rules.PermissionApprovalInput,
) (rules.ApprovalRequestRecord, error) {
	return port.authority.RequestPermissionApproval(ctx, input)
}

func (port *permissionApprovalPort) DecidePermissionApproval(
	ctx context.Context,
	approvalID, approvalDigest, decision, resolvedBy, correlationID string,
) (rules.ApprovalRequestRecord, error) {
	return port.authority.DecidePermissionApproval(
		ctx, approvalID, approvalDigest, decision, resolvedBy, correlationID,
	)
}

type permissionAuthorizer struct {
	now func() time.Time
}

func (authorizer *permissionAuthorizer) AuthorizeRuleSet(
	ctx context.Context,
	request rules.RuleSetActivationRequest,
) (rules.AuthorizedRuleSetActivation, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedRuleSetActivation{}, err
	}
	digest := sha256.Sum256([]byte("permission-rule-auth"))
	now := authorizer.now()
	return rules.NewAuthorizedRuleSetActivation(
		request,
		permissionApproverActor,
		fmt.Sprintf("%x", digest[:]),
		now,
		now.Add(time.Hour),
	)
}

func (authorizer *permissionAuthorizer) AuthorizeApprovalDecision(
	ctx context.Context,
	request rules.ApprovalDecisionRequest,
) (rules.AuthorizedApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedApprovalDecision{}, err
	}
	digest := sha256.Sum256([]byte("permission-decision-auth"))
	now := authorizer.now()
	return rules.NewAuthorizedApprovalDecision(
		request,
		permissionApproverActor,
		fmt.Sprintf("%x", digest[:]),
		now,
		now.Add(time.Hour),
	)
}

const permissionApproverActor = "approver:permission-owner"
