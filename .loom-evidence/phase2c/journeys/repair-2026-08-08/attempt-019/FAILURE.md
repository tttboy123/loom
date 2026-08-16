# Phase 2C Replacement Journey Attempt 019 - Failed

Attempt 019 is preserved and must not be promoted.

The Repair 18 source-locked binaries used main Journey UUID
`934b7224-e98e-4952-95a3-113a059c7bbb`. The daemon failed before opening its
product socket or accepting any IPC because the fresh attempt harness omitted
the required pre-existing private observer isolation directory. `loomd`
returned `daemon unavailable: build_observer`; both controlled journey logs
remain empty and no Journey action ran.

This is a harness construction error, not a product-source failure. The
observer validates its isolation root before the product journey harness can
perform stale-root recovery. Attempt 020 therefore adds an owned `0700`
`isolation` directory before daemon startup and otherwise retains the Repair 18
source lock, runtime model tuple, explicit Journey UUID discipline, and fresh
state boundaries.

The resident demo daemon was only observed and was not modified. J1-J10 were
not run and no Phase or ADR acceptance claim is made.
