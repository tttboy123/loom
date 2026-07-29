# P2A-W1 Reopen 3: Ambient Service-Manager Metadata Attribution

**Date**: 2026-07-28
**Status**: ACTIVE — explicitly authorized for one final controlled canary
**Parent**: frozen P2A-W1 Local App Shell and Read Experience Contract
**Supersedes only**: the loaded-service zero-marker predicate in section 16,
Amendment 2, and the consumed-canary stop

## Evidence-led diagnosis

Both governed live gates rolled back exactly. The second gate proved:

- the reviewed Candidate bootstrapped and exposed one private default socket;
- the disk plist contained no Provider environment entries;
- the wrapper executed `loomd` through `/usr/bin/env -i` with only `PATH` and
  `LANG`;
- the Candidate process command/environment and logs contained no Provider
  marker or secret;
- SQLite bytes, Event count, canonical head digest, and state authority were
  unchanged;
- loaded launchd service metadata still displayed five Provider keys.

Read-only follow-up now proves the same exact five keys have non-empty values in
the user-level launchd manager environment:

```text
DEEPSEEK_BASE_URL
STEPFUN_BASE_URL
MINIMAX_BASE_URL
DEEPSEEK_API_KEY
STEPFUN_API_KEY
```

No value was emitted, copied into evidence, or supplied to Loom. The service
metadata marker set was the ambient manager marker set; it was not introduced
by the Loom plist or inherited by the `env -i` child.

The former zero-marker predicate therefore conflated ambient manager metadata
visible to `launchctl print` with the actual child-process environment. Making
that diagnostic view empty would require deleting unrelated user-level
Provider configuration. Reversible deletion is also unsafe: restoring a
secret through `launchctl setenv KEY VALUE` would place it in another process's
arguments.

## Reopened trust predicate

This Reopen does not authorize credential mutation. It replaces only the
impossible zero-marker predicate with an attribution and non-inheritance
predicate:

1. The exact on-disk plist must contain no `EnvironmentVariables`, Provider
   key, Provider URL, token, credential, or secret marker.
2. The exact installed wrapper must retain its reviewed hash and invoke
   `loomd` through `/usr/bin/env -i` with only the reviewed non-secret
   allowlist.
3. Candidate process arguments and effective environment must contain none of
   the five keys and no credential marker or secret value.
4. Candidate stdout/stderr logs, Journal, Evidence, TUI output, screenshots,
   manifests, and repository evidence must remain secret-negative.
5. The ambient user-manager marker-name set and the service-visible
   marker-name set may contain the exact five names above only when:
   - the sets are equal;
   - no value is printed or persisted;
   - non-emitting per-key comparisons prove the manager values are unchanged
     before and after the gate;
   - the child-process environment contains none of them.
6. Any additional service-only Provider marker, any child inheritance, any
   value disclosure, or any ambient-value change fails closed and rolls back.
7. `launchctl setenv`, `launchctl unsetenv`, credential reads into files,
   secret restoration through process arguments, and OS Secret Store mutation
   are forbidden.

This criterion matches the Phase 2A objective: Provider secrets must not enter
source, process arguments, prompts, logs, Journal, Evidence, AgentDefinition,
or screenshots. It does not weaken those surfaces.

## Final canary authority requested

Because two prior allowances are consumed, execution requires a specific user
authorization after independent Review. If authorized, exactly one final
controlled W1 canary may:

1. rebuild and reverify the unchanged reviewed Candidate hashes;
2. capture exact installed/SQLite/service/socket pre-state;
3. install atomically and wait for launchd namespace quiescence;
4. bootstrap once from the reviewed clean command context;
5. apply the reopened attribution/non-inheritance predicate;
6. use Computer Use to open the installed no-argument TUI and inspect Runtime,
   Teams, Attention, Runs, Evidence, and the historical Team timeline without
   entering a Team ID, cursor, SQLite path, socket path, or service command;
7. quit/relaunch the TUI, restart the daemon once, and prove the same canonical
   view/terminal lineage with unchanged Event count and no duplicate effect;
8. preserve the Candidate only on full PASS; otherwise restore exact pre-state.

No fourth canary, hidden retry, alternate socket, direct foreground daemon,
Provider/model request, Runtime execution, Team/WorkPackage mutation,
credential change, global environment change, or authority write is permitted.

## Authorization boundary

Independent Review may validate this Reopen, but may not activate it. Execution
requires an explicit user message authorizing:

```text
P2A-W1 Ambient Service-Manager Metadata Attribution Reopen
and one final controlled live canary
```

Without that exact new authority, P2A-W1 remains
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`.

## Activation

The user supplied the exact authorization on `2026-07-28`:

```text
授权 P2A-W1 Ambient Service-Manager Metadata Attribution Reopen and one final controlled live canary
```

Reopen 3 is therefore active for exactly the final canary defined above. No
other credential, Provider, Runtime, authority, retry, or publication action is
authorized by activation.
