# P3A-W1 Alternative Verification Amendment — Independent Review Chain

Date: `2026-08-04`

Reviewers: fresh independent read-only Reviewers (Codex CLI, read-only
sandbox)

## Review 1 (FAIL)

```text
P0 = 0  P1 = 0  P2 = 1
Product/Authority: PASS
Operational/Trace Governance: FAIL
```

P2: owned runbook and prepare script still instructed Computer Use driving,
contradicting the Amendment §3 alternative method.

## Review 2 (FAIL)

```text
P0 = 0  P1 = 1  P2 = 2
Product/Authority: PASS
Operational/Trace Governance: FAIL
```

P1: S1 evidence files (artifacts/processes) did not match the frozen
`P3A-W1-CONTRACT-REPAIR-1.md` §8 item schemas and lacked `journey_id`.
P2a: Amendment §3.4 and runbook item lists contradicted §8.
P2b: residual Computer-Use-driving wording in inventory/S1 docs.

## Review 3 (FAIL)

```text
P0 = 0  P1 = 1  P2 = 1
Product/Authority: PASS
Operational/Trace Governance: FAIL
```

P1: processes pre/postflight carried an extra plaintext `root` field not in
§8. P2: residual Computer-Use status wording in the inventory.

## Review 4 (FINAL PASS)

```text
Reviewed Amendment SHA-256: e7710c140345f3da5a18a362547fa2f83da9388db3d8dfc4adc70c697c3b4965
P0 = 0  P1 = 0  P2 = 1 (non-blocking)
Product/Authority: PASS
Operational/Trace Governance: PASS
Overall Amendment: PASS
```

Closed: processes/artifacts/keystrokes/timeline evidence conform to §8;
no Computer-Use-driving instruction remains outside the Amendment and the
Environment Audit; supersession bounded; production-only client paths;
eight scenario IDs; no P3A-W2/dependency/migration; nothing staged.
Non-blocking P2: the inventory referenced the stale `/tmp/p3a-evidence.sh`
producer; corrected to the conforming `/tmp/p3a-evidence.py` producer.

VERDICT: `PASS`
