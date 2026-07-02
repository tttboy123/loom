# recipes.py
"""Loom's named pipeline presets.

Each preset names a list of stages plus a per-stage carrier (model) override
map, letting users pick a ready-made pipeline by name. Pure standard library.
"""

# Module-level dict mapping preset name -> {"stages": [...], "carriers": {...}}.
RECIPES = {
    "cheap-dev": {
        "stages": ["plan", "implement", "verify", "review"],
        "carriers": {
            "plan": "loom-orchestrator",
            "implement": "loom-dev",
            "verify": "loom-tester",
            "review": "loom-reviewer",
        },
    },
    "premium-architect": {
        "stages": ["plan", "implement", "verify", "review"],
        "carriers": {
            "plan": "loom-orchestrator",
            "implement": "loom-dev",
            "verify": "loom-tester",
            "review": "loom-reviewer",
        },
    },
    "local-first": {
        "stages": ["implement", "verify"],
        "carriers": {"implement": "minimax", "verify": "glm"},
    },
    "agent-team-research": {
        "stages": ["brainstorm", "plan", "review"],
        "carriers": {},
    },
}

def get_recipe(name):
    """Return the preset dict for `name`. Raises KeyError if unknown."""
    return RECIPES[name]

def list_recipes():
    """Return the sorted list of preset names."""
    return sorted(RECIPES.keys())
