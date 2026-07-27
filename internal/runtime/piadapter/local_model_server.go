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
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	piLocalModelSHA256              = "cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046"
	maxPiLocalModelSize             = int64(4 << 30)
	maxPiLocalBinarySize            = int64(1 << 30)
	maxPiLocalHealthBody            = 4096
	piLocalStableHealthObservations = 3
	piLocalExitClassificationGrace  = 25 * time.Millisecond
	piLocalModelContextTokens       = 32768
	piLocalModelMaxOutputTokens     = 256
)

type PiLocalModelServerConfig struct {
	PrivateRoot    string
	ExecutablePath string
	ModelPath      string
	Host           string
	Port           int
	StartupTimeout time.Duration
	CancelGrace    time.Duration
}

type PiLocalModelServer interface {
	BaseURL() string
	Close(context.Context) error
}

type PiLocalModelServerBinding struct {
	ExecutableSHA256 string
	ModelSHA256      string
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
	public      PiLocalModelServerBinding
	root        string
	directories []piLocalDirectoryBinding
	executable  piLocalFileBinding
	model       piLocalFileBinding
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
	return startPiLocalModelServer(ctx, config, piLocalModelSHA256)
}

func InspectPiLocalModelServerBinding(
	config PiLocalModelServerConfig,
) (PiLocalModelServerBinding, error) {
	binding, err := inspectPiLocalModelServerBinding(config, piLocalModelSHA256)
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
	return startPiLocalModelServerWithPortGate(
		ctx,
		config,
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
	binding, err := inspectPiLocalModelServerBinding(config, expectedModelDigest)
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
	if _, err := hex.DecodeString(expectedModelDigest); err != nil ||
		strings.ToLower(expectedModelDigest) != expectedModelDigest {
		return piLocalServerBinding{}, ErrInvalidPiLocalModel
	}

	directories, err := bindPiLocalDirectoryChains(
		config.PrivateRoot,
		config.ExecutablePath,
		config.ModelPath,
	)
	if err != nil {
		return piLocalServerBinding{}, err
	}
	executable, err := bindPiLocalFile(
		config.ExecutablePath,
		maxPiLocalBinarySize,
		true,
		"",
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
	return piLocalServerBinding{
		public: PiLocalModelServerBinding{
			ExecutableSHA256: executable.digest,
			ModelSHA256:      model.digest,
		},
		root:        config.PrivateRoot,
		directories: directories,
		executable:  executable,
		model:       model,
	}, nil
}

func revalidatePiLocalServerBinding(
	expected piLocalServerBinding,
	config PiLocalModelServerConfig,
	expectedModelDigest string,
) error {
	current, err := inspectPiLocalModelServerBinding(config, expectedModelDigest)
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
	return true
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
		!piLocalCurrentUserOwns(opened) {
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
		!piLocalCurrentUserOwns(after) {
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
