import os, json, sys, datetime
sys.path.insert(0, os.path.dirname(__file__))
# 兼容旧位置：之前是根目录 env_gate.py，2026-07 v0.1 整理后挪到 scripts/debug/
# 这里同时支持两种调用方式（包内调用 vs 包外调用）
try:
    from debug.env_gate import evaluate_gate, _fs_read       # python -m scripts.debug.report_env
except ImportError:                                          # 直接 import report_env
    from env_gate import evaluate_gate, _fs_read             # type: ignore[no-redef]

def render(run_id: str, paths) -> str:
    r = evaluate_gate(paths, _fs_read)
    ts = datetime.datetime.utcnow().isoformat() + "Z"
    body = f"""# env-provision-gate report

- run_id: {run_id}
- generated_at: {ts}
- mode: report_only
- paths_probed: {json.dumps(paths)}

## Result

- status: {r['status']}
- inspected_path: {r['inspected_path']}
- assignment_count: {r['assignment_count']}
- violations_empty: {json.dumps(r['violations']['empty'])}
- violations_placeholder: {json.dumps(r['violations']['placeholder'])}
- reason: {r['reason']}

## Acceptance Mapping

1. test -f (任一存在): {'PASS' if r['inspected_path'] else 'FAIL'}
2. grep -cE '^[A-Z_]+=\\S+' >= 2: {'PASS' if r['assignment_count'] >= 2 else 'FAIL'}
3. no '=\\s*$' and no '=your-key-here': {'PASS' if not r['violations']['empty'] and not r['violations']['placeholder'] else 'FAIL'}
"""
    return body

if __name__ == "__main__":
    run_id = os.environ.get("RUN_ID", "manual")
    paths = ["devkit/.env", "agent-platform/.env"]
    out_dir = f"runs/{run_id}"
    os.makedirs(out_dir, exist_ok=True)
    with open(f"{out_dir}/env-report.md", "w", encoding="utf-8") as f:
        f.write(render(run_id, paths))
    print(f"wrote {out_dir}/env-report.md")
