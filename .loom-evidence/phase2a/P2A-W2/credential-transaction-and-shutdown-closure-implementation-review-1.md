# P2A-W2 Credential Transaction and Shutdown Closure Implementation Review 1

Date: 2026-07-30

Reviewer mode: fresh independent, read-only

Verdict: `FAIL — REPAIR REQUIRED`

## Findings

### P1 — production credential-boundary construction failure was swallowed

`buildProductSetupService` called `credentials.NewProductKeychainStore`, but
returned `nil, nil` when construction failed. `newProductDaemonRunner` could
therefore continue with no setup service, leaving every setup method to return
`state_unavailable` instead of failing daemon construction at the reviewed
credential boundary.

The existing service-construction tests injected a fake `CredentialStore` and
did not cover this production branch.

Required repair: return a real closed setup/credential-boundary error and add a
targeted test proving construction fails closed.

### P2 — historical contract status text is stale

The frozen amendment's header still says Contract Repair 1 is pending, while
the later immutable review evidence and `docs/CURRENT.md` record PASS. This is
an evidence-hygiene finding, not a product-code blocker. Repair evidence must
make the historical/frozen header relationship explicit; the already reviewed
contract behavior may not be silently changed.

## Reviewer checks

The Reviewer ran the focused credential/helper/product-daemon tests. They
passed before the missing construction-failure test was added. No live,
Keychain, Provider, network or daemon action ran.
