# Final Live Gate Pi RPC Schema and Fresh Attempt Isolation Contract Review

- Amendment: `PHASE1-FINAL-LIVE-PI-RPC-FRESH-ATTEMPT-1`
- Baseline: `b34d8da635c6037c6f3658d580c1dc75bf541f63`
- Review date: `2026-07-27`
- Reviewer: fresh independent read-only Reviewer

## Review 1

### Finding

`Important`: the frozen settings publication language required atomic
materialization and said overwritten settings must fail, but it did not
explicitly prohibit the existing local temp-file-plus-`os.Rename` pattern from
replacing an existing destination.

The Reviewer required the contract to freeze:

- direct creation at the final `settings.json` leaf with
  `O_WRONLY|O_CREATE|O_EXCL`;
- failure when any object already occupies that leaf;
- complete write, `fsync`, close, and exact regular/non-symlink/`0600`
  revalidation before process start; and
- no rename, truncation, unlink, replacement, or reuse of an existing final
  path.

All other reviewed boundaries had no blocking finding:

- installed Pi `0.82.1` supports the exact array-only diagnosis;
- default Agent retry, Provider retry binding, compaction retry, and project
  settings precedence are correctly covered by the intended private settings
  plus `--no-approve`;
- fresh attempt root, exclusive `0600` SQLite, independent manifest, prior
  evidence preservation, and one additional canary are within authority; and
- the owned files and Phase 1 exclusions are bounded.

The Reviewer performed no edit, live Runtime/model/canary execution, network or
credential access, staging, or commit.

## Contract Repair 1

The contract now freezes direct final-leaf `O_CREATE|O_EXCL` creation and
explicitly forbids overwrite-capable rename or any mutation of a pre-existing
settings object. It also requires a rejection test proving that a pre-existing
regular file, symlink, directory, or other object prevents process start and is
not changed.

Fresh independent Contract Repair Review 2 is required before mandatory RED.

## Contract Repair Review 2

Fresh independent read-only Reviewer 2B returned `PASS` with no blocking
findings.

The Reviewer confirmed:

- final-leaf settings creation is unambiguously
  `O_WRONLY|O_CREATE|O_EXCL`/`0600`, with write, `fsync`, close, and exact
  revalidation before `command.Start`;
- no rename, truncate, unlink, replace, or other mutation of an existing
  settings object is allowed;
- installed Pi `0.82.1` constructs and forwards the frozen exact array user
  message through `message_start`, `message_end`, and `agent_end.messages`;
- the current string-only Loom parser makes the mandatory RED genuine;
- the exact private settings plus `--no-approve` close Agent retry, Provider
  retry, compaction recovery, and project-setting override paths;
- fresh `0700` attempt root, exclusive pre-`sql.Open` `0600` SQLite,
  independent manifest, and immutable prior evidence close the observed
  harness gaps; and
- ownership remains limited to the exact three Go files plus new amendment
  evidence, with only one post-commit additional canary and no Bridge,
  authority, credential, or Phase 2 expansion.

Reviewer 2B made no edit, stage, commit, live execution, network request, or
credential access.

Mandatory RED may proceed.

VERDICT: PASS
