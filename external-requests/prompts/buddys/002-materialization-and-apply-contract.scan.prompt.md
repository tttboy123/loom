# Loom Scan Prompt: Request 002

You are scanning a Buddys-side upstream request dropbox.

Your job is not to implement Buddys product features. Your job is to decide whether this request should become a Loom upstream task.

## Read first

1. `../002-materialization-and-apply-contract.request.md`
2. `../loom-upstream-requests.yaml`

## Decision rules

Accept the request only if:

- the capability is useful for external projects beyond Buddys
- it improves generic output materialization and apply semantics
- it does not assume a specific downstream repo layout or validator

Reject or split the request if:

- it depends on Buddys-specific overlay logic
- it asks Loom to bypass safety boundaries silently
- it really belongs in a downstream wrapper

## Output format

Produce:

1. `accept / reject / split`
2. one-paragraph rationale
3. proposed upstream task breakdown if accepted
4. explicit list of downstream-specific assumptions that must stay out of Loom core

## Hard boundary

Do not rewrite Loom into a downstream repo manager.
Treat this as a generic external-project execution contract question.
