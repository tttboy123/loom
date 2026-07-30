# P2A-W2 Interaction Continuity Repair 1 Implementation Re-review

**Reviewer**: second fresh independent read-only Reviewer
**Verdict**: `FAIL`

## Finding

### P1 — role-option continuity remains incomplete

The final preflight now exposes the complete already-bound role, Provider,
model, auth, permissions, compatibility and budget, but compatible role options
are still rendered only as responsibility labels. Native and TUI confirmation
can edit only name and purpose; neither submits the existing
`builder_edit main_role/subagent_role` fields.

The complete frozen contract requires role-option-backed Provider/model
presentation and switching within two user actions without introducing a new
model protocol or authority.

## Confirmed closures

The Reviewer confirmed all three Implementation Review 1 findings closed:

- complete native/TUI final preflight;
- bounded native/TUI task search preserving the selected task;
- observed exact-remove interleaving revalidation.

It also confirmed `internal/localipc/swift_contract_test.go` is unchanged and
outside the Candidate. Focused TUI, localipc, Swift and diff checks passed.

## Gate

Repair 2 remains inside this same complete contract. It adds no Amendment or
W4. Staging, commit and live actions remain locked.
