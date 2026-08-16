package projection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/work"
)

func TestProviderAccountCapacityProjectionFreezesRunPolicyAndPreservesPriorView(t *testing.T) {
	database, store := openLocalProductSetupProjectionStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 11, 11, 0, 0, 0, time.UTC)
	appendProviderAccountCapacityRuntime(t, store, now)
	authority, err := work.NewAuthority(
		store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x81}, 4096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	policy, err := authority.ConfigureProviderAccountPolicy(
		ctx, work.ProviderAccountPolicyCommand{
			CommandID:  "configure-deepseek-work-v1",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			ExpectedRevision: 0, MaximumConcurrentAttempts: 2,
			DispatchWindow: time.Minute, MaximumDispatchStarts: 10,
			MaximumAssignedBudgetUnits: 100,
			TrustDomain:                "external_provider", RetentionMode: "limited_retention",
			DataRegion:    "apac",
			CorrelationID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	binding := providerAccountCapacityProjectionBinding(t, 60)
	if _, _, err := authority.CreateAndAssign(
		ctx, work.WorkItemAssignmentInput{
			WorkItemID: "work-account-projection", Title: "Account projection",
			RunID: "run-account-projection", AgentInstanceID: "agent-account-projection",
			ExecutionBinding: binding,
			CorrelationID:    "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		},
	); err != nil {
		t.Fatal(err)
	}
	_, claimed, err := authority.Claim(ctx, work.RunClaimInput{
		WorkItemID: "work-account-projection", RunID: "run-account-projection",
		RuntimeInstanceID:    "runtime-account-projection",
		AgentInstanceID:      "agent-account-projection",
		PrepareLeaseDuration: time.Minute,
		CorrelationID:        "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
	})
	if err != nil {
		t.Fatal(err)
	}

	readModel := New(database)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	run, ok := readModel.GlobalReadView().Run("run-account-projection")
	if !ok || !run.ProviderAccountPolicyAvailable ||
		run.ProviderAccountPolicyVersion != policy.Version() ||
		run.ProviderAccountPolicyRevision != policy.Revision() ||
		run.ProviderAccountPolicyDigest != policy.Digest() ||
		run.ProviderAccountTrustDomain != policy.TrustDomain() ||
		run.ProviderAccountRetentionMode != policy.RetentionMode() ||
		run.ProviderAccountDataRegion != policy.DataRegion() ||
		run.ProviderAccountAssignedBudgetUnits != 60 {
		t.Fatalf("projected Run policy = %#v, ok=%t", run, ok)
	}

	if _, _, err := authority.CommitTerminal(ctx, work.RunTerminalInput{
		RunGenerationInput: work.RunGenerationInput{
			WorkItemID: claimed.WorkItemID(), RunID: claimed.ID(),
			ClaimID: claimed.ClaimID(), ClaimGeneration: claimed.ClaimGeneration(),
			RuntimeInstanceID: claimed.RuntimeInstanceID(),
			AgentInstanceID:   claimed.AgentInstanceID(),
			CorrelationID:     "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		},
		Status: "cancelled", Reason: "operator_cancelled",
	}); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	accepted := readModel.GlobalReadView()
	terminal, ok := accepted.Run("run-account-projection")
	if !ok || terminal.TerminalStatus != "cancelled" ||
		!terminal.ProviderAccountPolicyAvailable ||
		terminal.ProviderAccountPolicyDigest != policy.Digest() ||
		terminal.ProviderAccountTrustDomain != "external_provider" ||
		terminal.ProviderAccountRetentionMode != "limited_retention" ||
		terminal.ProviderAccountDataRegion != "apac" {
		t.Fatalf("terminal Run = %#v, ok=%t", terminal, ok)
	}

	events, err := store.ReadStream(
		ctx, "provider-account-capacity/deepseek.work",
	)
	if err != nil || len(events) != 2 {
		t.Fatalf("capacity events = %d, err=%v", len(events), err)
	}
	badPayload, err := json.Marshal(map[string]any{
		"work_item_id": "work-account-projection",
		"run_id":       "run-account-projection", "claim_id": claimed.ClaimID(),
		"claim_generation":    claimed.ClaimGeneration(),
		"runtime_instance_id": claimed.RuntimeInstanceID(),
		"agent_instance_id":   claimed.AgentInstanceID(),
		"provider_id":         "deepseek", "provider_account_id": "deepseek.work",
		"policy_revision":          policy.Revision(),
		"policy_digest":            strings.Repeat("f", 64),
		"execution_binding_digest": binding.BindingDigest,
		"assigned_budget_units":    int64(60),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(ctx, journal.Event{
		ID: "malformed-account-capacity", IdempotencyKey: "malformed-account-capacity",
		StreamID: "provider-account-capacity/deepseek.work", Seq: 3,
		Type: "ProviderAccountCapacityReserved", SchemaVersion: 1,
		EmittedAt:     now.Add(time.Second),
		CorrelationID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		CausationID:   "missing-claim", PayloadJSON: badPayload,
	}); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(ctx); !errors.Is(err, ErrInvalidProjectionEvent) {
		t.Fatalf("malformed account capacity rebuild = %v", err)
	}
	after, ok := readModel.GlobalReadView().Run("run-account-projection")
	if !ok || after.ProviderAccountPolicyDigest != terminal.ProviderAccountPolicyDigest ||
		after.TerminalStatus != terminal.TerminalStatus {
		t.Fatalf("prior view changed: before=%#v after=%#v", terminal, after)
	}
}

func TestGlobalReadViewCopiesFrozenDisclosurePolicyToExactTeamAttempt(t *testing.T) {
	snapshot := emptySnapshot()
	snapshot.Runs["run-disclosure"] = Run{
		ID: "run-disclosure", WorkItemID: "work-disclosure",
		ProviderAccountPolicyAvailable: true,
		ProviderAccountPolicyVersion:   2,
		ProviderAccountPolicyRevision:  7,
		ProviderAccountPolicyDigest:    strings.Repeat("d", 64),
		ProviderAccountTrustDomain:     "enterprise_tenant",
		ProviderAccountRetentionMode:   "zero_data_retention",
		ProviderAccountDataRegion:      "eu",
	}
	view := mustBuildGlobalReadView(
		t, snapshot, nil,
		map[string]TeamExecution{
			"team-disclosure": {
				TeamInstanceID: "team-disclosure",
				Nodes: []TeamExecutionNode{{
					LogicalNodeID: "review",
					Attempts: []TeamExecutionAttempt{{
						AttemptNumber: 1, RunID: "run-disclosure",
					}},
				}},
			},
		},
	)
	execution, ok := view.TeamExecution("team-disclosure")
	if !ok || len(execution.Nodes) != 1 || len(execution.Nodes[0].Attempts) != 1 {
		t.Fatalf("Team execution = %#v, ok=%t", execution, ok)
	}
	attempt := execution.Nodes[0].Attempts[0]
	if !attempt.ProviderAccountPolicyAvailable ||
		attempt.ProviderAccountPolicyVersion != 2 ||
		attempt.ProviderAccountPolicyRevision != 7 ||
		attempt.ProviderAccountPolicyDigest != strings.Repeat("d", 64) ||
		attempt.ProviderAccountTrustDomain != "enterprise_tenant" ||
		attempt.ProviderAccountRetentionMode != "zero_data_retention" ||
		attempt.ProviderAccountDataRegion != "eu" {
		t.Fatalf("Team Attempt disclosure = %#v", attempt)
	}
}

func appendProviderAccountCapacityRuntime(
	t *testing.T,
	store *journal.Store,
	now time.Time,
) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"discovery_digest": strings.Repeat("1", 64),
		"source_probe_id":  "provider-account-projection-probe",
		"instance": map[string]any{
			"id": "runtime-account-projection", "device_id": "device.local",
			"adapter_type": "loom-native", "display_name": "Loom Native",
			"executable_version": "1.0.0", "status": "online",
			"observed_capabilities": []string{"chat", "tools"}, "capacity": 4,
		},
		"model_ids": []string{"deepseek-chat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID:             "runtime-account-projection-discovered",
		IdempotencyKey: "runtime-account-projection-discovered",
		StreamID:       "runtime_instance:runtime-account-projection", Seq: 1,
		Type: "RuntimeInstanceDiscovered", SchemaVersion: 1,
		EmittedAt:     now.Add(-time.Minute),
		CorrelationID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		PayloadJSON:   body,
	}); err != nil {
		t.Fatal(err)
	}
}

func providerAccountCapacityProjectionBinding(
	t *testing.T,
	budget int64,
) loomruntime.FrozenExecutionBinding {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "profile-account-projection", AdapterType: "loom-native",
		ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		ModelID: "deepseek-chat", AuthMode: loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("2", 64),
		CredentialReference: "credential-ref-deepseek-work", CredentialRevision: 7,
		RequiredCapabilities: []string{"chat", "tools"},
		Timeout:              time.Minute, Budget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime-account-projection", DeviceID: "device.local",
		AdapterType: "loom-native", DisplayName: "Loom Native",
		Status:               loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"chat", "tools"}, Capacity: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}
