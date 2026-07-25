# S3-W1 Contract Amendment 1 — Exact Twelve-Field Envelope

- Parent contract SHA-256:
  `2a563528fe9c6148caf2fc221369994ce1628a1ff5fb2c689e7aba5548bb6a87`
- Trigger: Contract Review 1 `FAIL`
- Date: `2026-07-25`
- Product repair attempts: unchanged

This amendment makes one normative wording correction.

## Replacement

Replace Frame contract item 2 with:

> Top-level fields are exactly these twelve fields from `TECH-PLAN.md`:
> `protocol_version`, `message_id`, `correlation_id`, `work_item_id`, `run_id`,
> `claim_generation`, `runtime_instance_id`, `sender_agent_instance_id`, `seq`,
> `type`, `emitted_at`, and `payload`. Missing, unknown, or duplicate fields
> reject.

Required proof item 2 additionally requires direct tests that:

- a valid envelope contains exactly those twelve decoded top-level keys;
- removing any one of the twelve rejects;
- adding any thirteenth top-level key rejects.

## Unchanged boundary

All API, owned files, limits, sentinels, other semantics, RED markers, checks,
trust boundaries, and exclusions remain unchanged. This amendment adds no
payload schema, ACK/session, process, persistence, Grant, or execution
authority.

The active contract is the parent plus this amendment. Fresh independent
Contract Review must return `PASS` before mandatory RED.

VERDICT: PASS
