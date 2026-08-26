package contextcapsule_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/contextcapsule"
)

type fixtureTokenCounter struct {
	counts map[string]int
}

func (fixtureTokenCounter) ID() string      { return "counter:test" }
func (fixtureTokenCounter) Version() string { return "v1" }

func (counter fixtureTokenCounter) CountTokens(content []byte) (int, error) {
	return counter.counts[string(content)], nil
}

type recordingTokenCounter struct {
	identifier string
	version    string
	counts     map[string]int
	inputs     [][]byte
}

func (counter *recordingTokenCounter) ID() string { return counter.identifier }

func (counter *recordingTokenCounter) Version() string { return counter.version }

func (counter *recordingTokenCounter) CountTokens(content []byte) (int, error) {
	counter.inputs = append(counter.inputs, append([]byte(nil), content...))
	return counter.counts[string(content)], nil
}

type driftingTokenCounter struct {
	identifier string
	version    string
	count      int
}

func (counter *driftingTokenCounter) ID() string { return counter.identifier }

func (counter *driftingTokenCounter) Version() string { return counter.version }

func (counter *driftingTokenCounter) CountTokens([]byte) (int, error) {
	counter.version = "v2"
	return counter.count, nil
}

func TestCapacityAuthorityExactWorkedExampleAndDeterministicContributions(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 120_000)
	inputs := []contextcapsule.ItemInput{
		capacityItem("history", contextcapsule.PriorityHistory, 6, "history body", contextcapsule.SourceModelOutput),
		capacityItem("goal", contextcapsule.PrioritySystem, 4, "goal body", contextcapsule.SourceAuthority),
	}
	authority := exactCapacityAuthority()
	counter := fixtureTokenCounter{counts: map[string]int{"goal body": 4, "history body": 6}}
	first, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, inputs, authority, counter)
	if err != nil {
		t.Fatal(err)
	}
	second, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{inputs[1], inputs[0]}, authority, counter)
	if err != nil {
		t.Fatal(err)
	}
	projection, ok := first.CapacityProjection()
	if !ok || projection.SchemaVersion != contextcapsule.CapacitySchemaVersion ||
		projection.Status != contextcapsule.CapacityExact ||
		projection.ContextWindowTokens != 128_000 || projection.ReservedOutputTokens != 8_192 ||
		projection.AdapterToolOverheadTokens != 1_024 || projection.AdmittedInputBudgetTokens != 118_784 ||
		projection.PolicyInputBudgetTokens != 120_000 || projection.TokenCounterID != "counter:test" ||
		projection.TokenCounterVersion != "v1" || projection.AdmittedContributionTokens != 10 ||
		projection.BudgetOmittedContributionTokens != 0 {
		t.Fatalf("capacity projection = %#v, present=%t", projection, ok)
	}
	want := []contextcapsule.CapacityContribution{
		{Priority: contextcapsule.PrioritySystem, SourceType: contextcapsule.SourceAuthority, AdmittedItemCount: 1, AdmittedTokenCount: 4},
		{Priority: contextcapsule.PriorityHistory, SourceType: contextcapsule.SourceModelOutput, AdmittedItemCount: 1, AdmittedTokenCount: 6},
	}
	if !reflect.DeepEqual(projection.Contributions, want) {
		t.Fatalf("contributions = %#v", projection.Contributions)
	}
	secondProjection, _ := second.CapacityProjection()
	if first.Digest() != second.Digest() || first.DisclosureReceiptDigest() != second.DisclosureReceiptDigest() ||
		!reflect.DeepEqual(projection, secondProjection) {
		t.Fatal("capacity authority changed with input order")
	}
	record := first.AuthorityRecord()
	if record.CapacityProjectionDigest == "" || record.CapacitySchemaVersion != contextcapsule.CapacitySchemaVersion ||
		record.CapacityStatus != contextcapsule.CapacityExact ||
		record.AdmittedInputBudgetTokens != projection.AdmittedInputBudgetTokens ||
		record.AdmittedContributionTokens != projection.AdmittedContributionTokens {
		t.Fatalf("authority record = %#v", record)
	}
}

func TestCapacityAuthorityEstimatedAndUnavailableNeverClaimExactWindow(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 70_000)
	input := capacityItem("goal", contextcapsule.PrioritySystem, 4, "goal body", contextcapsule.SourceAuthority)
	counter := fixtureTokenCounter{counts: map[string]int{"goal body": 4}}
	estimated := exactCapacityAuthority()
	estimated.Status = contextcapsule.CapacityEstimated
	estimated.ContextWindowTokens = 64_000
	estimated.ReservedOutputTokens = 4_096
	estimated.AdapterToolOverheadTokens = 512
	capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{input}, estimated, counter)
	if err != nil {
		t.Fatal(err)
	}
	projection, _ := capsule.CapacityProjection()
	if projection.Status != contextcapsule.CapacityEstimated || projection.ContextWindowTokens != 64_000 ||
		projection.AdmittedInputBudgetTokens != 59_392 {
		t.Fatalf("estimated projection = %#v", projection)
	}
	unavailable := estimated
	unavailable.Status = contextcapsule.CapacityUnavailable
	unavailable.ContextWindowTokens = 0
	capsule, err = contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{input}, unavailable, counter)
	if err != nil {
		t.Fatal(err)
	}
	projection, _ = capsule.CapacityProjection()
	if projection.Status != contextcapsule.CapacityUnavailable || projection.ContextWindowTokens != 0 ||
		projection.AdmittedInputBudgetTokens != target.TokenBudget {
		t.Fatalf("unavailable projection = %#v", projection)
	}
	unavailable.ContextWindowTokens = 64_000
	if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{input}, unavailable, counter); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("unavailable authority invented a window: %v", err)
	}
	exact := unavailable
	exact.Status = contextcapsule.CapacityExact
	exact.ContextWindowTokens = 0
	if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{input}, exact, counter); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("exact authority accepted no window: %v", err)
	}
}

func TestCapacityAuthorityFailsClosedOnCounterMismatchAndBounds(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 64)
	input := capacityItem("goal", contextcapsule.PrioritySystem, 4, "goal body", contextcapsule.SourceAuthority)
	authority := exactCapacityAuthority()
	if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{input}, authority,
		fixtureTokenCounter{counts: map[string]int{"goal body": 5}}); !errors.Is(err, contextcapsule.ErrTokenCountMismatch) {
		t.Fatalf("counter mismatch error = %v", err)
	}
	if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{input}, authority, nil); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("nil counter error = %v", err)
	}
	wrongCounterAuthority := authority
	wrongCounterAuthority.TokenCounterVersion = "v2"
	if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{input}, wrongCounterAuthority,
		fixtureTokenCounter{counts: map[string]int{"goal body": 4}}); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("counter identity mismatch error = %v", err)
	}
	for name, mutate := range map[string]func(*contextcapsule.CapacityAuthority){
		"schema": func(value *contextcapsule.CapacityAuthority) { value.SchemaVersion++ },
		"window max": func(value *contextcapsule.CapacityAuthority) {
			value.ContextWindowTokens = contextcapsule.MaxCapacityTokens + 1
		},
		"reserved max": func(value *contextcapsule.CapacityAuthority) {
			value.ReservedOutputTokens = contextcapsule.MaxCapacityTokens + 1
		},
		"overhead max": func(value *contextcapsule.CapacityAuthority) {
			value.AdapterToolOverheadTokens = contextcapsule.MaxCapacityTokens + 1
		},
		"no input":   func(value *contextcapsule.CapacityAuthority) { value.ReservedOutputTokens = value.ContextWindowTokens },
		"counter id": func(value *contextcapsule.CapacityAuthority) { value.TokenCounterID = strings.Repeat("x", 513) },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := authority
			mutate(&candidate)
			if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{input}, candidate,
				fixtureTokenCounter{counts: map[string]int{"goal body": 4}}); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
				t.Fatalf("invalid authority error = %v", err)
			}
		})
	}
	if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{input}, authority,
		fixtureTokenCounter{counts: map[string]int{"goal body": contextcapsule.MaxCapacityTokens + 1}}); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("unbounded counter result error = %v", err)
	}
	if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target,
		[]contextcapsule.ItemInput{input},
		authority,
		&driftingTokenCounter{identifier: "counter:test", version: "v1", count: 4},
	); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("counter version drift error = %v", err)
	}
}

func TestCapacityAuthorityRequiredOverflowAndExplicitRetrievableBudgetOmission(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 8)
	authority := exactCapacityAuthority()
	authority.ContextWindowTokens = 11
	authority.ReservedOutputTokens = 2
	authority.AdapterToolOverheadTokens = 1
	inputs := []contextcapsule.ItemInput{
		capacityItem("goal", contextcapsule.PrioritySystem, 4, "goal body", contextcapsule.SourceAuthority),
		capacityItem("history", contextcapsule.PriorityHistory, 5, "history body", contextcapsule.SourceModelOutput),
		capacityItem("filtered", contextcapsule.PriorityHistory, 1, "filtered body", contextcapsule.SourceModelOutput),
		capacityItem("private", contextcapsule.PriorityRetrievable, 1, "private body", contextcapsule.SourceObservation),
	}
	inputs[0].Required = true
	inputs[2].PolicyFiltered = true
	inputs[3].Scope = contextcapsule.ScopeAgentPrivate
	inputs[3].AllowedAgentID = "someone-else"
	counter := fixtureTokenCounter{counts: map[string]int{"goal body": 4, "history body": 5, "filtered body": 1, "private body": 1}}
	capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, inputs, authority, counter)
	if err != nil {
		t.Fatal(err)
	}
	if !capsule.IsRetrievable("history") || capsule.IsRetrievable("filtered") || capsule.IsRetrievable("private") {
		t.Fatalf("retrievability does not match omission authority: %#v", capsule.Omitted())
	}
	omissions := capsule.Omitted()
	if len(omissions) != 3 || omissions[0].ItemID != "filtered" || omissions[1].ItemID != "history" ||
		omissions[1].Reason != contextcapsule.OmissionBudgetExceeded || omissions[2].ItemID != "private" {
		t.Fatalf("omissions = %#v", omissions)
	}
	retrieved, err := capsule.Retrieve(contextcapsule.RetrievalRequest{
		Authority: capsule.AuthorityRecord(), ItemID: omissions[1].ItemID,
		ContentDigest: omissions[1].ContentDigest, RequesterAgentID: target.AgentID, RequesterRoleID: target.RoleID,
	})
	if err != nil || string(retrieved.Content) != "history body" {
		t.Fatalf("retrieved = %#v, err=%v", retrieved, err)
	}
	projection, _ := capsule.CapacityProjection()
	if projection.AdmittedContributionTokens != 4 || projection.BudgetOmittedContributionTokens != 5 {
		t.Fatalf("contribution totals = %#v", projection)
	}
	overflow := []contextcapsule.ItemInput{inputs[0]}
	overflow[0].TokenCount = 9
	if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, overflow, authority,
		fixtureTokenCounter{counts: map[string]int{"goal body": 9}}); !errors.Is(err, contextcapsule.ErrRequiredContextOmitted) {
		t.Fatalf("required overflow error = %v", err)
	}
}

func TestCapacityProjectionBindsCapsuleReceiptAuthorityAndRoundTrip(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 64)
	input := capacityItem("goal", contextcapsule.PrioritySystem, 4, "goal body", contextcapsule.SourceAuthority)
	capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{input}, exactCapacityAuthority(),
		fixtureTokenCounter{counts: map[string]int{"goal body": 4}})
	if err != nil {
		t.Fatal(err)
	}
	body, err := contextcapsule.MarshalCanonicalRoleContextCapsule(capsule)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := contextcapsule.RestoreRoleContextCapsule(capsule.AuthorityRecord(), body)
	if err != nil || restored.AuthorityRecord() != capsule.AuthorityRecord() {
		t.Fatalf("restored authority = %#v, err=%v", restored.AuthorityRecord(), err)
	}
	wantProjection, _ := capsule.CapacityProjection()
	gotProjection, ok := restored.CapacityProjection()
	if !ok || !reflect.DeepEqual(gotProjection, wantProjection) {
		t.Fatalf("restored projection = %#v, present=%t", gotProjection, ok)
	}
	tamperedBody := bytes.Replace(body, []byte(`"context_window_tokens":128000`), []byte(`"context_window_tokens":127999`), 1)
	if bytes.Equal(tamperedBody, body) {
		t.Fatal("capacity fixture was not present in canonical body")
	}
	if _, err := contextcapsule.RestoreRoleContextCapsule(capsule.AuthorityRecord(), tamperedBody); !errors.Is(err, contextcapsule.ErrInvalidCapsule) {
		t.Fatalf("tampered capacity field error = %v", err)
	}
	for name, mutate := range map[string]func(*contextcapsule.AuthorityRecord){
		"projection digest": func(record *contextcapsule.AuthorityRecord) {
			record.CapacityProjectionDigest = strings.Repeat("f", 64)
		},
		"admitted budget":    func(record *contextcapsule.AuthorityRecord) { record.AdmittedInputBudgetTokens-- },
		"contribution total": func(record *contextcapsule.AuthorityRecord) { record.AdmittedContributionTokens++ },
	} {
		t.Run(name, func(t *testing.T) {
			record := capsule.AuthorityRecord()
			mutate(&record)
			if _, err := contextcapsule.ValidateAuthorityRecord(record); !errors.Is(err, contextcapsule.ErrInvalidCapsule) {
				t.Fatalf("tampered authority error = %v", err)
			}
		})
	}
	recordBody, err := json.Marshal(capsule.AuthorityRecord())
	if err != nil {
		t.Fatal(err)
	}
	var decoded contextcapsule.AuthorityRecord
	if json.Unmarshal(recordBody, &decoded) != nil || decoded != capsule.AuthorityRecord() {
		t.Fatalf("authority JSON round trip = %#v", decoded)
	}
	if _, err := contextcapsule.ValidateAuthorityRecord(decoded); err != nil {
		t.Fatalf("round-tripped authority invalid: %v", err)
	}
}

func TestCapacityProjectionCanonicalMetadataIsPrivacySafe(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 64)
	input := capacityItem("private-item-id", contextcapsule.PrioritySystem, 4, "secret-looking-body", contextcapsule.SourceAuthority)
	input.SourceRef = "private-source-reference"
	capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(target, []contextcapsule.ItemInput{input}, exactCapacityAuthority(),
		fixtureTokenCounter{counts: map[string]int{"secret-looking-body": 4}})
	if err != nil {
		t.Fatal(err)
	}
	projection, _ := capsule.CapacityProjection()
	body, err := contextcapsule.MarshalCanonicalCapacityProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"secret-looking-body", "private-item-id", "private-source-reference", target.ProviderAccountID, target.ConversationID, target.AgentID} {
		if bytes.Contains(body, []byte(forbidden)) {
			t.Fatalf("capacity metadata disclosed %q: %s", forbidden, body)
		}
	}
	if !bytes.Contains(body, []byte(`"priority":0`)) || !bytes.Contains(body, []byte(`"source_type":"authority"`)) {
		t.Fatalf("capacity metadata omitted deterministic grouping: %s", body)
	}
}

func TestCapacityAuthorityTreatsCredentialReferencesAsOpaqueStructuralCost(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 4)
	inputs := []contextcapsule.ItemInput{
		capacityItem("goal", contextcapsule.PrioritySystem, 2, "goal body", contextcapsule.SourceAuthority),
		{
			ItemID: "credential-a", Kind: contextcapsule.KindCredentialReference,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeSecretReferenceOnly,
			Priority: contextcapsule.PriorityConfirmed, TokenCount: 1,
			ReferenceID: "credential-ref-alpha", SourceType: contextcapsule.SourceCredentialReference,
			SourceRef: "provider-account:alpha",
		},
		{
			ItemID: "credential-b", Kind: contextcapsule.KindCredentialReference,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeSecretReferenceOnly,
			Priority: contextcapsule.PriorityHistory, TokenCount: 2,
			ReferenceID: "credential-ref-beta", SourceType: contextcapsule.SourceCredentialReference,
			SourceRef: "provider-account:beta",
		},
	}
	counter := &recordingTokenCounter{
		identifier: "counter:test", version: "v1",
		counts: map[string]int{"goal body": 2},
	}
	capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target, inputs, exactCapacityAuthority(), counter,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(counter.inputs) != 1 || string(counter.inputs[0]) != "goal body" {
		t.Fatalf("TokenCounter saw opaque credential input: %#v", counter.inputs)
	}
	if capsule.TokenCount() != 3 || capsule.IsRetrievable("credential-b") {
		t.Fatalf("credential budget state = %#v", capsule.AuthorityRecord())
	}
	if omissions := capsule.Omitted(); len(omissions) != 1 ||
		omissions[0].ItemID != "credential-b" ||
		omissions[0].Reason != contextcapsule.OmissionBudgetExceeded {
		t.Fatalf("credential omissions = %#v", omissions)
	}
	projection, ok := capsule.CapacityProjection()
	if !ok {
		t.Fatal("capacity projection missing")
	}
	wantCredentialContribution := contextcapsule.CapacityContribution{
		Priority: contextcapsule.PriorityConfirmed, SourceType: contextcapsule.SourceCredentialReference,
		AdmittedItemCount: 1, AdmittedTokenCount: 1,
	}
	wantOmittedCredentialContribution := contextcapsule.CapacityContribution{
		Priority: contextcapsule.PriorityHistory, SourceType: contextcapsule.SourceCredentialReference,
		BudgetOmittedItemCount: 1, BudgetOmittedTokenCount: 2,
	}
	if !containsCapacityContribution(projection.Contributions, wantCredentialContribution) ||
		!containsCapacityContribution(projection.Contributions, wantOmittedCredentialContribution) {
		t.Fatalf("credential contributions = %#v", projection.Contributions)
	}
	metadata, err := contextcapsule.MarshalCanonicalCapacityProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"credential-a", "credential-b", "credential-ref-alpha", "credential-ref-beta",
		"provider-account:alpha", "provider-account:beta",
	} {
		if bytes.Contains(metadata, []byte(forbidden)) {
			t.Fatalf("capacity metadata disclosed %q: %s", forbidden, metadata)
		}
	}
}

func TestCapacityAuthorityRejectsCredentialReferenceBodyBeforeTokenization(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 4)
	input := contextcapsule.ItemInput{
		ItemID: "credential-with-body", Kind: contextcapsule.KindCredentialReference,
		Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeSecretReferenceOnly,
		Priority: contextcapsule.PrioritySystem, TokenCount: 1,
		Content:     []byte("must not cross tokenizer boundary"),
		ReferenceID: "credential-ref", SourceType: contextcapsule.SourceCredentialReference,
		SourceRef: "provider-account-ref",
	}
	counter := &recordingTokenCounter{
		identifier: "counter:test", version: "v1",
		counts: map[string]int{"must not cross tokenizer boundary": 1},
	}
	if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target, []contextcapsule.ItemInput{input}, exactCapacityAuthority(), counter,
	); !errors.Is(err, contextcapsule.ErrInvalidCapsule) {
		t.Fatalf("credential body error = %v", err)
	}
	if len(counter.inputs) != 0 {
		t.Fatalf("TokenCounter saw credential body: %#v", counter.inputs)
	}
}

func TestCapacityAuthorityCountsOnlyPolicyAndScopeAdmittedContent(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 64)
	allowed := capacityItem(
		"allowed", contextcapsule.PrioritySystem, 2, "allowed body",
		contextcapsule.SourceAuthority,
	)
	filtered := capacityItem(
		"filtered", contextcapsule.PriorityConfirmed, 3, "filtered private body",
		contextcapsule.SourceAuthority,
	)
	filtered.PolicyFiltered = true
	denied := capacityItem(
		"denied", contextcapsule.PriorityWorkspace, 4, "role private body",
		contextcapsule.SourceAuthority,
	)
	denied.Scope = contextcapsule.ScopeRoleRestricted
	denied.AllowedRoleID = "another-role"
	counter := &recordingTokenCounter{
		identifier: "counter:test", version: "v1",
		counts: map[string]int{"allowed body": 2},
	}

	capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target,
		[]contextcapsule.ItemInput{denied, allowed, filtered},
		exactCapacityAuthority(),
		counter,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(counter.inputs) != 1 || string(counter.inputs[0]) != "allowed body" {
		t.Fatalf("TokenCounter saw non-dispatchable content: %#v", counter.inputs)
	}
	omissions := capsule.Omitted()
	if len(omissions) != 2 ||
		omissions[0].Reason != contextcapsule.OmissionPolicyFiltered ||
		omissions[1].Reason != contextcapsule.OmissionAccessDenied {
		t.Fatalf("non-dispatchable omissions = %#v", omissions)
	}
}

func TestCapacityAdmissionCompletesBeforeAnyTokenCounting(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 64)
	allowed := capacityItem(
		"allowed", contextcapsule.PrioritySystem, 2, "must not be counted",
		contextcapsule.SourceAuthority,
	)

	t.Run("invalid target", func(t *testing.T) {
		target := target
		target.ArtifactRefs = []string{"artifact-1", "artifact-1"}
		counter := &recordingTokenCounter{
			identifier: "counter:test", version: "v1",
			counts: map[string]int{"must not be counted": 2},
		}
		if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
			target, []contextcapsule.ItemInput{allowed}, exactCapacityAuthority(), counter,
		); !errors.Is(err, contextcapsule.ErrInvalidCapsule) {
			t.Fatalf("invalid target error = %v", err)
		}
		if len(counter.inputs) != 0 {
			t.Fatalf("counter observed content before target admission: %#v", counter.inputs)
		}
	})

	t.Run("invalid later item", func(t *testing.T) {
		invalid := capacityItem(
			"invalid", contextcapsule.PriorityHistory, 1, "invalid body",
			contextcapsule.SourceAuthority,
		)
		invalid.SourceRef = ""
		counter := &recordingTokenCounter{
			identifier: "counter:test", version: "v1",
			counts: map[string]int{"must not be counted": 2, "invalid body": 1},
		}
		if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
			target, []contextcapsule.ItemInput{allowed, invalid}, exactCapacityAuthority(), counter,
		); !errors.Is(err, contextcapsule.ErrInvalidCapsule) {
			t.Fatalf("invalid item error = %v", err)
		}
		if len(counter.inputs) != 0 {
			t.Fatalf("counter observed content before complete item admission: %#v", counter.inputs)
		}
	})

	t.Run("declared capacity overflow", func(t *testing.T) {
		overflow := capacityItem(
			"overflow", contextcapsule.PriorityHistory,
			contextcapsule.MaxCapacityTokens+1, "overflow body",
			contextcapsule.SourceModelOutput,
		)
		counter := &recordingTokenCounter{
			identifier: "counter:test", version: "v1",
			counts: map[string]int{
				"must not be counted": 2,
				"overflow body":       contextcapsule.MaxCapacityTokens + 1,
			},
		}
		if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
			target, []contextcapsule.ItemInput{allowed, overflow}, exactCapacityAuthority(), counter,
		); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
			t.Fatalf("declared overflow error = %v", err)
		}
		if len(counter.inputs) != 0 {
			t.Fatalf("counter observed content before capacity admission: %#v", counter.inputs)
		}
	})

	t.Run("declared capacity cumulative overflow", func(t *testing.T) {
		first := capacityItem(
			"first", contextcapsule.PrioritySystem,
			contextcapsule.MaxCapacityTokens, "first body",
			contextcapsule.SourceAuthority,
		)
		second := capacityItem(
			"second", contextcapsule.PriorityHistory, 1, "second body",
			contextcapsule.SourceModelOutput,
		)
		counter := &recordingTokenCounter{
			identifier: "counter:test", version: "v1",
			counts: map[string]int{
				"first body":  contextcapsule.MaxCapacityTokens,
				"second body": 1,
			},
		}
		if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
			target, []contextcapsule.ItemInput{first, second}, exactCapacityAuthority(), counter,
		); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
			t.Fatalf("declared cumulative overflow error = %v", err)
		}
		if len(counter.inputs) != 0 {
			t.Fatalf("counter observed content before cumulative admission: %#v", counter.inputs)
		}
	})
}

func TestCapacityCountingAndPackingOrderIsCanonical(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 7)
	inputs := []contextcapsule.ItemInput{
		capacityItem("history-z", contextcapsule.PriorityHistory, 2, "history z", contextcapsule.SourceModelOutput),
		capacityItem("confirmed-b", contextcapsule.PriorityConfirmed, 3, "confirmed b", contextcapsule.SourceAuthority),
		capacityItem("system-a", contextcapsule.PrioritySystem, 2, "system a", contextcapsule.SourceAuthority),
		capacityItem("history-a", contextcapsule.PriorityHistory, 2, "history a", contextcapsule.SourceModelOutput),
	}
	counts := map[string]int{"history z": 2, "confirmed b": 3, "system a": 2, "history a": 2}
	wantCounterOrder := [][]byte{[]byte("system a"), []byte("confirmed b"), []byte("history a"), []byte("history z")}

	var referenceDigest, referenceReceipt string
	permutations := [][]int{
		{0, 1, 2, 3},
		{3, 2, 1, 0},
		{1, 3, 0, 2},
		{2, 0, 3, 1},
	}
	for index, permutation := range permutations {
		permuted := make([]contextcapsule.ItemInput, 0, len(inputs))
		for _, itemIndex := range permutation {
			permuted = append(permuted, inputs[itemIndex])
		}
		counter := &recordingTokenCounter{
			identifier: "counter:test", version: "v1", counts: counts,
		}
		capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
			target, permuted, exactCapacityAuthority(), counter,
		)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(counter.inputs, wantCounterOrder) {
			t.Fatalf("permutation %d counter order = %#v", index, counter.inputs)
		}
		if index == 0 {
			referenceDigest = capsule.Digest()
			referenceReceipt = capsule.DisclosureReceiptDigest()
		} else if capsule.Digest() != referenceDigest || capsule.DisclosureReceiptDigest() != referenceReceipt {
			t.Fatalf("permutation %d changed frozen authority", index)
		}
		disclosed := capsule.Disclosed()
		if len(disclosed) != 3 || disclosed[0].ItemID != "system-a" ||
			disclosed[1].ItemID != "confirmed-b" || disclosed[2].ItemID != "history-a" {
			t.Fatalf("P0/P1 were displaced by lower priority input: %#v", disclosed)
		}
		omitted := capsule.Omitted()
		if len(omitted) != 1 || omitted[0].ItemID != "history-z" {
			t.Fatalf("deterministic omissions = %#v", omitted)
		}
		for _, item := range omitted {
			if item.Reason != contextcapsule.OmissionBudgetExceeded || !capsule.IsRetrievable(item.ItemID) {
				t.Fatalf("loss lacks deterministic reason/retrievability: %#v", item)
			}
		}
	}
}

func TestCapacityBoundaryFailuresAndReceiptsStayContentFree(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 8)
	authority := exactCapacityAuthority()

	for name, policyBudget := range map[string]int{
		"zero":     0,
		"negative": -1,
		"overflow": math.MaxInt,
	} {
		t.Run("policy budget "+name, func(t *testing.T) {
			if _, err := contextcapsule.ResolveAdmittedInputBudget(policyBudget, authority); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
				t.Fatalf("policy budget %d error = %v", policyBudget, err)
			}
		})
	}

	tight := authority
	tight.ContextWindowTokens = 3
	tight.ReservedOutputTokens = 1
	tight.AdapterToolOverheadTokens = 1
	p0 := capacityItem("p0", contextcapsule.PrioritySystem, 1, "p0 body", contextcapsule.SourceAuthority)
	p0.Required = true
	p1 := capacityItem("p1", contextcapsule.PriorityConfirmed, 1, "p1 body", contextcapsule.SourceAuthority)
	tightCapsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target, []contextcapsule.ItemInput{p1, p0}, tight,
		fixtureTokenCounter{counts: map[string]int{"p0 body": 1, "p1 body": 1}},
	)
	if err != nil {
		t.Fatal(err)
	}
	tightProjection, _ := tightCapsule.CapacityProjection()
	if tightProjection.AdmittedInputBudgetTokens != 1 || tightCapsule.TokenCount() != 1 ||
		len(tightCapsule.Disclosed()) != 1 || tightCapsule.Disclosed()[0].ItemID != "p0" ||
		len(tightCapsule.Omitted()) != 1 || tightCapsule.Omitted()[0].ItemID != "p1" ||
		!tightCapsule.IsRetrievable("p1") {
		t.Fatalf("one-token boundary packed unsafely: projection=%#v disclosed=%#v omitted=%#v",
			tightProjection, tightCapsule.Disclosed(), tightCapsule.Omitted())
	}
	noInput := tight
	noInput.AdapterToolOverheadTokens = 2
	if _, err := contextcapsule.ResolveAdmittedInputBudget(target.TokenBudget, noInput); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("zero-input capacity error = %v", err)
	}
	unavailableReservationOverflow := authority
	unavailableReservationOverflow.Status = contextcapsule.CapacityUnavailable
	unavailableReservationOverflow.ContextWindowTokens = 0
	unavailableReservationOverflow.ReservedOutputTokens = contextcapsule.MaxCapacityTokens
	unavailableReservationOverflow.AdapterToolOverheadTokens = 1
	if _, err := contextcapsule.ResolveAdmittedInputBudget(
		target.TokenBudget, unavailableReservationOverflow,
	); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("unavailable reservation overflow error = %v", err)
	}

	unknown := authority
	unknown.Status = contextcapsule.CapacityUnavailable
	unknown.ContextWindowTokens = 0
	oversized := capacityItem(
		"oversized", contextcapsule.PrioritySystem, target.TokenBudget+1,
		"oversized private prompt body", contextcapsule.SourceAuthority,
	)
	oversized.Required = true
	if _, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target, []contextcapsule.ItemInput{oversized}, unknown,
		fixtureTokenCounter{counts: map[string]int{"oversized private prompt body": target.TokenBudget + 1}},
	); !errors.Is(err, contextcapsule.ErrRequiredContextOmitted) {
		t.Fatalf("unknown-capacity oversized item error = %v", err)
	}

	goal := capacityItem("goal", contextcapsule.PrioritySystem, 2, "private prompt body", contextcapsule.SourceAuthority)
	capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target, []contextcapsule.ItemInput{goal}, unknown,
		fixtureTokenCounter{counts: map[string]int{"private prompt body": 2}},
	)
	if err != nil {
		t.Fatal(err)
	}
	recordBytes, err := json.Marshal(capsule.AuthorityRecord())
	if err != nil {
		t.Fatal(err)
	}
	projection, ok := capsule.CapacityProjection()
	if !ok || projection.Status != contextcapsule.CapacityUnavailable || projection.ContextWindowTokens != 0 {
		t.Fatalf("unknown capacity was not frozen honestly: %#v", projection)
	}
	projectionBytes, err := contextcapsule.MarshalCanonicalCapacityProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range [][]byte{recordBytes, projectionBytes} {
		if bytes.Contains(body, goal.Content) || bytes.Contains(body, []byte("goal")) ||
			bytes.Contains(body, []byte(goal.SourceRef)) {
			t.Fatalf("capacity receipt leaked prompt/body identity: %s", body)
		}
	}
}

func TestCapacityProjectionRejectsCombinedContributionOverflow(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 64)
	input := capacityItem("goal", contextcapsule.PrioritySystem, 1, "goal body", contextcapsule.SourceAuthority)
	capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target,
		[]contextcapsule.ItemInput{input},
		exactCapacityAuthority(),
		fixtureTokenCounter{counts: map[string]int{"goal body": 1}},
	)
	if err != nil {
		t.Fatal(err)
	}
	projection, ok := capsule.CapacityProjection()
	if !ok || len(projection.Contributions) != 1 {
		t.Fatalf("capacity projection = %#v, present=%t", projection, ok)
	}
	projection.Contributions[0].BudgetOmittedItemCount = 1
	projection.Contributions[0].BudgetOmittedTokenCount = contextcapsule.MaxCapacityTokens
	projection.BudgetOmittedContributionTokens = contextcapsule.MaxCapacityTokens
	if _, err := contextcapsule.MarshalCanonicalCapacityProjection(projection); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("combined contribution overflow error = %v", err)
	}
}

func TestRebuildDispatchSafePreservesCapacityAuthority(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-safe-capacity", "reviewer", 256)
	target.ContextAdapterID = "context:loom-native:v1"
	authority := exactCapacityAuthority()
	counter := fixtureTokenCounter{counts: map[string]int{
		"Continue the admitted task.": 4,
		"API_KEY=unsafe-model-output": 6,
	}}
	capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target,
		[]contextcapsule.ItemInput{
			capacityItem(
				"goal", contextcapsule.PrioritySystem, 4,
				"Continue the admitted task.", contextcapsule.SourceAuthority,
			),
			{
				ItemID: "old-output", Kind: contextcapsule.KindPriorModelOutput,
				Trust: contextcapsule.TrustUntrusted, Scope: contextcapsule.ScopeConversationShared,
				Priority: contextcapsule.PriorityHistory, TokenCount: 6,
				Content: []byte("API_KEY=unsafe-model-output"), SourceType: contextcapsule.SourceModelOutput,
				SourceRef: "attempt:old-1",
			},
		},
		authority,
		counter,
	)
	if err != nil {
		t.Fatal(err)
	}
	safe, err := contextcapsule.RebuildDispatchSafe(capsule, counter)
	if err != nil {
		t.Fatal(err)
	}
	projection, ok := safe.CapacityProjection()
	if !ok || projection.SchemaVersion != contextcapsule.CapacitySchemaVersion ||
		projection.Status != authority.Status ||
		projection.ContextWindowTokens != authority.ContextWindowTokens ||
		projection.ReservedOutputTokens != authority.ReservedOutputTokens ||
		projection.AdapterToolOverheadTokens != authority.AdapterToolOverheadTokens ||
		projection.TokenCounterID != authority.TokenCounterID ||
		projection.TokenCounterVersion != authority.TokenCounterVersion ||
		safe.AuthorityRecord().CapacityProjectionDigest == "" {
		t.Fatalf("safe capacity projection = %#v present=%t", projection, ok)
	}
	payload, err := contextcapsule.RenderDispatchPayload(safe)
	if err != nil || bytes.Contains(payload, []byte("API_KEY=unsafe-model-output")) {
		t.Fatalf("unsafe capacity output crossed dispatch: err=%v", err)
	}
	if _, err := contextcapsule.RebuildDispatchSafe(capsule, nil); !errors.Is(
		err, contextcapsule.ErrInvalidCapacityAuthority,
	) {
		t.Fatalf("capacity rebuild without exact counter error = %v", err)
	}
}

func TestRebuildDispatchSafePreservesExistingOmissionManifestAndCumulativeBound(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-safe-capacity", "reviewer", 2)
	target.ContextAdapterID = "context:loom-native:v1"
	authority := exactCapacityAuthority()
	fixed := capacityItem(
		"fixed-omission", contextcapsule.PriorityHistory,
		contextcapsule.MaxCapacityTokens-2, "must remain omitted",
		contextcapsule.SourceModelOutput,
	)
	fixed.PolicyFiltered = true
	counter := &recordingTokenCounter{
		identifier: "counter:test", version: "v1",
		counts: map[string]int{
			"safe goal":                   1,
			"API_KEY=unsafe-model-output": 1,
		},
	}
	base, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target,
		[]contextcapsule.ItemInput{
			capacityItem("goal", contextcapsule.PrioritySystem, 1, "safe goal", contextcapsule.SourceAuthority),
			capacityItem(
				"unsafe-output", contextcapsule.PriorityHistory, 1,
				"API_KEY=unsafe-model-output", contextcapsule.SourceModelOutput,
			),
			fixed,
		},
		authority,
		counter,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(base.Omitted()) != 1 || base.Omitted()[0].ItemID != fixed.ItemID {
		t.Fatalf("base omissions = %#v", base.Omitted())
	}

	counter.inputs = nil
	safe, err := contextcapsule.RebuildDispatchSafe(base, counter)
	if err != nil {
		t.Fatal(err)
	}
	omissions := safe.Omitted()
	if len(omissions) != 2 || omissions[0].ItemID != fixed.ItemID ||
		omissions[0].Reason != contextcapsule.OmissionPolicyFiltered ||
		omissions[1].ItemID != "unsafe-output" ||
		omissions[1].Reason != contextcapsule.OmissionPolicyFiltered {
		t.Fatalf("dispatch-safe omissions = %#v", omissions)
	}
	if len(counter.inputs) != 1 || string(counter.inputs[0]) != "safe goal" {
		t.Fatalf("dispatch-safe counter inputs = %#v", counter.inputs)
	}
	projection, ok := safe.CapacityProjection()
	if !ok || projection.Status != authority.Status ||
		projection.TokenCounterID != authority.TokenCounterID ||
		projection.TokenCounterVersion != authority.TokenCounterVersion {
		t.Fatalf("dispatch-safe projection = %#v present=%t", projection, ok)
	}
}

func TestExtendRoleContextCapsuleWithCapacityRepacksAndPreservesAuthority(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 8)
	authority := exactCapacityAuthority()
	baseCounter := &recordingTokenCounter{
		identifier: "counter:test", version: "v1",
		counts: map[string]int{
			"goal body": 4, "old history": 4, "policy filtered": 2, "other agent": 2,
		},
	}
	base, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target,
		[]contextcapsule.ItemInput{
			capacityItem("goal", contextcapsule.PrioritySystem, 4, "goal body", contextcapsule.SourceAuthority),
			capacityItem("history-old", contextcapsule.PriorityHistory, 4, "old history", contextcapsule.SourceModelOutput),
			func() contextcapsule.ItemInput {
				item := capacityItem("policy-filtered", contextcapsule.PriorityHistory, 2, "policy filtered", contextcapsule.SourceModelOutput)
				item.PolicyFiltered = true
				return item
			}(),
			func() contextcapsule.ItemInput {
				item := capacityItem("access-denied", contextcapsule.PriorityHistory, 2, "other agent", contextcapsule.SourceObservation)
				item.Scope = contextcapsule.ScopeAgentPrivate
				item.AllowedAgentID = "agent-other"
				return item
			}(),
		},
		authority,
		baseCounter,
	)
	if err != nil {
		t.Fatal(err)
	}
	baseProjection, _ := base.CapacityProjection()
	extensionCounter := &recordingTokenCounter{
		identifier: "counter:test", version: "v1",
		counts: map[string]int{
			"goal body": 4, "old history": 4, "accepted decision": 3, "new history": 2,
		},
	}
	extended, err := contextcapsule.ExtendRoleContextCapsuleWithCapacity(
		base,
		[]contextcapsule.ItemInput{
			capacityItem("accepted", contextcapsule.PriorityConfirmed, 3, "accepted decision", contextcapsule.SourceAuthority),
			capacityItem("history-new", contextcapsule.PriorityHistory, 2, "new history", contextcapsule.SourceModelOutput),
		},
		authority,
		extensionCounter,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !extended.Valid() || !reflect.DeepEqual(extended.Target(), base.Target()) ||
		extended.Digest() == base.Digest() || extended.TokenCount() != 7 {
		t.Fatalf("extended capacity Capsule = %#v", extended.AuthorityRecord())
	}
	if len(extensionCounter.inputs) != 4 {
		t.Fatalf("repack did not recount reconstructed inputs: %#v", extensionCounter.inputs)
	}
	omissions := make(map[string]contextcapsule.OmissionReason)
	for _, omission := range extended.Omitted() {
		omissions[omission.ItemID] = omission.Reason
	}
	if omissions["history-old"] != contextcapsule.OmissionBudgetExceeded ||
		omissions["history-new"] != contextcapsule.OmissionBudgetExceeded ||
		omissions["policy-filtered"] != contextcapsule.OmissionPolicyFiltered ||
		omissions["access-denied"] != contextcapsule.OmissionAccessDenied {
		t.Fatalf("extended omissions = %#v", extended.Omitted())
	}
	for _, itemID := range []string{"history-old", "history-new"} {
		if !extended.IsRetrievable(itemID) {
			t.Fatalf("budget omission %q is not retrievable", itemID)
		}
	}
	extendedProjection, ok := extended.CapacityProjection()
	if !ok || reflect.DeepEqual(extendedProjection.Contributions, baseProjection.Contributions) ||
		extendedProjection.AdmittedContributionTokens != 7 ||
		extendedProjection.BudgetOmittedContributionTokens != 6 {
		t.Fatalf("extended capacity projection = %#v", extendedProjection)
	}
	if _, err := contextcapsule.ExtendRoleContextCapsuleWithCapacity(
		base,
		[]contextcapsule.ItemInput{capacityItem("goal", contextcapsule.PriorityHistory, 1, "duplicate", contextcapsule.SourceAuthority)},
		authority,
		&recordingTokenCounter{identifier: "counter:test", version: "v1", counts: map[string]int{"duplicate": 1}},
	); !errors.Is(err, contextcapsule.ErrInvalidCapsule) {
		t.Fatalf("duplicate extension error = %v", err)
	}
}

func TestExtendRoleContextCapsuleWithCapacityFailsClosedOnAuthorityAndCounterDrift(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 8)
	authority := exactCapacityAuthority()
	base, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target,
		[]contextcapsule.ItemInput{capacityItem("goal", contextcapsule.PrioritySystem, 4, "goal body", contextcapsule.SourceAuthority)},
		authority,
		fixtureTokenCounter{counts: map[string]int{"goal body": 4}},
	)
	if err != nil {
		t.Fatal(err)
	}
	addition := capacityItem("history", contextcapsule.PriorityHistory, 2, "history body", contextcapsule.SourceModelOutput)
	driftedAuthority := authority
	driftedAuthority.ReservedOutputTokens++
	if _, err := contextcapsule.ExtendRoleContextCapsuleWithCapacity(
		base, []contextcapsule.ItemInput{addition}, driftedAuthority,
		fixtureTokenCounter{counts: map[string]int{"goal body": 4, "history body": 2}},
	); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("authority drift error = %v", err)
	}
	if _, err := contextcapsule.ExtendRoleContextCapsuleWithCapacity(
		base, []contextcapsule.ItemInput{addition}, authority,
		&recordingTokenCounter{identifier: "counter:test", version: "v2", counts: map[string]int{"goal body": 4, "history body": 2}},
	); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("counter identity drift error = %v", err)
	}
	if _, err := contextcapsule.ExtendRoleContextCapsuleWithCapacity(
		base, []contextcapsule.ItemInput{addition}, authority,
		fixtureTokenCounter{counts: map[string]int{"goal body": 5, "history body": 2}},
	); !errors.Is(err, contextcapsule.ErrTokenCountMismatch) {
		t.Fatalf("reconstructed token tamper error = %v", err)
	}
}

func TestExtendRoleContextCapsuleWithCapacityIncludesFixedOmissionsInTotalBound(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 8)
	authority := exactCapacityAuthority()
	fixed := capacityItem(
		"fixed-policy-omission", contextcapsule.PriorityHistory,
		contextcapsule.MaxCapacityTokens-1, "fixed private body",
		contextcapsule.SourceModelOutput,
	)
	fixed.PolicyFiltered = true
	base, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target,
		[]contextcapsule.ItemInput{
			capacityItem("goal", contextcapsule.PrioritySystem, 1, "goal body", contextcapsule.SourceAuthority),
			fixed,
		},
		authority,
		fixtureTokenCounter{counts: map[string]int{"goal body": 1}},
	)
	if err != nil {
		t.Fatal(err)
	}
	counter := &recordingTokenCounter{
		identifier: "counter:test", version: "v1",
		counts: map[string]int{"goal body": 1, "addition body": 1},
	}
	_, err = contextcapsule.ExtendRoleContextCapsuleWithCapacity(
		base,
		[]contextcapsule.ItemInput{
			capacityItem("addition", contextcapsule.PriorityConfirmed, 1, "addition body", contextcapsule.SourceAuthority),
		},
		authority,
		counter,
	)
	if !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("fixed omission cumulative overflow error = %v", err)
	}
	if len(counter.inputs) != 0 {
		t.Fatalf("counter observed content before extension capacity admission: %#v", counter.inputs)
	}
}

func TestExtendRoleContextCapsuleWithCapacityReconstructsOpaqueBudgetOmission(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-capacity", "capacity", 3)
	authority := exactCapacityAuthority()
	credential := contextcapsule.ItemInput{
		ItemID: "credential-budget", Kind: contextcapsule.KindCredentialReference,
		Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeSecretReferenceOnly,
		Priority: contextcapsule.PriorityHistory, TokenCount: 2,
		ReferenceID: "credential-ref-private", SourceType: contextcapsule.SourceCredentialReference,
		SourceRef: "provider-account:private",
	}
	base, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		target,
		[]contextcapsule.ItemInput{
			capacityItem("goal", contextcapsule.PrioritySystem, 2, "goal body", contextcapsule.SourceAuthority),
			credential,
		},
		authority,
		fixtureTokenCounter{counts: map[string]int{"goal body": 2}},
	)
	if err != nil {
		t.Fatal(err)
	}
	canonicalCapsule, err := contextcapsule.MarshalCanonicalRoleContextCapsule(base)
	if err != nil {
		t.Fatal(err)
	}
	retrievableContext, err := contextcapsule.MarshalCanonicalRetrievableContext(base)
	if err != nil {
		t.Fatal(err)
	}
	base, err = contextcapsule.RestoreRoleContextCapsule(base.AuthorityRecord(), canonicalCapsule)
	if err != nil {
		t.Fatalf("restore capacity Capsule with credential omission: %v", err)
	}
	base, err = contextcapsule.RestoreRetrievableContext(base, retrievableContext)
	if err != nil {
		t.Fatalf("restore opaque credential omission: %v", err)
	}
	counter := &recordingTokenCounter{
		identifier: "counter:test", version: "v1",
		counts: map[string]int{"goal body": 2, "accepted": 1},
	}
	extended, err := contextcapsule.ExtendRoleContextCapsuleWithCapacity(
		base,
		[]contextcapsule.ItemInput{capacityItem("accepted", contextcapsule.PriorityConfirmed, 1, "accepted", contextcapsule.SourceAuthority)},
		authority,
		counter,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(counter.inputs) != 2 {
		t.Fatalf("TokenCounter saw opaque credential input: %#v", counter.inputs)
	}
	projection, _ := extended.CapacityProjection()
	metadata, err := contextcapsule.MarshalCanonicalCapacityProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(metadata, []byte(credential.ReferenceID)) ||
		bytes.Contains(metadata, []byte(credential.SourceRef)) {
		t.Fatalf("extended capacity metadata disclosed credential identity: %s", metadata)
	}
	if omissions := extended.Omitted(); len(omissions) != 1 || omissions[0].ItemID != credential.ItemID {
		t.Fatalf("extended credential omission = %#v", omissions)
	}
}

func containsCapacityContribution(
	contributions []contextcapsule.CapacityContribution,
	want contextcapsule.CapacityContribution,
) bool {
	for _, contribution := range contributions {
		if reflect.DeepEqual(contribution, want) {
			return true
		}
	}
	return false
}

func TestLegacyCapsuleCanonicalBytesAndDigestsRemainStableWithoutCapacity(t *testing.T) {
	t.Parallel()
	target := testRoleTarget("agent-legacy", "legacy", 12)
	goal := capacityItem("goal", contextcapsule.PrioritySystem, 4, "Legacy goal.", contextcapsule.SourceAuthority)
	goal.Required = true
	goal.SourceRef = "goal:legacy"
	history := capacityItem("history", contextcapsule.PriorityHistory, 10, "Legacy history.", contextcapsule.SourceModelOutput)
	history.SourceRef = "attempt:legacy"
	history.Scope = contextcapsule.ScopeConversationShared
	capsule, err := contextcapsule.BuildRoleContextCapsule(target, []contextcapsule.ItemInput{goal, history})
	if err != nil {
		t.Fatal(err)
	}
	body, err := contextcapsule.MarshalCanonicalRoleContextCapsule(capsule)
	if err != nil {
		t.Fatal(err)
	}
	const legacyBody = `{"schema_version":1,"target":{"ConversationID":"conversation-1","TeamID":"team-1","AgentID":"agent-legacy","RoleID":"legacy","ProviderID":"deepseek","ProviderAccountID":"deepseek.primary","ModelID":"deepseek-chat","AuthMode":"brokered","ContextAdapterID":"loom-native-context-v1","DisclosurePolicyID":"policy.local-default","DisclosurePolicyVersion":1,"ArtifactRefs":null,"TokenBudget":12},"disclosed":[{"ItemID":"goal","Kind":"conversation_goal","Trust":"authoritative","Scope":"team_shared","Priority":0,"TokenCount":4,"Required":true,"Content":"TGVnYWN5IGdvYWwu","ContentDigest":"80332018080eb93e84f1942f0026490d8929e9936031ea7345e0d52cf916c61b","ReferenceID":"","SourceType":"authority","SourceRef":"goal:legacy","AllowedAgentID":"","AllowedRoleID":"","ArtifactRef":""}],"omitted":[{"ItemID":"history","Kind":"untrusted_model_output","Trust":"untrusted","Scope":"conversation_shared","Priority":3,"TokenCount":10,"Required":false,"ContentDigest":"c7de60d0e503d814bcfeec9702b0ec2db107df27e74d76a71392361cf9feb536","SourceType":"model_output","SourceRef":"attempt:legacy","AllowedAgentID":"","AllowedRoleID":"","ArtifactRef":"","Reason":"budget_exceeded"}]}`
	if string(body) != legacyBody {
		t.Fatalf("legacy canonical bytes changed:\n%s", body)
	}
	if capsule.Digest() != "3cbc1dc0825bbd550b514bcce7b22e952606a187954cd8dc982b5fc9a72869bd" ||
		capsule.DisclosureReceiptDigest() != "f496248781770bad3559ef18aabfc4b8df3f22a4d887e402aa5f7e62025e15ad" {
		t.Fatalf("legacy digests changed: capsule=%s receipt=%s", capsule.Digest(), capsule.DisclosureReceiptDigest())
	}
	if _, ok := capsule.CapacityProjection(); ok || capsule.AuthorityRecord().CapacitySchemaVersion != 0 {
		t.Fatal("legacy capsule invented capacity authority")
	}
	restored, err := contextcapsule.RestoreRoleContextCapsule(capsule.AuthorityRecord(), body)
	if err != nil || restored.Digest() != capsule.Digest() {
		t.Fatalf("legacy round trip failed: %#v, %v", restored, err)
	}
}

func exactCapacityAuthority() contextcapsule.CapacityAuthority {
	return contextcapsule.CapacityAuthority{
		SchemaVersion: contextcapsule.CapacitySchemaVersion,
		Status:        contextcapsule.CapacityExact, ContextWindowTokens: 128_000,
		ReservedOutputTokens: 8_192, AdapterToolOverheadTokens: 1_024,
		TokenCounterID: "counter:test", TokenCounterVersion: "v1",
	}
}

func capacityItem(itemID string, priority contextcapsule.Priority, tokenCount int, content string, sourceType contextcapsule.SourceType) contextcapsule.ItemInput {
	item := contextcapsule.ItemInput{
		ItemID: itemID, Kind: contextcapsule.KindConversationGoal,
		Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
		Priority: priority, TokenCount: tokenCount, Content: []byte(content),
		SourceType: sourceType, SourceRef: "source:" + itemID,
	}
	if sourceType == contextcapsule.SourceModelOutput {
		item.Kind = contextcapsule.KindPriorModelOutput
		item.Trust = contextcapsule.TrustUntrusted
	}
	if sourceType == contextcapsule.SourceObservation {
		item.Kind = contextcapsule.KindCurrentTaskState
		item.Trust = contextcapsule.TrustObserved
	}
	return item
}
