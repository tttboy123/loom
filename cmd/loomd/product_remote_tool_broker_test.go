package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/toolbroker"
)

type productRemoteSearchFixture struct{}

func (productRemoteSearchFixture) Search(
	context.Context,
	string,
	int,
) ([]toolbroker.SearchResult, error) {
	return []toolbroker.SearchResult{{
		Title: "Loom", URL: "https://example.com/loom", Snippet: "bounded result",
	}}, nil
}

type productBlockingRemoteSearchFixture struct {
	started chan struct{}
}

func (fixture productBlockingRemoteSearchFixture) Search(
	ctx context.Context,
	_ string,
	_ int,
) ([]toolbroker.SearchResult, error) {
	close(fixture.started)
	<-ctx.Done()
	return nil, context.Cause(ctx)
}

type productRemoteMCPFixture struct{}

func (productRemoteMCPFixture) CallTool(
	context.Context,
	mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{Content: []mcp.Content{
		mcp.TextContent{Type: "text", Text: "bounded MCP result"},
	}}, nil
}

func TestProductRemoteToolBrokerCompositionIsExplicitAndFailClosed(t *testing.T) {
	executor, effect, err := newProductRemoteToolBroker(
		context.Background(),
		nil,
	)
	if err != nil || executor != nil || effect != nil {
		t.Fatalf("default executor=%#v effect=%#v err=%v", executor, effect, err)
	}

	executor, effect, err = newProductRemoteToolBroker(
		context.Background(),
		&productRemoteToolBrokerConfig{
			Search: productRemoteSearchFixture{},
			MCPClients: map[string]toolbroker.MCPToolClient{
				"issues": productRemoteMCPFixture{},
			},
			MCPAllowlist: map[string][]string{"issues": {"get_issue"}},
			Timeout:      10 * time.Second, MaxResultBytes: 4096,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if executor == nil || effect == nil {
		t.Fatalf("configured executor=%#v effect=%#v", executor, effect)
	}
	allowed := executor.AllowedRemoteTools()
	if len(allowed) != 2 || allowed[0] != permissions.ToolWebSearch ||
		allowed[1] != permissions.ToolMCPTool {
		t.Fatalf("allowed tools=%v", allowed)
	}
	for _, proposal := range []permissions.ProposedCall{
		{Tool: permissions.ToolWebSearch, Path: "Loom status"},
		{Tool: permissions.ToolMCPTool, Path: "issues/get_issue", Command: `{}`},
	} {
		content, executeErr := executor.ExecuteProposalContent(
			context.Background(), proposal,
		)
		if executeErr != nil || len(content) == 0 {
			t.Fatalf("proposal=%#v content=%q err=%v", proposal, content, executeErr)
		}
		for index := range content {
			content[index] = 0
		}
	}
	if closeErr := effect.Close(context.Background()); closeErr != nil {
		t.Fatal(closeErr)
	}
	if len(executor.AllowedRemoteTools()) != 0 {
		t.Fatalf("closed executor retained tools=%v", executor.AllowedRemoteTools())
	}
	if err := executor.ValidateProposal(permissions.ProposedCall{
		Tool: permissions.ToolWebSearch, Path: "Loom status",
	}); !errors.Is(err, toolbroker.ErrToolDenied) {
		t.Fatalf("closed executor validation error=%v", err)
	}
	if _, err := executor.ExecuteProposalContent(
		context.Background(),
		permissions.ProposedCall{
			Tool: permissions.ToolWebSearch, Path: "Loom status",
		},
	); !errors.Is(err, toolbroker.ErrToolDenied) {
		t.Fatalf("closed executor execution error=%v", err)
	}
	if closeErr := effect.Close(context.Background()); closeErr != nil {
		t.Fatalf("idempotent close error=%v", closeErr)
	}
}

func TestProductRemoteToolBrokerCloseCancelsActiveCall(t *testing.T) {
	started := make(chan struct{})
	executor, effect, err := newProductRemoteToolBroker(
		context.Background(),
		&productRemoteToolBrokerConfig{
			Search:  productBlockingRemoteSearchFixture{started: started},
			Timeout: time.Minute, MaxResultBytes: 4096,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		content, executeErr := executor.ExecuteProposalContent(
			context.Background(),
			permissions.ProposedCall{
				Tool: permissions.ToolWebSearch, Path: "Loom status",
			},
		)
		for index := range content {
			content[index] = 0
		}
		result <- executeErr
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("remote call did not start")
	}
	if err := effect.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case executeErr := <-result:
		if executeErr == nil {
			t.Fatal("active remote call succeeded after composition close")
		}
	case <-time.After(time.Second):
		t.Fatal("composition close did not cancel active remote call")
	}
}

func TestProductRemoteToolBrokerRejectsUnbackedMCPAndUnsafeConfig(t *testing.T) {
	_, _, err := newProductRemoteToolBroker(
		context.Background(),
		&productRemoteToolBrokerConfig{
			MCPAllowlist: map[string][]string{"issues": {"get_issue"}},
			Timeout:      10 * time.Second, MaxResultBytes: 4096,
		},
	)
	if !errors.Is(err, toolbroker.ErrInvalidConfig) {
		t.Fatalf("unbacked MCP error=%v", err)
	}

	_, _, err = newProductRemoteToolBroker(
		context.Background(),
		&productRemoteToolBrokerConfig{
			Search: productRemoteSearchFixture{},
			MCPClients: map[string]toolbroker.MCPToolClient{
				"issues": productRemoteMCPFixture{},
			},
			MCPAllowlist: map[string][]string{"issues": {"delete_repo"}},
			Timeout:      0, MaxResultBytes: 4096,
		},
	)
	if !errors.Is(err, toolbroker.ErrInvalidConfig) {
		t.Fatalf("unsafe timeout error=%v", err)
	}
}

func TestProductRemoteToolBrokerConstructsOnlyInsideWorkBundle(t *testing.T) {
	work, err := parser.ParseFile(
		token.NewFileSet(), "product_composition_work.go", nil, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	foundWorkConstruction := false
	for _, declaration := range work.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductWorkRouteFactory" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			identifier, ok := call.Fun.(*ast.Ident)
			if ok && identifier.Name == "newProductRemoteToolBroker" {
				foundWorkConstruction = true
			}
			return true
		})
	}
	if !foundWorkConstruction {
		t.Fatal("loom-work does not construct the remote Tool Broker during Start")
	}

	product, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range product.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductDaemonRunnerWithPreparedDecisions" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			identifier, ok := call.Fun.(*ast.Ident)
			if ok && identifier.Name == "newProductRemoteToolBroker" {
				t.Error("product builder constructs remote Tool Broker outside loom-work")
			}
			return true
		})
	}
}
