package piadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
)

const (
	maxPiMetadataExecutableBytes = 512 * 1024 * 1024
	maxPiMetadataOutputBytes     = 256 * 1024
	maxPiMetadataProcessTimeout  = 30 * time.Second
	piMetadataProcessWaitDelay   = time.Second
	pi0821LockedExecutableSHA256 = "af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca"
)

var (
	ErrInvalidPiMetadataProcessRunner  = errors.New("invalid pi metadata process runner")
	ErrInvalidPiMetadataRequest        = errors.New("invalid pi metadata request")
	ErrPiMetadataBindingChanged        = errors.New("pi metadata binding changed")
	ErrPiMetadataProcessFailed         = errors.New("pi metadata process failed")
	ErrPiMetadataProcessTimeout        = errors.New("pi metadata process timeout")
	ErrPiMetadataProcessOutputTooLarge = errors.New("pi metadata process output too large")
	ErrPiMetadataIsolationCleanup      = errors.New("pi metadata isolation cleanup failed")
)

type PiMetadataProcessRunnerConfig struct {
	ExecutablePath     string
	IsolationRoot      string
	RuntimeSearchPaths []string
	Timeout            time.Duration
	LocalModelCatalog  *PiLocalModelCatalogConfig
}

type piMetadataProcessRunner struct {
	executable     piMetadataExecutableBinding
	isolationRoot  piMetadataDirectoryBinding
	searchPaths    []piMetadataDirectoryBinding
	timeout        time.Duration
	catalog        *piLocalModelCatalog
	conformance    sync.Once
	conformanceErr error
}

type piMetadataExecutableBinding struct {
	path   string
	info   os.FileInfo
	digest [sha256.Size]byte
}

type piMetadataDirectoryBinding struct {
	path string
	info os.FileInfo
}

func NewPiMetadataProcessRunner(config PiMetadataProcessRunnerConfig) (loomruntime.PiMetadataRunner, error) {
	return newPiMetadataProcessRunnerWithArtifactDigests(
		config,
		piLocalLlamaArchiveSHA256,
		piLocalLlamaExecutableSHA256,
		piLocalModelSHA256,
	)
}

func newPiMetadataProcessRunner(
	config PiMetadataProcessRunnerConfig,
	expectedModelDigest string,
) (loomruntime.PiMetadataRunner, error) {
	return newPiMetadataProcessRunnerWithArtifactDigests(
		config,
		"",
		"",
		expectedModelDigest,
	)
}

func newPiMetadataProcessRunnerWithArtifactDigests(
	config PiMetadataProcessRunnerConfig,
	expectedArchiveDigest string,
	expectedExecutableDigest string,
	expectedModelDigest string,
) (loomruntime.PiMetadataRunner, error) {
	runner, err := bindPiMetadataProcessRunnerCore(config)
	if err != nil {
		return nil, err
	}
	if config.LocalModelCatalog != nil {
		runner.catalog, err = bindPiLocalModelCatalogWithArtifactDigests(
			*config.LocalModelCatalog,
			expectedArchiveDigest,
			expectedExecutableDigest,
			expectedModelDigest,
		)
		if err != nil {
			return nil, ErrInvalidPiMetadataProcessRunner
		}
	}
	return runner, nil
}

func newPiMetadataProcessRunnerWithBoundCatalog(
	config PiMetadataProcessRunnerConfig,
	catalog *piLocalModelCatalog,
) (loomruntime.PiMetadataRunner, error) {
	runner, err := bindPiMetadataProcessRunnerCore(config)
	if err != nil {
		return nil, err
	}
	if catalog != nil {
		if err := catalog.validate(); err != nil {
			return nil, ErrInvalidPiMetadataProcessRunner
		}
		runner.catalog = catalog
	}
	return runner, nil
}

func bindPiMetadataProcessRunnerCore(
	config PiMetadataProcessRunnerConfig,
) (*piMetadataProcessRunner, error) {
	if config.Timeout <= 0 ||
		config.Timeout > maxPiMetadataProcessTimeout ||
		len(config.RuntimeSearchPaths) == 0 {
		return nil, ErrInvalidPiMetadataProcessRunner
	}

	executable, err := bindPiMetadataExecutable(config.ExecutablePath)
	if err != nil {
		return nil, ErrInvalidPiMetadataProcessRunner
	}
	root, err := bindPiMetadataIsolationRoot(config.IsolationRoot)
	if err != nil {
		return nil, ErrInvalidPiMetadataProcessRunner
	}
	searchPaths, err := bindPiMetadataSearchPaths(config.RuntimeSearchPaths)
	if err != nil {
		return nil, ErrInvalidPiMetadataProcessRunner
	}

	return &piMetadataProcessRunner{
		executable:    executable,
		isolationRoot: root,
		searchPaths:   append([]piMetadataDirectoryBinding(nil), searchPaths...),
		timeout:       config.Timeout,
	}, nil
}

func (r *piMetadataProcessRunner) RunPiMetadata(
	ctx context.Context,
	request loomruntime.PiMetadataRequest,
) (result loomruntime.PiMetadataResult, resultErr error) {
	if !validPiMetadataProcessRequest(ctx, request) {
		return loomruntime.PiMetadataResult{}, ErrInvalidPiMetadataRequest
	}
	if err := r.validateBindings(); err != nil {
		return loomruntime.PiMetadataResult{}, ErrPiMetadataBindingChanged
	}

	invocationDirectory, err := os.MkdirTemp(r.isolationRoot.path, "loom-pi-metadata-")
	if err != nil {
		return loomruntime.PiMetadataResult{}, ErrPiMetadataProcessFailed
	}
	defer func() {
		if cleanupErr := os.RemoveAll(invocationDirectory); cleanupErr != nil {
			result = loomruntime.PiMetadataResult{}
			resultErr = errors.Join(resultErr, ErrPiMetadataIsolationCleanup)
		}
	}()

	directories, err := createPiMetadataInvocationDirectories(invocationDirectory)
	if err != nil {
		return loomruntime.PiMetadataResult{}, ErrPiMetadataProcessFailed
	}
	if r.catalog != nil {
		if err := r.catalog.materializeValidated(directories.agent); err != nil {
			if errors.Is(err, ErrPiLocalModelBindingChanged) {
				return loomruntime.PiMetadataResult{}, ErrPiMetadataBindingChanged
			}
			return loomruntime.PiMetadataResult{}, ErrPiMetadataProcessFailed
		}
	}

	childContext, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	command := exec.CommandContext(childContext, r.executable.path, request.Args...)
	command.Dir = directories.home
	command.Env = r.environment(directories)
	command.Stdin = nil
	command.WaitDelay = piMetadataProcessWaitDelay
	configurePiMetadataProcess(command)

	stdout := newPiMetadataBoundedWriter(maxPiMetadataOutputBytes)
	stderr := newPiMetadataBoundedWriter(maxPiMetadataOutputBytes)
	command.Stdout = stdout
	command.Stderr = stderr

	runErr := command.Run()
	switch {
	case ctx.Err() != nil:
		return loomruntime.PiMetadataResult{}, ctx.Err()
	case childContext.Err() != nil:
		return loomruntime.PiMetadataResult{}, ErrPiMetadataProcessTimeout
	case stdout.overflow || stderr.overflow:
		return loomruntime.PiMetadataResult{}, ErrPiMetadataProcessOutputTooLarge
	case runErr != nil:
		return loomruntime.PiMetadataResult{}, ErrPiMetadataProcessFailed
	default:
		return loomruntime.PiMetadataResult{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
		}, nil
	}
}

func (r *piMetadataProcessRunner) SkillMaterializationConformance() (
	loomruntime.SkillMaterializationConformance,
	bool,
) {
	if r == nil || r.catalog == nil ||
		hex.EncodeToString(r.executable.digest[:]) != pi0821LockedExecutableSHA256 {
		return nil, false
	}
	return r, true
}

func (r *piMetadataProcessRunner) ContextRetrievalConformance() (
	loomruntime.ContextRetrievalConformance,
	bool,
) {
	if r == nil ||
		hex.EncodeToString(r.executable.digest[:]) != pi0821LockedExecutableSHA256 {
		return nil, false
	}
	return r, true
}

func (r *piMetadataProcessRunner) GovernedToolLoopConformance() (
	loomruntime.GovernedToolLoopConformance,
	bool,
) {
	if r == nil ||
		hex.EncodeToString(r.executable.digest[:]) != pi0821LockedExecutableSHA256 {
		return nil, false
	}
	return r, true
}

func (r *piMetadataProcessRunner) VerifyContextRetrieval(ctx context.Context) error {
	if ctx == nil || r == nil || ctx.Err() != nil ||
		hex.EncodeToString(r.executable.digest[:]) != pi0821LockedExecutableSHA256 ||
		r.validateBindings() != nil {
		return ErrPiMetadataBindingChanged
	}
	return nil
}

func (r *piMetadataProcessRunner) VerifyGovernedToolLoop(ctx context.Context) error {
	if ctx == nil || r == nil || ctx.Err() != nil ||
		hex.EncodeToString(r.executable.digest[:]) != pi0821LockedExecutableSHA256 ||
		r.validateBindings() != nil {
		return ErrPiMetadataBindingChanged
	}
	return nil
}

func (r *piMetadataProcessRunner) VerifySkillMaterialization(ctx context.Context) error {
	if ctx == nil || r == nil {
		return ErrInvalidPiMetadataProcessRunner
	}
	r.conformance.Do(func() {
		r.conformanceErr = r.verifySkillMaterialization(ctx)
	})
	return r.conformanceErr
}

func (r *piMetadataProcessRunner) verifySkillMaterialization(ctx context.Context) error {
	if err := r.validateBindings(); err != nil || r.catalog == nil {
		return ErrPiMetadataBindingChanged
	}
	root, err := os.MkdirTemp(r.isolationRoot.path, "loom-pi-skill-conformance-")
	if err != nil {
		return ErrPiMetadataProcessFailed
	}
	defer os.RemoveAll(root)
	if err := os.Chmod(root, 0o700); err != nil {
		return ErrPiMetadataProcessFailed
	}
	content := []byte("# Loom Pi materialization conformance\n")
	contentSum := sha256.Sum256(content)
	contentDigest := hex.EncodeToString(contentSum[:])
	bindings := []MaterializationBinding{{
		AssetKind: "skill", DefinitionID: "loom-conformance",
		RevisionID: "pi-0.82.1", Digest: contentDigest,
		ContentDigest: contentDigest, SourceScope: "local",
		Files: []MaterializationFile{{
			RelativePath: "SKILL.md", Bytes: content, Digest: contentDigest,
		}},
	}}
	setDigest, err := materializationBindingSetDigest(bindings)
	if err != nil {
		return ErrPiMetadataProcessFailed
	}
	plan := MaterializationPlan{
		IsolatedRoot: root, RunID: "conformance", AttemptNumber: 1,
		Generation: 1, RuntimeInstanceID: "pi-0.82.1",
		RuntimeIdentityDigest:  hex.EncodeToString(r.executable.digest[:]),
		RuntimeCapabilities:    []string{SkillMaterializationCapability},
		AssetRevisionSetDigest: setDigest,
		JourneyID:              "123e4567-e89b-42d3-a456-426614174000",
		OperationID:            "materialize:conformance:1:1",
		Bindings:               bindings,
	}
	materializer, err := NewSkillMaterializer(MaterializationHooks{})
	if err != nil {
		return ErrPiMetadataProcessFailed
	}
	result, err := materializer.Materialize(ctx, plan)
	if err != nil {
		return ErrPiMetadataProcessFailed
	}
	if _, err := materializer.Materialize(ctx, plan); !errors.Is(err, ErrMaterializationCollision) {
		return ErrPiMetadataProcessFailed
	}
	directories, err := createPiMetadataInvocationDirectories(root)
	if err != nil || r.catalog.materializeValidated(directories.agent) != nil {
		return ErrPiMetadataProcessFailed
	}
	childContext, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	arguments := []string{
		"--offline", "--no-approve", "--no-extensions", "--no-skills",
		"--skill", filepath.Join(result.Root, "skills"),
		"--no-prompt-templates", "--no-themes", "--no-context-files",
		"--list-models",
	}
	command := exec.CommandContext(childContext, r.executable.path, arguments...)
	command.Dir = directories.home
	command.Env = r.environment(directories)
	command.Stdin = nil
	command.WaitDelay = piMetadataProcessWaitDelay
	configurePiMetadataProcess(command)
	stdout := newPiMetadataBoundedWriter(maxPiMetadataOutputBytes)
	stderr := newPiMetadataBoundedWriter(maxPiMetadataOutputBytes)
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil || childContext.Err() != nil ||
		stdout.overflow || stderr.overflow {
		return ErrPiMetadataProcessFailed
	}
	if err := materializer.Cleanup(ctx, plan); err != nil {
		return ErrPiMetadataIsolationCleanup
	}
	if _, err := os.Lstat(result.Root); !errors.Is(err, os.ErrNotExist) {
		return ErrPiMetadataIsolationCleanup
	}
	return r.validateBindings()
}

func bindPiMetadataExecutable(path string) (piMetadataExecutableBinding, error) {
	if !validPiMetadataConfiguredPath(path) {
		return piMetadataExecutableBinding{}, ErrInvalidPiMetadataProcessRunner
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(resolved) {
		return piMetadataExecutableBinding{}, ErrInvalidPiMetadataProcessRunner
	}
	info, digest, err := inspectPiMetadataExecutable(resolved)
	if err != nil {
		return piMetadataExecutableBinding{}, ErrInvalidPiMetadataProcessRunner
	}
	return piMetadataExecutableBinding{path: resolved, info: info, digest: digest}, nil
}

func bindPiMetadataIsolationRoot(path string) (piMetadataDirectoryBinding, error) {
	if !validPiMetadataConfiguredPath(path) {
		return piMetadataDirectoryBinding{}, ErrInvalidPiMetadataProcessRunner
	}
	original, err := os.Lstat(path)
	if err != nil || original.Mode()&os.ModeSymlink != 0 {
		return piMetadataDirectoryBinding{}, ErrInvalidPiMetadataProcessRunner
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(resolved) {
		return piMetadataDirectoryBinding{}, ErrInvalidPiMetadataProcessRunner
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return piMetadataDirectoryBinding{}, ErrInvalidPiMetadataProcessRunner
	}
	return piMetadataDirectoryBinding{path: resolved, info: info}, nil
}

func bindPiMetadataSearchPaths(paths []string) ([]piMetadataDirectoryBinding, error) {
	bindings := make([]piMetadataDirectoryBinding, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range append([]string(nil), paths...) {
		if !validPiMetadataConfiguredPath(path) {
			return nil, ErrInvalidPiMetadataProcessRunner
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !filepath.IsAbs(resolved) {
			return nil, ErrInvalidPiMetadataProcessRunner
		}
		if _, exists := seen[resolved]; exists {
			continue
		}
		info, err := os.Lstat(resolved)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInvalidPiMetadataProcessRunner
		}
		seen[resolved] = struct{}{}
		bindings = append(bindings, piMetadataDirectoryBinding{path: resolved, info: info})
	}
	if len(bindings) == 0 {
		return nil, ErrInvalidPiMetadataProcessRunner
	}
	return bindings, nil
}

func validPiMetadataConfiguredPath(path string) bool {
	return path != "" &&
		!strings.ContainsRune(path, '\x00') &&
		filepath.IsAbs(path) &&
		filepath.Clean(path) == path
}

func inspectPiMetadataExecutable(path string) (os.FileInfo, [sha256.Size]byte, error) {
	info, err := os.Lstat(path)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode().Perm()&0o111 == 0 ||
		info.Size() < 0 ||
		info.Size() > maxPiMetadataExecutableBytes {
		return nil, [sha256.Size]byte{}, ErrInvalidPiMetadataProcessRunner
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, [sha256.Size]byte{}, ErrInvalidPiMetadataProcessRunner
	}
	defer file.Close()
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, maxPiMetadataExecutableBytes+1))
	if err != nil || written > maxPiMetadataExecutableBytes {
		return nil, [sha256.Size]byte{}, ErrInvalidPiMetadataProcessRunner
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return info, digest, nil
}

func (r *piMetadataProcessRunner) validateBindings() error {
	info, digest, err := inspectPiMetadataExecutable(r.executable.path)
	if err != nil || !os.SameFile(info, r.executable.info) || digest != r.executable.digest {
		return ErrPiMetadataBindingChanged
	}
	if !validPiMetadataDirectoryBinding(r.isolationRoot, true) {
		return ErrPiMetadataBindingChanged
	}
	for _, binding := range r.searchPaths {
		if !validPiMetadataDirectoryBinding(binding, false) {
			return ErrPiMetadataBindingChanged
		}
	}
	if r.catalog != nil {
		if err := r.catalog.validate(); err != nil {
			return ErrPiMetadataBindingChanged
		}
	}
	return nil
}

func validPiMetadataDirectoryBinding(binding piMetadataDirectoryBinding, private bool) bool {
	info, err := os.Lstat(binding.path)
	if err != nil ||
		!info.IsDir() ||
		info.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(info, binding.info) {
		return false
	}
	return !private || info.Mode().Perm() == 0o700
}

func validPiMetadataProcessRequest(ctx context.Context, request loomruntime.PiMetadataRequest) bool {
	if ctx == nil {
		return false
	}
	switch request.Command {
	case loomruntime.PiMetadataVersion:
		return slices.Equal(request.Args, []string{"--version"})
	case loomruntime.PiMetadataListModels:
		return slices.Equal(request.Args, []string{
			"--offline",
			"--no-approve",
			"--no-extensions",
			"--no-skills",
			"--no-prompt-templates",
			"--no-themes",
			"--no-context-files",
			"--list-models",
		})
	default:
		return false
	}
}

type piMetadataInvocationDirectories struct {
	home     string
	agent    string
	sessions string
	temp     string
}

func createPiMetadataInvocationDirectories(root string) (piMetadataInvocationDirectories, error) {
	if err := os.Chmod(root, 0o700); err != nil {
		return piMetadataInvocationDirectories{}, err
	}
	directories := piMetadataInvocationDirectories{
		home:     filepath.Join(root, "home"),
		agent:    filepath.Join(root, "agent"),
		sessions: filepath.Join(root, "sessions"),
		temp:     filepath.Join(root, "tmp"),
	}
	for _, path := range []string{directories.home, directories.agent, directories.sessions, directories.temp} {
		if err := os.Mkdir(path, 0o700); err != nil {
			return piMetadataInvocationDirectories{}, err
		}
		if err := os.Chmod(path, 0o700); err != nil {
			return piMetadataInvocationDirectories{}, err
		}
	}
	return directories, nil
}

func (r *piMetadataProcessRunner) environment(directories piMetadataInvocationDirectories) []string {
	searchPaths := make([]string, 0, len(r.searchPaths))
	for _, binding := range r.searchPaths {
		searchPaths = append(searchPaths, binding.path)
	}
	return []string{
		"HOME=" + directories.home,
		"TMPDIR=" + directories.temp,
		"PI_CODING_AGENT_DIR=" + directories.agent,
		"PI_CODING_AGENT_SESSION_DIR=" + directories.sessions,
		"PATH=" + strings.Join(searchPaths, string(os.PathListSeparator)),
		"LANG=C",
		"LC_ALL=C",
		"NO_COLOR=1",
		"TERM=dumb",
		"PI_OFFLINE=1",
		"PI_SKIP_VERSION_CHECK=1",
		"PI_TELEMETRY=0",
	}
}

type piMetadataBoundedWriter struct {
	limit    int
	bytes    []byte
	overflow bool
}

func newPiMetadataBoundedWriter(limit int) *piMetadataBoundedWriter {
	return &piMetadataBoundedWriter{
		limit: limit,
		bytes: make([]byte, 0, limit),
	}
}

func (w *piMetadataBoundedWriter) Write(input []byte) (int, error) {
	remaining := w.limit - len(w.bytes)
	if remaining > 0 {
		copied := len(input)
		if copied > remaining {
			copied = remaining
		}
		w.bytes = append(w.bytes, input[:copied]...)
	}
	if len(input) > remaining {
		w.overflow = true
	}
	return len(input), nil
}

func (w *piMetadataBoundedWriter) String() string {
	return string(append([]byte(nil), w.bytes...))
}
