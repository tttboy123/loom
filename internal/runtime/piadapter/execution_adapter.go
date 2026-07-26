package piadapter

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

var (
	ErrInvalidPiExecutionAdapter = errors.New("invalid Pi execution adapter")
	ErrPiExecutionBindingChanged = errors.New("Pi execution binding changed")
	ErrPiExecutionProcess        = errors.New("Pi execution process failed")
	ErrPiExecutionProtocol       = errors.New("Pi execution protocol failed")
	ErrPiExecutionOutputTooLarge = errors.New("Pi execution output too large")
	ErrPiExecutionCleanup        = errors.New("Pi execution cleanup failed")
)

const (
	maxPiExecutableBytes = int64(64 << 20)
	maxPiArguments       = 128
	maxPiArgumentBytes   = 64 << 10
	maxPiStderrBytes     = 256 << 10
)

type PiExecutionAdapterConfig struct {
	ExecutablePath     string
	Arguments          []string
	RuntimeInstanceID  string
	RuntimeSearchPaths []string
	CancelGrace        time.Duration
	Now                func() time.Time
	Random             io.Reader
}

type piFileBinding struct {
	path   string
	info   os.FileInfo
	size   int64
	mode   os.FileMode
	digest string
}

type piDirectoryBinding struct {
	path string
	info os.FileInfo
	mode os.FileMode
}

type piExecutionAdapter struct {
	executable  piFileBinding
	arguments   []string
	instanceID  string
	searchPaths []piDirectoryBinding
	cancelGrace time.Duration
	now         func() time.Time
	random      io.Reader
	randomMu    sync.Mutex
}

type piLineResult struct {
	line []byte
	err  error
}

type piStderrResult struct {
	content []byte
	err     error
}

func NewPiExecutionAdapter(
	config PiExecutionAdapterConfig,
) (supervisor.RuntimeAdapter, error) {
	if runtime.GOOS == "windows" ||
		piNoFollowFlag() == 0 ||
		!validPiOpaqueID(config.RuntimeInstanceID) ||
		config.CancelGrace <= 0 ||
		config.CancelGrace > 5*time.Second ||
		config.Now == nil ||
		nilPiInterface(config.Random) ||
		len(config.Arguments) > maxPiArguments ||
		len(config.RuntimeSearchPaths) == 0 {
		return nil, ErrInvalidPiExecutionAdapter
	}
	now := config.Now()
	if now.IsZero() || now.Location() != time.UTC {
		return nil, ErrInvalidPiExecutionAdapter
	}
	arguments, err := copyPiArguments(config.Arguments)
	if err != nil {
		return nil, err
	}
	executable, err := bindPiExecutable(config.ExecutablePath)
	if err != nil {
		return nil, err
	}
	searchPaths := make([]piDirectoryBinding, len(config.RuntimeSearchPaths))
	seenPaths := make(map[string]struct{}, len(config.RuntimeSearchPaths))
	for index, path := range config.RuntimeSearchPaths {
		binding, bindErr := bindPiSearchDirectory(path)
		if bindErr != nil {
			return nil, bindErr
		}
		if _, duplicate := seenPaths[binding.path]; duplicate {
			return nil, ErrInvalidPiExecutionAdapter
		}
		seenPaths[binding.path] = struct{}{}
		searchPaths[index] = binding
	}
	return &piExecutionAdapter{
		executable:  executable,
		arguments:   arguments,
		instanceID:  config.RuntimeInstanceID,
		searchPaths: searchPaths,
		cancelGrace: config.CancelGrace,
		now:         config.Now,
		random:      config.Random,
	}, nil
}

func (*piExecutionAdapter) AdapterType() string {
	return "pi"
}

func (adapter *piExecutionAdapter) RuntimeInstanceID() string {
	if adapter == nil {
		return ""
	}
	return adapter.instanceID
}

func (adapter *piExecutionAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	if adapter == nil || ctx == nil {
		return supervisor.AdapterResult{}, ErrInvalidPiExecutionAdapter
	}
	if err := adapter.validateRequest(request); err != nil {
		return supervisor.AdapterResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiExecutionProcess, err)
	}
	if err := adapter.revalidateBindings(); err != nil {
		return supervisor.AdapterResult{}, err
	}

	command := exec.Command(adapter.executable.path, adapter.arguments...)
	command.Dir = request.WorkspacePath
	command.Env = []string{
		"HOME=" + request.HomePath,
		"TMPDIR=" + request.TempPath,
		"PATH=" + adapter.searchPathValue(),
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"LOOM_AGENT_GRANT=" + request.Grant.Value(),
	}
	if err := configureExecutionProcess(command); err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiExecutionCleanup, err)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiExecutionProcess, err)
	}
	stdout, childStdout, err := os.Pipe()
	if err != nil {
		_ = stdin.Close()
		return supervisor.AdapterResult{}, errors.Join(ErrPiExecutionProcess, err)
	}
	stderr, childStderr, err := os.Pipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = childStdout.Close()
		return supervisor.AdapterResult{}, errors.Join(ErrPiExecutionProcess, err)
	}
	command.Stdout = childStdout
	command.Stderr = childStderr
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		_ = childStdout.Close()
		_ = childStderr.Close()
		return supervisor.AdapterResult{}, errors.Join(ErrPiExecutionProcess, err)
	}
	defer stdout.Close()
	defer stderr.Close()

	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	if closeErr := errors.Join(childStdout.Close(), childStderr.Close()); closeErr != nil {
		return supervisor.AdapterResult{}, adapter.failProcess(
			command,
			wait,
			stdin,
			nil,
			request,
			1,
			"cancelled",
			errors.Join(ErrPiExecutionCleanup, closeErr),
		)
	}
	scanContext, cancelScan := context.WithCancel(context.Background())
	defer cancelScan()
	lines := make(chan piLineResult, 1)
	go scanPiLines(scanContext, stdout, lines)
	stderrResult := make(chan piStderrResult, 1)
	go readPiStderr(stderr, stderrResult)

	dispatchLine, err := bridgev1.EncodeLine(request.Dispatch)
	if err != nil {
		return supervisor.AdapterResult{}, adapter.failProcess(
			command, wait, stdin, nil, request, 1, "cancelled",
			errors.Join(ErrPiExecutionProtocol, err),
		)
	}
	if _, err := stdin.Write(dispatchLine); err != nil {
		return supervisor.AdapterResult{}, adapter.failProcess(
			command, wait, stdin, nil, request, 1, "cancelled",
			errors.Join(ErrPiExecutionProcess, err),
		)
	}

	stream, err := bridgev1.NewBoundRunStream(request.Binding)
	if err != nil {
		return supervisor.AdapterResult{}, adapter.failProcess(
			command, wait, stdin, nil, request, 1, "cancelled",
			errors.Join(ErrPiExecutionProtocol, err),
		)
	}
	inbound := make([]bridgev1.Frame, 0)
	dispatchAcknowledged := false
	resultAcknowledged := false
	resultSeen := false
	var lastSequence int64 = 1
	var capturedStderr *piStderrResult
	for {
		select {
		case <-ctx.Done():
			return supervisor.AdapterResult{}, adapter.failProcess(
				command, wait, stdin, nil, request, lastSequence, piCancelReason(ctx),
				errors.Join(ErrPiExecutionProcess, ctx.Err()),
			)
		case stderrValue := <-stderrResult:
			capturedStderr = &stderrValue
			stderrResult = nil
			if stderrValue.err != nil {
				primary := ErrPiExecutionProcess
				if errors.Is(stderrValue.err, ErrPiExecutionOutputTooLarge) {
					primary = ErrPiExecutionOutputTooLarge
				}
				return supervisor.AdapterResult{}, adapter.failProcess(
					command, wait, stdin, nil, request, lastSequence, "cancelled",
					errors.Join(primary, stderrValue.err),
				)
			}
		case lineResult := <-lines:
			if lineResult.err != nil {
				if (errors.Is(lineResult.err, io.EOF) ||
					errors.Is(lineResult.err, os.ErrClosed)) && resultSeen {
					goto processExit
				}
				primary := ErrPiExecutionProtocol
				if errors.Is(lineResult.err, ErrPiExecutionOutputTooLarge) {
					primary = ErrPiExecutionOutputTooLarge
				}
				return supervisor.AdapterResult{}, adapter.failProcess(
					command, wait, stdin, nil, request, lastSequence, "cancelled",
					errors.Join(primary, lineResult.err),
				)
			}
			if resultSeen {
				return supervisor.AdapterResult{}, adapter.failProcess(
					command, wait, stdin, nil, request, lastSequence, "cancelled",
					ErrPiExecutionProtocol,
				)
			}
			frame, decodeErr := bridgev1.DecodeLine(lineResult.line)
			if decodeErr != nil {
				return supervisor.AdapterResult{}, adapter.failProcess(
					command, wait, stdin, nil, request, lastSequence, "cancelled",
					errors.Join(ErrPiExecutionProtocol, decodeErr),
				)
			}
			if bytes.Contains(
				lineResult.line,
				[]byte(request.Grant.Value()),
			) {
				return supervisor.AdapterResult{}, adapter.failProcess(
					command, wait, stdin, nil, request, lastSequence, "cancelled",
					ErrPiExecutionProtocol,
				)
			}
			if frame.Type() == bridgev1.MessageDispatch ||
				frame.Type() == bridgev1.MessageCancel ||
				(frame.Type() == bridgev1.MessageAck && len(inbound) != 0) {
				return supervisor.AdapterResult{}, adapter.failProcess(
					command, wait, stdin, nil, request, lastSequence, "cancelled",
					ErrPiExecutionProtocol,
				)
			}
			stream, err = bridgev1.AdvanceBoundRunStream(stream, frame)
			if err != nil {
				return supervisor.AdapterResult{}, adapter.failProcess(
					command, wait, stdin, nil, request, lastSequence, "cancelled",
					errors.Join(ErrPiExecutionProtocol, err),
				)
			}
			lastSequence = frame.Sequence()
			if len(inbound) == 0 {
				if frame.Type() != bridgev1.MessageAck ||
					!piExactMessagePayload(frame.Payload(), request.Dispatch.MessageID()) {
					return supervisor.AdapterResult{}, adapter.failProcess(
						command, wait, stdin, nil, request, lastSequence, "cancelled",
						ErrPiExecutionProtocol,
					)
				}
				dispatchAcknowledged = true
			}
			switch frame.Type() {
			case bridgev1.MessageAck, bridgev1.MessageEvent,
				bridgev1.MessageEvidence, bridgev1.MessageHeartbeat:
			case bridgev1.MessageResult:
				if _, _, parseErr := piParseResult(frame.Payload()); parseErr != nil {
					return supervisor.AdapterResult{}, adapter.failProcess(
						command, wait, stdin, nil, request, lastSequence, "cancelled",
						parseErr,
					)
				}
				resultSeen = true
				ack, ackErr := adapter.outboundFrame(
					request,
					lastSequence+1,
					bridgev1.MessageAck,
					mustPiPayload(map[string]string{"message_id": frame.MessageID()}),
				)
				if ackErr != nil {
					return supervisor.AdapterResult{}, adapter.failProcess(
						command, wait, stdin, nil, request, lastSequence, "cancelled",
						ackErr,
					)
				}
				ackLine, ackErr := bridgev1.EncodeLine(ack)
				if ackErr != nil {
					return supervisor.AdapterResult{}, adapter.failProcess(
						command, wait, stdin, nil, request, lastSequence, "cancelled",
						errors.Join(ErrPiExecutionProtocol, ackErr),
					)
				}
				if _, ackErr := stdin.Write(ackLine); ackErr != nil {
					return supervisor.AdapterResult{}, adapter.failProcess(
						command, wait, stdin, nil, request, lastSequence, "cancelled",
						errors.Join(ErrPiExecutionProcess, ackErr),
					)
				}
				resultAcknowledged = true
				lastSequence++
				if closeErr := stdin.Close(); closeErr != nil {
					return supervisor.AdapterResult{}, adapter.failProcess(
						command, wait, stdin, nil, request, lastSequence, "cancelled",
						errors.Join(ErrPiExecutionCleanup, closeErr),
					)
				}
			default:
				return supervisor.AdapterResult{}, adapter.failProcess(
					command, wait, stdin, nil, request, lastSequence, "cancelled",
					ErrPiExecutionProtocol,
				)
			}
			inbound = append(inbound, frame)
		}
	}

processExit:
	var waitErr error
	select {
	case <-ctx.Done():
		return supervisor.AdapterResult{}, adapter.failProcess(
			command, wait, stdin, nil, request, lastSequence, piCancelReason(ctx),
			errors.Join(ErrPiExecutionProcess, ctx.Err()),
		)
	case waitErr = <-wait:
	}
	var stderrOutput piStderrResult
	if capturedStderr != nil {
		stderrOutput = *capturedStderr
	} else {
		stderrOutput = <-stderrResult
	}
	if stderrOutput.err != nil {
		primary := ErrPiExecutionProcess
		if errors.Is(stderrOutput.err, ErrPiExecutionOutputTooLarge) {
			primary = ErrPiExecutionOutputTooLarge
		}
		return supervisor.AdapterResult{}, errors.Join(primary, stderrOutput.err)
	}
	if bytes.Contains(stderrOutput.content, []byte(request.Grant.Value())) {
		return supervisor.AdapterResult{}, ErrPiExecutionProtocol
	}
	if !resultSeen || !dispatchAcknowledged || !resultAcknowledged {
		return supervisor.AdapterResult{}, ErrPiExecutionProtocol
	}
	exitCode := command.ProcessState.ExitCode()
	if waitErr != nil || exitCode != 0 {
		return supervisor.AdapterResult{}, errors.Join(ErrPiExecutionProcess, waitErr)
	}
	result, err := supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames:        inbound,
		Stderr:               stderrOutput.content,
		ExitCode:             exitCode,
		DispatchAcknowledged: dispatchAcknowledged,
		ResultAcknowledged:   resultAcknowledged,
	})
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrPiExecutionProtocol, err)
	}
	return result, nil
}

func (adapter *piExecutionAdapter) validateRequest(
	request supervisor.AdapterRequest,
) error {
	if request.Binding.RuntimeInstanceID != adapter.instanceID ||
		request.Dispatch.WorkItemID() != request.Binding.WorkItemID ||
		request.Dispatch.RunID() != request.Binding.RunID ||
		request.Dispatch.ClaimGeneration() != request.Binding.ClaimGeneration ||
		request.Dispatch.RuntimeInstanceID() != request.Binding.RuntimeInstanceID ||
		request.Dispatch.SenderAgentInstanceID() != request.Binding.SenderAgentInstanceID ||
		request.Dispatch.Type() != bridgev1.MessageDispatch ||
		request.Dispatch.Sequence() != 1 ||
		request.Grant.Value() == "" {
		return ErrPiExecutionBindingChanged
	}
	for _, path := range []string{
		request.WorkspacePath,
		request.HomePath,
		request.TempPath,
	} {
		if err := validatePiPrivateDirectory(path); err != nil {
			return errors.Join(ErrPiExecutionBindingChanged, err)
		}
	}
	return nil
}

func (adapter *piExecutionAdapter) revalidateBindings() error {
	current, err := bindPiExecutable(adapter.executable.path)
	if err != nil ||
		!os.SameFile(current.info, adapter.executable.info) ||
		current.size != adapter.executable.size ||
		current.mode != adapter.executable.mode ||
		current.digest != adapter.executable.digest {
		return errors.Join(ErrPiExecutionBindingChanged, err)
	}
	for _, expected := range adapter.searchPaths {
		current, bindErr := bindPiSearchDirectory(expected.path)
		if bindErr != nil ||
			!os.SameFile(current.info, expected.info) ||
			current.mode != expected.mode {
			return errors.Join(ErrPiExecutionBindingChanged, bindErr)
		}
	}
	return nil
}

func (adapter *piExecutionAdapter) searchPathValue() string {
	paths := make([]string, len(adapter.searchPaths))
	for index, binding := range adapter.searchPaths {
		paths[index] = binding.path
	}
	return strings.Join(paths, string(os.PathListSeparator))
}

func (adapter *piExecutionAdapter) failProcess(
	command *exec.Cmd,
	wait <-chan error,
	stdin io.WriteCloser,
	_ io.Closer,
	request supervisor.AdapterRequest,
	lastSequence int64,
	reason string,
	primary error,
) error {
	if stdin != nil {
		cancel, err := adapter.outboundFrame(
			request,
			lastSequence+1,
			bridgev1.MessageCancel,
			mustPiPayload(map[string]string{"reason": reason}),
		)
		if err == nil {
			if line, encodeErr := bridgev1.EncodeLine(cancel); encodeErr == nil {
				_, _ = stdin.Write(line)
			}
		}
		_ = stdin.Close()
	}
	cleanupErr := terminateExecutionProcess(command, wait, adapter.cancelGrace)
	if cleanupErr != nil {
		cleanupErr = errors.Join(ErrPiExecutionCleanup, cleanupErr)
	}
	return errors.Join(primary, cleanupErr)
}

func (adapter *piExecutionAdapter) outboundFrame(
	request supervisor.AdapterRequest,
	sequence int64,
	messageType bridgev1.MessageType,
	payload []byte,
) (bridgev1.Frame, error) {
	messageID, err := adapter.randomUUID()
	if err != nil {
		return bridgev1.Frame{}, err
	}
	now := adapter.now()
	if now.IsZero() || now.Location() != time.UTC {
		return bridgev1.Frame{}, ErrInvalidPiExecutionAdapter
	}
	frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             messageID,
		CorrelationID:         request.Dispatch.CorrelationID(),
		WorkItemID:            request.Binding.WorkItemID,
		RunID:                 request.Binding.RunID,
		ClaimGeneration:       request.Binding.ClaimGeneration,
		RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
		SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
		Sequence:              sequence,
		Type:                  messageType,
		EmittedAt:             now,
		Payload:               payload,
	})
	if err != nil {
		return bridgev1.Frame{}, errors.Join(ErrPiExecutionProtocol, err)
	}
	return frame, nil
}

func (adapter *piExecutionAdapter) randomUUID() (string, error) {
	material := make([]byte, 16)
	adapter.randomMu.Lock()
	_, err := io.ReadFull(adapter.random, material)
	adapter.randomMu.Unlock()
	if err != nil {
		return "", errors.Join(ErrInvalidPiExecutionAdapter, err)
	}
	material[6] = material[6]&0x0f | 0x40
	material[8] = material[8]&0x3f | 0x80
	encoded := hex.EncodeToString(material)
	return encoded[0:8] + "-" +
		encoded[8:12] + "-" +
		encoded[12:16] + "-" +
		encoded[16:20] + "-" +
		encoded[20:32], nil
}

func bindPiExecutable(path string) (piFileBinding, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return piFileBinding{}, ErrInvalidPiExecutionAdapter
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(resolved) || filepath.Clean(resolved) != resolved {
		return piFileBinding{}, errors.Join(ErrInvalidPiExecutionAdapter, err)
	}
	info, err := os.Lstat(resolved)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode().Perm()&0o111 == 0 ||
		info.Size() <= 0 ||
		info.Size() > maxPiExecutableBytes {
		return piFileBinding{}, errors.Join(ErrInvalidPiExecutionAdapter, err)
	}
	file, err := os.OpenFile(resolved, os.O_RDONLY|piNoFollowFlag(), 0)
	if err != nil {
		return piFileBinding{}, errors.Join(ErrInvalidPiExecutionAdapter, err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return piFileBinding{}, errors.Join(ErrInvalidPiExecutionAdapter, err)
	}
	content, err := io.ReadAll(io.LimitReader(file, maxPiExecutableBytes+1))
	if err != nil || int64(len(content)) != opened.Size() {
		return piFileBinding{}, errors.Join(ErrInvalidPiExecutionAdapter, err)
	}
	after, err := os.Lstat(resolved)
	if err != nil ||
		!os.SameFile(opened, after) ||
		after.Mode() != opened.Mode() ||
		after.Size() != opened.Size() {
		return piFileBinding{}, errors.Join(ErrInvalidPiExecutionAdapter, err)
	}
	digest := sha256.Sum256(content)
	return piFileBinding{
		path:   resolved,
		info:   after,
		size:   after.Size(),
		mode:   after.Mode(),
		digest: hex.EncodeToString(digest[:]),
	}, nil
}

func bindPiSearchDirectory(path string) (piDirectoryBinding, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return piDirectoryBinding{}, ErrInvalidPiExecutionAdapter
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return piDirectoryBinding{}, errors.Join(ErrInvalidPiExecutionAdapter, err)
	}
	handle, err := os.Open(path)
	if err != nil {
		return piDirectoryBinding{}, errors.Join(ErrInvalidPiExecutionAdapter, err)
	}
	defer handle.Close()
	opened, err := handle.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return piDirectoryBinding{}, errors.Join(ErrInvalidPiExecutionAdapter, err)
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(opened, after) || after.Mode() != opened.Mode() {
		return piDirectoryBinding{}, errors.Join(ErrInvalidPiExecutionAdapter, err)
	}
	return piDirectoryBinding{path: path, info: after, mode: after.Mode()}, nil
}

func validatePiPrivateDirectory(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return ErrPiExecutionBindingChanged
	}
	info, err := os.Lstat(path)
	if err != nil ||
		!info.IsDir() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o700 {
		return errors.Join(ErrPiExecutionBindingChanged, err)
	}
	return nil
}

func copyPiArguments(arguments []string) ([]string, error) {
	output := make([]string, len(arguments))
	total := 0
	for index, argument := range arguments {
		if !utf8.ValidString(argument) || strings.IndexByte(argument, 0) >= 0 {
			return nil, ErrInvalidPiExecutionAdapter
		}
		total += len(argument)
		if total > maxPiArgumentBytes {
			return nil, ErrInvalidPiExecutionAdapter
		}
		output[index] = argument
	}
	return output, nil
}

func scanPiLines(ctx context.Context, reader io.Reader, output chan<- piLineResult) {
	buffer := bufio.NewReaderSize(reader, bridgev1.MaxLineBytes+1)
	for {
		line, err := buffer.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			err = ErrPiExecutionOutputTooLarge
		} else if err == nil {
			line = bytes.Clone(line)
		} else if len(line) > 0 {
			err = ErrPiExecutionProtocol
		}
		select {
		case output <- piLineResult{line: line, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func readPiStderr(reader io.Reader, output chan<- piStderrResult) {
	content, err := io.ReadAll(io.LimitReader(reader, maxPiStderrBytes+1))
	if err == nil && len(content) > maxPiStderrBytes {
		content = nil
		err = ErrPiExecutionOutputTooLarge
	}
	output <- piStderrResult{content: content, err: err}
}

func piExactMessagePayload(payload []byte, messageID string) bool {
	fields, err := piDecodeStringObject(payload)
	return err == nil && len(fields) == 1 && fields["message_id"] == messageID
}

func piParseResult(payload []byte) (string, string, error) {
	fields, err := piDecodeStringObject(payload)
	if err != nil || len(fields) != 2 {
		return "", "", ErrPiExecutionProtocol
	}
	status, statusFound := fields["status"]
	reason, reasonFound := fields["reason"]
	if !statusFound || !reasonFound {
		return "", "", ErrPiExecutionProtocol
	}
	switch status {
	case "succeeded":
		if reason != "" {
			return "", "", ErrPiExecutionProtocol
		}
	case "failed":
		if reason == "" ||
			len(reason) > 1024 ||
			!utf8.ValidString(reason) ||
			strings.TrimSpace(reason) != reason {
			return "", "", ErrPiExecutionProtocol
		}
		for _, character := range reason {
			if unicode.IsControl(character) {
				return "", "", ErrPiExecutionProtocol
			}
		}
	default:
		return "", "", ErrPiExecutionProtocol
	}
	return status, reason, nil
}

func piDecodeStringObject(payload []byte) (map[string]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, ErrPiExecutionProtocol
	}
	fields := make(map[string]string)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, ErrPiExecutionProtocol
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, ErrPiExecutionProtocol
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, ErrPiExecutionProtocol
		}
		var value string
		if err := decoder.Decode(&value); err != nil {
			return nil, ErrPiExecutionProtocol
		}
		fields[key] = value
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') {
		return nil, ErrPiExecutionProtocol
	}
	if token, err = decoder.Token(); !errors.Is(err, io.EOF) || token != nil {
		return nil, ErrPiExecutionProtocol
	}
	return fields, nil
}

func mustPiPayload(value any) []byte {
	payload, _ := json.Marshal(value)
	return payload
}

func piCancelReason(ctx context.Context) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timeout"
	}
	return "cancelled"
}

func validPiOpaqueID(value string) bool {
	if value == "" || len(value) > bridgev1.MaxOpaqueIDBytes || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

func piNoFollowFlag() int {
	switch runtime.GOOS {
	case "darwin", "dragonfly", "freebsd", "netbsd", "openbsd":
		return 0x100
	case "linux", "solaris":
		return 0x20000
	default:
		return 0
	}
}

func nilPiInterface(value any) bool {
	if value == nil {
		return true
	}
	reflection := reflect.ValueOf(value)
	switch reflection.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflection.IsNil()
	default:
		return false
	}
}
