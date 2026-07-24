# Loom Documentation Rules

These rules apply under `docs/`.

- Lead with actual product status and user impact. Put internal IDs and exact
  event names in technical detail, not in place of the conclusion.
- Mark capabilities as `CURRENT`, `PARTIAL`, `TARGET`, or `EXPERIMENTAL`.
  Design documents must not describe target behavior as implemented.
- `docs/CURRENT.md` is the small volatile status snapshot. Do not copy its
  changing status into root `AGENTS.md`.
- `TECH-PLAN.md` defines the Phase 1 implementation contract. Architecture
  documents explain it; they do not silently replace it.
- Record durable, consequential choices in `docs/adr/`. Keep rejected options
  and consequences.
- Mermaid text is the architecture source. Existing PNG files are snapshots and
  are updated only when explicitly requested.
- Prefer links to the authoritative definition over repeating long rules.
- Preserve historical evidence. Never rewrite a failed result into a pass.
- Check relative links and Mermaid syntax for edited documents.
