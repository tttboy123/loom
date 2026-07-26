package api

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"

	_ "modernc.org/sqlite"
)

type apiTestViewSource struct {
	view       projection.GlobalReadView
	rebuildErr error
	rebuilds   int
}

func (source *apiTestViewSource) Rebuild(context.Context) error {
	source.rebuilds++
	return source.rebuildErr
}

func (source *apiTestViewSource) GlobalReadView() projection.GlobalReadView {
	return source.view
}

func TestNewTeamExecutionStreamRejectsIncompleteConfiguration(t *testing.T) {
	now := func() time.Time {
		return time.Date(2026, 7, 26, 14, 0, 0, 0, time.UTC)
	}
	tests := []TeamExecutionStreamConfig{
		{},
		{TeamInstanceID: "team-1"},
		{TeamInstanceID: "team-1", Journal: &journal.Store{}},
		{
			TeamInstanceID: "team-1",
			Journal:        &journal.Store{},
			Projection:     &apiTestViewSource{},
		},
		{
			TeamInstanceID: "bad\nteam",
			Journal:        &journal.Store{},
			Projection:     &apiTestViewSource{},
			Now:            now,
		},
	}
	for index, config := range tests {
		if _, err := NewTeamExecutionStream(config); !errors.Is(err, ErrInvalidTimelineRequest) {
			t.Fatalf("case %d error = %v, want ErrInvalidTimelineRequest", index, err)
		}
	}
}

func TestTimelineCursorCanonicalRoundTripAndTeamBinding(t *testing.T) {
	viewVersion := strings.Repeat("a", 64)
	cursor := timelineCursor{
		SchemaVersion:  1,
		TeamInstanceID: "team-1",
		ViewVersion:    viewVersion,
		Heads: []journal.StreamHead{
			{StreamID: "run/run-1", Sequence: 2, EventID: "run-event-2"},
			{StreamID: "team-execution/team-1"},
		},
	}
	cursor.ScopeDigest = timelineScopeDigest(cursor.Heads)
	encoded, err := encodeTimelineCursor(cursor)
	if err != nil {
		t.Fatalf("encodeTimelineCursor() error = %v", err)
	}
	decoded, err := decodeTimelineCursor(encoded, "team-1")
	if err != nil {
		t.Fatalf("decodeTimelineCursor() error = %v", err)
	}
	if decoded.TeamInstanceID != cursor.TeamInstanceID ||
		decoded.ScopeDigest != cursor.ScopeDigest ||
		decoded.ViewVersion != viewVersion ||
		len(decoded.Heads) != 2 ||
		decoded.Heads[0].StreamID != "run/run-1" {
		t.Fatalf("decoded cursor = %#v", decoded)
	}

	if _, err := decodeTimelineCursor(encoded, "team-2"); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("wrong-Team cursor error = %v", err)
	}
	padded := encoded + "="
	if _, err := decodeTimelineCursor(padded, "team-1"); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("padded cursor error = %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	noncanonical := base64.RawURLEncoding.EncodeToString(append(raw, ' '))
	if _, err := decodeTimelineCursor(noncanonical, "team-1"); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("noncanonical JSON cursor error = %v", err)
	}
}

func TestTimelineCursorRejectsNoncanonicalShapesAndEncodedOverflow(t *testing.T) {
	viewVersion := strings.Repeat("a", 64)
	baseHeads := []journal.StreamHead{
		{StreamID: "run/run-1", Sequence: 2, EventID: "run-event-2"},
		{StreamID: "team-execution/team-1"},
		{StreamID: "work-item/work-1", Sequence: 1, EventID: "work-event-1"},
	}
	var canonical string
	for iteration := 0; iteration < 50; iteration++ {
		permuted := append([]journal.StreamHead(nil), baseHeads...)
		offset := iteration % len(permuted)
		permuted = append(permuted[offset:], permuted[:offset]...)
		if iteration%2 == 1 {
			for left, right := 0, len(permuted)-1; left < right; left, right = left+1, right-1 {
				permuted[left], permuted[right] = permuted[right], permuted[left]
			}
		}
		cursor := timelineCursor{
			SchemaVersion:  1,
			TeamInstanceID: "team-1",
			ViewVersion:    viewVersion,
			Heads:          permuted,
		}
		cursor.ScopeDigest = timelineScopeDigest(
			append([]journal.StreamHead(nil), baseHeads...),
		)
		encoded, err := encodeTimelineCursor(cursor)
		if err != nil {
			t.Fatalf("permutation %d encode error = %v", iteration, err)
		}
		if canonical == "" {
			canonical = encoded
		} else if encoded != canonical {
			t.Fatalf("permutation %d cursor is noncanonical", iteration)
		}
	}

	wire := timelineCursorWire{
		SchemaVersion:  1,
		TeamInstanceID: "team-1",
		ViewVersion:    viewVersion,
		Heads: []cursorHeadWire{
			{StreamID: "work-item/work-1", Sequence: 1, EventID: "work-event-1"},
			{StreamID: "run/run-1", Sequence: 2, EventID: "run-event-2"},
		},
	}
	wire.ScopeDigest = timelineScopeDigest([]journal.StreamHead{
		{StreamID: "run/run-1", Sequence: 2, EventID: "run-event-2"},
		{StreamID: "work-item/work-1", Sequence: 1, EventID: "work-event-1"},
	})
	encodeWire := func(value any) string {
		data, err := marshalCompact(value)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(data)
	}
	if _, err := decodeTimelineCursor(
		encodeWire(wire),
		"team-1",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("unsorted heads error = %v", err)
	}

	duplicate := wire
	duplicate.Heads = []cursorHeadWire{
		{StreamID: "run/run-1"},
		{StreamID: "run/run-1"},
	}
	duplicate.ScopeDigest = strings.Repeat("b", 64)
	if _, err := decodeTimelineCursor(
		encodeWire(duplicate),
		"team-1",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("duplicate heads error = %v", err)
	}

	tooMany := wire
	tooMany.Heads = make([]cursorHeadWire, journal.MaxCursorStreams+1)
	headValues := make([]journal.StreamHead, len(tooMany.Heads))
	for index := range tooMany.Heads {
		streamID := fmt.Sprintf("stream/%03d", index)
		tooMany.Heads[index] = cursorHeadWire{StreamID: streamID}
		headValues[index] = journal.StreamHead{StreamID: streamID}
	}
	tooMany.ScopeDigest = timelineScopeDigest(headValues)
	if _, err := decodeTimelineCursor(
		encodeWire(tooMany),
		"team-1",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("97-head cursor error = %v", err)
	}

	canonicalJSON, err := base64.RawURLEncoding.DecodeString(canonical)
	if err != nil {
		t.Fatal(err)
	}
	unknown := strings.TrimSuffix(string(canonicalJSON), "}") +
		`,"unknown":true}`
	if _, err := decodeTimelineCursor(
		base64.RawURLEncoding.EncodeToString([]byte(unknown)),
		"team-1",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("unknown field error = %v", err)
	}
	duplicateField := strings.Replace(
		string(canonicalJSON),
		`"schema_version":1`,
		`"schema_version":1,"schema_version":1`,
		1,
	)
	if _, err := decodeTimelineCursor(
		base64.RawURLEncoding.EncodeToString([]byte(duplicateField)),
		"team-1",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("duplicate field error = %v", err)
	}

	longHeads := make([]journal.StreamHead, journal.MaxCursorStreams)
	for index := range longHeads {
		longHeads[index] = journal.StreamHead{
			StreamID: fmt.Sprintf(
				"%03d-%s",
				index,
				strings.Repeat("x", 508),
			),
		}
	}
	overflow := timelineCursor{
		SchemaVersion:  1,
		TeamInstanceID: "team-1",
		ViewVersion:    viewVersion,
		Heads:          longHeads,
	}
	overflow.ScopeDigest = timelineScopeDigest(longHeads)
	if _, err := encodeTimelineCursor(overflow); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("encoded overflow error = %v", err)
	}
}

func TestTimelineGapErrorPreservesOnlySafeTypedCausesAndCopiedPage(t *testing.T) {
	occurredAt := time.Date(2026, 7, 26, 14, 1, 0, 0, time.UTC)
	gap, err := newStreamGap(streamGapInput{
		TeamInstanceID:       "team-1",
		Reason:               "cursor_conflict",
		PreviousCursorDigest: strings.Repeat("b", 64),
		CurrentViewVersion:   strings.Repeat("c", 64),
		ArtifactDigest:       strings.Repeat("d", 64),
		OccurredAt:           occurredAt,
	})
	if err != nil {
		t.Fatalf("newStreamGap() error = %v", err)
	}
	page := newTimelineGapPage("team-1", gap)
	gapErr := newTimelineGapError(page, ErrTimelineCursorConflict)
	if !errors.Is(gapErr, ErrStreamGap) ||
		!errors.Is(gapErr, ErrTimelineCursorConflict) {
		t.Fatalf("gap error causes = %v", gapErr)
	}
	if strings.Contains(gapErr.Error(), "team-1") ||
		strings.Contains(gapErr.Error(), strings.Repeat("d", 64)) {
		t.Fatalf("gap error leaks safe payload details: %q", gapErr.Error())
	}
	first := gapErr.Page()
	firstGap, ok := first.Gap()
	if !ok {
		t.Fatal("gap page has no gap")
	}
	data, err := json.Marshal(firstGap)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":1,"delivery_id":"` +
		firstGap.DeliveryID() +
		`","kind":"stream_gap","team_instance_id":"team-1","reason":"cursor_conflict","previous_cursor_digest":"` +
		strings.Repeat("b", 64) +
		`","current_view_version":"` + strings.Repeat("c", 64) +
		`","artifact_available":true,"artifact_digest":"` +
		strings.Repeat("d", 64) +
		`","recoverable":true,"occurred_at":"2026-07-26T14:01:00Z"}`
	if string(data) != want {
		t.Fatalf("gap JSON = %s, want %s", data, want)
	}
}

func TestReadPageRejectsMalformedCursorAsRecoverableGapWithoutJournalFallback(t *testing.T) {
	source := &apiTestViewSource{}
	stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: "team-1",
		Journal:        &journal.Store{},
		Projection:     source,
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 14, 2, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewTeamExecutionStream() error = %v", err)
	}
	page, err := stream.ReadPage(context.Background(), "not-base64!", 128)
	if !errors.Is(err, ErrStreamGap) ||
		!errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("ReadPage() error = %v", err)
	}
	gap, ok := page.Gap()
	if !ok || gap.Reason() != "invalid_cursor" || !gap.Recoverable() {
		t.Fatalf("gap = %#v, %v", gap, ok)
	}
	attention := page.Attention()
	if len(attention) != 1 ||
		attention[0].Kind != "stream_gap" ||
		attention[0].ActionRequired != "reconnect" {
		t.Fatalf("gap Attention = %#v", attention)
	}
	if source.rebuilds > 1 {
		t.Fatalf("malformed cursor rebuilds = %d, want at most 1", source.rebuilds)
	}
}

func TestReadPageReturnsJournalAuthoritativeTimelineAndReconnectsWithoutDuplicate(t *testing.T) {
	ctx := context.Background()
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: "team-instance.one",
		Journal:        store,
		Projection:     readModel,
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 15, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := stream.ReadPage(ctx, "", 1)
	if err != nil {
		t.Fatalf("first ReadPage() error = %v", err)
	}
	if first.TeamInstanceID() != "team-instance.one" ||
		first.ViewVersion() == "" ||
		first.NextCursor() == "" ||
		first.HasMore() {
		t.Fatalf("first page = %#v", first)
	}
	records := first.Records()
	if len(records) != 1 ||
		records[0].kind != "team_planned" ||
		records[0].authority != "journal" ||
		records[0].sourceStreamID != "team-execution/team-instance.one" ||
		records[0].sourceSequence != 1 {
		t.Fatalf("first records = %#v", records)
	}
	data, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "source_record_set_digest") ||
		strings.Contains(string(data), "team_definition_digest") {
		t.Fatalf("timeline leaked raw Team payload: %s", data)
	}

	reconnected, err := stream.ReadPage(ctx, first.NextCursor(), 1)
	if err != nil {
		t.Fatalf("reconnected ReadPage() error = %v", err)
	}
	if len(reconnected.Records()) != 0 ||
		reconnected.NextCursor() == "" ||
		reconnected.HasMore() {
		t.Fatalf("reconnected page = %#v", reconnected)
	}

	for iteration := 0; iteration < 50; iteration++ {
		start, err := stream.ReadPage(ctx, "", 1)
		if err != nil {
			t.Fatalf("walk %d initial page error = %v", iteration, err)
		}
		seen := make(map[string]struct{})
		for _, record := range start.Records() {
			if _, duplicate := seen[record.deliveryID]; duplicate {
				t.Fatalf("walk %d duplicate delivery %s", iteration, record.deliveryID)
			}
			seen[record.deliveryID] = struct{}{}
		}
		resumed, err := stream.ReadPage(ctx, start.NextCursor(), 1)
		if err != nil {
			t.Fatalf("walk %d resumed page error = %v", iteration, err)
		}
		for _, record := range resumed.Records() {
			if _, duplicate := seen[record.deliveryID]; duplicate {
				t.Fatalf("walk %d duplicate delivery %s", iteration, record.deliveryID)
			}
			seen[record.deliveryID] = struct{}{}
		}
		if len(seen) != 1 {
			t.Fatalf("walk %d delivery count = %d, want 1", iteration, len(seen))
		}
	}
}

func TestReadPageConcurrentAppendAdvancesOnlyOnCurrentOrNextCursor(t *testing.T) {
	ctx := context.Background()
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: "team-instance.one",
		Journal:        store,
		Projection:     readModel,
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 15, 30, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := stream.ReadPage(ctx, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	startingCursor, err := decodeTimelineCursor(
		initial.NextCursor(),
		"team-instance.one",
	)
	if err != nil {
		t.Fatal(err)
	}
	startingCursor.Heads = append(
		startingCursor.Heads,
		journal.StreamHead{StreamID: "safe-extra"},
	)
	startingCursor.Heads, err = normalizeTimelineHeads(startingCursor.Heads)
	if err != nil {
		t.Fatal(err)
	}
	startingCursor.ScopeDigest = timelineScopeDigest(startingCursor.Heads)
	startingEncoded, err := encodeTimelineCursor(startingCursor)
	if err != nil {
		t.Fatal(err)
	}
	appended := journal.Event{
		ID:             "event.team.safe-unknown",
		StreamID:       "safe-extra",
		Seq:            1,
		IdempotencyKey: "key.team.safe-unknown",
		Type:           "SafeVocabularyUnknown",
		SchemaVersion:  1,
		EmittedAt:      time.Date(2026, 7, 26, 15, 30, 1, 0, time.UTC),
		PayloadJSON:    []byte(`{"safe":"value"}`),
	}
	start := make(chan struct{})
	appendResult := make(chan error, 1)
	go func() {
		<-start
		_, err := store.Append(ctx, appended)
		appendResult <- err
	}()
	close(start)
	current, err := stream.ReadPage(ctx, startingEncoded, 128)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-appendResult; err != nil {
		t.Fatal(err)
	}
	cursor := current.NextCursor()
	decoded, err := decodeTimelineCursor(cursor, "team-instance.one")
	if err != nil {
		t.Fatal(err)
	}
	headSequence := func(value timelineCursor) int64 {
		for _, head := range value.Heads {
			if head.StreamID == appended.StreamID {
				if head.EventID != "" && head.EventID != appended.ID &&
					head.Sequence == appended.Seq {
					t.Fatalf("fabricated event ID at sequence %d", head.Sequence)
				}
				return head.Sequence
			}
		}
		return -1
	}
	sequence := headSequence(decoded)
	if sequence == 0 {
		next, err := stream.ReadPage(ctx, cursor, 128)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err = decodeTimelineCursor(
			next.NextCursor(),
			"team-instance.one",
		)
		if err != nil {
			t.Fatal(err)
		}
		sequence = headSequence(decoded)
	}
	if sequence != 1 || len(current.Records()) != 0 {
		t.Fatalf(
			"concurrent append sequence = %d current records = %#v",
			sequence,
			current.Records(),
		)
	}
}

func TestSubscriptionCoalescesTentativeTextAndSurfacesOverflowGapFirst(t *testing.T) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	teamID := "team-instance.one"
	stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: teamID,
		Journal:        store,
		Projection:     projection.New(db),
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 16, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	subscription, err := stream.Subscribe(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Close()
	makeRecord := func(node, text string, sequence int64) DeliveryRecord {
		return DeliveryRecord{
			schemaVersion:   1,
			deliveryID:      strings.Repeat("a", 63) + strconv.FormatInt(sequence%10, 10),
			kind:            "node_output_delta",
			authority:       "tentative",
			teamInstanceID:  teamID,
			logicalNodeID:   node,
			attemptNumber:   1,
			sourceSequence:  sequence,
			sourceEventID:   "frame-" + strconv.FormatInt(sequence, 10),
			occurredAt:      time.Date(2026, 7, 26, 16, 0, int(sequence%60), 0, time.UTC),
			payload:         DeliveryPayload{textDelta: text},
			runID:           "run-1",
			claimGeneration: 1,
		}
	}
	subscription.enqueue(makeRecord("main", "hello ", 1))
	subscription.enqueue(makeRecord("main", "world", 2))
	item, err := subscription.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	delivery, ok := item.Delivery()
	if !ok ||
		delivery.payload.textDelta != "hello world" ||
		delivery.sourceSequence != 2 {
		t.Fatalf("coalesced delivery = %#v, %v", delivery, ok)
	}

	lineage, err := stream.Subscribe(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer lineage.Close()
	lineage.enqueue(makeRecord("main", "generation-1", 3))
	nextGeneration := makeRecord("main", "generation-2", 4)
	nextGeneration.claimGeneration = 2
	lineage.enqueue(nextGeneration)
	firstLineage, err := lineage.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	secondLineage, err := lineage.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	firstDelivery, firstOK := firstLineage.Delivery()
	secondDelivery, secondOK := secondLineage.Delivery()
	if !firstOK || !secondOK ||
		firstDelivery.payload.textDelta != "generation-1" ||
		secondDelivery.payload.textDelta != "generation-2" {
		t.Fatalf(
			"cross-generation deliveries = %#v %#v",
			firstDelivery,
			secondDelivery,
		)
	}

	for iteration := 0; iteration < 100; iteration++ {
		overflow, err := stream.Subscribe(context.Background(), "")
		if err != nil {
			t.Fatal(err)
		}
		var producers sync.WaitGroup
		for producer := 0; producer < 4; producer++ {
			producers.Add(1)
			go func(producer int) {
				defer producers.Done()
				for index := 0; index < 17; index++ {
					sequence := int64(producer*17 + index + 1)
					overflow.enqueue(makeRecord(
						fmt.Sprintf("node-%d-%02d", producer, index),
						"x",
						sequence,
					))
				}
			}(producer)
		}
		producers.Wait()
		first, err := overflow.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		gap, ok := first.Gap()
		if !ok || gap.Reason() != "tentative_overflow" {
			t.Fatalf(
				"iteration %d overflow first item = %#v gap=%#v, %v",
				iteration,
				first,
				gap,
				ok,
			)
		}
		if err := overflow.Close(); err != nil {
			t.Fatal(err)
		}
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := overflow.Next(cancelled); !errors.Is(err, context.Canceled) {
			t.Fatalf("iteration %d closed Next() error = %v", iteration, err)
		}
	}
	authoritative, err := stream.ReadPage(context.Background(), "", 128)
	if err != nil {
		t.Fatal(err)
	}
	records := authoritative.Records()
	if len(records) != 1 ||
		records[0].kind != "team_planned" ||
		records[0].authority != "journal" {
		t.Fatalf("authoritative recovery after overflow = %#v", records)
	}
}

func TestSubscribeRequiresTeamValidatesPositionAndBoundsSubscribers(t *testing.T) {
	ctx := context.Background()
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: "team-instance.one",
		Journal:        store,
		Projection:     readModel,
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 16, 30, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := stream.ReadPage(ctx, "", 128)
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := decodeTimelineCursor(
		page.NextCursor(),
		"team-instance.one",
	)
	if err != nil {
		t.Fatal(err)
	}
	for index := range cursor.Heads {
		if cursor.Heads[index].Sequence > 0 {
			cursor.Heads[index].EventID = "conflicting-event"
			break
		}
	}
	cursor.ScopeDigest = timelineScopeDigest(cursor.Heads)
	conflicting, err := encodeTimelineCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Subscribe(
		ctx,
		conflicting,
	); !errors.Is(err, ErrTimelineCursorConflict) {
		t.Fatalf("conflicting cursor error = %v", err)
	}
	if _, err := stream.Subscribe(
		ctx,
		"not-base64!",
	); !errors.Is(err, ErrInvalidTimelineCursor) {
		t.Fatalf("invalid cursor error = %v", err)
	}

	subscriptions := make([]*Subscription, 0, maxSubscribers)
	for index := 0; index < maxSubscribers; index++ {
		subscription, err := stream.Subscribe(ctx, "")
		if err != nil {
			t.Fatalf("subscription %d error = %v", index, err)
		}
		subscriptions = append(subscriptions, subscription)
	}
	if _, err := stream.Subscribe(
		ctx,
		"",
	); !errors.Is(err, ErrTooManySubscribers) {
		t.Fatalf("ninth subscription error = %v", err)
	}
	if err := subscriptions[0].Close(); err != nil {
		t.Fatal(err)
	}
	replacement, err := stream.Subscribe(ctx, "")
	if err != nil {
		t.Fatalf("replacement subscription error = %v", err)
	}
	subscriptions[0] = replacement
	for _, subscription := range subscriptions {
		if err := subscription.Close(); err != nil {
			t.Fatal(err)
		}
	}

	missing, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: "team-missing",
		Journal:        store,
		Projection:     readModel,
		Now: func() time.Time {
			return time.Date(2026, 7, 26, 16, 30, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missing.Subscribe(
		ctx,
		"",
	); !errors.Is(err, ErrTeamTimelineNotFound) {
		t.Fatalf("missing Team subscription error = %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := stream.Subscribe(
		cancelled,
		"",
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled subscription error = %v", err)
	}
}

func TestTentativeDeltaRequiresExactBoundedSafeJSON(t *testing.T) {
	valid, err := decodeTentativeDelta([]byte(`{"delta":"line one\nline two\tok"}`))
	if err != nil || valid != "line one\nline two\tok" {
		t.Fatalf("valid delta = %q, %v", valid, err)
	}
	tests := [][]byte{
		[]byte(`{}`),
		[]byte(`{"delta":""}`),
		[]byte(`{"delta":1}`),
		[]byte(`{"delta":"ok","extra":true}`),
		[]byte(`{"delta":"one","delta":"two"}`),
		[]byte(`{"delta":"bad\u0000control"}`),
		[]byte(`{"delta":"` + strings.Repeat("x", maxTentativeDelta+1) + `"}`),
		{0xff, 0xfe},
	}
	for index, payload := range tests {
		if _, err := decodeTentativeDelta(
			payload,
		); !errors.Is(err, ErrInvalidNodeOutput) {
			t.Fatalf("case %d error = %v", index, err)
		}
	}
}

func TestAuthoritativeSafeFieldsRejectAmbiguityAndIgnoreRawKeys(t *testing.T) {
	fields, err := safeEventFields([]byte(
		`{"logical_node_id":"main","attempt_number":1,` +
			`"status":"running","retry_at":"2026-07-26T16:30:00Z",` +
			`"evidence_digest":"` + strings.Repeat("a", 64) + `",` +
			`"raw_grant":"never-publish"}`,
	))
	if err != nil ||
		fields["logical_node_id"] != "main" ||
		fields["attempt_number"] != "1" ||
		fields["status"] != "running" ||
		fields["retry_at"] != "2026-07-26T16:30:00Z" ||
		fields["evidence_digest"] != strings.Repeat("a", 64) {
		t.Fatalf("safe fields = %#v, %v", fields, err)
	}
	if _, exists := fields["raw_grant"]; exists {
		t.Fatalf("raw key escaped safe mapping: %#v", fields)
	}
	for index, payload := range [][]byte{
		[]byte(`{"status":"one","status":"two"}`),
		[]byte(`{"attempt_number":"1"}`),
		[]byte(`{"status":{"nested":true}}`),
		[]byte(`{"reason":"bad\u0000control"}`),
		[]byte(`{"retry_at":"tomorrow"}`),
		[]byte(`{"retry_at":"2026-07-26T16:30:00+08:00"}`),
		[]byte(`{"evidence_digest":"not-a-digest"}`),
		[]byte(`{"source_evidence_digest":"` + strings.Repeat("A", 64) + `"}`),
	} {
		if _, err := safeEventFields(
			payload,
		); !errors.Is(err, ErrInvalidDeliveryRecord) {
			t.Fatalf("case %d error = %v", index, err)
		}
	}
}

func TestAuthoritativeKindVocabularyIsExact(t *testing.T) {
	want := map[string]string{
		"TeamExecutionPlanned":          "team_planned",
		"TeamNodeAttemptScheduled":      "node_scheduled",
		"TeamReadySetDispatched":        "ready_set_dispatched",
		"TeamNodeAttemptRebound":        "node_rebound",
		"RunStarted":                    "run_started",
		"RunTerminalCommitted":          "run_terminal",
		"WorkItemApprovalPaused":        "approval_required",
		"ApprovalRequested":             "approval_requested",
		"ApprovalDecided":               "approval_decided",
		"ApprovalExpired":               "approval_expired",
		"WorkItemApprovalResolved":      "approval_resolved",
		"TeamNodeAttemptTerminal":       "node_attempt_terminal",
		"WorkItemReadyForReview":        "ready_for_review",
		"WorkItemVerificationCommitted": "verification_recorded",
		"WorkItemDone":                  "work_item_done",
		"WorkItemRejected":              "verification_rejected",
		"TeamNodeAcceptanceCommitted":   "node_acceptance",
		"TeamNodeRecoveryRecorded":      "node_recovery",
		"EvidenceSubmitted":             "evidence_available",
		"TeamExecutionTerminal":         "team_terminal",
	}
	if !reflect.DeepEqual(authoritativeKinds, want) {
		t.Fatalf("authoritativeKinds = %#v, want %#v", authoritativeKinds, want)
	}
}

func openAPITimelineDB(t *testing.T) *sql.DB {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s?%s",
		filepath.Join(t.TempDir(), "timeline.db"),
		values.Encode(),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func appendAPITimelineFixture(t *testing.T, store *journal.Store) {
	t.Helper()
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	teamPayload := map[string]any{
		"team": map[string]any{
			"id":                      "team-instance.one",
			"work_request_id":         "request.one",
			"source_kind":             "saved_team",
			"team_definition_id":      "team.delivery",
			"team_definition_version": 1,
			"team_definition_scope":   "project",
			"scope_identity": map[string]any{
				"project_id":    "project.one",
				"generation_id": "",
			},
			"team_definition_digest": digestA,
			"source_plan_digest":     digestB,
			"state":                  "created",
			"created_at":             int64(1_721_865_600),
		},
		"dormant_sub_agents":       []any{},
		"source_plan_digest":       digestB,
		"source_record_set_digest": digestA,
		"team_instance_count":      1,
		"agent_instance_count":     1,
		"active_sub_agent_count":   0,
		"work_item_count":          0,
	}
	planPayload := map[string]any{
		"team_instance_id": "team-instance.one",
		"plan_digest":      digestA,
		"view_version":     digestB,
		"nodes": []map[string]any{{
			"logical_node_id":     "main",
			"title":               "Main",
			"agent_instance_id":   "agent-instance.main",
			"runtime_instance_id": "runtime.shared",
			"role":                "main",
			"depends_on":          []string{},
			"max_attempts":        2,
		}},
	}
	agentPayload := map[string]any{
		"main_agent": map[string]any{
			"id":                       "agent-instance.main",
			"team_instance_id":         "team-instance.one",
			"agent_definition_id":      "agent.main",
			"agent_definition_version": 1,
			"agent_definition_scope":   "project",
			"scope_identity": map[string]any{
				"project_id":    "project.one",
				"generation_id": "",
			},
			"runtime_profile_id":  "profile.main",
			"runtime_instance_id": "runtime.shared",
			"is_main":             true,
			"state":               "created",
		},
		"runtime_binding": map[string]any{
			"accepted":    true,
			"profile_id":  "profile.main",
			"instance_id": "runtime.shared",
		},
		"source_plan_digest":       digestB,
		"source_record_set_digest": digestA,
		"team_created_at":          int64(1_721_865_600),
		"binding_digest":           digestB,
		"runtime_discovery_digest": digestA,
	}
	encode := func(value any) []byte {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	events := []journal.Event{
		{
			ID:             "event.agent.created",
			StreamID:       "agent_instance:agent-instance.main",
			Seq:            1,
			IdempotencyKey: "key.agent.created",
			Type:           "AgentInstanceCreated",
			SchemaVersion:  1,
			EmittedAt:      time.Date(2026, 7, 25, 2, 30, 0, 0, time.UTC),
			CorrelationID:  "request.one",
			CausationID:    "event.team.created",
			PayloadJSON:    encode(agentPayload),
		},
		{
			ID:             "event.team.created",
			StreamID:       "team_instance:team-instance.one",
			Seq:            1,
			IdempotencyKey: "key.team.created",
			Type:           "TeamInstanceCreated",
			SchemaVersion:  1,
			EmittedAt:      time.Date(2026, 7, 25, 2, 30, 0, 0, time.UTC),
			CorrelationID:  "request.one",
			PayloadJSON:    encode(teamPayload),
		},
		{
			ID:             "event.team.planned",
			StreamID:       "team-execution/team-instance.one",
			Seq:            1,
			IdempotencyKey: "key.team.planned",
			Type:           "TeamExecutionPlanned",
			SchemaVersion:  1,
			EmittedAt:      time.Date(2026, 7, 26, 1, 2, 3, 0, time.UTC),
			PayloadJSON:    encode(planPayload),
		},
	}
	for _, event := range events {
		if _, err := store.Append(context.Background(), event); err != nil {
			t.Fatalf("Append(%s) error = %v", event.ID, err)
		}
	}
}
