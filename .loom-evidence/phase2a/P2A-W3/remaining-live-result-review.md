# P2A-W3 MiniMax and Pi Result-Evidence Review

**Date**: 2026-08-02  
**Role**: fresh independent read-only Reviewer  
**Product result**: `FAIL` for both canaries  
**Evidence verdict**: `PASS`  
**Governance verdict**: `HUMAN_REQUIRED / NO COMMIT / NO P2A-W4`

## Findings

- P0: none.
- P1: none.
- P2: the native bundle manifests record the bundle path and executable digest,
  but not the resolved executable path. The Reviewer resolved
  `CFBundleExecutable = LoomLocalApp` from the bundle plist and independently
  reproduced the expected executable digest.
- P2: the Reviewer's final non-database secret-value scan command had a quoting
  error and did not produce a result. It printed no secrets. The Controller's
  separate bounded non-database scan returned `secret_pattern_file_count=0`;
  the Reviewer independently found only credential references/status metadata,
  not raw credential fields, in the retained DB payload keys.

## Reproduced identities

```text
repository HEAD
  848f068cbc0307f14473f6a71961db949a8734ca

source lock
  03d213d9862ce1f7be95d4547bfaae67893e58a8c98018d6a6cd651bd251c3ce

Implementation Review 4
  eba43885f1b8e0ab5ca7c8b27592475e12ee482e3bfc33e9eaa343a2c8156053

MiniMax active manifest
  a96860080c32c91e21bd399165a956d2164646a98ebff0e6ec52c307b5d20d36

Pi active manifest
  5657c07a0ca1d71862ce676a80c06e370bfa2d6c706cd41c5c7619cc99752bc1
```

The Reviewer independently resolved the native executable from the bundle,
reproduced digest `b0be856c4553291e62ff1b108664e3f167f2502ed4147309f55b598a7642329a`,
and verified the private modes and absence of symlinks under both attempt roots.

## MiniMax result

The retained MiniMax DB reproduced SHA-256
`946871393f5689b1d1eb10280e3d71dc80af581b220e6778abb9c0a4b0cc43f9`
and SQLite integrity `ok`. Its complete relevant inventory remains:

```text
ProviderCredentialConfigured | 1
ProviderCredentialVerified   | 3
RuntimeInstanceDiscovered    | 2
```

The initial W2-derived DB already contained three
`ProviderCredentialVerified` facts. The final DB still contains three, so the
single product `Test` action did not commit the mandatory new terminal
verification fact. Classifying this canary as
`FAIL — VERIFICATION FACT NOT COMMITTED` is exact.

## Pi result

The retained Pi DB reproduced SHA-256
`677624b624b68ddd908939766752177a802bd90b657075133ef845e67e5461a2`
and SQLite integrity `ok`. It contains exactly one Runtime discovery, one saved
TeamDefinition, one TeamInstance and one Main AgentInstance, plus the two
identity initialization facts. Exact terminal/start/run/grant/evidence queries
returned zero.

The Reviewer independently reproduced the conflict cause. Current
`ProjectionMissionExecutionBindingSource` requires:

```text
qwen2.5-coder-1.5b-instruct-q4-k-m
```

The retained live Runtime discovery instead contains:

```text
loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m
```

Existing tests inspected by the Reviewer use only the unnamespaced identity.
The product therefore failed closed before Start. Classifying this canary as
`FAIL — PRODUCT PREFLIGHT CONFLICT / NO START` is exact.

## Cleanup and decision

For both attempts, product socket and product lock are absent, isolation is
empty, no DB/lock/socket handle remains open, and narrowed attempt/executable
process scans return no rows. Attempt roots are `0700`; DBs and manifests are
`0600`.

The failed evidence is trustworthy, but it is not product acceptance. The
required no-terminal walkthrough is ineligible because the three prerequisite
live gates did not pass. Correcting the Pi mismatch requires a reviewed reopen
of the source-locked mission-binding boundary; MiniMax also needs a governed
product verification repair. The current W3 Candidate must stop
`HUMAN_REQUIRED`, without an atomic acceptance commit and without creating
P2A-W4.

**VERDICT**: `PASS FOR EVIDENCE / PRODUCT FAIL / HUMAN_REQUIRED`
