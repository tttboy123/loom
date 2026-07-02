# Loom · Usage Guide

> 🌐 中文版：[USAGE.zh.md](USAGE.zh.md)
> A locally-deployed, multi-agent / multi-vendor R&D toolkit. Lightweight, runs on your machine, low barrier to start.

---

## What is this

Loom lets **different models handle different R&D roles** — product, orchestration, development, testing, review — routed through one unified gateway and observed from one global console.

```
You (browser) →  Global Console :8899  →  LiteLLM Gateway :4000  →  5 model backends
                 (run / view / observe)    (unified · auto-fallback)   Claude sub / Codex sub / GLM / DeepSeek / MiniMax
```

---

## Start in 30 seconds (no experience needed)

1. **Start** (easiest: the one-command `loom` — auto-starts colima if the Docker engine is down):
   ```bash
   cd agent-platform
   ./loom up            # lite core (console + gateway, ≈730MB)
   ./loom up full       # full stack (+ chat UI + orchestration, ≈980MB)
   ./loom doctor        # health: services + endpoints
   # or use raw compose:
   docker compose up -d                 # = lite
   docker compose --profile full up -d  # = full
   ```
2. **Open the console**: visit **http://localhost:8899**
3. In **"Run a task"**, describe a dev task in one sentence → click **Run ▶**
4. After a few dozen seconds, click the row in **"Runs"** → read the five artifacts (product judgment / plan / code / tests / review)

> No API key needed for the first run: "subscription substitute" is on by default, so it runs on your Claude / ChatGPT subscriptions (review stays cross-vendor).

---

## I want to… (one table)

| I want to | How | Where |
|---|---|---|
| Run a full R&D pipeline | Type a task in the console, click Run | http://localhost:8899 |
| See each run's artifacts | Click any row under "Runs" | same / files in `devkit/runs/<timestamp>/` |
| Chat with a single role | Pick a role agent and chat | Agent UI http://localhost:3000 |
| See usage / cost | Console "Gateway Usage" panel | same / LiteLLM http://localhost:4000/ui |
| Change a role's model | Edit one config file | see "Swap a model" below |

---

## Roles = whatever you define (not hardcoded)

A ready-to-use **default** 5-role pipeline:

| Role | Default model | Job |
|---|---|---|
| Product | GPT-5.4 | Turn requirements into product judgment & tradeoffs |
| Orchestration | GPT-5.4 | Break down tasks, plan, dispatch |
| Development | MiniMax-M3 | TDD coding |
| Testing | MiniMax-M3 | Verification, eval |
| Review | GPT-5.4 | Independent review; catch "looks-done-but-isn't" |

> Core idea: **the reviewer uses a different vendor than the developer** — different blind spots, so it can actually catch real defects.

But this pipeline is **not hardcoded** — each role's name, order, model, and system prompt live in one file you own:

```bash
devkit roles init        # scaffold an editable loom.roles.toml from defaults (see loom.roles.example.toml)
devkit roles list        # show the active pipeline
devkit roles path        # show which file is active (none = built-in defaults)
./loom roles             # = devkit roles list
```

Each `[[stages]]` in `loom.roles.toml` is one role (one file fully describes "this agent: which model, which harness, how many tokens, what job"):
```toml
[[stages]]
key = "spec"
role = "product"
title = "Requirements breakdown"
carrier = "deepseek"     # which model ← see below
executor = "chat"        # executor: chat (default) | hermes | openclaw (tool-using agent)
max_tokens = 600         # optional: per-stage token cap (omit = run-level default)
system = """
You are the product role. Break the request into clear acceptance criteria. No code.
"""
```
> Runtime `--executor stage=...` / `--max-tokens` still override what's in the file.

**Lowest onboarding cost**: point `carrier` straight at a backend the gateway already knows (`deepseek` / `glm` / `minimax` / `claude-code-sub` / `codex-sub`) → **zero extra gateway config, no restart**. Want the "swap vendor with one click in the console" experience? Use a `loom-*` semantic carrier instead. Lookup order: `$LOOM_ROLES` → cwd → project root → `devkit/` → `~/.loom/`.

**Edit roles in the console too**: at http://localhost:8899, under "roles → carrier → backend" there's an **✎ Edit role pipeline** card — change fields, add/delete stages, move up/down, save. The console writes `devkit/loom.roles.toml`, **the same file `devkit run`/`devkit roles` use** (CLI and UI never drift); "restore defaults" deletes it to fall back to the built-in defaults.

---

## Configuration (optional, skippable)

- **Just the 3 APIs (GLM / DeepSeek / MiniMax)**: put the keys into the three lines of `.env`, then `docker compose restart litellm`.
- **Use subscriptions (Claude / ChatGPT)**: run the browser login once on the host:
  ```bash
  ./cli-proxy-api --claude-login     # log into your Claude
  ./cli-proxy-api --codex-login      # log into your ChatGPT
  ./cli-proxy-api --config ./config.yaml   # start, listens on :8317
  ```
- **Swap a role's model**: edit the matching `loom-*` block in `litellm/config.full.yaml` (just change `model`), then `docker compose restart litellm`. **No pipeline code changes.**

> ⚠️ Using Claude / ChatGPT subscriptions as an API may violate their Terms of Service and risks rate-limiting / bans. Decide for yourself.

---

## Developer usage (CLI)

The console is just a shell; the core is the `devkit` CLI — pure standard library, zero dependencies:

```bash
cd agent-platform
python3 -m devkit "Implement a small feature with tests and review" # all 5 stages
python3 -m devkit "..." --stages brainstorm,plan,implement   # only some stages
python3 -m devkit "..." --carrier review=codex-sub           # temporarily swap a stage's carrier
python3 -m devkit "..." --executor implement=hermes          # run a stage via the hermes agent (sandboxed)
python3 -m devkit "..." --budget 0.05                        # soft budget: stop remaining stages + NO-GO if spend exceeds $0.05
python3 -m devkit "..." --no-compact                         # disable context compaction (on by default)
```

**Iterate loop (inspired by Anthropic's "Planner→Generator→Evaluator" long-running harness)**: by default Loom is a one-pass pipeline; add `--iterate N` to make it a **loop** — when the evaluator (tests/Eval/review) returns NO-GO, the failure detail (Golden `want=` values + review critique) is **fed back to the generator** to fix, then re-tested and re-judged, up to N rounds until it passes:
```bash
python3 -m devkit "implement X, give x.py" --golden cases.json --iterate 3
```
It reports **converged/not + iteration cost** (echoing the article's "cost per accepted change" — low accept rate = burning money). The console run form has an "iterate N" input too. If round 0 already passes, it does 0 rounds (no waste).

`--contract N` (Sprint Contract) has the evaluator pre-write ~N machine-checkable golden cases before coding; add `--contract-rounds N` to let the **builder** then negotiate N rounds (tighten/fix the cases) under an **anti-weakening floor** (final case count and `raises`-case count can't drop below the evaluator's; evaluator has final say). `devkit feature --commit` makes **one git commit per green feature** (in-place, idempotent `git init`, no push, fail-open).

**Two subcommands (same code as the console, usable from the CLI)**:
```bash
# @ask-model: ask one or more carriers a quick question (parallel compare for multiple), no full loop
python3 -m devkit ask "explain dependency inversion in one line" --models deepseek,glm,loom-reviewer
./loom ask "..." --models deepseek,glm         # loom shortcut

# diff: compare a run's build/ artifacts against another (auto-picks the previous run with a build)
python3 -m devkit diff 20260623-1730 [--against 20260623-1700]
./loom diff 20260623-1730                       # loom shortcut
```

**Free-quota maximizer 🐑 + model scoring ⭐** (the console has matching panels):
```bash
devkit quota         # per backend: used$ / free allowance / remaining, recommends which to use first
devkit scores        # actual usage (success/latency/cost) + your 👍/👎 + official benchmarks → composite
devkit rate deepseek up --note "cheap and good enough"   # record real experience, feeds the composite
./loom quota | ./loom scores | ./loom rate <backend> up|down
```
- **Quota maximizer**: `cp loom.quota.example.toml loom.quota.toml` to declare each backend's free allowance / subscription; Loom computes remaining from **real spend** (via this gateway) and ranks by "wool value" (subscription > most free remaining > paid). *Only counts spend routed through this gateway — a forward-looking guide, not billing-accurate.*
- **Model scoring**: actual-usage scores come entirely from Loom's real logs; official benchmark scores are **not fabricated** — `cp loom.scores.example.toml loom.scores.toml` and fill them yourself (0-100, cite sources). Composite = actual 0.5 + user 0.2 + official 0.3 (renormalized over present components).

**Context compaction (compact pointer)**: on by default — when a stage's artifact is long, a cheap model (default `deepseek`, override with `--compact-model`) summarizes it into key points before feeding downstream, saving tokens while keeping the signal (conclusions/interfaces/constraints/risks). Each artifact is compacted once; the cost is counted in the run total.

**Soft budget guardrail**: `--budget $X` (the console run form also has a "budget$" input) — if cumulative spend exceeds that dollar amount, the remaining stages stop and the run ends NO-GO, giving automated batches a cost ceiling.

**Executors (embed multiple agent harnesses at once)**: each stage can use `chat` (default flat chat) / `hermes` (Nous Hermes agent, tool-using) / `openclaw` (open-source equivalent). Mix them in one run: `--executor implement=hermes --executor review=openclaw`. Agentic executors run in an isolated `runs/<ts>/sandbox-<stage>/` dir. Run `./loom doctor` to see which executors are available.

**Dev-as-agent closed loop (runnable + tested changes)**: the `implement` output is **materialized into files → unittest is run in the sandbox**; failing tests = NO-GO. Add `--apply DIR` to copy the artifacts to DIR **only when tests pass** (**apply is a human gate, off by default**):
```bash
python3 -m devkit "implement X with x.py + test_x.py" --executor implement=hermes --apply ./out
```

Artifacts land in `devkit/runs/<timestamp>/`: `00-task` + per-stage `.md` + `run-log.md`; the master ledger is `devkit/RUNS.md`.

**Add a new role**: add a row to `app/roles.py` + a matching `loom-*` carrier in `litellm/config.full.yaml`, then rebuild. See [LOOM-ROLES.md](LOOM-ROLES.md).

**Run tests**: `./loom test` (pure standard library, no live gateway, CI-friendly) — runs the ledger contract test + new-feature unit tests (compaction fallback / parallel-ask aggregation / diff status).

---

## Start / stop / troubleshooting

| Situation | Do |
|---|---|
| Stop (keep data) | `docker compose down` |
| After a reboot | `colima start` (if using colima), then `docker compose up -d` |
| Console won't open | check `docker compose ps`, `docker compose logs console` |
| Tester/Reviewer 401 | the matching API key or subscription proxy is unavailable; fill `.env` then `docker compose restart litellm`, or run `./loom login` to refresh subscriptions |
| Port in use (3000/4000/8899…) | change the port mapping in `docker-compose.yml` |

---

## Lightweight notes

- **Lite by default**: `docker compose up -d` starts only the lite core (console + gateway + DB + subscription proxy), **≈730 MiB**. Running tasks / viewing artifacts / usage all work; the chat UI and orchestrator move into `--profile full` (**≈980 MiB**).
- The console itself is tiny (pure Python stdlib, no third-party deps, ~17 MiB).
- The subscription proxy (cliproxy) is now containerized with `restart: always`, so it won't "drop" anymore.
- Everything runs locally; data never leaves your machine. Full optimization roadmap: [ROADMAP.md](ROADMAP.md).

---

## Quick links

| Entry | URL |
|---|---|
| Global Console (hub) | http://localhost:8899 |
| Chat UI (Agent UI) | http://localhost:3000 |
| Gateway dashboard (LiteLLM) | http://localhost:4000/ui |
| AgentOS API docs | http://localhost:8000/docs |
