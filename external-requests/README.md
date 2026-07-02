# Loom External Requests Inbox

This directory is a generic intake inbox for external projects that want Loom to consider upstream capability requests.

Boundaries:

- This is not Loom core execution logic.
- This inbox does not change `devkit/`, recipes, global role mapping, or model defaults.
- External projects can submit generalized capability requests here.
- Loom can scan this inbox and decide whether to accept, reject, or split requests into upstream work.

## Structure

- `requests.yaml`
  - machine-readable request index
- `requests/<source>/`
  - request documents copied from external projects
- `prompts/intake.prompt.md`
  - batch intake prompt for scanning pending requests
- `prompts/<source>/`
  - source-specific request scan prompts

## Intake rule

Only accept requests that improve Loom as a general external execution engine.

Do not accept requests that:

- hardcode one downstream project's docs or backlog model
- force Loom's global defaults to match one project
- are really wrapper glue for one product

## Current source

The first source wired into this inbox is `buddys`.

Its source-of-truth request drafts still live in Buddys' own docs workspace.
What appears here is the published copy that Loom should scan.
