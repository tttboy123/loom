# P5 Build 214 installed checkpoint

Status: `INSTALLED PARTIAL / P5-UX5 OPEN`

## Bundle identity

- version: `0.5.3`
- build: `214`
- identifier: `com.earendilworks.loom.local`
- signature: ad hoc; strict deep verification passed
- App SHA-256: `f8b1060e470b174c21a9596d3fbf3e5293de7c42f372a9a2941d41203e7da4ce`
- daemon SHA-256: `9795d053beb888ac02c4b4db8c2b43e54182ef8c2ef67286d9b5a0a298ffc23e`

## Verified locally

- `405` XCTest cases passed with two conditional skips.
- `20` Swift Testing contract cases passed.
- The App starts its bundled daemon as its managed child and exposes the
  owner-only private UDS without a separate user command.
- The first focusable screen appeared in about `1.4s` and truthfully reported
  `Starting local service` while recovery continued.
- The private daemon socket became available in about `17.3s`.
- Conversation -> linked Mission -> RoundTable stayed in one workspace.
- The installed RoundTable showed two Agent outputs and each frozen Harness,
  Provider Account and Model.
- Safe inline Markdown removed raw formatting markers and active links.
- The summary showed `Round 1 · 2 Agent results` instead of a contradictory
  zero-update count.

Screenshot: `P5-BUILD214-ROUNDTABLE-CONTEXT.jpeg`

## Still open

- reduce or explicitly accept the approximately 17 second cold daemon
  construction path;
- restore the exact linked governance context after restart;
- installed compact-window, keyboard, VoiceOver and large-text matrix;
- installed blocked/failure intervention path;
- a fresh real Mission and RoundTable workflow without terminal assistance.

No Provider credential was modified and no paid or remote model call was made
for this checkpoint.
