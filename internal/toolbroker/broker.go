// Package toolbroker provides Loom-owned, bounded network and MCP tool
// execution. Authorization remains outside this package; callers must pass
// only calls already allowed by the frozen permission profile.
package toolbroker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/permissions"
)

var (
	ErrInvalidConfig  = errors.New("invalid tool broker config")
	ErrInvalidCall    = errors.New("invalid tool broker call")
	ErrToolDenied     = errors.New("tool broker call denied")
	ErrToolFailed     = errors.New("tool broker call failed")
	ErrResultTooLarge = errors.New("tool broker result too large")
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

type SearchBackend interface {
	Search(context.Context, string, int) ([]SearchResult, error)
}

type MCPCaller interface {
	CallTool(context.Context, string, string, json.RawMessage) (string, error)
}

type Config struct {
	Search         SearchBackend
	HTTP           HTTPDoer
	MCP            MCPCaller
	MCPAllowlist   map[string][]string
	Timeout        time.Duration
	MaxResultBytes int
}

type Call struct {
	Tool       permissions.ToolKind
	Query      string
	MaxResults int
	URL        string
	MCPServer  string
	MCPTool    string
	Arguments  json.RawMessage
}

type Result struct {
	Content string
	Sources []string
}

type Broker struct {
	search         SearchBackend
	http           HTTPDoer
	mcp            MCPCaller
	mcpAllowlist   map[string]map[string]bool
	timeout        time.Duration
	maxResultBytes int
}

func (broker *Broker) AllowedRemoteTools() []permissions.ToolKind {
	if broker == nil {
		return nil
	}
	allowed := make([]permissions.ToolKind, 0, 3)
	if !nilInterface(broker.search) {
		allowed = append(allowed, permissions.ToolWebSearch)
	}
	if !nilInterface(broker.http) {
		allowed = append(allowed, permissions.ToolWebFetch)
	}
	if !nilInterface(broker.mcp) && len(broker.mcpAllowlist) > 0 {
		allowed = append(allowed, permissions.ToolMCPTool)
	}
	return allowed
}

// CallFromProposal maps the existing permission envelope onto the richer
// broker call without changing the authoritative authorization model. Path is
// the query/URL/MCP target; Command carries MCP's JSON object only.
func CallFromProposal(proposal permissions.ProposedCall) (Call, error) {
	switch proposal.Tool {
	case permissions.ToolWebSearch:
		if strings.TrimSpace(proposal.Path) == "" || proposal.Command != "" {
			return Call{}, ErrInvalidCall
		}
		return Call{
			Tool: proposal.Tool, Query: proposal.Path, MaxResults: 5,
		}, nil
	case permissions.ToolWebFetch:
		if strings.TrimSpace(proposal.Path) == "" || proposal.Command != "" {
			return Call{}, ErrInvalidCall
		}
		return Call{Tool: proposal.Tool, URL: proposal.Path}, nil
	case permissions.ToolMCPTool:
		parts := strings.Split(proposal.Path, "/")
		if len(parts) != 2 || !validIdentifier(parts[0]) ||
			!validIdentifier(parts[1]) || strings.TrimSpace(proposal.Command) == "" {
			return Call{}, ErrInvalidCall
		}
		return Call{
			Tool: proposal.Tool, MCPServer: parts[0], MCPTool: parts[1],
			Arguments: json.RawMessage(proposal.Command),
		}, nil
	default:
		return Call{}, ErrToolDenied
	}
}

func (broker *Broker) ExecuteProposal(
	ctx context.Context,
	proposal permissions.ProposedCall,
) (Result, error) {
	call, err := CallFromProposal(proposal)
	if err != nil {
		return Result{}, err
	}
	return broker.Execute(ctx, call)
}

func (broker *Broker) ValidateProposal(proposal permissions.ProposedCall) error {
	if broker == nil {
		return ErrInvalidCall
	}
	call, err := CallFromProposal(proposal)
	if err != nil {
		return err
	}
	return broker.validateCall(call)
}

// ExecuteProposalContent returns an owned mutable copy for the execution
// authority's encrypted result-commit boundary.
func (broker *Broker) ExecuteProposalContent(
	ctx context.Context,
	proposal permissions.ProposedCall,
) ([]byte, error) {
	result, err := broker.ExecuteProposal(ctx, proposal)
	if err != nil {
		return nil, err
	}
	return []byte(result.Content), nil
}

func New(config Config) (*Broker, error) {
	searchAvailable := !nilInterface(config.Search)
	httpAvailable := !nilInterface(config.HTTP)
	mcpAvailable := !nilInterface(config.MCP)
	if !searchAvailable && !httpAvailable &&
		(!mcpAvailable || len(config.MCPAllowlist) == 0) ||
		len(config.MCPAllowlist) > 0 && !mcpAvailable ||
		config.Timeout <= 0 ||
		config.Timeout > 2*time.Minute || config.MaxResultBytes < 256 ||
		config.MaxResultBytes > 1<<20 {
		return nil, ErrInvalidConfig
	}
	allowlist := make(map[string]map[string]bool, len(config.MCPAllowlist))
	for server, tools := range config.MCPAllowlist {
		if !validIdentifier(server) || len(tools) == 0 {
			return nil, ErrInvalidConfig
		}
		allowed := make(map[string]bool, len(tools))
		for _, tool := range tools {
			if !validIdentifier(tool) || allowed[tool] {
				return nil, ErrInvalidConfig
			}
			allowed[tool] = true
		}
		allowlist[server] = allowed
	}
	return &Broker{
		search: config.Search, http: config.HTTP, mcp: config.MCP,
		mcpAllowlist: allowlist, timeout: config.Timeout,
		maxResultBytes: config.MaxResultBytes,
	}, nil
}

func (broker *Broker) Execute(ctx context.Context, call Call) (Result, error) {
	if broker == nil || ctx == nil {
		return Result{}, ErrInvalidCall
	}
	if err := broker.validateCall(call); err != nil {
		return Result{}, err
	}
	toolContext, cancel := context.WithTimeout(ctx, broker.timeout)
	defer cancel()
	switch call.Tool {
	case permissions.ToolWebSearch:
		return broker.webSearch(toolContext, call)
	case permissions.ToolWebFetch:
		return broker.webFetch(toolContext, call)
	case permissions.ToolMCPTool:
		return broker.mcpCall(toolContext, call)
	default:
		return Result{}, ErrToolDenied
	}
}

func (broker *Broker) validateCall(call Call) error {
	if broker == nil {
		return ErrInvalidCall
	}
	switch call.Tool {
	case permissions.ToolWebSearch:
		if nilInterface(broker.search) {
			return ErrToolDenied
		}
		query := strings.TrimSpace(call.Query)
		if query == "" || len(query) > 512 || !safeText(query) ||
			call.MaxResults < 1 || call.MaxResults > 10 {
			return ErrInvalidCall
		}
		return nil
	case permissions.ToolWebFetch:
		if nilInterface(broker.http) {
			return ErrToolDenied
		}
		if !validPublicHTTPS(call.URL) {
			return ErrInvalidCall
		}
		return nil
	case permissions.ToolMCPTool:
		if nilInterface(broker.mcp) ||
			!validIdentifier(call.MCPServer) || !validIdentifier(call.MCPTool) ||
			!broker.mcpAllowlist[call.MCPServer][call.MCPTool] ||
			len(call.Arguments) == 0 || len(call.Arguments) > 16<<10 ||
			rejectDuplicateJSONKeys(call.Arguments) != nil {
			return ErrToolDenied
		}
		var arguments map[string]any
		decoder := json.NewDecoder(bytes.NewReader(call.Arguments))
		decoder.UseNumber()
		if decoder.Decode(&arguments) != nil || arguments == nil ||
			decoder.Decode(&struct{}{}) != io.EOF {
			return ErrInvalidCall
		}
		return nil
	default:
		return ErrToolDenied
	}
}

func (broker *Broker) webSearch(ctx context.Context, call Call) (Result, error) {
	query := strings.TrimSpace(call.Query)
	limit := call.MaxResults
	if query == "" || len(query) > 512 || !safeText(query) || limit < 1 || limit > 10 {
		return Result{}, ErrInvalidCall
	}
	results, err := broker.search.Search(ctx, query, limit)
	if err != nil {
		return Result{}, errors.Join(ErrToolFailed, err)
	}
	if len(results) == 0 || len(results) > limit {
		return Result{}, ErrToolFailed
	}
	var content strings.Builder
	sources := make([]string, 0, len(results))
	for index, result := range results {
		if strings.TrimSpace(result.Title) == "" || len(result.Title) > 512 ||
			len(result.Snippet) > 4096 || !safeText(result.Title) ||
			!safeText(result.Snippet) || !validPublicHTTPS(result.URL) {
			return Result{}, ErrToolFailed
		}
		fmt.Fprintf(&content, "%d. %s\nURL: %s\n%s\n", index+1,
			strings.TrimSpace(result.Title), result.URL, strings.TrimSpace(result.Snippet))
		sources = append(sources, result.URL)
		if content.Len() > broker.maxResultBytes {
			return Result{}, ErrResultTooLarge
		}
	}
	return Result{Content: strings.TrimSpace(content.String()), Sources: sources}, nil
}

func (broker *Broker) webFetch(ctx context.Context, call Call) (Result, error) {
	if !validPublicHTTPS(call.URL) {
		return Result{}, ErrInvalidCall
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, call.URL, nil)
	if err != nil {
		return Result{}, ErrInvalidCall
	}
	request.Header.Set("Accept", "text/plain, text/html, application/json, application/xml;q=0.8")
	request.Header.Set("User-Agent", "Loom-ToolBroker/1")
	response, err := broker.http.Do(request)
	if err != nil {
		return Result{}, errors.Join(ErrToolFailed, err)
	}
	if response == nil || response.Body == nil {
		return Result{}, ErrToolFailed
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 ||
		response.Request == nil || !validPublicHTTPS(response.Request.URL.String()) ||
		!allowedContentType(response.Header.Get("Content-Type")) {
		return Result{}, ErrToolFailed
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, int64(broker.maxResultBytes+1)))
	if err != nil {
		return Result{}, errors.Join(ErrToolFailed, err)
	}
	if len(body) > broker.maxResultBytes {
		return Result{}, ErrResultTooLarge
	}
	content := strings.TrimSpace(string(body))
	if content == "" || !safeText(content) {
		return Result{}, ErrToolFailed
	}
	return Result{Content: content, Sources: []string{response.Request.URL.String()}}, nil
}

func (broker *Broker) mcpCall(ctx context.Context, call Call) (Result, error) {
	if !validIdentifier(call.MCPServer) || !validIdentifier(call.MCPTool) ||
		!broker.mcpAllowlist[call.MCPServer][call.MCPTool] ||
		len(call.Arguments) == 0 || len(call.Arguments) > 16<<10 ||
		rejectDuplicateJSONKeys(call.Arguments) != nil {
		return Result{}, ErrToolDenied
	}
	var arguments map[string]any
	decoder := json.NewDecoder(bytes.NewReader(call.Arguments))
	decoder.UseNumber()
	if decoder.Decode(&arguments) != nil || arguments == nil ||
		decoder.Decode(&struct{}{}) != io.EOF {
		return Result{}, ErrInvalidCall
	}
	content, err := broker.mcp.CallTool(ctx, call.MCPServer, call.MCPTool, call.Arguments)
	if err != nil {
		return Result{}, errors.Join(ErrToolFailed, err)
	}
	content = strings.TrimSpace(content)
	if content == "" || len(content) > broker.maxResultBytes || !safeText(content) {
		if len(content) > broker.maxResultBytes {
			return Result{}, ErrResultTooLarge
		}
		return Result{}, ErrToolFailed
	}
	return Result{Content: content}, nil
}

func validPublicHTTPS(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" ||
		parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" && len(parsed.RawQuery) > 4096 {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return false
	}
	return parsed.Port() == "" || parsed.Port() == "443"
}

func allowedContentType(value string) bool {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	switch value {
	case "text/plain", "text/html", "application/json", "application/xml", "text/xml":
		return true
	default:
		return false
	}
}

func safeText(value string) bool {
	if !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return false
	}
	for _, character := range value {
		if character != '\n' && character != '\r' && character != '\t' && unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !(unicode.IsLetter(character) || unicode.IsDigit(character) ||
			character == '-' || character == '_' || character == '.') {
			return false
		}
	}
	return true
}

func rejectDuplicateJSONKeys(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := walkJSONValue(decoder); err != nil {
		return err
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) || token != nil {
		return ErrInvalidCall
	}
	return nil
}

func walkJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok || seen[key] {
				return ErrInvalidCall
			}
			seen[key] = true
			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return ErrInvalidCall
		}
	case '[':
		for decoder.More() {
			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return ErrInvalidCall
		}
	default:
		return ErrInvalidCall
	}
	return nil
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
