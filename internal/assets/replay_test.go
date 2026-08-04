package assets_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/journal"
)

func TestP3AReplayRebuildsExactSnapshotFromCommittedFacts(t *testing.T) {
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{})
	command := validCreateCommand("create_skill")
	command.OperationID = "replay-exact"
	if _, err := authority.CreateSkill(context.Background(), command); err != nil {
		t.Fatalf("CreateSkill() error = %v", err)
	}
	snapshot, err := assets.Replay(readAllEvents(t, store))
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if snapshot.Definitions["skill/skill-1"].DefinitionID != "skill-1" ||
		snapshot.Revisions["skill/skill-1/revision-1"].RevisionID != "revision-1" ||
		snapshot.Candidates["candidate-1"].CandidateID != "candidate-1" {
		t.Fatalf("replayed snapshot = %#v", snapshot)
	}
}

func TestP3AReplayRejectsInvalidEnumsAndTemplatePayloads(t *testing.T) {
	// A revision fact with an unknown risk must fail rebuild.
	badRevision := validReplayEvent("EvolutionAssetRevisionCreated", map[string]any{
		"schema_version": 1, "operation_id": "replay-enum",
		"asset_kind": "skill", "definition_id": "skill-1",
		"revision_id": "revision-1", "artifact_digest": digestA,
		"content_digest": digestA, "source_scope": "local",
		"source_reference_digest": digestA, "provenance_digest": digestA,
		"dependencies": []string{}, "compatible_runtime_capabilities": []string{},
		"risk": "extreme", "lifecycle": "candidate",
	})
	if _, err := assets.Replay([]journal.Event{badRevision}); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("invalid risk replay error = %v, want ErrInvalidInput", err)
	}
	// A template fact whose revision_digest does not match the revision must fail.
	validRevision := validReplayEvent("EvolutionAssetRevisionCreated", map[string]any{
		"schema_version": 1, "operation_id": "replay-enum",
		"asset_kind": "team_template", "definition_id": "team.1",
		"revision_id": "rev.1", "artifact_digest": digestA,
		"content_digest": digestA, "source_scope": "local",
		"source_reference_digest": digestA, "provenance_digest": digestA,
		"dependencies": []string{}, "compatible_runtime_capabilities": []string{},
		"risk": "low", "lifecycle": "candidate",
	})
	badTemplate := validReplayEvent("EvolutionTemplateInstantiated", map[string]any{
		"schema_version": 1, "operation_id": "replay-enum",
		"asset_kind": "team_template", "definition_id": "team.1",
		"revision_id": "rev.1", "revision_digest": digestB,
		"template_output": "team_draft", "parameter_digest": digestA,
		"output_candidate_id": "output.1", "output_digest": digestA,
	})
	if _, err := assets.Replay([]journal.Event{validRevision, badTemplate}); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("mismatched template digest replay error = %v, want ErrInvalidInput", err)
	}
	badEvaluation := validReplayEvent("EvolutionAssetEvaluationRecorded", map[string]any{
		"schema_version": 1, "operation_id": "replay-enum",
		"evaluation_id": "eval.1", "candidate_id": "candidate-1",
		"fixture_kind": "synthetic", "fixture_digest": digestA,
		"baseline_revision_id": "revision-previous", "baseline_digest": digestB,
		"candidate_revision_id": "revision-1", "candidate_digest": digestA,
		"quality_result": "garbage", "failure_count": 0, "case_count": 1,
		"usage_observed": false, "usage_microunits": 0,
		"cost_observed": false, "cost_microunits": 0, "cost_currency": "",
		"compatibility_result": "compatible", "applicable_scope": "fixture",
		"regression_result": "equivalent", "security_result": "pass",
		"evidence_id": "evidence-1", "evidence_digest": digestA,
	})
	if _, err := assets.Replay([]journal.Event{badEvaluation}); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("invalid quality-result replay error = %v, want ErrInvalidInput", err)
	}
	badArchive := validReplayEvent("EvolutionAssetRevisionArchived", map[string]any{
		"schema_version": 1, "operation_id": "replay-enum",
		"asset_kind": "skill", "definition_id": "skill-1",
		"revision_id": "revision-1", "revision_digest": digestA,
		"previous_lifecycle": "draft", "reason_code": "archive_requested",
	})
	if _, err := assets.Replay([]journal.Event{badArchive}); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("invalid previous_lifecycle replay error = %v, want ErrInvalidInput", err)
	}
	unobservedWithValue := validReplayEvent("EvolutionAssetEvaluationRecorded", map[string]any{
		"schema_version": 1, "operation_id": "replay-enum",
		"evaluation_id": "eval.2", "candidate_id": "candidate-1",
		"fixture_kind": "synthetic", "fixture_digest": digestA,
		"baseline_revision_id": "revision-previous", "baseline_digest": digestB,
		"candidate_revision_id": "revision-1", "candidate_digest": digestA,
		"quality_result": "pass", "failure_count": 0, "case_count": 1,
		"usage_observed": false, "usage_microunits": 5,
		"cost_observed": false, "cost_microunits": 0, "cost_currency": "",
		"compatibility_result": "compatible", "applicable_scope": "fixture",
		"regression_result": "equivalent", "security_result": "pass",
		"evidence_id": "evidence-2", "evidence_digest": digestA,
	})
	if _, err := assets.Replay([]journal.Event{unobservedWithValue}); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("unobserved-with-value replay error = %v, want ErrInvalidInput", err)
	}
	badMaterialization := validReplayEvent("RuntimeSkillMaterializationPublished", map[string]any{
		"schema_version": 1, "operation_id": "replay-enum",
		"team_execution_id": "team-1", "logical_node_id": "main",
		"run_id": "run-1", "attempt_number": 1, "generation": 1,
		"runtime_instance_id":     "runtime-1",
		"runtime_identity_digest": "not-a-digest",
		"capability":              "loom.skill-materialization.pi.v1",
		"asset_revision_bindings": []any{map[string]any{
			"asset_kind": "skill", "definition_id": "skill-1",
			"revision_id": "revision-1", "sha256_digest": digestA,
			"source_scope": "local",
		}},
		"asset_revision_set_digest":   digestA,
		"manifest_artifact_digest":    digestA,
		"materialization_root_digest": digestA,
	})
	if _, err := assets.Replay([]journal.Event{badMaterialization}); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("bad materialization fact replay error = %v, want ErrInvalidInput", err)
	}
}

func TestP3AReplayRejectsLocalCandidateWithPromotedFieldsOrDigestMismatch(t *testing.T) {
	localWithPromoted := validReplayEvent("EvolutionAssetCandidateCreated", map[string]any{
		"schema_version": 1, "operation_id": "replay-enum",
		"candidate_id": "candidate-1", "asset_kind": "skill",
		"definition_id": "skill-1", "revision_id": "revision-1",
		"source_scope": "local", "source_run_id": "run-1",
		"source_run_generation": 1, "source_run_digest": digestA,
		"source_evidence_ids": []string{}, "source_evidence_digests": []string{},
		"redacted_summary": "summary", "redacted_summary_digest": digestA,
		"scope_difference": "", "expected_benefit": "", "risk": "low",
		"required_evaluation_ids": []string{},
	})
	if _, err := assets.Replay([]journal.Event{localWithPromoted}); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("local candidate with promoted fields replay error = %v, want ErrInvalidInput", err)
	}
	badDigest := validReplayEvent("EvolutionAssetCandidateCreated", map[string]any{
		"schema_version": 1, "operation_id": "replay-enum",
		"candidate_id": "candidate-1", "asset_kind": "skill",
		"definition_id": "skill-1", "revision_id": "revision-1",
		"source_scope": "local", "source_run_id": "",
		"source_run_generation": 0, "source_run_digest": "",
		"source_evidence_ids": []string{}, "source_evidence_digests": []string{},
		"redacted_summary": "summary", "redacted_summary_digest": digestB,
		"scope_difference": "", "expected_benefit": "", "risk": "low",
		"required_evaluation_ids": []string{},
	})
	if _, err := assets.Replay([]journal.Event{badDigest}); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("mismatched redacted summary digest replay error = %v, want ErrInvalidInput", err)
	}
}

func validReplayEvent(eventType string, payload map[string]any) journal.Event {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return journal.Event{
		ID: eventType + "-id", SchemaVersion: 1, Type: eventType,
		StreamID: "stream", Seq: 1, EmittedAt: time.Now().UTC(),
		CorrelationID: "123e4567-e89b-42d3-a456-426614174000",
		PayloadJSON:   data,
	}
}
