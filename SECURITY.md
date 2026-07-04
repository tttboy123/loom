# Security Policy

> Loom is a **local-first Agent Runtime**. Most of the code runs on **your**
> machine, not ours — which means your security boundary is mostly your own
> environment. This document covers what we (the Loom maintainers) do, what
> you should do, and where the known risks are.

---

## Supported Versions

| Version | Supported |
| ------- | --------- |
| `main` branch (latest) | ✅ |
| Latest tagged release   | ✅ |
| Older minor versions    | ❌ best effort |

We backport security fixes to the latest minor only. Please upgrade before
filing a vulnerability.

---

## Reporting a Vulnerability

**Please do not file public GitHub issues for security problems.**

Email: **[loom-security@your-org.example](mailto:loom-security@your-org.example)**
(replace with the real address once the org is set up). Encrypt sensitive
details — we'll publish a PGP key in `docs/security/loom-security.asc` when
the org is up.

What to include:

- A short description of the issue and its impact
- Steps to reproduce (PoC code / config is great)
- Affected versions / commits
- Your name / handle for the acknowledgements list (optional)

Response timeline:

- **48h acknowledgement** (working days)
- **7-day triage** with severity rating and a fix plan
- **30-day fix target** for High / Critical
- Coordinated disclosure once a fix is shipped

---

## Threat Model

Loom runs **on your machine** and orchestrates:

1. Local containers (Docker / colima)
2. Local filesystem (writes to `devkit/runs/`, `devkit/MEMORY.md`, etc.)
3. Network calls to **LLM providers** you configured (OpenAI, Anthropic, GLM,
   MiniMax, DeepSeek, local models, **subscription proxies**)
4. Optional **`cliproxy` / `CLIProxyAPI`** — a small proxy that wraps
   Claude / ChatGPT subscription sessions as an OpenAI-compatible local API.

### What Loom can do

- Read / write files **inside the workspace directory** and a few declared
  state directories (`devkit/runs/`, `devkit/.cache/`)
- Run shell commands via the configured `executor` (chat / hermes / openclaw / codex)
- Make outbound HTTPS to model providers

### What Loom will **never** do without explicit opt-in

- ❌ Touch files outside the workspace
- ❌ Push code or open PRs (default L1 / report-only mode)
- ❌ Spend money — `L1 report-only` is the default; `L2 autonomous` requires
  `--apply` flags and human confirmation
- ❌ Send credentials anywhere except the configured providers

### Subscription proxies (`cliproxy` / `CLIProxyAPI`)

This is the **biggest security caveat** in the project and you should
understand it before turning it on:

> **Using subscription proxies to access Claude / ChatGPT / Codex may violate
> the provider's Terms of Service.** Loom does not endorse this usage; it
> simply provides a local proxy so people who already pay for those
> subscriptions can route them through Loom without sharing API keys.

What this means for you:

- The provider may **rate-limit**, **suspend**, or **ban** the account if
  they detect proxy use.
- Loom ships this proxy because it is genuinely useful for **personal /
  research / evaluation** purposes. We do **not** recommend it for
  commercial / production use.
- The proxy stores OAuth tokens **on your machine** (default:
  `~/.cli-proxy-api/`). Loom does **not** transmit these tokens anywhere.
- If you mount the proxy's token directory into the Docker container (see
  `docker-compose.yml`), the token is **only readable inside the Loom
  network namespace**. Do not expose `:8317` to the public internet.
- To disable the proxy entirely, comment out the `cliproxy` service in
  `docker-compose.yml` and remove `codex-sub` / `claude-code-sub` from
  `litellm/config.full.yaml`.

### Executors

Loom can invoke external coding agents as **executors** (`hermes`,
`openclaw`, `codex`, `opencode`). Each executor inherits the same boundary:

- It can read / write files in the workspace
- It can run shell commands
- It cannot (by default) reach the network beyond what Loom itself can reach

If you wire a custom executor into Loom, **you are responsible** for its
security properties. Pin its version. Don't run untrusted executor code.

---

## Secrets Management

- **Never commit `.env`.** It's gitignored for a reason. Use `.env.example`
  for templates.
- **Never paste API keys into issues / PRs / chat.** Use a secret manager
  or environment variables.
- **`LITELLM_MASTER_KEY`** is the single key that talks to the local gateway.
  Default `sk-change-me` is fine for solo local dev; **change it** if you
  expose any port.

---

## Build / Supply-chain

- We pin Python dependencies in `requirements*.txt` and aim to move to a
  lockfile (`pip-tools` / `uv`) in the next minor.
- The console Docker image is built from `Dockerfile.console` (multi-stage).
- We do **not** currently sign release artifacts; we plan to add Sigstore /
  cosign in the `release/v0.2` cycle. Track progress in
  `docs/architecture/loom-opensource-agent-team.md`.

---

## Known Issues

We track open security issues in the GitHub issue tracker with the
`security` label. Major items:

- Subscription proxy ToS risk (see above) — **accepted risk**, documented.
- Worktree isolation in `autopilot` mode — partial; see `applylock/` docs.

---

## Acknowledgements

We follow [GitHub's security advisories][gh-sec] workflow and are inspired by
the disclosure policies of [OpenSSF][openssf] and [Rust][rust-sec].

[gh-sec]: https://docs.github.com/en/code-security/security-advisories
[openssf]: https://openssf.org/
[rust-sec]: https://www.rust-lang.org/policies/security

---

Thanks for helping keep Loom and its users safe.