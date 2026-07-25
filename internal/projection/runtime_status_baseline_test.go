package projection

import (
	"errors"
	"go/parser"
	"go/token"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestBuildRuntimeStatusBaselinesSelectsCanonicalLatestFacts(t *testing.T) {
	discovery := runtimeStatusBaselineRecord("runtime.discovery", 1)
	firstStatus := runtimeStatusBaselineWithStatus(
		runtimeStatusBaselineRecord("runtime.first", 3),
		"event.discovery.runtime.first",
		3,
		"event.status.runtime.first",
		4,
	)
	consecutive := runtimeStatusBaselineWithStatus(
		runtimeStatusBaselineRecord("runtime.consecutive", 5),
		"event.status.previous.runtime.consecutive",
		7,
		"event.status.current.runtime.consecutive",
		8,
	)
	rediscovered := runtimeStatusBaselineRecord("runtime.rediscovered", 11)

	got, err := BuildRuntimeStatusBaselines(Snapshot{
		RuntimeInstances: map[string]RuntimeInstance{
			rediscovered.ID: rediscovered,
			discovery.ID:    discovery,
			consecutive.ID:  consecutive,
			firstStatus.ID:  firstStatus,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("baseline count = %d, want 4", len(got))
	}
	want := []struct {
		id       string
		eventID  string
		sequence int64
	}{
		{"runtime.consecutive", "event.status.current.runtime.consecutive", 8},
		{"runtime.discovery", "event.discovery.runtime.discovery", 1},
		{"runtime.first", "event.status.runtime.first", 4},
		{"runtime.rediscovered", "event.discovery.runtime.rediscovered", 11},
	}
	for index, expected := range want {
		entry := got[index]
		if entry.Instance.ID != expected.id ||
			entry.PreviousEventID != expected.eventID ||
			entry.PreviousSequence != expected.sequence {
			t.Fatalf("baseline[%d] = %#v, want %#v", index, entry, expected)
		}
		projected := map[string]RuntimeInstance{
			discovery.ID:    discovery,
			firstStatus.ID:  firstStatus,
			consecutive.ID:  consecutive,
			rediscovered.ID: rediscovered,
		}[expected.id]
		wantInstance := loomruntime.RuntimeInstance{
			ID:                   projected.ID,
			DeviceID:             projected.DeviceID,
			AdapterType:          projected.AdapterType,
			DisplayName:          projected.DisplayName,
			ExecutableVersion:    projected.ExecutableVersion,
			Status:               loomruntime.RuntimeStatus(projected.Status),
			ObservedCapabilities: append([]string(nil), projected.ObservedCapabilities...),
			Capacity:             projected.Capacity,
		}
		if !reflect.DeepEqual(entry.Instance, wantInstance) {
			t.Fatalf("baseline[%d].Instance = %#v, want %#v", index, entry.Instance, wantInstance)
		}
	}
}

func TestBuildRuntimeStatusBaselinesEmptyAndMutationIsolation(t *testing.T) {
	for name, snapshot := range map[string]Snapshot{
		"nil map":   {},
		"empty map": {RuntimeInstances: map[string]RuntimeInstance{}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := BuildRuntimeStatusBaselines(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || len(got) != 0 {
				t.Fatalf("baseline = %#v, want non-nil empty", got)
			}
		})
	}

	record := runtimeStatusBaselineRecord("runtime.copy", 2)
	snapshot := Snapshot{
		RuntimeInstances: map[string]RuntimeInstance{record.ID: record},
	}
	first, err := BuildRuntimeStatusBaselines(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildRuntimeStatusBaselines(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("fresh calls differ: first=%#v second=%#v", first, second)
	}

	inputRecord := snapshot.RuntimeInstances[record.ID]
	inputRecord.ObservedCapabilities[0] = "mutated-input"
	inputRecord.ModelIDs[0] = "mutated/model"
	if first[0].Instance.ObservedCapabilities[0] != "models" {
		t.Fatalf("input mutation changed baseline: %#v", first[0])
	}
	first[0].Instance.ObservedCapabilities[0] = "mutated-output"
	stored := snapshot.RuntimeInstances[record.ID]
	if stored.ObservedCapabilities[0] != "mutated-input" {
		t.Fatalf("output mutation changed input: %#v", stored)
	}
}

func TestBuildRuntimeStatusBaselinesRejectsInvalidProjection(t *testing.T) {
	valid := runtimeStatusBaselineRecord("runtime.valid", 3)
	validStatus := runtimeStatusBaselineWithStatus(
		valid,
		valid.DiscoveryEventID,
		valid.DiscoverySequence,
		"event.status.runtime.valid",
		valid.DiscoverySequence+1,
	)
	cases := []struct {
		name   string
		record RuntimeInstance
		key    string
	}{
		{name: "map key mismatch", record: valid, key: "runtime.other"},
		{name: "empty runtime id", record: withRuntimeStatusBaselineRecord(valid, func(value *RuntimeInstance) {
			value.ID = ""
		})},
		{name: "noncanonical capabilities", record: withRuntimeStatusBaselineRecord(valid, func(value *RuntimeInstance) {
			value.ObservedCapabilities = []string{"version", "models"}
		})},
		{name: "noncanonical models", record: withRuntimeStatusBaselineRecord(valid, func(value *RuntimeInstance) {
			value.ModelIDs = []string{"provider/z", "provider/a"}
		})},
		{name: "invalid discovery digest", record: withRuntimeStatusBaselineRecord(valid, func(value *RuntimeInstance) {
			value.DiscoveryDigest = strings.Repeat("A", 64)
		})},
		{name: "empty discovery source", record: withRuntimeStatusBaselineRecord(valid, func(value *RuntimeInstance) {
			value.SourceProbeID = ""
		})},
		{name: "empty discovery id", record: withRuntimeStatusBaselineRecord(valid, func(value *RuntimeInstance) {
			value.DiscoveryID = ""
		})},
		{name: "zero discovery time", record: withRuntimeStatusBaselineRecord(valid, func(value *RuntimeInstance) {
			value.DiscoveredAt = time.Time{}
		})},
		{name: "non utc discovery time", record: withRuntimeStatusBaselineRecord(valid, func(value *RuntimeInstance) {
			value.DiscoveredAt = time.Date(2026, 7, 25, 12, 0, 0, 0, time.FixedZone("fixture", 3600))
		})},
		{name: "empty discovery event", record: withRuntimeStatusBaselineRecord(valid, func(value *RuntimeInstance) {
			value.DiscoveryEventID = ""
		})},
		{name: "zero discovery sequence", record: withRuntimeStatusBaselineRecord(valid, func(value *RuntimeInstance) {
			value.DiscoverySequence = 0
		})},
		{name: "invalid reconciliation digest", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusReconciliationDigest = strings.Repeat("A", 64)
		})},
		{name: "invalid baseline digest", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusBaselineDigest = "invalid"
		})},
		{name: "invalid status discovery digest", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusDiscoveryDigest = ""
		})},
		{name: "empty status source", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusSourceProbeID = ""
		})},
		{name: "zero status time", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusChangedAt = time.Time{}
		})},
		{name: "non utc status time", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusChangedAt = time.Date(2026, 7, 25, 12, 0, 0, 0, time.FixedZone("fixture", 3600))
		})},
		{name: "empty status event", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusEventID = ""
		})},
		{name: "zero status sequence", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusSequence = 0
		})},
		{name: "empty previous event", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusPreviousEventID = ""
		})},
		{name: "zero previous sequence", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusPreviousSequence = 0
		})},
		{name: "non exact next status sequence", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusSequence++
		})},
		{name: "previous before discovery", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusPreviousSequence = value.DiscoverySequence - 1
			value.StatusSequence = value.StatusPreviousSequence + 1
			value.StatusPreviousEventID = "event.status.before-discovery"
		})},
		{name: "previous sequence overflow", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusPreviousSequence = math.MaxInt64
			value.StatusSequence = math.MaxInt64
			value.StatusPreviousEventID = "event.status.overflow"
		})},
		{name: "status event equals previous", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusEventID = value.StatusPreviousEventID
		})},
		{name: "status event equals discovery", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusEventID = value.DiscoveryEventID
		})},
		{name: "equal discovery sequence different previous event", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusPreviousEventID = "event.forged"
		})},
		{name: "equal discovery event later previous sequence", record: withRuntimeStatusBaselineRecord(validStatus, func(value *RuntimeInstance) {
			value.StatusPreviousSequence++
			value.StatusSequence++
		})},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			key := test.key
			if key == "" {
				key = test.record.ID
			}
			got, err := BuildRuntimeStatusBaselines(Snapshot{
				RuntimeInstances: map[string]RuntimeInstance{key: test.record},
			})
			if !errors.Is(err, ErrInvalidRuntimeStatusBaselineProjection) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidRuntimeStatusBaselineProjection)
			}
			if got != nil {
				t.Fatalf("partial baseline = %#v", got)
			}
		})
	}

	oversized := make(map[string]RuntimeInstance, 33)
	for index := 0; index < 33; index++ {
		id := "runtime." + string(rune('a'+index))
		oversized[id] = runtimeStatusBaselineRecord(id, int64(index+1))
	}
	got, err := BuildRuntimeStatusBaselines(Snapshot{RuntimeInstances: oversized})
	if !errors.Is(err, ErrInvalidRuntimeStatusBaselineProjection) || got != nil {
		t.Fatalf("oversized result = %#v, %v", got, err)
	}
}

func TestBuildRuntimeStatusBaselinesRejectsEveryPartialStatusGroup(t *testing.T) {
	valid := runtimeStatusBaselineWithStatus(
		runtimeStatusBaselineRecord("runtime.partial", 2),
		"event.discovery.runtime.partial",
		2,
		"event.status.runtime.partial",
		3,
	)
	clear := []func(*RuntimeInstance){
		func(value *RuntimeInstance) { value.StatusReconciliationID = "" },
		func(value *RuntimeInstance) { value.StatusReconciliationDigest = "" },
		func(value *RuntimeInstance) { value.StatusBaselineDigest = "" },
		func(value *RuntimeInstance) { value.StatusDiscoveryDigest = "" },
		func(value *RuntimeInstance) { value.StatusSourceProbeID = "" },
		func(value *RuntimeInstance) { value.StatusChangedAt = time.Time{} },
		func(value *RuntimeInstance) { value.StatusEventID = "" },
		func(value *RuntimeInstance) { value.StatusSequence = 0 },
		func(value *RuntimeInstance) { value.StatusPreviousEventID = "" },
		func(value *RuntimeInstance) { value.StatusPreviousSequence = 0 },
	}
	for index, mutate := range clear {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			record := withRuntimeStatusBaselineRecord(valid, mutate)
			got, err := BuildRuntimeStatusBaselines(Snapshot{
				RuntimeInstances: map[string]RuntimeInstance{record.ID: record},
			})
			if !errors.Is(err, ErrInvalidRuntimeStatusBaselineProjection) || got != nil {
				t.Fatalf("partial group %d result = %#v, %v", index, got, err)
			}
		})
	}
}

func TestBuildRuntimeStatusBaselinesRejectsCrossRuntimeEventIDReuse(t *testing.T) {
	roles := []string{"discovery", "status", "previous"}
	for _, sourceRole := range roles {
		for _, targetRole := range roles {
			t.Run(sourceRole+"_to_"+targetRole, func(t *testing.T) {
				left := runtimeStatusBaselineWithStatus(
					runtimeStatusBaselineRecord("runtime.left", 1),
					"event.status.previous.runtime.left",
					2,
					"event.status.current.runtime.left",
					3,
				)
				right := runtimeStatusBaselineWithStatus(
					runtimeStatusBaselineRecord("runtime.right", 10),
					"event.status.previous.runtime.right",
					11,
					"event.status.current.runtime.right",
					12,
				)
				runtimeStatusBaselineSetEventID(
					&right,
					targetRole,
					runtimeStatusBaselineEventID(left, sourceRole),
				)
				got, err := BuildRuntimeStatusBaselines(Snapshot{
					RuntimeInstances: map[string]RuntimeInstance{
						left.ID:  left,
						right.ID: right,
					},
				})
				if !errors.Is(err, ErrInvalidRuntimeStatusBaselineProjection) {
					t.Fatalf("error = %v, want %v", err, ErrInvalidRuntimeStatusBaselineProjection)
				}
				if got != nil {
					t.Fatalf("partial baseline = %#v", got)
				}
			})
		}
	}
}

func TestBuildRuntimeStatusBaselinesPreservesPureBoundary(t *testing.T) {
	const productPath = "runtime_status_baseline.go"
	source, err := os.ReadFile(productPath)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), productPath, source, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"\"errors\"":                           true,
		"\"math\"":                             true,
		"\"reflect\"":                          true,
		"\"sort\"":                             true,
		"\"time\"":                             true,
		"\"loom-pi-rebuild/internal/runtime\"": true,
	}
	for _, spec := range parsed.Imports {
		if !allowed[spec.Path.Value] {
			t.Fatalf("forbidden import %s", spec.Path.Value)
		}
	}
	product := string(source)
	for _, forbidden := range []string{
		"journal.",
		"state.",
		"DiscoverRuntime",
		"ReconcileObservedRuntimeStatuses",
		"CommitRuntime",
		"sql.",
		"os.",
		"exec.",
		"http.",
		"net.",
		"goroutine",
		"scheduler",
		"daemon",
		"credential",
		"RuntimeProfile",
		"AgentGrant",
	} {
		if strings.Contains(product, forbidden) {
			t.Fatalf("product contains forbidden surface %q", forbidden)
		}
	}
}

func runtimeStatusBaselineRecord(id string, sequence int64) RuntimeInstance {
	return RuntimeInstance{
		ID:                   id,
		DeviceID:             "device." + id,
		AdapterType:          "pi-cli",
		DisplayName:          "Runtime " + id,
		ExecutableVersion:    "1.0.0",
		Status:               string(loomruntime.RuntimeOnline),
		ObservedCapabilities: []string{"models", "version"},
		Capacity:             1,
		ModelIDs:             []string{"provider/model-a", "provider/model-b"},
		DiscoveryDigest:      strings.Repeat("a", 64),
		SourceProbeID:        "probe.runtime-status-baseline",
		DiscoveryID:          "discovery." + id,
		DiscoveredAt:         time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC),
		DiscoveryEventID:     "event.discovery." + id,
		DiscoverySequence:    sequence,
	}
}

func runtimeStatusBaselineWithStatus(
	record RuntimeInstance,
	previousEventID string,
	previousSequence int64,
	eventID string,
	sequence int64,
) RuntimeInstance {
	record.Status = string(loomruntime.RuntimeOffline)
	record.StatusReconciliationID = "reconciliation." + record.ID
	record.StatusReconciliationDigest = strings.Repeat("b", 64)
	record.StatusBaselineDigest = strings.Repeat("c", 64)
	record.StatusDiscoveryDigest = strings.Repeat("d", 64)
	record.StatusSourceProbeID = "probe.status." + record.ID
	record.StatusChangedAt = time.Date(2026, 7, 25, 13, 0, 0, 0, time.UTC)
	record.StatusEventID = eventID
	record.StatusSequence = sequence
	record.StatusPreviousEventID = previousEventID
	record.StatusPreviousSequence = previousSequence
	return record
}

func withRuntimeStatusBaselineRecord(
	record RuntimeInstance,
	change func(*RuntimeInstance),
) RuntimeInstance {
	change(&record)
	return record
}

func runtimeStatusBaselineEventID(record RuntimeInstance, role string) string {
	switch role {
	case "discovery":
		return record.DiscoveryEventID
	case "status":
		return record.StatusEventID
	case "previous":
		return record.StatusPreviousEventID
	default:
		panic("unknown Runtime status baseline Event role")
	}
}

func runtimeStatusBaselineSetEventID(
	record *RuntimeInstance,
	role string,
	eventID string,
) {
	switch role {
	case "discovery":
		record.DiscoveryEventID = eventID
	case "status":
		record.StatusEventID = eventID
	case "previous":
		record.StatusPreviousEventID = eventID
	default:
		panic("unknown Runtime status baseline Event role")
	}
}
