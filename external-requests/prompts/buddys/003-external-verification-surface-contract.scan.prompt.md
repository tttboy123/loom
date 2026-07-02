# Loom Scan Prompt: Request 003

You are scanning a Buddys-side upstream request dropbox.

Your job is not to implement Buddys product features. Your job is to decide whether this request should become a Loom upstream task.

## Read first

1. `../003-external-verification-surface-contract.request.md`
2. `../loom-upstream-requests.yaml`

## Decision rules

Accept the request only if:

- the capability helps external projects reason about verify authority and surface mismatch
- it can be implemented generically
- it does not hardcode Buddys tests, repos, or acceptance policy

Reject or split the request if:

- it mainly requests downstream-specific verifier adapters
- it depends on Buddys wrappers or task metadata not suitable for Loom core

## Output format

Produce:

1. `accept / reject / split`
2. one-paragraph rationale
3. proposed upstream task breakdown if accepted
4. explicit list of downstream-specific assumptions that must stay out of Loom core

## Hard boundary

Do not turn Loom into Buddys' private acceptance engine.
Treat Buddys only as one external-project signal source.
