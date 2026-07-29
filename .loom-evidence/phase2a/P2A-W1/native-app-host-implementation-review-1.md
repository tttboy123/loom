# P2A-W1 Native App Host Implementation Review 1

**Date**: 2026-07-28
**Reviewer**: fresh independent read-only Reviewer
**Verdict**: `FAIL`
**Live authority**: none

## Findings

### 1. Medium: request ID grammar is broader than Go v1

Go requires ASCII `[A-Za-z0-9._:-]`. The Swift validator uses
`CharacterSet.alphanumerics`, which accepts non-ASCII letters and numbers. The
injected request-ID seam can therefore produce a request the Go server rejects.

### 2. Medium: safe text retains newline and tab controls

The sanitizer explicitly preserves newline and tab. Native labels can
therefore become multiline or tabbed, contrary to the frozen single-line
safe-text boundary.

### 3. Low: bundle builder does not self-enforce static exclusions

The static forbidden-surface scan exists only in the build test wrapper. The
builder verifies signature, modes, identity, symlinks, and architecture, but
does not itself fail closed on forbidden native product source.

## Reviewer verification

- Swift 14-test matrix: `PASS`
- focused Go/Swift component tests: `PASS`
- Swift build state reset: `PASS`
- staging empty: `PASS`

This `FAIL` authorizes bounded Review Repair inside the existing exact W1
owned files. It does not authorize install, launch, live canary, resident
daemon mutation, P2A-W2, or another WorkItem.
