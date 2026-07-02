#!/usr/bin/env bash
# =============================================================================
# Loom 一键部署脚本
# 用法：curl -fsSL <url>/install.sh | bash
# 或本地执行：bash install.sh
# 目标：从零到运行第一个 Agent 任务，不超过 5 分钟。
# =============================================================================
set -euo pipefail

C()  { printf "\033[1;36m%s\033[0m\n" "$*"; }
OK() { printf "\033[1;32m✓ %s\033[0m\n" "$*"; }
ERR(){ printf "\033[1;31m✗ %s\033[0m\n" "$*" >&2; }
INF(){ printf "  %s\n" "$*"; }

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || pwd)"

# ── Step 1: Python ────────────────────────────────────────────────────────────
C "Step 1/4  检查 Python 版本…"
if ! command -v python3 &>/dev/null; then
  ERR "未找到 python3，请先安装 Python 3.10+（https://python.org）"
  exit 1
fi
PY_VER=$(python3 -c "import sys; print('%d.%d' % sys.version_info[:2])")
PY_MAJOR=$(python3 -c "import sys; print(sys.version_info[0])")
PY_MINOR=$(python3 -c "import sys; print(sys.version_info[1])")
if [ "$PY_MAJOR" -lt 3 ] || { [ "$PY_MAJOR" -eq 3 ] && [ "$PY_MINOR" -lt 10 ]; }; then
  ERR "需要 Python 3.10+，当前 $PY_VER"
  exit 1
fi
OK "Python $PY_VER"

# ── Step 2: 依赖 ──────────────────────────────────────────────────────────────
C "Step 2/4  安装 Python 依赖…"
if [ -f "$REPO_DIR/requirements.txt" ]; then
  python3 -m pip install --quiet -r "$REPO_DIR/requirements.txt" \
    && OK "requirements.txt 安装完成" \
    || { ERR "pip install 失败，请检查网络或手动运行：pip install -r requirements.txt"; exit 1; }
fi
if [ -f "$REPO_DIR/setup.py" ] || [ -f "$REPO_DIR/pyproject.toml" ]; then
  python3 -m pip install --quiet -e "$REPO_DIR" \
    && OK "包已以 editable 模式安装" || true
fi

# ── Step 3: 配置向导 ──────────────────────────────────────────────────────────
C "Step 3/4  运行 Loom 配置向导（引导填写 API key + 启动 LiteLLM 网关）…"
INF "可按 Ctrl-C 跳过，之后随时运行 ./loom setup 重新配置"
python3 -m devkit setup || {
  ERR "配置向导未完全通过（通常是 Docker 未运行或 API key 未填写）"
  INF "你仍可继续使用：./loom run \"任务\""
  INF "需要帮助？见 USAGE.zh.md 或 QUICKSTART.md"
}

# ── Step 4: 完成 ──────────────────────────────────────────────────────────────
C "Step 4/4  安装完成！"
echo ""
echo "  现在你可以运行："
echo ""
echo "    cd $(realpath "$REPO_DIR")"
printf '    \033[1m./loom run "帮我建一个 todo 应用"\033[0m\n'
echo ""
echo "  其他常用命令："
echo "    ./loom status   — 查看任务进度和决策记录"
echo "    ./loom bench    — 评估各模型速度/质量/成本"
echo "    ./loom doctor   — 诊断服务状态"
echo ""
OK "Loom 已就绪！"
