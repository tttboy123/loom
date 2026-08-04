package piadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestP3APrivateSkillMaterializerDeclaresAtomicLifecycle(t *testing.T) {
	parsed, err := parser.ParseFile(
		token.NewFileSet(), "skill_materialization.go", nil, parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("parse required Pi materializer: %v", err)
	}
	declared := make(map[string]bool)
	for _, declaration := range parsed.Decls {
		switch value := declaration.(type) {
		case *ast.GenDecl:
			for _, specification := range value.Specs {
				if named, ok := specification.(*ast.TypeSpec); ok {
					declared[named.Name.Name] = true
				}
			}
		case *ast.FuncDecl:
			declared[value.Name.Name] = true
		}
	}
	for _, name := range []string{
		"SkillMaterializer",
		"NewSkillMaterializer",
		"Materialize",
		"Recover",
		"Cleanup",
	} {
		if !declared[name] {
			t.Fatalf("Pi materializer declaration %s is missing", name)
		}
	}
}

func TestP3AIncompatibleRuntimeCannotMaterialize(t *testing.T) {
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	plan := validP3AMaterializationPlan(t)
	plan.RuntimeCapabilities = []string{"loom.bridge.v1"}
	if _, err := materializer.Materialize(context.Background(), plan); !errors.Is(err, ErrMaterializationCapabilityGap) {
		t.Fatalf("Materialize() error = %v, want capability gap", err)
	}
	if _, err := os.Stat(expectedP3ATarget(plan)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("incompatible materialization target exists: %v", err)
	}
}

func TestP3APrivateDescriptorValidationAcceptsOwnedModes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := validatePrivateDirectory(root); err != nil {
		t.Fatalf("validatePrivateDirectory() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "fixture"), []byte("exact"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := readPrivateMaterializationFile(root, "fixture", 5)
	if err != nil || string(data) != "exact" {
		t.Fatalf("readPrivateMaterializationFile() = %q, %v", data, err)
	}
}

func TestP3ARepositoryAndUserSkillsAreNeverOverwritten(t *testing.T) {
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	plan := validP3AMaterializationPlan(t)
	target := expectedP3ATarget(plan)
	if err := os.MkdirAll(filepath.Join(target, "skills", "skill-1", "revision-1"), 0o700); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(target, "skills", "skill-1", "revision-1", "SKILL.md")
	if err := os.WriteFile(protected, []byte("user-owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Materialize(context.Background(), plan); !errors.Is(err, ErrMaterializationCollision) {
		t.Fatalf("Materialize() error = %v, want collision", err)
	}
	data, err := os.ReadFile(protected)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "user-owned" {
		t.Fatalf("protected Skill overwritten with %q", data)
	}
}

func TestP3AFinalPublicationNeverReplacesEmptyDirectoryCreatedByRacer(t *testing.T) {
	plan := validP3AMaterializationPlan(t)
	target := expectedP3ATarget(plan)
	materializer, err := NewSkillMaterializer(MaterializationHooks{
		BeforePublish: func(string) error {
			return os.MkdirAll(target, 0o700)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Materialize(context.Background(), plan); !errors.Is(err, ErrMaterializationCollision) {
		t.Fatalf("Materialize() error = %v, want collision", err)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("racing empty directory was replaced: %#v", entries)
	}
}

func TestP3AMaterializationRejectsSymlinkAncestorBelowIsolatedRoot(t *testing.T) {
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	plan := validP3AMaterializationPlan(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(plan.IsolatedRoot, "materialized")); err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Materialize(context.Background(), plan); !errors.Is(err, ErrMaterializationIdentityDrift) {
		t.Fatalf("Materialize() error = %v, want identity drift", err)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("symlink target mutated: entries=%v err=%v", entries, err)
	}
}

func TestP3ARecoveryRejectsHardLinkedMaterializedFile(t *testing.T) {
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	plan := validP3AMaterializationPlan(t)
	result, err := materializer.Materialize(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(result.Root, "skills", "skill-1", "revision-1", "SKILL.md")
	if err := os.Link(skillPath, filepath.Join(plan.IsolatedRoot, "linked-skill")); err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Recover(context.Background(), plan); !errors.Is(err, ErrMaterializationIdentityDrift) {
		t.Fatalf("Recover() error = %v, want identity drift", err)
	}
}

func TestP3ARecoveryRejectsUnmanifestedTreeEntries(t *testing.T) {
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	plan := validP3AMaterializationPlan(t)
	result, err := materializer.Materialize(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(result.Root, "unexpected.txt"), []byte("extra"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Recover(context.Background(), plan); !errors.Is(err, ErrMaterializationIdentityDrift) {
		t.Fatalf("Recover() error = %v, want identity drift", err)
	}
}

func TestP3AExecutionLoadsOnlyExactRunGenerationMaterialization(t *testing.T) {
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	plan := validP3AMaterializationPlan(t)
	result, err := materializer.Materialize(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	root, err := executionMaterializationSkillRoot(
		result.Root, plan.RunID, plan.Generation, plan.RuntimeInstanceID,
	)
	if err != nil || root != filepath.Join(result.Root, "skills") {
		t.Fatalf("execution skill root = %q, %v", root, err)
	}
	if _, err := executionMaterializationSkillRoot(
		result.Root, plan.RunID, plan.Generation+1, plan.RuntimeInstanceID,
	); !errors.Is(err, ErrMaterializationIdentityDrift) {
		t.Fatalf("stale generation error = %v", err)
	}
	plain := t.TempDir()
	if err := os.Mkdir(filepath.Join(plain, "skills"), 0o700); err != nil {
		t.Fatal(err)
	}
	root, err = executionMaterializationSkillRoot(
		plain, plan.RunID, plan.Generation, plan.RuntimeInstanceID,
	)
	if err != nil || root != "" {
		t.Fatalf("plain source enabled skills: root=%q err=%v", root, err)
	}
}

func TestP3APartialMaterializationIsAtomicAndRestartRecoverable(t *testing.T) {
	interrupted := errors.New("injected crash before publish")
	materializer, err := NewSkillMaterializer(MaterializationHooks{
		BeforePublish: func(string) error { return interrupted },
	})
	if err != nil {
		t.Fatal(err)
	}
	plan := validP3AMaterializationPlan(t)
	if _, err := materializer.Materialize(context.Background(), plan); !errors.Is(err, interrupted) {
		t.Fatalf("interrupted Materialize() error = %v", err)
	}
	if _, err := os.Stat(expectedP3ATarget(plan)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial target published: %v", err)
	}
	recoveredMaterializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := recoveredMaterializer.Recover(context.Background(), plan)
	if err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	if result.Root != expectedP3ATarget(plan) || result.ManifestArtifactDigest == "" {
		t.Fatalf("Recover() result = %#v", result)
	}
	assertP3AMaterializedModes(t, result)
	second, err := recoveredMaterializer.Recover(context.Background(), plan)
	if err != nil {
		t.Fatalf("second Recover() error = %v", err)
	}
	if !second.Reused || second.ManifestArtifactDigest != result.ManifestArtifactDigest {
		t.Fatalf("second Recover() = %#v, want exact reuse", second)
	}
}

func TestP3AStartupReconcileRemovesVerifiedPreCASRootAndRebuildsCommittedRoot(t *testing.T) {
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	orphan := validP3AMaterializationPlan(t)
	if _, err := materializer.Materialize(context.Background(), orphan); err != nil {
		t.Fatal(err)
	}
	committed := orphan
	committed.RunID = "run-committed"
	committed.OperationID = "materialize-committed"
	results, err := materializer.Reconcile(
		context.Background(), orphan.IsolatedRoot, []MaterializationPlan{committed},
	)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if _, err := os.Lstat(expectedP3ATarget(orphan)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("verified pre-CAS orphan remains: %v", err)
	}
	if len(results) != 1 || results[0].Root != materializationTarget(committed) || results[0].Reused {
		t.Fatalf("reconciled committed root = %#v", results)
	}
}

func TestP3AStartupReconcileRejectsCorruptForeignRoot(t *testing.T) {
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	plan := validP3AMaterializationPlan(t)
	root := expectedP3ATarget(plan)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(plan.IsolatedRoot, "materialized"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(plan.IsolatedRoot, "materialized", "run-"+plan.RunID), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(root), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Reconcile(context.Background(), plan.IsolatedRoot, nil); err == nil {
		t.Fatal("Reconcile() accepted corrupt foreign root")
	}
	if _, err := os.Lstat(root); err != nil {
		t.Fatalf("foreign root was deleted instead of preserved for review: %v", err)
	}
}

func TestP3AStaleGenerationCleanupCannotDeleteCurrentMaterialization(t *testing.T) {
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	plan := validP3AMaterializationPlan(t)
	result, err := materializer.Materialize(context.Background(), plan)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	stale := plan
	stale.Generation--
	if err := materializer.Cleanup(context.Background(), stale); !errors.Is(err, ErrMaterializationIdentityDrift) {
		t.Fatalf("stale Cleanup() error = %v, want identity drift", err)
	}
	if _, err := os.Stat(result.Root); err != nil {
		t.Fatalf("current materialization removed by stale cleanup: %v", err)
	}
	if err := materializer.Cleanup(context.Background(), plan); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if _, err := os.Stat(result.Root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("current root remains after cleanup: %v", err)
	}
}

func TestP3ARetryAttemptPinsSameRevisionSetWithDistinctManifestAndRoot(t *testing.T) {
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	firstPlan := validP3AMaterializationPlan(t)
	first, err := materializer.Materialize(context.Background(), firstPlan)
	if err != nil {
		t.Fatal(err)
	}
	secondPlan := firstPlan
	secondPlan.AttemptNumber = 2
	secondPlan.OperationID = "materialize-attempt-2"
	second, err := materializer.Materialize(context.Background(), secondPlan)
	if err != nil {
		t.Fatal(err)
	}
	if firstPlan.AssetRevisionSetDigest != secondPlan.AssetRevisionSetDigest {
		t.Fatal("retry changed the pinned exact revision set")
	}
	if first.ManifestArtifactDigest == second.ManifestArtifactDigest ||
		first.MaterializationRootDigest == second.MaterializationRootDigest {
		t.Fatalf("attempt lineage reused manifest/root: first=%#v second=%#v", first, second)
	}
}

func TestP3AManifestContainsNoSecretGrantPromptReasoningOrTokenOutput(t *testing.T) {
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	plan := validP3AMaterializationPlan(t)
	result, err := materializer.Materialize(context.Background(), plan)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	manifest, err := os.ReadFile(result.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"sk-secret", "raw_grant", "sensitive_prompt", "hidden_reasoning", "token_delta"} {
		if bytesContain(manifest, []byte(forbidden)) {
			t.Fatalf("manifest disclosed %q: %s", forbidden, manifest)
		}
	}
}

func validP3AMaterializationPlan(t testing.TB) MaterializationPlan {
	t.Helper()
	content := []byte("# reviewed skill\n")
	digest := sha256.Sum256(content)
	digestText := hex.EncodeToString(digest[:])
	bindings := []MaterializationBinding{{
		AssetKind: "skill", DefinitionID: "skill-1", RevisionID: "revision-1", Digest: digestText,
		ContentDigest: digestText, SourceScope: "local",
		Files: []MaterializationFile{{RelativePath: "SKILL.md", Bytes: content, Digest: digestText}},
	}}
	setDigest, err := materializationBindingSetDigest(bindings)
	if err != nil {
		t.Fatal(err)
	}
	return MaterializationPlan{
		IsolatedRoot: t.TempDir(), RunID: "run-1", AttemptNumber: 1,
		Generation: 2, RuntimeInstanceID: "pi-runtime-1",
		RuntimeIdentityDigest:  digestText,
		RuntimeCapabilities:    []string{SkillMaterializationCapability},
		AssetRevisionSetDigest: setDigest, ExpectedManifestDigest: "",
		JourneyID:   "123e4567-e89b-42d3-a456-426614174000",
		OperationID: "materialize-1",
		Bindings:    bindings,
	}
}

func expectedP3ATarget(plan MaterializationPlan) string {
	return filepath.Join(
		plan.IsolatedRoot, "materialized", "run-"+plan.RunID,
		"attempt-1", "generation-2",
	)
}

func assertP3AMaterializedModes(t testing.TB, result MaterializationResult) {
	t.Helper()
	rootInfo, err := os.Stat(result.Root)
	if err != nil {
		t.Fatal(err)
	}
	if rootInfo.Mode().Perm() != 0o700 {
		t.Fatalf("root mode = %o, want 0700", rootInfo.Mode().Perm())
	}
	manifestInfo, err := os.Stat(result.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if manifestInfo.Mode().Perm() != 0o600 {
		t.Fatalf("manifest mode = %o, want 0600", manifestInfo.Mode().Perm())
	}
}

func bytesContain(data, fragment []byte) bool {
	if len(fragment) == 0 {
		return true
	}
	for index := 0; index+len(fragment) <= len(data); index++ {
		match := true
		for offset := range fragment {
			if data[index+offset] != fragment[offset] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
