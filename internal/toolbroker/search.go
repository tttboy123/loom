package toolbroker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Real web-search backend for the Loom harness. It queries the DuckDuckGo
// Lite HTML endpoint over HTTPS using the same SSRF-safe transport as
// WebFetch, so a public query can never be redirected to private address
// space. Results are bounded and each result URL is re-validated as public
// HTTPS before being returned.

const ddgLiteBase = "https://html.duckduckgo.com/html/"

// DDGSearchClient is the production SearchBackend. Zero value is invalid.
type DDGSearchClient struct {
	client         *http.Client
	baseURL        string
	maxResults     int
	maxResultBytes int
}

// NewDDGSearchClient creates the production web-search backend.
func NewDDGSearchClient(
	timeout time.Duration,
	maxResults int,
	maxResultBytes int,
) (*DDGSearchClient, error) {
	return newDDGSearchClient(timeout, ddgLiteBase, maxResults, maxResultBytes)
}

func newDDGSearchClient(
	timeout time.Duration,
	baseURL string,
	maxResults int,
	maxResultBytes int,
) (*DDGSearchClient, error) {
	client, err := NewSystemHTTPClient(timeout)
	if err != nil {
		return nil, err
	}
	return newDDGSearchClientWithClient(baseURL, client, maxResults, maxResultBytes)
}

// newDDGSearchClientWithClient injects a transport (tests use a TLS test
// server); production always uses the SSRF-safe system transport.
func newDDGSearchClientWithClient(
	baseURL string,
	client *http.Client,
	maxResults int,
	maxResultBytes int,
) (*DDGSearchClient, error) {
	if client == nil ||
		maxResults < 1 || maxResults > 16 ||
		maxResultBytes < 256 || maxResultBytes > 64<<10 ||
		!strings.HasPrefix(baseURL, "https://") {
		return nil, ErrInvalidConfig
	}
	return &DDGSearchClient{
		client: client, baseURL: baseURL,
		maxResults: maxResults, maxResultBytes: maxResultBytes,
	}, nil
}

func (search *DDGSearchClient) Search(
	ctx context.Context,
	query string,
	limit int,
) ([]SearchResult, error) {
	if search == nil || search.client == nil ||
		strings.TrimSpace(query) == "" || len(query) > 512 ||
		limit < 1 || limit > search.maxResults {
		return nil, ErrInvalidCall
	}
	endpoint, err := url.Parse(search.baseURL)
	if err != nil {
		return nil, ErrInvalidConfig
	}
	values := endpoint.Query()
	values.Set("q", query)
	endpoint.RawQuery = values.Encode()
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, endpoint.String(), nil,
	)
	if err != nil {
		return nil, ErrInvalidCall
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Loom harness; governed web search)")
	request.Header.Set("Accept", "text/html")
	response, err := search.client.Do(request)
	if err != nil {
		return nil, errors.Join(ErrToolFailed, err)
	}
	if response == nil || response.Body == nil {
		return nil, ErrToolFailed
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ErrToolFailed
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, int64(search.maxResultBytes+1)))
	if err != nil {
		return nil, errors.Join(ErrToolFailed, err)
	}
	if len(body) > search.maxResultBytes {
		return nil, ErrResultTooLarge
	}
	html := string(body)
	if !safeText(html) {
		return nil, ErrToolFailed
	}
	results := extractDDGResults(html, limit)
	if len(results) == 0 {
		return nil, ErrToolFailed
	}
	for _, result := range results {
		if !validPublicHTTPS(result.URL) {
			return nil, ErrToolDenied
		}
	}
	return results, nil
}

var (
	ddgTitleRe = regexp.MustCompile(
		`<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`,
	)
	ddgSnippetRe = regexp.MustCompile(
		`<a[^>]*class="[^"]*result__snippet[^"]*"[^>]*>(.*?)</a>`,
	)
)

// extractDDGResults pulls (title, href, snippet) triples from the DuckDuckGo
// Lite HTML page, bounded to limit. Result URLs that are DDG redirects
// (//duckduckgo.com/l/?uddg=...) are unwrapped to their public target.
func extractDDGResults(html string, limit int) []SearchResult {
	titles := ddgTitleRe.FindAllStringSubmatch(html, -1)
	snippets := ddgSnippetRe.FindAllStringSubmatch(html, -1)
	if len(titles) == 0 {
		return nil
	}
	if len(snippets) > len(titles) {
		snippets = snippets[:len(titles)]
	}
	results := make([]SearchResult, 0, limit)
	for index, match := range titles {
		if len(results) >= limit {
			break
		}
		rawURL := htmlUnescape(match[1])
		target := unwrapDDGURL(rawURL)
		if !validPublicHTTPS(target) {
			continue
		}
		title := boundedText(stripTags(htmlUnescape(match[2])), 256)
		snippet := ""
		if index < len(snippets) {
			snippet = boundedText(stripTags(htmlUnescape(snippets[index][1])), 512)
		}
		if title == "" {
			continue
		}
		results = append(results, SearchResult{Title: title, URL: target, Snippet: snippet})
	}
	return results
}

// unwrapDDGURL converts a DuckDuckGo l/?uddg=<encoded> redirect into the
// encoded public target; anything else is returned unchanged.
func unwrapDDGURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || strings.TrimSuffix(parsed.Host, ".") != "duckduckgo.com" {
		return raw
	}
	target := parsed.Query().Get("uddg")
	if target == "" {
		return raw
	}
	return target
}

var tagRe = regexp.MustCompile(`<[^>]+>`)

func stripTags(value string) string {
	return tagRe.ReplaceAllString(value, " ")
}

func htmlUnescape(value string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`,
		"&#x27;", "'", "&#39;", "'", "&nbsp;", " ",
	)
	return replacer.Replace(value)
}

func boundedText(value string, maximum int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= maximum {
		return value
	}
	if !utf8.ValidString(value[:maximum]) {
		value = value[:maximum]
		for len(value) > 0 && !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}
