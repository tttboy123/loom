# P2A-W1 Reopen 4 Controlled Live Canary Result-Evidence Review

**Date**: `2026-07-28`
**Reviewer**: fresh independent read-only Reviewer
**Verdict**: `PASS`
**Findings**: none

## Scope

The Reviewer independently inspected:

- the frozen Reopen 4 Vertical Live Closure contract;
- post-Review activation evidence;
- Implementation Re-review 3;
- the Reopen 4 controlled live canary result;
- the current installed original binaries, wrapper, plist, service, process,
  product paths, Terminal process state, resident SQLite, Candidate artifacts,
  and Git staging.

The Review performed no edit, stage, commit, installation, bootstrap, restart,
retry, Computer Use action, or P2A-W2 work.

## Assessment

Result-evidence coherence is `PASS`.

The evidence:

1. records the exact post-Review activation phrase;
2. correctly classifies both pre-install harness syntax failures as
   non-mutating and non-consuming;
3. records one successful Candidate bootstrap and allowance `0`;
4. truthfully records that Finder Computer Use opened the exact installed
   launcher but Computer Use could neither inspect nor operate Terminal;
5. does not substitute CLI/PTY evidence for the missing required TUI screens,
   empty-Team Enter/no-IPC proof, relaunch, or daemon restart;
6. correctly fails closed and preserves the P2A-W2 sequencing lock.

Independent read-only checks matched the rollback evidence:

- original installed `loom`, `loomd`, wrapper, plist, and SQLite hashes and
  modes match;
- retained Candidate hashes and modes match the reviewed Candidate;
- SQLite `integrity_check=ok`, Event count `1`, and canonical digest
  `6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05`;
- product run directory, sockets, socket locks, and `Loom.command` are absent;
- the original observer is loaded and running;
- Terminal is not running;
- Git staging is empty.

## Review boundary

This `PASS` validates only the fail-closed result evidence. It does not:

- turn the live canary into a product `PASS`;
- accept or install P2A-W1;
- authorize another canary, bootstrap, restart, or retry;
- authorize a new point Amendment;
- authorize commit, push, merge, release, or publication;
- unlock P2A-W2.

P2A-W1 remains `FAIL — ROLLED_BACK — HUMAN_REQUIRED`.
