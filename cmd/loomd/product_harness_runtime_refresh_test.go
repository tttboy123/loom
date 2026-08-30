package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/composition"
)

type productHarnessRuntimeDiagnosticSink struct {
	records []productOperationalDiagnosticRecord
}

func (sink *productHarnessRuntimeDiagnosticSink) operationalNow() time.Time {
	return time.Date(2026, 8, 27, 7, 0, 0, 0, time.UTC)
}

func (*productHarnessRuntimeDiagnosticSink) credentialRuntimeValue() string {
	return productCredentialRuntimeVault
}

func (sink *productHarnessRuntimeDiagnosticSink) append(
	record productOperationalDiagnosticRecord,
) error {
	if !validProductOperationalDiagnosticRecord(record) {
		return errors.New("invalid record")
	}
	sink.records = append(sink.records, record)
	return nil
}

func TestPhase5HarnessRuntimeRefresherReusesSuccessfulBackgroundRefreshForMission(t *testing.T) {
	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	calls := 0
	refresher := &productHarnessRuntimeRefresher{
		refresh: func(context.Context) error {
			calls++
			if calls == 1 {
				close(firstStarted)
				<-releaseFirst
			} else {
				close(secondStarted)
			}
			return nil
		},
	}
	firstDone := make(chan error, 1)
	go func() { firstDone <- refresher.Refresh(context.Background()) }()
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("background refresh did not start")
	}
	secondDone := make(chan error, 1)
	go func() { secondDone <- refresher.Refresh(context.Background()) }()
	select {
	case <-secondStarted:
		t.Fatal("Mission refresh raced the background refresh")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseFirst)
	for name, done := range map[string]<-chan error{
		"background": firstDone,
		"mission":    secondDone,
	} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("%s refresh error = %v", name, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s refresh did not finish", name)
		}
	}
	select {
	case <-secondStarted:
		t.Fatal("Mission repeated a successful background Harness refresh")
	default:
	}
	if calls != 1 {
		t.Fatalf("refresh calls = %d", calls)
	}
}

func TestPhase5HarnessRuntimeRefresherRetriesAfterBackgroundFailure(t *testing.T) {
	want := errors.New("transient catalog failure")
	calls := 0
	refresher := &productHarnessRuntimeRefresher{
		refresh: func(context.Context) error {
			calls++
			if calls == 1 {
				return want
			}
			return nil
		},
	}
	if err := refresher.Refresh(context.Background()); !errors.Is(err, want) {
		t.Fatalf("background refresh error = %v", err)
	}
	if err := refresher.Refresh(context.Background()); err != nil {
		t.Fatalf("Mission retry error = %v", err)
	}
	if err := refresher.Refresh(context.Background()); err != nil {
		t.Fatalf("memoized refresh error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("underlying refresh calls = %d", calls)
	}
}

func TestPhase5HarnessRuntimeRefresherAdmitsAgentRuntimeOnlyAfterIPCReady(t *testing.T) {
	refreshStarted := make(chan struct{})
	refresher := &productHarnessRuntimeRefresher{
		admitted: make(chan struct{}),
		refresh: func(context.Context) error {
			close(refreshStarted)
			return nil
		},
	}
	agentRuntimeDone := make(chan error, 1)
	go func() { agentRuntimeDone <- refresher.Refresh(context.Background()) }()
	select {
	case <-refreshStarted:
		t.Fatal("Agent Runtime refreshed Harnesses before local IPC was ready")
	case <-time.After(50 * time.Millisecond):
	}
	if err := refresher.RefreshAfterReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-agentRuntimeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Agent Runtime did not reuse the admitted refresh")
	}
	if !refresher.ready {
		t.Fatal("successful after-ready refresh was not memoized")
	}
}

func TestPhase5MissionRuntimeProjectionUsesSharedHarnessRefresh(t *testing.T) {
	calls := 0
	config := productMissionExecutionRuntimeConfig{
		HarnessRuntimeRefresh: func(context.Context) error {
			calls++
			return nil
		},
	}
	if err := ensureProductMissionRuntimeProjection(
		context.Background(), nil, nil, time.Now().UTC, config,
	); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("shared Harness refresh calls = %d", calls)
	}
}

func TestPhase5HarnessRuntimeRefreshDiagnosticIsPrivacySafeAndClassified(t *testing.T) {
	sink := &productHarnessRuntimeDiagnosticSink{}
	if err := recordProductHarnessRuntimeRefreshOutcome(
		sink,
		errors.New("private executable detail"),
		composition.ProfileDesktop,
		strings.Repeat("a", 64),
		125*time.Millisecond,
	); err != nil {
		t.Fatal(err)
	}
	if len(sink.records) != 1 {
		t.Fatalf("diagnostic records = %d", len(sink.records))
	}
	record := sink.records[0]
	if record.Stage != "bundle_start" || record.Result != "failed" ||
		record.ErrorCode != "runtime_catalog_refresh_failed" || !record.Retryable ||
		record.ElapsedMS != 125 || record.BundleID != "loom-runtime-catalog" {
		t.Fatalf("diagnostic = %#v", record)
	}
	encoded := productOperationalDiagnosticJSON(record)
	if strings.Contains(encoded, "private executable detail") {
		t.Fatal("diagnostic exposed the internal Harness error")
	}
}

func productOperationalDiagnosticJSON(record productOperationalDiagnosticRecord) string {
	encoded, _ := record.MarshalJSON()
	return string(encoded)
}
