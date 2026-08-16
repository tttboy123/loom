# Final Live Gate Source Provenance Amendment

Status: CONTRACT REVIEW, ATOMIC INSTALLATION, AND PRE-LIVE PREFLIGHT `PASS`

- Amendment: `PHASE1-FINAL-LIVE-SOURCE-PROVENANCE-1`
- Risk: `HIGH`
- Baseline: `7829423`
- Date: `2026-07-27`
- Depends on: Final Live Gate Compatibility Amendment Implementation Review 5
  `PASS` and local compatibility commit `7829423`
- Authority: explicit user acceptance and authorization on `2026-07-27`
- Product code ownership: none
- Live execution authorization: closed until fresh Contract Review `PASS`

## Exact reason

The frozen model identity is:

```text
repository = Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF
revision   = 2ab9f8f42af02fc212effaef7c4850c885e965f4
file       = qwen2.5-coder-1.5b-instruct-q4_k_m.gguf
size       = 1117320768 bytes
sha256     = cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046
format     = GGUF v3
```

Direct access to `huggingface.co:443` failed through the exact HTTPS revision
URL, official `hf.co` Git smart-HTTP, and Chrome. A user-supplied staging file
was instead transferred through the user-declared URL:

```text
https://hf-mirror.com/Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf
```

The URL uses `main`, not the frozen revision. This amendment does not claim
that the mirror proves repository revision ancestry or that `hf-mirror.com` is
an official or trusted Hugging Face endpoint. The user explicitly accepts that
transport-provenance deviation.

## Frozen trust decision

`hf-mirror.com` is treated only as an untrusted byte transport. It gains no
Runtime, Provider, repository, revision, signature, policy, or execution
authority.

The staged artifact may be accepted for this one controlled local gate only
because its complete bytes independently match the already-frozen SHA-256 and
size and its header is the expected GGUF v3 form. The frozen digest, not the
mirror URL or filename, is the content-identity authority.

This exception:

- does not add the mirror to product configuration, source discovery, Runtime
  download, fallback, retry, or update logic;
- does not authorize future mirror downloads or another model/revision/file;
- does not change the llama.cpp, Pi, Bridge, Supervisor, Grant, Evidence,
  Journal, Projection, or final-signoff boundaries;
- does not assert publisher authenticity beyond the previously reviewed frozen
  source lock;
- does not remove `com.apple.provenance` or any other xattr;
- does not permit credential use, remote Provider traffic, paid work, resident
  service, or production activation; and
- does not authorize a second live attempt after a canary failure.

## Exact ownership

This governance-only Candidate owns:

- `.loom-evidence/phase1-final-live-gate/source-provenance-amendment.md`
- the source-provenance status and appendix in
  `.loom-evidence/phase1-final-live-gate/contract.md`
- the model transport-provenance object and status in
  `.loom-evidence/phase1-final-live-gate/source-lock.json`
- the new review entry in
  `.loom-evidence/phase1-final-live-gate/contract-review.md`
- the private, non-repository materialization status file

It owns no Go file, test, ADR, dependency, CLI, daemon, credential, model
content, or accepted authority. Any product-code change stops
`HUMAN_REQUIRED`.

## Frozen pre-install evidence

The Controller must independently verify the incoming leaf before any rename:

1. absolute clean path below the selected private root;
2. current-user-owned regular non-symlink file;
3. exact mode `0600`;
4. exact size `1117320768`;
5. first eight bytes encode `GGUF` and little-endian version `3`;
6. SHA-256 exactly
   `cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046`;
7. no final model leaf already exists; and
8. resolved path remains below the exact private root.

The existing `com.apple.provenance` xattr may remain. It is neither trusted
evidence nor a rejection condition; the existing shared inspector's regular
file, owner, mode, identity, containment, and digest checks remain unchanged.

## Atomic installation

Only after a fresh independent Contract Reviewer returns `PASS` may the
Controller:

1. create an exact `0700` model directory below the existing `0700` private
   root;
2. repeat all incoming-file checks;
3. atomically rename the verified incoming leaf on the same filesystem to the
   exact final model path;
4. force the final leaf to exact mode `0600`;
5. repeat size, GGUF version, SHA-256, owner, mode, non-symlink, lexical and
   resolved containment checks;
6. call the production `InspectPiLocalModelServerBinding` path through the
   existing pre-live manifest harness; and
7. require the inspector-returned model and executable digests to match the
   frozen source lock.

Copying across filesystems, downloading during execution, following a symlink,
overwriting an existing final leaf, accepting a digest/size/header mismatch,
or losing the exact owner/mode stops and quarantines the Candidate. No live
process may start in those cases.

## Gate order after review

Fresh Contract Review `PASS` opens only atomic installation and read-only
preflight. The existing order remains:

```text
Contract Review PASS
→ atomic install
→ final digest/owner/mode/containment checks
→ shared inspector + sanitized resolved pre-live manifest
→ installed Pi offline metadata discovery
→ one controlled local llama.cpp/Pi RPC live canary
→ sanitized live evidence
→ fresh evidence/scope review
→ evidence-only local commit if reviewed
→ READY_FOR_FINAL_USER_SIGNOFF
```

The live canary remains prohibited before every preceding gate passes. A live
failure stops immediately with no retry, fallback, alternate source, alternate
model, or second attempt.

## Post-review execution evidence

After Contract Review `PASS`, the Controller:

1. repeated the incoming regular-file, non-symlink, uid `501`, mode `0600`,
   exact-size, GGUF v3, SHA-256, containment, and same-device checks;
2. created the final model directory at exact mode `0700`;
3. confirmed that the final leaf did not exist;
4. atomically renamed the incoming leaf on device `16777234`;
5. forced the final leaf to mode `0600` and repeated every content, identity,
   owner, mode, and containment check;
6. preserved `com.apple.provenance` as neither trusted nor rejected metadata;
7. invoked the existing sanitized pre-live manifest harness through a private
   temporary Go overlay that changed no repository/product source;
8. obtained shared-inspector-validated Pi, llama.cpp, and model digests in an
   exact `0600` resolved manifest without private paths or mirror details; and
9. removed the temporary overlay before any live process.

The installed model remained exactly `1117320768` bytes with SHA-256
`cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046`.
The resolved manifest binds llama-server SHA-256
`a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b`
and installed Pi SHA-256
`af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca`.

A final clean-environment raw Pi metadata process check exited zero for both
version and offline model listing. Version was exact `0.82.1`; the no-model
diagnostic was exactly three lines with the frozen first line, ordered
`providers.md` and `models.md` basenames, and one common absolute parent. This
command-level shape check did not validate leading indentation against the
production Loom parser. The later canary proved that both documentation-path
lines begin with two ASCII spaces and that production discovery rejects them.
No private diagnostic path was copied into repository evidence.

No local model server, Pi RPC execution, or live canary ran during these
preflight steps. They permitted the one controlled canary to begin; its later
result is the `FAIL — HUMAN_REQUIRED — NO RETRY` recorded in
`live-canary.md`. This provenance amendment itself remains `PASS`.

VERDICT: PASS
