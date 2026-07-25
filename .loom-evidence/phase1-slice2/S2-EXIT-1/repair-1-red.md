# S2-EXIT-1 Repair 1 Mandatory RED

- Baseline: `39a9e0a`
- Date: `2026-07-25`
- Command:

```text
go test ./internal/app -run '^TestS2EXIT1Repair1MandatoryMarkers$' -count=1
```

- Exit status: `1`
- Failure scope: exactly the seven frozen mandatory markers were absent.

```text
mandatory repair marker "s2_exit_metadata_duplicate_identity_zero_append" count = 0, want 1
mandatory repair marker "s2_exit_metadata_invalid_identity_zero_append" count = 0, want 1
mandatory repair marker "s2_exit_metadata_non_utc_zero_append" count = 0, want 1
mandatory repair marker "s2_exit_metadata_cancellation_zero_append" count = 0, want 1
mandatory repair marker "s2_exit_metadata_sequence_overflow" count = 0, want 1
mandatory repair marker "s2_exit_configuration_complete_rejection_matrix" count = 0, want 1
mandatory repair marker "s2_exit_configuration_typed_nil_identity" count = 0, want 1
```

No product file had been changed when this RED was captured.

VERDICT: PASS
