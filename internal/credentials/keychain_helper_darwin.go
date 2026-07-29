//go:build darwin && cgo

package credentials

/*
#include <libproc.h>
#include <sys/proc_info.h>
#include <sys/sysctl.h>
#include <stdlib.h>
#include <string.h>

static int loom_process_info(
	int pid,
	char *path,
	int path_capacity,
	unsigned long long *start_seconds,
	unsigned long long *start_microseconds,
	unsigned int *uid
) {
	struct proc_bsdinfo info;
	memset(&info, 0, sizeof(info));
	int count = proc_pidinfo(
		pid,
		PROC_PIDTBSDINFO,
		0,
		&info,
		sizeof(info)
	);
	if (count != sizeof(info)) return -1;
	if (proc_pidpath(pid, path, (uint32_t)path_capacity) <= 0) return -1;
	*start_seconds = info.pbi_start_tvsec;
	*start_microseconds = info.pbi_start_tvusec;
	*uid = info.pbi_uid;
	return 0;
}

static int loom_process_args(int pid, void *buffer, size_t *size) {
	int mib[3] = { CTL_KERN, KERN_PROCARGS2, pid };
	return sysctl(mib, 3, buffer, size, NULL, 0);
}
*/
import "C"

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	keychainHelperDeadline         = 2 * time.Second
	keychainHelperFlag             = "--credential-helper"
	keychainHelperRequestFD        = 3
	keychainHelperResponseFD       = 4
	maxKeychainHelperSocketBytes   = 1024
	maxKeychainHelperRequestBytes  = 4 + 1 + 2 + 2 + 2 + 128 + maxKeychainHelperSocketBytes + 8192
	maxKeychainHelperResponseBytes = 4 + 1 + 2 + 8192
	maxKeychainHelperProcessArgs   = 64 * 1024
)

var (
	errKeychainHelperProtocol     = errors.New("credential helper protocol")
	errKeychainHelperUnauthorized = errors.New("credential helper unauthorized")
	errKeychainHelperTimeout      = errors.New("credential helper timeout")
	errKeychainHelperExit         = errors.New("credential helper exit")
)

type keychainHelperOperation uint8

const (
	keychainHelperPut keychainHelperOperation = iota + 1
	keychainHelperRead
	keychainHelperDelete
)

type keychainHelperStatus uint8

const (
	keychainHelperOK keychainHelperStatus = iota
	keychainHelperNotFound
	keychainHelperDenied
	keychainHelperUnavailable
)

type keychainHelperRequest struct {
	Operation  keychainHelperOperation
	Reference  string
	SocketPath string
	Secret     []byte
}

type keychainHelperResponse struct {
	Status keychainHelperStatus
	Secret []byte
}

type keychainHelperAttestor interface {
	Attest(*os.File, *os.File) (keychainHelperAttestation, error)
}

type keychainHelperAttestation struct {
	SocketPath string
}

type processKeychainStore struct {
	invoke func(
		context.Context,
		keychainHelperRequest,
	) (keychainHelperResponse, error)
}

type keychainHelperProcessInfo struct {
	path              string
	startSeconds      uint64
	startMicroseconds uint64
	uid               uint32
}

type keychainHelperFileIdentity struct {
	device uint64
	inode  uint64
	uid    uint32
	mode   uint32
	digest [sha256.Size]byte
}

type darwinKeychainHelperAttestor struct{}

func init() {
	newProductKeychainStore = newProcessKeychainStore
	runProductKeychainHelper = runDarwinProductKeychainHelper
}

func newProcessKeychainStore(
	executablePath,
	socketPath string,
) (SecretStore, error) {
	executablePath, err := canonicalKeychainHelperExecutable(executablePath)
	if err != nil ||
		!filepath.IsAbs(socketPath) ||
		filepath.Clean(socketPath) != socketPath {
		return nil, ErrCredentialStoreUnavailable
	}
	return &processKeychainStore{
		invoke: func(
			ctx context.Context,
			request keychainHelperRequest,
		) (keychainHelperResponse, error) {
			request.SocketPath = socketPath
			return invokeKeychainHelperProcess(
				ctx,
				executablePath,
				request,
			)
		},
	}, nil
}

func newProcessKeychainStoreForTest(
	invoke func(
		context.Context,
		keychainHelperRequest,
	) (keychainHelperResponse, error),
) *processKeychainStore {
	return &processKeychainStore{invoke: invoke}
}

func (store *processKeychainStore) Put(
	ctx context.Context,
	reference string,
	secret []byte,
) error {
	if len(secret) == 0 || len(secret) > 8192 {
		return ErrCredentialStoreUnavailable
	}
	requestSecret := append([]byte(nil), secret...)
	defer clearBytes(requestSecret)
	response, err := store.invokeBounded(ctx, keychainHelperRequest{
		Operation: keychainHelperPut,
		Reference: reference,
		Secret:    requestSecret,
	})
	defer clearBytes(response.Secret)
	return keychainHelperStoreError(response, err, false)
}

func (store *processKeychainStore) Read(
	ctx context.Context,
	reference string,
) ([]byte, error) {
	response, err := store.invokeBounded(ctx, keychainHelperRequest{
		Operation: keychainHelperRead,
		Reference: reference,
	})
	if err := keychainHelperStoreError(response, err, true); err != nil {
		clearBytes(response.Secret)
		return nil, err
	}
	if len(response.Secret) == 0 || len(response.Secret) > 8192 {
		clearBytes(response.Secret)
		return nil, ErrCredentialStoreUnavailable
	}
	return response.Secret, nil
}

func (store *processKeychainStore) Delete(
	ctx context.Context,
	reference string,
) error {
	response, err := store.invokeBounded(ctx, keychainHelperRequest{
		Operation: keychainHelperDelete,
		Reference: reference,
	})
	defer clearBytes(response.Secret)
	return keychainHelperStoreError(response, err, false)
}

func (store *processKeychainStore) invokeBounded(
	ctx context.Context,
	request keychainHelperRequest,
) (keychainHelperResponse, error) {
	if store == nil ||
		store.invoke == nil ||
		ctx == nil ||
		!validCredentialIdentifier(request.Reference, 128) {
		return keychainHelperResponse{}, ErrCredentialStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return keychainHelperResponse{}, ErrCredentialStoreUnavailable
	}
	boundedCtx, cancel := context.WithTimeout(ctx, keychainHelperDeadline)
	defer cancel()
	response, err := store.invoke(boundedCtx, request)
	if err != nil {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, errKeychainHelperTimeout) {
			return keychainHelperResponse{}, ErrCredentialStoreUnavailable
		}
		return keychainHelperResponse{}, ErrCredentialStoreUnavailable
	}
	return response, nil
}

func keychainHelperStoreError(
	response keychainHelperResponse,
	err error,
	allowSecret bool,
) error {
	if err != nil {
		return ErrCredentialStoreUnavailable
	}
	switch response.Status {
	case keychainHelperOK:
		if !allowSecret && len(response.Secret) != 0 {
			return ErrCredentialStoreUnavailable
		}
		return nil
	case keychainHelperNotFound:
		return ErrCredentialNotFound
	case keychainHelperDenied:
		return ErrCredentialStoreDenied
	case keychainHelperUnavailable:
		return ErrCredentialStoreUnavailable
	default:
		return ErrCredentialStoreUnavailable
	}
}

func invokeKeychainHelperProcess(
	ctx context.Context,
	executablePath string,
	request keychainHelperRequest,
) (keychainHelperResponse, error) {
	if ctx == nil {
		return keychainHelperResponse{}, errKeychainHelperProtocol
	}
	if err := ctx.Err(); err != nil {
		return keychainHelperResponse{}, errKeychainHelperTimeout
	}
	encoded, err := encodeKeychainHelperRequest(request)
	if err != nil {
		return keychainHelperResponse{}, err
	}
	defer clearBytes(encoded)

	requestRead, requestWrite, err := os.Pipe()
	if err != nil {
		return keychainHelperResponse{}, errKeychainHelperProtocol
	}
	responseRead, responseWrite, err := os.Pipe()
	if err != nil {
		_ = requestRead.Close()
		_ = requestWrite.Close()
		return keychainHelperResponse{}, errKeychainHelperProtocol
	}
	defer requestRead.Close()
	defer requestWrite.Close()
	defer responseRead.Close()
	defer responseWrite.Close()

	output := &keychainHelperOutput{}
	command := exec.Command(executablePath, keychainHelperFlag)
	command.Env = []string{}
	command.Stdin = nil
	command.Stdout = output
	command.Stderr = output
	command.ExtraFiles = []*os.File{requestRead, responseWrite}
	if err := command.Start(); err != nil {
		return keychainHelperResponse{}, errKeychainHelperExit
	}
	_ = requestRead.Close()
	_ = responseWrite.Close()

	if _, err := requestWrite.Write(encoded); err != nil {
		_ = requestWrite.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
		return keychainHelperResponse{}, errKeychainHelperProtocol
	}
	if err := requestWrite.Close(); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return keychainHelperResponse{}, errKeychainHelperProtocol
	}

	type responseResult struct {
		bytes []byte
		err   error
	}
	responseDone := make(chan responseResult, 1)
	go func() {
		body, readErr := readBoundedKeychainHelperBytes(
			responseRead,
			maxKeychainHelperResponseBytes,
		)
		responseDone <- responseResult{bytes: body, err: readErr}
	}()
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- command.Wait()
	}()

	var (
		body        []byte
		responseErr error
		waitErr     error
		gotResponse bool
		gotWait     bool
	)
	for !gotResponse || !gotWait {
		select {
		case result := <-responseDone:
			body = result.bytes
			responseErr = result.err
			gotResponse = true
		case waitErr = <-waitDone:
			gotWait = true
		case <-ctx.Done():
			_ = command.Process.Kill()
			if !gotWait {
				waitErr = <-waitDone
				gotWait = true
			}
			if !gotResponse {
				_ = responseRead.Close()
				result := <-responseDone
				clearBytes(result.bytes)
				gotResponse = true
			}
			clearBytes(body)
			return keychainHelperResponse{}, errKeychainHelperTimeout
		}
	}
	defer clearBytes(body)
	if responseErr != nil ||
		waitErr != nil ||
		output.HasOutput() {
		return keychainHelperResponse{}, errKeychainHelperExit
	}
	return decodeKeychainHelperResponse(body)
}

type keychainHelperOutput struct {
	mu       sync.Mutex
	hasBytes bool
}

func (output *keychainHelperOutput) Write(value []byte) (int, error) {
	output.mu.Lock()
	output.hasBytes = output.hasBytes || len(value) != 0
	output.mu.Unlock()
	return len(value), nil
}

func (output *keychainHelperOutput) HasOutput() bool {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.hasBytes
}

func runDarwinProductKeychainHelper() error {
	request := os.NewFile(
		uintptr(keychainHelperRequestFD),
		"credential-helper-request",
	)
	response := os.NewFile(
		uintptr(keychainHelperResponseFD),
		"credential-helper-response",
	)
	if request == nil || response == nil {
		return errKeychainHelperUnauthorized
	}
	defer request.Close()
	defer response.Close()
	store, err := NewKeychainStore(KeychainStoreConfig{})
	if err != nil {
		return errKeychainHelperUnauthorized
	}
	return runKeychainHelper(
		request,
		response,
		darwinKeychainHelperAttestor{},
		store,
	)
}

func runKeychainHelper(
	requestFile,
	responseFile *os.File,
	attestor keychainHelperAttestor,
	store SecretStore,
) error {
	if requestFile == nil ||
		responseFile == nil ||
		attestor == nil ||
		interfaceNil(store) {
		return errKeychainHelperUnauthorized
	}
	if err := validateKeychainHelperDescriptors(
		requestFile,
		responseFile,
	); err != nil {
		return err
	}
	attestation, err := attestor.Attest(requestFile, responseFile)
	if err != nil {
		return errKeychainHelperUnauthorized
	}
	requestBytes, err := readBoundedKeychainHelperBytes(
		requestFile,
		maxKeychainHelperRequestBytes,
	)
	if err != nil {
		return errKeychainHelperProtocol
	}
	defer clearBytes(requestBytes)
	request, err := decodeKeychainHelperRequest(requestBytes)
	if err != nil {
		return errKeychainHelperProtocol
	}
	defer clearBytes(request.Secret)
	if request.SocketPath != attestation.SocketPath {
		return errKeychainHelperUnauthorized
	}

	response := executeKeychainHelperRequest(store, request)
	defer clearBytes(response.Secret)
	encoded, err := encodeKeychainHelperResponse(response)
	if err != nil {
		return errKeychainHelperProtocol
	}
	defer clearBytes(encoded)
	if err := writeAllKeychainHelperBytes(responseFile, encoded); err != nil {
		return errKeychainHelperProtocol
	}
	return nil
}

func executeKeychainHelperRequest(
	store SecretStore,
	request keychainHelperRequest,
) keychainHelperResponse {
	ctx := context.Background()
	switch request.Operation {
	case keychainHelperPut:
		err := store.Put(ctx, request.Reference, request.Secret)
		return keychainHelperResponse{Status: keychainHelperStatusForError(err)}
	case keychainHelperRead:
		secret, err := store.Read(ctx, request.Reference)
		if err != nil {
			clearBytes(secret)
			return keychainHelperResponse{
				Status: keychainHelperStatusForError(err),
			}
		}
		return keychainHelperResponse{
			Status: keychainHelperOK,
			Secret: secret,
		}
	case keychainHelperDelete:
		err := store.Delete(ctx, request.Reference)
		return keychainHelperResponse{Status: keychainHelperStatusForError(err)}
	default:
		return keychainHelperResponse{Status: keychainHelperUnavailable}
	}
}

func keychainHelperStatusForError(err error) keychainHelperStatus {
	switch {
	case err == nil:
		return keychainHelperOK
	case errors.Is(err, ErrCredentialNotFound):
		return keychainHelperNotFound
	case errors.Is(err, ErrCredentialStoreDenied):
		return keychainHelperDenied
	default:
		return keychainHelperUnavailable
	}
}

func encodeKeychainHelperRequest(
	request keychainHelperRequest,
) ([]byte, error) {
	if !validCredentialIdentifier(request.Reference, 128) ||
		!filepath.IsAbs(request.SocketPath) ||
		filepath.Clean(request.SocketPath) != request.SocketPath ||
		len(request.SocketPath) > maxKeychainHelperSocketBytes {
		return nil, errKeychainHelperProtocol
	}
	switch request.Operation {
	case keychainHelperPut:
		if len(request.Secret) == 0 || len(request.Secret) > 8192 {
			return nil, errKeychainHelperProtocol
		}
	case keychainHelperRead, keychainHelperDelete:
		if len(request.Secret) != 0 {
			return nil, errKeychainHelperProtocol
		}
	default:
		return nil, errKeychainHelperProtocol
	}
	body := make(
		[]byte,
		4+1+2+2+2+
			len(request.Reference)+len(request.SocketPath)+len(request.Secret),
	)
	copy(body[:4], "LKH1")
	body[4] = byte(request.Operation)
	binary.BigEndian.PutUint16(body[5:7], uint16(len(request.Reference)))
	binary.BigEndian.PutUint16(body[7:9], uint16(len(request.SocketPath)))
	binary.BigEndian.PutUint16(body[9:11], uint16(len(request.Secret)))
	copy(body[11:], request.Reference)
	copy(body[11+len(request.Reference):], request.SocketPath)
	copy(
		body[11+len(request.Reference)+len(request.SocketPath):],
		request.Secret,
	)
	return body, nil
}

func decodeKeychainHelperRequest(
	body []byte,
) (keychainHelperRequest, error) {
	if len(body) < 11 || !bytes.Equal(body[:4], []byte("LKH1")) {
		return keychainHelperRequest{}, errKeychainHelperProtocol
	}
	referenceLength := int(binary.BigEndian.Uint16(body[5:7]))
	socketLength := int(binary.BigEndian.Uint16(body[7:9]))
	secretLength := int(binary.BigEndian.Uint16(body[9:11]))
	if len(body) != 11+referenceLength+socketLength+secretLength {
		return keychainHelperRequest{}, errKeychainHelperProtocol
	}
	socketStart := 11 + referenceLength
	secretStart := socketStart + socketLength
	request := keychainHelperRequest{
		Operation:  keychainHelperOperation(body[4]),
		Reference:  string(body[11:socketStart]),
		SocketPath: string(body[socketStart:secretStart]),
		Secret: append(
			[]byte(nil),
			body[secretStart:]...,
		),
	}
	if _, err := encodeKeychainHelperRequest(request); err != nil {
		clearBytes(request.Secret)
		return keychainHelperRequest{}, err
	}
	return request, nil
}

func encodeKeychainHelperResponse(
	response keychainHelperResponse,
) ([]byte, error) {
	switch response.Status {
	case keychainHelperOK:
		if len(response.Secret) > 8192 {
			return nil, errKeychainHelperProtocol
		}
	case keychainHelperNotFound,
		keychainHelperDenied,
		keychainHelperUnavailable:
		if len(response.Secret) != 0 {
			return nil, errKeychainHelperProtocol
		}
	default:
		return nil, errKeychainHelperProtocol
	}
	body := make([]byte, 4+1+2+len(response.Secret))
	copy(body[:4], "LKR1")
	body[4] = byte(response.Status)
	binary.BigEndian.PutUint16(body[5:7], uint16(len(response.Secret)))
	copy(body[7:], response.Secret)
	return body, nil
}

func decodeKeychainHelperResponse(
	body []byte,
) (keychainHelperResponse, error) {
	if len(body) < 7 || !bytes.Equal(body[:4], []byte("LKR1")) {
		return keychainHelperResponse{}, errKeychainHelperProtocol
	}
	secretLength := int(binary.BigEndian.Uint16(body[5:7]))
	if len(body) != 7+secretLength {
		return keychainHelperResponse{}, errKeychainHelperProtocol
	}
	response := keychainHelperResponse{
		Status: keychainHelperStatus(body[4]),
		Secret: append([]byte(nil), body[7:]...),
	}
	if _, err := encodeKeychainHelperResponse(response); err != nil {
		clearBytes(response.Secret)
		return keychainHelperResponse{}, err
	}
	if response.Status == keychainHelperOK &&
		len(response.Secret) == 0 {
		return response, nil
	}
	return response, nil
}

func readBoundedKeychainHelperBytes(
	reader io.Reader,
	maximum int,
) ([]byte, error) {
	if reader == nil || maximum <= 0 {
		return nil, errKeychainHelperProtocol
	}
	body, err := io.ReadAll(io.LimitReader(reader, int64(maximum)+1))
	if err != nil || len(body) == 0 || len(body) > maximum {
		clearBytes(body)
		return nil, errKeychainHelperProtocol
	}
	return body, nil
}

func writeAllKeychainHelperBytes(writer io.Writer, body []byte) error {
	for len(body) != 0 {
		count, err := writer.Write(body)
		if err != nil || count <= 0 {
			return errKeychainHelperProtocol
		}
		body = body[count:]
	}
	return nil
}

func validateKeychainHelperDescriptors(
	request,
	response *os.File,
) error {
	for _, descriptor := range []*os.File{request, response} {
		info, err := descriptor.Stat()
		stat, statOK := infoSyscallStat(info)
		if err != nil ||
			info.Mode()&os.ModeNamedPipe == 0 ||
			info.Mode().IsRegular() ||
			info.Mode()&os.ModeCharDevice != 0 ||
			!statOK ||
			stat.Nlink != 0 {
			return errKeychainHelperUnauthorized
		}
	}
	return nil
}

func infoSyscallStat(info os.FileInfo) (*syscall.Stat_t, bool) {
	if info == nil {
		return nil, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return stat, ok
}

func (darwinKeychainHelperAttestor) Attest(
	request,
	response *os.File,
) (keychainHelperAttestation, error) {
	if err := validateKeychainHelperDescriptors(request, response); err != nil {
		return keychainHelperAttestation{}, err
	}
	if request.Fd() != keychainHelperRequestFD ||
		response.Fd() != keychainHelperResponseFD {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	parentPID := os.Getppid()
	if parentPID <= 1 {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	before, err := readKeychainHelperProcessInfo(parentPID)
	if err != nil || before.uid != uint32(os.Geteuid()) {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	childPath, err := os.Executable()
	if err != nil {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	childPath, err = canonicalKeychainHelperExecutable(childPath)
	if err != nil {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	parentPath, err := canonicalKeychainHelperExecutable(before.path)
	if err != nil {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	parentIdentity, err := keychainHelperExecutableIdentity(parentPath)
	if err != nil {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	childIdentity, err := keychainHelperExecutableIdentity(childPath)
	if err != nil || parentIdentity != childIdentity {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	arguments, err := readKeychainHelperProcessArguments(parentPID)
	if err != nil {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	argumentExecutable, err := canonicalKeychainHelperExecutable(arguments[0])
	if err != nil || argumentExecutable != parentPath {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	socketPath, ok := normalProductDaemonSocket(arguments)
	if !ok || !keychainHelperSocketOwnedByCurrentUser(socketPath) {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	peerPID, err := keychainHelperSocketPeerPID(socketPath)
	if err != nil || peerPID != parentPID {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	after, err := readKeychainHelperProcessInfo(parentPID)
	if err != nil ||
		before.startSeconds != after.startSeconds ||
		before.startMicroseconds != after.startMicroseconds ||
		before.path != after.path ||
		before.uid != after.uid {
		return keychainHelperAttestation{}, errKeychainHelperUnauthorized
	}
	return keychainHelperAttestation{SocketPath: socketPath}, nil
}

func canonicalKeychainHelperExecutable(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errKeychainHelperUnauthorized
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(resolved) {
		return "", errKeychainHelperUnauthorized
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", errKeychainHelperUnauthorized
	}
	return resolved, nil
}

func keychainHelperExecutableIdentity(
	path string,
) (keychainHelperFileIdentity, error) {
	file, err := os.Open(path)
	if err != nil {
		return keychainHelperFileIdentity{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return keychainHelperFileIdentity{}, errKeychainHelperUnauthorized
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return keychainHelperFileIdentity{}, errKeychainHelperUnauthorized
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return keychainHelperFileIdentity{}, err
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return keychainHelperFileIdentity{
		device: uint64(stat.Dev),
		inode:  stat.Ino,
		uid:    stat.Uid,
		mode:   uint32(stat.Mode),
		digest: digest,
	}, nil
}

func readKeychainHelperProcessInfo(
	pid int,
) (keychainHelperProcessInfo, error) {
	path := make([]byte, 4096)
	var (
		startSeconds      C.ulonglong
		startMicroseconds C.ulonglong
		uid               C.uint
	)
	if C.loom_process_info(
		C.int(pid),
		(*C.char)(unsafe.Pointer(&path[0])),
		C.int(len(path)),
		&startSeconds,
		&startMicroseconds,
		&uid,
	) != 0 {
		return keychainHelperProcessInfo{}, errKeychainHelperUnauthorized
	}
	end := bytes.IndexByte(path, 0)
	if end <= 0 {
		return keychainHelperProcessInfo{}, errKeychainHelperUnauthorized
	}
	return keychainHelperProcessInfo{
		path:              string(path[:end]),
		startSeconds:      uint64(startSeconds),
		startMicroseconds: uint64(startMicroseconds),
		uid:               uint32(uid),
	}, nil
}

func readKeychainHelperProcessArguments(pid int) ([]string, error) {
	buffer := make([]byte, maxKeychainHelperProcessArgs)
	size := C.size_t(len(buffer))
	if C.loom_process_args(
		C.int(pid),
		unsafe.Pointer(&buffer[0]),
		&size,
	) != 0 ||
		size < 4 ||
		int(size) > len(buffer) {
		return nil, errKeychainHelperUnauthorized
	}
	buffer = buffer[:int(size)]
	argumentCount := int(binary.LittleEndian.Uint32(buffer[:4]))
	if argumentCount <= 0 || argumentCount > 512 {
		return nil, errKeychainHelperUnauthorized
	}
	cursor := 4
	for cursor < len(buffer) && buffer[cursor] != 0 {
		cursor++
	}
	for cursor < len(buffer) && buffer[cursor] == 0 {
		cursor++
	}
	arguments := make([]string, 0, argumentCount)
	for len(arguments) < argumentCount && cursor < len(buffer) {
		end := cursor
		for end < len(buffer) && buffer[end] != 0 {
			end++
		}
		if end == cursor {
			cursor++
			continue
		}
		arguments = append(arguments, string(buffer[cursor:end]))
		cursor = end + 1
	}
	if len(arguments) != argumentCount {
		return nil, errKeychainHelperUnauthorized
	}
	return arguments, nil
}

func normalProductDaemonSocket(arguments []string) (string, bool) {
	if len(arguments) == 0 {
		return "", false
	}
	for _, argument := range arguments {
		if argument == keychainHelperFlag ||
			strings.HasPrefix(argument, keychainHelperFlag+"=") {
			return "", false
		}
	}
	required := map[string]string{
		"--state":          "",
		"--isolation-root": "",
		"--socket":         "",
	}
	for index := 1; index < len(arguments); index++ {
		for flag := range required {
			if arguments[index] == flag {
				if index+1 >= len(arguments) ||
					strings.HasPrefix(arguments[index+1], "-") ||
					required[flag] != "" {
					return "", false
				}
				required[flag] = arguments[index+1]
				index++
				break
			}
			prefix := flag + "="
			if strings.HasPrefix(arguments[index], prefix) {
				if required[flag] != "" {
					return "", false
				}
				required[flag] = strings.TrimPrefix(arguments[index], prefix)
				break
			}
		}
	}
	for _, value := range required {
		if value == "" ||
			!filepath.IsAbs(value) ||
			filepath.Clean(value) != value {
			return "", false
		}
	}
	return required["--socket"], true
}

func keychainHelperSocketOwnedByCurrentUser(path string) bool {
	info, err := os.Lstat(path)
	if err != nil ||
		info.Mode()&os.ModeSocket == 0 ||
		info.Mode().Perm() != 0o600 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}

func keychainHelperSocketPeerPID(path string) (int, error) {
	dialer := net.Dialer{Timeout: 250 * time.Millisecond}
	connection, err := dialer.Dial("unix", path)
	if err != nil {
		return 0, err
	}
	defer connection.Close()
	unixConnection, ok := connection.(*net.UnixConn)
	if !ok {
		return 0, errKeychainHelperUnauthorized
	}
	raw, err := unixConnection.SyscallConn()
	if err != nil {
		return 0, err
	}
	var (
		peerPID int
		peerErr error
	)
	if err := raw.Control(func(fd uintptr) {
		peerPID, peerErr = syscall.GetsockoptInt(int(fd), 0, 0x002)
	}); err != nil {
		return 0, err
	}
	return peerPID, peerErr
}
