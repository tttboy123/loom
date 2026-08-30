package api

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/controltool"
)

func TestConversationControlProposalRequiresUserConfirmationBeforeAlignment(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	responder := &conversationControlResponder{chat: chat}
	chat.responder = responder

	for _, source := range []struct {
		id      string
		content string
	}{
		{id: "s1", content: "Authoritative constraint from session one"},
		{id: "s2", content: "Authoritative decision from session two"},
	} {
		if _, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
			ThreadID: source.id, Content: source.content, ProfileID: "codex",
		}); err != nil {
			t.Fatalf("seed %s: %v", source.id, err)
		}
	}
	catalog := []controltool.SessionReference{
		{ConversationID: "s1", Title: "Session one", UpdatedAt: now.Add(-2 * time.Minute)},
		{ConversationID: "s2", Title: "Session two", UpdatedAt: now.Add(-time.Minute)},
		{ConversationID: "s3", Title: "Current session", UpdatedAt: now},
	}
	thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "s3", Content: "Align s1 and s2 here", ProfileID: "codex",
		IncidentID: "incident-control-1", SessionCatalog: catalog,
	})
	if err != nil {
		t.Fatalf("proposal SendMessage() error = %v", err)
	}
	if len(thread.ControlProposals) != 1 {
		t.Fatalf("control proposals = %#v", thread.ControlProposals)
	}
	proposal := thread.ControlProposals[0]
	if !proposal.Valid() || proposal.Status != controltool.ProposalPending ||
		proposal.MessageID == "" || !thread.RequiresConfirmation ||
		len(thread.ContextAlignments) != 0 || len(thread.Segments) != 1 {
		t.Fatalf("pending thread = %#v", thread)
	}
	if len(responder.requests) != 3 || responder.requests[2].CatalogDigest == "" ||
		len(responder.requests[2].SessionCatalog) != 3 {
		t.Fatalf("frozen responder request = %#v", responder.requests)
	}

	confirmed, err := chat.DecideControlProposal(
		context.Background(),
		LocalProductChatControlDecisionRequest{
			ThreadID: "s3", ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-confirm-1",
		},
	)
	if err != nil {
		t.Fatalf("DecideControlProposal(confirm) error = %v", err)
	}
	if confirmed.ControlProposals[0].Status != controltool.ProposalConfirmed ||
		len(confirmed.ContextAlignments) != 1 || confirmed.RequiresConfirmation ||
		len(confirmed.ProposalDecisionReceipts) != 1 {
		t.Fatalf("confirmed thread = %#v", confirmed)
	}
	decisionReceipt := confirmed.ProposalDecisionReceipts[0]
	if !decisionReceipt.Valid() ||
		!decisionReceipt.MatchesSessionAlignmentProposal(confirmed.ControlProposals[0]) ||
		decisionReceipt.Decision != controltool.ProposalDecisionConfirm ||
		decisionReceipt.DecisionIncidentID != "incident-confirm-1" {
		t.Fatalf("alignment decision receipt = %#v", decisionReceipt)
	}
	alignment := confirmed.ContextAlignments[0]
	if alignment.AppliedSegmentID != "" || alignment.ReceiptDigest == "" {
		t.Fatalf("confirmed alignment = %#v", alignment)
	}

	now = now.Add(time.Minute)
	aligned, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "s3", Content: "Continue with the aligned context", ProfileID: "codex",
		IncidentID: "incident-control-2", SessionCatalog: catalog,
	})
	if err != nil {
		t.Fatalf("aligned SendMessage() error = %v", err)
	}
	if len(aligned.Segments) != 2 || aligned.Segments[1].ContextAlignmentDigest == "" ||
		aligned.ContextAlignments[0].AppliedSegmentID != aligned.Segments[1].SegmentID {
		t.Fatalf("aligned thread = %#v", aligned)
	}
	request := responder.requests[len(responder.requests)-1]
	if request.ContextAlignmentDigest != alignment.ReceiptDigest || len(request.Messages) < 2 {
		t.Fatalf("aligned request = %#v", request)
	}
	combined := ""
	for _, message := range request.Messages {
		combined += message.Content
	}
	if !strings.Contains(combined, "Authoritative constraint from session one") ||
		!strings.Contains(combined, "Authoritative decision from session two") ||
		strings.Contains(combined, "reply for s1") || strings.Contains(combined, "reply for s2") {
		t.Fatalf("summary-only aligned context = %q", combined)
	}
	if _, err := chat.DecideControlProposal(
		context.Background(),
		LocalProductChatControlDecisionRequest{
			ThreadID: "s3", ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-confirm-replay",
		},
	); !errors.Is(err, ErrLocalProductChatControlConflict) {
		t.Fatalf("replayed confirmation error = %v", err)
	}
}

func TestConversationAlignmentExpiryAddsConfirmationMessage(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	responder := &conversationControlResponder{chat: chat}
	chat.responder = responder
	for _, threadID := range []string{"s1", "s2"} {
		if _, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
			ThreadID: threadID, Content: "Source " + threadID, ProfileID: "codex",
		}); err != nil {
			t.Fatal(err)
		}
	}
	thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "s3", Content: "Align s1 and s2 here", ProfileID: "codex",
		IncidentID: "incident-alignment-expiry-create",
		SessionCatalog: []controltool.SessionReference{
			{ConversationID: "s1", Title: "Session one", UpdatedAt: now},
			{ConversationID: "s2", Title: "Session two", UpdatedAt: now},
			{ConversationID: "s3", Title: "Current session", UpdatedAt: now},
		},
	})
	if err != nil || len(thread.ControlProposals) != 1 {
		t.Fatalf("pending thread=%#v error=%v", thread, err)
	}
	proposal := thread.ControlProposals[0]
	if _, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "s1", Content: "Source s1 changed after review", ProfileID: "codex",
	}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(6 * time.Minute)
	decided, err := chat.DecideControlProposal(
		context.Background(), LocalProductChatControlDecisionRequest{
			ThreadID: thread.ThreadID, ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-alignment-expiry-decision",
		},
	)
	if err != nil || decided.ControlProposals[0].Status != controltool.ProposalExpired ||
		len(decided.ContextAlignments) != 0 ||
		len(decided.Messages) != len(thread.Messages)+1 ||
		decided.Messages[len(decided.Messages)-1].Role != string(ChatRoleConfirmation) ||
		len(decided.ProposalDecisionReceipts) != 1 ||
		decided.ProposalDecisionReceipts[0].Decision != controltool.ProposalDecisionExpire {
		t.Fatalf("expired alignment thread=%#v error=%v", decided, err)
	}
}

func TestConversationAttemptPersistsExactCompletedControlToolsAndRejectsDrift(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	chat.responder = localProductConversationResponderFunc(func(
		context.Context,
		LocalProductConversationRequest,
	) (LocalProductConversationResponse, error) {
		return LocalProductConversationResponse{
			Content: "Runtime and Provider status checked.", Tentative: true,
			CompletedControlTools: []controltool.CompletedCall{
				{ToolID: controltool.ToolRuntimesStatus, ToolVersion: 1, Effect: controltool.EffectRead},
				{ToolID: controltool.ToolProvidersStatus, ToolVersion: 1, Effect: controltool.EffectRead},
			},
		}, nil
	})
	thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "conversation-tool-audit", Content: "Check Loom status",
		ProfileID: "codex", IncidentID: "incident-tool-audit-1",
	})
	if err != nil || len(thread.Attempts) != 1 || thread.Attempts[0].Status != "succeeded" ||
		len(thread.Attempts[0].CompletedControlTools) != 2 ||
		thread.Attempts[0].CompletedControlTools[0].ToolID != controltool.ToolRuntimesStatus ||
		thread.Messages[len(thread.Messages)-1].AttemptID != thread.Attempts[0].AttemptID ||
		validateStoredChatThread(thread) != nil {
		t.Fatalf("completed tool attempt = %#v, error = %v", thread.Attempts, err)
	}
	tampered := thread
	tampered.Attempts = append([]LocalProductConversationAttempt(nil), thread.Attempts...)
	tampered.Attempts[0].CompletedControlTools = controltool.CloneCompletedCalls(
		thread.Attempts[0].CompletedControlTools,
	)
	tampered.Attempts[0].CompletedControlTools[0].Effect = controltool.EffectProposal
	if validateStoredChatThread(tampered) == nil {
		t.Fatal("stored completed tool effect substitution was accepted")
	}
	tampered = thread
	tampered.Messages = append([]LocalProductChatMessage(nil), thread.Messages...)
	tampered.Messages[len(tampered.Messages)-1].AttemptID = "attempt-unknown"
	if validateStoredChatThread(tampered) == nil {
		t.Fatal("reply-to-Attempt substitution was accepted")
	}
	authoritative, err := chat.ChatThread(context.Background(), thread.ThreadID)
	if err != nil || authoritative.Attempts[0].CompletedControlTools[0].Effect !=
		controltool.EffectRead {
		t.Fatalf("authoritative completed tools mutated: %#v, %v", authoritative, err)
	}
}

func TestEncryptedConversationRestartRestoresCompletedControlToolAudit(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	root := privateLocalProductChatTestRoot(t)
	legacyPath := filepath.Join(root, "chat-threads.json")
	documents := newMemoryLocalProductChatDocumentStore()
	responder := localProductConversationResponderFunc(func(
		context.Context,
		LocalProductConversationRequest,
	) (LocalProductConversationResponse, error) {
		return LocalProductConversationResponse{
			Content: "Workspace status checked.", Tentative: true,
			CompletedControlTools: []controltool.CompletedCall{{
				ToolID:      controltool.ToolWorkspaceStatus,
				ToolVersion: 1,
				Effect:      controltool.EffectRead,
			}},
		}, nil
	})
	first, err := NewEncryptedPersistentLocalProductChatAPI(
		context.Background(), legacyPath, documents, func() time.Time { return now },
		responder,
	)
	if err != nil {
		t.Fatal(err)
	}
	setPhase7ControlTestBindingResolver(first)
	if err := first.SetControlWorkspace(phase7ControlTestWorkspace()); err != nil {
		t.Fatal(err)
	}
	created, err := first.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "conversation-tool-audit-restart", Content: "Check workspace",
		ProfileID: "codex", IncidentID: "incident-tool-audit-restart-1",
	})
	if err != nil || len(created.Attempts) != 1 ||
		len(created.Attempts[0].CompletedControlTools) != 1 {
		t.Fatalf("created attempt = %#v, error = %v", created.Attempts, err)
	}
	restarted, err := NewEncryptedPersistentLocalProductChatAPI(
		context.Background(), legacyPath, documents, func() time.Time { return now }, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := restarted.ChatThread(
		context.Background(), "conversation-tool-audit-restart",
	)
	if err != nil || len(restored.Attempts) != 1 ||
		len(restored.Attempts[0].CompletedControlTools) != 1 ||
		restored.Attempts[0].CompletedControlTools[0] != (controltool.CompletedCall{
			ToolID:      controltool.ToolWorkspaceStatus,
			ToolVersion: 1,
			Effect:      controltool.EffectRead,
		}) || validateStoredChatThread(restored) != nil {
		t.Fatalf("restored audit = %#v, error = %v", restored.Attempts, err)
	}
}

func TestConversationAttemptFailsClosedOnInvalidCompletedControlToolMetadata(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	chat.responder = localProductConversationResponderFunc(func(
		context.Context,
		LocalProductConversationRequest,
	) (LocalProductConversationResponse, error) {
		return LocalProductConversationResponse{
			Content: "This response must not be accepted.", Tentative: true,
			CompletedControlTools: []controltool.CompletedCall{{
				ToolID:      controltool.ToolRuntimesStatus,
				ToolVersion: 2,
				Effect:      controltool.EffectRead,
			}},
		}, nil
	})
	thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "conversation-tool-audit-invalid", Content: "Check Loom status",
		ProfileID: "codex", IncidentID: "incident-tool-audit-invalid-1",
	})
	if err != nil || len(thread.Attempts) != 1 || thread.Attempts[0].Status != "failed" ||
		thread.Attempts[0].FailureCode != "invalid_response" ||
		len(thread.Attempts[0].CompletedControlTools) != 0 ||
		strings.Contains(thread.Messages[len(thread.Messages)-1].Content, "must not be accepted") {
		t.Fatalf("invalid completed tool response = %#v, error = %v", thread, err)
	}
}

func TestConversationControlConfirmationRejectsSourceDrift(t *testing.T) {
	now := time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	responder := &conversationControlResponder{chat: chat}
	chat.responder = responder
	for _, request := range []LocalProductChatMessageRequest{
		{ThreadID: "s1", Content: "original source", ProfileID: "codex"},
		{
			ThreadID: "s3", Content: "Align s1 here", ProfileID: "codex",
			IncidentID: "incident-control-drift",
			SessionCatalog: []controltool.SessionReference{
				{ConversationID: "s1", Title: "Source", UpdatedAt: now},
				{ConversationID: "s3", Title: "Target", UpdatedAt: now},
			},
		},
	} {
		if _, err := chat.SendMessage(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	proposal := mustChatThread(t, chat, "s3").ControlProposals[0]
	now = now.Add(time.Second)
	if _, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "s1", Content: "source changed", ProfileID: "codex",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := chat.DecideControlProposal(
		context.Background(),
		LocalProductChatControlDecisionRequest{
			ThreadID: "s3", ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-confirm-drift",
		},
	); !errors.Is(err, ErrLocalProductChatControlConflict) {
		t.Fatalf("source drift confirmation error = %v", err)
	}
	cancelled, err := chat.DecideControlProposal(
		context.Background(), LocalProductChatControlDecisionRequest{
			ThreadID: "s3", ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionCancel,
			IncidentID: "incident-cancel-after-drift",
		},
	)
	if err != nil || cancelled.ControlProposals[0].Status != controltool.ProposalCancelled ||
		cancelled.RequiresConfirmation || len(cancelled.ProposalDecisionReceipts) != 1 ||
		cancelled.ProposalDecisionReceipts[0].Decision != controltool.ProposalDecisionCancel {
		t.Fatalf("source drift cancellation thread=%#v error=%v", cancelled, err)
	}
}

func TestConversationActionProposalRequiresUserConfirmationWithoutCreatingProductState(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	chat.responder = &conversationActionControlResponder{chat: chat}
	thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "action-thread", Content: "Plan a governed release mission",
		ProfileID: "codex", IncidentID: "incident-action-1",
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if len(thread.ActionProposals) != 1 || len(thread.ControlProposals) != 0 ||
		!thread.RequiresConfirmation {
		t.Fatalf("pending thread = %#v", thread)
	}
	proposal := thread.ActionProposals[0]
	if !proposal.Valid() || proposal.Status != controltool.ProposalPending ||
		proposal.Action != controltool.ConversationActionMission || proposal.MessageID == "" ||
		proposal.Argument != "Ship the governed release" {
		t.Fatalf("action proposal = %#v", proposal)
	}
	confirmed, err := chat.DecideControlProposal(
		context.Background(), LocalProductChatControlDecisionRequest{
			ThreadID: thread.ThreadID, ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-action-confirm-1",
		},
	)
	if err != nil {
		t.Fatalf("DecideControlProposal(confirm) error = %v", err)
	}
	if confirmed.ActionProposals[0].Status != controltool.ProposalConfirmed ||
		len(confirmed.ContextAlignments) != 0 || confirmed.RequiresConfirmation ||
		len(confirmed.ProposalDecisionReceipts) != 1 {
		t.Fatalf("confirmed action thread = %#v", confirmed)
	}
	receipt := confirmed.ProposalDecisionReceipts[0]
	if !receipt.Valid() ||
		!receipt.MatchesConversationActionProposal(confirmed.ActionProposals[0]) ||
		receipt.Decision != controltool.ProposalDecisionConfirm ||
		receipt.DecisionIncidentID != "incident-action-confirm-1" {
		t.Fatalf("confirmation receipt = %#v", receipt)
	}
	if _, err := chat.DecideControlProposal(
		context.Background(), LocalProductChatControlDecisionRequest{
			ThreadID: thread.ThreadID, ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-action-replay",
		},
	); !errors.Is(err, ErrLocalProductChatControlConflict) {
		t.Fatalf("replayed action confirmation error = %v", err)
	}
	afterReplay := mustChatThread(t, chat, thread.ThreadID)
	if len(afterReplay.ProposalDecisionReceipts) != 1 {
		t.Fatalf("replay created another receipt: %#v", afterReplay.ProposalDecisionReceipts)
	}
}

func TestConversationRoundTableSeatProposalFreezesBindingAndRejectsConfirmationDrift(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 35, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	resolver := &mutableRoundTableActionTargetResolver{
		target: LocalProductConversationRoundTableTargetBinding{
			MembershipRevision: 4,
			SeatBindingDigest:  controltool.DigestBytes([]byte("seat-binding-4")),
		},
	}
	chat.controlActionTargets = resolver
	chat.responder = &conversationGovernanceActionResponder{
		chat: chat, toolID: controltool.ToolRoundtablesSkipPreview,
		arguments: json.RawMessage(
			`{"session_id":"roundtable-1","round_id":"round-1","seat_id":"seat-coder"}`,
		),
	}
	thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "roundtable-binding-thread", Content: "Prepare the exact seat skip for review",
		ProfileID: "codex", IncidentID: "incident-roundtable-binding-1",
	})
	if err != nil || len(thread.ActionProposals) != 1 {
		t.Fatalf("pending RoundTable Proposal = %#v, error = %v", thread, err)
	}
	proposal := thread.ActionProposals[0]
	if proposal.Payload == nil || !proposal.Payload.ValidFrozenRoundTableSeatBinding() ||
		proposal.Payload.MembershipRevision != resolver.target.MembershipRevision ||
		proposal.Payload.SeatBindingDigest != resolver.target.SeatBindingDigest {
		t.Fatalf("frozen RoundTable target = %#v", proposal.Payload)
	}
	frozen := resolver.target
	resolver.target = LocalProductConversationRoundTableTargetBinding{
		MembershipRevision: frozen.MembershipRevision + 1,
		SeatBindingDigest:  controltool.DigestBytes([]byte("seat-binding-5")),
	}
	if _, err := chat.DecideControlProposal(
		context.Background(), LocalProductChatControlDecisionRequest{
			ThreadID: thread.ThreadID, ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-roundtable-binding-drift",
		},
	); !errors.Is(err, ErrLocalProductChatControlConflict) {
		t.Fatalf("drifted RoundTable confirmation error = %v", err)
	}
	unchanged := mustChatThread(t, chat, thread.ThreadID)
	if unchanged.ActionProposals[0].Status != controltool.ProposalPending ||
		len(unchanged.ProposalDecisionReceipts) != 0 {
		t.Fatalf("drifted confirmation changed authority: %#v", unchanged)
	}
	resolver.target = frozen
	confirmed, err := chat.DecideControlProposal(
		context.Background(), LocalProductChatControlDecisionRequest{
			ThreadID: thread.ThreadID, ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-roundtable-binding-confirm",
		},
	)
	if err != nil || confirmed.ActionProposals[0].Status != controltool.ProposalConfirmed ||
		len(confirmed.ProposalDecisionReceipts) != 1 {
		t.Fatalf("exact RoundTable confirmation = %#v, error = %v", confirmed, err)
	}
}

func TestConversationRoundTablePauseProposalRequiresCurrentRound(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 38, 0, 0, time.UTC)
	for _, test := range []struct {
		name    string
		err     error
		wantErr bool
	}{
		{name: "historical", err: controltool.ErrInvalidCall, wantErr: true},
		{name: "current"},
	} {
		t.Run(test.name, func(t *testing.T) {
			chat := newPhase7ControlTestChat(func() time.Time { return now })
			resolver := &mutableRoundTableActionTargetResolver{err: test.err}
			chat.controlActionTargets = resolver
			chat.responder = &conversationGovernanceActionResponder{
				chat: chat, toolID: controltool.ToolRoundtablesPausePreview,
				arguments: json.RawMessage(
					`{"session_id":"roundtable-1","round_id":"round-1"}`,
				),
			}
			thread, err := chat.SendMessage(
				context.Background(), LocalProductChatMessageRequest{
					ThreadID:  "roundtable-pause-" + test.name,
					Content:   "Prepare the active RoundTable pause for review",
					ProfileID: "codex", IncidentID: "incident-roundtable-pause-" + test.name,
				},
			)
			if test.wantErr {
				if err != nil || len(thread.ActionProposals) != 0 || resolver.validateCalls != 1 {
					t.Fatalf(
						"historical pause result = %#v, calls = %d, error = %v",
						thread.ActionProposals, resolver.validateCalls, err,
					)
				}
				return
			}
			if err != nil || len(thread.ActionProposals) != 1 ||
				thread.ActionProposals[0].ToolID != controltool.ToolRoundtablesPausePreview ||
				resolver.validateCalls != 1 {
				t.Fatalf("current pause proposal = %#v, error = %v", thread.ActionProposals, err)
			}
			proposal := thread.ActionProposals[0]
			resolver.err = controltool.ErrInvalidCall
			if _, err := chat.DecideControlProposal(
				context.Background(), LocalProductChatControlDecisionRequest{
					ThreadID: thread.ThreadID, ProposalID: proposal.ProposalID,
					ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
					IncidentID: "incident-roundtable-pause-drift",
				},
			); !errors.Is(err, ErrLocalProductChatControlConflict) {
				t.Fatalf("historical pause confirmation error = %v", err)
			}
			unchanged := mustChatThread(t, chat, thread.ThreadID)
			if unchanged.ActionProposals[0].Status != controltool.ProposalPending ||
				len(unchanged.ProposalDecisionReceipts) != 0 || resolver.validateCalls != 2 {
				t.Fatalf("historical pause confirmation changed state: %#v", unchanged)
			}
		})
	}
}

func TestConversationRoundTableSeatPayloadParityAcrossFiveRuntimeBindings(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 40, 0, 0, time.UTC)
	target := LocalProductConversationRoundTableTargetBinding{
		MembershipRevision: 9,
		SeatBindingDigest:  controltool.DigestBytes([]byte("seat-binding-9")),
	}
	runtimes := []struct {
		harness  string
		provider string
		model    string
	}{
		{harness: "codex", provider: "openai", model: "gpt-5.6-sol"},
		{harness: "opencode", provider: "deepseek", model: "deepseek/deepseek-chat"},
		{harness: "claude-code", provider: "anthropic", model: "claude-sonnet-4-5"},
		{harness: "pi", provider: "loom-local", model: "qwen2.5-coder"},
		{harness: "loom-native", provider: "minimax", model: "MiniMax-M2.5"},
	}
	var canonical *controltool.ConversationActionPayload
	for _, runtime := range runtimes {
		t.Run(runtime.harness, func(t *testing.T) {
			chat := newPhase7ControlTestChat(func() time.Time { return now })
			chat.bindingResolver = localProductConversationBindingResolverFunc(func(
				context.Context,
				string,
			) (LocalProductConversationExecutionBinding, error) {
				return LocalProductConversationExecutionBinding{
					SchemaVersion: 4, HarnessAdapter: runtime.harness,
					ProviderID: runtime.provider, ModelID: runtime.model,
				}, nil
			})
			chat.controlActionTargets = &mutableRoundTableActionTargetResolver{
				target: target,
			}
			chat.responder = &conversationGovernanceActionResponder{
				chat: chat, toolID: controltool.ToolRoundtablesSkipPreview,
				arguments: json.RawMessage(
					"{\"session_id\":\"session+本地\",\"round_id\":\"round#评审\",\"seat_id\":\"seat+c++/评审\"}",
				),
			}
			thread, err := chat.SendMessage(
				context.Background(), LocalProductChatMessageRequest{
					ThreadID:   "roundtable-parity-" + runtime.harness,
					Content:    "Prepare the exact RoundTable seat skip for review",
					ProfileID:  runtime.harness,
					IncidentID: "incident-roundtable-parity-" + runtime.harness,
				},
			)
			if err != nil || len(thread.ActionProposals) != 1 {
				t.Fatalf("thread = %#v, error = %v", thread, err)
			}
			proposal := thread.ActionProposals[0]
			if proposal.ToolID != controltool.ToolRoundtablesSkipPreview ||
				proposal.Payload == nil ||
				!proposal.Payload.ValidFrozenRoundTableSeatBinding() {
				t.Fatalf("proposal = %#v", proposal)
			}
			if canonical == nil {
				canonical = proposal.Payload
				return
			}
			if *proposal.Payload != *canonical {
				t.Fatalf(
					"%s payload = %#v, want %#v",
					runtime.harness, proposal.Payload, canonical,
				)
			}
		})
	}
}

func TestConversationControlGatewayRejectsDuplicateProposalArgumentKeys(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	var gatewayErr error
	chat.responder = localProductConversationResponderFunc(func(
		ctx context.Context,
		request LocalProductConversationRequest,
	) (LocalProductConversationResponse, error) {
		_, gatewayErr = chat.Call(ctx, phase7ControlTurnForRequest(request), controltool.Call{
			ToolID: controltool.ToolMissionsContinuePreview,
			Arguments: json.RawMessage(
				`{"mission_id":"mission-approved","mission_id":"mission-substituted","guidance":"Continue safely"}`,
			),
		})
		return LocalProductConversationResponse{
			Content: "The ambiguous Proposal was rejected.", Tentative: true,
		}, nil
	})
	thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "duplicate-proposal-arguments", Content: "Continue the approved Mission",
		ProfileID: "codex", IncidentID: "incident-duplicate-proposal-arguments",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(gatewayErr, controltool.ErrInvalidCall) ||
		len(thread.ActionProposals) != 0 || len(thread.ControlProposals) != 0 {
		t.Fatalf("gateway error = %v, thread = %#v", gatewayErr, thread)
	}
}

func TestConversationActionDecisionReceiptsCoverCancelExpiryAndSupersede(t *testing.T) {
	base := time.Date(2026, 8, 28, 14, 30, 0, 0, time.UTC)
	for _, test := range []struct {
		name         string
		decision     LocalProductChatControlDecision
		advance      time.Duration
		wantStatus   controltool.ProposalStatus
		wantDecision controltool.ProposalDecision
	}{
		{
			name: "cancel", decision: ControlDecisionCancel,
			wantStatus:   controltool.ProposalCancelled,
			wantDecision: controltool.ProposalDecisionCancel,
		},
		{
			name: "expire", decision: ControlDecisionConfirm,
			advance: 6 * time.Minute, wantStatus: controltool.ProposalExpired,
			wantDecision: controltool.ProposalDecisionExpire,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := base
			chat := newPhase7ControlTestChat(func() time.Time { return now })
			chat.responder = &conversationActionControlResponder{chat: chat}
			thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
				ThreadID: "action-decision-" + test.name,
				Content:  "Plan a governed Mission", ProfileID: "codex",
				IncidentID: "incident-action-decision-" + test.name,
			})
			if err != nil || len(thread.ActionProposals) != 1 {
				t.Fatalf("pending thread=%#v error=%v", thread, err)
			}
			proposal := thread.ActionProposals[0]
			now = now.Add(test.advance)
			decided, err := chat.DecideControlProposal(
				context.Background(), LocalProductChatControlDecisionRequest{
					ThreadID: thread.ThreadID, ProposalID: proposal.ProposalID,
					ProposalDigest: proposal.ProposalDigest, Decision: test.decision,
					IncidentID: "incident-action-decision-result-" + test.name,
				},
			)
			if err != nil || decided.ActionProposals[0].Status != test.wantStatus ||
				len(decided.ProposalDecisionReceipts) != 1 ||
				len(decided.Messages) != len(thread.Messages)+1 ||
				decided.Messages[len(decided.Messages)-1].Role != string(ChatRoleConfirmation) {
				t.Fatalf("decided thread=%#v error=%v", decided, err)
			}
			receipt := decided.ProposalDecisionReceipts[0]
			if !receipt.Valid() || receipt.Decision != test.wantDecision ||
				!receipt.MatchesConversationActionProposal(decided.ActionProposals[0]) {
				t.Fatalf("decision receipt=%#v", receipt)
			}
		})
	}

	now := base
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	chat.responder = &conversationActionControlResponder{chat: chat}
	first, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "action-decision-supersede", Content: "Plan Mission one",
		ProfileID: "codex", IncidentID: "incident-action-supersede-1",
	})
	if err != nil || len(first.ActionProposals) != 1 {
		t.Fatalf("first thread=%#v error=%v", first, err)
	}
	second, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: first.ThreadID, Content: "Plan Mission two",
		ProfileID: "codex", IncidentID: "incident-action-supersede-2",
	})
	if err != nil || len(second.ActionProposals) != 2 ||
		second.ActionProposals[0].Status != controltool.ProposalCancelled ||
		second.ActionProposals[1].Status != controltool.ProposalPending ||
		len(second.ProposalDecisionReceipts) != 1 ||
		second.ProposalDecisionReceipts[0].Decision != controltool.ProposalDecisionSupersede ||
		second.ProposalDecisionReceipts[0].DecisionIncidentID != "incident-action-supersede-2" {
		t.Fatalf("superseded thread=%#v error=%v", second, err)
	}
}

func TestConversationGovernanceActionProposalFreezesTypedPayloadAndDeepCopies(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 0, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	chat.responder = &conversationGovernanceActionResponder{
		chat: chat, toolID: controltool.ToolConversationRouteChangePreview,
		arguments: json.RawMessage(`{"profile_id":"conversation-deepseek-r4"}`),
	}
	thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "governance-action-thread", Content: "Use the DeepSeek route",
		ProfileID: "codex", IncidentID: "incident-governance-action-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.ActionProposals) != 1 || !thread.RequiresConfirmation {
		t.Fatalf("thread = %#v", thread)
	}
	proposal := thread.ActionProposals[0]
	if proposal.Action != controltool.ConversationActionRoute ||
		proposal.Payload == nil || proposal.Payload.ProfileID != "conversation-deepseek-r4" ||
		proposal.Argument != "" || !proposal.Valid() {
		t.Fatalf("proposal = %#v", proposal)
	}
	thread.ActionProposals[0].Payload.ProfileID = "conversation-tampered-r1"
	authoritative, err := chat.ChatThread(context.Background(), thread.ThreadID)
	if err != nil {
		t.Fatal(err)
	}
	if authoritative.ActionProposals[0].Payload == nil ||
		authoritative.ActionProposals[0].Payload.ProfileID != "conversation-deepseek-r4" {
		t.Fatalf("returned payload mutated authority: %#v", authoritative.ActionProposals)
	}
	confirmed, err := chat.DecideControlProposal(
		context.Background(), LocalProductChatControlDecisionRequest{
			ThreadID: authoritative.ThreadID, ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-governance-action-confirm-1",
		},
	)
	if err != nil || confirmed.ActionProposals[0].Status != controltool.ProposalConfirmed {
		t.Fatalf("confirmed = %#v, error = %v", confirmed, err)
	}
}

func TestConversationMissionActionsFreezeExactMissionAndTurnAuthority(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 30, 0, 0, time.UTC)
	for _, test := range []struct {
		name      string
		toolID    controltool.ToolID
		arguments json.RawMessage
		action    controltool.ConversationAction
	}{
		{
			name: "continue", toolID: controltool.ToolMissionsContinuePreview,
			arguments: json.RawMessage(`{"mission_id":"mission-release-7","guidance":"Retry verification with the accepted constraints"}`),
			action:    controltool.ConversationActionContinueMission,
		},
		{
			name: "roundtable", toolID: controltool.ToolRoundtablesOpenPreview,
			arguments: json.RawMessage(`{"mission_id":"mission-release-7"}`),
			action:    controltool.ConversationActionRoundTable,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			chat := newPhase7ControlTestChat(func() time.Time { return now })
			chat.responder = &conversationGovernanceActionResponder{
				chat: chat, toolID: test.toolID, arguments: test.arguments,
			}
			thread, err := chat.SendMessage(
				context.Background(), LocalProductChatMessageRequest{
					ThreadID: "mission-target-" + test.name,
					Content:  "Prepare the exact governed action", ProfileID: "codex",
					IncidentID: "incident-mission-target-" + test.name,
				},
			)
			if err != nil || len(thread.ActionProposals) != 1 {
				t.Fatalf("thread=%#v error=%v", thread, err)
			}
			proposal := thread.ActionProposals[0]
			if proposal.SchemaVersion != 2 || proposal.Action != test.action ||
				proposal.Payload == nil || proposal.Payload.MissionID != "mission-release-7" ||
				proposal.Route == nil || proposal.Workspace == nil ||
				proposal.RegistryDigest != localProductBuiltinControlRegistryDigest() ||
				proposal.IncidentID != "incident-mission-target-"+test.name ||
				!proposal.Valid() {
				t.Fatalf("proposal=%#v", proposal)
			}
			thread.ActionProposals[0].Payload.MissionID = "mission-tampered"
			thread.ActionProposals[0].Route.ModelID = "model-tampered"
			authoritative, readErr := chat.ChatThread(context.Background(), thread.ThreadID)
			if readErr != nil || authoritative.ActionProposals[0].Payload.MissionID != "mission-release-7" ||
				authoritative.ActionProposals[0].Route.ModelID != "gpt-test" {
				t.Fatalf("authority mutated=%#v error=%v", authoritative, readErr)
			}
		})
	}
}

func TestConversationActionProposalRejectsForgedFrozenRoute(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 45, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	chat.responder = &conversationForgedRouteActionResponder{chat: chat}
	thread, err := chat.SendMessage(
		context.Background(), LocalProductChatMessageRequest{
			ThreadID: "forged-route-action", Content: "Create a Mission",
			ProfileID: "codex", IncidentID: "incident-forged-route-action",
		},
	)
	if err != nil || len(thread.ActionProposals) != 0 ||
		len(thread.Attempts) != 1 || thread.Attempts[0].Status != "failed" ||
		thread.Attempts[0].FailureCode != "control_conflict" {
		t.Fatalf("forged Route thread = %#v error = %v", thread, err)
	}
	unchanged, err := chat.ChatThread(context.Background(), "forged-route-action")
	if err != nil || len(unchanged.ActionProposals) != 0 ||
		len(unchanged.Attempts) != 1 || unchanged.Attempts[0].Status != "failed" {
		t.Fatalf("forged proposal mutated authority: %#v error=%v", unchanged, err)
	}
}

func TestConversationActionConfirmationRejectsWorkspaceDrift(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 50, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	chat.responder = &conversationActionControlResponder{chat: chat}
	thread, err := chat.SendMessage(
		context.Background(), LocalProductChatMessageRequest{
			ThreadID: "workspace-drift-action", Content: "Create a Mission",
			ProfileID: "codex", IncidentID: "incident-workspace-drift-action",
		},
	)
	if err != nil || len(thread.ActionProposals) != 1 ||
		thread.ControlWorkspace == nil {
		t.Fatalf("thread=%#v error=%v", thread, err)
	}
	proposal := thread.ActionProposals[0]
	drifted := phase7ControlTestWorkspace()
	drifted.WorkspaceDigest = controltool.DigestBytes([]byte("workspace-drifted"))
	chat.mu.Lock()
	chat.controlWorkspace = cloneLocalProductControlWorkspace(&drifted)
	chat.mu.Unlock()

	_, err = chat.DecideControlProposal(
		context.Background(), LocalProductChatControlDecisionRequest{
			ThreadID: thread.ThreadID, ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-workspace-drift-confirm",
		},
	)
	if !errors.Is(err, ErrLocalProductChatControlConflict) {
		t.Fatalf("workspace drift confirmation error = %v", err)
	}
	cancelled, err := chat.DecideControlProposal(
		context.Background(), LocalProductChatControlDecisionRequest{
			ThreadID: thread.ThreadID, ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionCancel,
			IncidentID: "incident-workspace-drift-cancel",
		},
	)
	if err != nil || cancelled.ActionProposals[0].Status != controltool.ProposalCancelled ||
		cancelled.RequiresConfirmation || len(cancelled.ProposalDecisionReceipts) != 1 ||
		cancelled.ProposalDecisionReceipts[0].Decision != controltool.ProposalDecisionCancel {
		t.Fatalf("workspace drift cancellation thread=%#v error=%v", cancelled, err)
	}
}

func TestConversationActionExpiryWinsOverWorkspaceDrift(t *testing.T) {
	now := time.Date(2026, 8, 28, 15, 55, 0, 0, time.UTC)
	chat := newPhase7ControlTestChat(func() time.Time { return now })
	chat.responder = &conversationActionControlResponder{chat: chat}
	thread, err := chat.SendMessage(
		context.Background(), LocalProductChatMessageRequest{
			ThreadID: "workspace-drift-expiry", Content: "Create a Mission",
			ProfileID: "codex", IncidentID: "incident-workspace-drift-expiry",
		},
	)
	if err != nil || len(thread.ActionProposals) != 1 {
		t.Fatalf("thread=%#v error=%v", thread, err)
	}
	proposal := thread.ActionProposals[0]
	drifted := phase7ControlTestWorkspace()
	drifted.WorkspaceDigest = controltool.DigestBytes([]byte("workspace-expiry-drifted"))
	chat.mu.Lock()
	chat.controlWorkspace = cloneLocalProductControlWorkspace(&drifted)
	chat.mu.Unlock()
	now = now.Add(6 * time.Minute)

	expired, err := chat.DecideControlProposal(
		context.Background(), LocalProductChatControlDecisionRequest{
			ThreadID: thread.ThreadID, ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-workspace-drift-expired",
		},
	)
	if err != nil || expired.ActionProposals[0].Status != controltool.ProposalExpired ||
		expired.RequiresConfirmation || len(expired.ProposalDecisionReceipts) != 1 ||
		expired.ProposalDecisionReceipts[0].Decision != controltool.ProposalDecisionExpire {
		t.Fatalf("workspace drift expiry thread=%#v error=%v", expired, err)
	}
}

func TestConfirmedGovernanceActionProposalRestoresAfterEncryptedRestart(t *testing.T) {
	now := time.Date(2026, 8, 28, 16, 0, 0, 0, time.UTC)
	documents := newMemoryLocalProductChatDocumentStore()
	path := filepath.Join(privateLocalProductChatTestRoot(t), "chat-threads.json")
	chat, err := NewEncryptedPersistentLocalProductChatAPI(
		context.Background(), path, documents, func() time.Time { return now }, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	setPhase7ControlTestBindingResolver(chat)
	if err := chat.SetControlWorkspace(phase7ControlTestWorkspace()); err != nil {
		t.Fatal(err)
	}
	chat.responder = &conversationGovernanceActionResponder{
		chat: chat, toolID: controltool.ToolConversationRouteChangePreview,
		arguments: json.RawMessage(`{"profile_id":"conversation-deepseek-r4"}`),
	}
	thread, err := chat.SendMessage(
		context.Background(), LocalProductChatMessageRequest{
			ThreadID: "governance-restart-thread", Content: "Use DeepSeek",
			ProfileID: "codex", IncidentID: "incident-governance-restart-1",
		},
	)
	if err != nil || len(thread.ActionProposals) != 1 {
		t.Fatalf("thread=%#v error=%v", thread, err)
	}
	proposal := thread.ActionProposals[0]
	confirmed, err := chat.DecideControlProposal(
		context.Background(), LocalProductChatControlDecisionRequest{
			ThreadID: thread.ThreadID, ProposalID: proposal.ProposalID,
			ProposalDigest: proposal.ProposalDigest, Decision: ControlDecisionConfirm,
			IncidentID: "incident-governance-restart-confirm-1",
		},
	)
	if err != nil || confirmed.ActionProposals[0].Status != controltool.ProposalConfirmed {
		t.Fatalf("confirmed=%#v error=%v", confirmed, err)
	}

	restarted, err := NewEncryptedPersistentLocalProductChatAPI(
		context.Background(), path, documents, func() time.Time { return now }, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.SetControlWorkspace(phase7ControlTestWorkspace()); err != nil {
		t.Fatal(err)
	}
	restored, err := restarted.ChatThread(context.Background(), thread.ThreadID)
	if err != nil || len(restored.ActionProposals) != 1 {
		t.Fatalf("restored=%#v error=%v", restored, err)
	}
	if len(restored.ProposalDecisionReceipts) != 1 ||
		!restored.ProposalDecisionReceipts[0].Valid() ||
		!restored.ProposalDecisionReceipts[0].MatchesConversationActionProposal(
			restored.ActionProposals[0],
		) ||
		restored.ProposalDecisionReceipts[0].DecisionIncidentID !=
			"incident-governance-restart-confirm-1" {
		t.Fatalf("restored decision receipt=%#v", restored.ProposalDecisionReceipts)
	}
	restoredProposal := restored.ActionProposals[0]
	if restoredProposal.Status != controltool.ProposalConfirmed ||
		restoredProposal.ProposalDigest != proposal.ProposalDigest ||
		restoredProposal.Payload == nil ||
		restoredProposal.Payload.ProfileID != "conversation-deepseek-r4" ||
		restoredProposal.Route == nil || restoredProposal.Workspace == nil ||
		restored.ControlWorkspace == nil ||
		*restored.ControlWorkspace != phase7ControlTestWorkspace() ||
		restoredProposal.RegistryDigest != localProductBuiltinControlRegistryDigest() ||
		restoredProposal.IncidentID != "incident-governance-restart-1" {
		t.Fatalf("restored proposal=%#v", restoredProposal)
	}
	restored.ActionProposals[0].Payload.ProfileID = "conversation-tampered-r9"
	authoritative, err := restarted.ChatThread(context.Background(), thread.ThreadID)
	if err != nil || authoritative.ActionProposals[0].Payload == nil ||
		authoritative.ActionProposals[0].Payload.ProfileID != "conversation-deepseek-r4" {
		t.Fatalf("restart authority mutated: %#v error=%v", authoritative, err)
	}
}

func TestEncryptedRestartRestoresCancelledUnboundRoundTableProposal(t *testing.T) {
	now := time.Date(2026, 8, 29, 16, 10, 0, 0, time.UTC)
	documents := newMemoryLocalProductChatDocumentStore()
	path := filepath.Join(privateLocalProductChatTestRoot(t), "chat-threads.json")
	chat, err := NewEncryptedPersistentLocalProductChatAPI(
		context.Background(), path, documents, func() time.Time { return now },
		localProductConversationResponderFunc(func(
			context.Context,
			LocalProductConversationRequest,
		) (LocalProductConversationResponse, error) {
			return LocalProductConversationResponse{Content: "historical result"}, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: "historical-roundtable-proposal", Content: "historical request",
		ProfileID: "codex", IncidentID: "incident-historical-roundtable",
	})
	if err != nil || len(thread.Segments) != 1 || len(thread.Attempts) != 1 ||
		len(thread.Messages) != 2 {
		t.Fatalf("base historical thread=%#v error=%v", thread, err)
	}
	route := &controltool.FrozenRouteReference{
		HarnessAdapter: "codex", ProviderID: "openai", ModelID: "gpt-test",
		ExecutionBindingDigest: controltool.DigestBytes([]byte("historical-binding")),
		ContextCapsuleDigest:   controltool.DigestBytes([]byte("historical-capsule")),
	}
	proposal, err := controltool.NewConversationActionProposal(
		controltool.ConversationActionProposalInput{
			ProposalID:           "proposal-historical-roundtable-skip-1",
			ToolID:               controltool.ToolRoundtablesSkipPreview,
			TargetConversationID: thread.ThreadID,
			TargetContentDigest:  controltool.DigestBytes([]byte("historical-target")),
			Payload: &controltool.ConversationActionPayload{
				SessionID: "rt-20260827-0953", RoundID: "round-3",
				SeatID: "agent-subagent-loom-bounded-worker-loom-minimax-subagent-r23",
			},
			Route: route, Workspace: func() *controltool.FrozenWorkspaceReference {
				workspace := phase7ControlTestWorkspace()
				return &workspace
			}(),
			RegistryDigest: controltool.DigestBytes([]byte("historical-registry")),
			IncidentID:     "incident-historical-roundtable-proposal",
			SegmentID:      thread.Segments[0].SegmentID,
			AttemptID:      thread.Attempts[0].AttemptID,
			CreatedAt:      now, ExpiresAt: now.Add(5 * time.Minute),
		},
	)
	if err != nil || !proposal.Valid() || proposal.Confirmable() {
		t.Fatalf("legacy unbound proposal=%#v error=%v", proposal, err)
	}
	receipt, err := controltool.NewConversationActionDecisionReceipt(
		proposal, controltool.ProposalDecisionCancel,
		"incident-historical-roundtable-cancel", now.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	proposal.Status = controltool.ProposalCancelled
	proposal.MessageID = thread.Messages[len(thread.Messages)-1].MessageID
	thread.ActionProposals = []controltool.ConversationActionProposal{proposal}
	thread.ProposalDecisionReceipts = []controltool.ProposalDecisionReceipt{receipt}
	thread.Attempts[0].CompletedControlTools = []controltool.CompletedCall{{
		ToolID: controltool.ToolRoundtablesSkipPreview, ToolVersion: 2,
		Effect: controltool.EffectProposal,
	}}
	stored, err := documents.ConversationDocuments(
		context.Background(), localProductChatDocumentKind,
	)
	if err != nil || len(stored) != 1 {
		t.Fatalf("stored base documents=%#v error=%v", stored, err)
	}
	decoded, err := decodeLocalProductChatThreadDocument(stored[0].Payload)
	if err != nil {
		t.Fatal(err)
	}
	decoded.Thread = thread
	stored[0].Payload, err = json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	historical := newMemoryLocalProductChatDocumentStore()
	historical.documents[stored[0].ConversationID+"\x00"+stored[0].Kind] = stored[0]

	restarted, err := NewEncryptedPersistentLocalProductChatAPI(
		context.Background(), path, historical, func() time.Time { return now }, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := restarted.ChatThread(context.Background(), thread.ThreadID)
	if err != nil || len(restored.ActionProposals) != 1 ||
		restored.ActionProposals[0].Status != controltool.ProposalCancelled ||
		restored.ActionProposals[0].Confirmable() ||
		len(restored.ProposalDecisionReceipts) != 1 ||
		!restored.ProposalDecisionReceipts[0].MatchesConversationActionProposal(
			restored.ActionProposals[0],
		) {
		t.Fatalf("restored historical proposal=%#v error=%v", restored, err)
	}
}

type conversationControlResponder struct {
	chat     *LocalProductChatAPI
	requests []LocalProductConversationRequest
}

type conversationActionControlResponder struct{ chat *LocalProductChatAPI }

type conversationGovernanceActionResponder struct {
	chat      *LocalProductChatAPI
	toolID    controltool.ToolID
	arguments json.RawMessage
}

type mutableRoundTableActionTargetResolver struct {
	target        LocalProductConversationRoundTableTargetBinding
	err           error
	validateCalls int
}

func (resolver *mutableRoundTableActionTargetResolver) ValidateRoundTableActionTarget(
	context.Context,
	controltool.ToolID,
	controltool.ConversationActionPayload,
) error {
	resolver.validateCalls++
	return resolver.err
}

func (resolver *mutableRoundTableActionTargetResolver) ResolveRoundTableActionTarget(
	context.Context,
	controltool.ToolID,
	controltool.ConversationActionPayload,
) (LocalProductConversationRoundTableTargetBinding, error) {
	return resolver.target, resolver.err
}

type conversationForgedRouteActionResponder struct{ chat *LocalProductChatAPI }

func (responder *conversationForgedRouteActionResponder) Respond(
	ctx context.Context,
	request LocalProductConversationRequest,
) (LocalProductConversationResponse, error) {
	turn := phase7ControlTurnForRequest(request)
	result, err := responder.chat.Call(ctx, turn, controltool.Call{
		ToolID:    controltool.ToolMissionsCreatePreview,
		Arguments: json.RawMessage(`{"objective":"Ship the governed release"}`),
	})
	if err != nil {
		return LocalProductConversationResponse{}, err
	}
	legitimate := result.ActionProposal
	forgedRoute := *legitimate.Route
	forgedRoute.ModelID = "forged-model"
	forged, err := controltool.NewConversationActionProposal(
		controltool.ConversationActionProposalInput{
			ProposalID: legitimate.ProposalID, ToolID: legitimate.ToolID,
			TargetConversationID: legitimate.TargetConversationID,
			TargetContentDigest:  legitimate.TargetContentDigest,
			Argument:             legitimate.Argument, Payload: legitimate.Payload,
			Route: &forgedRoute, Workspace: legitimate.Workspace,
			RegistryDigest: legitimate.RegistryDigest, IncidentID: legitimate.IncidentID,
			SegmentID: legitimate.SegmentID, AttemptID: legitimate.AttemptID,
			CreatedAt: legitimate.CreatedAt, ExpiresAt: legitimate.ExpiresAt,
		},
	)
	if err != nil {
		return LocalProductConversationResponse{}, err
	}
	return LocalProductConversationResponse{
		Content: "I prepared a forged proposal.", Tentative: true,
		ActionProposals: []controltool.ConversationActionProposal{forged},
	}, nil
}

func newPhase7ControlTestChat(now func() time.Time) *LocalProductChatAPI {
	chat := NewLocalProductChatAPI(now)
	setPhase7ControlTestBindingResolver(chat)
	if err := chat.SetControlWorkspace(phase7ControlTestWorkspace()); err != nil {
		panic(err)
	}
	return chat
}

func phase7ControlTestWorkspace() controltool.FrozenWorkspaceReference {
	return controltool.FrozenWorkspaceReference{
		WorkspaceID:     "workspace-control-test",
		WorkspaceDigest: controltool.DigestBytes([]byte("workspace-control-test")),
	}
}

func setPhase7ControlTestBindingResolver(chat *LocalProductChatAPI) {
	chat.bindingResolver = localProductConversationBindingResolverFunc(func(
		context.Context,
		string,
	) (LocalProductConversationExecutionBinding, error) {
		return LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "codex", ProviderID: "openai",
			ModelID: "gpt-test",
		}, nil
	})
}

func phase7ControlTurnForRequest(
	request LocalProductConversationRequest,
) controltool.TurnContext {
	registry, _ := controltool.NewBuiltinRegistry()
	route := controltool.FrozenRouteReference{}
	if request.ExecutionBinding != nil {
		route = controltool.FrozenRouteReference{
			HarnessAdapter:         request.ExecutionBinding.HarnessAdapter,
			ProviderID:             request.ExecutionBinding.ProviderID,
			ProviderAccountID:      request.ExecutionBinding.ProviderAccountID,
			CredentialRevision:     request.ExecutionBinding.CredentialRevision,
			ModelID:                request.ExecutionBinding.ModelID,
			ReasoningEffort:        request.ReasoningEffort,
			ExecutionBindingDigest: request.SegmentBindingDigest,
			ContextCapsuleDigest:   request.ContextCapsuleDigest,
		}
	}
	return controltool.TurnContext{
		ConversationID: request.ThreadID, SegmentID: request.SegmentID,
		AttemptID: request.AttemptID, IncidentID: request.IncidentID,
		RegistryDigest: registry.Digest(),
		Catalog:        request.SessionCatalog, CatalogDigest: request.CatalogDigest,
		Route:     route,
		Workspace: phase7ControlTestWorkspace(),
	}
}

func (responder *conversationGovernanceActionResponder) Respond(
	ctx context.Context,
	request LocalProductConversationRequest,
) (LocalProductConversationResponse, error) {
	result, err := responder.chat.Call(
		ctx, phase7ControlTurnForRequest(request),
		controltool.Call{ToolID: responder.toolID, Arguments: responder.arguments},
	)
	if err != nil {
		return LocalProductConversationResponse{}, err
	}
	return LocalProductConversationResponse{
		Content: "I prepared a Loom governance change for review.", Tentative: true,
		ActionProposals: []controltool.ConversationActionProposal{*result.ActionProposal},
	}, nil
}

func (responder *conversationActionControlResponder) Respond(
	ctx context.Context,
	request LocalProductConversationRequest,
) (LocalProductConversationResponse, error) {
	arguments := json.RawMessage(`{"objective":"Ship the governed release"}`)
	result, err := responder.chat.Call(
		ctx, phase7ControlTurnForRequest(request),
		controltool.Call{ToolID: controltool.ToolMissionsCreatePreview, Arguments: arguments},
	)
	if err != nil {
		return LocalProductConversationResponse{}, err
	}
	return LocalProductConversationResponse{
		Content: "I prepared a Mission draft for review.", Tentative: true,
		ActionProposals: []controltool.ConversationActionProposal{*result.ActionProposal},
	}, nil
}

func (responder *conversationControlResponder) Respond(
	ctx context.Context,
	request LocalProductConversationRequest,
) (LocalProductConversationResponse, error) {
	responder.requests = append(responder.requests, request)
	if request.ThreadID != "s3" || !strings.Contains(request.Messages[len(request.Messages)-1].Content, "Align") {
		return LocalProductConversationResponse{
			Content: "reply for " + request.ThreadID, Tentative: true,
		}, nil
	}
	sessionIDs := []string{"s1", "s2"}
	if len(request.SessionCatalog) == 2 {
		sessionIDs = []string{"s1"}
	}
	arguments, _ := json.Marshal(map[string]any{
		"session_ids": sessionIDs, "context_mode": "summary_only",
	})
	result, err := responder.chat.Call(ctx, phase7ControlTurnForRequest(request), controltool.Call{
		ToolID: controltool.ToolSessionsAlignPreview, Arguments: arguments,
	})
	if err != nil {
		return LocalProductConversationResponse{}, err
	}
	return LocalProductConversationResponse{
		Content: "I prepared a context alignment for review.", Tentative: true,
		ControlProposals: []controltool.SessionAlignmentProposal{*result.Proposal},
	}, nil
}

func mustChatThread(t *testing.T, chat *LocalProductChatAPI, threadID string) LocalProductChatThread {
	t.Helper()
	thread, err := chat.ChatThread(context.Background(), threadID)
	if err != nil {
		t.Fatal(err)
	}
	return thread
}
