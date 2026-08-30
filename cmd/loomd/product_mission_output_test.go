package main

import (
	"strings"
	"testing"
	"time"
)

func TestDecodeProductMissionAttemptOutputsAdmitsOnlyAuthorizedTextEvents(
	t *testing.T,
) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	outputs, err := decodeProductMissionAttemptOutputs([]byte(
		`{"schema_version":1,"scope":"authorized_output_events","events":[`+
			`{"delta":"first update"},{"delta":"final result"}]}`,
	), 4096, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 2 || outputs[0].Sequence != 1 || outputs[1].Sequence != 2 ||
		outputs[0].Text != "first update" || outputs[1].Text != "final result" ||
		outputs[0].OccurredAt != now || len(outputs[0].ContentDigest) != 64 ||
		!strings.HasPrefix(outputs[0].SourceID, "output-") {
		t.Fatalf("Mission outputs = %#v", outputs)
	}
	for _, invalid := range [][]byte{
		[]byte(`{"schema_version":1,"scope":"provider_response","events":[{"delta":"no"}]}`),
		[]byte(`{"schema_version":1,"scope":"authorized_output_events","events":[{"delta":"ok","provider_body":"no"}]}`),
		[]byte(`{"schema_version":1,"scope":"authorized_output_events","events":[{"delta":""}]}`),
	} {
		if _, err := decodeProductMissionAttemptOutputs(invalid, 4096, now); err == nil {
			t.Fatalf("invalid Mission output admitted: %s", invalid)
		}
	}
}
