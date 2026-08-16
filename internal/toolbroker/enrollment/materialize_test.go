package enrollment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/toolbroker"
	"loom-pi-rebuild/internal/work"
)

type materializeSearchFixture struct{}

func (materializeSearchFixture) Search(
	context.Context, string, int,
) ([]toolbroker.SearchResult, error) {
	return []toolbroker.SearchResult{{Title: "t", URL: "https://example.com"}}, nil
}

type materializeMCPFixture struct{}

func (materializeMCPFixture) CallTool(
	context.Context,
	mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{Content: []mcp.Content{
		mcp.TextContent{Type: "text", Text: "bounded MCP result"},
	}}, nil
}

func materializeEnrollmentInput() work.RemoteToolBackendEnrollmentInput {
	return work.RemoteToolBackendEnrollmentInput{
		Version:                       1,
		EnrollmentID:                  "enr.search.alpha",
		BackendKind:                   work.RemoteToolBackendWebSearch,
		AdapterID:                     work.BuiltInSearchAdapterID,
		ProviderID:                    "deepseek",
		ProviderAccountID:             "deepseek.primary",
		ProviderAccountPolicyVersion:  1,
		ProviderAccountPolicyRevision: 3,
		ProviderAccountPolicyDigest:   strings.Repeat("a", 64),
		EndpointFingerprint:           strings.Repeat("b", 64),
		Revision:                      1,
		Status:                        work.RemoteToolBackendEnrollmentActive,
		MaximumConcurrentCalls:        2,
		MaximumCallsPerAttempt:        4,
		Timeout:                       30 * time.Second,
		MaximumResultBytes:            1 << 16,
		MaximumBudgetUnits:            1000,
		ConfiguredAt:                  time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
	}
}

func mustEnrollment(t *testing.T, input work.RemoteToolBackendEnrollmentInput) work.RemoteToolBackendEnrollment {
	t.Helper()
	enrollment, err := work.NewRemoteToolBackendEnrollment(input)
	if err != nil {
		t.Fatalf("NewRemoteToolBackendEnrollment error = %v", err)
	}
	return enrollment
}

func TestMaterializeSearchEnrollment(t *testing.T) {
	enrollment := mustEnrollment(t, materializeEnrollmentInput())
	executor, err := Materialize(
		enrollment, true,
		MaterializeDeps{Search: materializeSearchFixture{}},
	)
	if err != nil {
		t.Fatalf("Materialize error = %v", err)
	}
	if executor == nil {
		t.Fatal("nil executor")
	}
	allowed := executor.AllowedRemoteTools()
	if len(allowed) != 1 || allowed[0] != permissions.ToolWebSearch {
		t.Fatalf("allowed = %#v", allowed)
	}
	content, err := executor.ExecuteProposalContent(
		context.Background(),
		permissions.ProposedCall{
			Tool: permissions.ToolWebSearch, Path: "Loom status",
		},
	)
	if err != nil {
		t.Fatalf("execute error = %v", err)
	}
	if len(content) == 0 {
		t.Fatal("empty content")
	}
}

func TestMaterializeMCPEnrollment(t *testing.T) {
	input := materializeEnrollmentInput()
	input.BackendKind = work.RemoteToolBackendMCPServer
	input.AdapterID = work.BuiltInMCPAdapterID
	input.MCPServerID = "issues"
	input.AllowedTools = []string{"get_issue", "list_issues"}
	enrollment := mustEnrollment(t, input)
	executor, err := Materialize(
		enrollment, true,
		MaterializeDeps{MCPClients: map[string]toolbroker.MCPToolClient{
			"issues": materializeMCPFixture{},
		}},
	)
	if err != nil {
		t.Fatalf("Materialize error = %v", err)
	}
	allowed := executor.AllowedRemoteTools()
	if len(allowed) != 1 || allowed[0] != permissions.ToolMCPTool {
		t.Fatalf("allowed = %#v", allowed)
	}
	if err := executor.ValidateProposal(permissions.ProposedCall{
		Tool: permissions.ToolMCPTool, Path: "issues/get_issue", Command: "{}",
	}); err != nil {
		t.Fatalf("ValidateProposal error = %v", err)
	}
	if err := executor.ValidateProposal(permissions.ProposedCall{
		Tool: permissions.ToolMCPTool, Path: "issues/not_allowed", Command: "{}",
	}); err == nil {
		t.Fatal("non-allowlisted tool accepted")
	}
}

func TestMaterializeFailsClosed(t *testing.T) {
	searchDeps := MaterializeDeps{Search: materializeSearchFixture{}}
	t.Run("nil enrollment", func(t *testing.T) {
		if _, err := Materialize(work.RemoteToolBackendEnrollment{}, true, searchDeps); !errors.Is(
			err, ErrToolEnrollmentInvalid,
		) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("revoked", func(t *testing.T) {
		input := materializeEnrollmentInput()
		input.Revision = 2
		input.Status = work.RemoteToolBackendEnrollmentRevoked
		enrollment := mustEnrollment(t, input)
		if _, err := Materialize(enrollment, true, searchDeps); !errors.Is(
			err, ErrToolEnrollmentRevoked,
		) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("policy drift", func(t *testing.T) {
		enrollment := mustEnrollment(t, materializeEnrollmentInput())
		if _, err := Materialize(enrollment, false, searchDeps); !errors.Is(
			err, ErrToolEnrollmentPolicyDrift,
		) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("unsupported adapter", func(t *testing.T) {
		input := materializeEnrollmentInput()
		input.AdapterID = "arbitrary.adapter.v9"
		enrollment := mustEnrollment(t, input)
		if _, err := Materialize(enrollment, true, searchDeps); !errors.Is(
			err, ErrToolEnrollmentAdapterUnsupported,
		) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("missing search port", func(t *testing.T) {
		enrollment := mustEnrollment(t, materializeEnrollmentInput())
		if _, err := Materialize(enrollment, true, MaterializeDeps{}); !errors.Is(
			err, ErrToolEnrollmentPortUnavailable,
		) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("missing mcp port", func(t *testing.T) {
		input := materializeEnrollmentInput()
		input.BackendKind = work.RemoteToolBackendMCPServer
		input.AdapterID = work.BuiltInMCPAdapterID
		input.MCPServerID = "issues"
		input.AllowedTools = []string{"get_issue"}
		enrollment := mustEnrollment(t, input)
		if _, err := Materialize(enrollment, true, MaterializeDeps{}); !errors.Is(
			err, ErrToolEnrollmentPortUnavailable,
		) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("adapter kind mismatch", func(t *testing.T) {
		// A catalog-listed adapter whose stored kind disagrees must fail closed.
		input := materializeEnrollmentInput()
		input.AdapterID = work.BuiltInMCPAdapterID // catalog says mcp_server
		enrollment := mustEnrollment(t, input)     // but kind stays web_search
		if _, err := Materialize(enrollment, true, searchDeps); !errors.Is(
			err, ErrToolEnrollmentAdapterUnsupported,
		) {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestMaterializedExecutorSatisfiesInterface(t *testing.T) {
	enrollment := mustEnrollment(t, materializeEnrollmentInput())
	executor, err := Materialize(
		enrollment, true, MaterializeDeps{Search: materializeSearchFixture{}},
	)
	if err != nil {
		t.Fatalf("Materialize error = %v", err)
	}
	var _ execution.RemoteToolExecutor = executor
}
