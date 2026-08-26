package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/work"
)

var errProductMissionToolPermission = errors.New("Mission tool permission unavailable")

type productMissionToolPermissionPort interface {
	EnsureMissionToolPermissions(
		context.Context,
		string,
		string,
		loomruntime.FrozenExecutionBinding,
	) error
}

type productMissionToolPermissionProvisioner struct {
	store      *journal.Store
	projection *projection.Projection
	authority  *permissions.Authority
	now        func() time.Time

	mu sync.Mutex
}

func newProductMissionToolPermissionProvisioner(
	store *journal.Store,
	readModel *projection.Projection,
	now func() time.Time,
) (*productMissionToolPermissionProvisioner, error) {
	if store == nil || readModel == nil || now == nil {
		return nil, errProductMissionToolPermission
	}
	current := now()
	if current.IsZero() || current.Location() != time.UTC {
		return nil, errProductMissionToolPermission
	}
	authority, err := permissions.NewAuthority(store, now)
	if err != nil {
		return nil, errors.Join(errProductMissionToolPermission, err)
	}
	return &productMissionToolPermissionProvisioner{
		store: store, projection: readModel, authority: authority, now: now,
	}, nil
}

func (provisioner *productMissionToolPermissionProvisioner) EnsureMissionToolPermissions(
	ctx context.Context,
	workItemID string,
	correlationID string,
	binding loomruntime.FrozenExecutionBinding,
) error {
	if provisioner == nil || provisioner.store == nil || provisioner.projection == nil ||
		provisioner.authority == nil || ctx == nil || workItemID == "" || correlationID == "" {
		return errProductMissionToolPermission
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	validated, err := loomruntime.ValidateFrozenExecutionBinding(binding)
	if err != nil {
		return errors.Join(errProductMissionToolPermission, err)
	}
	provisioner.mu.Lock()
	defer provisioner.mu.Unlock()
	if err := provisioner.projection.Rebuild(ctx); err != nil {
		return errors.Join(errProductMissionToolPermission, err)
	}
	view := provisioner.projection.GlobalReadView()
	var enrollment work.RemoteToolBackendEnrollment
	if validated.RemoteToolEnrollmentID != "" {
		enrollment, err = exactMissionToolEnrollment(view, validated)
		if err != nil {
			return err
		}
		if enrollment.BackendKind() != work.RemoteToolBackendMCPServer {
			return errProductMissionToolPermission
		}
	}
	profileInput, err := missionToolPermissionProfileInput(workItemID, enrollment)
	if err != nil {
		return err
	}
	compiled, err := permissions.CompileProfile(profileInput)
	if err != nil {
		return errors.Join(errProductMissionToolPermission, err)
	}
	events, err := provisioner.store.ReadAll(ctx)
	if err != nil {
		return errors.Join(errProductMissionToolPermission, err)
	}
	permissionView, err := permissions.Replay(events)
	if err != nil {
		return errors.Join(errProductMissionToolPermission, err)
	}
	if existing, found := permissionView.Profiles[compiled.ProfileID]; found {
		if !reflect.DeepEqual(existing, compiled) {
			return errProductMissionToolPermission
		}
	} else {
		if _, err := provisioner.authority.DefineProfile(
			ctx,
			profileInput,
			"mission-tool-profile-"+productDeterministicUUID(
				workItemID, enrollment.Digest(),
			),
			correlationID,
		); err != nil {
			return errors.Join(errProductMissionToolPermission, err)
		}
	}
	if err := provisioner.bindMissionToolPermissionProfile(
		ctx, workItemID, correlationID, compiled,
	); err != nil {
		return err
	}
	return nil
}

func exactMissionToolEnrollment(
	view projection.GlobalReadView,
	binding loomruntime.FrozenExecutionBinding,
) (work.RemoteToolBackendEnrollment, error) {
	for _, candidate := range view.RemoteToolBackendEnrollmentCatalog() {
		if candidate.EnrollmentID() != binding.RemoteToolEnrollmentID {
			continue
		}
		policy, policyFound := view.ProviderAccountPolicy(
			candidate.ProviderID(), candidate.ProviderAccountID(),
		)
		if candidate.Digest() != binding.RemoteToolEnrollmentDigest ||
			candidate.ProviderID() != binding.ProviderID ||
			candidate.ProviderAccountID() != binding.ProviderAccountID ||
			candidate.Status() != work.RemoteToolBackendEnrollmentActive ||
			!policyFound || !remoteToolEnrollmentPolicyCurrent(candidate, policy) ||
			candidate.BackendKind() == work.RemoteToolBackendMCPServer &&
				(candidate.AdapterID() != work.BuiltInMCPAdapterID ||
					candidate.MCPServerID() == "" || len(candidate.AllowedTools()) == 0) {
			return work.RemoteToolBackendEnrollment{}, errProductMissionToolPermission
		}
		return candidate, nil
	}
	return work.RemoteToolBackendEnrollment{}, errProductMissionToolPermission
}

func missionToolPermissionProfileInput(
	workItemID string,
	enrollment work.RemoteToolBackendEnrollment,
) (permissions.ProfileInput, error) {
	if workItemID == "" {
		return permissions.ProfileInput{}, errProductMissionToolPermission
	}
	profileDomain := "workspace"
	rules := []permissions.Rule{}
	if enrollment.Valid() {
		if enrollment.BackendKind() != work.RemoteToolBackendMCPServer ||
			enrollment.MCPServerID() == "" || len(enrollment.AllowedTools()) == 0 {
			return permissions.ProfileInput{}, errProductMissionToolPermission
		}
		profileDomain = enrollment.Digest()
		rules = make([]permissions.Rule, len(enrollment.AllowedTools()))
		for index, tool := range enrollment.AllowedTools() {
			if tool == "" {
				return permissions.ProfileInput{}, errProductMissionToolPermission
			}
			rules[index] = permissions.Rule{
				RuleID: "mission-tool-rule-" + productDeterministicUUID(
					workItemID, enrollment.Digest(), strconv.Itoa(index),
				),
				Scope: permissions.ScopeJob, ScopeID: workItemID,
				Action: permissions.ActionAllow, Tool: permissions.ToolMCPTool,
				Pattern: fmt.Sprintf("%s/%s", enrollment.MCPServerID(), tool),
			}
		}
	} else if enrollment.EnrollmentID() != "" {
		return permissions.ProfileInput{}, errProductMissionToolPermission
	}
	return permissions.ProfileInput{
		ProfileID: "mission-tool-" + productDeterministicUUID(workItemID, profileDomain),
		Mode:      permissions.ModeAuto, Rules: rules, OwnedPaths: []string{"**"},
	}, nil
}

func (provisioner *productMissionToolPermissionProvisioner) bindMissionToolPermissionProfile(
	ctx context.Context,
	workItemID string,
	correlationID string,
	profile permissions.PermissionProfile,
) error {
	events, err := provisioner.store.ReadAll(ctx)
	if err != nil {
		return errors.Join(errProductMissionToolPermission, err)
	}
	permissionView, err := permissions.Replay(events)
	if err != nil {
		return errors.Join(errProductMissionToolPermission, err)
	}
	if existing, found := permissionView.Bindings[workItemID]; found {
		if existing.ProfileID != profile.ProfileID ||
			existing.ProfileGeneration != profile.Generation ||
			existing.ProfileDigest != profile.Digest {
			return errProductMissionToolPermission
		}
		return nil
	}
	if _, err := provisioner.authority.BindJob(
		ctx,
		workItemID,
		profile.ProfileID,
		"mission-tool-bind-"+productDeterministicUUID(
			workItemID, profile.Digest,
		),
		correlationID,
	); err != nil {
		return errors.Join(errProductMissionToolPermission, err)
	}
	return nil
}
