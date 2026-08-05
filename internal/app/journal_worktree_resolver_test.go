package app

import (
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
)

// NewJournalWorktreeResolverForTest exposes the resolver used by the product
// service for tests outside this package.
func NewJournalWorktreeResolverForTest(store *journal.Store) WorktreeResolver {
	return &journalWorktreeResolver{store: store}
}

type WorktreeResolver = execution.WorktreeResolver
