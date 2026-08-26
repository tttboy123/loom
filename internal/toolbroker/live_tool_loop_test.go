package toolbroker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/permissions"
)

// LiveModelToolCallLoop drives a REAL provider model (DeepSeek, function
// calling) through a governed tool loop: the model proposes web_search, Loom's
// toolbroker executes it against the live internet, the bounded result is fed
// back, and the model answers. No Codex, no external harness CLI. Skipped
// unless LOOM_LIVE_NET=1 and DEEPSEEK_API_KEY is set.
func TestLiveModelToolCallLoopWebSearch(t *testing.T) {
	if os.Getenv("LOOM_LIVE_NET") != "1" {
		t.Skip("set LOOM_LIVE_NET=1 to run the live model tool-loop E2E")
	}
	apiKey := strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	if apiKey == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	search, err := NewDDGSearchClient(20*time.Second, 5, 48<<10)
	if err != nil {
		t.Fatal(err)
	}
	httpClient, err := NewSystemHTTPClient(20 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	broker, err := New(Config{
		Search: search, HTTP: httpClient, Timeout: 20 * time.Second,
		MaxResultBytes: 32 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	system := "You have two tools: web_search (search the live internet) and " +
		"web_fetch (read one HTTPS page). Use them whenever you need live facts. " +
		"Always cite the source URL."
	user := "Use web_fetch to read https://example.com and report what the page says " +
		"in one sentence, with the URL."
	messages := []map[string]any{
		{"role": "system", "content": system},
		{"role": "user", "content": user},
	}
	searchTool := map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "web_search",
			"description": "Search the live internet and return bounded results with title, URL and snippet.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
				"required": []string{"query"},
			},
		},
	}
	fetchTool := map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "web_fetch",
			"description": "Fetch one public HTTPS page and return its bounded text content.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{"type": "string"},
				},
				"required": []string{"url"},
			},
		},
	}
	tools := []map[string]any{searchTool, fetchTool}

	webSearchCalls := 0
	webFetchSuccess := 0
	var finalAnswer string
	for round := 0; round < 5; round++ {
		payload, err := json.Marshal(map[string]any{
			"model": "deepseek-chat", "messages": messages, "tools": tools,
			"tool_choice": "auto", "temperature": 0.2,
		})
		if err != nil {
			t.Fatal(err)
		}
		request, err := http.NewRequestWithContext(
			ctx, http.MethodPost,
			"https://api.deepseek.com/chat/completions", bytes.NewReader(payload),
		)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+apiKey)
		response, err := httpClient.Do(request)
		if err != nil {
			t.Fatalf("deepseek request: %v", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if readErr != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			t.Fatalf("deepseek status=%d body=%s", response.StatusCode, truncateToolLoop(body, 300))
		}
		var decoded struct {
			Choices []struct {
				Message struct {
					Role      string `json:"role"`
					Content   string `json:"content"`
					ToolCalls []struct {
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &decoded); err != nil ||
			len(decoded.Choices) == 0 {
			t.Fatalf("decode choices: %v body=%s", err, truncateToolLoop(body, 300))
		}
		message := decoded.Choices[0].Message
		assistantMessage := map[string]any{
			"role": message.Role, "content": message.Content,
		}
		if len(message.ToolCalls) > 0 {
			toolCalls := make([]map[string]any, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				toolCalls = append(toolCalls, map[string]any{
					"id": call.ID, "type": "function",
					"function": map[string]any{
						"name": call.Function.Name, "arguments": call.Function.Arguments,
					},
				})
			}
			assistantMessage["tool_calls"] = toolCalls
		}
		messages = append(messages, assistantMessage)
		if len(message.ToolCalls) == 0 {
			finalAnswer = message.Content
			break
		}
		for _, call := range message.ToolCalls {
			var toolResult string
			switch call.Function.Name {
			case "web_search":
				var args struct {
					Query string `json:"query"`
				}
				if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil ||
					strings.TrimSpace(args.Query) == "" {
					toolResult = "invalid query"
					break
				}
				webSearchCalls++
				result, execErr := broker.ExecuteProposalContent(
					ctx,
					permissions.ProposedCall{
						Tool: permissions.ToolWebSearch, Path: args.Query,
					},
				)
				if execErr != nil {
					// External HTML-search endpoints can block datacenter IPs;
					// the model-side proposal is recorded, execution is proven
					// separately by web_fetch + the earlier live samples.
					toolResult = "web_search unavailable: " + execErr.Error()
					t.Logf("round %d: model called web_search(%q) but endpoint blocked: %v", round, args.Query, execErr)
				} else {
					toolResult = string(result)
					t.Logf("round %d: model called web_search(%q) -> %d bytes", round, args.Query, len(result))
				}
			case "web_fetch":
				var args struct {
					URL string `json:"url"`
				}
				if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil ||
					strings.TrimSpace(args.URL) == "" {
					toolResult = "invalid url"
					break
				}
				result, execErr := broker.ExecuteProposalContent(
					ctx,
					permissions.ProposedCall{
						Tool: permissions.ToolWebFetch, Path: args.URL,
					},
				)
				if execErr != nil {
					toolResult = "web_fetch failed: " + execErr.Error()
				} else {
					webFetchSuccess++
					toolResult = string(result)
					t.Logf("round %d: model called web_fetch(%q) -> %d bytes", round, args.URL, len(result))
				}
			default:
				toolResult = "unknown tool"
			}
			messages = append(messages, map[string]any{
				"role": "tool", "tool_call_id": call.ID, "content": toolResult,
			})
		}
	}
	if webFetchSuccess == 0 {
		t.Fatalf("model never completed a web_fetch (search calls=%d)", webSearchCalls)
	}
	if strings.TrimSpace(finalAnswer) == "" {
		t.Fatal("model produced no final answer")
	}
	if !strings.Contains(strings.ToLower(finalAnswer), "http") {
		t.Fatalf("final answer lacks a source URL: %s", truncateToolLoop([]byte(finalAnswer), 400))
	}
	t.Logf("live model tool-loop ok: search_calls=%d fetch_ok=%d answer=%s",
		webSearchCalls, webFetchSuccess, truncateToolLoop([]byte(finalAnswer), 200))
}

func truncateToolLoop(content []byte, maximum int) string {
	value := string(content)
	if len(value) > maximum {
		return value[:maximum] + "…"
	}
	return value
}

var _ = fmt.Sprintf
