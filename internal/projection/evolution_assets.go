package projection

import (
	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/journal"
)

type EvolutionAssetSnapshot = assets.Snapshot

func replayEvolutionAssetEvents(events []journal.Event) (EvolutionAssetSnapshot, error) {
	return assets.Replay(events)
}

func cloneEvolutionAssetSnapshot(input assets.Snapshot) assets.Snapshot {
	output := assets.Snapshot{
		Definitions:      map[string]assets.SkillDefinition{},
		Revisions:        map[string]assets.SkillRevision{},
		Candidates:       map[string]assets.EvolutionCandidate{},
		Evaluations:      map[string]assets.EvaluationRecord{},
		Bindings:         map[string]assets.EvolutionAssetBindingRecord{},
		Materializations: map[string]assets.RuntimeSkillMaterializationRecord{},
	}
	for key, record := range input.Definitions {
		output.Definitions[key] = record
	}
	for key, record := range input.Revisions {
		record.Dependencies = append([]string(nil), record.Dependencies...)
		record.CompatibleRuntimeCapabilities = append([]string(nil), record.CompatibleRuntimeCapabilities...)
		output.Revisions[key] = record
	}
	for key, record := range input.Candidates {
		record.SourceEvidenceIDs = append([]string(nil), record.SourceEvidenceIDs...)
		record.SourceEvidenceDigests = append([]string(nil), record.SourceEvidenceDigests...)
		record.RequiredEvaluationIDs = append([]string(nil), record.RequiredEvaluationIDs...)
		output.Candidates[key] = record
	}
	for key, record := range input.Evaluations {
		output.Evaluations[key] = record
	}
	for key, record := range input.Bindings {
		record.Bindings = append([]assets.ExactAssetRevisionBinding(nil), record.Bindings...)
		output.Bindings[key] = record
	}
	for key, record := range input.Materializations {
		output.Materializations[key] = record
	}
	return output
}
