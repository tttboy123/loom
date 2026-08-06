package api

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestChatAPIPlainMessageEchoesAndDoesNotRequireConfirmation(t *testing.T) {
	api := NewLocalProductChatAPI(func() time.Time { return time.Unix(0, 0).UTC() })
	ctx := context.Background()
	thread, err := api.SendMessage(ctx, LocalProductChatMessageRequest{ThreadID: "t1", Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(thread.Messages))
	}
	if thread.Messages[0].Role != string(ChatRoleUser) {
		t.Fatalf("expected user role, got %s", thread.Messages[0].Role)
	}
	if thread.Messages[1].Role != string(ChatRoleLoom) {
		t.Fatalf("expected loom role, got %s", thread.Messages[1].Role)
	}
	if thread.RequiresConfirmation {
		t.Fatal("plain chat must not require confirmation")
	}
}

func TestChatAPIExplicitAgentTriggerReturnsTentativeProposal(t *testing.T) {
	api := NewLocalProductChatAPI(func() time.Time { return time.Unix(0, 0).UTC() })
	ctx := context.Background()
	thread, err := api.SendMessage(ctx, LocalProductChatMessageRequest{ThreadID: "t2", Content: "use agent team for this mission"})
	if err != nil {
		t.Fatal(err)
	}
	if thread.Messages[1].Role != string(ChatRoleProposal) {
		t.Fatalf("expected proposal role, got %s", thread.Messages[1].Role)
	}
	if !thread.Messages[1].Tentative {
		t.Fatal("proposal must be marked tentative")
	}
	if !strings.Contains(thread.Messages[1].Content, "Agent Team") {
		t.Fatalf("proposal should mention Agent Team: %s", thread.Messages[1].Content)
	}
}

func TestChatAPIEmptyContentRejected(t *testing.T) {
	api := NewLocalProductChatAPI(func() time.Time { return time.Unix(0, 0).UTC() })
	ctx := context.Background()
	_, err := api.SendMessage(ctx, LocalProductChatMessageRequest{ThreadID: "t3", Content: "   "})
	if err != ErrInvalidLocalProductChatRequest {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestChatAPIThreadIsolatedPerThreadID(t *testing.T) {
	api := NewLocalProductChatAPI(func() time.Time { return time.Unix(0, 0).UTC() })
	ctx := context.Background()
	_, _ = api.SendMessage(ctx, LocalProductChatMessageRequest{ThreadID: "a", Content: "msg a"})
	thread, err := api.ChatThread(ctx, "b")
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.Messages) != 0 {
		t.Fatalf("thread b should be empty, got %d messages", len(thread.Messages))
	}
}
