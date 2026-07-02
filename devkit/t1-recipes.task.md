# Task: implement `recipes.py` (named pipeline presets)

Implement a single, pure-standard-library module `recipes.py`.
Write ONLY this one file. Do NOT add any dependencies. Do NOT write any test code
(no `unittest`, no `if __name__ == "__main__":` test block, no asserts, no pytest).

## What this module is
Loom's named pipeline presets: each preset names a list of stages plus a per-stage
carrier (model) override map. Lets users pick a ready-made pipeline by name.

## Milestones (acceptance contract)

- M1: A module-level dict `RECIPES` mapping preset name -> a dict of the shape
  `{"stages": [...], "carriers": {stage: model}}`.
  Include EXACTLY these 4 presets, with EXACTLY these contents:

  - `"cheap-dev"`:
    - stages: `["plan", "implement", "verify", "review"]`
    - carriers: `{"implement": "glm", "review": "glm"}`

  - `"premium-architect"`:
    - stages: `["plan", "implement", "verify", "review"]`
    - carriers: `{"plan": "claude-code-sub", "implement": "glm", "review": "glm"}`

  - `"local-first"`:
    - stages: `["implement", "verify"]`
    - carriers: `{"implement": "deepseek"}`

  - `"agent-team-research"`:
    - stages: `["brainstorm", "plan", "review"]`
    - carriers: `{}`  (empty dict)

- M2: `def get_recipe(name) -> dict` returns the preset dict for `name`.
  Raises `KeyError` for an unknown name. (Direct dict indexing satisfies this.)

- M3: `def list_recipes() -> list` returns the SORTED list of preset names.
  e.g. `["agent-team-research", "cheap-dev", "local-first", "premium-architect"]`.

## Style (M4)
- Pure standard library only. No imports needed beyond stdlib (likely none at all).
- Short docstrings on the module and on each function.
- The file's FIRST line must be exactly: `# recipes.py`
- NO test code in the module.
