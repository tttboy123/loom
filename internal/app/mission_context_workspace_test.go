package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestMissionCompilerDisclosesContentAddressedObservedWorkspaceSnapshot(t *testing.T) {
	command := missionExecutionTestCommand("preflight")
	command.ConfirmedConstraints = []string{"Do not change public APIs"}
	command.AcceptedDecisions = []string{"Use the existing execution adapter"}
	now := time.Date(2026, 8, 12, 1, 30, 0, 0, time.UTC)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "loom-main-native", AdapterType: "pi-cli", ProviderID: "local",
		ModelID: "qwen2.5-coder-1.5b-instruct-q4-k-m", AuthMode: loomruntime.AuthNative,
		RequiredCapabilities: []string{"models"}, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime-pi", DeviceID: "device-local", AdapterType: "pi-cli",
		DisplayName: "Local Pi", ExecutableVersion: "0.82.1",
		Status: loomruntime.RuntimeOnline, ObservedCapabilities: []string{"models"}, Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourcePath, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(sourcePath, "internal"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "internal", "logic.go"), []byte("package internal\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := &controlledMissionExecutionBindingSource{binding: MissionExecutionBinding{
		ViewVersion: command.ExpectedViewVersion, TeamInstanceID: command.TeamInstanceID,
		AgentInstanceID: "agent-main", Profile: profile, Instance: instance,
		CapacityAvailable: 1,
	}}
	compiler, err := NewBuiltInMissionExecutionCompiler(BuiltInMissionExecutionCompilerConfig{
		Bindings: source, SourcePath: sourcePath, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	compilation, err := compiler.CompileMissionExecution(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	primary := compilation.Request.Nodes[0]
	dispatch, err := contextcapsule.ValidateDispatchPayload(
		primary.ContextCapsule, primary.Dispatch.Payload(),
	)
	if err != nil {
		t.Fatal(err)
	}
	var prompt struct {
		Items []struct {
			ItemID     string                    `json:"item_id"`
			Kind       contextcapsule.ItemKind   `json:"kind"`
			Trust      contextcapsule.TrustClass `json:"trust"`
			Scope      contextcapsule.Scope      `json:"scope"`
			Content    string                    `json:"content"`
			SourceType contextcapsule.SourceType `json:"source_type"`
			SourceRef  string                    `json:"source_ref"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(dispatch.Prompt), &prompt); err != nil {
		t.Fatal(err)
	}
	var workspaceContent string
	var workspaceSourceRef string
	authorityContext := make(map[contextcapsule.ItemKind]string)
	for _, item := range prompt.Items {
		if item.Kind == contextcapsule.KindConfirmedConstraint ||
			item.Kind == contextcapsule.KindAcceptedDecision {
			authorityContext[item.Kind] = item.Content
		}
		if item.ItemID != "workspace-snapshot" {
			continue
		}
		if item.Kind != contextcapsule.KindWorkspaceSnapshot ||
			item.Trust != contextcapsule.TrustObserved ||
			item.SourceType != contextcapsule.SourceObservation ||
			item.Scope != contextcapsule.ScopeTeamShared {
			t.Fatalf("workspace item = %#v", item)
		}
		workspaceContent = item.Content
		workspaceSourceRef = item.SourceRef
	}
	if authorityContext[contextcapsule.KindConfirmedConstraint] !=
		"Do not change public APIs" ||
		authorityContext[contextcapsule.KindAcceptedDecision] !=
			"Use the existing execution adapter" {
		t.Fatalf("canonical mission context = %#v", authorityContext)
	}
	var snapshot struct {
		SchemaVersion  int    `json:"schema_version"`
		SnapshotKind   string `json:"snapshot_kind"`
		TreeDigest     string `json:"tree_digest"`
		EntryCount     int    `json:"entry_count"`
		FileCount      int    `json:"file_count"`
		DirectoryCount int    `json:"directory_count"`
		TotalBytes     int64  `json:"total_bytes"`
	}
	if workspaceContent == "" {
		t.Fatal("workspace snapshot is missing")
	}
	if err := json.Unmarshal([]byte(workspaceContent), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != 1 || snapshot.SnapshotKind != "managed_source_baseline" ||
		len(snapshot.TreeDigest) != 64 ||
		snapshot.EntryCount != 3 || snapshot.FileCount != 2 ||
		snapshot.DirectoryCount != 1 || snapshot.TotalBytes != 30 ||
		workspaceSourceRef != "managed-source:"+snapshot.TreeDigest {
		t.Fatalf("workspace snapshot = %#v, source=%q", snapshot, workspaceSourceRef)
	}
	if strings.Contains(dispatch.Prompt, sourcePath) ||
		strings.Contains(dispatch.Prompt, "package main") ||
		strings.Contains(dispatch.Prompt, "package internal") {
		t.Fatalf("workspace snapshot leaked path or content: %s", dispatch.Prompt)
	}
	if compilation.Request.Nodes[1].ContextCapsule.Digest() != primary.ContextCapsule.Digest() {
		t.Fatal("ordinary retry changed the observed workspace snapshot capsule")
	}
	if _, err := validateTeamExecutionRequest(context.Background(), compilation.Request); err != nil {
		t.Fatalf("workspace-bound request rejected: %v", err)
	}
	forged := compilation.Request
	forged.Nodes = append([]TeamNodeExecution{}, compilation.Request.Nodes...)
	forged.Nodes[0].SourceSnapshotDigest = strings.Repeat("f", 64)
	if _, err := validateTeamExecutionRequest(context.Background(), forged); err == nil {
		t.Fatal("workspace digest substitution was accepted")
	}
}

func TestMissionRecoveryRestoresFrozenContextInsteadOfRecompilingSource(t *testing.T) {
	command := missionExecutionTestCommand("start")
	command.PreflightDigest = strings.Repeat("e", 64)
	now := time.Date(2026, 8, 12, 2, 30, 0, 0, time.UTC)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "loom-main-native", AdapterType: "pi-cli", ProviderID: "local",
		ModelID: "qwen2.5-coder-1.5b-instruct-q4-k-m", AuthMode: loomruntime.AuthNative,
		RequiredCapabilities: []string{"models"}, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime-pi", DeviceID: "device-local", AdapterType: "pi-cli",
		DisplayName: "Local Pi", ExecutableVersion: "0.82.1",
		Status: loomruntime.RuntimeOnline, ObservedCapabilities: []string{"models"}, Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := t.TempDir()
	sourceFile := filepath.Join(sourcePath, "main.go")
	if err := os.WriteFile(sourceFile, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := &controlledMissionExecutionBindingSource{binding: MissionExecutionBinding{
		ViewVersion: command.ExpectedViewVersion, TeamInstanceID: command.TeamInstanceID,
		AgentInstanceID: "agent-main", Profile: profile, Instance: instance,
		CapacityAvailable: 1,
	}}
	capsuleStore := &controlledMissionContextCapsuleStore{}
	compiler, err := NewBuiltInMissionExecutionCompiler(BuiltInMissionExecutionCompilerConfig{
		Bindings: source, SourcePath: sourcePath, ContextCapsules: capsuleStore,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	compilation, err := compiler.CompileMissionExecution(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	primary := compilation.Request.Nodes[0]
	originalPayload := primary.Dispatch.Payload()
	originalDigest := primary.ContextCapsule.Digest()
	originalSourceDigest := primary.SourceSnapshotDigest
	storedCount := len(capsuleStore.records)
	semantics := compilation.Request.Semantics[0]
	frozenBinding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	projected := projection.TeamExecution{
		TeamInstanceID: command.TeamInstanceID,
		PlanDigest:     compilation.Plan.Digest(),
		Status:         "running",
		Nodes: []projection.TeamExecutionNode{{
			LogicalNodeID: "main", Title: command.Objective,
			AgentInstanceID:   source.binding.AgentInstanceID,
			RuntimeInstanceID: source.binding.Instance.ID, Role: "main",
			DependsOn: []string{}, MaxAttempts: 2, Status: "running", CurrentAttempt: 1,
			Attempts: []projection.TeamExecutionAttempt{{
				AttemptNumber: 1, ContextCapsuleAvailable: true,
				ContextCapsule:            primary.ContextCapsule.AuthorityRecord(),
				ExecutionBindingAvailable: true,
				ExecutionBinding:          frozenBinding,
			}},
			OutputContractVersion:       semantics.OutputContract.Version(),
			OutputContractDigest:        semantics.OutputContract.Digest(),
			RecoveryPolicyVersion:       semantics.RecoveryPolicy.Version(),
			RecoveryPolicyDigest:        semantics.RecoveryPolicy.Digest(),
			AttemptCredits:              semantics.RecoveryPolicy.AttemptCredits(),
			PrimaryWorkflowPath:         semantics.PrimaryWorkflowPath,
			WorkflowFallbackKey:         semantics.RecoveryPolicy.WorkflowFallbackKey(),
			RecoveryApprovalRequired:    semantics.RecoveryPolicy.RecoveryApprovalRequired(),
			AcceptanceContractVersion:   semantics.AcceptanceContract.Version(),
			AcceptanceContractDigest:    semantics.AcceptanceContract.Digest(),
			AcceptanceRisk:              string(semantics.AcceptanceContract.Risk()),
			IndependentVerifierRequired: semantics.AcceptanceContract.IndependentVerifierRequired(),
			VerifierAgentInstanceID:     semantics.VerifierAgentInstanceID,
			VerifierRuntimeInstanceID:   semantics.VerifierRuntimeInstanceID,
			VerifierWorkflowPath:        semantics.VerifierWorkflowPath,
		}},
	}

	if err := os.WriteFile(sourceFile, []byte("package main\n// changed after dispatch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recovered, err := compiler.ReconstructMissionExecution(context.Background(), projected)
	if err != nil {
		t.Fatal(err)
	}
	if len(capsuleStore.records) != storedCount {
		t.Fatalf("recovery persisted %d new capsules", len(capsuleStore.records)-storedCount)
	}
	for _, execution := range recovered.Nodes {
		if execution.ContextCapsule.Digest() != originalDigest ||
			execution.SourceSnapshotDigest != originalSourceDigest ||
			!bytes.Equal(execution.Dispatch.Payload(), originalPayload) {
			t.Fatalf("recovered execution drifted = %#v", execution)
		}
	}

	capsuleStore.records = nil
	if _, err := compiler.ReconstructMissionExecution(
		context.Background(), projected,
	); !errors.Is(err, ErrMissionExecutionConflict) {
		t.Fatalf("missing frozen capsule recovery error = %v", err)
	}
}

func TestMissionRecoveryRejectsFrozenExecutionBindingDrift(t *testing.T) {
	command := missionExecutionTestCommand("start")
	command.PreflightDigest = strings.Repeat("e", 64)
	now := time.Date(2026, 8, 12, 3, 0, 0, 0, time.UTC)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "deepseek-main", AdapterType: "loom-native", ProviderID: "deepseek",
		ProviderAccountID: "deepseek.primary", ModelID: "deepseek-chat",
		AuthMode:            loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("a", 64),
		CredentialReference: "credential-ref-deepseek", CredentialRevision: 3,
		RequiredCapabilities: []string{"chat"}, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime-deepseek", DeviceID: "device-local", AdapterType: "loom-native",
		DisplayName: "DeepSeek", ExecutableVersion: "1.0.0",
		Status: loomruntime.RuntimeOnline, ObservedCapabilities: []string{"chat"}, Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	source := &controlledMissionExecutionBindingSource{binding: MissionExecutionBinding{
		ViewVersion: command.ExpectedViewVersion, TeamInstanceID: command.TeamInstanceID,
		AgentInstanceID: "agent-main", Profile: profile, Instance: instance,
		CapacityAvailable: 1,
	}}
	store := &controlledMissionContextCapsuleStore{}
	compiler, err := NewBuiltInMissionExecutionCompiler(BuiltInMissionExecutionCompilerConfig{
		Bindings: source, SourcePath: t.TempDir(), ContextCapsules: store,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	compilation, err := compiler.CompileMissionExecution(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	primary := compilation.Request.Nodes[0]
	semantics := compilation.Request.Semantics[0]
	frozen, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	projected := projection.TeamExecution{
		TeamInstanceID: command.TeamInstanceID, PlanDigest: compilation.Plan.Digest(),
		Status: "running", Nodes: []projection.TeamExecutionNode{{
			LogicalNodeID: "main", Title: command.Objective,
			AgentInstanceID: "agent-main", RuntimeInstanceID: instance.ID,
			Role: "main", DependsOn: []string{}, MaxAttempts: 2, Status: "running",
			CurrentAttempt: 1, Attempts: []projection.TeamExecutionAttempt{{
				AttemptNumber: 1, ExecutionBindingAvailable: true,
				ExecutionBinding: frozen, ContextCapsuleAvailable: true,
				ContextCapsule: primary.ContextCapsule.AuthorityRecord(),
			}},
			OutputContractVersion:       semantics.OutputContract.Version(),
			OutputContractDigest:        semantics.OutputContract.Digest(),
			RecoveryPolicyVersion:       semantics.RecoveryPolicy.Version(),
			RecoveryPolicyDigest:        semantics.RecoveryPolicy.Digest(),
			AttemptCredits:              semantics.RecoveryPolicy.AttemptCredits(),
			PrimaryWorkflowPath:         semantics.PrimaryWorkflowPath,
			WorkflowFallbackKey:         semantics.RecoveryPolicy.WorkflowFallbackKey(),
			RecoveryApprovalRequired:    semantics.RecoveryPolicy.RecoveryApprovalRequired(),
			AcceptanceContractVersion:   semantics.AcceptanceContract.Version(),
			AcceptanceContractDigest:    semantics.AcceptanceContract.Digest(),
			AcceptanceRisk:              string(semantics.AcceptanceContract.Risk()),
			IndependentVerifierRequired: semantics.AcceptanceContract.IndependentVerifierRequired(),
			VerifierAgentInstanceID:     semantics.VerifierAgentInstanceID,
			VerifierRuntimeInstanceID:   semantics.VerifierRuntimeInstanceID,
			VerifierWorkflowPath:        semantics.VerifierWorkflowPath,
		}},
	}

	drifted := profile
	drifted.CredentialRevision++
	drifted.EndpointFingerprint = strings.Repeat("b", 64)
	source.binding.Profile = drifted
	if _, err := compiler.ReconstructMissionExecution(
		context.Background(), projected,
	); !errors.Is(err, ErrMissionExecutionConflict) {
		t.Fatalf("credential binding drift error = %v", err)
	}
}
