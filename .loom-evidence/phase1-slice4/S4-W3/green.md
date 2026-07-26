# S4-W3 GREEN

Date: 2026-07-26
Baseline: `6d3cbf2`

The frozen S4-W3 Candidate now closes deterministic acceptance, independent
Verifier isolation, Journal-authoritative WorkItem/Team acceptance, bounded
verification-rejection recovery, Projection, restart, and terminal-once
behavior.

Focused command:

```text
go test ./internal/verification ./internal/rules ./internal/work ./internal/app ./internal/projection -count=1
```

Observed:

```text
ok loom-pi-rebuild/internal/verification
ok loom-pi-rebuild/internal/rules
ok loom-pi-rebuild/internal/work
ok loom-pi-rebuild/internal/app
ok loom-pi-rebuild/internal/projection
```

Behavioral coverage includes:

- immutable versioned AcceptanceContract validation and canonical digests;
- source output stopping at `ready_for_review`;
- low-risk deterministic acceptance and exact terminal-once Done;
- distinct verifier WorkItem/Run/Grant/Evidence lineage;
- stale generation, forged Grant, and divergent lineage rejection;
- explicit bounded retry and exhaustion after verifier rejection;
- concurrent acceptance with one CAS winner and no partial mutation;
- expired never-started verifier Claim rebound and receipt restart without
  re-execution;
- exact new Event JSON schemas and deterministic Event IDs;
- Projection failure preserving the previous immutable GlobalReadView; and
- controlled local SQLite/Supervisor Team execution proof.

No installed Runtime, Provider, daemon, network, credential, API/CLI/Web/TUI,
autonomous loop, external action, dependency, checkpoint, or S4-W4 was added or
activated.

VERDICT: PASS
