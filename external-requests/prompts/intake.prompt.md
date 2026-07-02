# Loom External Requests Intake Prompt

You are scanning Loom's external request inbox.

## Read in order

1. `external-requests/README.md`
2. `external-requests/requests.yaml`
3. every request listed with `status: pending_scan`

## Your job

For each pending request:

1. decide `accept`, `reject`, or `split`
2. explain whether the capability is generic to Loom or still source-specific
3. identify which parts must remain outside Loom core
4. if accepted, propose a small upstream task breakdown

## Hard boundaries

- do not assume any external project's docs become Loom defaults
- do not hardcode source-specific roles, models, or backlog fields into Loom global behavior
- do not accept requests that are only one project's local wrapper concern

## Output shape

For each request id, emit:

- `decision`
- `reason`
- `upstreamable_scope`
- `keep_downstream`
- `suggested_split`

Then emit a final summary:

- accepted count
- rejected count
- split count
- top upstream themes
