package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/localipc"
)

func TestZZCodexProbe(t *testing.T) {
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: "/Users/lune/Library/Application Support/Loom/run/loomd.sock",
		Timeout:    60 * time.Second,
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var result json.RawMessage
	sendCtx, sendCancel := context.WithTimeout(ctx, 60*time.Second)
	defer sendCancel()
	err = client.Call(sendCtx, "chat_message", api.LocalProductChatMessageRequest{
		ThreadID:   "thread-live-codex-probe-" + time.Now().UTC().Format("20060102T150405Z"),
		Content:    "Reply with exactly: CODEX-OK",
		ProfileID:  "conversation-openai-codex-default-v1",
		ModelID:    "codex-default",
		ContextMode: api.ContextModeStartClean,
		ExpectedExecutionBinding: &api.LocalProductConversationExecutionBinding{
			SchemaVersion: 3, ProviderID: "openai",
		},
		IncidentID: "loom-chat-codex-probe-" + time.Now().UTC().Format("20060102T150405Z"),
	}, &result)
	if err != nil {
		var remote *localipc.RemoteError
		if errors.As(err, &remote) {
			t.Fatalf("codex chat error: code=%s stage=%s recoverable=%v", remote.Code, remote.Stage, remote.Recoverable)
		}
		t.Fatalf("codex chat error: %+v", err)
	}
	var thread struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	_ = json.Unmarshal(result, &thread)
	if len(thread.Messages) > 0 {
		last := thread.Messages[len(thread.Messages)-1]
		t.Logf("last message role=%s content=%q", last.Role, last.Content)
	}
}
