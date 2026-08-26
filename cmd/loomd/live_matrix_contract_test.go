package main

import (
	"reflect"
	"sort"
	"testing"
)

func TestMatrixBindingFreezesCurrentProfileAccountAndPolicy(t *testing.T) {
	profile := matrixProfile{
		ProfileID:         "conversation-deepseek-primary-r19",
		ProviderID:        "deepseek",
		ProviderAccountID: "deepseek.primary",
		PolicyVersion:     1,
		PolicyRevision:    7,
		PolicyDigest:      "sha256:policy",
		TrustDomain:       "external-provider",
		RetentionMode:     "provider-default",
		DataRegion:        "global",
	}

	binding := bindingForMatrixProfile(profile)
	if binding.SchemaVersion != 3 ||
		binding.ProviderID != profile.ProviderID ||
		binding.ProviderAccountID != profile.ProviderAccountID ||
		binding.ProviderAccountPolicyVersion != profile.PolicyVersion ||
		binding.ProviderAccountPolicyRevision != profile.PolicyRevision ||
		binding.ProviderAccountPolicyDigest != profile.PolicyDigest ||
		binding.TrustDomain != profile.TrustDomain ||
		binding.RetentionMode != profile.RetentionMode ||
		binding.DataRegion != profile.DataRegion {
		t.Fatalf("binding did not freeze the current profile: %#v", binding)
	}
}

func TestRequiredMatrixFailuresRejectTheGate(t *testing.T) {
	results := []matrixResult{
		{profile: "conversation-a", model: "model-a", effort: "low", passed: true},
		{profile: "conversation-b", model: "model-b", effort: "high", passed: false, summary: "provider timeout"},
		{profile: "conversation-c", model: "model-c", passed: false, summary: "unexpected reply"},
	}

	want := []string{
		"conversation-b/model-b/high: provider timeout",
		"conversation-c/model-c/default: unexpected reply",
	}
	if got := requiredMatrixFailures(results); !reflect.DeepEqual(got, want) {
		t.Fatalf("required failures = %#v, want %#v", got, want)
	}
}

func TestMatrixProfileSelectionUsesCurrentDirectoryRevision(t *testing.T) {
	profiles := []matrixProfile{
		{ProfileID: "conversation-opencode-default-v1", ProviderID: "opencode"},
		{ProfileID: "conversation-deepseek-primary-r19", ProviderID: "deepseek", ProviderAccountID: "deepseek.primary"},
	}

	profile, ok := matrixProfileForProvider(profiles, "deepseek")
	if !ok {
		t.Fatal("current DeepSeek profile was not selected")
	}
	if profile.ProfileID != "conversation-deepseek-primary-r19" {
		t.Fatalf("profile id = %q", profile.ProfileID)
	}
	if _, ok := matrixProfileForProvider(profiles, "minimax"); ok {
		t.Fatal("missing Provider unexpectedly resolved")
	}
}

func TestExactMatrixReplyRejectsQuotedOrRefusedMarker(t *testing.T) {
	if !exactMatrixReply("loom", " 42 ", "42") {
		t.Fatal("exact bounded reply was rejected")
	}
	if exactMatrixReply("loom", `I will not reply with "42".`, "42") {
		t.Fatal("quoted marker must not satisfy the live gate")
	}
	if exactMatrixReply("user", "42", "42") {
		t.Fatal("non-Loom role must not satisfy the live gate")
	}
}

func TestLiveBuilderConfirmParamsMatchClosedIPCShape(t *testing.T) {
	params := liveBuilderConfirmParams(builderSession{
		DraftID: "draft-live", Revision: 9,
		CatalogDg: "catalog-digest", ViewVer: "view-12",
		BindingDg: "binding-digest",
	}, "team-live")
	wantKeys := []string{
		"binding_digest", "catalog_digest", "confirm", "definition_id",
		"draft_id", "expected_revision", "project_id", "scope", "view_version",
	}
	gotKeys := make([]string, 0, len(params))
	for key := range params {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) || params["project_id"] != "" {
		t.Fatalf("confirm params = %#v", params)
	}
}
