package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidLocalProductChatRequest = errors.New("invalid local product chat request")
	ErrLocalProductChatUnavailable    = errors.New("local product chat unavailable")
)

type LocalProductChatRole string

const (
	ChatRoleUser         LocalProductChatRole = "user"
	ChatRoleLoom         LocalProductChatRole = "loom"
	ChatRoleProposal     LocalProductChatRole = "proposal"
	ChatRoleConfirmation LocalProductChatRole = "confirmation"
)

type LocalProductChatMessage struct {
	MessageID string    `json:"message_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Tentative bool      `json:"tentative"`
	CreatedAt time.Time `json:"created_at"`
}

type LocalProductChatThread struct {
	ThreadID             string                    `json:"thread_id"`
	Messages             []LocalProductChatMessage `json:"messages"`
	CanReply             bool                      `json:"can_reply"`
	RequiresConfirmation bool                      `json:"requires_confirmation"`
}

type LocalProductChatThreadRequest struct {
	ThreadID string `json:"thread_id"`
}

type LocalProductChatMessageRequest struct {
	ThreadID string `json:"thread_id"`
	Content  string `json:"content"`
}

type LocalProductChatAPI struct {
	mu      sync.Mutex
	threads map[string]*LocalProductChatThread
	now     func() time.Time
}

func NewLocalProductChatAPI(now func() time.Time) *LocalProductChatAPI {
	if now == nil {
		now = time.Now
	}
	return &LocalProductChatAPI{
		threads: make(map[string]*LocalProductChatThread),
		now:     now,
	}
}

func (api *LocalProductChatAPI) ChatThread(_ context.Context, threadID string) (LocalProductChatThread, error) {
	if threadID == "" {
		return LocalProductChatThread{}, ErrInvalidLocalProductChatRequest
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	thread, ok := api.threads[threadID]
	if !ok {
		return LocalProductChatThread{
			ThreadID: threadID,
			Messages: []LocalProductChatMessage{},
			CanReply: true,
		}, nil
	}
	return *cloneChatThread(thread), nil
}

func (api *LocalProductChatAPI) SendMessage(_ context.Context, req LocalProductChatMessageRequest) (LocalProductChatThread, error) {
	content := strings.TrimSpace(req.Content)
	if req.ThreadID == "" || content == "" || len(content) > 4096 {
		return LocalProductChatThread{}, ErrInvalidLocalProductChatRequest
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	thread, ok := api.threads[req.ThreadID]
	if !ok {
		thread = &LocalProductChatThread{
			ThreadID: req.ThreadID,
			Messages: []LocalProductChatMessage{},
			CanReply: true,
		}
		api.threads[req.ThreadID] = thread
	}
	userMsg := LocalProductChatMessage{
		MessageID: fmt.Sprintf("msg-%d", len(thread.Messages)+1),
		Role:      string(ChatRoleUser),
		Content:   content,
		Tentative: false,
		CreatedAt: api.now(),
	}
	thread.Messages = append(thread.Messages, userMsg)
	reply := fmt.Sprintf("Loom received: %s", content)
	role := string(ChatRoleLoom)
	tentative := false
	if isExplicitAgentTrigger(content) {
		role = string(ChatRoleProposal)
		reply = "This task looks like it needs an Agent Team. Open the governance panel or press 'u' to start a Team Draft. Nothing is created until you confirm."
		tentative = true
	}
	loomMsg := LocalProductChatMessage{
		MessageID: fmt.Sprintf("msg-%d", len(thread.Messages)+1),
		Role:      role,
		Content:   reply,
		Tentative: tentative,
		CreatedAt: api.now(),
	}
	thread.Messages = append(thread.Messages, loomMsg)
	return *cloneChatThread(thread), nil
}

func cloneChatThread(thread *LocalProductChatThread) *LocalProductChatThread {
	if thread == nil {
		return nil
	}
	copied := *thread
	copied.Messages = append([]LocalProductChatMessage(nil), thread.Messages...)
	return &copied
}

func isExplicitAgentTrigger(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "use agent") ||
		strings.Contains(lower, "agent team") ||
		(strings.Contains(lower, "team") && strings.Contains(lower, "mission"))
}
