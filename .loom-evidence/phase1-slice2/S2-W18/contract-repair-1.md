# S2-W18 Contract Repair 1

- Lineage: `S2-W18`
- Repair type: contract only
- Product/test writes: none authorized

## Repair

1. Freeze operation-plus-cleanup failure as a zero-result `errors.Join`
   contract. Caller cancellation/deadline and the cleanup sentinel must both
   remain inspectable with `errors.Is`; cleanup never hides the operation
   failure and the operation never hides cleanup residue.
2. Limit Unix claims to termination of the original process group. The test
   proves a same-group child is terminated; new-session/process-group escape is
   an explicit residual risk deferred to a later OS supervisor/sandbox.
3. Revalidate canonical runtime-search-directory identity/type before every
   call and state exactly that this binds directory selection, not interpreter
   bytes. `/usr/bin/env` interpreter/content drift remains a trusted
   higher-layer risk and cannot be described as covered by the executable
   digest.
4. Clarify cleanup evidence: ordinary terminal paths must leave no directory;
   an intentionally induced cleanup denial must return the cleanup sentinel,
   after which the test restores permissions and removes only the exact
   recorded fixture residue.

The repaired contract requires a fresh independent review and new digest before
mandatory RED.

VERDICT: PASS
