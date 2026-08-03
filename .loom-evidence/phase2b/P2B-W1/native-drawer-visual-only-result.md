# P2B-W1 Native Drawer Visual-only Result

Date: 2026-08-03

Verdict: `PASS`

This inspection was a replacement visual-only check of the final reviewed
Swift presentation bytes against bounded, read-only data derived from retained
`canary-001`. It did not rerun the authority canary, open a product socket,
dispatch work, write authority state, use a Provider, load credentials or make
a network request.

## Source binding

- original consumed-canary source lock:
  `411593c724dfc78424dbaf99c4936b2367b92dd65b402b6841220d5e738ef97c`;
- post-canary presentation-delta lock:
  `8e19132d75d4ae10da25f07d11f1e211a4b12ee40eee764961adf610ea75e970`;
- `MissionWorkbench.swift`:
  `4f6edec07e42f9fdad802dcff914721f890dfd54bc92e80f6d707e0e470f5e35`;
- `LocalProductExperienceViewTests.swift`:
  `1b2de1e6b5c6cb41d6f38279ac94e37a10c7393432425c9142b1a0a0971ef95b`.

The temporary native host was compiled against those final Swift bytes:

- executable SHA-256:
  `ef3936b7e1fd9a0450b00df2a048a8ab4a150ffb04bf111ac5d08efcd666db9f`;
- `Info.plist` SHA-256:
  `bdc195baac1f296fa613fc2b2611b73fa4b4bb0fc166b6838d6bad491284d11f`.

## Visible result

Computer Use opened the native window and its Side-task Drawer. The screenshot
and accessibility transcript both show:

- title `Recover admitted child`;
- `Purpose · Diagnosis`;
- `Decision Required · Decision Required`;
- summary and finding;
- `Risk · Medium · Evidence 2 · Artifacts 1`;
- exact Evidence and Artifact references;
- `Usage · Not observed`;
- `Uncertainty · None recorded`;
- `Scope · No parent scope expansion`;
- `Next action · Choose an explicit parent decision`;
- the decision menu and availability state.

Evidence:

- screenshot, mode `0600`, SHA-256
  `39fbb354bff36997283a0cd96a3bfc2a1092a633accaf279fc5580ef136832e6`:
  `/Users/lune/Library/Application Support/Loom/phase2b-visual/side-task-drawer-002/native-side-task-drawer-pass.png`;
- accessibility transcript, mode `0600`, SHA-256
  `54c5e888507fff82a2a30bd4f6124ee8b9a8a3bac851223fd9bc02a1317f5f08`:
  `/Users/lune/Library/Application Support/Loom/phase2b-visual/side-task-drawer-002/native-side-task-drawer-pass-ax.txt`.

The host intentionally had no product write client, so this evidence proves the
final SwiftUI presentation, while the consumed source-locked vertical canary
remains the authority and real Go IPC-to-Swift proof.

## Retained authority stability and cleanup

- SQLite SHA-256 remained
  `f47e4095ad8da5799ce07ca6ceb8fe0ac29e13dc491b5c3ab3854adc996d037a`;
- manifest SHA-256 remained
  `ece9b16eeab8738013f04d36db96d3b5011ddba54e5d91ef5af3f0cb1bbeede2`;
- original source-lock SHA-256 remained
  `411593c724dfc78424dbaf99c4936b2367b92dd65b402b6841220d5e738ef97c`;
- both bounded visual host processes were terminated after capture;
- no `P2BVisualHost` process remains.
