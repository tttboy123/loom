# S5-W1 Implementation Repair 2 RED

Date: 2026-07-26
Baseline: `006db8c`

Tests were added before product changes for both Implementation Review 2
findings.

```text
go test ./internal/api \
  -run '^TestAuthoritativeSafeFieldsRejectAmbiguityAndIgnoreRawKeys$' \
  -count=1

FAIL
case 4 error = <nil>
```

The first new case is `{"retry_at":"tomorrow"}`. The remaining new cases cover
non-UTC/noncanonical time plus malformed and uppercase Evidence digests.

```text
go test ./internal/app \
  -run '^TestTeamExecutionStreamReceivesPostCaptureAuthorizedOutput$' \
  -count=1

FAIL
missing verifier Evidence relation error = <nil>
```

The second test removes the verifier Evidence fact from an otherwise valid
high-risk local SQLite fixture, rebuilds Projection successfully, then proves
the current scope incorrectly accepts the dangling WorkItem Evidence
reference.

The failures are the intended behavioral RED. No product file was edited
before capture.

VERDICT: PASS
