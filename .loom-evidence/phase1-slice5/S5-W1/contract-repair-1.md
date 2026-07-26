# S5-W1 Contract Repair 1 — Exact Local API Surface

- Date: `2026-07-26`
- Baseline: `006db8c`
- Scope: governance only; no product or RED test edit

## Trigger

The Controller's pre-RED audit found that Contract Review 1 correctly accepted
the capability, ownership, bounds, and authority design, but the contract
listed exact Journal/View APIs without enumerating the equivalent public
`internal/api` stream/page/subscription/gap methods or the CLI injected
dependency signature.

That omission would allow RED or implementation to invent an interface after
freeze, contrary to the Slice 5 Exit Contract admission checklist.

## Bounded repair

Repair 1 changes only the S5-W1 contract. It freezes:

- `ViewSource` and `TeamExecutionStreamConfig`;
- constructor, page, subscription, observer, and close signatures;
- immutable result accessors and JSON boundary;
- the typed `TimelineGapError` multi-cause `errors.Is` contract; and
- the exact CLI `timelineInput` and injected dependency signature.

It does not add an owned file, behavior, package, WorkItem, dependency,
migration, writer, daemon, network surface, S5-W2, or S5-W3.

The repaired contract must receive a fresh independent Contract Repair Review
2 `PASS` before mandatory RED begins.

VERDICT: PASS
