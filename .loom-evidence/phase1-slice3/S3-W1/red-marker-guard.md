# S3-W1 Mandatory Marker RED

- Date: `2026-07-25`
- Product file present: no
- Command:
  `go test ./protocol/bridge/v1 -run '^TestS3W1MandatoryMarkers$' -count=1`
- Exit: `1`

The guard failed solely because every frozen behavior marker was absent:

```text
s3_w1_frame_exact_round_trip count = 0, want 1
s3_w1_frame_rejection_matrix count = 0, want 1
s3_w1_json_duplicate_key_rejection count = 0, want 1
s3_w1_frame_bounds count = 0, want 1
s3_w1_stream_binding_sequence_uniqueness count = 0, want 1
s3_w1_stream_terminal_and_buffer_bounds count = 0, want 1
s3_w1_mutation_concurrency_fuzz_static count = 0, want 1
```

This is the required marker-only RED. No production symbol existed.

VERDICT: RED
