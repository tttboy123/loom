# Loom Scan Prompt: Request 001

You are scanning a Buddys-side upstream request dropbox.

Your job is not to implement Buddys product features. Your job is to decide whether this request should become a Loom upstream task.

## Read first

1. `../001-devflow-capability-alignment.request.md`
2. `../loom-upstream-requests.yaml`

## Decision rules

Accept the request only if:

- the capability is useful for external projects beyond Buddys
- it can be implemented without hardcoding Buddys docs, backlog, roles, or product terms
- it improves Loom as a generic external execution engine

Reject or split the request if:

- it is really a downstream wrapper concern
- it depends on Buddys-specific acceptance logic
- it would force Loom global defaults to match Buddys

## Output format

Produce:

1. `accept / reject / split`
2. one-paragraph rationale
3. proposed upstream task breakdown if accepted
4. explicit list of Buddys-specific assumptions that must stay out of Loom core

## Hard boundary

Do not rewrite Loom into “the Buddys engine”.
Treat Buddys only as one concrete external-project signal source.
