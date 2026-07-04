---
name: Bug report
about: Something is broken or behaves unexpectedly
title: "[bug] "
labels: bug, needs-triage
---

## What happened?

<!-- Clear, factual description. -->

## What did you expect?

## Minimal reproduction

```bash
# Commands / config / inputs that reliably trigger the bug
./loom run "..."
```

## Environment

- Loom version / commit: (run `./loom doctor` and paste the version line)
- OS: (macOS 14.x / Ubuntu 22.04 / …)
- Docker: (Docker Desktop 4.x / colima 0.x)
- Mode: (real keys / `--mock`)
- Python: `python3 --version`

## Logs / screenshots

<!-- Paste relevant output from `./loom doctor`, `docker compose logs`,
or the run directory `devkit/runs/<run-id>/run-log.md`. -->

## Possible cause (optional)

<!-- If you have a hunch, share it. -->

## Acceptance criteria

What would "fixed" look like for you?