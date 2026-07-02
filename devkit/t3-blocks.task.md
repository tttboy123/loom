# Task: implement `blocks.py` (prioritized context-block builder)

Implement a single, pure-standard-library module `blocks.py`.
Write ONLY this one file. Do NOT add any dependencies. Do NOT write any test code
(no `unittest`, no `if __name__ == "__main__":` test block, no asserts, no pytest).

## What this module is
Loom's prioritized context-block builder. It assembles the ordered list of context
blocks that `budget.pack()` consumes. Each block is a plain dict
`{"name": str, "text": str, "prio": int, "protected": bool}`. Lower `prio` is more
important; `protected=True` blocks are never dropped by the packer.

## Milestones (acceptance contract)

- M1: `def build_blocks(task, system, upstreams, contract=None, failure=None) -> list`
  Returns a list of dicts, each of the shape
  `{"name": str, "text": str, "prio": int, "protected": bool}`,
  suitable to pass directly to `budget.pack`.
  - `task` (str): the task description text.
  - `system` (str): the system prompt text.
  - `upstreams` (list of `(name, text)` tuples): prior-stage artifacts.
  - `contract` (optional): if truthy, the sprint contract text.
  - `failure` (optional): if truthy, a prior failure/error text to feed back.

- M2: ordering / priority rules — produce blocks in EXACTLY this list order:
  1. if `contract` is truthy: `{"name":"contract","text":str(contract),"prio":0,"protected":True}`
  2. if `failure` is truthy: `{"name":"failure","text":str(failure),"prio":0,"protected":True}`
  3. `{"name":"system","text":system,"prio":1,"protected":False}`
  4. `{"name":"task","text":task,"prio":1,"protected":False}`
  5. for each `(name, text)` in `upstreams`, IN ORDER:
     `{"name":name,"text":text,"prio":2,"protected":False}`
  - Skip the `contract` block when `contract` is None/empty/falsy.
  - Skip the `failure` block when `failure` is None/empty/falsy.
  - The `contract` and `failure` blocks are the ONLY protected blocks.
  - `system` and `task` are always present (prio 1, not protected).
  - Examples:
    - `build_blocks('t','s',[],contract='C')[0]` ==
      `{"name":"contract","text":"C","prio":0,"protected":True}`
    - `build_blocks('t','s',[])[0]['name']` == `"system"` (no contract -> system first)
    - `[b['prio'] for b in build_blocks('t','s',[('u1','x')])]` == `[1,1,2]`
    - `[b['name'] for b in build_blocks('t','s',[('u1','x')],contract='C',failure='F')]`
      == `["contract","failure","system","task","u1"]`
    - `[b['name'] for b in build_blocks('t','s',[('u1','x')],contract='C',failure='F') if b['protected']]`
      == `["contract","failure"]`

## Style (M3)
- Pure standard library only. No imports needed (likely none at all).
- Short docstrings on the module and on the function.
- The file's FIRST line must be exactly: `# blocks.py`
- NO test code in the module.
