//go:build unix

package piadapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/permissions"
)

var ErrPiToolExtension = errors.New("Pi governed Tool extension unavailable")

const (
	piToolExtensionMaxRequestBytes = toolCallMaxEnvelopeBytes + 1024
	piToolExtensionMaxResultBytes  = 32 << 10
	piToolExtensionMaxContentBytes = 12 << 10
	piToolExtensionSocketLimit     = 100
	piToolApprovalPollInterval     = 100 * time.Millisecond
)

type piToolExtension struct {
	root          string
	extensionPath string
	socketRoot    string
	socketPath    string
	capability    string
	hook          ToolCallHook
	allowedTools  map[permissions.ToolKind]struct{}
	binding       ToolCallBinding
	parent        context.Context
	listener      *net.UnixListener
	expectedPID   chan int
	shutdown      chan struct{}
	done          chan struct{}
	connectionMu  sync.Mutex
	connection    *net.UnixConn
	resultMu      sync.Mutex
	envelope      ToolCallEnvelope
	result        ToolCallResult
	resolved      bool
	acknowledged  bool
	resolvedCalls []piToolResolvedCall
	closeOnce     sync.Once
}

type piToolResolvedCall struct {
	Sequence     int64
	Envelope     ToolCallEnvelope
	Result       ToolCallResult
	Acknowledged bool
}

type piToolExtensionRequest struct {
	SchemaVersion int              `json:"schema_version"`
	Capability    string           `json:"capability"`
	Envelope      ToolCallEnvelope `json:"envelope"`
}

func newPiToolExtension(
	ctx context.Context,
	homePath string,
	temporaryPath string,
	id string,
	hook ToolCallHook,
	binding ToolCallBinding,
) (*piToolExtension, error) {
	if ctx == nil || nilPiInterface(hook) || !validPiToolBinding(binding) ||
		homePath == "" || temporaryPath == "" || !validPiContextExtensionID(id) {
		return nil, ErrPiToolExtension
	}
	if err := ctx.Err(); err != nil {
		return nil, ErrPiToolExtension
	}
	allowedTools := allowedPiToolCalls(hook)
	if len(allowedTools) == 0 {
		return nil, ErrPiToolExtension
	}
	root := filepath.Join(homePath, ".loom-tool-"+id)
	if !filepath.IsAbs(root) || filepath.Clean(root) != root ||
		!filepath.IsAbs(temporaryPath) || filepath.Clean(temporaryPath) != temporaryPath ||
		ensurePiRPCPrivateDirectory(homePath) != nil ||
		ensurePiRPCPrivateDirectory(temporaryPath) != nil ||
		os.Mkdir(root, 0o700) != nil {
		return nil, ErrPiToolExtension
	}
	extensionPath := filepath.Join(root, "loom-tool.mjs")
	socketRoot := ""
	socketPath := filepath.Join(temporaryPath, ".loom-tool-"+id+".sock")
	if len(socketPath) > piToolExtensionSocketLimit {
		socketRoot = filepath.Join("/tmp", ".loom-tool-"+id)
		if !filepath.IsAbs(socketRoot) || filepath.Clean(socketRoot) != socketRoot ||
			os.Mkdir(socketRoot, 0o700) != nil {
			_ = os.Remove(root)
			return nil, ErrPiToolExtension
		}
		info, err := os.Lstat(socketRoot)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
			info.Mode().Perm() != 0o700 || !piLocalCurrentUserOwns(info) {
			_ = os.Remove(socketRoot)
			_ = os.Remove(root)
			return nil, ErrPiToolExtension
		}
		socketPath = filepath.Join(socketRoot, "tool.sock")
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
		!piLocalCurrentUserOwns(info) || len(socketPath) > piToolExtensionSocketLimit {
		return nil, ErrPiToolExtension
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return nil, ErrPiToolExtension
	}
	if err := os.Chmod(socketPath, 0o600); err != nil || !validPiToolSocket(socketPath) {
		_ = listener.Close()
		return nil, ErrPiToolExtension
	}
	capability := strings.ReplaceAll(id, "-", "")
	if len(capability) != 32 {
		_ = listener.Close()
		return nil, ErrPiToolExtension
	}
	source := piToolExtensionSource(socketPath, capability, allowedTools)
	file, err := os.OpenFile(extensionPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_ = listener.Close()
		return nil, ErrPiToolExtension
	}
	writeErr := error(nil)
	if written, err := file.Write(source); err != nil || written != len(source) {
		writeErr = errors.Join(writeErr, err, io.ErrShortWrite)
	}
	writeErr = errors.Join(writeErr, file.Sync(), file.Close())
	zeroPiRPCBytes(source)
	if writeErr != nil || !validPiToolFile(extensionPath) {
		_ = listener.Close()
		return nil, ErrPiToolExtension
	}
	extension := &piToolExtension{
		root: root, extensionPath: extensionPath, socketRoot: socketRoot,
		socketPath: socketPath, capability: capability, hook: hook,
		allowedTools: allowedTools, binding: binding, parent: ctx, listener: listener,
		expectedPID: make(chan int, 1), shutdown: make(chan struct{}),
		done: make(chan struct{}),
	}
	go extension.serve()
	cleanup = false
	return extension, nil
}

func validPiToolBinding(binding ToolCallBinding) bool {
	return binding.ConversationID != "" && binding.WorkItemID != "" &&
		binding.RunID != "" && binding.ClaimGeneration > 0 &&
		binding.RuntimeInstanceID != "" && binding.AgentInstanceID != "" &&
		binding.ExecutionBindingDigest != "" && binding.CapsuleDigest != "" &&
		binding.ClaimID != "" && binding.IncidentID != "" &&
		binding.JourneyID == binding.IncidentID
}

func validPiToolSocket(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSocket != 0 &&
		info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm() == 0o600 &&
		piLocalCurrentUserOwns(info)
}

func validPiToolFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 &&
		info.Mode().Perm() == 0o600 && piLocalCurrentUserOwns(info)
}

func (extension *piToolExtension) BindProcess(processID int) error {
	if extension == nil || processID <= 0 {
		return ErrPiToolExtension
	}
	select {
	case extension.expectedPID <- processID:
		return nil
	default:
		return ErrPiToolExtension
	}
}

func (extension *piToolExtension) Close() {
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

func (extension *piToolExtension) Acknowledge(
	ctx context.Context,
	proof attemptpayload.DeliveryProof,
) error {
	if extension == nil || ctx == nil || ctx.Err() != nil ||
		proof != attemptpayload.ProofHarnessFinalOutput {
		return ErrPiToolExtension
	}
	extension.resultMu.Lock()
	calls := append([]piToolResolvedCall(nil), extension.resolvedCalls...)
	if len(calls) == 0 && extension.resolved {
		calls = []piToolResolvedCall{{
			Sequence: 1, Envelope: extension.envelope, Result: extension.result,
			Acknowledged: extension.acknowledged,
		}}
	}
	extension.resultMu.Unlock()
	for _, call := range calls {
		if call.Acknowledged {
			continue
		}
		if call.Result.Verdict == permissions.VerdictAllow {
			acknowledger, ok := extension.hook.(ToolCallResultProofAcknowledger)
			if !ok || call.Result.Delivery == nil {
				return ErrPiToolExtension
			}
			if err := acknowledger.AcknowledgeToolCallResultWithProof(
				ctx, extension.binding, call.Result, proof,
			); err != nil {
				return errors.Join(ErrPiToolExtension, err)
			}
		} else if call.Result.Verdict != permissions.VerdictDeny || call.Result.Delivery != nil {
			return ErrPiToolExtension
		}
		extension.resultMu.Lock()
		if !extension.acknowledgeResolvedCall(call) {
			extension.resultMu.Unlock()
			return ErrPiToolExtension
		}
		extension.resultMu.Unlock()
	}
	return nil
}

func (extension *piToolExtension) acknowledgeResolvedCall(call piToolResolvedCall) bool {
	if len(extension.resolvedCalls) == 0 {
		if extension.envelope != call.Envelope ||
			!samePiToolResult(extension.result, call.Result) {
			return false
		}
		extension.acknowledged = true
		return true
	}
	for index := range extension.resolvedCalls {
		candidate := &extension.resolvedCalls[index]
		if candidate.Sequence == call.Sequence && candidate.Envelope == call.Envelope &&
			samePiToolResult(candidate.Result, call.Result) {
			candidate.Acknowledged = true
			extension.acknowledged = extension.allResolvedCallsAcknowledged()
			return true
		}
	}
	return false
}

func (extension *piToolExtension) allResolvedCallsAcknowledged() bool {
	if len(extension.resolvedCalls) == 0 {
		return extension.acknowledged
	}
	for _, call := range extension.resolvedCalls {
		if !call.Acknowledged {
			return false
		}
	}
	return true
}

func (extension *piToolExtension) serve() {
	defer close(extension.done)
	var expectedPID int
	select {
	case expectedPID = <-extension.expectedPID:
	case <-extension.shutdown:
		return
	case <-extension.parent.Done():
		return
	case <-time.After(10 * time.Second):
		return
	}
	for sequence := int64(1); sequence <= piMaxSequentialToolCalls+1; sequence++ {
		connection, err := extension.listener.AcceptUnix()
		if err != nil {
			return
		}
		extension.connectionMu.Lock()
		extension.connection = connection
		extension.connectionMu.Unlock()
		extension.serveConnection(connection, expectedPID, sequence)
		extension.connectionMu.Lock()
		if extension.connection == connection {
			extension.connection = nil
		}
		extension.connectionMu.Unlock()
		_ = connection.Close()
	}
}

func (extension *piToolExtension) serveConnection(
	connection *net.UnixConn,
	expectedPID int,
	sequence int64,
) {
	peerPID, err := piContextPeerPID(connection)
	if err != nil || peerPID != expectedPID {
		return
	}
	_ = connection.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(io.LimitReader(connection, piToolExtensionMaxRequestBytes+1))
	line, err := reader.ReadBytes('\n')
	if err != nil || len(line) == 0 || len(line) > piToolExtensionMaxRequestBytes ||
		reader.Buffered() != 0 {
		zeroPiRPCBytes(line)
		return
	}
	_ = connection.SetReadDeadline(time.Time{})
	line = bytes.TrimSuffix(line, []byte{'\n'})
	ctx, cancel := context.WithCancel(extension.parent)
	defer cancel()
	go func() {
		select {
		case <-extension.shutdown:
			cancel()
		case <-ctx.Done():
		}
	}()
	response := extension.handleCall(ctx, line, sequence)
	zeroPiRPCBytes(line)
	if len(response) != 0 {
		response = append(response, '\n')
		_, _ = connection.Write(response)
		zeroPiRPCBytes(response)
	}
}

func (extension *piToolExtension) handle(ctx context.Context, payload []byte) []byte {
	return extension.handleCall(ctx, payload, 1)
}

func (extension *piToolExtension) handleCall(
	ctx context.Context,
	payload []byte,
	sequence int64,
) []byte {
	denied := func() []byte {
		body, _ := json.Marshal(struct {
			SchemaVersion int    `json:"schema_version"`
			Status        string `json:"status"`
			Reason        string `json:"reason"`
		}{1, "denied", "tool_unavailable"})
		return body
	}
	if extension == nil || ctx == nil || extension.hook == nil ||
		sequence < 1 || sequence > piMaxSequentialToolCalls ||
		len(payload) == 0 || len(payload) > piToolExtensionMaxRequestBytes ||
		rejectPiRPCDuplicateKeys(payload) != nil {
		return denied()
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var request piToolExtensionRequest
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		request.SchemaVersion != 1 || request.Capability != extension.capability {
		return denied()
	}
	encodedEnvelope, err := json.Marshal(request.Envelope)
	if err != nil {
		return denied()
	}
	canonical, err := decodeToolCallEnvelope(encodedEnvelope, extension.governedTools())
	zeroPiRPCBytes(encodedEnvelope)
	if err != nil || canonical != request.Envelope || canonical.JobID != extension.binding.WorkItemID {
		return denied()
	}
	callContext, err := BindToolCallSequence(ctx, sequence)
	if err != nil {
		return denied()
	}
	for {
		result, err := extension.hook.ExecuteToolCall(callContext, canonical, extension.binding)
		if err != nil {
			return denied()
		}
		if result.Verdict == permissions.VerdictAsk {
			select {
			case <-ctx.Done():
				return denied()
			case <-time.After(piToolApprovalPollInterval):
				continue
			}
		}
		if !validResolvedPiToolResult(result) {
			return denied()
		}
		var content []byte
		if result.Verdict == permissions.VerdictAllow &&
			piToolContentKind(canonical.Call.Tool) {
			reader, ok := extension.hook.(ToolCallResultContentReader)
			if !ok {
				return denied()
			}
			content, err = reader.ReadToolCallResultContent(
				ctx, extension.binding, result,
			)
			if err != nil || len(content) == 0 ||
				len(content) > piToolExtensionMaxContentBytes ||
				!utf8.Valid(content) || piContextContentHasControls(content) ||
				result.OutputDigest != "sha256:"+piContextDigest(content) {
				zeroPiRPCBytes(content)
				return denied()
			}
			result.ContentDigest = result.OutputDigest
		}
		body, err := marshalPiToolResultPayload(
			canonical, piToolWireResult(result), content,
		)
		zeroPiRPCBytes(content)
		if err != nil || len(body) == 0 || len(body) > piToolExtensionMaxResultBytes {
			zeroPiRPCBytes(body)
			return denied()
		}
		extension.resultMu.Lock()
		if !extension.commitResolvedCall(sequence, canonical, result) {
			extension.resultMu.Unlock()
			zeroPiRPCBytes(body)
			return denied()
		}
		extension.resultMu.Unlock()
		return body
	}
}

func (extension *piToolExtension) commitResolvedCall(
	sequence int64,
	envelope ToolCallEnvelope,
	result ToolCallResult,
) bool {
	for _, call := range extension.resolvedCalls {
		if call.Sequence == sequence {
			return call.Envelope == envelope && samePiToolResult(call.Result, result)
		}
	}
	if sequence != int64(len(extension.resolvedCalls)+1) ||
		len(extension.resolvedCalls) >= piMaxSequentialToolCalls {
		return false
	}
	extension.resolvedCalls = append(extension.resolvedCalls, piToolResolvedCall{
		Sequence: sequence, Envelope: envelope, Result: result,
	})
	extension.envelope = envelope
	extension.result = result
	extension.resolved = true
	return true
}

func (extension *piToolExtension) governedTools() map[permissions.ToolKind]struct{} {
	if extension != nil && len(extension.allowedTools) > 0 {
		return extension.allowedTools
	}
	return bridgeAllowedTools
}

func validResolvedPiToolResult(result ToolCallResult) bool {
	if result.Verdict == permissions.VerdictAllow {
		return result.ExecutionID != "" && result.Delivery != nil
	}
	return result.Verdict == permissions.VerdictDeny && result.ExecutionID == "" &&
		result.Delivery == nil
}

func piToolWireResult(result ToolCallResult) ToolCallResult {
	result.ApprovalID = ""
	result.ApprovalDigest = ""
	result.Delivery = nil
	return result
}

func marshalPiToolResultPayload(
	envelope ToolCallEnvelope,
	result ToolCallResult,
	content []byte,
) ([]byte, error) {
	if len(content) == 0 {
		return marshalToolCallResultPayload(envelope, result)
	}
	return json.Marshal(struct {
		CallDigest string         `json:"call_digest"`
		Tool       string         `json:"tool"`
		Result     ToolCallResult `json:"result"`
		Content    string         `json:"content"`
	}{
		permissions.ProposedCallDigest(envelope.Call), string(envelope.Call.Tool),
		result, string(content),
	})
}

func samePiToolResult(left, right ToolCallResult) bool {
	leftDelivery := left.Delivery
	rightDelivery := right.Delivery
	left.Delivery = nil
	right.Delivery = nil
	if left != right || (leftDelivery == nil) != (rightDelivery == nil) {
		return false
	}
	return leftDelivery == nil || *leftDelivery == *rightDelivery
}

func decodePiToolExtensionResponse(
	payload []byte,
) (ToolCallEnvelope, ToolCallResult, error) {
	if len(payload) == 0 || len(payload) > piToolExtensionMaxResultBytes ||
		rejectPiRPCDuplicateKeys(payload) != nil {
		return ToolCallEnvelope{}, ToolCallResult{}, ErrPiToolExtension
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var body struct {
		CallDigest string         `json:"call_digest"`
		Tool       string         `json:"tool"`
		Result     ToolCallResult `json:"result"`
		Content    string         `json:"content,omitempty"`
	}
	if decoder.Decode(&body) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		len(body.CallDigest) != 64 || strings.Trim(body.CallDigest, "0123456789abcdef") != "" ||
		!validPiToolWireResult(body.Result) {
		return ToolCallEnvelope{}, ToolCallResult{}, ErrPiToolExtension
	}
	tool := permissions.ToolKind(body.Tool)
	if _, ok := governedToolKinds[tool]; !ok {
		return ToolCallEnvelope{}, ToolCallResult{}, ErrPiToolExtension
	}
	hasContent := body.Content != ""
	needsContent := body.Result.Verdict == permissions.VerdictAllow &&
		piToolContentKind(tool)
	if hasContent != needsContent || len(body.Content) > piToolExtensionMaxContentBytes ||
		hasContent && (!utf8.ValidString(body.Content) ||
			piContextContentHasControls([]byte(body.Content))) {
		return ToolCallEnvelope{}, ToolCallResult{}, ErrPiToolExtension
	}
	if hasContent {
		body.Result.ContentDigest = "sha256:" + piContextDigest([]byte(body.Content))
		if body.Result.OutputDigest != body.Result.ContentDigest {
			return ToolCallEnvelope{}, ToolCallResult{}, ErrPiToolExtension
		}
	}
	return ToolCallEnvelope{Call: permissions.ProposedCall{Tool: tool}}, body.Result, nil
}

func validPiToolWireResult(result ToolCallResult) bool {
	if result.Delivery != nil || result.Verdict == permissions.VerdictAsk ||
		result.ApprovalID != "" || result.ApprovalDigest != "" {
		return false
	}
	if result.Verdict == permissions.VerdictAllow {
		return result.ExecutionID != ""
	}
	return result.Verdict == permissions.VerdictDeny && result.ExecutionID == ""
}

func piToolExtensionSource(
	socketPath string,
	capability string,
	allowedSets ...map[permissions.ToolKind]struct{},
) []byte {
	socketLiteral := strconv.Quote(socketPath)
	capabilityLiteral := strconv.Quote(capability)
	allowed := bridgeAllowedTools
	if len(allowedSets) == 1 && len(allowedSets[0]) > 0 {
		allowed = allowedSets[0]
	}
	toolNames := make([]string, 0, len(allowed))
	for tool := range allowed {
		toolNames = append(toolNames, string(tool))
	}
	sort.Strings(toolNames)
	toolEnum, _ := json.Marshal(toolNames)
	contentNames := make([]string, 0, len(allowed))
	for tool := range allowed {
		if piToolContentKind(tool) {
			contentNames = append(contentNames, string(tool))
		}
	}
	sort.Strings(contentNames)
	contentEnum, _ := json.Marshal(contentNames)
	return []byte(fmt.Sprintf(`import net from "node:net";
const socketPath = %s;
const capability = %s;
function exactKeys(value, allowed) {
  return value && typeof value === "object" && !Array.isArray(value) &&
    Object.keys(value).every(key => allowed.includes(key));
}
function requestTool(params, signal) {
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
    if (signal) signal.addEventListener("abort", () => finish(new Error("tool_cancelled")), { once: true });
    socket.setEncoding("utf8");
    socket.on("connect", () => socket.write(JSON.stringify({
      schema_version: 1, capability, envelope: params
    }) + "\n"));
    socket.on("data", chunk => {
      received += chunk;
      if (received.length > 32768) return finish(new Error("tool_unavailable"));
      const newline = received.indexOf("\n");
      if (newline < 0) return;
      if (newline !== received.length - 1) return finish(new Error("tool_unavailable"));
      try {
        const value = JSON.parse(received.slice(0, newline));
        const resultKeys = ["verdict", "execution_id", "approval_id", "approval_digest",
          "result_note", "error_code", "denial_reason", "authorization_path", "exit_code",
          "output_digest", "changed_files_digest", "evidence_id", "duration_ms"];
        if (!exactKeys(value, ["call_digest", "tool", "result", "content"]) ||
            typeof value.call_digest !== "string" || !/^[0-9a-f]{64}$/.test(value.call_digest) ||
            value.tool !== params.call.tool || !exactKeys(value.result, resultKeys) ||
            !["allow", "deny"].includes(value.result.verdict) ||
            (value.result.verdict === "allow" && typeof value.result.execution_id !== "string") ||
		    (%s.includes(value.tool) && value.result.verdict === "allow") !==
              (typeof value.content === "string" && value.content.length > 0 && value.content.length <= 12288)) {
          return finish(new Error("tool_unavailable"));
        }
        finish(undefined, value);
      } catch (_) { finish(new Error("tool_unavailable")); }
    });
    socket.on("error", () => finish(new Error("tool_unavailable")));
    socket.on("end", () => { if (!settled) finish(new Error("tool_unavailable")); });
  });
}
export default function(pi) {
  pi.registerTool({
    name: "loom_tool",
    label: "Loom tool",
    description: "Submit one exact governed ToolCall to the Loom Attempt Tool Gateway.",
    parameters: {
      type: "object",
      properties: {
        job_id: { type: "string", minLength: 1, maxLength: 512 },
        call: {
          type: "object",
          properties: {
            tool: { type: "string", enum: %s },
            command: { type: "string", maxLength: 4096 },
            path: { type: "string", maxLength: 4096 },
            pattern: { type: "string", maxLength: 1024 }
          },
          required: ["tool"],
          additionalProperties: false
        }
      },
      required: ["job_id", "call"],
      additionalProperties: false
    },
    executionMode: "sequential",
    async execute(_toolCallId, params, signal) {
      const value = await requestTool(params, signal);
      return {
        content: [{ type: "text", text: JSON.stringify(value) }],
        details: {
          call_digest: value.call_digest,
          tool: value.tool,
          verdict: value.result.verdict,
          ...(value.result.execution_id ? { execution_id: value.result.execution_id } : {})
        }
      };
    }
  });
}
`, socketLiteral, capabilityLiteral, contentEnum, toolEnum))
}

func piToolContentKind(tool permissions.ToolKind) bool {
	return tool == permissions.ToolRead || tool == permissions.ToolGrep ||
		tool == permissions.ToolWebSearch || tool == permissions.ToolWebFetch ||
		tool == permissions.ToolMCPTool
}
