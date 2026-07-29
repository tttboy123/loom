# P2A-W1 Local Product Launch Failure Replacement Activation Audit

**Status**: `CONSUMED - FAIL - ROLLBACK_INCOMPLETE - ORIGINAL SERVICE LATE-RECOVERED`

**Allowance**: `0`

**Resident PID**: `18085`

**Resident Runs**: `37`

P2A-W1 Local Product Launch Failure Replacement and one controlled native-window canary

## Authority

The user's standing continuation authorization, frozen P2A-W1 repair contract,
Contract Re-review 4 PASS, and fresh Implementation Re-review PASS permit this
audit to issue one replacement allowance. This record does not itself install,
bootstrap, restart, launch the App, contact a Provider, or execute a Runtime.

Final allowance accounting:

```text
invocations=1
remaining=0
retry=no
```

## Stable resident baseline

Three read-only samples spanning sixteen seconds after all resource-heavy
verification ended were identical:

```text
sample=1 state=running pid=18085 runs=37 program=expected
sample=2 state=running pid=18085 runs=37 program=expected
sample=3 state=running pid=18085 runs=37 program=expected
```

The resident has no Candidate target marker. Historical observations remain
immutable and are not reinterpreted:

```text
accepted exact-path baseline: pid=85936 runs=1
disclosed invalid parallel matrix: pid=97912 runs=18 last_exit=4
post-repair matrix observation: pid=95529 runs=35 last_exit=4
first post-matrix stable window: pid=95853 runs=36
current audited stable window: pid=18085 runs=37
```

## Immutable continuity

```text
installed loom
60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698

installed loomd
e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a

installed wrapper
ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4

installed plist
2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4

resident Journal
91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4
integrity=ok events=1

historical reports
f284754aeca95cbceaf10070b2dd7465e4646ce3ab39a4e484c6ee04aff5e6c0
4804009d0fda5b9f29706be8a0373156706af9c845af5dd995dedc8054810f54
```

The native App, native App process, product run root/socket, replacement files,
failure-reason record, and staged changes are absent. No Provider credential
or Runtime execution was used.

## Reviewed identity chain

```text
contract
047f4bc691bc7304f0ba2e8082081ba1a91a176bae4aa4519ce4eff01b618fe3

source lock
8c75b532951de6ce8372fb76584d8246d625e310b7300e0af9c022b8cd903abb

Candidate manifest record
ec03f11e6b46512994ce2d9c389b019102feadb11398a986ff346b6f3d234167

transaction
42ceeb37364a2bfd065d7347f211c1c8d892e55d75b58025d84bb2c516770ca5

transaction fixture
71dc4a53e8b0097307e04ff0c3d7f933bd1f4bdbcd15cc7f488530e3657c7aca

Candidate loom
b12e5261632efc10585163ea323a22951f736b5565b5a717b0cf77a2765d0f29

Candidate loomd
af29fbb9cfa46ac0b97321a3240c9e7fd4048f0f64ce55cfd1b139a1a691d6e6

Candidate App executable
3aedc0ba90be3cdfb3df0865016abebbc9cc5b659bfa6a445ae4a54af272cfba

Candidate App UUID
93E3FC61-0A19-3379-BFDB-8CA7726F59F6

Candidate bundle manifest
bdbf57511fbbcd56c583af3e2bb84f03d45f2b012b4a1eb6a5f087203e2d4a58
```

Candidate directories and executables are uid `501`, mode `0700`; both
identity records are uid `501`, mode `0600`; the App signature is strict valid.
The source lock contains `201` inputs and independently closes all `171`
enumerated in-repository Go inputs plus all 16 native Swift inputs.

## Decision

The one reviewed invocation was consumed. It failed before
`LOCAL_PRODUCT_CLOSURE_READY` and returned `rollback_incomplete`. The native
App was never launched and no UI verification decision was sent. The original
service later recovered with exact immutable bytes and a stable running
process, but the transaction result remains `FAIL`; it is not reinterpreted as
success. No hidden retry or second Candidate bootstrap is authorized.
