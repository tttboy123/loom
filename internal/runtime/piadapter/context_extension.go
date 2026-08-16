package piadapter

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
)

var ErrPiContextExtension = errors.New("Pi scoped Context extension unavailable")

const (
	piContextExtensionMaxRequestBytes = 4096
	piContextExtensionMaxContentBytes = 24 << 10
	piContextExtensionMaxResultBytes  = 32 << 10
	piContextExtensionSocketLimit     = 100
)

type piContextExtension struct {
	root          string
	extensionPath string
	socketRoot    string
	socketPath    string
	capability    string
	retriever     contextcapsule.Retriever
	delivery      contextcapsule.DeliveryBroker
	deliveryMu    sync.Mutex
	prepared      attemptpayload.Binding
	acknowledged  bool
	listener      *net.UnixListener
	expectedPID   chan int
	shutdown      chan struct{}
	done          chan struct{}
	connectionMu  sync.Mutex
	connection    *net.UnixConn
	closeOnce     sync.Once
}

type piContextExtensionRequest struct {
	SchemaVersion int    `json:"schema_version"`
	Capability    string `json:"capability"`
	ItemID        string `json:"item_id"`
	ContentDigest string `json:"content_digest"`
	ArtifactRef   string `json:"artifact_ref,omitempty"`
}

func newPiContextExtension(
	homePath string,
	temporaryPath string,
	id string,
	retriever contextcapsule.Retriever,
) (*piContextExtension, error) {
	return newPiContextExtensionWithContext(
		context.Background(), homePath, temporaryPath, id, retriever,
	)
}

func newPiContextExtensionWithContext(
	ctx context.Context,
	homePath string,
	temporaryPath string,
	id string,
	retriever contextcapsule.Retriever,
) (*piContextExtension, error) {
	if nilPiInterface(retriever) {
		return nil, ErrPiContextExtension
	}
	return newPiContextExtensionService(
		ctx, homePath, temporaryPath, id, retriever, nil,
	)
}

func newPiContextDeliveryExtension(
	homePath string,
	temporaryPath string,
	id string,
	delivery contextcapsule.DeliveryBroker,
) (*piContextExtension, error) {
	return newPiContextDeliveryExtensionWithContext(
		context.Background(), homePath, temporaryPath, id, delivery,
	)
}

func newPiContextDeliveryExtensionWithContext(
	ctx context.Context,
	homePath string,
	temporaryPath string,
	id string,
	delivery contextcapsule.DeliveryBroker,
) (*piContextExtension, error) {
	if nilPiInterface(delivery) {
		return nil, ErrPiContextExtension
	}
	return newPiContextExtensionService(
		ctx, homePath, temporaryPath, id, nil, delivery,
	)
}

func newPiContextExtensionService(
	ctx context.Context,
	homePath string,
	temporaryPath string,
	id string,
	retriever contextcapsule.Retriever,
	delivery contextcapsule.DeliveryBroker,
) (*piContextExtension, error) {
	if ctx == nil || ctx.Err() != nil || homePath == "" || temporaryPath == "" ||
		!validPiContextExtensionID(id) || (retriever == nil) == (delivery == nil) {
		return nil, ErrPiContextExtension
	}
	root := filepath.Join(homePath, ".loom-context-"+id)
	if !filepath.IsAbs(root) || filepath.Clean(root) != root ||
		!filepath.IsAbs(temporaryPath) || filepath.Clean(temporaryPath) != temporaryPath {
		return nil, ErrPiContextExtension
	}
	if err := ensurePiRPCPrivateDirectory(homePath); err != nil {
		return nil, ErrPiContextExtension
	}
	if err := ensurePiRPCPrivateDirectory(temporaryPath); err != nil {
		return nil, ErrPiContextExtension
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		return nil, ErrPiContextExtension
	}
	extensionPath := filepath.Join(root, "loom-context.mjs")
	socketRoot := ""
	socketPath := filepath.Join(temporaryPath, ".loom-context-"+id+".sock")
	if len(socketPath) > piContextExtensionSocketLimit {
		socketRoot = filepath.Join("/tmp", ".loom-context-"+id)
		if !filepath.IsAbs(socketRoot) || filepath.Clean(socketRoot) != socketRoot ||
			os.Mkdir(socketRoot, 0o700) != nil {
			return nil, ErrPiContextExtension
		}
		info, err := os.Lstat(socketRoot)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
			info.Mode().Perm() != 0o700 || !piLocalCurrentUserOwns(info) {
			_ = os.Remove(socketRoot)
			return nil, ErrPiContextExtension
		}
		socketPath = filepath.Join(socketRoot, "context.sock")
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(extensionPath)
			_ = os.Remove(socketPath)
			if socketRoot != "" {
				_ = os.Remove(socketRoot)
			}
			_ = os.Remove(root)
		}
	}()
	if info, err := os.Lstat(root); err != nil || !info.IsDir() ||
		info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 ||
		!piLocalCurrentUserOwns(info) {
		return nil, ErrPiContextExtension
	}
	if len(socketPath) > piContextExtensionSocketLimit {
		return nil, ErrPiContextExtension
	}
	address := &net.UnixAddr{Name: socketPath, Net: "unix"}
	listener, err := net.ListenUnix("unix", address)
	if err != nil {
		return nil, ErrPiContextExtension
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(socketPath)
		return nil, ErrPiContextExtension
	}
	if info, err := os.Lstat(socketPath); err != nil || info.Mode()&os.ModeSocket == 0 ||
		info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 ||
		!piLocalCurrentUserOwns(info) {
		_ = listener.Close()
		_ = os.Remove(socketPath)
		return nil, ErrPiContextExtension
	}
	capability := strings.ReplaceAll(id, "-", "")
	if len(capability) != 32 {
		_ = listener.Close()
		_ = os.Remove(socketPath)
		return nil, ErrPiContextExtension
	}
	source := piContextExtensionSource(socketPath, capability)
	file, err := os.OpenFile(extensionPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_ = listener.Close()
		_ = os.Remove(socketPath)
		return nil, ErrPiContextExtension
	}
	writeErr := error(nil)
	if written, err := file.Write(source); err != nil || written != len(source) {
		writeErr = errors.Join(writeErr, err, io.ErrShortWrite)
	}
	if err := file.Sync(); err != nil {
		writeErr = errors.Join(writeErr, err)
	}
	if err := file.Close(); err != nil {
		writeErr = errors.Join(writeErr, err)
	}
	for index := range source {
		source[index] = 0
	}
	if writeErr != nil {
		_ = listener.Close()
		_ = os.Remove(socketPath)
		return nil, ErrPiContextExtension
	}
	info, err := os.Lstat(extensionPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 || !piLocalCurrentUserOwns(info) {
		_ = listener.Close()
		_ = os.Remove(socketPath)
		return nil, ErrPiContextExtension
	}
	extension := &piContextExtension{
		root: root, extensionPath: extensionPath, socketRoot: socketRoot,
		socketPath: socketPath,
		capability: capability, retriever: retriever, delivery: delivery, listener: listener,
		expectedPID: make(chan int, 1), shutdown: make(chan struct{}),
		done: make(chan struct{}),
	}
	go extension.serve()
	go func() {
		select {
		case <-ctx.Done():
			extension.Close()
		case <-extension.shutdown:
		}
	}()
	cleanup = false
	return extension, nil
}

func validPiContextExtensionID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' ||
		value[18] != '-' || value[23] != '-' {
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
	return value[14] == '4' && (value[19] == '8' || value[19] == '9' ||
		value[19] == 'a' || value[19] == 'b')
}

func (extension *piContextExtension) BindProcess(processID int) error {
	if extension == nil || processID <= 0 {
		return ErrPiContextExtension
	}
	select {
	case extension.expectedPID <- processID:
		return nil
	default:
		return ErrPiContextExtension
	}
}

func (extension *piContextExtension) Close() {
	if extension == nil {
		return
	}
	extension.closeOnce.Do(func() {
		close(extension.shutdown)
		_ = extension.listener.Close()
		extension.connectionMu.Lock()
		if extension.connection != nil {
			_ = extension.connection.Close()
		}
		extension.connectionMu.Unlock()
		<-extension.done
		_ = os.Remove(extension.socketPath)
		if extension.socketRoot != "" {
			_ = os.Remove(extension.socketRoot)
		}
		_ = os.Remove(extension.extensionPath)
		_ = os.Remove(extension.root)
	})
}

func (extension *piContextExtension) Acknowledge(
	ctx context.Context,
	proof attemptpayload.DeliveryProof,
) error {
	if extension == nil || ctx == nil || ctx.Err() != nil ||
		extension.delivery == nil || proof != attemptpayload.ProofHarnessFinalOutput {
		return ErrPiContextExtension
	}
	extension.deliveryMu.Lock()
	binding := extension.prepared
	alreadyAcknowledged := extension.acknowledged
	extension.deliveryMu.Unlock()
	if binding == (attemptpayload.Binding{}) || alreadyAcknowledged {
		return nil
	}
	if err := extension.delivery.Acknowledge(ctx, binding, proof); err != nil {
		return errors.Join(ErrPiContextExtension, err)
	}
	extension.deliveryMu.Lock()
	if extension.prepared != binding {
		extension.deliveryMu.Unlock()
		return ErrPiContextExtension
	}
	extension.acknowledged = true
	extension.deliveryMu.Unlock()
	return nil
}

func (extension *piContextExtension) serve() {
	defer close(extension.done)
	connection, err := extension.listener.AcceptUnix()
	if err != nil {
		return
	}
	extension.connectionMu.Lock()
	extension.connection = connection
	extension.connectionMu.Unlock()
	defer func() {
		extension.connectionMu.Lock()
		if extension.connection == connection {
			extension.connection = nil
		}
		extension.connectionMu.Unlock()
	}()
	defer connection.Close()
	var expectedPID int
	select {
	case expectedPID = <-extension.expectedPID:
	case <-extension.shutdown:
		return
	case <-time.After(10 * time.Second):
		return
	}
	peerPID, err := piContextPeerPID(connection)
	if err != nil || peerPID != expectedPID {
		return
	}
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(io.LimitReader(
		connection,
		piContextExtensionMaxRequestBytes+1,
	))
	line, err := reader.ReadBytes('\n')
	if err != nil || len(line) == 0 || len(line) > piContextExtensionMaxRequestBytes ||
		reader.Buffered() != 0 {
		return
	}
	line = bytes.TrimSuffix(line, []byte{'\n'})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-extension.shutdown:
			cancel()
		case <-ctx.Done():
		}
	}()
	response := extension.handle(ctx, line)
	for index := range line {
		line[index] = 0
	}
	if len(response) != 0 {
		response = append(response, '\n')
		_, _ = connection.Write(response)
		for index := range response {
			response[index] = 0
		}
	}
}

func (extension *piContextExtension) handle(ctx context.Context, payload []byte) []byte {
	denied := func() []byte {
		body, _ := json.Marshal(struct {
			SchemaVersion int    `json:"schema_version"`
			Status        string `json:"status"`
			Reason        string `json:"reason"`
		}{1, "denied", "context_retrieval_denied"})
		return body
	}
	if len(payload) == 0 || len(payload) > piContextExtensionMaxRequestBytes ||
		rejectPiRPCDuplicateKeys(payload) != nil {
		return denied()
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var request piContextExtensionRequest
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		request.SchemaVersion != 1 || request.Capability != extension.capability {
		return denied()
	}
	proposal := contextcapsule.RetrievalProposal{
		ItemID: request.ItemID, ContentDigest: request.ContentDigest,
		ArtifactRef: request.ArtifactRef,
	}
	if extension.delivery != nil {
		payload, err := extension.delivery.Prepare(
			ctx, proposal,
			contextcapsule.DeliveryRequest{Sequence: 1, ContentType: "application/json"},
			marshalPiContextToolResult,
		)
		if err != nil {
			payload.Close()
			return denied()
		}
		defer payload.Close()
		if _, err := piRPCContextInnerResult(payload.Content, proposal); err != nil ||
			len(payload.Content) > piContextExtensionMaxResultBytes {
			return denied()
		}
		extension.deliveryMu.Lock()
		if extension.prepared != (attemptpayload.Binding{}) &&
			extension.prepared != payload.Binding {
			extension.deliveryMu.Unlock()
			return denied()
		}
		extension.prepared = payload.Binding
		extension.deliveryMu.Unlock()
		return append([]byte(nil), payload.Content...)
	}
	item, err := extension.retriever.Retrieve(ctx, proposal)
	if err != nil {
		if item.Content != nil {
			item.Close()
		}
		return denied()
	}
	defer item.Close()
	if !validPiContextRetrievedItem(request, item) {
		return denied()
	}
	body, err := marshalPiContextToolResult(proposal, item)
	if err != nil {
		return denied()
	}
	return body
}

func marshalPiContextToolResult(
	proposal contextcapsule.RetrievalProposal,
	item contextcapsule.RetrievedItem,
) ([]byte, error) {
	request := piContextExtensionRequest{
		SchemaVersion: 1, ItemID: proposal.ItemID,
		ContentDigest: proposal.ContentDigest, ArtifactRef: proposal.ArtifactRef,
	}
	if !validPiContextRetrievedItem(request, item) {
		return nil, ErrPiContextExtension
	}
	body, err := json.Marshal(struct {
		SchemaVersion int                       `json:"schema_version"`
		Status        string                    `json:"status"`
		ItemID        string                    `json:"item_id"`
		ContentDigest string                    `json:"content_digest"`
		Trust         contextcapsule.TrustClass `json:"trust"`
		Scope         contextcapsule.Scope      `json:"scope"`
		SourceType    contextcapsule.SourceType `json:"source_type"`
		SourceRef     string                    `json:"source_ref"`
		ArtifactRef   string                    `json:"artifact_ref,omitempty"`
		Content       string                    `json:"content"`
	}{
		1, "succeeded", item.ItemID, item.ContentDigest, item.Trust, item.Scope,
		item.SourceType, item.SourceRef, item.ArtifactRef, string(item.Content),
	})
	if err != nil || len(body) > piContextExtensionMaxResultBytes {
		for index := range body {
			body[index] = 0
		}
		return nil, ErrPiContextExtension
	}
	return body, nil
}

func validPiContextRetrievedItem(
	request piContextExtensionRequest,
	item contextcapsule.RetrievedItem,
) bool {
	if item.ItemID != request.ItemID || item.ContentDigest != request.ContentDigest ||
		item.ArtifactRef != request.ArtifactRef ||
		item.Kind == contextcapsule.KindCredentialReference ||
		item.Scope == contextcapsule.ScopeSecretReferenceOnly ||
		item.SourceType == contextcapsule.SourceCredentialReference ||
		len(item.Content) == 0 || len(item.Content) > piContextExtensionMaxContentBytes ||
		!utf8.Valid(item.Content) || piContextContentHasControls(item.Content) ||
		piContextDigest(item.Content) != item.ContentDigest ||
		!validPiContextIdentifier(item.SourceRef, 512) ||
		(item.Scope == contextcapsule.ScopeArtifactScoped) != (item.ArtifactRef != "") {
		return false
	}
	switch item.SourceType {
	case contextcapsule.SourceAuthority:
		return item.Trust == contextcapsule.TrustAuthoritative
	case contextcapsule.SourceObservation:
		return item.Trust == contextcapsule.TrustObserved
	case contextcapsule.SourceModelOutput:
		return item.Trust == contextcapsule.TrustUntrusted &&
			item.Kind == contextcapsule.KindPriorModelOutput
	default:
		return false
	}
}

func piContextDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func piContextContentHasControls(content []byte) bool {
	for _, value := range string(content) {
		if unicode.IsControl(value) && value != '\n' && value != '\r' && value != '\t' {
			return true
		}
	}
	return false
}

func validPiContextIdentifier(value string, limit int) bool {
	return value != "" && len(value) <= limit && utf8.ValidString(value) &&
		strings.TrimSpace(value) == value && !piContextContentHasControls([]byte(value))
}

func piContextExtensionSource(socketPath, capability string) []byte {
	socketLiteral := strconv.Quote(socketPath)
	capabilityLiteral := strconv.Quote(capability)
	return []byte(fmt.Sprintf(`import net from "node:net";
const socketPath = %s;
const capability = %s;
function requestContext(params, signal) {
  return new Promise((resolve, reject) => {
    const socket = net.createConnection({ path: socketPath });
    let settled = false;
    let received = "";
    const finish = (error, value) => {
      if (settled) return;
      settled = true;
      socket.destroy();
      error ? reject(error) : resolve(value);
    };
    const timer = setTimeout(() => finish(new Error("context_retrieval_timeout")), 10000);
    const done = (error, value) => { clearTimeout(timer); finish(error, value); };
    if (signal) signal.addEventListener("abort", () => done(new Error("context_retrieval_cancelled")), { once: true });
    socket.setEncoding("utf8");
    socket.on("connect", () => socket.write(JSON.stringify({
      schema_version: 1,
      capability,
      item_id: params.item_id,
      content_digest: params.content_digest,
      ...(params.artifact_ref ? { artifact_ref: params.artifact_ref } : {})
    }) + "\n"));
    socket.on("data", chunk => {
      received += chunk;
      if (received.length > 32768) return done(new Error("context_retrieval_denied"));
      const newline = received.indexOf("\n");
      if (newline < 0) return;
      if (newline !== received.length - 1) return done(new Error("context_retrieval_denied"));
      try {
        const result = JSON.parse(received.slice(0, newline));
        const keys = Object.keys(result);
        if (result.schema_version !== 1 || result.status !== "succeeded" ||
            typeof result.item_id !== "string" || typeof result.content_digest !== "string" ||
            typeof result.trust !== "string" || typeof result.scope !== "string" ||
            typeof result.source_type !== "string" || typeof result.source_ref !== "string" ||
            typeof result.content !== "string" || keys.some(key => ![
              "schema_version", "status", "item_id", "content_digest", "trust", "scope",
              "source_type", "source_ref", "artifact_ref", "content"
            ].includes(key))) return done(new Error("context_retrieval_denied"));
        done(undefined, result);
      } catch (_) { done(new Error("context_retrieval_denied")); }
    });
    socket.on("error", () => done(new Error("context_retrieval_denied")));
    socket.on("end", () => { if (!settled) done(new Error("context_retrieval_denied")); });
  });
}
export default function(pi) {
  pi.registerTool({
    name: "loom_read_context",
    label: "Read context",
    description: "Read one exact omitted Context Capsule item by its Loom-provided item and content digest.",
    parameters: {
      type: "object",
      properties: {
        item_id: { type: "string", minLength: 1, maxLength: 512 },
        content_digest: { type: "string", pattern: "^[0-9a-f]{64}$" },
        artifact_ref: { type: "string", minLength: 1, maxLength: 512 }
      },
      required: ["item_id", "content_digest"],
      additionalProperties: false
    },
    executionMode: "sequential",
    async execute(_toolCallId, params, signal) {
      const result = await requestContext(params, signal);
      return {
        content: [{ type: "text", text: JSON.stringify(result) }],
        details: { item_id: result.item_id, content_digest: result.content_digest }
      };
    }
  });
}
`, socketLiteral, capabilityLiteral))
}
