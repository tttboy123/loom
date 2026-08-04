package projection

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/journal"
)

func TestP3ATeamExecutionProjectionPreservesExactAssetLineageAndDeepCopies(t *testing.T) {
	binding := assets.ExactAssetRevisionBinding{
		AssetKind: assets.AssetKindSkill, DefinitionID: "skill-1",
		RevisionID: "revision-1", SHA256Digest: strings.Repeat("a", 64),
		SourceScope: assets.SourceScopeLocal,
	}
	setDigest, err := assets.CanonicalAssetRevisionSetDigest(
		[]assets.ExactAssetRevisionBinding{binding},
	)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := strings.Repeat("b", 64)
	rootDigest := strings.Repeat("9", 64)
	planDigest := strings.Repeat("c", 64)
	viewVersion := strings.Repeat("d", 64)
	events := []journal.Event{
		teamProjectionEvent(t, "p3a-plan", "team-execution/team-asset", 1, "p3a-plan", "TeamExecutionPlanned", map[string]any{
			"team_instance_id": "team-asset", "plan_digest": planDigest,
			"view_version": viewVersion,
			"nodes": []map[string]any{{
				"logical_node_id": "main", "title": "Main",
				"agent_instance_id": "agent-main", "runtime_instance_id": "runtime-main",
				"role": "main", "depends_on": []string{}, "max_attempts": 1,
				"asset_revision_bindings":   []assets.ExactAssetRevisionBinding{binding},
				"asset_revision_set_digest": setDigest,
			}},
			"semantic_bindings": []map[string]any{{
				"logical_node_id":         "main",
				"output_contract_version": 1, "output_contract_digest": strings.Repeat("e", 64),
				"recovery_policy_version": 1, "recovery_policy_digest": strings.Repeat("f", 64),
				"attempt_credits": 0, "primary_workflow_path": "primary",
				"workflow_fallback_key": "", "recovery_approval_required": false,
				"asset_revision_bindings":   []assets.ExactAssetRevisionBinding{binding},
				"asset_revision_set_digest": setDigest,
			}},
		}),
		teamProjectionEvent(t, "p3a-scheduled", "team-execution/team-asset", 2, "p3a-scheduled", "TeamNodeAttemptScheduled", map[string]any{
			"logical_node_id": "main", "attempt_number": 1,
			"work_item_id": "work-asset", "run_id": "run-asset",
			"runtime_instance_id": "runtime-main", "agent_instance_id": "agent-main",
			"workflow_path": "primary", "retry_at": "",
		}),
		teamProjectionEvent(t, "p3a-dispatched", "team-execution/team-asset", 3, "p3a-dispatched", "TeamReadySetDispatched", map[string]any{
			"team_instance_id": "team-asset", "plan_digest": planDigest,
			"view_version": viewVersion,
			"attempts": []map[string]any{{
				"logical_node_id": "main", "attempt_number": 1,
				"work_item_id": "work-asset", "run_id": "run-asset",
				"claim_id": "11111111-1111-4111-8111-111111111111", "claim_generation": 1,
				"runtime_instance_id": "runtime-main", "agent_instance_id": "agent-main",
				"asset_revision_bindings":         []assets.ExactAssetRevisionBinding{binding},
				"asset_revision_set_digest":       setDigest,
				"materialization_manifest_digest": manifestDigest,
				"materialization_root_digest":     rootDigest,
			}},
		}),
	}
	record, err := projectTeamExecutionStream(
		"team-asset", events,
		map[string]teamExecutionRunClaimReference{
			teamExecutionRunReferenceKey("run/run-asset", 1): {eventID: "p3a-run-claim"},
		},
		map[string]journal.Event{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !record.AssetLineageAvailable || !record.Nodes[0].AssetLineageAvailable ||
		!record.Nodes[0].Attempts[0].AssetLineageAvailable ||
		record.Nodes[0].AssetRevisionSetDigest != setDigest ||
		record.Nodes[0].Attempts[0].MaterializationManifestDigest != manifestDigest ||
		record.Nodes[0].Attempts[0].MaterializationRootDigest != rootDigest {
		t.Fatalf("projected lineage = %#v", record)
	}
	copy := cloneGlobalTeamExecution(record)
	copy.Nodes[0].AssetRevisionBindings[0].RevisionID = "mutated"
	copy.Nodes[0].Attempts[0].AssetRevisionBindings[0].RevisionID = "mutated"
	if record.Nodes[0].AssetRevisionBindings[0].RevisionID != "revision-1" ||
		record.Nodes[0].Attempts[0].AssetRevisionBindings[0].RevisionID != "revision-1" {
		t.Fatal("projected asset lineage aliases returned copy")
	}
}

func TestP3AMalformedEvolutionAssetFactPreservesPublishedGlobalReadView(t *testing.T) {
	base := projectionEvent(
		"base-mode", "mode-stream", 1, "base-mode", "ModeSelected",
		map[string]string{"mode": "agent"},
	)
	projection := newForTestSource(eventSliceSource{events: []journal.Event{base}})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("initial Rebuild() error = %v", err)
	}
	accepted := projection.GlobalReadView()
	malformed := journal.Event{
		ID: "asset-bad", StreamID: "evolution-asset/skill-1", Seq: 1,
		IdempotencyKey: "asset-bad", Type: "EvolutionAssetRevisionCreated",
		SchemaVersion: 1, EmittedAt: time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC),
		CorrelationID: "123e4567-e89b-42d3-a456-426614174000",
		PayloadJSON:   []byte(`{"schema_version":1,"operation_id":"operation-1","unknown":true}`),
	}
	projection.source = eventSliceSource{events: []journal.Event{base, malformed}}
	if err := projection.Rebuild(context.Background()); err == nil {
		t.Fatal("malformed evolution asset Rebuild() error = nil")
	}
	if got := projection.GlobalReadView().Version(); got != accepted.Version() {
		t.Fatalf("failed rebuild replaced old view: got %s want %s", got, accepted.Version())
	}
}

func TestP3AMaterializationGlobalReadAccessorsDeepCopyBindings(t *testing.T) {
	record := assets.RuntimeSkillMaterializationRecord{
		RunID: "run-1", AttemptNumber: 1, Generation: 1,
		AssetRevisionBindings: []assets.ExactAssetRevisionBinding{{
			AssetKind: assets.AssetKindSkill, DefinitionID: "skill-1",
			RevisionID: "revision-1", SHA256Digest: strings.Repeat("a", 64),
			SourceScope: assets.SourceScopeLocal,
		}},
	}
	view := GlobalReadView{runtimeSkillMaterializations: map[string]assets.RuntimeSkillMaterializationRecord{
		"run-1/1/1": record,
	}}
	one, ok := view.RuntimeSkillMaterialization("run-1/1/1")
	if !ok {
		t.Fatal("materialization missing")
	}
	one.AssetRevisionBindings[0].RevisionID = "mutated-one"
	page, more := view.RuntimeSkillMaterializations("", 64)
	if more || len(page) != 1 {
		t.Fatalf("page = %#v more=%v", page, more)
	}
	page[0].AssetRevisionBindings[0].RevisionID = "mutated-page"
	again, _ := view.RuntimeSkillMaterialization("run-1/1/1")
	if again.AssetRevisionBindings[0].RevisionID != "revision-1" {
		t.Fatalf("authoritative view aliased returned bindings: %#v", again)
	}
}

func TestP3AEvolutionAssetReplayRejectsUnknownVersionAndDuplicateFields(t *testing.T) {
	validPayload := map[string]any{
		"schema_version": 1, "operation_id": "operation-1",
		"asset_kind": "skill", "definition_id": "skill-1", "name": "Skill",
		"description": "safe", "scope": "project",
	}
	data, err := json.Marshal(validPayload)
	if err != nil {
		t.Fatal(err)
	}
	base := journal.Event{
		ID: "asset-create", StreamID: "evolution-asset/skill-1", Seq: 1,
		IdempotencyKey: "asset-create", Type: "EvolutionAssetDefinitionCreated",
		SchemaVersion: 1, EmittedAt: time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC),
		CorrelationID: "123e4567-e89b-42d3-a456-426614174000", PayloadJSON: data,
	}
	for _, mutate := range []func(*journal.Event){
		func(event *journal.Event) { event.SchemaVersion = 2 },
		func(event *journal.Event) {
			event.PayloadJSON = []byte(`{"schema_version":1,"schema_version":1,"operation_id":"operation-1"}`)
		},
	} {
		event := base
		mutate(&event)
		projection := newForTestSource(eventSliceSource{events: []journal.Event{event}})
		if err := projection.Rebuild(context.Background()); err == nil {
			t.Fatalf("Rebuild(%s) error = nil", event.PayloadJSON)
		}
	}
}
