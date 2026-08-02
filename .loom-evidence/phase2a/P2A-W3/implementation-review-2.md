# P2A-W3 Independent Implementation Re-review 2

**Date**: 2026-08-02  
**Reviewer**: fresh independent read-only Reviewer  
**Source lock**: `source-lock.json` (all 29 digests independently matched)  
**Verdict**: `FAIL`  
**Live gate**: `LOCKED`

## P0

None.

## P1

1. Product clients generated an ephemeral `mission-<random>` identity, while
   the authoritative rebuildable read model derives `mission/<team_instance_id>`
   from TeamExecution facts. The native client therefore could fail its own
   post-Start projection visibility check and the TUI could not reconnect to
   the same Mission identity.
2. `mission_execution/control` checked Team, node, attempt, generation and
   execution digest but did not bind MissionID. A caller with the correct
   execution lineage and a different bounded MissionID could reach cancel or a
   prepared decision and receive a result echoing the drifted identity.

## P2

None.

## Independently verified

- all 29 source-lock digests matched;
- Review 1 repairs for strict Swift probe coverage, terminal-flight reaping and
  the five-minute preflight lease were implemented;
- prepared-control routing used the accepted decision boundary;
- focused App, real Swift probe and vertical loopback checks passed;
- no Provider, credential, daemon live attempt or canary ran.

## Gate decision

The Candidate remains unaccepted. Repair 2 must close both identity findings
inside the existing owned boundary and receive a fresh source lock plus a new
independent Re-review before any live action.
