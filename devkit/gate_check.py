from devkit.contract_check import (
    DEFAULT_CONTRACTS,
    verify_impl_contracts,
    gate_decision,
    format_stage_error_lines,
)

def write_stage_error(impl_dir: str, lines: list[str]) -> str:
    p = Path(impl_dir) / "stage-error.md"
    p.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return str(p)

def run_verify(impl_dir: str) -> int:
    # 1) 在 verify_command 之前先做 AST 符号契约
    result = verify_impl_contracts(impl_dir, DEFAULT_CONTRACTS)
    if not result["all_passed"]:
        lines = format_stage_error_lines(result["missing_symbols"])
        write_stage_error(impl_dir, lines)
        print(f"[gate] {gate_decision(result)}: missing {result['missing_symbols']}")
        return 1  # NO-GO，verify_command 不再执行
    # 2) 原 verify_command 继续
    return verify_command(impl_dir)
