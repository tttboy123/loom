package projection

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestPhase2DTeamExecutionProjectionCarriesAndValidatesFrozenBinding(t *testing.T) {
	digest := strings.Repeat("a", 64)
	viewVersion := strings.Repeat("b", 64)
	budget := int64(175)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "profile.claude", AdapterType: "claude-code",
		ProviderID: "anthropic", ProviderAccountID: "anthropic.work",
		ModelID: "claude-sonnet", AuthMode: loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("c", 64),
		CredentialReference: "credential-ref-anthropic-work",
		CredentialRevision:  7, ReasoningEffort: "high",
		RequiredCapabilities: []string{
			"edit", loomruntime.CapabilityReasoningEffort, "terminal",
		},
		Timeout: 2 * time.Minute, Budget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime-main", DeviceID: "device.local", AdapterType: "claude-code",
		DisplayName: "Claude Code", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{
			"edit", loomruntime.CapabilityReasoningEffort, "terminal",
		}, Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	eventsFor := func(bindingDigest string) []journal.Event {
		bindingPayload := map[string]any{
			"profile_id": profile.ID, "harness_adapter": profile.AdapterType,
			"runtime_instance_id": instance.ID, "provider_id": profile.ProviderID,
			"provider_account_id": profile.ProviderAccountID, "model_id": profile.ModelID,
			"auth_mode": string(profile.AuthMode), "endpoint_fingerprint": profile.EndpointFingerprint,
			"credential_reference": profile.CredentialReference,
			"credential_revision":  profile.CredentialRevision,
			"reasoning_effort":     profile.ReasoningEffort,
			"timeout_nanoseconds":  int64(profile.Timeout), "budget": budget,
			"capabilities": []string{
				"edit", loomruntime.CapabilityReasoningEffort, "terminal",
			}, "binding_digest": bindingDigest,
		}
		return []journal.Event{
			teamProjectionEvent(t, "plan-binding", "team-execution/team-binding", 1, "plan-binding", "TeamExecutionPlanned", map[string]any{
				"team_instance_id": "team-binding", "plan_digest": digest, "view_version": viewVersion,
				"nodes": []map[string]any{{
					"logical_node_id": "main", "title": "Main", "agent_instance_id": "agent-main",
					"runtime_instance_id": "runtime-main", "role": "main",
					"depends_on": []string{}, "max_attempts": 1,
				}},
			}),
			teamProjectionEvent(t, "scheduled-binding", "team-execution/team-binding", 2, "scheduled-binding", "TeamNodeAttemptScheduled", map[string]any{
				"logical_node_id": "main", "attempt_number": 1,
				"work_item_id": "team-work-binding", "run_id": "team-run-binding",
				"runtime_instance_id": "runtime-main", "agent_instance_id": "agent-main",
				"retry_at": "",
			}),
			teamProjectionEvent(t, "dispatch-binding", "team-execution/team-binding", 3, "dispatch-binding", "TeamReadySetDispatched", map[string]any{
				"team_instance_id": "team-binding", "plan_digest": digest, "view_version": viewVersion,
				"attempts": []map[string]any{{
					"logical_node_id": "main", "attempt_number": 1,
					"work_item_id": "team-work-binding", "run_id": "team-run-binding",
					"claim_id": "11111111-1111-4111-8111-111111111111", "claim_generation": 1,
					"runtime_instance_id": "runtime-main", "agent_instance_id": "agent-main",
					"execution_binding": bindingPayload,
				}},
			}),
		}
	}
	const incidentID = "22222222-2222-4222-8222-222222222222"
	events := eventsFor(binding.BindingDigest)
	events[2].CorrelationID = incidentID
	record, err := projectTeamExecutionStream(
		"team-binding", events, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	attempt := record.Nodes[0].Attempts[0]
	if !attempt.ExecutionBindingAvailable ||
		attempt.IncidentID != incidentID ||
		attempt.ExecutionBinding.ProviderAccountID != "anthropic.work" ||
		attempt.ExecutionBinding.CredentialRevision != 7 ||
		attempt.ExecutionBinding.ModelID != "claude-sonnet" ||
		attempt.ExecutionBinding.ReasoningEffort != "high" ||
		attempt.ExecutionBinding.Budget == nil ||
		*attempt.ExecutionBinding.Budget != budget {
		t.Fatalf("projected execution binding = %#v", attempt.ExecutionBinding)
	}
	if _, err := projectTeamExecutionStream(
		"team-binding", eventsFor(strings.Repeat("f", 64)), nil, nil,
	); !errors.Is(err, ErrInvalidProjectionEvent) {
		t.Fatalf("tampered projected binding error = %v", err)
	}
}

func TestPhase2DTeamExecutionProjectionCarriesParallelRouteIdentity(t *testing.T) {
	digest := strings.Repeat("a", 64)
	events := []journal.Event{
		teamProjectionEvent(t, "plan-parallel", "team-execution/team-parallel", 1, "plan-parallel", "TeamExecutionPlanned", map[string]any{
			"team_instance_id": "team-parallel", "plan_digest": digest,
			"view_version": strings.Repeat("b", 64),
			"nodes": []map[string]any{
				{"logical_node_id": "main", "title": "Synthesis", "agent_instance_id": "agent-main", "runtime_instance_id": "runtime-main", "role": "main", "kind": "aggregation", "route_group_id": "route-group-main", "depends_on": []string{"route-a", "route-b"}, "max_attempts": 1},
				{"logical_node_id": "route-a", "title": "Route A", "agent_instance_id": "agent-main", "runtime_instance_id": "runtime-a", "role": "main", "kind": "route_sibling", "route_group_id": "route-group-main", "depends_on": []string{}, "max_attempts": 1},
				{"logical_node_id": "route-b", "title": "Route B", "agent_instance_id": "agent-main", "runtime_instance_id": "runtime-b", "role": "main", "kind": "route_sibling", "route_group_id": "route-group-main", "depends_on": []string{}, "max_attempts": 1},
			},
		}),
	}
	record, err := projectTeamExecutionStream("team-parallel", events, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Nodes) != 3 ||
		record.Nodes[0].Kind != "aggregation" || record.Nodes[0].RouteGroupID != "route-group-main" ||
		record.Nodes[1].Kind != "route_sibling" || record.Nodes[2].Kind != "route_sibling" {
		t.Fatalf("parallel route projection = %#v", record.Nodes)
	}

	var payload map[string]any
	if err := json.Unmarshal(events[0].PayloadJSON, &payload); err != nil {
		t.Fatal(err)
	}
	nodes := payload["nodes"].([]any)
	nodes[0].(map[string]any)["route_group_id"] = "forged-group"
	events[0].PayloadJSON, _ = json.Marshal(payload)
	if _, err := projectTeamExecutionStream("team-parallel", events, nil, nil); !errors.Is(err, ErrInvalidProjectionEvent) {
		t.Fatalf("forged route group projection error = %v", err)
	}
}

func TestPhase2DTeamExecutionProjectionReplaysInitialAgentBlocks(t *testing.T) {
	digest := strings.Repeat("a", 64)
	viewVersion := strings.Repeat("b", 64)
	incidentID := "11111111-1111-4111-8111-111111111111"
	events := []journal.Event{
		teamProjectionEvent(t, "plan-isolation", "team-execution/team-isolation", 1, "plan-isolation", "TeamExecutionPlanned", map[string]any{
			"team_instance_id": "team-isolation", "plan_digest": digest, "view_version": viewVersion,
			"nodes": []map[string]any{
				{"logical_node_id": "blocked", "title": "Blocked", "agent_instance_id": "agent-blocked", "runtime_instance_id": "runtime-blocked", "role": "subagent", "depends_on": []string{}, "max_attempts": 1},
				{"logical_node_id": "main", "title": "Main", "agent_instance_id": "agent-main", "runtime_instance_id": "runtime-main", "role": "main", "depends_on": []string{"blocked"}, "max_attempts": 1},
			},
			"route_summaries": []map[string]any{
				{"logical_node_id": "blocked", "harness_adapter": "loom-native", "provider_id": "deepseek", "provider_account_id": "deepseek.primary", "model_id": "deepseek-chat", "timeout_nanoseconds": int64(time.Minute), "budget": nil, "capabilities": []string{"chat"}, "credential_revision": int64(6)},
				{"logical_node_id": "main", "harness_adapter": "codex", "provider_id": "openai", "provider_account_id": "", "model_id": "codex", "timeout_nanoseconds": int64(time.Minute), "budget": nil, "capabilities": []string{"terminal"}, "credential_revision": int64(0)},
			},
		}),
		teamProjectionEvent(t, "blocked-isolation", "team-execution/team-isolation", 2, "blocked-isolation", "TeamNodeInitiallyBlocked", map[string]any{
			"logical_node_id": "blocked", "code": "credential_unavailable", "stage": "credential_lease_issue",
			"reason": "Credential is not verified.", "retryable": true, "source_logical_node_id": "",
		}, "plan-isolation"),
		teamProjectionEvent(t, "main-isolation", "team-execution/team-isolation", 3, "main-isolation", "TeamNodeInitiallyBlocked", map[string]any{
			"logical_node_id": "main", "code": "dependency_blocked", "stage": "agent_attempt_dispatch",
			"reason": "A required Agent is blocked.", "retryable": true, "source_logical_node_id": "blocked",
		}, "blocked-isolation"),
		teamProjectionEvent(t, "terminal-isolation", "team-execution/team-isolation", 4, "terminal-isolation", "TeamExecutionTerminal", map[string]any{
			"team_instance_id": "team-isolation", "plan_digest": digest, "status": "blocked", "reason": "node_blocked_blocked",
		}, "main-isolation"),
	}
	events[1].CorrelationID = incidentID
	events[2].CorrelationID = incidentID
	record, err := projectTeamExecutionStream("team-isolation", events, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != "blocked" || record.Nodes[0].InitialBlockCode != "credential_unavailable" ||
		record.Nodes[0].InitialBlockStage != "credential_lease_issue" ||
		record.Nodes[0].InitialBlockIncidentID != incidentID || !record.Nodes[0].InitialBlockRetryable ||
		!record.Nodes[0].InitialRouteAvailable || record.Nodes[0].InitialProviderAccountID != "deepseek.primary" ||
		record.Nodes[0].InitialCredentialRevision != 6 || !record.Nodes[1].InitialRouteAvailable ||
		record.Nodes[1].InitialProviderID != "openai" || record.Nodes[1].InitialCredentialRevision != 0 ||
		record.Nodes[1].InitialBlockCode != "dependency_blocked" ||
		record.Nodes[1].InitialBlockSourceNodeID != "blocked" {
		t.Fatalf("projected initial blocks = %#v", record)
	}
	tampered := append([]journal.Event(nil), events...)
	var payload map[string]any
	if err := json.Unmarshal(tampered[2].PayloadJSON, &payload); err != nil {
		t.Fatal(err)
	}
	payload["source_logical_node_id"] = "missing"
	tampered[2].PayloadJSON, _ = json.Marshal(payload)
	if _, err := projectTeamExecutionStream("team-isolation", tampered, nil, nil); !errors.Is(err, ErrInvalidProjectionEvent) {
		t.Fatalf("forged dependency block error = %v", err)
	}
	tampered = append([]journal.Event(nil), events...)
	var planPayload map[string]any
	if err := json.Unmarshal(tampered[0].PayloadJSON, &planPayload); err != nil {
		t.Fatal(err)
	}
	routes := planPayload["route_summaries"].([]any)
	route := routes[0].(map[string]any)
	route["credential_revision"] = float64(0)
	tampered[0].PayloadJSON, _ = json.Marshal(planPayload)
	if _, err := projectTeamExecutionStream("team-isolation", tampered, nil, nil); !errors.Is(err, ErrInvalidProjectionEvent) {
		t.Fatalf("forged credential revision error = %v", err)
	}
}

func TestTeamExecutionProjectionTracksLogicalAttemptsAndDeepCopies(t *testing.T) {
	digest := strings.Repeat("a", 64)
	viewVersion := strings.Repeat("b", 64)
	events := []journal.Event{
		teamProjectionEvent(t, "team-plan", "team-execution/team-1", 1, "team-plan", "TeamExecutionPlanned", map[string]any{
			"team_instance_id": "team-1",
			"plan_digest":      digest,
			"view_version":     viewVersion,
			"nodes": []map[string]any{{
				"logical_node_id":     "main",
				"title":               "Main",
				"agent_instance_id":   "agent-main",
				"runtime_instance_id": "runtime-main",
				"role":                "main",
				"depends_on":          []string{},
				"max_attempts":        2,
			}},
		}),
		teamProjectionEvent(t, "team-scheduled", "team-execution/team-1", 2, "team-scheduled", "TeamNodeAttemptScheduled", map[string]any{
			"logical_node_id":     "main",
			"attempt_number":      1,
			"work_item_id":        "team-work-1",
			"run_id":              "team-run-1",
			"runtime_instance_id": "runtime-main",
			"agent_instance_id":   "agent-main",
			"retry_at":            "",
		}),
		teamProjectionEvent(t, "team-dispatch", "team-execution/team-1", 3, "team-dispatch", "TeamReadySetDispatched", map[string]any{
			"team_instance_id": "team-1",
			"plan_digest":      digest,
			"view_version":     viewVersion,
			"attempts": []map[string]any{{
				"logical_node_id":     "main",
				"attempt_number":      1,
				"work_item_id":        "team-work-1",
				"run_id":              "team-run-1",
				"claim_id":            "11111111-1111-4111-8111-111111111111",
				"claim_generation":    1,
				"runtime_instance_id": "runtime-main",
				"agent_instance_id":   "agent-main",
			}},
		}),
		teamProjectionEvent(t, "team-rebound", "team-execution/team-1", 4, "team-rebound", "TeamNodeAttemptRebound", map[string]any{
			"team_instance_id":          "team-1",
			"plan_digest":               digest,
			"logical_node_id":           "main",
			"attempt_number":            1,
			"work_item_id":              "team-work-1",
			"run_id":                    "team-run-1",
			"previous_claim_id":         "11111111-1111-4111-8111-111111111111",
			"previous_claim_generation": 1,
			"claim_id":                  "22222222-2222-4222-8222-222222222222",
			"claim_generation":          2,
			"runtime_instance_id":       "runtime-main",
			"agent_instance_id":         "agent-main",
			"run_stream":                "run/team-run-1",
			"run_sequence":              2,
			"run_event_id":              "run-reclaim",
		}),
		teamProjectionEvent(t, "team-attempt-terminal", "team-execution/team-1", 5, "team-attempt-terminal", "TeamNodeAttemptTerminal", map[string]any{
			"logical_node_id":     "main",
			"attempt_number":      1,
			"work_item_id":        "team-work-1",
			"run_id":              "team-run-1",
			"claim_id":            "22222222-2222-4222-8222-222222222222",
			"claim_generation":    2,
			"runtime_instance_id": "runtime-main",
			"agent_instance_id":   "agent-main",
			"status":              "succeeded",
			"evidence_id":         "team-evidence-1",
			"evidence_digest":     digest,
		}),
		teamProjectionEvent(t, "team-terminal", "team-execution/team-1", 6, "team-terminal", "TeamExecutionTerminal", map[string]any{
			"team_instance_id": "team-1",
			"plan_digest":      digest,
			"status":           "succeeded",
			"reason":           "",
		}, "team-attempt-terminal"),
	}
	events[2].CorrelationID = "11111111-1111-4111-8111-111111111111"
	events[3].CorrelationID = "22222222-2222-4222-8222-222222222222"
	events[4].CorrelationID = "22222222-2222-4222-8222-222222222222"
	record, err := projectTeamExecutionStream(
		"team-1",
		events,
		map[string]teamExecutionRunClaimReference{
			teamExecutionRunReferenceKey("run/team-run-1", 2): {
				eventID: "run-reclaim",
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("projectTeamExecutionStream() error = %v", err)
	}
	view := mustBuildGlobalReadView(t,
		emptySnapshot(),
		events,
		map[string]TeamExecution{"team-1": record},
	)
	record, ok := view.TeamExecution("team-1")
	if !ok || record.Status != "succeeded" || len(record.Nodes) != 1 ||
		len(record.Nodes[0].Attempts) != 1 {
		t.Fatalf("TeamExecution() = %#v, %v", record, ok)
	}
	if !record.LegacySemanticUnbound {
		t.Fatal("accepted schema-v1 Team must remain explicitly semantic-unbound")
	}
	want := TeamExecutionAttempt{
		AttemptNumber:     1,
		IncidentID:        "22222222-2222-4222-8222-222222222222",
		WorkItemID:        "team-work-1",
		RunID:             "team-run-1",
		ClaimID:           "22222222-2222-4222-8222-222222222222",
		ClaimGeneration:   2,
		RuntimeInstanceID: "runtime-main",
		AgentInstanceID:   "agent-main",
		Status:            "succeeded",
		EvidenceID:        "team-evidence-1",
		EvidenceDigest:    digest,
	}
	if got := record.Nodes[0].Attempts[0]; !reflect.DeepEqual(got, want) {
		t.Fatalf("attempt = %#v, want %#v", got, want)
	}
	record.Nodes[0].Attempts[0].EvidenceDigest = "mutated"
	again, _ := view.TeamExecution("team-1")
	if again.Nodes[0].Attempts[0].EvidenceDigest != digest {
		t.Fatal("Team execution view aliases caller mutation")
	}
}

func TestTeamExecutionProjectionReplaysSemanticRecoveryMetadata(t *testing.T) {
	digest := strings.Repeat("1", 64)
	contractDigest := strings.Repeat("2", 64)
	policyDigest := strings.Repeat("3", 64)
	classificationDigest := strings.Repeat("4", 64)
	summaryDigest := strings.Repeat("5", 64)
	decisionDigest := strings.Repeat("6", 64)
	viewVersion := strings.Repeat("7", 64)
	events := []journal.Event{
		teamProjectionEvent(t, "plan-v2", "team-execution/team-v2", 1, "plan-v2", "TeamExecutionPlanned", map[string]any{
			"team_instance_id": "team-v2",
			"plan_digest":      digest,
			"view_version":     viewVersion,
			"nodes": []map[string]any{{
				"logical_node_id":     "main",
				"title":               "Main",
				"agent_instance_id":   "agent-main",
				"runtime_instance_id": "runtime-main",
				"role":                "main",
				"depends_on":          []string{},
				"max_attempts":        2,
			}},
			"semantic_bindings": []map[string]any{{
				"logical_node_id":            "main",
				"output_contract_version":    1,
				"output_contract_digest":     contractDigest,
				"recovery_policy_version":    2,
				"recovery_policy_digest":     policyDigest,
				"attempt_credits":            0,
				"primary_workflow_path":      "primary",
				"workflow_fallback_key":      "cached-source",
				"recovery_approval_required": false,
			}},
		}),
		teamProjectionEvent(t, "scheduled-v2", "team-execution/team-v2", 2, "scheduled-v2", "TeamNodeAttemptScheduled", map[string]any{
			"logical_node_id":     "main",
			"attempt_number":      1,
			"work_item_id":        "team-work-v2",
			"run_id":              "team-run-v2",
			"runtime_instance_id": "runtime-main",
			"agent_instance_id":   "agent-main",
			"workflow_path":       "primary",
			"retry_at":            "",
		}),
		teamProjectionEvent(t, "dispatch-v2", "team-execution/team-v2", 3, "dispatch-v2", "TeamReadySetDispatched", map[string]any{
			"team_instance_id": "team-v2",
			"plan_digest":      digest,
			"view_version":     viewVersion,
			"attempts": []map[string]any{{
				"logical_node_id":     "main",
				"attempt_number":      1,
				"work_item_id":        "team-work-v2",
				"run_id":              "team-run-v2",
				"claim_id":            "11111111-1111-4111-8111-111111111111",
				"claim_generation":    1,
				"runtime_instance_id": "runtime-main",
				"agent_instance_id":   "agent-main",
			}},
		}),
		teamProjectionEvent(t, "terminal-v2", "team-execution/team-v2", 4, "terminal-v2", "TeamNodeAttemptTerminal", map[string]any{
			"logical_node_id":              "main",
			"attempt_number":               1,
			"work_item_id":                 "team-work-v2",
			"run_id":                       "team-run-v2",
			"claim_id":                     "11111111-1111-4111-8111-111111111111",
			"claim_generation":             1,
			"runtime_instance_id":          "runtime-main",
			"agent_instance_id":            "agent-main",
			"status":                       "succeeded",
			"evidence_id":                  "team-evidence-v2",
			"evidence_digest":              digest,
			"output_contract_version":      1,
			"output_contract_digest":       contractDigest,
			"output_classification":        "transient_empty",
			"output_classification_digest": classificationDigest,
			"output_summary_digest":        summaryDigest,
		}),
		teamProjectionEvent(t, "recovery-v2", "team-execution/team-v2", 5, "recovery-v2", "TeamNodeRecoveryRecorded", map[string]any{
			"logical_node_id":            "main",
			"attempt_number":             1,
			"action":                     "blocked",
			"decision_time":              "2026-07-26T01:02:03Z",
			"retry_at":                   "",
			"next_attempt_number":        0,
			"next_agent_instance_id":     "",
			"next_runtime_instance_id":   "",
			"workflow_fallback_key":      "",
			"recovery_policy_version":    2,
			"recovery_policy_digest":     policyDigest,
			"recovery_decision_digest":   decisionDigest,
			"classification_digest":      classificationDigest,
			"prior_classifications":      []string{},
			"credits_before":             0,
			"credits_after":              0,
			"fallback_consumed":          false,
			"recovery_approval_required": false,
			"dependency_satisfied":       false,
		}),
		teamProjectionEvent(t, "team-terminal-v2", "team-execution/team-v2", 6, "team-terminal-v2", "TeamExecutionTerminal", map[string]any{
			"team_instance_id": "team-v2",
			"plan_digest":      digest,
			"status":           "blocked",
			"reason":           "node_main_blocked",
		}, "recovery-v2"),
	}
	record, err := projectTeamExecutionStream("team-v2", events, nil, nil)
	if err != nil {
		t.Fatalf("projectTeamExecutionStream() error = %v", err)
	}
	if record.LegacySemanticUnbound || len(record.Nodes) != 1 {
		t.Fatalf("semantic binding = %#v", record)
	}
	node := record.Nodes[0]
	if node.OutputContractVersion != 1 ||
		node.OutputContractDigest != contractDigest ||
		node.RecoveryPolicyVersion != 2 ||
		node.RecoveryPolicyDigest != policyDigest ||
		node.AttemptCredits != 0 ||
		node.PrimaryWorkflowPath != "primary" ||
		node.WorkflowFallbackKey != "cached-source" ||
		node.RecoveryAction != "blocked" ||
		node.RecoveryDecisionDigest != decisionDigest ||
		node.CreditsBefore != 0 ||
		node.CreditsAfter != 0 ||
		len(node.PriorClassifications) != 0 {
		t.Fatalf("projected node metadata = %#v", node)
	}
	attempt := node.Attempts[0]
	if attempt.WorkflowPath != "primary" ||
		attempt.OutputContractVersion != 1 ||
		attempt.OutputContractDigest != contractDigest ||
		attempt.OutputClassification != "transient_empty" ||
		attempt.OutputClassificationDigest != classificationDigest ||
		attempt.OutputSummaryDigest != summaryDigest {
		t.Fatalf("projected attempt metadata = %#v", attempt)
	}
	view := mustBuildGlobalReadView(t,
		emptySnapshot(),
		events,
		map[string]TeamExecution{"team-v2": record},
	)
	copied, _ := view.TeamExecution("team-v2")
	copied.Nodes[0].Attempts[0].OutputClassification = "mutated"
	again, _ := view.TeamExecution("team-v2")
	if again.Nodes[0].Attempts[0].OutputClassification != "transient_empty" {
		t.Fatal("Team recovery attempt aliases caller mutation")
	}
	malformed := append([]journal.Event(nil), events...)
	var recoveryPayload map[string]any
	if err := json.Unmarshal(
		malformed[4].PayloadJSON,
		&recoveryPayload,
	); err != nil {
		t.Fatal(err)
	}
	recoveryPayload["credits_after"] = float64(1)
	malformed[4].PayloadJSON, err = json.Marshal(recoveryPayload)
	if err != nil {
		t.Fatal(err)
	}
	failing := newForTestSource(eventSliceSource{events: malformed})
	failing.snapshot = emptySnapshot()
	failing.view = view
	if err := failing.Rebuild(context.Background()); !errors.Is(
		err,
		ErrInvalidProjectionEvent,
	) {
		t.Fatalf("malformed recovery Rebuild() error = %v", err)
	}
	if got := failing.GlobalReadView(); got.Version() != view.Version() {
		t.Fatalf(
			"malformed recovery replaced old view: %q != %q",
			got.Version(),
			view.Version(),
		)
	}
}

func TestTeamExecutionProjectionStopsRecoveryAtMaxAttempts(t *testing.T) {
	digest := strings.Repeat("c", 64)
	viewVersion := strings.Repeat("d", 64)
	events := []journal.Event{
		teamProjectionEvent(t, "plan", "team-execution/team-max", 1, "plan", "TeamExecutionPlanned", map[string]any{
			"team_instance_id": "team-max",
			"plan_digest":      digest,
			"view_version":     viewVersion,
			"nodes": []map[string]any{{
				"logical_node_id":     "main",
				"title":               "Main",
				"agent_instance_id":   "agent-main",
				"runtime_instance_id": "runtime-main",
				"role":                "main",
				"depends_on":          []string{},
				"max_attempts":        1,
			}},
		}),
		teamProjectionEvent(t, "scheduled", "team-execution/team-max", 2, "scheduled", "TeamNodeAttemptScheduled", map[string]any{
			"logical_node_id":     "main",
			"attempt_number":      1,
			"work_item_id":        "team-work-max",
			"run_id":              "team-run-max",
			"runtime_instance_id": "runtime-main",
			"agent_instance_id":   "agent-main",
			"retry_at":            "",
		}),
		teamProjectionEvent(t, "dispatch", "team-execution/team-max", 3, "dispatch", "TeamReadySetDispatched", map[string]any{
			"team_instance_id": "team-max",
			"plan_digest":      digest,
			"view_version":     viewVersion,
			"attempts": []map[string]any{{
				"logical_node_id":     "main",
				"attempt_number":      1,
				"work_item_id":        "team-work-max",
				"run_id":              "team-run-max",
				"claim_id":            "11111111-1111-4111-8111-111111111111",
				"claim_generation":    1,
				"runtime_instance_id": "runtime-main",
				"agent_instance_id":   "agent-main",
			}},
		}),
		teamProjectionEvent(t, "terminal", "team-execution/team-max", 4, "terminal", "TeamNodeAttemptTerminal", map[string]any{
			"logical_node_id":     "main",
			"attempt_number":      1,
			"work_item_id":        "team-work-max",
			"run_id":              "team-run-max",
			"claim_id":            "11111111-1111-4111-8111-111111111111",
			"claim_generation":    1,
			"runtime_instance_id": "runtime-main",
			"agent_instance_id":   "agent-main",
			"status":              "failed",
			"evidence_id":         "team-evidence-max",
			"evidence_digest":     digest,
		}),
		teamProjectionEvent(t, "team-terminal", "team-execution/team-max", 5, "team-terminal", "TeamExecutionTerminal", map[string]any{
			"team_instance_id": "team-max",
			"plan_digest":      digest,
			"status":           "failed",
			"reason":           "node_main_failed",
		}, "terminal"),
	}
	readModel := newForTestSource(eventSliceSource{events: events})
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	record, ok := readModel.GlobalReadView().TeamExecution("team-max")
	if !ok || record.Status != "failed" ||
		len(record.Nodes) != 1 || record.Nodes[0].Status != "failed" {
		t.Fatalf("terminal max-attempt projection = %#v, %v", record, ok)
	}
}

func teamProjectionEvent(
	t testing.TB,
	id string,
	streamID string,
	sequence int64,
	idempotencyKey string,
	eventType string,
	payload any,
	causation ...string,
) journal.Event {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	event := journal.Event{
		ID: id, StreamID: streamID, Seq: sequence,
		IdempotencyKey: idempotencyKey, Type: eventType,
		SchemaVersion: 1,
		EmittedAt:     time.Date(2026, 7, 26, 1, 2, 3, 0, time.UTC),
		PayloadJSON:   encoded,
	}
	if len(causation) > 0 {
		event.CausationID = causation[0]
	}
	return event
}
