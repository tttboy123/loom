# P2A-W2 Observer Diagnostic and Live Closure Amendment Contract Review

**Date**: 2026-07-30
**Review type**: fresh independent read-only Contract Review
**Verdict**: `REPAIR REQUIRED`

## Findings

```text
P0 = none
P1 = 2
P2 = 1
VERDICT = REPAIR REQUIRED
```

### P1 — component retry was under-bounded

The Candidate froze one real locked-Pi component lineage and one
version/model-list pair per execution but allowed rerunning the installed-Pi
component “until GREEN”. That was an unbounded real-component loop.

Repair 1 permits at most execution A and, only after A fails and a code/test
repair returns deterministic GREEN, execution B. A GREEN forbids B; B failure
stops `HUMAN_REQUIRED`; no automatic retry exists.

### P1 — manifest and enable gate were not frozen

The Candidate required an exact Controller-supplied manifest/enable input but
did not freeze their path, schema, identity fields, hash checks, mismatch
behavior or residue/offline assertions.

Repair 1 freezes the exact Application Support root and manifest path, strict
schema and values, locked Pi/Node/llama/GGUF identities, ordered search paths,
private modes/owner/no-symlink requirements, manifest hashing, exact enable
environment names/values, skip-before-construction behavior, and post-run
cleanup/offline assertions.

### P2 — CLI reason projection was ambiguous

The Candidate simultaneously said caller-visible error text must not change and
allowed an allowlisted reason projection.

Repair 1 freezes exit code `4` and exact stderr
`daemon failed: <allowlisted observer reason>\n`, forbids raw wrapped text, and
keeps the existing `local_ipc` and `shutdown` strings unchanged.

## Confirmed boundaries

The amendment remains inside P2A-W2, creates no W4, keeps W3 locked, preserves
Journal/StateWriter/Projection authority, maintains construction and
per-command file identity fencing, and gates replacement attempt 004 behind
component, deterministic and Implementation Review GREEN.

## Independence

The Reviewer performed read-only contract/source/evidence inspection only. It
edited, staged and committed nothing and ran no test, installed Pi, daemon,
native app, network, Keychain, Provider or live action.

Repair 1 requires fresh independent Contract Re-review.

## Fresh independent Contract Re-review Repair 1

**Verdict**: `PASS`

```text
P0 = none
P1 = none
P2 = none
VERDICT = PASS
```

The Reviewer confirmed:

- execution A and conditional execution B are the complete finite
  installed-Pi component allowance;
- the exact component root, manifest path/schema, enable inputs, Pi resolved
  target/hash, Node/search paths, llama/model identities, skip behavior and
  offline/residue checks are frozen;
- observer stderr has one exact allowlisted shape with no raw wrapped text;
- the projection-synchronized observation seam is a necessary, bounded owned
  file and does not add authority;
- attempt 004 remains locked behind component, deterministic and fresh
  Implementation Review GREEN.

The Re-review was read-only. The Reviewer modified and executed nothing and
used no installed Pi, daemon, app, network, Keychain, Provider or credential
surface.

## Contract Repair 2

Implementation preflight found that Repair 1 overloaded `private_root` with the
fresh component root. The accepted llama-server and GGUF are descendants of
the existing private `/phase1-live` root, not the component root; using the
component root would make the accepted catalog binding fail closed before Pi.

Repair 2 separates:

- `component_root`: the fresh component lineage root;
- `isolation_root`: its empty disposable metadata child;
- `local_model_private_root`: the exact existing
  `/Users/lune/Library/Application Support/Loom/phase1-live` binding root.

No process or live action exposed this defect. Fresh independent Contract
Repair 2 Re-review is required before component execution A.

## Fresh independent Contract Repair 2 Re-review

**Verdict**: `PASS`

```text
P0 = none
P1 = none
P2 = none
VERDICT = PASS
```

The Reviewer confirmed that the frozen llama-server and GGUF are descendants
of `local_model_private_root`, while `component_root` and its
`isolation_root` remain fresh, separate and disposable. The strict schema,
skip-closed input, path/hash/mode/owner/size and empty-isolation gates remain
intact.

Repair 2 changes no authority or live allowance. The Re-review was read-only
and used no installed Pi, daemon, app, network, Keychain, Provider, credential
or live surface.
