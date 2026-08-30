# P7-HT2 installed Build 323 failed privacy verification

Status: `FAILED / PRESERVED HISTORICAL EVIDENCE / SUPERSEDED BY BUILD 324`

Date: 2026-08-31

Build 323 is preserved as a failed installed acceptance. It corrected Codex
native-auth protocol semantics and completed the real Codex, OpenCode, Pi and
Loom Native Tool sweep, governance decisions, expiry and managed restart. The
run reached the final privacy gate after 387.76 seconds and then failed with:

```text
installed privacy directory is not private
```

Exact Build 323 identities were:

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `1701db458e30e937ec2b6ec55931dec9cc3e9caeead6687df421012ad5423267` |
| `Contents/Library/Helpers/loomd` | `4622db68a42f41c7c841f42aa64eefb3b04ba8c476b041cc8a32c85fa142bb59` |
| `Contents/Info.plist` | `a178bb00b2c820a56b56913b484fe40cf588c4a7b4cc08f25071639f5e75aca0` |

The installed scratch tree contained Codex temporary directories and an
OpenCode child directory with mode `0755`, plus OpenCode temporary dynamic
library files with mode `0644`. Persistent private roots were `0700`, but the
child CLIs had received those roots directly as `TMPDIR` and created descendants
using the process default umask. The failure was therefore a real installed
privacy defect, not a test-only expectation mismatch.

Build 324 fixes the defect by validating and migrating existing scratch trees,
using a canonical owner-only per-call temporary directory and removing it on
all exit paths. The complete Build 324 matrix and post-restart mode checks pass.
Build 323 remains the transactional `.previous` bundle; this record is not
rewritten as acceptance.
