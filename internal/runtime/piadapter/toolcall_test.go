package piadapter

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/permissions"
)

type piToolCapabilityFixture struct {
	allowed []permissions.ToolKind
}

func (fixture piToolCapabilityFixture) AllowedToolCalls() []permissions.ToolKind {
	return append([]permissions.ToolKind(nil), fixture.allowed...)
}

func (piToolCapabilityFixture) ExecuteToolCall(
	context.Context,
	ToolCallEnvelope,
	ToolCallBinding,
) (ToolCallResult, error) {
	return ToolCallResult{}, nil
}

func TestRedW1_DecodeValidEnvelope(t *testing.T) {
	envelope, err := DecodeToolCallEnvelope([]byte(
		`{"job_id":"job-1","call":{"tool":"Bash","command":"printf hi","path":""}}`,
	))
	if err != nil {
		t.Fatalf("DecodeToolCallEnvelope() error = %v", err)
	}
	if envelope.JobID != "job-1" || envelope.Call.Tool != permissions.ToolBash ||
		envelope.Call.Command != "printf hi" {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestP2DAllowedPiToolCallsFailsClosedForUnknownCapability(t *testing.T) {
	allowed := allowedPiToolCalls(piToolCapabilityFixture{allowed: []permissions.ToolKind{
		permissions.ToolRead, permissions.ToolWebSearch,
	}})
	if len(allowed) != 2 {
		t.Fatalf("allowed=%v", allowed)
	}
	invalid := allowedPiToolCalls(piToolCapabilityFixture{allowed: []permissions.ToolKind{
		permissions.ToolRead, permissions.ToolKind("UnknownRemoteTool"),
	}})
	if len(invalid) != 0 {
		t.Fatalf("unknown capability was published: %v", invalid)
	}
}

func TestP2DToolResultFrameOmitsProposalArguments(t *testing.T) {
	marker := "private-command-must-stay-in-vault"
	envelope := ToolCallEnvelope{
		JobID: "job-private",
		Call: permissions.ProposedCall{
			Tool: permissions.ToolBash, Command: "printf " + marker,
			Path: "/private/path/" + marker,
		},
	}
	payload, err := marshalToolCallResultPayload(envelope, ToolCallResult{
		Verdict: permissions.VerdictAllow, ExecutionID: "execution-1",
		OutputDigest: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(payload, []byte(marker)) || bytes.Contains(payload, []byte(`"envelope"`)) ||
		!bytes.Contains(payload, []byte(permissions.ProposedCallDigest(envelope.Call))) ||
		!bytes.Contains(payload, []byte(`"tool":"Bash"`)) {
		t.Fatalf("unsafe or incomplete Tool result frame: %s", payload)
	}
}

func TestRedW2_RejectUnknownKeys(t *testing.T) {
	if _, err := DecodeToolCallEnvelope([]byte(
		`{"job_id":"j","call":{"tool":"Bash","command":"x"},"extra":1}`,
	)); err == nil {
		t.Fatal("unknown top-level key must be rejected")
	}
	if _, err := DecodeToolCallEnvelope([]byte(
		`{"job_id":"j","call":{"tool":"Bash","command":"x","hack":true}}`,
	)); err == nil {
		t.Fatal("unknown call key must be rejected")
	}
}

func TestRedW3_RejectInvalidCalls(t *testing.T) {
	for _, payload := range []string{
		`{"job_id":"","call":{"tool":"Bash","command":"x"}}`,
		`{"job_id":"j","call":{"tool":"Bogus","command":"x"}}`,
		`{"job_id":"j","call":{"tool":"Bash","command":""}}`,
		`{"job_id":"j"}`,
		`{"job_id":"j","call":{"tool":"Read","path":"","command":""}}`,
		`{"job_id":"j","call":{"tool":"Bash","command":"x"},"nested":{"a":1}}`,
	} {
		if _, err := DecodeToolCallEnvelope([]byte(payload)); err == nil {
			t.Fatalf("invalid envelope must be rejected: %s", payload)
		}
	}
}

func TestRedW4_RejectSizeAndUnboundTool(t *testing.T) {
	// 命令内容授权由权限层裁决（Evaluate 会 ask/deny rm），解码器不越权判断。
	big := `{"job_id":"` + strings.Repeat("a", 4096) + `","call":{"tool":"Bash","command":"x"}}`
	if _, err := DecodeToolCallEnvelope([]byte(big)); err == nil {
		t.Fatal("oversized envelope must be rejected")
	}
}

func TestRedW5_SystemPromptContract(t *testing.T) {
	prompt := ToolCallSystemPrompt()
	for _, forbidden := range []string{
		"Use no tools", "BEGIN PRIVATE KEY",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("system prompt must not contain %q", forbidden)
		}
	}
	if !strings.Contains(prompt, "JSON") || !strings.Contains(prompt, "job_id") {
		t.Fatalf("system prompt must declare the envelope protocol")
	}
	for _, required := range []string{
		"provider=loom-local",
		"model=" + piRPCModelID,
		"Bash, Edit, Grep, Read",
		"only a proposal",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("system prompt missing %q: %s", required, prompt)
		}
	}
	if strings.Contains(prompt, "WebSearch") || strings.Contains(prompt, "MCPTool") {
		t.Fatalf("system prompt advertised an unbridged tool: %s", prompt)
	}
}
