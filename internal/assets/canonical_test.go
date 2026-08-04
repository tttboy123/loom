package assets_test

import (
	"bytes"
	"testing"

	"loom-pi-rebuild/internal/assets"
)

func TestP3ACanonicalAssetRevisionSetGoldenBytesAndDigest(t *testing.T) {
	bindings := []assets.ExactAssetRevisionBinding{
		{AssetKind: assets.AssetKindTeamTemplate, DefinitionID: "team-a", RevisionID: "rev-2", SHA256Digest: digestB, SourceScope: assets.SourceScopeImported},
		{AssetKind: assets.AssetKindSkill, DefinitionID: "skill-a", RevisionID: "rev-1", SHA256Digest: digestA, SourceScope: assets.SourceScopeLocal},
	}
	wantJSON := []byte(`[{"asset_kind":"skill","definition_id":"skill-a","revision_id":"rev-1","sha256_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","source_scope":"local"},{"asset_kind":"team_template","definition_id":"team-a","revision_id":"rev-2","sha256_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","source_scope":"imported"}]`)
	gotJSON, err := assets.CanonicalAssetRevisionSetJSON(bindings)
	if err != nil {
		t.Fatalf("CanonicalAssetRevisionSetJSON() error = %v", err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("canonical set JSON = %s, want %s", gotJSON, wantJSON)
	}
	digest, err := assets.CanonicalAssetRevisionSetDigest(bindings)
	if err != nil {
		t.Fatalf("CanonicalAssetRevisionSetDigest() error = %v", err)
	}
	if want := "21c2952c01757271e7237c3b64b889938c62c8ddec15a39e7a8a4af584a9383c"; digest != want {
		t.Fatalf("canonical set digest = %s, want %s", digest, want)
	}
	bindings[0].DefinitionID = "mutated"
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatal("canonical output aliased caller input")
	}
}

func TestP3ACanonicalSubjectIdentityGoldenBytesAndDigest(t *testing.T) {
	subject := validSubject()
	wantJSON := []byte(`{"schema_version":1,"subject_kind":"agent_definition","subject_id":"agent-1","subject_version":1,"subject_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","subject_scope":"transient","subject_project_id":"","subject_generation_id":"generation-1"}`)
	gotJSON, err := assets.CanonicalSubjectIdentityJSON(subject)
	if err != nil {
		t.Fatalf("CanonicalSubjectIdentityJSON() error = %v", err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("canonical subject JSON = %s, want %s", gotJSON, wantJSON)
	}
	digest, err := assets.CanonicalSubjectIdentityDigest(subject)
	if err != nil {
		t.Fatalf("CanonicalSubjectIdentityDigest() error = %v", err)
	}
	if want := "999c744745f1ce0502c9650f60855b4cc48a421a72d0608d57502585ad160a6c"; digest != want {
		t.Fatalf("canonical subject digest = %s, want %s", digest, want)
	}
}

func TestP3ACanonicalAssetRevisionSetRejectsDuplicatesAndWrongDigests(t *testing.T) {
	binding := validBinding()
	for _, bindings := range [][]assets.ExactAssetRevisionBinding{
		{binding, binding},
		{{AssetKind: assets.AssetKindSkill, DefinitionID: "skill-1", RevisionID: "revision-1", SHA256Digest: "ABC", SourceScope: assets.SourceScopeLocal}},
	} {
		if _, err := assets.CanonicalAssetRevisionSetDigest(bindings); err == nil {
			t.Fatalf("CanonicalAssetRevisionSetDigest(%#v) error = nil", bindings)
		}
	}
}

func TestP3AEventIdentityGoldenSeedsUseReviewedLiteralCodes(t *testing.T) {
	tests := []struct {
		eventType string
		streamID  string
		operation string
		intent    string
		index     int
		eventID   string
		key       string
	}{
		{
			eventType: "EvolutionTemplateInstantiated",
			streamID:  "evolution-asset-revision/team_template/team.demo/rev.1",
			operation: "op-template-001", intent: digestA, index: 0,
			eventID: "p3a-template_instantiated-a62662704f64e9a4cad50ebea4ba3c00",
			key:     "p3a/op-template-001/template_instantiated/0",
		},
		{
			eventType: "EvolutionRunPromotionProposed",
			streamID:  "evolution-asset-candidate/candidate.demo",
			operation: "op-promotion-001", intent: digestB, index: 3,
			eventID: "p3a-run_promotion_proposed-f49957f1cbcf6b4331a8f75a7be855bc",
			key:     "p3a/op-promotion-001/run_promotion_proposed/3",
		},
	}
	for _, test := range tests {
		eventID, key, err := assets.EventIdentity(
			test.eventType, test.streamID, test.operation, test.intent, test.index,
		)
		if err != nil {
			t.Fatalf("EventIdentity(%s) error = %v", test.eventType, err)
		}
		if eventID != test.eventID || key != test.key {
			t.Fatalf("EventIdentity(%s) = %q, %q; want %q, %q", test.eventType, eventID, key, test.eventID, test.key)
		}
	}
	if _, _, err := assets.EventIdentity("EvolutionUnknown", "stream", "operation", digestA, 0); err == nil {
		t.Fatal("EventIdentity(unknown) error = nil")
	}
}
