package piadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

var (
	ErrInvalidMaterialization        = errors.New("invalid Pi skill materialization")
	ErrMaterializationCapabilityGap  = errors.New("Pi skill materialization capability gap")
	ErrMaterializationCollision      = errors.New("Pi skill materialization collision")
	ErrMaterializationIdentityDrift  = errors.New("Pi skill materialization identity drift")
	ErrMaterializationDigestMismatch = errors.New("Pi skill materialization digest mismatch")
)

const SkillMaterializationCapability = "loom.skill-materialization.pi.v1"

type MaterializationFile struct {
	RelativePath string
	Bytes        []byte
	Digest       string
}

type MaterializationBinding struct {
	AssetKind     string
	DefinitionID  string
	RevisionID    string
	Digest        string
	ContentDigest string
	SourceScope   string
	Files         []MaterializationFile
}

type MaterializationPlan struct {
	IsolatedRoot           string
	RunID                  string
	AttemptNumber          int
	Generation             int64
	RuntimeInstanceID      string
	RuntimeIdentityDigest  string
	RuntimeCapabilities    []string
	AssetRevisionSetDigest string
	ExpectedManifestDigest string
	JourneyID              string
	OperationID            string
	Bindings               []MaterializationBinding
}

type MaterializationResult struct {
	Root                      string
	ManifestPath              string
	ManifestArtifactDigest    string
	MaterializationRootDigest string
	Reused                    bool
}

type MaterializationHooks struct {
	BeforePublish func(string) error
}

type SkillMaterializer struct{ hooks MaterializationHooks }

type materializationManifest struct {
	SchemaVersion          int                              `json:"schema_version"`
	RunID                  string                           `json:"run_id"`
	AttemptNumber          int                              `json:"attempt_number"`
	Generation             int64                            `json:"generation"`
	RuntimeInstanceID      string                           `json:"runtime_instance_id"`
	RuntimeIdentityDigest  string                           `json:"runtime_identity_digest"`
	Capability             string                           `json:"capability"`
	AssetRevisionBindings  []materializationManifestBinding `json:"asset_revision_bindings"`
	AssetRevisionSetDigest string                           `json:"asset_revision_set_digest"`
	Entries                []materializationManifestEntry   `json:"entries"`
	ManifestDigest         string                           `json:"manifest_digest"`
}

type materializationManifestBinding struct {
	AssetKind    string `json:"asset_kind"`
	DefinitionID string `json:"definition_id"`
	RevisionID   string `json:"revision_id"`
	SHA256Digest string `json:"sha256_digest"`
	SourceScope  string `json:"source_scope"`
}

type materializationManifestEntry struct {
	AssetKind            string `json:"asset_kind"`
	DefinitionID         string `json:"definition_id"`
	RevisionID           string `json:"revision_id"`
	SourceArtifactDigest string `json:"source_artifact_digest"`
	ContentDigest        string `json:"content_digest"`
	TargetRelativePath   string `json:"target_relative_path"`
	FileMode             int    `json:"file_mode"`
	FileSize             int    `json:"file_size"`
	FileSHA256           string `json:"file_sha256"`
}

func NewSkillMaterializer(hooks MaterializationHooks) (*SkillMaterializer, error) {
	return &SkillMaterializer{hooks: hooks}, nil
}

func (materializer *SkillMaterializer) Materialize(
	ctx context.Context,
	plan MaterializationPlan,
) (MaterializationResult, error) {
	if err := validateMaterializationPlan(plan); err != nil {
		return MaterializationResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return MaterializationResult{}, err
	}
	target := materializationTarget(plan)
	if _, err := os.Lstat(target); err == nil {
		return MaterializationResult{}, ErrMaterializationCollision
	} else if !errors.Is(err, os.ErrNotExist) {
		return MaterializationResult{}, err
	}
	parent := filepath.Dir(target)
	if err := ensurePrivateDirectories(plan.IsolatedRoot, parent); err != nil {
		return MaterializationResult{}, err
	}
	temp, err := os.MkdirTemp(parent, ".loom-materializing-")
	if err != nil {
		return MaterializationResult{}, err
	}
	if err := os.Chmod(temp, 0o700); err != nil {
		_ = os.RemoveAll(temp)
		return MaterializationResult{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(temp)
		}
	}()
	manifest, err := writeMaterializationTree(ctx, temp, plan)
	if err != nil {
		return MaterializationResult{}, err
	}
	manifestDigest, err := calculateManifestDigest(manifest)
	if err != nil {
		return MaterializationResult{}, err
	}
	manifest.ManifestDigest = manifestDigest
	manifestBytes, err := canonicalManifest(manifest)
	if err != nil {
		return MaterializationResult{}, err
	}
	if plan.ExpectedManifestDigest != "" && plan.ExpectedManifestDigest != manifestDigest {
		return MaterializationResult{}, ErrMaterializationDigestMismatch
	}
	manifestPath := filepath.Join(temp, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o600); err != nil {
		return MaterializationResult{}, err
	}
	if err := verifyTree(temp, manifest); err != nil {
		return MaterializationResult{}, err
	}
	if err := syncMaterializationTree(temp, manifest); err != nil {
		return MaterializationResult{}, err
	}
	if materializer != nil && materializer.hooks.BeforePublish != nil {
		if err := materializer.hooks.BeforePublish(temp); err != nil {
			return MaterializationResult{}, err
		}
	}
	if err := publishMaterializationNoReplace(temp, target); err != nil {
		return MaterializationResult{}, err
	}
	if err := syncDirectory(parent); err != nil {
		return MaterializationResult{}, err
	}
	cleanup = false
	rootDigest, err := calculateMaterializationRootDigest(plan, manifestDigest)
	if err != nil {
		return MaterializationResult{}, err
	}
	return MaterializationResult{
		Root: target, ManifestPath: filepath.Join(target, "manifest.json"),
		ManifestArtifactDigest: digestBytes(manifestBytes), MaterializationRootDigest: rootDigest,
	}, nil
}

// publishMaterializationNoReplace is deliberately fail-closed. Darwin's
// renameatx_np(RENAME_EXCL) is the only publication primitive accepted by the
// Phase 3A Pi capability contract: it atomically publishes the verified tree
// while refusing every pre-existing target, including an empty directory.
// Platforms without that primitive expose a capability gap instead of falling
// back to replacement-capable os.Rename.
func publishMaterializationNoReplace(source, target string) error {
	if runtime.GOOS != "darwin" {
		return ErrMaterializationCapabilityGap
	}
	from, err := unix.BytePtrFromString(source)
	if err != nil {
		return ErrInvalidMaterialization
	}
	to, err := unix.BytePtrFromString(target)
	if err != nil {
		return ErrInvalidMaterialization
	}
	const (
		darwinRenameatxNp = uintptr(488)
		darwinRenameExcl  = uintptr(0x4)
		atFDCWD           = ^uintptr(1)
	)
	_, _, errno := unix.Syscall6(
		darwinRenameatxNp,
		atFDCWD,
		uintptr(unsafe.Pointer(from)),
		atFDCWD,
		uintptr(unsafe.Pointer(to)),
		darwinRenameExcl,
		0,
	)
	if errno == 0 {
		return nil
	}
	if errno == syscall.EEXIST || errno == syscall.ENOTEMPTY {
		return ErrMaterializationCollision
	}
	if errno == syscall.ENOSYS || errno == syscall.ENOTSUP {
		return ErrMaterializationCapabilityGap
	}
	return errno
}

func syncMaterializationTree(root string, manifest materializationManifest) error {
	for _, entry := range manifest.Entries {
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(entry.TargetRelativePath)))
		if err != nil {
			return err
		}
		syncErr := file.Sync()
		closeErr := file.Close()
		if syncErr != nil {
			return syncErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	manifestFile, err := os.Open(filepath.Join(root, "manifest.json"))
	if err != nil {
		return err
	}
	if err := manifestFile.Sync(); err != nil {
		_ = manifestFile.Close()
		return err
	}
	if err := manifestFile.Close(); err != nil {
		return err
	}
	return syncDirectory(root)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (materializer *SkillMaterializer) Recover(ctx context.Context, plan MaterializationPlan) (MaterializationResult, error) {
	if err := validateMaterializationPlan(plan); err != nil {
		return MaterializationResult{}, err
	}
	target := materializationTarget(plan)
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		return materializer.Materialize(ctx, plan)
	} else if err != nil {
		return MaterializationResult{}, err
	}
	manifest, manifestBytes, err := readAndValidateManifest(target, plan)
	if err != nil {
		return MaterializationResult{}, err
	}
	if err := verifyTree(target, manifest); err != nil {
		return MaterializationResult{}, err
	}
	if plan.ExpectedManifestDigest != "" && plan.ExpectedManifestDigest != manifest.ManifestDigest {
		return MaterializationResult{}, ErrMaterializationDigestMismatch
	}
	rootDigest, err := calculateMaterializationRootDigest(plan, manifest.ManifestDigest)
	if err != nil {
		return MaterializationResult{}, err
	}
	return MaterializationResult{Root: target, ManifestPath: filepath.Join(target, "manifest.json"), ManifestArtifactDigest: digestBytes(manifestBytes), MaterializationRootDigest: rootDigest, Reused: true}, nil
}

// Reconcile performs one bounded startup pass. It removes only verified
// uncommitted/stale Loom roots, rejects foreign or corrupt roots, and then
// reconstructs every authoritative current-generation root exactly once.
func (materializer *SkillMaterializer) Reconcile(
	ctx context.Context,
	isolatedRoot string,
	authoritative []MaterializationPlan,
) ([]MaterializationResult, error) {
	if materializer == nil || !filepath.IsAbs(isolatedRoot) ||
		filepath.Clean(isolatedRoot) != isolatedRoot {
		return nil, ErrInvalidMaterialization
	}
	known := make(map[string]struct{}, len(authoritative))
	for _, plan := range authoritative {
		if err := validateMaterializationPlan(plan); err != nil ||
			plan.IsolatedRoot != isolatedRoot {
			return nil, ErrInvalidMaterialization
		}
		target := materializationTarget(plan)
		if _, exists := known[target]; exists {
			return nil, ErrMaterializationCollision
		}
		known[target] = struct{}{}
	}
	if err := materializer.removeUncommittedMaterializations(ctx, isolatedRoot, known); err != nil {
		return nil, err
	}
	results := make([]MaterializationResult, len(authoritative))
	for index, plan := range authoritative {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result, err := materializer.Recover(ctx, plan)
		if err != nil {
			return nil, err
		}
		results[index] = result
	}
	return results, nil
}

func (materializer *SkillMaterializer) removeUncommittedMaterializations(
	ctx context.Context,
	isolatedRoot string,
	known map[string]struct{},
) error {
	root := filepath.Join(isolatedRoot, "materialized")
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return ErrMaterializationIdentityDrift
	}
	if err := validatePrivateDirectory(root); err != nil {
		return err
	}
	runEntries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, runEntry := range runEntries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if runEntry.Type()&os.ModeSymlink != 0 || !runEntry.IsDir() ||
			!strings.HasPrefix(runEntry.Name(), "run-") ||
			!cleanIdentifier(strings.TrimPrefix(runEntry.Name(), "run-")) {
			return ErrMaterializationIdentityDrift
		}
		runPath := filepath.Join(root, runEntry.Name())
		if err := validatePrivateDirectory(runPath); err != nil {
			return err
		}
		attemptEntries, err := os.ReadDir(runPath)
		if err != nil {
			return err
		}
		for _, attemptEntry := range attemptEntries {
			attemptNumber, valid := parsePositiveMaterializationNumber(
				attemptEntry.Name(), "attempt-",
			)
			if !valid || attemptEntry.Type()&os.ModeSymlink != 0 || !attemptEntry.IsDir() {
				return ErrMaterializationIdentityDrift
			}
			attemptPath := filepath.Join(runPath, attemptEntry.Name())
			if err := validatePrivateDirectory(attemptPath); err != nil {
				return err
			}
			generationEntries, err := os.ReadDir(attemptPath)
			if err != nil {
				return err
			}
			for _, generationEntry := range generationEntries {
				generationPath := filepath.Join(attemptPath, generationEntry.Name())
				if strings.HasPrefix(generationEntry.Name(), ".loom-materializing-") {
					if generationEntry.Type()&os.ModeSymlink != 0 || !generationEntry.IsDir() ||
						validatePrivateDirectory(generationPath) != nil {
						return ErrMaterializationIdentityDrift
					}
					if err := os.RemoveAll(generationPath); err != nil {
						return err
					}
					continue
				}
				generation, valid := parsePositiveMaterializationNumber(
					generationEntry.Name(), "generation-",
				)
				if !valid || generationEntry.Type()&os.ModeSymlink != 0 ||
					!generationEntry.IsDir() {
					return ErrMaterializationIdentityDrift
				}
				if _, authoritative := known[generationPath]; authoritative {
					continue
				}
				if err := validateUncommittedMaterializationRoot(
					generationPath,
					strings.TrimPrefix(runEntry.Name(), "run-"),
					attemptNumber,
					int64(generation),
				); err != nil {
					return err
				}
				if err := os.RemoveAll(generationPath); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func parsePositiveMaterializationNumber(value, prefix string) (int, bool) {
	if !strings.HasPrefix(value, prefix) {
		return 0, false
	}
	number, err := strconv.Atoi(strings.TrimPrefix(value, prefix))
	return number, err == nil && number > 0 && value == prefix+strconv.Itoa(number)
}

func validateUncommittedMaterializationRoot(
	root, runID string,
	attemptNumber int,
	generation int64,
) error {
	data, err := readPrivateMaterializationFile(root, "manifest.json", 1<<20)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest materializationManifest
	if err := decoder.Decode(&manifest); err != nil || decoder.Decode(&struct{}{}) == nil {
		return ErrInvalidMaterialization
	}
	canonical, err := canonicalManifest(manifest)
	digest, digestErr := calculateManifestDigest(manifest)
	if err != nil || digestErr != nil || !bytes.Equal(data, canonical) ||
		manifest.SchemaVersion != 1 || manifest.RunID != runID ||
		manifest.AttemptNumber != attemptNumber || manifest.Generation != generation ||
		!cleanIdentifier(manifest.RuntimeInstanceID) ||
		!validMaterializationDigest(manifest.RuntimeIdentityDigest) ||
		manifest.Capability != SkillMaterializationCapability ||
		!validMaterializationDigest(manifest.AssetRevisionSetDigest) ||
		manifest.ManifestDigest != digest || len(manifest.AssetRevisionBindings) == 0 {
		return ErrMaterializationIdentityDrift
	}
	return verifyTree(root, manifest)
}

func (materializer *SkillMaterializer) Cleanup(ctx context.Context, plan MaterializationPlan) error {
	if err := validateMaterializationPlan(plan); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	target := materializationTarget(plan)
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		generationParent := filepath.Dir(target)
		entries, readErr := os.ReadDir(generationParent)
		if readErr == nil && len(entries) > 0 {
			return ErrMaterializationIdentityDrift
		}
		if errors.Is(readErr, os.ErrNotExist) || readErr == nil {
			return nil
		}
		return readErr
	} else if err != nil {
		return err
	}
	manifest, _, err := readAndValidateManifest(target, plan)
	if err != nil {
		return err
	}
	if err := verifyTree(target, manifest); err != nil {
		return err
	}
	return os.RemoveAll(target)
}

// executionMaterializationSkillRoot recognizes only a complete Loom-produced
// materialization copied into the managed workspace for this exact Run and
// generation. Ordinary source trees, including repositories that happen to
// contain a skills directory, never enable Pi skill loading.
func executionMaterializationSkillRoot(
	workspacePath, runID string,
	generation int64,
	runtimeInstanceID string,
) (string, error) {
	manifestPath := filepath.Join(workspacePath, "manifest.json")
	if _, err := os.Lstat(manifestPath); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", ErrMaterializationIdentityDrift
	}
	data, err := readPrivateMaterializationFile(workspacePath, "manifest.json", 1<<20)
	if err != nil {
		return "", ErrMaterializationIdentityDrift
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest materializationManifest
	if err := decoder.Decode(&manifest); err != nil || decoder.Decode(&struct{}{}) == nil {
		return "", ErrInvalidMaterialization
	}
	canonical, err := canonicalManifest(manifest)
	if err != nil || !bytes.Equal(data, canonical) {
		return "", ErrInvalidMaterialization
	}
	digest, err := calculateManifestDigest(manifest)
	if err != nil || digest != manifest.ManifestDigest ||
		manifest.SchemaVersion != 1 || manifest.RunID != runID ||
		manifest.Generation != generation ||
		manifest.RuntimeInstanceID != runtimeInstanceID ||
		manifest.Capability != SkillMaterializationCapability {
		return "", ErrMaterializationIdentityDrift
	}
	if err := verifyTree(workspacePath, manifest); err != nil {
		return "", err
	}
	skillRoot := filepath.Join(workspacePath, "skills")
	info, err := os.Lstat(skillRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrMaterializationIdentityDrift
	}
	return skillRoot, nil
}

func validateMaterializationPlan(plan MaterializationPlan) error {
	if plan.IsolatedRoot == "" || !cleanIdentifier(plan.RunID) || plan.AttemptNumber < 1 ||
		plan.Generation < 1 || !cleanIdentifier(plan.RuntimeInstanceID) ||
		!validMaterializationDigest(plan.RuntimeIdentityDigest) ||
		!validMaterializationDigest(plan.AssetRevisionSetDigest) ||
		plan.ExpectedManifestDigest != "" && !validMaterializationDigest(plan.ExpectedManifestDigest) ||
		len(plan.Bindings) == 0 || len(plan.Bindings) > 32 || !validJourneyID(plan.JourneyID) || !cleanIdentifier(plan.OperationID) {
		return ErrInvalidMaterialization
	}
	capable := false
	for _, capability := range plan.RuntimeCapabilities {
		if capability == SkillMaterializationCapability {
			capable = true
		}
	}
	if !capable {
		return ErrMaterializationCapabilityGap
	}
	setDigest, err := materializationBindingSetDigest(plan.Bindings)
	if err != nil || setDigest != plan.AssetRevisionSetDigest {
		return ErrMaterializationDigestMismatch
	}
	root, err := filepath.Abs(plan.IsolatedRoot)
	if err != nil || root != filepath.Clean(plan.IsolatedRoot) {
		return ErrInvalidMaterialization
	}
	return nil
}

func materializationBindingSetDigest(bindings []MaterializationBinding) (string, error) {
	type exactBinding struct {
		AssetKind    string `json:"asset_kind"`
		DefinitionID string `json:"definition_id"`
		RevisionID   string `json:"revision_id"`
		SHA256Digest string `json:"sha256_digest"`
		SourceScope  string `json:"source_scope"`
	}
	values := make([]exactBinding, len(bindings))
	for index, binding := range bindings {
		values[index] = exactBinding{binding.AssetKind, binding.DefinitionID, binding.RevisionID, binding.Digest, binding.SourceScope}
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].AssetKind != values[j].AssetKind {
			return values[i].AssetKind < values[j].AssetKind
		}
		if values[i].DefinitionID != values[j].DefinitionID {
			return values[i].DefinitionID < values[j].DefinitionID
		}
		return values[i].RevisionID < values[j].RevisionID
	})
	data, err := canonicalJSON(values)
	if err != nil {
		return "", err
	}
	return digestBytes(data), nil
}

func writeMaterializationTree(ctx context.Context, root string, plan MaterializationPlan) (materializationManifest, error) {
	bindings := append([]MaterializationBinding(nil), plan.Bindings...)
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].AssetKind != bindings[j].AssetKind {
			return bindings[i].AssetKind < bindings[j].AssetKind
		}
		if bindings[i].DefinitionID != bindings[j].DefinitionID {
			return bindings[i].DefinitionID < bindings[j].DefinitionID
		}
		return bindings[i].RevisionID < bindings[j].RevisionID
	})
	manifest := materializationManifest{
		SchemaVersion: 1, RunID: plan.RunID, AttemptNumber: plan.AttemptNumber,
		Generation: plan.Generation, RuntimeInstanceID: plan.RuntimeInstanceID,
		RuntimeIdentityDigest: plan.RuntimeIdentityDigest, Capability: SkillMaterializationCapability,
		AssetRevisionBindings:  []materializationManifestBinding{},
		AssetRevisionSetDigest: plan.AssetRevisionSetDigest,
		Entries:                []materializationManifestEntry{},
	}
	seenBinding := map[string]struct{}{}
	total := 0
	for _, binding := range bindings {
		if binding.AssetKind == "" || !cleanIdentifier(binding.DefinitionID) || !cleanIdentifier(binding.RevisionID) || !validMaterializationDigest(binding.Digest) || !validMaterializationDigest(binding.ContentDigest) || binding.SourceScope == "" || len(binding.Files) == 0 || len(binding.Files) > 128 {
			return materializationManifest{}, ErrInvalidMaterialization
		}
		key := binding.AssetKind + "\x00" + binding.DefinitionID + "\x00" + binding.RevisionID
		if _, ok := seenBinding[key]; ok {
			return materializationManifest{}, ErrMaterializationCollision
		}
		seenBinding[key] = struct{}{}
		files := append([]MaterializationFile(nil), binding.Files...)
		sort.Slice(files, func(i, j int) bool { return files[i].RelativePath < files[j].RelativePath })
		manifestBinding := materializationManifestBinding{AssetKind: binding.AssetKind, DefinitionID: binding.DefinitionID, RevisionID: binding.RevisionID, SHA256Digest: binding.Digest, SourceScope: binding.SourceScope}
		seenPath := map[string]struct{}{}
		base := filepath.Join(root, "skills", binding.DefinitionID, binding.RevisionID)
		if err := os.MkdirAll(base, 0o700); err != nil {
			return materializationManifest{}, err
		}
		if err := ensurePrivateDirectories(root, base); err != nil {
			return materializationManifest{}, err
		}
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return materializationManifest{}, err
			}
			if !validRelativeMaterializationPath(file.RelativePath) || !validMaterializationDigest(file.Digest) || digestBytes(file.Bytes) != file.Digest {
				return materializationManifest{}, ErrMaterializationDigestMismatch
			}
			folded := strings.ToLower(file.RelativePath)
			if _, ok := seenPath[folded]; ok {
				return materializationManifest{}, ErrMaterializationCollision
			}
			seenPath[folded] = struct{}{}
			total += len(file.Bytes)
			if total > 16<<20 {
				return materializationManifest{}, ErrInvalidMaterialization
			}
			target := filepath.Join(base, filepath.FromSlash(file.RelativePath))
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return materializationManifest{}, err
			}
			if err := ensurePrivateDirectories(root, filepath.Dir(target)); err != nil {
				return materializationManifest{}, err
			}
			if err := os.WriteFile(target, append([]byte(nil), file.Bytes...), 0o600); err != nil {
				return materializationManifest{}, err
			}
			targetRelativePath := filepath.ToSlash(filepath.Join("skills", binding.DefinitionID, binding.RevisionID, filepath.FromSlash(file.RelativePath)))
			manifest.Entries = append(manifest.Entries, materializationManifestEntry{AssetKind: binding.AssetKind, DefinitionID: binding.DefinitionID, RevisionID: binding.RevisionID, SourceArtifactDigest: binding.Digest, ContentDigest: binding.ContentDigest, TargetRelativePath: targetRelativePath, FileMode: 384, FileSize: len(file.Bytes), FileSHA256: file.Digest})
		}
		manifest.AssetRevisionBindings = append(manifest.AssetRevisionBindings, manifestBinding)
	}
	sort.Slice(manifest.Entries, func(i, j int) bool {
		left, right := manifest.Entries[i], manifest.Entries[j]
		if left.TargetRelativePath != right.TargetRelativePath {
			return left.TargetRelativePath < right.TargetRelativePath
		}
		if left.AssetKind != right.AssetKind {
			return left.AssetKind < right.AssetKind
		}
		if left.DefinitionID != right.DefinitionID {
			return left.DefinitionID < right.DefinitionID
		}
		return left.RevisionID < right.RevisionID
	})
	return manifest, nil
}

func verifyTree(root string, manifest materializationManifest) error {
	if err := validatePrivateDirectory(root); err != nil {
		return ErrMaterializationIdentityDrift
	}
	if err := verifyExactMaterializationTree(root, manifest); err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		if !validRelativeMaterializationPath(entry.TargetRelativePath) {
			return ErrMaterializationIdentityDrift
		}
		data, err := readPrivateMaterializationFile(
			root, entry.TargetRelativePath, entry.FileSize,
		)
		if err != nil {
			return err
		}
		if digestBytes(data) != entry.FileSHA256 {
			return ErrMaterializationDigestMismatch
		}
	}
	return nil
}

func verifyExactMaterializationTree(root string, manifest materializationManifest) error {
	expectedFiles := map[string]struct{}{"manifest.json": {}}
	expectedDirectories := map[string]struct{}{".": {}}
	for _, entry := range manifest.Entries {
		if !validRelativeMaterializationPath(entry.TargetRelativePath) {
			return ErrMaterializationIdentityDrift
		}
		relative := filepath.Clean(entry.TargetRelativePath)
		expectedFiles[relative] = struct{}{}
		for parent := filepath.Dir(relative); parent != "."; parent = filepath.Dir(parent) {
			expectedDirectories[parent] = struct{}{}
		}
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return ErrMaterializationIdentityDrift
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == ".." || filepath.IsAbs(relative) {
			return ErrMaterializationIdentityDrift
		}
		if entry.IsDir() {
			if _, expected := expectedDirectories[relative]; !expected {
				return ErrMaterializationIdentityDrift
			}
			if err := validatePrivateDirectory(path); err != nil {
				return ErrMaterializationIdentityDrift
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return ErrMaterializationIdentityDrift
		}
		if _, expected := expectedFiles[relative]; !expected {
			return ErrMaterializationIdentityDrift
		}
		return nil
	})
}

func readAndValidateManifest(root string, plan MaterializationPlan) (materializationManifest, []byte, error) {
	data, err := readPrivateMaterializationFile(root, "manifest.json", 1<<20)
	if err != nil {
		return materializationManifest{}, nil, ErrMaterializationIdentityDrift
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest materializationManifest
	if err := decoder.Decode(&manifest); err != nil || decoder.Decode(&struct{}{}) == nil {
		return materializationManifest{}, nil, ErrInvalidMaterialization
	}
	canonical, err := canonicalManifest(manifest)
	if err != nil || !bytes.Equal(data, canonical) {
		return materializationManifest{}, nil, ErrInvalidMaterialization
	}
	expectedDigest, err := calculateManifestDigest(manifest)
	if err != nil || expectedDigest != manifest.ManifestDigest {
		return materializationManifest{}, nil, ErrMaterializationDigestMismatch
	}
	if manifest.SchemaVersion != 1 || manifest.RunID != plan.RunID || manifest.AttemptNumber != plan.AttemptNumber || manifest.Generation != plan.Generation || manifest.RuntimeInstanceID != plan.RuntimeInstanceID || manifest.RuntimeIdentityDigest != plan.RuntimeIdentityDigest || manifest.Capability != SkillMaterializationCapability || manifest.AssetRevisionSetDigest != plan.AssetRevisionSetDigest {
		return materializationManifest{}, nil, ErrMaterializationIdentityDrift
	}
	return manifest, data, nil
}

func canonicalManifest(manifest materializationManifest) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(manifest); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

func calculateManifestDigest(manifest materializationManifest) (string, error) {
	unsigned := struct {
		SchemaVersion          int                              `json:"schema_version"`
		RunID                  string                           `json:"run_id"`
		AttemptNumber          int                              `json:"attempt_number"`
		Generation             int64                            `json:"generation"`
		RuntimeInstanceID      string                           `json:"runtime_instance_id"`
		RuntimeIdentityDigest  string                           `json:"runtime_identity_digest"`
		Capability             string                           `json:"capability"`
		AssetRevisionBindings  []materializationManifestBinding `json:"asset_revision_bindings"`
		AssetRevisionSetDigest string                           `json:"asset_revision_set_digest"`
		Entries                []materializationManifestEntry   `json:"entries"`
	}{manifest.SchemaVersion, manifest.RunID, manifest.AttemptNumber, manifest.Generation,
		manifest.RuntimeInstanceID, manifest.RuntimeIdentityDigest, manifest.Capability,
		manifest.AssetRevisionBindings, manifest.AssetRevisionSetDigest, manifest.Entries}
	data, err := canonicalJSON(unsigned)
	if err != nil {
		return "", err
	}
	return digestBytes(data), nil
}

func calculateMaterializationRootDigest(plan MaterializationPlan, manifestDigest string) (string, error) {
	value := struct {
		SchemaVersion         int    `json:"schema_version"`
		RunID                 string `json:"run_id"`
		AttemptNumber         int    `json:"attempt_number"`
		Generation            int64  `json:"generation"`
		RuntimeInstanceID     string `json:"runtime_instance_id"`
		RuntimeIdentityDigest string `json:"runtime_identity_digest"`
		ManifestDigest        string `json:"manifest_digest"`
	}{1, plan.RunID, plan.AttemptNumber, plan.Generation, plan.RuntimeInstanceID, plan.RuntimeIdentityDigest, manifestDigest}
	data, err := canonicalJSON(value)
	if err != nil {
		return "", err
	}
	return digestBytes(data), nil
}

func canonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}
func materializationTarget(plan MaterializationPlan) string {
	return filepath.Join(plan.IsolatedRoot, "materialized", "run-"+plan.RunID, "attempt-"+strconv.Itoa(plan.AttemptNumber), "generation-"+strconv.FormatInt(plan.Generation, 10))
}
func ensurePrivateDirectories(root, target string) error {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ErrInvalidMaterialization
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	rootFD, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return ErrMaterializationIdentityDrift
	}
	defer unix.Close(rootFD)
	if err := validateOwnedDirectoryFD(rootFD); err != nil {
		return err
	}
	if relative == "." {
		return nil
	}
	currentFD := rootFD
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." || part == ".." {
			return ErrInvalidMaterialization
		}
		if err := unix.Mkdirat(currentFD, part, 0o700); err != nil && !errors.Is(err, syscall.EEXIST) {
			return err
		}
		nextFD, err := unix.Openat(
			currentFD, part,
			unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
			0,
		)
		if err != nil {
			return ErrMaterializationIdentityDrift
		}
		if currentFD != rootFD {
			_ = unix.Close(currentFD)
		}
		currentFD = nextFD
		if err := validatePrivateDirectoryFD(currentFD); err != nil {
			_ = unix.Close(currentFD)
			return err
		}
	}
	if currentFD != rootFD {
		return unix.Close(currentFD)
	}
	return nil
}

func validatePrivateDirectory(path string) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("%w: open private directory: %v", ErrMaterializationIdentityDrift, err)
	}
	defer unix.Close(fd)
	return validatePrivateDirectoryFD(fd)
}

func validatePrivateDirectoryFD(fd int) error {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("%w: fstat private directory: %v", ErrMaterializationIdentityDrift, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR ||
		stat.Mode&0o777 != 0o700 || int(stat.Uid) != os.Getuid() {
		return fmt.Errorf(
			"%w: private directory mode=%#o uid=%d",
			ErrMaterializationIdentityDrift, stat.Mode, stat.Uid,
		)
	}
	return nil
}

func validateOwnedDirectoryFD(fd int) error {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("%w: fstat owned directory: %v", ErrMaterializationIdentityDrift, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR ||
		stat.Mode&0o022 != 0 || int(stat.Uid) != os.Getuid() {
		return fmt.Errorf(
			"%w: owned directory mode=%#o uid=%d",
			ErrMaterializationIdentityDrift, stat.Mode, stat.Uid,
		)
	}
	return nil
}

func readPrivateMaterializationFile(root, relativePath string, maxBytes int) ([]byte, error) {
	if !validRelativeMaterializationPath(relativePath) || maxBytes < 0 {
		return nil, ErrInvalidMaterialization
	}
	rootFD, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, ErrMaterializationIdentityDrift
	}
	defer unix.Close(rootFD)
	if err := validatePrivateDirectoryFD(rootFD); err != nil {
		return nil, err
	}
	parts := strings.Split(relativePath, "/")
	currentFD := rootFD
	for _, part := range parts[:len(parts)-1] {
		nextFD, openErr := unix.Openat(
			currentFD, part,
			unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
			0,
		)
		if openErr != nil {
			if currentFD != rootFD {
				_ = unix.Close(currentFD)
			}
			return nil, ErrMaterializationIdentityDrift
		}
		if currentFD != rootFD {
			_ = unix.Close(currentFD)
		}
		currentFD = nextFD
		if err := validatePrivateDirectoryFD(currentFD); err != nil {
			_ = unix.Close(currentFD)
			return nil, err
		}
	}
	if currentFD != rootFD {
		defer unix.Close(currentFD)
	}
	fileFD, err := unix.Openat(
		currentFD, parts[len(parts)-1],
		unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		0,
	)
	if err != nil {
		return nil, ErrMaterializationIdentityDrift
	}
	file := os.NewFile(uintptr(fileFD), relativePath)
	if file == nil {
		_ = unix.Close(fileFD)
		return nil, ErrMaterializationIdentityDrift
	}
	defer file.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(fileFD, &stat); err != nil ||
		stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0o777 != 0o600 ||
		int(stat.Uid) != os.Getuid() || stat.Nlink != 1 ||
		stat.Size < 0 || stat.Size > int64(maxBytes) {
		return nil, ErrMaterializationIdentityDrift
	}
	data := make([]byte, stat.Size)
	if _, err := io.ReadFull(file, data); err != nil {
		return nil, err
	}
	if len(data) != int(stat.Size) {
		return nil, ErrMaterializationIdentityDrift
	}
	return data, nil
}
func validRelativeMaterializationPath(value string) bool {
	if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || filepath.Clean(value) != value {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
func cleanIdentifier(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}
func validMaterializationDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}
func validJourneyID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || value[14] != '4' || !strings.ContainsRune("89ab", rune(value[19])) {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
func digestBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
