# S2-W3 Contract Review 1

- Review type: fresh independent read-only contract Reviewer
- Reviewed contract SHA256:
  `9c30f92f0a4c8540099f6f8be338f935db9bbd38a0ac345425ddb027433ad8d9`
- Branch/head: `codex/loom-platform-slice2` at `1170062`
- Reviewer verdict: `FAIL`
- Findings: one blocking contract ambiguity

## Blocking finding

The frozen contract did not define the counted unit for:

1. raw AgentDefinition Candidates versus scope-resolved selected Agent catalog
   entries; and
2. unique model strings versus Runtime-scoped Runtime/model pairs after online
   filtering.

Because the same stable Agent ID may have several scoped/versioned Candidates
and the same model string may appear under several RuntimeInstances, two
implementations could accept different catalogs while both claiming to enforce
the same maxima. Mandatory RED could not be written deterministically.

No other authority leak or blocking issue was found. Team Draft revision,
default Main Agent, defined-Team load, TeamInstance, Bridge, AgentGrant, real
Runtime execution, persistence, and activation remained outside the WorkItem.

## Required repair

Freeze exact post-resolution/post-filter count units, including how duplicate
model strings across RuntimeInstances count, and require boundary tests for
every maximum before RED.

VERDICT: FAIL
