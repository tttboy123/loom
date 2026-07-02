# Loom Scan Prompt: Request 004

You are scanning a Buddys-side upstream request dropbox.

Your job is not to implement Buddys product features. Your job is to decide whether this request should become a Loom upstream task.

## Read first

1. `../004-external-queue-lease-and-recovery.request.md`
2. `../requests.yaml`

## Decision rules

Accept the request only if:

- the capability is useful for external projects beyond Buddys
- it can be implemented without hardcoding Buddys docs, backlog lanes, or retry policy
- it improves Loom as a generic external execution engine for resident queue loops

Reject or split the request if:

- it is really just downstream wrapper glue
- it assumes one external project's state files as Loom defaults
- it would force Loom global queue behavior to mirror Buddys exactly

## Output format

Produce:

1. `accept / reject / split`
2. one-paragraph rationale
3. proposed upstream task breakdown if accepted
4. explicit list of Buddys-specific assumptions that must stay out of Loom core

## Hard boundary

Do not rewrite Loom into “the Buddys engine”.
Treat Buddys only as one concrete external-project signal source.
