//go:build unix

package piadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const piRPCFixturePrompt = "Correct: func add(a, b int) int { return a - b }"

const piRPCFixtureSettings = `{"compaction":{"enabled":false},"retry":{"enabled":false,"maxRetries":0,"baseDelayMs":0,"provider":{"maxRetries":0,"maxRetryDelayMs":0}}}` + "\n"

type piRPCEventRejectingFrameSink struct {
	frames []bridgev1.Frame
	err    error
}

func TestPiRPCModelOutputBudget(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	runtimeAdapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	adapter, ok := runtimeAdapter.(*piRPCBridgeAdapter)
	if !ok {
		t.Fatal("Pi RPC adapter concrete type changed")
	}
	content, err := adapter.modelsJSON()
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Providers map[string]struct {
			Models []struct {
				ContextWindow int `json:"contextWindow"`
				MaxTokens     int `json:"maxTokens"`
			} `json:"models"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	provider, ok := document.Providers[piRPCProviderID]
	if !ok || len(document.Providers) != 1 || len(provider.Models) != 1 {
		t.Fatal("Pi RPC model declaration is not exact")
	}
	model := provider.Models[0]
	if model.ContextWindow != 32768 || model.MaxTokens != 256 {
		t.Fatalf(
			"Pi RPC model budget = context:%d output:%d, want 32768/256",
			model.ContextWindow,
			model.MaxTokens,
		)
	}
}

func (sink *piRPCEventRejectingFrameSink) AcceptFrame(
	_ context.Context,
	frame bridgev1.Frame,
) error {
	sink.frames = append(sink.frames, frame)
	if frame.Type() == bridgev1.MessageEvent {
		return sink.err
	}
	return nil
}

func TestPiRPCPi0821UserMessageCompatibility(t *testing.T) {
	prompt := "bounded prompt"
	for name, raw := range map[string][]byte{
		"integer timestamp": []byte(`{"role":"user","content":[{"type":"text","text":"bounded prompt"}],"timestamp":1}`),
		"zero timestamp":    []byte(`{"role":"user","content":[{"type":"text","text":"bounded prompt"}],"timestamp":0}`),
		"fractional timestamp": []byte(
			`{"role":"user","content":[{"type":"text","text":"bounded prompt"}],"timestamp":1.5}`,
		),
	} {
		t.Run("accept/"+name, func(t *testing.T) {
			if !piRPCUserMessage(raw, prompt) {
				t.Fatal("exact Pi 0.82.1 user message was rejected")
			}
		})
	}

	rejections := map[string][]byte{
		"legacy string":        []byte(`{"role":"user","content":"bounded prompt","timestamp":1}`),
		"null content":         []byte(`{"role":"user","content":null,"timestamp":1}`),
		"scalar content":       []byte(`{"role":"user","content":1,"timestamp":1}`),
		"object content":       []byte(`{"role":"user","content":{"type":"text","text":"bounded prompt"},"timestamp":1}`),
		"empty array":          []byte(`{"role":"user","content":[],"timestamp":1}`),
		"multiple blocks":      []byte(`{"role":"user","content":[{"type":"text","text":"bounded prompt"},{"type":"text","text":"bounded prompt"}],"timestamp":1}`),
		"image block":          []byte(`{"role":"user","content":[{"type":"image","data":"x"}],"timestamp":1}`),
		"tool block":           []byte(`{"role":"user","content":[{"type":"tool","text":"bounded prompt"}],"timestamp":1}`),
		"thinking block":       []byte(`{"role":"user","content":[{"type":"thinking","text":"bounded prompt"}],"timestamp":1}`),
		"missing outer role":   []byte(`{"content":[{"type":"text","text":"bounded prompt"}],"timestamp":1}`),
		"extra outer field":    []byte(`{"role":"user","content":[{"type":"text","text":"bounded prompt"}],"timestamp":1,"extra":true}`),
		"duplicate outer role": []byte(`{"role":"user","role":"user","content":[{"type":"text","text":"bounded prompt"}],"timestamp":1}`),
		"missing block type":   []byte(`{"role":"user","content":[{"text":"bounded prompt"}],"timestamp":1}`),
		"missing block text":   []byte(`{"role":"user","content":[{"type":"text"}],"timestamp":1}`),
		"extra block field":    []byte(`{"role":"user","content":[{"type":"text","text":"bounded prompt","textSignature":"x"}],"timestamp":1}`),
		"duplicate block type": []byte(`{"role":"user","content":[{"type":"text","type":"text","text":"bounded prompt"}],"timestamp":1}`),
		"changed prompt":       []byte(`{"role":"user","content":[{"type":"text","text":"other"}],"timestamp":1}`),
		"prefixed prompt":      []byte(`{"role":"user","content":[{"type":"text","text":" bounded prompt"}],"timestamp":1}`),
		"suffixed prompt":      []byte(`{"role":"user","content":[{"type":"text","text":"bounded prompt "}],"timestamp":1}`),
		"negative timestamp":   []byte(`{"role":"user","content":[{"type":"text","text":"bounded prompt"}],"timestamp":-1}`),
		"string timestamp":     []byte(`{"role":"user","content":[{"type":"text","text":"bounded prompt"}],"timestamp":"1"}`),
		"null timestamp":       []byte(`{"role":"user","content":[{"type":"text","text":"bounded prompt"}],"timestamp":null}`),
		"missing timestamp":    []byte(`{"role":"user","content":[{"type":"text","text":"bounded prompt"}]}`),
		"malformed JSON":       []byte(`{"role":"user"`),
		"invalid UTF-8":        append([]byte(`{"role":"user","content":[{"type":"text","text":"`), 0xff),
	}
	for name, raw := range rejections {
		t.Run("reject/"+name, func(t *testing.T) {
			if piRPCUserMessage(raw, prompt) {
				t.Fatal("invalid Pi user message was accepted")
			}
		})
	}
}

func TestPiRPCBridgeCreatesExactNoRetrySettingsBeforePrompt(t *testing.T) {
	t.Run("exact settings exist before prompt", func(t *testing.T) {
		fixture := newPiRPCBridgeFixture(t, "require-settings")
		adapter, err := NewPiRPCBridgeAdapter(fixture.config())
		if err != nil {
			t.Fatal(err)
		}
		request := fixture.request(t)
		if _, err := adapter.Execute(context.Background(), request); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		settingsPath := filepath.Join(fixture.homePath, ".pi", "agent", "settings.json")
		content, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatal("private settings were not materialized")
		}
		if string(content) != piRPCFixtureSettings {
			t.Fatal("private settings bytes are not exact")
		}
		info, err := os.Lstat(settingsPath)
		if err != nil ||
			!info.Mode().IsRegular() ||
			info.Mode()&os.ModeSymlink != 0 ||
			info.Mode().Perm() != 0o600 {
			t.Fatal("private settings binding is invalid")
		}
	})

	for _, existing := range []string{"regular", "symlink", "directory"} {
		t.Run("pre-existing "+existing+" is not replaced", func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, "settings-start-marker")
			agentPath := filepath.Join(fixture.homePath, ".pi", "agent")
			for _, path := range []string{
				filepath.Join(fixture.homePath, ".pi"),
				agentPath,
			} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			settingsPath := filepath.Join(agentPath, "settings.json")
			switch existing {
			case "regular":
				if err := os.WriteFile(settingsPath, []byte("preserve"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(agentPath, "target")
				if err := os.WriteFile(target, []byte("preserve"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, settingsPath); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(settingsPath, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			request := fixture.request(t)
			if _, err := adapter.Execute(context.Background(), request); !errors.Is(err, ErrPiRPCProtocol) {
				t.Fatalf("Execute() error = %v, want ErrPiRPCProtocol", err)
			}
			if _, err := os.Lstat(filepath.Join(fixture.homePath, "process-started")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("Pi process started despite a pre-existing settings object")
			}
			info, err := os.Lstat(settingsPath)
			if err != nil {
				t.Fatal("pre-existing settings object was removed")
			}
			switch existing {
			case "regular":
				content, readErr := os.ReadFile(settingsPath)
				if readErr != nil || string(content) != "preserve" {
					t.Fatal("pre-existing regular settings were changed")
				}
			case "symlink":
				if info.Mode()&os.ModeSymlink == 0 {
					t.Fatal("pre-existing settings symlink was replaced")
				}
			case "directory":
				if !info.IsDir() {
					t.Fatal("pre-existing settings directory was replaced")
				}
			}
		})
	}
}

func TestPiRPCBridgeTranslatesCorrelatedTranscript(t *testing.T) {
	t.Setenv("SHOULD_NOT_LEAK", "ambient-secret")
	fixture := newPiRPCBridgeFixture(t, "success")
	adapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatalf("NewPiRPCBridgeAdapter() error = %v", err)
	}
	if adapter.AdapterType() != "pi-cli" ||
		adapter.RuntimeInstanceID() != fixture.binding.RuntimeInstanceID {
		t.Fatalf("adapter identity = %q/%q", adapter.AdapterType(), adapter.RuntimeInstanceID())
	}

	request := fixture.request(t)
	sink := request.FrameSink.(*piRecordingFrameSink)
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExitCode() != 0 ||
		!result.DispatchAcknowledged() ||
		!result.ResultAcknowledged() ||
		result.CancelAcknowledged() {
		t.Fatalf("adapter result = %#v", result)
	}
	frames := result.InboundFrames()
	if len(frames) != 5 || len(sink.frames) != len(frames) {
		t.Fatalf("frames = %d/%d, want 5 translated frames", len(frames), len(sink.frames))
	}
	wantTypes := []bridgev1.MessageType{
		bridgev1.MessageAck,
		bridgev1.MessageEvent,
		bridgev1.MessageEvent,
		bridgev1.MessageEvidence,
		bridgev1.MessageResult,
	}
	for index, frame := range frames {
		if frame.Type() != wantTypes[index] || frame.Sequence() != int64(index+2) {
			t.Fatalf("frame[%d] = %q seq %d", index, frame.Type(), frame.Sequence())
		}
	}
	if got := string(frames[1].Payload()); got != `{"delta":"Hello "}` {
		t.Fatalf("first delta = %s", got)
	}
	if got := string(frames[2].Payload()); got != `{"delta":"world"}` {
		t.Fatalf("second delta = %s", got)
	}
	if bytes.Contains(mustJSONFrames(t, frames), []byte(piTestTokenValue)) {
		t.Fatal("translated frames leaked the raw Grant")
	}
	modelsPath := filepath.Join(fixture.homePath, ".pi", "agent", "models.json")
	info, err := os.Lstat(modelsPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("models.json binding = %#v, %v", info, err)
	}
}

func TestPiRPCTranscriptClosure(t *testing.T) {
	for _, mode := range []string{"success", "forward-start", "forward-multi"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, mode)
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			result, err := adapter.Execute(context.Background(), fixture.request(t))
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !result.DispatchAcknowledged() ||
				!result.ResultAcknowledged() ||
				len(result.InboundFrames()) != 5 {
				t.Fatalf("closed transcript result = %#v", result)
			}
		})
	}

	t.Run("sanitized audit closes after accepted result", func(t *testing.T) {
		fixture := newPiRPCBridgeFixture(t, "forward-multi")
		audits := make(chan PiRPCTranscriptAudit, 1)
		config := fixture.config()
		config.TranscriptAudit = audits
		adapter, err := NewPiRPCBridgeAdapter(config)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := adapter.Execute(context.Background(), fixture.request(t)); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		select {
		case audit := <-audits:
			if !audit.ForwardPartialObserved ||
				audit.TextStartSnapshotBytes <= audit.AcceptedBytesAtTextStart ||
				audit.AcceptedBytesAtTextStart != 0 ||
				audit.AcceptedDeltaCount != 2 ||
				audit.FinalSnapshotBytes != audit.FinalAcceptedDeltaBytes ||
				!audit.DeltaClosure {
				t.Fatalf("sanitized audit = %#v", audit)
			}
		default:
			t.Fatal("accepted transcript did not publish its sanitized audit")
		}
	})

	t.Run("audit availability is non-authoritative", func(t *testing.T) {
		full := make(chan PiRPCTranscriptAudit, 1)
		full <- PiRPCTranscriptAudit{}
		closed := make(chan PiRPCTranscriptAudit)
		close(closed)
		for name, audits := range map[string]chan PiRPCTranscriptAudit{
			"unbuffered": make(chan PiRPCTranscriptAudit),
			"full":       full,
			"closed":     closed,
		} {
			t.Run(name, func(t *testing.T) {
				fixture := newPiRPCBridgeFixture(t, "success")
				config := fixture.config()
				config.TranscriptAudit = audits
				adapter, err := NewPiRPCBridgeAdapter(config)
				if err != nil {
					t.Fatal(err)
				}
				started := time.Now()
				result, err := adapter.Execute(context.Background(), fixture.request(t))
				if err != nil {
					t.Fatalf("Execute() error = %v", err)
				}
				if time.Since(started) > 2*time.Second ||
					len(result.InboundFrames()) != 5 {
					t.Fatal("audit availability changed accepted execution")
				}
			})
		}
	})

	t.Run("concurrent audits do not cross deliver", func(t *testing.T) {
		firstFixture := newPiRPCBridgeFixture(t, "forward-start")
		secondFixture := newPiRPCBridgeFixture(t, "forward-multi")
		firstAudits := make(chan PiRPCTranscriptAudit, 1)
		secondAudits := make(chan PiRPCTranscriptAudit, 1)
		firstConfig := firstFixture.config()
		firstConfig.TranscriptAudit = firstAudits
		secondConfig := secondFixture.config()
		secondConfig.TranscriptAudit = secondAudits
		firstAdapter, err := NewPiRPCBridgeAdapter(firstConfig)
		if err != nil {
			t.Fatal(err)
		}
		secondAdapter, err := NewPiRPCBridgeAdapter(secondConfig)
		if err != nil {
			t.Fatal(err)
		}
		firstRequest := firstFixture.request(t)
		secondRequest := secondFixture.request(t)
		results := make(chan error, 2)
		go func() {
			_, executeErr := firstAdapter.Execute(
				context.Background(),
				firstRequest,
			)
			results <- executeErr
		}()
		go func() {
			_, executeErr := secondAdapter.Execute(
				context.Background(),
				secondRequest,
			)
			results <- executeErr
		}()
		for range 2 {
			if err := <-results; err != nil {
				t.Fatalf("concurrent Execute() error = %v", err)
			}
		}
		firstAudit := <-firstAudits
		secondAudit := <-secondAudits
		if firstAudit.TextStartSnapshotBytes >= secondAudit.TextStartSnapshotBytes ||
			firstAudit.AcceptedDeltaCount != 2 ||
			secondAudit.AcceptedDeltaCount != 2 {
			t.Fatal("concurrent transcript audits crossed executions")
		}
	})
}

func TestPiRPCTranscriptClosurePrefixSkew(t *testing.T) {
	for _, mode := range []string{
		"prefix-skew-nested-start",
		"prefix-skew-top-start",
		"prefix-skew-nested-delta",
		"prefix-skew-top-delta",
	} {
		t.Run(mode, func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, mode)
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			result, err := adapter.Execute(context.Background(), fixture.request(t))
			if err != nil {
				t.Fatalf("prefix-comparable transcript rejected: %v", err)
			}
			if !result.DispatchAcknowledged() ||
				!result.ResultAcknowledged() ||
				len(result.InboundFrames()) != 5 {
				t.Fatalf("prefix-comparable result = %#v", result)
			}
		})
	}
}

func TestPiRPCTranscriptClosureUsageProjectionSkew(t *testing.T) {
	for _, mode := range []string{
		"usage-skew-start",
		"usage-skew-delta",
		"usage-skew-reasoning-appears",
	} {
		t.Run(mode, func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, mode)
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			result, err := adapter.Execute(context.Background(), fixture.request(t))
			if err != nil {
				t.Fatalf("top-level-lagging usage transcript rejected: %v", err)
			}
			if !result.DispatchAcknowledged() ||
				!result.ResultAcknowledged() ||
				len(result.InboundFrames()) != 5 {
				t.Fatalf("usage projection result = %#v", result)
			}
		})
	}

	for _, mode := range []string{
		"usage-skew-reverse",
		"usage-skew-crossed",
		"usage-skew-reasoning-disappears",
		"usage-skew-cache-write-1h",
		"usage-skew-malformed",
		"usage-skew-extra-key",
		"usage-skew-terminal",
	} {
		t.Run(mode, func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, mode)
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			result, err := adapter.Execute(context.Background(), fixture.request(t))
			if !errors.Is(err, ErrPiRPCProtocol) {
				t.Fatalf("Execute() error = %v, want closed usage rejection", err)
			}
			if len(result.InboundFrames()) != 0 {
				t.Fatalf("usage rejection leaked AdapterResult = %#v", result)
			}
		})
	}
}

func TestPiRPCTranscriptClosureRejections(t *testing.T) {
	for _, mode := range []string{
		"forward-shrink",
		"forward-diverge",
		"forward-unreconstructed",
		"forward-reordered",
		"forward-partial-oversized",
		"forward-partial-grant",
		"forward-partial-invalid-utf8",
		"prefix-skew-diverge",
		"prefix-skew-nontext",
	} {
		t.Run(mode, func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, mode)
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			result, err := adapter.Execute(context.Background(), fixture.request(t))
			if !errors.Is(err, ErrPiRPCProtocol) &&
				!errors.Is(err, ErrPiRPCOutputTooLarge) {
				t.Fatalf("Execute() error = %v, want closed rejection", err)
			}
			if len(result.InboundFrames()) != 0 {
				t.Fatalf("rejection leaked AdapterResult = %#v", result)
			}
		})
	}
}

func TestPiRPCTranscriptClosureFrameAuthority(t *testing.T) {
	var want []bridgev1.Frame
	for _, mode := range []string{"success", "forward-start", "forward-multi"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, mode)
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			result, err := adapter.Execute(context.Background(), fixture.request(t))
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			frames := result.InboundFrames()
			if mode == "success" {
				want = frames
				return
			}
			if !bytes.Equal(mustJSONFrames(t, frames), mustJSONFrames(t, want)) {
				t.Fatal("forward partial changed authoritative Frame bytes")
			}
		})
	}
}

func TestPiRPCTranscriptLifecycleClosure(t *testing.T) {
	for _, mode := range []string{
		"lifecycle-missing-settled",
		"lifecycle-duplicate-settled",
		"lifecycle-out-of-order-terminal",
	} {
		t.Run(mode, func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, mode)
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			result, err := adapter.Execute(context.Background(), fixture.request(t))
			if !errors.Is(err, ErrPiRPCProtocol) {
				t.Fatalf("Execute() error = %v, want ErrPiRPCProtocol", err)
			}
			if len(result.InboundFrames()) != 0 {
				t.Fatalf("lifecycle rejection leaked AdapterResult = %#v", result)
			}
		})
	}
}

func TestPiRPCPi0821ProgressiveAssistantIdentity(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "progressive")
	adapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	request := fixture.request(t)
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.DispatchAcknowledged() ||
		!result.ResultAcknowledged() ||
		len(result.InboundFrames()) != 5 {
		t.Fatalf("progressive Pi transcript result = %#v", result)
	}
}

func TestPiRPCProgressiveAssistantIdentityRejections(t *testing.T) {
	modes := []string{
		"progressive-response-id-prepopulated",
		"progressive-response-id-missing-at-start",
		"progressive-response-id-removed",
		"progressive-response-id-terminal-removed",
		"progressive-response-id-mutated",
		"progressive-response-id-empty",
		"progressive-response-id-oversized",
		"progressive-response-id-late",
		"progressive-response-model",
		"progressive-timestamp-drift",
		"progressive-partial-mismatch",
		"progressive-cache-write-1h",
		"progressive-reasoning-removed",
		"progressive-reasoning-terminal-first",
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, mode)
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			request := fixture.request(t)
			result, err := adapter.Execute(context.Background(), request)
			if !errors.Is(err, ErrPiRPCProtocol) {
				t.Fatalf("Execute() error = %v, want ErrPiRPCProtocol", err)
			}
			if len(result.InboundFrames()) != 0 {
				t.Fatalf("Execute() leaked partial AdapterResult = %#v", result)
			}
		})
	}
}

func TestPiRPCRejectionReasonCodes(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want string
	}{
		{
			name: "response model on first update",
			mode: "progressive-response-model",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=text_start reason=response_model_present",
		},
		{
			name: "response ID missing on first update",
			mode: "progressive-response-id-missing-at-start",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=text_start reason=response_id_transition",
		},
		{
			name: "timestamp drift",
			mode: "progressive-timestamp-drift",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=text_start reason=timestamp_identity",
		},
		{
			name: "malformed usage",
			mode: "diagnostic-usage-schema",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=text_start reason=usage_schema",
		},
		{
			name: "usage progression",
			mode: "progressive-cache-write-1h",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=text_start reason=usage_progression",
		},
		{
			name: "partial mismatch",
			mode: "progressive-partial-mismatch",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=text_start reason=message_partial_mismatch",
		},
		{
			name: "assistant message schema",
			mode: "diagnostic-assistant-schema",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=text_start reason=assistant_message_schema",
		},
		{
			name: "assistant update event shape",
			mode: "diagnostic-update-event-shape",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=text_start reason=event_shape",
		},
		{
			name: "unsupported thinking event",
			mode: "diagnostic-thinking-first",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=thinking_start reason=event_kind_unsupported",
		},
		{
			name: "unsupported tool event",
			mode: "diagnostic-tool-first",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=toolcall_start reason=event_kind_unsupported",
		},
		{
			name: "content index",
			mode: "diagnostic-content-index",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=text_start reason=content_index",
		},
		{
			name: "delta policy",
			mode: "diagnostic-delta-policy",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=text_delta reason=delta_policy",
		},
		{
			name: "text progression",
			mode: "mismatch",
			want: "Pi RPC protocol failed: phase=assistant_update " +
				"event=text_end reason=text_content_progression",
		},
		{
			name: "assistant message end",
			mode: "length",
			want: "Pi RPC protocol failed: phase=assistant_message_end " +
				"event=message_end reason=terminal_stop_reason",
		},
		{
			name: "turn end",
			mode: "diagnostic-turn-end",
			want: "Pi RPC protocol failed: phase=turn_end " +
				"event=turn_end reason=terminal_identity",
		},
		{
			name: "agent end",
			mode: "retry",
			want: "Pi RPC protocol failed: phase=agent_end " +
				"event=agent_end reason=terminal_identity",
		},
		{
			name: "agent settled",
			mode: "diagnostic-agent-settled",
			want: "Pi RPC protocol failed: phase=agent_settled " +
				"event=agent_settled reason=event_shape",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, test.mode)
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			_, err = adapter.Execute(context.Background(), fixture.request(t))
			if !errors.Is(err, ErrPiRPCProtocol) {
				t.Fatalf("Execute() error = %v, want ErrPiRPCProtocol", err)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Execute() error = %q, want bounded reason %q", err, test.want)
			}
			if strings.Count(err.Error(), "phase=") != 1 ||
				strings.Count(err.Error(), "event=") != 1 ||
				strings.Count(err.Error(), "reason=") != 1 {
				t.Fatalf("Execute() error contains a non-single diagnostic = %q", err)
			}
		})
	}

	t.Run("frame sink", func(t *testing.T) {
		fixture := newPiRPCBridgeFixture(t, "progressive")
		adapter, err := NewPiRPCBridgeAdapter(fixture.config())
		if err != nil {
			t.Fatal(err)
		}
		request := fixture.request(t)
		sink := &piRPCEventRejectingFrameSink{
			err: errors.New("private sink detail"),
		}
		request.FrameSink = sink
		_, err = adapter.Execute(context.Background(), request)
		if !errors.Is(err, ErrPiRPCProtocol) {
			t.Fatalf("Execute() error = %v, want ErrPiRPCProtocol", err)
		}
		want := "Pi RPC protocol failed: phase=assistant_update " +
			"event=text_delta reason=frame_sink"
		if !strings.Contains(err.Error(), want) ||
			strings.Contains(err.Error(), "private sink detail") {
			t.Fatalf("Execute() error = %q, want bounded frame-sink reason", err)
		}
	})
}

func TestPiRPCRejectionReasonCodeNonDisclosure(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "progressive-response-model")
	adapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Execute(context.Background(), fixture.request(t))
	if !errors.Is(err, ErrPiRPCProtocol) {
		t.Fatalf("Execute() error = %v, want ErrPiRPCProtocol", err)
	}
	got := err.Error()
	for _, forbidden := range []string{
		piRPCFixturePrompt,
		piRPCSystemPrompt,
		piTestTokenValue,
		"other-model",
		"10000000-0000-4000-8000-000000000001",
		piRPCProviderID,
		piRPCModelID,
		fixture.homePath,
		fixture.workspacePath,
		fixture.tempPath,
		`{"`,
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("bounded reason error disclosed forbidden material")
		}
	}
}

func TestPiRPCRejectionReasonCodePrecedence(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "diagnostic-precedence")
	adapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Execute(context.Background(), fixture.request(t))
	if !errors.Is(err, ErrPiRPCProtocol) {
		t.Fatalf("Execute() error = %v, want ErrPiRPCProtocol", err)
	}
	want := "Pi RPC protocol failed: phase=assistant_update " +
		"event=text_start reason=message_partial_mismatch"
	if !strings.Contains(err.Error(), want) ||
		strings.Contains(err.Error(), "reason=response_model_present") ||
		strings.Contains(err.Error(), "reason=response_id_transition") {
		t.Fatalf("Execute() error = %q, want first-rejection precedence", err)
	}
}

func TestPiRPCBridgeFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want error
	}{
		{name: "unknown record", mode: "unknown", want: ErrPiRPCProtocol},
		{name: "tool use", mode: "tool", want: ErrPiRPCProtocol},
		{name: "hidden reasoning", mode: "thinking", want: ErrPiRPCProtocol},
		{name: "length completion", mode: "length", want: ErrPiRPCProtocol},
		{name: "invented nested start", mode: "nested-start-event", want: ErrPiRPCProtocol},
		{name: "invented nested done", mode: "nested-done-event", want: ErrPiRPCProtocol},
		{name: "invented nested error", mode: "nested-error-event", want: ErrPiRPCProtocol},
		{name: "top-level partial identity mismatch", mode: "partial-identity-mismatch", want: ErrPiRPCProtocol},
		{name: "response model substitution", mode: "response-model-substitution", want: ErrPiRPCProtocol},
		{name: "retry", mode: "retry", want: ErrPiRPCProtocol},
		{name: "mismatched text", mode: "mismatch", want: ErrPiRPCProtocol},
		{name: "uncorrelated response", mode: "uncorrelated", want: ErrPiRPCProtocol},
		{name: "duplicate key", mode: "duplicate", want: ErrPiRPCProtocol},
		{name: "unknown key", mode: "extra", want: ErrPiRPCProtocol},
		{name: "nested duplicate key", mode: "nested-duplicate", want: ErrPiRPCProtocol},
		{name: "nested unknown key", mode: "nested-extra", want: ErrPiRPCProtocol},
		{name: "error assistant field", mode: "assistant-error", want: ErrPiRPCProtocol},
		{name: "unexpected agent end message", mode: "agent-end-extra", want: ErrPiRPCProtocol},
		{name: "record after settled", mode: "after-settled", want: ErrPiRPCProtocol},
		{name: "carriage return", mode: "carriage-return", want: ErrPiRPCProtocol},
		{name: "unicode separator", mode: "unicode-separator", want: ErrPiRPCProtocol},
		{name: "grant output", mode: "grant", want: ErrPiRPCProtocol},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPiRPCBridgeFixture(t, test.mode)
			adapter, err := NewPiRPCBridgeAdapter(fixture.config())
			if err != nil {
				t.Fatal(err)
			}
			request := fixture.request(t)
			sink := request.FrameSink.(*piRecordingFrameSink)
			result, err := adapter.Execute(context.Background(), request)
			if !errors.Is(err, test.want) {
				t.Fatalf("Execute() error = %v, want %v", err, test.want)
			}
			if len(result.InboundFrames()) != 0 {
				t.Fatalf("Execute() leaked partial AdapterResult = %#v", result)
			}
			if test.mode == "after-settled" {
				for _, frame := range sink.frames {
					if frame.Type() == bridgev1.MessageEvidence ||
						frame.Type() == bridgev1.MessageResult {
						t.Fatalf("post-settled rejection leaked terminal frame %q", frame.Type())
					}
				}
			}
		})
	}
}

func TestPiRPCBridgeRejectsDispatchBeforeProcessStart(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	tests := [][]byte{
		[]byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"/help"}`),
		[]byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":""}`),
		[]byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"TOKEN=secret"}`),
		[]byte(`{"kind":"pi_rpc_prompt","schema_version":1,"prompt":"valid"}`),
		[]byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"valid","extra":true}`),
		[]byte(`{"schema_version":1,"schema_version":1,"kind":"pi_rpc_prompt","prompt":"valid"}`),
	}
	for index, payload := range tests {
		if _, err := parsePiRPCDispatch(payload, fixture.token.Value()); !errors.Is(err, ErrPiRPCProtocol) {
			t.Fatalf("dispatch[%d] error = %v, want ErrPiRPCProtocol", index, err)
		}
	}
}

func TestPiRPCBridgeCancellationAcknowledgementAndCleanup(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "wait-cancel")
	adapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	answer := make(chan error, 1)
	go func() {
		_, executeErr := adapter.Execute(ctx, fixture.request(t))
		answer <- executeErr
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-answer:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrPiRPCProtocol) {
			t.Fatalf("Execute() cancellation error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Pi RPC cancellation did not clean up within the bound")
	}
}

func TestPiRPCBridgeUTF8ChunkBoundaries(t *testing.T) {
	for offset := 0; offset < 100; offset++ {
		input := []byte(strings.Repeat("界", 1000) + strings.Repeat("x", offset))
		chunks := splitPiRPCDelta(input)
		var rebuilt []byte
		for index, chunk := range chunks {
			if len(chunk) == 0 || len(chunk) > piRPCMaxDeltaBytes || !json.Valid(mustDeltaJSON(t, chunk)) {
				t.Fatalf("offset %d chunk %d invalid: %d bytes", offset, index, len(chunk))
			}
			rebuilt = append(rebuilt, chunk...)
		}
		if !bytes.Equal(rebuilt, input) {
			t.Fatalf("offset %d changed UTF-8 content", offset)
		}
	}
}

func FuzzPiRPCObjectNoPanic(f *testing.F) {
	f.Add([]byte(`{"type":"agent_start"}`))
	f.Add([]byte(`{"type":"x","type":"y"}`))
	f.Fuzz(func(t *testing.T, value []byte) {
		_, _ = piRPCObject(value)
	})
}

type piRPCBridgeFixture struct {
	executablePath string
	searchPath     string
	workspacePath  string
	homePath       string
	tempPath       string
	binding        bridgev1.RunStreamBinding
	dispatch       bridgev1.Frame
	token          authorization.Token
}

func newPiRPCBridgeFixture(t testing.TB, mode string) *piRPCBridgeFixture {
	t.Helper()
	root := piPrivateDirectory(t, "rpc-bridge")
	executablePath := filepath.Join(root, "pi-fixture")
	script := piRPCFixtureScript(mode)
	if err := os.WriteFile(executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(executablePath, 0o700); err != nil {
		t.Fatal(err)
	}
	binding := bridgev1.RunStreamBinding{
		WorkItemID:            "S5-W2",
		RunID:                 "run-rpc-1",
		ClaimGeneration:       1,
		RuntimeInstanceID:     "runtime.pi.earendil-works.0.82.1",
		SenderAgentInstanceID: "agent-main-1",
	}
	payload, err := json.Marshal(struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Prompt        string `json:"prompt"`
	}{SchemaVersion: 1, Kind: "pi_rpc_prompt", Prompt: piRPCFixturePrompt})
	if err != nil {
		t.Fatal(err)
	}
	token, err := authorization.ParseToken(piTestTokenValue)
	if err != nil {
		t.Fatal(err)
	}
	return &piRPCBridgeFixture{
		executablePath: executablePath,
		searchPath:     piPrivateDirectoryAt(t, root, "search"),
		workspacePath:  piPrivateDirectoryAt(t, root, "workspace"),
		homePath:       piPrivateDirectoryAt(t, root, "home"),
		tempPath:       piPrivateDirectoryAt(t, root, "tmp"),
		binding:        binding,
		dispatch: piTestFrame(
			t,
			binding,
			1,
			bridgev1.MessageDispatch,
			payload,
		),
		token: token,
	}
}

func (fixture *piRPCBridgeFixture) config() PiRPCBridgeAdapterConfig {
	return PiRPCBridgeAdapterConfig{
		Execution: PiExecutionAdapterConfig{
			ExecutablePath:     fixture.executablePath,
			RuntimeInstanceID:  fixture.binding.RuntimeInstanceID,
			RuntimeSearchPaths: []string{fixture.searchPath},
			CancelGrace:        200 * time.Millisecond,
			Now:                func() time.Time { return managedPiTestNow },
			Random:             bytes.NewReader(bytes.Repeat([]byte{0x45}, 2048)),
		},
		ProviderID:        "loom-local",
		ModelID:           "qwen2.5-coder-1.5b-instruct-q4-k-m",
		BaseURL:           "http://127.0.0.1:18427/v1",
		MaxAssistantBytes: 16384,
	}
}

func (fixture *piRPCBridgeFixture) request(t testing.TB) supervisor.AdapterRequest {
	t.Helper()
	return supervisor.AdapterRequest{
		WorkspacePath: fixture.workspacePath,
		HomePath:      fixture.homePath,
		TempPath:      fixture.tempPath,
		Binding:       fixture.binding,
		Dispatch:      fixture.dispatch,
		Grant:         fixture.token,
		FrameSink:     &piRecordingFrameSink{},
	}
}

func piRPCFixtureScript(mode string) string {
	responseID := "10000000-0000-4000-8000-000000000001"
	expectedArguments := []string{
		"--mode", "rpc",
		"--offline",
		"--no-approve",
		"--no-session",
		"--no-tools",
		"--no-extensions",
		"--no-skills",
		"--no-prompt-templates",
		"--no-themes",
		"--no-context-files",
		"--provider", "loom-local",
		"--model", "loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m",
		"--thinking", "off",
		"--system-prompt", piRPCSystemPrompt,
	}
	header := "#!/bin/sh\n" +
		"[ -z \"${SHOULD_NOT_LEAK+x}\" ] || exit 41\n" +
		"[ -z \"${LOOM_AGENT_GRANT+x}\" ] || exit 42\n" +
		"[ \"$PI_OFFLINE\" = 1 ] || exit 43\n" +
		"[ \"$PI_SKIP_VERSION_CHECK\" = 1 ] || exit 44\n" +
		"[ \"$PI_TELEMETRY\" = 0 ] || exit 45\n" +
		"[ \"$#\" -eq " + fmt.Sprint(len(expectedArguments)) + " ] || exit 46\n"
	for _, argument := range expectedArguments {
		header += "[ \"$1\" = " + piRPCShellQuote(argument) + " ] || exit 47\nshift\n"
	}
	if mode == "settings-start-marker" {
		header += ": > \"$HOME/process-started\"\n"
	}
	if mode == "require-settings" || mode == "settings-start-marker" {
		header += "exec 3<\"$PI_CODING_AGENT_DIR/settings.json\" || exit 50\n" +
			"IFS= read -r settings <&3 || exit 51\n" +
			"[ \"$settings\" = " + piRPCShellQuote(strings.TrimSuffix(piRPCFixtureSettings, "\n")) + " ] || exit 52\n" +
			"if IFS= read -r extra <&3; then exit 53; fi\n" +
			"exec 3<&-\n"
	}
	if mode == "wait-cancel" {
		return header +
			"read request || exit 48\n" +
			"read abort || exit 49\n" +
			"printf '%s\\n' '{\"id\":\"45454545-4545-4545-8545-454545454545\",\"type\":\"response\",\"command\":\"abort\",\"success\":true}'\n" +
			"exec /bin/sleep 30\n"
	}
	userMessage := piRPCFixtureUserMessage(piRPCFixturePrompt)
	emptyAssistant := piRPCFixtureAssistantMessage("")
	emptyTextAssistant := strings.Replace(
		emptyAssistant,
		`"content":[]`,
		`"content":[{"type":"text","text":""}]`,
		1,
	)
	firstAssistant := piRPCFixtureAssistantMessage("Hello ")
	finalAssistant := piRPCFixtureAssistantMessage("Hello world")
	lines := []string{
		`{"id":"` + responseID + `","type":"response","command":"prompt","success":true}`,
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"message_start","message":` + userMessage + `}`,
		`{"type":"message_end","message":` + userMessage + `}`,
		`{"type":"message_start","message":` + emptyAssistant + `}`,
		`{"type":"message_update","message":` + emptyTextAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + emptyTextAssistant + `}}`,
		`{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + firstAssistant + `}}`,
		`{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"world","partial":` + finalAssistant + `}}`,
		`{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"Hello world","partial":` + finalAssistant + `}}`,
		`{"type":"message_end","message":` + finalAssistant + `}`,
		`{"type":"turn_end","message":` + finalAssistant + `,"toolResults":[]}`,
		`{"type":"agent_end","messages":[` + userMessage + `,` + finalAssistant + `],"willRetry":false}`,
		`{"type":"agent_settled"}`,
	}
	if mode != "wait-cancel" {
		emptyTextAssistant = piRPCFixtureWithResponseID(
			emptyTextAssistant,
			responseID,
		)
		firstAssistant = piRPCFixtureWithResponseID(firstAssistant, responseID)
		finalAssistant = piRPCFixtureWithResponseID(finalAssistant, responseID)
		finalWithReasoning := piRPCFixtureWithUsageField(
			finalAssistant,
			`"reasoning":0`,
		)
		lines = []string{
			`{"id":"` + responseID + `","type":"response","command":"prompt","success":true}`,
			`{"type":"agent_start"}`,
			`{"type":"turn_start"}`,
			`{"type":"message_start","message":` + userMessage + `}`,
			`{"type":"message_end","message":` + userMessage + `}`,
			`{"type":"message_start","message":` + emptyAssistant + `}`,
			`{"type":"message_update","message":` + emptyTextAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + emptyTextAssistant + `}}`,
			`{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + firstAssistant + `}}`,
			`{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"world","partial":` + finalAssistant + `}}`,
			`{"type":"message_update","message":` + finalWithReasoning + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"Hello world","partial":` + finalWithReasoning + `}}`,
			`{"type":"message_end","message":` + finalWithReasoning + `}`,
			`{"type":"turn_end","message":` + finalWithReasoning + `,"toolResults":[]}`,
			`{"type":"agent_end","messages":[` + userMessage + `,` + finalWithReasoning + `],"willRetry":false}`,
			`{"type":"agent_settled"}`,
		}
	}
	switch mode {
	case "forward-start":
		lines[6] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + firstAssistant + `}}`
	case "forward-multi":
		lines[6] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + finalAssistant + `}}`
		lines[7] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + finalAssistant + `}}`
	case "forward-shrink":
		lines[6] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + finalAssistant + `}}`
		lines[7] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + firstAssistant + `}}`
	case "forward-diverge":
		lines[6] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + firstAssistant + `}}`
		lines[7] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"world","partial":` + finalAssistant + `}}`
	case "forward-unreconstructed":
		lines[6] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + finalAssistant + `}}`
		lines[7] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + finalAssistant + `}}`
		lines[8] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"Hello world","partial":` + finalAssistant + `}}`
	case "forward-reordered":
		lines[6] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + finalAssistant + `}}`
		lines[7] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"world","partial":` + finalAssistant + `}}`
	case "forward-partial-oversized":
		oversized := piRPCFixtureWithResponseID(
			piRPCFixtureAssistantMessage(strings.Repeat("x", 20000)),
			responseID,
		)
		lines[6] = `{"type":"message_update","message":` + oversized + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + oversized + `}}`
	case "forward-partial-grant":
		grant := piRPCFixtureWithResponseID(
			piRPCFixtureAssistantMessage(piTestTokenValue),
			responseID,
		)
		lines[6] = `{"type":"message_update","message":` + grant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + grant + `}}`
	case "forward-partial-invalid-utf8":
		invalid := string([]byte{0xff})
		invalidAssistant := strings.Replace(
			firstAssistant,
			"Hello ",
			invalid,
			1,
		)
		lines[6] = `{"type":"message_update","message":` + invalidAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + invalidAssistant + `}}`
	case "prefix-skew-nested-start":
		lines[6] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + finalAssistant + `}}`
		lines[7] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + finalAssistant + `}}`
	case "prefix-skew-top-start":
		lines[6] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + firstAssistant + `}}`
		lines[7] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + firstAssistant + `}}`
	case "prefix-skew-nested-delta":
		lines[6] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + firstAssistant + `}}`
		lines[7] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + finalAssistant + `}}`
	case "prefix-skew-top-delta":
		lines[6] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + firstAssistant + `}}`
		lines[7] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + firstAssistant + `}}`
	case "prefix-skew-diverge":
		divergent := piRPCFixtureWithResponseID(
			piRPCFixtureAssistantMessage("different"),
			responseID,
		)
		lines[6] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + divergent + `}}`
	case "prefix-skew-nontext":
		differentIdentity := strings.Replace(
			finalAssistant,
			responseID,
			"20000000-0000-4000-8000-000000000002",
			1,
		)
		lines[6] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + differentIdentity + `}}`
	case "usage-skew-start":
		advanced := piRPCFixtureWithUsageNumbers(
			finalAssistant,
			3,
			2,
			5,
			false,
		)
		lines[6] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + advanced + `}}`
		lines[7] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + finalAssistant + `}}`
	case "usage-skew-delta":
		advanced := piRPCFixtureWithUsageNumbers(
			finalAssistant,
			3,
			2,
			5,
			false,
		)
		lines[6] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + firstAssistant + `}}`
		lines[7] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + advanced + `}}`
	case "usage-skew-reasoning-appears":
		advanced := piRPCFixtureWithUsageNumbers(
			finalAssistant,
			3,
			2,
			5,
			true,
		)
		lines[8] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"world","partial":` + advanced + `}}`
	case "usage-skew-reverse":
		advanced := piRPCFixtureWithUsageNumbers(
			finalAssistant,
			3,
			2,
			5,
			false,
		)
		lines[7] = `{"type":"message_update","message":` + advanced + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + firstAssistant + `}}`
	case "usage-skew-crossed":
		top := piRPCFixtureWithUsageNumbers(
			finalAssistant,
			2,
			0,
			2,
			false,
		)
		partial := piRPCFixtureWithUsageNumbers(
			finalAssistant,
			0,
			2,
			2,
			false,
		)
		lines[7] = `{"type":"message_update","message":` + top + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + partial + `}}`
	case "usage-skew-reasoning-disappears":
		top := piRPCFixtureWithUsageNumbers(
			firstAssistant,
			0,
			0,
			0,
			true,
		)
		lines[7] = `{"type":"message_update","message":` + top + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + firstAssistant + `}}`
	case "usage-skew-cache-write-1h":
		withCacheWrite1h := piRPCFixtureWithUsageField(
			firstAssistant,
			`"cacheWrite1h":0`,
		)
		lines[7] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + withCacheWrite1h + `}}`
	case "usage-skew-malformed":
		malformed := strings.Replace(
			firstAssistant,
			`"input":0`,
			`"input":"invalid"`,
			1,
		)
		lines[7] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + malformed + `}}`
	case "usage-skew-extra-key":
		extra := strings.Replace(
			firstAssistant,
			`"totalTokens":0`,
			`"totalTokens":0,"extra":0`,
			1,
		)
		lines[7] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + extra + `}}`
	case "usage-skew-terminal":
		terminal := piRPCFixtureWithUsageField(
			finalAssistant,
			`"reasoning":0`,
		)
		advanced := piRPCFixtureWithUsageNumbers(
			terminal,
			3,
			2,
			5,
			true,
		)
		lines[9] = `{"type":"message_update","message":` + terminal + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"Hello world","partial":` + advanced + `}}`
	case "lifecycle-missing-settled":
		lines = lines[:len(lines)-1]
	case "lifecycle-duplicate-settled":
		lines = append(lines, `{"type":"agent_settled"}`)
	case "lifecycle-out-of-order-terminal":
		lines[10], lines[11] = lines[11], lines[10]
	case "unknown":
		lines[3] = `{"type":"extension_error"}`
	case "tool":
		toolAssistant := strings.Replace(
			emptyAssistant,
			`"content":[]`,
			`"content":[{"type":"toolCall","id":"x","name":"bash","arguments":{}}]`,
			1,
		)
		lines[7] = `{"type":"message_update","message":` + toolAssistant + `,"assistantMessageEvent":{"type":"toolcall_start","contentIndex":0,"partial":` + toolAssistant + `}}`
	case "thinking":
		thinkingAssistant := strings.Replace(
			emptyAssistant,
			`"content":[]`,
			`"content":[{"type":"thinking","thinking":"hidden"}]`,
			1,
		)
		lines[7] = `{"type":"message_update","message":` + thinkingAssistant + `,"assistantMessageEvent":{"type":"thinking_start","contentIndex":0,"partial":` + thinkingAssistant + `}}`
	case "length":
		lengthAssistant := strings.ReplaceAll(finalAssistant, `"stopReason":"stop"`, `"stopReason":"length"`)
		lines[10] = `{"type":"message_end","message":` + lengthAssistant + `}`
	case "nested-start-event":
		lines[6] = `{"type":"message_update","message":` + emptyAssistant + `,"assistantMessageEvent":{"type":"start","partial":` + emptyAssistant + `}}`
	case "nested-done-event":
		lines[9] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"done","reason":"stop","message":` + finalAssistant + `}}`
	case "nested-error-event":
		lines[9] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"error","reason":"error","error":` + finalAssistant + `}}`
	case "partial-identity-mismatch":
		differentPartial := strings.Replace(
			firstAssistant,
			responseID,
			"different",
			1,
		)
		lines[7] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + differentPartial + `}}`
	case "response-model-substitution":
		substituted := strings.Replace(finalAssistant, `"timestamp":1`, `"responseModel":"other-model","timestamp":1`, 1)
		lines[10] = `{"type":"message_end","message":` + substituted + `}`
	case "retry":
		lines[12] = `{"type":"agent_end","messages":[` + userMessage + `,` + finalAssistant + `],"willRetry":true}`
	case "mismatch":
		differentAssistant := piRPCFixtureAssistantMessage("different")
		lines[9] = `{"type":"message_update","message":` + differentAssistant + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"different","partial":` + differentAssistant + `}}`
	case "uncorrelated":
		lines[0] = `{"id":"20000000-0000-4000-8000-000000000002","type":"response","command":"prompt","success":true}`
	case "duplicate":
		lines[0] = `{"id":"` + responseID + `","type":"response","type":"response","command":"prompt","success":true}`
	case "extra":
		lines[0] = `{"id":"` + responseID + `","type":"response","command":"prompt","success":true,"extra":true}`
	case "nested-duplicate":
		lines[5] = strings.Replace(emptyAssistant, `"role":"assistant"`, `"role":"assistant","role":"assistant"`, 1)
		lines[5] = `{"type":"message_start","message":` + lines[5] + `}`
	case "nested-extra":
		nested := strings.Replace(emptyAssistant, `"timestamp":1`, `"timestamp":1,"extra":true`, 1)
		lines[5] = `{"type":"message_start","message":` + nested + `}`
	case "assistant-error":
		nested := strings.Replace(emptyAssistant, `"timestamp":1`, `"timestamp":1,"errorMessage":"secret"`, 1)
		lines[5] = `{"type":"message_start","message":` + nested + `}`
	case "agent-end-extra":
		lines[12] = `{"type":"agent_end","messages":[` + userMessage + `,` + finalAssistant + `,` + userMessage + `],"willRetry":false}`
	case "after-settled":
		lines = append(lines, `{"type":"agent_start"}`)
	case "carriage-return":
		lines[0] += "\r"
	case "unicode-separator":
		lines[0] += "\u2028"
	case "grant":
		grantAssistant := piRPCFixtureAssistantMessage(piTestTokenValue)
		lines[7] = `{"type":"message_update","message":` + grantAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"` + piTestTokenValue + `","partial":` + grantAssistant + `}}`
	case "progressive-response-id-prepopulated":
		prepopulated := piRPCFixtureWithResponseID(emptyAssistant, responseID)
		lines[5] = `{"type":"message_start","message":` + prepopulated + `}`
	case "progressive-response-id-missing-at-start":
		missing := strings.Replace(
			emptyTextAssistant,
			`,"responseId":"`+responseID+`"`,
			"",
			1,
		)
		lines[6] = `{"type":"message_update","message":` + missing + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + missing + `}}`
	case "progressive-response-id-removed":
		removed := strings.Replace(
			finalAssistant,
			`,"responseId":"`+responseID+`"`,
			"",
			1,
		)
		lines[8] = `{"type":"message_update","message":` + removed + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"world","partial":` + removed + `}}`
	case "progressive-response-id-terminal-removed":
		removed := strings.Replace(
			lines[10],
			`,"responseId":"`+responseID+`"`,
			"",
			1,
		)
		lines[10] = removed
	case "progressive-response-id-mutated":
		mutated := strings.ReplaceAll(
			finalAssistant,
			responseID,
			"20000000-0000-4000-8000-000000000002",
		)
		lines[8] = `{"type":"message_update","message":` + mutated + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"world","partial":` + mutated + `}}`
	case "progressive-response-id-empty":
		empty := strings.Replace(
			emptyTextAssistant,
			responseID,
			"",
			1,
		)
		lines[6] = `{"type":"message_update","message":` + empty + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + empty + `}}`
	case "progressive-response-id-oversized":
		oversized := strings.Replace(
			emptyTextAssistant,
			responseID,
			strings.Repeat("x", 513),
			1,
		)
		lines[6] = `{"type":"message_update","message":` + oversized + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + oversized + `}}`
	case "progressive-response-id-late":
		missing := strings.Replace(
			emptyTextAssistant,
			`,"responseId":"`+responseID+`"`,
			"",
			1,
		)
		lines[6] = `{"type":"message_update","message":` + missing + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + missing + `}}`
	case "progressive-response-model":
		substituted := strings.Replace(
			emptyTextAssistant,
			`,"timestamp":1`,
			`,"responseModel":"other-model","timestamp":1`,
			1,
		)
		lines[6] = `{"type":"message_update","message":` + substituted + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + substituted + `}}`
	case "progressive-timestamp-drift":
		drifted := strings.Replace(
			emptyTextAssistant,
			`"timestamp":1`,
			`"timestamp":2`,
			1,
		)
		lines[6] = `{"type":"message_update","message":` + drifted + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + drifted + `}}`
	case "progressive-partial-mismatch":
		different := strings.Replace(
			emptyTextAssistant,
			responseID,
			"20000000-0000-4000-8000-000000000002",
			1,
		)
		lines[6] = `{"type":"message_update","message":` + emptyTextAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + different + `}}`
	case "progressive-cache-write-1h":
		withCacheWrite1h := piRPCFixtureWithUsageField(
			emptyTextAssistant,
			`"cacheWrite1h":0`,
		)
		lines[6] = `{"type":"message_update","message":` + withCacheWrite1h + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + withCacheWrite1h + `}}`
	case "progressive-reasoning-removed":
		withReasoning := piRPCFixtureWithUsageField(
			firstAssistant,
			`"reasoning":0`,
		)
		lines[7] = `{"type":"message_update","message":` + withReasoning + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello ","partial":` + withReasoning + `}}`
	case "progressive-reasoning-terminal-first":
		lines[9] = `{"type":"message_update","message":` + finalAssistant + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"Hello world","partial":` + finalAssistant + `}}`
	case "diagnostic-usage-schema":
		invalidUsage := strings.Replace(
			emptyTextAssistant,
			`"input":0`,
			`"input":"invalid"`,
			1,
		)
		lines[6] = `{"type":"message_update","message":` + invalidUsage + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + invalidUsage + `}}`
	case "diagnostic-assistant-schema":
		invalidAssistant := strings.Replace(
			emptyTextAssistant,
			`,"timestamp":1`,
			`,"timestamp":1,"private":"value"`,
			1,
		)
		lines[6] = `{"type":"message_update","message":` + invalidAssistant + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + invalidAssistant + `}}`
	case "diagnostic-update-event-shape":
		lines[6] = strings.Replace(
			lines[6],
			`"contentIndex":0`,
			`"contentIndex":0,"private":true`,
			1,
		)
	case "diagnostic-thinking-first":
		thinkingAssistant := strings.Replace(
			emptyTextAssistant,
			`{"type":"text","text":""}`,
			`{"type":"thinking","thinking":"private"}`,
			1,
		)
		lines[6] = `{"type":"message_update","message":` + thinkingAssistant + `,"assistantMessageEvent":{"type":"thinking_start","contentIndex":0,"partial":` + thinkingAssistant + `}}`
	case "diagnostic-tool-first":
		toolAssistant := strings.Replace(
			emptyTextAssistant,
			`{"type":"text","text":""}`,
			`{"type":"toolCall","id":"private","name":"bash","arguments":{}}`,
			1,
		)
		lines[6] = `{"type":"message_update","message":` + toolAssistant + `,"assistantMessageEvent":{"type":"toolcall_start","contentIndex":0,"partial":` + toolAssistant + `}}`
	case "diagnostic-content-index":
		lines[6] = strings.Replace(lines[6], `"contentIndex":0`, `"contentIndex":1`, 1)
	case "diagnostic-delta-policy":
		lines[7] = `{"type":"message_update","message":` + firstAssistant + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"","partial":` + firstAssistant + `}}`
	case "diagnostic-turn-end":
		terminal := piRPCFixtureWithUsageField(
			finalAssistant,
			`"reasoning":0`,
		)
		lines[11] = `{"type":"turn_end","message":` + terminal + `,"toolResults":[{}]}`
	case "diagnostic-agent-settled":
		lines[13] = `{"type":"agent_settled","extra":true}`
	case "diagnostic-precedence":
		top := strings.Replace(
			emptyTextAssistant,
			`,"timestamp":1`,
			`,"responseModel":"private-model","timestamp":1`,
			1,
		)
		partial := strings.Replace(
			top,
			responseID,
			"20000000-0000-4000-8000-000000000002",
			1,
		)
		lines[6] = `{"type":"message_update","message":` + top + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + partial + `}}`
	}
	var quoted []string
	for _, line := range lines {
		quoted = append(quoted, "'"+strings.ReplaceAll(line, "'", "'\\''")+"'")
	}
	return header +
		"read request || exit 42\n" +
		"printf '%s\\n' " + strings.Join(quoted, " ") + "\n"
}

func piRPCFixtureUserMessage(prompt string) string {
	type content struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	value, err := json.Marshal(struct {
		Role      string    `json:"role"`
		Content   []content `json:"content"`
		Timestamp int64     `json:"timestamp"`
	}{
		Role:      "user",
		Content:   []content{{Type: "text", Text: prompt}},
		Timestamp: 1,
	})
	if err != nil {
		panic(err)
	}
	return string(value)
}

func piRPCFixtureAssistantMessage(text string) string {
	type content struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type cost struct {
		Input      int `json:"input"`
		Output     int `json:"output"`
		CacheRead  int `json:"cacheRead"`
		CacheWrite int `json:"cacheWrite"`
		Total      int `json:"total"`
	}
	type usage struct {
		Input       int  `json:"input"`
		Output      int  `json:"output"`
		CacheRead   int  `json:"cacheRead"`
		CacheWrite  int  `json:"cacheWrite"`
		TotalTokens int  `json:"totalTokens"`
		Cost        cost `json:"cost"`
	}
	message := struct {
		Role       string    `json:"role"`
		Content    []content `json:"content"`
		API        string    `json:"api"`
		Provider   string    `json:"provider"`
		Model      string    `json:"model"`
		Usage      usage     `json:"usage"`
		StopReason string    `json:"stopReason"`
		Timestamp  int64     `json:"timestamp"`
	}{
		Role:       "assistant",
		API:        "openai-completions",
		Provider:   piRPCProviderID,
		Model:      piRPCModelID,
		StopReason: "stop",
		Timestamp:  1,
	}
	if text != "" {
		message.Content = []content{{Type: "text", Text: text}}
	} else {
		message.Content = []content{}
	}
	value, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}
	return string(value)
}

func piRPCFixtureWithResponseID(message string, responseID string) string {
	return strings.Replace(
		message,
		`,"timestamp":1`,
		`,"responseId":"`+responseID+`","timestamp":1`,
		1,
	)
}

func piRPCFixtureWithUsageField(message string, field string) string {
	return strings.Replace(
		message,
		`"cacheWrite":0,"totalTokens":0`,
		`"cacheWrite":0,`+field+`,"totalTokens":0`,
		1,
	)
}

func piRPCFixtureWithUsageNumbers(
	message string,
	input int,
	output int,
	totalTokens int,
	reasoning bool,
) string {
	var value map[string]any
	if json.Unmarshal([]byte(message), &value) != nil {
		panic("invalid assistant fixture")
	}
	usage, ok := value["usage"].(map[string]any)
	if !ok {
		panic("invalid usage fixture")
	}
	usage["input"] = input
	usage["output"] = output
	usage["totalTokens"] = totalTokens
	if reasoning {
		usage["reasoning"] = 0
	} else {
		delete(usage, "reasoning")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func piRPCShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func mustDeltaJSON(t testing.TB, chunk []byte) []byte {
	t.Helper()
	payload, err := json.Marshal(struct {
		Delta string `json:"delta"`
	}{Delta: string(chunk)})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func mustJSONFrames(t testing.TB, frames []bridgev1.Frame) []byte {
	t.Helper()
	var result []byte
	for _, frame := range frames {
		line, err := bridgev1.EncodeLine(frame)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, line...)
	}
	return result
}
