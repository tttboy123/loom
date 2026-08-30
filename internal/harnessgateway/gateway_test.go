package harnessgateway

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBuiltInRegistryRequiresFiveVersionedHarnesses(t *testing.T) {
	registrations := builtInFixtureRegistrations()
	registry, err := NewBuiltInBackendRegistry(registrations)
	if err != nil {
		t.Fatal(err)
	}
	for _, harnessID := range BuiltInHarnessIDs() {
		configured, backend, ok := registry.Resolve(harnessID)
		if !ok || backend == nil || configured.HarnessID != harnessID ||
			configured.SchemaVersion != ConfiguredHarnessSchemaVersion ||
			configured.Version <= 0 || configured.BackendID == "" ||
			configured.BackendVersion <= 0 || configured.ConfigurationDigest == "" {
			t.Fatalf("resolve(%s) = %#v, %T, %t", harnessID, configured, backend, ok)
		}
	}

	for _, mutation := range []func([]Registration) []Registration{
		func(values []Registration) []Registration { return values[:len(values)-1] },
		func(values []Registration) []Registration {
			values[1].Configured.HarnessID = values[0].Configured.HarnessID
			return values
		},
		func(values []Registration) []Registration {
			values[0].Configured.BackendVersion++
			return values
		},
	} {
		copy := append([]Registration(nil), registrations...)
		if _, err := NewBuiltInBackendRegistry(mutation(copy)); !errors.Is(err, ErrInvalidRegistry) {
			t.Fatalf("invalid registry error = %v", err)
		}
	}
}

func TestBuiltInRegistryCopiesConfiguredAuthorityAcrossResolve(t *testing.T) {
	registrations := builtInFixtureRegistrations()
	registry, err := NewBuiltInBackendRegistry(registrations)
	if err != nil {
		t.Fatal(err)
	}
	want, _, ok := registry.Resolve(HarnessCodex)
	if !ok {
		t.Fatal("Codex registration missing")
	}
	registrations[0].Configured.Version++
	registrations[0].Configured.Capabilities[0] = Capability("mutated-input")
	first, _, ok := registry.Resolve(HarnessCodex)
	if !ok || first.Version != want.Version ||
		len(first.Capabilities) != len(want.Capabilities) ||
		first.Capabilities[0] != want.Capabilities[0] {
		t.Fatalf("registry authority changed through input alias: %#v", first)
	}
	first.Capabilities[0] = Capability("mutated-output")
	second, _, ok := registry.Resolve(HarnessCodex)
	if !ok || second.Capabilities[0] != want.Capabilities[0] {
		t.Fatalf("registry authority changed through resolved alias: %#v", second)
	}
}

func TestGatewayReusesExactSegmentSessionAndFreezesEveryResponse(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	events := &eventSinkFixture{}
	gateway, err := New(Config{Registry: registry, Events: events, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	secondGateway, err := New(Config{Registry: registry, Events: &eventSinkFixture{}, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	defer secondGateway.Close(context.Background())
	if gateway.instanceID == "" || gateway.instanceID == secondGateway.instanceID {
		t.Fatalf("Gateway instance IDs are not unique: %q / %q", gateway.instanceID, secondGateway.instanceID)
	}

	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	binding.RouteTransitionReviewDigest = digest64("reviewed-transition")
	workspace := workspaceFixture("workspace-a")
	first := responseFixture(binding, "response-1")
	second := responseFixture(binding, "response-2")
	first.Authority.ContextCapsuleDigest = digest64("attempt-capsule-1")
	second.Authority.ContextCapsuleDigest = digest64("attempt-capsule-2")
	if _, err := gateway.Respond(context.Background(), binding, workspace, first); err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.Respond(context.Background(), binding, workspace, second); err != nil {
		t.Fatal(err)
	}
	if backend.opened.Load() != 1 || backend.responses.Load() != 2 {
		t.Fatalf("opened=%d responses=%d", backend.opened.Load(), backend.responses.Load())
	}
	session := backend.lastSession()
	if session == nil || len(session.authorities) != 2 ||
		session.authorities[0].ContextCapsuleDigest == session.authorities[1].ContextCapsuleDigest ||
		session.authorities[0].SegmentContextCapsuleDigest != binding.SegmentContextCapsuleDigest ||
		session.authorities[1].SegmentContextCapsuleDigest != binding.SegmentContextCapsuleDigest ||
		session.authorities[0].ExecutionBindingDigest != binding.ExecutionBindingDigest ||
		session.authorities[1].CredentialRevision != binding.CredentialRevision {
		t.Fatalf("session = %#v", session)
	}
	assertLifecycleEvents(t, events.snapshot(), []EventType{
		EventSessionOpening, EventSessionReady,
		EventResponseStarted, EventResponseCompleted,
		EventResponseStarted, EventResponseCompleted,
	})
	for _, event := range events.snapshot() {
		if event.WorkspaceDigest != binding.WorkspaceDigest ||
			event.ExecutionBindingDigest != binding.ExecutionBindingDigest ||
			event.ProviderID != binding.ProviderID ||
			event.ProviderAccountID != binding.ProviderAccountID ||
			event.CredentialRevision != binding.CredentialRevision ||
			event.ModelID != binding.ModelID ||
			event.ReasoningEffort != binding.ReasoningEffort ||
			event.SegmentContextCapsuleDigest != binding.SegmentContextCapsuleDigest ||
			event.GovernancePolicyDigest != binding.GovernancePolicyDigest {
			t.Fatalf("event lost frozen Session authority: %#v", event)
		}
		if event.Type == EventResponseStarted || event.Type == EventResponseCompleted {
			wantCapsule := first.Authority.ContextCapsuleDigest
			if event.ResponseID == second.Authority.ResponseID {
				wantCapsule = second.Authority.ContextCapsuleDigest
			}
			if event.ContextCapsuleDigest != wantCapsule {
				t.Fatalf("event Attempt Capsule = %q, want %q", event.ContextCapsuleDigest, wantCapsule)
			}
		} else if event.ContextCapsuleDigest != "" {
			t.Fatalf("Session event carried Attempt Capsule authority: %#v", event)
		}
	}
}

func TestGatewayRejectsCompletedResponseReplayBeforeBackendCall(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	gateway := gatewayForSingleHarness(t, HarnessCodex, backend)
	defer gateway.Close(context.Background())
	binding := segmentBindingFixture(
		HarnessCodex, "conversation-replay", "segment-1", "workspace-a",
	)
	request := responseFixture(binding, "response-replay")
	if _, err := gateway.Respond(
		context.Background(), binding, workspaceFixture("workspace-a"), request,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.Respond(
		context.Background(), binding, workspaceFixture("workspace-a"), request,
	); !errors.Is(err, ErrAuthorityConflict) {
		t.Fatalf("completed Response replay error = %v", err)
	}
	if backend.opened.Load() != 1 || backend.responses.Load() != 1 {
		t.Fatalf(
			"completed replay reached Backend: opens=%d responses=%d",
			backend.opened.Load(), backend.responses.Load(),
		)
	}
}

func TestGatewayOrdinaryResponseFailureKeepsExactSessionReady(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.responseErr = errors.New("provider rejected this response")
	events := &eventSinkFixture{}
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{Registry: registry, Events: events, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	binding := segmentBindingFixture(HarnessCodex, "conversation-ordinary", "segment-1", "workspace-a")
	workspace := workspaceFixture("workspace-a")
	if _, err := gateway.Respond(
		context.Background(), binding, workspace, responseFixture(binding, "response-failed"),
	); err == nil || errors.Is(err, ErrSessionUnhealthy) {
		t.Fatalf("ordinary response error = %v", err)
	}
	backend.responseErr = nil
	if _, err := gateway.Respond(
		context.Background(), binding, workspace, responseFixture(binding, "response-next"),
	); err != nil {
		t.Fatal(err)
	}
	if backend.opened.Load() != 1 {
		t.Fatalf("sessions opened = %d, want exact Session reuse", backend.opened.Load())
	}
	if hasEvent(events.snapshot(), EventSessionFailed, "") {
		t.Fatalf("ordinary response error failed Session: %#v", events.snapshot())
	}
}

func TestGatewayUnhealthyResponseFailsAndReopensOnlyExactSession(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.responseErr = errors.Join(ErrSessionUnhealthy, errors.New("native protocol lost"))
	events := &eventSinkFixture{}
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{Registry: registry, Events: events, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	binding := segmentBindingFixture(HarnessCodex, "conversation-unhealthy", "segment-1", "workspace-a")
	workspace := workspaceFixture("workspace-a")
	if _, err := gateway.Respond(
		context.Background(), binding, workspace, responseFixture(binding, "response-fatal"),
	); !errors.Is(err, ErrSessionUnhealthy) {
		t.Fatalf("unhealthy response error = %v", err)
	}
	failed := backend.lastSession()
	if failed == nil || failed.closeCalls.Load() != 1 {
		t.Fatalf("failed Session = %#v", failed)
	}
	backend.responseErr = nil
	if _, err := gateway.Respond(
		context.Background(), binding, workspace, responseFixture(binding, "response-reopened"),
	); err != nil {
		t.Fatal(err)
	}
	if backend.opened.Load() != 2 {
		t.Fatalf("sessions opened = %d, want failed Session replacement", backend.opened.Load())
	}
	assertLifecycleEvents(t, events.snapshot(), []EventType{
		EventSessionOpening, EventSessionReady,
		EventResponseStarted, EventResponseFailed, EventSessionClosing, EventSessionFailed,
		EventSessionOpening, EventSessionReady,
		EventResponseStarted, EventResponseCompleted,
	})
}

func TestGatewayUnhealthySessionFinishesCleanupBeforeTerminalEventAndExactReopen(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.responseErr = errors.Join(ErrSessionUnhealthy, errors.New("native protocol lost"))
	backend.closeBlock = make(chan struct{})
	var releaseOnce sync.Once
	releaseClose := func() { releaseOnce.Do(func() { close(backend.closeBlock) }) }
	defer releaseClose()
	events := &eventSinkFixture{}
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{Registry: registry, Events: events, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	binding := segmentBindingFixture(HarnessCodex, "conversation-cleanup", "segment-1", "workspace-a")
	workspace := workspaceFixture("workspace-a")
	firstDone := make(chan error, 1)
	go func() {
		_, respondErr := gateway.Respond(
			context.Background(), binding, workspace, responseFixture(binding, "response-fatal"),
		)
		firstDone <- respondErr
	}()
	<-backend.openStarted
	<-backend.started
	failed := backend.lastSession()
	select {
	case <-failed.closeStarted:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("unhealthy Session cleanup did not start")
	}
	terminalBeforeCleanup := hasEvent(events.snapshot(), EventSessionFailed, "")
	backend.responseErr = nil
	secondDone := make(chan error, 1)
	go func() {
		_, respondErr := gateway.Respond(
			context.Background(), binding, workspace, responseFixture(binding, "response-reopened"),
		)
		secondDone <- respondErr
	}()
	reopenedBeforeCleanup := false
	select {
	case <-backend.openStarted:
		reopenedBeforeCleanup = true
	case <-time.After(40 * time.Millisecond):
	}
	releaseClose()
	if err := <-firstDone; !errors.Is(err, ErrSessionUnhealthy) {
		t.Fatalf("unhealthy response error = %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("reopened response error = %v", err)
	}
	if terminalBeforeCleanup {
		t.Fatal("SessionFailed was emitted before native Session cleanup completed")
	}
	if reopenedBeforeCleanup {
		t.Fatal("exact Session reopened before prior native Session cleanup completed")
	}
	if !hasEvent(events.snapshot(), EventSessionFailed, "") {
		t.Fatal("missing terminal SessionFailed after cleanup")
	}
}

func TestGatewayOpeningFailureHasTerminalSessionEvent(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.openErr = errors.New("native session failed to open")
	events := &eventSinkFixture{}
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{Registry: registry, Events: events, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	binding := segmentBindingFixture(HarnessCodex, "conversation-open-failed", "segment-1", "workspace-a")
	if _, err := gateway.Respond(
		context.Background(), binding, workspaceFixture("workspace-a"),
		responseFixture(binding, "response-never-started"),
	); err == nil {
		t.Fatal("opening failure unexpectedly succeeded")
	}
	assertLifecycleEvents(t, events.snapshot(), []EventType{
		EventSessionOpening, EventSessionFailed,
	})
}

func TestGatewayBoundsSessionOpeningAndReleasesExactSlotForRetry(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	neverOpen := make(chan struct{})
	backend.openBlocks["conversation-open-timeout"] = neverOpen
	backend.openCanceled = make(chan struct{}, 1)
	events := &eventSinkFixture{}
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{
		Registry: registry, Events: events, Now: fixedGatewayTime,
		SessionOpenTimeout: 40 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	binding := segmentBindingFixture(
		HarnessCodex, "conversation-open-timeout", "segment-1", "workspace-a",
	)
	callerCtx, cancelCaller := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelCaller()
	startedAt := time.Now()
	if _, err := gateway.Respond(
		callerCtx, binding, workspaceFixture("workspace-a"),
		responseFixture(binding, "response-open-timeout"),
	); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded Session opening error = %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 250*time.Millisecond {
		t.Fatalf("bounded Session opening took %s", elapsed)
	}
	select {
	case <-backend.openCanceled:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Session opening timeout did not cancel the Backend opener")
	}
	assertLifecycleEvents(t, events.snapshot(), []EventType{
		EventSessionOpening, EventSessionFailed,
	})

	delete(backend.openBlocks, binding.ConversationID)
	if _, err := gateway.Respond(
		context.Background(), binding, workspaceFixture("workspace-a"),
		responseFixture(binding, "response-open-retry"),
	); err != nil {
		t.Fatalf("retry after bounded Session opening = %v", err)
	}
	if backend.opened.Load() != 2 {
		t.Fatalf("Session opens = %d, want timed-out opening plus retry", backend.opened.Load())
	}
}

func TestGatewayCleansLateSessionReturnedAfterOpeningDeadlineBeforeRetry(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.openBlocks["conversation-late-open"] = make(chan struct{})
	backend.openCanceled = make(chan struct{}, 1)
	backend.openIgnoresCancellation = true
	events := &eventSinkFixture{}
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{
		Registry: registry, Events: events, Now: fixedGatewayTime,
		SessionOpenTimeout: 40 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	binding := segmentBindingFixture(
		HarnessCodex, "conversation-late-open", "segment-1", "workspace-a",
	)
	callerCtx, cancelCaller := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelCaller()
	if _, err := gateway.Respond(
		callerCtx, binding, workspaceFixture("workspace-a"),
		responseFixture(binding, "response-late-open"),
	); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("late Session opening error = %v", err)
	}
	select {
	case <-backend.openCanceled:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("late Backend opener did not observe its deadline")
	}
	late := backend.lastSession()
	if late == nil {
		t.Fatal("fixture did not return a late Backend Session")
	}
	select {
	case <-late.closeStarted:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("late Backend Session cleanup did not start")
	}

	delete(backend.openBlocks, binding.ConversationID)
	backend.openIgnoresCancellation = false
	if _, err := gateway.Respond(
		context.Background(), binding, workspaceFixture("workspace-a"),
		responseFixture(binding, "response-after-late-open"),
	); err != nil {
		t.Fatalf("retry after late Session cleanup = %v", err)
	}
	if late.closeCalls.Load() != 1 || !late.closed || backend.opened.Load() != 2 {
		t.Fatalf(
			"late cleanup calls=%d closed=%t opens=%d",
			late.closeCalls.Load(), late.closed, backend.opened.Load(),
		)
	}
	assertLifecycleEvents(t, events.snapshot(), []EventType{
		EventSessionOpening, EventSessionClosing, EventSessionFailed,
		EventSessionOpening, EventSessionReady,
		EventResponseStarted, EventResponseCompleted,
	})
}

func TestGatewayCloseFailureEndsWithTerminalSessionFailure(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.closeErr = errors.New("native session cleanup failed")
	events := &eventSinkFixture{}
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{Registry: registry, Events: events, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	binding := segmentBindingFixture(HarnessCodex, "conversation-close-failed", "segment-1", "workspace-a")
	if _, err := gateway.Respond(
		context.Background(), binding, workspaceFixture("workspace-a"),
		responseFixture(binding, "response-complete"),
	); err != nil {
		t.Fatal(err)
	}
	if err := gateway.Close(context.Background()); !errors.Is(err, backend.closeErr) {
		t.Fatalf("Gateway.Close error = %v", err)
	}
	assertLifecycleEvents(t, events.snapshot(), []EventType{
		EventSessionOpening, EventSessionReady,
		EventResponseStarted, EventResponseCompleted,
		EventSessionClosing, EventSessionFailed,
	})
}

func TestSegmentSessionIDIncludesFrozenContextCapsuleDigest(t *testing.T) {
	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	substituted := binding
	substituted.SegmentContextCapsuleDigest = digest64("substituted-capsule")

	if binding.SessionID() == substituted.SessionID() {
		t.Fatal("SessionID did not change with the frozen Context Capsule digest")
	}
}

func TestGatewayRejectsResponseAuthoritySubstitutionWithoutBackendCall(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	gateway := gatewayForSingleHarness(t, HarnessCodex, backend)
	defer gateway.Close(context.Background())
	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	workspace := workspaceFixture("workspace-a")

	mutations := []func(*ResponseRequest){
		func(request *ResponseRequest) { request.Authority.SegmentContextCapsuleDigest = digest64("other") },
		func(request *ResponseRequest) { request.Authority.ProviderAccountID = "account-other" },
		func(request *ResponseRequest) { request.Authority.CredentialRevision++ },
		func(request *ResponseRequest) { request.Authority.ModelID = "model-other" },
		func(request *ResponseRequest) { request.Authority.ExecutionBindingDigest = digest64("other") },
		func(request *ResponseRequest) { request.Authority.GovernancePolicyDigest = digest64("other") },
		func(request *ResponseRequest) { request.Authority.RouteTransitionReviewDigest = digest64("other") },
	}
	for index, mutate := range mutations {
		request := responseFixture(binding, "response-substitution-"+string(rune('a'+index)))
		mutate(&request)
		if _, err := gateway.Respond(context.Background(), binding, workspace, request); !errors.Is(err, ErrAuthorityConflict) {
			t.Fatalf("mutation %d error = %v", index, err)
		}
	}
	if backend.opened.Load() != 0 || backend.responses.Load() != 0 {
		t.Fatalf("backend reached opened=%d responses=%d", backend.opened.Load(), backend.responses.Load())
	}
}

func TestGatewaySerializesOneSessionButRunsDifferentConversationsConcurrently(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.block = make(chan struct{})
	gateway := gatewayForSingleHarness(t, HarnessCodex, backend)
	defer gateway.Close(context.Background())

	firstBinding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	secondBinding := segmentBindingFixture(HarnessCodex, "conversation-2", "segment-1", "workspace-b")
	done := make(chan error, 3)
	go func() {
		_, err := gateway.Respond(context.Background(), firstBinding, workspaceFixture("workspace-a"), responseFixture(firstBinding, "response-1"))
		done <- err
	}()
	<-backend.started
	go func() {
		_, err := gateway.Respond(context.Background(), firstBinding, workspaceFixture("workspace-a"), responseFixture(firstBinding, "response-2"))
		done <- err
	}()
	go func() {
		_, err := gateway.Respond(context.Background(), secondBinding, workspaceFixture("workspace-b"), responseFixture(secondBinding, "response-3"))
		done <- err
	}()
	<-backend.started
	select {
	case <-backend.started:
		t.Fatal("same Segment response ran concurrently")
	case <-time.After(40 * time.Millisecond):
	}
	if backend.maximumActive.Load() != 2 {
		t.Fatalf("maximum active = %d, want 2", backend.maximumActive.Load())
	}
	close(backend.block)
	for range 3 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestGatewayQueuedSameSessionResponseHonorsContextBeforeStarting(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.block = make(chan struct{})
	events := &eventSinkFixture{}
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{Registry: registry, Events: events, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())

	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	workspace := workspaceFixture("workspace-a")
	firstDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(context.Background(), binding, workspace, responseFixture(binding, "response-1"))
		firstDone <- err
	}()
	<-backend.started

	queuedCtx, cancelQueued := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancelQueued()
	queuedDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(queuedCtx, binding, workspace, responseFixture(binding, "response-queued"))
		queuedDone <- err
	}()

	select {
	case err := <-queuedDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("queued Response error = %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		close(backend.block)
		<-firstDone
		<-queuedDone
		t.Fatal("queued Response did not return when its context expired")
	}
	if backend.responses.Load() != 1 {
		t.Fatalf("backend responses = %d, want only the active Response", backend.responses.Load())
	}
	if hasEvent(events.snapshot(), EventResponseStarted, "response-queued") {
		t.Fatalf("queued Response emitted response_started: %#v", events.snapshot())
	}

	close(backend.block)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
}

func TestGatewayCoalescesConcurrentSameSessionOpen(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	openRelease := make(chan struct{})
	backend.openBlocks["conversation-1"] = openRelease
	gateway := gatewayForSingleHarness(t, HarnessCodex, backend)
	defer gateway.Close(context.Background())

	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	done := make(chan error, 2)
	for _, responseID := range []string{"response-1", "response-2"} {
		go func() {
			_, err := gateway.Respond(context.Background(), binding, workspaceFixture("workspace-a"), responseFixture(binding, responseID))
			done <- err
		}()
	}
	if conversationID := <-backend.openStarted; conversationID != binding.ConversationID {
		t.Fatalf("opened conversation = %q", conversationID)
	}
	select {
	case conversationID := <-backend.openStarted:
		t.Fatalf("duplicate same-Session open for %q", conversationID)
	case <-time.After(40 * time.Millisecond):
	}
	close(openRelease)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if backend.opened.Load() != 1 {
		t.Fatalf("sessions opened = %d, want 1", backend.opened.Load())
	}
}

func TestGatewayCoalescedOpenOutlivesLeaderCancellationAndWaiterDeadline(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	openRelease := make(chan struct{})
	backend.openBlocks["conversation-1"] = openRelease
	events := &eventSinkFixture{}
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{Registry: registry, Events: events, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())

	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	workspace := workspaceFixture("workspace-a")
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(leaderCtx, binding, workspace, responseFixture(binding, "response-leader"))
		leaderDone <- err
	}()
	<-backend.openStarted

	deadlineBase, cancelDeadline := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancelDeadline()
	deadlineCtx := newDoneObservedContext(deadlineBase)
	deadlineDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(deadlineCtx, binding, workspace, responseFixture(binding, "response-deadline"))
		deadlineDone <- err
	}()
	<-deadlineCtx.observed

	validCtx := newDoneObservedContext(context.Background())
	validDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(validCtx, binding, workspace, responseFixture(binding, "response-valid"))
		validDone <- err
	}()
	<-validCtx.observed
	cancelLeader()

	if err := receiveError(t, leaderDone, "opening leader"); !errors.Is(err, context.Canceled) {
		t.Fatalf("opening leader error = %v", err)
	}
	if err := receiveError(t, deadlineDone, "opening waiter"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("opening waiter error = %v", err)
	}
	select {
	case err := <-validDone:
		close(openRelease)
		t.Fatalf("valid waiter returned before Gateway opening completed: %v", err)
	case <-time.After(40 * time.Millisecond):
	}

	close(openRelease)
	if err := receiveError(t, validDone, "valid opening waiter"); err != nil {
		t.Fatal(err)
	}
	if backend.opened.Load() != 1 || backend.responses.Load() != 1 {
		t.Fatalf("opened=%d responses=%d, want one coalesced open and one valid Response", backend.opened.Load(), backend.responses.Load())
	}
	for _, responseID := range []string{"response-leader", "response-deadline"} {
		if hasEvent(events.snapshot(), EventResponseStarted, responseID) {
			t.Fatalf("cancelled opening caller emitted response_started for %q", responseID)
		}
	}
}

func TestGatewayCancelsExactAdmittedResponseWhileSessionIsOpening(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.openBlocks["conversation-1"] = make(chan struct{})
	gateway := gatewayForSingleHarness(t, HarnessCodex, backend)
	defer gateway.Close(context.Background())

	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	request := responseFixture(binding, "response-opening-cancel")
	done := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(context.Background(), binding, workspaceFixture("workspace-a"), request)
		done <- err
	}()
	<-backend.openStarted

	if err := gateway.CancelResponse(context.Background(), CancelRequest{
		SessionID: binding.SessionID(), ResponseID: request.Authority.ResponseID,
		IncidentID: request.Authority.IncidentID,
	}); err != nil {
		t.Fatalf("cancel opening Response: %v", err)
	}
	if err := receiveError(t, done, "opening Response"); !errors.Is(err, context.Canceled) {
		t.Fatalf("opening Response error = %v, want cancelled", err)
	}
	if backend.responses.Load() != 0 {
		t.Fatalf("cancelled opening Response reached BackendSession.Respond %d times", backend.responses.Load())
	}
	if err := gateway.CancelResponse(context.Background(), CancelRequest{
		SessionID: binding.SessionID(), ResponseID: request.Authority.ResponseID,
		IncidentID: request.Authority.IncidentID,
	}); !errors.Is(err, ErrResponseNotFound) {
		t.Fatalf("terminal Response cancellation error = %v, want not found", err)
	}
}

func TestGatewaySessionLimitCountsOpeningSessions(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	openRelease := make(chan struct{})
	backend.openBlocks["conversation-1"] = openRelease
	configured := configuredHarnessFixture(HarnessCodex, backend.ID(), backend.Version(), 1)
	configured.MaxConcurrentSessions = 1
	gateway := gatewayForConfiguredHarness(t, configured, backend, fixedGatewayTime)

	firstBinding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	secondBinding := segmentBindingFixture(HarnessCodex, "conversation-2", "segment-1", "workspace-b")
	firstDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(context.Background(), firstBinding, workspaceFixture("workspace-a"), responseFixture(firstBinding, "response-1"))
		firstDone <- err
	}()
	<-backend.openStarted

	if _, err := gateway.Respond(context.Background(), secondBinding, workspaceFixture("workspace-b"), responseFixture(secondBinding, "response-2")); err == nil {
		t.Fatal("second Session opened above MaxConcurrentSessions")
	}
	select {
	case conversationID := <-backend.openStarted:
		t.Fatalf("over-limit Session reached backend for %q", conversationID)
	default:
	}

	close(openRelease)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := gateway.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestGatewayReopensIdleExpiredSession(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	clock := newGatewayClock(fixedGatewayTime())
	configured := configuredHarnessFixture(HarnessCodex, backend.ID(), backend.Version(), 1)
	configured.IdleTimeout = 5 * time.Minute
	configured.MaxSessionAge = time.Hour
	gateway := gatewayForConfiguredHarness(t, configured, backend, clock.Now)
	defer gateway.Close(context.Background())

	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	workspace := workspaceFixture("workspace-a")
	if _, err := gateway.Respond(context.Background(), binding, workspace, responseFixture(binding, "response-1")); err != nil {
		t.Fatal(err)
	}
	first := backend.lastSession()
	clock.Advance(configured.IdleTimeout)
	if _, err := gateway.Respond(context.Background(), binding, workspace, responseFixture(binding, "response-2")); err != nil {
		t.Fatal(err)
	}
	if backend.opened.Load() != 2 {
		t.Fatalf("sessions opened = %d, want 2", backend.opened.Load())
	}
	if first.closeCalls.Load() != 1 {
		t.Fatalf("expired Session close calls = %d, want 1", first.closeCalls.Load())
	}
}

func TestGatewayMaxAgeExpiryDrainsActiveSessionBeforeReopen(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.block = make(chan struct{})
	clock := newGatewayClock(fixedGatewayTime())
	configured := configuredHarnessFixture(HarnessCodex, backend.ID(), backend.Version(), 1)
	configured.IdleTimeout = time.Minute
	configured.MaxSessionAge = 10 * time.Minute
	gateway := gatewayForConfiguredHarness(t, configured, backend, clock.Now)
	defer gateway.Close(context.Background())

	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	workspace := workspaceFixture("workspace-a")
	firstDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(context.Background(), binding, workspace, responseFixture(binding, "response-1"))
		firstDone <- err
	}()
	<-backend.started
	first := backend.lastSession()
	clock.Advance(configured.MaxSessionAge)

	secondDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(context.Background(), binding, workspace, responseFixture(binding, "response-2"))
		secondDone <- err
	}()
	select {
	case <-first.closeStarted:
		t.Fatal("active expired Session closed before its Response drained")
	case <-time.After(40 * time.Millisecond):
	}
	close(backend.block)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	if first.closeCalls.Load() != 1 || backend.opened.Load() != 2 {
		t.Fatalf("close calls=%d opened=%d, want 1 and 2", first.closeCalls.Load(), backend.opened.Load())
	}
}

func TestGatewayConcurrentCloseCancelsAndWaitsForOperations(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.block = make(chan struct{})
	openingRelease := make(chan struct{})
	backend.openBlocks["conversation-2"] = openingRelease
	backend.closeBlock = make(chan struct{})
	gateway := gatewayForSingleHarness(t, HarnessCodex, backend)

	activeBinding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	openingBinding := segmentBindingFixture(HarnessCodex, "conversation-2", "segment-1", "workspace-b")
	activeDone := make(chan error, 1)
	openingDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(context.Background(), activeBinding, workspaceFixture("workspace-a"), responseFixture(activeBinding, "response-1"))
		activeDone <- err
	}()
	<-backend.openStarted
	<-backend.started
	activeSession := backend.lastSession()
	go func() {
		_, err := gateway.Respond(context.Background(), openingBinding, workspaceFixture("workspace-b"), responseFixture(openingBinding, "response-2"))
		openingDone <- err
	}()
	<-backend.openStarted

	closeResults := make(chan error, 2)
	go func() { closeResults <- gateway.Close(context.Background()) }()
	go func() { closeResults <- gateway.Close(context.Background()) }()
	if err := <-activeDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("active Response error = %v", err)
	}
	if err := <-openingDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("opening Session error = %v", err)
	}
	select {
	case <-activeSession.closeStarted:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Gateway.Close did not progress to Session close")
	}
	select {
	case err := <-closeResults:
		t.Fatalf("Gateway.Close returned before Session close completed: %v", err)
	default:
	}
	close(backend.closeBlock)
	for range 2 {
		if err := <-closeResults; err != nil {
			t.Fatal(err)
		}
	}
	if activeSession.closeCalls.Load() != 1 {
		t.Fatalf("Session close calls = %d, want 1", activeSession.closeCalls.Load())
	}
}

func TestGatewayCloseDeadlineReturnsWhileOpeningCleanupContinues(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	openRelease := make(chan struct{})
	cleanupRelease := make(chan struct{})
	var releaseOnce sync.Once
	releaseCleanup := func() { releaseOnce.Do(func() { close(cleanupRelease) }) }
	defer releaseCleanup()
	backend.openBlocks["conversation-1"] = openRelease
	backend.openCanceled = make(chan struct{}, 1)
	backend.openCancelBlock = cleanupRelease
	gateway := gatewayForSingleHarness(t, HarnessCodex, backend)

	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	responseDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(context.Background(), binding, workspaceFixture("workspace-a"), responseFixture(binding, "response-1"))
		responseDone <- err
	}()
	<-backend.openStarted

	closeCtx, cancelClose := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancelClose()
	firstClose := make(chan error, 1)
	go func() { firstClose <- gateway.Close(closeCtx) }()
	select {
	case <-backend.openCanceled:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Gateway.Close did not cancel the opening Session")
	}
	select {
	case err := <-firstClose:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("deadline Close error = %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		releaseCleanup()
		<-responseDone
		<-firstClose
		t.Fatal("Gateway.Close exceeded its caller deadline")
	}

	finalClose := make(chan error, 1)
	go func() { finalClose <- gateway.Close(context.Background()) }()
	select {
	case err := <-finalClose:
		t.Fatalf("later Gateway.Close returned before opening cleanup: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	releaseCleanup()
	if err := <-responseDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("opening Response error = %v", err)
	}
	if err := <-finalClose; err != nil {
		t.Fatal(err)
	}
	if err := gateway.Close(context.Background()); err != nil {
		t.Fatalf("completed Gateway.Close result = %v", err)
	}
	if backend.opened.Load() != 1 {
		t.Fatalf("Session opens = %d, want exactly 1", backend.opened.Load())
	}
}

func TestGatewayCloseDeadlineDoesNotPoisonFinalSessionCloseResult(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	gateway := gatewayForSingleHarness(t, HarnessCodex, backend)
	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	if _, err := gateway.Respond(context.Background(), binding, workspaceFixture("workspace-a"), responseFixture(binding, "response-1")); err != nil {
		t.Fatal(err)
	}
	session := backend.lastSession()
	backend.closeBlock = make(chan struct{})

	closeCtx, cancelClose := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancelClose()
	if err := gateway.Close(closeCtx); !errors.Is(err, context.DeadlineExceeded) {
		close(backend.closeBlock)
		t.Fatalf("deadline Close error = %v", err)
	}
	select {
	case <-session.closeStarted:
	case <-time.After(200 * time.Millisecond):
		close(backend.closeBlock)
		t.Fatal("Gateway cleanup did not start Backend Session close")
	}

	finalClose := make(chan error, 1)
	go func() { finalClose <- gateway.Close(context.Background()) }()
	select {
	case err := <-finalClose:
		close(backend.closeBlock)
		t.Fatalf("later Gateway.Close returned before Session cleanup: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	close(backend.closeBlock)
	if err := <-finalClose; err != nil {
		t.Fatal(err)
	}
	if session.closeCalls.Load() != 1 || !session.closed {
		t.Fatalf("Session close calls=%d closed=%t, want exactly-once completion", session.closeCalls.Load(), session.closed)
	}
	if err := gateway.Close(context.Background()); err != nil {
		t.Fatalf("completed Gateway.Close result = %v", err)
	}
}

func TestGatewayBoundsBackendSessionCleanupIndependentlyOfCallerDeadline(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.closeBlock = make(chan struct{})
	defer close(backend.closeBlock)
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{
		Registry: registry, Events: &eventSinkFixture{}, Now: fixedGatewayTime,
		SessionCloseTimeout: 40 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding := segmentBindingFixture(HarnessCodex, "conversation-bounded-close", "segment-1", "workspace-a")
	if _, err := gateway.Respond(
		context.Background(), binding, workspaceFixture("workspace-a"), responseFixture(binding, "response-1"),
	); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Now()
	if err := gateway.Close(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded Gateway.Close error = %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
		t.Fatalf("bounded Gateway.Close took %s", elapsed)
	}
}

func TestGatewayOpensDifferentSessionsIndependently(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	firstOpenRelease := make(chan struct{})
	backend.openBlocks["conversation-1"] = firstOpenRelease
	gateway := gatewayForSingleHarness(t, HarnessCodex, backend)
	defer gateway.Close(context.Background())

	firstBinding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	secondBinding := segmentBindingFixture(HarnessCodex, "conversation-2", "segment-1", "workspace-b")
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(context.Background(), firstBinding, workspaceFixture("workspace-a"), responseFixture(firstBinding, "response-1"))
		firstDone <- err
	}()
	if conversationID := <-backend.openStarted; conversationID != firstBinding.ConversationID {
		t.Fatalf("first opened conversation = %q", conversationID)
	}
	go func() {
		_, err := gateway.Respond(context.Background(), secondBinding, workspaceFixture("workspace-b"), responseFixture(secondBinding, "response-2"))
		secondDone <- err
	}()

	select {
	case conversationID := <-backend.openStarted:
		if conversationID != secondBinding.ConversationID {
			t.Fatalf("second opened conversation = %q", conversationID)
		}
	case <-time.After(200 * time.Millisecond):
		close(firstOpenRelease)
		<-firstDone
		t.Fatal("different Session open blocked behind the first open")
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	close(firstOpenRelease)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
}

func TestCancelResponseRemainsResponsiveWhileAnotherSessionOpens(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	responseRelease := make(chan struct{})
	secondOpenRelease := make(chan struct{})
	backend.block = responseRelease
	backend.openBlocks["conversation-2"] = secondOpenRelease
	gateway := gatewayForSingleHarness(t, HarnessCodex, backend)
	defer gateway.Close(context.Background())

	firstBinding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	secondBinding := segmentBindingFixture(HarnessCodex, "conversation-2", "segment-1", "workspace-b")
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(context.Background(), firstBinding, workspaceFixture("workspace-a"), responseFixture(firstBinding, "response-1"))
		firstDone <- err
	}()
	<-backend.openStarted
	<-backend.started
	go func() {
		_, err := gateway.Respond(context.Background(), secondBinding, workspaceFixture("workspace-b"), responseFixture(secondBinding, "response-2"))
		secondDone <- err
	}()
	if conversationID := <-backend.openStarted; conversationID != secondBinding.ConversationID {
		t.Fatalf("opening conversation = %q", conversationID)
	}

	cancelDone := make(chan error, 1)
	go func() {
		cancelDone <- gateway.CancelResponse(context.Background(), CancelRequest{
			SessionID: firstBinding.SessionID(), ResponseID: "response-1", IncidentID: "incident-response-1",
		})
	}()
	select {
	case err := <-cancelDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(200 * time.Millisecond):
		close(secondOpenRelease)
		close(responseRelease)
		<-cancelDone
		<-firstDone
		<-secondDone
		t.Fatal("cancellation blocked behind another Session open")
	}
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled response error = %v", err)
	}
	close(secondOpenRelease)
	<-backend.started
	close(responseRelease)
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
}

func TestCancelResponseTerminatesOnlyExactResponseAndKeepsSessionReady(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.block = make(chan struct{})
	events := &eventSinkFixture{}
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{Registry: registry, Events: events, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())

	bindingA := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	bindingB := segmentBindingFixture(HarnessCodex, "conversation-2", "segment-1", "workspace-b")
	doneA := make(chan error, 1)
	doneB := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(context.Background(), bindingA, workspaceFixture("workspace-a"), responseFixture(bindingA, "response-a"))
		doneA <- err
	}()
	<-backend.started
	go func() {
		_, err := gateway.Respond(context.Background(), bindingB, workspaceFixture("workspace-b"), responseFixture(bindingB, "response-b"))
		doneB <- err
	}()
	<-backend.started

	if err := gateway.CancelResponse(context.Background(), CancelRequest{
		SessionID: bindingA.SessionID(), ResponseID: "response-a", IncidentID: "incident-response-a",
	}); err != nil {
		t.Fatal(err)
	}
	if err := <-doneA; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled response error = %v", err)
	}
	select {
	case err := <-doneB:
		t.Fatalf("peer response terminated: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	close(backend.block)
	if err := <-doneB; err != nil {
		t.Fatal(err)
	}

	backend.block = nil
	if _, err := gateway.Respond(context.Background(), bindingA, workspaceFixture("workspace-a"), responseFixture(bindingA, "response-a-next")); err != nil {
		t.Fatalf("session not reusable after response cancellation: %v", err)
	}
	if backend.opened.Load() != 2 {
		t.Fatalf("sessions opened = %d, want one per conversation", backend.opened.Load())
	}
	if !hasEvent(events.snapshot(), EventResponseCancelled, "response-a") {
		t.Fatalf("missing content-free cancel event: %#v", events.snapshot())
	}
}

func TestCancelResponseRejectsWrongStartedIncidentID(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	backend.block = make(chan struct{})
	gateway := gatewayForSingleHarness(t, HarnessCodex, backend)
	defer gateway.Close(context.Background())

	binding := segmentBindingFixture(HarnessCodex, "conversation-1", "segment-1", "workspace-a")
	done := make(chan error, 1)
	go func() {
		_, err := gateway.Respond(context.Background(), binding, workspaceFixture("workspace-a"), responseFixture(binding, "response-1"))
		done <- err
	}()
	<-backend.started

	wrongErr := gateway.CancelResponse(context.Background(), CancelRequest{
		SessionID: binding.SessionID(), ResponseID: "response-1", IncidentID: "incident-other",
	})
	if !errors.Is(wrongErr, ErrResponseNotFound) {
		if wrongErr == nil {
			<-done
		}
		t.Fatalf("wrong IncidentID cancellation error = %v", wrongErr)
	}
	select {
	case err := <-done:
		t.Fatalf("wrong IncidentID cancelled response: %v", err)
	case <-time.After(40 * time.Millisecond):
	}

	if err := gateway.CancelResponse(context.Background(), CancelRequest{
		SessionID: binding.SessionID(), ResponseID: "response-1", IncidentID: "incident-response-1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled response error = %v", err)
	}
}

func TestGatewayEventsAreContentFreeAndMonotonic(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	events := &eventSinkFixture{}
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{Registry: registry, Events: events, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	binding := segmentBindingFixture(HarnessCodex, "conversation-private", "segment-private", "workspace-private")
	request := responseFixture(binding, "response-private")
	request.Input = []byte("private prompt must not enter events")
	if _, err := gateway.Respond(context.Background(), binding, workspaceFixture("workspace-private"), request); err != nil {
		t.Fatal(err)
	}
	var previous uint64
	for _, event := range events.snapshot() {
		if !event.Valid() || event.SchemaVersion != EventSchemaVersion ||
			event.GatewayInstanceID != gateway.instanceID ||
			event.ConfiguredHarnessVersion != binding.ConfiguredHarnessVersion ||
			event.BackendVersion != binding.BackendVersion || event.Sequence <= previous ||
			event.WorkspacePath != "" ||
			event.Content != "" || event.ProviderBody != "" {
			t.Fatalf("unsafe event = %#v", event)
		}
		previous = event.Sequence
	}
}

func TestGatewayResponseFailureEventUsesOnlyValidatedSafeClassification(t *testing.T) {
	tests := []struct {
		name       string
		classifier ResponseFailureClassifier
		want       ResponseFailure
	}{
		{
			name: "exact safe failure",
			classifier: func(error) (ResponseFailure, bool) {
				return ResponseFailure{
					Code: "provider_insufficient_balance", Stage: "provider_http",
					HTTPStatus: 402, ProviderCode: "insufficient_balance",
				}, true
			},
			want: ResponseFailure{
				Code: "provider_insufficient_balance", Stage: "provider_http",
				HTTPStatus: 402, ProviderCode: "insufficient_balance",
			},
		},
		{
			name: "invalid classification falls back",
			classifier: func(error) (ResponseFailure, bool) {
				return ResponseFailure{
					Code: "provider_unavailable", Stage: "provider_http",
					ProviderCode: "unsafe provider body\nleak", Retryable: true,
				}, true
			},
			want: ResponseFailure{
				Code: "provider_unavailable", Stage: "conversation_dispatch", Retryable: true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend := newBackendFixture("backend.codex", 1)
			backend.responseErr = errors.New("private backend failure")
			events := &eventSinkFixture{}
			registry := registryForSingleHarness(t, HarnessCodex, backend)
			gateway, err := New(Config{
				Registry: registry, Events: events, Now: fixedGatewayTime,
				ClassifyFailure: test.classifier,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer gateway.Close(context.Background())
			binding := segmentBindingFixture(
				HarnessCodex, "conversation-failure-event", "segment-1", "workspace-a",
			)
			if _, err := gateway.Respond(
				context.Background(), binding, workspaceFixture("workspace-a"),
				responseFixture(binding, "response-failure-event"),
			); err == nil {
				t.Fatal("failed response unexpectedly succeeded")
			}
			var terminal *Event
			for _, event := range events.snapshot() {
				if event.Type == EventResponseFailed {
					candidate := event
					terminal = &candidate
					break
				}
			}
			if terminal == nil || !terminal.Valid() || terminal.Failure != test.want ||
				terminal.Content != "" || terminal.ProviderBody != "" {
				t.Fatalf("terminal failure event = %#v, want %#v", terminal, test.want)
			}
		})
	}
}

func TestGatewayConcurrentEventsReachSinkInSequenceOrder(t *testing.T) {
	backend := newBackendFixture("backend.codex", 1)
	events := newFirstEventBlockingSink()
	registry := registryForSingleHarness(t, HarnessCodex, backend)
	gateway, err := New(Config{Registry: registry, Events: events, Now: fixedGatewayTime})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())

	done := make(chan error, 2)
	for index := range 2 {
		binding := segmentBindingFixture(
			HarnessCodex,
			"conversation-ordered-"+string(rune('a'+index)),
			"segment-1",
			"workspace-"+string(rune('a'+index)),
		)
		go func() {
			_, err := gateway.Respond(
				context.Background(), binding, workspaceFixture(binding.WorkspaceID),
				responseFixture(binding, "response-"+string(rune('a'+index))),
			)
			done <- err
		}()
	}
	<-events.firstEntered
	time.Sleep(40 * time.Millisecond)
	close(events.releaseFirst)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	var previous uint64
	for _, event := range events.snapshot() {
		if event.Sequence <= previous {
			t.Fatalf("events reached sink out of order: %#v", events.snapshot())
		}
		previous = event.Sequence
	}
}

func builtInFixtureRegistrations() []Registration {
	values := make([]Registration, 0, len(BuiltInHarnessIDs()))
	for index, harnessID := range BuiltInHarnessIDs() {
		backend := newBackendFixture("backend."+string(harnessID), 1)
		values = append(values, Registration{
			Configured: configuredHarnessFixture(harnessID, backend.ID(), backend.Version(), index+1),
			Backend:    backend,
		})
	}
	return values
}

func registryForSingleHarness(t *testing.T, harnessID HarnessID, backend *backendFixture) *BackendRegistry {
	t.Helper()
	registry, err := NewBackendRegistry([]Registration{{
		Configured: configuredHarnessFixture(harnessID, backend.ID(), backend.Version(), 1),
		Backend:    backend,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func gatewayForSingleHarness(t *testing.T, harnessID HarnessID, backend *backendFixture) *Gateway {
	t.Helper()
	gateway, err := New(Config{
		Registry: registryForSingleHarness(t, harnessID, backend),
		Events:   &eventSinkFixture{}, Now: fixedGatewayTime,
	})
	if err != nil {
		t.Fatal(err)
	}
	return gateway
}

func gatewayForConfiguredHarness(t *testing.T, configured ConfiguredHarness, backend *backendFixture, now func() time.Time) *Gateway {
	t.Helper()
	registry, err := NewBackendRegistry([]Registration{{Configured: configured, Backend: backend}})
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := New(Config{Registry: registry, Events: &eventSinkFixture{}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return gateway
}

func configuredHarnessFixture(harnessID HarnessID, backendID BackendID, backendVersion, version int) ConfiguredHarness {
	return ConfiguredHarness{
		SchemaVersion: ConfiguredHarnessSchemaVersion,
		HarnessID:     harnessID, Version: version,
		BackendID: backendID, BackendVersion: backendVersion,
		ConfigurationDigest:   digest64("configuration:" + string(harnessID)),
		Capabilities:          []Capability{CapabilitySegmentSession, CapabilityResponseCancel},
		MaxConcurrentSessions: 8,
		IdleTimeout:           5 * time.Minute, MaxSessionAge: time.Hour,
	}
}

func segmentBindingFixture(harnessID HarnessID, conversationID, segmentID, workspaceID string) SegmentSessionBinding {
	return SegmentSessionBinding{
		SchemaVersion:       SegmentSessionBindingSchemaVersion,
		ConfiguredHarnessID: harnessID, ConfiguredHarnessVersion: 1,
		BackendID: BackendID("backend." + string(harnessID)), BackendVersion: 1,
		ConversationID: conversationID, SegmentID: segmentID,
		WorkspaceID: workspaceID, WorkspaceDigest: digest64(workspaceID),
		ExecutionBindingDigest: digest64("binding:" + conversationID + ":" + segmentID),
		ProviderID:             "openai", ProviderAccountID: "openai.primary",
		CredentialRevision: 7, ModelID: "gpt-5.6-sol",
		SegmentContextCapsuleDigest: digest64("segment-capsule:" + segmentID),
		GovernancePolicyDigest:      digest64("policy:" + conversationID),
	}
}

func workspaceFixture(id string) Workspace {
	return Workspace{ID: id, Digest: digest64(id), Path: "/private/tmp/" + id}
}

func responseFixture(binding SegmentSessionBinding, responseID string) ResponseRequest {
	return ResponseRequest{
		Authority: ResponseAuthority{
			SchemaVersion: ResponseAuthoritySchemaVersion,
			ResponseID:    responseID, IncidentID: "incident-" + responseID,
			ExecutionBindingDigest:      binding.ExecutionBindingDigest,
			ContextCapsuleDigest:        binding.SegmentContextCapsuleDigest,
			SegmentContextCapsuleDigest: binding.SegmentContextCapsuleDigest,
			GovernancePolicyDigest:      binding.GovernancePolicyDigest,
			RouteTransitionReviewDigest: binding.RouteTransitionReviewDigest,
			ProviderID:                  binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
			CredentialRevision: binding.CredentialRevision, ModelID: binding.ModelID,
		},
		Input: []byte("bounded input"),
	}
}

func digest64(value string) string {
	const alphabet = "0123456789abcdef"
	result := make([]byte, 64)
	for index := range result {
		result[index] = alphabet[(int(value[index%len(value)])+index)%len(alphabet)]
	}
	return string(result)
}

func fixedGatewayTime() time.Time { return time.Unix(1_787_553_600, 0).UTC() }

type backendFixture struct {
	id                      BackendID
	version                 int
	opened                  atomic.Int64
	responses               atomic.Int64
	active                  atomic.Int64
	maximumActive           atomic.Int64
	openStarted             chan string
	openBlocks              map[string]<-chan struct{}
	openCanceled            chan struct{}
	openCancelBlock         <-chan struct{}
	openIgnoresCancellation bool
	openErr                 error
	started                 chan struct{}
	block                   chan struct{}
	responseErr             error
	closeBlock              chan struct{}
	closeErr                error
	mu                      sync.Mutex
	sessions                []*sessionFixture
}

func newBackendFixture(id string, version int) *backendFixture {
	return &backendFixture{
		id: BackendID(id), version: version,
		openStarted: make(chan string, 16), openBlocks: make(map[string]<-chan struct{}),
		started: make(chan struct{}, 16),
	}
}

func (backend *backendFixture) ID() BackendID { return backend.id }
func (backend *backendFixture) Version() int  { return backend.version }
func (backend *backendFixture) OpenSession(ctx context.Context, _ ConfiguredHarness, binding SegmentSessionBinding, workspace Workspace) (BackendSession, error) {
	backend.opened.Add(1)
	backend.openStarted <- binding.ConversationID
	if block := backend.openBlocks[binding.ConversationID]; block != nil {
		select {
		case <-ctx.Done():
			if backend.openCanceled != nil {
				backend.openCanceled <- struct{}{}
			}
			if backend.openCancelBlock != nil {
				<-backend.openCancelBlock
			}
			if !backend.openIgnoresCancellation {
				return nil, ctx.Err()
			}
		case <-block:
		}
	}
	if backend.openErr != nil {
		return nil, backend.openErr
	}
	session := &sessionFixture{
		backend: backend, binding: binding, workspace: workspace,
		closeStarted: make(chan struct{}),
	}
	backend.mu.Lock()
	backend.sessions = append(backend.sessions, session)
	backend.mu.Unlock()
	return session, nil
}

func (backend *backendFixture) lastSession() *sessionFixture {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if len(backend.sessions) == 0 {
		return nil
	}
	return backend.sessions[len(backend.sessions)-1]
}

type sessionFixture struct {
	backend      *backendFixture
	binding      SegmentSessionBinding
	workspace    Workspace
	mu           sync.Mutex
	authorities  []ResponseAuthority
	closed       bool
	closeCalls   atomic.Int64
	closeStarted chan struct{}
}

func (session *sessionFixture) Respond(ctx context.Context, request ResponseRequest) (Response, error) {
	session.backend.responses.Add(1)
	active := session.backend.active.Add(1)
	for {
		maximum := session.backend.maximumActive.Load()
		if active <= maximum || session.backend.maximumActive.CompareAndSwap(maximum, active) {
			break
		}
	}
	defer session.backend.active.Add(-1)
	session.mu.Lock()
	session.authorities = append(session.authorities, request.Authority)
	session.mu.Unlock()
	session.backend.started <- struct{}{}
	if session.backend.block != nil {
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		case <-session.backend.block:
		}
	}
	if session.backend.responseErr != nil {
		return Response{}, session.backend.responseErr
	}
	return Response{Content: []byte("bounded response")}, nil
}

func (session *sessionFixture) Close(ctx context.Context) error {
	session.closeCalls.Add(1)
	close(session.closeStarted)
	if session.backend.closeBlock != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-session.backend.closeBlock:
		}
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	session.closed = true
	return session.backend.closeErr
}

type gatewayClock struct {
	mu  sync.Mutex
	now time.Time
}

func newGatewayClock(now time.Time) *gatewayClock { return &gatewayClock{now: now} }

func (clock *gatewayClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *gatewayClock) Advance(elapsed time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(elapsed)
	clock.mu.Unlock()
}

type eventSinkFixture struct {
	mu     sync.Mutex
	events []Event
}

type firstEventBlockingSink struct {
	mu           sync.Mutex
	events       []Event
	firstEntered chan struct{}
	releaseFirst chan struct{}
}

func newFirstEventBlockingSink() *firstEventBlockingSink {
	return &firstEventBlockingSink{
		firstEntered: make(chan struct{}), releaseFirst: make(chan struct{}),
	}
}

func (sink *firstEventBlockingSink) Record(event Event) {
	if event.Sequence == 1 {
		close(sink.firstEntered)
		<-sink.releaseFirst
	}
	sink.mu.Lock()
	sink.events = append(sink.events, event)
	sink.mu.Unlock()
}

func (sink *firstEventBlockingSink) snapshot() []Event {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	return append([]Event(nil), sink.events...)
}

func (sink *eventSinkFixture) Record(event Event) {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.events = append(sink.events, event)
}
func (sink *eventSinkFixture) snapshot() []Event {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	return append([]Event(nil), sink.events...)
}

func assertLifecycleEvents(t *testing.T, events []Event, types []EventType) {
	t.Helper()
	if len(events) != len(types) {
		t.Fatalf("events = %#v", events)
	}
	for index := range types {
		if events[index].Type != types[index] {
			t.Fatalf("event %d = %#v", index, events[index])
		}
	}
}

func hasEvent(events []Event, eventType EventType, responseID string) bool {
	for _, event := range events {
		if event.Type == eventType && event.ResponseID == responseID {
			return true
		}
	}
	return false
}

type doneObservedContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func newDoneObservedContext(ctx context.Context) *doneObservedContext {
	return &doneObservedContext{Context: ctx, observed: make(chan struct{})}
}

func (ctx *doneObservedContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.observed) })
	return ctx.Context.Done()
}

func receiveError(t *testing.T, result <-chan error, operation string) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("%s did not return promptly", operation)
		return nil
	}
}
