# P2A-W1 Native App Host Final Post-Review Activation Audit

Date: 2026-07-28  
Status: `PASS — PRE-LIVE ONLY — ACTIVATION NOT RECEIVED`  
Candidate bootstrap count: `0`  
Native live allowance: `1`, unconsumed  
Historical Reopen 4 allowance: `0`, consumed

## Closed implementation and review lineage

The exact live phrase supplied before the repair lineage reached only
pre-bootstrap dry-run. It safely exposed the accepted legacy resident installer
shape gap and did not mutate live state or consume the allowance.

The final reviewed lineage is:

1. Live Pre-Bootstrap Repair 1 RED: accepted `loom + loomd` resident rejected;
2. Repair 1 GREEN: exact pair/triple optional-launcher install and rollback;
3. Repair 1 Implementation Review 1 `FAIL`: signal recovery gap;
4. Repair 1 Review Repair: active install/rollback signal restores and exits
   `98`;
5. Repair 1 Implementation Re-review 2 `PASS`;
6. Live Pre-Bootstrap Repair 2 RED: clean Swift bundles differed through
   LC_UUID and N_OSO timestamps;
7. Repair 2 GREEN: UUID-free unsigned link, `strip -S`, one final signature,
   and two package-reset reproducible builds;
8. Repair 2 Implementation Review 1 `FAIL`: canonical complete manifest bytes
   not stated;
9. Repair 2 evidence repair: exact
   `relative_path NUL stat-%Sp-mode NUL lowercase-sha256 LF` canonical rows;
10. Repair 2 Implementation Re-review 2 `PASS`, findings none.

No point Amendment, Reopen 5, W4, authority change, or live retry was created.

## Final independently reproduced Candidate

```text
loom
3ad7deb7e2210aa305c071690bad24b50aa22984a13df6215d8ee8ea537c919f

loomd
ff5a00dc59cdd1db39998d7013e0989b388da403bb9d4307400e0eb9fd680357

LoomLocalApp
6e9f03c064a1af09e27446a194bcbf5f5e14c3cd7e69d4a959af73229f2ebea8

Info.plist
554a6c8b990a75b5e86318a2a4e4ed7002fd2b8c4063c6a6071e5900d5132ba5

CodeResources
6686de10a28a2fe11b36cbb86dcbacc827cfc4ea116b4dabf1845e5aee629e9b

canonical complete bundle manifest
221e9878db56b451dd5b724ec0b2e3ceacf349269da9fea3cf571690bc33f67a
```

The Reviewer independently reproduced every value after a fresh Swift package
reset and passed both exact live destination dry-runs.

## Frozen original state

```text
original loom
60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698

original loomd
e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a

original wrapper
ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4

original plist
2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4

resident SQLite
91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4

canonical GlobalReadView head digest
6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05
```

The original observer is loaded and running. SQLite integrity is `ok` and
Event count is `1`. Product run directory, default and historical sockets,
compatibility launcher, native app, and Swift build cache are absent. Git
staging is empty.

The target resident process contains none of the five frozen Provider marker
names. No Provider or launchd ambient value is recorded.

## Activation decision

All deterministic and fresh independent implementation-review gates are
closed. No Candidate is installed or retained outside private build fixtures.
No bootstrap, Computer Use, app launch, daemon restart, Journal write, Runtime
execution, Team creation, Provider action, or authority transition occurred.

The only remaining live gate is a new exact post-Review user message:

```text
P2A-W1 Native App Host and one controlled native-window canary
```

Only that message activates the single runbook transaction. Any earlier
activation predates the final reviewed Candidate and cannot be reused.
