# S5-W2 Focused GREEN

Command:

```sh
go test ./internal/work ./internal/app \
  -run 'WorkPackage|Phase1EngineeringDemo|Phase1LiveGate' \
  -count=1
```

Result:

```text
ok  	loom-pi-rebuild/internal/work	0.744s
ok  	loom-pi-rebuild/internal/app	2.259s
```

The focused suite proves:

- exact built-in WorkPackage JSON and frozen SHA-256 identities;
- 100 input-order permutations, aggregate encoded-size rejection, typed
  mismatch/invalid errors, zero-value rejection, and input/accessor deep copy;
- explicit Agent routing plus saved-Team instantiation prerequisites for both
  built-ins;
- the same normalized Journal authority/state trace across Coding and
  knowledge-work packages;
- Main plus two independent SubAgents, bounded explicit recovery, distinct
  verifier lineage, tentative source observation before authoritative Done,
  and no tentative payload in Journal;
- approval pause/decision across new Rule Authority and Projection instances;
- coordinator restart and cursor reconnect without duplicate authoritative
  delivery, Run, Grant, Evidence, Done, or terminal facts;
- stale WorkPackage digest, generation, cursor, and failed Projection rebuild
  fail closed while retaining the last good read view;
- private fixture modes and the exact bounded non-executing live-gate manifest.

No installed Runtime, Provider/model traffic, network, daemon, credential,
external effect, or user-file mutation was used.
