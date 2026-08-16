package projection

import (
	"fmt"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

type ProviderAccountPolicyRecord struct {
	Policy      work.ProviderAccountPolicy
	LastEventID string
}

func applyProviderAccountPolicyProjection(
	snapshot *Snapshot,
	event journal.Event,
) error {
	if snapshot == nil {
		return fmt.Errorf("%w: Provider Account policy snapshot", ErrInvalidProjectionEvent)
	}
	if snapshot.ProviderAccountPolicies == nil {
		snapshot.ProviderAccountPolicies = make(
			map[string]ProviderAccountPolicyRecord,
		)
	}
	accountID := providerAccountPolicyAccountID(event.StreamID)
	if accountID == "" {
		return fmt.Errorf("%w: Provider Account policy stream", ErrInvalidProjectionEvent)
	}
	existing, found := snapshot.ProviderAccountPolicies[accountID]
	previousEventID := ""
	if found {
		previousEventID = existing.LastEventID
	}
	policy, _, err := work.DecodeProviderAccountPolicyConfiguredEvent(
		event, previousEventID,
	)
	if err != nil || policy.ProviderAccountID() != accountID ||
		(found && (policy.ProviderID() != existing.Policy.ProviderID() ||
			policy.Revision() != existing.Policy.Revision()+1 ||
			!policy.ConfiguredAt().After(existing.Policy.ConfiguredAt()))) ||
		(!found && policy.Revision() != 1) {
		return fmt.Errorf("%w: Provider Account policy fact", ErrInvalidProjectionEvent)
	}
	snapshot.ProviderAccountPolicies[accountID] = ProviderAccountPolicyRecord{
		Policy: policy, LastEventID: event.ID,
	}
	return nil
}

func providerAccountPolicyAccountID(streamID string) string {
	const prefix = "provider-account-policy/"
	if len(streamID) <= len(prefix) || streamID[:len(prefix)] != prefix {
		return ""
	}
	return streamID[len(prefix):]
}
