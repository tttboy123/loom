package localipc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

const (
	connectionDeadline       = 5 * time.Second
	extendedRequestDeadline  = 10 * time.Second
	extendedResponseDeadline = extendedRequestDeadline + 2*time.Second
	missionRequestDeadline   = 180 * time.Second
	missionResponseDeadline  = missionRequestDeadline + 2*time.Second
	chatRequestDeadline      = 1805 * time.Second
	chatResponseDeadline     = chatRequestDeadline + 2*time.Second
	agentRecoveryDeadline    = 50 * time.Second
	agentRecoveryResponse    = agentRecoveryDeadline + 2*time.Second
	maxConnections           = 16
)

type Handler interface {
	// Handle is a trusted in-process boundary. Implementations must observe
	// ctx.Done and return; Server joins every accepted connection during Close
	// and deliberately does not detach handler work into an untracked goroutine.
	Handle(context.Context, Request) Response
}

type HandlerFunc func(context.Context, Request) Response

func (function HandlerFunc) Handle(
	ctx context.Context,
	request Request,
) Response {
	return function(ctx, request)
}

type ServerConfig struct {
	SocketPath   string
	EffectiveUID int
	BuildID      string
	Handler      Handler
}

type Server struct {
	config ServerConfig

	mu             sync.Mutex
	listener       *net.UnixListener
	serveCancel    context.CancelFunc
	socketIdentity fileIdentity
	lock           *os.File
	lockIdentity   fileIdentity
	connections    map[*net.UnixConn]struct{}
	closed         bool
	closeErr       error
	closeOnce      sync.Once
	readyOnce      sync.Once
	wait           sync.WaitGroup
	slots          chan struct{}
	ready          chan struct{}
	closeDone      chan struct{}
}

func NewServer(config ServerConfig) (*Server, error) {
	if config.EffectiveUID < 0 ||
		config.BuildID == "" ||
		config.Handler == nil ||
		validateSocketPath(config.SocketPath, config.EffectiveUID) != nil {
		return nil, ErrInvalidSocketPath
	}
	return &Server{
		config:      config,
		connections: make(map[*net.UnixConn]struct{}),
		slots:       make(chan struct{}, maxConnections),
		ready:       make(chan struct{}),
		closeDone:   make(chan struct{}),
	}, nil
}

func (server *Server) Ready() <-chan struct{} {
	return server.ready
}

func (server *Server) Serve(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	server.mu.Lock()
	if server.closed || server.listener != nil {
		server.mu.Unlock()
		return ErrInvalidProtocol
	}
	server.mu.Unlock()

	lock, err := prepareSocket(
		server.config.SocketPath,
		server.config.EffectiveUID,
	)
	if err != nil {
		return err
	}
	lockInfo, err := lock.Stat()
	lockIdentity, identityOK := identityOf(lockInfo)
	if err != nil || !identityOK ||
		!validLockInfo(lockInfo, server.config.EffectiveUID) ||
		!pathMatchesOwnedLock(
			server.config.SocketPath+".lock",
			lockIdentity,
			server.config.EffectiveUID,
		) {
		_ = releaseLockFile(server.config.SocketPath+".lock", lock)
		return ErrInvalidSocketPath
	}
	listener, err := net.ListenUnix(
		"unix",
		&net.UnixAddr{Name: server.config.SocketPath, Net: "unix"},
	)
	if err != nil {
		_ = releaseLockFile(server.config.SocketPath+".lock", lock)
		return ErrInvalidSocketPath
	}
	listener.SetUnlinkOnClose(false)
	if err := os.Chmod(server.config.SocketPath, 0o600); err != nil {
		_ = listener.Close()
		_ = releaseLockFile(server.config.SocketPath+".lock", lock)
		return ErrInvalidSocketPath
	}
	socketInfo, err := os.Lstat(server.config.SocketPath)
	socketIdentity, identityOK := identityOf(socketInfo)
	if err != nil || !identityOK || !isSocket(socketInfo) ||
		socketInfo.Mode().Perm() != 0o600 ||
		ownerUID(socketInfo) != server.config.EffectiveUID ||
		!serverLockStillOwned(lock, lockIdentity, server.config) {
		_ = listener.Close()
		_ = releaseLockFile(server.config.SocketPath+".lock", lock)
		return ErrInvalidSocketPath
	}

	server.mu.Lock()
	if server.closed {
		server.mu.Unlock()
		_ = listener.Close()
		_ = removeExactFile(server.config.SocketPath, socketIdentity)
		_ = releaseLockFile(server.config.SocketPath+".lock", lock)
		return nil
	}
	server.listener = listener
	serveCtx, serveCancel := context.WithCancel(ctx)
	server.serveCancel = serveCancel
	server.socketIdentity = socketIdentity
	server.lock = lock
	server.lockIdentity = lockIdentity
	server.mu.Unlock()
	server.readyOnce.Do(func() { close(server.ready) })

	defer serveCancel()
	cancelDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = server.Close()
		case <-cancelDone:
		}
	}()
	defer close(cancelDone)

	for {
		connection, acceptErr := listener.AcceptUnix()
		if acceptErr != nil {
			if server.isClosed() || ctx.Err() != nil ||
				errors.Is(acceptErr, net.ErrClosed) {
				server.wait.Wait()
				<-server.closeDone
				return server.closeResult()
			}
			_ = server.Close()
			server.wait.Wait()
			return acceptErr
		}
		select {
		case server.slots <- struct{}{}:
			if !server.startConnection(connection) {
				<-server.slots
				_ = connection.Close()
				continue
			}
			go server.serveConnection(serveCtx, connection)
		default:
			_ = server.writeError(
				connection,
				"",
				"busy",
				ErrInvalidProtocol,
			)
			_ = connection.Close()
		}
	}
}

func serverLockStillOwned(
	lock *os.File,
	identity fileIdentity,
	config ServerConfig,
) bool {
	info, err := lock.Stat()
	current, ok := identityOf(info)
	return err == nil && ok && current == identity &&
		validLockInfo(info, config.EffectiveUID) &&
		pathMatchesOwnedLock(
			config.SocketPath+".lock",
			identity,
			config.EffectiveUID,
		)
}

func (server *Server) serveConnection(
	ctx context.Context,
	connection *net.UnixConn,
) {
	defer func() {
		server.untrackConnection(connection)
		<-server.slots
		server.wait.Done()
		_ = connection.Close()
	}()
	_ = connection.SetDeadline(time.Now().Add(connectionDeadline))
	peerUID, err := peerEffectiveUID(connection)
	if err != nil {
		_ = server.writeError(
			connection,
			"",
			"unsupported_platform",
			err,
		)
		return
	}
	if peerUID != server.config.EffectiveUID {
		_ = server.writeError(
			connection,
			"",
			"unauthorized_peer",
			ErrInvalidProtocol,
		)
		return
	}
	body, err := readServerRequestFrame(connection)
	if err != nil {
		_ = server.writeError(
			connection,
			"",
			"invalid_request",
			err,
		)
		return
	}
	request, err := decodeRequest(body)
	if err != nil {
		_ = server.writeError(
			connection,
			requestIDFromUntrustedBody(body),
			requestErrorCode(err),
			err,
		)
		return
	}
	_ = connection.SetDeadline(
		time.Now().Add(responseDeadline(request.Method)),
	)
	response := server.handle(ctx, request)
	response.Version = protocolVersion
	response.RequestID = request.RequestID
	encoded, err := encodeResponse(response)
	if err != nil {
		_ = server.writeError(
			connection,
			request.RequestID,
			"internal",
			err,
		)
		return
	}
	_ = writeFrame(connection, encoded, maxResponseBodyBytes)
}

func (server *Server) handle(
	ctx context.Context,
	request Request,
) Response {
	if request.Method == "ping" {
		result, _ := json.Marshal(PingResult{
			ProtocolVersion: protocolVersion,
			Available:       true,
			BuildID:         server.config.BuildID,
		})
		return Response{OK: true, Result: result}
	}
	handlerCtx, cancel := context.WithTimeout(
		ctx,
		requestDeadline(request.Method),
	)
	defer cancel()
	response := server.config.Handler.Handle(handlerCtx, request)
	if !response.OK && response.Error == nil {
		response.Error = safeProtocolError("internal", ErrInvalidProtocol)
	}
	return response
}

func requestDeadline(method string) time.Duration {
	if method == "chat_message" {
		return chatRequestDeadline
	}
	if method == "agent_attempt_recovery" {
		return agentRecoveryDeadline
	}
	if method == "mission_execution" {
		return missionRequestDeadline
	}
	if usesExtendedRequestDeadline(method) {
		return extendedRequestDeadline
	}
	return connectionDeadline
}

func responseDeadline(method string) time.Duration {
	if method == "chat_message" {
		return chatResponseDeadline
	}
	if method == "agent_attempt_recovery" {
		return agentRecoveryResponse
	}
	if method == "mission_execution" {
		return missionResponseDeadline
	}
	if usesExtendedRequestDeadline(method) {
		return extendedResponseDeadline
	}
	return connectionDeadline
}

func usesExtendedRequestDeadline(method string) bool {
	switch method {
	case "credential_import", "credential_verify", "credential_vault_rotate", "credential_vault_lock",
		"credential_vault_unlock", "credential_vault_reset", "mission_execution", "chat_message",
		"agent_attempt_recovery", "tool_recovery", "setup_snapshot":
		return true
	case "credential_vault_export":
		return true
	default:
		return false
	}
}

func readServerRequestFrame(connection *net.UnixConn) ([]byte, error) {
	body, err := readFrame(connection, maxRequestBodyBytes)
	if err != nil {
		return nil, err
	}
	_ = connection.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
	var trailing [1]byte
	count, trailingErr := connection.Read(trailing[:])
	_ = connection.SetDeadline(time.Now().Add(connectionDeadline))
	if count != 0 || !errors.Is(trailingErr, io.EOF) {
		return nil, ErrInvalidProtocol
	}
	return body, nil
}

func (server *Server) writeError(
	connection *net.UnixConn,
	requestID,
	code string,
	cause error,
) error {
	if !validRequestID(requestID) {
		requestID = "invalid-request"
	}
	encoded, err := encodeResponse(Response{
		Version:   protocolVersion,
		RequestID: requestID,
		OK:        false,
		Error:     safeProtocolError(code, cause),
	})
	if err != nil {
		return err
	}
	return writeFrame(connection, encoded, maxResponseBodyBytes)
}

func requestIDFromUntrustedBody(body []byte) string {
	var envelope struct {
		RequestID string `json:"request_id"`
	}
	if json.Unmarshal(body, &envelope) == nil &&
		validRequestID(envelope.RequestID) {
		return envelope.RequestID
	}
	return ""
}

func (server *Server) Close() error {
	var closeErr error
	server.closeOnce.Do(func() {
		defer close(server.closeDone)
		server.mu.Lock()
		server.closed = true
		listener := server.listener
		serveCancel := server.serveCancel
		connections := make([]*net.UnixConn, 0, len(server.connections))
		for connection := range server.connections {
			connections = append(connections, connection)
		}
		lock := server.lock
		socketIdentity := server.socketIdentity
		lockIdentity := server.lockIdentity
		server.mu.Unlock()

		if serveCancel != nil {
			serveCancel()
		}
		if listener != nil {
			closeErr = errors.Join(
				closeErr,
				closeIgnoringNetworkClosed(listener),
			)
		}
		for _, connection := range connections {
			closeErr = errors.Join(
				closeErr,
				closeIgnoringNetworkClosed(connection),
			)
		}
		server.wait.Wait()
		if socketIdentity != (fileIdentity{}) {
			closeErr = errors.Join(
				closeErr,
				removeExactFile(server.config.SocketPath, socketIdentity),
			)
		}
		if lock != nil && lockIdentity != (fileIdentity{}) {
			closeErr = errors.Join(
				closeErr,
				releaseLockFile(server.config.SocketPath+".lock", lock),
			)
		} else if lock != nil {
			closeErr = errors.Join(closeErr, lock.Close())
		}
		server.mu.Lock()
		server.closeErr = closeErr
		server.mu.Unlock()
	})
	return server.closeResult()
}

func (server *Server) startConnection(connection *net.UnixConn) bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.closed {
		return false
	}
	server.wait.Add(1)
	server.connections[connection] = struct{}{}
	return true
}

func (server *Server) untrackConnection(connection *net.UnixConn) {
	server.mu.Lock()
	defer server.mu.Unlock()
	delete(server.connections, connection)
}

func (server *Server) isClosed() bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.closed
}

func (server *Server) closeResult() error {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.closeErr
}

func closeIgnoringNetworkClosed(closer io.Closer) error {
	err := closer.Close()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}
