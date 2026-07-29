# P2A-W1 Local Product Launch Failure Replacement Live Canary

**Status**: `FAIL - ROLLBACK_INCOMPLETE - ORIGINAL SERVICE LATE-RECOVERED`

**Exit**: `HUMAN_REQUIRED - NO RETRY`

**Allowance before**: `1`

**Allowance after**: `0`

## Invocation

The exact reviewed transaction was invoked once:

```text
transaction_sha256=42ceeb37364a2bfd065d7347f211c1c8d892e55d75b58025d84bb2c516770ca5
initial_bootstrap_calls=1
consumed=1
restart_count=0
```

Exact bounded output:

```text
LOCAL_PRODUCT_CLOSURE_RESULT result=rollback_incomplete initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0
```

The transaction failed before emitting `LOCAL_PRODUCT_CLOSURE_READY`.
Therefore:

- the native App was not opened;
- Computer Use performed no UI action;
- `ui_verified` was not sent;
- the controlled restart phase was not reached;
- `result_review_pass` was not sent;
- no retry or second Candidate bootstrap occurred.

No closed failure-reason record was produced. The exact internal pre-ready
predicate that triggered `set -e` is therefore not claimed.

## Rollback result

The synchronous rollback deadline ended while the restored original observer
was still `spawn scheduled`, so the transaction correctly reported
`rollback_incomplete`. The exact original service subsequently recovered
without another transaction or manual lifecycle command.

Three read-only samples spanning sixteen seconds were identical:

```text
sample=1 state=running pid=43503 runs=32
sample=2 state=running pid=43503 runs=32
sample=3 state=running pid=43503 runs=32
```

Immutable post-recovery state:

```text
installed loom
60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698
mode=0755 uid=501

installed loomd
e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a
mode=0755 uid=501

installed wrapper
ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4
mode=0700 uid=501

installed plist
2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4
mode=0600 uid=501

resident Journal
91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4
integrity=ok events=1
```

Both historical crash-report hashes remain exact. The native App, native App
process, run root/socket, `.previous` files, command wrapper replacements,
private transaction root, and failure-reason record are absent. Git staging is
empty.

## Verdict

The replacement canary is a governed `FAIL`, not a partial success. Late
recovery proves the user's pre-existing resident installation is safe; it does
not satisfy the P2A-W1 native-window acceptance boundary. P2A-W1 remains
unaccepted and uncommitted, P2A-W2 remains locked, and no additional live
allowance exists.
