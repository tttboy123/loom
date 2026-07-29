# P2A-W1 Native App Launchability Diagnosis

**Date**: `2026-07-28`
**Status**: `ROOT CAUSE CONFIRMED — NO LIVE RETRY`
**Method**: post-canary immutable crash-report inspection and isolated linker
experiment

## Preserved live evidence

The two Computer Use `get_app_state` calls from the consumed native-window
canary correspond exactly to two macOS crash reports:

| Report | SHA-256 | Mode / uid |
|---|---|---|
| `LoomLocalApp-2026-07-28-213614.ips` | `f284754aeca95cbceaf10070b2dd7465e4646ce3ab39a4e484c6ee04aff5e6c0` | `0600` / `501` |
| `LoomLocalApp-2026-07-28-213628.ips` | `4804009d0fda5b9f29706be8a0373156706af9c845af5dd995dedc8054810f54` | `0600` / `501` |

Bounded JSON inspection of both reports independently reproduced:

```text
process: LoomLocalApp
bundle identifier: com.earendilworks.loom.local
version: 0.2.0 (1)
exception: EXC_CRASH / SIGABRT
termination namespace: DYLD
termination reason: missing LC_UUID load command
```

No report was copied into the repository. No user path, environment value,
credential, prompt, model output, raw identifier, or unbounded crash payload is
recorded here.

## Source and test contradiction

The reviewed builder passes `-no_uuid` to both Swift build invocations. The
reviewed build fixture then requires `dwarfdump --uuid` to return nothing.

That pair made the Candidate byte-reproducible by deliberately removing the
Mach-O `LC_UUID` load command, but current macOS dyld rejects the application
before SwiftUI entrypoint execution. No native process survived long enough to
register an Accessibility window. The Computer Use timeouts were therefore a
consequence of an application launch defect, not an Accessibility navigation
defect.

The installed `ld` manual confirms:

- `-no_uuid` omits the `LC_UUID` load command;
- the default linker UUID is derived from output-content hash;
- `-reproducible` ignores selected non-semantic input properties to produce
  reproducible output;
- `-random_uuid` is explicitly unsuitable for reproducible builds.

## Isolated hypothesis check

Without changing repository files or launching the installed app, two complete
Swift package-reset builds were linked with:

```text
-Xlinker -reproducible
-Xlinker -no_adhoc_codesign
```

Both outputs were stripped with `strip -S`. Before final signing they were
byte-identical:

```text
unsigned SHA-256
a75a648079b836d30bd325dd6e90be8a39b96b1b28f84b48ca27dc22c2fb353c
```

Both carried the same non-empty arm64 UUID:

```text
CE91F84E-4333-35DB-B493-88FADCBC6EC1
```

After placing each executable under the same basename `LoomLocalApp` and
performing one ad-hoc signature with timestamps disabled, both signed outputs
were also byte-identical and passed strict signature verification:

```text
signed SHA-256
6e1318a82cdfb2f4b21b5e5f7e9a7d20e96d736d054dc45068f53be76842d8d4
```

The first experimental signing used distinct basenames `first` and `second`;
its CodeDirectory identifiers correctly differed. Removing those signatures
and signing the identical payloads under the production basename proved that
the difference was test construction, not linker nondeterminism.

## Conclusion

The defect is fully localized to the native packaging reproducibility
configuration and its inverted UUID assertion. No SwiftUI, IPC, daemon,
Journal, Projection, Provider, Runtime, or authority change is supported by
the evidence.

The consumed canary remains failed and no additional live invocation is
authorized. The next legal step is one reviewed, same-W1 launchability and
reproducible-identity closure contract.
