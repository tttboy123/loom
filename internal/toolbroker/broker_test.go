package toolbroker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/permissions"
)

type searchFixture struct {
	query string
	limit int
}

func (fixture *searchFixture) Search(
	_ context.Context,
	query string,
	limit int,
) ([]SearchResult, error) {
	fixture.query, fixture.limit = query, limit
	return []SearchResult{{
		Title: "Loom docs", URL: "https://example.com/loom", Snippet: "Current Loom documentation.",
	}}, nil
}

type httpFixture struct{ request *http.Request }

func (fixture *httpFixture) Do(request *http.Request) (*http.Response, error) {
	fixture.request = request
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader("bounded page content")),
		Request:    request,
	}, nil
}

type mcpFixture struct {
	server string
	tool   string
	args   json.RawMessage
}

func (fixture *mcpFixture) CallTool(
	_ context.Context,
	server string,
	tool string,
	arguments json.RawMessage,
) (string, error) {
	fixture.server, fixture.tool = server, tool
	fixture.args = append([]byte(nil), arguments...)
	return "bounded MCP result", nil
}

func TestBrokerExecutesBoundedWebSearchFetchAndMCP(t *testing.T) {
	search := &searchFixture{}
	httpClient := &httpFixture{}
	mcp := &mcpFixture{}
	broker, err := New(Config{
		Search: search, HTTP: httpClient, MCP: mcp,
		MCPAllowlist: map[string][]string{"github": {"get_issue"}},
		Timeout:      10 * time.Second, MaxResultBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}

	searchResult, err := broker.Execute(context.Background(), Call{
		Tool: permissions.ToolWebSearch, Query: "latest Loom release", MaxResults: 3,
	})
	if err != nil || search.query != "latest Loom release" || search.limit != 3 ||
		!strings.Contains(searchResult.Content, "https://example.com/loom") ||
		len(searchResult.Sources) != 1 {
		t.Fatalf("search=%#v fixture=%#v error=%v", searchResult, search, err)
	}

	fetchResult, err := broker.Execute(context.Background(), Call{
		Tool: permissions.ToolWebFetch, URL: "https://example.com/loom",
	})
	if err != nil || fetchResult.Content != "bounded page content" ||
		httpClient.request == nil || httpClient.request.Method != http.MethodGet ||
		httpClient.request.Header.Get("Authorization") != "" {
		t.Fatalf("fetch=%#v request=%#v error=%v", fetchResult, httpClient.request, err)
	}

	mcpResult, err := broker.Execute(context.Background(), Call{
		Tool: permissions.ToolMCPTool, MCPServer: "github", MCPTool: "get_issue",
		Arguments: json.RawMessage(`{"owner":"earendil","number":42}`),
	})
	if err != nil || mcpResult.Content != "bounded MCP result" ||
		mcp.server != "github" || mcp.tool != "get_issue" || len(mcp.args) == 0 {
		t.Fatalf("mcp=%#v fixture=%#v error=%v", mcpResult, mcp, err)
	}
}

func TestBrokerPublishesOnlyConfiguredRemoteTools(t *testing.T) {
	broker, err := New(Config{
		Search: &searchFixture{}, HTTP: &httpFixture{}, MCP: &mcpFixture{},
		Timeout: 10 * time.Second, MaxResultBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	allowed := broker.AllowedRemoteTools()
	if len(allowed) != 2 || allowed[0] != permissions.ToolWebSearch ||
		allowed[1] != permissions.ToolWebFetch {
		t.Fatalf("allowed tools=%v", allowed)
	}
	if err := broker.ValidateProposal(permissions.ProposedCall{
		Tool: permissions.ToolMCPTool, Path: "github/get_issue", Command: `{}`,
	}); err == nil {
		t.Fatal("MCP was accepted without an allowlist")
	}
}

func TestBrokerPublishesOnlyBackedRemoteCapabilities(t *testing.T) {
	searchOnly, err := New(Config{
		Search: &searchFixture{}, Timeout: 10 * time.Second, MaxResultBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	if allowed := searchOnly.AllowedRemoteTools(); len(allowed) != 1 ||
		allowed[0] != permissions.ToolWebSearch {
		t.Fatalf("search-only tools=%v", allowed)
	}
	if err := searchOnly.ValidateProposal(permissions.ProposedCall{
		Tool: permissions.ToolWebFetch, Path: "https://example.com/docs",
	}); !errors.Is(err, ErrToolDenied) {
		t.Fatalf("unbacked WebFetch error=%v", err)
	}

	mcpOnly, err := New(Config{
		MCP: &mcpFixture{}, MCPAllowlist: map[string][]string{
			"github": {"get_issue"},
		},
		Timeout: 10 * time.Second, MaxResultBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	if allowed := mcpOnly.AllowedRemoteTools(); len(allowed) != 1 ||
		allowed[0] != permissions.ToolMCPTool {
		t.Fatalf("MCP-only tools=%v", allowed)
	}
	if err := mcpOnly.ValidateProposal(permissions.ProposedCall{
		Tool: permissions.ToolWebSearch, Path: "Loom",
	}); !errors.Is(err, ErrToolDenied) {
		t.Fatalf("unbacked WebSearch error=%v", err)
	}

	if _, err := New(Config{
		MCPAllowlist: map[string][]string{"github": {"get_issue"}},
		Timeout:      10 * time.Second, MaxResultBytes: 4096,
	}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("allowlist without MCP backend error=%v", err)
	}
	if _, err := New(Config{
		Timeout: 10 * time.Second, MaxResultBytes: 4096,
	}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("empty broker error=%v", err)
	}
	if _, err := New(Config{
		MCP: &mcpFixture{}, Timeout: 10 * time.Second, MaxResultBytes: 4096,
	}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("MCP backend without allowlist error=%v", err)
	}
}

func TestBrokerFailsClosedForUnsafeOrUndeclaredCalls(t *testing.T) {
	broker, err := New(Config{
		Search: &searchFixture{}, HTTP: &httpFixture{}, MCP: &mcpFixture{},
		MCPAllowlist: map[string][]string{"github": {"get_issue"}},
		Timeout:      10 * time.Second, MaxResultBytes: 256,
	})
	if err != nil {
		t.Fatal(err)
	}
	invalid := []Call{
		{Tool: permissions.ToolWebSearch, Query: ""},
		{Tool: permissions.ToolWebFetch, URL: "http://example.com"},
		{Tool: permissions.ToolWebFetch, URL: "https://127.0.0.1/private"},
		{Tool: permissions.ToolWebFetch, URL: "https://user:pass@example.com"},
		{Tool: permissions.ToolMCPTool, MCPServer: "github", MCPTool: "delete_repo", Arguments: json.RawMessage(`{}`)},
		{Tool: permissions.ToolMCPTool, MCPServer: "github", MCPTool: "get_issue", Arguments: json.RawMessage(`{"owner":"a","owner":"b"}`)},
		{Tool: permissions.ToolBash},
	}
	for _, call := range invalid {
		if _, err := broker.Execute(context.Background(), call); err == nil {
			t.Fatalf("unsafe call accepted: %#v", call)
		}
	}
}

func TestCallFromProposalMapsExistingPermissionEnvelopeWithoutAmbiguity(t *testing.T) {
	tests := []struct {
		proposal permissions.ProposedCall
		want     Call
	}{
		{
			proposal: permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "latest Loom release"},
			want:     Call{Tool: permissions.ToolWebSearch, Query: "latest Loom release", MaxResults: 5},
		},
		{
			proposal: permissions.ProposedCall{Tool: permissions.ToolWebFetch, Path: "https://example.com/docs"},
			want:     Call{Tool: permissions.ToolWebFetch, URL: "https://example.com/docs"},
		},
		{
			proposal: permissions.ProposedCall{
				Tool: permissions.ToolMCPTool, Path: "github/get_issue",
				Command: `{"owner":"earendil","number":42}`,
			},
			want: Call{
				Tool: permissions.ToolMCPTool, MCPServer: "github", MCPTool: "get_issue",
				Arguments: json.RawMessage(`{"owner":"earendil","number":42}`),
			},
		},
	}
	for _, test := range tests {
		got, err := CallFromProposal(test.proposal)
		if err != nil || got.Tool != test.want.Tool || got.Query != test.want.Query ||
			got.MaxResults != test.want.MaxResults || got.URL != test.want.URL ||
			got.MCPServer != test.want.MCPServer || got.MCPTool != test.want.MCPTool ||
			string(got.Arguments) != string(test.want.Arguments) {
			t.Fatalf("proposal=%#v got=%#v want=%#v error=%v", test.proposal, got, test.want, err)
		}
	}
	if _, err := CallFromProposal(permissions.ProposedCall{
		Tool: permissions.ToolMCPTool, Path: "github/get/issue", Command: `{}`,
	}); err == nil {
		t.Fatal("ambiguous MCP target accepted")
	}
}

func TestBrokerImplementsExecutionRemoteContentBoundary(t *testing.T) {
	broker, err := New(Config{
		Search: &searchFixture{}, HTTP: &httpFixture{}, MCP: &mcpFixture{},
		MCPAllowlist: map[string][]string{"github": {"get_issue"}},
		Timeout:      10 * time.Second, MaxResultBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal := permissions.ProposedCall{
		Tool: permissions.ToolWebSearch, Path: "Loom result boundary",
	}
	if err := broker.ValidateProposal(proposal); err != nil {
		t.Fatal(err)
	}
	content, err := broker.ExecuteProposalContent(context.Background(), proposal)
	if err != nil || !strings.Contains(string(content), "Loom docs") {
		t.Fatalf("content=%q err=%v", content, err)
	}
	for index := range content {
		content[index] = 0
	}
	if err := broker.ValidateProposal(permissions.ProposedCall{
		Tool: permissions.ToolMCPTool, Path: "github/get/issue", Command: `{}`,
	}); err == nil {
		t.Fatal("ambiguous MCP target passed pre-dispatch validation")
	}
	for _, invalid := range []permissions.ProposedCall{
		{
			Tool: permissions.ToolWebFetch, Path: "https://127.0.0.1/private",
		},
		{
			Tool: permissions.ToolMCPTool, Path: "github/delete_repo", Command: `{}`,
		},
		{
			Tool: permissions.ToolMCPTool, Path: "github/get_issue",
			Command: `{"owner":"a","owner":"b"}`,
		},
	} {
		if err := broker.ValidateProposal(invalid); err == nil {
			t.Fatalf("unsafe proposal passed pre-dispatch validation: %#v", invalid)
		}
	}
}
