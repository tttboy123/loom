//go:build darwin || linux

package piadapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
)

type piContextRetrieverFixture struct {
	want    contextcapsule.RetrievalProposal
	content []byte
	item    contextcapsule.RetrievedItem
	err     error
	calls   int
}

func (retriever *piContextRetrieverFixture) Retrieve(
	_ context.Context,
	proposal contextcapsule.RetrievalProposal,
) (contextcapsule.RetrievedItem, error) {
	retriever.calls++
	if proposal != retriever.want {
		return contextcapsule.RetrievedItem{}, contextcapsule.ErrContextRetrievalDenied
	}
	if retriever.err != nil {
		return contextcapsule.RetrievedItem{}, retriever.err
	}
	item := retriever.item
	item.Content = append([]byte(nil), retriever.content...)
	return item, nil
}

type piContextDeliveryFixture struct {
	retriever contextcapsule.Retriever
	payload   attemptpayload.Payload
	prepares  int
	acks      int
	proof     attemptpayload.DeliveryProof
}

type piBlockingContextDeliveryFixture struct {
	started  chan struct{}
	canceled chan error
}

func (fixture *piBlockingContextDeliveryFixture) Prepare(
	ctx context.Context,
	_ contextcapsule.RetrievalProposal,
	_ contextcapsule.DeliveryRequest,
	_ contextcapsule.DeliveryEncoder,
) (attemptpayload.Payload, error) {
	close(fixture.started)
	<-ctx.Done()
	fixture.canceled <- ctx.Err()
	return attemptpayload.Payload{}, ctx.Err()
}

func (*piBlockingContextDeliveryFixture) Acknowledge(
	context.Context,
	attemptpayload.Binding,
	attemptpayload.DeliveryProof,
) error {
	return contextcapsule.ErrInvalidContextDelivery
}

func (fixture *piContextDeliveryFixture) Prepare(
	ctx context.Context,
	proposal contextcapsule.RetrievalProposal,
	request contextcapsule.DeliveryRequest,
	encode contextcapsule.DeliveryEncoder,
) (attemptpayload.Payload, error) {
	fixture.prepares++
	item, err := fixture.retriever.Retrieve(ctx, proposal)
	if err != nil {
		return attemptpayload.Payload{}, err
	}
	content, err := encode(proposal, item)
	item.Close()
	if err != nil {
		return attemptpayload.Payload{}, err
	}
	fixture.payload = attemptpayload.Payload{
		Binding: attemptpayload.Binding{
			PayloadID: "payload-pi-context",
			CallID:    contextcapsule.DeliveryCallID(proposal, request.Sequence),
			Sequence:  request.Sequence, ContentType: request.ContentType,
			ContentDigest: piContextDigest(content),
		},
		Status: attemptpayload.StatusPending, Content: append([]byte(nil), content...),
	}
	payload := fixture.payload
	payload.Content = append([]byte(nil), fixture.payload.Content...)
	return payload, nil
}

func (fixture *piContextDeliveryFixture) Acknowledge(
	_ context.Context,
	binding attemptpayload.Binding,
	proof attemptpayload.DeliveryProof,
) error {
	if binding != fixture.payload.Binding || proof != attemptpayload.ProofHarnessFinalOutput {
		return contextcapsule.ErrInvalidContextDelivery
	}
	fixture.acks++
	fixture.proof = proof
	fixture.payload.Status = attemptpayload.StatusDelivered
	return nil
}

func TestPiContextExtensionDefersDeliveryAckUntilValidatedHarnessFinalOutput(t *testing.T) {
	content := []byte("persisted Pi Context")
	proposal := contextcapsule.RetrievalProposal{
		ItemID: "current-task", ContentDigest: piContextDigest(content),
	}
	retriever := &piContextRetrieverFixture{
		want: proposal, content: content,
		item: contextcapsule.RetrievedItem{
			ItemID: proposal.ItemID, Kind: contextcapsule.KindCurrentTaskState,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			ContentDigest: proposal.ContentDigest, SourceType: contextcapsule.SourceAuthority,
			SourceRef: "authority:current-task",
		},
	}
	delivery := &piContextDeliveryFixture{retriever: retriever}
	extension := &piContextExtension{
		capability: strings.Repeat("1", 32), delivery: delivery,
	}
	payload, err := json.Marshal(piContextExtensionRequest{
		SchemaVersion: 1, Capability: extension.capability,
		ItemID: proposal.ItemID, ContentDigest: proposal.ContentDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := extension.handle(context.Background(), payload)
	if !bytes.Contains(response, content) || delivery.prepares != 1 ||
		delivery.acks != 0 || delivery.payload.Status != attemptpayload.StatusPending {
		t.Fatalf("UDS write response=%s delivery=%#v", response, delivery)
	}
	if err := extension.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); err != nil {
		t.Fatal(err)
	}
	if delivery.acks != 1 || delivery.proof != attemptpayload.ProofHarnessFinalOutput ||
		delivery.payload.Status != attemptpayload.StatusDelivered {
		t.Fatalf("Pi final output acknowledgement = %#v", delivery)
	}
}

func TestPiContextExtensionServesOneAttemptBoundNativeToolResultAndCleansUp(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	temporary, err := os.MkdirTemp("/tmp", "loom-pi-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(temporary) })
	content := []byte("bounded observed context for this exact Pi Attempt")
	proposal := contextcapsule.RetrievalProposal{
		ItemID: "diff-detail", ContentDigest: piContextDigest(content),
		ArtifactRef: "artifact:diff-1",
	}
	retriever := &piContextRetrieverFixture{
		want: proposal, content: content,
		item: contextcapsule.RetrievedItem{
			ItemID: proposal.ItemID, Kind: contextcapsule.KindArtifactReference,
			Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
			Priority: contextcapsule.PriorityRetrievable, TokenCount: 8,
			ContentDigest: proposal.ContentDigest,
			SourceType:    contextcapsule.SourceObservation, SourceRef: "evidence:diff-1",
			ArtifactRef: proposal.ArtifactRef,
		},
	}
	extension, err := newPiContextExtension(
		home, temporary, "11111111-1111-4111-8111-111111111111", retriever,
	)
	if err != nil {
		t.Fatal(err)
	}
	root := extension.root
	extensionPath := extension.extensionPath
	socketPath := extension.socketPath
	for path, mode := range map[string]os.FileMode{root: 0o700, extensionPath: 0o600, socketPath: 0o600} {
		info, statErr := os.Lstat(path)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != mode {
			t.Fatalf("private path %s = %#v, %v", path, info, statErr)
		}
	}
	source, err := os.ReadFile(extensionPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(source, []byte(`name: "loom_read_context"`)) ||
		!bytes.Contains(source, []byte(socketPath)) ||
		!bytes.Contains(source, []byte(extension.capability)) ||
		bytes.Contains(source, content) || bytes.Contains(source, []byte("credential")) {
		t.Fatalf("extension source boundary drifted: %s", source)
	}
	if err := extension.BindProcess(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(piContextExtensionRequest{
		SchemaVersion: 1, Capability: extension.capability,
		ItemID: proposal.ItemID, ContentDigest: proposal.ContentDigest,
		ArtifactRef: proposal.ArtifactRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Write(append(request, '\n')); err != nil {
		t.Fatal(err)
	}
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := bufio.NewReader(connection).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	var response struct {
		SchemaVersion int    `json:"schema_version"`
		Status        string `json:"status"`
		ItemID        string `json:"item_id"`
		ContentDigest string `json:"content_digest"`
		Content       string `json:"content"`
	}
	if json.Unmarshal(bytes.TrimSuffix(line, []byte{'\n'}), &response) != nil ||
		response.SchemaVersion != 1 || response.Status != "succeeded" ||
		response.ItemID != proposal.ItemID || response.ContentDigest != proposal.ContentDigest ||
		response.Content != string(content) || retriever.calls != 1 {
		t.Fatalf("response=%s decoded=%#v calls=%d", line, response, retriever.calls)
	}
	extension.Close()
	for _, path := range []string{root, extensionPath, socketPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("private path survived Close: %s err=%v", path, err)
		}
	}
}

func TestPiContextExtensionCloseCancelsAcceptedUnboundConnection(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	temporary, err := os.MkdirTemp("/tmp", "loom-pi-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(temporary) })
	retriever := &piContextRetrieverFixture{}
	extension, err := newPiContextExtension(
		home, temporary, "22222222-2222-4222-8222-222222222222", retriever,
	)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.DialUnix(
		"unix", nil, &net.UnixAddr{Name: extension.socketPath, Net: "unix"},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	closed := make(chan struct{})
	go func() {
		extension.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close waited for the process-binding timeout")
	}
}

func TestPiContextExtensionAttemptCancellationRevokesActiveDeliveryAndResources(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	temporary, err := os.MkdirTemp("/tmp", "loom-pi-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(temporary) })
	delivery := &piBlockingContextDeliveryFixture{
		started: make(chan struct{}), canceled: make(chan error, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	extension, err := newPiContextDeliveryExtensionWithContext(
		ctx,
		home,
		temporary,
		"99999999-9999-4999-8999-999999999999",
		delivery,
	)
	if err != nil {
		t.Fatal(err)
	}
	root := extension.root
	extensionPath := extension.extensionPath
	socketPath := extension.socketPath
	if err := extension.BindProcess(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	connection, err := net.DialUnix(
		"unix", nil, &net.UnixAddr{Name: socketPath, Net: "unix"},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	request, err := json.Marshal(piContextExtensionRequest{
		SchemaVersion: 1,
		Capability:    extension.capability,
		ItemID:        "active-context",
		ContentDigest: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Write(append(request, '\n')); err != nil {
		t.Fatal(err)
	}
	select {
	case <-delivery.started:
	case <-time.After(time.Second):
		t.Fatal("Context delivery did not start")
	}
	cancel()
	select {
	case err := <-delivery.canceled:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Context delivery cancellation = %v", err)
		}
	case <-time.After(time.Second):
		extension.Close()
		t.Fatal("Context delivery survived Attempt cancellation")
	}
	deadline := time.Now().Add(time.Second)
	for {
		removed := true
		for _, path := range []string{root, extensionPath, socketPath} {
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				removed = false
			}
		}
		if removed {
			break
		}
		if time.Now().After(deadline) {
			extension.Close()
			t.Fatal("Pi Context extension resources survived Attempt cancellation")
		}
		time.Sleep(10 * time.Millisecond)
	}
	extension.Close()
}

func TestPiContextExtensionRejectsInsecurePrivateDirectories(t *testing.T) {
	for _, target := range []string{"home", "temporary"} {
		t.Run(target, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "home")
			temporary := filepath.Join(t.TempDir(), "temporary")
			for _, path := range []string{home, temporary} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			path := home
			if target == "temporary" {
				path = temporary
			}
			if err := os.Chmod(path, 0o755); err != nil {
				t.Fatal(err)
			}
			if extension, err := newPiContextExtension(
				home, temporary, "33333333-3333-4333-8333-333333333333",
				&piContextRetrieverFixture{},
			); !errors.Is(err, ErrPiContextExtension) || extension != nil {
				t.Fatalf("newPiContextExtension() = %#v, %v", extension, err)
			}
		})
	}
}

func TestPiContextExtensionRejectsNonCanonicalAttemptIDBeforeFilesystemWrite(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	temporary := filepath.Join(t.TempDir(), "temporary")
	for _, path := range []string{home, temporary} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{
		"", "../escape", "11111111111141118111111111111111",
		"11111111-1111-1111-8111-111111111111",
		"11111111-1111-4111-7111-111111111111",
		"11111111-1111-4111-8111-11111111111g",
		"11111111-1111-4111-8111-111111111111/escape",
	} {
		if extension, err := newPiContextExtension(
			home, temporary, id, &piContextRetrieverFixture{},
		); !errors.Is(err, ErrPiContextExtension) || extension != nil {
			t.Fatalf("id %q = %#v, %v", id, extension, err)
		}
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid ID wrote private state: %#v, %v", entries, err)
	}
}

func TestPiContextExtensionRejectsProtocolAndRetrievedItemDrift(t *testing.T) {
	content := []byte("bounded context")
	proposal := contextcapsule.RetrievalProposal{
		ItemID: "item-1", ContentDigest: piContextDigest(content),
	}
	baseItem := contextcapsule.RetrievedItem{
		ItemID: proposal.ItemID, Kind: contextcapsule.KindCurrentTaskState,
		Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
		Priority: contextcapsule.PriorityConfirmed, TokenCount: 3,
		ContentDigest: proposal.ContentDigest,
		SourceType:    contextcapsule.SourceAuthority, SourceRef: "work-item:1",
	}
	tests := []struct {
		name      string
		payload   func(string) []byte
		mutate    func(*contextcapsule.RetrievedItem)
		wantCalls int
	}{
		{name: "wrong capability", payload: func(_ string) []byte {
			return []byte(`{"schema_version":1,"capability":"wrong","item_id":"item-1","content_digest":"` + proposal.ContentDigest + `"}`)
		}},
		{name: "unknown field", payload: func(capability string) []byte {
			return []byte(`{"schema_version":1,"capability":"` + capability + `","item_id":"item-1","content_digest":"` + proposal.ContentDigest + `","authority":"forged"}`)
		}},
		{name: "duplicate item", payload: func(capability string) []byte {
			return []byte(`{"schema_version":1,"capability":"` + capability + `","item_id":"item-1","item_id":"item-1","content_digest":"` + proposal.ContentDigest + `"}`)
		}},
		{name: "body digest drift", payload: func(capability string) []byte {
			body, _ := json.Marshal(piContextExtensionRequest{
				SchemaVersion: 1, Capability: capability, ItemID: proposal.ItemID,
				ContentDigest: proposal.ContentDigest,
			})
			return body
		}, mutate: func(item *contextcapsule.RetrievedItem) { item.ContentDigest = strings.Repeat("a", 64) }, wantCalls: 1},
		{name: "credential classification", payload: func(capability string) []byte {
			body, _ := json.Marshal(piContextExtensionRequest{
				SchemaVersion: 1, Capability: capability, ItemID: proposal.ItemID,
				ContentDigest: proposal.ContentDigest,
			})
			return body
		}, mutate: func(item *contextcapsule.RetrievedItem) {
			item.Kind = contextcapsule.KindCredentialReference
			item.SourceType = contextcapsule.SourceCredentialReference
		}, wantCalls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := baseItem
			if test.mutate != nil {
				test.mutate(&item)
			}
			retriever := &piContextRetrieverFixture{want: proposal, content: content, item: item}
			extension := &piContextExtension{capability: strings.Repeat("1", 32), retriever: retriever}
			response := extension.handle(context.Background(), test.payload(extension.capability))
			var denied map[string]any
			if json.Unmarshal(response, &denied) != nil || denied["status"] != "denied" ||
				denied["reason"] != "context_retrieval_denied" || retriever.calls != test.wantCalls {
				t.Fatalf("response=%s calls=%d want=%d", response, retriever.calls, test.wantCalls)
			}
		})
	}
}

func TestPiContextExtensionSourceIsDeterministicAndContentFree(t *testing.T) {
	one := piContextExtensionSource("/private/attempt.sock", strings.Repeat("1", 32))
	two := piContextExtensionSource("/private/attempt.sock", strings.Repeat("1", 32))
	if !reflect.DeepEqual(one, two) || !bytes.Contains(one, []byte("additionalProperties: false")) ||
		!bytes.Contains(one, []byte("executionMode: \"sequential\"")) ||
		bytes.Contains(one, []byte("API_KEY")) || bytes.Contains(one, []byte("Authorization")) {
		t.Fatalf("extension source drifted: %s", one)
	}
}

func TestPiContextExtensionUsesPrivateShortSocketRootForDarwinLengthBoundary(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	temporary := filepath.Join(t.TempDir(), strings.Repeat("long", 20))
	for _, path := range []string{home, temporary} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	extension, err := newPiContextExtension(
		home, temporary, "44444444-4444-4444-8444-444444444444",
		&piContextRetrieverFixture{},
	)
	if err != nil {
		t.Fatal(err)
	}
	socketRoot := extension.socketRoot
	if socketRoot == "" || filepath.Dir(extension.socketPath) != socketRoot ||
		len(extension.socketPath) > piContextExtensionSocketLimit {
		t.Fatalf("short socket boundary = root:%q path:%q", socketRoot, extension.socketPath)
	}
	info, err := os.Lstat(socketRoot)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 ||
		!piLocalCurrentUserOwns(info) {
		t.Fatalf("short socket root is not private: %#v, %v", info, err)
	}
	extension.Close()
	if _, err := os.Lstat(socketRoot); !os.IsNotExist(err) {
		t.Fatalf("short socket root survived Close: %v", err)
	}
}
