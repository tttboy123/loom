#!/usr/bin/env bash
# =============================================================================
# 一条命令部署（在你自己的 Mac 上跑）。
#   bash deploy.sh            # 最稳起步栈：LiteLLM + Open WebUI（默认 mock，零 key）
#   bash deploy.sh full       # 完整栈：Agno 多 Agent + LiteLLM + agent-ui + DB
#   bash deploy.sh down       # 停止并清理
# =============================================================================
set -euo pipefail
cd "$(dirname "$0")"

MODE="${1:-minimal}"
say() { printf "\033[1;36m▶ %s\033[0m\n" "$*"; }
err() { printf "\033[1;31m✗ %s\033[0m\n" "$*" >&2; }
ok()  { printf "\033[1;32m✓ %s\033[0m\n" "$*"; }

# ---- 1. 检查 Docker ----------------------------------------------------------
if ! command -v docker >/dev/null 2>&1; then
  err "没装 Docker。请先安装 Docker Desktop：https://www.docker.com/products/docker-desktop/"
  err "装好后打开 Docker Desktop（菜单栏出现鲸鱼图标 = 已运行），再重跑本脚本。"
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  err "Docker 已安装但没在运行。请打开 Docker Desktop 等它启动完成，再重跑本脚本。"
  exit 1
fi
ok "Docker 正常"

COMPOSE="docker compose"
$COMPOSE version >/dev/null 2>&1 || COMPOSE="docker-compose"

# ---- down --------------------------------------------------------------------
if [ "$MODE" = "down" ]; then
  say "停止 minimal 栈"; $COMPOSE -f docker-compose.minimal.yml down || true
  say "停止 full 栈";    $COMPOSE down || true
  ok "已停止"; exit 0
fi

# ---- 2. 起栈 -----------------------------------------------------------------
if [ "$MODE" = "full" ]; then
  [ -f .env ] || { cp .env.example .env; say "已生成 .env，请按需填入真实 key（留空也能跑，对应模型会被跳过）"; }
  say "拉起完整栈（首次会构建镜像 + 拉取 agent-ui，耐心等几分钟）"
  $COMPOSE up -d --build
  UI="http://localhost:3000"; GW="http://localhost:4000/ui"; API="http://localhost:8000/docs"
else
  say "拉起最稳起步栈（LiteLLM + Open WebUI，官方镜像，默认 mock 零 key）"
  $COMPOSE -f docker-compose.minimal.yml up -d
  UI="http://localhost:3000"; GW="http://localhost:4000"; API=""
fi

# ---- 3. 等服务就绪 -----------------------------------------------------------
say "等待网关就绪…"
for i in $(seq 1 60); do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:4000/health/liveliness 2>/dev/null || echo 000)
  [ "$code" != "000" ] && { ok "LiteLLM 网关已就绪"; break; }
  sleep 2
  [ "$i" = "60" ] && err "网关 2 分钟未就绪，看日志：$COMPOSE logs litellm"
done

say "等待聊天控制台就绪…"
for i in $(seq 1 60); do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3000 2>/dev/null || echo 000)
  [ "$code" != "000" ] && { ok "控制台已就绪"; break; }
  sleep 2
done

# ---- 4. 打印结果 -------------------------------------------------------------
echo
ok "部署完成。现在用浏览器打开："
echo "   聊天控制台 : $UI"
echo "   网关/看板  : $GW"
[ -n "$API" ] && echo "   AgentOS API: $API"
echo
echo "下一步：在控制台里随便发一句话。看到回复 = 全链路打通。"
echo "想换真实 key / 订阅后端 → 见 README.md 与 SUBSCRIPTION-BACKENDS.md"
echo "停止：bash deploy.sh down"
