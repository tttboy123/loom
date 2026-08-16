package projection

import (
	"fmt"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

type RemoteToolBackendEnrollmentRecord struct {
	Enrollment  work.RemoteToolBackendEnrollment
	LastEventID string
}

func applyRemoteToolBackendEnrollmentProjection(
	snapshot *Snapshot,
	event journal.Event,
) error {
	if snapshot == nil ||
		(event.Type != "RemoteToolBackendEnrollmentConfigured" &&
			event.Type != "RemoteToolBackendEnrollmentRevoked") {
		return fmt.Errorf("%w: remote tool backend enrollment snapshot", ErrInvalidProjectionEvent)
	}
	if snapshot.RemoteToolBackendEnrollments == nil {
		snapshot.RemoteToolBackendEnrollments = make(
			map[string]RemoteToolBackendEnrollmentRecord,
		)
	}
	existing, found := snapshot.RemoteToolBackendEnrollments[event.StreamID]
	previousEventID := ""
	if found {
		previousEventID = existing.LastEventID
	}
	enrollment, _, err := work.DecodeRemoteToolBackendEnrollmentEvent(
		event, previousEventID,
	)
	if err != nil || !enrollment.Valid() ||
		(found && (enrollment.EnrollmentID() != existing.Enrollment.EnrollmentID() ||
			enrollment.ProviderID() != existing.Enrollment.ProviderID() ||
			enrollment.ProviderAccountID() != existing.Enrollment.ProviderAccountID() ||
			enrollment.BackendKind() != existing.Enrollment.BackendKind() ||
			enrollment.Revision() != existing.Enrollment.Revision()+1 ||
			!enrollment.ConfiguredAt().After(existing.Enrollment.ConfiguredAt()))) ||
		(!found && (enrollment.Revision() != 1 ||
			enrollment.Status() != work.RemoteToolBackendEnrollmentActive)) {
		return fmt.Errorf("%w: remote tool backend enrollment fact", ErrInvalidProjectionEvent)
	}
	snapshot.RemoteToolBackendEnrollments[event.StreamID] = RemoteToolBackendEnrollmentRecord{
		Enrollment: enrollment, LastEventID: event.ID,
	}
	return nil
}
