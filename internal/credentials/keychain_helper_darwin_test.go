//go:build darwin && cgo

package credentials

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestKeychainHelperCountsAttemptBeforeProcessStart(t *testing.T) {
	contents, err := os.ReadFile("keychain_helper_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	recorder := []byte("recordProductKeychainHelperSpawnAttempt()")
	start := []byte("command.Start()")
	recorderIndex := bytes.Index(contents, recorder)
	startIndex := bytes.Index(contents, start)
	if recorderIndex < 0 || startIndex < 0 || recorderIndex > startIndex {
		t.Fatalf(
			"helper spawn instrumentation order = recorder %d, start %d",
			recorderIndex,
			startIndex,
		)
	}
	if bytes.Count(contents, recorder) != 1 {
		t.Fatalf("helper spawn recorder calls = %d, want 1", bytes.Count(contents, recorder))
	}
}

type helperTestStore struct {
	mu      sync.Mutex
	values  map[string][]byte
	calls   []string
	callErr error
}

func (store *helperTestStore) Put(
	_ context.Context,
	reference string,
	secret []byte,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.calls = append(store.calls, "put:"+reference)
	if store.callErr != nil {
		return store.callErr
	}
	if store.values == nil {
		store.values = make(map[string][]byte)
	}
	store.values[reference] = append([]byte(nil), secret...)
	return nil
}

func (store *helperTestStore) Read(
	_ context.Context,
	reference string,
) ([]byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.calls = append(store.calls, "read:"+reference)
	if store.callErr != nil {
		return nil, store.callErr
	}
	secret, ok := store.values[reference]
	if !ok {
		return nil, ErrCredentialNotFound
	}
	return append([]byte(nil), secret...), nil
}

func (store *helperTestStore) Delete(
	_ context.Context,
	reference string,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.calls = append(store.calls, "delete:"+reference)
	if store.callErr != nil {
		return store.callErr
	}
	if _, ok := store.values[reference]; !ok {
		return ErrCredentialNotFound
	}
	delete(store.values, reference)
	return nil
}

type helperTestAttestor struct {
	err   error
	calls int
}

func (attestor *helperTestAttestor) Attest(
	_ *os.File,
	_ *os.File,
) (keychainHelperAttestation, error) {
	attestor.calls++
	return keychainHelperAttestation{
		SocketPath: "/private/tmp/loomd.sock",
	}, attestor.err
}

func TestKeychainHelperStrictProtocolPutReadDelete(t *testing.T) {
	store := &helperTestStore{}
	attestor := &helperTestAttestor{}

	putResponse := runHelperProtocolFixture(
		t,
		attestor,
		store,
		keychainHelperRequest{
			Operation:  keychainHelperPut,
			Reference:  "credential-ref-helper",
			SocketPath: "/private/tmp/loomd.sock",
			Secret:     []byte{0x10, 0x20, 0x30, 0x40},
		},
	)
	if putResponse.Status != keychainHelperOK ||
		len(putResponse.Secret) != 0 {
		t.Fatalf("put response = %#v", putResponse)
	}

	readResponse := runHelperProtocolFixture(
		t,
		attestor,
		store,
		keychainHelperRequest{
			Operation:  keychainHelperRead,
			Reference:  "credential-ref-helper",
			SocketPath: "/private/tmp/loomd.sock",
		},
	)
	if readResponse.Status != keychainHelperOK ||
		!bytes.Equal(
			readResponse.Secret,
			[]byte{0x10, 0x20, 0x30, 0x40},
		) {
		t.Fatalf("read response = %#v", readResponse)
	}
	clearBytes(readResponse.Secret)

	deleteResponse := runHelperProtocolFixture(
		t,
		attestor,
		store,
		keychainHelperRequest{
			Operation:  keychainHelperDelete,
			Reference:  "credential-ref-helper",
			SocketPath: "/private/tmp/loomd.sock",
		},
	)
	if deleteResponse.Status != keychainHelperOK ||
		len(deleteResponse.Secret) != 0 {
		t.Fatalf("delete response = %#v", deleteResponse)
	}
	if attestor.calls != 3 {
		t.Fatalf("attestation calls = %d, want 3", attestor.calls)
	}
}

func TestKeychainHelperRejectsBeforeStoreAccess(t *testing.T) {
	store := &helperTestStore{}
	attestor := &helperTestAttestor{
		err: errKeychainHelperUnauthorized,
	}
	requestRead, requestWrite, responseRead, responseWrite :=
		helperProtocolPipes(t)
	request := keychainHelperRequest{
		Operation:  keychainHelperPut,
		Reference:  "credential-ref-helper",
		SocketPath: "/private/tmp/loomd.sock",
		Secret:     []byte{0xde, 0xad, 0xbe, 0xef},
	}
	encoded, err := encodeKeychainHelperRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := requestWrite.Write(encoded); err != nil {
		t.Fatal(err)
	}
	clearBytes(encoded)
	if err := requestWrite.Close(); err != nil {
		t.Fatal(err)
	}

	err = runKeychainHelper(requestRead, responseWrite, attestor, store)
	if !errors.Is(err, errKeychainHelperUnauthorized) {
		t.Fatalf("runKeychainHelper() error = %v", err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("unauthorized helper reached store: %v", store.calls)
	}
	if info, statErr := responseRead.Stat(); statErr != nil ||
		info.Mode().IsRegular() {
		t.Fatalf("response descriptor is not a private pipe: %#v %v", info, statErr)
	}
}

func TestKeychainHelperRejectsAttestedSocketMismatchBeforeStoreAccess(
	t *testing.T,
) {
	store := &helperTestStore{}
	attestor := &helperTestAttestor{}
	requestRead, requestWrite, _, responseWrite := helperProtocolPipes(t)
	encoded, err := encodeKeychainHelperRequest(keychainHelperRequest{
		Operation:  keychainHelperPut,
		Reference:  "credential-ref-helper",
		SocketPath: "/private/tmp/different.sock",
		Secret:     []byte{0x10, 0x20},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := requestWrite.Write(encoded); err != nil {
		t.Fatal(err)
	}
	clearBytes(encoded)
	if err := requestWrite.Close(); err != nil {
		t.Fatal(err)
	}
	err = runKeychainHelper(requestRead, responseWrite, attestor, store)
	if !errors.Is(err, errKeychainHelperUnauthorized) {
		t.Fatalf("socket mismatch error = %v", err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("socket mismatch reached store: %v", store.calls)
	}
}

func TestKeychainHelperRejectsNonPrivateDescriptors(t *testing.T) {
	root := t.TempDir()
	request, err := os.OpenFile(
		filepath.Join(root, "request"),
		os.O_CREATE|os.O_EXCL|os.O_RDWR,
		0o600,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer request.Close()
	response, err := os.OpenFile(
		filepath.Join(root, "response"),
		os.O_CREATE|os.O_EXCL|os.O_RDWR,
		0o600,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Close()

	if err := validateKeychainHelperDescriptors(request, response); !errors.Is(
		err,
		errKeychainHelperUnauthorized,
	) {
		t.Fatalf("regular descriptors error = %v", err)
	}

	fifoPath := filepath.Join(root, "named-fifo")
	if err := syscall.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatal(err)
	}
	fifo, err := os.OpenFile(fifoPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer fifo.Close()
	pipeRead, pipeWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pipeRead.Close()
	defer pipeWrite.Close()
	if err := validateKeychainHelperDescriptors(
		fifo,
		pipeWrite,
	); !errors.Is(err, errKeychainHelperUnauthorized) {
		t.Fatalf("named FIFO descriptor error = %v", err)
	}
}

func TestKeychainHelperRejectsMalformedAndTrailingProtocol(t *testing.T) {
	valid, err := encodeKeychainHelperRequest(keychainHelperRequest{
		Operation:  keychainHelperRead,
		Reference:  "credential-ref-helper",
		SocketPath: "/private/tmp/loomd.sock",
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		body []byte
	}{
		{name: "unknown operation", body: func() []byte {
			body := append([]byte(nil), valid...)
			body[4] = 0xff
			return body
		}()},
		{
			name: "trailing byte",
			body: append(append([]byte(nil), valid...), 0x00),
		},
		{name: "truncated", body: append([]byte(nil), valid[:8]...)},
		{name: "wrong magic", body: append([]byte("NOPE"), valid[4:]...)},
	}
	defer clearBytes(valid)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if request, err := decodeKeychainHelperRequest(
				test.body,
			); err == nil {
				clearBytes(request.Secret)
				t.Fatal("malformed request accepted")
			}
			clearBytes(test.body)
		})
	}
}

func TestKeychainHelperRequiresNormalDaemonArguments(t *testing.T) {
	valid := []string{
		"/private/tmp/loomd",
		"--state",
		"/private/tmp/state.db",
		"--isolation-root=/private/tmp/isolation",
		"--socket",
		"/private/tmp/loomd.sock",
	}
	if socket, ok := normalProductDaemonSocket(valid); !ok ||
		socket != "/private/tmp/loomd.sock" {
		t.Fatalf("valid daemon arguments = %q, %t", socket, ok)
	}
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "direct helper",
			args: append(
				append([]string(nil), valid...),
				keychainHelperFlag,
			),
		},
		{
			name: "missing state",
			args: []string{
				"/private/tmp/loomd",
				"--isolation-root",
				"/private/tmp/isolation",
				"--socket",
				"/private/tmp/loomd.sock",
			},
		},
		{
			name: "relative socket",
			args: []string{
				"/private/tmp/loomd",
				"--state",
				"/private/tmp/state.db",
				"--isolation-root",
				"/private/tmp/isolation",
				"--socket",
				"loomd.sock",
			},
		},
		{
			name: "missing socket value",
			args: []string{
				"/private/tmp/loomd",
				"--state",
				"/private/tmp/state.db",
				"--isolation-root",
				"/private/tmp/isolation",
				"--socket",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if socket, ok := normalProductDaemonSocket(test.args); ok {
				t.Fatalf("arguments accepted with socket %q", socket)
			}
		})
	}
}

func TestKeychainHelperSocketPeerPIDUsesKernelPeerIdentity(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "loom-kh-peer-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove peer root: %v", err)
		}
	})
	socketPath := filepath.Join(root, "helper-peer.sock")
	listener, err := net.ListenUnix(
		"unix",
		&net.UnixAddr{Name: socketPath, Net: "unix"},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	listener.SetUnlinkOnClose(true)
	if err := os.Chmod(socketPath, 0o600); err != nil {
		t.Fatal(err)
	}
	accepted := make(chan *net.UnixConn, 1)
	go func() {
		connection, _ := listener.AcceptUnix()
		accepted <- connection
	}()
	peerPID, err := keychainHelperSocketPeerPID(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	connection := <-accepted
	if connection != nil {
		defer connection.Close()
	}
	if peerPID != os.Getpid() {
		t.Fatalf("peer PID = %d, want %d", peerPID, os.Getpid())
	}
}

func TestKeychainHelperProcessOutputFailsClosed(t *testing.T) {
	executable := "/bin/echo"
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		keychainHelperDeadline+time.Second,
	)
	defer cancel()
	_, err := invokeKeychainHelperProcess(
		ctx,
		executable,
		keychainHelperRequest{
			Operation:  keychainHelperRead,
			Reference:  "credential-ref-output",
			SocketPath: "/private/tmp/loomd.sock",
		},
	)
	if err == nil {
		t.Fatalf("output-producing helper error = %v", err)
	}
	if stage := CredentialFailureStage(err); stage != CredentialStageHelperExit {
		t.Fatalf("output-producing helper stage = %q", stage)
	}
}

func TestKeychainHelperCancellationKillsAndJoinsFixture(t *testing.T) {
	executable := buildKeychainHelperProcessFixture(
		t,
		`package main

import "time"

func main() {
	time.Sleep(30 * time.Second)
}
`,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := invokeKeychainHelperProcess(
		ctx,
		executable,
		keychainHelperRequest{
			Operation:  keychainHelperRead,
			Reference:  "credential-ref-timeout",
			SocketPath: "/private/tmp/loomd.sock",
		},
	)
	elapsed := time.Since(started)
	if !errors.Is(err, errKeychainHelperTimeout) {
		t.Fatalf("hanging helper error = %v", err)
	}
	if stage := CredentialFailureStage(err); stage != CredentialStageHelperTimeout {
		t.Fatalf("hanging helper stage = %q", stage)
	}
	if elapsed < 50*time.Millisecond || elapsed > time.Second {
		t.Fatalf("hanging helper elapsed = %s", elapsed)
	}
}

func TestKeychainHelperMalformedResponseFailsClosed(t *testing.T) {
	executable := buildKeychainHelperProcessFixture(
		t,
		`package main

import (
	"io"
	"os"
)

func main() {
	request := os.NewFile(3, "request")
	response := os.NewFile(4, "response")
	_, _ = io.ReadAll(request)
	_, _ = response.Write([]byte("malformed"))
	_ = request.Close()
	_ = response.Close()
}
`,
	)
	ctx, cancel := context.WithTimeout(
		context.Background(),
		keychainHelperDeadline+time.Second,
	)
	defer cancel()
	_, err := invokeKeychainHelperProcess(
		ctx,
		executable,
		keychainHelperRequest{
			Operation:  keychainHelperRead,
			Reference:  "credential-ref-malformed",
			SocketPath: "/private/tmp/loomd.sock",
		},
	)
	if !errors.Is(err, errKeychainHelperProtocol) &&
		!errors.Is(err, errKeychainHelperTimeout) {
		t.Fatalf("malformed helper response error = %v", err)
	}
	wantStage := CredentialStageHelperResponse
	if errors.Is(err, errKeychainHelperTimeout) {
		wantStage = CredentialStageHelperTimeout
	}
	if stage := CredentialFailureStage(err); stage != wantStage {
		t.Fatalf("malformed helper stage = %q, want %q", stage, wantStage)
	}
}

func TestProcessKeychainStorePreservesHelperAuthorizationStage(t *testing.T) {
	store := newProcessKeychainStoreForTest(func(
		context.Context,
		keychainHelperRequest,
	) (keychainHelperResponse, error) {
		return keychainHelperResponse{}, withCredentialFailureStage(
			CredentialStageHelperAuthorization,
			errKeychainHelperUnauthorized,
		)
	})
	_, err := store.Read(context.Background(), "credential-ref-stage")
	if !errors.Is(err, ErrCredentialStoreUnavailable) ||
		CredentialFailureStage(err) != CredentialStageHelperAuthorization ||
		strings.Contains(err.Error(), CredentialStageHelperAuthorization) {
		t.Fatalf("authorization failure = %v stage=%q", err,
			CredentialFailureStage(err))
	}
}

func TestKeychainHelperExecutableIdentityBindsFileMetadataAndDigest(
	t *testing.T,
) {
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := keychainHelperExecutableIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := keychainHelperExecutableIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok ||
		first != second ||
		first.inode != stat.Ino ||
		first.device != uint64(stat.Dev) ||
		first.uid != uint32(os.Geteuid()) {
		t.Fatalf("identity = %#v %#v", first, second)
	}
}

func buildKeychainHelperProcessFixture(
	t *testing.T,
	source string,
) string {
	t.Helper()
	root := t.TempDir()
	sourcePath := filepath.Join(root, "main.go")
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "fixture")
	build := exec.Command("go", "build", "-o", executable, sourcePath)
	build.Env = append([]string(nil), os.Environ()...)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper process fixture: %v output=%q", err, output)
	}
	return executable
}

func TestProcessKeychainStoreAppliesTwoSecondBudgetToEveryOperation(
	t *testing.T,
) {
	var (
		mu         sync.Mutex
		operations []keychainHelperOperation
	)
	invoker := func(
		ctx context.Context,
		request keychainHelperRequest,
	) (keychainHelperResponse, error) {
		mu.Lock()
		operations = append(operations, request.Operation)
		mu.Unlock()
		deadline, ok := ctx.Deadline()
		if !ok {
			return keychainHelperResponse{}, errors.New("missing deadline")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > keychainHelperDeadline {
			return keychainHelperResponse{}, errors.New("invalid deadline")
		}
		response := keychainHelperResponse{Status: keychainHelperOK}
		if request.Operation == keychainHelperRead {
			response.Secret = []byte{0x01}
		}
		return response, nil
	}
	store := newProcessKeychainStoreForTest(invoker)

	if err := store.Put(
		context.Background(),
		"credential-ref-budget",
		[]byte{0x01},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(
		context.Background(),
		"credential-ref-budget",
	); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(
		context.Background(),
		"credential-ref-budget",
	); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []keychainHelperOperation{
		keychainHelperPut,
		keychainHelperRead,
		keychainHelperDelete,
	}
	if len(operations) != len(want) {
		t.Fatalf("operations = %v", operations)
	}
	for index := range want {
		if operations[index] != want[index] {
			t.Fatalf("operations = %v, want %v", operations, want)
		}
	}
}

func runHelperProtocolFixture(
	t *testing.T,
	attestor keychainHelperAttestor,
	store SecretStore,
	request keychainHelperRequest,
) keychainHelperResponse {
	t.Helper()
	requestRead, requestWrite, responseRead, responseWrite :=
		helperProtocolPipes(t)
	encoded, err := encodeKeychainHelperRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := requestWrite.Write(encoded); err != nil {
		t.Fatal(err)
	}
	clearBytes(encoded)
	if err := requestWrite.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runKeychainHelper(
		requestRead,
		responseWrite,
		attestor,
		store,
	); err != nil {
		t.Fatal(err)
	}
	if err := responseWrite.Close(); err != nil {
		t.Fatal(err)
	}
	responseBytes, err := os.ReadFile(responseRead.Name())
	if err == nil {
		clearBytes(responseBytes)
		t.Fatal("pipe unexpectedly readable by pathname")
	}
	responseBytes, err = readBoundedKeychainHelperBytes(
		responseRead,
		maxKeychainHelperResponseBytes,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer clearBytes(responseBytes)
	response, err := decodeKeychainHelperResponse(responseBytes)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func helperProtocolPipes(
	t *testing.T,
) (*os.File, *os.File, *os.File, *os.File) {
	t.Helper()
	requestRead, requestWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	responseRead, responseWrite, err := os.Pipe()
	if err != nil {
		requestRead.Close()
		requestWrite.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = requestRead.Close()
		_ = requestWrite.Close()
		_ = responseRead.Close()
		_ = responseWrite.Close()
	})
	return requestRead, requestWrite, responseRead, responseWrite
}
