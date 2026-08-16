# Phase 2C Repair Journey Attempt 007 Non-product Failure

**Date**: 2026-08-08  
**Source-lock SHA-256**: `fe5ea9bf23399c0a8e7c9361629b0a72d84321ba55206b373b1cadfc90758952`  
**Outcome**: `INVALID - OPERATOR INPUT CONTAMINATION`

The controlled J6 companion fixture passed: the real PTY TUI selected
`allow_once`, the daemon accepted `mission_decision`, and the Journal advanced
from 73 to 75 unique facts with `ApprovalDecided` and
`WorkItemApprovalResolved`; SQLite integrity remained `ok`.

The primary fixture also proved J1/J2, one-shot restart-expired Team Draft
recovery, and explicit Team confirmation. While leaving Team Builder, the
operator entered `g b`; that screen owns `g` as credential entry, so `b` was
submitted as a temporary MiniMax credential. It was immediately revoked and no
secret was retained, but the append-only Journal correctly preserved one
`ProviderCredentialConfigured` and one `ProviderCredentialRevoked` fact. The
fixture is therefore intentionally excluded from acceptance rather than
rewritten or promoted.

All Attempt 007 clients and daemons stopped. The subsequent clean primary
fixture is Attempt 008.
