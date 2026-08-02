# P2A-W3 Timeline Lineage Repair RED

Date: 2026-08-03

root004 reproduced a read-only vertical failure when the TUI opened the
completed Mission. A bounded diagnostic against the copied 103-Event SQLite
located the first rejected authoritative record:

```text
WorkItemRejected
stream: work-item/<internal work identity>
attempt_number: 1
error: invalid delivery record
```

The Event is valid under the accepted write contract: the work-item stream and
GlobalReadView already bind it to one logical node and attempt, while the
payload carries `attempt_number` without redundantly carrying
`logical_node_id`. The old reader treated either lineage field as requiring
both fields, so it rejected the valid partial metadata before any rendering.

The permanent causal test was added first and failed to compile because the
new closed validation boundary did not yet exist:

```text
go test -count=1 -run TestDeliveryLineageFieldsAcceptOnlyIndependentlyCorroboratedPartialMetadata ./internal/api

undefined: validateDeliveryLineageFields
FAIL loom-pi-rebuild/internal/api [build failed]
```

