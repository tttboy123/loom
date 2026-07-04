# Contributing to Loom

Thanks for your interest in contributing to **Loom** — a local-first,
multi-model Agent Runtime that turns software development into an observable,
verifiable, recoverable control system. This guide covers everything you need
to make a useful first contribution.

> **TL;DR** — Install Docker + Python 3.11+, run `./loom up`, then run a task
> with `./loom run "your task"`. Pick an issue tagged `good first issue` or
> `help wanted`, open a PR.

---

## Table of Contents

1. [Project structure](#project-structure)
2. [Setup](#setup)
3. [Running Loom locally](#running-loom-locally)
4. [Where to contribute](#where-to-contribute)
5. [Pull Request workflow](#pull-request-workflow)
6. [Coding conventions](#coding-conventions)
7. [Testing](#testing)
8. [Documentation](#documentation)
9. [Community & governance](#community--governance)
10. [Release process](#release-process)

---

## Project structure

| Path | What lives here |
| --- | --- |
| `loom` | One-shot CLI entry (`./loom up`, `./loom run`, …) |
| `devkit/` | Execution kernel: `rdloop.py`, role pipeline, gates, ledger |
| `app/` | Agent / Team registry (Agno adapter) |
| `console/` | Local web console (FastAPI) on `:8899` |
| `litellm/` | LiteLLM gateway config (model routing + fallback) |
| `ui/` | Optional Next.js chat UI (only loaded with `./loom up full`) |
| `docs/` | Architecture, design docs, RFCs |
| `tests/` | Unit + contract tests for `devkit/` and `gate/` |
| `.github/` | Issue / PR templates, CI workflows |

Before opening a PR, please skim:

- [`README.md`](README.md) — product positioning & quick start
- [`VISION.md`](VISION.md) — north star (you don't need to memorize this)
- [`CONSTITUTION.md`](CONSTITUTION.md) — rules every role must follow
- [`docs/architecture/loom-architecture.md`](docs/architecture/loom-architecture.md)
  — current architecture diagram

---

## Setup

### Prerequisites

- **macOS** (primary), **Linux** (tested in CI). Windows is **not supported**
  in this version.
- **Python ≥ 3.11** (we use stdlib `tomllib`, `asyncio.TaskGroup`)
- **Docker Desktop** or `colima` + the Docker CLI
- ~5 GB of free disk for LiteLLM, Postgres, Redis images

### First clone

```bash
git clone https://github.com/<your-org>/loom.git
cd loom

# (optional but recommended) create a venv
python3 -m venv .venv && source .venv/bin/activate

# Install dev dependencies
pip install -r requirements.txt
pip install -r requirements-crew.txt   # only if you need the crew-service
```

### Configuration

Copy `.env.example` to `.env` and fill in keys **only for the providers you
want to use**:

```bash
cp .env.example .env
# edit .env: ZHIPU_API_KEY, MINIMAX_API_KEY, DEEPSEEK_API_KEY are optional but
# recommended; LITELLM_MASTER_KEY is required (default works locally).
```

> **No keys?** No problem — `./loom up --mock` boots Loom with a fake model
> so you can explore the UI and CLI without spending a cent.

---

## Running Loom locally

```bash
./loom up           # lite core: console :8899 + gateway :4000 + cliproxy :8317
./loom doctor       # health check
./loom open         # open the console in your browser

# Run a real task (uses the default multi-stage pipeline)
./loom run "implement a tiny CLI that prints the current time"

# Run without API keys (mock mode)
./loom up --mock
./loom run "echo hello" --mock
```

Verify everything works:

```bash
./loom doctor
PYTHONPATH=. python3 -m unittest discover tests/ -v
```

---

## Where to contribute

Pick the area that matches your strength and the project's current needs:

| Area | What we need | Where to look |
| --- | --- | --- |
| **Core kernel** (`devkit/rdloop.py`) | bug fixes, performance, gate logic | issues labeled `kernel` |
| **Role pipelines** (`devkit/roles.py`, `loom.roles.toml`) | new roles, better prompts | `loom.roles.example.toml` |
| **Gates** (`devkit/gate/`) | new compliance / safety gates | `evaluate_final_gate`, `opensource.py` |
| **Console** (`console/`) | FastAPI server, OpenAPI contract | `console/server.py`, `console/test_openapi_contract.py` |
| **LiteLLM gateway** (`litellm/config.full.yaml`) | provider adapters, fallback tuning | `litellm/` |
| **Docs** (`docs/`, `*.md`) | architecture, RFCS, translations | `docs/`, `USAGE.zh.md` & `USAGE.en.md` |
| **Tests** (`tests/`) | fill coverage gaps, contract tests | `tests/` |

**Good first issues** are tagged `good first issue`. We try to keep a fresh
batch available each release.

---

## Pull Request workflow

1. **Fork & branch.** Branch off `main`:
   ```bash
   git checkout -b feat/short-slug
   ```
2. **Stay small.** One PR = one logical change. If your change spans
   kernel + docs + tests, that's fine; if it spans five subsystems, split.
3. **Write tests first.** Per `CONSTITUTION.md` §3 (TDD). Add a failing
   test under `tests/`, then implement. Existing tests should still pass.
4. **Run gates locally:**
   ```bash
   PYTHONPATH=. python3 -m unittest discover tests/ -v
   PYTHONPATH=. python3 -m devkit.gate.opensource .  # opensource readiness
   ```
5. **Fill in the PR template.** The CI bot will read it; missing sections
   block merge.
6. **Sign your commits.** DCO (`git commit -s`) or include a `Signed-off-by:`
   line. We may move to a CLA later; we'll update this doc when we do.
7. **One approval + green CI = merge.** Maintainers squash-merge by default.

> The `opensource_gate` is mandatory for the first release PR of a new
> contributor. After your first merge, the gate is automatic via CI.

---

## Coding conventions

- **Python style:** PEP 8 + type hints on every public function. We do not
  enforce a formatter in CI yet; please run `black` and `ruff` locally if
  you have them. (See [issue #N](#) for the formatter track.)
- **Naming:** `snake_case` for modules, `PascalCase` for classes, `UPPER_SNAKE`
  for constants. Carrier names (`loom-dev`, `loom-reviewer`) live in
  `litellm/config.full.yaml`.
- **Imports:** stdlib first, then third-party, then local. Avoid circular
  imports — if you hit one, the layering is probably wrong.
- **No silent failures.** If you catch an exception, log or re-raise. The
  constitution explicitly forbids "fake completion".
- **Tests live next to `tests/`**, not next to source. Test names start
  with `test_`, classes with `Test`.
- **Stdlib first.** Avoid adding new dependencies unless absolutely
  necessary; if you do, justify in the PR description.

---

## Testing

```bash
# All tests
PYTHONPATH=. python3 -m unittest discover tests/ -v

# Just the open-source gate
PYTHONPATH=. python3 -m unittest tests.test_gate_opensource -v

# Just the materialize / final gate
PYTHONPATH=. python3 -m unittest discover devkit/tests -v
```

We aim for ≥ 80% coverage on `devkit/` and `gate/`. UI has separate visual
tests (Playwright) — see `scripts/verify_console.py`.

---

## Documentation

- User docs (`README.md`, `USAGE.*.md`, `QUICKSTART.md`) explain **how**.
- Architecture docs (`docs/architecture/`) explain **why** and **how it works**.
- RFCs (`docs/architecture/loom-*.md`) explain **decisions and trade-offs**.

If you change behavior, update docs in the **same PR**. The README's
"Project Map" table should be regenerated for any new top-level module.

---

## Community & governance

- **Be kind.** See [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) (Contributor
  Covenant v2.1).
- **Discuss first** for non-trivial changes: open an
  [RFC issue](https://github.com/<your-org>/loom/issues/new?template=rfc.md)
  before sending the PR. Trivial fixes (typos, broken links, single-file
  bug fixes) can skip this step.
- **Stay on topic.** Loom is a **local-first Agent Runtime**, not a
  chat product, not a hosted service. If your idea is "turn Loom into a
  cloud SaaS", please fork and experiment — that's great — but we won't
  merge that direction here. See `CONSTITUTION.md` §7.

---

## Release process

1. Maintainer cuts a release branch (`release/vX.Y.Z`).
2. CI runs the **full test suite** + `opensource_gate` + `evaluate_final_gate`.
3. A "Release" issue is opened summarizing what's new and what's known broken.
4. Once green, tag `vX.Y.Z` → CI publishes Docker images → GitHub Release
   notes auto-generate from the changelog.
5. Minor versions (X.Y) add features; patch versions (X.Y.Z) fix bugs.

See [`docs/architecture/loom-opensource-agent-team.md`](docs/architecture/loom-opensource-agent-team.md)
§7 for the planned PR sequence that led to this document.

---

## Questions?

- Open an [issue](https://github.com/<your-org>/loom/issues)
- Or read the [architecture docs](docs/architecture/)

Thanks again for contributing — let's build Loom together.