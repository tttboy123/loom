package main

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/work"
)

func TestProductMissionToolPermissionsFreezeExactMCPEnrollment(t *testing.T) {
	ctx := context.Background()
	database := openProductEnrollmentDB(t)
	store := journal.NewStore(database)
	now := time.Unix(3_401, 0).UTC()
	workAuthority, err := work.NewAuthority(
		store,
		func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0xe2}, 4_096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	policy := configureProductEnrollmentPolicy(
		t, workAuthority, "deepseek", "deepseek.primary", 0,
	)
	enrollment, err := workAuthority.ConfigureRemoteToolBackendEnrollment(
		ctx,
		work.RemoteToolBackendEnrollmentCommand{
			CommandID: "configure-enr-mcp-permission", EnrollmentID: "enr-mcp-permission",
			BackendKind: work.RemoteToolBackendMCPServer,
			AdapterID:   work.BuiltInMCPAdapterID,
			ProviderID:  policy.ProviderID(), ProviderAccountID: policy.ProviderAccountID(),
			ProviderAccountPolicyVersion:  policy.Version(),
			ProviderAccountPolicyRevision: policy.Revision(),
			ProviderAccountPolicyDigest:   policy.Digest(),
			EndpointFingerprint:           strings.Repeat("e", 64),
			MCPServerID:                   "mcp.live.probe", AllowedTools: []string{"lookup", "read_status"},
			MaximumConcurrentCalls: 2, MaximumCallsPerAttempt: 4,
			Timeout: 30 * time.Second, MaximumResultBytes: 1 << 16,
			MaximumBudgetUnits: 2_000, CorrelationID: "mcp-permission-configure",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(database)
	provisioner, err := newProductMissionToolPermissionProvisioner(
		store, readModel, func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	binding := productMissionToolPermissionBinding(t, enrollment)
	if err := provisioner.EnsureMissionToolPermissions(
		ctx, "work-mcp-permission", "mcp-permission-correlation", binding,
	); err != nil {
		t.Fatal(err)
	}
	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	firstCount := len(events)
	if err := provisioner.EnsureMissionToolPermissions(
		ctx, "work-mcp-permission", "mcp-permission-correlation", binding,
	); err != nil {
		t.Fatal(err)
	}
	events, err = store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != firstCount {
		t.Fatalf("idempotent ensure appended events: before=%d after=%d", firstCount, len(events))
	}
	permissionView, err := permissions.Replay(events)
	if err != nil {
		t.Fatal(err)
	}
	effective, err := permissions.ResolveEffectiveProfile(permissionView, "work-mcp-permission")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"mcp.live.probe/lookup", "mcp.live.probe/read_status"} {
		verdict, denial, err := permissions.Evaluate(effective, permissions.ProposedCall{
			Tool: permissions.ToolMCPTool, Path: path, Command: `{}`,
		})
		if err != nil || verdict != permissions.VerdictAllow || !reflect.DeepEqual(denial, permissions.Denial{}) {
			t.Fatalf("exact MCP path %q verdict=%q denial=%#v err=%v", path, verdict, denial, err)
		}
	}
	verdict, _, err := permissions.Evaluate(effective, permissions.ProposedCall{
		Tool: permissions.ToolMCPTool, Path: "mcp.live.probe/delete_all", Command: `{}`,
	})
	if err != nil || verdict != permissions.VerdictAsk {
		t.Fatalf("undeclared MCP path verdict=%q err=%v", verdict, err)
	}
	peer, err := permissions.ResolveEffectiveProfile(permissionView, "work-unbound-peer")
	if err != nil {
		t.Fatal(err)
	}
	verdict, _, err = permissions.Evaluate(peer, permissions.ProposedCall{
		Tool: permissions.ToolMCPTool, Path: "mcp.live.probe/lookup", Command: `{}`,
	})
	if err != nil || verdict != permissions.VerdictAsk {
		t.Fatalf("unbound peer verdict=%q err=%v", verdict, err)
	}
}

func TestProductMissionToolPermissionsAuthorizeBoundedWorkspaceEditAndTests(t *testing.T) {
	ctx := context.Background()
	database := openProductEnrollmentDB(t)
	store := journal.NewStore(database)
	now := time.Unix(3_451, 0).UTC()
	readModel := projection.New(database)
	provisioner, err := newProductMissionToolPermissionProvisioner(
		store, readModel, func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	profile := loomruntime.RuntimeProfile{
		ID: "profile-minimax", AdapterType: "opencode",
		ProviderID: "minimax", ProviderAccountID: "minimax.primary",
		ModelID: "minimax-cn/MiniMax-M3", AuthMode: loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("e", 64),
		CredentialReference: "credential-ref-minimax", CredentialRevision: 23,
		Timeout: time.Minute, RequiredCapabilities: []string{
			loomruntime.CapabilityGovernedToolLoop, "workspace_edit",
		},
	}
	instance := loomruntime.RuntimeInstance{
		ID: "runtime.opencode.local", DeviceID: "device.local", AdapterType: "opencode",
		DisplayName: "OpenCode", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{
			loomruntime.CapabilityGovernedToolLoop, "workspace_edit",
		}, Capacity: 1,
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	if err := provisioner.EnsureMissionToolPermissions(
		ctx, "work-minimax-snake", "mission-minimax-snake", binding,
	); err != nil {
		t.Fatal(err)
	}
	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	permissionView, err := permissions.Replay(events)
	if err != nil {
		t.Fatal(err)
	}
	effective, err := permissions.ResolveEffectiveProfile(permissionView, "work-minimax-snake")
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []permissions.ProposedCall{
		{Tool: permissions.ToolEdit, Path: "index.html", Command: "<main>Snake</main>"},
		{Tool: permissions.ToolBash, Command: "node tests.js"},
	} {
		verdict, _, evalErr := permissions.Evaluate(effective, call)
		if evalErr != nil || verdict != permissions.VerdictAllow {
			t.Fatalf("call %#v verdict=%q err=%v", call, verdict, evalErr)
		}
	}
	verdict, _, err := permissions.Evaluate(effective, permissions.ProposedCall{
		Tool: permissions.ToolBash, Command: "rm -rf .",
	})
	if err != nil || verdict != permissions.VerdictAsk {
		t.Fatalf("dangerous command verdict=%q err=%v", verdict, err)
	}
}

func TestProductMissionToolPermissionsRejectDriftAndRevocation(t *testing.T) {
	ctx := context.Background()
	database := openProductEnrollmentDB(t)
	store := journal.NewStore(database)
	now := time.Unix(3_501, 0).UTC()
	workAuthority, err := work.NewAuthority(
		store,
		func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0xf2}, 4_096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	policy := configureProductEnrollmentPolicy(
		t, workAuthority, "deepseek", "deepseek.primary", 0,
	)
	enrollment, err := workAuthority.ConfigureRemoteToolBackendEnrollment(
		ctx,
		work.RemoteToolBackendEnrollmentCommand{
			CommandID: "configure-enr-mcp-revoke", EnrollmentID: "enr-mcp-revoke",
			BackendKind: work.RemoteToolBackendMCPServer, AdapterID: work.BuiltInMCPAdapterID,
			ProviderID: policy.ProviderID(), ProviderAccountID: policy.ProviderAccountID(),
			ProviderAccountPolicyVersion:  policy.Version(),
			ProviderAccountPolicyRevision: policy.Revision(),
			ProviderAccountPolicyDigest:   policy.Digest(),
			EndpointFingerprint:           strings.Repeat("e", 64), MCPServerID: "mcp.live.probe",
			AllowedTools: []string{"lookup"}, MaximumConcurrentCalls: 1,
			MaximumCallsPerAttempt: 1, Timeout: 30 * time.Second,
			MaximumResultBytes: 4096, MaximumBudgetUnits: 100,
			CorrelationID: "mcp-revoke-configure",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(database)
	provisioner, err := newProductMissionToolPermissionProvisioner(
		store, readModel, func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	drifted := productMissionToolPermissionBinding(t, enrollment)
	drifted.RemoteToolEnrollmentDigest = strings.Repeat("f", 64)
	if err := provisioner.EnsureMissionToolPermissions(
		ctx, "work-mcp-drift", "mcp-drift-correlation", drifted,
	); err == nil {
		t.Fatal("drifted frozen binding received MCP permission")
	}
	now = now.Add(time.Minute)
	if _, err := workAuthority.RevokeRemoteToolBackendEnrollment(
		ctx,
		work.RemoteToolBackendEnrollmentRevokeCommand{
			CommandID: "revoke-enr-mcp-revoke", EnrollmentID: enrollment.EnrollmentID(),
			ProviderID: enrollment.ProviderID(), ProviderAccountID: enrollment.ProviderAccountID(),
			ExpectedRevision: enrollment.Revision(), CorrelationID: "mcp-revoke-correlation",
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := provisioner.EnsureMissionToolPermissions(
		ctx, "work-mcp-revoked", "mcp-revoke-correlation",
		productMissionToolPermissionBinding(t, enrollment),
	); err == nil {
		t.Fatal("revoked Enrollment received MCP permission")
	}
}

func productMissionToolPermissionBinding(
	t *testing.T,
	enrollment work.RemoteToolBackendEnrollment,
) loomruntime.FrozenExecutionBinding {
	t.Helper()
	profile := loomruntime.RuntimeProfile{
		ID: "profile-mcp-permission", AdapterType: "opencode-agent",
		ProviderID: enrollment.ProviderID(), ProviderAccountID: enrollment.ProviderAccountID(),
		ModelID: "deepseek-chat", AuthMode: loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("e", 64),
		CredentialReference: "credential-ref-mcp-permission", CredentialRevision: 3,
		RequiredCapabilities: []string{loomruntime.CapabilityGovernedToolLoop},
		Timeout:              90 * time.Second, RemoteToolEnrollmentID: enrollment.EnrollmentID(),
		RemoteToolEnrollmentDigest: enrollment.Digest(),
	}
	instance := loomruntime.RuntimeInstance{
		ID: "runtime-opencode-local", DeviceID: "device.local",
		AdapterType: "opencode-agent", DisplayName: "OpenCode", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{loomruntime.CapabilityGovernedToolLoop}, Capacity: 1,
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}
