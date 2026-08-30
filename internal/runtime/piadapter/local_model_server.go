package piadapter

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidPiLocalModel        = errors.New("invalid Pi local model")
	ErrPiLocalModelBindingChanged = errors.New("Pi local model binding changed")
	ErrPiLocalModelProcess        = errors.New("Pi local model process failed")
	ErrPiLocalModelHealth         = errors.New("Pi local model health failed")
	ErrPiLocalModelCleanup        = errors.New("Pi local model cleanup failed")
)

const (
	piLocalLlamaArchiveSHA256       = "b9554ab4c9f6e91199f48387cb4ab27466fb1d724881f81463ef03f6370cfa32"
	piLocalLlamaExecutableSHA256    = "a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b"
	piLocalModelSHA256              = "cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046"
	maxPiLocalModelSize             = int64(4 << 30)
	maxPiLocalBinarySize            = int64(1 << 30)
	maxPiLocalRuntimeArchiveSize    = int64(64 << 20)
	maxPiLocalRuntimeTreeSize       = int64(1 << 30)
	maxPiLocalRuntimeEntries        = 512
	maxPiLocalHealthBody            = 4096
	piLocalStableHealthObservations = 3
	piLocalExitClassificationGrace  = 25 * time.Millisecond
	piLocalModelContextTokens       = 32768
	piLocalModelMaxOutputTokens     = 256
)

type PiLocalModelServerConfig struct {
	PrivateRoot        string
	RuntimeArchivePath string
	ExecutablePath     string
	ModelPath          string
	Host               string
	Port               int
	StartupTimeout     time.Duration
	CancelGrace        time.Duration
}

type PiLocalModelServer interface {
	BaseURL() string
	Close(context.Context) error
}

type PiLocalModelServerBinding struct {
	RuntimeArchiveSHA256 string
	RuntimeTreeSHA256    string
	ExecutableSHA256     string
	ModelSHA256          string
}

type piLocalDirectoryBinding struct {
	path string
	info os.FileInfo
	mode os.FileMode
	uid  uint32
}

type piLocalFileBinding struct {
	path   string
	info   os.FileInfo
	mode   os.FileMode
	size   int64
	digest string
}

type piLocalServerBinding struct {
	public             PiLocalModelServerBinding
	root               string
	directories        []piLocalDirectoryBinding
	runtimeDirectories []piLocalDirectoryBinding
	runtimeFiles       []piLocalFileBinding
	archive            piLocalFileBinding
	executable         piLocalFileBinding
	model              piLocalFileBinding
}

type piLocalRuntimeArchiveEntry struct {
	path       string
	digest     string
	size       int64
	executable bool
	target     string
	directory  bool
}

type piLocalModelServer struct {
	baseURL     string
	hostPort    string
	command     *exec.Cmd
	wait        <-chan error
	cancelGrace time.Duration
	closeMu     sync.Mutex
	closed      bool
}

func StartPiLocalModelServer(
	ctx context.Context,
	config PiLocalModelServerConfig,
) (PiLocalModelServer, error) {
	return startPiLocalModelServerWithArtifactDigests(
		ctx,
		config,
		piLocalLlamaArchiveSHA256,
		piLocalLlamaExecutableSHA256,
		piLocalModelSHA256,
		nil,
	)
}

func InspectPiLocalModelServerBinding(
	config PiLocalModelServerConfig,
) (PiLocalModelServerBinding, error) {
	binding, err := inspectPiLocalModelServerBindingWithArtifactDigests(
		config,
		piLocalLlamaArchiveSHA256,
		piLocalLlamaExecutableSHA256,
		piLocalModelSHA256,
	)
	if err != nil {
		return PiLocalModelServerBinding{}, err
	}
	return binding.public, nil
}

func startPiLocalModelServer(
	ctx context.Context,
	config PiLocalModelServerConfig,
	expectedModelDigest string,
) (PiLocalModelServer, error) {
	return startPiLocalModelServerWithDigests(
		ctx,
		config,
		"",
		expectedModelDigest,
		nil,
	)
}

func startPiLocalModelServerWithPortGate(
	ctx context.Context,
	config PiLocalModelServerConfig,
	expectedModelDigest string,
	afterFirstPortCheck func() error,
) (PiLocalModelServer, error) {
	return startPiLocalModelServerWithDigests(
		ctx,
		config,
		"",
		expectedModelDigest,
		afterFirstPortCheck,
	)
}

func startPiLocalModelServerWithDigests(
	ctx context.Context,
	config PiLocalModelServerConfig,
	expectedExecutableDigest string,
	expectedModelDigest string,
	afterFirstPortCheck func() error,
) (PiLocalModelServer, error) {
	return startPiLocalModelServerWithArtifactDigests(
		ctx,
		config,
		"",
		expectedExecutableDigest,
		expectedModelDigest,
		afterFirstPortCheck,
	)
}

func startPiLocalModelServerWithArtifactDigests(
	ctx context.Context,
	config PiLocalModelServerConfig,
	expectedArchiveDigest string,
	expectedExecutableDigest string,
	expectedModelDigest string,
	afterFirstPortCheck func() error,
) (PiLocalModelServer, error) {
	if ctx == nil ||
		runtime.GOOS == "windows" ||
		config.Host != "127.0.0.1" ||
		config.Port < 1024 ||
		config.Port > 65535 ||
		config.StartupTimeout <= 0 ||
		config.StartupTimeout > 60*time.Second ||
		config.CancelGrace < time.Second/20 ||
		config.CancelGrace > 5*time.Second ||
		len(expectedModelDigest) != 64 {
		return nil, ErrInvalidPiLocalModel
	}
	if _, err := hex.DecodeString(expectedModelDigest); err != nil {
		return nil, ErrInvalidPiLocalModel
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(ErrPiLocalModelProcess, err)
	}
	binding, err := inspectPiLocalModelServerBindingWithArtifactDigests(
		config,
		expectedArchiveDigest,
		expectedExecutableDigest,
		expectedModelDigest,
	)
	if err != nil {
		return nil, err
	}
	hostPort := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	if err := provePiLocalPortFree(hostPort); err != nil {
		return nil, err
	}
	if afterFirstPortCheck != nil {
		if err := afterFirstPortCheck(); err != nil {
			return nil, errors.Join(ErrInvalidPiLocalModel, err)
		}
	}
	homePath := filepath.Join(binding.root, ".llama-home")
	tempPath := filepath.Join(binding.root, ".llama-tmp")
	for _, path := range []string{homePath, tempPath} {
		if err := ensurePiLocalPrivateDirectory(path); err != nil {
			return nil, err
		}
	}
	args := piLocalModelArguments(binding.model.path, config.Host, config.Port)
	command := exec.Command(binding.executable.path, args...)
	command.Dir = binding.root
	command.Env = []string{
		"HOME=" + homePath,
		"TMPDIR=" + tempPath,
		"PATH=" + filepath.Dir(binding.executable.path),
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"NO_PROXY=127.0.0.1",
		"no_proxy=127.0.0.1",
	}
	if err := configureLocalModelProcess(command); err != nil {
		return nil, errors.Join(ErrPiLocalModelCleanup, err)
	}
	if err := revalidatePiLocalServerBinding(
		binding,
		config,
		expectedModelDigest,
	); err != nil {
		return nil, err
	}
	if err := provePiLocalPortFree(hostPort); err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, ErrPiLocalModelProcess
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, ErrPiLocalModelProcess
	}
	if err := command.Start(); err != nil {
		return nil, ErrPiLocalModelProcess
	}
	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	drainErr := make(chan error, 2)
	go drainPiLocalOutput(stdout, drainErr)
	go drainPiLocalOutput(stderr, drainErr)

	startupContext, cancel := context.WithTimeout(ctx, config.StartupTimeout)
	defer cancel()
	if err := waitForPiLocalHealth(startupContext, hostPort, wait); err != nil {
		cleanupErr := terminateLocalModelProcess(command, wait, config.CancelGrace)
		if cleanupErr != nil {
			cleanupErr = errors.Join(ErrPiLocalModelCleanup, cleanupErr)
		}
		return nil, errors.Join(err, cleanupErr)
	}
	if piLocalProcessExited(wait) {
		cleanupErr := terminateLocalModelProcess(command, wait, config.CancelGrace)
		return nil, errors.Join(ErrPiLocalModelProcess, cleanupErr)
	}
	select {
	case outputErr := <-drainErr:
		if outputErr != nil {
			cleanupErr := terminateLocalModelProcess(command, wait, config.CancelGrace)
			return nil, errors.Join(ErrPiLocalModelProcess, outputErr, cleanupErr)
		}
	default:
	}
	return &piLocalModelServer{
		baseURL:     "http://" + hostPort + "/v1",
		hostPort:    hostPort,
		command:     command,
		wait:        wait,
		cancelGrace: config.CancelGrace,
	}, nil
}

func (server *piLocalModelServer) BaseURL() string {
	if server == nil {
		return ""
	}
	return server.baseURL
}

func (server *piLocalModelServer) Close(ctx context.Context) error {
	if server == nil || ctx == nil {
		return ErrPiLocalModelCleanup
	}
	server.closeMu.Lock()
	defer server.closeMu.Unlock()
	if server.closed {
		return nil
	}
	grace := server.cancelGrace
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errors.Join(ErrPiLocalModelCleanup, ctx.Err())
		}
		if remaining < grace {
			grace = remaining
		}
	}
	cleanupErr := terminateLocalModelProcess(server.command, server.wait, grace)
	if cleanupErr == nil {
		cleanupErr = provePiLocalListenerGone(ctx, server.hostPort, grace)
	}
	if cleanupErr != nil {
		return errors.Join(ErrPiLocalModelCleanup, cleanupErr)
	}
	server.closed = true
	return nil
}

func piLocalModelArguments(modelPath string, host string, port int) []string {
	return []string{
		"--model", modelPath,
		"--alias", piRPCModelID,
		"--host", host,
		"--port", strconv.Itoa(port),
		"--ctx-size", strconv.Itoa(piLocalModelContextTokens),
		"--parallel", "1",
		"--threads", "4",
		"--n-predict", strconv.Itoa(piLocalModelMaxOutputTokens),
		"--gpu-layers", "all",
		"--offline",
		"--no-webui",
		"--no-agent",
		"--no-ui-mcp-proxy",
		"--no-context-shift",
		"--no-cache-prompt",
		"--sse-ping-interval", "-1",
		"--timeout", "30",
	}
}

func inspectPiLocalModelServerBinding(
	config PiLocalModelServerConfig,
	expectedModelDigest string,
) (piLocalServerBinding, error) {
	return inspectPiLocalModelServerBindingWithDigests(
		config,
		"",
		expectedModelDigest,
	)
}

func inspectPiLocalModelServerBindingWithDigests(
	config PiLocalModelServerConfig,
	expectedExecutableDigest string,
	expectedModelDigest string,
) (piLocalServerBinding, error) {
	return inspectPiLocalModelServerBindingWithArtifactDigests(
		config,
		"",
		expectedExecutableDigest,
		expectedModelDigest,
	)
}

func inspectPiLocalModelServerBindingWithArtifactDigests(
	config PiLocalModelServerConfig,
	expectedArchiveDigest string,
	expectedExecutableDigest string,
	expectedModelDigest string,
) (piLocalServerBinding, error) {
	if runtime.GOOS == "windows" ||
		config.PrivateRoot == "" ||
		!filepath.IsAbs(config.PrivateRoot) ||
		filepath.Clean(config.PrivateRoot) != config.PrivateRoot ||
		config.ExecutablePath == "" ||
		!filepath.IsAbs(config.ExecutablePath) ||
		filepath.Clean(config.ExecutablePath) != config.ExecutablePath ||
		config.ModelPath == "" ||
		!filepath.IsAbs(config.ModelPath) ||
		filepath.Clean(config.ModelPath) != config.ModelPath ||
		config.ExecutablePath == config.ModelPath ||
		(expectedArchiveDigest != "" &&
			(config.RuntimeArchivePath == "" ||
				!filepath.IsAbs(config.RuntimeArchivePath) ||
				filepath.Clean(config.RuntimeArchivePath) != config.RuntimeArchivePath ||
				config.RuntimeArchivePath == config.ExecutablePath ||
				config.RuntimeArchivePath == config.ModelPath)) ||
		(expectedArchiveDigest == "" && config.RuntimeArchivePath != "") ||
		config.Host != "127.0.0.1" ||
		config.Port < 1024 ||
		config.Port > 65535 ||
		config.StartupTimeout <= 0 ||
		config.StartupTimeout > 60*time.Second ||
		config.CancelGrace < time.Second/20 ||
		config.CancelGrace > 5*time.Second ||
		len(expectedModelDigest) != sha256.Size*2 {
		return piLocalServerBinding{}, ErrInvalidPiLocalModel
	}
	if expectedArchiveDigest != "" &&
		(len(expectedArchiveDigest) != sha256.Size*2 ||
			strings.ToLower(expectedArchiveDigest) != expectedArchiveDigest) {
		return piLocalServerBinding{}, ErrInvalidPiLocalModel
	}
	if expectedArchiveDigest != "" {
		if _, err := hex.DecodeString(expectedArchiveDigest); err != nil {
			return piLocalServerBinding{}, ErrInvalidPiLocalModel
		}
	}
	if expectedExecutableDigest != "" &&
		(len(expectedExecutableDigest) != sha256.Size*2 ||
			strings.ToLower(expectedExecutableDigest) != expectedExecutableDigest) {
		return piLocalServerBinding{}, ErrInvalidPiLocalModel
	}
	if expectedExecutableDigest != "" {
		if _, err := hex.DecodeString(expectedExecutableDigest); err != nil {
			return piLocalServerBinding{}, ErrInvalidPiLocalModel
		}
	}
	if _, err := hex.DecodeString(expectedModelDigest); err != nil ||
		strings.ToLower(expectedModelDigest) != expectedModelDigest {
		return piLocalServerBinding{}, ErrInvalidPiLocalModel
	}

	leaves := []string{config.ExecutablePath, config.ModelPath}
	if config.RuntimeArchivePath != "" {
		leaves = append(leaves, config.RuntimeArchivePath)
	}
	directories, err := bindPiLocalDirectoryChains(config.PrivateRoot, leaves...)
	if err != nil {
		return piLocalServerBinding{}, err
	}
	executable, err := bindPiLocalFile(
		config.ExecutablePath,
		maxPiLocalBinarySize,
		true,
		expectedExecutableDigest,
	)
	if err != nil {
		return piLocalServerBinding{}, err
	}
	model, err := bindPiLocalFile(
		config.ModelPath,
		maxPiLocalModelSize,
		false,
		expectedModelDigest,
	)
	if err != nil {
		return piLocalServerBinding{}, err
	}
	var archive piLocalFileBinding
	var runtimeDirectories []piLocalDirectoryBinding
	var runtimeFiles []piLocalFileBinding
	var runtimeTreeDigest string
	if expectedArchiveDigest != "" {
		archive, err = bindPiLocalFile(
			config.RuntimeArchivePath,
			maxPiLocalRuntimeArchiveSize,
			false,
			expectedArchiveDigest,
		)
		if err != nil {
			return piLocalServerBinding{}, err
		}
		runtimeDirectories, runtimeFiles, runtimeTreeDigest, err =
			bindPiLocalRuntimeArchiveTree(archive, config, expectedExecutableDigest)
		if err != nil {
			return piLocalServerBinding{}, err
		}
		matchedExecutable := false
		for _, file := range runtimeFiles {
			if samePiLocalFileBinding(file, executable) {
				matchedExecutable = true
				break
			}
		}
		if !matchedExecutable {
			return piLocalServerBinding{}, ErrInvalidPiLocalModel
		}
	}
	return piLocalServerBinding{
		public: PiLocalModelServerBinding{
			RuntimeArchiveSHA256: archive.digest,
			RuntimeTreeSHA256:    runtimeTreeDigest,
			ExecutableSHA256:     executable.digest,
			ModelSHA256:          model.digest,
		},
		root:               config.PrivateRoot,
		directories:        directories,
		runtimeDirectories: runtimeDirectories,
		runtimeFiles:       runtimeFiles,
		archive:            archive,
		executable:         executable,
		model:              model,
	}, nil
}

func revalidatePiLocalServerBinding(
	expected piLocalServerBinding,
	config PiLocalModelServerConfig,
	expectedModelDigest string,
) error {
	current, err := inspectPiLocalModelServerBindingWithArtifactDigests(
		config,
		expected.archive.digest,
		expected.executable.digest,
		expectedModelDigest,
	)
	if err != nil || !samePiLocalServerBinding(expected, current) {
		return errors.Join(ErrPiLocalModelBindingChanged, err)
	}
	return nil
}

func samePiLocalServerBinding(
	expected piLocalServerBinding,
	current piLocalServerBinding,
) bool {
	if expected.root != current.root ||
		expected.public != current.public ||
		len(expected.directories) != len(current.directories) ||
		len(expected.runtimeDirectories) != len(current.runtimeDirectories) ||
		len(expected.runtimeFiles) != len(current.runtimeFiles) ||
		!sameOptionalPiLocalFileBinding(expected.archive, current.archive) ||
		!samePiLocalFileBinding(expected.executable, current.executable) ||
		!samePiLocalFileBinding(expected.model, current.model) {
		return false
	}
	for index := range expected.directories {
		first := expected.directories[index]
		second := current.directories[index]
		if first.path != second.path ||
			first.mode != second.mode ||
			first.uid != second.uid ||
			!os.SameFile(first.info, second.info) {
			return false
		}
	}
	for index := range expected.runtimeDirectories {
		first := expected.runtimeDirectories[index]
		second := current.runtimeDirectories[index]
		if first.path != second.path ||
			first.mode != second.mode ||
			first.uid != second.uid ||
			!os.SameFile(first.info, second.info) {
			return false
		}
	}
	for index := range expected.runtimeFiles {
		if !samePiLocalFileBinding(
			expected.runtimeFiles[index],
			current.runtimeFiles[index],
		) {
			return false
		}
	}
	return true
}

func sameOptionalPiLocalFileBinding(
	expected piLocalFileBinding,
	current piLocalFileBinding,
) bool {
	if expected.path == "" || current.path == "" {
		return expected.path == current.path
	}
	return samePiLocalFileBinding(expected, current)
}

func samePiLocalFileBinding(
	expected piLocalFileBinding,
	current piLocalFileBinding,
) bool {
	return expected.path == current.path &&
		expected.mode == current.mode &&
		expected.size == current.size &&
		expected.digest == current.digest &&
		os.SameFile(expected.info, current.info)
}

func bindPiLocalRuntimeArchiveTree(
	archive piLocalFileBinding,
	config PiLocalModelServerConfig,
	expectedExecutableDigest string,
) ([]piLocalDirectoryBinding, []piLocalFileBinding, string, error) {
	content, err := readPiLocalBoundFile(archive, maxPiLocalRuntimeArchiveSize)
	if err != nil {
		return nil, nil, "", err
	}
	entries, err := parsePiLocalRuntimeArchive(content)
	if err != nil {
		return nil, nil, "", err
	}
	runtimeRoot := filepath.Dir(config.ExecutablePath)
	if runtimeRoot == config.PrivateRoot ||
		!piLocalPathWithin(config.PrivateRoot, runtimeRoot) ||
		piLocalPathWithin(runtimeRoot, config.RuntimeArchivePath) {
		return nil, nil, "", ErrInvalidPiLocalModel
	}
	executableRelative, err := filepath.Rel(runtimeRoot, config.ExecutablePath)
	if err != nil || executableRelative == "." || filepath.IsAbs(executableRelative) ||
		strings.HasPrefix(executableRelative, ".."+string(filepath.Separator)) {
		return nil, nil, "", ErrInvalidPiLocalModel
	}
	executableEntry, ok := entries[filepath.ToSlash(executableRelative)]
	if !ok || executableEntry.directory ||
		executableEntry.digest != expectedExecutableDigest ||
		!executableEntry.executable {
		return nil, nil, "", ErrInvalidPiLocalModel
	}

	expectedDirectories := make(map[string]struct{})
	expectedFiles := make(map[string]piLocalRuntimeArchiveEntry)
	for relative, entry := range entries {
		if entry.directory {
			if relative != "." {
				expectedDirectories[relative] = struct{}{}
			}
			continue
		}
		expectedFiles[relative] = entry
		for parent := pathpkg.Dir(relative); parent != "."; parent = pathpkg.Dir(parent) {
			expectedDirectories[parent] = struct{}{}
		}
	}

	var directoryBindings []piLocalDirectoryBinding
	var fileBindings []piLocalFileBinding
	seenDirectories := make(map[string]struct{})
	seenFiles := make(map[string]struct{})
	walkErr := filepath.WalkDir(runtimeRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return ErrInvalidPiLocalModel
		}
		relative, err := filepath.Rel(runtimeRoot, path)
		if err != nil || filepath.IsAbs(relative) ||
			strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return ErrInvalidPiLocalModel
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return ErrInvalidPiLocalModel
		}
		if relative == "." {
			binding, err := bindPiLocalDirectory(path, true)
			if err != nil {
				return err
			}
			directoryBindings = append(directoryBindings, binding)
			return nil
		}
		canonical := filepath.ToSlash(relative)
		if entry.IsDir() {
			if _, ok := expectedDirectories[canonical]; !ok {
				return ErrInvalidPiLocalModel
			}
			binding, err := bindPiLocalDirectory(path, true)
			if err != nil {
				return err
			}
			seenDirectories[canonical] = struct{}{}
			directoryBindings = append(directoryBindings, binding)
			return nil
		}
		expected, ok := expectedFiles[canonical]
		if !ok || !entry.Type().IsRegular() {
			return ErrInvalidPiLocalModel
		}
		binding, err := bindPiLocalFile(
			path,
			maxPiLocalRuntimeTreeSize,
			expected.executable,
			expected.digest,
		)
		if err != nil || binding.size != expected.size {
			return ErrInvalidPiLocalModel
		}
		seenFiles[canonical] = struct{}{}
		fileBindings = append(fileBindings, binding)
		return nil
	})
	if walkErr != nil ||
		len(seenDirectories) != len(expectedDirectories) ||
		len(seenFiles) != len(expectedFiles) {
		return nil, nil, "", ErrInvalidPiLocalModel
	}
	sort.Slice(directoryBindings, func(first, second int) bool {
		return directoryBindings[first].path < directoryBindings[second].path
	})
	sort.Slice(fileBindings, func(first, second int) bool {
		return fileBindings[first].path < fileBindings[second].path
	})
	treeDigest, err := piLocalRuntimeTreeDigest(entries)
	if err != nil {
		return nil, nil, "", err
	}
	return directoryBindings, fileBindings, treeDigest, nil
}

func readPiLocalBoundFile(
	binding piLocalFileBinding,
	maxSize int64,
) ([]byte, error) {
	if binding.path == "" || binding.size <= 0 || binding.size > maxSize {
		return nil, ErrInvalidPiLocalModel
	}
	file, err := os.OpenFile(binding.path, os.O_RDONLY|piNoFollowFlag(), 0)
	if err != nil {
		return nil, ErrInvalidPiLocalModel
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(opened, binding.info) ||
		opened.Mode() != binding.mode || opened.Size() != binding.size {
		_ = file.Close()
		return nil, ErrInvalidPiLocalModel
	}
	content, readErr := io.ReadAll(io.LimitReader(file, maxSize+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || int64(len(content)) != binding.size {
		return nil, ErrInvalidPiLocalModel
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != binding.digest {
		return nil, ErrInvalidPiLocalModel
	}
	current, err := bindPiLocalFile(
		binding.path,
		maxSize,
		false,
		binding.digest,
	)
	if err != nil || !samePiLocalFileBinding(binding, current) {
		return nil, ErrInvalidPiLocalModel
	}
	return content, nil
}

func parsePiLocalRuntimeArchive(
	content []byte,
) (map[string]piLocalRuntimeArchiveEntry, error) {
	if len(content) == 0 || int64(len(content)) > maxPiLocalRuntimeArchiveSize {
		return nil, ErrInvalidPiLocalModel
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return nil, ErrInvalidPiLocalModel
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	raw := make(map[string]piLocalRuntimeArchiveEntry)
	var totalSize int64
	for len(raw) <= maxPiLocalRuntimeEntries {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || header == nil {
			return nil, ErrInvalidPiLocalModel
		}
		name, ok := canonicalPiLocalArchivePath(header.Name)
		if !ok {
			return nil, ErrInvalidPiLocalModel
		}
		if _, exists := raw[name]; exists {
			return nil, ErrInvalidPiLocalModel
		}
		entry := piLocalRuntimeArchiveEntry{path: name}
		switch header.Typeflag {
		case tar.TypeDir:
			entry.directory = true
		case tar.TypeReg, tar.TypeRegA:
			if header.Size <= 0 || header.Size > maxPiLocalRuntimeTreeSize ||
				totalSize > maxPiLocalRuntimeTreeSize-header.Size {
				return nil, ErrInvalidPiLocalModel
			}
			hash := sha256.New()
			written, copyErr := io.Copy(hash, io.LimitReader(tarReader, header.Size+1))
			if copyErr != nil || written != header.Size {
				return nil, ErrInvalidPiLocalModel
			}
			entry.digest = hex.EncodeToString(hash.Sum(nil))
			entry.size = header.Size
			entry.executable = header.Mode&0o111 != 0
			totalSize += header.Size
		case tar.TypeSymlink:
			if header.Linkname == "" || pathpkg.IsAbs(header.Linkname) ||
				strings.Contains(header.Linkname, "\\") ||
				strings.ContainsRune(header.Linkname, 0) {
				return nil, ErrInvalidPiLocalModel
			}
			target, ok := canonicalPiLocalArchivePath(
				pathpkg.Join(pathpkg.Dir(name), header.Linkname),
			)
			if !ok {
				return nil, ErrInvalidPiLocalModel
			}
			entry.target = target
		case tar.TypeLink:
			target, ok := canonicalPiLocalArchivePath(header.Linkname)
			if !ok {
				return nil, ErrInvalidPiLocalModel
			}
			entry.target = target
		default:
			return nil, ErrInvalidPiLocalModel
		}
		raw[name] = entry
	}
	if len(raw) == 0 || len(raw) > maxPiLocalRuntimeEntries {
		return nil, ErrInvalidPiLocalModel
	}
	prefix := piLocalArchiveCommonPrefix(raw)
	normalized := make(map[string]piLocalRuntimeArchiveEntry, len(raw))
	for name, entry := range raw {
		name = trimPiLocalArchivePrefix(name, prefix)
		if name == "." {
			continue
		}
		if entry.target != "" {
			entry.target = trimPiLocalArchivePrefix(entry.target, prefix)
			if entry.target == "." {
				return nil, ErrInvalidPiLocalModel
			}
		}
		entry.path = name
		if _, exists := normalized[name]; exists {
			return nil, ErrInvalidPiLocalModel
		}
		normalized[name] = entry
	}
	resolved := make(map[string]piLocalRuntimeArchiveEntry, len(normalized))
	for name := range normalized {
		entry, err := resolvePiLocalArchiveEntry(name, normalized, nil)
		if err != nil {
			return nil, err
		}
		resolved[name] = entry
	}
	return resolved, nil
}

func canonicalPiLocalArchivePath(value string) (string, bool) {
	if value == "" || strings.Contains(value, "\\") || strings.ContainsRune(value, 0) ||
		pathpkg.IsAbs(value) {
		return "", false
	}
	clean := pathpkg.Clean(value)
	if clean == "." {
		return clean, true
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func piLocalArchiveCommonPrefix(
	entries map[string]piLocalRuntimeArchiveEntry,
) string {
	prefix := ""
	for name, entry := range entries {
		if entry.directory {
			continue
		}
		first, _, found := strings.Cut(name, "/")
		if !found {
			return ""
		}
		if prefix == "" {
			prefix = first
		} else if first != prefix {
			return ""
		}
	}
	return prefix
}

func trimPiLocalArchivePrefix(value string, prefix string) string {
	if prefix == "" {
		return value
	}
	if value == prefix {
		return "."
	}
	return strings.TrimPrefix(value, prefix+"/")
}

func resolvePiLocalArchiveEntry(
	name string,
	entries map[string]piLocalRuntimeArchiveEntry,
	visiting map[string]struct{},
) (piLocalRuntimeArchiveEntry, error) {
	entry, ok := entries[name]
	if !ok {
		return piLocalRuntimeArchiveEntry{}, ErrInvalidPiLocalModel
	}
	if entry.directory || entry.target == "" {
		return entry, nil
	}
	if visiting == nil {
		visiting = make(map[string]struct{})
	}
	if _, exists := visiting[name]; exists || len(visiting) >= maxPiLocalRuntimeEntries {
		return piLocalRuntimeArchiveEntry{}, ErrInvalidPiLocalModel
	}
	visiting[name] = struct{}{}
	target, err := resolvePiLocalArchiveEntry(entry.target, entries, visiting)
	delete(visiting, name)
	if err != nil || target.directory || target.digest == "" {
		return piLocalRuntimeArchiveEntry{}, ErrInvalidPiLocalModel
	}
	entry.digest = target.digest
	entry.size = target.size
	entry.executable = target.executable
	entry.target = ""
	return entry, nil
}

func piLocalRuntimeTreeDigest(
	entries map[string]piLocalRuntimeArchiveEntry,
) (string, error) {
	type canonicalEntry struct {
		Path       string `json:"path"`
		Kind       string `json:"kind"`
		SHA256     string `json:"sha256,omitempty"`
		Bytes      int64  `json:"bytes,omitempty"`
		Executable bool   `json:"executable,omitempty"`
	}
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	canonical := make([]canonicalEntry, 0, len(paths))
	for _, path := range paths {
		entry := entries[path]
		kind := "file"
		if entry.directory {
			kind = "directory"
		}
		canonical = append(canonical, canonicalEntry{
			Path: path, Kind: kind, SHA256: entry.digest,
			Bytes: entry.size, Executable: entry.executable,
		})
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", ErrInvalidPiLocalModel
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func piLocalPathWithin(parent string, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != "." && !filepath.IsAbs(relative) &&
		relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func bindPiLocalDirectoryChains(
	privateRoot string,
	leaves ...string,
) ([]piLocalDirectoryBinding, error) {
	resolvedRoot, err := filepath.EvalSymlinks(privateRoot)
	if err != nil ||
		!filepath.IsAbs(resolvedRoot) ||
		filepath.Clean(resolvedRoot) != resolvedRoot ||
		resolvedRoot == string(filepath.Separator) {
		return nil, ErrInvalidPiLocalModel
	}
	rootInfo, err := os.Lstat(privateRoot)
	if err != nil ||
		!rootInfo.IsDir() ||
		rootInfo.Mode()&os.ModeSymlink != 0 ||
		rootInfo.Mode().Perm() != 0o700 ||
		!piLocalCurrentUserOwns(rootInfo) {
		return nil, ErrInvalidPiLocalModel
	}

	var bindings []piLocalDirectoryBinding
	seen := make(map[string]struct{})
	for _, ancestor := range piLocalAbsoluteDirectoryPrefixes(
		filepath.Dir(resolvedRoot),
	) {
		binding, err := bindPiLocalDirectory(ancestor, false)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[binding.path]; !exists {
			seen[binding.path] = struct{}{}
			bindings = append(bindings, binding)
		}
	}
	rootBinding, err := bindPiLocalDirectory(privateRoot, true)
	if err != nil {
		return nil, err
	}
	seen[rootBinding.path] = struct{}{}
	bindings = append(bindings, rootBinding)

	for _, leaf := range leaves {
		relative, err := filepath.Rel(privateRoot, leaf)
		if err != nil ||
			relative == "." ||
			relative == ".." ||
			strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
			filepath.IsAbs(relative) {
			return nil, ErrInvalidPiLocalModel
		}
		resolvedLeaf, err := filepath.EvalSymlinks(leaf)
		if err != nil {
			return nil, ErrInvalidPiLocalModel
		}
		resolvedRelative, err := filepath.Rel(resolvedRoot, resolvedLeaf)
		if err != nil ||
			resolvedRelative == "." ||
			resolvedRelative == ".." ||
			strings.HasPrefix(
				resolvedRelative,
				".."+string(filepath.Separator),
			) ||
			filepath.IsAbs(resolvedRelative) {
			return nil, ErrInvalidPiLocalModel
		}
		descendant := privateRoot
		parentRelative := filepath.Dir(relative)
		if parentRelative == "." {
			continue
		}
		for _, component := range strings.Split(
			parentRelative,
			string(filepath.Separator),
		) {
			if component == "" || component == "." || component == ".." {
				return nil, ErrInvalidPiLocalModel
			}
			descendant = filepath.Join(descendant, component)
			binding, err := bindPiLocalDirectory(descendant, true)
			if err != nil {
				return nil, err
			}
			if _, exists := seen[binding.path]; !exists {
				seen[binding.path] = struct{}{}
				bindings = append(bindings, binding)
			}
		}
	}
	return bindings, nil
}

func piLocalAbsoluteDirectoryPrefixes(path string) []string {
	if path == string(filepath.Separator) {
		return []string{path}
	}
	trimmed := strings.TrimPrefix(path, string(filepath.Separator))
	parts := strings.Split(trimmed, string(filepath.Separator))
	prefixes := make([]string, 0, len(parts)+1)
	current := string(filepath.Separator)
	prefixes = append(prefixes, current)
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		prefixes = append(prefixes, current)
	}
	return prefixes
}

func bindPiLocalDirectory(
	path string,
	private bool,
) (piLocalDirectoryBinding, error) {
	info, err := os.Lstat(path)
	if err != nil ||
		!info.IsDir() ||
		info.Mode()&os.ModeSymlink != 0 ||
		(private &&
			(info.Mode().Perm() != 0o700 ||
				!piLocalCurrentUserOwns(info))) ||
		(!private && info.Mode().Perm()&0o022 != 0) {
		return piLocalDirectoryBinding{}, ErrInvalidPiLocalModel
	}
	handle, err := os.OpenFile(path, os.O_RDONLY|piNoFollowFlag(), 0)
	if err != nil {
		return piLocalDirectoryBinding{}, ErrInvalidPiLocalModel
	}
	opened, err := handle.Stat()
	closeErr := handle.Close()
	if err != nil ||
		closeErr != nil ||
		!os.SameFile(info, opened) ||
		info.Mode() != opened.Mode() {
		return piLocalDirectoryBinding{}, ErrInvalidPiLocalModel
	}
	after, err := os.Lstat(path)
	if err != nil ||
		!os.SameFile(opened, after) ||
		opened.Mode() != after.Mode() {
		return piLocalDirectoryBinding{}, ErrInvalidPiLocalModel
	}
	uid, ok := piLocalFileUID(after)
	if !ok {
		return piLocalDirectoryBinding{}, ErrInvalidPiLocalModel
	}
	return piLocalDirectoryBinding{
		path: path,
		info: after,
		mode: after.Mode(),
		uid:  uid,
	}, nil
}

func bindPiLocalFile(
	path string,
	maxSize int64,
	executable bool,
	expectedDigest string,
) (piLocalFileBinding, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return piLocalFileBinding{}, ErrInvalidPiLocalModel
	}
	info, err := os.Lstat(path)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Size() <= 0 ||
		info.Size() > maxSize ||
		!piLocalCurrentUserOwns(info) ||
		!piLocalFileHasSingleLink(info) ||
		(executable && info.Mode().Perm() != 0o700) ||
		(!executable && info.Mode().Perm() != 0o600) {
		return piLocalFileBinding{}, ErrInvalidPiLocalModel
	}
	file, err := os.OpenFile(path, os.O_RDONLY|piNoFollowFlag(), 0)
	if err != nil {
		return piLocalFileBinding{}, ErrInvalidPiLocalModel
	}
	opened, err := file.Stat()
	if err != nil ||
		!os.SameFile(info, opened) ||
		opened.Mode() != info.Mode() ||
		opened.Size() != info.Size() ||
		!piLocalCurrentUserOwns(opened) ||
		!piLocalFileHasSingleLink(opened) {
		_ = file.Close()
		return piLocalFileBinding{}, ErrInvalidPiLocalModel
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, io.LimitReader(file, maxSize+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		return piLocalFileBinding{}, ErrInvalidPiLocalModel
	}
	after, err := os.Lstat(path)
	if err != nil ||
		!os.SameFile(opened, after) ||
		after.Mode() != opened.Mode() ||
		after.Size() != opened.Size() ||
		!piLocalCurrentUserOwns(after) ||
		!piLocalFileHasSingleLink(after) {
		return piLocalFileBinding{}, ErrInvalidPiLocalModel
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if expectedDigest != "" && digest != expectedDigest {
		return piLocalFileBinding{}, ErrInvalidPiLocalModel
	}
	return piLocalFileBinding{
		path:   path,
		info:   after,
		mode:   after.Mode(),
		size:   after.Size(),
		digest: digest,
	}, nil
}

func revalidatePiLocalBinding(
	expected piLocalFileBinding,
	maxSize int64,
	executable bool,
	expectedDigest string,
) error {
	current, err := bindPiLocalFile(expected.path, maxSize, executable, expectedDigest)
	if err != nil ||
		!os.SameFile(current.info, expected.info) ||
		current.mode != expected.mode ||
		current.size != expected.size ||
		current.digest != expected.digest {
		return ErrPiLocalModelBindingChanged
	}
	return nil
}

func provePiLocalPortFree(hostPort string) error {
	listener, err := net.Listen("tcp", hostPort)
	if err != nil {
		return ErrInvalidPiLocalModel
	}
	if err := listener.Close(); err != nil {
		return ErrInvalidPiLocalModel
	}
	return nil
}

func ensurePiLocalPrivateDirectory(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return ErrInvalidPiLocalModel
	}
	info, err := os.Lstat(path)
	if err != nil ||
		!info.IsDir() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o700 ||
		!piLocalCurrentUserOwns(info) {
		return ErrInvalidPiLocalModel
	}
	return nil
}

func waitForPiLocalHealth(
	ctx context.Context,
	hostPort string,
	wait chan error,
) error {
	transport := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: 250 * time.Millisecond,
		}).DialContext,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   500 * time.Millisecond,
	}
	timer := time.NewTicker(25 * time.Millisecond)
	defer timer.Stop()
	consecutive := 0
	for {
		if piLocalProcessExited(wait) {
			return ErrPiLocalModelProcess
		}
		if exactPiLocalHealth(client, "http://"+hostPort+"/health") {
			consecutive++
			if consecutive >= piLocalStableHealthObservations {
				if piLocalProcessExited(wait) {
					return ErrPiLocalModelProcess
				}
				return nil
			}
		} else {
			consecutive = 0
		}
		select {
		case <-ctx.Done():
			return classifyPiLocalHealthContext(ctx, wait)
		case waitErr := <-wait:
			wait <- waitErr
			return ErrPiLocalModelProcess
		case <-timer.C:
		}
	}
}

func classifyPiLocalHealthContext(
	ctx context.Context,
	wait chan error,
) error {
	if piLocalProcessExited(wait) {
		return ErrPiLocalModelProcess
	}
	timer := time.NewTimer(piLocalExitClassificationGrace)
	defer timer.Stop()
	select {
	case waitErr := <-wait:
		wait <- waitErr
		return ErrPiLocalModelProcess
	case <-timer.C:
		if piLocalProcessExited(wait) {
			return ErrPiLocalModelProcess
		}
		return errors.Join(ErrPiLocalModelHealth, ctx.Err())
	}
}

func piLocalProcessExited(wait chan error) bool {
	select {
	case waitErr := <-wait:
		wait <- waitErr
		return true
	default:
		return false
	}
}

func exactPiLocalHealth(client *http.Client, endpoint string) bool {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxPiLocalHealthBody+1))
	if err != nil || len(body) > maxPiLocalHealthBody || response.StatusCode != http.StatusOK {
		return false
	}
	var compact bytes.Buffer
	if json.Compact(&compact, body) != nil {
		return false
	}
	return bytes.Equal(compact.Bytes(), []byte(`{"status":"ok"}`))
}

func drainPiLocalOutput(reader io.Reader, output chan<- error) {
	written, err := io.Copy(io.Discard, io.LimitReader(reader, piRPCMaxStderrBytes+1))
	if err == nil && written > piRPCMaxStderrBytes {
		err = ErrPiRPCOutputTooLarge
	}
	output <- err
}

func provePiLocalListenerGone(
	ctx context.Context,
	hostPort string,
	grace time.Duration,
) error {
	deadline := time.Now().Add(grace)
	for {
		connection, err := net.DialTimeout("tcp", hostPort, 25*time.Millisecond)
		if err != nil {
			return nil
		}
		_ = connection.Close()
		if ctx.Err() != nil || time.Now().After(deadline) {
			return fmt.Errorf("listener still present")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
