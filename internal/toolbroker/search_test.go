package toolbroker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type searchRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip searchRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func liteHTML(results ...string) string {
	var builder strings.Builder
	builder.WriteString("<html><body>")
	for _, result := range results {
		builder.WriteString(result)
	}
	builder.WriteString("</body></html>")
	return builder.String()
}

func liteResult(href, title, snippet string) string {
	return `<div class="result"><h2 class="result__title"><a rel="nofollow" class="result__a" href="` +
		href + `">` + title + `</a></h2><a class="result__snippet" href="` + href + `">` + snippet + `</a></div>`
}

func TestDDGSearchExtractsBoundedResultsAndUnwrapsRedirects(t *testing.T) {
	html := liteHTML(
		liteResult("https://example.com/a", "First &amp; Result", "Bounded snippet one"),
		liteResult("//duckduckgo.com/l/?uddg="+strings.ReplaceAll(urlQueryEscape("https://example.com/b"), "+", "%20"), "Second", "Snippet two"),
		liteResult("http://192.168.1.5/private", "Private", "must be skipped"),
	)
	results := extractDDGResults(html, 5)
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].Title != "First & Result" || results[0].URL != "https://example.com/a" ||
		results[0].Snippet != "Bounded snippet one" {
		t.Fatalf("first = %#v", results[0])
	}
	if results[1].URL != "https://example.com/b" || results[1].Title != "Second" {
		t.Fatalf("second (unwrapped) = %#v", results[1])
	}
	// Limit is honored.
	if limited := extractDDGResults(html, 1); len(limited) != 1 {
		t.Fatalf("limited = %#v", limited)
	}
}

func urlQueryEscape(value string) string {
	escaped := strings.ReplaceAll(value, ":", "%3A")
	escaped = strings.ReplaceAll(escaped, "/", "%2F")
	return escaped
}

func TestDDGSearchClientBoundedAndRejectsNonPublicTargets(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte(liteHTML(
			liteResult("https://example.com/a", "Loom docs", "Current documentation."),
			liteResult("https://example.com/b", "Loom second", "More content."),
		)))
	}))
	defer server.Close()
	search, err := newDDGSearchClientWithClient(
		server.URL, server.Client(), 4, 32<<10,
	)
	if err != nil {
		t.Fatal(err)
	}
	results, err := search.Search(context.Background(), "loom governed handoff", 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 || results[0].Title != "Loom docs" {
		t.Fatalf("results = %#v", results)
	}

	// A page with only private-address results yields no usable results.
	server2 := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		_, _ = writer.Write([]byte(liteHTML(
			liteResult("http://192.168.1.5/x", "Private", "skip"),
			liteResult("https://localhost/x", "Loopback", "skip"),
		)))
	}))
	defer server2.Close()
	search2, err := newDDGSearchClientWithClient(
		server2.URL, server2.Client(), 4, 32<<10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := search2.Search(context.Background(), "private", 3); !errors.Is(err, ErrToolFailed) {
		t.Fatalf("private-only search error = %v", err)
	}

	// Invalid inputs are rejected before any network.
	if _, err := search.Search(context.Background(), "", 3); !errors.Is(err, ErrInvalidCall) {
		t.Fatalf("empty query error = %v", err)
	}
	if _, err := search.Search(context.Background(), "x", 0); !errors.Is(err, ErrInvalidCall) {
		t.Fatalf("zero limit error = %v", err)
	}
	if _, err := newDDGSearchClientWithClient(server.URL, server.Client(), 0, 1024); err == nil {
		t.Fatal("zero maxResults accepted")
	}
}

func TestDDGSearchFailsClosedWithoutFallbackEgress(t *testing.T) {
	for _, test := range []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "DDG failure", statusCode: http.StatusBadGateway},
		{name: "DDG no result", statusCode: http.StatusOK, body: liteHTML()},
	} {
		t.Run(test.name, func(t *testing.T) {
			var requests []string
			client := &http.Client{Transport: searchRoundTripper(func(request *http.Request) (*http.Response, error) {
				requests = append(requests, request.URL.String())
				return &http.Response{
					StatusCode: test.statusCode,
					Body:       io.NopCloser(strings.NewReader(test.body)),
					Header:     make(http.Header),
					Request:    request,
				}, nil
			})}
			search, err := newDDGSearchClientWithClient(ddgLiteBase, client, 4, 32<<10)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := search.Search(context.Background(), "loom", 3); !errors.Is(err, ErrToolFailed) {
				t.Fatalf("Search() error = %v, want ErrToolFailed", err)
			}
			if len(requests) != 1 || !strings.HasPrefix(requests[0], ddgLiteBase) {
				t.Fatalf("requests = %v, want one DDG request", requests)
			}
		})
	}
}

// TestDDGSearchLiveQueriesInternet proves the Loom harness search backend
// really queries the internet over HTTPS (no API key, DuckDuckGo Lite).
func TestDDGSearchLiveQueriesInternet(t *testing.T) {
	if os.Getenv("LOOM_LIVE_NET") != "1" {
		t.Skip("set LOOM_LIVE_NET=1 to run the live network probe")
	}
	search, err := NewDDGSearchClient(20*time.Second, 5, 48<<10)
	if err != nil {
		t.Fatal(err)
	}
	results, err := search.Search(context.Background(), "Loom governed handoff", 3)
	if err != nil {
		// The public HTML search endpoint can block datacenter IPs; that is an
		// external dependency, not a Loom defect. WebFetch remains the stable
		// live internet-access proof; this test asserts strictly when results
		// ARE returned.
		if errors.Is(err, ErrToolFailed) {
			t.Skipf("external search endpoint blocked from this host: %v", err)
		}
		t.Fatalf("live search failed (is network up?): %v", err)
	}
	if len(results) == 0 {
		t.Skip("external search endpoint returned no parseable results")
	}
	for _, result := range results {
		if !validPublicHTTPS(result.URL) || result.Title == "" {
			t.Fatalf("invalid live result = %#v", result)
		}
	}
	t.Logf("live search ok: %d results (first: %s)", len(results), results[0].Title)
}
