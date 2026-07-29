# P2A-W1 Local Product Vertical Live Closure Activation Audit

**Date**: 2026-07-29  
**Status**: `FAIL - ROLLED_BACK - HUMAN_REQUIRED`
**Allowance**: `0`
**Candidate bootstrap count**: `1`  
**Native app launch count**: `0`

## Reviewed gates

- replacement Contract Review 1: `FAIL`, repaired;
- fresh replacement Contract Re-review 2: `PASS`;
- mandatory RED: preserved;
- deterministic GREEN and complete matrix: `PASS`;
- Implementation Review 1: `FAIL`, activation-path defect repaired;
- fresh independent Implementation Re-review 2: `PASS`;
- Git staging: empty.

## Exact Candidate

```text
Candidate manifest
7c0ec8cd75129f0184d345ea39684bc1b1850db56907f1b95ea57f61302a1700

loom
b12e5261632efc10585163ea323a22951f736b5565b5a717b0cf77a2765d0f29

loomd
fdca8152e924d32e2a0163a74e0e357afbcd135fce58be25f1c531d3ebb1596b

native executable
3aedc0ba90be3cdfb3df0865016abebbc9cc5b659bfa6a445ae4a54af272cfba

arm64 LC_UUID
93E3FC61-0A19-3379-BFDB-8CA7726F59F6

canonical bundle manifest
bdbf57511fbbcd56c583af3e2bb84f03d45f2b012b4a1eb6a5f087203e2d4a58

transaction
a23b3d3ca04040c42d8fdb2b891e945e44cf3ff7b2966cca354faf35da6c528a

transaction fixture
6f7b8252d861561bc2beed3616fdbd3e8343bb5316497f8f03580796438d2e1d
```

Candidate parent/root, bundle directories, and executables are user-owned
`0700`; ordinary bundle files are `0600`. The App is arm64, has a non-zero
UUID, is strict-signature valid, and contains no symlink. The manifest contains
no credential, Provider environment, private path, or live Journal copy.

## Exact resident pre-state

```text
loom
60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698
mode 0755

loomd
e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a
mode 0755

loomd-clean
ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4
mode 0700

LaunchAgent plist
2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4
mode 0600

resident SQLite
91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4
mode 0600
integrity ok
Events 1
historical model_ids JSON type null

canonical GlobalReadView version
6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05
```

The observer is running from the exact `loomd-clean` wrapper. Both expected
provenance attributes are present. The closed Provider-marker count is zero;
no process environment value was captured.

The crash inventory is exactly the two accepted historical reports:

```text
f284754aeca95cbceaf10070b2dd7465e4646ce3ab39a4e484c6ee04aff5e6c0
4804009d0fda5b9f29706be8a0373156706af9c845af5dd995dedc8054810f54
```

Installed native App, compatibility launcher and backups, product run root,
socket, native process, Swift cache, and staged Git changes are absent.

## Activation

The active Phase 2A objective and the user's standing authorization permit the
single replacement canary after reviewed gates and exact pre-state audit:

P2A-W1 Local Product Live Closure Replacement and one controlled native-window canary

This record atomically changes only the frozen replacement allowance from `0`
to `1`. It permits the exact reviewed transaction, one initial Candidate
bootstrap, the bounded Home/Work/Teams/Inbox/System native-window checks, one
Refresh, one App quit/relaunch recovery proof, and the one classified later
daemon lifecycle restart. It does not permit retry, alternate socket,
Provider/Runtime execution, Team creation, product-source change, credential
change, commit, acceptance, or P2A-W2.

Any post-bootstrap failure consumes the allowance, rolls back, and stops
`FAIL - ROLLED_BACK - HUMAN_REQUIRED`.

## Consumed result

The reviewed transaction performed exactly one initial Candidate bootstrap.
The Candidate daemon exited with the closed `daemon failed` surface before the
transaction emitted `LOCAL_PRODUCT_CLOSURE_READY`. The native App was never
opened and no UI action occurred.

The transaction emitted:

```text
LOCAL_PRODUCT_CLOSURE_RESULT result=rollback_incomplete initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0
```

The incomplete classification was limited to original-service stabilization
after all Candidate files, App, run/socket, backups, plist, binaries, and
Journal had already been restored. Bounded rollback completion then:

- re-established the exact original `loomd` and wrapper bytes at their
  governed paths;
- retained the exact original modes and macOS provenance values;
- allowed launchd's bounded retries to converge on one stable original
  observer PID;
- observed that same PID for more than 60 seconds across metadata cycles;
- restored two byte-identical historical crash reports after macOS moved them
  to `DiagnosticReports/Retired`;
- revalidated the one-Event Journal, paths, crash inventory, process marker
  count, and staging.

Final rollback is exact. The allowance remains consumed and cannot authorize a
retry. P2A-W1 stops `FAIL - ROLLED_BACK - HUMAN_REQUIRED`.
