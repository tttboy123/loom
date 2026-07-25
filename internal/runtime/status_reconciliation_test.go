package runtime

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReconcileObservedRuntimeStatusesCoversAcceptedTransitions(t *testing.T) {
	statuses := []RuntimeStatus{
		RuntimeOnline,
		RuntimeOffline,
		RuntimeIncompatible,
		RuntimeDisabled,
	}
	for _, from := range statuses {
		for _, to := range statuses {
			name := string(from) + "_to_" + string(to)
			t.Run(name, func(t *testing.T) {
				baseline := []RuntimeStatusBaseline{
					statusBaseline(t, "runtime.a", from, 1),
				}
				current := statusDiscovery(t, []statusObservationSpec{
					{id: "runtime.a", status: to},
				})
				candidate, err := ReconcileObservedRuntimeStatuses(
					context.Background(),
					baseline,
					current,
				)
				if err != nil {
					t.Fatalf("ReconcileObservedRuntimeStatuses() error = %v", err)
				}
				if !candidate.Reconciled() ||
					!validRuntimeStatusDigest(candidate.BaselineDigest()) ||
					candidate.SourceDiscoveryDigest() != current.Digest() ||
					!validRuntimeStatusDigest(candidate.CandidateDigest()) {
					t.Fatalf("Candidate = %#v", candidate)
				}
				if from == to {
					if candidate.TransitionCount() != 0 ||
						len(candidate.Transitions()) != 0 {
						t.Fatalf("same-status transitions = %#v", candidate.Transitions())
					}
					return
				}
				transitions := candidate.Transitions()
				if candidate.TransitionCount() != 1 || len(transitions) != 1 {
					t.Fatalf("transition Candidate = %#v", candidate)
				}
				got := transitions[0]
				if got.RuntimeInstanceID != "runtime.a" ||
					got.DeviceID != "device.runtime.a" ||
					got.AdapterType != "pi-cli" ||
					got.FromStatus != from ||
					got.ToStatus != to ||
					got.SourceProbeID != "probe.status" ||
					got.SourceDiscoveryDigest != current.Digest() ||
					got.PreviousEventID != "event.runtime.a" ||
					got.PreviousSequence != 1 {
					t.Fatalf("transition = %#v", got)
				}
			})
		}
	}
}

func TestReconcileObservedRuntimeStatusesOrdersAndDigestsDeterministically(t *testing.T) {
	baseline := []RuntimeStatusBaseline{
		statusBaseline(t, "runtime.b", RuntimeOnline, 2),
		statusBaseline(t, "runtime.a", RuntimeOffline, 1),
	}
	current := statusDiscovery(t, []statusObservationSpec{
		{id: "runtime.b", status: RuntimeDisabled},
		{id: "runtime.a", status: RuntimeOnline},
	})
	first, err := ReconcileObservedRuntimeStatuses(
		context.Background(), baseline, current,
	)
	if err != nil {
		t.Fatal(err)
	}
	baseline[0], baseline[1] = baseline[1], baseline[0]
	second, err := ReconcileObservedRuntimeStatuses(
		context.Background(), baseline, current,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.BaselineDigest() != second.BaselineDigest() ||
		first.CandidateDigest() != second.CandidateDigest() ||
		!reflect.DeepEqual(first.Transitions(), second.Transitions()) {
		t.Fatalf("reordered baseline changed Candidate: first=%#v second=%#v", first, second)
	}
	transitions := first.Transitions()
	if len(transitions) != 2 ||
		transitions[0].RuntimeInstanceID != "runtime.a" ||
		transitions[1].RuntimeInstanceID != "runtime.b" {
		t.Fatalf("transition order = %#v", transitions)
	}
}

func TestReconcileObservedRuntimeStatusesUsesPreviousStatusBearingProvenance(t *testing.T) {
	baseline := []RuntimeStatusBaseline{
		statusBaseline(t, "runtime.a", RuntimeOnline, 7),
	}
	baseline[0].PreviousEventID = "event.status.runtime.a"
	current := statusDiscovery(t, []statusObservationSpec{
		{id: "runtime.a", status: RuntimeOffline},
	})
	candidate, err := ReconcileObservedRuntimeStatuses(
		context.Background(), baseline, current,
	)
	if err != nil {
		t.Fatal(err)
	}
	transitions := candidate.Transitions()
	if len(transitions) != 1 ||
		transitions[0].PreviousEventID != "event.status.runtime.a" ||
		transitions[0].PreviousSequence != 7 {
		t.Fatalf("transition provenance = %#v", transitions)
	}
}

func TestReconcileObservedRuntimeStatusesDoesNotInferAbsence(t *testing.T) {
	baseline := []RuntimeStatusBaseline{
		statusBaseline(t, "runtime.baseline", RuntimeOnline, 1),
	}
	cases := []struct {
		name     string
		baseline []RuntimeStatusBaseline
		current  RuntimeDiscoverySnapshot
	}{
		{
			name:     "empty completed discovery",
			baseline: baseline,
			current:  statusDiscovery(t, nil),
		},
		{
			name:     "baseline only",
			baseline: baseline,
			current: statusDiscovery(t, []statusObservationSpec{
				{id: "runtime.other", status: RuntimeOffline},
			}),
		},
		{
			name:     "current only",
			baseline: nil,
			current: statusDiscovery(t, []statusObservationSpec{
				{id: "runtime.new", status: RuntimeDisabled},
			}),
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			candidate, err := ReconcileObservedRuntimeStatuses(
				context.Background(), test.baseline, test.current,
			)
			if err != nil {
				t.Fatal(err)
			}
			if !candidate.Reconciled() ||
				candidate.TransitionCount() != 0 ||
				len(candidate.Transitions()) != 0 {
				t.Fatalf("absence fabricated transitions: %#v", candidate)
			}
		})
	}
}

func TestReconcileObservedRuntimeStatusesIgnoresNonStatusInventoryChanges(t *testing.T) {
	baseline := []RuntimeStatusBaseline{
		statusBaseline(t, "runtime.a", RuntimeOnline, 1),
	}
	tests := []struct {
		name   string
		change func(*RuntimeDiscoverySnapshot)
	}{
		{name: "display name", change: func(source *RuntimeDiscoverySnapshot) {
			source.observations[0].Instance.DisplayName = "Changed"
		}},
		{name: "executable version", change: func(source *RuntimeDiscoverySnapshot) {
			source.observations[0].Instance.ExecutableVersion = "2.0.0"
		}},
		{name: "capabilities", change: func(source *RuntimeDiscoverySnapshot) {
			source.observations[0].Instance.ObservedCapabilities =
				[]string{"models", "sandbox", "version"}
		}},
		{name: "capacity", change: func(source *RuntimeDiscoverySnapshot) {
			source.observations[0].Instance.Capacity = 2
		}},
		{name: "models", change: func(source *RuntimeDiscoverySnapshot) {
			source.observations[0].ModelIDs =
				[]string{"provider/model", "provider/model-b"}
		}},
		{name: "source probe", change: func(source *RuntimeDiscoverySnapshot) {
			source.observations[0].SourceProbeID = "probe.changed"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := statusDiscovery(t, []statusObservationSpec{
				{id: "runtime.a", status: RuntimeOnline},
			})
			test.change(&source)
			digest, err := digestRuntimeDiscovery(source.observations)
			if err != nil {
				t.Fatal(err)
			}
			source.digest = digest
			candidate, err := ReconcileObservedRuntimeStatuses(
				context.Background(), baseline, source,
			)
			if err != nil {
				t.Fatalf("ReconcileObservedRuntimeStatuses() error = %v", err)
			}
			if !candidate.Reconciled() ||
				candidate.TransitionCount() != 0 ||
				len(candidate.Transitions()) != 0 {
				t.Fatalf("non-status change produced transition: %#v", candidate)
			}
		})
	}
}

func TestReconcileObservedRuntimeStatusesRejectsIdentityAndInvalidSources(t *testing.T) {
	validBaseline := []RuntimeStatusBaseline{
		statusBaseline(t, "runtime.a", RuntimeOnline, 1),
	}
	validCurrent := statusDiscovery(t, []statusObservationSpec{
		{id: "runtime.a", status: RuntimeOffline},
	})

	for _, test := range []struct {
		name    string
		current RuntimeDiscoverySnapshot
	}{
		{
			name: "device drift",
			current: statusDiscovery(t, []statusObservationSpec{{
				id: "runtime.a", status: RuntimeOffline, deviceID: "device.other",
			}}),
		},
		{
			name: "adapter drift",
			current: statusDiscovery(t, []statusObservationSpec{{
				id: "runtime.a", status: RuntimeOffline, adapterType: "other",
			}}),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := ReconcileObservedRuntimeStatuses(
				context.Background(), validBaseline, test.current,
			)
			if !errors.Is(err, ErrRuntimeStatusIdentityDrift) {
				t.Fatalf("error = %v", err)
			}
			assertZeroRuntimeStatusCandidate(t, got)
		})
	}

	baselineCases := []struct {
		name   string
		change func([]RuntimeStatusBaseline) []RuntimeStatusBaseline
	}{
		{name: "invalid instance", change: func(input []RuntimeStatusBaseline) []RuntimeStatusBaseline {
			input[0].Instance.ID = ""
			return input
		}},
		{name: "noncanonical capabilities", change: func(input []RuntimeStatusBaseline) []RuntimeStatusBaseline {
			input[0].Instance.ObservedCapabilities = []string{"version", "models"}
			return input
		}},
		{name: "empty event id", change: func(input []RuntimeStatusBaseline) []RuntimeStatusBaseline {
			input[0].PreviousEventID = ""
			return input
		}},
		{name: "zero sequence", change: func(input []RuntimeStatusBaseline) []RuntimeStatusBaseline {
			input[0].PreviousSequence = 0
			return input
		}},
		{name: "duplicate id", change: func(input []RuntimeStatusBaseline) []RuntimeStatusBaseline {
			return append(input, input[0])
		}},
		{name: "oversized", change: func([]RuntimeStatusBaseline) []RuntimeStatusBaseline {
			result := make([]RuntimeStatusBaseline, 33)
			for index := range result {
				result[index] = statusBaseline(
					t,
					"runtime."+string(rune('A'+index)),
					RuntimeOnline,
					int64(index+1),
				)
			}
			return result
		}},
	}
	for _, test := range baselineCases {
		t.Run(test.name, func(t *testing.T) {
			input := cloneStatusBaselines(validBaseline)
			got, err := ReconcileObservedRuntimeStatuses(
				context.Background(), test.change(input), validCurrent,
			)
			if !errors.Is(err, ErrInvalidRuntimeStatusBaseline) {
				t.Fatalf("error = %v", err)
			}
			assertZeroRuntimeStatusCandidate(t, got)
		})
	}

	sourceCases := []struct {
		name   string
		source RuntimeDiscoverySnapshot
	}{
		{name: "zero", source: RuntimeDiscoverySnapshot{}},
		{name: "digest mismatch", source: func() RuntimeDiscoverySnapshot {
			source := validCurrent
			source.digest = strings.Repeat("f", 64)
			return source
		}()},
		{name: "unsorted observations", source: func() RuntimeDiscoverySnapshot {
			source := statusDiscovery(t, []statusObservationSpec{
				{id: "runtime.a", status: RuntimeOnline},
				{id: "runtime.b", status: RuntimeOnline},
			})
			source.observations[0], source.observations[1] =
				source.observations[1], source.observations[0]
			return source
		}()},
		{name: "empty probe", source: func() RuntimeDiscoverySnapshot {
			source := validCurrent
			source.observations[0].SourceProbeID = ""
			return source
		}()},
		{name: "noncanonical models", source: func() RuntimeDiscoverySnapshot {
			source := validCurrent
			source.observations[0].ModelIDs = []string{"model.z", "model.a"}
			return source
		}()},
		{name: "invalid runtime", source: func() RuntimeDiscoverySnapshot {
			source := validCurrent
			source.observations[0].Instance.Status = "busy"
			return source
		}()},
		{name: "oversized", source: func() RuntimeDiscoverySnapshot {
			specs := make([]statusObservationSpec, 33)
			for index := range specs {
				specs[index] = statusObservationSpec{
					id:     "runtime." + string(rune('A'+index)),
					status: RuntimeOnline,
				}
			}
			return statusDiscovery(t, specs)
		}()},
	}
	for _, test := range sourceCases {
		t.Run(test.name, func(t *testing.T) {
			got, err := ReconcileObservedRuntimeStatuses(
				context.Background(), validBaseline, test.source,
			)
			if !errors.Is(err, ErrInvalidRuntimeStatusSource) {
				t.Fatalf("error = %v", err)
			}
			assertZeroRuntimeStatusCandidate(t, got)
		})
	}

	for name, ctx := range map[string]context.Context{
		"nil": nil,
		"canceled": func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}(),
		"deadline": func() context.Context {
			ctx, cancel := context.WithDeadline(
				context.Background(), time.Now().Add(-time.Second),
			)
			t.Cleanup(cancel)
			return ctx
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ReconcileObservedRuntimeStatuses(
				ctx, validBaseline, validCurrent,
			)
			if err == nil {
				t.Fatal("error = nil")
			}
			assertZeroRuntimeStatusCandidate(t, got)
		})
	}
}

func TestReconcileObservedRuntimeStatusesIsMutationIsolated(t *testing.T) {
	baseline := []RuntimeStatusBaseline{
		statusBaseline(t, "runtime.a", RuntimeOnline, 1),
	}
	current := statusDiscovery(t, []statusObservationSpec{
		{id: "runtime.a", status: RuntimeOffline},
	})
	candidate, err := ReconcileObservedRuntimeStatuses(
		context.Background(), baseline, current,
	)
	if err != nil {
		t.Fatal(err)
	}
	beforeDigest := candidate.CandidateDigest()
	baseline[0].Instance.Status = RuntimeDisabled
	baseline[0].Instance.ObservedCapabilities[0] = "mutated"
	current.observations[0].Instance.Status = RuntimeDisabled
	accessor := candidate.Transitions()
	accessor[0].ToStatus = RuntimeDisabled

	got := candidate.Transitions()
	if got[0].FromStatus != RuntimeOnline ||
		got[0].ToStatus != RuntimeOffline ||
		candidate.CandidateDigest() != beforeDigest {
		t.Fatalf("Candidate mutated: %#v", candidate)
	}
}

func TestReconcileObservedRuntimeStatusesDigestCoversSemantics(t *testing.T) {
	baseline := []RuntimeStatusBaseline{
		statusBaseline(t, "runtime.a", RuntimeOnline, 1),
	}
	current := statusDiscovery(t, []statusObservationSpec{
		{id: "runtime.a", status: RuntimeOffline},
	})
	candidate, err := ReconcileObservedRuntimeStatuses(
		context.Background(), baseline, current,
	)
	if err != nil {
		t.Fatal(err)
	}
	baselineChanged := cloneStatusBaselines(baseline)
	if runtimeStatusBaselineDigestVersion != 2 {
		t.Fatalf("runtimeStatusBaselineDigestVersion = %d, want 2", runtimeStatusBaselineDigestVersion)
	}
	baselineChanged[0].PreviousSequence++
	changedBaseline, err := ReconcileObservedRuntimeStatuses(
		context.Background(), baselineChanged, current,
	)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.BaselineDigest() == changedBaseline.BaselineDigest() ||
		candidate.CandidateDigest() == changedBaseline.CandidateDigest() {
		t.Fatal("baseline semantic change did not change digests")
	}
	expandedBaseline := append(
		cloneStatusBaselines(baseline),
		statusBaseline(t, "runtime.b", RuntimeOnline, 2),
	)
	expandedCandidate, err := ReconcileObservedRuntimeStatuses(
		context.Background(), expandedBaseline, current,
	)
	if err != nil {
		t.Fatal(err)
	}
	expandedBaseline[0], expandedBaseline[1] =
		expandedBaseline[1], expandedBaseline[0]
	reorderedExpanded, err := ReconcileObservedRuntimeStatuses(
		context.Background(), expandedBaseline, current,
	)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.BaselineDigest() == expandedCandidate.BaselineDigest() ||
		candidate.CandidateDigest() == expandedCandidate.CandidateDigest() {
		t.Fatal("baseline set change did not change both digests")
	}
	if expandedCandidate.BaselineDigest() != reorderedExpanded.BaselineDigest() ||
		expandedCandidate.CandidateDigest() != reorderedExpanded.CandidateDigest() {
		t.Fatal("reordered expanded baseline changed digests")
	}

	baselineMutations := []func(*RuntimeStatusBaseline){
		func(value *RuntimeStatusBaseline) { value.Instance.ID += ".changed" },
		func(value *RuntimeStatusBaseline) { value.Instance.DeviceID += ".changed" },
		func(value *RuntimeStatusBaseline) { value.Instance.AdapterType += ".changed" },
		func(value *RuntimeStatusBaseline) { value.Instance.DisplayName += ".changed" },
		func(value *RuntimeStatusBaseline) { value.Instance.ExecutableVersion += ".changed" },
		func(value *RuntimeStatusBaseline) { value.Instance.Status = RuntimeDisabled },
		func(value *RuntimeStatusBaseline) {
			value.Instance.ObservedCapabilities = []string{"models", "sandbox", "version"}
		},
		func(value *RuntimeStatusBaseline) { value.Instance.Capacity++ },
		func(value *RuntimeStatusBaseline) { value.PreviousEventID += ".changed" },
		func(value *RuntimeStatusBaseline) { value.PreviousSequence++ },
	}
	for index, mutate := range baselineMutations {
		changed := cloneStatusBaselines(baseline)
		mutate(&changed[0])
		ordered, err := validateRuntimeStatusBaseline(changed)
		if err != nil {
			t.Fatalf("baseline mutation %d invalid: %v", index, err)
		}
		digest, err := digestRuntimeStatusBaseline(ordered)
		if err != nil {
			t.Fatal(err)
		}
		if digest == candidate.BaselineDigest() {
			t.Fatalf("baseline mutation %d did not change digest", index)
		}
	}

	mutations := []func(*RuntimeStatusReconciliationCandidate){
		func(value *RuntimeStatusReconciliationCandidate) { value.baselineDigest = strings.Repeat("a", 64) },
		func(value *RuntimeStatusReconciliationCandidate) {
			value.sourceDiscoveryDigest = strings.Repeat("b", 64)
		},
		func(value *RuntimeStatusReconciliationCandidate) { value.transitionCount++ },
		func(value *RuntimeStatusReconciliationCandidate) {
			value.transitions[0].RuntimeInstanceID += ".changed"
		},
		func(value *RuntimeStatusReconciliationCandidate) { value.transitions[0].DeviceID += ".changed" },
		func(value *RuntimeStatusReconciliationCandidate) { value.transitions[0].AdapterType += ".changed" },
		func(value *RuntimeStatusReconciliationCandidate) { value.transitions[0].FromStatus = RuntimeDisabled },
		func(value *RuntimeStatusReconciliationCandidate) { value.transitions[0].ToStatus = RuntimeIncompatible },
		func(value *RuntimeStatusReconciliationCandidate) { value.transitions[0].SourceProbeID += ".changed" },
		func(value *RuntimeStatusReconciliationCandidate) {
			value.transitions[0].SourceDiscoveryDigest = strings.Repeat("c", 64)
		},
		func(value *RuntimeStatusReconciliationCandidate) { value.transitions[0].PreviousEventID += ".changed" },
		func(value *RuntimeStatusReconciliationCandidate) { value.transitions[0].PreviousSequence++ },
	}
	for index, mutate := range mutations {
		clone := cloneRuntimeStatusCandidate(candidate)
		mutate(&clone)
		digest, err := digestRuntimeStatusCandidate(clone)
		if err != nil {
			t.Fatal(err)
		}
		if digest == candidate.CandidateDigest() {
			t.Fatalf("mutation %d did not change Candidate digest", index)
		}
	}
}

func TestReconcileObservedRuntimeStatusesProductionBoundary(t *testing.T) {
	content, err := os.ReadFile("status_reconciliation.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(
		token.NewFileSet(),
		"status_reconciliation.go",
		content,
		parser.ImportsOnly,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range file.Imports {
		if strings.HasPrefix(imported.Path.Value, `"loom-pi-rebuild/`) {
			t.Fatalf("product imports forbidden project package %s", imported.Path.Value)
		}
	}
	for _, forbidden := range []string{
		"internal/journal",
		"internal/state",
		"internal/projection",
		"piadapter",
		"database/sql",
		"os/exec",
		"net/",
		"os.",
		"Append",
		"RuntimeInstanceStatusChanged",
		"goroutine",
		"credential",
		"scheduler",
		"daemon",
	} {
		if strings.Contains(string(content), forbidden) {
			t.Fatalf("product contains forbidden surface %q", forbidden)
		}
	}
}

type statusObservationSpec struct {
	id          string
	status      RuntimeStatus
	deviceID    string
	adapterType string
}

type statusProbe struct {
	observations []RuntimeObservation
}

func (statusProbe) ID() string {
	return "probe.status"
}

func (p statusProbe) ObserveRuntime(context.Context) ([]RuntimeObservation, error) {
	return copyRuntimeObservations(p.observations), nil
}

func statusDiscovery(
	t *testing.T,
	specs []statusObservationSpec,
) RuntimeDiscoverySnapshot {
	t.Helper()
	observations := make([]RuntimeObservation, len(specs))
	for index, spec := range specs {
		deviceID := spec.deviceID
		if deviceID == "" {
			deviceID = "device." + spec.id
		}
		adapterType := spec.adapterType
		if adapterType == "" {
			adapterType = "pi-cli"
		}
		instance, err := NewRuntimeInstance(RuntimeInstance{
			ID:                   spec.id,
			DeviceID:             deviceID,
			AdapterType:          adapterType,
			DisplayName:          "Runtime " + spec.id,
			ExecutableVersion:    "1.0.0",
			Status:               spec.status,
			ObservedCapabilities: []string{"models", "version"},
			Capacity:             1,
		})
		if err != nil {
			t.Fatal(err)
		}
		observations[index] = RuntimeObservation{
			Instance: instance,
			ModelIDs: []string{"provider/model"},
		}
	}
	snapshot, err := DiscoverRuntime(
		context.Background(),
		[]RuntimeProbe{statusProbe{observations: observations}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func statusBaseline(
	t *testing.T,
	id string,
	status RuntimeStatus,
	sequence int64,
) RuntimeStatusBaseline {
	t.Helper()
	instance, err := NewRuntimeInstance(RuntimeInstance{
		ID:                   id,
		DeviceID:             "device." + id,
		AdapterType:          "pi-cli",
		DisplayName:          "Runtime " + id,
		ExecutableVersion:    "1.0.0",
		Status:               status,
		ObservedCapabilities: []string{"models", "version"},
		Capacity:             1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return RuntimeStatusBaseline{
		Instance:         instance,
		PreviousEventID:  "event." + id,
		PreviousSequence: sequence,
	}
}

func cloneStatusBaselines(input []RuntimeStatusBaseline) []RuntimeStatusBaseline {
	result := make([]RuntimeStatusBaseline, len(input))
	for index, baseline := range input {
		baseline.Instance = copyRuntimeInstance(baseline.Instance)
		result[index] = baseline
	}
	return result
}

func assertZeroRuntimeStatusCandidate(
	t *testing.T,
	candidate RuntimeStatusReconciliationCandidate,
) {
	t.Helper()
	if !reflect.DeepEqual(candidate, RuntimeStatusReconciliationCandidate{}) {
		t.Fatalf("Candidate = %#v, want zero", candidate)
	}
}
