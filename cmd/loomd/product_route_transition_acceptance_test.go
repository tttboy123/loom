package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/localipc"
)

const (
	productRouteTransitionThreadID    = "phase2d-route-transition"
	productRouteTransitionSourceID    = "conversation-codex-openai-source"
	productRouteTransitionTargetID    = "conversation-native-anthropic-target"
	productRouteTransitionSourceModel = "gpt-5.5-codex"
	productRouteTransitionTargetModel = "claude-sonnet-4"
)

var productRouteTransitionDiagnosticSecrets = []string{
	"provider-body-must-not-cross-wire",
	"full-prompt-must-not-cross-wire",
	"credential-must-not-cross-wire",
}

type productRouteTransitionBindingResolver struct {
	bindings map[string]api.LocalProductConversationExecutionBinding
}

func (resolver *productRouteTransitionBindingResolver) ResolveConversationExecutionBinding(
	_ context.Context,
	profileID string,
	modelID string,
) (api.LocalProductConversationExecutionBinding, error) {
	binding, ok := resolver.bindings[profileID]
	if !ok || binding.ModelID != modelID {
		return api.LocalProductConversationExecutionBinding{}, api.ErrInvalidLocalProductChatRequest
	}
	return binding, nil
}

func (resolver *productRouteTransitionBindingResolver) set(
	profileID string,
	binding api.LocalProductConversationExecutionBinding,
) {
	resolver.bindings[profileID] = binding
}

type productRouteTransitionResponder struct {
	requests []api.LocalProductConversationRequest
}

type productRouteTransitionTrustAcknowledgementWire = api.LocalProductTrustBoundaryAcknowledgement

type productRouteTransitionMessageWire struct {
	ThreadID                    string                                          `json:"thread_id"`
	Content                     string                                          `json:"content"`
	ProfileID                   string                                          `json:"profile_id"`
	ModelID                     string                                          `json:"model_id,omitempty"`
	ReasoningEffort             string                                          `json:"reasoning_effort,omitempty"`
	ContextMode                 api.LocalProductContextMode                     `json:"context_mode"`
	ExpectedExecutionBinding    *api.LocalProductConversationExecutionBinding   `json:"expected_execution_binding,omitempty"`
	TrustBoundaryAcknowledgment *productRouteTransitionTrustAcknowledgementWire `json:"trust_boundary_acknowledgement,omitempty"`
}

func (responder *productRouteTransitionResponder) Respond(
	_ context.Context,
	request api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error) {
	responder.requests = append(responder.requests, request)
	return api.LocalProductConversationResponse{
		Content:   "route transition response " + string(rune('0'+len(responder.requests))),
		Tentative: true,
	}, nil
}

type productRouteTransitionReadRoute struct {
	productReadRoute
	chat *api.LocalProductChatAPI
}

func (route *productRouteTransitionReadRoute) ReadChatThread(
	ctx context.Context,
	threadID string,
) (api.LocalProductChatThread, error) {
	return route.chat.ChatThread(ctx, threadID)
}

func (route *productRouteTransitionReadRoute) SendChatMessage(
	ctx context.Context,
	request api.LocalProductChatMessageRequest,
) (api.LocalProductChatThread, error) {
	return route.chat.SendMessage(ctx, request)
}

type productRouteTransitionAcceptanceFixture struct {
	client    *localipc.Client
	resolver  *productRouteTransitionBindingResolver
	responder *productRouteTransitionResponder
}

func newProductRouteTransitionAcceptanceFixture(
	t *testing.T,
) *productRouteTransitionAcceptanceFixture {
	t.Helper()
	root, err := os.MkdirTemp("/private/tmp", "loom-route-transition-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	responder := &productRouteTransitionResponder{}
	chat, err := api.NewPersistentLocalProductChatAPI(
		filepath.Join(root, "chat.json"), time.Now, responder,
	)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &productRouteTransitionBindingResolver{
		bindings: make(map[string]api.LocalProductConversationExecutionBinding),
	}
	if err := chat.SetConversationExecutionBindingResolver(resolver); err != nil {
		t.Fatal(err)
	}
	route := &productRouteTransitionReadRoute{chat: chat}
	socketPath := filepath.Join(root, "loomd.sock")
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(),
		BuildID: "phase2d-route-transition-acceptance",
		Handler: localipc.HandlerFunc(newProductRouteHandler(productRouteServices{
			read: route,
		})),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case err := <-done:
		t.Fatalf("route transition server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("route transition server did not become ready")
	}
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("route transition server close: %v", err)
		}
	})
	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if rootInfo.Mode().Perm() != 0o700 {
		t.Fatalf("private IPC root mode = %v", rootInfo.Mode().Perm())
	}
	socketInfo, err := os.Lstat(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	if socketInfo.Mode().Perm() != 0o600 || socketInfo.Mode()&os.ModeSocket == 0 {
		t.Fatalf("private IPC socket = %v", socketInfo.Mode())
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &productRouteTransitionAcceptanceFixture{
		client: client, resolver: resolver, responder: responder,
	}
}

func (fixture *productRouteTransitionAcceptanceFixture) seed(
	t *testing.T,
) api.LocalProductChatThread {
	t.Helper()
	sourceBinding := productRouteTransitionSourceBinding()
	fixture.resolver.set(productRouteTransitionSourceID, sourceBinding)
	var thread api.LocalProductChatThread
	if err := fixture.client.Call(
		context.Background(),
		"chat_message",
		api.LocalProductChatMessageRequest{
			ThreadID:  productRouteTransitionThreadID,
			Content:   "preserve this source conversation context",
			ProfileID: productRouteTransitionSourceID,
			ModelID:   productRouteTransitionSourceModel,
		},
		&thread,
	); err != nil {
		t.Fatal(err)
	}
	if len(thread.Segments) != 1 || len(thread.Messages) != 2 ||
		len(thread.Attempts) != 1 || len(fixture.responder.requests) != 1 ||
		thread.Segments[0].ExecutionBinding == nil ||
		*thread.Segments[0].ExecutionBinding != sourceBinding {
		t.Fatalf("seeded source thread = %#v, responder calls = %d", thread, len(fixture.responder.requests))
	}
	return thread
}

func (fixture *productRouteTransitionAcceptanceFixture) readThread(
	t *testing.T,
) api.LocalProductChatThread {
	t.Helper()
	var thread api.LocalProductChatThread
	if err := fixture.client.Call(
		context.Background(),
		"chat_thread",
		api.LocalProductChatThreadRequest{ThreadID: productRouteTransitionThreadID},
		&thread,
	); err != nil {
		t.Fatal(err)
	}
	return thread
}

func TestPhase2DRouteTransitionRejectsMissingExpectedBindingThroughAuthenticatedIPC(
	t *testing.T,
) {
	tests := []struct {
		name            string
		profileID       string
		modelID         string
		reasoningEffort string
		binding         api.LocalProductConversationExecutionBinding
	}{
		{
			name:      "profile switch",
			profileID: productRouteTransitionTargetID,
			modelID:   productRouteTransitionTargetModel,
			binding:   productRouteTransitionTargetBinding(),
		},
		{
			name:      "same profile policy switch",
			profileID: productRouteTransitionSourceID,
			modelID:   productRouteTransitionSourceModel,
			binding:   productRouteTransitionSourcePolicyDrift(),
		},
		{
			name:            "same profile reasoning switch",
			profileID:       productRouteTransitionSourceID,
			modelID:         productRouteTransitionSourceModel,
			reasoningEffort: "high",
			binding:         productRouteTransitionSourceBinding(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newProductRouteTransitionAcceptanceFixture(t)
			before := fixture.seed(t)
			fixture.resolver.set(test.profileID, test.binding)
			callsBefore := len(fixture.responder.requests)
			var ignored api.LocalProductChatThread
			err := fixture.client.Call(
				context.Background(),
				"chat_message",
				api.LocalProductChatMessageRequest{
					ThreadID: productRouteTransitionThreadID,
					Content: strings.Join(
						productRouteTransitionDiagnosticSecrets, " ",
					),
					ProfileID:       test.profileID,
					ModelID:         test.modelID,
					ReasoningEffort: test.reasoningEffort,
					ContextMode:     api.ContextModeSummaryOnly,
				},
				&ignored,
			)
			assertProductRouteTransitionConflict(t, err)
			if len(fixture.responder.requests) != callsBefore {
				t.Fatalf("responder calls = %d, want %d", len(fixture.responder.requests), callsBefore)
			}
			after := fixture.readThread(t)
			assertProductRouteTransitionUnchanged(t, before, after)
		})
	}
}

func TestPhase2DRouteTransitionRequiresSeparateTrustBoundaryAcknowledgementThroughAuthenticatedIPC(
	t *testing.T,
) {
	t.Run("same trust model transition missing acknowledgement", func(t *testing.T) {
		fixture := newProductRouteTransitionAcceptanceFixture(t)
		before := fixture.seed(t)
		target := productRouteTransitionSourceBinding()
		target.ModelID = "gpt-5.6-sol"
		fixture.resolver.set(productRouteTransitionSourceID, target)
		callsBefore := len(fixture.responder.requests)

		var ignored api.LocalProductChatThread
		err := fixture.client.Call(
			context.Background(),
			"chat_message",
			api.LocalProductChatMessageRequest{
				ThreadID: productRouteTransitionThreadID, Content: "model transition was not reviewed",
				ProfileID: productRouteTransitionSourceID, ModelID: target.ModelID,
				ContextMode: api.ContextModeSummaryOnly, ExpectedExecutionBinding: &target,
			},
			&ignored,
		)
		assertProductRouteTransitionConflict(t, err)
		if len(fixture.responder.requests) != callsBefore {
			t.Fatalf("responder calls = %d, want %d", len(fixture.responder.requests), callsBefore)
		}
		assertProductRouteTransitionUnchanged(t, before, fixture.readThread(t))
	})

	t.Run("missing acknowledgement", func(t *testing.T) {
		fixture := newProductRouteTransitionAcceptanceFixture(t)
		before := fixture.seed(t)
		target := productRouteTransitionTargetBinding()
		fixture.resolver.set(productRouteTransitionTargetID, target)
		callsBefore := len(fixture.responder.requests)

		var ignored api.LocalProductChatThread
		err := fixture.client.Call(
			context.Background(),
			"chat_message",
			api.LocalProductChatMessageRequest{
				ThreadID: productRouteTransitionThreadID, Content: "trust review was not acknowledged",
				ProfileID: productRouteTransitionTargetID, ModelID: productRouteTransitionTargetModel,
				ContextMode: api.ContextModeSummaryOnly, ExpectedExecutionBinding: &target,
			},
			&ignored,
		)
		assertProductRouteTransitionConflict(t, err)
		if len(fixture.responder.requests) != callsBefore {
			t.Fatalf("responder calls = %d, want %d", len(fixture.responder.requests), callsBefore)
		}
		assertProductRouteTransitionUnchanged(t, before, fixture.readThread(t))
	})

	t.Run("acknowledged exact immutable source and target", func(t *testing.T) {
		fixture := newProductRouteTransitionAcceptanceFixture(t)
		before := fixture.seed(t)
		target := productRouteTransitionTargetBinding()
		fixture.resolver.set(productRouteTransitionTargetID, target)
		acknowledgement := productRouteTransitionTrustAcknowledgement(
			t, before, productRouteTransitionTargetID, target, api.ContextModeSummaryOnly,
		)

		var after api.LocalProductChatThread
		if err := fixture.client.Call(
			context.Background(),
			"chat_message",
			productRouteTransitionMessageWire{
				ThreadID: productRouteTransitionThreadID, Content: "trust review was explicitly acknowledged",
				ProfileID: productRouteTransitionTargetID, ModelID: productRouteTransitionTargetModel,
				ContextMode: api.ContextModeSummaryOnly, ExpectedExecutionBinding: &target,
				TrustBoundaryAcknowledgment: acknowledgement,
			},
			&after,
		); err != nil {
			t.Fatal(err)
		}
		if len(after.Segments) != 2 || len(after.Attempts) != 2 ||
			after.Segments[1].ExecutionBinding == nil ||
			*after.Segments[1].ExecutionBinding != target {
			t.Fatalf("acknowledged transition = %#v", after)
		}
	})
}

func TestPhase2DRouteTransitionRejectsStaleExpectedBindingThroughAuthenticatedIPC(
	t *testing.T,
) {
	fixture := newProductRouteTransitionAcceptanceFixture(t)
	before := fixture.seed(t)
	approved := productRouteTransitionTargetBinding()
	drifted := approved
	drifted.ProviderAccountPolicyRevision++
	drifted.ProviderAccountPolicyDigest = strings.Repeat("f", 64)
	fixture.resolver.set(productRouteTransitionTargetID, drifted)
	callsBefore := len(fixture.responder.requests)
	var ignored api.LocalProductChatThread
	err := fixture.client.Call(
		context.Background(),
		"chat_message",
		productRouteTransitionMessageWire{
			ThreadID: productRouteTransitionThreadID,
			Content: strings.Join(
				productRouteTransitionDiagnosticSecrets, " ",
			),
			ProfileID:                productRouteTransitionTargetID,
			ModelID:                  productRouteTransitionTargetModel,
			ContextMode:              api.ContextModeSummaryOnly,
			ExpectedExecutionBinding: &approved,
			TrustBoundaryAcknowledgment: productRouteTransitionTrustAcknowledgement(
				t, before, productRouteTransitionTargetID, approved, api.ContextModeSummaryOnly,
			),
		},
		&ignored,
	)
	assertProductRouteTransitionConflict(t, err)
	if len(fixture.responder.requests) != callsBefore {
		t.Fatalf("responder calls = %d, want %d", len(fixture.responder.requests), callsBefore)
	}
	after := fixture.readThread(t)
	assertProductRouteTransitionUnchanged(t, before, after)
}

func TestPhase2DRouteTransitionRejectsReasoningAddedAfterTrustReviewThroughAuthenticatedIPC(
	t *testing.T,
) {
	fixture := newProductRouteTransitionAcceptanceFixture(t)
	before := fixture.seed(t)
	target := productRouteTransitionTargetBinding()
	fixture.resolver.set(productRouteTransitionTargetID, target)
	acknowledgement := productRouteTransitionTrustAcknowledgement(
		t, before, productRouteTransitionTargetID, target, api.ContextModeSummaryOnly,
	)
	callsBefore := len(fixture.responder.requests)

	var ignored api.LocalProductChatThread
	err := fixture.client.Call(
		context.Background(),
		"chat_message",
		productRouteTransitionMessageWire{
			ThreadID: productRouteTransitionThreadID,
			Content: strings.Join(
				productRouteTransitionDiagnosticSecrets, " ",
			),
			ProfileID:                   productRouteTransitionTargetID,
			ModelID:                     productRouteTransitionTargetModel,
			ReasoningEffort:             "high",
			ContextMode:                 api.ContextModeSummaryOnly,
			ExpectedExecutionBinding:    &target,
			TrustBoundaryAcknowledgment: acknowledgement,
		},
		&ignored,
	)
	assertProductRouteTransitionConflict(t, err)
	if len(fixture.responder.requests) != callsBefore {
		t.Fatalf("responder calls = %d, want %d", len(fixture.responder.requests), callsBefore)
	}
	assertProductRouteTransitionUnchanged(t, before, fixture.readThread(t))
}

func TestPhase2DRouteTransitionFreezesApprovedBindingThroughAuthenticatedIPC(
	t *testing.T,
) {
	for _, mode := range []api.LocalProductContextMode{
		api.ContextModeContinueWithContext,
		api.ContextModeSummaryOnly,
		api.ContextModeStartClean,
	} {
		t.Run(string(mode), func(t *testing.T) {
			fixture := newProductRouteTransitionAcceptanceFixture(t)
			before := fixture.seed(t)
			oldSegment := before.Segments[0]
			targetBinding := productRouteTransitionTargetBinding()
			fixture.resolver.set(productRouteTransitionTargetID, targetBinding)
			content := "approved transition using " + string(mode)
			acknowledgement := productRouteTransitionTrustAcknowledgement(
				t, before, productRouteTransitionTargetID, targetBinding, mode,
			)
			var after api.LocalProductChatThread
			if err := fixture.client.Call(
				context.Background(),
				"chat_message",
				productRouteTransitionMessageWire{
					ThreadID:                    productRouteTransitionThreadID,
					Content:                     content,
					ProfileID:                   productRouteTransitionTargetID,
					ModelID:                     productRouteTransitionTargetModel,
					ContextMode:                 mode,
					ExpectedExecutionBinding:    &targetBinding,
					TrustBoundaryAcknowledgment: acknowledgement,
				},
				&after,
			); err != nil {
				t.Fatal(err)
			}
			if len(after.Segments) != len(before.Segments)+1 ||
				len(after.Attempts) != len(before.Attempts)+1 ||
				len(after.Messages) != len(before.Messages)+2 ||
				len(fixture.responder.requests) != 2 {
				t.Fatalf("approved transition thread = %#v, responder calls = %d", after, len(fixture.responder.requests))
			}
			if !reflect.DeepEqual(after.Segments[0], oldSegment) {
				t.Fatalf("source segment changed:\n before=%#v\n after=%#v", oldSegment, after.Segments[0])
			}
			segment := after.Segments[1]
			attempt := after.Attempts[1]
			if after.ProfileID != productRouteTransitionTargetID ||
				segment.ProfileID != productRouteTransitionTargetID ||
				segment.ModelID != productRouteTransitionTargetModel ||
				segment.ContextMode != mode ||
				segment.ExecutionBinding == nil ||
				*segment.ExecutionBinding != targetBinding ||
				segment.RouteTransitionReviewDigest != acknowledgement.ReviewDigest {
				t.Fatalf("frozen target segment = %#v", segment)
			}
			if attempt.SegmentID != segment.SegmentID ||
				attempt.ProfileID != segment.ProfileID ||
				attempt.ModelID != segment.ModelID ||
				attempt.ContextMode != mode ||
				attempt.ContextCapsuleDigest != segment.ContextCapsuleDigest ||
				attempt.DisclosureReceiptDigest != segment.DisclosureReceiptDigest ||
				attempt.BindingDigest != segment.BindingDigest ||
				attempt.RouteTransitionReviewDigest != acknowledgement.ReviewDigest ||
				attempt.ExecutionBinding == nil ||
				*attempt.ExecutionBinding != targetBinding ||
				!validProductRouteTransitionClientIncidentID(attempt.IncidentID) ||
				attempt.IncidentID == before.Attempts[len(before.Attempts)-1].IncidentID ||
				attempt.Status != "succeeded" {
				t.Fatalf("frozen target attempt = %#v", attempt)
			}
			for name, digest := range map[string]string{
				"capsule": segment.ContextCapsuleDigest,
				"receipt": segment.DisclosureReceiptDigest,
				"binding": segment.BindingDigest,
			} {
				if decoded, err := hex.DecodeString(digest); err != nil || len(decoded) != 32 {
					t.Fatalf("%s digest = %q, error = %v", name, digest, err)
				}
			}
			assertProductRouteTransitionModeDispatch(
				t, mode, content, fixture.responder.requests[1],
			)
		})
	}
}

func TestPhase2DRouteTransitionFreezesSameProfileModelAndReasoningChangesThroughAuthenticatedIPC(
	t *testing.T,
) {
	for _, test := range []struct {
		name            string
		modelID         string
		reasoningEffort string
		binding         api.LocalProductConversationExecutionBinding
	}{
		{
			name:    "model",
			modelID: "gpt-5.6-sol",
			binding: func() api.LocalProductConversationExecutionBinding {
				binding := productRouteTransitionSourceBinding()
				binding.ModelID = "gpt-5.6-sol"
				return binding
			}(),
		},
		{
			name: "reasoning", modelID: productRouteTransitionSourceModel,
			reasoningEffort: "high", binding: productRouteTransitionSourceBinding(),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newProductRouteTransitionAcceptanceFixture(t)
			before := fixture.seed(t)
			fixture.resolver.set(productRouteTransitionSourceID, test.binding)
			acknowledgement, err := api.NewLocalProductConversationTrustBoundaryAcknowledgement(
				before.ThreadID, before.Segments[len(before.Segments)-1],
				productRouteTransitionSourceID, test.binding, test.reasoningEffort,
				api.ContextModeSummaryOnly,
			)
			if err != nil {
				t.Fatal(err)
			}
			var after api.LocalProductChatThread
			if err := fixture.client.Call(
				context.Background(),
				"chat_message",
				api.LocalProductChatMessageRequest{
					ThreadID: productRouteTransitionThreadID, Content: "reviewed same-route change",
					ProfileID: productRouteTransitionSourceID, ModelID: test.modelID,
					ReasoningEffort: test.reasoningEffort, ContextMode: api.ContextModeSummaryOnly,
					ExpectedExecutionBinding:     &test.binding,
					TrustBoundaryAcknowledgement: &acknowledgement,
				},
				&after,
			); err != nil {
				t.Fatal(err)
			}
			if len(after.Segments) != 2 || len(after.Attempts) != 2 ||
				len(fixture.responder.requests) != 2 ||
				!reflect.DeepEqual(after.Segments[0], before.Segments[0]) {
				t.Fatalf("same-route transition = %#v", after)
			}
			segment, attempt := after.Segments[1], after.Attempts[1]
			if segment.ModelID != test.modelID ||
				segment.ReasoningEffort != test.reasoningEffort ||
				segment.ContextMode != api.ContextModeSummaryOnly ||
				segment.ExecutionBinding == nil || *segment.ExecutionBinding != test.binding ||
				attempt.SegmentID != segment.SegmentID ||
				attempt.BindingDigest != segment.BindingDigest ||
				segment.RouteTransitionReviewDigest != acknowledgement.ReviewDigest ||
				attempt.RouteTransitionReviewDigest != acknowledgement.ReviewDigest ||
				attempt.ExecutionBinding == nil || *attempt.ExecutionBinding != test.binding {
				t.Fatalf("same-route frozen segment=%#v attempt=%#v", segment, attempt)
			}
		})
	}
}

func TestPhase2DRouteTransitionFreezesProviderAndAccountChangesThroughAuthenticatedIPC(
	t *testing.T,
) {
	for _, test := range []struct {
		name    string
		binding api.LocalProductConversationExecutionBinding
	}{
		{
			name: "provider",
			binding: func() api.LocalProductConversationExecutionBinding {
				binding := productRouteTransitionSourceBinding()
				binding.HarnessAdapter = "loom-native"
				binding.ProviderID = "anthropic"
				binding.ProviderAccountID = "anthropic.primary"
				binding.CredentialRevision = 10
				binding.ProviderAccountPolicyRevision = 4
				binding.ProviderAccountPolicyDigest = strings.Repeat("d", 64)
				return binding
			}(),
		},
		{
			name: "provider account",
			binding: func() api.LocalProductConversationExecutionBinding {
				binding := productRouteTransitionSourceBinding()
				binding.ProviderAccountID = "openai.secondary"
				return binding
			}(),
		},
		{
			name: "credential revision",
			binding: func() api.LocalProductConversationExecutionBinding {
				binding := productRouteTransitionSourceBinding()
				binding.CredentialRevision++
				return binding
			}(),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newProductRouteTransitionAcceptanceFixture(t)
			before := fixture.seed(t)
			fixture.resolver.set(productRouteTransitionSourceID, test.binding)
			acknowledgement := productRouteTransitionTrustAcknowledgement(
				t, before, productRouteTransitionSourceID, test.binding,
				api.ContextModeSummaryOnly,
			)

			var after api.LocalProductChatThread
			if err := fixture.client.Call(
				context.Background(),
				"chat_message",
				api.LocalProductChatMessageRequest{
					ThreadID: productRouteTransitionThreadID, Content: "reviewed " + test.name + " change",
					ProfileID: productRouteTransitionSourceID, ModelID: productRouteTransitionSourceModel,
					ContextMode: api.ContextModeSummaryOnly, ExpectedExecutionBinding: &test.binding,
					TrustBoundaryAcknowledgement: acknowledgement,
				},
				&after,
			); err != nil {
				t.Fatal(err)
			}
			assertProductRouteTransitionSuccess(
				t, before, after, test.binding, api.ContextModeSummaryOnly, "",
			)
		})
	}
}

func TestPhase2DRouteTransitionRequiresIndependentTrustDimensionReviewThroughAuthenticatedIPC(
	t *testing.T,
) {
	for _, test := range []struct {
		name string
		edit func(*api.LocalProductConversationExecutionBinding)
	}{
		{name: "trust domain", edit: func(binding *api.LocalProductConversationExecutionBinding) {
			binding.TrustDomain = "external_provider"
		}},
		{name: "retention", edit: func(binding *api.LocalProductConversationExecutionBinding) {
			binding.RetentionMode = "limited_retention"
		}},
		{name: "region", edit: func(binding *api.LocalProductConversationExecutionBinding) {
			binding.DataRegion = "eu"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newProductRouteTransitionAcceptanceFixture(t)
			before := fixture.seed(t)
			target := productRouteTransitionSourceBinding()
			test.edit(&target)
			fixture.resolver.set(productRouteTransitionSourceID, target)
			callsBefore := len(fixture.responder.requests)

			request := productRouteTransitionMessageWire{
				ThreadID: productRouteTransitionThreadID, Content: "reviewed " + test.name + " change",
				ProfileID: productRouteTransitionSourceID, ModelID: productRouteTransitionSourceModel,
				ContextMode: api.ContextModeSummaryOnly, ExpectedExecutionBinding: &target,
			}
			var ignored api.LocalProductChatThread
			err := fixture.client.Call(context.Background(), "chat_message", request, &ignored)
			assertProductRouteTransitionConflict(t, err)
			if len(fixture.responder.requests) != callsBefore {
				t.Fatalf("responder calls = %d, want %d", len(fixture.responder.requests), callsBefore)
			}
			assertProductRouteTransitionUnchanged(t, before, fixture.readThread(t))

			acknowledgement := productRouteTransitionTrustAcknowledgement(
				t, before, productRouteTransitionSourceID, target, api.ContextModeSummaryOnly,
			)
			request.TrustBoundaryAcknowledgment = acknowledgement
			var after api.LocalProductChatThread
			if err := fixture.client.Call(context.Background(), "chat_message", request, &after); err != nil {
				t.Fatal(err)
			}
			assertProductRouteTransitionSuccess(
				t, before, after, target, api.ContextModeSummaryOnly, "",
			)
			segment := after.Segments[len(after.Segments)-1]
			if segment.RouteTransitionReviewDigest != acknowledgement.ReviewDigest {
				t.Fatalf(
					"route transition review digest = %q, want %q",
					segment.RouteTransitionReviewDigest, acknowledgement.ReviewDigest,
				)
			}
		})
	}
}

func TestPhase2DRouteTransitionRejectsTrustAcknowledgementTamperAndReplayThroughAuthenticatedIPC(
	t *testing.T,
) {
	for _, mutate := range []struct {
		name string
		edit func(*productRouteTransitionTrustAcknowledgementWire)
	}{
		{name: "schema", edit: func(value *productRouteTransitionTrustAcknowledgementWire) {
			value.SchemaVersion++
		}},
		{name: "source segment", edit: func(value *productRouteTransitionTrustAcknowledgementWire) {
			value.SourceSegmentID = "segment-stale"
		}},
		{name: "source binding", edit: func(value *productRouteTransitionTrustAcknowledgementWire) {
			value.SourceBindingDigest = strings.Repeat("0", 64)
		}},
		{name: "target binding", edit: func(value *productRouteTransitionTrustAcknowledgementWire) {
			value.TargetBinding.CredentialRevision++
		}},
		{name: "context mode", edit: func(value *productRouteTransitionTrustAcknowledgementWire) {
			value.ContextMode = api.ContextModeStartClean
		}},
		{name: "acknowledged", edit: func(value *productRouteTransitionTrustAcknowledgementWire) {
			value.Acknowledged = false
		}},
		{name: "review digest", edit: func(value *productRouteTransitionTrustAcknowledgementWire) {
			value.ReviewDigest = strings.Repeat("0", 64)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			fixture := newProductRouteTransitionAcceptanceFixture(t)
			before := fixture.seed(t)
			target := productRouteTransitionTargetBinding()
			fixture.resolver.set(productRouteTransitionTargetID, target)
			acknowledgement := productRouteTransitionTrustAcknowledgement(
				t, before, productRouteTransitionTargetID, target, api.ContextModeSummaryOnly,
			)
			mutate.edit(acknowledgement)
			callsBefore := len(fixture.responder.requests)

			var ignored api.LocalProductChatThread
			err := fixture.client.Call(
				context.Background(),
				"chat_message",
				productRouteTransitionMessageWire{
					ThreadID: productRouteTransitionThreadID, Content: "tampered trust acknowledgement",
					ProfileID: productRouteTransitionTargetID, ModelID: productRouteTransitionTargetModel,
					ContextMode: api.ContextModeSummaryOnly, ExpectedExecutionBinding: &target,
					TrustBoundaryAcknowledgment: acknowledgement,
				},
				&ignored,
			)
			assertProductRouteTransitionConflict(t, err)
			if len(fixture.responder.requests) != callsBefore {
				t.Fatalf("responder calls = %d, want %d", len(fixture.responder.requests), callsBefore)
			}
			assertProductRouteTransitionUnchanged(t, before, fixture.readThread(t))
		})
	}

	t.Run("replay after accepted transition", func(t *testing.T) {
		fixture := newProductRouteTransitionAcceptanceFixture(t)
		before := fixture.seed(t)
		target := productRouteTransitionTargetBinding()
		fixture.resolver.set(productRouteTransitionTargetID, target)
		acknowledgement := productRouteTransitionTrustAcknowledgement(
			t, before, productRouteTransitionTargetID, target, api.ContextModeSummaryOnly,
		)
		request := productRouteTransitionMessageWire{
			ThreadID: productRouteTransitionThreadID, Content: "accepted trust transition",
			ProfileID: productRouteTransitionTargetID, ModelID: productRouteTransitionTargetModel,
			ContextMode: api.ContextModeSummaryOnly, ExpectedExecutionBinding: &target,
			TrustBoundaryAcknowledgment: acknowledgement,
		}
		var accepted api.LocalProductChatThread
		if err := fixture.client.Call(context.Background(), "chat_message", request, &accepted); err != nil {
			t.Fatal(err)
		}
		callsBefore := len(fixture.responder.requests)
		request.Content = "replayed trust transition"
		var ignored api.LocalProductChatThread
		err := fixture.client.Call(context.Background(), "chat_message", request, &ignored)
		assertProductRouteTransitionConflict(t, err)
		if len(fixture.responder.requests) != callsBefore {
			t.Fatalf("responder calls = %d, want %d", len(fixture.responder.requests), callsBefore)
		}
		assertProductRouteTransitionUnchanged(t, accepted, fixture.readThread(t))
	})
}

func assertProductRouteTransitionConflict(t *testing.T, err error) {
	t.Helper()
	var remote *localipc.RemoteError
	if !errors.As(err, &remote) || remote.Code != "conflict" ||
		remote.Stage != "conversation_dispatch" || !remote.Recoverable {
		t.Fatalf("route transition error = %#v, %v", remote, err)
	}
	diagnostic, marshalErr := json.Marshal(struct {
		Error       string `json:"error"`
		Code        string `json:"code"`
		Stage       string `json:"stage"`
		Recoverable bool   `json:"recoverable"`
	}{
		Error: err.Error(), Code: remote.Code,
		Stage: remote.Stage, Recoverable: remote.Recoverable,
	})
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	lower := strings.ToLower(string(diagnostic))
	for _, forbidden := range append(
		[]string{"provider_body", "prompt", "credential"},
		productRouteTransitionDiagnosticSecrets...,
	) {
		if strings.Contains(lower, strings.ToLower(forbidden)) {
			t.Fatalf("private data %q crossed wire diagnostics: %s", forbidden, diagnostic)
		}
	}
}

func assertProductRouteTransitionUnchanged(
	t *testing.T,
	before api.LocalProductChatThread,
	after api.LocalProductChatThread,
) {
	t.Helper()
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("rejected transition mutated thread:\n before=%#v\n after=%#v", before, after)
	}
}

func assertProductRouteTransitionModeDispatch(
	t *testing.T,
	mode api.LocalProductContextMode,
	content string,
	request api.LocalProductConversationRequest,
) {
	t.Helper()
	if request.ContextMode != mode || request.ProfileID != productRouteTransitionTargetID ||
		request.ModelID != productRouteTransitionTargetModel ||
		request.ExecutionBinding == nil ||
		*request.ExecutionBinding != productRouteTransitionTargetBinding() ||
		!validProductHex(request.ContextCapsuleDigest, 64) ||
		!validProductHex(request.DisclosureReceiptDigest, 64) ||
		!validProductHex(request.BindingDigest, 64) ||
		request.SegmentBindingDigest != request.BindingDigest {
		t.Fatalf("responder request = %#v", request)
	}
	switch mode {
	case api.ContextModeContinueWithContext:
		if len(request.Messages) != 2 ||
			!strings.Contains(request.Messages[0].Content, "UNTRUSTED MODEL OUTPUT") ||
			!strings.Contains(request.Messages[0].Content, "route transition response 1") {
			t.Fatalf("continue_with_context dispatch = %#v", request.Messages)
		}
	case api.ContextModeSummaryOnly:
		if len(request.Messages) != 2 ||
			!strings.Contains(request.Messages[0].Content, "summary_only") ||
			strings.Contains(request.Messages[0].Content, "route transition response 1") {
			t.Fatalf("summary_only dispatch = %#v", request.Messages)
		}
	case api.ContextModeStartClean:
		if len(request.Messages) != 1 || request.Messages[0].Content != content {
			t.Fatalf("start_clean dispatch = %#v", request.Messages)
		}
	default:
		t.Fatalf("unexpected context mode %q", mode)
	}
}

func assertProductRouteTransitionSuccess(
	t *testing.T,
	before,
	after api.LocalProductChatThread,
	binding api.LocalProductConversationExecutionBinding,
	mode api.LocalProductContextMode,
	reasoningEffort string,
) {
	t.Helper()
	if after.ThreadID != before.ThreadID ||
		len(after.Segments) != len(before.Segments)+1 ||
		len(after.Attempts) != len(before.Attempts)+1 ||
		!reflect.DeepEqual(after.Segments[:len(before.Segments)], before.Segments) {
		t.Fatalf("route transition continuity before=%#v after=%#v", before, after)
	}
	segment := after.Segments[len(after.Segments)-1]
	attempt := after.Attempts[len(after.Attempts)-1]
	if segment.ExecutionBinding == nil || *segment.ExecutionBinding != binding ||
		segment.ContextMode != mode || segment.ReasoningEffort != reasoningEffort ||
		attempt.SegmentID != segment.SegmentID || attempt.ExecutionBinding == nil ||
		*attempt.ExecutionBinding != binding || attempt.BindingDigest != segment.BindingDigest ||
		!validProductHex(segment.ContextCapsuleDigest, 64) ||
		attempt.ContextCapsuleDigest != segment.ContextCapsuleDigest ||
		!validProductHex(segment.DisclosureReceiptDigest, 64) ||
		attempt.DisclosureReceiptDigest != segment.DisclosureReceiptDigest ||
		!validProductHex(segment.BindingDigest, 64) ||
		attempt.RouteTransitionReviewDigest != segment.RouteTransitionReviewDigest ||
		!validProductHex(segment.RouteTransitionReviewDigest, 64) ||
		!validProductRouteTransitionClientIncidentID(attempt.IncidentID) ||
		attempt.Status != "succeeded" {
		t.Fatalf("frozen segment=%#v attempt=%#v", segment, attempt)
	}
	for _, previous := range before.Attempts {
		if previous.IncidentID == attempt.IncidentID {
			t.Fatalf("route transition reused Incident ID %q", attempt.IncidentID)
		}
	}
	privacySafe, err := json.Marshal(struct {
		Segment api.LocalProductConversationSegment `json:"segment"`
		Attempt api.LocalProductConversationAttempt `json:"attempt"`
	}{Segment: segment, Attempt: attempt})
	if err != nil {
		t.Fatal(err)
	}
	assertProductRouteTransitionPrivacySafe(t, privacySafe)
}

func validProductRouteTransitionClientIncidentID(value string) bool {
	const prefix = "loom-client-"
	if !strings.HasPrefix(value, prefix) || len(value) > 64 {
		return false
	}
	remainder := strings.TrimPrefix(value, prefix)
	separator := strings.LastIndexByte(remainder, '-')
	if separator != 16 || separator == len(remainder)-1 {
		return false
	}
	if decoded, err := hex.DecodeString(remainder[:separator]); err != nil || len(decoded) != 8 {
		return false
	}
	for _, character := range remainder[separator+1:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func assertProductRouteTransitionPrivacySafe(t *testing.T, payload []byte) {
	t.Helper()
	lower := strings.ToLower(string(payload))
	for _, forbidden := range append(
		[]string{"authorization", "provider_body", "full_prompt", "api_key"},
		productRouteTransitionDiagnosticSecrets...,
	) {
		if strings.Contains(lower, strings.ToLower(forbidden)) {
			t.Fatalf("private data %q crossed authority metadata: %s", forbidden, payload)
		}
	}
}

func productRouteTransitionSourceBinding() api.LocalProductConversationExecutionBinding {
	return api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "codex", ProviderID: "openai",
		ProviderAccountID: "openai.primary", CredentialRevision: 9,
		ModelID:                      productRouteTransitionSourceModel,
		ProviderAccountPolicyVersion: 2, ProviderAccountPolicyRevision: 3,
		ProviderAccountPolicyDigest: strings.Repeat("a", 64),
		TrustDomain:                 "enterprise_tenant", RetentionMode: "zero_data_retention",
		DataRegion: "us",
	}
}

func productRouteTransitionSourcePolicyDrift() api.LocalProductConversationExecutionBinding {
	binding := productRouteTransitionSourceBinding()
	binding.ProviderAccountPolicyRevision++
	binding.ProviderAccountPolicyDigest = strings.Repeat("b", 64)
	binding.RetentionMode = "limited_retention"
	return binding
}

func productRouteTransitionTargetBinding() api.LocalProductConversationExecutionBinding {
	return api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "loom-native", ProviderID: "anthropic",
		ProviderAccountID: "anthropic.enterprise", CredentialRevision: 12,
		ModelID:                      productRouteTransitionTargetModel,
		ProviderAccountPolicyVersion: 2, ProviderAccountPolicyRevision: 7,
		ProviderAccountPolicyDigest: strings.Repeat("c", 64),
		TrustDomain:                 "external_provider", RetentionMode: "limited_retention",
		DataRegion: "apac",
	}
}

func productRouteTransitionTrustAcknowledgement(
	t *testing.T,
	thread api.LocalProductChatThread,
	targetProfileID string,
	target api.LocalProductConversationExecutionBinding,
	mode api.LocalProductContextMode,
) *productRouteTransitionTrustAcknowledgementWire {
	t.Helper()
	segment := thread.Segments[len(thread.Segments)-1]
	acknowledgement, err := api.NewLocalProductConversationTrustBoundaryAcknowledgement(
		thread.ThreadID, segment, targetProfileID, target, "", mode,
	)
	if err != nil {
		t.Fatal(err)
	}
	return &acknowledgement
}
