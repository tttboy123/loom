# blocks.py
"""Loom prioritized context-block builder.

Assembles the ordered list of context blocks that ``budget.pack()`` consumes.
Each block is a plain dict ``{"name": str, "text": str, "prio": int,
"protected": bool}``. Lower ``prio`` is more important; ``protected=True``
blocks are never dropped by the packer.
"""

def build_blocks(task, system, upstreams, contract=None, failure=None):
    """Build the ordered list of context blocks for the budget packer.

    Returns a list of dicts, each of shape
    ``{"name": str, "text": str, "prio": int, "protected": bool}``,
    in this fixed order:

      1. ``contract`` (prio 0, protected) — only if ``contract`` is truthy
      2. ``failure``  (prio 0, protected) — only if ``failure`` is truthy
      3. ``system``   (prio 1, not protected) — always present
      4. ``task``     (prio 1, not protected) — always present
      5. each upstream ``(name, text)`` in order (prio 2, not protected)

    Args:
        task: the task description text.
        system: the system prompt text.
        upstreams: list of ``(name, text)`` tuples from prior stages.
        contract: optional sprint contract text; falsy values are skipped.
        failure: optional prior failure/error text; falsy values are skipped.

    Returns:
        list of block dicts suitable to pass directly to ``budget.pack``.
    """
    blocks = []

    if contract:
        blocks.append({
            "name": "contract",
            "text": str(contract),
            "prio": 0,
            "protected": True,
        })

    if failure:
        blocks.append({
            "name": "failure",
            "text": str(failure),
            "prio": 0,
            "protected": True,
        })

    blocks.append({
        "name": "system",
        "text": system,
        "prio": 1,
        "protected": False,
    })

    blocks.append({
        "name": "task",
        "text": task,
        "prio": 1,
        "protected": False,
    })

    for name, text in upstreams:
        blocks.append({
            "name": name,
            "text": text,
            "prio": 2,
            "protected": False,
        })

    return blocks
