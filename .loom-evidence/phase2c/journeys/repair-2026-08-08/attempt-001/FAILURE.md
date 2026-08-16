# Phase 2C Cross-Client Journey Attempt 001

**Result**: `FAIL_PRE_ACCEPTANCE`  
**Journey ID**: `9669340a-a641-49e1-b8c2-84c3ec74e74b`  
**Source lock SHA-256**: `eb0376b6d8f42da8e6ac586b57fe4796fdc887b2c84c14c8dac6988e252feee9`

## Failure

The real PTY TUI rendered `i message` and `u Agent Team` on Home, but both
keys were inert. Source inspection after the observed failure confirmed that
the Home key handler had no branch for either advertised action. No
`chat_message`, `setup_snapshot`, or builder request followed those keypresses.

This is a product defect, not an environment or harness failure. J1 and J4
cannot pass on the frozen Repair 2 source, so the source lock and its completed
deterministic matrix are retained as historical evidence only. Repair 3 must
add RED interaction tests, implement both Home actions, pass independent
review, regenerate the source lock, and rerun deterministic verification before
a replacement journey begins.

## Observed State

- Native App opened chat-first and performed production `chat_thread`,
  `snapshot`, `setup_snapshot`, and permission reads over the journey socket.
- TUI opened chat-first in a real PTY and performed its initial production
  `snapshot` read.
- Runtime discovery was online with one configured local model.
- Journal contained exactly three initialization/runtime facts:
  `AgentGrantIdentityIndexInitialized`, `RuntimeInstanceDiscovered`, and
  `WorkRunIdentityIndexInitialized`.
- Team facts: `0`; Mission facts: `0`; duplicate Event IDs: `0`; duplicate
  idempotency keys: `0`; SQLite integrity: `ok`.
- Native App, TUI, daemon, and socket were all closed after the failure.

## Artifact Digests

The failed root is retained outside the repository as consumed diagnostic
material. Raw screenshots are not copied into source control because the
full-display capture contains unrelated desktop content.

| Artifact | SHA-256 |
|---|---|
| Native full-display checkpoint | `9b6e1cff31c8327777bc812f0e55ba0259a7207c759b8d5ea345d599240d0c49` |
| Real PTY transcript | `a65310cce66f92df739fbbffa6e9b0d63b668794c53cf72e1f5310b1f366d96a` |
| Sanitized IPC summary | `65d202fc9a464e7c1d969240fb0227b4fd6b4d510736cfe4eafd6482b69108f2` |
| Sanitized daemon log | `b212f36eeab7ef986d112adb2ea58bfe6b067fc6d254a3c04937ea7d9d6408d4` |
| Closed SQLite state | `f33ff25ee81a36cd5f5b7e2875a3aed8d5a40d128427d3288d46cabeea56231f` |

