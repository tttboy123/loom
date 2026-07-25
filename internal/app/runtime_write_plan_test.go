package app

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestPlanRuntimeObservationWritePrevalidatesContext(t *testing.T) {
	_, current, previous := appStatusSources(
		t, "write-plan-context", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOnline,
	)
	projected := appProjectedRuntimeSnapshot(
		t, previous, "write-plan-context",
	)
	tests := []struct {
		name string
		ctx  context.Context
		want error
	}{
		{name: "nil", want: ErrInvalidRuntimeObservationWritePlan},
		{name: "canceled", ctx: canceledAppDiscoveryContext(), want: context.Canceled},
		{
			name: "deadline", ctx: expiredAppDiscoveryContext(),
			want: context.DeadlineExceeded,
		},
		{
			name: "after_baseline", ctx: &cancelAfterErrCallsContext{cancelAt: 2},
			want: context.Canceled,
		},
		{
			name: "during_classification",
			ctx:  &cancelAfterErrCallsContext{cancelAt: 7},
			want: context.Canceled,
		},
		{
			name: "before_success",
			ctx:  &cancelAfterErrCallsContext{cancelAt: 8},
			want: context.Canceled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate, err := PlanRuntimeObservationWrite(
				test.ctx, projected, current,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroRuntimeObservationWritePlan(t, candidate)
		})
	}
}

func TestPlanRuntimeObservationWriteClassifiesNoneWithoutAbsence(
	t *testing.T,
) {
	emptyCurrent := appStatusSnapshot(
		t, "write-plan-empty", 0,
		loomruntime.RuntimeOnline, "device.local",
	)
	candidate, err := PlanRuntimeObservationWrite(
		context.Background(), projection.Snapshot{}, emptyCurrent,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeObservationWritePlan(
		t, candidate, RuntimeObservationWriteNone,
		emptyCurrent.Digest(), 0, 0, 0, 0,
	)

	_, unchanged, previous := appStatusSources(
		t, "write-plan-none", 2,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOnline,
	)
	projected := appProjectedRuntimeSnapshot(t, previous, "write-plan-none")
	candidate, err = PlanRuntimeObservationWrite(
		context.Background(), projected, unchanged,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeObservationWritePlan(
		t, candidate, RuntimeObservationWriteNone,
		unchanged.Digest(), 2, 2, 0, 0,
	)

	probeID := previous.Observations()[0].SourceProbeID
	presentOnly := runtimeWritePlanSnapshotFrom(
		t, previous, probeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			return observations[:1]
		},
	)
	candidate, err = PlanRuntimeObservationWrite(
		context.Background(), projected, presentOnly,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeObservationWritePlan(
		t, candidate, RuntimeObservationWriteNone,
		presentOnly.Digest(), 2, 1, 0, 0,
	)

	statusWithAbsent := runtimeWritePlanSnapshotFrom(
		t, previous, probeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations = observations[:1]
			observations[0].Instance.Status = loomruntime.RuntimeOffline
			return observations
		},
	)
	candidate, err = PlanRuntimeObservationWrite(
		context.Background(), projected, statusWithAbsent,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeObservationWritePlan(
		t, candidate, RuntimeObservationWriteStatus,
		statusWithAbsent.Digest(), 2, 1, 0, 1,
	)

	candidate, err = PlanRuntimeObservationWrite(
		context.Background(), projected, emptyCurrent,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeObservationWritePlan(
		t, candidate, RuntimeObservationWriteNone,
		emptyCurrent.Digest(), 2, 0, 0, 0,
	)
}

func TestPlanRuntimeObservationWriteClassifiesEveryInventoryChange(
	t *testing.T,
) {
	_, unchanged, previous := appStatusSources(
		t, "write-plan-inventory", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOnline,
	)
	projected := appProjectedRuntimeSnapshot(
		t, previous, "write-plan-inventory",
	)
	probeID := previous.Observations()[0].SourceProbeID

	currentOnly, err := PlanRuntimeObservationWrite(
		context.Background(), projection.Snapshot{}, unchanged,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeObservationWritePlan(
		t, currentOnly, RuntimeObservationWriteDiscovery,
		unchanged.Digest(), 0, 1, 1, 0,
	)

	tests := []struct {
		name    string
		probeID string
		mutate  func([]loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation
	}{
		{
			name: "display_name",
			mutate: func(input []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
				input[0].Instance.DisplayName = "Changed display"
				return input
			},
		},
		{
			name: "executable_version",
			mutate: func(input []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
				input[0].Instance.ExecutableVersion = "2.0.0"
				return input
			},
		},
		{
			name: "observed_capabilities",
			mutate: func(input []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
				input[0].Instance.ObservedCapabilities = []string{"go_test"}
				return input
			},
		},
		{
			name: "capacity",
			mutate: func(input []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
				input[0].Instance.Capacity = 2
				return input
			},
		},
		{
			name: "models",
			mutate: func(input []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
				input[0].ModelIDs = []string{"model.changed"}
				return input
			},
		},
		{
			name:    "source_probe",
			probeID: "probe.write-plan-inventory.changed",
			mutate: func(input []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
				return input
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selectedProbeID := test.probeID
			if selectedProbeID == "" {
				selectedProbeID = probeID
			}
			current := runtimeWritePlanSnapshotFrom(
				t, previous, selectedProbeID, test.mutate,
			)
			candidate, err := PlanRuntimeObservationWrite(
				context.Background(), projected, current,
			)
			if err != nil {
				t.Fatal(err)
			}
			assertRuntimeObservationWritePlan(
				t, candidate, RuntimeObservationWriteDiscovery,
				current.Digest(), 1, 1, 1, 0,
			)
		})
	}
}

func TestPlanRuntimeObservationWriteStatusAndDiscoveryPrecedence(
	t *testing.T,
) {
	for _, count := range []int{1, 2} {
		t.Run(fmt.Sprintf("status_only_%d", count), func(t *testing.T) {
			_, current, previous := appStatusSources(
				t, fmt.Sprintf("write-plan-status-%d", count), count,
				loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
			)
			projected := appProjectedRuntimeSnapshot(
				t, previous, fmt.Sprintf("write-plan-status-%d", count),
			)
			candidate, err := PlanRuntimeObservationWrite(
				context.Background(), projected, current,
			)
			if err != nil {
				t.Fatal(err)
			}
			assertRuntimeObservationWritePlan(
				t, candidate, RuntimeObservationWriteStatus,
				current.Digest(), count, count, 0, count,
			)
		})
	}

	_, _, previous := appStatusSources(
		t, "write-plan-mixed", 2,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOnline,
	)
	projected := appProjectedRuntimeSnapshot(t, previous, "write-plan-mixed")
	mixed := runtimeWritePlanSnapshotFrom(
		t, previous, previous.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DisplayName = "Changed inventory"
			for index := range observations {
				observations[index].Instance.Status = loomruntime.RuntimeOffline
			}
			return observations
		},
	)
	candidate, err := PlanRuntimeObservationWrite(
		context.Background(), projected, mixed,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeObservationWritePlan(
		t, candidate, RuntimeObservationWriteDiscovery,
		mixed.Digest(), 2, 2, 1, 2,
	)
}

func TestPlanRuntimeObservationWriteRejectsInvalidSources(t *testing.T) {
	_, current, previous := appStatusSources(
		t, "write-plan-invalid", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	projected := appProjectedRuntimeSnapshot(t, previous, "write-plan-invalid")

	invalidProjected := cloneProjectedRuntimeSnapshot(projected)
	record := invalidProjected.RuntimeInstances["runtime.write-plan-invalid.a"]
	record.DiscoveryEventID = ""
	invalidProjected.RuntimeInstances[record.ID] = record

	oversized := cloneProjectedRuntimeSnapshot(projected)
	template := oversized.RuntimeInstances["runtime.write-plan-invalid.a"]
	oversized.RuntimeInstances =
		make(map[string]projection.RuntimeInstance, 33)
	for index := 0; index < 33; index++ {
		id := fmt.Sprintf("runtime.write-plan-oversized.%02d", index)
		entry := cloneProjectedRuntimeRecord(template)
		entry.ID = id
		entry.DiscoveryEventID = "event.discovery." + id
		oversized.RuntimeInstances[id] = entry
	}

	identityDrift := runtimeWritePlanSnapshotFrom(
		t, current, current.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DeviceID = "device.other"
			return observations
		},
	)
	tests := []struct {
		name      string
		projected projection.Snapshot
		current   loomruntime.RuntimeDiscoverySnapshot
		want      error
	}{
		{
			name: "invalid_projection", projected: invalidProjected,
			current: current,
			want:    projection.ErrInvalidRuntimeStatusBaselineProjection,
		},
		{
			name: "oversized_projection", projected: oversized,
			current: current,
			want:    projection.ErrInvalidRuntimeStatusBaselineProjection,
		},
		{
			name: "invalid_discovery", projected: projected,
			current: loomruntime.RuntimeDiscoverySnapshot{},
			want:    loomruntime.ErrInvalidRuntimeStatusSource,
		},
		{
			name: "identity_drift", projected: projected,
			current: identityDrift,
			want:    loomruntime.ErrRuntimeStatusIdentityDrift,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate, err := PlanRuntimeObservationWrite(
				context.Background(), test.projected, test.current,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroRuntimeObservationWritePlan(t, candidate)
		})
	}
}

func TestPlanRuntimeObservationWriteDigestAndMutationIsolation(t *testing.T) {
	_, unchanged, previous := appStatusSources(
		t, "write-plan-digest", 2,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOnline,
	)
	projected := appProjectedRuntimeSnapshot(t, previous, "write-plan-digest")
	original := cloneProjectedRuntimeSnapshot(projected)

	first, err := PlanRuntimeObservationWrite(
		context.Background(), projected, unchanged,
	)
	if err != nil {
		t.Fatal(err)
	}
	reversed := projection.Snapshot{
		RuntimeInstances: make(map[string]projection.RuntimeInstance, 2),
	}
	records := []projection.RuntimeInstance{}
	for _, entry := range projected.RuntimeInstances {
		records = append(records, cloneProjectedRuntimeRecord(entry))
	}
	for index := len(records) - 1; index >= 0; index-- {
		reversed.RuntimeInstances[records[index].ID] = records[index]
	}
	second, err := PlanRuntimeObservationWrite(
		context.Background(), reversed, unchanged,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.CandidateDigest() != second.CandidateDigest() ||
		!reflect.DeepEqual(first, second) {
		t.Fatalf("map-order plans differ: first=%#v second=%#v", first, second)
	}
	if !reflect.DeepEqual(projected, original) {
		t.Fatal("projected Snapshot or nested slices were mutated")
	}

	presentOnly := runtimeWritePlanSnapshotFrom(
		t, previous, previous.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			return observations[:1]
		},
	)
	projectedCountChanged, err := PlanRuntimeObservationWrite(
		context.Background(), projected, presentOnly,
	)
	if err != nil {
		t.Fatal(err)
	}
	oneProjected := cloneProjectedRuntimeSnapshot(projected)
	delete(oneProjected.RuntimeInstances, "runtime.write-plan-digest.b")
	oneCount, err := PlanRuntimeObservationWrite(
		context.Background(), oneProjected, presentOnly,
	)
	if err != nil {
		t.Fatal(err)
	}
	if projectedCountChanged.CandidateDigest() == oneCount.CandidateDigest() {
		t.Fatal("projected count did not change Candidate digest")
	}

	statusCurrent := runtimeWritePlanSnapshotFrom(
		t, previous, previous.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.Status = loomruntime.RuntimeOffline
			return observations
		},
	)
	statusCandidate, err := PlanRuntimeObservationWrite(
		context.Background(), projected, statusCurrent,
	)
	if err != nil {
		t.Fatal(err)
	}
	discoveryCurrent := runtimeWritePlanSnapshotFrom(
		t, previous, previous.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DisplayName = "Digest inventory change"
			return observations
		},
	)
	discoveryCandidate, err := PlanRuntimeObservationWrite(
		context.Background(), projected, discoveryCurrent,
	)
	if err != nil {
		t.Fatal(err)
	}
	digests := map[string]bool{
		first.CandidateDigest():              true,
		statusCandidate.CandidateDigest():    true,
		discoveryCandidate.CandidateDigest(): true,
	}
	if len(digests) != 3 {
		t.Fatalf("semantic plan digests collided: %#v", digests)
	}

	digestFacts := []struct {
		name   string
		mutate func(*RuntimeObservationWritePlanCandidate)
	}{
		{
			name: "planned",
			mutate: func(candidate *RuntimeObservationWritePlanCandidate) {
				candidate.planned = false
			},
		},
		{
			name: "kind",
			mutate: func(candidate *RuntimeObservationWritePlanCandidate) {
				candidate.kind = RuntimeObservationWriteDiscovery
			},
		},
		{
			name: "source_discovery_digest",
			mutate: func(candidate *RuntimeObservationWritePlanCandidate) {
				candidate.sourceDiscoveryDigest = strings.Repeat("a", 64)
			},
		},
		{
			name: "projected_runtime_count",
			mutate: func(candidate *RuntimeObservationWritePlanCandidate) {
				candidate.projectedRuntimeCount++
			},
		},
		{
			name: "observed_runtime_count",
			mutate: func(candidate *RuntimeObservationWritePlanCandidate) {
				candidate.observedRuntimeCount++
			},
		},
		{
			name: "inventory_change_count",
			mutate: func(candidate *RuntimeObservationWritePlanCandidate) {
				candidate.inventoryChangeCount++
			},
		},
		{
			name: "status_transition_count",
			mutate: func(candidate *RuntimeObservationWritePlanCandidate) {
				candidate.statusTransitionCount++
			},
		},
	}
	for _, test := range digestFacts {
		t.Run("digest_"+test.name, func(t *testing.T) {
			mutated := first
			test.mutate(&mutated)
			got, err := digestRuntimeObservationWritePlan(mutated)
			if err != nil {
				t.Fatal(err)
			}
			if got == first.CandidateDigest() {
				t.Fatalf("%s did not change Candidate digest", test.name)
			}
		})
	}

	assertZeroRuntimeObservationWritePlan(
		t, RuntimeObservationWritePlanCandidate{},
	)
}

func TestPlanRuntimeObservationWriteStaticBoundary(t *testing.T) {
	const productPath = "runtime_write_plan.go"
	source, err := os.ReadFile(productPath)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(
		token.NewFileSet(), productPath, source, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"context": true, "crypto/sha256": true, "encoding/hex": true,
		"encoding/json": true, "errors": true, "reflect": true,
		"loom-pi-rebuild/internal/projection": true,
		"loom-pi-rebuild/internal/runtime":    true,
	}
	for _, imported := range parsed.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		if !allowed[path] {
			t.Fatalf("forbidden product import %q", path)
		}
	}
	forbiddenSelectors := map[string]bool{
		"CommitRuntimeDiscovery":                      true,
		"CommitRuntimeStatus":                         true,
		"CommitRuntimeDiscoverySnapshot":              true,
		"CommitRuntimeStatusTransitions":              true,
		"RunConfiguredRuntimeDiscoveryOnce":           true,
		"RunProjectedRuntimeStatusReconciliationOnce": true,
		"DiscoverRuntime":                             true,
		"Rebuild":                                     true,
		"AppendBatch":                                 true,
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.GoStmt:
			t.Error("forbidden go statement")
		case *ast.SelectorExpr:
			if forbiddenSelectors[typed.Sel.Name] {
				t.Errorf("forbidden selector %q", typed.Sel.Name)
			}
		}
		return true
	})
	product := string(source)
	for _, marker := range []string{
		"internal/journal", "internal/state", "internal/runtime/discoveryscan",
		"RuntimeInstanceDiscovered", "RuntimeInstanceStatusChanged",
		"IdempotencyKey", "EmittedAt", "NewTicker", "Sleep(",
		"os.", "exec.", "net.", "http.", "sqlite", "daemon",
		"AgentGrant", "WorkItem", "Bridge",
	} {
		if strings.Contains(product, marker) {
			t.Errorf("forbidden product marker %q", marker)
		}
	}
}

func runtimeWritePlanSnapshotFrom(
	t *testing.T,
	input loomruntime.RuntimeDiscoverySnapshot,
	probeID string,
	mutate func([]loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation,
) loomruntime.RuntimeDiscoverySnapshot {
	t.Helper()
	observations := input.Observations()
	observations = mutate(observations)
	snapshot, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{&appDiscoveryProbe{
			id: probeID, observations: observations,
		}},
	)
	if err != nil {
		t.Fatalf("DiscoverRuntime() error = %v", err)
	}
	return snapshot
}

func assertRuntimeObservationWritePlan(
	t *testing.T,
	candidate RuntimeObservationWritePlanCandidate,
	kind RuntimeObservationWriteKind,
	sourceDigest string,
	projectedCount int,
	observedCount int,
	inventoryCount int,
	statusCount int,
) {
	t.Helper()
	if !candidate.Planned() ||
		candidate.Kind() != kind ||
		candidate.SourceDiscoveryDigest() != sourceDigest ||
		candidate.ProjectedRuntimeCount() != projectedCount ||
		candidate.ObservedRuntimeCount() != observedCount ||
		candidate.InventoryChangeCount() != inventoryCount ||
		candidate.StatusTransitionCount() != statusCount ||
		!isAppDiscoveryDigest(candidate.CandidateDigest()) {
		t.Fatalf("plan = %#v, unexpected facts", candidate)
	}
}

func assertZeroRuntimeObservationWritePlan(
	t *testing.T,
	candidate RuntimeObservationWritePlanCandidate,
) {
	t.Helper()
	if candidate.Planned() ||
		candidate.Kind() != "" ||
		candidate.SourceDiscoveryDigest() != "" ||
		candidate.ProjectedRuntimeCount() != 0 ||
		candidate.ObservedRuntimeCount() != 0 ||
		candidate.InventoryChangeCount() != 0 ||
		candidate.StatusTransitionCount() != 0 ||
		candidate.CandidateDigest() != "" {
		t.Fatalf("candidate = %#v, want zero", candidate)
	}
}
