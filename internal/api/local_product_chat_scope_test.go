package api

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

type recordingConversationScopeManager struct {
	request            LocalProductConversationScopeRequest
	opened             bool
	closed             bool
	err                error
	executionContext   context.Context
	noExecutionContext bool
}

type recordingConversationScopeLease struct {
	manager *recordingConversationScopeManager
}

func (manager *recordingConversationScopeManager) OpenConversationAttempt(
	ctx context.Context,
	request LocalProductConversationScopeRequest,
) (LocalProductConversationScopeLease, error) {
	manager.request = request
	manager.opened = true
	if manager.err != nil {
		return nil, manager.err
	}
	if !manager.noExecutionContext {
		manager.executionContext = context.WithValue(
			ctx, recordingConversationScopeContextKey{}, "attempt-owned",
		)
	}
	return &recordingConversationScopeLease{manager: manager}, nil
}

type recordingConversationScopeContextKey struct{}

func TestCOMP2DChatScopeFailureRollsBackBeforeProviderDispatch(t *testing.T) {
	want := errors.New("scope unavailable")
	manager := &recordingConversationScopeManager{err: want}
	dispatched := false
	chat, err := NewPersistentLocalProductChatAPI(
		filepath.Join(t.TempDir(), "chat.json"), time.Now,
		localProductConversationResponderFunc(func(
			context.Context,
			LocalProductConversationRequest,
		) (LocalProductConversationResponse, error) {
			dispatched = true
			return LocalProductConversationResponse{Content: "must not run"}, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := chat.SetConversationScopeManager(manager); err != nil {
		t.Fatal(err)
	}
	_, err = chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "scope-failure", Content: "hello", ProfileID: "scope-profile",
	})
	if !errors.Is(err, want) || dispatched {
		t.Fatalf("err=%v dispatched=%t", err, dispatched)
	}
	thread, readErr := chat.ChatThread(context.Background(), "scope-failure")
	if readErr != nil || len(thread.Messages) != 0 || len(thread.Attempts) != 0 {
		t.Fatalf("thread=%#v err=%v", thread, readErr)
	}
}

func TestCOMP2DChatRejectsMissingScopeExecutionContextBeforeProviderDispatch(t *testing.T) {
	manager := &recordingConversationScopeManager{noExecutionContext: true}
	dispatched := false
	chat, err := NewPersistentLocalProductChatAPI(
		filepath.Join(t.TempDir(), "chat.json"), time.Now,
		localProductConversationResponderFunc(func(
			context.Context,
			LocalProductConversationRequest,
		) (LocalProductConversationResponse, error) {
			dispatched = true
			return LocalProductConversationResponse{Content: "must not run"}, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := chat.SetConversationScopeManager(manager); err != nil {
		t.Fatal(err)
	}
	_, err = chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "scope-context-missing", Content: "hello", ProfileID: "scope-profile",
	})
	if err == nil || dispatched || !manager.closed {
		t.Fatalf("err=%v dispatched=%t closed=%t", err, dispatched, manager.closed)
	}
	thread, readErr := chat.ChatThread(context.Background(), "scope-context-missing")
	if readErr != nil || len(thread.Messages) != 0 || len(thread.Attempts) != 0 {
		t.Fatalf("thread=%#v err=%v", thread, readErr)
	}
}

func (lease *recordingConversationScopeLease) Close(context.Context) error {
	lease.manager.closed = true
	return nil
}

func (*recordingConversationScopeLease) CompositionSnapshotDigest() string { return "" }
func (*recordingConversationScopeLease) ExecutionBindingDigest() string    { return "" }
func (lease *recordingConversationScopeLease) ExecutionContext() context.Context {
	return lease.manager.executionContext
}

func TestCOMP2DChatOpensFrozenAttemptScopeBeforeDispatchAndClosesIt(t *testing.T) {
	manager := &recordingConversationScopeManager{}
	chat, err := NewPersistentLocalProductChatAPI(
		filepath.Join(t.TempDir(), "chat.json"),
		func() time.Time { return time.Unix(1, 0).UTC() },
		localProductConversationResponderFunc(func(
			ctx context.Context,
			_ LocalProductConversationRequest,
		) (LocalProductConversationResponse, error) {
			if !manager.opened || manager.closed {
				t.Fatal("Provider dispatch ran outside the Attempt scope")
			}
			if got := ctx.Value(recordingConversationScopeContextKey{}); got != "attempt-owned" {
				t.Fatalf("Provider dispatch context marker=%v", got)
			}
			return LocalProductConversationResponse{Content: "scoped reply"}, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := chat.SetConversationScopeManager(manager); err != nil {
		t.Fatal(err)
	}
	thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "scope#thread?token", Content: "hello", ProfileID: "scope-profile",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.Attempts) != 1 || !manager.opened || !manager.closed {
		t.Fatalf("attempts=%#v opened=%t closed=%t", thread.Attempts, manager.opened, manager.closed)
	}
	want := thread.Attempts[0]
	if manager.request.ConversationID != "scope#thread?token" ||
		manager.request.TeamID != "conversation:scope#thread?token" ||
		manager.request.AgentID != "conversation-agent:loom" ||
		manager.request.AttemptID != want.AttemptID ||
		manager.request.TurnID != want.AttemptID+":turn-1" ||
		manager.request.TurnGeneration != 1 ||
		manager.request.ExecutionBindingDigest != want.BindingDigest ||
		len(want.BindingDigest) != 64 {
		t.Fatalf("scope request=%#v attempt=%#v", manager.request, want)
	}
}
